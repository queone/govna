package apply

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queone/govna/internal/canon"
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
		"Govna executable v9.8.7 added its embedded governance files (canon v0.63.0) for the CODE repository widget.",
		"Govna executable v9.8.7 added its embedded governance files (canon v0.63.0). The list below records whether each file was written, merged, or preserved.",
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
	for _, invalid := range []string{"Applied govna v0.63.0", "Director reads", "review applied governance", "overlay", "consumer-owned"} {
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

func writeMode(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != content {
		t.Fatalf("%s content=%q err=%v want %q", path, data, err, content)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != mode {
		t.Fatalf("%s mode=%v err=%v want %v", path, info.Mode(), err, mode)
	}
}

func newSentinel(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sentinel.md")
	writeMode(t, path, "sentinel\n", 0o600)
	return path
}

func assertSentinel(t *testing.T, path string) {
	t.Helper()
	assertFile(t, path, "sentinel\n", 0o600)
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("sentinel directory gained entries: %v err=%v", entries, err)
	}
}

func TestFirstAdoptionPreservesProjectDocumentsWithoutAgentFiles(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		docs []string
	}{
		{"code", []string{"-f", "code", "-s", "rust"}, []string{"README.md", "CHANGELOG.md", "arch.md", "plan.md"}},
		{"doc", []string{"-f", "doc"}, []string{"README.md", "CHANGELOG.md", "plan.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := filepath.Join(t.TempDir(), "widget")
			if err := os.Mkdir(d, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, doc := range tc.docs {
				writeMode(t, filepath.Join(d, doc), "local "+doc+"\n", 0o600)
			}
			out, stderr, code := runAt(t, d, tc.args...)
			if code != 0 {
				t.Fatal(stderr)
			}
			if strings.Contains(stderr, "existing governance files detected") {
				t.Fatalf("notice no longer keyed to agent instruction files: %s", stderr)
			}
			ac, err := os.ReadFile(filepath.Join(d, "govna", "ac1-govna-apply.md"))
			if err != nil {
				t.Fatal(err)
			}
			for _, doc := range tc.docs {
				assertFile(t, filepath.Join(d, doc), "local "+doc+"\n", 0o600)
				if !strings.Contains(out, "kept "+doc+" (existing file)\n") {
					t.Errorf("stdout omits kept %s: %s", doc, out)
				}
				if !strings.Contains(string(ac), "- `"+doc+"` (kept existing file)\n") {
					t.Errorf("adoption AC omits kept %s", doc)
				}
			}
			for _, written := range []string{"AGENTS.md", "govna/metadata.txt", ".gitignore"} {
				if !exists(filepath.Join(d, filepath.FromSlash(written))) {
					t.Errorf("missing %s", written)
				}
			}
		})
	}
}

func TestReapplyCreatesMissingCounterpartsAndReportsKeptFiles(t *testing.T) {
	d := filepath.Join(t.TempDir(), "widget")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := runAt(t, d, "-f", "code", "-s", "rust"); code != 0 {
		t.Fatal(stderr)
	}
	if err := os.Remove(filepath.Join(d, "arch.md")); err != nil {
		t.Fatal(err)
	}
	writeMode(t, filepath.Join(d, "README.md"), "local readme\n", 0o600)
	out, stderr, code := runAt(t, d, "-f", "code", "-s", "rust")
	if code != 0 || !strings.Contains(stderr, "existing governance files detected") {
		t.Fatalf("code=%d stderr=%s", code, stderr)
	}
	if !strings.Contains(out, "wrote arch.md (Govna-managed file)\n") || !strings.Contains(out, "kept README.md (existing file)\n") {
		t.Fatalf("stdout=%s", out)
	}
	assertFile(t, filepath.Join(d, "README.md"), "local readme\n", 0o600)
	if !exists(filepath.Join(d, "arch.md")) {
		t.Fatal("arch.md not recreated")
	}
	ac, err := os.ReadFile(filepath.Join(d, "govna", "ac2-govna-apply.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- `README.md` (kept existing file)\n", "- `arch.md` (written)\n"} {
		if !strings.Contains(string(ac), want) {
			t.Errorf("adoption AC omits %q", want)
		}
	}
}

func TestMergeBoundaryVariants(t *testing.T) {
	for _, path := range []string{"AGENTS.md", "govna/build-release.md", "govna/development-guidelines.md", "govna/editing-guidelines.md"} {
		boundary, _ := canon.Boundary(path)
		fresh := []byte("fresh canon\n\n" + boundary + "\n\n- Seed.\n")
		for _, tc := range []struct{ name, old, tail string }{
			{"lf", "old canon\n\n" + boundary + "\n\n- Local.\n", boundary + "\n\n- Local.\n"},
			{"crlf", "old canon\r\n\r\n" + boundary + "\r\n\r\n- Local.\r\n", boundary + "\r\n\r\n- Local.\r\n"},
			{"boundary at start", boundary + "\n\n- Local.\n", boundary + "\n\n- Local.\n"},
			{"boundary at eof without newline", "old canon\n" + boundary, boundary},
			{"first exact boundary wins", "old\n" + boundary + "\n- First.\n" + boundary + "\n- Second.\n", boundary + "\n- First.\n" + boundary + "\n- Second.\n"},
		} {
			got, ok := merge([]byte(tc.old), fresh, path)
			want := "fresh canon\n\n" + tc.tail
			if !ok || string(got) != want {
				t.Errorf("%s %s: ok=%v got=%q want=%q", path, tc.name, ok, got, want)
			}
		}
		for _, tc := range []struct{ name, old string }{
			{"trailing space", "old\n" + boundary + " \n- Local.\n"},
			{"deeper heading", "old\n#" + boundary + "\n- Local.\n"},
			{"inline", "old\nprefix " + boundary + "\n- Local.\n"},
			{"missing", "old\n- Local.\n"},
		} {
			if _, ok := merge([]byte(tc.old), fresh, path); ok {
				t.Errorf("%s %s: near match accepted as boundary", path, tc.name)
			}
		}
	}
	if _, ok := merge([]byte("x\n"), []byte("y\n"), "README.md"); ok {
		t.Error("unregistered path merged")
	}
}

func TestMergePreservesLocalTailWithoutAgentFiles(t *testing.T) {
	for _, tc := range []struct{ name, eol string }{{"lf", "\n"}, {"crlf", "\r\n"}} {
		t.Run(tc.name, func(t *testing.T) {
			d := filepath.Join(t.TempDir(), "widget")
			tail := "## Project Practices" + tc.eol + tc.eol + "- Local practice." + tc.eol
			writeMode(t, filepath.Join(d, "govna", "development-guidelines.md"), "stale canon"+tc.eol+tc.eol+tail, 0o644)
			_, stderr, code := runAt(t, d, "-f", "code", "-s", "rust")
			if code != 0 || strings.Contains(stderr, "existing governance files detected") {
				t.Fatalf("code=%d stderr=%s", code, stderr)
			}
			got, err := os.ReadFile(filepath.Join(d, "govna", "development-guidelines.md"))
			if err != nil {
				t.Fatal(err)
			}
			files, err := canon.Render(canon.Config{Flavor: canon.Code, RepoName: "widget", Stack: "Rust"})
			if err != nil {
				t.Fatal(err)
			}
			var head []byte
			for _, file := range files {
				if file.Path == "govna/development-guidelines.md" {
					head, _ = canon.ComparisonRegion(file.Path, file.Content)
				}
			}
			if len(head) == 0 || string(got) != string(head)+tail {
				t.Fatalf("merged=%q\nwant head+%q", got, tail)
			}
			ac, err := os.ReadFile(filepath.Join(d, "govna", "ac1-govna-apply.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(ac), "- `govna/development-guidelines.md` (updated Govna-managed section; kept repository-owned section)\n") {
				t.Fatalf("adoption AC omits merge outcome: %s", ac)
			}
		})
	}
}

func TestApplyRejectsUnsafeDestinationsBeforeWriting(t *testing.T) {
	sentinel := newSentinel(t)
	outside := filepath.Dir(sentinel)
	link := func(target, name string) func(string) {
		return func(d string) {
			if err := os.Symlink(target, filepath.Join(d, filepath.FromSlash(name))); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, tc := range []struct {
		name  string
		setup func(string)
		want  string
	}{
		{"escaping leaf link", link(sentinel, "plan.md"), "apply: check destination plan.md: plan.md is a symbolic link; replace the link with a regular file and retry"},
		{"linked intermediate directory", link(outside, "govna"), "apply: check destination govna/README.md: govna is a symbolic link; replace the link with a real directory and retry"},
		{"dangling link", link("missing.md", "plan.md"), "apply: check destination plan.md: plan.md is a symbolic link; replace the link with a regular file and retry"},
		{"multi-hop link", func(d string) { link("hop.md", "plan.md")(d); link(sentinel, "hop.md")(d) }, "apply: check destination plan.md: plan.md is a symbolic link; replace the link with a regular file and retry"},
		{"in-repository link", func(d string) {
			writeMode(t, filepath.Join(d, "docs", "plan.md"), "docs plan\n", 0o644)
			link("docs/plan.md", "plan.md")(d)
		}, "apply: check destination plan.md: plan.md is a symbolic link; replace the link with a regular file and retry"},
		{"directory at file path", func(d string) {
			if err := os.Mkdir(filepath.Join(d, "plan.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "apply: check destination plan.md: plan.md is a directory, not a regular file; move the directory aside and retry"},
		{"directory at CLAUDE.md", func(d string) {
			if err := os.Mkdir(filepath.Join(d, "CLAUDE.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "apply: check destination CLAUDE.md: CLAUDE.md is a directory, not a regular file; move the directory aside and retry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := filepath.Join(t.TempDir(), "widget")
			if err := os.Mkdir(d, 0o755); err != nil {
				t.Fatal(err)
			}
			writeMode(t, filepath.Join(d, "build.sh"), "#!/bin/sh\n", 0o600)
			tc.setup(d)
			out, stderr, code := runAt(t, d, "-f", "code", "-s", "rust")
			if code != 1 || !strings.Contains(stderr, tc.want) {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if strings.Contains(out, "wrote ") || strings.Contains(out, "symlink ") {
				t.Fatalf("output claims writes: %s", out)
			}
			for _, untouched := range []string{".gitignore", "AGENTS.md", "govna/ac1-govna-apply.md", "govna/README.md"} {
				if exists(filepath.Join(d, filepath.FromSlash(untouched))) && tc.name != "linked intermediate directory" {
					t.Errorf("%s created before validation finished", untouched)
				}
			}
			assertFile(t, filepath.Join(d, "build.sh"), "#!/bin/sh\n", 0o600)
			assertSentinel(t, sentinel)
			if tc.name == "in-repository link" {
				assertFile(t, filepath.Join(d, "docs", "plan.md"), "docs plan\n", 0o644)
				if info, err := os.Lstat(filepath.Join(d, "plan.md")); err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("in-repository link changed: %v err=%v", info, err)
				}
			}
		})
	}
}

func TestApplyAcceptsRootAliasAndReplacesClaudeLink(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere.md", filepath.Join(real, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runAt(t, alias, "-f", "doc")
	if code != 0 {
		t.Fatal(stderr)
	}
	if !strings.Contains(out, "target: "+alias+"\n") {
		t.Fatalf("stdout=%s", out)
	}
	for _, p := range []string{"AGENTS.md", "govna/metadata.txt", "govna/ac1-govna-apply.md"} {
		if info, err := os.Lstat(filepath.Join(real, filepath.FromSlash(p))); err != nil || !info.Mode().IsRegular() {
			t.Errorf("%s missing from resolved root: %v", p, err)
		}
	}
	if got, err := os.Readlink(filepath.Join(real, "CLAUDE.md")); err != nil || got != "AGENTS.md" {
		t.Fatalf("CLAUDE.md link=%q err=%v", got, err)
	}
}
