# Improve

Measure one proposed source change on fresh cases and export it only when an
independent judge supports it. `improve` is a Unix filter and experiment
controller: JSON in, one JSON result out, progress on stderr, evidence in an
explicit directory. Hire authors, Agent runs, Record records, and the selected
judge decides whether the change helped.

Version **0.1.0** supports existing text files: system prompts, skills, subagent
definitions, checks, and check responses. Select the exact mutable files. The
independent evaluator remains frozen even when a worker's own check changes.

```sh
go build -o improve .
./improve -n < experiment.json                 # no execution or writes
./improve -o /absolute/new-study < experiment.json > result.json
./improve verify /absolute/new-study           # offline Record replay
```

The output's parent must already exist. An execution invocation explicitly
admits its selected commands, including any configured paid model calls.
Planning requires those executables to be locatable but runs none of them.

## Run an offline example

Build/install the independent `improve`, `record` and `ask` commands and put them
on PATH. From this component directory:

```sh
python3 examples/router/spec.py --offline > /tmp/router-experiment.json
improve -n < /tmp/router-experiment.json
improve -o /tmp/router-study-001 < /tmp/router-experiment.json
improve verify /tmp/router-study-001
```

Use a fresh output name each time. This fixture invents observations to exercise
acceptance and export; it makes **zero model calls and proves no model quality**.
For real Hire/Agent adapters and model selection, see the
[router example](examples/router/README.md).

## The experiment

1. Freeze source, case bytes, command identities and declared dependencies.
2. Measure the baseline on development cases.
3. Ask the selected proposer for one change using development observations.
4. Run fresh baseline/candidate development pairs, reversing order on alternate
   repetitions. Discard a rejected candidate without spending reserved cases.
5. Freeze the candidate and compare it against the original baseline on holdout.
6. Export exact supported bytes in `proposal/source`, with a manifest and evidence.

One invocation tests one proposal. There is no retry, resume, adaptive holdout
search, implicit model selection or runtime hook. A later experiment may use
prior findings as development evidence and must select new reserved cases.
An experiment never overwrites the selected worker or publishes a release.
The caller reviews the exact source and evidence and uses its existing promotion
path; the manifest supplies baseline and candidate hashes for that operation.

## Specification

Paths are resolved from the invoking working directory. Data paths must be
absolute/canonical after resolution: symlinks in their path are refused. Selected
executable symlinks are resolved once; command arguments remain literal. Use
absolute paths for scripts passed as arguments. List scripts, rubrics and other
transitive input files under `dependencies` so their bytes and modes are pinned.
Only argv[0] is discovered through PATH; shell syntax is never evaluated.

```json
{
  "version": 1,
  "source": "/definitions/router",
  "mutable": ["AGENTS.md"],
  "development": [{"id": "direct", "family": "direct-requests", "file": "/cases/dev.json"}],
  "holdout": [{"id": "relayed", "family": "relayed-requests", "file": "/cases/final.json"}],
  "commands": {
    "propose": {"argv": ["python3", "/adapters/propose.py"]},
    "trial": {"argv": ["python3", "/adapters/trial.py"]},
    "judge": {"argv": ["python3", "/adapters/judge.py"]}
  },
  "dependencies": ["/adapters/propose.py", "/adapters/trial.py", "/adapters/judge.py"],
  "record": "record",
  "settings": {"runner_model": "explicit/model", "proposer_model": "explicit/model"},
  "repeats": 3,
  "command_seconds": 180,
  "max_seconds": 2400
}
```

Cases are opaque files; an adapter defines their inputs and independent labels.
IDs must be unique. Development and holdout families and identical file bytes
must not overlap. This catches basic leakage, not paraphrases or poor labels.
Settings are opaque JSON passed to adapters; credentials belong in their existing
environment/descriptor mechanisms, never this retained specification.

Defaults: 3 repeats, 180 seconds per selected command, 2400 seconds overall.
Allowed ranges: 2–10 repeats, 1–20 cases per split, 1–64 mutable files,
1–3600 seconds per command and 1–86400 seconds overall. Maximum worker jobs are
`repeats * (3 * development_cases + 2 * holdout_cases)`, plus one proposer and
at most two judges. `-n` prints these limits. Admission reserves at most 1 MiB for the worst-case
call ledger (including repeated literal argv), leaving room within the 2 MiB
result contract for metadata and diagnostics. Timeout cleanup allows three
additional seconds, signals the command group, and kills remaining members.
Detached descendants require cleanup by the selected runner. The controller
never resubmits an interrupted or otherwise unknown command.

JSON documents and captured command streams are bounded at 2 MiB. Crossing a
stream bound cancels the command group immediately; only the bounded prefix is
retained and that call is incomplete, with unknown cost. Source trees and each
command directory allow 4096 files and 128 MiB; the complete study allows 8192
files and 512 MiB, with a 64 MiB per-file limit. The final inventory has its own 2 MiB bound and is excluded
from that aggregate. Planning rejects a workload when
its unavoidable source copies alone exceed the study bound. This is a lower
bound, not a disk reservation: candidate growth and command-written evidence
can still exhaust the allowance. Selected commands remain trusted; these checks
are not filesystem quotas. Source paths and trees must contain regular files/directories,
no symlinks or `.git` metadata. Select clean definitions, not an entire checkout.
Mutable files must already exist and contain UTF-8 text; v0.1 cannot add/delete
files or change their modes.

## Adapter contracts

Each command gets one JSON request on stdin, a separate cwd, and a new `work/`
and `evidence/` directory. Its stdout must contain one versioned JSON envelope;
stderr is diagnostic. Every envelope carries `cost`, a nonnegative number of
provider dollars or `null` when unavailable. The controller's study total is
null if any admitted stage has unknown cost; `known_cost` remains a subtotal.
Zero is appropriate for an observed no-inference deterministic stage, never
for unreported billing. Include authoring, repairs, nested calls and retry costs.

**Proposer input:** `version`, `source`, `source_sha256`, `files` (mutable path to
current text), `development` (case descriptors), `observations` (diagnostic
samples), `settings`, `work`, `evidence`. No holdout contents or paths are supplied.
It returns, with exit 0:

```json
{"version":1,"hypothesis":"Require raw JSON output.","changes":[{"path":"AGENTS.md","content":"Exact replacement text\n"}],"cost":0.001}
```

Only selected files may appear, at most once. Unchanged proposals stop without
comparison. Exit 1 means no proposal; an empty response leaves cost unknown.
To retain known authoring cost, return the same envelope with empty `changes`.
Another exit or malformed evidence is an error. The proposer changes its work
copy only; Improve constructs the exact candidate from returned text.

**Trial input:** `version`, `source` (fresh per-trial copy), `source_sha256`,
`case` (descriptor), `repeat`, `settings`, `work`, `evidence`. It executes the
worker, independently scores the outcome, and returns with exit 0:

```json
{"version":1,"score":{"accepted":true,"correct":12,"total":12,"model_calls":1},"cost":0.0007}
```

`score` is caller-defined non-null JSON. A failed worker can be a successfully
measured observation; return its failure in the score. A nonzero trial-adapter
exit is broken/unfinished measurement, not an invented zero-quality sample.
Retain and verify nested evidence through Ask/Record public replay commands.

**Judge input:** `version`, `split`, `baseline`, `candidate`, `settings`. Each
sample binds `case`, `repeat`, `source_sha256`, `input_sha256`, `observation` and
`evidence` (the retained call directory). Arrays align by repeat and case despite
alternating execution order. It returns:

```json
{"version":1,"decision":"keep","reason":"Perfect outputs and lower measured cost.","cost":0}
```

`keep` requires exit 0; `discard` requires exit 1. Any disagreement or another
exit is invalid evidence. Improve has no built-in business metric. The supplied
router judge requires perfect candidate jobs, no label regressions or extra
calls, complete positive costs, at least `settings.min_savings_percent` lower total cost (default 10%), and lower cost in
a majority of pairs. A shorter prompt alone cannot pass that judge.

For use-case-dependent quality/cost trade-offs, see the
[acceptance example](examples/acceptance/README.md). The experiment-creating
application defines the quality floor, valuable gain, acceptable cost premium
and operating limits from the intended use case, then freezes them before
testing. The example judge has no business defaults: it can accept higher
quality at higher cost or adequate quality at lower cost under that selected
policy. It includes a policy preflight, detailed decisions and the handoff to
the caller's existing promotion process. Its synthetic fixtures are not release
criteria for real workers.

## Evidence and outcomes

`result.json` and stdout name `supported`, `keep_baseline`, `inconclusive` or
`invalid_evidence`, with costs and all calls. Every call retains raw stdin,
stdout, stderr and a Record process receipt. Improve consumes a command's JSON
only after Record replay matches its invocation, streams and exit status.
`spec.json`, `pins.json`, baseline, cases, candidate, comparison files and the
final inventory remain beside these records. Failed experiments remain useful
research evidence. `verify` checks the inventory and replays each completed
process; it never reruns the judge, worker or proposer, and refuses incomplete
receipts. It verifies the retained snapshot, not current external dependencies.

Exit **0** means supported (or successful plan/verification); **1** means no
supported proposal/incomplete execution; **2** means invalid input/evidence or
an operational error. An admitted run emits terminal JSON even when finalization
fails, marks it `invalid_evidence`, and removes any proposal export. Invalid
invocations before an output directory is created report only stderr. Missing
final inventory is incomplete evidence.

Selected commands are trusted, caller-controlled programs. Improve is not a
sandbox or an authorization service. Agent/Cage retain their existing boundary;
other adapters must select their own. Holdouts remain host-readable. Pins do not
freeze ambient environment, remote aliases, caches or undeclared dependencies.
Record consistency does not prove correct labels or generalization. Review
checks before executing them and match a promotion claim to its tested scope.

Weigh can be composed explicitly inside a research adapter and counted in cost.
It is not required by Improve or inserted into live workers. Hone continues to
require a real checked recovery; independent trials never fabricate one.

## Build and check

```sh
go test ./...
go test -race ./...
go vet ./...
python3 -m unittest discover -s examples/router -p 'test_*.py'
python3 -m unittest discover -s examples/acceptance -p 'test_*.py'
```

The Go leaf uses only the standard library and fake public commands in tests.
From the Bench checkout, build `improve record ask` and run
`python3 scripts/check-improve.py --bin-dir .build/bin` for the real process
boundary. Add `--bench-adapters` with Hire/Agent/Ply/Brief/Cage built to test
the real authoring and scoring adapters against a loopback model fixture, with
zero paid calls. See [DESIGN.md](DESIGN.md) and [improve.1](improve.1).
