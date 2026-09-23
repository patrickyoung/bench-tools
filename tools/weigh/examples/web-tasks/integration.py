#!/usr/bin/env python3
"""Real Web + Chromium + Weigh + Record, with an offline Decisions fixture.

This proves executable composition and application behavior, not Jev accuracy.
All inputs and all model answers are synthetic. Never reads ambient credentials.
"""
import argparse
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import subprocess
import sys
import threading

from fixture_site import site

HERE = Path(__file__).resolve().parent


@contextmanager
def decisions():
    calls = []
    controls = {"fail": False, "missing_distribution": False}

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def do_POST(self):
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            calls.append(request)
            if self.headers.get("Authorization") != "Bearer offline-fixture-key":
                self.send_error(401)
                return
            if controls["fail"]:
                self.send_error(503, "fixture private diagnostic")
                return
            state, answers = request["state"], {}
            for name, q in request["questions"].items():
                if q["type"] == "noul":
                    answers[name] = {"type": "noul", "noul": .99 if "14 days" in state["page"]["text"] else .01}
                    continue
                if name == "next":
                    candidates = state["links"]
                    desired = {"/": "/support", "/help/index.html": "/help/guides.html",
                               "/help/guides.html": "/help/backup.html"}
                    from urllib.parse import urlsplit
                    target = desired.get(urlsplit(state["page"]["url"]).path)
                    selected = next((key for key, value in candidates.items()
                                     if urlsplit(value["url"]).path == target), "stop")
                else:
                    rid, criterion = name.split(".")
                    # Deliberately scripted labels over fixtures, not a keyword
                    # classifier masquerading as JEV or a quality benchmark.
                    selected = {"record_1": {"workspace": "yes", "refundable": "yes"},
                                "record_2": {"workspace": "no", "refundable": "no"},
                                "record_3": {"workspace": "yes", "refundable": "unknown"}}[rid][criterion]
                others = len(q["criteria"]) - 1
                answer = {"type": "choice", "choice": selected, "confidence": .96,
                          "probabilities": {key: .98 if key == selected else .02 / others for key in q["criteria"]}}
                if controls["missing_distribution"]:
                    del answer["probabilities"]
                answers[name] = answer
            body = json.dumps({"model": "typesafe/offline-browser-fixture", "answers": answers,
                               "usage": {"input_tokens": 100, "output_tokens": 10, "cost": 0}}).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}/api/alpha/decisions", calls, controls
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


def require(condition, reason):
    if not condition:
        raise RuntimeError(reason)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--web", required=True)
    parser.add_argument("--weigh", required=True)
    parser.add_argument("--record", required=True)
    parser.add_argument("--ask", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=False)
    root = args.output.resolve()
    env = {key: os.environ[key] for key in ("PATH", "TMPDIR", "PLAYWRIGHT_BROWSERS_PATH") if key in os.environ}
    env.update(HOME=str(Path.home()), BENCH_WEIGH="1", OPENROUTER_API_KEY="offline-fixture-key",
               WEB_STATE=str(root / "web-audit.jsonl"), PYTHONDONTWRITEBYTECODE="1")
    # HOME only locates Chromium's installed cache; no provider environment or
    # saved browser profile is inherited. Web runs a new anonymous context.
    criteria = root / "criteria.json"
    criteria.write_text(json.dumps({"workspace": "There is a quiet dedicated place to work.",
                                    "refundable": "Cancellation with a full refund is explicitly offered."}))
    with site() as (base, visits), decisions() as (endpoint, calls, controls):
        # Selecting a link as a record must retain that link's own URL, not
        # only descendant anchors. This is common for result/card components.
        anchors = subprocess.run([args.web, "snapshot", base + "/stays", "--records-selector", "article > a"],
                                 env=env, capture_output=True, timeout=45)
        require(anchors.returncode == 0, "Web anchor snapshot failed")
        observed = json.loads(anchors.stdout)
        require(len(observed["records"]) == 3 and all(len(row["links"]) == 1 for row in observed["records"]),
                "link-shaped record lost its own URL")
        visits.clear()
        common = [sys.executable, str(HERE / "browser_tasks.py"), "--live", "--accept-at", ".9",
                  "--model", "openrouter/~typesafe/fixture", "--endpoint", endpoint,
                  "--web", args.web, "--weigh", args.weigh, "--record", args.record, "--ask", args.ask]

        def run(mode, name, extra, code=0, environment=env):
            proc = subprocess.run([*common, mode, "--output", str(root / name), *extra],
                                  env=environment, capture_output=True, timeout=180)
            require(proc.returncode == code, f"{name}: expected {code}, got {proc.returncode}: {proc.stderr.decode()}")
            if code:
                require(not proc.stdout, name + " emitted a result after failure")
                return None
            return json.loads(proc.stdout)

        nav = ["--url", base + "/", "--goal", "Find the backup export instructions and download retention period.",
               "--min-confidence", ".8", "--expect-text", "14 days"]
        before = len(calls)
        run("navigate", "disabled", nav, 2, dict(env, BENCH_WEIGH="0"))
        require(not (root / "disabled").exists() and len(calls) == before and not visits,
                "disabled path started dependencies")
        found = run("navigate", "navigate", nav)
        require(found["status"] == "found" and found["verification"] == "observed_text"
                and found["url"] == base + "/help/backup.html" and len(found["steps"]) == 4,
                "did not reach independently checked backup page")
        require(len(calls) == 4, "navigation made unexpected inference calls")
        require("/help/guides.html" in visits, "redirect-relative link was not resolved")
        rows = run("shortlist", "shortlist", ["--url", base + "/stays", "--criteria", str(criteria), "--selector", "article"])
        require(len(calls) == 5 and len(calls[-1]["questions"]) == 6, "record criteria were not batched in one inference")
        for group, name in (("selected", "Garden Studio"), ("rejected", "Festival Loft"), ("review", "Harbor Cabin")):
            require(len(rows[group]) == 1 and name in rows[group][0]["text"], group + " classification failed")
        require("Hidden stale" not in json.dumps(calls[-1]), "hidden record reached judgment")
        empty = run("shortlist", "empty", ["--url", base + "/stays", "--criteria", str(criteria), "--selector", ".missing"])
        require(empty["status"] == "empty" and len(calls) == 5, "empty records made an inference")
        controls["fail"] = True
        before = len(calls)
        run("navigate", "failure", nav, 2)
        require(len(calls) == before + 1, "provider failure retried")
        controls.update(fail=False, missing_distribution=True)
        before = len(calls)
        run("navigate", "invalid-response", nav, 2)
        require(len(calls) == before + 1, "invalid provider response retried")
    # Service is now offline, proving replay does not repeat browser/model work.
    receipts = list(root.glob("*/*.record.jsonl"))
    for receipt in receipts:
        check = subprocess.run([args.record, "check", "-ask", args.ask, "-f", str(receipt)],
                               env=env, capture_output=True, timeout=30)
        require(check.returncode == 0, "Record verification failed")
    receipt = sorted((root / "navigate").glob("*-weigh.record.jsonl"))[-1]
    replay = subprocess.run([args.record, "replay", "-ask", args.ask, "-f", str(receipt)],
                            env={key: value for key, value in env.items() if key != "OPENROUTER_API_KEY"},
                            capture_output=True, timeout=30)
    expected = Path(str(receipt).removesuffix(".record.jsonl") + ".json").read_bytes()
    require(replay.returncode == 0 and replay.stdout == expected, "offline replay changed the judgment")
    report = {"mode": "offline HTTP fixture with real Chromium and real CLIs", "model_quality_evaluated": False,
              "navigation_pages": len(found["steps"]), "shortlisted": len(rows["selected"]),
              "review": len(rows["review"]), "rejected": len(rows["rejected"]),
              "verified_recordings": len(receipts), "output": str(root)}
    (root / "summary.json").write_text(json.dumps(report, indent=2) + "\n")
    print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()
