"""Exercise source builds, relocatable payloads, and failed-publication safety.

These fixtures run the real builder and Go compiler in disposable repositories.
The architecture checker is a no-op here: its source-policy mutations have their
own suite, while these tests target the build and publication boundary.
"""
import hashlib
import json
import os
from pathlib import Path
import runpy
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch


ROOT = Path(__file__).resolve().parents[2]
BUILD = runpy.run_path(str(ROOT / "scripts/build"))


def snapshot(directory):
    """Include link targets and modes, without following a link out of a build."""
    result = {}
    for path in sorted(directory.rglob("*")):
        name = path.relative_to(directory).as_posix()
        if path.is_symlink():
            result[name] = ("link", os.readlink(path))
        elif path.is_file():
            result[name] = ("file", stat.S_IMODE(path.stat().st_mode), path.read_bytes())
        else:
            result[name] = ("directory",)
    return result


@unittest.skipUnless(shutil.which("go") and shutil.which("git"), "Go and Git build prerequisites")
class BuildWorkflowTests(unittest.TestCase):
    def setUp(self):
        self.scratch = tempfile.TemporaryDirectory(prefix="bench-build-test-")
        self.addCleanup(self.scratch.cleanup)
        self.base = Path(self.scratch.name).resolve()
        self.root = self.base / "checkout"
        (self.root / "scripts").mkdir(parents=True)
        for name in ("build", "check"):
            shutil.copy2(ROOT / "scripts" / name, self.root / "scripts" / name)
        (self.root / "scripts/check-boundaries.py").write_text("# Policy checks have an independent mutation suite.\n")
        self.output = self.base / "output"
        self.components = []
        for name in ("one", "two"):
            leaf = self.root / "tools" / name
            leaf.mkdir(parents=True)
            (leaf / "go.mod").write_text("module example.com/" + name + "\n\ngo 1.26\n")
            (leaf / "main.go").write_text('package main\nimport "fmt"\nfunc main() { fmt.Print(message) }\n')
            (leaf / "value.go").write_text('package main\nconst message = "original-' + name + '"\n')
            (leaf / "LICENSE").write_text("fixture license\n")
            (leaf / (name + ".1")).write_text('.TH ' + name.upper() + ' 1\n')
            (leaf / ".gitignore").write_text("var/\n")
            self.components.append({"name": name, "path": "tools/" + name,
                                    "module": "example.com/" + name,
                                    "source": {"commit": "original-fixture"},
                                    "commands": [{"name": name, "package": "."}]})
        for name in ("agent", "hire", "draft"):
            leaf = self.root / "tools" / name
            leaf.mkdir(parents=True)
            for entry in ("README.md", "LICENSE"):
                shutil.copy2(ROOT / "tools" / name / entry, leaf / entry)
            if name == "draft":
                (leaf / "bin").mkdir()
                shutil.copy2(ROOT / "tools/draft/bin/draft", leaf / "bin/draft")
                shutil.copytree(ROOT / "tools/draft/skills", leaf / "skills")
                module = None
            else:
                for entry in (ROOT / "tools" / name).glob("*.go"):
                    if not entry.name.endswith("_test.go"):
                        shutil.copy2(entry, leaf / entry.name)
                shutil.copy2(ROOT / "tools" / name / "go.mod", leaf / "go.mod")
                module = "github.com/patrickyoung/" + ("bench-hire" if name == "hire" else name)
                if name == "hire":
                    for directory in ("builder", "expert"):
                        shutil.copytree(ROOT / "tools/hire" / directory, leaf / directory)
            self.components.append({"name": name, "path": "tools/" + name,
                                    "module": module, "source": {"commit": "original-fixture"},
                                    "commands": [{"name": name, "package": "."} if module
                                                 else {"name": name, "entry": "bin/" + name}]})
        (self.root / "components.json").write_text(json.dumps({"schema": 1, "components": self.components}))
        subprocess.run(["git", "init", "-q", str(self.root)], check=True)
        subprocess.run(["git", "add", "."], cwd=self.root, check=True)
        subprocess.run(["git", "-c", "user.name=Build Fixture", "-c", "user.email=fixture@example.invalid",
                        "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"], cwd=self.root, check=True)
        self.env = dict(os.environ, GOPROXY="off", GOSUMDB="off")

    def build(self, *names, output=None, success=True):
        result = subprocess.run([sys.executable, str(self.root / "scripts/build"),
                                 "--output", str(output or self.output), *names],
                                cwd=self.base, env=self.env, capture_output=True, text=True)
        if success:
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        else:
            self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def mark_output(self):
        self.output.mkdir()
        (self.output / "build.json").write_text(json.dumps(BUILD["MARKER"]))

    def test_real_build_uses_current_edits_new_files_and_deletions(self):
        leaf = self.root / "tools/one"
        (leaf / "main.go").write_text('package main\nimport "fmt"\nfunc main() { fmt.Print(message + suffix) }\n')
        (leaf / "value.go").unlink()
        (leaf / "new.go").write_text('package main\nconst message = "working-tree"\nconst suffix = "-new"\n')
        (leaf / "var").mkdir()
        (leaf / "var/session.jsonl").write_text("private runtime data\n")
        self.build("one")
        result = subprocess.check_output([str(self.output / "bin/one")], text=True)
        self.assertEqual(result, "working-tree-new")
        self.assertFalse((self.output / "tools/two").exists())
        package = self.output / "tools/one"
        receipt = json.loads((package / "package.json").read_text())
        source = {entry["path"]: entry for entry in receipt["source"]["files"]}
        self.assertIn("new.go", source)
        self.assertNotIn("value.go", source)
        self.assertFalse(any("session" in name or name.startswith("../") for name in source))
        self.assertEqual(source["new.go"]["sha256"], hashlib.sha256((leaf / "new.go").read_bytes()).hexdigest())
        self.assertEqual(receipt["source"]["path"], "tools/one")
        self.assertEqual(receipt["commands"], ["one"])
        for entry in receipt["files"]:
            path = package / entry["path"]
            self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), entry["sha256"])
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), entry["mode"])
        self.assertEqual((package / "LICENSE").read_text(), "fixture license\n")
        self.assertTrue((package / "one.1").is_file())
        self.assertEqual(os.readlink(self.output / "bin/one"), "../tools/one/bin/one")

    def test_failed_second_compile_preserves_every_previous_command(self):
        self.build("one", "two")
        before = snapshot(self.output)
        (self.root / "tools/one/value.go").write_text('package main\nconst message = "new-one"\n')
        (self.root / "tools/two/value.go").write_text("package main\ninvalid syntax !!!\n")
        self.build("one", "two", success=False)
        self.assertEqual(snapshot(self.output), before)
        self.assertEqual(subprocess.check_output([str(self.output / "bin/one")], text=True), "original-one")
        self.assertEqual(subprocess.check_output([str(self.output / "bin/two")], text=True), "original-two")

    def test_explicit_no_cgo_build_records_and_applies_the_compiler_choice(self):
        self.build("one")
        receipt = self.output / "tools/one/package.json"
        self.assertNotIn("build_environment", json.loads(receipt.read_text())["source"])
        self.build("one", "--no-cgo")
        self.assertEqual(json.loads(receipt.read_text())["source"]["build_environment"], {"CGO_ENABLED": "0"})
        metadata = subprocess.check_output(["go", "version", "-m", str(self.output / "bin/one")], text=True)
        self.assertIn("\tbuild\tCGO_ENABLED=0\n", metadata)
        self.assertEqual(subprocess.check_output([str(self.output / "bin/one")], text=True), "original-one")

    def test_compiler_receives_an_export_without_the_checkout_or_sibling(self):
        # Observe the real compiler boundary rather than inferring isolation
        # from the receipt that the builder later writes about that boundary.
        compiler = str(Path(shutil.which("go")).resolve())
        wrappers = self.base / "compiler-probe"
        wrappers.mkdir()
        report = self.base / "compiler-input.json"
        wrapper = wrappers / "go"
        wrapper.write_text("#!" + sys.executable + "\n"
                           "import json, os, sys\nfrom pathlib import Path\n"
                           "if sys.argv[1:2] == ['build']:\n"
                           "    source = Path.cwd().resolve()\n"
                           "    record = {'cwd': str(source), 'sibling': (source.parent / 'two').exists(), "
                           "'git': (source / '.git').exists(), 'gowork': os.environ.get('GOWORK'), "
                           "'provider_key': 'OPENAI_API_KEY' in os.environ}\n"
                           "    Path(" + repr(str(report)) + ").write_text(json.dumps(record))\n"
                           "os.execv(" + repr(compiler) + ", [" + repr(compiler) + ", *sys.argv[1:]])\n")
        wrapper.chmod(0o755)
        self.env.update(PATH=str(wrappers) + os.pathsep + os.environ["PATH"],
                        OPENAI_API_KEY="fixture-must-not-reach-compiler")
        self.build("one")
        observed = json.loads(report.read_text())
        self.assertNotIn(self.root, Path(observed["cwd"]).parents)
        self.assertFalse(observed["sibling"])
        self.assertFalse(observed["git"])
        self.assertEqual(observed["gowork"], "off")
        self.assertFalse(observed["provider_key"])
        self.assertEqual(subprocess.check_output([str(self.output / "bin/one")], text=True), "original-one")

    def test_separate_agent_hire_and_draft_remain_usable_after_relocation(self):
        self.build("agent", "hire", "draft")
        relocated = self.base / "relocated prefix with spaces"
        self.output.rename(relocated)
        self.assertEqual(sorted(p.name for p in (relocated / "bin").iterdir()), ["agent", "draft", "hire"])
        home, work = self.base / "home", self.base / "work"
        home.mkdir(); work.mkdir()
        env = {"PATH": str(relocated / "bin") + ":/usr/bin:/bin:/usr/sbin:/sbin",
               "HOME": str(home), "TMPDIR": str(self.base)}
        for command in (["agent", "version"], ["hire", "new", "-home", "worker"], ["agent", "check", "worker"],
                        ["draft", "version"], ["draft", "new", "project"]):
            result = subprocess.run(command, cwd=work, env=env, text=True, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        native = relocated / "tools/agent/bin/agent"
        self.assertFalse(native.read_bytes().startswith(b"#!"))
        result = subprocess.run([str(native), "-c"], cwd=work, env=env, text=True, capture_output=True)
        self.assertEqual(result.returncode, 125)
        self.assertIn("expects -c SCRIPT", result.stderr)
        self.assertEqual((work / "project/DESIGN.md").read_bytes(),
                         (ROOT / "tools/draft/skills/draft/references/template.md").read_bytes())
        draft = relocated / "tools/draft"
        receipt = json.loads((draft / "package.json").read_text())
        names = {entry["path"] for entry in receipt["files"]}
        self.assertTrue({"skills/draft/SKILL.md", "skills/draft/references/template.md",
                         "skills/draft/references/tools.md"}.issubset(names))
        self.assertEqual(receipt["mutable_paths"], ["skills/draft/references/tools.md"])
        agent_receipt = json.loads((relocated / "tools/agent/package.json").read_text())
        self.assertNotIn("bin/agent-action-shell", {entry["path"] for entry in agent_receipt["files"]})
        self.assertEqual(agent_receipt["commands"], ["agent"])

    def test_weigh_examples_survive_build_install_and_relocation(self):
        # Package the real leaf: its visual wrapper imports a sibling module,
        # and its checker must remain executable when composed by literal argv.
        leaf = self.root / "tools/weigh"
        shutil.copytree(ROOT / "tools/weigh", leaf,
                        ignore=shutil.ignore_patterns("__pycache__", "*.pyc", "*.pyo"))
        self.components.append({"name": "weigh", "path": "tools/weigh",
                                "module": "github.com/patrickyoung/weigh",
                                "source": {"commit": "original-fixture"},
                                "commands": [{"name": "weigh", "package": "."}]})
        (self.root / "components.json").write_text(json.dumps({"schema": 1, "components": self.components}))
        examples = leaf / "examples"
        expected = {"examples/" + name: value for name, value in snapshot(examples).items()
                    if value[0] == "file"}
        cache = examples / "visual-check/__pycache__"
        cache.mkdir()
        (cache / "visual_contract.cpython-314.pyc").write_bytes(b"stale cache")
        (examples / "semantic-check/check.pyc").write_bytes(b"stale bytecode")
        (examples / "semantic-check/check.pyo").write_bytes(b"stale optimized bytecode")
        self.build("weigh")
        relocated_build = self.base / "build moved with spaces"
        self.output.rename(relocated_build)
        package = relocated_build / "tools/weigh"
        receipt = json.loads((package / "package.json").read_text())
        payload = {entry["path"]: entry for entry in receipt["files"]
                   if entry["path"].startswith("examples/")}
        self.assertEqual(set(payload), set(expected))
        for name, (_, mode, data) in expected.items():
            self.assertEqual((package / name).read_bytes(), data)
            self.assertEqual(payload[name]["sha256"], hashlib.sha256(data).hexdigest())
            self.assertEqual(payload[name]["mode"], mode)
        self.assertFalse((package / "examples/visual-check/__pycache__").exists())
        self.assertEqual(receipt["commands"], ["weigh"])

        for name in ("install", "uninstall", "install_support.py"):
            shutil.copy2(ROOT / "scripts" / name, self.root / "scripts" / name)
        prefix = self.base / "installed prefix"
        result = subprocess.run([sys.executable, str(self.root / "scripts/install"), "weigh",
                                 "--from-build", str(relocated_build), "--prefix", str(prefix)],
                                env=self.env, text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        relocated = self.base / "installed and moved with spaces"
        prefix.rename(relocated)
        # Remove both original payload sources: no runtime path may reach back.
        shutil.rmtree(leaf)
        shutil.rmtree(relocated_build)
        installed = relocated / "lib/bench-tools/weigh"
        actual = {"examples/" + name: value for name, value in snapshot(installed / "examples").items()
                  if value[0] == "file"}
        self.assertEqual(actual, expected)
        self.assertEqual(sorted(p.name for p in (relocated / "bin").iterdir()), ["weigh"])
        home, work = self.base / "weigh-home", self.base / "weigh-work"
        home.mkdir(); work.mkdir()
        python_bin = self.base / "python-bin"
        python_bin.mkdir()
        (python_bin / "python3").symlink_to(Path(sys.executable).resolve())
        env = {"PATH": str(python_bin) + ":/usr/bin:/bin:/usr/sbin:/sbin",
               "HOME": str(home), "TMPDIR": str(self.base), "PYTHONDONTWRITEBYTECODE": "1"}
        commands = [[str(relocated / "bin/weigh"), "version"]]
        for entry in ("semantic-check/check.py", "semantic-check/evaluate.py",
                      "visual-check/check-current.py", "visual-check/observe.py"):
            path = installed / "examples" / entry
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o755)
            commands.append([str(path), "--help"])
        # The packaged tests select only local executable fixtures; running
        # them after relocation proves the usable recipe, not provider quality.
        for directory in ("semantic-check", "visual-check"):
            commands.append([sys.executable, "-m", "unittest", "discover", "-s",
                             str(installed / "examples" / directory), "-p", "test_*.py"])
        for command in commands:
            with self.subTest(command=command):
                result = subprocess.run(command, cwd=work, env=env, text=True, capture_output=True)
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual({"examples/" + name: value for name, value in
                          snapshot(installed / "examples").items() if value[0] == "file"}, expected)
        result = subprocess.run([sys.executable, str(self.root / "scripts/uninstall"), "weigh",
                                 "--prefix", str(relocated)], env=env, text=True, capture_output=True)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse(installed.exists())

    def test_nonempty_unmanaged_output_is_untouched(self):
        self.output.mkdir()
        (self.output / "valuable.txt").write_text("keep this\n")
        before = snapshot(self.output)
        result = self.build("agent", success=False)
        self.assertIn("unmanaged", result.stderr)
        self.assertEqual(snapshot(self.output), before)

    def test_output_symlink_does_not_write_into_its_target(self):
        target = self.base / "foreign"
        target.mkdir()
        (target / "valuable.txt").write_text("keep this\n")
        self.output.symlink_to(target, target_is_directory=True)
        before = snapshot(target)
        self.build("agent", success=False)
        self.assertTrue(self.output.is_symlink())
        self.assertEqual(snapshot(target), before)

    def test_managed_subdirectory_symlinks_do_not_write_outside_build(self):
        for name in ("tools", "bin"):
            with self.subTest(name=name):
                self.mark_output()
                target = self.base / ("foreign-" + name)
                target.mkdir()
                (target / "valuable.txt").write_text("keep this\n")
                (self.output / name).symlink_to(target, target_is_directory=True)
                before = snapshot(target)
                self.build("agent", success=False)
                self.assertEqual(snapshot(target), before)
                self.assertTrue((self.output / name).is_symlink())
                shutil.rmtree(self.output)

    def test_foreign_command_refuses_before_any_selected_package_changes(self):
        self.build("agent")
        (self.output / "bin/draft").write_text("foreign executable\n")
        before = snapshot(self.output)
        self.build("agent", "draft", success=False)
        self.assertEqual(snapshot(self.output), before)

    def test_foreign_command_symlink_is_not_replaced(self):
        self.build("agent")
        target = self.base / "foreign-command"
        target.write_text("keep this\n")
        (self.output / "bin/draft").symlink_to(target)
        before = snapshot(self.output)
        self.build("agent", "draft", success=False)
        self.assertEqual(snapshot(self.output), before)
        self.assertEqual(target.read_text(), "keep this\n")

    def test_arbitrary_package_json_does_not_grant_directory_ownership(self):
        self.build("agent")
        destination = self.output / "tools/draft"
        destination.mkdir()
        (destination / "valuable.txt").write_text("keep this\n")
        for receipt in ({}, [], None, {"schema": 1, "name": "agent", "commands": ["agent"], "files": [{}]}):
            with self.subTest(receipt=receipt):
                (destination / "package.json").write_text(json.dumps(receipt) + "\n")
                before = snapshot(self.output)
                result = self.build("draft", success=False)
                self.assertNotIn("Traceback", result.stderr)
                self.assertEqual(snapshot(self.output), before)

    def test_publication_error_rolls_back_payload_and_public_command(self):
        self.build("agent")
        before = snapshot(self.output)
        staged = self.base / "staged"
        (staged / "tools").mkdir(parents=True)
        (staged / "bin").mkdir()
        shutil.copytree(self.output / "tools/agent", staged / "tools/agent")
        (staged / "tools/agent/bin/agent").write_text("replacement that must be rolled back\n")
        real_rename = Path.rename

        def fail_command_publication(path, destination):
            if path == staged / "bin/agent":
                raise OSError("injected command publication failure")
            return real_rename(path, destination)

        agent = next(c for c in self.components if c["name"] == "agent")
        with patch.object(Path, "rename", fail_command_publication):
            with self.assertRaisesRegex(OSError, "injected command publication failure"):
                BUILD["publish"](self.output, staged, [agent])
        self.assertEqual(snapshot(self.output), before)


if __name__ == "__main__":
    unittest.main()
