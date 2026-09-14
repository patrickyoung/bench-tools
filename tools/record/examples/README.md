# Record at Ply's existing interpreter boundary

`ply-shell` implements Ply's ordinary `-c SCRIPT` interpreter interface. It
captures each action and verifier before Ply merges and shortens output for
the model. No Ply code or stream contract changes.

```sh
mkdir private-records
export RECORD_DIR="$PWD/private-records"
export RECORD_BIN="$(command -v record)"
export RECORD_ASK="$(command -v ask)"
ply -sh -shell /absolute/path/to/ply-shell -f work.jsonl \
  -check 'test -f result.txt' 'Create result.txt'
```

Record's executable and recording directory must be accessible to the selected
interpreter. Keep them outside model-writable authority. This adapter does
not bypass Cage: an action interpreter inside Cage can only write where Cage
allows it. If the recorder needs trusted write access, place it outside the
confinement launcher under the caller's control; do not grant a model write
access to trusted history merely to make the adapter work.

Each `private-records/invocation.*/session.jsonl` independently retains the
script argv, interpreter hash, full binary stdin/stdout/stderr, and outcome.
Ply's `-cap` still bounds model-visible output and retains its existing
verifier semantics; the separate recording has no presentation cap.
For additional action adapters, use `-action-shell` and retain the actual
adapter selection. Do not describe a wrapper hash as a hash of its transitive
dependencies or a claim that remote execution was observed locally.

Archive the selected invocation files alongside `work.jsonl`. To create one
self-contained archive, select each as a repeated `-session FILE` on an outer
Record invocation; Record never silently scans or guesses which sessions
belong to a run. Compacted parent and summary sessions also need explicit
selection. `record replay -stream ...` recovers their bytes offline.
