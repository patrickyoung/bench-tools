"""Protect the independent-export and credential-isolation verification seams."""
import os
import contextlib
import io
import json
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


CHECK = runpy.run_path(str(Path(__file__).resolve().parents[1] / "check"))


class ExportTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="bench-export-test-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.leaf = self.root / "tools/one"
        self.leaf.mkdir(parents=True)
        (self.root / "tools/two").mkdir()
        (self.root / "tools/two/private.go").write_text("sibling source\n")
        (self.leaf / "main.go").write_text("original\n")
        (self.leaf / ".gitignore").write_text("var/\n")
        subprocess.run(["git", "init", "-q", str(self.root)], check=True)
        subprocess.run(["git", "add", "."], cwd=self.root, check=True)
        self.component = {"name": "one", "path": "tools/one"}
        self.destination = self.root / "export"
        self.destination.mkdir()

    def export(self):
        return CHECK["export_source"](self.root, self.component, self.destination)

    def test_current_edits_and_new_source_survive_without_siblings_or_ignored_state(self):
        (self.leaf / "main.go").write_text("working tree edit\n")
        (self.leaf / "added.go").write_text("new source\n")
        (self.leaf / "script").write_text("#!/bin/sh\n")
        (self.leaf / "script").chmod(0o755)
        (self.leaf / "var").mkdir()
        (self.leaf / "var/session.jsonl").write_text("private runtime\n")
        records = self.export()
        self.assertEqual((self.destination / "main.go").read_text(), "working tree edit\n")
        self.assertTrue((self.destination / "added.go").exists())
        self.assertEqual((self.destination / "script").stat().st_mode & 0o777, 0o755)
        self.assertFalse((self.destination / "var").exists())
        self.assertFalse((self.destination / "two").exists())
        self.assertFalse(any(record["path"].startswith("../") for record in records))

    def test_current_deletion_is_not_restored_from_index(self):
        (self.leaf / "main.go").unlink()
        self.export()
        self.assertFalse((self.destination / "main.go").exists())

    def test_ignored_go_source_cannot_be_silently_omitted(self):
        (self.leaf / ".gitignore").write_text("*.go\n")
        (self.leaf / "broken.go").write_text("package main\nfunc broken(\n")
        with self.assertRaisesRegex(ValueError, "ignored files may affect.*broken.go"):
            self.export()

    def test_ignored_embed_bytes_cannot_be_silently_omitted(self):
        (self.leaf / "main.go").write_text('package main\nimport _ "embed"\n//go:embed payload.dat\nvar payload string\n')
        (self.leaf / ".gitignore").write_text("*.dat\n")
        (self.leaf / "payload.dat").write_text("compiled into the real program\n")
        with self.assertRaisesRegex(ValueError, "ignored files may affect.*payload.dat"):
            self.export()

    def test_cross_component_symlink_fails_closed(self):
        (self.leaf / "leak.go").symlink_to("../two/private.go")
        with self.assertRaisesRegex(ValueError, "cross-component"):
            self.export()

    def test_internal_relative_symlink_keeps_its_own_component_target(self):
        (self.leaf / "alias").symlink_to("main.go")
        self.export()
        self.assertEqual(os.readlink(self.destination / "alias"), "main.go")
        self.assertEqual((self.destination / "alias").read_text(), "original\n")


class EnvironmentTests(unittest.TestCase):
    def test_ambient_authority_does_not_enter_standalone_checks(self):
        with patch.dict(os.environ, {"OPENAI_API_KEY": "private", "ASK": "/ambient/ask",
                                    "BRIEF_PATH": "/private/skills", "GOFLAGS": "-modfile=/outside.mod",
                                    "GOWORK": "/outside/go.work", "HOME": "/private/home"}):
            env = CHECK["isolated_environment"](Path("/test/home"), Path("/test/tmp"),
                                                 Path("/test/bin"), {"GOCACHE": "/cache/go", "GOMODCACHE": "/cache/mod"})
        for name in ("OPENAI_API_KEY", "ASK", "BRIEF_PATH"):
            self.assertNotIn(name, env)
        self.assertEqual(env["HOME"], "/test/home")
        self.assertEqual(env["GOFLAGS"], "")
        self.assertEqual(env["GOWORK"], "off")
        self.assertNotIn("/private", env["PATH"])

    def test_ply_rejection_fixtures_cannot_nominate_real_cli_or_auth_state(self):
        with tempfile.TemporaryDirectory(prefix="bench-ply-sentinel-test-") as scratch:
            root = Path(scratch).resolve()
            with patch.dict(os.environ, {"PLY_EVAL_CLAUDE": "/ambient/claude", "PLY_EVAL_PI": "/ambient/pi",
                                        "PLY_EVAL_AUTH_ROOT": "/private/auth", "CLAUDE_CONFIG_DIR": "/private/claude",
                                        "PI_CODING_AGENT_DIR": "/private/pi", "ANTHROPIC_API_KEY": "private"}):
                base = CHECK["isolated_environment"](root / "home", root / "tmp", Path("/usr/bin"),
                                                     {"GOCACHE": "/cache/go", "GOMODCACHE": "/cache/mod"})
                env = CHECK["ply_evaluation_environment"](base, root / "sentinels")
            for name in ("PLY_EVAL_AUTH_ROOT", "CLAUDE_CONFIG_DIR", "PI_CODING_AGENT_DIR", "ANTHROPIC_API_KEY"):
                self.assertNotIn(name, env)
            for name in ("CLAUDE", "PI"):
                key = "PLY_EVAL_" + name
                self.assertNotIn(key, base)
                executable = Path(env[key])
                self.assertEqual(executable.parent, root / "sentinels")
                result = subprocess.run([str(executable), "--version"], env=env, capture_output=True, text=True)
                self.assertEqual(result.returncode, 125)
                self.assertEqual(result.stdout, "")
                self.assertIn("execution is forbidden", result.stderr)

    @unittest.skipUnless(sys.platform == "darwin", "Darwin native mktemp precedence")
    def test_draft_native_temp_boundary_preserves_trusted_and_unsafe_fixture_roots(self):
        source_parent, native_root = CHECK["darwin_draft_layout"]()
        with tempfile.TemporaryDirectory(prefix="bench-draft-layout-test-", dir=source_parent) as scratch:
            source = Path(scratch).resolve() / "source"
            source.mkdir()
            temporary = Path(scratch) / "tmp"
            temporary.mkdir()
            env = CHECK["isolated_environment"](Path(scratch) / "home", native_root,
                                                 Path("/usr/bin"), {"GOCACHE": "/cache/go", "GOMODCACHE": "/cache/mod"})
            actual = Path(subprocess.check_output(["/usr/bin/mktemp", "-d"], env=env, text=True).strip())
            try:
                unsafe_verifier = actual.resolve() / "unsafe-state/verifier"
                trusted_verifier = source / ".draft-test-state/verifier"
                self.assertEqual(actual.resolve().parent, Path(env["TMPDIR"]).resolve())
                self.assertIn(native_root, unsafe_verifier.parents)
                self.assertNotIn(native_root, trusted_verifier.parents)
            finally:
                actual.rmdir()


class CheckPlanTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="bench-check-plan-test-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.sequence = 0
        self.components = {name: {"name": name, "module": None if name in ("agent", "draft") else name,
                                  "path": "tools/" + name, "commands": [{"name": name, "package": "."}]}
                           for name in ("agent", "draft", "cite", "may", "cage", "ply", "weave")}
        (self.root / "components.json").write_text(json.dumps({"components": list(self.components.values())}))

    def new_run(self, selected, **options):
        self.sequence += 1
        run_root = self.root / str(self.sequence)
        run_root.mkdir()
        (run_root / "components.json").write_bytes((self.root / "components.json").read_bytes())
        run = CHECK["CheckRun"](run_root, selected, False, **options)
        self.addCleanup(run.stack.close)
        return run

    def capture_component_commands(self, name, quick):
        run = self.new_run([name], quick=quick)
        commands = []
        with patch.object(run, "build"), patch.object(run, "export", return_value=(self.root, {"PATH": "/usr/bin:/bin"})), \
             patch.object(run, "command", side_effect=lambda label, argv, cwd, env: commands.append((label, argv))):
            run.check_component(name)
        return commands

    def test_quick_go_checks_keep_tests_and_built_commands_but_exclude_supplemental_proofs(self):
        for name in ("cite", "may", "cage", "ply", "weave"):
            with self.subTest(component=name):
                commands = self.capture_component_commands(name, quick=True)
                self.assertEqual(commands, [(name + "-test", ["go", "test", "-count=1", "./..."]),
                                            (name + "-version-" + name, [commands[1][1][0], "version"])])

    def test_full_checks_retain_race_vet_and_component_specific_proofs(self):
        required = {"may": {"may-self-check"}, "cage": {"cage-cross-linux", "cage-cross-windows"},
                    "ply": {"ply-eval-fixtures", "ply-job-tests", "ply-edit-tests"},
                    "weave": {"weave-python-business", "weave-python-protocol", "weave-python-receipts",
                              "weave-python-research_domain", "weave-python-release", "weave-built-smoke"}}
        for name, extras in required.items():
            with self.subTest(component=name):
                labels = {label for label, argv in self.capture_component_commands(name, quick=False)}
                self.assertTrue({name + "-test", name + "-race", name + "-vet", *extras} <= labels)

    def test_quick_shell_checks_retain_the_complete_existing_suite(self):
        for name in ("agent", "draft"):
            with self.subTest(component=name):
                full = [label for label, argv in self.capture_component_commands(name, quick=False)]
                quick = [label for label, argv in self.capture_component_commands(name, quick=True)]
                self.assertEqual(full, quick)
                self.assertIn(name + "-test", quick)
                if name == "draft":
                    self.assertIn("draft-scratch-sync", quick)
                    self.assertIn("draft-lint", quick)

    def test_prerequisites_follow_the_checks_that_will_run(self):
        dependencies = CHECK["verification_dependencies"]
        self.assertEqual(set(dependencies(self.components, ["cite"], quick=True)), {"go", "python3", "git", "sh"})
        self.assertNotIn("cc", dependencies(self.components, ["agent"]))
        self.assertIn("perl", dependencies(self.components, ["draft"], quick=True))
        self.assertIn("perl", dependencies(self.components, ["draft"]))
        self.assertNotIn("perl", dependencies(self.components, ["cite"], quick=True))
        self.assertIn("cc", dependencies(self.components, ["cite"]))
        self.assertNotIn("make", dependencies(self.components, ["cite"]))
        self.assertNotIn("make", dependencies(self.components, ["weave"], quick=True))
        self.assertTrue({"make", "install", "cc"} <= set(dependencies(self.components, ["weave"])))
        integration = dependencies(self.components, list(self.components), integration=True, integration_only=True)
        self.assertTrue({"make", "install"} <= set(integration))
        self.assertNotIn("cc", integration)
        self.assertNotIn("perl", integration)
        self.assertNotIn("jq", integration)

    def test_quick_prepare_works_without_c_compiler_jq_or_make(self):
        run = self.new_run(["cite"], quick=True)
        actual_which = CHECK["shutil"].which
        def limited_path(name):
            return None if name in ("cc", "jq", "make", "bash") else actual_which(name)
        with patch.object(CHECK["shutil"], "which", side_effect=limited_path), \
             patch.object(CHECK["subprocess"], "check_output", return_value=b'{"GOCACHE":"/cache/go","GOMODCACHE":"/cache/mod"}'), \
             patch.object(run, "command"):
            run.prepare()
        self.assertEqual(run.env["CGO_ENABLED"], "0")
        self.assertFalse((run.runtime / "cc").exists())
        self.assertEqual(run.env["GOWORK"], "off")

    def test_missing_full_race_compiler_explains_requirement_before_commands_start(self):
        run = self.new_run(["cite"])
        actual_which = CHECK["shutil"].which
        with patch.object(CHECK["shutil"], "which", side_effect=lambda name: None if name == "cc" else actual_which(name)), \
             patch.object(run, "command") as command:
            with self.assertRaisesRegex(RuntimeError, "cc .*Go race detector"):
                run.prepare()
        command.assert_not_called()

    def test_missing_draft_perl_is_reported_before_dependency_builds(self):
        run = self.new_run(["draft"], quick=True)
        actual_which = CHECK["shutil"].which
        with patch.object(CHECK["shutil"], "which", side_effect=lambda name: None if name == "perl" else actual_which(name)), \
             patch.object(run, "command") as command:
            with self.assertRaisesRegex(RuntimeError, "perl .*Draft prove fixtures"):
                run.prepare()
        command.assert_not_called()

    def test_quick_cli_never_runs_integration_and_marks_its_summary(self):
        calls = []
        run_type = CHECK["CheckRun"]
        with patch.dict(CHECK["main"].__globals__, {"ROOT": self.root}), \
             patch.object(run_type, "prepare", lambda run: setattr(run, "env", {})), \
             patch.object(run_type, "check_component", lambda run, name: calls.append(name)), \
             patch.object(run_type, "command"), patch.object(run_type, "integration") as integration, \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(CHECK["main"](["--quick"]), 0)
        self.assertEqual(calls, list(self.components))
        integration.assert_not_called()
        summary = json.loads(next((self.root / ".checks").glob("*/summary.json")).read_text())
        self.assertEqual(summary["status"], "complete")
        self.assertEqual(summary["mode"], "quick")
        self.assertFalse(summary["integration_requested"])

    def test_default_cli_retains_standalone_and_integration_checks(self):
        calls = []
        run_type = CHECK["CheckRun"]
        with patch.dict(CHECK["main"].__globals__, {"ROOT": self.root}), \
             patch.object(run_type, "prepare", lambda run: setattr(run, "env", {})), \
             patch.object(run_type, "check_component", lambda run, name: calls.append(name)), \
             patch.object(run_type, "command"), patch.object(run_type, "integration") as integration, \
             contextlib.redirect_stdout(io.StringIO()):
            self.assertEqual(CHECK["main"]([]), 0)
        self.assertEqual(calls, list(self.components))
        integration.assert_called_once()

    def test_conflicting_quick_proofs_and_unknown_components_fail_before_a_run(self):
        for arguments in (["--quick", "--integration-only"], ["--quick", "--native-cage"], ["--quick", "missing"]):
            with self.subTest(arguments=arguments), patch.dict(CHECK["main"].__globals__, {"ROOT": self.root}), \
                 contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit) as caught:
                    CHECK["main"](arguments)
                self.assertEqual(caught.exception.code, 2)
        self.assertFalse((self.root / ".checks").exists())

    def test_requirements_cli_is_read_only_and_scoped(self):
        output = io.StringIO()
        with patch.dict(CHECK["main"].__globals__, {"ROOT": self.root}), contextlib.redirect_stdout(output):
            self.assertEqual(CHECK["main"](["--quick", "cite", "--requirements"]), 0)
        self.assertIn("go: Go 1.26+", output.getvalue())
        for unused in ("cc:", "make:", "jq:"):
            self.assertNotIn(unused, output.getvalue())
        self.assertFalse((self.root / ".checks").exists())


if __name__ == "__main__":
    unittest.main()
