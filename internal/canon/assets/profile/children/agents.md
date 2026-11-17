### Child Repositories

- Apply these rules when `govna/children.txt` declares a child repository.
- Treat a declared child named in the active AC's `## In Scope` as primary scope instead of ancillary work.
- Exclude child files and child commit messages from every bare-identifier allowance in `### AC Mechanics`.
- Add each declared child as a writable session directory before editing it.
- Follow `govna/development-cycle.md` `## Child Repositories` for the session procedure.
- Change child membership only through `govna apply --child` and `govna rm --child`.
- Record a child's own rules in the parent's `## Project Rules` under a `###` heading named for the child.
- Build the parent against each child that the active AC changes through the stack's local override during Implement and Ratify validation.
- Treat a Package request as authorization for each child prep, local-override removal, and parent pin update.
- Release each changed child before the parent's pin update.
- Run each child's printed release command with `--yes` appended under `release` or `rel`.
- Stop before the parent's pin update when a child release fails.
- Fix a failure after a published child tag forward with a child patch release.
- Keep every published child tag unchanged.
- Record each child version change in the parent's `history.md` section.
- Follow `govna/build-release.md` `## Child Releases` for the two Package stages.
- Exempt each child release from the `### Agent Release` limit on other git writes.
