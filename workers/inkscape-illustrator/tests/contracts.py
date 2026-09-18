#!/usr/bin/env python3
"""Deterministic contract cases only; no model-backed or artistic evaluation."""
import ast, hashlib, json, os, pathlib, re, shutil, struct, subprocess, sys, tempfile, zlib
import xml.etree.ElementTree as ET

if len(sys.argv)!=2:
    raise SystemExit("usage: contracts.py /path/to/expert")
EXPERT=pathlib.Path(sys.argv[1]).resolve()
FINISH=EXPERT/"tools"/"finish"; CHECK=EXPERT/"bin"/"check"; MAKER=EXPERT/"tools"/"make-handoff"
INK=os.environ.get("INKSCAPE") or shutil.which("inkscape")
if not INK: raise SystemExit("Inkscape required")
INK=str(pathlib.Path(INK).resolve())
ENV={**os.environ,"INKSCAPE":INK,"PYTHONDONTWRITEBYTECODE":"1"}

EXPECTED={
 "AGENTS.md","LICENSE","README.md","bin/check","bin/validate-handoff",
 "references/inkscape-cli.md","skills/vector-illustration/SKILL.md",
 "tools/finish","tools/make-handoff","tools/svg_contract.py"}
SCRIPT_PATHS={"bin/check","bin/validate-handoff","tools/finish","tools/make-handoff","tools/svg_contract.py"}

def preflight():
    found=set()
    for p in EXPERT.rglob("*"):
        rel=str(p.relative_to(EXPERT))
        if p.is_symlink(): raise AssertionError("unsafe reusable symlink: "+rel)
        if p.is_dir():
            if p.name=="__pycache__": raise AssertionError("pycache in reusable source")
            continue
        found.add(rel)
        if p.stat().st_size>20*1024*1024: raise AssertionError("oversized reusable file: "+rel)
        data=p.read_bytes()
        if b"\0" in data: raise AssertionError("NUL in reusable file: "+rel)
        if rel in SCRIPT_PATHS:
            text=data.decode("utf-8")
            if not text.startswith("#!/usr/bin/env python3\n"):
                raise AssertionError("unexpected executable format: "+rel)
            ast.parse(text,filename=rel)
            if not os.access(p,os.X_OK): raise AssertionError("script not executable: "+rel)
    if found!=EXPECTED:
        raise AssertionError("unexpected reusable inventory: "+repr(sorted(found^EXPECTED)))
    print("PASS inspected every reusable file before execution")

preflight()

SVG='''<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg"
 xmlns:inkscape="http://www.inkscape.org/namespaces/inkscape"
 width="320" height="256" viewBox="0 0 320 256">
 <title>Synthetic layered contract fixture</title>
 <desc>Simple geometric fixture for structural testing only</desc>
 <defs><linearGradient id="sky" x1="0" y1="0" x2="1" y2="1">
  <stop offset="0" stop-color="#17324d"/><stop offset="1" stop-color="#366f86"/>
 </linearGradient></defs>
 <g inkscape:groupmode="layer" inkscape:label="Background field" id="background">
  <rect width="320" height="256" fill="url(#sky)"/>
 </g>
 <g inkscape:groupmode="layer" inkscape:label="Foreground subject" id="subject">
  <path d="M42 211 C82 126 113 103 157 176 C194 89 230 74 278 211 Z" fill="#efb04c"/>
  <circle cx="224" cy="65" r="24" fill="#f7e4b3"/>
 </g>
</svg>
'''
NOTES='''# Design notes

## Intent
This deliberately simple synthetic image exercises the artifact contract only.

## Composition
Two overlapping geometric masses provide visible foreground and background.

## Palette
Dark blue-green values contrast with a warm ochre accent.

## Style
A minimal flat geometric style was selected solely for deterministic testing.

## Deliberate simplifications
There is no claim of polished illustration quality or brief-specific drawing craft.

## Remaining review limits
No professional-quality claim is made; independent visual review is not part of this contract fixture.
'''

def write_fixture(root,request=None):
    root.mkdir(parents=True,exist_ok=True); (root/"output").mkdir()
    (root/"request.json").write_text(json.dumps(request or {
      "description":"Synthetic geometric contract fixture","width":320,"height":256})+"\n")
    (root/"output"/"illustration.inkscape.svg").write_text(SVG)
    (root/"output"/"design-notes.md").write_text(NOTES)

def run(cmd,cwd,timeout=180):
    return subprocess.run([str(x) for x in cmd],cwd=cwd,env=ENV,
      stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True,timeout=timeout)

def success(name,cp):
    if cp.returncode:
        raise AssertionError(name+" unexpectedly failed\nSTDOUT:\n"+cp.stdout+"\nSTDERR:\n"+cp.stderr)
    print("PASS",name)

def failure(name,cp,category):
    if cp.returncode==0:
        raise AssertionError(name+" unexpectedly succeeded\n"+cp.stdout)
    diagnostic=(cp.stdout+"\n"+cp.stderr).lower()
    if category.lower() not in diagnostic:
        raise AssertionError(name+" wrong diagnostic; wanted %r\n%s" % (category,diagnostic))
    print("PASS",name,"rejected as",category)

def valid_case(top,name):
    root=top/name; write_fixture(root)
    success(name+" finish with rebased paths",run([FINISH],root))
    success(name+" valid before mutation",run([CHECK],root))
    receipt=json.loads((root/"output"/"render.json").read_text())
    for command in receipt["commands"]:
        for arg in command["argv"][1:]:
            if str(top/"positive") in arg and root.name!="positive":
                raise AssertionError("stale positive-workspace argv in "+name)
    manifest=json.loads((root/"handoff.json").read_text())
    for item in manifest["files"]:
        if pathlib.Path(item["source"]).parent.parent != root:
            raise AssertionError("stale manifest source in "+name)
    return root

def mutate_final(root,fragment):
    p=root/"output"/"illustration.svg"; text=p.read_text()
    p.write_text(text.replace("</svg>",fragment+"\n</svg>"))

def digest(path): return hashlib.sha256(path.read_bytes()).hexdigest()

def refresh_self_reports(root,*keys):
    receipt_path=root/"output"/"render.json"; receipt=json.loads(receipt_path.read_text())
    paths={"master":root/"output"/"illustration.inkscape.svg",
           "final":root/"output"/"illustration.svg","preview":root/"output"/"preview.png"}
    for key in keys: receipt["bindings"][key]["sha256"]=digest(paths[key])
    receipt_path.write_text(json.dumps(receipt,sort_keys=True,indent=2)+"\n")
    outputs=sorted("output/"+p.name for p in (root/"output").iterdir() if p.is_file())
    success("refresh bound handoff",run([sys.executable,MAKER,"inkscape-illustrator",
      "Refreshed deterministic negative fixture",*outputs],root))

def png_chunk(kind,payload):
    return struct.pack(">I",len(payload))+kind+payload+struct.pack(">I",zlib.crc32(kind+payload)&0xffffffff)

def rgba_png(width,height,pixel_fn):
    rows=[]
    for y in range(height):
        row=bytearray([0])
        for x in range(width): row.extend(pixel_fn(x,y))
        rows.append(bytes(row))
    ihdr=struct.pack(">IIBBBBB",width,height,8,6,0,0,0)
    return b"\x89PNG\r\n\x1a\n"+png_chunk(b"IHDR",ihdr)+png_chunk(b"IDAT",zlib.compress(b"".join(rows)))+png_chunk(b"IEND",b"")

with tempfile.TemporaryDirectory(prefix="inkscape-contracts-") as td:
    top=pathlib.Path(td).resolve()
    missing=top/"missing-description"; write_fixture(missing,{"width":320,"height":256})
    failure("missing description",run([FINISH],missing),"description")
    bad=top/"bad-dimensions"; write_fixture(bad,{"description":"x","width":True,"height":256})
    failure("bad dimensions including boolean",run([FINISH],bad),"integer")

    oversized_request=top/"oversized-request"; write_fixture(oversized_request)
    with (oversized_request/"request.json").open("wb") as f: f.truncate(64*1024+1)
    failure("oversized request stat bound",run([FINISH],oversized_request),"size")

    symlink_request=top/"symlink-request"; write_fixture(symlink_request)
    real=symlink_request/"real-request.json"; shutil.move(symlink_request/"request.json",real)
    (symlink_request/"request.json").symlink_to(real)
    failure("request symlink",run([FINISH],symlink_request),"non-symlink")

    symlink_svg=top/"symlink-svg"; write_fixture(symlink_svg)
    real=symlink_svg/"output"/"real.svg"; shutil.move(symlink_svg/"output"/"illustration.inkscape.svg",real)
    (symlink_svg/"output"/"illustration.inkscape.svg").symlink_to(real)
    failure("SVG symlink",run([FINISH],symlink_svg),"non-symlink")

    oversized_svg=top/"oversized-svg"; write_fixture(oversized_svg)
    with (oversized_svg/"output"/"illustration.inkscape.svg").open("wb") as f: f.truncate(16*1024*1024+1)
    failure("oversized SVG stat bound",run([FINISH],oversized_svg),"size")

    base=valid_case(top,"positive")

    case=valid_case(top,"missing-output")
    (case/"output"/"illustration.svg").unlink()
    failure("missing output",run([CHECK],case),"missing required")

    case=valid_case(top,"active-style")
    mutate_final(case,'<style>@font-face { font-family: bad; src: url(https://example.invalid/x); }</style>')
    failure("active style element",run([CHECK],case),"element forbidden")

    case=valid_case(top,"processing-instruction")
    p=case/"output"/"illustration.svg"; p.write_text(re.sub(r"<svg\b", "<?bad x?>\n<svg", p.read_text(), count=1))
    failure("processing instruction",run([CHECK],case),"processing instruction")

    case=valid_case(top,"external-href")
    mutate_final(case,'<use href="https://example.invalid/a.svg#x"/>')
    failure("external href",run([CHECK],case),"external svg reference")

    case=valid_case(top,"blank-preview")
    (case/"output"/"preview.png").write_bytes(rgba_png(320,256,lambda x,y:(0,0,0,0)))
    failure("fully transparent blank PNG",run([CHECK],case),"blank")

    case=valid_case(top,"oversized-png")
    (case/"output"/"preview.png").write_bytes(rgba_png(4097,1,lambda x,y:(x%251,0,0,255)))
    failure("oversized PNG dimensions",run([CHECK],case),"dimensions exceed")

    case=valid_case(top,"truncated-png")
    p=case/"output"/"preview.png"; p.write_bytes(p.read_bytes()[:-8])
    failure("truncated PNG",run([CHECK],case),"truncated PNG")

    case=valid_case(top,"trailing-png")
    p=case/"output"/"preview.png"; p.write_bytes(p.read_bytes()+b"trailing")
    failure("PNG trailing data",run([CHECK],case),"trailing data")

    case=valid_case(top,"tampered-preview-valid")
    alt=case/"alternate.svg"; alt.write_text(SVG.replace("#efb04c","#ef304c"))
    preview=case/"output"/"preview.png"
    success("render valid changed preview",run([INK,"--export-type=png","--export-width","320",
      "--export-height","256","--export-filename",preview,alt],case))
    refresh_self_reports(case,"preview")
    failure("valid changed preview with refreshed hashes",run([CHECK],case),
            "supplied preview pixel content differs")

    case=valid_case(top,"wrong-master")
    p=case/"output"/"illustration.inkscape.svg"; p.write_text(p.read_text().replace("#efb04c","#20cc62"))
    refresh_self_reports(case,"master")
    failure("wrong master with refreshed hashes",run([CHECK],case),
            "editable master pixel content differs")

    for name, fragment, category in [
        ("embedded-raster", '<image href="data:image/png;base64,AAAA" width="2" height="2"/>', "element forbidden"),
        ("script", '<script>throw 1</script>', "element forbidden"),
        ("unresolved-reference", '<path d="M1 1L8 8" fill="url(#absent)"/>', "unresolved internal"),
    ]:
        case=valid_case(top,name)
        mutate_final(case,fragment)
        failure(name,run([CHECK],case),category)

    case=valid_case(top,"malformed-xml")
    (case/"output"/"illustration.svg").write_text("<svg")
    failure("malformed XML",run([CHECK],case),"malformed SVG XML")

    case=valid_case(top,"wrong-specialist")
    p=case/"handoff.json"; data=json.loads(p.read_text()); data["specialist"]="blender"
    p.write_text(json.dumps(data))
    failure("wrong specialist",run([CHECK],case),"wrong handoff specialist")

    case=valid_case(top,"stale-request")
    p=case/"request.json"; data=json.loads(p.read_text()); data["description"]="A different new job"
    p.write_text(json.dumps(data))
    failure("stale request",run([CHECK],case),"stale render binding: request")

    case=top/"lettering"; write_fixture(case)
    p=case/"output"/"illustration.inkscape.svg"
    p.write_text(p.read_text().replace('</svg>', '<text x="24" y="42" style="font-size:28px;font-family:serif;letter-spacing:2px;fill:#ffffff">SUNROOM</text></svg>'))
    success("finish editable lettering",run([FINISH],case))
    success("accept outlined lettering",run([CHECK],case))
    assert any(x.tag.endswith('}text') for x in ET.parse(p).iter())
    assert not any(x.tag.endswith('}text') for x in ET.parse(case/"output"/"illustration.svg").iter())
    print("PASS master lettering stays editable and final lettering is outlined")

print("all deterministic contract cases passed; no artistic-quality claim")
