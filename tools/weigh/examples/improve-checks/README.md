# Improve a check and review the change

This example supports a conversational workflow: **“Improve this run” → tested
numbered suggestions → “Apply 1” or “leave it.”** The harness handles the
commands and retains the mapping from each suggestion to its exact proposal.
The user sees what changes, why, the observed benefit, and the tradeoff.

The helpers are independent caller-owned programs. They do not scan runs,
schedule work, create an agent loop, learn automatically, or change defaults.
Use Python 3.9+ and the explicitly selected public executables. Keep inputs,
records, labels and proposals outside reusable source. Optional model calls
remain explicit; the offline evaluation and review helpers need no credentials.

These adapters default to Weigh off. Set `BENCH_WEIGH=1` to permit a selected
Weigh model path; unset, empty or `0` disables it. Backend and model remain
explicit. Any other value is an error only when a Weigh path is reached.
Disabled or invalid Weigh configuration exits 2 without calling a dependency,
emitting a selection, or switching to Ask. Deterministic rules and explicitly
selected Ask paths work regardless of this variable. It controls these caller
adapters, not the standalone `weigh` command.

An enabled Weigh path still depends on the selected executable, a compatible
model, credentials and a reachable provider. Missing dependencies, invalid
responses and provider failures stop that path; they never accept a candidate
or silently choose a different model. Explicit backend/model invocation enables
the call. The old `--live` option is accepted for compatibility and has no effect.

## Inspect one selected run

`triage.py` accepts one version-1 snapshot from stdin or `--input FILE`:

```json
{
  "version": 1,
  "id": "selected-run",
  "candidate": "The current workload balance is incomplete.",
  "evidence": "Current selected work has no unknown effort. The text is in the current-warning field.",
  "criteria": [{
    "id": "current_accuracy",
    "requirement": "Current warnings must follow from current observations.",
    "feedback": "Remove unsupported current warnings."
  }],
  "check": {"verdict": "accept"},
  "review": {
    "verdict": "reject",
    "independent": true,
    "findings": ["The current incompleteness claim is unsupported by the selected observation."]
  }
}
```

The example above illustrates the contract, not a measured model result.
Supply actual candidate/evidence bytes and reviewed findings for real work.
The root fields are exact; `check` and `review` may include additional selected
observations. Snapshot size is limited to 4 MiB. Duplicate JSON keys, nonfinite
numbers, invalid criteria, changed selected files and malformed results fail.

```sh
BENCH_WEIGH=1 python3 triage.py --backend weigh --model "$WEIGH_MODEL" \
  --input /selected/run/snapshot.json --records /selected/run/diagnosis \
  --weigh /selected/bin/weigh --ask /selected/bin/ask --record /selected/bin/record
```

Select `--backend ask --model "$ASK_MODEL"` for an Ask-only route; it works
without Weigh. Existing approved credential wrappers can be selected as the
executables. No model or fallback is selected implicitly. `--endpoint` is an
explicit Weigh Decisions endpoint override, including loopback fixtures. The
default dependency deadline is 60 seconds.

The result has `status: "hypothesis"`, the exact input SHA256, model identity,
record path and two classifications: `target` (artifact, evidence, applicability,
rubric, feedback, threshold, execution, unknown) and `action` (repair,
gather_evidence, revise, calibrate, ablate, repair_execution, inspect).
Weigh retains the native distributions; Ask supplies choices without invented
probabilities. Native confidence is not a probability that the diagnosis is true.
Record verifies the selected invocation; Ask also retains its own session.
Exit 0 means valid recorded hypothesis, and exit 2 means invalid/broken input
or execution. This is not a deliverable acceptance gate.

## Select one fix from check results

Use `select-fix.py` when choosing the next action requires judgment. Keep cheap
rules for fixes established by inspected facts. The helper uses one trusted
rule result or makes one explicitly selected Weigh/Ask call, then returns an
action ID. It executes no repair and changes no checker, threshold or skill.
Run it from this example directory, or copy `select-fix.py` and its adjacent
`triage.py` JSON/process helpers together. Neither imports another tool's code.

Supply this separate version-1 snapshot on stdin or with `--input FILE`:

```json
{
  "version": 1,
  "id": "selected-candidate",
  "task": "Return the measured mass and whether it was measured; preserve a measured zero.",
  "candidate": "{\"mass_g\":0,\"measured\":true}",
  "evidence": {"measurement": {"mass_g": 0, "recorded": true}},
  "check": {
    "verdict": "reject",
    "findings": ["The checker treated zero mass as missing."]
  },
  "actions": {
    "path_a": "Keep the candidate when the task and evidence establish that it is correct.",
    "path_b": "Repair a content defect once using the caller's selected repair runner.",
    "path_c": "Fix only a serialization defect using the caller's reviewed lossless formatter."
  }
}
```

These IDs are opaque labels; the caller owns their meanings and execution.
Choose a menu suited to the actual task. Other possible actions include reading
an explicitly selected evidence source or a bounded repair with an explicitly
selected escalation model. Do not offer actions the caller cannot execute.

The root fields are exact. `id` and `task` are nonempty text, `candidate` is
text, `evidence` is a string/object/array, and `actions` maps 2–255 IDs matching
`[a-z][a-z0-9_]{0,63}` to nonempty descriptions. `check` requires `verdict`
(`accept`, `reject`, `error` or `unknown`) and a `findings` array of nonempty
text entries; an empty array and additional selected check observations are
allowed. Supply actual task and source evidence, not just a check status. A
check can pass a defective artifact or reject a valid one.

An optional `--rules FILE` accepts exactly `version: 1`, `input_sha256`,
`action`, and `reason`. The hash binds the exact snapshot bytes; `action` is
either an allowed ID or `null`, and `reason` is nonempty text. The rule file
comes from trusted deterministic code that inspects relevant facts outside
worker-writable paths. A nonnull action bypasses the model entirely; `null`
leaves the semantic choice open. A rule that only sees a check pass must not
infer that keeping the candidate is correct.
For example, a confirmed serialization defect can select a lossless formatter;
a failed checker executable can select process investigation without asking a
model to infer a content defect. These rules use observed conditions, not a
model's confidence score.

For a rule-settled action, no backend or model is needed:

```sh
python3 select-fix.py --input /selected/run/fix-snapshot.json \
  --rules /selected/run/rule-result.json --records /selected/run/fix-selection
```

For a semantic choice, explicitly select the backend and model:

```sh
BENCH_WEIGH=1 python3 select-fix.py --backend weigh --model "$WEIGH_MODEL" \
  --input /selected/run/fix-snapshot.json --rules /selected/run/rule-result.json \
  --records /selected/run/fix-selection \
  --weigh /selected/bin/weigh --record /selected/bin/record \
  > /selected/run/selection.json
```

Omit `--rules` when there is no rule result. Use `--backend ask --model
"$ASK_MODEL" --ask /selected/bin/ask` for an Ask selector. The selector model
and the eventual repair model are separate caller choices. Existing credential
wrappers can be selected as executables. `--endpoint` optionally selects a
Weigh Decisions endpoint, including local fixtures. No model, threshold, retry
or fallback is implied.

Successful output contains `status: "selected"`, `action`, `input_sha256`,
`selector` (`rule`, `weigh` or `ask`), and `requires_final_check: true`.
`model` identifies the semantic model; `inference` retains the full native
Weigh result or Ask choice object, without inventing probabilities. `record`
identifies the recorded model invocation; those three fields are null for a
rule decision. `rules_sha256` binds a supplied rule file, `reason` carries the
rule's reason when selected, and `records` names the retained evidence location.
The helper retains the exact snapshot, request, response, supplied rule and
result as applicable, and verifies Record evidence. A Weigh choice uses the
native returned value, not an inferred argmax or confidence cutoff.

Exit 0 means a valid selection, **never deliverable acceptance**. Invalid
inputs, broken dependencies or invalid results exit 2 with no result on stdout.
Stop that selection; do not silently pay another model or execute a default
repair. Each deliberate selection creates a fresh subdirectory beneath
`--records`, named in the returned `records` field.

### Dispatch through the existing caller

The caller validates the selection's snapshot binding and maps only allowed IDs
to reviewed literal argv. Use the current runner's remaining action/turn limits
and execution boundary. For example, this one-action adapter illustrates the
boundary; `repair_argv`, `formatter_argv` and `check_argv` come from reviewed
caller configuration, not model output. Adapt their stream contracts to the
actual public programs. The snapshot file and its selected inputs must remain
immutable in a caller-controlled location during execution.

```python
import hashlib
import json
import subprocess
from pathlib import Path

def dispatch_one(snapshot_path, selection, repair_argv, formatter_argv, check_argv):
    raw = Path(snapshot_path).read_bytes()
    snapshot = json.loads(raw)
    commands = {"path_a": None, "path_b": repair_argv, "path_c": formatter_argv}
    if (selection.get("status") != "selected"
            or selection.get("requires_final_check") is not True
            or selection.get("input_sha256") != hashlib.sha256(raw).hexdigest()
            or selection.get("action") not in snapshot["actions"]
            or selection.get("action") not in commands):
        return 2
    argv = commands[selection["action"]]
    candidate = snapshot["candidate"].encode("utf-8")
    try:
        if argv is not None:
            fixed = subprocess.run(argv, input=raw, capture_output=True, timeout=60)
            if fixed.returncode != 0:
                return 2
            candidate = fixed.stdout
        # The reviewed checker has the same frozen task/evidence and reads candidate stdin.
        checked = subprocess.run(check_argv, input=candidate, timeout=30)
    except (OSError, subprocess.TimeoutExpired):
        return 2
    return checked.returncode if checked.returncode in (0, 1, 2) else 2
```

The caller selects any repair model in `repair_argv` and supplies the appropriate
boundary/recording wrappers for both actions and checks. The example timeouts
are illustrative; use limits chosen for the task. No shell evaluates model
text. Every path, including `path_a`, reaches the unchanged independent final
checker: 0 accepts, 1 rejects/unfinished, 2 is broken. Preserve the action and
check records, feedback and failures using the existing runner. This adapter
does not add a repair loop or make a selected checker investigation into an
authorized policy change.

Before expanding adoption, compare rules, the optional selector and any chosen
Ask selector with the same action menu, runner and independent final check on
fresh cases. Retain repeat outcomes and all selection/repair/check time, tokens
and cost. Selector agreement or speed alone does not establish a gain in
completed work. Fix selection is separate from `triage.py`'s investigation
hypothesis and from Hone's verified-recovery requirements below.

## Test a proposal with the existing tools

Use Hire to author a focused skill or method revision in a clean definition
copy. Use existing Ply/Agent execution for iterative work, with an independent
unchanged experiment gate. For checks, evidence selection and configuration,
make a separately inspectable source change. Inspect generated check code
before it runs with controller authority. A candidate cannot edit its evaluator.

Keep the requested outcomes fixed. Compare old/new checks on independently
labeled good, bad and plausible near-miss candidates. Use calibration data for
threshold sweeps and removal experiments, then freeze the proposal before
evaluation. Collect actual matched jobs and independently review final outputs
before claiming improved completed work. Use the adjacent
[semantic checker](../semantic-check/README.md) as an optional subprocess, not
as a provider library. Preserve per-criterion judgments and the full native
score scales. Missing results cannot be entered as successful scores.

`evaluate.py` reads the explicit collected evidence and produces an offline
comparison. See [CALIBRATION.md](CALIBRATION.md) for the complete schema:

```sh
python3 evaluate.py --input /selected/run/evaluation.json \
  > /selected/run/evaluation-result.json
```

It reports calibration-only sweeps/ablations, heldout baseline/proposal errors,
coverage violations, missing/unknown/error outcomes, recorded costs, times,
tokens and worker turns,
and separate matched-output reviews. `--gate` returns 0 for supported, 1 for a
valid unsupported result and 2 for invalid input. A valid report without
`--gate` always exits 0; inspect its `decision`. No setting is automatically
chosen or applied. A small sample is evidence for a bounded experiment, not
statistical certainty. Equal adequate quality with fewer measured tokens,
worker turns, elapsed seconds or cost can support an improvement. Show every
measured dimension: a faster result may cost more, and the user's choice decides
that tradeoff. Missing measurements remain unknown. Source pins, independence and split assignment remain
caller attestations, not authenticated truth.

## Present the exact change

For text checks, skills or configuration, `review.py` freezes selected file
changes and their evaluated source identities. It supports regular UTF-8 text,
additions, deletions and Git executable modes; it refuses symlink traversal,
binary data and changes to `.git`. Select every source/configuration file the
experiment depends on. Unselected dependencies are not attested by the helper.

Before collecting evaluation evidence, obtain source fingerprints for the
baseline and candidate, and put them in the corresponding settings'
`source_sha256` fields. Select identical relative path inventories:

```sh
python3 review.py fingerprint --root /selected/source --file rubric.json --file policy.json
python3 review.py fingerprint --root /selected/candidate --file rubric.json --file policy.json
```

After evaluation, prepare one immutable suggestion:

```sh
python3 review.py prepare --source /selected/source --candidate /selected/candidate \
  --file rubric.json --file policy.json --evaluation /selected/run/evaluation.json \
  --title 'Separate current warnings from import history' \
  --reason 'Independent review found an unsupported current warning in an accepted output.' \
  --tradeoff 'The measured benefit is limited to the supplied cases and output trials.' \
  --out /selected/run/proposal-1
python3 review.py show /selected/run/proposal-1 --number 1
```

Preparation reruns the offline evaluator, binds its settings to the selected
old/new files, and retains `proposal.json` plus `change.patch`. It never writes
to source. A supported report offers Apply; other outcomes offer further testing
or leaving the source alone. Show its scope honestly: fixed-candidate accuracy
is different from independently reviewed output quality. The harness uses the
JSON evidence to write a short understandable card, retaining `reviewed_sha256`
outside the source for that exact displayed suggestion.

When the user accepts that suggestion, verify the retained review identity:

```sh
python3 review.py verify /selected/run/proposal-1 --reviewed-sha "$REVIEWED_SHA"
```

Verification checks source, patch, evaluator and reviewed evidence; unsupported
or stale proposals fail. It returns literal `apply_argv` for the existing
`git apply` command and the expected resulting source fingerprint. It makes no
source change itself. Execute that argv without a shell, promptly after
verification, then repeat `fingerprint` and compare the result. Use a controlled
authoring copy without concurrent writers; verification is not a filesystem
lock or protection against a hostile writer. Do not resolve patch conflicts or
regenerate a proposal silently after acceptance. Show a fresh review instead.

All review operations exit 0 on success and 2 for invalid/stale/unsupported
input. `prepare` creates only the new selected proposal directory; other modes
are read-only. The digest identifies reviewed bytes, not user authorization.
Overlapping suggestions require a reviewed combined change. Keep original source
and records for a later deliberate revert; do not overwrite intervening edits.

## Retain a lesson only when evidence supports one

For a real verified rejection followed by a changed accepted candidate, use
`hone -why SESSION`, then Hone's existing prepare/show/admit workflow. Hone
0.3.0 supports recorded content recoveries as well as command recoveries.
First-try success and an empty precheck are not rejected assistant candidates.
Independent output review must still establish that a lesson is useful.
No qualifying recovery is an ordinary outcome; never manufacture one or turn
a classifier hypothesis into a lesson.

Keep `hone admit` until the user accepts the exact displayed lesson in this
review-first workflow. Use Hire for independently justified method changes
without a qualifying Hone recovery. Neither this example nor Weigh introduces
a learning service, an approval system, or another runner.

## Offline verification

```sh
python3 -m unittest discover -s . -p 'test_*.py' -v
```

These are protocol/evaluation/review tests. Root executable integration also
tests public Weigh/Ask/Record diagnosis and Ask/Ply/Hone recovery against local
fixtures. Fixtures do not establish actual model accuracy or output gains.
