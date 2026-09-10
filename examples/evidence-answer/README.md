# Answer from saved evidence

This starter passes fictional policy records through Context, gives the
normalized evidence to Ask as input data, and checks the answer with Cite.
The example URLs identify sample sources; the starter does not fetch them.

Install the commands and [configure
Ask](../../docs/GETTING-STARTED.md#2-connect-a-model). From the checkout root:

```sh
python3 scripts/install ask context cite
export PATH="$HOME/.local/bin:$PATH"
practice=$(mktemp -d)
cp -R examples/evidence-answer "$practice/evidence-answer"
cd "$practice/evidence-answer"
python3 run.py
cat answer.md
```

The Ask invocation uses your provider account and may incur charges. An
accepted answer should say exports are available during an active subscription
and should happen before closure, with exact links to the supplied sources.
The evidence says nothing about export duration or file-size limits.

Edit `question.txt` or supply other paths:

```sh
python3 run.py sources.jsonl another-answer.md --question question.txt
```

Each run retains normalized `evidence.jsonl`, the model's `candidate.md`, and
Ask's `session.jsonl` under a fresh `runs/` directory beside the output. Its
path is printed on stderr. Invalid input, provider failure, or citation
rejection exits nonzero and preserves any existing final answer. A rejected
candidate remains available for inspection; there is no automatic retry.

Cite requires at least one exact citation and rejects malformed or unknown
`ctx:` references. Acceptance checks citation identities; review whether the
sources support the prose and whether important facts are missing. The
90-second timeout applies to the direct Ask process, and Ask may retry
transient provider failures within its invocation.
