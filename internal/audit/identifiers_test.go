package audit

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/repository"
)

// identifierFixtureFiles exercises every rule of the Markdown identifier scan.
// Identifier-shaped test-name fragments are joined so the test-naming lint
// does not read them as test labels.
func identifierFixtureFiles() map[string]string {
	return map[string]string{
		"README.md":                "# Widget\n\nUse `Package AC70+AC71` to release.\nUse `Package AC<N>+AC<M>` instead.\nACs, FAT32, " + "_" + "AT1, AC1x, AC1AT2, and \xc3\xa9AT5.\n",
		"history.md":               "# History\r\n\r\nShipped in AT3.\r\n",
		"docs/guide.md":            "# Guide\n\nSupports IE11.\n<!-- Historical: AC12 shipped this --> then AT4 <!-- note AC9 -->\n<!-- Historical: AC14 without a close\n",
		"govna/ac-template.md":     "**AT1** [Automated]\n\nSee IE2.\n",
		"govna/ac81-example.md":    "# AC81 Example\n",
		"plan.md":                  "# Plan\n\n- IE5: Explore a scan; see AC3.\n* IE6: Another.\n  - IE7: Nested.\n",
		"CHANGELOG.md":             "# Changelog\n\n| Version | Summary |\n|---------|---------|\n| Unreleased | |\n| 1.0.0 | AC12 shipped |\n",
		"internal/x/testdata/g.md": "AC1 fixture\n",
		"tests/fixtures/f.md":      "AC2 fixture\n",
		"testdata/top.md":          "AC3 fixture\n",
		"empty.md":                 "",
	}
}

var identifierParentFindings = []string{
	"README.md:3:AC70", "README.md:3:AC71", "README.md:5:AT5",
	"history.md:3:AT3",
	"docs/guide.md:3:IE11", "docs/guide.md:4:AT4", "docs/guide.md:4:AC9", "docs/guide.md:5:AC14",
	"govna/ac-template.md:3:IE2",
	"plan.md:3:AC3",
}

var identifierChildOnlyFindings = []string{
	"CHANGELOG.md:6:AC12",
	"govna/ac81-example.md:1:AC81",
	"govna/ac-template.md:1:AT1",
	"plan.md:3:IE5", "plan.md:4:IE6", "plan.md:5:IE7",
	"docs/guide.md:4:AC12",
}

// identifierDiscoveryFindings are the findings that only Git-listed discovery
// produces: a followed link and directories that find prunes.
var identifierDiscoveryFindings = []string{
	"linked.md:3:IE11", "linked.md:4:AT4", "linked.md:4:AC9", "linked.md:5:AC14",
	"outside.md:1:AT6",
	"vendor/lib.md:1:AT10",
	"node_modules/pkg/readme.md:1:AC20",
}

// writeIdentifierDiscovery adds the link and pruned-directory cases: a link to
// a Markdown file, a link outside dir, a dangling link, a link to a directory,
// and Markdown under vendor and node_modules.
func writeIdentifierDiscovery(t *testing.T, dir, outside string) {
	t.Helper()
	writeIdentifierFixture(t, dir, map[string]string{"vendor/lib.md": "AT10 vendored\n", "node_modules/pkg/readme.md": "AC20 package\n"})
	for link, target := range map[string]string{"linked.md": "docs/guide.md", "outside.md": outside, "dangling.md": "missing.md", "dirlink.md": "docs"} {
		if err := os.Symlink(target, filepath.Join(dir, link)); err != nil {
			t.Fatal(err)
		}
	}
}

func writeIdentifierFixture(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for path, body := range files {
		writeAuditFile(t, filepath.Join(dir, filepath.FromSlash(path)), []byte(body))
	}
}

var shellFindingRE = regexp.MustCompile(`^(.+):([0-9]+): bare identifier ([A-Z]+[0-9]+);`)

// shellIdentifierFindings runs the rendered Go build script's scan in dir.
func shellIdentifierFindings(t *testing.T, dir string) []string {
	t.Helper()
	files, err := canon.Render(canon.Config{Flavor: canon.Code, RepoName: "widget", Stack: "Go", ModulePath: "example.com/widget"})
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(t.TempDir(), "build.sh")
	for _, file := range files {
		if file.Path == "build.sh" {
			writeAuditFile(t, script, file.Content)
		}
	}
	cmd := exec.Command("/bin/bash", "-c", "source \"$1\"\n_bare_id_scan\n", "_", script)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shell scan: %v\n%s", err, out)
	}
	var findings []string
	for line := range strings.SplitSeq(string(out), "\n") {
		if m := shellFindingRE.FindStringSubmatch(line); m != nil {
			findings = append(findings, m[1]+":"+m[2]+":"+m[3])
		}
	}
	sort.Strings(findings)
	return findings
}

// goIdentifierFindings runs the Go scan over dir with no adoption results.
func goIdentifierFindings(t *testing.T, dir string, child bool) []string {
	t.Helper()
	access, err := repository.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer access.Close()
	found, err := scanKeptMarkdown(access, "", child, map[string]FileResult{})
	if err != nil {
		t.Fatal(err)
	}
	var findings []string
	for _, file := range found {
		for _, id := range file.Identifiers {
			findings = append(findings, file.Path+":"+id)
		}
	}
	sort.Strings(findings)
	return findings
}

func sortedCopy(values ...[]string) []string {
	var out []string
	for _, v := range values {
		out = append(out, v...)
	}
	sort.Strings(out)
	return out
}

func TestIdentifierScanAgreesWithShellScan(t *testing.T) {
	childPointer := ""
	files, err := canon.RenderChild("Go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Path == "AGENTS.md" {
			childPointer = string(file.Content)
		}
	}
	if !strings.Contains(childPointer, "\nThis repository is a Govna child repository.") {
		t.Fatalf("child pointer lacks the child line:\n%s", childPointer)
	}

	outside := filepath.Join(t.TempDir(), "outside.md")
	writeAuditFile(t, outside, []byte("AT6 outside\n"))

	repo := t.TempDir()
	writeIdentifierFixture(t, repo, identifierFixtureFiles())
	writeIdentifierDiscovery(t, repo, outside)
	writeIdentifierFixture(t, repo, map[string]string{".gitignore": "ignored.md\n", "ignored.md": "AT8 ignored\n"})
	git(t, repo, "init", "-q")
	git(t, repo, "add", "-A")
	writeAuditFile(t, filepath.Join(repo, "notes.md"), []byte("AT9 note"))

	plain := t.TempDir()
	writeIdentifierFixture(t, plain, identifierFixtureFiles())
	writeIdentifierDiscovery(t, plain, outside)

	for _, tc := range []struct {
		name  string
		dir   string
		child bool
		want  []string
	}{
		{name: "git repository", dir: repo, want: sortedCopy(identifierParentFindings, identifierDiscoveryFindings, []string{"notes.md:1:AT9"})},
		{name: "directory outside Git", dir: plain, want: sortedCopy(identifierParentFindings)},
		{name: "child repository", dir: repo, child: true, want: sortedCopy(identifierParentFindings, identifierChildOnlyFindings, identifierDiscoveryFindings, []string{"notes.md:1:AT9", "linked.md:4:AC12"})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.child {
				writeAuditFile(t, filepath.Join(tc.dir, "AGENTS.md"), []byte(childPointer))
				defer os.Remove(filepath.Join(tc.dir, "AGENTS.md"))
			}
			shell := shellIdentifierFindings(t, tc.dir)
			got := goIdentifierFindings(t, tc.dir, tc.child)
			if strings.Join(got, "\n") != strings.Join(shell, "\n") {
				t.Fatalf("Go scan:\n%s\nshell scan:\n%s", strings.Join(got, "\n"), strings.Join(shell, "\n"))
			}
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Fatalf("findings:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
		})
	}
}

func TestKeptTextFollowsAdoption(t *testing.T) {
	practices := []byte("canon AC8\n\n## Project Practices\n\n- Ship AT7 notes.\n")
	rules := []byte("canon AC6\n## Project Rules\n- Keep the AC5 rule.\n")
	for _, tc := range []struct {
		name string
		path string
		data []byte
		file *FileResult
		want []string
	}{
		{name: "file outside canon keeps its whole text", path: "notes.md", data: []byte("AC1\nAT2\n"), want: []string{"1:AC1", "2:AT2"}},
		{name: "updated boundary file keeps its local section", path: "govna/build-release.md", data: practices, file: &FileResult{Classification: "clear-sync"}, want: []string{"5:AT7"}},
		{name: "always-synced AGENTS.md keeps its project rules", path: "AGENTS.md", data: rules, file: &FileResult{Classification: "ambiguity", forceSync: true}, want: []string{"3:AC5"}},
		{name: "always-synced AGENTS.md without its boundary keeps nothing", path: "AGENTS.md", data: []byte("AC6 only\n"), file: &FileResult{Classification: "ambiguity", forceSync: true}},
		{name: "updated whole file keeps nothing", path: "README.md", data: []byte("AC70\n"), file: &FileResult{Classification: "clear-sync"}},
		{name: "retired link keeps nothing", path: "CLAUDE.md", data: []byte("AC4\n"), file: &FileResult{Classification: "retired-link"}},
		{name: "Director choice keeps its whole text", path: "govna/roles.md", data: []byte("x\nAC9\n"), file: &FileResult{Classification: "ambiguity"}, want: []string{"2:AC9"}},
		{name: "expected local difference keeps its whole text", path: "arch.md", data: []byte("AC70\n"), file: &FileResult{Classification: "expected-divergence"}, want: []string{"1:AC70"}},
		{name: "manual fix keeps its whole text", path: "engine/CHANGELOG.md", data: []byte("AC13\n"), file: &FileResult{Classification: "manual-fix", childEntry: "engine"}, want: []string{"1:AC13"}},
		{name: "always-synced child file keeps nothing", path: "engine/AGENTS.md", data: []byte("AC1\n"), file: &FileResult{Classification: "ambiguity", forceSync: true, childEntry: "engine"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			byPath := map[string]FileResult{}
			if tc.file != nil {
				tc.file.Path = tc.path
				byPath[tc.path] = *tc.file
			}
			text, first, kept := keptText(tc.path, tc.data, byPath)
			var got []string
			if kept {
				got = scanIdentifiers(filepath.Base(tc.path), text, first, false)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("findings=%v want=%v (kept=%v first=%d)", got, tc.want, kept, first)
			}
		})
	}
}

// lineOf returns the 1-based line in path that holds needle.
func lineOf(t *testing.T, path, needle string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, needle) {
			return i + 1
		}
	}
	t.Fatalf("%s lacks %q", path, needle)
	return 0
}

// setBaselineHash records content's comparison region as path's baseline hash,
// so a changed governed file reads as safe to update.
func setBaselineHash(t *testing.T, root, path string, content []byte) {
	t.Helper()
	region, ok := canon.ComparisonRegion(path, content)
	if !ok {
		t.Fatalf("%s lacks its comparison region", path)
	}
	basePath := filepath.Join(root, filepath.FromSlash(baselinePath))
	base, err := os.ReadFile(basePath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(base), "\n")
	found := false
	for i, line := range lines {
		if fields := strings.Split(line, "\t"); len(fields) == 3 && fields[0] == path {
			lines[i] = fmt.Sprintf("%s\t%s\t%x", fields[0], fields[1], sha256.Sum256(region))
			found = true
		}
	}
	if !found {
		t.Fatalf("baseline lacks %s", path)
	}
	writeAuditFile(t, basePath, []byte(strings.Join(lines, "\n")))
}

func appendAuditFile(t *testing.T, path, text string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, text...)
	writeAuditFile(t, path, data)
	return data
}

func findingsByPath(findings []IdentifierFinding) map[string]string {
	out := map[string]string{}
	for _, f := range findings {
		out[f.Path] = strings.Join(f.Identifiers, ",")
	}
	return out
}

func TestAuditListsBareIdentifiersInAdoptionAC(t *testing.T) {
	root := fixture(t)
	path := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }
	appendAuditFile(t, path("arch.md"), "\nUse `Package AC70+AC71` to release.\n")
	writeAuditFile(t, path("history.md"), []byte("# History\n\nShipped in AT3.\n"))
	writeAuditFile(t, path("docs/guide.md"), []byte("# Guide\n\nSupports IE11.\n"))
	replaceFileText(t, path("AGENTS.md"), "- Edit governed sections only in AGENTS.md.\n", "- Edit governed sections only in AGENTS.md, as AC6 said.\n")
	appendAuditFile(t, path("AGENTS.md"), "- Keep the AC5 rule.\n")
	release, err := os.ReadFile(path("govna/build-release.md"))
	if err != nil {
		t.Fatal(err)
	}
	release = bytes.Replace(release, []byte("## Project Practices\n"), []byte("Local canon note AC8.\n\n## Project Practices\n"), 1)
	release = append(release, "- Ship AT7 notes.\n"...)
	writeAuditFile(t, path("govna/build-release.md"), release)
	setBaselineHash(t, root, "govna/build-release.md", release)
	readme := appendAuditFile(t, path("README.md"), "\nUse `Package AC70+AC71` to release.\n")
	setBaselineHash(t, root, "README.md", readme)
	appendAuditFile(t, path("govna/roles.md"), "\nSee AC9.\n")
	for link, target := range map[string]string{"docs/agents-link.md": "../AGENTS.md", "docs/readme-link.md": "../README.md", "docs/roles-link.md": "../govna/roles.md"} {
		if err := os.Symlink(target, path(link)); err != nil {
			t.Fatal(err)
		}
	}

	report, body := runAuditFixture(t, root, []string{"--json"})
	classes := map[string]FileResult{}
	for _, file := range report.Files {
		classes[file.Path] = file
	}
	if classes["arch.md"].Classification != "expected-divergence" || classes["README.md"].Classification != "clear-sync" ||
		classes["govna/build-release.md"].Classification != "clear-sync" || classes["AGENTS.md"].EffectiveClassification != "force-sync" ||
		classes["govna/roles.md"].Classification != "ambiguity" {
		t.Fatalf("fixture classifications: arch=%q readme=%q release=%q agents=%q roles=%q", classes["arch.md"].Classification,
			classes["README.md"].Classification, classes["govna/build-release.md"].Classification, classes["AGENTS.md"].EffectiveClassification, classes["govna/roles.md"].Classification)
	}
	archLine := lineOf(t, path("arch.md"), "Package AC70+AC71")
	agentsLine := lineOf(t, path("AGENTS.md"), "Keep the AC5 rule")
	rolesLine := lineOf(t, path("govna/roles.md"), "See AC9")
	want := map[string]string{
		"docs/agents-link.md":    fmt.Sprintf("%d:AC5", agentsLine),
		"docs/roles-link.md":     fmt.Sprintf("%d:AC9", rolesLine),
		"arch.md":                fmt.Sprintf("%d:AC70,%d:AC71", archLine, archLine),
		"history.md":             "3:AT3",
		"docs/guide.md":          "3:IE11",
		"AGENTS.md":              fmt.Sprintf("%d:AC5", agentsLine),
		"govna/build-release.md": fmt.Sprintf("%d:AT7", lineOf(t, path("govna/build-release.md"), "Ship AT7 notes")),
		"govna/roles.md":         fmt.Sprintf("%d:AC9", rolesLine),
	}
	got := findingsByPath(report.IdentifierFindings)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("JSON identifier_findings=%v want=%v", got, want)
	}
	if !strings.Contains(body, fmt.Sprintf("Govna also found %d files with bare identifiers to fix.\n", len(want))) || !strings.Contains(body, "### Bare identifiers to fix\n") {
		t.Fatalf("adoption AC omits the identifier count or group:\n%s", body)
	}
	conditional := map[string]bool{"govna/roles.md": true, "docs/roles-link.md": true}
	if history := classes["history.md"]; history.Classification == "target-has-no-canon" || history.Classification == "ambiguity" {
		conditional["history.md"] = true
	}
	for _, finding := range report.IdentifierFindings {
		choice := conditional[finding.Path]
		if finding.Conditional != choice {
			t.Errorf("%s conditional=%v want %v", finding.Path, finding.Conditional, choice)
		}
		bullet := "- `" + finding.Path + "` — " + identifierLines(finding.Identifiers)
		at := "Verify `" + finding.Path + "`, if it remains, holds no bare AC, AT, or IE identifier that the Markdown identifier scan rejects.\n"
		if choice {
			bullet += "; fix these only when the resolved action keeps this file's current text in place.\n"
			at = "Verify `" + finding.Path + "` holds no bare AC, AT, or IE identifier that the Markdown identifier scan rejects when its resolved action keeps its current text in place.\n"
		} else {
			bullet += ".\n"
		}
		if !strings.Contains(body, bullet) || !strings.Contains(body, at) {
			t.Errorf("adoption AC omits %q or %q:\n%s", bullet, at, body)
		}
	}
}

func TestAuditSkipsRetiredLinkForIdentifiers(t *testing.T) {
	root := fixture(t)
	if err := os.Symlink("AGENTS.md", filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	writeAuditFile(t, filepath.Join(root, "notes.md"), []byte("Shipped in AT3.\n"))
	report, _ := runAuditFixture(t, root, []string{"--json"})
	got := findingsByPath(report.IdentifierFindings)
	if _, linked := got["CLAUDE.md"]; linked || got["notes.md"] != "1:AT3" {
		t.Fatalf("identifier_findings=%v, want only notes.md", got)
	}
}

func TestAuditListsChildIdentifiers(t *testing.T) {
	for _, tc := range []struct {
		name, changelog, classification, want string
	}{
		{name: "canonical shape", changelog: "# Changelog\n\n| Version | Summary |\n|---------|---------|\n| Unreleased | |\n| 1.0.0 | AC12 shipped |\n", classification: "match", want: "6:AC12"},
		{name: "broken shape", changelog: "# Changes\n\nAC13 notes\n", classification: "manual-fix", want: "3:AC13"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, child, entry := childAuditFixture(t)
			writeAuditFile(t, filepath.Join(child, "CHANGELOG.md"), []byte(tc.changelog))
			if err := os.Remove(filepath.Join(root, "govna", "roles.md")); err != nil {
				t.Fatal(err)
			}
			report, _ := runAuditFixture(t, root, []string{"--json"})
			path := entry + "/CHANGELOG.md"
			var file FileResult
			for _, f := range report.Files {
				if f.Path == path {
					file = f
				}
			}
			if file.Classification != tc.classification {
				t.Fatalf("%s classification=%q want %q", path, file.Classification, tc.classification)
			}
			if got := findingsByPath(report.IdentifierFindings)[path]; got != tc.want {
				t.Fatalf("%s findings=%q want %q (all=%v)", path, got, tc.want, report.IdentifierFindings)
			}
		})
	}
}

func TestAuditFindingsNeverWriteAnAC(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		age        bool
	}{
		{name: "up-to-date repository", want: "No Govna updates or Director choices found"},
		{name: "version-only update", want: "Only the canon version changed", age: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			if tc.age {
				ageStamps(t, root, "v0.1.0")
			}
			writeAuditFile(t, filepath.Join(root, "history.md"), []byte("# History\n\nShipped in AT3.\n"))
			git(t, root, "add", ".")
			git(t, root, "commit", "-qm", "history")
			for _, args := range [][]string{{"--check"}, nil} {
				var stdout, stderr bytes.Buffer
				if code := Run(args, &stdout, &stderr, root, testProgramVersion); code != 0 || !strings.HasPrefix(stdout.String(), tc.want) {
					t.Fatalf("audit %v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
				}
			}
			report, body := runAuditFixture(t, root, []string{"--json"})
			if report.Emitted != nil || body != "" || len(report.IdentifierFindings) != 0 {
				t.Fatalf("findings wrote an AC: emitted=%v findings=%v", report.Emitted, report.IdentifierFindings)
			}
			if out, err := exec.Command("git", "-C", root, "status", "--porcelain", "--untracked-files=all").CombinedOutput(); err != nil || len(out) != 0 {
				t.Fatalf("audit changed the repository: %v %s", err, out)
			}
		})
	}
}
