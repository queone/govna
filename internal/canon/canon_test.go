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
			if !strings.HasPrefix(text, "govna-canon-baseline-v1\ncanon_version = v0.29.0\n") {
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
		{"govna/roles.md", []string{"assets/overlays/code/files/govna/roles.md.tmpl", "assets/overlays/doc/files/govna/roles.md.tmpl"}},
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
