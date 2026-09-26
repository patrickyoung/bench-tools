package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type uploadFixture struct {
	name string
	data []byte
}

func uploadTaskRequest(t *testing.T, path string, form url.Values, files ...uploadFixture) *http.Request {
	t.Helper()
	var body bytes.Buffer
	m := multipart.NewWriter(&body)
	for key, values := range form {
		for _, value := range values {
			if err := m.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, file := range files {
		part, err := m.CreateFormFile("attachments", file.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(file.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "http://127.0.0.1:8787"+path, &body)
	r.Header.Set("Content-Type", m.FormDataContentType())
	return r
}

func uploadTask(t *testing.T, a *app, path string, form url.Values, files ...uploadFixture) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	a.handler().ServeHTTP(w, uploadTaskRequest(t, path, form, files...))
	return w
}

func uploadedTaskTurn(t *testing.T, a *app, w *httptest.ResponseRecorder) taskTurn {
	t.Helper()
	if w.Code != 303 {
		t.Fatalf("upload failed: %d %s", w.Code, w.Body.String())
	}
	thread := strings.TrimPrefix(w.Header().Get("Location"), "/work/")
	turns := a.taskTurns(thread)
	if len(turns) == 0 {
		t.Fatal("upload did not create conversation")
	}
	turn := turns[len(turns)-1]
	turn.Job = awaitJob(t, a.jobs, turn.Job.ID)
	turn.Result = a.taskResult(turn.Job)
	return turn
}

func TestTaskUploadsPreserveBytesAndReachPlannerWorkerAndRefinement(t *testing.T) {
	a := taskFixture(t)
	content := []byte("reference\x00\xff\noriginal bytes")
	raw, err := os.ReadFile(a.cfg.Agent)
	if err != nil {
		t.Fatal(err)
	}
	needle := "assert Path(os.environ['DOCUMENT_INPUT']).parent != Path.cwd()"
	if !strings.Contains(string(raw), needle) {
		t.Fatal("fixture execution seam changed")
	}
	script := strings.Replace(string(raw), needle, `selected=[Path(sys.argv[i+1]) for i,arg in enumerate(sys.argv[:-1]) if arg=='-record-input']
assert any(p.read_bytes()==b'reference\x00\xff\noriginal bytes' for p in selected)
assert all(not p.is_relative_to(Path.cwd()) for p in selected)
`+needle, 1)
	writeFixture(t, a.cfg.Agent, script, 0700)
	f := formFor(a)
	f.Set("message", "Use the manual")
	first := uploadedTaskTurn(t, a, uploadTask(t, a, "/work", f, uploadFixture{"Manual 'quoted'.pdf", content}))
	if first.Job.State != "completed" || len(first.Record.Attachments) != 1 {
		t.Fatalf("worker missed reference: %+v %s", first, a.jobs.Log(first.Job.ID, "stderr"))
	}
	file := first.Record.Attachments[0]
	if file.Size != int64(len(content)) || file.SHA256 != digestText(content) || within(filepath.Dir(first.Job.Dir), file.Path) {
		t.Fatal("attachment metadata or authority changed")
	}
	if info, err := os.Stat(file.Path); err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("original is not private", err)
	}
	download := serveTest(a, "GET", "/work/attachments/"+first.Record.Thread+"/"+file.ID, nil)
	_, params, err := mime.ParseMediaType(download.Header().Get("Content-Disposition"))
	if err != nil || download.Code != 200 || !bytes.Equal(download.Body.Bytes(), content) || params["filename"] != file.Name || !strings.HasPrefix(download.Header().Get("Content-Disposition"), "attachment") {
		t.Fatal("download changed bytes or safe disposition", download.Code, err)
	}
	if got := serveTest(a, "GET", "/work/attachments/"+randomID()+"/"+file.ID, nil); got.Code != 404 {
		t.Fatal("attachment was exposed under a different conversation")
	}
	for _, turn := range []taskTurn{first, taskSubmit(t, a, first.Record.Thread, "Refine the same document")} {
		if turn.Job.State != "completed" || !reflect.DeepEqual(turn.Record.Attachments, first.Record.Attachments) {
			t.Fatalf("refinement lost attachment: %+v", turn)
		}
		root := filepath.Dir(turn.Job.Dir)
		request, err := os.ReadFile(filepath.Join(root, "plan", "request.json"))
		if err != nil {
			t.Fatal(err)
		}
		var plan struct{ Attachments []taskAttachment }
		if json.Unmarshal(request, &plan) != nil || !reflect.DeepEqual(plan.Attachments, first.Record.Attachments) {
			t.Fatal("planner missed controller-selected attachments")
		}
		goal, err := os.ReadFile(filepath.Join(root, "execution.txt"))
		if err != nil || !bytes.Contains(goal, []byte(file.Path)) || !bytes.Contains(goal, []byte(file.Name)) {
			t.Fatal("worker goal missed the supplied reference", err)
		}
	}
	// Integrity changes never silently change a subsequently downloaded original.
	writeFixture(t, file.Path, "changed", 0600)
	if got := serveTest(a, "GET", "/work/attachments/"+first.Record.Thread+"/"+file.ID, nil); got.Code != 409 {
		t.Fatal("download accepted altered attachment bytes")
	}
}

func TestTaskUploadsSurviveCheckpointContinuation(t *testing.T) {
	a := taskFixture(t)
	raw, err := os.ReadFile(a.cfg.Agent)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, a.cfg.Agent, string(raw)+"\nsys.exit(2)\n", 0700)
	f := formFor(a)
	f.Set("message", "Create a draft")
	first := uploadedTaskTurn(t, a, uploadTask(t, a, "/work", f, uploadFixture{"notes.txt", []byte("reference facts")}))
	if first.Job.State != "unfinished" {
		t.Fatal("fixture did not pause")
	}
	second := continueFixtureTask(t, a, first)
	if second.Job.State != "unfinished" || second.Record.Resume == nil || !reflect.DeepEqual(first.Record.Attachments, second.Record.Attachments) {
		t.Fatal("continuation lost uploads or original runtime")
	}
	goal, err := os.ReadFile(filepath.Join(filepath.Dir(second.Job.Dir), "execution.txt"))
	if err != nil || !bytes.Contains(goal, []byte(first.Record.Attachments[0].Path)) {
		t.Fatal("continued worker missed attachment", err)
	}
	if b, err := os.ReadFile(first.Record.Attachments[0].Path); err != nil || string(b) != "reference facts" {
		t.Fatal("continuation changed the original", err)
	}
}

func TestTaskLiveUploadsCommitManifestOnceWithoutStartingWork(t *testing.T) {
	a := taskFixture(t)
	raw, err := os.ReadFile(a.cfg.Agent)
	if err != nil {
		t.Fatal(err)
	}
	needle := "assert Path(os.environ['DOCUMENT_INPUT']).parent != Path.cwd()"
	script := strings.Replace(string(raw), needle, `import time
steer=Path(sys.argv[sys.argv.index('-steer')+1])
Path('upload-ready').write_text('ready')
for _ in range(250):
 if steer.read_text().strip(): break
 time.sleep(.02)
else: sys.exit(1)
note=json.loads(steer.read_text().splitlines()[0])
manifest=Path(note['attachments'])
assert not manifest.is_relative_to(Path.cwd())
files=json.loads(manifest.read_text())
assert len(files)==1 and Path(files[0]['Path']).read_bytes()==b'live reference'
`+needle, 1)
	writeFixture(t, a.cfg.Agent, script, 0700)
	f := formFor(a)
	f.Set("message", "Create a document")
	w := serveTest(a, "POST", "/work", f)
	if w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	thread := strings.TrimPrefix(w.Header().Get("Location"), "/work/")
	turn := a.taskTurns(thread)[0]
	ready := filepath.Join(turn.Job.Dir, "execution", "upload-ready")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(ready); err != nil {
		t.Fatal("worker never reached live upload seam", a.jobs.Log(turn.Job.ID, "stderr"))
	}
	f = formFor(a) // Attachment-only updates receive a bounded default message.
	path := "/work/jobs/" + turn.Job.ID + "/message"
	for i := 0; i < 2; i++ {
		w = uploadTask(t, a, path, f, uploadFixture{"live.pdf", []byte("live reference")})
		if w.Code != 303 {
			t.Fatalf("live upload %d: %d %s", i, w.Code, w.Body.String())
		}
	}
	job := awaitJob(t, a.jobs, turn.Job.ID)
	if job.State != "completed" || len(a.jobs.List()) != 1 {
		t.Fatal("live upload failed or started another job", a.jobs.Log(job.ID, "stderr"))
	}
	notes, err := readTaskMessages(filepath.Dir(job.Dir))
	if err != nil || len(notes) != 1 || notes[0].Attachments == "" {
		t.Fatal("manifest note was lost or duplicated", err)
	}
	files, err := taskAttachments(a.cfg.Data, thread)
	if err != nil || len(files) != 1 || filepath.Dir(files[0].Path) != filepath.Dir(notes[0].Attachments) {
		t.Fatal("duplicate nonce wrote a second upload batch", err)
	}
}

func TestTaskUploadsRejectInvalidTypesAndMessageLimits(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []uploadFixture
	}{
		{"type", []uploadFixture{{"program.exe", []byte("bytes")}}},
		{"empty", []uploadFixture{{"empty.pdf", nil}}},
		{"count", []uploadFixture{{"1.pdf", []byte("x")}, {"2.pdf", []byte("x")}, {"3.pdf", []byte("x")}, {"4.pdf", []byte("x")}, {"5.pdf", []byte("x")}, {"6.pdf", []byte("x")}}},
		{"size", []uploadFixture{{"large.pdf", make([]byte, taskUploadLimit+1)}}},
		{"total", []uploadFixture{{"1.pdf", make([]byte, taskUploadLimit)}, {"2.pdf", make([]byte, taskUploadLimit)}, {"3.pdf", []byte("x")}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := taskFixture(t)
			f := formFor(a)
			thread := randomID()
			f.Set("thread", thread)
			w := uploadTask(t, a, "/work", f, tc.files...)
			if w.Code != 409 || len(a.jobs.List()) != 0 {
				t.Fatalf("invalid upload admitted: %d %s", w.Code, w.Body.String())
			}
			files, err := taskAttachments(a.cfg.Data, thread)
			if err != nil || len(files) != 0 {
				t.Fatal("failed batch left committed files", err)
			}
		})
	}
}

func TestTaskUploadsEnforceConversationLimitsBeforeCommit(t *testing.T) {
	for _, tc := range []struct {
		name        string
		count, size int
	}{
		{"count", 40, 1},
		{"bytes", 8, taskUploadLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := taskFixture(t)
			thread := randomID()
			digest := digestText(make([]byte, tc.size))
			// Sparse regular files keep this aggregate-limit fixture inexpensive;
			// their actual size and zero bytes match the controller manifests.
			for i := 0; i < tc.count; {
				batch := filepath.Join(a.cfg.Data, "attachments", thread, randomID())
				if err := os.MkdirAll(batch, 0700); err != nil {
					t.Fatal(err)
				}
				files := []taskAttachment{}
				for j := 0; j < 5 && i < tc.count; j++ {
					id := randomID()
					path := filepath.Join(batch, id+".pdf")
					writeFixture(t, path, "", 0600)
					if err := os.Truncate(path, int64(tc.size)); err != nil {
						t.Fatal(err)
					}
					files = append(files, taskAttachment{ID: id, Name: "reference.pdf", Type: "application/pdf", Size: int64(tc.size), SHA256: digest, Path: path})
					i++
				}
				if err := saveTaskJSON(filepath.Join(batch, "files.json"), files); err != nil {
					t.Fatal(err)
				}
			}
			before, err := taskAttachments(a.cfg.Data, thread)
			if err != nil || len(before) != tc.count {
				t.Fatal("invalid limit fixture", err)
			}
			f := formFor(a)
			f.Set("thread", thread)
			w := uploadTask(t, a, "/work", f, uploadFixture{"one-more.pdf", []byte("x")})
			if w.Code != 409 || len(a.jobs.List()) != 0 {
				t.Fatalf("conversation limit bypassed: %d", w.Code)
			}
			after, err := taskAttachments(a.cfg.Data, thread)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("rejected upload changed committed references", err)
			}
		})
	}
}

type uploadReadProbe struct{ read bool }

func (p *uploadReadProbe) Read([]byte) (int, error) { p.read = true; return 0, io.EOF }
func (p *uploadReadProbe) Close() error             { return nil }

func TestTaskUploadCrossOriginRejectedBeforeBodyRead(t *testing.T) {
	a := taskFixture(t)
	for _, path := range []string{"/work", "/work/jobs/" + randomID() + "/message"} {
		body := &uploadReadProbe{}
		r := httptest.NewRequest("POST", "http://127.0.0.1:8787"+path, nil)
		r.Body = body
		r.Header.Set("Content-Type", "multipart/form-data; boundary=probe")
		r.Header.Set("Origin", "https://untrusted.example")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		a.handler().ServeHTTP(w, r)
		if w.Code != 403 || body.read || len(a.jobs.List()) != 0 {
			t.Fatalf("cross-origin upload reached body parsing: %d read=%t", w.Code, body.read)
		}
	}
}
