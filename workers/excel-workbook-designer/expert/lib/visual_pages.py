"""Trusted public PDF renderer, invoked inside Cage with private writes only."""
import hashlib
import json
import math
from pathlib import Path
import struct
import subprocess
import sys
import time

from pypdf import PdfReader, __version__ as pypdf_version


def need(value, message):
    if not value:
        raise ValueError(message)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write_json(path, value):
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2, allow_nan=False) + '\n')


def page_mapping(reader, names):
    """Require exact explicit mapping; never infer sheet names from page order."""
    need(len(names) == len(set(names)), 'Duplicate saved sheet names')
    need(len(reader.pages) == len(names), 'PDF page count differs from saved sheet count')
    outline = reader.outline
    need(len(outline) == len(names) and all(isinstance(x, dict) for x in outline),
         'Need one flat explicit PDF sheet bookmark per saved sheet')
    mapped = []
    for item in outline:
        name = item.get('/Title')
        page = reader.get_destination_page_number(item)
        need(isinstance(name, str) and name in names and isinstance(page, int) and 0 <= page < len(names),
             'Invalid PDF sheet bookmark')
        mapped.append({'sheet': name, 'page': page + 1})
    need({x['sheet'] for x in mapped} == set(names) and len({x['page'] for x in mapped}) == len(names),
         'Duplicate or missing sheet/page bookmark mapping')
    return sorted(mapped, key=lambda x: x['page'])


def page_geometry(page, dpi):
    box, crop = list(map(float, page.mediabox)), list(map(float, page.cropbox))
    need(all(math.isfinite(n) for n in box) and box[0:2] == [0.0, 0.0] and box == crop and page.rotation == 0,
         'Unsupported PDF page geometry or rotation')
    width, height = box[2], box[3]
    need(width > 0 and height > 0, 'Empty PDF page geometry')
    return box, math.ceil(width * dpi / 72), math.ceil(height * dpi / 72)


def render(input_path, plan_path, output):
    plan = json.loads(plan_path.read_text())
    limits, tools = plan['limits'], plan['executables']
    commands = []
    original = sha(input_path)

    def run(argv):
        start = time.monotonic()
        result = subprocess.run(argv, cwd=output, capture_output=True, text=True, timeout=90)
        commands.append({'argv': argv, 'exit_code': result.returncode, 'elapsed_seconds': time.monotonic() - start,
                         'stdout': result.stdout, 'stderr': result.stderr})
        need(result.returncode == 0, 'Render command failed: ' + str(argv[0]) + ': ' + result.stderr[-4000:])
        return result.stdout + result.stderr

    profile = '-env:UserInstallation=' + (output / 'profile').as_uri()
    versions = {'python': sys.version, 'pypdf': pypdf_version}
    versions['cage'] = run([tools['cage']['path'], 'version']).strip()
    versions['soffice'] = run([tools['soffice']['path'], profile, '--version']).strip()
    versions['pdftoppm'] = run([tools['pdftoppm']['path'], '-v']).strip()
    filter_options = json.dumps({'SinglePageSheets': {'type': 'boolean', 'value': 'true'},
                                 'ExportBookmarks': {'type': 'boolean', 'value': 'true'}}, separators=(',', ':'))
    run([tools['soffice']['path'], profile, '--headless', '--convert-to',
         'pdf:calc_pdf_Export:' + filter_options, '--outdir', str(output), str(input_path)])
    generated = output / (input_path.stem + '.pdf')
    need(generated.is_file() and not generated.is_symlink() and 0 < generated.stat().st_size <= 20_000_000,
         'Missing, unsafe or oversized rendered PDF')
    pdf = output / 'workbook.pdf'
    generated.rename(pdf)
    reader = PdfReader(pdf, strict=True)
    mapping = page_mapping(reader, plan['sheet_names'])
    pages = {item['sheet']: item['page'] for item in mapping}
    total = 0
    planned = []
    for crop in plan['crops']:
        page_number = pages[crop['sheet']]
        box, width, height = page_geometry(reader.pages[page_number - 1], limits['dpi'])
        need(width <= limits['image_width'] and height <= limits['image_height'],
             'Full-sheet PDF exceeds image pixel budget: ' + crop['sheet'])
        total += width * height
        need(total <= limits['total_pixels'], 'Full-sheet PDF exceeds total pixel budget')
        planned.append((crop, page_number, box, width, height))
    # Every page is checked before any rasterization; no auto-downscaling or tiles.
    crops = []
    for crop, page_number, box, width, height in planned:
        prefix = output / crop['id']
        run([tools['pdftoppm']['path'], '-f', str(page_number), '-l', str(page_number),
             '-singlefile', '-r', str(limits['dpi']), '-png', str(pdf), str(prefix)])
        image = prefix.with_suffix('.png')
        need(image.is_file() and not image.is_symlink(), 'Missing or unsafe raster output')
        raw = image.read_bytes()
        need(len(raw) >= 24 and raw[:8] == b'\x89PNG\r\n\x1a\n' and raw[12:16] == b'IHDR', 'Expected PNG raster')
        actual_width, actual_height = struct.unpack('>II', raw[16:24])
        need((actual_width, actual_height) == (width, height), 'PNG dimensions differ from fixed-DPI PDF geometry')
        layout_path = output / (crop['id'] + '.layout.json')
        layout = {'schema': 'bench.workbook-page-context/v1', 'sheet': crop['sheet'], 'page': page_number,
                  'page_box_points': box, 'dpi': limits['dpi'], 'inventory_range': crop['range'],
                  'geometry_policy': 'Whole sheet page only. Inventory addresses and row context are not pixel coordinates. No cell glyph or fit measurements.',
                  'cells': crop['cells'], 'rows': crop['rows'], 'header_candidate_rows': crop['header_candidate_rows'],
                  'localization_policy': 'Use exact sheet/page, cell/text and neighboring row/header facts. If the pictured target cannot be located unambiguously, the review must be uncertain.'}
        write_json(layout_path, layout)
        crops.append({**crop, 'page': page_number,
                      'image': {'path': image.name, 'sha256': sha(image), 'width': width, 'height': height},
                      'layout': {'path': layout_path.name, 'sha256': sha(layout_path), 'schema': layout['schema'],
                                 'coordinate_system': 'PDF sheet page; no cell-to-pixel geometry'},
                      'render_options': {'page': page_number, 'dpi': limits['dpi'], 'format': 'png', 'whole_sheet': True}})
    need(sha(input_path) == original, 'Saved XLSX snapshot changed during export')
    write_json(output / 'rendered.json', {'crops': crops, 'total_pixels': total, 'versions': versions, 'commands': commands,
        'pdf': {'path': pdf.name, 'sha256': sha(pdf), 'pages': len(reader.pages), 'page_map': mapping}})


if __name__ == '__main__':
    try:
        render(Path(sys.argv[1]), Path(sys.argv[2]), Path(sys.argv[3]))
    except Exception as exc:
        print('visual-pages: ' + str(exc), file=sys.stderr)
        raise SystemExit(1)
