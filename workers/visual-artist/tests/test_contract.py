"""Synthetic contract regressions, not artwork or a model-quality evaluation."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

EXPERT = Path(os.environ.get("VISUAL_ARTIST_EXPERT", Path(__file__).resolve().parents[1] / "expert")).resolve()


class Contract(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="visual-contract-")
        self.work = Path(self.tmp.name).resolve()
        self.output = self.work / "output"
        self.output.mkdir()
        (self.work / "input.json").write_text('{"value": 7}\n')
        self.spec = {
            "schema": "bench.visual/v1", "title": "Synthetic contract fixture",
            "intent": "Exercise the file boundary, not aesthetic quality.",
            "libraries": [{"name": "d3", "version": "7.9.0"}],
            "inputs": [{"path": "input.json", "sha256": self.digest(self.work / "input.json")}],
            "encodings": [{"field": "value", "channel": "area", "meaning": "Synthetic mapping"}],
            "sensors": [], "limitations": ["Not an artwork; only tests file acceptance."]}
        # This would leave evidence if the trusted checker executed the source.
        source = "require('node:fs').writeFileSync('CHECK_EXECUTED_SOURCE', 'bad');\n"
        for name, value in {
            "visual.js": source, "visual-source.js": source,
            "visual.css": ".contract-fixture { color: #222; }\n",
            "fallback.svg": '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><title>Fixture</title><circle cx="5" cy="5" r="2"/></svg>',
            "preview.html": '<!doctype html><title>Synthetic fixture</title>',
            "visual-notes.md": "Synthetic fixture for mechanical acceptance only.\n",
        }.items():
            (self.output / name).write_text(value)
        self.manifest()

    def tearDown(self):
        self.tmp.cleanup()

    @staticmethod
    def digest(file):
        return hashlib.sha256(file.read_bytes()).hexdigest()

    def manifest(self):
        (self.output / "visual.json").write_text(json.dumps(self.spec))
        files = [str(p.relative_to(self.work)) for p in sorted(self.output.rglob("*")) if p.is_file()]
        result = subprocess.run([str(EXPERT / "tools/make-handoff"), "visual-artist", "Synthetic fixture", *files],
                                cwd=self.work, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    def check(self, expected=1):
        result = subprocess.run([str(EXPERT / "bin/check")], cwd=self.work, capture_output=True, text=True, timeout=15)
        self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
        self.assertFalse((self.work / "CHECK_EXECUTED_SOURCE").exists(), "check must never execute generated JS")

    def test_accepts_bound_files_without_executing_them(self):
        self.check(0)

    def test_missing_deliverable(self):
        (self.output / "fallback.svg").unlink()
        self.check()

    def test_altered_deliverable(self):
        (self.output / "visual.css").write_text("changed bytes")
        self.check()

    def test_stale_declared_input(self):
        (self.work / "input.json").write_text('{"value": 8}\n')
        self.check()

    def test_escape_input(self):
        self.spec["inputs"][0]["path"] = "../input.json"
        self.manifest()
        self.check()

    def test_symlink_input(self):
        (self.work / "input.json").rename(self.work / "other.json")
        (self.work / "input.json").symlink_to(self.work / "other.json")
        self.check()

    def test_symlink_parent_input(self):
        (self.work / "alias").symlink_to(self.work, target_is_directory=True)
        self.spec["inputs"][0]["path"] = "alias/input.json"
        self.manifest()
        self.check()

    def test_symlink_output(self):
        (self.output / "visual.css").rename(self.work / "other.css")
        (self.output / "visual.css").symlink_to(self.work / "other.css")
        self.check()

    def test_unlisted_output(self):
        (self.output / "extra.txt").write_text("This must be included in the handoff")
        self.check()

    def test_wrong_specialist(self):
        p = self.work / "handoff.json"
        value = json.loads(p.read_text())
        value["specialist"] = "frontend"
        p.write_text(json.dumps(value))
        self.check()

    def test_wrong_library_version(self):
        self.spec["libraries"][0]["version"] = "0.0.0"
        self.manifest()
        self.check()

    def test_duplicate_library(self):
        self.spec["libraries"].append(dict(self.spec["libraries"][0]))
        self.manifest()
        self.check()

    def test_wrong_schema(self):
        self.spec["schema"] = "bench.visual/v99"
        self.manifest()
        self.check()

    def test_bad_sensor(self):
        self.spec["sensors"] = [{"kind": "biometrics", "purpose": "Invalid kind", "fallback": "Manual"}]
        self.manifest()
        self.check()

    def test_sensor_requires_fallback(self):
        self.spec["sensors"] = [{"kind": "camera", "purpose": "Local brightness", "fallback": ""}]
        self.manifest()
        self.check()

    def test_bad_js_syntax(self):
        (self.output / "visual.js").write_text("function { definitely not JavaScript")
        self.manifest()
        self.check()

    def test_local_svg_gradient(self):
        (self.output / "fallback.svg").write_text('<svg xmlns="http://www.w3.org/2000/svg"><defs><linearGradient id="tone"><stop offset="0" stop-color="#fff"/></linearGradient></defs><rect width="10" height="10" fill="url(#tone)"/></svg>')
        self.manifest()
        self.check(0)

    def test_missing_node_is_broken_not_unfinished(self):
        result = subprocess.run([str(EXPERT / "bin/check")], cwd=self.work, capture_output=True,
                                env={**os.environ, "NODE": str(self.work / "missing-node")}, timeout=15)
        self.assertEqual(result.returncode, 2, result.stderr)

    def test_non_utf8_svg(self):
        (self.output / "fallback.svg").write_bytes('<svg xmlns="http://www.w3.org/2000/svg"/>'.encode("utf-16"))
        self.manifest()
        self.check()

    def test_svg_active_content(self):
        bad = [
            '<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>',
            '<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>',
            '<svg xmlns="http://www.w3.org/2000/svg"><foreignObject/></svg>',
            '<svg xmlns="http://www.w3.org/2000/svg"><image href="https://example.invalid/image"/></svg>',
            '<svg xmlns="http://www.w3.org/2000/svg"><style>@import "https://example.invalid/a.css";</style></svg>',
            '<svg xmlns="http://www.w3.org/2000/svg"><rect style="fill:url(https://example.invalid/image)"/></svg>',
            '<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///etc/hosts">]><svg xmlns="http://www.w3.org/2000/svg">&x;</svg>',
            '<svg xmlns="http://www.w3.org/2000/svg"><animate attributeName="href" values="https://example.invalid/x"/></svg>',
            '<svg xmlns="http://www.w3.org/2000/svg" xml:base="https://example.invalid/"><use href="#x"/></svg>',
            '<svg xmlns="http://www.w3.org/2000/svg"><style>@im\\70ort "https://example.invalid/a.css";</style></svg>',
        ]
        for value in bad:
            with self.subTest(svg=value):
                (self.output / "fallback.svg").write_text(value)
                self.manifest()
                self.check()


if __name__ == "__main__":
    unittest.main()
