# WorldWeaver offline regression tests

Run from the repository root with the catalog runner:

    python3 scripts/check-worker-evaluations.py --entry worldweaver-omega --profile core
    PLAYWRIGHT_BROWSERS_PATH=/absolute/browser-cache python3 scripts/check-worker-evaluations.py --entry worldweaver-omega --profile browser --node-modules /absolute/installed/node_modules

The driver copies exact source/tests into retained external scratch. It makes no
model calls and does not install dependencies. Core uses Python 3.10+, Node 22+
and Unix ps/process groups. It covers byte/admission/receipt contracts, malformed
and oversized input, style-independent identities, dependency profiles, seeded
coordinates/noise, reservations, vertical assertions and bounded cleanup. Fake
Ask receipts and process fixtures are explicitly synthetic, not semantic truth.

Browser requires the selected pinned Three 0.186.0, Playwright 1.63.0 and esbuild
runtime plus Chromium. It covers IndexedDB/helper behavior, actual Three/worker
rendering, positive/negative scene/UI/motion/time fixtures and real hung-page
cleanup. The audit retains Chromium's native sandbox; macOS nested Seatbelt
initialization can block it, so select the controller browser boundary explicitly.
No unsafe retry or automatic sandbox disabling is provided.

Optional profile installation/build verification is an explicit manual check:

    WW_TEST_HOME=/absolute/export/expert node workers/worldweaver-omega/tests/dependency-profiles.mjs /absolute/new/results

It uses npm ci offline with lifecycle scripts disabled and needs the exact
packages already cached. It installs only into the selected fresh result folder.
No network install is part of core/browser regression tests.

These are curated synthetic tests, not generated worlds or training evidence.
Fresh Agent cases and source/visual reviews live outside this library. No all-style
quality, calibrated semantic-judge accuracy or physical-mobile 90 FPS claim follows
from these suites. The separate catalog quality plan describes fresh trials.
