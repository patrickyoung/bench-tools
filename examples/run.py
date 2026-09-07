#!/usr/bin/env python3
"""Finite business recipe: Weave selects work; Tend executes; Ask owns logs."""
import argparse
import fcntl
import json
import os
from pathlib import Path
import shutil
import signal
import select
import subprocess
import sys
import tempfile
import time

import business
import protocol
from protocol import command, digest, encoded, exact, json_rows, read, replace


HERE = Path(__file__).resolve().parent


def guarded_work(tend):
    """EOF means the recipe driver died. Ask Tend to stop its owned execution."""
    child = subprocess.Popen([tend, "work"], stdin=subprocess.DEVNULL)
    while child.poll() is None:
        readable, _, _ = select.select([0], [], [], 0.05)
        if readable and not os.read(0, 1):
            try:
                child.send_signal(signal.SIGINT)
            except ProcessLookupError:
                pass
            break
    return child.wait()


def tool(name):
    configured = os.environ.get(name.upper())
    path = shutil.which(configured or name)
    if path is None:
        raise ValueError(f"install {name} or set {name.upper()} to its executable")
    return str(Path(path).resolve())


def prepare(root, name, mode, model):
    return prepare_plan(root, name, business.plans(name), mode, model, HERE / "business.py")


def prepare_plan(root, label, tasks, mode, model, domain_source, extra_sources=None):
    """Snapshot one finite plan and its trusted, caller-selected Python procedure.

    Workers import the domain as business.py. Extra sources are explicit paths,
    or a mapping from Python filenames to paths; task data never selects code.
    """
    tasks = list(tasks)
    plan = b"".join(encoded(task) for task in tasks)
    tools = {"weave": tool("weave"), "tend": tool("tend")}
    if mode == "model":
        tools.update({"ask": tool("ask"), "ply": tool("ply")})
    sources = {name: read(HERE / name) for name in ("run.py", "worker.py", "protocol.py")}
    sources["business.py"] = read(domain_source)
    extras = extra_sources or {}
    items = extras.items() if hasattr(extras, "items") else ((Path(path).name, path) for path in extras)
    for name, path in items:
        if (not isinstance(name, str) or not name.endswith(".py")
                or not name[:-3].isidentifier() or name in sources):
            raise ValueError(f"invalid or conflicting recipe source name: {name}")
        sources[name] = read(path)
    manifest = {"version": 1, "workflow": label, "mode": mode, "model": model,
                "plan_sha256": digest(plan), "python": str(Path(sys.executable).resolve()),
                "sources": {name: digest(body) for name, body in sources.items()},
                "tools": {name: {"path": path, "sha256": digest_file(path)} for name, path in tools.items()}}
    # A complete manifest is the input contract, not a task-state database.
    exact(root / "manifest.json", encoded(manifest))
    exact(root / "tasks.jsonl", plan)
    recipe = root / "recipe"
    recipe.mkdir(exist_ok=True)
    for name, body in sources.items():
        exact(recipe / name, body)
    (root / "tasks").mkdir(exist_ok=True)
    return tasks, tools


def digest_file(path):
    import hashlib
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for b in iter(lambda: f.read(1024 * 1024), b""):
            h.update(b)
    return "sha256:" + h.hexdigest()


def identity(task):
    return digest(encoded(task)[:-1])


def job_id(task):
    return "w-" + identity(task)[7:47]


def tend(tools, env, *args, data=None):
    return command([tools["tend"], *args], data=data, env=env)


def model_evidence(root, tools, envelope, payload, taskdir):
    mode = envelope["evidence_mode"]
    if mode == "controller-precheck":
        # The worker actually ran the checker in this Tend attempt. No model
        # evidence is claimed; the enclosing output/check bindings are required.
        if (taskdir / "session.jsonl").exists():
            raise ValueError("precheck fallback unexpectedly has a model session")
        return []
    session = taskdir / "session.jsonl"
    if mode != "ask-sealed-check" or envelope.get("session") != str(session):
        raise ValueError("model result lacks its expected Ask evidence")
    snapshot = command([tools["ask"], "replay", "-check", "-json", str(session)])
    events = json_rows(snapshot)
    matches = []
    for i, event in enumerate(events):
        data = event.get("data", {})
        if event.get("type") != "note" or data.get("source") != "weave-example" or data.get("kind") != "weave.example.check/v1":
            continue
        body = data.get("body", {})
        keys = ("task_sha256", "input_sha256", "result_sha256", "checker_sha256", "outcome")
        if any(body.get(key) != envelope.get(key) for key in keys):
            continue
        if i + 1 >= len(events) or events[i+1].get("type") != "seal":
            raise ValueError("accepted check has no adjacent Ask seal")
        seal = events[i+1]
        matches.append({"kind": "ask", "ref": str(session) + "#" + str(event["seq"]),
                        "session": events[0]["data"].get("id"), "seq": event["seq"],
                        "seal_seq": seal["seq"], "seal_sha256": seal["data"]["sha256"]})
    if not matches:
        raise ValueError("verified Ask snapshot has no matching accepted business check")
    if "ply_verifier" in envelope:
        expected = envelope["ply_verifier"]
        matching = []
        for i, event in enumerate(events):
            data = event.get("data", {})
            body = data.get("body", {})
            if (event.get("type") == "note" and data.get("source") == "ply"
                    and data.get("kind") == "ply.verifier/v1" and body.get("outcome") == "accepted"
                    and body.get("exit_code") == 0 and all(body.get(k) == v for k, v in expected.items())):
                if i + 1 >= len(events) or events[i+1].get("type") != "seal":
                    raise ValueError("Ply verifier receipt has no adjacent seal")
                matching.append({"kind": "ply-verifier", "ref": str(session) + "#" + str(event["seq"]),
                                 "seal_sha256": events[i+1]["data"]["sha256"], **expected})
        if not matching:
            raise ValueError("Ply result lacks its matching accepted verifier receipt")
        matches.append(matching[-1])
    # Preserve the snapshot only as a disposable view of the verified log.
    replace(taskdir / "verified.jsonl", snapshot)
    return matches[-2:] if "ply_verifier" in envelope else [matches[-1]]


def observe(root, tasks, tools, env, domain=business):
    jobs = json_rows(tend(tools, env, "list"))
    expected = {job_id(t): t for t in tasks}
    if any(j["id"] not in expected for j in jobs):
        raise ValueError("dedicated Tend root contains an unrelated job")
    # Check the public execution ledger and artifact bindings before deriving
    # outcomes. No SQLite reads or competing projection of Tend event history.
    tend(tools, env, "check")
    observations, results, payloads = [], {}, {}
    manifest = json.loads(read(root / "manifest.json"))
    for name, expected_digest in manifest["sources"].items():
        if digest(read(root / "recipe" / name)) != expected_digest:
            raise ValueError("recipe source changed after admission: " + name)
    for job in jobs:
        task = expected[job["id"]]
        taskdir = root / "tasks" / job["id"]
        expected_argv = [manifest["python"], str(root / "recipe" / "worker.py")]
        if (job["cwd"] != str(taskdir) or job["argv"] != expected_argv
                or job["serial_key"] != str(taskdir) or job.get("check_argv")
                or job["run_dir"] != str(root / "tend" / "jobs" / job["id"])):
            raise ValueError("Tend job differs from the expected recipe execution")
        payload_bytes = read(Path(job["run_dir"]) / "input")
        payload = json.loads(payload_bytes)
        if (payload.get("task") != task or payload.get("task_sha256") != identity(task)
                or payload.get("mode") != manifest["mode"] or payload.get("model") != manifest["model"]
                or payload.get("tools") != tools or set(payload.get("dependencies", {})) != set(task["needs"])):
            raise ValueError("submitted task differs from immutable plan")
        payloads[task["id"]] = payload
        events = json_rows(tend(tools, env, "events", job["id"]))
        evidence = [{"kind": "tend", "ref": str(root / "tend") + "/" + job["id"] + "#" + str(events[-1]["seq"])}]
        state = {"ready": "queued", "running": "running", "waiting": "queued",
                 "unknown": "unknown", "failed": "rejected", "cancelled": "rejected", "done": "accepted"}[job["status"]]
        observation = {"id": task["id"], "task_sha256": identity(task), "state": state,
                       "job": job["id"], "execution_state": job["status"], "evidence": evidence}
        if job["status"] == "done":
            finished = [e for e in events if e["kind"] == "attempt.finished"]
            if not finished or finished[-1]["payload"]["status"] != "done" or finished[-1]["payload"]["exit"] != 0:
                raise ValueError("done job has no observed successful attempt; explicit recheck required")
            event = finished[-1]
            record = event["payload"]
            prepared = next(e["payload"] for e in events if e["kind"] == "attempt.prepared" and e["payload"]["attempt"] == record["attempt"])
            output = Path(job["run_dir"]) / "attempts" / (f'{prepared["number"]:03d}.out')
            raw = read(output)
            if digest(raw)[7:] != record["output_digest"] or len(raw) != record["output_size"]:
                raise ValueError("Tend output differs from its finished event")
            envelope = json.loads(raw)
            if (envelope.get("task_sha256") != identity(task) or envelope.get("input_sha256") != digest(payload_bytes)
                    or envelope.get("checker_sha256") != digest(read(root / "recipe" / "business.py"))
                    or envelope.get("outcome") != "accepted"
                    or envelope.get("result_sha256") != digest(encoded(envelope.get("result")))):
                raise ValueError("result lacks matching task, input, check, or artifact bindings")
            # Also ensure the named artifact still contains those exact bytes.
            if read(taskdir / "result.json") != encoded(envelope["result"]):
                raise ValueError("result artifact changed after acceptance")
            domain.check(task, envelope["result"], payload["dependencies"])
            if payload["mode"] == "model" and domain.agent_task(task):
                evidence.extend(model_evidence(root, tools, envelope, payload, taskdir))
            elif envelope["evidence_mode"] != "controller-check":
                raise ValueError("reference result claims unexpected model evidence")
            evidence.extend([{"kind": "artifact", "ref": str(output), "sha256": digest(raw)},
                             {"kind": "check", "ref": str(output) + "#outcome", "checker_sha256": envelope["checker_sha256"]}])
            observation.update({"attempt": record["attempt"], "result": str(taskdir / "result.json"),
                                "result_sha256": envelope["result_sha256"], "evidence_mode": envelope["evidence_mode"]})
            results[task["id"]] = envelope["result"]
        observations.append(observation)
    for payload in payloads.values():
        for id, dependency in payload["dependencies"].items():
            if id not in results or encoded(dependency) != encoded(results[id]):
                raise ValueError("submitted dependency differs from its accepted predecessor artifact")
    return observations, results, jobs


def drive(args, root, tasks, tools, env, domain=business, output=None):
    if output is None:
        output = sys.stdout.buffer
    deadline = time.monotonic() + args.seconds
    protocol.DEADLINE = deadline
    active = []
    stopped = False
    def interrupt(signum, frame):
        nonlocal stopped
        stopped = True
        protocol.INTERRUPTED = True
    old = {s: signal.signal(s, interrupt) for s in (signal.SIGINT, signal.SIGTERM)}
    try:
        while True:
            if stopped or time.monotonic() >= deadline:
                raise InterruptedError("recipe interrupted or run time limit reached")
            observations, results, jobs = observe(root, tasks, tools, env, domain)
            snapshot = b"".join(encoded(o) for o in observations)
            replace(root / "observations.jsonl", snapshot)
            ready = json_rows(command([tools["weave"], str(root / "tasks.jsonl")], snapshot))
            for task in ready:
                if stopped or time.monotonic() >= deadline:
                    raise InterruptedError("recipe interrupted during admission")
                directory = root / "tasks" / job_id(task)
                directory.mkdir(exist_ok=True)
                payload = {"task": task, "task_sha256": identity(task),
                           "dependencies": {id: results[id] for id in task["needs"]},
                           "mode": args.mode, "model": args.model, "tools": tools}
                tend(tools, env, "submit", "-id", job_id(task), "-C", str(directory), "--",
                     str(Path(sys.executable).resolve()), str(root / "recipe" / "worker.py"), data=encoded(payload))
            # All admitted jobs are in this root. A bounded wave of one-shot
            # calls may recover a prior attempt instead of executing new work.
            jobs = json_rows(tend(tools, env, "list"))
            runnable = sum(j["status"] == "ready" for j in jobs)
            running = sum(j["status"] == "running" for j in jobs)
            count = min(args.jobs - running, runnable)
            if count <= 0:
                observations, results, jobs = observe(root, tasks, tools, env, domain)
                replace(root / "observations.jsonl", b"".join(encoded(o) for o in observations))
                states = {o["id"]: o for o in observations}
                for task in tasks:
                    row = states.get(task["id"], {"id": task["id"], "task_sha256": identity(task),
                                                 "state": "blocked", "needs": task["needs"]})
                    output.write(encoded(row))
                output.flush()
                return 0 if len(states) == len(tasks) and all(o["state"] == "accepted" for o in observations) else 1
            remaining = runnable
            while active or remaining:
                if stopped or time.monotonic() >= deadline:
                    raise InterruptedError("recipe interrupted or run time limit reached")
                while remaining and len(active) < args.jobs - running:
                    out = tempfile.TemporaryFile()
                    process = subprocess.Popen([sys.executable, str(root / "recipe" / "run.py"), "_work", tools["tend"]],
                                               stdin=subprocess.PIPE, stdout=out, env=env)
                    active.append((process, out))
                    remaining -= 1
                for process, out in active[:]:
                    if process.poll() is None:
                        continue
                    process.stdin.close()
                    out.seek(0)
                    body = out.read(1024 * 1024 + 1)
                    out.close()
                    active.remove((process, out))
                    if len(body) > 1024 * 1024 or process.returncode not in (0, 1):
                        raise RuntimeError("Tend work controller failed; inspect its event log")
                if active:
                    time.sleep(0.025)
    except subprocess.TimeoutExpired:
        if time.monotonic() >= deadline:
            raise InterruptedError("recipe run time limit reached")
        raise
    finally:
        protocol.DEADLINE = None
        protocol.INTERRUPTED = False
        # Closing the parent pipe asks each guard to signal Tend, which owns
        # cancellation and records observed versus uncertain effects.
        for process, out in active:
            process.stdin.close()
        for process, out in active:
            process.wait(timeout=30)
            out.close()
        if stopped or active or time.monotonic() >= deadline:
            for job in json_rows(tend(tools, env, "list")):
                if job["status"] in ("ready", "running"):
                    tend(tools, env, "cancel", job["id"])
        for signum, handler in old.items():
            signal.signal(signum, handler)


def main():
    if len(sys.argv) == 3 and sys.argv[1] == "_work":
        return guarded_work(sys.argv[2])
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("workflow", choices=("policy-review", "invoice-process"))
    parser.add_argument("directory", type=Path)
    parser.add_argument("-j", "--jobs", type=int, default=4)
    parser.add_argument("--mode", choices=("reference", "model"), default="reference")
    parser.add_argument("--model", default=os.environ.get("ASK_MODEL", ""))
    parser.add_argument("--seconds", type=int, default=600)
    args = parser.parse_args()
    if not 1 <= args.jobs <= 32 or not 1 <= args.seconds <= 86400:
        parser.error("jobs must be 1..32; seconds must be 1..86400")
    if args.mode == "model" and not args.model:
        parser.error("model mode requires --model or ASK_MODEL")
    os.umask(0o077)
    root = args.directory.resolve()
    root.mkdir(parents=True, exist_ok=True)
    with open(root / ".lock", "a+b") as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        tasks, tools = prepare(root, args.workflow, args.mode, args.model)
        env = os.environ.copy()
        env.update({"TEND_ROOT": str(root / "tend"), "TEND_JOB_MAX": "5m", "TEND_LEASE": "3s"})
        if args.mode == "model":
            env.update({"ASK": tools["ask"], "PLY": tools["ply"]})
        print(f"{args.workflow}: {len(tasks)} tasks, at most {args.jobs} active workers; {args.mode} mode", file=sys.stderr)
        return drive(args, root, tasks, tools, env)


if __name__ == "__main__":
    try:
        code = main()
    except (InterruptedError, KeyboardInterrupt) as e:
        print(f"run: {str(e) or 'interrupted'}; inspect Tend before resuming", file=sys.stderr)
        code = 130
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError, KeyError) as e:
        print(f"run: {e}", file=sys.stderr)
        code = 2
    sys.exit(code)
