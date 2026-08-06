# govna Plan

## Product Direction

Provide a narrow, usable governance template that bootstraps new repos — the same product governa provides, ported to Rust with the intent of eventually replacing governa as the canonical implementation, not remaining a permanent dependent alongside it. Govna is a one-time apply — after bootstrap, the consumer repo owns all governance files and evolves them independently. Template improvements are adopted by having consumer-repo agents read govna's source and cherry-pick what's useful.

## Ideas To Explore

Ideas captured for future reference. A bullet list — each line starts with `- IE<N>: ` (sequential N) for stable references. Two kinds: (a) **pre-rubric IE** — `IE<N>: <one-liner>`, awaiting director discussion and the objective-fit rubric (see `AGENTS.md` Approval Boundaries); (b) **AC-pointer** — `IE<N>: <one-liner> → govna/ac<N>-<slug>.md`, pointing at a drafted AC stub not yet through critique. A pre-rubric entry that clears the rubric converts to an AC-pointer at AC-draft time, keeping its `IE<N>` number. Remove entries when the idea is rejected, retired, or (for AC-pointers) the AC has shipped and its file deleted. Not a historical record.

- IE3: Port `deps` — report direct dependency freshness
- IE6: Port `updatecheck` — governa's binary self-checks for a newer release on every invocation (`internal/updatecheck`, called via `defer updatecheck.Check(programVersion)` in governa's `main()`); govna has no equivalent yet
- IE8: Reconcile `templates/overlays/doc/files/AGENTS.md.tmpl` against root `AGENTS.md` — the DOC overlay copy is missing whole sections present in root (`### Build Verification`, `### AC Mechanics`, `### Errors`, `### Versioning and Dependencies`, `### Code Style and Conventions`) plus assorted rewording elsewhere, apparently never updated since those sections were added to root. Surfaced during AC6's Audit (2026-08-05), out of that AC's drift-scan-only scope
