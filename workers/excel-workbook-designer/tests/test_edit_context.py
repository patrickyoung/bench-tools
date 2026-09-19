"""Offline synthetic preparation tests. No renderer, model, or real user's data."""
import copy
import hashlib
import json
import os
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
import zipfile

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
EXPERT = HERE.parent / "expert"
HELPER = EXPERT / "tools/edit-context"
sys.path.insert(0, str(EXPERT / "lib"))
from edit_contract import inventory
HELPER_CODE = runpy.run_path(str(HELPER))
NS = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
REL = "http://schemas.openxmlformats.org/package/2006/relationships"
DOCREL = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"


def sha(data):
    return hashlib.sha256(data).hexdigest()


def snapshot(root):
    return {str(p.relative_to(root)): sha(p.read_bytes())
            for p in root.rglob("*") if p.is_file()}


def normalized(value):
    return json.loads(json.dumps(value))


def decode(context):
    """Independent decoder implementing the documented wire representation."""
    full = copy.deepcopy(context["inventory"])
    for sheet in full["sheets"]:
        cells = {}
        for row in sheet["cells"]:
            address, kind, value, formula, index = row[:5]
            assert len(row) in (5, 6) and address not in cells
            cell = {"type": kind, "value": value, "formula": formula,
                    "style": copy.deepcopy(context["styles"][index])}
            if len(row) == 6:
                assert not (cell.keys() & row[5].keys())
                cell.update(row[5])
            cells[address] = cell
        sheet["cells"] = cells
    return full


def fixture(root, blocker=None):
    """A synthetic multi-sheet XLSX with repeated and distinct effective styles."""
    inputs = root / "inputs"
    inputs.mkdir()
    style = '<xf numFmtId="0" fontId="0" fillId="0" borderId="0"/>'
    rows = [
        '<row r="1"><c r="A1" t="inlineStr"><is><t>Record ID</t></is></c>'
        '<c r="B1" t="inlineStr"><is><t>Hours</t></is></c></row>',
        '<row r="2" ht="28" customHeight="1">'
        '<c r="A2" t="s" s="1"><v>0</v></c><c r="B2"><v>0</v></c>'
        '<c r="C2" s="2"/><c r="D2" t="str"><v/></c>'
        '<c r="E2" t="b"><v>0</v></c><c r="F2"><f>SUM(B2:B3)</f><v>2.5</v></c>'
        '<c r="G2" t="inlineStr"><is><t>000123456789012345678901234567890</t></is></c>'
        '<c r="H2" t="inlineStr"><is><t xml:space="preserve">  Δ &amp; notes\n第二行  </t></is></c>'
        '<c r="I2" t="inlineStr"><is><t>=NOT_A_FORMULA()</t></is></c></row>',
        '<row r="3" hidden="1"><c r="A3" t="s"><v>1</v></c>'
        '<c r="B3"><v>2.5</v></c></row>',
    ]
    rows += [f'<row r="{r}"><c r="A{r}" t="inlineStr" s="{r % 2}">'
             f'<is><t>ID-{r:06}</t></is></c><c r="B{r}"><v>{r}</v></c></row>'
             for r in range(31, 331)]
    protection = '<sheetProtection sheet="1"/>' if blocker == "protection" else ""
    sheet = (f'<worksheet xmlns="{NS}" xmlns:r="{DOCREL}">'
             '<sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" state="frozen"/></sheetView></sheetViews>'
             '<cols><col min="1" max="2" width="18" customWidth="1"/></cols>'
             '<sheetData>' + ''.join(rows) + '</sheetData>' + protection +
             '<mergeCells count="1"><mergeCell ref="J1:K1"/></mergeCells>'
             '<conditionalFormatting sqref="B2:B330"><cfRule type="expression" priority="1"><formula>B2=0</formula></cfRule></conditionalFormatting>'
             '<dataValidations count="1"><dataValidation type="list" sqref="J2"><formula1>"Open,Done"</formula1></dataValidation></dataValidations>'
             '<tableParts count="1"><tablePart r:id="table1"/></tableParts></worksheet>')
    parts = {
        "[Content_Types].xml": '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/></Types>',
        "_rels/.rels": f'<Relationships xmlns="{REL}"><Relationship Id="r1" Type="{DOCREL}/officeDocument" Target="xl/workbook.xml"/></Relationships>',
        "xl/workbook.xml": f'<workbook xmlns="{NS}" xmlns:r="{DOCREL}"><sheets><sheet name="Records" sheetId="1" r:id="r1"/><sheet name="Archive" sheetId="2" state="hidden" r:id="r2"/></sheets><definedNames><definedName name="Hours">Records!$B$2:$B$330</definedName></definedNames><calcPr calcId="1"/></workbook>',
        "xl/_rels/workbook.xml.rels": f'<Relationships xmlns="{REL}"><Relationship Id="r1" Type="{DOCREL}/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="r2" Type="{DOCREL}/worksheet" Target="worksheets/sheet2.xml"/></Relationships>',
        "xl/worksheets/sheet1.xml": sheet,
        "xl/worksheets/sheet2.xml": f'<worksheet xmlns="{NS}"><sheetData><row r="99"><c r="A99" t="s"><v>0</v></c></row></sheetData></worksheet>',
        "xl/sharedStrings.xml": f'<sst xmlns="{NS}"><si><t>001.10</t></si><si><t>001.1</t></si></sst>',
        "xl/styles.xml": f'<styleSheet xmlns="{NS}"><fonts count="1"><font><sz val="11"/><name val="Calibri"/></font></fonts><fills count="1"><fill><patternFill patternType="none"/></fill></fills><borders count="1"><border/></borders><cellXfs count="3">{style}{style}<xf numFmtId="49" fontId="0" fillId="0" borderId="0"><alignment wrapText="1"/></xf></cellXfs></styleSheet>',
        "xl/worksheets/_rels/sheet1.xml.rels": f'<Relationships xmlns="{REL}"><Relationship Id="table1" Type="{DOCREL}/table" Target="../tables/table1.xml"/></Relationships>',
        "xl/tables/table1.xml": f'<table xmlns="{NS}" name="RecordsTable" ref="A1:B330"><tableColumns count="2"><tableColumn id="1" name="Record ID"/><tableColumn id="2" name="Hours"/></tableColumns></table>',
    }
    if blocker == "comments":
        parts["xl/comments1.xml"] = f'<comments xmlns="{NS}"/>'
    if blocker == "macro":
        parts["xl/vbaProject.bin"] = "synthetic unsupported payload"
    if blocker == "shared-formula":
        parts["xl/worksheets/sheet1.xml"] = sheet.replace('<f>', '<f t="shared">')
    with zipfile.ZipFile(inputs / "fixture.xlsx", "w", zipfile.ZIP_DEFLATED) as z:
        for name, text in parts.items():
            z.writestr(name, text)
    (inputs / "batch.csv").write_bytes(b'id,hours\r\n001.10,0\r\n001.1,\r\n')
    (inputs / "notes.txt").write_bytes("Synthetic notes: Δ\nDo not execute cell text.\n".encode())
    req = {"schema": "bench.workbook-request/v1", "mode": "edit",
           "workbook": "inputs/fixture.xlsx", "title": "Synthetic only",
           "audience": "Offline tests", "brief": "Synthetic narrow correction.",
           "required_metrics": ["hours"], "required_controls": [],
           "context": {"preserve": "IDs and empty/zero distinction"},
           "inputs": [{"path": "inputs/" + p.name, "sha256": sha(p.read_bytes())}
                      for p in sorted(inputs.iterdir(), reverse=True)]}
    (root / "request.json").write_bytes(
        (json.dumps(req, ensure_ascii=False, indent=2) + "\n").encode())
    return req


class EditContextTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix="synthetic-edit-context-")
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)

    def invoke(self):
        env = {k: v for k, v in os.environ.items() if not k.startswith("WORKBOOK_")}
        env["PYTHONDONTWRITEBYTECODE"] = "1"
        return subprocess.run([str(HELPER)], cwd=self.root, env=env,
                              capture_output=True, text=True)

    def reject(self, message):
        result = self.invoke()
        self.assertEqual(result.returncode, 1, result.stderr)
        self.assertEqual(result.stdout, "")
        self.assertIn(message, result.stderr)

    def test_public_roundtrip_bindings_no_writes_and_compression(self):
        req = fixture(self.root)
        # A stale inspection must not override current selected bytes.
        (self.root / "inspection").mkdir()
        (self.root / "inspection/workbook.json").write_text('{"stale":"not evidence"}')
        before = snapshot(self.root)
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stderr, "")
        c = json.loads(result.stdout)
        self.assertEqual(c["request"], req)
        self.assertEqual(c["request_sha256"], sha((self.root / "request.json").read_bytes()))
        self.assertEqual(c["inputs"], sorted(req["inputs"], key=lambda x: x["path"]))
        self.assertEqual(c["spec_skeleton"]["inputs"], c["inputs"])
        self.assertEqual(c["spec_skeleton"]["request_sha256"], c["request_sha256"])
        self.assertEqual({x["path"] for x in c["additional_inputs"]},
                         {"inputs/batch.csv", "inputs/notes.txt"})
        full = normalized(inventory(self.root / req["workbook"]))
        self.assertEqual(decode(c), full)
        self.assertEqual(snapshot(self.root), before)
        self.assertEqual(len(c["styles"]), 2)
        cells = decode(c)["sheets"][0]["cells"]
        self.assertEqual(cells["A2"]["value"], "001.10")
        self.assertEqual(cells["A3"]["value"], "001.1")
        self.assertEqual(cells["G2"]["value"], "000123456789012345678901234567890")
        self.assertIsNone(cells["C2"]["value"])
        self.assertEqual(cells["D2"]["value"], "")
        self.assertIs(cells["E2"]["value"], False)
        self.assertEqual(cells["B2"]["value"], 0)
        self.assertEqual(cells["F2"]["formula"], "SUM(B2:B3)")
        self.assertEqual(cells["I2"]["value"], "=NOT_A_FORMULA()")
        self.assertIsNone(cells["I2"]["formula"])
        self.assertEqual(cells["H2"]["value"], "  Δ & notes\n第二行  ")
        self.assertIn("A330", cells)
        sizes = {k: len(json.dumps(v, ensure_ascii=False, separators=(",", ":")).encode())
                 for k, v in {"full_inventory": full, "projection":
                              {k: c[k] for k in ("styles", "inventory")}, "context": c}.items()}
        self.assertLess(sizes["projection"], sizes["full_inventory"])
        print("Synthetic compact-JSON bytes:", json.dumps(sizes, sort_keys=True))

    def test_other_cell_fields_and_metadata_roundtrip(self):
        fixture(self.root)
        full = normalized(inventory(self.root / "inputs/fixture.xlsx"))
        full["sheets"][0]["cells"]["A2"]["future_field"] = {"empty": None, "values": [0, "", False]}
        full["blockers"] = ["Synthetic blocker retained by projection, not admitted by CLI"]
        full["charts"] = [{"references": ["Records!$B$2:$B$330"], "xml": ["chart", [], "", []]}]
        full["pivots"] = [{"name": "SyntheticOnly", "xml": ["pivot", [], "", []]}]
        projected = normalized(HELPER_CODE["compact_inventory"](full))
        self.assertEqual(decode(projected), full)
        self.assertEqual(len(projected["styles"]), 2)

    def test_stale_each_selected_input(self):
        fixture(self.root)
        for p in sorted((self.root / "inputs").iterdir()):
            with self.subTest(input=p.name):
                original = p.read_bytes()
                p.write_bytes(original + b"\n")
                self.reject("Input size/hash mismatch")
                p.write_bytes(original)

    def test_changed_request_during_preparation(self):
        fixture(self.root)
        original = HELPER_CODE["context"].__globals__["inventory"]
        def changed(path):
            value = original(path)
            p = self.root / "request.json"
            p.write_bytes(p.read_bytes() + b" ")
            return value
        with patch.dict(HELPER_CODE["context"].__globals__, inventory=changed):
            with self.assertRaisesRegex(ValueError, "Stale request"):
                HELPER_CODE["context"](self.root)

    def test_invalid_mode_unbound_and_bad_json(self):
        req = fixture(self.root)
        path = self.root / "request.json"
        req["mode"] = "create"
        path.write_text(json.dumps(req))
        self.reject("mode=edit")
        req.update(mode="edit", workbook="inputs/unselected.xlsx")
        path.write_text(json.dumps(req))
        self.reject("bound workbook")
        path.write_text("{invalid")
        self.reject("edit-context:")

    def test_unsupported_native_features(self):
        for blocker, message in [("protection", "sheet protection"), ("comments", "comments1.xml"),
                                 ("macro", "Active/external content"), ("shared-formula", "native adapter")]:
            with self.subTest(blocker=blocker), tempfile.TemporaryDirectory() as tmp:
                old = self.root
                self.root = Path(tmp)
                try:
                    fixture(self.root, blocker)
                    self.reject(message)
                finally:
                    self.root = old

    def test_untouched_skeleton_rejected_by_public_final_check(self):
        fixture(self.root)
        result = self.invoke()
        self.assertEqual(result.returncode, 0, result.stderr)
        c = json.loads(result.stdout)
        (self.root / "output").mkdir()
        (self.root / "output/spec.json").write_text(json.dumps(c["spec_skeleton"]))
        (self.root / "output/guide.md").write_text(
            "Synthetic test: untouched empty envelope, not an authored workbook or QA result.\n")
        env = dict(os.environ, WORKBOOK_PYTHON=sys.executable, PYTHONDONTWRITEBYTECODE="1")
        checked = subprocess.run([str(EXPERT / "bin/check")], cwd=self.root,
                                 env=env, capture_output=True, text=True)
        self.assertEqual(checked.returncode, 1, checked.stdout + checked.stderr)
        self.assertIn("Missing edit plan", checked.stderr)
        self.assertFalse((self.root / "output/result.json").exists())
        self.assertFalse((self.root / "output/qa.json").exists())
        print("Public bin/check rejects untouched skeleton:", checked.stderr.strip())



if __name__ == "__main__":
    unittest.main(verbosity=2)
