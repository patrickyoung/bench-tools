#!/usr/bin/env python3
"""Public Ply/Ask CLI adapter. Credentials are passed only by explicit environment names."""
import json
import os
from pathlib import Path
import shutil
import signal
import subprocess
import sys


def aggregate_usage(snapshots):
    """Sum unique assistant events, including summary and partial responses."""
    seen, turns, summary_ids, requests, retries = {}, [], set(), set(), set()
    session_ids = set()
    for events in snapshots:
        if not events or events[0].get("type") != "session":
            raise ValueError("verified replay has no session header")
        header = events[0]["data"]
        session = header["id"]
        session_ids.add(session)
        if header.get("summary"):
            summary_ids.add(header["summary"])
        for event in events:
            if event.get("type") == "request":
                requests.add((session, event["seq"]))
            if event.get("type") == "retry":
                retries.add((session, event["seq"]))
            if event.get("type") != "assistant":
                continue
            key = session, event["seq"]
            if key in seen:
                if seen[key] != event["data"]:
                    raise ValueError("conflicting assistant observations for one session event")
                continue
            seen[key] = event["data"]
            turns.append((session, event["data"]))
    if not turns:
        return None
    counts = {"input_tokens": "in", "output_tokens": "out", "cached_input_tokens": "cache_read",
              "cache_write_tokens": "cache_write", "reasoning_tokens": "reasoning"}
    usages = [data.get("usage", {}) for _, data in turns]
    unreported_requests = max(0, len(requests) - len(turns))
    unreported = unreported_requests + len(retries) + sum(not any(usage.get(key, 0) > 0 for key in counts.values()) for usage in usages)
    result = {}
    for field, key in counts.items():
        observations = [usage[key] for usage in usages if key in usage]
        result[field] = sum(observations) if not unreported and len(observations) == len(usages) else None
        result["observed_" + field] = sum(observations) if observations else None
    costs = [usage.get("cost") for usage in usages]
    result.update({"cost_usd": sum(costs) if not unreported and all(cost is not None and cost > 0 for cost in costs) else None,
                   "observed_cost_usd": sum(cost for cost in costs if cost is not None and cost > 0) if any(cost is not None and cost > 0 for cost in costs) else None,
                   "observed_turns": len(turns), "unreported_turns": unreported,
                   "unreported_requests": unreported_requests, "unreported_retries": len(retries),
                   "partial_turns": sum(bool(data.get("partial")) for _, data in turns),
                   "summary_turns": sum(session in summary_ids for session, _ in turns), "session_count": len(session_ids),
                   "source": "ask replay -check -json; unique session-id/assistant-seq; cumulative trial including summaries and partial responses"})
    return result


def main():
    request = json.load(sys.stdin)
    state = Path(request["state_dir"])
    ply = os.environ.get("PLY_EVAL_PLY") or shutil.which("ply")
    ask = os.environ.get("PLY_EVAL_ASK") or shutil.which("ask")
    if not ply or not ask:
        raise ValueError("configure PLY_EVAL_PLY and PLY_EVAL_ASK")
    lifecycle = request["suite"] == "lifecycle"
    model = "anthropic/fixture" if lifecycle else request["model"]
    budget, options = request["budget"], request["options"]
    args = [ply, "-sh", "-no-delegate", "-C", request["workdir"],
            "-checkpoint", str(state / "checkpoint"), "-m", model, "-turns", str(budget.get("turns", 12)),
            "-cycles", str(budget.get("cycles", 1)), "-timeout", str(budget.get("action_seconds", 60)) + "s"]
    compact_at = options.get("compact_at", 1 if request["case"] == "repeated-compaction" else 0)
    if compact_at:
        args.extend(["-compact-at", str(compact_at)])
    args.extend(options.get("extra_args", []))
    env = {**os.environ, "ASK": ask, "ASK_DIR": str(state / "ask"), "PLY_DIR": str(state / "ply")}
    if lifecycle:
        env.update({"ANTHROPIC_API_KEY": "offline-fixture", "ANTHROPIC_BASE_URL": request["fixture_url"]})
    process = subprocess.Popen(args, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=sys.stderr, env=env)

    def interrupted(signum, frame):
        if process.poll() is None:
            process.send_signal(signum)

    signal.signal(signal.SIGTERM, interrupted)
    signal.signal(signal.SIGINT, interrupted)
    answer, _ = process.communicate(request["prompt"].encode())
    answer = answer.decode(errors="replace")
    interventions, snapshots = [], []
    replay_dir = state / "verified-replays"
    replay_dir.mkdir(exist_ok=True)
    for session in sorted(state.rglob("*.jsonl")):
        replay = subprocess.run([ask, "replay", "-check", "-json", str(session)], capture_output=True, env=env)
        if replay.returncode:
            interventions.append({"kind": "unverified_session", "session": str(session)})
        else:
            snapshots.append([json.loads(line) for line in replay.stdout.splitlines() if line.strip()])
            (replay_dir / (session.stem + ".json")).write_bytes(replay.stdout)
    if interventions:
        raise ValueError("an Ask session did not replay; see captured stderr")
    claimed = "EVAL_SUCCESS" in answer.splitlines()
    usage = aggregate_usage(snapshots)
    if lifecycle and usage:
        usage["source"] += "; scripted fixture counts"
    print(json.dumps({"answer": answer, "claimed_success": claimed, "exit_code": process.returncode,
                      "usage": usage, "interventions": interventions}))
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except (ValueError, OSError) as error:
        print("ply driver: " + str(error), file=sys.stderr)
        sys.exit(1)
