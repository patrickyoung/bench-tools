#!/usr/bin/env python3
"""Bind Agenda human items to existing May decisions and Tend input waits."""
import argparse
from datetime import timedelta
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile

from runner import (MAX_JSON, require, sha, encode, decode, physical, read, load,
                    fields, text, ident, instant, stamp, now, program, verify_program,
                    durable_mkdir, syncdir, lock, tend_call, capture)
import team

VERSION = '0.1.0'


def item_id(value):
    require(isinstance(value, str) and 0 < len(value.encode()) <= 256 and
            not any(c in value for c in '\x00\r\n'), 'invalid Agenda item ID')
    return value


def tracking_key(identifier):
    return sha(encode(item_id(identifier)))[:48]


def runtime_root(root):
    root = physical(root)
    source = Path(__file__).resolve().parent
    require(root != source and source not in root.parents, 'handoff data must stay outside source')
    return root


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


def agenda_selection(agenda, root):
    require(Path(agenda).is_absolute() and Path(root).is_absolute(),
            'select absolute Agenda executable and root')
    return {'program':program(agenda), 'root':str(physical(root))}


def current_item(binding, agenda, root, require_current=False):
    selected = agenda_selection(agenda, root)
    require(binding['agenda'] == selected, 'selected Agenda binding changed')
    snapshot = team.export(selected['program']['path'], selected['root'])
    identifier = binding['request']['id']
    item = team.latest(snapshot, 'item').get(identifier)
    current = bool(item and item['change']['value']['basis'] == 'human' and
                   item['revision'] == binding['request']['item_revision'])
    if require_current:
        require(current, 'select the current human item revision')
        report = team.latest(snapshot, 'report').get(identifier)
        disposition = team.latest(snapshot, 'disposition').get(identifier)
        require(not report or report['change']['value']['state'] not in ('done','cancelled'),
                'select an open human item')
        require(not disposition or disposition['change']['value']['kind'] == 'reopened',
                'human item is disposed')
    return current


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


def link_human(root, request, agenda, agenda_root):
    fields(request, ['schema','id','item_revision','kind','tend','queue','job','signal','by','reason'],
           ['may','may_job','action','response_path'])
    require(request['schema'] == 'bench.agenda-human-link/v1' and request['kind'] in ('approval','input'), 'invalid human link')
    item_id(request['id']); ident(request['job'])
    for key in ('by','reason'): text(request[key],key)
    require(isinstance(request['signal'], str) and re.fullmatch(r'[a-z0-9._-]{1,64}',request['signal']), 'invalid wait signal')
    options = {'may','may_job','action'} if request['kind'] == 'approval' else {'response_path'}
    require(set(request) & {'may','may_job','action','response_path'} == options, 'select only the fields for this link kind')
    root = runtime_root(root)
    key = tracking_key(request['id'])
    folder = root/'human-links'/key
    with lock(root):
        if (folder/'binding.json').exists():
            binding = load(folder/'binding.json')
            require(binding['request'] == request, 'human link is immutable; create a new activity for another request')
            current_item(binding, agenda, agenda_root, require_current=True)
            return binding
        selected = agenda_selection(agenda, agenda_root)
        current_item({'agenda':selected,'request':request}, agenda, agenda_root, require_current=True)
        require(Path(request['queue']).is_absolute(), 'queue must be absolute')
        binding = {'schema':'bench.agenda-human-binding/v1','request':request,'recorded_at':now(),'agenda':selected,
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


def human_link_view(root, identifier, agenda, agenda_root):
    folder = physical(root)/'human-links'/tracking_key(identifier)
    if not folder.exists(): return None
    binding = load(folder/'binding.json')
    require(binding['schema'] == 'bench.agenda-human-binding/v1' and
            binding['request']['id'] == identifier, 'human link identity changed')
    item_current = current_item(binding, agenda, agenda_root)
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
    return {'schema':'bench.agenda-human-view/v1','id':identifier,'item_revision':binding['request']['item_revision'],
            'item_current':item_current,'kind':binding['request']['kind'],'state':state,'job_id':binding['job_id'],
            'job_status':job['status'],'signal':binding['request']['signal'],
            'digest':binding.get('digest'),'pending':pending,'response':response,
            'signal_recorded':bool(responses),'signal_woke':woke,'observed_at':now(),
            'notice':'Wakeup is not approval. The resumed controller must recheck May or validate supplied input.'}


def respond_human(root, request, agenda, agenda_root):
    fields(request, ['schema','id','by','reason'], ['input'])
    require(request['schema'] == 'bench.agenda-human-response/v1', 'invalid human response')
    item_id(request['id'])
    for key in ('by','reason'): text(request[key],key)
    root = runtime_root(root)
    folder = root/'human-links'/tracking_key(request['id'])
    require((folder/'binding.json').is_file(), 'human activity has no linked wait')
    with lock(folder):
        binding = load(folder/'binding.json')
        current_item(binding, agenda, agenda_root, require_current=True)
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
        view = human_link_view(root,request['id'],agenda,agenda_root)
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
        current_item(binding, agenda, agenda_root, require_current=True)
        tend_call(binding, ['signal','-id','human-'+sha(encode(binding))[:58],binding['job_id'],binding['request']['signal']],
                  stdin=encode(response))
        return human_link_view(root,request['id'],agenda,agenda_root)


def observation(root, identifier, agenda, agenda_root):
    """Emit coordination evidence without asserting human or team completion."""
    folder = physical(root)/'human-links'/tracking_key(identifier)
    binding = load(folder/'binding.json')
    require(binding['schema'] == 'bench.agenda-human-binding/v1' and
            binding['request']['id'] == identifier, 'human link identity changed')
    revision = binding['request']['item_revision']
    require(isinstance(revision,str) and re.fullmatch(r'[a-f0-9]{64}',revision),
            'invalid bound item revision')
    try:
        view = human_link_view(root,identifier,agenda,agenda_root)
        state = {'ready':'queued','running':'running','waiting':'waiting',
                 'unknown':'unknown','failed':'rejected','cancelled':'cancelled',
                 'done':'unsubmitted'}.get(view['job_status'],'unverified')
        if view['state'] in ('awaiting-approval','awaiting-input'):
            state = 'waiting'
        if not view['item_current']:
            state = 'unverified'
    except (ValueError,OSError,KeyError,TypeError,subprocess.SubprocessError) as error:
        state = 'unverified'
        view = {'schema':'bench.agenda-human-view/v1','id':identifier,
                'item_revision':revision,'item_current':False,'error':str(error)}
    observed = now()
    return {'id':identifier,'revision':revision,'state':state,'observed_at':observed,
            'valid_until':stamp(instant(observed)+timedelta(seconds=60)),
            'evidence':[{'kind':'human-coordination',
                         'ref':'json:extensions/human_coordination#sha256='+sha(encode(view))}],
            'extensions':{'human_coordination':view}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', action='version', version=VERSION)
    commands = parser.add_subparsers(dest='command', required=True)
    for name in ('link','inspect','observe','respond'):
        sub = commands.add_parser(name)
        sub.add_argument('root')
        sub.add_argument('id' if name in ('inspect','observe') else 'request')
        sub.add_argument('--agenda', required=True)
        sub.add_argument('--agenda-root', required=True)
    args = parser.parse_args()
    try:
        if args.command == 'inspect':
            result = human_link_view(args.root,args.id,args.agenda,args.agenda_root)
        elif args.command == 'observe':
            result = observation(args.root,args.id,args.agenda,args.agenda_root)
        else:
            request = load(args.request)
            function = link_human if args.command == 'link' else respond_human
            result = function(args.root,request,args.agenda,args.agenda_root)
        sys.stdout.buffer.write(encode(result))
        return 0
    except (ValueError,OSError,KeyError,TypeError,subprocess.SubprocessError) as error:
        print('human: '+str(error),file=sys.stderr)
        return 2


if __name__ == '__main__':
    sys.exit(main())
