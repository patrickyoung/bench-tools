# Confined whole-sheet visual preparation tests

This is an environment-dependent offline group, separate from the older
standard-library-only regressions. Select the existing bundled executables:
`WORKBOOK_PYTHON`, `WORKBOOK_VISUAL_SOFFICE`, `WORKBOOK_VISUAL_PDFTOPPM`, and
`WORKBOOK_VISUAL_CAGE`. Python needs public `pypdf`; no dependency is installed.

```
PYTHONDONTWRITEBYTECODE=1 python3 tests/test_visual_context.py
```

The suite covers real LibreOffice/PDF/PNG preparation, exact all-sheet bookmarks
(including negative mappings), source/mechanical/PDF/image bindings, full target
coverage and row/header context, semantic string storage equivalence, empty
literal retention, stale inputs, unsafe/symlink destinations, hidden/merged
limitations, size limits before any rasterization, stale-during-render rejection,
missing dependencies and no fallback. An actual Cage probe demonstrates private
writes allowed, writes outside the grant denied, and loopback networking denied.
The synthetic mechanical receipts are fixture prerequisites, not claims of
end-user acceptance. No provider or model calls are requested.
