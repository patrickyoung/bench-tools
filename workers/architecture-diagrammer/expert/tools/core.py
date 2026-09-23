#!/usr/bin/env -S python3 -I
"""Reviewed pure contract/compiler. No subprocess, network or artifact imports."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import struct
import xml.etree.ElementTree as ET
import zlib

HOME = Path(__file__).resolve().parent.parent
MIB = 1024 * 1024
SEQUENCE_ACTOR_WIDTH = 240
QUAL = {"supplied", "proposed", "unknown"}
STATES = {"current", "transition", "target", "unspecified"}
KINDS = {"capability", "application", "platform", "person", "service",
         "datastore", "queue", "boundary", "node"}
KEY = ("Boxes: named elements; rounded: people; cylinder: data; outlined group: boundary. "
       "Arrow points to receiver; label gives intent and message type. "
       "Unmarked facts are supplied, not independently verified. Proposed and unknown are explicit; "
       "state follows the view unless labeled otherwise. No approval implied.")
class Reject(ValueError):
    pass

def need(ok, message):
    if not ok:
        raise Reject(message)

def sha(data):
    return hashlib.sha256(data).hexdigest()

def dump(obj):
    return (json.dumps(obj, ensure_ascii=False, indent=2, allow_nan=False) + "\n").encode()

def closed(obj, keys):
    need(type(obj) is dict and set(obj) == set(keys.split()), "closed object: " + keys)

def arr(v, limit=256):
    need(type(v) is list and len(v) <= limit, "invalid/bounded list")
    return v

def enum(v, values):
    need(type(v) is str and v in values, "unsupported value: " + repr(v))

def text(v, limit=240):
    need(type(v) is str and 0 < len(v) <= limit and v == v.strip(), "invalid text")
    need(all(c.isprintable() and c not in "<>{}\\`" for c in v), "unsafe/control text")
    need(not any(x in v for x in ("%%", "://", "javascript:", "data:")), "directive or URL")
    return v

def ident(v):
    need(type(v) is str and re.fullmatch(r"[A-Za-z][A-Za-z0-9_-]{0,63}", v), "invalid ID")
    return v

def path(v):
    need(type(v) is str and len(v) <= 240 and
         re.fullmatch(r"[A-Za-z0-9_. /-]+", v) and
         all(s not in ("", ".", "..") for s in v.split("/")), "noncanonical path")
    return v

def read(p, limit=MIB):
    p = Path(p)
    # The controller must freeze a workspace during checking. Reject links in
    # every component; O_NOFOLLOW and fstat also protect the final open.
    for a in [p] + list(p.parents):
        need(not a.is_symlink(), "symlink: " + str(a))
    st = p.lstat()
    need(stat.S_ISREG(st.st_mode) and st.st_nlink == 1, "nonregular/hardlinked file")
    need(0 < st.st_size <= limit, "file size bound: " + str(p))
    fd = os.open(str(p), os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0))
    try:
        now = os.fstat(fd)
        need((now.st_dev, now.st_ino, now.st_size, now.st_nlink) ==
             (st.st_dev, st.st_ino, st.st_size, 1), "file changed during read")
        with os.fdopen(fd, "rb", closefd=False) as f:
            data = f.read(limit + 1)
        need(len(data) == st.st_size, "file changed/too large")
        return data
    finally:
        os.close(fd)

def tree(root, output=False):
    root = Path(root)
    need(root.is_dir() and not root.is_symlink(), "missing/linked directory")
    found, count, total = {}, 0, 0
    def walk(d, depth):
        nonlocal count, total
        need(depth <= 8, "directory depth")
        with os.scandir(d) as it:
            for e in it:
                count += 1
                need(count <= 256, "too many filesystem entries")
                rel = e.path[len(str(root)) + 1:]
                path(rel)
                st = e.stat(follow_symlinks=False)
                if stat.S_ISDIR(st.st_mode):
                    need(not output, "output must be flat")
                    walk(Path(e.path), depth + 1)
                else:
                    cap = 32 * MIB if output and rel.endswith(".png") else (
                        8 * MIB if output and rel.endswith(".svg") else MIB)
                    b = read(Path(e.path), cap)
                    total += len(b)
                    need(total <= (192 if output else 64) * MIB, "total byte bound")
                    found[rel] = b
        need(len(found) <= (64 if output else 128), "too many files")
    walk(root, 0)
    return dict(sorted(found.items()))

def load(data):
    def pairs(rows):
        out = {}
        for k, v in rows:
            need(k not in out, "duplicate JSON key")
            out[k] = v
        return out
    def bad(v):
        raise Reject("nonfinite JSON: " + v)
    try:
        return json.loads(data.decode("utf-8"), object_pairs_hook=pairs, parse_constant=bad)
    except (UnicodeError, json.JSONDecodeError, RecursionError) as e:
        raise Reject("malformed JSON") from e

def snapshot(work):
    request = read(work / "request.md")
    request.decode("utf-8")
    inputs = tree(work / "inputs")
    return {"request_sha256": sha(request), "profile_sha256": sha(read(HOME / "PROFILE.md")),
            "definition_sha256": definition_hash(),
            "inputs": [{"path": "inputs/" + p, "sha256": sha(b)} for p, b in inputs.items()]}

def definition_hash():
    # Explicit list: checker never recursively imports or executes artifacts.
    names = ["AGENTS.md", "PROFILE.md", "CONTRACT.md", "QUALITY.md", "README.md", "package.json", "bin/check",
             "tools/core.py", "tools/compile", "tools/render", "tools/render.mjs",
             "skills/architecture-views/SKILL.md", "skills/diagram-review/SKILL.md"]
    rows = []
    for n in names:
        data = read(HOME / n)
        data.decode("utf-8")  # Immutable source/profile corruption is infrastructure failure.
        rows.append({"path": n, "sha256": sha(data)})
    return sha(dump(rows))

def sources(rows, allowed):
    need(0 < len(arr(rows, 8)), "source required")
    for r in rows:
        closed(r, "path locator")
        path(r["path"])
        need(r["path"] in allowed, "unstaged source")
        text(r["locator"], 160)

def atom(a, allowed):
    closed(a, "value qualifier sources")
    text(a["value"], 96)
    enum(a["qualifier"], QUAL)
    sources(a["sources"], allowed)
    need((a["value"] == "unknown") == (a["qualifier"] == "unknown"),
         "unknown atom must use value unknown and qualifier unknown")

def validate(m, snap):
    closed(m, "schema status scope summary next_action questions entities relationships views omissions")
    need(m["schema"] == "architecture-diagram/v1", "schema version")
    enum(m["status"], {"prepared", "provisional", "needs-input"})
    for k in ("scope", "summary", "next_action"):
        text(m[k], 480)
    for q in arr(m["questions"], 3):
        text(q)
    allowed = {"request.md"} | {r["path"] for r in snap["inputs"]}
    es, rs, vs = {}, {}, {}
    for e in arr(m["entities"], 96):
        closed(e, "id name kind abstraction state parent qualifier sources owner attributes")
        ident(e["id"])
        need(e["id"] not in es, "duplicate entity")
        es[e["id"]] = e
        text(e["name"], 64)
        enum(e["kind"], KINDS)
        enum(e["abstraction"], {"business", "logical", "physical"})
        enum(e["state"], STATES)
        enum(e["qualifier"], QUAL)
        sources(e["sources"], allowed)
        atom(e["owner"], allowed)
        seen = set()
        for a in arr(e["attributes"], 4):
            closed(a, "label value qualifier sources")
            text(a["label"], 24)
            need(a["label"] not in seen, "duplicate attribute")
            seen.add(a["label"])
            atom({k: a[k] for k in ("value", "qualifier", "sources")}, allowed)
        need(e["kind"] != "capability" or e["abstraction"] == "business", "capability is business")
        need(e["kind"] != "node" or e["abstraction"] == "physical", "node is physical")
        need(e["kind"] not in {"application", "service", "datastore", "queue"} or
             e["abstraction"] != "business", "contradictory kind/abstraction")
        need(e["state"] != "current" or e["qualifier"] != "proposed", "proposed is not existing current")
    for e in es.values():
        p, visited = e["parent"], {e["id"]}
        while p is not None:
            need(type(p) is str and p in es and p not in visited, "dangling/cyclic containment")
            need(es[p]["kind"] == "boundary", "parent must be boundary")
            need(es[p]["abstraction"] == e["abstraction"], "containment abstraction mismatch")
            need(es[p]["state"] in {e["state"], "unspecified", "transition"}, "containment state mismatch")
            visited.add(p)
            p = es[p]["parent"]
    for r in arr(m["relationships"], 192):
        closed(r, "id from to intent kind protocol data state qualifier sources")
        ident(r["id"])
        need(r["id"] not in rs and r["id"] not in es, "duplicate ID")
        rs[r["id"]] = r
        need(r["from"] in es and r["to"] in es, "missing endpoint")
        need(es[r["from"]]["kind"] != "boundary" and es[r["to"]]["kind"] != "boundary",
             "boundary is containment, not an endpoint")
        text(r["intent"], 72)
        enum(r["kind"], {"request", "response", "async", "dependency", "flow"})
        enum(r["state"], STATES)
        enum(r["qualifier"], QUAL)
        need(r["state"] != "current" or r["qualifier"] != "proposed", "proposed current edge")
        sources(r["sources"], allowed)
        atom(r["protocol"], allowed)
        atom(r["data"], allowed)
        for x in (r["from"], r["to"]):
            need(r["state"] in {"transition", "unspecified"} or
                 es[x]["state"] in {r["state"], "unspecified"}, "edge/endpoint state mismatch")
    for v in arr(m["views"], 8):
        closed(v, "id title audience caption alternative caveats key type state abstraction direction layout entities relationships")
        ident(v["id"])
        need(v["id"] not in vs and v["id"] not in es and v["id"] not in rs, "duplicate ID")
        vs[v["id"]] = v
        for k in ("title", "audience", "caption", "alternative", "key"):
            text(v[k], 480 if k == "alternative" else 180)
        for c in arr(v["caveats"], 5):
            text(c, 180)
        enum(v["type"], {"flowchart", "sequence"})
        enum(v["state"], STATES)
        enum(v["abstraction"], {"business", "logical", "physical"})
        enum(v["direction"], {"LR", "TB"})
        enum(v["layout"], {"dagre", "elk"})
        need(v["type"] != "sequence" or (v["direction"] == "LR" and v["layout"] == "dagre"),
             "sequence uses fixed LR/dagre option, not ELK")
        for field, table, cap in (("entities", es, 18), ("relationships", rs, 32)):
            ids = arr(v[field], cap)
            need(all(type(i) is str and i in table for i in ids), "dangling view reference")
            need(len(ids) == len(set(ids)), "duplicate selected ID")
        need(len(v["entities"]) > 0, "empty view")
        need(v["type"] != "sequence" or 0 < len(v["relationships"]) <= 16, "sequence message bound")
        chosen = set(v["entities"])
        for i in chosen:
            e = es[i]
            need(e["abstraction"] == v["abstraction"], "mixed logical/physical abstraction")
            need(v["state"] == "transition" or e["state"] in {v["state"], "unspecified"},
                 "accidental state mixture")
            need(e["parent"] is None or e["parent"] in chosen, "omitted containing boundary")
        for i in v["relationships"]:
            r = rs[i]
            need({r["from"], r["to"]} <= chosen, "selected edge endpoint omitted")
            need(v["state"] == "transition" or r["state"] in {v["state"], "unspecified"},
                 "mixed relationship state")
            need(v["type"] != "sequence" or r["kind"] in {"request", "response", "async"},
                 "sequence needs explicit message semantics")
        uncertain = any(es[i]["qualifier"] != "supplied" or es[i]["owner"]["qualifier"] == "unknown"
                        for i in chosen) or any(rs[i]["qualifier"] != "supplied" or
                        any(rs[i][k]["qualifier"] == "unknown" for k in ("protocol", "data"))
                        for i in v["relationships"])
        need(not uncertain or v["caveats"], "qualified view needs visible caveat")
        need(m["status"] != "provisional" or v["caveats"], "provisional caveat missing")
    covered = {i for v in vs.values() for k in ("entities", "relationships") for i in v[k]}
    omitted = set()
    for o in arr(m["omissions"], 288):
        closed(o, "id rationale")
        need(type(o["id"]) is str and o["id"] in set(es) | set(rs), "unknown omission")
        need(o["id"] not in omitted and o["id"] not in covered, "duplicate/covered omission")
        text(o["rationale"])
        omitted.add(o["id"])
    need(covered | omitted == set(es) | set(rs), "unaccounted chosen-scope fact")
    if m["status"] == "needs-input":
        need(1 <= len(m["questions"]) <= 3 and not es and not rs and not vs and not omitted,
             "needs-input: questions only, no purported complete diagram")
    else:
        need(vs and es, "diagram required")
        need(m["status"] != "prepared" or not m["questions"], "prepared cannot have blocking questions")
        # Unknown source qualifiers cannot silently look complete.
        encoded = json.dumps([m["entities"], m["relationships"]])
        need(m["status"] != "prepared" or '"qualifier": "unknown"' not in encoded,
             "unknown facts require provisional status")
    return es, rs

def esc(s):
    # Mermaid's documented decimal entities, not HTML or raw grammar tokens.
    return "".join(c if c.isalnum() or c in " .,_/-" else "#%d;" % ord(c) for c in s)

def flow_literal(s):
    # CLI 11.17.0 double-encodes decimal entities in edge/cluster labels
    # when htmlLabels is false. Quoted grammar safely carries these literals;
    # quote/entity syntax cannot be represented faithfully here, so reject it.
    need(not any(x in s for x in ('"', '#')) and not re.search(r"&[A-Za-z0-9]+;", s),
         "unrepresentable flow edge/boundary label: quote or entity; shorten faithfully")
    return s

def atom_label(a):
    return a["value"] + " [" + a["qualifier"] + "]"

def entity_lines(e):
    lines = [e["name"], e["id"] + " | " + e["kind"] + " | " + e["state"] + " | " + e["qualifier"],
             "Owner: " + atom_label(e["owner"])]
    lines += [a["label"] + ": " + atom_label(a) for a in e["attributes"]]
    return lines

def edge_label(r):
    return (r["id"] + " " + r["intent"] + " [" + r["kind"] + "; " + r["state"] + "; " +
            r["qualifier"] + "] | Protocol: " + atom_label(r["protocol"]) +
            " | Data: " + atom_label(r["data"]))

def alias(i):
    # Disjoint prefixes avoid collisions with authoritative underscores.
    return "h_" + i.encode("ascii").hex() if "-" in i else "n_" + i

def compact_atom(a):
    return a["value"] + (" (proposed)" if a["qualifier"] == "proposed" else "")

def marks(row, view):
    out = []
    if row["qualifier"] != "supplied":
        out.append(row["qualifier"])
    if row["state"] != view["state"] or row["state"] == "unspecified":
        out.append(row["state"])
    return out

def figure_key(v):
    if v["type"] == "flowchart":
        return KEY
    return ("Boxes: participants; shaded group: boundary; vertical lines: lifelines. "
            "Arrow points to receiver; numbered messages give scenario order. " +
            KEY[KEY.index("Unmarked facts"):])

def visible_entity(e, v, es):
    name = e["name"]
    if sum(es[i]["name"] == name for i in v["entities"]) > 1:
        name += " [" + e["id"] + "]"
    suffix = marks(e, v)
    if e["kind"] == "boundary":
        return [name + (" (" + ", ".join(suffix) + ")" if suffix else "")]
    lines = [name, " / ".join([e["kind"]] + suffix)]
    if v["type"] == "flowchart":
        lines += [a["label"] + ": " + compact_atom(a) for a in e["attributes"]]
    return lines

def visible_edge(r, v):
    suffix = marks(r, v)
    return (r["intent"] + " (" + ", ".join([r["kind"]] + suffix) + ")"
            + " | " + ("Protocol: unknown" if r["protocol"]["qualifier"] == "unknown"
                        else compact_atom(r["protocol"]))
            + " | Data: " + compact_atom(r["data"]))

def frame_notes(v, es):
    chosen = [es[i] for i in v["entities"]]
    owners = {(e["owner"]["value"], e["owner"]["qualifier"]) for e in chosen}
    notes = []
    if len(owners) == 1:
        notes.append("Owner (all elements): " + compact_atom(chosen[0]["owner"]))
    else:
        notes += [e["name"] + " [" + e["id"] + "] owner: " + compact_atom(e["owner"])
                  for e in chosen]
    for e in chosen:
        if e["kind"] == "boundary" or v["type"] == "sequence":
            notes += [e["name"] + " [" + e["id"] + "] " + a["label"] + ": " + compact_atom(a)
                      for a in e["attributes"]]
    return notes

def visible_labels(v, es, rs):
    return ([" ".join(visible_entity(es[i], v, es)) for i in v["entities"]] +
            [visible_edge(rs[i], v) for i in v["relationships"]])

def sequence_header(lines):
    # Conservative advance estimates for admitted Arial 16px, with 20px padding
    # on each side. Actual browser bounds remain authoritative (font fallback
    # can differ). Wrap before escaping so entity syntax is never split.
    budget = SEQUENCE_ACTOR_WIDTH - 40

    def advance(char):
        if char in " ilI.,'!:;|":
            return 5
        if char in "MWmw@%":
            return 16
        if "A" <= char <= "Z":
            return 13
        if "a" <= char <= "z" or "0" <= char <= "9":
            return 10
        return 16

    def width(value):
        return sum(advance(char) for char in value)

    wrapped = []
    for value in lines:
        line = ""
        for word in value.split():
            if line and width(line + " " + word) > budget:
                wrapped.append(line)
                line = ""
            # Preserve every character of unusually long words/IDs as well.
            while width(word) > budget:
                end = 1
                while end < len(word) and width(word[:end + 1]) <= budget:
                    end += 1
                wrapped.append(word[:end])
                word = word[end:]
            line = line + " " + word if line else word
        if line:
            wrapped.append(line)
    return "<br/>".join(map(esc, wrapped))

def compile_view(m, v, es, rs):
    title = v["title"] + " | " + v["state"] + " | " + m["status"]
    lines = [v["type"] + (" " + v["direction"] if v["type"] == "flowchart" else "Diagram")]
    # sequenceDiagram is a single keyword, flowchart has its direction.
    lines += ["accTitle: " + title, "accDescr: " + v["alternative"]]
    if v["type"] == "flowchart":
        def emit(parent, indent):
            for i in v["entities"]:
                e = es[i]
                if e["parent"] != parent:
                    continue
                label = flow_literal(" ".join(visible_entity(e, v, es))) if e["kind"] == "boundary" else "<br/>".join(map(esc, visible_entity(e, v, es)))
                n = alias(i)
                if e["kind"] == "boundary":
                    lines.append(indent + 'subgraph ' + n + '["' + label + '"]')
                    emit(i, indent + "  ")
                    lines.append(indent + "end")
                else:
                    shape = ('(["', '"])') if e["kind"] == "person" else (
                        ('[("', '")]') if e["kind"] == "datastore" else ('["', '"]'))
                    lines.append(indent + n + shape[0] + label + shape[1])
                    lines.append(indent + "class " + n + " " + e["qualifier"])
        emit(None, "  ")
        for i in v["relationships"]:
            r = rs[i]
            lines.append('  ' + alias(r["from"]) + ' -->|"' + flow_literal(visible_edge(r, v)) + '"| ' + alias(r["to"]))
        lines += ["  classDef supplied fill:#f1f5f9,stroke:#334155,color:#172b4d",
                  "  classDef proposed fill:#ecfdf5,stroke:#0f766e,stroke-dasharray:5 3,color:#172b4d",
                  "  classDef unknown fill:#fffbeb,stroke:#92400e,stroke-dasharray:2 3,color:#172b4d"]
    else:
        # Box grouping preserves one-level trust/deployment boundaries; nested
        # sequence boxes are unsupported and rejected by compilation.
        def participant(i):
            e = es[i]
            lines.append("participant " + alias(i) + " as " + sequence_header(visible_entity(e, v, es)))
        for i in v["entities"]:
            e = es[i]
            if e["kind"] == "boundary":
                need(e["parent"] is None, "nested sequence boundaries unsupported; split view")
                lines.append("box rgb(241,245,249) " + esc(" ".join(visible_entity(e, v, es))))
                for j in v["entities"]:
                    if es[j]["parent"] == i:
                        need(es[j]["kind"] != "boundary", "nested sequence boundary")
                        participant(j)
                lines.append("end")
            elif e["parent"] is None:
                participant(i)
        lines.append("autonumber")
        for i in v["relationships"]:
            r = rs[i]
            arrow = {"request": "->>", "response": "-->>", "async": "-)"}[r["kind"]]
            lines.append(alias(r["from"]) + arrow + alias(r["to"]) + ": " + esc(visible_edge(r, v)))
    return ("\n".join(lines) + "\n").encode()

def products(m, snap):
    es, rs = validate(m, snap)
    report = ["# " + m["status"], "", m["summary"], "", "Scope: " + m["scope"],
              "", "Next action: " + m["next_action"], ""]
    report += ["- " + q for q in m["questions"]]
    report += ["", "Structural preparation only; not independent review, approval or publication readiness."]
    out = {"report.md": ("\n".join(report) + "\n").encode()}
    if m["status"] == "needs-input":
        return out
    coverage = ["# Cross-view coverage", "", "All modeled facts are selected or explicitly omitted.",
                "Source completeness is an independent semantic review, not a count guarantee.", ""]
    for i in list(es) + list(rs):
        views = [v["id"] for v in m["views"] if i in v["entities"] + v["relationships"]]
        omission = next((o["rationale"] for o in m["omissions"] if o["id"] == i), "")
        coverage.append("- " + i + ": " + (", ".join(views) if views else "OMITTED: " + omission))
    out["coverage.md"] = ("\n".join(coverage) + "\n").encode()
    for n, v in enumerate(m["views"], 1):
        base = "view-%02d" % n
        out[base + ".mmd"] = compile_view(m, v, es, rs)
        alt = [v["title"], v["alternative"], "Scope: " + m["scope"],
               "Audience: " + v["audience"], "State: " + v["state"], "Status: " + m["status"],
               v["caption"], figure_key(v), v["key"]] + v["caveats"]
        for i in v["entities"]:
            alt.append(" | ".join(entity_lines(es[i])))
            alt.append("Full entity facts/provenance: " + json.dumps(es[i], ensure_ascii=False))
        for i in v["relationships"]:
            alt.append(rs[i]["from"] + " -> " + rs[i]["to"] + ": " + edge_label(rs[i]))
            alt.append("Full relationship facts/provenance: " + json.dumps(rs[i], ensure_ascii=False))
        out[base + ".alt.md"] = ("\n\n".join(alt) + "\n").encode()
    return out

def manifest(snap, files):
    return dict(schema="architecture-package/v1", stage="authored", **snap,
                artifacts=[{"path": n, "sha256": sha(b)} for n, b in sorted(files.items())])

def config(v):
    return {"securityLevel": "strict", "startOnLoad": False, "theme": "base",
            "layout": v["layout"], "look": "classic", "htmlLabels": False,
            "fontFamily": "Arial, sans-serif",
            "themeCSS": ".edgeLabel rect { opacity:1 !important; fill:#ffffff !important; } "
                        ".cluster-label text { paint-order:stroke; stroke:#f8fafc; stroke-width:4px; stroke-linejoin:round; } "
                        ".messageText { paint-order:stroke; stroke:#ffffff; stroke-width:4px; stroke-linejoin:round; }",
            "themeVariables": {"fontFamily": "Arial, sans-serif", "fontSize": "16px",
                              "primaryColor": "#f1f5f9", "primaryTextColor": "#172b4d",
                              "primaryBorderColor": "#334155", "lineColor": "#475569",
                              "secondaryColor": "#ecfdf5", "tertiaryColor": "#fffbeb",
                              "clusterBkg": "#f8fafc", "clusterBorder": "#cbd5e1",
                              "actorBkg": "#f1f5f9", "actorTextColor": "#172b4d",
                              "signalColor": "#334155", "signalTextColor": "#172b4d"},
            "flowchart": {"htmlLabels": False, "curve": "linear", "nodeSpacing": 40,
                          "rankSpacing": 65, "padding": 18, "wrappingWidth": 240},
            "sequence": {"useMaxWidth": False, "wrap": True, "width": SEQUENCE_ACTOR_WIDTH,
                         "actorMargin": 65, "messageMargin": 55}}

def safe_css(value):
    need(not re.search(r"\\|@|(?:https?|file|data|javascript):|//|image-set|expression|behavior|-moz-binding",
                       value, re.I), "external/active SVG CSS")
    for target in re.findall(r"url\(([^)]*)\)", value, re.I):
        target = target.strip().strip("'\\\"")
        need(re.fullmatch(r"#[A-Za-z0-9_-]+", target), "external SVG URL")

def image_dimensions(data, ext):
    if ext == "png":
        need(data[:8] == b"\x89PNG\r\n\x1a\n", "PNG signature")
        pos, dims, ended, idat = 8, None, False, False
        while pos < len(data):
            need(pos + 12 <= len(data), "truncated PNG")
            n = struct.unpack(">I", data[pos:pos+4])[0]
            kind = data[pos+4:pos+8]
            end = pos + 12 + n
            need(end <= len(data), "PNG chunk bound")
            chunk = data[pos+8:pos+8+n]
            need(zlib.crc32(kind + chunk) & 0xffffffff == struct.unpack(">I", data[end-4:end])[0],
                 "PNG CRC")
            if pos == 8:
                need(kind == b"IHDR" and n == 13, "PNG IHDR")
                dims = struct.unpack(">II", chunk[:8])
            need(kind in {b"IHDR", b"IDAT", b"IEND", b"sRGB", b"gAMA", b"cHRM", b"pHYs"}, "PNG unexpected chunk")
            idat |= kind == b"IDAT"
            pos = end
            if kind == b"IEND":
                need(n == 0 and pos == len(data), "PNG trailing data")
                ended = True
                break
        need(ended and idat, "incomplete PNG")
    else:
        s = data.decode("utf-8")
        need(not re.search(r"<!|<\?", s, re.I),
             "unsafe SVG resource/declaration")
        try:
            root = ET.fromstring(s)
        except ET.ParseError as e:
            raise Reject("malformed SVG") from e
        need(root.tag == "{http://www.w3.org/2000/svg}svg", "SVG root")
        allowed = {"svg", "g", "path", "rect", "circle", "ellipse", "line", "polyline",
                   "polygon", "text", "tspan", "defs", "marker", "style", "title", "desc",
                   "symbol", "clipPath", "linearGradient", "stop", "filter", "feDropShadow"}
        for el in root.iter():
            need(el.tag.startswith("{http://www.w3.org/2000/svg}") and
                 el.tag.rsplit("}", 1)[-1] in allowed, "unsafe SVG element")
            for k, v in el.attrib.items():
                need(not k.lower().startswith("on") and not k.endswith("href") and
                     not k.startswith("{"), "SVG callback/link/namespace attribute")
                if k == "style" or "url" in v.lower():
                    safe_css(v)
            if el.tag.endswith("}style"):
                safe_css(el.text or "")
        try:
            dims = tuple(int(root.attrib[k]) for k in ("width", "height"))
        except (ValueError, KeyError) as e:
            raise Reject("SVG integer dimensions") from e
        need(root.attrib.get("viewBox") == "0 0 %d %d" % dims, "SVG viewBox")
        need(any(e.tag.endswith("}title") and e.text for e in root) and
             any(e.tag.endswith("}desc") and e.text for e in root), "SVG accessibility")
    need(dims and all(200 <= n <= 7000 for n in dims) and dims[0] * dims[1] <= 24000000,
         "image dimensions bound")
    return list(dims)

def check(work, rendered=False):
    snap = snapshot(work)
    files = tree(work / "output", True)
    need("model.json" in files and "manifest.json" in files, "missing model/manifest")
    m = load(files["model.json"])
    expected = {"model.json": files["model.json"], **products(m, snap)}
    for n, b in expected.items():
        need(files.get(n) == b, "model/source mismatch: " + n)
    need(load(files["manifest.json"]) == manifest(snap, expected), "stale/invalid manifest")
    expected_names = set(expected) | {"manifest.json"}
    if "render.json" in files or rendered:
        need(m["status"] != "needs-input", "needs-input cannot render")
        need("render.json" in files, "render stage missing")
        receipt = load(files["render.json"])
        closed(receipt, "schema stage manifest_sha256 renderer_sha256 runtime views")
        need(receipt["schema"] == "architecture-render/v1" and receipt["stage"] == "rendered-unreviewed",
             "render stage")
        need(receipt["manifest_sha256"] == sha(files["manifest.json"]), "stale render manifest")
        need(receipt["renderer_sha256"] == sha(read(HOME / "tools/render.mjs")), "renderer hash")
        rt = receipt["runtime"]
        closed(rt, "path cli mermaid puppeteer node browser cli_sha256 lock_sha256 browser_sha256")
        need(type(rt["path"]) is str and Path(rt["path"]).is_absolute(), "runtime identity")
        need((rt["cli"], rt["mermaid"], rt["puppeteer"]) == ("11.17.0", "11.17.2", "25.11.0"),
             "unsupported runtime")
        for k in ("node", "browser"):
            need(type(rt[k]) is str and 0 < len(rt[k]) < 200, "runtime version")
        for k in ("cli_sha256", "lock_sha256", "browser_sha256"):
            need(type(rt[k]) is str and re.fullmatch("[0-9a-f]{64}", rt[k]), "runtime digest")
        rows = arr(receipt["views"], 8)
        need(len(rows) == len(m["views"]), "render view count")
        for n, (v, r) in enumerate(zip(m["views"], rows), 1):
            closed(r, "id source_sha256 config_sha256 svg png")
            base = "view-%02d" % n
            need(r["id"] == v["id"] and r["source_sha256"] == sha(files[base + ".mmd"]) and
                 r["config_sha256"] == sha(dump(config(v))), "render source/config binding")
            for ext in ("svg", "png"):
                a = r[ext]
                closed(a, "path sha256 dimensions")
                name = base + "." + ext
                expected_names.add(name)
                need(a["path"] == name and name in files and a["sha256"] == sha(files[name]),
                     "render artifact hash")
                need(a["dimensions"] == image_dimensions(files[name], ext), "render dimensions")
            need(r["svg"]["dimensions"] == r["png"]["dimensions"], "SVG/PNG extent differs")
        expected_names.add("render.json")
    need(set(files) == expected_names, "missing/extra output")
    return m, snap, files
