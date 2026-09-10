"""Protect the independent-export and credential-isolation verification seams."""
import os
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


if __name__ == "__main__":
    unittest.main()
