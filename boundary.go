package main

import (
	"context"
	"errors"
	"fmt"
)

const externalActionBoundaryReceiptKind = "ply.action-boundary/v1"

// externalActionBoundary is an operator-owned adapter convention. Unlike
// Ply's native Cage launcher, Ply cannot inspect the adapter's internal
// policy, so the receipt binds only what Ply can prove at its own boundary.
type externalActionBoundary struct {
	ExitCode int
	Path     string
	SHA256   string
}

func (b *externalActionBoundary) checkDigest() error {
	digest, err := executableDigest("external action adapter", b.Path)
	if err != nil {
		return err
	}
	if digest != b.SHA256 {
		return errors.New("external action adapter changed during the run")
	}
	return nil
}

type externalActionBoundaryReceipt struct {
	Version       int    `json:"version"`
	ContractID    string `json:"contract_id,omitempty"`
	AdapterPath   string `json:"adapter_path"`
	AdapterSHA256 string `json:"adapter_sha256"`
	ScriptSHA256  string `json:"script_sha256"`
	ExitCode      int    `json:"exit_code"`
	MayHaveRun    bool   `json:"may_have_run"`
	Detail        string `json:"detail"`
	Output        []byte `json:"output,omitempty"`
	OutputSHA256  string `json:"output_sha256"`
	OutputBytes   int64  `json:"output_bytes"`
	ElidedBytes   int64  `json:"elided_bytes,omitempty"`
}

func (l *Loop) recordExternalActionBoundary(ctx context.Context, script string, result Result, detail string, mayHaveRun bool) error {
	if l.Model.Session == "" || l.ActionBoundary == nil {
		return fmt.Errorf("record external action boundary: missing session or adapter policy")
	}
	receipt := externalActionBoundaryReceipt{
		Version: 1, ContractID: l.ContractID,
		AdapterPath: l.ActionBoundary.Path, AdapterSHA256: l.ActionBoundary.SHA256,
		ScriptSHA256: digestText(script), ExitCode: result.Code,
		MayHaveRun: mayHaveRun, Detail: detail, Output: []byte(result.Output),
		OutputSHA256: digestText(result.Output), OutputBytes: result.Total,
		ElidedBytes: result.Elided,
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), approvalRecordTimeout)
	defer cancel()
	if err := l.Model.Record(recordCtx, verdictSource, externalActionBoundaryReceiptKind, receipt); err != nil {
		return fmt.Errorf("record external action boundary: %w", err)
	}
	return nil
}
