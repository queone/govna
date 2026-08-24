package apply

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runAt(t *testing.T, d string, args ...string) (string, string, int) {
	t.Helper()
	var out, err bytes.Buffer
	code := Run(args, &out, &err, d, func(string, ...string) ([]byte, error) { return nil, nil })
	return out.String(), err.String(), code
}
func TestFreshAndReapply(t *testing.T) {
	d := filepath.Join(t.TempDir(), "widget")
	os.Mkdir(d, 0755)
	out, err, code := runAt(t, d, "-f", "code", "-s", "rust")
	if code != 0 {
		t.Fatalf("%s", err)
	}
	for _, p := range []string{"AGENTS.md", "CLAUDE.md", "govna/ac1-govna-apply.md"} {
		if !exists(filepath.Join(d, p)) {
			t.Fatalf("missing %s", p)
		}
	}
	if !strings.Contains(out, "write govna/ac1-govna-apply.md") {
		t.Fatal(out)
	}
	for _, line := range []string{"mode: apply\n", "repo-shape: empty\n", "signals: code=0 doc=0\n", "existing-artifacts: none\n", "overwrite-risk: low\n"} {
		if !strings.Contains(out, line) {
			t.Fatalf("assessment missing %q in %q", line, out)
		}
	}
	assertGolden(t, filepath.Join(d, "govna/ac1-govna-apply.md"), "fresh-code-golden.md")
	_, err, code = runAt(t, d, "-f", "code", "-s", "rust")
	if code != 0 || !strings.Contains(err, "existing governance") {
		t.Fatalf("code=%d err=%s", code, err)
	}
	if !exists(filepath.Join(d, "govna/ac2-govna-apply.md")) {
		t.Fatal("reapply AC missing")
	}
}

func TestFreshDocGolden(t *testing.T) {
	d := filepath.Join(t.TempDir(), "handbook")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runAt(t, d, "-f", "doc")
	if code != 0 {
		t.Fatal(stderr)
	}
	assertGolden(t, filepath.Join(d, "govna/ac1-govna-apply.md"), "fresh-doc-golden.md")
}

func TestExistingGolden(t *testing.T) {
	d := filepath.Join(t.TempDir(), "widget")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runAt(t, d, "-f", "code", "-s", "rust"); code != 0 {
		t.Fatal(stderr)
	}
	if err := os.Remove(filepath.Join(d, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "CLAUDE.md"), []byte("local claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	agents, err := os.OpenFile(filepath.Join(d, "AGENTS.md"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := agents.WriteString("\n- Local rule.\n"); err != nil {
		t.Fatal(err)
	}
	agents.Close()
	if _, stderr, code := runAt(t, d, "-f", "code", "-s", "rust"); code != 0 {
		t.Fatal(stderr)
	}
	assertGolden(t, filepath.Join(d, "govna/ac2-govna-apply.md"), "existing-golden.md")
}

func TestAssessmentForExistingCodeRepository(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "go.mod"), []byte("module example.com/widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(d, "cmd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "cmd", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "README.md"), []byte("# Widget\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runAt(t, d, "--flavor", "code", "--stack", "go")
	if code != 0 {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	for _, line := range []string{"repo-shape: likely CODE\n", "signals: code=4 doc=2\n", "existing-artifacts: README.md\n", "overwrite-risk: medium\n"} {
		if !strings.Contains(out, line) {
			t.Fatalf("assessment missing %q in %q", line, out)
		}
	}
}

func assertGolden(t *testing.T, actual, golden string) {
	t.Helper()
	got, err := os.ReadFile(actual)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", golden))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s differs from %s", actual, golden)
	}
}
func TestExistingPreservationAndGovernaIgnored(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "governa"), 0755)
	os.WriteFile(filepath.Join(d, "governa/metadata.txt"), []byte("legacy\n"), 0600)
	os.WriteFile(filepath.Join(d, "README.md"), []byte("mine\n"), 0600)
	os.WriteFile(filepath.Join(d, "CLAUDE.md"), []byte("mine\n"), 0600)
	_, err, code := runAt(t, d, "-f", "doc")
	if code != 0 {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "README.md")); string(b) != "mine\n" {
		t.Fatal("README changed")
	}
	if b, _ := os.ReadFile(filepath.Join(d, "governa/metadata.txt")); string(b) != "legacy\n" {
		t.Fatal("governa changed")
	}
	if !strings.Contains(err, "regular file") {
		t.Fatal(err)
	}
}
func TestGitMain(t *testing.T) {
	d := t.TempDir()
	calls := ""
	var out, err bytes.Buffer
	code := Run([]string{"-f", "doc", "-g"}, &out, &err, d, func(name string, args ...string) ([]byte, error) {
		calls = name + " " + strings.Join(args, " ")
		return nil, nil
	})
	if code != 0 || !strings.Contains(calls, "git init -b main ") {
		t.Fatalf("code=%d calls=%s err=%s", code, calls, err.String())
	}
}
func TestParseErrors(t *testing.T) {
	for _, args := range [][]string{{"x"}, {"-f"}, {"-f", "bad"}, {"-s", " "}, {"-m"}} {
		var out, err bytes.Buffer
		if Run(args, &out, &err, t.TempDir(), nil) != 2 {
			t.Fatalf("accepted %v", args)
		}
	}
}
