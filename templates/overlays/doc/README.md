# DOC Overlay

Governance + planning + release tooling for documentation repos. Editorial structure (voice guides, style guides, publishing workflows) is the repo owner's domain. Not rendered to consumers itself — maintainer-facing documentation of the template source.

Current concrete templates live under `files/`.

Current contents:

- `.gitignore`
- `AGENTS.md`
- `build.sh`
- `CHANGELOG.md`
- `README.md`
- `govna/ac-template.md`
- `govna/canon-cycle.md`
- `govna/drift-scan.md`
- `govna/editing-cycle.md`
- `govna/editing-guidelines.md`
- `govna/metadata.txt`
- `govna/operator-contract-rationale.md`
- `govna/README.md`
- `govna/release.md`
- `govna/roles.md`
- `plan.md`

`build.sh` is a self-contained Bash 3.2+ script for release preparation and
annotated-tag release orchestration. Generated DOC repos require no compiler
toolchain for those workflows and define no automated content validation.
