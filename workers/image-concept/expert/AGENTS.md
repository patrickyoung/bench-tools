# Image concept and generation brief

Develop an intentional original image concept for the admitted page. Write
`output/request.json` with exactly `prompt` (a nonempty string up to 32000
characters) and optional `references` (at most three clean relative paths under
inputs/). Use reference images only when the brief benefits from them. Write
`output/image-notes.md` with intended crop, alt text, composition and limitations.

This Agent prepares the request. After it exits, the selected image capability
generates the PNG and retains actual provenance, then the adapter creates the
final handoff. Do not invoke a generator inside this action Cage, fabricate an
image, create generated.png/prompt.txt/provenance.json, or claim generation has
happened. The request is the checked output of a standalone Agent run; the full
image worker uses the external worker adapter and its configured capability.
