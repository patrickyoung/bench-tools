package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testDir(t *testing.T) string {
	t.Helper()
	p, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func when(s string) time.Time {
	t, err := instant(s)
	if err != nil {
		panic(err)
	}
	return t
}

var testNow = when("2026-01-01T12:00:00Z")

func item(basis string) Item {
	return Item{Title: "Review report", Owner: "Morgan", NotBefore: "2026-01-01T09:00:00Z", DueAt: "2026-01-01T17:00:00Z", Timezone: "UTC", Basis: basis}
}
func change(kind, id, request string, value any, previous *string) Change {
	return Change{Schema: "agenda.change/v1", RequestID: request, Kind: kind, ID: id, Previous: previous, By: "Morgan", Reason: "Explicit example change", Value: encoded(value)}
}
func mustApply(t *testing.T, root string, c Change, at time.Time) Record {
	t.Helper()
	r, err := apply(root, c, at)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func snapshot(t *testing.T, root string) Snapshot {
	t.Helper()
	s, _, err := loadSnapshot(filepath.Join(root, "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func schedule() Schedule {
	return Schedule{ID: "daily", Title: "Daily check", Owner: "Morgan", Timezone: "UTC", StartDate: "2026-01-01", EndDate: "2026-01-05", Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, ExcludedDates: []string{}, StartTime: "09:00", DueTime: "17:00", Missed: "all", EffectiveFrom: "2026-01-01", CheckEverySeconds: 3600}
}
func evidence(t *testing.T) Evidence {
	t.Helper()
	path := filepath.Join(testDir(t), "evidence")
	raw := []byte("Reviewed by the named operator.\n")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return Evidence{Path: path, SHA256: digest(raw)}
}
func projection(t *testing.T, s Snapshot, obs []Observation, at string) Projection {
	t.Helper()
	p, err := project(s, digest(encoded(s)), obs, when(at))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStrictJSON(t *testing.T) {
	for _, raw := range []string{`{"x":1,"x":2}`, `{"x":{"a":1,"a":2}}`, `{"x":"\ud800"}`, `{"x":"\udc00"}`, `{"x":1e999}`, `{} {}`, `{"x":NaN}`, string([]byte{'"', 0xff, '"'}), strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34)} {
		var value any
		if err := strict([]byte(raw), &value); err == nil {
			t.Errorf("accepted malformed JSON %q", raw)
		}
	}
	var value any
	if err := strict([]byte(`{"x":"\ud83d\ude00","y":"\\ud800"}`), &value); err != nil {
		t.Fatal(err)
	}
	var c Change
	if err := strict([]byte(`{"bogus":true}`), &c); err == nil {
		t.Fatal("accepted unknown field")
	}
}

func TestExplicitPortableTime(t *testing.T) {
	for _, s := range []string{"2026-01-01T12:00:00+24:00", "2026-01-01T12:00:00+00:60", "2026-01-01T12:00:00,1Z"} {
		if _, err := instant(s); err == nil {
			t.Fatal("accepted non-RFC3339 timestamp", s)
		}
	}
	for _, zone := range []string{"", "Local"} {
		s := schedule()
		s.Timezone = zone
		if _, err := expand(s, s.ID, "", testNow); err == nil {
			t.Fatal("accepted implicit machine timezone")
		}
	}
	s := schedule()
	s.StartTime = "9:00"
	if _, err := expand(s, s.ID, "", testNow); err == nil {
		t.Fatal("accepted noncanonical local time")
	}
}

func TestDurableHistoryIdempotencyAndConflict(t *testing.T) {
	root := filepath.Join(testDir(t), "nested", "store")
	c := change("item", "review", "first", item("external"), nil)
	r := mustApply(t, root, c, testNow)
	same := mustApply(t, root, c, testNow.Add(time.Hour))
	if same.Revision != r.Revision {
		t.Fatal("duplicate changed record")
	}
	c.Reason = "conflicting reuse"
	if _, err := apply(root, c, testNow); err == nil {
		t.Fatal("request reuse succeeded")
	}
	c.RequestID = "second"
	if _, err := apply(root, c, testNow); err == nil {
		t.Fatal("missing expected revision succeeded")
	}
	c.Previous = &r.Revision
	next := mustApply(t, root, c, testNow.Add(time.Minute))
	s := snapshot(t, root)
	if len(s.Records) != 2 || *s.Head != next.Revision || s.Records[0].Revision != r.Revision {
		t.Fatal("history not retained")
	}
	p := projection(t, s, nil, "2026-01-02T00:00:00Z")
	if len(p.Obligations) != 1 || member("schedule-conflict", p.Obligations[0].Attention...) {
		t.Fatal("revised ordinary item became schedule conflict")
	}
}

func TestConcurrentIdenticalAndCompetingWrites(t *testing.T) {
	root := filepath.Join(testDir(t), "store")
	c := change("item", "review", "first", item("external"), nil)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	revs := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := apply(root, c, testNow); errs <- e; revs <- r.Revision }()
	}
	wg.Wait()
	close(errs)
	close(revs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	s := snapshot(t, root)
	if len(s.Records) != 1 {
		t.Fatal("duplicate committed records")
	}
	for rev := range revs {
		if rev != *s.Head {
			t.Fatal("inconsistent duplicate reply")
		}
	}
	first := *s.Head
	successes := make(chan bool, 2)
	for _, id := range []string{"second", "third"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := apply(root, change("item", "review", id, item("external"), &first), testNow.Add(time.Minute))
			successes <- err == nil
		}(id)
	}
	wg.Wait()
	close(successes)
	count := 0
	for ok := range successes {
		if ok {
			count++
		}
	}
	if count != 1 || len(snapshot(t, root).Records) != 2 {
		t.Fatal("optimistic updates did not conflict")
	}
}

func TestHumanCompletionIsSeparateAndRetainsEvidence(t *testing.T) {
	root := testDir(t)
	parent := mustApply(t, root, change("item", "parent", "p", item("external"), nil), testNow)
	h := item("human")
	h.Parent = "parent"
	mustApply(t, root, change("item", "review", "h", h, nil), testNow)
	ev := evidence(t)
	c := change("report", "review", "done", Report{Target: "review", State: "done", CompletedAt: "2026-01-01T11:00:00Z"}, nil)
	c.Evidence = []Evidence{ev}
	r := mustApply(t, root, c, testNow)
	if err := os.Remove(ev.Path); err != nil {
		t.Fatal(err)
	}
	// A lost reply succeeds without needing original evidence files again.
	if got := mustApply(t, root, c, testNow.Add(time.Hour)); got.Revision != r.Revision {
		t.Fatal("lost reply changed identity")
	}
	s := snapshot(t, root)
	p := projection(t, s, nil, "2026-01-01T18:00:00Z")
	if p.Obligations[0].ID != "parent" || p.Obligations[0].Acceptance != "unconfirmed" || p.Obligations[1].Acceptance != "reported-done" || *p.Obligations[1].CompletedAt != "2026-01-01T11:00:00Z" {
		t.Fatalf("wrong completion: %+v", p.Obligations)
	}
	invalid := change("report", "parent", "invalid", Report{Target: "parent", State: "done"}, nil)
	invalid.Evidence = []Evidence{ev}
	if _, err := apply(root, invalid, testNow); err == nil {
		t.Fatal("human report accepted external parent")
	}
	obs := Observation{ID: "parent", Revision: parent.Revision, State: "accepted", ObservedAt: "2026-01-01T18:00:00Z", CompletedAt: "2026-01-01T16:00:00Z", Evidence: []Reference{{"checker", "result:1"}}}
	p = projection(t, s, []Observation{obs}, "2026-01-01T18:00:00Z")
	if p.Obligations[0].Acceptance != "accepted" || p.Obligations[1].Acceptance != "reported-done" {
		t.Fatal("completion authorities merged")
	}
}

func TestUninterruptedDoneTimeAndReopen(t *testing.T) {
	root := testDir(t)
	mustApply(t, root, change("item", "review", "item", item("human"), nil), testNow)
	ev := evidence(t)
	c := change("report", "review", "done", Report{Target: "review", State: "done"}, nil)
	c.Evidence = []Evidence{ev}
	first := mustApply(t, root, c, testNow)
	c.RequestID = "edit"
	c.Previous = &first.Revision
	c.Reason = "Changed attribution reason"
	second := mustApply(t, root, c, testNow.Add(time.Hour))
	p := projection(t, snapshot(t, root), nil, "2026-01-01T15:00:00Z")
	if *p.Obligations[0].CompletedAt != stamp(testNow) {
		t.Fatal("done edit reset completion")
	}
	c = change("report", "review", "reopen", Report{Target: "review", State: "ready"}, &second.Revision)
	third := mustApply(t, root, c, testNow.Add(2*time.Hour))
	c = change("report", "review", "redone", Report{Target: "review", State: "done"}, &third.Revision)
	c.Evidence = []Evidence{ev}
	mustApply(t, root, c, testNow.Add(3*time.Hour))
	p = projection(t, snapshot(t, root), nil, "2026-01-01T16:00:00Z")
	if *p.Obligations[0].CompletedAt != stamp(testNow.Add(3*time.Hour)) {
		t.Fatal("reopen failed to reset completion")
	}
}

func TestExpansionNeverErasesMissedOccurrences(t *testing.T) {
	s := schedule()
	s.Missed = "skip"
	rows, err := expand(s, s.ID, "revision", when("2026-01-05T12:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || rows[0].Eligible || !rows[0].Overdue || !rows[4].Eligible {
		t.Fatal("missed work erased or selected")
	}
	s.Missed = "latest"
	rows, err = expand(s, s.ID, "revision", when("2026-01-05T12:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	for i, r := range rows {
		if r.Eligible != (i == 4) {
			t.Fatal("latest selected wrong date")
		}
	}
	for _, d := range []string{"2026-03-08", "2026-11-01"} {
		s = schedule()
		s.Timezone = "America/New_York"
		s.StartDate = d
		s.EndDate = d
		s.EffectiveFrom = d
		s.StartTime = "02:30"
		if strings.Contains(d, "11-") {
			s.StartTime = "01:30"
		}
		if _, err = expand(s, s.ID, "", testNow); err == nil {
			t.Fatal("accepted DST ambiguity/nonexistence")
		}
	}
}

func TestScheduleRevisionsAndDispositionsRemainAccountable(t *testing.T) {
	root := testDir(t)
	v := schedule()
	r := mustApply(t, root, change("schedule", "daily", "initial", v, nil), testNow)
	c := change("disposition", "daily/2026-01-04", "skip", Disposition{Target: "daily/2026-01-04", Kind: "skipped", ScheduleRevision: r.Revision}, nil)
	mustApply(t, root, c, testNow)
	v.EffectiveFrom = "2026-01-03"
	v.ExcludedDates = []string{"2026-01-04"}
	mustApply(t, root, change("schedule", "daily", "revision", v, &r.Revision), testNow.Add(time.Minute))
	p := projection(t, snapshot(t, root), nil, "2026-01-05T18:00:00Z")
	if len(p.Obligations) != 5 {
		t.Fatal("old waived occurrence disappeared")
	}
	var conflict bool
	for _, row := range p.Obligations {
		if row.ID == "daily/2026-01-04" {
			conflict = member("disposition-schedule-conflict", row.Attention...)
		}
	}
	if !conflict {
		t.Fatal("changed waiver not visible")
	}
}

func TestScheduleGapAndOccurrenceAdmissionConflict(t *testing.T) {
	root := testDir(t)
	v := schedule()
	v.EndDate = "2026-01-02"
	r := mustApply(t, root, change("schedule", "daily", "initial", v, nil), testNow)
	i := item("external")
	i.NotBefore = "2026-01-01T04:00:00-05:00"
	i.DueAt = "2026-01-01T12:00:00-05:00"
	i.Occurrence = &OccurrenceBinding{ScheduleID: "daily", Date: "2026-01-01", ScheduleRevision: r.Revision}
	mustApply(t, root, change("item", "daily/2026-01-01", "admit", i, nil), testNow)
	v.StartDate = "2026-01-04"
	v.EndDate = "2026-01-05"
	v.EffectiveFrom = "2026-01-04"
	mustApply(t, root, change("schedule", "daily", "revise", v, &r.Revision), testNow.Add(time.Minute))
	p := projection(t, snapshot(t, root), nil, "2026-01-05T12:00:00Z")
	if len(p.Obligations) != 4 || len(p.Errors) == 0 || p.Errors[0].Reason != "schedule-coverage-gap" {
		t.Fatal("gap concealed")
	}
	if member("schedule-conflict", p.Obligations[0].Attention...) {
		t.Fatal("matching admission conflicts")
	}
}

func TestUnknownStaleAndLateObservations(t *testing.T) {
	root := testDir(t)
	r := mustApply(t, root, change("item", "review", "item", item("external"), nil), testNow)
	s := snapshot(t, root)
	base := Observation{ID: "review", Revision: r.Revision, State: "unknown", ObservedAt: "2026-01-01T18:00:00Z", Evidence: []Reference{{"receipt", "unknown:1"}}}
	p := projection(t, s, []Observation{base}, "2026-01-01T18:00:00Z")
	if p.Obligations[0].Execution != "unknown" || p.Obligations[0].Acceptance != "unconfirmed" {
		t.Fatal("unknown became accepted")
	}
	base.State = "accepted"
	base.CompletedAt = "2026-01-01T18:00:00Z"
	p = projection(t, s, []Observation{base}, "2026-01-01T18:00:00Z")
	if p.Obligations[0].Timeliness != "late" {
		t.Fatal("late acceptance hidden")
	}
	base.Revision = strings.Repeat("a", 64)
	p = projection(t, s, []Observation{base}, "2026-01-01T18:00:00Z")
	if p.Coverage != "unverified" || p.Obligations[0].Acceptance != "unconfirmed" {
		t.Fatal("stale binding accepted")
	}
	base.Revision = r.Revision
	base.ValidUntil = "2026-01-01T18:00:00Z"
	p = projection(t, s, []Observation{base}, "2026-01-01T18:00:01Z")
	if p.Coverage != "unverified" {
		t.Fatal("expired provenance accepted")
	}
}

func TestLinkedObservationDoesNotCompleteHumanReport(t *testing.T) {
	root := testDir(t)
	r := mustApply(t, root, change("item", "review", "item", item("human"), nil), testNow)
	report := mustApply(t, root, change("report", "review", "ready", Report{Target: "review", State: "ready"}, nil), testNow)
	o := Observation{ID: "review", Revision: r.Revision, State: "waiting", ObservedAt: stamp(testNow), Evidence: []Reference{}}
	p := projection(t, snapshot(t, root), []Observation{o}, stamp(testNow))
	row := p.Obligations[0]
	if row.Observation == nil || row.ReportedState != "ready" || row.ReportRevision != report.Revision || row.Execution != "human-reported" || row.Acceptance != "unconfirmed" || !member("execution-waiting", row.Attention...) {
		t.Fatal("lost linked wait/report distinctions")
	}
	o.State = "accepted"
	o.CompletedAt = stamp(testNow)
	o.Evidence = []Reference{{"receipt", "controller:1"}}
	p = projection(t, snapshot(t, root), []Observation{o}, stamp(testNow))
	if p.Obligations[0].Acceptance != "unconfirmed" || p.Obligations[0].ReportedState != "ready" {
		t.Fatal("linked job completed human report")
	}
	o.ValidUntil = stamp(testNow)
	p = projection(t, snapshot(t, root), []Observation{o}, stamp(testNow.Add(time.Second)))
	if p.Coverage != "unverified" || p.Obligations[0].Observation == nil {
		t.Fatal("stale human observation not visible")
	}
}

func TestEmptyAccountIsNotHealthy(t *testing.T) {
	p := projection(t, Snapshot{Schema: "agenda.snapshot/v1", Records: []Record{}}, nil, stamp(testNow))
	if p.Coverage != "unverified" || len(p.Errors) != 1 || p.Errors[0].Reason != "empty-account" {
		t.Fatal("empty account appeared healthy")
	}
}

func TestCorruptionAndIncompleteHistoryFailBeforeOutput(t *testing.T) {
	root := testDir(t)
	mustApply(t, root, change("item", "review", "item", item("human"), nil), testNow)
	ev := evidence(t)
	c := change("report", "review", "report", Report{Target: "review", State: "done"}, nil)
	c.Evidence = []Evidence{ev}
	mustApply(t, root, c, testNow)
	s := snapshot(t, root)
	s.Records[1].Data[ev.SHA256] = "eA=="
	path := filepath.Join(root, "corrupt.json")
	os.WriteFile(path, encoded(s), 0600)
	var out, diag bytes.Buffer
	if code := run([]string{"project", path, "--as-of", "2026-01-01T18:00:00Z"}, strings.NewReader(""), &out, &diag); code == 0 || out.Len() != 0 {
		t.Fatal("corrupt evidence yielded projection")
	}
	s = snapshot(t, root)
	s.Records = s.Records[:1]
	if err := validateSnapshot(s); err == nil {
		t.Fatal("missing tail accepted against original head")
	}
}

func TestEmptyEvidenceStillRequiresRetainedIdentity(t *testing.T) {
	root := testDir(t)
	mustApply(t, root, change("item", "review", "item", item("human"), nil), testNow)
	path := filepath.Join(testDir(t), "empty")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	c := change("report", "review", "report", Report{Target: "review", State: "done"}, nil)
	c.Evidence = []Evidence{{Path: path, SHA256: digest(nil)}}
	mustApply(t, root, c, testNow)
	s := snapshot(t, root)
	s.Records[1].Data = map[string]string{"wrong-key": ""}
	s.Records[1].Revision = recordHash(s.Records[1])
	s.Head = &s.Records[1].Revision
	if err := validateSnapshot(s); err == nil {
		t.Fatal("missing empty evidence identity was accepted")
	}
}

func TestExportDoesNotNeedOriginalFilesAndDoesNotWrite(t *testing.T) {
	root := testDir(t)
	mustApply(t, root, change("item", "review", "item", item("human"), nil), testNow)
	ev := evidence(t)
	c := change("report", "review", "report", Report{Target: "review", State: "done"}, nil)
	c.Evidence = []Evidence{ev}
	mustApply(t, root, c, testNow)
	os.Remove(ev.Path)
	before, _ := os.ReadFile(filepath.Join(root, "snapshot.json"))
	entries, _ := os.ReadDir(root)
	var out, diag bytes.Buffer
	if code := run([]string{"export", root}, strings.NewReader(""), &out, &diag); code != 0 {
		t.Fatal(code, diag.String())
	}
	after, _ := os.ReadFile(filepath.Join(root, "snapshot.json"))
	afterEntries, _ := os.ReadDir(root)
	if !bytes.Equal(before, after) || len(entries) != len(afterEntries) {
		t.Fatal("export modified store")
	}
	var s Snapshot
	if err := strict(out.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if err := validateSnapshot(s); err != nil {
		t.Fatal(err)
	}
}

func TestStandaloneExecutable(t *testing.T) {
	if testing.Short() {
		t.Skip("standalone source build")
	}
	dir := testDir(t)
	for _, name := range []string{"go.mod", "main.go"} {
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("go", "build", "-trimpath", "-buildvcs=false", "-o", filepath.Join(dir, "agenda"), ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	cmd = exec.Command(filepath.Join(dir, "agenda"), "expand", "--as-of", "2026-01-03T12:00:00Z")
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(encoded(schedule()))
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes.Split(bytes.TrimSpace(out), []byte{'\n'})) != 5 {
		t.Fatal("standalone lost finite occurrences")
	}
	var first Occurrence
	if err = json.Unmarshal(bytes.Split(out, []byte{'\n'})[0], &first); err != nil || first.ID != "daily/2026-01-01" {
		t.Fatal("bad standalone stream")
	}
	// Public commands compose without another Bench executable or source tree.
	root := filepath.Join(dir, "account")
	c := change("item", "review", "human-only", item("human"), nil)
	cmd = exec.Command(filepath.Join(dir, "agenda"), "apply", root)
	cmd.Stdin = bytes.NewReader(encoded(c))
	if out, err = cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	cmd = exec.Command(filepath.Join(dir, "agenda"), "export", root)
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	selected := filepath.Join(dir, "selected.json")
	if err = os.WriteFile(selected, out, 0600); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(filepath.Join(dir, "agenda"), "project", selected, "--as-of", "2026-01-02T00:00:00Z")
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var p Projection
	if err = json.Unmarshal(out, &p); err != nil || len(p.Obligations) != 1 || p.Obligations[0].Acceptance != "unconfirmed" || !member("missing-human-report", p.Obligations[0].Attention...) {
		t.Fatal("public human-only projection incorrect")
	}
}
