#!/usr/bin/env python3
"""Opt-in real local render tests. No installs or network; no aesthetic verdict."""
import argparse
import copy
import importlib.util
import json
import re
import xml.etree.ElementTree as ET
from pathlib import Path
import sys
sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("common", Path(__file__).with_name("common.py"))
f = importlib.util.module_from_spec(spec)
spec.loader.exec_module(f)

p = argparse.ArgumentParser()
p.add_argument("--runtime", required=True)
p.add_argument("--node", required=True)
p.add_argument("--browser", required=True)
p.add_argument("--qualifiers", action="store_true")
p.add_argument("--long-participants", action="store_true")
p.add_argument("--out", required=True, help="fresh external result directory, outside the source library")
a = p.parse_args()
root = Path(a.out).resolve()
root.mkdir(parents=True, exist_ok=True)
cases = ([("long-sequence", f.long_participants())] if a.long_participants else
         [("dagre", f.fixture()), ("elk", f.fixture(layout="elk")), ("sequence", f.fixture(True))])
for name, m in cases:
    w = root / name
    if w.exists():
        raise SystemExit("Use a fresh --out directory to preserve earlier evidence: " + str(w))
    w.mkdir()
    v = m["views"][0]
    if not a.long_participants:
        # Punctuation probes exercise literal escaping rather than just simple IDs.
        m["entities"][2]["name"] = 'Work & records "intake" [EU]'
        m["entities"][1]["name"] = "Processing [EU] & audit | zone"
        m["relationships"][0]["intent"] = "Submit [work] & audit | record"
        m["views"][0]["title"] += ' [EU] & "review"'
        m["views"][0]["alternative"] += ' Literal [work] & "audit" | records.'
        ids = {"User": "user", "Zone": "CAP-01", "Worker": "S-01", "Store": "D-01",
               "Submit": "R-01", "Write": "C-01", "Reply": "reply", "Workflow": "view-01"}
        for e in m["entities"]:
            e["id"] = ids[e["id"]]
            if e["parent"]: e["parent"] = ids[e["parent"]]
        for r in m["relationships"]:
            for k in ("id", "from", "to"): r[k] = ids[r[k]]
        v = m["views"][0]
        v["id"] = ids[v["id"]]
        for k in ("entities", "relationships"): v[k] = [ids[i] for i in v[k]]
        if a.qualifiers:
            m["status"] = "provisional"
            v["state"] = "transition"
            m["entities"][1]["state"] = "transition"
            m["entities"][2].update(state="target", qualifier="proposed")
            m["entities"][2]["owner"] = f.atom("unknown", "unknown")
            m["entities"][2]["attributes"] = [dict(label="Restriction", **f.atom("EU only"))]
            m["entities"][1]["attributes"] = [dict(label="Gate", **f.atom("Review first", "proposed"))]
            for r in m["relationships"]: r["state"] = "transition"
            m["relationships"][0]["protocol"] = f.atom("unknown", "unknown")
            m["relationships"][0]["data"] = f.atom("No personal data", "proposed")
    f.stage(w, m)
    with (w / "inputs/facts.md").open("a") as out:
        out.write("\n# Authoritative punctuation/ID probe overrides\n" +
                  json.dumps({"entities": m["entities"], "relationships": m["relationships"]}) + "\n")
    for tool, args in (("tools/compile", []),
                       ("tools/render", ["--outer-cage", "--runtime", a.runtime, "--node", a.node, "--browser", a.browser]),
                       ("bin/check", ["--rendered"])):
        result = f.run(w, tool, *args)
        (w / (Path(tool).name + ".log")).write_text(result.stdout)
        print(name, tool, result.returncode, result.stdout.strip())
        if result.returncode:
            raise SystemExit(result.returncode)
    r = json.loads((w / "output/render.json").read_text())
    svg = ET.parse(w / "output/view-01.svg").getroot()
    ns = {"s": "http://www.w3.org/2000/svg"}
    texts = ["".join(el.itertext()) for el in svg.findall(".//s:text", ns)]
    normalize = lambda s: re.sub(r"\s+", "", s)
    visible = normalize(" ".join(texts))
    graph = svg.find("s:svg", ns)
    graph_text = normalize(" ".join("".join(el.itertext()) for el in graph.findall(".//s:text", ns)))
    frame_text = " ".join(" ".join("".join(el.itertext()) for el in svg.findall("s:text", ns)).split())
    expected = ['Work & records "intake" [EU]', 'Processing [EU] & audit | zone',
                'Submit [work] & audit | record (request) | HTTPS | Data: Work record']
    frame_expected = ['Owner (all elements): Service operations']
    if a.qualifiers:
        expected[2:] = ["Protocol: unknown | Data: No personal data (proposed)",
                        "service / proposed / target"]
        frame_expected = ['Work & records "intake" [EU] [S-01] owner: unknown',
                          "Processing [EU] & audit | zone [CAP-01] Gate: Review first (proposed)"]
        if v["type"] == "sequence":
            frame_expected.append('Work & records "intake" [EU] [S-01] Restriction: EU only')
        else:
            expected.append("Restriction: EU only")
    if a.long_participants:
        expected = [e["name"] for e in m["entities"]] + ["service / proposed / target"]
        frame_expected = ["Owner (all elements): Service operations"]
        for e in m["entities"]:
            for attr in e["attributes"]:
                value = attr["value"] + (" (proposed)" if attr["qualifier"] == "proposed" else "")
                literal = e["name"] + " [" + e["id"] + "] " + attr["label"] + ": " + value
                frame_expected.append(literal)
                assert normalize(attr["label"] + ": " + value) not in graph_text, literal
        assert len(graph.findall(".//s:rect[@class='actor actor-top']", ns)) == 3
        # Production renderer measures every header against actual actor boxes;
        # additionally discriminate unexpected text, not just aggregate presence.
        for e in m["entities"]:
            if e["kind"] == "boundary":
                continue
            name_id = "n_" + e["id"]
            box = graph.find(".//s:rect[@class='actor actor-top'][@name='" + name_id + "']", ns)
            center = float(box.get("x")) + float(box.get("width")) / 2
            headers = [el for el in graph.findall(".//s:text", ns)
                       if "actor" in el.get("class", "").split() and float(el.get("x")) == center]
            label = e["name"] + " " + e["kind"]
            label += " / proposed / target" if e["id"] == "Worker" else " / current"
            assert normalize(" ".join("".join(el.itertext()) for el in headers)) == normalize(label) * 2
    for literal in expected:
        assert normalize(literal) in graph_text, (name, "graph", literal, texts)
    for literal in frame_expected:
        assert " ".join(literal.split()) in frame_text, (name, "frame", literal, texts)
    assert not re.search(r"&#\d+;|&#x[0-9a-f]+;|#\d+;", " ".join(texts), re.I)
    assert svg.find("s:title", ns).text == v["title"]
    assert svg.find("s:desc", ns).text == v["alternative"] + " " + " ".join(v["caveats"])
    assert "@keyframes" not in (w / "output/view-01.svg").read_text()
    assert "Service operations [supplied]" in (w / "output/view-01.alt.md").read_text()
    (w / "literal-results.json").write_text(json.dumps({"expected_graph": expected, "expected_frame": frame_expected, "visible_text": texts,
        "metadata": "exact", "dimensions": r["views"][0]["png"]["dimensions"]}, indent=2) + "\n")
    print(name, "literal DOM/metadata assertions passed; dimensions", r["views"][0]["png"]["dimensions"])
print(str(len(cases)) + " real render cases completed. Independent pixel quality not assessed by this suite.")
