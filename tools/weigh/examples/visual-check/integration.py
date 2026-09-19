#!/usr/bin/env python3
"""Required real Ask + Record executable proof against a loopback vision fixture.

No model service is contacted. Missing binaries are errors, never skips.
"""
import argparse
import base64
import hashlib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import os
from pathlib import Path
import struct
import subprocess
import sys
import tempfile
import threading
import zlib

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def png(rgb):
    """One actual decodable pixel, constructed locally without image generation."""
    def chunk(kind, data):
        return struct.pack('!I', len(data)) + kind + data + struct.pack('!I', zlib.crc32(kind + data))
    return (b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('!IIBBBBB', 1, 1, 8, 2, 0, 0, 0))
            + chunk(b'IDAT', zlib.compress(b'\x00' + bytes(rgb))) + chunk(b'IEND', b''))


def run(argv, env, expected=0):
    result = subprocess.run([str(x) for x in argv], env=env, capture_output=True, timeout=45)
    require(result.returncode == expected,
            'unexpected exit %s from %s:\n%s' % (result.returncode, argv[0], result.stderr.decode(errors='replace')))
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bin-dir', required=True, help='directory containing freshly built ask and record')
    args = parser.parse_args()
    bins = Path(args.bin_dir).resolve()
    ask, record = bins/'ask', bins/'record'
    for binary in (ask, record):
        require(binary.is_file() and os.access(binary, os.X_OK), 'required executable unavailable: ' + str(binary))
    checker = HERE.parent/'semantic-check'/'check.py'
    require(checker.is_file() and os.access(checker, os.X_OK), 'semantic checker unavailable')
    with tempfile.TemporaryDirectory(prefix='bench-visual-integration-') as temporary:
        work = Path(temporary)
        private_home = work/'home'
        private_home.mkdir()
        rubric = work/'rubric.json'
        rubric.write_text(json.dumps({'version':1, 'criteria':[
            {'id':'color','requirement':'The supplied pixel is red.','feedback':'Use red.'},
            {'id':'visible','requirement':'The pixel is visible.','feedback':'Show the pixel.'}]}))
        images = [work/'one.png', work/'two.png']
        image_bytes = [png((255,0,0)), png((0,0,255))]
        for path, data in zip(images, image_bytes):
            path.write_bytes(data)
        state = {'mode':'observed', 'record_dir':None, 'calls':[], 'errors':[]}

        class Fixture(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def do_POST(self):
                try:
                    require(self.path == '/api/v1/chat/completions', 'unexpected provider endpoint: ' + self.path)
                    require(self.headers.get('Authorization') == 'Bearer offline-visual-fixture', 'fixture authorization missing')
                    request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
                    state['calls'].append(request)
                    require(request.get('stream') is True, 'Ask must stream the response')
                    response_format = request['response_format']
                    require(response_format['type'] == 'json_schema', 'missing native schema')
                    require(response_format['json_schema']['strict'] is True, 'schema is not strict')
                    require(request['provider']['require_parameters'] is True, 'schema routing not required')
                    schema = response_format['json_schema']['schema']
                    parts = []
                    for message in request['messages']:
                        content = message['content']
                        parts.extend(content if isinstance(content, list) else [{'type':'text','text':content}])
                    if request['model'] == 'fixture/vision':
                        require(schema == json.loads((state['record_dir']/'observations.schema.json').read_text()),
                                'wire observation schema differs from recorded schema')
                        attached = [x['image_url']['url'] for x in parts if x['type'] == 'image_url']
                        require(len(attached) == len(images), 'missing or extra image attachment')
                        payload = json.loads((state['record_dir']/'input.json').read_text())
                        text_parts = '\n'.join(x['text'] for x in parts if x['type'] == 'text')
                        require((state['record_dir']/'input.json').read_text().strip() in text_parts,
                                'recorded input not sent to observer verbatim')
                        for url, item, data in zip(attached, payload['images'], image_bytes):
                            require(url.startswith('data:image/png;base64,'), 'attachment is not inline PNG')
                            require(base64.b64decode(url.split(',',1)[1], validate=True) == data,
                                    'image bytes changed on Ask wire')
                            require(Path(item['snapshot']).read_bytes() == data, 'snapshot differs from attachment')
                            require(item['sha256'] == hashlib.sha256(data).hexdigest(), 'incorrect binding')
                        rows = [{'criterion_id':c['id'],'image_id':i['id'],'status':'observed',
                                 'observation':'Fixture describes an inspected pixel; no model inference occurred.'}
                                for c in payload['rubric']['criteria'] for i in payload['images']]
                        if state['mode'] in ('uncertain','not_visible'):
                            rows[0]['status'] = state['mode']
                        if state['mode'] == 'invalid':
                            rows[0]['image_id'] = 'image-unknown'
                        answer = {'observations':rows,'limitations':['Offline transport fixture; no vision quality claim.']}
                    elif request['model'] == 'fixture/judge':
                        require(not any(x['type'] == 'image_url' for x in parts), 'semantic judge unexpectedly received pixels')
                        require(set(schema['properties']) == {'answers'}, 'unexpected semantic checker schema')
                        answer = {'answers':{name:'satisfied' for name in schema['properties']['answers']['properties']}}
                    else:
                        raise AssertionError('unexpected fixture model: ' + request['model'])
                    if state['mode'] == 'provider_error':
                        self.send_error(400, 'deliberate offline provider failure')
                        return
                    chunk = {'id':'fixture','object':'chat.completion.chunk','created':1,'model':request['model'],
                             'choices':[{'index':0,'delta':{'content':json.dumps(answer)},'finish_reason':'stop'}]}
                    wire = ('data: ' + json.dumps(chunk) + '\n\ndata: [DONE]\n\n').encode()
                    self.send_response(200)
                    self.send_header('Content-Type','text/event-stream')
                    self.send_header('Content-Length',str(len(wire)))
                    self.end_headers()
                    self.wfile.write(wire)
                except Exception as error:
                    state['errors'].append(str(error))
                    self.send_error(500, 'offline fixture assertion failed')

        server = ThreadingHTTPServer(('127.0.0.1',0), Fixture)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        # Only test credentials and a loopback provider endpoint enter child processes.
        env = {key:os.environ[key] for key in ('PATH','TMPDIR','SYSTEMROOT') if key in os.environ}
        env.update(HOME=str(private_home), PYTHONDONTWRITEBYTECODE='1',
                   OPENROUTER_API_KEY='offline-visual-fixture',
                   OPENROUTER_BASE_URL='http://127.0.0.1:%d/api/v1' % server.server_port)
        try:
            def observe(mode):
                state.update(mode=mode, record_dir=work/('run-'+mode))
                argv = [sys.executable,HERE/'observe.py','--model','openrouter/fixture/vision','--rubric',rubric,
                        '--records',state['record_dir'],'--ask',ask,'--record',record,'--timeout','15']
                for path in images:
                    argv.extend(['--image',path])
                return run(argv, env, 1 if mode in ('invalid','provider_error') else 0)

            def check(directory, expected):
                return run([sys.executable,HERE/'check-current.py','--observations',directory/'observations.json',
                            '--rubric',rubric,'--checker',checker,'--','--backend','ask',
                            '--model','openrouter/fixture/judge','--records',work/'checks',
                            '--ask',ask,'--record',record,'--timeout','15'], env, expected)

            result = observe('observed')
            good = state['record_dir']
            doc = json.loads(result.stdout)
            require(len(doc['observations']) == 4, 'missing criterion/view cross product')
            require(doc['observer']['model'] == 'openrouter/fixture/vision', 'observer model not stamped')
            require(good.stat().st_mode & 0o777 == 0o700, 'records directory is not private')
            run([record,'check','-ask',ask,'-f',good/'invocation.jsonl'], env)
            replay = run([record,'replay','-ask',ask,'-f',good/'invocation.jsonl','-stream','stdout'], env)
            require(replay.stdout == (good/'observations.raw.json').read_bytes(), 'Record replay lost observer bytes')
            run([ask,'replay','-check','-json',good/'observer.jsonl'], env)
            result = check(good, 0)
            require(json.loads(result.stdout)['verdict'] == 'accept', 'compatible checker did not accept fixture')
            before = len(state['calls'])
            images[0].write_bytes(png((0,255,0)))
            result = check(good, 2)
            require(not result.stdout and len(state['calls']) == before, 'stale source reached judge or emitted pass')
            images[0].write_bytes(image_bytes[0])
            for mode in ('uncertain','not_visible'):
                observe(mode)
                before = len(state['calls'])
                result = check(state['record_dir'], 1)
                require(json.loads(result.stdout)['verdict'] == 'reject', 'incomplete observation passed')
                require(len(state['calls']) == before, 'incomplete observation invoked semantic judge')
            for mode in ('invalid','provider_error'):
                result = observe(mode)
                require(not result.stdout, 'failed observation emitted envelope')
                require(not (state['record_dir']/'observations.json').exists(), 'failed observation saved accepted envelope')
            require(not state['errors'], 'fixture assertions: ' + repr(state['errors']))
            require(len(state['calls']) == 6, 'unexpected number of inference requests or retries')
        finally:
            server.shutdown()
            server.server_close()
            thread.join(timeout=5)
    print('visual-check: real Ask + Record image transport, replay, checker compatibility, and fail-closed guards passed')


if __name__ == '__main__':
    try:
        main()
    except (AssertionError, OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        print('visual-check integration: ' + str(error), file=sys.stderr)
        sys.exit(1)
