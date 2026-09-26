package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDeliveryManifestCannotSelectUnreturnedFiles(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, filepath.Join(dir, "delivery.json"), `{"title":"An unsafe instruction","artifacts":[{"id":"one","title":"Secret","original":"../outside.txt"}]}`, 0600)
	result := taskResult{Title: "Your work", Artifacts: []taskArtifact{{Name: "note.md"}}}
	spec, err := makeDeliverySpec(dir, result)
	if err != nil || len(spec.Artifacts) != 1 || spec.Artifacts[0].Original != "note.md" {
		t.Fatalf("bad fallback: %+v %v", spec, err)
	}
}

// This opt-in check composes the real public Plonk executable with fresh local
// data and synthetic workers. It never reads personal credentials or calls a model.
func TestDeliveryPublicExecutableIntegration(t *testing.T) {
	bin := os.Getenv("BENCH_UI_PLONK_BIN")
	if bin == "" {
		t.Skip("select the built Plonk executable")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	listener.Close()
	data := t.TempDir()
	cmd := exec.Command(bin, "serve", "-addr", addr, "-base-domain", addr, "-data", data)
	var logs limitedDeliveryBuffer
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Signal(os.Interrupt); cmd.Wait() })
	server := "http://" + addr
	tokenFile := filepath.Join(data, "admin.token")
	ready := false
	for end := time.Now().Add(8 * time.Second); time.Now().Before(end); {
		c := http.Client{Timeout: 100 * time.Millisecond}
		resp, e := c.Get(server + "/healthz")
		if e == nil {
			resp.Body.Close()
			ready = resp.StatusCode == 200
			if ready {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("local Plonk did not start")
	}
	a := taskFixture(t)
	a.cfg.Plonk = bin
	a.cfg.PlonkURL = server
	a.cfg.PlonkTokenFile = tokenFile
	first := taskSubmit(t, a, "", "Write the first welcome note")
	deliver := func(turn taskTurn, action string) deliveryState {
		t.Helper()
		f := formFor(a)
		f.Set("action", action)
		w := serveTest(a, "POST", "/work/jobs/"+turn.Job.ID+"/delivery", f)
		if w.Code != 303 {
			t.Fatalf("delivery admission: %d %s", w.Code, w.Body.String())
		}
		var job Job
		for _, j := range a.jobs.List() {
			if j.Kind == "delivery" {
				job = j
				break
			}
		}
		job = awaitJob(t, a.jobs, job.ID)
		if job.ExitCode == nil || *job.ExitCode != 0 {
			t.Fatalf("delivery failed: %+v\n%s", job, a.jobs.Log(job.ID, "stderr"))
		}
		return a.deliveryState(turn.Record.Thread)
	}
	prepared := deliver(first, "prepare")
	if prepared.ShareID != "" {
		t.Fatal("preview created a share")
	}
	gallery := serveTest(a, "GET", "/work/jobs/"+first.Job.ID+"/delivery/", nil)
	if gallery.Code != 200 || !strings.Contains(gallery.Body.String(), "/work/jobs/"+first.Job.ID+"/delivery/a/") {
		t.Fatalf("private gallery broken: %d %s", gallery.Code, gallery.Body.String())
	}
	original := serveTest(a, "GET", "/work/jobs/"+first.Job.ID+"/delivery/files/result.md?download=1", nil)
	if original.Code != 200 || !strings.Contains(original.Body.String(), "first welcome") {
		t.Fatalf("private file broken: %d %s", original.Code, original.Body.String())
	}
	token, _ := os.ReadFile(tokenFile)
	if strings.Contains(gallery.Body.String(), strings.TrimSpace(string(token))) {
		t.Fatal("token exposed")
	}
	shared := deliver(first, "share")
	if shared.URL == "" {
		t.Fatal("no share link")
	}
	get := func(u string) (int, string) {
		t.Helper()
		resp, e := http.Get(u)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	if st, _ := get(server + "/" + shared.Slug + "/result.md"); st != 404 {
		t.Fatal("private bytes exposed by legacy route")
	}
	second := taskSubmit(t, a, first.Record.Thread, "Write the second welcome note")
	deliver(second, "prepare")
	if _, body := get(shared.URL + "files/result.md"); strings.Contains(body, "second welcome") {
		t.Fatal("draft changed share")
	}
	updated := deliver(second, "share")
	if updated.URL != shared.URL {
		t.Fatal("update lost stable link")
	}
	if _, body := get(shared.URL + "files/result.md"); !strings.Contains(body, "second welcome") {
		t.Fatal("shared update not visible")
	}
	// A failed earlier update must not override a later successful revocation.
	failedDir, err := a.workspace()
	if err != nil {
		t.Fatal(err)
	}
	failedRecord := deliveryRecord{Thread: second.Record.Thread, TaskID: second.Job.ID, Action: "share", Before: updated}
	if err := saveTaskJSON(filepath.Join(filepath.Dir(failedDir), "delivery-request.json"), failedRecord); err != nil {
		t.Fatal(err)
	}
	failedJob, err := a.jobs.Start("delivery", "Synthetic lost response", failedDir, []string{"/usr/bin/false"}, "")
	if err != nil {
		t.Fatal(err)
	}
	awaitJob(t, a.jobs, failedJob.ID)
	// Hosted access can still be revoked after a local original disappears.
	localOriginal := filepath.Join(filepath.Dir(second.Job.Dir), "deliverables", "result.md")
	originalBytes, err := os.ReadFile(localOriginal)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(localOriginal); err != nil {
		t.Fatal(err)
	}
	deliver(second, "revoke")
	if err := os.WriteFile(localOriginal, originalBytes, 0600); err != nil {
		t.Fatal(err)
	}
	if st, _ := get(shared.URL + "files/result.md"); st != 404 {
		t.Fatal("revoked file visible")
	}
	again := deliver(second, "share")
	if again.URL == shared.URL || again.Revoked {
		t.Fatal("reshare revived old capability")
	}
	if st, _ := get(again.URL); st != 200 {
		t.Fatal(fmt.Sprintf("new link status %d", st))
	}
}

func TestDeliveryConnectionSurvivesWorkingDirectoryChange(t *testing.T) {
	dir := t.TempDir()
	token := filepath.Join(dir, "connection")
	writeFixture(t, token, "synthetic-token", 0600)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, token)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := prepare(config{Source: fixtureRepository(t), Data: filepath.Join(t.TempDir(), "private"), Hire: os.Args[0], Python: os.Args[0], Plonk: os.Args[0], PlonkURL: "http://localhost:8790", PlonkTokenFile: relative})
	if err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(token)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PlonkTokenFile != physical {
		t.Fatalf("connection path not stable: %q", cfg.PlonkTokenFile)
	}
}
