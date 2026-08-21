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

Run `govna audit` from an adopted consumer repo root to compare it against current canon and emit a `govna/ac<N>-audit-<canon-version>.md` stub listing divergences for Director review. The generated `govna/canon-baseline.txt` distinguishes untouched prior canon from consumer edits and identifies previously canonical paths removed from current canon without relying on commit history. A bounded tombstone registry bridges removals that predate baseline adoption while unrelated consumer-owned governance docs remain quiet. See [`govna/audit.md`](govna/audit.md) for the full classification model.

### `rm`

Run `govna rm` from an adopted consumer repo root to emit a Director-reviewed cleanup AC for removing govna canon. The emitted AC lists whole-file removals, preserves repo-owned content, and routes hybrid files through Director review before any deletion occurs — `rm` deletes nothing itself. Review items carry an on-demand `govna render` + `diff -ru` recipe rather than a pre-computed diff file.

### `render`

Run `govna render --flavor code|doc <target>` to render flavor-specific canon into a target directory for inspection or testing. CODE rendering infers the stack from the current directory; use `-s, --stack <name>` to override it. Go rendering reads the module path from `go.mod` unless `-m, --module-path <path>` is supplied. See `govna render -h` for full usage.

## Design

The target repo stays self-contained. govna's templates are embedded in the binary at compile time and are read-only at bootstrap — nothing is imported as a submodule, package, or runtime dependency. The bootstrap tool is a single Rust binary, so it works across macOS, Linux, and Windows without requiring a specific shell. Terminal color and usage-formatting logic is hand-rolled directly in `src/main.rs` rather than pulled in as a dependency, keeping the external crate surface minimal.

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

**Scope:** This ranking applies to conventional application and CLI repositories under repeated agentic maintenance, not universally. TypeScript tends to gain an additional advantage in web applications because of ecosystem depth and library typing. This reflects the judgment of several coding agents (Claude, OpenAI Codex, ChatGPT), not an empirical benchmark. Treat the two-axis framework as the durable claim and the ordinal ranking as a workload- and workflow-dependent heuristic.

## Current Stage

Releases, commits, and pushes remain Director-controlled; `build.sh` provides validation, release prep, and interactive release orchestration without removing that human gate. There's no branch or PR workflow yet. These are phase choices while the governance contract stabilizes.

Scope is deliberately narrow: govna aims to be a small, stable collaboration contract — not a full-stack generator or an opinionated starter kit. The fewer primitives it ships, the less there is to drift against.

The primary validation surface so far has been CLI-type coding agents. The contract is file-based and agent-agnostic in principle — desktop clients and IDE-integrated agents can read the same files — but expect rougher edges there until those patterns are exercised.

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
