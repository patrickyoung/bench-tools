#!/usr/bin/env python3
"""Fixed-path file/binding contract shared by finish and check. No code loading."""
import hashlib, pathlib, re, xml.etree.ElementTree as ET
from svg_contract import (fail,bounded_regular_bytes,strict_json,read_request,dark_requested,local)
from selector import read_selector,obj,text,texts
BASE={"diagram.inkscape.svg","diagram.svg","diagram.png","layout.py","diagram-plan.json",
      "design-notes.md","render.log","render.json","result.json"}
DARK={"diagram-dark.inkscape.svg","diagram-dark.svg","diagram-dark.png"}
SUPPORT={"layout-inputs.json","thumbnail.png","grayscale.png","dark-thumbnail.png","dark-grayscale.png"}
CONFLICT={"conflicts.json","design-notes.md","result.json"}
ID="inkscape-decision-diagrammer"

def sha(p):
    return hashlib.sha256(bounded_regular_bytes(p,64*1024*1024,str(p))).hexdigest()
def load(p):
    return strict_json(bounded_regular_bytes(p,1048576,str(p)))
def output(root):
    out=root/"output"
    if out.is_symlink() or not out.is_dir(): fail("output must be real directory")
    return out
def inventory(out):
    paths=list(out.iterdir())
    if len(paths)>24: fail("output file-count bound exceeded")
    result=set(); total=0
    for p in paths:
        limit=1048576 if p.suffix==".json" or p.name in ("layout.py","design-notes.md") else 64*1024*1024
        data=bounded_regular_bytes(p,limit,"output "+p.name)
        total+=len(data); result.add(p.name)
    if total>200*1024*1024: fail("output total size bound exceeded")
    return result
def inputs(root):
    req,w,h,_=read_request(root/"request.json")
    sel=read_selector(root)
    paths=["request.json"]
    if sel is not None: paths+=["selector/"+n for n in ("diagram-brief.json","request.md","response.md")]
    return req,w,h,sel,{p:sha(root/p) for p in paths}
def bindings(root,names):
    return {n:sha(root/n) for n in sorted(names)}
def source_bindings(root,input_hashes):
    names=["output/diagram-plan.json","output/layout.py","output/design-notes.md"]
    if (root/"output/layout-inputs.json").exists(): names.append("output/layout-inputs.json")
    return dict(input_hashes,**bindings(root,names))
def bound_master(path,expected):
    root=ET.fromstring(bounded_regular_bytes(path,16*1024*1024,"master"))
    elements=[e for e in root if local(e.tag)=="desc" and e.get("id")=="source-bindings"]
    if len(elements)!=1 or strict_json((elements[0].text or "").encode())!=expected:
        fail("master source bindings stale/missing; regenerate geometry, do not patch receipt")
def notes(out,conflict=False):
    s=bounded_regular_bytes(out/"design-notes.md",1048576,"notes").decode("utf-8")
    headings=("Decision","Framework","Axes and regions","Items and evidence","Audience and medium",
              "Omissions","Style","Review")
    if not conflict and any("## "+k not in s for k in headings): fail("notes omit required sections")
    if not conflict and "visual review pending" not in s.lower(): fail("honest visual review pending required")
    if len(s.strip())<120: fail("notes too short")
    return s
def conflict_data(out,sel):
    c=load(out/"conflicts.json"); obj(c,"schema conflicts")
    if c["schema"]!=ID+".conflicts/v1" or not isinstance(c["conflicts"],list) or not 1<=len(c["conflicts"])<=20: fail("invalid conflict report")
    for v in c["conflicts"]:
        obj(v,"kind evidence question")
        if v["kind"] not in ("contradiction","missing-evidence","incompatible-framework","selector-needs-input"):
            fail("only diagnosed semantic conflicts allowed; tool failure is unfinished")
        text(v["evidence"]); text(v["question"])
    if sel and sel["status"]=="needs-input" and not any(v["kind"]=="selector-needs-input" for v in c["conflicts"]):
        fail("must identify selector intake status")
    n=notes(out,True)
    for v in c["conflicts"]:
        if v["evidence"] not in n or v["question"] not in n: fail("notes must state exact conflicts/questions")
    return c
def result(root,status,inputs_hashes,files):
    names=["output/"+n for n in files if n!="result.json"]
    return {"schema":ID+".result/v1","worker":ID,"status":status,
            "accepted_diagram":False,"visual_review":"pending" if status=="visual-review-pending" else "not-rendered",
            "inputs":inputs_hashes,"files":sorted(files),"bindings":bindings(root,names)}
def check_result(root,r,status,hashes,files):
    if r!=result(root,status,hashes,files): fail("stale/malformed result bindings/status")
def receipt_check(r,root,hashes,files,w,h,dark):
    obj(r,"schema inkscape requested_pixels commands bindings bounds")
    if r["schema"]!=ID+".render/v1" or r["requested_pixels"]!={"width":w,"height":h}: fail("render identity/dimensions")
    obj(r["inkscape"],"executable version")
    exe=r["inkscape"]["executable"]
    if not isinstance(exe,str) or not pathlib.Path(exe).is_absolute() or not re.search(r"Inkscape\s+\d+\.\d+",r["inkscape"]["version"]): fail("render tool identity")
    expected=dict(hashes,**bindings(root,["output/"+n for n in files-{"result.json","render.json"}]))
    if r["bindings"]!=expected: fail("stale render/source/artifact bindings")
    labels=["version","help"]
    for stem in ("diagram","diagram-dark") if dark else ("diagram",):
        labels += [stem+"-master-query",stem+"-export",stem+"-final-query",stem+"-render"]
        prefix="dark-" if stem.endswith("-dark") else ""
        previews={prefix+"thumbnail.png",prefix+"grayscale.png"}
        if previews & files:
            if not previews <= files: fail("support previews must be paired")
            labels.append(stem+"-thumbnail")
    if [v.get("label") for v in r["commands"]]!=labels: fail("native command sequence")
    for v in r["commands"]:
        obj(v,"label argv exit_code timed_out duration_seconds")
        if v["exit_code"]!=0 or v["timed_out"] is not False or not isinstance(v["argv"],list) or not all(isinstance(a,str) for a in v["argv"]) or v["argv"][0]!=exe:
            fail("unsuccessful/malformed native command")
        if "--batch-process" in v["argv"]: fail("batch-process forbidden")
        label=v["label"]; argv=v["argv"]
        if label in ("version","help") and argv!=[exe,"--"+label]: fail("probe argv")
        if label.endswith("-query") and (len(argv)!=3 or argv[1]!="--query-all"): fail("query argv")
        if label.endswith("-master-query") and argv[-1]!=str(root/"output"/(label[:-13]+".inkscape.svg")): fail("query source")
        if label.endswith("-export"):
            stem=label[:-7]
            if argv[1:6]!=["--export-type=svg","--export-plain-svg","--export-text-to-path","--export-area-page","--export-filename"] or len(argv)!=8 or argv[-1]!=str(root/"output"/(stem+".inkscape.svg")): fail("export argv")
        if label.endswith("-render"):
            if argv[1:8]!=["--export-type=png","--export-area-page","--export-width",str(w),"--export-height",str(h),"--export-filename"] or len(argv)!=10: fail("render argv")

    # Literal export/render paths must form a single chain, not unrelated successful calls.
    for stem in ("diagram","diagram-dark") if dark else ("diagram",):
        by={v["label"]:v["argv"] for v in r["commands"]}
        final=by[stem+"-export"][-2]
        if by[stem+"-final-query"][-1]!=final or by[stem+"-render"][-1]!=final: fail("receipt export/render chain")
        if stem+"-thumbnail" in by:
            a=by[stem+"-thumbnail"]; tw=min(320,w); th=max(1,round(h*tw/w))
            if len(a)!=10 or a[1:8]!=["--export-type=png","--export-area-page","--export-width",str(tw),"--export-height",str(th),"--export-filename"] or a[-1]!=final:
                fail("thumbnail argv")

def footer_check(path,req,plan):
    from plan_contract import provenance
    values=provenance(req,plan)
    root=ET.fromstring(bounded_regular_bytes(path,16*1024*1024,"master"))
    texts=["".join(e.itertext()) for e in root.iter() if local(e.tag)=="text"]
    # One text element (tspans allowed), not a second API-metadata disclaimer.
    candidates=[t for t in texts if re.search(r"\b(?:owner|date):",t,re.I)]
    if len(candidates)!=1: fail("one clean provenance footer required")
    footer=candidates[0]
    for field,value in values.items():
        if len(re.findall(r"\b"+field+r":",footer,re.I))!=1:
            fail("duplicate/missing provenance footer label")
        pattern=r"(?i:\b"+field+r":)\s*"+re.escape(value)+r"(?=\s*(?:[|·;]|$))"
        if not re.search(pattern,footer): fail("visible "+field+" footer missing or invented")
    if re.search(r"request\.json|\bJSON\b|top[- ]level|(?:missing|absent)\s+(?:request|field)|(?:request|field)\s+(?:missing|absent)",footer,re.I):
        fail("provenance footer must not expose implementation commentary")
