# Your first useful result

[Home](../README.md) · [Recipes](RECIPES.md) · [Full installation guide](INSTALL.md)

For creating and using digital workers or teams, start with the
[worker walkthrough](BUILD-WITH-AN-LLM.md) or give your harness
[START-HERE.md](../START-HERE.md). This smaller tutorial establishes the model
connection and basic command notation used by those workflows.

Turn a few meeting notes into a short team update with Ask. Once that works,
add Brief to save the method for next time. You do not need to know Bash to
follow along. Prefer a complete small program? Try a [ready-to-run
starter](../examples/README.md); the signup audit works entirely offline.

## 1. Install Ask

Open a terminal in this checkout's `bench-tools` directory. You should see
`README.md`, `components.json`, and `scripts/`. You need Go 1.26+, Python 3.9+,
Git, and a Unix shell (macOS, Linux, or WSL).

```sh
python3 scripts/install ask
export PATH="$HOME/.local/bin:$PATH"
ask version
```

The installer builds your copy under `~/.local`. `PATH` lists the directories
the terminal searches for commands; `export` passes a setting to programs you
launch. The version command should print Ask's version. If installation fails,
fix that error before continuing. For an existing-command conflict, use an
[alternate prefix](INSTALL.md#install-a-few-tools-or-the-whole-toolkit).

Keep the PATH line in your shell startup file for future terminals. You can
also [build without installing](INSTALL.md#try-without-installing).

## 2. Connect a model

Choose a model ID available to your provider account. For Anthropic, replace
both placeholders before running:

```sh
export ASK_MODEL='anthropic/YOUR_MODEL_ID'
export ANTHROPIC_API_KEY='YOUR_API_KEY'
```

The key belongs in your local environment, not in a committed script. These
settings last for the current terminal session. Other providers and OAuth use
[Ask's documented configuration](../tools/ask/README.md#install). Calls use
your provider account and may incur charges. `YOUR_MODEL_ID` is a placeholder,
not a built-in default.

## 3. Turn notes into an update

Create a fresh practice directory. If `bench-demo` already exists, choose
another name so the examples do not overwrite earlier work.

```sh
mkdir bench-demo
cd bench-demo
cat > notes.txt <<'NOTES'
Decision: move the customer demo to Thursday.
Sam will update the invitation by Tuesday.
The setup guide still needs a reviewer. No owner was assigned.
NOTES

ask -q -f update.jsonl 'Summarize these notes with Decisions and Next steps.
Preserve names and dates. Say "Owner needed" for actions without an owner.' \
  < notes.txt > update.tmp.md &&
mv update.tmp.md update.md &&
cat update.md
```

Your update should preserve Thursday's demo, Sam's Tuesday deadline, and the
missing reviewer. Wording varies by model; compare the result with the notes.

`mkdir` makes a directory; `cd` enters it. Paste the whole `cat` block, including
the final `NOTES` line: it writes the lines between the markers literally into
`notes.txt`. The closing marker must be on its own line.

`< notes.txt` sends the file's contents to Ask. A filename in the prompt alone
would not let Ask read it. `>` saves the answer; `&&` continues only if the
previous command succeeded. `mv` replaces `update.md` only after Ask succeeds.
Errors stay visible in the terminal. If a call fails, inspect them and the
temporary file before trying again.

You now have the facts in `notes.txt`, a draft in `update.md`, and a retained
conversation in `update.jsonl`. Inspect the conversation without another call:

```sh
ask replay update.jsonl
ask replay -check update.jsonl
```

Replay verification checks the record's internal consistency, not whether the
summary is accurate. The log contains your notes and answer. Use a new log
filename for an independent meeting: `-f update.jsonl` continues this one.

## 4. Save the method with Brief

Add Brief when the same kind of update becomes a recurring task. From your
practice directory, the checkout's installer is one directory above:

```sh
python3 ../scripts/install brief
brief new -d ./skills team-update
cat > skills/team-update/SKILL.md <<'SKILL'
---
name: team-update
description: Turn meeting notes into decisions and next steps. Use when summarizing a meeting or preparing a team update.
---

# Team update

1. Write two sections: Decisions and Next steps.
2. Use short bullets. Preserve names and dates exactly as supplied.
3. For each next step, name its owner. Say "Owner needed" when absent.
4. Do not invent a decision, deadline, or commitment.
SKILL

brief lint -strict ./skills/team-update
brief cat ./skills/team-update
```

These Brief commands work offline. Lint checks the skill's format; cat prints
the procedure without its metadata. Brief has not followed the procedure or
called a model. To apply it to your existing conversation:

```sh
system=$(ask system) &&
procedure=$(brief cat ./skills/team-update) &&
ASK_SYSTEM="$system
$procedure" ask -q -f update.jsonl 'Rewrite the update using this procedure.' \
  > update.tmp.md &&
mv update.tmp.md update.md &&
cat update.md
```

`$(command)` captures output in a variable. The first two lines load Ask's
normal instructions and your procedure; `ASK_SYSTEM=... ask` combines them for
this invocation. The newline inside the quotes is intentional. The procedure
guides the model; review that the facts still match your notes.

For a reusable command with fresh logs and output preserved on failure, copy
the complete [meeting-brief starter](../examples/meeting-brief/README.md).

## What to try next

- **Use your own input:** replace the notes and choose a new conversation file.
- **Reuse a worker or team:** inspect the [worker catalog](../workers/README.md)
  and [team catalog](../teams/README.md), export a reviewed version and supply
  fresh inputs. The [walkthrough](BUILD-WITH-AN-LLM.md) shows both execution modes.
- **Build your own worker:** the [LLM builder guide](BUILD-WITH-AN-LLM.md) shows
  Hire producing a reusable definition for Agent. Start with the job and its
  acceptance examples; reuse the existing runner.
- **Let the model edit files:** install Ply from the checkout root with
  `python3 scripts/install ply`, then try the [checked repair recipe](RECIPES.md#3-fix-a-small-tool-and-make-done-executable).
  Ply adds actions and a stopping check; Ask alone produces an answer.
- **Understand the pieces:** [How it works](HOW-IT-WORKS.md) explains where
  instructions, evidence, checks, and durable jobs belong.

## If something goes wrong

| Symptom | First thing to check |
| --- | --- |
| `command not found` | Repeat the PATH line; run `command -v ask` to see the selected binary |
| `go` or `python3` is missing or too old | Install the prerequisites listed above, then rerun installation |
| No model or a provider error | Replace the model placeholder; check the matching provider key and account access |
| A file cannot be found | Run `pwd` and `ls`; these examples use paths relative to `bench-demo` |
| The procedure cannot be found | Use the exact path `./skills/team-update`, including `./` |
| The result contains extra old facts | Start a new named session; `-f` continues an existing one |
| Ask exits with status 2 | Its context window is full; start a new session or follow [Ask's compaction guide](../tools/ask/GUIDE.md) |

Run `echo $?` immediately after a command to see its numeric exit status.
For Ask, 0 means the call completed, 1 means an error, and 2 means full context.
Other tools assign their own meanings to nonzero statuses; their manuals say
which ones are expected outcomes and which ones mean something broke.
