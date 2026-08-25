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
