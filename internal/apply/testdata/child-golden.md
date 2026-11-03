# AC1 Review Child Repository ../engine

## Summary

Govna executable v9.8.7 added its embedded child files (canon v0.87.0) for the Go child repository `../engine` and listed it in `govna/children.txt`. The list below records whether each file was written, replaced, or kept.

## In Scope

Files Govna processed:

- `../engine/.gitignore` (written)
- `../engine/AGENTS.md` (written)
- `../engine/CHANGELOG.md` (written)
- `../engine/build.sh` (written)
- `govna/children.txt` (lists `../engine`)

## Out Of Scope

- Files not listed above.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify every written child file matches `govna render --child` for the Go stack.

**AT2** [Automated] [Pre-release gate] — Verify `./build.sh` runs the build of `../engine` before its own steps.

## Status

`PENDING` — apply emission; awaiting explicit Director Audit.
