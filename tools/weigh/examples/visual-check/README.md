# Optional visual observations for a semantic checker

Jev currently reads text, not pixels. This example first uses an explicitly
selected **vision-capable Ask model** to inspect actual images. Weigh can then
judge that observer's text against a rubric. This is an optional composition
of public executables, not a renderer, OCR service, image generator, or new
Bench runtime. It does not make Weigh multimodal.

The same workflow works without Jev: choose `--backend ask` for the semantic
checker. Either route needs actual vision inspection before judging visual
properties. A generation prompt, filename, alt text, or intended appearance
does not establish what an image contains.

## Observe the actual artifact

Install Ask and Record separately. Select a model supporting both images and
native JSON Schema; Ask checks the provider boundary, and the provider checks
the selected model. Missing capability, credentials, failed calls, or malformed
observations fail rather than falling back or manufacturing a pass. Calls use
the selected provider account. No model is selected implicitly.

Use the same rubric format as `../semantic-check/check.py`:

```json
{
  "version": 1,
  "criteria": [
    {
      "id": "headline",
      "requirement": "The supplied view visibly contains the complete headline without clipping.",
      "feedback": "Show the complete headline in an unclipped view."
    }
  ]
}
```

Set `EXAMPLES` to this tool's `examples` directory, `RUN` to an existing
caller-owned directory, and `VISION_MODEL` to your selected Ask model. An
OpenRouter selector has the form `openrouter/VENDOR/MODEL`. Keep the observation
record directory new; repeated work gets another directory.

```sh
python3 "$EXAMPLES/visual-check/observe.py" \
  --model "$VISION_MODEL" \
  --rubric "$RUN/rubric.json" \
  --records "$RUN/visual-001" \
  --image "$RUN/generated.png" \
  > "$RUN/observations.json"
```

Repeat `--image` for additional views. Each criterion must have an observation
for **every** supplied image: `observed`, `uncertain`, or `not_visible`.
`observed` means evidence is visible; it does not mean the requirement passes.
A clearly visible defect is an observed violation. Unknown or missing views
cannot be turned into acceptance by an optimistic downstream model. Choose
rubrics appropriate to every view, or run distinct rubric/view groups.

`observe.py` supports `--ask EXECUTABLE`, `--record EXECUTABLE`, and
`--timeout SECONDS` (default 120). It accepts 1–16 PNG/JPEG/GIF/WebP images,
16 MiB per image and 32 MiB total, by file signature. Signature recognition
is not image decoding or a guarantee that a vision model can read the image.
Inputs must be regular files; final-component symlinks are refused.

The new record directory is private (0700). It contains exact image copies,
rubric, fixed observation prompt, schema, input, raw model output, Ask session,
Record invocation, diagnostics, and the normalized observation envelope.
Record selects the input files and child Ask session, retaining invocation
streams through its existing public contract. It is checked before observations
are admitted. Image hashes and the original source paths are stamped only
after local output validation and a second check of the source images/rubric.
Provider responses and attachments may be private; retain the directory in
the caller's evidence area rather than the source library.

The envelope requires integer `version: 1` and exact documented fields. It
contains `kind`, `rubric_sha256`, observer model/session
and raw-output identity, image IDs/source/snapshot hashes, per-criterion/per-view
observations, and limitations. The observer supplies only the observations and
limitations; local code supplies the bindings. Inspect `observations.raw.json`
and the retained image copies when reviewing a judgment.

The observer must state at least one limitation. All bound evidence paths are
absolute. Keep the original images and the record directory at those paths;
moving them requires a new observation. Parent directories may be symlinks,
so the caller must keep its selected directory tree stable. The wrapper binds
image and raw-output bytes, but does not authenticate a model or re-run Record
verification on later checks. For an archive audit, use `record check` with
the retained invocation; source-hash checks alone are not that audit.

## Check while the source is still current

Use `check-current.py` to compose the semantic checker. It verifies the rubric,
image snapshots, current original images, raw observer output, and normalized
candidate before and after the selected checker. It holds stdout until the
second verification finishes. The wrapper supplies `--candidate` and
`--rubric`; forwarded overrides and help-only invocations are rejected. The
checker must return a version 1 report whose verdict matches its exit status
and whose candidate/rubric hashes match the selected inputs. An empty successful
command cannot stand in for a check.

After evaluating appropriate thresholds on your own held-out visual cases:

```sh
BENCH_WEIGH=1 python3 "$EXAMPLES/visual-check/check-current.py" \
  --observations "$RUN/observations.json" \
  --rubric "$RUN/rubric.json" \
  --checker "$EXAMPLES/semantic-check/check.py" -- \
  --backend weigh --model openrouter/typesafe/jev-1.13 \
  --accept-at "$ACCEPT_AT" --reject-at "$REJECT_AT" \
  --records "$RUN/checks"
```

`ACCEPT_AT` and `REJECT_AT` are explicit caller policy, not recommended universal
values. An optional `--evidence FILE` supplies additional selected text to the
semantic checker. It cannot substitute for a missing image observation.
The environment reaches the semantic checker unchanged. Its Weigh path
requires `BENCH_WEIGH=1`; unset, empty or `0` disables it, and other values
are errors when that path is reached. A disabled path fails with exit 2 and
no verdict instead of skipping the check or switching models. The vision
observer and Ask-only checking remain independent of this Weigh opt-in.

Without Jev, use the same current-image guard and retained observations:

```sh
python3 "$EXAMPLES/visual-check/check-current.py" \
  --observations "$RUN/observations.json" \
  --rubric "$RUN/rubric.json" \
  --checker "$EXAMPLES/semantic-check/check.py" -- \
  --backend ask --model "$JUDGE_MODEL" --records "$RUN/checks"
```

The Ask-only route performs no Weigh call. A caller can also use Ask directly
for an attached-image review when the additional judgment stage adds no value;
the existing page team's `expert/bin/review-page` demonstrates that route with
desktop/mobile screenshots and browser results. Ask's public syntax is:

```sh
ask -m "$VISION_MODEL" -f "$RUN/visual-review.jsonl" \
  -schema "$RUN/review.schema.json" \
  -a "$RUN/desktop.png" -a "$RUN/mobile.png" \
  'Inspect these actual screenshots against the supplied brief; report concrete defects and limitations.' \
  < "$RUN/brief.json" > "$RUN/visual-review.json"
```

Supply the review schema yourself. Direct Ask use does not automatically add
this example's source/snapshot binding; retain and validate the artifact used
for acceptance. The example does not invoke an image generator or rewrite an
artifact.

`observe.py` exits 0 for locally valid observations, including uncertainty;
nonzero means failure and emits no observation envelope. `check-current.py`
exits 0 only when its selected semantic checker accepts and the evidence remains
current; 1 rejects with feedback (including uncertainty or unseen evidence);
2 indicates invalid/stale evidence or a broken check. Other nonzero child
statuses are preserved. A changed artifact requires a new observation run.
Keep artifacts stable throughout checking and final publication: before/after
hash checks detect changes at those observations, not an atomic filesystem
transaction or authentication against someone who can rewrite all evidence.

## What this can establish

A vision observer can miss defects or describe pixels incorrectly. Weigh's
confidence is conditional on that text; it does not measure the reliability of
the original visual observation. Re-reading a model's description is not an
independent visual review. Aesthetics, subtle composition, occlusion, tiny text,
and ambiguous details need actual vision inspection and sometimes a human.

Keep file decoding, exact dimensions, transparency, pixel comparisons, and
other computable requirements in deterministic checks. Screenshots cannot
establish unseen states, interaction behavior, accessibility semantics, or
responsive layouts at unobserved sizes. Browser/media checks remain separate.

Vision is the bottleneck in this composition: its latency, token/image cost,
and errors remain. Reusing observations across many rubrics may help; adding
Weigh for one judgment may add overhead. Compare direct vision judgments with
vision→Weigh on held-out defects, including missing views and misleading text.
Measure false acceptance, observer omissions, manual-review rate, total cost,
and end-to-end latency. No paid evaluation is included in the offline tests.

Verified 2026-09-18: [Jev models](https://docs.typesafe.ai/models) explicitly list
text-only input; [OpenRouter Jev](https://openrouter.ai/typesafe/jev-1.13) lists
text→decisions. Check current provider capabilities before choosing models.

## Offline checks

```sh
python3 "$EXAMPLES/visual-check/test_visual.py"
```

These orchestration fixtures use fake Ask, Record, and checker executables;
they verify literal public argv, exact attached bytes, multiple views, complete
observation coverage, uncertainty, retained bindings, invocation failures, and
source changes before/during checking. They do not test provider vision quality
or reimplement Record's sealing checks. Record's own executable integration
suite owns its archive contract.

For a required executable integration proof, supply a directory containing
freshly built `ask` and `record` binaries (missing binaries fail; no skips):

```sh
python3 "$EXAMPLES/visual-check/integration.py" --bin-dir "$BIN_DIR"
```

This test runs the real programs against a loopback OpenRouter protocol
fixture with synthetic PNGs and test-only credentials. It verifies the actual
image bytes and native schema on Ask's wire, Record checking and exact replay,
and composition with the real semantic checker. Stale images, missing views,
uncertainty, malformed observations, and provider errors cannot produce a pass.
The fixture supplies scripted observations and judgments; it proves transport
and orchestration, not visual recognition or judgment accuracy. It makes no
paid model calls and keeps all records in a temporary directory.
