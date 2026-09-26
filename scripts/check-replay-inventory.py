#!/usr/bin/env python3
"""Exercise public Bench commands through Record and audit inventory coverage.

This is a process-boundary gate. It complements each tool's semantic checks;
it does not infer unselected child sessions or filesystem effects. All receipts
and a coverage report remain under the explicitly selected new output directory.
"""
import argparse
import base64
import json
import hashlib
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import runpy
import selectors
import shutil

ROOT = Path(__file__).resolve().parents[1]
# A new command requires review of its replay boundary before the gate runs.
BOUNDARIES = {
    "a2a": "protocol requests and returned task/artifact objects",
    "a2aserve": "finite listener lifecycle and separate diagnostics",
    "action": "exact request, parked/accepted result and existing effect receipts",
    "agent": "definition selection and checked worker execution",
    "ask": "model requests, response and existing conversation replay",
    "brief": "selected procedure bytes",
    "cage": "selected read-only execution boundary and observed success or refusal",
    "cite": "exact acceptance, rejection and invalid evidence",
    "context": "structured records, exact query and retrieval outcome",
    "draft": "deterministic definition artifacts",
    "hire": "definition creation, validation and adaptation",
    "hone": "replay-verified recovery inspection",
    "improve": "bounded experiment specification, pinned evaluation and tested source proposal",
    "may": "offline gate self-check with isolated temporary approval state",
    "mcp": "current protocol requests and results",
    "mcp-legacy": "legacy protocol requests and refusal against current-only servers",
    "mcpbox": "explicit protocol admission and materialization",
    "mcpserve": "bidirectional initialized protocol session",
    "moniker": "selected registry and theme, reserved name and process outcome",
    "oauth": "non-secret profile selection, private header descriptor and offline child outcome",
    "ply": "model/action/verifier loop and interpreter observations",
    "record": "recording/extraction through its own public executable boundary",
    "rules": "ordered discovered instruction bytes",
    "tend": "durable job input, work and observed transitions",
    "trail": "read-only archive verification",
    "weave": "deterministic task/observation projection",
    "weigh": "explicit typed questions, native probabilities and inference outcome",
}


def require(value, message):
    if not value:
        raise RuntimeError(message)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--native-cage", action="store_true")
    args = parser.parse_args()
    bins, out = args.bin_dir.resolve(), args.output.resolve()
    names = {c["name"] for component in json.loads((ROOT / "components.json").read_text())["components"]
             for c in component["commands"]}
    require(names == set(BOUNDARIES), "replay boundary map differs from components.json: "
            + str(sorted(names.symmetric_difference(BOUNDARIES))))
    out.mkdir(parents=True, exist_ok=False)
    receipts = out / "receipts"
    receipts.mkdir()
    for name in sorted(names):
        require((bins / name).is_file(), "missing independently built executable: " + name)
    support = runpy.run_path(str(ROOT / "scripts/check-integration.py"))
    env = {k: os.environ[k] for k in ("PATH", "TMPDIR", "GOCACHE", "GOMODCACHE", "GOPATH",
                                      "CC", "CXX", "SDKROOT", "DEVELOPER_DIR") if k in os.environ}
    cache = json.loads(subprocess.check_output(["go", "env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH"]))
    for name, value in cache.items():
        env.setdefault(name, value)
    env.update(PATH=str(bins) + os.pathsep + env.get("PATH", os.defpath),
               HOME=str(out / "home"), GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local", ASK_LIVE="",
               BENCH_REPLAY_RECORD_DIR=str(receipts), BENCH_REPLAY_BIN_DIR=str(bins))
    Path(env["HOME"]).mkdir()
    failures = []
    with tempfile.TemporaryDirectory(prefix="bench-replay-fixtures-") as temporary:
        work = Path(temporary).resolve()

        def run(argv, data=b"", code=0, timeout=180):
            return support["invoke"](argv, cwd=work, env=env, data=data, code=code, timeout=timeout)

        try:
            existing = run([sys.executable, ROOT / "scripts/check-integration.py", "--bin-dir", bins, "--record-dir", receipts]
                           + (["--native-cage"] if args.native_cage else []), timeout=900)
            (out / "integration.stdout").write_bytes(existing.stdout)
            (out / "integration.stderr").write_bytes(existing.stderr)
            print("ok existing semantic integration with externally recorded invocations", flush=True)
        except (RuntimeError, subprocess.SubprocessError) as error:
            failures.append(str(error))

        # Meaningful offline boundaries absent from the existing composition
        # suite. These never grant an approval or fetch a credential.
        try:
            run(["git", "init", "-q", work])
            (work / "AGENTS.md").write_text("Fixture instruction: preserve exact bytes.\n")
            run([bins / "rules", work])
            skills = work / "skills"
            run([bins / "brief", "new", "-d", skills, "fixture"])
            env["BRIEF_PATH"] = str(skills)
            run([bins / "brief", "cat", "fixture"])
            run([bins / "draft", "new", work / "draft-project"])
            improve_spec = run([sys.executable, ROOT / "tools/improve/examples/router/spec.py", "--offline"]).stdout
            run([bins / "improve", "-n"], improve_spec)
            run([bins / "may", "check"])
            moniker_contract(bins, work, run)
            run([bins / "cage", "status"])
            confined, receipt = support["recorded_argv"](
                [bins / "cage", "-ro", "--", "/usr/bin/printf", "confined fixture\\n"], env)
            boundary = subprocess.run(confined, cwd=work, env=env, stdin=subprocess.DEVNULL,
                                      capture_output=True, timeout=30)
            require(boundary.returncode in (0,125), "unexpected Cage boundary outcome")
            if boundary.returncode == 0:
                require(boundary.stdout == b"confined fixture\n", "Cage lost child output")
            support["verify_recorded"](receipt, boundary, env)
            run([bins / "oauth", "status"])
            session = work / "seed.jsonl"
            run([bins / "ask", "init", "-f", session])
            run([bins / "hone", "-why", session], code=1)
            run([bins / "record", "run", "-ask", bins / "ask", "-f", work / "record.jsonl",
                 "--", "/usr/bin/printf", "seed\n"])
            env["TEND_ROOT"] = str(work / "queue")
            job = run([bins / "tend", "submit", "--", "/bin/cat"], b"retained job input\n").stdout.decode().strip()
            run([bins / "tend", "work"])
            run([bins / "tend", "show", job])
            run([bins / "tend", "events", job])
            run([bins / "tend", "check"])
            tasks = ROOT / "tools/weave/examples/quickstart/tasks.jsonl"
            observed = (tasks.parent / "observations.jsonl").read_bytes()
            projection = run([bins / "weave", tasks], observed)
            require(b"operations" in projection.stdout and b"controls" in projection.stdout, "Weave lost the ready tasks")
        except (RuntimeError, subprocess.SubprocessError) as error:
            failures.append(str(error))

        try:
            finite_servers(bins, work, env, support, run)
        except (RuntimeError, subprocess.SubprocessError) as error:
            failures.append(str(error))
        try:
            credential_handoff(bins, work, env, receipts, run)
        except (RuntimeError, subprocess.SubprocessError) as error:
            failures.append(str(error))
        try:
            compacted_archive(bins, work, env, receipts, support, run)
        except (RuntimeError, subprocess.SubprocessError) as error:
            failures.append(str(error))

    counts = {name: {"boundary": BOUNDARIES[name], "complete": 0, "incomplete": 0, "other": 0} for name in sorted(names)}
    paths = sorted(receipts.glob("*/session.jsonl"))
    for path in paths:
        # Ask validates its own snapshot. Inspect only enough metadata to map
        # the case; Record remains the authority for process completeness.
        checked = subprocess.run([bins / "ask", "replay", "-check", "-json", path], capture_output=True)
        if checked.returncode:
            failures.append("invalid Ask evidence: " + str(path))
            continue
        events = [json.loads(line) for line in checked.stdout.splitlines()]
        intents = [e["data"]["body"] for e in events if e["type"] == "note"
                   and e["data"].get("kind") == "record.intent/v1"]
        if len(intents) != 1:
            failures.append("missing invocation identity: " + str(path))
            continue
        argv = [os.fsdecode(base64.b64decode(arg)) for arg in intents[0]["argv"]]
        name = Path(argv[0]).name
        if name not in counts:
            failures.append("unknown recorded executable: " + name)
            continue
        if len(argv) > 1 and argv[1] in ("version", "help", "--version", "--help", "-h", "capabilities"):
            counts[name]["other"] += 1
            continue
        verified = subprocess.run([bins / "record", "check", "-ask", bins / "ask", "-f", path], capture_output=True)
        if verified.returncode == 0:
            counts[name]["complete"] += 1
        elif verified.returncode == 125:
            counts[name]["incomplete"] += 1
        else:
            failures.append("unexpected replay verification exit: " + str(path))
    for name, values in counts.items():
        if values["complete"] == 0:
            failures.append(name + ": no completed meaningful record/replay case")
    report = {"schema": 1, "scope": "observed process boundaries", "commands": counts,
              "recordings": len(paths), "failures": failures}
    (out / "coverage.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2), flush=True)
    require(not failures, "replay inventory gate failed; see " + str(out / "coverage.json"))


def moniker_contract(bins, work, run, manifest=None, legacy=True):
    """Compose the real name filter, dispatcher, server, and protocol clients."""
    registry = work / "name registry"
    seen = []

    def identity(value, theme):
        require(all(isinstance(value.get(key), str) and value[key]
                    for key in ("id", "name", "slug", "theme")), "Moniker lost the reserved identity")
        require(len(value["id"]) == 32 and all(c in "0123456789abcdef" for c in value["id"]),
                "Moniker returned an invalid ID")
        require(value["theme"] == theme, "Moniker lost the caller-selected theme")
        require(all(value["id"] != prior["id"] and value["slug"] != prior["slug"] for prior in seen),
                "Moniker reused a reserved identity")
        seen.append(value)

    identity(json.loads(run([bins / "moniker", "-dir", registry, "-json"]).stdout), "playful")
    manifest = manifest or ROOT / "tools/moniker/mcp/manifest.json"
    server = [bins / "mcpserve", manifest, "--", bins / "moniker", "mcp", "-dir", registry]
    blocked = work / "registry-is-a-file"
    blocked.write_text("unrelated caller file\n")
    cases = [("mcp", server, "space")]
    if legacy:
        cases.append(("mcp-legacy", [server[0], "-allow-legacy", *server[1:]], "nature"))
    for client, selected, theme in cases:
        listing = json.loads(run([bins / client, "request", "tools/list", "--", *selected], b"{}").stdout)
        require(any(tool["name"] == "generate_team_name" for tool in listing["tools"]),
                "Moniker MCP discovery lost its descriptor")
        request = json.dumps({"name": "generate_team_name", "arguments": {"theme": theme}}).encode()
        reply = json.loads(run([bins / client, "request", "tools/call", "--", *selected], request).stdout)
        require(not reply.get("isError"), "Moniker MCP call returned an error")
        identity(reply["structuredContent"], theme)
        require(any(part.get("text") == seen[-1]["name"] for part in reply["content"]),
                "Moniker MCP text and structured result disagree")
        failure = json.loads(run([bins / client, "request", "tools/call", "--", *selected[:-1], blocked],
                                 request, code=1).stdout)
        require(failure.get("isError") is True and failure.get("content"),
                "Moniker MCP lost a completed application failure")
        require(blocked.read_text() == "unrelated caller file\n", "Moniker replaced an unrelated file")
    if legacy:
        run([bins / "mcp-legacy", "request", "tools/list", "--", *server], b"{}", code=2)
    print("ok Moniker: literal CLI, distinct reservations, " + ("modern/legacy" if legacy else "modern")
          + " MCP and application failures", flush=True)


def readable_line(stream, label):
    with selectors.DefaultSelector() as ready:
        ready.register(stream, selectors.EVENT_READ)
        require(ready.select(20), label + " did not respond while stdin was open")
        return stream.readline()


def finite_servers(bins, work, env, support, run):
    manifest = ROOT / "tools/mcp/examples/filter-server/manifest.json"
    command, recording = support["recorded_argv"]([bins / "mcpserve", "-allow-legacy", manifest, "--",
                                                   manifest.parent / "dispatch"], env)
    process = subprocess.Popen(command, cwd=work, env=env, stdin=subprocess.PIPE,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    try:
        request = {"jsonrpc":"2.0", "id":1, "method":"initialize", "params":{
            "protocolVersion":"2025-03-26", "capabilities":{}, "clientInfo":{"name":"replay-fixture","version":"1"}}}
        process.stdin.write(json.dumps(request).encode()+b"\n");process.stdin.flush()
        first = readable_line(process.stdout, "MCP server")
        reply = json.loads(first)
        require(reply.get("id") == 1 and "result" in reply, "MCP initialization failed: " + str(reply))
        process.stdin.close();process.stdin=None
        rest, diagnostic = process.communicate(timeout=15)
        result = subprocess.CompletedProcess(command, process.returncode, first+rest, diagnostic)
        require(result.returncode == 0, "MCP server did not finish a finite input session")
        support["verify_recorded"](recording, result, env)
    finally:
        if process.poll() is None:
            process.terminate();process.wait(timeout=15)

    card = work / "card.json"
    card.write_text(json.dumps({"name":"Replay echo", "description":"Finite fixture", "version":"1",
        "skills":[{"id":"echo","name":"Echo","description":"Echo input","tags":["fixture"]}]}))
    command, recording = support["recorded_argv"]([bins / "a2aserve", "-dev-loopback", "-listen", "127.0.0.1:0",
        "-state", work / "a2a-state", "-tend", bins / "tend", card, "--", "/bin/cat"], env)
    process = subprocess.Popen(command, cwd=work, env=env, stdin=subprocess.DEVNULL,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    first = b""
    try:
        first = readable_line(process.stderr, "A2A listener")
        require(b" card " in first, "A2A listener startup failed: " + first.decode(errors="replace"))
        endpoint = first.decode().split(" card ",1)[1].strip().split("/.well-known/")[0]
        request = {"message":{"messageId":"replay-echo","role":"ROLE_USER","parts":[{"text":"retained echo"}]}}
        result = run([bins / "a2a", "request", "-http-loopback", "send", endpoint+"/rpc"], json.dumps(request).encode())
        require(b"retained echo" in result.stdout, "A2A response lost the task artifact")
    finally:
        if process.poll() is None:
            process.terminate()
        stdout, diagnostic = process.communicate(timeout=20)
    result = subprocess.CompletedProcess(command, process.returncode, stdout, first+diagnostic)
    require(result.returncode == 0, "A2A listener did not close gracefully")
    support["verify_recorded"](recording, result, env)


def credential_handoff(bins, work, env, receipts, run):
    secret = b"offline-replay-fixture-private-access-token"
    resource = "https://resource.example.test/mcp"
    home = work / "oauth"
    profile_dir = home / "fixture"
    profile_dir.mkdir(parents=True, mode=0o700)
    profile = {"version":1,"binding":"replay-fixture-binding","name":"fixture","resource":resource,
               "issuer":"https://issuer.example.test", "token_endpoint":"https://issuer.example.test/token",
               "client_id":"fixture", "client_auth":"none"}
    credential = {"version":1,"binding":profile["binding"],"access_token":secret.decode(),
                  "token_type":"Bearer","expiry":"2030-01-01T00:00:00Z"}
    for name,value in (("profile.json",profile),("credential.json",credential)):
        path = profile_dir / name
        path.write_text(json.dumps(value));path.chmod(0o600)
    expected = hashlib.sha256(b"Authorization: Bearer "+secret+b"\n").hexdigest()
    child = work / "check-private-header.py"
    child.write_text("import os, hashlib\nheader = os.fdopen(3, 'rb').read()\n"
                     + "assert hashlib.sha256(header).hexdigest() == " + repr(expected) + "\n"
                     + "print('authenticated fixture')\n")
    env["OAUTH_HOME"] = str(home)
    before = set(receipts.glob("*/session.jsonl"))
    recorded = Path(tempfile.mkdtemp(prefix="invocation.",dir=receipts)) / "session.jsonl"
    result = run([bins / "record", "run", "-ask", bins / "ask", "-f", recorded,
                  "-input", child, "-input", profile_dir / "profile.json", "-label", "resource="+resource,
                  "--", bins / "oauth", "with", "fixture", "--", sys.executable, child])
    require(result.stdout == b"authenticated fixture\n", "OAuth lost its private header handoff")
    shutil.rmtree(home);child.unlink()
    replay = run([bins / "record", "replay", "-ask", bins / "ask", "-f", recorded])
    require(replay.stdout == result.stdout, "OAuth replay depended on credentials or original files")
    for path in set(receipts.glob("*/session.jsonl")) - before:
        raw = path.read_bytes()
        require(secret not in raw, "credential entered plain recorded metadata")
        for line in raw.splitlines():
            event = json.loads(line)
            if event["type"] != "note":
                continue
            note = event["data"]
            if note.get("kind") == "record.chunk/v1":
                require(secret not in base64.b64decode(note["body"]["data"]), "credential entered a recorded stream")
            elif note.get("kind") == "record.intent/v1":
                require(all(secret not in base64.b64decode(arg) for arg in note["body"]["argv"]), "credential entered argv")


def compacted_archive(bins, work, env, receipts, support, run):
    sessions = work / "model-sessions"
    sessions.mkdir()
    source = sessions / "source.jsonl"
    with support["model_fixture"](env, lambda _: "Retain the exact supplied goal and observed result.") as (fixture_env, calls):
        support["invoke"]([bins / "ask", "-q", "-m", "openai/fixture", "-f", source, "Remember the fixture."],
                          cwd=work, env=fixture_env, data=b"observed result: 17\n")
        compacted = Path(support["invoke"]([bins / "ask", "compact", "-q", "-d", sessions, source],
                                          cwd=work, env=fixture_env).stdout.decode().strip())
        require(len(calls) == 2, "compaction made unexpected provider calls")
    selected = sorted(sessions.glob("*.jsonl"))
    require(len(selected) == 3, "compaction did not retain source, summary, and continuation sessions")
    originals = {path.name:path.read_bytes() for path in selected}
    header = json.loads(compacted.read_bytes().splitlines()[0])["data"]
    require(header.get("parent") == "source" and header.get("summary"), "compaction lost lineage")
    archive = Path(tempfile.mkdtemp(prefix="invocation.",dir=receipts)) / "session.jsonl"
    command = [bins / "record", "run", "-ask", bins / "ask", "-f", archive, "-label", "collection=compacted-fixture"]
    for path in selected:
        command.extend(["-session",path])
    command.extend(["--",bins / "ask","replay","-check",source])
    run(command)
    shutil.rmtree(sessions)
    restored = work / "restored-model-sessions"
    restored.mkdir(mode=0o700)
    for i,path in enumerate(selected):
        data = run([bins / "record", "replay", "-ask", bins / "ask", "-f", archive, "-stream", f"artifact:{i}"]).stdout
        require(data == originals[path.name], "archiving changed a child conversation")
        (restored / path.name).write_bytes(data)
        (restored / path.name).chmod(0o600)
        run([bins / "ask", "replay", "-check", restored / path.name])
    checked = run([bins / "trail", "check", restored])
    require(len(checked.stdout.splitlines()) == 3 and all(json.loads(line).get("ok") for line in checked.stdout.splitlines()),
            "Trail did not verify every restored conversation")
    lineage = run([bins / "trail", "lineage", restored / compacted.name, restored])
    relations = {json.loads(line).get("relation") for line in lineage.stdout.splitlines()}
    require({"parent","summary"}.issubset(relations), "offline archive lost parent/summary links")


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, ValueError, subprocess.SubprocessError) as error:
        raise SystemExit(str(error))
