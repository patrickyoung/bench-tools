"""Read-only literal-text evidence preparation, separate from acceptance."""
import argparse
import hashlib
import json
import os
import re
from pathlib import Path
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import zipfile

from contract import bounds, cell, col, require
from edit_contract import inventory

LIMITS = dict(changed_cells=512, crops=16, columns=100, rows=10000,
              image_width=4096, image_height=2048, total_pixels=16_000_000,
              dpi=160)
ARTIFACTS = {'output/spec.json', 'output/guide.md', 'output/workbook.xlsx',
             'output/qa.json', 'output/change-report.json'}
POLICY = ('Judge the listed exact texts against the attached saved-workbook crops. '
          'The image edge is not an occupied-cell boundary. If text could continue '
          'beyond any crop edge, report uncertain, never a clipping defect based '
          'on that edge. These images alone provide no overall workbook verdict.')


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def bound_hash(value):
    require(isinstance(value, str) and re.fullmatch(r'[0-9a-f]{64}', value), 'Invalid SHA-256 binding')
    return value


def encode(value):
    return (json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + '\n').encode()


def absolute_plain(path):
    """Reject symlink components, including dangling final links, before use."""
    path = Path(path)
    require(path.is_absolute() and '..' not in path.parts, 'Need absolute plain path: ' + str(path))
    for part in [*reversed(path.parents), path]:
        require(not part.is_symlink(), 'Symlink path component: ' + str(part))
    return path


def overlaps(a, b):
    return a == b or a in b.parents or b in a.parents


def output_path(value, work, definition, excluded):
    output = absolute_plain(value)
    require(output.parent.is_dir(), 'Output parent must exist')
    require(not output.exists(), 'Output directory must be new')
    roots = [work, definition, *[Path(x).absolute() for x in excluded]]
    for name in ('AGENT_WORK', 'AGENT_HOME', 'AGENT_STATE', 'AGENT_ACTION_TMP', 'PLY_DIR', 'TMPDIR'):
        if os.environ.get(name):
            roots.append(Path(os.environ[name]).absolute())
    for root in roots:
        require(not overlaps(output.resolve(), root.resolve()),
                'Output overlaps work/definition/state/temp/excluded root: ' + str(root))
    return output


class BoundFiles:
    """Retain exact bytes and recheck pathname, identity, metadata and digest."""
    def __init__(self, root):
        self.root = absolute_plain(root)
        self.files = {}

    def _read(self, relative):
        path = Path(relative)
        require(not path.is_absolute() and path.parts and '..' not in path.parts,
                'Unsafe relative artifact path: ' + str(relative))
        absolute_plain(self.root / path)
        # Open each component with no-follow: rendering never reads mutable paths.
        fd = os.open(self.root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            for part in path.parts[:-1]:
                new_fd = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=fd)
                os.close(fd)
                fd = new_fd
            file_fd = os.open(path.parts[-1], os.O_RDONLY | os.O_NOFOLLOW, dir_fd=fd)
            with os.fdopen(file_fd, 'rb') as stream:
                start = os.fstat(stream.fileno())
                require(stat.S_ISREG(start.st_mode) and start.st_size <= 20_000_000,
                        'Missing, nonregular or oversized file: ' + relative)
                raw = stream.read(20_000_001)
                end = os.fstat(stream.fileno())
                identity = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns, s.st_ctime_ns)
                require(len(raw) <= 20_000_000 and identity(start) == identity(end),
                        'File changed while reading: ' + relative)
                return raw, identity(end)
        finally:
            os.close(fd)

    def read(self, relative, expected=None):
        raw, identity = self._read(relative)
        digest = sha(raw)
        require(expected is None or digest == expected, 'Stale binding: ' + relative)
        if relative in self.files:
            require(self.files[relative] == (raw, identity), 'Stale repeated read: ' + relative)
        self.files[relative] = raw, identity
        return raw

    def json(self, relative, expected=None):
        return json.loads(self.read(relative, expected))

    def recheck(self):
        for relative, previous in self.files.items():
            require(self._read(relative) == previous, 'Stale evidence during preparation: ' + relative)

    def digest(self, relative):
        return sha(self.files[relative][0])


def bindings(files):
    req = files.json('request.json')
    require(req.get('schema') == 'bench.workbook-request/v1' and req.get('mode') == 'edit',
            'visual-context requires a bound edit request')
    require(isinstance(req.get('brief'), str) and req['brief'].strip(), 'Missing brief')
    inputs = req.get('inputs')
    require(isinstance(inputs, list) and inputs, 'Missing selected inputs')
    paths = set()
    for item in inputs:
        require(set(item) == {'path', 'sha256'} and isinstance(item['path'], str), 'Invalid input binding')
        path = Path(item['path'])
        require(path.parts and path.parts[0] == 'inputs' and item['path'] not in paths,
                'Unsafe or duplicate selected input')
        paths.add(item['path'])
        files.read(item['path'], bound_hash(item['sha256']))
    inputs = sorted(inputs, key=lambda x: x['path'])
    require(req.get('workbook') in paths and Path(req['workbook']).suffix.lower() == '.xlsx',
            'Need selected XLSX workbook')
    receipt = files.json('output/result.json')
    require(receipt.get('schema') == 'bench.workbook-result/v1' and
            receipt.get('status') == 'accepted-mechanically' and
            receipt.get('workbook') == 'output/workbook.xlsx',
            'Need current accepted-mechanically result')
    require(receipt.get('request_sha256') == files.digest('request.json'), 'Stale mechanical request')
    artifacts = receipt.get('artifacts', [])
    require(len(artifacts) == len(ARTIFACTS) and {x.get('path') for x in artifacts} == ARTIFACTS,
            'Mechanical manifest must bind exactly the edit artifacts')
    for item in artifacts:
        files.read(item['path'], bound_hash(item['sha256']))
    spec = files.json('output/spec.json')
    require(spec.get('schema') == 'bench.workbook-edit/v1' and
            spec.get('request_sha256') == files.digest('request.json') and spec.get('inputs') == inputs,
            'Stale edit spec bindings')
    qa = files.json('output/qa.json')
    require(qa.get('schema') == 'bench.workbook-edit-qa/v1' and
            qa.get('spec_sha256') == files.digest('output/spec.json') and
            qa.get('workbook_sha256') == files.digest('output/workbook.xlsx'), 'Stale QA bindings')
    require(len(qa.get('assertions', [])) == len(spec.get('assertions', [])) and
            qa.get('assertions') and all(x.get('passed') is True for x in qa['assertions']),
            'Missing/pending mechanical assertions')
    require(len(qa.get('tests', [])) == len(spec.get('tests', [])) and
            all(x.get('passed') is True for x in qa.get('tests', [])), 'Pending mechanical tests')
    require(len(qa.get('previews', [])) == len(spec.get('previews', [])) and qa.get('previews'),
            'Missing mechanical previews')
    for preview in qa['previews']:
        path = Path(preview['path'])
        require(path.parts and path.parts[0] in ('output', 'previews'), 'Unsafe mechanical preview')
        files.read(preview['path'], bound_hash(preview['sha256']))
    report = files.json('output/change-report.json')
    require(report.get('source_sha256') == files.digest(req['workbook']) and
            report.get('output_sha256') == files.digest('output/workbook.xlsx') and
            report.get('passed') is True and all(x.get('passed') is True for x in report.get('checks', [])),
            'Stale or failed preservation report')
    return req, inputs


def workbook_inventory(path):
    with zipfile.ZipFile(path) as package:
        require(sum(x.file_size for x in package.infolist()) <= 100_000_000,
                'Expanded workbook exceeds 100MB')
    data = inventory(path)
    require(not data['blockers'], 'Unsupported workbook: ' + '; '.join(data['blockers']))
    # Importing preserved native pivots is not supported by this render path.
    require(not data.get('pivots'), 'Native pivot rendering unsupported by visual-context v1')
    require(1 <= len(data['sheets']) <= 8, 'Need 1-8 sheets')
    for sheet in data['sheets']:
        for address in sheet['cells']:
            bounds(address)
    return data


def select(before, after):
    old_sheets = {s['name']: s for s in before['sheets']}
    cells, crops = [], []
    for sheet in after['sheets']:
        changed = []
        old = old_sheets.get(sheet['name'], {}).get('cells', {})
        ordered_cells = sorted(sheet['cells'].items(), key=lambda x: cell(x[0]))
        for address, value in ordered_cells:
            previous = old.get(address)
            if value.get('formula') is not None or not isinstance(value.get('value'), str) or value.get('type') not in ('s', 'str', 'inlineStr'):
                continue
            if previous is not None and previous.get('formula') is None and isinstance(previous.get('value'), str) and previous.get('type') in ('s', 'str', 'inlineStr') and previous['value'] == value['value']:
                continue
            changed.append({'id': 'cell-%03d' % (len(cells) + len(changed) + 1), 'sheet': sheet['name'],
                'cell': address, 'text': value['value'], 'type': 'string', 'storage_type': value.get('type'),
                'previous': ({k: previous.get(k) for k in ('value', 'type', 'formula')} if previous is not None else None)})
        if not changed:
            continue
        require(len(cells) + len(changed) <= LIMITS['changed_cells'], 'Changed-cell coverage exceeds 512')
        require(len(crops) < LIMITS['crops'], 'Image coverage exceeds 16')
        require(sheet['state'] == 'visible', 'Changed text on hidden sheet: ' + sheet['name'])
        last_row = max(cell(a)[0] for a in sheet['cells'])
        last_column = max(cell(a)[1] for a in sheet['cells'])
        for row in sheet['rows']:
            require(row.get('hidden') not in ('1', 'true'), 'Hidden row on changed-text sheet: ' + sheet['name'])
        for column in sheet['columns']:
            require(not (int(column['min']) <= last_column and (column.get('hidden') in ('1', 'true') or float(column.get('width', 1)) <= 0)),
                    'Hidden/zero-width column on changed-text sheet: ' + sheet['name'])
        require(all(float(height) > 0 for height in sheet['row_heights'].values()), 'Zero-height row on changed-text sheet')
        if sheet['merges']:
            for merge in sheet['merges'][3]:
                r, c, R, C = bounds(dict(merge[1])['ref'])
                for target in changed:
                    row, column = cell(target['cell'])
                    require(not (r <= row <= R and c <= column <= C and (row, column) != (r, c)),
                            'Selected text in non-anchor merged cell: ' + sheet['name'] + '!' + target['cell'])
        populated_rows = sorted({cell(a)[0] for a, v in sheet['cells'].items() if v.get('value') is not None or v.get('formula') is not None})
        header_rows = populated_rows[:3]
        wanted_rows = set(header_rows)
        for target in changed:
            row, _ = cell(target['cell'])
            wanted_rows.update(range(max(1, row - 1), min(LIMITS['rows'], row + 1) + 1))
        row_index = {}
        for address, value in ordered_cells:
            row, _ = cell(address)
            if row in wanted_rows and (value.get('value') is not None or value.get('formula') is not None):
                row_index.setdefault(row, []).append({'cell': address, 'value': value.get('value'), 'type': value.get('type'), 'formula': value.get('formula')})
        rows = [{'row': row, 'cells': row_index.get(row, [])} for row in sorted(wanted_rows)]
        crop_id = 'crop-%02d' % (len(crops) + 1)
        for target in changed:
            target['crop_id'] = crop_id
        crops.append({'id': crop_id, 'sheet': sheet['name'], 'range': 'A1:' + col(last_column) + str(last_row),
                      'range_meaning': 'Saved inventory address extent only; not a cell-to-pixel mapping or PDF crop.',
                      'cells': [{'id': x['id'], 'cell': x['cell'], 'text': x['text'], 'row_context': cell(x['cell'])[0]} for x in changed],
                      'rows': rows, 'header_candidate_rows': header_rows,
                      'header_context_policy': 'First three nonempty saved rows; candidate orientation context, not an inferred header verdict.',
                      'edges': {'whole_sheet_pdf_page': True, 'no_blank_guard_guaranteed': True,
                                'crop_edges_are_not_cell_clipping_evidence': True},
                      'review_policy': POLICY})
        cells.extend(changed)
    require(sum(len(x['cells']) for x in crops) == len(cells), 'Incomplete selected-cell coverage')
    return cells, crops


def executable_identity(path):
    canonical = Path(path).resolve(strict=True)
    require(canonical.is_file() and os.access(canonical, os.X_OK), 'Need regular executable: ' + str(path))
    before = canonical.stat()
    h = hashlib.sha256()
    with canonical.open('rb') as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            h.update(chunk)
    after = canonical.stat()
    identity = lambda st: (st.st_dev, st.st_ino, st.st_size, st.st_mtime_ns, st.st_ctime_ns)
    require(identity(before) == identity(after), 'Executable changed while hashing')
    return {'selected_path': str(path), 'path': str(canonical), 'sha256': h.hexdigest(), 'file_identity': list(identity(after))}


def selected_executables(work, definition, excluded):
    names = {'soffice': 'WORKBOOK_VISUAL_SOFFICE', 'pdftoppm': 'WORKBOOK_VISUAL_PDFTOPPM',
             'cage': 'WORKBOOK_VISUAL_CAGE', 'python': 'WORKBOOK_PYTHON'}
    roots = [work, definition, *[Path(x).absolute() for x in excluded]]
    for name in ('AGENT_WORK', 'AGENT_HOME', 'AGENT_STATE', 'AGENT_ACTION_TMP', 'PLY_DIR', 'TMPDIR'):
        if os.environ.get(name):
            roots.append(Path(os.environ[name]).absolute())
    selected = {}
    for key, name in names.items():
        value = os.environ.get(name)
        require(value and Path(value).is_absolute(), 'Need caller-selected ' + name + ' absolute executable')
        info = executable_identity(value)
        require(not any(overlaps(Path(info['path']), root.resolve()) for root in roots),
                'Render executable overlaps worker/state/temp/definition root: ' + name)
        selected[key] = info
    return selected


def run_renderer(command, runtime, render):
    env = {**os.environ, 'TMPDIR': str(render / 'tmp'), 'XDG_CACHE_HOME': str(render / 'cache'),
           'XDG_CONFIG_HOME': str(render / 'config'), 'PYTHONDONTWRITEBYTECODE': '1'}
    for name in ('tmp', 'cache', 'config'):
        (render / name).mkdir(mode=0o700)
    with subprocess.Popen(command, cwd=runtime, env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                          text=True, start_new_session=True) as process:
        try:
            out, err = process.communicate(timeout=180)
        except (subprocess.TimeoutExpired, KeyboardInterrupt):
            os.killpg(process.pid, signal.SIGTERM)
            try:
                process.communicate(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                process.communicate()
            raise ValueError('Visual renderer timed out/interrupted; no fallback')
    require(process.returncode == 0, 'Confined saved-workbook render failed: ' + err[-12000:])


def prepare(work, definition, output, excluded=()):
    work, definition = absolute_plain(work), absolute_plain(definition)
    output = output_path(output, work, definition, excluded)
    files = BoundFiles(work)
    req, inputs = bindings(files)
    code = BoundFiles(definition)
    for relative in ('tools/visual-context', 'lib/visual_context.py', 'lib/visual_pages.py',
                     'lib/contract.py', 'lib/edit_contract.py'):
        code.read(relative)
    # Private sibling staging is outside the same denied roots as the destination.
    with tempfile.TemporaryDirectory(prefix='.visual-context-', dir=output.parent) as temporary:
        staging = Path(temporary)
        runtime = staging / 'runtime'
        runtime.mkdir(mode=0o700)
        published = staging / 'published'
        published.mkdir(mode=0o700)
        for name, relative in [('source.xlsx', req['workbook']), ('saved.xlsx', 'output/workbook.xlsx')]:
            (runtime / name).write_bytes(files.files[relative][0])
        before = workbook_inventory(runtime / 'source.xlsx')
        after = workbook_inventory(runtime / 'saved.xlsx')
        cells, crops = select(before, after)
        plan = {'limits': LIMITS, 'crops': crops, 'sheet_names': [x['name'] for x in after['sheets']]}
        selected = {}
        if crops:
            selected = selected_executables(work, definition, excluded)
            plan['executables'] = selected
            require(len(encode(plan)) <= 8 * 1024 * 1024, 'Visual context metadata exceeds 8MB')
            (runtime / 'visual_pages.py').write_bytes(code.files['lib/visual_pages.py'][0])
            (runtime / 'plan.json').write_bytes(encode(plan))
            render_dir = staging / 'render'
            render_dir.mkdir(mode=0o700)
            command = [selected['cage']['path'], '-w', str(render_dir), '--', selected['python']['path'],
                       str(runtime / 'visual_pages.py'), str(runtime / 'saved.xlsx'),
                       str(runtime / 'plan.json'), str(render_dir)]
            run_renderer(command, runtime, render_dir)
            rendered = json.loads((render_dir / 'rendered.json').read_bytes())
            require(len(rendered['crops']) == len(crops), 'Incomplete rendered sheet coverage')
            crops = rendered['crops']
            rendered_files = BoundFiles(render_dir)
            assets = [rendered['pdf']] + [crop[kind] for crop in crops for kind in ('image', 'layout')]
            for asset in assets:
                raw = rendered_files.read(asset['path'], bound_hash(asset['sha256']))
                (published / asset['path']).write_bytes(raw)
        else:
            command = None
            rendered = {'total_pixels': 0, 'versions': {}, 'commands': [], 'pdf': None}
        images = BoundFiles(published)
        for asset in ([rendered['pdf']] if rendered['pdf'] else []) + [crop[kind] for crop in crops for kind in ('image', 'layout')]:
            images.read(asset['path'], bound_hash(asset['sha256']))
        mapped = [c['id'] for crop in crops for c in crop['cells']]
        require(sorted(mapped) == sorted(x['id'] for x in cells), 'Incomplete rendered cell coverage')
        manifest = {
            'schema': 'bench.workbook-visual-context/v1', 'status': 'prepared',
            'scope': 'Added or changed literal string cells only; exact decoded value/semantic type comparison, including empty strings and formula-to-literal changes. Unchanged strings, formatting-only changes, removed strings and formula results are outside v1 scope. Independent broader image review is separate.',
            'request_sha256': files.digest('request.json'),
            'spec_sha256': files.digest('output/spec.json'),
            'qa_sha256': files.digest('output/qa.json'),
            'result_sha256': files.digest('output/result.json'),
            'source': {'path': req['workbook'], 'sha256': files.digest(req['workbook'])},
            'workbook': {'path': 'output/workbook.xlsx', 'sha256': files.digest('output/workbook.xlsx')},
            'inputs': inputs,
            'mechanical_evidence': [{'path': p, 'sha256': files.digest(p)} for p in sorted(files.files)],
            'preparer': [{'path': p, 'sha256': code.digest(p)} for p in sorted(code.files)],
            'pdf': rendered['pdf'],
            'render': {'backend': 'libreoffice-single-page-sheets/v1',
                       'api': 'LibreOffice CLI calc_pdf_Export SinglePageSheets; Poppler pdftoppm',
                       'saved_bytes_reimported': bool(crops), 'dpi': LIMITS['dpi'],
                       'executables': {key: {**value, 'version': rendered['versions'].get(key)} for key, value in selected.items()},
                       'pypdf_version': rendered['versions'].get('pypdf'),
                       'argv': command, 'commands': rendered['commands'],
                       'confinement': {'public_cage': bool(crops), 'network': 'denied',
                                       'write_grant': 'private render staging only; TMPDIR inside that staging',
                                       'host_reads': 'unrestricted by Cage'}},
            'output_directory': str(output), 'cells': cells, 'crops': crops,
            'coverage': {'selected_cells': len(cells), 'mapped_cells': len(mapped), 'crops': len(crops),
                         'total_pixels': rendered['total_pixels']},
            'limits': LIMITS, 'review_policy': POLICY,
            'limitations': ['No model call, automatic formatting, clipping measurement or quality verdict.',
                           'Mechanical receipt is retained caller-selected evidence, not independently authenticated provenance.',
                           'PNG evidence uses LibreOffice print rendering, not native Microsoft Excel or Artifact Tool pixels.',
                           'Full sheet pages have no guaranteed blank guard or cell pixel coordinates. Ambiguous localization or possible page-edge cutoff must remain uncertain.'],
        }
        for value in selected.values():
            require(executable_identity(value['selected_path']) == value, 'Stale render executable')
        files.recheck()
        code.recheck()
        output_path(output, work, definition, excluded)
        encoded = encode(manifest)
        require(len(encoded) <= 8 * 1024 * 1024, 'Visual context metadata exceeds 8MB')
        (published / 'manifest.json').write_bytes(encoded)
        # Caller must provide a private stable output parent; no overwrite permitted.
        output.mkdir(mode=0o700)
        try:
            for item in published.iterdir():
                if item.name != 'manifest.json':
                    item.rename(output / item.name)
            files.recheck()
            code.recheck()
            evidence = BoundFiles(output)
            for asset in ([rendered['pdf']] if rendered['pdf'] else []) + [crop[kind] for crop in crops for kind in ('image', 'layout')]:
                evidence.read(asset['path'], asset['sha256'])
            for value in selected.values():
                require(executable_identity(value['selected_path']) == value, 'Stale render executable')
            (published / 'manifest.json').rename(output / 'manifest.json')
        except Exception:
            shutil.rmtree(output)
            raise
    return manifest


def main():
    parser = argparse.ArgumentParser(description='Prepare complete bound saved-workbook images for changed literal text; no verdict.')
    parser.add_argument('--out', required=True, help='New absolute evidence directory outside all worker write grants')
    parser.add_argument('--exclude-root', action='append', default=[], help='Additional selected state/action write root to exclude (repeatable)')
    args = parser.parse_args()
    try:
        result = prepare(Path.cwd(), Path(__file__).resolve().parents[1], args.out, args.exclude_root)
        print(encode(result).decode(), end='')
        return 0
    except Exception as exc:
        print('visual-context: ' + str(exc), file=sys.stderr)
        return 1
