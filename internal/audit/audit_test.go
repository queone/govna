package audit

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queone/govna/internal/canon"
)

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
	git(t, root, "init", "-q")
	git(t, root, "config", "user.email", "fixture@example.invalid")
	git(t, root, "config", "user.name", "Fixture")
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "govna apply")
	return root
}

func git(t *testing.T, root string, args ...string) {
	t.Helper()
	args = append([]string{"-C", root}, args...)
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func TestAuditCleanAndJSON(t *testing.T) {
	root := fixture(t)
	var stdout, stderr bytes.Buffer
	if code := Run(nil, &stdout, &stderr, root); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	want, err := os.ReadFile("testdata/clean-golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	if stdout.String() != string(want) {
		t.Fatalf("clean output=%q want=%q", stdout.String(), want)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run([]string{"--json"}, &stdout, &stderr, root); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	var report Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Header.CanonSHA != "v0.30.0" || report.Emitted != nil || strings.Contains(stdout.String(), "no AC emitted") {
		t.Fatalf("bad JSON: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "canon_reference") || strings.Contains(stdout.String(), "prior_commits") || !strings.Contains(stdout.String(), `"canon_ref"`) || !strings.Contains(stdout.String(), `"compare_command"`) {
		t.Fatalf("incorrect JSON schema: %s", stdout.String())
	}
}

func TestAuditActionableEmissionAndGuard(t *testing.T) {
	root := fixture(t)
	readme := filepath.Join(root, "README.md")
	if err := os.WriteFile(readme, []byte("consumer edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--repo-name", "widget"}, &stdout, &stderr, root); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "wrote govna/ac1-audit-v0.30.0.md") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	stub := filepath.Join(root, "govna", "ac1-audit-v0.30.0.md")
	body, err := os.ReadFile(stub)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "## Summary") || !strings.Contains(string(body), "`README.md` — `ambiguity`") {
		t.Fatalf("bad stub: %s", body)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(nil, &stdout, &stderr, root); code != 0 {
		t.Fatalf("idempotent code=%d stderr=%q", code, stderr.String())
	}
	if err := os.WriteFile(stub, append(body, []byte("edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(nil, &stdout, &stderr, root); code != 1 || !strings.Contains(stderr.String(), "has been edited") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestAuditStateAndClassification(t *testing.T) {
	root := fixture(t)
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(preservePath)), []byte("govna-preserve-v1\nREADME.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	preserve, err := ParsePreserve(root)
	if err != nil || !preserve["README.md"] {
		t.Fatalf("preserve=%v err=%v", preserve, err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(preservePath)), []byte("govna-preserve-v1\n../bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePreserve(root); err == nil {
		t.Fatal("malformed preserve accepted")
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(baselinePath)))
	if err != nil {
		t.Fatal(err)
	}
	base, err := parseBaseline(data, canon.Code)
	if err != nil || len(base.Entries) == 0 {
		t.Fatalf("baseline=%v err=%v", base, err)
	}
	if _, err := parseBaseline([]byte("bad\n"), canon.Code); err == nil {
		t.Fatal("malformed baseline accepted")
	}
}

func TestAuditFormatForcingAndCoherence(t *testing.T) {
	root := fixture(t)
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(agents, []byte("consumer edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(preservePath)), []byte("govna-preserve-v1\nAGENTS.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, clean, err := inspect(Config{DiffLines: 200}, root)
	if err != nil || clean {
		t.Fatalf("clean=%v err=%v", clean, err)
	}
	for _, file := range report.Files {
		if file.Path == "AGENTS.md" && (file.Classification != "preserve" || !file.forceSync) {
			t.Fatalf("format classification=%s", file.Classification)
		}
	}
	old := coherenceRules
	coherenceRules = []coherenceRule{{Path: "AGENTS.md", Contains: "impossible canon token"}}
	t.Cleanup(func() { coherenceRules = old })
	if _, _, err := inspect(Config{DiffLines: 200}, root); err == nil || !strings.Contains(err.Error(), "canon-coherence") {
		t.Fatalf("coherence err=%v", err)
	}
}

func TestMissingFormatFileKeepsClassificationAndForcesSync(t *testing.T) {
	root := fixture(t)
	if err := os.Remove(filepath.Join(root, "govna", "ac-template.md")); err != nil {
		t.Fatal(err)
	}
	report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
	if err != nil || clean {
		t.Fatalf("clean=%v err=%v", clean, err)
	}
	for _, file := range report.Files {
		if file.Path == "govna/ac-template.md" {
			if file.Classification != "missing-in-target" || !file.forceSync {
				t.Fatalf("file=%+v", file)
			}
			body := buildAC(report, "govna/ac1-audit-v0.30.0.md", "`./build.sh`")
			if !strings.Contains(body, "### Direct sync\n\n- `govna/ac-template.md` — `missing-in-target`.") {
				t.Fatalf("routing=%s", body)
			}
			return
		}
	}
	t.Fatal("missing format file absent from report")
}

func TestAuditArgumentsAndPreconditions(t *testing.T) {
	for _, args := range [][]string{{"extra"}, {"--diff-lines", "0"}, {"--flavor", "bad"}, {"--stack", ""}} {
		var out, err bytes.Buffer
		if code := Run(args, &out, &err, t.TempDir()); code != 2 {
			t.Fatalf("args=%v code=%d stderr=%q", args, code, err.String())
		}
	}
	root := t.TempDir()
	var out, err bytes.Buffer
	if code := Run(nil, &out, &err, root); code != 1 || !strings.Contains(err.String(), "AGENTS.md") {
		t.Fatalf("code=%d stderr=%q", code, err.String())
	}
}

func TestAuditDiffTruncationAndCrossFlavor(t *testing.T) {
	diff := diffText([]byte("a\nb\nc\n"), []byte("x\ny\nz\n"), "x", 3)
	if !strings.Contains(diff, "more lines truncated") {
		t.Fatalf("diff=%q", diff)
	}
	root := fixture(t)
	if err := os.WriteFile(filepath.Join(root, "govna", "release.md"), []byte("doc-only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, _ := canon.Render(canon.Config{Flavor: canon.Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"})
	current := map[string][]byte{}
	for _, f := range files {
		current[f.Path] = f.Content
	}
	got := targetOnly(root, current, nil, canon.Code, "widget")
	if got["govna/release.md"] != "present in other flavor canon" {
		t.Fatalf("target-only=%v", got)
	}
}

func TestAuditGoldens(t *testing.T) {
	report := Report{
		Header: Header{Invocation: "govna audit --repo-name widget", CanonSHA: "v0.30.0", Target: "<TARGET>", Flavor: "code", FlavorSource: "explicit", RepoName: "widget", CanonVersion: "v0.28.0", CodeStack: "Go"},
		Files: []FileResult{
			{Path: "README.md", Classification: "clear-sync", PriorCommits: []string{"abc123"}, CanonReference: "govna @ v0.30.0: README.md", CompareCommand: "compare embedded canon with target: README.md"},
			{Path: "govna/canon-baseline.txt", Classification: "migration-required", CanonReference: "generated baseline manifest", CompareCommand: "compare generated baseline with target govna/canon-baseline.txt"},
			{Path: "local.md", Classification: "target-has-no-canon", CanonReference: "name-referenced from divergent governed file", CompareCommand: "inspect target-only file local.md"},
			{Path: "plan.md", Classification: "expected-divergence"},
		},
		Emitted: &Emitted{ACStub: "govna/ac7-audit-v0.30.0.md"},
	}
	markdown, err := os.ReadFile("testdata/actionable-golden.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := buildAC(report, report.Emitted.ACStub, "`./build.sh` inferred from exact AGENTS.md declarations"); got != string(markdown) {
		t.Fatalf("markdown golden mismatch\n%s", got)
	}
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(report); err != nil {
		t.Fatal(err)
	}
	jsonGolden, err := os.ReadFile("testdata/actionable-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	if encoded.String() != string(jsonGolden) {
		t.Fatalf("JSON golden mismatch\n%s", encoded.String())
	}
}

func TestAuditInvocationPreservesOptionOrder(t *testing.T) {
	cfg, err := parse([]string{"--repo-name", "widget", "--json", "--diff-lines", "12", "--flavor", "code"})
	if err != nil {
		t.Fatal(err)
	}
	want := "govna audit --repo-name widget --json --diff-lines 12 --flavor code"
	if cfg.invocation != want {
		t.Fatalf("invocation=%q want=%q", cfg.invocation, want)
	}
}
