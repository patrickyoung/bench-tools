#!/usr/bin/env python3
"""Claude Code task adapter; consumes native login only from a private auth root."""
import json
import os
from pathlib import Path
import shutil
import signal
import stat
import subprocess
import sys


def parse_events(events, process_code):
    results = [event for event in events if event.get("type") == "result"]
    if len(results) != 1:
        raise ValueError("Claude stream must contain one terminal result")
    result = results[0]
    answer = result.get("result", "")
    if not isinstance(answer, str):
        raise ValueError("Claude result text is not a string")
    source = result.get("usage")
    usage = None
    if isinstance(source, dict):
        usage = {field: source.get(key) for field, key in {
            "input_tokens": "input_tokens", "output_tokens": "output_tokens",
            "cached_input_tokens": "cache_read_input_tokens",
            "cache_write_tokens": "cache_creation_input_tokens"}.items()}
        usage.update({"cost_usd": None,
                      "source": "Claude terminal result usage; CLI cost is a client estimate, billed cost unknown"})
    models = set(result.get("modelUsage", {}))
    models.update(event["model"] for event in events if event.get("type") == "system" and isinstance(event.get("model"), str))
    models.update(event["message"]["model"] for event in events
                  if event.get("type") == "assistant" and isinstance(event.get("message", {}).get("model"), str))
    errors = result.get("errors", [])
    failed = bool(result.get("is_error")) or result.get("subtype", "success") != "success"
    interventions = [{"kind": "permission_denial", "detail": denial}
                     for denial in result.get("permission_denials", [])]
    return {"answer": answer, "claimed_success": "EVAL_SUCCESS" in answer.splitlines(),
            "exit_code": process_code or (1 if failed else 0), "usage": usage,
            "interventions": interventions,
            "native": {"resolved_models": sorted(models), "result_subtype": result.get("subtype"),
                       "num_turns": result.get("num_turns"), "errors": errors,
                       "estimated_cost_usd": result.get("total_cost_usd"),
                       "cost_basis": "client estimate, not billed cost"}}


def invocation(request, environ):
    if request.get("suite") != "task" or request.get("resume"):
        raise ValueError("Claude adapter supports fresh task trials only")
    executable = environ.get("PLY_EVAL_CLAUDE") or shutil.which("claude")
    if not executable:
        raise ValueError("configure PLY_EVAL_CLAUDE")
    root = environ.get("PLY_EVAL_AUTH_ROOT")
    if not root:
        raise ValueError("configure a private PLY_EVAL_AUTH_ROOT outside the run artifacts")
    auth = Path(root).resolve() / "claude"
    state = Path(request["state_dir"]).resolve()
    for path in (auth.parent, auth, auth / ".credentials.json"):
        metadata = path.lstat()
        if stat.S_ISLNK(metadata.st_mode) or metadata.st_uid != os.getuid() or metadata.st_mode & 0o077:
            raise ValueError("Claude auth paths must be private, owned, and not symlinks")
    if state == auth or state in auth.parents or auth in state.parents:
        raise ValueError("auth and trial state must be separate")
    budget, options = request.get("budget", {}), request.get("options", {})
    args = [executable, "--safe-mode", "-p", "--output-format", "stream-json", "--verbose",
            "--no-chrome", "--disable-slash-commands", "--strict-mcp-config", "--mcp-config", '{"mcpServers":{}}',
            "--setting-sources", "", "--no-session-persistence",
            "--tools", "Bash,Read,Edit,Write,Glob,Grep", "--allowedTools", "Bash,Read,Edit,Write,Glob,Grep",
            "--permission-mode", "dontAsk", "--permission-prompts", "none",
            "--max-turns", str(budget.get("turns", 20)), "--model", request["model"],
            "--effort", options.get("effort", "medium")]
    env = dict(environ, CLAUDE_CONFIG_DIR=str(auth), CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1")
    # The sole shared mutable state is native login and its refresh/cache directory.
    # Conversation persistence is disabled; raw native events live in trial state.
    return args, env


def main():
    request = json.load(sys.stdin)
    args, env = invocation(request, os.environ)
    state = Path(request["state_dir"])
    native_path = state / "claude-events.jsonl"
    with native_path.open("wb") as output:
        process = subprocess.Popen(args, cwd=request["workdir"], env=env, stdin=subprocess.PIPE,
                                   stdout=output, stderr=sys.stderr)

        def interrupted(signum, frame):
            if process.poll() is None:
                process.send_signal(signum)

        signal.signal(signal.SIGTERM, interrupted)
        signal.signal(signal.SIGINT, interrupted)
        process.communicate(request["prompt"].encode())
    events = [json.loads(line) for line in native_path.read_text().splitlines() if line.strip()]
    result = parse_events(events, process.returncode)
    (state / "claude-metadata.json").write_text(json.dumps(result.pop("native"), indent=2) + "\n")
    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError, KeyError, TypeError) as error:
        print("Claude driver: " + str(error), file=sys.stderr)
        sys.exit(1)
