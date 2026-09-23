#!/usr/bin/env python3
"""Synthetic, offline contract tests; never run application code."""
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

CHECK = Path(__file__).resolve().parents[1] / "expert" / "bin" / "check"


class ContractTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="django-contract-")
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.write("request.md", "# Synthetic plan\nMode: plan\nSelected inputs: inputs/brief.md\n")
        self.write("inputs/brief.md", "Synthetic task: propose a small staff workflow.\n")
        self.write("report.md", "# Plan\nImplementation not run. Review the proposed workflow.\n")
        self.write("evidence/check.txt", "Synthetic evidence only; not a real application run.\n")
        self.data = {
            "schema_version": 1, "mode": "plan", "status": "ready_for_review",
            "summary": "Synthetic plan for structural validation only.",
            "application": None,
            "request": self.binding("request.md"),
            "inputs": [self.binding("inputs/brief.md")],
            "artifacts": [self.binding("report.md")],
            "environments": {
                name: {
                    "database": f"Separate PostgreSQL role/data for {name}; patch unresolved.",
                    "authentication": (
                        "Explicit local synthetic password option."
                        if name == "local" else "OIDC with PKCE; owner must supply configuration."
                    ),
                    "deployment": "Loopback only." if name == "local" else
                                  "HTTPS and DEBUG false; no provisioning performed.",
                } for name in ("local", "DevTest", "UAT", "production")
            },
            "validations": [],
            "limitations": ["No application, browser or IdP tests run."],
            "next_actions": ["Product owner reviews plan before implementation."],
            "blockers": [],
        }

    def write(self, name, content):
        path = self.work / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding="utf-8")

    def binding(self, name):
        return {"path": name, "sha256": hashlib.sha256((self.work / name).read_bytes()).hexdigest()}

    def run_check(self, expected=0, data=None, raw=None):
        if raw is None:
            raw = json.dumps(self.data if data is None else data)
        self.write("handoff.json", raw)
        result = subprocess.run(
            [str(CHECK)], cwd=self.work, capture_output=True, text=True, timeout=10,
            env={**os.environ, "PYTHONDONTWRITEBYTECODE": "1"},
        )
        self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
        if expected == 1:
            self.assertIn("unfinished:", result.stderr)
            self.assertNotIn("Traceback", result.stderr)
        return result

    def evidence_validation(self, result="passed"):
        self.data["artifacts"].append(self.binding("evidence/check.txt"))
        self.data["validations"] = [{
            "command": "synthetic-only-check", "result": result, "evidence": "evidence/check.txt",
        }]

    def block(self):
        self.data.update(status="blocked", blockers=["UAT issuer and client assignment missing."])
        self.data["validations"] = [{
            "command": "OIDC integration check", "result": "blocked", "evidence": None,
        }]

    def test_valid_plan(self):
        self.assertIn("ready_for_review", self.run_check().stdout)

    def test_valid_blocked(self):
        self.block()
        self.assertIn("structurally valid: blocked", self.run_check().stdout)

    def test_valid_evidence_does_not_execute_commands(self):
        self.evidence_validation()
        self.data["validations"][0]["command"] = "touch MUST_NOT_EXIST"
        self.run_check()
        self.assertFalse((self.work / "MUST_NOT_EXIST").exists())

    def test_valid_application_and_modes(self):
        self.write("app/artifact.txt", "Synthetic artifact, not an application.\n")
        self.data["artifacts"].append(self.binding("app/artifact.txt"))
        for mode in ("plan", "build", "iterate", "review_debug"):
            with self.subTest(mode=mode):
                self.data.update(mode=mode, application="app")
                self.run_check()

    def test_missing_report(self):
        (self.work / "report.md").unlink()
        self.run_check(1)

    def test_missing_declared_output(self):
        self.data["artifacts"].append({"path": "app/missing.py", "sha256": "0" * 64})
        self.run_check(1)

    def test_stale_request(self):
        self.write("request.md", "Entirely new task")
        self.run_check(1)

    def test_stale_input(self):
        self.write("inputs/brief.md", "Changed dependency")
        self.run_check(1)

    def test_tampered_artifact(self):
        self.write("report.md", "Changed report")
        self.run_check(1)

    def test_invalid_environments(self):
        for key in ("local", "DevTest", "UAT", "production"):
            with self.subTest(key=key):
                data = copy.deepcopy(self.data)
                data["environments"][key.upper() + "_wrong"] = data["environments"].pop(key)
                self.run_check(1, data=data)

    def test_bad_fields_and_types(self):
        mutations = [
            ("status", "complete"), ("mode", "deploy"), ("schema_version", True),
            ("summary", ""), ("application", []), ("inputs", {}), ("artifacts", []),
            ("environments", []), ("validations", [None]), ("blockers", [17]),
            ("request", {"path": "other.md", "sha256": "0" * 64}),
        ]
        for key, value in mutations:
            with self.subTest(key=key):
                data = copy.deepcopy(self.data)
                data[key] = value
                self.run_check(1, data=data)
        data = copy.deepcopy(self.data)
        del data["summary"]
        self.run_check(1, data=data)
        data = copy.deepcopy(self.data)
        data["extra"] = True
        self.run_check(1, data=data)

    def test_invalid_paths(self):
        for path in ("../report.md", "/etc/passwd", "a/../report.md", "./report.md",
                     "a//b", "a/", "C:\\file", "a\x00b"):
            with self.subTest(path=path):
                data = copy.deepcopy(self.data)
                data["artifacts"].append({"path": path, "sha256": "0" * 64})
                self.run_check(1, data=data)

    def test_file_symlink(self):
        original = self.work / "report.md"
        original.rename(self.work / "real-report.md")
        original.symlink_to("real-report.md")
        self.run_check(1)

    def test_directory_symlink(self):
        (self.work / "inputs").rename(self.work / "real-inputs")
        (self.work / "inputs").symlink_to("real-inputs", target_is_directory=True)
        self.run_check(1)

    def test_application_symlink(self):
        (self.work / "app").symlink_to("inputs", target_is_directory=True)
        self.data["application"] = "app"
        self.run_check(1)

    def test_request_symlink(self):
        (self.work / "request.md").rename(self.work / "real-request.md")
        (self.work / "request.md").symlink_to("real-request.md")
        self.run_check(1)

    def test_handoff_symlink(self):
        self.write("actual.json", json.dumps(self.data))
        (self.work / "handoff.json").symlink_to("actual.json")
        self.run_check(1)

    def test_nonregular_artifact(self):
        os.mkfifo(self.work / "pipe")
        self.data["artifacts"].append({"path": "pipe", "sha256": "0" * 64})
        self.run_check(1)

    def test_pass_without_evidence(self):
        self.data["validations"] = [{"command": "pytest", "result": "passed", "evidence": None}]
        self.run_check(1)

    def test_unmanifested_evidence(self):
        self.data["validations"] = [{
            "command": "pytest", "result": "passed", "evidence": "evidence/check.txt",
        }]
        self.run_check(1)

    def test_missing_evidence(self):
        self.evidence_validation()
        (self.work / "evidence/check.txt").unlink()
        self.run_check(1)

    def test_empty_evidence(self):
        self.write("evidence/check.txt", "")
        self.evidence_validation()
        self.run_check(1)

    def test_failed_cannot_be_ready(self):
        self.evidence_validation("failed")
        self.run_check(1)

    def test_blocked_oidc_with_passed_local_tests(self):
        self.evidence_validation()
        local = self.data["validations"][0]
        local["command"] = "synthetic local unit tests"
        self.block()
        self.data["validations"].append(local)
        self.data.update(mode="build", summary="Local checks passed; OIDC integration blocked.")
        self.data["limitations"] = ["No UAT issuer/client; external OIDC integration not tested."]
        self.data["next_actions"] = ["IdP owner supplies UAT configuration; run integration tests."]
        self.run_check()
        self.write("app/partial.txt", "Synthetic partial artifact only.")
        self.data.update(application="app")
        self.data["artifacts"].append(self.binding("app/partial.txt"))
        self.run_check()

    def test_blocked_pass_requires_evidence(self):
        self.block()
        self.data["validations"].append({
            "command": "local tests", "result": "passed", "evidence": None,
        })
        self.run_check(1)
        self.data["validations"][-1]["evidence"] = "evidence/check.txt"
        self.run_check(1)
        self.data["artifacts"].append(self.binding("evidence/check.txt"))
        (self.work / "evidence/check.txt").unlink()
        self.run_check(1)

    def test_blocked_pass_requires_disclosures(self):
        self.block()
        self.evidence_validation()
        for key in ("blockers", "limitations", "next_actions"):
            with self.subTest(key=key):
                data = copy.deepcopy(self.data)
                data[key] = []
                self.run_check(1, data=data)

    def test_blocked_validation_cannot_be_ready(self):
        self.data["validations"] = [{
            "command": "OIDC integration", "result": "blocked", "evidence": None,
        }]
        self.run_check(1)

    def test_ready_build_requires_application_artifact(self):
        self.data["mode"] = "build"
        self.run_check(1)  # Report-only, null application.
        self.data["application"] = "app"
        self.run_check(1)  # Missing application.
        (self.work / "app").mkdir()
        self.run_check(1)  # Empty directory.
        self.write("app-other/artifact.txt", "Synthetic sibling, not below app.")
        self.data["artifacts"].append(self.binding("app-other/artifact.txt"))
        self.run_check(1)  # Prefix collision is not a descendant.
        self.write("app/nested/artifact.txt", "Synthetic artifact only.")
        self.data["artifacts"].append(self.binding("app/nested/artifact.txt"))
        self.run_check()

    def test_blocked_requires_blockers(self):
        self.data["status"] = "blocked"
        self.run_check(1)

    def test_not_run_needs_disclosure(self):
        self.data["validations"] = [{"command": "pytest", "result": "not_run", "evidence": None}]
        self.run_check()
        self.data["limitations"] = []
        self.run_check(1)

    def test_duplicate_bindings_and_invalid_hash(self):
        self.data["artifacts"].append(self.binding("report.md"))
        self.run_check(1)
        self.data["artifacts"].pop()
        self.data["request"]["sha256"] = "garbage"
        self.run_check(1)

    def test_parser_bounds_and_malformed_json(self):
        cases = [
            "{}", "[]", "{", '{"x":1,"x":2}', '{"x":NaN}', '{"x":Infinity}',
            '{"x":1e999}', '{"x":' + "9" * 5000 + "}",
            "[" * 1001 + "0" + "]" * 1001,
            '{"x":' + json.dumps("x" * 8193) + "}",
            '{"x":' + json.dumps([0] * 513) + "}",
            '{"x":"\\ud800"}', " " * (256 * 1024 + 1),
        ]
        for index, raw in enumerate(cases):
            with self.subTest(index=index):
                self.run_check(1, raw=raw)


if __name__ == "__main__":
    unittest.main(verbosity=2)
