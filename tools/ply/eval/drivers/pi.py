#!/usr/bin/env python3
"""Pi task adapter; uses native OAuth refresh with private shared auth state."""
import json
import os
from pathlib import Path
import shutil
import signal
import stat
import subprocess
import sys


def parse_events(events, process_code):
    assistants = [event["message"] for event in events
                  if event.get("type") == "message_end" and event.get("message", {}).get("role") == "assistant"]
    if not assistants or not any(event.get("type") == "agent_end" for event in events):
        raise ValueError("Pi stream has no completed agent turn")
    final = assistants[-1]
    answer = "\n".join(part["text"] for part in final.get("content", []) if part.get("type") == "text")
    usages = [message.get("usage", {}) for message in assistants]
    usages.extend(event["message"]["usage"] for event in events if event.get("type") == "message_end"
                  and event.get("message", {}).get("role") == "toolResult" and event["message"].get("usage"))
    compactions = [event for event in events if event.get("type") == "compaction_end"]
    usages.extend(event["result"]["usage"] for event in compactions
                  if isinstance(event.get("result"), dict) and event["result"].get("usage"))
    retries = sum(event.get("type") in ("auto_retry_start", "summarization_retry_attempt_start") for event in events)
    unreported = sum(not any(usage.get(key, 0) > 0 for key in ("input", "output", "cacheRead", "cacheWrite")) for usage in usages)
    unreported += sum(not isinstance(event.get("result"), dict) or not event["result"].get("usage") for event in compactions)
    usage = {"cost_usd": None, "source": "Pi completed native message and compaction events; catalog cost estimate only",
             "observed_turns": len(assistants), "unreported_turns": unreported, "unreported_retries": retries}
    for field, key in {"input_tokens": "input", "output_tokens": "output", "cached_input_tokens": "cacheRead",
                       "cache_write_tokens": "cacheWrite", "reasoning_tokens": "reasoning"}.items():
        observed = [u[key] for u in usages if key in u]
        usage[field] = sum(observed) if len(observed) == len(usages) and not (unreported or retries) else None
        usage["observed_" + field] = sum(observed) if observed else None
    estimates = [u.get("cost", {}).get("total") for u in usages]
    estimated_cost = sum(estimates) if all(n is not None and n > 0 for n in estimates) and not (unreported or retries) else None
    failed = final.get("stopReason") in ("error", "aborted")
    interventions = [{"kind": event["type"], "detail": event.get("errorMessage", event.get("reason", ""))}
                     for event in events if event.get("type") in ("auto_retry_start", "compaction_start", "summarization_retry_attempt_start")]
    return {"answer": answer, "claimed_success": "EVAL_SUCCESS" in answer.splitlines(),
            "exit_code": process_code or (1 if failed else 0), "usage": usage, "interventions": interventions,
            "native": {"resolved_models": sorted({str(m.get("provider")) + "/" + str(m.get("model")) for m in assistants}),
                       "stop_reason": final.get("stopReason"), "error": final.get("errorMessage"),
                       "estimated_cost_usd": estimated_cost,
                       "cost_basis": "catalog estimate, not billed cost", "turn_limit": "not available in native Pi CLI"}}


def invocation(request, environ):
    if request.get("suite") != "task" or request.get("resume"):
        raise ValueError("Pi adapter supports fresh task trials only")
    executable = environ.get("PLY_EVAL_PI") or shutil.which("pi")
    if not executable:
        raise ValueError("configure PLY_EVAL_PI")
    root = environ.get("PLY_EVAL_AUTH_ROOT")
    if not root:
        raise ValueError("configure a private PLY_EVAL_AUTH_ROOT outside the run artifacts")
    auth = Path(root).resolve() / "pi"
    state = Path(request["state_dir"]).resolve()
    for path in (auth.parent, auth, auth / "auth.json"):
        metadata = path.lstat()
        if stat.S_ISLNK(metadata.st_mode) or metadata.st_uid != os.getuid() or metadata.st_mode & 0o077:
            raise ValueError("Pi auth paths must be private, owned, and not symlinks")
    if state == auth or state in auth.parents or auth in state.parents:
        raise ValueError("auth and trial state must be separate")
    options = request.get("options", {})
    args = [executable, "--offline", "--mode", "json", "--print", "--no-extensions", "--no-skills",
            "--no-prompt-templates", "--no-themes", "--no-context-files", "--no-approve",
            "--tools", "read,bash,edit,write", "--model", request["model"],
            "--thinking", options.get("effort", "medium"), "--session", str(state / "pi-session.jsonl"),
            "--session-dir", str(state)]
    env = dict(environ, PI_CODING_AGENT_DIR=str(auth), PI_CODING_AGENT_SESSION_DIR=str(state), PI_TELEMETRY="0")
    return args, env


def main():
    request = json.load(sys.stdin)
    args, env = invocation(request, os.environ)
    native_path = Path(request["state_dir"]) / "pi-events.jsonl"
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
    (Path(request["state_dir"]) / "pi-metadata.json").write_text(json.dumps(result.pop("native"), indent=2) + "\n")
    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError, KeyError, TypeError) as error:
        print("Pi driver: " + str(error), file=sys.stderr)
        sys.exit(1)
