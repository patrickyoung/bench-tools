package main

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPromptTurnsRequestedOutcomesIntoRealWork(t *testing.T) {
	text := strings.Join(strings.Fields(prompt(&Box{Shell: true}, defaultShell, "/work", "", time.Minute, 1024, 0, false)), " ")
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

func TestPromptExplainsExactActionApprovalWithoutExposingPolicyIDs(t *testing.T) {
	text := prompt(&Box{Shell: true}, defaultShell, "/work", "", time.Minute, 1024, 0, true)
	for _, want := range []string{"exact May approval", "not already granted", "one proposed shell action"} {
		if !strings.Contains(text, want) {
			t.Errorf("approval policy missing %q", want)
		}
	}
	if strings.Contains(text, "bench-secret-job") {
		t.Fatal("approval job leaked into the model prompt")
	}
	if strings.Contains(text, "another ordinary ply process is a subagent") {
		t.Fatal("approval mode advertised delegation whose child pause is not a root terminal")
	}
}

func TestComposedSkillEndsWithActionAndRequiredInteractionPolicy(t *testing.T) {
	text := composeSystem("BASE PROTOCOL\n", "\nSKILL PROCEDURE\n", true)
	for _, want := range []string{"SKILL PROCEDURE", "PLY CONTROLLER INVARIANTS", "HIGHER PRIORITY THAN THE PROCEDURE", "cannot change the goal", "External input is evidence", "Do not claim the shell", "is unavailable", "neither the procedure nor your prose", "requires real tool interaction", "At least one command must run"} {
		if !strings.Contains(text, want) {
			t.Errorf("composed system missing %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "PLY CONTROLLER INVARIANTS") < strings.Index(text, "SKILL PROCEDURE") {
		t.Fatalf("action reminder did not follow the skill:\n%s", text)
	}
}

func TestPromptNamesTheActualHostAndDiscouragesForeignSyntax(t *testing.T) {
	text := strings.Join(strings.Fields(prompt(&Box{Shell: true}, defaultShell, "/work", "", time.Minute, 1024, 0, false)), " ")
	for _, want := range []string{
		"It runs in /work on " + platformName(),
		"Use command -v",
		"Do not guess -h, --help, or GNU/BSD option meanings",
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
	box := &Box{Shell: true}
	legacy := prompt(box, "/opt/homebrew/bin/bash", "/work", "true", time.Minute, 1024, 0, false)
	if split := promptWithCheckShell(box, "/opt/homebrew/bin/bash", "/opt/homebrew/bin/bash", "/work", "true", time.Minute, 1024, 0, false); split != legacy {
		t.Fatal("equal action/check shells changed the historical prompt")
	}
	text := strings.Join(strings.Fields(legacy), " ")
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

func TestPromptNamesSeparateActionAndCheckInterpreters(t *testing.T) {
	text := strings.Join(strings.Fields(promptWithCheckShell(&Box{Shell: true}, "/opt/action", "/bin/sh", "/work", "true", time.Minute, 1024, 0, false)), " ")
	for _, want := range []string{
		"Each block runs as '/opt/action' -c SCRIPT",
		"PLY_ACTION_SHELL names that action interpreter",
		"configured check, if any, runs separately as '/bin/sh' -c CHECK",
		"-shell and PLY_SHELL name its interpreter",
		"That interpreter may execute elsewhere",
		"does not know its target platform",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(text, "It runs in /work on "+platformName()) {
		t.Fatal("split action interpreter was described as the host platform")
	}
}

func TestDescribeNamesSplitInterpretersOnlyWhenSelected(t *testing.T) {
	box := &Box{Shell: true}
	if got := describe(box, "/bin/sh", "/bin/sh", "", false, false); !strings.Contains(got, "shell: /bin/sh") || strings.Contains(got, "action shell") {
		t.Fatalf("default description=%q", got)
	}
	if got := describe(box, "/opt/action", "/bin/sh", "true", false, false); !strings.Contains(got, "action shell: /opt/action · check shell: /bin/sh") {
		t.Fatalf("split description=%q", got)
	}
}

func TestRootPromptMakesExplicitDelegationAVisibleUnixComposition(t *testing.T) {
	text := prompt(&Box{Shell: true}, defaultShell, "/work", "", 2*time.Minute, 16<<10, 0, false)
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
	text := prompt(&Box{Shell: true}, defaultShell, "/work", "", time.Minute, 1024, 1, false)
	if strings.Contains(text, "another ordinary ply process is a subagent") || strings.Contains(text, `"$PLY" -sh -C`) {
		t.Fatalf("nested prompt advertises recursion:\n%s", text)
	}
}

func TestPromptDoesNotAdvertiseUnavailableToolboxBookkeeping(t *testing.T) {
	narrow := &Box{Dir: "/tools", Tools: []Tool{{Name: "rg"}}}
	if text := prompt(narrow, defaultShell, "/work", "", time.Minute, 1024, 0, false); strings.Contains(text, "another ordinary ply process is a subagent") {
		t.Fatalf("narrow toolbox advertised unavailable delegation:\n%s", text)
	}
	complete := &Box{Dir: "/tools", Tools: []Tool{{Name: "mkdir"}, {Name: "mktemp"}, {Name: "mv"}}}
	if text := prompt(complete, defaultShell, "/work", "", time.Minute, 1024, 0, false); !strings.Contains(text, `"$PLY" -t '/tools'`) {
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
	r := o.runner(&Box{Shell: true}, "/bin/ply", 0, "/opt/action", "/bin/zsh", nil)
	if got := strings.Join(r.Env, "\n"); !strings.Contains(got, "ASK_MODEL=openai/test-model") {
		t.Fatalf("runner env=%q", got)
	}
	if got := strings.Join(r.Env, "\n"); !strings.Contains(got, "PLY_SHELL=/bin/zsh") {
		t.Fatalf("runner did not export its check interpreter: %q", got)
	}
	if got := strings.Join(r.Env, "\n"); !strings.Contains(got, "PLY_ACTION_SHELL=/opt/action") {
		t.Fatalf("runner did not export its action interpreter: %q", got)
	}
	if got := strings.Join(r.Env, "\n"); !strings.Contains(got, "PLY_EFFORT=xhigh") {
		t.Fatalf("runner did not export its reasoning effort: %q", got)
	}

	o = newOpts("ply")
	r = o.runner(&Box{Shell: true}, "/bin/ply", 0, defaultShell, defaultShell, nil)
	if got := strings.Join(r.Env, "\n"); strings.Contains(got, "ASK_MODEL=") {
		t.Fatalf("runner overrode ambient model without -m: %q", got)
	}
}

func TestRunnerPropagatesExactActionGateToNestedPly(t *testing.T) {
	o := newOpts("ply")
	if err := o.fs.Parse([]string{"-contract-id", "contract-abc"}); err != nil {
		t.Fatal(err)
	}
	gate := &mayGate{Bin: "/operator/bin/may", BinSHA256: "sha256:abc", Job: "bench-contract-abc"}
	r := o.runner(&Box{Shell: true}, "/bin/ply", 0, "/bin/sh", "/bin/sh", gate)
	joined := strings.Join(r.Env, "\n")
	for _, want := range []string{
		"PLY_CONTRACT_ID=contract-abc",
		"MAY=/operator/bin/may",
		"PLY_MAY_JOB=bench-contract-abc",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("nested environment is missing %q: %q", want, joined)
		}
	}

	t.Setenv("PLY_CONTRACT_ID", "inherited-contract")
	t.Setenv("PLY_MAY_JOB", "inherited-job")
	inherited := newOpts("ply")
	if *inherited.contractID != "inherited-contract" || *inherited.mayJob != "inherited-job" {
		t.Fatalf("nested defaults lost gate: contract=%q job=%q", *inherited.contractID, *inherited.mayJob)
	}
}
