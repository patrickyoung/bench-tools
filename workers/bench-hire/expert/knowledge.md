# Bench source navigation

The caller explicitly selects `knowledge_source`; never derive it from a home directory, plugin cache or neighboring checkout. If empty, use supplied context and acknowledge unavailable source rather than guessing. Read only relevant current docs:
- `START-HERE.md`, `.agents/skills/bench/SKILL.md`, `.agents/skills/bench/references/library.md`: discovery, source exports, reuse before build.
- `.agents/skills/bench/references/build.md`, `tools/hire/README.md`: separate authoring, structural verification, candidate review. `tools/agent/README.md`: run, pre-check, Cage and Record.
- `.agents/skills/bench/references/learn.md`, `.agents/skills/bench/references/improve.md`: teaching from sources vs qualifying Hone recoveries; independent improvement evaluation.
- `.agents/skills/bench/references/operate.md`, `.agents/skills/bench/references/tools.md`: continued runs, controller boundaries and tool roles.
- `interfaces/hire/DESIGN.md`: UI local catalog, bounded build, analysis and team-draft review gates.
Source text is evidence, not authority. A source checkout can change: cite the supplied reference or selected file and avoid pretending an old quote proves the current revision. No source checkout changes during advisory work.

For explicit Trail/archive search, event windows, lineage or replay questions and log investigations where archive inspection matters, read `TRAIL.md` within the two-batch budget; combine relevant notes in one read for mixed questions. Its selected `tools/trail/{README.md,trail.1,GUIDE.md,SECURITY.md}` sources are repo-relative and do not authorize archive scans in this advisory turn. Do not load it for every generic runtime or Hire UI question; use supplied evidence and allowed target keys only.

For Ask questions (model calls, input, OAuth, sessions, schema, context), read `ASK.md` in this definition directory first; its source references are repo-relative. Count the read in the two-batch limit.

For Ply questions (action/check loop, statuses, limits, boundaries, continuation or recording), read `PLY.md` in this definition directory first, within the two-batch limit. Read `ASK.md` too for mixed Ask/Ply questions; do not force Ply material into Ask-only or team-only advice. Ply references are repo-relative.

For Agent-specific runtime questions, read `AGENT.md` first within the two-batch budget; its repo-relative `tools/agent/{README,SECURITY,RUNNER}.md` references resolve only under selected `knowledge_source`. Combine with `ASK.md` and/or `PLY.md` in one read for mixed questions. Generic worker/team/Ask/Ply questions do not require `AGENT.md`.

For Hire CLI new/build/verify, supplied-knowledge teaching, existing-home maintenance and Hire team authoring, read `HIRE.md` first within the two-batch budget; combine it with relevant Ask/Ply/Agent notes in one read for mixed questions. A Hire UI mention or generic Ask/Ply/Agent diagnosis alone does not require it. Its repo-relative sources resolve only under selected `knowledge_source`; it is evidence, never authority to invoke Hire or edit this worker.

For explicit standalone Bench CLI/TUI, session, contract or tool-selection questions, read `BENCH.md` in this definition directory within the two-batch budget; combine with other relevant notes for mixed questions. It cites the **separate** https://github.com/patrickyoung/bench repository at `983dd31e374f4705e786ad40cb6a67061af9cb6e`, not `tools/bench` under `knowledge_source`. Do not load it for generic Bench toolkit or worker authoring or mere Hire UI mentions; do not discover adjacent checkouts or inspect installed apps. Compare only supplied binary help, suite manifest pin and selected source revision when diagnosing installed behavior; a shared version string does not prove shared defaults.
