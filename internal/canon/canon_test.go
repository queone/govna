package canon

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	efficiencyPurposeDefinition = "Govna exists to make programming and publishing ceremonies—the recurring CODE and DOC checkpoints around intent, authorization, scope, review, implementation or editing, verification, and release—more effective and efficient."
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
			rationale := fileText(t, files, "govna/operator-contract-rationale.md")
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
			if !strings.HasPrefix(text, "govna-canon-baseline-v1\ncanon_version = v0.47.0\n") {
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
	files, err := Render(Config{Flavor: Code, RepoName: "govna", Stack: "Go", ModulePath: "github.com/queone/govna"})
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
	for _, tc := range []struct {
		authority string
		assets    []string
	}{
		{"govna/ac-template.md", []string{"assets/overlays/code/files/govna/ac-template.md.tmpl"}},
		{"govna/audit.md", []string{"assets/overlays/code/files/govna/audit.md.tmpl", "assets/overlays/doc/files/govna/audit.md.tmpl"}},
		{"govna/canon-cycle.md", []string{"assets/overlays/code/files/govna/canon-cycle.md.tmpl", "assets/overlays/doc/files/govna/canon-cycle.md.tmpl"}},
		{"govna/development-cycle.md", []string{"assets/overlays/code/files/govna/development-cycle.md.tmpl"}},
		{"govna/operator-contract-rationale.md", []string{"assets/overlays/code/files/govna/operator-contract-rationale.md.tmpl", "assets/overlays/doc/files/govna/operator-contract-rationale.md.tmpl"}},
		{"govna/roles.md", []string{"assets/overlays/code/files/govna/roles.md.tmpl"}},
	} {
		want, err := os.ReadFile(filepath.Join(root, tc.authority))
		if err != nil {
			t.Fatal(err)
		}
		for _, asset := range tc.assets {
			got, err := fs.ReadFile(assets, asset)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Errorf("%s does not mirror %s", asset, tc.authority)
			}
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
	if gotParts[0] != wantZone {
		t.Error("base AGENTS canon zone differs from authority")
	}
	if strings.TrimSpace(gotParts[1]) != "- Follow existing repo patterns unless an approved improvement says otherwise." {
		t.Errorf("unexpected Project Rules seed: %s", gotParts[1])
	}
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
		if authority == nil {
			authority = content
		} else if string(content) != string(authority) {
			t.Errorf("%s does not mirror govna/audit.md", path)
		}
		for _, required := range []string{
			"- Require the selected CODE stack's recognized root manifest before inferring `./build.sh`.",
			"- Ignore unrelated manifests, other prose, governance documents, executables, CI files, and flavor defaults.",
			"- Name each emitted adoption AC `# AC<N> Adopt Govna Governance Files v<CANON_VERSION>`.",
			"- Place the repository paragraph first under `## Summary`.",
			"- Start the repository paragraph with `This AC updates`.",
			"- Follow it with `The result label (classification)`.",
			"- Place the count paragraph after the repository paragraph.",
			"- Start the count paragraph with `Govna found`.",
			"- Confirm each file selected for update exists in the selected CODE render.",
			"- Emit an unresolved repository check as the final numbered routing decision.",
			"- Emit one manual resolution AT for an unresolved repository check.",
			"- Emit one automated verification AT for an unresolved repository check.",
			"- Classify an existing target-only path named in the preserve registry as `preserve`.",
			"- Keep that preserved target-only path visible in audit and JSON results.",
			"- Omit another routing question for that preserved target-only path.",
			"- Offer only these outcomes for a canon-backed ambiguity: sync, preserve, explicitly named migration, delete.",
			"- Offer only these outcomes for an ordinary `target-has-no-canon` item: preserve, explicitly named migration, delete.",
			"- Install an exact current-canon replacement before retired-source routing.",
			"- Omit restore as a routing outcome.",
			"- Offer conversion to `govna/preserve.txt` or exact-phrase removal for marker-only evidence.",
			"- Preserve unrelated CHANGELOG Summary text and historical rows.",
			"- Emit a conditional named-destination check for each offered migration outcome.",
			"- Emit a replacement-before-retired-source check for each replacement-missing route.",
			"- Emit an exact-phrase absence check for each legacy-phrase route.",
			"- Emit an unrelated-Summary preservation check for each legacy-phrase route.",
			"- Emit an outside-Summary preservation check for each legacy-phrase route.",
			"- Keep every emitted routing check atomic.",
			"- Keep emitted AT numbering stable across identical reports.",
			"<N>. **Repository check**: Which command should run after the selected file updates, or what repository evidence shows that no command applies?",
		} {
			if strings.Count(string(content), required) != 1 {
				t.Errorf("%s requires one occurrence of %q", path, required)
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
}

func TestPackageCompletionContract(t *testing.T) {
	agentRules := []string{
		"Start every Package completion report with the plain, unbulleted, unindented line `Package complete.`.",
		"Keep `Verified:`, `Red-teamed:`, `Not checked:`, and `Run below to release:` in the Package completion report.",
		"Present the exact drafted release command.",
	}
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

func TestSupportedCODEStackContract(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, path := range []string{"govna/code-stacks.md", "internal/canon/assets/overlays/code/files/govna/code-stacks.md.tmpl"} {
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
		"- Run the consumer-equivalent candidate-canon review in `govna/canon-cycle.md` before completing any canon change.",
		"- Block canon-change Implement completion while that review has an unresolved Govna-canon finding.",
	} {
		if strings.Count(string(agents), rule) != 1 {
			t.Errorf("AGENTS.md requires one maintainer gate: %s", rule)
		}
	}
	for _, path := range []string{
		"govna/canon-cycle.md",
		"internal/canon/assets/overlays/code/files/govna/canon-cycle.md.tmpl",
		"internal/canon/assets/overlays/doc/files/govna/canon-cycle.md.tmpl",
	} {
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
			"- Audit every actionable emitted AC immediately.",
			"- Exercise one-AC and fitting multi-AC pending batches.",
			"- Exercise an oversized projected batch before another Implement.",
			"- Exercise oversized, partial, and partly unaccepted batches before Package prep.",
			"- Require every implemented pending-batch member to complete Ratify before Package.",
			"- Verify whole-tree release staging cannot bypass complete-batch rules.",
			"- Verify 80-byte acceptance with ASCII and multibyte messages.",
			"- Reject 81-byte messages before release-prep mutation.",
			"- Verify numbered Audit and Refine steps remain atomic in CODE and DOC renders.",
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
		"- Apply each resolved routing action while leaving the emitted AC stub unchanged.",
		"- Treat `CHANGELOG.md` as effective implementation scope only when a resolved legacy-phrase outcome requires removing an exact phrase.",
		"- Install each missing canon-backed replacement before retired-source routing.",
		"- Remove each exact legacy phrase only after verifying its resolved target and registry state.",
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
		"- Report implementation readiness only when Pre-Implementation Verification passes.",
		"- Remain in Refine when Pre-Implementation Verification finds a gap.",
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
			"- Treat an explicit request to run govna audit as authorization for integrated audit adoption under ### Audit Adoption.",
			"- Pause after Audit until the Director requests Refine unless integrated audit adoption applies.",
			"- Treat a Director-resolved routing decision or explicit workflow override recorded in chat for an immutable emitted AC as satisfying the verbatim-in-AC check.",
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
			"- Remove the exact scratch directory before reporting Audit completion or a blocker.",
			"- Resume Refine after the Director resolves every blocking finding and decision.",
			"- Run Pre-Implementation Verification after Refine.",
			"- Stop before Implement.",
			"`PENDING` — immutable audit emission; workflow state is tracked in the active session.",
		} {
			if strings.Count(text, required) != 1 {
				t.Errorf("%s requires one integrated-audit phrase %q", path, required)
			}
		}
		if strings.Contains(text, "audit emission; awaiting explicit Director Audit") {
			t.Errorf("%s retains the separate-Audit waiting status", path)
		}
	}

	docRequirements := map[string]string{
		"README.md":       "An explicit agent-mediated request to run `govna audit` also authorizes immediate review",
		"arch.md":         "Integrated audit adoption is the only command-mediated phase exception.",
		"govna/README.md": "continues through immediate Operator Audit and no-edit Refine",
		"internal/canon/assets/overlays/code/files/README.md.tmpl":                  "also authorizes immediate Audit and Refine",
		"internal/canon/assets/overlays/doc/files/README.md.tmpl":                   "also authorizes immediate Audit and Refine",
		"internal/canon/assets/overlays/code/files/arch.md.tmpl":                    "Integrated audit adoption is the only command-mediated phase exception.",
		"internal/canon/assets/overlays/code/files/govna/README.md.tmpl":            "continues through immediate Operator Audit and no-edit Refine",
		"internal/canon/assets/overlays/doc/files/govna/README.md.tmpl":             "continues through immediate Operator Audit and no-edit Refine",
		"govna/development-cycle.md":                                                "The `govna` executable ends after deterministic audit comparison and emission.",
		"internal/canon/assets/overlays/code/files/govna/development-cycle.md.tmpl": "The `govna` executable ends after deterministic audit comparison and emission.",
		"internal/canon/assets/overlays/doc/files/govna/editing-cycle.md.tmpl":      "The `govna` executable ends after deterministic audit comparison and emission.",
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
			"- Treat an explicit valid Package instruction for an established Ratified release batch as the trigger for release-prep bookkeeping.",
			"- Define the pending release batch as every unpackaged AC whose implementation is present in the unreleased repository state.",
			"- Include an implemented AC in the pending release batch while it awaits Ratify.",
			"- Require every pending release-batch member to complete Ratify before Package.",
			"- Require the established release batch to equal the complete pending release batch.",
			"- Reject Package while excluded implemented work remains in the unreleased repository state.",
			"- Require the release-message AC-reference set to equal the established release batch before Package runs prep.",
			"- Treat one active Ratified AC as an established one-AC release batch only when it is the complete pending release batch.",
			"- Treat only a Director-named complete pending release batch as an established multi-AC release batch.",
			"- Accept only Package followed by a plus-joined list of uppercase AC<number> references as the named-batch Package form.",
			"- Apply standalone Package, package, pack, or prep to the established Ratified release batch.",
			"- Ask the Director to name the release batch when multiple ungrouped Ratified ACs can enter Package.",
			"- Reject a named release batch that contains a non-Ratified AC.",
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
			"- Start this checklist only when the Director explicitly requests a valid Package instruction for the established Ratified release batch.",
			"- Map every unpackaged AC with implementation in the unreleased repository state to the complete pending release batch.",
			"- Require every pending release-batch member to complete Ratify before prep.",
			"- Reject prep while excluded implemented work remains in the unreleased repository state.",
			"- Require the unique release-message AC-reference set to equal the established release batch before prep.",
			"- Require the established release batch to equal the complete pending release batch before prep.",
			"- Reject a release message longer than 80 bytes before prep.",
			"- Prohibit a smaller release batch while excluded implemented work remains.",
			"- Prohibit automatic release-batch splitting.",
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
		if !strings.Contains(string(content), "Package requires every implemented batch member to be Ratified, compares the complete pending batch with the exact message, and rejects partial or oversized batches before prep.") {
			t.Errorf("%s omits complete release-batch safety", path)
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
		"- Recheck new or unresolved contract-integrity findings during Ratify.",
		"- Skip requests for a second acceptance signal after a clean Ratify review.",
		"- Use the complete primary-repository state as the default dependency boundary.",
		"- Narrow the dependency boundary only when repository evidence proves that a reused check cannot read the excluded state.",
		"- Treat the snapshot as incomplete when a relevant ignored or external input cannot be identified.",
		"- Apply `AGENTS.md` evidence-currentness rules before repeating validation.",
		"- Run `./build.sh` only when required build evidence is missing or stale.",
		"- Run each acceptance test in the active AC when its current disposition is unavailable.",
		"- Report each current acceptance-test disposition.",
		"## Why Clean Ratify Reuses Implement Evidence",
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
				fileText(t, files, "govna/operator-contract-rationale.md"),
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
		"- Start an AC cycle only after the Director identifies the AC and authorizes Audit or integrated audit adoption identifies the emitted AC.",
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
		"- Accept only Package followed by a plus-joined list of uppercase AC<number> references as the named-batch Package form.",
		"- Apply standalone Package, package, pack, or prep to the established Ratified release batch.",
		"- Ask the Director to name the release batch when multiple ungrouped Ratified ACs can enter Package.",
		"- Reject a named release batch that contains a non-Ratified AC.",
		"- Recheck the complete pending release batch and exact release message before Package runs prep.",
		"- Reject an oversized or partial release batch.",
		"- Prohibit automatic release-batch splitting.",
	}
	retiredRules := []string{
		"Apply an unnumbered action instruction to the sole AC under `govna/`.",
		"Use an unnumbered phase instruction when one AC is under `govna/`.",
		"Require the AC number when multiple ACs are present.",
		"Start an AC cycle only when the Director identifies the active AC and explicitly requests Audit.",
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
		assertRules(t, path, string(content))
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
			assertRules(t, variant.name+" "+variant.cyclePath, fileText(t, files, variant.cyclePath))
		})
	}
}

func TestPhaseAuthorizationMatrix(t *testing.T) {
	const refineRule = "- Exempt integrated audit adoption and an eligible bounded completeness correction from a fresh Refine action instruction."
	const implementRule = "- Exempt only an eligible bounded completeness correction from a fresh Implement action instruction."
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
		{name: "bounded completeness correction", token: "eligible bounded completeness correction", wantRefine: true, wantImplement: true},
	}
	guards := []string{
		"- Require fresh approval for every new action.",
		"- Treat an explicit request to run govna audit as authorization for integrated audit adoption under ### Audit Adoption.",
		"- Continue Director-authorized Implement authority only for an eligible bounded completeness correction under ### Four-Phase Workflow.",
		"- Pause after Audit until the Director requests Refine unless integrated audit adoption applies.",
		"- Pause after Refine and await explicit Director implementation-ready confirmation to Implement.",
		"- Apply the bounded completeness exception only after the Director authorizes Implement.",
		"- Prevent the exception from authorizing initial Implement, Audit, Ratify, Package, release preparation, publication, delegation, or commits.",
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

func TestGovernanceGrowthBaseline(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "governance-growth-baseline.txt"))
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 {
			t.Fatalf("invalid governance-growth line: %q", line)
		}
		expected[fields[0]] = fields[1] + "\t" + fields[2]
	}
	seen := map[string]bool{}
	assetRoot := filepath.Join("assets")
	err = filepath.WalkDir(assetRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		key, err := filepath.Rel(assetRoot, path)
		if err != nil {
			return err
		}
		key = filepath.ToSlash(key)
		if key == "base/AGENTS.md.tmpl" {
			key = "base/AGENTS.md"
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := strings.Count(string(content), "\n")
		rules := 0
		for line := range strings.SplitSeq(string(content), "\n") {
			if strings.HasPrefix(line, "- ") {
				rules++
			}
		}
		got := fmt.Sprintf("%d\t%d", lines, rules)
		if expected[key] != got {
			t.Errorf("%s growth count=%s want=%s", key, got, expected[key])
		}
		seen[key] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for key := range expected {
		if !seen[key] {
			t.Errorf("governance-growth fixture names missing asset %s", key)
		}
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
	for _, helper := range []string{"_print_coverage_summary()", "_domain_coverage()", "_extract_program_version()", "_is_strict_stable_semver()"} {
		if strings.Count(goBuild, helper) != 1 {
			t.Errorf("rendered Go build helper %q count=%d", helper, strings.Count(goBuild, helper))
		}
	}
	if !strings.Contains(goBuild, "honnef.co/go/tools/cmd/staticcheck@v0.8.1") || strings.Contains(goBuild, "staticcheck@v0.8.0") {
		t.Fatal("rendered Go build does not use the repository-governed Staticcheck pin")
	}
	if !strings.Contains(goBuild, "validation-token options are unsupported for Go") {
		t.Fatal("rendered Go build does not reject validation-token input")
	}
	for _, rootOnly := range []string{"internal/canon.Version", "cmd/govna canonVersion", "_literal_const_value()", "_validate_root_canon_version()", "_install_compiled_utility()", "_release_rebuild_and_verify()"} {
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

	goAgents := fileText(t, goFiles, "AGENTS.md")
	if !strings.Contains(goAgents, "Rust prep") || strings.Contains(goAgents, "CODE stack provides no validation-token support") {
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
