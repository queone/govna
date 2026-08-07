## Rust Compilation Reuse

- Keep separate Clippy, test, and release-build validation unless a measured change preserves their coverage.
- Reopen optimization work when build duration becomes materially costly.
- Reopen optimization work when stable Cargo or Clippy behavior offers measurable artifact reuse.
- Reopen compiler-cache evaluation only with Director authorization.
- Record the toolchain version, exact commands, isolated target-directory conditions, repeated timings, and unchanged validation coverage in any renewed investigation.

Note: Cargo may compile overlapping dependency graphs separately because Clippy, tests, and release builds use different analysis, code-generation, and profile artifacts. Direct measurements did not demonstrate a reliable wall-clock improvement from command reordering alone.
