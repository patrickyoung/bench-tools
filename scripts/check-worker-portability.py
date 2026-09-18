#!/usr/bin/env python3
"""Exercise real committed worker/team projections without any model calls.

Checks source bytes, file modes, locks, skill lint, complete-package relocation,
and a real response check. Optional host discovery uses isolated homes and
initialization/listing interfaces only; it does not establish model job quality.
"""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import runpy
import shutil
import stat
import sys
import tarfile
import tempfile


ROOT = Path(__file__).resolve().parents[1]
SUPPORT = runpy.run_path(str(ROOT / "scripts/check-harnesses.py"))
run, peer, require = (SUPPORT[name] for name in ("run", "peer", "require"))
TARGETS = ("skill", "codex", "claude-code", "cowork", "pi", "openclaw", "hermes")
WORKERS = ("product-owner", "page-planner", "visual-artist")
TEAM = "page-team"
GENERATED = {"SKILL.md", "PORTABILITY.md", "portability.json"}
SOURCE_ROOT = "references/bench"


def snapshot(directory):
    result = {}
    for path in directory.rglob("*"):
        require(not path.is_symlink(), "export contains symlink: " + str(path))
        if path.is_file():
            result[path.relative_to(directory).as_posix()] = (
                path.read_bytes(), stat.S_IMODE(path.stat().st_mode))
    return result


def isolated_environment(work, bins):
    home = work / "home"
    for directory in (".codex", ".claude", ".pi/agent", ".config", ".cache", ".local/share"):
        (home / directory).mkdir(parents=True, exist_ok=True)
    env = {"HOME": str(home), "PATH": str(bins) + os.pathsep + os.environ.get("PATH", os.defpath),
           "TMPDIR": str(work), "LANG": "C", "CODEX_HOME": str(home / ".codex"),
           "CLAUDE_CONFIG_DIR": str(home / ".claude"), "PI_CODING_AGENT_DIR": str(home / ".pi/agent"),
           "XDG_CONFIG_HOME": str(home / ".config"), "XDG_CACHE_HOME": str(home / ".cache"),
           "XDG_DATA_HOME": str(home / ".local/share"), "PI_OFFLINE": "1", "PI_TELEMETRY": "0",
           "DISABLE_TELEMETRY": "1", "DISABLE_AUTOUPDATER": "1",
           "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
           "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1"}
    env.update({key: os.environ[key] for key in ("BENCH_REPLAY_RECORD_DIR", "BENCH_REPLAY_BIN_DIR")
                if key in os.environ})
    return env


def committed_source(commit, work, env):
    tree = run(["git", "-C", ROOT, "ls-tree", "-r", "-z", commit, "--", "workers", "teams"], work, env)
    modes = {}
    for row in tree.split(b"\0"):
        if row:
            details, name = row.split(b"\t", 1)
            modes[name.decode()] = int(details.split()[0], 8) & 0o777
    archive = run(["git", "-C", ROOT, "archive", "--format=tar", commit, "workers", "teams"], work, env)
    with tarfile.open(fileobj=io.BytesIO(archive)) as source:
        return {item.name: (source.extractfile(item).read(), modes[item.name])
                for item in source if item.isfile()}


def expected_source(kind, name, source):
    """Build the expected source mapping from committed catalog metadata."""
    prefix = f"{kind}s/{name}/expert/"
    metadata = json.loads(source[f"{kind}s/{name}/{kind}.json"][0])
    expected = {"expert/" + path[len(prefix):]: source[path] for path in metadata["files"]}
    for role, member in metadata.get("members", {}).items():
        member_files, _ = expected_source("worker", member["worker"], source)
        expected.update({"expert/agents/" + role + "/" + path[len("expert/"):]: value
                         for path, value in member_files.items()})
        if "adapter" in member:
            expected["expert/bin/workers/" + role] = source[prefix + member["adapter"]]
    return expected, metadata


def verify_plain(package, kind, name, source, commit):
    expected, metadata = expected_source(kind, name, source)
    actual = snapshot(package)
    lock_name = kind + ".lock.json"
    require(set(actual) == set(expected) | {lock_name}, "plain export inventory differs from committed catalog")
    require({path: actual[path] for path in expected} == expected,
            "plain export changed committed source bytes or modes: " + name)
    lock = json.loads(actual[lock_name][0])
    require(lock["source"]["commit"] == commit and lock[kind] == name and lock["adaptations"] == [],
            "plain source lock does not identify unchanged pinned source")
    require(lock["requires"] == metadata["requires"], "source requirements changed")
    require(set(lock["files"]) == set(expected), "source lock omitted files")
    for path, (data, mode) in expected.items():
        require(lock["files"][path] == {"sha256": hashlib.sha256(data).hexdigest(),
                                       "mode": "100755" if mode == 0o755 else "100644"},
                "source lock hash or mode mismatch: " + path)
    if kind == "team":
        require(set(lock["members"]) == set(metadata["members"]), "team lock lost roster members")
        for role, member in metadata["members"].items():
            require(lock["members"][role]["worker"] == member["worker"], "team lock changed member identity")
    return actual


def export(kind, name, destination, commit, work, env, target=None, execution=None):
    argv = [sys.executable, ROOT / "scripts/workers", "export-team" if kind == "team" else "export",
            name, destination, "--ref", commit, "--allow-experimental"]
    if target:
        argv += ["--target", target, "--execution", execution]
    result = json.loads(run(argv, work, env))
    require(result["commit"] == commit and Path(result["directory"]) == destination,
            "export reported a different pin or destination")


def verify_projection(package, plain, kind, name, target, execution, bins, work, env):
    actual = snapshot(package)
    mounted = {SOURCE_ROOT + "/" + path: value for path, value in plain.items()}
    require(set(actual) == set(mounted) | GENERATED, "projection lost source or added unexpected files")
    require({path: actual[path] for path in mounted} == mounted,
            "projection changed original source, modes, or lock: " + package.name)
    manifest = json.loads(actual["portability.json"][0])
    expected_name = f"bench-{kind}-{name}-{execution}"
    require(manifest["schema"] == "bench.portability/v1" and manifest["name"] == expected_name
            and manifest["target"] == target and manifest["execution"] == execution
            and manifest["kind"] == kind and manifest["source_root"] == SOURCE_ROOT
            and manifest["source_lock"] == SOURCE_ROOT + "/" + kind + ".lock.json",
            "projection identity mismatch")
    require(manifest["requirements"] == json.loads(plain[kind + ".lock.json"][0])["requires"]
            and manifest["limitations"], "projection lost requirements or fidelity limits")
    require(set(manifest["files"]) == {"SKILL.md", "PORTABILITY.md"}, "projection manifest inventory mismatch")
    for path, record in manifest["files"].items():
        require(record == {"sha256": hashlib.sha256(actual[path][0]).hexdigest(), "mode": "100644"}
                and actual[path][1] == 0o644, "projection document hash or mode mismatch")
    skill = actual["SKILL.md"][0].decode()
    require("\nname: " + expected_name + "\n" in skill, "generated skill name differs from manifest")
    require(str(work) not in skill, "generated instructions embed a temporary absolute path")
    # Original nested definitions intentionally retain their layout. Brief's
    # advice about deep links is not a license to rewrite that original source.
    run([bins / "brief", "lint", package], work, env)
    return expected_name


def check_relocated_planner(package, work, env):
    current = work / "current planner inputs"
    state = work / "planner state"
    current.mkdir()
    state.mkdir()
    snapshot_value = {"schema": "bench.manage.snapshot/v1", "workers": ["frontend"], "criteria": ["page"]}
    digest = "sha256:" + hashlib.sha256(json.dumps(snapshot_value, sort_keys=True).encode()).hexdigest()
    packet = {"snapshot_sha256": digest, "snapshot": snapshot_value}
    (current / "snapshot.json").write_text(json.dumps(packet) + "\n")
    candidate = {"snapshot_sha256": digest, "action": "blocked", "reason": "No admitted work in this fixture",
                 "tasks": [], "supersede": [], "result": ""}
    exact = json.dumps(candidate).encode() + b"\n"
    expert = package / SOURCE_ROOT / "expert"
    check_env = dict(env, AGENT_HOME=str(expert), AGENT_WORK=str(current), AGENT_STATE=str(state),
                     BRIEF_PATH=str(expert / "skills"))
    checker = expert / "bin/check"
    original = snapshot(package)
    accepted = run([checker], current, check_env, exact)
    require(b"snapshot-bound" in accepted, "planner checker did not accept the exact response")
    candidate["snapshot_sha256"] = "sha256:" + "0" * 64
    rejected = run([checker], current, check_env, json.dumps(candidate).encode(), code=1)
    require(b"stale snapshot digest" in rejected, "wrong candidate did not fail for snapshot binding")
    missing = run([checker], current, check_env, b"", code=1)
    require(b"missing/invalid" in missing, "missing candidate did not fail parsing")
    require(snapshot(package) == original, "checker mutated its installed definition")
    require({p.name for p in current.iterdir()} == {"snapshot.json"}, "planner checker invented an output artifact")
    print("ok relocated page-planner: exact candidate stdin accepted; stale hash and empty stdin rejected", flush=True)


def native_hosts(work, bins, packages):
    programs = {name: shutil.which(name) for name in ("codex", "claude", "pi")}
    require(all(programs.values()), "--host-clis requires installed Codex, Claude Code, and Pi")
    expected = {f"bench-worker-{name}-{execution}" for name in WORKERS for execution in ("native", "bench")}
    expected.add("bench-team-" + TEAM + "-bench")
    require(all(set(packages[target]) == expected for target in ("codex", "claude-code", "pi")),
            "host check lacks an expected generated worker/team skill")
    for host in programs:
        host_work = work / ("host-" + host)
        host_work.mkdir()
        env = isolated_environment(host_work, bins)
        print(run([programs[host], "--version"], host_work, env).decode().strip(), flush=True)
        if host == "codex":
            installed = Path(env["HOME"]) / ".agents/skills"
            for name, package in packages["codex"].items():
                shutil.copytree(package, installed / name)
            with peer([programs[host], "app-server"], host_work, env) as call:
                call({"id": 1, "method": "initialize", "params": {"clientInfo": {"name": "bench-portability-check", "version": "1"}}})
                call({"method": "initialized"})
                response = call({"id": 2, "method": "skills/list", "params": {"cwds": [str(host_work)], "forceReload": True}})
                skills = [skill for group in response["result"]["data"] for skill in group["skills"]]
                packaged = [skill for skill in skills if Path(skill["path"]).resolve().is_relative_to(installed.resolve())]
                wrappers = [skill for skill in packaged if skill["name"] in expected
                            and Path(skill["path"]).resolve() == (installed / skill["name"] / "SKILL.md").resolve()]
                found = {skill["name"] for skill in wrappers}
                extras = [{"name": skill["name"], "path": str(Path(skill["path"]).resolve().relative_to(installed.resolve()))}
                          for skill in packaged if skill not in wrappers]
        elif host == "claude":
            plugin = host_work / "plugin"
            (plugin / ".claude-plugin").mkdir(parents=True)
            plugin_name = "bench-portability-check"
            (plugin / ".claude-plugin/plugin.json").write_text(json.dumps({
                "name": plugin_name, "version": "1.0.0", "description": "Offline generated worker discovery check.",
                "author": {"name": "Bench portability checks"},
                "skills": "./skills/"}) + "\n")
            for name, package in packages["claude-code"].items():
                shutil.copytree(package, plugin / "skills" / name)
            validated = json.loads(run([programs[host], "plugin", "validate", plugin, "--strict", "--json"], host_work, env))
            require(validated["success"], "Claude rejected the generated-worker plugin")
            initialized = run([programs[host], "-p", "--input-format", "stream-json", "--output-format", "stream-json",
                               "--verbose", "--no-session-persistence", "--setting-sources", "", "--strict-mcp-config",
                               "--plugin-dir", plugin], host_work, env,
                              json.dumps({"type": "control_request", "request_id": "worker-discovery",
                                          "request": {"subtype": "initialize"}}).encode() + b"\n")
            responses = [json.loads(line).get("response", {}) for line in initialized.splitlines()]
            discovered_names = [command["name"].removeprefix(plugin_name + ":")
                     for response in responses if response.get("request_id") == "worker-discovery"
                     and response.get("subtype") == "success"
                     for command in response.get("response", {}).get("commands", [])
                     if command["name"].startswith(plugin_name + ":")]
            found = set(discovered_names)
            extras = [{"name": name, "path": None} for name in discovered_names if name not in expected]
        else:
            argv = [programs[host], "--offline", "--mode", "rpc", "--no-session", "--no-extensions",
                    "--no-prompt-templates", "--no-themes", "--no-context-files"]
            for package in packages["pi"].values():
                argv += ["--skill", package]
            with peer(argv, host_work, env) as call:
                response = call({"id": "commands", "type": "get_commands"})
                discovered_names = [command["name"].removeprefix("skill:") for command in response["data"]["commands"]
                                    if command["name"].startswith("skill:")]
                found = set(discovered_names)
                extras = [{"name": command["name"].removeprefix("skill:"), "path": command.get("path")}
                          for command in response["data"]["commands"] if command["name"].startswith("skill:")
                          and command["name"].removeprefix("skill:") not in expected]
        require(expected <= found, host + " missed generated wrappers: " + ", ".join(sorted(expected - found)))
        print("ok " + host + " fresh wrapper discovery: " + ", ".join(sorted(expected)), flush=True)
        # Hosts differ in whether they recursively expose original bundled
        # procedures as separate skills. Preserve source and report those
        # entries; wrappers require exact bundled paths, never global selection.
        print(host + " additional discovered helpers (null path means unavailable in listing): "
              + json.dumps(extras, sort_keys=True), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--bin-dir", type=Path, default=ROOT / ".build/bin")
    parser.add_argument("--ref", help="full library source commit; defaults to current HEAD")
    parser.add_argument("--host-clis", action="store_true", help="also discover every generated skill in installed Codex, Claude Code, and Pi")
    args = parser.parse_args()
    bins = args.bin_dir.resolve()
    require((bins / "brief").is_file(), "build Brief first or select its directory with --bin-dir")
    with tempfile.TemporaryDirectory(prefix="bench-worker-portability-") as temporary:
        work = Path(temporary).resolve()
        env = isolated_environment(work, bins)
        commit = args.ref or run(["git", "-C", ROOT, "rev-parse", "HEAD"], work, env).decode().strip()
        source = committed_source(commit, work, env)
        entries = [("worker", name) for name in WORKERS] + [("team", TEAM)]
        packages = {target: {} for target in TARGETS}
        for kind, name in entries:
            plain = work / "plain" / (kind + "-" + name)
            export(kind, name, plain, commit, work, env)
            expected = verify_plain(plain, kind, name, source, commit)
            for target in TARGETS:
                for execution in (("bench",) if kind == "team" else ("native", "bench")):
                    skill_name = f"bench-{kind}-{name}-{execution}"
                    before = work / "original" / target / skill_name
                    export(kind, name, before, commit, work, env, target, execution)
                    verified_name = verify_projection(before, expected, kind, name, target, execution, bins, work, env)
                    # A path containing spaces catches accidental shell/path dependence.
                    after = work / "relocated packages" / target / skill_name
                    after.parent.mkdir(parents=True, exist_ok=True)
                    before.rename(after)
                    verify_projection(after, expected, kind, name, target, execution, bins, work, env)
                    packages[target][verified_name] = after
            print(f"ok {kind} {name}: committed bytes/modes and locks preserved across {len(TARGETS)} targets; lint and relocation pass", flush=True)
        check_relocated_planner(packages["skill"]["bench-worker-page-planner-native"], work, env)
        if args.host_clis:
            native_hosts(work, bins, packages)
        print(f"worker portability checks passed at {commit}; 49 projections; no model calls or job-quality claim", flush=True)


if __name__ == "__main__":
    main()
