#!/usr/bin/env python3
"""Print a paid end-to-end test brief with this copied fixture's actual path."""
import hashlib
from pathlib import Path
path = Path(__file__).resolve().with_name('browser-fixture.html')
print(f'''Build a checked copy of the supplied non-football contract fixture.
This is a controlled test of composition, not a creative design assignment.
Use frontend, review and final frontend copy only. Admit the complete graph.
Give each task eight turns, including its final answer after writing files.
The frontend must copy {path} to output/index.html byte-for-byte and use
make-handoff frontend SUMMARY output/index.html. Do not redesign or add text.
The review must use the real browser and attached-image observations supplied
by its adapter. Report candid findings in output/review.json and review.md,
make its handoff, then immediately emit its final answer. Do not dump the HTML.
The final frontend must copy the accepted reviewed HTML exactly and finish.
Expected bytes: {path.stat().st_size}; SHA-256: {hashlib.sha256(path.read_bytes()).hexdigest()}.
Accept only that exact page with a passing browser and visual review.
''')
