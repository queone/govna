package emission

import (
	"errors"
	"os"
	"path/filepath"
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
	if _, e := Next(d, cmd); e == nil {
		t.Fatal("unexpected git error ignored")
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
	body := AuditBody("v0.29.0", []byte("body\n"))
	if !VerifyAuditBody(body) || VerifyAuditBody(append(body, 'x')) {
		t.Fatal("audit marker verification failed")
	}
	os.WriteFile(filepath.Join(d, filepath.FromSlash(path)), body, 0644)
	got, reused, err := AuditPath(d, "v0.29.0", cmd)
	if err != nil || !reused || got != path {
		t.Fatalf("path=%q reused=%v err=%v", got, reused, err)
	}
}
