#!/usr/bin/env python3
"""Build independent, pinned component sources for this deployment image."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('source', type=Path)
parser.add_argument('prefix', type=Path)
parser.add_argument('components', nargs='*')
parser.add_argument('--deployment', type=Path,
                    help='select runtime or Hire builder from deployment.json')
args = parser.parse_args()
source, prefix = args.source, args.prefix
# Docker supplies the action boundary instead of Cage. Authoring is explicit.
selection = set(args.components) or {'agent', 'ask', 'brief', 'ply', 'record',
                                    'mcp', 'tend', 'weave'}
if args.deployment:
    if args.components:
        parser.error('--deployment cannot be combined with component names')
    deployment = json.loads(args.deployment.read_text())
    if deployment.get('entry') == 'hire':
        selection.add('hire')
components = json.loads((source / 'components.json').read_text())['components']
unknown = selection - {component['name'] for component in components}
if unknown:
    parser.error('unknown components: ' + ', '.join(sorted(unknown)))
(prefix / 'bin').mkdir(parents=True, exist_ok=True)
env = dict(os.environ, GOWORK='off', CGO_ENABLED='0')
for component in components:
    if component['name'] not in selection:
        continue
    directory = source / component['path']
    for command in component['commands']:
        subprocess.run(['go', 'build', '-mod=readonly', '-trimpath', '-buildvcs=false',
                        '-o', str(prefix / 'bin' / command['name']), command['package']],
                       cwd=directory, env=env, check=True)
    docs = prefix / 'share' / component['name']
    docs.mkdir(parents=True, exist_ok=True)
    for path in directory.iterdir():
        if path.is_file() and (path.name == 'LICENSE' or path.suffix in ('.md', '.1')):
            shutil.copy2(path, docs / path.name)
