package main

import (
	"context"
	"errors"
	"fmt"
)

const confinementReceiptKind = "ply.confinement/v1"

type confinementReceipt struct {
	Version              int    `json:"version"`
	ContractID           string `json:"contract_id,omitempty"`
	ApprovalDigest       string `json:"approval_digest"`
	ApprovalActionSHA256 string `json:"approval_action_sha256"`
	ScriptSHA256         string `json:"script_sha256"`
	CagePath             string `json:"cage_path"`
	CageSHA256           string `json:"cage_sha256"`
	Workspace            string `json:"workspace"`
	TempDir              string `json:"temp_dir"`
	ExitCode             int    `json:"exit_code"`
	MayHaveRun           bool   `json:"may_have_run"`
	Detail               string `json:"detail"`
	Output               []byte `json:"output,omitempty"`
	OutputSHA256         string `json:"output_sha256"`
	OutputBytes          int64  `json:"output_bytes"`
	ElidedBytes          int64  `json:"elided_bytes,omitempty"`
}

func (l *Loop) recordConfinement(ctx context.Context, approval *approvalReceipt, result Result) error {
	if l.Model.Session == "" {
		return errors.New("record confinement receipt: no Ask session")
	}
	if approval == nil || l.Runner.Cage == nil {
		return errors.New("record confinement receipt: missing approval or Cage policy")
	}
	if result.Interrupted {
		result.ConfinementDetail += "; interrupted; action effects may exist"
	}
	receipt := confinementReceipt{
		Version: 1, ContractID: l.ContractID,
		ApprovalDigest: approval.Digest, ApprovalActionSHA256: approval.ActionSHA256,
		ScriptSHA256: digestText(result.Cmd), CagePath: l.Runner.Cage.Bin,
		CageSHA256: l.Runner.Cage.BinSHA256, Workspace: l.Runner.Cage.Workspace,
		TempDir: l.Runner.Cage.TempDir, ExitCode: result.Code,
		MayHaveRun: result.ConfinementMayHaveRun, Detail: result.ConfinementDetail,
		Output: []byte(result.Output), OutputSHA256: digestText(result.Output),
		OutputBytes: result.Total, ElidedBytes: result.Elided,
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), approvalRecordTimeout)
	defer cancel()
	if err := l.Model.Record(recordCtx, verdictSource, confinementReceiptKind, receipt); err != nil {
		return fmt.Errorf("record confinement receipt: %w", err)
	}
	return nil
}
