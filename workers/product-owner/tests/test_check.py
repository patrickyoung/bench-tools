import copy
import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path

SOURCE_EXPERT = Path(os.environ.get(
    "PRODUCT_OWNER_EXPERT", Path(__file__).resolve().parents[1] / "expert"
)).resolve()


def digest(data):
    return hashlib.sha256(data).hexdigest()


DETAILS = {
    "intake": {
        "sipoc": {
            "suppliers": ["Support"],
            "inputs": ["Bound request"],
            "process": ["Triage demand"],
            "outputs": ["Disposition"],
            "customers": ["Product customer"],
        },
        "constraints": ["No release authority"],
        "measures": ["Accepted demands / all demands, monthly cohort"],
        "boundaries": {"start": "Demand received", "end": "Disposition accepted"},
        "triage": [{
            "demand": "Reduce avoidable support contact",
            "decision": "discover",
            "rationale": "Outcome is plausible; baseline is not supplied",
        }],
    },
    "planning": {
        "goal": "Learn whether the first vertical slice reduces avoidable contact",
        "backlog": [{
            "item": "Test one supported self-service path",
            "outcome": "Eligible customers complete without support",
            "rationale": "Small reversible outcome slice",
            "acceptance": ["A supplied eligible case can complete end to end"],
        }],
        "forecast": {
            "basis": "Sequence only; no delivery history was supplied",
            "uncertainty": "Team sizing and capacity are unknown",
        },
    },
    "review": {
        "decision": "hold",
        "findings": [{
            "finding": "Outcome baseline is absent",
            "evidence": "No baseline appears in supplied files",
            "action": "Name and validate the baseline source",
        }],
    },
    "interview": {
        "answers": [{
            "question": "What would you do first?",
            "answer": "Hypothetically, I would establish the customer outcome and evidence.",
            "experience_basis": "hypothetical",
        }],
    },
}


class PackageCheckTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        root = Path(self.temp.name)
        # The exported definition is copied alone and kept separate from work.
        self.expert = root / "exported" / "expert"
        shutil.copytree(SOURCE_EXPERT, self.expert)
        self.work = root / "fresh-workspace"
        self.work.mkdir()
        self.check = self.expert / "bin" / "check"

    def tearDown(self):
        self.temp.cleanup()

    def write_package(self, mode="review", status="ready", with_input=False):
        request = b"Make a bounded product decision from supplied evidence.\n"
        response = b"# Recommendation\n\nHold pending the named evidence.\n"
        (self.work / "request.md").write_bytes(request)
        output = self.work / "output"
        output.mkdir()
        output.joinpath("response.md").write_bytes(response)
        records = []
        if with_input:
            source = self.work / "inputs" / "nested" / "facts.txt"
            source.parent.mkdir(parents=True)
            source.write_bytes(b"Current supplied fact.\n")
            records.append({
                "path": "inputs/nested/facts.txt",
                "sha256": digest(source.read_bytes()),
            })
        document = {
            "schema": "bench.product-owner/v1",
            "mode": mode,
            "status": status,
            "request_sha256": digest(request),
            "inputs": records,
            "response_sha256": digest(response),
            "summary": "A bounded decision is available.",
            "recommendation": "Hold pending evidence.",
            "assumptions": [],
            "questions": [],
            "next_actions": [{
                "owner": "Product Owner",
                "action": "Obtain the named evidence",
                "acceptance": "A current source is supplied",
            }],
            "details": copy.deepcopy(DETAILS[mode]),
        }
        if status == "needs-input":
            document["questions"] = ["Who owns the current outcome baseline?"]
            if mode == "intake":
                document["details"]["sipoc"]["suppliers"] = []
                document["details"]["boundaries"]["start"] = ""
        output.joinpath("decision.json").write_text(
            json.dumps(document, sort_keys=True), encoding="utf-8"
        )
        return document

    def run_check(self, expected):
        env = os.environ.copy()
        env["PYTHONDONTWRITEBYTECODE"] = "1"
        result = subprocess.run(
            [str(self.check)],
            cwd=self.work,
            env=env,
            stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=10,
        )
        self.assertEqual(
            expected,
            result.returncode,
            msg=f"stdout={result.stdout}\nstderr={result.stderr}",
        )
        return result

    def rewrite(self, document):
        (self.work / "output" / "decision.json").write_text(
            json.dumps(document, sort_keys=True), encoding="utf-8"
        )

    def test_good_intake(self):
        self.write_package("intake")
        self.run_check(0)

    def test_good_planning(self):
        self.write_package("planning")
        self.run_check(0)

    def test_good_review_with_nested_input(self):
        self.write_package("review", with_input=True)
        self.run_check(0)

    def test_good_interview(self):
        self.write_package("interview")
        self.run_check(0)

    def test_good_needs_input_can_retain_unknowns(self):
        self.write_package("intake", status="needs-input")
        self.run_check(0)

    def test_stale_request(self):
        self.write_package()
        (self.work / "request.md").write_text("Changed request.\n")
        self.run_check(1)

    def test_modified_input(self):
        self.write_package(with_input=True)
        (self.work / "inputs" / "nested" / "facts.txt").write_text("Changed.\n")
        self.run_check(1)

    def test_modified_response(self):
        self.write_package()
        (self.work / "output" / "response.md").write_text("Changed.\n")
        self.run_check(1)

    def test_omitted_evidence(self):
        document = self.write_package(with_input=True)
        document["inputs"] = []
        self.rewrite(document)
        self.run_check(1)

    def test_missing_sipoc(self):
        document = self.write_package("intake")
        del document["details"]["sipoc"]
        self.rewrite(document)
        self.run_check(1)

    def test_invalid_planning_shape(self):
        document = self.write_package("planning")
        document["details"]["backlog"][0]["acceptance"] = []
        self.rewrite(document)
        self.run_check(1)

    def test_invalid_review_shape(self):
        document = self.write_package("review")
        document["details"]["decision"] = "ship"
        self.rewrite(document)
        self.run_check(1)

    def test_invalid_interview_shape(self):
        document = self.write_package("interview")
        document["details"]["answers"][0]["experience_basis"] = "my-career"
        self.rewrite(document)
        self.run_check(1)

    def test_needs_input_requires_question(self):
        document = self.write_package("review", status="needs-input")
        document["questions"] = []
        self.rewrite(document)
        self.run_check(1)

    def test_path_traversal_rejected(self):
        document = self.write_package(with_input=True)
        document["inputs"][0]["path"] = "inputs/../facts.txt"
        self.rewrite(document)
        self.run_check(1)

    def test_symlink_request_rejected(self):
        self.write_package()
        target = self.work / "actual-request.txt"
        target.write_text("Request target.\n")
        (self.work / "request.md").unlink()
        (self.work / "request.md").symlink_to(target)
        self.run_check(1)

    def test_symlink_input_rejected(self):
        self.write_package()
        inputs = self.work / "inputs"
        inputs.mkdir()
        target = self.work / "fact-target.txt"
        target.write_text("Fact.\n")
        (inputs / "fact.txt").symlink_to(target)
        self.run_check(1)

    def test_symlink_output_rejected(self):
        self.write_package()
        response = self.work / "output" / "response.md"
        target = self.work / "actual-response.md"
        target.write_bytes(response.read_bytes())
        response.unlink()
        response.symlink_to(target)
        self.run_check(1)

    def test_extra_output_rejected(self):
        self.write_package()
        (self.work / "output" / "notes.txt").write_text("Not allowed.\n")
        self.run_check(1)

    def test_malformed_enums_are_invalid_not_broken(self):
        original = self.write_package()
        for key in ("mode", "status"):
            for value in ([], {}, None, 1):
                with self.subTest(key=key, value=value):
                    document = copy.deepcopy(original)
                    document[key] = value
                    self.rewrite(document)
                    self.run_check(1)
        original["details"]["decision"] = []
        self.rewrite(original)
        self.run_check(1)

    def test_duplicate_keys_and_non_json_numbers_rejected(self):
        document = self.write_package()
        target = self.work / "output" / "decision.json"
        for extra in ('"mode":"review",', '"unused":NaN,'):
            target.write_text("{" + extra + json.dumps(document)[1:])
            self.run_check(1)

    def test_oversized_request_and_json(self):
        self.write_package()
        request = self.work / "request.md"
        original = request.read_bytes()
        request.write_bytes(b"x" * (1024 * 1024 + 1))
        self.run_check(1)
        request.write_bytes(original)
        (self.work / "output" / "decision.json").write_bytes(b" " * (256 * 1024 + 1))
        self.run_check(1)

    def test_input_directory_and_output_directory_symlinks(self):
        self.write_package()
        elsewhere = Path(self.temp.name) / "elsewhere"
        elsewhere.mkdir()
        (self.work / "inputs").symlink_to(elsewhere, target_is_directory=True)
        self.run_check(1)
        (self.work / "inputs").unlink()
        shutil.rmtree(self.work / "output")
        (self.work / "output").symlink_to(elsewhere, target_is_directory=True)
        self.run_check(1)

    def test_input_counts_include_empty_directories(self):
        self.write_package()
        inputs = self.work / "inputs"
        inputs.mkdir()
        for index in range(257):
            (inputs / str(index)).mkdir()
        self.run_check(1)

    def test_deep_input_tree(self):
        self.write_package()
        deepest = self.work / "inputs"
        for index in range(18):
            deepest = deepest / str(index)
        deepest.mkdir(parents=True)
        self.run_check(1)

    def test_fifo_rejected_without_blocking(self):
        self.write_package()
        request = self.work / "request.md"
        request.unlink()
        os.mkfifo(request)
        self.run_check(1)

    def test_missing_or_empty_response(self):
        self.write_package()
        response = self.work / "output" / "response.md"
        response.write_text("  \n")
        self.run_check(1)
        response.unlink()
        self.run_check(1)


if __name__ == "__main__":
    unittest.main()
