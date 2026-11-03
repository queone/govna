# govna History

One section per release, newest first: what the release added, changed, removed, or deferred in the product, its design, and its code, and why. Governance, toolchain, and release mechanics stay in `CHANGELOG.md` and git. Settled design is in `arch.md`. Search it to learn why a release changed the product.

## v0.51.0

Added:
- CODE and DOC `govna/roles.md` tell the Operator to run a check itself when current authorization allows it, instead of listing it under `Not checked`.
- `Not checked` lists only checks that need a Director action or a new Director authorization, and names that action or authorization.

Changed:
- The Package `Not checked:` line states the remaining check that needs the Director, instead of the remaining manual check.

Decisions:
- The examples are a read-only command, a file inspection, or a render check, each only within current authorization.
- Scratch reviews are not an example, because they create files, and creating files needs authorization.
- `AGENTS.md` gets no new rule, because the self-review detail belongs in `govna/roles.md`.

Risks:
- The Operator judges what current authorization allows, so an agent can still skip a check that it should run.

## v0.50.0

Added:
- `govna apply --child <path>` adds a same-stack child repository beside a CODE parent and lists it in `govna/children.txt`.
- A child gets a pointer `AGENTS.md`, the stack build tooling, `.gitignore`, and `CHANGELOG.md`, and no `govna/` directory.
- `govna audit` checks each declared child's files and names them by parent-relative path, such as `../engine/build.sh`.
- Audit always updates a child pointer or build tooling file, and gives a child `.gitignore` the ordinary choices.
- Audit checks only the shape of a child `CHANGELOG.md`, and the new `manual-fix` label marks a broken shape.
- `govna/preserve.txt` can keep a declared child's `.gitignore`.
- `govna rm` rewrites each child `AGENTS.md` to an orphan note, and `govna rm --child` drops one child.
- `govna render --child` writes the child files, and `govna render --orphan` writes the orphan note.
- Every CODE stack's full build runs each declared child's `build.sh` first.
- CODE `AGENTS.md`, `development-cycle.md`, `code-stacks.md`, and `audit.md` add child repository rules.

Changed:
- `govna apply` replaces an unedited child pointer or orphan note with the full Govna files.
- Audit ACs with child files confirm parent files against the CODE render and child files against the child render.

Deferred:
- Child release from the parent's release, the parent pin update, and override detection, to a later AC.
- Distribution children, such as a Homebrew tap, a GitHub Action, or a docs site.

Decisions:
- A child is a separate same-stack repository that the parent imports at a pinned version.
- Each child keeps its own stack build script, so the design works for every CODE stack.
- The pointer never names the parent, so a public child does not expose a private parent.
- A dropped child gets an orphan note that tells a later agent to run `govna apply` there.
- A preserve entry outside the parent can name only a child `.gitignore`, because the pointer and tooling must match canon.
- The parent skips child builds for a scoped build, and fails when a child's tooling differs from its own.
- DOC repositories have no children, and DOC `audit.md` omits the child text.
- The Director releases a child by hand until the child release AC ships.

Risks:
- No release check has run a real child build with a real toolchain yet.
- The parent full build runs each child's `build.sh` with all of its side effects.

## v0.49.0

Added:
- CODE agents search `history.md` before Draft, Audit, or Refine, and read only the matching sections.
- CODE Session Entry treats each recorded `history.md` decision as settled until the Director reopens it.
- The `history.md` header tells readers to search it to learn why a release changed the product.

Decisions:
- Agents search `history.md` instead of reading it in full, because mud's file already holds about 19,000 tokens.
- The settled-decision rule sits in Session Entry, so it also covers direct-handled changes and chat advice.
- History bullets keep the 25-word limit, the Simplified Technical English limit for descriptive text.
- The CODE `README.md` starter gets no `history.md` link, because consumers preserve their own `README.md`.

Risks:
- A search can miss a decision that `history.md` records under other terms.

## v0.48.0

Added:
- CODE Package writes one `history.md` section per release before prep runs.
- Each section records what the release added, changed, removed, and deferred, with each Director decision and residual risk.
- CODE `govna/build-release.md` defines the section format: labels in fixed order, one fact per bullet, at most 25 words.
- The CODE `arch.md` starter lists `history.md` under Core Files.

Decisions:
- DOC repositories get no history rules, because their documents already state their content.
- The agent creates `history.md` at the first Package, so `apply`, `audit`, and `rm` need no change.
- `govna rm` keeps `history.md` as a repository-owned file.
- Existing consumers start at their next release and back-fill no earlier release.
- govna counts canon changes as product facts, because canon is its product.

Risks:
- mud keeps its local copies of the history rules until it retires them after it adopts canon v0.84.0.
