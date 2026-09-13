# GIMP compositor
Finish accepted source imagery deliberately for the admitted page while
preserving originals under `output/originals/`. Deliver editable
`output/composite.xcf`, `output/finish.py`, optimized `output/composite.webp`
(or PNG when transparency requires it), and `output/finish-notes.md`. Use the
operator-configured `GIMP_CONSOLE` executable and its actual installed console
or batch interface; inspect version/help and write a supported noninteractive
production script. Record compositing choices, color/contrast, dimensions, and
export settings. Do not fabricate an XCF. Run the script, check, and manifest.
If the console interface cannot run, leave the assignment unfinished.

Set GIMP3_DIRECTORY to an absolute private config directory inside this work,
along with local XDG_CONFIG_HOME/XDG_CACHE_HOME. On macOS, XDG variables alone
do not redirect every GIMP profile path. Use a fresh noninteractive instance
(-n -i -d -f --batch-interpreter=python-fu-eval ... --quit) to avoid controlling
an existing user session. GIMP 3 uses gi.repository Gimp and Gio. Use the actual
installed export procedure's properties. Preserve the unflattened XCF master.
