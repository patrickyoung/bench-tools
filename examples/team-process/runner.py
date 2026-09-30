#!/usr/bin/env python3
"""Stable team execution adapter: admit, submit and inspect through public Tend."""
import argparse
from contextlib import contextmanager
from datetime import date, datetime, time, timedelta, timezone
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import sys
import tempfile
from zoneinfo import ZoneInfo

VERSION = '0.1.0'
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


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', action='version', version=VERSION)
    commands = parser.add_subparsers(dest='command', required=True)
    p = commands.add_parser('admit'); p.add_argument('export'); p.add_argument('request'); p.add_argument('root'); p.add_argument('--tend', required=True)
    p = commands.add_parser('submit'); p.add_argument('instance')
    p = commands.add_parser('status'); p.add_argument('instance'); p.add_argument('--as-of', required=True); p.add_argument('--milestones')
    p = commands.add_parser('_execute'); p.add_argument('instance'); p.add_argument('digest')
    args = parser.parse_args()
    if args.command == 'admit':
        instance = admit(args.export, load(args.request), args.root, args.tend)
        result = {'instance': str(instance), 'job_id': instance.name}
    elif args.command == 'submit': result = submit(args.instance)
    elif args.command == 'status': result = status(args.instance,args.as_of,load(args.milestones) if args.milestones else None)
    else: return execute(args.instance,args.digest)
    sys.stdout.buffer.write(encode(result)); return 0


if __name__ == '__main__':
    try: sys.exit(main())
    except (ValueError,OSError,KeyError,TypeError,UnicodeError,RecursionError,subprocess.SubprocessError) as error:
        print('team-runner: '+str(error),file=sys.stderr)
        sys.exit(125 if len(sys.argv) > 1 and sys.argv[1] == '_execute' else 2)
