"""Portable preparation contract tests; actual confined LibreOffice/PDF/PNG, no models.

Synthetic mechanical receipts are fixture inputs, not claims of end-user quality.
The separate retained Sales proof uses an actual completed mechanical run.
"""
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch
import zipfile
from xml.sax.saxutils import escape

sys.dont_write_bytecode = True
EXPERT = Path(__file__).resolve().parents[1] / 'expert'
sys.path.insert(0, str(EXPERT / 'lib'))
import visual_context as visual

S = 'http://schemas.openxmlformats.org/spreadsheetml/2006/main'
R = 'http://schemas.openxmlformats.org/officeDocument/2006/relationships'
P = 'http://schemas.openxmlformats.org/package/2006/relationships'


def put(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(data))


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def xlsx(path, sheets, *, row_height=20, column_width=18, storage='inlineStr'):
    """Standard minimal OOXML fixture, consumed by the actual documented API."""
    shared = []
    parts = {
        '_rels/.rels': f'<Relationships xmlns="{P}"><Relationship Id="r1" Type="{R}/officeDocument" Target="xl/workbook.xml"/></Relationships>',
        'xl/styles.xml': f'<styleSheet xmlns="{S}"><fonts count="1"><font><name val="Arial"/><sz val="11"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/></cellXfs></styleSheet>',
    }
    sheet_tags, relations, overrides = [], [], []
    for i, (name, cells) in enumerate(sheets.items(), 1):
        rows = {}
        for address, value in cells.items():
            row, _ = visual.cell(address)
            if isinstance(value, tuple):
                content = '<f>' + escape(value[0]) + '</f><v>' + escape(value[1]) + '</v>'
                kind = 'str'
            elif isinstance(value, str):
                kind = storage
                if storage == 's':
                    shared.append(value)
                    content = '<v>' + str(len(shared) - 1) + '</v>'
                else:
                    content = '<is><t xml:space="preserve">' + escape(value) + '</t></is>'
            else:
                kind, content = 'n', '<v>' + str(value) + '</v>'
            rows.setdefault(row, []).append(f'<c r="{address}" t="{kind}">{content}</c>')
        body = ''.join(f'<row r="{row}" ht="{row_height}" customHeight="1">' + ''.join(cells) + '</row>' for row, cells in sorted(rows.items()))
        parts[f'xl/worksheets/sheet{i}.xml'] = f'<worksheet xmlns="{S}"><cols><col min="1" max="6" width="{column_width}" customWidth="1"/></cols><sheetData>{body}</sheetData></worksheet>'
        sheet_tags.append(f'<sheet name="{name}" sheetId="{i}" r:id="r{i}"/>')
        relations.append(f'<Relationship Id="r{i}" Type="{R}/worksheet" Target="worksheets/sheet{i}.xml"/>')
        overrides.append(f'<Override PartName="/xl/worksheets/sheet{i}.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>')
    relations.append(f'<Relationship Id="styles" Type="{R}/styles" Target="styles.xml"/>')
    if shared:
        parts['xl/sharedStrings.xml'] = f'<sst xmlns="{S}" count="{len(shared)}" uniqueCount="{len(shared)}">' + ''.join('<si><t xml:space="preserve">' + escape(s) + '</t></si>' for s in shared) + '</sst>'
        relations.append(f'<Relationship Id="strings" Type="{R}/sharedStrings" Target="sharedStrings.xml"/>')
        overrides.append('<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/>')
    parts['xl/workbook.xml'] = f'<workbook xmlns="{S}" xmlns:r="{R}"><sheets>' + ''.join(sheet_tags) + '</sheets></workbook>'
    parts['xl/_rels/workbook.xml.rels'] = f'<Relationships xmlns="{P}">' + ''.join(relations) + '</Relationships>'
    parts['[Content_Types].xml'] = '<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/xml"/><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/>' + ''.join(overrides) + '</Types>'
    path.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(path, 'w', zipfile.ZIP_DEFLATED) as z:
        for name, content in parts.items():
            z.writestr(name, content)


def bind_fixture(work):
    inputs = [{'path': 'inputs/source.xlsx', 'sha256': digest(work / 'inputs/source.xlsx')}]
    put(work / 'request.json', {'schema': 'bench.workbook-request/v1', 'mode': 'edit', 'brief': 'Synthetic preparation fixture', 'workbook': inputs[0]['path'], 'inputs': inputs})
    put(work / 'output/spec.json', {'schema': 'bench.workbook-edit/v1', 'request_sha256': digest(work / 'request.json'), 'inputs': inputs, 'assertions': [{'metric': 'fixture'}], 'tests': [], 'previews': [{'sheet': 'Data', 'range': 'A1:F4'}]})
    (work / 'output/guide.md').write_text('Synthetic fixture; no independent acceptance or visual-quality claim.\n')
    (work / 'output/old-preview.png').write_bytes(b'Previous preview bytes are hash-bound but never chosen for new visual evidence.')
    put(work / 'output/qa.json', {'schema': 'bench.workbook-edit-qa/v1', 'spec_sha256': digest(work / 'output/spec.json'), 'workbook_sha256': digest(work / 'output/workbook.xlsx'), 'assertions': [{'passed': True}], 'tests': [], 'previews': [{'path': 'output/old-preview.png', 'sha256': digest(work / 'output/old-preview.png')}]})
    put(work / 'output/change-report.json', {'source_sha256': inputs[0]['sha256'], 'output_sha256': digest(work / 'output/workbook.xlsx'), 'passed': True, 'checks': [{'passed': True}]})
    put(work / 'output/result.json', {'schema': 'bench.workbook-result/v1', 'status': 'accepted-mechanically', 'workbook': 'output/workbook.xlsx', 'request_sha256': digest(work / 'request.json'), 'artifacts': [{'path': p, 'sha256': digest(work / p)} for p in sorted(visual.ARTIFACTS)]})


class VisualContext(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix='visual-context-test-', dir=Path(__file__).resolve().parent)
        self.addCleanup(temporary.cleanup)
        self.base = Path(temporary.name)
        self.work = self.base / 'work'
        self.work.mkdir()
        self.source = self.work / 'inputs/source.xlsx'
        self.saved = self.work / 'output/workbook.xlsx'
        xlsx(self.source, {'Data': {'A1': 'Unchanged', 'B2': 'Old', 'C2': 7, 'D2': ('"fixed"', 'fixed')}})
        xlsx(self.saved, {'Data': {'A1': 'Unchanged', 'B2': ' Changed ', 'C2': '7', 'D2': 'fixed', 'E2': 'Added'}, 'Added': {'A1': 'New sheet'}})
        bind_fixture(self.work)
        self.env = {k: v for k, v in os.environ.items() if k not in ('AGENT_WORK', 'AGENT_HOME', 'AGENT_STATE', 'AGENT_ACTION_TMP', 'PLY_DIR', 'TMPDIR')}
        self.env['PYTHONDONTWRITEBYTECODE'] = '1'

    def command(self, out='evidence', *args):
        target = self.base / out
        result = subprocess.run([str(EXPERT / 'tools/visual-context'), '--out', str(target), *args], cwd=self.work, env=self.env, capture_output=True, text=True, timeout=120)
        return result, target

    def test_actual_runtime_complete_text_mapping_and_unchanged_workspace(self):
        self.assertTrue(self.env.get('WORKBOOK_VISUAL_SOFFICE'), 'Select the actual LibreOffice/Cage/Poppler runtime')
        before = {str(p.relative_to(self.work)): digest(p) for p in self.work.rglob('*') if p.is_file()}
        result, target = self.command()
        self.assertEqual(result.returncode, 0, result.stderr)
        data = json.loads(result.stdout)
        self.assertEqual(data, json.loads((target / 'manifest.json').read_text()))
        self.assertEqual({(x['sheet'], x['cell'], x['text']) for x in data['cells']},
                         {('Data', 'B2', ' Changed '), ('Data', 'C2', '7'), ('Data', 'D2', 'fixed'), ('Data', 'E2', 'Added'), ('Added', 'A1', 'New sheet')})
        self.assertEqual(data['coverage']['selected_cells'], 5)
        self.assertEqual(data['coverage']['mapped_cells'], 5)
        self.assertEqual(data['coverage']['crops'], 2)
        self.assertEqual(data['crops'][0]['range'], 'A1:E2')
        self.assertTrue(data['crops'][0]['edges']['whole_sheet_pdf_page'])
        self.assertTrue(data['crops'][0]['edges']['no_blank_guard_guaranteed'])
        self.assertEqual(data['pdf']['pages'], 2)
        self.assertEqual(data['pdf']['page_map'], [{'sheet': 'Data', 'page': 1}, {'sheet': 'Added', 'page': 2}])
        self.assertEqual(digest(target / data['pdf']['path']), data['pdf']['sha256'])
        self.assertTrue(data['render']['confinement']['public_cage'])
        self.assertNotIn('-net', data['render']['argv'])
        self.assertIn('uncertain', data['review_policy'])
        self.assertNotIn('verdict', data)
        for crop in data['crops']:
            self.assertEqual(digest(target / crop['image']['path']), crop['image']['sha256'])
            self.assertEqual(digest(target / crop['layout']['path']), crop['layout']['sha256'])
            self.assertEqual(crop['layout']['schema'], 'bench.workbook-page-context/v1')
            self.assertTrue(all('frame_px' not in c for c in crop['cells']))
            self.assertTrue(crop['rows'])
        after = {str(p.relative_to(self.work)): digest(p) for p in self.work.rglob('*') if p.is_file()}
        self.assertEqual(before, after)

    def test_storage_encoding_only_is_zero_scope_without_runtime(self):
        xlsx(self.source, {'Data': {'A1': 'unchanged'}}, storage='s')
        xlsx(self.saved, {'Data': {'A1': 'unchanged'}}, storage='inlineStr')
        bind_fixture(self.work)
        for name in ('WORKBOOK_VISUAL_SOFFICE', 'WORKBOOK_VISUAL_PDFTOPPM', 'WORKBOOK_VISUAL_CAGE', 'WORKBOOK_PYTHON'):
            self.env.pop(name, None)
        result, _ = self.command()
        self.assertEqual(result.returncode, 0, result.stderr)
        data = json.loads(result.stdout)
        self.assertEqual(data['coverage'], {'selected_cells': 0, 'mapped_cells': 0, 'crops': 0, 'total_pixels': 0})
        self.assertFalse(data['render']['saved_bytes_reimported'])
        self.assertEqual(data['status'], 'prepared')

    def test_every_bound_file_change_rejected(self):
        for name in ('request.json', 'inputs/source.xlsx', *visual.ARTIFACTS, 'output/old-preview.png'):
            with self.subTest(name=name):
                path = self.work / name
                raw = path.read_bytes()
                path.write_bytes(raw + b' ')
                result, target = self.command()
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(target.exists())
                path.write_bytes(raw)

    def test_symlink_ancestor_and_file_rejected(self):
        for name in ('inputs', 'output/qa.json'):
            with self.subTest(name=name):
                path = self.work / name
                stash = self.base / 'original'
                path.rename(stash)
                path.symlink_to(stash, target_is_directory=stash.is_dir())
                result, target = self.command()
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('Symlink', result.stderr)
                self.assertFalse(target.exists())
                path.unlink()
                stash.rename(path)

    def test_external_output_grants_existing_and_symlink_rejected(self):
        for target, extra, env in [(self.work / 'new', [], {}), (EXPERT / 'new', [], {}),
                (self.base / 'new', ['--exclude-root', str(self.base)], {}),
                (self.base / 'new', [], {'AGENT_STATE': str(self.base)}),
                (self.base / 'new', [], {'AGENT_ACTION_TMP': str(self.base)}),
                (self.base / 'new', [], {'TMPDIR': str(self.base)})]:
            with self.subTest(target=target, extra=extra, env=env):
                result = subprocess.run([str(EXPERT / 'tools/visual-context'), '--out', str(target), *extra], cwd=self.work, env={**self.env, **env}, capture_output=True, text=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn('overlaps', result.stderr)
                self.assertFalse(target.exists())
        (self.base / 'exists').mkdir()
        result, _ = self.command('exists')
        self.assertNotEqual(result.returncode, 0)
        (self.base / 'link').symlink_to(self.base / 'exists', target_is_directory=True)
        result, _ = self.command('link/new')
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Symlink', result.stderr)

    def test_selection_limits_hidden_and_non_anchor_merge(self):
        before = visual.workbook_inventory(self.source)
        original = visual.workbook_inventory(self.saved)
        cases = []
        for where, value in [('state', 'hidden'), ('rows', [{'r': '2', 'hidden': '1'}]),
                             ('columns', [{'min': '2', 'max': '2', 'hidden': '1'}]),
                             ('merges', ['mergeCells', [], '', [['mergeCell', [['ref', 'A2:B2']], '', []]]])]:
            after = copy.deepcopy(original)
            after['sheets'][0][where] = value
            cases.append(after)
        for count, step in ((513, 1),):
            after = copy.deepcopy(original)
            after['sheets'] = [after['sheets'][0]]
            after['sheets'][0]['cells'] = {'A' + str(1 + i * step): {'value': 'x', 'type': 'str', 'formula': None} for i in range(count)}
            cases.append(after)
        for after in cases:
            with self.subTest(case=len(after['sheets'][0]['cells'])), self.assertRaises(ValueError):
                visual.select(before, after)

    def test_actual_runtime_oversized_crop_has_no_partial_result(self):
        xlsx(self.source, {'Data': {'A1': 'Old'}}, row_height=1800)
        xlsx(self.saved, {'Data': {'A1': 'Changed'}}, row_height=1800)
        bind_fixture(self.work)
        result, target = self.command()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('pixel budget', result.stderr)
        self.assertFalse(target.exists())
        self.assertFalse(list(self.base.glob('.visual-context-*')))

    def test_stale_during_render_rejected_and_no_partial_manifest(self):
        original = visual.run_renderer
        def mutate_after(*args, **kwargs):
            result = original(*args, **kwargs)
            with (self.work / 'output/guide.md').open('a') as stream:
                stream.write('Changed after render')
            return result
        with patch.dict(os.environ, self.env, clear=True), patch.object(visual, 'run_renderer', mutate_after):
            with self.assertRaisesRegex(ValueError, 'Stale evidence'):
                visual.prepare(self.work, EXPERT, self.base / 'evidence')
        self.assertFalse((self.base / 'evidence').exists())

    def test_empty_literal_is_selected_in_page_evidence(self):
        xlsx(self.saved, {'Data': {'A1': 'Visible anchor', 'B2': ''}})
        bind_fixture(self.work)
        selected, _ = visual.select(visual.workbook_inventory(self.source), visual.workbook_inventory(self.saved))
        self.assertIn(('B2', ''), [(x['cell'], x['text']) for x in selected])
        result, target = self.command()
        self.assertEqual(result.returncode, 0, result.stderr)
        data = json.loads(result.stdout)
        self.assertIn(('B2', ''), [(x['cell'], x['text']) for x in data['cells']])

    def test_pdf_mapping_never_guesses_page_order_or_missing_bookmarks(self):
        program = """import sys
sys.path.insert(0, sys.argv[1])
from visual_pages import page_mapping, page_geometry
class Reader:
    pages=[None,None]
    def __init__(self, outline): self.outline=outline
    def get_destination_page_number(self, item): return item['page']
def item(name, page): return {'/Title':name,'page':page}
assert page_mapping(Reader([item('Beta',0),item('Alpha',1)]), ['Alpha','Beta']) == [{'sheet':'Beta','page':1},{'sheet':'Alpha','page':2}]
for entries in [[], [item('Alpha',0)], [item('Alpha',0),item('Alpha',1)], [item('Alpha',0),item('Beta',0)], [item('Alpha',0),item('Other',1)], [[item('Alpha',0)],item('Beta',1)], [item('Alpha',0),item('Beta',2)]]:
    try: page_mapping(Reader(entries), ['Alpha','Beta'])
    except ValueError: pass
    else: raise AssertionError(entries)
class Page:
    mediabox=[0,0,72,144];cropbox=[0,0,72,144];rotation=0
assert page_geometry(Page(),160) == ([0.0,0.0,72.0,144.0],160,320)
for attr,value in [('rotation',90),('cropbox',[0,0,70,144]),('mediabox',[1,0,72,144]),('mediabox',[0,0,float('inf'),144])]:
    page=Page();setattr(page,attr,value)
    try: page_geometry(page,160)
    except ValueError: pass
    else: raise AssertionError((attr,value))
"""
        result = subprocess.run([self.env['WORKBOOK_PYTHON'], '-c', program, str(EXPERT / 'lib')], env=self.env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 0, result.stderr)

    def test_all_page_dimensions_checked_before_first_raster(self):
        xlsx(self.source, {'Data': {'A1': 'Old'}, 'Tall': {'A1': 'Old'}})
        xlsx(self.saved, {'Data': {'A1': 'Changed'}, 'Tall': {'A1': 'Changed'}}, row_height=1800)
        # Keep the first sheet small while the second exceeds the fixed-DPI cap.
        with zipfile.ZipFile(self.saved) as z:
            parts = {name: z.read(name) for name in z.namelist()}
        parts['xl/worksheets/sheet1.xml'] = parts['xl/worksheets/sheet1.xml'].replace(b'ht="1800"', b'ht="20"')
        with zipfile.ZipFile(self.saved, 'w', zipfile.ZIP_DEFLATED) as z:
            for name, raw in parts.items(): z.writestr(name, raw)
        bind_fixture(self.work)
        wrapper = self.base / 'poppler-probe'
        wrapper.write_text('#!/bin/sh\nif [ "$1" = "-v" ]; then printf "test poppler\\n"; exit 0; fi\nprintf EARLY_RASTER >&2\nexit 93\n')
        wrapper.chmod(0o755)
        self.env['WORKBOOK_VISUAL_PDFTOPPM'] = str(wrapper)
        result, target = self.command()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('pixel budget: Tall', result.stderr)
        self.assertNotIn('EARLY_RASTER', result.stderr)
        self.assertFalse(target.exists())

    def test_actual_cage_only_private_writes_and_no_network(self):
        listener = socket.socket()
        self.addCleanup(listener.close)
        listener.bind(('127.0.0.1', 0)); listener.listen(1)
        outside = self.base / 'forbidden.txt'
        wrapper = self.base / 'soffice-boundary-probe'
        wrapper.write_text('#!' + str(Path(self.env['WORKBOOK_PYTHON']).resolve()) + '\n' + """import os,socket,sys
from pathlib import Path
Path(os.environ['TMPDIR'],'allowed').write_text('allowed')
try:
    Path(""" + repr(str(outside)) + """).write_text('forbidden')
    print('outside write escaped',file=sys.stderr);sys.exit(71)
except PermissionError: pass
s=socket.socket();s.settimeout(1)
try:
    s.connect(('127.0.0.1',""" + str(listener.getsockname()[1]) + """))
    print('network escaped',file=sys.stderr);sys.exit(72)
except (PermissionError,OSError): pass
print('private_write=allowed outside_write=denied network=denied',file=sys.stderr)
sys.exit(73)
""")
        wrapper.chmod(0o755)
        self.env['WORKBOOK_VISUAL_SOFFICE'] = str(wrapper)
        result, target = self.command()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('private_write=allowed outside_write=denied network=denied', result.stderr)
        self.assertFalse(outside.exists())
        self.assertFalse(target.exists())

    def test_unavailable_cage_does_not_fall_back(self):
        self.env['WORKBOOK_VISUAL_CAGE'] = '/usr/bin/false'
        result, target = self.command()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('Confined saved-workbook render failed', result.stderr)
        self.assertFalse(target.exists())

    def test_missing_runtime_is_not_success(self):
        self.env.pop('WORKBOOK_VISUAL_SOFFICE', None)
        result, target = self.command()
        self.assertNotEqual(result.returncode, 0)
        self.assertIn('WORKBOOK_VISUAL_SOFFICE', result.stderr)
        self.assertFalse(target.exists())


if __name__ == '__main__':
    unittest.main(verbosity=2)
