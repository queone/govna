# govna History

One section per release, newest first: what the release added, changed, removed, or deferred in the product, its design, and its code, and why. Governance, toolchain, and release mechanics stay in `CHANGELOG.md` and git. Settled design is in `arch.md`. Search it to learn why a release changed the product.

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
