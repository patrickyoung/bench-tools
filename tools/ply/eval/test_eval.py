"""No credentials or network services are needed for harness self-tests."""
import contextlib
import io
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import subprocess
import signal
import time
import unittest
import sys

import run as harness

spec = importlib.util.spec_from_file_location("ply_driver", harness.HERE / "drivers" / "ply.py")
ply_driver = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ply_driver)


class HarnessTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="ply-eval-test-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)

    def config(self, *behaviors):
        path = self.root / "drivers.json"
        path.write_text(json.dumps([{"name": f"driver-{n}", "argv": [sys.executable, str(harness.HERE / "drivers" / "fake.py")],
                                    "model": "deterministic-selftest", "suites": ["task"],
                                    "budget": {"wall_seconds": 5}, "options": {"behavior": behavior}}
                                   for n, behavior in enumerate(behaviors)]))
        return path

    def execute(self, config, cases, repetitions=1):
        with contextlib.redirect_stderr(io.StringIO()):
            summary = harness.run(config, self.root / "results", repetitions, cases)
        rows = [json.loads(line) for line in (self.root / "results" / "results.jsonl").read_text().splitlines()]
        return summary, rows

    def test_repeated_pairs_use_identical_fixtures_isolated_state_and_external_oracles(self):
        summary, rows = self.execute(self.config("correct", "liar"),
                                     ["multi-file-repair", "ambiguous-request", "large-output"], repetitions=2)
        self.assertEqual(len(rows), 12)
        self.assertEqual(sum(r["false_success"] is True for r in rows), 6)
        self.assertTrue(all(r["oracle"]["passed"] for r in rows if r["driver"] == "driver-0"))
        self.assertTrue(all(r["integrity_verified"] for r in rows))
        self.assertTrue(all(r["usage"] is None for r in rows))
        self.assertTrue(all(p["left_only"] == 2 and p["unpaired"] == 0 for p in summary["pairs"]))
        self.assertIsNone(summary["cases"]["large-output"]["driver-0"]["usage"]["cost_usd"]["sum"])
        for case in summary["cases"]:
            self.assertEqual(len({r["initial_sha256"] for r in rows if r["case"] == case}), 1)
        # Rotating order prevents one driver always receiving the first slot.
        first_pair = [r["driver"] for r in rows if r["case"] == "multi-file-repair" and r["repetition"] == 1]
        second_pair = [r["driver"] for r in rows if r["case"] == "multi-file-repair" and r["repetition"] == 2]
        self.assertEqual(first_pair, list(reversed(second_pair)))

    def test_oracle_tamper_cannot_produce_measured_success(self):
        _, rows = self.execute(self.config("tamper"), ["large-output"])
        self.assertEqual(rows[0]["status"], "invalid")
        self.assertFalse(rows[0]["integrity_verified"])
        self.assertIsNone(rows[0]["oracle"]["passed"])

    def test_malformed_result_is_invalid_even_when_exit_zero(self):
        _, rows = self.execute(self.config("malformed"), ["large-output"])
        self.assertEqual(rows[0]["status"], "invalid")
        self.assertEqual(rows[0]["attempts"][0]["exit_code"], 0)

    def test_timeout_is_bounded_and_does_not_count_as_a_paired_loss(self):
        config = self.config("timeout", "correct")
        drivers = json.loads(config.read_text())
        drivers[0]["budget"]["wall_seconds"] = 0.1
        config.write_text(json.dumps(drivers))
        summary, rows = self.execute(config, ["large-output"])
        self.assertEqual(rows[0]["attempts"][0]["stop"], "timeout")
        self.assertLess(rows[0]["elapsed_seconds"], 5)
        self.assertEqual(summary["pairs"][0]["unpaired"], 1)

    def test_unsupported_lifecycle_is_explicit_and_unpaired(self):
        summary, rows = self.execute(self.config("correct", "liar"), ["repeated-compaction"])
        self.assertTrue(all(r["status"] == "unsupported" for r in rows))
        self.assertEqual(summary["pairs"][0]["unpaired"], 1)

    def test_expired_budget_does_not_launch_a_driver(self):
        driver = harness.load_drivers(self.config("correct"))[0]
        directory = self.root / "attempt"
        directory.mkdir()
        work, state = directory / "work", directory / "state"
        work.mkdir()
        state.mkdir()
        request = {"resume": True, "workdir": str(work), "state_dir": str(state)}
        result = harness.attempt(driver, request, directory, None, False, time.monotonic() - 1)
        self.assertEqual(result["stop"], "timeout")
        self.assertIsNone(result["exit_code"])
        self.assertFalse((state / "used").exists())

    def test_normal_exit_with_surviving_descendants_is_invalid_and_cleaned(self):
        _, rows = self.execute(self.config("orphan"), ["large-output"])
        self.assertEqual(rows[0]["status"], "invalid")
        self.assertEqual(rows[0]["attempts"][0]["stop"], "orphaned_descendants")
        ticks = self.root / "results/trials/large-output/1/driver-0/work/ticks"
        if ticks.exists():
            size = ticks.stat().st_size
            time.sleep(.15)
            self.assertEqual(ticks.stat().st_size, size)

    def test_oracle_child_with_inherited_output_cannot_outlive_timeout(self):
        directory = self.root / "oracle"
        directory.mkdir()
        script = 'import subprocess; subprocess.Popen(["/bin/sh","-c", "trap \'\' INT TERM; sleep 60"]); print(\'{"passed":true,"detail":"forged"}\')'
        started = time.monotonic()
        result = harness.evaluate_oracle([sys.executable, "-c", script], directory, timeout=.1)
        self.assertIsNone(result["passed"])
        self.assertIn(result["detail"], ("oracle orphaned_descendants", "oracle timeout"))
        self.assertLess(time.monotonic() - started, 5)

    def test_unknown_usage_is_not_zero_and_impossible_usage_is_refused(self):
        result = {"answer": "", "claimed_success": None, "usage": None}
        self.assertIsNone(harness.validate_result(result)["usage"])
        for usage in ({"input_tokens": -1, "source": "driver"}, {"cost_usd": float("nan"), "source": "driver"}, {"input_tokens": 1}):
            with self.assertRaises(ValueError):
                harness.validate_result({**result, "usage": usage})

    def test_existing_output_directory_is_never_retried(self):
        config = self.config("correct")
        self.execute(config, ["large-output"])
        with self.assertRaises(FileExistsError):
            harness.run(config, self.root / "results", 1, ["large-output"])

    def test_cumulative_usage_counts_compaction_and_cancelled_turns_once(self):
        def session(name, incoming, outgoing, cost=None, **extra):
            return [{"type": "session", "data": {"id": name, **extra}},
                    {"type": "assistant", "seq": 4, "data": {"usage": {"in": incoming, "out": outgoing, "cost": cost}, "partial": name == "cancelled"}}]
        original = session("original", 100, 50, .3)
        summary = session("summary", 30, 10, .1)
        cancelled = session("cancelled", 20, 4, .02)
        resumed = session("resumed", 10, 5, .01, summary="summary", parent="original")
        usage = ply_driver.aggregate_usage([original, summary, cancelled, resumed, original])
        self.assertEqual(usage["input_tokens"], 160)
        self.assertEqual(usage["output_tokens"], 69)
        self.assertEqual(usage["observed_turns"], 4)
        self.assertEqual(usage["summary_turns"], 1)
        self.assertEqual(usage["partial_turns"], 1)
        self.assertIsNone(usage["cached_input_tokens"])
        self.assertIsNone(usage["observed_cached_input_tokens"])
        self.assertIsNone(usage["cache_write_tokens"])
        self.assertIsNone(usage["reasoning_tokens"])
        self.assertAlmostEqual(usage["cost_usd"], .43)
        self.assertIsNone(ply_driver.aggregate_usage([original, session("unknown-cost", 2, 1)])["cost_usd"])
        unknown = ply_driver.aggregate_usage([original, session("unknown-usage", 0, 0)])
        self.assertIsNone(unknown["input_tokens"])
        self.assertEqual(unknown["unreported_turns"], 1)
        missing = [{"type": "session", "data": {"id": "failed-before-response"}}, {"type": "request", "seq": 3, "data": {}}]
        # Include one request per completed response, then one with no response.
        original.insert(1, {"type": "request", "seq": 3, "data": {}})
        unknown = ply_driver.aggregate_usage([original, missing])
        self.assertIsNone(unknown["input_tokens"])
        self.assertIsNone(unknown["cost_usd"])
        self.assertEqual(unknown["unreported_requests"], 1)
        original.insert(2, {"type": "retry", "seq": 5, "data": {"status": 503}})
        retry = ply_driver.aggregate_usage([original])
        self.assertIsNone(retry["input_tokens"])
        self.assertIsNone(retry["cost_usd"])
        self.assertEqual(retry["observed_input_tokens"], 100)
        self.assertEqual(retry["observed_cost_usd"], .3)
        self.assertEqual(retry["unreported_retries"], 1)

    def test_group_escalation_outlives_exited_leader_and_redirected_output(self):
        ticks = self.root / "ticks"
        script = '(trap "" INT TERM; while :; do printf x >> "$1"; sleep .03; done) >/dev/null 2>&1 & exit 0'
        process = subprocess.Popen(["/bin/sh", "-c", script, "fixture", str(ticks)], start_new_session=True)
        try:
            process.wait(timeout=2)
            deadline = time.monotonic() + 2
            while not ticks.exists() and time.monotonic() < deadline:
                time.sleep(.01)
            self.assertTrue(ticks.exists())
            harness.terminate(process)
            after = ticks.stat().st_size
            time.sleep(.15)
            self.assertEqual(ticks.stat().st_size, after, "redirected descendant survived group escalation")
        finally:
            try:
                os.killpg(process.pid, signal.SIGKILL)
            except ProcessLookupError:
                pass


if __name__ == "__main__":
    unittest.main()
