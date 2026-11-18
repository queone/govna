package audit

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const retiredGuide = "govna/old-guide.md"

// retiredFixture returns an adopted Go repository whose recorded baseline
// still holds a document that the current canon no longer ships. present
// writes that document with the recorded content.
func retiredFixture(t *testing.T, present bool) string {
	t.Helper()
	root := fixture(t)
	content := []byte("# Old guide\n\nGovna shipped this text once.\n")
	baseline := filepath.Join(root, filepath.FromSlash(baselinePath))
	data, err := os.ReadFile(baseline)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	entries := append(lines[2:], fmt.Sprintf("%s\tfull\t%x", retiredGuide, sha256.Sum256(content)))
	sort.Strings(entries)
	writeAuditFile(t, baseline, []byte(strings.Join(append(lines[:2:2], entries...), "\n")+"\n"))
	if present {
		writeAuditFile(t, filepath.Join(root, filepath.FromSlash(retiredGuide)), content)
	}
	return root
}

func commitFixture(t *testing.T, root string) {
	t.Helper()
	git(t, root, "add", ".")
	git(t, root, "commit", "-qm", "local work")
}

func TestUneditedRetiredFileIsDeletedWithoutDirectorChoice(t *testing.T) {
	root := retiredFixture(t, true)
	commitFixture(t, root)
	report, body := runAuditFixture(t, root, []string{"--json"})
	file := reportFile(t, report, retiredGuide)
	if file.Classification != "retired-file" || file.Diff != "" {
		t.Fatalf("retired guide entry=%+v", file)
	}
	for _, want := range []string{
		"Govna also found 1 retired Govna file that this AC deletes.\n",
		"### Retired Govna files\n\n- `govna/old-guide.md` — `retired-file`: a document that left the Govna files, that the repository never edited, and that the AC deletes.\n",
		"- Verify `govna/old-guide.md` is absent from the selected scratch render with `test ! -e <scratch>/govna/old-guide.md`.\n",
		"- Delete `govna/old-guide.md`, as listed under `### Retired Govna files`.\n",
		"[Automated] [Pre-release gate] — Verify `govna/old-guide.md` no longer exists.\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("emitted AC omits %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "### Routing Decisions") {
		t.Errorf("an unedited retired file asks for a Director choice:\n%s", body)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(retiredGuide))); err != nil {
		t.Fatalf("audit changed the retired file: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"--check"}, &stdout, &stderr, root, testProgramVersion); code != 3 || !strings.Contains(stdout.String(), "1 retired Govna file") {
		t.Fatalf("check code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRetiredFileWithLocalStateKeepsItsDirectorChoice(t *testing.T) {
	guide := filepath.FromSlash(retiredGuide)
	t.Run("edited", func(t *testing.T) {
		root := retiredFixture(t, true)
		writeAuditFile(t, filepath.Join(root, guide), []byte("# Old guide\n\nLocal notes.\n"))
		commitFixture(t, root)
		report, body := runAuditFixture(t, root, []string{"--json"})
		if file := reportFile(t, report, retiredGuide); file.Classification != "target-has-no-canon" {
			t.Fatalf("edited retired guide classification=%q", file.Classification)
		}
		if !strings.Contains(body, "**`govna/old-guide.md`**: Which action should Govna record: keep local (preserve), move content to a destination named in the response (migrate), or remove (delete)?\n") {
			t.Errorf("edited retired file lost its routing question:\n%s", body)
		}
		if strings.Contains(body, "### Retired Govna files") {
			t.Errorf("edited retired file is listed for deletion:\n%s", body)
		}
	})
	t.Run("preserved", func(t *testing.T) {
		root := retiredFixture(t, true)
		writeAuditFile(t, filepath.Join(root, filepath.FromSlash(preservePath)), []byte("govna-preserve-v1\n"+retiredGuide+"\n"))
		commitFixture(t, root)
		report, body := runAuditFixture(t, root, []string{"--json"})
		if file := reportFile(t, report, retiredGuide); file.Classification != "preserve" {
			t.Fatalf("preserved retired guide classification=%q", file.Classification)
		}
		if strings.Contains(body, "### Retired Govna files") {
			t.Errorf("preserved retired file is listed for deletion:\n%s", body)
		}
	})
	t.Run("legacy phrase", func(t *testing.T) {
		root := retiredFixture(t, true)
		writeAuditFile(t, filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n\n## Unreleased\n\n- preserve "+retiredGuide+"\n"))
		commitFixture(t, root)
		report, _ := runAuditFixture(t, root, []string{"--json"})
		if file := reportFile(t, report, retiredGuide); file.Classification != "target-has-no-canon" {
			t.Fatalf("retired guide with a legacy phrase classification=%q", file.Classification)
		}
	})
}

func TestAdoptionListsReferencesToRetiredFiles(t *testing.T) {
	write := func(t *testing.T, root string) {
		t.Helper()
		for path, content := range map[string]string{
			"docs/notes.md":      "# Notes\n\nRead `govna/old-guide.md` first.\nThen xgovna/old-guide.md and govna/old-guide.md.bak are other files.\nSee govna/old-guide.md.\n",
			"govna/local.md":     "# Local\n\nSee the [guide](old-guide.md#top).\n",
			"docs/unrelated.md":  "# Other\n\nSee the [guide](old-guide.md).\n",
			"history.md":         "# History\n\nAdopted govna/old-guide.md.\n",
			"govna/ac7-local.md": "# AC7\n\nNames govna/old-guide.md.\n",
		} {
			writeAuditFile(t, filepath.Join(root, filepath.FromSlash(path)), []byte(content))
		}
		commitFixture(t, root)
	}
	want := []ReferenceFinding{
		{Path: "docs/notes.md", References: []string{"3:govna/old-guide.md", "4:govna/old-guide.md", "5:govna/old-guide.md"}},
		{Path: "govna/local.md", References: []string{"3:govna/old-guide.md"}},
	}
	for _, present := range []bool{true, false} {
		t.Run(fmt.Sprintf("present=%v", present), func(t *testing.T) {
			root := retiredFixture(t, present)
			write(t, root)
			report, body := runAuditFixture(t, root, []string{"--json"})
			if !reflect.DeepEqual(report.RetiredReferences, want) {
				t.Fatalf("references=%+v want %+v", report.RetiredReferences, want)
			}
			for _, wantText := range []string{
				"Govna also found 2 files with references to retired files.\n",
				"### References to retired files\n\nRemove or repoint each reference below.",
				"- `docs/notes.md` — line 3 (`govna/old-guide.md`), line 4 (`govna/old-guide.md`), line 5 (`govna/old-guide.md`).\n",
				"- `govna/local.md` — line 3 (`govna/old-guide.md`).\n",
				"- Remove or repoint each reference listed under `### References to retired files`.\n",
				"[Automated] [Pre-release gate] — Verify `docs/notes.md`, if it remains, no longer names `govna/old-guide.md`.\n",
				"[Automated] [Pre-release gate] — Verify `govna/local.md`, if it remains, no longer names `govna/old-guide.md`.\n",
			} {
				if !strings.Contains(body, wantText) {
					t.Errorf("emitted AC omits %q:\n%s", wantText, body)
				}
			}
			for _, skipped := range []string{"`history.md` — line", "`docs/unrelated.md` — line", "`govna/ac7-local.md` — line"} {
				if strings.Contains(body, skipped) {
					t.Errorf("emitted AC lists a skipped file %q", skipped)
				}
			}
			if file := reportFile(t, report, "plan.md"); file.Classification != "expected-divergence" && file.Classification != "match" {
				t.Errorf("a reference scan changed a classification: %+v", file)
			}
		})
	}
	t.Run("conditional on a Director choice", func(t *testing.T) {
		root := retiredFixture(t, true)
		writeAuditFile(t, filepath.Join(root, filepath.FromSlash(retiredGuide)), []byte("# Old guide\n\nLocal notes.\n"))
		write(t, root)
		report, body := runAuditFixture(t, root, []string{"--json"})
		if len(report.RetiredReferences) != 2 || !report.RetiredReferences[0].Conditional {
			t.Fatalf("references=%+v", report.RetiredReferences)
		}
		for _, wantText := range []string{
			"- `docs/notes.md` — line 3 (`govna/old-guide.md`), line 4 (`govna/old-guide.md`), line 5 (`govna/old-guide.md`); fix each reference to a file that needs a Director choice only when the resolved action removes that file.\n",
			"Verify `docs/notes.md`, if it remains, names no retired file that this AC or a resolved action removes: `govna/old-guide.md`.\n",
		} {
			if !strings.Contains(body, wantText) {
				t.Errorf("emitted AC omits %q:\n%s", wantText, body)
			}
		}
	})
}

func TestReferencesAloneWriteNoAdoptionAC(t *testing.T) {
	root := fixture(t)
	writeAuditFile(t, filepath.Join(root, "docs", "notes.md"), []byte("# Notes\n\nSee govna/canon-cycle.md and govna/old-guide.md.\n"))
	commitFixture(t, root)
	for _, args := range [][]string{{"--check"}, nil} {
		var stdout, stderr bytes.Buffer
		if code := Run(args, &stdout, &stderr, root, testProgramVersion); code != 0 || !strings.Contains(stdout.String(), "No AC was written.") {
			t.Fatalf("args=%v code=%d stdout=%q stderr=%q", args, code, stdout.String(), stderr.String())
		}
	}
	if matches, _ := filepath.Glob(filepath.Join(root, "govna", "ac*-audit-*.md")); len(matches) != 0 {
		t.Fatalf("references alone wrote %v", matches)
	}
}

func TestAdoptionACCarriesLegacyStateInstructions(t *testing.T) {
	root := fixture(t)
	writeAuditFile(t, filepath.Join(root, "govna", "build-release.md"), []byte("# Build\n\nRun the local release script.\n"))
	writeAuditFile(t, filepath.Join(root, "local.md"), []byte("# Local\n"))
	writeAuditFile(t, filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n\n## Unreleased\n\n- preserve govna/build-release.md\n- preserve local.md\n"))
	commitFixture(t, root)
	_, body := runAuditFixture(t, root, []string{"--json"})
	for _, want := range []string{
		// A build guide without its boundary.
		"- Place the reviewed repository-specific release mechanics of `govna/build-release.md` below a new `## Project Practices` heading for an update choice.\n",
		"- Replace the text of `govna/build-release.md` above that heading with the rendered canon region for an update choice.\n",
		"**`govna/build-release.md`**: Which action should Govna record: update (sync), keep local (preserve), move content to a destination named in the response (migrate), or remove (delete)?\n",
		"Verify `govna/build-release.md` is absent from `govna/preserve.txt` when its resolved action is sync.\n",
		// A legacy preserve phrase on a file with its own action.
		"- Convert `preserve govna/build-release.md` into `govna/preserve.txt` for a preserve choice on `govna/build-release.md`.\n",
		"- Verify every applicable resolved-route AT for `govna/build-release.md` before legacy-phrase cleanup.\n",
		"- Remove `preserve govna/build-release.md` from `CHANGELOG.md` after resolved-state verification for `govna/build-release.md`.\n",
		// A marker-only phrase.
		"**`local.md`**: Which action should Govna record for `preserve local.md`: convert the exact legacy phrase into `govna/preserve.txt`, or remove only the phrase?\n",
		"- Convert `preserve local.md` into `govna/preserve.txt` for a conversion choice on `local.md`.\n",
		"- Verify every applicable marker-only AT for `local.md` before legacy-phrase cleanup.\n",
		"- Remove `preserve local.md` from `CHANGELOG.md` after marker-only verification for `local.md`.\n",
		"for every marker-only choice.\n",
		"Verify `local.md` occurs exactly once in `govna/preserve.txt` when its marker-only action is convert.\n",
		"Verify `local.md` remains absent from `govna/preserve.txt` when its marker-only action is remove.\n",
		"Verify the Unreleased CHANGELOG Summary changes only through exact removal of `preserve local.md` for `local.md`.\n",
		"Verify every CHANGELOG line outside the Unreleased Summary remains byte-identical for `local.md`.\n",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("emitted AC omits %q", want)
		}
	}
	if t.Failed() {
		t.Log(body)
	}
}
