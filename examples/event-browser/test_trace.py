"""Real public-command fixtures; no model, credentials, or provider network."""
import base64
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import threading
import time
import unittest
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, HTTPServer

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("bench_trace", HERE / "trace.py")
trace = importlib.util.module_from_spec(spec)
spec.loader.exec_module(trace)
ASK = shutil.which(os.environ.get("TRACE_TEST_ASK", "ask"))
RECORD = shutil.which(os.environ.get("TRACE_TEST_RECORD", "record"))


@unittest.skipUnless(ASK and RECORD, "select current public Ask and Record with TRACE_TEST_ASK / TRACE_TEST_RECORD")
class ArchiveTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="trace-test-")
        self.root = Path(self.temp.name)
        self.addCleanup(self.temp.cleanup)

    def command(self, *args, data=b"", code=0):
        p = subprocess.run(list(map(str, args)), cwd=self.root, input=data, capture_output=True, timeout=20)
        self.assertEqual(p.returncode, code, p.stderr.decode(errors="replace"))
        return p.stdout

    def archive(self, *paths, **kwargs):
        archive = trace.Archive(paths, ASK, RECORD, **kwargs)
        self.addCleanup(archive.close)
        return archive

    def note(self, path, kind, body):
        self.command(ASK, "note", "-q", "-f", path, "-s", "fixture", "-k", kind, "-json", "-", "-seal", data=json.dumps(body).encode())

    def session(self, path):
        path.parent.mkdir(parents=True, exist_ok=True)
        self.command(ASK, "init", "-f", path)
        self.note(path, "example/v1", {"text": "<script>window.EVIDENCE_EXECUTED=true</script>"})
        return path

    def record(self, path, command=None, options=(), data=b"", code=0):
        path.parent.mkdir(parents=True, exist_ok=True)
        self.command(RECORD, "run", "-ask", ASK, "-f", path, *options, "--", *(command or ["/usr/bin/true"]), data=data, code=code)
        return path

    def index(self, directory, process, parent="", conversation=None):
        directory.mkdir(parents=True, exist_ok=True)
        inputs = self.record(directory / "inputs.jsonl")
        outputs = self.record(directory / "outputs.jsonl", options=["-session", conversation] if conversation else [])
        index = directory / "index.jsonl"
        self.command(ASK, "init", "-f", index)
        digest = lambda p: "sha256:" + hashlib.sha256(p.read_bytes()).hexdigest()
        self.note(index, "ply.recording/v1", {"phase": "start", "parent": str(parent), "directory": str(directory.parent.parent / "workspace"), "inputs": str(inputs), "inputs_sha256": digest(inputs)})
        self.note(index, "ply.recording/v1", {"phase": "process", "role": "action", "path": str(process), "session": str(conversation) if conversation else ""})
        self.note(index, "ply.recording/v1", {"phase": "process-complete", "path": str(process), "sha256": digest(process)})
        if conversation:
            self.note(index, "ply.recording/v1", {"phase": "session", "path": str(conversation), "previous": ""})
        self.note(index, "ply.recording/v1", {"phase": "terminal", "exit": 0, "complete": True, "outputs": str(outputs), "outputs_sha256": digest(outputs), "missing_sessions": []})
        return index

    def test_binary_streams_and_artifacts_do_not_repeat_effect(self):
        artifact = self.root / "input.bin"
        content = bytes(range(256)) * 100
        artifact.write_bytes(content)
        effect = self.root / "effect"
        session = self.record(self.root / "receipt.jsonl", [sys.executable, "-c", "import pathlib,sys;pathlib.Path('effect').write_text('one');sys.stdout.buffer.write(sys.stdin.buffer.read());sys.stderr.write('distinct stderr');sys.exit(7)"], options=["-input", artifact], data=content, code=7)
        original = session.read_bytes()
        archive = self.archive(session)
        snapshot = archive.refresh()
        s = snapshot["sessions"][0]
        self.assertEqual((s["state"], s["verification"], s["complete"], s["exit"]), ("failed", "verified", True, 7))
        self.assertEqual(base64.b64decode(archive.stream(s["id"], "stdout")["base64"]), content)
        self.assertEqual(base64.b64decode(archive.stream(s["id"], "stderr")["base64"]), b"distinct stderr")
        artifact.unlink();effect.write_text("do not rerun")
        self.assertEqual(base64.b64decode(archive.stream(s["id"], "artifact:0")["base64"]), content)
        exported = archive.export()
        self.assertIn(base64.b64encode(content).decode(), exported)
        self.assertEqual(effect.read_text(), "do not rerun")
        self.assertEqual(session.read_bytes(), original)

    def test_live_append_partial_line_then_corruption_and_deletion(self):
        session = self.session(self.root / "live.jsonl")
        archive = self.archive(self.root)
        first = archive.refresh();self.assertEqual(len(first["events"]), 2)
        self.assertEqual(archive.refresh()["revision"], first["revision"])
        self.note(session, "progress/v1", {"step": "next"})
        second = archive.refresh();self.assertGreater(second["revision"], first["revision"])
        self.assertEqual(len(second["events"]), 3)
        good = session.read_bytes()
        session.write_bytes(good + b'{"seq":')
        incomplete = archive.refresh();self.assertEqual(incomplete["sessions"][0]["verification"], "incomplete")
        sid = incomplete["sessions"][0]["id"]
        self.assertEqual(archive.sessions[sid]["originalFile"].read_bytes(), good + b'{"seq":')
        self.assertEqual(len(incomplete["events"]), 3)
        session.write_bytes(good.replace(b'"next"', b'"evil"'))
        damaged = archive.refresh();self.assertEqual(damaged["sessions"][0]["verification"], "invalid")
        session.unlink();self.assertEqual(archive.refresh()["sessions"], [])

    def test_live_record_prefix_then_terminal(self):
        session = self.root / "process.jsonl"
        child = subprocess.Popen([RECORD, "run", "-ask", ASK, "-f", str(session), "--", sys.executable, "-c", "import sys,time;print('first',flush=True);time.sleep(1);print('last',flush=True)"], cwd=self.root, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.addCleanup(lambda: child.poll() is None and child.kill())
        self.assertEqual(child.stdout.readline(), b"first\n")
        archive = self.archive(session)
        prefix = archive.refresh();s = prefix["sessions"][0]
        self.assertFalse(s["complete"])
        self.assertEqual(s["state"], "incomplete")
        observed = archive.stream(s["id"], "stdout")
        self.assertFalse(observed["verified"])
        self.assertEqual(base64.b64decode(observed["base64"]), b"first\n")
        out, err = child.communicate(timeout=10);self.assertEqual(child.returncode, 0, err)
        completed = archive.refresh()["sessions"][0]
        self.assertEqual((completed["state"], completed["complete"]), ("complete", True))
        self.assertEqual(base64.b64decode(archive.stream(s["id"], "stdout")["base64"]), b"first\nlast\n")

    def test_nested_roots_relocation_and_retained_conversation(self):
        original = self.root / "original"
        parentdir = original / "parent/recordings/run.1"
        process = self.record(parentdir / "action.1/session.jsonl", options=["-label", "role=action"])
        conversation = self.session(original / "parent/runs/conversation.jsonl")
        self.index(parentdir, process, conversation=conversation)
        childdir = original / "child/recordings/run.2"
        childprocess = self.record(childdir / "action.2/session.jsonl")
        self.index(childdir, childprocess, parent=process)
        moved = self.root / "moved"
        shutil.copytree(original, moved);shutil.rmtree(original)
        (moved / "parent/runs/conversation.jsonl").unlink()
        archive = self.archive(moved)
        snapshot = archive.refresh()
        self.assertEqual(len(snapshot["lanes"]), 2)
        self.assertEqual(len([e for e in snapshot["edges"] if e["kind"] == "child"]), 1)
        self.assertEqual(len([s for s in snapshot["sessions"] if s["embedded"]]), 1)
        self.assertTrue(all(l["state"] == "accepted" for l in snapshot["lanes"]))
        self.assertFalse(snapshot["issues"], snapshot["issues"])

    def test_index_commitments_reject_substituted_valid_receipt(self):
        directory = self.root / "control/recordings/run.1"
        process = self.record(directory / "action.1/session.jsonl")
        self.index(directory, process)
        replacement = self.record(self.root / "replacement.jsonl", ["/bin/echo", "other"])
        process.write_bytes(replacement.read_bytes())
        snapshot = self.archive(self.root / "control").refresh()
        self.assertEqual(snapshot["lanes"][0]["verification"], "invalid")
        self.assertFalse(snapshot["lanes"][0]["state"] == "accepted")
        self.assertTrue(any("commitment" in i["message"] for i in snapshot["issues"]))

    def test_missing_committed_receipt_is_incomplete(self):
        directory = self.root / "control/recordings/run.1"
        process = self.record(directory / "action.1/session.jsonl")
        self.index(directory, process);process.unlink()
        snapshot = self.archive(self.root / "control").refresh()
        self.assertEqual(snapshot["lanes"][0]["state"], "incomplete")

    def test_relocated_duplicate_session_is_not_counted_twice(self):
        original = self.root / "original"
        directory = original / "recordings/run.1"
        process = self.record(directory / "action.1/session.jsonl")
        conversation = self.session(original / "runs/ask.jsonl")
        self.index(directory, process, conversation=conversation)
        moved = self.root / "moved";shutil.copytree(original, moved);shutil.rmtree(original)
        snapshot = self.archive(moved).refresh()
        self.assertEqual(len([s for s in snapshot["sessions"] if s["kind"] == "conversation"]), 1)
        self.assertFalse(snapshot["issues"])

    def test_exact_request_via_ask_public_replay_without_model_on_inspection(self):
        calls = []
        class Fixture(BaseHTTPRequestHandler):
            def log_message(self, *_):pass
            def do_POST(self):
                calls.append(json.loads(self.rfile.read(int(self.headers["Content-Length"]))))
                response = {"id": "response_fixture", "object": "response", "status": "in_progress", "model": "fixture", "output": []}
                answer = "A retained answer."
                events = [
                    {"type": "response.created", "sequence_number": 0, "response": response},
                    {"type": "response.output_text.delta", "sequence_number": 1, "item_id": "msg_fixture", "output_index": 0, "content_index": 0, "delta": answer},
                    {"type": "response.completed", "sequence_number": 2, "response": {**response, "status": "completed", "output": [{"type": "message", "id": "msg_fixture", "role": "assistant", "status": "completed", "content": [{"type": "output_text", "text": answer, "annotations": []}]}], "usage": {"input_tokens": 10, "output_tokens": 5, "total_tokens": 15}}}]
                wire = b"".join(("event: " + e["type"] + "\ndata: " + json.dumps(e) + "\n\n").encode() for e in events)
                self.send_response(200);self.send_header("Content-Type", "text/event-stream");self.send_header("Content-Length", str(len(wire)));self.end_headers();self.wfile.write(wire)
        server = HTTPServer(("127.0.0.1", 0), Fixture)
        thread = threading.Thread(target=server.serve_forever, daemon=True);thread.start()
        session = self.root / "ask.jsonl"
        env = dict(os.environ, OPENAI_API_KEY="fixture-only", OPENAI_BASE_URL="http://127.0.0.1:" + str(server.server_port) + "/v1")
        try:
            result = subprocess.run([ASK, "-q", "-m", "openai/fixture", "-f", str(session), "Distinct user input."], env=env, capture_output=True, timeout=20)
            self.assertEqual(result.returncode, 0, result.stderr)
        finally:
            server.shutdown();server.server_close();thread.join()
        archive = self.archive(session);snapshot = archive.refresh()
        event = next(e for e in snapshot["events"] if e["type"] == "model")
        detail = archive.detail(event["id"])
        self.assertIn("Distinct user input.", json.dumps(detail["normalizedRequest"]))
        self.assertEqual(len(calls), 1)
        self.assertEqual(snapshot["sessions"][0]["state"], "complete")
        self.assertIn("normalizedRequest", archive.export())

    def test_clock_rollback_preserves_session_sequence(self):
        # Historical unsealed text-only log: interpreted by actual Ask, with no
        # fabricated Record completion or cryptographic assertion.
        session = self.root / "legacy.jsonl"
        events = [
            {"seq": 1, "time": "2026-01-01T00:00:01Z", "type": "session", "data": {"id": "old", "ask": "0.1.0", "model": "fixture", "system": ""}},
            {"seq": 2, "time": "2026-01-01T00:00:02Z", "type": "user", "data": {"text": "First"}},
            {"seq": 3, "time": "2026-01-01T00:00:00Z", "type": "user", "data": {"text": "Second, after clock rollback"}}]
        session.write_text("".join(json.dumps(e) + "\n" for e in events))
        snapshot = self.archive(session).refresh()
        self.assertEqual([e["seq"] for e in snapshot["events"]], [1, 2, 3])
        self.assertTrue(any("clock moved backward" in i["message"] for i in snapshot["issues"]))

    def test_null_payload_abort_is_visible(self):
        session = self.root / "abort.jsonl"
        self.command(ASK, "init", "-f", session)
        rows = [json.loads(line) for line in session.read_text().splitlines()]
        with session.open("a") as output:
            output.write(json.dumps({"seq": rows[-1]["seq"] + 1, "time": rows[-1]["time"], "type": "abort", "data": None}) + "\n")
        snapshot = self.archive(session).refresh()
        self.assertEqual(snapshot["sessions"][0]["state"], "interrupted")
        self.assertTrue(any(e["type"] == "error" and e["title"] == "Interrupted conversation" for e in snapshot["events"]))

    def test_matching_artifact_bytes_are_separate_from_parentage(self):
        produced = self.root / "answer.txt";produced.write_text("A unique retained deliverable")
        producer = self.record(self.root / "producer.jsonl", options=["-output", produced])
        consumed = self.root / "copied.txt";shutil.copyfile(produced, consumed)
        consumer = self.record(self.root / "consumer.jsonl", options=["-input", consumed])
        snapshot = self.archive(producer, consumer).refresh()
        self.assertEqual(len(snapshot["edges"]), 1)
        self.assertEqual(snapshot["edges"][0]["kind"], "artifact")
        self.assertIn("Matching retained bytes", snapshot["edges"][0]["label"])
        self.assertTrue(snapshot["edges"][0]["verified"])

    def test_duplicate_roots_and_caps_and_no_symlink_following(self):
        session = self.session(self.root / "selected/log.jsonl")
        secret = self.session(self.root / "outside/private.jsonl")
        (session.parent / "outside.jsonl").symlink_to(secret)
        archive = self.archive(session.parent, session)
        self.assertEqual(len(archive.refresh()["sessions"]), 1)
        capped = self.archive(session, max_bytes=10).refresh()
        self.assertEqual(capped["sessions"], [])
        self.assertTrue(any("not silently truncated" in i["message"] for i in capped["issues"]))

    def test_http_scope_live_updates_and_original_download(self):
        session = self.session(self.root / "serve.jsonl")
        archive = self.archive(session);archive.refresh()
        server = trace.TraceServer(("127.0.0.1", 0), archive, .05)
        workers = [threading.Thread(target=server.serve_forever, daemon=True), threading.Thread(target=server.collect, daemon=True)]
        for worker in workers:worker.start()
        def cleanup():
            server.stop.set()
            with server.condition:server.condition.notify_all()
            server.shutdown();server.server_close()
            for worker in workers:worker.join(timeout=5)
        self.addCleanup(cleanup)
        base = "http://127.0.0.1:" + str(server.server_port)
        url = base + server.prefix
        with urllib.request.urlopen(url + "api/snapshot") as r: first = json.load(r)
        self.note(session, "new/v1", {"message": "arrived"})
        deadline = time.monotonic() + 3
        while time.monotonic() < deadline:
            with urllib.request.urlopen(url + "api/snapshot") as r: second = json.load(r)
            if second["revision"] > first["revision"]:break
            time.sleep(.05)
        self.assertGreater(second["revision"], first["revision"])
        sid = second["sessions"][0]["id"]
        with urllib.request.urlopen(url + "api/session/" + sid) as r:self.assertEqual(r.read(), session.read_bytes())
        for path, headers, expected in [("/api/snapshot", {}, 404), (server.prefix + "api/snapshot", {"Origin": "https://example.com"}, 403), (server.prefix + "api/snapshot", {"Host": "evil.example"}, 403), (server.prefix + "api/session/../../etc/passwd", {}, 400)]:
            with self.assertRaises(urllib.error.HTTPError) as error:urllib.request.urlopen(urllib.request.Request(base + path, headers=headers))
            self.assertEqual(error.exception.code, expected)


class ProjectionTests(unittest.TestCase):
    def test_go_fractional_seconds_and_script_escaping(self):
        self.assertEqual(trace.millis("2026-09-14T23:49:04.21856Z"), 1789429744218)
        self.assertEqual(trace.millis("2026-09-14T23:49:04.218567891Z"), 1789429744218)
        self.assertEqual(trace.millis("2026-09-14T23:49:04Z"), 1789429744000)
        encoded = trace.javascript_json({"attack": "</script><script>alert(1)</script>"})
        self.assertNotIn("<", encoded)
        self.assertEqual(json.loads(encoded)["attack"], "</script><script>alert(1)</script>")
        page = trace.page({"schema": trace.SCHEMA, "text": "__DATA__ __OFFLINE__ __PREFIX__"})
        self.assertIn('"text":"__DATA__ __OFFLINE__ __PREFIX__"', page)


if __name__ == "__main__":
    unittest.main()
