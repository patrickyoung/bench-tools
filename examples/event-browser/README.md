# Bench Trace

**See what your agents did, and follow new evidence as they work.**

[trace.py](trace.py) is the entire application: a local server, evidence reader,
and embedded interactive browser. Copy that one file anywhere. There is no
frontend build, package installation, database, or model connection.

It needs Python 3.9+ and the current public `ask` and `record` commands. Install
them from this checkout if needed:

```sh
python3 scripts/install ask record
```

Point it at the directory supplied to Agent's `-evidence` flag, a recurring
agent home with `.agent/`, or an individual Ask/Record JSONL file:

```sh
python3 trace.py /path/to/agent-evidence
python3 trace.py /path/to/ask-session.jsonl
python3 trace.py /path/to/record-session.jsonl
```

It opens a browser on a random loopback port. `--no-open` only prints the URL;
`--port 8765` selects a fixed port. Ctrl-C stops the local server. Only the
explicitly selected evidence is read. An expert definition alone does not
identify a run: use that invocation's evidence directory.

For several agents, supply multiple roots or their common evidence parent.
Optional labels identify the selected sources:

```sh
python3 trace.py Planner=/runs/planner Frontend=/runs/frontend Review=/runs/review
```

New runs and appended events are discovered automatically. Each recorded Agent
invocation gets a timeline lane; its process receipts and conversations join
that lane through the recorded index. Explicit parent/child links connect
lanes. Dotted artifact links identify byte-identical retained outputs and inputs
across lanes; matching bytes alone do not prove consumption or parentage.
Standalone or unlinked sessions have their own lanes. The browser also
resolves moved evidence trees and expands conversations retained in complete
Record receipts when the originals are absent. It does not open arbitrary
paths named by archived events.

## Explore a run

- Follow incoming evidence, or pause and step, scrub, and play through history.
- Compare model calls, actions, checks, outcomes, and explicit relationships.
- Filter by agent and event kind; search messages and commands.
- Inspect exact normalized model requests through Ask replay.
- Load full separate stdin/stdout/stderr and retained artifacts as text or hex,
  download their bytes, or download the selected original JSONL snapshot.
- See incomplete evidence, failed outcomes, damaged seals and missing linked
  receipts separately. A failed action can still have a complete valid receipt.

Replay moves a display cursor. It never repeats an archived action or calls a
model. Live updates arrive at recorded event boundaries; Ask records completed
or partial turns, not a stream of every generated token. An open or incomplete
record does not prove its process is alive. The connection indicator describes
the browser's connection to the local evidence feed.

Wall-clock timestamps remain as recorded. Cross-host clock synchronization is
unknown, and timestamps alone do not establish causality. Session sequence
numbers preserve within-session order even when a clock moves backward.
Record preserves each stream's byte order, not the exact scheduling between
stdout and stderr.

## Narrate the timeline

Click **Narrate** beside the timeline heading to create a written walkthrough.
It groups requests with responses and commands with their results, describes
check rejections and later acceptance from the same verifier, and explains
recorded agent relationships. Every passage has buttons back to its evidence.

**Through cursor** is the default: scrubbing backward removes later outcomes
from the narration, and Follow live updates it as evidence arrives. Choose
**Whole recording** to include later events explicitly. The agent filter limits
the walkthrough; search, status and event-kind filters leave its evidence intact.

**Download narration** saves the complete walkthrough as Markdown, including
event references and qualifications. The offline HTML export contains the
narration engine too, so it can generate walkthroughs without the local server.

**Read aloud** is optional and starts only when clicked. It uses a voice that
the browser reports as local; if none is available, the written narration
remains available. Stop reading, changing the cursor/scope/agent, or closing
the panel stops playback. Speech reads the selected narration snapshot; new
events are not silently added to its queue. Voice availability varies by host.
The voice selection uses the browser's
[localService flag](https://developer.mozilla.org/en-US/docs/Web/API/SpeechSynthesisVoice/localService).

Narration is generated locally from event types, recorded outcomes and quoted
excerpts. It needs no model connection and does not infer unrecorded intent or
claim that a later success proves which change fixed a failure. Archive integrity
warnings remain visible, and matching file bytes are qualified as evidence of
identical content. Old projections without relationship event anchors omit
those relationships from narration rather than assigning an invented time.

## One-file offline replay

Use Export in the browser, or:

```sh
python3 trace.py /path/to/evidence --export replay.html
```

The exported HTML contains the selected data, full stream payloads, raw events,
normalized requests and original session bytes. Open it without Python, Bench
commands, the original paths, or network access. It retains the same evidence
qualifications. Export refuses to overwrite an existing file. Treat the HTML
as the selected logs and artifacts: those bytes are embedded, not redacted.

For shell composition, emit the visualization's JSON projection:

```sh
python3 trace.py /path/to/evidence --json | jq '.lanes'
```

Select binaries with `--ask`, `--record`, `TRACE_ASK` or `TRACE_RECORD`. The
browser uses only public read-only replay commands, so no provider credentials
are needed. Ask checks event replay and seals; Record checks receipt ordering,
offsets and all retained stream commitments. Index-to-receipt hashes are also
checked. These are integrity checks relative to the retained archive, not
signatures or proof of business correctness.

Large evidence has explicit limits: 256 MiB per file, 256 MiB of expanded child
sessions, 2,000 selected files, and 30 seconds per replay command. Oversized
inputs are reported instead of silently truncated. Use `--max-file-mib`,
`--max-embedded-mib`, `--max-sessions` and `--command-timeout` to change them.
Full exports and full-stream viewing require memory proportional to the
selected retained bytes. Preview text is bounded and labeled; byte downloads
retain the full selected stream.

## Development checks

The application was built using the existing Product Owner and Frontend
workers through Agent, with separate workspaces and recorded evidence. Page
Reviewer inspected the final page against supplied browser and visual evidence.
The collector and independent checks are ordinary Python/JavaScript around their
deliverables; no worker runtime or transcript implementation was added.

Run the public-command fixtures with this checkout's matching Ask and Record:

```sh
TRACE_TEST_ASK=/absolute/bin/ask TRACE_TEST_RECORD=/absolute/bin/record \
  python3 -m unittest discover -s examples/event-browser -p 'test_*.py' -v
```

They cover exact binary bytes, live appends and completion, corruption, moved
multi-agent trees, missing/changed commitments, embedded conversations, clock
rollback, offline request reconstruction, and the HTTP read boundary. Browser
interaction and visual review are separate from these data checks.

Run the separate Chromium interaction suite with Playwright available in the
test environment (the application itself has no Node dependency):

```sh
TRACE_TEST_ASK=/absolute/bin/ask TRACE_TEST_RECORD=/absolute/bin/record \
  TRACE_PLAYWRIGHT=/absolute/path/to/node_modules/playwright \
  TRACE_CHROMIUM=/absolute/path/to/chromium \
  node examples/event-browser/test_browser.mjs
```

`TRACE_CHROMIUM` is optional when Playwright's bundled Chromium is installed.
The suite checks live updates, pause preservation, replay during incoming
updates, byte downloads, offline export, feed pagination, zoom, keyboard
controls, and responsive layouts. It writes its browser report and screenshots
to a temporary directory, or to `TRACE_TEST_OUTPUT` when set.

The embedded narration engine has separate deterministic regressions:

```sh
node examples/event-browser/test_narration.mjs
```

These check cursor boundaries, failed processes, same-verifier recovery,
relationship anchors, scoped evidence, malformed/missing cursor selection,
archive qualifications and instruction-like input. Browser tests also exercise
narration downloads, offline use, focus/scroll retention and speech control with
a controlled Speech Synthesis API; they do not certify audible voice quality.
