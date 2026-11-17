## Child Releases

- Apply this section when `govna/children.txt` declares a child.
- Treat a child as changed when its working tree or commits differ from its latest tag, or when it has no tag.
- Run stage 1 when any declared child is changed.
- Choose each changed child's version by semver from that child's own changes.
- Write each child release message in plain words without AC identifiers.
- Run each changed child's prep in that child.
- End plain Package stage 1 by presenting each child's release command.
- Start the stage 1 report with the plain line `Package paused for child releases.`.
- Continue to stage 2 after the child releases under `release` or `rel`.
- Run stage 2 when no declared child is changed.
- Remove each local override that resolves to a declared child.
- Move each child pin to that child's latest tag with the pin command at the end of this section.
- Run the full build only after every override is removed and every pin is moved.
- Write the `history.md` section after that full build passes.
- Run the parent's prep after the `history.md` section.
- List each child release before the parent's release in the Package completion report.
- List every `go.mod` change beyond the child pin lines in a Go parent's Package completion report.
- State the fix-forward recovery in every report of a failure after a published child tag.
{{CHILD_PIN_COMMANDS}}