#!/usr/bin/env python3
"""Offline contract tests; no binaries, providers, credentials or paid calls."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import unittest


HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("improve_evaluate", HERE / "evaluate.py")
EVAL = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(EVAL)


def sha(text):
    return hashlib.sha256(text.encode()).hexdigest()


def fixture():
    request = {
        "version": 1,
        "criteria": [{"id": "content", "check": "semantic"}, {"id": "format", "check": "structural"}],
        "settings": [
            {"id": "before", "profile": "observed", "source_sha256": sha("checker-source"),
             "thresholds": {"content": .7, "format": 1}, "drop_checks": [], "drop_criteria": []},
            {"id": "after", "profile": "observed", "source_sha256": sha("checker-source"),
             "thresholds": {"content": .5, "format": 1}, "drop_checks": [], "drop_criteria": []},
        ],
        "baseline": "before", "proposal": "after", "proposal_basis": "calibration",
        "constraints": {"required_criteria": ["content", "format"],
                        "required_checks": ["semantic", "structural"],
                        "min_evaluation_cases": 2, "max_false_pass_rate": 0,
                        "max_false_rejection_rate": 0, "require_final_review": False},
        "cases": [],
        "sweeps": [{"criterion": "content", "values": [.4, .6, .8]}],
        "ablations": [{"criterion": "content"}, {"check": "structural"}],
    }
    for split in ("calibration", "evaluation"):
        for expected, score in (("accept", .6), ("reject", .2)):
            cid = split + "-" + expected
            request["cases"].append({
                "id": cid, "split": split, "candidate_sha256": sha(cid), "evidence_sha256": None,
                "expected": expected, "label_basis": "Independent reviewer assessed source evidence.",
                "label_independent": True,
                "profiles": {"observed": {"criteria": {"content": {"status": "known", "score": score},
                                                       "format": {"status": "known", "score": 1}}}},
            })
    return request


def add_repairs(request, qualities=(("reject", "accept"), ("accept", "accept"))):
    request["constraints"]["require_final_review"] = True
    request["repair_trials"] = []
    for index, pair in enumerate(qualities):
        trial = {"id": "trial-%s" % index, "split": "evaluation", "task_sha256": sha("task-%s" % index), "settings": {}}
        for setting, quality in zip(request["settings"], pair):
            trial["settings"][setting["id"]] = {
                "setting_sha256": EVAL.setting_hash(setting, request["criteria"]),
                "artifact_sha256": sha("artifact-%s-%s" % (index, setting["id"])),
                "checker_accepted": quality == "accept",
                "final_review": {"quality": quality, "basis": "Blind review against the task requirements.", "independent": True},
            }
        request["repair_trials"].append(trial)


class EvaluationTests(unittest.TestCase):
    def test_threshold_improvement_has_explicit_denominators_and_hashes(self):
        request = fixture()
        report = EVAL.evaluate(request)
        self.assertEqual(report["decision"], "supported")
        self.assertEqual(report["evaluation"]["baseline"]["false_rejections"], 1)
        self.assertEqual(report["evaluation"]["proposal"]["expected_accept_completed"], 1)
        self.assertEqual(report["evaluation"]["proposal"]["expected_reject_completed"], 1)
        self.assertEqual(report["evaluation"]["paired"]["corrected"], 1)
        self.assertIn("final-output improvement not established", report["scope"])
        self.assertNotEqual(report["hashes"]["settings"]["before"], report["hashes"]["settings"]["after"])
        self.assertEqual(report, EVAL.evaluate(copy.deepcopy(request)))

    def test_native_scores_are_neither_clamped_nor_normalized(self):
        request = fixture()
        for setting in request["settings"]:
            setting["thresholds"]["content"] *= 4
        for case in request["cases"]:
            case["profiles"]["observed"]["criteria"]["content"]["score"] *= 4
        self.assertEqual(EVAL.evaluate(request)["decision"], "supported")

    def test_calibration_variants_never_receive_evaluation_summaries(self):
        request = fixture()
        first = EVAL.evaluate(request)
        for case in request["cases"]:
            if case["split"] == "evaluation":
                case["profiles"]["observed"]["criteria"]["content"]["score"] = .99
        second = EVAL.evaluate(request)
        self.assertEqual(first["calibration"], second["calibration"])
        self.assertEqual(second["decision"], "blocked")
        self.assertNotIn("sweep-0001", second["evaluation"])
        variants = {c["id"]: c for c in first["calibration"]}
        self.assertIn("required_criterion_removed", variants["ablate-0001"]["coverage_violations"])
        self.assertIn("required_check_removed", variants["ablate-0002"]["coverage_violations"])

    def test_required_coverage_cannot_be_removed_to_pass(self):
        request = fixture()
        request["settings"][1]["drop_checks"] = ["semantic"]
        report = EVAL.evaluate(request)
        self.assertEqual(report["decision"], "blocked")
        self.assertIn("required_check_removed", report["reasons"])

    def test_frozen_threshold_floor_is_enforced(self):
        request = fixture()
        request["constraints"]["min_thresholds"] = {"content": .6}
        report = EVAL.evaluate(request)
        self.assertEqual(report["decision"], "blocked")
        self.assertIn("frozen_minimum_threshold_violated", report["reasons"])

    def test_missing_unknown_error_and_unknown_label_remain_distinct(self):
        request = fixture()
        # Keep two complete evaluation cases and add independent incomplete ones.
        for state in ("missing", "unknown", "error"):
            case = copy.deepcopy(request["cases"][-1])
            case.update(id=state, candidate_sha256=sha(state), expected="accept")
            case["profiles"]["observed"]["criteria"]["content"] = {"status": state}
            request["cases"].append(case)
        case = copy.deepcopy(request["cases"][2])
        case.update(id="unlabeled", candidate_sha256=sha("unlabeled"), expected="unknown")
        request["cases"].append(case)
        report = EVAL.evaluate(request)
        summary = report["evaluation"]["proposal"]
        self.assertEqual({s: summary["states"][s] for s in ("missing", "unknown", "error")},
                         {"missing": 1, "unknown": 1, "error": 1})
        self.assertEqual(summary["expected_accept_attempted"], 4)
        self.assertEqual(summary["expected_accept_completed"], 1)
        self.assertEqual(summary["label_unknown"], 1)
        self.assertEqual(report["decision"], "inconclusive")

    def test_error_is_not_hidden_by_another_rejected_criterion(self):
        request = fixture()
        case = request["cases"][-1]
        case["profiles"]["observed"]["criteria"]["format"] = {"status": "error"}
        self.assertEqual(EVAL.decide(case, request["settings"][0], request["criteria"]), "error")

    def test_omitted_profile_or_criterion_is_missing(self):
        request = fixture()
        request["cases"][2]["profiles"] = {}
        del request["cases"][3]["profiles"]["observed"]["criteria"]["format"]
        summary = EVAL.evaluate(request)["evaluation"]["proposal"]
        self.assertEqual(summary["states"]["missing"], 2)
        self.assertIsNone(summary["false_pass_rate"])
        self.assertIsNone(summary["false_rejection_rate"])

    def test_agreement_is_not_independent_ground_truth(self):
        request = fixture()
        for case in request["cases"]:
            case["label_independent"] = False
        report = EVAL.evaluate(request)
        self.assertEqual(report["decision"], "inconclusive")
        self.assertEqual(report["evaluation"]["proposal"]["independent_labeled"], 0)

    def test_final_output_regression_blocks_better_fixed_candidate_scores(self):
        request = fixture()
        add_repairs(request, (("accept", "reject"), ("accept", "accept")))
        report = EVAL.evaluate(request)
        self.assertEqual(report["decision"], "blocked")
        self.assertIn("fewer_fixed_candidate_errors", report["improvements"])
        self.assertIn("final_output_quality_regressed", report["reasons"])

    def test_fresh_independent_repair_improvement_supports_proposal(self):
        request = fixture()
        add_repairs(request)
        report = EVAL.evaluate(request)
        self.assertEqual(report["decision"], "supported")
        self.assertEqual(report["evaluation"]["repair_trials"]["improved"], 1)

    def test_unknown_missing_and_nonindependent_final_reviews_are_inconclusive(self):
        for mode in ("unknown", "missing", "not_independent", "absent_trials"):
            with self.subTest(mode=mode):
                request = fixture()
                add_repairs(request)
                result = request["repair_trials"][0]["settings"]["after"]
                if mode == "unknown":
                    result["final_review"]["quality"] = "unknown"
                elif mode == "missing":
                    del request["repair_trials"][0]["settings"]["after"]
                elif mode == "not_independent":
                    result["final_review"]["independent"] = False
                else:
                    del request["repair_trials"]
                self.assertEqual(EVAL.evaluate(request)["decision"], "inconclusive")

    def test_supplied_final_reviews_gate_even_if_constraint_optional(self):
        request = fixture()
        add_repairs(request, (("accept", "reject"), ("accept", "accept")))
        request["constraints"]["require_final_review"] = False
        self.assertEqual(EVAL.evaluate(request)["decision"], "blocked")

    def test_accepted_bad_final_output_increase_is_blocked(self):
        request = fixture()
        add_repairs(request, (("reject", "reject"), ("accept", "accept")))
        request["repair_trials"][0]["settings"]["after"]["checker_accepted"] = True
        report = EVAL.evaluate(request)
        self.assertEqual(report["decision"], "blocked")
        self.assertIn("accepted_bad_final_outputs_increased", report["reasons"])

    def test_old_repair_setting_hash_cannot_be_reused_after_threshold_change(self):
        request = fixture()
        add_repairs(request)
        request["settings"][1]["thresholds"]["content"] = .55
        with self.assertRaises(ValueError):
            EVAL.evaluate(request)

    def test_changed_source_cannot_reuse_the_same_observation_profile(self):
        request = fixture()
        request["settings"][1]["source_sha256"] = sha("different checker source")
        with self.assertRaises(ValueError):
            EVAL.evaluate(request)

    def test_unknown_cost_is_not_zero_and_partial_latency_is_identified(self):
        request = fixture()
        request["cases"][2]["profiles"]["observed"].update(cost=.01, seconds=2)
        report = EVAL.evaluate(request)
        summary = report["evaluation"]["proposal"]["usage"]
        self.assertEqual(summary["cost_known_subtotal"], .01)
        self.assertIsNone(summary["cost_total"])
        self.assertIsNone(summary["seconds_total"])
        self.assertEqual(summary["cost_unknown"], 1)
        self.assertEqual(summary["seconds_unknown"], 1)
        self.assertFalse(any(i.startswith("lower_observed") for i in report["improvements"]))

    @staticmethod
    def efficiency_fixture():
        request = fixture()
        request["settings"][0]["thresholds"]["content"] = .5
        request["settings"][1].update(profile="efficient", source_sha256=sha("efficient source"))
        for case in request["cases"]:
            case["profiles"]["efficient"] = copy.deepcopy(case["profiles"]["observed"])
        return request

    def test_equal_quality_fewer_tokens_or_turns_supports_with_tradeoff_visible(self):
        for metric in ("tokens", "turns"):
            with self.subTest(metric=metric):
                request = self.efficiency_fixture()
                for case in request["cases"]:
                    case["profiles"]["observed"].update({metric: 10, "seconds": 1})
                    case["profiles"]["efficient"].update({metric: 6, "seconds": 2})
                report = EVAL.evaluate(request)
                self.assertEqual(report["decision"], "supported")
                self.assertEqual(report["improvements"], ["lower_observed_" + metric + "_total"])
                before, after = (report["evaluation"][side] for side in ("baseline", "proposal"))
                self.assertEqual(before["false_passes"], after["false_passes"])
                self.assertEqual(before["false_rejections"], after["false_rejections"])
                self.assertEqual(before["usage"][metric + "_total"], 20)
                self.assertEqual(after["usage"][metric + "_total"], 12)
                self.assertEqual(after["usage"][metric + "_known_subtotal"], 12)
                self.assertEqual(after["usage"][metric + "_unknown"], 0)
                self.assertEqual(before["usage"]["seconds_total"], 2)
                self.assertEqual(after["usage"]["seconds_total"], 4)
                self.assertEqual(self.run_cli(request, "--gate").returncode, 0)

    def test_repair_efficiency_is_first_class_and_zero_turns_is_observed(self):
        request = self.efficiency_fixture()
        add_repairs(request, (("accept", "accept"), ("accept", "accept")))
        for trial in request["repair_trials"]:
            trial["settings"]["before"].update(tokens=1000, turns=2)
            trial["settings"]["after"].update(tokens=100, turns=0)
        report = EVAL.evaluate(request)
        self.assertEqual(report["decision"], "supported")
        self.assertEqual(report["improvements"], ["lower_observed_tokens_total", "lower_observed_turns_total"])
        repairs = report["evaluation"]["repair_trials"]
        self.assertEqual(repairs["proposal_usage"]["turns_total"], 0)
        self.assertEqual(repairs["proposal_usage"]["turns_unknown"], 0)
        self.assertEqual(repairs["proposal_usage"]["tokens_total"], 200)
        self.assertIsNone(report["evaluation"]["proposal"]["usage"]["tokens_total"])

    def test_missing_counter_measurements_do_not_invent_efficiency(self):
        for metric in ("tokens", "turns"):
            for mode in ("omitted", "null"):
                with self.subTest(metric=metric, mode=mode):
                    request = self.efficiency_fixture()
                    for case in request["cases"]:
                        case["profiles"]["observed"][metric] = 10
                        case["profiles"]["efficient"][metric] = 1
                    result = request["cases"][-1]["profiles"]["efficient"]
                    if mode == "null":
                        result[metric] = None
                    else:
                        del result[metric]
                    report = EVAL.evaluate(request)
                    self.assertEqual(report["decision"], "no_improvement")
                    usage = report["evaluation"]["proposal"]["usage"]
                    self.assertIsNone(usage[metric + "_total"])
                    self.assertEqual(usage[metric + "_known_subtotal"], 1)
                    self.assertEqual(usage[metric + "_unknown"], 1)
                    self.assertNotIn("lower_observed_" + metric + "_total", report["improvements"])

    def test_incomplete_paired_outcomes_do_not_claim_counter_efficiency(self):
        request = self.efficiency_fixture()
        for case in request["cases"]:
            case["profiles"]["observed"].update(tokens=100, turns=2)
            case["profiles"]["efficient"].update(tokens=10, turns=1)
        request["cases"][-1]["profiles"]["efficient"]["criteria"]["content"] = {"status": "error"}
        report = EVAL.evaluate(request)
        self.assertEqual(report["decision"], "blocked")
        self.assertNotIn("lower_observed_tokens_total", report["improvements"])
        self.assertNotIn("lower_observed_turns_total", report["improvements"])
        add_repairs(request, (("accept", "accept"), ("accept", "unknown")))
        for trial in request["repair_trials"]:
            trial["settings"]["before"].update(tokens=100, turns=2)
            trial["settings"]["after"].update(tokens=10, turns=1)
        report = EVAL.evaluate(request)
        self.assertNotIn("lower_observed_tokens_total", report["improvements"])
        self.assertNotIn("lower_observed_turns_total", report["improvements"])

    def test_lower_token_use_cannot_override_bad_final_quality(self):
        request = self.efficiency_fixture()
        add_repairs(request, (("accept", "reject"), ("accept", "accept")))
        for trial in request["repair_trials"]:
            trial["settings"]["before"].update(tokens=100, turns=2)
            trial["settings"]["after"].update(tokens=10, turns=1)
        report = EVAL.evaluate(request)
        self.assertEqual(report["decision"], "blocked")
        self.assertIn("final_output_quality_regressed", report["reasons"])
        self.assertIn("lower_observed_tokens_total", report["improvements"])
        self.assertIn("lower_observed_turns_total", report["improvements"])

    def test_counter_fields_are_bounded_nonnegative_integer_counts(self):
        for metric in ("tokens", "turns"):
            for location in ("profile", "repair"):
                for value in (-1, .5, 1.0, True, 10**12 + 1, "12"):
                    with self.subTest(metric=metric, location=location, value=value):
                        request = self.efficiency_fixture()
                        add_repairs(request)
                        result = (request["cases"][0]["profiles"]["observed"] if location == "profile" else
                                  request["repair_trials"][0]["settings"]["before"])
                        result[metric] = value
                        with self.assertRaises(ValueError):
                            EVAL.evaluate(request)
                request = self.efficiency_fixture()
                for case in request["cases"]:
                    case["profiles"]["observed"][metric] = 10**12
                self.assertEqual(EVAL.evaluate(request)["evaluation"]["baseline"]["usage"][metric + "_total"], 2 * 10**12)

    def test_counterfactual_ablation_reuses_observations_without_inventing_savings(self):
        request = self.efficiency_fixture()
        for case in request["cases"]:
            case["profiles"]["observed"].update(tokens=100, turns=2)
        calibration = {r["id"]: r for r in EVAL.evaluate(request)["calibration"]}
        for variant in ("sweep-0001", "ablate-0001", "ablate-0002"):
            self.assertEqual(calibration[variant]["summary"]["usage"], calibration["before"]["summary"]["usage"])

    def test_no_improvement_does_not_pass_gate(self):
        request = fixture()
        request["settings"][1]["thresholds"]["content"] = .7
        request["constraints"]["max_false_rejection_rate"] = 1
        self.assertEqual(EVAL.evaluate(request)["decision"], "no_improvement")
        result = self.run_cli(request, "--gate")
        self.assertEqual(result.returncode, 1)
        self.assertEqual(json.loads(result.stdout)["decision"], "no_improvement")

    def test_evaluation_split_cannot_be_declared_as_selection_basis(self):
        request = fixture()
        request["proposal_basis"] = "evaluation"
        with self.assertRaises(ValueError):
            EVAL.evaluate(request)

    def test_repeated_candidate_evidence_pair_cannot_cross_splits(self):
        request = fixture()
        request["cases"][2]["candidate_sha256"] = request["cases"][0]["candidate_sha256"]
        with self.assertRaises(ValueError):
            EVAL.evaluate(request)

    def test_repeated_repair_tasks_are_rejected(self):
        request = fixture()
        add_repairs(request)
        request["repair_trials"][1]["task_sha256"] = request["repair_trials"][0]["task_sha256"]
        with self.assertRaises(ValueError):
            EVAL.evaluate(request)

    def test_bounded_sweeps_fail_before_building_candidates(self):
        request = fixture()
        request["sweeps"][0]["values"] = [0] * 129
        with self.assertRaises(ValueError):
            EVAL.evaluate(request)

    def test_unsupported_and_nonfinite_or_ambiguous_json_is_rejected(self):
        invalid = [b'{"version":1,"version":1}', b'{"a":NaN}', b'{"a":1e999}',
                   b'{"a":"\\ud800"}', b'[' * 66 + b'0' + b']' * 66,
                   b' ' * (EVAL.MAX_BYTES + 1)]
        for raw in invalid:
            with self.subTest(raw=raw[:60]):
                result = self.run_raw(raw)
                self.assertEqual(result.returncode, 2)
                self.assertEqual(result.stdout, b"")
        for mutate in (lambda r: r.update(secret="must not echo"),
                       lambda r: r["settings"][1]["thresholds"].update(content=True),
                       lambda r: r["cases"][0]["profiles"]["observed"]["criteria"]["content"].update(status="error")):
            request = fixture()
            mutate(request)
            result = self.run_cli(request)
            self.assertEqual(result.returncode, 2)
            self.assertNotIn(b"must not echo", result.stderr)
            self.assertEqual(result.stdout, b"")

    def test_valid_cli_is_one_json_document_with_raw_input_hash(self):
        request = fixture()
        raw = json.dumps(request, indent=1).encode()
        result = self.run_raw(raw, "--gate")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stderr, b"")
        report = json.loads(result.stdout)
        self.assertEqual(report["hashes"]["request_sha256"], hashlib.sha256(raw).hexdigest())

    @staticmethod
    def run_cli(request, *args):
        return EvaluationTests.run_raw(json.dumps(request).encode(), *args)

    @staticmethod
    def run_raw(raw, *args):
        return subprocess.run([sys.executable, str(HERE / "evaluate.py"), "--input", "-", *args],
                              input=raw, capture_output=True, timeout=10)


if __name__ == "__main__":
    unittest.main()
