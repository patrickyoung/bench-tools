#!/usr/bin/env python3
"""Bind standing-team commitments to existing team commands and Tend."""
import argparse
import calendar as month_calendar
from contextlib import contextmanager
from datetime import date, datetime, time, timedelta, timezone
import fcntl
import hashlib
import html
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import threading
import time as wall_time
from zoneinfo import ZoneInfo

VERSION = '0.4.0'
MAX_JSON = 2 * 1024 * 1024
MAX_FILE = 64 * 1024 * 1024
UTC = timezone.utc


def require(condition, message):
    if not condition:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


def encode(value):
    return (json.dumps(value, sort_keys=True, ensure_ascii=False, allow_nan=False,
                       separators=(',', ':')) + '\n').encode()


def pairs(items):
    result = {}
    for key, value in items:
        require(key not in result, 'duplicate JSON key: ' + key)
        result[key] = value
    return result


def decode(data):
    require(len(data) <= MAX_JSON, 'JSON exceeds 2 MiB')
    value = json.loads(data.decode('utf-8'), object_pairs_hook=pairs,
                       parse_constant=lambda s: require(False, 'invalid number: ' + s))
    def walk(item, depth=0):
        require(depth <= 32, 'JSON exceeds depth limit')
        if isinstance(item, dict):
            for key, child in item.items():
                key.encode('utf-8')
                walk(child, depth + 1)
        elif isinstance(item, list):
            for child in item:
                walk(child, depth + 1)
        elif isinstance(item, str):
            item.encode('utf-8')
    walk(value)
    return value


def physical(path):
    path = Path(os.path.abspath(path))
    for item in [path, *path.parents]:
        require(not item.is_symlink(), 'symlink is not admitted: ' + str(item))
    return path


def read(path, limit=MAX_FILE):
    path = physical(path)
    require(stat.S_ISREG(path.stat().st_mode), 'not a regular file: ' + str(path))
    with path.open('rb') as stream:
        data = stream.read(limit + 1)
    require(len(data) <= limit, 'file exceeds bound: ' + str(path))
    return data


def load(path):
    return decode(read(path, MAX_JSON))


def fields(value, required, optional=()):
    require(isinstance(value, dict), 'expected an object')
    require(set(required) <= set(value) and not set(value) - set(required) - set(optional),
            'missing or unknown fields; expected ' + ', '.join(required))


def text(value, label):
    require(isinstance(value, str) and value.strip() and len(value) <= 8192,
            label + ' must be nonempty bounded text')
    return value


def ident(value):
    require(isinstance(value, str) and re.fullmatch(r'[a-z][a-z0-9._/-]{0,127}', value)
            and '..' not in value and '//' not in value, 'invalid identifier')
    return value


def relative(value):
    text(value, 'relative path')
    require(not Path(value).is_absolute() and all(p not in ('', '.', '..') for p in value.split('/')),
            'unsafe relative path: ' + value)
    return value


def instant(value):
    require(isinstance(value, str) and re.fullmatch(
        r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,6})?(?:Z|[+-]\d\d:\d\d)', value),
        'timestamp requires RFC3339 seconds and explicit offset')
    return datetime.fromisoformat(value.replace('Z', '+00:00')).astimezone(UTC)


def stamp(value):
    return value.astimezone(UTC).isoformat().replace('+00:00', 'Z')


def now():
    return stamp(datetime.now(UTC))


def strings(value, label, minimum=0, maximum=64):
    require(isinstance(value, list) and minimum <= len(value) <= maximum, 'invalid ' + label)
    for item in value:
        text(item, label)
    require(len(value) == len(set(value)), 'duplicate ' + label)


def command(value, label):
    require(isinstance(value, list) and 1 <= len(value) <= 64, 'invalid ' + label)
    for item in value:
        text(item, label)
    relative(value[0])
    require(value[0].startswith('bin/'), label + ' must select a team bin command')


def process(value):
    fields(value, ['schema', 'id', 'team', 'purpose', 'owner_role', 'inputs', 'stages',
                   'entry', 'verification', 'artifacts', 'configuration', 'agreements', 'escalation'], ['inspection'])
    require(value['schema'] == 'bench.team-process/v1', 'unsupported process schema')
    for key in ('id', 'team', 'owner_role'):
        ident(value[key])
    text(value['purpose'], 'purpose')
    strings(value['inputs'], 'inputs', 1, 16)
    for name in value['inputs']:
        require(re.fullmatch(r'[a-z][a-z0-9-]*', name), 'invalid input name')
    require(isinstance(value['stages'], list) and 1 <= len(value['stages']) <= 32, 'invalid stages')
    seen = set()
    for stage in value['stages']:
        fields(stage, ['id', 'role', 'needs', 'outcome', 'evidence'])
        ident(stage['id']); ident(stage['role'])
        require(stage['id'] not in seen, 'duplicate stage')
        strings(stage['needs'], 'stage needs')
        require(set(stage['needs']) <= seen, 'stages must follow their declared prerequisites')
        seen.add(stage['id'])
        text(stage['outcome'], 'stage outcome')
        strings(stage['evidence'], 'stage evidence', 1)
    entry = value['entry']
    fields(entry, ['argv', 'stdin', 'environment', 'continuation'])
    command(entry['argv'], 'entry argv')
    require(entry['stdin'] is None or entry['stdin'] in value['inputs'], 'invalid stdin input')
    require(entry['continuation'] in ('new-run', 'resume'), 'invalid continuation policy')
    require(isinstance(entry['environment'], dict), 'invalid entry environment')
    command(value['verification'], 'verification argv')
    if 'inspection' in value:
        command(value['inspection'], 'inspection argv')
    strings(value['artifacts'], 'artifacts', 1)
    for item in value['artifacts']:
        relative(item)
    require(isinstance(value['configuration'], dict) and len(value['configuration']) <= 64,
            'invalid configuration')
    for name, kind in value['configuration'].items():
        require(re.fullmatch(r'[A-Z][A-Z0-9_]*', name) and kind in ('text', 'program', 'file'),
                'invalid configuration declaration')
    for name, val in entry['environment'].items():
        require(re.fullmatch(r'[A-Z][A-Z0-9_]*', name), 'invalid entry environment name')
        require(val == '{run}', 'entry environment may only bind the run directory')
    require(isinstance(value['agreements'], list) and 1 <= len(value['agreements']) <= 32,
            'invalid agreements')
    for agreement in value['agreements']:
        fields(agreement, ['requirement', 'enforcement'])
        text(agreement['requirement'], 'working agreement')
        require(agreement['enforcement'] in ('entry', 'verification', 'guidance'), 'invalid enforcement')
    text(value['escalation'], 'escalation')
    allowed = {'{run}'} | {'{input.' + k + '}' for k in value['inputs']}
    for part in entry['argv'][1:] + value['verification'][1:] + value.get('inspection', [])[1:]:
        require(('{' not in part and '}' not in part) or part in allowed,
                'only whole-argument placeholders are supported')
    return value


def bundle(path):
    root = physical(path)
    lock_bytes = read(root / 'team.lock.json', MAX_JSON)
    lock = decode(lock_bytes)
    require(isinstance(lock, dict), 'team lock must be an object')
    require(lock.get('schema') == 'bench.team-lock/v1', 'a pinned team export is required')
    require(re.fullmatch(r'[a-f0-9]{40}', lock['source']['commit']), 'full source commit required')
    require(not lock.get('adaptations'), 'export adaptations require a new source export')
    require(isinstance(lock['files'], dict) and 1 <= len(lock['files']) <= 4096, 'invalid source inventory')
    total = 0
    for name, binding in lock['files'].items():
        relative(name)
        require(name.startswith('expert/'), 'invalid team source path')
        target = root / name
        data = read(target)
        total += len(data)
        require(total <= MAX_FILE, 'team source inventory exceeds 64 MiB')
        require(sha(data) == binding['sha256'], 'changed exported source: ' + name)
        require(binding['mode'] in ('100644', '100755') and
                bool(target.stat().st_mode & 0o111) == (binding['mode'] == '100755'),
                'changed executable mode: ' + name)
    require('expert/process.json' in lock['files'], 'team has no exported process definition')
    definition = process(load(root / 'expert/process.json'))
    require(definition['team'] == lock['team'], 'process/team identity mismatch')
    for argv in [definition['entry']['argv'], definition['verification']] + ([definition['inspection']] if 'inspection' in definition else []):
        require(lock['files'].get('expert/' + argv[0], {}).get('mode') == '100755',
                'process command must be inventoried executable team source')
    roles = {stage['role'] for stage in definition['stages']} | {definition['owner_role']}
    require(roles <= set(lock['members']), 'process references an absent team role')
    return definition, sha(lock_bytes)


def local_instant(day, clock, zone):
    require(re.fullmatch(r'\d\d:\d\d', clock) is not None, 'local time requires HH:MM')
    naive = datetime.combine(day, time.fromisoformat(clock))
    options = {naive.replace(tzinfo=zone, fold=fold).astimezone(UTC) for fold in (0, 1)
               if naive.replace(tzinfo=zone, fold=fold).astimezone(UTC).astimezone(zone).replace(tzinfo=None) == naive}
    require(len(options) == 1, 'ambiguous or nonexistent local time: ' + naive.isoformat())
    return next(iter(options))


def occurrences(definition, calendar):
    fields(calendar, ['schema', 'id', 'timezone', 'start_date', 'end_date', 'weekdays',
                      'excluded_dates', 'start_time', 'due_time', 'due_day_offset', 'missed'])
    require(calendar['schema'] == 'bench.team-calendar/v1', 'unsupported calendar schema')
    ident(calendar['id'])
    zone = ZoneInfo(calendar['timezone'])
    start, end = date.fromisoformat(calendar['start_date']), date.fromisoformat(calendar['end_date'])
    require(0 <= (end - start).days <= 365, 'calendar must span at most 366 dates')
    days = calendar['weekdays']
    require(isinstance(days, list) and 1 <= len(days) <= 7 and
            all(type(d) is int and 0 <= d <= 6 for d in days) and len(set(days)) == len(days), 'invalid weekdays')
    strings(calendar['excluded_dates'], 'excluded dates', maximum=366)
    exclusions = {date.fromisoformat(d) for d in calendar['excluded_dates']}
    require(type(calendar['due_day_offset']) is int and 0 <= calendar['due_day_offset'] <= 30,
            'due_day_offset must be 0..30')
    require(calendar['missed'] in ('all', 'latest', 'skip'), 'invalid missed-occurrence policy')
    result = []
    for offset in range((end - start).days + 1):
        day = start + timedelta(days=offset)
        if day.weekday() not in days or day in exclusions:
            continue
        begin = local_instant(day, calendar['start_time'], zone)
        due = local_instant(day + timedelta(days=calendar['due_day_offset']), calendar['due_time'], zone)
        require(begin <= due, 'due time precedes scheduled start')
        result.append({'id': calendar['id'] + '/' + day.isoformat(), 'not_before': stamp(begin),
                       'due_at': stamp(due), 'timezone': calendar['timezone'],
                       'process_sha256': sha(encode(definition)),
                       'occurrence': {'schedule_id': calendar['id'], 'date': day.isoformat(),
                                      'calendar_sha256': sha(encode(calendar))}})
    return result


def plan(definition, calendar, as_of):
    evaluated = instant(as_of)
    result = [{**item, 'overdue': evaluated > instant(item['due_at'])}
              for item in occurrences(definition, calendar) if instant(item['not_before']) <= evaluated]
    if calendar['missed'] == 'skip':
        result = [item for item in result if not item['overdue']]
    return result[-1:] if calendar['missed'] == 'latest' else result


def commitment(value, definition):
    fields(value, ['schema', 'id', 'namespace', 'owner', 'objective', 'not_before', 'due_at',
                   'timezone', 'inputs', 'environment', 'pass_env', 'milestones'],
           ['occurrence', 'calendar', 'supporting_files', 'supersedes', 'additional_check'])
    require(value['schema'] == 'bench.commitment/v1', 'unsupported commitment schema')
    ident(value['id']); ident(value['namespace'])
    text(value['owner'], 'owner'); text(value['objective'], 'objective')
    ZoneInfo(value['timezone'])
    require(instant(value['not_before']) <= instant(value['due_at']), 'due time precedes start')
    require(isinstance(value['inputs'], dict) and set(value['inputs']) == set(definition['inputs']),
            'inputs must match process input names')
    for path in value['inputs'].values():
        require(isinstance(path, str) and Path(path).is_absolute(), 'input paths must be absolute')
    supporting = value.get('supporting_files', {})
    require(isinstance(supporting, dict) and len(supporting) <= 64, 'invalid supporting files')
    names = set(value['inputs'])
    for name, path in supporting.items():
        relative(name)
        require(name not in names and Path(path).is_absolute(), 'supporting input collision or nonabsolute path')
        names.add(name)
    require(not any(a != b and b.startswith(a + '/') for a in names for b in names), 'input path collision')
    environment = value['environment']
    require(isinstance(environment, dict) and set(environment) <= set(definition['configuration']),
            'unknown configuration name')
    require(not set(environment) & set(definition['entry']['environment']), 'run bindings cannot be overridden')
    for key, val in environment.items():
        text(val, key)
    strings(value['pass_env'], 'pass_env')
    for name in value['pass_env']:
        require(re.fullmatch(r'[A-Z][A-Z0-9_]*', name) and not name.startswith(('TEND_', 'PYTHON', 'LD_', 'DYLD_'))
                and name not in ('PATH', 'HOME', 'ENV', 'BASH_ENV') and
                name not in definition['configuration'] and name not in definition['entry']['environment'], 'unsafe pass_env name')
    require(isinstance(value['milestones'], list) and len(value['milestones']) <= 32, 'invalid milestones')
    seen = set()
    for milestone in value['milestones']:
        fields(milestone, ['id', 'owner', 'due_at', 'expectation'])
        ident(milestone['id'])
        require(milestone['id'] not in seen, 'duplicate milestone')
        seen.add(milestone['id'])
        text(milestone['owner'], 'milestone owner'); text(milestone['expectation'], 'milestone expectation')
        require(instant(value['not_before']) <= instant(milestone['due_at']) <= instant(value['due_at']),
                'milestone must be within commitment dates')
    if 'occurrence' in value:
        require('calendar' in value and Path(value['calendar']).is_absolute(), 'occurrence requires selected calendar file')
        fields(value['occurrence'], ['schedule_id', 'date', 'calendar_sha256'])
        item = value['occurrence']
        ident(item['schedule_id']); date.fromisoformat(item['date'])
        require(value['id'] == item['schedule_id'] + '/' + item['date'], 'occurrence identity mismatch')
        require(re.fullmatch(r'[a-f0-9]{64}', item['calendar_sha256']), 'invalid calendar hash')
    else:
        require('calendar' not in value, 'calendar requires occurrence binding')
    if value.get('supersedes') is not None:
        ident(value['supersedes'])
        require(value['supersedes'] != value['id'], 'cannot supersede self')
    if 'additional_check' in value:
        extra = value['additional_check']
        fields(extra, ['argv', 'files'])
        require(isinstance(extra['argv'], list) and 1 <= len(extra['argv']) <= 32, 'invalid additional check')
        strings(extra['files'], 'additional check files')
        require(Path(extra['argv'][0]).is_absolute(), 'additional check executable must be absolute')
        for arg in extra['argv']:
            text(arg, 'additional check argv')
            require(('{' not in arg and '}' not in arg) or arg in ('{run}', '{commitment}'),
                    'invalid additional check placeholder')
    return value


def program(path):
    # Preserve venv/basename-sensitive invocation, pin resolution separately.
    selected = shutil.which(str(path))
    require(selected is not None, 'missing executable: ' + str(path))
    target = Path(selected).resolve()
    require(os.access(target, os.X_OK), 'not executable: ' + str(target))
    return {'path': os.path.abspath(selected), 'target': str(target), 'sha256': sha(read(target))}


def verify_program(binding):
    require(str(Path(binding['path']).resolve()) == binding['target'] and
            os.access(binding['path'], os.X_OK) and
            sha(read(binding['target'])) == binding['sha256'], 'admitted program changed: ' + binding['path'])


def durable(path, data):
    with open(path, 'xb') as stream:
        stream.write(data); stream.flush(); os.fsync(stream.fileno())


def syncdir(path):
    fd = os.open(path, os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def durable_mkdir(path):
    path = physical(path)
    missing = []
    parent = path
    while not parent.exists():
        missing.append(parent); parent = parent.parent
    path.mkdir(parents=True, exist_ok=True)
    for created in reversed(missing):
        syncdir(created); syncdir(created.parent)


@contextmanager
def lock(root):
    root = physical(root)
    durable_mkdir(root)
    target = root / '.admission.lock'
    physical(target)
    with target.open('a+b') as stream:
        fcntl.flock(stream, fcntl.LOCK_EX)
        yield


def expand(argv, exported, instance, definition):
    values = {'{run}': str(instance / 'run')}
    values.update({'{input.' + name + '}': str(instance / 'inputs' / name) for name in definition['inputs']})
    return [str(exported / 'expert' / argv[0])] + [values.get(arg, arg) for arg in argv[1:]]


def admit(exported, request, root, tend):
    exported, root = physical(exported), physical(root)
    definition, lock_hash = bundle(exported)
    request = commitment(request, definition)
    require(root != exported and exported not in root.parents, 'commitments must stay outside team source')
    own = Path(__file__).resolve().parent
    require(root != own and own not in root.parents, 'commitments must stay outside application source')
    key = 'process-' + sha(encode([request['namespace'], request['id']]))[:48]
    instance = root / key
    selected = {**request['inputs'], **request.get('supporting_files', {})}
    inputs = {name: read(path) for name, path in selected.items()}
    require(sum(map(len, inputs.values())) <= MAX_FILE, 'selected inputs exceed 64 MiB')
    calendar = load(request['calendar']) if 'calendar' in request else None
    if calendar is not None:
        expected_occurrences = plan(definition, calendar, request['not_before'])
        proposed = next((p for p in expected_occurrences if p['id'] == request['id']), None)
        require(proposed and proposed['occurrence'] == request['occurrence'] and
                all(proposed[k] == request[k] for k in ('not_before', 'due_at', 'timezone')),
                'commitment does not match its selected calendar occurrence')
    executables = {name: program(value) for name, value in request['environment'].items()
                   if definition['configuration'][name] == 'program'}
    extra_files = {}
    for name, val in request['environment'].items():
        if definition['configuration'][name] == 'file':
            require(Path(val).is_absolute(), 'file configuration requires an absolute path')
            extra_files[str(physical(val))] = sha(read(val))
    if 'additional_check' in request:
        extra = request['additional_check']
        executables['additional_check'] = program(extra['argv'][0])
        for path in extra['files']:
            require(Path(path).is_absolute(), 'additional check file paths must be absolute')
            extra_files[str(physical(path))] = sha(read(path))
    binding = {'schema': 'bench.process-admission/v1', 'request': request, 'process': definition,
               'export': str(exported), 'lock_sha256': lock_hash, 'job_id': key,
               'queue': str(root / 'tend'), 'instance': str(instance),
               'inputs': {name: sha(data) for name, data in inputs.items()}, 'calendar': calendar,
               'programs': executables, 'check_files': extra_files,
               'tend': program(tend), 'python': program(sys.executable),
               'base_environment': {k: os.environ[k] for k in ('PATH','HOME','LANG','LC_ALL','TMPDIR') if k in os.environ},
               'application': {'path': str(Path(__file__).resolve()), 'sha256': sha(read(__file__))},
               'argv': expand(definition['entry']['argv'], exported, instance, definition),
               'check_argv': expand(definition['verification'], exported, instance, definition)}
    raw = encode(binding)
    with lock(root):
        if instance.exists():
            require(read(instance / 'admission.json', MAX_JSON) == raw, 'commitment ID already binds different bytes')
            verify(instance)
            return instance
        temporary = Path(tempfile.mkdtemp(prefix='.prepare-', dir=root))
        try:
            (temporary / 'inputs').mkdir(); (temporary / 'control').mkdir()
            for name, data in inputs.items():
                (temporary / 'inputs' / name).parent.mkdir(parents=True, exist_ok=True)
                durable(temporary / 'inputs' / name, data)
            durable(temporary / 'admission.json', raw)
            durable(temporary / 'commitment.json', encode(request))
            for folder in sorted((p for p in temporary.rglob('*') if p.is_dir()), reverse=True):
                syncdir(folder)
            syncdir(temporary)
            temporary.rename(instance); syncdir(root)
        finally:
            if temporary.exists():
                shutil.rmtree(temporary)
    return instance


def verify(instance):
    instance = physical(instance)
    value = load(instance / 'admission.json')
    require(value['instance'] == str(instance), 'instance cannot be relocated')
    require(encode(load(instance / 'commitment.json')) == encode(value['request']), 'commitment changed')
    definition, lock_hash = bundle(value['export'])
    require(lock_hash == value['lock_sha256'] and definition == value['process'], 'team/process changed')
    commitment(value['request'], definition)
    for name, digest in value['inputs'].items():
        require(sha(read(instance / 'inputs' / name)) == digest, 'admitted input changed: ' + name)
    require({p.relative_to(instance / 'inputs').as_posix() for p in (instance / 'inputs').rglob('*') if p.is_file()}
            == set(value['inputs']), 'input inventory changed')
    for binding in [value['tend'], value['python'], *value['programs'].values()]:
        verify_program(binding)
    require(sha(read(value['application']['path'])) == value['application']['sha256'], 'application source changed')
    for path, digest in value['check_files'].items():
        require(sha(read(path)) == digest, 'additional check source changed')
    require(value['argv'] == expand(definition['entry']['argv'], Path(value['export']), instance, definition)
            and value['check_argv'] == expand(definition['verification'], Path(value['export']), instance, definition),
            'command binding changed')
    return value


def tend_call(value, args, stdin=b''):
    env = os.environ.copy()
    env['TEND_ROOT'] = value['queue']
    result = capture([value['tend']['path'], *args], input=stdin, env=env, timeout=30)
    require(result.returncode == 0, 'Tend operation failed: ' + result.stderr.decode(errors='replace')[:2000])
    require(len(result.stdout) <= MAX_JSON, 'Tend response exceeds bound')
    return result.stdout


def capture(argv, **options):
    # Spool bounded reads instead of retaining arbitrary subprocess output in RAM.
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        result = subprocess.run(argv, stdout=out, stderr=err, **options)
        out.seek(0); err.seek(0)
        result.stdout, result.stderr = out.read(MAX_JSON + 1), err.read(MAX_JSON + 1)
        require(len(result.stdout) <= MAX_JSON and len(result.stderr) <= MAX_JSON,
                'process response exceeds 2 MiB')
        return result


def submit(instance):
    instance = physical(instance)
    value = verify(instance)
    request = value['request']
    raw = read(instance / 'admission.json', MAX_JSON)
    digest = sha(raw)
    argv = [value['python']['target'], value['application']['path'], '_execute', str(instance), digest]
    result = tend_call(value, ['submit', '-id', value['job_id'], '-key', 'team-process:' + request['namespace'],
                             '-C', str(instance), '-at', request['not_before'], '--', *argv], stdin=b'')
    require(result.decode().strip() == value['job_id'], 'unexpected Tend job identity')
    return {'instance': str(instance), 'job_id': value['job_id'], 'queue': value['queue'],
            'pass_env': request['pass_env']}


def seal_receipt(instance, attempt, receipt):
    require(re.fullmatch(r'[a-zA-Z0-9._-]{1,200}', attempt), 'invalid Tend attempt key')
    path = instance / 'control' / ('receipt-' + attempt + '.json')
    raw = encode(receipt)
    durable(path, raw); syncdir(path.parent)
    # Bind the receipt into Tend's sealed stderr without changing team stdout.
    print('team-process receipt ' + path.name + ' ' + sha(raw), file=sys.stderr, flush=True)


def artifacts(instance, definition):
    return {name: sha(read(instance / 'run' / name)) for name in definition['artifacts']}


def execute(instance, expected):
    instance = physical(instance)
    require(sha(read(instance / 'admission.json', MAX_JSON)) == expected, 'admission bytes changed')
    value = verify(instance)
    require(os.environ.get('TEND_JOB_ID') == value['job_id'], 'execution requires its admitted Tend job')
    attempt = os.environ.get('TEND_ATTEMPT_KEY', '')
    require(attempt, 'execution requires a Tend attempt')
    request, definition = value['request'], value['process']
    require(datetime.now(UTC) >= instant(request['not_before']), 'not-before time has not arrived')
    env = dict(value['base_environment'])
    env.update(request['environment'])
    for name, binding in value['programs'].items():
        if name != 'additional_check':
            env[name] = binding['path']
    for name in request['pass_env']:
        require(name in os.environ, 'missing selected environment name: ' + name)
        env[name] = os.environ[name]
    env.update({name: str(instance / 'run') for name in definition['entry']['environment']})
    env['PYTHONDONTWRITEBYTECODE'] = '1'
    started = instance / 'control/started.json'
    continuing = started.exists()
    if continuing:
        require(definition['entry']['continuation'] == 'resume', 'team requires a fresh run; this attempt cannot resume')
    else:
        durable(started, encode({'at': now(), 'job_id': value['job_id']})); syncdir(started.parent)
    # Page-team accepts the identical brief on resume, including recovery before
    # its manifest was first written. Marker existence is not proof of admission.
    stdin = b'' if definition['entry']['stdin'] is None else read(instance / 'inputs' / definition['entry']['stdin'])
    result = subprocess.run(value['argv'], input=stdin, cwd=instance, env=env)
    code = result.returncode if 0 <= result.returncode < 128 else 125
    receipt = {'schema': 'bench.process-receipt/v1', 'admission_sha256': expected,
               'attempt_key': attempt, 'job_id': value['job_id'], 'entry_exit': result.returncode,
               'verification_exit': None, 'additional_check_exit': None,
               'accepted_at': None, 'artifacts': {}, 'finished_at': now(), 'inspection': None}
    if code not in (0, 125) and 'inspection' in definition:
        inspected = capture(expand(definition['inspection'], Path(value['export']), instance, definition),
                            input=b'', cwd=instance, env=env, timeout=60)
        sys.stderr.buffer.write(inspected.stderr)
        require(inspected.returncode == 0, 'could not inspect unsuccessful team outcome')
        outcome = decode(inspected.stdout)
        require(outcome.get('outcome') in ('unknown','unfinished','needs-input','needs-revision'), 'invalid team inspection')
        receipt['inspection'] = outcome['outcome']
        if outcome['outcome'] == 'unknown': code = 125
    if code == 0:
        checked = subprocess.run(value['check_argv'], stdin=subprocess.DEVNULL,
                                 stdout=sys.stderr, cwd=instance / 'run', env=env)
        receipt['verification_exit'] = checked.returncode
        code = 0 if checked.returncode == 0 else 125 if checked.returncode < 0 else 2 if checked.returncode == 1 else 1
        if code == 0 and 'additional_check' in request:
            argv = request['additional_check']['argv']
            replacements = {'{run}': str(instance / 'run'), '{commitment}': str(instance / 'commitment.json')}
            argv = [value['programs']['additional_check']['path']] + [replacements.get(a, a) for a in argv[1:]]
            checked = subprocess.run(argv, stdin=subprocess.DEVNULL, stdout=sys.stderr,
                                     cwd=instance / 'run', env=env)
            receipt['additional_check_exit'] = checked.returncode
            code = 0 if checked.returncode == 0 else 125 if checked.returncode < 0 else 2 if checked.returncode == 1 else 1
        if code == 0:
            verify(instance)
            receipt['artifacts'] = artifacts(instance, definition)
            receipt['accepted_at'] = now()
    receipt['finished_at'] = now()
    seal_receipt(instance, attempt, receipt)
    return code


def observation(value):
    # Avoid creating a queue during inspection before first submission.
    if not (physical(value['queue']) / 'state/tend.db').exists():
        return None, []
    jobs = [decode(row) for row in tend_call(value, ['list']).splitlines()]
    job = next((j for j in jobs if j['id'] == value['job_id']), None)
    if job is None:
        return None, []
    expected_argv = [value['python']['target'], value['application']['path'], '_execute',
                     value['instance'], sha(read(Path(value['instance']) / 'admission.json', MAX_JSON))]
    require(job['argv'] == expected_argv and job['cwd'] == value['instance'] and
            job['serial_key'] == 'team-process:' + value['request']['namespace'], 'Tend admission differs from commitment')
    events = [decode(row) for row in tend_call(value, ['events', value['job_id']]).splitlines()]
    return job, events


def sealed_streams(value, events, finished):
    payload = finished['payload']
    prepared = next(e['payload'] for e in events if e['kind'] == 'attempt.prepared'
                    and e['payload']['attempt'] == payload['attempt'])
    base = Path(value['queue']) / 'jobs' / value['job_id'] / 'attempts'
    result = {}
    for suffix, prefix in (('out','output'), ('err','stderr')):
        raw = read(base / ('%03d.%s' % (prepared['number'], suffix)))
        require(len(raw) == payload[prefix + '_size'] and sha(raw) == payload[prefix + '_digest'],
                'Tend attempt stream changed: ' + suffix)
        result[suffix] = raw
    return result


def milestone_view(request, supplied, evaluated):
    require(isinstance(supplied, list) and len(supplied) <= 32, 'invalid milestone evidence')
    definitions = {item['id']: item for item in request['milestones']}
    observations = {}
    for item in supplied:
        fields(item, ['id', 'at', 'by', 'artifact', 'sha256'])
        require(item['id'] in definitions and item['id'] not in observations, 'unknown or repeated milestone')
        require(item['by'] == definitions[item['id']]['owner'], 'milestone attribution differs from declared owner')
        require(instant(request['not_before']) <= instant(item['at']) <= evaluated,
                'milestone evidence is outside the observed commitment interval')
        require(sha(read(item['artifact'])) == item['sha256'], 'stale milestone evidence')
        observations[item['id']] = item
    result = []
    for item in request['milestones']:
        observation = observations.get(item['id'])
        timing = instant(observation['at']) if observation else evaluated
        result.append({**item, 'state': 'reported-met' if observation else 'unconfirmed',
                       'timeliness': ('late' if observation else 'overdue') if timing > instant(item['due_at']) else 'on-time' if observation else 'pending',
                       'evidence': observation})
    return result


def status_checked(instance, as_of, milestones=None):
    instance = physical(instance)
    value = verify(instance)
    evaluated = instant(as_of)
    job, events = observation(value)
    execution = job['status'] if job else 'unsubmitted'
    receipts = [load(path) for path in sorted((instance / 'control').glob('receipt-*.json'))]
    digest = sha(read(instance / 'admission.json', MAX_JSON))
    for receipt in receipts:
        require(receipt['admission_sha256'] == digest and receipt['job_id'] == value['job_id'], 'stale receipt binding')
    starts = [e for e in events if e['kind'] == 'attempt.started']
    latest = starts[-1]['payload'] if starts else None
    receipt = next((r for r in receipts if latest and r['attempt_key'] == latest['effect_key']), None)
    finished = [e for e in events if e['kind'] == 'attempt.finished' and latest and
                e['payload']['attempt'] == latest['attempt']]
    if finished:
        streams = sealed_streams(value, events, finished[-1])
        if receipt:
            name = 'receipt-' + receipt['attempt_key'] + '.json'
            marker = ('team-process receipt ' + name + ' ' + sha(read(instance/'control'/name)) + '\n').encode()
            require(marker in streams['err'], 'receipt differs from Tend sealed evidence')
    acceptance = 'unconfirmed'
    if receipt and execution == 'done':
        if (finished and finished[-1]['payload']['status'] == 'done' and finished[-1]['payload']['exit'] == 0
                and receipt['entry_exit'] == 0 and receipt['verification_exit'] == 0
                and receipt['additional_check_exit'] in (None, 0) and receipt['accepted_at']):
            require(instant(receipt['accepted_at']) <= evaluated, 'as-of precedes current acceptance; historical reconstruction is unsupported')
            try:
                current = artifacts(instance, value['process'])
                acceptance = 'accepted' if current == receipt['artifacts'] else 'stale'
            except (ValueError, OSError):
                acceptance = 'stale'
    elif receipt and execution in ('failed', 'waiting', 'cancelled'):
        if receipt['entry_exit'] == 75:
            acceptance = 'needs-input'
        elif receipt['inspection'] in ('needs-revision','needs-input'):
            acceptance = receipt['inspection']
        elif receipt['verification_exit'] == 1 or receipt['additional_check_exit'] == 1:
            acceptance = 'needs-revision'
        elif receipt['entry_exit'] == 0:
            acceptance = 'broken-verification'
        else:
            acceptance = 'unfinished'
    timing = instant(receipt['accepted_at']) if acceptance == 'accepted' else evaluated
    due = instant(value['request']['due_at'])
    timeliness = ('late' if acceptance == 'accepted' else 'overdue') if timing > due else 'on-time' if acceptance == 'accepted' else 'pending'
    milestones = milestone_view(value['request'], milestones or [], evaluated)
    reasons = []
    if execution == 'unknown': reasons.append('execution-outcome-unknown')
    if acceptance in ('needs-input', 'needs-revision', 'broken-verification', 'stale'): reasons.append(acceptance)
    if timeliness in ('overdue', 'late'): reasons.append(timeliness)
    reasons.extend('milestone:' + m['id'] for m in milestones if m['timeliness'] == 'overdue')
    return {'schema': 'bench.commitment-status/v1', 'id': value['request']['id'],
            'namespace': value['request']['namespace'], 'owner': value['request']['owner'],
            'objective': value['request']['objective'], 'due_at': value['request']['due_at'],
            'as_of': as_of, 'execution': execution, 'acceptance': acceptance, 'timeliness': timeliness,
            'accepted_at': receipt['accepted_at'] if acceptance == 'accepted' else None,
            'milestones': milestones, 'escalation': {'owner': value['request']['owner'],
                'reasons': reasons, 'guidance': value['process']['escalation'], 'sent': False},
            'job_id': value['job_id'], 'queue': value['queue'], 'instance': str(instance),
            'entry_exit': receipt['entry_exit'] if receipt else None,
            'continuation': value['process']['entry']['continuation'],
            'evidence': {'receipt': receipt, 'tend_event_count': len(events)}}


def status(instance, as_of, milestones=None):
    instant(as_of)
    require(physical(instance).is_dir(), 'instance directory does not exist')
    try:
        return status_checked(instance, as_of, milestones)
    except (ValueError, OSError, KeyError, TypeError, StopIteration, subprocess.SubprocessError) as error:
        # A broken binding remains visible; it cannot establish execution facts
        # or acceptance. Do not execute changed programs to repair the view.
        try:
            request = load(Path(instance)/'commitment.json')
        except (ValueError, OSError):
            request = {}
        return {'schema':'bench.commitment-status/v1', 'instance':str(instance),
                'id':request.get('id'), 'owner':request.get('owner'),
                'objective':request.get('objective'), 'due_at':request.get('due_at'),
                'as_of':as_of, 'execution':'unverified', 'acceptance':'unverified',
                'timeliness':'unverified', 'error':str(error),
                'escalation':{'owner':request.get('owner'), 'reasons':['evidence-invalid'], 'sent':False}}


def tracking_key(namespace, identifier):
    return sha(encode([ident(namespace), ident(identifier)]))[:48]


def runtime_root(root, exported=None):
    root = physical(root)
    for source in [Path(__file__).resolve().parent] + ([physical(exported)] if exported else []):
        require(root != source and source not in root.parents, 'tracking data must stay outside source')
    return root


def history(folder, schema):
    folder = physical(folder)
    if not folder.exists():
        return []
    paths = sorted(folder.glob('*.json'))
    require(len(paths) <= 100, 'history exceeds 100 revisions')
    result, previous = [], None
    for sequence, path in enumerate(paths, 1):
        require(path.name == '%06d.json' % sequence, 'history sequence is incomplete')
        raw = read(path, MAX_JSON)
        item = decode(raw)
        fields(item, ['schema', 'sequence', 'previous', 'recorded_at', 'payload'])
        require(item['schema'] == schema and item['sequence'] == sequence and item['previous'] == previous,
                'history chain is invalid')
        instant(item['recorded_at'])
        previous = sha(raw)
        result.append({**item, 'revision': previous})
    return result


def replace_bytes(path, raw):
    path = physical(path)
    fd, temporary = tempfile.mkstemp(prefix='.record-', dir=path.parent)
    try:
        with os.fdopen(fd, 'wb') as stream:
            stream.write(raw); stream.flush(); os.fsync(stream.fileno())
        os.replace(temporary, path); syncdir(path.parent)
    finally:
        if os.path.exists(temporary): os.unlink(temporary)


def replace_record(path, value):
    raw = encode(value)
    require(len(raw) <= MAX_JSON, 'record exceeds 2 MiB')
    replace_bytes(path, raw)


def append_history(folder, schema, payload, previous):
    old = history(folder, schema)
    if old and old[-1]['payload'] == payload:
        return old[-1]
    require(previous == (old[-1]['revision'] if old else None), 'history changed; select its current revision')
    require(len(old) < 100, 'history exceeds 100 revisions')
    durable_mkdir(folder)
    record = {'schema': schema, 'sequence': len(old) + 1, 'previous': previous,
              'recorded_at': now(), 'payload': payload}
    replace_record(folder / ('%06d.json' % record['sequence']), record)
    syncdir(folder.parent)
    return {**record, 'revision': sha(encode(record))}


def registration_payload(value):
    fields(value, ['namespace', 'owner', 'effective_from', 'reason', 'check_every_seconds',
                   'calendar', 'process', 'source'])
    ident(value['namespace']); text(value['owner'], 'calendar owner'); text(value['reason'], 'change reason')
    date.fromisoformat(value['effective_from'])
    require(type(value['check_every_seconds']) is int and 30 <= value['check_every_seconds'] <= 604800,
            'check_every_seconds must be 30..604800')
    process(value['process']); occurrences(value['process'], value['calendar'])
    fields(value['source'], ['team', 'commit', 'lock_sha256'])
    require(value['source']['team'] == value['process']['team'] and
            re.fullmatch(r'[a-f0-9]{40}', value['source']['commit']) and
            re.fullmatch(r'[a-f0-9]{64}', value['source']['lock_sha256']), 'invalid calendar source binding')
    return value


def register_calendar(exported, request, root):
    fields(request, ['schema', 'namespace', 'owner', 'calendar', 'effective_from', 'reason',
                     'check_every_seconds'], ['previous'])
    require(request['schema'] == 'bench.calendar-registration/v1', 'unsupported registration schema')
    root = runtime_root(root, exported)
    definition, lock_hash = bundle(exported)
    payload = registration_payload({k: request[k] for k in
        ('namespace', 'owner', 'effective_from', 'reason', 'check_every_seconds')} | {
        'calendar': load(request['calendar']), 'process': definition,
        'source': {'team': definition['team'], 'commit': load(Path(exported)/'team.lock.json')['source']['commit'],
                   'lock_sha256': lock_hash}})
    cal = payload['calendar']
    folder = root / 'calendars' / tracking_key(payload['namespace'], cal['id'])
    with lock(root):
        old = history(folder, 'bench.calendar-revision/v1')
        if old and old[-1]['payload'] == payload:
            return old[-1]
        if old:
            prior = old[-1]['payload']
            require(prior['namespace'] == payload['namespace'] and prior['calendar']['id'] == cal['id'],
                    'calendar identity changed')
            require(prior['calendar']['timezone'] == cal['timezone'] and prior['process']['team'] == definition['team'],
                    'timezone or team change requires a new calendar identity')
            today = instant(now()).astimezone(ZoneInfo(cal['timezone'])).date().isoformat()
            require(payload['effective_from'] > today and payload['effective_from'] > prior['effective_from'],
                    'revision must take effect after today and after the preceding revision')
            require(cal['start_date'] <= payload['effective_from'] <= cal['end_date'],
                    'revision effective date must be within its calendar')
        else:
            require(payload['effective_from'] == cal['start_date'], 'first revision must start at calendar start_date')
            require(len(list((root/'calendars').glob('*'))) < 100, 'registry exceeds 100 calendars')
        return append_history(folder, 'bench.calendar-revision/v1', payload, request.get('previous'))


def registered(root):
    result, errors = [], []
    folders = sorted((physical(root)/'calendars').glob('*'))
    require(len(folders) <= 100, 'registry exceeds 100 calendars')
    for folder in folders:
        try:
            revisions = history(folder, 'bench.calendar-revision/v1')
            require(revisions, 'empty calendar history')
            last = None
            for revision in revisions:
                value = registration_payload(revision['payload'])
                require(folder.name == tracking_key(value['namespace'], value['calendar']['id']), 'calendar identity differs from directory')
                if last:
                    require(value['effective_from'] > last['effective_from'] and
                            value['calendar']['timezone'] == last['calendar']['timezone'] and
                            value['process']['team'] == last['process']['team'], 'invalid calendar revision boundary')
                last = value
            result.append(revisions)
        except (ValueError, OSError, KeyError, TypeError) as error:
            errors.append({'id': folder.name, 'reason': 'calendar-unverified', 'detail': str(error)})
    return result, errors


def expected_work(registrations):
    expected = {}
    for revisions in registrations:
        for index, revision in enumerate(revisions):
            value = revision['payload']
            end = revisions[index+1]['payload']['effective_from'] if index+1 < len(revisions) else '9999-12-31'
            for item in occurrences(value['process'], value['calendar']):
                if value['effective_from'] <= item['occurrence']['date'] < end:
                    key = (value['namespace'], item['id'])
                    require(key not in expected, 'duplicate expected obligation')
                    expected[key] = {**item, 'namespace': value['namespace'], 'owner': value['owner'],
                        'objective': value['process']['purpose'], 'calendar_id': value['calendar']['id'],
                        'calendar_revision': revision['revision'], 'calendar_sequence': revision['sequence'],
                        'lock_sha256': value['source']['lock_sha256']}
    require(len(expected) <= 20000, 'calendar view exceeds 20000 obligations')
    return expected


def disposition_payload(value):
    fields(value, ['schema', 'namespace', 'calendar_id', 'id', 'calendar_revision', 'kind', 'by', 'reason'])
    require(value['schema'] == 'bench.calendar-disposition/v1', 'unsupported disposition schema')
    for key in ('namespace', 'calendar_id', 'id'): ident(value[key])
    require(value['kind'] in ('skipped', 'cancelled', 'reopened'), 'invalid disposition')
    text(value['by'], 'attribution'); text(value['reason'], 'disposition reason')
    require(re.fullmatch(r'[a-f0-9]{64}', value['calendar_revision']), 'invalid calendar revision')
    return value


def record_disposition(root, request):
    request = dict(request)
    previous = request.pop('previous', None)
    value = disposition_payload(request)
    root = runtime_root(root)
    with lock(root):
        registrations, errors = registered(root)
        require(not errors, 'repair calendar history before recording a disposition')
        expected = expected_work(registrations).get((value['namespace'], value['id']))
        require(expected and expected['calendar_id'] == value['calendar_id'] and
                expected['calendar_revision'] == value['calendar_revision'], 'select a current expected occurrence and revision')
        folder = root / 'dispositions' / tracking_key(value['namespace'], value['id'])
        old = history(folder, 'bench.disposition-revision/v1')
        if old and old[-1]['payload'] == value:
            return old[-1]
        if value['kind'] == 'reopened':
            require(old and old[-1]['payload']['kind'] != 'reopened', 'only a disposition can be reopened')
        if value['kind'] == 'skipped':
            require(not (root/('process-'+tracking_key(value['namespace'], value['id']))).exists(),
                    'admitted work cannot be skipped; inspect it and record a cancellation if appropriate')
        return append_history(folder, 'bench.disposition-revision/v1', value, previous)


def activity_payload(value):
    fields(value, ['schema', 'namespace', 'id', 'parent', 'title', 'description', 'assignee',
                   'due_at', 'timezone', 'state', 'by', 'reason', 'evidence'])
    require(value['schema'] == 'bench.human-activity/v1', 'unsupported human activity schema')
    for key in ('namespace','id','parent'): ident(value[key])
    for key in ('title','description','assignee','by','reason'): text(value[key], key)
    instant(value['due_at']); ZoneInfo(value['timezone'])
    require(value['state'] in ('planned','ready','in-progress','needs-attention','done','cancelled'),
            'invalid human activity state')
    require(isinstance(value['evidence'], list) and len(value['evidence']) <= 16, 'invalid activity evidence')
    names = set()
    for item in value['evidence']:
        fields(item, ['path','sha256'])
        text(item['path'], 'evidence path')
        require(Path(item['path']).is_absolute() and item['path'] not in names and
                re.fullmatch(r'[a-f0-9]{64}', item['sha256']), 'invalid or repeated activity evidence')
        names.add(item['path'])
    require(value['state'] != 'done' or value['evidence'], 'human completion requires selected evidence')
    return value


def record_activity(root, request):
    request = dict(request)
    previous = request.pop('previous', None)
    value = activity_payload(request)
    root = runtime_root(root)
    folder = root/'activities'/tracking_key(value['namespace'], value['id'])
    with lock(root):
        report = calendar_view(root, now())
        parents = [row for row in report['obligations'] if row.get('kind') == 'commitment' and
                   (row['namespace'],row['id']) == (value['namespace'],value['parent'])]
        require(len(parents) == 1 and parents[0]['execution'] != 'unverified', 'select a verified existing parent commitment or occurrence')
        old = history(folder, 'bench.activity-revision/v1')
        if old:
            require(activity_evidence(folder, old), 'retained activity history evidence changed')
            require(old[-1]['payload']['parent'] == value['parent'], 'activity parent cannot change')
            if old[-1]['payload'] == value:
                for item in value['evidence']:
                    require(sha(read(folder/'evidence'/item['sha256'])) == item['sha256'], 'retained activity evidence changed')
                return old[-1]
            require(previous == old[-1]['revision'], 'history changed; select its current revision')
        else:
            require(previous is None, 'new activity cannot select a previous revision')
            require(len(list((root/'activities').glob('*'))) < 1000, 'activity count exceeds 1000')
        # Read selected evidence before creating any record. A partial write may
        # leave unreferenced blobs, but cannot assert completion without a revision.
        selected, total = {}, 0
        for item in value['evidence']:
            raw = read(item['path']); total += len(raw)
            require(total <= MAX_FILE and sha(raw) == item['sha256'], 'activity evidence changed or exceeds 64 MiB')
            selected[item['sha256']] = raw
        durable_mkdir(folder/'evidence')
        for digest, raw in selected.items():
            target = folder/'evidence'/digest
            if target.exists():
                require(sha(read(target)) == digest, 'retained activity evidence changed')
            else:
                replace_bytes(target, raw)
        syncdir(folder/'evidence')
        return append_history(folder, 'bench.activity-revision/v1', value, previous)


def activity_evidence(folder, revisions):
    digests = {item['sha256'] for r in revisions for item in activity_payload(r['payload'])['evidence']}
    valid = True
    for digest in digests:
        try:
            require(sha(read(folder/'evidence'/digest)) == digest, 'activity evidence changed')
        except (ValueError,OSError): valid = False
    return valid


def human_job(binding):
    verify_program(binding['tend'])
    require((physical(binding['queue'])/'state/tend.db').is_file(), 'linked Tend queue is missing')
    jobs = [decode(line) for line in tend_call(binding, ['list']).splitlines()]
    matches = [j for j in jobs if j['id'] == binding['job_id']]
    require(len(matches) == 1, 'linked Tend job is missing')
    job = matches[0]
    identity = {k:job.get(k) for k in ('id','argv','cwd','serial_key','created_us','check_argv')}
    require('job_identity' not in binding or binding['job_identity'] == identity, 'linked Tend job identity changed')
    events = [decode(line) for line in tend_call(binding, ['events', binding['job_id']]).splitlines()]
    if 'wait' in binding:
        require(binding['wait'] in events, 'linked Tend wait evidence changed')
    return job, identity, events


def human_pending(binding):
    verify_program(binding['may'])
    require(os.getuid() == binding['operator_uid'], 'May link belongs to a different OS operator')
    result = capture([binding['may']['path'], 'pending'], input=b'',
                     timeout=30)
    require(result.returncode == 0, 'May pending inspection failed')
    pending = [decode(line) for line in result.stdout.splitlines()]
    matches = [r for r in pending if r.get('digest') == binding['digest']]
    require(len(matches) <= 1, 'duplicate May request')
    for record in matches:
        require(record.get('version') == 1 and record.get('job') == binding['request']['may_job'] and
                record.get('action') == binding['action'], 'May request differs from linked action')
    return bool(matches)


def link_human(root, request):
    fields(request, ['schema','namespace','id','activity_revision','kind','tend','queue','job','signal','by','reason'],
           ['may','may_job','action','response_path'])
    require(request['schema'] == 'bench.human-link/v1' and request['kind'] in ('approval','input'), 'invalid human link')
    for key in ('namespace','id','job'): ident(request[key])
    for key in ('by','reason'): text(request[key],key)
    require(isinstance(request['signal'], str) and re.fullmatch(r'[a-z0-9._-]{1,64}',request['signal']), 'invalid wait signal')
    options = {'may','may_job','action'} if request['kind'] == 'approval' else {'response_path'}
    require(set(request) & {'may','may_job','action','response_path'} == options, 'select only the fields for this link kind')
    root = runtime_root(root)
    key = tracking_key(request['namespace'],request['id'])
    folder = root/'human-links'/key
    with lock(root):
        if (folder/'binding.json').exists():
            binding = load(folder/'binding.json')
            require(binding['request'] == request, 'human link is immutable; create a new activity for another request')
            return binding
        revisions = history(root/'activities'/key, 'bench.activity-revision/v1')
        require(revisions and revisions[-1]['revision'] == request['activity_revision'], 'select the current activity revision')
        require(revisions[-1]['payload']['state'] not in ('done','cancelled'), 'select an open human activity')
        require(Path(request['queue']).is_absolute(), 'queue must be absolute')
        binding = {'schema':'bench.human-binding/v1','request':request,'recorded_at':now(),
                   'queue':str(physical(request['queue'])), 'job_id':request['job'], 'tend':program(request['tend'])}
        job, identity, events = human_job(binding)
        waits = [e for e in events if e['kind'] == 'attempt.finished' and
                 e['payload'].get('status') == 'waiting' and e['payload'].get('wait_key') == request['signal']]
        require(job['status'] == 'waiting' and job.get('wait_kind') == 'signal' and
                job.get('wait_key') == request['signal'] and len(waits) == 1,
                'select a current signal wait with a name unique to this human request')
        binding.update(job_identity=identity, wait=waits[0])
        for other in (root/'human-links').glob('*/binding.json'):
            old = load(other)
            require((old['queue'],old['job_id'],old['wait']['seq']) !=
                    (binding['queue'],binding['job_id'],binding['wait']['seq']), 'wait already linked to another activity')
        if request['kind'] == 'approval':
            action = read(request['action'], 16384).decode('utf-8')
            text(request['may_job'], 'May job')
            require(action and '\0' not in action and '\0' not in request['may_job'] and
                    len(request['may_job'].encode()) <= 1024, 'invalid May action or job')
            binding.update(may=program(request['may']), operator_uid=os.getuid(), action=action,
                           digest=sha(b'may-v1\0'+request['may_job'].encode()+b'\0'+action.encode()))
            require(human_pending(binding), 'exact May request is not pending')
        else:
            destination = physical(request['response_path'])
            require(Path(request['response_path']).is_absolute() and physical(job['cwd']) in destination.parents,
                    'response path must be absolute and inside the linked job working directory')
            require(not destination.exists(), 'response path already exists; select a new mailbox for this request')
        durable_mkdir(folder)
        replace_record(folder/'binding.json',binding)
        return binding


def human_link_view(root, namespace, identifier):
    folder = physical(root)/'human-links'/tracking_key(namespace,identifier)
    if not folder.exists(): return None
    binding = load(folder/'binding.json')
    require(binding['schema'] == 'bench.human-binding/v1' and
            (binding['request']['namespace'],binding['request']['id']) == (namespace,identifier), 'human link identity changed')
    job, _, events = human_job(binding)
    signal_id = 'human-'+sha(encode(binding))[:58]
    responses = [e for e in events if e['kind'] == 'signal.received' and e['payload'].get('id') == signal_id]
    response = load(folder/'response.json') if (folder/'response.json').exists() else None
    if response:
        fields(response, ['schema','kind','by','reason','binding_sha256','input_sha256','input_source','recorded_at'])
        require(response['schema'] == 'bench.human-response-record/v1' and
                response['binding_sha256'] == sha(encode(binding)) and response['kind'] == binding['request']['kind'],
                'human response binding changed')
        instant(response['recorded_at'])
    if response and response['kind'] == 'input':
        require(sha(read(folder/'input',MAX_JSON)) == response['input_sha256'], 'retained human input changed')
    if responses:
        require(response is not None and responses[0]['payload']['payload_digest'] == sha(encode(response)),
                'human response differs from Tend signal evidence')
    pending = human_pending(binding) if binding['request']['kind'] == 'approval' else None
    finished = [e for e in events if e['kind'] == 'attempt.finished']
    same_wait = job['status'] == 'waiting' and finished and finished[-1] == binding['wait']
    state = 'awaiting-approval' if pending else 'awaiting-input' if binding['request']['kind'] == 'input' else 'decision-not-pending'
    woke = responses[0]['payload'].get('woke',False) if responses else False
    if responses: state = ('wake-recorded' if woke else 'signal-recorded') if job['status'] == 'ready' else 'resumed'
    if job['status'] in ('unknown','failed','cancelled','done'): state = 'job-'+job['status']
    elif job['status'] == 'waiting' and not same_wait: state = 'waiting-again'
    return {'kind':binding['request']['kind'],'state':state,'job_id':binding['job_id'],
            'job_status':job['status'],'signal':binding['request']['signal'],
            'digest':binding.get('digest'),'pending':pending,'response':response,
            'signal_recorded':bool(responses),'signal_woke':woke,'observed_at':now(),
            'notice':'Wakeup is not approval. The resumed controller must recheck May or validate supplied input.'}


def respond_human(root, request):
    fields(request, ['schema','namespace','id','by','reason'], ['input'])
    require(request['schema'] == 'bench.human-response/v1', 'invalid human response')
    for key in ('namespace','id'): ident(request[key])
    for key in ('by','reason'): text(request[key],key)
    root = runtime_root(root)
    folder = root/'human-links'/tracking_key(request['namespace'],request['id'])
    require((folder/'binding.json').is_file(), 'human activity has no linked wait')
    with lock(folder):
        binding = load(folder/'binding.json')
        kind = binding['request']['kind']
        require(('input' in request) == (kind == 'input'), 'input responses require a selected file; approval responses use May')
        prepared = load(folder/'response.json') if (folder/'response.json').exists() else None
        raw = None
        if kind == 'input':
            require(Path(request['input']).is_absolute(), 'input file must be absolute')
            selected = physical(request['input'])
            if not selected.exists() and prepared and prepared.get('input_source') == str(selected):
                raw = read(folder/'input',MAX_JSON)
            else: raw = read(selected,MAX_JSON)
        response = {'schema':'bench.human-response-record/v1','kind':kind,'by':request['by'],'reason':request['reason'],
                    'binding_sha256':sha(encode(binding)), 'input_sha256':sha(raw) if raw is not None else None,
                    'input_source':str(selected) if raw is not None else None,
                    'recorded_at':prepared['recorded_at'] if prepared else now()}
        if prepared:
            require(prepared == response, 'response already prepared with different bytes or attribution')
        view = human_link_view(root,request['namespace'],request['id'])
        if view['signal_recorded']: return view
        job, _, events = human_job(binding)
        finished = [e for e in events if e['kind'] == 'attempt.finished']
        require(job['status'] == 'waiting' and job.get('wait_kind') == 'signal' and job.get('wait_key') == binding['request']['signal'] and
                finished and finished[-1] == binding['wait'], 'linked wait changed; inspect the job before responding')
        if raw is not None:
            if (folder/'input').exists(): require(read(folder/'input',MAX_JSON) == raw, 'retained input changed')
            else: replace_bytes(folder/'input',raw)
        if not (folder/'response.json').exists(): replace_record(folder/'response.json',response)
        if kind == 'approval':
            if human_pending(binding):
                # May reads /dev/tty. No answer, grant, or consuming request is supplied here.
                decided = subprocess.run([binding['may']['path'],'decide',binding['digest']],
                                         stdin=subprocess.DEVNULL, stdout=sys.stderr)
                require(decided.returncode in (0,3), 'May decision failed; inspect May before repeating')
            require(not human_pending(binding), 'May request remains pending; a human terminal decision is required')
        else:
            destination = physical(binding['request']['response_path'])
            durable_mkdir(destination.parent)
            if destination.exists(): require(read(destination,MAX_JSON) == raw, 'human input mailbox has conflicting bytes')
            else:
                # Publish complete bytes without overwriting a concurrent response.
                with tempfile.NamedTemporaryFile(dir=destination.parent,delete=False) as output:
                    temporary = Path(output.name)
                    output.write(raw); output.flush(); os.fsync(output.fileno())
                try: os.link(temporary,destination); syncdir(destination.parent)
                finally: temporary.unlink()
        # Never retry/resolve/work here. Tend's stable signal ID makes recovery
        # after a lost response idempotent; the controller owns validation.
        job, _, events = human_job(binding)
        finished = [e for e in events if e['kind'] == 'attempt.finished']
        require(job['status'] == 'waiting' and job.get('wait_kind') == 'signal' and job.get('wait_key') == binding['request']['signal'] and
                finished and finished[-1] == binding['wait'], 'linked wait changed before wakeup; inspect the job')
        tend_call(binding, ['signal','-id','human-'+sha(encode(binding))[:58],binding['job_id'],binding['request']['signal']],
                  stdin=encode(response))
        return human_link_view(root,request['namespace'],request['id'])


def activity_rows(root, parents, as_of):
    parent_map = {(r['namespace'],r['id']):r for r in parents}
    rows, errors = [], []
    folders = sorted((physical(root)/'activities').glob('*'))
    require(len(folders) <= 1000, 'activity count exceeds 1000')
    for folder in folders:
        try:
            revisions = history(folder, 'bench.activity-revision/v1')
            # A write interrupted before its first revision has not created work.
            if not revisions:
                errors.append({'id':folder.name,'reason':'activity-unverified','detail':'Activity write has no committed revision.'})
                continue
            parent_id, prior_state, completed_at = None, None, None
            for revision in revisions:
                value = activity_payload(revision['payload'])
                require(folder.name == tracking_key(value['namespace'],value['id']), 'activity identity differs from directory')
                require(parent_id is None or parent_id == value['parent'], 'activity parent changed')
                parent_id = value['parent']
                if value['state'] == 'done' and prior_state != 'done': completed_at = revision['recorded_at']
                if value['state'] != 'done': completed_at = None
                prior_state = value['state']
            latest = revisions[-1]; value = latest['payload']
            reasons = []
            evidence_valid = activity_evidence(folder, revisions)
            if not evidence_valid: reasons.append('activity-evidence-invalid')
            parent = parent_map.get((value['namespace'],value['parent']))
            if not parent: reasons.append('activity-parent-missing')
            elif parent['execution'] == 'unverified': reasons.append('activity-parent-unverified')
            elif parent['state'] in ('skipped','cancelled') and value['state'] not in ('done','cancelled'):
                reasons.append('activity-parent-closed')
            done = value['state'] == 'done' and evidence_valid
            completed_at = completed_at if done else None
            timing = instant(completed_at) if done else instant(as_of)
            timeliness = ('late' if done else 'overdue') if timing > instant(value['due_at']) else 'on-time' if done else 'pending'
            if done and instant(completed_at) > instant(as_of):
                reasons.append('activity-completion-after-as-of')
            if value['state'] != 'cancelled' and timeliness in ('overdue','late'): reasons.append(timeliness)
            if value['state'] == 'needs-attention': reasons.append('human-needs-attention')
            try:
                coordination = human_link_view(root,value['namespace'],value['id'])
                if coordination and coordination['state'] in ('awaiting-approval','awaiting-input','decision-not-pending',
                                                               'waiting-again','job-unknown','job-failed','job-cancelled'):
                    reasons.append('human-'+coordination['state'])
            except (ValueError,OSError,KeyError,TypeError,subprocess.SubprocessError) as error:
                coordination = {'state':'unverified','error':str(error)}
                reasons.append('human-link-unverified')
            rows.append({'kind':'human-activity','namespace':value['namespace'],'id':value['id'],
                'parent':value['parent'],'owner':value['assignee'],'objective':value['title'],
                'description':value['description'],'not_before':revisions[0]['recorded_at'],
                'due_at':value['due_at'],'timezone':value['timezone'], 'state':value['state'],
                'execution':'unverified' if not evidence_valid or coordination and coordination['state'] == 'unverified' else 'human-reported',
                'acceptance':'reported-done' if done else 'unconfirmed','timeliness':timeliness,
                'completed_at':completed_at,'milestones':[], 'attention':sorted(set(reasons)),
                'disposition':None,'disposition_history':[], 'revision':latest['revision'],
                'activity_history':revisions, 'evidence':value['evidence'], 'coordination':coordination, 'calendar_id':None,
                'calendar_revision':None})
        except (ValueError,OSError,KeyError,TypeError) as error:
            errors.append({'id':folder.name,'reason':'activity-unverified','detail':str(error)})
    return rows, errors


def tracking_digest(root):
    root = physical(root)
    paths = []
    for part in ('calendars', 'dispositions', 'activities', 'human-links'):
        paths.extend((root/part).glob('*/*.json'))
    paths.extend(root.glob('process-*/admission.json'))
    paths.extend(root.glob('process-*/commitment.json'))
    require(len(paths) <= 22000, 'tracking inventory exceeds bound')
    total, items = 0, []
    for path in sorted(paths):
        raw = read(path, MAX_JSON); total += len(raw)
        require(total <= MAX_FILE, 'tracking inventory exceeds 64 MiB')
        items.append([path.relative_to(root).as_posix(), sha(raw)])
    return sha(encode(items))


def monitor_view(root, digest, interval, observed):
    path = physical(root)/'last-reconciliation.json'
    result = {'state': 'never-reconciled', 'last_success_at': None, 'next_check_due': None,
              'check_every_seconds': interval}
    if path.exists():
        try:
            value = load(path)
            fields(value, ['schema', 'completed_at', 'input_sha256', 'obligations', 'attention'])
            require(value['schema'] == 'bench.reconciliation/v1', 'invalid reconciliation record')
            completed = instant(value['completed_at'])
            require(completed <= instant(observed), 'reconciliation timestamp is in the future')
            due = completed + timedelta(seconds=interval)
            result.update(last_success_at=value['completed_at'], next_check_due=stamp(due),
                          state='changed' if digest != value['input_sha256'] else
                          'stale' if instant(observed) > due else 'current')
        except (ValueError, OSError, KeyError, TypeError) as error:
            result.update(state='unverified', error=str(error))
    return result


def calendar_view(root, as_of, milestones=None):
    root = physical(root)
    require(root.is_dir(), 'commitment root does not exist')
    evaluated, observed = instant(as_of), now()
    try:
        starting_digest = tracking_digest(root)
    except (ValueError, OSError):
        starting_digest = None
    registrations, errors = registered(root)
    expected = expected_work(registrations)
    supplied = milestones or {}
    require(isinstance(supplied, dict), 'milestone evidence must map instance directory names to observations')
    instances = sorted(root.glob('process-*'))
    require(len(instances) <= 1000, 'calendar view exceeds 1000 commitments')
    require(set(supplied) <= {p.name for p in instances}, 'milestone evidence names an absent commitment')
    admitted, broken = {}, {}
    expected_instances = {'process-'+tracking_key(*key): key for key in expected}
    for instance in instances:
        key = expected_instances.get(instance.name)
        try:
            request = load(instance/'commitment.json')
            key = (ident(request['namespace']), ident(request['id']))
            require(instance.name == 'process-'+tracking_key(*key), 'commitment identity differs from directory')
            binding = load(instance/'admission.json')
            commitment(request, process(binding['process']))
            require(key not in admitted, 'duplicate commitment identity')
            admitted[key] = (request, binding, status(instance, as_of, supplied.get(instance.name)))
        except (ValueError, OSError, KeyError, TypeError) as error:
            if key: broken[key] = str(error)
            errors.append({'id': instance.name, 'reason': 'commitment-unverified', 'detail': str(error)})
    dispositions = {}
    folders = sorted((root/'dispositions').glob('*'))
    require(len(folders) <= 20000, 'disposition count exceeds bound')
    for folder in folders:
        try:
            revisions = history(folder, 'bench.disposition-revision/v1')
            require(revisions, 'empty disposition history')
            for revision in revisions:
                value = disposition_payload(revision['payload'])
                require(folder.name == tracking_key(value['namespace'], value['id']), 'disposition identity differs from directory')
            dispositions[(value['namespace'], value['id'])] = revisions
        except (ValueError, OSError, KeyError, TypeError) as error:
            errors.append({'id': folder.name, 'reason': 'disposition-unverified', 'detail': str(error)})
    rows = []
    for key in sorted(set(expected) | set(admitted) | set(dispositions)):
        proposed, work = expected.get(key), admitted.get(key)
        decisions = dispositions.get(key, [])
        decision = decisions[-1] if decisions else None
        reasons = []
        if proposed:
            row = dict(proposed)
        elif work:
            row = {k: work[0][k] for k in ('id', 'namespace', 'owner', 'objective', 'not_before', 'due_at', 'timezone')}
            row.update(calendar_id=work[0].get('occurrence', {}).get('schedule_id'), calendar_revision=None)
        else:
            value = decision['payload']
            row = {k: value[k] for k in ('id', 'namespace', 'calendar_id')}
            row.update(owner=value['by'], objective='Recorded calendar disposition', not_before=None,
                       due_at=None, timezone=None, calendar_revision=None)
        row.update(disposition=decision, disposition_history=decisions, milestones=[])
        if work:
            request, binding, view = work
            row.update(owner=request['owner'], objective=request['objective'], commitment=view,
                       milestones=view.get('milestones', []), execution=view['execution'],
                       acceptance=view['acceptance'], timeliness=view['timeliness'])
            state = ('completed' if view['acceptance'] == 'accepted' else
                     'unverified' if view['execution'] == 'unverified' else
                     'unsubmitted' if view['execution'] == 'unsubmitted' else 'active')
            reasons.extend(view['escalation']['reasons'])
            if state == 'unsubmitted' and instant(request['not_before']) <= evaluated:
                reasons.append('not-submitted')
            if view['execution'] in ('failed', 'cancelled', 'waiting') and view['acceptance'] != 'accepted':
                reasons.append('execution-'+view['execution'])
            if proposed:
                if (any(request[k] != proposed[k] for k in ('not_before','due_at','timezone')) or
                        request.get('occurrence') != proposed['occurrence'] or
                        sha(encode(binding['process'])) != proposed['process_sha256'] or
                        binding['lock_sha256'] != proposed['lock_sha256']):
                    reasons.append('calendar-conflict')
                row['commitment_dates'] = {k: request[k] for k in ('not_before','due_at','timezone')}
            elif row['calendar_id']:
                reasons.append('outside-registered-calendar')
        else:
            state = 'upcoming' if row['not_before'] and evaluated < instant(row['not_before']) else 'missing'
            row.update(execution='unsubmitted', acceptance='unconfirmed',
                       timeliness='overdue' if row['due_at'] and evaluated > instant(row['due_at']) else 'pending')
            if state == 'missing': reasons.append('missing-commitment')
            if row['timeliness'] == 'overdue': reasons.append('overdue')
        if key in broken:
            state = 'unverified'
            row.update(execution='unverified', acceptance='unverified', timeliness='unverified')
            reasons = ['evidence-invalid']
        if decision:
            current = decision['payload']
            if not proposed or current['calendar_revision'] != proposed['calendar_revision']:
                reasons.append('disposition-calendar-conflict')
            elif current['kind'] != 'reopened':
                state = current['kind']
                reasons = [r for r in reasons if r not in ('missing-commitment', 'overdue', 'not-submitted')]
                if work and (row['execution'] not in ('cancelled','unsubmitted') or current['kind'] == 'skipped'):
                    reasons.append('disposition-execution-conflict')
                elif current['kind'] == 'cancelled':
                    reasons = [r for r in reasons if r != 'execution-cancelled' and not r.startswith('milestone:')]
        row.update(state=state, attention=sorted(set(reasons)))
        row['kind'] = 'commitment'
        rows.append(row)
    activities, activity_errors = activity_rows(root, rows, as_of)
    rows.extend(activities); errors.extend(activity_errors)
    calendars = []
    intervals = []
    for revisions in registrations:
        zone = ZoneInfo(revisions[0]['payload']['calendar']['timezone'])
        local_day = evaluated.astimezone(zone).date()
        actual_day = instant(observed).astimezone(zone).date().isoformat()
        active = next((r for r in reversed(revisions) if r['payload']['effective_from'] <= local_day.isoformat()), revisions[0])
        monitored = next((r for r in reversed(revisions) if r['payload']['effective_from'] <= actual_day), revisions[0])
        value, cal = active['payload'], active['payload']['calendar']
        local_day = evaluated.astimezone(ZoneInfo(cal['timezone'])).date()
        remaining = (date.fromisoformat(cal['end_date']) - local_day).days
        gaps = []
        for prior, following in zip(revisions, revisions[1:]):
            begin = date.fromisoformat(prior['payload']['calendar']['end_date']) + timedelta(days=1)
            end = date.fromisoformat(following['payload']['effective_from']) - timedelta(days=1)
            if begin <= end:
                gaps.append({'start_date': begin.isoformat(), 'end_date': end.isoformat(),
                             'reason': following['payload']['reason']})
        calendars.append({'namespace': value['namespace'], 'id': cal['id'], 'owner': value['owner'],
            'timezone': cal['timezone'], 'end_date': cal['end_date'], 'revision': active['revision'],
            'latest_revision': revisions[-1]['revision'], 'coverage_gaps': gaps,
            'effective_from': value['effective_from'], 'check_every_seconds': value['check_every_seconds'],
            'state': 'upcoming' if local_day.isoformat() < value['effective_from'] else
                     'expired' if remaining < 0 else 'ending-soon' if remaining <= 14 else 'current',
            'history': revisions})
        intervals.append(monitored['payload']['check_every_seconds'])
    try:
        digest = tracking_digest(root)
    except (ValueError, OSError) as error:
        digest = None
        errors.append({'id': 'tracking', 'reason': 'inventory-unverified', 'detail': str(error)})
    if starting_digest != digest or starting_digest is None:
        errors.append({'id': 'tracking', 'reason': 'snapshot-changed',
                       'detail': 'Tracking changed during inspection or could not be read; generate a fresh view.'})
    monitor = monitor_view(root, digest, min(intervals, default=3600), observed)
    if errors or any(row['execution'] == 'unverified' for row in rows): monitor['state'] = 'unverified'
    attention = [{'namespace': r['namespace'], 'id': r['id'], 'owner': r['owner'],
                  'reasons': r['attention']} for r in rows if r['attention']]
    attention.extend({'namespace': c['namespace'], 'id': c['id'], 'owner': c['owner'],
                      'reasons': ['calendar-'+c['state']]} for c in calendars if c['state'] not in ('current','upcoming'))
    attention.extend({'namespace': c['namespace'], 'id': c['id'], 'owner': c['owner'],
                      'reasons': ['calendar-coverage-gap'], 'detail': c['coverage_gaps']}
                     for c in calendars if c['coverage_gaps'])
    attention.extend({'id': e['id'], 'owner': None, 'reasons': [e['reason']], 'detail': e['detail']} for e in errors)
    if not calendars: attention.append({'id': 'registry', 'owner': None, 'reasons': ['no-registered-calendars']})
    if monitor['state'] != 'current':
        attention.append({'id': 'monitor', 'owner': None, 'reasons': ['reconciliation-'+monitor['state']]})
    return {'schema': 'bench.team-calendar-view/v1', 'as_of': as_of, 'observed_at': observed,
            'input_sha256': digest, 'monitor': monitor, 'calendars': calendars, 'obligations': rows,
            'attention': attention, 'errors': errors, 'notifications_sent': False}


def reconcile(root, milestones=None):
    root = runtime_root(root)
    report = calendar_view(root, now(), milestones)
    require(report['calendars'] and not report['errors'] and
            all(r['execution'] != 'unverified' for r in report['obligations']),
            'reconciliation incomplete; inspect calendar view errors; previous success is unchanged')
    with lock(root):
        require(tracking_digest(root) == report['input_sha256'], 'tracking changed during reconciliation; run again')
        checkpoint = {'schema': 'bench.reconciliation/v1', 'completed_at': now(),
                      'input_sha256': report['input_sha256'], 'obligations': len(report['obligations']),
                      'attention': sum(1 for item in report['attention'] if item['id'] != 'monitor')}
        replace_record(root/'last-reconciliation.json', checkpoint)
    report['monitor'] = monitor_view(root, report['input_sha256'], report['monitor']['check_every_seconds'], now())
    report['attention'] = [a for a in report['attention'] if not (a['id'] == 'monitor' and 'namespace' not in a)]
    return report


def calendar_events(report):
    events = []
    for index, row in enumerate(report['obligations']):
        if not row['due_at']:
            continue
        common = {'namespace': row['namespace'], 'owner': row['owner'], 'timezone': row['timezone'],
                  'row': index, 'sequence': row.get('calendar_sequence', 0)}
        events.append({**common, 'id': row['id'],
                       'start': row['due_at'] if row.get('kind') == 'human-activity' else row['not_before'], 'due': row['due_at'],
                       'identity': ['human-activity' if row.get('kind') == 'human-activity' else 'work', row['namespace'], row['id']],
                       'title': row['objective'], 'state': row['state'], 'attention': row['attention'],
                       'timeliness': row['timeliness']})
        for milestone in row['milestones']:
            events.append({**common, 'id': row['id']+'/milestone/'+milestone['id'],
                'identity': ['milestone', row['namespace'], row['id'], milestone['id']],
                'owner': milestone['owner'], 'start': milestone['due_at'], 'due': milestone['due_at'],
                'title': milestone['expectation'], 'state': milestone['state'],
                'attention': ['milestone-overdue'] if milestone['timeliness'] == 'overdue' else [],
                'timeliness': milestone['timeliness']})
    return events


def display_time(value, zone):
    return instant(value).astimezone(ZoneInfo(zone)).strftime('%b %d, %Y %H:%M %Z')


def attention_label(reason):
    labels = {'missing-commitment': 'Scheduled work has no commitment', 'not-submitted': 'Work has not been submitted',
        'calendar-conflict': 'Commitment differs from the registered calendar',
        'outside-registered-calendar': 'Commitment is outside the registered calendar',
        'disposition-calendar-conflict': 'Recorded decision differs from the current calendar',
        'disposition-execution-conflict': 'Recorded decision conflicts with execution',
        'calendar-coverage-gap': 'Dates are not covered by a registered calendar',
        'evidence-invalid': 'Work evidence could not be verified',
        'execution-outcome-unknown': 'Execution outcome is unknown', 'calendar-ending-soon': 'Calendar ends within 14 days',
        'no-registered-calendars': 'No calendars are registered',
        'reconciliation-never-reconciled': 'Calendar reconciliation has never run',
        'reconciliation-changed': 'Tracking changed since the last reconciliation',
        'reconciliation-stale': 'Calendar reconciliation is overdue',
        'reconciliation-unverified': 'Calendar reconciliation could not be verified'}
    if reason.startswith('milestone:'): return 'Overdue milestone: '+reason.split(':',1)[1]
    return labels.get(reason, reason.replace('-', ' ').capitalize())


def render_calendar(report):
    escape = lambda value: html.escape(str(value), quote=True)
    monitor = report['monitor']
    events = calendar_events(report)
    grouped = {}
    for event in events:
        day = instant(event['due']).astimezone(ZoneInfo(event['timezone'])).date()
        grouped.setdefault(day, []).append(event)
    months = sorted({(day.year, day.month) for day in grouped})
    if not months:
        today = instant(report['as_of']).date(); months = [(today.year, today.month)]
    options, grids = [], []
    for year, month in months:
        name = '%04d-%02d' % (year, month)
        label = '%s %s' % (month_calendar.month_name[month], year)
        options.append('<option value="%s">%s</option>' % (name, label))
        cells = ['<div class="weekday">%s</div>' % day for day in ('Mon','Tue','Wed','Thu','Fri','Sat','Sun')]
        for week in month_calendar.Calendar().monthdatescalendar(year, month):
            for day in week:
                if day.month != month:
                    cells.append('<div class="day outside"></div>'); continue
                items = []
                for event in grouped.get(day, []):
                    label = event['title'] + ' · ' + event['owner']
                    items.append('<a class="event %s" href="#work-%d"><strong>%s</strong><span>%s · %s</span></a>' % (
                        'flag' if event['attention'] else '', event['row'], escape(label),
                        escape(event['state']), escape(display_time(event['due'], event['timezone']))))
                cells.append('<div class="day"><b>%d</b>%s</div>' % (day.day, ''.join(items)))
        grids.append('<section class="month" data-month="%s"><h3>%s</h3><div class="grid">%s</div></section>' % (
            name, label, ''.join(cells)))
    attention_items = []
    for item in report['attention']:
        detail = item.get('detail', '')
        if isinstance(detail, list):
            detail = '; '.join(gap['start_date']+' through '+gap['end_date']+': '+gap['reason'] for gap in detail)
        attention_items.append('<li><strong>%s</strong> — %s <span>%s</span>%s</li>' % (
            escape(item['id']), escape('; '.join(attention_label(r) for r in item['reasons'])),
            escape(item.get('owner') or 'Calendar operator'), '<p>'+escape(detail)+'</p>' if detail else ''))
    attention = ''.join(attention_items) or '<li>No exceptions at the selected time.</li>'
    rows = []
    for index, row in enumerate(report['obligations']):
        dates = ('Starts '+display_time(row['not_before'], row['timezone'])+'; due '+
                 display_time(row['due_at'], row['timezone'])) if row['due_at'] else 'No current calendar dates'
        detail = '<p>'+escape(dates)+'</p>'
        if row.get('commitment_dates') and any(row['commitment_dates'][k] != row[k] for k in ('not_before','due_at','timezone')):
            old = row['commitment_dates']
            detail += '<p><strong>Existing commitment due:</strong> '+escape(display_time(old['due_at'], old['timezone']))+'</p>'
        if row['disposition_history']:
            detail += '<ul>'+''.join('<li>%s by %s: %s (%s)</li>' % (
                escape(d['payload']['kind']), escape(d['payload']['by']), escape(d['payload']['reason']),
                escape(d['recorded_at'])) for d in row['disposition_history'])+'</ul>'
        if row['milestones']:
            detail += '<ul>'+''.join('<li>%s — %s · %s · %s · %s</li>' % (
                escape(m['expectation']), escape(m['owner']), escape(display_time(m['due_at'], row['timezone'])),
                escape(m['state']), escape(m['timeliness'])) for m in row['milestones'])+'</ul>'
        rows.append('<tr id="work-%d"><td><details><summary>%s</summary><p>%s / %s</p>%s</details></td>'
                    '<td>%s</td><td>%s<br><small>%s / %s</small></td><td>%s</td><td>%s</td></tr>' % (
            index, escape(row['objective']), escape(row['namespace']), escape(row['id']), detail,
            escape(row['owner']), escape(row['state']), escape(row['execution']), escape(row['acceptance']),
            escape(row['timeliness']), escape('; '.join(attention_label(r) for r in row['attention']) or '—')))
    history_html = ''.join('<details><summary>%s · %s · %s · ends %s</summary><ul>%s</ul></details>' % (
        escape(c['id']), escape(c['owner']), escape(c['state']), escape(c['end_date']), ''.join(
        '<li>Version %s, effective %s: %s (owner: %s; recorded %s)</li>' % (
            r['sequence'], escape(r['payload']['effective_from']), escape(r['payload']['reason']),
            escape(r['payload']['owner']), escape(r['recorded_at'])) for r in c['history'])) for c in report['calendars'])
    return '''<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Team calendar and attention</title><style>
:root{font:15px/1.5 system-ui,sans-serif;color:#172638;background:#f4f6fa}body{max-width:1440px;margin:auto;padding:28px}
h1{font-size:32px;margin-bottom:8px}h2{margin-top:32px}small,span{color:#4d5e70}header p{margin:6px 0}
.banner{padding:16px;border:1px solid #b26a00;border-radius:8px;background:#fff3db;margin:20px 0}.banner.current{background:#e5f3ef;border-color:#357763}
.grid{display:grid;grid-template-columns:repeat(7,minmax(0,1fr));gap:1px;background:#cbd4e0;border:1px solid #cbd4e0}
.weekday{background:#e7ecf4;padding:8px}.day{min-height:110px;background:white;padding:8px}.outside{background:#edf0f5}
.event{display:block;color:#193e69;text-decoration:none;background:#eaf1fa;border-left:3px solid #406fa6;border-radius:4px;margin-top:8px;padding:8px;overflow-wrap:anywhere;font-size:12px}
.event span{display:block}.event.flag{background:#fff1dd;border-color:#b76b00}a:hover{text-decoration:underline}
select,input{font:inherit;padding:8px;border:1px solid #8294a9;border-radius:5px;background:white}table{border-collapse:collapse;width:100%%;background:white}
td,th{text-align:left;vertical-align:top;border-bottom:1px solid #dbe1e9;padding:12px}th{background:#e7ecf4}summary{cursor:pointer}li{margin:8px 0}
.table-wrap,.calendar-wrap{overflow-x:auto}.month{min-width:770px}#attention{border-left:4px solid #b76b00;padding-left:20px}
@media(max-width:650px){body{padding:14px}h1{font-size:26px}td,th{padding:8px}}@media print{input,select{display:none}.month{display:block!important}body{padding:0}.grid{break-inside:avoid}}
</style><header><h1>Team calendar</h1><p>Obligations, milestones and work needing attention.</p>
<p>Deadline view: %s · Observed: %s</p><p>This is a snapshot. Regenerate it to see new work. No reminders have been sent.</p></header>
<div id="monitor" class="banner %s" data-due="%s"><strong>Reconciliation: <span id="monitor-state">%s</span></strong><br>
Last successful check: %s · Next check due: %s<br>A current check can still find missing or overdue work. It does not mean everything is complete.</div>
<section id="attention"><h2>Needs attention</h2><ul>%s</ul></section>
<h2>Calendar</h2><p>Work and milestones appear on their due dates in each calendar’s timezone.</p><label>Month <select id="month">%s</select></label>
<div class="calendar-wrap">%s</div><h2>All obligations</h2><label>Filter by owner, work or status <input id="filter" type="search"></label>
<div class="table-wrap"><table><thead><tr><th>Work and dates</th><th>Owner</th><th>Status</th><th>Timeliness</th><th>Attention</th></tr></thead><tbody>%s</tbody></table></div>
<h2>Calendar history</h2>%s
<script>
const month=document.getElementById('month');
function choose(){document.querySelectorAll('.month').forEach(s=>s.hidden=s.dataset.month!==month.value)}
const selected=%s;if([...month.options].some(o=>o.value===selected))month.value=selected;choose();month.addEventListener('change',choose);
document.getElementById('filter').addEventListener('input',e=>{const q=e.target.value.toLowerCase();document.querySelectorAll('tbody tr').forEach(r=>r.hidden=!r.textContent.toLowerCase().includes(q))});
function freshness(){const b=document.getElementById('monitor');if(b.classList.contains('current')&&Date.now()>Date.parse(b.dataset.due)){b.classList.remove('current');document.getElementById('monitor-state').textContent='stale — generate a fresh view and check the monitor'}}
freshness();setInterval(freshness,10000);
</script></html>''' % (escape(report['as_of']), escape(report['observed_at']),
        'current' if monitor['state']=='current' else '', escape(monitor['next_check_due'] or ''),
        escape(monitor['state']), escape(monitor['last_success_at'] or 'Never'), escape(monitor['next_check_due'] or 'Not scheduled'),
        attention, ''.join(options), ''.join(grids), ''.join(rows), history_html,
        json.dumps(instant(report['as_of']).strftime('%Y-%m')))


def calendar_ics(report):
    def quote(value):
        return str(value).replace('\\', '\\\\').replace('\r\n', '\n').replace('\r', '\n').replace('\n', '\\n').replace(';', '\\;').replace(',', '\\,')
    def utc(value):
        return instant(value).strftime('%Y%m%dT%H%M%SZ')
    lines = ['BEGIN:VCALENDAR', 'VERSION:2.0', 'PRODID:-//Bench//Team process 0.3//EN',
             'CALSCALE:GREGORIAN', 'X-WR-CALNAME:Team obligations (snapshot)']
    for event in calendar_events(report):
        description = ('Owner: '+event['owner']+'; state: '+event['state']+'; '+event['timeliness']+
            '; attention: '+', '.join(event['attention'])+'; timezone: '+event['timezone']+
            '; snapshot: '+report['observed_at']+'; reconciliation: '+report['monitor']['state'])
        lines.extend(['BEGIN:VEVENT', 'UID:'+sha(encode(event['identity']))+'@bench-team-process',
            'DTSTAMP:'+utc(report['observed_at']), 'DTSTART:'+utc(event['start']),
            *(['DTEND:'+utc(event['due'])] if instant(event['due']) > instant(event['start']) else []),
            'SUMMARY:'+quote(event['title']),
            'DESCRIPTION:'+quote(description), 'CATEGORIES:'+quote(event['state']), 'TRANSP:TRANSPARENT', 'END:VEVENT'])
    lines.append('END:VCALENDAR')
    folded = []
    for line in lines:
        part = ''
        for char in line:
            if len((part+char).encode()) > 75:
                folded.append(part); part = ' '
            part += char
        folded.append(part)
    return '\r\n'.join(folded)+'\r\n'


KANBAN_COLUMNS = ('Planned', 'Ready', 'In progress', 'Needs attention', 'Done')


def kanban_column(row):
    # Date warnings stay on cards in their real stage; they never un-complete
    # an accepted delivery or move running work out of execution.
    blockers = [r for r in row['attention'] if r not in ('overdue','late','not-submitted') and not r.startswith('milestone:')]
    if row['execution'] == 'unverified' or blockers: return 'Needs attention'
    if row['state'] in ('skipped','cancelled'): return 'History'
    if row.get('kind') == 'human-activity':
        return {'planned':'Planned','ready':'Ready','in-progress':'In progress',
                'needs-attention':'Needs attention','done':'Done'}[row['state']]
    if row['acceptance'] == 'accepted': return 'Done'
    if row['execution'] == 'running': return 'In progress'
    if row['execution'] == 'ready': return 'Ready'
    if row['execution'] == 'unsubmitted': return 'Planned'
    return 'Needs attention'


def assignment_snapshot(instance, selected):
    instance = physical(instance)
    binding = verify(instance)
    require(binding['process']['team'] == 'page-team', 'assignment snapshot adapter supports page-team only')
    manifest_raw = read(instance/'run/manifest.json', MAX_JSON)
    manifest = decode(manifest_raw)
    require(isinstance(manifest,dict) and manifest.get('schema') == 'bench.manage.run/v1', 'invalid admitted Manage manifest')
    live = selected == 'manage-status'
    if live:
        program_binding = binding['programs'].get('BENCH_MANAGE')
        require(program_binding is not None, 'live assignments require an explicitly pinned BENCH_MANAGE program')
        queue = physical(instance/'run/tend')
        require(not queue.exists() or (queue/'state/tend.db').is_file(), 'Manage queue has not been initialized; cannot poll it')
        env = {**binding['base_environment'], **binding['request']['environment'], 'PYTHONDONTWRITEBYTECODE':'1'}
        result = capture([program_binding['path'],'status','-json',str(instance/'run')],
                         input=b'',env=env,timeout=10)
        require(result.returncode == 0, 'Manage assignment inspection failed')
        raw, source = result.stdout, 'BENCH_MANAGE status -json'
    else:
        require(Path(selected).is_absolute(), 'select an absolute public Manage status export path')
        raw, source = read(selected, MAX_JSON), str(physical(selected))
    snapshot = decode(raw)
    require(read(instance/'run/manifest.json', MAX_JSON) == manifest_raw, 'Manage manifest changed during inspection')
    require(isinstance(snapshot,dict) and isinstance(snapshot.get('tasks'),list) and len(snapshot['tasks']) <= 1000,
            'invalid Page Team assignment snapshot')
    require(snapshot.get('schema') == 'bench.manage.snapshot/v1' and
            snapshot.get('run_sha256') == 'sha256:'+sha(manifest_raw), 'assignment snapshot differs from admitted Manage run')
    observed = now()
    timestamp = snapshot.get('as_of')
    require(type(timestamp) in (int,float) and 0 <= timestamp <= instant(observed).timestamp(), 'invalid assignment observation timestamp')
    age = instant(observed).timestamp()-timestamp
    brief = read(instance/'inputs/brief').decode('utf-8').rstrip('\n')
    require(snapshot.get('goal') == brief, 'assignment snapshot differs from admitted brief')
    roster = load(Path(binding['export'])/'team.lock.json')['members']
    assignments, seen = [], set()
    for task in snapshot['tasks']:
        require(isinstance(task,dict), 'invalid assignment')
        task_id = text(task['id'],'assignment id')
        require(task_id not in seen, 'duplicate assignment id'); seen.add(task_id)
        inputs = task['input']
        require(isinstance(inputs,dict) and inputs.get('worker') in roster, 'assignment names an absent team role')
        title = text(inputs['goal'],'assignment goal')
        state = text(task['state'],'assignment state')
        strings(task['needs'], 'assignment dependencies')
        receipt = task.get('receipt')
        assignments.append({'id':task_id,'assignee':inputs['worker'],'title':title,
            'reported_state':state,'needs':task['needs'], 'job':task.get('job'),
            'check_exit':receipt.get('check_exit') if isinstance(receipt,dict) else None,
            'editable':False})
    require(all(set(t['needs']) <= seen for t in assignments), 'assignment dependency is absent from snapshot')
    return {'coverage':'public-status' if live else 'retained-snapshot',
            'freshness':('current' if age <= 30 else 'stale') if live else 'retained',
            'age_seconds':age,'source_as_of':stamp(datetime.fromtimestamp(timestamp,UTC)),
            'sha256':sha(raw),'source':source,'observed_at':observed,'assignments':assignments,
            'notice':'Controller-reported assignments; parent completion still requires its delivery checks.' if live else
                     'Assignments from a retained public status export; not live state or parent completion evidence.'}


def kanban_view(root, as_of, milestones=None, assignments=None, live_assignments=False):
    report = calendar_view(root, as_of, milestones)
    selected = dict(assignments) if isinstance(assignments,dict) else {} if assignments is None else assignments
    require(isinstance(selected,dict) and len(selected) <= 1000, 'assignments must map instance directory names to snapshots')
    instances = {Path(row['commitment']['instance']).name: row for row in report['obligations'] if row.get('commitment')}
    require(set(selected) <= set(instances), 'assignments name an absent or unverified commitment')
    if live_assignments:
        for name, row in instances.items():
            if name not in selected and row['execution'] != 'unverified' and (Path(root)/name/'run/manifest.json').is_file():
                binding = load(Path(root)/name/'admission.json')
                if binding['process']['team'] == 'page-team': selected[name] = 'manage-status'
    snapshots = {}
    for name, path in selected.items():
        try:
            snapshots[name] = assignment_snapshot(Path(root)/name, path)
        except (ValueError,OSError,KeyError,TypeError,UnicodeError,subprocess.SubprocessError) as error:
            snapshots[name] = {'coverage':'unverified','assignments':[],'error':str(error)}
            report['attention'].append({'id':name,'owner':instances[name]['owner'],
                'reasons':['assignments-unverified'],'detail':str(error)})
    cards, archived = [], []
    for row in report['obligations']:
        kind = row.get('kind','commitment')
        name = Path(row['commitment']['instance']).name if row.get('commitment') else None
        snapshot = snapshots.get(name, {'coverage':'not-selected','assignments':[]})
        column = kanban_column(row)
        if snapshot['coverage'] == 'unverified' or snapshot.get('freshness') == 'stale': column = 'Needs attention'
        card = {**row, 'card_id':sha(encode([kind,row['namespace'],row['id']])), 'column':column,
                'editable':kind == 'human-activity', 'assignments':snapshot,
                'completion_basis':'human report with retained evidence' if kind == 'human-activity' else 'team delivery acceptance'}
        (archived if column == 'History' else cards).append(card)
    # Assignment inspection never seals a checkpoint. Detect intervening tracking
    # changes so this read cannot look like a coherent current view after an edit.
    try:
        stable = tracking_digest(root) == report['input_sha256']
    except (ValueError,OSError): stable = False
    if not stable:
        report['monitor']['state'] = 'unverified'
        report['attention'].append({'id':'monitor','owner':None,'reasons':['snapshot-changed']})
    return {'schema':'bench.kanban/v1','as_of':as_of,'observed_at':report['observed_at'],
        'columns':list(KANBAN_COLUMNS),'cards':cards,'history':archived,'monitor':report['monitor'],
        'attention':report['attention'],'errors':report['errors'],'notifications_sent':False}


def render_kanban(report):
    esc = lambda x: html.escape(str(x),quote=True)
    def card(item):
        human = item.get('kind') == 'human-activity'
        due = display_time(item['due_at'],item['timezone']) if item['due_at'] else 'No current deadline'
        warning = '; '.join(attention_label(r) for r in item['attention'])
        details = '<p>'+esc(item.get('description',''))+'</p>'
        if human:
            details += '<p>Parent: '+esc(item['parent'])+'</p><p>Revision: <code>'+esc(item['revision'])+'</code></p>'
            details += '<ul>'+''.join('<li>%s — %s · %s: %s (assignee: %s; due: %s)</li>' % (
                esc(r['recorded_at']),esc(r['payload']['state']),esc(r['payload']['by']),
                esc(r['payload']['reason']),esc(r['payload']['assignee']),esc(r['payload']['due_at']))
                for r in item['activity_history'])+'</ul>'
            details += '<p>Completion evidence: '+esc(', '.join(Path(e['path']).name for e in item['evidence']) or 'None selected')+'</p>'
            if item.get('coordination'):
                linked = item['coordination']
                details += '<p><strong>Human handoff: '+esc(linked['state'])+'</strong></p>'
                if linked.get('error'): details += '<p>'+esc(linked['error'])+'</p>'
                else:
                    details += '<p>Waiting job: '+esc(linked['job_id'])+' · '+esc(linked['job_status'])+'</p>'
                    if linked.get('digest'): details += '<p>May request: <code>'+esc(linked['digest'])+'</code></p>'
                    if linked.get('response'):
                        details += '<p>Response by '+esc(linked['response']['by'])+': '+esc(linked['response']['reason'])+'</p>'
                    details += '<p>'+esc(linked['notice'])+'</p>'
        else:
            details += '<p>Execution: '+esc(item['execution'])+' · Acceptance: '+esc(item['acceptance'])+'</p>'
            details += '<p>Assignments: '+esc(item['assignments']['coverage'])+'</p>'
            if item['assignments']['coverage'] in ('retained-snapshot','public-status'):
                details += '<p>'+esc(item['assignments']['notice'])+'</p><ul>'
                details += ''.join('<li><strong>%s</strong> · %s · reported %s<br>%s</li>' % (
                    esc(a['id']),esc(a['assignee']),esc(a['reported_state']),esc(a['title']))
                    for a in item['assignments']['assignments'])+'</ul><p>Assignment observation: '+esc(item['assignments']['source_as_of'])+' · '+esc(item['assignments']['freshness'])+'</p>'
            if item['assignments'].get('error'): details += '<p>'+esc(item['assignments']['error'])+'</p>'
        for milestone in item['milestones']:
            details += '<p>Milestone: '+esc(milestone['expectation'])+' · '+esc(milestone['owner'])+' · '+esc(milestone['timeliness'])+'</p>'
        if item.get('disposition_history'):
            details += '<ul>'+''.join('<li>%s by %s: %s</li>' % (
                esc(d['payload']['kind']),esc(d['payload']['by']),esc(d['payload']['reason']))
                for d in item['disposition_history'])+'</ul>'
        return '<article class="card" data-column="%s"><div class="type">%s</div><h3>%s</h3><p class="owner">%s</p><p>%s</p><p class="%s">%s</p>%s<details id="detail-%s"><summary>Details and history</summary><p>%s / %s</p>%s</details></article>' % (
            esc(item['column']),
            'Human activity · reported status' if human else 'Team commitment · read-only',esc(item['objective']),
            esc(item['owner']),esc(due),'late' if item['timeliness'] in ('overdue','late') else '',
            esc(item['timeliness']),'<p class="warning">'+esc(warning)+'</p>' if warning else '',
            esc(item['card_id']),esc(item['namespace']),esc(item['id']),details)
    lanes = ''.join('<section class="lane"><h2>%s <span>%d</span></h2>%s</section>' % (
        esc(column),sum(c['column']==column for c in report['cards']),
        ''.join(card(c) for c in report['cards'] if c['column']==column)) for column in KANBAN_COLUMNS)
    attention = ''.join('<li>%s — %s · %s</li>' % (esc(a['id']),
        esc('; '.join(attention_label(r) for r in a['reasons'])),esc(a.get('owner') or 'Calendar operator'))
        for a in report['attention']) or '<li>No exceptions at the selected time.</li>'
    monitor = report['monitor']
    return '''<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Team work board</title><style>
:root{font:15px/1.5 system-ui,sans-serif;color:#172638;background:#f3f5fa}body{margin:0;padding:28px}h1{margin:0;font-size:32px}h2{font-size:17px}h2 span{color:#5a6c81;font-weight:400}h3{font-size:16px;margin:8px 0}.intro{max-width:1000px}p{margin:7px 0}
.board{display:grid;grid-template-columns:repeat(5,minmax(230px,1fr));gap:16px;margin:22px 0}.scroll{overflow-x:auto}.lane{background:#e5ebf3;border-radius:10px;padding:12px;min-height:250px}.card{background:white;border:1px solid #cfd9e6;border-radius:8px;padding:14px;margin:12px 0;overflow-wrap:anywhere}.type{font-size:12px;color:#56687d}.owner{font-weight:600}.late,.warning{color:#923a07;font-weight:600}.banner{border-left:4px solid #b76b00;background:#fff1da;padding:14px;margin:20px 0}.banner.current{border-color:#357763;background:#e3f1ec}input{font:inherit;padding:9px;border:1px solid #7890aa;border-radius:5px}summary{cursor:pointer}code{font-size:11px}li{margin:8px 0}.history{max-width:1100px}.history .card{display:inline-block;width:280px;vertical-align:top;margin-right:12px}
@media(max-width:600px){body{padding:14px}.board{gap:10px}}@media print{.board{display:block}.lane{break-inside:avoid}input{display:none}}
</style><header class="intro"><h1>Team work board</h1><p>Assignments, activities and delivery commitments share the same owners and deadlines as the calendar.</p>
<p id="observation">Deadline view: %s · Observed: %s</p><p id="view-mode">This is a read-only snapshot. Team status comes from execution and acceptance checks. Human updates are recorded with attribution and evidence; refresh this view after recording a change.</p></header>
<div id="monitor" class="banner %s" data-due="%s"><strong>Reconciliation: <span id="freshness">%s</span></strong><p>Last successful check: %s · Next check due: %s</p><p>A current check can still find missing or overdue work.</p></div>
<label>Filter by owner, activity or status <input id="filter" type="search"></label><div id="kanban-content"><div class="scroll"><div class="board">%s</div></div>
<details id="attention-detail"><summary>Attention list (%d)</summary><ul>%s</ul></details><details class="history" id="history-detail"><summary>Skipped and cancelled history (%d)</summary>%s</details></div>
<script>function filterCards(){const q=document.getElementById('filter').value.toLowerCase();document.querySelectorAll('.card').forEach(c=>c.hidden=!(c.textContent+' '+c.dataset.column).toLowerCase().includes(q))}document.getElementById('filter').addEventListener('input',filterCards);
function freshness(){const b=document.getElementById('monitor');if(b.classList.contains('current')&&Date.now()>Date.parse(b.dataset.due)){b.classList.remove('current');document.getElementById('freshness').textContent='stale — generate a fresh view and check the monitor'}}freshness();setInterval(freshness,10000);</script></html>''' % (
        esc(report['as_of']),esc(report['observed_at']),'current' if monitor['state']=='current' else '',
        esc(monitor['next_check_due'] or ''),esc(monitor['state']),esc(monitor['last_success_at'] or 'Never'),
        esc(monitor['next_check_due'] or 'Not scheduled'),lanes,len(report['attention']),attention,
        len(report['history']),''.join(card(c) for c in report['history']))


def live_kanban_page(page, interval):
    script = '''<script>
document.getElementById('view-mode').textContent='Live read-only view. Changes to recorded work refresh automatically; team completion still requires acceptance checks.';
const live=document.createElement('p');live.id='live-status';live.setAttribute('role','status');live.setAttribute('aria-live','polite');live.textContent='Connecting to live updates…';document.querySelector('header').appendChild(live);
let lastLive='none yet';
async function refresh(){const controller=new AbortController();const timeout=setTimeout(()=>controller.abort(),15000);
try{const response=await fetch('/view',{cache:'no-store',signal:controller.signal});if(!response.ok)throw Error('Update unavailable');
const parsed=new DOMParser().parseFromString(await response.text(),'text/html');
for(const id of ['kanban-content','monitor','observation'])if(!parsed.getElementById(id))throw Error('Incomplete update');
const opened=[...document.querySelectorAll('details[open][id]')].map(d=>d.id);
for(const id of ['kanban-content','monitor','observation'])document.getElementById(id).replaceWith(parsed.getElementById(id));
for(const id of opened){const d=document.getElementById(id);if(d)d.open=true}filterCards();freshness();
lastLive=new Date().toLocaleTimeString();live.textContent='Live · last successful refresh '+lastLive;live.className='';}
catch(error){live.textContent='Live update unavailable — last successful refresh: '+lastLive+'. Showing the last received view; check the connection or server.';live.className='warning';}
finally{clearTimeout(timeout);setTimeout(refresh,INTERVAL)}}refresh();
</script>'''.replace('INTERVAL',str(interval*1000))
    return page.replace('</html>',script+'</html>')


def kanban_server(root, port=8765, interval=5, milestones=None, assignments=None):
    root = physical(root)
    require(root.is_dir(), 'commitment root does not exist')
    require(type(port) is int and 0 <= port <= 65535, 'invalid port')
    require(type(interval) is int and 1 <= interval <= 60, 'poll-seconds must be 1..60')
    cache, guard = {}, threading.Lock()
    def render():
        if cache and wall_time.monotonic()-cache['at'] < interval:
            return cache['page']
        require(guard.acquire(blocking=False), 'view refresh is in progress; try again')
        try:
            report = kanban_view(root, now(), load(milestones) if milestones else None,
                                 load(assignments) if assignments else None, live_assignments=True)
            page = render_kanban(report)
            cache.update(page=page,at=wall_time.monotonic())
            return page
        finally: guard.release()
    class Handler(BaseHTTPRequestHandler):
        def do_GET(self):
            authority = '127.0.0.1:'+str(self.server.server_port)
            allowed = (authority,'localhost:'+str(self.server.server_port))
            host = self.headers.get('Host','')
            origin = self.headers.get('Origin')
            if host not in allowed or (origin and origin != 'http://'+host) or self.headers.get('Sec-Fetch-Site') == 'cross-site':
                self.send_error(403,'Only same-origin local viewing is supported'); return
            if self.path not in ('/','/view'):
                self.send_error(404); return
            try:
                page = render()
                if self.path == '/': page = live_kanban_page(page,interval)
                raw = page.encode()
            except (ValueError,OSError,KeyError,TypeError,subprocess.SubprocessError):
                self.send_error(503,'View unavailable; previous client view remains unchanged'); return
            self.send_response(200)
            self.send_header('Content-Type','text/html; charset=utf-8')
            self.send_header('Content-Length',str(len(raw)))
            self.send_header('Cache-Control','no-store')
            self.send_header('X-Content-Type-Options','nosniff')
            self.send_header('Referrer-Policy','no-referrer')
            self.send_header('Content-Security-Policy',"default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'none'")
            self.end_headers()
            try: self.wfile.write(raw)
            except (BrokenPipeError,ConnectionResetError): pass
        def do_POST(self): self.send_error(405,'This view is read-only')
        def log_message(self, pattern, *args): pass
    server = ThreadingHTTPServer(('127.0.0.1',port),Handler)
    server.daemon_threads = True
    return server


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', action='version', version=VERSION)
    commands = parser.add_subparsers(dest='command', required=True)
    p = commands.add_parser('validate'); p.add_argument('process')
    p = commands.add_parser('plan'); p.add_argument('process'); p.add_argument('calendar'); p.add_argument('--as-of', required=True)
    p = commands.add_parser('admit'); p.add_argument('export'); p.add_argument('request'); p.add_argument('root'); p.add_argument('--tend', default='tend')
    p = commands.add_parser('submit'); p.add_argument('instance')
    p = commands.add_parser('status'); p.add_argument('instance'); p.add_argument('--as-of', required=True); p.add_argument('--milestones')
    p = commands.add_parser('board'); p.add_argument('root'); p.add_argument('--as-of', required=True); p.add_argument('--milestones')
    p = commands.add_parser('_execute'); p.add_argument('instance'); p.add_argument('digest')
    p = commands.add_parser('register-calendar'); p.add_argument('export'); p.add_argument('registration'); p.add_argument('root')
    p = commands.add_parser('record-disposition'); p.add_argument('root'); p.add_argument('request')
    p = commands.add_parser('reconcile'); p.add_argument('root'); p.add_argument('--milestones')
    p = commands.add_parser('calendar'); p.add_argument('root'); p.add_argument('--as-of', required=True)
    p.add_argument('--milestones'); p.add_argument('--format', choices=('json','html','ics'), default='json')
    p = commands.add_parser('record-activity'); p.add_argument('root'); p.add_argument('request')
    p = commands.add_parser('link-human'); p.add_argument('root'); p.add_argument('request')
    p = commands.add_parser('respond-human'); p.add_argument('root'); p.add_argument('request')
    p = commands.add_parser('kanban'); p.add_argument('root'); p.add_argument('--as-of', required=True)
    p.add_argument('--milestones'); p.add_argument('--assignments'); p.add_argument('--format', choices=('json','html'), default='json')
    p.add_argument('--live-assignments',action='store_true')
    p = commands.add_parser('serve'); p.add_argument('root'); p.add_argument('--port',type=int,default=8765)
    p.add_argument('--poll-seconds',type=int,default=5); p.add_argument('--milestones'); p.add_argument('--assignments')
    args = parser.parse_args()
    if args.command == 'validate':
        definition = process(load(args.process))
        result = {'valid': True, 'process': definition['id'], 'stages': len(definition['stages'])}
    elif args.command == 'plan':
        results = plan(process(load(args.process)), load(args.calendar), args.as_of)
        sys.stdout.buffer.write(b''.join(encode(item) for item in results))
        return 0
    elif args.command == 'admit':
        instance = admit(args.export, load(args.request), args.root, args.tend)
        result = {'instance': str(instance), 'job_id': instance.name}
    elif args.command == 'submit':
        result = submit(args.instance)
    elif args.command == 'register-calendar':
        result = register_calendar(args.export, load(args.registration), args.root)
    elif args.command == 'record-disposition':
        result = record_disposition(args.root, load(args.request))
    elif args.command == 'record-activity':
        result = record_activity(args.root, load(args.request))
    elif args.command == 'link-human':
        result = link_human(args.root, load(args.request))
    elif args.command == 'respond-human':
        result = respond_human(args.root, load(args.request))
    elif args.command == 'kanban':
        result = kanban_view(args.root, args.as_of, load(args.milestones) if args.milestones else None,
                             load(args.assignments) if args.assignments else None,args.live_assignments)
        if args.format == 'html':
            sys.stdout.buffer.write(render_kanban(result).encode())
            return 0
    elif args.command == 'serve':
        with kanban_server(args.root,args.port,args.poll_seconds,args.milestones,args.assignments) as server:
            print('Live read-only board: http://127.0.0.1:'+str(server.server_port)+'/',file=sys.stderr,flush=True)
            try: server.serve_forever()
            except KeyboardInterrupt: pass
        return 0
    elif args.command == 'reconcile':
        result = reconcile(args.root, load(args.milestones) if args.milestones else None)
    elif args.command == 'calendar':
        result = calendar_view(args.root, args.as_of, load(args.milestones) if args.milestones else None)
        if args.format != 'json':
            sys.stdout.buffer.write((render_calendar(result) if args.format == 'html' else calendar_ics(result)).encode())
            return 0
    elif args.command == 'status':
        result = status(args.instance, args.as_of, load(args.milestones) if args.milestones else None)
    elif args.command == 'board':
        root = physical(args.root)
        require(root.is_dir(), 'commitment root does not exist')
        instances = sorted(root.glob('process-*'))
        require(len(instances) <= 1000, 'board exceeds 1000 commitments')
        supplied = load(args.milestones) if args.milestones else {}
        require(isinstance(supplied, dict) and set(supplied) <= {path.name for path in instances},
                'board milestone evidence must map admitted instance directory names to observations')
        result = [status(path, args.as_of, supplied.get(path.name)) for path in instances]
    else:
        return execute(args.instance, args.digest)
    sys.stdout.buffer.write(encode(result))
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (ValueError, OSError, KeyError, TypeError, UnicodeError, RecursionError, subprocess.SubprocessError) as error:
        print('team-process: ' + str(error), file=sys.stderr)
        sys.exit(125 if '_execute' in sys.argv[1:2] else 2)
