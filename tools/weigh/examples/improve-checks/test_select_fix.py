#!/usr/bin/env python3
"""Offline selection contracts; these fixtures do not measure model quality."""
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from test_triage import FAKE


HERE = Path(__file__).resolve().parent


def snapshot():
    return {"version": 1, "id": "current", "task": "Report the due date only when the evidence supplies one.",
            "candidate": '{"due_date":"2030-04-05"}',
            "evidence": {"completion_date": "2030-04-05", "due_date": None},
            "check": {"verdict": "accept", "findings": ["The field has the expected type."]},
            "actions": {"repair": "Repair the artifact with the selected runner and present evidence.",
                        "leave": "Keep the artifact unchanged; the final independent check still applies.",
                        "inspect": "Inspect the check execution without changing the artifact or checker."}}


def native_result():
    return {"version": 1, "model": {"requested": "fixture/selector", "reported": "fixture/resolved"},
            "answers": {"fix": {"type": "choice", "value": "repair",
                                "probabilities": {"repair": .2, "leave": .7, "inspect": .1}}},
            "metadata": {"confidence": {"fix": .4}, "request_id": "fixture-request",
                         "usage": {"input_tokens": 12, "output_tokens": 3, "cost": .00001}}}


class SelectFixTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="select-fix-test-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.input, self.rules = self.root / "snapshot.json", self.root / "rules.json"
        self.input.write_text(json.dumps(snapshot(), indent=2))
        self.records, self.calls_path = self.root / "records", self.root / "calls.jsonl"
        self.executables = {}
        for role in ("ask", "weigh", "record"):
            executable = self.root / role
            executable.write_text("#!" + sys.executable + "\n" + FAKE)
            executable.chmod(0o700)
            self.executables[role] = executable
        self.weigh_response, self.ask_response = self.root / "weigh.json", self.root / "ask.json"
        self.weigh_response.write_text(json.dumps(native_result()))
        self.ask_response.write_text(json.dumps({"action": "repair"}))
        self.env = {key: os.environ[key] for key in ("PATH", "TMPDIR", "SYSTEMROOT") if key in os.environ}
        self.env.update(TRIAGE_TEST_CALLS=str(self.calls_path),
                        TRIAGE_TEST_WEIGH_RESPONSE=str(self.weigh_response),
                        TRIAGE_TEST_ASK_RESPONSE=str(self.ask_response), PYTHONDONTWRITEBYTECODE="1",
                        BENCH_WEIGH="1")

    def call(self, *, backend="weigh", live=False, rules=False, raw=None, extra=()):
        argv = [sys.executable, str(HERE / "select-fix.py"), "--records", str(self.records)]
        if backend:
            argv += ["--backend", backend, "--model", "fixture/selector"]
        if live:
            argv += ["--live"]
        if rules:
            argv += ["--rules", str(self.rules)]
        if raw is None:
            argv += ["--input", str(self.input)]
        for role, executable in self.executables.items():
            argv += ["--" + role, str(executable)]
        return subprocess.run(argv + list(extra), input=raw or b"", capture_output=True,
                              env=self.env, timeout=15)

    def calls(self, role=None):
        values = [json.loads(line) for line in self.calls_path.read_text().splitlines()] if self.calls_path.exists() else []
        return [value for value in values if role is None or value["role"] == role]

    def rule(self, action, **overrides):
        value = {"version": 1, "input_sha256": hashlib.sha256(self.input.read_bytes()).hexdigest(),
                 "action": action, "reason": "Caller inspected the exact structural condition."}
        value.update(overrides)
        self.rules.write_text(json.dumps(value))

    def broken(self, result):
        self.assertEqual(result.returncode, 2, result.stderr)
        self.assertEqual(result.stdout, b"")
        self.assertNotIn(b"Traceback", result.stderr)
        self.assertNotIn(b"SENTINEL", result.stderr)

    def test_rule_avoids_all_dependencies_and_requires_final_check(self):
        self.executables = {role: self.root / "not-installed" for role in self.executables}
        for action in ("repair", "inspect"):
            with self.subTest(action=action):
                value = snapshot()
                value["check"]["verdict"] = "error" if action == "inspect" else "reject"
                self.input.write_text(json.dumps(value))
                self.rule(action)
                result = self.call(backend=None, live=False, rules=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                output = json.loads(result.stdout)
                self.assertEqual(output["selector"], "rule")
                self.assertEqual(output["action"], action)
                self.assertTrue(output["requires_final_check"])
                self.assertIsNone(output["record"])
                self.assertIsNone(output["inference"])
                self.assertIsNone(output["model"])
                run = Path(output["records"])
                self.assertEqual((run / "rules.json").read_bytes(), self.rules.read_bytes())
                self.assertEqual(output["rules_sha256"], hashlib.sha256(self.rules.read_bytes()).hexdigest())
                self.assertEqual(run.stat().st_mode & 0o777, 0o700)
        self.assertEqual(self.calls(), [])

    def test_weigh_opt_in_precedes_all_dependencies_and_records(self):
        self.executables = {role: self.root / "not-installed" for role in self.executables}
        for value in (None, "", "0", "true", " 1"):
            with self.subTest(value=value):
                self.env.pop("BENCH_WEIGH", None)
                if value is not None:
                    self.env["BENCH_WEIGH"] = value
                result = self.call()
                self.broken(result)
                self.assertIn(b"BENCH_WEIGH", result.stderr)
                self.assertFalse(self.records.exists())
        self.assertEqual(self.calls(), [])

    def test_rules_and_ask_ignore_weigh_opt_in(self):
        for value in (None, "", "0", "invalid"):
            with self.subTest(value=value):
                self.env.pop("BENCH_WEIGH", None)
                if value is not None:
                    self.env["BENCH_WEIGH"] = value
                self.rule("repair")
                result = self.call(rules=True, live=False)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(json.loads(result.stdout)["selector"], "rule")
                result = self.call(backend="ask")
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(json.loads(result.stdout)["selector"], "ask")
        self.assertEqual(self.calls("weigh"), [])
        self.assertEqual(len(self.calls("ask")), 4)

    def test_null_rule_uses_one_model_but_does_not_send_rule_reason(self):
        self.rule(None, reason="Unselected private-rule-SENTINEL")
        result = self.call(rules=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        output = json.loads(result.stdout)
        self.assertEqual(output["selector"], "weigh")
        self.assertIsNone(output["reason"])
        self.assertIsNotNone(output["rules_sha256"])
        self.assertEqual(len(self.calls("weigh")), 1)
        self.assertNotIn("SENTINEL", self.calls("weigh")[0]["input"])

    def test_check_pass_is_not_an_automatic_no_change(self):
        result = self.call()
        self.assertEqual(result.returncode, 0, result.stderr)
        output = json.loads(result.stdout)
        self.assertEqual(output["action"], "repair")
        self.assertEqual(output["status"], "selected")
        self.assertTrue(output["requires_final_check"])
        self.assertEqual(output["inference"], native_result())
        self.assertEqual(output["input_sha256"], hashlib.sha256(self.input.read_bytes()).hexdigest())
        request = json.loads(self.calls("weigh")[0]["input"])
        self.assertEqual(request["state"], snapshot())
        self.assertEqual(request["questions"]["fix"]["options"], snapshot()["actions"])
        self.assertEqual([call["args"][0] for call in self.calls("record")], ["run", "check"])
        self.assertEqual(self.calls("ask"), [])

    def test_ask_only_stdin_path_has_same_state_and_no_invented_probabilities(self):
        self.executables["weigh"] = self.root / "not-installed"
        raw = json.dumps(snapshot(), indent=3).encode()
        result = self.call(backend="ask", raw=raw)
        self.assertEqual(result.returncode, 0, result.stderr)
        output = json.loads(result.stdout)
        self.assertEqual(output["inference"], {"action": "repair"})
        self.assertEqual(output["model"], "fixture/selector")
        self.assertEqual(output["input_sha256"], hashlib.sha256(raw).hexdigest())
        self.assertEqual(json.loads(self.calls("ask")[0]["input"])["state"], snapshot())
        run = Path(output["records"])
        self.assertEqual((run / "snapshot.json").read_bytes(), raw)
        schema = json.loads((run / "action.schema.json").read_bytes())
        self.assertEqual(set(schema["properties"]["action"]["enum"]), set(snapshot()["actions"]))
        self.assertEqual(self.calls("weigh"), [])

    def test_inference_must_be_explicit_and_invalid_inputs_have_no_effects(self):
        self.broken(self.call(backend=None))
        self.broken(self.call(backend=None, extra=("--backend", "weigh")))
        self.broken(self.call(backend=None, extra=("--model", "fixture/selector")))
        self.rule(None)
        self.broken(self.call(backend=None, rules=True))
        for extra in (("--timeout", "nan"), ("--timeout", "0")):
            self.broken(self.call(extra=extra))
        invalid = [b'{"version":1,"version":1}', b'{"private-SENTINEL":NaN}',
                   b'{"private-SENTINEL":"\ud800"}', b"[" * 66 + b"0" + b"]" * 66,
                   b" " * (4 * 1024 * 1024 + 1), json.dumps({**snapshot(), "unknown": 1}).encode()]
        for field, value in (("version", True), ("task", ""), ("evidence", None),
                             ("check", {"verdict": "accept"}), ("actions", {"repair": "Only choice"}),
                             ("actions", {"shell;run": "Bad ID", "leave": "Keep"})):
            invalid.append(json.dumps({**snapshot(), field: value}).encode())
        for raw in invalid:
            with self.subTest(raw=raw[:70]):
                self.broken(self.call(raw=raw))
        self.assertEqual(self.calls(), [])
        self.assertFalse(self.records.exists())

    def test_deprecated_live_flag_is_compatible_but_does_not_enable_weigh(self):
        result = self.call(live=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(json.loads(result.stdout)["selector"], "weigh")
        self.assertEqual(len(self.calls("weigh")), 1)
        self.env.pop("BENCH_WEIGH")
        self.broken(self.call(live=True))
        self.assertEqual(len(self.calls("weigh")), 1)

    def test_stale_unknown_or_malformed_rule_is_not_ignored_or_sent_to_a_model(self):
        for overrides in ({"input_sha256": "0" * 64}, {"action": "unavailable"},
                          {"version": True}, {"reason": ""}, {"action": []}, {"extra": 1}):
            self.rule("repair", **overrides) if "action" not in overrides else self.rule(**overrides)
            self.broken(self.call(rules=True))
        self.rules.write_text("null")
        self.broken(self.call(rules=True))
        self.assertEqual(self.calls(), [])
        self.assertFalse(self.records.exists())

    def test_provider_or_record_failure_emits_no_selection_and_never_falls_back(self):
        for name in ("TRIAGE_TEST_MODEL_EXIT", "TRIAGE_TEST_RUN_EXIT", "TRIAGE_TEST_CHECK_EXIT"):
            self.env[name] = "7"
            before = len(self.calls("weigh"))
            self.broken(self.call())
            self.assertLessEqual(len(self.calls("weigh")) - before, 1)
            self.assertEqual(self.calls("ask"), [])
            del self.env[name]
        self.assertEqual(list(self.records.glob("*/result.json")), [])

    def test_unavailable_choices_bad_envelopes_and_incomplete_distributions_are_broken(self):
        changes = [lambda x: x["answers"]["fix"].update(value="unavailable"),
                   lambda x: x["answers"]["fix"].update(probabilities={"repair": 1}),
                   lambda x: x["answers"]["fix"].update(probabilities={"repair": 0, "leave": 0, "inspect": 0}),
                   lambda x: x["answers"]["fix"].update(probabilities={"repair": True, "leave": 0, "inspect": 0}),
                   lambda x: x["answers"].update(other=x["answers"]["fix"]),
                   lambda x: x.update(model=None), lambda x: x["model"].update(requested="wrong"),
                   lambda x: x.update(version=True), lambda x: x.update(metadata=[])]
        for change in changes:
            value = copy.deepcopy(native_result())
            change(value)
            self.weigh_response.write_text(json.dumps(value))
            self.broken(self.call())
        for value in ({"action": "unavailable"}, {"action": ["repair"]}, {"action": "repair", "shell": "run"}):
            self.ask_response.write_text(json.dumps(value))
            self.broken(self.call(backend="ask"))
        self.assertEqual(list(self.records.glob("*/result.json")), [])

    def test_snapshot_and_rules_changes_during_inference_invalidate_selection(self):
        for target in (self.input, self.rules):
            self.input.write_text(json.dumps(snapshot()))
            self.rule(None)
            self.env["TRIAGE_TEST_MUTATE_PATH"] = str(target)
            self.broken(self.call(rules=True))
        self.assertEqual(list(self.records.glob("*/result.json")), [])

    def test_precise_evidence_literals_and_native_response_are_retained(self):
        raw = json.dumps(snapshot()).replace('"due_date": null', '"due_date": null, "exact": 9007199254740993, "decimal": 0.90000000000000001').encode()
        response = json.dumps(native_result()).replace('"cost": 1e-05', '"cost": 0.00001000000000000000001')
        self.weigh_response.write_text(response)
        result = self.call(raw=raw)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("9007199254740993", self.calls("weigh")[0]["input"])
        self.assertIn("0.90000000000000001", self.calls("weigh")[0]["input"])
        self.assertIn(b"0.00001000000000000000001", result.stdout)


if __name__ == "__main__":
    unittest.main()
