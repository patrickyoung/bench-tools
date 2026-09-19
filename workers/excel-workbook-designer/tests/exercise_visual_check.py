#!/usr/bin/env python3
"""Offline public check/Ask/Record/Cage integration with a loopback model fixture.

Uses actual saved XLSX LibreOffice/PDF pages. Fixture answers test process contracts,
not visual accuracy. No paid endpoint is selected. Keep the fresh --out evidence.
"""
import argparse
import base64
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import threading
import time

sys.dont_write_bytecode=True
from test_visual_context import xlsx,digest,put
EXPERT=Path(__file__).resolve().parents[1]/'expert'


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--out',type=Path,required=True)
    parser.add_argument('--bin',type=Path,required=True,help='Installed public Bench binaries')
    parser.add_argument('--ask',type=Path,required=True,help='Selected public Ask executable or bridge')
    args=parser.parse_args();out=args.out.absolute();out.mkdir()
    tools=args.bin.absolute();ask=args.ask.absolute()
    env={k:os.environ[k] for k in ('HOME','LANG','WORKBOOK_NODE','WORKBOOK_NODE_MODULES','WORKBOOK_PYTHON','WORKBOOK_VISUAL_SOFFICE','WORKBOOK_VISUAL_PDFTOPPM','WORKBOOK_VISUAL_CAGE') if k in os.environ}
    env.update(PATH=str(Path(env['WORKBOOK_NODE']).parent)+':'+str(tools)+':/usr/bin:/bin',PRESENT_ASK=str(tools/'ask'),AGENT_RECORD=str(tools/'record'),PYTHONDONTWRITEBYTECODE='1',BENCH_WEIGH='0',OPENROUTER_API_KEY='offline-fixture')
    state={'requests':[],'mode':'pass','work':None,'records':None};reports=[]
    class Fixture(BaseHTTPRequestHandler):
        def log_message(self,*args):pass
        def do_POST(self):
            request=json.loads(self.rfile.read(int(self.headers['Content-Length'])));state['requests'].append(request)
            run=next(p for p in state['records'].glob('visual-*') if (p/'input.json').is_file() and not (p/'judge.stdout').exists())
            packet=json.loads((run/'input.json').read_text());cells=packet['context']['cells']
            kind=state['mode'];answer={'findings':[{'id':c['id'],'verdict':kind if kind in ('fail','uncertain') else 'pass','visible_text':c['text'],'evidence':'Synthetic full-character observation.','feedback':'Wrap the selected cell and adjust only its row height.' if kind in ('fail','uncertain') else ''} for c in cells],'limitations':['Synthetic loopback answer; not a visual-quality evaluation.']}
            if kind=='partial':answer['findings']=[]
            if kind=='forged':answer['findings'][0]['id']='not-selected'
            if kind=='transcript':answer['findings'][0]['visible_text']=answer['findings'][0]['visible_text'][:-1]
            if kind=='image-changed':
                crop=packet['context']['crops'][0]['image']['path'];p=run/'context'/crop;p.write_bytes(p.read_bytes()+b'changed')
            if kind=='pdf-changed':
                p=run/'context'/packet['context']['pdf']['path'];p.write_bytes(p.read_bytes()+b'changed')
            if kind=='workbook-changed':
                p=state['work']/'output/workbook.xlsx';p.write_bytes(p.read_bytes()+b'changed')
            text='not JSON' if kind=='malformed' else json.dumps(answer)
            chunk={'id':'offline-visual','object':'chat.completion.chunk','model':'offline-visual','choices':[{'index':0,'delta':{'content':text},'finish_reason':'stop'}],'usage':{'prompt_tokens':10,'completion_tokens':20}}
            wire=('data: '+json.dumps(chunk)+'\n\ndata: [DONE]\n\n').encode()
            self.send_response(200);self.send_header('Content-Type','text/event-stream');self.send_header('Content-Length',str(len(wire)));self.end_headers();self.wfile.write(wire)
    server=ThreadingHTTPServer(('127.0.0.1',0),Fixture);threading.Thread(target=server.serve_forever,daemon=True).start()
    env['OPENROUTER_BASE_URL']='http://127.0.0.1:'+str(server.server_port)+'/v1'
    def command(label,argv,work,selected_env):
        start=time.monotonic();result=subprocess.run([str(x) for x in argv],cwd=work,env=selected_env,capture_output=True,timeout=80)
        dest=out/'processes';dest.mkdir(exist_ok=True)
        (dest/(label+'.stdout')).write_bytes(result.stdout);(dest/(label+'.stderr')).write_bytes(result.stderr)
        put(dest/(label+'.json'),{'argv':[str(x) for x in argv],'exit_code':result.returncode,'seconds':time.monotonic()-start})
        return result
    base=out/'base';(base/'inputs').mkdir(parents=True);(base/'output').mkdir()
    creator=out/'create-source.mjs';module=(Path(env['WORKBOOK_NODE_MODULES'])/'@oai/artifact-tool/dist/artifact_tool.mjs').as_uri()
    creator.write_text('import {Workbook,SpreadsheetFile} from '+json.dumps(module)+';\nconst wb=Workbook.create(),s=wb.worksheets.add("Data");s.getRange("A1:B2").values=[["Label","Count"],["Original",7]];s.getRange("A1:C3").format.columnWidth=20;await(await SpreadsheetFile.exportXlsx(wb)).save('+json.dumps(str(base/'inputs/source.xlsx'))+');\n')
    created=command('source',[env['WORKBOOK_NODE'],creator],out,env);assert created.returncode==0,created.stderr.decode()
    inputs=[{'path':'inputs/source.xlsx','sha256':digest(base/'inputs/source.xlsx')}]
    put(base/'request.json',{'schema':'bench.workbook-request/v1','mode':'edit','brief':'Change A2 to Full text, keeping all other content.','workbook':'inputs/source.xlsx','inputs':inputs})
    spec={'schema':'bench.workbook-edit/v1','request_sha256':digest(base/'request.json'),'inputs':inputs,'purpose':'Synthetic text visibility gate fixture','interpretation':'Only replace A2.',
          'changes':[{'sheet':'Data','range':'A2','reason':'Replace selected literal text.'}],
          'operations':[{'op':'values','sheet':'Data','range':'A2','values':[['Full text']]}],
          'metrics':[{'id':'label','sheet':'Data','cell':'A2'},{'id':'count','sheet':'Data','cell':'B2'}],
          'assertions':[{'metric':'label','value':'Full text'},{'metric':'count','value':7}],
          'tests':[],'previews':[{'sheet':'Data','range':'A1:C3'}],'mappings':[],'pivots':[]}
    put(base/'output/spec.json',spec);(base/'output/guide.md').write_text('Synthetic offline fixture: only Data A2 changes. All inference replies are local fixture observations, not a claim about workbook quality.\n')
    try:
        rendered=command('base-render',[EXPERT/'tools/render'],base,env);assert rendered.returncode==0,rendered.stderr.decode()
        checked=command('base-mechanical',[EXPERT/'bin/check','--mechanical'],base,env);assert checked.returncode==0,checked.stderr.decode()
        for mode,expected in [('pass',0),('fail',1),('uncertain',1),('transcript',1),('partial',2),('forged',2),('malformed',2),('image-changed',2),('pdf-changed',2),('workbook-changed',2)]:
            root=out/mode;work=root/'work';shutil.copytree(base,work);records=root/'records';records.mkdir()
            selected={**env,'WORKBOOK_VISUAL_ASK':str(ask),'WORKBOOK_VISUAL_MODEL':'openrouter/offline-visual','WORKBOOK_VISUAL_RECORDS':str(records)}
            state.update(mode=mode,work=work,records=records);count=len(state['requests'])
            p=command(mode,[EXPERT/'bin/check'],work,selected);assert p.returncode==expected,(mode,p.returncode,p.stderr.decode())
            assert len(state['requests'])==count+1,(mode,len(state['requests'])-count)
            first=state['requests'][-1];parts=[part for m in first['messages'] if isinstance(m.get('content'),list) for part in m['content']]
            images=[p['image_url']['url'] for p in parts if p.get('type')=='image_url'];assert images and all(x.startswith('data:image/png;base64,') for x in images)
            run=next(records.glob('visual-*'));packet=(run/'input.json').read_text();texts=[m['content'] for m in first['messages'] if isinstance(m.get('content'),str)]+[p['text'] for p in parts if p.get('type')=='text']
            assert any(packet in text for text in texts),'Complete manifest must reach the judge'
            context=json.loads(packet)['context'];pdf=run/'context'/context['pdf']['path']
            invocation=json.loads((run/'judge.invocation.json').read_text())['argv'];assert str(pdf) in invocation,'Record must capture the exact PDF'
            put(root/'http-request.json',first)
            if mode!='workbook-changed':
                repeated=command(mode+'-repeat',[EXPERT/'bin/check'],work,selected)
                assert repeated.returncode==expected,(mode,repeated.returncode,repeated.stderr.decode())
                assert len(state['requests'])==count+1,'An unchanged check must not resample'
                if expected<2:
                    receipt=json.loads((work/'output/result.json').read_text())['visual_check'];assert receipt['cached'] is True
                else:assert list(records.glob('pending-*'))
            if mode=='pass':
                path=run/'context'/context['pdf']['path'];path.write_bytes(path.read_bytes()+b'tampered')
                tampered=command('pass-stale-cache',[EXPERT/'bin/check'],work,selected);assert tampered.returncode==2 and len(state['requests'])==count+1
            reports.append({'case':mode,'exit':p.returncode,'loopback_requests':1,'same_bytes_resampled':False})
            put(out/'report.json',{'cases':reports,'paid_calls':0})
        # Broken visual config cannot weaken the ordinary mechanical acceptance path.
        root=out/'manual';work=root/'work';shutil.copytree(base,work)
        broken={**env,'WORKBOOK_VISUAL_ASK':'/missing','WORKBOOK_VISUAL_MODEL':'x','WORKBOOK_VISUAL_RECORDS':str(work/'records')};count=len(state['requests'])
        default=command('invalid-config',[EXPERT/'bin/check'],work,broken);assert default.returncode==2
        mechanical=command('mechanical-in-cage',[tools/'cage','-w',work,'--',EXPERT/'bin/check','--mechanical'],work,broken);assert mechanical.returncode==0,mechanical.stderr.decode()
        unknown=command('unknown-option',[EXPERT/'bin/check','--unknown'],work,env);assert unknown.returncode==2
        missing_render={**env,'WORKBOOK_VISUAL_ASK':str(ask),'WORKBOOK_VISUAL_MODEL':'openrouter/offline-visual','WORKBOOK_VISUAL_RECORDS':str(root/'no-render-records')}
        for name in ('WORKBOOK_VISUAL_SOFFICE','WORKBOOK_VISUAL_PDFTOPPM','WORKBOOK_VISUAL_CAGE','WORKBOOK_PYTHON'):missing_render.pop(name,None)
        unavailable=command('changed-text-without-renderers',[EXPERT/'bin/check'],work,missing_render);assert unavailable.returncode==2 and len(state['requests'])==count
        # Clarification remains a source-bound, workbook-free outcome; no judge.
        (work/'output/workbook.xlsx').unlink();req=json.loads((work/'request.json').read_text())
        put(work/'output/spec.json',{'schema':'bench.workbook-clarification/v1','request_sha256':digest(work/'request.json'),'inputs':req['inputs'],'reason':'Synthetic ambiguity','questions':[{'question':'Which field is intended?','source':'inputs/source.xlsx','locator':'Data!A2','alternatives':['A','B']}]})
        clarification=command('clarification',[EXPERT/'bin/check'],work,broken);assert clarification.returncode==0,clarification.stderr.decode()
        assert json.loads(clarification.stdout)['status']=='needs-input' and len(state['requests'])==count
        # Same selected strings with a numeric-only change produce zero inference.
        root=out/'zero-targets';work=root/'work';shutil.copytree(base,work);records=root/'records';records.mkdir()
        no_text=json.loads((work/'output/spec.json').read_text());no_text['changes']=[{'sheet':'Data','range':'B2','reason':'Keep numeric value.'}];no_text['operations']=[{'op':'values','sheet':'Data','range':'B2','values':[[7]]}];no_text['assertions'][0]['value']='Original';put(work/'output/spec.json',no_text)
        rendered=command('zero-render',[EXPERT/'tools/render'],work,env);assert rendered.returncode==0,rendered.stderr.decode()
        selected={**env,'WORKBOOK_VISUAL_ASK':str(ask),'WORKBOOK_VISUAL_MODEL':'openrouter/offline-visual','WORKBOOK_VISUAL_RECORDS':str(records)}
        for name in ('WORKBOOK_VISUAL_SOFFICE','WORKBOOK_VISUAL_PDFTOPPM','WORKBOOK_VISUAL_CAGE','WORKBOOK_PYTHON'):selected.pop(name,None)
        checked=command('zero-check',[EXPERT/'bin/check'],work,selected);assert checked.returncode==0,checked.stderr.decode()
        receipt=json.loads(checked.stdout)['visual_check'];assert receipt['status']=='not-applicable' and receipt['selected_cells']==0 and len(state['requests'])==count
        # Stale source/spec cannot be made acceptable by a forged old result receipt.
        (work/'inputs/source.xlsx').write_bytes(b'stale source')
        stale=command('stale-mechanical',[EXPERT/'bin/check'],work,selected);assert stale.returncode==1 and len(state['requests'])==count
        final={'cases':reports,'selected_ask':str(ask),'selected_ask_offline_record_check_and_replay':True,'actual_images_and_complete_manifest':True,'pdf_recorded_and_revalidated':True,'mechanical_in_cage':True,'clarification_and_zero_scope_inference':0,'zero_scope_requires_no_renderers':True,'changed_text_without_renderers_exit':2,'stale_mechanical_not_bypassed':True,'loopback_requests':len(state['requests']),'paid_calls':0}
        put(out/'report.json',final);print(json.dumps(final))
    finally:server.shutdown();server.server_close()


if __name__=='__main__':main()
