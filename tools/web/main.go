package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const version = "2.0.0"
const maxInput = 8 << 20
const help = `web — a filter with a browser
Usage:
  web get|read|text|html|links|snapshot URL [render options]
  web shot URL [out.png] [render options]
  web run [plan.json|-] [--may-job JOB] [--attach ENDPOINT] [--keep | --tab ID]
  web auth URL FILE [--channel chrome] [--attach ENDPOINT] [--may-job JOB]
  web setup | check | version
Render options: --wait domcontentloaded|load|networkidle --timeout MS
                --profile FILE | --attach ENDPOINT
Snapshot option: --records-selector CSS
WEB_BROWSER selects installed Chromium. WEB_ATTACH_TIMEOUT defaults to 10000 ms.
WEB_STATE selects the audit JSONL. See README.md and web(1) for contracts.
`

type failure struct {
	code int
	msg  string
}

func (e *failure) Error() string        { return e.msg }
func bad(format string, a ...any) error { return &failure{2, fmt.Sprintf(format, a...)} }

type options struct {
	attach, profile, tab, wait, selector, channel, job string
	keep                                               bool
	timeout                                            time.Duration
	args                                               []string
}

func parseOptions(cmd string, args []string) (o options, err error) {
	o.wait = "domcontentloaded"
	o.timeout = 30 * time.Second
	allowed := map[string]bool{}
	switch cmd {
	case "run":
		for _, s := range []string{"--attach", "--tab", "--keep", "--may-job"} {
			allowed[s] = true
		}
	case "auth":
		allowed["--attach"] = true
		allowed["--channel"] = true
		allowed["--may-job"] = true
	default:
		for _, s := range []string{"--attach", "--profile", "--wait", "--timeout"} {
			allowed[s] = true
		}
		if cmd == "snapshot" {
			allowed["--records-selector"] = true
		}
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			o.args = append(o.args, args[i+1:]...)
			break
		}
		if a == "-" || !strings.HasPrefix(a, "-") {
			o.args = append(o.args, a)
			continue
		}
		if !allowed[a] {
			return o, bad("unknown option %s for %s", a, cmd)
		}
		if a == "--keep" {
			o.keep = true
			continue
		}
		i++
		if i == len(args) || args[i] == "" {
			return o, bad("%s needs a value", a)
		}
		v := args[i]
		switch a {
		case "--may-job":
			o.job = v
		case "--attach":
			o.attach = endpoint(v)
		case "--profile":
			o.profile = v
		case "--tab":
			o.tab = v
		case "--wait":
			o.wait = v
		case "--records-selector":
			o.selector = v
		case "--channel":
			o.channel = v
		case "--timeout":
			o.timeout, err = milliseconds(v)
			if err != nil {
				return o, err
			}
		}
	}
	if o.attach != "" && o.profile != "" {
		return o, bad("--attach and --profile select conflicting identities")
	}
	if (o.keep || o.tab != "") && o.attach == "" {
		return o, bad("--keep and --tab require --attach")
	}
	if o.keep && o.tab != "" {
		return o, bad("--keep and --tab cannot be combined")
	}
	if o.wait != "load" && o.wait != "domcontentloaded" && o.wait != "networkidle" {
		return o, bad("unknown wait mode %q", o.wait)
	}
	if o.channel != "" && o.channel != "chrome" {
		return o, bad("only --channel chrome is supported; otherwise select WEB_BROWSER")
	}
	min, max := 1, 1
	switch cmd {
	case "run":
		min = 0
	case "shot":
		max = 2
	case "auth":
		min = 2
		max = 2
	}
	if len(o.args) < min || len(o.args) > max {
		return o, bad("wrong number of arguments for %s; see web help", cmd)
	}
	return o, nil
}
func milliseconds(s string) (time.Duration, error) {
	n, e := strconv.ParseInt(s, 10, 64)
	if e != nil || n <= 0 || n > int64((24*time.Hour)/time.Millisecond) {
		return 0, bad("timeout must be positive milliseconds, at most 86400000")
	}
	return time.Duration(n) * time.Millisecond, nil
}
func endpoint(s string) string {
	if _, e := strconv.ParseUint(s, 10, 16); e == nil {
		return "http://127.0.0.1:" + s
	}
	return s
}
func boundedRead(r io.Reader) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, maxInput+1))
	if e == nil && len(b) > maxInput {
		e = bad("input exceeds 8 MiB")
	}
	return b, e
}
func readFile(path string) ([]byte, error) {
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	return boundedRead(f)
}
func writeJSON(w io.Writer, v any) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	b = append(b, '\n')
	_, e = w.Write(b)
	return e
}
func app(ctx context.Context, args []string, in io.Reader, out, stderr io.Writer) error {
	if len(args) == 0 {
		_, e := io.WriteString(out, help)
		return e
	}
	cmd := args[0]
	switch cmd {
	case "help", "--help", "-h":
		_, e := io.WriteString(out, help)
		return e
	case "version", "--version", "-V":
		_, e := fmt.Fprintln(out, "web "+version)
		return e
	case "check":
		if len(args) != 1 {
			return bad("check takes no arguments")
		}
		return selfCheck(out)
	case "setup":
		if len(args) != 1 {
			return bad("setup takes no arguments")
		}
		path, e := browserPath("")
		if e != nil {
			return e
		}
		_, e = fmt.Fprintln(out, path)
		return e
	case "get", "read", "text", "html", "links", "snapshot", "shot", "run", "auth":
	default:
		return &failure{64, "unknown command: " + cmd}
	}
	o, e := parseOptions(cmd, args[1:])
	if e != nil {
		return e
	}
	switch cmd {
	case "run":
		return runPlan(ctx, o, in, out, stderr)
	case "auth":
		return authenticate(ctx, o, in, stderr)
	default:
		return render(ctx, cmd, o, out, stderr)
	}
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	err := app(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err == nil {
		return
	}
	code := 1
	var f *failure
	if errors.As(err, &f) {
		code = f.code
	}
	fmt.Fprintln(os.Stderr, "web:", err)
	os.Exit(code)
}
