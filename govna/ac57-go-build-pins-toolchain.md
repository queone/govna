# AC57 Go build.sh builds with the Go toolchain that go.mod declares

## Summary

Every Go `build.sh` that Govna renders, and Govna's own, exports `GOTOOLCHAIN` from `go.mod` before its first Go
command, so builds, release prep, and releases run on the Go version the repository declares even when a newer local
Go is installed. Today the default `GOTOOLCHAIN=auto` lets a newer local Go win, and the pinned staticcheck can then
fail to read that Go's export data. That happened in funf on 2026-10-09: Homebrew moved Go to 1.27.2, `go.mod`
declared 1.26.4, and staticcheck 2026.2.1 failed on every standard-library import until the build ran under
`GOTOOLCHAIN=go1.26.4`. The pin makes the toolchain deterministic in the same way the staticcheck pin does.

Code impact: the Go stack template and the root `build.sh` gain `_pin_go_toolchain` and one build step line. The
code-stacks canon and the Go practices canon gain one rule each. The canon version becomes 0.72.0, with the baseline,
the render golden, and the growth baseline regenerated. Release prep bumps govna to 0.37.0, a MINOR bump for the
changed build output and behavior.

### Behavior

- `_pin_go_toolchain` reads `go.mod` in the working directory. With a `toolchain goX.Y.Z` directive it exports
  `GOTOOLCHAIN=goX.Y.Z`. Otherwise it exports `GOTOOLCHAIN=go` plus the `go` directive, appending `.0` to a two-part
  directive such as `go 1.27`. A prerelease directive such as `go 1.28rc1` passes through as `go1.28rc1`. With no
  `go.mod`, no `go` directive, or a `toolchain` directive that is not a Go version name, it leaves `GOTOOLCHAIN`
  untouched.
- `main` calls `_pin_go_toolchain` before dispatching to build, prep, or release, so every Go command in every path
  sees the pin, including the `go install` of staticcheck.
- The pin replaces any `GOTOOLCHAIN` from the environment or from `go env -w`, as the staticcheck pin ignores PATH.
- The full and scoped build print the step `==> Pin the Go toolchain from go.mod` and the line `GOTOOLCHAIN=go1.27.0`
  before `==> Update go.mod to reflect actual dependencies`. Prep and release print nothing for the pin.
- The first build after a host Go upgrade downloads the pinned toolchain once when it is not cached. Later builds run
  offline as before.

## In Scope

### Files to modify

- `internal/canon/assets/overlays/code/stacks/go/build.sh.tmpl` — `_pin_go_toolchain`, its call in `main`, and the
  build step line.
- `build.sh` — the same change, so the root and rendered scripts stay in parity.
- `internal/canon/assets/overlays/code/files/govna/code-stacks.md.tmpl` and `govna/code-stacks.md` — one Go rule after
  the staticcheck rule: `Pin GOTOOLCHAIN in build.sh to the go.mod toolchain directive, or else to its go directive,
  before the first Go command.`
- `internal/canon/assets/stack-guidelines/go.md` and its rendered copy in `govna/development-guidelines.md` — one
  bullet after the staticcheck bullet: `Build with the Go toolchain that go.mod declares; build.sh exports GOTOOLCHAIN
  from the toolchain or go directive.`
- `internal/canon/canon.go` `Version` and `cmd/govna/main.go` `canonVersion` — `0.72.0`.
- `govna/metadata.txt` and `govna/canon-baseline.txt` — `canon_version = v0.72.0` and the hashes of `build.sh`,
  `govna/code-stacks.md`, `govna/development-guidelines.md`, and `govna/metadata.txt`.
- `internal/canon/testdata/render-golden.txt` and `internal/canon/testdata/governance-growth-baseline.txt` — the
  hashes of the changed rendered files and the line and bullet counts of the changed templates.
- `internal/buildtest/build_test.go` — the fake `go` records `GOTOOLCHAIN` on every call, the no-network test asserts
  `go1.27.0` from its `go 1.27.0` fixture and the step line, a sourced-helper test covers the directive cases, and the
  helper-closure test lists `_pin_go_toolchain`.
- `internal/canon/canon_test.go` — the rendered Go build contains `_pin_go_toolchain()` and `export GOTOOLCHAIN=`.
- `cmd/govna/main_test.go`, `internal/apply/apply_test.go`, `internal/apply/testdata/existing-golden.md`,
  `internal/apply/testdata/fresh-code-golden.md`, `internal/apply/testdata/fresh-doc-golden.md`,
  `internal/audit/audit_test.go`, `internal/audit/testdata/actionable-golden.json`,
  `internal/audit/testdata/actionable-golden.md`, `internal/audit/testdata/unresolved-validation-golden.md`,
  `internal/remove/remove_test.go`, and `internal/remove/testdata/removal-golden.md` — the marker version `v0.71.0`
  becomes `v0.72.0`.

### Schema changes

None.

## Out Of Scope

- Honoring an explicit `GOTOOLCHAIN` from the environment. The pin is unconditional, like the staticcheck pin.
- Changing the staticcheck pin. v0.8.1 stays. The go-tools fix for Go 1.27.2, pull request 1834, arrives in a later
  release and gets its own bump.
- The Rust, Swift, Terraform, and DOC build scripts.
- Adding a `toolchain` directive to any `go.mod`.
- `arch.md` and `README.md`. Their one-line description of `build.sh` still holds.
- Consumer adoption. Each consumer picks up the change through its next `govna audit` adoption AC.

## Migration findings

None.

## Acceptance Tests

**AT1** [Automated] [Pre-release gate] — The rendered Go build run in the no-network fixture with `go 1.27.0` runs
every traced `go` call and the staticcheck call with `GOTOOLCHAIN=go1.27.0`, and its output holds `==> Pin the Go
toolchain from go.mod` and `GOTOOLCHAIN=go1.27.0` before `Update go.mod`.

**AT2** [Automated] [Pre-release gate] — Sourced from the rendered script, `_pin_go_toolchain` exports `go1.27.1` for
`go 1.27.0` with `toolchain go1.27.1`, `go1.27.0` for `go 1.27`, `go1.28rc1` for `go 1.28rc1`, and leaves a preset
`GOTOOLCHAIN=local` untouched for a `go.mod` without a `go` directive and for a missing `go.mod`.

**AT3** [Automated] [Pre-release gate] — The helper-closure test counts one `_pin_go_toolchain` definition, and a
test that sources the rendered script beside a `go 1.27.0` fixture, runs `main -h`, and reads `GOTOOLCHAIN` gets
`go1.27.0`, which shows the pin precedes every dispatch.

**AT4** [Automated] [Pre-release gate] — The root and rendered Go scripts both contain `_pin_go_toolchain()` and the
root-parity test passes, and the rendered Go build contains `export GOTOOLCHAIN=` in the stack-boundaries test.

**AT5** [Automated] [Pre-release gate] — The code-stacks rule appears exactly once in `govna/code-stacks.md` and in
its template, and the Go-practices bullet appears exactly once in `stack-guidelines/go.md` and in the rendered
`govna/development-guidelines.md`.

**AT6** [Automated] [Pre-release gate] — `./build.sh` passes with canon version 0.72.0 in `canon.go`, `main.go`,
`metadata.txt`, and `canon-baseline.txt`, with the render golden and the growth baseline matching the new canon, and
with every marker-version golden updated.

**AT7** [Manual] [Pre-release gate] — Render a scratch Go consumer with the built govna, set its `go.mod` to
`go 1.26.4`, and run its `./build.sh` on a host with Go 1.27.2 and no `GOTOOLCHAIN` prefix. The build prints
`GOTOOLCHAIN=go1.26.4`, staticcheck passes, and the scratch directory is removed afterwards.

**AT8** [Manual] [Post-release verification] — After funf adopts canon v0.72.0, `./build.sh` in funf passes without an
environment prefix on the same host.

## Status

`DEFERRED` — the Director deferred Implement on 2026-10-09 until a staticcheck release reads Go 1.27.2 export
data. Tracking: dominikh/go-tools issue 1832 and pull request 1834. Resume with the Director's implementation-ready
confirmation.
