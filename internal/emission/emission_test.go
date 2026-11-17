package emission

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNext(t *testing.T) {
	d := t.TempDir()
	os.Mkdir(filepath.Join(d, "govna"), 0755)
	os.WriteFile(filepath.Join(d, "govna/ac4-x.md"), nil, 0644)
	cmd := func(string, ...string) ([]byte, error) { return []byte("AC7 shipped"), nil }
	n, e := Next(d, cmd)
	if e != nil || n != 8 {
		t.Fatalf("n=%d err=%v", n, e)
	}
}
func TestNextErrors(t *testing.T) {
	d := t.TempDir()
	cmd := func(string, ...string) ([]byte, error) { return []byte("fatal: unexpected"), errors.New("exit") }
	if _, e := Next(d, cmd); e == nil || !strings.Contains(e.Error(), "Govna could not read Git history to choose the next AC number") || !strings.Contains(e.Error(), "Fix Git access before retrying") {
		t.Fatalf("unexpected Git error: %v", e)
	}
}

func TestAuditGuard(t *testing.T) {
	d := t.TempDir()
	os.Mkdir(filepath.Join(d, "govna"), 0755)
	cmd := func(string, ...string) ([]byte, error) { return nil, nil }
	path, reused, _, err := AuditPath(d, "v0.29.0", cmd)
	if err != nil || reused || path != "govna/ac1-audit-v0.29.0.md" {
		t.Fatalf("path=%q reused=%v err=%v", path, reused, err)
	}
	body := AuditBody("9.8.7", "0.29.0", []byte("body\n"))
	if !VerifyAuditBody(body) || VerifyAuditBody(append(body, 'x')) {
		t.Fatal("audit marker verification failed")
	}
	if !strings.HasPrefix(string(body), "<!-- audit: emitted-by govna executable v9.8.7 with embedded canon v0.29.0 sha256:") {
		t.Fatalf("unexpected audit marker: %s", body)
	}
	os.WriteFile(filepath.Join(d, filepath.FromSlash(path)), body, 0644)
	got, reused, _, err := AuditPath(d, "v0.29.0", cmd)
	if err != nil || !reused || got != path {
		t.Fatalf("path=%q reused=%v err=%v", got, reused, err)
	}
}

func TestRemovalGuardAndAmbiguity(t *testing.T) {
	d := t.TempDir()
	os.Mkdir(filepath.Join(d, "govna"), 0755)
	cmd := func(string, ...string) ([]byte, error) { return nil, nil }
	path, reused, err := GuardedPath(d, "govna-rm", "v0.29.0", cmd)
	if err != nil || reused || path != "govna/ac1-govna-rm-v0.29.0.md" {
		t.Fatalf("path=%q reused=%v err=%v", path, reused, err)
	}
	body := GuardedBody(RemovalMarkerPrefix, "9.8.7", "0.29.0", []byte("body\n"))
	if !VerifyGuardedBody(body, RemovalMarkerPrefix) || VerifyGuardedBody(append(body, 'x'), RemovalMarkerPrefix) {
		t.Fatal("removal marker verification failed")
	}
	if !strings.HasPrefix(string(body), "<!-- govna-rm: emitted-by govna executable v9.8.7 with embedded canon v0.29.0 sha256:") {
		t.Fatalf("unexpected removal marker: %s", body)
	}
	os.WriteFile(filepath.Join(d, "govna", "ac1-govna-rm-v0.29.0.md"), body, 0644)
	os.WriteFile(filepath.Join(d, "govna", "ac2-govna-rm-v0.29.0.md"), body, 0644)
	_, _, err = GuardedPath(d, "govna-rm", "v0.29.0", cmd)
	want := "Govna found more than one generated govna-rm AC for canon v0.29.0: [govna/ac1-govna-rm-v0.29.0.md govna/ac2-govna-rm-v0.29.0.md]. Rename extra files so only one matches before retrying"
	if err == nil || err.Error() != want {
		t.Fatalf("err=%v", err)
	}
}

func TestAuditGuardReportsEveryDuplicate(t *testing.T) {
	d := t.TempDir()
	if err := os.Mkdir(filepath.Join(d, "govna"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ac1-audit-v0.29.0.md", "ac2-audit-v0.29.0.md"} {
		if err := os.WriteFile(filepath.Join(d, "govna", name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	_, _, _, err := AuditPath(d, "v0.29.0", nil)
	want := "Govna found more than one generated audit AC for canon v0.29.0: [govna/ac1-audit-v0.29.0.md govna/ac2-audit-v0.29.0.md]. Rename extra files so only one matches before retrying"
	if err == nil || err.Error() != want {
		t.Fatalf("err=%v", err)
	}
}

func TestLegacyGuardedBodiesRemainValid(t *testing.T) {
	body := []byte("legacy body\n")
	for _, prefix := range []string{AuditMarkerPrefix, RemovalMarkerPrefix} {
		legacy := guardedBody(prefix, "v0.29.0", body)
		if !VerifyGuardedBody(legacy, prefix) {
			t.Errorf("valid legacy marker rejected for %q", prefix)
		}
		if VerifyGuardedBody(append(legacy, 'x'), prefix) {
			t.Errorf("edited legacy body accepted for %q", prefix)
		}
	}
}

func TestAuditPathKeepsSupersededNumber(t *testing.T) {
	cmd := func(string, ...string) ([]byte, error) { return nil, nil }
	write := func(t *testing.T, d string, names ...string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(d, "govna"), 0o755); err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if err := os.WriteFile(filepath.Join(d, "govna", name), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Run("sole superseded stub", func(t *testing.T) {
		d := t.TempDir()
		write(t, d, "ac7-audit-v0.28.0.md", "ac9-govna-rm-v0.28.0.md")
		path, reused, superseded, err := AuditPath(d, "v0.29.0", cmd)
		if err != nil || reused || path != "govna/ac7-audit-v0.29.0.md" || strings.Join(superseded, ",") != "govna/ac7-audit-v0.28.0.md" {
			t.Fatalf("path=%q reused=%v superseded=%v err=%v", path, reused, superseded, err)
		}
		if got := AuditStubVersion(superseded[0]); got != "v0.28.0" {
			t.Fatalf("version=%q", got)
		}
	})
	t.Run("current stub wins", func(t *testing.T) {
		d := t.TempDir()
		write(t, d, "ac7-audit-v0.28.0.md", "ac9-audit-v0.29.0.md")
		path, reused, superseded, err := AuditPath(d, "v0.29.0", cmd)
		if err != nil || !reused || path != "govna/ac9-audit-v0.29.0.md" || strings.Join(superseded, ",") != "govna/ac7-audit-v0.28.0.md" {
			t.Fatalf("path=%q reused=%v superseded=%v err=%v", path, reused, superseded, err)
		}
	})
	t.Run("several superseded stubs", func(t *testing.T) {
		d := t.TempDir()
		write(t, d, "ac7-audit-v0.28.0.md", "ac8-audit-v0.27.0.md")
		_, _, _, err := AuditPath(d, "v0.29.0", cmd)
		want := "Govna found more than one generated audit AC for an older canon version: [govna/ac7-audit-v0.28.0.md govna/ac8-audit-v0.27.0.md]. Keep one, delete the others or rename them off the audit slug, and retry"
		if err == nil || err.Error() != want {
			t.Fatalf("err=%v", err)
		}
	})
}

func TestParentProfileNamesFilesStepsAndTests(t *testing.T) {
	adds := NewParentProfile(true, []string{"AGENTS.md", "govna/audit.md"}, []string{"`govna/build-release.md` (listed in `govna/preserve.txt`)"})
	for _, want := range []string{
		"### Parent Profile Update\n\nThis parent declares its first child, so its contract and documents gain the child rules.\n",
		"- Complete this section before any other work in this AC.\n",
		"`govna render --with-children <scratch>`",
		"Files to update:\n\n- `AGENTS.md`\n- `govna/audit.md`\n- `govna/canon-baseline.txt`\n",
		"Files left unchanged:\n\n- `govna/build-release.md` (listed in `govna/preserve.txt`)\n",
	} {
		if !strings.Contains(adds.Instructions, want) {
			t.Errorf("profile update omits %q:\n%s", want, adds.Instructions)
		}
	}
	if len(adds.Paths) != 3 || adds.Paths[2] != BaselinePath {
		t.Errorf("paths=%v", adds.Paths)
	}
	removes := NewParentProfile(false, []string{"AGENTS.md"}, nil)
	if !strings.Contains(removes.Instructions, "`govna render <scratch>`") || strings.Contains(removes.Instructions, "--with-children") || strings.Contains(removes.Instructions, "Files left unchanged") {
		t.Errorf("removal profile update:\n%s", removes.Instructions)
	}
	tests := removes.Tests(3)
	for _, want := range []string{"**AT3** [Automated] [Pre-release gate] — Verify each updated parent file", "**AT4** [Automated] [Pre-release gate] — Verify every line from the boundary heading down", "**AT5** [Automated] [Pre-release gate] — Verify each updated file's line in `govna/canon-baseline.txt`"} {
		if !strings.Contains(tests, want) {
			t.Errorf("tests omit %q:\n%s", want, tests)
		}
	}
}
