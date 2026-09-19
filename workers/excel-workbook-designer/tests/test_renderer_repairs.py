"""Portable synthetic OOXML regressions; no models, renderer, or saved jobs."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import zipfile
from xml.etree import ElementTree as E

sys.dont_write_bytecode = True
EXPERT = Path(__file__).resolve().parents[1] / "expert"
sys.path.insert(0, str(EXPERT / "lib"))
import contract
import features

S = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
R = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
P = "http://schemas.openxmlformats.org/package/2006/relationships"
NS = {"s": S}


def read_parts(path):
    with zipfile.ZipFile(path) as archive:
        return {name: archive.read(name) for name in archive.namelist()}


def write_parts(path, parts):
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, content in parts.items():
            archive.writestr(name, content)


def xml_value(element):
    """Compare XML meaning without relying on the implementation's canonicalizer."""
    return (element.tag, dict(element.attrib), element.text or "",
            [xml_value(child) for child in element])


def synthetic_package():
    """Minimal source package with one conditional fill and unrelated content."""
    return {name: text.encode() for name, text in {
        "[Content_Types].xml": '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/></Types>',
        "_rels/.rels": f'<Relationships xmlns="{P}"><Relationship Id="r1" Type="{R}/officeDocument" Target="xl/workbook.xml"/></Relationships>',
        "xl/workbook.xml": f'<workbook xmlns="{S}" xmlns:r="{R}"><sheets><sheet name="Records" sheetId="1" r:id="r1"/></sheets></workbook>',
        "xl/_rels/workbook.xml.rels": f'<Relationships xmlns="{P}"><Relationship Id="r1" Type="{R}/worksheet" Target="worksheets/sheet1.xml"/></Relationships>',
        "xl/worksheets/sheet1.xml": f'<worksheet xmlns="{S}"><sheetData><row r="2"><c r="B2"><v>7</v></c></row></sheetData><conditionalFormatting sqref="B2:B4"><cfRule type="expression" priority="1" dxfId="0"><formula>B2&gt;3</formula></cfRule></conditionalFormatting></worksheet>',
        "xl/styles.xml": f'<styleSheet xmlns="{S}"><fonts count="1"><font><name val="Calibri"/><sz val="11"/></font></fonts><fills count="1"><fill><patternFill patternType="none"/></fill></fills><borders count="1"><border/></borders><cellXfs count="1"><xf fontId="0" fillId="0" borderId="0" numFmtId="0"/></cellXfs><dxfs count="1"><dxf><fill><patternFill patternType="solid"><fgColor rgb="FFF2B632"/><bgColor rgb="FFABCDEF"/></patternFill></fill></dxf></dxfs></styleSheet>',
        "docProps/custom.xml": '<properties><description>Unrelated synthetic metadata</description></properties>',
    }.items()}


class RendererRepairs(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="synthetic-renderer-repairs-")
        self.addCleanup(temporary.cleanup)
        self.work = Path(temporary.name)
        (self.work / "inputs").mkdir()
        (self.work / "output").mkdir()
        self.source = self.work / "inputs/source.xlsx"
        self.book = self.work / "output/workbook.xlsx"
        source_parts = synthetic_package()
        write_parts(self.source, source_parts)
        output_parts = dict(source_parts)
        output_parts["xl/styles.xml"] = output_parts["xl/styles.xml"].replace(b' patternType="solid"', b'')
        write_parts(self.book, output_parts)
        (self.work / "request.json").write_text(json.dumps({"mode": "edit", "workbook": "inputs/source.xlsx"}))
        (self.work / "output/spec.json").write_text(json.dumps({"operations": []}))
        (self.work / "output/qa.json").write_text(json.dumps({"workbook_sha256": hashlib.sha256(self.book.read_bytes()).hexdigest(), "synthetic_check": "retained"}))

    def assert_unchanged(self, operation):
        before = {path: path.read_bytes() for path in (self.book, self.source, self.work / "output/qa.json")}
        operation()
        for path, raw in before.items():
            self.assertEqual(path.read_bytes(), raw, str(path))

    def test_bounded_ranges_and_actionable_diagnostics(self):
        accepted = {"A1": (1, 1, 1, 1), "A1:CV10000": (1, 1, 10000, 100), "C3:E5": (3, 3, 5, 5)}
        for value, expected in accepted.items():
            with self.subTest(value=value):
                self.assertEqual(contract.bounds(value), expected)
        rejected = ["E:E", "A0", "A1:A10001", "CW1", "B2:A1", "", "A1:B2:C3", "$A$1", None, 7, "Table range A1:D5 -> A1:E5"]
        for value in rejected:
            with self.subTest(value=value), self.assertRaises(ValueError) as error:
                contract.bounds(value)
            self.assertIn(repr(value), str(error.exception))
        with self.assertRaisesRegex(ValueError, "Invalid cell"):
            contract.cell("E")

    def test_missing_pattern_restored_without_other_package_changes(self):
        source_raw = self.source.read_bytes()
        source, before = read_parts(self.source), read_parts(self.book)
        features.restore_conditional_fills(self.work)
        after = read_parts(self.book)
        self.assertEqual(set(before), set(after))
        self.assertEqual([name for name in before if before[name] != after[name]], ["xl/styles.xml"])
        self.assertEqual(xml_value(E.fromstring(source["xl/styles.xml"])), xml_value(E.fromstring(after["xl/styles.xml"])))
        self.assertEqual(self.source.read_bytes(), source_raw)
        qa = json.loads((self.work / "output/qa.json").read_text())
        self.assertEqual(qa["workbook_sha256"], hashlib.sha256(self.book.read_bytes()).hexdigest())
        self.assertEqual(qa["preserved_conditional_fills"], [0])
        self.assertEqual(qa["synthetic_check"], "retained")
        self.assert_unchanged(lambda: features.restore_conditional_fills(self.work))

    def test_requested_conditional_change_not_overridden(self):
        (self.work / "output/spec.json").write_text(json.dumps({"operations": [{"op": "conditional_format"}]}))
        self.assert_unchanged(lambda: features.restore_conditional_fills(self.work))

    def test_other_style_difference_not_concealed(self):
        parts = read_parts(self.book)
        styles = E.fromstring(parts["xl/styles.xml"])
        styles.find("s:dxfs/s:dxf/s:fill/s:patternFill/s:bgColor", NS).set("rgb", "FF123456")
        parts["xl/styles.xml"] = E.tostring(styles)
        write_parts(self.book, parts)
        self.assert_unchanged(lambda: features.restore_conditional_fills(self.work))

    def test_changed_rule_not_reinterpreted(self):
        parts = read_parts(self.book)
        sheet = E.fromstring(parts["xl/worksheets/sheet1.xml"])
        sheet.find("s:conditionalFormatting/s:cfRule/s:formula", NS).text = "B2>5"
        parts["xl/worksheets/sheet1.xml"] = E.tostring(sheet)
        write_parts(self.book, parts)
        self.assert_unchanged(lambda: features.restore_conditional_fills(self.work))

    def test_creation_untouched(self):
        (self.work / "request.json").write_text(json.dumps({"mode": "create", "workbook": "inputs/source.xlsx"}))
        self.assert_unchanged(lambda: features.restore_conditional_fills(self.work))

    def test_negative_differential_style_index_rejected(self):
        for path in (self.source, self.book):
            parts = read_parts(path)
            sheet = E.fromstring(parts["xl/worksheets/sheet1.xml"])
            sheet.find("s:conditionalFormatting/s:cfRule", NS).set("dxfId", "-1")
            parts["xl/worksheets/sheet1.xml"] = E.tostring(sheet)
            write_parts(path, parts)
        before = self.book.read_bytes()
        with self.assertRaisesRegex(ValueError, "differential style -1"):
            features.restore_conditional_fills(self.work)
        self.assertEqual(before, self.book.read_bytes())

    def test_public_check_rejects_referenced_conditional_color_change(self):
        # A complete synthetic acceptance fixture tests the public checker, not
        # only the style resolver. It is evidence for this contract, not a render.
        features.restore_conditional_fills(self.work)
        inputs = [{"path": "inputs/source.xlsx", "sha256": hashlib.sha256(self.source.read_bytes()).hexdigest()}]
        request_path = self.work / "request.json"
        request_path.write_text(json.dumps({"schema": "bench.workbook-request/v1", "mode": "edit",
            "workbook": "inputs/source.xlsx", "brief": "Keep the synthetic recorded value and its conditional formatting.",
            "inputs": inputs, "required_metrics": ["value"]}))
        specification = {"schema": "bench.workbook-edit/v1", "inputs": inputs,
            "request_sha256": hashlib.sha256(request_path.read_bytes()).hexdigest(),
            "purpose": "Synthetic preservation test", "interpretation": "The current value is retained.",
            "changes": [{"sheet": "Records", "range": "B2", "reason": "Explicitly retain the current synthetic value."}],
            "operations": [{"op": "values", "sheet": "Records", "range": "B2", "values": [[7]]}],
            "metrics": [{"id": "value", "sheet": "Records", "cell": "B2"}],
            "assertions": [{"metric": "value", "value": 7}], "tests": [],
            "previews": [{"sheet": "Records", "range": "A1:B4"}], "mappings": [], "pivots": []}
        spec_path = self.work / "output/spec.json"
        spec_path.write_text(json.dumps(specification))
        (self.work / "output/guide.md").write_text("Synthetic contract fixture only. This is not a rendered or visually reviewed workbook deliverable.\n")
        preview = self.work / "output/preview.svg"
        preview.write_text('<svg xmlns="http://www.w3.org/2000/svg"><text x="0" y="12">Synthetic fixture</text></svg>')
        qa_path = self.work / "output/qa.json"
        qa = {"spec_sha256": hashlib.sha256(spec_path.read_bytes()).hexdigest(),
            "workbook_sha256": hashlib.sha256(self.book.read_bytes()).hexdigest(),
            "assertions": [{"passed": True}], "tests": [], "baseline": {"value": 7},
            "previews": [{"path": "output/preview.svg", "sha256": hashlib.sha256(preview.read_bytes()).hexdigest()}]}
        qa_path.write_text(json.dumps(qa))
        env = dict(os.environ, WORKBOOK_PYTHON=sys.executable, PYTHONDONTWRITEBYTECODE="1")
        command = [str(EXPERT / "bin/check")]
        good = subprocess.run(command, cwd=self.work, env=env, capture_output=True, text=True, timeout=15)
        self.assertEqual(good.returncode, 0, good.stderr)

        parts = read_parts(self.book)
        styles = E.fromstring(parts["xl/styles.xml"])
        styles.find("s:dxfs/s:dxf/s:fill/s:patternFill/s:bgColor", NS).set("rgb", "FF00FF00")
        parts["xl/styles.xml"] = E.tostring(styles)
        write_parts(self.book, parts)
        qa["workbook_sha256"] = hashlib.sha256(self.book.read_bytes()).hexdigest()
        qa_path.write_text(json.dumps(qa))
        bad = subprocess.run(command, cwd=self.work, env=env, capture_output=True, text=True, timeout=15)
        self.assertEqual(bad.returncode, 1)
        self.assertIn("Preserved Records conditional-format differential styles", bad.stderr)
        self.assertNotIn("Stale edit output", bad.stderr)
        report = json.loads((self.work / "output/change-report.json").read_text())
        self.assertEqual([item["check"] for item in report["checks"] if not item["passed"]],
                         ["Preserved Records conditional-format differential styles"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
