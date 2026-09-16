#!/usr/bin/env python3
"""Constrained Inkscape namespace DOM construction and native serialization.

No model-provided markup, attribute names, commands or paths are interpolated.
"""
import hashlib, json, math, os, pathlib, re, shutil, signal, subprocess, tempfile
import xml.etree.ElementTree as ET
from svg_contract import bounded_regular_bytes, read_request, inspect_svg, ContractError, SVG, INK

HERE = pathlib.Path(__file__).resolve().parent
MASTER = "illustration.inkscape.svg"
def sha(b): return hashlib.sha256(b).hexdigest()
def raw(p): return bounded_regular_bytes(p, 16*1024*1024, str(p))
def exact(o, keys):
    if not isinstance(o,dict) or set(o)!=set(keys.split()):
        raise ContractError("unknown or missing fields")

TARGET_IMPORTANCE={"primary","required","support"}

def review_targets(plan, objects=None):
    """Validate and return the plan's explicit render-review targets.

    A target is a real drawable object that the author says must survive the
    final paint order.  It is deliberately a small, bounded claim: the adapter
    cannot decide whether a natural-language brief was fulfilled, but it can
    keep a declared focal or required object from being painted over entirely.
    """
    review=plan.get("review")
    exact(review,"targets")
    targets=review["targets"]
    if not isinstance(targets,list) or not 1<=len(targets)<=24:
        raise ContractError("1..24 review targets required")
    seen=set(); primary=0; result=[]
    for target in targets:
        exact(target,"id importance")
        ident=target["id"]
        importance=target["importance"]
        if not isinstance(ident,str) or not re.fullmatch(r"[a-z][a-z0-9-]{0,39}",ident):
            raise ContractError("invalid review target ID")
        if ident in seen: raise ContractError("duplicate review target ID")
        if importance not in TARGET_IMPORTANCE: raise ContractError("invalid review target importance")
        if objects is not None and ident not in objects:
            raise ContractError("review target is not a drawable object")
        seen.add(ident); primary += importance=="primary"
        result.append({"id":ident,"importance":importance})
    if primary!=1: raise ContractError("exactly one primary review target required")
    return result

def path_bounds(value):
    """Conservative bounds for the already-bounded absolute path grammar."""
    tokens=re.findall(r"[MLCQZ]|(?:\d+(?:\.\d+)?|\.\d+)",value)
    if not tokens or tokens[0]!="M": raise ContractError("path must start M")
    coords=[]; i=0
    while i<len(tokens):
        cmd=tokens[i]; i+=1
        if cmd not in {"M","L","C","Q","Z"}: raise ContractError("path command")
        count={"M":2,"L":2,"C":6,"Q":4,"Z":0}[cmd]
        if i+count>len(tokens): raise ContractError("path arity")
        nums=[]
        for raw_number in tokens[i:i+count]:
            try: nums.append(float(raw_number))
            except ValueError: raise ContractError("path coordinate")
        coords.extend(zip(nums[0::2],nums[1::2])); i+=count
    if not coords: raise ContractError("path has no coordinates")
    xs,ys=zip(*coords)
    return min(xs),min(ys),max(xs),max(ys)

def object_bounds(op):
    """Return conservative canvas bounds for a bounded plan operation.

    Text is intentionally not estimated: its host-dependent font metrics make
    a geometry-only visibility assertion dishonest.  The final export still
    outlines text and image review remains necessary.
    """
    kind=op["op"]
    if kind=="rect":
        return op["x"],op["y"],op["x"]+op["width"],op["y"]+op["height"]
    if kind=="ellipse":
        return op["cx"]-op["rx"],op["cy"]-op["ry"],op["cx"]+op["rx"],op["cy"]+op["ry"]
    if kind=="polygon":
        xs,ys=zip(*op["points"]); return min(xs),min(ys),max(xs),max(ys)
    if kind=="path": return path_bounds(op["d"])
    if kind=="text": return None
    raise ContractError("unknown operation")
def text(s):
    if not isinstance(s,str) or not 1<=len(s)<=500 or not re.fullmatch(r"[\w ,.!?()'’–—:+\-]+",s):
        raise ContractError("unsafe text: only plain words and punctuation permitted")
    if re.search(r"(?:[a-zA-Z][a-zA-Z0-9+.-]*:|\\.\\.)",s):
        raise ContractError("references are not plain text")
    return s
def num(n, maximum):
    if type(n) not in (int,float) or not math.isfinite(n) or not 0<=n<=maximum:
        raise ContractError("out-of-range geometry")
    return n
def load(p):
    def pairs(items):
        d={}
        for k,v in items:
            if k in d: raise ContractError("duplicate JSON key")
            d[k]=v
        return d
    try: return json.loads(raw(p),object_pairs_hook=pairs)
    except (ValueError,UnicodeError) as e: raise ContractError(str(e))
def dom(plan,w,h):
    exact(plan,"schema title description layers review")
    if plan["schema"]!="inkscape-plan/v1": raise ContractError("plan schema")
    ET.register_namespace("",SVG); ET.register_namespace("inkscape",INK)
    root=ET.Element("{%s}svg"%SVG,{"width":str(w),"height":str(h),"viewBox":f"0 0 {w} {h}","id":"drawing"})
    for tag,key in (("title","title"),("desc","description")):
        ET.SubElement(root,"{%s}%s"%(SVG,tag)).text=text(plan[key])
    layers=plan["layers"]
    if not isinstance(layers,list) or not 2<=len(layers)<=12: raise ContractError("2..12 layers required")
    ids={"drawing"}; labels=set(); colors=set(); count=0; objects={}
    def ident(s):
        if not isinstance(s,str) or not re.fullmatch(r"[a-z][a-z0-9-]{0,39}",s) or s in ids:
            raise ContractError("invalid/duplicate ID")
        ids.add(s); return s
    for layer in layers:
        exact(layer,"id name objects")
        label=text(layer["name"])
        if label in labels: raise ContractError("duplicate layer name")
        labels.add(label)
        g=ET.SubElement(root,"{%s}g"%SVG,{"id":ident(layer["id"]),"{%s}groupmode"%INK:"layer","{%s}label"%INK:label})
        if not isinstance(layer["objects"],list): raise ContractError("objects must be list")
        for op in layer["objects"]:
            count+=1
            if count>500: raise ContractError("too many operations")
            if not isinstance(op,dict): raise ContractError("object required")
            kind=op.get("op")
            shapes={"rect":"x y width height","ellipse":"cx cy rx ry","polygon":"points","path":"d","text":"x y text font_size"}
            if not isinstance(kind,str) or kind not in shapes: raise ContractError("unknown operation")
            exact(op,"op id fill stroke stroke_width opacity "+shapes[kind])
            object_id=ident(op["id"])
            objects[object_id]=op
            attrs={"id":object_id}
            for key in ("fill","stroke"):
                c=op[key]
                if not isinstance(c,str) or not re.fullmatch(r"none|#[0-9a-fA-F]{6}",c): raise ContractError("unsafe color")
                attrs[key]=c
                if c!="none": colors.add(c.lower())
            if len(colors)>32: raise ContractError("too many colors")
            attrs["stroke-width"]=str(num(op["stroke_width"],min(w,h)/10))
            attrs["opacity"]=str(num(op["opacity"],1))
            for key in shapes[kind].split():
                value=op[key]
                if key in ("x","cx","width","rx"): value=num(value,w)
                elif key in ("y","cy","height","ry"): value=num(value,h)
                elif key=="font_size":
                    attrs["font-size"]=str(num(value,min(w,h)/2)); attrs["font-family"]="sans-serif"; continue
                elif key=="text": continue
                elif key=="points":
                    if not isinstance(value,list) or not 3<=len(value)<=256: raise ContractError("polygon points")
                    for pt in value:
                        if not isinstance(pt,list) or len(pt)!=2: raise ContractError("point pair")
                        num(pt[0],w); num(pt[1],h)
                    value=" ".join(f"{x},{y}" for x,y in value)
                elif key=="d":
                    # Absolute M/L/C/Q/Z only, explicit command for every segment.
                    if not isinstance(value,str) or len(value)>16000: raise ContractError("path size")
                    tokens=re.findall(r"[MLCQZ]|(?:\d+(?:\.\d+)?|\.\d+)",value)
                    if re.sub(r"[MLCQZ]|(?:\d+(?:\.\d+)?|\.\d+)|[\s,]","",value): raise ContractError("path syntax")
                    if not tokens or tokens[0]!="M": raise ContractError("path must start M")
                    i=0; segments=0
                    while i<len(tokens):
                        cmd=tokens[i]; i+=1; segments+=1
                        if cmd not in {"M","L","C","Q","Z"} or segments>256: raise ContractError("path command")
                        n={"M":2,"L":2,"C":6,"Q":4,"Z":0}[cmd]
                        if i+n>len(tokens): raise ContractError("path arity")
                        for j in range(n):
                            try: num(float(tokens[i+j]),w if j%2==0 else h)
                            except ValueError: raise ContractError("path coordinate")
                        i+=n
                attrs[key]=str(value)
            if kind=="rect" and (op["width"]<=0 or op["height"]<=0 or op["x"]+op["width"]>w or op["y"]+op["height"]>h): raise ContractError("rect bounds")
            if kind=="ellipse" and (op["rx"]<=0 or op["ry"]<=0 or not op["rx"]<=op["cx"]<=w-op["rx"] or not op["ry"]<=op["cy"]<=h-op["ry"]): raise ContractError("ellipse bounds")
            elem=ET.SubElement(g,"{%s}%s"%(SVG,kind),attrs)
            if kind=="text": elem.text=text(op["text"])
    if not count: raise ContractError("empty plan")
    review_targets(plan,objects)
    return ET.tostring(root,encoding="utf-8",xml_declaration=True)

def executable():
    name=os.environ.get("INKSCAPE") or shutil.which("inkscape")
    if not name or not pathlib.Path(name).is_absolute() or not os.access(name,os.X_OK):
        raise ContractError("select installed absolute INKSCAPE executable")
    return str(pathlib.Path(name).resolve())
def source_digest():
    return sha(b"".join(p.name.encode()+b"\0"+raw(p) for p in
        (HERE/"author_in_inkscape",HERE/"authoring.py",HERE/"svg_contract.py")))
def invoke(argv,env,cwd):
    p=subprocess.Popen(argv,env=env,cwd=cwd,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
    timed=False
    try: out,err=p.communicate(timeout=120)
    except subprocess.TimeoutExpired:
        timed=True
        try: os.killpg(p.pid,signal.SIGKILL)
        except ProcessLookupError: pass
        out,err=p.communicate()
    record={"argv":argv,"exit_code":p.returncode,"timed_out":timed,
            "stdout":out.decode("utf-8","replace"),"stderr":err.decode("utf-8","replace")}
    if timed or p.returncode: raise ContractError("Inkscape authoring failed: "+str(record))
    return record

def generate(work,temp):
    _,w,h,request=read_request(work/"request.json")
    plan=raw(work/"output/inkscape-plan.json")
    document=dom(load(work/"output/inkscape-plan.json"),w,h)
    exe=executable()
    env=os.environ.copy()
    for key,name in (("INKSCAPE_PROFILE_DIR","profile"),("XDG_CONFIG_HOME","config"),("XDG_CACHE_HOME","cache"),("XDG_DATA_HOME","data"),("TMPDIR","tmp")):
        (temp/name).mkdir(); env[key]=str(temp/name)
    # All operands are adapter-owned; caller's plan supplies no filesystem names.
    source=temp/"constructed.svg"; target=temp/MASTER
    source.write_bytes(document)
    commands=[invoke([exe,"--version"],env,temp)]
    version=commands[0]["stdout"].strip()
    m=re.search(r"Inkscape (\d+)\.(\d+)",version)
    if not m or tuple(map(int,m.groups()))<(1,2): raise ContractError("Inkscape >=1.2 required")
    commands.append(invoke([exe,"--export-type=svg","--export-filename",str(target),str(source)],env,temp))
    # Strip only native editor UI metadata, retaining every vector/layer node.
    # Baseline conservative contract forbids foreign-namespace elements.
    native=ET.fromstring(raw(target))
    sod="http://sodipodi.sourceforge.net/DTD/sodipodi-0.dtd"
    for child in list(native):
        if child.tag=="{"+sod+"}namedview": native.remove(child)
    native.attrib.pop("{"+sod+"}docname",None)
    target.write_bytes(ET.tostring(native,encoding="utf-8",xml_declaration=True))
    inspect_svg(target,w,h,master=True)
    master=raw(target)
    receipt={"schema":"inkscape-authoring/v1","request_sha256":sha(request),"plan_sha256":sha(plan),
        "master_sha256":sha(master),"adapter_sha256":source_digest(),
        "inkscape":{"executable":exe,"sha256":sha(pathlib.Path(exe).read_bytes()),"version":version,"version_sha256":sha(version.encode())},
        "run_directory":str(temp),"commands":commands}
    return master,receipt
def verify(work):
    out=work/"output"
    receipt=load(out/"authoring-receipt.json")
    with tempfile.TemporaryDirectory(prefix="inkscape-author-check-") as td:
        master,expected=generate(work,pathlib.Path(td))
    if master!=raw(out/MASTER): raise ContractError("independently regenerated master differs")
    exact(receipt,"schema request_sha256 plan_sha256 master_sha256 adapter_sha256 inkscape run_directory commands")
    for key in expected:
        if key not in ("run_directory","commands") and receipt[key]!=expected[key]:
            raise ContractError("stale/forged authoring binding: "+key)
    rd=receipt["run_directory"]
    if not isinstance(rd,str) or not pathlib.Path(rd).is_absolute(): raise ContractError("invalid run directory")
    cmds=receipt["commands"]
    exe=expected["inkscape"]["executable"]
    argvs=[[exe,"--version"],[exe,"--export-type=svg","--export-filename",rd+"/"+MASTER,rd+"/constructed.svg"]]
    if not isinstance(cmds,list) or len(cmds)!=2: raise ContractError("authoring commands")
    for c,a in zip(cmds,argvs):
        exact(c,"argv exit_code timed_out stdout stderr")
        if c["argv"]!=a or type(c["exit_code"]) is not int or c["exit_code"]!=0 or c["timed_out"] is not False or not isinstance(c["stdout"],str) or not isinstance(c["stderr"],str):
            raise ContractError("invalid authoring outcome")
    if cmds[0]["stdout"].strip()!=expected["inkscape"]["version"]: raise ContractError("version outcome")
def author():
    work=pathlib.Path.cwd().resolve(); out=work/"output"
    if out.is_symlink() or not out.is_dir(): raise ContractError("output must be real directory")
    for name in (MASTER,"authoring-receipt.json"):
        if (out/name).is_symlink(): raise ContractError("symlink output")
    with tempfile.TemporaryDirectory(prefix="inkscape-author-") as td:
        master,receipt=generate(work,pathlib.Path(td))
    (out/MASTER).write_bytes(master)
    (out/"authoring-receipt.json").write_text(json.dumps(receipt,indent=2)+"\n")
