"""Stable outcome manifest for completed work or a source-bound clarification."""
import json,sys
from pathlib import Path
from contract import input_bindings,digest,require,validate_output
from edit_contract import check
from visual_check import Broken,configuration,visual_check
root=Path.cwd()
if sys.argv[1:] not in ([],['--mechanical']):
    print('usage: bin/check [--mechanical]',file=sys.stderr);sys.exit(2)
try:
    req=json.loads((root/'request.json').read_text());s=json.loads((root/'output/spec.json').read_text())
    if s.get('schema')=='bench.workbook-clarification/v1':
        inputs=input_bindings(root,req);require(s.get('request_sha256')==digest(root/'request.json') and s.get('inputs')==inputs,'Stale clarification binding')
        require(not (root/'output/workbook.xlsx').exists(),'Clarification cannot masquerade as delivered workbook')
        require(s.get('reason') and isinstance(s.get('questions'),list) and s['questions'],'Clarification needs exact decision')
        for q in s['questions']:require(q.get('question') and q.get('source') in [x['path'] for x in inputs] and q.get('locator') and len(q.get('alternatives',[]))>=2,'Clarification needs source locator and alternatives')
        require((root/'output/guide.md').stat().st_size>50,'Missing clarification guide')
        report={'status':'needs-input','reason':s['reason'],'questions':s['questions'],'workbook':None}
    else:
        report=check(root) if req.get('mode')=='edit' else validate_output(root)
        report['workbook']='output/workbook.xlsx'
    files=['output/spec.json','output/guide.md']+(['output/workbook.xlsx','output/qa.json'] if report['workbook'] else [])
    if report['workbook'] and (root/'output/change-report.json').exists():files.append('output/change-report.json')
    manifest={'schema':'bench.workbook-result/v1',**report,'request_sha256':digest(root/'request.json'),'artifacts':[{'path':n,'sha256':digest(root/n)} for n in files]}
    (root/'output/result.json').write_text(json.dumps(manifest,indent=2)+'\n')
except Exception as e:print(str(e),file=sys.stderr);sys.exit(1)
if sys.argv[1:]!=['--mechanical'] and report['workbook']:
    try:
        config=configuration(root,Path(__file__).resolve().parents[1])
        if config:
            visual,feedback=visual_check(root,Path(__file__).resolve().parents[1],config)
            manifest['visual_check']=visual;report['visual_check']=visual
            (root/'output/result.json').write_text(json.dumps(manifest,indent=2)+'\n')
            if feedback:
                print('Changed-text visual check rejected:\n'+'\n'.join(feedback),file=sys.stderr);sys.exit(1)
    except (Broken,OSError,ValueError,KeyError,TypeError,AttributeError,IndexError) as e:
        print('workbook visual check: '+str(e),file=sys.stderr);sys.exit(2)
print(json.dumps(report))
