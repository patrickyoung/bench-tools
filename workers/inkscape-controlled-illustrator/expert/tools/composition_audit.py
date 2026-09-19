#!/usr/bin/env python3
"""Trusted, bounded visibility audit for the Inkscape illustration plan.

This is intentionally a geometry check, not an image judge.  It catches a
specific high-value failure mode: a declared focal or required object becomes
entirely hidden beneath a later opaque rectangular panel.  It cannot establish
semantic fidelity, beauty, or the visibility of text and irregular overlap.
"""
import hashlib, json, pathlib, sys
sys.dont_write_bytecode = True

HERE=pathlib.Path(__file__).resolve().parent
sys.path.insert(0,str(HERE))
import authoring
from svg_contract import ContractError, bounded_regular_bytes, read_request

AUDIT_NAME="composition-audit.json"
MAX_AUDIT=1024*1024

def digest(path,limit=64*1024*1024):
    return hashlib.sha256(bounded_regular_bytes(path,limit,str(path))).hexdigest()

def source_digest():
    parts=[]
    for path in (HERE/"composition_audit.py",HERE/"authoring.py",HERE/"svg_contract.py"):
        parts.extend((path.name.encode("utf-8"),b"\0",bounded_regular_bytes(path,1024*1024,str(path))))
    return hashlib.sha256(b"".join(parts)).hexdigest()

def canonical(value):
    return (json.dumps(value,sort_keys=True,indent=2)+"\n").encode("utf-8")

def exact_json(path):
    def pairs(items):
        result={}
        for key,value in items:
            if key in result: raise ContractError("duplicate JSON key")
            result[key]=value
        return result
    try:
        return json.loads(bounded_regular_bytes(path,MAX_AUDIT,str(path)).decode("utf-8"),
                          object_pairs_hook=pairs)
    except (UnicodeDecodeError,ValueError) as error:
        raise ContractError("invalid composition audit: "+str(error))

def bounds_record(bounds):
    if bounds is None: return None
    x0,y0,x1,y1=bounds
    return {"x0":x0,"y0":y0,"x1":x1,"y1":y1}

def contains(outer,inner):
    return outer[0]<=inner[0] and outer[1]<=inner[1] and outer[2]>=inner[2] and outer[3]>=inner[3]

def opaque_rectangle(op):
    return op["op"]=="rect" and op["fill"]!="none" and op["opacity"]>=0.999

def flatten(plan):
    """Return trusted-plan paint order and an object lookup after validation."""
    paint=[]; objects={}
    for layer_index,layer in enumerate(plan["layers"]):
        for object_index,op in enumerate(layer["objects"]):
            entry={"id":op["id"],"op":op,"layer_id":layer["id"],"layer_name":layer["name"],
                   "layer_index":layer_index,"object_index":object_index}
            paint.append(entry); objects[op["id"]]=entry
    return paint,objects

def build(work):
    work=pathlib.Path(work).resolve(); out=work/"output"
    _,width,height,request_raw=read_request(work/"request.json")
    plan_path=out/"inkscape-plan.json"
    plan_raw=bounded_regular_bytes(plan_path,16*1024*1024,str(plan_path))
    plan=authoring.load(plan_path)
    # Validate the complete plan through the same trusted schema/DOM path that
    # creates the master.  The audit never trusts an unvalidated local object.
    authoring.dom(plan,width,height)
    targets=authoring.review_targets(plan)
    paint,objects=flatten(plan)
    position={entry["id"]:index for index,entry in enumerate(paint)}
    rows=[]; covered=[]
    for target in targets:
        entry=objects[target["id"]]
        bounds=authoring.object_bounds(entry["op"])
        row={"id":target["id"],"importance":target["importance"],
             "layer_id":entry["layer_id"],"layer_name":entry["layer_name"],
             "bounds":bounds_record(bounds)}
        if bounds is None:
            row["status"]="geometry-unassessable"
            rows.append(row)
            continue
        occluders=[]
        for candidate in paint[position[target["id"]]+1:]:
            if not opaque_rectangle(candidate["op"]): continue
            candidate_bounds=authoring.object_bounds(candidate["op"])
            if contains(candidate_bounds,bounds):
                occluders.append({"id":candidate["id"],"layer_id":candidate["layer_id"],
                                  "layer_name":candidate["layer_name"],"bounds":bounds_record(candidate_bounds)})
        if occluders:
            row["status"]="fully-covered-by-opaque-rectangle"
            row["occluders"]=occluders
            covered.append({"target_id":target["id"],"importance":target["importance"],
                            "occluders":occluders})
        else:
            row["status"]="not-fully-covered-by-opaque-rectangle"
        rows.append(row)
    bindings={
        "request":{"path":"request.json","sha256":hashlib.sha256(request_raw).hexdigest()},
        "plan":{"path":"output/inkscape-plan.json","sha256":hashlib.sha256(plan_raw).hexdigest()},
        "master":{"path":"output/illustration.inkscape.svg","sha256":digest(out/"illustration.inkscape.svg",16*1024*1024)},
        "final":{"path":"output/illustration.svg","sha256":digest(out/"illustration.svg",16*1024*1024)},
        "preview":{"path":"output/preview.png","sha256":digest(out/"preview.png")},
    }
    return {"schema":"inkscape-illustrator.composition-audit/v1","auditor_sha256":source_digest(),
            "canvas":{"width":width,"height":height},"bindings":bindings,"targets":rows,
            "fully_covered_targets":covered,
            "limitations":[
                "Only full coverage by a later opaque rectangle is mechanically detected.",
                "This audit does not judge semantic fidelity, style, text visibility, partial occlusion, or aesthetic quality.",
            ]}

def write(work):
    work=pathlib.Path(work).resolve(); out=work/"output"; path=out/AUDIT_NAME
    if path.exists() and path.is_symlink(): raise ContractError("composition audit must not be a symlink")
    value=build(work)
    path.write_bytes(canonical(value))
    return value

def blocking(value):
    return [item for item in value["fully_covered_targets"] if item["importance"] in {"primary","required"}]

def verify(work):
    work=pathlib.Path(work).resolve(); path=work/"output"/AUDIT_NAME
    actual=bounded_regular_bytes(path,MAX_AUDIT,str(path))
    # Parse before comparison for a clear malformed-artifact diagnostic.
    exact_json(path)
    expected=canonical(build(work))
    if actual!=expected: raise ContractError("stale or forged composition audit")
    value=json.loads(expected.decode("utf-8"))
    problems=blocking(value)
    if problems:
        names=", ".join(item["target_id"] for item in problems)
        raise ContractError("review target fully covered by a later opaque rectangle: "+names)
    return value
