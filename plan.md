# govna Plan

## Product Direction

Maintain govna as deterministic, dependency-free governance tooling with package-owned behavior tests and self-contained product tooling. Make recurring programming and publishing ceremonies more effective and efficient by preserving Director-owned choices and automating settled mechanics.

## Ideas To Explore

Ideas captured for future reference. [`govna/development-cycle.md`](govna/development-cycle.md) `## Notes` holds the rules for these items.

- IE1: Add complete canonical build adapters for Java, Node, and Python before making those CODE stacks selectable again; a child repository on those stacks may also need a registry publish step for npm, PyPI, or Maven Central, which no stack has today.

- IE3: Explore adopting, loading, reviewing, and removing Govna through IDE extensions and the vendors' desktop, mobile, and web agent apps; support stays limited to the Claude Code and Codex CLIs until then.

- IE7: Explore a monorepo version of Govna: one root `AGENTS.md` holding the canon once, and each project directory with its own `AGENTS.md` holding only its project rules; weigh the alternatives and whether Sapling, Jujutsu, Josh, or git's own subtree, sparse checkout, and worktrees help; research in `brainstorm.md`.

- IE14: Explore dated `vYY.MM.DDa` release tags for CODE repositories, as DOC already allows, with the MAJOR/MINOR/PATCH judgment kept in the CHANGELOG or `history.md` instead of the version; weigh that Go modules need semver tags, a major version above 1 needs a `/vN` module path, and the `a` suffix is not valid semver, so `go install`, the module proxy, and the `govna version` update check would break, and Cargo and Swift Package Manager expect semver too.

- IE19: Explore distribution child repositories, such as a Homebrew tap, a GitHub Action, or a docs site, which use the parent's release and so release after it with no pin; most have no govna stack and so no canonical `build.sh`; research in `brainstorm.md`.

