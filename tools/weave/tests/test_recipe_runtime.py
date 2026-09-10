"""Entry-point failures before admission require no models or external tools."""
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]


class RecipeRuntimeTests(unittest.TestCase):
    def test_interrupt_between_rounds_exits_without_a_traceback(self):
        script = """import runpy, sys
from unittest import mock
sys.path.insert(0, sys.argv[1])
entry = sys.argv[2]
sys.argv = sys.argv[2:]
with mock.patch('fcntl.flock', side_effect=KeyboardInterrupt):
    runpy.run_path(entry, run_name='__main__')
"""
        with tempfile.TemporaryDirectory(prefix="weave-interrupt-test-") as directory:
            for entry, args in (("run.py", ["policy-review", directory]),
                                ("research.py", [directory, "--arms", "search"])):
                with self.subTest(entry=entry):
                    result = subprocess.run([sys.executable, "-c", script, str(ROOT / "examples"),
                                             str(ROOT / "examples" / entry), *args],
                                            capture_output=True, timeout=10)
                    self.assertEqual(result.returncode, 130, result.stderr.decode())
                    self.assertEqual(result.stdout, b"")
                    self.assertIn(b"interrupted", result.stderr)
                    self.assertNotIn(b"Traceback", result.stderr)


if __name__ == "__main__":
    unittest.main()
