"""Release bundles retain independent package integrity and source matching."""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
TOOLS = ("hire", "agent", "ask", "brief", "ply", "cage", "record", "trail", "hone")


@unittest.skipUnless(shutil.which("go") and shutil.which("git"), "Go and Git release prerequisites")
class RuntimePackageTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.template = tempfile.TemporaryDirectory(prefix="bench-runtime-template-")
        cls.addClassCleanup(cls.template.cleanup)
        base = Path(cls.template.name).resolve()
        cls.source, cls.packages = base / "checkout", base / "build"
        (cls.source / "scripts").mkdir(parents=True)
        for name in ("package-runtime", "build", "check", "install", "install_support.py"):
            shutil.copy2(ROOT / "scripts" / name, cls.source / "scripts" / name)
        (cls.source / "scripts/check-boundaries.py").write_text("# Source policy has its own suite.\n")
        components = []
        for name in TOOLS:
            leaf = cls.source / "tools" / name
            leaf.mkdir(parents=True)
            (leaf / "go.mod").write_text("module example.invalid/" + name + "\n\ngo 1.26\n")
            (leaf / "main.go").write_text('package main\nimport "fmt"\nfunc main() { fmt.Println("' + name + '") }\n')
            (leaf / "LICENSE").write_text("fixture license\n")
            components.append({"name": name, "path": "tools/" + name, "module": "example.invalid/" + name,
                               "source": {"commit": "fixture-import"}, "commands": [{"name": name, "package": "."}]})
        (cls.source / "components.json").write_text(json.dumps({"components": components}))
        (cls.source / "README.md").write_text("root documentation\n")
        subprocess.run(["git", "init", "-q", str(cls.source)], check=True)
        subprocess.run(["git", "add", "."], cwd=cls.source, check=True)
        subprocess.run(["git", "-c", "user.name=Release Fixture", "-c", "user.email=fixture@example.invalid",
                        "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"], cwd=cls.source, check=True)
        cls.env = dict(os.environ, GOPROXY="off", GOSUMDB="off")
        built = subprocess.run([sys.executable, str(cls.source / "scripts/build"), "--output", str(cls.packages)],
                               env=cls.env, capture_output=True, text=True, timeout=90)
        if built.returncode:
            raise AssertionError(built.stdout + built.stderr)

    def setUp(self):
        self.scratch = tempfile.TemporaryDirectory(prefix="bench-runtime-package-")
        self.addCleanup(self.scratch.cleanup)
        self.work = Path(self.scratch.name).resolve()
        self.root, self.build = self.work / "checkout", self.work / "build"
        shutil.copytree(self.source, self.root)
        shutil.copytree(self.packages, self.build, symlinks=True)
        self.output = self.work / "release with spaces" / "builder.tar.gz"

    def package(self, output=None):
        return subprocess.run([sys.executable, str(self.root / "scripts/package-runtime"),
                               "--from-build", str(self.build), "--output", str(output or self.output)],
                              env=self.env, capture_output=True, text=True, timeout=45)

    def commit(self, message):
        subprocess.run(["git", "add", "."], cwd=self.root, check=True)
        subprocess.run(["git", "-c", "user.name=Release Fixture", "-c", "user.email=fixture@example.invalid",
                        "-c", "commit.gpgsign=false", "commit", "-qm", message], cwd=self.root, check=True)
        return subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=self.root, text=True).strip()

    def test_archive_is_reproducible_and_installs_unchanged_packages(self):
        (self.build / "private-build-note").write_text("do not publish\n")
        first = self.package()
        self.assertEqual(first.returncode, 0, first.stdout + first.stderr)
        second = self.work / "second.tar.gz"
        repeated = self.package(second)
        self.assertEqual(repeated.returncode, 0, repeated.stdout + repeated.stderr)
        self.assertEqual(self.output.read_bytes(), second.read_bytes())
        external = json.loads(Path(str(self.output) + ".json").read_text())
        self.assertEqual(external["sha256"], hashlib.sha256(self.output.read_bytes()).hexdigest())
        self.assertEqual(external["size"], self.output.stat().st_size)
        self.assertEqual(Path(str(self.output) + ".sha256").read_text(), external["sha256"] + "  builder.tar.gz\n")
        unpacked = self.work / "unpacked"
        expected = {"tools/" + name + "/" + path.relative_to(self.build / "tools" / name).as_posix()
                    for name in TOOLS for path in (self.build / "tools" / name).rglob("*") if path.is_file()}
        with tarfile.open(self.output, "r:gz") as archive:
            self.assertEqual(set(archive.getnames()), expected | {"runtime.json"})
            metadata = json.load(archive.extractfile("runtime.json"))
            self.assertEqual(metadata, {key: value for key, value in external.items() if key not in ("sha256", "size")})
            self.assertEqual(metadata["tools"], list(TOOLS))
            for member in archive.getmembers():
                self.assertTrue(member.isfile())
                self.assertEqual(member.uid, 0)
                self.assertEqual(member.mtime, 0)
                path = unpacked / member.name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(archive.extractfile(member).read())
                path.chmod(member.mode)
            for name in TOOLS:
                receipt = self.build / "tools" / name / "package.json"
                self.assertEqual(metadata["packages"][name], hashlib.sha256(receipt.read_bytes()).hexdigest())
                self.assertEqual(metadata["sources"][name], json.loads(receipt.read_text())["source"]["files_sha256"])
        prefix = self.work / "installed"
        installed = subprocess.run([sys.executable, str(self.root / "scripts/install"), *TOOLS,
                                    "--from-build", str(unpacked), "--prefix", str(prefix)],
                                   env=self.env, capture_output=True, text=True, timeout=30)
        self.assertEqual(installed.returncode, 0, installed.stdout + installed.stderr)
        for name in TOOLS:
            self.assertEqual(subprocess.check_output([str(prefix / "bin" / name)], text=True).strip(), name)

    def test_component_edit_rejects_stale_build_without_writing_an_archive(self):
        leaf = self.root / "tools/agent/main.go"
        leaf.write_text(leaf.read_text() + "// changed after building\n")
        result = self.package()
        self.assertEqual(result.returncode, 1)
        self.assertIn("source does not match", result.stderr)
        self.assertFalse(self.output.exists())

    def test_root_documentation_changes_do_not_invalidate_identical_component_source(self):
        (self.root / "README.md").write_text("updated installation instructions\n")
        self.commit("root docs only")
        result = self.package()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)

    def test_mixed_build_revisions_are_rejected(self):
        (self.root / "README.md").write_text("another root-only commit\n")
        revision = self.commit("new release docs")
        receipt = self.build / "tools/agent/package.json"
        value = json.loads(receipt.read_text())
        value["source"]["repository_revision"] = revision
        receipt.write_text(json.dumps(value))
        result = self.package()
        self.assertEqual(result.returncode, 1)
        self.assertIn("share one source commit", result.stderr)
        self.assertFalse(self.output.exists())

    def test_rebuilding_dirty_component_cannot_publish_it_under_the_old_commit(self):
        leaf = self.root / "tools/agent/main.go"
        leaf.write_text(leaf.read_text() + "// included in the rebuilt receipt but not its commit\n")
        built = subprocess.run([sys.executable, str(self.root / "scripts/build"), "agent",
                                "--output", str(self.build)], env=self.env, capture_output=True, text=True, timeout=60)
        self.assertEqual(built.returncode, 0, built.stdout + built.stderr)
        result = self.package()
        self.assertEqual(result.returncode, 1)
        self.assertIn("differs from its recorded commit", result.stderr)
        self.assertFalse(self.output.exists())

    def test_rebuilt_untracked_component_file_cannot_enter_a_release(self):
        (self.root / "tools/agent/new.go").write_text("package main\n// uncommitted addition\n")
        built = subprocess.run([sys.executable, str(self.root / "scripts/build"), "agent",
                                "--output", str(self.build)], env=self.env, capture_output=True, text=True, timeout=60)
        self.assertEqual(built.returncode, 0, built.stdout + built.stderr)
        result = self.package()
        self.assertEqual(result.returncode, 1)
        self.assertIn("differs from its recorded commit", result.stderr)
        self.assertFalse(self.output.exists())

    def test_corrupt_binary_is_rejected_by_the_existing_package_verifier(self):
        binary = self.build / "tools/agent/bin/agent"
        binary.write_bytes(binary.read_bytes() + b"corruption")
        result = self.package()
        self.assertEqual(result.returncode, 1)
        self.assertIn("checksum mismatch", result.stderr)
        self.assertFalse(self.output.exists())

    def test_wrong_platform_binary_is_rejected_even_with_a_matching_receipt(self):
        binary = self.build / "tools/agent/bin/agent"
        env = dict(self.env, GOOS="linux" if sys.platform == "darwin" else "darwin", CGO_ENABLED="0",
                   GOWORK="off", GOENV="off", GOTOOLCHAIN="local", GOFLAGS="")
        subprocess.run(["go", "build", "-o", str(binary), "."], cwd=self.root / "tools/agent", env=env, check=True)
        receipt = self.build / "tools/agent/package.json"
        value = json.loads(receipt.read_text())
        for item in value["files"]:
            if item["path"] == "bin/agent":
                item["sha256"] = hashlib.sha256(binary.read_bytes()).hexdigest()
        receipt.write_text(json.dumps(value))
        result = self.package()
        self.assertEqual(result.returncode, 1)
        self.assertIn("binary platform mismatch", result.stderr)
        self.assertFalse(self.output.exists())

    def test_existing_release_files_are_preserved(self):
        self.output.parent.mkdir()
        adjacent = Path(str(self.output) + ".json")
        adjacent.write_text("operator release metadata\n")
        result = self.package()
        self.assertEqual(result.returncode, 1)
        self.assertIn("refusing to replace", result.stderr)
        self.assertEqual(adjacent.read_text(), "operator release metadata\n")
        self.assertFalse(self.output.exists())

    def test_linked_payload_is_rejected_even_when_the_package_receipt_allows_it(self):
        directory = self.build / "tools/agent"
        (directory / "alias").symlink_to("bin/agent")
        receipt = directory / "package.json"
        value = json.loads(receipt.read_text())
        value["files"].append({"path": "alias", "symlink": "bin/agent"})
        receipt.write_text(json.dumps(value))
        result = self.package()
        self.assertEqual(result.returncode, 1)
        self.assertIn("only regular files", result.stderr)
        self.assertFalse(self.output.exists())

    def test_malformed_source_revision_is_a_clean_error(self):
        receipt = self.build / "tools/agent/package.json"
        value = json.loads(receipt.read_text())
        value["source"]["repository_revision"] = []
        receipt.write_text(json.dumps(value))
        result = self.package()
        self.assertEqual(result.returncode, 1)
        self.assertIn("full source commit", result.stderr)
        self.assertNotIn("Traceback", result.stderr)
        self.assertFalse(self.output.exists())


if __name__ == "__main__":
    unittest.main()
