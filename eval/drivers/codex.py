#!/usr/bin/env python3
"""Isolated, task-only adapter for the installed Codex exec JSON interface."""
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys
import tempfile

LIMIT = 8 * 1024 * 1024


def observations(events):
    completed = [event for event in events if event.get("type") == "turn.completed"]
    failed = sum(event.get("type") == "turn.failed" for event in events)
    started = sum(event.get("type") == "turn.started" for event in events)
    usages = [event.get("usage") for event in completed]
    missing = max(failed, started - len(completed)) + sum(not isinstance(value, dict) for value in usages)
    usages = [value for value in usages if isinstance(value, dict)]
    models = set()
    for event in events:
        for value in (event, event.get("session"), event.get("response")):
            if isinstance(value, dict) and isinstance(value.get("model"), str):
                models.add(value["model"])
    if not usages:
        return None, sorted(models)
    result = {}
    fields = {"input_tokens": "input_tokens", "output_tokens": "output_tokens",
              "cached_input_tokens": "cached_input_tokens", "reasoning_tokens": "reasoning_output_tokens"}
    for target, source in fields.items():
        values = [value[source] for value in usages if source in value]
        if any(type(value) is not int or value < 0 for value in values):
            raise ValueError("invalid Codex usage counter")
        result[target] = sum(values) if not missing and len(values) == len(completed) else None
        result["observed_" + target] = sum(values) if values else None
    result.update({"cost_usd": None, "cache_write_tokens": None,
                   "observed_turns": len(completed), "unreported_turns": missing,
                   "source": "codex exec JSON turn.completed usage; native user-turn totals, not individual model-call counts; billed cost unavailable"})
    return result, sorted(models)


def private_source(root):
    if root.is_symlink() or not root.is_dir():
        raise ValueError("PLY_EVAL_AUTH_ROOT must be a private directory")
    info = root.stat()
    if info.st_uid != os.getuid() or info.st_mode & 0o077:
        raise ValueError("PLY_EVAL_AUTH_ROOT must be owned by this user with mode 0700")
    auth = root / "codex" / "auth.json"
    if auth.is_symlink() or not auth.is_file():
        raise ValueError("private Codex auth.json is missing or is a symlink")
    info = auth.stat()
    if info.st_uid != os.getuid() or info.st_mode & 0o077:
        raise ValueError("private Codex auth.json must be owned by this user with mode 0600")
    return auth


def main():
    request = json.load(sys.stdin)
    if request.get("schema") != "ply.eval/request/v1" or request.get("suite") != "task" or request.get("resume"):
        raise ValueError("Codex adapter supports fresh task-suite trials only")
    binary = os.environ.get("PLY_EVAL_CODEX")
    private_root = os.environ.get("PLY_EVAL_AUTH_ROOT")
    if not binary or not private_root:
        raise ValueError("configure PLY_EVAL_CODEX and PLY_EVAL_AUTH_ROOT")
    root = Path(private_root)
    auth = private_source(root)
    work, state = Path(request["workdir"]).resolve(), Path(request["state_dir"]).resolve()
    if root.resolve().is_relative_to(work) or root.resolve().is_relative_to(state):
        raise ValueError("private auth root must be outside trial artifacts")
    state.mkdir(parents=True, exist_ok=True)
    events_path, answer_path = state / "codex-events.jsonl", state / "codex-answer.txt"
    if events_path.exists() or answer_path.exists():
        raise ValueError("Codex artifacts already exist; refusing to overwrite a trial")
    effort = request.get("options", {}).get("effort", "medium")
    if not isinstance(effort, str) or not effort:
        raise ValueError("options.effort must be a nonempty string")
    with tempfile.TemporaryDirectory(prefix="codex-invocation-", dir=root) as private:
        home = Path(private)
        codex_home = home / ".codex"
        codex_home.mkdir(mode=0o700)
        shutil.copyfile(auth, codex_home / "auth.json")
        (codex_home / "auth.json").chmod(0o600)
        cache = auth.parent / "models_cache.json"
        if cache.is_file() and not cache.is_symlink():
            shutil.copyfile(cache, codex_home / cache.name)
        # An empty HOME and CODEX_HOME exclude personal configuration, skills,
        # sessions and hooks. These overrides also exclude project guidance,
        # host skill discovery, connected apps and tool network access.
        args = [binary, "exec", "--json", "--ignore-user-config", "--ignore-rules", "--ephemeral",
                "--skip-git-repo-check", "--color", "never", "-s", "workspace-write",
                "-c", 'approval_policy="never"', "-c", 'cli_auth_credentials_store="file"',
                "-c", 'web_search="disabled"',
                "-c", "sandbox_workspace_write.network_access=false", "-c", "project_doc_max_bytes=0",
                "-c", "features.skip_host_skill_discovery=true", "-c", "features.plugins=false",
                "-c", "features.apps=false", "-c", "features.skill_mcp_dependency_install=false",
                "-c", "features.multi_agent=false", "-c", "shell_environment_policy.experimental_use_profile=false",
                "-c", "model_reasoning_effort=" + json.dumps(effort),
                "-m", request["model"], "-C", str(work), "-o", str(answer_path), "-"]
        env = {**os.environ, "HOME": str(home), "CODEX_HOME": str(codex_home)}
        env.pop("PLY_EVAL_AUTH_ROOT", None)
        env.pop("PLY_EVAL_CODEX", None)
        with events_path.open("wb") as output:
            process = subprocess.Popen(args, cwd=work, stdin=subprocess.PIPE, stdout=output, stderr=sys.stderr, env=env)

            def interrupted(signum, frame):
                if process.poll() is None:
                    try:
                        process.send_signal(signum)
                    except ProcessLookupError:
                        pass

            previous = {signum: signal.signal(signum, interrupted) for signum in (signal.SIGINT, signal.SIGTERM)}
            try:
                process.communicate(request["prompt"].encode())
            finally:
                for signum, handler in previous.items():
                    signal.signal(signum, handler)
        # OAuth refresh can rotate credentials. Publish only to the campaign's
        # private base; the controller owns any reconciliation with user auth.
        # Campaign native calls are serialized, so this never rewrites another
        # concurrent trial's refresh from an older starting copy.
        refreshed = codex_home / "auth.json"
        if refreshed.read_bytes() != auth.read_bytes():
            descriptor, replacement = tempfile.mkstemp(prefix=".auth-refresh-", dir=auth.parent)
            try:
                with os.fdopen(descriptor, "wb") as output:
                    output.write(refreshed.read_bytes())
                    output.flush()
                    os.fsync(output.fileno())
                os.replace(replacement, auth)
            finally:
                if os.path.exists(replacement):
                    os.unlink(replacement)
        if events_path.stat().st_size > LIMIT or (answer_path.exists() and answer_path.stat().st_size > LIMIT):
            raise ValueError("Codex artifact exceeded capture limit")
        events = [json.loads(line) for line in events_path.read_bytes().splitlines() if line.strip()]
        if any(not isinstance(event, dict) for event in events):
            raise ValueError("Codex emitted a non-object JSON event")
        usage, models = observations(events)
        # -o identifies the final message. Only a successfully completed native
        # turn permits fallback to the last agent message; interrupted progress
        # is not a final response.
        answer = answer_path.read_text() if answer_path.exists() else ""
        if not answer and process.returncode == 0 and any(event.get("type") == "turn.completed" for event in events):
            messages = [event["item"].get("text", "") for event in events if event.get("type") == "item.completed"
                        and isinstance(event.get("item"), dict) and event["item"].get("type") == "agent_message"]
            answer = messages[-1] if messages else ""
        (state / "codex-provenance.json").write_text(json.dumps({
            "configured_model": request["model"], "effort": effort, "observed_model_ids": models,
            "model_evidence": "nonsecret native JSON model fields" if models else "native JSON did not identify the resolved model",
            "tool_protocol": "native", "web_search": "disabled", "sandbox": "workspace-write",
            "command_network_access": False, "multi_agent": False,
            "budget_enforcement": "parent harness wall-clock deadline; native CLI has no adapter-enforced turn or action count cap",
        }, sort_keys=True) + "\n")
        print(json.dumps({"answer": answer, "claimed_success": "EVAL_SUCCESS" in answer.splitlines(),
                          "exit_code": process.returncode, "usage": usage, "interventions": []}))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError) as error:
        print("codex driver: " + str(error), file=sys.stderr)
        sys.exit(1)
