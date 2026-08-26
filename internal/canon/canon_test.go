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
	for _, stack := range []string{"Go", "Rust", "Swift", "Terraform", "Node", "Python", "Java"} {
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
			if !strings.HasPrefix(text, "govna-canon-baseline-v1\ncanon_version = v0.39.0\n") {
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
	for input, want := range map[string]string{"go": "Go", "GOLANG": "Go", "rUsT": "Rust", "swift": "Swift", "terraform": "Terraform", "node": "Node", "python": "Python", "java": "Java"} {
		got, ok := CanonicalStack(input)
		if !ok || got != want {
			t.Fatalf("%s: %s, %v", input, got, ok)
		}
	}
	if _, ok := CanonicalStack("ruby"); ok {
		t.Fatal("accepted unsupported stack")
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
	if got := Stacks(); len(got) != 7 || got[0] != "Go" || got[6] != "Terraform" {
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

func TestImmutableAuditACVerificationContract(t *testing.T) {
	root := filepath.Join("..", "..")
	paths := []string{
		"AGENTS.md",
		"internal/canon/assets/base/AGENTS.md.tmpl",
		"internal/canon/assets/overlays/doc/files/AGENTS.md.tmpl",
	}
	required := []string{
		"- Confirm each settled decision landed verbatim in the AC.",
		"- Treat a Director-resolved routing decision recorded in chat for an immutable emitted AC as satisfying the verbatim-in-AC check.",
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

func TestGovernanceScenarios(t *testing.T) {
	for _, flavor := range []Flavor{Code, Doc} {
		stack := ""
		if flavor == Code {
			stack = "Rust"
		}
		files, err := Render(Config{Flavor: flavor, RepoName: "widget", Stack: stack})
		if err != nil {
			t.Fatal(err)
		}
		agents := fileText(t, files, "AGENTS.md")
		for _, required := range []string{"Audit", "Refine", "Implement", "Ratify", "Package", "bounded completeness", "Primary And Ancillary Scope", "Contract Integrity"} {
			if !strings.Contains(agents, required) {
				t.Errorf("%s AGENTS lacks scenario marker %q", flavor, required)
			}
		}
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
	if !strings.Contains(goBuild, "honnef.co/go/tools/cmd/staticcheck@v0.8.0") || strings.Contains(goBuild, "staticcheck@v0.7.0") {
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
