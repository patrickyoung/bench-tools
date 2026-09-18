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
PLUGIN_FILES = (".claude-plugin/plugin.json", ".codex-plugin/plugin.json")
PACKAGE_FILES = (PLUGIN_FILES[0], "LICENSE")
MARKETPLACE_FILES = (".claude-plugin/marketplace.json", ".agents/plugins/marketplace.json", "package.json")


def require(ok, message):
    if not ok:
        raise RuntimeError(message)


def skill_files(root):
    """Read the complete source skill, refusing links to unpackaged host files."""
    directory = root / ".agents/skills"
    require(directory.is_dir() and not directory.is_symlink() and
            directory.resolve().is_relative_to(root.resolve()), "missing or linked skill directory")
    files = {}
    for path in sorted(directory.rglob("*")):
        require(not path.is_symlink(), f"skill package contains a symbolic link: {path}")
        if path.is_file():
            files[path.relative_to(directory)] = path.read_bytes()
    require(Path("bench/SKILL.md") in files, "canonical Bench skill missing")
    return files


def package_metadata(root):
    """Catch Git marketplace routes that do not load the shared, relocatable skill."""
    manifests = [json.loads((root / name).read_text()) for name in PLUGIN_FILES]
    pi = json.loads((root / "package.json").read_text())
    require(len({m["version"] for m in [*manifests, pi]}) == 1,
            "Claude, Codex and Pi skill package versions disagree")
    for manifest in manifests:
        require(manifest["name"] == "bench-tools", "unexpected plugin name")
        require(not Path(manifest["skills"]).is_absolute() and ".." not in Path(manifest["skills"]).parts,
                "plugin skill path must be relative and relocatable")
        require((root / manifest["skills"]).resolve() == (root / ".agents/skills").resolve(),
                "plugin does not use the canonical skills directory")
        require(not any(name in manifest for name in ("hooks", "mcpServers", "apps")),
                "Bench knowledge installation must not register runtime hooks or services")
    require(pi["pi"]["skills"] == ["./.agents/skills"], "Pi does not use the canonical skills")
    for name in MARKETPLACE_FILES[:2]:
        marketplace = json.loads((root / name).read_text())
        require(marketplace["name"] == "bench-tools", "unexpected marketplace name")
        plugins = [p for p in marketplace["plugins"] if p["name"] == "bench-tools"]
        require(len(plugins) == 1, "marketplace must contain exactly one Bench entry")
        source = plugins[0]["source"]
        if isinstance(source, dict):
            require(source["source"] == "local", "Codex marketplace must use its bundled plugin")
            source = source["path"]
        require(source == "./", "marketplace must resolve the repository root plugin")
    skill_files(root)
    return manifests[0]["version"]


def portable_package(root, archive, destination):
    """Build the documented upload ZIP and verify every relocated knowledge byte."""
    expected = skill_files(root)
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as target:
        for name in PACKAGE_FILES:
            target.write(root / name, name)
        for relative in expected:
            path = Path(".agents/skills") / relative
            target.write(root / path, path)
    with zipfile.ZipFile(archive) as source:
        source.extractall(destination)
    require(skill_files(destination) == expected, "relocated plugin lost or changed supporting skill files")
    return destination


def marketplace_fixture(work, env, package):
    """Exercise Git URL loading without publishing source or contacting a service."""
    source = work / "marketplace source"
    shutil.copytree(package, source)
    for name in (*MARKETPLACE_FILES, PLUGIN_FILES[1]):
        path = source / name
        path.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(ROOT / name, path)
    run(["git", "init", "-q"], source, env)
    run(["git", "add", "."], source, env)
    run(["git", "-c", "user.name=Bench fixture", "-c", "user.email=fixture@example.invalid",
         "-c", "commit.gpgsign=false", "commit", "-qm", "Current Bench package fixture"], source, env)
    url = "https://bench-install.invalid/bench-tools.git"
    git_env = dict(env, GIT_CONFIG_COUNT="2", GIT_CONFIG_KEY_0="url." + source.as_uri() + ".insteadOf",
                   GIT_CONFIG_VALUE_0=url, GIT_CONFIG_KEY_1="protocol.file.allow", GIT_CONFIG_VALUE_1="always")
    return url, git_env


def run(argv, cwd, env, data=None, code=0, timeout=45):
    command, recording = replay_support["recorded_argv"](argv, env)
    result = subprocess.run(command, cwd=cwd, env=env, input=data,
                            capture_output=True, timeout=timeout)
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


def native_hosts(work, env, bins, package, source_url=None, source_ref=None):
    programs = {name: shutil.which(name) for name in ("codex", "claude", "pi")}
    require(all(programs.values()), "--host-clis requires installed Codex, Claude Code, and Pi")
    for name, executable in programs.items():
        print(run([executable, "--version"], work, env).decode().strip(), flush=True)

    if source_url:
        url = source_url
        print("Checking published marketplace: " + url + (" at " + source_ref if source_ref else ""), flush=True)
    else:
        url, env = marketplace_fixture(work, env, package)

    # A package contains only its declared skill and manifest, so project
    # CLAUDE.md instructions cannot accidentally become plugin instructions.
    validated = json.loads(run([programs["claude"], "plugin", "validate", package, "--strict", "--json"], work, env))
    require(validated["success"], "Claude rejected the portable plugin")
    run([programs["claude"], "plugin", "marketplace", "add", url + ("#" + source_ref if source_ref else "")],
        work, env, timeout=180)
    run([programs["claude"], "plugin", "install", "bench-tools@bench-tools"], work, env)
    installed = json.loads(run([programs["claude"], "plugin", "list", "--json"], work, env))
    plugin = next((p for p in installed if p["id"] == "bench-tools@bench-tools"), None)
    require(plugin is not None and plugin["enabled"], "Claude marketplace plugin was not enabled")
    require(skill_files(Path(plugin["installPath"])) == skill_files(package),
            "Claude marketplace installation changed the shared skill")
    # The SDK's control initialization lists commands without a user/model turn.
    initialized = run([programs["claude"], "-p", "--input-format", "stream-json",
                       "--output-format", "stream-json", "--verbose", "--no-session-persistence",
                       "--setting-sources", "user", "--strict-mcp-config"],
                      work, env, json.dumps({"type": "control_request", "request_id": "bench-discovery",
                                             "request": {"subtype": "initialize"}}).encode() + b"\n")
    responses = [json.loads(line).get("response", {}) for line in initialized.splitlines()]
    discovered = [command["name"] for response in responses
                  if response.get("request_id") == "bench-discovery" and response.get("subtype") == "success"
                  for command in response.get("response", {}).get("commands", [])
                  if command["name"].startswith("bench-tools:")]
    require(discovered == ["bench-tools:bench"],
            "Claude must discover exactly the packaged Bench skill: " + str(discovered))

    server = [bins / "mcpserve", "-allow-legacy",
              ROOT / "tools/mcp/examples/filter-server/manifest.json", "--",
              ROOT / "tools/mcp/examples/filter-server/dispatch"]
    run([programs["claude"], "mcp", "add", "--scope", "local", "bench-fixture", "--", *server], work, env)
    connection = run([programs["claude"], "mcp", "get", "bench-fixture"], work, env).decode()
    require("Connected" in connection, "Claude did not connect to MCPserve: " + connection)
    print("ok Claude: Git marketplace install preserves skill; fresh session discovers installed plugin; MCP connects", flush=True)

    run([programs["codex"], "plugin", "marketplace", "add", url, "--json",
         *(["--ref", source_ref] if source_ref else [])], work, env, timeout=180)
    installed = json.loads(run([programs["codex"], "plugin", "add", "bench-tools@bench-tools", "--json"], work, env))
    codex_package = Path(installed["installedPath"])
    require(skill_files(codex_package) == skill_files(package), "Codex marketplace installation changed the shared skill")
    listed = json.loads(run([programs["codex"], "plugin", "list", "--marketplace", "bench-tools", "--json"], work, env))
    require(any(p["pluginId"] == "bench-tools@bench-tools" and p["enabled"] for p in listed["installed"]),
            "Codex marketplace plugin was not enabled")
    run([programs["codex"], "mcp", "add", "bench-fixture", "--", *server], work, env)
    with peer([programs["codex"], "app-server"], work, env) as call:
        call({"id": 1, "method": "initialize", "params": {"clientInfo": {"name": "bench-fixture", "version": "1"}}})
        call({"method": "initialized"})
        listed = call({"id": 2, "method": "skills/list", "params": {"cwds": [str(work)], "forceReload": True}})
        skills = [skill for item in listed["result"]["data"] for skill in item["skills"]]
        plugin_skills = [skill for skill in skills if skill.get("pluginId") == "bench-tools@bench-tools"]
        require(len(plugin_skills) == 1 and any(skill["name"] == "bench-tools:bench"
                    and skill.get("enabled") and Path(skill["path"]).resolve() ==
                    (codex_package / ".agents/skills/bench/SKILL.md").resolve()
                    for skill in plugin_skills), "Codex must discover exactly the installed Bench skill: " + str(plugin_skills))
        status = call({"id": 3, "method": "mcpServerStatus/list", "params": {}})
        servers = status["result"]["data"]
        require(any(item["name"] == "bench-fixture" and item.get("tools") for item in servers),
                "Codex did not discover the MCP tool: " + str(status))
    print("ok Codex: Git marketplace install preserves skill; fresh app-server discovers installed plugin and MCP tool", flush=True)

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
    parser.add_argument("--source-url", help="explicit network check: install this published Git marketplace instead of the local fixture")
    parser.add_argument("--ref", help="Git branch or tag to use with --source-url")
    args = parser.parse_args()
    if args.source_url and not args.host_clis:
        parser.error("--source-url requires --host-clis")
    if args.ref and not args.source_url:
        parser.error("--ref requires --source-url")
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
        version = package_metadata(ROOT)
        package = portable_package(ROOT, work / "bench-tools.zip", work / "extracted package")
        manifest = json.loads((package / ".claude-plugin/plugin.json").read_text())
        skills = (package / manifest["skills"]).resolve()
        require(skills.is_relative_to(package), "plugin skills escaped package")
        run([bins / "brief", "lint", "-strict", skills / "bench"], work, env)
        print(f"ok skill {version}: both marketplaces resolve the shared skill; strict lint and relocated ZIP preserve all references", flush=True)

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
            native_hosts(work, env, bins, package, args.source_url, args.ref)
    print("harness checks passed; no hosted model calls")


if __name__ == "__main__":
    main()
