# govna

Template repo that bootstraps governance into new repositories and helps existing ones adopt it with minimal disruption. Built from:

- a common base contract in `templates/base/`
- a repo-type overlay in `templates/overlays/code/` or `templates/overlays/doc/`
- a deterministic Rust CLI that renders templates into target repos

## Why

AI-assisted coding is here to stay, across teams that code alone, teams that are entirely human, and teams that mix both — often in the same repo across different phases. govna isn't a prerequisite for any of them. If you prefer to code without agents, govna stays out of the way. What it adds is a little order to the new paradigm: when you bring a coding agent into a repo, the collaboration contract is already explicit, versioned, and reproducible — not reinvented prompt by prompt.

The contract covers what humans and agents agree on before work starts: who is authorized to make which changes, how proposals are reviewed, what governance files mean, and how the template itself evolves. File-based and deterministic; nothing depends on transient session context.

## Roles

govna ships a closed two-role model so agent sessions have a predictable starting point:

- **Operator** — LLM agent role. Owns implementation, tests, doc alignment, and mandatory self-review. Automatic and unannounced; it is the only agent role.
- **Director** — human role. Owns intent, priorities, irreversible decisions (releases, architectural bets, scope), and the meta-loop. Not assignable to an agent.

Full role definitions and the self-review contract live in [`govna/roles.md`](govna/roles.md). The shared `AGENTS.md` contract applies in every case. The reasoning behind the contract structure — particularly the session-entry rule — is in [`govna/operator-contract-rationale.md`](govna/operator-contract-rationale.md).

## Acceptance Criteria

govna uses Acceptance Criteria (AC) as its central change-control artifact for non-trivial work: a bounded, executable contract that translates Director intent into the change the Operator implements and verifies. Every non-trivial change is AC-first.

An AC records the change summary, authoritative scope, exclusions, acceptance tests, review state, and implementation status. After critique and pre-implementation verification, the Director explicitly confirms that the AC is implementation-ready. Release prep deletes completed ACs after their durable decisions have landed in code or governing documentation.

Trivial changes may proceed without an AC when explicitly authorized; size alone does not make a change trivial.

Here, "AC" names both the acceptance-criteria document — the change blueprint — and the governed change it tracks from Draft through Package.

## Workflow at a glance

Use the standalone action vocabulary `Draft → Audit → Refine → Implement → Ratify → Package` for an active AC. Draft creates the AC; Audit, Refine, Implement, and Ratify are the four AC phases; Package is post-Ratify release preparation. Accept lowercase forms for the phase actions and `package`, `pack`, or `prep` for Package. Ordinary coding phrases such as `build`, `prepare the build`, and `package the binary` do not advance the workflow.

## Commands

| Command | Status |
| --- | --- |
| `apply` | implemented |
| `audit` | implemented |
| `rm` | implemented |
| `render` | implemented |
| `ver`, `v`, `--version` | implemented |
| `-h`, `--help`, `-?`, `help`, `h` | implemented |

## Usage

```text
govna v0.13.0
Repo governance templates — github.com/queone/govna

Usage: govna <command> [options]

  apply                         apply governance template to a repo
  audit                         drift scan an adopted repo against govna canon
  rm                            emit cleanup AC for removing govna canon
  render                        render flavor-specific canon files into a target directory
  ver, v, --version             print version
  help, h                       show this help

Run 'govna <command> -h' for command-specific flags.
```

Install the binary:

```bash
cargo install --git https://github.com/queone/govna govna
```

### `apply`

One-time governance bootstrap. Run from a target repo or empty directory. govna is read-only source — templates are embedded in the binary. After apply, all files are consumer-owned — modify freely to fit the repo's needs. No interactive prompting: missing parameters are inferred from the target directory, or supplied via flags.

```bash
govna apply
```

Or with explicit flags:

```bash
govna apply -f code -n my-service -s Rust
```

Go, Rust, Terraform, and Swift have first-class CODE overlays; each emits a stack-specific canonical `build.sh`. Other stack values are accepted but produce the generic CODE scaffold without a build script. See [`govna/code-stacks.md`](govna/code-stacks.md) for stack contracts.

Flags:

- `-f, --flavor code|doc` — overlay flavor (default: auto-detect).
- `-s, --stack <name>` — CODE stack (default: inferred from manifests).
- `-n, --repo-name <name>` — repo name (default: basename of cwd).
- `-m, --module-path <path>` — module path for Go CODE canon (default: read from `go.mod`).
- `-g, --init-git` — initialize git if the target is not a repo.
- `-h, --help` — show this help.

Existing governance artifacts are overwritten directly on repeat runs; mixed-content files (like `AGENTS.md`) are hunk-merged rather than blindly overwritten, preserving the repo-owned tail below their canon boundary.

#### Migrating from governa

Tell an agent to run `govna apply` inside a repo currently managed by governa. `apply` auto-detects a governa-managed target (`governa/metadata.txt` or `governa/ac-template.md` present), carries the legacy repo-type and stack metadata forward when govna's own metadata isn't already present, and adds a `## Migration findings` section to the same adoption AC (`govna/ac<N>-govna-apply-<version>.md`) covering the legacy `governa/` tree — one file, not a separate tracking AC. No special flag needed — this happens automatically as part of a normal `govna apply` run.

The migration findings compare against a live `governa render-canon` output when the `governa` binary is available on `PATH`, falling back to a plain file enumeration otherwise. Nothing under `governa/` is deleted automatically — review and removal are Director-driven, tracked in that section.

### `audit`

Run `govna audit` from an adopted consumer repo root to compare it against current canon and emit a `govna/ac<N>-audit-<canon-version>.md` stub listing divergences for Director review. The generated `govna/canon-baseline.txt` lets audit distinguish untouched prior canon from consumer edits without relying on commit history. See [`govna/audit.md`](govna/audit.md) for the full classification model.

### `rm`

Run `govna rm` from an adopted consumer repo root to emit a Director-reviewed cleanup AC for removing govna canon. The emitted AC lists whole-file removals, preserves repo-owned content, and routes hybrid files through Director review before any deletion occurs — `rm` deletes nothing itself. Review items carry an on-demand `govna render` + `diff -ru` recipe rather than a pre-computed diff file.

### `render`

Run `govna render --flavor code|doc <target>` to render flavor-specific canon into a target directory for inspection or testing. CODE rendering infers the stack from the current directory; use `-s, --stack <name>` to override it. Go rendering reads the module path from `go.mod` unless `-m, --module-path <path>` is supplied. See `govna render -h` for full usage.

## Design

The target repo stays self-contained. govna's templates are embedded in the binary at compile time and are read-only at bootstrap — nothing is imported as a submodule, package, or runtime dependency. The bootstrap tool is a single Rust binary, so it works across macOS, Linux, and Windows without requiring a specific shell. Terminal color and usage-formatting logic is hand-rolled directly in `src/main.rs` rather than pulled in as a dependency, keeping the external crate surface minimal.

## Current Stage

Releases, commits, and pushes remain Director-controlled; `build.sh` provides validation, release prep, and interactive release orchestration without removing that human gate. There's no branch or PR workflow yet. These are phase choices while the governance contract stabilizes.

Scope is deliberately narrow: govna aims to be a small, stable collaboration contract — not a full-stack generator or an opinionated starter kit. The fewer primitives it ships, the less there is to drift against.

The primary validation surface so far has been CLI-type coding agents, principally [Claude Code](https://github.com/anthropics/claude-code). The contract is file-based and agent-agnostic in principle — desktop clients and IDE-integrated agents can read the same files — but expect rougher edges there until those patterns are exercised.

## Self-Hosting Status

This repo is itself governed as a `CODE` repo and carries the core artifacts at the root:

- [`AGENTS.md`](AGENTS.md)
- [`arch.md`](arch.md)
- [`plan.md`](plan.md)
- [`CHANGELOG.md`](CHANGELOG.md)
- [`govna/README.md`](govna/README.md)
- [`govna/roles.md`](govna/roles.md)

## Build

```
cargo build
```

## Test

```
cargo test
```
