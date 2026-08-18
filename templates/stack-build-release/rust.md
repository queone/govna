## Rust Compilation Reuse

- Keep separate Clippy, test, and release-build validation unless a measured change preserves their coverage.
- Reopen optimization work when build duration becomes materially costly.
- Reopen optimization work when stable Cargo or Clippy behavior offers measurable artifact reuse.
- Reopen compiler-cache evaluation only with Director authorization.
- Record the toolchain version, exact commands, isolated target-directory conditions, repeated timings, and unchanged validation coverage in any renewed investigation.
- Compare release-prep changes in disposable repository copies with isolated Cargo and installation roots.
- Require lower median warm duration in every representative Rust consumer.
- Require at least 25 percent lower aggregate warm duration before adopting a release-prep optimization.

Note: Cargo may compile overlapping dependency graphs separately because Clippy, tests, and release builds use different analysis, code-generation, and profile artifacts. Direct measurements did not demonstrate a reliable wall-clock improvement from command reordering alone.

### Rust Release Prep

- Run `GOVNA_PREP_VALIDATION_TOKEN='<token>' ./build.sh prep vX.Y.Z "message"` during Package.
- Use the token printed by the successful final full build reviewed during Ratify.
- Run `./build.sh refresh-validation-token -b <scratch-baseline> -t '<token>'` after exact baseline-only audit-adoption completion.
- Use the refreshed token as Package evidence.
- Omit the token to require a fallback pre-change full build.
- Require prep to recompute HEAD and the Git-visible-state fingerprint before writes.
- Run one post-change full build after prep writes.
- Reuse one prep-owned isolated Cargo target across fallback validation, Cargo.lock refresh, and post-change validation.
- Keep prep responsible for shared-target cleanup.
- Use `-v, --verbose` to stream complete prep phase output.
- Replay captured phase output on failure in default mode.
- Reject `--no-build, -B` outside Rust dry-run prep.
