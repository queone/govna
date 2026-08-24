# govna

Dependency-free governance tooling for CODE and DOC repositories.

Govna embeds a deterministic governance canon in a single Go binary. It can bootstrap that canon into a repository, inspect an adopted repository for drift, prepare a Director-reviewed removal plan, or render the canon for inspection.

## Why

AI-assisted development works better when the collaboration contract is explicit, versioned, and reproducible instead of being reconstructed from transient session context.

Govna gives human Directors and agent Operators a shared, file-based contract for authorization, scope, review, implementation, and release decisions. Repositories remain self-contained after adoption, and the generated governance can be adapted to local needs.

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
Repo governance templates — github.com/queone/govna

Usage: govna <command> [options]

  apply                         apply governance template to a repo
  audit                         drift scan an adopted repo against govna canon
  rm                            emit cleanup AC for removing govna canon
  render                        render flavor-specific canon files into a target directory
  version                       print binary and embedded canon versions
  ver, v, --version             print binary version
  help, h                       show this help
```

Run `govna <command> -h` for command-specific flags.

### `apply`

Run `govna apply` from the target repository or empty directory. Govna detects the repository shape, resolves omitted parameters, writes the selected canon, and emits an adoption AC for review.

```bash
govna apply
```

Supply explicit values when inference is not appropriate:

```bash
govna apply --flavor code --stack Go --repo-name my-service --module-path example.com/my-service
```

Flags:

- `-f, --flavor code|doc` — select the overlay flavor; otherwise auto-detect it.
- `-s, --stack <name>` — select the CODE stack; otherwise infer it from manifests.
- `-n, --repo-name <name>` — set the repository name; otherwise use the current directory name.
- `-m, --module-path <path>` — set the Go module path; otherwise read it from `go.mod`.
- `-g, --init-git` — initialize Git on `main` when the target is not already a repository.

For an existing repository, apply preserves designated consumer-owned files, merges registered governance boundaries, and reports every outcome in the adoption AC. After adoption, the repository owns its generated files and may adapt them to local needs.

### `audit`

Run `govna audit` from an adopted CODE or DOC Git worktree to compare its governed files with embedded canon:

```bash
govna audit
```

Audit validates the repository’s metadata, canon baseline, and optional preserve registry. It classifies drift and emits one guarded routing AC only when it finds actionable work. It does not apply routing decisions or modify existing governed content.

Use `--json` to emit the deterministic machine report alongside the Markdown result. Use `--diff-lines <N>` to control the per-file diff truncation limit. See [`govna/audit.md`](govna/audit.md) for the classification and adoption model.

### `rm`

Run `govna rm` from an adopted CODE or DOC Git worktree to assess removal of Govna canon:

```bash
govna rm
```

The command classifies paths for deletion, preservation, or Director review and emits a guarded cleanup AC. It does not execute any route or delete repository content.

### `render`

Render flavor-specific canon into a target directory for inspection or deterministic comparison:

```bash
govna render --flavor code --stack Go --module-path example.com/my-service <target>
```

Render writes canon files only and creates no adoption record. The target is not pre-cleaned.

### `version`

Inspect both the product and embedded canon versions:

```text
$ govna version
govna binary: v<binary-version>
embedded canon: v<canon-version>
```

## Canon Model

Canon is embedded into the binary at compile time and rendered deterministically. Adopted repositories carry metadata and a generated baseline so audit can distinguish canon changes from consumer edits without depending on transient session state.

Consumer-owned divergence can be recorded in `govna/preserve.txt`. Audit keeps those decisions explicit while continuing to identify sync, migration, and review paths elsewhere.

## Repository Types and CODE Stacks

Govna provides two overlay flavors:

- **CODE** — governance, architecture, development, build, and release support for software repositories.
- **DOC** — governance and editing support for documentation repositories.

CODE repositories have first-class stack contracts for Go, Rust, Terraform, and Swift. Each supported stack defines inference, canonical validation, installation, scoped-build, and release behavior. See [`govna/code-stacks.md`](govna/code-stacks.md) for the complete contracts.

## Design

Govna is implemented as a standard-library-only Go module. Its templates are embedded in the executable, so rendering and adoption require no runtime package, network service, submodule, or template checkout.

Command output is deterministic and terminal color is gated by TTY capability, `NO_COLOR`, `TERM=dumb`, and 256-color support. The generated build scripts are self-contained and remain compatible with their documented stack environments.

See [`arch.md`](arch.md) for the component and data-flow overview.

## Language for AI-Assisted Coding

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
