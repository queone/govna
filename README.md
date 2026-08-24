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
