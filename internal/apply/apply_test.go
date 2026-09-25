package apply

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/repository"
)

const testProgramVersion = "9.8.7"

const upgradeHint = `hint: Claude Code 2.1.276 cannot read AGENTS.md; upgrade to v2.1.277 or later with "claude update"`

// Stub the Claude Code version probe so no test runs the real program.
func init() { stubAgentVersion("2.1.277 (Claude Code)", nil) }

func stubAgentVersion(output string, err error) {
	repository.AgentVersionHook = func() ([]byte, error) { return []byte(output), err }
}

func runAt(t *testing.T, d string, args ...string) (string, string, int) {
	t.Helper()
	var out, err bytes.Buffer
	code := Run(args, &out, &err, d, testProgramVersion, func(string, ...string) ([]byte, error) { return nil, nil })
	return out.String(), err.String(), code
}

func TestAdoptionVersionAxesAndInstructions(t *testing.T) {
	created := adoption(7, "widget", "CODE", testProgramVersion, nil, repository.AgentFileAbsent)
	for _, want := range []string{
		"# AC7 Review Files Added by Govna",
		"Govna executable v9.8.7 added its embedded governance files (canon v0.68.0) for the CODE repository widget.",
		"Govna executable v9.8.7 added its embedded governance files (canon v0.68.0). The list below records whether each file was written, merged, or preserved.",
		"Files Govna processed:",
		"- Files not listed above.",
		"**AT1** [Manual] [Pre-release gate] — Verify AGENTS.md reflects the repository's actual practices.",
		"**AT2** [Manual] [Pre-release gate] — Verify govna/roles.md reflects the repository's delivery model (Operator + Director).",
		"`PENDING` — apply emission; awaiting explicit Director Audit.",
	} {
		if !strings.Contains(created, want) {
			t.Errorf("created adoption omits %q", want)
		}
	}
	for _, invalid := range []string{"Applied govna v0.68.0", "Director reads", "review applied governance", "overlay", "consumer-owned", "CLAUDE.md", "**AT3**"} {
		if strings.Contains(created, invalid) {
			t.Errorf("created adoption retains invalid text %q", invalid)
		}
	}
	kept := adoption(8, "widget", "CODE", testProgramVersion, nil, repository.AgentFileOwned)
	for _, want := range []string{
		"- `CLAUDE.md` (existing file kept — Claude Code reads it instead of AGENTS.md; see the apply hint)\n",
		"**AT3** [Manual] [Pre-release gate] — Verify CLAUDE.md is deleted or deliberately kept.",
	} {
		if !strings.Contains(kept, want) {
			t.Errorf("kept adoption omits %q", want)
		}
	}
	removed := adoption(9, "widget", "CODE", testProgramVersion, nil, repository.AgentFileRetiredLink)
	for _, want := range []string{
		"- `CLAUDE.md` (retired Govna link removed)\n",
		"**AT3** [Automated] [Pre-release gate] — Verify CLAUDE.md no longer exists.",
	} {
		if !strings.Contains(removed, want) {
			t.Errorf("removed-link adoption omits %q", want)
		}
	}
}
func TestFreshAndReapply(t *testing.T) {
	d := filepath.Join(t.TempDir(), "widget")
	os.Mkdir(d, 0755)
	out, err, code := runAt(t, d, "-f", "code", "-s", "rust")
	if code != 0 {
		t.Fatalf("%s", err)
	}
	for _, p := range []string{"AGENTS.md", "govna/ac1-govna-apply.md"} {
		if !exists(filepath.Join(d, p)) {
			t.Fatalf("missing %s", p)
		}
	}
	if _, statErr := os.Lstat(filepath.Join(d, "CLAUDE.md")); !os.IsNotExist(statErr) {
		t.Fatalf("fresh apply created CLAUDE.md: %v", statErr)
	}
	if strings.Contains(out, "symlink") || strings.Contains(out, "CLAUDE.md") || strings.Contains(err, "CLAUDE.md") {
		t.Fatalf("fresh apply mentions the link: stdout=%q stderr=%q", out, err)
	}
	if record, _ := os.ReadFile(filepath.Join(d, "govna/ac1-govna-apply.md")); strings.Contains(string(record), "CLAUDE.md") {
		t.Fatalf("fresh adoption AC mentions CLAUDE.md:\n%s", record)
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
	if !strings.Contains(err, repository.DeletableHint+"\n") {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(d, "CLAUDE.md")); string(b) != "mine\n" {
		t.Fatal("CLAUDE.md changed")
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

func TestApplyAcceptsRootAliasAndKeepsForeignLink(t *testing.T) {
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
	if got, err := os.Readlink(filepath.Join(real, "CLAUDE.md")); err != nil || got != "elsewhere.md" {
		t.Fatalf("CLAUDE.md link=%q err=%v", got, err)
	}
	assertKeptAgentFile(t, real, out, stderr)
}

func assertKeptAgentFile(t *testing.T, d, stdout, stderr string) {
	t.Helper()
	if !strings.Contains(stderr, repository.DeletableHint+"\n") || strings.Count(stderr, "hint:") != 1 {
		t.Fatalf("stderr=%q", stderr)
	}
	if strings.Contains(stdout, "removed CLAUDE.md") {
		t.Fatalf("stdout claims a removal: %s", stdout)
	}
	record, err := os.ReadFile(filepath.Join(d, "govna", "ac1-govna-apply.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- `CLAUDE.md` (existing file kept — Claude Code reads it instead of AGENTS.md; see the apply hint)\n",
		"**AT3** [Manual] [Pre-release gate] — Verify CLAUDE.md is deleted or deliberately kept.\n",
	} {
		if !strings.Contains(string(record), want) {
			t.Errorf("adoption AC omits %q", want)
		}
	}
}

func TestApplyKeepsRegularAgentFile(t *testing.T) {
	d := filepath.Join(t.TempDir(), "handbook")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	writeMode(t, filepath.Join(d, "CLAUDE.md"), "local claude\n", 0o600)
	out, stderr, code := runAt(t, d, "-f", "doc")
	if code != 0 {
		t.Fatal(stderr)
	}
	assertFile(t, filepath.Join(d, "CLAUDE.md"), "local claude\n", 0o600)
	assertKeptAgentFile(t, d, out, stderr)
}

func TestApplyRemovesRetiredLink(t *testing.T) {
	d := filepath.Join(t.TempDir(), "handbook")
	if err := os.Mkdir(d, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("AGENTS.md", filepath.Join(d, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	out, stderr, code := runAt(t, d, "-f", "doc")
	if code != 0 {
		t.Fatal(stderr)
	}
	if _, err := os.Lstat(filepath.Join(d, "CLAUDE.md")); !os.IsNotExist(err) {
		t.Fatalf("retired link remains: %v", err)
	}
	if !strings.Contains(out, "removed CLAUDE.md (retired Govna link)\n") {
		t.Fatalf("stdout=%s", out)
	}
	if strings.Contains(stderr, "hint:") {
		t.Fatalf("stderr=%q", stderr)
	}
	record, err := os.ReadFile(filepath.Join(d, "govna", "ac1-govna-apply.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"- `CLAUDE.md` (retired Govna link removed)\n",
		"**AT3** [Automated] [Pre-release gate] — Verify CLAUDE.md no longer exists.\n",
	} {
		if !strings.Contains(string(record), want) {
			t.Errorf("adoption AC omits %q", want)
		}
	}
}

func TestApplyIgnoresAgentFileDirectory(t *testing.T) {
	d := filepath.Join(t.TempDir(), "handbook")
	if err := os.MkdirAll(filepath.Join(d, "CLAUDE.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runAt(t, d, "-f", "doc")
	if code != 0 || strings.Contains(stderr, "hint:") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if info, err := os.Lstat(filepath.Join(d, "CLAUDE.md")); err != nil || !info.IsDir() {
		t.Fatalf("directory changed: %v err=%v", info, err)
	}
	record, err := os.ReadFile(filepath.Join(d, "govna", "ac1-govna-apply.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(record), "CLAUDE.md") {
		t.Errorf("adoption AC mentions the directory:\n%s", record)
	}
}

func TestApplyUpgradeHint(t *testing.T) {
	defer stubAgentVersion("2.1.277 (Claude Code)", nil)
	run := func(t *testing.T) (string, string, int) {
		t.Helper()
		d := filepath.Join(t.TempDir(), "handbook")
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
		return runAt(t, d, "-f", "doc")
	}
	stubAgentVersion("2.1.277 (Claude Code)", nil)
	baseOut, _, _ := run(t)
	for _, tc := range []struct {
		name, output string
		err          error
		want         string
	}{
		{"one patch below", "2.1.276 (Claude Code)", nil, upgradeHint + "\n"},
		{"old major", "1.0.0 (Claude Code)", nil, strings.Replace(upgradeHint, "2.1.276", "1.0.0", 1) + "\n"},
		{"minimum", "2.1.277 (Claude Code)", nil, ""},
		{"next patch", "2.1.278 (Claude Code)", nil, ""},
		{"next minor", "2.2.0 (Claude Code)", nil, ""},
		{"next major", "3.0.0 (Claude Code)", nil, ""},
		{"absent program", "", os.ErrNotExist, ""},
		{"failing probe", "2.1.276 (Claude Code)", os.ErrDeadlineExceeded, ""},
		{"unparseable output", "Claude Code, probably", nil, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stubAgentVersion(tc.output, tc.err)
			out, stderr, code := run(t)
			if code != 0 || stderr != tc.want {
				t.Fatalf("code=%d stderr=%q want=%q", code, stderr, tc.want)
			}
			if normalizeTarget(out) != normalizeTarget(baseOut) {
				t.Fatalf("stdout changed:\n%s\nwant:\n%s", out, baseOut)
			}
		})
	}
}

// normalizeTarget drops the per-test target line so stdout comparisons ignore the temporary path.
func normalizeTarget(out string) string {
	var kept []string
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.HasPrefix(line, "target: ") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
