"""File formats and validation for this example application.

No process execution, queue, model loop, protocol, mutable globals or tool imports.
All authority-bearing directories are explicit function arguments.
"""
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import tempfile

MAX_INPUT = 1024 * 1024
MAX_FILES = 8 * 1024 * 1024
MAX_ENVELOPE = 20 * 1024 * 1024
TAIL = 24000

def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':'), ensure_ascii=True).encode()


def identifier(value):
    if not isinstance(value, str) or not re.fullmatch(r'[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}', value):
        raise ValueError('identifier must be 1-64 letters, digits, underscores or hyphens')
    return value


def read_json(path, private=False):
    if private:
        info = path.lstat()
        if info.st_mode & 0o077 or info.st_uid != os.getuid():
            raise RuntimeError('private configuration must be owned by this account and mode 0600')
    return json.loads(read_regular(path, MAX_ENVELOPE + 1))


def durable_json(path, value):
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as out:
        tmp = Path(out.name)
        try:
            out.write(canonical(value))
            out.flush()
            os.fsync(out.fileno())
            tmp.replace(path)
            sync_dir(path.parent)
        finally:
            tmp.unlink(missing_ok=True)


def sync_dir(path):
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def relative_path(value):
    if not isinstance(value, str) or len(value) > 512:
        raise ValueError('invalid relative file path')
    parts = value.split('/')
    if len(parts) > 16 or any(not p or p in ('.', '..') or len(p) > 128 or
                              any(ord(c) < 32 or c == '\\' for c in p) for p in parts):
        raise ValueError('invalid relative file path')
    return parts


def read_regular(path, size, tail=False):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, 'rb') as stream:
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
            raise ValueError('output must be a regular file without hard links')
        if tail:
            stream.seek(max(0, info.st_size - size))
        return stream.read(size)


def config(root):
    return json.loads((root / 'deployment.json').read_text())


def verify(root):
    cfg = config(root)
    for name, digest in cfg['adapter_sha256'].items():
        if hashlib.sha256((root / name).read_bytes()).hexdigest() != digest:
            raise ValueError('deployment adapter changed: ' + name)
    lock = json.loads((root / 'definition' / (cfg['kind'] + '.lock.json')).read_text())
    for name, entry in lock['files'].items():
        path = root / 'definition' / name
        if path.is_symlink() or hashlib.sha256(path.read_bytes()).hexdigest() != entry['sha256']:
            raise ValueError('definition changed: ' + name)
    return cfg


def validate_credentials(profile, root):
    allowed = set(read_json(root / 'installed.json')['pass_env'])
    if not isinstance(profile, dict) or not profile.get('ASK_MODEL'):
        raise ValueError('client credentials must be a JSON object containing ASK_MODEL')
    for key, value in profile.items():
        if (key not in allowed or not re.fullmatch(r'[A-Z][A-Z0-9_]*', key)
                or key in {'PATH', 'HOME', 'TMPDIR', 'PYTHONPATH', 'PYTHONHOME', 'ENV', 'BASH_ENV'}
                or key.startswith(('DOCKER_', 'LD_', 'DYLD_', 'TEND_', 'SSH_', 'BENCH_'))
                or not isinstance(value, str) or '\0' in value):
            raise ValueError('client credential name/value is not admitted')


def validate_submission(session, key, request, files, cfg):
    identifier(session)
    identifier(key)
    if not isinstance(request, str) or not request.strip() or len(request.encode()) > MAX_INPUT:
        raise ValueError('request must be nonempty text, at most 1 MiB')
    if cfg['kind'] == 'team' and cfg['name'] != 'page-team' and cfg.get('entry') != 'agent':
        json.loads(request)
    if not isinstance(files, dict) or len(files) > 64:
        raise ValueError('files must map at most 64 relative paths to base64 data')
    size = 0
    paths = set()
    for name, encoded in files.items():
        parts = relative_path(name)
        if parts[0] == 'request.txt' or not isinstance(encoded, str):
            raise ValueError('request.txt is reserved; file values must be base64 strings')
        try:
            size += len(base64.b64decode(encoded, validate=True))
        except ValueError:
            raise ValueError('invalid base64 file data') from None
        paths.add(name)
    if size > MAX_FILES:
        raise ValueError('selected files exceed 8 MiB')
    if any('/'.join(name.split('/')[:i]) in paths for name in paths
           for i in range(1, len(name.split('/')))):
        raise ValueError('file paths overlap')


def state_path(value):
    path = Path(value).expanduser()
    if not path.is_absolute():
        raise ValueError('--state requires an absolute directory')
    return path.resolve()


def settings(state, root):
    info = state.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_mode & 0o077 or info.st_uid != os.getuid():
        raise RuntimeError('state must be an account-owned private directory, mode 0700')
    cfg = read_json(state / 'config.json', private=True)
    if (cfg.get('deployment') != str(root) or
            cfg.get('deployment_sha256') != hashlib.sha256((root / 'deployment.json').read_bytes()).hexdigest()):
        raise RuntimeError('state is not bound to this deployment; do not reuse or move an existing queue')
    for key, maximum in [('workers', 32), ('max_pending', 10000), ('max_jobs', 100000)]:
        if type(cfg.get(key)) is not int or not 1 <= cfg[key] <= maximum:
            raise RuntimeError('invalid service setting: ' + key)
    if not isinstance(cfg.get('job_timeout'), str) or not cfg['job_timeout']:
        raise RuntimeError('job_timeout must be a positive Tend duration')
    return cfg


def client_config(state, client, root):
    identifier(client)
    settings(state, root)
    try:
        profile = read_json(state / 'clients' / (client + '.json'), private=True)
        validate_credentials(profile, root)
    except (ValueError, TypeError, KeyError) as e:
        raise RuntimeError('invalid client configuration') from e
    return profile


def scope_digest(scope):
    return hashlib.sha256(str(scope).encode()).hexdigest()


def container_name(job, scope):
    return 'bench-' + scope_digest(scope)[:12] + '-' + job
