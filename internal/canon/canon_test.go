package canon

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const (
	efficiencyPurposeDefinition = "Govna exists to make programming and publishing ceremonies more effective and efficient. Those ceremonies are the recurring CODE and DOC checkpoints around intent, authorization, scope, review, implementation or editing, verification, and release."
	efficiencyGateBoundary      = "Efficiency does not weaken authorization, review, verification, or release gates."
)

func TestRenderVariants(t *testing.T) {
	for _, stack := range Stacks() {
		t.Run(stack, func(t *testing.T) {
			module := ""
			if stack == "Go" {
				module = "example.com/widget"
			}
			first, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: stack, ModulePath: module})
			if err != nil {
				t.Fatal(err)
			}
			second, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: strings.ToLower(stack), ModulePath: module})
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(first) != fmt.Sprint(second) {
				t.Fatal("render is not deterministic")
			}
			assertFiles(t, first, "CODE", stack)
		})
	}
	doc, err := Render(Config{Flavor: Doc, RepoName: "handbook"})
	if err != nil {
		t.Fatal(err)
	}
	assertFiles(t, doc, "DOC", "")
}

func TestEfficiencyPurposeRenders(t *testing.T) {
	variants := []struct {
		name         string
		config       Config
		requirements map[string]string
	}{
		{
			name:   "DOC",
			config: Config{Flavor: Doc, RepoName: "handbook"},
			requirements: map[string]string{
				"README.md":              "Govna makes recurring publishing ceremonies more effective and efficient",
				"govna/README.md":        "the contract makes publishing ceremonies more effective and efficient",
				"govna/ac-template.md":   "An AC records settled intent, scope, and proof once so later review, editing, and verification can reuse the same context instead of reconstructing it.",
				"govna/editing-cycle.md": "The lifecycle makes recurring publishing checkpoints and their settled context reusable across phases and sessions.",
			},
		},
	}
	for _, stack := range Stacks() {
		modulePath := ""
		if stack == "Go" {
			modulePath = "example.com/widget"
		}
		variants = append(variants, struct {
			name         string
			config       Config
			requirements map[string]string
		}{
			name:   stack,
			config: Config{Flavor: Code, RepoName: "widget", Stack: stack, ModulePath: modulePath},
			requirements: map[string]string{
				"README.md":                  "Govna makes recurring programming ceremonies more effective and efficient",
				"govna/README.md":            "the contract makes programming and publishing ceremonies more effective and efficient",
				"govna/ac-template.md":       "An AC records settled intent, scope, and proof once so later review, implementation, and verification can reuse the same context instead of reconstructing it.",
				"govna/development-cycle.md": "The lifecycle makes recurring programming checkpoints and their settled context reusable across phases and sessions.",
			},
		})
	}

	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			files, err := Render(variant.config)
			if err != nil {
				t.Fatal(err)
			}
			rationale := repoFile(t, "arch.md")
			if count := strings.Count(rationale, efficiencyPurposeDefinition); count != 1 {
				t.Errorf("contract purpose definition count=%d, want 1", count)
			}
			if count := strings.Count(rationale, efficiencyGateBoundary); count != 1 {
				t.Errorf("efficiency gate boundary count=%d, want 1", count)
			}
			for path, required := range variant.requirements {
				if content := fileText(t, files, path); !strings.Contains(content, required) {
					t.Errorf("%s omits purpose reminder %q", path, required)
				}
			}
		})
	}
}

func assertFiles(t *testing.T, files []File, flavor, stack string) {
	t.Helper()
	previous := ""
	foundBaseline := false
	for _, file := range files {
		if previous >= file.Path {
			t.Fatalf("paths not sorted: %s then %s", previous, file.Path)
		}
		previous = file.Path
		text := string(file.Content)
		if file.Path == "govna/canon-baseline.txt" {
			foundBaseline = true
			if !strings.HasPrefix(text, "govna-canon-baseline-v1\ncanon_version = "+"v"+Version+"\n") {
				t.Fatalf("bad baseline: %s", text)
			}
			if strings.Contains(text, "govna/canon-baseline.txt\t") {
				t.Fatal("baseline hashes itself")
			}
		}
		if strings.HasPrefix(file.Path, "govna/") || file.Path == "AGENTS.md" {
			for _, placeholder := range []string{"{{REPO_NAME}}", "{{STACK_OR_PLATFORM}}", "{{MODULE_PATH}}", "{{CANON_VERSION}}", "{{CODE_STACK}}", "{{STACK_BUILD_RELEASE_GUIDANCE}}"} {
				if strings.Contains(text, placeholder) {
					t.Fatalf("%s retains %s", file.Path, placeholder)
				}
			}
		}
	}
	if !foundBaseline {
		t.Fatal("baseline missing")
	}
	metadata := fileText(t, files, "govna/metadata.txt")
	if !strings.Contains(metadata, "repo_type = "+flavor+"\n") {
		t.Fatalf("metadata: %s", metadata)
	}
	if stack != "" && !strings.Contains(metadata, "code_stack = "+stack+"\n") {
		t.Fatalf("metadata: %s", metadata)
	}
}

func TestBaselineHashes(t *testing.T) {
	files, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: "Rust"})
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string][]byte{}
	for _, file := range files {
		byPath[file.Path] = file.Content
	}
	for _, line := range strings.Split(fileText(t, files, "govna/canon-baseline.txt"), "\n")[2:] {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatalf("bad baseline line: %q", line)
		}
		region := byPath[fields[0]]
		if after, ok := strings.CutPrefix(fields[1], "before:"); ok {
			boundary := after
			region = []byte(strings.Split(string(region), "\n"+boundary+"\n")[0] + "\n")
		}
		hash := sha256.Sum256(region)
		if fmt.Sprintf("%x", hash) != fields[2] {
			t.Fatalf("hash mismatch: %s", fields[0])
		}
	}
}

func TestSelfHostedGoBaseline(t *testing.T) {
	files, err := Render(Config{Flavor: Code, RepoName: "govna", Stack: "Go", ModulePath: "github.com/queone/govna", Utilities: true})
	if err != nil {
		t.Fatal(err)
	}
	got := fileText(t, files, "govna/canon-baseline.txt")
	want, err := os.ReadFile(filepath.Join("..", "..", "govna", "canon-baseline.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("self-hosted Go baseline differs from render\n%s", got)
	}
}

// TestRenderedIdeaRulesNameTheCeremonyEvaluation checks that the cycle
// document's IE rules point idea entries at the AC-First Workflow section and
// that plan.md names no undefined rubric.
func TestRenderedIdeaRulesNameTheCeremonyEvaluation(t *testing.T) {
	profiles := map[string]Config{"DOC": {Flavor: Doc, RepoName: "handbook"}}
	for _, stack := range Stacks() {
		module := ""
		if stack == "Go" {
			module = "example.com/widget"
		}
		profiles["CODE "+stack] = Config{Flavor: Code, RepoName: "widget", Stack: stack, ModulePath: module}
	}
	for name, config := range profiles {
		t.Run(name, func(t *testing.T) {
			files, err := Render(config)
			if err != nil {
				t.Fatal(err)
			}
			cycle := "govna/development-cycle.md"
			if config.Flavor == Doc {
				cycle = "govna/editing-cycle.md"
			}
			if !strings.Contains(sectionText(t, fileText(t, files, cycle), "## Notes\n"), "`AGENTS.md` `### AC-First Workflow`") {
				t.Errorf("%s ## Notes does not point idea entries at the AC-First Workflow section", cycle)
			}
			plan := fileText(t, files, "plan.md")
			if strings.Contains(strings.ToLower(plan), "rubric") {
				t.Error("plan.md names a rubric that AGENTS.md does not define")
			}
		})
	}
}

func TestCanonicalStack(t *testing.T) {
	for input, want := range map[string]string{"go": "Go", "GOLANG": "Go", "rUsT": "Rust", "swift": "Swift", "terraform": "Terraform"} {
		got, ok := CanonicalStack(input)
		if !ok || got != want {
			t.Fatalf("%s: %s, %v", input, got, ok)
		}
	}
	for _, stack := range []string{"Java", "Node", "Python", "Ruby"} {
		if _, ok := CanonicalStack(stack); ok {
			t.Errorf("accepted unsupported stack %s", stack)
		}
		if _, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: stack}); err == nil || !strings.Contains(err.Error(), SupportedStackChoices) {
			t.Errorf("unsupported stack %s error=%v", stack, err)
		}
	}
}

func TestComparisonRegions(t *testing.T) {
	content := []byte("canon\n\n## Project Rules\r\nlocal\n")
	region, ok := ComparisonRegion("AGENTS.md", content)
	if !ok || string(region) != "canon\n\n" {
		t.Fatalf("region=%q ok=%v", region, ok)
	}
	protected, ok := ProtectedRegion("AGENTS.md", content)
	if !ok || string(protected) != "## Project Rules\r\nlocal\n" {
		t.Fatalf("protected=%q ok=%v", protected, ok)
	}
	if got := strings.Join(Stacks(), ","); got != "Go,Rust,Swift,Terraform" {
		t.Fatalf("stacks=%v", got)
	}
}

func TestRendererErrorsExplainTheMissingRequirement(t *testing.T) {
	if _, err := insertBeforeBoundary("# Guidelines\n", "stack rules"); err == nil || err.Error() != "Govna could not add stack guidance because the ## Project Practices boundary is missing from govna/development-guidelines.md" {
		t.Fatalf("stack-guidance error=%v", err)
	}
	_, err := baseline(map[string][]byte{"AGENTS.md": []byte("# Rules\n")})
	want := "Govna could not build the baseline for AGENTS.md because its required boundary \"## Project Rules\" is missing"
	if err == nil || err.Error() != want {
		t.Fatalf("baseline error=%v", err)
	}
}

func TestAuthorityMirrorsAndBoundarySeeds(t *testing.T) {
	root := filepath.Join("..", "..")
	// This repository is a Go parent without children, so each of its canon
	// documents equals the matching render.
	goFiles, err := Render(Config{Flavor: Code, RepoName: "govna", Stack: "Go", ModulePath: "github.com/queone/govna", Utilities: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, authority := range []string{"govna/ac-template.md", "govna/audit.md", "govna/canon-cycle.md", "govna/development-cycle.md", "govna/README.md", "govna/roles.md"} {
		if fileText(t, goFiles, authority) != repoFile(t, authority) {
			t.Errorf("the Go render of %s does not mirror the repository copy", authority)
		}
	}
	roles, err := os.ReadFile(filepath.Join(root, "govna", "roles.md"))
	if err != nil {
		t.Fatal(err)
	}
	wantDocRoles := strings.Replace(string(roles), "`govna/build-release.md`", "`govna/release.md`", 1)
	docRoles, err := fs.ReadFile(assets, "assets/overlays/doc/files/govna/roles.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if string(docRoles) != wantDocRoles {
		t.Error("DOC roles differ from authority beyond the release-document path")
	}
	audit, err := os.ReadFile(filepath.Join(root, "govna", "audit.md"))
	if err != nil {
		t.Fatal(err)
	}
	docAudit, err := fs.ReadFile(assets, "assets/overlays/doc/files/govna/audit.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if string(docAudit) != docAuditText(t, string(audit)) {
		t.Error("DOC audit differs from authority beyond the CODE-only child repository text")
	}
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := fs.ReadFile(assets, "assets/base/AGENTS.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	wantZone, _, _ := strings.Cut(string(agents), "## Project Rules\n")
	gotParts := strings.Split(string(base), "## Project Rules\n")
	if zone, _, _ := strings.Cut(fileText(t, goFiles, "AGENTS.md"), "## Project Rules\n"); zone != wantZone {
		t.Error("base AGENTS canon zone differs from authority")
	}
	if strings.TrimSpace(gotParts[1]) != "" {
		t.Errorf("unexpected Project Rules seed: %s", gotParts[1])
	}
}

var auditValidationRules = []string{
	"- Classify an existing target-only path named in the preserve registry as `preserve`.",
	"- Keep that preserved target-only path visible in audit and JSON results.",
	"- Omit another routing question for that preserved target-only path.",
	"- Offer only these outcomes for a canon-backed ambiguity: sync, preserve, explicitly named migration, delete.",
	"- Offer only these outcomes for an ordinary `target-has-no-canon` item: preserve, explicitly named migration, delete.",
	"- Install an exact current-canon replacement before retired-source routing.",
	"- Omit restore as a routing outcome.",
	"- Offer conversion to `govna/preserve.txt` or exact-phrase removal for marker-only evidence.",
	"- Fail the audit before emission when a governed path is a symbolic link, a directory, a special file, or unreadable.",
	"- Name the failed path and its recovery action in that failure.",
	"- Leave every existing emitted AC unchanged after that failure.",
	"- Read the Summary from the canonical `| Unreleased | <summary> |` table row, where each `\\|` pair is one escaped pipe.",
	"- Read the Summary from a legacy `## Unreleased` heading section as well.",
	"- Drop a duplicate phrase-and-path pair.",
	"- Ignore a phrase whose path is not a normalized repository-relative path.",
	"- Drop an escaping reference or legacy phrase path without inspecting it.",
	"- Fail the audit with a replace-the-link recovery action when a candidate path is a symbolic link.",
	"- Reject an entry that is absolute, contains a backslash or control character, or has an empty, `.`, or `..` component.",
	"- Direct the consumer to correct the invalid entry before retrying.",
	"- Render canon into a scratch directory using `govna render <scratch>`.",
	"- Apply mixed-content items by hunk-merge.",
	"- Run the chosen repository command after all selected sync, migration, and deletion work.",
	"- Verify each resolved preserve target remains and its exact path occurs in `govna/preserve.txt`.",
}

// auditEmissionRules live in arch.md: they bind the emitted AC shape that
// internal/audit produces, not the consumer-facing audit guide.
var auditEmissionRules = []string{
	"- Require the selected CODE stack's recognized root manifest before inferring `./build.sh`.",
	"- Ignore unrelated manifests, other prose, governance documents, executables, CI files, and flavor defaults.",
	"- Name each emitted adoption AC `# AC<N> Adopt Govna Governance Files v<CANON_VERSION>`.",
	"- Place the repository paragraph first under `## Summary`.",
	"- Start the repository paragraph with `This AC updates`.",
	"- Follow it with `The result label (classification)`.",
	"- Place the count paragraph after the repository paragraph.",
	"- Start the count paragraph with `Govna found`.",
	"- Confirm each file selected for update exists in the selected CODE render.",
	"- Emit the repository-check outcome pre-resolved to the configured command.",
	"- Emit an unresolved repository check as the final numbered routing decision.",
	"- Emit one manual resolution AT for an unresolved repository check.",
	"- Emit one automated verification AT for an unresolved repository check.",
	"- Emit a conditional named-destination check for each offered migration outcome.",
	"- Emit a replacement-before-retired-source check for each replacement-missing route.",
	"- Emit an exact-phrase absence check for each legacy-phrase route.",
	"- Emit an unrelated-Summary preservation check for each legacy-phrase route.",
	"- Emit an outside-Summary preservation check for each legacy-phrase route.",
	"- Keep every emitted routing check atomic.",
	"- Keep emitted AT numbering stable across identical reports.",
	"- End every emitted adoption AC with exact status `` `PENDING` — immutable audit emission; workflow state is tracked in the active session.``",
	"- Capture the SHA-256 digest of each existing mixed-content target from the first byte of its exact registered boundary-heading line through end of file.",
	"- Keep the protected-region digest out of classification, baseline scope, and JSON output.",
	"<N>. **Repository check**: Which command should run after the selected file updates, or what repository evidence shows that no command applies?",
}

// profilePlaceholder matches a placeholder that the render fills with profile
// text or with nothing.
var profilePlaceholder = regexp.MustCompile(`\{\{(CHILD|RUST|UTILITY|CLI)_[A-Z_]+\}\}`)

// docAuditText derives the DOC audit document from the govna/audit.md
// authority. The child repository text is profile text, so a repository
// without children and a DOC repository hold the same audit document.
func docAuditText(t *testing.T, authority string) string {
	t.Helper()
	return authority
}

// repoFile reads one file of this repository.
func repoFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

func TestAuditValidationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	paths := []string{
		"govna/audit.md",
		"internal/canon/assets/overlays/code/files/govna/audit.md.tmpl",
		"internal/canon/assets/overlays/doc/files/govna/audit.md.tmpl",
	}
	var authority []byte
	for _, path := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		want := string(authority)
		if strings.Contains(path, "/doc/") {
			want = docAuditText(t, want)
		}
		if authority == nil {
			authority = content
		} else if profilePlaceholder.ReplaceAllString(string(content), "") != want {
			t.Errorf("%s does not mirror govna/audit.md", path)
		}
		// A rule that specifies the tool lives in govna/canon-spec.md instead.
		for _, required := range auditValidationRules {
			if strings.Count(string(content), required)+strings.Count(repoFile(t, "govna/canon-spec.md"), required) != 1 {
				t.Errorf("%s requires one occurrence of %q", path, required)
			}
		}
		for _, moved := range []string{"### Emitted AC instruction and phase shape", "### Conditional routing verification", "- Emit "} {
			if strings.Contains(string(content), moved) {
				t.Errorf("%s retains audit emission contract text %q", path, moved)
			}
		}
		for _, removed := range []string{
			"Ignore other prose, governance documents, executables, manifests, CI files, and flavor defaults.",
			"existing manual and conditional routing ATs",
			"Validation disposition",
			"validation disposition",
			"This adoption covers",
		} {
			if strings.Contains(string(content), removed) {
				t.Errorf("%s retains superseded claim %q", path, removed)
			}
		}
	}
	arch, err := os.ReadFile(filepath.Join(root, "arch.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range auditEmissionRules {
		if strings.Count(string(arch), required) != 1 {
			t.Errorf("arch.md requires one occurrence of %q", required)
		}
	}
}

func TestPackageCompletionContract(t *testing.T) {
	agentRules := []string{
		"Start every Package completion report with the plain, unbulleted, unindented line `Package complete.`.",
		"Keep `Verified:`, `Red-teamed:`, `Not checked:`, and `Run below to release:` in the Package completion report.",
		"Present the exact drafted release command.",
		"Apply these rules to the Package completion report's self-review in place of roles.md's self-review reporting rules.",
		"Limit each self-review heading to one sentence.",
		"State the semver reason under `Verified:`.",
		"State what changed after the Package evidence under `Red-teamed:`.",
		"State the remaining check that needs a Director action or a new Director authorization, or that none remains, under `Not checked:`.",
	}
	const reportGroupings = "- Present the exact drafted release command.\n\n### Package Self-review\n\n" +
		"- Apply these rules to the Package completion report's self-review in place of roles.md's self-review reporting rules.\n" +
		"- Limit each self-review heading to one sentence.\n" +
		"- State the semver reason under `Verified:`.\n" +
		"- State what changed after the Package evidence under `Red-teamed:`.\n" +
		"- State the remaining check that needs a Director action or a new Director authorization, or that none remains, under `Not checked:`.\n\n### Completed Release\n\n" +
		"- Treat the Director's `shipped` as a completed release.\n" +
		"- Treat the successful end of a `## Project Rules` step that completes a release as a completed release.\n" +
		"- Run no git or gh command to confirm a completed release.\n" +
		"- Acknowledge a completed release in one line.\n" +
		"- Name the next one to three `plan.md` items after that line.\n" +
		"- Recommend one of those items.\n\n## "
	releaseRules := []string{
		"End the structured Package completion report with `Run below to release:`.",
		"Place the exact release command immediately after that line.",
		"Add nothing after the release command.",
	}
	retired := []string{
		"Present only the release command after prep.",
		"Do not add trailing commentary about wrapper routing or prompts.",
		"Do not add trailing commentary after presenting the command.",
	}
	assertContract := func(t *testing.T, name, agents, release string) {
		t.Helper()
		for _, rule := range agentRules {
			if count := strings.Count(agents, rule); count != 1 {
				t.Errorf("%s AGENTS rule count=%d, want 1: %s", name, count, rule)
			}
		}
		if strings.Count(agents, reportGroupings) != 1 {
			t.Errorf("%s AGENTS lacks the Package Self-review and Completed Release groupings after the release-command rule", name)
		}
		position := -1
		for _, rule := range releaseRules {
			if count := strings.Count(release, rule); count != 1 {
				t.Errorf("%s release rule count=%d, want 1: %s", name, count, rule)
			}
			next := strings.Index(release, rule)
			if next <= position {
				t.Errorf("%s release rule is out of order: %s", name, rule)
			}
			position = next
		}
		for _, phrase := range retired {
			if strings.Contains(release, phrase) {
				t.Errorf("%s retains conflicting Package instruction: %s", name, phrase)
			}
		}
	}

	root := filepath.Join("..", "..")
	rootAgents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, path string }{
		{name: "CODE authority", path: "govna/build-release.md"},
		{name: "CODE template", path: "internal/canon/assets/overlays/code/files/govna/build-release.md.tmpl"},
		{name: "DOC template", path: "internal/canon/assets/overlays/doc/files/govna/release.md.tmpl"},
	} {
		release, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(tc.path)))
		if err != nil {
			t.Fatal(err)
		}
		assertContract(t, tc.name, string(rootAgents), string(release))
	}

	profiles := []struct {
		name, releasePath string
		config            Config
	}{{name: "DOC", releasePath: "govna/release.md", config: Config{Flavor: Doc, RepoName: "handbook"}}}
	for _, stack := range Stacks() {
		modulePath := ""
		if stack == "Go" {
			modulePath = "example.com/widget"
		}
		profiles = append(profiles, struct {
			name, releasePath string
			config            Config
		}{name: "CODE/" + stack, releasePath: "govna/build-release.md", config: Config{Flavor: Code, RepoName: "widget", Stack: stack, ModulePath: modulePath}})
	}
	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			files, err := Render(profile.config)
			if err != nil {
				t.Fatal(err)
			}
			assertContract(t, profile.name, fileText(t, files, "AGENTS.md"), fileText(t, files, profile.releasePath))
		})
	}
}

func TestAgentReleaseContract(t *testing.T) {
	const docBlock = "\n### Agent Release\n\n" +
		"- Apply these rules when the Director sends `release` or `rel`, standalone or with a named batch.\n" +
		"- Treat standalone `release` or `rel` as authorization for Package plus one agent-run release.\n" +
		"- Apply standalone `release` or `rel` to the same release batch as standalone Package.\n" +
		"- Treat `release` or `rel` followed by a named batch as Package of that batch plus one agent-run release.\n" +
		"- Stop before the release when Package fails or needs a Director decision.\n" +
		"- Run the printed release command with `--yes` appended.\n" +
		"- Run no other git or GitHub write under the keyword.\n" +
		"- Retry nothing after a release failure.\n" +
		"- Report the failure output and the printed recovery command to the Director.\n" +
		"- Report Package and the release result in one completion report.\n" +
		"- Replace `No commit or release command executed.` and `Run below to release:` with the release result in that report.\n" +
		"- Treat a successful agent-run release as a completed release under `### Completed Release` unless `## Project Rules` name a later step that completes a release.\n\n###"
	retired := []string{
		"No EXCEPTION.",
		"Never execute release commands.",
		"The agent never runs the release command.",
		"`shipped` or `released`",
		"Accept the keyword only after a clean Package completion in the active session.",
		"Record the `git status --porcelain` output at each Package completion.",
		"Reject the keyword when the current `git status --porcelain` output differs from that record.",
		"Run the exact release command from that Package report with `--yes` appended.",
		"Interpret standalone `release` or `rel` only as an agent-run release under `### Agent Release`.",
		"as the named-batch Package form.",
	}
	codeBlock := docBlock
	assertContract := func(t *testing.T, name string, flavor Flavor, agents string, governance map[string]string) {
		t.Helper()
		block := docBlock
		if flavor == Code {
			block = codeBlock
		}
		_, rest, ok := strings.Cut(agents, "\n## Approval Boundaries\n")
		boundaries, _, _ := strings.Cut(rest, "\n## File-Change Discipline\n")
		if !ok || strings.Count(boundaries, block) != 1 || strings.Count(agents, "### Agent Release\n") != 1 {
			t.Errorf("%s AGENTS lacks one complete `### Agent Release` grouping under `## Approval Boundaries`", name)
		}
		for path, content := range governance {
			for _, phrase := range retired {
				if strings.Contains(content, phrase) {
					t.Errorf("%s %s retains retired release wording: %s", name, path, phrase)
				}
			}
		}
	}

	root := filepath.Join("..", "..")
	rootFiles := map[string]string{}
	for _, path := range []string{"AGENTS.md", "govna/roles.md", "govna/build-release.md", "govna/canon-spec.md"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		rootFiles[path] = string(data)
	}
	assertContract(t, "repository", Code, rootFiles["AGENTS.md"], rootFiles)

	for _, p := range dedupProfiles() {
		t.Run(p.name, func(t *testing.T) {
			files, err := Render(p.config)
			if err != nil {
				t.Fatal(err)
			}
			governance := map[string]string{}
			for _, file := range files {
				if file.Path == "AGENTS.md" || strings.HasPrefix(file.Path, "govna/") {
					governance[file.Path] = string(file.Content)
				}
			}
			assertContract(t, p.name, p.config.Flavor, fileText(t, files, "AGENTS.md"), governance)
		})
	}
}

func TestSupportedCODEStackContract(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, path := range []string{"govna/canon-spec.md"} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		if strings.Count(text, "- Use only Go, Rust, Swift, or Terraform as selectable CODE stacks.") != 1 {
			t.Errorf("%s omits the supported-stack boundary", path)
		}
		for _, retired := range []string{"Prefer Swift over Node", "Infer Node", "Infer Python", "Infer Java"} {
			if strings.Contains(text, retired) {
				t.Errorf("%s retains unsupported inference language %q", path, retired)
			}
		}
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "CODE repositories can select only Go, Rust, Swift, and Terraform because those stacks have complete canonical build adapters.") {
		t.Error("README omits the complete selectable CODE-stack set")
	}
	for _, stack := range Stacks() {
		modulePath := ""
		if stack == "Go" {
			modulePath = "example.com/widget"
		}
		files, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: stack, ModulePath: modulePath})
		if err != nil {
			t.Fatal(err)
		}
		if build := fileText(t, files, "build.sh"); !strings.HasPrefix(build, "#!/usr/bin/env bash\n") {
			t.Errorf("%s render lacks its canonical build adapter", stack)
		}
	}
}

func TestCandidateCanonReviewContract(t *testing.T) {
	root := filepath.Join("..", "..")
	agents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{
		"- Run the consumer-equivalent candidate-canon review in `govna/canon-maintenance.md` before completing any canon change.",
	} {
		if strings.Count(string(agents), rule) != 1 {
			t.Errorf("AGENTS.md requires one maintainer gate: %s", rule)
		}
	}
	for _, path := range []string{"govna/canon-maintenance.md"} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, rule := range []string{
			"- Render DOC and every registered CODE stack.",
			"- Exercise every rendered profile through ordinary non-JSON `govna audit` at least once.",
			"- Resolve the Govna executable before each consumer-equivalent audit.",
			"- Reuse that resolved executable for the Audit scratch render.",
			"- Verify the emitted marker versions against the resolved executable.",
			"- Verify the rendered baseline canon version against the emitted marker.",
			"- Complete one bounded scratch review for every actionable emitted AC.",
			"- Compare every actionable path without JSON diff fields.",
			"- Keep the emitted AC and consumer fixture byte-identical during review.",
			"- Remove the exact scratch directory before completing review.",
			"- Exercise one-AC and fitting multi-AC pending batches.",
			"- Exercise an oversized projected batch before another Implement.",
			"- Exercise oversized, partial, and partly unaccepted batches before Package prep.",
			"- Verify whole-tree release staging cannot bypass complete-batch rules.",
			"- Verify 80-byte acceptance with ASCII and multibyte messages.",
			"- Reject 81-byte messages before release-prep mutation.",
			"- Map every provided command or executable artifact to its governing claims.",
			"- Map every governing claim to a named behavioral regression.",
			"- Verify every supported profile provides its required canonical command implementation.",
			"- Block Implement completion on any unresolved Govna-canon finding.",
		} {
			if strings.Count(string(content), rule) != 1 {
				t.Errorf("%s requires one candidate-canon rule: %s", path, rule)
			}
		}
	}
}

func TestImmutableAuditACVerificationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	paths := []string{
		"AGENTS.md",
		"internal/canon/assets/base/AGENTS.md.tmpl",
		"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl",
	}
	required := []string{
		"- Confirm each settled decision landed verbatim in the AC.",
		"- Treat a Director-resolved routing decision or explicit workflow override recorded in chat for an immutable emitted AC as satisfying the verbatim-in-AC check.",
		"- Treat `CHANGELOG.md` as effective implementation scope only when a resolved legacy-phrase outcome requires removing an exact phrase.",
	}
	for _, path := range paths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, rule := range required {
			if count := strings.Count(string(content), rule); count != 1 {
				t.Errorf("%s rule count=%d, want 1 for %q", path, count, rule)
			}
		}
	}
	for _, path := range []string{
		"govna/audit.md",
		"internal/canon/assets/overlays/code/files/govna/audit.md.tmpl",
		"internal/canon/assets/overlays/doc/files/govna/audit.md.tmpl",
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, rule := range []string{
			"- Install an exact current-canon replacement before retired-source routing.",
			"- Remove the exact legacy phrase after that verification.",
		} {
			if count := strings.Count(string(content), rule); count != 1 {
				t.Errorf("%s rule count=%d, want 1 for %q", path, count, rule)
			}
		}
	}
}

func TestIntegratedAuditAdoptionContract(t *testing.T) {
	root := filepath.Join("..", "..")
	agentPaths := []string{
		"AGENTS.md",
		"internal/canon/assets/base/AGENTS.md.tmpl",
		"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl",
	}
	adoptionRules := []string{
		"- Apply integrated audit adoption only when the Director explicitly requests govna audit.",
		"- Enter Audit only when govna audit emits or reuses one guarded adoption AC.",
		"- Keep a clean audit result or pre-emission failure outside the AC phases.",
		"- Audit the emitted AC immediately.",
		"- Authorize the bounded scratch-review procedure in `govna/audit.md` from the original explicit `govna audit` request.",
		"- Bind the scratch review to the resolved Govna executable and the emitted AC marker versions.",
		"- Limit scratch-review authority to one unique system-temporary directory outside the consumer repository.",
		"- Keep the emitted AC and consumer repository unchanged during integrated Audit and Refine.",
		"- Remove the exact scratch directory before reporting Audit completion or a blocker.",
		"- End scratch-review authority when Audit ends.",
		"- Pause integrated audit adoption while any blocking finding or Director decision remains unresolved.",
		"- Resume Refine after the Director resolves every blocking finding and decision.",
		"- Require a new audit emission when a required correction would change the immutable AC.",
		"- Complete Refine without editing the emitted AC when no blocker remains.",
		"- Run Pre-Implementation Verification after integrated Refine.",
		"- Stop integrated audit adoption before Implement.",
		"- Track the active phase in the session instead of the emitted AC.",
	}
	for _, path := range agentPaths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, rule := range append([]string{
			"- Treat an explicit request to run govna audit as authorization for integrated audit adoption under `### Audit Adoption`.",
			"- Pause after Audit until the Director requests Refine unless integrated audit adoption or eligible automatic Refine entry applies.",
			"- Treat a Director-resolved routing decision or explicit workflow override recorded in chat for an immutable emitted AC as satisfying the verbatim-in-AC check.",
			"- Enter Audit automatically when Draft completes the active AC with populated scope and acceptance tests.",
			"- Keep an unscoped stub paused until the Director scopes it.",
			"- Enter Refine automatically when Audit completes with only advancement-eligible findings.",
			"- Define an advancement-eligible finding as one outside every Director-owned category in `### General Gates` and roles.md `What the Operator Must Defer` with exactly one materially valid correction.",
			"- Block automatic Refine entry on any unresolved contract-integrity finding.",
			"- Report every autonomously resolved Audit finding with its resolution in the Refine completion.",
			"- Run Pre-Implementation Verification after eligible automatic Refine completion.",
			"- Remain in Refine when the checklist finds a gap.",
			"- Exempt completed-Draft automatic Audit entry from a fresh Audit action instruction.",
			"- Pause immediately when an automatic-transition eligibility condition fails.",
			"- Prevent automatic advancement from authorizing Implement, Ratify, Package, release preparation, publication, delegation, or commits.",
		}, adoptionRules...) {
			if count := strings.Count(text, rule); count != 1 {
				t.Errorf("%s integrated-audit rule count=%d, want 1: %s", path, count, rule)
			}
		}
		_, adoption, ok := strings.Cut(text, "### Audit Adoption\n")
		if !ok {
			t.Errorf("%s omits Audit Adoption", path)
			continue
		}
		if end := strings.Index(adoption, "\n## "); end >= 0 {
			adoption = adoption[:end]
		}
		for _, rule := range adoptionRules {
			if !strings.Contains(adoption, rule) {
				t.Errorf("%s places integrated-audit rule outside Audit Adoption: %s", path, rule)
			}
		}
	}

	auditPaths := []string{
		"govna/audit.md",
		"internal/canon/assets/overlays/code/files/govna/audit.md.tmpl",
		"internal/canon/assets/overlays/doc/files/govna/audit.md.tmpl",
	}
	for _, path := range auditPaths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, required := range []string{
			"### Agent-mediated review",
			"- Run the ordinary agent-mediated audit without `--json`.",
			"- Require the emitted AC marker versions to match the recorded detailed version.",
			"- Render the selected canon into that scratch directory once with the resolved executable.",
			"- Compare every actionable path through the emitted `### Audit Review` instructions.",
			"- Follow `AGENTS.md` `### Audit Adoption` for every phase entry, pause, and exit of this review.",
		} {
			if strings.Count(text, required) != 1 {
				t.Errorf("%s requires one integrated-audit phrase %q", path, required)
			}
		}
		if strings.Contains(text, "audit emission; awaiting explicit Director Audit") {
			t.Errorf("%s retains the separate-Audit waiting status", path)
		}
	}

	const cyclePointer = "- Apply `AGENTS.md` `### Four-Phase Workflow`, `### Phase-Advancement Rules`, and `### Audit Adoption` to this cycle."
	docRequirements := map[string]string{
		"README.md":       "An explicit agent-mediated request to run `govna audit` also authorizes immediate review",
		"arch.md":         "Integrated audit adoption is the only command-mediated phase exception.",
		"govna/README.md": "continues through immediate Operator Audit and no-edit Refine",
		"internal/canon/assets/overlays/code/files/README.md.tmpl":                  "also authorizes immediate Audit and Refine",
		"internal/canon/assets/overlays/doc/files/README.md.tmpl":                   "also authorizes immediate Audit and Refine",
		"internal/canon/assets/overlays/code/files/arch.md.tmpl":                    "Integrated audit adoption is the only command-mediated phase exception.",
		"internal/canon/assets/overlays/code/files/govna/README.md.tmpl":            "continues through immediate Operator Audit and no-edit Refine",
		"internal/canon/assets/overlays/doc/files/govna/README.md.tmpl":             "continues through immediate Operator Audit and no-edit Refine",
		"govna/development-cycle.md":                                                cyclePointer,
		"internal/canon/assets/overlays/code/files/govna/development-cycle.md.tmpl": cyclePointer,
		"internal/canon/assets/overlays/doc/files/govna/editing-cycle.md.tmpl":      cyclePointer,
	}
	for path, required := range docRequirements {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), required) {
			t.Errorf("%s omits integrated-audit guidance %q", path, required)
		}
	}
}

func TestRatifiedReleaseBatchContract(t *testing.T) {
	root := filepath.Join("..", "..")
	const cyclePointer = "- Apply `AGENTS.md` `### Four-Phase Workflow`, `### Phase-Advancement Rules`, and `### Audit Adoption` to this cycle."
	agentPaths := []string{
		"AGENTS.md",
		"internal/canon/assets/base/AGENTS.md.tmpl",
		"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl",
	}
	for _, path := range agentPaths {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, required := range []string{
			"- Define the pending release batch as every unpackaged AC whose implementation is present in the unreleased repository state.",
			"- Include an implemented AC in the pending release batch while it awaits Ratify.",
			"- Require every pending release-batch member to complete Ratify before Package.",
			"- Require the established release batch to equal the complete pending release batch.",
			"- Reject Package while excluded implemented work remains in the unreleased repository state.",
			"- Require the release-message AC-reference set to equal the established release batch before Package runs prep.",
			"- Define a direct batch as one or more unreleased direct-handled changes while the pending release batch holds no AC.",
			"- Treat a direct batch as established only after an explicit Director Package request.",
			"- Describe each direct-handled change in the release message for a direct batch.",
			"- Reject a release message without AC references while any unpackaged implemented AC exists.",
			"- Apply standalone Package, package, pack, or prep to an established direct batch when no AC can enter Package.",
			"- Treat one active Ratified AC as an established one-AC release batch only when it is the complete pending release batch.",
			"- Treat only a Director-named complete pending release batch as an established multi-AC release batch.",
			"- Accept only Package, `release`, or `rel` followed by a plus-joined list of uppercase AC<number> references as the named-batch form.",
			"- Apply standalone Package, package, pack, or prep to the established Ratified release batch.",
			"- Ask the Director to name the release batch when multiple ungrouped Ratified ACs can enter Package.",
			"- Measure the projected complete pending release batch with one private provisional prefix-plus-summary string before another AC enters Implement.",
			"- Use the provisional string only for the 80-byte fit check.",
			"- Discard the provisional string after the fit check.",
			"- Start another Implement only when the projected complete pending release batch can fit one compliant release message.",
			"- Require Package for the current fitting batch before another Implement when the projection cannot fit.",
			"- Recheck the complete pending release batch and exact release message before Package runs prep.",
			"- Reject an oversized or partial release batch.",
			"- Prohibit automatic release-batch splitting.",
		} {
			if strings.Count(text, required) != 1 {
				t.Errorf("%s requires one release-batch rule %q", path, required)
			}
		}
		if strings.Contains(text, "Treat an explicit standalone `Package`, `package`, `pack`, or `prep` request in an active Ratified AC context as the trigger") {
			t.Errorf("%s retains the singular standalone Package trigger", path)
		}
	}

	for _, path := range []string{
		"govna/build-release.md",
		"internal/canon/assets/overlays/code/files/govna/build-release.md.tmpl",
		"internal/canon/assets/overlays/doc/files/govna/release.md.tmpl",
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, required := range []string{
			"- Apply this checklist only to an explicit Director Package instruction for an established Ratified release batch or direct batch.",
		} {
			if strings.Count(text, required) != 1 {
				t.Errorf("%s requires one release-batch checklist rule %q", path, required)
			}
		}
	}

	for _, path := range []string{
		"govna/development-cycle.md",
		"internal/canon/assets/overlays/code/files/govna/development-cycle.md.tmpl",
		"internal/canon/assets/overlays/doc/files/govna/editing-cycle.md.tmpl",
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		if strings.Count(text, cyclePointer) != 1 {
			t.Errorf("%s requires one AGENTS.md cycle pointer %q", path, cyclePointer)
		}
		if strings.Contains(text, "as equivalent post-Ratify release-preparation actions.") {
			t.Errorf("%s retains the Ratify-only Package naming rule", path)
		}
	}

	for _, path := range []string{"govna/canon-maintenance.md"} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(content), "- Exercise a direct batch before Package prep.") != 1 {
			t.Errorf("%s omits the empty-release-batch safety scenario", path)
		}
	}
}

func TestEmptyReleaseBatchFlavorRules(t *testing.T) {
	root := filepath.Join("..", "..")
	const buildEvidence = "- Use the successful final full build as current Package evidence for a direct batch."
	const codeInterpretation = "- Interpret standalone `Package`, `package`, `pack`, or `prep` as Package only after an explicit request for a Ratified release batch or direct batch."
	const docInterpretation = "- Interpret `Package` as the release-preparation action only after an explicit Director request for a Ratified release batch or direct batch."
	const retiredInterpretation = "as post-Ratify Package only after acceptance and explicit request."
	const retiredDocInterpretation = "as the post-Ratify release-preparation action only after Ratify acceptance and an explicit Director request."
	for path, want := range map[string]map[string]int{
		"AGENTS.md": {buildEvidence: 1, codeInterpretation: 1, docInterpretation: 0, retiredInterpretation: 0, retiredDocInterpretation: 0},
		"internal/canon/assets/base/AGENTS.md.tmpl":               {buildEvidence: 1, codeInterpretation: 1, docInterpretation: 0, retiredInterpretation: 0, retiredDocInterpretation: 0},
		"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl": {buildEvidence: 0, codeInterpretation: 0, docInterpretation: 1, retiredInterpretation: 0, retiredDocInterpretation: 0},
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for text, count := range want {
			if got := strings.Count(string(content), text); got != count {
				t.Errorf("%s has %d of %q, want %d", path, got, text, count)
			}
		}
	}

	const checklistEvidence = "   - Use the successful final full build as current Package evidence for a direct batch."
	const acceptWithout = "Accept a message without AC references."
	for path, count := range map[string]int{
		"govna/canon-spec.md":    1,
		"govna/build-release.md": 0,
		"internal/canon/assets/overlays/code/files/govna/build-release.md.tmpl": 0,
		"internal/canon/assets/overlays/doc/files/govna/release.md.tmpl":        0,
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(content), checklistEvidence); got != 0 {
			t.Errorf("%s repeats the AGENTS.md direct-batch evidence rule %q", path, checklistEvidence)
		}
		if got := strings.Count(string(content), acceptWithout); got != count {
			t.Errorf("%s has %d of %q, want %d", path, got, acceptWithout, count)
		}
	}
}

func TestDocContentEntryFlavorRules(t *testing.T) {
	root := filepath.Join("..", "..")
	const entryDefault = "- Handle a content entry and its index, register, and log companions directly by default.\n"
	const plumbingAC = "- Require an AC for a checker, build, template, or site-structure change regardless of file count.\n"
	const docNonTrivial = "- Treat every non-trivial change outside a content entry as AC-first work unless the Director explicitly overrides it.\n"
	const codeNonTrivial = "- Treat every non-trivial change as AC-first work unless the Director explicitly overrides it.\n"
	const readmeSentence = "A content entry, meaning one post, page, chapter, note, or scene, and its index, register, and log companions are handled directly by default; checker, build, template, site-structure, governance, and release changes take an AC."
	codeCounts := map[string]int{entryDefault: 0, plumbingAC: 0, docNonTrivial: 0, codeNonTrivial: 1}
	docCounts := map[string]int{entryDefault: 1, plumbingAC: 1, docNonTrivial: 1, codeNonTrivial: 0}
	for path, want := range map[string]map[string]int{
		"AGENTS.md": codeCounts,
		"internal/canon/assets/base/AGENTS.md.tmpl":               codeCounts,
		"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl": docCounts,
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		for text, count := range want {
			if got := strings.Count(string(content), text); got != count {
				t.Errorf("%s has %d of %q, want %d", path, got, text, count)
			}
		}
	}
	for path, count := range map[string]int{
		"internal/canon/assets/overlays/code/files/README.md.tmpl": 0,
		"internal/canon/assets/overlays/doc/files/README.md.tmpl":  1,
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(string(content), readmeSentence); got != count {
			t.Errorf("%s has %d of the content-entry sentence, want %d", path, got, count)
		}
	}
	for _, variant := range []struct {
		name   string
		config Config
		want   map[string]int
		readme int
	}{
		{name: "rendered CODE", config: Config{Flavor: Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"}, want: codeCounts, readme: 0},
		{name: "rendered DOC", config: Config{Flavor: Doc, RepoName: "handbook"}, want: docCounts, readme: 1},
	} {
		files, err := Render(variant.config)
		if err != nil {
			t.Fatal(err)
		}
		agents := fileText(t, files, "AGENTS.md")
		for text, count := range variant.want {
			if got := strings.Count(agents, text); got != count {
				t.Errorf("%s AGENTS.md has %d of %q, want %d", variant.name, got, text, count)
			}
		}
		if got := strings.Count(fileText(t, files, "README.md"), readmeSentence); got != variant.readme {
			t.Errorf("%s README.md has %d of the content-entry sentence, want %d", variant.name, got, variant.readme)
		}
	}
}

func TestFlavorSpecificRolesAndDotfiles(t *testing.T) {
	type variant struct {
		name      string
		config    Config
		release   string
		absent    string
		stackLine string
	}
	variants := []variant{{
		name:    "DOC",
		config:  Config{Flavor: Doc, RepoName: "handbook"},
		release: "`govna/release.md`",
		absent:  "`govna/build-release.md`",
	}}
	stackLines := map[string]string{
		"Go":        "go.work",
		"Rust":      "/target/",
		"Swift":     "/.build/",
		"Terraform": ".terraform/",
	}
	for _, stack := range Stacks() {
		modulePath := ""
		if stack == "Go" {
			modulePath = "example.com/widget"
		}
		variants = append(variants, variant{
			name:      stack,
			config:    Config{Flavor: Code, RepoName: "widget", Stack: stack, ModulePath: modulePath},
			release:   "`govna/build-release.md`",
			absent:    "`govna/release.md`",
			stackLine: stackLines[stack],
		})
	}

	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			files, err := Render(variant.config)
			if err != nil {
				t.Fatal(err)
			}
			roles := fileText(t, files, "govna/roles.md")
			if !strings.Contains(roles, variant.release) || strings.Contains(roles, variant.absent) {
				t.Errorf("roles release references are not flavor-safe: %s", roles)
			}
			ignore := fileText(t, files, ".gitignore")
			for _, common := range []string{".DS_Store", "Thumbs.db", "*.swp", ".idea/", ".vscode/"} {
				if !strings.Contains(ignore, common) {
					t.Errorf(".gitignore omits common entry %q", common)
				}
			}
			if variant.stackLine != "" && !strings.Contains(ignore, variant.stackLine) {
				t.Errorf(".gitignore omits %s entry %q", variant.name, variant.stackLine)
			}
			if strings.Contains(ignore, "{{") {
				t.Errorf(".gitignore retains a template placeholder: %s", ignore)
			}
		})
	}
}

type ratifyEvidenceState struct {
	snapshotPresent      bool
	snapshotComplete     bool
	identitiesMatch      bool
	unrecordedReadable   bool
	uncertainRelevance   bool
	excludedInputChanged bool
	excludedProvenUnread bool
	inlineCorrection     bool
	nonCleanDisposition  bool
}

type ratifyExpectedActions struct {
	reuseCurrentEvidence bool
	revalidateEvidence   bool
	repeatCurrentWork    bool
	applyFindingRouting  bool
	performFinalReview   bool
	requestSecondSignal  bool
}

func ratifyActions(state ratifyEvidenceState) ratifyExpectedActions {
	current := state.snapshotPresent && state.snapshotComplete && state.identitiesMatch
	current = current && !state.unrecordedReadable && !state.uncertainRelevance
	if state.excludedInputChanged && !state.excludedProvenUnread {
		current = false
	}
	if state.inlineCorrection {
		current = false
	}
	return ratifyExpectedActions{
		reuseCurrentEvidence: current,
		revalidateEvidence:   !current,
		applyFindingRouting:  current && state.nonCleanDisposition,
		performFinalReview:   true,
	}
}

func TestGovernanceScenarios(t *testing.T) {
	scenarios := []struct {
		name  string
		state ratifyEvidenceState
		want  ratifyExpectedActions
	}{
		{
			name:  "clean current evidence",
			state: ratifyEvidenceState{snapshotPresent: true, snapshotComplete: true, identitiesMatch: true},
			want:  ratifyExpectedActions{reuseCurrentEvidence: true, performFinalReview: true},
		},
		{
			name:  "missing snapshot",
			state: ratifyEvidenceState{},
			want:  ratifyExpectedActions{revalidateEvidence: true, performFinalReview: true},
		},
		{
			name:  "incomplete snapshot",
			state: ratifyEvidenceState{snapshotPresent: true, identitiesMatch: true},
			want:  ratifyExpectedActions{revalidateEvidence: true, performFinalReview: true},
		},
		{
			name:  "changed recorded input",
			state: ratifyEvidenceState{snapshotPresent: true, snapshotComplete: true},
			want:  ratifyExpectedActions{revalidateEvidence: true, performFinalReview: true},
		},
		{
			name:  "unrecorded readable input",
			state: ratifyEvidenceState{snapshotPresent: true, snapshotComplete: true, identitiesMatch: true, unrecordedReadable: true},
			want:  ratifyExpectedActions{revalidateEvidence: true, performFinalReview: true},
		},
		{
			name:  "uncertain relevance",
			state: ratifyEvidenceState{snapshotPresent: true, snapshotComplete: true, identitiesMatch: true, uncertainRelevance: true},
			want:  ratifyExpectedActions{revalidateEvidence: true, performFinalReview: true},
		},
		{
			name:  "irrelevant change proved unreadable",
			state: ratifyEvidenceState{snapshotPresent: true, snapshotComplete: true, identitiesMatch: true, excludedInputChanged: true, excludedProvenUnread: true},
			want:  ratifyExpectedActions{reuseCurrentEvidence: true, performFinalReview: true},
		},
		{
			name:  "unproved excluded change",
			state: ratifyEvidenceState{snapshotPresent: true, snapshotComplete: true, identitiesMatch: true, excludedInputChanged: true},
			want:  ratifyExpectedActions{revalidateEvidence: true, performFinalReview: true},
		},
		{
			name:  "inline correction",
			state: ratifyEvidenceState{snapshotPresent: true, snapshotComplete: true, identitiesMatch: true, inlineCorrection: true},
			want:  ratifyExpectedActions{revalidateEvidence: true, performFinalReview: true},
		},
		{
			name:  "current non-clean disposition",
			state: ratifyEvidenceState{snapshotPresent: true, snapshotComplete: true, identitiesMatch: true, nonCleanDisposition: true},
			want:  ratifyExpectedActions{reuseCurrentEvidence: true, applyFindingRouting: true, performFinalReview: true},
		},
		{
			name:  "stale non-clean disposition",
			state: ratifyEvidenceState{snapshotPresent: true, snapshotComplete: true, nonCleanDisposition: true},
			want:  ratifyExpectedActions{revalidateEvidence: true, performFinalReview: true},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			got := ratifyActions(scenario.state)
			if got.repeatCurrentWork {
				t.Error("Ratify scenario repeats current work")
			}
			if got.requestSecondSignal {
				t.Error("Ratify scenario requests a second acceptance signal")
			}
			if got != scenario.want {
				t.Fatalf("Ratify actions=%+v want=%+v", got, scenario.want)
			}
		})
	}

	profiles := []struct {
		name      string
		config    Config
		cyclePath string
	}{
		{name: "DOC", config: Config{Flavor: Doc, RepoName: "handbook"}, cyclePath: "govna/editing-cycle.md"},
	}
	for _, stack := range Stacks() {
		config := Config{Flavor: Code, RepoName: "widget", Stack: stack}
		if stack == "Go" {
			config.ModulePath = "example.com/widget"
		}
		profiles = append(profiles, struct {
			name      string
			config    Config
			cyclePath string
		}{name: "CODE/" + stack, config: config, cyclePath: "govna/development-cycle.md"})
	}

	required := []string{
		"- Treat the person in the session as the Director.",
		"- Address the Director as `you` in chat.",
		"- Reserve third-person `the Director` for rules and records.",
		"- Ask the Director for each decision as a direct question with a recommendation.",
		"- Capture one deterministic Implement evidence snapshot in the closure-audit working record.",
		"- Capture the snapshot after final validation and the last repository mutation.",
		"- Treat standalone `Ratify` or `ratify` after successful Implement completion as the Director's acceptance action.",
		"- Check the Implement evidence snapshot with non-mutating state, version, content-identity, and diff checks.",
		"- Treat evidence as current only when the snapshot is present, complete, and identity-matching.",
		"- Treat every non-current snapshot as stale evidence.",
		"- Treat an unrecorded input as stale when a reused validation or acceptance check can read it.",
		"- Treat uncertainty about an input's relevance as stale evidence.",
		"- Distinguish evidence currentness from acceptance-test outcome.",
		"- Preserve each recorded acceptance-test disposition exactly during reuse.",
		"- Apply existing Ratify routing to every current non-clean disposition.",
		"- Reuse current closure-audit findings, acceptance-test dispositions, render evidence, and final validation during Ratify.",
		"- Prohibit a new scratch directory, canon render, build, or test solely to repeat current evidence.",
		"- Rerun applicable validation when Ratify evidence is missing or stale.",
		"- Rerun applicable validation after an inline Ratify correction.",
		"- Perform the final review during the same Ratify turn.",
		"- Recheck only new or unresolved contract-integrity findings during Audit completion, Implement completion, closure audit, and Ratify.",
		"- Skip requests for a second acceptance signal after a clean Ratify review.",
		"- Use the complete primary-repository state as the default dependency boundary.",
		"- Narrow the dependency boundary only when repository evidence proves that a reused check cannot read the excluded state.",
		"- Treat the snapshot as incomplete when a relevant ignored or external input cannot be identified.",
		"- Treat evidence as current only when the snapshot is present, complete, and identity-matching.",
		"- Run `./build.sh` only when required build evidence is missing or stale.",
		"- Run each acceptance test in the active AC when its current disposition is unavailable.",
		"- Report each failed, pending, manual, or unexercised acceptance test in the Implement completion report.",
		"- Cover every passing automated acceptance test in one line of that report.",
		"- Enter Audit automatically when Draft completes the active AC with populated scope and acceptance tests.",
		"- Enter Refine automatically when Audit completes with only advancement-eligible findings.",
		"**Why Clean Ratify Reuses Implement Evidence.**",
	}
	for _, profile := range profiles {
		t.Run(profile.name, func(t *testing.T) {
			files, err := Render(profile.config)
			if err != nil {
				t.Fatal(err)
			}
			contract := strings.Join([]string{
				fileText(t, files, "AGENTS.md"),
				fileText(t, files, "govna/roles.md"),
				fileText(t, files, profile.cyclePath),
				repoFile(t, "arch.md"),
			}, "\n")
			for _, rule := range required {
				if !strings.Contains(contract, rule) {
					t.Errorf("rendered contract omits scenario rule %q", rule)
				}
			}
			for _, retired := range []string{
				"- Run `./build.sh` only when reviewing code changes or build-output claims.",
				"- Run each acceptance test in the active AC when it can be exercised.",
				"- Report the result of each exercised acceptance test.",
			} {
				if strings.Contains(contract, retired) {
					t.Errorf("rendered contract retains conflicting rule %q", retired)
				}
			}
		})
	}
}

func TestPhaseEligibleACRoutingRules(t *testing.T) {
	rules := []string{
		"- Start an AC cycle only after the Director identifies the AC and authorizes Audit, integrated audit adoption identifies the emitted AC, or a completed Draft identifies the active AC.",
		"- Apply an unnumbered Audit, Refine, Implement, or Ratify instruction when exactly one AC can enter the requested phase.",
		"- Require the AC number when multiple ACs can enter the requested phase.",
		"- Ask the Director for the AC number and last completed lifecycle action when phase eligibility cannot be established.",
		"- Measure the projected complete pending release batch with one private provisional prefix-plus-summary string before another AC enters Implement.",
		"- Use the provisional string only for the 80-byte fit check.",
		"- Discard the provisional string after the fit check.",
		"- Start another Implement only when the projected complete pending release batch can fit one compliant release message.",
		"- Require Package for the current fitting batch before another Implement when the projection cannot fit.",
		"- Treat one active Ratified AC as an established one-AC release batch only when it is the complete pending release batch.",
		"- Treat only a Director-named complete pending release batch as an established multi-AC release batch.",
		"- Accept only Package, release, or rel followed by a plus-joined list of uppercase AC<number> references as the named-batch form.",
		"- Apply standalone Package, package, pack, or prep to the established Ratified release batch.",
		"- Ask the Director to name the release batch when multiple ungrouped Ratified ACs can enter Package.",
		"- Recheck the complete pending release batch and exact release message before Package runs prep.",
		"- Reject an oversized or partial release batch.",
		"- Prohibit automatic release-batch splitting.",
	}
	retiredRules := []string{
		"Apply an unnumbered action instruction to the sole AC under `govna/`.",
		"Use an unnumbered phase instruction when one AC is under `govna/`.",
		"Require the AC number when multiple ACs are present.",
		"Start an AC cycle only when the Director identifies the active AC and explicitly requests Audit.",
		"Start an AC cycle only after the Director identifies the AC and authorizes Audit or integrated audit adoption identifies the emitted AC.",
		"Apply an unnumbered Audit, Refine, Implement, Ratify, or Package instruction when exactly one AC can enter the requested action under its established lifecycle state.",
		"Require the AC number when multiple ACs can enter the requested action.",
		"Ask the Director for the AC number and last completed lifecycle action when eligibility cannot be established.",
		"Exempt only an eligible bounded completeness correction under `### Four-Phase Workflow` from a fresh Refine or Implement action instruction.",
	}
	assertRules := func(t *testing.T, path, content string) {
		t.Helper()
		normalized := strings.ReplaceAll(content, "`", "")
		for _, rule := range rules {
			if count := strings.Count(normalized, rule); count != 1 {
				t.Errorf("%s phase-routing rule count=%d, want 1: %s", path, count, rule)
			}
		}
		for _, retired := range retiredRules {
			if strings.Contains(content, retired) {
				t.Errorf("%s retains broad file-count rule: %s", path, retired)
			}
		}
	}
	assertAGENTSRules := func(t *testing.T, path, content string) {
		t.Helper()
		assertRules(t, path, content)
		const heading = "### Phase-Advancement Rules\n"
		_, after, ok := strings.Cut(content, heading)
		if !ok {
			t.Errorf("%s omits Phase-Advancement Rules", path)
			return
		}
		section := strings.ReplaceAll(after, "`", "")
		if end := strings.Index(section, "\n### "); end >= 0 {
			section = section[:end]
		}
		if end := strings.Index(section, "\n## "); end >= 0 {
			section = section[:end]
		}
		for _, rule := range rules {
			if !strings.Contains(section, rule) {
				t.Errorf("%s does not place phase-routing rule under Phase-Advancement Rules: %s", path, rule)
			}
		}
	}

	assertCycleRules := func(t *testing.T, path, content string) {
		t.Helper()
		if !strings.Contains(content, "- Apply `AGENTS.md` `### Four-Phase Workflow`, `### Phase-Advancement Rules`, and `### Audit Adoption` to this cycle.") {
			t.Errorf("%s does not reference Phase-Advancement Rules", path)
		}
		normalized := strings.ReplaceAll(content, "`", "")
		for _, rule := range rules {
			if strings.Contains(normalized, rule) {
				t.Errorf("%s repeats AGENTS.md phase-routing rule: %s", path, rule)
			}
		}
		for _, retired := range retiredRules {
			if strings.Contains(content, retired) {
				t.Errorf("%s retains broad file-count rule: %s", path, retired)
			}
		}
	}

	root := filepath.Join("..", "..")
	for _, path := range []string{
		"AGENTS.md",
		"internal/canon/assets/base/AGENTS.md.tmpl",
		"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl",
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		assertAGENTSRules(t, path, string(content))
	}
	for _, path := range []string{
		"govna/development-cycle.md",
		"internal/canon/assets/overlays/code/files/govna/development-cycle.md.tmpl",
		"internal/canon/assets/overlays/doc/files/govna/editing-cycle.md.tmpl",
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		assertCycleRules(t, path, string(content))
	}

	for _, variant := range []struct {
		name      string
		config    Config
		cyclePath string
	}{
		{name: "CODE", config: Config{Flavor: Code, RepoName: "widget", Stack: "Rust"}, cyclePath: "govna/development-cycle.md"},
		{name: "DOC", config: Config{Flavor: Doc, RepoName: "handbook"}, cyclePath: "govna/editing-cycle.md"},
	} {
		t.Run(variant.name, func(t *testing.T) {
			files, err := Render(variant.config)
			if err != nil {
				t.Fatal(err)
			}
			assertAGENTSRules(t, variant.name+" AGENTS.md", fileText(t, files, "AGENTS.md"))
			assertCycleRules(t, variant.name+" "+variant.cyclePath, fileText(t, files, variant.cyclePath))
		})
	}
}

func TestPhaseAuthorizationMatrix(t *testing.T) {
	const refineRule = "- Exempt integrated audit adoption, eligible automatic Refine entry, and an eligible bounded completeness correction from a fresh Refine action instruction."
	const implementRule = "- Exempt only an eligible bounded completeness correction and an eligible decision re-entry from a fresh Implement action instruction."
	const retiredRule = "Exempt only an eligible bounded completeness correction under ### Four-Phase Workflow from a fresh Refine or Implement action instruction."

	root := filepath.Join("..", "..")
	contracts := []struct {
		name string
		text string
	}{}
	for _, path := range []string{
		"AGENTS.md",
		"internal/canon/assets/base/AGENTS.md.tmpl",
		"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl",
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		contracts = append(contracts, struct {
			name string
			text string
		}{name: path, text: string(content)})
	}
	for _, variant := range []struct {
		name   string
		config Config
	}{
		{name: "rendered CODE", config: Config{Flavor: Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"}},
		{name: "rendered DOC", config: Config{Flavor: Doc, RepoName: "handbook"}},
	} {
		files, err := Render(variant.config)
		if err != nil {
			t.Fatal(err)
		}
		contracts = append(contracts, struct {
			name string
			text string
		}{name: variant.name, text: fileText(t, files, "AGENTS.md")})
	}

	scenarios := []struct {
		name          string
		token         string
		wantRefine    bool
		wantImplement bool
	}{
		{name: "hand-authored AC", token: "hand-authored AC", wantRefine: false, wantImplement: false},
		{name: "integrated audit adoption", token: "integrated audit adoption", wantRefine: true, wantImplement: false},
		{name: "automatic Refine entry", token: "eligible automatic Refine entry", wantRefine: true, wantImplement: false},
		{name: "bounded completeness correction", token: "eligible bounded completeness correction", wantRefine: true, wantImplement: true},
	}
	guards := []string{
		"- Require fresh approval for every new action.",
		"- Treat an explicit request to run govna audit as authorization for integrated audit adoption under ### Audit Adoption.",
		"- Pause after Audit until the Director requests Refine unless integrated audit adoption or eligible automatic Refine entry applies.",
		"- Pause after Refine and await explicit Director implementation-ready confirmation to Implement, except for an eligible bounded completeness correction or an eligible decision re-entry.",
		"- Apply the bounded completeness exception only after the Director authorizes Implement.",
		"- Prevent the exception and a decision re-entry from authorizing initial Implement, Audit, Ratify, Package, release preparation, publication, delegation, or commits.",
		"- Treat only explicit Director action language as authorization to enter the named next action.",
		"- Treat a compound request as authorization for only the named action.",
		"- Pause before any unnamed action.",
		"- Stop integrated audit adoption before Implement.",
	}

	for _, contract := range contracts {
		t.Run(contract.name, func(t *testing.T) {
			normalized := strings.ReplaceAll(contract.text, "`", "")
			_, after, ok := strings.Cut(normalized, "### Phase-Advancement Rules\n")
			if !ok {
				t.Fatal("missing Phase-Advancement Rules")
			}
			section := after
			if end := strings.Index(section, "\n### "); end >= 0 {
				section = section[:end]
			}
			for _, rule := range []string{refineRule, implementRule} {
				if count := strings.Count(section, rule); count != 1 {
					t.Errorf("phase exception count=%d, want 1: %s", count, rule)
				}
			}
			if strings.Contains(section, retiredRule) {
				t.Errorf("retains combined phase exception: %s", retiredRule)
			}
			for _, guard := range guards {
				if count := strings.Count(normalized, guard); count != 1 {
					t.Errorf("authorization guard count=%d, want 1: %s", count, guard)
				}
			}
			for _, scenario := range scenarios {
				if got := strings.Contains(refineRule, scenario.token); got != scenario.wantRefine {
					t.Errorf("%s Refine authorization=%t, want %t", scenario.name, got, scenario.wantRefine)
				}
				if got := strings.Contains(implementRule, scenario.token); got != scenario.wantImplement {
					t.Errorf("%s Implement authorization=%t, want %t", scenario.name, got, scenario.wantImplement)
				}
			}
		})
	}
}

func TestProductToolingStackBoundaries(t *testing.T) {
	goFiles, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"})
	if err != nil {
		t.Fatal(err)
	}
	goBuild := fileText(t, goFiles, "build.sh")
	for _, marker := range []string{"refresh-validation-token", "v2:%s:%s:%s", "Validation token:"} {
		if strings.Contains(goBuild, marker) {
			t.Errorf("rendered Go build unexpectedly contains %q", marker)
		}
	}
	for _, helper := range []string{"_print_coverage_summary()", "_domain_coverage()", "_extract_program_version()", "_is_strict_stable_semver()", "_pin_go_toolchain()", "_staticcheck_failure_hint()"} {
		if strings.Count(goBuild, helper) != 1 {
			t.Errorf("rendered Go build helper %q count=%d", helper, strings.Count(goBuild, helper))
		}
	}
	if !strings.Contains(goBuild, "export GOTOOLCHAIN=") {
		t.Fatal("rendered Go build does not export the go.mod toolchain pin")
	}
	if strings.Count(goBuild, "honnef.co/go/tools/cmd/staticcheck@v0.8.2") != 1 || strings.Count(goBuild, "cmd/staticcheck@v") != 1 {
		t.Fatal("rendered Go build does not use the repository-governed Staticcheck pin")
	}
	if !strings.Contains(goBuild, "validation-token options are unsupported for Go") {
		t.Fatal("rendered Go build does not reject validation-token input")
	}
	for _, marker := range []string{
		"_capture_worktree_tree()",
		"Compile release utilities",
		"go build -mod=readonly -buildvcs=true -o \"$compiled\" -ldflags '-s -w' \"./cmd/$target\"",
		"resulting paths differ from the planned transformations",
	} {
		if !strings.Contains(goBuild, marker) {
			t.Errorf("rendered Go release tooling omits %q", marker)
		}
	}
	for _, retired := range []string{"prep: running pre-check build", "prep: running post-check build", "--no-build, -B", "-v, --verbose    print detailed prep"} {
		if strings.Contains(goBuild, retired) {
			t.Errorf("rendered Go prep retains %q", retired)
		}
	}
	for _, rootOnly := range []string{"internal/canon.Version", "cmd/govna canonVersion", "_literal_const_value()", "_validate_root_canon_version()", "_release_rebuild_and_verify()"} {
		if strings.Contains(goBuild, rootOnly) {
			t.Errorf("rendered Go consumer contains root-only marker %q", rootOnly)
		}
	}

	rustFiles, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: "Rust"})
	if err != nil {
		t.Fatal(err)
	}
	rustBuild := fileText(t, rustFiles, "build.sh")
	if !strings.Contains(rustBuild, "refresh-validation-token") || strings.Contains(rustBuild, "cmd/govna canonVersion") {
		t.Fatal("rendered Rust token behavior or canon-version boundary changed")
	}
	if strings.Contains(rustBuild, "Compile release utilities") || strings.Contains(rustBuild, "govna-go-release.XXXXXX") {
		t.Fatal("rendered Rust build gained Go-only release compilation")
	}

	goAgents := fileText(t, goFiles, "AGENTS.md")
	rustAgents := fileText(t, rustFiles, "AGENTS.md")
	if strings.Contains(goAgents, "Rust prep") || !strings.Contains(rustAgents, "Rust prep") || strings.Contains(rustAgents, "CODE stack provides no validation-token support") {
		t.Fatal("rendered CODE governance is not Rust-token-specific")
	}

	docFiles, err := Render(Config{Flavor: Doc, RepoName: "handbook"})
	if err != nil {
		t.Fatal(err)
	}
	docBuild := fileText(t, docFiles, "build.sh")
	if strings.Contains(docBuild, "validation-token") || strings.Contains(docBuild, "Validation token:") {
		t.Fatal("DOC build unexpectedly gained validation-token behavior")
	}
}

func fileText(t *testing.T, files []File, path string) string {
	t.Helper()
	for _, file := range files {
		if file.Path == path {
			return string(file.Content)
		}
	}
	t.Fatalf("missing %s", path)
	return ""
}

// sharedInvariantRules names the rule bullets that stay in both AGENTS.md and
// an owning canon document because audit content-coherence checks require them
// in the rendered overlay document.
var sharedInvariantRules = map[string]bool{}

type dedupProfile struct {
	name      string
	config    Config
	cyclePath string
}

// dedupProfiles lists every rendered profile with its cycle document path.
func dedupProfiles() []dedupProfile {
	profiles := []dedupProfile{{name: "DOC", config: Config{Flavor: Doc, RepoName: "handbook"}, cyclePath: "govna/editing-cycle.md"}}
	for _, stack := range Stacks() {
		config := Config{Flavor: Code, RepoName: "widget", Stack: stack}
		if stack == "Go" {
			config.ModulePath = "example.com/widget"
		}
		profiles = append(profiles, dedupProfile{name: "CODE/" + stack, config: config, cyclePath: "govna/development-cycle.md"})
		// A parent that declares a child renders the child profile text too.
		config.Children = true
		profiles = append(profiles, dedupProfile{name: "CODE/" + stack + "+children", config: config, cyclePath: "govna/development-cycle.md"})
		// A repository with an installable utility renders the command-line rules.
		if CanHaveUtilities(stack) {
			config.Children = false
			config.Utilities = true
			profiles = append(profiles, dedupProfile{name: "CODE/" + stack + "+utilities", config: config, cyclePath: "govna/development-cycle.md"})
		}
	}
	return profiles
}

// ruleLinePattern matches a bullet at any indent or a numbered step.
var ruleLinePattern = regexp.MustCompile(`^\s*(?:- |\d+\. )(.+)$`)

// ruleWordPattern matches one content word of a normalized rule.
var ruleWordPattern = regexp.MustCompile(`[a-z0-9.\-/<>]+`)

// ruleStopWords lists words ignored when comparing rule content. Negation and
// ordering words stay, so rules that differ only by them are not near duplicates.
var ruleStopWords = map[string]bool{}

func init() {
	for word := range strings.FieldsSeq("a an the and or of to in on for with by as at from each every any all is are be this that its it their when under into than one") {
		ruleStopWords[word] = true
	}
}

// normalizeRule lowercases a rule and strips markup and trailing punctuation.
func normalizeRule(rule string) string {
	rule = strings.ToLower(rule)
	rule = strings.NewReplacer("`", "", "*", "").Replace(rule)
	return strings.TrimRight(strings.TrimSpace(rule), ".:;")
}

// codeSpanOnly matches a bullet that is a single code span, such as a path.
var codeSpanOnly = regexp.MustCompile("^`[^`]+`$")

// ruleTexts returns the bullet and numbered-step texts in content, stopping at
// boundary when it is non-empty. A bullet that is a single code span is skipped.
func ruleTexts(content, boundary string) []string {
	var rules []string
	for line := range strings.SplitSeq(content, "\n") {
		if boundary != "" && line == boundary {
			break
		}
		if match := ruleLinePattern.FindStringSubmatch(line); match != nil && !codeSpanOnly.MatchString(strings.TrimSpace(match[1])) {
			rules = append(rules, match[1])
		}
	}
	return rules
}

// canonZoneRules collects normalized rules above the repo-owned boundary.
func canonZoneRules(content, boundary string) map[string]bool {
	rules := map[string]bool{}
	for _, rule := range ruleTexts(content, boundary) {
		rules[normalizeRule(rule)] = true
	}
	return rules
}

// tailRules collects normalized rules below the repo-owned boundary.
func tailRules(content, boundary string) map[string]bool {
	_, tail, ok := strings.Cut(content, boundary+"\n")
	if !ok {
		return map[string]bool{}
	}
	return canonZoneRules(tail, "")
}

// repeatedRules reports every document rule that repeats an AGENTS.md rule or
// a rule from another document.
func repeatedRules(agents map[string]bool, documents map[string]map[string]bool) []string {
	paths := make([]string, 0, len(documents))
	for path := range documents {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var repeats []string
	owner := map[string]string{}
	for _, path := range paths {
		rules := make([]string, 0, len(documents[path]))
		for rule := range documents[path] {
			rules = append(rules, rule)
		}
		sort.Strings(rules)
		for _, rule := range rules {
			switch {
			case agents[rule] && !sharedInvariantRules[rule]:
				repeats = append(repeats, fmt.Sprintf("%s repeats an AGENTS.md rule: %s", path, rule))
			case owner[rule] != "":
				repeats = append(repeats, fmt.Sprintf("%s repeats a %s rule: %s", path, owner[rule], rule))
			default:
				owner[rule] = path
			}
		}
	}
	return repeats
}

// governanceRule is one rule with the file it came from.
type governanceRule struct {
	path, text string
	words      map[string]bool
}

// contentWords returns the normalized content words of a rule.
func contentWords(rule string) map[string]bool {
	words := map[string]bool{}
	for _, word := range ruleWordPattern.FindAllString(normalizeRule(rule), -1) {
		if !ruleStopWords[word] {
			words[word] = true
		}
	}
	return words
}

// nearDuplicateThreshold is the Jaccard similarity at which two rules repeat.
const nearDuplicateThreshold = 0.8

// guardWords lists negation and ordering words. Two rules whose guard words
// differ state different permissions or orderings, however similar the rest.
var guardWords = map[string]bool{"not": true, "no": true, "never": true, "only": true, "without": true, "unless": true, "before": true, "after": true}

// sameGuards reports whether two rules carry the same guard words.
func sameGuards(a, b map[string]bool) bool {
	for word := range guardWords {
		if a[word] != b[word] {
			return false
		}
	}
	return true
}

// nearDuplicates reports every rule pair with the same guard words whose
// content-word Jaccard similarity reaches nearDuplicateThreshold.
func nearDuplicates(rules []governanceRule) []string {
	var pairs []string
	for i := range rules {
		for j := i + 1; j < len(rules); j++ {
			a, b := rules[i].words, rules[j].words
			if !sameGuards(a, b) {
				continue
			}
			shared := 0
			for word := range a {
				if b[word] {
					shared++
				}
			}
			union := len(a) + len(b) - shared
			if union > 0 && float64(shared)/float64(union) >= nearDuplicateThreshold {
				pairs = append(pairs, fmt.Sprintf("%s: %s\n  %s: %s", rules[i].path, rules[i].text, rules[j].path, rules[j].text))
			}
		}
	}
	return pairs
}

// renderedGovernanceRules collects the canon rules of AGENTS.md and every
// rendered govna/*.md document.
func renderedGovernanceRules(files []File) []governanceRule {
	var rules []governanceRule
	for _, file := range files {
		boundary := ""
		switch {
		case file.Path == "AGENTS.md":
			boundary = "## Project Rules"
		case strings.HasPrefix(file.Path, "govna/") && strings.HasSuffix(file.Path, ".md"):
			boundary = "## Project Practices"
		default:
			continue
		}
		for _, text := range ruleTexts(string(file.Content), boundary) {
			if words := contentWords(text); len(words) >= 3 {
				rules = append(rules, governanceRule{path: file.Path, text: text, words: words})
			}
		}
	}
	return rules
}

func TestCanonDocumentsDoNotRepeatAGENTSRules(t *testing.T) {
	for _, profile := range dedupProfiles() {
		t.Run(profile.name, func(t *testing.T) {
			files, err := Render(profile.config)
			if err != nil {
				t.Fatal(err)
			}
			agents := canonZoneRules(fileText(t, files, "AGENTS.md"), "## Project Rules")
			documents := map[string]map[string]bool{}
			for _, file := range files {
				if file.Path == "AGENTS.md" || !strings.HasSuffix(file.Path, ".md") {
					continue
				}
				documents[file.Path] = canonZoneRules(string(file.Content), "## Project Practices")
			}
			for _, repeat := range repeatedRules(agents, documents) {
				t.Error(repeat)
			}
		})
	}
}

func TestRepositoryTailsDoNotRepeatAGENTSRules(t *testing.T) {
	root := filepath.Join("..", "..")
	read := func(path string) string {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	agentsText := read("AGENTS.md")
	agents := canonZoneRules(agentsText, "## Project Rules")
	tails := map[string]map[string]bool{
		"AGENTS.md":                       tailRules(agentsText, "## Project Rules"),
		"govna/build-release.md":          tailRules(read("govna/build-release.md"), "## Project Practices"),
		"govna/development-guidelines.md": tailRules(read("govna/development-guidelines.md"), "## Project Practices"),
	}
	for path, rules := range tails {
		for rule := range rules {
			if agents[rule] {
				t.Errorf("%s repo-owned tail repeats an AGENTS.md rule: %s", path, rule)
			}
		}
	}
}

func TestCanonRulesHaveNoNearDuplicates(t *testing.T) {
	for _, profile := range dedupProfiles() {
		t.Run(profile.name, func(t *testing.T) {
			files, err := Render(profile.config)
			if err != nil {
				t.Fatal(err)
			}
			for _, pair := range nearDuplicates(renderedGovernanceRules(files)) {
				t.Errorf("near-duplicate rules:\n  %s", pair)
			}
		})
	}
}

func TestRepeatDetectorsCatchInjectedRepeats(t *testing.T) {
	agents := canonZoneRules("- Keep the closure-audit working record out of chat replies.\n", "")
	indented := canonZoneRules("1. **Step.**\n   - Keep the closure-audit working record out of chat replies.\n", "## Project Practices")
	if repeats := repeatedRules(agents, map[string]map[string]bool{"govna/a.md": indented}); len(repeats) != 1 {
		t.Errorf("indented AGENTS.md repeat reports=%q, want one", repeats)
	}
	documents := map[string]map[string]bool{
		"govna/a.md": canonZoneRules("- Verify every offered routing outcome is executable.\n", ""),
		"govna/b.md": canonZoneRules("- Verify every offered routing outcome is `executable`\n", ""),
	}
	if repeats := repeatedRules(map[string]bool{}, documents); len(repeats) != 1 {
		t.Errorf("cross-document repeat reports=%q, want one", repeats)
	}
	short := canonZoneRules("- Skip preamble summaries.\n", "")
	if repeats := repeatedRules(short, map[string]map[string]bool{"govna/roles.md": short}); len(repeats) != 1 {
		t.Errorf("short-rule repeat reports=%q, want one", repeats)
	}
	if paths := canonZoneRules("- `AGENTS.md`\n- `plan.md`\n", ""); len(paths) != 0 {
		t.Errorf("file-list bullets treated as rules: %v", paths)
	}
	paraphrase := []governanceRule{
		{path: "AGENTS.md", text: "Keep the emitted AC and consumer repository unchanged during integrated Audit and Refine."},
		{path: "govna/audit.md", text: "Keep the emitted AC and consumer repository unchanged during Audit and Refine."},
	}
	for index := range paraphrase {
		paraphrase[index].words = contentWords(paraphrase[index].text)
	}
	if pairs := nearDuplicates(paraphrase); len(pairs) != 1 {
		t.Errorf("paraphrase reports=%q, want one", pairs)
	}
	longParaphrase := []governanceRule{
		{path: "govna/build-release.md", text: "Validate every compiled utility result before release-metadata writes in every supported stack."},
		{path: "govna/canon-spec.md", text: "Validate each compiled utility result before release-metadata writes in every supported stack."},
	}
	for index := range longParaphrase {
		longParaphrase[index].words = contentWords(longParaphrase[index].text)
	}
	if pairs := nearDuplicates(longParaphrase); len(pairs) != 1 {
		t.Errorf("long paraphrase with matching guards reports=%q, want one", pairs)
	}
	distinct := []governanceRule{
		{path: "govna/roles.md", text: "Do not negotiate scope questions without the director in the loop."},
		{path: "govna/roles.md", text: "Do not resolve scope questions without the director in the loop."},
		{path: "AGENTS.md", text: "Run `./build.sh` before prep."},
		{path: "AGENTS.md", text: "Run `./build.sh` after prep."},
		{path: "AGENTS.md", text: "Treat the snapshot as current."},
		{path: "AGENTS.md", text: "Treat the snapshot as not current."},
		{path: "govna/canon-spec.md", text: "Validate every compiled utility's help output before installation during release compilation."},
		{path: "govna/canon-spec.md", text: "Validate every compiled utility's help output after installation during release compilation."},
		{path: "AGENTS.md", text: "Use isolated binary smoke commands before validation only when they do not claim repository health."},
		{path: "AGENTS.md", text: "Use isolated binary smoke commands before validation when they do not claim repository health."},
		{path: "AGENTS.md", text: "Rerun applicable validation before prep when Package evidence is missing or stale for the release batch."},
		{path: "AGENTS.md", text: "Rerun applicable validation before prep unless Package evidence is missing or stale for the release batch."},
	}
	for index := range distinct {
		distinct[index].words = contentWords(distinct[index].text)
	}
	if pairs := nearDuplicates(distinct); len(pairs) != 0 {
		t.Errorf("distinct rules reported as near duplicates: %q", pairs)
	}
}

func TestPrepAppendixDescribesEveryStack(t *testing.T) {
	headingRef := regexp.MustCompile("`#{2,3} [^`]*`")
	for _, stack := range Stacks() {
		t.Run(stack, func(t *testing.T) {
			config := Config{Flavor: Code, RepoName: "widget", Stack: stack}
			if stack == "Go" {
				config.ModulePath = "example.com/widget"
			}
			if _, err := Render(config); err != nil {
				t.Fatal(err)
			}
			_, appendix, ok := strings.Cut(repoFile(t, "govna/canon-spec.md"), "## What prep does\n\n")
			if !ok {
				t.Fatal("govna/canon-spec.md omits the prep description")
			}
			opening, _, _ := strings.Cut(appendix, "\n\n")
			if strings.Contains(opening, "Go `./build.sh prep` performs bookkeeping only") {
				t.Errorf("appendix describes every stack as Go bookkeeping: %s", opening)
			}
			for _, want := range []string{
				"`./build.sh prep` performs release bookkeeping in every CODE stack.",
				"Go and Terraform prep perform bookkeeping only.",
				"Rust prep adds a fallback pre-change full build when validation evidence is missing or stale and one post-change full build after its writes.",
				"Swift prep runs the canonical build before its writes and again with installation after them unless `-B` skips both.",
			} {
				if !strings.Contains(opening, want) {
					t.Errorf("appendix opening omits %q: %s", want, opening)
				}
			}
			if ref := headingRef.FindString(opening); ref != "" {
				t.Errorf("appendix opening references heading %s", ref)
			}
		})
	}
}

func TestCanonAGENTSReferencesResolve(t *testing.T) {
	headingRef := regexp.MustCompile("`(#{2,3} [^`]+)`")
	for _, profile := range dedupProfiles() {
		t.Run(profile.name, func(t *testing.T) {
			files, err := Render(profile.config)
			if err != nil {
				t.Fatal(err)
			}
			headings := map[string]bool{}
			for line := range strings.SplitSeq(fileText(t, files, "AGENTS.md"), "\n") {
				if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
					headings[line] = true
				}
			}
			required := map[string][]string{
				profile.cyclePath: {"### Four-Phase Workflow", "### Phase-Advancement Rules", "### Audit Adoption"},
				"govna/audit.md":  {"### Audit Adoption"},
				"govna/roles.md":  {"## Interaction Mode", "## Review Style"},
			}
			for path, wants := range required {
				found := map[string]bool{}
				for line := range strings.SplitSeq(fileText(t, files, path), "\n") {
					if !strings.Contains(line, "AGENTS.md") {
						continue
					}
					for _, match := range headingRef.FindAllStringSubmatch(line, -1) {
						if !headings[match[1]] {
							t.Errorf("%s references missing AGENTS.md heading %q", path, match[1])
						}
						found[match[1]] = true
					}
				}
				for _, want := range wants {
					if !found[want] {
						t.Errorf("%s does not reference AGENTS.md %s", path, want)
					}
				}
			}
		})
	}
}

func TestCheckModeDocumentation(t *testing.T) {
	const stackLine = "- `-s, --stack <name>` — CODE stack (default: inferred from manifests; not accepted with `--flavor doc`).\n"
	const checkLine = "- `-c, --check` — report the result without writing an AC; exit `3` when updates or Director choices are needed.\n"
	const checkParagraph = "A check-mode audit (`-c` or `--check`) runs the same comparison and writes nothing. It performs no AC-number allocation, stub inspection, directory creation, or file write. A clean result prints the clean-result line above and exits `0`. An actionable result prints `Govna updates or Director choices found`, followed by a plain result tally and `Run govna audit without --check to write the review AC.`, and exits `3`. With `--json`, the complete report is printed with `emitted` set to `null` and no additional prose. Because check mode emits no AC, it starts no agent-mediated adoption review."
	const legacyPurposeDefinition = "Govna exists to make programming and publishing ceremonies—the recurring CODE and DOC checkpoints around intent, authorization, scope, review, implementation or editing, verification, and release—more effective and efficient."
	const phaseSentence = "A clean result, a check-mode result, or a pre-emission failure enters no AC phase."
	const readmeUsage = "Use `--json` to emit the deterministic machine report alongside the Markdown result. Use `--check` (`-c`) to report the result without writing anything; it exits `3` when updates or Director choices are needed and `0` when none are."
	const archSentence = "Actionable audits write or reuse one unedited AC keyed by canon version. Check mode writes nothing for either result and exits 3 when updates or Director choices are needed."
	documents := map[string]map[string]string{}
	for name, config := range map[string]Config{
		"CODE": {Flavor: Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"},
		"DOC":  {Flavor: Doc, RepoName: "handbook"},
	} {
		files, err := Render(config)
		if err != nil {
			t.Fatal(err)
		}
		documents[name] = map[string]string{}
		for _, path := range []string{"govna/audit.md", "README.md"} {
			documents[name][path] = fileText(t, files, path)
		}
	}
	root := filepath.Join("..", "..")
	documents["repository"] = map[string]string{}
	for _, path := range []string{"govna/audit.md", "README.md", "arch.md"} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		documents["repository"][path] = string(content)
	}
	for name, docs := range documents {
		if count := strings.Count(docs["govna/audit.md"], stackLine+checkLine); count != 1 {
			t.Errorf("%s govna/audit.md check flag line after stack line count=%d, want 1", name, count)
		}
		// The check-mode paragraph specifies the tool, so no consumer receives it.
		if count := strings.Count(docs["govna/audit.md"], checkParagraph); count != 0 {
			t.Errorf("%s govna/audit.md check-mode paragraph count=%d, want 0", name, count)
		}
		if count := strings.Count(repoFile(t, "govna/canon-spec.md"), checkParagraph); count != 1 {
			t.Errorf("govna/canon-spec.md check-mode paragraph count=%d, want 1", count)
		}
		rationale := documents["repository"]["arch.md"]
		if count := strings.Count(rationale, efficiencyPurposeDefinition); count != 1 || strings.Contains(rationale, legacyPurposeDefinition) {
			t.Errorf("%s arch.md purpose definition count=%d, legacy=%t", name, count, strings.Contains(rationale, legacyPurposeDefinition))
		}
		if count := strings.Count(docs["README.md"], phaseSentence); count != 1 {
			t.Errorf("%s README.md check-mode phase sentence count=%d, want 1", name, count)
		}
	}
	if !strings.Contains(documents["repository"]["README.md"], readmeUsage) {
		t.Error("README.md omits the --check usage sentence after the --json sentence")
	}
	if !strings.Contains(documents["repository"]["arch.md"], archSentence) {
		t.Error("arch.md omits the check-mode sentence after the actionable-audit sentence")
	}
}

func TestDocReleaseAcceptsDatedVersions(t *testing.T) {
	docFiles, err := Render(Config{Flavor: Doc, RepoName: "handbook"})
	if err != nil {
		t.Fatal(err)
	}
	goFiles, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"})
	if err != nil {
		t.Fatal(err)
	}
	docBuild := fileText(t, docFiles, "build.sh")
	goBuild := fileText(t, goFiles, "build.sh")
	for _, helper := range []string{"_is_dated_version()", "_is_release_version()"} {
		if got := strings.Count(docBuild, helper); got != 1 {
			t.Errorf("rendered DOC build helper %q count=%d", helper, got)
		}
		if strings.Contains(goBuild, helper) {
			t.Errorf("rendered Go build unexpectedly contains %q", helper)
		}
	}
	for path, wants := range map[string][]string{
		"AGENTS.md": {
			"- Use strict semver `vMAJOR.MINOR.PATCH` or the dated form `vYY.MM.DDa` for every release tag.\n",
			"- Keep one version scheme per repository.\n",
			"- Assign `a` to the first dated release of a day and the next letter to each later one.\n",
		},
		"govna/release.md": {
			"Accept strict semver `vMAJOR.MINOR.PATCH` or the dated form `vYY.MM.DDa` as the release tag.\n",
			"- Versions are unprefixed (`0.29.0` or `26.10.10a`, not `v0.29.0`).\n",
		},
	} {
		text := fileText(t, docFiles, path)
		for _, want := range wants {
			if got := strings.Count(text, want); got != 1 {
				t.Errorf("rendered DOC %s count=%d for %q", path, got, want)
			}
		}
	}
	if strings.Contains(fileText(t, goFiles, "AGENTS.md"), "dated form") {
		t.Error("rendered CODE AGENTS.md mentions the dated version form")
	}

	// The semver bullets apply only under the semver scheme in DOC; CODE stays semver-only.
	scoped := []string{
		"- Use PATCH for invisible changes and MINOR for user-visible changes under semver.\n",
		"- Batch PATCH-level changes under semver.\n",
	}
	unscoped := []string{
		"- Follow semver with PATCH for invisible changes and MINOR for user-visible changes.\n",
		"- Batch PATCH-level changes.\n",
	}
	docAgents := fileText(t, docFiles, "AGENTS.md")
	goAgents := fileText(t, goFiles, "AGENTS.md")
	for i := range scoped {
		if got := strings.Count(docAgents, scoped[i]); got != 1 {
			t.Errorf("rendered DOC AGENTS.md count=%d for %q", got, scoped[i])
		}
		if strings.Contains(docAgents, unscoped[i]) {
			t.Errorf("rendered DOC AGENTS.md keeps the unscoped bullet %q", unscoped[i])
		}
		if got := strings.Count(goAgents, unscoped[i]); got != 1 {
			t.Errorf("rendered CODE AGENTS.md count=%d for %q", got, unscoped[i])
		}
		if strings.Contains(goAgents, scoped[i]) {
			t.Errorf("rendered CODE AGENTS.md gained the DOC-scoped bullet %q", scoped[i])
		}
	}
	release := fileText(t, docFiles, "govna/release.md")
	steps := []string{
		"   - Use today's zero-padded `YY.MM.DD` for a dated version.\n",
		"   - Append `a` for the first dated release of the day and the next letter for each later one.\n",
		"   - Classify a semver change set as PATCH for formatting, fixes, or user-invisible refactors.\n",
		"   - Classify a semver change set as MINOR for user-visible structure, navigation, or schema changes.\n",
		"   - Bump the semver version from the latest tag.\n",
	}
	last := -1
	for _, step := range steps {
		if got := strings.Count(release, step); got != 1 {
			t.Errorf("rendered DOC govna/release.md count=%d for %q", got, step)
		}
		if at := strings.Index(release, step); at < last {
			t.Errorf("rendered DOC govna/release.md lists %q before an earlier step", step)
		} else {
			last = at
		}
	}
	if strings.Contains(release, "Classify the change set under semver.") {
		t.Error("rendered DOC govna/release.md keeps the unscoped classification step")
	}
}

func TestStaticcheckPinRulesRenderOnce(t *testing.T) {
	root := filepath.Join("..", "..")
	const stackRule = "- Name the pinned staticcheck, the active `GOTOOLCHAIN`, and both remedies when staticcheck cannot decode the toolchain's export data.\n"
	const practice = "- Bump the `go.mod` `go` directive and the `staticcheck` pin together, after the `staticcheck` release notes name the new Go version.\n"
	files, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"})
	if err != nil {
		t.Fatal(err)
	}
	type check struct{ name, text, want string }
	checks := []check{
		{"rendered govna/development-guidelines.md", fileText(t, files, "govna/development-guidelines.md"), practice},
	}
	for path, want := range map[string]string{
		"govna/canon-spec.md":                          stackRule,
		"govna/development-guidelines.md":              practice,
		"internal/canon/assets/stack-guidelines/go.md": practice,
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		checks = append(checks, check{path, string(content), want})
	}
	for _, c := range checks {
		if got := strings.Count(c.text, c.want); got != 1 {
			t.Errorf("%s count=%d for %q", c.name, got, c.want)
		}
	}
}

func TestGoToolchainPinRulesRenderOnce(t *testing.T) {
	root := filepath.Join("..", "..")
	const stackRule = "- Pin `GOTOOLCHAIN` in `build.sh` to the `go.mod` `toolchain` directive, or else to its `go` directive, before the first Go command.\n"
	const practice = "- Build with the Go toolchain that `go.mod` declares.\n"
	files, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"})
	if err != nil {
		t.Fatal(err)
	}
	type check struct{ name, text, want string }
	checks := []check{
		{"rendered govna/development-guidelines.md", fileText(t, files, "govna/development-guidelines.md"), practice},
	}
	for path, want := range map[string]string{
		"govna/canon-spec.md":                          stackRule,
		"govna/development-guidelines.md":              practice,
		"internal/canon/assets/stack-guidelines/go.md": practice,
	} {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		checks = append(checks, check{path, string(content), want})
	}
	for _, c := range checks {
		if got := strings.Count(c.text, c.want); got != 1 {
			t.Errorf("%s count=%d for %q", c.name, got, c.want)
		}
	}
}

func TestChildRenderPerStack(t *testing.T) {
	orphan, err := RenderOrphan()
	if err != nil || len(orphan) != 1 || orphan[0].Path != "AGENTS.md" {
		t.Fatalf("orphan render=%v err=%v", orphan, err)
	}
	for _, stack := range Stacks() {
		t.Run(stack, func(t *testing.T) {
			child, err := RenderChild(stack)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{".gitignore", "AGENTS.md", "CHANGELOG.md", "build.sh"}
			if stack == "Rust" {
				want = append(want, "tests/build_cli.sh")
			}
			got := make([]string, 0, len(child))
			for _, file := range child {
				got = append(got, file.Path)
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("child files=%v want=%v", got, want)
			}
			full, err := Render(Config{Flavor: Code, RepoName: "zq-private-parent", Stack: stack, ModulePath: "example.com/zq-private-parent"})
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range want[2:] {
				if fileText(t, child, path) != fileText(t, full, path) && path != "AGENTS.md" {
					t.Errorf("child %s differs from the parent's canon %s", path, path)
				}
			}
			if fileText(t, child, ".gitignore") != fileText(t, full, ".gitignore") {
				t.Error("child .gitignore differs from the parent's canon .gitignore")
			}
			pointer := fileText(t, child, "AGENTS.md")
			for _, text := range []string{pointer, string(orphan[0].Content)} {
				if strings.Contains(text, "zq-private-parent") || strings.Contains(text, "{{") {
					t.Fatalf("child agent file names a parent or keeps a placeholder:\n%s", text)
				}
				if !IsChildAgentFile([]byte(text)) {
					t.Fatal("child agent file is not recognized")
				}
			}
			if !strings.Contains(pointer, "- Stop when this session started in this repository.\n") {
				t.Fatalf("pointer does not limit its stop to a session started in the child:\n%s", pointer)
			}
			if IsChildAgentFile([]byte(fileText(t, full, "AGENTS.md"))) {
				t.Fatal("full canon AGENTS.md is mistaken for a child agent file")
			}
		})
	}
}

func TestChildRepositoryRulesRenderForCodeOnly(t *testing.T) {
	block := "### Child Repositories\n\n" +
		"- Apply these rules when `govna/children.txt` declares a child repository.\n" +
		"- Treat a declared child named in the active AC's `## In Scope` as primary scope instead of ancillary work.\n" +
		"- Exclude child files and child commit messages from every bare-identifier allowance in `### AC Mechanics`.\n" +
		"- Add each declared child as a writable session directory before editing it.\n" +
		"- Follow `govna/development-cycle.md` `## Child Repositories` for the session procedure.\n" +
		"- Change child membership only through `govna apply --child` and `govna rm --child`.\n"
	for _, stack := range Stacks() {
		files, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: stack, ModulePath: "example.com/widget", Children: true})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(fileText(t, files, "AGENTS.md"), block) {
			t.Errorf("%s AGENTS.md omits the child repository rules", stack)
		}
		cycle := fileText(t, files, "govna/development-cycle.md")
		for _, want := range []string{
			"- Add a child to a Claude Code session with `--add-dir <path>` at startup or `/add-dir <path>` during the session.\n",
			"- Add a child to a Codex CLI session with `--add-dir <path>` at startup.\n",
			"- Record the same identities for each in-scope child repository.\n",
		} {
			if !strings.Contains(cycle, want) {
				t.Errorf("%s development cycle omits %q", stack, want)
			}
		}
	}
	doc, err := Render(Config{Flavor: Doc, RepoName: "handbook"})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range doc {
		if strings.Contains(string(file.Content), "children.txt") || strings.Contains(string(file.Content), "Child Repositories") {
			t.Errorf("DOC %s mentions child repositories", file.Path)
		}
	}
}

func TestNotCheckedListsOnlyDirectorChecks(t *testing.T) {
	roleRules := []string{
		"- Run a check before reporting instead of listing it under `Not checked` when current authorization allows it.\n",
		"- List under `Not checked` only a check that needs a Director action or a new Director authorization.\n",
		"- Name that Director action or authorization in each `Not checked` item.\n",
		"\nNote: checks the Operator runs itself include a read-only command, a file inspection, or a render check that current authorization covers.\n",
	}
	const packageRule = "- State the remaining check that needs a Director action or a new Director authorization, or that none remains, under `Not checked:`.\n"
	configs := map[string]Config{"DOC": {Flavor: Doc, RepoName: "handbook"}}
	for _, stack := range Stacks() {
		configs["CODE/"+stack] = Config{Flavor: Code, RepoName: "widget", Stack: stack, ModulePath: "example.com/widget"}
	}
	for name, config := range configs {
		files, err := Render(config)
		if err != nil {
			t.Fatal(err)
		}
		selfReview := sectionText(t, fileText(t, files, "govna/roles.md"), "### Required Self-review\n")
		for _, rule := range roleRules {
			if count := strings.Count(selfReview, rule); count != 1 {
				t.Errorf("%s roles.md Required Self-review rule count=%d, want 1 for %q", name, count, rule)
			}
		}
		agents := fileText(t, files, "AGENTS.md")
		if count := strings.Count(sectionText(t, agents, "### Package Self-review\n"), packageRule); count != 1 {
			t.Errorf("%s AGENTS.md Package Self-review Not checked rule count=%d, want 1", name, count)
		}
		if strings.Contains(agents, "State the remaining manual check") {
			t.Errorf("%s AGENTS.md keeps the manual-check Not checked rule", name)
		}
	}
}

// sectionText returns text from heading up to the next ### or ## heading.
func sectionText(t *testing.T, text, heading string) string {
	t.Helper()
	start := strings.Index(text, heading)
	if start < 0 {
		t.Fatalf("missing heading %q", heading)
	}
	body := text[start+len(heading):]
	if end := strings.Index(body, "\n#"); end >= 0 {
		body = body[:end+1]
	}
	return body
}

func TestGoWorkspaceRecipeSetsTheGoModGoLine(t *testing.T) {
	const recipe = "- Use `go work init . <child>` for a child override.\n" +
		"- Set the `go.work` `go` line to the `go.mod` `go` line with `go work edit -go=<version>`.\n"
	files, err := Render(Config{Flavor: Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget", Children: true})
	if err != nil {
		t.Fatal(err)
	}
	if count := strings.Count(fileText(t, files, "govna/build-release.md"), recipe); count != 1 {
		t.Fatalf("build-release.md Go workspace recipe count=%d, want 1", count)
	}
}

// movedPlanRules lists the plan.md IE rules that live in the cycle
// document's ## Notes instead of the plan.md stub.
var movedPlanRules = []string{
	"- Separate `plan.md` IE items with one blank line.\n",
	"- Start each IE item with `- IE<N>: `.\n",
	"- Number IE items sequentially.\n",
	"- Write an idea IE as `- IE<N>: <one-liner>`.\n",
	"- Hold an idea IE for Director discussion and the AC-ceremony evaluation in `AGENTS.md` `### AC-First Workflow`.\n",
	"- Write an AC-pointer IE as `- IE<N>: <one-liner> → govna/ac<N>-<slug>.md`.\n",
	"- Convert an idea IE to an AC-pointer IE when the Director sends it to Draft.\n",
	"- Keep the IE number through that conversion.\n",
}

// TestRenderedIESpacingRuleLivesInSyncedFiles checks that every profile keeps
// the IE spacing rule in its synced cycle document instead of the plan.md stub,
// commits to keeping rules out of the stubs, and documents audit's IE spacing.
func TestRenderedIESpacingRuleLivesInSyncedFiles(t *testing.T) {
	const rule = "- Separate `plan.md` IE items with one blank line.\n"
	configs := []Config{{Flavor: Doc, RepoName: "handbook"}}
	for _, stack := range Stacks() {
		config := Config{Flavor: Code, RepoName: "widget", Stack: stack}
		if stack == "Go" {
			config.ModulePath = "example.com/widget"
		}
		configs = append(configs, config)
	}
	for _, config := range configs {
		files, err := Render(config)
		if err != nil {
			t.Fatal(err)
		}
		cycle := "govna/development-cycle.md"
		if config.Flavor == Doc {
			cycle = "govna/editing-cycle.md"
		}
		notes := sectionText(t, fileText(t, files, cycle), "## Notes\n")
		if count := strings.Count(fileText(t, files, cycle), rule); count != 1 || !strings.Contains(notes, rule) {
			t.Errorf("%v %s %s holds the spacing rule %d times, want once under ## Notes", config.Flavor, config.Stack, cycle, count)
		}
		cycleText := fileText(t, files, cycle)
		for _, bullet := range append(append([]string{}, movedPlanRules...), "- Remove an IE when rejected, retired, or shipped through its AC pointer.\n") {
			if count := strings.Count(cycleText, bullet); count != 1 || !strings.Contains(notes, bullet) {
				t.Errorf("%v %s %s holds %q %d times, want once under ## Notes", config.Flavor, config.Stack, cycle, bullet, count)
			}
		}
		if strings.Contains(cycleText, "Keep historical records out of") {
			t.Errorf("%v %s %s still bans historical records in plan.md", config.Flavor, config.Stack, cycle)
		}
		plan := fileText(t, files, "plan.md")
		for _, gone := range []string{"one blank line separates items", "sequential N", "Two kinds", "**idea IE**", "**AC-pointer**", "converts to an AC-pointer", "Remove entries", "Not a historical record"} {
			if strings.Contains(plan, gone) {
				t.Errorf("%v %s plan.md stub still holds %q", config.Flavor, config.Stack, gone)
			}
		}
		preamble := "\n## Ideas To Explore\n\nIdeas captured for future reference. [`" + cycle + "`](" + cycle + ") `## Notes` holds the rules for these items.\n"
		if !strings.Contains(plan, preamble) {
			t.Errorf("%v %s plan.md lacks the linked preamble %q", config.Flavor, config.Stack, preamble)
		}
		const commitment = "5. **Stub rules.**\n   - Keep rules out of the `plan.md` and `arch.md` stubs.\n   - Place each such rule in a synced canon file.\n"
		if count := strings.Count(repoFile(t, "govna/canon-maintenance.md"), commitment); count != 1 {
			t.Errorf("%v %s canon-maintenance.md holds the stub-rules commitment %d times, want 1", config.Flavor, config.Stack, count)
		}
		audit := fileText(t, files, "govna/audit.md")
		for _, want := range []string{"\n## IE spacing\n", "`## IE spacing` names the one exception for `plan.md`."} {
			if count := strings.Count(repoFile(t, "govna/canon-spec.md"), want); count != 1 {
				t.Errorf("govna/canon-spec.md holds %q %d times, want 1", want, count)
			}
		}
		for _, want := range []string{"- Put exactly one blank line between each pair of adjacent IE items in a `plan.md` listed under `### IE items to space`.\n"} {
			if count := strings.Count(audit, want); count != 1 {
				t.Errorf("%v %s audit.md holds %q %d times, want 1", config.Flavor, config.Stack, want, count)
			}
		}
	}
}

// governanceText returns the rendered AGENTS.md and govna/*.md files by path.
func governanceText(files []File) map[string]string {
	out := map[string]string{}
	for _, file := range files {
		if file.Path == "AGENTS.md" || (strings.HasPrefix(file.Path, "govna/") && strings.HasSuffix(file.Path, ".md")) {
			out[file.Path] = string(file.Content)
		}
	}
	return out
}

// headingSection returns the text of one AGENTS.md section, from its heading
// to the next heading of the same or a higher level.
func headingSection(t *testing.T, text, heading string) string {
	t.Helper()
	_, after, ok := strings.Cut(text, "\n"+heading+"\n")
	if !ok {
		t.Fatalf("missing heading %q", heading)
	}
	end := len(after)
	stops := []string{"\n## "}
	if strings.HasPrefix(heading, "### ") {
		stops = append(stops, "\n### ")
	}
	for _, stop := range stops {
		if index := strings.Index(after, stop); index >= 0 && index < end {
			end = index
		}
	}
	return after[:end]
}

// embeddedText returns every embedded canon asset by path.
func embeddedText(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := fs.WalkDir(assets, "assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := fs.ReadFile(assets, path)
		if err != nil {
			return err
		}
		out[path] = string(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// audienceSplitRules lists each canon rule that the audience split added or
// reworded, by the AGENTS.md section that owns it.
func audienceSplitRules(cycle string) map[string][]string {
	return map[string][]string{
		"## Governed Sections": {
			"Add repository rules only under `## Project Rules`.",
		},
		"### Session Entry": {
			"Read `govna/roles.md` before the first substantive govna-governed action of a session.",
		},
		"### Rule Review": {
			"Run a rule-review after authorized governance edits and before Implement completion.",
			"Review each touched section for a rule to retire, merge, or enforce mechanically before adding a rule.",
			"Report the review result in one line only when it finds a rule to retire, merge, or enforce mechanically.",
			"Name each read trigger as a state or event that an agent can recognize without reading the target document.",
			"Label every proposal that loosens an approval or scope boundary.",
			"Treat a canon rule that this repository can never act on as rule-review evidence.",
			"Treat a rule that slowed or misdirected the active work, with an observed consequence, as rule-review evidence.",
			"Test representative triggers, authorization boundaries, allowed actions, exceptions, and exit conditions after governance restructuring.",
			"Record each governance-restructuring scenario input and expected outcome in the closure-audit record.",
		},
		"### Four-Phase Workflow": {
			"Read `" + cycle + "` `### Implement Evidence Snapshot` before capturing the snapshot.",
			"Read `" + cycle + "` `### Closure Audit` before the closure audit.",
			"Pause after Refine and await explicit Director implementation-ready confirmation to Implement, except for an eligible bounded completeness correction or an eligible decision re-entry.",
			"Define a decision re-entry as a return to Implement after a Refine that records only decisions the Director gave during that Implement.",
			"Require the Director's message to name each change that the Refine records.",
			"Require a clean Pre-Implementation Verification before a decision re-entry.",
			"Re-enter Implement automatically when a decision re-entry is eligible.",
			"Apply decision re-entry only after the Director authorizes Implement.",
			"Prevent the exception and a decision re-entry from authorizing initial Implement, Audit, Ratify, Package, release preparation, publication, delegation, or commits.",
		},
		"### Phase-Advancement Rules": {
			"Exempt only an eligible bounded completeness correction and an eligible decision re-entry from a fresh Implement action instruction.",
		},
		"### Agent Release": {
			"Run no other git or GitHub write under the keyword.",
		},
	}
}

func TestAudienceSplitRulesRenderOnce(t *testing.T) {
	for _, profile := range dedupProfiles() {
		t.Run(profile.name, func(t *testing.T) {
			files, err := Render(profile.config)
			if err != nil {
				t.Fatal(err)
			}
			agents := fileText(t, files, "AGENTS.md")
			homes := audienceSplitRules(profile.cyclePath)
			if profile.config.Children {
				homes["### Child Repositories"] = []string{"Exempt each child release from the `### Agent Release` limit on other git writes."}
			}
			for heading, rules := range homes {
				section := headingSection(t, agents, heading)
				for _, rule := range rules {
					line := "- " + rule + "\n"
					if strings.Count(agents, line) != 1 || !strings.Contains(section, line) {
						t.Errorf("AGENTS.md needs %q once, under %s", rule, heading)
					}
				}
			}
			if strings.Contains(agents, "### Contract Growth") || !strings.Contains(agents, "\n### Rule Review\n") {
				t.Error("AGENTS.md does not name the review section `### Rule Review`")
			}
		})
	}
}

// retiredAudienceSplitText lists wording that the audience split removed from
// every canon asset and every render.
var retiredAudienceSplitText = []string{
	"Preserve short atomic imperative instructions as the default.",
	"` `### Implement Evidence Snapshot` for snapshot contents and dependency boundaries.",
	"` `### Closure Audit` for the paths, documents, and records the closure audit maps.",
	"Exempt only an eligible bounded completeness correction from a fresh Implement action instruction.",
	"await explicit Director implementation-ready confirmation to Implement.\n",
	"Execute a release command only under `AGENTS.md` `### Agent Release`.",
	"Follow `govna/build-release.md` `## Child Releases` to release a child.",
	"Add new rules under the best-fit existing section.",
	"except each child release under `### Child Repositories`",
	"Contract Growth",
	"contract-growth",
	"code-stacks.md",
	"operator-contract-rationale",
}

func TestAudienceSplitRetiredTextIsGone(t *testing.T) {
	for path, text := range embeddedText(t) {
		for _, retired := range retiredAudienceSplitText {
			if strings.Contains(text, retired) {
				t.Errorf("asset %s retains %q", path, retired)
			}
		}
	}
	for _, profile := range dedupProfiles() {
		files, err := Render(profile.config)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if file.Path == "govna/code-stacks.md" || file.Path == "govna/operator-contract-rationale.md" {
				t.Errorf("%s renders the retired document %s", profile.name, file.Path)
			}
			for _, retired := range retiredAudienceSplitText {
				if strings.Contains(string(file.Content), retired) {
					t.Errorf("%s %s retains %q", profile.name, file.Path, retired)
				}
			}
			if strings.Contains(string(file.Content), "{{") && strings.HasSuffix(file.Path, ".md") {
				t.Errorf("%s %s keeps an unfilled placeholder", profile.name, file.Path)
			}
		}
	}
}

// canonMaintenanceRules lists the rules that only this repository follows.
var canonMaintenanceRules = []string{
	"Apply these rules to every canon change.",
	"Define consumer text as text that a consumer's agent or Director acts on.",
	"Classify each new or changed canon rule and document section as consumer, profile, or Govna-only before writing it.",
	"Keep Govna-only text out of every canon asset.",
	"Render profile text only into a repository whose profile needs it.",
	"Review each touched canon file for text that fails the consumer test.",
	"Record each profile candidate that a canon change leaves in place as a `plan.md` idea entry.",
	"Check that each touched pointer to an owning document states its trigger, its authority boundary, and the decision or action that waits.",
	"Confirm that each moved rule's text exists at its new location.",
	"Label each governance verification claim as an inventory check, a semantic review, observed behavior, or a mechanical check.",
	"Add each canon rule under the best-fit existing section.",
	"Preserve each section's semantic intent across edits.",
	"Edit sections in place.",
	"Change section order or the `##` section list only when the user explicitly requests a contract amendment.",
	"Name the exact sections to change during every update.",
	"Keep edits local during every update.",
	"Use `##` for top-level sections.",
	"Conform overlay templates and other canon files to AGENTS.md.",
	"Walk each affected scenario in `govna/canon-maintenance.md` `## Contract scenarios` before completing a canon change to a phase, approval, scope, or release rule.",
	"Add a scenario only for an observed failure with a distinct gap, or for a new or materially changed boundary.",
	"Give each new scenario a named gap and an expected outcome.",
}

func TestCanonMaintenanceRulesStayInThisRepository(t *testing.T) {
	_, tail, ok := strings.Cut(repoFile(t, "AGENTS.md"), "\n## Project Rules\n")
	if !ok {
		t.Fatal("AGENTS.md omits ## Project Rules")
	}
	section := headingSection(t, tail, "### Canon Maintenance")
	embedded := embeddedText(t)
	for _, rule := range canonMaintenanceRules {
		line := "- " + rule + "\n"
		if strings.Count(repoFile(t, "AGENTS.md"), line) != 1 || !strings.Contains(section, line) {
			t.Errorf("AGENTS.md needs %q once, under ### Canon Maintenance", rule)
		}
		for path, text := range embedded {
			if strings.Contains(text, line) {
				t.Errorf("asset %s carries the maintenance rule %q", path, rule)
			}
		}
	}
}

// maintenanceTerms name work that only this repository does.
var maintenanceTerms = []string{"candidate-canon", "state-equivalence", "overlay templates", "tombstone"}

func TestRendersHoldNoMaintenanceText(t *testing.T) {
	for _, profile := range dedupProfiles() {
		files, err := Render(profile.config)
		if err != nil {
			t.Fatal(err)
		}
		for path, text := range governanceText(files) {
			for _, term := range maintenanceTerms {
				if strings.Contains(text, term) {
					t.Errorf("%s %s holds the maintenance term %q", profile.name, path, term)
				}
			}
		}
	}
}

// profileLines returns the rule, heading, and table lines of the named
// profile assets.
func profileLines(t *testing.T, names ...string) []string {
	t.Helper()
	var lines []string
	for _, name := range names {
		content, err := fs.ReadFile(assets, "assets/profile/"+name)
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.SplitSeq(string(content), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "| ") || strings.HasPrefix(trimmed, "7. ") {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

// countLine counts the files' lines that equal line.
func countLine(texts map[string]string, line string) int {
	count := 0
	for _, text := range texts {
		count += strings.Count("\n"+text, "\n"+line+"\n")
	}
	return count
}

func TestChildProfileRendersOnlyForParents(t *testing.T) {
	var blocks []string
	for _, block := range childBlocks {
		blocks = append(blocks, block.asset)
	}
	for _, stack := range Stacks() {
		t.Run(stack, func(t *testing.T) {
			config := Config{Flavor: Code, RepoName: "widget", Stack: stack, ModulePath: "example.com/widget"}
			without, err := Render(config)
			if err != nil {
				t.Fatal(err)
			}
			config.Children = true
			with, err := Render(config)
			if err != nil {
				t.Fatal(err)
			}
			plain, parent := governanceText(without), governanceText(with)
			for _, line := range profileLines(t, blocks...) {
				if strings.Contains(line, "{{") {
					continue
				}
				if count := countLine(plain, line); count != 0 {
					t.Errorf("a render without children holds the child line %q", line)
				}
				if count := countLine(parent, line); count != 1 {
					t.Errorf("a render with children holds %q %d times, want 1", line, count)
				}
			}
			for _, heading := range []string{"### Child Repositories", "## Child Repositories", "## Child repositories", "## Child Releases"} {
				if countLine(plain, heading) != 0 {
					t.Errorf("a render without children holds %s", heading)
				}
			}
			for _, other := range Stacks() {
				want := 0
				if other == stack {
					want = 1
				}
				for _, line := range profileLines(t, "children/pins/"+strings.ToLower(other)+".md") {
					if count := countLine(parent, line); count != want {
						t.Errorf("a %s parent holds the %s pin line %q %d times, want %d", stack, other, line, count, want)
					}
					if countLine(plain, line) != 0 {
						t.Errorf("a render without children holds the pin line %q", line)
					}
				}
			}
			releases := headingSection(t, parent["govna/build-release.md"], "## Child Releases")
			for _, line := range profileLines(t, "children/pins/"+strings.ToLower(stack)+".md") {
				if !strings.Contains(releases, line+"\n") {
					t.Errorf("## Child Releases omits the pin line %q", line)
				}
			}
			if fileText(t, without, "build.sh") != fileText(t, with, "build.sh") {
				t.Error("build.sh differs between the renders with and without children")
			}
		})
	}
	doc, err := Render(Config{Flavor: Doc, RepoName: "handbook", Children: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range profileLines(t, blocks...) {
		if !strings.Contains(line, "{{") && countLine(governanceText(doc), line) != 0 {
			t.Errorf("a DOC render holds the child line %q", line)
		}
	}
}

func TestRustProfileRendersOnlyForRust(t *testing.T) {
	var blocks []string
	for _, block := range rustBlocks {
		blocks = append(blocks, block.asset)
	}
	lines := profileLines(t, blocks...)
	if len(lines) != 6 {
		t.Fatalf("the Rust profile holds %d lines, want 6", len(lines))
	}
	for _, profile := range dedupProfiles() {
		files, err := Render(profile.config)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if profile.config.Stack == "Rust" {
			want = 1
		}
		for _, line := range lines {
			if count := countLine(governanceText(files), line); count != want {
				t.Errorf("%s holds the Rust line %q %d times, want %d", profile.name, line, count, want)
			}
		}
	}
}

func TestRenderedHeadingReferencesResolve(t *testing.T) {
	pathRef := regexp.MustCompile("`((?:AGENTS|govna/[a-z-]+)\\.md)` `(#{2,3} [^`]+)`")
	bareRef := regexp.MustCompile("`(#{2,3} [^`]+)`")
	// documentHeadings are headings of an AC or of another repository-owned
	// document, which a rule may name without the heading being in the contract.
	documentHeadings := map[string]bool{
		"## In Scope": true, "## Out Of Scope": true, "## Summary": true, "## Status": true,
		"## Acceptance Tests": true, "## Migration findings": true, "## Project Practices": true,
	}
	for _, profile := range dedupProfiles() {
		t.Run(profile.name, func(t *testing.T) {
			files, err := Render(profile.config)
			if err != nil {
				t.Fatal(err)
			}
			texts := governanceText(files)
			headings := map[string]map[string]bool{}
			for path, text := range texts {
				headings[path] = map[string]bool{}
				for line := range strings.SplitSeq(text, "\n") {
					if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
						headings[path][line] = true
					}
				}
			}
			for path, text := range texts {
				for _, match := range pathRef.FindAllStringSubmatch(text, -1) {
					target, ok := headings[match[1]]
					if !ok {
						t.Errorf("%s references %s, which this profile does not render", path, match[1])
						continue
					}
					if !target[match[2]] {
						t.Errorf("%s references the missing heading %s `%s`", path, match[1], match[2])
					}
				}
			}
			agents := pathRef.ReplaceAllString(texts["AGENTS.md"], "")
			for _, match := range bareRef.FindAllStringSubmatch(agents, -1) {
				if !headings["AGENTS.md"][match[1]] && !documentHeadings[match[1]] {
					t.Errorf("AGENTS.md references the missing heading `%s`", match[1])
				}
			}
		})
	}
}

func TestMovedSectionsHaveOneHome(t *testing.T) {
	homes := map[string][]string{
		"govna/canon-spec.md": {
			"## Metadata and retired routing marker", "## Classification order and file handling", "## Format-defining files",
			"## Expected-divergence registry", "## Target-only detection", "## Migration-required items", "## Markdown identifiers",
			"## IE spacing", "## Retired link", "## Canon baseline manifest", "## Canon-coherence precondition",
			"## First-Class CODE Stacks", "## Multi-Utility Versioning", "## Markdown Identifier Check", "## Go", "## Rust",
			"## Terraform", "## Swift", "## CODE Build Presentation", "## What prep does", "## DOC Release",
		},
		"govna/canon-maintenance.md": {
			"## govna-side commitments", "## Candidate-canon review", "### Render and audit", "### Emitted AC review",
			"### Behavior parity", "### Release-batch safety", "### Review evidence",
		},
		"arch.md": {"### Canon Audiences", "### Contract Rationale"},
	}
	var renders []map[string]string
	for _, profile := range dedupProfiles() {
		files, err := Render(profile.config)
		if err != nil {
			t.Fatal(err)
		}
		renders = append(renders, governanceText(files))
	}
	for path, headings := range homes {
		home := map[string]string{path: repoFile(t, path)}
		for _, heading := range headings {
			if count := countLine(home, heading); count != 1 {
				t.Errorf("%s holds %q %d times, want 1", path, heading, count)
			}
			for _, render := range renders {
				if countLine(render, heading) != 0 {
					t.Errorf("a render still holds the moved section %q", heading)
				}
			}
		}
	}
	for _, path := range []string{"govna/canon-spec.md", "govna/canon-maintenance.md"} {
		for _, asset := range embeddedText(t) {
			if strings.Contains(asset, strings.TrimSuffix(strings.SplitN(repoFile(t, path), "\n", 2)[0], "\n")+"\n") {
				t.Errorf("an embedded asset carries the title of %s", path)
			}
		}
	}
}

func TestConsumerReadingListLinksResolve(t *testing.T) {
	link := regexp.MustCompile(`^- \[[^\]]+\]\(https://github\.com/queone/govna/blob/main/([^)]+)\) — .+\.$`)
	for _, profile := range dedupProfiles() {
		files, err := Render(profile.config)
		if err != nil {
			t.Fatal(err)
		}
		_, list, ok := strings.Cut(fileText(t, files, "govna/README.md"), "\n## More About Govna\n")
		if !ok {
			t.Fatalf("%s govna/README.md omits ## More About Govna", profile.name)
		}
		if !strings.Contains(list, "They carry no rule for this repository and do not specify its installed version.") {
			t.Errorf("%s reading list does not say that it carries no rule", profile.name)
		}
		bullets := 0
		for line := range strings.SplitSeq(list, "\n") {
			if !strings.HasPrefix(line, "- ") {
				continue
			}
			bullets++
			match := link.FindStringSubmatch(line)
			if match == nil {
				t.Errorf("%s reading list bullet has no Govna repository link: %s", profile.name, line)
				continue
			}
			repoFile(t, match[1])
		}
		if bullets != 3 {
			t.Errorf("%s reading list holds %d bullets, want 3", profile.name, bullets)
		}
	}
}

// movedAdoptionRules left every contract: the emitted adoption AC carries six
// of them, and govna/audit.md already owns the first.
var movedAdoptionRules = []string{
	"Require the Director to name every migration destination.",
	"Apply each resolved routing action while leaving the emitted AC stub unchanged.",
	"Preserve the boundary heading and every line below it as repo-owned content.",
	"Resolve an unresolved emitted repository check in chat.",
	"Cite repository evidence when choosing `Not applicable` for the repository check.",
	"Write `govna/canon-baseline.txt` from the scratch render only after every other applicable acceptance test and routing outcome passes and the resolved repository check succeeds or its `Not applicable` evidence holds.",
	"Do not re-run `govna audit` as an implementation gate for the emitted AC.",
}

func TestAuditAdoptionKeepsOnlyRulesThatMustLoadFirst(t *testing.T) {
	for path, text := range embeddedText(t) {
		for _, rule := range movedAdoptionRules {
			if strings.Contains(text, "- "+rule+"\n") {
				t.Errorf("asset %s still carries the adoption rule %q", path, rule)
			}
		}
	}
	want := headingSection(t, repoFile(t, "AGENTS.md"), "### Audit Adoption")
	if rules := strings.Count(want, "\n- "); rules != 23 {
		t.Errorf("### Audit Adoption holds %d rules, want 23", rules)
	}
	for _, kept := range []string{
		"- Apply integrated audit adoption only when the Director explicitly requests govna audit.\n",
		"- Limit scratch-review authority to one unique system-temporary directory outside the consumer repository.\n",
		"- Stop integrated audit adoption before Implement.\n",
		"- Apply these rules whenever implementing an audit-emitted AC.\n",
		"- Treat every Director-resolved routing target as effective implementation scope, even when it is absent from `## In Scope`.\n",
		"- Treat `CHANGELOG.md` as effective implementation scope only when a resolved legacy-phrase outcome requires removing an exact phrase.\n",
		"- Follow `govna/audit.md` `## Adoption procedure` for each resolved routing action.\n",
	} {
		if strings.Count(want, kept) != 1 {
			t.Errorf("### Audit Adoption needs %q once", kept)
		}
	}
	for _, profile := range dedupProfiles() {
		files, err := Render(profile.config)
		if err != nil {
			t.Fatal(err)
		}
		agents := fileText(t, files, "AGENTS.md")
		if headingSection(t, agents, "### Audit Adoption") != want {
			t.Errorf("%s ### Audit Adoption differs from the repository contract", profile.name)
		}
		for _, rule := range movedAdoptionRules {
			if strings.Contains(agents, "- "+rule+"\n") {
				t.Errorf("%s AGENTS.md still carries the adoption rule %q", profile.name, rule)
			}
		}
		if !strings.Contains(fileText(t, files, "govna/audit.md"), "- Require the Director to name every migration destination in the routing response.\n") {
			t.Errorf("%s govna/audit.md no longer requires a named migration destination", profile.name)
		}
	}
}

func TestUtilityProfileRendersCommandLineRules(t *testing.T) {
	const cli, versions = "## CLI Usage Formatting", "## Independent Utility Versions"
	const request = "- Ask the Director to run `govna audit --with-utilities` before Implement of an AC that adds the first command-line utility.\n"
	flags := profileLines(t, "utilities/flag-rules.md")
	if len(flags) != 2 {
		t.Fatalf("the flag rules hold %d lines, want 2", len(flags))
	}
	count := func(files []File, path, text string) int {
		for _, file := range files {
			if file.Path == path {
				return strings.Count("\n"+string(file.Content), "\n"+text)
			}
		}
		return 0
	}
	check := func(name string, config Config, wantSections, wantFlags, wantRequest int) {
		t.Helper()
		files, err := Render(config)
		if err != nil {
			t.Fatal(err)
		}
		if got := count(files, "govna/development-guidelines.md", cli+"\n"); got != wantSections {
			t.Errorf("%s holds %s %d times, want %d", name, cli, got, wantSections)
		}
		if got := count(files, "govna/build-release.md", versions+"\n"); got != wantSections {
			t.Errorf("%s holds %s %d times, want %d", name, versions, got, wantSections)
		}
		for _, line := range flags {
			if got := count(files, "AGENTS.md", line+"\n"); got != wantFlags {
				t.Errorf("%s holds the flag rule %q %d times, want %d", name, line, got, wantFlags)
			}
		}
		if got := count(files, "AGENTS.md", request); got != wantRequest {
			t.Errorf("%s holds the audit-request rule %d times, want %d", name, got, wantRequest)
		}
		if wantRequest == 1 && !strings.Contains(headingSection(t, fileText(t, files, "AGENTS.md"), "### AC-First Workflow"), request) {
			t.Errorf("%s keeps the audit-request rule outside ### AC-First Workflow", name)
		}
	}
	for _, stack := range Stacks() {
		config := Config{Flavor: Code, RepoName: "widget", Stack: stack, ModulePath: "example.com/widget"}
		with := config
		with.Utilities = true
		if stack == "Terraform" {
			check("Terraform", config, 0, 0, 0)
			check("Terraform with the flag", with, 0, 0, 0)
			continue
		}
		check(stack+" without a utility", config, 0, 1, 1)
		check(stack+" with a utility", with, 1, 1, 0)
	}
	check("DOC", Config{Flavor: Doc, RepoName: "handbook", Utilities: true}, 0, 0, 0)
}

func TestContractScenariosStayInThisRepository(t *testing.T) {
	_, section, ok := strings.Cut(repoFile(t, "govna/canon-maintenance.md"), "\n## Contract scenarios\n")
	if !ok {
		t.Fatal("govna/canon-maintenance.md omits ## Contract scenarios")
	}
	rows := 0
	for line := range strings.SplitSeq(section, "\n") {
		if strings.HasPrefix(line, "| S") {
			rows++
			if cells := strings.Split(strings.Trim(line, "| "), " | "); len(cells) != 4 || cells[3] == "" {
				t.Errorf("scenario row lacks a boundary, a state, or an expected outcome: %s", line)
			}
		}
	}
	if rows != 8 {
		t.Errorf("the scenario set holds %d rows, want 8", rows)
	}
	for _, profile := range dedupProfiles() {
		files, err := Render(profile.config)
		if err != nil {
			t.Fatal(err)
		}
		for path, text := range governanceText(files) {
			if strings.Contains(text, "Contract scenarios") {
				t.Errorf("%s %s holds the scenario set", profile.name, path)
			}
		}
	}
}
