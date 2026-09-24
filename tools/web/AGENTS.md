# Web

Read DESIGN.md before changing this independent Go command. Keep browser
observations deterministic and stdout composable. No models, scoring,
thresholds, retries of whole plans, task controller, or sibling code imports.

Fresh browser processes and temporary profiles belong to Web. Explicit native
profiles are caller-owned and must survive all exits; their newly launched
browser process belongs to Web. Respect Chrome/Web profile locks; never use
stale native port files to connect or disable Chrome's default-profile restriction.
Stealth is opt-in and scoped to new documents in the selected tab. It must not
weaken action gates or add a challenge solver or autonomous retry loop. Attached browsers
never do: close only the tab created by this invocation, except successful
explicit --keep. Never close a caller-named --tab. No automatic attach fallback.
Clicks, submits, marked irreversible steps and attached/native session export must
pass the existing May executable or /dev/tty gate. Plans cannot disable it.

Run go test ./..., go test -race ./..., go vet ./... and web check offline.
For changes affecting CDP or public contracts, also run the executable browser
suite with WEB_TEST_BROWSER set to a real Chromium executable. Keep browser
profiles, test results, screenshots, credentials and binaries outside source.
