#!/usr/bin/env python3
"""Exercise a packaged two-worker app through real public Bench executables.

Only the model HTTP transport is a loopback fixture. Every Bench command is an
independently built executable, and native Cage must work; missing dependencies
fail rather than skip. No model account, paid request, or personal state is used.
"""

import argparse
import importlib.util
import json
import os
from pathlib import Path
import shutil
import sys
import tempfile


ROOT = Path(__file__).resolve().parents[1]
COMMANDS = ("agent", "ask", "brief", "ply", "cage", "record")
sys.dont_write_bytecode = True


def load_helper(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


contracts = load_helper("bundle_executable_contracts", ROOT / "scripts/check-integration.py")
installer = load_helper("bundle_package_verification", ROOT / "scripts/install_support.py")
require = contracts.require
invoke = contracts.invoke


def program(path, body):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text("#!/bin/sh\nset -eu\n" + body, encoding="utf-8")
    path.chmod(0o755)


def runtime_prefix(bins, target):
    """Copy exact selected packages; never synthesize receipts for raw binaries."""
    (target / "bin").mkdir(parents=True)
    for name in COMMANDS:
        binary = (bins / name).resolve()
        candidates = (binary.parent.parent, bins.parent / "tools" / name,
                      bins.parent / "lib/bench-tools" / name)
        package = next((p for p in candidates if (p / "package.json").is_file()), None)
        require(package is not None, f"missing independent package receipt for {bins / name}")
        receipt = installer.verify_package(package, name)
        require(name in receipt["commands"], f"package {name} does not declare its command")
        require((package / "bin" / name).read_bytes() == binary.read_bytes(),
                f"{bins / name} does not match its independently built package")
        destination = target / "lib/bench-tools" / name
        shutil.copytree(package, destination)
        for command in receipt["commands"]:
            (target / "bin" / command).symlink_to(f"../lib/bench-tools/{name}/bin/{command}")


def app_source(directory):
    directory.mkdir()
    program(directory / "bin/entry", r'''mode=$1
case "$(cat "$BUNDLE_GOAL_FILE")" in
  fixture-exit-0) printf '\000fixture\377'; printf 'fixture diagnostic\n' >&2; exit 0 ;;
  fixture-exit-1) printf '\000fixture\377'; printf 'fixture diagnostic\n' >&2; exit 1 ;;
  fixture-exit-2) printf '\000fixture\377'; printf 'fixture diagnostic\n' >&2; exit 2 ;;
  fixture-exit-3) printf '\000fixture\377'; printf 'fixture diagnostic\n' >&2; exit 3 ;;
  fixture-exit-75) printf '\000fixture\377'; printf 'fixture diagnostic\n' >&2; exit 75 ;;
  fixture-exit-125) printf '\000fixture\377'; printf 'fixture diagnostic\n' >&2; exit 125 ;;
  fixture-exit-130) printf '\000fixture\377'; printf 'fixture diagnostic\n' >&2; exit 130 ;;
esac
printf '%s\n' "$BUNDLE_ROOT" "$BUNDLE_CONTROL" "$BUNDLE_STATE" "$BUNDLE_GOAL_FILE" > paths.txt
printf '%s\n' "$mode" >> "$BUNDLE_CONTROL/adapter-modes.txt"
mkdir -p "$BUNDLE_WORK/writer" "$BUNDLE_WORK/reviewer" "$BUNDLE_STATE/writer" "$BUNDLE_STATE/reviewer"
cat > "$BUNDLE_WORK/evidence.txt"
if [ -f "$BUNDLE_INPUT/source.txt" ]; then
  cp "$BUNDLE_INPUT/source.txt" "$BUNDLE_WORK/writer/source.txt"
fi
if [ "$mode" != resume ]; then
  cp "$BUNDLE_GOAL_FILE" "$BUNDLE_WORK/writer/request.txt"
fi
set --
if [ "$mode" = follow ]; then set -- -B; fi
agent run "$@" -C "$BUNDLE_WORK/writer" -state "$BUNDLE_STATE/writer" \
  -evidence "$BUNDLE_CONTROL/writer" -checkpoint conversation -m openai/fixture -turns 4 \
  -record-input "$BUNDLE_WORK/writer/request.txt" -record-output "$BUNDLE_WORK/writer/draft.txt" \
  -goal-file "$BUNDLE_GOAL_FILE" "$BUNDLE_ROOT/workers/writer" < "$BUNDLE_WORK/evidence.txt" >&2
cp "$BUNDLE_WORK/writer/draft.txt" "$BUNDLE_WORK/reviewer/request.txt"
agent run "$@" -C "$BUNDLE_WORK/reviewer" -state "$BUNDLE_STATE/reviewer" \
  -evidence "$BUNDLE_CONTROL/reviewer" -checkpoint conversation -m openai/fixture -turns 4 \
  -record-input "$BUNDLE_WORK/reviewer/request.txt" -record-output "$BUNDLE_WORK/reviewer/result.txt" \
  -goal-file "$BUNDLE_GOAL_FILE" "$BUNDLE_ROOT/workers/reviewer" < /dev/null >&2
cp "$BUNDLE_WORK/reviewer/result.txt" "$BUNDLE_OUTPUT/result.txt"
cat "$BUNDLE_OUTPUT/result.txt"
''')
    for role, output in (("writer", "draft.txt"), ("reviewer", "result.txt")):
        expert = directory / "workers" / role
        expert.mkdir(parents=True)
        (expert / "AGENTS.md").write_text(
            f"# {role}\nBUNDLE_{role.upper()}_IDENTITY. Read request.txt and preserve its exact bytes in {output}.\n")
        skill = expert / "skills/packet-work"
        skill.mkdir(parents=True)
        (skill / "SKILL.md").write_text(
            "---\nname: packet-work\ndescription: Preserve supplied packet bytes. Use when asked to copy a request into a checked artifact.\n---\n"
            "BUNDLE_LOCAL_PROCEDURE. Copy only the explicitly selected current request.\n")
        program(expert / "bin/check", f"test -f {output}\ncmp -s request.txt {output}\n")
    files = sorted(p.relative_to(directory).as_posix() for p in directory.rglob("*") if p.is_file())
    (directory / "app.json").write_text(json.dumps({
        "schema": 1, "name": "packet-team", "description": "Offline two-worker executable contract fixture.",
        "entry": "bin/entry", "files": files,
        "requires": [*COMMANDS, "sh", "cat", "cmp", "cp", "mkdir"],
        "followup": True, "resume": True,
    }, indent=2) + "\n")


def verify_records(bins, directory, work, env):
    sessions = list(directory.rglob("*.jsonl"))
    require(sessions, "Agent did not retain any Ask evidence")
    record_count = 0
    for session in sessions:
        replay = invoke([bins / "ask", "replay", "-check", "-json", session], cwd=work, env=env)
        events = [json.loads(line) for line in replay.stdout.splitlines()]
        if any(e["type"] == "note" and e["data"].get("kind") == "record.intent/v1" for e in events):
            invoke([bins / "record", "check", "-ask", bins / "ask", "-f", session], cwd=work, env=env)
            record_count += 1
    require(record_count >= 4, "missing separate real Record action/check receipts for both workers")


def check_app(bins, scratch, env):
    source = scratch / "app source"
    prefix = scratch / "runtime prefix"
    runtime_prefix(bins, prefix)
    app_source(source)
    application = scratch / "relocated distribution" / "packet-team"
    application.parent.mkdir()
    build = [bins / "bundle", "build", "-o", application, "-runtime", prefix]
    for name in COMMANDS:
        build.extend(("-tool", name))
    invoke([*build, source], cwd=scratch, env=env)
    require(application.is_file() and os.access(application, os.X_OK), "bundle did not create a runnable app")
    shutil.rmtree(prefix)
    shutil.rmtree(source)
    # Only system utilities remain ambient. No installed Bench command can hide
    # a missing packaged executable or an absolute path back to the build prefix.
    app_env = dict(env, PATH=os.defpath)
    counters = {"writer": 0, "reviewer": 0}
    seen = {"writer": [], "reviewer": []}

    def respond(request):
        instructions = request.get("instructions", "")
        if "You are choosing which skill" in instructions:
            return "packet-work"
        role = next((name for name in counters if f"BUNDLE_{name.upper()}_IDENTITY" in instructions), None)
        require(role is not None, "packaged worker lost its private definition")
        other = "reviewer" if role == "writer" else "writer"
        require(f"BUNDLE_{other.upper()}_IDENTITY" not in instructions,
                "the two workers inherited each other's definition context")
        require("BUNDLE_LOCAL_PROCEDURE" in instructions, "real Brief selection lost the packaged local skill")
        seen[role].append(request)
        counters[role] += 1
        if counters[role] % 2 == 0:
            return f"{role} checked the selected request."
        output = "draft.txt" if role == "writer" else "result.txt"
        # Prove the real native action boundary; a wrapper passing -no-cage would
        # succeed at these writes and fail the selected worker's artifact check.
        return ("```ply\n"
                'if printf forbidden > "$AGENT_HOME/forbidden-definition"; then exit 99; fi\n'
                'if printf forbidden > "$BUNDLE_CONTROL/forbidden-record"; then exit 99; fi\n'
                'if printf forbidden > "$BUNDLE_INPUT/forbidden-input"; then exit 99; fi\n'
                'if printf forbidden > "$BUNDLE_OUTPUT/unchecked-output"; then exit 99; fi\n'
                f"cp request.txt {output}\n```")

    goal = 'Write literal $(touch GOAL_EXECUTED); `touch ALSO_EXECUTED` and preserve "quotes".\nSecond line.'
    evidence = b"BUNDLE_SELECTED_EVIDENCE: $(touch EVIDENCE_EXECUTED)\n"
    workspace = scratch / "workspace with spaces"
    inputs, working, output = (workspace / name for name in ("input", "working", "output"))
    inputs.mkdir(parents=True)
    source_bytes = b"Selected source material stays unchanged.\x00\xff\n"
    (inputs / "source.txt").write_bytes(source_bytes)
    with contracts.model_fixture(app_env, respond) as (fixture_env, calls):
        first = invoke([application, "-w", workspace, "--", goal], cwd=scratch, env=fixture_env, data=evidence)
        require(first.stdout == goal.encode(), "app mixed diagnostics into the exact final artifact")
        require((working / "evidence.txt").read_bytes() == evidence, "argv goal consumed or changed stdin evidence")
        require("BUNDLE_SELECTED_EVIDENCE" in json.dumps(seen["writer"][0]), "Agent did not receive piped evidence")
        require("BUNDLE_SELECTED_EVIDENCE" not in json.dumps(seen["reviewer"][0]),
                "reviewer inherited evidence without an explicit handoff")
        require((working / "writer/source.txt").read_bytes() == source_bytes, "staged input was not handed to writer")
        require((inputs / "source.txt").read_bytes() == source_bytes, "staged input changed")
        require(sorted(p.name for p in workspace.iterdir()) == ["input", "output", "working"], "workspace root polluted")
        require((output / "result.txt").read_bytes() == first.stdout, "checked artifact not published")
        require(sorted(p.name for p in output.iterdir()) == ["result.txt"], "unchecked files reached output")
        require(not (inputs / "forbidden-input").exists(), "Cage allowed a staged input write")
        require(counters == {"writer": 2, "reviewer": 2}, "app did not run both public Agent commands")
        root, control, state, goal_file = map(Path, (working / "paths.txt").read_text().splitlines())
        require(root.is_dir() and control.is_dir() and state.is_dir(), "app did not expose selected runtime roots")
        require(workspace not in control.parents and workspace != control, "controller evidence is writable work")
        require(workspace not in root.parents and workspace != root, "embedded definition is writable work")
        require(goal_file.read_bytes() == goal.encode(), "goal file changed literal bytes")
        require(not (root / "workers/writer/forbidden-definition").exists(), "Cage allowed a definition write")
        require(not (control / "forbidden-record").exists(), "Cage allowed a controller write")
        for name in ("GOAL_EXECUTED", "ALSO_EXECUTED", "EVIDENCE_EXECUTED"):
            require(not any(scratch.rglob(name)), f"literal input was evaluated: {name}")
        checkpoints = {role: control / role / "checkpoints/conversation.current" for role in counters}
        initial_pointers = {role: path.read_bytes() for role, path in checkpoints.items()}
        before = len(calls)
        resumed = invoke([application, "-resume", workspace], cwd=scratch, env=fixture_env)
        require(resumed.stdout == goal.encode() and len(calls) == before,
                "unchanged resume bypassed the team's zero-model passing check")
        update = 'Clarification: preserve the original context and write this revised goal.\n'
        followed = invoke([application, "-c", workspace, "--", update], cwd=scratch, env=fixture_env,
                          data=b"BUNDLE_FOLLOWUP_EVIDENCE\n")
        require((output / "result.txt").read_bytes() == update.encode(), "follow-up did not publish checked revision")
        require((inputs / "source.txt").read_bytes() == source_bytes, "continuation changed input")
        require(followed.stdout == update.encode(), "follow-up reused the previously accepted artifact")
        require(counters == {"writer": 4, "reviewer": 4}, "follow-up did not reach both Agent checkpoints")
        for role, pointer in checkpoints.items():
            require(pointer.read_bytes() == initial_pointers[role], "follow-up replaced the Ask conversation")
            request_text = json.dumps(seen[role][-2])
            require(goal in request_text or json.dumps(goal)[1:-1] in request_text,
                    f"{role} follow-up lost its earlier goal")
            require(update.strip() in request_text, f"{role} follow-up lost the new request")
        require(goal_file.read_bytes() == goal.encode(), "follow-up overwrote the retained original request")
        require((control / "adapter-modes.txt").read_text().splitlines() == ["run", "resume", "follow"],
                "app conflated initial run, resume, and follow-up")
        print("ok bundle: relocated single file, real two-worker team, input/working/output layout, native Cage, literal input, continuation", flush=True)

        piped_goal = b"A fresh request supplied entirely through stdin.\n"
        piped = scratch / "piped workspace"
        result = invoke([application, "-w", piped], cwd=scratch, env=fixture_env, data=piped_goal)
        require(result.stdout == piped_goal and (piped / "working/evidence.txt").read_bytes() == b"",
                "stdin goal was changed or forwarded a second time as evidence")
        before = len(calls)
        for code in (0, 1, 2, 3, 75, 125, 130):
            result = invoke([application, "-w", scratch / f"status-{code}", "--", f"fixture-exit-{code}"],
                            cwd=scratch, env=fixture_env, code=code)
            require(result.stdout == b"\x00fixture\xff", f"exit {code} changed binary stdout")
            require(b"fixture diagnostic\n" in result.stderr, f"exit {code} lost child stderr")
        require(len(calls) == before, "an adapter outcome was automatically retried through a model")
        print("ok bundle: stdin goal, exact binary streams, decline/wait/unknown/interruption statuses", flush=True)
    verify_records(bins, control, scratch, env)
    print("ok bundle: retained Agent/Ask sessions and separate Record receipts replay offline", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, default=ROOT / ".build/bin",
                        help="independently built commands with their package receipts")
    args = parser.parse_args()
    bins = args.bin_dir.expanduser().resolve()
    for name in ("bundle", *COMMANDS):
        require((bins / name).is_file() and os.access(bins / name, os.X_OK), f"missing executable: {bins / name}")
    with tempfile.TemporaryDirectory(prefix="bench-bundle-integration-") as temporary:
        scratch = Path(temporary).resolve()
        home = scratch / "home"
        home.mkdir()
        temporary_files = scratch / "tmp"
        temporary_files.mkdir()
        keep = ("PATH", "TMPDIR", "SYSTEMROOT", "GOCACHE", "GOMODCACHE", "GOPATH",
                "CGO_ENABLED", "CC", "CXX", "SDKROOT", "DEVELOPER_DIR")
        env = {key: os.environ[key] for key in keep if key in os.environ}
        env.update(HOME=str(home), PATH=env.get("PATH", os.defpath), ASK_LIVE="", PYTHONDONTWRITEBYTECODE="1",
                   GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local",
                   TMPDIR=str(temporary_files),
                   XDG_CONFIG_HOME=str(home / ".config"), XDG_STATE_HOME=str(home / ".local/state"),
                   XDG_CACHE_HOME=str(home / ".cache"))
        invoke([bins / "cage", "check"], cwd=scratch, env=env)
        check_app(bins, scratch, env)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (RuntimeError, ValueError, OSError) as error:
        print(f"check-bundle: {error}", file=sys.stderr)
        raise SystemExit(1)
