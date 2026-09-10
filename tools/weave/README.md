# Weave

Read a finite task graph and observed outcomes. Print the work that is ready.

```sh
weave tasks.jsonl < observations.jsonl > ready.jsonl
```

**v0.1.0 supports this Unix filter and its stream contract.** It has no model,
worker pool, queue, database, retry policy or conversation log. A caller supplies
verified observations, uses Weave to select work, and submits that work to an
execution program such as Tend. The included Python research and business
recipes are experimental; their APIs and study formats may change.

## Install and try it

From the [Bench tools monorepo](https://github.com/patrickyoung/bench-tools),
run `python3 scripts/install weave` at the repository root. See
[installation and updates](https://github.com/patrickyoung/bench-tools/blob/main/docs/INSTALL.md)
for prerequisites and PATH setup, or
[getting started](https://github.com/patrickyoung/bench-tools/blob/main/docs/GETTING-STARTED.md)
for a guided first result.

**Standalone install.** The filter is one binary with no runtime dependencies.
Build with Go 1.26 or later from Weave's own source directory (`tools/weave`
in the monorepo):

```sh
go install .
weave version
```

Or install the binary and man page into your own prefix:

```sh
make install PREFIX="$HOME/.local"
export PATH="$HOME/.local/bin:$PATH"
```

The same command works from an extracted binary bundle; its prebuilt `weave`
and included sources share timestamps, so rebuilding is unnecessary. Archives
are produced for macOS and Linux on arm64 and amd64. See [release instructions](RELEASING.md)
for checksums, source bundles and publication status. A remote `go install ...@v0.1.0`
requires the repository and tag to be published first.

From Weave's source directory (`cd tools/weave` from the monorepo root), try
the bundled [filter-only example](examples/quickstart/README.md):

```sh
weave examples/quickstart/tasks.jsonl < /dev/null
weave examples/quickstart/tasks.jsonl < examples/quickstart/observations.jsonl
```

The first prints the baseline task. The second prints two independent reviews.
These are complete original task records, ready for another program to consume.
The example observation is fabricated demonstration data, not an authenticated
business outcome. No model, Python, Tend or Ask is needed to run these commands.

## Stream contract

Each task is one JSON object with three required fields:

```json
{"id":"baseline","needs":[],"input":{"goal":"measure current performance"}}
{"id":"experiment","needs":["baseline"],"input":{"method":"routing"}}
```

`id` is a nonempty string of at most 256 decoded UTF-8 bytes. `needs` is an array
of distinct existing task IDs. `input` is any JSON value, including null. Unknown
fields, numeric precision, record bytes and input order survive projection.

An observation snapshot has at most one record per admitted task:

```text
{"id":"baseline","task_sha256":"sha256:<64 lowercase hex digits>","state":"accepted","evidence":[{"kind":"check","ref":"study/check.jsonl#27"}]}
```

The digest covers the **exact task bytes**, excluding LF or CRLF. The digest in
this schema illustration is a placeholder; the quickstart contains a real one.
Reformatting a task intentionally invalidates its observations.

| State | Readiness behavior |
| --- | --- |
| No observation | Emit if all prerequisites are accepted |
| `queued` / `running` | Already admitted; do not emit again |
| `accepted` | Dependents may advance |
| `rejected` / `unknown` | Do not retry or release dependents |

Terminal states require nonempty evidence references containing `kind` and
`ref`. Extra observation/reference fields are allowed. **The adapter verifies
the evidence; Weave validates structure and task identity.** A model-written
`accepted` record is not sufficient authority.

Both finite inputs are validated completely before any output. Unknown IDs,
duplicate observations or JSON keys, missing dependencies, cycles, stale hashes,
invalid UTF-8, unpaired escaped Unicode surrogates and inconsistent admission
states fail. Known field names are case-sensitive. An admitted task must have
accepted prerequisites.

| Bound | Maximum |
| --- | ---: |
| Each stream, including terminators | 16 MiB |
| Each record, excluding its terminator | 1 MiB |
| Records per stream | 10,000 |
| Dependency edges | 100,000 |
| JSON container depth, including empty containers | 64 |

LF and CRLF are accepted; so is an unterminated final record. A CR without a
following LF is payload. Output uses LF, or CRLF if the payload itself ends in
CR, preserving its exact bytes and digest when read again. Blank lines fail.

| Exit | Meaning |
| --- | --- |
| 0 | Valid projection, possibly empty |
| 1 | Invalid or inconsistent input |
| 2 | Usage or operating failure |

**Empty output is not completion.** Work may be complete, blocked or already
running. Check the observation snapshot. Output failures can leave a partial
stream, so check exit status before admitting work. Normal OS signals apply,
including SIGPIPE when a downstream reader closes. `weave -- filename` handles
literal filenames such as `help`, `version` or `-`; observations always use stdin.

## Experimental business recipes

The [bounded research recipe](examples/RESEARCH.md) compares parameter search,
one adaptive agent and a small team. Agents choose concrete interventions;
ordinary programs measure them against a frozen synthetic evaluator. All
selections freeze before final cases are evaluated. Failed and duplicate
proposals consume slots; unknown outcomes are never retried automatically.

The [completed comparison](examples/RESEARCH-RESULTS.md) found no qualifying
improvement. The team reached the same selection as one agent while repeating
14 of 27 configurations. Its apparent validation gain failed the final service
and rework checks. This supports the experiment/evidence mechanism, not a claim
of real business improvement or a demonstrated need for hundreds of agents.

The [original business demos](examples/RESULTS.md) cover policy review with a
supported minority objection and a simpler invoice simulator. They are retained
as historical fixtures, separate from the newer benchmark.

For the optional recipes, install Python 3.9 or later, Git and Go, then build
pinned public Tend/Ask/Ply dependencies without sibling checkouts:

```sh
make bootstrap
export WEAVE="$PWD/var/tools/weave" TEND="$PWD/var/tools/tend"
export ASK="$PWD/var/tools/ask" PLY="$PWD/var/tools/ply"
python3 examples/research.py var/my-study --arms search --evaluations 9 --trials 3
```

This search-only command uses no model. Real agent comparisons require ordinary
Ask authentication and an explicit `--model`; see the recipe guide. Credentials
and provider setup remain outside Weave. The report-only examples refuse
model-authored shell actions. General arbitrary agent execution requires its own
boundary and is outside this release.

Use a dedicated local run directory. Tend owns execution state and attempts;
Ask owns model events and sealed receipts. The adapter verifies output and
source bindings before deriving observations. Study reports and JSONL summaries
are disposable views, not competing authorities. Private run directories,
authentication wrappers and transcripts are excluded from release bundles.

Resume with the identical command and original tools/sources. Completed tasks
are rechecked without new attempts. Changed inputs or binaries are refused;
use a new study directory for an upgrade. Preserve older tools and sources with
archived studies. Budgets bound slots, workers and time; reported token usage is
not an aggregate dollar reservation. Synthetic effects need independent
calibration before supporting a real business pilot.

## Development

```sh
make check                 # independent filter + pure Python checks
make bootstrap             # network fetch/build of pinned optional tools
make check-examples        # full offline suite; local inference fixtures only
make release               # local archives, checksums and source manifest
```

The [CI workflow](.github/workflows/ci.yml) runs on Linux and macOS. It also checks
installation and builds candidate archives without publishing them. Tests cover
stream boundaries, malformed input, crash/interrupt paths, evidence tampering,
resumption, finite adaptive rounds and bounded 300-task execution. The latter
uses ordinary programs, not 300 live model conversations.

[MIT license](LICENSE) · [Changes](CHANGELOG.md) · [Release procedure](RELEASING.md)
