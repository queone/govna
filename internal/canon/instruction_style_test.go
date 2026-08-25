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
	{"AGENTS.md", "Install or replace `govna/canon-baseline.txt` from the scratch render only after every other applicable acceptance test, routing outcome, and validation disposition passes.", "exclusive operations apply to one target state"},
	{"AGENTS.md", "Reach for `Read` only to fetch unseen content or check for recent changes.", "one reach action has two exclusive purposes"},
	{"govna/development-cycle.md", "Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.", "one keep action applies to two object classes"},
	{"internal/canon/assets/overlays/code/files/govna/development-cycle.md.tmpl", "Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.", "one keep action applies to two object classes"},
	{"internal/canon/assets/overlays/doc/files/govna/editing-cycle.md.tmpl", "Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.", "one keep action applies to two object classes"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Ask the Director to narrow the task or split the AC before proposing delegation when the task exceeds practical inline capacity.", "one ask action offers two exclusive request objects"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Map every in-scope command entry point, provider/API fetch, normalized-table write, durable snapshot, stale fallback, freshness gate, and complete-snapshot reconciliation path in the closure audit.", "one map action applies to a path-category list"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Install or replace `govna/canon-baseline.txt` from the scratch render only after every other applicable acceptance test, routing outcome, and validation disposition passes.", "exclusive operations apply to one target state"},
	{"internal/canon/assets/base/AGENTS.md.tmpl", "Reach for `Read` only to fetch unseen content or check for recent changes.", "one reach action has two exclusive purposes"},
	{"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl", "Ask the Director to narrow the task or split the AC before proposing delegation when the task exceeds practical inline capacity.", "one ask action offers two exclusive request objects"},
	{"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl", "Install or replace `govna/canon-baseline.txt` from the scratch render only after every other applicable acceptance test, routing outcome, and validation disposition passes.", "exclusive operations apply to one target state"},
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

type generatedInstructionTemplate struct {
	id   string
	text string
}

var generatedInstructionManifest = []generatedInstructionTemplate{
	{"I01", "Verify AGENTS.md reflects the repository's actual practices."},
	{"I02", "Verify govna/roles.md reflects the repository's delivery model (Operator + Director)."},
	{"I03", "Verify CLAUDE.md is a symlink to AGENTS.md."},
	{"I04", "Verify CLAUDE.md remains the existing regular file instead of a symlink to AGENTS.md."},
	{"I05", "Resolve every routing decision in chat."},
	{"I06", "Leave this emitted stub unchanged."},
	{"I07", "Render canon into a scratch directory."},
	{"I08", "Verify every direct-sync and canon-backed migration path exists in the selected CODE stack scratch render as a precondition."},
	{"I09", "Apply every resolved outcome within the authorized content boundaries."},
	{"I10", "Install govna/canon-baseline.txt last."},
	{"I11", "Remove the legacy preserve phrase for <path> only after the required sync and registry state are verified."},
	{"I12", "Create govna/canon-baseline.txt from the final scratch render only after all other work and validation pass."},
	{"I13", "Create govna/metadata.txt from the selected scratch render before govna/canon-baseline.txt installation."},
	{"I14", "Create <path> from the selected scratch render."},
	{"I15", "Verify every resolved sync target except govna/canon-baseline.txt against rendered canon and every resolved preserve target against govna/preserve.txt."},
	{"I16", "Preserve the protected region in <path> from <boundary> through EOF with SHA-256 <hash> for any sync outcome."},
	{"I17", "Satisfy validation disposition <validation> after selected work and before baseline installation."},
	{"I18", "Verify the final adoption step installed govna/canon-baseline.txt from the same scratch render."},
	{"I19", "Render the selected canon into <scratch> with <render-command>."},
	{"I20", "Preserve every routing-pending path until its route is resolved."},
	{"I21", "Apply each in-scope route and each Director-resolved review route."},
	{"I22", "Apply each in-scope route."},
	{"I23", "Compare <path> with <diff-command>."},
	{"I24", "Choose one route for <path>: canon-only deletion, full preservation, or full deletion."},
	{"I25", "Verify every resolved removal target under ## In Scope is absent."},
	{"I26", "Verify every routing-pending path matches its Director-resolved route."},
	{"I27", "Verify every preserve-registry decision is applied before the final removal of govna/preserve.txt."},
	{"I28", "Resolve the validation disposition in chat."},
	{"I29", "Satisfy the resolved validation disposition after selected work and before baseline installation."},
}

var generatedSecondAction = regexp.MustCompile(`(?i)(?:\b(?:and|or|then)\s+(?:also\s+)?|;\s*|[.!?]\s+)(?:apply|choose|compare|create|install|leave|preserve|remove|render|resolve|satisfy|verify)\b|\b(?:before|after)\s+(?:apply|choose|compare|create|install|leave|preserve|remove|render|resolve|satisfy|verify|applying|choosing|comparing|creating|installing|leaving|preserving|removing|rendering|resolving|satisfying|verifying)\b`)

var (
	legacyPreserveInstruction  = regexp.MustCompile("^Remove the legacy preserve phrase for `[^`]+` only after the required sync and registry state are verified\\.$")
	protectedRegionInstruction = regexp.MustCompile("^Preserve the protected region in `[^`]+` from `[^`]+` through EOF with SHA-256 `[^`]+` for any sync outcome\\.$")
	renderRemovalInstruction   = regexp.MustCompile("^Render the selected canon into `<scratch>` with `govna render --flavor (?:doc|code(?: --stack [^`]+)?) <scratch>`\\.$")
	compareRemovalInstruction  = regexp.MustCompile("^Compare `[^`]+` with `diff -ru <scratch>/[^`]+ [^`]+`\\.$")
	chooseRemovalInstruction   = regexp.MustCompile("^Choose one route for `[^`]+`: canon-only deletion, full preservation, or full deletion\\.$")
	otherMigrationInstruction  = regexp.MustCompile("^Create `[^`]+` from the selected scratch render\\.$")
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
			"executable version separately from the embedded canon version",
			"stub filenames remain keyed by canon version",
			"legacy canon-only marker upgrades in place",
			"semantic instruction gate",
		},
		"arch.md": {
			"executable and embedded-canon versions separately",
			"canon-version-keyed AC stub",
			"legacy canon-only marker upgrades at the same path and AC number",
			"same dual-axis and legacy-upgrade behavior",
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

func TestGeneratedInstructionExtractionRejectsHiddenImperative(t *testing.T) {
	body := "## Summary\n\nEvery file listed below is consumer-owned. Delete unrelated content.\n\n### Routing Decisions\n\n1. **`local.md`**: Which outcome applies: sync or preserve?\n\n### Removal Instructions\n\n- Apply each in-scope route.\n\n## Status\n\n`PENDING` — removal emission; awaiting explicit Director Audit.\n"
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
	case normalized == "Create govna/canon-baseline.txt from the final scratch render only after all other work and validation pass.":
		return normalized
	case normalized == "Create govna/metadata.txt from the selected scratch render before govna/canon-baseline.txt installation.":
		return normalized
	case legacyPreserveInstruction.MatchString(trimmed):
		return "Remove the legacy preserve phrase for <path> only after the required sync and registry state are verified."
	case protectedRegionInstruction.MatchString(trimmed):
		return "Preserve the protected region in <path> from <boundary> through EOF with SHA-256 <hash> for any sync outcome."
	case strings.HasPrefix(normalized, "Satisfy validation disposition ") && strings.HasSuffix(normalized, " after selected work and before baseline installation."):
		return "Satisfy validation disposition <validation> after selected work and before baseline installation."
	case renderRemovalInstruction.MatchString(trimmed):
		return "Render the selected canon into <scratch> with <render-command>."
	case compareRemovalInstruction.MatchString(trimmed):
		return "Compare <path> with <diff-command>."
	case chooseRemovalInstruction.MatchString(trimmed):
		return "Choose one route for <path>: canon-only deletion, full preservation, or full deletion."
	case otherMigrationInstruction.MatchString(trimmed):
		return "Create <path> from the selected scratch render."
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
			"- Start the count paragraph with `This adoption covers`.",
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
			if countRuleInstruction(content, review.v033) != 1 {
				t.Errorf("%s does not contain exactly one expected instruction: %s", path, review.v033)
			}
			if review.v032 != review.v033 && countRuleInstruction(content, review.v032) != 0 {
				t.Errorf("%s unexpectedly retains replaced instruction: %s", path, review.v032)
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
