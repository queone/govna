package buildtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func run(t *testing.T, dir string, input string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("/bin/bash", args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestGoBuildRejectsValidationTokens(t *testing.T) {
	root := repoRoot(t)
	script, err := os.ReadFile(filepath.Join(root, "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"_validation_token()", "_validation_fingerprints()", "refresh_validation_token_run()", "==> Validation token:"} {
		if strings.Contains(string(script), forbidden) {
			t.Fatalf("Go build contains token behavior %q", forbidden)
		}
	}
	out, err := run(t, root, "", "./build.sh", "prep", "--validation-token", "secret-evidence", "v9.9.9", "test")
	if err == nil || !strings.Contains(out, "unsupported for Go") || strings.Contains(out, "secret-evidence") {
		t.Fatalf("token rejection: %v: %s", err, out)
	}
}

func TestRenderedGoBuildMatchesRoot(t *testing.T) {
	root := repoRoot(t)
	a, err := os.ReadFile(filepath.Join(root, "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "internal/canon/assets/overlays/code/stacks/go/build.sh.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"Building and validating", "prep: running pre-check build", "prep: running post-check build"} {
		if !strings.Contains(string(a), marker) || !strings.Contains(string(b), marker) {
			t.Fatalf("root/rendered Go build scripts lack shared marker %q", marker)
		}
	}
	if !strings.Contains(string(a), "_validate_root_canon_version") || strings.Contains(string(b), "_validate_root_canon_version") {
		t.Fatal("root-only canon-version boundary is incorrect")
	}
}

func TestReleaseCancellationIsNonMutating(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	script, _ := os.ReadFile(filepath.Join(root, "build.sh"))
	if err := os.WriteFile(filepath.Join(dir, "build.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	before, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	out, err := run(t, dir, "n\n", "./build.sh", "v1.0.0", "cancel")
	if err == nil || !strings.Contains(out, "release aborted") {
		t.Fatalf("unexpected cancellation: %v: %s", err, out)
	}
	after, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if string(before) != string(after) {
		t.Fatalf("cancellation changed Git state")
	}
}

func TestReleaseApprovalAndFailureOrdering(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	script, _ := os.ReadFile(filepath.Join(root, "build.sh"))
	if err := os.WriteFile(filepath.Join(dir, "build.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	approved := `source ./build.sh
_run_git() { shift; printf '%s\n' "$*" >> operations; return 0; }
_release_rebuild_and_verify() { printf '%s\n' provenance >> operations; return 0; }
printf 'y\n' | rel_run v1.2.3 release
`
	out, err := run(t, dir, "", "-c", approved)
	if err != nil {
		t.Fatalf("approved release fixture: %v: %s", err, out)
	}
	operations, err := os.ReadFile(filepath.Join(dir, "operations"))
	if err != nil {
		t.Fatal(err)
	}
	want := "status --short\nadd .\ncommit -m release\ntag v1.2.3\nprovenance\npush origin v1.2.3\npush origin\n"
	if string(operations) != want {
		t.Fatalf("operations:\n%s\nwant:\n%s", operations, want)
	}

	failing := `source ./build.sh
_run_git() { name="$1"; shift; printf '%s\n' "$*" >> failed-operations; if [ "$name" = 'git tag' ]; then _git_err='git tag failed: exit status 9'; return 1; fi; return 0; }
printf 'y\n' | rel_run v1.2.3 release
`
	out, err = run(t, dir, "", "-c", failing)
	if err == nil || !strings.Contains(out, "completed before failure: git add, git commit") {
		t.Fatalf("failed release fixture: %v: %s", err, out)
	}
	failed, err := os.ReadFile(filepath.Join(dir, "failed-operations"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(failed), "push origin") {
		t.Fatalf("release continued after failure: %s", failed)
	}

	provenanceFailure := `source ./build.sh
_run_git() { shift; printf '%s\n' "$*" >> provenance-failed-operations; return 0; }
_release_rebuild_and_verify() { return 1; }
printf 'y\n' | rel_run v1.2.3 release
`
	if out, err = run(t, dir, "", "-c", provenanceFailure); err == nil {
		t.Fatalf("provenance failure accepted: %s", out)
	}
	provenanceFailed, err := os.ReadFile(filepath.Join(dir, "provenance-failed-operations"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(provenanceFailed), "push origin") {
		t.Fatalf("release pushed after provenance failure: %s", provenanceFailed)
	}
}

func TestCanonAssetChangesRequireVersionIncrease(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	for path, content := range map[string]string{
		"build.sh":                    string(mustRead(t, filepath.Join(root, "build.sh"))),
		"internal/canon/canon.go":     "package canon\nconst Version = \"1.0.0\"\n",
		"cmd/govna/main.go":           "package main\nconst canonVersion = \"1.0.0\"\n",
		"internal/canon/assets/a.txt": "one\n",
	} {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitFixture(t, dir, "init", "-q")
	gitFixture(t, dir, "config", "user.name", "Fixture")
	gitFixture(t, dir, "config", "user.email", "fixture@example.com")
	gitFixture(t, dir, "add", ".")
	gitFixture(t, dir, "commit", "-qm", "baseline")
	gitFixture(t, dir, "tag", "v1.0.0")
	if err := os.WriteFile(filepath.Join(dir, "internal/canon/assets/a.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, dir, "", "-c", "source ./build.sh; _color_init; _validate_root_canon_version")
	if err == nil || !strings.Contains(out, "increase internal/canon.Version") {
		t.Fatalf("unchanged version accepted: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/canon/canon.go"), []byte("package canon\nconst Version = \"1.1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd/govna/main.go"), []byte("package main\nconst canonVersion = \"1.1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, dir, "", "-c", "source ./build.sh; _color_init; _validate_root_canon_version"); err != nil {
		t.Fatalf("increased version rejected: %v: %s", err, out)
	}
}

func TestTaggedBinaryProvenanceVerification(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	tooling := mustRead(t, filepath.Join(root, "build.sh"))
	if err := os.WriteFile(filepath.Join(dir, "tooling.sh"), tooling, 0o644); err != nil {
		t.Fatal(err)
	}
	gopath := filepath.Join(dir, "gopath")
	fakebin := filepath.Join(dir, "fakebin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	build := "#!/bin/bash\nmkdir -p \"$FAKE_GOPATH/bin\"\nprintf '#!/bin/bash\\nprintf \\\"govna v1.2.3\\\\n\\\"\\n' > \"$FAKE_GOPATH/bin/govna\"\nchmod +x \"$FAKE_GOPATH/bin/govna\"\n"
	if err := os.WriteFile(filepath.Join(dir, "build.sh"), []byte(build), 0o755); err != nil {
		t.Fatal(err)
	}
	goTool := "#!/bin/bash\nif [ \"$1\" = env ]; then printf '%s\\n' \"$FAKE_GOPATH\"; exit; fi\nif [ \"$1\" = version ]; then printf 'path\\tgovna\\nbuild\\tvcs.revision=%s\\nbuild\\tvcs.modified=false\\n' \"$(git rev-parse HEAD)\"; exit; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(fakebin, "go"), []byte(goTool), 0o755); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, dir, "init", "-q")
	gitFixture(t, dir, "config", "user.name", "Fixture")
	gitFixture(t, dir, "config", "user.email", "fixture@example.com")
	gitFixture(t, dir, "add", ".")
	gitFixture(t, dir, "commit", "-qm", "release")
	cmd := exec.Command("/bin/bash", "-c", "source ./tooling.sh; _color_init; _release_rebuild_and_verify v1.2.3")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb", "FAKE_GOPATH="+gopath, "PATH="+fakebin+":"+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("provenance rejected: %v: %s", err, out)
	}
}

func TestUtilityDeclarationValidationAndAtomicInstall(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tooling.sh"), mustRead(t, filepath.Join(root, "build.sh")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nconst programVersion = \"1.2.3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	compiled := filepath.Join(dir, "compiled")
	if err := os.WriteFile(compiled, []byte("#!/bin/bash\nprintf 'widget 1.2.3\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "bin", "widget")
	if err := os.Mkdir(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "source ./tooling.sh; _color_init; [ \"$(_extract_program_version main.go)\" = 1.2.3 ]; _install_compiled_utility ./compiled ./bin/widget widget 1.2.3"
	if out, err := run(t, dir, "", "-c", script); err != nil {
		t.Fatalf("install failed: %v: %s", err, out)
	}
	if out, err := exec.Command(destination, "--version").CombinedOutput(); err != nil || string(out) != "widget 1.2.3\n" {
		t.Fatalf("installed output: %v: %s", err, out)
	}
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("compiled", destination); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, dir, "", "-c", "source ./tooling.sh; _color_init; _install_compiled_utility ./compiled ./bin/widget widget 1.2.3"); err == nil || !strings.Contains(out, "must be absent or a regular file") {
		t.Fatalf("unsafe destination accepted: %v: %s", err, out)
	}
}

func TestPreparationMutationFailureAndCleanup(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tooling.sh"), mustRead(t, filepath.Join(root, "build.sh")), 0o644); err != nil {
		t.Fatal(err)
	}
	versionFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(versionFile, []byte("package main\nconst programVersion = \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	owned := filepath.Join(dir, "owned")
	if err := os.Mkdir(owned, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "source ./tooling.sh; _color_init; _prep_apply_version_bump main.go programVersion 1.1.0; _build_owned_dir=owned; _cleanup_build_owned; ! _prep_apply_version_bump main.go unknown 2.0.0"
	if out, err := run(t, dir, "", "-c", script); err != nil {
		t.Fatalf("prep fixture failed: %v: %s", err, out)
	}
	content := string(mustRead(t, versionFile))
	if !strings.Contains(content, `programVersion = "1.1.0"`) || strings.Contains(content, "2.0.0") {
		t.Fatalf("version mutation=%s", content)
	}
	if _, err := os.Stat(owned); !os.IsNotExist(err) {
		t.Fatalf("owned scratch remains: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".govna-install-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("install temporaries=%v err=%v", matches, err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func gitFixture(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
