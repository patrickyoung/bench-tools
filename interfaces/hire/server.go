package main

import (
	"bytes"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"
)

//go:embed web
var web embed.FS

type app struct {
	cfg       config
	cat       catalog
	jobs      *jobManager
	templates *template.Template
	hosts     map[string]bool
	csrf      string
	mu        sync.Mutex // Serializes form admission, not execution.
	submitted map[string]string
}

type page struct {
	AllowRun, Runnable                                                       bool
	Check, Output, OutputName, OutputError                                   string
	Title, View, Nav, Query, Filter, Error, CSRF, Nonce, Readme, ReadmeError string
	Catalog                                                                  catalog
	Workers, Teams                                                           []entry
	Entry                                                                    entry
	Jobs                                                                     []Job
	Job                                                                      Job
	Stdout, Stderr, Result                                                   string
	JobLabel, JobNote, WorkerURL                                             string
	AllowBuild                                                               bool
	Form                                                                     map[string]string
}

func randomID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func newApp(cfg config, cat catalog, jobs *jobManager, addr string) (*app, error) {
	funcs := template.FuncMap{"name": displayName, "short": func(s string) string {
		if len(s) > 7 {
			return s[:7]
		}
		return s
	}, "jobLabel": jobLabel, "jobNote": jobNote}
	t, err := template.New("pages").Funcs(funcs).ParseFS(web, "web/*.html")
	if err != nil {
		return nil, err
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	return &app{cfg: cfg, cat: cat, jobs: jobs, templates: t, csrf: randomID(), hosts: map[string]bool{addr: true, net.JoinHostPort("localhost", port): true}, submitted: make(map[string]string)}, nil
}

func (a *app) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", a.home)
	mux.HandleFunc("GET /workers", a.home)
	mux.HandleFunc("GET /local-workers/{id}", a.localDetail)
	mux.HandleFunc("GET /teams", a.home)
	mux.HandleFunc("GET /workers/{id}", func(w http.ResponseWriter, r *http.Request) { a.detail(w, r, "worker") })
	mux.HandleFunc("GET /teams/{id}", func(w http.ResponseWriter, r *http.Request) { a.detail(w, r, "team") })
	mux.HandleFunc("GET /hire", func(w http.ResponseWriter, r *http.Request) {
		a.render(w, 200, page{Title: "Hire a worker", View: "hire", Nav: "hire", Form: map[string]string{"model": a.cfg.Model}})
	})
	mux.HandleFunc("POST /hire", a.hire)
	mux.HandleFunc("POST /exports", a.export)
	mux.HandleFunc("GET /jobs", func(w http.ResponseWriter, r *http.Request) {
		a.render(w, 200, page{Title: "Your activity", View: "jobs", Nav: "jobs", Jobs: a.jobs.List()})
	})
	mux.HandleFunc("GET /jobs/{id}", a.job)
	mux.HandleFunc("GET /jobs/{id}/run", a.runForm)
	mux.HandleFunc("POST /jobs/{id}/run", a.runWorker)
	mux.HandleFunc("GET /jobs/{id}/output", a.runDownload)
	mux.HandleFunc("GET /jobs/{id}/status", a.status)
	mux.HandleFunc("POST /jobs/{id}/cancel", a.cancel)
	mux.HandleFunc("POST /jobs/{id}/verify", a.verify)
	assets, _ := fs.Sub(web, "web")
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(assets)))
	protected := http.NewCrossOriginProtection().Handler(mux)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; script-src 'self'; connect-src 'self'; img-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		if !a.hosts[r.Host] {
			http.Error(w, "Unrecognized local address", http.StatusForbidden)
			return
		}
		if r.Method == "POST" {
			// URL encoding can use twelve bytes for one four-byte Unicode rune.
			// Decoded per-field limits below remain the authoring contract.
			r.Body = http.MaxBytesReader(w, r.Body, 384<<10)
			if err := r.ParseForm(); err != nil {
				a.fail(w, 413, "This request is too large or malformed. Keep the brief under 24,000 characters.")
				return
			}
			if subtle.ConstantTimeCompare([]byte(r.PostForm.Get("csrf")), []byte(a.csrf)) != 1 {
				a.fail(w, 403, "This form has expired. Reload the page and try again.")
				return
			}
		}
		protected.ServeHTTP(w, r)
	})
}

func (a *app) render(w http.ResponseWriter, status int, p page) {
	if p.Catalog.Source == "" {
		p.Catalog = a.catalog()
	}
	p.AllowRun = a.cfg.AllowRun
	p.CSRF, p.Nonce, p.AllowBuild = a.csrf, randomID(), a.cfg.AllowBuild
	var b bytes.Buffer
	if err := a.templates.ExecuteTemplate(&b, "layout", p); err != nil {
		http.Error(w, "Unable to render page", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(b.Bytes())
}
func (a *app) fail(w http.ResponseWriter, status int, message string) {
	a.render(w, status, page{Title: "Something needs attention", View: "error", Error: message})
}

func (a *app) home(w http.ResponseWriter, r *http.Request) {
	p := page{Title: "Your bench", View: "home", Nav: "home", Query: strings.TrimSpace(r.URL.Query().Get("q")), Filter: r.URL.Query().Get("status"), Jobs: a.jobs.List()}
	if len(p.Query) > 500 {
		a.fail(w, 400, "Search is limited to 500 characters.")
		return
	}
	p.Catalog = a.catalog()
	if r.URL.Path == "/workers" {
		p.Title, p.Nav = "Workers", "workers"
	}
	if r.URL.Path == "/teams" {
		p.Title, p.Nav = "Teams", "teams"
	}
	match := func(e entry) bool {
		if p.Filter != "" && e.Status != p.Filter {
			return false
		}
		for _, word := range strings.Fields(strings.ToLower(p.Query)) {
			if strings.Contains(strings.ToLower(e.Name()+" "+e.ID+" "+e.Description), word) {
				return true
			}
		}
		return p.Query == ""
	}
	for _, e := range p.Catalog.Workers {
		if match(e) {
			p.Workers = append(p.Workers, e)
		}
	}
	for _, e := range p.Catalog.Teams {
		if match(e) {
			p.Teams = append(p.Teams, e)
		}
	}
	if len(p.Jobs) > 4 {
		p.Jobs = p.Jobs[:4]
	}
	a.render(w, 200, p)
}

func (a *app) detail(w http.ResponseWriter, r *http.Request, kind string) {
	e, ok := a.cat.find(kind, r.PathValue("id"))
	if !ok {
		a.fail(w, 404, "That entry is not in the selected library.")
		return
	}
	p := page{Title: e.Name(), View: "detail", Nav: kind + "s", Entry: e}
	var err error
	p.Readme, err = readText(a.cfg.Source, filepath.Join(kind+"s", e.ID, "expert", "README.md"), 256<<10)
	if err != nil {
		p.ReadmeError = "The worker guide could not be read safely. Inspect the selected source checkout."
	}
	for _, team := range a.cat.Teams {
		for _, m := range team.Members {
			if kind == "worker" && m.Worker == e.ID {
				p.Teams = append(p.Teams, team)
				break
			}
		}
	}
	a.render(w, 200, p)
}

// A form can start at most one job during this server session. Old forms become
// invalid across restart because their CSRF token changes. Failed admission can retry.
func (a *app) admit(w http.ResponseWriter, r *http.Request, start func() (Job, error)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	nonce := r.PostForm.Get("nonce")
	if !hexID.MatchString(nonce) {
		a.fail(w, 400, "Reload the form before submitting.")
		return
	}
	if id, ok := a.submitted[nonce]; ok {
		http.Redirect(w, r, "/jobs/"+id, 303)
		return
	}
	if len(a.submitted) >= 4096 {
		a.fail(w, 503, "This session has reached its action limit. Restart the interface to begin another session.")
		return
	}
	j, err := start()
	if err != nil {
		if strings.HasSuffix(r.URL.Path, "/run") {
			a.runError(w, r, 409, err.Error())
			return
		}
		if r.URL.Path == "/hire" {
			a.hireError(w, r, 409, err.Error())
			return
		}
		a.fail(w, 409, err.Error())
		return
	}
	a.submitted[nonce] = j.ID
	http.Redirect(w, r, "/jobs/"+j.ID, 303)
}

func (a *app) workspace() (string, error) {
	dir := filepath.Join(a.cfg.Data, "workspaces", randomID(), "work")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	if err := os.Mkdir(filepath.Join(filepath.Dir(dir), "evidence"), 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func (a *app) hireError(w http.ResponseWriter, r *http.Request, status int, message string) {
	values := make(map[string]string)
	for _, key := range []string{"title", "goal", "reads", "writes", "good", "bad", "mode", "model", "model-consent"} {
		values[key] = r.PostForm.Get(key)
	}
	a.render(w, status, page{Title: "Hire a worker", View: "hire", Nav: "hire", Error: message, Form: values})
}

var modelName = regexp.MustCompile(`^[a-z][a-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._:/@+-]*$`)

func (a *app) hire(w http.ResponseWriter, r *http.Request) {
	title, goal, mode := strings.TrimSpace(r.PostForm.Get("title")), strings.TrimSpace(r.PostForm.Get("goal")), r.PostForm.Get("mode")
	if title == "" || !utf8.ValidString(title) || utf8.RuneCountInString(title) > 120 || goal == "" || !utf8.ValidString(goal) || utf8.RuneCountInString(goal) > 24000 {
		a.hireError(w, r, 422, "Add a name (up to 120 characters) and a job description (up to 24,000 characters).")
		return
	}
	for _, field := range []struct {
		key   string
		limit int
	}{{"reads", 1000}, {"writes", 1000}, {"good", 1500}, {"bad", 1500}} {
		value := r.PostForm.Get(field.key)
		if !utf8.ValidString(value) || utf8.RuneCountInString(value) > field.limit {
			a.hireError(w, r, 422, "Keep input and output descriptions under 1,000 characters each, and examples under 1,500 characters each.")
			return
		}
	}
	if mode != "blank" && mode != "build" {
		a.hireError(w, r, 400, "Choose a blank draft or a model-backed build.")
		return
	}
	if mode == "build" && !a.cfg.AllowBuild {
		a.hireError(w, r, 403, "Model-backed builds are disabled for this server. Start it with -allow-build to enable them.")
		return
	}
	if mode == "build" && r.PostForm.Get("model-consent") != "yes" {
		a.hireError(w, r, 422, "Confirm use of your configured model account before building.")
		return
	}
	model := strings.TrimSpace(r.PostForm.Get("model"))
	if mode == "build" && (len(model) > 200 || !modelName.MatchString(model)) {
		a.hireError(w, r, 422, "Choose a model in provider/model format before building. Your brief is kept below; no command has started.")
		return
	}
	brief := goal
	for _, field := range []struct{ key, label string }{{"reads", "Inputs"}, {"writes", "Outputs"}, {"good", "Acceptable result"}, {"bad", "Result to reject"}} {
		if s := strings.TrimSpace(r.PostForm.Get(field.key)); s != "" {
			brief += "\n\n" + field.label + ":\n" + s
		}
	}
	a.admit(w, r, func() (Job, error) {
		dir, err := a.workspace()
		if err != nil {
			return Job{}, err
		}
		briefPath := filepath.Join(filepath.Dir(dir), "brief.txt")
		if err := os.WriteFile(briefPath, []byte(brief+"\n"), 0600); err != nil {
			return Job{}, err
		}
		args := []string{a.cfg.Hire, "new", filepath.Join(dir, "expert"), brief}
		kind := "new"
		if mode == "build" {
			kind = "build"
			args = []string{a.cfg.Hire, "build", "-C", dir, "-evidence", filepath.Join(filepath.Dir(dir), "evidence"), "-goal-file", briefPath, "-m", model, "-turns", "8", "-timeout", "10m"}
		}
		return a.jobs.Start(kind, title, dir, args, "")
	})
}

func (a *app) export(w http.ResponseWriter, r *http.Request) {
	kind := r.PostForm.Get("kind")
	if kind != "worker" && kind != "team" {
		a.fail(w, 400, "Unknown library type.")
		return
	}
	e, ok := a.cat.find(kind, r.PostForm.Get("id"))
	if !ok || !e.Exportable() {
		a.fail(w, 422, "This library entry is unavailable for a new export.")
		return
	}
	if e.Status == "experimental" && r.PostForm.Get("experimental") != "yes" {
		a.fail(w, 422, "Acknowledge experimental status before exporting.")
		return
	}
	if r.PostForm.Get("revision") != a.cat.Revision {
		a.fail(w, 409, "The source selection changed. Reload before exporting.")
		return
	}
	a.admit(w, r, func() (Job, error) {
		dir, err := a.workspace()
		if err != nil {
			return Job{}, err
		}
		command := "export"
		if kind == "team" {
			command = "export-team"
		}
		args := []string{a.cfg.Python, filepath.Join(a.cfg.Source, "scripts", "workers"), command, e.ID, filepath.Join(dir, "export"), "--ref", a.cat.Revision}
		if e.Status == "experimental" {
			args = append(args, "--allow-experimental")
		}
		return a.jobs.Start("export", e.Name(), dir, args, "")
	})
}

func resultPath(j Job) string {
	if j.Kind == "run" {
		return j.Dir
	}
	if j.Kind == "export" {
		return filepath.Join(j.Dir, "export", "expert")
	}
	if j.Kind == "verify" && len(j.Args) == 3 {
		return j.Args[2]
	}
	return filepath.Join(j.Dir, "expert")
}

func (a *app) job(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	if !ok {
		a.fail(w, 404, "That activity record was not found.")
		return
	}
	p := page{Title: j.Title, View: "job", Nav: "jobs", Job: j, Stdout: a.jobs.Log(j.ID, "stdout"), Stderr: a.jobs.Log(j.ID, "stderr"), Result: resultPath(j)}
	p.JobLabel, p.JobNote = jobSummary(j, p.Stderr)
	_, p.Runnable = a.runnable(j.ID)
	if j.Kind == "run" {
		p.OutputName = runOutput(j)
		if !j.Active() && p.OutputName != "" {
			var err error
			p.Output, err = a.readRunOutput(j)
			if err != nil {
				p.OutputError = "No readable result file yet. Inspect the command output and diagnostics."
			}
		}
	}
	p.Catalog = a.catalog()
	for _, e := range p.Catalog.Workers {
		if e.Local && e.Path == p.Result {
			p.WorkerURL = e.URL()
			break
		}
	}
	if !j.Active() && j.Kind != "run" {
		relative, relErr := filepath.Rel(a.cfg.Data, filepath.Join(p.Result, "README.md"))
		readme, err := readText(a.cfg.Data, relative, 64<<10)
		if relErr != nil {
			err = relErr
		}
		if err == nil {
			p.Readme = readme
		} else {
			p.ReadmeError = "No readable README.md yet. A blank draft needs a README and a real acceptance check before verification can pass."
		}
	}
	a.render(w, 200, p)
}

func (a *app) status(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	stderr := a.jobs.Log(j.ID, "stderr")
	label, note := jobSummary(j, stderr)
	_ = json.NewEncoder(w).Encode(struct {
		Active                      bool
		Label, Note, Stdout, Stderr string
	}{j.Active(), label, note, a.jobs.Log(j.ID, "stdout"), stderr})
}

func jobSummary(j Job, stderr string) (string, string) {
	if j.Kind == "build" && j.State == "failed" && strings.Contains(stderr, "ask: openai-codex requires -header-fd") {
		return "Connect your model account", "The model is selected, but Ask has no OAuth authorization header for Codex. Configure an authenticated Ask wrapper through AGENT_ASK before starting a new build. Selecting a model does not sign you in. Your brief and diagnostics are retained; this build has not been retried."
	}
	if j.Kind == "build" && j.State == "failed" && strings.Contains(stderr, "ask: no model: pass -m provider/model or set ASK_MODEL") {
		return "Choose a model", "Hire stopped because no model was selected. Set a provider/model in the Hire form and configure its account connection before starting a new build. The original brief and diagnostics are retained; this build has not been retried."
	}
	return jobLabel(j), jobNote(j)
}
func (a *app) cancel(w http.ResponseWriter, r *http.Request) {
	if err := a.jobs.Cancel(r.PathValue("id")); err != nil {
		a.fail(w, 409, err.Error())
		return
	}
	http.Redirect(w, r, "/jobs/"+r.PathValue("id"), 303)
}
func (a *app) verify(w http.ResponseWriter, r *http.Request) {
	j, ok := a.jobs.Get(r.PathValue("id"))
	if !ok || j.Active() || j.Kind == "run" {
		a.fail(w, 409, "Wait for the command to finish before inspecting its definition.")
		return
	}
	target := resultPath(j)
	// Structural verification is delegated to Hire. Never execute bin/check here.
	a.admit(w, r, func() (Job, error) {
		return a.jobs.Start("verify", j.Title, j.Dir, []string{a.cfg.Hire, "verify", target}, "")
	})
}

func jobLabel(j Job) string {
	if j.Active() {
		if j.State == "cancelling" {
			return "Stopping"
		}
		return "In progress"
	}
	if j.ExitCode != nil && *j.ExitCode == 0 && j.State != "unknown" {
		switch j.Kind {
		case "new":
			return "Draft created"
		case "export":
			return "Exported"
		case "verify":
			return "Structure verified"
		case "run":
			return "Run completed"
		case "build":
			return "Build completed"
		}
	}
	switch j.State {
	case "unknown":
		return "Outcome unknown"
	case "unfinished":
		return "Unfinished"
	case "cancelled":
		return "Interrupted"
	}
	if j.ExitCode != nil {
		switch *j.ExitCode {
		case 3:
			return "Declined"
		case 75:
			return "Waiting"
		case 125:
			return "Outcome uncertain"
		}
	}
	return "Needs attention"
}
func jobNote(j Job) string {
	if j.Active() {
		return "The command is running. You can leave this page and return."
	}
	if j.State == "unknown" {
		return "The command's final outcome could not be observed. Inspect its files and evidence before starting again."
	}
	if j.ExitCode != nil && *j.ExitCode == 0 {
		switch j.Kind {
		case "new":
			return "A starting definition is on disk. Add a README and replace the deliberately failing check."
		case "verify":
			return "Hire checked the definition's structure. It did not run the worker or its generated check."
		case "export":
			return "Clean source and its lock file are ready in the selected workspace."
		case "run":
			return "The worker’s check accepted this run. Review the result and its evidence before relying on it."
		case "build":
			return "Inspect the generated definition and acceptance check, then evaluate it on a fresh case."
		}
	}
	return "The command stopped without a completed result. Its output and exact exit status are retained. Nothing retries automatically."
}
