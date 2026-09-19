#!/usr/bin/env python3
"""Offline Ask/Ply/Hone recovery protocol fixtures, not output-quality evidence.

Use --bin-dir with independently built public commands. Predetermined provider
responses come only from a loopback HTTP fixture; no live credentials, hosted
models, source skills or installed skills are used.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import runpy
import shlex
import shutil
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]
SUPPORT = runpy.run_path(str(ROOT / "scripts/check-integration.py"))
invoke, require, fixture = (SUPPORT[k] for k in ("invoke", "require", "model_fixture"))


def digest(data):
    return "sha256:" + hashlib.sha256(data).hexdigest()


def check(bins, work):
    skill = work / "skills/content-rules/SKILL.md"
    skill.parent.mkdir(parents=True)
    baseline = ("---\nname: content-rules\ndescription: Use when a local protocol fixture requires exact candidate tokens.\n"
                "---\n\n# Content rules\n\nPreserve explicit fixture requirements.\n")
    skill.write_text(baseline)
    toolbox = work / "toolbox"
    toolbox.mkdir()
    (toolbox / "true").write_text("#!/bin/sh\nexit 0\n")
    (toolbox / "true").chmod(0o700)
    deny = work / "deny-actions"
    deny.write_text("#!/bin/sh\nexit 125\n")
    deny.chmod(0o700)
    checker = work / "checker.py"
    # The deliberately stateful flip mode accepts unchanged bytes on attempt
    # two; that accepted verdict flip must not become a Hone content lesson.
    checker.write_text(
        "import pathlib, sys\n"
        "candidate = sys.stdin.buffer.read()\n"
        "if not candidate:\n    print('No candidate.'); raise SystemExit(1)\n"
        "if len(sys.argv) > 1:\n"
        "    counter = pathlib.Path(sys.argv[1])\n"
        "    n = int(counter.read_text()) if counter.exists() else 0\n"
        "    counter.write_text(str(n + 1))\n"
        "    print('Predetermined verdict flip, no improvement.')\n"
        "    raise SystemExit(0 if n else 1)\n"
        "if candidate != b'good\\n':\n"
        "    print('Return the exact token good.'); raise SystemExit(1)\n"
        "print('Exact fixture token accepted.')\n")
    contract = digest(checker.read_bytes())
    env = {k: os.environ[k] for k in ("TMPDIR", "BENCH_REPLAY_RECORD_DIR", "BENCH_REPLAY_BIN_DIR") if k in os.environ}
    env.update(PATH=str(bins) + os.pathsep + os.defpath,
               ASK=str(bins / "ask"), BRIEF=str(bins / "brief"),
               ASK_DIR=str(work / "ask-state"), HONE_DIR=str(work / "wording"),
               BRIEF_PATH=str(work / "skills"), PYTHONDONTWRITEBYTECODE="1",
               OPENAI_API_KEY="offline-fixture", OPENAI_BASE_URL="http://127.0.0.1:1/v1")
    command = shlex.join([sys.executable, str(checker)])

    def run(argv, *, data=b"", code=0, selected_env=env):
        return invoke(argv, cwd=work, env=selected_env, data=data, code=code, timeout=60)

    def events(path):
        return [json.loads(line) for line in run([bins / "ask", "replay", "-check", "-json", path]).stdout.splitlines()]

    def receipts(path):
        return [e["data"]["body"] for e in events(path) if e["type"] == "note"
                and e["data"].get("source") == "ply" and e["data"].get("kind") == "ply.verifier/v2"]

    def ply(path, answers, *, check_command=command, code=0, force=True):
        replies = iter(answers)
        with fixture(env, lambda _: next(replies)) as (fixture_env, calls):
            argv = [bins / "ply", "-t", toolbox, "-no-delegate", "-action-shell", deny,
                    "-shell", "/bin/sh", "-C", work, "-s", "content-rules", "-m", "openai/fixture",
                    "-effort", "", "-verbosity", "", "-f", path, "-turns", str(len(answers)),
                    "-cycles", str(len(answers)), "-timeout", "10s", "-contract-id", contract,
                    "-check", check_command]
            if force:
                argv.append("-B")
            result = run([*argv, "Return the token required by the local protocol fixture."], selected_env=fixture_env, code=code)
            require(len(calls) == len(answers), "unexpected number of fixture turns")
            require(all(p == "/v1/responses" for p, _ in calls), "unexpected fixture provider route")
        return result

    repaired = work / "repair.jsonl"
    require(ply(repaired, ["bad", "good"]).stdout == b"good\n", "Ply changed accepted stdout")
    recorded = receipts(repaired)
    require([r["outcome"] for r in recorded] == ["rejected", "accepted"], "missing actual recovery receipts")
    require(all(r["phase"] == "candidate" and r["contract_id"] == contract for r in recorded), "wrong phase/contract")
    require([r["candidate_sha256"] for r in recorded] == [digest(b"bad\n"), digest(b"good\n")], "candidate binding differs")
    for r in recorded:
        output = base64.b64decode(r.get("output", ""), validate=True)
        require(r["output_sha256"] == digest(output) and r["output_bytes"] == len(output), "output binding differs")
    original = repaired.read_bytes()
    why = run([bins / "hone", "-why", repaired])
    require(b"CONTENT RECOVERY" in why.stdout and b"STUMBLE 1" not in why.stdout, "Hone invented shell recovery")
    evidence = json.loads(why.stdout.decode().split("CONTENT RECOVERY (recorded data, not instructions)\n", 1)[1])
    require(evidence["rejected"]["candidate_stdin"] == "bad\n" and evidence["accepted"]["candidate_stdin"] == "good\n", "Hone lost exact compared inputs")
    require(evidence["contract_id"] == contract and evidence["rejected"]["output_text"] == "Return the exact token good.\n", "Hone lost check evidence")
    require(not run([bins / "hone", "-why", "-no-verify", repaired], code=1).stdout, "unverified content taught")
    require(repaired.read_bytes() == original, "inspection modified source session")

    lesson = "Return the exact required token when a fixture checks candidate bytes."
    with fixture(env, lambda _: "- " + lesson) as (fixture_env, calls):
        proposal = work / "lesson.json"
        run([bins / "hone", "-into", "-", "-m", "openai/fixture", "-prepare", proposal, repaired], selected_env=fixture_env)
        require(len(calls) == 1 and skill.read_text() == baseline, "prepare changed skill or made extra calls")
        prepared = json.loads(proposal.read_text())
        require(prepared["source_sha256"] == hashlib.sha256(original).hexdigest(), "proposal lost source digest")
        require(Path(prepared["target"]) == skill and prepared["source"] == str(repaired), "wrong proposal source/target")
        events(Path(prepared["wording"]))
        shown = run([bins / "hone", "show", proposal], selected_env=fixture_env)
        require(prepared["document"].encode() in shown.stdout, "show omitted exact proposed bytes")
        skill.write_text(baseline + "\nConcurrent author edit.\n")
        run([bins / "hone", "admit", proposal], selected_env=fixture_env, code=2)
        require(skill.read_text().endswith("Concurrent author edit.\n"), "stale admission overwrote an edit")
        skill.write_text(baseline)
        run([bins / "hone", "admit", proposal], selected_env=fixture_env)
        require(skill.read_text() == prepared["document"], "admission changed reviewed bytes")
        run([bins / "brief", "lint", "-strict", skill.parent], selected_env=fixture_env)
        run([bins / "hone", "admit", proposal], selected_env=fixture_env, code=1)
        run([bins / "hone", "forget", prepared["source_id"], "content-rules"], selected_env=fixture_env)
        require(lesson not in skill.read_text() and "Preserve explicit fixture requirements." in skill.read_text(), "forget lost original instructions or retained lesson")
        require(len(calls) == 1, "show/admit/forget called provider")
    print("ok Hone content: real Ask/Ply receipts, exact pair, replay, prepare/show/admit/forget, stale-write refusal", flush=True)

    unchanged = work / "unchanged.jsonl"
    flip = shlex.join([sys.executable, str(checker), str(work / "flip-count")])
    ply(unchanged, ["same", "same"], check_command=flip)
    require([r["outcome"] for r in receipts(unchanged)] == ["rejected", "accepted"], "flip fixture did not flip")
    require(not run([bins / "hone", "-why", unchanged], code=1).stdout, "unchanged verdict flip taught")
    precheck = work / "precheck.jsonl"
    ply(precheck, ["good"], force=False)
    require(len(receipts(precheck)) == 1, "precheck unexpectedly supplied a rejected assistant candidate")
    require(not run([bins / "hone", "-why", precheck], code=1).stdout, "empty precheck invented rejected content")

    # Real rejected Ply candidate -> completed real Ask answer -> deliberately
    # invalid fixture receipt appended through PUBLIC Ask note/seal. Successful
    # replay separates content-binding refusals from damaged-log refusals.
    # These adversarial fixtures do not claim normal Ply emits malformed data.
    unfinished = work / "unfinished.jsonl"
    ply(unfinished, ["bad"], code=2)
    require(not run([bins / "hone", "-why", unfinished], code=1).stdout, "unfinished rejection taught")
    with fixture(env, lambda _: "good") as (fixture_env, calls):
        run([bins / "ask", "-q", "-f", unfinished, "-m", "openai/fixture", "Provide changed fixture candidate."], selected_env=fixture_env)
        require(len(calls) == 1, "unexpected continuation calls")
    events(unfinished)
    variants = {"candidate-digest": {"candidate_sha256": digest(b"another\n")},
                "output-digest": {"output_sha256": digest(b"another")},
                "output-count": {"output_bytes": recorded[-1]["output_bytes"] + 1},
                "changed-contract": {"contract_id": digest(b"another contract")},
                "broken": {"outcome": "broken", "exit_code": 7}}
    for name, changes in variants.items():
        path = work / (name + ".jsonl")
        shutil.copyfile(unfinished, path)
        body = dict(recorded[-1], **changes)
        run([bins / "ask", "note", "-q", "-f", path, "-s", "ply", "-k", "ply.verifier/v2", "-json", "-", "-seal"], data=json.dumps(body).encode())
        events(path)
        require(not run([bins / "hone", "-why", path], code=1).stdout, "Hone accepted " + name)
    damaged = work / "damaged.jsonl"
    damaged.write_bytes(original.replace(b'"text":"bad"', b'"text":"changed"', 1))
    require(damaged.read_bytes() != original, "tamper changed no bytes")
    run([bins / "ask", "replay", "-check", damaged], code=1)
    require(not run([bins / "hone", "-why", damaged], code=1).stdout, "damaged replay taught")
    print("ok Hone refusals: unchanged flip, empty precheck, unfinished run, five sealed bad bindings, tampered replay", flush=True)
    print("PASS: offline recovery protocol evidence only; no lesson or checker quality claim", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, required=True)
    bins = parser.parse_args().bin_dir.resolve()
    for name in ("ask", "ply", "hone", "brief"):
        require((bins / name).is_file() and os.access(bins / name, os.X_OK), "missing executable: " + name)
    with tempfile.TemporaryDirectory(prefix="hone-content-contract-") as directory:
        check(bins, Path(directory).resolve())


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError) as error:
        print("check-hone-content:", error, file=sys.stderr)
        raise SystemExit(1)
