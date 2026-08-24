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
	if !strings.Contains(string(a), "govna/parity-check.sh") || strings.Contains(string(b), "govna/parity-check.sh") {
		t.Fatal("root-only parity-check boundary is incorrect")
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
	want := "status --short\nadd .\ncommit -m release\ntag v1.2.3\npush origin v1.2.3\npush origin\n"
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
}
