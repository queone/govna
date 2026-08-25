package audit

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/emission"
)

const testProgramVersion = "9.8.7"

func inferredBuildOutcome() validationOutcome {
	return validationOutcome{kind: validationInferred, evidence: "`./build.sh` inferred from exact AGENTS.md declarations"}
}

func notApplicableOutcome(evidence string) validationOutcome {
	return validationOutcome{kind: validationNotApplicable, evidence: evidence}
}

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
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 0 || stderr.Len() != 0 {
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
	if code := Run([]string{"--json"}, &stdout, &stderr, root, testProgramVersion); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	var report Report
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Header.CanonSHA != "v0.35.0" || report.Emitted != nil || strings.Contains(stdout.String(), "no AC emitted") {
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
	if code := Run([]string{"--repo-name", "widget"}, &stdout, &stderr, root, testProgramVersion); code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "wrote govna/ac1-audit-v0.35.0.md") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	stub := filepath.Join(root, "govna", "ac1-audit-v0.35.0.md")
	body, err := os.ReadFile(stub)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "## Summary") || !strings.Contains(string(body), "`README.md` — `ambiguity`") {
		t.Fatalf("bad stub: %s", body)
	}
	markerPrefix := "<!-- audit: emitted-by govna executable v9.8.7 with embedded canon v0.35.0 sha256:"
	if !strings.HasPrefix(string(body), markerPrefix) {
		t.Fatalf("bad marker: %s", body)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 0 {
		t.Fatalf("idempotent code=%d stderr=%q", code, stderr.String())
	}
	rerunBody, err := os.ReadFile(stub)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, rerunBody) {
		t.Fatal("unchanged audit rerun rewrote body")
	}
	_, rawBody, ok := strings.Cut(string(body), "\n")
	if !ok {
		t.Fatal("guarded body has no marker line")
	}
	legacyBody := strings.Replace(
		rawBody,
		"Verify the final adoption step installed `govna/canon-baseline.txt` from the same scratch render.",
		"Install and verify `govna/canon-baseline.txt` from the same scratch render as the final adoption step.",
		1,
	)
	if legacyBody == rawBody {
		t.Fatal("failed to construct legacy audit body")
	}
	legacy := legacyGuardedBody(emission.AuditMarkerPrefix, "v"+canon.Version, []byte(legacyBody))
	if err := os.WriteFile(stub, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 0 {
		t.Fatalf("legacy upgrade code=%d stderr=%q", code, stderr.String())
	}
	upgraded, err := os.ReadFile(stub)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(upgraded), markerPrefix) || bytes.Equal(upgraded, legacy) || strings.Contains(string(upgraded), "Install and verify") {
		t.Fatalf("legacy marker not upgraded: %s", upgraded)
	}
	matches, err := filepath.Glob(filepath.Join(root, "govna", "ac*-audit-v0.35.0.md"))
	if err != nil || len(matches) != 1 || matches[0] != stub {
		t.Fatalf("same-canon upgrade changed stub identity: matches=%v err=%v", matches, err)
	}
	if err := os.WriteFile(stub, append(upgraded, []byte("edited\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := Run(nil, &stdout, &stderr, root, testProgramVersion); code != 1 || !strings.Contains(stderr.String(), "has been edited") {
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
			body := buildAC(report, "govna/ac1-audit-v0.35.0.md", inferredBuildOutcome())
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
		Header: Header{Invocation: "govna audit --repo-name widget", CanonSHA: "v0.35.0", Target: "<TARGET>", Flavor: "code", FlavorSource: "explicit", RepoName: "widget", CanonVersion: "v0.28.0", CodeStack: "Go"},
		Files: []FileResult{
			{Path: "README.md", Classification: "clear-sync", PriorCommits: []string{"abc123"}, CanonReference: "govna @ v0.35.0: README.md", CompareCommand: "compare embedded canon with target: README.md"},
			{Path: "govna/canon-baseline.txt", Classification: "clear-sync", CanonReference: "generated baseline manifest", CompareCommand: "compare generated baseline with target govna/canon-baseline.txt"},
			{Path: "local.md", Classification: "target-has-no-canon", CanonReference: "name-referenced from divergent governed file", CompareCommand: "inspect target-only file local.md"},
			{Path: "plan.md", Classification: "expected-divergence"},
		},
		Emitted: &Emitted{ACStub: "govna/ac7-audit-v0.35.0.md"},
	}
	markdown, err := os.ReadFile("testdata/actionable-golden.md")
	if err != nil {
		t.Fatal(err)
	}
	if got := buildAC(report, report.Emitted.ACStub, inferredBuildOutcome()); got != string(markdown) {
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

func TestExistingAndMissingBaselineClassification(t *testing.T) {
	t.Run("existing stale baseline is sync", func(t *testing.T) {
		root := fixture(t)
		path := filepath.Join(root, filepath.FromSlash(baselinePath))
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.Replace(data, []byte("canon_version = v0.35.0\n"), []byte("canon_version = v0.32.0\n"), 1)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil || clean {
			t.Fatalf("clean=%v err=%v", clean, err)
		}
		baseline := reportFile(t, report, baselinePath)
		if baseline.Classification != "clear-sync" {
			t.Fatalf("baseline=%+v", baseline)
		}
		disposition := validationDisposition(root, report)
		if disposition.kind != validationInferred || !strings.Contains(disposition.evidence, "`./build.sh`") {
			t.Fatalf("validation disposition=%+v", disposition)
		}
		body := buildAC(report, "govna/ac1-audit-v0.35.0.md", disposition)
		if !strings.Contains(body, "### Direct sync\n\n- `govna/canon-baseline.txt` — `clear-sync`.") || !strings.Contains(body, "## Migration findings\n\n- None.") {
			t.Fatalf("body=%s", body)
		}
	})
	t.Run("missing baseline is migration", func(t *testing.T) {
		root := fixture(t)
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(baselinePath))); err != nil {
			t.Fatal(err)
		}
		report, clean, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil || clean {
			t.Fatalf("clean=%v err=%v", clean, err)
		}
		baseline := reportFile(t, report, baselinePath)
		if baseline.Classification != "migration-required" {
			t.Fatalf("baseline=%+v", baseline)
		}
		body := buildAC(report, "govna/ac1-audit-v0.35.0.md", inferredBuildOutcome())
		want := "- Create `govna/canon-baseline.txt` from the final scratch render only after all other work and validation pass."
		if !strings.Contains(body, want) {
			t.Fatalf("body=%s", body)
		}
	})
}

func TestValidationManifestReachability(t *testing.T) {
	for _, tc := range []struct {
		name, stack, manifest string
	}{
		{"go", "Go", "go.mod"},
		{"rust", "Rust", "Cargo.toml"},
		{"swift", "Swift", "Package.swift"},
		{"terraform lock", "Terraform", ".terraform.lock.hcl"},
		{"terraform root file", "Terraform", "main.tf"},
		{"node", "Node", "package.json"},
		{"python", "Python", "pyproject.toml"},
		{"java maven", "Java", "pom.xml"},
		{"java gradle", "Java", "build.gradle"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, tc.manifest), []byte("fixture\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if !stackManifestReachable(root, tc.stack) {
				t.Fatalf("%s manifest %s is not reachable", tc.stack, tc.manifest)
			}
		})
	}

	for _, tc := range []struct {
		name, stack, manifest string
		directory             bool
	}{
		{"missing", "Go", "", false},
		{"directory", "Go", "go.mod", true},
		{"other stack only", "Go", "Cargo.toml", false},
		{"terraform directory", "Terraform", "main.tf", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.manifest != "" {
				path := filepath.Join(root, tc.manifest)
				var err error
				if tc.directory {
					err = os.Mkdir(path, 0o755)
				} else {
					err = os.WriteFile(path, []byte("fixture\n"), 0o644)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if stackManifestReachable(root, tc.stack) {
				t.Fatalf("%s accepted invalid manifest evidence %q", tc.stack, tc.manifest)
			}
		})
	}
}

func TestValidationStackSelectionSources(t *testing.T) {
	for _, tc := range []struct {
		name       string
		configure  func(*testing.T, string) Config
		wantHeader string
	}{
		{
			name: "explicit",
			configure: func(t *testing.T, root string) Config {
				replaceFileText(t, filepath.Join(root, "govna", "metadata.txt"), "code_stack = Go\n", "code_stack = Rust\n")
				return Config{Flavor: "code", Stack: "Go", RepoName: "widget", DiffLines: 200, invocation: "govna audit --flavor code --stack Go"}
			},
			wantHeader: "Rust",
		},
		{
			name: "metadata",
			configure: func(_ *testing.T, _ string) Config {
				return Config{DiffLines: 200, invocation: "govna audit"}
			},
			wantHeader: "Go",
		},
		{
			name: "manifest",
			configure: func(t *testing.T, root string) Config {
				if err := os.Remove(filepath.Join(root, "govna", "metadata.txt")); err != nil {
					t.Fatal(err)
				}
				return Config{Flavor: "code", RepoName: "widget", DiffLines: 200, invocation: "govna audit --flavor code"}
			},
			wantHeader: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			makeBaselineStale(t, root)
			cfg := tc.configure(t, root)
			report, clean, err := inspect(cfg, root)
			if err != nil || clean {
				t.Fatalf("clean=%v err=%v", clean, err)
			}
			if report.Header.CodeStack != tc.wantHeader {
				t.Fatalf("public code_stack=%q want=%q", report.Header.CodeStack, tc.wantHeader)
			}
			outcome := validationDisposition(root, report)
			if outcome.kind != validationInferred || outcome.evidence != inferredBuildOutcome().evidence {
				t.Fatalf("validation outcome=%+v", outcome)
			}
			encoded, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(encoded, []byte("selectedStack")) || bytes.Contains(encoded, []byte("selected_stack")) {
				t.Fatalf("private selected stack leaked into JSON: %s", encoded)
			}
		})
	}
}

func TestValidationEvidenceOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*testing.T, string)
	}{
		{
			name: "missing selected manifest",
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "directory selected manifest",
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(root, "go.mod"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "other stack only",
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname = \"widget\"\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			makeBaselineStale(t, root)
			tc.mutate(t, root)
			report, clean, err := inspect(Config{Flavor: "code", Stack: "Go", RepoName: "widget", DiffLines: 200, invocation: "govna audit --flavor code --stack Go"}, root)
			if err != nil || clean {
				t.Fatalf("clean=%v err=%v", clean, err)
			}
			if outcome := validationDisposition(root, report); outcome.kind != validationUnresolved {
				t.Fatalf("validation outcome=%+v", outcome)
			}
		})
	}

	t.Run("exact declarations", func(t *testing.T) {
		root := fixture(t)
		makeBaselineStale(t, root)
		report, _, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil {
			t.Fatal(err)
		}
		replaceFileText(t, filepath.Join(root, "AGENTS.md"), "- Run `./build.sh` as the first validation command", "- Run `make` as the first validation command")
		if outcome := validationDisposition(root, report); outcome.kind != validationUnresolved {
			t.Fatalf("validation outcome=%+v", outcome)
		}
	})

	t.Run("regular build file", func(t *testing.T) {
		root := fixture(t)
		makeBaselineStale(t, root)
		report, _, err := inspect(Config{DiffLines: 200, invocation: "govna audit"}, root)
		if err != nil {
			t.Fatal(err)
		}
		build := filepath.Join(root, "build.sh")
		if err := os.Remove(build); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(build, 0o755); err != nil {
			t.Fatal(err)
		}
		if outcome := validationDisposition(root, report); outcome.kind != validationUnresolved {
			t.Fatalf("validation outcome=%+v", outcome)
		}
	})

	t.Run("no baseline update", func(t *testing.T) {
		outcome := validationDisposition(t.TempDir(), Report{Header: Header{Flavor: "code"}, selectedStack: "Go"})
		if outcome.kind != validationNotApplicable || outcome.evidence != "`Not applicable` because no baseline migration is present" {
			t.Fatalf("validation outcome=%+v", outcome)
		}
	})

	t.Run("doc evidence", func(t *testing.T) {
		root := renderedFixture(t, canon.Doc, "")
		report := Report{Header: Header{Flavor: "doc"}, Files: []FileResult{{Path: baselinePath, Classification: "clear-sync"}}}
		outcome := validationDisposition(root, report)
		if outcome.kind != validationNotApplicable || outcome.evidence != "`Not applicable` inferred from exact DOC governance evidence" {
			t.Fatalf("validation outcome=%+v", outcome)
		}
		replaceFileText(t, filepath.Join(root, "govna", "release.md"), "define no automated content-validation command.", "define repository content validation locally.")
		if outcome := validationDisposition(root, report); outcome.kind != validationUnresolved {
			t.Fatalf("validation outcome=%+v", outcome)
		}
	})
}

func TestUnresolvedValidationGolden(t *testing.T) {
	root := fixture(t)
	makeBaselineStale(t, root)
	if err := os.Remove(filepath.Join(root, "go.mod")); err != nil {
		t.Fatal(err)
	}
	inspected, _, err := inspect(Config{Flavor: "code", Stack: "Go", RepoName: "widget", DiffLines: 200, invocation: "govna audit --flavor code --stack Go"}, root)
	if err != nil {
		t.Fatal(err)
	}
	outcome := validationDisposition(root, inspected)
	if outcome.kind != validationUnresolved {
		t.Fatalf("validation outcome=%+v", outcome)
	}
	report := Report{
		Header: Header{Flavor: "code", RepoName: "widget"},
		Files: []FileResult{
			{Path: "AGENTS.md", Classification: "clear-sync", Boundary: "## Project Rules", protectedHash: "abc123"},
			{Path: baselinePath, Classification: "clear-sync"},
			{Path: "local.md", Classification: "ambiguity"},
			{Path: "plan.md", Classification: "preserve"},
		},
	}
	body := buildAC(report, "govna/ac7-audit-v0.35.0.md", outcome)
	want, err := os.ReadFile("testdata/unresolved-validation-golden.md")
	if err != nil {
		t.Fatal(err)
	}
	if body != string(want) {
		t.Fatalf("unresolved validation golden mismatch\n%s", body)
	}
	ordered := []string{
		"1. **`local.md`**: Which outcome applies: sync, preserve, migrate, or delete?",
		"2. **Validation disposition**: Which outcome applies after selected work: run a repository validation command, or record `Not applicable` with repository evidence?",
		"Preserve the protected region in `AGENTS.md` from `## Project Rules` through EOF with SHA-256 `abc123` for any sync outcome.",
		"Resolve the validation disposition in chat.",
		"Satisfy the resolved validation disposition after selected work and before baseline installation.",
		"Verify the final adoption step installed `govna/canon-baseline.txt` from the same scratch render.",
	}
	position := -1
	for _, text := range ordered {
		next := strings.Index(body, text)
		if next <= position {
			t.Fatalf("unresolved instruction %q is missing or out of order: %s", text, body)
		}
		position = next
	}
	if strings.Contains(body, "validation disposition `./build.sh`") {
		t.Fatalf("unresolved validation was hard-coded: %s", body)
	}
}

func TestInferredValidationOmitsManualRouting(t *testing.T) {
	report := Report{Header: Header{Flavor: "code", RepoName: "widget"}, Files: []FileResult{{Path: baselinePath, Classification: "clear-sync"}}}
	code := buildAC(report, "govna/ac1-audit-v0.35.0.md", inferredBuildOutcome())
	for _, absent := range []string{"**Validation disposition**:", "Resolve the validation disposition in chat."} {
		if strings.Contains(code, absent) {
			t.Fatalf("inferred CODE emission contains %q: %s", absent, code)
		}
	}
	report.Header.Flavor = "doc"
	doc := buildAC(report, "govna/ac1-audit-v0.35.0.md", notApplicableOutcome("`Not applicable` inferred from exact DOC governance evidence"))
	for _, absent := range []string{"**Validation disposition**:", "Resolve the validation disposition in chat."} {
		if strings.Contains(doc, absent) {
			t.Fatalf("inferred DOC emission contains %q: %s", absent, doc)
		}
	}
}

func TestEmittedSummaryAndFlavorInstructions(t *testing.T) {
	report := Report{Header: Header{Flavor: "code", RepoName: "widget"}, Files: []FileResult{{Path: "README.md", Classification: "clear-sync"}, {Path: "local.md", Classification: "ambiguity"}, {Path: "plan.md", Classification: "preserve"}}}
	body := buildAC(report, "govna/ac1-audit-v0.35.0.md", inferredBuildOutcome())
	wantSummary := "## Summary\n\nThis adoption covers 1 sync path, 0 migration paths, 1 review path, and 1 out-of-scope path.\n\nThis audit adoption synchronizes"
	if !strings.Contains(body, wantSummary) {
		t.Fatalf("summary=%s", body)
	}
	reachability := "Verify every direct-sync and canon-backed migration path exists in the selected CODE stack scratch render as a precondition."
	if !strings.Contains(body, reachability) {
		t.Fatalf("CODE instruction missing: %s", body)
	}
	report.Header.Flavor = "doc"
	if doc := buildAC(report, "govna/ac1-audit-v0.35.0.md", notApplicableOutcome("`Not applicable`")); strings.Contains(doc, reachability) {
		t.Fatalf("DOC contains CODE instruction: %s", doc)
	}
}

func TestGeneratedInstructionBranches(t *testing.T) {
	report := Report{
		Header: Header{Flavor: "code", RepoName: "widget"},
		Files: []FileResult{
			{Path: "AGENTS.md", Classification: "clear-sync", Boundary: "## Project Rules", protectedHash: "abc123", LegacyPreserveMarkers: []string{"legacy"}},
			{Path: baselinePath, Classification: "migration-required"},
			{Path: "govna/metadata.txt", Classification: "migration-required"},
			{Path: "govna/other.md", Classification: "migration-required"},
			{Path: "local.md", Classification: "ambiguity"},
		},
	}
	body := buildAC(report, "govna/ac1-audit-v0.35.0.md", inferredBuildOutcome())
	for _, want := range []string{
		"- Verify every direct-sync and canon-backed migration path exists in the selected CODE stack scratch render as a precondition.",
		"- Apply every resolved outcome within the authorized content boundaries.",
		"- Remove the legacy preserve phrase for `AGENTS.md` only after the required sync and registry state are verified.",
		"- Create `govna/canon-baseline.txt` from the final scratch render only after all other work and validation pass.",
		"- Create `govna/metadata.txt` from the selected scratch render before `govna/canon-baseline.txt` installation.",
		"- Create `govna/other.md` from the selected scratch render.",
		"Preserve the protected region in `AGENTS.md` from `## Project Rules` through EOF with SHA-256 `abc123` for any sync outcome.",
		"Verify the final adoption step installed `govna/canon-baseline.txt` from the same scratch render.",
		"`PENDING` — audit emission; awaiting explicit Director Audit.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("audit body omits %q", want)
		}
	}
	legacyIndex := strings.Index(body, "Remove the legacy preserve phrase")
	routingIndex := strings.Index(body, "### Routing Decisions")
	if legacyIndex < 0 || routingIndex < 0 || legacyIndex > routingIndex {
		t.Errorf("legacy instruction is not under Adoption Instructions before routing: %s", body)
	}
	for _, invalid := range []string{"When `AGENTS.md` is synced", "Install and verify", "Resolve the legacy preserve phrase", "before applying changes"} {
		if strings.Contains(body, invalid) {
			t.Errorf("audit body retains invalid text %q", invalid)
		}
	}
	report.Header.Flavor = "doc"
	if doc := buildAC(report, "govna/ac1-audit-v0.35.0.md", notApplicableOutcome("`Not applicable`")); strings.Contains(doc, "selected CODE stack") {
		t.Errorf("DOC body contains CODE reachability instruction: %s", doc)
	}
}

func reportFile(t *testing.T, report Report, path string) FileResult {
	t.Helper()
	for _, file := range report.Files {
		if file.Path == path {
			return file
		}
	}
	t.Fatalf("report omits %s", path)
	return FileResult{}
}

func makeBaselineStale(t *testing.T, root string) {
	t.Helper()
	replaceFileText(t, filepath.Join(root, filepath.FromSlash(baselinePath)), "canon_version = v"+canon.Version+"\n", "canon_version = v0.1.0\n")
}

func replaceFileText(t *testing.T, path, old, replacement string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(data), old, replacement, 1)
	if updated == string(data) {
		t.Fatalf("%s omits replacement source %q", path, old)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func renderedFixture(t *testing.T, flavor canon.Flavor, stack string) string {
	t.Helper()
	root := t.TempDir()
	files, err := canon.Render(canon.Config{Flavor: flavor, RepoName: "widget", Stack: stack})
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
	return root
}

func legacyGuardedBody(prefix, version string, body []byte) []byte {
	hash := sha256.Sum256(body)
	return []byte(fmt.Sprintf("%s%s sha256:%x -->\n%s", prefix, version, hash, body))
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
