# Web: a filter with a browser

Web renders one URL to Markdown, text, HTML, links, a snapshot or a screenshot.
It can execute a finite caller-written JSON plan in one browser tab. It makes
no judgments: the caller selects URLs, steps and acceptance criteria. A model,
Weigh, Ply or an ordinary shell pipeline can consume its public streams.

The Go port preserves the standalone Web command family and version-1 snapshot
and transcript shapes. It uses Rod over Chromium's DevTools protocol, with an
explicitly owned socket and process. There is no Python, Node, Playwright,
mandatory daemon, model client, or shared Bench runtime.

## Ownership

Fresh reads launch installed Chromium with a newly created temporary profile;
they carry no ambient login. WEB_BROWSER selects the executable; otherwise
Web discovers installed Chrome/Chromium. --profile explicitly replays a JSON
storage-state file. --attach connects to the supplied endpoint in the default
browser context and never launches a fallback. --user-data-dir explicitly selects
a persistent native browser directory; --profile-directory selects a child
profile (default Default). These three identity modes conflict.

Web normally creates one tab and closes it, even on error or SIGINT/SIGTERM.
An attached browser is never closed. --keep requires --attach and retains
only a successfully completed plan's own tab, printing its target ID on stderr.
--tab requires --attach, selects exactly the supplied target and never closes
it. --keep and --tab conflict. Closing the CDP socket does not close targets.
Ordinary attachment does not inspect other pages; protocol target metadata can
arrive from the browser.

Native mode uses Rod's NewUserMode argument preset with an explicit installed
binary, user data directory and random loopback debugging port. Web owns the
process it launches and closes it, but never deletes the selected profile.
It does not discover/copy logins or close an existing personal browser. Profile
locks fail closed; a browser already running remains accessible only through
explicit --attach. The CDP endpoint comes from the new process's stderr, never
from a potentially stale persistent DevToolsActivePort file. The default Chrome
personal data directory (including aliases/descendants) is refused: Chrome 136+
disables remote debugging there. No restriction-disabling switch is used.

At the user's explicit request, version 2.1.0 adds --stealth with pinned
`go-rod/stealth`. It installs the upstream JavaScript before new documents on
only the selected tab. It is opt-in and announced on stderr; it does not patch
unrelated tabs or retroactively change an already loaded document. This changes
the earlier no-stealth design. Masking some browser automation properties does
not guarantee access through Cloudflare/Akamai or solve CAPTCHA/MFA. A site's
challenge/refusal is returned as observed page content; there is no challenge
solver, authentication bypass, proxy rotation or automatic retry loop.

## Actions and approvals

Every click, submit and step marked irreversible:true goes through one gate
before the action. irreversible:false cannot exempt clicks or submits. With explicit --may-job JOB, invoke literal argv `may request JOB` and supply
exact description bytes on stdin. Validate the version-1 result's job, action
and verdict against the exit status. Only spent/0 allows; parked/75 parks,
declined/3 declines; anything else means no approver (77). The audit uses
May's returned digest, without duplicating May's state or digest implementation.
For legacy callers, CLERK_WORKER, JOB_ID and CLERK_WROOT together select
WORKER/JOB_ID when --may-job is absent. Outside a job, ask y/N on /dev/tty; never read approval from plan
stdin. No terminal means 77. Descriptions come from the plan's may field or
operation, selector and preceding goto host, never from mutable page text.

The gate also protects auth --attach and native-profile JSON export before
connecting, launching or reading credentials.
Fresh or native auth opens a headed browser for manual login. Native auth can
omit the export filename: Chrome keeps the session in the selected directory.
Native profile writes persist even on EOF/failure, following normal Chrome
cookie/session rules; Enter only confirms completion (and any requested JSON
export). Session reuse relies on a prior valid login; new MFA remains manual. Web never automates login
or types into password fields. Storage-state exports contain cookies and
localStorage from origins with open pages in the chosen context. They do not
include closed origins, IndexedDB, sessionStorage or browser passwords. Partitioned
cookies fail export/replay explicitly rather than losing their scope. Export
is an atomic mode-0600 file; the operator owns its location and lifecycle.

## Streams and bounds

Reads write only data to stdout. Run emits one JSON transcript, including
failures and gate refusals. Diagnostics and kept tab IDs go to stderr. Exit 0
means complete, 1 browser/operation failure, 2 invalid input, 3 declined,
64 unknown command, 75 parked, 77 no approver. Signals cancel work and clean up.

Snapshots collect live URL, title, body text, visible links and CSS-selected
records in one evaluation. They preserve record/link ownership and document
base resolution, including redirects. Visibility means a layout rectangle and
computed display/visibility; frames, shadow roots and pixel perception are not
included. Snapshot JSON is limited to 8 MiB; oversized output fails before any
stdout. Plans and profiles are limited to 8 MiB on input. Per-step deadlines
bound browser operations; WEB_ATTACH_TIMEOUT bounds endpoint discovery and CDP
connection. Networkidle waits for Chromium's lifecycle networkIdle event.

Optional best-effort audit JSONL retains time, command, URL, byte count,
duration, outcome, identity mode (fresh/profile/native/attach), and gate words/digest. WEB_STATE selects its
file; default is XDG_STATE_HOME/web/web.jsonl, or ~/.local/state/web/web.jsonl.
Audit is diagnostic evidence, not authorization or a mandatory shared service.

## Port changes

Version 2.0.0 makes these intentional migration changes explicit:

- Approval composition uses Bench May's `request JOB`/stdin contract. Old Clerk
  grants are not migrated; a new exact approval is required.
- `web setup` checks the installed browser and prints its path. Browser
  installation/update is the operator's package manager's responsibility.
- The default audit path is user state rather than a writable source checkout.
- Selectors are CSS; Playwright-only selector syntax is not accepted.
- Invalid flags/types, nonpositive deadlines and oversized inputs fail early.
- Inline plan HTML above 200,000 characters fails instead of silently truncating;
  use the existing out field for complete HTML. File byte counts count UTF-8.
- Read link reduction resolves against the live document base after redirects.
- Fresh auth requires an actual Enter; EOF does not save a session.
- Password fields are explicitly refused by type, enforcing the original design.

Fixtures copied from the standalone Web repository prove Markdown/links
compatibility offline. Go unit tests cover gates, parsing, bounds and reduction.
The separate opt-in real-browser executable suite proves process streams,
rendering, profiles, action gates, target ownership and connection deadlines.
Neither suite calls a model or uses ambient user profiles.
