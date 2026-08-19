# Development Cycle

This repo uses an acceptance-criteria-first workflow.

## AC Workflow

- Follow the lifecycle `Draft → Audit → Refine → Implement → Ratify → Package`.
- Treat standalone `Draft` or `draft` as the Director-authorized pre-cycle action that creates the active AC; Draft is not an AC phase.
- Treat standalone `Audit` or `audit` as the adversarial-review phase action that starts the active AC cycle.
- Treat standalone `Refine` or `refine` as the scope-and-decision-resolution phase action.
- Treat standalone `Implement` or `implement` as the implementation-and-verification phase action.
- Treat standalone `Ratify` or `ratify` as the Director acceptance action that initiates the final review and completes Ratify when that review is clean.
- Treat standalone `Package`, `package`, `pack`, and `prep` as equivalent post-Ratify release-preparation actions; do not infer Package from Ratify acceptance.
- Start a cycle only when the director identifies the active AC and explicitly
  requests Audit.
- Use an unnumbered phase instruction when one AC is under `govna/`; require
  the AC number when multiple ACs are present.
- Pause after each lifecycle action until the director explicitly advances the active AC.

## Required Artifacts

- `AGENTS.md`
- `README.md`
- `arch.md`
- `plan.md`
- `govna/`

## Cycle

1. **Draft.** Create the authorized AC from `govna/ac-template.md`.
2. **Audit.** Challenge the contract without mutation.
3. **Refine.** Resolve findings and Director decisions in the AC.
4. **Implement.** Deliver, test, verify, correct, and closure-audit the settled scope.
5. **Ratify.** Perform the Director-triggered final review and bounded correction behavior.
6. **Package.** Run `govna/build-release.md` release preparation only after separate Director authorization.

Apply the complete phase, scope, correction, contract-integrity, and advancement rules in `AGENTS.md` throughout this cycle.

## Notes

- Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`, architecture in `arch.md`, and repo governance in `AGENTS.md`.
- Remove an IE when rejected, retired, or shipped through its AC pointer.
- Keep ACs in `govna/ac<N>-<slug>.md` and summarize rather than reproduce them in chat.
- Mark an unscoped stub in `## Summary`, keep scope and tests TBD, and leave it `PENDING` until scoped.
