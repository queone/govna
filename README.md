# govna

Shared working rules for you and your coding agent, installed in your repository.

Govna helps you agree on a change, review its scope, and check the result with an AI coding agent. It adds readable instruction and workflow files to software (CODE) or documentation (DOC) repositories. You describe the work and approve decisions; the agent follows the repository's rules.

**"Coding agent" in this README means a terminal CLI: [Claude Code](https://code.claude.com/docs/en/quickstart) (`claude`) or [Codex CLI](https://learn.chatgpt.com/docs/codex/cli) (`codex`) running in your repository.** Those CLIs are Govna's primary and only tested interaction target. Govna writes the files they read at startup; you keep talking to the agent in its terminal.

**Not yet tested or supported:** the VS Code and JetBrains extensions of either agent, the Claude and ChatGPT desktop apps on Windows and macOS, their iOS and Android apps, and the web or cloud versions of either agent. They may read the same files, but nothing about Govna's workflow has been exercised there; `plan.md` tracks that exploration. Govna itself is exercised on macOS and Linux; on Windows, creating the `CLAUDE.md` link needs Developer Mode or administrator rights.

[Try it in a disposable clone](#quick-start) · [Try it in place with an instant revert](#try-it-in-place-with-an-instant-git-revert) · [Leave the trial or remove Govna](#leave-the-trial-or-remove-govna) · [Command reference](#usage)

## Quick Start

### 1. Install Govna and one agent CLI

You need Git, Go 1.27 or later, and one signed-in CLI: follow the [Claude Code quickstart](https://code.claude.com/docs/en/quickstart) or the [Codex CLI guide](https://learn.chatgpt.com/docs/codex/cli). The shell examples use Bash or Zsh.

```bash
go install github.com/queone/govna/cmd/govna@latest
export PATH="$(go env GOPATH)/bin:$PATH"
govna version
```

If you set a custom `GOBIN`, add that directory to `PATH` instead. The `PATH` change lasts for the current shell session.

### 2. Adopt Govna in a disposable clone

Pick a Go, Rust, Swift, or Terraform code repository, or a documentation repository; other stacks are not yet selectable. Clone it to a new directory so your real checkout, including uncommitted work, is never touched:

```bash
git clone /path/to/your-repo govna-trial
cd govna-trial
govna apply
git status --short
```

`apply` writes immediately and prints the path of an adoption AC, a short review record of what it wrote, merged, or kept. It keeps an existing `README.md`, `CHANGELOG.md`, `arch.md`, and `plan.md`; merges `AGENTS.md` below its `## Project Rules` boundary when one exists and replaces it otherwise; and overwrites `.gitignore`, `build.sh`, and everything under `govna/`. It saves no previous contents, which is why this walkthrough uses a clone. Govna detects the repository type and stack; when it cannot, pass `--flavor code|doc` and `--stack` as shown under [`apply`](#apply).

### 3. Start the agent and confirm it sees the rules

Run one of these from `govna-trial`:

| Agent | Command | How it loads the rules |
| --- | --- | --- |
| Claude Code | `claude` | [Reads `CLAUDE.md`](https://code.claude.com/docs/en/memory), which Govna links to `AGENTS.md`. Type `/context` and check that `CLAUDE.md` is listed under Memory files. |
| Codex CLI | `codex` | [Reads `AGENTS.md` before doing any work](https://learn.chatgpt.com/docs/agent-configuration/agents-md). |

The contract makes the agent begin its first substantive reply with the line `Govna contract loaded.` If that line never appears, the rules did not load. When `apply` warned that an existing regular `CLAUDE.md` was kept, that file is why Claude Code did not see them.

### 4. Review the adoption, then try one change

Type this into the agent chat, replacing the placeholder with the path `apply` printed:

```text
Audit <adoption-document-path>
```

The agent reviews what was written, merged, or kept, explains any warning in plain language, and asks for any decision it needs. That is the whole adoption review for a trial.

Then describe a small outcome you want and ask the agent to `Draft` an acceptance-criteria document for it. The agent audits its own draft, pauses for your decisions, and changes nothing until you say the document is implementation-ready. When it reports completion, say `Ratify` to accept. These words are Govna's [workflow vocabulary](#workflow-at-a-glance); the agent explains each step as it goes.

### Try it in place with an instant Git revert

If you would rather adopt in your usual checkout, make the tree clean first so Git can restore it exactly:

```bash
git status --short   # must print nothing; commit or `git stash -u` first
govna apply
```

To put everything back the way it was:

```bash
git restore .        # restores every tracked file apply changed
git clean -fd        # removes the new files: AGENTS.md, CLAUDE.md, govna/, and the adoption AC
git status --short   # prints nothing again
```

`git clean -fd` deletes every untracked file and directory, not only Govna's, which is why the tree must start clean. Ignored files are left alone, and Govna never writes to them.

## Leave the Trial or Remove Govna

Three exits, fastest first:

1. **Discard the clone.** Exit the agent, return to your original checkout, and delete `govna-trial`. Nothing in the original changed.
2. **Revert in place.** Run the `git restore .` and `git clean -fd` pair above in a checkout that was clean before `govna apply`.
3. **Remove Govna from a repository you kept working in.** Once you have commits on top of the adoption, Git cannot separate Govna's files from your work, so use `govna rm`.

```bash
govna rm
```

**`govna rm` writes a removal plan; it deletes nothing and cannot restore files that `apply` overwrote.** Complete the removal through the agent:

1. Open the removal AC at the path `govna rm` printed.
2. Tell the agent `Audit <removal-document-path>` and ask it to explain every choice while keeping your project content.
3. Resolve the listed choices about deleting Govna files, keeping edited ones, or removing Govna sections from mixed files. Ask the agent to `Refine` the removal AC, then, when it reports readiness, tell it the AC is implementation-ready and to implement the approved removals.
4. Check `git status --short` and `git diff`, then start a fresh agent session so it reads whatever instructions remain.

Removal keeps `plan.md`, `arch.md`, every file registered in `govna/preserve.txt`, and repository-owned files with no Govna counterpart; it deletes the preserve registry last. Files with both Govna and local content wait for your choice. Anything `apply` overwrote comes back only from your Git history or a backup.

## Why

Govna exists to make programming and publishing ceremonies—the recurring CODE and DOC checkpoints around intent, authorization, scope, review, implementation or editing, verification, and release—more effective and efficient. By making those checkpoints explicit and reusable, Directors and Operators spend less time reconstructing or renegotiating process from transient session context and more time delivering the change.

Beyond saving coordination time, the contract keeps decision-bearing choices with the human Director while giving the agent Operator clear authority for settled mechanical work. Bounded scope and testable acceptance criteria reduce ambiguity, scope drift, and missed paths; recorded decisions improve continuity across sessions; and versioned governance files plus auditing make the workflow reproducible and governance drift detectable.

Because the generated governance is file-based, adopted repositories remain self-contained, inspectable, and adaptable to local needs.

## What Govna Provides

Govna carries a versioned set of governance files inside one dependency-free Go executable. That embedded file set is the canon.

- Deterministic embedded canon for CODE and DOC repositories.
- A two-role Director and Operator collaboration model.
- An Acceptance Criteria workflow for bounded, reviewable changes.
- Stack-aware CODE overlays for Go, Rust, Terraform, and Swift.
- Non-interactive `apply`, `audit`, `rm`, and `render` commands.
- Canonical build scripts with validation, installation, and release support.

## Roles

Govna uses a closed two-role model:

- **Operator** — the coding agent responsible for implementation, tests, documentation alignment, and self-review.
- **Director** — the human responsible for intent, priorities, scope, and irreversible or decision-bearing actions.

The full role definitions and review contract live in [`govna/roles.md`](govna/roles.md). The reasoning behind the session-entry contract is documented in [`govna/operator-contract-rationale.md`](govna/operator-contract-rationale.md).

## Acceptance Criteria

An Acceptance Criteria document, or AC, translates Director intent into a bounded change that an Operator can implement and verify. It records the summary, scope, exclusions, acceptance tests, review state, and current status for non-trivial work.

Here, “AC” refers both to that document and to the governed change it tracks.

## Workflow at a Glance

Govna uses the standalone action vocabulary:

```text
Draft → Audit → Refine → Implement → Ratify → Package
```

Draft creates the AC. Audit, Refine, Implement, and Ratify are its four phases. Package is the separate post-Ratify release-preparation action.

Audit, Refine, Implement, and Ratify target one eligible AC. The pending release batch contains every unpackaged AC whose implementation is present, including work still awaiting Ratify. Before another AC enters Implement, the Operator privately checks that the projected plus-joined references and a brief result summary can fit one 80-byte message; that calculation does not start Package. Package requires every pending member to be Ratified and targets the complete batch. Use `Package AC70+AC71` to establish a fitting multi-AC batch, or use standalone `Package`, `package`, `pack`, or `prep` after a complete batch is already known. Prep rejects an oversized batch, a partial batch, and a smaller batch that would leave implemented work outside the release. An empty release batch, one with no implemented AC awaiting release, packages direct-handled changes with a release message that names no AC.

## Usage

```text
govna v<version>
Add and maintain Govna governance files
github.com/queone/govna

Usage
  govna COMMAND [options]

Commands
  apply    add Govna governance files to a repository
  audit    check a repository with Govna for updates and local changes
  rm       write a reviewable AC for removing Govna files
  render   write the selected built-in Govna files to a directory
  version  print executable and embedded governance-file versions
  help     show this help

  Run 'govna COMMAND -h' for command-specific options.

Options
  -v, --version   print executable version
  -h, -?, --help  show this help
```

Run `govna COMMAND -h` for command-specific options.

### `apply`

Run `govna apply` from the target repository or empty directory. Adding Govna's governance files to a repository is adoption. Govna determines the repository type, writes the selected CODE or DOC file set (the flavor), and creates an adoption AC for review.

The adoption AC records the executable version—the version of the installed `govna` program—separately from the canon version, which identifies the governance files embedded in that program.

```bash
govna apply
```

Supply explicit values when inference is not appropriate:

```bash
govna apply --flavor code --stack Go --repo-name my-service --module-path example.com/my-service
```

Flags:

- `-f, --flavor code|doc` — select the CODE or DOC Govna file set; otherwise auto-detect it.
- `-s, --stack <name>` — select the CODE stack; otherwise infer it from manifests.
- `-n, --repo-name <name>` — set the repository name; otherwise use the current directory name.
- `-m, --module-path <path>` — set the Go module path; otherwise read it from `go.mod`.
- `-g, --init-git` — initialize Git on `main` when the target is not already a repository.

Apply keeps an existing `README.md`, `CHANGELOG.md`, `arch.md`, or `plan.md`, merges every registered governance boundary whether or not an agent instruction file already exists, recognizes a boundary with LF or CRLF line endings, and reports every outcome in the adoption AC. Before writing anything, apply validates every destination: it rejects a symbolic link at a managed path or intermediate directory (the `CLAUDE.md` alias link excepted), a directory or special file where a regular file belongs, and any path outside the target, naming the path and the recovery action. Every read and write runs through a handle contained in the resolved target. After adoption, the repository owns its generated files and may adapt them to local needs.

### `audit`

Run `govna audit` from a CODE or DOC Git worktree that has adopted Govna. Audit compares the repository's Govna-managed files with the versioned governance files built into the executable:

```bash
govna audit
```

Audit reads the repository metadata and its baseline, the saved hashes of Govna-managed file regions previously installed there. It also reads the optional preserve registry, the list of files a Director chose to keep local. Each file receives a classification, which is the exact result label explaining its state. When Govna cannot safely act, the emitted AC asks for a routing decision: a Director choice to update, keep, migrate, or remove the file. The AC also records the repository check, meaning the command to run after updates or the reason no command applies. Audit does not make those choices or modify existing governed content.

Audit reads every governed path without following links. It stops before emission, naming the path and its recovery action, when a path is a link, a directory, a special file, or unreadable, or when the saved baseline holds an entry that is not a normalized repository-relative path. Preserve phrases are read from the canonical Unreleased table row as well as from a legacy `## Unreleased` section.

Audit stub filenames remain keyed by canon version. Their guarded markers record both the executable and canon versions; an unedited legacy canon-only marker upgrades in place without changing the AC number, while an edited body remains rejected.

An explicit agent-mediated request to run `govna audit` also authorizes immediate review of one emitted or reused adoption AC. The executable still performs only deterministic comparison and emission. The Operator performs Audit, completes no-edit Refine after every blocker is resolved, runs Pre-Implementation Verification, reports readiness only when that checklist passes, and stops before Implement. A clean result or pre-emission failure enters no AC phase. A correction that would change the immutable AC requires a new audit emission.

Use `--json` to emit the deterministic machine report alongside the Markdown result. Use `--diff-lines <N>` to control the per-file diff truncation limit. See [`govna/audit.md`](govna/audit.md) for the classification and adoption model.

### `rm`

Run `govna rm` from a CODE or DOC Git worktree that has adopted Govna to review removal of Govna-managed files:

```bash
govna rm
```

The command labels files for deletion, preservation, or Director review and writes a guarded removal AC. It does not carry out any removal choice or delete repository content.

Follow [Leave the Trial or Remove Govna](#leave-the-trial-or-remove-govna) to review and implement that plan through your agent CLI. Removal does not recover pre-adoption file contents.

Removal stubs use the same canon-keyed path and dual-axis guarded-marker model as audit stubs.

### `render`

Write the selected CODE or DOC built-in governance files to a target directory for inspection or deterministic comparison. This temporary copy is a scratch render:

```bash
govna render --flavor code --stack Go --module-path example.com/my-service <target>
```

Render writes embedded Govna files only and creates no adoption record. The target is not pre-cleaned. Render validates destinations the same way apply does, rejects links, directories, and special files at managed paths before writing anything, and replaces a regular `CLAUDE.md` file with the alias link.

### `version`

Inspect both version axes. The executable version identifies the installed program; the canon version identifies its embedded governance files:

```text
$ govna version
govna v<executable-version>
Embedded governance-file version (canon version): v<canon-version>
```

## Canon Model

Canon is the versioned set of governance files embedded into the executable at compile time. Govna writes those files deterministically. A consumer repository is any repository that has adopted Govna. It carries metadata and a baseline—the saved hashes of the Govna-managed regions previously installed there—so audit can distinguish new Govna files from local edits.

The preserve registry at `govna/preserve.txt` lists files that a Director chose to keep local. Audit keeps those decisions explicit while continuing to identify updates, migrations, and review choices elsewhere.

## Repository Types and CODE Stacks

Govna provides two overlay flavors:

- **CODE** — governance, architecture, development, build, and release support for software repositories.
- **DOC** — governance and editing support for documentation repositories.

CODE repositories can select only Go, Rust, Swift, and Terraform because those stacks have complete canonical build adapters. Each supported stack defines inference, canonical validation, installation, scoped-build, and release behavior. See [`govna/code-stacks.md`](govna/code-stacks.md) for the complete contracts.

## Choosing a DOC Kind and Page Types

This section is advice. Govna does not enforce it, record it, or copy it into a repository. Editorial structure stays the repository owner's domain.

Two questions are worth settling when you start a DOC repository. The first is what the whole repository is. The second is what each page is.

**What the repository is.** Each kind of document is written, ordered, and released in its own way:

| Kind             | Unit you write   | Order                          | What a release means            | Does a candidate-judging step help? |
| ---------------- | ---------------- | ------------------------------ | ------------------------------- | ----------------------------------- |
| Blog             | Post             | By date                        | New posts go live               | Sometimes                           |
| Wiki             | Page             | None; pages link to each other | A batch of page edits           | Rarely                              |
| Book             | Chapter          | Linear                         | A draft milestone or an edition | No; the outline decides             |
| Notes collection | Entry            | None; grouped by topic         | New and revised entries         | Yes                                 |
| Novel            | Chapter or scene | Linear                         | A draft milestone               | No; the outline decides             |
| Script           | Scene            | Linear                         | A draft revision                | No; the outline decides             |

A candidate-judging step is a short review that decides whether a raw idea earns a place before anyone drafts it.

**What each page is.** The [Diátaxis](https://diataxis.fr/) scheme sorts documentation pages into four types:

- **Tutorial** — a lesson that teaches a newcomer by doing.
- **How-to guide** — the steps that solve one specific problem.
- **Reference** — facts to look up while working.
- **Explanation** — background that builds understanding.

A notes collection may prefer a looser set, such as opinion, explainer, how-to, reference, and quotation.

## Design

Govna is a standard-library-only Go module. It keeps every governance template inside the executable, so adding or rendering Govna files needs no runtime package, network service, submodule, or separate template checkout.

Every command reaches repository files through one contained-access helper in `internal/repository`. It validates each path as normalized and repository-relative, rejects symbolic links at managed paths, and performs each operation through a handle rooted at the resolved target, so a link substituted after preflight cannot escape it.

The canonical build checks generated apply, audit, and removal ACs for direct imperative instructions, one action per instruction, expected wording, and every expected output branch. These language checks run separately from byte-for-byte fixture comparisons.

Command output is deterministic and terminal color is gated by TTY capability, `NO_COLOR`, `TERM=dumb`, and 256-color support. The generated build scripts are self-contained and remain compatible with their documented stack environments.

See [`arch.md`](arch.md) for the component and data-flow overview.

## Language for AI-Assisted Coding

Efficient programming ceremonies depend partly on how quickly an Operator can understand, change, and verify the code. Language and stack choices therefore affect workflow efficiency, not just implementation style.

Stack choice (`apply`'s `-s, --stack`) is a Director decision, but Operator readability is a real input, distinct from human readability — and it splits into two axes that don't always agree.

* **Comprehension readability**: how fast an agent can understand what code does. Favors explicit control flow, few equivalent ways to write the same thing, a canonical formatter, and minimal macro/metaprogramming machinery.
* **Correctness readability**: how clearly valid and invalid states are encoded, and how fast the toolchain flags a broken edit. Favors static types paired with a fast, high-signal checker (`cargo check`, `tsc`, `go build`) over relying on tests and docstrings that can drift from behavior.

Architecture, framework choice, strictness settings, validation speed, and repository structure can outweigh the language pick — treat the ranking below as a tiebreaker among otherwise comparable stacks, not the primary lever.

The ranking also depends on workflow and review intensity. **More autonomous maintenance** rewards compiler-enforced invariants that can reject bad edits without relying on a reviewer — Rust gains ground here. **Human-in-the-loop iteration** rewards fast feedback, low edit friction, and easy diff review — TypeScript's `tsc` loop plus runtime validation (Zod, Valibot, etc.) can be especially effective, since human review covers some classes of errors the type system does not.

| Rank | Language            | Note                                                                                                                                                                                                                                                                                                                  |
| ---- | ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1    | Go                  | Canonical formatter, explicit control flow, minimal metaprogramming, and fast tooling make it the best low-complexity default across workflows. Its comparatively limited type-level expressiveness leaves some semantic contracts implicit, so the rank should not be read as strongest correctness guarantees.      |
| 2    | TypeScript (strict) | Structural types give strong call-site guidance without runtime tracing, and `tsc` provides a fast feedback loop — particularly strong for human-in-the-loop application work. Types erase at runtime, structural typing has escape hatches (`any`, assertions), and external data still requires runtime validation. |
| 3    | Rust                | Compiler-enforced invariants — ownership, traits, exhaustive matching, and lifetimes — give agents unusually strong constraints and useful repair signals. It becomes more attractive as review decreases, but the additional language machinery and compile-time friction can reduce iteration throughput.           |
| 4    | Python              | Reads cleanly in isolation; correctness leans heavily on tests unless mypy or Pyright is enforced project-wide, which remains opt-in and often partial in practice.                                                                                                                                                   |
| 5    | JavaScript          | Familiar syntax and fast tooling support rapid edits, but implicit coercion, runtime-only contracts, mutable object shapes, and mixed module conventions increase ambiguity. JSDoc with `checkJs` helps, though strict TypeScript provides a stronger and more consistent contract.                                       |
| 6    | Java / C#           | Explicit and tool-friendly by default; reflection-heavy DI, annotation frameworks, runtime proxies, and convention-driven behavior can erode that advantage.                                                                                                                                                          |
| 7    | C                   | Simple syntax hides implicit contracts — pointer ownership, aliasing, unchecked buffer lengths — that allow incorrect code to remain locally plausible.                                                                                                                                                               |
| 8    | C++                 | Very large semantic surface area — templates, macros, overload resolution, implicit conversions, lifetime hazards, and unsafe semantics compound reasoning cost.                                                                                                                                                      |

The top three are close enough that their ordering should not be treated as stable across repositories. Go is the strongest low-complexity baseline; TypeScript often maximizes iteration throughput in typed application code; Rust gains relative value as autonomous operation increases and compiler-enforced invariants substitute for some human review. None of these mechanisms guarantees logic correctness.

Reflection-heavy frameworks — heavy DI containers, declarative/metaclass-driven ORMs, runtime proxies — erode comprehension readability in any language by moving behavior away from what an agent can directly search and trace. Repository observability often matters as much as language choice: small modules, explicit boundaries, typed validation at external-data boundaries, deterministic builds, focused tests, fast checks, and searchable control flow.

**Scope:** This ranking applies to conventional application and CLI repositories under repeated agentic maintenance, not universally. TypeScript tends to gain an additional advantage in web applications because of ecosystem depth and library typing. This reflects the judgment of several CLI-type coding agents, not an empirical benchmark. Treat the two-axis framework as the durable claim and the ordinal ranking as a workload- and workflow-dependent heuristic.

## Current Stage

Releases, commits, and pushes remain Director-controlled; `build.sh` provides validation, release prep, and interactive release orchestration without removing that human gate. There's no branch or PR workflow yet. These are phase choices while the governance contract stabilizes.

Scope is deliberately narrow: govna aims to be a small, stable collaboration contract — not a full-stack generator or an opinionated starter kit. The fewer primitives it ships, the less there is to drift against.

The current interaction scope is the Claude Code and Codex terminal CLIs. IDE extensions and the vendors' desktop, mobile, and web agent apps remain untested and unsupported; `plan.md` tracks that exploration. A shared file format alone does not establish support.

## Development and Release

Run the full canonical validation and installation path with:

```bash
./build.sh
```

The build installs `govna` into `$(go env GOPATH)/bin` after validation succeeds.

Release preparation performs bookkeeping only, then prints the release command without executing it:

```bash
./build.sh prep vX.Y.Z "release message"
```

Commits, tags, releases, and publication remain Director-controlled. See [`govna/build-release.md`](govna/build-release.md) for the complete build and release contract.

## Self-Hosting

This repository governs itself as a CODE repository. Its core governance and project artifacts include:

- [`AGENTS.md`](AGENTS.md)
- [`arch.md`](arch.md)
- [`plan.md`](plan.md)
- [`CHANGELOG.md`](CHANGELOG.md)
- [`govna/README.md`](govna/README.md)
- [`govna/roles.md`](govna/roles.md)
- [`govna/development-cycle.md`](govna/development-cycle.md)
- [`govna/build-release.md`](govna/build-release.md)
