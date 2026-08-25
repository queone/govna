package remove

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/emission"
)

const testProgramVersion = "9.8.7"

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files, err := canon.Render(canon.Config{Flavor: canon.Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		path := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, file.Content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/widget\n\ngo 1.27.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	git(t, root, "config", "user.name", "Fixture")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "fixture")
	return root
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	args = append([]string{"-C", root}, args...)
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestRemovalFreshAndIdempotent(t *testing.T) {
	root := fixture(t)
	var stdout, stderr bytes.Buffer
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if stdout.String() != "wrote govna/ac1-govna-rm-v0.33.0.md\n" {
		t.Fatalf("stdout=%q", stdout.String())
	}
	path := filepath.Join(root, "govna", "ac1-govna-rm-v0.33.0.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !emission.VerifyGuardedBody(before, emission.RemovalMarkerPrefix) {
		t.Fatal("invalid marker")
	}
	markerPrefix := "<!-- govna-rm: emitted-by govna executable v9.8.7 with embedded canon v0.33.0 sha256:"
	if !strings.HasPrefix(string(before), markerPrefix) {
		t.Fatalf("unexpected marker: %s", before)
	}
	stdout.Reset()
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 0 {
		t.Fatalf("rerun code=%d stderr=%q", code, stderr.String())
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("unchanged rerun rewrote body")
	}
	_, rawBody, ok := strings.Cut(string(after), "\n")
	if !ok {
		t.Fatal("guarded body has no marker line")
	}
	legacyBody := strings.Replace(
		rawBody,
		"`PENDING` — removal emission; awaiting explicit Director Audit.",
		"`PENDING` — Emitted by `govna rm`; awaiting Director review.",
		1,
	)
	if legacyBody == rawBody {
		t.Fatal("failed to construct legacy removal body")
	}
	legacy := legacyGuardedBody(emission.RemovalMarkerPrefix, "v"+canon.Version, []byte(legacyBody))
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 0 {
		t.Fatalf("legacy upgrade code=%d stderr=%q", code, stderr.String())
	}
	upgraded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(upgraded), markerPrefix) || bytes.Equal(upgraded, legacy) || strings.Contains(string(upgraded), "awaiting Director review") {
		t.Fatalf("legacy marker not upgraded: %s", upgraded)
	}
	matches, err := filepath.Glob(filepath.Join(root, "govna", "ac*-govna-rm-v0.33.0.md"))
	if err != nil || len(matches) != 1 || matches[0] != path {
		t.Fatalf("same-canon upgrade changed stub identity: matches=%v err=%v", matches, err)
	}
	if err := os.WriteFile(path, append(upgraded, []byte("edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 1 || stderr.String() != "rm: govna/ac1-govna-rm-v0.33.0.md has been edited since last emission — delete or rename the emitted file before re-running\n" {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestRemovalClassificationAndTraversal(t *testing.T) {
	root := fixture(t)
	if err := os.WriteFile(filepath.Join(root, "govna", "roles.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "custom.md"), []byte("local\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "govna", "preserve.txt"), []byte("govna-preserve-v1\nbuild.sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..", filepath.Join(root, "linked-dir")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "govna", "roles.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../README.md", filepath.Join(root, "govna", "roles.md")); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mkfifo(filepath.Join(root, "local.pipe"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := canon.Render(canon.Config{Flavor: canon.Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := classify(root, files, map[string]bool{"build.sh": true}, "govna/ac1-govna-rm-v0.33.0.md")
	if err != nil {
		t.Fatal(err)
	}
	assertRoute(t, a.InScope, "CLAUDE.md", "delete symlink")
	assertRoute(t, a.OutOfScope, "build.sh", "keep")
	assertRoute(t, a.OutOfScope, "custom.md", "keep")
	assertRoute(t, a.OutOfScope, "linked-dir", "keep")
	assertRoute(t, a.OutOfScope, "local.pipe", "keep")
	assertRoute(t, a.Review, "README.md", "hybrid")
	assertRoute(t, a.Review, "govna/roles.md", "ambiguity")
	for _, route := range a.Review {
		if route.Path == "govna/roles.md" && route.Reason != "non-regular canon path" {
			t.Fatalf("roles reason=%q", route.Reason)
		}
	}
	if a.ControlState == nil || a.ControlState.Path != "govna/preserve.txt" {
		t.Fatalf("control=%v", a.ControlState)
	}
}

func TestRemovalGolden(t *testing.T) {
	a := Assessment{InScope: []Route{{"CLAUDE.md", "delete symlink", "govna compatibility link"}, {"govna/roles.md", "delete file", "byte-equal govna canon"}}, OutOfScope: []Route{{"custom.md", "keep", "target-only repo-owned file"}, {"plan.md", "keep", "repo-owned govna-adjacent content"}}, Review: []Route{{"README.md", "hybrid", "mixed canon-shape and consumer content"}, {"govna/metadata.txt", "ambiguity", "consumer-edited canon file"}}}
	control := Route{"govna/preserve.txt", "delete control state last", "preserve decisions applied before registry removal"}
	a.ControlState = &control
	got := buildAC("govna/ac7-govna-rm-v0.33.0.md", testProgramVersion, canon.Version, canon.Code, "Go", a)
	want, err := os.ReadFile("testdata/removal-golden.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("golden mismatch\n%s", got)
	}
	wrapped := emission.GuardedBody(emission.RemovalMarkerPrefix, testProgramVersion, canon.Version, []byte(got))
	if !emission.VerifyGuardedBody(wrapped, emission.RemovalMarkerPrefix) {
		t.Fatal("golden marker invalid")
	}
}

func TestRemovalInstructionBranches(t *testing.T) {
	withRouting := Assessment{
		InScope: []Route{{"CLAUDE.md", "delete symlink", "govna compatibility link"}},
		Review:  []Route{{"README.md", "hybrid", "mixed canon-shape and consumer content"}},
	}
	body := buildAC("govna/ac7-govna-rm-v0.33.0.md", testProgramVersion, canon.Version, canon.Code, "Go", withRouting)
	ordered := []string{
		"- Render the selected canon into `<scratch>` with `govna render --flavor code --stack Go <scratch>`.",
		"- Preserve every routing-pending path until its route is resolved.",
		"- Resolve every routing decision in chat.",
		"- Apply each in-scope route and each Director-resolved review route.",
	}
	position := -1
	for _, want := range ordered {
		next := strings.Index(body, want)
		if next <= position {
			t.Fatalf("removal instruction %q is missing or out of order: %s", want, body)
		}
		position = next
	}
	for _, want := range []string{
		"This removal AC was emitted by govna executable v9.8.7 with embedded canon v0.33.0.",
		"1. `README.md` is mixed canon-shape and consumer content.\n   - Compare `README.md` with `diff -ru <scratch>/README.md README.md`.\n   - Choose one route for `README.md`: canon-only deletion, full preservation, or full deletion.",
		"## Migration findings\n\n- None.",
		"**AT1** [Automated] [Pre-release gate] — Verify every resolved removal target under `## In Scope` is absent.",
		"**AT2** [Manual] [Pre-release gate] — Verify every routing-pending path matches its Director-resolved route.",
		"`PENDING` — removal emission; awaiting explicit Director Audit.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("removal body omits %q", want)
		}
	}
	for _, invalid := range []string{"before choosing", "Director confirms", "Removed files", "Apply only the Director-resolved", "awaiting Director review"} {
		if strings.Contains(body, invalid) {
			t.Errorf("removal body retains invalid text %q", invalid)
		}
	}

	withoutRouting := buildAC(
		"govna/ac8-govna-rm-v0.33.0.md",
		testProgramVersion,
		canon.Version,
		canon.Doc,
		"",
		Assessment{InScope: []Route{{"CLAUDE.md", "delete symlink", "govna compatibility link"}}},
	)
	if !strings.Contains(withoutRouting, "### Removal Instructions\n\n- Apply each in-scope route.\n\n### Routing Decisions\n\n`None` — no review items.") {
		t.Fatalf("no-routing instructions are incorrect: %s", withoutRouting)
	}
	for _, absent := range []string{"Render the selected canon", "Preserve every routing-pending", "Resolve every routing decision", "Choose one route", "Compare `"} {
		if strings.Contains(withoutRouting, absent) {
			t.Errorf("no-routing body unexpectedly contains %q", absent)
		}
	}
	if strings.Contains(withoutRouting, "AT3") {
		t.Errorf("no-control-state body unexpectedly contains AT3: %s", withoutRouting)
	}
}

func TestRemovalArgumentsAndPreconditions(t *testing.T) {
	for _, args := range [][]string{{"extra"}, {"--flavor"}, {"--flavor", "bad"}, {"--stack", ""}, {"--flavor", "doc", "--stack", "Go"}} {
		var out, err bytes.Buffer
		if code := Run(args, &out, &err, t.TempDir(), testProgramVersion); code != 2 {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, err.String())
		}
	}
	root := t.TempDir()
	var out, err bytes.Buffer
	if code := Run(nil, &out, &err, root, testProgramVersion); code != 1 || !strings.Contains(err.String(), "AGENTS.md") {
		t.Fatalf("code=%d stderr=%q", code, err.String())
	}
}

func TestRemovalFlavorOverride(t *testing.T) {
	root := fixture(t)
	var out, err bytes.Buffer
	if code := Run([]string{"--flavor", "doc", "--repo-name", "widget"}, &out, &err, root, testProgramVersion); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, err.String())
	}
	data, _ := os.ReadFile(filepath.Join(root, "govna", "ac1-govna-rm-v0.33.0.md"))
	if !strings.Contains(string(data), "govna render --flavor doc <scratch>") {
		t.Fatalf("stub=%s", data)
	}
}

func assertRoute(t *testing.T, routes []Route, path, kind string) {
	t.Helper()
	for _, r := range routes {
		if r.Path == path && r.Kind == kind {
			return
		}
	}
	t.Fatalf("missing %s %s in %#v", path, kind, routes)
}

func legacyGuardedBody(prefix, version string, body []byte) []byte {
	hash := sha256.Sum256(body)
	return []byte(fmt.Sprintf("%s%s sha256:%x -->\n%s", prefix, version, hash, body))
}
