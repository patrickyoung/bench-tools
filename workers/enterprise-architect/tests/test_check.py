import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

SOURCE_EXPERT = Path(os.environ.get("ENTERPRISE_ARCHITECT_EXPERT",
                                  Path(__file__).resolve().parents[1] / "expert"))
MODES = {
    "advice": {"report"},
    "north-star": {"report", "diagram"},
    "capability-map": {"report", "catalog"},
    "roadmap": {"report", "diagram"},
    "guardrails": {"report", "policy", "tests", "ci"},
    "decision": {"report", "decision-record"},
    "integration-catalog": {"report", "catalog", "api-contract"},
    "finops": {"report", "allocation"},
    "data-ai-governance": {"report", "catalog", "policy"},
    "portfolio": {"report", "catalog"},
}
ROLE_NAMES = {
    "report": "report.md",
    "diagram": "platform.svg",
    "catalog": "catalog.json",
    "policy": "policy.rego",
    "tests": "policy_test.rego",
    "ci": "ci.md",
    "decision-record": "decision.md",
    "api-contract": "openapi.yaml",
    "allocation": "allocation.csv",
}


def digest(data):
    return hashlib.sha256(data).hexdigest()


class CheckTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        root = Path(self.temp.name)
        self.expert = root / "expert"
        shutil.copytree(SOURCE_EXPERT, self.expert)
        self.work = root / "work"
        self.work.mkdir()
        (self.work / "request.md").write_text("Recommend a bounded direction.\n")
        self.check = self.expert / "bin" / "check"

    def tearDown(self):
        self.temp.cleanup()

    def write_package(self, mode="advice", status="ready", with_input=False):
        output = self.work / "output"
        output.mkdir(exist_ok=True)
        records = []
        if with_input:
            evidence = self.work / "inputs" / "nested" / "facts.bin"
            evidence.parent.mkdir(parents=True, exist_ok=True)
            evidence.write_bytes(b"\x00current evidence\n")
            records = [{"path": "inputs/nested/facts.bin",
                        "sha256": digest(evidence.read_bytes())}]
        artifacts = []
        roles = {"report"} if status == "needs-input" else MODES[mode]
        for role in sorted(roles):
            path = output / ROLE_NAMES[role]
            path.write_text(f"Reviewable {role} artifact.\n", encoding="utf-8")
            artifacts.append({"path": "output/" + path.name,
                              "sha256": digest(path.read_bytes()), "role": role})
        artifacts.sort(key=lambda item: item["path"])
        ready = status == "ready"
        document = {
            "schema": "bench.enterprise-architect/v1",
            "deliverable": mode,
            "status": status,
            "request_sha256": digest((self.work / "request.md").read_bytes()),
            "profile_sha256": digest((self.expert / "PROFILE.md").read_bytes()),
            "inputs": records,
            "artifacts": artifacts,
            "summary": "A bounded recommendation is available.",
            "recommendation": "Use a reversible evidence gate.",
            "lenses": ["product-capabilities"],
            "outcomes": ([{"outcome": "Improve service", "measure": "Unknown baseline",
                           "evidence": "Supplied request"}] if ready else []),
            "decisions": ([{"decision": "Pilot", "rationale": "Limits exposure",
                            "alternatives": "Do nothing",
                            "tradeoffs": "Learning before scale",
                            "review_trigger": "Pilot evidence available"}] if ready else []),
            "assumptions": [],
            "questions": ([] if ready else ["Which outcome baseline is authoritative?"]),
            "next_actions": [{"owner": "Business owner", "action": "Confirm baseline",
                              "acceptance": "Dated source is supplied"}],
        }
        self.rewrite(document)
        return document

    def rewrite(self, document, raw=None):
        path = self.work / "output" / "architecture.json"
        path.write_text(raw if raw is not None else json.dumps(document, sort_keys=True),
                        encoding="utf-8")

    def run_check(self, expected):
        env = os.environ.copy()
        env["PYTHONDONTWRITEBYTECODE"] = "1"
        result = subprocess.run(
            [str(self.check)], cwd=self.work, env=env, stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=10,
        )
        self.assertEqual(expected, result.returncode,
                         f"stdout={result.stdout}\nstderr={result.stderr}")
        return result

    def test_all_ten_ready_deliverables(self):
        for mode in MODES:
            with self.subTest(mode=mode):
                shutil.rmtree(self.work / "output", ignore_errors=True)
                self.write_package(mode)
                self.run_check(0)

    def test_every_required_role_is_enforced(self):
        for mode, roles in MODES.items():
            for role in roles:
                with self.subTest(mode=mode, role=role):
                    shutil.rmtree(self.work / "output", ignore_errors=True)
                    document = self.write_package(mode)
                    victim = next(item for item in document["artifacts"]
                                  if item["role"] == role)
                    (self.work / victim["path"]).unlink()
                    document["artifacts"].remove(victim)
                    self.rewrite(document)
                    self.run_check(1)

    def test_needs_input_allows_unknown_outcomes_and_only_report(self):
        self.write_package("portfolio", "needs-input")
        self.run_check(0)

    def test_needs_input_limits_questions(self):
        document = self.write_package(status="needs-input")
        document["questions"] = ["one", "two", "three", "four"]
        self.rewrite(document)
        self.run_check(1)
        document["questions"] = []
        self.rewrite(document)
        self.run_check(1)

    def test_stale_request_input_artifact_and_profile(self):
        mutations = ("request", "input", "artifact", "profile")
        for mutation in mutations:
            with self.subTest(mutation=mutation):
                shutil.rmtree(self.work / "output", ignore_errors=True)
                shutil.rmtree(self.work / "inputs", ignore_errors=True)
                document = self.write_package("portfolio", with_input=True)
                if mutation == "request":
                    (self.work / "request.md").write_text("changed")
                elif mutation == "input":
                    (self.work / "inputs/nested/facts.bin").write_bytes(b"changed")
                elif mutation == "artifact":
                    (self.work / "output/catalog.json").write_text("changed")
                else:
                    (self.expert / "PROFILE.md").write_text("changed")
                self.run_check(1)
                if mutation == "profile":
                    shutil.copy2(SOURCE_EXPERT / "PROFILE.md", self.expert / "PROFILE.md")

    def test_malformed_json_duplicate_nonfinite_and_schema(self):
        document = self.write_package()
        valid = json.dumps(document)
        self.rewrite(document, '{"schema":"x","schema":"y"}')
        self.run_check(1)
        self.rewrite(document, valid[:-1] + ',"extra":NaN}')
        self.run_check(1)
        document["extra"] = "closed means closed"
        self.rewrite(document)
        self.run_check(1)

    def test_unsorted_traversal_and_extra_evidence(self):
        document = self.write_package(with_input=True)
        document["inputs"][0]["path"] = "inputs/../facts.bin"
        self.rewrite(document)
        self.run_check(1)
        document = self.write_package(with_input=True)
        extra = self.work / "inputs/extra.txt"
        extra.write_text("not manifested")
        self.run_check(1)

    def test_request_input_output_and_profile_symlinks_rejected(self):
        for target_kind in ("request", "input", "output", "profile"):
            with self.subTest(target_kind=target_kind):
                shutil.rmtree(self.work / "output", ignore_errors=True)
                shutil.rmtree(self.work / "inputs", ignore_errors=True)
                self.write_package(with_input=True)
                if target_kind == "request":
                    path = self.work / "request.md"
                elif target_kind == "input":
                    path = self.work / "inputs/nested/facts.bin"
                elif target_kind == "output":
                    path = self.work / "output/report.md"
                else:
                    path = self.expert / "PROFILE.md"
                target = path.with_name(path.name + ".target")
                target.write_bytes(path.read_bytes())
                path.unlink()
                path.symlink_to(target)
                self.run_check(1)
                path.unlink()
                target.rename(path)

    def test_wrong_enum_types_are_rejected_not_broken(self):
        for field in ("deliverable", "role", "status"):
            for value in ([], {}, None, 1):
                with self.subTest(field=field, value=value):
                    document = self.write_package()
                    if field == "role":
                        document["artifacts"][0][field] = value
                    else:
                        document[field] = value
                    self.rewrite(document)
                    self.run_check(1)

    def test_profile_binding_cannot_be_redirected_by_environment(self):
        self.write_package()
        old = os.environ.get("ENTERPRISE_ARCHITECT_EXPERT")
        os.environ["ENTERPRISE_ARCHITECT_EXPERT"] = str(self.work / "absent")
        try:
            self.run_check(0)
        finally:
            if old is None:
                os.environ.pop("ENTERPRISE_ARCHITECT_EXPERT", None)
            else:
                os.environ["ENTERPRISE_ARCHITECT_EXPERT"] = old

    def test_extra_output_directory_links_and_nonregular_files(self):
        self.write_package()
        output = self.work / "output"
        extra = output / "unlisted.txt"
        extra.write_text("not in manifest")
        self.run_check(1)
        extra.unlink()
        link = output / "linked"
        link.symlink_to(self.expert, target_is_directory=True)
        self.run_check(1)
        link.unlink()
        fifo = output / "pipe"
        os.mkfifo(fifo)
        self.run_check(1)

    def test_oversized_files_rejected(self):
        for target in ("request.md", "output/report.md", "output/architecture.json"):
            with self.subTest(target=target):
                self.write_package()
                path = self.work / target
                original = path.read_bytes()
                path.write_bytes(b"x" * (1024 * 1024 + 1))
                self.run_check(1)
                path.write_bytes(original)

    def test_input_file_entry_and_depth_limits(self):
        self.write_package()
        inputs = self.work / "inputs"
        inputs.mkdir()
        for index in range(65):
            (inputs / f"{index:03}.txt").write_text("x")
        self.run_check(1)
        shutil.rmtree(inputs)
        folder = inputs
        for index in range(18):
            folder = folder / str(index)
        folder.mkdir(parents=True)
        (folder / "deep.txt").write_text("x")
        self.run_check(1)
        shutil.rmtree(inputs)
        inputs.mkdir()
        for index in range(257):
            (inputs / f"d{index:03}").mkdir()
        self.run_check(1)

    def test_output_file_entry_and_depth_limits(self):
        document = self.write_package()
        output = self.work / "output"
        for index in range(31):
            path = output / f"s{index:02}.txt"
            path.write_text("x")
            document["artifacts"].append({
                "path": f"output/s{index:02}.txt", "sha256": digest(b"x"),
                "role": "supporting"})
        document["artifacts"].sort(key=lambda item: item["path"])
        self.rewrite(document)
        self.run_check(1)
        shutil.rmtree(output)
        self.write_package()
        folder = output
        for index in range(10):
            folder = folder / str(index)
        folder.mkdir(parents=True)
        (folder / "deep.txt").write_text("x")
        self.run_check(1)
        shutil.rmtree(output)
        self.write_package()
        for index in range(127):
            (output / f"d{index:03}").mkdir()
        self.run_check(1)

    def test_non_utf8_and_empty_artifacts_rejected(self):
        for payload in (b"\xff", b""):
            with self.subTest(payload=payload):
                shutil.rmtree(self.work / "output", ignore_errors=True)
                document = self.write_package()
                report = self.work / "output/report.md"
                report.write_bytes(payload)
                document["artifacts"][0]["sha256"] = digest(payload)
                self.rewrite(document)
                self.run_check(1)

    def test_policy_artifact_is_never_executed(self):
        document = self.write_package("guardrails")
        marker = self.work / "EXECUTED"
        policy = self.work / "output/policy.rego"
        policy.write_text(
            "#!/bin/sh\n"
            f"touch {marker}\n"
            "package architecture.guardrail\nallow := true\n"
        )
        policy.chmod(0o755)
        for item in document["artifacts"]:
            if item["role"] == "policy":
                item["sha256"] = digest(policy.read_bytes())
        self.rewrite(document)
        self.run_check(0)
        self.assertFalse(marker.exists())

    def test_ready_requires_outcome_decision_and_known_lens(self):
        document = self.write_package()
        document["outcomes"] = []
        self.rewrite(document)
        self.run_check(1)
        document = self.write_package()
        document["decisions"] = []
        self.rewrite(document)
        self.run_check(1)
        document = self.write_package()
        document["lenses"] = ["not-a-skill"]
        self.rewrite(document)
        self.run_check(1)


if __name__ == "__main__":
    unittest.main()
