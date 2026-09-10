"""Native stream and executable-fixture tests: no provider requests or real login."""
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

from run import validate_result


HERE = Path(__file__).resolve().parent


def load_driver(name):
    spec = importlib.util.spec_from_file_location("native_" + name, HERE / "drivers" / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


claude, pi = load_driver("claude"), load_driver("pi")


def claude_events():
    return [{"type": "system", "model": "claude-opus-exact"},
            {"type": "assistant", "message": {"model": "claude-opus-exact"}},
            {"type": "result", "subtype": "success", "is_error": False, "result": "Fixed.\nEVAL_SUCCESS",
             "usage": {"input_tokens": 100, "output_tokens": 30, "cache_read_input_tokens": 20, "cache_creation_input_tokens": 5},
             "total_cost_usd": .08, "modelUsage": {"claude-opus-exact": {}}, "num_turns": 2}]


def pi_events():
    message = {"role": "assistant", "provider": "openai-codex", "model": "gpt-exact", "stopReason": "stop",
               "content": [{"type": "text", "text": "Fixed.\nEVAL_SUCCESS"}],
               "usage": {"input": 100, "output": 30, "cacheRead": 20, "cacheWrite": 0,
                         "reasoning": 10, "cost": {"total": .08}}}
    return [{"type": "session", "id": "fixture"}, {"type": "message_update", "message": message},
            {"type": "message_end", "message": message}, {"type": "turn_end", "message": message},
            {"type": "agent_end", "messages": [message]}]


class NativeParserTests(unittest.TestCase):
    def test_claude_terminal_usage_and_model_not_double_counted(self):
        result = claude.parse_events(claude_events(), 0)
        self.assertTrue(result["claimed_success"])
        self.assertEqual(result["usage"]["input_tokens"], 100)
        self.assertEqual(result["native"]["resolved_models"], ["claude-opus-exact"])
        self.assertIsNone(result["usage"]["cost_usd"])
        self.assertEqual(result["native"]["estimated_cost_usd"], .08)

    def test_claude_error_and_missing_usage_remain_visible(self):
        result = claude.parse_events([{"type": "result", "subtype": "error_max_turns", "is_error": True,
                                      "errors": ["turn budget"]}], 0)
        self.assertEqual(result["exit_code"], 1)
        self.assertFalse(result["claimed_success"])
        self.assertIsNone(result["usage"])
        self.assertEqual(result["native"]["errors"], ["turn budget"])

    def test_missing_or_duplicate_claude_result_is_invalid(self):
        with self.assertRaises(ValueError):
            claude.parse_events(claude_events()[:-1], 0)
        with self.assertRaises(ValueError):
            claude.parse_events(claude_events() + claude_events()[-1:], 0)

    def test_pi_counts_completed_messages_once_and_reports_catalog_estimate(self):
        result = pi.parse_events(pi_events(), 0)
        self.assertTrue(result["claimed_success"])
        self.assertEqual(result["usage"]["input_tokens"], 100)
        self.assertEqual(result["usage"]["reasoning_tokens"], 10)
        self.assertEqual(result["usage"]["observed_turns"], 1)
        self.assertIsNone(result["usage"]["cost_usd"])
        self.assertEqual(result["native"]["resolved_models"], ["openai-codex/gpt-exact"])

    def test_pi_compaction_and_nested_tool_usage_are_included(self):
        events = pi_events()
        events.insert(1, {"type": "compaction_end", "result": {"usage": {"input": 12, "output": 3, "cost": {"total": .01}}}})
        events.insert(1, {"type": "message_end", "message": {"role": "toolResult", "usage": {"input": 4, "output": 2, "cost": {"total": .01}}}})
        result = pi.parse_events(events, 0)
        self.assertEqual(result["usage"]["input_tokens"], 116)
        self.assertEqual(result["usage"]["output_tokens"], 35)
        self.assertIsNone(result["usage"]["cached_input_tokens"])

    def test_pi_retry_makes_total_unknown_preserves_observed_usage(self):
        events = pi_events() + [{"type": "auto_retry_start", "errorMessage": "429"}]
        result = pi.parse_events(events, 0)
        self.assertIsNone(result["usage"]["input_tokens"])
        self.assertEqual(result["usage"]["observed_input_tokens"], 100)
        self.assertEqual(result["usage"]["unreported_retries"], 1)
        self.assertEqual(result["interventions"][0]["kind"], "auto_retry_start")

    def test_pi_native_error_overrides_zero_cli_exit(self):
        events = pi_events()
        events[2]["message"]["stopReason"] = "error"
        events[2]["message"]["content"] = [{"type": "text", "text": "Authentication failed"}]
        result = pi.parse_events(events, 0)
        self.assertEqual(result["exit_code"], 1)
        self.assertFalse(result["claimed_success"])

    def test_pi_incomplete_agent_stream_is_invalid(self):
        with self.assertRaises(ValueError):
            pi.parse_events(pi_events()[:-1], 0)

    def test_success_marker_must_be_whole_line(self):
        events = claude_events()
        events[-1]["result"] = "I will write EVAL_SUCCESS when done."
        self.assertFalse(claude.parse_events(events, 0)["claimed_success"])
        events = pi_events()
        events[2]["message"]["content"] = [{"type": "text", "text": "maybe EVAL_SUCCESS"}]
        self.assertFalse(pi.parse_events(events, 0)["claimed_success"])


class NativeExecutableTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="ply-native-test-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.auth = self.root / "private-auth"
        self.auth.mkdir(mode=0o700)
        for name, filename in (("claude", ".credentials.json"), ("pi", "auth.json")):
            directory = self.auth / name
            directory.mkdir(mode=0o700)
            path = directory / filename
            path.write_text('{"credential":"fixture-auth-must-not-appear"}')
            path.chmod(0o600)
        self.work, self.state, self.home = [self.root / name for name in ("work", "state", "home")]
        for path in (self.work, self.state, self.home):
            path.mkdir()
        self.request = {"suite": "task", "resume": False, "state_dir": str(self.state), "workdir": str(self.work),
                        "prompt": "Fixture prompt\nwith newline", "model": "provider/exact-id", "budget": {"turns": 7},
                        "options": {"effort": "medium"}}
        self.env = dict(os.environ, HOME=str(self.home), PLY_EVAL_AUTH_ROOT=str(self.auth))

    def execute_fixture(self, name, events):
        executable = self.root / ("fake-" + name)
        executable.write_text("#!" + sys.executable + "\nimport os,json,sys\nfrom pathlib import Path\n"
                              "Path('observed.json').write_text(json.dumps({'argv':sys.argv[1:],'stdin':sys.stdin.read(),"
                              "'cwd':os.getcwd(),'home':os.environ['HOME'],'auth':os.environ.get('CLAUDE_CONFIG_DIR',os.environ.get('PI_CODING_AGENT_DIR'))}))\n"
                              + "\n".join("print(" + repr(json.dumps(event)) + ")" for event in events) + "\n")
        executable.chmod(0o700)
        env = dict(self.env, **{"PLY_EVAL_" + name.upper(): str(executable)})
        result = subprocess.run([sys.executable, str(HERE / "drivers" / (name + ".py"))], env=env,
                                input=json.dumps(self.request), text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stderr)
        output = json.loads(result.stdout)
        validate_result(output)
        observed = json.loads((self.work / "observed.json").read_text())
        self.assertEqual(observed["stdin"], self.request["prompt"])
        self.assertEqual(observed["cwd"], str(self.work))
        self.assertEqual(observed["home"], str(self.home))
        self.assertEqual(observed["auth"], str(self.auth / name))
        self.assertNotIn("fixture-auth-must-not-appear", result.stdout + result.stderr)
        self.assertFalse(any("fixture-auth-must-not-appear" in path.read_text() for path in self.state.rglob("*") if path.is_file()))
        return output, observed

    def test_claude_cli_isolated_prompt_native_tools_and_turn_limit(self):
        result, observed = self.execute_fixture("claude", claude_events())
        self.assertTrue(result["claimed_success"])
        args = observed["argv"]
        self.assertIn("--safe-mode", args)
        self.assertIn("--no-session-persistence", args)
        self.assertIn("--strict-mcp-config", args)
        self.assertNotIn("--bare", args)
        self.assertEqual(args[args.index("--max-turns") + 1], "7")
        self.assertEqual(args[args.index("--model") + 1], self.request["model"])

    def test_pi_cli_disables_discovery_and_uses_trial_session(self):
        result, observed = self.execute_fixture("pi", pi_events())
        self.assertTrue(result["claimed_success"])
        args = observed["argv"]
        for flag in ("--no-context-files", "--no-extensions", "--no-skills", "--offline"):
            self.assertIn(flag, args)
        self.assertEqual(args[args.index("--session") + 1], str(self.state / "pi-session.jsonl"))
        metadata = json.loads((self.state / "pi-metadata.json").read_text())
        self.assertIn("not available", metadata["turn_limit"])

    def test_rejects_lifecycle_resume_and_public_auth_root(self):
        for module in (claude, pi):
            with self.assertRaises(ValueError):
                module.invocation(dict(self.request, suite="lifecycle"), self.env)
            with self.assertRaises(ValueError):
                module.invocation(dict(self.request, resume=True), self.env)
        self.auth.chmod(0o755)
        for module in (claude, pi):
            with self.assertRaisesRegex(ValueError, "private"):
                module.invocation(self.request, self.env)


if __name__ == "__main__":
    unittest.main()
