"""Exercise the Codex adapter through a fake CLI; no auth or network is needed."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("codex_driver", HERE / "drivers" / "codex.py")
driver = importlib.util.module_from_spec(spec)
spec.loader.exec_module(driver)


class CodexAdapterTests(unittest.TestCase):
    def test_fake_cli_receives_isolated_auth_and_literal_policy_and_records_usage(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            private = root / "private"
            private.mkdir(mode=0o700)
            (private / "codex").mkdir(mode=0o700)
            auth = private / "codex" / "auth.json"
            auth.write_text('{"fake":"credential-must-stay-private"}')
            auth.chmod(0o600)
            work, state = root / "work", root / "state"
            work.mkdir()
            state.mkdir()
            fake = root / "fake-codex"
            fake.write_text("#!" + sys.executable + "\n" + '''import json, os
from pathlib import Path
import sys
args = sys.argv[1:]
home = Path(os.environ["HOME"])
codex = Path(os.environ["CODEX_HOME"])
assert codex.parent == home
assert (codex / "auth.json").read_text() == '{"fake":"credential-must-stay-private"}'
assert list(codex.iterdir()) == [codex / "auth.json"]
(codex / "auth.json").write_text('{"fake":"rotated-private-credential"}')
assert "PLY_EVAL_AUTH_ROOT" not in os.environ
assert "--ignore-user-config" in args and "--ignore-rules" in args and "--ephemeral" in args
assert args[args.index("-m") + 1] == "exact-test-model"
assert 'model_reasoning_effort="medium"' in args
assert 'cli_auth_credentials_store="file"' in args
assert 'web_search="disabled"' in args
assert "sandbox_workspace_write.network_access=false" in args
assert "features.skip_host_skill_discovery=true" in args
assert "project_doc_max_bytes=0" in args
assert args[args.index("-s") + 1] == "workspace-write"
assert Path.cwd() == Path(args[args.index("-C") + 1])
assert sys.stdin.read() == "exact task prompt"
answer = Path(args[args.index("-o") + 1])
answer.write_text("Done\\nEVAL_SUCCESS\\n")
for event in [
    {"type":"thread.started", "model":"resolved-test-model"},
    {"type":"turn.started"},
    {"type":"item.completed", "item":{"type":"agent_message", "text":"earlier commentary"}},
    {"type":"turn.completed", "usage":{"input_tokens":100,"cached_input_tokens":80,"output_tokens":20,"reasoning_output_tokens":7}},
]: print(json.dumps(event))
''')
            fake.chmod(0o700)
            request = {"schema": "ply.eval/request/v1", "suite": "task", "resume": False,
                       "workdir": str(work), "state_dir": str(state), "model": "exact-test-model",
                       "prompt": "exact task prompt", "options": {"effort": "medium"}, "budget": {"wall_seconds": 10}}
            env = {**os.environ, "PLY_EVAL_CODEX": str(fake), "PLY_EVAL_AUTH_ROOT": str(private)}
            result = subprocess.run([sys.executable, str(HERE / "drivers" / "codex.py")],
                                    input=json.dumps(request), capture_output=True, text=True, env=env, timeout=10)
            self.assertEqual(result.returncode, 0, result.stderr)
            observed = json.loads(result.stdout)
            self.assertTrue(observed["claimed_success"])
            self.assertEqual(observed["answer"], "Done\nEVAL_SUCCESS\n")
            self.assertEqual(observed["usage"]["input_tokens"], 100)
            self.assertEqual(observed["usage"]["cached_input_tokens"], 80)
            self.assertEqual(observed["usage"]["reasoning_tokens"], 7)
            self.assertIsNone(observed["usage"]["cost_usd"])
            self.assertEqual(list(private.iterdir()), [private / "codex"])
            self.assertEqual(auth.read_text(), '{"fake":"rotated-private-credential"}')
            self.assertEqual(auth.stat().st_mode & 0o777, 0o600)
            provenance = json.loads((state / "codex-provenance.json").read_text())
            self.assertEqual(provenance["observed_model_ids"], ["resolved-test-model"])
            for path in state.iterdir():
                self.assertNotIn("credential-must-stay-private", path.read_text())
                self.assertNotIn("rotated-private-credential", path.read_text())

    def test_incomplete_usage_remains_unknown(self):
        usage, models = driver.observations([
            {"type": "turn.started"}, {"type": "turn.completed", "usage": {"input_tokens": 10, "output_tokens": 2}},
            {"type": "turn.started"}, {"type": "turn.failed"},
        ])
        self.assertIsNone(usage["input_tokens"])
        self.assertEqual(usage["observed_input_tokens"], 10)
        self.assertEqual(usage["unreported_turns"], 1)
        self.assertIsNone(usage["cached_input_tokens"])
        self.assertEqual(models, [])

    def test_lifecycle_is_rejected_before_auth_or_process_launch(self):
        result = subprocess.run([sys.executable, str(HERE / "drivers" / "codex.py")],
                                input=json.dumps({"schema": "ply.eval/request/v1", "suite": "lifecycle"}),
                                capture_output=True, text=True, timeout=5)
        self.assertEqual(result.returncode, 1)
        self.assertIn("fresh task-suite", result.stderr)


if __name__ == "__main__":
    unittest.main()
