#!/usr/bin/env python3
"""Check portable skill packaging and MCP host setup without hosted model calls."""
import argparse
from contextlib import contextmanager
import json
import os
from pathlib import Path
import queue
import runpy
import shutil
import signal
import subprocess
import tempfile
import threading
import time
import zipfile

ROOT = Path(__file__).resolve().parents[1]
replay_support = runpy.run_path(str(ROOT / "scripts/check-integration.py"))


def require(ok, message):
    if not ok:
        raise RuntimeError(message)


def run(argv, cwd, env, data=None, code=0):
    command, recording = replay_support["recorded_argv"](argv, env)
    result = subprocess.run(command, cwd=cwd, env=env, input=data,
                            capture_output=True, timeout=45)
    replay_support["verify_recorded"](recording, result, env)
    require(result.returncode == code,
            f"{argv}: exit {result.returncode}, wanted {code}\n{result.stdout.decode()}\n{result.stderr.decode()}")
    return result.stdout


@contextmanager
def peer(argv, cwd, env):
    with tempfile.TemporaryFile() as errors:
        process = subprocess.Popen(list(map(str, argv)), cwd=cwd, env=env, stdin=subprocess.PIPE,
                                   stdout=subprocess.PIPE, stderr=errors, start_new_session=True)
        messages = queue.Queue()

        def read():
            for line in process.stdout:
                messages.put(line)
            messages.put(None)

        reader = threading.Thread(target=read, daemon=True)
        reader.start()

        def call(request):
            process.stdin.write(json.dumps(request).encode() + b"\n")
            process.stdin.flush()
            if "id" not in request:
                return None
            deadline = time.monotonic() + 30
            while time.monotonic() < deadline:
                try:
                    line = messages.get(timeout=max(.01, deadline - time.monotonic()))
                except queue.Empty:
                    break
                if line is None:
                    break
                response = json.loads(line)
                if response.get("id") == request.get("id"):
                    require("error" not in response and response.get("success", True), str(response))
                    return response
            errors.seek(0)
            raise RuntimeError("Host did not answer " + str(request) + "\n" + errors.read().decode()[-3000:])

        try:
            yield call
        finally:
            if process.poll() is None:
                process.terminate()
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.wait(timeout=5)
            process.stdin.close()
            reader.join(timeout=2)
            process.stdout.close()


def native_hosts(work, env, bins, package):
    programs = {name: shutil.which(name) for name in ("codex", "claude", "pi")}
    require(all(programs.values()), "--host-clis requires installed Codex, Claude Code, and Pi")
    for name, executable in programs.items():
        print(run([executable, "--version"], work, env).decode().strip(), flush=True)

    # A package contains only its declared skill and manifest, so project
    # CLAUDE.md instructions cannot accidentally become plugin instructions.
    validated = json.loads(run([programs["claude"], "plugin", "validate", package, "--strict", "--json"], work, env))
    require(validated["success"], "Claude rejected the portable plugin")
    # The SDK's control initialization lists commands without a user/model turn.
    initialized = run([programs["claude"], "-p", "--input-format", "stream-json",
                       "--output-format", "stream-json", "--verbose", "--no-session-persistence",
                       "--setting-sources", "", "--strict-mcp-config", "--plugin-dir", package],
                      work, env, json.dumps({"type": "control_request", "request_id": "bench-discovery",
                                             "request": {"subtype": "initialize"}}).encode() + b"\n")
    responses = [json.loads(line).get("response", {}) for line in initialized.splitlines()]
    require(any(response.get("request_id") == "bench-discovery" and response.get("subtype") == "success"
                and any(command["name"] == "bench-tools:bench"
                        for command in response.get("response", {}).get("commands", []))
                for response in responses), "Claude did not discover the packaged Bench skill")

    server = [bins / "mcpserve", "-allow-legacy",
              ROOT / "tools/mcp/examples/filter-server/manifest.json", "--",
              ROOT / "tools/mcp/examples/filter-server/dispatch"]
    run([programs["claude"], "mcp", "add", "--scope", "local", "bench-fixture", "--", *server], work, env)
    connection = run([programs["claude"], "mcp", "get", "bench-fixture"], work, env).decode()
    require("Connected" in connection, "Claude did not connect to MCPserve: " + connection)
    print("ok Claude: extracted plugin validates; fresh session discovers skill; native MCP registration connects", flush=True)

    personal = Path(env["HOME"]) / ".agents/skills/bench"
    personal.parent.mkdir(parents=True, exist_ok=True)
    shutil.copytree(ROOT / ".agents/skills/bench", personal)
    run([programs["codex"], "mcp", "add", "bench-fixture", "--", *server], work, env)
    with peer([programs["codex"], "app-server"], work, env) as call:
        call({"id": 1, "method": "initialize", "params": {"clientInfo": {"name": "bench-fixture", "version": "1"}}})
        call({"method": "initialized"})
        listed = call({"id": 2, "method": "skills/list", "params": {"cwds": [str(work)], "forceReload": True}})
        skills = [skill for item in listed["result"]["data"] for skill in item["skills"]]
        require(any(skill["name"] == "bench" and Path(skill["path"]).resolve() == (personal / "SKILL.md").resolve()
                    for skill in skills), "Codex did not discover the installed Bench skill")
        status = call({"id": 3, "method": "mcpServerStatus/list", "params": {}})
        servers = status["result"]["data"]
        require(any(item["name"] == "bench-fixture" and item.get("tools") for item in servers),
                "Codex did not discover the MCP tool: " + str(status))
    print("ok Codex: fresh app-server discovers copied skill and registered MCP tool", flush=True)

    shutil.rmtree(personal)  # Pi must discover the package, not Codex's copied skill.
    run([programs["pi"], "install", ROOT], work, env)
    with peer([programs["pi"], "--offline", "--mode", "rpc", "--no-session", "--no-extensions",
               "--no-prompt-templates", "--no-themes", "--no-context-files"], work, env) as call:
        response = call({"id": "commands", "type": "get_commands"})
        require(any(command["name"] == "skill:bench" for command in response["data"]["commands"]),
                "Pi did not discover Bench: " + str(response))
    print("ok Pi: local package installs and a fresh RPC session discovers Bench", flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, default=ROOT / ".build/bin")
    parser.add_argument("--host-clis", action="store_true", help="also verify installed host CLIs in isolated homes")
    args = parser.parse_args()
    bins = args.bin_dir.resolve()
    with tempfile.TemporaryDirectory(prefix="bench-harness-") as temp:
        work = Path(temp).resolve()
        home = work / "home"
        home.mkdir()
        for directory in (".codex", ".claude", ".pi/agent"):
            (home / directory).mkdir(parents=True)
        env = {"HOME": str(home), "PATH": str(bins) + os.pathsep + os.environ.get("PATH", os.defpath),
               "TMPDIR": str(work), "LANG": "C", "CODEX_HOME": str(home / ".codex"),
               "CLAUDE_CONFIG_DIR": str(home / ".claude"), "PI_CODING_AGENT_DIR": str(home / ".pi/agent"),
               "PI_OFFLINE": "1", "PI_TELEMETRY": "0", "DISABLE_TELEMETRY": "1",
               "DISABLE_AUTOUPDATER": "1", "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"}
        env.update({k:os.environ[k] for k in ("BENCH_REPLAY_RECORD_DIR","BENCH_REPLAY_BIN_DIR") if k in os.environ})
        run([bins / "brief", "lint", "-strict", ROOT / ".agents/skills/bench"], work, env)
        archive = work / "bench-tools.zip"
        files = [ROOT / ".claude-plugin/plugin.json", ROOT / "LICENSE"]
        files += sorted((ROOT / ".agents/skills").rglob("*"))
        with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as target:
            for path in files:
                if path.is_file():
                    target.write(path, path.relative_to(ROOT))
        package = work / "extracted"
        with zipfile.ZipFile(archive) as source:
            source.extractall(package)
        manifest = json.loads((package / ".claude-plugin/plugin.json").read_text())
        skills = (package / manifest["skills"]).resolve()
        require(skills.is_relative_to(package), "plugin skills escaped package")
        run([bins / "brief", "lint", "-strict", skills / "bench"], work, env)
        require((skills / "bench/SKILL.md").read_bytes() == (ROOT / ".agents/skills/bench/SKILL.md").read_bytes(),
                "packaging changed the canonical skill")
        expected = {p.relative_to(ROOT / ".agents/skills"): p.read_bytes()
                    for p in (ROOT / ".agents/skills").rglob("*") if p.is_file()}
        actual = {p.relative_to(skills): p.read_bytes() for p in skills.rglob("*") if p.is_file()}
        require(actual == expected, "relocated plugin lost or changed supporting skill files")
        print("ok skill: standalone strict lint and relocated plugin ZIP with complete references", flush=True)

        server = [bins / "mcpserve", ROOT / "tools/mcp/examples/filter-server/manifest.json", "--",
                  ROOT / "tools/mcp/examples/filter-server/dispatch"]
        for client, selected in (("mcp", server), ("mcp", [server[0], "-allow-legacy", *server[1:]]),
                                 ("mcp-legacy", [server[0], "-allow-legacy", *server[1:]])):
            listing = json.loads(run([bins / client, "request", "tools/list", "--", *selected], work, env, b"{}"))
            require(any(tool["name"] == "hello" for tool in listing["tools"]), "MCP descriptor missing")
            result = json.loads(run([bins / client, "request", "tools/call", "--", *selected], work, env,
                                    b'{"name":"hello","arguments":{"name":"Unix"}}'))
            require(result["structuredContent"]["greeting"] == "hello from a Unix filter", "MCP call lost result")
        run([bins / "mcp-legacy", "request", "tools/list", "--", *server], work, env, b"{}", code=2)
        print("ok MCP: real stdio discovery/calls; explicit legacy compatibility; default refusal", flush=True)
        if args.host_clis:
            native_hosts(work, env, bins, package)
    print("harness checks passed; no hosted model calls")


if __name__ == "__main__":
    main()
