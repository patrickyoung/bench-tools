"""Pinned downloads compose the real setup/installer without a compiler or network."""
import hashlib
import io
import json
import os
from pathlib import Path
import runpy
import shlex
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
TOOLS = ("hire", "agent", "ask", "brief", "ply", "cage", "record", "trail", "hone")
REPOSITORY = "https://github.com/patrickyoung/bench-tools"
URL = REPOSITORY + "/releases/download/runtime-fixture/builder.tar.gz"


def sha256(value):
    return hashlib.sha256(value).hexdigest()


@unittest.skipUnless(shutil.which("git"), "Git source discovery prerequisite")
class PrebuiltSetup(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="bench-prebuilt-test-")
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name).resolve()
        self.root = self.base / "source checkout"
        scripts = self.root / "scripts"
        scripts.mkdir(parents=True)
        for name in ("setup", "install", "install_support.py", "prebuilt_support.py", "check"):
            shutil.copy2(ROOT / "scripts" / name, scripts / name)
        self.source_route = self.base / "source-build-selected"
        (scripts / "build").write_text(
            "from pathlib import Path\n"
            f"Path({str(self.source_route)!r}).write_text('selected source build')\n"
            "raise SystemExit(73)\n")
        self.executed = self.base / "executed-commands"
        self.downloaded = self.base / "downloaded-url"
        self.prefix = self.base / "runtime with spaces"
        self.state = self.base / "setup handoff"
        self.archive = self.base / "release.tar.gz"
        components = []
        for name in TOOLS:
            leaf = self.root / "tools" / name
            leaf.mkdir(parents=True)
            (leaf / "README.md").write_text("Synthetic independent source for " + name + "\n")
            components.append({"name": name, "path": "tools/" + name,
                               "commands": [{"name": name}]})
        (self.root / "components.json").write_text(json.dumps({"schema": 1, "components": components}))
        self.git("init", "-q")
        self.git("add", ".")
        self.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "Reusable source fixture")
        self.revision = self.git("rev-parse", "HEAD")
        check = runpy.run_path(str(scripts / "check"))
        self.sources = {}
        for component in components:
            records = check["export_source"](self.root, component, self.base / "export" / component["name"])
            self.sources[component["name"]] = sha256(json.dumps(records, sort_keys=True).encode())

        self.files, packages = [], {}
        for name in TOOLS:
            # Setup's complete verification uses these fixtures, never a model.
            command = ("#!/bin/sh\n"
                       'printf "%s\\n" "' + name + ':$1" >> ' + shlex.quote(str(self.executed)) + '\n'
                       'if [ "$1" = version ]; then echo "' + name + ' fixture"; exit 0; fi\n')
            if name == "hire":
                command += 'test "$1" = verify && "$HIRE_AGENT" fixture-check\n'
            elif name == "agent":
                command += 'test "$1" = fixture-check\n'
            elif name == "cage":
                command += 'test "$1" = check && echo "fixture boundary"\n'
            else:
                command += 'echo "unexpected invocation" >&2\nexit 98\n'
            data = command.encode()
            receipt = {"schema": 1, "name": name, "commands": [name],
                       "source": {"repository_revision": self.revision,
                                  "path": "tools/" + name, "files_sha256": self.sources[name]},
                       "files": [{"path": "bin/" + name, "mode": 0o755, "sha256": sha256(data)}]}
            raw = (json.dumps(receipt, indent=2) + "\n").encode()
            packages[name] = sha256(raw)
            self.files += [("tools/" + name + "/bin/" + name, data, 0o755),
                           ("tools/" + name + "/package.json", raw, 0o644)]
        # Exercise Linux selection on every CI host; the payloads are portable
        # shell fixtures, while package-runtime checks real binary platforms.
        self.target = "linux-amd64"
        self.metadata = {"schema": 1, "repository": REPOSITORY, "revision": self.revision,
                         "platform": self.target, "tools": list(TOOLS),
                         "sources": self.sources, "packages": packages}
        self.make_archive()
        self.write_pin()
        self.git("add", "releases/builder.json")
        self.git("-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid",
                 "-c", "commit.gpgsign=false", "commit", "-qm", "Pin unchanged component packages")
        self.checkout_revision = self.git("rev-parse", "HEAD")

        self.driver = self.base / "download-fixture.py"
        self.driver.write_text(
            "import io, os, pathlib, runpy, sys, urllib.request\n"
            "from unittest.mock import patch\n"
            f"sys.path.insert(0, {str(scripts)!r})\n"
            "class Response(io.BytesIO):\n"
            "    def geturl(self): return 'https://release-assets.githubusercontent.com/fixture'\n"
            "def download(request, timeout):\n"
            f"    assert request.full_url == {URL!r}, request.full_url\n"
            f"    pathlib.Path({str(self.downloaded)!r}).write_text(request.full_url)\n"
            f"    return Response(pathlib.Path({str(self.archive)!r}).read_bytes())\n"
            f"sys.argv = [{str(scripts / 'setup')!r}, *sys.argv[1:]]\n"
            "with patch.object(urllib.request, 'urlopen', download), "
            "patch('prebuilt_support.host_platform', return_value=os.environ['BENCH_TEST_PLATFORM']):\n"
            f"    runpy.run_path({str(scripts / 'setup')!r}, run_name='__main__')\n")
        runtime_bin = self.base / "system-bin"
        runtime_bin.mkdir()
        for name in ("git", "sh"):
            (runtime_bin / name).symlink_to(shutil.which(name))
        self.env = {"PATH": str(runtime_bin), "HOME": str(self.base / "home"),
                    "TMPDIR": str(self.base), "LANG": "C", "PYTHONDONTWRITEBYTECODE": "1",
                    "GIT_CONFIG_GLOBAL": os.devnull, "GIT_CONFIG_NOSYSTEM": "1",
                    "BENCH_TEST_PLATFORM": self.target}
        self.assertIsNone(shutil.which("go", path=self.env["PATH"]))

    def git(self, *args):
        return subprocess.check_output(["git", "-C", str(self.root), *args],
                                       stderr=subprocess.PIPE, text=True).strip()

    def make_archive(self, extra=None):
        with tarfile.open(self.archive, "w:gz") as archive:
            entries = [("runtime.json", (json.dumps(self.metadata) + "\n").encode(), 0o644), *self.files]
            for name, raw, mode in entries:
                info = tarfile.TarInfo(name)
                info.size, info.mode = len(raw), mode
                archive.addfile(info, io.BytesIO(raw))
            if extra:
                info, raw = extra
                archive.addfile(info, io.BytesIO(raw))

    def write_pin(self, **overrides):
        artifact = dict(self.metadata, url=URL, size=self.archive.stat().st_size,
                        sha256=sha256(self.archive.read_bytes()))
        artifact.update(overrides)
        self.pin = self.root / "releases/builder.json"
        self.pin.parent.mkdir(exist_ok=True)
        self.pin.write_text(json.dumps({"schema": 1, "artifacts": {self.target: artifact}}))

    def setup(self, *args):
        return subprocess.run([sys.executable, str(self.driver), "--prefix", str(self.prefix),
                               "--state-dir", str(self.state), *args], env=self.env,
                              capture_output=True, text=True, timeout=30)

    def assert_no_install(self, result):
        self.assertNotEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn("Traceback", result.stderr)
        self.assertFalse(self.prefix.exists())
        self.assertFalse(self.executed.exists())
        self.assertFalse((self.state / "BENCH-SETUP.md").exists())

    def test_source_matched_download_installs_without_go_and_retains_both_source_pins(self):
        for _ in range(2):
            result = self.setup()
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual(self.downloaded.read_text(), URL)
        self.assertFalse(self.source_route.exists())
        record = json.loads((self.state / "setup.json").read_text())
        self.assertEqual(record["source_commit"], self.checkout_revision)
        self.assertNotEqual(self.checkout_revision, self.revision)
        self.assertEqual(len(record["checks"]), 11)
        self.assertTrue(all(item["exit"] == 0 for item in record["checks"]))
        for name in TOOLS:
            self.assertEqual(record["installed_sources"][name]["repository_revision"], self.revision)
            self.assertEqual(record["installed_sources"][name]["files_sha256"], self.sources[name])
            self.assertTrue((self.prefix / "bin" / name).is_symlink())
        self.assertEqual(set(self.executed.read_text().splitlines()),
                         {*(name + ":version" for name in TOOLS), "hire:verify", "agent:fixture-check", "cage:check"})

    def test_changed_component_selects_source_build_without_downloading_or_installing(self):
        (self.root / "tools/ask/README.md").write_text("Edited source must not use the old binary\n")
        result = self.setup()
        self.assert_no_install(result)
        self.assertIn("do not match current component sources", result.stdout)
        self.assertTrue(self.source_route.exists())
        self.assertFalse(self.downloaded.exists())

    def test_explicit_source_route_does_not_download_matching_packages(self):
        result = self.setup("--from-source")
        self.assert_no_install(result)
        self.assertTrue(self.source_route.exists())
        self.assertFalse(self.downloaded.exists())

    def test_macos_defaults_to_source_even_with_matching_published_packages(self):
        for target in ("darwin-amd64", "darwin-arm64"):
            with self.subTest(target=target):
                self.target = self.metadata["platform"] = self.env["BENCH_TEST_PLATFORM"] = target
                self.make_archive()
                self.write_pin()
                result = self.setup()
                self.assert_no_install(result)
                self.assertIn("macOS setup builds from source", result.stdout)
                self.assertTrue(self.source_route.exists())
                self.assertFalse(self.downloaded.exists())

    def test_macos_accepts_explicit_local_packages_without_download(self):
        self.env["BENCH_TEST_PLATFORM"] = "darwin-arm64"
        build = self.base / "caller-selected-build"
        for name, raw, mode in self.files:
            path = build / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(raw)
            path.chmod(mode)
        result = self.setup("--from-build", str(build))
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertFalse(self.source_route.exists())
        self.assertFalse(self.downloaded.exists())

    def test_bad_archive_checksum_fails_before_installed_program_execution(self):
        self.write_pin(sha256="0" * 64)
        result = self.setup()
        self.assert_no_install(result)
        self.assertTrue(self.downloaded.exists())
        self.assertFalse(self.source_route.exists())
        self.assertIn("checksum or size mismatch", result.stderr)

    def test_tar_paths_links_and_duplicate_names_fail_before_install(self):
        for name, kind in (("../escaped", tarfile.REGTYPE),
                           ("/absolute", tarfile.REGTYPE),
                           ("tools/ask/../../escaped", tarfile.REGTYPE),
                           ("tools/ask/link", tarfile.SYMTYPE),
                           ("tools/ask/hardlink", tarfile.LNKTYPE),
                           ("runtime.json", tarfile.REGTYPE)):
            with self.subTest(name=name, kind=kind):
                info = tarfile.TarInfo(name)
                info.type, info.mode, info.linkname = kind, 0o644, "../../escaped"
                raw = b"malicious" if kind == tarfile.REGTYPE else b""
                info.size = len(raw)
                self.make_archive((info, raw))
                self.write_pin()
                result = self.setup()
                self.assert_no_install(result)
                self.assertIn("unsafe or unexpected archive member", result.stderr)
                self.assertFalse((self.base / "escaped").exists())

    def test_valid_archive_checksum_cannot_hide_changed_binary(self):
        # A late package failure must prevent every earlier command from running.
        name, data, mode = self.files[-2]
        self.files[-2] = (name, data + b"# changed without a receipt\n", mode)
        self.make_archive()
        self.write_pin()
        result = self.setup()
        self.assert_no_install(result)
        self.assertIn("file checksum mismatch", result.stderr)

    def test_package_metadata_must_match_selected_host_and_source(self):
        self.metadata["platform"] = "other-platform"
        self.make_archive()
        self.write_pin(platform=self.target)
        result = self.setup()
        self.assert_no_install(result)
        self.assertIn("metadata does not match", result.stderr)

    def test_inconsistent_pin_identity_fails_before_download(self):
        for overrides in ({"schema": 2}, {"repository": "https://example.invalid/other"},
                          {"platform": "other-platform"}, {"tools": list(reversed(TOOLS))}):
            with self.subTest(overrides=overrides):
                self.write_pin(**overrides)
                result = self.setup()
                self.assert_no_install(result)
                self.assertIn("invalid published package identity", result.stderr)
                self.assertFalse(self.downloaded.exists())

    def test_truncated_archive_is_rejected_cleanly_before_install(self):
        contents = self.archive.read_bytes()
        self.archive.write_bytes(contents[:len(contents) // 2])
        self.write_pin()
        result = self.setup()
        self.assert_no_install(result)
        self.assertIn("cannot install the pinned packages", result.stderr)

    def test_release_download_url_is_fixed_to_the_project_release_assets(self):
        for url in ("http://github.com/patrickyoung/bench-tools/releases/download/v1/a.tar.gz",
                    "https://example.invalid/releases/download/v1/a.tar.gz",
                    "https://github.com/other/bench-tools/releases/download/v1/a.tar.gz"):
            with self.subTest(url=url):
                self.write_pin(url=url)
                result = self.setup()
                self.assert_no_install(result)
                self.assertIn("invalid published package url", result.stderr)
                self.assertFalse(self.downloaded.exists())


if __name__ == "__main__":
    unittest.main()
