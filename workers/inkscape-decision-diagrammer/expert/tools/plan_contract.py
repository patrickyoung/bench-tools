#!/usr/bin/env python3
"""Inspectable geometry and evidence checks, not a layout engine."""
import math, re
import xml.etree.ElementTree as ET
from collections import Counter
from svg_contract import fail, local, bounded_regular_bytes, strict_json, INK
from selector import obj, text, texts, boolean
FRAMEWORKS = {
 "Wardley Mapping": (25, None), "SWOT Analysis": (24,6),
 "Eisenhower Matrix": (28,7), "Impact–effort": (15,None),
 "Risk heatmap": (18,None), "Power–interest grid": (16,None),
 "BCG growth–share": (12,None), "Kano": (14,None), "MoSCoW": (40,10),
 "Force-field": (14,7), "Decision tree": (64,None),
 "Options × criteria": (40,None), "RACI / DACI": (12,None),
 "Cynefin Framework": (25,5), "Now / Next / Later": (24,8),
 "Porter five forces": (20,4), "Pre-mortem / fishbone": (20,4)}

def num(v):
    if type(v) not in (int,float) or not math.isfinite(v): fail("expected finite number")
    return v
def close(a,b):
    if not math.isclose(num(a),num(b),rel_tol=1e-7,abs_tol=1e-7): fail("framework arithmetic mismatch")
def box(b):
    if not isinstance(b,list) or len(b)!=4: fail("box must be [x,y,width,height]")
    for n in b: num(n)
    if b[2]<=0 or b[3]<=0: fail("box must have positive extent")
def inside(a,b,pad=0,tol=.15):
    return a[0]>=b[0]+pad-tol and a[1]>=b[1]+pad-tol and a[0]+a[2]<=b[0]+b[2]-pad+tol and a[1]+a[3]<=b[1]+b[3]-pad+tol

def arithmetic(p):
    f=p["framework"]; m=p["math"]
    if not isinstance(m,dict): fail("math must be an object")
    required={"Decision tree":"tree","BCG growth–share":"bubbles",
              "Options × criteria":"matrix","MoSCoW":"scope","RACI / DACI":"accountability",
              "Force-field":"forces"}
    if f in required and required[f] not in m: fail("missing framework arithmetic: "+required[f])
    if set(m)-{"tree","bubbles","matrix","scope","accountability","forces"}: fail("unknown math fields")
    ids={i["id"] for i in p["items"]}
    if "tree" in m:
        t=m["tree"]; obj(t,"root nodes")
        nodes={n["id"]:n for n in t["nodes"]}
        if len(nodes)!=len(t["nodes"]) or set(nodes)!=ids or t["root"] not in nodes: fail("tree node coverage")
        visiting=set(); visited=set(); leaves=[]; parents=Counter()
        def walk(k,depth):
            if depth>4 or k in visiting or k in visited: fail("tree depth/cycle/shared child")
            visiting.add(k); visited.add(k); n=nodes[k]
            obj(n,"id kind branches value evidence"); text(n["evidence"])
            if n["kind"] not in ("decision","chance","terminal"): fail("tree node kind")
            if n["kind"]=="terminal":
                if n["branches"]: fail("terminal branches")
                leaves.append(k)
                if n["value"] is not None: num(n["value"])
            else:
                if not n["branches"]: fail("empty internal tree node")
                ev=0; prob=0; values=[]
                for b in n["branches"]:
                    obj(b,"to probability evidence"); text(b["evidence"])
                    if b["to"] not in nodes: fail("missing tree child")
                    parents[b["to"]]+=1
                    v=walk(b["to"],depth+1); values.append(v)
                    if n["kind"]=="chance":
                        q=num(b["probability"])
                        if not 0<=q<=1: fail("probability outside [0,1]")
                        prob+=q
                        if v is not None: ev+=q*v
                    elif b["probability"] is not None: fail("decision has probability")
                if n["kind"]=="chance":
                    close(prob,1)
                    if n["value"] is not None:
                        if any(v is None for v in values): fail("EV requires all payoffs")
                        close(n["value"],ev)
                elif n["value"] is not None:
                    fail("decision node value must be null; explain chosen policy separately")
            visiting.remove(k)
            return n["value"]
        walk(t["root"],1)
        if visited!=set(nodes) or any(v!=1 for v in parents.values()) or len(leaves)>16: fail("not a bounded tree")
    if "bubbles" in m:
        b=m["bubbles"]; obj(b,"unit denominator growth_threshold share_threshold scale entries")
        for k in ("unit","denominator"): text(b[k])
        for k in ("growth_threshold","share_threshold","scale"): num(b[k])
        if b["scale"]<=0: fail("bubble area scale")
        if {e["id"] for e in b["entries"]}!=ids or len(b["entries"])!=len(ids): fail("bubble coverage")
        for e in b["entries"]:
            obj(e,"id revenue radius share growth category evidence"); text(e["evidence"])
            if num(e["revenue"])<=0 or num(e["radius"])<=0: fail("positive bubble quantities required")
            close(math.pi*e["radius"]**2,b["scale"]*e["revenue"])
            hi=num(e["share"])>=b["share_threshold"]; growth=num(e["growth"])>=b["growth_threshold"]
            expected=("stars" if hi else "question-marks") if growth else ("cash-cows" if hi else "dogs")
            if e["category"]!=expected: fail("BCG category")
            if next(i for i in p["items"] if i["id"]==e["id"])["region"]!=expected: fail("BCG placement")
    if "matrix" in m:
        a=m["matrix"]; obj(a,"options criteria totals score_min score_max weight_basis")
        text(a["weight_basis"])
        lo=num(a["score_min"]); hi=num(a["score_max"])
        if hi<=lo or not 1<=len(a["options"])<=5 or len(set(a["options"]))!=len(a["options"]) or not 1<=len(a["criteria"])<=8: fail("matrix bounds")
        total=[0]*len(a["options"]); weights=0
        for c in a["criteria"]:
            obj(c,"label weight scores weighted evidence"); text(c["label"]); text(c["evidence"])
            w=num(c["weight"]); weights+=w
            if w<0 or len(c["scores"])!=len(total) or len(c["weighted"])!=len(total): fail("matrix dimensions/weight")
            for j,s in enumerate(c["scores"]):
                if not lo<=num(s)<=hi: fail("matrix scoring scale")
                close(c["weighted"][j],w*s); total[j]+=w*s
        if weights<=0 or len(a["totals"])!=len(total): fail("matrix totals")
        for a,b in zip(a["totals"],total): close(a,b)
    if "scope" in m:
        a=m["scope"]; obj(a,"unit entries bands total_count total_effort"); text(a["unit"])
        counts=Counter(); efforts=Counter()
        if {e["id"] for e in a["entries"]}!=ids or len(a["entries"])!=len(ids): fail("scope coverage")
        for e in a["entries"]:
            obj(e,"id band effort evidence"); text(e["evidence"])
            if e["band"] not in ("must","should","could","wont") or num(e["effort"])<0: fail("scope entry")
            if next(i for i in p["items"] if i["id"]==e["id"])["region"]!=e["band"]: fail("scope placement")
            counts[e["band"]]+=1; efforts[e["band"]]+=e["effort"]
        if set(a["bands"])!={"must","should","could","wont"}: fail("scope bands")
        for k,b in a["bands"].items():
            obj(b,"count effort"); close(b["count"],counts[k]); close(b["effort"],efforts[k])
            if counts[k]>10: fail("scope density")
        close(a["total_count"],sum(counts.values())); close(a["total_effort"],sum(efforts.values()))
    if "accountability" in m:
        a=m["accountability"]; obj(a,"mode roles rows")
        if a["mode"] not in ("RACI","DACI") or not a["roles"] or len(set(a["roles"]))!=len(a["roles"]): fail("accountability roles")
        texts(a["roles"],1,20)
        if not 1<=len(a["rows"])<=12: fail("accountability density")
        for r in a["rows"]:
            obj(r,"id cells evidence"); text(r["evidence"])
            if len(r["cells"])!=len(a["roles"]) or r["cells"].count("A")!=1: fail("exactly one A per row required")
            if any(v not in set(a["mode"])|{""} for v in r["cells"]): fail("invalid responsibility")
        if {r["id"] for r in a["rows"]}!=ids or len(a["rows"])!=len(ids): fail("accountability row coverage")
    if "forces" in m:
        a=m["forces"]; obj(a,"scale unit entries"); text(a["unit"])
        if num(a["scale"])<=0: fail("force scale")
        if {e["id"] for e in a["entries"]}!=ids: fail("force coverage")
        for e in a["entries"]:
            obj(e,"id weight extent evidence"); text(e["evidence"])
            if num(e["weight"])<=0: fail("force weight")
            close(e["extent"],e["weight"]*a["scale"])

def provenance(request, plan):
    """Resolve only bound claims; free-form evidence semantics remain a review task."""
    metadata=plan.get("metadata")
    if "metadata" in plan:
        obj(metadata,"owner date")
    resolved={}
    for field in ("owner","date"):
        fallback=request.get(field,"unspecified")
        if metadata is None:
            resolved[field]=fallback
            continue
        row=metadata[field]; obj(row,"value source evidence")
        value=row["value"]; source=row["source"]; quote=row["evidence"]
        if not isinstance(value,str) or not value.strip() or len(value)>2000:
            fail("metadata value must be a nonblank bounded string")
        if not isinstance(source,str) or source not in ("structured","brief","unspecified"):
            fail("metadata source must be structured, brief or unspecified")
        if field in request:
            if source!="structured" or value!=request[field] or quote is not None:
                fail("metadata must preserve structured "+field)
        elif source=="brief":
            if not isinstance(quote,str) or not quote.strip() or len(quote)>48000:
                fail("metadata brief evidence must be a nonblank bounded quote")
            if quote not in request["brief"] or value not in quote:
                fail("metadata brief evidence/value unsupported by current brief")
        elif source!="unspecified" or value!="unspecified" or quote is not None:
            fail("metadata without evidence must be unspecified")
        resolved[field]=value
    return resolved

def read_plan(out, request, width, height, selector):
    p=strict_json(bounded_regular_bytes(out/"diagram-plan.json",1048576,"plan"))
    obj(p,"schema framework grammar decision audience axes regions tray visibility_bands items relationships omissions selector math geometry style"+(" metadata" if "metadata" in p else ""))
    provenance(request,p)
    if p["schema"]!="inkscape-decision-diagrammer.plan/v1": fail("plan identity")
    for k in ("framework","grammar","decision","audience","axes"): text(p[k])
    if p["selector"]!=selector: fail("plan must preserve complete selector verbatim as data")
    if selector and (selector["status"]=="needs-input" or p["framework"]!=selector["framework"]): fail("selector not renderable/framework conflict")
    if not isinstance(p["items"],list) or not p["items"]: fail("plan requires items")
    limit,per=FRAMEWORKS.get(p["framework"],(25,None))
    if len(p["items"])>limit: fail("framework density ceiling exceeded")
    if not isinstance(p["regions"],list) or len(p["regions"])>25: fail("region bound")
    regions={}
    for r in p["regions"]:
        obj(r,"id label box meaning"); text(r["id"]); text(r["label"]); text(r["meaning"]); box(r["box"])
        if r["id"] in regions: fail("duplicate region")
        regions[r["id"]]=r
    if p["tray"] is not None: box(p["tray"])
    items={}; source_seen=set()
    for i in p["items"]:
        obj(i,"id label source_ids evidence assumption region visibility x y group mark label_id")
        for k in ("id","label","evidence","group","mark","label_id"): text(i[k])
        if not re.fullmatch("[a-z][a-z0-9_-]{0,63}",i["id"]) or i["id"] in items: fail("invalid item ID")
        texts(i["source_ids"],1); boolean(i["assumption"]); num(i["x"]); num(i["y"])
        if source_seen.intersection(i["source_ids"]): fail("duplicate source membership")
        source_seen.update(i["source_ids"])
        if i["region"] is None:
            if p["tray"] is None: fail("unplaced requires tray")
            b=p["tray"]
        else:
            if i["region"] not in regions: fail("item region missing")
            b=regions[i["region"]]["box"]
        if not inside([i["x"],i["y"],.001,.001],b): fail("item outside declared region/tray")
        items[i["id"]]=i
    counts=Counter(i["region"] for i in items.values())
    if per and any(n>per for n in counts.values()): fail("region density exceeded")
    if not isinstance(p["omissions"],list): fail("omissions must be list")
    for o in p["omissions"]:
        obj(o,"source_ids reason"); texts(o["source_ids"],1); text(o["reason"])
        if source_seen.intersection(o["source_ids"]): fail("source both shown and omitted")
        source_seen.update(o["source_ids"])
    if selector:
        src={e["id"]:e for e in selector["elements"]}
        if source_seen!=set(src): fail("selector source coverage")
        if [r["id"] for r in p["regions"]]!=[r["id"] for r in selector["layout"]["regions"]]: fail("preserve empty selector regions")
        for r,sr in zip(p["regions"],selector["layout"]["regions"]):
            if r["label"]!=sr["label"]: fail("selector region label changed")
        proposed={r["id"]:r["box"] for r in p["regions"]}
        for ra in selector["layout"]["regions"]:
            for rb in selector["layout"]["regions"]:
                a,b=proposed[ra["id"]],proposed[rb["id"]]
                pa,pb=ra["position"],rb["position"]
                if pa.endswith("left") and pb.endswith("right") and a[0]+a[2]/2>=b[0]+b[2]/2: fail("selector left/right region order")
                if pa.startswith("upper") and pb.startswith("lower") and a[1]+a[3]/2>=b[1]+b[3]/2: fail("selector upper/lower region order")
                if pa.startswith("column-") and pb.startswith("column-") and pa<pb and a[0]+a[2]/2>=b[0]+b[2]/2: fail("selector evolution order")
        for i in items.values():
            for k in i["source_ids"]:
                e=src[k]
                if i["region"]!=e["placement"]["region"]: fail("selector placement changed")
                if len(i["source_ids"])==1 and i["label"]!=e["label"]: fail("selector literal label changed")
                if p["framework"]=="Wardley Mapping" and i["visibility"]!=e["placement"]["visibility"]: fail("selector visibility changed")
            if len(i["source_ids"])>1 and any(r["from"] in i["source_ids"] or r["to"] in i["source_ids"] for r in selector["relationships"]):
                fail("cannot cluster connected selector items; propose second diagram")
        if p["relationships"]!=selector["relationships"]: fail("selector relationships changed")
    if not isinstance(p["relationships"],list) or len(p["relationships"])>200: fail("relationship bounds")
    mapping={s:i for i in items.values() for s in i["source_ids"]}
    edges=[]
    for r in p["relationships"]:
        obj(r,"id from to type direction meaning style uncertain timing"); text(r["meaning"]); boolean(r["uncertain"])
        if r["from"] not in mapping: fail("relationship source omitted")
        if r["type"] not in ("dependency","movement","association"): fail("relationship type")
        if r["type"]!="movement" and r["to"] not in mapping: fail("relationship target omitted")
        if r["type"]=="dependency":
            if p["framework"]!="Wardley Mapping" or r["style"]!="solid" or r["direction"]!="source-to-target": fail("dependency convention")
            a,b=mapping[r["from"]],mapping[r["to"]]
            if a["y"]>=b["y"]: fail("Wardley dependencies must point downward")
            edges.append((a["id"],b["id"]))
        if r["type"]=="movement":
            if p["framework"]!="Wardley Mapping" or r["style"]!="dashed" or r["to"] not in regions or r["timing"]!="uncertain" or not r["uncertain"]: fail("movement convention")
    if p["framework"]=="Wardley Mapping":
        bands=p["visibility_bands"]
        obj(bands,"high medium low")
        for b in bands.values(): box(b)
        if not (bands["high"][1]+bands["high"][3]<=bands["medium"][1] and bands["medium"][1]+bands["medium"][3]<=bands["low"][1]): fail("visibility band order")
        for i in items.values():
            if i["visibility"] not in (*bands,"unknown"): fail("visibility category")
            if i["visibility"]!="unknown" and not inside([i["x"],i["y"],.001,.001],bands[i["visibility"]]): fail("visibility band violated")
        def depth(k,seen):
            if k in seen: fail("dependency cycle")
            return 1+max([depth(b,seen|{k}) for a,b in edges if a==k] or [0])
        if max(depth(k,set()) for k in items)>4: fail("Wardley chain depth exceeds four")
    elif p["visibility_bands"]!={}: fail("visibility bands reserved for Wardley")
    if p["framework"]=="Pre-mortem / fishbone" and len(regions)>5: fail("fishbone at most five cause classes")
    if p["framework"]=="Cynefin Framework" and set(regions)!={"complex","complicated","clear","chaotic","confused"}:
        fail("Cynefin requires four domains plus central unresolved area")
    if p["framework"]=="BCG growth–share":
        if set(regions)!={"stars","cash-cows","question-marks","dogs"}: fail("BCG categories")
        center=lambda k:(regions[k]["box"][0]+regions[k]["box"][2]/2, regions[k]["box"][1]+regions[k]["box"][3]/2)
        if not (center("stars")[0]<center("question-marks")[0] and center("cash-cows")[0]<center("dogs")[0]
                and center("stars")[1]<center("cash-cows")[1] and center("question-marks")[1]<center("dogs")[1]):
            fail("BCG high share left, high growth above")
    arithmetic(p)
    obj(p["geometry"],"anchors contains disjoint overlaps contrast")
    for a in p["geometry"]["anchors"]:
        obj(a,"id x y"); text(a["id"])
        for k in ("x","y"):
            if abs(num(a[k])/8-round(a[k]/8))>1e-7: fail("structural anchors must use 8-unit grid")
    for c in p["geometry"]["contains"]:
        obj(c,"inner outer padding"); text(c["inner"]); text(c["outer"])
        if num(c["padding"])<0: fail("containment padding")
    for pair in p["geometry"]["disjoint"]:
        if not isinstance(pair,list) or len(pair)!=2 or pair[0]==pair[1]: fail("disjoint pair")
    for o in p["geometry"]["overlaps"]:
        obj(o,"ids reason"); texts(o["ids"],2,2); text(o["reason"])
    for c in p["geometry"]["contrast"]:
        obj(c,"foreground background paint")
        if c["paint"] not in ("fill","stroke"): fail("contrast paint must be fill or stroke")
        text(c["foreground"]); text(c["background"])
    obj(p["style"],"font sizes weights margin tokens semantic_channels")
    text(p["style"]["font"]); texts(p["style"]["semantic_channels"])
    sizes=p["style"]["sizes"]; weights=p["style"]["weights"]
    if not 1<=len(sizes)<=3 or any(num(n)<11 for n in sizes) or not 1<=len(weights)<=2: fail("type limits")
    if num(p["style"]["margin"])<48*min(width/1600,height/1000): fail("proportional margin")
    if not isinstance(p["style"]["tokens"],dict) or not p["style"]["tokens"]: fail("semantic tokens required")
    for name,v in p["style"]["tokens"].items():
        text(name); obj(v,"light dark")
        if any(not isinstance(c,str) or not re.fullmatch("#[0-9a-fA-F]{6}",c) for c in v.values()):
            fail("tokens must be paired #rrggbb values")
    return p

def attrs(e):
    a=dict(e.attrib)
    for d in a.pop("style","").split(";"):
        if ":" in d:
            k,v=d.split(":",1); a[k.strip()]=v.strip()
    return a

def master_plan(path,p):
    root=ET.fromstring(bounded_regular_bytes(path,16*1024*1024,"master"))
    ids={e.get("id"):e for e in root.iter() if e.get("id")}
    parents={c:e for e in root.iter() for c in e}
    marks=next(e for e in root if e.get("{%s}label"%INK)=="marks")
    text_ids=set()
    for e in root.iter():
        if local(e.tag)=="text":
            if not e.get("id"): fail("all live labels need IDs")
            text_ids.add(e.get("id"))
    for i in p["items"]:
        try: g,mark,label=(ids[i[k]] for k in ("group","mark","label_id"))
        except KeyError: fail("missing item geometry/live label")
        if parents.get(g)!=marks or mark not in list(g.iter()) or label not in list(g.iter()) or local(label.tag)!="text":
            fail("logical marks must group live label with mark")
        if " ".join("".join(label.itertext()).split())!=" ".join(i["label"].split()): fail("live label differs from plan")
    allowed_sizes=set(map(float,p["style"]["sizes"])); allowed_weights=set(map(str,p["style"]["weights"]))
    rotations=[]
    def walk(e,inherited):
        own=attrs(e)
        if own.get("transform"):
            if local(e.tag)!="text" or not re.fullmatch(r"rotate\(\s*-?90(?:[ ,]+[+-]?[0-9.]+){0,2}\s*\)",own["transform"]):
                fail("master transforms reserved for one rotated Y-axis title; compute coordinates")
            rotations.append(e)
        if local(e.tag) in ("g","svg") and float(own.get("opacity",1))!=1:
            fail("use per-paint opacity, not group compositing")
        a=inherited.copy(); a.update(own)
        if local(e.tag) in ("text","tspan"):
            try: size=float(a.get("font-size","").removesuffix("px"))
            except ValueError: fail("explicit numeric font size required")
            if size not in allowed_sizes or a.get("font-weight","400") not in allowed_weights: fail("actual type limits")
            if a.get("font-family")!=p["style"]["font"] or len(a["font-family"].split(","))!=2 or a["font-family"].split(",")[-1].strip() not in ("sans-serif","serif","monospace"): fail("one family plus generic fallback")
            if a.get("font-style","normal")!="normal" or a.get("text-decoration","none")!="none": fail("italic/underline forbidden")
        for c in e: walk(c,a)
    walk(root,{})
    if len(rotations)>1: fail("only one rotated Y title allowed")
    for b in p["math"].get("bubbles",{}).get("entries",[]):
        mark=ids[next(i["mark"] for i in p["items"] if i["id"]==b["id"])]
        if local(mark.tag)!="circle": fail("bubble must be circle")
        close(float(mark.get("r","nan")),b["radius"])
    # Relationships have one actual connector path with the semantic ID.
    mapping={s:i for i in p["items"] for s in i["source_ids"]}
    for r in p["relationships"]:
        e=ids.get(r["id"])
        if e is None or local(e.tag)!="path": fail("missing relationship path: "+r["id"])
        a=attrs(e)
        if r["direction"]=="source-to-target" and not a.get("marker-end","").startswith("url(#"):
            fail("directed relationship needs local arrowhead at target")
        if r["style"]=="dashed" and a.get("stroke-dasharray","none")=="none": fail("dashed cue missing")
        if r["style"]=="solid" and a.get("stroke-dasharray","none")!="none": fail("solid cue changed")
        if r["type"]=="dependency":
            # Restrict current Wardley dependency paths to absolute M/L/Q/C segments.
            # Endpoints attach to mark boundary, preserving source->target downward meaning.
            d=e.get("d","")
            if re.search(r"[A-Za-z]",re.sub(r"[MLQC]", "", re.sub(r"[eE][+-]?\d+","",d))):
                fail("Wardley dependencies require absolute M/L/Q/C geometry")
            values=[float(v) for v in re.findall(r"[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?",d)]
            if len(values)<4 or values[1]>=values[-1]: fail("dependency path must point downward")
            # For final L/Q/C, the preceding point/control point determines the
            # endpoint tangent. A lower target alone permits a sideways arrow.
            if values[-3]>=values[-1]: fail("dependency arrowhead must point downward")
            for node,point in ((mapping[r["from"]],values[:2]),(mapping[r["to"]],values[-2:])):
                mark=ids[node["mark"]]; ma=attrs(mark)
                if local(mark.tag)=="rect":
                    b=[float(ma[k]) for k in ("x","y","width","height")]
                elif local(mark.tag)=="circle":
                    cx,cy,rr=(float(ma[k]) for k in ("cx","cy","r")); b=[cx-rr,cy-rr,2*rr,2*rr]
                else: fail("Wardley dependency marks use rect or circle for verifiable attachment")
                if not inside([point[0],point[1],0,0],b,tol=2): fail("dependency not attached to endpoint")
    return ids,text_ids

def bounds_check(query,p,ids,text_ids,width,height):
    # Every returned native bound is checked, not just IDs chosen by the author.
    for k,b in query.items():
        if not inside(b,[0,0,width,height],tol=.1): fail("native object outside page: "+k)
    for i in p["items"]:
        if i["mark"] not in query or i["label_id"] not in query: fail("missing native item bounds")
        b=query[i["mark"]]
        if abs(b[0]+b[2]/2-i["x"])>.2 or abs(b[1]+b[3]/2-i["y"])>.2: fail("plan/mark center mismatch: "+i["id"])
    for k in text_ids:
        if k not in query: fail("missing native text bounds: "+k)
    minimum_margin=48*min(width/1600,height/1000)
    inset=[minimum_margin,minimum_margin,width-2*minimum_margin,height-2*minimum_margin]
    for k in text_ids|{i["mark"] for i in p["items"]}:
        if not inside(query[k],inset,tol=.1):
            fail("visible content violates minimum outer margin: "+k)
    def get(k):
        if k not in query: fail("declared geometry not queried: "+k)
        return query[k]
    for c in p["geometry"]["contains"]:
        if not inside(get(c["inner"]),get(c["outer"]),c["padding"]): fail("native card overflow: "+c["inner"])
    exempt={frozenset(o["ids"]) for o in p["geometry"]["overlaps"]}
    pairs={tuple(sorted((a,b))) for a in text_ids for b in text_ids if a<b}
    pairs.update(tuple(sorted(x)) for x in p["geometry"]["disjoint"])
    for a,b in pairs:
        if frozenset((a,b)) in exempt: continue
        x,y=get(a),get(b)
        if min(x[0]+x[2],y[0]+y[2])-max(x[0],y[0])>.1 and min(x[1]+x[3],y[1]+y[3])-max(x[1],y[1])>.1:
            fail("native label/declared collision: "+a+" / "+b)

    contrast_check(p,ids,text_ids,query)

def contrast_check(p,ids,text_ids,query):
    parents={c:e for e in ids.values() for c in e}
    def effective(e):
        chain=[]; seen=set()
        while e is not None and e not in seen:
            seen.add(e); chain.append(e); e=parents.get(e)
        a={}
        for e in reversed(chain): a.update(attrs(e))
        return a
    def rgb(c):
        if not isinstance(c,str) or not re.fullmatch("#[0-9a-fA-F]{6}",c):
            fail("contrast uses explicit #rrggbb semantic tokens")
        return [int(c[j:j+2],16)/255 for j in (1,3,5)]
    def luminance(c):
        v=[n/12.92 if n<=.04045 else ((n+.055)/1.055)**2.4 for n in c]
        return sum(x*y for x,y in zip(v,(.2126,.7152,.0722)))
    covered=set()
    for c in p["geometry"]["contrast"]:
        f,b=c["foreground"],c["background"]
        if f not in ids or b not in ids or f not in query or b not in query: fail("contrast ID missing")
        fa=effective(ids[f]); ba=effective(ids[b])
        # Flat opaque rectangle surfaces make the actual background independently checkable.
        if local(ids[b].tag)!="rect" or not inside(query[f],query[b],tol=1): fail("contrast surface must contain object")
        if float(ba.get("opacity",1))!=1 or float(ba.get("fill-opacity",1))!=1: fail("contrast background must be opaque")
        bg=rgb(ba.get("fill")); fg=rgb(fa.get(c["paint"]))
        alpha=float(fa.get("opacity",1))*float(fa.get(c["paint"]+"-opacity",1))
        if not 0<=alpha<=1: fail("paint opacity range")
        actual=[x*alpha+y*(1-alpha) for x,y in zip(fg,bg)]
        lo,hi=sorted((luminance(actual),luminance(bg)))
        if (hi+.05)/(lo+.05) < (4.5 if f in text_ids else 3)-1e-9: fail("insufficient actual paint contrast: "+f)
        # Reject an intervening opaque rectangular surface with different paint.
        ordered=list(ids)
        if ordered.index(b)>=ordered.index(f): fail("background must precede foreground")
        for k in ordered[ordered.index(b)+1:ordered.index(f)]:
            if k in query and local(ids[k].tag)=="rect" and inside(query[f],query[k],tol=0):
                a=effective(ids[k])
                if a.get("fill","none")!="none" and a.get("fill")!=ba.get("fill"):
                    fail("contrast declaration ignores intervening surface")
        covered.add(f)
    if not text_ids|{i["mark"] for i in p["items"]} <= covered:
        fail("contrast must cover every live text and meaningful mark")

def geometry_signature(path):
    root=ET.fromstring(bounded_regular_bytes(path,16*1024*1024,"master"))
    # Only semantic paint tokens may differ, not text, typography, transform or geometry.
    paint={"fill","stroke","color","stop-color","fill-opacity","stroke-opacity","opacity","stop-opacity"}
    return [(e.tag, sorted((k,v) for k,v in attrs(e).items() if k not in paint),
             (e.text or "").strip(),(e.tail or "").strip()) for e in root.iter()]
