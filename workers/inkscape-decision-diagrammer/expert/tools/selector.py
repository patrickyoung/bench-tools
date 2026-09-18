#!/usr/bin/env python3
"""Strict portable selector handoff parsing. Source strings are never commands."""
import hashlib, re
from svg_contract import fail, strict_json, bounded_regular_bytes

NAMES = ["Wardley Mapping","Cynefin Framework","SWOT Analysis","Eisenhower Matrix"]
REGIONS = [
    [("genesis","Genesis","column-1"),("custom-built","Custom-built","column-2"),
     ("product","Product (+rental)","column-3"),("commodity","Commodity (+utility)","column-4")],
    [("complex","Complex","upper-left"),("complicated","Complicated","upper-right"),
     ("clear","Clear (Simple/Obvious)","lower-right"),("chaotic","Chaotic","lower-left"),
     ("confused","Confused/Disorder","center")],
    [("strengths","Strengths","upper-left"),("weaknesses","Weaknesses","upper-right"),
     ("opportunities","Opportunities","lower-left"),("threats","Threats","lower-right")],
    [("do","Do","upper-right"),("schedule","Schedule","upper-left"),
     ("delegate","Delegate","lower-right"),("eliminate-defer","Eliminate/Defer","lower-left")]]
AXES = [
    {"mode":"qualitative","x":{"label":"Evolution","direction":"left-to-right",
     "values":["Genesis","Custom-built","Product (+rental)","Commodity (+utility)"]},
     "y":{"label":"Visibility to the user","direction":"bottom-to-top","values":["Low","High"]}},
    {"mode":"no-numeric-axes"},
    {"mode":"categorical","x":{"label":"Effect","direction":"left-to-right","values":["Helpful","Harmful"]},
     "y":{"label":"Origin","direction":"top-to-bottom","values":["Internal","External"]}},
    {"mode":"qualitative","x":{"label":"Urgency","direction":"left-to-right","values":["Low","High"]},
     "y":{"label":"Importance","direction":"bottom-to-top","values":["Low","High"]}}]

def obj(v, keys):
    if not isinstance(v,dict) or set(v)!=set(keys.split()): fail("invalid fields: expected "+keys)
def text(v):
    if not isinstance(v,str) or not v.strip(): fail("expected nonblank string")
def texts(v, minimum=0, maximum=1000):
    if not isinstance(v,list) or not minimum <= len(v) <= maximum: fail("invalid string array")
    for s in v: text(s)
def boolean(v):
    if type(v) is not bool: fail("expected boolean")

def read_selector(root):
    folder=root/"selector"
    if not folder.exists() and not folder.is_symlink(): return None
    if folder.is_symlink() or not folder.is_dir(): fail("selector must be a real directory")
    if {p.name for p in folder.iterdir()} != {"diagram-brief.json","request.md","response.md"}:
        fail("selector requires exactly all three handoff files")
    raw={n:bounded_regular_bytes(folder/n,1048576 if n!="response.md" else 262144,n)
         for n in ("diagram-brief.json","request.md","response.md")}
    for v in raw.values(): v.decode("utf-8")
    s=strict_json(raw["diagram-brief.json"])
    obj(s,"schema request_sha256 response_sha256 framework status title audience decision uncertainty questions layout elements relationships legend traps review")
    if s["schema"]!="bench.diagram-brief/v1" or s["framework"] not in NAMES: fail("selector identity/framework")
    for field,name in (("request_sha256","request.md"),("response_sha256","response.md")):
        if s[field]!=hashlib.sha256(raw[name]).hexdigest(): fail("stale selector "+field)
    if s["status"] not in ("ready","provisional","needs-input"): fail("selector status")
    for k in ("title","audience","decision"): text(s[k])
    obj(s["uncertainty"],"assumptions unknowns")
    for v in s["uncertainty"].values(): texts(v)
    texts(s["questions"],0,3); texts(s["legend"],1); texts(s["traps"],1)
    obj(s["review"],"next_decision trigger scope transition")
    for v in s["review"].values(): text(v)
    obj(s["layout"],"orientation axes regions")
    i=NAMES.index(s["framework"])
    if s["layout"]["orientation"]!="landscape" or s["layout"]["axes"]!=AXES[i]: fail("selector axes")
    regions=s["layout"]["regions"]
    if not isinstance(regions,list) or len(regions)!=len(REGIONS[i]): fail("selector regions")
    seen=set()
    def ident(v):
        if not isinstance(v,str) or not re.fullmatch("[a-z][a-z0-9_-]{0,63}",v) or v in seen: fail("invalid/duplicate ID")
        seen.add(v)
    for r,expected in zip(regions,REGIONS[i]):
        obj(r,"id label position definition"); ident(r["id"]); text(r["definition"])
        if tuple(r[k] for k in ("id","label","position"))!=expected: fail("selector literal region changed")
    region_ids={r["id"] for r in regions}
    if not isinstance(s["elements"],list) or len(s["elements"])>500: fail("selector element bound")
    elements={}
    for e in s["elements"]:
        obj(e,"id label role placement basis evidence reason uncertain"); ident(e["id"])
        for k in ("label","evidence","reason"): text(e[k])
        boolean(e["uncertain"])
        if e["basis"] not in ("supplied","inferred","proposed","unknown"): fail("selector basis")
        if e["basis"]=="unknown" and not e["uncertain"]: fail("unknown must be uncertain")
        if e["role"] not in (("user","need","component") if i==0 else ("item",)): fail("selector role")
        obj(e["placement"],"region visibility" if i==0 else "region")
        r=e["placement"]["region"]
        if r is not None and r not in region_ids: fail("unknown region")
        if r is None and not (i==0 and e["role"] in ("user","need")) and not e["uncertain"]:
            fail("unplaced must be uncertain")
        if i==0:
            v=e["placement"]["visibility"]
            if v not in ("high","medium","low","unknown") or (v=="unknown" and not e["uncertain"]):
                fail("selector visibility")
        elements[e["id"]]=e
    if i==0 and elements and not {"user","need"} <= {e["role"] for e in elements.values()}:
        fail("Wardley requires user and need anchors")
    if not isinstance(s["relationships"],list) or len(s["relationships"])>1000: fail("relationship bound")
    for r in s["relationships"]:
        obj(r,"id from to type direction meaning style uncertain timing"); ident(r["id"]); text(r["meaning"]); boolean(r["uncertain"])
        if r["from"] not in elements or r["from"]==r["to"]: fail("relationship source/self")
        if r["direction"] not in ("source-to-target","undirected") or r["style"] not in ("solid","dashed"): fail("relationship style")
        if r["type"]=="movement":
            if i!=0 or elements[r["from"]]["role"]!="component" or r["to"] not in region_ids or elements[r["from"]]["placement"]["region"]==r["to"]:
                fail("invalid movement endpoints")
            if (r["direction"],r["style"],r["uncertain"],r["timing"])!=("source-to-target","dashed",True,"uncertain"): fail("movement semantics")
        else:
            if r["to"] not in elements or r["timing"] is not None: fail("relationship target/timing")
            if r["type"]=="dependency":
                if i!=0 or r["direction"]!="source-to-target" or r["style"]!="solid": fail("dependency semantics")
            elif r["type"]!="association": fail("relationship type")
    if s["status"]=="needs-input":
        if elements or s["relationships"] or not s["uncertainty"]["unknowns"] or not s["questions"]: fail("invalid selector needs-input")
    elif s["status"]=="ready" and not elements: fail("empty ready selector")
    # The raw response is bound evidence, not an executable or an instruction.
    # Its editorial quality is the selector's responsibility; no inference from prose.
    return s
