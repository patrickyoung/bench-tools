#!/usr/bin/env python3
"""Offline public-process proof. Real Agent/Cage; real Ply precheck, controlled Ply action driver.
Usage: protected-inputs.py AGENT CAGE PLY BRIEF ASK RECORD [OUTPUT_DIRECTORY]
No model is called. Retains argv, frozen policy, streams and work under OUTPUT_DIRECTORY.
"""
import json, os, pathlib, subprocess, sys, tempfile
if len(sys.argv) not in (7, 8):
    raise SystemExit(__doc__)
agent, cage, ply, brief, ask, record = map(os.path.realpath, sys.argv[1:7])
root = pathlib.Path(sys.argv[7] if len(sys.argv) == 8 else tempfile.mkdtemp(prefix='agent-input-proof.')).resolve()
root.mkdir(parents=True, exist_ok=True)
def write(path, text, mode=0o600):
    path.write_text(text); path.chmod(mode)
# Both the checker and action execute this same test under the actual kernel boundary.
probe = '''set -eu
[ "$(cat inputs/source.txt)" = original ]
if (printf damage > inputs/source.txt) 2>/dev/null; then exit 91; fi
if rm inputs/source.txt 2>/dev/null; then exit 92; fi
if mv inputs inputs-moved 2>/dev/null; then exit 93; fi
if (printf damage > request.md) 2>/dev/null; then exit 94; fi
if rm request.md 2>/dev/null; then exit 95; fi
if (printf damage > "$AGENT_INPUT_PROTECTION") 2>/dev/null; then exit 96; fi
printf legitimate > output.txt
printf 'probe stdout\\n'
printf 'probe stderr\\n' >&2
exit "${PROBE_EXIT:-0}"
'''
shim = root/'ply-driver'
write(shim, '''#!/usr/bin/env python3
import json, os, pathlib, subprocess, sys
if sys.argv[1:] == ['capabilities']:
 print('{"schema":"ply.capabilities/v1","features":{"goal_file":true,"no_delegate":true,"process_recording":"ply.recording/v1","action_boundary_receipt":"ply.action-boundary/v1"}}'); sys.exit(0)
a=sys.argv[1:]; root=pathlib.Path(os.environ['PROBE_ROOT']); (root/'argv.json').write_text(json.dumps(a))
p=pathlib.Path(os.environ['AGENT_INPUT_PROTECTION']); (root/'frozen-policy.json').write_bytes(p.read_bytes())
work=a[a.index('-C')+1]
if os.environ.get('PROBE_ACTION') == '1':
 command=[a[a.index('-action-shell')+1], '-c', pathlib.Path(os.environ['PROBE_SCRIPT']).read_text()]
else: command=['/bin/sh', '-c', a[a.index('-check')+1]]
sys.exit(subprocess.call(command, cwd=work))
''', 0o700)
results=[]
for label, action, code, actual_ply in [('real-ply-precheck',False,0,True),('action',True,0,False),('check-reject',False,1,False),('check-interrupt',False,130,False)]:
    case=root/label; case.mkdir()
    home=case/'definition'; (home/'bin').mkdir(parents=True)
    work=case/'work'; (work/'inputs').mkdir(parents=True)
    write(home/'AGENTS.md','Offline protection proof.\n')
    write(work/'inputs/source.txt','original'); write(work/'request.md','request')
    write(home/'bin/check','#!/bin/sh\n'+probe,0o700)
    script=case/'action.sh'; write(script,probe)
    env={'PATH':'/usr/bin:/bin','HOME':str(case),'TMPDIR':str(root), 'AGENT_PROTECT_INPUTS':'1', 'AGENT_CAGE':cage,'AGENT_PLY':ply if actual_ply else str(shim), 'AGENT_BRIEF':brief,'AGENT_ASK':ask,'AGENT_RECORD':record,'PROBE_ROOT':str(case),'PROBE_ACTION':str(int(action)),'PROBE_SCRIPT':str(script),'PROBE_EXIT':str(code)}
    command=[agent,'run','-q','-B=false','-C',str(work),'-evidence',str(case/'evidence'),str(home),'--','Verify protected inputs without a model call.']
    run=subprocess.run(command,env=env,stdin=subprocess.DEVNULL,capture_output=True)
    (case/'stdout').write_bytes(run.stdout); (case/'stderr').write_bytes(run.stderr)
    good=run.returncode==code and (work/'inputs/source.txt').read_text()=='original' and (work/'request.md').read_text()=='request' and (work/'output.txt').read_text()=='legitimate'
    results.append({'case':label,'exit':run.returncode,'expected':code,'passed':good,'scope':'real Agent/Ply/Cage/Record zero-model precheck' if actual_ply else 'real Agent/Cage with controlled Ply process driver'})
(root/'results.json').write_text(json.dumps(results,indent=2)+'\n')
print(json.dumps({'root':str(root),'results':results},indent=2))
sys.exit(0 if all(r['passed'] for r in results) else 1)
