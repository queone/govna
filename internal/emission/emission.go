package emission

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/queone/govna/internal/usererr"
)

var acPattern = regexp.MustCompile(`(?i)AC([0-9]+)`)
var filePattern = regexp.MustCompile(`^ac([0-9]+)-`)

const AuditMarkerPrefix = "<!-- audit: emitted-by govna "
const RemovalMarkerPrefix = "<!-- govna-rm: emitted-by govna "

// GuardedPath allocates or reuses the sole stem-and-version-keyed AC stub.
func GuardedPath(root, stem, version string, command func(string, ...string) ([]byte, error)) (string, bool, error) {
	pattern := regexp.MustCompile(`^ac[0-9]+-` + regexp.QuoteMeta(stem) + `-` + regexp.QuoteMeta(version) + `\.md$`)
	matches := stubPaths(root, func(name string) bool { return pattern.MatchString(name) })
	if len(matches) > 1 {
		return "", false, usererr.Errorf("Govna found more than one generated %s AC for canon %s: %v. Rename extra files so only one matches before retrying", stem, version, matches)
	}
	if len(matches) == 1 {
		return matches[0], true, nil
	}
	n, err := Next(root, command)
	if err != nil {
		return "", false, err
	}
	return fmt.Sprintf("govna/ac%d-%s-%s.md", n, stem, version), false, nil
}

// stubPaths lists the govna/ files whose names match, in byte order.
func stubPaths(root string, match func(string) bool) []string {
	var matches []string
	entries, _ := os.ReadDir(filepath.Join(root, "govna"))
	for _, entry := range entries {
		if !entry.IsDir() && match(entry.Name()) {
			matches = append(matches, filepath.ToSlash(filepath.Join("govna", entry.Name())))
		}
	}
	sort.Strings(matches)
	return matches
}

// auditStubPattern matches a generated audit stub name and captures its AC
// number and canon version.
var auditStubPattern = regexp.MustCompile(`^ac([0-9]+)-audit-(v[^/]+)\.md$`)

// AuditStubVersion returns the canon version in an audit stub path, or "".
func AuditStubVersion(path string) string {
	if m := auditStubPattern.FindStringSubmatch(filepath.Base(path)); m != nil {
		return m[2]
	}
	return ""
}

// AuditPath allocates or reuses the sole canon-version-keyed audit stub. It
// also returns each superseded stub, which an earlier audit wrote for another
// canon version. Without a stub for version, the new stub keeps the AC number
// of a sole superseded stub.
func AuditPath(root, version string, command func(string, ...string) ([]byte, error)) (string, bool, []string, error) {
	superseded := stubPaths(root, func(name string) bool {
		m := auditStubPattern.FindStringSubmatch(name)
		return m != nil && m[2] != version
	})
	current := stubPaths(root, func(name string) bool {
		m := auditStubPattern.FindStringSubmatch(name)
		return m != nil && m[2] == version
	})
	if len(current) > 0 {
		path, reused, err := GuardedPath(root, "audit", version, command)
		return path, reused, superseded, err
	}
	switch len(superseded) {
	case 0:
		path, reused, err := GuardedPath(root, "audit", version, command)
		return path, reused, nil, err
	case 1:
		m := auditStubPattern.FindStringSubmatch(filepath.Base(superseded[0]))
		return fmt.Sprintf("govna/ac%s-audit-%s.md", m[1], version), false, superseded, nil
	default:
		return "", false, nil, usererr.Errorf("Govna found more than one generated audit AC for an older canon version: %v. Keep one, delete the others or rename them off the audit slug, and retry", superseded)
	}
}

// GuardedBody wraps body with deterministic executable and canon provenance.
func GuardedBody(prefix, programVersion, canonVersion string, body []byte) []byte {
	provenance := fmt.Sprintf(
		"executable v%s with embedded canon v%s",
		strings.TrimPrefix(programVersion, "v"),
		strings.TrimPrefix(canonVersion, "v"),
	)
	return guardedBody(prefix, provenance, body)
}

func guardedBody(prefix, provenance string, body []byte) []byte {
	hash := sha256.Sum256(body)
	marker := fmt.Sprintf("%s%s sha256:%x -->\n", prefix, provenance, hash)
	return append([]byte(marker), body...)
}

// VerifyGuardedBody verifies a marker prefix and its body hash.
func VerifyGuardedBody(content []byte, prefix string) bool {
	line, body, ok := strings.Cut(string(content), "\n")
	if !ok || !strings.HasPrefix(line, prefix) {
		return false
	}
	index := strings.LastIndex(line, " sha256:")
	if index < 0 || !strings.HasSuffix(line, " -->") {
		return false
	}
	want := line[index+8 : len(line)-4]
	hash := sha256.Sum256([]byte(body))
	return want == fmt.Sprintf("%x", hash)
}

// AuditBody wraps body with a deterministic edit-detection marker.
func AuditBody(programVersion, canonVersion string, body []byte) []byte {
	return GuardedBody(AuditMarkerPrefix, programVersion, canonVersion, body)
}

// VerifyAuditBody reports whether a generated audit stub remains unedited.
func VerifyAuditBody(content []byte) bool {
	return VerifyGuardedBody(content, AuditMarkerPrefix)
}

func Next(root string, command func(string, ...string) ([]byte, error)) (int, error) {
	max := 0
	entries, _ := os.ReadDir(filepath.Join(root, "govna"))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := filePattern.FindStringSubmatch(entry.Name())
		if len(m) > 0 {
			n, _ := strconv.Atoi(m[1])
			if n > max {
				max = n
			}
		}
	}
	if command == nil {
		command = func(name string, args ...string) ([]byte, error) { return exec.Command(name, args...).CombinedOutput() }
	}
	out, err := command("git", "-C", root, "log", "--all", "--pretty=%B")
	if err != nil {
		text := string(out)
		if !strings.Contains(text, "not a git repository") && !strings.Contains(text, "does not have any commits") && !strings.Contains(text, "bad default revision") && !strings.Contains(text, "Not a valid object name") {
			return 0, usererr.Errorf("Govna could not read Git history to choose the next AC number in %s: %w: %s. Fix Git access before retrying", root, err, strings.TrimSpace(text))
		}
		out = nil
	}
	for _, m := range acPattern.FindAllSubmatch(out, -1) {
		n, _ := strconv.Atoi(string(m[1]))
		if n > max {
			max = n
		}
	}
	return max + 1, nil
}
