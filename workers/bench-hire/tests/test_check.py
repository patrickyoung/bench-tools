"""Offline public-check contract cases. No model, credentials or UI effects."""
import copy
import hashlib
import json
from pathlib import Path
import subprocess
import tempfile
import unittest


CHECK = Path(__file__).resolve().parents[1] / "expert/bin/check"


class Contract(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.request = {
            "version": 1, "message": "Plan the next step", "history": [],
            "focus": "", "catalog": [
                {"key": "worker:source:writer", "name": "Writer", "kind": "worker", "description": "Write"},
                {"key": "worker:source:reviewer", "name": "Reviewer", "kind": "worker", "description": "Review"}],
            "diagnostics": {}, "knowledge_source": "", "context": {},
            "allowed_targets": {"open": ["worker:source:writer"],
                                "create_worker": [],
                                "create_team": ["worker:source:writer", "worker:source:reviewer"],
                                "revise_worker": ["worker:local:chosen"],
                                "analyze_run": ["job:chosen"]}}
        raw = json.dumps(self.request).encode()
        (self.work / "request.json").write_bytes(raw)
        self.reply = {"version": 1, "request_sha256": hashlib.sha256(raw).hexdigest(),
                      "message": "Here is the proposed next step.", "question": "", "actions": []}

    def check(self, reply=None, raw=None, ok=True):
        (self.work / "response.json").write_bytes(raw if raw is not None else json.dumps(reply or self.reply).encode())
        result = subprocess.run([str(CHECK)], cwd=self.work, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0 if ok else 1, result.stderr)
        self.assertEqual(sorted(p.name for p in self.work.iterdir()), ["request.json", "response.json"])

    def actions(self):
        return [
            {"kind": "create_worker", "label": "Create indexer", "target": "", "roles": [],
             "fields": {"title": "Indexer", "goal": "Index supplied files", "reads": "Rules text", "writes": "index.md", "good": "Cites a rule", "bad": "Invents a rule"}},
            {"kind": "create_team", "label": "Team draft", "target": "", "fields": {"title": "Team", "goal": "Cited guide", "handoffs": "Guide to review", "acceptance": "No unsupported claims"},
             "roles": [{"role": "writer", "worker": "worker:source:writer", "responsibility": "Draft"}, {"role": "reviewer", "worker": "worker:source:reviewer", "responsibility": "Review"}]},
            {"kind": "revise_worker", "label": "Teach", "target": "worker:local:chosen", "fields": {"change": "Cite supplied evidence"}, "roles": []},
            {"kind": "analyze_run", "label": "Analyze", "target": "job:chosen", "fields": {"question": "Was content available?"}, "roles": []},
            {"kind": "open", "label": "Open", "target": "worker:source:writer", "fields": {}, "roles": []}]

    def test_all_action_kinds_and_question(self):
        self.check()
        for action in self.actions():
            with self.subTest(kind=action["kind"]):
                self.reply["actions"] = [action]
                self.check()
        self.reply["actions"] = []
        self.reply["question"] = "Which input should the worker read?"
        self.check()

    def test_binding_duplicates_nulls_and_limits(self):
        valid = json.dumps(self.reply).encode()
        for raw in [b"{", valid + b"{}", valid.replace(b'"version": 1', b'"version": 1, "version": 1'),
                    valid.replace(b'"question": ""', b'"question": null'),
                    valid.replace(b'"question": ""', b'"question": "\\u0000"'),
                    b" " * (128 * 1024) + valid, valid.decode().encode("utf-16")]:
            with self.subTest(raw=raw[:50]):
                self.check(raw=raw, ok=False)
        self.reply["request_sha256"] = "0" * 64
        self.check(ok=False)
        (self.work / "request.json").write_bytes(b" " * (2 * 1024 * 1024 + 1))
        self.check(ok=False)

    def test_unknown_targets_fields_and_missing_keys(self):
        for action in self.actions():
            for key in list(action):
                broken = copy.deepcopy(action)
                del broken[key]
                self.reply["actions"] = [broken]
                self.check(ok=False)
            for key, value in [("kind", "run_shell"), ("target", "/private/file"), ("fields", {"argv": "touch marker"}), ("roles", None)]:
                broken = copy.deepcopy(action)
                broken[key] = value
                self.reply["actions"] = [broken]
                self.check(ok=False)

    def test_team_must_use_catalog_and_eligible_unique_roles(self):
        for key, value in [("role", "state"), ("role", "writer"), ("role", "../escape"),
                           ("worker", "worker:source:invented"), ("responsibility", "")]:
            action = self.actions()[1]
            action["roles"][1][key] = value
            self.reply["actions"] = [action]
            self.check(ok=False)
        # Allowlisting alone does not make an invented worker a catalog member.
        self.request["allowed_targets"]["create_team"].append("worker:source:invented")
        raw = json.dumps(self.request).encode()
        (self.work / "request.json").write_bytes(raw)
        self.reply["request_sha256"] = hashlib.sha256(raw).hexdigest()
        action = self.actions()[1]
        action["roles"][1]["worker"] = "worker:source:invented"
        self.reply["actions"] = [action]
        self.check(ok=False)


if __name__ == "__main__":
    unittest.main()
