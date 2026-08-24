package canon

import (
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

const Version = "0.29.0"

//go:embed assets
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

func Render(cfg Config) ([]File, error) {
	stack := ""
	if cfg.Flavor == Code {
		var ok bool
		stack, ok = CanonicalStack(cfg.Stack)
		if !ok {
			return nil, fmt.Errorf("unsupported CODE stack %q: use Go, Rust, Swift, Terraform, Node, Python, or Java", cfg.Stack)
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
			return nil, fmt.Errorf("compose stack build/release guidance: %w", err)
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
		return "", fmt.Errorf("compose stack guidelines: ## Project Practices boundary not found")
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
			marker := []byte("\n" + boundary + "\n")
			index := strings.Index(string(content), string(marker))
			if index < 0 {
				return "", fmt.Errorf("render baseline: %s is missing registered boundary %q", path, boundary)
			}
			scope = "before:" + boundary
			region = content[:index+1]
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
	case "node":
		return "Node", true
	case "python":
		return "Python", true
	case "java":
		return "Java", true
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
