# Ask

**Put a language model in your shell. Pipe in data; get an answer you can use.**

Explain a build failure, review a diff, read a chart, or turn an alert into
validated JSON. Ask handles the model connection and keeps a replayable
conversation. Your existing programs supply the data and use the result.

Start with one answer. If the job later needs commands and correction turns,
Ply can call this same Ask. Agent adds an expert folder around that work. A
simple summary still needs only Ask.

```sh
git diff | ask 'Review this patch for bugs.'
ask -a chart.png 'Explain the trend in three sentences.'
```

[Install](#install) · [First answer](#get-your-first-answer) ·
[Recipes](#go-further) · [Field guide](GUIDE.md) · [Manual](ask.1)

## Install

From the [Bench tools monorepo](https://github.com/patrickyoung/bench-tools),
run `python3 scripts/install ask` at the repository root. See
[installation and updates](https://github.com/patrickyoung/bench-tools/blob/main/docs/INSTALL.md)
for prerequisites and PATH setup, or
[getting started](https://github.com/patrickyoung/bench-tools/blob/main/docs/GETTING-STARTED.md)
for a guided first result.

**Standalone install.** You need **Go 1.26+** and **Linux, macOS, BSD, or
WSL**. This installs the current `main` source:

```sh
mkdir -p "$HOME/.local/bin"
GOBIN="$HOME/.local/bin" go install github.com/patrickyoung/ask@main
export PATH="$HOME/.local/bin:$PATH"
ask version
```

Keep that PATH setting in your shell startup file to use Ask in new terminals.

Choose a model your provider account can access. Replace both placeholders:

```sh
export ASK_MODEL='anthropic/YOUR_MODEL_ID'
export ANTHROPIC_API_KEY='YOUR_API_KEY'
```

Ask uses `provider/model` names. Choose the matching credential:

| Provider | API-key environment variable |
| --- | --- |
| `anthropic` | `ANTHROPIC_API_KEY` |
| `openai` | `OPENAI_API_KEY` |
| `gemini` | `GEMINI_API_KEY` |
| `openrouter` | `OPENROUTER_API_KEY` |
| `deepseek` | `DEEPSEEK_API_KEY` |
| `cerebras` | `CEREBRAS_API_KEY` |
| `openai-codex` | An inherited authorization header; see OAuth below |

Calls use your provider account and its usage limits. Model IDs and supported
features come from that provider. For OpenRouter, keep the full routed name:
`openrouter/PROVIDER/MODEL_ID`.

## Get your first answer

```sh
printf '%s\n' 'Build failed: PORT must be an integer; received "http".' |
  ask 'Explain this error and suggest the next step.'
```

The answer appears on **stdout**. Progress, reasoning, usage, and errors appear
on **stderr**, so saving the answer is straightforward:

```sh
printf '%s\n' 'We fixed login failures and reduced startup time.' |
  ask -q 'Write a short release note.' > release-note.md
cat release-note.md
```

Arguments are the instruction; stdin is the material to work on. Ask cannot
open a path merely mentioned in a prompt or run a command. Use `< file`, a
pipe, or `-a file` to give it the actual contents.

## Keep a conversation when you want one

Each plain `ask` starts fresh. `-c` continues the current conversation:

```sh
ask 'Explain a ring buffer simply.'
ask -c 'Now show a small Go example.'
ask replay
ask replay -check
```

For work you want to name and return to, use an explicit file:

```sh
ask -f review.jsonl 'Explain this patch.' < patch.diff
ask -f review.jsonl 'Which change deserves the closest review?'
ask replay -check review.jsonl
```

`-f` creates or continues only that file and never moves `current`. Normal
sessions live in `$ASK_DIR`, defaulting to `~/.ask/sessions`. Session files are
private, append-only JSONL, and allow only one writer at a time.

If Ask reports a full context window (exit 2), start a fresh conversation or
run `ask compact review.jsonl`. Compaction verifies the source, records a
separate summarizer call, and creates a successor with an attributed handoff;
it leaves the source intact. To branch without summarizing, copy an idle
session file and continue the copy with `-f`.

## Go further

### Get JSON a program can validate

Create the schema before requesting a structured answer:

```sh
cat > severity.schema.json <<'JSON'
{"type":"object","properties":{"severity":{"type":"string","enum":["low","medium","high"]}},"required":["severity"],"additionalProperties":false}
JSON

printf '%s\n' 'The service is unavailable for every customer.' |
  ask -schema severity.schema.json 'Classify this incident.' > severity.json
```

On success, `severity.json` contains an object such as `{"severity":"high"}`.
The schema is native request metadata, and Ask validates the completed JSON
locally before releasing it. Invalid, incomplete, or refused answers produce
no normal answer on stdout. Check the command's status before using the file.

Structured output requires provider/model support. DeepSeek is refused for
`-schema` because its Chat API offers JSON mode without the required schema
boundary. Cerebras uses strict native JSON Schema. Ask does not silently turn
an unsupported schema into prompt instructions. Schemas are bounded at 1 MB;
external references are refused and numeric values retain their precision.

### Read files and images

```sh
ask -a report.txt 'Summarize the decisions.'
ask -a before.png -a after.png 'What changed?'
ask -a q3.pdf -a q4.pdf 'Compare the two reports.'
```

UTF-8 text files are inlined. Media support depends on the selected model:
Anthropic, OpenAI, OpenAI Codex, and OpenRouter adapters support images and
PDF; Gemini also supports audio/video; OpenRouter supports WAV/MP3; DeepSeek
accepts supported images with a vision model; Cerebras accepts text only.

Limits: 16 attachments, 16 MB per file, 32 MB total, and 16 MB stdin. Ask refuses
oversized or unsupported input instead of silently truncating it. Attachment
bytes are retained in the session, so treat archives as sensitive material.

### Bring your own procedure, evidence, or tools

| You want to… | Add… | How it fits |
| --- | --- | --- |
| Reuse a written procedure | [Brief](https://github.com/patrickyoung/bench-tools/tree/main/tools/brief) | Print a chosen skill into the system prompt |
| Retrieve external evidence | [Context](https://github.com/patrickyoung/bench-tools/tree/main/tools/context) | Pipe normalized records into Ask as data |
| Check citation identities | [Cite](https://github.com/patrickyoung/bench-tools/tree/main/tools/cite) | Validate an answer against the same evidence file |
| Execute commands until a check passes | [Ply](https://github.com/patrickyoung/bench-tools/tree/main/tools/ply) | Run Ask inside an explicit action/check loop |
| Run a reusable expert | [Agent](https://github.com/patrickyoung/bench-tools/tree/main/tools/agent) | Supply an expert folder and workspace; it reuses Ask and Ply |
| Build the expert's definition | [Hire](https://github.com/patrickyoung/bench-tools/tree/main/tools/hire) | Write the folder by running its builder through Agent |
| Search past conversations | [Trail](https://github.com/patrickyoung/bench-tools/tree/main/tools/trail) | Browse Ask's existing JSONL archives |
| Use a terminal workspace | [Bench](https://github.com/patrickyoung/bench) | Review tasks and run the same public tools interactively |

For example, with Brief installed and a `release-notes` skill available:

```sh
system=$(ask system) &&
procedure=$(brief cat release-notes) &&
ASK_SYSTEM="$system
$procedure" ask 'Draft a release note.' < changes.txt
```

`ask system` prints the built-in prompt. `ASK_SYSTEM` supplies an override;
`-S` overrides it for one invocation. A previous `-S` value does not persist
into the next call.

`-verbosity low|medium|high` controls answer detail on OpenAI Responses,
including `openai-codex`; other adapters ignore it. The default leaves this
choice to the provider. Set `ASK_VERBOSITY=low` to keep the setting across
calls and `ask compact`, or use `-verbosity ""` for the provider default on
one call. This is request metadata, separate from reasoning effort and
recorded by replay; it does not change the system prompt or cap output.

### Use OAuth and gateways

Ask does not have a login command, token store, or refresh loop. Configure a
profile with [OAuth](https://github.com/patrickyoung/bench-tools/tree/main/tools/oauth), then pass its
header on an inherited descriptor:

```sh
oauth with llm -- ask -header-fd 3 -m openai/YOUR_MODEL_ID 'Hello.'
```

The same interface supports `openai-codex/YOUR_MODEL_ID`; set the non-secret
`OPENAI_CODEX_ACCOUNT_ID` when required. The header stays out of argv,
environment variables, diagnostics, and session logs, and is bound to the
initial provider origin.

Provider endpoints can be overridden with `ANTHROPIC_BASE_URL`,
`OPENAI_BASE_URL`, `OPENAI_CODEX_BASE_URL`, `GEMINI_BASE_URL`,
`OPENROUTER_BASE_URL`, `DEEPSEEK_BASE_URL`, or `CEREBRAS_BASE_URL`.
Anthropic Vertex routing uses `ANTHROPIC_VERTEX_PROJECT_ID`, `CLOUD_ML_REGION`,
and optionally `ANTHROPIC_VERTEX_BASE_URL`; credential acquisition remains
external. See [the manual](ask.1) for effort mappings and endpoint details.

## Inspect the record

Create an explicit session before recording deterministic work, without a
model call:

```sh
ask init -f tools.jsonl
printf '%s\n' '{"exit_code":0,"result":"checked"}' |
  ask note -q -s checker -f tools.jsonl -k checker.result/v1 -json - -seal
ask replay -check tools.jsonl
```

`init` prints the new absolute path, refuses existing files, leaves `current`
unchanged, and does not read stdin or select a model. The header is sealed
before the path is printed. Later `note` and `append` calls work normally;
select a model with `-m` or `ASK_MODEL` when adding a first model turn. A
structured note is a record, so it does not enter the model's conversation.

```sh
ask replay -json review.jsonl         # raw events
ask replay -check -json review.jsonl  # the same snapshot, verified first
ask replay -step 4 review.jsonl       # reconstruct one recorded request
ask note -s reviewer -f review.jsonl 'Human review completed.'
```

For a sequence of typed records, `ask note -s recorder -f tools.jsonl -jsonl - -seal`
reads `{ "kind": "example/v1", "body": {} }` objects, one per line. It holds
the existing session writer lock and emits `{"seq":N}` on stdout only after
each record and its prefix seal are durable. Invalid input stops the stream;
already acknowledged records remain sealed. Notes still never enter the model
conversation. This is an ordinary stdin filter, with no background service.

`ask note` requires attribution and an existing session; a note is a record,
not another model message. Programs can atomically append typed JSON notes
with `-k KIND -json - -seal`.

Replay checks request folds, event order, structured-note seals, and any
Context evidence manifest against the exact recorded bytes. That manifest
retains the source query and connector fingerprint as well as citation identities. It detects
inconsistency; it does not establish factual truth, remote signer identity, or
protect against removal of a valid suffix. `-json` requests raw events even
when a provider turn fails; ordinary answer mode withholds incomplete output.

## Record observations and manage context

Programs that observe an action before stopping can record the result as a
message without requesting another model turn:

```sh
printf '%s\n' 'Observed: tests passed; publication is still pending.' |
  ask append -q -s ply -f review.jsonl
ask -f review.jsonl 'Continue from the recorded observation.'
```

`ask append` requires an existing session and explicit attribution. It seals
and fsyncs a user message, so replay detects incomplete or altered records and
the next request includes it exactly once. It preserves UTF-8 text from argv
or stdin (16 MB maximum), keeps Context evidence manifests, and rejects binary
text rather than silently replacing bytes. Notes remain separate records
that do not enter the conversation. A checkpoint and an observation do not
prove an uncertain external effect is safe to repeat.

For proactive context budgets, use `ask context -json -limit 100000 review.jsonl`
and `ask compact -q -at 100000 review.jsonl`. The latter prints the same absolute
session path below the threshold, without calling a provider; at or above the
threshold it prints a new compacted session path. The caller chooses a token
budget that leaves room for the next input and answer. No model window sizes
are guessed. Provider-normalized input and output usage supplies the baseline;
unmeasured new messages use a cautious serialized-byte allowance plus framing.
Old usage fields use a conservative sum, labeled `legacy_usage_allowance`.
The latest logged request supplies the effective system prompt; a replacement
after the measured turn adds its full byte allowance. Estimates cannot predict
unlogged changes to the system, schema, or next input. They are not exact
tokenization, and media or provider-specific replay can change the next request
size. Overflow remains a separate recoverable outcome.
Compaction retains an inspectable, attributed handoff with the active goal,
constraints, recent observations, unresolved effects, and pending job handles.
Repeated compaction still requires evaluating the chosen summarizer's quality;
Ask does not silently enable a provider-native replacement.

## Troubleshooting and outcomes

| Symptom or status | What to do |
| --- | --- |
| `ask: command not found` | Add the installation directory to PATH |
| Provider/model error | Check the model ID, matching credential, and endpoint |
| `-c` cannot find `current` | Start with plain `ask` or name the intended file with `-f` |
| Exit 0 | The invocation completed successfully; this is not a factual correctness check |
| Exit 1 | Usage, provider, output, validation, or other operational failure |
| Exit 2 | The model context window is full; start fresh or compact |
| Exit 130 | Interrupted |

`-q` hides progress, not errors. An output-token limit is exit 1, not context
capacity exit 2. Always check the exit status when saving or processing output.

## Reference and development

```text
ask [flags] [message ...]
ask replay [flags] [session]
ask init -f FILE
ask compact [flags] [session]
ask context [flags] [session]
ask append -s SOURCE [flags] [text]
ask note -s SOURCE [flags] [text]
ask system
ask version
ask help
```

[GUIDE.md](GUIDE.md) has shell recipes; [ask.1](ask.1) is the full reference.
Contributors should read [AGENTS.md](AGENTS.md) and run `go test ./...`; add
`go test -race ./...` for log or stream changes. Report security issues using
[SECURITY.md](SECURITY.md). [MIT license](LICENSE).

`ask compact -json SESSION` emits `{source, summary, session}` with absolute
paths for controllers retaining all three compaction artifacts. Below an
`-at` threshold the summary is empty and the session equals the source.
The default compact output remains the single continuation path.
