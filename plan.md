# govna Plan

## Product Direction

Provide a narrow, usable governance template that bootstraps new repos — the same product governa provides, ported to Rust with the intent of eventually replacing governa as the canonical implementation, not remaining a permanent dependent alongside it. Govna is a one-time apply — after bootstrap, the consumer repo owns all governance files and evolves them independently. Template improvements are adopted by having consumer-repo agents read govna's source and cherry-pick what's useful.

## Ideas To Explore

Ideas captured for future reference. A bullet list — each line starts with `- IE<N>: ` (sequential N) for stable references. Two kinds: (a) **pre-rubric IE** — `IE<N>: <one-liner>`, awaiting director discussion and the objective-fit rubric (see `AGENTS.md` Approval Boundaries); (b) **AC-pointer** — `IE<N>: <one-liner> → govna/ac<N>-<slug>.md`, pointing at a drafted AC stub not yet through critique. A pre-rubric entry that clears the rubric converts to an AC-pointer at AC-draft time, keeping its `IE<N>` number. Remove entries when the idea is rejected, retired, or (for AC-pointers) the AC has shipped and its file deleted. Not a historical record.

- IE3: Port `deps` — report direct dependency freshness
- IE4: Port `rm` — emit cleanup AC for removing govna canon
- IE5: Port `apply` — the full governance-application flow; most involved of the five, likely depends on the embedded-template engine `render-canon` (AC4) already built
- IE6: Port `updatecheck` — governa's binary self-checks for a newer release on every invocation (`internal/updatecheck`, called via `defer updatecheck.Check(programVersion)` in governa's `main()`); govna has no equivalent yet
- IE7: Resolve "Deferred pending drift-scan" / "not yet implemented" markers now that `drift-scan` (AC5) is real — `AGENTS.md`'s `### Drift-Scan Adoption` subsection and its two Project Rules mirroring bullets, `govna/drift-scan.md`'s placeholder framing, and other references across `govna/*.md`, `templates/base/AGENTS.md`, and both overlays' template files (23 files mention drift-scan at all, per closure-audit grep — not all need changes, e.g. `arch.md`/`plan.md` already state the pre-AC5 status accurately as history; each needs individual review for stale present-tense "doesn't exist yet" claims). Same category of gap as AC4's Part C for render-canon, but AC5 didn't scope it upfront — surfaced here rather than expanded into mid-Implement
