# Behavioral Parity Contract

## Authority

Use `queone/govna-rust` tag `v0.37.1` at commit `7416cb919b48284f2db45adc99875bddfdb87564` as the frozen behavioral reference. Treat `govna/parity-index.txt` as the deterministic source-test snapshot and this document as the authoritative disposition matrix.

For S1 byte-exact output, substitute the Go successor's current `programVersion`, reported embedded-canon version, and source repository `github.com/queone/govna` for the corresponding Rust identity values. Preserve every other byte, stream, trailing newline, and exit status.

## Dispositions

- Use `byte-exact` when bytes, stream, and trailing newline are contractual.
- Use `semantic` when observable meaning and effects must match while implementation mechanics may differ.
- Use `intentional-difference` only with an approved reason and replacement behavior.
- Use `implementation-specific` only when the Rust mechanism is not a Go product contract.
- Use `not-applicable` only when the Rust surface does not exist in the Go successor.
- Record a reason for every `intentional-difference`, `implementation-specific`, or `not-applicable` row.

## Behavioral Inventory

### Command dispatch and version

- Preserve no-argument usage failure on stderr with exit status 2.
- Preserve top-level help aliases `-h`, `--help`, `-?`, `help`, and `h` on stdout with success.
- Preserve hidden legacy aliases `render-canon` and `drift-scan`.
- Preserve unknown-command diagnostics on stderr with exit status 2.
- Preserve compact aliases `--version`, `ver`, and `v` as one identical stdout line.
- Preserve `version` as two stdout lines naming the binary and embedded canon versions.
- Reject arguments to `version` on stderr with exit status 2.

### Render and embedded canon

- Preserve `render [-f|--flavor code|doc] [-s|--stack <name>] [-m|--module-path <path>] <target>`.
- Preserve cwd-based flavor, stack, repository-name, and Go module-path inference.
- Preserve case-insensitive stack overrides and reject stack options for DOC consumers.
- Restrict module-path resolution to Go CODE consumers.
- Preserve deterministic substitution, emitted file sets, stack overlays, flavor overlays, source-only exclusions, baseline generation, and `CLAUDE.md` symlink creation.
- Preserve mixed-content boundaries at `## Project Rules` and applicable `## Project Practices` headings.
- Keep the embedded canon version independent from the binary version.
- Treat Rust-only embedding mechanics as non-contractual while preserving rendered bytes where a row requires them.

### Apply

- Preserve `apply [-f|--flavor code|doc] [-s|--stack <name>] [-n|--repo-name <name>] [-m|--module-path <path>] [-g|--init-git]` against the current directory.
- Preserve target assessment, flavor and stack inference, new and existing modes, overwrite labels, preserved repo-owned files, mixed-content hunk merging, and deterministic adoption-AC emission.
- Preserve idempotent AC reuse and edited-stub refusal.
- Preserve source-checkout refusal before target traversal.
- Preserve governa migration detection, precise and fallback classification, merged migration emission, and self-termination.
- Intentionally require `apply -g` to initialize `main` and prohibit creation or publication of `master`.

### Audit

- Preserve `audit [-f|--flavor code|doc] [-s|--stack <name>] [-j|--json] [-l|--diff-lines <N>] [-n|--repo-name <name>]` against the current directory.
- Require an adopted repository, readable `AGENTS.md`, a Git worktree, valid metadata, valid baseline, and valid preserve registry before emission.
- Preserve all-match, expected-divergence, consumer-edited, canon-changed, ambiguous, target-only, cross-flavor orphan, retired-path, and migration classifications.
- Preserve freshness gates, validation-evidence inference, stale-metadata routing, mixed-content region handling, protected-region digests, and complete-snapshot reconciliation.
- Preserve deterministic Markdown emission, optional JSON shape, idempotent reuse, edited-stub refusal, and source-checkout refusal.

### Removal

- Preserve `rm [-f|--flavor code|doc] [-s|--stack <name>] [-n|--repo-name <name>]` against the current directory.
- Require adoption and a Git worktree.
- Preserve pure-canon delete routing, hybrid and ambiguous review routing, preserve-registry keep routing, target-only keep routing, flavor overrides, deterministic emission, idempotent reuse, edited-stub refusal, and source-checkout refusal.
- Preserve non-destructive command behavior; emit a reviewable removal AC without deleting consumer files.

### Product tooling

- Preserve the Go product repository's build, validation, version-target, installation, release-prep, and release-orchestration outcomes separately from build scripts rendered into consumers.
- Preserve version declaration validation, compiled-version output, target discovery, isolated build ownership, install reporting, validation-token freshness, baseline-token refresh, prep evidence routing, pre-mutation failure, output modes, cleanup, and release mutation behavior.
- Defer Go-specific commands and mechanics to S6 while retaining each observable outcome.

## Command Surface Matrix

| Surface record | Entry point | Flags and aliases | Streams and exit statuses | Filesystem mutations and emitted artifacts |
| --- | --- | --- | --- | --- |
| IF-001 | No command | None | Write usage to stderr and exit 2. | Make no filesystem mutation. |
| IF-002 | Top-level help | `-h`, `--help`, `-?`, `help`, `h` | Write byte-exact usage to stdout and exit 0. | Make no filesystem mutation. |
| IF-003 | Unknown command | Any unrecognized subcommand | Write the diagnostic and usage to stderr and exit 2. | Make no filesystem mutation. |
| IF-004 | Compact version | `--version`, `ver`, `v` | Write one byte-exact version line to stdout and exit 0. | Make no filesystem mutation. |
| IF-005 | Detailed version | `version`; reject additional arguments | Write two byte-exact version lines to stdout and exit 0; write an argument error to stderr and exit 2. | Make no filesystem mutation. |
| IF-006 | `render`; legacy alias `render-canon` | `-f`, `--flavor`, `-s`, `--stack`, `-m`, `--module-path`, `-h`, `--help`, `-?`; require one target | Write help to stderr and exit 0; write argument errors to stderr and exit 2; write runtime errors to stderr and exit 1; write the target path to stdout and exit 0 on success. | Write the deterministic flavor and stack canon set, `govna/canon-baseline.txt`, and the `CLAUDE.md` symlink under the target; preserve unrelated target content and do not pre-clean. |
| IF-007 | `apply` | `-f`, `--flavor`, `-s`, `--stack`, `-n`, `--repo-name`, `-m`, `--module-path`, `-g`, `--init-git`, `-h`, `--help`, `-?`; reject positional arguments | Write help to stderr and exit 0; write argument errors to stderr and exit 2; write runtime errors and warnings to stderr and exit 1 on failure; write assessment and write/skip results to stdout and exit 0 on success. | Write or merge the canon set and baseline, create or preserve `CLAUDE.md` as specified, emit one adoption AC, and initialize Git on `main` only with `-g`; never delete the legacy `governa/` tree. |
| IF-008 | `audit`; legacy alias `drift-scan` | `-f`, `--flavor`, `-s`, `--stack`, `-j`, `--json`, `-l`, `--diff-lines`, `-n`, `--repo-name`, `-h`, `--help`, `-?`; reject positional arguments | Write help to stderr and exit 0; write argument errors to stderr and exit 2; write runtime or prerequisite errors to stderr and exit 1; write the summary or JSON to stdout and exit 0 on success. | Emit one deterministic audit AC only for actionable results; reuse an unedited matching stub; emit no AC for a clean result; mutate no pre-existing governed consumer artifact other than an eligible emitted-AC stub. |
| IF-009 | `rm` | `-f`, `--flavor`, `-s`, `--stack`, `-n`, `--repo-name`, `-h`, `--help`, `-?`; reject positional arguments | Write help to stderr and exit 0; write argument errors to stderr and exit 2; write runtime or prerequisite errors to stderr and exit 1; write the emitted-AC path to stdout and exit 0 on success. | Emit or reuse one deterministic removal AC and mutate no pre-existing governed consumer artifact other than an eligible emitted-AC stub; never perform a routed deletion itself. |

## Implementation Boundary Matrix

| Boundary record | Surface | Required outcome | Excluded mechanism |
| --- | --- | --- | --- |
| BND-001 | Go product tooling | Preserve validation ordering, version ownership, target discovery, isolated temporary-output ownership, installation reporting, validation evidence, release-prep mutation ordering, failure-before-mutation, and cleanup outcomes. | Defer Go commands, package layout, dependency choices, build-cache layout, and tool selection to S6. |
| BND-002 | Emitted Rust consumer tooling | Preserve the rendered Rust build and validation-token contracts semantically because they remain externally observable consumer output. | Exclude the Go product repository's own build mechanics from this surface. |
| BND-003 | Frozen Rust product mechanics | Retain source traceability for Cargo manifest parsing, shared Cargo target ownership, and Cargo target cleanup. | Classify those mechanics as `implementation-specific`; require Go-native replacements only for the outcomes in BND-001. |
| BND-004 | Implementation architecture | Preserve only behavior and single-stage ownership. | Defer packages, dependencies, CLI libraries, embedding strategy, data structures, concurrency, and internal control flow to owning implementation ACs. |
| BND-005 | Intentional difference | Initialize `apply -g` repositories on `main` and prohibit creation or publication of `master`. | Permit no other intentional difference without Director approval and a new reasoned requirement. |

## Future Stages

- **S1:** Implement the Go module, command dispatch, usage, help, and version surfaces.
- **S2:** Implement embedded canon, rendering, consumer build-script emission, and deterministic output.
- **S3:** Implement apply, adoption, mixed-content behavior, Git initialization, and governa migration.
- **S4:** Implement audit, baseline, preserve-registry, classification, and emitted audit AC behavior.
- **S5:** Implement removal classification and emitted removal AC behavior.
- **S6:** Implement product validation, installation, release preparation, release orchestration, and final cross-surface parity verification.

Keep stage boundaries behavioral. Defer package layout, dependencies, libraries, embedding strategy, data structures, concurrency, and other architecture choices to each owning implementation AC.

## Traceability Matrix

| Requirement | Surface | Rust reference | Observable contract | Disposition | Primary stage | Verification source or reason |
| --- | --- | --- | --- | --- | --- | --- |
| VER-001 | Version | integration:version_aliases_are_all_single_line_and_identical | Version aliases are all single line and identical. | byte-exact | S1 | tests/govna_cli.rs::version_aliases_are_all_single_line_and_identical |
| VER-002 | Version | integration:detailed_version_reports_binary_and_embedded_canon | Detailed version reports binary and embedded canon. | byte-exact | S1 | tests/govna_cli.rs::detailed_version_reports_binary_and_embedded_canon |
| VER-003 | Version | integration:detailed_version_rejects_arguments | Detailed version rejects arguments. | byte-exact | S1 | tests/govna_cli.rs::detailed_version_rejects_arguments |
| CLI-001 | Command dispatch | integration:no_args_exits_with_usage_error | No args exits with usage error. | byte-exact | S1 | tests/govna_cli.rs::no_args_exits_with_usage_error |
| CLI-002 | Command dispatch | integration:top_level_help_aliases_use_stdout | Top level help aliases use stdout. | byte-exact | S1 | tests/govna_cli.rs::top_level_help_aliases_use_stdout |
| CLI-003 | Command dispatch | integration:legacy_command_aliases_remain_functional_but_hidden | Legacy command aliases remain functional but hidden. | semantic | S1 | tests/govna_cli.rs::legacy_command_aliases_remain_functional_but_hidden |
| CLI-004 | Command dispatch | integration:unrecognized_subcommand_exits_two | Unrecognized subcommand exits two. | semantic | S1 | tests/govna_cli.rs::unrecognized_subcommand_exits_two |
| RND-001 | Render and canon | integration:render_doc_flavor_metadata | Render doc flavor metadata. | semantic | S2 | tests/govna_cli.rs::render_doc_flavor_metadata |
| RND-002 | Render and canon | integration:render_baselines_are_valid_flavor_specific_and_deterministic | Render baselines are valid flavor specific and deterministic. | semantic | S2 | tests/govna_cli.rs::render_baselines_are_valid_flavor_specific_and_deterministic |
| RND-003 | Render and canon | integration:render_infers_rust_and_accepts_case_insensitive_override | Render infers rust and accepts case insensitive override. | semantic | S2 | tests/govna_cli.rs::render_infers_rust_and_accepts_case_insensitive_override |
| RND-004 | Render and canon | integration:render_infers_swift_and_accepts_case_insensitive_override | Render infers swift and accepts case insensitive override. | semantic | S2 | tests/govna_cli.rs::render_infers_swift_and_accepts_case_insensitive_override |
| RND-005 | Render and canon | integration:render_doc_rejects_stack | Render doc rejects stack. | semantic | S2 | tests/govna_cli.rs::render_doc_rejects_stack |
| RND-006 | Render and canon | integration:render_module_path_rejected_outside_go_code | Render module path rejected outside go code. | semantic | S2 | tests/govna_cli.rs::render_module_path_rejected_outside_go_code |
| RND-007 | Render and canon | integration:render_go_module_path_and_override | Render go module path and override. | semantic | S2 | tests/govna_cli.rs::render_go_module_path_and_override |
| RND-008 | Render and canon | integration:render_stitches_gitignore_and_guidelines | Render stitches gitignore and guidelines. | semantic | S2 | tests/govna_cli.rs::render_stitches_gitignore_and_guidelines |
| RND-009 | Render and canon | integration:render_help_documents_flags | Render help documents flags. | semantic | S2 | tests/govna_cli.rs::render_help_documents_flags |
| RND-010 | Render and canon | integration:render_output_is_fully_substituted | Render output is fully substituted. | semantic | S2 | tests/govna_cli.rs::render_output_is_fully_substituted |
| RND-011 | Render and canon | integration:render_doc_agents_overrides_base | Render doc agents overrides base. | semantic | S2 | tests/govna_cli.rs::render_doc_agents_overrides_base |
| RND-012 | Render and canon | integration:rendered_agents_define_active_ac_exceptions | Rendered agents define active ac exceptions. | semantic | S2 | tests/govna_cli.rs::rendered_agents_define_active_ac_exceptions |
| RND-013 | Render and canon | integration:rendered_contracts_define_concise_reporting_and_ceremony_triage | Rendered contracts define concise reporting and ceremony triage. | semantic | S2 | tests/govna_cli.rs::rendered_contracts_define_concise_reporting_and_ceremony_triage |
| RND-014 | Render and canon | integration:rendered_contracts_define_contract_growth_integrity | Rendered contracts define contract growth integrity. | semantic | S2 | tests/govna_cli.rs::rendered_contracts_define_contract_growth_integrity |
| RND-015 | Render and canon | integration:documentation_avoids_unnecessary_coding_agent_names | Documentation avoids unnecessary coding agent names. | semantic | S2 | tests/govna_cli.rs::documentation_avoids_unnecessary_coding_agent_names |
| RND-016 | Render and canon | integration:rendered_contract_bundle_preserves_authority | Rendered contract bundle preserves authority. | semantic | S2 | tests/govna_cli.rs::rendered_contract_bundle_preserves_authority |
| RND-017 | Render and canon | integration:rendered_contract_defines_bounded_completeness_scenarios | Rendered contract defines bounded completeness scenarios. | semantic | S2 | tests/govna_cli.rs::rendered_contract_defines_bounded_completeness_scenarios |
| RND-018 | Render and canon | integration:render_creates_claude_symlink | Render creates claude symlink. | semantic | S2 | tests/govna_cli.rs::render_creates_claude_symlink |
| RND-019 | Render and canon | integration:root_docs_have_no_stale_governa_tokens | Root docs have no stale governa tokens. | semantic | S2 | tests/govna_cli.rs::root_docs_have_no_stale_governa_tokens |
| RND-020 | Render and canon | integration:root_has_no_self_referential_metadata | Root has no self referential metadata. | semantic | S2 | tests/govna_cli.rs::root_has_no_self_referential_metadata |
| AUD-001 | Audit | integration:audit_refuses_govna_source | Audit refuses govna source. | semantic | S4 | tests/govna_cli.rs::audit_refuses_govna_source |
| AUD-002 | Audit | integration:audit_requires_agents_md | Audit requires agents md. | semantic | S4 | tests/govna_cli.rs::audit_requires_agents_md |
| AUD-003 | Audit | integration:audit_rejects_unreadable_agents_path_before_emission | Audit rejects unreadable agents path before emission. | semantic | S4 | tests/govna_cli.rs::audit_rejects_unreadable_agents_path_before_emission |
| AUD-004 | Audit | integration:audit_requires_git_worktree | Audit requires git worktree. | semantic | S4 | tests/govna_cli.rs::audit_requires_git_worktree |
| AUD-005 | Audit | integration:audit_fresh_fixture_all_match | Audit fresh fixture all match. | semantic | S4 | tests/govna_cli.rs::audit_fresh_fixture_all_match |
| AUD-006 | Audit | integration:audit_expected_divergence_only_does_not_emit | Audit expected divergence only does not emit. | semantic | S4 | tests/govna_cli.rs::audit_expected_divergence_only_does_not_emit |
| AUD-007 | Audit | integration:audit_baseline_distinguishes_untouched_prior_canon_from_consumer_edit | Audit baseline distinguishes untouched prior canon from consumer edit. | semantic | S4 | tests/govna_cli.rs::audit_baseline_distinguishes_untouched_prior_canon_from_consumer_edit |
| AUD-008 | Audit | integration:audit_accepts_only_eligible_legacy_build_release_full_scope | Audit accepts only eligible legacy build release full scope. | semantic | S4 | tests/govna_cli.rs::audit_accepts_only_eligible_legacy_build_release_full_scope |
| AUD-009 | Audit | integration:audit_missing_baseline_and_entry_route_without_silent_trust | Audit missing baseline and entry route without silent trust. | semantic | S4 | tests/govna_cli.rs::audit_missing_baseline_and_entry_route_without_silent_trust |
| AUD-010 | Audit | integration:audit_doc_baseline_migration_infers_evidenced_no_validation | Audit doc baseline migration infers evidenced no validation. | semantic | S4 | tests/govna_cli.rs::audit_doc_baseline_migration_infers_evidenced_no_validation |
| AUD-011 | Audit | integration:audit_code_validation_evidence_failures_remain_unresolved | Audit code validation evidence failures remain unresolved. | semantic | S4 | tests/govna_cli.rs::audit_code_validation_evidence_failures_remain_unresolved |
| AUD-012 | Audit | integration:audit_doc_validation_evidence_failures_remain_unresolved | Audit doc validation evidence failures remain unresolved. | semantic | S4 | tests/govna_cli.rs::audit_doc_validation_evidence_failures_remain_unresolved |
| AUD-013 | Audit | integration:audit_validation_ignores_unrecognized_evidence_loci | Audit validation ignores unrecognized evidence loci. | semantic | S4 | tests/govna_cli.rs::audit_validation_ignores_unrecognized_evidence_loci |
| AUD-014 | Audit | integration:audit_routes_prebaseline_retired_path_without_mutation | Audit routes prebaseline retired path without mutation. | semantic | S4 | tests/govna_cli.rs::audit_routes_prebaseline_retired_path_without_mutation |
| AUD-015 | Audit | integration:audit_routes_prior_baseline_retirement_and_ignores_unrelated_local_doc | Audit routes prior baseline retirement and ignores unrelated local doc. | semantic | S4 | tests/govna_cli.rs::audit_routes_prior_baseline_retirement_and_ignores_unrelated_local_doc |
| AUD-016 | Audit | integration:audit_merges_retired_path_evidence_and_retains_tombstone_replacement | Audit merges retired path evidence and retains tombstone replacement. | semantic | S4 | tests/govna_cli.rs::audit_merges_retired_path_evidence_and_retains_tombstone_replacement |
| AUD-017 | Audit | integration:audit_requires_replacement_before_recommending_retired_path_deletion | Audit requires replacement before recommending retired path deletion. | semantic | S4 | tests/govna_cli.rs::audit_requires_replacement_before_recommending_retired_path_deletion |
| AUD-018 | Audit | integration:audit_rejects_malformed_baseline_before_emission | Audit rejects malformed baseline before emission. | semantic | S4 | tests/govna_cli.rs::audit_rejects_malformed_baseline_before_emission |
| AUD-019 | Audit | integration:audit_stale_metadata_version_forces_clear_sync | Audit stale metadata version forces clear sync. | semantic | S4 | tests/govna_cli.rs::audit_stale_metadata_version_forces_clear_sync |
| AUD-020 | Audit | integration:audit_stale_metadata_version_overrides_preserve_registry | Audit stale metadata version overrides preserve registry. | semantic | S4 | tests/govna_cli.rs::audit_stale_metadata_version_overrides_preserve_registry |
| AUD-021 | Audit | integration:audit_stale_metadata_legacy_phrase_emits_explicit_removal_decision | Audit stale metadata legacy phrase emits explicit removal decision. | semantic | S4 | tests/govna_cli.rs::audit_stale_metadata_legacy_phrase_emits_explicit_removal_decision |
| AUD-022 | Audit | integration:audit_stale_metadata_with_other_difference_routes_to_review | Audit stale metadata with other difference routes to review. | semantic | S4 | tests/govna_cli.rs::audit_stale_metadata_with_other_difference_routes_to_review |
| AUD-023 | Audit | integration:audit_rejects_non_adoptable_metadata_versions_before_emission | Audit rejects non adoptable metadata versions before emission. | semantic | S4 | tests/govna_cli.rs::audit_rejects_non_adoptable_metadata_versions_before_emission |
| AUD-024 | Audit | integration:audit_ambiguity_routes_to_review | Audit ambiguity routes to review. | semantic | S4 | tests/govna_cli.rs::audit_ambiguity_routes_to_review |
| AUD-025 | Audit | integration:audit_emitted_ac_instruction_and_phase_shape_is_deterministic | Audit emitted ac instruction and phase shape is deterministic. | semantic | S4 | tests/govna_cli.rs::audit_emitted_ac_instruction_and_phase_shape_is_deterministic |
| AUD-026 | Audit | integration:audit_format_defining_forces_sync | Audit format defining forces sync. | semantic | S4 | tests/govna_cli.rs::audit_format_defining_forces_sync |
| AUD-027 | Audit | integration:audit_preserve_registry_is_non_actionable | Audit preserve registry is non actionable. | semantic | S4 | tests/govna_cli.rs::audit_preserve_registry_is_non_actionable |
| AUD-028 | Audit | integration:audit_accepts_empty_registry_and_suppresses_registered_missing_target | Audit accepts empty registry and suppresses registered missing target. | semantic | S4 | tests/govna_cli.rs::audit_accepts_empty_registry_and_suppresses_registered_missing_target |
| AUD-029 | Audit | integration:audit_and_rm_reject_malformed_preserve_registry_before_emission | Audit and rm reject malformed preserve registry before emission. | semantic | S4 | tests/govna_cli.rs::audit_and_rm_reject_malformed_preserve_registry_before_emission |
| AUD-030 | Audit | integration:audit_ignores_legacy_phrases_outside_unreleased_summary | Audit ignores legacy phrases outside unreleased summary. | semantic | S4 | tests/govna_cli.rs::audit_ignores_legacy_phrases_outside_unreleased_summary |
| AUD-031 | Audit | integration:audit_mixed_content_below_boundary_matches | Audit mixed content below boundary matches. | semantic | S4 | tests/govna_cli.rs::audit_mixed_content_below_boundary_matches |
| AUD-032 | Audit | integration:audit_emits_protected_region_digest_for_direct_mixed_syncs | Audit emits protected region digest for direct mixed syncs. | semantic | S4 | tests/govna_cli.rs::audit_emits_protected_region_digest_for_direct_mixed_syncs |
| AUD-033 | Audit | integration:audit_emits_conditional_protected_region_digest_for_reviewed_mixed_syncs | Audit emits conditional protected region digest for reviewed mixed syncs. | semantic | S4 | tests/govna_cli.rs::audit_emits_conditional_protected_region_digest_for_reviewed_mixed_syncs |
| AUD-034 | Audit | integration:audit_boundaryless_build_release_requires_review_even_with_preserve_marker | Audit boundaryless build release requires review even with preserve marker. | semantic | S4 | tests/govna_cli.rs::audit_boundaryless_build_release_requires_review_even_with_preserve_marker |
| AUD-035 | Audit | integration:audit_build_release_boundary_scopes_local_and_canon_changes | Audit build release boundary scopes local and canon changes. | semantic | S4 | tests/govna_cli.rs::audit_build_release_boundary_scopes_local_and_canon_changes |
| AUD-036 | Audit | integration:audit_clean_run_leaves_existing_edited_stub_untouched | Audit clean run leaves existing edited stub untouched. | semantic | S4 | tests/govna_cli.rs::audit_clean_run_leaves_existing_edited_stub_untouched |
| AUD-037 | Audit | integration:audit_clean_run_does_not_consume_next_ac_number | Audit clean run does not consume next ac number. | semantic | S4 | tests/govna_cli.rs::audit_clean_run_does_not_consume_next_ac_number |
| AUD-038 | Audit | integration:audit_idempotent_reuse_and_edit_detection_guard | Audit idempotent reuse and edit detection guard. | semantic | S4 | tests/govna_cli.rs::audit_idempotent_reuse_and_edit_detection_guard |
| AUD-039 | Audit | integration:audit_cross_flavor_orphans | Audit cross flavor orphans. | semantic | S4 | tests/govna_cli.rs::audit_cross_flavor_orphans |
| AUD-040 | Audit | integration:audit_name_referenced_target_only_file | Audit name referenced target only file. | semantic | S4 | tests/govna_cli.rs::audit_name_referenced_target_only_file |
| AUD-041 | Audit | integration:audit_json_output_shape | Audit json output shape. | semantic | S4 | tests/govna_cli.rs::audit_json_output_shape |
| AUD-042 | Audit | integration:audit_rejects_positional_args | Audit rejects positional args. | semantic | S4 | tests/govna_cli.rs::audit_rejects_positional_args |
| AUD-043 | Audit | integration:audit_repo_name_override | Audit repo name override. | semantic | S4 | tests/govna_cli.rs::audit_repo_name_override |
| AUD-044 | Audit | integration:audit_diff_lines_truncates | Audit diff lines truncates. | semantic | S4 | tests/govna_cli.rs::audit_diff_lines_truncates |
| RND-021 | Render and canon | integration:render_metadata_txt_wins_over_manifest_inference | Render metadata txt wins over manifest inference. | semantic | S2 | tests/govna_cli.rs::render_metadata_txt_wins_over_manifest_inference |
| RND-022 | Render and canon | integration:render_fallback_flavor_conflict_errors | Render fallback flavor conflict errors. | semantic | S2 | tests/govna_cli.rs::render_fallback_flavor_conflict_errors |
| RND-023 | Render and canon | integration:render_fallback_flavor_absent_errors | Render fallback flavor absent errors. | semantic | S2 | tests/govna_cli.rs::render_fallback_flavor_absent_errors |
| RND-024 | Render and canon | integration:render_infers_terraform_from_tf_glob | Render infers terraform from tf glob. | semantic | S2 | tests/govna_cli.rs::render_infers_terraform_from_tf_glob |
| APL-001 | Apply | integration:apply_fresh_code_target | Apply fresh code target. | semantic | S3 | tests/govna_cli.rs::apply_fresh_code_target |
| APL-002 | Apply | integration:apply_fresh_doc_target | Apply fresh doc target. | semantic | S3 | tests/govna_cli.rs::apply_fresh_doc_target |
| APL-003 | Apply | integration:apply_infers_flavor_and_stack_from_manifest | Apply infers flavor and stack from manifest. | semantic | S3 | tests/govna_cli.rs::apply_infers_flavor_and_stack_from_manifest |
| APL-004 | Apply | integration:apply_unresolvable_flavor_errors | Apply unresolvable flavor errors. | semantic | S3 | tests/govna_cli.rs::apply_unresolvable_flavor_errors |
| APL-005 | Apply | integration:apply_reapply_bumps_ac_number | Apply reapply bumps ac number. | semantic | S3 | tests/govna_cli.rs::apply_reapply_bumps_ac_number |
| APL-006 | Apply | integration:apply_fresh_target_without_git_succeeds | Apply fresh target without git succeeds. | semantic | S3 | tests/govna_cli.rs::apply_fresh_target_without_git_succeeds |
| APL-007 | Apply | integration:apply_preserves_existing_regular_claude_file | Apply preserves existing regular claude file. | semantic | S3 | tests/govna_cli.rs::apply_preserves_existing_regular_claude_file |
| APL-008 | Apply | integration:apply_init_git_then_skips_on_rerun | Apply init git then skips on rerun. | semantic | S3 | tests/govna_cli.rs::apply_init_git_then_skips_on_rerun |
| APL-009 | Apply | integration:apply_refuses_govna_source | Apply refuses govna source. | semantic | S3 | tests/govna_cli.rs::apply_refuses_govna_source |
| APL-010 | Apply | integration:apply_excludes_govna_source_only_content | Apply excludes govna source only content. | semantic | S3 | tests/govna_cli.rs::apply_excludes_govna_source_only_content |
| APL-011 | Apply | integration:apply_adoption_ac_has_required_sections | Apply adoption ac has required sections. | semantic | S3 | tests/govna_cli.rs::apply_adoption_ac_has_required_sections |
| REM-001 | Removal | integration:rm_fresh_fixture_pure_canon_deletes | Rm fresh fixture pure canon deletes. | semantic | S5 | tests/govna_cli.rs::rm_fresh_fixture_pure_canon_deletes |
| REM-002 | Removal | integration:rm_hybrid_files_always_route_to_review | Rm hybrid files always route to review. | semantic | S5 | tests/govna_cli.rs::rm_hybrid_files_always_route_to_review |
| REM-003 | Removal | integration:rm_expected_divergence_files_kept | Rm expected divergence files kept. | semantic | S5 | tests/govna_cli.rs::rm_expected_divergence_files_kept |
| REM-004 | Removal | integration:rm_preserve_registry_routes_to_keep | Rm preserve registry routes to keep. | semantic | S5 | tests/govna_cli.rs::rm_preserve_registry_routes_to_keep |
| REM-005 | Removal | integration:rm_target_only_file_kept | Rm target only file kept. | semantic | S5 | tests/govna_cli.rs::rm_target_only_file_kept |
| REM-006 | Removal | integration:rm_edited_canon_file_routes_to_ambiguity | Rm edited canon file routes to ambiguity. | semantic | S5 | tests/govna_cli.rs::rm_edited_canon_file_routes_to_ambiguity |
| REM-007 | Removal | integration:rm_idempotent_reuse_and_edit_detection_guard | Rm idempotent reuse and edit detection guard. | semantic | S5 | tests/govna_cli.rs::rm_idempotent_reuse_and_edit_detection_guard |
| REM-008 | Removal | integration:rm_refuses_govna_source | Rm refuses govna source. | semantic | S5 | tests/govna_cli.rs::rm_refuses_govna_source |
| REM-009 | Removal | integration:rm_requires_adoption_and_git_worktree | Rm requires adoption and git worktree. | semantic | S5 | tests/govna_cli.rs::rm_requires_adoption_and_git_worktree |
| REM-010 | Removal | integration:rm_flavor_override_changes_canon_set | Rm flavor override changes canon set. | semantic | S5 | tests/govna_cli.rs::rm_flavor_override_changes_canon_set |
| REM-011 | Removal | integration:rm_rejects_positional_args | Rm rejects positional args. | semantic | S5 | tests/govna_cli.rs::rm_rejects_positional_args |
| APL-012 | Apply | integration:apply_migration_carries_over_legacy_metadata | Apply migration carries over legacy metadata. | semantic | S3 | tests/govna_cli.rs::apply_migration_carries_over_legacy_metadata |
| APL-013 | Apply | integration:apply_migration_emits_single_merged_ac | Apply migration emits single merged ac. | semantic | S3 | tests/govna_cli.rs::apply_migration_emits_single_merged_ac |
| APL-014 | Apply | integration:apply_migration_precise_tier_classifies_via_fake_governa | Apply migration precise tier classifies via fake governa. | semantic | S3 | tests/govna_cli.rs::apply_migration_precise_tier_classifies_via_fake_governa |
| APL-015 | Apply | integration:apply_migration_crude_tier_fallback_no_governa_binary | Apply migration crude tier fallback no governa binary. | semantic | S3 | tests/govna_cli.rs::apply_migration_crude_tier_fallback_no_governa_binary |
| APL-016 | Apply | integration:apply_migration_falls_back_when_render_fails | Apply migration falls back when render fails. | semantic | S3 | tests/govna_cli.rs::apply_migration_falls_back_when_render_fails |
| APL-017 | Apply | integration:apply_migration_idempotent_reuse_and_edit_detection_guard | Apply migration idempotent reuse and edit detection guard. | semantic | S3 | tests/govna_cli.rs::apply_migration_idempotent_reuse_and_edit_detection_guard |
| APL-018 | Apply | integration:apply_migration_noop_without_governa_dir_and_self_terminates | Apply migration noop without governa dir and self terminates. | semantic | S3 | tests/govna_cli.rs::apply_migration_noop_without_governa_dir_and_self_terminates |
| APL-019 | Apply | integration:apply_hunk_merges_agents_md_preserving_extra_bullets | Apply hunk merges agents md preserving extra bullets. | semantic | S3 | tests/govna_cli.rs::apply_hunk_merges_agents_md_preserving_extra_bullets |
| APL-020 | Apply | integration:apply_hunk_merge_idempotent_when_unmodified | Apply hunk merge idempotent when unmodified. | semantic | S3 | tests/govna_cli.rs::apply_hunk_merge_idempotent_when_unmodified |
| APL-021 | Apply | integration:apply_skips_readme_and_changelog_when_existing | Apply skips readme and changelog when existing. | semantic | S3 | tests/govna_cli.rs::apply_skips_readme_and_changelog_when_existing |
| APL-022 | Apply | integration:apply_new_mode_unaffected_by_hunk_merge_logic | Apply new mode unaffected by hunk merge logic. | semantic | S3 | tests/govna_cli.rs::apply_new_mode_unaffected_by_hunk_merge_logic |
| APL-023 | Apply | integration:apply_falls_back_to_overwrite_when_boundary_missing | Apply falls back to overwrite when boundary missing. | semantic | S3 | tests/govna_cli.rs::apply_falls_back_to_overwrite_when_boundary_missing |
| APL-024 | Apply | integration:apply_preserves_boundaryless_build_release_for_manual_migration | Apply preserves boundaryless build release for manual migration. | semantic | S3 | tests/govna_cli.rs::apply_preserves_boundaryless_build_release_for_manual_migration |
| APL-025 | Apply | integration:apply_merges_build_release_project_practices | Apply merges build release project practices. | semantic | S3 | tests/govna_cli.rs::apply_merges_build_release_project_practices |
| APL-026 | Apply | integration:apply_preserves_existing_arch_and_plan_content | Apply preserves existing arch and plan content. | semantic | S3 | tests/govna_cli.rs::apply_preserves_existing_arch_and_plan_content |
| APL-027 | Apply | integration:apply_new_mode_labels_every_file_written | Apply new mode labels every file written. | semantic | S3 | tests/govna_cli.rs::apply_new_mode_labels_every_file_written |
| APL-028 | Apply | integration:apply_existing_mode_ac_labels_preserved_files_correctly | Apply existing mode ac labels preserved files correctly. | semantic | S3 | tests/govna_cli.rs::apply_existing_mode_ac_labels_preserved_files_correctly |
| APL-029 | Apply | integration:apply_ac_at1_is_manual_review_wording | Apply ac at1 is manual review wording. | semantic | S3 | tests/govna_cli.rs::apply_ac_at1_is_manual_review_wording |
| APL-030 | Apply | integration:apply_ac_at3_reflects_symlink_conflict | Apply ac at3 reflects symlink conflict. | semantic | S3 | tests/govna_cli.rs::apply_ac_at3_reflects_symlink_conflict |
| RND-025 | Render and canon | integration:render_doc_closure_audit_bullet_has_no_code_vocabulary | Render doc closure audit bullet has no code vocabulary. | semantic | S2 | tests/govna_cli.rs::render_doc_closure_audit_bullet_has_no_code_vocabulary |
| RND-026 | Render and canon | integration:render_audit_docs_and_version_match_authority | Render audit docs and version match authority. | semantic | S2 | tests/govna_cli.rs::render_audit_docs_and_version_match_authority |
| RND-027 | Render and canon | integration:development_guidance_uses_settled_imperative_style | Development guidance uses settled imperative style. | semantic | S2 | tests/govna_cli.rs::development_guidance_uses_settled_imperative_style |
| RND-028 | Render and canon | integration:render_code_build_release_is_stack_aware_and_bounded | Render code build release is stack aware and bounded. | semantic | S2 | tests/govna_cli.rs::render_code_build_release_is_stack_aware_and_bounded |
| BLD-001 | Product tooling | integration:rendered_rust_prep_validation_token_contract_matches_source | Rendered rust prep validation token contract matches source. | semantic | S6 | tests/govna_cli.rs::rendered_rust_prep_validation_token_contract_matches_source |
| RND-029 | Render and canon | integration:rendered_agents_scope_rust_validation_token_contract | Rendered agents scope rust validation token contract. | semantic | S2 | tests/govna_cli.rs::rendered_agents_scope_rust_validation_token_contract |
| RND-030 | Render and canon | integration:rendered_refine_no_change_contract_matches_authority | Rendered refine no change contract matches authority. | semantic | S2 | tests/govna_cli.rs::rendered_refine_no_change_contract_matches_authority |
| RND-031 | Render and canon | integration:render_project_rules_seed_has_no_govna_specific_content | Render project rules seed has no govna specific content. | semantic | S2 | tests/govna_cli.rs::render_project_rules_seed_has_no_govna_specific_content |
| RND-032 | Render and canon | integration:root_agents_project_rules_unchanged | Root agents project rules unchanged. | semantic | S2 | tests/govna_cli.rs::root_agents_project_rules_unchanged |
| BLD-002 | Product tooling | integration:build_sh_validates_canon_version_bump | Build sh validates canon version bump. | semantic | S6 | tests/govna_cli.rs::build_sh_validates_canon_version_bump |
| BLD-003 | Product tooling | integration:rust_stack_template_omits_canon_version_check | Rust stack template omits canon version check. | semantic | S6 | tests/govna_cli.rs::rust_stack_template_omits_canon_version_check |
| APL-031 | Apply | integration:apply_boundary_fallback_labeled_distinctly_in_ac | Apply boundary fallback labeled distinctly in ac. | semantic | S3 | tests/govna_cli.rs::apply_boundary_fallback_labeled_distinctly_in_ac |
| BLD-004 | Product tooling | build:test_utility_declaration_validation | Test utility declaration validation. | semantic | S6 | tests/build_cli.sh::test_utility_declaration_validation |
| BLD-005 | Product tooling | build:test_compiled_version_output | Test compiled version output. | byte-exact | S6 | tests/build_cli.sh::test_compiled_version_output |
| BLD-006 | Product tooling | build:test_manifest_path_mapping | Test manifest path mapping. | implementation-specific | S6 | Reason: Cargo manifest parsing is a Rust product mechanism; preserve Go target-discovery outcomes through BND-001. |
| BLD-007 | Product tooling | build:test_install_reporting | Test install reporting. | semantic | S6 | tests/build_cli.sh::test_install_reporting |
| BLD-008 | Product tooling | build:test_prep_no_build_rejection | Test prep no build rejection. | semantic | S6 | tests/build_cli.sh::test_prep_no_build_rejection |
| BLD-009 | Product tooling | build:test_validation_token_tracks_git_visible_state | Test validation token tracks git visible state. | semantic | S6 | tests/build_cli.sh::test_validation_token_tracks_git_visible_state |
| BLD-010 | Product tooling | build:test_baseline_validation_token_refresh | Test baseline validation token refresh. | semantic | S6 | tests/build_cli.sh::test_baseline_validation_token_refresh |
| BLD-011 | Product tooling | build:test_refresh_validation_token_cli_contract | Test refresh validation token cli contract. | semantic | S6 | tests/build_cli.sh::test_refresh_validation_token_cli_contract |
| BLD-012 | Product tooling | build:test_govna_prep_version_mutation | Test govna prep version mutation. | semantic | S6 | tests/build_cli.sh::test_govna_prep_version_mutation |
| BLD-013 | Product tooling | build:test_shared_cargo_target_ownership | Test shared cargo target ownership. | implementation-specific | S6 | Reason: Cargo target ownership is a Rust product mechanism; preserve isolated temporary-output ownership through BND-001. |
| BLD-014 | Product tooling | build:test_prep_evidence_routing | Test prep evidence routing. | semantic | S6 | tests/build_cli.sh::test_prep_evidence_routing |
| BLD-015 | Product tooling | build:test_prep_validation_token_cli | Test prep validation token cli. | semantic | S6 | tests/build_cli.sh::test_prep_validation_token_cli |
| BLD-016 | Product tooling | build:test_fallback_failure_precedes_mutation | Test fallback failure precedes mutation. | semantic | S6 | tests/build_cli.sh::test_fallback_failure_precedes_mutation |
| BLD-017 | Product tooling | build:test_prep_phase_output_modes | Test prep phase output modes. | semantic | S6 | tests/build_cli.sh::test_prep_phase_output_modes |
| BLD-018 | Product tooling | build:test_successful_full_build_emits_token | Test successful full build emits token. | semantic | S6 | tests/build_cli.sh::test_successful_full_build_emits_token |
| BLD-019 | Product tooling | build:test_failed_full_build_omits_token | Test failed full build omits token. | semantic | S6 | tests/build_cli.sh::test_failed_full_build_omits_token |
| BLD-020 | Product tooling | build:test_prep_verbose_aliases | Test prep verbose aliases. | semantic | S6 | tests/build_cli.sh::test_prep_verbose_aliases |
| BLD-021 | Product tooling | build:test_owned_target_cleanup_paths | Test owned target cleanup paths. | implementation-specific | S6 | Reason: Cargo target cleanup paths are a Rust product mechanism; preserve tool-owned temporary-output cleanup through BND-001. |
| DIF-001 | Apply | integration:apply_init_git_then_skips_on_rerun | Initialize a new Git repository on `main` for `apply -g` and never create or publish `master`. | intentional-difference | S3 | Reason: replace platform-dependent default-branch behavior with the settled `main` invariant; verify with an S3 Go regression test. |
