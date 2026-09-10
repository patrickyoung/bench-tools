"""Controlled Messages-API faults for lifecycle contracts, never task-quality scores."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import threading
import time


class Fixture:
    def __init__(self, case):
        self.case, self.events, self.turns, self.failures, self.effects = case, [], 0, 0, 0
        self.lock, self.closed = threading.Lock(), threading.Event()
        fixture = self

        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_GET(self):
                if self.path != "/effect":
                    self.send_error(404)
                    return
                fixture.record("effect_inspected")
                self.respond(json.dumps({"count": fixture.effects}).encode(), "application/json")

            def do_POST(self):
                if self.path == "/effect":
                    with fixture.lock:
                        fixture.effects += 1
                        fixture.events.append({"kind": "effect_committed", "origin": "fixture", "count": fixture.effects, "time_ns": time.monotonic_ns()})
                    # The effect exists while its response is deliberately withheld.
                    fixture.closed.wait(20)
                    self.respond(b'{"count":1}', "application/json")
                    return
                if self.path not in ("/v1/messages", "/messages"):
                    self.send_error(404)
                    return
                payload = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                with fixture.lock:
                    if fixture.case == "transient-provider-failure" and fixture.failures == 0:
                        fixture.failures += 1
                        fixture.events.append({"kind": "provider_failure", "origin": "fixture", "status": 429, "time_ns": time.monotonic_ns()})
                        self.send_response(429)
                        self.send_header("Content-Type", "application/json")
                        self.send_header("Retry-After", "0")
                        self.end_headers()
                        self.wfile.write(b'{"type":"error","error":{"type":"rate_limit_error","message":"controlled transient failure"}}')
                        return
                    system = json.dumps(payload.get("system", ""))
                    summary = "handoff note" in system
                    if summary:
                        fixture.events.append({"kind": "compaction", "origin": "fixture", "continuity": "AMBER-7401" in json.dumps(payload.get("messages", [])), "time_ns": time.monotonic_ns()})
                        answer = "AMBER-7401. Preserve the full three-stage goal. Inspect progress.txt and complete stage-one, stage-two, complete exactly once. The previous action already ran; do not repeat it."
                    else:
                        n = fixture.turns
                        fixture.turns += 1
                        continuity = "AMBER-7401" in json.dumps(payload.get("messages", []))
                        fixture.events.append({"kind": "model_response", "origin": "fixture", "ordinal": n, "continuity": continuity, "time_ns": time.monotonic_ns()})
                        answer = fixture.answer(n)
                wire = [
                    ("message_start", {"type": "message_start", "message": {"id": "msg_fixture", "type": "message", "role": "assistant", "model": "fixture", "content": [], "usage": {"input_tokens": 100, "output_tokens": 0}}}),
                    ("content_block_start", {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}),
                    ("content_block_delta", {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": answer}}),
                    ("content_block_stop", {"type": "content_block_stop", "index": 0}),
                    ("message_delta", {"type": "message_delta", "delta": {"stop_reason": "end_turn"}, "usage": {"output_tokens": 50}}),
                    ("message_stop", {"type": "message_stop"}),
                ]
                self.respond("".join(f"event: {kind}\ndata: {json.dumps(data)}\n\n" for kind, data in wire).encode(), "text/event-stream")

            def respond(self, body, content_type):
                try:
                    self.send_response(200)
                    self.send_header("Content-Type", content_type)
                    self.send_header("Content-Length", str(len(body)))
                    self.end_headers()
                    self.wfile.write(body)
                except (BrokenPipeError, ConnectionResetError):
                    pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.server.daemon_threads = True
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.url = f"http://127.0.0.1:{self.server.server_port}"

    def record(self, kind, **fields):
        with self.lock:
            self.events.append({"kind": kind, "origin": "fixture", "time_ns": time.monotonic_ns(), **fields})

    def snapshot(self):
        with self.lock:
            return list(self.events)

    def answer(self, n):
        if self.case == "transient-provider-failure":
            script = "printf recovered > result.txt" if n == 0 else None
        elif self.case == "repeated-compaction":
            scripts = ["printf 'stage-one\\n' > progress.txt", "printf 'stage-two\\n' >> progress.txt", "printf 'complete\\n' >> progress.txt"]
            script = scripts[n] if n < len(scripts) else None
        elif self.case == "cancellation-resume":
            script = "echo $$ > worker.pid; touch .ready; sleep 60" if n == 0 else "printf resumed > result.txt" if n == 1 else None
        else:
            script = f"curl -fsS -X POST '{self.url}/effect' > effect-response.json" if n == 0 else f"curl -fsS '{self.url}/effect' > result.json" if n == 1 else None
        return f"```sh\n{script}\n```" if script else "Observed the requested result.\nEVAL_SUCCESS"

    def __enter__(self):
        self.thread.start()
        return self

    def __exit__(self, *args):
        self.closed.set()
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()
