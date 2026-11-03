package repository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/usererr"
)

// PathError describes a rejected repository path together with its recovery action.
type PathError struct {
	Path, Problem, Recovery string
}

func (e *PathError) Error() string {
	return e.Path + " " + e.Problem + "; " + e.Recovery
}

// ValidatePath reports why rel is not a nonempty, normalized, repository-relative slash path.
func ValidatePath(rel string) error {
	if rel == "" {
		return errors.New("is empty")
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return errors.New("is an absolute path")
	}
	if strings.ContainsRune(rel, '\\') {
		return errors.New("contains a backslash")
	}
	for _, r := range rel {
		if r < 0x20 || r == 0x7f {
			return errors.New("contains a control character")
		}
	}
	for part := range strings.SplitSeq(rel, "/") {
		switch part {
		case "":
			return errors.New("contains an empty path component")
		case ".", "..":
			return fmt.Errorf("contains a %q component", part)
		}
	}
	return nil
}

// ReadHook performs every contained file read; a test replaces it to inject a deterministic failure.
var ReadHook = func(root *os.Root, rel string) ([]byte, error) { return root.ReadFile(rel) }

// Access is a contained handle on a resolved repository root.
type Access struct {
	root *os.Root
	dir  string
}

// Open resolves root through any filesystem alias and returns a handle contained beneath it.
func Open(root string) (*Access, error) {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return nil, err
	}
	handle, err := os.OpenRoot(resolved)
	if err != nil {
		return nil, err
	}
	return &Access{root: handle, dir: resolved}, nil
}

// Close releases the contained handle.
func (a *Access) Close() error { return a.root.Close() }

// Dir returns the resolved root directory.
func (a *Access) Dir() string { return a.dir }

// Preflight validates rel beneath the root before any read or write.
// It rejects an escaping path, a symbolic link at any directory component, and a
// directory, special file, or link at the leaf.
func (a *Access) Preflight(rel string) error {
	if err := ValidatePath(rel); err != nil {
		return &PathError{Path: rel, Problem: err.Error(), Recovery: "use a normalized repository-relative path"}
	}
	parts := strings.Split(rel, "/")
	for i := 1; i < len(parts); i++ {
		dir := strings.Join(parts[:i], "/")
		info, err := a.root.Lstat(dir)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return &PathError{Path: dir, Problem: "is a symbolic link", Recovery: "replace the link with a real directory and retry"}
		}
		if !info.IsDir() {
			return &PathError{Path: dir, Problem: "is not a directory", Recovery: "move the file aside and retry"}
		}
	}
	info, err := a.root.Lstat(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	mode := info.Mode()
	switch {
	case mode&fs.ModeSymlink != 0:
		return &PathError{Path: rel, Problem: "is a symbolic link", Recovery: "replace the link with a regular file and retry"}
	case mode.IsDir():
		return &PathError{Path: rel, Problem: "is a directory, not a regular file", Recovery: "move the directory aside and retry"}
	case !mode.IsRegular():
		return &PathError{Path: rel, Problem: "is not a regular file", Recovery: "remove the special file and retry"}
	}
	return nil
}

// Lstat returns the entry at rel without following a leaf link.
func (a *Access) Lstat(rel string) (fs.FileInfo, error) {
	if err := ValidatePath(rel); err != nil {
		return nil, &PathError{Path: rel, Problem: err.Error(), Recovery: "use a normalized repository-relative path"}
	}
	return a.root.Lstat(rel)
}

// Regular reports whether a regular file exists at rel after preflight.
func (a *Access) Regular(rel string) (bool, error) {
	if err := a.Preflight(rel); err != nil {
		return false, err
	}
	_, err := a.root.Lstat(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// ReadFile reads the regular file at rel after preflight.
func (a *Access) ReadFile(rel string) ([]byte, error) {
	if err := a.Preflight(rel); err != nil {
		return nil, err
	}
	return a.readContained(rel)
}

// ReadOptional reads rel and reports absence separately from every other failure.
func (a *Access) ReadOptional(rel string) ([]byte, bool, error) {
	data, err := a.ReadFile(rel)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// WriteFile writes the regular file at rel with perm after preflight, creating parent directories.
func (a *Access) WriteFile(rel string, data []byte, perm fs.FileMode) error {
	if err := a.Preflight(rel); err != nil {
		return err
	}
	return a.writeContained(rel, data, perm)
}

// Remove removes the entry at rel without following a leaf link.
func (a *Access) Remove(rel string) error {
	if err := ValidatePath(rel); err != nil {
		return &PathError{Path: rel, Problem: err.Error(), Recovery: "use a normalized repository-relative path"}
	}
	return a.root.Remove(rel)
}

// Readlink returns the link text at rel without following it.
func (a *Access) Readlink(rel string) (string, error) {
	if err := ValidatePath(rel); err != nil {
		return "", &PathError{Path: rel, Problem: err.Error(), Recovery: "use a normalized repository-relative path"}
	}
	return a.root.Readlink(rel)
}

// AgentFileState classifies the CLAUDE.md entry at the repository root.
type AgentFileState int

const (
	// AgentFileAbsent means the root holds no regular file or link named CLAUDE.md.
	AgentFileAbsent AgentFileState = iota
	// AgentFileRetiredLink means CLAUDE.md is the link Govna used to create: a symbolic link to AGENTS.md.
	AgentFileRetiredLink
	// AgentFileOwned means CLAUDE.md is a repository-owned regular file or link.
	AgentFileOwned
)

// AgentFilePath is the root instruction file that Claude Code reads instead of AGENTS.md.
const AgentFilePath = "CLAUDE.md"

// DeletableHint tells the operator that a repository-owned CLAUDE.md is no longer needed.
const DeletableHint = "hint: CLAUDE.md can now be deleted; move anything you still need into AGENTS.md first. Claude Code v2.1.277 or later reads AGENTS.md directly and skips it while CLAUDE.md exists — see https://code.claude.com/docs/en/memory#agents-md"

// minAgentsFileVersion is the first Claude Code version that reads AGENTS.md directly.
var minAgentsFileVersion = [3]int{2, 1, 277}

// agentVersionLimit bounds the Claude Code version probe.
const agentVersionLimit = 2 * time.Second

// AgentVersionHook returns the output of `claude --version`; a test replaces it so no test runs the real program.
var AgentVersionHook = probeAgentVersion

func probeAgentVersion() ([]byte, error) {
	program, err := exec.LookPath("claude")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), agentVersionLimit)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, "--version")
	// A child that inherits stdout must not hold the probe open past the limit.
	cmd.WaitDelay = 100 * time.Millisecond
	return cmd.Output()
}

// AgentFile reports whether the root CLAUDE.md is absent, the retired Govna link, or repository-owned.
// A directory or special file counts as absent.
func (a *Access) AgentFile() (AgentFileState, error) {
	info, err := a.root.Lstat(AgentFilePath)
	if errors.Is(err, fs.ErrNotExist) {
		return AgentFileAbsent, nil
	}
	if err != nil {
		return AgentFileAbsent, err
	}
	switch mode := info.Mode(); {
	case mode&fs.ModeSymlink != 0:
		target, err := a.root.Readlink(AgentFilePath)
		if err != nil {
			return AgentFileAbsent, err
		}
		if target == "AGENTS.md" {
			return AgentFileRetiredLink, nil
		}
		return AgentFileOwned, nil
	case mode.IsRegular():
		return AgentFileOwned, nil
	}
	return AgentFileAbsent, nil
}

// UpgradeHint returns the upgrade hint for an installed Claude Code that cannot read AGENTS.md.
// It returns "" when the program is absent, the probe fails, or its output does not parse.
func UpgradeHint() string {
	out, err := AgentVersionHook()
	if err != nil {
		return ""
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return ""
	}
	version, ok := parseAgentVersion(fields[0])
	if !ok || slices.Compare(version[:], minAgentsFileVersion[:]) >= 0 {
		return ""
	}
	return fmt.Sprintf("hint: Claude Code %s cannot read AGENTS.md; upgrade to v2.1.277 or later with \"claude update\"", fields[0])
}

func parseAgentVersion(text string) ([3]int, bool) {
	var version [3]int
	core, _, _ := strings.Cut(strings.TrimPrefix(text, "v"), "-")
	core, _, _ = strings.Cut(core, "+")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return version, false
	}
	for i, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return version, false
		}
		version[i] = n
	}
	return version, true
}

// WriteAgentHints prints the upgrade hint and, for a repository-owned CLAUDE.md, the deletable hint.
func (a *Access) WriteAgentHints(w io.Writer) {
	if hint := UpgradeHint(); hint != "" {
		fmt.Fprintln(w, hint)
	}
	if state, err := a.AgentFile(); err == nil && state == AgentFileOwned {
		fmt.Fprintln(w, DeletableHint)
	}
}

func (a *Access) readContained(rel string) ([]byte, error) {
	return ReadHook(a.root, rel)
}

func (a *Access) writeContained(rel string, data []byte, perm fs.FileMode) error {
	if dir := path.Dir(rel); dir != "." {
		if err := a.root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if err := a.root.WriteFile(rel, data, perm); err != nil {
		return err
	}
	return a.root.Chmod(rel, perm)
}

func Flavor(root, explicit string) (canon.Flavor, error) {
	if explicit == "code" {
		return canon.Code, nil
	}
	if explicit == "doc" {
		return canon.Doc, nil
	}
	metadata, present, err := readContainedOptional(root, "govna/metadata.txt")
	if err != nil {
		return "", fmt.Errorf("read %s: %v", filepath.Join(root, "govna", "metadata.txt"), err)
	}
	if present {
		text := string(metadata)
		if !strings.HasSuffix(text, "\n") {
			return "", fmt.Errorf("invalid %s: require a final newline", filepath.Join(root, "govna", "metadata.txt"))
		}
		values := map[string]string{}
		for line := range strings.SplitSeq(strings.TrimSuffix(text, "\n"), "\n") {
			k, v, ok := strings.Cut(line, " = ")
			if !ok || k == "" || v == "" {
				return "", fmt.Errorf("invalid %s: each line must use `key = value`", filepath.Join(root, "govna", "metadata.txt"))
			}
			values[k] = v
		}
		switch values["repo_type"] {
		case "CODE":
			return canon.Code, nil
		case "DOC":
			return canon.Doc, nil
		case "":
			return "", fmt.Errorf("invalid govna/metadata.txt: missing repo_type")
		default:
			return "", fmt.Errorf("invalid govna/metadata.txt: unknown repo_type %q", values["repo_type"])
		}
	}
	hasDoc := exists(filepath.Join(root, "_config.yml"))
	hasCode := false
	for _, name := range []string{"go.mod", "Cargo.toml", "Package.swift", ".terraform.lock.hcl"} {
		hasCode = hasCode || regularFile(filepath.Join(root, name))
	}
	if hasRegularMatch(filepath.Join(root, "*.tf")) {
		hasCode = true
	}
	if hasCode && hasDoc {
		return "", usererr.Errorf("Govna found both CODE and DOC evidence: the repository has _config.yml and a CODE project manifest; pass --flavor code or --flavor doc")
	}
	if hasCode {
		return canon.Code, nil
	}
	if hasDoc {
		return canon.Doc, nil
	}
	if stack, manifest := unsupportedStackManifest(root); stack != "" {
		return "", usererr.Errorf("Govna found %s for the unsupported %s CODE stack; use %s", manifest, stack, canon.SupportedStackChoices)
	}
	return "", usererr.Errorf("Govna could not determine whether this is a CODE or DOC repository; add govna/metadata.txt, pass --flavor code|doc, or add a recognized project manifest")
}

func Stack(root string) string {
	for _, x := range []struct{ f, s string }{{"go.mod", "Go"}, {".terraform.lock.hcl", "Terraform"}, {"Cargo.toml", "Rust"}, {"Package.swift", "Swift"}} {
		if regularFile(filepath.Join(root, x.f)) {
			return x.s
		}
	}
	if hasRegularMatch(filepath.Join(root, "*.tf")) {
		return "Terraform"
	}
	return ""
}

func unsupportedStackManifest(root string) (string, string) {
	for _, item := range []struct{ file, stack string }{
		{"package.json", "Node"},
		{"pyproject.toml", "Python"},
		{"pom.xml", "Java"},
		{"build.gradle", "Java"},
	} {
		if regularFile(filepath.Join(root, item.file)) {
			return item.stack, item.file
		}
	}
	return "", ""
}
func ModulePath(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func Name(root, module, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if module != "" {
		return path.Base(module)
	}
	return filepath.Base(root)
}
func IsSource(root string) bool {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return false
	}
	module := false
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == "module github.com/queone/govna" {
			module = true
		}
	}
	return module && exists(filepath.Join(root, "internal/canon/assets/base/AGENTS.md.tmpl")) && exists(filepath.Join(root, "cmd/govna/main.go"))
}

// RequireAdopted verifies the stable audit adoption signals.
func RequireAdopted(root string) error {
	access, err := Open(root)
	if err != nil {
		return fmt.Errorf("open %s: %w", root, err)
	}
	defer access.Close()
	info, err := access.Lstat("AGENTS.md")
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("AGENTS.md must be a readable regular file")
	}
	if _, err := access.ReadFile("AGENTS.md"); err != nil {
		return fmt.Errorf("read AGENTS.md: %w", err)
	}
	for _, name := range []string{"govna/ac-template.md", "govna/release.md", "govna/build-release.md"} {
		if _, err := access.Lstat(name); err == nil {
			return nil
		}
	}
	data, _ := access.ReadFile("CHANGELOG.md")
	text := string(data)
	if strings.Contains(text, "govna apply") || strings.Contains(text, "govna render") || strings.Contains(text, "govna render-canon") {
		return nil
	}
	return usererr.Errorf("Govna could not find the files that confirm Govna was added to this repository; expected AGENTS.md plus govna/ac-template.md, govna/release.md, govna/build-release.md, or a CHANGELOG.md entry for govna apply or govna render")
}

// RequireGitWorktree verifies Git availability and worktree membership.
func RequireGitWorktree(root string) error {
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git binary not found on PATH")
	}
	cmd := exec.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return fmt.Errorf("target %s is not a git worktree", root)
	}
	return nil
}

// ChildRegistryPath is the parent's list of declared child repositories.
const ChildRegistryPath = "govna/children.txt"

const childRegistryHeader = "govna-children-v1"

// ValidateChildEntry reports why entry is not a slash-normalized path relative to
// the parent root. Leading ".." components are allowed; "." components, a ".."
// after a name, and empty components are not.
func ValidateChildEntry(entry string) error {
	switch {
	case entry == "":
		return errors.New("is empty")
	case strings.HasPrefix(entry, "/") || filepath.IsAbs(entry):
		return errors.New("is an absolute path")
	case strings.HasSuffix(entry, "/"):
		return errors.New("ends with a slash")
	case strings.ContainsRune(entry, '\\'):
		return errors.New("contains a backslash")
	}
	for _, r := range entry {
		if r < 0x20 || r == 0x7f {
			return errors.New("contains a tab or another control character")
		}
	}
	named := false
	for part := range strings.SplitSeq(entry, "/") {
		switch part {
		case "":
			return errors.New("contains an empty path component")
		case ".":
			return errors.New(`contains a "." component`)
		case "..":
			if named {
				return errors.New(`contains a ".." component after a directory name`)
			}
		default:
			named = true
		}
	}
	if !named {
		return errors.New("names no directory")
	}
	return nil
}

// ParseChildren validates registry bytes and returns its entries in file order.
func ParseChildren(data []byte) ([]string, error) {
	bad := func(problem string) ([]string, error) {
		return nil, usererr.Errorf("invalid %s: %s; keep the %s header, then one byte-sorted child path per line", ChildRegistryPath, problem, childRegistryHeader)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		return bad("require a final newline")
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if lines[0] != childRegistryHeader {
		return bad("first line must be " + childRegistryHeader)
	}
	entries := lines[1:]
	for i, entry := range entries {
		if err := ValidateChildEntry(entry); err != nil {
			return bad(fmt.Sprintf("entry %q %v", entry, err))
		}
		if i > 0 && entry <= entries[i-1] {
			return bad("entries must be unique and byte-sorted")
		}
	}
	return entries, nil
}

// FormatChildren renders entries as a registry with its header, sorted and unique.
func FormatChildren(entries []string) []byte {
	sorted := slices.Clone(entries)
	slices.Sort(sorted)
	sorted = slices.Compact(sorted)
	return []byte(childRegistryHeader + "\n" + strings.Join(append(sorted, ""), "\n"))
}

// ReadChildren reads and validates the parent's child registry.
func (a *Access) ReadChildren() ([]string, bool, error) {
	data, present, err := a.ReadOptional(ChildRegistryPath)
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", ChildRegistryPath, err)
	}
	if !present {
		return nil, false, nil
	}
	entries, err := ParseChildren(data)
	return entries, true, err
}

// ResolveChild resolves a child path from the parent's resolved root. It rejects a
// missing path, a link, a non-directory, a child inside the parent, and a parent
// inside the child, and returns the child's resolved directory.
func ResolveChild(parentDir, entry string) (string, error) {
	joined := filepath.Join(parentDir, filepath.FromSlash(entry))
	if filepath.IsAbs(entry) {
		joined = filepath.Clean(entry)
	}
	info, err := os.Lstat(joined)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", &PathError{Path: entry, Problem: "does not exist", Recovery: "clone or restore the child repository there, or pass the child's real path"}
	case err != nil:
		return "", fmt.Errorf("inspect child %s: %w", entry, err)
	case info.Mode()&fs.ModeSymlink != 0:
		return "", &PathError{Path: entry, Problem: "is a symbolic link", Recovery: "pass the child repository's real directory"}
	case !info.IsDir():
		return "", &PathError{Path: entry, Problem: "is not a directory", Recovery: "pass the child repository's root directory"}
	}
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", fmt.Errorf("resolve child %s: %w", entry, err)
	}
	if within(parentDir, resolved) {
		return "", &PathError{Path: entry, Problem: "is inside the parent repository", Recovery: "keep a child repository outside its parent's worktree"}
	}
	if within(resolved, parentDir) {
		return "", &PathError{Path: entry, Problem: "contains the parent repository", Recovery: "pass a separate repository beside the parent"}
	}
	return resolved, nil
}

// ChildEntry returns the registry entry for a resolved child directory.
func ChildEntry(parentDir, childDir string) (string, error) {
	rel, err := filepath.Rel(parentDir, childDir)
	if err != nil {
		return "", fmt.Errorf("relate child %s to parent %s: %w", childDir, parentDir, err)
	}
	entry := filepath.ToSlash(rel)
	if err := ValidateChildEntry(entry); err != nil {
		return "", &PathError{Path: entry, Problem: err.Error(), Recovery: "pass a child repository on the same filesystem beside the parent"}
	}
	return entry, nil
}

func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

func exists(name string) bool { _, err := os.Lstat(name); return err == nil }

func readContainedOptional(root, rel string) ([]byte, bool, error) {
	access, err := Open(root)
	if err != nil {
		return nil, false, err
	}
	defer access.Close()
	return access.ReadOptional(rel)
}

func regularFile(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.Mode().IsRegular()
}

func hasRegularMatch(pattern string) bool {
	matches, _ := filepath.Glob(pattern)
	return slices.ContainsFunc(matches, regularFile)
}
