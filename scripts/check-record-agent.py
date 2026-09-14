#!/usr/bin/env python3
"""Require automatic Agent recording through the actual public executables."""
import argparse
import base64
import json
import hashlib
import os
from pathlib import Path
import runpy
import shlex
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
support = runpy.run_path(str(ROOT / "scripts/check-integration.py"))
invoke, require, fixture = (support[x] for x in ("invoke", "require", "model_fixture"))


def check(bins, root):
    home, work, evidence = (root / x for x in ("expert", "workspace", "evidence"))
    (home / "bin").mkdir(parents=True)
    work.mkdir()
    (home / "AGENTS.md").write_text("PRIVATE_RECORDING_IDENTITY\n")
    checker = home / "bin/check"
    checker.write_text("#!/bin/sh\ntest -f result.bin\n")
    checker.chmod(0o700)
    env = {"PATH": str(bins) + os.pathsep + os.defpath, "HOME": str(root), "TMPDIR": str(root), "ASK_LIVE": ""}
    command = [bins / "agent", "run", "-no-cage", "-C", work, "-evidence", evidence,
               "-m", "openai/fixture", "-cap", "1024"]
    direct = root / "direct-ply-records"
    direct.mkdir()
    raw_input = b"\n  A goal supplied only on stdin.  \n"
    invoke([bins / "ply", "-sh", "-C", work, "-check", "true", "-record-dir", direct], cwd=root, env=env, data=raw_input)
    input_record, = direct.glob("run.*/inputs.jsonl")
    metadata = json.loads(invoke([bins / "record", "replay", "-f", input_record, "-json"], cwd=root, env=env).stdout)
    artifact = next(a for a in metadata["intent"]["artifacts"] if base64.b64decode(a["path"]).endswith(b"/stdin.bin"))
    captured = invoke([bins / "record", "replay", "-f", input_record, "-stream", artifact["name"]], cwd=root, env=env).stdout
    require(captured == raw_input, "goal normalization changed retained stdin")
    data = b"0123456789abcdef" * 512
    script = "i=0; while [ $i -lt 512 ]; do printf '0123456789abcdef'; i=$((i+1)); done; printf 'separate-error\\n' >&2; printf '\\000\\377artifact' > result.bin"
    answers = iter(["```sh\n" + script + "\n```", "Completed."])
    with fixture(env, lambda _: next(answers)) as (model_env, calls):
        result = invoke([*command, "-record-output", "result.bin", "-checkpoint", "case", home, "--", "Make the result."],
                        cwd=root, env=model_env, data=b"exact supplied input\n")
        require(result.stdout == b"Completed.\n" and len(calls) == 2, "automatic recording changed normal completion")
        require("elided" in json.dumps(calls[1]), "fixture did not exceed the model presentation cap")
        count = len(calls)
        invoke([*command, home, "--", "Already satisfied."], cwd=root, env=model_env)
        require(len(calls) == count, "recorded pre-check called a model")

    def events(path):
        result = invoke([bins / "ask", "replay", "-check", "-json", path], cwd=root, env=env)
        return [json.loads(line) for line in result.stdout.splitlines()]

    def index(path):
        rows = [e["data"]["body"] for e in events(path) if e["type"] == "note" and e["data"].get("kind") == "ply.recording/v1"]
        require(rows[-1]["phase"] == "terminal" and rows[-1]["complete"] and rows[-1]["exit"] == 0,
                "successful Agent run lacks a complete index")
        for row in rows:
            if row["phase"] == "process":
                invoke([bins / "record", "check", "-f", row["path"]], cwd=root, env=env)
            for path_key, digest_key in (("path", "sha256"), ("inputs", "inputs_sha256"), ("outputs", "outputs_sha256")):
                if digest_key in row:
                    digest = "sha256:" + hashlib.sha256(Path(row[path_key]).read_bytes()).hexdigest()
                    require(digest == row[digest_key], "index does not bind the retained receipt bytes")
        return rows

    indexes = list(evidence.glob("recordings/run.*/index.jsonl"))
    require(len(indexes) == 2, "normal and zero-model Agent runs did not both record")
    rows = [index(path) for path in indexes]
    action = next(row for group in rows for row in group if row.get("role") == "action")
    first = Path(action["path"]).parents[1]
    stdout = invoke([bins / "record", "replay", "-f", action["path"], "-stream", "stdout"], cwd=root, env=env)
    stderr = invoke([bins / "record", "replay", "-f", action["path"], "-stream", "stderr"], cwd=root, env=env)
    require(stdout.stdout == data and stderr.stdout == b"separate-error\n", "default Agent recording lost full separate bytes")
    require(sum(row.get("role") == "verifier" for group in rows for row in group) == 3,
            "pre-check and candidate verifiers were not recorded")

    # Snapshot bytes must survive deletion and later checkpoint continuation.
    snapshots = json.loads(invoke([bins / "record", "replay", "-f", first / "outputs.jsonl", "-json"], cwd=root, env=env).stdout)
    artifacts = snapshots["intent"]["artifacts"]
    session_artifact = next(x for x in artifacts if x["phase"] == "session")
    session_path = Path(os.fsdecode(base64.b64decode(session_artifact["path"])))
    original_session = session_path.read_bytes()
    (work / "result.bin").unlink()
    with fixture(env, lambda _: "A later report.") as (model_env, _):
        invoke([*command, "-checkpoint", "case", "-B", "-turns", "1", "-cycles", "1", home, "--", "Continue."],
               cwd=root, env=model_env, code=2)
    require(session_path.read_bytes() != original_session, "checkpoint fixture did not append")
    for item in artifacts:
        restored = invoke([bins / "record", "replay", "-f", first / "outputs.jsonl", "-stream", item["name"]], cwd=root, env=env).stdout
        expected = original_session if item["phase"] == "session" else b"\x00\xffartifact"
        require(restored == expected, "snapshot changed after resume or source deletion")

    # Explicit missing or failing recorders cannot reach a verifier/model.
    marker = work / "must-not-run"
    checker.write_text("#!/bin/sh\ntouch must-not-run\nexit 0\n")
    for recorder in (root / "absent-record", "/usr/bin/false", "/usr/bin/true"):
        with fixture(env, lambda _: "must not be called") as (model_env, calls):
            invoke([*command, home, "--", "Refuse missing evidence."], cwd=root,
                   env=dict(model_env, AGENT_RECORD=str(recorder)), code=125)
            require(not marker.exists() and not calls, "recorder failure permitted execution")

    # Loss of evidence after an effect must stop before another model call.
    checker.write_text("#!/bin/sh\nexit 1\n")
    proxy = root / "record-proxy"
    proxy.write_text("#!/bin/sh\nfile=\nprevious=\nfor arg do\n"
                     '  if [ "$previous" = -f ]; then file=$arg; fi\n  previous=$arg\ndone\n' +
                     shlex.quote(str(bins / "record")) + ' "$@"\nstatus=$?\n' +
                     'case "$1:$file" in run:*/action.*/session.jsonl) rm -f "$file";; esac\nexit "$status"\n')
    proxy.chmod(0o700)
    with fixture(env, lambda _: "```sh\ntouch must-not-run\n```") as (model_env, calls):
        failed = invoke([*command, home, "--", "Stop if evidence is lost."], cwd=root,
                        env=dict(model_env, AGENT_RECORD=str(proxy)), code=125)
        require(marker.exists() and len(calls) == 1 and failed.stdout == b"", "evidence loss permitted continuation or success")

    checker.write_text("#!/bin/sh\nexit 0\n")
    with fixture(env, lambda _: "Accepted candidate.") as (model_env, calls):
        failed = invoke([*command, "-B", "-record-output", "missing-required-output", home, "--", "Retain the required artifact."],
                        cwd=root, env=model_env, code=125)
        require(len(calls) == 1 and failed.stdout == b"", "accepted stdout escaped before required artifact retention")

    # Force one actual compaction and prove all three Ask files are snapshotted.
    checker.write_text("#!/bin/sh\ntest -f compacted\n")
    records_before = set(evidence.glob("recordings/run.*/index.jsonl"))
    turns = 0
    def compact_response(request):
        nonlocal turns
        if "handoff note" in request.get("instructions", ""):
            return "Continue the same goal; the action created compacted."
        turns += 1
        return "```sh\ntouch compacted\n```" if turns == 1 else "Compacted result."
    with fixture(env, compact_response) as (model_env, calls):
        invoke([*command, "-compact-at", "1", "-compactions", "1", home, "--", "Compact once."], cwd=root, env=model_env)
        require(len(calls) == 3, "expected action, summary and final model calls")
    compact_index, = set(evidence.glob("recordings/run.*/index.jsonl")) - records_before
    compact_rows = index(compact_index)
    require(any(row["phase"] == "summary" for row in compact_rows), "summary session was not linked")
    output = compact_index.parent / "outputs.jsonl"
    metadata = json.loads(invoke([bins / "record", "replay", "-f", output, "-json"], cwd=root, env=env).stdout)
    sessions = [a for a in metadata["intent"]["artifacts"] if a["phase"] == "session"]
    require(len(sessions) == 3, "source, summary and continuation must all be retained")
    for artifact in sessions:
        path = Path(os.fsdecode(base64.b64decode(artifact["path"])))
        original = path.read_bytes()
        path.unlink()
        restored = invoke([bins / "record", "replay", "-f", output, "-stream", artifact["name"]], cwd=root, env=env).stdout
        require(restored == original, "offline compaction snapshot lost bytes")
    print("ok automatic Agent recording: full binary streams, all checks, zero-model pre-check, selected artifacts, checkpoint snapshots, fail-closed dependencies, complete compaction handoff", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, required=True)
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix="bench-record-agent-") as tmp:
        check(args.bin_dir.resolve(), Path(tmp).resolve())


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError, subprocess.SubprocessError) as error:
        raise SystemExit(str(error))
