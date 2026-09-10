# Draft

**Turn an idea into a design you can discuss, build, and check.**

Draft helps you decide which parts of a system need ordinary code, which need
a model, and how you will know the result works. The output is a `DESIGN.md`
with an executable check. Once you have reviewed it, the same check guides the
build through Ply.

```text
Describe → Review DESIGN.md → Check the design → Build → Test the check itself
```

Draft is a shell program that composes Ask, Brief, Ply, and Hone. The useful
part is the agreement it keeps explicit: what should happen, what should not,
and what command decides whether the work is done.

## Install

The [Bench suite](https://github.com/patrickyoung/bench#install) includes Draft
and compatible dependencies. For a direct current-`main` source install, use
**Git, Go 1.26+, a Unix shell, and Perl**:

```sh
git clone https://github.com/patrickyoung/draft.git
cd draft
mkdir -p "$HOME/.local/bin" "$HOME/.claude/skills"
for tool in ask brief ply hone; do
  GOBIN="$HOME/.local/bin" go install "github.com/patrickyoung/$tool@main"
done
ln -s "$PWD/bin/draft" "$HOME/.local/bin/draft"
ln -s "$PWD/skills/draft" "$HOME/.claude/skills/draft"
export PATH="$HOME/.local/bin:$PATH"
draft sync
draft version
```

Choose one installation method. If Draft or its skill already exists, inspect
that installation before replacing links. Keep the checkout in place and the
PATH setting in your shell startup file.

`draft sync` writes a reference from the installed tools' help and versions.
Run it again after changing those tools. Configure
[Ask](https://github.com/patrickyoung/ask#install) before model-backed design
or build work; Draft owns no provider credentials.

## Start without a model

```sh
draft new notes-tool
```

You get `notes-tool/DESIGN.md`, a template with the questions you need to
answer. Open it in your editor. Describe the goal, inputs, outputs, requirements,
limits, and the split between scripts and model judgment.

Then try:

```sh
draft check notes-tool
```

An untouched template should be refused. Its `false` check and unfinished
requirements are deliberate reminders that there is no usable agreement yet.
This validation does not call a model.

## Let a model draft the first version

Use a different, new directory and a concrete description:

```sh
draft new release-notes \
  'Read a local changes.txt and produce a concise RELEASE.md. Preserve the input, work offline, and check the required sections.'
```

Draft calls Ask with its own skill and records the design conversation under
`release-notes/.draft/`. Read the result, correct assumptions, and replace weak
checks with ones that actually cover the requirements.

```sh
draft check release-notes
```

On success, stdout contains the shell check extracted from `## Check`. That
means the design is buildable according to Draft's structural rules. It does
not mean the program has been built, or that the check is a good specification.

## Build against the agreement

After reviewing the design and its executable check:

```sh
draft build release-notes
```

Draft passes the design to Ply and the exact check to `-check`. Ply runs shell
commands and iterates until the check accepts or a limit/failure stops it.
Read the resulting files and the check output before relying on the result.

The normal build grants the builder ordinary shell authority. For a check
whose own bytes must stay outside the builder's write access, use the admitted
workflow below.

## Test whether the check notices broken behavior

After the clean program passes:

```sh
draft prove -n 20 release-notes
```

This is mutation testing: Draft changes selected boundary/equality operations,
runs the check, and reports which changes it failed to notice. No model is
called. A surviving mutation points to a case worth examining; some mutations
are equivalent, so the score is a signal rather than a target.

Prove temporarily changes source while measuring, journals originals, restores
them, and bounds each check in a process group. Run it in a quiet checkout with
no concurrent editor or worker changing the same files. An interrupted journal
is recovered on the next run.

## Freeze a reviewed verifier

Install [May](https://github.com/patrickyoung/may) and
[Cage](https://github.com/patrickyoung/cage), and prove Cage on your host first:

```sh
draft admit release-notes
draft build -admitted release-notes
```

Admission asks a person to approve the exact project path and normalized Check
bytes. Only then are those bytes stored under `$XDG_STATE_HOME/draft/verifiers`
or `~/.local/state/draft/verifiers`, outside the project and temporary roots.
An admitted build refuses stale or missing approval evidence.

The builder runs through `cage -net -w PROJECT`: project and temporary writes
are allowed; the stored verifier is read-only; networking is deliberately
allowed for Ask's provider connection. Host reads remain unrestricted.

Freezing a shell command does not freeze files it calls. Put the substantive
assertions in the admitted block or other operator-controlled programs, rather
than handing a writable `bin/check` the final say.

## Choose the smallest useful component

| Work | Starting point |
| --- | --- |
| Fetch, parse, move files, append, schedule | An ordinary script or program |
| Summarize, classify, interpret | [Ask](https://github.com/patrickyoung/ask) |
| Apply a written method | [Brief](https://github.com/patrickyoung/brief) |
| Iterate until an executable check passes | [Ply](https://github.com/patrickyoung/ply) |
| Learn from a verified recovery | [Hone](https://github.com/patrickyoung/hone) |
| Give recurring work a persistent home | [Agent](https://github.com/patrickyoung/agent) |
| Work through the design/build flow interactively | [Bench](https://github.com/patrickyoung/bench) |

Most systems contain plenty of work that does not need model judgment. Draft's
split table makes that visible before you pay for a model to do it repeatedly.

## Troubleshooting and reference

| Situation | Next step |
| --- | --- |
| Template/check refused | Finish the requirements and write a meaningful `## Check` |
| Tool reference is stale | Run `draft sync` after updating dependencies |
| Build returns not done | Read Ply's feedback and the design/check before retrying |
| Prove reports survivors | Inspect those mutations and strengthen missing assertions |
| Admission is stale | Review and admit the current exact check again |

`draft check` uses 0 for buildable, 1 for not buildable, and 2 for operational
failure. Build and prove have their own outcome meanings: a build's 2 can mean
Ply is unfinished; prove's 1 means gaps were found. Admission preserves 3/75
for declined/pending May decisions, and admitted boundary failure uses 125.

```text
draft new DIR [description ...]
draft check [DIR]
draft build [-admitted] [DIR]
draft prove [-n N] [DIR]
draft admit [DIR]
draft sync
draft tools
draft version
draft help
```

See [DESIGN.md](DESIGN.md), including Draft's own check. For development, run
`draft sync` to refresh the installed tool reference, then
`sh -n bin/draft`, `sh bin/draft_test.sh`, and
`brief lint -strict skills/draft` with the four dependencies on PATH.
[MIT license](LICENSE).
