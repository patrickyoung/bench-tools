---
name: shipping-tools
description: Package a new deterministic Go CLI for an Agent worker when existing executable tools and Brief skills cannot perform the bounded transformation; build a clean source library and a separate per-host runtime copy with checked invocation.
---

# Ship a deterministic tool without changing the runner

First inspect the Agent/Ply executable catalogue, relevant Brief skills and public help of existing programs. Reuse a capable tool rather than adding one. State the deterministic gap and the tool's independently useful input/output contract. Agent/Ply lists executable names without executing them; absence of a catalogue synopsis is not a failure. Never auto-run a binary merely to discover it. Do not add a loader, registry, runtime, model loop, umbrella Go module or automatic MCP server. MCP is optional only for a separately selected external service whose protocol requires it.

## Source and host package

For a newly authored tool under this procedure, keep `src/NAME/` as an independent Go module with `go.mod`, source, meaningful positive and negative tests, and a reproducible build/install recipe. Build with the selected Go toolchain and `GOWORK=off`: no adjacent-module imports, replacements or root workspace. Name any admitted external executable dependencies and their input/output contracts; prefer none. A clean source-library export contains source, tests, skill and recipe, not compiled binaries, caches, jobs or logs. Do not symlink source across components.

The caller creates a separate per-host runtime copy *before* Agent runs, then builds `tools/NAME` in that copy for the selected host. It must be a reviewed executable with executable mode. `tools/` contains no nonexecutable docs or source; runtime definitions are read-only, so never compile or install lazily into them. Keep `bin/check` as the worker's deliverable completion gate, not a synonym for CLI success. Respect separate work, state and evidence locations and caller-selected write/network authority.

For example, suppose a reviewed independent `slug` module implements a UTF-8, newline-delimited stdin → stdout slug transform, sends diagnostics to stderr, returns 0 on valid input and nonzero on invalid input, and has a side-effect-free `--help` returning 0 without mutation or network. From the caller's build shell, with illustrative portable paths:

    mkdir -p runtime/
    cp -R library/worker runtime/worker
    mkdir -p runtime/worker/tools/
    (cd runtime/worker/src/slug && GOWORK=off go test ./... && GOWORK=off go build -o ../../tools/slug .)
    chmod 755 runtime/worker/tools/slug

`library/worker` remains clean; `runtime/worker` is the host-specific package supplied to Agent, not a destination for run outputs. The hypothetical worker's documented, reviewed invocation might be:

    runtime/worker/tools/slug --help
    printf 'Hello World\n' | runtime/worker/tools/slug > work/slug.txt

These are caller examples, not automatic discovery or a request to run a worker task while building this definition. Specify the real tool's literal argv, accepted stdin, stdout format, stderr diagnostics, exit meanings, safe `--help`, and output paths instead of borrowing the hypothetical contract. Write a concise Brief skill in that worker's `skills/` and route explicitly to it from that worker's `AGENTS.md`; Brief owns skill selection. Document the build recipe, required Go toolchain and any external executables for the caller.

## Evidence before claiming completion

Inspect source, tests, skill and build recipe; compile and test the independent module. Strict-lint its skill and confirm runtime `tools/NAME` catalogue visibility and structural Hire/Agent checks. These prove wiring only. To claim a working packaged capability, run a fresh Agent task that discovers and loads the worker skill, actually invokes the installed executable, and produces a correct artifact verified by an independent check. Keep a held-out input/output oracle beyond model-editable scope and test a relevant rejected case; keep actual process evidence outside reusable source. If a live run is unavailable, say exactly what remains unverified. Compilation alone does not demonstrate adoption or benefit. No automatic permission or deployment rule follows from this procedure.
