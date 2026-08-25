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
	path, reused, err := AuditPath(d, "v0.29.0", cmd)
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
	got, reused, err := AuditPath(d, "v0.29.0", cmd)
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
	_, _, err := AuditPath(d, "v0.29.0", nil)
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
