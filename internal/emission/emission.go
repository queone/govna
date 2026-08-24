package emission

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var acPattern = regexp.MustCompile(`(?i)AC([0-9]+)`)
var filePattern = regexp.MustCompile(`^ac([0-9]+)-`)

const AuditMarkerPrefix = "<!-- audit: emitted-by govna "

// AuditPath allocates or reuses the sole canon-version-keyed audit stub.
func AuditPath(root, version string, command func(string, ...string) ([]byte, error)) (string, bool, error) {
	dir := filepath.Join(root, "govna")
	pattern := regexp.MustCompile(`^ac[0-9]+-audit-` + regexp.QuoteMeta(version) + `\.md$`)
	var matches []string
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if !entry.IsDir() && pattern.MatchString(entry.Name()) {
			matches = append(matches, entry.Name())
		}
	}
	if len(matches) > 1 {
		return "", false, fmt.Errorf("audit: multiple matching audit stubs for %s", version)
	}
	if len(matches) == 1 {
		return filepath.ToSlash(filepath.Join("govna", matches[0])), true, nil
	}
	n, err := Next(root, command)
	if err != nil {
		return "", false, err
	}
	return fmt.Sprintf("govna/ac%d-audit-%s.md", n, version), false, nil
}

// AuditBody wraps body with a deterministic edit-detection marker.
func AuditBody(version string, body []byte) []byte {
	hash := sha256.Sum256(body)
	marker := fmt.Sprintf("%sv%s sha256:%x -->\n", AuditMarkerPrefix, strings.TrimPrefix(version, "v"), hash)
	return append([]byte(marker), body...)
}

// VerifyAuditBody reports whether a generated audit stub remains unedited.
func VerifyAuditBody(content []byte) bool {
	line, body, ok := strings.Cut(string(content), "\n")
	if !ok || !strings.HasPrefix(line, AuditMarkerPrefix) {
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
			return 0, fmt.Errorf("read git log for AC-number allocation in %s: %w: %s", root, err, strings.TrimSpace(text))
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
