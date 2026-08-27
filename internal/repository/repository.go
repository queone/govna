package repository

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/usererr"
)

func Flavor(root, explicit string) (canon.Flavor, error) {
	if explicit == "code" {
		return canon.Code, nil
	}
	if explicit == "doc" {
		return canon.Doc, nil
	}
	metadata, err := os.ReadFile(filepath.Join(root, "govna", "metadata.txt"))
	if err == nil {
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
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %v", filepath.Join(root, "govna", "metadata.txt"), err)
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
	agents := filepath.Join(root, "AGENTS.md")
	info, err := os.Stat(agents)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("AGENTS.md must be a readable regular file")
	}
	if _, err := os.ReadFile(agents); err != nil {
		return fmt.Errorf("read AGENTS.md: %w", err)
	}
	for _, name := range []string{"govna/ac-template.md", "govna/release.md", "govna/build-release.md"} {
		if exists(filepath.Join(root, filepath.FromSlash(name))) {
			return nil
		}
	}
	data, _ := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
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
func exists(name string) bool { _, err := os.Lstat(name); return err == nil }

func regularFile(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.Mode().IsRegular()
}

func hasRegularMatch(pattern string) bool {
	matches, _ := filepath.Glob(pattern)
	return slices.ContainsFunc(matches, regularFile)
}
