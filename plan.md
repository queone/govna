# govna Plan

## Product Direction

Maintain govna as deterministic, dependency-free governance tooling with package-owned behavior tests and self-contained product tooling. Make recurring programming and publishing ceremonies more effective and efficient by preserving Director-owned choices and automating settled mechanics.

## Ideas To Explore

Ideas captured for future reference. A bullet list — each line starts with `- IE<N>: ` (sequential N) for stable references. Two kinds: (a) **idea IE** — `IE<N>: <one-liner>`, awaiting Director discussion and the AC-ceremony evaluation in `AGENTS.md` `### AC-First Workflow`; (b) **AC-pointer** — `IE<N>: <one-liner> → govna/ac<N>-<slug>.md`, pointing at a drafted AC stub not yet through critique. An idea entry that the Director sends to Draft converts to an AC-pointer at AC-draft time, keeping its `IE<N>` number. Remove entries when the idea is rejected, retired, or (for AC-pointers) the AC has shipped and its file deleted. Not a historical record.

- IE1: Add complete canonical build adapters for Java, Node, and Python before making those CODE stacks selectable again.
- IE3: Explore adopting, loading, reviewing, and removing Govna through IDE extensions and the vendors' desktop, mobile, and web agent apps; support stays limited to the Claude Code and Codex CLIs until then.
- IE7: Explore a monorepo version of Govna: one root `AGENTS.md` holding the canon once, and each project directory with its own `AGENTS.md` holding only its project rules; weigh the alternatives and whether Sapling, Jujutsu, Josh, or git's own subtree, sparse checkout, and worktrees help; research in `brainstorm.md`.
- IE14: Explore dated `vYY.MM.DDa` release tags for CODE repositories, as DOC already allows, with the MAJOR/MINOR/PATCH judgment kept in the CHANGELOG or `history.md` instead of the version; weigh that Go modules need semver tags, a major version above 1 needs a `/vN` module path, and the `a` suffix is not valid semver, so `go install`, the module proxy, and the `govna version` update check would break, and Cargo and Swift Package Manager expect semver too.
- IE15: Detect when a govna update changes nothing in the working repository's own profile, such as a DOC-only or other-stack canon change that leaves only `govna/metadata.txt` and `govna/canon-baseline.txt` to update; consider a per-profile canon fingerprint so the session-entry audit suggestion and `govna audit` stay quiet or emit a version-only result for those updates.
