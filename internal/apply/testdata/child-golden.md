# AC1 Review Child Repository ../engine

## Summary

Govna executable v9.8.7 added its embedded child files (canon {{CANON_VERSION}}) for the Go child repository `../engine` and listed it in `govna/children.txt`. The list below records whether each file was written, replaced, or kept. The parent's full `./build.sh` builds `../engine` first and fails until the child's own `./build.sh` passes.

## In Scope

Files Govna processed:

- `../engine/.gitignore` (written)
- `../engine/AGENTS.md` (written)
- `../engine/CHANGELOG.md` (written)
- `../engine/build.sh` (written)
- `govna/children.txt` (lists `../engine`)

Files this AC updates in the parent:

- `AGENTS.md` (update: child rules)
- `govna/audit.md` (update: child rules)
- `govna/build-release.md` (update: child rules)
- `govna/development-cycle.md` (update: child rules)
- `govna/canon-baseline.txt` (update: child rules)

### Parent Profile Update

This parent declares its first child, so its contract and documents gain the child rules.

- Complete this section before any other work in this AC.
- Create a temporary copy of the parent's Govna files with `govna render --with-children <scratch>`.
- Apply each file below from that copy through `govna/audit.md` `## Adoption procedure`.
- Replace only the text above the registered boundary heading of a file that has one.
- Keep that boundary heading and every line below it.
- Replace each updated file's line in `govna/canon-baseline.txt` with its line from `<scratch>/govna/canon-baseline.txt` after that file matches the copy.

Files to update:

- `AGENTS.md`
- `govna/audit.md`
- `govna/build-release.md`
- `govna/development-cycle.md`
- `govna/canon-baseline.txt`

### Follow-up

- Run `./build.sh` in `../engine` before the parent's next full build.
- Plan follow-up child work as a second AC in the same release batch as this AC.

## Out Of Scope

- Files not listed above.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every written child file matches `govna render --child` for the Go stack.

**AT2** [Automated] [Pre-release gate] — Verify `./build.sh` runs the build of `../engine` before its own steps.

**AT3** [Automated] [Pre-release gate] — Verify each updated parent file matches the temporary copy above its registered boundary.

**AT4** [Automated] [Pre-release gate] — Verify every line from the boundary heading down is unchanged in each updated parent file.

**AT5** [Automated] [Pre-release gate] — Verify each updated file's line in `govna/canon-baseline.txt` equals its line in the same temporary copy's baseline.

## Status

`PENDING` — apply emission; awaiting explicit Director Audit.
