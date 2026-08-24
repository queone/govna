package canon

import (
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
