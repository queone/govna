# CODE Overlay

This overlay owns code-repo artifacts and rules only. Not rendered to consumers itself — maintainer-facing documentation of the template source.

Current concrete templates live under `files/`, stack-specific build tooling under `stacks/<name>/`.

Current contents (`files/`):

- `.gitignore`
- `arch.md`
- `CHANGELOG.md`
- `govna/ac-template.md`
- `govna/build-release.md`
- `govna/canon-cycle.md`
- `govna/code-stacks.md`
- `govna/development-cycle.md`
- `govna/development-guidelines.md`
- `govna/drift-scan.md`
- `govna/metadata.txt`
- `govna/operator-contract-rationale.md`
- `govna/README.md`
- `govna/roles.md`
- `plan.md`
- `README.md`

Stack overlays (`stacks/<name>/build.sh.tmpl`, first-class: Go, Rust, Swift, Terraform) carry the rest of the pipeline inline and require no external govna tools. Rust CODE builds validate independent utility declarations and compiled `--version` output. Swift CODE builds require Swift 6 and a root `Package.swift`. All target Bash 3.2+ so macOS system Bash is supported.

See `plan.md` for future overlay improvements.
