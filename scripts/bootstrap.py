#!/usr/bin/env python3
"""Install the examples' pinned public dependencies into a private bin directory."""
import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bin', type=Path, default=ROOT / 'var' / 'tools')
    args = parser.parse_args()
    if not shutil.which('go') or not shutil.which('git'):
        parser.error('Go 1.26 or later and Git are required')
    target = args.bin.resolve()
    target.mkdir(parents=True, exist_ok=True)
    pins = json.loads((ROOT / 'scripts' / 'dependencies.json').read_text())
    if (not isinstance(pins, dict) or set(pins) != {'ask', 'ply', 'tend'} or
            any(not isinstance(value, str) for value in pins.values())):
        raise ValueError('dependency pins must name exactly ask, ply and tend')
    # Install into staging first; failed downloads never replace working tools.
    with tempfile.TemporaryDirectory(prefix='.weave-bootstrap-', dir=target) as temp:
        env = {key: value for key, value in os.environ.items()
               if not key.startswith(('GO', 'CGO_'))}
        env.update({'GOBIN': temp, 'GOWORK': 'off', 'GOENV': 'off',
                    'GOTOOLCHAIN': 'local', 'GOFLAGS': '', 'CGO_ENABLED': '0'})
        for name, module in sorted(pins.items()):
            if not re.fullmatch(r'github\.com/patrickyoung/' + name + r'@[0-9a-f]{40}', module):
                raise ValueError('invalid pinned module: ' + name)
            print('building ' + module, file=sys.stderr)
            repository, revision = module.split('@')
            source = Path(temp) / ('source-' + name)
            subprocess.run(['git', 'init', '-q', str(source)], check=True)
            subprocess.run(['git', '-C', str(source), 'fetch', '-q', '--depth=1',
                            'https://' + repository + '.git', revision], check=True)
            subprocess.run(['git', '-C', str(source), 'checkout', '-q', '--detach', 'FETCH_HEAD'], check=True)
            actual = subprocess.check_output(['git', '-C', str(source), 'rev-parse', 'HEAD'], text=True).strip()
            if actual != revision:
                raise ValueError('fetched revision differs from pin: ' + name)
            subprocess.run(['go', 'build', '-trimpath', '-buildvcs=false', '-o', str(Path(temp) / name), '.'],
                           cwd=source, env=env, check=True)
        subprocess.run(['go', 'build', '-trimpath', '-buildvcs=false', '-o', str(Path(temp) / 'weave'), '.'],
                       cwd=ROOT, env=env, check=True)
        for name in [*sorted(pins), 'weave']:
            os.replace(Path(temp) / name, target / name)
    print('Example tools installed in ' + str(target))
    print('Run: ./tests/check --bin ' + str(target))


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print('bootstrap: ' + str(error), file=sys.stderr)
        sys.exit(2)
    except KeyboardInterrupt:
        print('bootstrap: interrupted', file=sys.stderr)
        sys.exit(130)
