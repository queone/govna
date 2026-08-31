package canon

import (
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/queone/govna/internal/usererr"
)

const Version = "0.48.0"

const SupportedStackChoices = "Go, Rust, Swift, or Terraform"

//go:embed all:assets
var assets embed.FS

type Flavor string

const (
	Code Flavor = "CODE"
	Doc  Flavor = "DOC"
)

type Config struct {
	Flavor     Flavor
	RepoName   string
	Stack      string
	ModulePath string
}

type File struct {
	Path    string
	Content []byte
}

var boundaries = map[string]string{
	"AGENTS.md":                       "## Project Rules",
	"govna/build-release.md":          "## Project Practices",
	"govna/development-guidelines.md": "## Project Practices",
	"govna/editing-guidelines.md":     "## Project Practices",
}

// Boundary returns the registered mixed-content boundary for path.
func Boundary(path string) (string, bool) {
	boundary, ok := boundaries[path]
	return boundary, ok
}

// ComparisonRegion returns the canon-owned comparison region for path.
func ComparisonRegion(path string, content []byte) ([]byte, bool) {
	boundary, mixed := boundaries[path]
	if !mixed {
		return content, true
	}
	for offset := 0; offset <= len(content); {
		end := strings.IndexByte(string(content[offset:]), '\n')
		if end < 0 {
			end = len(content) - offset
		}
		line := strings.TrimSuffix(string(content[offset:offset+end]), "\r")
		if line == boundary {
			return content[:offset], true
		}
		if offset+end == len(content) {
			break
		}
		offset += end + 1
	}
	return nil, false
}

// ProtectedRegion returns the boundary heading through EOF for a mixed file.
func ProtectedRegion(path string, content []byte) ([]byte, bool) {
	boundary, mixed := boundaries[path]
	if !mixed {
		return nil, false
	}
	needle := []byte(boundary)
	for offset := 0; offset <= len(content); {
		end := strings.IndexByte(string(content[offset:]), '\n')
		if end < 0 {
			end = len(content) - offset
		}
		line := content[offset : offset+end]
		line = []byte(strings.TrimSuffix(string(line), "\r"))
		if string(line) == string(needle) {
			return content[offset:], true
		}
		if offset+end == len(content) {
			break
		}
		offset += end + 1
	}
	return nil, false
}

// Stacks returns every registered CODE stack in stable order.
func Stacks() []string {
	return []string{"Go", "Rust", "Swift", "Terraform"}
}

func Render(cfg Config) ([]File, error) {
	stack := ""
	if cfg.Flavor == Code {
		var ok bool
		stack, ok = CanonicalStack(cfg.Stack)
		if !ok {
			return nil, fmt.Errorf("unsupported CODE stack %q: use %s", cfg.Stack, SupportedStackChoices)
		}
	}
	modulePath := cfg.ModulePath
	if modulePath == "" {
		modulePath = cfg.RepoName
	}
	values := map[string]string{
		"{{REPO_NAME}}":                    cfg.RepoName,
		"{{STACK_OR_PLATFORM}}":            fallback(stack, "TBD"),
		"{{MODULE_PATH}}":                  modulePath,
		"{{CANON_VERSION}}":                "v" + Version,
		"{{CODE_STACK}}":                   fallback(stack, "TBD"),
		"{{STACK_BUILD_RELEASE_GUIDANCE}}": "",
	}
	if stack == "Rust" {
		raw, err := fs.ReadFile(assets, "assets/stack-build-release/rust.md")
		if err != nil {
			return nil, usererr.Errorf("Govna could not load the Rust build and release guidance: %w", err)
		}
		values["{{STACK_BUILD_RELEASE_GUIDANCE}}"] = strings.TrimSpace(string(raw))
	}
	out := map[string][]byte{}
	base, err := renderAsset("assets/base/AGENTS.md.tmpl", values)
	if err != nil {
		return nil, err
	}
	out["AGENTS.md"] = base
	prefix := "assets/overlays/code/files/"
	if cfg.Flavor == Doc {
		prefix = "assets/overlays/doc/files/"
	}
	if err := walkOverlay(prefix, values, out); err != nil {
		return nil, err
	}
	if cfg.Flavor == Code {
		stackPrefix := "assets/overlays/code/stacks/" + strings.ToLower(stack) + "/"
		if err := walkOverlay(stackPrefix, values, out); err != nil {
			return nil, err
		}
		if block, ok := rawStack("stack-ignores", strings.ToLower(stack), "txt"); ok {
			out[".gitignore"] = append(append(out[".gitignore"], '\n'), block...)
		}
		if block, ok := rawStack("stack-guidelines", strings.ToLower(stack), "md"); ok {
			content, err := insertBeforeBoundary(string(out["govna/development-guidelines.md"]), strings.TrimSpace(string(block)))
			if err != nil {
				return nil, err
			}
			out["govna/development-guidelines.md"] = []byte(content)
		}
	}
	baseline, err := baseline(out)
	if err != nil {
		return nil, err
	}
	out["govna/canon-baseline.txt"] = []byte(baseline)
	paths := make([]string, 0, len(out))
	for path := range out {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	files := make([]File, 0, len(paths))
	for _, path := range paths {
		files = append(files, File{Path: path, Content: out[path]})
	}
	return files, nil
}

func walkOverlay(prefix string, values map[string]string, out map[string][]byte) error {
	root := strings.TrimSuffix(prefix, "/")
	if _, err := fs.Stat(assets, root); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return fs.WalkDir(assets, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(path, prefix)
		if rel == path {
			return nil
		}
		rel = strings.TrimSuffix(rel, ".tmpl")
		content, err := renderAsset(path, values)
		if err != nil {
			return err
		}
		out[rel] = content
		return nil
	})
}

func renderAsset(path string, values map[string]string) ([]byte, error) {
	data, err := fs.ReadFile(assets, path)
	if err != nil {
		return nil, fmt.Errorf("read template file %s: %w", path, err)
	}
	text := string(data)
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		text = strings.ReplaceAll(text, key, values[key])
	}
	return []byte(text), nil
}

func rawStack(group, stack, extension string) ([]byte, bool) {
	if group == "stack-guidelines" && stack != "go" && stack != "rust" && stack != "swift" {
		return nil, false
	}
	if group == "stack-ignores" && stack != "go" && stack != "rust" && stack != "swift" && stack != "terraform" {
		return nil, false
	}
	data, err := fs.ReadFile(assets, "assets/"+group+"/"+stack+"."+extension)
	return data, err == nil
}

func insertBeforeBoundary(content, block string) (string, error) {
	const marker = "\n## Project Practices\n"
	index := strings.Index(content, marker)
	if index < 0 {
		return "", usererr.Errorf("Govna could not add stack guidance because the ## Project Practices boundary is missing from govna/development-guidelines.md")
	}
	return strings.TrimRight(content[:index+1], "\n") + "\n\n" + block + "\n\n" + content[index+1:], nil
}

func baseline(files map[string][]byte) (string, error) {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	var result strings.Builder
	fmt.Fprintf(&result, "govna-canon-baseline-v1\ncanon_version = v%s\n", Version)
	for _, path := range paths {
		content := files[path]
		scope := "full"
		region := content
		if boundary, ok := boundaries[path]; ok {
			var found bool
			region, found = ComparisonRegion(path, content)
			if !found {
				return "", usererr.Errorf("Govna could not build the baseline for %s because its required boundary %q is missing", path, boundary)
			}
			scope = "before:" + boundary
		}
		hash := sha256.Sum256(region)
		fmt.Fprintf(&result, "%s\t%s\t%x\n", path, scope, hash)
	}
	return result.String(), nil
}

func CanonicalStack(stack string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(stack)) {
	case "go", "golang":
		return "Go", true
	case "rust":
		return "Rust", true
	case "swift":
		return "Swift", true
	case "terraform":
		return "Terraform", true
	default:
		return "", false
	}
}

func fallback(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
