#!/usr/bin/env python3
"""Offline executable contracts for the imported tools; no shared runtime.

Build commands separately, then pass their directory with --bin-dir. All data
and helper programs live in a temporary directory. The only model transport is
a loopback HTTP fixture. May's operating-system-user store cannot be redirected
with HOME: its parked adapter is therefore a strict fixture here, while May's
own isolated acceptance suite remains part of the independent component checks.
"""

import argparse
import base64
from contextlib import contextmanager
import hashlib
import json
import os
from pathlib import Path
import shutil
import shlex
import signal
import subprocess
import sys
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer


ROOT = Path(__file__).resolve().parents[1]
REQUIRED = ("a2a", "a2aserve", "hire", "agent", "ask", "brief", "ply", "cage", "mcp", "mcpbox", "hone", "context", "cite", "action", "may", "trail", "tend", "weave")


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def invoke(argv, *, cwd, env, data=b"", code=0, timeout=180):
    command = [str(arg) for arg in argv]
    with subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE, cwd=cwd, env=env, start_new_session=True) as process:
        try:
            stdout, stderr = process.communicate(data, timeout=timeout)
        except subprocess.TimeoutExpired:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass
            process.communicate()
            raise
        result = subprocess.CompletedProcess(command, process.returncode, stdout, stderr)
    require(result.returncode == code,
            f"{Path(str(argv[0])).name} {list(map(str, argv[1:]))}: exit {result.returncode}, expected {code}\n"
            + result.stdout.decode(errors="replace") + result.stderr.decode(errors="replace"))
    return result


def write_program(path, body):
    path.write_text(f"#!{sys.executable}\n" + body, encoding="utf-8")
    path.chmod(0o700)
    return path


def go_contract(component, test, env):
    result = invoke(["go", "test", "-count=1", "-json", "-run", f"^{test}$", "."],
                    cwd=ROOT / "tools" / component, env=env)
    events = [json.loads(line) for line in result.stdout.splitlines()]
    require(any(e.get("Test") == test and e.get("Action") == "pass" for e in events),
            f"{component}/{test}: required test did not pass")
    require(not any(e.get("Action") == "skip" for e in events),
            f"{component}/{test}: executable contract silently skipped")
    print(f"ok {component}: {test} (required, no skips)", flush=True)


def check_context_cite(bins, work, env):
    row = {"kind": "context", "version": 1, "source": "fixture", "type": "document",
           "id": "hours", "title": "Offline fixture", "retrieved_at": "2026-01-01T00:00:00Z",
           "content": {"text": "The fictional library opens at 09:00."},
           "citation": {"locator": "hours", "url": "https://example.test/hours"},
           "extra": {"preserve": [1, 2]}}
    raw = json.dumps(row).encode() + b"\n"
    result = invoke([bins / "context", "merge"], cwd=work, env=env, data=raw)
    require(result.stderr == b"", "Context mixed diagnostics into a successful fixture merge")
    normalized = json.loads(result.stdout)
    require(normalized["extra"] == row["extra"], "Context lost an unknown structured field")
    checked = invoke([bins / "context", "check"], cwd=work, env=env, data=result.stdout)
    require(checked.stdout == b"", "Context check emitted evidence instead of a status")
    evidence = work / "evidence.jsonl"
    evidence.write_bytes(result.stdout)
    ref = normalized["ref"]
    candidate = f"  Opening time: 09:00. [{ref}]({row['citation']['url']})\n\n".encode()
    accepted = invoke([bins / "cite", evidence], cwd=work, env=env, data=candidate)
    require(accepted.stdout == candidate and accepted.stderr == b"", "Cite changed accepted candidate bytes")
    rejected = invoke([bins / "cite", evidence], cwd=work, env=env,
                      data=candidate.replace(b"example.test/hours", b"example.test/other"), code=1)
    require(rejected.stdout == b"" and rejected.stderr, "Cite leaked a rejected candidate or hid its diagnostic")
    duplicate = dict(normalized)
    duplicate["citation"] = {"locator": "hours", "url": "https://example.test/conflict"}
    evidence.write_bytes(result.stdout + json.dumps(duplicate).encode() + b"\n")
    broken = invoke([bins / "cite", evidence], cwd=work, env=env, data=candidate, code=2)
    require(broken.stdout == b"" and broken.stderr, "Cite failed to separate broken evidence from rejection")
    evidence.write_bytes(result.stdout)
    print("ok Context -> Cite: structured evidence, exact output, rejection and broken-evidence status", flush=True)
    return result.stdout


@contextmanager
def model_fixture(env, respond):
    """A local provider fixture shared by executable composition checks."""
    calls = []

    class Fixture(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            calls.append((self.path, request))
            answer = respond(request)
            response = {"id": "resp_fixture", "object": "response", "status": "in_progress",
                        "model": "fixture", "output": []}
            events = [
                {"type": "response.created", "sequence_number": 0, "response": response},
                {"type": "response.output_text.delta", "sequence_number": 1, "item_id": "msg_fixture",
                 "output_index": 0, "content_index": 0, "delta": answer},
                {"type": "response.completed", "sequence_number": 2, "response": {
                    **response, "status": "completed", "output": [{"type": "message", "id": "msg_fixture",
                    "role": "assistant", "status": "completed", "content": [{"type": "output_text",
                    "text": answer, "annotations": []}]}],
                    "usage": {"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}}},
            ]
            wire = b"".join(("event: " + item["type"] + "\ndata: " + json.dumps(item) + "\n\n").encode()
                            for item in events)
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Content-Length", str(len(wire)))
            self.end_headers()
            self.wfile.write(wire)

    server = HTTPServer(("127.0.0.1", 0), Fixture)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    fixture_env = dict(env, OPENAI_API_KEY="offline-fixture", OPENAI_BASE_URL=f"http://127.0.0.1:{server.server_port}/v1")
    try:
        yield fixture_env, calls
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=5)


def create_ask_session(bins, work, env, evidence):
    """Have the actual Ask binary create a session through a loopback wire fixture."""
    archive = work / "archive"
    archive.mkdir()
    session = archive / "action.jsonl"
    answer = "Offline fixture answer."
    with model_fixture(env, lambda _: answer) as (fixture_env, calls):
        result = invoke([bins / "ask", "-q", "-f", session, "-m", "openai/fixture", "Read the supplied evidence."],
                        cwd=work, env=fixture_env, data=evidence)
        require(result.stdout == (answer + "\n").encode(), "Ask did not preserve fixture answer output")
        require(len(calls) == 1, "Ask made an unexpected number of fixture requests")
        require(calls[0][0] == "/v1/responses", "Ask fixture hit an unexpected endpoint")
    verified = invoke([bins / "ask", "replay", "-check", "-json", session], cwd=work, env=env)
    events = [json.loads(line) for line in verified.stdout.splitlines()]
    require(any(e["type"] == "user" and e["data"].get("evidence") for e in events),
            "Ask did not record a Context evidence manifest")
    return archive, session


def check_agent(bins, work, env, agent):
    """First prove what the existing Agent already does before replacing it."""
    agent_env = dict(env, **{"AGENT_" + name.upper(): str(bins / name)
                            for name in ("ask", "brief", "ply", "cage", "trail")})
    home = work / "agent home"
    invoke([bins / "hire", "new", "-home", home, "Sort supplied records"], cwd=work, env=agent_env)
    (home / "AGENTS.md").write_text("# Instructions\nPARENT_PRIVATE_IDENTITY. Sort records, then verify exact output.\n")
    (home / "MEMORY.md").write_text("# Memory\nPARENT_CURATED_FACT. Preserve input bytes.\n")
    (home / "GOAL.md").write_text("Sort and deduplicate supplied records into sorted.txt.\n")
    (home / "work/input.txt").write_bytes(b"pear\napple\npear\nbanana\n")
    skill = home / "skills/sort-records"
    skill.mkdir()
    (skill / "SKILL.md").write_text("---\nname: sort-records\ndescription: Sort and deduplicate records. Use when asked to sort supplied records into an output file.\n---\nLOCAL_SORT_PROCEDURE. Read the input; sort it; compare exact bytes.\n")
    write_program(home / "bin/check", '''import pathlib, sys
path = pathlib.Path("sorted.txt")
if not path.exists() or path.read_bytes() != b"apple\\nbanana\\npear\\n":
    print("Expected exact sorted and deduplicated input records.")
    sys.exit(1)
''')
    turns = []

    def respond(request):
        if "You are choosing which skill" in request.get("instructions", ""):
            return "sort-records"
        turns.append(request)
        replies = ["```ply\nprintf 'wrong\\n' > sorted.txt\n```", "First candidate.",
                   "```ply\nsort -u input.txt > sorted.txt\n```", "sorted.txt is verified."]
        return replies[min(len(turns), len(replies)) - 1]

    evidence = b"INPUT_EVIDENCE: $(touch UNEXECUTED) is literal supplied text.\n"
    with model_fixture(agent_env, respond) as (fixture_env, calls):
        result = invoke([agent, "run", "-no-cage", "-m", "openai/fixture", home,
                         "--", "Sort and deduplicate supplied records."], cwd=work, env=fixture_env, data=evidence)
        require(result.stdout == b"sorted.txt is verified.\n", "Agent mixed progress into its answer")
        require((home / "work/sorted.txt").read_bytes() == b"apple\nbanana\npear\n", "Agent did not create the checked artifact")
        require(not (home / "work/UNEXECUTED").exists(), "Supplied input was evaluated as shell text")
        require(len(turns) == 4, "Agent did not reuse Ply's rejection/repair loop")
        for token in ("PARENT_PRIVATE_IDENTITY", "PARENT_CURATED_FACT", "LOCAL_SORT_PROCEDURE"):
            require(token in turns[0].get("instructions", ""), "Agent omitted " + token)
        require("INPUT_EVIDENCE" in json.dumps(turns[0].get("input")), "Agent lost piped evidence")
        for _, request in calls:
            if "You are choosing which skill" in request.get("instructions", ""):
                require("PARENT_CURATED_FACT" not in json.dumps(request), "Private memory leaked into selection")
        before = len(calls)
        again = invoke([agent, "run", "-no-cage", "-m", "openai/fixture", home], cwd=work, env=fixture_env)
        require(not again.stdout and len(calls) == before, "Passing Agent pre-check called a model or invented output")
    sessions = list((home / ".agent/runs").glob("*.jsonl"))
    require(len(sessions) == 1, "Agent pre-check minted another execution session")
    replay = invoke([bins / "ask", "replay", "-check", "-json", sessions[0]], cwd=work, env=agent_env)
    events = [json.loads(line) for line in replay.stdout.splitlines()]
    verdicts = [e["data"]["body"].get("outcome") for e in events
                if e["type"] == "note" and e["data"].get("kind") == "ply.verifier/v2"]
    require("rejected" in verdicts and "accepted" in verdicts, "Agent did not retain Ply's verified rejection and acceptance")
    print("ok Agent -> Brief -> Ply -> Ask: private identity, memory, local skill, piped evidence, real artifact, rejected candidate repaired, replay, zero-model re-entry", flush=True)

    child = home / "agents/reviewer"
    invoke([bins / "hire", "new", "-home", child, "Review one result"], cwd=work, env=agent_env)
    (child / "AGENTS.md").write_text("# Instructions\nCHILD_PRIVATE_IDENTITY. Review only the explicitly supplied result.\n")
    (child / "GOAL.md").write_text("Write reviewed.txt containing reviewed.\n")
    write_program(child / "bin/check", '''import pathlib, sys
path = pathlib.Path("reviewed.txt")
sys.exit(0 if path.exists() and path.read_bytes() == b"reviewed\\n" else 1)
''')
    write_program(home / "bin/check", '''import pathlib, sys
path = pathlib.Path("review.txt")
sys.exit(0 if path.exists() and path.read_bytes() == b"Child review verified.\\n" else 1)
''')
    parent_turns, child_turns = [], []

    def delegate(request):
        instructions = request.get("instructions", "")
        if "You are choosing which skill" in instructions:
            return "none"
        if "CHILD_PRIVATE_IDENTITY" in instructions:
            child_turns.append(request)
            return "```ply\nprintf 'reviewed\\n' > reviewed.txt\n```" if len(child_turns) == 1 else "Child review verified."
        parent_turns.append(request)
        if len(parent_turns) == 1:
            return ('```ply\nprintf \'EXPLICIT_CHILD_INPUT\\n\' | "$AGENT_BIN" specialist "$AGENT_HOME" reviewer '
                    '-no-cage -m openai/fixture -- \'Review the supplied result.\' > review.txt\n```')
        return "Parent result verified."

    with model_fixture(agent_env, delegate) as (fixture_env, calls):
        result = invoke([agent, "run", "-no-cage", "-m", "openai/fixture", home,
                         "--", "Ask the reviewer to review the result; retain its answer."], cwd=work, env=fixture_env)
        require(result.stdout == b"Parent result verified.\n", "Nested Agent answer was not collected")
        require(len(parent_turns) == 2 and len(child_turns) == 2, "Agent did not compose a bounded specialist process")
        child_request = json.dumps(child_turns[0])
        require("EXPLICIT_CHILD_INPUT" in child_request, "Child did not receive explicit stdin")
        require("PARENT_PRIVATE_IDENTITY" not in child_request and "PARENT_CURATED_FACT" not in child_request,
                "Child inherited the parent's private context")
    for root in (home, child):
        for session in (root / ".agent/runs").glob("*.jsonl"):
            invoke([bins / "ask", "replay", "-check", session], cwd=work, env=agent_env)
    print("ok Agent -> Agent: ordinary nested command, explicit stdin, collected stdout, separate private context and replayable sessions (explicit host boundary)", flush=True)

    # Reuse MCP's shipped local server and public admission command. Agent
    # receives only an executable; it has no MCP-specific code or catalogue.
    hello = work / "hello-server"
    invoke(["go", "build", "-o", hello, "./examples/hello-server"], cwd=ROOT / "tools/mcp", env=env)
    box = work / "hello.mcp"
    invoke([bins / "mcpbox", "make", "-mcp", bins / "mcp", box, "--", hello], cwd=work, env=env)
    invoke([bins / "mcpbox", "admit", box, "tools", "hello"], cwd=work, env=env)
    (home / "tools/hello").symlink_to(box / "tools/hello")
    write_program(home / "bin/check", '''import json, pathlib, sys
path = pathlib.Path("greeting.json")
if not path.exists(): sys.exit(1)
value = json.loads(path.read_bytes())
sys.exit(0 if value.get("structuredContent", {}).get("greeting") == "hello, Unix" else 1)
''')
    mcp_turns = []

    def use_mcp(request):
        if "You are choosing which skill" in request.get("instructions", ""):
            return "none"
        mcp_turns.append(request)
        return ('```ply\nprintf \'{"name":"Unix"}\\n\' | hello > greeting.json\n```'
                if len(mcp_turns) == 1 else "greeting.json is verified.")

    with model_fixture(agent_env, use_mcp) as (fixture_env, calls):
        result = invoke([agent, "run", "-no-cage", "-m", "openai/fixture", home,
                         "--", "Use hello to greet Unix and save greeting.json."], cwd=work, env=fixture_env)
        require(result.stdout == b"greeting.json is verified.\n" and len(mcp_turns) == 2,
                "Agent did not use the admitted MCP program as an ordinary tool")
    print("ok Agent -> admitted MCP program: real local MCP server, MCPbox admission, PATH tool and executable output check; no Agent MCP implementation", flush=True)


def check_portable_agent(bins, work, env, agent, native_cage=False):
    definition = work / "portable expert"
    invoke([bins / "hire", "new", definition, "Normalize one supplied result"], cwd=work, env=env)
    (definition / "AGENTS.md").write_text("# Operating instructions\nPORTABLE_PRIVATE_EXPERT. Preserve input and produce the requested output.\n")
    (definition / "GOAL.md").write_text("UNSELECTED_STANDING_GOAL must not replace an explicit invocation goal.\n")
    write_program(definition / "bin/check", '''import pathlib, sys
output = pathlib.Path("result.txt")
sys.exit(0 if output.exists() and output.read_bytes() == pathlib.Path("input.txt").read_bytes() else 1)
''')
    before = {p.relative_to(definition).as_posix(): p.read_bytes() for p in definition.rglob("*") if p.is_file()}
    agent_env = dict(env, **{"AGENT_" + name.upper(): str(bins / name) for name in ("ask", "brief", "ply", "cage")})
    for name, private_goal in (("first workspace", False), ("second workspace", True)):
        workspace, control = work / name, work / (name + " evidence")
        workspace.mkdir()
        (workspace / "input.txt").write_text(name + "\n")
        goal = "EXPLICIT_PORTABLE_GOAL. Copy input.txt to result.txt exactly."
        inspection = invoke([agent, "show", "-C", workspace, "-evidence", control, definition], cwd=work, env=agent_env)
        require(str(workspace).encode() in inspection.stdout and b"PORTABLE_PRIVATE_EXPERT" in inspection.stdout,
                "Portable show does not inspect the native definition/workspace composition")
        require(not control.exists() and not (workspace / "state").exists(), "Inspection created runtime state")
        argv = [agent, "run", "-m", "openai/fixture", "-C", workspace, "-evidence", control]
        if not native_cage:
            argv.append("-no-cage")
        if private_goal:
            goal_file = work / "private-goal.md"
            goal_file.write_text(goal)
            argv += ["-goal-file", goal_file, definition]
        else:
            argv += [definition, "--", goal]
        turns = []

        def respond(request):
            turns.append(request)
            guard = 'if (printf changed > "$AGENT_HOME/AGENTS.md") 2>/dev/null; then exit 99; fi\n' if native_cage else ''
            return ('```ply\n' + guard + 'cp input.txt result.txt\n```') if len(turns) == 1 else "result.txt is verified."

        with model_fixture(agent_env, respond) as (fixture_env, calls):
            result = invoke(argv, cwd=work, env=fixture_env, data=b"PORTABLE_STDIN_EVIDENCE\n")
            require(result.stdout == b"result.txt is verified.\n" and len(turns) == 2, "Portable Agent did not finish through Ply")
            request = json.dumps(turns[0])
            require("EXPLICIT_PORTABLE_GOAL" in request and "PORTABLE_STDIN_EVIDENCE" in request,
                    "Portable invocation lost goal or stdin")
            require("UNSELECTED_STANDING_GOAL" not in request, "Standing goal replaced or contaminated the explicit task")
            require((workspace / "result.txt").read_bytes() == (workspace / "input.txt").read_bytes(),
                    "Reusable definition ran in the wrong workspace")
            reentry = invoke(argv, cwd=work, env=fixture_env)
            require(reentry.stdout == b"" and len(turns) == 2, "Portable pre-check called a model")
        sessions = list((control / "runs").glob("*.jsonl"))
        require(len(sessions) == 1, "Portable execution did not use its explicit evidence directory")
        invoke([bins / "ask", "replay", "-check", sessions[0]], cwd=work, env=agent_env)
    after = {p.relative_to(definition).as_posix(): p.read_bytes() for p in definition.rglob("*") if p.is_file()}
    require(before == after, "Portable runs modified the reusable definition")
    require(not (definition / "work").exists() and not (definition / ".agent").exists(),
            "Portable runner silently converted the definition into a persistent home")
    boundary = "default Cage rejects definition writes" if native_cage else "explicit host actions"
    print(f"ok native Agent portability: one unchanged definition, two workspaces, {boundary}, explicit/private goals, stdin evidence, checked artifacts, separate replay evidence, zero-model re-entry", flush=True)


def check_agent_lifecycle(bins, work, env, agent):
    """Ply remains responsible for cancellation, unfinished work and depth."""
    root = work / "agent lifecycle"
    root.mkdir()
    temporary = root / "temporary"
    temporary.mkdir()
    agent_env = dict(env, TMPDIR=str(temporary),
                     **{"AGENT_" + name.upper(): str(bins / name) for name in ("ask", "brief", "ply", "cage")})
    definitions, workspaces, controls = [], [], []
    for name in ("parent", "child"):
        definition, workspace, control = (root / (name + suffix) for suffix in (" definition", " workspace", " evidence"))
        (definition / "bin").mkdir(parents=True)
        workspace.mkdir()
        (definition / "AGENTS.md").write_text("LIFECYCLE_" + name.upper() + "_IDENTITY\n")
        (definition / "bin/check").write_text("#!/bin/sh\nexit 1\n")
        (definition / "bin/check").chmod(0o700)
        definitions.append(definition); workspaces.append(workspace); controls.append(control)

    def arguments(index):
        return [agent, "run", "-no-cage", "-C", workspaces[index], "-evidence", controls[index]]

    # A textual claim of success cannot override the definition's check.
    with model_fixture(agent_env, lambda _: "The work is done.") as (fixture_env, calls):
        invoke([*arguments(0), "-m", "openai/fixture", "-turns", "1", "-cycles", "1", definitions[0],
                "--", "Produce an accepted result."], cwd=work, env=fixture_env, code=2)
        require(len(calls) > 0, "Unfinished fixture never reached the goal loop")
        count = len(calls)
        invoke([*arguments(0), "-m", "openai/fixture", definitions[0], "--", "Cannot descend."],
               cwd=work, env=dict(fixture_env, PLY_DEPTH="8", PLY_ACTION_SHELL="/bin/sh"), code=1)
        require(len(calls) == count, "Agent reset Ply's existing depth limit")

    # Interrupt both a direct action and an action inside a portable child.
    # No process supervisor is added to Agent: the real Ply owns descendants.
    for nested in (False, True):
        for workspace in workspaces:
            for name in ("action.pid", "parent.pid"):
                (workspace / name).unlink(missing_ok=True)

        def respond(request):
            if nested and "LIFECYCLE_CHILD_IDENTITY" not in request.get("instructions", ""):
                child = shlex.join(map(str, [*arguments(1), definitions[1], "--", "Wait for cancellation."]))
                return "```ply\necho $$ > parent.pid\nprintf 'explicit child input\\n' | " + child + "\n```"
            return "```ply\necho $$ > action.pid\nexec sleep 60\n```"

        with model_fixture(agent_env, respond) as (fixture_env, calls):
            process = subprocess.Popen(list(map(str, [*arguments(0), "-m", "openai/fixture", definitions[0],
                                                       "--", "Wait for cancellation."])),
                                       cwd=work, env=fixture_env, stdin=subprocess.DEVNULL,
                                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
            pids = []
            try:
                target = workspaces[int(nested)] / "action.pid"
                deadline = time.monotonic() + 20
                while not target.exists() and process.poll() is None and time.monotonic() < deadline:
                    time.sleep(0.05)
                require(target.exists(), "Cancellation fixture did not reach its action")
                pids.append(int(target.read_text()))
                if nested:
                    pids.append(int((workspaces[0] / "parent.pid").read_text()))
                    require(len(calls) == 2, "Portable child did not inherit the selected model")
                process.send_signal(signal.SIGTERM)
                stdout, stderr = process.communicate(timeout=10)
                require(process.returncode == 130, f"Agent cancellation lost Ply's status: {process.returncode}: {stderr!r}")
                require(stdout == b"", "Cancellation manufactured a successful answer")
                for pid in pids:
                    try:
                        os.kill(pid, 0)
                    except ProcessLookupError:
                        continue
                    raise RuntimeError(f"Interrupted Agent left action process {pid} running")
                require(not list(temporary.glob("agent-run.*")), "Interrupted Agent left private context on disk")
            finally:
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.communicate(timeout=5)
                for pid in pids:
                    try:
                        os.kill(pid, signal.SIGKILL)
                    except ProcessLookupError:
                        pass
    for control in controls:
        for session in (control / "runs").glob("*.jsonl"):
            invoke([bins / "ask", "replay", "-check", session], cwd=work, env=agent_env)
    print("ok native Agent lifecycle: real unfinished verdict, preserved Ply depth limit, direct/nested cancellation, descendant cleanup and sealed replay", flush=True)


def check_hire(bins, work, env, agent, native_cage=False):
    build_work, run_work = work / "hire build workspace", work / "hired expert workspace"
    build_work.mkdir(); run_work.mkdir()
    hire_env = dict(env, HIRE_AGENT=str(agent),
                    **{"AGENT_" + name.upper(): str(bins / name) for name in ("ask", "brief", "ply", "cage")})
    builder_turns, worker_turns = [], []
    revision_turns = []
    revising = False
    create = '''mkdir -p expert/bin
cat > expert/AGENTS.md <<'EOF'
GENERATED_EXPERT_CONTEXT. Normalize names according to the invocation goal.
EOF
cat > expert/README.md <<'EOF'
Read names.txt; write sorted.txt. A sorted unique result is accepted.
Example: pear, apple, pear becomes apple, pear. A duplicate result is rejected.
The check compares bytes; it does not verify whether a name is factually correct.
EOF
'''
    complete = '''cat > expert/bin/check <<'EOF'
#!/bin/sh
set -eu
test -f names.txt && test -f sorted.txt || exit 1
LC_ALL=C sort -u names.txt | cmp -s - sorted.txt
EOF
chmod 700 expert/bin/check
'''

    def respond(request):
        if "You are choosing which skill" in request.get("instructions", ""):
            return "none"  # This simple sorting fixture does not need a team.
        if revising:
            revision_turns.append(request)
            return "```ply\nprintf '\\nDocumented revision.\\n' >> expert/README.md\n```" if len(revision_turns) == 1 else "Expert documentation revised."
        if "GENERATED_EXPERT_CONTEXT" in request.get("instructions", ""):
            worker_turns.append(request)
            return "```ply\nLC_ALL=C sort -u names.txt > sorted.txt\n```" if len(worker_turns) == 1 else "Names verified."
        builder_turns.append(request)
        return ["```ply\n" + create + "```", "Expert definition verified.",
                "```ply\n" + complete + "```", "Expert definition verified."][min(len(builder_turns)-1, 3)]

    with model_fixture(hire_env, respond) as (fixture_env, calls):
        flags = [] if native_cage else ["-no-cage"]
        built = invoke([bins / "hire", "build", "-C", build_work, "-evidence", work / "hire evidence",
                        "-m", "openai/fixture", *flags, "--", "Build an expert that sorts and deduplicates names."],
                       cwd=work, env=fixture_env, data=b"BUILDER_PRIVATE_BRIEF. Use the supplied Unix tools.\n")
        require(built.stdout == b"Expert definition verified.\n" and len(builder_turns) == 4,
                "Hire did not build and repair through the existing Agent loop")
        definition = build_work / "expert"
        check = definition / "bin/check"
        before = check.read_bytes()
        marker = work / "generated-check-must-not-execute"
        check.write_text("#!/bin/sh\ntouch " + shlex.quote(str(marker)) + "\nexit 0\n")
        invoke([bins / "hire", "verify", definition], cwd=work, env=fixture_env)
        require(not marker.exists(), "Hire verify executed generated code with controller authority")
        check.write_bytes(before)
        # Agent already recursively validates definitions. Hire must use that
        # public behavior rather than implement another tree validator.
        specialist = definition / "agents" / "reviewer"
        (specialist / "bin").mkdir(parents=True)
        (specialist / "AGENTS.md").write_text("Review one explicitly supplied result.\n")
        invoke([bins / "hire", "verify", definition], cwd=work, env=fixture_env, code=1)
        child_check = specialist / "bin/check"
        child_check.write_text("#!/bin/sh\ntouch " + shlex.quote(str(marker)) + "\nexit 0\n")
        child_check.chmod(0o700)
        invoke([bins / "hire", "verify", definition], cwd=work, env=fixture_env)
        require(not marker.exists(), "Hire verify executed a nested specialist's check")
        snapshot = {p.relative_to(definition): p.read_bytes() for p in definition.rglob("*") if p.is_file()}
        (run_work / "names.txt").write_bytes(b"pear\napple\npear\n")
        result = invoke([agent, "run", "-C", run_work, "-evidence", work / "hired expert evidence",
                         "-m", "openai/fixture", *flags, definition, "--", "Write sorted unique names to sorted.txt."],
                        cwd=work, env=fixture_env, data=b"EXPLICIT_WORKER_INPUT\n")
        require(result.stdout == b"Names verified.\n" and (run_work / "sorted.txt").read_bytes() == b"apple\npear\n",
                "The generated expert could not run independently")
        request = json.dumps(worker_turns[0])
        require("BUILDER_PRIVATE_BRIEF" not in request and "You are Hire's expert builder" not in request,
                "The generated expert inherited builder context")
        require("EXPLICIT_WORKER_INPUT" in request, "The generated expert lost its explicit input")
        require(snapshot == {p.relative_to(definition): p.read_bytes() for p in definition.rglob("*") if p.is_file()},
                "Running the generated expert changed its definition")
        revising = True
        revised = invoke([bins / "hire", "build", "-C", build_work, "-evidence", work / "hire evidence",
                          "-m", "openai/fixture", *flags, "--", "Document the requested revision."], cwd=work, env=fixture_env)
        require(revised.stdout == b"Expert documentation revised.\n" and len(revision_turns) == 2
                and "Documented revision." in (definition / "README.md").read_text(),
                "Hire treated an old valid definition as completion of a new build request")
    for evidence in (work / "hire evidence", work / "hired expert evidence"):
        for session in (evidence / "runs").glob("*.jsonl"):
            invoke([bins / "ask", "replay", "-check", session], cwd=work, env=hire_env)
    print("ok Hire -> Agent builds an expert -> separate Agent runs it: repaired definition, real artifact, fresh context, untouched runtime definition, later builder revision, generated checks never executed by builder verification", flush=True)


def check_action_receipts(bins, work, env, evidence):
    archive, session = create_ask_session(bins, work, env, evidence)
    (work / "complete").write_bytes(b"already verified\n")
    write_program(work / "precheck", '''import pathlib, sys
assert not sys.stdin.buffer.read()
sys.exit(0 if pathlib.Path("complete").read_bytes() == b"already verified\\n" else 1)
''')
    precheck = invoke([bins / "ply", "-sh", "-C", work, "-f", session,
                       "-contract-id", "sha256:" + hashlib.sha256(b"integration precheck").hexdigest(),
                       "-check", "./precheck", "Keep the verified result."], cwd=work, env=env)
    require(precheck.stdout == b"", "Ply printed an unrequested answer for a passing precheck")
    precheck_events = [json.loads(line) for line in invoke(
        [bins / "ask", "replay", "-check", "-json", session], cwd=work, env=env).stdout.splitlines()]
    require(sum(e["type"] == "assistant" for e in precheck_events) == 1,
            "Ply's passing precheck called the model")
    require(any(e["type"] == "note" and e["data"].get("kind") == "ply.verifier/v2"
                and e["data"]["body"].get("phase") == "baseline"
                and e["data"]["body"].get("outcome") == "accepted" for e in precheck_events),
            "Built Ply did not seal its passing precheck through real Ask")
    print("ok built Ply -> Ask: passing baseline check, sealed verifier receipt, no model call", flush=True)
    connectors = work / "connectors"
    connectors.mkdir()
    marker = work / "effect.json"
    connector = write_program(connectors / "echo-request", '''import json, os, pathlib, sys
if sys.argv[1] == "describe":
    print(json.dumps({"version":1,"name":"echo-request","description":"Echo exact offline input","input_schema":{"type":"object"}}))
elif sys.argv[1] == "run":
    raw = sys.stdin.buffer.read()
    json.loads(raw)
    pathlib.Path(os.environ["FIXTURE_EFFECT"]).write_bytes(raw)
    sys.stderr.write("connector progress\\n")
    sys.stdout.buffer.write(raw)
else:
    sys.exit(2)
''')
    parked = write_program(work / "parked-may", '''import hashlib, json, os, pathlib, sys
assert sys.argv[1] == "request"
action = sys.stdin.buffer.read()
job = sys.argv[2]
digest = hashlib.sha256(b"may-v1\\0" + job.encode() + b"\\0" + action).hexdigest()
pathlib.Path(os.environ["FIXTURE_MAY"]).write_bytes(action)
print(json.dumps({"version":1,"job":job,"digest":digest,"action":action.decode(),"verdict":"parked"}, separators=(",", ":")))
sys.exit(75)
''')
    policy = write_program(work / "fixture-policy", '''import hashlib, json, sys
raw = sys.stdin.buffer.read()
print(json.dumps({"version":1,"action_sha256":"sha256:"+hashlib.sha256(raw).hexdigest(),"decision":"allow","reason":"operator-owned offline fixture"}, separators=(",", ":")))
''')
    action_env = dict(env, ACTION_PATH=str(connectors), FIXTURE_EFFECT=str(marker), FIXTURE_MAY=str(work / "may-envelope"))
    request = b'{"z":2,"message":"offline"}\n'
    parked_result = invoke([bins / "action", "run", "-job", "fixture-parked", "-may", parked, "echo-request"],
                           cwd=work, env=action_env, data=request, code=75)
    require(parked_result.stdout == b"" and not marker.exists(), "Parked Action released connector input")
    may_envelope = json.loads((work / "may-envelope").read_bytes())
    require(may_envelope["input"] == json.loads(request), "Action did not bind exact requested input to May")
    accepted = invoke([bins / "action", "run", "-job", "fixture-allowed", "-policy", policy,
                       "-record", session, "-ask", bins / "ask", "echo-request"],
                      cwd=work, env=action_env, data=request)
    canonical = b'{"message":"offline","z":2}'
    require(accepted.stdout == marker.read_bytes() == canonical, "Action changed or leaked connector output")
    require(accepted.stderr == b"connector progress\n", "Action changed connector diagnostics")
    verified = invoke([bins / "ask", "replay", "-check", "-json", session], cwd=work, env=env)
    events = [json.loads(line) for line in verified.stdout.splitlines()]
    notes = [e for e in events if e["type"] == "note" and e["data"].get("source") == "action"]
    require([e["data"]["kind"] for e in notes] == ["action.proposal/v1", "action.decision/v1",
            "action.attempt/v1", "action.sent/v1", "action.result/v1"], "Action receipt sequence is incomplete")
    terminal = notes[-1]["data"]["body"]
    require(terminal["sent"] and terminal["exit_code"] == 0, "Action receipt did not bind completed execution")
    require(base64.b64decode(terminal["stdout"]) == canonical
            and terminal["stdout_sha256"] == "sha256:" + hashlib.sha256(canonical).hexdigest(),
            "Action receipt does not prove the observed output bytes")
    before = session.read_bytes()
    trail = invoke([bins / "trail", "check", archive], cwd=work, env=env)
    rows = [json.loads(line) for line in trail.stdout.splitlines()]
    require(len(rows) == 1 and rows[0]["kind"] == "check" and rows[0]["ok"], "Trail did not verify the Action archive")
    require(session.read_bytes() == before, "Trail modified a verified session")
    damaged = work / "damaged"
    damaged.mkdir()
    tampered = damaged / "action.jsonl"
    changed = before.splitlines(keepends=True)
    terminal_event = json.loads(changed[-2])
    require(terminal_event["type"] == "note", "Fixture expected a terminal note followed by its seal")
    terminal_event["data"]["body"]["stdout_bytes"] += 1
    changed[-2] = json.dumps(terminal_event, separators=(",", ":")).encode() + b"\n"
    tampered.write_bytes(b"".join(changed))
    tampered_before = tampered.read_bytes()
    broken = invoke([bins / "trail", "check", damaged], cwd=work, env=env, code=1)
    require(any(row.get("kind") == "check" and row.get("ok") is False
                for row in map(json.loads, broken.stdout.splitlines())), "Trail did not expose Ask's damaged-seal verdict")
    require(tampered.read_bytes() == tampered_before, "Trail repaired damaged evidence")
    print("ok Action -> Ask -> Trail: parked fixture refuses effects; exact output, sealed receipts, read-only replay and tamper rejection", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", required=True, type=Path, help="directory of separately built imported commands")
    parser.add_argument("--agent-only", action="store_true", help="run the real Agent filter contract only")
    parser.add_argument("--agent", type=Path, help="Agent executable to test (default: built agent)")
    parser.add_argument("--portable", action="store_true", help="also require Agent's separate-workspace interface")
    parser.add_argument("--native-cage", action="store_true", help="prove portable Agent's default boundary on a supported native Cage backend")
    args = parser.parse_args()
    bins = args.bin_dir.resolve()
    required = ("hire", "agent", "ask", "brief", "ply", "cage", "trail", "mcp", "mcpbox") if args.agent_only else REQUIRED
    for name in required:
        if name == "agent" and args.agent:
            require(args.agent.is_file() and os.access(args.agent, os.X_OK), f"missing executable: {args.agent}")
            continue
        require((bins / name).is_file() and os.access(bins / name, os.X_OK), f"missing executable: {bins / name}")
    with tempfile.TemporaryDirectory(prefix="bench-integration-") as tmp:
        work = Path(tmp).resolve()
        home = work / "home"
        home.mkdir()
        # Preserve only build/runtime plumbing. Never inherit model credentials,
        # tool policy, session selectors, or an operator's writable state.
        keep = ("PATH", "TMPDIR", "GOCACHE", "GOMODCACHE", "GOPATH", "CGO_ENABLED",
                "CC", "CXX", "SDKROOT", "DEVELOPER_DIR", "SYSTEMROOT")
        env = {key: os.environ[key] for key in keep if key in os.environ}
        for name in ("GOCACHE", "GOMODCACHE", "GOPATH"):
            if name not in env:
                env[name] = subprocess.check_output(["go", "env", name], text=True).strip()
        env.update(HOME=str(home), PATH=str(bins) + os.pathsep + env.get("PATH", os.defpath),
                   GOWORK="off", GOPROXY="off", GOTOOLCHAIN="local", ASK_LIVE="",
                   XDG_CONFIG_HOME=str(home / ".config"), XDG_STATE_HOME=str(home / ".local/state"),
                   XDG_CACHE_HOME=str(home / ".cache"), PYTHONDONTWRITEBYTECODE="1")
        for name in REQUIRED:
            env[name.upper()] = str(bins / name)
        invoke(["sh", ROOT / "scripts/agent-hire_test.sh"], cwd=work, env=dict(env,
               AGENT_TEST_EXECUTABLE=str((args.agent or bins / "agent").resolve()),
               HIRE_TEST_EXECUTABLE=str((bins / "hire").resolve())))
        print("ok separate Agent/Hire executables: existing 163 runtime and home-maintenance checks", flush=True)
        if args.agent_only:
            check_agent(bins, work, env, (args.agent or bins / "agent").resolve())
            if args.portable:
                check_portable_agent(bins, work, env, (args.agent or bins / "agent").resolve(), args.native_cage)
                check_agent_lifecycle(bins, work, env, (args.agent or bins / "agent").resolve())
                check_hire(bins, work, env, (args.agent or bins / "agent").resolve(), args.native_cage)
            return
        check_agent(bins, work, env, (args.agent or bins / "agent").resolve())
        check_portable_agent(bins, work, env, (args.agent or bins / "agent").resolve(), args.native_cage)
        check_agent_lifecycle(bins, work, env, (args.agent or bins / "agent").resolve())
        check_hire(bins, work, env, (args.agent or bins / "agent").resolve(), args.native_cage)
        a2a = invoke([sys.executable, ROOT / "scripts/check-a2a.py", "--bin-dir", bins], cwd=work, env=env)
        sys.stdout.buffer.write(a2a.stdout)
        sys.stderr.buffer.write(a2a.stderr)
        harnesses = invoke([sys.executable, ROOT / "scripts/check-harnesses.py", "--bin-dir", bins], cwd=work, env=env)
        sys.stdout.buffer.write(harnesses.stdout)
        sys.stderr.buffer.write(harnesses.stderr)
        env["PLY_TEST_ASK_SOURCE"] = str(ROOT / "tools/ask")
        go_contract("ply", "TestAskPlyContract", env)
        go_contract("hone", "TestScaffoldWritesFrontmatterBriefCanRead", env)
        evidence = check_context_cite(bins, work, env)
        check_action_receipts(bins, work, env, evidence)
        print("running Weave's offline executable/example suite", flush=True)
        weave = work / "weave-source"
        shutil.copytree(ROOT / "tools/weave", weave, ignore=shutil.ignore_patterns(".git", "__pycache__", "*.pyc"))
        weave_env = dict(env, WEAVE_TEST_BIN=str(bins))
        # These are tests/check's full smoke/example steps. Go/race/vet belong
        # to the independent runner; do not rebuild or replace its binaries.
        for command in ([sys.executable, "tests/smoke", bins / "weave"],
                        [sys.executable, "-m", "unittest", "discover", "-s", "tests", "-v"]):
            result = invoke(command, cwd=weave, env=weave_env, timeout=600)
            sys.stdout.buffer.write(result.stdout)
            sys.stderr.buffer.write(result.stderr)
        print("ok Weave -> Tend/Ask/Ply offline examples", flush=True)
    print("All offline executable integration checks passed.", flush=True)


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, subprocess.SubprocessError, OSError, ValueError, KeyError) as error:
        print(f"integration: {error}", file=sys.stderr)
        sys.exit(1)
