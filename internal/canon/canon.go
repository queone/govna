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

const Version = "0.106.0"

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
	// Children reports that the repository declares at least one child
	// repository, so the render includes the child profile text.
	Children bool
	// Utilities reports that the repository has, or is about to add, an
	// installable command-line utility, so the render includes those rules.
	Utilities bool
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
	if err := profileValues(values, stack, cfg.Flavor == Code && cfg.Children, cfg.Utilities); err != nil {
		return nil, err
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

const (
	childPointerAsset = "assets/child/pointer-AGENTS.md.tmpl"
	childOrphanAsset  = "assets/child/orphan-AGENTS.md.tmpl"
)

// ChildTooling returns the build tooling a child repository on stack receives:
// every file in the stack's build overlay, in byte order.
func ChildTooling(stack string) ([]string, error) {
	canonical, ok := CanonicalStack(stack)
	if !ok {
		return nil, fmt.Errorf("unsupported CODE stack %q: use %s", stack, SupportedStackChoices)
	}
	out := map[string][]byte{}
	if err := walkOverlay("assets/overlays/code/stacks/"+strings.ToLower(canonical)+"/", nil, out); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(out))
	for path := range out {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

// RenderChild returns the child files for a CODE stack in byte order: the
// pointer AGENTS.md, the stack's build tooling, .gitignore, and CHANGELOG.md.
func RenderChild(stack string) ([]File, error) {
	tooling, err := ChildTooling(stack)
	if err != nil {
		return nil, err
	}
	files, err := Render(Config{Flavor: Code, RepoName: "child", Stack: stack, ModulePath: "child"})
	if err != nil {
		return nil, err
	}
	keep := map[string]bool{".gitignore": true, "CHANGELOG.md": true}
	for _, path := range tooling {
		keep[path] = true
	}
	pointer, err := renderAsset(childPointerAsset, nil)
	if err != nil {
		return nil, err
	}
	out := []File{{Path: "AGENTS.md", Content: pointer}}
	for _, file := range files {
		if keep[file.Path] {
			out = append(out, file)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// RenderOrphan returns the orphan note that replaces a dropped child's pointer.
func RenderOrphan() ([]File, error) {
	orphan, err := renderAsset(childOrphanAsset, nil)
	if err != nil {
		return nil, err
	}
	return []File{{Path: "AGENTS.md", Content: orphan}}, nil
}

// IsChildAgentFile reports whether content is exactly the canon child pointer or orphan note.
func IsChildAgentFile(content []byte) bool {
	for _, asset := range []string{childPointerAsset, childOrphanAsset} {
		if data, err := fs.ReadFile(assets, asset); err == nil && string(data) == string(content) {
			return true
		}
	}
	return false
}

// ChildProfilePaths returns the parent files whose canon text changes when
// the parent declares its first child or drops its last one, in byte order.
func ChildProfilePaths() []string {
	return []string{"AGENTS.md", "govna/audit.md", "govna/build-release.md", "govna/development-cycle.md"}
}

// profileBlock names one placeholder and the profile asset that fills it.
// A section block ends with a blank line so the next heading stays separated.
type profileBlock struct {
	key, asset string
	section    bool
}

var childBlocks = []profileBlock{
	{"{{CHILD_AGENTS_SECTION}}", "children/agents.md", true},
	{"{{CHILD_CYCLE_SECTION}}", "children/development-cycle.md", true},
	{"{{CHILD_SNAPSHOT_RULE}}", "children/snapshot-rule.md", false},
	{"{{CHILD_PRESERVE_RULE}}", "children/audit.md", true},
	{"{{CHILD_AUDIT_CLASSIFICATION_ROW}}", "children/audit-classification-row.md", false},
	{"{{CHILD_RELEASES_SECTION}}", "children/build-release.md", true},
}

// utilityBlocks render for a repository with an installable utility.
var utilityBlocks = []profileBlock{
	{"{{UTILITY_CLI_SECTION}}", "utilities/development-guidelines.md", true},
	{"{{UTILITY_VERSIONS_SECTION}}", "utilities/build-release.md", true},
}

// flagBlocks render for every stack that can have a utility.
var flagBlocks = []profileBlock{
	{"{{CLI_FLAG_RULES}}", "utilities/flag-rules.md", false},
}

// auditRequestBlocks render for a stack that can have a utility while the
// repository has none: the rule asks for the audit that adds the rules.
var auditRequestBlocks = []profileBlock{
	{"{{UTILITY_AUDIT_RULE}}", "utilities/audit-request-rule.md", false},
}

// CanHaveUtilities reports whether stack can produce an installable utility.
func CanHaveUtilities(stack string) bool {
	canonical, ok := CanonicalStack(stack)
	return ok && canonical != "Terraform"
}

var rustBlocks = []profileBlock{
	{"{{RUST_AGENTS_RULES}}", "rust/agents.md", false},
	{"{{RUST_PREP_TOKEN_LINE}}", "rust/build-release-token.md", false},
	{"{{RUST_PREP_EVIDENCE_LINES}}", "rust/build-release-evidence.md", false},
}

// profileValues fills every profile placeholder: with its asset when the
// repository's profile needs the text, and with nothing otherwise.
func profileValues(values map[string]string, stack string, children, utilities bool) error {
	fill := func(blocks []profileBlock, include bool) error {
		for _, block := range blocks {
			values[block.key] = ""
			if !include {
				continue
			}
			raw, err := fs.ReadFile(assets, "assets/profile/"+block.asset)
			if err != nil {
				return usererr.Errorf("Govna could not load the profile text %s: %w", block.asset, err)
			}
			text := string(raw)
			if block.section {
				text = strings.TrimRight(text, "\n") + "\n\n"
			}
			values[block.key] = text
		}
		return nil
	}
	if err := fill(childBlocks, children); err != nil {
		return err
	}
	if children {
		// The release block carries only the override and pin commands of
		// the parent's own stack.
		raw, err := fs.ReadFile(assets, "assets/profile/children/pins/"+strings.ToLower(stack)+".md")
		if err != nil {
			return usererr.Errorf("Govna could not load the child pin commands for the %s stack: %w", stack, err)
		}
		const key = "{{CHILD_RELEASES_SECTION}}"
		values[key] = strings.Replace(values[key], "{{CHILD_PIN_COMMANDS}}\n", string(raw), 1)
	}
	capable := CanHaveUtilities(stack)
	if err := fill(utilityBlocks, capable && utilities); err != nil {
		return err
	}
	if err := fill(flagBlocks, capable); err != nil {
		return err
	}
	if err := fill(auditRequestBlocks, capable && !utilities); err != nil {
		return err
	}
	return fill(rustBlocks, stack == "Rust")
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
