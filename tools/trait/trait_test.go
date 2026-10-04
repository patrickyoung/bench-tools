package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeTest(t *testing.T, name, data string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(name, mode); err != nil {
		t.Fatal(err)
	}
}

func expertFiles() map[string]content {
	return map[string]content{
		"AGENTS.md":            {[]byte("Read the packet and preserve exact bytes.\n"), 0644},
		"README.md":            {[]byte("A bounded packet worker.\n"), 0644},
		"bin/check":            {[]byte("#!/bin/sh\ncmp input.txt answer.txt\n"), 0755},
		"skills/copy/SKILL.md": {[]byte("---\nname: copy\ndescription: Copy the input bytes.\n---\nCopy exactly.\n"), 0644},
	}
}

func packetTest(t *testing.T, action string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "input")
	files := map[string]content{"REQUEST.md": {[]byte("Teach precise copying.\n"), 0644}}
	if action != "create" {
		for name, c := range expertFiles() {
			files["expert/"+name] = c
		}
	}
	if action == "train" || action == "eval" {
		for _, purpose := range []string{"transfer", "regression"} {
			base := "cases/" + purpose + "/"
			files[base+"goal.md"] = content{[]byte("Copy the selected current input.\n"), 0644}
			files[base+"case.json"] = content{[]byte(`{"purpose":"` + purpose + `","outputs":["answer.txt"]}`), 0644}
			files[base+"check"] = content{[]byte("#!/bin/sh\ncmp input.txt answer.txt\n"), 0755}
		}
	}
	if action == "train" {
		files["train.json"] = content{[]byte(`{"kind":"knowledge","scope":"private"}`), 0644}
	}
	if err := materialize(root, files, false); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAdmitRejectsUnsafeOrIncompletePacketsBeforeWork(t *testing.T) {
	for _, test := range []struct {
		name  string
		alter func(string)
	}{
		{"missing-transfer", func(p string) { os.RemoveAll(filepath.Join(p, "cases/transfer")) }},
		{"executable-missing", func(p string) { os.Chmod(filepath.Join(p, "cases/transfer/check"), 0644) }},
		{"unknown-top-level", func(p string) { writeTest(t, filepath.Join(p, "credential.txt"), "not selected", 0600) }},
		{"unknown-training-field", func(p string) {
			writeTest(t, filepath.Join(p, "train.json"), `{"kind":"knowledge","scope":"private","approve":true}`, 0644)
		}},
		{"unselected-scope", func(p string) { writeTest(t, filepath.Join(p, "train.json"), `{"kind":"knowledge"}`, 0644) }},
		{"escaping-output", func(p string) {
			writeTest(t, filepath.Join(p, "cases/transfer/case.json"), `{"purpose":"transfer","outputs":["../outside"]}`, 0644)
		}},
		{"symlink-input", func(p string) {
			if err := os.Symlink("REQUEST.md", filepath.Join(p, "sources")); err != nil {
				t.Fatal(err)
			}
		}},
		{"private-runtime", func(p string) { writeTest(t, filepath.Join(p, "expert/.agent/private"), "old evidence", 0600) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := packetTest(t, "train")
			test.alter(p)
			if _, err := admit(p, "train"); err == nil {
				t.Fatal("unsafe/incomplete packet admitted")
			}
		})
	}
	if _, err := admit(packetTest(t, "train"), "train"); err != nil {
		t.Fatal(err)
	}
}

func TestPathsRefuseInPlaceOutputAndStorageWithoutChangingInput(t *testing.T) {
	in := packetTest(t, "create")
	in, err := physicalDir(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{in, filepath.Join(in, "result"), filepath.Dir(in)} {
		if _, err := parse([]string{"create", in, out}); err == nil {
			t.Fatalf("admitted overlapping output %s", out)
		}
	}
	// Missing state ancestors are validated without creating them in input.
	t.Setenv("XDG_STATE_HOME", filepath.Join(in, "new-state", "nested"))
	p, err := admit(in, "create")
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "result")
	if _, err := prepare(config{action: "create", input: in, output: out}, p, nil); err == nil {
		t.Fatal("overlapping storage admitted")
	}
	if _, err := os.Lstat(filepath.Join(in, "new-state")); !os.IsNotExist(err) {
		t.Fatal("rejected input was modified")
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		t.Fatal("rejected run created output")
	}
	if !overlap("/", in) {
		t.Fatal("filesystem root must overlap its descendants")
	}
}

func TestTrainingFreezesCodeIncludingExecutableMarkdown(t *testing.T) {
	for _, change := range []string{"method", "checker", "data", "executable-md", "chmod-md", "unchanged"} {
		t.Run(change, func(t *testing.T) {
			before := expertFiles()
			after := expertFiles()
			before["bin/check.md"] = content{[]byte("#!/bin/sh\nexit 1\n"), 0755}
			after["bin/check.md"] = before["bin/check.md"]
			if change != "unchanged" {
				after["AGENTS.md"] = content{[]byte("Copy exact bytes; check the source binding.\n"), 0644}
			}
			switch change {
			case "checker":
				after["bin/check"] = content{[]byte("#!/bin/sh\nexit 0\n"), 0755}
			case "data":
				after["answer.json"] = content{[]byte(`{"answer":"expected"}`), 0644}
			case "executable-md":
				after["bin/check.md"] = content{[]byte("#!/bin/sh\nexit 0\n"), 0755}
			case "chmod-md":
				after["AGENTS.md"] = content{after["AGENTS.md"].Data, 0755}
			}
			p := packet{Files: map[string]content{}}
			for name, f := range before {
				p.Files["expert/"+name] = f
			}
			candidate := filepath.Join(t.TempDir(), "expert")
			if err := materialize(candidate, after, false); err != nil {
				t.Fatal(err)
			}
			r := run{packet: p, candidate: candidate}
			err := r.trainingBoundary()
			if (err == nil) != (change == "method") {
				t.Fatalf("%s: %v", change, err)
			}
		})
	}
}

func fakeRun(t *testing.T, scripts map[string]string) *run {
	t.Helper()
	root := t.TempDir()
	programs := map[string]string{}
	for name, script := range scripts {
		programs[name] = filepath.Join(root, "bin", name)
		writeTest(t, programs[name], "#!/bin/sh\nset -eu\n"+script+"\n", 0755)
	}
	return &run{root: root, programs: programs, env: []string{"PATH=/usr/bin:/bin", "HOME=" + root}, ctx: context.Background(), config: config{timeout: time.Minute}}
}

func TestLiteralProcessBoundaryAndExitOutcomes(t *testing.T) {
	r := fakeRun(t, map[string]string{"probe": `printf '%s\n' "$@"; cat; printf 'diagnostic\n' >&2; exit "${CODE:-0}"`})
	for _, code := range []int{0, 2, 3, 75, 125, 130} {
		label := strings.ReplaceAll(time.Now().Format("150405.000000000"), ".", "-")
		args := []string{"space argument", "$(touch unexpected)", "; exit 9"}
		got := r.invoke(label, "probe", args, r.root, []byte("input\x00bytes\n"), map[string]string{"CODE": jsonNumber(code)})
		if got != code {
			t.Fatalf("exit %d became %d", code, got)
		}
		stdout, err := r.stdout(label)
		if err != nil {
			t.Fatal(err)
		}
		if string(stdout) != strings.Join(args, "\n")+"\ninput\x00bytes\n" {
			t.Fatalf("literal argv/stdin changed: %q", stdout)
		}
		diag, err := os.ReadFile(filepath.Join(r.root, "records", label, "stderr"))
		if err != nil || string(diag) != "diagnostic\n" {
			t.Fatal("stderr mixed with result")
		}
	}
}

func jsonNumber(n int) string { b, _ := json.Marshal(n); return string(b) }

func TestTimeoutAndCancellationAreNotSuccess(t *testing.T) {
	r := fakeRun(t, map[string]string{"sleep": "sleep 10"})
	r.config.timeout = 30 * time.Millisecond
	if code := r.invoke("timeout", "sleep", nil, r.root, nil, nil); code != 124 {
		t.Fatalf("timeout: %d", code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.ctx = ctx
	if code := r.invoke("cancelled", "sleep", nil, r.root, nil, nil); code != 130 {
		t.Fatalf("cancel: %d", code)
	}
}

func TestRelativeSearchAndProviderSelectionsStayAtCallerCwd(t *testing.T) {
	env, err := stableEnvironment([]string{"PATH=bin::/usr/bin", "AGENT_ASK=./approved-wrapper", "AGENT_PLY=./ply"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range env {
		key, value, _ := strings.Cut(item, "=")
		if key == "PATH" {
			for _, part := range filepath.SplitList(value) {
				if !filepath.IsAbs(part) {
					t.Fatalf("relative search directory %q", part)
				}
			}
		} else if !filepath.IsAbs(value) {
			t.Fatalf("relative selected companion: %s", item)
		}
	}
}

func TestEvaluationRefusesDefinitionDriftBeforeCreditingACase(t *testing.T) {
	r := fakeRun(t, map[string]string{"agent": `for last do :; done; printf '\nchanged during verification\n' >> "$last/AGENTS.md"`})
	p, err := admit(packetTest(t, "train"), "train")
	if err != nil {
		t.Fatal(err)
	}
	r.packet = p
	r.candidate = filepath.Join(r.root, "candidate")
	r.source = filepath.Join(r.root, "input")
	if err := materialize(r.candidate, subtree(p.Files, "expert"), false); err != nil {
		t.Fatal(err)
	}
	if err := materialize(r.source, p.Files, true); err != nil {
		t.Fatal(err)
	}
	source, err := readTree(r.source)
	if err != nil {
		t.Fatal(err)
	}
	r.sourceSHA256 = treeDigest(source)
	r.config.output = t.TempDir()
	if err := os.Mkdir(filepath.Join(r.root, "cases"), 0700); err != nil {
		t.Fatal(err)
	}
	if code := r.evaluate(); code != 125 {
		t.Fatalf("changed definition produced status %d", code)
	}
	if len(r.result.Cases) != 1 || r.result.Cases[0].CheckExit != nil || r.result.Cases[0].Status != "definition_or_case_changed" {
		t.Fatalf("drift was credited as acceptance: %+v", r.result.Cases)
	}
}

func TestRecoveryReviewRejectsUnreviewedProposalBytes(t *testing.T) {
	for _, review := range []string{"approve", "reject", "replace", "malformed"} {
		t.Run(review, func(t *testing.T) {
			ask := `cat > review-input.json; printf '{"approved":true,"reason":"supported bounded lesson"}\n'`
			switch review {
			case "replace":
				ask = `cat > review-input.json; printf 'other valid proposal' > lesson.json; printf '{"approved":true,"reason":"supported"}\n'`
			case "reject":
				ask = `cat > review-input.json; printf '{"approved":false,"reason":"unsupported claim"}\n'`
			case "malformed":
				ask = `cat > review-input.json; printf '{"approved":true}\n'`
			}
			r := fakeRun(t, map[string]string{
				"hone": `case "$1" in
  -why) printf 'qualifying recovery\n';;
  -into) printf 'exact prepared proposal' > "$4";;
  show) cat "$2";;
  admit) printf 'admitted' > admission-marker;;
  *) exit 19;;
esac`,
				"ask": ask, "brief": "exit 0",
			})
			r.candidate = filepath.Join(r.root, "expert")
			r.packet = packet{Training: training{Kind: "recovery", Scope: "private", Skill: "copy", Session: "/explicit/session.jsonl"}, Files: map[string]content{"REQUEST.md": {[]byte("Teach the selected recovery"), 0644}}}
			code := r.recoverLesson()
			_, err := os.Stat(filepath.Join(r.root, "admission-marker"))
			if review == "approve" {
				if code != 0 || err != nil {
					t.Fatalf("approved lesson not admitted: %d %v", code, err)
				}
			} else if code != 2 || !os.IsNotExist(err) {
				t.Fatalf("unsafe review admitted (%s): %d %v", review, code, err)
			}
			var evidence map[string]any
			b, err := os.ReadFile(filepath.Join(r.root, "review-input.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(b, &evidence); err != nil {
				t.Fatal(err)
			}
			if evidence["proposal_sha256"] != digest([]byte("exact prepared proposal")) || evidence["scope"] != "private" {
				t.Fatal("review lost exact selected proposal or scope")
			}
		})
	}
}
