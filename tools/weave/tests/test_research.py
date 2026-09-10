"""Research-controller invariants; real simulation and explicitly mocked plumbing."""
from collections import defaultdict
import copy
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import sys
import tempfile
import threading
import time
from types import SimpleNamespace
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "examples"))
import research
import research_domain as domain
import research_tasks
from protocol import digest, encoded, read


def options(**changes):
    values = dict(model="", mode="reference", arms=["search"], evaluations=2, trials=1,
                  team_size=3, seed=7301, seconds=120, dataset=None)
    values.update(changes)
    return SimpleNamespace(**values)


class ResearchTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="weave-research-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.benchmark = domain.synthetic_benchmark()
        self.dataset = self.root / "frozen-data.json"
        self.dataset.write_bytes(encoded(self.benchmark))
        self.binding = {"benchmark_path": str(self.dataset),
                        "benchmark_file_sha256": digest(encoded(self.benchmark)),
                        "benchmark_sha256": domain.benchmark_digest(self.benchmark)}
        self.baseline = research_tasks.execute(research.task("baseline", "evaluate",
                                              candidate=domain.baseline_config(), **self.binding), {})

    def test_deadline_dataset_and_allowances_are_frozen_on_resume(self):
        args = options()
        with mock.patch.object(research.recipe, "tool", return_value=sys.executable):
            _, first, _ = research.prepare(self.root, args)
            with mock.patch.object(research.time, "time", return_value=first["created_unix"] + 999):
                _, resumed, _ = research.prepare(self.root, args)
            self.assertEqual(first, resumed)
            for key, value in (("seconds", 999), ("evaluations", 3), ("seed", 1), ("team_size", 2)):
                changed = copy.copy(args)
                setattr(changed, key, value)
                with self.subTest(key=key), self.assertRaisesRegex(ValueError, "immutable input changed"):
                    research.prepare(self.root, changed)
            changed_data = copy.deepcopy(self.benchmark)
            changed_data["splits"]["final-holdout"][0]["observations"]["handling_minutes"] += 1
            self.dataset.write_bytes(encoded(changed_data))
            with self.assertRaisesRegex(ValueError, "immutable input changed"):
                research.prepare(self.root, options(dataset=self.dataset))

    def test_proposal_prompts_exclude_final_cases_and_local_evidence_paths(self):
        history = []
        for slot in range(1, 9):
            record = research.feedback_record(slot, self.baseline,
                       {"state": "accepted", "evidence": [{"kind": "test", "ref": "/private/never-send"}]})
            record["holdout"] = {"secret": "FINAL_SENTINEL"}
            history.append(record)
        plans = research.round_plan(self.root, self.binding, "team", 9, 3, [],
                                    domain.public_evidence(self.benchmark), self.baseline, history)
        proposals = [task for task in plans if research_tasks.agent_task(task)]
        self.assertEqual(len(proposals), 3)
        self.assertEqual(len({task["input"]["role"] for task in proposals}), 3)
        for task in proposals:
            prompt = research_tasks.prompt(task, {})
            self.assertLessEqual(len(prompt.encode()), 64 * 1024)
            self.assertNotIn("FINAL_SENTINEL", prompt)
            self.assertNotIn("/private/never-send", prompt)
            self.assertNotIn(str(self.dataset), prompt)
            for case in self.benchmark["splits"]["final-holdout"]:
                self.assertNotIn('"' + case["id"] + '"', prompt)

    def test_unknown_round_is_never_driven_or_retried(self):
        task = research.task("proposal-001", "review")
        tools = {name: "unused-" + name for name in ("weave", "tend", "ask", "ply")}
        observed = ([{"id": task["id"], "state": "unknown"}], {}, [])
        with mock.patch.object(research.recipe, "prepare_plan", return_value=([task], tools)), \
                mock.patch.object(research.recipe, "observe", return_value=observed), \
                mock.patch.object(research.recipe, "drive", side_effect=AssertionError("retried")):
            with self.assertRaisesRegex(RuntimeError, "unknown execution outcome"):
                research.run_round(self.root / "unknown", [task], options(mode="model", model="fixture"), time.time()+30)

    def test_rejected_proposal_blocked_evaluation_rederives_after_deadline(self):
        proposal = research.task("proposal-001", "review")
        evaluation = research.task("experiment-001", "evaluate", [proposal["id"]])
        tasks = [proposal, evaluation]
        tools = {name: "unused-" + name for name in ("weave", "tend", "ask", "ply")}
        observed = ([{"id": proposal["id"], "state": "rejected"}], {}, [])
        with mock.patch.object(research.recipe, "prepare_plan", return_value=(tasks, tools)), \
                mock.patch.object(research.recipe, "observe", return_value=observed), \
                mock.patch.object(research.recipe, "drive", side_effect=AssertionError("terminal slot rerun")):
            _, states, usage = research.run_round(self.root / "rejected", tasks,
                         options(mode="model", model="fixture"), time.time()-30)
        self.assertEqual(states[evaluation["id"]]["state"], "blocked")
        self.assertFalse(usage["complete"])
        self.assertEqual(usage["requests"], 0)

    def test_partial_answer_and_retry_remain_visible_in_usage(self):
        # Mocked verified Ask event snapshot exercises accounting only; it is
        # deliberately not evidence of an actual inference or accepted proposal.
        task = research.task("proposal-001", "review")
        path = self.root / "partial"
        session = path / "tasks" / research.recipe.job_id(task) / "session.jsonl"
        session.parent.mkdir(parents=True)
        session.write_bytes(b"fixture marker\n")
        events = [{"type": "request", "data": {}}, {"type": "retry", "data": {"attempt": 1}},
                  {"type": "assistant", "data": {"partial": True, "ms": 20,
                    "usage": {"in": 23, "out": 5, "reasoning": 2, "cache_read": 7}}}]
        tools = {name: "unused-" + name for name in ("weave", "tend", "ask", "ply")}
        observed = ([{"id": task["id"], "state": "rejected"}], {}, [])
        with mock.patch.object(research.recipe, "prepare_plan", return_value=([task], tools)), \
                mock.patch.object(research.recipe, "observe", return_value=observed), \
                mock.patch.object(research, "command", return_value=b"".join(encoded(e) for e in events)):
            _, _, usage = research.run_round(path, [task], options(mode="model", model="fixture"), time.time()+30)
        self.assertFalse(usage["complete"])
        self.assertEqual(usage["requests"], 1)
        self.assertEqual(usage["retries"], 1)
        self.assertEqual(usage["input_tokens"], 23)
        self.assertEqual(usage["output_tokens"], 5)

    def test_receipt_usage_requires_observed_counts_and_preserves_cache_writes(self):
        # These are accounting fixtures, not inference evidence. Missing or
        # malformed counters must not turn an unknown cost into reported zero.
        task = research.task("proposal-001", "review")
        path = self.root / "accounting"
        session = path / "tasks" / research.recipe.job_id(task) / "session.jsonl"
        session.parent.mkdir(parents=True)
        session.write_bytes(b"fixture marker\n")
        tools = {name: "unused-" + name for name in ("weave", "tend", "ask", "ply")}
        observed = ([{"id": task["id"], "state": "rejected"}], {}, [])
        cases = [
            ({"ms": 20, "usage": {"in": 23, "out": 5, "cache_write": 4}}, True, 23, 5, 4),
            ({"ms": 20}, False, 0, 0, 0),
            ({"usage": {"in": 23, "out": 5}}, False, 23, 5, 0),
            ({"ms": 20, "usage": {"in": True, "out": 5}}, False, 0, 5, 0),
            ({"ms": 20, "usage": {"in": 23, "out": -1}}, False, 23, 0, 0),
            ({"ms": 20, "usage": {"in": 0, "out": 0}}, False, 0, 0, 0),
            ({"ms": 20, "usage": {"in": 0, "out": 5, "cache_read": 23}}, True, 0, 5, 0),
            ({"ms": 20, "usage": {"in": 23, "out": 5, "cache_write": "unknown"}}, False, 23, 5, 0),
        ]
        for data, complete, inputs, outputs, writes in cases:
            events = [{"type": "request", "data": {}}, {"type": "assistant", "data": data}]
            with self.subTest(data=data), \
                    mock.patch.object(research.recipe, "prepare_plan", return_value=([task], tools)), \
                    mock.patch.object(research.recipe, "observe", return_value=observed), \
                    mock.patch.object(research, "command", return_value=b"".join(encoded(e) for e in events)):
                _, _, usage = research.run_round(path, [task], options(mode="model", model="fixture"), time.time()+30)
            self.assertIs(usage["complete"], complete)
            self.assertEqual(usage["input_tokens"], inputs)
            self.assertEqual(usage["output_tokens"], outputs)
            self.assertEqual(usage["cached_write_input_tokens"], writes)

    def test_fixture_plumbing_counts_failed_and_duplicate_slots_and_freezes_all_selections(self):
        # No model is called: fixture proposals exercise controller transitions;
        # deterministic evaluations still use the real research-domain scorer.
        args = options(arms=list(research.ARMS), mode="model", model="fixture", evaluations=4, trials=2)
        calls, confirmations = [], []
        def fixture_round(path, tasks, args, deadline):
            results, states = {}, {}
            proposals = sum(research_tasks.agent_task(task) for task in tasks)
            usage = defaultdict(int, complete=True, timing_complete=True, requests=proposals)
            for task in tasks:
                identity, kind = task["id"], task["input"]["kind"]
                if kind == "review":
                    calls.append((str(path), identity))
                    if task["input"]["slot"] == 1:
                        states[identity] = {"state": "rejected"}
                        continue
                    results[identity] = {"mode": "model", "candidate": domain.baseline_config(),
                                         "hypothesis": "Explicit fixture proposal, not model evidence.",
                                         "expected_tradeoff": "Repeated baseline tests duplicate accounting."}
                elif any(parent not in results for parent in task["needs"]):
                    states[identity] = {"state": "blocked"}
                    continue
                else:
                    if kind == "confirm":
                        frozen = json.loads(read(self.root / "selections.json"))
                        self.assertEqual(len(frozen["selections"]), len(args.arms)*args.trials)
                        self.assertEqual(len(calls), 2*args.trials*args.evaluations)
                        confirmations.append(identity)
                    results[identity] = research_tasks.execute(task, {p: results[p] for p in task["needs"]})
                states[identity] = {"state": "accepted", "evidence": [{"kind": "fixture", "ref": identity}]}
            return results, states, usage
        with mock.patch.object(research.recipe, "tool", return_value=sys.executable), \
                mock.patch.object(research, "run_round", side_effect=fixture_round):
            report = research.experiment(args, self.root)
        self.assertEqual(len(confirmations), len(args.arms)*args.trials)
        self.assertEqual(len(calls), 2*args.trials*args.evaluations)
        self.assertFalse(report["claims_live_business_improvement"])
        for summary in report["arms"]:
            self.assertEqual(summary["experiment_slots"], args.evaluations)
            if summary["arm"] != "search":
                self.assertEqual(summary["proposal_slots"], args.evaluations)
                self.assertEqual(summary["failed"], 1)
                self.assertEqual(summary["evaluated"], args.evaluations-1)
                self.assertEqual(summary["duplicates"], args.evaluations-2)

    def test_confirmation_requires_exact_frozen_selection_and_dataset(self):
        frozen = {"selections": [{"trial": 1, "arm": "search", "candidate": domain.baseline_config()}]}
        selections = self.root / "selections.json"
        selections.write_bytes(encoded(frozen))
        task = research.task("confirm", "confirm", candidate=domain.baseline_config(), trial=1, arm="search",
                  selection_path=str(selections), selection_sha256=digest(encoded(frozen)), **self.binding)
        result = research_tasks.execute(task, {})
        self.assertEqual(result["holdout"]["split"], "final-holdout")
        changed = copy.deepcopy(task)
        changed["input"]["candidate"] = next(c for c in domain.all_configs() if c != domain.baseline_config())
        with self.assertRaisesRegex(ValueError, "frozen arm selection"):
            research_tasks.execute(changed, {})
        selections.write_bytes(encoded({"selections": []}))
        with self.assertRaisesRegex(ValueError, "frozen selections changed"):
            research_tasks.execute(task, {})

    def test_real_reference_search_resume_has_no_new_tend_attempts(self):
        bins = Path(os.environ.get("WEAVE_TEST_BIN", ROOT / "var" / "bin"))
        env = os.environ.copy()
        for name in ("weave", "tend"):
            path = bins / name
            if not path.is_file():
                raise RuntimeError(f"build tools first with tests/check; missing {path}")
            env[name.upper()] = str(path)
        args = options()
        with mock.patch.dict(os.environ, env):
            report = research.experiment(args, self.root)
            round_roots = [p.parent for p in self.root.rglob("manifest.json")]
            def events():
                return {str(path): research.recipe.tend({"tend": env["TEND"]},
                        {**env, "TEND_ROOT": str(path / "tend")}, "events") for path in round_roots}
            before = events()
            manifest = json.loads(read(self.root / "experiment.json"))
            with mock.patch.object(research.time, "time", return_value=manifest["deadline_unix"]+1):
                resumed = research.experiment(args, self.root)
            self.assertEqual(events(), before)
            self.assertEqual(report, resumed)
        self.assertEqual(report["arms"][0]["evaluated"], args.evaluations)
        self.assertEqual(report["arms"][0]["usage"]["requests"], 0)
        self.assertIs(report["arms"][0]["usage"]["timing_complete"], True)

    def test_real_three_arm_controller_with_messages_fixture_and_rejected_proposal(self):
        # Actual Ask/Tend/Weave run every task. Only the provider's text is a
        # deterministic fixture; this proves plumbing, never agent performance.
        calls = []
        class Handler(BaseHTTPRequestHandler):
            def log_message(self, *args):
                pass

            def do_POST(self):
                request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
                payload = None
                for message in request["messages"]:
                    content = message["content"]
                    for part in ([{"text": content}] if isinstance(content, str) else content):
                        for line in part.get("text", "").splitlines():
                            try:
                                value = json.loads(line)
                            except ValueError:
                                continue
                            if isinstance(value, dict) and value.get("study") == "weave-invoice-research-v1":
                                payload = value
                if payload is None:
                    self.send_error(400, "missing fixture study payload")
                    return
                calls.append((payload["role"], payload["slot"]))
                answer = {"mode": "model", "candidate": payload["candidate_space"][payload["slot"]],
                          "hypothesis": "Explicit local fixture proposal, not actual reasoning.",
                          "expected_tradeoff": "The real evaluator will determine all process tradeoffs."}
                if payload["slot"] == 1 and "one researcher" in payload["role"]:
                    answer["hypothesis"] = "bad"  # Schema-valid, business checker rejects.
                output = encoded(answer).decode().strip()
                events = [
                    ("message_start", {"type": "message_start", "message": {"id": "fixture", "type": "message", "role": "assistant", "model": "fixture", "content": [], "stop_reason": None, "usage": {"input_tokens": 20, "output_tokens": 0}}}),
                    ("content_block_start", {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}),
                    ("content_block_delta", {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": output}}),
                    ("content_block_stop", {"type": "content_block_stop", "index": 0}),
                    ("message_delta", {"type": "message_delta", "delta": {"stop_reason": "end_turn", "stop_sequence": None}, "usage": {"output_tokens": 50}}),
                    ("message_stop", {"type": "message_stop"}),
                ]
                body = "".join(f"event: {kind}\ndata: {json.dumps(event)}\n\n" for kind, event in events).encode()
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

        bins = Path(os.environ.get("WEAVE_TEST_BIN", ROOT / "var" / "bin"))
        env = os.environ.copy()
        for name in ("weave", "tend", "ask", "ply"):
            path = bins / name
            if not path.is_file():
                raise RuntimeError(f"build tools first with tests/check; missing {path}")
            env[name.upper()] = str(path)
        server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        env.update({"ANTHROPIC_API_KEY": "offline-fixture-key",
                    "ANTHROPIC_BASE_URL": f"http://127.0.0.1:{server.server_port}",
                    "TEND_PASS": "ANTHROPIC_API_KEY ANTHROPIC_BASE_URL"})
        args = options(arms=list(research.ARMS), mode="model", model="anthropic/fixture")
        try:
            with mock.patch.dict(os.environ, env):
                report = research.experiment(args, self.root)
                before = list(calls)
                resumed = research.experiment(args, self.root)
                self.assertEqual(calls, before)
                self.assertEqual(report, resumed)
            self.assertEqual(len(calls), 4)
            arms = {arm["arm"]: arm for arm in report["arms"]}
            self.assertEqual(arms["single"]["failed"], 1)
            self.assertEqual(arms["single"]["evaluated"], 1)
            self.assertEqual(arms["team"]["evaluated"], 2)
            for name in ("single", "team"):
                self.assertEqual(arms[name]["usage"]["requests"], 2)
                self.assertEqual(arms[name]["usage"]["input_tokens"], 40)
                self.assertIs(arms[name]["usage"]["complete"], True)
                self.assertIs(arms[name]["usage"]["timing_complete"], True)
        finally:
            server.shutdown()
            server.server_close()
            thread.join()


if __name__ == "__main__":
    unittest.main()
