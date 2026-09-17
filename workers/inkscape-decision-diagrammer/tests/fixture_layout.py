#!/usr/bin/env python3
"""One synthetic SWOT export fixture, not a worker template. stdlib only."""
import hashlib,json,pathlib,xml.etree.ElementTree as E
O=pathlib.Path(__file__).resolve().parent
def write(p,v): p.write_text(json.dumps(v,ensure_ascii=False,sort_keys=True,indent=2)+"\n",encoding="utf-8")
def sha(p): return hashlib.sha256(p.read_bytes()).hexdigest()
data=json.loads((O/"layout-inputs.json").read_text())
source=data["selector"]; W=800; H=640; M=32; gap=16; col=(W-2*M-gap)/2; row=144
regions=[]
for j,r in enumerate(source["layout"]["regions"]):
    regions.append({"id":r["id"],"label":r["label"],"meaning":r["definition"],
                    "box":[M+(j%2)*(col+gap),128+(j//2)*(row+gap),col,row]})
tray=[M,464,W-2*M,80]
items=[]
for j,e in enumerate(source["elements"]):
    b=regions[0]["box"] if e["placement"]["region"] else tray
    x=b[0]+b[2]/2; y=b[1]+b[3]/2+8
    items.append({"id":e["id"],"label":e["label"],"source_ids":[e["id"]],"evidence":e["evidence"],
        "assumption":e["basis"]!="supplied","region":e["placement"]["region"],"visibility":None,
        "x":x,"y":y,"group":e["id"]+"-node","mark":e["id"]+"-card","label_id":e["id"]+"-text"})
P={"schema":"inkscape-decision-diagrammer.plan/v1","framework":"SWOT Analysis",
 "grammar":"Internal/external crossed with helpful/harmful; unplaced evidence tray.",
 "decision":source["decision"],"audience":source["audience"],"axes":"Effect left helpful, right harmful. Origin top internal, bottom external.",
 "regions":regions,"tray":tray,"visibility_bands":{},"items":items,"relationships":[],
 "omissions":[],"selector":source,"math":{},
 "geometry":{"anchors":[{"id":"plot","x":M,"y":128}],
  "contains":[{"inner":i["label_id"],"outer":i["mark"],"padding":8} for i in items],
  "disjoint":[],"overlaps":[],
  "contrast":[{"foreground":r["id"]+"-heading","background":"page","paint":"fill"} for r in regions]
    +[{"foreground":k,"background":"page","paint":"fill"} for k in ("unplaced-heading","decision-title","footer")]
    +[{"foreground":i["label_id"],"background":i["mark"],"paint":"fill"} for i in items]
    +[{"foreground":i["mark"],"background":"page","paint":"stroke"} for i in items]},
 "style":{"font":"Arial, sans-serif","sizes":[24,18,16],"weights":[400,600],"margin":M,
 "tokens":{"canvas":{"light":"#fafaf8","dark":"#16181d"},"ink":{"light":"#16181d","dark":"#fafaf8"},
           "accent":{"light":"#145da0","dark":"#8dc8ff"}},"semantic_channels":[]}}
if "metadata" in data: P["metadata"]=data["metadata"]
write(O/"diagram-plan.json",P)
bindings=dict(data["inputs"])
for name in ("layout.py","layout-inputs.json","diagram-plan.json","design-notes.md"):
    bindings["output/"+name]=sha(O/name)
S="http://www.w3.org/2000/svg"; I="http://www.inkscape.org/namespaces/inkscape"
E.register_namespace("",S); E.register_namespace("inkscape",I)
def elem(parent,tag,**kw):
    return E.SubElement(parent,"{"+S+"}"+tag,{k.replace("_","-"):str(v) for k,v in kw.items()})
for dark in [False,True] if data["dark"] else [False]:
    canvas="#16181d" if dark else "#fafaf8"; ink="#fafaf8" if dark else "#16181d"
    root=E.Element("{"+S+"}svg",{"width":str(W),"height":str(H),"viewBox":f"0 0 {W} {H}"})
    elem(root,"title").text="Where should we investigate next?"
    elem(root,"desc",id="source-bindings").text=json.dumps(bindings,sort_keys=True)
    layers={}
    for name in ("canvas","frame","regions","connectors","marks","labels","annotation","title-block"):
        g=elem(root,"g",id="layer-"+name); g.set("{"+I+"}groupmode","layer"); g.set("{"+I+"}label",name); layers[name]=g
    elem(layers["canvas"],"rect",id="page",x=0,y=0,width=W,height=H,fill=canvas)
    def label(parent,ident,x,y,t,size=16,weight=400):
        e=elem(parent,"text",id=ident,x=x,y=y,fill=ink,font_family="Arial, sans-serif",
               font_size=size,font_weight=weight); e.text=t; return e
    for r in regions:
        x,y,w,h=r["box"]
        label(layers["labels"],r["id"]+"-heading",x,y+24,r["label"],18,600)
    for i in items:
        x,y=i["x"],i["y"]; cw=col-gap*2; ch=48
        g=elem(layers["marks"],"g",id=i["group"])
        elem(g,"rect",id=i["mark"],x=x-cw/2,y=y-ch/2,width=cw,height=ch,rx=8,
             fill=canvas,stroke=ink,stroke_width=1)
        label(g,i["label_id"],x-cw/2+16,y+5,i["label"])
    label(layers["labels"],"unplaced-heading",M,tray[1],"Unplaced — evidence required",18,600)
    label(layers["title-block"],"decision-title",M,64,"Where should we investigate next?",24,600)
    label(layers["annotation"],"footer",M,H-M,data.get("footer","Owner: unspecified · Date: unspecified · Basis: supplied fixture"))
    E.ElementTree(root).write(O/("diagram-dark.inkscape.svg" if dark else "diagram.inkscape.svg"),encoding="utf-8",xml_declaration=True)
