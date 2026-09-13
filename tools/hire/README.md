# Hire

**Build an expert folder. Run it separately with Agent.**

Hire is a headless Unix command. `hire build` runs its builder expert through
the installed Agent command. That expert writes instructions, skills, memory,
optional specialists and an executable acceptance check into a reusable folder.
Agent remains the only runtime; Hire adds no provider client or goal loop.

```sh
mkdir authoring names-work
hire build -C authoring -evidence authoring-evidence -- \
  'Build an expert that sorts and deduplicates names from names.txt into sorted.txt.'

agent show -C names-work authoring/expert
printf '%s\n' pear apple pear banana > names-work/names.txt
agent run -C names-work -evidence names-evidence authoring/expert -- \
  'Normalize the supplied names.'
```

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

This uses the extracted Agent scaffolder, without running a model. Edit the
definition and replace the deliberately unfinished `bin/check`. `hire new
-home HOME` retains the old recurring-home layout for existing workflows.

`hire verify EXPERT` checks the folder through `agent check`, requires a
bounded nonempty README, and rejects runtime directories inside the definition.
It does **not** execute generated checks or claim that structural readiness
proves the worker's quality. Inspect its check and exercise its positive and
negative examples before relying on its verdict.

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

From the monorepo, use `python3 scripts/build agent hire` and the existing
installer. `HIRE_AGENT` selects the exact runtime executable. Keep the other
installed Bench commands on PATH. There is no required web application.

This is a new development interface, not an update to existing pinned Hire
web installations. Those installations still require their compatible Agent
version; their `agent new/learn/history` call sites must move to Hire before a
release cutover. The retained web checkout and its data have not been migrated.

Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and
`sh -n builder/home.sh expert/bin/check`. The monorepo integration suite proves
Hire builds an expert through Agent and a separate Agent invocation runs that
generated definition with fresh context and a different workspace.
