# govna Plan

## Product Direction

Maintain govna as deterministic, dependency-free governance tooling with package-owned behavior tests and self-contained product tooling. Make recurring programming and publishing ceremonies more effective and efficient by preserving Director-owned choices and automating settled mechanics.

## Ideas To Explore

Ideas captured for future reference. A bullet list — each line starts with `- IE<N>: ` (sequential N) for stable references. Two kinds: (a) **idea IE** — `IE<N>: <one-liner>`, awaiting Director discussion and the AC-ceremony evaluation in `AGENTS.md` `### AC-First Workflow`; (b) **AC-pointer** — `IE<N>: <one-liner> → govna/ac<N>-<slug>.md`, pointing at a drafted AC stub not yet through critique. An idea entry that the Director sends to Draft converts to an AC-pointer at AC-draft time, keeping its `IE<N>` number. Remove entries when the idea is rejected, retired, or (for AC-pointers) the AC has shipped and its file deleted. Not a historical record.

- IE1: Add complete canonical build adapters for Java, Node, and Python before making those CODE stacks selectable again.
- IE3: Explore adopting, loading, reviewing, and removing Govna through IDE extensions and the vendors' desktop, mobile, and web agent apps; support stays limited to the Claude Code and Codex CLIs until then.
- IE7: Explore a monorepo version of Govna: one root `AGENTS.md` holding the canon once, and each project directory with its own `AGENTS.md` holding only its project rules; weigh the alternatives and whether Sapling, Jujutsu, Josh, or git's own subtree, sparse checkout, and worktrees help; research in `brainstorm.md`.
- IE11: Adopt mud's `history.md` as a canon feature: Package writes one section per release, newest first, under `Added:`, `Changed:`, `Removed:`, `Deferred:`, `Decisions:`, and `Risks:` labels; keep mud's strict growth bounds (condensed STE, one fact per bullet, at most 25 words per bullet, empty labels omitted, product and design facts only, governance and release mechanics left to `CHANGELOG.md` and git); existing consumers start at their next release with no back-fill of earlier releases.
- IE12: Adopt `~/code/program-conventions.md` in govna as attune did: a daily version check under its rules, shown in `govna version` so scripted commands stay network-free; decide between gkit's `update` package, which would be govna's first dependency, and an in-repo implementation; the shared-store rules do not apply, because govna keeps no shared data file.
