"""Exercise real release archives and installation, substituting only Go builds."""
import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("weave_release", ROOT / "scripts" / "release.py")
release = importlib.util.module_from_spec(spec)
spec.loader.exec_module(release)


class ReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="weave-release-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / "checkout"
        self.root.mkdir()
        self.output = Path(self.temp.name) / "artifacts"
        self.files = {
            "main.go": b'package main\nconst (\n version = "0.1.0"\n)\nfunc main() {}\n',
            "main_test.go": b"package main\n",
            "go.mod": b"module example.test/weave\n\ngo 1.26\n",
            "README.md": b"Fixture documentation.\n",
            "LICENSE": b"Fixture license.\n",
            "weave.1": b'.TH WEAVE 1 "September 7, 2026"\n',
            "Makefile": (ROOT / "Makefile").read_bytes(),
            "scripts/release.py": b"# Reviewed fixture source.\n",
            "scripts/dependencies.json": b"{}\n",
            "tests/check": b"#!/bin/sh\nexit 0\n",
            "tests/test_fixture.py": b"# Fixture test.\n",
            "examples/quickstart/tasks.jsonl": b'{"id":"a","needs":[],"input":null}\n',
            "examples/quickstart/observations.jsonl": b"",
            "examples/results/2026-09-07.json": b'{"synthetic":true}\n',
            ".github/workflows/check.yml": b"name: fixture\n",
        }
        names = sorted([*self.files, "scripts/release-files.txt"])
        self.files["scripts/release-files.txt"] = ("\n".join(names) + "\n").encode()
        for name, body in self.files.items():
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(body)
            path.chmod(0o755 if name == "tests/check" else 0o644)
        for name in ("private.go", "examples/run/session.jsonl", "var/session.jsonl",
                     ".git/config", "dist/old.tar.gz"):
            path = self.root / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(b"PRIVATE_RELEASE_MARKER\n")
        self.builds = []
        self.after_snapshot = None
        self.fail_build = False
        self.root_patch = patch.object(release, "ROOT", self.root)
        self.root_patch.start()
        self.addCleanup(self.root_patch.stop)

    def assert_build_environment(self, env):
        for key, value in {"GOENV": "off", "GOTOOLCHAIN": "local", "GOWORK": "off",
                           "GOFLAGS": "", "CGO_ENABLED": "0", "GOAMD64": "v1", "GOARM64": "v8.0"}.items():
            self.assertEqual(env[key], value)
        self.assertNotIn("GOEXPERIMENT", env)
        self.assertNotIn("CGO_CFLAGS", env)
        self.assertNotIn("GOTMPDIR", env)

    def fake_version(self, argv, **options):
        self.assertEqual(argv, ["go", "version"])
        self.assertTrue(options["text"])
        self.assertNotEqual(Path(options["cwd"]), self.root)
        self.assert_build_environment(options["env"])
        if self.after_snapshot:
            self.after_snapshot()
        return "go version go1.26.0 fixture/fixture\n"

    def fake_build(self, argv, **options):
        self.assertEqual(argv[:4], ["go", "build", "-trimpath", "-buildvcs=false"])
        self.assertEqual(argv[-1], ".")
        self.assertTrue(options["check"])
        self.assert_build_environment(options["env"])
        stage = Path(options["cwd"])
        self.assertNotEqual(stage, self.root)
        staged = {path.relative_to(stage).as_posix(): path.read_bytes()
                  for path in stage.rglob("*") if path.is_file()}
        self.assertEqual(staged, self.files)
        if self.fail_build:
            raise subprocess.CalledProcessError(1, argv)
        env = options["env"]
        target = env["GOOS"] + "-" + env["GOARCH"]
        body = b"fixture-binary:" + target.encode() + b":" + hashlib.sha256(staged["main.go"]).digest()
        Path(argv[argv.index("-o") + 1]).write_bytes(body)
        self.builds.append((target, body))

    def invoke(self, targets=("linux-amd64", "darwin-arm64")):
        argv = ["release.py", "--output", str(self.output), "--targets", *targets]
        polluted = {"GOFLAGS": "-tags=private", "GOEXPERIMENT": "unreviewed",
                    "GOENV": "/private/go-env", "GOWORK": "/private/go.work",
                    "CGO_ENABLED": "1", "CGO_CFLAGS": "-I/private", "GOTMPDIR": "/private/tmp"}
        with patch.object(sys, "argv", argv), patch.dict(os.environ, polluted), \
                patch.object(release.subprocess, "check_output", side_effect=self.fake_version), \
                patch.object(release.subprocess, "run", side_effect=self.fake_build), \
                contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            release.main()

    def artifacts(self):
        return {path.name: path.read_bytes() for path in self.output.iterdir() if path.is_file()}

    def test_archives_checksums_manifest_and_reproducible_rebuild(self):
        self.invoke()
        artifacts = self.artifacts()
        expected_archives = {"weave-0.1.0-source.tar.gz", "weave-0.1.0-linux-amd64.tar.gz",
                             "weave-0.1.0-darwin-arm64.tar.gz"}
        self.assertEqual(set(artifacts), expected_archives | {"build.json", "SHA256SUMS"})
        checksums = {name: value for value, name in
                     (line.split("  ", 1) for line in artifacts["SHA256SUMS"].decode().splitlines())}
        self.assertEqual(set(checksums), set(artifacts) - {"SHA256SUMS"})
        for name, value in checksums.items():
            self.assertEqual(value, hashlib.sha256(artifacts[name]).hexdigest())
        metadata = json.loads(artifacts["build.json"])
        self.assertEqual(metadata["version"], "0.1.0")
        self.assertEqual(metadata["go"], "go version go1.26.0 fixture/fixture")
        self.assertEqual(metadata["targets"], ["darwin-arm64", "linux-amd64"])
        self.assertFalse(metadata["cgo"])
        self.assertEqual(metadata["sources"], {name: hashlib.sha256(body).hexdigest()
                                              for name, body in self.files.items()})
        self.assertNotIn(str(self.root), artifacts["build.json"].decode())
        binaries = dict(self.builds)
        for name in expected_archives:
            prefix = name.removesuffix(".tar.gz")
            expected = dict(self.files)
            if prefix.endswith("-source"):
                prefix = prefix.removesuffix("-source")
            else:
                target = prefix.removeprefix("weave-0.1.0-")
                expected["weave"] = binaries[target]
            self.assertEqual(artifacts[name][4:8], b"\0" * 4)  # Fixed gzip timestamp.
            with tarfile.open(fileobj=io.BytesIO(artifacts[name]), mode="r:gz") as archive:
                members = archive.getmembers()
                self.assertEqual([entry.name for entry in members], [prefix + "/" + key for key in sorted(expected)])
                for entry in members:
                    relative = entry.name[len(prefix) + 1:]
                    self.assertTrue(entry.isfile())
                    self.assertEqual((entry.uid, entry.gid, entry.mtime, entry.uname, entry.gname), (0, 0, 0, "", ""))
                    self.assertEqual(entry.mode, 0o755 if relative in ("weave", "tests/check") else 0o644)
                    self.assertEqual(archive.extractfile(entry).read(), expected[relative])
        mtimes = {path.name: path.stat().st_mtime_ns for path in self.output.iterdir()}
        for name in self.files:
            os.utime(self.root / name, (123456789, 123456789))
        self.invoke(tuple(reversed(metadata["targets"])))
        self.assertEqual(artifacts, self.artifacts())
        self.assertEqual(mtimes, {path.name: path.stat().st_mtime_ns for path in self.output.iterdir()})

    def test_build_uses_snapshot_even_if_checkout_changes_after_capture(self):
        self.after_snapshot = lambda: (self.root / "main.go").write_bytes(b"changed after capture\n")
        self.invoke()
        self.assertNotEqual((self.root / "main.go").read_bytes(), self.files["main.go"])
        self.assertEqual(len(self.builds), 2)

    def test_changed_output_refuses_every_new_artifact(self):
        self.output.mkdir()
        (self.output / "build.json").write_bytes(b"previous release\n")
        with self.assertRaisesRegex(ValueError, "differs"):
            self.invoke()
        self.assertEqual(self.artifacts(), {"build.json": b"previous release\n"})

    def test_failed_build_leaves_existing_output_untouched(self):
        self.output.mkdir()
        (self.output / "old.txt").write_bytes(b"keep\n")
        self.fail_build = True
        with self.assertRaises(subprocess.CalledProcessError):
            self.invoke()
        self.assertEqual(self.artifacts(), {"old.txt": b"keep\n"})

    def test_allowlist_rejects_ambiguous_missing_or_indirect_paths(self):
        listing = self.root / "scripts" / "release-files.txt"
        cases = [["."], ["../outside"], [str(self.root / "main.go")], [""],
                 ["./main.go"], ["main.go", "main.go"], ["missing.py"], ["var/session.jsonl"]]
        (self.root / "link.py").symlink_to(self.root / "main.go")
        (self.root / "linked").symlink_to(self.root / "examples", target_is_directory=True)
        cases += [["link.py"], ["linked/quickstart/tasks.jsonl"]]
        for names in cases:
            with self.subTest(names=names):
                listing.write_text("\n".join([*sorted(self.files), *names]) + "\n")
                with self.assertRaises(ValueError):
                    release.source_files()
        listing.write_text("\n".join(name for name in sorted(self.files)
                                     if name != "scripts/release-files.txt") + "\n")
        with self.assertRaises(ValueError):
            release.source_files()

    def test_duplicate_targets_fail_before_building(self):
        with self.assertRaises(SystemExit) as caught:
            self.invoke(("linux-amd64", "linux-amd64"))
        self.assertEqual(caught.exception.code, 2)
        self.assertEqual(self.builds, [])
        self.assertFalse(self.output.exists())

    @unittest.skipUnless(shutil.which("make") and shutil.which("install"), "Unix installation tools required")
    def test_make_install_accepts_staged_paths_with_spaces_without_rebuilding(self):
        binary = self.root / "weave"
        binary.write_bytes(b"#!/bin/sh\nprintf 'fixture\\n'\n")
        binary.chmod(0o755)
        for name in ("main.go", "go.mod"):
            os.utime(self.root / name, (1, 1))
        os.utime(binary, (2, 2))
        destination = Path(self.temp.name) / "staged installation"
        prefix = "/opt/weave test"
        subprocess.run(["make", "-s", "install", "DESTDIR=" + str(destination), "PREFIX=" + prefix,
                        "GO=" + str(self.root / "must-not-execute-go")], cwd=self.root, check=True,
                       capture_output=True)
        installed = destination / prefix.lstrip("/")
        for source, target, mode in ((binary, installed / "bin/weave", 0o755),
                                     (self.root / "weave.1", installed / "share/man/man1/weave.1", 0o644)):
            self.assertEqual(target.read_bytes(), source.read_bytes())
            self.assertEqual(target.stat().st_mode & 0o777, mode)


if __name__ == "__main__":
    unittest.main()
