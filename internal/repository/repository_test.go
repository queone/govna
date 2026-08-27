package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolution(t *testing.T) {
	d := t.TempDir()
	os.WriteFile(filepath.Join(d, "go.mod"), []byte("module example.com/widget\n"), 0644)
	f, e := Flavor(d, "")
	if e != nil || f != "CODE" || Stack(d) != "Go" || ModulePath(d) != "example.com/widget" || Name(d, "example.com/widget", "") != "widget" {
		t.Fatalf("bad resolution %s %v", f, e)
	}
	if IsSource(d) {
		t.Fatal("consumer identified as source")
	}
}

func TestStackSelectionSupportsOnlyCompleteAdapters(t *testing.T) {
	for _, tc := range []struct {
		name, manifest, stack string
		supported             bool
	}{
		{name: "Go", manifest: "go.mod", stack: "Go", supported: true},
		{name: "Rust", manifest: "Cargo.toml", stack: "Rust", supported: true},
		{name: "Swift", manifest: "Package.swift", stack: "Swift", supported: true},
		{name: "Terraform lock", manifest: ".terraform.lock.hcl", stack: "Terraform", supported: true},
		{name: "Terraform file", manifest: "main.tf", stack: "Terraform", supported: true},
		{name: "Node", manifest: "package.json", stack: "Node"},
		{name: "Python", manifest: "pyproject.toml", stack: "Python"},
		{name: "Java Maven", manifest: "pom.xml", stack: "Java"},
		{name: "Java Gradle", manifest: "build.gradle", stack: "Java"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, tc.manifest), []byte("fixture\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if tc.supported {
				if got := Stack(root); got != tc.stack {
					t.Fatalf("stack=%q, want %q", got, tc.stack)
				}
				if flavor, err := Flavor(root, ""); err != nil || flavor != "CODE" {
					t.Fatalf("flavor=%q err=%v", flavor, err)
				}
				return
			}
			if got := Stack(root); got != "" {
				t.Fatalf("unsupported manifest inferred stack %q", got)
			}
			_, err := Flavor(root, "")
			if err == nil || !strings.Contains(err.Error(), tc.stack) || !strings.Contains(err.Error(), "Go, Rust, Swift, or Terraform") {
				t.Fatalf("unsupported manifest error=%v", err)
			}
		})
	}
}

func TestStackSelectionIgnoresManifestDirectories(t *testing.T) {
	for _, manifest := range []string{"go.mod", "Cargo.toml", "Package.swift", ".terraform.lock.hcl", "main.tf", "package.json", "pyproject.toml", "pom.xml", "build.gradle"} {
		t.Run(manifest, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, manifest), 0o755); err != nil {
				t.Fatal(err)
			}
			if got := Stack(root); got != "" {
				t.Fatalf("manifest directory inferred stack %q", got)
			}
		})
	}
}
func TestSourceSignature(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "internal/canon/assets/base"), 0755)
	os.MkdirAll(filepath.Join(d, "cmd/govna"), 0755)
	os.WriteFile(filepath.Join(d, "go.mod"), []byte("module github.com/queone/govna\n"), 0644)
	os.WriteFile(filepath.Join(d, "internal/canon/assets/base/AGENTS.md.tmpl"), nil, 0644)
	if IsSource(d) {
		t.Fatal("partial signature accepted")
	}
	os.WriteFile(filepath.Join(d, "cmd/govna/main.go"), nil, 0644)
	if !IsSource(d) {
		t.Fatal("full signature rejected")
	}
}

func TestAuditPreconditions(t *testing.T) {
	d := t.TempDir()
	if err := RequireAdopted(d); err == nil {
		t.Fatal("missing AGENTS accepted")
	}
	os.WriteFile(filepath.Join(d, "AGENTS.md"), []byte("ok\n"), 0644)
	os.MkdirAll(filepath.Join(d, "govna"), 0755)
	os.WriteFile(filepath.Join(d, "govna", "ac-template.md"), []byte("ok\n"), 0644)
	if err := RequireAdopted(d); err != nil {
		t.Fatal(err)
	}
	if err := RequireGitWorktree(d); err == nil {
		t.Fatal("non-worktree accepted")
	}
}

func TestFlavorErrorsExplainTheProblemAndRecovery(t *testing.T) {
	t.Run("no evidence", func(t *testing.T) {
		_, err := Flavor(t.TempDir(), "")
		want := "Govna could not determine whether this is a CODE or DOC repository; add govna/metadata.txt, pass --flavor code|doc, or add a recognized project manifest"
		if err == nil || err.Error() != want {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("conflicting evidence", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/widget\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "_config.yml"), []byte("title: Widget\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := Flavor(root, "")
		want := "Govna found both CODE and DOC evidence: the repository has _config.yml and a CODE project manifest; pass --flavor code or --flavor doc"
		if err == nil || err.Error() != want {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestAdoptionErrorNamesAcceptedEvidence(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := RequireAdopted(root)
	if err == nil || !strings.Contains(err.Error(), "Govna could not find the files that confirm Govna was added") || !strings.Contains(err.Error(), "govna/ac-template.md") || !strings.Contains(err.Error(), "CHANGELOG.md") {
		t.Fatalf("err=%v", err)
	}
}
