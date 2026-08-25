package canon

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type instructionException struct {
	path   string
	text   string
	reason string
}

var secondAction = regexp.MustCompile(`(?i)\b(?:and|or)\s+(?:also\s+)?(?:add|adopt|allow|apply|ask|avoid|begin|capture|change|check|classify|complete|confirm|continue|create|define|delete|detect|draft|edit|emit|end|ensure|fail|follow|format|infer|install|keep|leave|limit|make|mark|move|omit|pass|place|prefer|preserve|prevent|prohibit|record|refresh|reject|remove|render|replace|require|reserve|resolve|restore|return|reuse|review|route|run|scan|set|skip|split|start|state|stop|synchronize|treat|update|use|validate|verify|wait|write)\b|\bpre-\s+and\s+post-`)
var acDocument = regexp.MustCompile(`^ac[0-9]+-`)

var instructionExceptions = []instructionException{
	{"AGENTS.md", "Ask the Director to narrow the task or split the AC before proposing delegation when the task exceeds practical inline capacity.", "one ask action offers two exclusive request objects"},
	{"AGENTS.md", "Map every in-scope command entry point, provider/API fetch, normalized-table write, durable snapshot, stale fallback, freshness gate, and complete-snapshot reconciliation path in the closure audit.", "one map action applies to a path-category list"},
	{"AGENTS.md", "Reach for `Read` only to fetch unseen content or check for recent changes.", "one reach action has two exclusive purposes"},
	{"govna/development-cycle.md", "Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.", "one keep action applies to two object classes"},
	{"internal/canon/assets/overlays/code/files/govna/development-cycle.md.tmpl", "Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.", "one keep action applies to two object classes"},
	{"internal/canon/assets/overlays/doc/files/govna/editing-cycle.md.tmpl", "Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.", "one keep action applies to two object classes"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Ask the Director to narrow the task or split the AC before proposing delegation when the task exceeds practical inline capacity.", "one ask action offers two exclusive request objects"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Map every in-scope command entry point, provider/API fetch, normalized-table write, durable snapshot, stale fallback, freshness gate, and complete-snapshot reconciliation path in the closure audit.", "one map action applies to a path-category list"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Reach for `Read` only to fetch unseen content or check for recent changes.", "one reach action has two exclusive purposes"},
	{"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl", "Ask the Director to narrow the task or split the AC before proposing delegation when the task exceeds practical inline capacity.", "one ask action offers two exclusive request objects"},
	{"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl", "Reach for `Read` only to fetch unseen content or check for recent changes.", "one reach action has two exclusive purposes"},
	{"internal/canon/assets/overlays/code/files/arch.md.tmpl", "storage, messaging, or state boundaries", "architecture-outline list item is not an instruction"},
}

type rewrittenInstructionReview struct {
	v032        string
	v033        string
	paths       []string
	disposition string
}

const (
	reviewClean            = "clean"
	reviewCorrectedStarter = "corrected non-imperative starter"
)

var rewrittenInstructionPaths = []string{
	"AGENTS.md",
	"govna/audit.md",
	"govna/build-release.md",
	"govna/code-stacks.md",
	"govna/development-guidelines.md",
	"govna/roles.md",
	"internal/canon/assets/base/AGENTS.md.tmpl",
	"internal/canon/assets/overlays/code/files/govna/audit.md.tmpl",
	"internal/canon/assets/overlays/code/files/govna/build-release.md.tmpl",
	"internal/canon/assets/overlays/code/files/govna/code-stacks.md.tmpl",
	"internal/canon/assets/overlays/code/files/govna/development-guidelines.md.tmpl",
	"internal/canon/assets/overlays/code/files/govna/roles.md.tmpl",
	"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl",
	"internal/canon/assets/overlays/doc/files/govna/audit.md.tmpl",
	"internal/canon/assets/overlays/doc/files/govna/editing-guidelines.md.tmpl",
	"internal/canon/assets/overlays/doc/files/govna/roles.md.tmpl",
}

var (
	agentsRewrittenPaths = []string{
		"AGENTS.md",
		"internal/canon/assets/base/AGENTS.md.tmpl",
		"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl",
	}
	auditRewrittenPaths = []string{
		"govna/audit.md",
		"internal/canon/assets/overlays/code/files/govna/audit.md.tmpl",
		"internal/canon/assets/overlays/doc/files/govna/audit.md.tmpl",
	}
	buildReleaseRewrittenPaths = []string{
		"govna/build-release.md",
		"internal/canon/assets/overlays/code/files/govna/build-release.md.tmpl",
	}
	buildReleaseProjectRewrittenPaths = []string{
		"govna/build-release.md",
	}
	codeStacksRewrittenPaths = []string{
		"govna/code-stacks.md",
		"internal/canon/assets/overlays/code/files/govna/code-stacks.md.tmpl",
	}
	developmentRewrittenPaths = []string{
		"govna/development-guidelines.md",
		"internal/canon/assets/overlays/code/files/govna/development-guidelines.md.tmpl",
	}
	rolesRewrittenPaths = []string{
		"govna/roles.md",
		"internal/canon/assets/overlays/code/files/govna/roles.md.tmpl",
		"internal/canon/assets/overlays/doc/files/govna/roles.md.tmpl",
	}
	editingRewrittenPaths = []string{
		"internal/canon/assets/overlays/doc/files/govna/editing-guidelines.md.tmpl",
	}
)

var rewrittenInstructionReviews = []rewrittenInstructionReview{
	{"Name the exact sections to change during every update.", "Name the exact sections to change during every update.", agentsRewrittenPaths, reviewClean},
	{"Keep edits local during every update.", "Keep edits local during every update.", agentsRewrittenPaths, reviewClean},
	{"Create files only after explicit user authorization — including draft files, scratch scripts, scaffolding, and config tweaks.", "Create files only after explicit user authorization — including draft files, scratch scripts, scaffolding, and config tweaks.", agentsRewrittenPaths, reviewClean},
	{"Make repository edits only after explicit user authorization.", "Make repository edits only after explicit user authorization.", agentsRewrittenPaths, reviewClean},
	{"Stop when a request lacks authorization, scope, or required context.", "Stop when a request lacks authorization, scope, or required context.", agentsRewrittenPaths, reviewClean},
	{"Ask for the missing authorization, scope, or context.", "Ask for the missing authorization, scope, or context.", agentsRewrittenPaths, reviewClean},
	{"Mark every temporary consumer mitigation explicitly.", "Mark every temporary consumer mitigation explicitly.", agentsRewrittenPaths, reviewClean},
	{"State every temporary consumer mitigation's removal condition.", "State every temporary consumer mitigation's removal condition.", agentsRewrittenPaths, reviewClean},
	{"Stop when a request is ambiguous or the change is hard to reverse.", "Stop when a request is ambiguous or the change is hard to reverse.", agentsRewrittenPaths, reviewClean},
	{"Ask for direction before proceeding.", "Ask for direction before proceeding.", agentsRewrittenPaths, reviewClean},

	{"Place the count paragraph first under `## Summary`.", "Place the count paragraph first under `## Summary`.", auditRewrittenPaths, reviewClean},
	{"Start the count paragraph with `This adoption covers`.", "Start the count paragraph with `This adoption covers`.", auditRewrittenPaths, reviewClean},
	{"Recompute the protected-region digest after adoption.", "Recompute the protected-region digest after adoption.", auditRewrittenPaths, reviewClean},
	{"Require the protected-region digest to match the emitted digest.", "Require the protected-region digest to match the emitted digest.", auditRewrittenPaths, reviewClean},

	{"Require `--version` to exit 0.", "Require `--version` to exit 0.", buildReleaseRewrittenPaths, reviewClean},
	{"Require `--version` to print exactly `<utility-id> <MAJOR.MINOR.PATCH>` or `<utility-id> v<MAJOR.MINOR.PATCH>` plus its newline to stdout.", "Require `--version` to print exactly `<utility-id> <MAJOR.MINOR.PATCH>` or `<utility-id> v<MAJOR.MINOR.PATCH>` plus its newline to stdout.", buildReleaseRewrittenPaths, reviewClean},
	{"Require `--version` to write nothing to stderr.", "Require `--version` to write nothing to stderr.", buildReleaseRewrittenPaths, reviewClean},
	{"Run ordinary canonical pre-change validation for Go prep.", "Run ordinary canonical pre-change validation for Go prep.", buildReleaseRewrittenPaths, reviewClean},
	{"Run ordinary canonical post-change validation for Go prep.", "Run ordinary canonical post-change validation for Go prep.", buildReleaseRewrittenPaths, reviewClean},
	{"Reserve validation-token evidence for Rust prep.", "Reserve validation-token evidence for Rust prep.", buildReleaseRewrittenPaths, reviewClean},
	{"Refresh validation-token evidence for Rust prep.", "Refresh validation-token evidence for Rust prep.", buildReleaseRewrittenPaths, reviewClean},
	{"Rebuild a release from its clean tagged commit before publication.", "Rebuild a release from its clean tagged commit before publication.", buildReleaseProjectRewrittenPaths, reviewClean},
	{"Install the rebuilt release before publication.", "Install the rebuilt release before publication.", buildReleaseProjectRewrittenPaths, reviewClean},

	{"Run ordinary canonical pre-change validation during Go release prep.", "Run ordinary canonical pre-change validation during Go release prep.", codeStacksRewrittenPaths, reviewClean},
	{"Run ordinary canonical post-change validation during Go release prep.", "Run ordinary canonical post-change validation during Go release prep.", codeStacksRewrittenPaths, reviewClean},
	{"Bump the root package version during release prep.", "Bump the root package version during release prep.", codeStacksRewrittenPaths, reviewClean},
	{"Refresh `Cargo.lock` during release prep.", "Refresh `Cargo.lock` during release prep.", codeStacksRewrittenPaths, reviewClean},
	{"Accept declared binary names for scoped builds.", "Accept declared binary names for scoped builds.", codeStacksRewrittenPaths, reviewClean},
	{"Preserve package-wide shared validation during scoped builds.", "Preserve package-wide shared validation during scoped builds.", codeStacksRewrittenPaths, reviewClean},
	{"Derive release versions from Git tags.", "Derive release versions from Git tags.", codeStacksRewrittenPaths, reviewClean},
	{"Leave `Package.swift` unchanged during release prep.", "Leave `Package.swift` unchanged during release prep.", codeStacksRewrittenPaths, reviewClean},
	{"Build only selected executable products during scoped builds.", "Build only selected executable products during scoped builds.", codeStacksRewrittenPaths, reviewClean},
	{"Install only selected executable products during scoped builds.", "Install only selected executable products during scoped builds.", codeStacksRewrittenPaths, reviewClean},

	{"Never assume old data fits new schemas.", "Verify old data compatibility with new schemas.", developmentRewrittenPaths, reviewCorrectedStarter},
	{"Write migration logic for old data.", "Write migration logic for old data.", developmentRewrittenPaths, reviewClean},
	{"Fail explicitly when migration logic is unavailable.", "Fail explicitly when migration logic is unavailable.", developmentRewrittenPaths, reviewClean},

	{"Run `./build.sh` when the change touches code or build-relevant files (skip for AC critique, doc-only review, design discussion).", "Run `./build.sh` when the change touches code or build-relevant files (skip for AC critique, doc-only review, design discussion).", rolesRewrittenPaths, reviewClean},
	{"Confirm that `./build.sh` passes when the change touches code or build-relevant files.", "Confirm that `./build.sh` passes when the change touches code or build-relevant files.", rolesRewrittenPaths, reviewClean},
	{"Run each acceptance test in the active AC when it can be exercised.", "Run each acceptance test in the active AC when it can be exercised.", rolesRewrittenPaths, reviewClean},
	{"Report the result of each exercised acceptance test.", "Report the result of each exercised acceptance test.", rolesRewrittenPaths, reviewClean},
	{"State explicitly why each unexercised acceptance test was only reasoned about.", "State explicitly why each unexercised acceptance test was only reasoned about.", rolesRewrittenPaths, reviewClean},
	{"Do not negotiate scope questions without the director in the loop.", "Do not negotiate scope questions without the director in the loop.", rolesRewrittenPaths, reviewClean},
	{"Do not resolve scope questions without the director in the loop.", "Do not resolve scope questions without the director in the loop.", rolesRewrittenPaths, reviewClean},

	{"Grep the repository for all references when renaming or moving a file.", "Grep the repository for all references when renaming or moving a file.", editingRewrittenPaths, reviewClean},
	{"Update every reference in the same pass.", "Update every reference in the same pass.", editingRewrittenPaths, reviewClean},
}

var currentInstructionReplacements = map[string]string{
	"Start the count paragraph with `This adoption covers`.": "Start the count paragraph with `Govna found`.",
}

func currentReviewedInstruction(review rewrittenInstructionReview) string {
	if current, ok := currentInstructionReplacements[review.v033]; ok {
		return current
	}
	return review.v033
}

type generatedInstructionTemplate struct {
	id   string
	text string
}

var generatedInstructionManifest = []generatedInstructionTemplate{
	{"I01", "Verify AGENTS.md reflects the repository's actual practices."},
	{"I02", "Verify govna/roles.md reflects the repository's delivery model (Operator + Director)."},
	{"I03", "Verify CLAUDE.md is a symlink to AGENTS.md."},
	{"I04", "Verify CLAUDE.md remains the existing regular file instead of a symlink to AGENTS.md."},
	{"I05", "Resolve every Director choice in chat."},
	{"I06", "Leave this generated AC unchanged."},
	{"I07", "Create a temporary copy of the embedded Govna files with govna render."},
	{"I08", "Confirm each file selected for update exists in the selected CODE render."},
	{"I09", "Apply each Director choice only to its authorized file region."},
	{"I10", "Write govna/canon-baseline.txt as the final file update."},
	{"I11", "Remove the legacy preserve phrase for <path> only after the file update and preserve-list state are verified."},
	{"I12", "Write govna/canon-baseline.txt from the final temporary render only after all other work is complete and the repository check succeeds."},
	{"I13", "Create govna/metadata.txt from the selected temporary render before govna/canon-baseline.txt installation."},
	{"I14", "Create <path> from the selected temporary render."},
	{"I15", "Verify every file selected for update except govna/canon-baseline.txt against the rendered Govna files and every preserved file against govna/preserve.txt."},
	{"I16", "Preserve the protected region in <path> from <boundary> through EOF with SHA-256 <hash> for any update choice."},
	{"I17", "Run <command> after the selected file updates and before govna/canon-baseline.txt installation (<evidence>)."},
	{"I18", "Verify the final file update installed govna/canon-baseline.txt from the same temporary render."},
	{"I19", "Create a temporary copy of the selected Govna files with <render-command>."},
	{"I20", "Preserve every file under Routing Decisions until the Director resolves it."},
	{"I21", "Apply each in-scope removal and Director choice."},
	{"I22", "Apply each in-scope removal."},
	{"I23", "Compare <path> with <diff-command>."},
	{"I24", "Choose what to remove from <path>: only its Govna-managed section, nothing, or the whole file."},
	{"I25", "Verify every resolved removal target under ## In Scope is absent."},
	{"I26", "Verify every file under Routing Decisions matches its Director-resolved action."},
	{"I27", "Verify every keep-local choice is applied before the final removal of govna/preserve.txt."},
	{"I28", "Choose the repository check in chat."},
	{"I29", "Verify the chosen repository check succeeds before govna/canon-baseline.txt installation."},
}

var generatedSecondAction = regexp.MustCompile(`(?i)(?:\b(?:and|or|then)\s+(?:also\s+)?|;\s*|[.!?]\s+)(?:apply|choose|compare|create|install|leave|preserve|remove|render|resolve|satisfy|verify)(?:\s|$)|\b(?:before|after)\s+(?:apply|choose|compare|create|install|leave|preserve|remove|render|resolve|satisfy|verify|applying|choosing|comparing|creating|installing|leaving|preserving|removing|rendering|resolving|satisfying|verifying)(?:\s|$)`)

var (
	legacyPreserveInstruction  = regexp.MustCompile("^Remove the legacy preserve phrase for `[^`]+` only after the file update and preserve-list state are verified\\.$")
	protectedRegionInstruction = regexp.MustCompile("^Preserve the protected region in `[^`]+` from `[^`]+` through EOF with SHA-256 `[^`]+` for any update choice\\.$")
	inferredCheckInstruction   = regexp.MustCompile("^Run `[^`]+` after the selected file updates and before `govna/canon-baseline.txt` installation \\([^)]*\\)\\.$")
	notApplicableInstruction   = regexp.MustCompile("^Verify the `Not applicable` evidence still holds after the selected file updates and before `govna/canon-baseline.txt` installation \\([^)]*\\)\\.$")
	renderRemovalInstruction   = regexp.MustCompile("^Create a temporary copy of the selected Govna files with `govna render --flavor (?:doc|code(?: --stack [^`]+)?) <scratch>`\\.$")
	compareRemovalInstruction  = regexp.MustCompile("^Compare `[^`]+` with `diff -ru <scratch>/[^`]+ [^`]+`\\.$")
	chooseRemovalInstruction   = regexp.MustCompile("^Choose what to remove from `[^`]+`: only its Govna-managed section, nothing, or the whole file\\.$")
	otherMigrationInstruction  = regexp.MustCompile("^Create `[^`]+` from the selected temporary render\\.$")
)

var generatedProseStarters = func() map[string]bool {
	words := strings.Fields("Add Adopt Allow Apply Ask Avoid Begin Capture Change Check Choose Classify Compare Complete Confirm Continue Create Define Delete Detect Do Draft Edit Emit End Ensure Extricate Fail Follow Format Infer Install Keep Leave Limit Make Mark Move Omit Pass Place Prefer Preserve Prevent Prohibit Record Refresh Reject Remove Render Replace Require Reserve Resolve Restore Return Reuse Review Route Run Satisfy Scan Set Skip Split Start State Stop Synchronize Treat Update Use Validate Verify Wait Write Implement")
	starters := make(map[string]bool, len(words))
	for _, word := range words {
		starters[word] = true
	}
	return starters
}()

func TestGeneratedInstructionManifest(t *testing.T) {
	if len(generatedInstructionManifest) != 29 {
		t.Fatalf("generated instruction manifest has %d entries, want 29", len(generatedInstructionManifest))
	}
	seenText := map[string]string{}
	for index, instruction := range generatedInstructionManifest {
		wantID := fmt.Sprintf("I%02d", index+1)
		if instruction.id != wantID {
			t.Errorf("generated instruction ID %q at index %d, want %q", instruction.id, index, wantID)
		}
		if instruction.text == "" {
			t.Errorf("generated instruction %s is empty", instruction.id)
			continue
		}
		if prior, exists := seenText[instruction.text]; exists {
			t.Errorf("generated instruction %s duplicates %s: %s", instruction.id, prior, instruction.text)
		}
		seenText[instruction.text] = instruction.id
		if normalized := normalizeGeneratedInstruction(instruction.text); normalized != instruction.text {
			t.Errorf("manifest instruction %s is not normalized: %q -> %q", instruction.id, instruction.text, normalized)
		}
		if err := validateGeneratedInstruction(instruction.text); err != nil {
			t.Errorf("generated instruction %s is invalid: %v", instruction.id, err)
		}
	}
}

func TestGeneratedInstructionStarters(t *testing.T) {
	for _, text := range []string{
		"Director reads AGENTS.md.",
		"Removed files no longer exist.",
		"Every preserve decision is applied.",
		"When a path is synced, preserve its tail.",
		"Never assume generated prose is valid.",
		"Always run the command.",
		"CLAUDE.md remains a regular file.",
	} {
		if generatedImperativeStarter(text) {
			t.Errorf("accepted invalid generated instruction starter: %s", text)
		}
	}
	seen := map[string]bool{}
	for _, instruction := range generatedInstructionManifest {
		starter := firstInstructionWord(instruction.text)
		if seen[starter] {
			continue
		}
		seen[starter] = true
		if !generatedImperativeStarter(instruction.text) {
			t.Errorf("rejected manifest starter %s: %s", starter, instruction.text)
		}
	}
}

func TestGeneratedInstructionAtomicity(t *testing.T) {
	for _, text := range []string{
		"Verify the path and Remove the legacy phrase.",
		"Verify the path; Remove the legacy phrase.",
		"Verify the path. Remove the legacy phrase.",
		"Verify the path then Remove the legacy phrase.",
		"Compare the path before choosing its route.",
		"Verify the path after applying the route.",
	} {
		if !generatedSecondAction.MatchString(text) {
			t.Errorf("did not reject generated compound action: %s", text)
		}
		if err := validateGeneratedInstruction(text); err == nil {
			t.Errorf("generated instruction gate accepted compound action: %s", text)
		}
	}
	for _, instruction := range generatedInstructionManifest {
		if generatedSecondAction.MatchString(instruction.text) {
			t.Errorf("rejected atomic manifest instruction %s: %s", instruction.id, instruction.text)
		}
	}
}

func TestGeneratedGoldenInstructionGate(t *testing.T) {
	expected := map[string]map[string]int{
		"internal/apply/testdata/fresh-code-golden.md": {"I01": 1, "I02": 1, "I03": 1},
		"internal/apply/testdata/fresh-doc-golden.md":  {"I01": 1, "I02": 1, "I03": 1},
		"internal/apply/testdata/existing-golden.md":   {"I01": 1, "I02": 1, "I04": 1},
		"internal/audit/testdata/actionable-golden.md": {
			"I05": 1, "I06": 1, "I07": 1, "I08": 1, "I09": 1, "I10": 1, "I15": 1, "I17": 1, "I18": 1,
		},
		"internal/audit/testdata/unresolved-validation-golden.md": {
			"I05": 1, "I06": 1, "I07": 1, "I08": 1, "I09": 1, "I10": 1, "I15": 1, "I16": 1, "I18": 1, "I28": 1, "I29": 1,
		},
		"internal/remove/testdata/removal-golden.md": {
			"I05": 1, "I19": 1, "I20": 1, "I21": 1, "I23": 2, "I24": 2, "I25": 1, "I26": 1, "I27": 1,
		},
	}
	manifest := generatedManifestByText(t)
	root := filepath.Join("..", "..")
	for path, want := range expected {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]int{}
		for _, instruction := range extractGeneratedInstructions(string(content)) {
			if err := validateGeneratedInstruction(instruction); err != nil {
				t.Errorf("%s: %v", path, err)
				continue
			}
			normalized := normalizeGeneratedInstruction(instruction)
			got[manifest[normalized]]++
		}
		if !equalInstructionCounts(got, want) {
			t.Errorf("%s instruction occurrences=%v want=%v", path, got, want)
		}
	}
}

func TestGeneratedInstructionDocumentation(t *testing.T) {
	expectations := map[string][]string{
		"README.md": {
			"executable version—the version of the installed `govna` program—separately from the canon version",
			"stub filenames remain keyed by canon version",
			"legacy canon-only marker upgrades in place",
			"language checks run separately from byte-for-byte fixture comparisons",
		},
		"arch.md": {
			"executable version and canon version separately",
			"one unedited AC keyed by canon version",
			"marker records the executable and canon versions separately",
			"one canon-version-keyed removal AC",
		},
	}
	root := filepath.Join("..", "..")
	for path, required := range expectations {
		content, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range required {
			if !strings.Contains(string(content), phrase) {
				t.Errorf("%s omits generated-emission documentation %q", path, phrase)
			}
		}
	}
}

func TestPlainLanguageContractAndMirrors(t *testing.T) {
	root := filepath.Join("..", "..")
	paths := []string{
		"AGENTS.md",
		"internal/canon/assets/base/AGENTS.md.tmpl",
		"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl",
	}
	required := []string{
		"### Plain Language\n\n- Apply plain-language rules to responses, ACs, findings, completion reports, and release summaries.\n- Lead with the concrete problem, effect, or decision in plain language.\n- Pair each necessary Govna label with its plain-language meaning at first use.",
		"- Treat changed-content integrity, AC-template structure, Plain Language, Instruction Style, and applicable Pre-Implementation Verification as the tests-in-the-same-pass gate when a change pass creates or edits only an active AC document.",
		"- Confirm the AC title and Summary lead with the concrete outcome in plain language.",
		"- Resolve an unresolved emitted repository check in chat.",
		"- Run the chosen repository command after all selected sync, migration, and deletion work.",
		"- Cite repository evidence when choosing `Not applicable` for the repository check.",
		"- Write `govna/canon-baseline.txt` from the scratch render only after every other applicable acceptance test and routing outcome passes and the resolved repository check succeeds or its `Not applicable` evidence holds.",
	}
	for _, path := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, text := range required {
			if strings.Count(string(content), text) != 1 {
				t.Errorf("%s requires one occurrence of %q", path, text)
			}
		}
	}
}

func TestPlainLanguageFirstUseExplanations(t *testing.T) {
	root := filepath.Join("..", "..")
	checks := map[string][]string{
		"README.md": {
			"That embedded file set is the canon.",
			"Adding Govna's governance files to a repository is adoption.",
			"selected CODE or DOC file set (the flavor)",
			"executable version—the version of the installed `govna` program—separately from the canon version",
			"baseline, the saved hashes of Govna-managed file regions previously installed there",
			"preserve registry, the list of files a Director chose to keep local",
			"classification, which is the exact result label explaining its state",
			"routing decision: a Director choice to update, keep, migrate, or remove the file",
			"repository check, meaning the command to run after updates or the reason no command applies",
			"This temporary copy is a scratch render",
			"A consumer repository is any repository that has adopted Govna.",
		},
		"govna/roles.md": {
			"effective implementation scope as a narrow exception for a directly broken supporting artifact whose result the Director already settled",
			"bounded completeness correction as an Implement-time fix for a missed path or instruction whose required result is already settled by the active AC",
		},
		"govna/operator-contract-rationale.md": {
			"A contract-integrity finding reports a proven governance-rule problem rather than an implementation bug.",
			"A contract-growth review checks whether new rules duplicate, hide, misplace, or crowd out existing rules.",
			"final read-only closure audit",
			"final AC wording and scope check called Pre-Implementation Verification",
		},
	}
	for path, phrases := range checks {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range phrases {
			if !strings.Contains(string(content), phrase) {
				t.Errorf("%s omits first-use explanation %q", path, phrase)
			}
		}
	}
}

func TestScopedPlainLanguageReplacements(t *testing.T) {
	type replacement struct {
		path, legacy, current string
	}
	checks := []replacement{
		{"govna/audit.md", "Start the count paragraph with `This adoption covers`.", "Start the count paragraph with `Govna found`."},
		{"govna/audit.md", "Start the Summary sentences with `This audit adoption synchronizes`", "Start the repository paragraph with `This AC updates`."},
		{"govna/audit.md", "Audit surfaced", "Follow it with `The result label (classification)`."},
		{"internal/audit/audit.go", " Audit v", "Review Govna File Updates"},
		{"internal/audit/audit.go", "This adoption covers", "Govna found %s, %s, %s, and %s."},
		{"internal/audit/audit.go", "This audit adoption synchronizes", "This AC updates"},
		{"internal/audit/audit.go", "Per-file inspection uses rendered canon", "The result label (classification) beside each path explains why Govna can update it"},
		{"internal/audit/audit.go", "Validation disposition", "**Repository check**"},
		{"internal/audit/audit.go", "Which outcome applies after selected work", "Which command should run after the selected file updates"},
		{"internal/audit/audit.go", "Resolve the validation disposition in chat.", "Choose the repository check in chat."},
		{"internal/audit/audit.go", "Satisfy the resolved validation disposition", "Verify the chosen repository check succeeds"},
		{"internal/audit/audit.go", "Satisfy validation disposition", "Run `./build.sh` after the selected file updates"},
		{"internal/audit/audit.go", "Verify every direct-sync and canon-backed migration path", "Confirm each file selected for update exists in the selected CODE render."},
		{"internal/remove/remove.go", "Govna Removal from", "Review Removal of Govna Files"},
		{"internal/remove/remove.go", "This removal AC was emitted by govna executable", "Govna executable v%s created this removal plan from its embedded governance files"},
		{"internal/remove/remove.go", "It removes Govna canon from this consumer repository", "This AC removes Govna-managed content without deleting repository-owned content."},
		{"internal/remove/remove.go", "Director-resolved routing protects every review path", "Files needing a choice stay unchanged until the Director decides what to do."},
		{"internal/remove/remove.go", "routing-pending path", "file under Routing Decisions"},
		{"internal/remove/remove.go", "mixed canon-shape and consumer content", "contains both Govna-managed and repository-owned content"},
		{"internal/remove/remove.go", "consumer-edited canon file", "Govna-managed file has local edits"},
		{"internal/remove/remove.go", "byte-equal govna canon", "matches the current Govna file exactly"},
		{"internal/remove/remove.go", "target-only repo-owned file", "repository-owned file with no matching entry in Govna's current canon"},
		{"internal/remove/remove.go", "repo-owned govna-adjacent content", "a repository-owned planning file that Govna never manages"},
		{"internal/remove/remove.go", "Choose one route for", "Choose what to remove from"},
		{"internal/apply/apply.go", "repo-shape", "repository type"},
		{"internal/apply/apply.go", "signals: code=", "type evidence: CODE score="},
		{"internal/apply/apply.go", "existing-artifacts", "existing files"},
		{"internal/apply/apply.go", "overwrite-risk", "risk of replacing content"},
		{"internal/apply/apply.go", "existing governance files detected; apply will overwrite them", "existing governance files detected; Govna will report whether each file is written, merged, or preserved"},
		{"internal/apply/apply.go", "canon zone merged, existing tail preserved", "updated Govna-managed section; kept repository-owned section"},
		{"internal/apply/apply.go", "existing content preserved — manual boundary migration required", "kept existing file; add the missing Govna/local boundary and merge the Govna-managed section manually"},
		{"internal/apply/apply.go", "written — no boundary found, blind overwrite", "replaced whole file because the Govna/local boundary was missing"},
		{"internal/apply/apply.go", "existing content preserved for manual migration", "kept the existing file; add the named boundary and merge the Govna-managed section manually"},
		{"internal/apply/apply.go", "overwriting whole file", "replacing the whole file because the named Govna/local boundary is missing"},
		{"internal/apply/apply.go", "Govna Apply", "Review Files Added by Govna"},
		{"internal/apply/apply.go", "applied embedded canon", "added its embedded governance files"},
		{"internal/apply/apply.go", "Every file listed below is consumer-owned", "The list below records whether each file was written, merged, or preserved."},
		{"cmd/govna/main.go", "Repo governance templates", "Add and maintain Govna governance files"},
		{"cmd/govna/main.go", "apply governance template to a repo", "add Govna governance files to a repository"},
		{"cmd/govna/main.go", "drift scan an adopted repo against govna canon", "check a repository with Govna for updates and local changes"},
		{"cmd/govna/main.go", "emit cleanup AC for removing govna canon", "write a reviewable AC for removing Govna files"},
		{"cmd/govna/main.go", "render flavor-specific canon files into a target directory", "write the selected built-in Govna files to a directory"},
		{"cmd/govna/main.go", "print binary and embedded canon versions", "print executable and embedded governance-file versions"},
		{"cmd/govna/main.go", "print binary version", "print executable version"},
		{"cmd/govna/main.go", "govna binary:", "Govna executable version:"},
		{"cmd/govna/main.go", "embedded canon:", "Embedded governance-file version (canon version):"},
		{"cmd/govna/main.go", "Scan an adopted-govna repo against canon.", "Compare a repository's Govna files with the files built into this executable."},
		{"cmd/govna/main.go", "Emits an AC stub under govna/.", "Writes a reviewable"},
		{"cmd/govna/main.go", "Apply governance template to the current directory", "Add Govna governance files to the current directory."},
		{"cmd/govna/main.go", "Detects repo state, resolves missing parameters", "Govna identifies the"},
		{"cmd/govna/main.go", "Render canon files into <target>/", "Write the selected built-in Govna files to <target>/"},
		{"internal/remove/remove.go", "Emit a Director-reviewed cleanup AC", "Write an AC that lists which Govna files can be removed"},
		{"internal/audit/audit.go", "clean (%s); no AC emitted", "No Govna updates or Director choices found"},
		{"internal/audit/audit.go", "wrote %s (%s)", "Wrote %s for review"},
		{"internal/audit/audit.go", "byte-equal with embedded canon", "matches the embedded Govna file"},
		{"internal/audit/audit.go", "target missing; compare embedded canon", "repository is missing"},
		{"internal/audit/audit.go", "inspect target-only file", "because it is not in the selected embedded Govna files"},
		{"internal/audit/audit.go", "compare embedded canon with target", "compare the embedded Govna file with the repository file"},
		{"internal/repository/repository.go", "conflicting flavor signals", "Govna found both CODE and DOC evidence"},
		{"internal/repository/repository.go", "could not infer flavor", "Govna could not determine whether this is a CODE or DOC repository"},
		{"internal/repository/repository.go", "repository has no govna adoption signal", "Govna could not find the files that confirm Govna was added to this repository"},
		{"internal/render/render.go", "conflicting flavor signals", "Govna found both CODE and DOC evidence"},
		{"internal/render/render.go", "could not infer flavor", "Govna could not determine whether this is a CODE or DOC repository"},
		{"internal/audit/audit.go", "canon-coherence precondition failed", "embedded Govna files disagree with each other"},
		{"internal/emission/emission.go", "multiple emitted AC stubs", "Govna found more than one generated"},
		{"internal/emission/emission.go", "audit: multiple matching audit stubs", "Rename extra files so only one matches before retrying"},
		{"internal/canon/canon.go", "compose stack build/release guidance", "Govna could not load the Rust build and release guidance"},
		{"internal/canon/canon.go", "compose stack guidelines: ## Project Practices boundary not found", "Govna could not add stack guidance because the ## Project Practices boundary is missing"},
		{"internal/canon/canon.go", "render baseline:", "Govna could not build the baseline for"},
		{"arch.md", "strict durable-state parsing", "rejects malformed saved Govna state before comparing files"},
		{"arch.md", "bounded target-only evidence", "reviews extra repository files only when specific Govna evidence identifies them"},
		{"arch.md", "guarded stem/version-keyed stub reuse with body-hash edit detection", "reuses an unedited AC for the same canon version"},
		{"arch.md", "dual-axis and legacy-upgrade behavior", "records executable and canon versions separately and upgrades unedited legacy markers"},
		{"govna/operator-contract-rationale.md", "deterministic fallout", "one directly broken supporting file with only one valid correction"},
		{"govna/operator-contract-rationale.md", "Evidence-triggered reporting distinguishes contract defects", "A contract-integrity finding reports a proven governance-rule problem rather than an implementation bug."},
		{"govna/operator-contract-rationale.md", "Atomicity reduces dropped qualifiers", "A contract-growth review checks whether new rules duplicate, hide, misplace, or crowd out existing rules."},
		{"govna/operator-contract-rationale.md", "the three-round limit returns repeated or decision-bearing churn", "The Operator may correct at most three missed paths or instructions before asking the Director again."},
	}
	root := filepath.Join("..", "..")
	for _, check := range checks {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(check.path)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), check.legacy) {
			t.Errorf("%s retains scoped legacy wording %q", check.path, check.legacy)
		}
		if !strings.Contains(string(content), check.current) {
			t.Errorf("%s omits scoped replacement %q", check.path, check.current)
		}
	}
	if !strings.Contains(readSourceForLanguageTest(t, "internal/audit/audit.go"), "validationDisposition") {
		t.Error("path-specific checks must permit the unchanged internal validationDisposition identifier")
	}
}

func TestManualACTitleAndSummaryScenario(t *testing.T) {
	legacy := "gate audit validation on stack manifest reachability"
	want := "avoid invalid repository checks when required project files are missing"
	if legacy == want || !strings.HasPrefix(want, "avoid invalid repository checks") {
		t.Fatalf("manual AC title scenario was not replaced: %q", want)
	}
	content := readSourceForLanguageTest(t, "govna/ac-template.md")
	for _, requirement := range []string{
		"Use a kebab-case slug and a `# AC<N> Title` heading that names the concrete outcome.",
		"Lead with the concrete outcome in one short paragraph.",
		"Explain each necessary Govna label before relying on it.",
	} {
		if !strings.Contains(content, requirement) {
			t.Errorf("manual AC gate omits %q", requirement)
		}
	}
}

func readSourceForLanguageTest(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestGeneratedInstructionExtractionRejectsHiddenImperative(t *testing.T) {
	body := "## Summary\n\nEvery file listed below is consumer-owned. Delete unrelated content.\n\n### Routing Decisions\n\n1. **`local.md`**: Which outcome applies: sync or preserve?\n\n### Removal Instructions\n\n- Apply each in-scope removal.\n\n## Status\n\n`PENDING` — removal emission; awaiting explicit Director Audit.\n"
	instructions := extractGeneratedInstructions(body)
	if len(instructions) != 2 {
		t.Fatalf("extracted instructions=%v, want hidden and section instructions", instructions)
	}
	valid, invalid := 0, 0
	for _, instruction := range instructions {
		if err := validateGeneratedInstruction(instruction); err != nil {
			invalid++
		} else {
			valid++
		}
	}
	if valid != 1 || invalid != 1 {
		t.Fatalf("hidden imperative disposition valid=%d invalid=%d instructions=%v", valid, invalid, instructions)
	}
}

func validateGeneratedInstruction(instruction string) error {
	normalized := normalizeGeneratedInstruction(instruction)
	semantic := strings.ReplaceAll(strings.TrimSpace(normalized), "`", "")
	if !generatedImperativeStarter(semantic) {
		return fmt.Errorf("generated instruction lacks an allowed imperative starter: %s", instruction)
	}
	if generatedSecondAction.MatchString(semantic) {
		return fmt.Errorf("generated instruction contains a second action: %s", instruction)
	}
	for _, expected := range generatedInstructionManifest {
		if normalized == expected.text {
			return nil
		}
	}
	return fmt.Errorf("generated instruction is outside the settled manifest: %s", instruction)
}

func generatedImperativeStarter(instruction string) bool {
	starter := firstInstructionWord(instruction)
	for _, expected := range generatedInstructionManifest {
		if starter == firstInstructionWord(expected.text) {
			return true
		}
	}
	return false
}

func firstInstructionWord(instruction string) string {
	fields := strings.Fields(strings.TrimSpace(instruction))
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], "`*_.,:;!?")
}

func normalizeGeneratedInstruction(instruction string) string {
	trimmed := strings.TrimSpace(instruction)
	normalized := strings.ReplaceAll(trimmed, "`", "")
	switch {
	case normalized == "Write govna/canon-baseline.txt from the final temporary render only after all other work is complete and the repository check succeeds.":
		return normalized
	case normalized == "Create govna/metadata.txt from the selected temporary render before govna/canon-baseline.txt installation.":
		return normalized
	case legacyPreserveInstruction.MatchString(trimmed):
		return "Remove the legacy preserve phrase for <path> only after the file update and preserve-list state are verified."
	case protectedRegionInstruction.MatchString(trimmed):
		return "Preserve the protected region in <path> from <boundary> through EOF with SHA-256 <hash> for any update choice."
	case inferredCheckInstruction.MatchString(trimmed):
		return "Run <command> after the selected file updates and before govna/canon-baseline.txt installation (<evidence>)."
	case notApplicableInstruction.MatchString(trimmed):
		return "Verify the chosen repository check succeeds before govna/canon-baseline.txt installation."
	case renderRemovalInstruction.MatchString(trimmed):
		return "Create a temporary copy of the selected Govna files with <render-command>."
	case compareRemovalInstruction.MatchString(trimmed):
		return "Compare <path> with <diff-command>."
	case chooseRemovalInstruction.MatchString(trimmed):
		return "Choose what to remove from <path>: only its Govna-managed section, nothing, or the whole file."
	case otherMigrationInstruction.MatchString(trimmed):
		return "Create <path> from the selected temporary render."
	default:
		return normalized
	}
}

func extractGeneratedInstructions(content string) []string {
	section := ""
	var instructions []string
	for line := range strings.SplitSeq(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") || strings.HasPrefix(trimmed, "### ") {
			section = trimmed
			continue
		}
		if strings.HasPrefix(trimmed, "**AT") {
			if _, instruction, ok := strings.Cut(trimmed, " — "); ok {
				instructions = append(instructions, instruction)
			}
			continue
		}
		if after, ok := strings.CutPrefix(trimmed, "- "); ok {
			instruction := after
			if instruction == "None." {
				continue
			}
			if section == "### Adoption Instructions" || section == "### Removal Instructions" || section == "## Migration findings" || section == "### Routing Decisions" && strings.HasPrefix(line, "   - ") {
				instructions = append(instructions, instruction)
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, "`") || trimmed[0] >= '0' && trimmed[0] <= '9' {
			continue
		}
		for _, sentence := range splitGeneratedSentences(trimmed) {
			if generatedProseStarters[firstInstructionWord(sentence)] {
				instructions = append(instructions, sentence)
			}
		}
	}
	return instructions
}

func splitGeneratedSentences(line string) []string {
	replacer := strings.NewReplacer(". ", ".\n", "? ", "?\n", "! ", "!\n")
	var sentences []string
	for sentence := range strings.SplitSeq(replacer.Replace(line), "\n") {
		if trimmed := strings.TrimSpace(sentence); trimmed != "" {
			sentences = append(sentences, trimmed)
		}
	}
	return sentences
}

func generatedManifestByText(t *testing.T) map[string]string {
	t.Helper()
	manifest := make(map[string]string, len(generatedInstructionManifest))
	for _, instruction := range generatedInstructionManifest {
		if _, exists := manifest[instruction.text]; exists {
			t.Fatalf("duplicate generated instruction: %s", instruction.text)
		}
		manifest[instruction.text] = instruction.id
	}
	return manifest
}

func equalInstructionCounts(got, want map[string]int) bool {
	if len(got) != len(want) {
		return false
	}
	for id, count := range want {
		if got[id] != count {
			return false
		}
	}
	return true
}

func TestInstructionStyleAtomicityGate(t *testing.T) {
	exceptions := make(map[string]instructionException, len(instructionExceptions))
	for _, exception := range instructionExceptions {
		if exception.path == "" || exception.text == "" || exception.reason == "" {
			t.Fatalf("incomplete instruction exception: %+v", exception)
		}
		key := exception.path + "\x00" + exception.text
		if prior, exists := exceptions[key]; exists {
			t.Fatalf("duplicate instruction exception for %s: %q and %q", exception.path, prior.reason, exception.reason)
		}
		exceptions[key] = exception
	}

	used := map[string]bool{}
	for path, content := range governanceCorpus(t) {
		for line := range strings.SplitSeq(content, "\n") {
			instruction, ok := ruleInstruction(line)
			if !ok || !secondAction.MatchString(instruction) {
				continue
			}
			key := path + "\x00" + instruction
			if _, allowed := exceptions[key]; !allowed {
				t.Errorf("%s: compound-action candidate requires a split or exact exception: %s", path, instruction)
				continue
			}
			used[key] = true
		}
	}
	for key, exception := range exceptions {
		if !used[key] {
			t.Errorf("%s: stale instruction exception: %s", exception.path, exception.text)
		}
	}
}

func TestCompoundActionDetection(t *testing.T) {
	for _, text := range []string{
		"Place the count paragraph first and start it with the required text.",
		"Reserve validation evidence and refresh it for Rust prep.",
		"Run ordinary canonical pre- and post-change validation.",
	} {
		if !secondAction.MatchString(text) {
			t.Errorf("did not reject compound action: %s", text)
		}
	}
	for _, text := range []string{
		"Reject missing, malformed, duplicate, or unsafe targets.",
		"Use `--dry-run` or `-n` to inspect without writes.",
		"Start the sentences with A, B, and C in that order.",
	} {
		if secondAction.MatchString(text) {
			t.Errorf("rejected compound object, alias, or result: %s", text)
		}
	}
}

func TestAtomicInstructionCorrections(t *testing.T) {
	want := map[string][]string{
		"govna/audit.md": {
			"- Place the count paragraph first under `## Summary`.",
			"- Start the count paragraph with `Govna found`.",
			"- Recompute the protected-region digest after adoption.",
			"- Require the protected-region digest to match the emitted digest.",
		},
		"govna/build-release.md": {
			"- Run ordinary canonical pre-change validation for Go prep.",
			"- Run ordinary canonical post-change validation for Go prep.",
			"- Reserve validation-token evidence for Rust prep.",
			"- Refresh validation-token evidence for Rust prep.",
			"- Rebuild a release from its clean tagged commit before publication.",
			"- Install the rebuilt release before publication.",
		},
		"govna/code-stacks.md": {
			"- Bump the root package version during release prep.",
			"- Refresh `Cargo.lock` during release prep.",
			"- Build only selected executable products during scoped builds.",
			"- Install only selected executable products during scoped builds.",
		},
		"govna/development-guidelines.md": {
			"- Verify old data compatibility with new schemas.",
			"- Write migration logic for old data.",
			"- Fail explicitly when migration logic is unavailable.",
		},
		"govna/roles.md": {
			"- Run each acceptance test in the active AC when it can be exercised.",
			"- Report the result of each exercised acceptance test.",
			"- State explicitly why each unexercised acceptance test was only reasoned about.",
		},
		"AGENTS.md": {
			"- Name the exact sections to change during every update.",
			"- Keep edits local during every update.",
			"- Stop when a request lacks authorization, scope, or required context.",
			"- Ask for the missing authorization, scope, or context.",
		},
	}
	corpus := governanceCorpus(t)
	for path, instructions := range want {
		content, ok := corpus[path]
		if !ok {
			t.Fatalf("governance corpus omits %s", path)
		}
		for _, instruction := range instructions {
			if !strings.Contains(content, instruction+"\n") {
				t.Errorf("%s omits atomic instruction %q", path, instruction)
			}
		}
	}
}

func TestRewrittenInstructionManifest(t *testing.T) {
	if len(rewrittenInstructionReviews) != 45 {
		t.Fatalf("rewritten instruction manifest has %d entries, want 45", len(rewrittenInstructionReviews))
	}
	if len(rewrittenInstructionPaths) != 16 {
		t.Fatalf("rewritten instruction path inventory has %d entries, want 16", len(rewrittenInstructionPaths))
	}

	expectedPaths := make(map[string]bool, len(rewrittenInstructionPaths))
	for _, path := range rewrittenInstructionPaths {
		if path == "" {
			t.Fatal("rewritten instruction path inventory contains an empty path")
		}
		if expectedPaths[path] {
			t.Fatalf("rewritten instruction path inventory duplicates %s", path)
		}
		expectedPaths[path] = true
	}

	corpus := governanceCorpus(t)
	coveredPaths := map[string]bool{}
	beforeSeen := map[string]bool{}
	afterSeen := map[string]bool{}
	currentReplacementsSeen := map[string]bool{}
	corrected := 0
	for index, review := range rewrittenInstructionReviews {
		if review.v032 == "" || review.v033 == "" || review.disposition == "" || len(review.paths) == 0 {
			t.Fatalf("incomplete rewritten instruction review at index %d: %+v", index, review)
		}
		if beforeSeen[review.v032] {
			t.Fatalf("duplicate v0.32 instruction review: %s", review.v032)
		}
		beforeSeen[review.v032] = true
		if afterSeen[review.v033] {
			t.Fatalf("duplicate v0.33 instruction review: %s", review.v033)
		}
		afterSeen[review.v033] = true
		if !focusedImperativeStarter(review.v033) {
			t.Errorf("v0.33 instruction lacks a focused imperative starter: %s", review.v033)
		}
		current := currentReviewedInstruction(review)
		if !focusedImperativeStarter(current) {
			t.Errorf("current instruction lacks a focused imperative starter: %s", current)
		}
		if current != review.v033 {
			currentReplacementsSeen[review.v033] = true
		}

		switch {
		case review.v032 == review.v033 && review.disposition != reviewClean:
			t.Errorf("unchanged instruction has disposition %q: %s", review.disposition, review.v032)
		case review.v032 != review.v033 && review.disposition != reviewCorrectedStarter:
			t.Errorf("corrected instruction has disposition %q: %s", review.disposition, review.v032)
		case review.v032 != review.v033:
			corrected++
		}

		recordPaths := map[string]bool{}
		for _, path := range review.paths {
			if recordPaths[path] {
				t.Errorf("instruction review duplicates path %s: %s", path, review.v032)
				continue
			}
			recordPaths[path] = true
			if !expectedPaths[path] {
				t.Errorf("instruction review uses unexpected path %s: %s", path, review.v032)
				continue
			}
			coveredPaths[path] = true
			content, ok := corpus[path]
			if !ok {
				t.Errorf("governance corpus omits reviewed path %s", path)
				continue
			}
			if countRuleInstruction(content, current) != 1 {
				t.Errorf("%s does not contain exactly one current instruction: %s", path, current)
			}
			if review.v032 != review.v033 && countRuleInstruction(content, review.v032) != 0 {
				t.Errorf("%s unexpectedly retains replaced instruction: %s", path, review.v032)
			}
			if current != review.v033 && countRuleInstruction(content, review.v033) != 0 {
				t.Errorf("%s unexpectedly treats historical v0.33 text as current: %s", path, review.v033)
			}
		}
	}
	if corrected != 1 {
		t.Errorf("rewritten instruction manifest has %d corrected dispositions, want 1", corrected)
	}
	for path := range expectedPaths {
		if !coveredPaths[path] {
			t.Errorf("rewritten instruction manifest does not cover %s", path)
		}
		if _, ok := corpus[path]; !ok {
			t.Errorf("governance corpus omits expected path %s", path)
		}
	}
	for historical := range currentInstructionReplacements {
		if !currentReplacementsSeen[historical] {
			t.Errorf("current instruction replacement is not tied to v0.33 history: %s", historical)
		}
	}
}

func TestFocusedInstructionStarter(t *testing.T) {
	for _, text := range []string{
		"Never assume old data fits new schemas.",
		"Always run the canonical build.",
		"When changing a file, update its references.",
		"",
	} {
		if focusedImperativeStarter(text) {
			t.Errorf("accepted invalid instruction starter: %q", text)
		}
	}
	for _, text := range []string{
		"Verify old data compatibility with new schemas.",
		"Re-read AGENTS.md before reporting completion.",
		"Do not publish without authorization.",
	} {
		if !focusedImperativeStarter(text) {
			t.Errorf("rejected valid instruction starter: %q", text)
		}
	}
}

func TestAffectedInstructionSectionEnvelopes(t *testing.T) {
	corpus := governanceCorpus(t)
	for _, path := range rolesRewrittenPaths {
		content := corpus[path]
		if !strings.Contains(content, "### Required Self-review\n\n- Re-read `AGENTS.md` and the active AC before reporting completion.\n") {
			t.Errorf("%s omits the required self-review section opening", path)
		}
		if strings.Contains(content, "### Self-review (mandatory)") {
			t.Errorf("%s retains the parenthetical self-review heading", path)
		}
	}

	const first = "Start this checklist only when the director explicitly requests standalone `Package`, `package`, `pack`, or `prep` in the active Ratified AC context."
	const second = "Do not treat `./build.sh prep ...` or ordinary build-preparation language as a workflow request."
	codeOpening := "## Pre-Release Checklist\n\n- " + first + "\n- " + second + "\n\nNote: the operator flow has two steps.\n\n1. **Run prep.**\n"
	for _, path := range buildReleaseRewrittenPaths {
		content := corpus[path]
		if !strings.Contains(content, codeOpening) {
			t.Errorf("%s omits the required CODE checklist opening", path)
		}
		if strings.Contains(content, "## Pre-Release Checklist (`Package`, `package`, `pack`, or `prep`)") ||
			strings.Contains(content, "Do not start this checklist unless") ||
			strings.Contains(content, "The operator flow is two steps:") {
			t.Errorf("%s retains an obsolete checklist envelope", path)
		}
	}

	const docPath = "internal/canon/assets/overlays/doc/files/govna/release.md.tmpl"
	docOpening := "## Pre-Release Checklist\n\n- " + first + "\n- " + second + "\n\n1. **Verify completion.**\n"
	doc := corpus[docPath]
	if !strings.Contains(doc, docOpening) {
		t.Errorf("%s omits the required DOC checklist opening", docPath)
	}
	if strings.Contains(doc, "Note: the operator flow has two steps.") ||
		strings.Contains(doc, "## Pre-Release Checklist (`Package`, `package`, `pack`, or `prep`)") ||
		strings.Contains(doc, "Do not start this checklist unless") {
		t.Errorf("%s retains an obsolete or CODE-only checklist envelope", docPath)
	}
}

func focusedImperativeStarter(instruction string) bool {
	fields := strings.Fields(strings.TrimSpace(instruction))
	if len(fields) == 0 {
		return false
	}
	starter := strings.Trim(fields[0], "`*_")
	switch starter {
	case "Never", "Always", "When":
		return false
	default:
		return true
	}
}

func countRuleInstruction(content, instruction string) int {
	want := "- " + instruction
	count := 0
	for line := range strings.SplitSeq(content, "\n") {
		if strings.TrimSpace(line) == want {
			count++
		}
	}
	return count
}

func governanceCorpus(t *testing.T) map[string]string {
	t.Helper()
	corpus := map[string]string{}
	root := filepath.Join("..", "..")
	paths, err := filepath.Glob(filepath.Join(root, "govna", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		name := filepath.Base(path)
		if acDocument.MatchString(name) {
			continue
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		corpus[filepath.ToSlash(filepath.Join("govna", name))] = string(content)
	}
	if err := fs.WalkDir(assets, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".md.tmpl") {
			return nil
		}
		content, err := fs.ReadFile(assets, path)
		if err != nil {
			return err
		}
		corpus["internal/canon/"+path] = string(content)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	corpus["AGENTS.md"] = string(agents)
	return corpus
}

func ruleInstruction(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "- ") {
		return "", false
	}
	return strings.TrimPrefix(trimmed, "- "), true
}

func TestGovernanceCorpusOrder(t *testing.T) {
	paths := make([]string, 0, len(governanceCorpus(t)))
	for path := range governanceCorpus(t) {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 || paths[0] != "AGENTS.md" {
		t.Fatalf("unexpected governance corpus: %v", paths)
	}
}
