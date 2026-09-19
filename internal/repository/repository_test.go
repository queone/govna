package repository

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
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

func TestValidatePath(t *testing.T) {
	for _, valid := range []string{"README.md", "govna/audit.md", "a/b/c.txt", ".gitignore"} {
		if err := ValidatePath(valid); err != nil {
			t.Errorf("%q rejected: %v", valid, err)
		}
	}
	for _, tc := range []struct{ path, want string }{
		{"", "is empty"},
		{"/etc/passwd", "is an absolute path"},
		{`govna\audit.md`, "contains a backslash"},
		{"govna/a\x01b.md", "contains a control character"},
		{"govna//audit.md", "contains an empty path component"},
		{"govna/audit.md/", "contains an empty path component"},
		{"./govna/audit.md", `contains a "." component`},
		{"../secret.md", `contains a ".." component`},
		{"govna/../../secret.md", `contains a ".." component`},
	} {
		if err := ValidatePath(tc.path); err == nil || err.Error() != tc.want {
			t.Errorf("%q err=%v want %q", tc.path, err, tc.want)
		}
	}
}

func newSentinel(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sentinel.md")
	if err := os.WriteFile(path, []byte("sentinel\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertSentinel(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "sentinel\n" {
		t.Fatalf("sentinel changed: %q err=%v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("sentinel mode changed: %v err=%v", info.Mode(), err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Fatalf("sentinel directory gained entries: %v err=%v", entries, err)
	}
}

func TestOpenResolvesRootAlias(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	access, err := Open(alias)
	if err != nil {
		t.Fatal(err)
	}
	defer access.Close()
	want, err := filepath.EvalSymlinks(real)
	if err != nil || access.Dir() != want {
		t.Fatalf("dir=%s want %s err=%v", access.Dir(), want, err)
	}
	if err := access.WriteFile("nested/dir/file.txt", []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(real, "nested", "dir", "file.txt")); err != nil || string(got) != "x\n" {
		t.Fatalf("nested write got=%q err=%v", got, err)
	}
	if regular, err := access.Regular("nested/dir/file.txt"); err != nil || !regular {
		t.Fatalf("regular=%v err=%v", regular, err)
	}
	if regular, err := access.Regular("nested/absent.txt"); err != nil || regular {
		t.Fatalf("absent regular=%v err=%v", regular, err)
	}
}

func TestPreflightRejectsLinksAndNonRegularEntries(t *testing.T) {
	root := t.TempDir()
	sentinel := newSentinel(t)
	at := func(name string) string { return filepath.Join(root, filepath.FromSlash(name)) }
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Mkdir(at("docs"), 0o755))
	must(os.WriteFile(at("docs/readme.md"), []byte("inside\n"), 0o644))
	must(os.Symlink(sentinel, at("escaping.md")))
	must(os.Symlink("docs/readme.md", at("inside.md")))
	must(os.Symlink("missing.md", at("dangling.md")))
	must(os.Symlink("escaping.md", at("hop.md")))
	must(os.Symlink(filepath.Dir(sentinel), at("linkdir")))
	must(os.Mkdir(at("dir.md"), 0o755))
	must(os.WriteFile(at("notdir"), []byte("file\n"), 0o644))
	must(syscall.Mkfifo(at("fifo.md"), 0o600))
	access, err := Open(root)
	must(err)
	defer access.Close()
	for _, tc := range []struct{ rel, want string }{
		{"escaping.md", "escaping.md is a symbolic link; replace the link with a regular file and retry"},
		{"inside.md", "inside.md is a symbolic link; replace the link with a regular file and retry"},
		{"dangling.md", "dangling.md is a symbolic link; replace the link with a regular file and retry"},
		{"hop.md", "hop.md is a symbolic link; replace the link with a regular file and retry"},
		{"linkdir/file.md", "linkdir is a symbolic link; replace the link with a real directory and retry"},
		{"dir.md", "dir.md is a directory, not a regular file; move the directory aside and retry"},
		{"notdir/file.md", "notdir is not a directory; move the file aside and retry"},
		{"fifo.md", "fifo.md is not a regular file; remove the special file and retry"},
		{"../outside.md", `../outside.md contains a ".." component; use a normalized repository-relative path`},
	} {
		err := access.Preflight(tc.rel)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%s err=%v want %q", tc.rel, err, tc.want)
		}
		if _, ok := errors.AsType[*PathError](err); !ok {
			t.Errorf("%s error is not a PathError", tc.rel)
		}
		if _, readErr := access.ReadFile(tc.rel); readErr == nil || readErr.Error() != tc.want {
			t.Errorf("%s read err=%v", tc.rel, readErr)
		}
		if writeErr := access.WriteFile(tc.rel, []byte("x\n"), 0o644); writeErr == nil || writeErr.Error() != tc.want {
			t.Errorf("%s write err=%v", tc.rel, writeErr)
		}
	}
	for _, ok := range []string{"docs/readme.md", "docs/new.md", "new/deep/file.md"} {
		if err := access.Preflight(ok); err != nil {
			t.Errorf("%s rejected: %v", ok, err)
		}
	}
	for rel, want := range map[string]string{"escaping.md": sentinel, "inside.md": "docs/readme.md", "dangling.md": "missing.md"} {
		if got, err := access.Readlink(rel); err != nil || got != want {
			t.Errorf("Readlink(%s)=%q err=%v want %q", rel, got, err, want)
		}
	}
	if _, err := access.Readlink("../outside.md"); err == nil {
		t.Error("Readlink accepted an escaping path")
	}
	if _, err := access.Readlink("linkdir/secret.txt"); err == nil {
		t.Error("Readlink followed a linked directory out of the root")
	}
	if got, err := os.ReadFile(at("docs/readme.md")); err != nil || string(got) != "inside\n" {
		t.Fatalf("in-repository link target changed: %q err=%v", got, err)
	}
	assertSentinel(t, sentinel)
}

func TestContainedAccessRejectsSubstitutionAfterPreflight(t *testing.T) {
	root := t.TempDir()
	sentinel := newSentinel(t)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(root, "README.md"), []byte("inside\n"), 0o644))
	must(os.Mkdir(filepath.Join(root, "govna"), 0o755))
	access, err := Open(root)
	must(err)
	defer access.Close()
	must(access.Preflight("README.md"))
	must(access.Preflight("govna/audit.md"))
	must(os.Remove(filepath.Join(root, "README.md")))
	must(os.Symlink(sentinel, filepath.Join(root, "README.md")))
	must(os.Remove(filepath.Join(root, "govna")))
	must(os.Symlink(filepath.Dir(sentinel), filepath.Join(root, "govna")))
	if data, err := access.readContained("README.md"); err == nil {
		t.Fatalf("contained read followed a substituted link: %q", data)
	}
	if err := access.writeContained("README.md", []byte("x\n"), 0o644); err == nil {
		t.Fatal("contained write followed a substituted link")
	}
	if err := access.writeContained("govna/audit.md", []byte("x\n"), 0o644); err == nil {
		t.Fatal("contained write followed a substituted directory link")
	}
	assertSentinel(t, sentinel)
}

func TestReadHookInjectsDeterministicFailures(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("inside\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := ReadHook
	t.Cleanup(func() { ReadHook = original })
	ReadHook = func(r *os.Root, rel string) ([]byte, error) {
		if rel == "README.md" {
			return nil, &fs.PathError{Op: "open", Path: rel, Err: fs.ErrPermission}
		}
		return original(r, rel)
	}
	access, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer access.Close()
	if _, err := access.ReadFile("README.md"); !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("read err=%v", err)
	}
	if _, present, err := access.ReadOptional("README.md"); present || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("optional present=%v err=%v", present, err)
	}
	if _, present, err := access.ReadOptional("absent.md"); present || err != nil {
		t.Fatalf("absent present=%v err=%v", present, err)
	}
}

func TestAuditPreconditionsRejectLinkedAgents(t *testing.T) {
	d := t.TempDir()
	if err := os.WriteFile(filepath.Join(d, "real.md"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.md", filepath.Join(d, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	if err := RequireAdopted(d); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("linked AGENTS.md accepted: %v", err)
	}
}

func TestFlavorRejectsLinkedMetadata(t *testing.T) {
	root := t.TempDir()
	sentinel := newSentinel(t)
	if err := os.Mkdir(filepath.Join(root, "govna"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sentinel, filepath.Join(root, "govna", "metadata.txt")); err != nil {
		t.Fatal(err)
	}
	_, err := Flavor(root, "")
	if err == nil || !strings.Contains(err.Error(), "govna/metadata.txt is a symbolic link; replace the link with a regular file and retry") {
		t.Fatalf("linked metadata err=%v", err)
	}
	assertSentinel(t, sentinel)
}

// Stub the Claude Code version probe so no test runs the real program.
func init() { stubAgentVersion("2.1.277 (Claude Code)", nil) }

func stubAgentVersion(output string, err error) {
	AgentVersionHook = func() ([]byte, error) { return []byte(output), err }
}

func TestUpgradeHintComparesVersions(t *testing.T) {
	defer stubAgentVersion("2.1.277 (Claude Code)", nil)
	hint := func(version string) string {
		return "hint: Claude Code " + version + ` cannot read AGENTS.md; upgrade to v2.1.277 or later with "claude update"`
	}
	for _, tc := range []struct {
		output string
		err    error
		want   string
	}{
		{"2.1.276 (Claude Code)", nil, hint("2.1.276")},
		{"1.0.0 (Claude Code)", nil, hint("1.0.0")},
		{"2.0.999 (Claude Code)", nil, hint("2.0.999")},
		{"2.1.276-beta.1 (Claude Code)\n", nil, hint("2.1.276-beta.1")},
		{"2.1.277 (Claude Code)", nil, ""},
		{"2.1.278 (Claude Code)", nil, ""},
		{"2.2.0 (Claude Code)", nil, ""},
		{"3.0.0 (Claude Code)", nil, ""},
		{"", os.ErrNotExist, ""},
		{"2.1.276 (Claude Code)", os.ErrDeadlineExceeded, ""},
		{"", nil, ""},
		{"Claude Code, probably", nil, ""},
		{"2.1 (Claude Code)", nil, ""},
		{"2.1.x (Claude Code)", nil, ""},
	} {
		stubAgentVersion(tc.output, tc.err)
		if got := UpgradeHint(); got != tc.want {
			t.Errorf("UpgradeHint for %q err=%v = %q want %q", tc.output, tc.err, got, tc.want)
		}
	}
}

func TestAgentFileStatesAndHints(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, root string)
		state AgentFileState
		hint  bool
	}{
		{"absent", func(*testing.T, string) {}, AgentFileAbsent, false},
		{"retired link", func(t *testing.T, root string) {
			if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
				t.Fatal(err)
			}
		}, AgentFileRetiredLink, false},
		{"foreign link", func(t *testing.T, root string) {
			if err := os.Symlink("elsewhere.md", filepath.Join(root, "CLAUDE.md")); err != nil {
				t.Fatal(err)
			}
		}, AgentFileOwned, true},
		{"dotted link", func(t *testing.T, root string) {
			if err := os.Symlink("./AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
				t.Fatal(err)
			}
		}, AgentFileOwned, true},
		{"regular file", func(t *testing.T, root string) {
			if err := os.WriteFile(filepath.Join(root, "CLAUDE.md"), []byte("mine\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, AgentFileOwned, true},
		{"directory", func(t *testing.T, root string) {
			if err := os.Mkdir(filepath.Join(root, "CLAUDE.md"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, AgentFileAbsent, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			access, err := Open(root)
			if err != nil {
				t.Fatal(err)
			}
			defer access.Close()
			state, err := access.AgentFile()
			if err != nil || state != tc.state {
				t.Fatalf("state=%v err=%v want %v", state, err, tc.state)
			}
			var hints bytes.Buffer
			access.WriteAgentHints(&hints)
			want := ""
			if tc.hint {
				want = DeletableHint + "\n"
			}
			if hints.String() != want {
				t.Fatalf("hints=%q want %q", hints.String(), want)
			}
		})
	}
}

func TestAgentVersionProbeReadsAndBoundsTheProgram(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake program is a shell script")
	}
	fake := func(t *testing.T, script string) {
		t.Helper()
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+"/usr/bin"+string(os.PathListSeparator)+"/bin")
	}
	t.Run("reads the version", func(t *testing.T) {
		fake(t, `echo "2.1.276 (Claude Code)"`)
		out, err := probeAgentVersion()
		if err != nil || strings.TrimSpace(string(out)) != "2.1.276 (Claude Code)" {
			t.Fatalf("out=%q err=%v", out, err)
		}
	})
	t.Run("absent program", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		if _, err := probeAgentVersion(); err == nil {
			t.Fatal("absent program reported no error")
		}
	})
	t.Run("failing program", func(t *testing.T) {
		fake(t, "exit 3")
		if _, err := probeAgentVersion(); err == nil {
			t.Fatal("failing program reported no error")
		}
	})
	t.Run("hung program", func(t *testing.T) {
		fake(t, "sleep 30\necho \"2.1.276 (Claude Code)\"")
		start := time.Now()
		_, err := probeAgentVersion()
		if err == nil {
			t.Fatal("hung program reported no error")
		}
		if elapsed := time.Since(start); elapsed > agentVersionLimit+2*time.Second {
			t.Fatalf("probe took %s, limit %s", elapsed, agentVersionLimit)
		}
	})
}
