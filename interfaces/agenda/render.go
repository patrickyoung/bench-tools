package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type obligation struct {
	ID            string                     `json:"id"`
	Revision      string                     `json:"revision"`
	Title         string                     `json:"title"`
	Owner         string                     `json:"owner"`
	NotBefore     string                     `json:"not_before"`
	DueAt         string                     `json:"due_at"`
	Timezone      string                     `json:"timezone"`
	Basis         string                     `json:"basis"`
	State         string                     `json:"state"`
	Execution     string                     `json:"execution"`
	Acceptance    string                     `json:"acceptance"`
	Timeliness    string                     `json:"timeliness"`
	Attention     []string                   `json:"attention"`
	ReportedState string                     `json:"reported_state"`
	Extensions    map[string]json.RawMessage `json:"extensions"`
	Observation   *struct {
		Extensions map[string]json.RawMessage `json:"extensions"`
	} `json:"observation"`
}
type attention struct {
	ID      string   `json:"id"`
	Reasons []string `json:"reasons"`
	Reason  string   `json:"reason"`
}
type projection struct {
	Schema      string       `json:"schema"`
	AsOf        string       `json:"as_of"`
	SnapshotSHA string       `json:"snapshot_sha256"`
	Coverage    string       `json:"coverage"`
	Obligations []obligation `json:"obligations"`
	Attention   []attention  `json:"attention"`
	Errors      []attention  `json:"errors"`
	Notice      string       `json:"notice"`
}

func readProjection(raw []byte) (projection, error) {
	var p projection
	if err := decode(raw, &p, false); err != nil {
		return p, err
	}
	if p.Schema != "agenda.projection/v1" {
		return p, errors.New("expected agenda.projection/v1")
	}
	if err := validInstant(p.AsOf); err != nil {
		return p, err
	}
	if p.Coverage != "complete" && p.Coverage != "unverified" {
		return p, errors.New("invalid observation coverage")
	}
	if len(p.Obligations) > 20000 {
		return p, errors.New("projection exceeds 20000 obligations")
	}
	seen := map[string]bool{}
	for _, row := range p.Obligations {
		if row.ID == "" || seen[row.ID] {
			return p, errors.New("missing or duplicate obligation identity")
		}
		seen[row.ID] = true
		if row.DueAt != "" {
			if err := validInstant(row.DueAt); err != nil {
				return p, fmt.Errorf("invalid due date for %s", row.ID)
			}
		}
		if row.Timezone != "" {
			if _, err := time.LoadLocation(row.Timezone); err != nil {
				return p, fmt.Errorf("invalid timezone for %s", row.ID)
			}
		}
	}
	return p, nil
}

type card struct {
	obligation
	Due, Label, Column, Anchor string
	Coordination, Details      string
	Flag                       bool
}
type day struct {
	Number  int
	Outside bool
	Cards   []card
}
type month struct {
	ID, Title string
	Days      []day
}
type column struct {
	Name  string
	Cards []card
}
type page struct {
	projection
	View, Title, OtherView, OtherTitle string
	Live                               bool
	Poll                               int
	Cards, History                     []card
	Months                             []month
	Columns                            []column
}

func cardColumn(r obligation) string {
	if r.State == "cancelled" || r.State == "skipped" {
		return "History"
	}
	if len(r.Attention) > 0 || r.Execution == "unknown" || r.Execution == "rejected" || r.State == "missing" {
		return "Needs attention"
	}
	if r.State == "completed" && (r.Acceptance == "accepted" || r.Acceptance == "reported-done") {
		return "Done"
	}
	if r.ReportedState == "needs-attention" {
		return "Needs attention"
	}
	if r.ReportedState == "in-progress" {
		return "In progress"
	}
	if r.ReportedState == "ready" {
		return "Ready"
	}
	if r.Basis == "human" {
		return "Planned"
	}
	if r.Execution == "queued" {
		return "Ready"
	}
	if r.Execution == "unobserved" || r.Execution == "unsubmitted" {
		return "Planned"
	}
	if r.Execution == "waiting" || r.Execution == "unverified" {
		return "Needs attention"
	}
	if r.Execution == "running" || r.State == "active" {
		return "In progress"
	}
	return "Planned"
}

func makePage(p projection, view string, live bool, poll int) (page, error) {
	if view != "calendar" && view != "kanban" {
		return page{}, errors.New("view must be calendar or kanban")
	}
	data := page{projection: p, View: view, Live: live, Poll: poll, Title: "Calendar", OtherView: "kanban", OtherTitle: "Kanban"}
	if view == "kanban" {
		data.Title = "Kanban"
		data.OtherView = "calendar"
		data.OtherTitle = "Calendar"
	}
	byDay := map[string][]card{}
	months := map[string]time.Time{}
	for i, r := range p.Obligations {
		c := card{obligation: r, Column: cardColumn(r), Anchor: fmt.Sprintf("work-%d", i), Flag: len(r.Attention) > 0}
		details := map[string]any{}
		if len(r.Extensions) > 0 {
			details["work"] = r.Extensions
		}
		if r.Observation != nil && len(r.Observation.Extensions) > 0 {
			details["observation"] = r.Observation.Extensions
			var coordination struct {
				State string `json:"state"`
			}
			if raw, ok := r.Observation.Extensions["human_coordination"]; ok && json.Unmarshal(raw, &coordination) == nil {
				c.Coordination = coordination.State
			}
		}
		if len(details) > 0 {
			raw, _ := json.MarshalIndent(details, "", "  ")
			if len(raw) > 16384 {
				c.Details = string(raw[:16384]) + "\n[Detail shortened; inspect the full projection for all fields.]"
			} else {
				c.Details = string(raw)
			}
		}
		c.Label = "Unconfirmed"
		if r.Acceptance == "reported-done" {
			c.Label = "Human-reported completion"
		} else if r.Acceptance == "accepted" {
			c.Label = "Externally observed acceptance"
		}
		if r.DueAt != "" {
			date, _ := time.Parse(time.RFC3339Nano, r.DueAt)
			zone := time.UTC
			if r.Timezone != "" {
				zone, _ = time.LoadLocation(r.Timezone)
			}
			local := date.In(zone)
			c.Due = local.Format("Jan 2, 2006 · 15:04 MST")
			key := local.Format("2006-01-02")
			byDay[key] = append(byDay[key], c)
			months[local.Format("2006-01")] = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, time.UTC)
		} else {
			c.Due = "No due date"
		}
		data.Cards = append(data.Cards, c)
		if c.Column == "History" {
			data.History = append(data.History, c)
		}
	}
	if len(months) == 0 {
		at, _ := time.Parse(time.RFC3339Nano, p.AsOf)
		months[at.Format("2006-01")] = time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	keys := make([]string, 0, len(months))
	for key := range months {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		first := months[key]
		m := month{ID: key, Title: first.Format("January 2006")}
		start := first.AddDate(0, 0, -(int(first.Weekday())+6)%7)
		end := first.AddDate(0, 1, 0)
		for d := start; d.Before(end) || len(m.Days)%7 != 0; d = d.AddDate(0, 0, 1) {
			m.Days = append(m.Days, day{Number: d.Day(), Outside: d.Month() != first.Month(), Cards: byDay[d.Format("2006-01-02")]})
		}
		data.Months = append(data.Months, m)
	}
	for _, name := range []string{"Planned", "Ready", "In progress", "Needs attention", "Done"} {
		col := column{Name: name}
		for _, c := range data.Cards {
			if c.Column == name {
				col.Cards = append(col.Cards, c)
			}
		}
		data.Columns = append(data.Columns, col)
	}
	return data, nil
}

//go:embed view.html
var pageSource string
var pageTemplate = template.Must(template.New("page").Funcs(template.FuncMap{"join": strings.Join}).Parse(pageSource))

func render(out io.Writer, p projection, view string, live bool, poll int) error {
	if view == "ics" {
		return renderICS(out, p)
	}
	data, err := makePage(p, view, live, poll)
	if err != nil {
		return err
	}
	return pageTemplate.Execute(out, data)
}

type viewServer struct {
	Controller controller
	Poll       int
	mu         sync.Mutex
	cached     projection
	checked    time.Time
}

func (s *viewServer) current(ctx context.Context) (projection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.checked.IsZero() && time.Since(s.checked) < time.Duration(s.Poll)*time.Second {
		return s.cached, nil
	}
	raw, err := s.Controller.project(ctx, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return projection{}, err
	}
	p, err := readProjection(raw)
	if err != nil {
		return projection{}, err
	}
	s.cached = p
	s.checked = time.Now()
	return p, nil
}

func (s *viewServer) handler(port int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorities := map[string]bool{"127.0.0.1:" + strconv.Itoa(port): true, "localhost:" + strconv.Itoa(port): true}
		if !authorities[r.Host] || (r.Header.Get("Origin") != "" && r.Header.Get("Origin") != "http://"+r.Host) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			http.Error(w, "Local same-origin viewing only", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "Read-only view", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.RawQuery != "" {
			http.Error(w, "Unknown query", http.StatusBadRequest)
			return
		}
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/calendar", http.StatusSeeOther)
			return
		}
		view := strings.TrimPrefix(r.URL.Path, "/")
		if view != "calendar" && view != "kanban" && view != "calendar.ics" {
			http.NotFound(w, r)
			return
		}
		p, err := s.current(r.Context())
		if err != nil {
			http.Error(w, "Projection unavailable. Inspect the selected Agenda command and observations; no success is implied.", http.StatusServiceUnavailable)
			return
		}
		var body bytes.Buffer
		renderView := view
		if view == "calendar.ics" {
			renderView = "ics"
		}
		if err = render(&body, p, renderView, true, s.Poll); err != nil {
			http.Error(w, "Projection cannot be rendered", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if view == "calendar.ics" {
			w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
			w.Header().Set("Content-Disposition", `attachment; filename="agenda.ics"`)
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
		_, _ = w.Write(body.Bytes())
	})
}

func serve(ctx context.Context, c controller, port, poll int, diagnostics io.Writer) error {
	if port < 0 || port > 65535 || poll < 1 || poll > 60 {
		return errors.New("port must be 0..65535 and poll-seconds 1..60")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:"+strconv.Itoa(port))
	if err != nil {
		return err
	}
	actual := listener.Addr().(*net.TCPAddr).Port
	views := &viewServer{Controller: c, Poll: poll}
	server := &http.Server{Handler: views.handler(actual), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 40 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	fmt.Fprintf(diagnostics, "Calendar: http://127.0.0.1:%d/calendar\nKanban: http://127.0.0.1:%d/kanban\n", actual, actual)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			stop, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = server.Shutdown(stop)
		case <-done:
		}
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
