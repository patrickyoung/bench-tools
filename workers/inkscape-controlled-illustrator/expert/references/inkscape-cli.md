# Supported Inkscape CLI use

Requirement: Inkscape 1.2 or newer. `INKSCAPE` may select an absolute executable; otherwise `inkscape` is resolved on PATH. The installed executable's `--version` and `--help` output is authoritative.

For explicit exports this worker passes separate argv elements and uses:

- `--export-type=svg`, `--export-plain-svg`, `--export-text-to-path`
- `--export-filename OUTPUT INPUT`
- `--export-type=png`, `--export-width WIDTH`, `--export-height HEIGHT`
- `--query-all INPUT`

Use explicit export options without `--batch-process`. Inkscape 1.4.4 on the evaluated macOS host crashed or hung with batch mode, while explicit SVG and PNG exports succeeded. The external evaluation runbook retains the native probe evidence.

The helper preserves `HOME` and assigns run-specific `INKSCAPE_PROFILE_DIR`, XDG cache/config/data paths, and `TMPDIR`. Bundled macOS fontconfig/GTK may still attempt fixed native cache paths; kernel-denied writes are warnings and not evidence that every cache was redirected. Never disable Cage to silence them. Child commands run in new process sessions; timeout handling kills the entire process group.

Upstream references:

The supported inline style subset includes font and spacing properties because Inkscape retains these attributes on outlined paths. The final SVG still requires zero live text elements. Editable text in a master uses locally installed fonts; final outlines do not require those fonts for display.

- https://inkscape.org/doc/inkscape-man.html
- https://wiki.inkscape.org/wiki/Using_the_Command_Line
