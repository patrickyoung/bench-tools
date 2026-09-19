import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("evaluate", Path(__file__).with_name("evaluate.py"))
evaluate = importlib.util.module_from_spec(spec)
spec.loader.exec_module(evaluate)


class EvaluationMetrics(unittest.TestCase):
    def test_known_stage_and_turn_costs_survive_incomplete_totals(self):
        stages = [{"backend": "weigh", "metadata": {"usage": {"cost": .03}}},
                  {"backend": "ask", "session": "/fixture/session"}]
        replay = b'\n'.join(json.dumps({"type": "assistant", "data": {"usage": usage}}).encode()
                            for usage in ({"cost": .02}, {"in": 100, "out": 10}))
        # Missing data before or after a known charge must not discard it.
        for ordered in (stages, list(reversed(stages))):
            with self.subTest(order=ordered), patch.object(evaluate.subprocess, "run",
                    return_value=SimpleNamespace(returncode=0, stdout=replay)) as run:
                cost = evaluate.reported_cost({"stages": ordered}, "fixture-ask")
                self.assertIsNone(cost["cost"])
                self.assertAlmostEqual(cost["cost_known_subtotal"], .05)
                run.assert_called_once()
                report = evaluate.summary([{"exit": 0, "expected": "accept", "seconds": 1, **cost}])
                self.assertIsNone(report["reported_cost_total"])
                self.assertAlmostEqual(report["reported_cost_known_subtotal"], .05)
                self.assertEqual(report["cost_unknown_cases"], 1)

    def test_complete_cost_and_failed_replay_keep_distinct_totals(self):
        stages = [{"backend": "weigh", "metadata": {"usage": {"cost": .03}}},
                  {"backend": "ask", "session": "/fixture/session"}]
        for exit_status, complete, known in ((0, .05, .05), (1, None, .03)):
            with self.subTest(exit=exit_status), patch.object(evaluate.subprocess, "run",
                    return_value=SimpleNamespace(returncode=exit_status,
                        stdout=b'{"type":"assistant","data":{"usage":{"cost":0.02}}}\n')):
                costs = evaluate.reported_cost({"stages": stages}, "fixture-ask")
                self.assertEqual(costs["cost"], complete)
                self.assertAlmostEqual(costs["cost_known_subtotal"], known)

    def test_errors_are_not_correct_rejections_and_unknown_cost_stays_unknown(self):
        rows = [
            {"exit": 0, "expected": "reject", "seconds": .1, "cost": .01},
            {"exit": 1, "expected": "accept", "seconds": .2, "cost": .02},
            {"exit": 2, "expected": "reject", "seconds": .3, "cost": None},
            {"exit": 0, "expected": "accept", "seconds": .4, "cost": .03, "escalated": True}]
        report = evaluate.summary(rows)
        self.assertEqual(report["false_passes"], 1)
        self.assertEqual(report["false_rejections"], 1)
        self.assertEqual(report["errors"], 1)
        self.assertEqual(report["completed"], 3)
        self.assertEqual(report["expected_reject_attempted"], 2)
        self.assertEqual(report["expected_reject_completed"], 1)
        self.assertEqual(report["expected_accept_attempted"], 2)
        self.assertEqual(report["expected_accept_completed"], 2)
        self.assertEqual(report["false_pass_rate"], 1)
        self.assertEqual(report["false_rejection_rate"], .5)
        self.assertEqual(report["escalations"], 1)
        self.assertIsNone(report["reported_cost_total"])
        self.assertAlmostEqual(report["reported_cost_known_subtotal"], .06)
        self.assertEqual(report["latency_p95_seconds"], .4)

    def test_unavailable_checker_has_no_measured_error_rates(self):
        rows = [{"exit": 2, "expected": label, "seconds": .1, "cost": None}
                for label in ("accept", "reject")]
        report = evaluate.summary(rows)
        self.assertEqual(report["errors"], 2)
        self.assertEqual(report["completed"], 0)
        self.assertIsNone(report["false_pass_rate"])
        self.assertIsNone(report["false_rejection_rate"])
        self.assertEqual(report["expected_accept_attempted"], 1)
        self.assertEqual(report["expected_reject_attempted"], 1)
        self.assertEqual(report["expected_accept_completed"], 0)
        self.assertEqual(report["expected_reject_completed"], 0)
        self.assertIsNone(report["reported_cost_total"])


class EvaluationCLI(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="semantic-evaluate-fixture-")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.cases_path = self.root / "cases.json"
        self.profiles_path = self.root / "profiles.json"
        self.rubric = self.root / "rubric.json"
        self.rubric.write_text('{"version":1,"criteria":[]}')
        self.output = self.root / "output"
        self.calls = self.root / "calls.jsonl"
        self.checker = self.root / "checker.py"
        self.checker.write_text('''import json, pathlib, sys
args = sys.argv[1:]
with open(%r, "a") as log:
    log.write(json.dumps(args) + "\\n")
candidate = pathlib.Path(args[args.index("--candidate") + 1]).read_text()
if candidate == "broken":
    raise SystemExit(2)
print(json.dumps({"stages": [], "judgments": {}}))
raise SystemExit(1 if candidate == "reject" else 0)
''' % str(self.calls))
        self.case = {"id": "good", "candidate": "good", "expected": "accept", "label_basis": "Independent fixture label"}
        self.cases = {"version": 1, "cases": [self.case]}
        self.ask = {"name": "ask", "backend": "ask", "model": "fixture/ask"}
        self.weigh = {"name": "weigh", "backend": "weigh", "model": "fixture/weigh", "accept_at": .8, "reject_at": .9}
        self.profiles = {"version": 1, "profiles": [self.ask]}
        self.env = {key: value for key, value in os.environ.items()
                    if key in ("PATH", "SYSTEMROOT", "TMPDIR")}
        self.env["PYTHONDONTWRITEBYTECODE"] = "1"

    def run_cli(self, cases=None, profiles=None, raw_cases=None, raw_profiles=None, extra=()):
        self.cases_path.write_text(raw_cases if raw_cases is not None else json.dumps(self.cases if cases is None else cases))
        self.profiles_path.write_text(raw_profiles if raw_profiles is not None else json.dumps(self.profiles if profiles is None else profiles))
        return subprocess.run([sys.executable, str(Path(evaluate.__file__)), "--live", "--cases", str(self.cases_path),
                               "--profiles", str(self.profiles_path), "--rubric", str(self.rubric),
                               "--checker", str(self.checker), "--output", str(self.output), *extra],
                              env=self.env, capture_output=True, timeout=10)

    def assert_preflight_rejection(self, result):
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(result.stdout, b"")
        self.assertNotIn(b"Traceback", result.stderr)
        self.assertFalse(self.calls.exists(), "an earlier profile spent work before invalid input was found")
        self.assertFalse(self.output.exists(), "invalid evaluation created a results directory")

    def test_invalid_later_profile_never_invokes_earlier_valid_profile(self):
        invalid = {"name": "incomplete", "backend": "weigh", "model": "fixture/weigh"}
        self.assert_preflight_rejection(self.run_cli(profiles={"version": 1, "profiles": [self.ask, invalid]}))

    def test_backend_specific_profile_errors_are_all_preflight(self):
        invalid = [
            {**self.weigh, "accept_at": True}, {**self.weigh, "reject_at": ".9"},
            {**self.weigh, "accept_at": .5}, {**self.weigh, "reject_at": 1.1},
            {**self.weigh, "accept_at": float("nan")}, {**self.weigh, "reject_at": float("inf")},
            {**self.weigh, "accept_at": 10 ** 400}, {**self.weigh, "fallback_model": " "},
            {**self.weigh, "fallback_model": 12}, {**self.weigh, "model": 12},
            {**self.weigh, "model": " "}, {**self.weigh, "name": []},
            {**self.weigh, "name": " "}, {**self.weigh, "backend": "automatic"},
            {**self.weigh, "extra": "unsupported"},
            {**self.ask, "name": "later", "accept_at": .8},
            {**self.ask, "name": "later", "fallback_model": "fixture/other"},
            {**self.ask, "name": "later", "model": "\ud800"},
            dict(self.ask),
        ]
        for profile in invalid:
            with self.subTest(profile=profile):
                self.assert_preflight_rejection(self.run_cli(profiles={"version": 1, "profiles": [self.ask, profile]}))

    def test_case_data_is_validated_before_any_profile_runs(self):
        invalid = [
            {**self.case, "id": []}, {**self.case, "id": " "},
            {**self.case, "label_basis": 1}, {**self.case, "label_basis": " "},
            {**self.case, "expected": "maybe"}, {**self.case, "candidate": None},
            {**self.case, "candidate": "\ud800"}, {**self.case, "extra": "unsupported"},
            {key: value for key, value in self.case.items() if key != "id"},
        ]
        for case in invalid:
            with self.subTest(case=case):
                self.assert_preflight_rejection(self.run_cli(cases={"version": 1, "cases": [case]}))
        self.assert_preflight_rejection(self.run_cli(cases={"version": 1, "cases": [self.case, self.case]}))

    def test_strict_documents_refuse_wrong_shapes_and_duplicate_keys(self):
        for cases in ([], {"version": True, "cases": [self.case]}, {"version": 1, "cases": []},
                      {"version": 1, "cases": {}}, {**self.cases, "label_source": " "}):
            with self.subTest(cases=cases):
                self.assert_preflight_rejection(self.run_cli(cases=cases))
        for profiles in ([], {"version": True, "profiles": [self.ask]}, {"version": 1, "profiles": []},
                         {"version": 1, "profiles": {}}, {**self.profiles, "extra": True}):
            with self.subTest(profiles=profiles):
                self.assert_preflight_rejection(self.run_cli(profiles=profiles))
        self.assert_preflight_rejection(self.run_cli(raw_profiles='{"version":1,"version":1,"profiles":[]}'))
        self.assert_preflight_rejection(self.run_cli(raw_cases='{"version":1,"cases":[],"cases":[]}'))

    def test_valid_profiles_run_and_retain_original_input_bytes(self):
        profiles = {"version": 1, "profiles": [self.ask, {**self.weigh, "fallback_model": "fixture/fallback"}]}
        result = self.run_cli(profiles=profiles)
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = [json.loads(line) for line in self.calls.read_text().splitlines()]
        self.assertEqual(len(calls), 2)
        self.assertNotIn("--accept-at", calls[0])
        self.assertEqual(calls[1][calls[1].index("--fallback-model") + 1], "fixture/fallback")
        self.assertEqual((self.output / "cases.json").read_bytes(), self.cases_path.read_bytes())
        self.assertEqual((self.output / "profiles.json").read_bytes(), self.profiles_path.read_bytes())
        report = json.loads((self.output / "summary.json").read_text())
        self.assertIn("fixed candidates", report["scope"])
        self.assertEqual(report["profiles"]["ask"]["expected_accept_completed"], 1)

    def test_checker_error_sets_aggregate_failure_without_improving_rates(self):
        cases = {"version": 1, "cases": [self.case, {"id": "unavailable", "candidate": "broken",
                 "expected": "reject", "label_basis": "Independent negative fixture"}]}
        result = self.run_cli(cases=cases)
        self.assertEqual(result.returncode, 2, result.stderr)
        report = json.loads((self.output / "summary.json").read_text())["profiles"]["ask"]
        self.assertEqual(report["errors"], 1)
        self.assertEqual(report["expected_reject_attempted"], 1)
        self.assertEqual(report["expected_reject_completed"], 0)
        self.assertIsNone(report["false_pass_rate"])
        self.assertIsNone(report["reported_cost_total"])
        self.assertEqual(len(self.calls.read_text().splitlines()), 2)

    def test_partial_cost_is_retained_in_case_rows_and_summary(self):
        result = {"stages": [{"backend": "weigh", "metadata": {"usage": {"cost": .03}}},
                             {"backend": "ask", "session": "/fixture/session"}], "escalated": True}
        self.checker.write_text("import json\nprint(json.dumps(" + repr(result) + "))\n")
        ask = self.root / "fixture-ask"
        ask.write_text('#!/usr/bin/env python3\nprint(\'{"type":"assistant","data":{"usage":{"in":100,"out":10}}}\')\n')
        ask.chmod(0o755)
        checked = self.run_cli(profiles={"version": 1, "profiles": [self.weigh]}, extra=("--ask", str(ask)))
        self.assertEqual(checked.returncode, 0, checked.stderr)
        row = json.loads((self.output / "results.jsonl").read_text())
        self.assertIsNone(row["cost"])
        self.assertAlmostEqual(row["cost_known_subtotal"], .03)
        report = json.loads((self.output / "summary.json").read_text())["profiles"]["weigh"]
        self.assertIsNone(report["reported_cost_total"])
        self.assertAlmostEqual(report["reported_cost_known_subtotal"], .03)
        self.assertEqual(report["cost_unknown_cases"], 1)


if __name__ == "__main__":
    unittest.main()
