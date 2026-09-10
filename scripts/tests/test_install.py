"""Installer ownership, actual command layout, integrity and rollback tests."""

import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


SCRIPTS = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("install_support", SCRIPTS / "install_support.py")
INSTALL = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(INSTALL)


class InstallTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="bench-install-test-")
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.build = self.root / "build"
        self.prefix = self.root / "prefix with spaces"

    def package(self, name="rules", marker="v1", extra=None, commands=None, mutable=None):
        directory = self.build / "tools" / name
        if directory.exists():
            shutil.rmtree(directory)
        (directory / "bin").mkdir(parents=True)
        commands = commands or [name]
        for command in commands:
            path = directory / "bin" / command
            path.write_text(f"#!/bin/sh\nprintf '%s\\n' '{marker}'\n")
            path.chmod(0o755)
        (directory / "LICENSE").write_text("test license\n")
        (directory / "README.md").write_text("test documentation\n")
        for relative, value in (extra or {}).items():
            path = directory / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(value)
            if relative.startswith("bin/"):
                path.chmod(0o755)
        records = []
        for path in sorted(directory.rglob("*")):
            if path.is_file():
                records.append({"path": path.relative_to(directory).as_posix(),
                                "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
                                "mode": format(path.stat().st_mode & 0o777, "04o")})
        receipt = {"schema": 1, "name": name, "commands": commands, "files": records,
                   "source": {"fixture": marker}}
        if mutable is not None:
            receipt["mutable_paths"] = mutable
        (directory / "package.json").write_text(json.dumps(receipt))
        return directory

    def command(self, script, *arguments, ok=True):
        result = subprocess.run([sys.executable, str(SCRIPTS / script), *arguments,
                                 "--prefix", str(self.prefix)], text=True, capture_output=True)
        if ok:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def install(self, *names, ok=True):
        return self.command("install", *names, "--from-build", str(self.build), ok=ok)

    def run_tool(self, name):
        return subprocess.check_output([str(self.prefix / "bin" / name)], text=True).strip()

    def test_install_update_repeat_and_uninstall_actual_command(self):
        self.package()
        self.install("rules")
        self.assertEqual(self.run_tool("rules"), "v1")
        self.assertEqual(os.readlink(self.prefix / "bin/rules"), "../lib/bench-tools/rules/bin/rules")
        self.package(marker="v2")
        self.install("rules")
        self.install("rules")
        self.assertEqual(self.run_tool("rules"), "v2")
        self.command("uninstall", "rules")
        self.assertFalse(os.path.lexists(self.prefix / "bin/rules"))
        self.assertFalse((self.prefix / "lib/bench-tools/rules").exists())
        self.command("uninstall")

    def test_install_selects_only_requested_tool_and_needs_no_compiler(self):
        self.package("rules")
        self.install("rules")
        self.assertEqual(sorted(path.name for path in (self.prefix / "bin").iterdir()), ["rules"])
        self.assertFalse((self.prefix / "lib/bench-tools/ask").exists())

    def test_unmanaged_command_blocks_whole_install(self):
        self.package("rules")
        self.package("ask")
        (self.prefix / "bin").mkdir(parents=True)
        foreign = self.prefix / "bin/ask"
        foreign.write_text("my existing command")
        result = self.install("rules", "ask", ok=False)
        self.assertIn("unmanaged command", result.stderr)
        self.assertEqual(foreign.read_text(), "my existing command")
        self.assertFalse((self.prefix / "lib/bench-tools/rules").exists())

    def test_broken_unmanaged_link_is_not_overwritten(self):
        self.package()
        (self.prefix / "bin").mkdir(parents=True)
        (self.prefix / "bin/rules").symlink_to("missing")
        self.install("rules", ok=False)
        self.assertEqual(os.readlink(self.prefix / "bin/rules"), "missing")

    def test_modified_installed_payload_refuses_update_and_uninstall(self):
        self.package()
        self.install("rules")
        path = self.prefix / "lib/bench-tools/rules/README.md"
        path.write_text("user content")
        self.assertIn("checksum mismatch", self.install("rules", ok=False).stderr)
        self.assertIn("checksum mismatch", self.command("uninstall", "rules", ok=False).stderr)
        self.assertEqual(path.read_text(), "user content")
        self.assertEqual(self.run_tool("rules"), "v1")

    def test_modified_installed_receipt_is_not_new_authority(self):
        self.package()
        self.install("rules")
        path = self.prefix / "lib/bench-tools/rules/package.json"
        path.write_text(path.read_text() + " ")
        self.assertIn("receipt changed", self.command("uninstall", "rules", ok=False).stderr)
        self.assertTrue(path.exists())

    def test_replaced_managed_link_is_not_overwritten(self):
        self.package()
        self.install("rules")
        path = self.prefix / "bin/rules"
        path.unlink()
        path.symlink_to("/usr/bin/true")
        self.assertIn("missing or changed", self.install("rules", ok=False).stderr)
        self.assertIn("missing or changed", self.command("uninstall", "rules", ok=False).stderr)
        self.assertEqual(os.readlink(path), "/usr/bin/true")

    def test_build_checksum_and_unlisted_files_are_checked_before_prefix_creation(self):
        directory = self.package()
        (directory / "bin/rules").write_text("tampered")
        self.assertIn("checksum mismatch", self.install("rules", ok=False).stderr)
        self.assertFalse(self.prefix.exists())
        directory = self.package()
        (directory / "secret").write_text("not in receipt")
        self.assertIn("inventory mismatch", self.install("rules", ok=False).stderr)
        self.assertFalse(self.prefix.exists())

    def test_draft_generated_reference_survives_update_then_uninstalls(self):
        reference = INSTALL.DRAFT_REFERENCE
        self.package("draft", extra={reference: "built reference"}, mutable=[reference])
        result = self.install("draft")
        self.assertIn("BRIEF_PATH=", result.stdout)
        self.assertIn("draft sync", result.stdout)
        path = self.prefix / "lib/bench-tools/draft" / reference
        path.write_text("generated by draft sync")
        self.package("draft", marker="v2", extra={reference: "new built reference"}, mutable=[reference])
        self.install("draft")
        self.assertEqual(path.read_text(), "generated by draft sync")
        self.assertEqual(self.run_tool("draft"), "v2")
        self.command("uninstall", "draft")
        self.assertFalse(path.exists())

    def test_draft_mutability_does_not_allow_tampered_build_payload(self):
        reference = INSTALL.DRAFT_REFERENCE
        directory = self.package("draft", extra={reference: "built reference"}, mutable=[reference])
        (directory / reference).write_text("tampered build")
        self.assertIn("checksum mismatch", self.install("draft", ok=False).stderr)

    def test_draft_shell_guidance_preserves_default_and_custom_skill_paths(self):
        reference = INSTALL.DRAFT_REFERENCE
        self.package("draft", extra={reference: "built reference"}, mutable=[reference])
        result = self.install("draft")
        guidance = next(line.strip() for line in result.stdout.splitlines()
                        if line.strip().startswith("export BRIEF_PATH="))
        private_home = str(self.root / "home with spaces")
        defaults = f".claude/skills:{private_home}/.claude/skills:{private_home}/.brief/skills"
        for previous in (None, "", "/skills with spaces:/another skills"):
            with self.subTest(previous=previous):
                env = {"HOME": private_home, "PATH": "/usr/bin:/bin"}
                if previous is not None:
                    env["BRIEF_PATH"] = previous
                actual = subprocess.check_output(
                    ["/bin/sh", "-c", guidance + '\nprintf "%s" "$BRIEF_PATH"'],
                    env=env, text=True)
                self.assertEqual(actual, str(self.prefix / "lib/bench-tools/draft/skills")
                                 + ":" + (previous or defaults))

    def test_draft_restrictive_umask_reference_survives_update_and_uninstall(self):
        reference = INSTALL.DRAFT_REFERENCE
        self.package("draft", extra={reference: "built reference"}, mutable=[reference])
        self.install("draft")
        path = self.prefix / "lib/bench-tools/draft" / reference
        path.write_text("generated with umask 077")
        path.chmod(0o600)
        self.package("draft", marker="v2", extra={reference: "new built reference"}, mutable=[reference])
        self.install("draft")
        self.assertEqual(path.read_text(), "generated with umask 077")
        self.assertEqual(path.stat().st_mode & 0o777, 0o600)
        self.assertEqual(self.run_tool("draft"), "v2")
        self.command("uninstall", "draft")
        self.assertFalse(path.exists())

    def test_draft_mutable_mode_cannot_expand_permissions_or_be_executable(self):
        reference = INSTALL.DRAFT_REFERENCE
        self.package("draft", extra={reference: "built reference"}, mutable=[reference])
        self.install("draft")
        path = self.prefix / "lib/bench-tools/draft" / reference
        for mode in (0o666, 0o744, 0o200):
            with self.subTest(mode=oct(mode)):
                path.chmod(mode)
                self.assertIn("file mode mismatch", self.install("draft", ok=False).stderr)
                self.assertIn("file mode mismatch", self.command("uninstall", "draft", ok=False).stderr)
        path.chmod(0o644)
        self.command("uninstall", "draft")

    def test_draft_build_mutable_reference_mode_stays_strict(self):
        reference = INSTALL.DRAFT_REFERENCE
        directory = self.package("draft", extra={reference: "built reference"}, mutable=[reference])
        (directory / reference).chmod(0o600)
        self.assertIn("file mode mismatch", self.install("draft", ok=False).stderr)
        self.assertFalse(self.prefix.exists())

    def test_agent_private_helper_stays_with_actual_script(self):
        directory = self.package("agent", extra={"bin/agent-action-shell": "#!/bin/sh\necho helper\n"})
        self.install("agent")
        self.assertTrue((self.prefix / "lib/bench-tools/agent/bin/agent-action-shell").is_file())
        self.assertFalse(os.path.lexists(self.prefix / "bin/agent-action-shell"))
        self.assertEqual((self.prefix / "lib/bench-tools/agent/bin/agent-action-shell").read_bytes(),
                         (directory / "bin/agent-action-shell").read_bytes())

    def test_symlinked_managed_directory_cannot_redirect_install(self):
        self.package()
        self.prefix.mkdir()
        outside = self.root / "outside"
        outside.mkdir()
        (self.prefix / "lib").symlink_to(outside)
        self.assertIn("symlink", self.install("rules", ok=False).stderr)
        self.assertEqual(list(outside.iterdir()), [])

    def test_package_traversal_and_escaping_symlink_are_rejected(self):
        directory = self.package()
        receipt = json.loads((directory / "package.json").read_text())
        receipt["files"][0]["path"] = "../outside"
        (directory / "package.json").write_text(json.dumps(receipt))
        self.assertIn("unsafe package path", self.install("rules", ok=False).stderr)
        directory = self.package()
        (directory / "outside").symlink_to("../../../../outside")
        receipt = json.loads((directory / "package.json").read_text())
        receipt["files"].append({"path": "outside", "symlink": "../../../../outside"})
        (directory / "package.json").write_text(json.dumps(receipt))
        self.assertIn("symlink escapes", self.install("rules", ok=False).stderr)

    def test_mid_transaction_failure_rolls_back_tools_links_and_ownership(self):
        self.package("rules")
        self.package("ask")
        self.install("rules", "ask")
        state = (self.prefix / "lib/bench-tools/installed.json").read_bytes()
        sources = {name: self.package(name, marker="v2") for name in ("rules", "ask")}
        original_replace = os.replace
        failed = False

        def fail_once(source, destination):
            nonlocal failed
            if not failed and Path(destination) == self.prefix / "lib/bench-tools/installed.json":
                failed = True
                raise OSError("simulated full filesystem at ownership commit")
            return original_replace(source, destination)

        with patch.object(INSTALL.os, "replace", side_effect=fail_once):
            with self.assertRaisesRegex(OSError, "simulated full filesystem"):
                INSTALL.transact(self.prefix, sources)
        self.assertTrue(failed)
        self.assertEqual(self.run_tool("rules"), "v1")
        self.assertEqual(self.run_tool("ask"), "v1")
        self.assertEqual((self.prefix / "lib/bench-tools/installed.json").read_bytes(), state)
        self.assertEqual(list((self.prefix / "lib/bench-tools").glob(".stage-*")), [])
        self.install("rules", "ask")
        self.assertEqual(self.run_tool("rules"), "v2")

    def test_uninstall_one_component_preserves_other_tool_and_unmanaged_files(self):
        self.package("rules")
        self.package("ask")
        self.install("rules", "ask")
        (self.prefix / "bin/my-script").write_text("personal")
        self.command("uninstall", "rules")
        self.assertEqual(self.run_tool("ask"), "v1")
        self.assertEqual((self.prefix / "bin/my-script").read_text(), "personal")

    def test_concurrent_installer_refuses_before_replacing_files(self):
        self.package()
        self.install("rules")
        self.package(marker="v2")
        with INSTALL.locked(self.prefix / "lib/bench-tools"):
            self.assertIn("another installation", self.install("rules", ok=False).stderr)
        self.assertEqual(self.run_tool("rules"), "v1")

    def test_failed_rollback_keeps_backup_for_recovery(self):
        self.package()
        self.install("rules")
        source = self.package(marker="v2")
        original_replace = os.replace
        failed = False

        def fail_commit_and_restore(source, destination):
            nonlocal failed
            if not failed and Path(destination) == self.prefix / "lib/bench-tools/installed.json":
                failed = True
                raise OSError("commit failed")
            if failed and Path(source).parent.name == "old" and Path(source).name == "rules":
                raise OSError("cannot restore original directory")
            return original_replace(source, destination)

        with patch.object(INSTALL.os, "replace", side_effect=fail_commit_and_restore):
            with self.assertRaisesRegex(INSTALL.RecoveryError, "Previous files retained"):
                INSTALL.transact(self.prefix, {"rules": source})
        stages = list((self.prefix / "lib/bench-tools").glob(".stage-*"))
        self.assertEqual(len(stages), 1)
        old = stages[0] / "old/rules"
        self.assertEqual(subprocess.check_output([str(old / "bin/rules")], text=True).strip(), "v1")
        self.assertIn("ownership receipt", self.install("rules", ok=False).stderr)


if __name__ == "__main__":
    unittest.main()
