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
    "shape-rendering", "vector-effect",
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

def read_request(path=pathlib.Path("request.json")):
    raw = bounded_regular_bytes(path, MAX_REQUEST, "request.json")
    try:
        obj = json.loads(raw.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as e:
        fail("invalid request.json: " + str(e))
    if not isinstance(obj, dict):
        fail("request.json must be an object")
    extra = set(obj) - {"description", "style", "width", "height"}
    if extra:
        fail("unknown request field(s): " + ", ".join(sorted(extra)))
    description = obj.get("description")
    if not isinstance(description, str) or not description.strip() or len(description) > 12000:
        fail("description must be a nonempty string of at most 12000 characters")
    style = obj.get("style")
    if style is not None and (not isinstance(style, str) or not style.strip() or len(style) > 2000):
        fail("style must be null/omitted or a nonempty string of at most 2000 characters")
    for key in ("width", "height"):
        value = obj.get(key, 1200 if key == "width" else 900)
        if isinstance(value, bool) or not isinstance(value, int) or not 256 <= value <= 4096:
            fail(key + " must be an integer from 256 through 4096")
    return obj, obj.get("width", 1200), obj.get("height", 900), raw

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
    if abs(vals[2]/vals[3] - width/height) > 1e-6:
        fail("viewBox aspect ratio does not match request")
    ids, refs, visible, layers = set(), [], 0, []
    titles = descs = 0

    def walk(elem, hidden=False):
        nonlocal visible, titles, descs
        tag = local(elem.tag)
        if isinstance(elem.tag, str) and elem.tag.startswith("{") and not elem.tag.startswith("{"+SVG+"}"):
            fail("foreign-namespace SVG element is forbidden")
        if tag in FORBIDDEN_ELEMENTS:
            fail("active, CSS, foreign, raster, or animation element forbidden: " + tag)
        if tag == "text" and not master:
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
        if tag == "g":
            mode = elem.get("{%s}groupmode" % INK)
            label = elem.get("{%s}label" % INK)
            if mode == "layer" and label and label.strip():
                layers.append(label.strip())
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
    if master and len(set(layers)) < 2:
        fail("editable master needs at least two meaningfully named Inkscape layers")
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
