package main

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPromptTurnsRequestedOutcomesIntoRealWork(t *testing.T) {
	text := strings.Join(strings.Fields(prompt(&Box{Shell: true}, defaultShell, "/work", "", time.Minute, 1024, 0)), " ")
	for _, want := range []string{
		"A reply is only a report",
		"inspect the relevant evidence",
		"without making unrequested changes",
		"use the available programs to make that effect real",
		"while the requested effect remains undone",
		"preserve unrelated work",
		"prefer reversible operations",
		"exactly one nonempty fenced shell block, as its final content",
		"A turn with no shell block is your final report",
		"runs only the first complete command block",
		"defers every later block",
		"send the first command now and wait",
		"Each block runs as '/bin/sh' -c SCRIPT",
		"POSIX shell syntax is the portable baseline",
		"inspect the result with an independent command",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestComposedSkillEndsWithActionAndRequiredInteractionPolicy(t *testing.T) {
	text := composeSystem("BASE PROTOCOL\n", "\nSKILL PROCEDURE\n", true)
	for _, want := range []string{"SKILL PROCEDURE", "PLY ACTION PROTOCOL REMINDER", "Do not claim the shell", "is unavailable", "requires real tool interaction", "At least one command must run"} {
		if !strings.Contains(text, want) {
			t.Errorf("composed system missing %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "PLY ACTION PROTOCOL REMINDER") < strings.Index(text, "SKILL PROCEDURE") {
		t.Fatalf("action reminder did not follow the skill:\n%s", text)
	}
}

func TestPromptNamesTheActualHostAndDiscouragesForeignSyntax(t *testing.T) {
	text := strings.Join(strings.Fields(prompt(&Box{Shell: true}, defaultShell, "/work", "", time.Minute, 1024, 0)), " ")
	for _, want := range []string{
		"It runs in /work on " + platformName(),
		"Use command -v",
		"instead of assuming GNU and BSD options are interchangeable",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if got := platformName(); got == "" || (runtime.GOOS == "darwin" && got != "macOS") || (runtime.GOOS == "linux" && got != "Linux") {
		t.Errorf("platformName()=%q for GOOS=%q", got, runtime.GOOS)
	}
}

func TestPromptNamesTheSelectedInterpreterAndFenceLabelsDoNotChooseIt(t *testing.T) {
	text := strings.Join(strings.Fields(prompt(&Box{Shell: true}, "/opt/homebrew/bin/bash", "/work", "true", time.Minute, 1024, 0)), " ")
	for _, want := range []string{
		"Each block runs as '/opt/homebrew/bin/bash' -c SCRIPT",
		"The same interpreter runs the check",
		"PLY_SHELL names it",
		"bash or zsh on a fence does not select another interpreter",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestRootPromptMakesExplicitDelegationAVisibleUnixComposition(t *testing.T) {
	text := prompt(&Box{Shell: true}, defaultShell, "/work", "", 2*time.Minute, 16<<10, 0)
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
	text := prompt(&Box{Shell: true}, defaultShell, "/work", "", time.Minute, 1024, 1)
	if strings.Contains(text, "another ordinary ply process is a subagent") || strings.Contains(text, `"$PLY" -sh -C`) {
		t.Fatalf("nested prompt advertises recursion:\n%s", text)
	}
}

func TestPromptDoesNotAdvertiseUnavailableToolboxBookkeeping(t *testing.T) {
	narrow := &Box{Dir: "/tools", Tools: []Tool{{Name: "rg"}}}
	if text := prompt(narrow, defaultShell, "/work", "", time.Minute, 1024, 0); strings.Contains(text, "another ordinary ply process is a subagent") {
		t.Fatalf("narrow toolbox advertised unavailable delegation:\n%s", text)
	}
	complete := &Box{Dir: "/tools", Tools: []Tool{{Name: "mkdir"}, {Name: "mktemp"}, {Name: "mv"}}}
	if text := prompt(complete, defaultShell, "/work", "", time.Minute, 1024, 0); !strings.Contains(text, `"$PLY" -t '/tools'`) {
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

func TestRunnerExportsExplicitModelAndEffortForNestedPly(t *testing.T) {
	o := newOpts("ply")
	if err := o.fs.Parse([]string{"-m", "openai/test-model", "-effort", "xhigh"}); err != nil {
		t.Fatal(err)
	}
	r := o.runner(&Box{Shell: true}, "/bin/ply", 0, "/bin/zsh")
	if got := strings.Join(r.Env, "\n"); !strings.Contains(got, "ASK_MODEL=openai/test-model") {
		t.Fatalf("runner env=%q", got)
	}
	if got := strings.Join(r.Env, "\n"); !strings.Contains(got, "PLY_SHELL=/bin/zsh") {
		t.Fatalf("runner did not export its interpreter: %q", got)
	}
	if got := strings.Join(r.Env, "\n"); !strings.Contains(got, "PLY_EFFORT=xhigh") {
		t.Fatalf("runner did not export its reasoning effort: %q", got)
	}

	o = newOpts("ply")
	r = o.runner(&Box{Shell: true}, "/bin/ply", 0, defaultShell)
	if got := strings.Join(r.Env, "\n"); strings.Contains(got, "ASK_MODEL=") {
		t.Fatalf("runner overrode ambient model without -m: %q", got)
	}
}
