package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMayGateReceiptBindsFreshProcessAndExactStreams(t *testing.T) {
	_ = fakeMay(t, "parked")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := openMayGate(bin, "bench-exact")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := canonicalWorkDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{Dir: dir, Path: "/bin", Shell: "/bin/sh", Timeout: time.Second, Cap: 1024}
	receipt, err := gate.Request(context.Background(), "contract-exact", "printf exact", runner)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(receipt.MayPath) || receipt.MayPath != gate.Bin ||
		receipt.MaySHA256 != gate.BinSHA256 {
		t.Fatalf("May identity not bound: %#v", receipt)
	}
	if !reflect.DeepEqual(receipt.MayArgv, []string{gate.Bin, "request", gate.Job}) {
		t.Fatalf("May argv=%q", receipt.MayArgv)
	}
	if receipt.ActionSHA256 == "" || receipt.ActionSHA256 != receipt.MayInputSHA256 ||
		receipt.MayStdoutSHA256 == "" || receipt.MayStdoutBytes <= 0 || receipt.MayExitCode != 75 {
		t.Fatalf("May streams/status not bound: %#v", receipt)
	}
	body, err := json.Marshal(receipt.Action)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Digest != mayDigest(gate.Job, string(append(body, '\n'))) {
		t.Fatalf("May digest=%q does not bind its exact action", receipt.Digest)
	}
}

func TestApprovalEnvelopePreservesFractionalTimeoutExactly(t *testing.T) {
	runner := Runner{Dir: "/work", Path: "/tools", Shell: "/bin/sh", Timeout: 501 * time.Nanosecond, Cap: 1024}
	action := approvalActionFor("contract", "true", runner)
	if action.TimeoutNS != int64(runner.Timeout) || action.Path != runner.Path {
		t.Fatalf("process context drifted: action=%#v runner=%#v", action, runner)
	}
}

func TestCagedApprovalEnvelopeBindsExactLauncherPolicy(t *testing.T) {
	workspace, err := canonicalWorkDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cageBin := filepath.Join(t.TempDir(), "cage")
	if err := os.WriteFile(cageBin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	launcher, err := openCageLauncher(cageBin, workspace, filepath.Join(t.TempDir(), "private"))
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{Dir: workspace, Path: "/tools", Shell: "/bin/sh",
		Timeout: 501 * time.Nanosecond, Cap: 1024, Cage: launcher}
	action := approvalActionFor("contract", "printf exact", runner)
	if action.Version != 2 || action.Confinement == nil {
		t.Fatalf("action=%#v", action)
	}
	wantArgv := []string{launcher.Bin, "-w", workspace, "--", "/bin/sh", "-c", "printf exact"}
	if action.Confinement.Kind != "cage" || action.Confinement.CageSHA256 != launcher.BinSHA256 ||
		action.Confinement.Workspace != workspace || action.Confinement.TempDir != launcher.TempDir ||
		action.Confinement.Network || !reflect.DeepEqual(action.Confinement.Argv, wantArgv) {
		t.Fatalf("confinement=%#v", action.Confinement)
	}
}

func TestLegacyApprovalEnvelopeBytesStayVersionOne(t *testing.T) {
	runner := Runner{Dir: "/work", Path: "/tools", Shell: "/bin/sh", Timeout: time.Second}
	body, err := json.Marshal(approvalActionFor("contract", "true", runner))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":1,"contract_id":"contract","directory":"/work","shell":"/bin/sh","path":"/tools","timeout_ns":1000000000,"script":"true"}`
	if string(body) != want {
		t.Fatalf("legacy v1 bytes changed:\n%s\nwant:\n%s", body, want)
	}
}

func TestApprovalEnvelopeBindsAnIntentionallyEmptyPATH(t *testing.T) {
	runner := Runner{Dir: "/work", Path: "", Shell: "/bin/sh", Timeout: time.Second, Cap: 1024}
	if err := validateApprovalText("PATH", runner.Path, true); err != nil {
		t.Fatalf("empty PATH is valid execution context: %v", err)
	}
	if got := approvalActionFor("", "true", runner).Path; got != "" {
		t.Fatalf("PATH=%q, want exact empty value", got)
	}
}

func TestMayDigestKnownWireVector(t *testing.T) {
	const want = "dacbdd07dd815422813fd388df0a988315d29eeb79312c0bc0dc4ea120f11b41"
	if got := mayDigest("bench-vector", "exact action\n"); got != want {
		t.Fatalf("May v1 digest=%s, want published vector %s", got, want)
	}
}

func TestApprovalRejectsTextThatJSONWouldRewrite(t *testing.T) {
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := openMayGate(bin, "bench-text")
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{Dir: "/work", Path: "/bin", Shell: "/bin/sh", Timeout: time.Second, Cap: 1024}
	for _, script := range []string{"bad\xffbytes", "nul\x00byte"} {
		if _, err := gate.Request(context.Background(), "contract", script, runner); err == nil {
			t.Fatalf("approval accepted non-exact script %q", script)
		}
	}
}

func TestCagedApprovalRejectsConfinementTextThatJSONWouldRewrite(t *testing.T) {
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := openMayGate(bin, "bench-cage-text")
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{Dir: "/work", Path: "/bin", Shell: "/bin/sh", Timeout: time.Second,
		Cage: &cageLauncher{Bin: "bad\xffcage", BinSHA256: "sha256:x", Workspace: "/work", TempDir: "/tmp/private"}}
	if _, err := gate.Request(context.Background(), "contract", "true", runner); err == nil ||
		!strings.Contains(err.Error(), "valid UTF-8") {
		t.Fatalf("invalid Cage bytes reached JSON/May: %v", err)
	}
}

func TestMayResultCarriesAHeavilyEscapedValidAction(t *testing.T) {
	_ = fakeMay(t, "parked")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	gate, err := openMayGate(bin, "bench-escaped")
	if err != nil {
		t.Fatal(err)
	}
	runner := Runner{Dir: "/work", Path: "/bin", Shell: "/bin/sh", Timeout: time.Second, Cap: 1024}
	script := strings.Repeat("\\\"", 3_000)
	body, err := json.Marshal(approvalActionFor("contract", script, runner))
	if err != nil {
		t.Fatal(err)
	}
	if len(body)+1 >= maxMayAction {
		t.Fatalf("test action unexpectedly exceeds May input bound: %d", len(body)+1)
	}
	receipt, err := gate.Request(context.Background(), "contract", script, runner)
	if err != nil {
		t.Fatalf("valid escaped action was rejected: %v", err)
	}
	if receipt.MayStdoutBytes <= int64(len(body)) || receipt.MayStdoutBytes > maxApprovalResult {
		t.Fatalf("unexpected expanded result size: input=%d output=%d", len(body)+1, receipt.MayStdoutBytes)
	}
}

func TestMayCancellationStopsDescendantsRetainingOutputPipes(t *testing.T) {
	t.Setenv("PLY_DEPTH", "0")
	dir := t.TempDir()
	script, pidfile := stubbornDescendant(t, dir, false)
	bin := filepath.Join(dir, "may")
	write(t, bin, "#!/bin/sh\ncat >/dev/null\n"+script+"\n", 0o755)
	gate, err := openMayGate(bin, "test-cancellation")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := gate.Request(ctx, "", "true", newRunner(t, dir, os.Getenv("PATH")))
		done <- err
	}()
	pid := waitForPID(t, pidfile)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v, want context cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("May cancellation waited indefinitely for its descendant's pipe")
	}
	requireProcessStopped(t, pid)
}

func TestMayCancellationAfterNonzeroLeaderExitStopsPipeHolder(t *testing.T) {
	t.Setenv("PLY_DEPTH", "0")
	dir := t.TempDir()
	script, pidfile := stubbornDescendant(t, dir, false)
	script = strings.TrimSuffix(script, "\nwait") + "\nexit 75"
	bin := filepath.Join(dir, "may")
	write(t, bin, "#!/bin/sh\ncat >/dev/null\n"+script+"\n", 0o755)
	gate, err := openMayGate(bin, "test-cancellation-after-exit")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := gate.Request(ctx, "", "true", newRunner(t, dir, os.Getenv("PATH")))
		done <- err
	}()
	pid := waitForPID(t, pidfile)
	// The direct May process is already gone. os/exec has stopped watching
	// its context, but the child still owns the stdout and stderr writers.
	requireProcessStopped(t, waitForPID(t, filepath.Join(dir, "group.pid")))
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v, want context cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("May cancellation after its leader exited was not bounded")
	}
	requireProcessStopped(t, pid)
}
