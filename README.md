# govna

Dependency-free governance tooling for CODE and DOC repositories.

Govna carries a versioned set of governance files inside one Go executable. That embedded file set is the canon. Govna can add those files to a repository, check an adopted repository for updates, prepare a Director-reviewed removal plan, or write a temporary copy for inspection.

## Why

Govna exists to make programming and publishing ceremonies—the recurring CODE and DOC checkpoints around intent, authorization, scope, review, implementation or editing, verification, and release—more effective and efficient. By making those checkpoints explicit and reusable, Directors and Operators spend less time reconstructing or renegotiating process from transient session context and more time delivering the change.

Beyond saving coordination time, the contract keeps decision-bearing choices with the human Director while giving the agent Operator clear authority for settled mechanical work. Bounded scope and testable acceptance criteria reduce ambiguity, scope drift, and missed paths; recorded decisions improve continuity across sessions; and deterministic canon plus auditing make the workflow reproducible and governance drift detectable.

Because the generated governance is file-based, adopted repositories remain self-contained, inspectable, and adaptable to local needs.

## What Govna Provides

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

## Installation

Govna requires Go 1.27 or later when building from source.

Install the latest release with:

```bash
go install github.com/queone/govna/cmd/govna@latest
```

For local development, use the repository’s canonical build, test, and installation path:

```bash
./build.sh
```

The build installs `govna` into `$(go env GOPATH)/bin` after validation succeeds.

## Usage

```text
govna v<version>
Add and maintain Govna governance files — github.com/queone/govna

Usage: govna <command> [options]

  apply                         add Govna governance files to a repository
  audit                         check a repository with Govna for updates and local changes
  rm                            write a reviewable AC for removing Govna files
  render                        write the selected built-in Govna files to a directory
  version                       print executable and embedded governance-file versions
  ver, v, --version             print executable version
  help, h                       show this help
```

Run `govna <command> -h` for command-specific flags.

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

For an existing repository, apply keeps designated repository-owned files, merges registered governance boundaries, and reports every outcome in the adoption AC. After adoption, the repository owns its generated files and may adapt them to local needs.

### `audit`

Run `govna audit` from a CODE or DOC Git worktree that has adopted Govna. Audit compares the repository's Govna-managed files with the versioned governance files built into the executable:

```bash
govna audit
```

Audit reads the repository metadata and its baseline, the saved hashes of Govna-managed file regions previously installed there. It also reads the optional preserve registry, the list of files a Director chose to keep local. Each file receives a classification, which is the exact result label explaining its state. When Govna cannot safely act, the emitted AC asks for a routing decision: a Director choice to update, keep, migrate, or remove the file. The AC also records the repository check, meaning the command to run after updates or the reason no command applies. Audit does not make those choices or modify existing governed content.

Audit stub filenames remain keyed by canon version. Their guarded markers record both the executable and canon versions; an unedited legacy canon-only marker upgrades in place without changing the AC number, while an edited body remains rejected.

Use `--json` to emit the deterministic machine report alongside the Markdown result. Use `--diff-lines <N>` to control the per-file diff truncation limit. See [`govna/audit.md`](govna/audit.md) for the classification and adoption model.

### `rm`

Run `govna rm` from a CODE or DOC Git worktree that has adopted Govna to review removal of Govna-managed files:

```bash
govna rm
```

The command labels files for deletion, preservation, or Director review and writes a guarded removal AC. It does not carry out any removal choice or delete repository content.

Removal stubs use the same canon-keyed path and dual-axis guarded-marker model as audit stubs.

### `render`

Write the selected CODE or DOC built-in governance files to a target directory for inspection or deterministic comparison. This temporary copy is a scratch render:

```bash
govna render --flavor code --stack Go --module-path example.com/my-service <target>
```

Render writes embedded Govna files only and creates no adoption record. The target is not pre-cleaned.

### `version`

Inspect both version axes. The executable version identifies the installed program; the canon version identifies its embedded governance files:

```text
$ govna version
Govna executable version: v<executable-version>
Embedded governance-file version (canon version): v<canon-version>
```

## Canon Model

Canon is the versioned set of governance files embedded into the executable at compile time. Govna writes those files deterministically. A consumer repository is any repository that has adopted Govna. It carries metadata and a baseline—the saved hashes of the Govna-managed regions previously installed there—so audit can distinguish new Govna files from local edits.

The preserve registry at `govna/preserve.txt` lists files that a Director chose to keep local. Audit keeps those decisions explicit while continuing to identify updates, migrations, and review choices elsewhere.

## Repository Types and CODE Stacks

Govna provides two overlay flavors:

- **CODE** — governance, architecture, development, build, and release support for software repositories.
- **DOC** — governance and editing support for documentation repositories.

CODE repositories have first-class stack contracts for Go, Rust, Terraform, and Swift. Each supported stack defines inference, canonical validation, installation, scoped-build, and release behavior. See [`govna/code-stacks.md`](govna/code-stacks.md) for the complete contracts.

## Design

Govna is a standard-library-only Go module. It keeps every governance template inside the executable, so adding or rendering Govna files needs no runtime package, network service, submodule, or separate template checkout.

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

The primary validation surface so far has been CLI-type coding agents. The contract is file-based and agent-agnostic in principle — desktop clients and IDE-integrated agents can read the same files — but expect rougher edges there until those patterns are exercised.

## Development and Release

Run the full canonical validation and installation path with:

```bash
./build.sh
```

Release preparation validates before and after its controlled mutations, then prints the release command without executing it:

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
