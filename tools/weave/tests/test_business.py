"""Business checks test evidence and causal effects, independent of execution."""

import copy
import importlib.util
import json
import pathlib
import unittest

SOURCE = pathlib.Path(__file__).resolve().parents[1] / "examples" / "business.py"
SPEC = importlib.util.spec_from_file_location("weave_business", SOURCE)
business = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(business)


def run(name):
    tasks, results = business.plans(name), {}
    for task in tasks:
        dependencies = {identity: results[identity] for identity in task["needs"]}
        result = business.execute(task, dependencies)
        business.check(task, result, dependencies)
        results[task["id"]] = result
    return {task["id"]: task for task in tasks}, results


class BusinessTests(unittest.TestCase):
    def test_review_schema_is_closed_required_and_matches_results(self):
        schema = business.review_schema()

        def inspect(node):
            self.assertIn(node["type"], {"object", "array", "string", "boolean"})
            if node["type"] == "object":
                self.assertIs(node.get("additionalProperties"), False)
                self.assertEqual(set(node["required"]), set(node["properties"]))
                self.assertEqual(len(node["required"]), len(set(node["required"])))
                for child in node["properties"].values():
                    inspect(child)
            elif node["type"] == "array":
                inspect(node["items"])

        def accepts(node, value):
            # Evaluate this deliberately small schema vocabulary without a new
            # test dependency. The real Ask integration also validates output.
            if "enum" in node and value not in node["enum"]:
                return False
            kind = node["type"]
            if kind == "object":
                return isinstance(value, dict) and set(node["required"]) <= set(value) and (
                    node.get("additionalProperties") is not False or set(value) <= set(node["properties"])) and all(
                    accepts(child, value[key]) for key, child in node["properties"].items() if key in value)
            if kind == "array":
                return isinstance(value, list) and all(accepts(node["items"], item) for item in value)
            return type(value) is {"string": str, "boolean": bool}[kind]

        inspect(schema)
        for task in business.plans("policy-review"):
            if task["input"]["kind"] != "review":
                continue
            result = business.execute(task, {})
            result["mode"] = "model"
            self.assertTrue(accepts(schema, result), task["id"])
            business.check(task, result, {})
            for key in result:
                missing = copy.deepcopy(result)
                del missing[key]
                self.assertFalse(accepts(schema, missing))
            for key in result["findings"][0]:
                missing = copy.deepcopy(result)
                del missing["findings"][0][key]
                self.assertFalse(accepts(schema, missing))
            for nested in (False, True):
                extra = copy.deepcopy(result)
                target = extra["findings"][0] if nested else extra
                target["unlisted"] = "field"
                self.assertFalse(accepts(schema, extra))
            for key, value in [("blocking", "true"), ("claim", []), ("cases", ["invented-case"])]:
                wrong = copy.deepcopy(result)
                wrong["findings"][0][key] = value
                self.assertFalse(accepts(schema, wrong))

    def test_both_complete_and_reference_results_are_labelled(self):
        for name in business.NAMES:
            tasks, results = run(name)
            for identity, task in tasks.items():
                self.assertEqual(results[identity]["mode"], "reference-fixture" if
                                 business.agent_task(task) else "deterministic-simulation")

    def test_supported_singleton_survives_synthesis(self):
        tasks, results = run("policy-review")
        reviews = {key: value for key, value in results.items() if key.startswith("review-")}
        blockers = [finding["id"] for review in reviews.values() for finding in review["findings"]
                    if finding["blocking"]]
        self.assertEqual(len(blockers), 1)
        self.assertIn(blockers[0], results["synthesis"]["finding_ids"])
        bad = copy.deepcopy(results["synthesis"])
        bad["finding_ids"].remove(blockers[0])
        with self.assertRaisesRegex(ValueError, "rare objection"):
            business.check(tasks["synthesis"], bad, reviews)
        # Evidence-linked checks do not require the reference prose verbatim.
        changed = copy.deepcopy(results["synthesis"])
        changed["recommendation"] = "Require cumulative authorization and documented urgent handling before adoption."
        business.check(tasks["synthesis"], changed, reviews)

    def test_bad_source_and_conflicting_tradeoff_references_rejected(self):
        tasks, results = run("policy-review")
        bad = copy.deepcopy(results["review-controls"])
        bad["findings"][0]["cases"] = ["invented-case"]
        with self.assertRaises(ValueError):
            business.check(tasks["review-controls"], bad, {})
        changed_source = copy.deepcopy(tasks["review-controls"])
        changed_source["input"]["evidence"]["cases"][1]["committed"] = 1000
        with self.assertRaisesRegex(ValueError, "contradicts source"):
            business.check(changed_source, results["review-controls"], {})
        bad = copy.deepcopy(results["synthesis"])
        bad["contradictions"] = []
        with self.assertRaisesRegex(ValueError, "tradeoff"):
            business.check(tasks["synthesis"], bad, {key: results[key] for key in tasks["synthesis"]["needs"]})

    def test_design_parameters_change_simulated_outcomes(self):
        tasks, results = run("invoice-process")
        task = tasks["simulate-routing"]
        dependencies = {key: copy.deepcopy(results[key]) for key in task["needs"]}
        before = business.execute(task, dependencies)
        dependencies["method-routing"]["parameters"]["staff"] = 1
        after = business.execute(task, dependencies)
        self.assertNotEqual(before["scenarios"][0]["metrics"], after["scenarios"][0]["metrics"])
        self.assertGreater(after["scenarios"][0]["metrics"]["p95_minutes"],
                           before["scenarios"][0]["metrics"]["p95_minutes"])
        business.check(task, after, dependencies)

    def test_demand_capacity_and_heldout_evidence(self):
        tasks, results = run("invoice-process")
        scenarios = {s["scenario"]: s for s in results["simulate-routing"]["scenarios"]}
        self.assertEqual(set(scenarios), {"development", "demand-40", "staff-loss", "heldout", "heldout-combined"})
        self.assertGreater(scenarios["demand-40"]["baseline"]["p95_minutes"],
                           scenarios["development"]["baseline"]["p95_minutes"])
        self.assertGreater(scenarios["staff-loss"]["baseline"]["p95_minutes"],
                           scenarios["development"]["baseline"]["p95_minutes"])
        self.assertTrue(set(scenarios["heldout"]["case_ids"]).isdisjoint(scenarios["development"]["case_ids"]))
        prompt = business.prompt(tasks["method-routing"], {"baseline": results["baseline"]})
        self.assertNotIn('"H01"', prompt)
        self.assertIn('"D01"', prompt)

    def test_negative_experiment_is_valid_completed_work(self):
        tasks, results = run("invoice-process")
        outcomes = {row["method_id"]: row["outcome"] for row in results["comparison"]["methods"]}
        self.assertEqual(outcomes["method-risk"], "not_improved")
        self.assertGreater(results["simulate-risk"]["scenarios"][0]["metrics"]["rework_pct"], 0)
        self.assertFalse(results["pilot"]["claims_live_improvement"])
        dependencies = {key: copy.deepcopy(results[key]) for key in tasks["comparison"]["needs"]}
        # A valid all-negative comparison still produces a checked no-pilot result.
        for result in dependencies.values():
            for scenario in result["scenarios"]:
                scenario["metrics"] = copy.deepcopy(scenario["baseline"])
        comparison = business.execute(tasks["comparison"], dependencies)
        business.check(tasks["comparison"], comparison, dependencies)
        self.assertIsNone(comparison["recommended"])
        pilot = business.execute(tasks["pilot"], {"comparison": comparison})
        self.assertEqual(pilot["decision"], "no-pilot")
        business.check(tasks["pilot"], pilot, {"comparison": comparison})

    def test_numeric_and_shape_corruption_fail(self):
        tasks, results = run("invoice-process")
        task = tasks["simulate-routing"]
        dependencies = {key: results[key] for key in task["needs"]}
        for value in [999, "19", True, float("nan")]:
            bad = copy.deepcopy(results[task["id"]])
            bad["scenarios"][0]["metrics"]["median_minutes"] = value
            with self.assertRaises(ValueError):
                business.check(task, bad, dependencies)
        method = tasks["method-routing"]
        for bad in [None, [], {"mode": []}, {"mode": "model", "parameters": {}}]:
            with self.assertRaises(ValueError):
                business.check(method, bad, {"baseline": results["baseline"]})
        bad = copy.deepcopy(results["pilot"])
        bad["basis"][0]["metrics"]["labor_minutes"] += 1
        with self.assertRaisesRegex(ValueError, "numeric evidence"):
            business.check(tasks["pilot"], bad, {"comparison": results["comparison"]})
        bad = copy.deepcopy(results["pilot"])
        bad["stop_conditions"]["max_rework_pct"] = False
        with self.assertRaisesRegex(ValueError, "thresholds"):
            business.check(tasks["pilot"], bad, {"comparison": results["comparison"]})
        equivalent = copy.deepcopy(results["pilot"])
        for evidence in equivalent["basis"]:
            for key, value in evidence["metrics"].items():
                if isinstance(value, float) and value.is_integer():
                    evidence["metrics"][key] = int(value)
        business.check(tasks["pilot"], equivalent, {"comparison": results["comparison"]})

    def test_prompts_are_json_explicit_and_do_not_mutate_inputs(self):
        for name in business.NAMES:
            tasks, results = run(name)
            for identity, task in tasks.items():
                if business.agent_task(task):
                    dependencies = {key: results[key] for key in task["needs"]}
                    original = json.dumps({"task": task, "deps": dependencies}, sort_keys=True)
                    text = business.prompt(task, dependencies)
                    self.assertIn("exactly one JSON object", text)
                    self.assertIn('"mode": "model"', text)
                    self.assertEqual(original, json.dumps({"task": task, "deps": dependencies}, sort_keys=True))


if __name__ == "__main__":
    unittest.main()
