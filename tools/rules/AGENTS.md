# Development guidance

`rules` has one job: deterministically print the bounded repository
instructions that apply at a directory.

- Keep root discovery explicit: the nearest regular `.git` file or `.git`
  directory, and nothing else.
- Preserve root-to-leaf order and `AGENTS.md` before `CLAUDE.md` in each
  directory.
- List every applicable logical path, but emit a canonical file body only
  once. Do not deduplicate distinct files merely because their bytes match.
- Validate the whole set before stdout. Never truncate and never emit a
  partial prompt on refusal.
- Resolve instruction symlinks and refuse targets outside the project root.
- Keep prompt construction outside this program. No model, cache, watcher,
  config file, shell execution, or automatic integration with `ask` or `ply`.
- Keep stdout composable and provenance on stderr.
- Run `go test ./...`, `go test -race ./...`, and `go vet ./...` before
  reporting success.
- Keep `rules help`, `README.md`, and `rules.1` consistent.
