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
