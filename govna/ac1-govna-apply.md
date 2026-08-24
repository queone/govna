# AC1 Govna Apply

Adopt the govna v0.29.0 Go CODE governance baseline, adapt its repository-specific rules and product documents for the Go `govna` successor, and leave product implementation and parity architecture to a separate AC.

## Summary

Adopt the generated Go CODE governance baseline and replace its repository placeholders with the settled purpose, behavioral-reference relationship, delivery boundaries, and next product action for the Go `govna` successor.

## In Scope

### Adopted files

- `.gitignore` (written)
- `AGENTS.md` (written)
- `CHANGELOG.md` (written)
- `README.md` (written)
- `arch.md` (written)
- `build.sh` (written)
- `govna/README.md` (written)
- `govna/ac-template.md` (written)
- `govna/audit.md` (written)
- `govna/build-release.md` (written)
- `govna/canon-baseline.txt` (written)
- `govna/canon-cycle.md` (written)
- `govna/code-stacks.md` (written)
- `govna/development-cycle.md` (written)
- `govna/development-guidelines.md` (written)
- `govna/metadata.txt` (written)
- `govna/operator-contract-rationale.md` (written)
- `govna/roles.md` (written)
- `plan.md` (written)
- `CLAUDE.md` (agent alias link)

### Repository adaptations

- `AGENTS.md` — add repository-specific rules for parity with `govna-rust`, intentional-difference records, and Director-executed GitHub writes.
- `README.md` — identify this repository as the planned Go successor to the frozen Rust behavioral reference.
- `arch.md` — record the current pre-implementation boundary without choosing the product architecture.
- `plan.md` — replace the placeholder direction with behavioral parity as the next repository-local AC.

### Schema changes

- None.

## Out Of Scope

- Create Go product code, `go.mod`, packages, commands, tests, or fixtures.
- Choose the Go architecture, package layout, dependencies, or parity implementation strategy.
- Inventory or implement `govna-rust` behavior.
- Create a GitHub repository, configure a remote, publish, or release.
- Change shared Govna canon or generated build behavior.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every adopted file exists, `build.sh` is executable, and `CLAUDE.md` is a symlink to `AGENTS.md`.

**AT2** [Automated] [Pre-release gate] — Verify `AGENTS.md` records `govna-rust` as the behavioral reference, requires intentional differences to be recorded, and leaves GitHub write operations to the Director.

**AT3** [Automated] [Pre-release gate] — Verify `README.md`, `arch.md`, and `plan.md` contain no generated placeholder instructions and identify the Go successor, frozen Rust reference, current pre-implementation state, and parity AC as the next action.

**AT4** [Automated] [Pre-release gate] — Verify `govna/metadata.txt` records the CODE repository type and Go stack.

**AT5** [Manual] [Pre-release gate] — Confirm `govna/roles.md` reflects the Operator and Director delivery model.

## Status

`DEFERRED` — Ratified; Package deferred until the parity AC creates a valid Go module.
