package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Workers produce local files. Only the controller can publish or share them.
type deliveryArtifact struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Original    string   `json:"original"`
	Preview     string   `json:"preview,omitempty"`
	Thumbnail   string   `json:"thumbnail,omitempty"`
	Pages       []string `json:"pages,omitempty"`
}
type deliverySpec struct {
	Title     string             `json:"title"`
	Summary   string             `json:"summary,omitempty"`
	Artifacts []deliveryArtifact `json:"artifacts"`
	Files     []string           `json:"files,omitempty"`
}
type deliveryState struct {
	Slug, VersionID, TaskID, ShareID, SharedVersion, SharedTask, URL string
	Revoked                                                          bool
}
type deliveryRecord struct {
	Thread, TaskID, Action, Root, StateFile, Manifest, RequestID, ShareRequestID, Plonk, URL, TokenFile string
	Before                                                                                              deliveryState
}
type deliveryView struct {
	Enabled, Busy, Ready, Shared, Revoked, Current, Local bool
	JobID, URL, Message                                   string
}

func (a *app) deliveryState(thread string) deliveryState {
	var s deliveryState
	if hexID.MatchString(thread) {
		if raw, e := os.ReadFile(filepath.Join(a.cfg.Data, "deliveries", thread+".json")); e == nil {
			_ = json.Unmarshal(raw, &s)
		}
	}
	return s
}
func (a *app) deliveryView(thread, taskID string) deliveryView {
	s := a.deliveryState(thread)
	v := deliveryView{Enabled: a.cfg.Plonk != "" && a.cfg.PlonkURL != "" && a.cfg.PlonkTokenFile != "", Ready: s.TaskID == taskID && s.VersionID != "", Shared: s.ShareID != "" && !s.Revoked, URL: s.URL, Revoked: s.Revoked}
	for _, j := range a.jobs.List() {
		if j.Kind != "delivery" {
			continue
		}
		var rec deliveryRecord
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(j.Dir), "delivery-request.json"))
		if err != nil || json.Unmarshal(raw, &rec) != nil || rec.Thread != thread {
			continue
		}
		v.JobID = j.ID
		v.Busy = j.Active()
		if v.Busy {
			v.Message = "Preparing your share link…"
			if rec.Action == "prepare" {
				v.Message = "Opening your results…"
			}
			if rec.Action == "revoke" {
				v.Message = "Stopping access to the link…"
			}
		} else if j.State == "unknown" {
			v.Message = "The last sharing step was interrupted. Your files are safe. Choose the action again to check and finish that same request."
		} else if j.ExitCode != nil && *j.ExitCode != 0 {
			v.Message = "I couldn’t finish the sharing step. Your files are safe here. Try again when your connection is ready."
		}
		break
	}
	v.Current = v.Shared && s.SharedTask == taskID
	if u, e := url.Parse(s.URL); e == nil {
		ip := net.ParseIP(u.Hostname())
		v.Local = u.Hostname() == "localhost" || strings.HasSuffix(u.Hostname(), ".localhost") || (ip != nil && ip.IsLoopback())
	}
	if v.Shared && s.SharedTask != taskID && !v.Busy {
		v.Message = "Your link still shows the earlier work. Share this update when it’s ready."
	}
	return v
}
func makeDeliverySpec(root string, result taskResult) (deliverySpec, error) {
	spec := deliverySpec{Title: result.Title, Summary: truncateMessage(result.Message, 1200)}
	if spec.Title == "" {
		spec.Title = "Your work"
	}
	known := map[string]bool{}
	for _, f := range result.Artifacts {
		known[f.Name] = true
		spec.Files = append(spec.Files, f.Name)
	}
	if raw, err := taskFile(root, "delivery.json"); err == nil {
		var supplied deliverySpec
		if strictJSON(raw, &supplied) == nil && len(supplied.Artifacts) > 0 && len(supplied.Artifacts) <= 32 && boundedText(supplied.Title, 200, true) && boundedText(supplied.Summary, 4000, false) {
			valid := true
			seen := map[string]bool{}
			for _, f := range supplied.Artifacts {
				if !known[f.Original] || !validRunFile(f.ID) || seen[f.ID] || !boundedText(f.Title, 200, true) || !boundedText(f.Description, 2000, false) {
					valid = false
				}
				seen[f.ID] = true
				for _, p := range append([]string{f.Preview, f.Thumbnail}, f.Pages...) {
					if p != "" && !known[p] {
						valid = false
					}
				}
			}
			if valid {
				supplied.Files = spec.Files
				return supplied, nil
			}
		}
	}
	for i, f := range result.Artifacts {
		spec.Artifacts = append(spec.Artifacts, deliveryArtifact{ID: fmt.Sprintf("item-%d", i+1), Title: strings.TrimSuffix(f.Name, filepath.Ext(f.Name)), Original: f.Name})
	}
	if len(spec.Artifacts) == 0 {
		return spec, fmt.Errorf("there are no finished files to share yet")
	}
	return spec, nil
}
func (a *app) taskDeliver(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	if !ok || j.Kind != "task" || j.Active() {
		a.fail(w, 404, "That work is unavailable.")
		return
	}
	rec, err := a.taskRecord(j)
	if err != nil {
		a.fail(w, 404, "That conversation is unavailable.")
		return
	}
	action := r.PostForm.Get("action")
	if action != "prepare" && action != "share" && action != "revoke" {
		a.fail(w, 400, "Choose a delivery action.")
		return
	}
	if a.cfg.Plonk == "" || a.cfg.PlonkURL == "" || a.cfg.PlonkTokenFile == "" {
		a.fail(w, 409, "Sharing hasn’t been connected yet. Your downloads are ready in the conversation.")
		return
	}
	out := &assistantResponse{header: make(http.Header)}
	a.admit(out, r, func() (Job, error) {
		state := a.deliveryState(rec.Thread)
		latest := ""
		for _, t := range a.taskTurns(rec.Thread) {
			if t.Job.Active() {
				return Job{}, fmt.Errorf("let this work finish before sharing")
			}
			if len(t.Result.Artifacts) > 0 {
				latest = t.Job.ID
			}
		}
		if latest != j.ID {
			return Job{}, fmt.Errorf("open the latest work before sharing an update")
		}
		if action == "revoke" && state.ShareID == "" {
			return Job{}, fmt.Errorf("this work has no shared link")
		}
		dir, err := a.workspace()
		if err != nil {
			return Job{}, err
		}
		parent := filepath.Dir(dir)
		root := filepath.Join(parent, "package")
		if err = os.Mkdir(root, 0700); err != nil {
			return Job{}, err
		}
		// Revocation needs only the retained share identity, never local files.
		if action != "revoke" {
			spec, err := makeDeliverySpec(filepath.Join(filepath.Dir(j.Dir), "deliverables"), a.taskResult(j))
			if err != nil {
				return Job{}, err
			}
			for _, name := range spec.Files {
				b, e := taskFile(filepath.Join(filepath.Dir(j.Dir), "deliverables"), name)
				if e != nil {
					return Job{}, e
				}
				if e = os.WriteFile(filepath.Join(root, name), b, 0600); e != nil {
					return Job{}, e
				}
			}
			if err = saveTaskJSON(filepath.Join(root, "delivery.json"), spec); err != nil {
				return Job{}, err
			}
		}
		if err = os.MkdirAll(filepath.Join(a.cfg.Data, "deliveries"), 0700); err != nil {
			return Job{}, err
		}
		record := deliveryRecord{Thread: rec.Thread, TaskID: j.ID, Action: action, Root: root, StateFile: filepath.Join(a.cfg.Data, "deliveries", rec.Thread+".json"), Manifest: "delivery.json", RequestID: "hire-" + j.ID, ShareRequestID: "share-" + randomID(), Plonk: a.cfg.Plonk, URL: a.cfg.PlonkURL, TokenFile: a.cfg.PlonkTokenFile, Before: state}
		// Reuse the exact interrupted request/base. Only the server receipt can tell
		// whether the earlier attempt committed. Any intervening operation ends
		// this replay chain, so old snapshots cannot undo later revocation.
		for _, prior := range a.jobs.List() {
			if prior.Kind != "delivery" {
				continue
			}
			var p deliveryRecord
			b, e := os.ReadFile(filepath.Join(filepath.Dir(prior.Dir), "delivery-request.json"))
			if e == nil && json.Unmarshal(b, &p) == nil && p.Thread == rec.Thread {
				if prior.Active() {
					return Job{}, fmt.Errorf("sharing is still working")
				}
				if p.TaskID == j.ID && p.Action == action && (prior.ExitCode == nil || *prior.ExitCode != 0) {
					record.Before = p.Before
					record.RequestID = p.RequestID
					record.ShareRequestID = p.ShareRequestID
				}
				break
			}
		}
		recordPath := filepath.Join(parent, "delivery-request.json")
		if err = saveTaskJSON(recordPath, record); err != nil {
			return Job{}, err
		}
		self, err := os.Executable()
		if err != nil {
			return Job{}, err
		}
		title := "Share your work"
		if action == "prepare" {
			title = "Open your results"
		}
		if action == "revoke" {
			title = "Stop sharing your work"
		}
		return a.jobs.Start("delivery", title, dir, []string{self, "delivery", recordPath}, "")
	})
	if out.status != 303 {
		a.fail(w, out.status, "I couldn’t start that sharing step yet. Your work is safe; return to the conversation and try again.")
		return
	}
	http.Redirect(w, r, "/work/"+rec.Thread, 303)
}
func deliveryProcess(recordPath string) int {
	var rec deliveryRecord
	raw, err := os.ReadFile(recordPath)
	if err != nil || json.Unmarshal(raw, &rec) != nil {
		fmt.Fprintln(os.Stderr, "invalid delivery request")
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	state := rec.Before
	run := func(action string, output any, args ...string) error {
		argv := []string{action, "-url", rec.URL, "-token-file", rec.TokenFile}
		argv = append(argv, args...)
		cmd := exec.CommandContext(ctx, rec.Plonk, argv...)
		cmd.Dir = rec.Root
		var out limitedDeliveryBuffer
		cmd.Stdout = &out
		cmd.Stderr = os.Stderr
		cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
		cmd.WaitDelay = 2 * time.Second
		fmt.Fprintln(os.Stderr, "Preparing delivery:", action)
		if err := cmd.Run(); err != nil {
			return err
		}
		return json.Unmarshal(out.b, output)
	}
	fail := func(err error) int { fmt.Fprintln(os.Stderr, "Delivery unfinished:", err); return 1 }
	if rec.Action != "revoke" && state.TaskID != rec.TaskID {
		var published struct {
			Slug      string `json:"slug"`
			VersionID string `json:"versionId"`
		}
		args := []string{"-root", rec.Root, "-manifest", rec.Manifest, "-request-id", rec.RequestID}
		if state.Slug != "" {
			args = append(args, "-slug", state.Slug, "-base-version", state.VersionID)
		}
		if err = run("publish", &published, args...); err != nil {
			return fail(err)
		}
		if published.Slug == "" || published.VersionID == "" {
			return fail(fmt.Errorf("publisher returned no version"))
		}
		state.Slug, state.VersionID, state.TaskID = published.Slug, published.VersionID, rec.TaskID
		if err = saveTaskJSON(rec.StateFile, state); err != nil {
			return fail(err)
		}
	}
	var shared struct {
		Share struct {
			ID        string `json:"id"`
			VersionID string `json:"versionId"`
		} `json:"share"`
		URL string `json:"url"`
	}
	if rec.Action == "share" {
		if state.ShareID == "" || state.Revoked {
			err = run("share", &shared, "-slug", state.Slug, "-version", state.VersionID, "-request-id", rec.ShareRequestID)
		} else {
			err = run("update-share", &shared, "-share", state.ShareID, "-base-version", state.SharedVersion, "-version", state.VersionID)
		}
		if err != nil {
			return fail(err)
		}
		u, e := url.Parse(shared.URL)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || shared.Share.ID == "" {
			return fail(fmt.Errorf("publisher returned no usable link"))
		}
		state.ShareID, state.SharedVersion, state.SharedTask, state.URL, state.Revoked = shared.Share.ID, shared.Share.VersionID, rec.TaskID, shared.URL, false
	}
	if rec.Action == "revoke" {
		if err = run("revoke-share", &shared, "-share", state.ShareID); err != nil {
			return fail(err)
		}
		state.Revoked = true
	}
	if err = saveTaskJSON(rec.StateFile, state); err != nil {
		return fail(err)
	}
	fmt.Fprintln(os.Stdout, "Your delivery is ready.")
	return 0
}

type limitedDeliveryBuffer struct{ b []byte }

func (b *limitedDeliveryBuffer) Write(p []byte) (int, error) {
	if len(b.b)+len(p) > 1<<20 {
		return 0, fmt.Errorf("delivery receipt exceeds bounds")
	}
	b.b = append(b.b, p...)
	return len(p), nil
}

// Authenticate every private gallery descendant. Publishing tokens never reach
// the browser, generated files, or workers.
func (a *app) taskDeliveryPreview(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	if !ok || j.Kind != "task" || j.Active() {
		http.NotFound(w, r)
		return
	}
	rec, err := a.taskRecord(j)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	state := a.deliveryState(rec.Thread)
	if state.TaskID != j.ID || state.VersionID == "" {
		http.NotFound(w, r)
		return
	}
	rest := r.PathValue("rest")
	if strings.Contains(rest, "..") || strings.ContainsAny(rest, "\\\x00") {
		http.NotFound(w, r)
		return
	}
	remoteBase := "/api/v1/bundles/" + url.PathEscape(state.Slug) + "/view/v/" + url.PathEscape(state.VersionID) + "/"
	localBase := "/work/jobs/" + j.ID + "/delivery/"
	endpoint := strings.TrimRight(a.cfg.PlonkURL, "/") + remoteBase + escapedDeliveryPath(rest)
	if r.URL.Query().Get("download") == "1" {
		endpoint += "?download=1"
	}
	token, err := os.ReadFile(a.cfg.PlonkTokenFile)
	if err != nil {
		a.fail(w, 503, "The private preview connection is unavailable.")
		return
	}
	upstream, err := http.NewRequestWithContext(r.Context(), "GET", endpoint, nil)
	if err != nil {
		a.fail(w, 503, "Preview unavailable.")
		return
	}
	upstream.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	if value := r.Header.Get("Range"); value != "" {
		upstream.Header.Set("Range", value)
	}
	hc := http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := hc.Do(upstream)
	if err != nil {
		a.fail(w, 503, "The preview connection is unavailable. Your local downloads are still ready.")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 303 && strings.HasPrefix(rest, "web/") {
		target, e := url.Parse(resp.Header.Get("Location"))
		if e == nil && target.User == nil && target.Host != "" && (target.Scheme == "https" || target.Scheme == "http") && strings.HasPrefix(target.Path, "/p/") {
			http.Redirect(w, r, target.String(), 303)
			return
		}
	}
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		a.fail(w, 502, "This preview is temporarily unavailable. Your local downloads are still ready.")
		return
	}
	for _, key := range []string{"Content-Type", "Content-Disposition", "Content-Security-Policy", "X-Content-Type-Options", "Content-Range", "Accept-Ranges", "Referrer-Policy"} {
		if value := resp.Header.Get(key); value != "" {
			w.Header().Set(key, value)
		}
	}
	w.Header().Set("Cache-Control", "private, no-store")
	if strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") && resp.Header.Get("Content-Disposition") == "" {
		b, e := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if e != nil {
			a.fail(w, 502, "Preview unavailable.")
			return
		}
		b = []byte(strings.ReplaceAll(string(b), remoteBase, localBase))
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(b)
		return
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 110<<20))
}
func escapedDeliveryPath(p string) string {
	parts := strings.Split(p, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

func validateDeliveryConfig(cfg config) error {
	if cfg.Plonk == "" && cfg.PlonkURL == "" && cfg.PlonkTokenFile == "" {
		return nil
	}
	if cfg.Plonk == "" || cfg.PlonkURL == "" || cfg.PlonkTokenFile == "" {
		return fmt.Errorf("select all three Plonk connection options")
	}
	u, err := url.Parse(cfg.PlonkURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return fmt.Errorf("invalid Plonk server URL")
	}
	ip := net.ParseIP(u.Hostname())
	local := u.Hostname() == "localhost" || (ip != nil && ip.IsLoopback())
	if u.Scheme != "https" && !(u.Scheme == "http" && local) {
		return fmt.Errorf("Plonk needs HTTPS, or HTTP on loopback")
	}
	info, err := os.Stat(cfg.PlonkTokenFile)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 4096 {
		return fmt.Errorf("Plonk connection file must be private and bounded")
	}
	return nil
}

func (a *app) taskDeliveryStop(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	if !ok || j.Kind != "delivery" {
		http.NotFound(w, r)
		return
	}
	var rec deliveryRecord
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(j.Dir), "delivery-request.json"))
	if err != nil || json.Unmarshal(raw, &rec) != nil {
		http.NotFound(w, r)
		return
	}
	if j.Active() {
		if err = a.jobs.Cancel(j.ID); err != nil {
			a.fail(w, 409, "That sharing step has already stopped.")
			return
		}
	}
	http.Redirect(w, r, "/work/"+rec.Thread, 303)
}

type taskOutput struct {
	Title, Name, Type, Preview, Description string
	Index, Image                            int
	HasImage                                bool
}

func taskOutputs(root string, result taskResult) []taskOutput {
	spec, err := makeDeliverySpec(root, result)
	if err != nil {
		return nil
	}
	indexes := map[string]int{}
	for i, f := range result.Artifacts {
		indexes[f.Name] = i
	}
	out := []taskOutput{}
	for _, a := range spec.Artifacts {
		i, ok := indexes[a.Original]
		if !ok {
			continue
		}
		f := result.Artifacts[i]
		item := taskOutput{Title: a.Title, Name: f.Name, Type: f.Type, Index: i, Description: a.Description}
		thumb := a.Thumbnail
		if thumb == "" && (f.Type == "image/png" || f.Type == "image/jpeg" || f.Type == "image/webp") {
			thumb = f.Name
		}
		if n, ok := indexes[thumb]; ok {
			kind := taskTypes[strings.ToLower(filepath.Ext(thumb))]
			if kind == "image/png" || kind == "image/jpeg" || kind == "image/webp" {
				item.Image = n
				item.HasImage = true
			}
		}
		if !item.HasImage && f.Type == "text/plain" {
			item.Preview = truncateMessage(f.Preview, 600)
		}
		out = append(out, item)
	}
	return out
}
