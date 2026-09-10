"""A frozen caller-selected domain reuses the existing Tend/evidence adapter."""
import importlib.util
import io
import json
import os
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "examples"))
import run as recipe
from protocol import digest, encoded, json_rows, read

DOMAIN = """import round_helper
def agent_task(task):
    return False
def execute(task, dependencies):
    return {"value": round_helper.VALUE + sum(r["value"] for r in dependencies.values())}
def check(task, result, dependencies):
    if result != execute(task, dependencies):
        raise ValueError("wrong round result")
"""
TASKS = [{"id": "start", "needs": [], "input": {"development": [1, 2]}},
         {"id": "join", "needs": ["start"], "input": {"round": 1}}]


class RecipeSeamTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="weave-seam-")
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name).resolve()
        self.root = self.base / "round"
        self.root.mkdir()
        self.source = self.base / "new_domain.py"
        self.source.write_text(DOMAIN)
        self.helper = self.base / "round_helper.py"
        self.helper.write_text("VALUE = 7\n")

    def prepare(self, extras=None):
        return recipe.prepare_plan(self.root, "round-one", TASKS, "reference", "", self.source,
                                   {"round_helper.py": self.helper} if extras is None else extras)

    def test_domain_and_explicit_helpers_are_immutable_manifest_inputs(self):
        with mock.patch.object(recipe, "tool", return_value=sys.executable):
            tasks, _ = self.prepare([self.helper])
            self.assertEqual(tasks, TASKS)
            manifest = json.loads(read(self.root / "manifest.json"))
            self.assertEqual(manifest["sources"]["business.py"], digest(self.source.read_bytes()))
            self.assertEqual(manifest["sources"]["round_helper.py"], digest(self.helper.read_bytes()))
            self.assertEqual(read(self.root / "recipe" / "business.py"), self.source.read_bytes())
            self.prepare()  # Mapping and list forms name the same frozen inputs.
            self.helper.write_text("VALUE = 8\n")
            with self.assertRaisesRegex(ValueError, "immutable input changed"):
                self.prepare()

    def test_helpers_cannot_replace_support_code_or_escape_recipe(self):
        with mock.patch.object(recipe, "tool", return_value=sys.executable):
            for name in ("../outside.py", "worker.py", "business.py", "a/b.py", "invalid-name.py"):
                with self.subTest(name=name), self.assertRaisesRegex(ValueError, "source name"):
                    self.prepare({name: self.helper})
            with self.assertRaisesRegex(ValueError, "source name"):
                self.prepare([self.helper, self.helper])
        self.assertFalse((self.root / "manifest.json").exists())

    def test_frozen_helper_tamper_is_detected_before_acceptance(self):
        with mock.patch.object(recipe, "tool", return_value=sys.executable):
            tasks, tools = self.prepare()
        (self.root / "recipe" / "round_helper.py").write_text("VALUE = 99\n")
        with mock.patch.object(recipe, "tend", return_value=b""):
            with self.assertRaisesRegex(ValueError, "recipe source changed"):
                recipe.observe(self.root, tasks, tools, {}, domain=SimpleNamespace())

    def test_real_tend_custom_domain_output_capture_and_resume(self):
        bins = Path(os.environ.get("WEAVE_TEST_BIN", ROOT / "var" / "bin"))
        env = os.environ.copy()
        for name in ("weave", "tend"):
            path = bins / name
            if not path.is_file():
                raise RuntimeError(f"build tools first with tests/check; missing {path}")
            env[name.upper()] = str(path)
        # Load only the explicitly selected procedure for this Python caller;
        # workers import its exact frozen copies from recipe/ instead.
        with mock.patch.dict(sys.modules), mock.patch.object(sys, "path", [str(self.base), *sys.path]):
            spec = importlib.util.spec_from_file_location("seam_domain", self.source)
            domain = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(domain)
        with mock.patch.dict(os.environ, env):
            tasks, tools = self.prepare()
        env.update({"TEND_ROOT": str(self.root / "tend"), "TEND_LEASE": "3s", "TEND_JOB_MAX": "1m"})
        args = SimpleNamespace(seconds=30, jobs=2, mode="reference", model="")
        output = io.BytesIO()
        # Any accidental use of the original business module is a regression.
        with mock.patch.object(recipe.business, "check", side_effect=AssertionError("wrong domain")):
            self.assertEqual(recipe.drive(args, self.root, tasks, tools, env, domain, output), 0)
            observations, results, _ = recipe.observe(self.root, tasks, tools, env, domain)
            self.assertEqual(results, {"start": {"value": 7}, "join": {"value": 14}})
            self.assertEqual(json_rows(output.getvalue()), observations)
            events = recipe.tend(tools, env, "events")
            second = io.BytesIO()
            self.assertEqual(recipe.drive(args, self.root, tasks, tools, env, domain, second), 0)
            self.assertEqual(events, recipe.tend(tools, env, "events"))
            self.assertEqual(second.getvalue(), output.getvalue())


if __name__ == "__main__":
    unittest.main()
