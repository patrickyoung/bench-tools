package main

import (
	"context"
	"encoding/json"
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
