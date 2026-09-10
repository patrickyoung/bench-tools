"""Public-CLI integration: real Weave/Tend/Ask/Ply; only inference is a fixture."""
import copy
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import signal
import subprocess
import sys
import tempfile
import threading
import time
import unittest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "examples"))
import business
import run as recipe
from protocol import encoded, json_rows, read


class ModelFixture:
    """An explicit local Messages API fixture, never a claim about model quality."""
    def __init__(self, broken_synthesis=False, delay=0, fail=False, action_report=None):
        self.calls = []
        self.lock = threading.Lock()
        self.broken_synthesis = broken_synthesis
        self.delay, self.fail = delay, fail
        self.action_report = action_report
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                data = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                payload = None
                for message in data["messages"]:
                    content = message["content"]
                    for part in ([{"text": content}] if isinstance(content, str) else content):
                        for line in part.get("text", "").splitlines():
                            try:
                                item = json.loads(line)
                            except ValueError:
                                continue
                            if isinstance(item, dict) and "task" in item and "accepted_dependencies" in item:
                                payload = item
                if payload is None:
                    self.send_error(400, "fixture requires explicit task evidence")
                    return
                task = payload["task"]
                with fixture.lock:
                    prior = fixture.calls.count(task["id"])
                    fixture.calls.append(task["id"])
                if fixture.fail:
                    self.send_error(400, "intentional fixture rejection")
                    return
                delay = fixture.delay.get(task["id"], 0) if isinstance(fixture.delay, dict) else fixture.delay
                time.sleep(delay)
                result = business.execute(task, payload["accepted_dependencies"])
                result["mode"] = "model"
                if fixture.broken_synthesis and task["input"]["kind"] == "synthesis" and prior == 0:
                    result["guardrails"] = []
                answer = encoded(result).decode().strip()
                if fixture.action_report is not None and task["input"]["kind"] == "synthesis":
                    answer = fixture.action_report
                events = [
                    ("message_start", {"type": "message_start", "message": {"id": "msg_fixture", "type": "message", "role": "assistant", "model": "fixture", "content": [], "stop_reason": None, "usage": {"input_tokens": 10, "output_tokens": 0}}}),
                    ("content_block_start", {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}),
                    ("content_block_delta", {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": answer}}),
                    ("content_block_stop", {"type": "content_block_stop", "index": 0}),
                    ("message_delta", {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": None}, "usage": {"output_tokens": max(1, len(answer)//4)}}),
                    ("message_stop", {"type": "message_stop"}),
                ]
                body = "".join(f"event: {kind}\ndata: {json.dumps(event)}\n\n" for kind, event in events).encode()
                try:
                    self.send_response(200)
                    self.send_header("Content-Type", "text/event-stream")
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                except (BrokenPipeError, ConnectionResetError):
                    pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def __enter__(self):
        self.thread.start()
        return self

    def __exit__(self, *args):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def env(self):
        return {"ANTHROPIC_API_KEY": "offline-fixture-key",
                "ANTHROPIC_BASE_URL": f"http://127.0.0.1:{self.server.server_port}",
                "TEND_PASS": "ANTHROPIC_API_KEY ANTHROPIC_BASE_URL"}


class IntegrationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        bins = Path(os.environ.get("WEAVE_TEST_BIN", ROOT / "var" / "bin"))
        cls.env = os.environ.copy()
        for name in ("weave", "tend", "ask", "ply"):
            path = bins / name
            if not path.is_file():
                raise RuntimeError(f"build tools first with tests/check; missing {path}")
            cls.env[name.upper()] = str(path)

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="weave-test-")
        self.addCleanup(self.temp.cleanup)
        self.directory = Path(self.temp.name) / "study"

    def invoke(self, workflow="policy-review", extra=(), env=None, directory=None):
        return subprocess.run([sys.executable, str(ROOT / "examples" / "run.py"), workflow,
                               str(directory or self.directory), *extra],
                              env={**self.env, **(env or {})}, capture_output=True, timeout=120)

    def events(self, directory=None):
        r = subprocess.run([self.env["TEND"], "events"], env={**self.env, "TEND_ROOT": str((directory or self.directory) / "tend")},
                           capture_output=True, check=True)
        return json_rows(r.stdout)

    def test_reference_workflows_resume_without_new_attempts(self):
        for workflow in ("policy-review", "invoice-process"):
            with self.subTest(workflow=workflow):
                directory = Path(self.temp.name) / workflow
                result = self.invoke(workflow, directory=directory)
                self.assertEqual(result.returncode, 0, result.stderr.decode())
                rows = json_rows(result.stdout)
                self.assertEqual(len(rows), len(business.plans(workflow)))
                self.assertTrue(all(row["state"] == "accepted" for row in rows))
                before = self.events(directory)
                again = self.invoke(workflow, directory=directory)
                self.assertEqual(again.returncode, 0, again.stderr.decode())
                self.assertEqual(before, self.events(directory))
                self.assertEqual(result.stdout, again.stdout)

    def test_model_wires_and_ply_rejection_recovery(self):
        with ModelFixture(broken_synthesis=True) as fixture:
            result = self.invoke(extra=("--mode", "model", "--model", "anthropic/fixture"), env=fixture.env())
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            self.assertEqual(fixture.calls.count("synthesis"), 2)
            rows = json_rows(result.stdout)
            for row in rows:
                self.assertIn("ask", [ref["kind"] for ref in row["evidence"]])
            final = next(row for row in rows if row["id"] == "synthesis")
            self.assertIn("ply-verifier", [ref["kind"] for ref in final["evidence"]])
            snapshot = json_rows(read(Path(final["result"]).parent / "verified.jsonl"))
            verdicts = [e["data"]["body"]["outcome"] for e in snapshot if e["type"] == "note" and e["data"].get("kind") in ("ply.verifier/v1", "ply.verifier/v2")]
            self.assertIn("rejected", verdicts)
            self.assertIn("accepted", verdicts)

    def test_invoice_model_parameters_reach_simulation(self):
        with ModelFixture() as fixture:
            result = self.invoke("invoice-process", extra=("--mode", "model", "--model", "anthropic/fixture"), env=fixture.env())
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            rows = json_rows(result.stdout)
            self.assertEqual(len(rows), 9)
            self.assertEqual(len(fixture.calls), 4)  # three designs and one pilot
            self.assertTrue(all(row["state"] == "accepted" for row in rows))

    def test_failed_worker_does_not_release_synthesis(self):
        with ModelFixture(fail=True) as fixture:
            result = self.invoke(extra=("--mode", "model", "--model", "anthropic/fixture"), env=fixture.env())
            self.assertEqual(result.returncode, 1, result.stderr.decode())
            rows = json_rows(result.stdout)
            self.assertEqual(next(r for r in rows if r["id"] == "synthesis")["state"], "blocked")
            self.assertNotIn("synthesis", fixture.calls)

    def test_changed_artifact_prevents_admission_on_resume(self):
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        rows = json_rows(result.stdout)
        Path(rows[0]["result"]).write_text('{}\n')
        again = self.invoke()
        self.assertEqual(again.returncode, 2)
        self.assertIn(b"result artifact changed", again.stderr)
        self.assertEqual(again.stdout, b"")

    def test_three_hundred_tasks_respect_eight_worker_bound(self):
        # Use the actual business worker 300 times, with unique immutable task
        # identities. The filter/driver/Tend path is unchanged by the harness.
        harness = """
import copy,sys
sys.path.insert(0,sys.argv[1])
import run,business
template=business.plans('policy-review')[0]
tasks=[]
for i in range(300):
    t=copy.deepcopy(template); t['id']='review-%03d'%i; tasks.append(t)
business.plans=lambda name:tasks
sys.argv=['run.py','policy-review',sys.argv[2],'-j','8']
raise SystemExit(run.main())
"""
        result = subprocess.run([sys.executable, "-c", harness, str(ROOT / "examples"), str(self.directory)],
                                env=self.env, capture_output=True, timeout=120)
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        rows = json_rows(result.stdout)
        self.assertEqual(len(rows), 300)
        self.assertEqual(len({r["id"] for r in rows}), 300)
        self.assertTrue(all(r["state"] == "accepted" for r in rows))
        active, peak, started, finished = 0, 0, 0, 0
        for event in sorted(self.events(), key=lambda e: e["created_us"]):
            if event["kind"] == "attempt.started":
                active += 1; started += 1; peak = max(peak, active)
            elif event["kind"] == "attempt.finished":
                active -= 1; finished += 1
        self.assertEqual((active, started, finished), (0, 300, 300))
        self.assertGreater(peak, 1)
        self.assertLessEqual(peak, 8)

    def test_tend_same_key_writers_serialize(self):
        root = Path(self.temp.name)
        env = {**self.env, "TEND_ROOT": str(root / "serial-tend")}
        script = "import time; time.sleep(.08); print('checked')"
        for i in range(6):
            subprocess.run([env["TEND"], "submit", "-id", f"writer-{i}", "-C", str(root),
                            "--", sys.executable, "-c", script], input=b"", env=env, capture_output=True, check=True)
        for _ in range(6):
            workers = [subprocess.Popen([env["TEND"], "work"], env=env, stdin=subprocess.DEVNULL,
                                        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL) for _ in range(4)]
            for p in workers:
                self.assertIn(p.wait(timeout=15), (0, 1))
        events = json_rows(subprocess.check_output([env["TEND"], "events"], env=env))
        active = peak = 0
        for e in sorted(events, key=lambda e: e["created_us"]):
            if e["kind"] == "attempt.started":
                active += 1; peak = max(peak, active)
            elif e["kind"] == "attempt.finished":
                active -= 1
        self.assertEqual((active, peak), (0, 1))
        self.assertEqual(len([e for e in events if e["kind"] == "attempt.finished"]), 6)

    def test_precheck_without_session_has_current_controller_evidence(self):
        harness = """
import sys
from pathlib import Path
sys.path.insert(0,sys.argv[1])
import run,business
from protocol import encoded
root=Path(sys.argv[2]);root.mkdir()
task=next(t for t in business.plans('invoice-process') if t['input']['kind']=='method')
task['needs']=[]
business.plans=lambda name:[task]
run.prepare(root,'invoice-process','model','anthropic/fixture')
d=root/'tasks'/run.job_id(task);d.mkdir()
result=business.execute(task,{});result['mode']='model'
(d/'result.json').write_bytes(encoded(result))
sys.argv=['run.py','invoice-process',str(root),'--mode','model','--model','anthropic/fixture']
raise SystemExit(run.main())
"""
        result = subprocess.run([sys.executable, "-c", harness, str(ROOT / "examples"), str(self.directory)],
                                env=self.env, capture_output=True, timeout=30)
        self.assertEqual(result.returncode, 0, result.stderr.decode())
        row = json_rows(result.stdout)[0]
        self.assertEqual(row["state"], "accepted")
        self.assertEqual(row["evidence_mode"], "controller-precheck")
        self.assertIn("check", [r["kind"] for r in row["evidence"]])
        self.assertFalse(list(self.directory.glob("tasks/*/session.jsonl")))

    def test_corrupted_ask_receipt_is_not_reused(self):
        with ModelFixture() as fixture:
            args = ("--mode", "model", "--model", "anthropic/fixture")
            result = self.invoke(extra=args, env=fixture.env())
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            session = next(self.directory.glob("tasks/*/session.jsonl"))
            data = session.read_bytes()
            self.assertIn(b'"outcome":"accepted"', data)
            session.write_bytes(data.replace(b'"outcome":"accepted"', b'"outcome":"rejected"', 1))
            calls = list(fixture.calls)
            again = self.invoke(extra=args, env=fixture.env())
            self.assertEqual(again.returncode, 2)
            self.assertEqual(again.stdout, b"")
            self.assertEqual(calls, fixture.calls)

    def test_model_shell_action_is_refused_without_effect(self):
        marker = Path(self.temp.name) / "model-created"
        with ModelFixture(action_report="```ply\ntouch " + str(marker) + "\n```") as fixture:
            result = self.invoke(extra=("--mode", "model", "--model", "anthropic/fixture"), env=fixture.env())
            self.assertEqual(result.returncode, 1, result.stderr.decode())
            rows = {row["id"]: row for row in json_rows(result.stdout)}
            self.assertEqual(rows["synthesis"]["state"], "rejected")
            self.assertTrue(all(rows["review-" + lens]["state"] == "accepted" for lens in business.LENSES))
            self.assertFalse(marker.exists(), "model-authored shell action ran")
            self.assertEqual(fixture.calls.count("synthesis"), 1)

    def test_driver_death_interrupts_owned_tend_jobs(self):
        self.check_driver_death("review")

    def test_driver_death_during_ply_stops_without_retry(self):
        self.check_driver_death("synthesis")

    def check_driver_death(self, phase):
        with ModelFixture(delay=8 if phase == "review" else {"synthesis": 8}) as fixture:
            env = {**self.env, **fixture.env()}
            err = tempfile.TemporaryFile()
            proc = subprocess.Popen([sys.executable, str(ROOT / "examples" / "run.py"), "policy-review",
                                     str(self.directory), "-j", "1", "--mode", "model", "--model", "anthropic/fixture"],
                                    env=env, stdout=subprocess.DEVNULL, stderr=err)
            try:
                until = time.monotonic() + 15
                def started():
                    return bool(fixture.calls) if phase == "review" else "synthesis" in fixture.calls
                while not started() and time.monotonic() < until:
                    time.sleep(.05)
                self.assertTrue(started(), phase + " model worker never started")
                proc.kill()
                proc.wait(timeout=5)
                until = time.monotonic() + 12
                statuses = []
                while time.monotonic() < until:
                    r = subprocess.run([env["TEND"], "list"], env={**env, "TEND_ROOT": str(self.directory / "tend")}, capture_output=True, check=True)
                    statuses = [j["status"] for j in json_rows(r.stdout)]
                    if "running" not in statuses:
                        break
                    time.sleep(.1)
                self.assertNotIn("running", statuses)
                self.assertIn("unknown", statuses)
                if phase == "review":
                    self.assertNotIn("done", statuses)
                before = len([e for e in self.events() if e["kind"] == "attempt.started"])
                again = self.invoke(extra=("-j", "1", "--mode", "model", "--model", "anthropic/fixture"), env=fixture.env())
                self.assertEqual(again.returncode, 1, again.stderr.decode())
                # Unstarted independent siblings may run, but the uncertain job
                # has exactly its original attempt and is never resubmitted.
                unknown = next(j for j in json_rows(r.stdout) if j["status"] == "unknown")
                task = json.loads(read(Path(unknown["run_dir"]) / "input"))["task"]
                self.assertEqual(task["input"]["kind"], phase)
                self.assertEqual(fixture.calls.count(task["id"]), 1)
                starts = [e for e in self.events() if e["job"] == unknown["id"] and e["kind"] == "attempt.started"]
                self.assertEqual(len(starts), 1)
                self.assertGreaterEqual(before, 1)
            finally:
                if proc.poll() is None:
                    proc.kill(); proc.wait()
                err.close()


if __name__ == "__main__":
    unittest.main()
