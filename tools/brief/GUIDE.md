# The Brief field guide

A procedure should be reusable wherever the work happens. Brief makes finding
and reading it ordinary commands: select a name, read the body, and pass the
instructions to the program that will do the job.

Start with [installation and your first skill](README.md#make-your-first-skill).
This guide assumes Brief is on PATH. Examples that call Ask need a configured
model; finding, reading, and linting normally work offline.

## Save the method, not a past answer

A release-note skill should say how to explain a change, what to preserve,
and when to omit a section. Last week's release note is evidence or an
example, not the procedure itself.

An Agent Skill is a directory containing `SKILL.md`, with a name, description,
and Markdown body. References and scripts can live beside it. Brief reads
those files; it never executes a bundled script. A model with command access,
such as Agent through Ply, can follow a procedure that needs tools. Ask alone
can only reason over the material its caller supplies.

## Bring the catalogue you already have

The default search order is project `.claude/skills`, personal
`~/.claude/skills`, then `~/.brief/skills`. For another layout, select it:

```sh
export BRIEF_PATH="$PWD/skills:$HOME/shared-skills"
brief ls
brief path -a release-notes
```

The first matching name wins. `path -a` shows shadowed copies, which is useful
when an edit seems to have no effect. `BRIEF_PATH` replaces the defaults;
include each location you intend to search.

You can write a skill, copy a reviewed folder, or check out a catalogue with
Git. Brief has no `install` verb or package registry. Inspect imported skills
with `brief lint -strict PATH` before relying on them. Lint is a format check,
not a judgment that a script or instruction should receive authority.

A direct path is useful while authoring:

```sh
brief new -d ./skills release-notes
brief cat ./skills/release-notes
brief lint -strict ./skills/release-notes
```

Edit the scaffold using the [first-skill example](README.md#make-your-first-skill)
before expecting it to express your team's method.

## Choose cheaply, load deliberately

```sh
brief find 'release notes'
brief find -v -n 3 'announce what shipped'
brief cat release-notes
```

Offline matching ranks task words against names and descriptions. `-v` explains
its choices on stderr; stdout still contains names only. No match is exit 1.
It is an expected result, not a broken selector.

A description that says only “communications” may miss “release announcement.”
Use the words people use for the job. When vocabulary is too different, choose
with a model explicitly:

```sh
brief find -ask 'explain the latest improvements to our customers'
```

That call sends the catalogue's names and descriptions plus the task to Ask.
It does not disclose every skill body. Brief validates the returned name and
records the selection in its own Ask session. A model call can cost money and
its duration depends on the provider, model, and request.

`BRIEF_MODEL` and `BRIEF_EFFORT` set the selection policy; unset values keep
Ask's defaults. Select a model available to your account and test whether it
chooses correctly on your tasks before optimizing for price or latency.

## Put one procedure into one model request

With the release-notes skill installed and Ask configured:

```sh
printf '%s\n' 'Added CSV export. Fixed duplicate notifications.' > changes.txt
system=$(ask system) &&
procedure=$(brief cat release-notes) &&
ASK_SYSTEM="$system
$procedure" ask 'Write the release note.' < changes.txt
```

The two reads must succeed before Ask starts. The newline inside the quoted
assignment separates the default prompt from the procedure. The change list
remains input data. Use `brief cat house-style release-notes` to combine two
installed procedures in that explicit order.

For a frequently used shell function, distinguish “no skill” from “the
selector broke”:

```sh
skilled() (
  task=$1
  if selected=$(brief find "$task" < /dev/null); then
    system=$(ask system) || exit "$?"
    procedure=$(brief cat "$selected") || exit "$?"
    ASK_SYSTEM="$system
$procedure" ask "$task"
  else
    skill_status=$?
    case "$skill_status" in
      1) ask "$task" ;;
      *) exit "$skill_status" ;;
    esac
  fi
)

skilled 'Write release notes.' < changes.txt
```

This starts a fresh Ask conversation. It falls back to plain Ask only for
Brief's exit 1; unreadable skills or a broken dependency must not silently
remove the procedure. Selection reads no task evidence from stdin, leaving it
for the writer. Use a named `ask -f SESSION` deliberately when continuity is
part of the job; do not make unrelated requests share one conversation.

## Let the existing runner apply a tool-using skill

A procedure that says “read the source, edit the file, run the check” needs an
action loop. Give it to Ply:

```sh
ply -sh -s release-notes -turns 8 -check 'test -s RELEASE.md' \
  'Read changes.txt and write RELEASE.md using the release-notes procedure.'
```

The named skill must exist. `-sh` grants ordinary shell execution. This small
check only proves a nonempty file exists; content quality needs review or a
stronger check. Ply records which skill it loaded and whether a candidate
passed the verifier.

For a reusable specialist, put the skill at
`EXPERT/skills/release-notes/SKILL.md`. Agent already uses Brief for validation
and selection. [Hire](https://github.com/patrickyoung/bench-tools/tree/main/tools/hire)
can build the folder, and
[Agent](https://github.com/patrickyoung/bench-tools/tree/main/tools/agent) runs
it against each workspace. The
[support-reply starter](https://github.com/patrickyoung/bench-tools/tree/main/examples/support-reply)
shows this composition with actual inputs and a citation check.

## Keep references available without loading everything

```sh
brief ls release-notes
brief cat release-notes/references/example.md
```

The second command requires that resource to exist. It loads just the named
file. A skill can tell an agent when a reference is needed; Brief does not
follow every link and inject its contents into every request.

For Ask, the caller must supply needed resources itself, just as it supplies
the input document. For Agent, the worker can use its tools to read them.
This distinction matters for a PDF procedure with an extraction script:
loading the skill does not give Ask a filesystem or execute that script.

## Inspect a selection and improve the description

`brief find -ask` prints a replay command on stderr. Run it against the exact
session it names, or inspect the default selection archive with Trail:

```sh
trail ls "$HOME/.brief/find"
trail check "$HOME/.brief/find"
```

Install Trail and Ask for this inspection. `BRIEF_DIR` can select another
archive root. Replay checks retained-record consistency, not whether the
choice was appropriate. Keep selection records when you need to explain why
a procedure was chosen; removing them removes that evidence.

Try the phrases you actually expect:

```sh
for task in 'release notes' 'announce what shipped' 'customer update'; do
  printf '%s\n' "$task" >&2
  brief find -v "$task"
done
```

Inspect misses rather than treating every nonzero result as a crash. A useful
description names both the work and the circumstances in which it applies.
Avoid stuffing unrelated keywords into it: that makes the skill easier to
find for the wrong task.

## Keep the catalogue usable

```sh
brief lint
brief lint -strict
brief lint -q ./skills/release-notes
```

Errors cover invalid structure such as mismatched names, duplicate metadata
keys, missing referenced files, and an empty body. Warnings draw attention to
issues such as overly long instructions. `-strict` makes warnings fail too.
The manual describes the exact supported fields and limits.

Keep the main procedure short enough to use. Put occasional detail in named
references. Add an example where the procedure should apply and one where it
should not. After a checked recovery, Hone can propose a small, traceable
lesson; review it before making it part of future instructions.

## Outcomes

| Exit | Meaning |
| --- | --- |
| 0 | A result, match, or clean check |
| 1 | No match, or lint findings |
| 2 | Bad usage, unreadable input, or a failed dependency |

Brief uses a different contract from Ask, whose exit 2 means full context.
Check the status of the program you actually invoked. See the [README](README.md)
for setup and [brief.1](brief.1) for the complete command reference.
