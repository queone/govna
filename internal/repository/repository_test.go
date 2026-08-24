package repository

import (
	"os"
	"path/filepath"
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
