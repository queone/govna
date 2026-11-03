# govna Brainstorm

Research and open questions that are not yet ideas in `plan.md` or decisions in `arch.md`. An item leaves this file
when it becomes an AC or is rejected.

## One govna copy for many repositories

Captured 2026-10-14. Today every governed repository carries its own copy of govna: `AGENTS.md`, `build.sh`, and
the `govna/` directory, each kept current by `govna apply` and checked by `govna audit`. One Director working
across several repositories, such as `bits`, `gkit`, `govna`, `mud`, and `skout` under `github.com/queone`, pays
for that in three ways:

- Each repository is its own agent session, so moving between projects means switching session context.
- A change that spans govna canon and a consumer is two sessions, two reviews, and two commits.
- Every consumer copy of canon must be refreshed and audited on its own after each canon release.

The question: can one copy of govna govern several repositories or projects, and can monorepo tooling help?

### What the large companies did

Source: "Redesigning source control at Anthropic", 2026-10-13,
https://www.anthropic.com/engineering/redesigning-source-control

- Anthropic built Scion, an in-house version control system for its monorepo. A team of seven built it, plus the
  code review tool Antique and half a dozen related systems, and migrated the monorepo off git in five months.
- The driver was scale: about 2,500 merged changes a day in May 2026, over 30,000 a day by September, a
  main-branch history near 100 GB, about 25 million CI checkouts a day, and several hundred git mirrors at peak.
- The design assumes clients are always online, mostly agents and jobs. The unit of work is a server-side
  changeset with content-addressed blobs; checkouts are virtualized and sparse; a traditional merge queue "doesn't
  make much sense at agent scale", so post-merge and pre-deploy testing carry the load.
- Named prior art: Google's Piper and Meta's Sapling.
- The post calls git "the right design" for most repositories, says Scion "is by no means a suitable replacement
  for git for general use", and shares the ideas rather than Scion itself.

Takeaway: these systems answer scale, not governance. What carries over is the monorepo with trunk-based
development and one set of rules at the root, which git supports at any size this project will reach.

### Tools in the landscape

- Piper (Google): internal, not available.
- Sapling (Meta): open source. The `sl` client works on ordinary git repositories, including GitHub. Its scaling
  servers, Mononoke and EdenFS, are not practical outside Meta. A client UX, not a governance model.
- Jujutsu (`jj`): open source, started at Google. Uses a git repository as its backing store, so it can be tried
  on any repository and removed with no migration. Also a client UX.
- Josh: open source. A proxy that serves subdirectories of a monorepo as separate git repositories. The one tool
  here that could let a single governed monorepo still publish per-project repositories.
- Scalar: built into git since 2.38. Large-repository settings in one command; not needed at this size.
- git itself: sparse checkout in cone mode, partial clone, worktrees, subtrees, and submodules.
- Build orchestration (Bazel, Buck2, Pants, Nx, Turborepo, moon): decides what to rebuild per changed directory.
  Overlaps with `build.sh` and its canonical validation, so any adoption is a canon question.

### Shapes for one govna copy

1. Monorepo with one master contract. One repository, one root `AGENTS.md` holding the canon sections once, one
   `govna/` directory of canon documents, and one directory per project. Each project directory has its own short
   `AGENTS.md` holding only what its `## Project Rules` section holds today. No project carries a copy of the
   canon. One session sees everything and a cross-project change is one commit. This is the target shape.
2. Shared canon, separate repositories. Each repository pulls the canon from one source, as a git subtree or
   submodule at `govna/`, or by `govna apply` from a pinned release as today. Removes drift, not session switching.
3. Control checkout. Repositories stay separate; one session opens with extra working directories, for Claude
   Code `claude --add-dir ../mud ../gkit`. One session, many repositories, but each repository still carries its
   own copy and commits stay per repository.
4. Monorepo published as repositories. Shape 1 internally, with Josh serving each project directory as its own
   repository for outside consumers.

### What govna would need for shape 1

The split already exists: `govna apply` treats everything above `## Project Rules` as canon and everything below
it as repository-owned. Shape 1 turns that line inside one file into a file boundary: canon in the root
`AGENTS.md`, project rules in `PROJECT/AGENTS.md`.

- Loading. Both files must be in force before any project work. Codex merges `AGENTS.md` files from the
  repository root down to the working directory, which fits. Claude Code reads the root file at startup; whether
  it loads a project's `AGENTS.md` at startup, on first read in that directory, or not at all needs checking. If
  not, the root contract needs a rule to read the active project's file first, and `Govna contract loaded.` must
  mean both.
- Precedence. A project file extends the canon and cannot override it, as `## Project Rules` does today. The
  conflict order becomes Director instruction, root contract, project contract, govna documents, model defaults.
- Routing. Contract-integrity findings classed `Consumer-local` route to the project's `AGENTS.md` instead of a
  `## Project Rules` section; `Govna canon` findings route to the root, which is now the only canon copy.

- A project scope: the primary scope becomes a project directory instead of a repository, and the rules in
  `### Primary And Ancillary Scope` name directories.
- `govna apply`, `govna audit`, `govna remove`, and the canon baseline take a project path.
- `build.sh` runs per project, with Go modules joined by a root `go.work` and tags in the form `mud/v0.12.0`.
- AC numbering, `CHANGELOG.md`, release-message batches, and Package per project, or one global sequence with a
  project prefix. A single commit may release two projects.
- Per-project homes for what is project-owned today: `README.md`, `CHANGELOG.md`, `arch.md`, `plan.md`, and active
  AC files, either under each project directory or in shared root files with a project prefix.
- The canon-versus-consumer boundary becomes a directory boundary inside one repository, which is simpler for the
  govna project itself: canon edits and the consumer-equivalent review happen in one session.

### Costs and risks

- Every project's history moves; issues and Actions mix unless filtered by path.
- A public project may need to stay separate, or be published through Josh.
- The Director runs every git write today; one repository means every release command names its project.
- Lock-in is low: `git filter-repo --to-subdirectory-filter NAME` imports a repository with history, and the same
  tool splits it back out.

### Trial plan

1. Try shape 3 first: one Claude Code session with `--add-dir` across govna and its tightest consumer.
2. Create a trial monorepo with govna and one consumer imported by `git filter-repo`, preserving history.
3. Give `build.sh` a project scope and prefixed tags.
4. Give `govna audit` and the canon baseline a project path.
5. Make one cross-project change as a single commit and release both projects from it.
6. Decide: shape 1 for everything, shape 2 or 4 as a middle ground, or stay with shape 3.

### Open questions

- Does a project `AGENTS.md` keep the fixed `##` section list, or hold only bullets under one heading?
- Where do active AC files live: `PROJECT/govna/` or the root `govna/` with a project prefix?
- How does Package name a release batch when two projects ship from one commit?
- Should the trial repository also try Jujutsu, since it costs nothing to add or remove?

## Parent-governed child repositories

Captured 2026-11-03 from the `mud` consumer. `mud` is private; its game engine and a game verifier must be public, so
anyone can replay a game and check its fairness proof. Go modules and GitHub visibility make that a separate
repository, `mud-engine`, which `mud` imports at a pinned tag. The engine has no roadmap of its own: every change
comes from a `mud` need. Today's canon offers two poor fits:

- Full adoption makes the child a peer: two AC sequences, two sessions, two closure audits, and two Package runs for
  each engine change, and a `mud` AC that waits on another repository's AC.
- Ancillary work (`### Primary And Ancillary Scope`) needs a fresh request each time, cannot satisfy primary scope,
  tests, validation, or phase gates, and `### Agent Release` runs "no other git or GitHub write", so `rel` cannot
  release the child before the parent.

The "Costs and risks" above already name the gap: "A public project may need to stay separate, or be published
through Josh."

### The boundary

- A child repository is separate only for a structural reason: other visibility, or a package system that requires its
  own repository, such as SwiftPM's root `Package.swift` or the Terraform registry's one-repository-per-module rule.
- A child has exactly one parent, which plans, validates, and releases it.
- A child is a dependency child: the parent imports it at a pinned version, so the child releases first.
- A child uses the same CODE stack as its parent, because the parent's build script updates the pin in the parent's own
  manifest.
- A repository with its own roadmap, or several consumers that develop against it, stays a peer with full adoption,
  as `gkit` is for `macfit`, `skout`, and `attune`.
- A child that gains a second developing consumer is promoted to a peer.

### Settled design

Decided 2026-11-03 after a cross-stack and DOC review.

- Declaration: the parent lists each child's relative path in a registry file under `govna/`.
- Child files: a pointer `AGENTS.md`, the canon `build.sh` for the shared stack, that stack's `.gitignore`, and a
  `CHANGELOG.md`. The child gets no `govna/` directory and no canon copy.
- Pointer: the child's `AGENTS.md` never names the parent, so a private parent's name stays out of a public child. An
  agent opened in the child stops and asks for the parent's session.
- Adoption: `govna apply` gains a child mode with a short and a long flag. It rejects a DOC parent and a child whose
  stack differs from the parent's.
- Scope: a declared child's paths are primary scope whenever an AC's `## In Scope` names them, not ancillary work.
- ACs: one sequence, in the parent only. The child's commits and CHANGELOG use plain words without AC identifiers, and
  the parent's Package deletes the AC.
- Validation: the parent's full build runs each declared child's `build.sh` first. Each stack sets versions its own
  way (Go `programVersion`, Rust `Cargo.toml` and `Cargo.lock`, Swift and Terraform tag only), so only the child's own
  stack script can check and release it. The canon scripts already run without a `govna/` directory.
- Audit: `govna audit` in the parent also checks each child's pointer, build script, `.gitignore`, and CHANGELOG shape.
- Orphaning: when the parent removes Govna or drops a child, the child's `AGENTS.md` becomes a short orphan note: a
  parent used to govern this repository, no Govna contract applies now, and `govna apply` adopts Govna here as a
  standalone repository. The note does not name the parent. The child keeps `build.sh`, `.gitignore`, and
  `CHANGELOG.md`.
- Promotion: `govna apply` in the child recognizes a pointer or orphan `AGENTS.md` as Govna-managed and replaces it
  with the full canon. The parent drops the child first, then the child adopts.
- Sessions: the parent's session adds each child as a writable directory. Claude Code uses `--add-dir` or `/add-dir`,
  and the Codex CLI uses `--add-dir`.
- Release order: child release, parent pin update, parent full build, parent release. `rel` runs the whole chain. An
  unused child tag is harmless, while a missing one is never pinned.
- Versions: the child keeps its own strict semver and moves only when it changes. The child's release message uses
  plain words, the parent's message keeps its AC references, and the parent's `history.md` section records the
  child's version change.
- Local overrides: Package evidence uses the child's released version with no local override. Each stack adapter
  detects its own override form: Go `replace` or `go.work`, Rust path dependencies or `[patch]`, Swift
  `.package(path:)`, and Terraform `source = "../child"`.
- Consumption: git tags only. A registry publish step for crates.io, npm, PyPI, or Maven Central stays out of scope,
  and the plan's Java, Node, and Python adapter item carries that note.
- Go libraries: the Go adapter releases a repository with no `cmd/<target>/main.go`, with its version from the tag
  only, as Swift already does. Today it fails with `release: no cmd/<target>/main.go utilities were discovered`.
- DOC: out of scope as parent and as child. A DOC repository has no manifest to pin, and its natural children, such as
  docs sites, are distribution children.

### Delivery

- First AC: declaration, child files, `govna apply` and `govna rm` child modes, scope rules, validation, audit,
  orphaning, promotion, and sessions.
- Second AC: the `rel` chain, pin updates, local-override detection in every stack adapter, the child's release
  message and `history.md` record, and the Go library-only release.

### Distribution children

Deferred to a separate plan item. A distribution child uses the parent's release, so it releases after the parent and
pins nothing: a Homebrew tap formula needs the checksum of the parent's release file, and a GitHub Action or docs site
wraps a released version. Taps and Action repositories also have no govna stack, so they cannot get a canonical
`build.sh`.

Explored 2026-11-03 and parked until a real distribution repository needs support. How it differs from a dependency
child:

- Release order: it releases after the parent and consumes the parent's release.
- Pin direction: it records the parent's version, and for a Homebrew formula a checksum; the parent pins nothing.
- Stack: usually none of the CODE stacks, so it cannot be listed in `govna/children.txt`; the parent's full build would
  stop because the child's `build.sh` differs from the parent's copy.
- Checks: each kind has its own checker, such as `brew audit` and `brew test`, `actionlint`, or a site build, and most
  can run only after the parent's release exists, which breaks the validate-before-release model.
- Tags: a tap usually has none, and a GitHub Action moves a major tag such as `v1` on each release, which conflicts
  with the rule that keeps every published child tag unchanged.

Three possible shapes, lightest first:

- Docs only: treat the repository as ancillary work after the parent's release, with a short canon pattern for the
  update step, and no registry or govna files in it.
- Pointer-only kind: a second registry such as `govna/distributions.txt`, a pointer `AGENTS.md` in each repository,
  audit of the pointer only, and a final `rel` stage after the parent's push. It needs another Agent Release exception
  and its own post-release failure rules.
- Release CI: the parent's release workflow publishes the tap or Action, for example through GoReleaser's `brews:`
  section, and govna only stays out of the way.

Choose between docs only and release CI when a concrete repository appears; the pointer-only kind adds release-chain
rules and a registry that nothing needs yet.

### Against shape 4

Shape 4 keeps the child as a directory of the parent and publishes it. For a private parent it fits worse:
`git subtree split` carries the parent's private commit messages into the public repository unless tooling squashes
them, Josh needs a running proxy, and the parent builds the child from a permanent local `replace`, so "the parent
runs the published version" rests on the release script instead of Go's checksum database. A real child repository
with real tags keeps the module system honest. In a later monorepo (shape 1), a child becomes a published project
directory, so the two directions do not conflict.

### Interim in a consumer

`mud` runs `mud-engine` as a child now, under a temporary mitigation recorded only in its active AC and compatible
with canon: the Director names the child when authorizing Implement; the child is checked with the same commands its
`build.sh` would run; only parent-side evidence satisfies parent gates; the Director runs the child's release, and the
parent pins the tag before Ratify. The mitigation ends when govna ships child support and `mud` adopts it.
