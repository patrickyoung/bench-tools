# Drawing plan v1 (normative bounded schema)

UTF-8 JSON object, maximum 16 MiB. Duplicate or unknown keys are rejected.
No nulls, booleans as numbers, nonfinite numbers, markup, code, references,
URLs, filesystem paths, embedded images, custom attribute names or raw SVG.
All listed keys are required; no other keys are accepted.

Root: `schema` = `inkscape-plan/v1`, `title`, `description`, `layers`, `review`.

Plain text fields (title, description, layer name, text content): 1–500
characters; Unicode word characters, spaces, and `,.!?()'’–—:+-` only.
These are literal displayed prose, not interpretable instructions or code.
Slash, backslash, angle brackets, quotes, equals, semicolons and newlines
are disallowed, as are URI-like word-colon sequences and double dots. Unsupported lettering must be reported rather than escaped.

`layers`: ordered array of 2–12 objects, each with `id`, `name`, `objects`.
Layer names must be distinct and meaningful. Objects are painted in array order.
Maximum 500 objects total. IDs for objects and layers must be globally unique,
match `[a-z][a-z0-9-]{0,39}`, and must not equal `drawing`.

`review`: exactly `{ "targets": [...] }`. Targets are 1–24 distinct objects
of exactly `{ "id", "importance" }`; `id` must name a real drawing object,
never a layer. `importance` is `primary`, `required`, or `support`, and exactly
one target is `primary`. Name visibly exposed objects that carry the focal read
or an essential pictured fact. The trusted finishing audit rejects a primary or
required target whose complete geometry bounds fall beneath a later opaque
rectangle. This is a narrow paint-order guard, not proof that the image meets
the request or looks good.

Every object has:
- `op`, `id`
- `fill`, `stroke`: `none` or exactly `#RRGGBB`; at most 32 distinct colors
- `stroke_width`: number in [0, min(canvas width, height)/10]
- `opacity`: number in [0,1]
- operation-specific fields below, with no other attributes.

Operations:
- `rect`: `x`, `y`, `width`, `height`. Positive dimensions; entire rectangle
  must fit the canvas.
- `ellipse`: `cx`, `cy`, `rx`, `ry`. Positive radii; entire ellipse must fit.
  Equal radii make a circle.
- `polygon`: `points`, an array of 3–256 `[x,y]` pairs.
- `path`: `d`, at most 16000 characters and 256 segments. Starts with `M`.
  Only explicit absolute `M x y`, `L x y`, `C x1 y1 x2 y2 x y`,
  `Q x1 y1 x y`, and `Z` are admitted. Each segment repeats its command.
  Numbers are unsigned decimals, not exponent notation. Spaces/commas separate
  values. No relative coordinates, arcs, implicit repeats, transforms or hrefs.
- `text`: `x`, `y`, `text`, `font_size` in [0,min(width,height)/2].
  Adapter selects sans-serif; final export outlines the live master text.
  Font-dependent text extent is reviewed in the rendered preview.

Every x coordinate and x control point is in [0, requested width], every y in
[0, requested height]. Stroke extents and glyph extents may exceed geometry
bounds; inspect the actual preview. Units are canvas pixels with origin top-left.
Canvas dimensions come only from request.json (defaults 1200×900); the plan
cannot override them. This first version intentionally omits gradients, clipping,
filters, transforms and arbitrary groups. Use layered flat vector shapes instead.

The executable validator is tools/authoring.py:dom. It builds an Inkscape-native
namespace DOM using Python's XML DOM facilities (not inkex). This is an adapter,
not a general SVG interface: object and attribute names are fixed in trusted code.
Inkscape opens and serializes its constructed document. The adapter removes only
the generated sodipodi namedview and docname editor metadata, then deterministically
serializes the DOM to retain the baseline's prohibition on foreign elements.
All vector content and Inkscape layer attributes remain. The independent check
repeats this exact process and compares master bytes, without executing any
workspace-provided script.
