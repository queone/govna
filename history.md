# govna History

One section per release, newest first: what the release added, changed, removed, or deferred in the product, its design, and its code, and why. Governance, toolchain, and release mechanics stay in `CHANGELOG.md` and git. Settled design is in `arch.md`. Search it to learn why a release changed the product.

## v0.59.0

Changed:
- The Markdown identifier scan in every `build.sh`, and audit's copy of it,
  accept bare IE identifiers anywhere in `plan.md`.
- AC and AT identifiers in `plan.md` still fail, and a child repository still gets no allowance.
- Canon `### AC Mechanics` merges the Part and IE reservations into one rule.
- A new canon rule allows bare IE identifiers anywhere in `plan.md`.
- The canon version is v0.95.0.

Decisions:
- IE numbers live in `plan.md`, so references between items and priority lines may name them.
- The earlier scan accepted only each bullet's own label, a narrower reading than the rule's wording.
- The merged wording keeps the shared restriction in one rule and avoids a near-duplicate rule.
- The allowance needs no child qualifier,
  because `### Child Repositories` already denies a child's files every allowance.

## v0.58.0

Added:
- When `govna audit` writes an adoption AC, it lists each bare AC, AT, or IE identifier
  in the Markdown text that the repository keeps.
- Audit checks that text with the same rules as the Markdown identifier scan in `build.sh`.
- The adoption AC lists each file under `### Bare identifiers to fix`,
  with pre-adoption line numbers and one acceptance test per file.
- The JSON report gains `identifier_findings`, with a `conditional` flag for findings that wait on a Director choice.
- Canon `govna/audit.md` gains `## Markdown identifiers`.

Changed:
- The canon version is v0.94.0.

Decisions:
- Findings appear only in an adoption AC that audit writes for another reason,
  so clean and version-only results stay quiet.
- Findings attach to their file and never change its classification.
- Audit skips text that adoption replaces or deletes, and checks the local section of a boundary file.
- A symbolic link keeps what its target keeps.
- Findings that wait on a Director choice count only when that choice keeps the text.
- Requiring the scan in a kept `build.sh` stays a separate scope decision.

Risks:
- A repository that keeps its own `build.sh` runs no scan,
  and its clean and version-only audits stay silent about bare identifiers.
- The scan rules exist in Go and in shell, and a shared agreement test guards them against drift.

## v0.57.0

Added:
- Canon `### Completed Release` treats the successful end of a `## Project Rules` step that completes a release
  as a completed release.

Changed:
- Canon `### Agent Release` treats a successful agent-run release as a completed release
  only when `## Project Rules` name no later step that completes a release.
- The canon version is v0.93.0.

Decisions:
- A repository whose release ends with a later step, such as a production promote,
  names that step in `## Project Rules`.
- The next `plan.md` items then follow that later step instead of the agent-run release.
- A repository without such a step keeps today's behavior.

Risks:
- A project rule must name its step clearly, or the agent-run release still counts as the completed release.

## v0.56.0

Added:
- Every `build.sh` that Govna renders, and Govna's own, scans Markdown for bare AC, AT, and IE identifiers.
- CODE builds run the scan before the first formatting step of every full and scoped build.
- Release prep in every stack runs the scan before any write or dry-run report.
- The scan fails on each identifier that `AGENTS.md` `### AC Mechanics` does not allow,
  and names the file, line, and a placeholder fix.
- In a child repository, which the scan finds by its pointer `AGENTS.md`, no allowance applies.
- `govna/code-stacks.md` gains `## Markdown Identifier Check`, and DOC `govna/release.md` gains the same rules for prep.

Changed:
- Canon's `README.md` and `arch.md` show the named-batch example as `Package AC<N>+AC<M>`,
  because the old example broke the identifier rule.
- The canon version is v0.92.0.

Deferred:
- Class, Part, and Round identifiers stay unscanned until canon defines a written form for them.
- An audit warning for a consumer `arch.md` that keeps the old named-batch example waits in `plan.md`.

Decisions:
- Prep runs the scan, because Package writes the `history.md` section after the final full build.
- DOC runs the scan only in prep, because DOC `build.sh` has no build step.
- The scan skips `testdata/` and `tests/fixtures/`, because test fixtures are not documentation.
- `govna/release.md` keeps its sentence about no content-validation command,
  because `govna audit` matches that sentence for DOC adoptions.
- One helper block, identical in every build script, holds the scan.

Risks:
- A consumer whose Markdown holds bare identifiers fails its build after adoption until someone rewrites them.
- A CODE consumer whose `arch.md` keeps the old named-batch example fails its build after adoption.
- Rust, Swift, and Terraform prep print the scan step twice when prep also runs a build.
- An identifier directly after a non-ASCII letter counts as bare.

## v0.55.0

Added:
- Canon `AGENTS.md` gains `### Chat Format`, so Claude, Codex,
  and every other agent format chat replies by one rule set.
- Each chat sentence starts a new line, and each chat line stays at about 120 characters.
- A longer sentence breaks at a clause boundary, or else at the last space that fits.
- Chat replies number each list that the Director may act on, use plain ASCII, and give one step per reply on request.
- A summary of literal command or file output follows that output and never replaces it.
- Two AC-First Workflow rules keep related straightforward changes in one AC.
- `## Release History Format` keeps each `history.md` line at about 120 characters,
  with the chat break rules and two-space continuation lines.
- A `history.md` line keeps each path, URL, identifier, command, and quoted output whole,
  and passes the limit only when one cannot fit.

Changed:
- `### STE Replies` again limits a sentence that tells the reader to act to 20 words,
  and any other sentence to 25 words.

Decisions:
- The chat rules live in canon instead of one agent's personal rules file, because other agents never read that file.
- Lines break at sentence and clause boundaries before any other space, so each source line reads naturally.
- The 120-character limit applies to lines; the restored STE word limits govern sentence length.
- An AC split needs independent objectives, risks, release timing, or more work than one agent can do inline.
- The history line rules stay inside the history format, like its 25-word bullet limit.
- Existing `history.md` sections keep their lines, because the format applies only to sections that Package writes.
- The DOC flavor gets no history line rules, because it has no history format.

Risks:
- An agent cannot see the terminal width,
  so a terminal narrower than 120 columns still wraps some lines inside a phrase.
- Agents estimate character counts, so line lengths vary around 120.

## v0.54.2

Changed:
- The Go build reads the module path with `GOWORK=off`, so a full build passes while `go.work` names a declared child.
- The Go child recipe sets the `go.work` `go` line to the `go.mod` `go` line with `go work edit -go=<version>`.

Decisions:
- The toolchain stays pinned from `go.mod`, so the parent validates with the same toolchain it releases with.
- `GOWORK=off go list -m` replaces the old read because it keeps a major-version suffix that the coverage step needs.

Risks:
- The `go.work` version step is manual; an agent that skips it gets the toolchain error, which names the cause.

## v0.54.0

Added:
- `govna audit` detects a version-only update: nothing to update except the two version stamps, which both record the same older version.
- A version-only update prints that only the canon version changed, writes no AC and no file, and exits `0` in both modes.
- The JSON audit report gains a `version_only` field.
- Session Entry runs `govna audit --check` once when the installed canon is newer and recommends an audit only on exit `3`.

Decisions:
- A version-only update stays quiet, so a release that changes nothing in a repository leaves nothing to commit or release.
- The version stamps keep the last canon version that changed the repository, until a real update moves them.
- Each session in a repository with lagging stamps therefore runs one quick read-only check, which stays silent.
- The recorded baseline serves as the per-profile fingerprint, so no new file is needed.
- Preserved files still count as version-only, while child drift, a retired link, or a forced update rules it out.
- Stamps that record different versions still get an AC, which repairs them.

## v0.53.0

Added:
- `govna apply --child` prints a note that the parent's full build fails until the child's own `./build.sh` passes.
- The apply-child review AC states that build fact and adds a `### Follow-up` list.
- The follow-up list says to run the child's build first and to plan follow-up child work as a second AC in the same batch.
- CODE `AGENTS.md` records a child's own rules in the parent's `## Project Rules` under a `###` heading named for the child.

Changed:
- When `apply --child` replaces a child `AGENTS.md` that held its own rules, the warning names the `### <name>` heading.
- In that case the review AC adds a rules-move instruction and a manual check that each rule moved or was dropped on purpose.

Decisions:
- `apply --child` does not run the child's build, because that is slow and has side effects.
- A child's rules live under a `###` heading named for the child, so they stay findable and a later drop knows what to review.
- The rules move stays manual; Govna does not move the replaced rules itself.

## v0.52.0

Added:
- In a CODE parent, `rel` releases each changed child first, moves the parent's pin, rebuilds, and then releases the parent.
- Plain Package runs in two stages when a child changed: child preps and release commands first, then the parent.
- Each stack's prep and release refuse a local override that points at a declared child.
- The parent's Implement and Ratify validation builds against each changed child through the stack's local override.
- The Go build skips `go mod tidy` only while `go.work` names a declared child.
- The Go build releases a library with no `cmd/<target>/main.go` from its tag alone.
- A missing child stops the parent build with a clone, scoped-build, and `govna rm --child` hint.

Changed:
- CODE `### Agent Release` allows each child release under `### Child Repositories` as its only other git write.
- Prep and release stop on a malformed `govna/children.txt`, because they now read it for the override check.

Decisions:
- The child release rules sit in CODE `### Child Repositories`, so DOC `AGENTS.md` stays unchanged.
- The Operator moves each pin with the stack's own command, because shell edits of each stack's manifest are fragile.
- Plain Package pauses after the child preps, `rel` runs both stages, and the report lists child releases first.
- A failure after a published child tag is fixed forward with a child patch release, and published tags never change.
- Parent validation needs a local override, because without it the parent tests the old pinned child until the tag is public.
- Stage 2's full build runs only after every override is removed and every pin is moved, so it builds exactly what ships.
- Tidy is skipped only for a workspace that names a declared child, so a stray `go.work` cannot turn tidy off.
- A Go parent's Package report lists every `go.mod` change beyond the pin, because Ratify never saw those changes.
- A missing child keeps failing the build, with a hint, and the docs say every clone needs its children.
- Go workspace detection follows the `go env GOWORK` lookup in shell, so prep runs no `go` command.

Risks:
- Swift edit-mode detection reads SwiftPM's internal `.build/workspace-state.json`.
- A Rust `[patch]` in the user-level `~/.cargo/config.toml` is not detected.
- No real toolchain has run the override flows end to end yet; mud's next engine change checks them.
- mud's real build ran its child build first, which closes the v0.50.0 risk about real child builds.

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
