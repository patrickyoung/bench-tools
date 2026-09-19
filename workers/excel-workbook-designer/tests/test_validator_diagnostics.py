"""Synthetic, offline validation diagnostics through real request/spec boundaries."""
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import zipfile

sys.dont_write_bytecode = True
EXPERT = Path(os.environ.get('WORKBOOK_EXPERT', Path(__file__).resolve().parents[1] / 'expert'))
sys.path.insert(0, str(EXPERT / 'lib'))
import contract
import edit_contract

S = 'http://schemas.openxmlformats.org/spreadsheetml/2006/main'
R = 'http://schemas.openxmlformats.org/officeDocument/2006/relationships'
P = 'http://schemas.openxmlformats.org/package/2006/relationships'


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def fixture(root, mode='edit'):
    (root / 'inputs').mkdir(exist_ok=True)
    (root / 'output').mkdir(exist_ok=True)
    if mode == 'edit':
        source = root / 'inputs/source.xlsx'
        parts = {
            'xl/workbook.xml': f'<workbook xmlns="{S}" xmlns:r="{R}"><sheets><sheet name="Records" sheetId="1" r:id="r1"/></sheets></workbook>',
            'xl/_rels/workbook.xml.rels': f'<Relationships xmlns="{P}"><Relationship Id="r1" Type="{R}/worksheet" Target="worksheets/sheet1.xml"/></Relationships>',
            'xl/worksheets/sheet1.xml': f'<worksheet xmlns="{S}"><sheetData><row r="2"><c r="B2"><v>7</v></c><c r="C2"><v>2</v></c></row></sheetData></worksheet>',
            'xl/styles.xml': f'<styleSheet xmlns="{S}"><fonts count="1"><font/></fonts><fills count="1"><fill/></fills><borders count="1"><border/></borders><cellXfs count="1"><xf fontId="0" fillId="0" borderId="0" numFmtId="0"/></cellXfs></styleSheet>',
        }
        with zipfile.ZipFile(source, 'w') as archive:
            for name, data in parts.items():
                archive.writestr(name, data)
    else:
        source = root / 'inputs/source.csv'
        source.write_text('name,amount\nA,7\nB,2\n')
    inputs = [{'path': source.relative_to(root).as_posix(), 'sha256': digest(source)}]
    req = {'schema': 'bench.workbook-request/v1', 'brief': 'Update the selected records.', 'inputs': inputs,
           'required_metrics': ['total'], 'required_controls': []}
    if mode == 'edit':
        req.update(mode='edit', workbook=inputs[0]['path'])
    (root / 'request.json').write_text(json.dumps(req))
    common = {'request_sha256': digest(root / 'request.json'), 'inputs': inputs,
              'metrics': [{'id': 'total', 'sheet': 'Records', 'cell': 'B2'}, {'id': 'count', 'sheet': 'Records', 'cell': 'C2'}],
              'tests': [{'name': 'zero amount', 'edits': [{'sheet': 'Records', 'cell': 'B2', 'value': 0}], 'expect': [{'metric': 'total', 'value': 0}]},
                        {'name': 'next record', 'edits': [{'sheet': 'Records', 'cell': 'B3', 'value': 4}], 'expect': [{'metric': 'total', 'value': 11}, {'metric': 'count', 'value': 3}]}]}
    if mode == 'edit':
        return {**common, 'schema': 'bench.workbook-edit/v1', 'purpose': 'Update', 'interpretation': 'Selected cells',
                'changes': [{'sheet': 'Records', 'range': 'B2:C3', 'reason': 'Requested update'}],
                'operations': [{'op': 'format', 'sheet': 'Records', 'range': 'B2', 'format': {'numberFormat': '0'}},
                               {'op': 'values', 'sheet': 'Records', 'range': 'B2:C3', 'values': [[7, 2], [4, None]]}],
                'assertions': [{'metric': 'total', 'value': 7}, {'metric': 'count', 'value': 2}],
                'previews': [{'sheet': 'Records', 'range': 'A1:D4'}]}
    spec = {**common, 'schema': 'bench.workbook-spec/v1', 'title': 'Records', 'purpose': 'Totals', 'audience': 'Reader',
            'target_engine': 'Excel', 'design_notes': [], 'limitations': [], 'controls': [], 'issues': [],
            'update_policy': {'instructions': 'Edit rows.', 'capacity': '2 rows.', 'filter_scope': 'All rows.'},
            'lineage': [{'source': inputs[0]['path'], 'locator': 'Rows 2-3', 'target': 'Records!A2:B3', 'note': 'Direct'}],
            'sheets': [{'name': 'Records', 'role': 'output', 'display_range': 'A1:D4',
                        'blocks': [{'range': 'A1:B1', 'values': [['name', 'amount']]},
                                   {'range': 'A2:B3', 'values': [['A', 7], ['B', 2]]},
                                   {'range': 'D2:D3', 'formulas': [['=SUM(B2:B3)'], ['=COUNTA(A2:A3)']]}],
                        'tables': [{'name': 'Records', 'range': 'A1:B3'}]}]}
    spec['metrics'][0]['cell'] = 'D2'
    spec['metrics'][1]['cell'] = 'D3'
    spec['tests'].append({'name': 'blank amount', 'edits': [{'sheet': 'Records', 'cell': 'B2', 'value': None}], 'expect': [{'metric': 'total', 'value': 2}]})
    return spec


def save(root, spec):
    (root / 'output/spec.json').write_text(json.dumps(spec))


def validate(root, spec):
    save(root, spec)
    return (edit_contract.validate if spec['schema'] == 'bench.workbook-edit/v1' else contract.validate_spec)(root)


class ValidatorDiagnostics(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix='validator-diagnostics-')
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.spec = fixture(self.root)

    def error(self, spec, *fragments):
        with self.assertRaises(ValueError) as caught:
            validate(self.root, spec)
        for fragment in fragments:
            self.assertIn(fragment, str(caught.exception))
        self.assertEqual(json.loads((self.root / 'output/spec.json').read_text()), spec)

    def test_valid_edit_and_creation_are_read_only(self):
        for mode in ('edit', 'create'):
            with self.subTest(mode=mode):
                spec = fixture(self.root, mode)
                save(self.root, spec)
                before = {p.relative_to(self.root): p.read_bytes() for p in self.root.rglob('*') if p.is_file()}
                result = validate(self.root, spec)
                self.assertEqual(result[1], spec)
                after = {p.relative_to(self.root): p.read_bytes() for p in self.root.rglob('*') if p.is_file()}
                self.assertEqual(after, before)

    def test_baseline_unknown_metric_names_exact_assertion_and_known_ids(self):
        self.spec['assertions'][1]['metric'] = 'missing_count'
        self.error(self.spec, 'Unknown assertion', 'assertions[1].metric', "unknown metric ID 'missing_count'", "known IDs: ['count', 'total']")

    def test_edit_test_unknown_metric_names_test_and_expectation(self):
        self.spec['tests'][1]['expect'][1]['metric'] = 'missing_count'
        self.error(self.spec, 'Unknown test metric', "tests[1] ('next record').expect[1].metric", "unknown metric ID 'missing_count'", "known IDs: ['count', 'total']")

    def test_matrix_diagnostic_reports_range_and_actual_dimensions(self):
        for matrix, actual in [([[1, 2]], '1 rows with column counts [2]'),
                               ([[1], [2, 3, 4]], '2 rows with column counts [1, 3]'),
                               ([[1, 2], None], "2 rows with column counts [2, 'NoneType (no column count)']"),
                               (None, 'NoneType (no matrix dimensions)')]:
            for kind in ('values', 'formulas'):
                with self.subTest(matrix=matrix, kind=kind):
                    spec = copy.deepcopy(self.spec)
                    spec['operations'][1] = {'op': kind, 'sheet': 'Records', 'range': 'B2:C3', kind: matrix}
                    self.error(spec, 'Bad matrix shape', f'operations[1] ({kind})', "sheet='Records'", "range='B2:C3'", 'expected 2 rows x 2 columns', 'actual '+actual)

    def test_scope_error_names_operation_and_missing_containing_scope(self):
        self.spec['operations'][1]['range'] = 'D2:E3'
        self.error(self.spec, 'Operation outside scope', 'operations[1] (values)', "sheet='Records'", "range='D2:E3'", "missing containing changes range for 'Records'!D2:E3", "declared ranges on this sheet: ['B2:C3']")
        self.spec['changes'][0]['sheet'] = 'Elsewhere'
        self.error(self.spec, 'declared ranges on this sheet: []')

    def test_creation_block_diagnostic_names_sheet_block_and_shape(self):
        spec = fixture(self.root, 'create')
        spec['sheets'][0]['blocks'][1]['values'] = [['A', 7]]
        self.error(spec, 'Block shape', 'sheets[0].blocks[1] (values)', "sheet='Records'", "range='A2:B3'", 'expected 2 rows x 2 columns', 'actual 1 rows with column counts [2]')

    def test_creation_test_unknown_metric_names_test_and_known_ids(self):
        spec = fixture(self.root, 'create')
        spec['tests'][1]['expect'][1]['metric'] = 'missing_count'
        self.error(spec, 'Unknown metric', "tests[1] ('next record').expect[1].metric", "unknown metric ID 'missing_count'", "known IDs: ['count', 'total']")

    def test_public_check_exposes_diagnostic_without_acceptance_or_writes(self):
        self.spec['assertions'][1]['metric'] = 'missing_count'
        save(self.root, self.spec)
        before = {p.relative_to(self.root): p.read_bytes() for p in self.root.rglob('*') if p.is_file()}
        env = dict(os.environ, WORKBOOK_PYTHON=sys.executable, PYTHONDONTWRITEBYTECODE='1')
        result = subprocess.run([str(EXPERT / 'bin/check')], cwd=self.root, env=env, capture_output=True, text=True)
        self.assertEqual(result.returncode, 1)
        self.assertEqual(result.stdout, '')
        self.assertIn('assertions[1].metric', result.stderr)
        self.assertIn("known IDs: ['count', 'total']", result.stderr)
        after = {p.relative_to(self.root): p.read_bytes() for p in self.root.rglob('*') if p.is_file()}
        self.assertEqual(after, before)


def verdicts():
    """Same synthetic inputs can be evaluated against any selected worker copy."""
    results = {}
    with tempfile.TemporaryDirectory(prefix='validator-verdicts-') as d:
        root = Path(d)
        for mode in ('edit', 'create'):
            base = fixture(root, mode)
            cases = [('valid', base)]
            for value in ('missing', 'TOTAL', None, 5):
                s = copy.deepcopy(base)
                s['tests'][1]['expect'][1]['metric'] = value
                cases.append(('test-id-'+repr(value), s))
                if mode == 'edit':
                    s = copy.deepcopy(base)
                    s['assertions'][1]['metric'] = value
                    cases.append(('assertion-id-'+repr(value), s))
            for n, matrix in enumerate(([[1, 2], [3, 4]], [[1]], [[1, 2]], [[1], [2, 3, 4]], [[1, 2], None], None, [], [[None, 0], [False, 'x']], ['ab', 'cd'], [[1, 2], [3, 4], [5, 6]])):
                s = copy.deepcopy(base)
                target = s['operations'][1] if mode == 'edit' else s['sheets'][0]['blocks'][1]
                target['values'] = matrix
                cases.append(('matrix-'+str(n), s))
            if mode == 'edit':
                for changed_range in ('B2:C3', 'A1:D4', 'B2:B3', 'B2:C2', 'D2:E3'):
                    s = copy.deepcopy(base)
                    s['changes'][0]['range'] = changed_range
                    cases.append(('scope-'+changed_range, s))
                s = copy.deepcopy(base)
                s['operations'][1] = {'op': 'chart', 'sheet': 'Records', 'range': 'G2:H4', 'type': 'bar', 'from': 'J2', 'to': 'N8'}
                cases.append(('chart-existing-scope-exception', s))
            for name, spec in cases:
                try:
                    validate(root, spec)
                    results[mode+'/'+name] = 'accepted'
                except Exception:
                    results[mode+'/'+name] = 'rejected'
    return results


if __name__ == '__main__':
    if '--verdicts' in sys.argv:
        print(json.dumps(verdicts(), sort_keys=True))
    else:
        unittest.main()
