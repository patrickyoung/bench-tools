#!/usr/bin/env python3
"""Build reproducible v0.1 release archives locally; never publish or tag."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
TARGETS = ('darwin-arm64', 'darwin-amd64', 'linux-arm64', 'linux-amd64')


def source_files():
    names = (ROOT / 'scripts' / 'release-files.txt').read_text().splitlines()
    if len(names) != len(set(names)):
        raise ValueError('duplicate release source entry')
    if not {'main.go', 'go.mod', 'scripts/release.py', 'scripts/release-files.txt'}.issubset(names):
        raise ValueError('release source list omits required build or packaging files')
    data = {}
    for name in sorted(names):
        relative = Path(name)
        if (not name or not relative.parts or relative.is_absolute() or '..' in relative.parts or
                name != relative.as_posix() or relative.parts[0] in ('var', 'dist', '.git')):
            raise ValueError('invalid release source entry: ' + name)
        path = ROOT / relative
        if any(parent.is_symlink() for parent in [path, *list(path.parents)[:len(relative.parts)-1]]) or not path.is_file():
            raise ValueError('missing or indirect release source: ' + str(path))
        data[name] = (path.read_bytes(), 0o755 if path.stat().st_mode & 0o111 else 0o644)
    return data


def build_environment():
    env = {key: value for key, value in os.environ.items()
           if not key.startswith(('GO', 'CGO_'))}
    env.update({'GOENV': 'off', 'GOTOOLCHAIN': 'local', 'GOWORK': 'off',
                'GOFLAGS': '', 'CGO_ENABLED': '0', 'GOAMD64': 'v1', 'GOARM64': 'v8.0'})
    return env


def archive(prefix, files):
    stream = io.BytesIO()
    with gzip.GzipFile(fileobj=stream, mode='wb', filename='', mtime=0) as zipped:
        with tarfile.open(fileobj=zipped, mode='w') as tar:
            for name, (body, mode) in sorted(files.items()):
                entry = tarfile.TarInfo(prefix + '/' + name)
                entry.size, entry.mode, entry.mtime = len(body), mode, 0
                tar.addfile(entry, io.BytesIO(body))
    return stream.getvalue()


def write_once(path, body):
    if path.exists():
        if path.read_bytes() != body:
            raise ValueError('release artifact differs; choose a fresh --output directory: ' + str(path))
        return
    with open(path, 'xb') as stream:
        stream.write(body)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, default=ROOT / 'dist')
    parser.add_argument('--targets', nargs='+', choices=TARGETS, default=list(TARGETS))
    args = parser.parse_args()
    if len(set(args.targets)) != len(args.targets):
        parser.error('duplicate targets')
    source = source_files()
    match = re.search(r'^\s*version\s*=\s*"([0-9]+\.[0-9]+\.[0-9]+)"', source['main.go'][0].decode(), re.M)
    if not match:
        raise ValueError('cannot read the CLI version')
    version = match[1]
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    env = build_environment()
    metadata = {'version': version,
                'sources': {name: hashlib.sha256(body).hexdigest() for name, (body, _) in source.items()},
                'targets': sorted(args.targets), 'flags': ['-trimpath', '-buildvcs=false'], 'cgo': False,
                'go_environment': {key: env[key] for key in ('GOENV', 'GOTOOLCHAIN', 'GOWORK', 'GOFLAGS', 'GOAMD64', 'GOARM64')}}
    artifacts = {'weave-' + version + '-source.tar.gz': archive('weave-' + version, source)}
    with tempfile.TemporaryDirectory(prefix='weave-release-') as temp:
        staging = Path(temp) / 'source'
        for name, (body, mode) in source.items():
            path = staging / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(body)
            path.chmod(mode)
        metadata['go'] = subprocess.check_output(['go', 'version'], text=True, env=env, cwd=staging).strip()
        for target in sorted(args.targets):
            platform, architecture = target.split('-')
            binary = Path(temp) / target
            target_env = {**env, 'GOOS': platform, 'GOARCH': architecture}
            print('building ' + target, file=sys.stderr)
            subprocess.run(['go', 'build', '-trimpath', '-buildvcs=false', '-o', str(binary), '.'],
                           cwd=staging, env=target_env, check=True)
            contents = {**source, 'weave': (binary.read_bytes(), 0o755)}
            name = 'weave-' + version + '-' + target
            artifacts[name + '.tar.gz'] = archive(name, contents)
    artifacts['build.json'] = (json.dumps(metadata, sort_keys=True, indent=2) + '\n').encode()
    checksums = ''.join(hashlib.sha256(body).hexdigest() + '  ' + name + '\n' for name, body in sorted(artifacts.items()))
    artifacts['SHA256SUMS'] = checksums.encode()
    # Refuse changed output before writing anything from this invocation.
    if any(path.name not in artifacts or path.is_symlink() or not path.is_file() for path in output.iterdir()):
        raise ValueError('output contains unexpected files; choose a fresh --output directory')
    for name, body in artifacts.items():
        if (output / name).exists() and (output / name).read_bytes() != body:
            raise ValueError('release artifact differs; choose a fresh --output directory: ' + str(output / name))
    for name, body in artifacts.items():
        write_once(output / name, body)
    print(str(output))


if __name__ == '__main__':
    try:
        main()
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print('release: ' + str(error), file=sys.stderr)
        sys.exit(2)
    except KeyboardInterrupt:
        print('release: interrupted', file=sys.stderr)
        sys.exit(130)
