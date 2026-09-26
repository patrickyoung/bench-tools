package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var inputName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

func validRunFile(name string) bool { return inputName.MatchString(name) && name != "state" }

// The operator selects this endpoint at startup, never a page or a worker.
func browserAddress(endpoint string) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil || len(endpoint) > 2048 || strings.ContainsAny(endpoint, "\r\n") {
		return "", fmt.Errorf("-web-attach requires a loopback HTTP or WebSocket endpoint")
	}
	ip := net.ParseIP(u.Hostname())
	port, portErr := strconv.Atoi(u.Port())
	if (u.Scheme != "http" && u.Scheme != "ws") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback())) || portErr != nil || port < 1 || port > 65535 {
		return "", fmt.Errorf("-web-attach requires a loopback HTTP or WebSocket endpoint with an explicit port and no credentials")
	}
	return net.JoinHostPort(u.Hostname(), u.Port()), nil
}

func (a *app) runnable(id string) (Job, bool) {
	j, ok := a.jobs.Get(id)
	if !ok || j.State != "completed" || j.ExitCode == nil || *j.ExitCode != 0 {
		return Job{}, false
	}
	if j.Kind == "export" && len(j.Args) > 2 && j.Args[2] == "export" {
		return j, true
	}
	for _, e := range a.catalog().Workers {
		if e.Local && e.JobID == id {
			return j, true
		}
	}
	return Job{}, false
}

func (a *app) runPage(w http.ResponseWriter, r *http.Request, status int, message string, form map[string]string) {
	j, ok := a.runnable(r.PathValue("id"))
	if !ok {
		a.fail(w, 404, "Choose a completed worker build, verified draft, or exported worker.")
		return
	}
	relative, err := filepath.Rel(a.cfg.Data, resultPath(j))
	if err != nil {
		a.fail(w, 404, "The worker folder is unavailable.")
		return
	}
	guide, err := readText(a.cfg.Data, filepath.Join(relative, "README.md"), 64<<10)
	if err != nil {
		a.fail(w, 404, "The worker guide is unavailable.")
		return
	}
	check, err := readText(a.cfg.Data, filepath.Join(relative, "bin", "check"), 256<<10)
	if err != nil {
		a.fail(w, 404, "The worker check is unavailable.")
		return
	}
	if form == nil {
		form = map[string]string{"model": a.cfg.Model, "turns": "50", "output": "report.md"}
		if strings.Contains(guide, "`tracking_numbers.txt`") {
			form["input-name"] = "tracking_numbers.txt"
			form["output"] = "tracking_report.md"
			form["goal"] = "Look up the supplied tracking numbers using the carrier and procedure in the worker guide, and write the tracking report."
		}
	}
	a.render(w, status, page{Title: "Run " + j.Title, View: "run", Nav: "workers", Job: j, Readme: guide, Check: check, Form: form, Error: message, WebAttach: a.cfg.WebAttach})
}
func (a *app) runForm(w http.ResponseWriter, r *http.Request) { a.runPage(w, r, 200, "", nil) }
func (a *app) runError(w http.ResponseWriter, r *http.Request, status int, message string) {
	f := map[string]string{}
	for _, key := range []string{"goal", "model", "turns", "input-name", "input", "output", "network", "connected-browser", "run-consent"} {
		f[key] = r.PostForm.Get(key)
	}
	a.runPage(w, r, status, message, f)
}
func (a *app) runWorker(w http.ResponseWriter, r *http.Request) {
	j, ok := a.runnable(r.PathValue("id"))
	if !ok {
		a.fail(w, 409, "This worker is not ready for a new run. Check its latest activity.")
		return
	}
	if !a.cfg.AllowRun {
		a.runError(w, r, 403, "Worker runs are disabled for this server. Start it with -allow-run to enable them.")
		return
	}
	goal, model := strings.TrimSpace(r.PostForm.Get("goal")), strings.TrimSpace(r.PostForm.Get("model"))
	name, content, output := strings.TrimSpace(r.PostForm.Get("input-name")), r.PostForm.Get("input"), strings.TrimSpace(r.PostForm.Get("output"))
	turns, err := strconv.Atoi(r.PostForm.Get("turns"))
	if goal == "" || !utf8.ValidString(goal) || utf8.RuneCountInString(goal) > 24000 || len(model) > 200 || !modelName.MatchString(model) || err != nil || turns < 1 || turns > 100 {
		a.runError(w, r, 422, "Enter a task, a provider/model, and a turn limit from 1 to 100.")
		return
	}
	if !validRunFile(output) || (name != "" && !validRunFile(name)) || name == output || (content != "" && name == "") || len(content) > 128<<10 || !utf8.ValidString(content) {
		a.runError(w, r, 422, "Use distinct simple input and result filenames (letters, numbers, dots, underscores or hyphens). Input text is limited to 128 KiB; state is reserved.")
		return
	}
	if r.PostForm.Get("run-consent") != "yes" {
		a.runError(w, r, 422, "Review the worker guide and check, then confirm the run.")
		return
	}
	useBrowser := r.PostForm.Get("connected-browser") == "yes"
	if useBrowser {
		address, err := browserAddress(a.cfg.WebAttach)
		if err != nil {
			a.runError(w, r, 422, "No browser connection is configured for this server.")
			return
		}
		connection, err := net.DialTimeout("tcp", address, time.Second)
		if err != nil {
			a.runError(w, r, 422, "The selected browser is unavailable. Reconnect it before starting, or turn off Use connected browser.")
			return
		}
		connection.Close()
		goal += "\n\nBrowser connection selected for this run by the caller:\nUse the configured Web command with --attach " + strconv.Quote(a.cfg.WebAttach) + ". The browser is already running outside Cage; do not start another browser inside Cage. Use only Web's own new tab, do not close the browser or touch existing tabs, and retain Web's action approval gates. Do not discover other sessions or fall back to a new browser if this connection fails. Prefer bounded content/selector waits; networkidle can time out on otherwise usable pages. Cage remains enabled; networking is explicitly allowed for this connection and page access.\n"
	}
	agent := a.cfg.Agent
	if agent == "" {
		agent = "agent"
	}
	agent, err = exec.LookPath(agent)
	if err != nil {
		a.runError(w, r, 422, "Agent is not installed or configured for this server.")
		return
	}
	agent, err = filepath.Abs(agent)
	if err != nil {
		a.runError(w, r, 422, "Agent could not be resolved.")
		return
	}
	a.admit(w, r, func() (Job, error) {
		dir, err := a.workspace()
		if err != nil {
			return Job{}, err
		}
		goalFile := filepath.Join(filepath.Dir(dir), "task.txt")
		if err = os.WriteFile(goalFile, []byte(goal+"\n"), 0600); err != nil {
			return Job{}, err
		}
		args := []string{agent, "run", "-C", dir, "-evidence", filepath.Join(filepath.Dir(dir), "evidence"), "-goal-file", goalFile, "-m", model, "-turns", strconv.Itoa(turns), "-timeout", "10m", "-checkpoint", "run", "-record-output", output}
		if name != "" {
			if err = os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
				return Job{}, err
			}
			args = append(args, "-record-input", name)
		}
		if r.PostForm.Get("network") == "yes" || useBrowser {
			args = append(args, "-net")
		}
		args = append(args, resultPath(j))
		return a.jobs.Start("run", j.Title, dir, args, "")
	})
}
func runOutput(j Job) string {
	if j.Kind != "run" {
		return ""
	}
	for i, arg := range j.Args {
		if arg == "-record-output" && i+1 < len(j.Args) && validRunFile(j.Args[i+1]) {
			return j.Args[i+1]
		}
	}
	return ""
}
func (a *app) readRunOutput(j Job) (string, error) {
	name := runOutput(j)
	if name == "" {
		return "", fmt.Errorf("no result selected")
	}
	relative, err := filepath.Rel(a.cfg.Data, filepath.Join(j.Dir, name))
	if err != nil {
		return "", err
	}
	return readText(a.cfg.Data, relative, 2<<20)
}
func (a *app) runDownload(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	if !ok || j.Kind != "run" || j.Active() {
		http.NotFound(w, r)
		return
	}
	output, err := a.readRunOutput(j)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+runOutput(j)+`"`)
	_, _ = w.Write([]byte(output))
}
