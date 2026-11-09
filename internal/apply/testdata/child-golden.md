# AC1 Review Child Repository ../engine

## Summary

Govna executable v9.8.7 added its embedded child files (canon v0.92.0) for the Go child repository `../engine` and listed it in `govna/children.txt`. The list below records whether each file was written, replaced, or kept. The parent's full `./build.sh` builds `../engine` first and fails until the child's own `./build.sh` passes.

## In Scope

Files Govna processed:

- `../engine/.gitignore` (written)
- `../engine/AGENTS.md` (written)
- `../engine/CHANGELOG.md` (written)
- `../engine/build.sh` (written)
- `govna/children.txt` (lists `../engine`)

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

## Status

`PENDING` — apply emission; awaiting explicit Director Audit.
