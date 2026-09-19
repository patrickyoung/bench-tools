"""Read-only contracts; never import or execute candidate JS/Python."""
import copy, hashlib, json, math, re, os, stat
from contextlib import contextmanager
from pathlib import Path

class Reject(Exception): pass
class Broken(Exception): pass
def need(ok, msg):
    if not ok: raise Reject(msg)
def text(v): return isinstance(v,str) and bool(v.strip()) and len(v)<=100000
def number(v): return type(v) in (int,float) and math.isfinite(v)
def num(v, lo=0, hi=math.inf): return number(v) and lo <= v <= hi
def positive(v, hi=math.inf): return num(v,0,hi) and v>0
def arr(v, nonempty=False): return isinstance(v,list) and (not nonempty or bool(v))
def strings(v, nonempty=False): return arr(v,nonempty) and all(text(x) for x in v)
def obj(v): return isinstance(v,dict)
def digest(b): return hashlib.sha256(b).hexdigest()
FILE_LIMIT=256*1024*1024
JSON_LIMIT=16*1024*1024
REFERENCE_LIMIT=16*1024*1024
INPUT_LIMIT=64*1024*1024
TEXT_LIMIT=1500000
IMAGE_LIMIT=32*1024*1024
def regular_size(p, limit=FILE_LIMIT):
    p=Path(p)
    try:
        for part in (p,*p.parents):
            need(not part.is_symlink(),f'symlink: {p}')
        st=p.lstat()
        need(stat.S_ISREG(st.st_mode),f'not a regular file: {p}')
        need(st.st_size<=limit,f'file exceeds {limit} byte limit: {p}')
        return st.st_size
    except OSError as e: raise Reject(f'cannot stat {p}: {e}')
@contextmanager
def bounded_file(p,limit=FILE_LIMIT):
    size=regular_size(p,limit)
    try:
        fd=os.open(p,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
        with os.fdopen(fd,'rb') as f:
            st=os.fstat(f.fileno())
            need(stat.S_ISREG(st.st_mode) and st.st_size==size and st.st_size<=limit,'file changed before read')
            yield f,size
            need(os.fstat(f.fileno()).st_size==size,'file changed during read')
    except OSError as e: raise Reject(f'cannot read {p}: {e}')
def bounded_bytes(p,limit=FILE_LIMIT):
    with bounded_file(p,limit) as (f,size):
        data=f.read(size+1)
        need(len(data)==size,'file changed during read')
        return data
def file_digest(p,limit=FILE_LIMIT,target=None):
    h=hashlib.sha256()
    with bounded_file(p,limit) as (f,size):
        remaining=size
        while remaining:
            block=f.read(min(1024*1024,remaining))
            need(bool(block),'file shrank during read')
            remaining-=len(block);h.update(block)
            if target is not None: target.write(block)
        need(not f.read(1),'file grew during read')
    return h.hexdigest()
def preflight(files,total,limit=FILE_LIMIT):
    need(len(files)<=20000,'too many files')
    sizes={str(p):regular_size(p,limit) for p in files}
    need(sum(sizes.values())<=total,f'aggregate exceeds {total} byte limit')
    return sizes
def canonical(v): return json.dumps(v,sort_keys=True,separators=(',',':'),ensure_ascii=False).encode()
def unique(pairs):
    d={}
    for k,v in pairs:
        need(k not in d, f'duplicate JSON key: {k}'); d[k]=v
    return d
def load(p):
    try:
        return json.loads(bounded_bytes(p,JSON_LIMIT),object_pairs_hook=unique,parse_constant=lambda x: (_ for _ in ()).throw(Reject('nonfinite JSON')))
    except (ValueError,UnicodeError,OSError) as e: raise Reject(f'cannot read JSON {p.name}: {e}')
def safe(root, rel, exists=True):
    need(isinstance(rel,str) and rel and chr(92) not in rel and chr(0) not in rel, 'invalid relative path')
    # Validate raw spelling BEFORE Path erases empty and dot components.
    need(all(x not in ('','..','.') for x in rel.split('/')),f'noncanonical path: {rel}')
    p=Path(rel)
    need(not p.is_absolute(),f'unsafe path: {rel}')
    dest=root/p
    for cur in (dest,*dest.parents):
        need(not cur.is_symlink(),f'symlink: {rel}')
    if exists: regular_size(dest)
    return dest
def read(root,rel): return load(safe(root,rel))
def request(d, root=None):
    """Resolve technical defaults in a separate object; original bytes remain bound."""
    need(obj(d),'request must be object')
    for k in ('roughDescription','style'):
        need(text(d.get(k)),f'missing/invalid request.{k}')
    r=copy.deepcopy(d)
    # Resolved content only, never presentation/verification or serialization.
    identity={k:copy.deepcopy(d.get(k,v)) for k,v in {
        'roughDescription':'','seed':'worldweaver-default-1','generationVersion':'1',
        'physicalParameters':{},'references':[],'editingRequested':False,
        'abstractOptIn':False}.items()}
    defaults={'seed':'worldweaver-default-1',
              'worldId':'world-'+digest(canonical(identity))[:24], 'generationVersion':'1',
              'abstractOptIn':False,'references':[],'performanceRequired':False,
              'editingRequested':False}
    for k,v in defaults.items(): r.setdefault(k,v)
    for k in ('seed','worldId','generationVersion'):
        need(text(r[k]),f'invalid request.{k}')
    for k in ('abstractOptIn','performanceRequired','editingRequested'):
        need(type(r[k]) is bool,f'{k} must be boolean')
    # Explicit named styles qualify; ambiguous/negated language still needs C02 review.
    named_abstract=bool(re.search(r'\b(?:abstract|non[- ]euclidean)\b',r['style'],re.I))
    negated=bool(re.search(r'\b(?:not|no|non)[ -]+abstract\b|\bnot[ -]+non[- ]euclidean\b',r['style'],re.I))
    r['abstractOptIn']=r['abstractOptIn'] or (named_abstract and not negated)
    need(strings(r['references']),'references must be path array')
    need(len(set(r['references']))==len(r['references']),'duplicate references')
    if root:
        for ref in r['references']: safe(root,ref)
    if 'physicalParameters' in r: need(obj(r['physicalParameters']),'physicalParameters must be object')
    return r

CAPS={'workers':4,'pendingJobs':64,'pendingBytes':67108864,'residentChunks':256,
      'cpuBytes':268435456,'gpuBytes':268435456,'uploadsPerFrame':4,'drawCalls':512,'particles':8192}
def pointer(d,path):
    need(text(path) and path.startswith('/'),'feature parameter must be JSON pointer')
    try:
        v=d
        for key in path[1:].split('/'):
            key=key.replace('~1','/').replace('~0','~')
            v=v[int(key)] if isinstance(v,list) else v[key]
        return v
    except (KeyError,ValueError,IndexError,TypeError): raise Reject('unresolved feature parameter: '+path)
def blueprint(b, r=None):
    if r is not None: r=request(r)
    need(obj(b) and b.get('schemaVersion')==1,'blueprint schemaVersion')
    for k in ('worldId','generationVersion','seed','style'):
        need(text(b.get(k)),f'blueprint.{k}')
        if r: need(b[k]==r[k],f'blueprint/request mismatch: {k}')
    p=b.get('physics',{})
    need(p.get('mode') in ('euclidean','abstract') and strings(p.get('rules'),True),'physics rules')
    need(p.get('conflicts')==[] and strings(p.get('alternateRules')),'unresolved physics conflicts')
    if p['mode']=='abstract':
        need(bool(p['alternateRules']),'abstract alternate rules missing')
        if r: need(r['abstractOptIn'],'abstract physics without opt-in')
    c=b.get('coordinates',{})
    need(c.get('identity')=='signed-bigint-chunks' and c.get('chunkSize') in (16,32) and
         c.get('renderOrigin')=='floating' and c.get('topology')=='unbounded-xz' and c.get('units')=='metres','coordinate contract')
    n=b.get('noise',{})
    need(n.get('algorithm') in ('perlin3','simplex3','curl3') and n.get('execution') in ('worker','webgpu-with-worker-fallback'),'noise contract')
    need(strings(n.get('streams'),True) and arr(n.get('octaves'),True) and len(n['octaves'])<=12,'noise streams/octaves')
    for o in n['octaves']:
        need(obj(o) and all(type(o.get(k)) is int and 0<o[k]<=1000000000 for k in ('numerator','denominator'))
             and number(o.get('amplitude')),'noise frequency/amplitude')
    need(arr(b.get('biomes'),True),'biomes missing')
    for v in b['biomes']:
        need(obj(v) and all(text(v.get(k)) for k in ('id','macro','meso','micro')) and positive(v.get('weight')),'biome structure')
        for k in ('elevation','moisture','temperature'):
            a=v.get(k)
            need(arr(a) and len(a)==2 and all(number(x) for x in a) and a[0]<=a[1],f'biome {k} range')
        need(all(num(x,0,1) for x in v['moisture']),'moisture range')
    if 'composition' in b:
        c=b['composition']
        need(obj(c) and all(strings(c.get(k),True) for k in
             ('largeForms','landmarks','relationships','detail','atmosphere')),'composition relationships')
    if 'population' in b:
        p=b['population']
        need(obj(p) and type(p.get('maxActive')) is int and 0<=p['maxActive']<=8192
             and arr(p.get('groups')),'population bound')
        ids=set(); total=0
        for g in p['groups']:
            need(obj(g) and all(text(g.get(k)) for k in
                 ('id','requestSpan','representation','habitat','motion'))
                 and type(g.get('maxActive')) is int and 0<g['maxActive']<=p['maxActive'],'population group')
            need(g['id'] not in ids,'duplicate population group');ids.add(g['id']);total+=g['maxActive']
            if r: need(g['requestSpan'] in r['roughDescription'],'unbound population span')
        need(total<=p['maxActive'],'population group caps exceed total')
    palette=b.get('palette')
    need(obj(palette) and palette and all(text(k) and re.fullmatch(r'#[0-9a-fA-F]{6}',str(v)) for k,v in palette.items()),'palette')
    need(arr(b.get('materials'),True),'materials')
    for m in b['materials']:
        need(obj(m) and text(m.get('id')) and m.get('type') in ('standard','physical','toon','node')
             and m.get('color') in palette and all(num(m.get(k),0,1) for k in ('roughness','metalness','transmission'))
             and type(m.get('flatShading')) is bool,'material parameters')
    need(arr(b.get('structures')),'structures must be array (may be empty)')
    for s in b['structures']:
        need(obj(s) and all(text(s.get(k)) for k in ('id','primitive','role')) and num(s.get('minClearance'))
             and type(s.get('supportRequired')) is bool and arr(s.get('size')) and len(s['size'])==3
             and all(positive(x) for x in s['size']),'structure constraints')
    i=b.get('interactions',{})
    need(obj(i) and set(i)=={'navigation','camera','editing'},'exploration interaction contract')
    for k in ('navigation','camera'):
        need(obj(i.get(k)) and all(text(i[k].get(f)) for f in ('id','action','effect')),f'interaction {k}')
    e=i.get('editing')
    need(obj(e) and type(e.get('enabled')) is bool and strings(e.get('requestSpans')),'editing applicability')
    if r and r['editingRequested']: need(e['enabled'],'explicit editing request cannot be disabled')
    if e['enabled']:
        need(r is None or bool(e['requestSpans']) or r['editingRequested'],'editing needs requested intent')
        if r:
            need(all(span in r['roughDescription'] for span in e['requestSpans']),'unbound editing request span')
        need(arr(e.get('actions'),True),'requested edit actions missing')
        ids=set()
        for a in e['actions']:
            need(obj(a) and all(text(a.get(f)) for f in ('id','action','effect'))
                 and re.fullmatch(r'[a-z][a-z0-9-]*',a['id']),'edit action contract')
            need(a['id'] not in ids,'duplicate edit action'); ids.add(a['id'])
    else:
        need(not e['requestSpans'] and not e.get('actions'),'disabled editing has actions or request spans')
    # Description intent/unsupported additions are ALWAYS reviewed under C08-C10.
    w=b.get('weather',{})
    need(positive(w.get('cycleSeconds')) and strings(w.get('states'),True),'weather cycle')
    budgets=b.get('budgets',{})
    for k,limit in CAPS.items(): need(type(budgets.get(k)) is int and 0<budgets[k]<=limit,f'budget {k} out of bounds')
    need(positive(budgets.get('dpr'),2) and budgets.get('frameMs')==11.11 and budgets.get('targetFps')==90,'frame/DPR target')
    for k in ('defaults','referenceObservations','constraints','repairs'): need(arr(b.get(k)),f'blueprint.{k}')
    need(arr(b.get('features'),True),'feature traceability')
    seen=set()
    for f in b['features']:
        need(obj(f) and all(text(f.get(k)) for k in ('id','requestSpan','parameter','implementation','observation'))
             and f.get('status')=='implemented','incomplete feature mapping')
        need(f['id'] not in seen,'duplicate feature ID'); seen.add(f['id'])
        pointer(b,f['parameter'])
        need(f['implementation'].startswith('phase-03/runtime/src/'),'feature implementation path')
        if r: need(f['requestSpan'] in r['roughDescription'] or f['requestSpan'] in r['style'],'unbound feature quote')
    return b
def lighting(l):
    need(obj(l) and l.get('schemaVersion')==1,'lighting schemaVersion')
    need((l.get('renderer'),l.get('pipeline')) in (('webgl2','webgl-composer'),('webgpu','webgpu-nodes')),'renderer/effect incompatibility')
    need(l.get('fallback') in ('webgl2','unavailable'),'fallback missing')
    e=l.get('environment',{})
    need(e.get('kind') in ('procedural','local-hdri') and text(e.get('description')),'environment')
    if e['kind']=='local-hdri': need(text(e.get('path')),'HDRI path')
    f=l.get('fog',{})
    need(re.fullmatch(r'#[0-9a-fA-F]{6}',str(f.get('color'))) and num(f.get('near')) and number(f.get('far')) and f['far']>f['near'],'fog')
    need(positive(l.get('exposure')) and arr(l.get('lights'),True),'lighting/exposure')
    for v in l['lights']:
        need(obj(v) and text(v.get('type')) and re.fullmatch(r'#[0-9a-fA-F]{6}',str(v.get('color'))) and num(v.get('intensity')),'light')
    need(arr(l.get('effects')) and obj(l.get('weather')) and bool(l['weather']),'effects/weather')
    for e in l['effects']:
        need(obj(e) and text(e.get('id')) and type(e.get('enabled')) is bool and text(e.get('implementation')),'effect implementation')
    return l
def input_paths(root):
    paths={}
    rp=safe(root,'request.json',exists=False)
    if rp.exists():
        regular_size(rp,JSON_LIMIT)
        paths['request.json']=rp
        try: refs=load(rp).get('references',[])
        except (Reject,AttributeError): refs=[]
        if isinstance(refs,list):
            need(len(refs)<=256,'too many references')
            for ref in refs:
                p=safe(root,ref)
                regular_size(p,REFERENCE_LIMIT)
                paths[ref]=p
    preflight(list(paths.values()),INPUT_LIMIT,REFERENCE_LIMIT)
    return paths
def input_hashes(root):
    paths=input_paths(root)
    return {'request.json':None,**{rel:file_digest(p,REFERENCE_LIMIT) for rel,p in paths.items()}}
def admission(root):
    body={'schemaVersion':1,'inputHashes':input_hashes(root)}
    return {**body,'admissionHash':digest(canonical(body))}
def external_path(path,root):
    p=Path(path)
    need(p.is_absolute() and not p.resolve().is_relative_to(root),'controller-owned external absolute file required')
    regular_size(p)
    return p
def check_admission(path,root,hashes):
    p=external_path(path,root);a=load(p)
    need(obj(a) and set(a)=={'schemaVersion','inputHashes','admissionHash'} and a['schemaVersion']==1,'invalid original admission')
    body={'schemaVersion':1,'inputHashes':a['inputHashes']}
    need(a['admissionHash']==digest(canonical(body)),'invalid admission hash')
    need(a['inputHashes']==hashes,'original admission input mismatch')
    return a
def bindings(root):
    paths=input_paths(root)
    outputs={}
    for dn in ('phase-01','phase-02','phase-03','evidence'):
        base=root/dn
        need(not base.is_symlink(),f'symlink directory: {dn}')
        if not base.exists(): continue
        need(base.is_dir(),f'not a directory: {dn}')
        for p in sorted(base.rglob('*')):
            rel=p.relative_to(root).as_posix()
            if 'node_modules' in p.relative_to(base).parts: continue
            need(not p.is_symlink(),f'symlink: {rel}')
            if p.is_dir(): continue
            outputs[rel]=safe(root,rel)
            need(len(outputs)<=20000,'too many output files')
    for rel in ('notes.md','DEPENDENCIES.md','rejection.json'):
        p=safe(root,rel,exists=False)
        if p.exists(): outputs[rel]=safe(root,rel)
    need(bool(outputs),'no outputs')
    preflight(list(outputs.values()),FILE_LIMIT)
    # All aggregate/stat checks precede payload hashing.
    inputs={'request.json':None,**{rel:file_digest(p,REFERENCE_LIMIT) for rel,p in paths.items()}}
    hashes={rel:file_digest(p) for rel,p in outputs.items()}
    body={'inputHashes':inputs,'outputHashes':hashes}
    return dict(schemaVersion=1,**body,candidateHash=digest(canonical(body)))
def definition_hash(home):
    # Includes instructions, tools and policy; no mutable state permitted.
    return digest(canonical({p.relative_to(home).as_posix():digest(p.read_bytes())
          for p in sorted(home.rglob('*')) if p.is_file() and '__pycache__' not in p.parts}))
def percentile(a,q):
    a=sorted(a); return a[max(0,math.ceil(len(a)*q)-1)]
def performance(p,b,required=False):
    need(obj(p) and p.get('targetFps')==90 and p.get('frameBudgetMs')==11.11 and p.get('certified') is False,'dishonest/missing performance target or certification')
    need(p.get('status') in ('unknown','failed','measured'),'performance status')
    if p['status']!='measured':
        need(text(p.get('reason')),'performance reason missing')
        need(not p.get('targetAchieved',False),'unmeasured target claim')
        need(not required,'required 90 FPS remains unverified/failed')
        return {}
    for k in ('device','hardware','browser','backend','memoryMethod'): need(text(p.get(k)),f'performance {k}')
    need(arr(p.get('viewport')) and len(p['viewport'])==2 and all(positive(v) for v in p['viewport']),'viewport')
    need(positive(p.get('dpr'),b['dpr']) and positive(p.get('refreshHz')) and type(p.get('softwareRenderer')) is bool,'device display/backend evidence')
    need(num(p.get('warmupSeconds'),10) and num(p.get('sampleSeconds'),30) and num(p.get('coldStartMs')),'measurement duration')
    need(arr(p.get('framesMs'),True) and len(p['framesMs'])>=100 and all(positive(v) for v in p['framesMs']),'raw frame samples')
    need(abs(sum(p['framesMs'])/1000-p['sampleSeconds']) <= max(1,p['sampleSeconds']*.1),'frame duration inconsistent')
    need(arr(p.get('longTasksMs')) and all(num(v) for v in p['longTasksMs']),'long tasks')
    for k,bk in (('drawCalls','drawCalls'),('cpuBytes','cpuBytes'),('gpuBytes','gpuBytes'),('queueJobs','pendingJobs'),('queueBytes','pendingBytes'),('residentChunks','residentChunks')):
        need(arr(p.get(k),True) and all(num(v,0,b[bk]) for v in p[k]),f'measured {k} exceeded cap or missing')
    stats={f'p{int(q*100)}':percentile(p['framesMs'],q) for q in (.5,.95,.99)}
    achieved=stats['p95']<=11.11 and p['refreshHz']>=90 and not p['softwareRenderer']
    need(not p.get('targetAchieved',False) or achieved,'unsupported 90 FPS claim')
    need(not required or achieved,'required 90 FPS not established')
    return stats
def dependencies(package,lock,home,renderer):
    need(obj(package) and obj(lock),'dependency objects')
    profile=package.get('worldweaverProfile','base')
    need(isinstance(profile,str) and profile in ('base','effects','spatial','effects-spatial'),'unreviewed dependency profile')
    directory=home/('templates/runtime' if profile=='base' else 'templates/profiles/'+profile)
    expected=load(directory/'package.json')['dependencies']
    reviewed=load(directory/'package-lock.json')
    need(package.get('dependencies')==expected and lock.get('lockfileVersion')==3,'dependency pins changed')
    need(lock.get('packages')==reviewed['packages'],'lock package graph/integrities changed')
    need(not any(k in package for k in ('devDependencies','optionalDependencies','peerDependencies',
         'bundledDependencies','bundleDependencies','overrides','resolutions','workspaces')),'unchecked dependency declarations')
    need('dependencies' not in lock,'unexpected legacy lock graph')
    need('postprocessing' not in expected or renderer=='webgl2','postprocessing profile requires WebGL2')
    return profile

MODULES='coordinates noise generation chunk-worker scheduler meshing renderer interactions main'.split()
def candidate(root,home):
    current=bindings(root)
    need(read(root,'manifest.json')==current,'missing/stale input/output hashes')
    try: r=request(read(root,'request.json'),root); input_error=None
    except Reject as e: r=None; input_error=str(e)
    if (root/'rejection.json').exists():
        j=read(root,'rejection.json')
        need(obj(j) and j.get('schemaVersion')==1 and j.get('status')=='rejected' and
             j.get('code') in ('missing-input','invalid-input','causality-conflict'),'rejection format')
        need(arr(j.get('conflicts'),True) and all(obj(x) and text(x.get('requestSpan')) and text(x.get('reason')) for x in j['conflicts'])
             and strings(j.get('requiredChanges'),True),'rejection explanation')
        need(not any((root/d).exists() for d in ('phase-01','phase-02','phase-03')),'rejection mixed with runtime')
        if input_error: need(j['code'] in ('missing-input','invalid-input'),'wrong rejection kind')
        else: need(j['code']=='causality-conflict' and not r['abstractOptIn'],'unsupported input rejection')
        return current, ['C01','C02','C15'], 'rejection'
    need(not input_error,input_error or '')
    b=blueprint(read(root,'phase-01/blueprint.json'),r)
    l=lighting(read(root,'phase-02/lighting.json'))
    if l['environment']['kind']=='local-hdri': safe(root,l['environment']['path'])
    for m in MODULES + (['persistence'] if b['interactions']['editing']['enabled'] else []): need(safe(root,f'phase-03/runtime/src/{m}.js').stat().st_size>20,f'empty source {m}')
    for f in b['features']: safe(root,f['implementation'])
    for rel in ('notes.md','DEPENDENCIES.md','phase-03/runtime/index.html','phase-03/runtime/dist/index.html','phase-03/runtime/dist/app.js','phase-03/runtime/dist/chunk-worker.js'):
        need(safe(root,rel).stat().st_size>20,f'empty output {rel}')
    package=read(root,'phase-03/runtime/package.json'); lock=read(root,'phase-03/runtime/package-lock.json')
    dependencies(package,lock,home,l['renderer'])
    need(obj(package.get('scripts')) and all(text(package['scripts'].get(k)) for k in ('build','serve')),'build/serve instructions')
    for rel in current['outputHashes']:
        if rel.startswith('phase-03/runtime/') and rel.endswith(('.js','.html')):
            source=bounded_bytes(safe(root,rel)).decode('utf-8',errors='replace')
            need(not re.search(r'(?:from\s*|import\s*\(|src\s*=\s*)[\'"]https?://',source),'remote runtime import')
    performance(read(root,'evidence/performance.json'),b['budgets'],r.get('performanceRequired',False))
    use=read(root,'evidence/procedure-use.json')
    need(use.get('phases')==['intent','blueprint','lighting','runtime','verification'],'phase order evidence')
    need(arr(use.get('skills'),True),'procedure evidence')
    bypath={x.get('path'):x for x in use['skills'] if obj(x)}
    for skill in ('ww-compile','ww-runtime','ww-verify'):
        path=f'skills/{skill}/SKILL.md'; item=bypath.get(path,{})
        need(item.get('sha256')==digest((home/path).read_bytes()) and text(item.get('appliedDecision')),f'missing/stale routed procedure: {skill}')
    return current,[f'C{i:02d}' for i in range(1,16)],'world'

def review_plan(root,m,evidence):
    """Stat ALL selected evidence/source/attachments before reading any payload."""
    need(obj(evidence.get('observations')) and arr(evidence.get('files')),'controller evidence structure')
    need(len(evidence['files'])<=256,'too many evidence files')
    images=[]; records=[]; sources=[]; seen=set()
    for item in evidence['files']:
        need(obj(item) and text(item.get('path')) and text(item.get('sha256')),'invalid evidence item')
        f=external_path(item['path'],root)
        need(str(f) not in seen,'duplicate evidence file');seen.add(str(f))
        if item.get('kind')=='image':
            need(f.suffix.lower() in ('.png','.jpg','.jpeg','.webp'),'image format')
            regular_size(f,REFERENCE_LIMIT);images.append(f)
        else:
            need(item.get('kind') in ('record','browser','source-review'),'unknown evidence file kind')
            regular_size(f,512000);records.append(f)
    references=[]
    for rel,h in m['inputHashes'].items():
        if h and Path(rel).suffix.lower() in ('.png','.jpg','.jpeg','.webp'):
            f=safe(root,rel);regular_size(f,REFERENCE_LIMIT)
            references.append(f)
    for rel,h in {**m['inputHashes'],**m['outputHashes']}.items():
        if not h or rel=='manifest.json' or 'package-lock' in rel or '/dist/' in rel: continue
        if Path(rel).suffix in ('.json','.js','.mjs','.html','.md','.txt','.ts'):
            sources.append(safe(root,rel))
    preflight(images+references,IMAGE_LIMIT,REFERENCE_LIMIT)
    need(len(images+references)<=16,'too many actual image attachments')
    # Raw bytes are a lower bound on the serialized packet; final encoding is checked too.
    sizes=preflight(records+sources,TEXT_LIMIT,TEXT_LIMIT)
    need(sum(sizes.values())+len(canonical(evidence))<=TEXT_LIMIT,'aggregate review packet too large')
    return images,records,references,sources
