package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/cdp"
	"github.com/go-rod/rod/lib/proto"
)

type session struct {
	browser         *rod.Browser
	page            *rod.Page
	socket          *cdp.WebSocket
	proc            *exec.Cmd
	done            chan error
	dir             string
	attached, named bool
	cancel          context.CancelFunc
}

func browserPath(channel string) (string, error) {
	if p := os.Getenv("WEB_BROWSER"); p != "" && channel == "" {
		v, e := exec.LookPath(p)
		if e != nil {
			return "", fmt.Errorf("WEB_BROWSER: %w", e)
		}
		return v, nil
	}
	candidates := []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"}
	if runtime.GOOS == "darwin" {
		candidates = append([]string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", "/Applications/Chromium.app/Contents/MacOS/Chromium"}, candidates...)
	}
	for _, p := range candidates {
		if v, e := exec.LookPath(p); e == nil {
			return v, nil
		}
	}
	return "", fmt.Errorf("install Chrome/Chromium using your package manager, or set WEB_BROWSER to its executable; web setup only checks the installation")
}
func resolveEndpoint(ctx context.Context, endpoint string) (string, error) {
	u, e := url.Parse(endpoint)
	if e != nil {
		return "", e
	}
	if (u.Scheme == "ws" || u.Scheme == "wss") && u.Host != "" {
		return endpoint, nil
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", bad("attach endpoint must be a port, HTTP(S) endpoint or browser WebSocket URL")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/json/version"
	req, e := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if e != nil {
		return "", e
	}
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return "", e
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("CDP endpoint returned HTTP %d", resp.StatusCode)
	}
	var v struct {
		URL string `json:"webSocketDebuggerUrl"`
	}
	e = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&v)
	if e != nil {
		return "", e
	}
	if v.URL == "" {
		return "", fmt.Errorf("endpoint did not advertise a browser WebSocket")
	}
	return v.URL, nil
}
func startSession(ctx context.Context, o options, headed bool, stderr io.Writer) (s *session, err error) {
	life, cancel := context.WithCancel(ctx)
	s = &session{attached: o.attach != "", named: o.tab != "", cancel: cancel}
	defer func() {
		if err != nil {
			s.close(false, stderr)
		}
	}()
	timeout := 10 * time.Second
	if v := os.Getenv("WEB_ATTACH_TIMEOUT"); v != "" {
		timeout, err = milliseconds(v)
		if err != nil {
			return s, err
		}
	}
	var state *storageState
	if o.profile != "" {
		state, err = loadState(o.profile)
		if err != nil {
			return s, err
		}
	}
	connect, stop := context.WithTimeout(life, timeout)
	defer stop()
	address := o.attach
	if !s.attached {
		path, e := browserPath(o.channel)
		if e != nil {
			return s, e
		}
		s.dir, e = os.MkdirTemp("", "web-browser-")
		if e != nil {
			return s, e
		}
		argv := []string{"--remote-debugging-port=0", "--remote-debugging-address=127.0.0.1", "--user-data-dir=" + s.dir, "--no-first-run", "--no-default-browser-check", "--enable-automation"}
		if !headed {
			argv = append(argv, "--headless=new")
		}
		argv = append(argv, "about:blank")
		s.proc = exec.Command(path, argv...)
		s.proc.Stderr = stderr
		if e = s.proc.Start(); e != nil {
			return s, e
		}
		s.done = make(chan error, 1)
		go func() { s.done <- s.proc.Wait() }()
		tick := time.NewTicker(25 * time.Millisecond)
		defer tick.Stop()
	launch:
		for {
			select {
			case <-connect.Done():
				return s, fmt.Errorf("browser launch: %w", connect.Err())
			case e := <-s.done:
				s.done = nil
				return s, fmt.Errorf("browser exited before CDP became available: %v", e)
			case <-tick.C:
				b, e := os.ReadFile(filepath.Join(s.dir, "DevToolsActivePort"))
				if e == nil {
					lines := strings.Split(strings.TrimSpace(string(b)), "\n")
					if len(lines) >= 2 {
						address = "ws://127.0.0.1:" + lines[0] + lines[1]
						break launch
					}
				}
			}
		}
	}
	address, err = resolveEndpoint(connect, address)
	if err != nil {
		return s, fmt.Errorf("connect browser: %w", err)
	}
	ws := &cdp.WebSocket{}
	if err = ws.Connect(connect, address, nil); err != nil {
		return s, fmt.Errorf("connect browser: %w", err)
	}
	s.socket = ws
	// Own the socket explicitly: disconnecting must never send Browser.close to
	// an attached browser or close a caller-named/kept target.
	s.browser = rod.New().Context(life).Client(cdp.New().Start(ws)).NoDefaultDevice()
	stopSetup := context.AfterFunc(connect, cancel)
	defer stopSetup()
	if err = s.browser.Connect(); err != nil {
		return s, err
	}
	s.browser = s.browser.Context(life)
	// Rod retains a page's original context in input/event helpers. Allocate
	// it under the session lifetime, with a watchdog that cancels failed setup.
	b := s.browser
	if o.tab != "" {
		info, e := (proto.TargetGetTargetInfo{TargetID: proto.TargetTargetID(o.tab)}).Call(b)
		if e != nil {
			return s, e
		}
		contexts, e := (proto.TargetGetBrowserContexts{}).Call(b)
		if e != nil {
			return s, e
		}
		private := false
		for _, id := range contexts.BrowserContextIDs {
			if id == info.TargetInfo.BrowserContextID {
				private = true
			}
		}
		if info.TargetInfo.Type != "page" || private {
			return s, fmt.Errorf("named tab is not a page in the default browser context")
		}
		s.page, err = b.PageFromTarget(proto.TargetTargetID(o.tab))
	} else {
		s.page, err = b.Page(proto.TargetCreateTarget{})
	}
	if err != nil {
		return s, err
	}
	s.page = s.page.Context(life)
	if !s.attached {
		if err = (proto.EmulationSetDeviceMetricsOverride{Width: 1280, Height: 720, DeviceScaleFactor: 1, Mobile: false}).Call(s.page.Context(connect)); err != nil {
			return s, err
		}
	}
	if state != nil {
		if err = applyState(s, state); err != nil {
			return s, err
		}
	}
	return s, nil
}
func (s *session) close(keep bool, stderr io.Writer) {
	cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if s.browser != nil {
		if !s.attached {
			_ = s.browser.Context(cleanup).Close()
		} else if s.page != nil {
			if s.named {
				fmt.Fprintf(stderr, "web: left named tab %s open\n", s.page.TargetID)
			} else if keep {
				fmt.Fprintf(stderr, "web: leaving tab open (--keep)\nweb: tab %s   <- pass this to --tab to come back to it\n", s.page.TargetID)
			} else {
				_, _ = (proto.TargetCloseTarget{TargetID: s.page.TargetID}).Call(s.browser.Context(cleanup))
			}
		}
	}
	if s.socket != nil {
		_ = s.socket.Close()
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.proc != nil && s.proc.Process != nil && s.done != nil {
		select {
		case <-s.done:
		case <-cleanup.Done():
			_ = s.proc.Process.Kill()
			<-s.done
		}
	}
	if s.dir != "" {
		_ = os.RemoveAll(s.dir)
	}
}
func navigate(p *rod.Page, address, wait string) error {
	name := proto.PageLifecycleEventNameDOMContentLoaded
	if wait == "load" {
		name = proto.PageLifecycleEventNameLoad
	}
	if wait == "networkidle" {
		name = proto.PageLifecycleEventNameNetworkIdle
	}
	// Bind readiness to this navigation's main frame/loader. An iframe event
	// or the preceding document must not satisfy the requested lifecycle wait.
	ctx, cancel := context.WithCancel(p.GetContext())
	defer cancel()
	p = p.Context(ctx)
	if e := (proto.PageSetLifecycleEventsEnabled{Enabled: true}).Call(p); e != nil {
		return e
	}
	defer func() { _ = (proto.PageSetLifecycleEventsEnabled{Enabled: false}).Call(p) }()
	var frame proto.PageFrameID
	var loader proto.NetworkLoaderID
	ready := p.EachEvent(func(e *proto.PageLifecycleEvent) bool {
		return e.Name == name && e.FrameID == frame && e.LoaderID == loader
	})
	result, e := (proto.PageNavigate{URL: address}).Call(p)
	if e != nil {
		return e
	}
	if result.ErrorText != "" {
		return fmt.Errorf("navigate: %s", result.ErrorText)
	}
	frame, loader = result.FrameID, result.LoaderID
	if loader == "" {
		return nil
	} // A fragment/same-document navigation has no new lifecycle.
	ready()
	return ctx.Err()
}
func pageValue(p *rod.Page, js string, args ...any) (json.RawMessage, error) {
	v, e := p.Eval(js, args...)
	if e != nil {
		return nil, e
	}
	return v.Value.MarshalJSON()
}
func pageString(p *rod.Page, js string, args ...any) (string, error) {
	v, e := p.Eval(js, args...)
	if e != nil {
		return "", e
	}
	return v.Value.Str(), nil
}
func screenshot(p *rod.Page, path string) error {
	b, e := p.Screenshot(true, &proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng})
	if e != nil {
		return e
	}
	return os.WriteFile(path, b, 0600)
}

const snapshotJS = `(selector) => {
 const visible = el => el.getClientRects().length > 0 && getComputedStyle(el).visibility !== 'hidden' && getComputedStyle(el).display !== 'none';
 const links = root => [...(root.matches && root.matches('a[href]') ? [root] : []), ...root.querySelectorAll('a[href]')].filter(visible).map(el => ({label: (el.innerText || el.getAttribute('aria-label') || el.getAttribute('title') || '').trim(), url: new URL(el.getAttribute('href'), document.baseURI).href}));
 return {version:1,url:location.href,title:document.title,text:document.body ? document.body.innerText : '',links:links(document),records:selector ? [...document.querySelectorAll(selector)].filter(visible).map(el => ({text:el.innerText || '',links:links(el)})) : []};
}`

func render(ctx context.Context, cmd string, o options, out, stderr io.Writer) (err error) {
	start := time.Now()
	size := 0
	auditURL := o.args[0]
	defer func() {
		result := "ok"
		if err != nil {
			result = "fail"
		}
		audit(cmd, auditURL, size, start, result, mode(o), "", "")
	}()
	s, e := startSession(ctx, o, false, stderr)
	if e != nil {
		return e
	}
	defer s.close(false, stderr)
	run, cancel := context.WithTimeout(ctx, o.timeout)
	defer cancel()
	p := s.page.Context(run)
	if e = navigate(p, o.args[0], o.wait); e != nil {
		return e
	}
	var data []byte
	switch cmd {
	case "snapshot":
		raw, e := pageValue(p, snapshotJS, o.selector)
		if e != nil {
			return e
		}
		var obj map[string]any
		if e = json.Unmarshal(raw, &obj); e != nil {
			return e
		}
		obj["requested_url"] = o.args[0]
		auditURL, _ = obj["url"].(string)
		data, e = json.Marshal(obj)
		if e != nil {
			return e
		}
		data = append(data, '\n')
		if len(data) > maxInput {
			return fmt.Errorf("snapshot exceeds 8 MiB; select a smaller page or record set")
		}
	case "shot":
		path := "shot.png"
		if len(o.args) > 1 {
			path = o.args[1]
		}
		if e = screenshot(p, path); e != nil {
			return e
		}
		if st, e := os.Stat(path); e == nil {
			size = int(st.Size())
		}
		_, e = fmt.Fprintln(out, path)
		return e
	default:
		src, e := p.HTML()
		if e != nil {
			return e
		}
		base, e := pageString(p, `() => document.baseURI`)
		if e != nil {
			return e
		}
		md, links := reduceHTML(src, base)
		switch cmd {
		case "html":
			data = []byte(strings.TrimSuffix(src, "\n") + "\n")
		case "text":
			data = []byte(plainText(md))
		case "links":
			data = []byte(linkText(links))
		default:
			data = []byte(md)
		}
	}
	size = len(data)
	_, e = out.Write(data)
	return e
}
