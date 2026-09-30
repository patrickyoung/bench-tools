#!/usr/bin/env python3
"""Compose a pinned team runner with the independently selected Agenda command."""
import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

RUNNER = Path(__file__).resolve().with_name('runner.py')
MAX = 128 * 1024 * 1024


def require(value, message):
    if not value: raise ValueError(message)


def physical(value):
    path = Path(os.path.abspath(value))
    require(not any(p.is_symlink() for p in [path,*path.parents]), 'symlink is not selected: '+str(path))
    return path


def read(path):
    with physical(path).open('rb') as stream: data = stream.read(MAX+1)
    require(len(data) <= MAX, 'input exceeds bound')
    return data


def pairs(values):
    result = {}
    for key,value in values:
        require(key not in result,'duplicate JSON key: '+key); result[key] = value
    return result


def decode(raw):
    return json.loads(raw.decode(),object_pairs_hook=pairs,
                      parse_constant=lambda _: require(False,'nonfinite JSON number'))


def encode(value):
    return (json.dumps(value,sort_keys=True,ensure_ascii=False,allow_nan=False,separators=(',',':'))+'\n').encode()


def digest(raw): return hashlib.sha256(raw).hexdigest()


def now(): return datetime.now(timezone.utc).isoformat().replace('+00:00','Z')


def selected_program(value):
    require(Path(value).is_absolute(),'select an absolute program path')
    path = Path(value); target = path.resolve(strict=True)
    require(os.access(path,os.X_OK),'selected program is not executable')
    return {'path':str(path),'target':str(target),'sha256':digest(read(target))}


def call(argv, value=None, jsonl=False):
    raw = b'' if value is None else value if isinstance(value,bytes) else encode(value)
    with tempfile.TemporaryFile() as out, tempfile.TemporaryFile() as err:
        result = subprocess.run(argv,input=raw,stdout=out,stderr=err,timeout=60)
        out.seek(0); err.seek(0); output,diagnostic=out.read(MAX+1),err.read(65536)
    require(result.returncode == 0,'command failed ('+str(result.returncode)+'): '+diagnostic.decode(errors='replace'))
    require(len(output) <= MAX,'command output exceeds bound')
    return [decode(line) for line in output.splitlines()] if jsonl else decode(output)


def export(agenda, root):
    result = call([agenda,'export',str(physical(root))])
    require(result.get('schema') == 'agenda.snapshot/v1','unsupported Agenda snapshot')
    return result


def latest(snapshot, kind):
    records = {}
    for record in snapshot['records']:
        change = record['change']
        if change['kind'] == kind: records[change['id']] = record
    return records


def runner(command, *args):
    return call([sys.executable,str(RUNNER),command,*map(str,args)])


def apply(agenda, root, kind, identifier, value, by, reason, previous=None, evidence=None):
    change={'schema':'agenda.change/v1','kind':kind,'id':identifier,'previous':previous,
            'by':by,'reason':reason,'value':value}
    if evidence: change['evidence']=evidence
    change['request_id']='team-'+digest(encode(change))
    return call([agenda,'apply',str(physical(root))],change)


def admit(exported, request_file, root, tend, agenda, agenda_root, item=None):
    result = runner('admit',exported,request_file,root,'--tend',tend)
    return adopt(result['instance'],agenda,agenda_root,item)


def adopt(instance, agenda, agenda_root, item=None):
    """Index an existing admission without replacing its pinned executor."""
    tool = selected_program(agenda)
    instance = physical(instance)
    checked = runner('status',instance,'--as-of',now())
    require(checked['execution'] != 'unverified','cannot adopt an unverified admission')
    request = decode(read(instance/'commitment.json'))
    result = {'instance':str(instance),'job_id':instance.name}
    identity = item or instance.name
    binding_path=instance/'control/agenda-binding.json'
    if binding_path.exists():
        existing=decode(read(binding_path))
        require(existing['agenda'] == tool and existing['root'] == str(physical(agenda_root)) and
                existing['id'] == identity, 'instance already bound to a different Agenda selection')
        records=export(agenda,agenda_root)['records']
        require(any(r['revision']==existing['revision'] and r['change']['kind']=='item' and
                    r['change']['id']==identity for r in records), 'original Agenda admission is missing')
        return {**result,'agenda':existing}
    value = {'title':request['objective'],'owner':request['owner'],'not_before':request['not_before'],
             'due_at':request['due_at'],'timezone':request['timezone'],'basis':'external',
             'extensions':{'bench_team':{'instance':str(instance),'admission_sha256':digest(read(instance/'admission.json')),
                                        'namespace':request['namespace'],'id':request['id'],
                                        'milestones':request.get('milestones',[])}}}
    # Binding an admitted execution to an existing expected occurrence is explicit.
    if item:
        snapshot = export(agenda,agenda_root)
        if '/' in item:
            schedule_id,local_date=item.rsplit('/',1)
            declarations=[r for r in snapshot['records'] if r['change']['kind']=='schedule' and
                          r['change']['id']==schedule_id and r['change']['value']['effective_from'] <= local_date]
            if declarations:
                record=max(declarations,key=lambda r:r['change']['value']['effective_from'])
                proposed=call([agenda,'expand','--as-of',now()],
                              {**record['change']['value'],'id':schedule_id},jsonl=True)
                expected=next((r for r in proposed if r['id']==item),None)
                def timestamp(value): return datetime.fromisoformat(value.replace('Z','+00:00'))
                require(expected and timestamp(expected['not_before'])==timestamp(value['not_before']) and
                        timestamp(expected['due_at'])==timestamp(value['due_at']) and
                        expected['timezone']==value['timezone'], 'commitment differs from the selected schedule occurrence')
                value['occurrence']={'schedule_id':schedule_id,'date':local_date,'schedule_revision':record['revision']}
        require('occurrence' in value, '--item must select an existing schedule occurrence')
    committed = apply(agenda,agenda_root,'item',identity,value,request['owner'],'Admit the explicitly selected team commitment')
    # Execution may have been admitted before a failed Agenda write. Repeating
    # this command uses both tools' stable identities; no job has been submitted.
    binding={'schema':'bench.agenda-admission/v1','agenda':tool,'root':str(physical(agenda_root)),
             'id':identity,'revision':committed['revision']}
    path=binding_path
    if path.exists(): require(decode(read(path)) == binding,'Agenda admission binding changed')
    else:
        with tempfile.NamedTemporaryFile(dir=path.parent,delete=False) as f:
            temporary=Path(f.name);f.write(encode(binding));f.flush();os.fsync(f.fileno())
        try:
            try: os.link(temporary,path)
            except FileExistsError: require(decode(read(path)) == binding,'concurrent Agenda admission differs')
            fd=os.open(path.parent,os.O_RDONLY)
            try: os.fsync(fd)
            finally: os.close(fd)
        finally: temporary.unlink()
    return {**result,'agenda':binding}


def submit(instance):
    instance=physical(instance)
    binding=decode(read(instance/'control/agenda-binding.json'))
    require(selected_program(binding['agenda']['path']) == binding['agenda'],'selected Agenda program changed')
    record=latest(export(binding['agenda']['path'],binding['root']),'item').get(binding['id'])
    require(record and record['revision'] == binding['revision'],'Agenda commitment changed; review before submitting')
    return runner('submit',instance)


def observations(agenda, agenda_root, run_root, as_of):
    snapshot=export(agenda,agenda_root)
    run_root=physical(run_root)
    results=[]
    for identifier,record in latest(snapshot,'item').items():
        value=record['change']['value']; selected=value.get('extensions',{}).get('bench_team')
        if not selected: continue
        observed=now()
        row={'id':identifier,'revision':record['revision'],'state':'unverified','observed_at':observed,'evidence':[]}
        try:
            instance=physical(selected['instance'])
            require(instance.parent == run_root and instance.name.startswith('process-'),'team instance is outside the selected run root')
            admission=read(instance/'admission.json')
            require(digest(admission) == selected['admission_sha256'],'team admission changed')
            binding=decode(read(instance/'control/agenda-binding.json'))
            require(selected_program(agenda) == binding['agenda'],'selected Agenda program changed')
            require(binding['id'] == identifier and binding['revision'] == record['revision'] and
                    binding['root'] == str(physical(agenda_root)), 'Agenda commitment changed since admission')
            status=runner('status',instance,'--as-of',as_of)
            observed=now(); row['observed_at']=observed
            state='accepted' if status['acceptance']=='accepted' else {'ready':'queued','done':'rejected',
                  'failed':'rejected'}.get(status['execution'],status['execution'])
            row.update(state=state,extensions={'bench_status':status},
                       evidence=[{'kind':'team-status','ref':'json:extensions/bench_status#sha256='+digest(encode(status))}])
            row['valid_until']=(datetime.fromisoformat(observed.replace('Z','+00:00'))+timedelta(seconds=60)).isoformat().replace('+00:00','Z')
            receipt=status.get('evidence',{}).get('receipt')
            if receipt:
                proof=instance/'control'/('receipt-'+receipt['attempt_key']+'.json')
                row['evidence'].append({'kind':'verified-team-receipt','ref':str(proof)+'#sha256='+digest(read(proof))})
            if state=='accepted': row['completed_at']=status['accepted_at']
        except (ValueError,OSError,KeyError,TypeError,subprocess.SubprocessError) as error:
            row.update(state='unverified',evidence=[],extensions={'error':str(error)})
            row.pop('completed_at',None)
        results.append(row)
    return results


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    commands=parser.add_subparsers(dest='command',required=True)
    p=commands.add_parser('admit'); p.add_argument('export');p.add_argument('request');p.add_argument('run_root')
    p.add_argument('--tend',required=True);p.add_argument('--agenda',required=True);p.add_argument('--agenda-root',required=True);p.add_argument('--item')
    p=commands.add_parser('submit');p.add_argument('instance')
    p=commands.add_parser('adopt');p.add_argument('instance');p.add_argument('--agenda',required=True)
    p.add_argument('--agenda-root',required=True);p.add_argument('--item')
    p=commands.add_parser('observe');p.add_argument('run_root');p.add_argument('--agenda',required=True)
    p.add_argument('--agenda-root',required=True);p.add_argument('--as-of',required=True)
    args=parser.parse_args()
    if args.command=='admit':
        result=admit(args.export,args.request,args.run_root,args.tend,args.agenda,args.agenda_root,args.item)
    elif args.command=='submit': result=submit(args.instance)
    elif args.command=='adopt': result=adopt(args.instance,args.agenda,args.agenda_root,args.item)
    else:
        result=observations(args.agenda,args.agenda_root,args.run_root,args.as_of)
        sys.stdout.buffer.write(b''.join(encode(r) for r in result));return 0
    sys.stdout.buffer.write(encode(result));return 0


if __name__=='__main__':
    try: sys.exit(main())
    except (ValueError,OSError,KeyError,TypeError,RecursionError,subprocess.SubprocessError) as error:
        print('team-agenda: '+str(error),file=sys.stderr);sys.exit(2)
