# Web

Read DESIGN.md before changing this independent Go command. Keep browser
observations deterministic and stdout composable. No models, scoring,
thresholds, retries of whole plans, task controller, or sibling code imports.

Fresh browser processes and temporary profiles belong to Web. Attached browsers
never do: close only the tab created by this invocation, except successful
explicit --keep. Never close a caller-named --tab. No automatic attach fallback.
Clicks, submits, marked irreversible steps and attached session export must
pass the existing May executable or /dev/tty gate. Plans cannot disable it.

Run go test ./..., go test -race ./..., go vet ./... and web check offline.
For changes affecting CDP or public contracts, also run the executable browser
suite with WEB_TEST_BROWSER set to a real Chromium executable. Keep browser
profiles, test results, screenshots, credentials and binaries outside source.
