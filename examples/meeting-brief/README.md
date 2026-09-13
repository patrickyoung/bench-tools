# Meeting notes to a reusable brief

The meeting moved the pilot, assigned two actions, and left support coverage
unassigned. A useful brief keeps those details visible instead of smoothing
them into a vague “next steps” paragraph.

This starter uses Brief to read a checked procedure and Ask to write one draft.
It preserves an existing output if the model call fails or returns no text.

Install Ask and Brief from the checkout root, and [configure
Ask](../../docs/GETTING-STARTED.md#2-connect-a-model). Then, from the checkout root:

```sh
python3 scripts/install ask brief
export PATH="$HOME/.local/bin:$PATH"
practice=$(mktemp -d)
cp -R examples/meeting-brief "$practice/meeting-brief"
cd "$practice/meeting-brief"
python3 run.py
cat brief.md
```

This makes one model invocation, which may incur provider charges. The result
should contain Decisions, Actions, and Open questions; retain the September
21 pilot date, Maya's September 14 deadline, Luis's September 10 deadline, and
the unassigned support question. Review those facts against `notes.txt`.

Change the input and procedure, or supply your own paths:

```sh
python3 run.py notes.txt another-brief.md
```

Each invocation prints a fresh conversation path on stderr and retains its
candidate in the same `runs/` directory beside the output. Use the exact path
printed by the run with `ask replay -check PATH` to verify its record. Repeated
runs start fresh conversations and replace the final brief only on success.

The procedure guides wording; the wrapper checks command success and nonempty
output, not factual accuracy. Its 90-second Ask timeout bounds the direct
process; Ask may retry provider failures internally. Sessions contain the
notes and answer. The programs do not execute model-written commands.

This is enough for a one-call draft. The small caller owns fresh filenames and
output replacement; Brief owns the procedure and Ask the model request. If
the job needs files edited or a rejected result corrected, use the
[expert-folder starter](../support-reply/README.md) to reuse Agent's runner.
