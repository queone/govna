package audit

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/repository"
)

// IdentifierFinding lists the bare AC, AT, and IE identifiers in the text that
// one Markdown file keeps after the adoption AC, each as "<line>:<ID>".
type IdentifierFinding struct {
	Path        string   `json:"relpath"`
	Identifiers []string `json:"identifiers"`
	// Conditional marks findings that count only when an unresolved Director
	// choice keeps the scanned text in place.
	Conditional bool `json:"conditional,omitempty"`
}

var (
	identifierPattern    = regexp.MustCompile(`(AC|AT|IE)[0-9]+`)
	identifierACDocument = regexp.MustCompile(`^govna/ac[0-9]+-[^/]+\.md$`)
	identifierPlanLabel  = regexp.MustCompile(`^[ \t]*[-*][ \t]+IE[0-9]+:`)
	identifierHistorical = regexp.MustCompile(`^[ \t]*Historical:`)
)

// identifierFindings applies the Markdown identifier scan's rules to the text
// that the repository and each declared child keep after the adoption AC.
func identifierFindings(access *repository.Access, report Report) ([]IdentifierFinding, error) {
	byPath := map[string]FileResult{}
	for _, file := range report.Files {
		byPath[file.Path] = file
	}
	findings, err := scanKeptMarkdown(access, "", false, byPath)
	if err != nil {
		return nil, err
	}
	children, _, err := access.ReadChildren()
	if err != nil {
		return nil, err
	}
	for _, entry := range children {
		dir, err := repository.ResolveChild(access.Dir(), entry)
		if err != nil {
			return nil, fmt.Errorf("inspect child %w; fix %s or restore the child repository", err, repository.ChildRegistryPath)
		}
		child, err := repository.Open(dir)
		if err != nil {
			return nil, fmt.Errorf("open child %s: %w", entry, err)
		}
		found, err := scanKeptMarkdown(child, entry+"/", true, byPath)
		child.Close()
		if err != nil {
			return nil, err
		}
		findings = append(findings, found...)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Path < findings[j].Path })
	return findings, nil
}

// scanKeptMarkdown scans the Markdown files that access lists. prefix turns a
// child's paths into parent-relative paths, and child drops every allowance.
func scanKeptMarkdown(access *repository.Access, prefix string, child bool, byPath map[string]FileResult) ([]IdentifierFinding, error) {
	paths, err := markdownPaths(access.Dir())
	if err != nil {
		return nil, fmt.Errorf("list Markdown files in %s: %w; fix the listed directory and retry", access.Dir(), err)
	}
	var findings []IdentifierFinding
	for _, rel := range paths {
		if identifierSkipped(rel, child) {
			continue
		}
		// Follow a symbolic link and skip anything but a regular file, as the
		// scan's [ -f ] test does. Outside Git, the listing already holds only
		// regular files, as find -type f does.
		full := filepath.Join(access.Dir(), filepath.FromSlash(rel))
		if info, err := os.Stat(full); err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, inspectError(prefix+rel, err)
		}
		path := prefix + rel
		text, first, kept := keptText(path, data, byPath)
		conditional := directorChoice(byPath, path)
		if target, linked := linkTarget(access.Dir(), full); kept && linked {
			// A link shows its target's text, so it keeps what its target keeps.
			text, first, kept = keptText(prefix+target, data, byPath)
			conditional = conditional || directorChoice(byPath, prefix+target)
		}
		if !kept {
			continue
		}
		if ids := scanIdentifiers(rel, text, first, child); len(ids) > 0 {
			findings = append(findings, IdentifierFinding{Path: path, Identifiers: ids, Conditional: conditional})
		}
	}
	return findings, nil
}

// linkTarget returns the path, relative to root, that the symbolic link at full
// resolves to. It reports false for a regular file or a target outside root.
func linkTarget(root, full string) (string, bool) {
	info, err := os.Lstat(full)
	if err != nil || info.Mode()&fs.ModeSymlink == 0 {
		return "", false
	}
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// directorChoice reports whether path's adoption result waits on a Director
// choice, as the adoption AC's Director-choice group does.
func directorChoice(byPath map[string]FileResult, path string) bool {
	file, listed := byPath[path]
	return listed && !file.forceSync && (file.Classification == "ambiguity" || file.Classification == "target-has-no-canon")
}

// markdownPaths lists each tracked or unignored Markdown file in dir, as the
// build script's scan does. Outside a Git work tree, such as a child checkout
// without Git, it lists every Markdown file, pruning .git, node_modules, and
// vendor directories as the scan's find fallback does.
func markdownPaths(dir string) ([]string, error) {
	if exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree").Run() != nil {
		var paths []string
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path == dir {
					return nil
				}
				switch entry.Name() {
				case ".git", "node_modules", "vendor":
					return filepath.SkipDir
				}
				return nil
			}
			if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".md") {
				rel, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}
				paths = append(paths, filepath.ToSlash(rel))
			}
			return nil
		})
		return paths, err
	}
	out, err := exec.Command("git", "-C", dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", "*.md").Output()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var paths []string
	for path := range strings.SplitSeq(string(out), "\x00") {
		if path != "" && !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// identifierSkipped reports whether the scan skips rel: test fixtures always,
// and the CHANGELOG and active AC documents outside a child repository.
func identifierSkipped(rel string, child bool) bool {
	if strings.HasPrefix(rel, "tests/fixtures/") || strings.HasPrefix(rel, "testdata/") || strings.Contains(rel, "/testdata/") {
		return true
	}
	return !child && (rel == "CHANGELOG.md" || identifierACDocument.MatchString(rel))
}

// keptText returns the text that path keeps after the adoption AC and the line
// number of its first line. It reports false when the AC replaces the file
// whole with canon or deletes it. A file that the AC updates from canon keeps
// only its text from the boundary heading to the end of the file.
func keptText(path string, data []byte, byPath map[string]FileResult) ([]byte, int, bool) {
	file, listed := byPath[path]
	if !listed {
		return data, 1, true
	}
	if file.Classification == "retired-link" {
		return nil, 0, false
	}
	if !file.forceSync && file.Classification != "clear-sync" && file.Classification != "missing-in-target" {
		return data, 1, true
	}
	if file.childEntry != "" {
		return nil, 0, false
	}
	region, ok := canon.ProtectedRegion(path, data)
	if !ok {
		return nil, 0, false
	}
	return region, bytes.Count(data[:len(data)-len(region)], []byte("\n")) + 1, true
}

// scanIdentifiers returns each bare identifier in text as "<line>:<ID>", with
// line numbers counted from first. rel is the path inside its own repository.
func scanIdentifiers(rel string, text []byte, first int, child bool) []string {
	template := !child && rel == "govna/ac-template.md"
	var ids []string
	for index, line := range strings.Split(string(text), "\n") {
		if !child {
			line = stripHistorical(line)
			if rel == "plan.md" && identifierPlanLabel.MatchString(line) {
				label := strings.Index(line, "IE")
				line = line[:label] + "##" + line[label+2:]
			}
		}
		for _, match := range identifierPattern.FindAllStringIndex(line, -1) {
			if match[0] > 0 && identifierByte(line[match[0]-1]) {
				continue
			}
			if match[1] < len(line) && identifierByte(line[match[1]]) {
				continue
			}
			id := line[match[0]:match[1]]
			if template && !strings.HasPrefix(id, "IE") {
				continue
			}
			ids = append(ids, strconv.Itoa(first+index)+":"+id)
		}
	}
	return ids
}

// stripHistorical blanks each one-line <!-- Historical: ... --> comment.
func stripHistorical(line string) string {
	var out strings.Builder
	for {
		before, rest, found := strings.Cut(line, "<!--")
		if !found {
			break
		}
		_, after, ok := strings.Cut(rest, "-->")
		if ok && identifierHistorical.MatchString(rest) {
			out.WriteString(before)
			out.WriteString(" ")
			line = after
			continue
		}
		out.WriteString(before)
		out.WriteString("<!--")
		line = rest
	}
	out.WriteString(line)
	return out.String()
}

// identifierByte reports whether c is an ASCII letter, digit, or underscore.
func identifierByte(c byte) bool {
	return c == '_' || ('0' <= c && c <= '9') || ('A' <= c && c <= 'Z') || ('a' <= c && c <= 'z')
}

// identifierLines formats a finding's identifiers for the adoption AC.
func identifierLines(ids []string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		line, name, _ := strings.Cut(id, ":")
		parts = append(parts, "pre-adoption line "+line+" `"+name+"`")
	}
	return strings.Join(parts, "; ")
}
