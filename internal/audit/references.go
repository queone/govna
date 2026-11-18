package audit

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/queone/govna/internal/repository"
)

// ReferenceFinding lists the references to retired canon documents in the text
// that one Markdown file keeps after the adoption AC, each as "<line>:<path>".
type ReferenceFinding struct {
	Path       string   `json:"relpath"`
	References []string `json:"references"`
	// Conditional marks a file with a reference to a retired document that
	// waits on a Director choice, which may keep that document.
	Conditional bool `json:"conditional,omitempty"`
}

// retired returns the distinct retired paths that the finding names, sorted.
func (f ReferenceFinding) retired() []string {
	seen := map[string]bool{}
	var paths []string
	for _, reference := range f.References {
		_, retired, _ := strings.Cut(reference, ":")
		if !seen[retired] {
			seen[retired] = true
			paths = append(paths, retired)
		}
	}
	sort.Strings(paths)
	return paths
}

// referenceLines renders "<line>:<path>" references as "line 7 (`path`)" items.
func referenceLines(references []string) string {
	items := make([]string, 0, len(references))
	for _, reference := range references {
		line, retired, _ := strings.Cut(reference, ":")
		items = append(items, "line "+line+" (`"+retired+"`)")
	}
	return strings.Join(items, ", ")
}

// referenceFindings scans the Markdown text that the repository keeps after the
// adoption AC for the path of each document that left the canon. It skips the
// records of past work: CHANGELOG.md, history.md, and AC documents.
func referenceFindings(access *repository.Access, report Report) ([]ReferenceFinding, error) {
	if len(report.retiredPaths) == 0 {
		return nil, nil
	}
	byPath := map[string]FileResult{}
	for _, file := range report.Files {
		byPath[file.Path] = file
	}
	retired := map[string]bool{}
	for _, retiredPath := range report.retiredPaths {
		retired[retiredPath] = true
	}
	paths, err := markdownPaths(access.Dir())
	if err != nil {
		return nil, fmt.Errorf("list Markdown files in %s: %w; fix the listed directory and retry", access.Dir(), err)
	}
	var findings []ReferenceFinding
	for _, rel := range paths {
		if retired[rel] || rel == "history.md" || identifierSkipped(rel, false) {
			continue
		}
		full := filepath.Join(access.Dir(), filepath.FromSlash(rel))
		if info, err := os.Lstat(full); err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, inspectError(rel, err)
		}
		text, first, kept := keptText(rel, data, byPath)
		if !kept {
			continue
		}
		finding := ReferenceFinding{Path: rel}
		for index, line := range strings.Split(string(text), "\n") {
			for _, retiredPath := range report.retiredPaths {
				if !namesPath(line, rel, retiredPath) {
					continue
				}
				finding.References = append(finding.References, strconv.Itoa(first+index)+":"+retiredPath)
				if directorChoice(byPath, retiredPath) {
					finding.Conditional = true
				}
			}
		}
		if len(finding.References) > 0 {
			findings = append(findings, finding)
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Path < findings[j].Path })
	return findings, nil
}

// namesPath reports whether line, in the file at rel, names retiredPath: by its
// repository-relative path anywhere, or by a Markdown link from the same
// directory that uses the bare file name.
func namesPath(line, rel, retiredPath string) bool {
	for rest := line; ; {
		index := strings.Index(rest, retiredPath)
		if index < 0 {
			break
		}
		end := index + len(retiredPath)
		if (index == 0 || !pathByte(rest[index-1])) && (end == len(rest) || !pathByte(rest[end])) {
			return true
		}
		rest = rest[end:]
	}
	if path.Dir(rel) != path.Dir(retiredPath) || path.Dir(rel) == "." {
		return false
	}
	base := path.Base(retiredPath)
	return strings.Contains(line, "]("+base+")") || strings.Contains(line, "]("+base+"#") || strings.Contains(line, "](./"+base+")")
}

// pathByte reports whether c can extend a path, so that a longer path that
// only contains a retired path does not count as a reference to it.
func pathByte(c byte) bool {
	return c == '_' || c == '-' || c == '/' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}
