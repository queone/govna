# AC7 Drop Child Repository ../engine

## Summary

Govna executable v9.8.7 created this plan from its embedded governance files (canon v0.86.0). This AC drops the child repository `../engine` from this parent: it rewrites the child's `AGENTS.md` to the orphan note and removes the child from `govna/children.txt`. Every other file stays unchanged.

### Removal Instructions

- Create the orphan note with `govna render --orphan <orphan-scratch>`.
- Copy `<orphan-scratch>/AGENTS.md` to `../engine/AGENTS.md`.
- Remove the `../engine` line from `govna/children.txt`.

### Routing Decisions

`None` — no review items.

## In Scope

- `../engine/AGENTS.md` — rewrite to the orphan note; the child repository loses its parent.
- `govna/children.txt` — remove entry `../engine`; the parent stops governing the child.

## Out Of Scope

- Every other parent and child file.

## Migration findings

- None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — Verify `../engine/AGENTS.md` matches `govna render --orphan`.

**AT2** [Automated] [Pre-release gate] — Verify `govna/children.txt` no longer lists `../engine`.

## Status

`PENDING` — removal emission; awaiting explicit Director Audit.
