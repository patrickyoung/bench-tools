// Agenda records promises and projects finite obligations. It never executes work.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"
	"unicode/utf8"
)

const version = "0.1.0"
const maxJSON = 2 << 20
const maxSnapshot = 128 << 20
const maxEvidence = 64 << 20
const maxRecords = 2000

type Evidence struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type Change struct {
	Schema    string          `json:"schema"`
	RequestID string          `json:"request_id"`
	Kind      string          `json:"kind"`
	ID        string          `json:"id"`
	Previous  *string         `json:"previous"`
	By        string          `json:"by"`
	Reason    string          `json:"reason"`
	Value     json.RawMessage `json:"value"`
	Evidence  []Evidence      `json:"evidence,omitempty"`
}
type Record struct {
	Schema     string            `json:"schema"`
	Sequence   int               `json:"sequence"`
	Previous   *string           `json:"previous"`
	RecordedAt string            `json:"recorded_at"`
	Change     Change            `json:"change"`
	Data       map[string]string `json:"data,omitempty"`
	Revision   string            `json:"revision,omitempty"`
}
type Snapshot struct {
	Schema  string   `json:"schema"`
	Records []Record `json:"records"`
	Head    *string  `json:"head"`
}
type OccurrenceBinding struct {
	ScheduleID       string `json:"schedule_id"`
	Date             string `json:"date"`
	ScheduleRevision string `json:"schedule_revision"`
}
type Item struct {
	Title      string                     `json:"title"`
	Owner      string                     `json:"owner"`
	NotBefore  string                     `json:"not_before"`
	DueAt      string                     `json:"due_at"`
	Timezone   string                     `json:"timezone"`
	Basis      string                     `json:"basis"`
	Parent     string                     `json:"parent,omitempty"`
	Occurrence *OccurrenceBinding         `json:"occurrence,omitempty"`
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}
type Schedule struct {
	ID                string                     `json:"id,omitempty"`
	Title             string                     `json:"title"`
	Owner             string                     `json:"owner"`
	Timezone          string                     `json:"timezone"`
	StartDate         string                     `json:"start_date"`
	EndDate           string                     `json:"end_date"`
	Weekdays          []int                      `json:"weekdays"`
	ExcludedDates     []string                   `json:"excluded_dates"`
	StartTime         string                     `json:"start_time"`
	DueTime           string                     `json:"due_time"`
	DueDayOffset      int                        `json:"due_day_offset"`
	Missed            string                     `json:"missed"`
	EffectiveFrom     string                     `json:"effective_from"`
	CheckEverySeconds int                        `json:"check_every_seconds"`
	Extensions        map[string]json.RawMessage `json:"extensions,omitempty"`
}
type Report struct {
	Target      string                     `json:"target"`
	State       string                     `json:"state"`
	CompletedAt string                     `json:"completed_at,omitempty"`
	Extensions  map[string]json.RawMessage `json:"extensions,omitempty"`
}
type Disposition struct {
	Target           string                     `json:"target"`
	Kind             string                     `json:"kind"`
	ScheduleRevision string                     `json:"schedule_revision,omitempty"`
	Extensions       map[string]json.RawMessage `json:"extensions,omitempty"`
}
type Reference struct {
	Kind string `json:"kind"`
	Ref  string `json:"ref"`
}
type Observation struct {
	ID          string                     `json:"id"`
	Revision    string                     `json:"revision"`
	State       string                     `json:"state"`
	ObservedAt  string                     `json:"observed_at"`
	CompletedAt string                     `json:"completed_at,omitempty"`
	ValidUntil  string                     `json:"valid_until,omitempty"`
	Evidence    []Reference                `json:"evidence"`
	Extensions  map[string]json.RawMessage `json:"extensions,omitempty"`
}
type Occurrence struct {
	ID         string `json:"id"`
	ScheduleID string `json:"schedule_id"`
	Date       string `json:"date"`
	NotBefore  string `json:"not_before"`
	DueAt      string `json:"due_at"`
	Timezone   string `json:"timezone"`
	Revision   string `json:"revision"`
	Eligible   bool   `json:"eligible"`
	Overdue    bool   `json:"overdue"`
}
type Obligation struct {
	ID             string                     `json:"id"`
	Revision       string                     `json:"revision"`
	Title          string                     `json:"title"`
	Owner          string                     `json:"owner"`
	NotBefore      string                     `json:"not_before"`
	DueAt          string                     `json:"due_at"`
	Timezone       string                     `json:"timezone"`
	Basis          string                     `json:"basis"`
	State          string                     `json:"state"`
	Execution      string                     `json:"execution"`
	Acceptance     string                     `json:"acceptance"`
	Timeliness     string                     `json:"timeliness"`
	CompletedAt    *string                    `json:"completed_at"`
	CalendarID     *string                    `json:"calendar_id"`
	Attention      []string                   `json:"attention"`
	Extensions     map[string]json.RawMessage `json:"extensions"`
	Observation    *Observation               `json:"observation,omitempty"`
	ReportedState  string                     `json:"reported_state,omitempty"`
	ReportRevision string                     `json:"report_revision,omitempty"`
}
type Attention struct {
	ID      string   `json:"id"`
	Reasons []string `json:"reasons"`
}
type Finding struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}
type Projection struct {
	Schema         string       `json:"schema"`
	AsOf           string       `json:"as_of"`
	SnapshotSHA256 string       `json:"snapshot_sha256"`
	Coverage       string       `json:"coverage"`
	Obligations    []Obligation `json:"obligations"`
	Attention      []Attention  `json:"attention"`
	Errors         []Finding    `json:"errors"`
	Notice         string       `json:"notice"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func encoded(v any) []byte {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	return append(b, '\n')
}
func fail(s string) error { return errors.New(s) }
func bounded(r io.Reader, n int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, n+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > n {
		return nil, fail("input exceeds bound")
	}
	return b, nil
}
func validID(s string) bool {
	return utf8.ValidString(s) && len(s) > 0 && len(s) <= 256 && !strings.ContainsAny(s, "\x00\r\n")
}
func validText(s string) bool {
	return utf8.ValidString(s) && len(s) <= 8192 && strings.TrimSpace(s) != "" && !strings.ContainsRune(s, 0)
}
func validHash(s string) bool {
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && strings.ToLower(s) == s
}
func member(s string, choices ...string) bool {
	for _, c := range choices {
		if s == c {
			return true
		}
	}
	return false
}
func instant(s string) (time.Time, error) {
	if strings.Contains(s, ",") {
		return time.Time{}, fail("timestamp fraction requires a decimal point")
	}
	if len(s) >= 6 && (s[len(s)-6] == '+' || s[len(s)-6] == '-') {
		hour, eh := strconv.Atoi(s[len(s)-5 : len(s)-3])
		minute, em := strconv.Atoi(s[len(s)-2:])
		if eh != nil || em != nil || hour > 23 || minute > 59 {
			return time.Time{}, fail("invalid timestamp offset")
		}
	}
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil {
		return t, fail("timestamp must include RFC3339 date, seconds and offset")
	}
	return t, nil
}
func location(s string) (*time.Location, error) {
	if s == "" || s == "Local" {
		return nil, fail("explicit IANA timezone required; Local is not portable")
	}
	return time.LoadLocation(s)
}
func stamp(t time.Time) string         { return t.UTC().Format(time.RFC3339Nano) }
func date(s string) (time.Time, error) { return time.Parse("2006-01-02", s) }
func samePtr(a, b *string) bool        { return a == nil && b == nil || a != nil && b != nil && *a == *b }
func ptr(s string) *string             { return &s }
func sameInstant(a, b string) bool {
	x, ex := instant(a)
	y, ey := instant(b)
	return ex == nil && ey == nil && x.Equal(y)
}

// Token validation rejects duplicate keys and excessive depth before typed decode.
// encoding/json otherwise silently replaces invalid UTF-16 escape sequences.
func strict(raw []byte, into any) error {
	if !utf8.Valid(raw) {
		return fail("invalid UTF-8")
	}
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		if i+1 >= len(raw) {
			break
		}
		if raw[i+1] != 'u' {
			i++
			continue
		}
		if i+6 > len(raw) {
			break
		}
		u, e := strconv.ParseUint(string(raw[i+2:i+6]), 16, 16)
		if e != nil {
			continue
		}
		if u >= 0xD800 && u <= 0xDBFF {
			if i+12 > len(raw) || string(raw[i+6:i+8]) != "\\u" {
				return fail("unpaired Unicode surrogate")
			}
			v, e := strconv.ParseUint(string(raw[i+8:i+12]), 16, 16)
			if e != nil || v < 0xDC00 || v > 0xDFFF {
				return fail("unpaired Unicode surrogate")
			}
			i += 11
		} else if u >= 0xDC00 && u <= 0xDFFF {
			return fail("unpaired Unicode surrogate")
		} else {
			i += 5
		}
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var walk func(int) error
	walk = func(depth int) error {
		if depth > 32 {
			return fail("JSON depth exceeds 32")
		}
		tok, e := d.Token()
		if e != nil {
			return e
		}
		switch x := tok.(type) {
		case json.Delim:
			if x == '{' {
				seen := map[string]bool{}
				for d.More() {
					k, e := d.Token()
					if e != nil {
						return e
					}
					s, ok := k.(string)
					if !ok || seen[s] {
						return fail("duplicate JSON key")
					}
					seen[s] = true
					if e = walk(depth + 1); e != nil {
						return e
					}
				}
				end, e := d.Token()
				if e != nil || end != json.Delim('}') {
					return fail("invalid object")
				}
			} else if x == '[' {
				for d.More() {
					if e := walk(depth + 1); e != nil {
						return e
					}
				}
				end, e := d.Token()
				if e != nil || end != json.Delim(']') {
					return fail("invalid array")
				}
			} else {
				return fail("unexpected delimiter")
			}
		case json.Number:
			if _, e := strconv.ParseFloat(string(x), 64); e != nil {
				return fail("number outside finite range")
			}
		}
		return nil
	}
	if e := walk(0); e != nil {
		return e
	}
	if _, e := d.Token(); e != io.EOF {
		return fail("trailing JSON")
	}
	d = json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(into)
}

func parseValue[T any](c Change) (T, error) { var v T; e := strict(c.Value, &v); return v, e }
func validateItem(v Item) error {
	if !validText(v.Title) || !validText(v.Owner) || !member(v.Basis, "external", "human") {
		return fail("invalid item title, owner or basis")
	}
	a, e := instant(v.NotBefore)
	if e != nil {
		return e
	}
	b, e := instant(v.DueAt)
	if e != nil || b.Before(a) {
		return fail("invalid item dates")
	}
	if _, e = location(v.Timezone); e != nil {
		return fail("invalid timezone")
	}
	if v.Parent != "" && !validID(v.Parent) {
		return fail("invalid parent")
	}
	if v.Occurrence != nil {
		if !validID(v.Occurrence.ScheduleID) || !validHash(v.Occurrence.ScheduleRevision) {
			return fail("invalid occurrence binding")
		}
		if _, e = date(v.Occurrence.Date); e != nil {
			return e
		}
	}
	return nil
}

func local(day time.Time, clock string, zone *time.Location) (time.Time, error) {
	if len(clock) != 5 || clock[2] != ':' {
		return time.Time{}, fail("local clock requires HH:MM")
	}
	c, e := time.Parse("15:04", clock)
	if e != nil {
		return time.Time{}, fail("local clock requires HH:MM")
	}
	wall := time.Date(day.Year(), day.Month(), day.Day(), c.Hour(), c.Minute(), 0, 0, time.UTC)
	offsets := map[int]bool{}
	for h := -48; h <= 48; h++ {
		_, off := wall.Add(time.Duration(h) * time.Hour).In(zone).Zone()
		offsets[off] = true
	}
	matches := []time.Time{}
	for off := range offsets {
		t := wall.Add(-time.Duration(off) * time.Second)
		z := t.In(zone)
		if z.Year() == day.Year() && z.Month() == day.Month() && z.Day() == day.Day() && z.Hour() == c.Hour() && z.Minute() == c.Minute() {
			matches = append(matches, t)
		}
	}
	if len(matches) != 1 {
		return time.Time{}, fail("ambiguous or nonexistent local time")
	}
	return matches[0], nil
}
func expand(s Schedule, id, revision string, asof time.Time) ([]Occurrence, error) {
	if !validID(id) || s.ID != "" && s.ID != id || !validText(s.Title) || !validText(s.Owner) {
		return nil, fail("invalid schedule identity/title/owner")
	}
	zone, e := location(s.Timezone)
	if e != nil {
		return nil, fail("invalid schedule timezone")
	}
	start, e := date(s.StartDate)
	if e != nil {
		return nil, e
	}
	end, e := date(s.EndDate)
	if e != nil || end.Before(start) || end.Sub(start) > 365*24*time.Hour {
		return nil, fail("schedule must span 1..366 days")
	}
	effective, e := date(s.EffectiveFrom)
	if e != nil || effective.Before(start) || effective.After(end) {
		return nil, fail("invalid effective_from")
	}
	if s.DueDayOffset < 0 || s.DueDayOffset > 30 || !member(s.Missed, "all", "latest", "skip") || s.CheckEverySeconds < 30 || s.CheckEverySeconds > 604800 {
		return nil, fail("invalid schedule bounds/policy")
	}
	if len(s.Weekdays) < 1 || len(s.Weekdays) > 7 || len(s.ExcludedDates) > 366 {
		return nil, fail("invalid weekday/exclusion bounds")
	}
	days := map[int]bool{}
	for _, d := range s.Weekdays {
		if d < 0 || d > 6 || days[d] {
			return nil, fail("invalid weekdays")
		}
		days[d] = true
	}
	excluded := map[string]bool{}
	for _, d := range s.ExcludedDates {
		if _, e = date(d); e != nil || excluded[d] {
			return nil, fail("invalid excluded dates")
		}
		excluded[d] = true
	}
	result := []Occurrence{}
	latest := -1
	for day := start; !day.After(end); day = day.AddDate(0, 0, 1) {
		if !days[(int(day.Weekday())+6)%7] || excluded[day.Format("2006-01-02")] {
			continue
		}
		begin, e := local(day, s.StartTime, zone)
		if e != nil {
			return nil, e
		}
		due, e := local(day.AddDate(0, 0, s.DueDayOffset), s.DueTime, zone)
		if e != nil {
			return nil, e
		}
		if due.Before(begin) {
			return nil, fail("due time precedes start")
		}
		eligible := !begin.After(asof)
		overdue := asof.After(due)
		if s.Missed == "skip" && overdue {
			eligible = false
		}
		if eligible {
			latest = len(result)
		}
		d := day.Format("2006-01-02")
		result = append(result, Occurrence{ID: id + "/" + d, ScheduleID: id, Date: d, NotBefore: stamp(begin), DueAt: stamp(due), Timezone: s.Timezone, Revision: revision, Eligible: eligible, Overdue: overdue})
	}
	if s.Missed == "latest" {
		for i := range result {
			result[i].Eligible = i == latest
		}
	}
	return result, nil
}

func latest(s Snapshot, kind, id string) *Record {
	for i := len(s.Records) - 1; i >= 0; i-- {
		r := &s.Records[i]
		if r.Change.Kind == kind && r.Change.ID == id {
			return r
		}
	}
	return nil
}
func recordHash(r Record) string { r.Revision = ""; return digest(encoded(r)) }
func humanCompletion(s Snapshot, id string) *string {
	var completed *string
	prior := ""
	for _, r := range s.Records {
		if r.Change.Kind != "report" || r.Change.ID != id {
			continue
		}
		v, _ := parseValue[Report](r.Change)
		if v.State == "done" && prior != "done" {
			completed = ptr(r.RecordedAt)
			if v.CompletedAt != "" {
				completed = ptr(v.CompletedAt)
			}
		}
		if v.State != "done" {
			completed = nil
		}
		prior = v.State
	}
	return completed
}
func validateChange(s Snapshot, c Change, at time.Time) error {
	if c.Schema != "agenda.change/v1" || !validID(c.ID) || !validID(c.RequestID) || !validText(c.By) || !validText(c.Reason) || len(c.Evidence) > 16 {
		return fail("invalid change envelope")
	}
	old := latest(s, c.Kind, c.ID)
	var rev *string
	if old != nil {
		rev = &old.Revision
	}
	if !samePtr(c.Previous, rev) {
		return fail("revision conflict")
	}
	seen := map[string]bool{}
	for _, v := range c.Evidence {
		if !filepath.IsAbs(v.Path) || !validHash(v.SHA256) || seen[v.SHA256] {
			return fail("invalid evidence selection")
		}
		seen[v.SHA256] = true
	}
	switch c.Kind {
	case "item":
		v, e := parseValue[Item](c)
		if e != nil {
			return e
		}
		if e = validateItem(v); e != nil {
			return e
		}
		if v.Parent == c.ID {
			return fail("item cannot parent itself")
		}
		if old != nil {
			prior, _ := parseValue[Item](old.Change)
			if v.Basis != prior.Basis || v.Parent != prior.Parent {
				return fail("item basis and parent are immutable")
			}
		}
		if v.Occurrence != nil && c.ID != v.Occurrence.ScheduleID+"/"+v.Occurrence.Date {
			return fail("occurrence identity mismatch")
		}
	case "schedule":
		v, e := parseValue[Schedule](c)
		if e != nil {
			return e
		}
		if _, e = expand(v, c.ID, "", at); e != nil {
			return e
		}
		if old == nil {
			if v.EffectiveFrom != v.StartDate {
				return fail("first schedule revision starts at start_date")
			}
		} else {
			prior, _ := parseValue[Schedule](old.Change)
			zone, _ := location(v.Timezone)
			if prior.Timezone != v.Timezone || v.EffectiveFrom <= prior.EffectiveFrom || v.EffectiveFrom <= at.In(zone).Format("2006-01-02") {
				return fail("schedule revision must preserve timezone and take effect after today and prior revision")
			}
		}
	case "report":
		v, e := parseValue[Report](c)
		if e != nil {
			return e
		}
		if v.Target != c.ID || !member(v.State, "planned", "ready", "in-progress", "needs-attention", "done", "cancelled") {
			return fail("invalid report")
		}
		item := latest(s, "item", v.Target)
		if item == nil {
			return fail("report target is absent")
		}
		target, _ := parseValue[Item](item.Change)
		if target.Basis != "human" {
			return fail("human report cannot complete external work")
		}
		if v.State == "done" && len(c.Evidence) == 0 {
			return fail("done report requires selected evidence")
		}
		begin, _ := instant(target.NotBefore)
		if v.State == "done" && at.Before(begin) {
			return fail("done report precedes item start")
		}
		if v.CompletedAt != "" {
			done, err := instant(v.CompletedAt)
			begin, _ := instant(target.NotBefore)
			if err != nil || v.State != "done" || done.After(at) || done.Before(begin) {
				return fail("reported completion must be between item start and recording time")
			}
		}
		if old != nil {
			prior, _ := parseValue[Report](old.Change)
			if prior.State == "done" && v.State == "done" && v.CompletedAt != "" {
				completion := humanCompletion(s, c.ID)
				if completion == nil || !sameInstant(*completion, v.CompletedAt) {
					return fail("uninterrupted done reports preserve their completion time")
				}
			}
		}
	case "disposition":
		v, e := parseValue[Disposition](c)
		if e != nil {
			return e
		}
		if v.Target != c.ID || !member(v.Kind, "skipped", "cancelled", "reopened") {
			return fail("invalid disposition")
		}
		if v.ScheduleRevision != "" && !validHash(v.ScheduleRevision) {
			return fail("invalid schedule revision")
		}
		expected, revisions, _, err := scheduleRows(s, at)
		if err != nil {
			return err
		}
		_, scheduled := expected[v.Target]
		if latest(s, "item", v.Target) == nil && !scheduled {
			return fail("disposition target is absent")
		}
		if scheduled && v.ScheduleRevision != revisions[v.Target] {
			return fail("disposition must bind current schedule revision")
		}
		if !scheduled && v.ScheduleRevision != "" {
			return fail("disposition names absent schedule")
		}
		if v.Kind == "skipped" && latest(s, "item", v.Target) != nil {
			return fail("cannot skip admitted item")
		}
		if v.Kind == "reopened" {
			if old == nil {
				return fail("reopening requires prior disposition")
			}
			prior, _ := parseValue[Disposition](old.Change)
			if prior.Kind == "reopened" {
				return fail("already reopened")
			}
		}
	default:
		return fail("unsupported change kind")
	}
	return nil
}

func validateSnapshot(s Snapshot) error {
	if s.Schema != "agenda.snapshot/v1" || s.Records == nil || len(s.Records) > maxRecords {
		return fail("invalid snapshot")
	}
	prefix := Snapshot{Schema: s.Schema, Records: []Record{}}
	requests := map[string]bool{}
	total := 0
	for i, r := range s.Records {
		if r.Schema != "agenda.record/v1" || r.Sequence != i+1 || !samePtr(r.Previous, prefix.Head) || r.Revision != recordHash(r) {
			return fail("record chain mismatch")
		}
		at, e := instant(r.RecordedAt)
		if e != nil {
			return e
		}
		if i > 0 {
			prior, _ := instant(s.Records[i-1].RecordedAt)
			if at.Before(prior) {
				return fail("recording time moved backwards")
			}
		}
		if requests[r.Change.RequestID] {
			return fail("duplicate request ID in history")
		}
		requests[r.Change.RequestID] = true
		if e = validateChange(prefix, r.Change, at); e != nil {
			return e
		}
		if len(r.Data) != len(r.Change.Evidence) {
			return fail("incomplete retained evidence")
		}
		for _, evidence := range r.Change.Evidence {
			retained, exists := r.Data[evidence.SHA256]
			if !exists {
				return fail("selected evidence is absent from retained data")
			}
			raw, e := base64.StdEncoding.DecodeString(retained)
			if e != nil || digest(raw) != evidence.SHA256 {
				return fail("retained evidence changed")
			}
			total += len(raw)
			if total > maxEvidence {
				return fail("evidence exceeds aggregate bound")
			}
		}
		prefix.Records = append(prefix.Records, r)
		prefix.Head = ptr(r.Revision)
	}
	if !samePtr(s.Head, prefix.Head) {
		return fail("snapshot head mismatch")
	}
	return nil
}
func noSymlink(path string) error {
	p, e := filepath.Abs(path)
	if e != nil {
		return e
	}
	for {
		st, e := os.Lstat(p)
		if e != nil && !os.IsNotExist(e) {
			return e
		}
		if e == nil && st.Mode()&os.ModeSymlink != 0 {
			return fail("symlink paths are refused")
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return nil
}
func readFile(path string, n int64) ([]byte, error) {
	if e := noSymlink(path); e != nil {
		return nil, e
	}
	f, e := os.Open(path)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if !st.Mode().IsRegular() {
		return nil, fail("selected path is not a regular file")
	}
	return bounded(f, n)
}
func syncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func durableMkdir(path string) error {
	if e := noSymlink(path); e != nil {
		return e
	}
	if _, e := os.Stat(path); e == nil {
		return nil
	} else if !os.IsNotExist(e) {
		return e
	}
	parent := filepath.Dir(path)
	if parent == path {
		return fail("missing filesystem root")
	}
	if e := durableMkdir(parent); e != nil {
		return e
	}
	if e := os.Mkdir(path, 0700); e != nil && !os.IsExist(e) {
		return e
	}
	if e := syncDir(path); e != nil {
		return e
	}
	return syncDir(parent)
}
func loadSnapshot(path string) (Snapshot, []byte, error) {
	var s Snapshot
	raw, e := readFile(path, maxSnapshot)
	if e != nil {
		return s, nil, e
	}
	if e = strict(raw, &s); e != nil {
		return s, nil, e
	}
	e = validateSnapshot(s)
	return s, raw, e
}
func apply(root string, c Change, at time.Time) (Record, error) {
	var empty Record
	if !filepath.IsAbs(root) {
		return empty, fail("store root must be absolute")
	}
	if e := durableMkdir(root); e != nil {
		return empty, e
	}
	lockPath := filepath.Join(root, ".lock")
	if e := noSymlink(lockPath); e != nil {
		return empty, e
	}
	f, e := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return empty, e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); e != nil {
		return empty, e
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	path := filepath.Join(root, "snapshot.json")
	s, _, e := loadSnapshot(path)
	if os.IsNotExist(e) {
		s = Snapshot{Schema: "agenda.snapshot/v1", Records: []Record{}}
	} else if e != nil {
		return empty, e
	}
	for _, r := range s.Records {
		if r.Change.RequestID == c.RequestID {
			if bytes.Equal(encoded(r.Change), encoded(c)) {
				return r, nil
			}
			return empty, fail("request ID already names different bytes")
		}
	}
	if e = validateChange(s, c, at); e != nil {
		return empty, e
	}
	r := Record{Schema: "agenda.record/v1", Sequence: len(s.Records) + 1, Previous: s.Head, RecordedAt: stamp(at), Change: c, Data: map[string]string{}}
	for _, selected := range c.Evidence {
		raw, e := readFile(selected.Path, maxEvidence)
		if e != nil {
			return empty, e
		}
		if digest(raw) != selected.SHA256 {
			return empty, fail("selected evidence digest changed")
		}
		r.Data[selected.SHA256] = base64.StdEncoding.EncodeToString(raw)
	}
	r.Revision = recordHash(r)
	s.Records = append(s.Records, r)
	s.Head = ptr(r.Revision)
	if e = validateSnapshot(s); e != nil {
		return empty, e
	}
	raw := encoded(s)
	if len(raw) > maxSnapshot {
		return empty, fail("snapshot exceeds bound")
	}
	tmp, e := os.CreateTemp(root, ".prepare-")
	if e != nil {
		return empty, e
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, e = tmp.Write(raw); e == nil {
		e = tmp.Sync()
	}
	closeErr := tmp.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return empty, e
	}
	if e = os.Rename(name, path); e != nil {
		return empty, e
	}
	if e = syncDir(root); e != nil {
		return empty, e
	}
	return r, nil
}

func scheduleRows(s Snapshot, asof time.Time) (map[string]Obligation, map[string]string, []Finding, error) {
	rows := map[string]Obligation{}
	revisions := map[string]string{}
	findings := []Finding{}
	groups := map[string][]Record{}
	for _, r := range s.Records {
		if r.Change.Kind == "schedule" {
			groups[r.Change.ID] = append(groups[r.Change.ID], r)
		}
	}
	ids := []string{}
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		rs := groups[id]
		for i, r := range rs {
			v, _ := parseValue[Schedule](r.Change)
			end := "9999-12-31"
			if i+1 < len(rs) {
				next, _ := parseValue[Schedule](rs[i+1].Change)
				end = next.EffectiveFrom
				if v.EndDate < next.EffectiveFrom {
					d, _ := date(v.EndDate)
					if d.AddDate(0, 0, 1).Format("2006-01-02") < next.EffectiveFrom {
						findings = append(findings, Finding{id, "schedule-coverage-gap"})
					}
				}
			}
			occ, e := expand(v, id, r.Revision, asof)
			if e != nil {
				return nil, nil, nil, e
			}
			for _, o := range occ {
				if o.Date < v.EffectiveFrom || o.Date >= end {
					continue
				}
				if _, ok := rows[o.ID]; ok {
					return nil, nil, nil, fail("duplicate occurrence identity")
				}
				rows[o.ID] = Obligation{ID: o.ID, Revision: r.Revision, Title: v.Title, Owner: v.Owner, NotBefore: o.NotBefore, DueAt: o.DueAt, Timezone: o.Timezone, Basis: "external", CalendarID: ptr(id), Extensions: v.Extensions}
				revisions[o.ID] = r.Revision
			}
		}
		active := rs[0]
		for _, r := range rs {
			v, _ := parseValue[Schedule](r.Change)
			zone, _ := location(v.Timezone)
			if v.EffectiveFrom <= asof.In(zone).Format("2006-01-02") {
				active = r
			}
		}
		v, _ := parseValue[Schedule](active.Change)
		zone, _ := location(v.Timezone)
		day := asof.In(zone).Format("2006-01-02")
		if day > v.EndDate {
			findings = append(findings, Finding{id, "schedule-expired"})
		} else {
			end, _ := date(v.EndDate)
			today, _ := date(day)
			if end.Sub(today) <= 14*24*time.Hour {
				findings = append(findings, Finding{id, "schedule-horizon-approaching"})
			}
		}
	}
	if len(rows) > 20000 {
		return nil, nil, nil, fail("expected obligations exceed bound")
	}
	return rows, revisions, findings, nil
}

func project(s Snapshot, snapshotHash string, observations []Observation, asof time.Time) (Projection, error) {
	result := Projection{Schema: "agenda.projection/v1", AsOf: stamp(asof), SnapshotSHA256: snapshotHash, Coverage: "complete", Obligations: []Obligation{}, Attention: []Attention{}, Errors: []Finding{}, Notice: "Observations are caller-supplied; Agenda does not authenticate external acceptance."}
	if len(s.Records) == 0 {
		result.Coverage = "unverified"
		result.Errors = append(result.Errors, Finding{"", "empty-account"})
	}
	rows, scheduleRevs, findings, e := scheduleRows(s, asof)
	if e != nil {
		return result, e
	}
	result.Errors = append(result.Errors, findings...)
	items := map[string]Item{}
	for _, r := range s.Records {
		if r.Change.Kind != "item" {
			continue
		}
		if latest(s, "item", r.Change.ID).Revision != r.Revision {
			continue
		}
		v, _ := parseValue[Item](r.Change)
		items[r.Change.ID] = v
		row := Obligation{ID: r.Change.ID, Revision: r.Revision, Title: v.Title, Owner: v.Owner, NotBefore: v.NotBefore, DueAt: v.DueAt, Timezone: v.Timezone, Basis: v.Basis, Extensions: v.Extensions}
		if old, ok := rows[row.ID]; ok {
			row.CalendarID = old.CalendarID
			if !sameInstant(old.NotBefore, v.NotBefore) || !sameInstant(old.DueAt, v.DueAt) || old.Timezone != v.Timezone || v.Occurrence == nil || v.Occurrence.ScheduleRevision != scheduleRevs[row.ID] {
				row.Attention = append(row.Attention, "schedule-conflict")
			}
		} else if v.Occurrence != nil {
			row.CalendarID = ptr(v.Occurrence.ScheduleID)
			row.Attention = append(row.Attention, "outside-declared-schedule")
		}
		rows[row.ID] = row
	}
	// A changed schedule may stop proposing a previously waived occurrence.
	// Its recorded decision still belongs in the account as a visible conflict.
	for _, r := range s.Records {
		if r.Change.Kind != "disposition" || latest(s, "disposition", r.Change.ID).Revision != r.Revision {
			continue
		}
		if _, exists := rows[r.Change.ID]; exists {
			continue
		}
		d, _ := parseValue[Disposition](r.Change)
		for _, original := range s.Records {
			if original.Change.Kind != "schedule" || original.Revision != d.ScheduleRevision {
				continue
			}
			v, _ := parseValue[Schedule](original.Change)
			occurrences, err := expand(v, original.Change.ID, original.Revision, asof)
			if err != nil {
				return result, err
			}
			for _, occurrence := range occurrences {
				if occurrence.ID != r.Change.ID {
					continue
				}
				rows[r.Change.ID] = Obligation{ID: r.Change.ID, Revision: original.Revision, Title: v.Title, Owner: v.Owner, NotBefore: occurrence.NotBefore, DueAt: occurrence.DueAt, Timezone: v.Timezone, Basis: "external", CalendarID: ptr(original.Change.ID), Attention: []string{"outside-declared-schedule"}, Extensions: v.Extensions}
			}
		}
		if _, exists := rows[r.Change.ID]; !exists {
			return result, fail("disposition has no retained target declaration")
		}
	}
	obs := map[string]Observation{}
	for _, o := range observations {
		if !validID(o.ID) || !validHash(o.Revision) || !member(o.State, "unsubmitted", "queued", "running", "waiting", "cancelled", "unverified", "accepted", "rejected", "unknown") {
			return result, fail("invalid observation")
		}
		if _, ok := obs[o.ID]; ok {
			return result, fail("duplicate observation")
		}
		at, e := instant(o.ObservedAt)
		if e != nil {
			return result, e
		}
		if at.After(asof) {
			return result, fail("observation follows evaluation time")
		}
		if o.ValidUntil != "" {
			until, err := instant(o.ValidUntil)
			if err != nil || until.Before(at) {
				return result, fail("invalid observation valid_until")
			}
		}
		if len(o.Evidence) > 64 {
			return result, fail("too many evidence references")
		}
		for _, ref := range o.Evidence {
			if !validText(ref.Kind) || !validText(ref.Ref) {
				return result, fail("invalid evidence reference")
			}
		}
		if member(o.State, "accepted", "rejected", "unknown") && len(o.Evidence) == 0 {
			return result, fail("terminal observation requires evidence references")
		}
		if o.State == "accepted" {
			done, e := instant(o.CompletedAt)
			if e != nil || done.After(at) {
				return result, fail("invalid observed completion time")
			}
		} else if o.CompletedAt != "" {
			return result, fail("only acceptance has completion time")
		}
		if _, ok := rows[o.ID]; !ok {
			result.Errors = append(result.Errors, Finding{o.ID, "observation-without-obligation"})
			result.Coverage = "unverified"
		}
		obs[o.ID] = o
	}
	ids := []string{}
	for id := range rows {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		row := rows[id]
		if row.Attention == nil {
			row.Attention = []string{}
		}
		if row.Extensions == nil {
			row.Extensions = map[string]json.RawMessage{}
		}
		begin, _ := instant(row.NotBefore)
		due, _ := instant(row.DueAt)
		row.State = "missing"
		if begin.After(asof) {
			row.State = "upcoming"
		}
		row.Execution = "unobserved"
		row.Acceptance = "unconfirmed"
		row.Timeliness = "pending"
		if asof.After(due) {
			row.Timeliness = "overdue"
			row.Attention = append(row.Attention, "overdue")
		}
		if row.Basis == "human" {
			row.Execution = "human-reported"
			row.State = "active"
			var completed *string
			var report *Record
			prior := ""
			for i := range s.Records {
				r := &s.Records[i]
				if r.Change.Kind != "report" || r.Change.ID != id {
					continue
				}
				v, _ := parseValue[Report](r.Change)
				if v.State == "done" && prior != "done" {
					completed = ptr(r.RecordedAt)
					if v.CompletedAt != "" {
						completed = ptr(v.CompletedAt)
					}
				}
				if v.State != "done" {
					completed = nil
				}
				prior = v.State
				report = r
			}
			if report == nil {
				row.Attention = append(row.Attention, "missing-human-report")
			} else {
				v, _ := parseValue[Report](report.Change)
				row.ReportedState = v.State
				row.ReportRevision = report.Revision
				if v.State == "done" {
					row.State = "completed"
					row.Acceptance = "reported-done"
					row.CompletedAt = completed
				} else if v.State == "cancelled" {
					row.State = "cancelled"
				} else if v.State == "needs-attention" {
					row.Attention = append(row.Attention, "human-needs-attention")
				}
			}
			if o, ok := obs[id]; ok {
				row.Observation = &o
				expired := false
				if o.ValidUntil != "" {
					until, _ := instant(o.ValidUntil)
					expired = asof.After(until)
				}
				if o.Revision != row.Revision || expired {
					row.Attention = append(row.Attention, "stale-observation")
					result.Coverage = "unverified"
				} else if member(o.State, "unknown", "rejected", "waiting", "unverified", "cancelled") {
					row.Attention = append(row.Attention, "execution-"+o.State)
					if o.State == "unverified" {
						result.Coverage = "unverified"
					}
				}
			}
		} else if o, ok := obs[id]; ok {
			row.Observation = &o
			expired := false
			if o.ValidUntil != "" {
				until, _ := instant(o.ValidUntil)
				expired = asof.After(until)
			}
			if o.Revision != row.Revision || expired {
				row.Attention = append(row.Attention, "stale-observation")
				result.Coverage = "unverified"
			} else {
				row.Execution = o.State
				row.State = "active"
				if o.State == "accepted" {
					row.State = "completed"
					row.Acceptance = "accepted"
					row.CompletedAt = ptr(o.CompletedAt)
				} else if member(o.State, "unknown", "rejected", "waiting", "unverified", "cancelled") {
					row.Attention = append(row.Attention, "execution-"+o.State)
					if o.State == "unverified" {
						result.Coverage = "unverified"
					}
					if o.State == "cancelled" {
						row.State = "cancelled"
					}
				} else if o.State == "unsubmitted" && !begin.After(asof) {
					row.Attention = append(row.Attention, "not-submitted")
				}
			}
		} else if !begin.After(asof) {
			row.Attention = append(row.Attention, "missing-observation")
		}
		if row.CompletedAt != nil {
			done, _ := instant(*row.CompletedAt)
			if done.After(asof) {
				row.Attention = append(row.Attention, "completion-after-as-of")
				result.Coverage = "unverified"
			}
			row.Timeliness = "on-time"
			if done.After(due) {
				row.Timeliness = "late"
			}
			row.Attention = remove(row.Attention, "overdue")
		}
		if r := latest(s, "disposition", id); r != nil {
			d, _ := parseValue[Disposition](r.Change)
			if d.ScheduleRevision != "" && d.ScheduleRevision != scheduleRevs[id] {
				row.Attention = append(row.Attention, "disposition-schedule-conflict")
			} else if d.Kind != "reopened" {
				row.State = d.Kind
				if row.Execution != "unobserved" && row.Execution != "human-reported" {
					row.Attention = append(row.Attention, "disposition-execution-conflict")
				} else {
					row.Attention = remove(row.Attention, "overdue", "missing-observation")
				}
			}
		}
		if item, ok := items[id]; ok && item.Parent != "" {
			if _, ok := rows[item.Parent]; !ok {
				row.Attention = append(row.Attention, "missing-parent")
			}
		}
		sort.Strings(row.Attention)
		row.Attention = unique(row.Attention)
		if len(row.Attention) > 0 {
			result.Attention = append(result.Attention, Attention{id, row.Attention})
		}
		result.Obligations = append(result.Obligations, row)
	}
	for _, f := range result.Errors {
		result.Attention = append(result.Attention, Attention{f.ID, []string{f.Reason}})
	}
	return result, nil
}
func remove(items []string, names ...string) []string {
	result := []string{}
	for _, s := range items {
		if !member(s, names...) {
			result = append(result, s)
		}
	}
	return result
}
func unique(items []string) []string {
	r := []string{}
	for _, s := range items {
		if len(r) == 0 || r[len(r)-1] != s {
			r = append(r, s)
		}
	}
	return r
}

func run(args []string, in io.Reader, out, errout io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errout, "usage: agenda apply ROOT | export ROOT | expand --as-of TIME | project SNAPSHOT --as-of TIME")
		return 2
	}
	emit := func(v any) int {
		if _, e := out.Write(encoded(v)); e != nil {
			fmt.Fprintln(errout, "agenda:", e)
			return 2
		}
		return 0
	}
	bad := func(e error) int {
		fmt.Fprintln(errout, "agenda:", e)
		var path *os.PathError
		if errors.As(e, &path) {
			return 2
		}
		return 1
	}
	switch args[0] {
	case "version", "--version":
		fmt.Fprintln(out, version)
		return 0
	case "help", "--help":
		fmt.Fprintln(out, "agenda apply ROOT < change.json\nagenda export ROOT\nagenda expand --as-of TIME < schedule.json\nagenda project SNAPSHOT --as-of TIME < observations.jsonl\nagenda version")
		return 0
	case "apply":
		if len(args) != 2 {
			return 2
		}
		raw, e := bounded(in, maxJSON)
		if e != nil {
			return bad(e)
		}
		var c Change
		if e = strict(raw, &c); e != nil {
			return bad(e)
		}
		r, e := apply(args[1], c, time.Now())
		if e != nil {
			return bad(e)
		}
		return emit(r)
	case "export":
		if len(args) != 2 {
			return 2
		}
		s, _, e := loadSnapshot(filepath.Join(args[1], "snapshot.json"))
		if e != nil {
			return bad(e)
		}
		return emit(s)
	case "expand", "project":
		fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
		fs.SetOutput(errout)
		asof := fs.String("as-of", "", "explicit evaluation timestamp")
		rest := args[1:]
		path := ""
		if args[0] == "project" {
			if len(rest) == 0 {
				return 2
			}
			path = rest[0]
			rest = rest[1:]
		}
		if e := fs.Parse(rest); e != nil {
			return 2
		}
		if fs.NArg() != 0 || *asof == "" {
			return 2
		}
		at, e := instant(*asof)
		if e != nil {
			return bad(e)
		}
		if args[0] == "expand" {
			raw, e := bounded(in, maxJSON)
			if e != nil {
				return bad(e)
			}
			var s Schedule
			if e = strict(raw, &s); e != nil {
				return bad(e)
			}
			rows, e := expand(s, s.ID, digest(encoded(s)), at)
			if e != nil {
				return bad(e)
			}
			var b bytes.Buffer
			for _, r := range rows {
				b.Write(encoded(r))
			}
			if _, e = out.Write(b.Bytes()); e != nil {
				return bad(e)
			}
			return 0
		}
		s, raw, e := loadSnapshot(path)
		if e != nil {
			return bad(e)
		}
		input, e := bounded(in, 16<<20)
		if e != nil {
			return bad(e)
		}
		observations := []Observation{}
		if len(input) > 0 {
			lines := bytes.Split(input, []byte{'\n'})
			if len(lines[len(lines)-1]) == 0 {
				lines = lines[:len(lines)-1]
			}
			if len(lines) > 10000 {
				return bad(fail("too many observations"))
			}
			for _, line := range lines {
				if len(line) > maxJSON {
					return bad(fail("observation exceeds bound"))
				}
				var o Observation
				if e = strict(line, &o); e != nil {
					return bad(e)
				}
				observations = append(observations, o)
			}
		}
		p, e := project(s, digest(raw), observations, at)
		if e != nil {
			return bad(e)
		}
		return emit(p)
	default:
		fmt.Fprintln(errout, "agenda: unknown command")
		return 2
	}
}
func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
