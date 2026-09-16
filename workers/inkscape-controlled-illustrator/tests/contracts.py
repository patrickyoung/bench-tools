#!/usr/bin/env python3
"""Synthetic deterministic tests; no model quality claims or retained artwork."""
import sys
sys.dont_write_bytecode=True
import copy, hashlib, json, pathlib, subprocess, tempfile
HOME=pathlib.Path(__file__).resolve().parents[1]/"expert"
sys.path.insert(0,str(HOME/"tools"))
import authoring
PLAN={"schema":"inkscape-plan/v1","title":"Contract sample","description":"A warm field and a blue curved silhouette",
 "layers":[{"id":"field","name":"Background field","objects":[
 {"op":"rect","id":"back","fill":"#eee5cc","stroke":"none","stroke_width":0,"opacity":1,"x":0,"y":0,"width":256,"height":256}]},
 {"id":"subject","name":"Subject silhouette","objects":[
 {"op":"path","id":"curve","fill":"#226688","stroke":"none","stroke_width":0,"opacity":1,"d":"M 35 210 C 25 20 220 20 220 210 Z"}]}],
 "review":{"targets":[{"id":"curve","importance":"primary"}]}}
def write(p,o): p.write_text(json.dumps(o))
def run(work,tool,accept=True):
    p=subprocess.run([str(HOME/tool)],cwd=work,stdout=subprocess.PIPE,stderr=subprocess.PIPE,timeout=120)
    if p.returncode!=(0 if accept else 1):
        raise AssertionError(f"{tool}: {p.returncode}\n{p.stdout.decode()}\n{p.stderr.decode()}")
    return p.stderr.decode()
def main():
    guidance=(HOME/"skills"/"composition-direction"/"SKILL.md").read_text().lower()
    visibility=(HOME/"skills"/"render-visibility"/"SKILL.md").read_text().lower()
    agents=(HOME/"AGENTS.md").read_text()
    assert "composition-direction" in agents
    assert "render-visibility" in agents
    assert "method, not a layout library" in guidance
    # Training examples must not become scene-specific instructions.
    assert all(term not in guidance+visibility for term in ("greenhouse","heron","sunroom","astra","telescope","moon","deepseek","flash"))
    with tempfile.TemporaryDirectory(prefix="controlled-contract-") as td:
        w=pathlib.Path(td); out=w/"output"; out.mkdir()
        write(w/"request.json",{"description":"Synthetic curve contract","width":256,"height":256})
        pp=out/"inkscape-plan.json"; write(pp,PLAN)
        (out/"design-notes.md").write_text("Intent: synthetic contract example.\nComposition: a curved silhouette on a field.\nPalette: warm cream and blue.\nStyle: economical flat vector.\nSimplifications: no extra details.\nVisibility: the primary curve is not covered by a later opaque rectangle.\nReview: deterministic checks only, no aesthetic review claimed.\n")
        run(w,"tools/author_in_inkscape"); run(w,"tools/finish"); run(w,"tools/review_composition"); run(w,"bin/check")
        print("PASS positive native plan-master-export and independent pixels")
        saved={p:p.read_bytes() for p in out.iterdir() if p.is_file()}
        hand=(w/"handoff.json").read_bytes()
        def restore():
            for p in out.iterdir():
                if p.is_file(): p.unlink()
            for p,b in saved.items(): p.write_bytes(b)
            (w/"handoff.json").write_bytes(hand)
        for name,change in [
            ("unsafe markup",lambda p:p.update(title="<svg/>")),
            ("URL",lambda p:p.update(title="https://example.com")),
            ("unknown operation",lambda p:p["layers"][1]["objects"][0].update(op="script")),
            ("out of range",lambda p:p["layers"][0]["objects"][0].update(width=257)),
            ("unsafe path",lambda p:p["layers"][1]["objects"][0].update(d="M 0 0 javascript:1")),
            ("unknown attribute",lambda p:p["layers"][0]["objects"][0].update(onload="code"))]:
            restore(); plan=copy.deepcopy(PLAN); change(plan); write(pp,plan)
            run(w,"tools/author_in_inkscape",False); run(w,"bin/check",False)
            print("PASS reject "+name)
        restore(); plan=copy.deepcopy(PLAN); plan.pop("review"); write(pp,plan)
        run(w,"tools/author_in_inkscape",False); run(w,"bin/check",False)
        print("PASS reject missing review targets")
        restore(); plan=copy.deepcopy(PLAN); plan["review"]["targets"][0]["id"]="not-real"; write(pp,plan)
        run(w,"tools/author_in_inkscape",False); run(w,"bin/check",False)
        print("PASS reject unknown review target")
        restore(); plan=copy.deepcopy(PLAN); plan["layers"].append({"id":"cover","name":"Opaque cover","objects":[
            {"op":"rect","id":"cover-panel","fill":"#222222","stroke":"none","stroke_width":0,"opacity":1,"x":20,"y":10,"width":210,"height":210}]})
        write(pp,plan); run(w,"tools/author_in_inkscape")
        covered_error=run(w,"tools/finish",False)
        assert "review target fully covered" in covered_error, covered_error
        audit=json.loads((out/"composition-audit.json").read_text())
        assert audit["fully_covered_targets"][0]["target_id"]=="curve"
        run(w,"bin/check",False)
        print("PASS reject fully covered primary review target")
        restore(); plan=copy.deepcopy(PLAN); plan["layers"][1]["objects"][0]["fill"]="#993355"; write(pp,plan)
        assert "authoring rejected" in run(w,"bin/check",False)
        print("PASS changed plan stale receipt")
        restore(); master=out/authoring.MASTER
        master.write_bytes(master.read_bytes().replace(b"#226688",b"#993355"))
        # Forge all self-reported digest bindings, including the complete manifest.
        ar=json.loads((out/"authoring-receipt.json").read_text())
        ar["master_sha256"]=hashlib.sha256(master.read_bytes()).hexdigest()
        write(out/"authoring-receipt.json",ar)
        render=json.loads((out/"render.json").read_text())
        render["bindings"]["master"]["sha256"]=ar["master_sha256"]; write(out/"render.json",render)
        subprocess.run([str(HOME/"tools/make-handoff"),"inkscape-illustrator","forged fixture",
            *["output/"+p.name for p in out.iterdir()]],cwd=w,check=True,stdout=subprocess.DEVNULL)
        assert "independently regenerated master differs" in run(w,"bin/check",False)
        print("PASS direct master tampering with forged hashes")
        restore()
        # White-box fault injection is confined to the test process, never the definition.
        original=authoring.generate
        def divergent(work,temp):
            master,receipt=original(work,temp)
            return master+b"\n",receipt
        authoring.generate=divergent
        try:
            authoring.verify(w)
            raise AssertionError("accepted mismatched regeneration")
        except authoring.ContractError as e:
            assert "regenerated master differs" in str(e)
        finally: authoring.generate=original
        print("PASS independently regenerated mismatch")
        for filename in ("inkscape-plan.json","authoring-receipt.json"):
            restore(); (out/filename).unlink(); run(w,"bin/check",False)
            print("PASS absent "+filename)
        restore(); (out/"authoring-receipt.json").write_text("{"); run(w,"bin/check",False)
        print("PASS malformed receipt")
        restore(); ar=json.loads((out/"authoring-receipt.json").read_text()); ar["adapter_sha256"]="0"*64
        write(out/"authoring-receipt.json",ar); run(w,"bin/check",False)
        print("PASS stale adapter binding")
if __name__=="__main__": main()
