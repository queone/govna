# Canon Maintenance

This repository-owned document holds the procedures that Govna follows for a canon change. No consumer receives it.
`AGENTS.md` `### Canon Maintenance` holds the rules, and `govna/canon-spec.md` records what the tools do.

## govna-side commitments

1. **Registries.** Update format-defining or expected-divergence registries in the AC that adds the governed file.
2. **Breaking changes.** Ship extension-clobbering removals or shape changes as MINOR with migration cost in the CHANGELOG.
3. **Alerting.**
   - Surface updates through audit.
   - Hard-fail incoherent canon for maintainer correction.
4. **Prior-canon check.** Build the prior canon from the latest release tag in the upgrade test, so a canon version bump rotates no committed fixture.
5. **Stub rules.**
   - Keep rules out of the `plan.md` and `arch.md` stubs.
   - Place each such rule in a synced canon file.

## Candidate-canon review

Candidate canon is the embedded governance version under review. A consumer-equivalent review uses the rendered files, actual audit output, and emitted AC that an adopting repository receives. A state-equivalence fixture covers one distinct audit branch without repeating a profile-independent branch for every profile. A Govna-canon finding is a fault in shared governance rather than repository-owned behavior.

### Render and audit

- Run this review before completing a Govna canon change.
- Render DOC and every registered CODE stack.
- Render every CODE stack with and without the child profile.
- Exercise every rendered profile through ordinary non-JSON `govna audit` at least once.
- Resolve the Govna executable before each consumer-equivalent audit.
- Reuse that resolved executable for the Audit scratch render.
- Verify the emitted marker versions against the resolved executable.
- Verify the rendered baseline canon version against the emitted marker.
- Cover every audit classification across the state-equivalence fixtures.
- Cover each force-sync state across the state-equivalence fixtures.
- Cover every routing outcome across the state-equivalence fixtures.
- Cover every marker-only choice across the state-equivalence fixtures.
- Cover legacy-phrase cleanup across the state-equivalence fixtures.
- Cover retired-replacement ordering across the state-equivalence fixtures.
- Cover mixed-content protection across the state-equivalence fixtures.
- Cover inferred, not-applicable, and unresolved repository checks across the state-equivalence fixtures.
- Reuse a fixture only for a profile-independent branch.
- Exercise each flavor-specific branch in its owning profile.
- Exercise each stack-specific branch in its owning profile.

### Emitted AC review

- Complete one bounded scratch review for every actionable emitted AC.
- Compare every actionable path without JSON diff fields.
- Keep the emitted AC and consumer fixture byte-identical during review.
- Remove the exact scratch directory before completing review.
- Verify every offered routing outcome is executable.
- Verify every offered routing outcome has conditional acceptance coverage.
- Verify every emitted reference resolves in the selected render or named repository state.
- Verify the emitted instructions agree with the selected render.
- Verify phase entry and exit remain compatible.
- Verify completion output remains compatible across its governing documents.

### Behavior parity

- Map every provided command or executable artifact to its governing claims.
- Map every governing claim to a named behavioral regression.
- Verify every supported profile provides its required canonical command implementation.
- Block completion on any claim-to-behavior mismatch.

### Release-batch safety

- Exercise one-AC and fitting multi-AC pending batches.
- Exercise a direct batch before Package prep.
- Exercise an oversized projected batch before another Implement.
- Exercise oversized, partial, and partly unaccepted batches before Package prep.
- Verify whole-tree release staging cannot bypass complete-batch rules.
- Verify 80-byte acceptance with ASCII and multibyte messages.
- Reject 81-byte messages before release-prep mutation.

### Review evidence

- Keep the review record in the closure-audit working record.
- Record each profile, render, fixture scenario, audit result, and emitted AC.
- Record each reference, instruction, phase, route, acceptance, and completion check.
- Record behavior parity and finding disposition.
- Block Implement completion on any unresolved Govna-canon finding.
