package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"syscall"
)

func (r *recorder) streamNote(kind string, value any) error {
	line, err := json.Marshal(struct {
		Kind string `json:"kind"`
		Body any    `json:"body"`
	}{"record." + kind + "/v1", value})
	if err != nil {
		return err
	}
	if err := writeAll(r.writer, append(line, '\n')); err != nil {
		return err
	}
	var ack struct {
		Seq int `json:"seq"`
	}
	if err := r.acks.Decode(&ack); err != nil {
		return fmt.Errorf("Ask seal acknowledgement: %w", err)
	}
	if ack.Seq <= 0 || (r.lastAck > 0 && ack.Seq != r.lastAck+2) {
		return fmt.Errorf("invalid Ask seal acknowledgement: %d", ack.Seq)
	}
	r.lastAck = ack.Seq
	return nil
}

func (r *recorder) startNotes() error {
	cmd := exec.Command(r.ask, "note", "-q", "-f", r.file, "-s", "record", "-jsonl", "-", "-seal")
	// The caller may interrupt Record's whole process group. Its model-free
	// writer must survive to acknowledge and seal the interrupted outcome.
	// Closing our input still terminates this helper, including on a crash.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	writer, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	reader, err := cmd.StdoutPipe()
	if err != nil {
		writer.Close()
		return err
	}
	cmd.Stderr = &r.diagnostic
	if err := cmd.Start(); err != nil {
		writer.Close()
		reader.Close()
		return err
	}
	r.writer, r.acks, r.process = writer, json.NewDecoder(reader), cmd
	return nil
}

func (r *recorder) closeNotes() error {
	if r.process == nil {
		return nil
	}
	r.writer.Close()
	err := r.process.Wait()
	r.process = nil
	if err != nil {
		return fmt.Errorf("Ask recording: %w: %s", err, r.diagnostic.String())
	}
	return nil
}
