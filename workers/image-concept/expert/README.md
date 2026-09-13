# Image concept

Prepares an original image concept and a checked request for a selected image
backend. A standalone Agent run produces the request; it does not generate an
image or claim that an image exists.

```sh
agent run -C /path/to/current-work -evidence /path/to/control \
  /path/to/export/expert -- 'Describe the image needed for this brief'
```

Supply the actual brief and optional admitted reference files under inputs/.
The output is output/request.json (prompt and optional references) plus
output/image-notes.md. The check validates request shape, bounded prompt and
composition notes. It does not execute a backend, authorize references, or
produce a PNG/handoff. A request with an empty prompt must fail.

In page-team, the image-generation role uses this worker and the existing
controller-owned generate-image adapter. That adapter validates references,
calls the operator-selected capability and emits the image with its final
handoff and provenance. Other callers can consume the same request contract.
Keep backend permissions and credentials outside the worker definition.
