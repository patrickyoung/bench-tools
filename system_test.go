package main

import (
	"strings"
	"testing"
	"time"
)

func TestRootPromptMakesExplicitDelegationAVisibleUnixComposition(t *testing.T) {
	text := prompt(&Box{Shell: true}, "/work", "", 2*time.Minute, 16<<10, 0)
	for _, want := range []string{
		"If and only if the goal explicitly asks for subagents",
		`"$PLY" -sh -turns 12 -C . -f "$run/NNN.jsonl" --`,
		"Run at most",
		"skills do not carry over",
		"stdout and stderr to matching NNN.out and NNN.err",
		"results in task order",
		"missing or nonzero status",
		"sole writer and synthesizer",
		"disjoint worktrees",
		"Synthesize the child summaries yourself",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestNestedPromptDoesNotAdvertiseRecursiveDelegation(t *testing.T) {
	text := prompt(&Box{Shell: true}, "/work", "", time.Minute, 1024, 1)
	if strings.Contains(text, "another ordinary ply process is a subagent") || strings.Contains(text, `"$PLY" -sh -C`) {
		t.Fatalf("nested prompt advertises recursion:\n%s", text)
	}
}

func TestPromptDoesNotAdvertiseUnavailableToolboxBookkeeping(t *testing.T) {
	narrow := &Box{Dir: "/tools", Tools: []Tool{{Name: "rg"}}}
	if text := prompt(narrow, "/work", "", time.Minute, 1024, 0); strings.Contains(text, "another ordinary ply process is a subagent") {
		t.Fatalf("narrow toolbox advertised unavailable delegation:\n%s", text)
	}
	complete := &Box{Dir: "/tools", Tools: []Tool{{Name: "mkdir"}, {Name: "mktemp"}, {Name: "mv"}}}
	if text := prompt(complete, "/work", "", time.Minute, 1024, 0); !strings.Contains(text, `"$PLY" -t '/tools'`) {
		t.Fatalf("capable toolbox omitted delegation:\n%s", text)
	}
}

func TestSubagentCommandPreservesTheResolvedToolGrant(t *testing.T) {
	for _, tc := range []struct {
		name string
		box  *Box
		want string
	}{
		{name: "shell", box: &Box{Shell: true}, want: `"$PLY" -sh -turns 12 -C . -f "$run/NNN.jsonl" --`},
		{name: "toolbox", box: &Box{Dir: "/work/tool box"}, want: `"$PLY" -t '/work/tool box' -turns 12 -C . -f "$run/NNN.jsonl" --`},
		{name: "toolbox and shell", box: &Box{Dir: "/work/it's tools", Shell: true}, want: `"$PLY" -t '/work/it'"'"'s tools' -sh -turns 12 -C . -f "$run/NNN.jsonl" --`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := subagentCommand(tc.box); got != tc.want {
				t.Fatalf("command=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunnerExportsAnExplicitModelForNestedPly(t *testing.T) {
	o := newOpts("ply")
	if err := o.fs.Parse([]string{"-m", "openai/test-model"}); err != nil {
		t.Fatal(err)
	}
	r := o.runner(&Box{Shell: true}, "/bin/ply", 0)
	if got := strings.Join(r.Env, "\n"); !strings.Contains(got, "ASK_MODEL=openai/test-model") {
		t.Fatalf("runner env=%q", got)
	}

	o = newOpts("ply")
	r = o.runner(&Box{Shell: true}, "/bin/ply", 0)
	if got := strings.Join(r.Env, "\n"); strings.Contains(got, "ASK_MODEL=") {
		t.Fatalf("runner overrode ambient model without -m: %q", got)
	}
}
