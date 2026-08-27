package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	proposalVersion  = "hone.proposal/v1"
	absentHash       = "absent"
	maxProposalBytes = 512 * 1024
)

// proposal is an explicit delta, not a Hone store. The operator names its
// file, may inspect the exact final document, and later admits only while the
// source evidence, wording provenance, and destination-before bytes still
// match.
type proposal struct {
	Version       string   `json:"version"`
	Into          string   `json:"into"`
	Target        string   `json:"target"`
	Source        string   `json:"source"`
	SourceID      string   `json:"source_id"`
	SourceSHA256  string   `json:"source_sha256"`
	Wording       string   `json:"wording"`
	WordingID     string   `json:"wording_id"`
	WordingSHA256 string   `json:"wording_sha256"`
	BeforeSHA256  string   `json:"before_sha256"`
	AfterSHA256   string   `json:"after_sha256"`
	Description   string   `json:"description,omitempty"`
	Lessons       []string `json:"lessons"`
	Document      string   `json:"document"`
}

func (g *hone) prepare(ctx context.Context, artifact, dir, into, source string, s *session, lessons []string, by string) (int, int) {
	name := filepath.Base(strings.TrimRight(dir, string(filepath.Separator)))
	description := ""
	change, err := buildAddition(dir, name, lessons, s.ID, by, func() string {
		g.say("%s is new; wording the line brief will find it by", name)
		description = g.describe(ctx, name, lessons)
		return description
	})
	if err != nil {
		return 0, fail(err)
	}
	if change.added == 0 {
		g.say("%s: %d lesson(s), all already there", s.ID, len(lessons))
		return 0, 1
	}

	target, err := filepath.Abs(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return 0, fail(err)
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return 0, fail(err)
	}
	honeDir, err := honeDir()
	if err != nil {
		return 0, fail(err)
	}
	wording, err := filepath.Abs(filepath.Join(honeDir, by+".jsonl"))
	if err != nil {
		return 0, fail(err)
	}
	sourceHash, err := sha256File(source)
	if err != nil {
		return 0, fail(fmt.Errorf("hash source session: %w", err))
	}
	wordingHash, err := sha256File(wording)
	if err != nil {
		return 0, fail(fmt.Errorf("hash wording session: %w", err))
	}
	beforeHash := absentHash
	if change.existed {
		beforeHash = sha256Bytes(change.before)
	}
	p := proposal{
		Version: proposalVersion, Into: into, Target: target,
		Source: source, SourceID: s.ID, SourceSHA256: sourceHash,
		Wording: wording, WordingID: by, WordingSHA256: wordingHash,
		BeforeSHA256: beforeHash, AfterSHA256: sha256Bytes([]byte(change.document)),
		Description: description, Lessons: append([]string(nil), change.lessons...), Document: change.document,
	}
	if err := validateProposal(p); err != nil {
		return 0, fail(err)
	}
	if err := lintDocument(ctx, g.briefBin, target, p.Document); err != nil {
		return 0, fail(fmt.Errorf("proposed skill does not lint: %w", err))
	}
	if err := writeProposal(artifact, p); err != nil {
		return 0, fail(err)
	}
	if err := showProposal(artifact, p); err != nil {
		return 0, fail(err)
	}
	g.say("%s: %d exact lesson(s) prepared in %s; no skill was changed", s.ID, change.added, artifact)
	return change.added, 0
}

func writeProposal(path string, p proposal) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create proposal %s: %w", path, err)
	}
	keep := false
	defer func() {
		_ = f.Close()
		if !keep {
			_ = os.Remove(path)
		}
	}()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(p); err != nil {
		return fmt.Errorf("write proposal %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close proposal %s: %w", path, err)
	}
	keep = true
	return nil
}

func checkProposalDestination(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("-prepare needs a proposal file")
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("proposal already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect proposal destination %s: %w", path, err)
	}
	parent := filepath.Dir(path)
	info, err := os.Stat(parent)
	if err != nil {
		return fmt.Errorf("proposal directory %s: %w", parent, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("proposal directory %s is not a directory", parent)
	}
	return nil
}

func readProposal(path string) (proposal, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return proposal{}, err
	}
	if !info.Mode().IsRegular() {
		return proposal{}, fmt.Errorf("proposal %s is not a regular file", path)
	}
	if info.Size() > maxProposalBytes {
		return proposal{}, fmt.Errorf("proposal %s exceeds %d bytes", path, maxProposalBytes)
	}
	f, err := os.Open(path)
	if err != nil {
		return proposal{}, err
	}
	defer f.Close()
	dec := json.NewDecoder(io.LimitReader(f, maxProposalBytes+1))
	dec.DisallowUnknownFields()
	var p proposal
	if err := dec.Decode(&p); err != nil {
		return proposal{}, fmt.Errorf("decode proposal %s: %w", path, err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return proposal{}, fmt.Errorf("decode proposal %s: trailing JSON", path)
		}
		return proposal{}, fmt.Errorf("decode proposal %s: %w", path, err)
	}
	if err := validateProposal(p); err != nil {
		return proposal{}, fmt.Errorf("proposal %s: %w", path, err)
	}
	return p, nil
}

func validateProposal(p proposal) error {
	if p.Version != proposalVersion {
		return fmt.Errorf("unsupported version %q", p.Version)
	}
	for label, value := range map[string]string{
		"destination": p.Into, "target": p.Target, "source": p.Source,
		"source id": p.SourceID, "wording": p.Wording, "wording id": p.WordingID,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is empty", label)
		}
		if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			return fmt.Errorf("%s is not safe text", label)
		}
	}
	if !filepath.IsAbs(p.Target) || filepath.Base(p.Target) != "SKILL.md" {
		return errors.New("target is not an absolute SKILL.md path")
	}
	if !filepath.IsAbs(p.Source) || idOf(p.Source) != p.SourceID {
		return errors.New("source path and id do not match")
	}
	if !filepath.IsAbs(p.Wording) || idOf(p.Wording) != p.WordingID {
		return errors.New("wording path and id do not match")
	}
	for label, value := range map[string]string{
		"source sha256": p.SourceSHA256, "wording sha256": p.WordingSHA256,
		"after sha256": p.AfterSHA256,
	} {
		if !validSHA256(value) {
			return fmt.Errorf("%s is invalid", label)
		}
	}
	if p.BeforeSHA256 != absentHash && !validSHA256(p.BeforeSHA256) {
		return errors.New("before sha256 is invalid")
	}
	if p.BeforeSHA256 != absentHash && p.Description != "" {
		return errors.New("an existing-skill proposal carries an unexpected description")
	}
	if !utf8.ValidString(p.Description) || strings.ContainsRune(p.Description, 0) {
		return errors.New("proposal description is not safe text")
	}
	if len(p.Lessons) == 0 {
		return errors.New("proposal has no lessons")
	}
	if len(p.Document) == 0 || len(p.Document) > maxProposalBytes || !utf8.ValidString(p.Document) {
		return errors.New("proposed skill document is empty, oversized, or invalid UTF-8")
	}
	if sha256Bytes([]byte(p.Document)) != p.AfterSHA256 {
		return errors.New("proposed skill document does not match after sha256")
	}
	for _, lesson := range p.Lessons {
		if strings.TrimSpace(lesson) == "" || !utf8.ValidString(lesson) || strings.ContainsRune(lesson, 0) {
			return errors.New("proposal contains an empty or unsafe lesson")
		}
		if !strings.Contains(p.Document, lesson+"\n"+mark(p.SourceID, p.WordingID)) {
			return errors.New("proposed skill does not contain an exact lesson and provenance mark")
		}
	}
	return nil
}

func showProposal(path string, p proposal) error {
	artifactHash, err := sha256File(path)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", proposalVersion)
	fmt.Printf("artifact: %s\nartifact-sha256: %s\n", path, artifactHash)
	fmt.Printf("target: %s\nbefore-sha256: %s\nafter-sha256: %s\n", p.Target, p.BeforeSHA256, p.AfterSHA256)
	fmt.Printf("source: %s\nsource-sha256: %s\n", p.Source, p.SourceSHA256)
	fmt.Printf("wording: %s\nwording-sha256: %s\n", p.Wording, p.WordingSHA256)
	fmt.Println("skill-bytes:")
	fmt.Print(p.Document)
	if !strings.HasSuffix(p.Document, "\n") {
		fmt.Println()
	}
	return nil
}

func cmdShowProposal(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: hone show <proposal>")
		return 2
	}
	p, err := readProposal(args[0])
	if err != nil {
		return fail(err)
	}
	if err := showProposal(args[0], p); err != nil {
		return fail(err)
	}
	return 0
}

func cmdAdmit(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: hone admit <proposal>")
		return 2
	}
	p, err := readProposal(args[0])
	if err != nil {
		return fail(err)
	}
	askBin, err := tool("ASK", "ask", "hone has no model of its own")
	if err != nil {
		return fail(err)
	}
	ctx := context.Background()
	for _, item := range []struct {
		label string
		path  string
		want  string
	}{{"source", p.Source, p.SourceSHA256}, {"wording", p.Wording, p.WordingSHA256}} {
		label, path, want := item.label, item.path, item.want
		got, err := sha256File(path)
		if err != nil {
			return fail(fmt.Errorf("hash %s session: %w", label, err))
		}
		if got != want {
			return fail(fmt.Errorf("%s session changed after proposal review", label))
		}
		if err := verify(ctx, askBin, path); err != nil {
			return fail(err)
		}
	}
	s, err := readSession(p.Source)
	if err != nil {
		return fail(err)
	}
	if s.ID != p.SourceID {
		return fail(errors.New("source session id changed after proposal review"))
	}
	if ok, why := s.Teaches(); !ok {
		fmt.Fprintf(os.Stderr, "hone: %s: %s\n", s.ID, why)
		return 1
	}
	dir, err := skillDir(p.Into)
	if err != nil {
		return fail(err)
	}
	target, err := filepath.Abs(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return fail(err)
	}
	if target != p.Target {
		return fail(fmt.Errorf("destination now resolves to %s, not reviewed target %s", target, p.Target))
	}
	if taught(dir, p.SourceID) {
		fmt.Fprintf(os.Stderr, "hone: %s: already learned from, in %s\n", p.SourceID, p.Into)
		return 1
	}
	before, err := currentHash(target)
	if err != nil {
		return fail(err)
	}
	if before != p.BeforeSHA256 {
		return fail(errors.New("destination skill changed after proposal review"))
	}
	change, err := buildAddition(dir, filepath.Base(filepath.Dir(target)), p.Lessons, p.SourceID, p.WordingID, func() string {
		return p.Description
	})
	if err != nil {
		return fail(err)
	}
	if change.added != len(p.Lessons) || change.document != p.Document {
		return fail(errors.New("proposal is not the exact Hone append for the reviewed destination"))
	}
	briefBin := optional("BRIEF", "brief")
	if err := lintDocument(ctx, briefBin, target, p.Document); err != nil {
		return fail(fmt.Errorf("proposed skill does not lint: %w", err))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fail(err)
	}
	// Recheck immediately before the atomic replace. This also closes the
	// ordinary absent-target race after creating a new skill directory.
	before, err = currentHash(target)
	if err != nil {
		return fail(err)
	}
	if before != p.BeforeSHA256 {
		return fail(errors.New("destination skill changed while admission was being checked"))
	}
	if err := save(target, p.Document); err != nil {
		return fail(err)
	}
	fmt.Printf("%s: %d reviewed lesson(s) admitted (%d total)\n", target, len(p.Lessons), total(p.Document))
	fmt.Fprintf(os.Stderr, "hone: %s · exact proposal %s · no model called\n", p.SourceID, args[0])
	return 0
}

func lintDocument(ctx context.Context, briefBin, target, document string) error {
	if briefBin == "" {
		return nil
	}
	root, err := os.MkdirTemp("", "hone-lint-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	dir := filepath.Join(root, filepath.Base(filepath.Dir(target)))
	if err := os.Mkdir(dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(document), 0o600); err != nil {
		return err
	}
	return lint(ctx, briefBin, dir)
}

func currentHash(path string) (string, error) {
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return absentHash, nil
	}
	if err != nil {
		return "", err
	}
	return sha256Bytes(body), nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sha256Bytes(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func validSHA256(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
