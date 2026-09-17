#!/usr/bin/env python3
"""Bounded checks for the worker's conservative, portable SVG/PNG subset."""
import sys
sys.dont_write_bytecode = True
import os, json, math, pathlib, re, stat, struct, zlib
import xml.etree.ElementTree as ET

SVG = "http://www.w3.org/2000/svg"
INK = "http://www.inkscape.org/namespaces/inkscape"
XML = "http://www.w3.org/XML/1998/namespace"
MAX_XML = 16 * 1024 * 1024
MAX_REQUEST = 64 * 1024
MAX_PNG = 64 * 1024 * 1024
MAX_DIMENSION = 4096
VECTOR = {"path", "rect", "circle", "ellipse", "line", "polyline", "polygon"}
CONTAINERS = {"defs", "clipPath", "mask", "symbol", "metadata"}
FORBIDDEN_ELEMENTS = {
    "script", "style", "foreignObject", "image", "feImage", "animate",
    "animateMotion", "animateTransform", "set", "discard", "mpath",
}
EVENT_RE = re.compile(r"^on[a-z]+$", re.I)
URL_RE = re.compile(r"url\s*\(\s*([^)]+)\)", re.I)
SAFE_STYLE = {
    "fill", "fill-opacity", "fill-rule", "stroke", "stroke-opacity", "stroke-width",
    "stroke-linecap", "stroke-linejoin", "stroke-miterlimit", "stroke-dasharray",
    "stroke-dashoffset", "opacity", "display", "visibility", "clip-path", "mask",
    "filter", "color", "stop-color", "stop-opacity", "paint-order",
    "font-family", "font-size", "font-style", "font-weight", "font-stretch",
    "font-variant", "font-feature-settings", "font-variation-settings",
    "-inkscape-font-specification", "letter-spacing", "word-spacing",
    "text-anchor", "dominant-baseline", "alignment-baseline", "baseline-shift",
    "line-height", "text-align", "text-decoration", "text-rendering",
    "shape-rendering", "vector-effect", "marker-start", "marker-mid", "marker-end",
    "font-variant-numeric", "text-orientation", "writing-mode", "white-space",
}
XML_DECL = re.compile(br"^\s*<\?xml(?:\s[^?]*)?\?>", re.I)

class ContractError(ValueError):
    pass

def fail(msg):
    raise ContractError(msg)

def bounded_regular_bytes(path, limit, kind):
    path = pathlib.Path(path)
    try:
        info = os.lstat(path)
    except OSError as e:
        fail("cannot stat %s: %s" % (kind, e))
    for parent in path.absolute().parents:
        if parent.is_symlink(): fail("%s has a symlink ancestor" % kind)
    if stat.S_ISLNK(info.st_mode) or not stat.S_ISREG(info.st_mode):
        fail("%s must be a regular non-symlink file" % kind)
    if info.st_size > limit:
        fail("%s exceeds size limit" % kind)
    try:
        with path.open("rb") as stream:
            data = stream.read(limit + 1)
    except OSError as e:
        fail("cannot read %s: %s" % (kind, e))
    if len(data) > limit:
        fail("%s exceeds size limit" % kind)
    return data

def strict_json(raw):
    def pairs(items):
        obj = {}
        for k,v in items:
            if k in obj: fail("duplicate JSON key: "+k)
            obj[k] = v
        return obj
    def constant(v): fail("nonfinite JSON number: "+v)
    try:
        return json.loads(raw.decode("utf-8"), object_pairs_hook=pairs, parse_constant=constant)
    except (UnicodeDecodeError, ValueError) as e:
        fail("invalid JSON: "+str(e))

def read_request(path=pathlib.Path("request.json")):
    raw = bounded_regular_bytes(path, MAX_REQUEST, "request.json")
    obj = strict_json(raw)
    if not isinstance(obj, dict): fail("request must be an object")
    if set(obj) - {"brief","width","height","dark","owner","date","palette"}:
        fail("unknown request field")
    if not isinstance(obj.get("brief"),str) or not 1 <= len(obj["brief"].strip()) <= 48000:
        fail("brief must be nonblank, at most 48000 characters")
    for key in ("owner","date","palette"):
        if key in obj and (not isinstance(obj[key],str) or not 1 <= len(obj[key].strip()) <= 2000):
            fail(key+" must be a nonblank bounded string")
    if "dark" in obj and type(obj["dark"]) is not bool: fail("dark must be boolean")
    for key,default in (("width",1600),("height",1000)):
        v=obj.get(key,default)
        if type(v) is not int or not 640 <= v <= 4096: fail(key+" must be integer 640–4096")
    return obj,obj.get("width",1600),obj.get("height",1000),raw

def dark_requested(request):
    if "dark" in request: return request["dark"]
    brief=request["brief"].lower()
    # Bounded local negation; this deliberately is not a language parser.
    neg=r"\b(?:not|no|without|avoid|never)\b(?:[ -]+(?:a|an|the|for|use|using|make|making|create|produce|intended|meant|designed|any|in|as)){0,4}[ -]*$"
    light=(r"\blight(?:[- ](?:mode|theme|version))?[- ]only\b"
           r"|\bonly (?:a |the )?light(?:[- ](?:mode|theme|version))?\b"
           r"|\b(?:use|keep|remain) (?:it |the canvas )?light\b"
           r"|\bcanvas[^.!?;\n]{0,100},\s*light\s*(?:[.!?;]|$)"
           r"|\b(?:no|without) (?:a |any )?dark(?:[- ](?:mode|deck|version|companion|output))?\b")
    # Explicit light/no-dark instructions override inferred medium, not dark:boolean.
    for match in re.finditer(light,brief):
        if not re.search(neg,brief[:match.start()]): return False
    cue=r"\b(?:dark[ -](?:mode|deck)|slides?)\b"
    for clause in re.split(r"[,;.!?\n]",brief):
        for match in re.finditer(cue,clause):
            if re.search(neg,clause[:match.start()]): continue
            if re.match(r"\s+(?:(?:is|are)\s+)?(?:not (?:needed|required|wanted)|unnecessary)\b",clause[match.end():]): continue
            return True
    return False

def local(tag):
    return tag.rsplit("}", 1)[-1] if isinstance(tag, str) else ""

def number(value, name):
    if not isinstance(value, str):
        fail("missing " + name)
    match = re.fullmatch(r"\s*([+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)\s*(?:px)?\s*", value)
    if not match:
        fail("invalid " + name)
    result = float(match.group(1))
    if not math.isfinite(result) or result <= 0:
        fail("non-positive " + name)
    return result

def _reference(value):
    value = value.strip().strip("'\"")
    if value.startswith("#") and len(value) > 1 and not any(c.isspace() for c in value):
        return value[1:]
    if value and value.lower() != "none":
        fail("external SVG reference: " + value)
    return None

def _style(value, refs):
    if "\\" in value or "/*" in value or "*/" in value or "{" in value or "}" in value or "@" in value:
        fail("escaped or rule-based CSS is forbidden")
    for declaration in value.split(";"):
        if not declaration.strip():
            continue
        if ":" not in declaration:
            fail("malformed inline style")
        name, val = declaration.split(":", 1)
        name = name.strip().lower()
        if name not in SAFE_STYLE:
            fail("unsupported inline style property: " + name)
        if re.search(r"@import|@font-face", val, re.I):
            fail("CSS imports and font faces are forbidden")
        for found in URL_RE.findall(val):
            ref = _reference(found)
            if ref:
                refs.append(ref)

LAYERS=["canvas","frame","regions","connectors","marks","labels","annotation","title-block"]

def inspect_svg(path, width, height, *, master=False):
    raw = bounded_regular_bytes(path, MAX_XML, "SVG")
    if not raw:
        fail("SVG is empty")
    low = raw.lower()
    if b"<!doctype" in low or b"<!entity" in low:
        fail("DTD/entity declarations are forbidden")
    without_decl = XML_DECL.sub(b"", raw, count=1)
    if b"<?" in without_decl:
        fail("SVG processing instructions are forbidden")
    try:
        root = ET.fromstring(raw)
    except ET.ParseError as e:
        fail("malformed SVG XML: " + str(e))
    if root.tag != "{" + SVG + "}svg":
        fail("root must be an SVG-namespace svg element")
    rw, rh = number(root.get("width"), "SVG width"), number(root.get("height"), "SVG height")
    if abs(rw-width) > 0.01 or abs(rh-height) > 0.01:
        fail("SVG canvas dimensions do not match request")
    vb = root.get("viewBox")
    try:
        vals = [float(x) for x in re.split(r"[\s,]+", vb.strip()) if x] if vb else []
    except ValueError:
        vals = []
    if len(vals) != 4 or not all(math.isfinite(x) for x in vals) or vals[2] <= 0 or vals[3] <= 0:
        fail("invalid or missing viewBox")
    if any(abs(a-b)>0.001 for a,b in zip(vals,[0,0,width,height])):
        fail("viewBox aspect ratio does not match request")
    ids, refs, visible, layers = set(), [], 0, []
    titles = descs = 0

    def walk(elem, hidden=False):
        nonlocal visible, titles, descs
        tag = local(elem.tag)
        if isinstance(elem.tag, str) and elem.tag.startswith("{") and not elem.tag.startswith("{"+SVG+"}"):
            fail("foreign-namespace SVG element is forbidden")
        if tag not in VECTOR | {"svg","g","defs","clipPath","symbol","use","marker",
                                 "title","desc","text","tspan","linearGradient","stop"}:
            fail("unsupported SVG element: "+tag)
        if tag in FORBIDDEN_ELEMENTS:
            fail("active, CSS, foreign, raster, or animation element forbidden: " + tag)
        if tag in ("text","tspan") and not master:
            fail("final SVG contains live text")
        now_hidden = hidden or tag in CONTAINERS
        if tag == "title" and (elem.text or "").strip():
            titles += 1
        if tag == "desc" and (elem.text or "").strip():
            descs += 1
        ident = elem.get("id")
        if ident:
            if ident in ids:
                fail("duplicate SVG id")
            ids.add(ident)
        if elem.get("{%s}groupmode" % INK) == "layer":
            if master and (tag != "g" or elem not in list(root)):
                fail("master layers must be root Inkscape groups")
            layers.append(elem.get("{%s}label" % INK))
        for attr, value in elem.attrib.items():
            aname = local(attr)
            if attr == "{%s}base" % XML or aname == "base":
                fail("xml:base is forbidden")
            if EVENT_RE.match(aname):
                fail("SVG event attributes are forbidden")
            if aname in ("href", "src"):
                ref = _reference(value)
                if ref:
                    refs.append(ref)
            if aname == "style":
                _style(value, refs)
            elif "\\" in value and ("url" in value.lower() or aname in ("href", "src")):
                fail("escaped SVG references are forbidden")
            for found in URL_RE.findall(value):
                ref = _reference(found)
                if ref:
                    refs.append(ref)
        if tag in VECTOR and not now_hidden:
            if tag == "path" and not (elem.get("d") or "").strip():
                fail("empty path geometry")
            if tag in ("polyline", "polygon") and not (elem.get("points") or "").strip():
                fail("empty point geometry")
            if tag == "line" and all(elem.get(a) is None for a in ("x1","y1","x2","y2")):
                fail("empty line geometry")
            if tag == "rect":
                number(elem.get("width"), "rect width"); number(elem.get("height"), "rect height")
            if tag == "circle":
                number(elem.get("r"), "circle radius")
            if tag == "ellipse":
                number(elem.get("rx"), "ellipse rx"); number(elem.get("ry"), "ellipse ry")
            display = (elem.get("display") or "").lower()
            style = (elem.get("style") or "").replace(" ", "").lower()
            if display != "none" and "display:none" not in style and "visibility:hidden" not in style:
                visible += 1
        for child in elem:
            walk(child, now_hidden)

    walk(root)
    if not titles or not descs:
        fail("SVG requires nonempty title and desc")
    if not visible:
        fail("SVG has no visible vector geometry outside definitions")
    missing = sorted(set(refs) - ids)
    if missing:
        fail("unresolved internal SVG reference(s): " + ", ".join(missing[:8]))
    if master and layers != LAYERS:
        fail("master requires exact eight named root Inkscape layers in order")
    return {"width":rw, "height":rh, "viewBox":vals, "visible_geometry":visible,
            "layers":layers, "ids":len(ids)}

def png_pixels(path):
    data = bounded_regular_bytes(path, MAX_PNG, "PNG")
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        fail("invalid PNG signature")
    pos = 8
    width = height = color = channels = expected = None
    compressed = bytearray()
    saw_ihdr = saw_idat = saw_iend = False
    while pos < len(data):
        if pos + 12 > len(data):
            fail("truncated PNG chunk")
        length = struct.unpack(">I", data[pos:pos+4])[0]
        if length > MAX_PNG or pos + 12 + length > len(data):
            fail("malformed PNG chunk")
        kind = data[pos+4:pos+8]
        payload = data[pos+8:pos+8+length]
        stored = struct.unpack(">I", data[pos+8+length:pos+12+length])[0]
        if zlib.crc32(kind + payload) & 0xffffffff != stored:
            fail("bad PNG CRC")
        pos += 12 + length
        if not saw_ihdr and kind != b"IHDR":
            fail("PNG IHDR must be first")
        if kind == b"IHDR":
            if saw_ihdr or len(payload) != 13:
                fail("bad PNG IHDR")
            saw_ihdr = True
            width, height, depth, color, comp, filt, interlace = struct.unpack(">IIBBBBB", payload)
            if not (1 <= width <= MAX_DIMENSION and 1 <= height <= MAX_DIMENSION):
                fail("PNG dimensions exceed supported bound")
            if depth != 8 or color not in (0,2,4,6) or comp or filt or interlace:
                fail("unsupported PNG format")
            channels = {0:1, 2:3, 4:2, 6:4}[color]
            expected = height * (width * channels + 1)
        elif kind == b"IDAT":
            if saw_iend:
                fail("PNG IDAT after IEND")
            saw_idat = True
            compressed.extend(payload)
            if len(compressed) > MAX_PNG:
                fail("PNG compressed data exceeds bound")
        elif kind == b"IEND":
            if length or not saw_idat:
                fail("invalid PNG IEND")
            saw_iend = True
            if pos != len(data):
                fail("trailing data after PNG IEND")
            break
    if not saw_iend:
        fail("missing complete PNG IEND")
    decompressor = zlib.decompressobj()
    try:
        raw = decompressor.decompress(bytes(compressed), expected + 1)
        if len(raw) > expected or decompressor.unconsumed_tail:
            fail("PNG decompression exceeds expected rows")
    except zlib.error as e:
        fail("invalid PNG compression: " + str(e))
    if len(raw) != expected or not decompressor.eof or decompressor.unused_data:
        fail("unexpected or incomplete PNG data")
    stride = width * channels
    rows, prior, off = [], bytearray(stride), 0
    for _ in range(height):
        filter_type = raw[off]
        scan = bytearray(raw[off+1:off+1+stride])
        off += stride + 1
        for i in range(stride):
            a = scan[i-channels] if i >= channels else 0
            b = prior[i]
            c = prior[i-channels] if i >= channels else 0
            if filter_type == 1: scan[i] = (scan[i] + a) & 255
            elif filter_type == 2: scan[i] = (scan[i] + b) & 255
            elif filter_type == 3: scan[i] = (scan[i] + ((a+b)//2)) & 255
            elif filter_type == 4:
                p=a+b-c; pa=abs(p-a); pb=abs(p-b); pc=abs(p-c)
                scan[i]=(scan[i]+(a if pa<=pb and pa<=pc else b if pb<=pc else c))&255
            elif filter_type != 0: fail("invalid PNG filter")
        rows.append(bytes(scan)); prior = scan
    pixels = b"".join(rows)
    first = pixels[:channels]
    if all(pixels[i:i+channels] == first for i in range(0, len(pixels), channels)):
        fail("uniformly blank PNG")
    if color in (4,6) and all(pixels[i+channels-1] == 0 for i in range(0, len(pixels), channels)):
        fail("fully transparent PNG")
    return width, height, color, pixels
