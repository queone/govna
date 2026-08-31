package apply

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testProgramVersion = "9.8.7"

func runAt(t *testing.T, d string, args ...string) (string, string, int) {
	t.Helper()
	var out, err bytes.Buffer
	code := Run(args, &out, &err, d, testProgramVersion, func(string, ...string) ([]byte, error) { return nil, nil })
	return out.String(), err.String(), code
}

func TestAdoptionVersionAxesAndInstructions(t *testing.T) {
	created := adoption(7, "widget", "CODE", testProgramVersion, nil, "created")
	for _, want := range []string{
		"# AC7 Review Files Added by Govna",
		"Govna executable v9.8.7 added its embedded governance files (canon v0.50.0) for the CODE repository widget.",
		"Govna executable v9.8.7 added its embedded governance files (canon v0.50.0). The list below records whether each file was written, merged, or preserved.",
		"Files Govna processed:",
		"- Files not listed above.",
		"**AT1** [Manual] [Pre-release gate] — Verify AGENTS.md reflects the repository's actual practices.",
		"**AT2** [Manual] [Pre-release gate] — Verify govna/roles.md reflects the repository's delivery model (Operator + Director).",
		"**AT3** [Manual] [Pre-release gate] — Verify CLAUDE.md is a symlink to AGENTS.md.",
		"`PENDING` — apply emission; awaiting explicit Director Audit.",
	} {
		if !strings.Contains(created, want) {
			t.Errorf("created adoption omits %q", want)
		}
	}
	for _, invalid := range []string{"Applied govna v0.50.0", "Director reads", "review applied governance", "overlay", "consumer-owned"} {
		if strings.Contains(created, invalid) {
			t.Errorf("created adoption retains invalid text %q", invalid)
		}
	}
	preserved := adoption(8, "widget", "CODE", testProgramVersion, nil, "preserved")
	want := "**AT3** [Manual] [Pre-release gate] — Verify CLAUDE.md remains the existing regular file instead of a symlink to AGENTS.md."
	if !strings.Contains(preserved, want) {
		t.Errorf("preserved adoption omits %q", want)
	}
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
	if !strings.Contains(out, "wrote govna/ac1-govna-apply.md (review AC)") {
		t.Fatal(out)
	}
	for _, line := range []string{"mode: apply\n", "repository type: empty\n", "type evidence: CODE score=0 DOC score=0\n", "existing files: none\n", "risk of replacing content: low\n"} {
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
	gotIgnore, err := os.ReadFile(filepath.Join(d, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	wantIgnore, err := os.ReadFile(filepath.Join("..", "canon", "assets", "overlays", "doc", "files", ".gitignore.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotIgnore, wantIgnore) {
		t.Fatalf("fresh DOC .gitignore differs from its intended source\ngot:\n%s\nwant:\n%s", gotIgnore, wantIgnore)
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
	for _, line := range []string{"repository type: likely CODE\n", "type evidence: CODE score=4 DOC score=2\n", "existing files: README.md\n", "risk of replacing content: medium\n"} {
		if !strings.Contains(out, line) {
			t.Fatalf("assessment missing %q in %q", line, out)
		}
	}
}

func TestExistingRepositoryWarningsExplainTheEffectAndRecovery(t *testing.T) {
	t.Run("whole file replacement", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("local rules without boundary\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runAt(t, root, "--flavor", "doc")
		if code != 0 {
			t.Fatalf("code=%d stderr=%q", code, stderr)
		}
		for _, want := range []string{
			"existing governance files detected; Govna will report whether each file is written, merged, or preserved",
			"warning: AGENTS.md has no `## Project Rules` boundary; replacing the whole file because the named Govna/local boundary is missing",
		} {
			if !strings.Contains(stderr, want) {
				t.Errorf("stderr %q omits %q", stderr, want)
			}
		}
		if !strings.Contains(stdout, "wrote AGENTS.md (Govna-managed file)") {
			t.Errorf("stdout=%q", stdout)
		}
	})

	t.Run("manual boundary merge", func(t *testing.T) {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("canon\n\n## Project Rules\n\n- Local.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(root, "govna"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "govna", "build-release.md"), []byte("local release rules\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, stderr, code := runAt(t, root, "--flavor", "code", "--stack", "rust")
		if code != 0 {
			t.Fatalf("code=%d stderr=%q", code, stderr)
		}
		want := "warning: govna/build-release.md has no `## Project Practices` boundary; kept the existing file; add the named boundary and merge the Govna-managed section manually"
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr %q omits %q", stderr, want)
		}
		content, err := os.ReadFile(filepath.Join(root, "govna", "ac1-govna-apply.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "kept existing file; add the missing Govna/local boundary and merge the Govna-managed section manually") {
			t.Errorf("adoption AC omits recovery action: %s", content)
		}
	})
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
	code := Run([]string{"-f", "doc", "-g"}, &out, &err, d, testProgramVersion, func(name string, args ...string) ([]byte, error) {
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
		if Run(args, &out, &err, t.TempDir(), testProgramVersion, nil) != 2 {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestUnsupportedCODEStacksNameSupportedChoices(t *testing.T) {
	for _, stack := range []string{"Java", "Node", "Python"} {
		t.Run(stack, func(t *testing.T) {
			_, stderr, code := runAt(t, t.TempDir(), "--flavor", "code", "--stack", stack)
			if code != 1 || !strings.Contains(stderr, "use Go, Rust, Swift, or Terraform") {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
		})
	}
}
