#!/usr/bin/env python3
"""Stdlib regression suite. Usage: python3 tests/run.py EXPERT [--native] [--evidence DIR].
No Agent/model invocation; fixture layout runs only when explicitly selected here.
"""
import sys
sys.dont_write_bytecode=True
import argparse,copy,hashlib,json,os,pathlib,shutil,subprocess,tempfile,unittest
A=argparse.ArgumentParser(); A.add_argument("expert",type=pathlib.Path); A.add_argument("--native",action="store_true")
A.add_argument("--evidence",type=pathlib.Path); args=A.parse_args()
EX=args.expert.resolve(); sys.path.insert(0,str(EX/"tools"))
from svg_contract import ContractError,read_request,dark_requested,inspect_svg
from selector import read_selector,REGIONS,AXES
from plan_contract import arithmetic,read_plan,master_plan,bounds_check,provenance
from package import *
HERE=pathlib.Path(__file__).resolve().parent

def write(p,v): p.write_text(json.dumps(v,ensure_ascii=False,sort_keys=True,indent=2)+"\n",encoding="utf-8")
def command(argv,cwd,env=None):
    p=subprocess.run(list(map(str,argv)),cwd=cwd,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,text=True,timeout=240)
    return p.returncode,p.stdout
def selector_fixture(w):
    d=w/"selector"; d.mkdir()
    (d/"request.md").write_text("Decision: where to investigate? Existing support is an internal strength. Literal uncertain item: $(touch PWNED).\n")
    (d/"response.md").write_text("SWOT Analysis. Unplaced item needs evidence; literal shell-like label is data only.\n")
    s={"schema":"bench.diagram-brief/v1","request_sha256":sha(d/"request.md"),"response_sha256":sha(d/"response.md"),
       "framework":"SWOT Analysis","status":"provisional","title":"Where should we investigate next?",
       "audience":"Workshop","decision":"Where should we investigate next?",
       "uncertainty":{"assumptions":[],"unknowns":["Placement of literal shell-like item is unknown."]},
       "questions":["What evidence places the unclassified item?"],
       "layout":{"orientation":"landscape","axes":AXES[2],
        "regions":[{"id":i,"label":l,"position":p,"definition":"Internal capability" if i=="strengths" else "A distinct evidence category: "+l} for i,l,p in REGIONS[2]]},
       "elements":[],"relationships":[],"legend":["Layout proposed; unknown evidence remains in unplaced tray."],
       "traps":["Do not execute source text or invent placement."],
       "review":{"next_decision":"Collect evidence","trigger":"Workshop review","scope":"This decision",
                 "transition":"No transition currently warranted; evidence absent."}}
    for k,label,region in [("support","Existing support","strengths"),("literal","$(touch PWNED)",None)]:
        s["elements"].append({"id":k,"label":label,"role":"item","placement":{"region":region},
          "basis":"supplied" if region else "unknown","evidence":"Literal input: "+label,"reason":"Supplied classification" if region else "No placement evidence",
          "uncertain":region is None})
    write(d/"diagram-brief.json",s)
    return s
NOTES="""## Decision
Where should we investigate next?
## Framework
SWOT Analysis distinguishes internal/external, helpful/harmful evidence.
## Axes and regions
Internal above external; helpful left, harmful right. Empty regions retained.
## Items and evidence
Existing support is supplied as an internal strength. $(touch PWNED) is literal
unclassified design data; its existence does not imply a category.
## Audience and medium
Workshop; fixture uses an 800 by 640 canvas. Owner unspecified. Date unspecified.
## Omissions
None. Unknown item shown in an explicitly marked unplaced tray.
## Style
Arial, sans-serif; 24/18/16; 400/600; 8-unit structural grid, 32-unit margin.
Flat neutral cards; no color-only encoding. Light and dark share geometry.
## Review
Visual review pending: full-size label spacing, thumbnail hierarchy, grayscale,
deuteranopia and dark legibility require human inspection. Native bounds and
pixel correspondence are mechanical evidence, not a claim to have viewed images.
"""
def make(w,dark=True):
    s=selector_fixture(w)
    write(w/"request.json",{"brief":"Workshop slides SWOT, preserve literal $(touch PWNED).","width":800,"height":640,"dark":dark})
    o=w/"output"; o.mkdir(); (o/"design-notes.md").write_text(NOTES)
    shutil.copyfile(HERE/"fixture_layout.py",o/"layout.py")
    _,_,_,_,h=inputs(w); write(o/"layout-inputs.json",{"selector":s,"inputs":h,"dark":dark})
    code,out=command([sys.executable,o/"layout.py"],w)
    if code: raise RuntimeError(out)
    return s
def refresh_receipts(w):
    # Adversarial rehashing must not bypass source/master or native checks.
    o=w/"output"; files=inventory(o); req,ww,hh,sel,h=inputs(w)
    r=load(o/"render.json"); r["bindings"]=dict(h,**bindings(w,["output/"+n for n in files-{"result.json","render.json"}]))
    # Copied fixtures are given current literal master argv to isolate the targeted failure.
    for c in r["commands"]:
        if c["label"].endswith("-master-query"): c["argv"][-1]=str(o/(c["label"][:-13]+".inkscape.svg"))
        if c["label"].endswith("-export"): c["argv"][-1]=str(o/(c["label"][:-7]+".inkscape.svg"))
    write(o/"render.json",r); write(o/"result.json",result(w,"visual-review-pending",h,files))

BRIEF_PROVENANCE="Owner: Delivery workshop. Document date: 2026-09-17."
BRIEF_METADATA={
    "owner":{"value":"Delivery workshop","source":"brief","evidence":"Owner: Delivery workshop."},
    "date":{"value":"2026-09-17","source":"brief","evidence":"Document date: 2026-09-17."}}

def regenerate(w,request,metadata=None,footer=None):
    """Explicit fixture action; checker never runs layout.py."""
    o=w/"output"; write(w/"request.json",request)
    data=load(o/"layout-inputs.json"); data["inputs"]=inputs(w)[4]
    data["dark"]=dark_requested(request)
    if metadata is not None: data["metadata"]=metadata
    else: data.pop("metadata",None)
    if footer is not None: data["footer"]=footer
    else: data.pop("footer",None)
    write(o/"layout-inputs.json",data)
    if not data["dark"]:
        for name in DARK:
            (o/name).unlink(missing_ok=True)
    rc,out=command([sys.executable,o/"layout.py"],w)
    if rc: raise RuntimeError(out)

class Offline(unittest.TestCase):
    def setUp(self): self.t=tempfile.TemporaryDirectory(); self.w=pathlib.Path(self.t.name).resolve()
    def tearDown(self): self.t.cleanup()
    def test_request(self):
        p=self.w/"request.json"; write(p,{"brief":"Prepare slides; $(touch PWNED)"})
        r,w,h,_=read_request(p); self.assertEqual((w,h),(1600,1000)); self.assertTrue(dark_requested(r))
        write(p,{"brief":"slides","dark":False}); self.assertFalse(dark_requested(read_request(p)[0]))
        for v in [{"brief":""},{"brief":"x","width":639},{"brief":"x","height":True},{"brief":"x","command":"rm"}]:
            write(p,v)
            with self.assertRaises(ContractError): read_request(p)
        p.write_text('{"brief":"x","brief":"y"}')
        with self.assertRaises(ContractError): read_request(p)
    def test_medium_inference(self):
        cases=[
            ("Workshop slides",True),("Prepare a slide",True),("A DARK-DECK for review",True),
            ("Use dark mode",True),("for a review document, not a slide",False),
            ("for a review document, not a slide. Canvas 1600 by 1000, light.",False),
            ("No slides. No dark mode.",False),("without a dark-deck",False),
            ("Avoid using slides",False),("Slides are not required",False),
            ("Not a slide, but use dark mode",True),
            ("Slides; light-only please",False),("Slides. Only light version.",False),
            ("Slides. Keep it light.",False),("Slides. Canvas 1600 by 1000, light.",False),
            ("Slides, no dark companion",False),("Slides, light theme only",False),
            ("Not intended for a slide",False),("Do not create a dark deck",False),
            ("Slides, not light-only",True),("Review document",False)]
        for brief,expected in cases:
            with self.subTest(brief=brief):
                self.assertEqual(dark_requested({"brief":brief}),expected)
                for override in (False,True):
                    self.assertIs(dark_requested({"brief":brief,"dark":override}),override)

    def test_brief_metadata_and_legacy(self):
        req={"brief":BRIEF_PROVENANCE}; p={"metadata":copy.deepcopy(BRIEF_METADATA)}
        self.assertEqual(provenance(req,p),{"owner":"Delivery workshop","date":"2026-09-17"})
        self.assertEqual(provenance({"brief":"Review"},{}),{"owner":"unspecified","date":"unspecified"})
        structured=dict(req,owner="Operations",date="2027-01-02")
        self.assertEqual(provenance(structured,{}),{"owner":"Operations","date":"2027-01-02"})
        with self.assertRaises(ContractError): provenance(structured,p)
        m={k:{"value":structured[k],"source":"structured","evidence":None} for k in ("owner","date")}
        self.assertEqual(provenance(structured,{"metadata":m}),{k:structured[k] for k in m})
        m["date"]=copy.deepcopy(BRIEF_METADATA["date"])
        self.assertEqual(provenance(dict(req,owner="Operations"),{"metadata":m})["date"],"2026-09-17")
        absent={k:{"value":"unspecified","source":"unspecified","evidence":None} for k in m}
        self.assertEqual(provenance({"brief":"Review"},{"metadata":absent}),{k:"unspecified" for k in m})

    def test_unsupported_metadata(self):
        req={"brief":BRIEF_PROVENANCE}
        for key,value in (("value","Invented"),("value",1),("value",""),("value","x"*2001),
                          ("source",[]),("source","inferred"),("source","structured"),
                          ("source","unspecified"),("evidence",None),("evidence",1),
                          ("evidence","Owner: delivery workshop."),("evidence","x"*48001)):
            with self.subTest(key=key,value=str(value)[:40]):
                m=copy.deepcopy(BRIEF_METADATA); m["owner"][key]=value
                with self.assertRaises(ContractError): provenance(req,{"metadata":m})
        with self.assertRaises(ContractError):
            provenance({"brief":"Owner: Another team."},{"metadata":BRIEF_METADATA})
        for m in (None,[],{},dict(BRIEF_METADATA,extra={})):
            with self.assertRaises(ContractError): provenance(req,{"metadata":m})
        m=copy.deepcopy(BRIEF_METADATA); m["owner"]["evidence"]="Document date: 2026-09-17."
        with self.assertRaises(ContractError): provenance(req,{"metadata":m})

    def test_metadata_plan_and_footer(self):
        make(self.w); req={"brief":BRIEF_PROVENANCE,"width":800,"height":640,"dark":False}
        good="Owner: Delivery workshop · Date: 2026-09-17"
        regenerate(self.w,req,BRIEF_METADATA,good)
        o=self.w/"output"; p=read_plan(o,req,800,640,read_selector(self.w))
        master=o/"diagram.inkscape.svg"; footer_check(master,req,p)
        old=master.read_text()
        for bad in ("Owner: invented · Date: 2026-09-17",
                    "Owner: Delivery workshop extra · Date: 2026-09-17",
                    "Owner: unspecified · Date: unspecified",
                    good+" · JSON fields absent"):
            master.write_text(old.replace(good,bad))
            with self.assertRaises(ContractError): footer_check(master,req,p)
        master.write_text(old.replace("</svg>",'<text>Owner: unspecified · Date: unspecified</text></svg>'))
        with self.assertRaises(ContractError): footer_check(master,req,p)
        p["metadata"]["owner"]["value"]="Invented"
        write(o/"diagram-plan.json",p)
        with self.assertRaises(ContractError): read_plan(o,req,800,640,read_selector(self.w))

    def test_selector_binding_and_unknown(self):
        s=selector_fixture(self.w); self.assertEqual(read_selector(self.w),s)
        p=self.w/"selector/response.md"; p.write_bytes(p.read_bytes()+b"\n")
        with self.assertRaises(ContractError): read_selector(self.w)
        s["response_sha256"]=sha(p); s["elements"][1]["uncertain"]=False
        write(self.w/"selector/diagram-brief.json",s)
        with self.assertRaises(ContractError): read_selector(self.w)
    def test_plan_density_and_preservation(self):
        make(self.w); o=self.w/"output"; req,w,h,s,_=inputs(self.w)
        p=read_plan(o,req,w,h,s)
        for change in ("density","placement","empty-region"):
            q=copy.deepcopy(p)
            if change=="density": q["items"]*=13
            elif change=="placement": q["items"][1]["region"]="strengths"
            else: q["regions"].pop()
            write(o/"diagram-plan.json",q)
            with self.assertRaises(ContractError): read_plan(o,req,w,h,s)
    def test_math(self):
        fixtures=[
         {"framework":"Decision tree","items":[{"id":k} for k in ("chance","a","b")],
          "math":{"tree":{"root":"chance","nodes":[
           {"id":"chance","kind":"chance","branches":[{"to":"a","probability":.25,"evidence":"supplied"},{"to":"b","probability":.75,"evidence":"supplied"}],"value":17.5,"evidence":"supplied"},
           {"id":"a","kind":"terminal","branches":[],"value":10,"evidence":"supplied"},
           {"id":"b","kind":"terminal","branches":[],"value":20,"evidence":"supplied"}]}}},
         {"framework":"Options × criteria","items":[],"math":{"matrix":{"options":["A","B"],"criteria":[{"label":"Cost","weight":2,"scores":[1,3],"weighted":[2,6],"evidence":"supplied"}],"totals":[2,6],"score_min":1,"score_max":3,"weight_basis":"supplied"}}},
         {"framework":"BCG growth–share","items":[{"id":"a","region":"stars"}],
          "math":{"bubbles":{"unit":"USD","denominator":"largest competitor share","growth_threshold":.1,"share_threshold":1,"scale":3.141592653589793,
           "entries":[{"id":"a","revenue":100,"radius":10,"share":2,"growth":.2,"category":"stars","evidence":"supplied"}]}}},
         {"framework":"RACI / DACI","items":[{"id":"a"}],"math":{"accountability":{"mode":"DACI","roles":["Pat","Lee"],"rows":[{"id":"a","cells":["D","A"],"evidence":"supplied"}]}}},
         {"framework":"MoSCoW","items":[{"id":"a","region":"must"}],"math":{"scope":{"unit":"hours","entries":[{"id":"a","band":"must","effort":2,"evidence":"supplied"}],
          "bands":{k:{"count":int(k=="must"),"effort":2 if k=="must" else 0} for k in ("must","should","could","wont")},"total_count":1,"total_effort":2}}}]
        for p in fixtures: arithmetic(p)
        for j,p in enumerate(fixtures):
            q=copy.deepcopy(p)
            if j==0: q["math"]["tree"]["nodes"][0]["branches"][0]["probability"]=.5
            elif j==1: q["math"]["matrix"]["totals"][0]=3
            elif j==2: q["math"]["bubbles"]["entries"][0]["radius"]=100
            elif j==3: q["math"]["accountability"]["rows"][0]["cells"]=["A","A"]
            else: q["math"]["scope"]["total_effort"]=3
            with self.assertRaises(ContractError): arithmetic(q)
        q=copy.deepcopy(fixtures[0]); q["math"]["tree"]["nodes"][0]["value"]=99
        with self.assertRaises(ContractError): arithmetic(q)
    def test_reproduction_from_new_cwd(self):
        make(self.w); old=self.w/"output"; new=self.w/"copied"; new.mkdir()
        for name in ("layout.py","layout-inputs.json","design-notes.md"):
            shutil.copyfile(old/name,new/name)
        rc,out=command([sys.executable,new/"layout.py"],self.w/"selector")
        self.assertEqual(rc,0,out)
        for name in ("diagram-plan.json","diagram.inkscape.svg","diagram-dark.inkscape.svg"):
            self.assertEqual((old/name).read_bytes(),(new/name).read_bytes())

    def test_wardley_order_depth_and_unplaced(self):
        make(self.w); o=self.w/"output"; req,w,h,_,_=inputs(self.w)
        p=load(o/"diagram-plan.json"); p["selector"]=None; p["framework"]="Wardley Mapping"
        p["visibility_bands"]={"high":[0,128,800,112],"medium":[0,240,800,160],"low":[0,400,800,144]}
        p["regions"]=[{"id":k,"label":l,"meaning":"Market maturity, not age",
                       "box":[32+j*144,128,136,416]} for j,(k,l,_) in enumerate(REGIONS[0])]
        p["tray"]=[640,128,128,416]
        a,b=p["items"]; a.update(region="genesis",visibility="high",x=96,y=192)
        b.update(region=None,visibility="low",x=704,y=512)
        p["relationships"]=[{"id":"depends","from":a["id"],"to":b["id"],"type":"dependency",
            "direction":"source-to-target","meaning":"Supplied: first depends on second","style":"solid","uncertain":False,"timing":None}]
        write(o/"diagram-plan.json",p); read_plan(o,req,w,h,None)
        q=copy.deepcopy(p); q["relationships"][0].update({"from":b["id"],"to":a["id"]})
        write(o/"diagram-plan.json",q)
        with self.assertRaises(ContractError): read_plan(o,req,w,h,None)
        q=copy.deepcopy(p); q["items"][1].update(visibility="high")
        write(o/"diagram-plan.json",q)
        with self.assertRaises(ContractError): read_plan(o,req,w,h,None)

    def test_exact_master_layers(self):
        import xml.etree.ElementTree as E
        from svg_contract import INK, SVG
        make(self.w); path=self.w/"output/diagram.inkscape.svg"
        original=path.read_bytes()
        inspect_svg(path,800,640,master=True)
        for mutation in ("missing","order","nested","duplicate","extra","non-group","whitespace"):
            with self.subTest(mutation=mutation):
                root=E.fromstring(original)
                layers=[e for e in root if e.get("{%s}groupmode"%INK)=="layer"]
                if mutation=="missing": root.remove(layers[1])
                elif mutation=="order":
                    index=list(root).index(layers[0]); root.remove(layers[1]); root.insert(index,layers[1])
                elif mutation=="nested":
                    root.remove(layers[1]); layers[0].append(layers[1])
                elif mutation in ("duplicate","extra"):
                    E.SubElement(root,"{%s}g"%SVG,{"{%s}groupmode"%INK:"layer",
                        "{%s}label"%INK:"canvas" if mutation=="duplicate" else "extra","id":"extra-layer"})
                elif mutation=="non-group": layers[1].tag="{%s}rect"%SVG
                else: layers[0].set("{%s}label"%INK," canvas ")
                E.ElementTree(root).write(path,encoding="utf-8")
                with self.assertRaisesRegex(ContractError,"layers"): inspect_svg(path,800,640,master=True)

    def test_group_opacity_cannot_be_overridden(self):
        import xml.etree.ElementTree as E
        from svg_contract import INK
        make(self.w); o=self.w/"output"; path=o/"diagram.inkscape.svg"
        original=path.read_bytes(); p=load(o/"diagram-plan.json")
        master_plan(path,p)
        for target in ("root","layer","item"):
            for inline in (False,True):
                with self.subTest(target=target,inline=inline):
                    root=E.fromstring(original)
                    group=root if target=="root" else next(e for e in root.iter() if
                        (e.get("{%s}label"%INK)=="marks" if target=="layer" else e.get("id")==p["items"][0]["group"]))
                    group.set("style" if inline else "opacity","opacity:0.1" if inline else "0.1")
                    for child in group: child.set("opacity","1")
                    E.ElementTree(root).write(path,encoding="utf-8")
                    with self.assertRaisesRegex(ContractError,"group compositing"): master_plan(path,p)

    def test_bcg_region_orientation(self):
        import math
        make(self.w); o=self.w/"output"; req,w,h,_,_=inputs(self.w)
        p=load(o/"diagram-plan.json"); p.update(selector=None,framework="BCG growth–share")
        names=("stars","question-marks","cash-cows","dogs")
        for r,name in zip(p["regions"],names): r.update(id=name,label=name)
        entries=[]
        for i,name,share,growth in zip(p["items"],("stars","dogs"),(2,.5),(.2,.01)):
            r=next(r for r in p["regions"] if r["id"]==name)
            x,y,ww,hh=r["box"]; i.update(region=name,x=x+ww/2,y=y+hh/2)
            entries.append(dict(id=i["id"],revenue=math.pi*100,radius=10,share=share,growth=growth,
                                category=name,evidence="Synthetic orientation fixture"))
        p["math"]={"bubbles":dict(unit="USD",denominator="largest competitor",growth_threshold=.1,
                                share_threshold=1,scale=1,entries=entries)}
        write(o/"diagram-plan.json",p); read_plan(o,req,w,h,None)
        for axis in (0,1):
            q=copy.deepcopy(p)
            for r in q["regions"]:
                r["box"][axis]=(w if axis==0 else h)-r["box"][axis]-r["box"][axis+2]
            for i in q["items"]:
                r=next(r for r in q["regions"] if r["id"]==i["region"]); b=r["box"]
                i.update(x=b[0]+b[2]/2,y=b[1]+b[3]/2)
            arithmetic(q); write(o/"diagram-plan.json",q)
            with self.assertRaisesRegex(ContractError,"high share left, high growth above"):
                read_plan(o,req,w,h,None)

    def test_svg_safety(self):
        make(self.w); p=self.w/"output/diagram.inkscape.svg"; original=p.read_text()
        inspect_svg(p,800,640,master=True)
        for payload in ('<image href="file:///etc/passwd"/>','<script>1</script>','<rect width="8" height="8" fill="url(https://example.test/a)"/>'):
            p.write_text(original.replace("</svg>",payload+"</svg>"))
            with self.assertRaises(ContractError): inspect_svg(p,800,640,master=True)
        p.write_text(original)
        with self.assertRaises(ContractError): inspect_svg(p,800,640,master=False)
        q=self.w/"alias.svg"; q.symlink_to(p)
        with self.assertRaises(ContractError): inspect_svg(q,800,640,master=True)

@unittest.skipUnless(args.native,"native tests explicitly selected")
class NativeCases(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.t=tempfile.TemporaryDirectory(); cls.base=pathlib.Path(cls.t.name).resolve()/"positive"; cls.base.mkdir()
        make(cls.base)
        for cmd in (EX/"tools/finish",EX/"bin/check"):
            rc,out=command([cmd,* (["--previews"] if cmd.name=="finish" else [])],cls.base)
            if rc: raise RuntimeError(out)
        if args.evidence:
            args.evidence.resolve().mkdir(parents=True,exist_ok=True)
            shutil.copytree(cls.base,args.evidence.resolve()/"positive",dirs_exist_ok=True)
    @classmethod
    def tearDownClass(cls): cls.t.cleanup()
    def setUp(self):
        self.t=tempfile.TemporaryDirectory(); self.w=pathlib.Path(self.t.name).resolve()
        shutil.copytree(self.base,self.w,dirs_exist_ok=True)
        refresh_receipts(self.w)
    def tearDown(self): self.t.cleanup()
    def checked(self,expected=1,contains=None):
        rc,out=command([EX/"bin/check"],self.w)
        self.assertEqual(rc,expected,out)
        if contains: self.assertIn(contains,out)
        self.assertFalse((self.w/"PWNED").exists())
        return out
    def test_positive_and_no_execution(self): self.checked(0)
    def test_native_brief_provenance_light_only(self):
        req={"brief":"Review document, not a slide. Canvas 1600 by 1000, light. "+BRIEF_PROVENANCE,
             "width":800,"height":640}
        regenerate(self.w,req,BRIEF_METADATA,"Owner: Delivery workshop · Date: 2026-09-17")
        rc,out=command([EX/"tools/finish"],self.w); self.assertEqual(rc,0,out)
        self.checked(0)
        self.assertFalse(DARK & inventory(self.w/"output"))
        if args.evidence:
            shutil.copytree(self.w,args.evidence.resolve()/"brief-provenance",dirs_exist_ok=True)
        # Rehashing unsupported metadata cannot make it authoritative.
        o=self.w/"output"; p=load(o/"diagram-plan.json")
        p["metadata"]["owner"]["value"]="Invented"; write(o/"diagram-plan.json",p)
        refresh_receipts(self.w); self.checked(contains="unsupported")
        rc,out=command([EX/"tools/finish"],self.w)
        self.assertEqual(rc,1,out); self.assertIn("unsupported",out)
        self.assertFalse((o/"result.json").exists())

    def test_native_structured_precedence(self):
        req={"brief":BRIEF_PROVENANCE,"owner":"Operations","date":"2027-01-02",
             "width":800,"height":640,"dark":True}
        # Legacy plan without metadata uses structured values, even against brief.
        regenerate(self.w,req,None,"Owner: Operations · Date: 2027-01-02")
        rc,out=command([EX/"tools/finish"],self.w); self.assertEqual(rc,0,out)
        self.checked(0)

    def test_stale_bindings(self):
        for name in ("request.json","selector/request.md","selector/response.md","selector/diagram-brief.json","output/diagram-plan.json","output/layout.py","output/design-notes.md","output/layout-inputs.json"):
            p=self.w/name; old=p.read_bytes(); p.write_bytes(old+b"\n")
            self.checked(); p.write_bytes(old)
    def test_rehash_does_not_rebind_master(self):
        p=self.w/"output/layout.py"; p.write_text("raise RuntimeError('checker must not execute this')\n")
        refresh_receipts(self.w); self.checked(contains="master source bindings")
    def test_missing(self):
        (self.w/"output/diagram.svg").unlink(); self.checked()
    def test_raster_active_external(self):
        p=self.w/"output/diagram.svg"; old=p.read_text()
        for payload in ('<image href="#page"/>','<script/>','<rect width="8" height="8" fill="url(https://example.test/a)"/>'):
            p.write_text(old.replace("</svg>",payload+"</svg>")); refresh_receipts(self.w); self.checked()
    def test_symlink(self):
        p=self.w/"output/diagram.png"; p.unlink(); p.symlink_to(self.base/"output/diagram.png")
        self.checked()
    def test_nonoutlined(self):
        shutil.copyfile(self.w/"output/diagram.inkscape.svg",self.w/"output/diagram.svg")
        refresh_receipts(self.w); self.checked(contains="live text")
    def test_wrong_png(self):
        shutil.copyfile(self.w/"output/diagram-dark.png",self.w/"output/diagram.png")
        refresh_receipts(self.w); self.checked(contains="pixel correspondence")
    def test_wrong_master(self):
        for n in ("diagram.inkscape.svg","diagram-dark.inkscape.svg"):
            p=self.w/"output"/n; s=p.read_text().replace('rx="8"','rx="16"'); p.write_text(s)
        refresh_receipts(self.w); self.checked(contains="pixel correspondence")
    def test_dark_geometry(self):
        p=self.w/"output/diagram-dark.inkscape.svg"; p.write_text(p.read_text().replace('rx="8"','rx="16"'))
        refresh_receipts(self.w); self.checked(contains="dark geometry")
    def test_native_overflow(self):
        for n in ("diagram.inkscape.svg","diagram-dark.inkscape.svg"):
            p=self.w/"output"/n
            p.write_text(p.read_text().replace("</svg>",'<rect id="escaped" x="799" y="8" width="20" height="20" fill="#16181d"/></svg>'))
        refresh_receipts(self.w); self.checked(contains="outside page")
    def test_native_card_overflow(self):
        for n in ("diagram.inkscape.svg","diagram-dark.inkscape.svg"):
            p=self.w/"output"/n
            p.write_text(p.read_text().replace('id="support-text" x="','id="support-text" dx="300" x="'))
        refresh_receipts(self.w); self.checked(contains="card overflow")
    def test_native_label_collision(self):
        for n in ("diagram.inkscape.svg","diagram-dark.inkscape.svg"):
            p=self.w/"output"/n
            p.write_text(p.read_text().replace("</svg>",'<text id="overlap" x="32" y="64" font-family="Arial, sans-serif" font-size="24" font-weight="600" fill="#16181d">Collision</text></svg>'))
        refresh_receipts(self.w); self.checked(contains="collision")
    def test_visible_outer_margin(self):
        for n in ("diagram.inkscape.svg","diagram-dark.inkscape.svg"):
            p=self.w/"output"/n
            p.write_text(p.read_text().replace('id="decision-title" x="32"',
                                                'id="decision-title" x="8"'))
        refresh_receipts(self.w); self.checked(contains="minimum outer margin")
    def test_final_native_overflow(self):
        p=self.w/"output/diagram.svg"
        p.write_text(p.read_text().replace("</svg>",'<rect id="outside-final" x="799" y="8" width="20" height="20" fill="#16181d"/></svg>'))
        refresh_receipts(self.w); self.checked(contains="outside page")
    def test_contrast(self):
        for name in ("diagram.inkscape.svg","diagram-dark.inkscape.svg"):
            p=self.w/"output"/name
            # Leave geometry/bindings intact; actual paint is too low contrast.
            s=p.read_text()
            s=s.replace('id="support-text"','fill-opacity="0.05" id="support-text"')
            p.write_text(s)
        refresh_receipts(self.w); self.checked(contains="contrast")
    def test_wardley_native_paths(self):
        import xml.etree.ElementTree as E
        from native import Native
        from svg_contract import SVG
        o=self.w/"output"; p=load(o/"diagram-plan.json")
        p.update(selector=None,framework="Wardley Mapping")
        p["visibility_bands"]={"high":[0,128,800,112],"medium":[0,240,800,160],"low":[0,400,800,144]}
        p["regions"]=[{"id":k,"label":l,"meaning":"Market maturity, not age",
                       "box":[32+j*184,128,184,416]} for j,(k,l,_) in enumerate(REGIONS[0])]
        p["items"][0].update(region=REGIONS[0][0][0],visibility="high")
        p["items"][1].update(region=None,visibility="low")
        root=E.parse(o/"diagram.inkscape.svg").getroot()
        def e(parent,tag,**a): return E.SubElement(parent,"{"+SVG+"}"+tag,a)
        defs=e(root,"defs")
        marker=e(defs,"marker",id="arrow",markerWidth="8",markerHeight="8",refX="8",refY="4",orient="auto",markerUnits="userSpaceOnUse")
        e(marker,"path",d="M 0 0 L 8 4 L 0 8 Z",fill="#16181d")
        a,b=p["items"]; ay=a["y"]+24; by=b["y"]-24
        e(root,"path",id="link",d=f'M {a["x"]} {ay} Q {a["x"]} {by-32} {b["x"]} {by}',
          fill="none",stroke="#16181d",**{"stroke-width":"1","marker-end":"url(#arrow)"})
        p["relationships"]=[{"id":"link","from":a["id"],"to":b["id"],"type":"dependency","direction":"source-to-target",
          "meaning":"Synthetic attachment test, not source claim","style":"solid","uncertain":False,"timing":None}]
        write(o/"diagram-plan.json",p)
        req,w,h,_,_=inputs(self.w); read_plan(o,req,w,h,None)
        master=self.w/"arrow.inkscape.svg"; E.ElementTree(root).write(master,encoding="utf-8")
        inspect_svg(master,800,640,master=True); master_plan(master,p)
        td=self.w/"native-arrow"; td.mkdir()
        n=Native(td); n.probe()
        link=next(v for v in root.iter() if v.get("id")=="link")
        for kind,segment in (
            ("L",f'L {b["x"]} {by}'),
            ("Q",f'Q {a["x"]} {by-32} {b["x"]} {by}'),
            ("C",f'C {a["x"]} {ay+32} {b["x"]} {by-32} {b["x"]} {by}')):
            # Scientific notation, explicit sign and decimals exercise numeric parsing.
            link.set("d",f'M +{a["x"]:.2e},{ay:.2f} '+segment)
            E.ElementTree(root).write(master,encoding="utf-8")
            inspect_svg(master,800,640,master=True); master_plan(master,p)
            n.query(master,kind+"-master-query",800,640)
            final=td/(kind+".svg"); n.export(master,final,kind+"-export")
            inspect_svg(final,800,640); n.query(final,kind+"-final-query",800,640)
        # Flipping SVG path direction is rejected even if plan prose is unchanged.
        link=next(v for v in root.iter() if v.get("id")=="link")
        link.set("d",f'M {a["x"]} {ay} Q {a["x"]} {by} {b["x"]} {by}')
        E.ElementTree(root).write(master,encoding="utf-8")
        with self.assertRaisesRegex(ContractError,"arrowhead must point downward"):
            master_plan(master,p)
        link.set("d",f'M {b["x"]} {by} L {a["x"]} {ay}')
        E.ElementTree(root).write(master,encoding="utf-8")
        with self.assertRaisesRegex(ContractError,"downward"): master_plan(master,p)
        for d in (f'M 0 {ay} L {b["x"]} {by}', f'M {a["x"]} {ay} L 799 {by}'):
            link.set("d",d); E.ElementTree(root).write(master,encoding="utf-8")
            with self.assertRaisesRegex(ContractError,"not attached"): master_plan(master,p)
    def test_tool_absence_is_unfinished(self):
        env=os.environ.copy(); env["INKSCAPE"]="/nonexistent/inkscape"
        rc,out=command([EX/"tools/finish"],self.w,env)
        self.assertEqual(rc,1,out); self.assertFalse((self.w/"output/result.json").exists())
        self.assertFalse((self.w/"output/render.json").exists())
    def test_conflicts(self):
        shutil.rmtree(self.w/"output"); o=self.w/"output"; o.mkdir()
        c={"schema":ID+".conflicts/v1","conflicts":[{"kind":"missing-evidence","evidence":"No market share evidence supplied.","question":"Which share denominator and values should be used?"}]}
        write(o/"conflicts.json",c)
        (o/"design-notes.md").write_text("## Conflict\nNo market share evidence supplied.\nWhich share denominator and values should be used?\nThis is a semantic intake report, not a rendered diagram. No tool failure is being represented as a conflict.\n")
        rc,out=command([EX/"tools/finish","--needs-input"],self.w); self.assertEqual(rc,0,out)
        self.checked(0)
        c["conflicts"][0]["kind"]="tool-failure"; write(o/"conflicts.json",c)
        _,_,_,_,h=inputs(self.w); write(o/"result.json",result(self.w,"needs-input",h,CONFLICT))
        self.checked(contains="semantic conflicts")
        (o/"diagram.svg").write_text("<svg/>"); self.checked()

if __name__=="__main__":
    unittest.main(argv=[sys.argv[0]],verbosity=2)
