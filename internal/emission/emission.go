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
)

var acPattern = regexp.MustCompile(`(?i)AC([0-9]+)`)
var filePattern = regexp.MustCompile(`^ac([0-9]+)-`)

const AuditMarkerPrefix = "<!-- audit: emitted-by govna "
const RemovalMarkerPrefix = "<!-- govna-rm: emitted-by govna "

func userMessageErrorf(format string, args ...any) error {
	return fmt.Errorf(format, args...)
}

// GuardedPath allocates or reuses the sole stem-and-version-keyed AC stub.
func GuardedPath(root, stem, version string, command func(string, ...string) ([]byte, error)) (string, bool, error) {
	dir := filepath.Join(root, "govna")
	pattern := regexp.MustCompile(`^ac[0-9]+-` + regexp.QuoteMeta(stem) + `-` + regexp.QuoteMeta(version) + `\.md$`)
	var matches []string
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !entry.IsDir() && pattern.MatchString(entry.Name()) {
			matches = append(matches, filepath.ToSlash(filepath.Join("govna", entry.Name())))
		}
	}
	sort.Strings(matches)
	if len(matches) > 1 {
		return "", false, userMessageErrorf("Govna found more than one generated %s AC for canon %s: %v. Rename extra files so only one matches before retrying", stem, version, matches)
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

// AuditPath allocates or reuses the sole canon-version-keyed audit stub.
func AuditPath(root, version string, command func(string, ...string) ([]byte, error)) (string, bool, error) {
	return GuardedPath(root, "audit", version, command)
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
			return 0, userMessageErrorf("Govna could not read Git history to choose the next AC number in %s: %w: %s. Fix Git access before retrying", root, err, strings.TrimSpace(text))
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
