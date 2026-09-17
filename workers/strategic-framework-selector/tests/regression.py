#!/usr/bin/env python3
"""Synthetic structural tests; no models. Usage: python3 tests/regression.py expert"""
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

TOOLS = ["Wardley Mapping", "Cynefin Framework", "SWOT Analysis", "Eisenhower Matrix"]
HEADINGS = ["## Step 1: The Direct Verdict",
            "## Step 2: High-Level Contextual Comparison",
            "## Step 3: Execution Instructions"]
# Independent fixture expectations; do not import validator implementation.
REGIONS = [
    [("genesis", "Genesis", "column-1", "Novel and uncertain market practice."),
     ("custom-built", "Custom-built", "column-2", "Bespoke but understood implementations."),
     ("product", "Product (+rental)", "column-3", "Repeatable offerings bought or rented."),
     ("commodity", "Commodity (+utility)", "column-4", "Standardized interchangeable supply.")],
    [("complex", "Complex", "upper-left", "Emergent causality: probe-sense-respond."),
     ("complicated", "Complicated", "upper-right", "Expert-discoverable causality: sense-analyze-respond."),
     ("clear", "Clear (Simple/Obvious)", "lower-right", "Repeatable causality: sense-categorize-respond."),
     ("chaotic", "Chaotic", "lower-left", "Absent effective constraints: act-sense-respond."),
     ("confused", "Confused/Disorder", "center", "Unresolved diagnosis: partition and investigate.")],
    [("strengths", "Strengths", "upper-left", "Internal helpful capabilities."),
     ("weaknesses", "Weaknesses", "upper-right", "Internal harmful limitations."),
     ("opportunities", "Opportunities", "lower-left", "External helpful conditions."),
     ("threats", "Threats", "lower-right", "External harmful conditions.")],
    [("do", "Do", "upper-right", "High urgency and importance: act on the time consequence."),
     ("schedule", "Schedule", "upper-left", "Low urgency, high importance: protect outcome work."),
     ("delegate", "Delegate", "lower-right", "High urgency, low importance: only delegate with capacity and authority."),
     ("eliminate-defer", "Eliminate/Defer", "lower-left", "Low urgency and importance: remove or revisit.")],
]
AXES = [
    {"mode": "qualitative",
     "x": {"label": "Evolution", "direction": "left-to-right",
           "values": ["Genesis", "Custom-built", "Product (+rental)", "Commodity (+utility)"]},
     "y": {"label": "Visibility to the user", "direction": "bottom-to-top", "values": ["Low", "High"]}},
    {"mode": "no-numeric-axes"},
    {"mode": "categorical",
     "x": {"label": "Effect", "direction": "left-to-right", "values": ["Helpful", "Harmful"]},
     "y": {"label": "Origin", "direction": "top-to-bottom", "values": ["Internal", "External"]}},
    {"mode": "qualitative",
     "x": {"label": "Urgency", "direction": "left-to-right", "values": ["Low", "High"]},
     "y": {"label": "Importance", "direction": "bottom-to-top", "values": ["Low", "High"]}},
]
REQUESTS = [
    "Dispatchers need reliable routing. Our custom dispatch algorithm depends on "
    "standard identity and storage. Engineering needs build/buy guidance; market evidence is incomplete.",
    "The operations team faces a new service disruption with unknown cause; effective "
    "constraints have not been assessed. Decide how to diagnose before acting.",
    "The planning group wants a quick alignment scan. We have experienced support staff; "
    "external market signals are not yet verified.",
    "The delivery team must prioritize this week's tasks. Filing the essential compliance "
    "report is due Friday; missing it blocks operations. Other task deadlines are unknown.",
]
ITEMS = [
    ("dispatch-algorithm", "Dispatch algorithm", "custom-built", "component"),
    ("disruption-diagnosis", "Service disruption diagnosis", "confused", "item"),
    ("support-staff", "Experienced support staff", "strengths", "item"),
    ("compliance-report", "Compliance report — due Friday", "do", "item"),
]
RATIONALES = [
    "Dependency structure and market maturity inform sourcing better than a fast SWOT scan.",
    "Choosing an action mode precedes a SWOT inventory or a time-priority grid.",
    "A quick shared scan fits better than mapping dependencies or diagnosing causal domains.",
    "Time consequence and outcome impact matter before a sourcing map or a SWOT scan.",
]
METRICS = [
    ["Dispatch component evolution", "Diagnostic uncertainty in sourcing", "Quick sourcing inventory", "Sourcing task urgency"],
    ["Supply dependency exposure", "Causal diagnosis of disruption", "Disruption observations", "Response task urgency"],
    ["Support ecosystem maturity", "Uncertainty in market signals", "Internal capability vs external signals", "Scan preparation urgency"],
    ["Compliance tooling maturity", "Uncertainty in compliance process", "Compliance capability scan", "Deadline consequence vs outcome impact"],
]
OUTPUTS = [
    ["Separate algorithm from enabling services", "Choose sourcing evidence investigation", "List candidate sourcing considerations", "Sequence sourcing evidence tasks"],
    ["Locate supply dependency failures", "Choose bounded diagnosis before intervention", "Collect disruption observations", "Prioritize stabilization tasks"],
    ["Map support supply options", "Choose market learning approach", "Validate support advantage hypotheses", "Schedule scan preparation"],
    ["Map compliance dependencies", "Choose compliance diagnosis approach", "List compliance planning considerations", "Act on Friday report and protect important work"],
]


def element(identifier, label, region, role="item", wardley=False):
    item = dict(id=identifier, label=label, role=role, placement={"region": region},
                basis="supplied", evidence="Named in the synthetic request.",
                reason="Conditional qualitative placement based on the described decision.",
                uncertain=True)
    if wardley:
        item["placement"]["visibility"] = "medium" if role == "component" else "high"
    return item


def edge(identifier, source, target, kind="dependency"):
    return dict(id=identifier, **{"from": source, "to": target}, type=kind,
                direction="source-to-target", meaning="Supplied reliance of source on target.",
                style="solid", uncertain=False, timing=None)


def fixture(index, intake=False):
    framework = TOOLS[index]
    request = b"" if intake else (REQUESTS[index] + "\n").encode()
    choices = [index] + [i for i in range(4) if i != index][:2]
    rows = [
        "| Tool | Primary Metric | Strategic Setup Time | Actionable Output |",
        "| --- | --- | --- | --- |",
    ]
    for i in choices:
        metric = "Potential intake dimension" if intake else METRICS[index][i]
        output = "Conditional template after scenario clarification" if intake else OUTPUTS[index][i]
        rows.append("| {} | {} | {} | {} |".format(TOOLS[i], metric,
                                                 "High" if i == 0 else "Low", output))
    question = "What evidence would confirm this placement?"
    steps = [
        "1. Use the proposed landscape layout and the named regions in the brief; retain empty regions.",
        "2. Place the supplied labels provisionally and validate their evidence before acting.",
        "3. Avoid substituting the diagram for the actual decision; revisit after placement evidence arrives.",
        "4. " + question,
    ]
    elements = [element(*ITEMS[index], wardley=index == 0)]
    relationships = []
    if index == 0:
        elements = [
            element("dispatchers", "Dispatchers", None, "user", True),
            element("routing-need", "Reliable routing", None, "need", True),
            elements[0],
            element("identity-service", "Identity service", "commodity", "component", True),
            element("storage-service", "Storage service", "commodity", "component", True),
        ]
        relationships = [
            edge("dispatch-identity", "dispatch-algorithm", "identity-service"),
            edge("dispatch-storage", "dispatch-algorithm", "storage-service"),
        ]
    if intake:
        question = "What scenario, next decision and audience should this template serve?"
        steps = [
            "1. Keep the template unpopulated, with Helpful/Harmful columns and Internal/External rows.",
            "2. Retain Strengths, Weaknesses, Opportunities and Threats without inventing claims.",
            "3. Avoid treating an intake template as a fitted strategy; re-triage when context arrives.",
            "4. " + question,
        ]
        elements, relationships = [], []
    verdict = (framework + " is a provisional intake template, not a fitness conclusion; "
               "the alternatives depend on the missing next decision." if intake else
               framework + " is a provisional fit. " + RATIONALES[index])
    response = ("\n\n".join([HEADINGS[0], verdict, HEADINGS[1], "\n".join(rows),
                              HEADINGS[2], "\n".join(steps)]) + "\n").encode()
    data = {
        "schema": "bench.diagram-brief/v1",
        "request_sha256": hashlib.sha256(request).hexdigest(),
        "response_sha256": hashlib.sha256(response).hexdigest(),
        "framework": framework, "status": "needs-input" if intake else "provisional",
        "title": "Intake template" if intake else ITEMS[index][1] + " decision",
        "audience": "Unspecified" if intake else "Team named in request",
        "decision": "Await scenario" if intake else "Choose next action for the supplied decision",
        "uncertainty": {"assumptions": [], "unknowns": [
            "Scenario and audience missing." if intake else "Placement evidence requires review."]},
        "questions": [question],
        "layout": {"orientation": "landscape", "axes": copy.deepcopy(AXES[index]),
                   "regions": [dict(zip(("id", "label", "position", "definition"), row))
                               for row in REGIONS[index]]},
        "elements": elements, "relationships": relationships,
        "legend": ["Layout is proposed; uncertain items need review.",
                   "Solid dependency arrow A to B means A depends on B; no arrow implies no asserted relation."],
        "traps": ["Do not treat a missing scenario as evidence for SWOT." if intake else
                  "Do not treat " + ITEMS[index][1] + "'s provisional placement as established; verify it."],
        "review": {"next_decision": "Re-triage on evidence",
                   "trigger": "Scenario supplied" if intake else "Placement evidence arrives",
                   "scope": "Only the bounded decision supplied",
                   "transition": "No transition warranted until the missing evidence clarifies the bottleneck."},
    }
    return request, response, data


def replace_at(data, path, value):
    obj = data
    for key in path[:-1]:
        obj = obj[key]
    obj[path[-1]] = value


def patch(path, value):
    return lambda data: replace_at(data, path, value)


def remove(path):
    def mutate(data):
        obj = data
        for key in path[:-1]:
            obj = obj[key]
        del obj[path[-1]]
    return mutate


def main():
    if len(sys.argv) != 2:
        print("usage: regression.py EXPERT", file=sys.stderr)
        return 2
    checker = Path(sys.argv[1]).resolve() / "bin/check"
    if not checker.is_file() or not os.access(checker, os.X_OK):
        print("missing executable bin/check", file=sys.stderr)
        return 2
    passed = failed = 0

    def run(name, index=0, intake=False, mutate=None, response_edit=None,
            disk=None, expect=1):
        nonlocal passed, failed
        request, response, data = fixture(index, intake)
        if mutate:
            mutate(data)
        if response_edit:
            response = response_edit(response.decode()).encode()
            data["response_sha256"] = hashlib.sha256(response).hexdigest()
        with tempfile.TemporaryDirectory(prefix="framework-check-") as temp:
            work = Path(temp)
            (work / "output").mkdir()
            (work / "request.md").write_bytes(request)
            (work / "output/response.md").write_bytes(response)
            (work / "output/diagram-brief.json").write_text(
                json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
            if disk:
                disk(work)
            # Verify the checker did not mutate any regular input/artifact.
            def snapshot():
                return {str(p.relative_to(work)): p.read_bytes()
                        for p in work.rglob("*")
                        if not p.is_symlink() and p.is_file()}
            before = snapshot()
            proc = subprocess.run([str(checker)], cwd=work, input=b"ignored stdout candidate",
                                  capture_output=True, timeout=10)
            okay = proc.returncode == expect and before == snapshot()
            if expect == 1:
                okay = okay and b"invalid:" in proc.stderr and b"Traceback" not in proc.stderr
            if okay:
                passed += 1
                print("PASS " + name)
            else:
                failed += 1
                print("FAIL {}: expected {}, got {}: {}".format(
                    name, expect, proc.returncode, (proc.stdout + proc.stderr).decode(errors="replace")))

    for i in range(4):
        run("positive " + TOOLS[i], index=i, expect=0)
    run("positive needs-input empty request", index=2, intake=True, expect=0)
    run("positive ready for review", index=3, mutate=patch(["status"], "ready"), expect=0)
    run("positive unplaced uncertain item", index=3,
        mutate=patch(["elements", 0, "placement", "region"], None), expect=0)
    run("positive request at 1 MiB", disk=lambda w: pad_request(w), expect=0)

    def movement(data):
        relation = edge("dispatch-evolution", "dispatch-algorithm", "product", "movement")
        relation.update(style="dashed", uncertain=True, timing="uncertain",
                        meaning="Proposed future productization, timing unknown.")
        data["relationships"].append(relation)
    run("positive proposed movement", mutate=movement, expect=0)

    mutations = [
        ("wrong schema", ["schema"], "bench.diagram-brief/v2"),
        ("wrong framework", ["framework"], "PESTLE"),
        ("wrong status", ["status"], "implemented"),
        ("missing uncertainty", ["uncertainty"], None),
        ("missing assumptions", ["uncertainty", "assumptions"], None),
        ("missing unknowns", ["uncertainty", "unknowns"], None),
        ("missing traps", ["traps"], None),
        ("missing review", ["review"], None),
        ("empty trap", ["traps"], [""]),
        ("empty review trigger", ["review", "trigger"], ""),
        ("too many questions", ["questions"], ["a", "b", "c", "d"]),
        ("invalid edge reference", ["relationships", 0, "to"], "nonexistent"),
        ("invalid edge source", ["relationships", 0, "from"], "nonexistent"),
        ("self dependency", ["relationships", 0, "to"], "dispatch-algorithm"),
        ("reversed dependency convention", ["relationships", 0, "direction"], "target-to-source"),
        ("dashed dependency", ["relationships", 0, "style"], "dashed"),
        ("invalid region reference", ["elements", 2, "placement", "region"], "nonexistent"),
        ("duplicate element ID", ["elements", 1, "id"], "dispatchers"),
        ("cross-kind duplicate ID", ["relationships", 0, "id"], "genesis"),
        ("unsafe element ID", ["elements", 0, "id"], "../request.md"),
        ("wrong Wardley y", ["layout", "axes", "y", "label"], "Business value"),
        ("reversed Wardley y", ["layout", "axes", "y", "direction"], "top-to-bottom"),
        ("missing Wardley stage", ["layout", "regions"], []),
        ("unknown basis certain", ["elements", 0, "basis"], "unknown"),
        ("boolean masquerading as string", ["elements", 0, "uncertain"], "true"),
        ("invalid element type", ["elements", 0], []),
        ("wrong questions type", ["questions"], {}),
        ("missing placement reason", ["elements", 0, "reason"], None),
        ("needs-input populated", ["status"], "needs-input"),
    ]
    # The fixture normally marks supplied elements uncertain; make this specific
    # negative exercise the unknown-basis implication rather than another field.
    for name, path, value in mutations:
        if name == "unknown basis certain":
            def unknown_certain(d):
                d["elements"][0].update(basis="unknown", uncertain=False)
            run(name, mutate=unknown_certain)
        else:
            run(name, mutate=remove(path) if value is None else patch(path, value))
    run("Cynefin numeric axes", index=1,
        mutate=patch(["layout", "axes"], {"mode": "numeric", "x": "Urgency", "y": "Importance"}))
    run("Cynefin extra axis", index=1,
        mutate=patch(["layout", "axes", "x"], {"label": "Certainty", "values": [0, 1]}))
    run("Cynefin center missing", index=1,
        mutate=lambda d: d["layout"]["regions"].pop())
    run("SWOT quadrant swap", index=2,
        mutate=patch(["layout", "regions", 0, "position"], "lower-left"))
    run("Eisenhower axis swap", index=3,
        mutate=patch(["layout", "axes", "x", "label"], "Importance"))
    run("intake has no question", index=2, intake=True, mutate=patch(["questions"], []))
    run("intake has no unknown", index=2, intake=True,
        mutate=patch(["uncertainty", "unknowns"], []))
    run("intake has relationship", index=2, intake=True,
        mutate=patch(["relationships"], [edge("fiction", "fiction-a", "fiction-b")]))
    run("ready empty", index=2, intake=True, mutate=patch(["status"], "ready"))
    run("Wardley missing user", mutate=lambda d: d["elements"].pop(0))
    run("movement not dashed", mutate=lambda d: (
        movement(d), d["relationships"][-1].update(style="solid")))
    run("movement certain timing", mutate=lambda d: (
        movement(d), d["relationships"][-1].update(timing="next year")))

    edits = [
        ("extra section", lambda s: s + "\n## Appendix\nAdditional claims\n"),
        ("indented extra section", lambda s: s + "\n  ## Appendix\nExtra\n"),
        ("setext extra section", lambda s: s + "\nAppendix\n---\nExtra\n"),
        ("empty numbered step", lambda s: s.split(HEADINGS[2])[0] + HEADINGS[2] + "\n\n1. \ntext\n"),
        ("missing section", lambda s: s.replace(HEADINGS[2], "Instructions")),
        ("wrong verdict framework", lambda s: s.replace(
            "Wardley Mapping is", "SWOT Analysis is", 1)),
        ("missing table row", lambda s: "\n".join(
            line for line in s.splitlines() if not line.startswith("| SWOT Analysis |"))),
        ("extra table row", lambda s: s.replace(HEADINGS[2],
            "| Eisenhower Matrix | Urgency | Low | Task queue |\n\n" + HEADINGS[2])),
        ("wrong table columns", lambda s: s.replace(
            "| Strategic Setup Time |", "| Time | Extra |")),
        ("duplicate alternative", lambda s: s.replace("| SWOT Analysis |", "| Cynefin Framework |")),
        ("selected table mismatch", lambda s: s.replace("| Wardley Mapping |", "| Eisenhower Matrix |")),
        ("wrong setup time", lambda s: s.replace("| High |", "| 2 hours |")),
        ("missing steps", lambda s: s.replace("1. Use", "- Use")),
        ("nonsequential steps", lambda s: s.replace("2. Place", "8. Place")),
        ("unexpected preamble", lambda s: "Here is the answer\n" + s),
    ]
    for name, edit in edits:
        run(name, response_edit=edit)

    brief_path = "output/diagram-brief.json"
    run("missing response", disk=lambda w: (w / "output/response.md").unlink())
    run("missing brief", disk=lambda w: (w / brief_path).unlink())
    run("stale request", disk=lambda w: (w / "request.md").write_bytes(b"changed\n"))
    run("stale response", disk=lambda w: (w / "output/response.md").write_bytes(b"changed\n"))
    for label, raw in [
        ("malformed JSON", b"{"),
        ("nonfinite NaN", b'{"x":NaN}'),
        ("nonfinite Infinity", b'{"x":Infinity}'),
        ("nonfinite overflow", b'{"x":1e9999}'),
        ("nonobject JSON", b"[]"),
    ]:
        run(label, disk=lambda w, raw=raw: (w / brief_path).write_bytes(raw))
    run("duplicate valid top-level key", disk=lambda w: duplicate_key(w, "title"))
    run("duplicate valid nested key", disk=lambda w: duplicate_key(w, "orientation"))
    for filename, maximum in [("request.md", 1048576),
                              ("output/response.md", 262144), (brief_path, 1048576)]:
        run("oversize " + filename,
            disk=lambda w, f=filename, n=maximum: (w / f).write_bytes(b"x" * (n + 1)))
        run("non-UTF8 " + filename,
            disk=lambda w, f=filename: (w / f).write_bytes(b"\xff"))
        run("symlink " + filename,
            disk=lambda w, f=filename: symlink_file(w, f))
    run("symlink output directory", disk=symlink_output)
    run("request FIFO", disk=lambda w: fifo_request(w))
    print("{} passed, {} failed (synthetic structural tests only)".format(passed, failed))
    return int(bool(failed))


def duplicate_key(work, key):
    path = work / "output/diagram-brief.json"
    source = path.read_text(encoding="utf-8")
    lines = source.splitlines()
    for i, line in enumerate(lines):
        if line.strip().startswith(json.dumps(key) + ":"):
            lines.insert(i, line)
            break
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def pad_request(work):
    path = work / "request.md"
    raw = path.read_bytes()
    raw += b" " * (1048576 - len(raw))
    path.write_bytes(raw)
    brief = work / "output/diagram-brief.json"
    data = json.loads(brief.read_text(encoding="utf-8"))
    data["request_sha256"] = hashlib.sha256(raw).hexdigest()
    brief.write_text(json.dumps(data), encoding="utf-8")


def symlink_file(work, filename):
    original = work / filename
    target = work / "relocated-file"
    original.rename(target)
    original.symlink_to(target)


def symlink_output(work):
    (work / "output").rename(work / "relocated-output")
    (work / "output").symlink_to(work / "relocated-output", target_is_directory=True)


def fifo_request(work):
    (work / "request.md").unlink()
    os.mkfifo(work / "request.md")


if __name__ == "__main__":
    sys.exit(main())
