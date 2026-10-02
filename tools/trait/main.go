// Trait composes the public Hire, Agent, Hone, Ask, Brief, Record and Cage
// commands. Expertise remains in Hire's embedded creator and worker files.
package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const version = "0.2.1-dev"

// These are action briefs for the unchanged Hire creator, not a second loop.
//
//go:embed procedures/*.md
var procedures embed.FS

type config struct {
	action, input, output, model, request, entryBoundary string
	turns                                                int
	timeout                                              time.Duration
}

type result struct {
	Schema           int             `json:"schema"`
	Action           string          `json:"action"`
	Status           string          `json:"status"`
	Exit             int             `json:"exit"`
	Input            string          `json:"input"`
	InputSHA256      string          `json:"input_sha256"`
	Output           string          `json:"output"`
	Records          string          `json:"records"`
	Expert           string          `json:"expert,omitempty"`
	ExpertSHA256     string          `json:"expert_sha256,omitempty"`
	EvaluatedSHA256  string          `json:"evaluated_sha256,omitempty"`
	Structural       bool            `json:"structurally_valid"`
	Authored         bool            `json:"authored"`
	Retained         bool            `json:"retained_on_cases"`
	Cases            []caseResult    `json:"cases"`
	Processes        []processResult `json:"processes"`
	Limitations      []string        `json:"limitations"`
	EvaluationOrigin string          `json:"evaluation_origin,omitempty"`
	Entry            string          `json:"entry,omitempty"`
	EntryBoundary    string          `json:"entry_boundary,omitempty"`
	Review           string          `json:"review,omitempty"`
	Error            string          `json:"error,omitempty"`
}

type run struct {
	config                           config
	packet                           packet
	root, source, author, candidate  string
	sourceSHA256                     string
	evaluationRoot, evaluationSHA256 string
	evaluationFiles                  map[string]content
	execution                        execution
	capabilities                     []byte
	programs                         map[string]string
	env                              []string
	ctx                              context.Context
	result                           result
}

func main() { os.Exit(command(os.Args[1:])) }

func command(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" || args[0] == "-help" {
		fmt.Print(help)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" || args[0] == "-version" {
		fmt.Println("trait " + version)
		return 0
	}
	c, err := parse(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Print(help)
			return 0
		}
		return diagnostic(err, 2)
	}
	p, err := admitRequest(c.input, c.action, c.request)
	if err != nil {
		return diagnostic(err, 2)
	}
	programs, err := resolvePrograms(c.action, p.Training.Kind)
	if err != nil {
		return diagnostic(err, 127)
	}
	r, err := prepare(c, p, programs)
	if err != nil {
		return diagnostic(err, 1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	r.ctx = ctx
	code := r.perform()
	if code != 0 && r.result.Status == "running" {
		r.result.Status = "incomplete"
	}
	r.result.Exit = code
	if err := r.finish(); err != nil {
		return diagnostic(err, 125)
	}
	return code
}

func parse(args []string) (config, error) {
	c := config{action: args[0]}
	switch c.action {
	case "create", "edit", "train", "eval":
	default:
		return c, fmt.Errorf("unknown action %q", c.action)
	}
	f := flag.NewFlagSet("trait "+c.action, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	f.StringVar(&c.request, "request", "", "outcome or change; overrides REQUEST.md")
	f.StringVar(&c.entryBoundary, "entry-boundary", "host", "controller boundary: host or cage")
	f.StringVar(&c.model, "m", os.Getenv("ASK_MODEL"), "model")
	f.IntVar(&c.turns, "turns", 12, "maximum worker turns")
	f.DurationVar(&c.timeout, "timeout", 5*time.Minute, "limit per process")
	if err := f.Parse(args[1:]); err != nil {
		return c, err
	}
	if f.NArg() != 2 || c.turns < 1 || c.turns > 64 || c.timeout <= 0 {
		return c, errors.New("usage: trait ACTION [-m MODEL -turns N -timeout D] INPUT_DIR NEW_OUTPUT_DIR")
	}
	if c.entryBoundary != "host" && c.entryBoundary != "cage" {
		return c, errors.New("entry-boundary must be host or cage")
	}
	var err error
	c.input, err = physicalDir(f.Arg(0))
	if err != nil {
		return c, err
	}
	parent, err := physicalDir(filepath.Dir(f.Arg(1)))
	if err != nil {
		return c, err
	}
	c.output = filepath.Join(parent, filepath.Base(f.Arg(1)))
	if overlap(c.input, c.output) {
		return c, errors.New("input and output directories must be disjoint")
	}
	if bundled := os.Getenv("BUNDLE_ROOT"); filepath.IsAbs(bundled) && overlap(filepath.Dir(bundled), c.output) {
		return c, errors.New("output cannot overlap the packaged application")
	}
	if _, err := os.Lstat(c.output); !os.IsNotExist(err) {
		return c, errors.New("output directory must not already exist")
	}
	return c, nil
}

func diagnostic(err error, code int) int { fmt.Fprintln(os.Stderr, "trait:", err); return code }

func (r *run) fail(err error, code int) int {
	r.result.Error = err.Error()
	fmt.Fprintln(os.Stderr, "trait:", err)
	return code
}

func (r *run) agentFlags() []string {
	args := []string{"-turns", fmt.Sprint(r.config.turns), "-timeout", r.config.timeout.String()}
	if r.config.model != "" {
		args = append(args, "-m", r.config.model)
	}
	return args
}

func (r *run) perform() int {
	// Teaching cases freeze before the amendment; they cannot be rewritten by it.
	if r.config.action == "train" {
		if code := r.prepareEvaluation(); code != 0 {
			return code
		}
	}
	if r.config.action == "eval" {
		r.candidate = filepath.Join(r.source, "expert")
	} else {
		if r.config.action == "train" && r.packet.Training.Kind == "recovery" {
			if code := r.recoverLesson(); code != 0 {
				return code
			}
		} else if code := r.authorWorker(); code != 0 {
			return code
		}
		r.result.Authored = true
		if r.config.action == "train" {
			if err := r.trainingBoundary(); err != nil {
				return r.fail(err, 2)
			}
		}
	}
	if code := r.invoke("verify", "hire", []string{"verify", r.candidate}, r.root, nil, nil); code != 0 {
		return code
	}
	r.result.Structural = true
	if r.config.action != "eval" {
		files, err := readTree(r.candidate)
		if err != nil {
			return r.fail(err, 1)
		}
		if err := cleanDefinition(files); err != nil {
			return r.fail(err, 2)
		}
		// Publish a clean candidate before retention testing. A rejected lesson
		// remains reviewable but is never labeled trained by the controller.
		destination := filepath.Join(r.config.output, "expert")
		if err := materialize(destination, files, false); err != nil {
			return r.fail(err, 1)
		}
		r.result.Expert = destination
		r.result.ExpertSHA256 = treeDigest(files)
	}
	if r.config.action != "train" {
		if code := r.prepareEvaluation(); code != 0 {
			return code
		}
	}
	if code := r.reviewEvaluation(); code != 0 {
		return code
	}
	if code := r.evaluate(); code != 0 {
		return code
	}
	if r.config.action == "train" {
		r.result.Status = "retained_on_cases"
		r.result.Retained = true
	} else {
		r.result.Status = "accepted_on_cases"
	}
	return 0
}

func (r *run) authorWorker() int {
	procedure, err := procedures.ReadFile("procedures/" + r.config.action + ".md")
	if err != nil {
		return r.fail(err, 1)
	}
	request := r.packet.Files["REQUEST.md"].Data
	goal := append([]byte{}, procedure...)
	capability, err := procedures.ReadFile("procedures/capabilities.md")
	if err != nil {
		return r.fail(err, 1)
	}
	goal = append(goal, capability...)

	if r.config.action == "train" {
		goal = append(goal, []byte("\nThe caller explicitly selected training scope: "+r.packet.Training.Scope+". General scope must exclude private/company-specific facts; private scope applies only to this selected worker and material.\n")...)
	}
	goal = append(goal, []byte("\nThe immutable input packet is at "+r.source+". Read its REQUEST.md and sources/ as the caller's selected material. Never change that packet.\n\nRequested work:\n")...)
	goal = append(goal, request...)
	goalPath := filepath.Join(r.root, "author-goal.md")
	if err := os.WriteFile(goalPath, goal, 0600); err != nil {
		return r.fail(err, 1)
	}
	args := []string{"build", "-C", r.author, "-state", filepath.Join(r.root, "author-state"), "-evidence", filepath.Join(r.root, "records", "author"), "-goal-file", goalPath}
	args = append(args, r.agentFlags()...)
	return r.invoke("author", "hire", args, r.author, nil, nil)
}

func (r *run) trainingBoundary() error {
	after, err := readTree(r.candidate)
	if err != nil {
		return err
	}
	before := subtree(r.packet.Files, "expert")
	for name, file := range before {
		if strings.HasSuffix(strings.ToLower(name), ".md") && file.Mode&0111 == 0 {
			continue
		}
		current, ok := after[name]
		if !ok || current.Mode != file.Mode || digest(current.Data) != digest(file.Data) {
			return fmt.Errorf("training changed executable/non-Markdown contract %s; use edit for code changes", name)
		}
	}
	for name := range after {
		if current := after[name]; current.Mode&0111 != 0 {
			previous, ok := before[name]
			if !ok || previous.Mode != current.Mode || digest(previous.Data) != digest(current.Data) {
				return fmt.Errorf("training added or changed executable %s", name)
			}
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			if _, ok := before[name]; !ok {
				return fmt.Errorf("training added runtime file %s; use edit", name)
			}
		}
	}
	if treeDigest(before) == treeDigest(after) {
		return errors.New("training produced no definition change; retention cannot be claimed")
	}
	return nil
}

func (r *run) finish() error {
	data, err := json.MarshalIndent(r.result, "", "  ")
	if err != nil {
		return err
	}
	if err := atomicFile(filepath.Join(r.config.output, "result.json"), append(data, '\n')); err != nil {
		return err
	}
	var text strings.Builder
	fmt.Fprintf(&text, "# Trait %s\n\nStatus: %s (exit %d).\n\nInput: %s\n\nPrivate records: %s\n\n", r.config.action, r.result.Status, r.result.Exit, r.config.input, r.root)
	for _, c := range r.result.Cases {
		fmt.Fprintf(&text, "- %s (%s): %s", c.ID, c.Purpose, c.Status)
		if c.EntryExit != nil {
			fmt.Fprintf(&text, "; entry %d", *c.EntryExit)
		} else {
			fmt.Fprintf(&text, "; Agent %d", c.AgentExit)
		}
		if c.CheckExit != nil {
			fmt.Fprintf(&text, "; independent check %d", *c.CheckExit)
		}
		text.WriteByte('\n')
	}
	text.WriteString("\n")
	for _, limit := range r.result.Limitations {
		fmt.Fprintf(&text, "- %s\n", limit)
	}
	if r.result.Error != "" {
		fmt.Fprintf(&text, "\nError: %s\n", r.result.Error)
	}
	if err := atomicFile(filepath.Join(r.config.output, "EVALUATION.md"), []byte(text.String())); err != nil {
		return err
	}
	if _, err := os.Stdout.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

const help = `trait - create, edit, teach and evaluate worker definitions

  trait create [options] INPUT_DIR NEW_OUTPUT_DIR
  trait edit   [options] INPUT_DIR NEW_OUTPUT_DIR
  trait train  [options] INPUT_DIR NEW_OUTPUT_DIR
  trait eval   [options] INPUT_DIR NEW_OUTPUT_DIR
  trait help | version

Options: -request TEXT, -m MODEL, -turns N (12), -timeout D (5m),
         -entry-boundary host|cage (host).
Give create a REQUEST.md or -request; edit/train also need expert/. Eval can
use a previous Trait result directory directly. The creator automatically
assesses the full capability, prepares missing cases/entry wiring, and evaluates.
Optional cases/, evaluation/, CAPABILITIES.md and train.json override preparation.
Train defaults to knowledge/private; explicit recovery needs its original session.
Cases use goal.md, case.json, executable check, optional input/ and expected data.
Stdin is unused: all current inputs are explicitly in INPUT_DIR.

Output: expert/ candidate, reusable evaluation/, checked case snapshots,
CAPABILITIES.md, REVIEW.md, result.json and EVALUATION.md. The same result JSON goes to stdout; progress
goes to stderr. Input is never edited; output must be new. Private records
stay separate from case work. No implicit continuation or publication.
Train changes instructions/skills, not model weights; fresh selected checks
support retained_on_cases. Create/edit also run fresh cases automatically.
Team controllers run on the host; child actions normally retain Agent's Cage.
Optional cage confines entry writes, with full network/host reads; nested Cage
may be unsupported. No fallback. Model review is advisory inspection.
Model access uses the existing Ask configuration. Evaluation makes model calls.
`
