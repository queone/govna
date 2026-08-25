package render

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRunCodeAndAlias(t *testing.T) {
	for _, command := range []string{"render", "render-canon"} {
		t.Run(command, func(t *testing.T) {
			cwd := t.TempDir()
			if err := os.WriteFile(filepath.Join(cwd, "go.mod"), []byte("module example.com/widget\n\ngo 1.27.0\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			target := filepath.Join(cwd, "out")
			if code := Run([]string{"--flavor", "code", target}, &stdout, &stderr, cwd); code != 0 {
				t.Fatalf("code=%d stderr=%s", code, stderr.String())
			}
			if stdout.String() != target+"\n" || stderr.Len() != 0 {
				t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			if got, err := os.Readlink(filepath.Join(target, "CLAUDE.md")); err != nil || got != "AGENTS.md" {
				t.Fatalf("symlink=%q err=%v", got, err)
			}
			if runtime.GOOS != "windows" {
				info, _ := os.Stat(filepath.Join(target, "build.sh"))
				if info.Mode().Perm() != 0o755 {
					t.Fatalf("build mode %o", info.Mode().Perm())
				}
			}
		})
	}
}

func TestRunDocPreservesUnrelated(t *testing.T) {
	cwd := t.TempDir()
	target := filepath.Join(cwd, "out")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "keep.txt"), []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "CLAUDE.md"), []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--flavor", "doc", "out"}, &stdout, &stderr, cwd); code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if got, _ := os.ReadFile(filepath.Join(target, "keep.txt")); string(got) != "keep\n" {
		t.Fatalf("unrelated=%q", got)
	}
	if agents, _ := os.ReadFile(filepath.Join(target, "AGENTS.md")); strings.Contains(string(agents), "provider/API fetch") {
		t.Fatal("DOC AGENTS contains CODE vocabulary")
	}
}

func TestRunErrors(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		setup func(string)
		want  string
		code  int
	}{
		{"missing target", nil, nil, "render requires a positional <target> argument\n", 2},
		{"missing flavor", []string{"--flavor"}, nil, "--flavor requires a value\n", 2},
		{"missing stack", []string{"--stack"}, nil, "--stack requires a value\n", 2},
		{"missing module path", []string{"--module-path"}, nil, "--module-path requires a value\n", 2},
		{"invalid flavor", []string{"--flavor", "CODE", "out"}, nil, "invalid --flavor: \"CODE\"", 2},
		{"blank stack", []string{"--stack", " ", "out"}, nil, "--stack requires a non-empty value\n", 2},
		{"duplicate target", []string{"one", "two"}, nil, "unexpected argument: two", 2},
		{"doc stack", []string{"--flavor", "doc", "--stack", "Rust", "out"}, nil, "applies only to CODE canon", 1},
		{"doc module", []string{"--flavor", "doc", "--module-path", "x/y", "out"}, nil, "applies only to Go CODE canon", 1},
		{"non-go module", []string{"--flavor", "code", "--stack", "Rust", "--module-path", "x/y", "out"}, nil, "applies only to Go CODE canon", 1},
		{"unsupported stack", []string{"--flavor", "code", "--stack", "Ruby", "out"}, nil, "unsupported CODE stack", 1},
		{"missing go module", []string{"--flavor", "code", "--stack", "Go", "out"}, nil, "could not read module path", 1},
		{"absent flavor", []string{"out"}, nil, "Govna could not determine whether this is a CODE or DOC repository; add govna/metadata.txt, pass --flavor code|doc, or add a recognized project manifest", 1},
		{"conflict", []string{"out"}, func(cwd string) {
			os.WriteFile(filepath.Join(cwd, "go.mod"), []byte("module x\n"), 0o644)
			os.WriteFile(filepath.Join(cwd, "_config.yml"), []byte("x\n"), 0o644)
		}, "Govna found both CODE and DOC evidence: the repository has _config.yml and a CODE project manifest; pass --flavor code or --flavor doc", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cwd := t.TempDir()
			if tc.setup != nil {
				tc.setup(cwd)
			}
			var out, err bytes.Buffer
			code := Run(tc.args, &out, &err, cwd)
			if code != tc.code || !strings.Contains(err.String(), tc.want) || out.Len() != 0 {
				t.Fatalf("code=%d out=%q err=%q", code, out.String(), err.String())
			}
		})
	}
}

func TestFlavorAndStackInference(t *testing.T) {
	for _, tc := range []struct{ name, manifest, stack string }{{"Go", "go.mod", "Go"}, {"Terraform lock", ".terraform.lock.hcl", "Terraform"}, {"Rust", "Cargo.toml", "Rust"}, {"Swift", "Package.swift", "Swift"}, {"Node", "package.json", "Node"}, {"Python", "pyproject.toml", "Python"}, {"Java pom", "pom.xml", "Java"}, {"Java gradle", "build.gradle", "Java"}, {"Terraform glob", "main.tf", "Terraform"}} {
		t.Run(tc.name, func(t *testing.T) {
			cwd := t.TempDir()
			os.WriteFile(filepath.Join(cwd, tc.manifest), []byte("module example.com/x\n"), 0o644)
			if tc.stack == "Go" {
				os.WriteFile(filepath.Join(cwd, "go.mod"), []byte("module example.com/x\n"), 0o644)
			}
			if got := inferStack(cwd); got != tc.stack {
				t.Fatalf("got %s", got)
			}
		})
	}
}

func TestMetadataPrecedence(t *testing.T) {
	cwd := t.TempDir()
	os.Mkdir(filepath.Join(cwd, "govna"), 0o755)
	os.WriteFile(filepath.Join(cwd, "govna", "metadata.txt"), []byte("schema_version = 1\ncanon_version = v0.29.0\nrepo_type = DOC\n"), 0o644)
	os.WriteFile(filepath.Join(cwd, "go.mod"), []byte("module x\n"), 0o644)
	got, err := resolveFlavor(cwd, "")
	if err != nil || got != "DOC" {
		t.Fatalf("got=%s err=%v", got, err)
	}
}

func TestInvalidMetadata(t *testing.T) {
	for name, tc := range map[string]struct{ content, want string }{
		"missing newline":   {"repo_type = DOC", "require a final newline"},
		"malformed line":    {"repo_type DOC\n", "each line must use `key = value`"},
		"missing repo type": {"schema_version = 1\n", "missing repo_type"},
		"unknown repo type": {"repo_type = OTHER\n", "unknown repo_type \"OTHER\""},
	} {
		t.Run(name, func(t *testing.T) {
			cwd := t.TempDir()
			if err := os.Mkdir(filepath.Join(cwd, "govna"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(cwd, "govna", "metadata.txt"), []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := resolveFlavor(cwd, ""); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid metadata error=%v want fragment=%q", err, tc.want)
			}
		})
	}
}

func TestRenderGoldenManifest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "canon", "testdata", "render-golden.txt"))
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("invalid golden line: %q", line)
		}
		expected[fields[0]+"\t"+fields[1]] = fields[2] + "\t" + fields[3]
	}
	seen := map[string]bool{}
	for _, variant := range []string{"doc", "go", "rust", "swift", "terraform", "node", "python", "java"} {
		cwd := filepath.Join(t.TempDir(), "widget")
		if err := os.Mkdir(cwd, 0o755); err != nil {
			t.Fatal(err)
		}
		target := t.TempDir()
		args := []string{"--flavor", "doc", target}
		if variant != "doc" {
			args = []string{"--flavor", "code", "--stack", variant}
			if variant == "go" {
				args = append(args, "--module-path", "example.com/widget")
			}
			args = append(args, target)
		}
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, cwd); code != 0 {
			t.Fatalf("%s: %s", variant, stderr.String())
		}
		err := filepath.Walk(target, func(filePath string, info os.FileInfo, err error) error {
			if err != nil || !info.Mode().IsRegular() {
				return err
			}
			rel, _ := filepath.Rel(target, filePath)
			content, readErr := os.ReadFile(filePath)
			if readErr != nil {
				return readErr
			}
			hash := sha256.Sum256(content)
			key := variant + "\t" + filepath.ToSlash(rel)
			got := fmt.Sprintf("%o\t%x", info.Mode().Perm(), hash)
			if expected[key] != got {
				t.Errorf("%s: got %s, want %s", key, got, expected[key])
			}
			seen[key] = true
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for key := range expected {
		if !seen[key] {
			t.Errorf("golden entry not rendered: %s", key)
		}
	}
}
