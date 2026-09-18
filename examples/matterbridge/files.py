"""Private application files and literal command contracts; no tool imports."""
import contextlib
import fcntl
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile
import urllib.error
import urllib.request


def read(path, limit=16 * 1024 * 1024):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, 'rb') as stream:
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size > limit:
            raise ValueError('expected a bounded regular file')
        return stream.read(limit + 1)


def load(path):
    return json.loads(read(path, 64 * 1024 * 1024 if path.name == 'app.json' else 16 * 1024 * 1024))


def save(path, value):
    data = json.dumps(value, sort_keys=True, ensure_ascii=True).encode()
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as stream:
        temporary = Path(stream.name)
        try:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
            temporary.replace(path)
            fd = os.open(path.parent, os.O_RDONLY | os.O_DIRECTORY)
            try:
                os.fsync(fd)
            finally:
                os.close(fd)
        finally:
            temporary.unlink(missing_ok=True)


@contextlib.contextmanager
def lock(path):
    fd = os.open(path, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
    try:
        fcntl.flock(fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
        yield
    finally:
        os.close(fd)


def config(state):
    info = state.lstat()
    if not stat.S_ISDIR(info.st_mode) or info.st_mode & 0o077 or info.st_uid != os.getuid():
        raise ValueError('state must be an owned directory with mode 0700')
    return load(state / 'config.json')


def secret(path):
    path = Path(path)
    info = path.lstat()
    if info.st_mode & 0o077 or info.st_uid != os.getuid():
        raise ValueError('credential files must be owned by this account and mode 0600')
    return read(path, 65536).decode().strip()


def credential(cfg, name):
    if name + '_env' in cfg:
        value = os.environ.get(cfg[name + '_env'], '').strip()
        if not value or len(value.encode()) > 65536:
            raise ValueError('missing or excessive credential environment variable: ' + cfg[name + '_env'])
        return value
    return secret(cfg[name + '_file'])


def provider(cfg):
    if 'provider_env' in cfg:
        value = {name: os.environ[name] for name in cfg['provider_env'] if os.environ.get(name)}
        if not value.get('ASK_MODEL'):
            raise ValueError('ASK_MODEL must be set in the service environment')
        return value
    return json.loads(secret(cfg['provider_file']))


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True).encode()).hexdigest()


def command(argv, value=None, timeout=60):
    # No shell, no credential values in argv or returned diagnostics.
    result = subprocess.run([str(a) for a in argv],
                            input=json.dumps(value).encode() if value is not None else b'',
                            capture_output=True, timeout=timeout)
    if result.returncode:
        raise RuntimeError(Path(argv[0]).name + ' exited ' + str(result.returncode))
    return json.loads(result.stdout) if result.stdout.strip() else {}


def service(binding, operation, value=None):
    return command([Path(binding['package']) / 'service', '--state', binding['state'],
                    operation, 'chat'], value)


def http_json(url, payload=None, token=None):
    headers = {'Content-Type': 'application/json'}
    if token:
        headers['Authorization'] = 'Bearer ' + token
    request = urllib.request.Request(url,
        data=json.dumps(payload).encode() if payload is not None else None, headers=headers)
    # Do not follow redirects with credentials, including Telegram's token URL.
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *args, **kwargs):
            return None
    try:
        with urllib.request.build_opener(NoRedirect).open(request, timeout=20) as response:
            data = response.read(16 * 1024 * 1024 + 1)
            if len(data) > 16 * 1024 * 1024:
                raise RuntimeError('network response exceeds limit')
            return 'OK' if data == b'OK' else json.loads(data)
    except (OSError, ValueError) as error:
        # urllib errors include URLs; Telegram puts the secret in the URL.
        raise RuntimeError('network request failed; outcome may be unknown') from None


def topic(state, cfg, directory, name):
    """Never repeat a create request whose response could have been lost."""
    marker = directory / 'topic.json'
    if marker.exists():
        result = load(marker)
        if result.get('id'):
            return result['id']
        raise RuntimeError('topic creation uncertain; use attach-topic after inspecting Telegram')
    save(marker, {'status': 'unknown'})
    token = credential(cfg, 'telegram_token')
    result = http_json('https://api.telegram.org/bot' + token + '/createForumTopic',
                      {'chat_id': cfg['telegram_chat_id'], 'name': name[:128]})
    if not result.get('ok'):
        raise RuntimeError('Telegram refused topic creation; inspect topic.json')
    topic_id = result['result']['message_thread_id']
    save(marker, {'status': 'created', 'id': topic_id})
    return topic_id
