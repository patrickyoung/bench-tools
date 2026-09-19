"""Explicit offline Artifact Tool render/check fixture; no model calls.

Select WORKBOOK_NODE and WORKBOOK_NODE_MODULES, then pass a fresh evidence
directory. This script uses the public render/check entry points on synthetic
data, retaining the date assertion in both the passing and wrong-day cases.
"""
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import time

sys.dont_write_bytecode=True
EXPERT=Path(__file__).resolve().parents[1]/'expert'
DEST=Path(sys.argv[1]).resolve();DEST.mkdir()
NODE=os.environ['WORKBOOK_NODE'];MODULES=Path(os.environ['WORKBOOK_NODE_MODULES'])
ENV=dict(os.environ,WORKBOOK_PYTHON=sys.executable,PYTHONDONTWRITEBYTECODE='1')
def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def run(label,argv,cwd):
    start=time.monotonic();p=subprocess.run([str(a) for a in argv],cwd=cwd,env=ENV,capture_output=True,text=True,timeout=90)
    (cwd/(label+'.stdout')).write_text(p.stdout);(cwd/(label+'.stderr')).write_text(p.stderr)
    return {'argv':[str(a) for a in argv],'exit_code':p.returncode,'elapsed_seconds':time.monotonic()-start,'stdout':p.stdout,'stderr':p.stderr}

source=DEST/'synthetic-source.xlsx'
module=(MODULES/'@oai/artifact-tool/dist/artifact_tool.mjs').as_uri()
creator=DEST/'create-source.mjs'
creator.write_text(f'''import {{Workbook,SpreadsheetFile}} from {json.dumps(module)};
const wb=Workbook.create(),sh=wb.worksheets.add('Records');
sh.getRange('A1:C2').values=[['Date label','Selected date','Count'],['Recorded',new Date('2026-09-19T00:00:00.000Z'),7]];
sh.getRange('B2').setNumberFormat('yyyy-mm-dd');
sh.getRange('A1:C3').format.columnWidth=20;
await (await SpreadsheetFile.exportXlsx(wb)).save({json.dumps(str(source))});
''')
results={'source':run('create',[NODE,creator],DEST),'cases':{},'models_called':0}
assert results['source']['exit_code']==0,results['source']
for case,expected in [('typed-date',{'date':'2026-09-19'}),('iso-date','2026-09-19T00:00:00.000Z'),('wrong-day',{'date':'2026-09-18'})]:
    root=DEST/case;root.mkdir();(root/'inputs').mkdir();(root/'output').mkdir();shutil.copy2(source,root/'inputs/source.xlsx')
    inputs=[{'path':'inputs/source.xlsx','sha256':sha(root/'inputs/source.xlsx')}]
    req={'schema':'bench.workbook-request/v1','mode':'edit','workbook':'inputs/source.xlsx','brief':'Retain the selected calendar date and verify a one-day mutation.','inputs':inputs,'required_metrics':['day','count']}
    (root/'request.json').write_text(json.dumps(req))
    spec={'schema':'bench.workbook-edit/v1','request_sha256':sha(root/'request.json'),'inputs':inputs,'purpose':'Synthetic calendar date check','interpretation':'Keep the date and the independent count.',
        'changes':[{'sheet':'Records','range':'B2:C2','reason':'Write exact typed date and count.'}],
        'operations':[{'op':'values','sheet':'Records','range':'B2:C2','values':[[{'date':'2026-09-19'},7]]}],
        'metrics':[{'id':'day','sheet':'Records','cell':'B2'},{'id':'count','sheet':'Records','cell':'C2'}],
        'assertions':[{'metric':'day','value':expected},{'metric':'count','value':7}],
        'tests':[{'name':'next calendar day','edits':[{'sheet':'Records','cell':'B2','value':{'date':'2026-09-20'}}],'expect':[{'metric':'day','value':{'date':'2026-09-20'}},{'metric':'count','value':7}]}],
        'previews':[{'sheet':'Records','range':'A1:C3'}],'mappings':[],'pivots':[]}
    (root/'output/spec.json').write_text(json.dumps(spec,indent=2)+'\n');(root/'output/guide.md').write_text('Synthetic offline fixture: the selected date and count are retained, the date mutation is checked and restored. This is not a model-quality or native Excel evaluation.\n')
    rendered=run('render',[EXPERT/'tools/render'],root);checked=run('check',[EXPERT/'bin/check'],root)
    results['cases'][case]={'render':rendered,'check':checked,'spec_sha256':sha(root/'output/spec.json'),'qa':json.loads((root/'output/qa.json').read_text())}
    if case=='wrong-day':
        assert rendered['exit_code']!=0 and checked['exit_code']!=0,results['cases'][case]
        assert any(not a['passed'] for a in results['cases'][case]['qa']['assertions'])
    else:
        assert rendered['exit_code']==0 and checked['exit_code']==0,results['cases'][case]
        assert results['cases'][case]['qa']['baseline_date_metrics']==['day']
        assert results['cases'][case]['qa']['baseline']['day']=='2026-09-19T00:00:00.000Z'
(DEST/'report.json').write_text(json.dumps(results,indent=2)+'\n')
print(json.dumps({'models_called':0,'cases':{k:{'render':v['render']['exit_code'],'check':v['check']['exit_code']} for k,v in results['cases'].items()}}))
