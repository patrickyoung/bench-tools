# Brief

**Give an agent the right procedure, without loading your whole library.**

Brief finds, reads, and checks Agent Skills: folders containing a `SKILL.md`
with a name, description, and Markdown instructions. Use it to share house
rules, repeat a reliable method, or keep a growing skill collection usable.

```sh
brief find 'release notes'
brief cat release-notes
```

Brief works offline. A model is optional for finding a skill by meaning;
executing the procedure belongs to your agent.

[Install](#install) · [First skill](#make-your-first-skill) ·
[Use with Ask and Ply](#put-the-procedure-to-work) · [Field guide](GUIDE.md)

## Install

Requires **Go 1.26+** and a **Unix system or WSL**. Install current `main`:

```sh
mkdir -p "$HOME/.local/bin"
GOBIN="$HOME/.local/bin" go install github.com/patrickyoung/brief@main
export PATH="$HOME/.local/bin:$PATH"
brief version
```

Keep the PATH setting in your shell startup file. No account or API key is
needed for this tutorial.

## Make your first skill

Start in a project directory. This creates a project-local skill:

```sh
brief new -d .claude/skills release-notes
```

Replace its starter text with a short, usable procedure:

```sh
cat > .claude/skills/release-notes/SKILL.md <<'SKILL'
---
name: release-notes
description: Write concise release notes from a change list. Use when announcing a release or summarizing shipped changes.
---

# Release notes

1. Lead with the change a user will notice.
2. Group the remaining changes into Added, Fixed, and Changed.
3. Explain required user action explicitly.
4. Omit empty sections and do not invent changes.
SKILL

brief lint -strict .claude/skills/release-notes
brief find 'release notes'
brief cat release-notes
```

The check should pass, `find` should print `release-notes`, and `cat` should
print the instructions without their YAML frontmatter. You now have a reusable
procedure in an ordinary, versionable file.

## Put the procedure to work

Install and configure [Ask](https://github.com/patrickyoung/ask) for your
provider, then combine its default prompt with your chosen skill:

```sh
printf '%s\n' 'Added CSV export. Fixed duplicate notifications.' > changes.txt
system=$(ask system) &&
procedure=$(brief cat release-notes) &&
ASK_SYSTEM="$system
$procedure" ask 'Write this release note.' < changes.txt
```

The shell checks both reads before calling Ask. Your file is the input, the
skill is the method, and Ask supplies the wording.

For work that needs file edits or repeated checks, use
[Ply](https://github.com/patrickyoung/ply):

```sh
ply -sh -s release-notes -check 'test -s RELEASE.md' \
  'Read changes.txt and write RELEASE.md using the release-notes procedure.'
```

This example grants shell execution. The check proves that a nonempty file
exists; review its content or supply a stronger check for publication quality.

## Find what fits, then load only what you need

| Step | Command | What is disclosed |
| --- | --- | --- |
| Browse | `brief ls` | Names and descriptions |
| Choose offline | `brief find 'release notes'` | Matching skill names |
| Choose with a model | `brief find -ask 'announce what shipped'` | Catalogue and task, never skill bodies |
| Read instructions | `brief cat release-notes` | The named skill's body |
| Inspect resources | `brief ls release-notes` | Files bundled with that skill |
| Read one resource | `brief cat release-notes/references/example.md` | That explicitly named file, if present |

Offline finding ranks names and descriptions. It can return **no match**
(exit 1), which is preferable to guessing. `-v` explains the ranking and
`-n 3` requests up to three names. `-ask` requires configured Ask, makes a
provider call, validates the returned name, and records its own replayable
session under `~/.brief/find/` by default.

## Use the skills you already have

The default search path is, in order:

```text
.claude/skills
~/.claude/skills
~/.brief/skills
```

A project skill shadows a personal skill of the same name. Inspect the winner
with `brief path release-notes`, or every location with
`brief path -a release-notes`.

For another layout:

```sh
export BRIEF_PATH="$PWD/skills:$HOME/shared-skills"
brief ls
```

`BRIEF_PATH` is colon-separated and replaces the defaults. Direct paths also
work: `brief cat ./skills/release-notes` and `brief lint ./skills`.

## Keep the library healthy

```sh
brief lint                 # validate the catalogue
brief lint -strict         # also fail on warnings
brief lint -q .claude/skills/release-notes # status only, useful in scripts
brief prompt               # inspect the model selector's system prompt
```

Lint checks the Agent Skills format, including frontmatter, names, descriptions,
and bundled references. Errors are violations; warnings are advice, such as
an overly long body or an unresolved reference. `-strict` makes warnings fail
as well. A clean skill is structurally valid; try representative tasks to
judge whether its procedure is useful.

[Hone](https://github.com/patrickyoung/hone) can add a reviewed lesson after a
Ply run failed, recovered, and passed its check. Brief then finds that same
skill on the next task. [Bench](https://github.com/patrickyoung/bench) adds an
interactive skill browser over these commands.

## Outcomes and common fixes

| Status | Meaning |
| --- | --- |
| 0 | A result, a match, or a clean check |
| 1 | No match, or lint findings; inspect stderr |
| 2 | Invalid invocation, unreadable input, or a failed dependency |

Stdout contains only the requested names, text, or findings. Diagnostics go
to stderr. If a skill is missing, check `BRIEF_PATH`, its directory name, and
its frontmatter. If offline matching returns nothing, improve the description
or deliberately use `find -ask`.

Optional settings: `BRIEF_MODEL` and `BRIEF_EFFORT` choose the model policy for
selection; `BRIEF_DIR` chooses its session root; `ASK` selects the Ask binary.
Brief has no installer, registry, execution engine, cache, or background service.

## Reference and development

```text
brief ls [ref]
brief cat ref...
brief find [flags] task
brief lint [flags] [path]
brief new [flags] name
brief path [flags] name
brief prompt [flags]
brief version
brief help
```

Continue with [GUIDE.md](GUIDE.md) or the [manual](brief.1).
Contributors: read [AGENTS.md](AGENTS.md), then run `go test ./...` and
`go test -race ./...`. See [SECURITY.md](SECURITY.md) for the trust boundary.
[MIT license](LICENSE).
