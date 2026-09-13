# Hire

**Build an expert folder. Run it separately with Agent.**

Hire is a headless Unix command. `hire build` runs its builder expert through
the installed Agent command. That expert writes instructions, skills, memory,
optional specialists and an executable acceptance check into a reusable folder.
Agent remains the only runtime; Hire adds no provider client or goal loop.

The useful output is a specialty you can inspect and reuse. A support expert
needs a policy and a writing method; a report expert may need a renderer and
a check. Both can use the same runner. Keep their specialist knowledge in
files and their deterministic work in ordinary programs.

## Install

From the [Bench tools monorepo](https://github.com/patrickyoung/bench-tools)
root, with Go 1.26+, Python 3.9+, Git, and a Unix shell:

```sh
python3 scripts/install hire agent ask brief ply cage
export PATH="$HOME/.local/bin:$PATH"
hire version
cage check
```

[Configure Ask](https://github.com/patrickyoung/bench-tools/blob/main/docs/GETTING-STARTED.md#2-connect-a-model)
before model-backed building. `hire new` works without a model; `hire build`
uses your provider account. The source installer is Python; Hire and Agent
are native Go commands with no Python runtime requirement.

## Describe the job, then inspect the folder

Use a fresh practice directory:

```sh
mkdir authoring customer-work
hire build -C authoring -evidence authoring-evidence -- \
  'Build a support expert that reads question.txt and policy.txt, writes
reply.md, and flags questions the policy does not answer. Keep the reply
under 200 words. Include a reusable writing skill, an executable check,
and examples showing both an acceptable reply and a plausible wrong one.
Explain which requirements the check proves and which need human review.'

hire verify authoring/expert
agent show -C customer-work authoring/expert
```

Inspect `authoring/expert/README.md`, the instructions and skill, and especially
`bin/check`. Hire verifies structure without executing generated checker code.
An instruction that says “be accurate” is not an accuracy test.

Now use a fictional customer case:

```sh
printf '%s\n' 'Can I export before closing? How long does it take?' > customer-work/question.txt
printf '%s\n' 'Owners can export CSV while their subscription is active. Exports are unavailable after closure. Export duration is not specified.' > customer-work/policy.txt
agent run -C customer-work -evidence customer-evidence authoring/expert -- \
  'Draft reply.md for this customer.' > run-summary.txt
cat customer-work/reply.md
```

The reply should explain export timing and acknowledge the missing duration.
Review it against the policy. A different question gets a fresh workspace,
while the expert definition stays reusable. The complete
[support-reply starter](https://github.com/patrickyoung/bench-tools/tree/main/examples/support-reply)
is a ready-made alternative with exact citation checking; use it to learn the
folder layout before generating your own.

## Inputs, outputs, and limits

Use the usual Ask model configuration or pass `-m`. A private job description
can come from `-goal-file FILE` or stdin. Additional piped input is evidence
when an explicit goal is present. The answer goes to stdout, progress to
stderr, and files to the workspace. Status is Agent's exact outcome; unfinished
builds exit 2 and interruptions exit 130. Nothing retries automatically.

The build workspace must already exist. The result is `WORKSPACE/expert`.
`-evidence DIR` selects controller records outside the workspace; otherwise
Hire uses `$HIRE_DIR/KEY` or `~/.hire/KEY`, with KEY derived from the physical
build workspace. State, model, effort, checkpoint and limit flags pass to Agent.
Each build starts a work session so a new request can revise an already valid
expert. `-B=false` explicitly permits a passing pre-check to skip model work.
Default model actions use Agent's Cage boundary. `-net` and `-no-cage` are
explicit caller selections with the same meaning as in Agent.

## Start without a model

```sh
hire new experts/reviewer 'Review a supplied result against its requirements'
```

This creates a definition without running a model. Edit the
definition and replace the deliberately unfinished `bin/check`. `hire new
-home HOME` retains the old recurring-home layout for existing workflows.

`hire verify EXPERT` checks the folder through `agent check`, requires a
bounded nonempty README, and rejects runtime directories inside the definition.
Agent also checks nested specialist definitions and their Brief procedures.
It does **not** execute generated checks or claim that structural readiness
proves the worker's quality. Inspect its check and exercise its positive and
negative examples before relying on its verdict.

## Assemble a team when the job needs one

The embedded builder includes an `assembling-experts` skill. Give Hire the
outcome, available capabilities, requested interfaces, and acceptance cases.
It can assemble a manager and independently reusable specialists under
`expert/agents/`, with explicit artifact handoffs and checks. The result is
still an ordinary expert definition; Hire does not manage its later runs.

Agent runs the experts, Weave selects ready work, and Tend retains attempts.
Use an existing external controller when child execution cannot occur inside
the manager's action boundary. MCPserve and A2Aserve provide the requested
protocol edges. A valid assembly needs real evaluation of its contributions
and integrated result; structural verification does not establish quality.

The monorepo's [page-team example](https://github.com/patrickyoung/bench-tools/tree/main/examples/page-team)
contains a manager, graphics/frontend/review specialists, executable contract
tests, MCP/A2A adapters, and an evaluated single-file EPL scouting showcase.
Its evaluation records the operator repairs as well as successful runs.

## Existing home maintenance

The former Agent authoring/controller commands are now available under Hire:
`learn`, `history`, `actions`, `act`, `proposals`, and `amend`. Their arguments
and receipt formats are retained. They continue to use Hone, Trail, Action and
May through public commands. These commands operate on existing recurring
homes; use the underlying tools directly for separately selected portable
state and evidence. Agent itself offers execution and read-only inspection.

## Source and installation

This headless split lives in the Bench tools monorepo at `tools/hire`. It keeps
Hire's existing module identity, `github.com/patrickyoung/bench-hire`, and is
built independently with `go build .`. The source and license of Agent's
existing home-maintenance implementation are retained here; the builder expert
adapts Hire's platform, evidence and authority guidance into Markdown. The old
web router, worker store and expert-team loops are not included.

For a local build without installing, use `python3 scripts/build agent hire`.
`HIRE_AGENT` selects the exact runtime executable. Keep the other
installed Bench commands on PATH. There is no required web application.

This is a new development interface, not an update to existing pinned Hire
web installations. Those installations still require their compatible Agent
version; their `agent new/learn/history` call sites must move to Hire before a
release cutover. The retained web checkout and its data have not been migrated.

Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and
`sh -n builder/home.sh expert/bin/check`. The monorepo integration suite proves
Hire builds an expert through Agent and a separate Agent invocation runs that
generated definition with fresh context and a different workspace.
