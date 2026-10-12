## Go Practices

- Add single-line godoc comments to exported functions in shared Go packages.
- Declare a non-empty `const programVersion` string literal in every installable `cmd/<name>/main.go`.
- Validate every `programVersion` declaration through `build.sh` before compiling installable binaries.
- Pin `staticcheck` to the repository-governed version and invoke the pinned installation path directly.
- Bump the `go.mod` `go` directive and the `staticcheck` pin together, after the `staticcheck` release notes name the new Go version.
- Treat `go vet` and `staticcheck` findings as build failures.
- Build with the Go toolchain that `go.mod` declares.
- Scan all `.go` and `.go.tmpl` files for stale import paths after a module rename.
