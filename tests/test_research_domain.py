"""Contract, event simulation and evidence-separation tests for invoice research."""

import copy
import importlib.util
import json
from pathlib import Path
import unittest

PATH = Path(__file__).resolve().parents[1] / "examples" / "research_domain.py"
SPEC = importlib.util.spec_from_file_location("research_domain", PATH)
domain = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(domain)


def tiny_benchmark(count=1):
    benchmark = domain.synthetic_benchmark()
    base = copy.deepcopy(benchmark["splits"]["development"][0])
    base.update({"arrival_minutes": 0, "deadline_minutes": 100, "risk_flag": False,
                 "audit_cohort": False, "missing_receipt": False,
                 "adjudication": {"defect": False, "precheck_resolves": False, "review_resolves": False}})
    base["observations"] = {"handling_minutes": 10, "review_minutes": 4, "precheck_minutes": 2,
                            "precheck_wait_minutes": 0, "late_chase_minutes": 0, "late_wait_minutes": 0,
                            "single_setup_minutes": 6, "paired_setup_minutes": 3,
                            "correction_minutes": 20, "discovery_delay_minutes": 90}
    for split in domain.SPLITS:
        benchmark["splits"][split] = []
        for index in range(count):
            row = copy.deepcopy(base)
            row["id"] = split + str(index)
            benchmark["splits"][split].append(row)
    return benchmark


def scenario(result, identity="normal"):
    return next(row for row in result["scenarios"] if row["id"] == identity)


class ResearchDomainTests(unittest.TestCase):
    def test_space_is_exact_and_physical_parameters_are_not_choices(self):
        configs = domain.all_configs()
        self.assertEqual(len(configs), 36)
        self.assertEqual(len({json.dumps(config, sort_keys=True) for config in configs}), 36)
        self.assertIn(domain.baseline_config(), configs)
        for name in ("handling_minutes", "review_minutes", "detection_rate", "staff", "ground_truth"):
            with self.assertRaises(ValueError):
                domain.candidate({**configs[0], name: 0})
        for bad in (None, {}, [], {**configs[0], "receipt_precheck": 1}, {**configs[0], "order": "oracle"}):
            with self.assertRaises(ValueError):
                domain.candidate(bad)
        schema = domain.candidate_schema()
        self.assertIs(schema["additionalProperties"], False)
        self.assertEqual(set(schema["properties"]), set(schema["required"]))

    def test_public_evidence_and_bindings_are_development_only(self):
        benchmark = domain.synthetic_benchmark()
        public = domain.public_evidence(benchmark)
        self.assertFalse(public["provenance"]["calibrated"])
        self.assertEqual(public["development_rows"], benchmark["splits"]["development"])
        text = json.dumps(public)
        for split in ("feedback-validation", "final-holdout"):
            for row in benchmark["splits"][split]:
                self.assertNotIn('"' + row["id"] + '"', text)
        public["development_rows"][0]["observations"]["handling_minutes"] = 100
        self.assertNotEqual(public["development_rows"], benchmark["splits"]["development"])
        changed = copy.deepcopy(benchmark)
        changed["splits"]["final-holdout"][0]["observations"]["handling_minutes"] += 1
        self.assertNotEqual(domain.benchmark_digest(changed), domain.benchmark_digest(benchmark))

    def test_frozen_rules_and_malformed_data_fail_closed(self):
        benchmark = domain.synthetic_benchmark()
        domain.validate_benchmark(benchmark)
        for key, value in [("staff", 99), ("relative_gates", {"min_mean_median_reduction_pct": 5})]:
            bad = copy.deepcopy(benchmark)
            bad["contract"][key] = value
            with self.assertRaises(ValueError):
                domain.validate_benchmark(bad)
        for value in (float("nan"), float("inf"), -1, True, 1e308):
            bad = copy.deepcopy(benchmark)
            bad["splits"]["development"][0]["observations"]["handling_minutes"] = value
            with self.assertRaises(ValueError):
                domain.validate_benchmark(bad)
        bad = copy.deepcopy(benchmark)
        bad["splits"]["final-holdout"][0]["id"] = bad["splits"]["development"][0]["id"]
        with self.assertRaises(ValueError):
            domain.validate_benchmark(bad)
        bad = copy.deepcopy(benchmark)
        bad["provenance"]["calibrated"] = True
        with self.assertRaises(ValueError):
            domain.validate_benchmark(bad)
        for value in (None, [], {}, {**benchmark, "known_best": domain.baseline_config()}):
            with self.assertRaises(ValueError):
                domain.validate_benchmark(value)

    def test_precheck_and_correction_use_fixed_labor_and_waits(self):
        benchmark = tiny_benchmark()
        for rows in benchmark["splits"].values():
            row = rows[0]
            row["missing_receipt"] = True
            row["adjudication"] = {"defect": True, "precheck_resolves": True, "review_resolves": False}
            row["observations"].update({"precheck_wait_minutes": 30, "late_chase_minutes": 5, "late_wait_minutes": 100})
        baseline = domain.evaluate(benchmark, domain.baseline_config(), "development")
        checked = domain.evaluate(benchmark, {**domain.baseline_config(), "receipt_precheck": True}, "development")
        self.assertEqual(scenario(baseline)["metrics"]["median_cycle_minutes"], 235)
        self.assertEqual(scenario(baseline)["metrics"]["labor_minutes"], 45)
        self.assertEqual(scenario(baseline)["metrics"]["rework_pct"], 100)
        self.assertEqual(scenario(checked)["metrics"]["median_cycle_minutes"], 52)
        self.assertEqual(scenario(checked)["metrics"]["labor_minutes"], 22)
        self.assertEqual(scenario(checked)["metrics"]["rework_pct"], 0)
        self.assertEqual(scenario(checked)["metrics"]["touches"], 2)

    def test_relative_gain_does_not_override_absolute_failure(self):
        benchmark = tiny_benchmark()
        for rows in benchmark["splits"].values():
            row = rows[0]
            row["deadline_minutes"] = 10
            row["missing_receipt"] = True
            row["observations"].update({"late_chase_minutes": 20, "late_wait_minutes": 100})
        result = domain.evaluate(benchmark, {**domain.baseline_config(), "receipt_precheck": True}, "development")
        self.assertTrue(result["relative_improved"])
        self.assertFalse(result["feasible"])
        self.assertFalse(result["eligible"])
        self.assertIn("service", scenario(result)["feasibility"]["violations"])

    def test_priority_selects_ready_work_and_does_not_preempt(self):
        benchmark = tiny_benchmark(2)
        for rows in benchmark["splits"].values():
            rows[1]["observations"].update({"handling_minutes": 2, "review_minutes": 1, "single_setup_minutes": 2})
        fifo = domain.evaluate(benchmark, domain.baseline_config(), "development")
        shortest_config = {**domain.baseline_config(), "order": "shortest"}
        shortest = domain.evaluate(benchmark, shortest_config, "development")
        self.assertEqual(scenario(fifo, "staff-absence")["metrics"]["median_cycle_minutes"], 22.5)
        self.assertEqual(scenario(shortest, "staff-absence")["metrics"]["median_cycle_minutes"], 15)
        for rows in benchmark["splits"].values():
            rows[1]["arrival_minutes"] = 1
        fifo = domain.evaluate(benchmark, domain.baseline_config(), "development")
        shortest = domain.evaluate(benchmark, shortest_config, "development")
        self.assertEqual(scenario(fifo, "staff-absence")["metrics"], scenario(shortest, "staff-absence")["metrics"])

    def test_batches_save_recorded_setup_but_charge_waiting(self):
        benchmark = tiny_benchmark(2)
        for rows in benchmark["splits"].values():
            rows[1]["arrival_minutes"] = 20
        single = domain.evaluate(benchmark, domain.baseline_config(), "development")
        pairs = domain.evaluate(benchmark, {**domain.baseline_config(), "batch": "pairs"}, "development")
        self.assertEqual(scenario(single)["metrics"]["labor_minutes"], 40)
        self.assertEqual(scenario(pairs)["metrics"]["labor_minutes"], 34)
        self.assertEqual(scenario(single)["metrics"]["median_cycle_minutes"], 20)
        self.assertEqual(scenario(pairs)["metrics"]["median_cycle_minutes"], 27)

    def test_every_split_has_stress_and_deterministic_complete_results(self):
        benchmark = domain.synthetic_benchmark()
        original = json.dumps(benchmark, sort_keys=True)
        for split in domain.SPLITS:
            for config in domain.all_configs():
                result = domain.evaluate(benchmark, config, split)
                self.assertEqual(result, domain.evaluate(benchmark, config, split))
                self.assertEqual({row["id"] for row in result["scenarios"]}, {"normal", "demand-40", "staff-absence"})
                self.assertFalse(result["claims_real_improvement"])
                self.assertEqual(result["eligible"], result["feasible"] and result["relative_improved"])
                self.assertEqual(scenario(result, "staff-absence")["metrics"]["staff"], 1)
                self.assertEqual(result["benchmark_sha256"], domain.benchmark_digest(benchmark))
        self.assertEqual(original, json.dumps(benchmark, sort_keys=True))
        baseline = domain.evaluate(benchmark, domain.baseline_config(), "development")
        self.assertFalse(baseline["relative_improved"])
        self.assertFalse(baseline["feasible"])
        self.assertLess(scenario(baseline, "staff-absence")["metrics"]["on_time_pct"], 95)

    def test_ranking_never_promotes_infeasibility_over_feasibility(self):
        result = domain.evaluate(domain.synthetic_benchmark(), domain.baseline_config(), "development")
        feasible = {**result, "eligible": False, "feasible": True,
                    "objective": {"mean_median_minutes": 100, "worst_p95_minutes": 100, "mean_labor_minutes": 100}}
        fast_failure = {**result, "eligible": False, "feasible": False,
                        "objective": {"mean_median_minutes": 1, "worst_p95_minutes": 1, "mean_labor_minutes": 1}}
        self.assertLess(domain.rank_key(feasible), domain.rank_key(fast_failure))


if __name__ == "__main__":
    unittest.main()
