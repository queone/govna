package emission

import (
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
