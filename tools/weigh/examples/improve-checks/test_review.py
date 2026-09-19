#!/usr/bin/env python3
"""Exercise the caller-owned review helper through real public subprocesses."""
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest


HERE = Path(__file__).resolve().parent


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def encode(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()


def b64(raw):
    return base64.b64encode(raw).decode()


class ReviewTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="bench-review-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source, self.candidate = self.root / "source", self.root / "candidate"
        self.source.mkdir()
        self.candidate.mkdir()
        self.tools = self.root / "tools"
        self.tools.mkdir()
        for name in ("review.py", "evaluate.py"):
            shutil.copy2(HERE / name, self.tools / name)
        self.helper = self.tools / "review.py"
        self.environment = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
        self.environment.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull)
        self.process(["git", "init", "--quiet", str(self.source)])
        content = {
            ".gitattributes": b"*.txt -diff\n*.md diff=private-driver\n",
            "expert/skills/checks/SKILL.md": b"Accept at 0.7.\nKeep factual coverage.\n",
            "expert/checks/run": b"#!/bin/sh\nexit 0\n",
            "old.txt": b"Remove this obsolete check.\n",
            "spaces/na\u00efve name.txt": b"keep exact CRLF bytes\r\nold value\r\n",
        }
        for name, raw in content.items():
            for root in (self.source, self.candidate):
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(raw)
                target.chmod(0o644)
        (self.candidate / "expert/skills/checks/SKILL.md").write_text("Accept at 0.5.\nKeep factual coverage.\n")
        (self.candidate / "expert/checks/run").chmod(0o755)
        (self.candidate / "old.txt").unlink()
        (self.candidate / "new.txt").write_bytes(b"A new criterion.  \n")
        (self.candidate / "spaces/na\u00efve name.txt").write_bytes(b"keep exact CRLF bytes\r\nnew value\r\n")
        self.files = sorted([*content, "new.txt"])
        self.selection = [item for name in self.files for item in ("--file", name)]
        self.before = self.fingerprint(self.source)
        self.after = self.fingerprint(self.candidate)
        self.request = self.evaluation_request()
        self.evaluation = self.root / "evaluation.json"
        self.evaluation.write_bytes(encode(self.request))
        self.proposal_dir = self.root / "proposal"

    def process(self, argv, *, env=None, input=None):
        result = subprocess.run(argv, capture_output=True, timeout=30,
                                env=self.environment if env is None else env, input=input)
        self.assertEqual(result.returncode, 0, result.stderr.decode(errors="replace"))
        return result

    def call(self, *args, ok=True, env=None):
        result = subprocess.run([sys.executable, str(self.helper), *map(str, args)],
                                capture_output=True, timeout=30, env=self.environment if env is None else env)
        if ok:
            self.assertEqual(result.returncode, 0, result.stderr.decode(errors="replace"))
        else:
            self.assertEqual(result.returncode, 2)
            self.assertEqual(result.stdout, b"")
        return result

    def fingerprint(self, root):
        return self.call("fingerprint", "--root", root, *self.selection).stdout.decode().strip()

    def evaluation_request(self):
        criteria = [{"id": "content", "check": "semantic"}]
        settings = [
            {"id": "baseline", "profile": "old", "source_sha256": self.before,
             "thresholds": {"content": .7}, "drop_checks": [], "drop_criteria": []},
            {"id": "proposal", "profile": "new", "source_sha256": self.after,
             "thresholds": {"content": .5}, "drop_checks": [], "drop_criteria": []},
        ]
        request = {
            "version": 1, "criteria": criteria, "settings": settings,
            "baseline": "baseline", "proposal": "proposal", "proposal_basis": "calibration",
            "constraints": {"required_criteria": ["content"], "required_checks": ["semantic"],
                            "min_evaluation_cases": 2, "max_false_pass_rate": 0,
                            "max_false_rejection_rate": 0, "require_final_review": True},
            "cases": [], "repair_trials": [],
        }
        for split in ("calibration", "evaluation"):
            for label, score in (("accept", .6), ("reject", .2)):
                identifier = split + "-" + label
                request["cases"].append({
                    "id": identifier, "split": split, "candidate_sha256": sha(identifier.encode()),
                    "evidence_sha256": None, "expected": label, "label_independent": True,
                    "label_basis": "Independent review against source evidence.",
                    "profiles": {profile: {"criteria": {"content": {"status": "known", "score": score}}}
                                 for profile in ("old", "new")},
                })
        for index in range(2):
            trial = {"id": "repair-%s" % index, "split": "evaluation", "task_sha256": sha(str(index).encode()), "settings": {}}
            for setting in settings:
                quality = "reject" if index == 0 and setting["id"] == "baseline" else "accept"
                trial["settings"][setting["id"]] = {
                    "setting_sha256": sha(encode({"criteria": criteria, "setting": setting})),
                    "artifact_sha256": sha((str(index) + setting["id"]).encode()), "checker_accepted": quality == "accept",
                    "final_review": {"quality": quality, "basis": "Independent blind review of the actual final artifact.", "independent": True},
                }
            request["repair_trials"].append(trial)
        return request

    def prepare(self, *, out=None, ok=True, env=None):
        result = self.call("prepare", "--source", self.source, "--candidate", self.candidate,
                           "--evaluation", self.evaluation, "--out", out or self.proposal_dir,
                           *self.selection, "--title", "Improve checker sensitivity",
                           "--reason", "Independent paired cases showed fewer false rejections.",
                           "--tradeoff", "The observed sample is small; retain the original coverage.", ok=ok, env=env)
        return json.loads(result.stdout) if ok else result

    def manifest(self):
        return json.loads((self.proposal_dir / "proposal.json").read_bytes())

    def rewrite(self, manifest):
        raw = encode(manifest) + b"\n"
        (self.proposal_dir / "proposal.json").write_bytes(raw)
        return sha(raw)

    def test_public_prepare_show_verify_and_git_apply_exact_selected_bytes(self):
        prepared = self.prepare()
        self.assertEqual(prepared["tested"], "supported")
        self.assertEqual(prepared["number"], 1)
        self.assertEqual(self.fingerprint(self.source), self.before)
        shown = json.loads(self.call("show", self.proposal_dir, "--number", "3").stdout)
        self.assertEqual(shown["number"], 3)
        self.assertEqual(shown["reviewed_sha256"], prepared["reviewed_sha256"])
        verified = json.loads(self.call("verify", self.proposal_dir, "--reviewed-sha", shown["reviewed_sha256"]).stdout)
        self.assertEqual(self.fingerprint(self.source), self.before)
        self.assertEqual(verified["expected_source_sha256"], self.after)
        self.assertEqual(verified["apply_argv"][0], "git")
        self.assertTrue(all(isinstance(item, str) for item in verified["apply_argv"]))
        self.process(verified["apply_argv"])
        self.assertEqual(self.fingerprint(self.source), self.after)
        self.assertEqual((self.source / "new.txt").read_bytes(), b"A new criterion.  \n")
        self.assertEqual((self.source / "spaces/na\u00efve name.txt").read_bytes(),
                         (self.candidate / "spaces/na\u00efve name.txt").read_bytes())
        self.assertFalse((self.source / "old.txt").exists())
        self.assertTrue((self.source / "expert/checks/run").stat().st_mode & 0o111)

    def test_wrong_review_digest_is_rejected(self):
        self.prepare()
        result = self.call("verify", self.proposal_dir, "--reviewed-sha", "0" * 64, ok=False)
        self.assertIn(b"changed since review", result.stderr)

    def test_changed_source_cannot_be_applied(self):
        prepared = self.prepare()
        (self.source / "expert/skills/checks/SKILL.md").write_text("Concurrent source change.\n")
        result = self.call("verify", self.proposal_dir, "--reviewed-sha", prepared["reviewed_sha256"], ok=False)
        self.assertIn(b"source changed since review", result.stderr)

    def test_source_root_cannot_be_replaced_by_a_symlink(self):
        prepared = self.prepare()
        moved = self.root / "moved-source"
        self.source.rename(moved)
        self.source.symlink_to(moved, target_is_directory=True)
        result = self.call("verify", self.proposal_dir, "--reviewed-sha", prepared["reviewed_sha256"], ok=False)
        self.assertIn(b"directory identity changed", result.stderr)

    def test_changed_patch_cannot_be_applied(self):
        prepared = self.prepare()
        patch = self.proposal_dir / "change.patch"
        patch.write_bytes(patch.read_bytes() + b"unreviewed bytes\n")
        result = self.call("verify", self.proposal_dir, "--reviewed-sha", prepared["reviewed_sha256"], ok=False)
        self.assertIn(b"patch differs", result.stderr)
        self.assertEqual(self.fingerprint(self.source), self.before)

    def test_changed_evaluator_requires_fresh_evaluation(self):
        self.prepare()
        evaluator = self.tools / "evaluate.py"
        evaluator.write_bytes(evaluator.read_bytes() + b"\n# Later evaluator revision.\n")
        result = self.call("show", self.proposal_dir, ok=False)
        self.assertIn(b"evaluator changed", result.stderr)

    def test_unsupported_real_evaluation_can_be_shown_but_not_verified(self):
        self.request["repair_trials"][0]["settings"]["baseline"]["final_review"]["quality"] = "accept"
        self.request["repair_trials"][0]["settings"]["proposal"]["final_review"]["quality"] = "reject"
        self.evaluation.write_bytes(encode(self.request))
        prepared = self.prepare()
        self.assertEqual(prepared["tested"], "blocked")
        self.call("show", self.proposal_dir)
        result = self.call("verify", self.proposal_dir, "--reviewed-sha", prepared["reviewed_sha256"], ok=False)
        self.assertIn(b"no supported change", result.stderr)

    def test_forged_cached_report_is_not_trusted(self):
        self.prepare()
        manifest = self.manifest()
        manifest["report"]["decision"] = "blocked"
        self.rewrite(manifest)
        result = self.call("show", self.proposal_dir, ok=False)
        self.assertIn(b"evaluation differs", result.stderr)

    def test_load_rechecks_source_pins_even_with_consistent_fresh_report(self):
        self.prepare()
        manifest = self.manifest()
        request = copy.deepcopy(self.request)
        request["settings"][0]["source_sha256"] = "0" * 64
        request["repair_trials"] = []
        request["constraints"]["require_final_review"] = False
        raw = encode(request)
        report = self.process([sys.executable, str(self.tools / "evaluate.py"), "--input", "-"], input=raw)
        manifest["evaluation"], manifest["report"] = b64(raw), json.loads(report.stdout)
        new_digest = self.rewrite(manifest)
        result = self.call("verify", self.proposal_dir, "--reviewed-sha", new_digest, ok=False)
        self.assertIn(b"source pins do not match", result.stderr)

    def test_malicious_snapshot_paths_are_rejected_before_temp_writes(self):
        self.prepare()
        original = self.manifest()
        sentinel = self.root / "outside-selected-source.txt"
        sentinel.write_text("untouched\n")
        for dangerous in (str(sentinel), "../escaped.txt", "old/../../escaped.txt", ".git/config", ".GIT/config", "./relative.txt", ".", "bad\\path"):
            with self.subTest(path=dangerous):
                manifest = copy.deepcopy(original)
                manifest["files"][0] = dangerous
                manifest["before"][0]["path"] = dangerous
                manifest["after"][0]["path"] = dangerous
                manifest["after"][0]["content"] = b64(b"do not write this\n")
                self.rewrite(manifest)
                self.call("show", self.proposal_dir, ok=False)
                self.assertEqual(sentinel.read_text(), "untouched\n")
                self.assertFalse((self.root / "escaped.txt").exists())

    def test_snapshot_rows_modes_base64_text_and_file_lists_are_strict(self):
        self.prepare()
        original = self.manifest()
        mutations = [
            lambda m: m["after"][0].update(extra="unsupported"),
            lambda m: m["after"][0].update(mode="120000"),
            lambda m: m["after"][0].update(content=None, mode="100644"),
            lambda m: m["after"][0].update(content="YR=="),
            lambda m: m["after"][0].update(content="%%%"),
            lambda m: m["after"][0].update(content=b64(b"\x00")),
            lambda m: m["after"][0].update(content=b64(b"\xff")),
            lambda m: m["after"].pop(),
            lambda m: m["after"].reverse(),
            lambda m: m["files"].append("extra.txt"),
            lambda m: m["files"].reverse(),
            lambda m: m.update(version=True),
            lambda m: m.update(source="relative/source"),
            lambda m: m.update(source=str(self.source / ".." / "source")),
            lambda m: m.update(unsupported="field"),
        ]
        for index, mutate in enumerate(mutations):
            with self.subTest(mutation=index):
                manifest = copy.deepcopy(original)
                mutate(manifest)
                self.rewrite(manifest)
                self.call("show", self.proposal_dir, ok=False)

    def test_file_selection_rejects_aliases_collisions_and_symlinks(self):
        for files in (("A", "a"), ("dir", "dir/file"), ("file", "file"), (".",), ("../outside",)):
            with self.subTest(files=files):
                selection = [item for name in files for item in ("--file", name)]
                self.call("fingerprint", "--root", self.source, *selection, ok=False)
        (self.source / "link").symlink_to(self.candidate / "new.txt")
        (self.source / "linked-dir").symlink_to(self.candidate, target_is_directory=True)
        (self.source / "dangling").symlink_to(self.root / "absent")
        for name in ("link", "linked-dir/new.txt", "dangling"):
            self.call("fingerprint", "--root", self.source, "--file", name, ok=False)

    def test_output_cannot_mutate_selected_source_or_candidate(self):
        for root in (self.source, self.candidate):
            result = self.prepare(out=root / "proposal", ok=False)
            self.assertIn(b"outside the source and candidate", result.stderr)
            self.assertFalse((root / "proposal").exists())

    def test_git_diff_is_deterministic_under_ambient_configuration(self):
        first = self.prepare()
        config = self.root / "hostile.gitconfig"
        config.write_text("[core]\n\tautocrlf = true\n\tquotePath = true\n[diff]\n\texternal = never-run-private-command\n\tnoprefix = true\n\talgorithm = patience\n[color]\n\tui = always\n[apply]\n\twhitespace = fix\n")
        environment = dict(self.environment, GIT_CONFIG_GLOBAL=str(config), GIT_DIR=str(self.root / "wrong-git"),
                           GIT_WORK_TREE=str(self.candidate), GIT_EXTERNAL_DIFF="never-run-secret", GIT_DIFF_OPTS="--unified=0")
        second_dir = self.root / "second-proposal"
        second = self.prepare(out=second_dir, env=environment)
        self.assertEqual(first["reviewed_sha256"], second["reviewed_sha256"])
        self.assertEqual((self.proposal_dir / "change.patch").read_bytes(), (second_dir / "change.patch").read_bytes())
        verified = json.loads(self.call("verify", second_dir, "--reviewed-sha", second["reviewed_sha256"], env=environment).stdout)
        self.process(verified["apply_argv"], env=environment)
        self.assertEqual(self.fingerprint(self.source), self.after)
        self.assertEqual(self.fingerprint(self.candidate), self.after)

    def test_invalid_evaluation_diagnostics_do_not_echo_private_data(self):
        self.request["private-secret-do-not-echo"] = "sensitive model dependency output"
        self.evaluation.write_bytes(encode(self.request))
        result = self.prepare(ok=False)
        self.assertIn(b"evaluation input did not validate", result.stderr)
        self.assertNotIn(b"private-secret", result.stderr)
        self.assertNotIn(b"sensitive model", result.stderr)

    def test_suggestion_numbers_must_be_positive_and_bounded(self):
        self.prepare()
        for number in (0, -1, 10001):
            self.call("show", self.proposal_dir, "--number", str(number), ok=False)


if __name__ == "__main__":
    unittest.main()
