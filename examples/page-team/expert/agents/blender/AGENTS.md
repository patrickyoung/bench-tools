# Blender spatial asset artist
Author a brief-specific spatial asset. Keep `output/asset.blend` as the editable
master, `output/build_asset.py` as a deterministic production script,
`output/preview.png`, and an appropriate page export such as
`output/asset.glb`. Use only the operator-configured `BLENDER` executable; first
inspect its read-only version/help. Execute the production script through
Blender, do not fabricate binary formats, and retain script logs. Use camera,
lighting, materials, and geometry intentionally for the brief. Optimize the web
export and document scale/units. Run the child check and manifest all outputs.
If Blender is unavailable or execution fails, report unfinished rather than
substituting placeholder text.

Read the supplied packet once, create one script using the canonical names
above, run it, make the handoff and finish. Do not create duplicate aliases.
Use regular files, never symlinks. After the script and handoff succeed, emit
your final answer promptly so Agent can run its checker; an action that calls
the checker is not itself a final answer. Aim to finish within eight turns.
Blender 5.2 uses BLENDER_EEVEE (not BLENDER_EEVEE_NEXT); inspect the installed
version. Avoid optional PIL checks when Pillow is not configured. Save with
compress=False for easy inspection. Start with --background --factory-startup
--disable-autoexec and add --python-exit-code 1 before the production script.

For a repair, inspect the existing artifacts first. Reuse a valid scene and
render when only the handoff or validation is missing; do not rebuild the art
unnecessarily. Blender can save Zstandard- or gzip-compressed .blend files;
compression does not make a master invalid. Reopen the saved master with
Blender and auto-execution disabled to verify it is editable. Use the installed
engine enumeration, not a remembered EEVEE name from another release. CYCLES
CPU is available in the tested setup. Use --python-exit-code 1 so a script error
cannot appear as a successful Blender process, and retain actual diagnostics.
