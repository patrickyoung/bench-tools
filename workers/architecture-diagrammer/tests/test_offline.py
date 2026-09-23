#!/usr/bin/env python3
import copy
import hashlib
import importlib.util
import json
import os
import re
from pathlib import Path
import struct
import shutil
import subprocess
import sys
import tempfile
import unittest
import zlib
sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("common", Path(__file__).with_name("common.py"))
f = importlib.util.module_from_spec(spec)
spec.loader.exec_module(f)
spec = importlib.util.spec_from_file_location("core", f.EXPERT / "tools/core.py")
c = importlib.util.module_from_spec(spec)
spec.loader.exec_module(c)

class ContractTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.w = Path(self.tmp.name).resolve()

    def compile(self, m):
        f.stage(self.w, m)
        p = f.run(self.w, "tools/compile")
        self.assertEqual(p.returncode, 0, p.stdout)
        return p

    def check(self, expected=0, *args):
        p = f.run(self.w, "bin/check", *args)
        self.assertEqual(p.returncode, expected, p.stdout)
        return p.stdout

    def test_definition_vs_job_exit_contract(self):
        self.compile(f.fixture())
        self.check()
        d = self.w / "definition-copy"
        for name, contents in (("PROFILE.md", None), ("PROFILE.md", b""),
                               ("PROFILE.md", b"\xff"), ("tools/core.py", None),
                               ("tools/core.py", b"syntax broken !"),
                               ("tools/render.mjs", None)):
            if d.exists(): shutil.rmtree(d)
            shutil.copytree(f.EXPERT, d)
            p = d / name
            if contents is None: p.unlink()
            else: p.write_bytes(contents)
            result = subprocess.run([sys.executable, "-I", str(d / "bin/check")],
                                    cwd=self.w, capture_output=True, text=True)
            self.assertEqual(result.returncode, 2, (name, result.stderr))
        (self.w / "request.md").unlink()
        self.check(1)

    def test_compact_labels_and_full_provenance(self):
        m = f.fixture()
        self.compile(m)
        src = (self.w / "output/view-01.mmd").read_text()
        self.assertNotIn("Owner:", src)
        self.assertNotIn(" | current | supplied", src)
        self.assertNotIn("Processing boundary | Zone", src)
        self.assertIn("Submit work (request) | HTTPS | Data: Work record", src)
        es = {e["id"]: e for e in m["entities"]}
        self.assertEqual(c.frame_notes(m["views"][0], es), ["Owner (all elements): Service operations"])
        alt = (self.w / "output/view-01.alt.md").read_text()
        for row in m["entities"] + m["relationships"]:
            self.assertIn(json.dumps(row, ensure_ascii=False), alt)

    def test_qualifiers_restrictions_and_attribution(self):
        m = f.fixture()
        v = m["views"][0]
        v["state"] = "transition"
        m["status"] = "provisional"
        e = m["entities"][2]
        e.update(state="target", qualifier="proposed")
        e["owner"] = f.atom("unknown", "unknown")
        e["attributes"] = [dict(label="Residency", **f.atom("EU only")),
                           dict(label="Change", **f.atom("Migration gated", "proposed"))]
        for r in m["relationships"]: r["state"] = "transition"
        m["relationships"][0]["protocol"] = f.atom("unknown", "unknown")
        m["relationships"][0]["data"] = f.atom("No personal data", "proposed")
        m["entities"][1]["state"] = "transition"
        m["entities"][1]["attributes"] = [dict(label="Restriction", **f.atom("EU only"))]
        self.compile(m)
        self.check()
        src = (self.w / "output/view-01.mmd").read_text()
        for literal in ("proposed", "target", "Residency", "EU only", "Migration gated",
                        "Protocol: unknown", "No personal data (proposed)"):
            self.assertIn(literal, src)
        notes = c.frame_notes(v, {e["id"]:e for e in m["entities"]})
        self.assertIn("Work service [Worker] owner: unknown", notes)
        self.assertIn("Processing boundary [Zone] Restriction: EU only", notes)

    def test_ambiguous_names_retain_ids_and_type_specific_key(self):
        m = f.fixture()
        m["entities"][2]["name"] = m["entities"][3]["name"] = "Shared name"
        self.compile(m)
        self.check()
        es = {e["id"]:e for e in m["entities"]}
        self.assertEqual(c.visible_entity(es["Worker"], m["views"][0], es)[0],
                         "Shared name [Worker]")
        self.assertEqual(c.visible_entity(es["Store"], m["views"][0], es)[0],
                         "Shared name [Store]")
        self.assertIn("lifelines", c.figure_key(f.fixture(True)["views"][0]))
        self.assertNotIn("cylinder", c.figure_key(f.fixture(True)["views"][0]))

    def test_safe_ids_and_alias_collision(self):
        ids = ["CAP-01", "R-01", "C-01", "S-01", "D-01", "lowercase", "CAP_01",
               "h_4341502d3031", "end", "a" * 64]
        self.assertEqual(len(set(c.alias(i) for i in ids)), len(ids))
        for i in ids: self.assertEqual(c.ident(i), i)
        for i in ["a b", "x;end", "x\ny", "x-->y", "../id", "-id", "é", "a"*65, "1id"]:
            with self.subTest(i=i), self.assertRaises(c.Reject): c.ident(i)

    def test_unrepresentable_edge_boundary_punctuation(self):
        for literal in ['Quote "name"', "hash #1", "entity &amp;", "entity &#91;"]:
            with self.subTest(literal=literal), self.assertRaises(c.Reject):
                c.flow_literal(literal)

    def test_css_allow_local_reject_active(self):
        c.safe_css("g {fill: url(#marker); stroke: #123456}")
        for css in ['@keyframes dash{to{stroke-dashoffset:0;}}',
                    '@import "local.css";', 'g {fill:url(https://bad/x)}',
                    r'g {fill:u\72l(//bad/x)}', 'g {fill: image-set("x")}',
                    'g {behavior:x}', 'g {fill:url(data:image/png,x)}']:
            with self.subTest(css=css), self.assertRaises(c.Reject): c.safe_css(css)

    def test_positive_flowchart(self):
        self.compile(f.fixture())
        self.check()
        src = (self.w / "output/view-01.mmd").read_text()
        self.assertIn("n_User -->", src)
        self.assertIn('"| n_Worker', src)

    def test_positive_sequence_arrows(self):
        self.compile(f.fixture(True))
        self.check()
        src = (self.w / "output/view-01.mmd").read_text()
        # Independent directed-message expectations from fixture evidence.
        for s in ("n_User->>n_Worker:", "n_Worker-)n_Store:", "n_Worker-->>n_User:"):
            self.assertIn(s, src)
        self.assertNotIn("n_Store->>n_Worker:", src)

    def test_long_sequence_headers_and_attributed_notes(self):
        m = f.long_participants()
        self.compile(m)
        before = (self.w / "output/model.json").read_bytes()
        self.check()
        self.assertEqual(before, (self.w / "output/model.json").read_bytes())
        v = m["views"][0]
        es = {e["id"]: e for e in m["entities"]}
        rs = {r["id"]: r for r in m["relationships"]}
        notes = c.frame_notes(v, es)
        labels = c.visible_labels(v, es, rs)
        src = (self.w / "output/view-01.mmd").read_text()
        alt = (self.w / "output/view-01.alt.md").read_text()
        normalize = lambda s: re.sub(r"\s+", "", s)
        for e in m["entities"]:
            self.assertIn(json.dumps(e, ensure_ascii=False), alt)
            for a in e["attributes"]:
                value = a["value"] + (" (proposed)" if a["qualifier"] == "proposed" else "")
                self.assertIn(e["name"] + " [" + e["id"] + "] " + a["label"] + ": " + value, notes)
                self.assertNotIn(a["label"] + ": " + value, " ".join(labels))
            if e["kind"] == "boundary":
                continue
            header = next(line.split(" as ", 1)[1] for line in src.splitlines()
                          if line.startswith("participant " + c.alias(e["id"]) + " as "))
            decoded = re.sub(r"#(\d+);", lambda match: chr(int(match[1])), header)
            self.assertGreater(decoded.count("<br/>"), 1)
            expected = e["name"] + " " + e["kind"]
            expected += " / proposed / target" if e["id"] == "Worker" else " / current"
            self.assertEqual(normalize(decoded.replace("<br/>", "")), normalize(expected))
        self.assertIn("Review date: unknown", " ".join(notes))

    def test_sequence_wrapping_preserves_literals_and_duplicate_ids(self):
        m = f.long_participants()
        m["entities"][2]["name"] = m["entities"][3]["name"]
        es = {e["id"]: e for e in m["entities"]}
        for i in ("Worker", "Store"):
            lines = c.visible_entity(es[i], m["views"][0], es)
            self.assertIn("[" + i + "]", lines[0])
            wrapped = c.sequence_header(lines)
            decoded = re.sub(r"#(\d+);", lambda match: chr(int(match[1])), wrapped)
            self.assertEqual("".join(decoded.replace("<br/>", "").split()),
                             "".join(" ".join(lines).split()))

    def test_positive_needs_input(self):
        self.compile(f.blocked())
        self.check()
        self.assertEqual(set(p.name for p in (self.w / "output").iterdir()),
                         {"model.json", "manifest.json", "report.md"})
        self.check(1, "--rendered")

    def test_positive_proposed_unknown_transition(self):
        m = f.fixture()
        m["status"] = "provisional"
        m["entities"][2]["state"] = "target"
        m["entities"][2]["qualifier"] = "proposed"
        m["entities"][1]["state"] = "transition"
        m["entities"][2]["owner"] = f.atom("unknown", "unknown")
        for r in m["relationships"]:
            r["state"] = "transition"
            r["protocol"] = f.atom("unknown", "unknown")
        m["views"][0]["state"] = "transition"
        self.compile(m)
        self.check()
        self.assertIn("proposed", (self.w / "output/view-01.mmd").read_text())
        self.assertIn("unknown", (self.w / "output/view-01.mmd").read_text())

    def test_positive_decomposition_and_omission(self):
        m = f.fixture()
        second = copy.deepcopy(m["views"][0])
        second.update(id="Acceptance", relationships=["Reply"])
        m["views"][0]["relationships"] = ["Submit", "Write"]
        m["views"].append(second)
        spare = f.entity("Archive", "Historical archive", "datastore")
        m["entities"].append(spare)
        m["omissions"] = [{"id": "Archive", "rationale": "Archive is outside the acceptance-path question."}]
        self.compile(m)
        self.check()
        self.assertIn("Reply: Acceptance", (self.w / "output/coverage.md").read_text())

    def test_sorted_recursive_inventory_and_stale_input(self):
        self.compile(f.fixture())
        (self.w / "inputs/z").mkdir()
        (self.w / "inputs/z/table.md").write_text("Supplemental evidence\n")
        self.check(1)
        p = f.run(self.w, "tools/compile")
        self.assertEqual(p.returncode, 0, p.stdout)
        self.check()
        manifest = json.loads((self.w / "output/manifest.json").read_text())
        self.assertEqual([r["path"] for r in manifest["inputs"]], ["inputs/facts.md", "inputs/z/table.md"])
        (self.w / "inputs/facts.md").write_text("Changed evidence\n")
        self.check(1)

    def test_stale_request(self):
        self.compile(f.fixture())
        (self.w / "request.md").write_text("Different scope\n")
        self.check(1)

    def test_stale_profile_hash(self):
        self.compile(f.fixture())
        p = self.w / "output/manifest.json"
        d = json.loads(p.read_text())
        d["profile_sha256"] = "0" * 64
        p.write_text(json.dumps(d))
        self.check(1)

    def test_reversed_arrow_rehashed(self):
        self.compile(f.fixture())
        p = self.w / "output/view-01.mmd"
        s = p.read_text()
        lines = s.splitlines()
        for n, line in enumerate(lines):
            if "n_User -->" in line and '"| n_Worker' in line:
                lines[n] = line.replace("n_User -->", "n_Worker -->").replace('"| n_Worker', '"| n_User')
        p.write_text("\n".join(lines) + "\n")
        f.rebind(self.w, "view-01.mmd")
        self.assertIn("model/source mismatch", self.check(1))

    def test_missing_edge_rehashed(self):
        self.compile(f.fixture())
        p = self.w / "output/view-01.mmd"
        p.write_text("\n".join(line for line in p.read_text().splitlines() if "n_User -->" not in line) + "\n")
        f.rebind(self.w, "view-01.mmd")
        self.check(1)

    def test_reversed_sequence_rehashed(self):
        self.compile(f.fixture(True))
        p = self.w / "output/view-01.mmd"
        p.write_text(p.read_text().replace("n_User->>n_Worker:", "n_Worker->>n_User:"))
        f.rebind(self.w, "view-01.mmd")
        self.check(1)

    def test_missing_artifact(self):
        self.compile(f.fixture())
        (self.w / "output/view-01.alt.md").unlink()
        self.check(1)

    def test_extra_artifact(self):
        self.compile(f.fixture())
        (self.w / "output/executable.py").write_text("raise Exception('do not execute')\n")
        self.check(1)

    def test_output_symlink(self):
        self.compile(f.fixture())
        p = self.w / "output/view-01.mmd"
        data = p.read_bytes()
        target = self.w / "outside"
        target.write_bytes(data)
        p.unlink()
        p.symlink_to(target)
        self.check(1)

    def test_output_hardlink(self):
        self.compile(f.fixture())
        os.link(self.w / "output/view-01.mmd", self.w / "linked")
        self.check(1)

    def test_input_directory_symlink(self):
        self.compile(f.fixture())
        (self.w / "inputs/link").symlink_to(self.w / "output", target_is_directory=True)
        self.check(1)

    def test_input_hardlink(self):
        self.compile(f.fixture())
        os.link(self.w / "inputs/facts.md", self.w / "linked")
        self.check(1)

    def test_fifo_not_opened(self):
        self.compile(f.fixture())
        os.mkfifo(self.w / "output/pipe")
        self.check(1)

    def test_no_cwd_or_pythonpath_import(self):
        self.compile(f.fixture())
        for name in ("core.py", "json.py", "pathlib.py"):
            (self.w / name).write_text("raise RuntimeError('artifact imported')\n")
        self.check()

    def test_check_read_only(self):
        self.compile(f.fixture())
        def snapshot():
            return {str(p.relative_to(self.w)): (p.read_bytes(), p.stat().st_mtime_ns)
                    for p in self.w.rglob("*") if p.is_file()}
        before = snapshot()
        self.check()
        self.assertEqual(snapshot(), before)

    def test_closed_manifest_and_wrong_hash(self):
        self.compile(f.fixture())
        p = self.w / "output/manifest.json"
        d = json.loads(p.read_text())
        d["artifacts"][0]["sha256"] = "a" * 64
        p.write_text(json.dumps(d))
        self.check(1)

    def test_malformed_json(self):
        self.compile(f.fixture())
        (self.w / "output/model.json").write_text('{"schema":')
        self.check(1)

    def test_duplicate_json_key(self):
        self.compile(f.fixture())
        p = self.w / "output/model.json"
        p.write_text(p.read_text().replace('"status": "prepared"', '"status": "prepared", "status": "prepared"'))
        self.check(1)

    def test_nonfinite_json(self):
        self.compile(f.fixture())
        p = self.w / "output/model.json"
        p.write_text(p.read_text().replace('"questions": []', '"questions": [NaN]'))
        self.check(1)

    def test_oversized_prose(self):
        self.compile(f.fixture())
        (self.w / "request.md").write_bytes(b"x" * (1024 * 1024 + 1))
        self.check(1)

    def test_protocol_receipt_not_authenticity(self):
        self.compile(f.fixture())
        # Independent fabricated format fixture deliberately has no browser.
        # It demonstrates the stated checker limit, never visual quality.
        width, height = 1200, 400
        svg = ('<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="400" viewBox="0 0 1200 400">'
               '<title>Protocol fixture</title><desc>Not a real diagram</desc>'
               '<rect width="1200" height="400" fill="white"/></svg>').encode()
        def chunk(k, d):
            return struct.pack(">I", len(d)) + k + d + struct.pack(">I", zlib.crc32(k+d) & 0xffffffff)
        png = (b"\x89PNG\r\n\x1a\n" +
               chunk(b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 2, 0, 0, 0)) +
               chunk(b"IDAT", zlib.compress((b"\0" + b"\xff" * width * 3) * height)) +
               chunk(b"IEND", b""))
        row = {"id": "Workflow", "source_sha256": c.sha((self.w / "output/view-01.mmd").read_bytes()),
               "config_sha256": c.sha(c.dump(c.config(f.fixture()["views"][0])))}
        for ext, data in (("svg", svg), ("png", png)):
            name = "view-01." + ext
            (self.w / "output" / name).write_bytes(data)
            row[ext] = {"path": name, "sha256": c.sha(data), "dimensions": [width,height]}
        receipt = {"schema": "architecture-render/v1", "stage": "rendered-unreviewed",
                   "manifest_sha256": c.sha((self.w / "output/manifest.json").read_bytes()),
                   "renderer_sha256": c.sha((f.EXPERT / "tools/render.mjs").read_bytes()),
                   "runtime": {"path": "/operator/admitted/runtime", "cli": "11.17.0", "mermaid": "11.17.2",
                               "puppeteer": "25.11.0", "node": "v22.0.0", "browser": "fixture",
                               "cli_sha256": "a"*64, "lock_sha256": "b"*64, "browser_sha256": "c"*64},
                   "views": [row]}
        p = self.w / "output/render.json"
        p.write_bytes(c.dump(receipt))
        self.check(0, "--rendered")
        receipt["views"][0]["svg"]["sha256"] = "d"*64
        p.write_bytes(c.dump(receipt))
        self.check(1, "--rendered")
        receipt["views"][0]["svg"]["sha256"] = c.sha(svg)
        receipt["stage"] = "approved"
        p.write_bytes(c.dump(receipt))
        self.check(1, "--rendered")

    def test_svg_active_and_entity_rejection(self):
        prefix = '<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="400" viewBox="0 0 1200 400">'
        for fragment in ('<script>alert(1)</script>', '<image href="file:///secret"/>',
                         '<g onload="x"/>', '<style>@import "file:///x";</style>',
                         '<style>g { fill: url(//remote/x) }</style>', '<foreignObject/>'):
            with self.subTest(fragment=fragment), self.assertRaises(c.Reject):
                c.image_dimensions((prefix + '<title>T</title><desc>D</desc>' + fragment + '</svg>').encode(), "svg")
        with self.assertRaises(c.Reject):
            c.image_dimensions(b'<!DOCTYPE svg [<!ENTITY x "x">]>' + prefix.encode() + b'</svg>', "svg")

    def test_png_truncated_crc(self):
        for data in (b"not png", b"\x89PNG\r\n\x1a\n", b"\x89PNG\r\n\x1a\n" + b"\0"*20):
            with self.subTest(data=data), self.assertRaises(c.Reject):
                c.image_dimensions(data, "png")

def invalid_case(name, mutate):
    def test(self):
        m = f.fixture()
        mutate(m)
        f.stage(self.w, m)
        p = f.run(self.w, "tools/compile")
        self.assertEqual(p.returncode, 1, p.stdout)
        self.assertFalse((self.w / "output/manifest.json").exists())
    test.__name__ = "test_reject_" + name
    setattr(ContractTests, test.__name__, test)

invalid_case("duplicate_entity", lambda m: m["entities"].append(copy.deepcopy(m["entities"][0])))
invalid_case("duplicate_global_id", lambda m: m["relationships"][0].update(id="Worker"))
invalid_case("dangling_endpoint", lambda m: m["relationships"][0].update(to="Missing"))
invalid_case("dangling_view", lambda m: m["views"][0]["relationships"].append("Missing"))
invalid_case("omitted_endpoint", lambda m: m["views"][0]["entities"].remove("User"))
invalid_case("duplicate_selected_edge", lambda m: m["views"][0]["relationships"].append("Submit"))
invalid_case("cycle", lambda m: m["entities"][1].update(parent="Zone"))
invalid_case("missing_parent", lambda m: m["entities"][2].update(parent="NoBoundary"))
invalid_case("parent_not_boundary", lambda m: m["entities"][2].update(parent="User"))
invalid_case("boundary_endpoint", lambda m: m["relationships"][0].update(to="Zone"))
invalid_case("mixed_state", lambda m: m["entities"][2].update(state="target"))
invalid_case("proposed_current", lambda m: m["entities"][2].update(qualifier="proposed"))
invalid_case("capability_as_application", lambda m: m["entities"][2].update(kind="capability"))
invalid_case("physical_mix", lambda m: m["entities"][0].update(abstraction="physical"))
invalid_case("unknown_as_supplied", lambda m: m["entities"][0].update(owner=f.atom("unknown")))
invalid_case("unknown_prepared", lambda m: m["entities"][0].update(owner=f.atom("unknown","unknown")))
invalid_case("missing_coverage", lambda m: m["views"][0]["relationships"].remove("Reply"))
invalid_case("overlapping_omission", lambda m: m["omissions"].append({"id":"User","rationale":"Outside scope"}))
invalid_case("directive", lambda m: m["entities"][0].update(name="%% init"))
invalid_case("html", lambda m: m["entities"][0].update(name="<script>alert</script>"))
invalid_case("url", lambda m: m["entities"][0].update(name="https://evil.example"))
invalid_case("newline", lambda m: m["relationships"][0].update(intent="Submit\nclick node"))
invalid_case("path_traversal", lambda m: m["entities"][0]["sources"][0].update(path="inputs/../request.md"))
invalid_case("absolute_source", lambda m: m["entities"][0]["sources"][0].update(path="/etc/passwd"))
invalid_case("unstaged_source", lambda m: m["entities"][0]["sources"][0].update(path="inputs/absent.md"))
invalid_case("missing_locator", lambda m: m["entities"][0]["sources"][0].pop("locator"))
invalid_case("arbitrary_renderer_option", lambda m: m["views"][0].update(css="color:red"))
invalid_case("unsupported_diagram", lambda m: m["views"][0].update(type="architecture-beta"))
invalid_case("unsupported_layout", lambda m: m["views"][0].update(layout="core12"))
invalid_case("unsupported_direction", lambda m: m["views"][0].update(direction="RL"))
invalid_case("sequence_elk", lambda m: m["views"][0].update(type="sequence", direction="LR", layout="elk"))
invalid_case("needs_input_with_diagram", lambda m: m.update(status="needs-input", questions=["Which source?"]))
invalid_case("oversized_label", lambda m: m["entities"][0].update(name="A"*65))
invalid_case("closed_root", lambda m: m.update(execute="ignored command"))
invalid_case("unhashable_endpoint", lambda m: m["relationships"][0].update(to=[]))

if __name__ == "__main__":
    unittest.main(verbosity=2)
