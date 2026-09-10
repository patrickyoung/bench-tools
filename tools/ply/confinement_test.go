package main

import (
	"encoding/json"
	"testing"
)

func TestConfinementReceiptV1WireShape(t *testing.T) {
	receipt := confinementReceipt{
		Version: 1, ContractID: "contract", ApprovalDigest: "may-digest",
		ApprovalActionSHA256: "sha256:action", ScriptSHA256: "sha256:script",
		CagePath: "/bin/cage", CageSHA256: "sha256:cage", Workspace: "/work",
		TempDir: "/state/run.cage-tmp", ExitCode: 125, MayHaveRun: true,
		Detail: "reserved status", Output: []byte{0xff},
		OutputSHA256: "sha256:output", OutputBytes: 1,
	}
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"version":1,"contract_id":"contract","approval_digest":"may-digest","approval_action_sha256":"sha256:action","script_sha256":"sha256:script","cage_path":"/bin/cage","cage_sha256":"sha256:cage","workspace":"/work","temp_dir":"/state/run.cage-tmp","exit_code":125,"may_have_run":true,"detail":"reserved status","output":"/w==","output_sha256":"sha256:output","output_bytes":1}`
	if string(body) != want {
		t.Fatalf("confinement v1 bytes changed:\n%s\nwant:\n%s", body, want)
	}
}
