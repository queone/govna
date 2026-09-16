package apply

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/emission"
	"github.com/queone/govna/internal/repository"
)

type Config struct {
	Flavor, Stack, RepoName, ModulePath string
	InitGit                             bool
}
type Outcome struct{ Path, Label string }
type Command func(string, ...string) ([]byte, error)
type assessment struct {
	shape, risk       string
	code, doc         int
	existingArtifacts []string
}

func Run(args []string, stdout, stderr io.Writer, cwd, programVersion string, command Command) int {
	cfg, err := parse(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	if repository.IsSource(cwd) {
		fmt.Fprintf(stderr, "apply: Govna files cannot be added inside the Govna source checkout at %s; run this command from the target repository\n", cwd)
		return 1
	}
	access, err := repository.Open(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "apply: open target %s: %v\n", cwd, err)
		return 1
	}
	defer access.Close()
	a, err := assess(cwd, access)
	if err != nil {
		fmt.Fprintf(stderr, "apply: scan target repo: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "mode: apply")
	fmt.Fprintf(stdout, "target: %s\n", cwd)
	fmt.Fprintf(stdout, "repository type: %s\n", a.shape)
	fmt.Fprintf(stdout, "type evidence: CODE score=%d DOC score=%d\n", a.code, a.doc)
	existing := "none"
	if len(a.existingArtifacts) > 0 {
		existing = strings.Join(a.existingArtifacts, ", ")
	}
	fmt.Fprintf(stdout, "existing files: %s\n", existing)
	fmt.Fprintf(stdout, "risk of replacing content: %s\n", a.risk)
	flavor, err := repository.Flavor(cwd, cfg.Flavor)
	if err != nil {
		fmt.Fprintf(stderr, "apply: %v\n", err)
		return 1
	}
	stack := cfg.Stack
	module := cfg.ModulePath
	if flavor == canon.Doc && (stack != "" || module != "") {
		fmt.Fprintln(stderr, "apply: --stack and --module-path cannot be used for a DOC repository")
		return 2
	}
	if flavor == canon.Code {
		if stack == "" {
			stack = repository.Stack(cwd)
		}
		if stack == "" {
			fmt.Fprintf(stderr, "apply: could not infer a supported CODE stack from cwd=%s; use %s with --stack\n", cwd, canon.SupportedStackChoices)
			return 1
		}
		canonical, ok := canon.CanonicalStack(stack)
		if !ok {
			fmt.Fprintf(stderr, "apply: unsupported CODE stack %q: use %s\n", stack, canon.SupportedStackChoices)
			return 1
		}
		stack = canonical
		if stack == "Go" && module == "" {
			module = repository.ModulePath(cwd)
		}
		if stack != "Go" && module != "" {
			fmt.Fprintln(stderr, "apply: --module-path applies only to Go CODE canon")
			return 2
		}
	}
	if present(access, "AGENTS.md") || present(access, "CLAUDE.md") {
		fmt.Fprintln(stderr, "existing governance files detected; Govna will report whether each file is written, merged, or preserved")
	}
	name := repository.Name(cwd, module, cfg.RepoName)
	files, err := canon.Render(canon.Config{Flavor: flavor, RepoName: name, Stack: stack, ModulePath: module})
	if err != nil {
		fmt.Fprintf(stderr, "apply: %v\n", err)
		return 1
	}
	for _, file := range files {
		if err := access.Preflight(file.Path, false); err != nil {
			fmt.Fprintf(stderr, "apply: check destination %s: %v\n", file.Path, err)
			return 1
		}
	}
	if err := access.Preflight("CLAUDE.md", true); err != nil {
		fmt.Fprintf(stderr, "apply: check destination CLAUDE.md: %v\n", err)
		return 1
	}
	outcomes := []Outcome{}
	for _, file := range files {
		label := "written"
		if present(access, file.Path) {
			switch file.Path {
			case "README.md", "CHANGELOG.md", "arch.md", "plan.md":
				label = "kept existing file"
				fmt.Fprintf(stdout, "kept %s (existing file)\n", file.Path)
				outcomes = append(outcomes, Outcome{file.Path, label})
				continue
			}
			if boundary, mixed := canon.Boundary(file.Path); mixed {
				old, e := access.ReadFile(file.Path)
				if e != nil {
					fmt.Fprintf(stderr, "apply: read %s: %v\n", file.Path, e)
					return 1
				}
				merged, ok := merge(old, file.Content, file.Path)
				if ok {
					file.Content = merged
					label = "updated Govna-managed section; kept repository-owned section"
				} else if file.Path == "govna/build-release.md" {
					label = "kept existing file; add the missing Govna/local boundary and merge the Govna-managed section manually"
					fmt.Fprintf(stderr, "warning: %s has no `%s` boundary; kept the existing file; add the named boundary and merge the Govna-managed section manually\n", file.Path, boundary)
					outcomes = append(outcomes, Outcome{file.Path, label})
					continue
				} else {
					label = "replaced whole file because the Govna/local boundary was missing"
					fmt.Fprintf(stderr, "warning: %s has no `%s` boundary; replacing the whole file because the named Govna/local boundary is missing\n", file.Path, boundary)
				}
			}
		}
		perm := os.FileMode(0o644)
		if path.Ext(file.Path) == ".sh" {
			perm = 0o755
		}
		if err := access.WriteFile(file.Path, file.Content, perm); err != nil {
			fmt.Fprintf(stderr, "apply: write %s: %v\n", file.Path, err)
			return 1
		}
		fmt.Fprintf(stdout, "wrote %s (Govna-managed file)\n", file.Path)
		outcomes = append(outcomes, Outcome{file.Path, label})
	}
	symlink := "created"
	if info, err := access.Lstat("CLAUDE.md"); err == nil && info.Mode().IsRegular() {
		symlink = "preserved"
		fmt.Fprintln(stderr, "warning: CLAUDE.md exists as a regular file; expected symlink to AGENTS.md — delete the file and re-run to create the symlink")
	} else {
		_ = access.Remove("CLAUDE.md")
		if err := access.Symlink("AGENTS.md", "CLAUDE.md"); err != nil {
			return fail(stderr, err)
		}
		fmt.Fprintln(stdout, "symlink CLAUDE.md -> AGENTS.md")
	}
	n, err := emission.Next(cwd, nil)
	if err != nil {
		return fail(stderr, err)
	}
	rel := fmt.Sprintf("govna/ac%d-govna-apply.md", n)
	body := adoption(n, name, string(flavor), programVersion, outcomes, symlink)
	if err := access.WriteFile(rel, []byte(body), 0o644); err != nil {
		fmt.Fprintf(stderr, "apply: write %s: %v\n", rel, err)
		return 1
	}
	fmt.Fprintf(stdout, "wrote %s (review AC)\n", rel)
	if cfg.InitGit {
		if exists(filepath.Join(cwd, ".git")) {
			fmt.Fprintln(stdout, "skip git init (git repo already present)")
		} else {
			if command == nil {
				command = func(name string, args ...string) ([]byte, error) { return exec.Command(name, args...).CombinedOutput() }
			}
			fmt.Fprintf(stdout, "exec git init -b main %s\n", cwd)
			if out, err := command("git", "init", "-b", "main", cwd); err != nil {
				fmt.Fprintf(stderr, "apply: git init %s: %s\n", cwd, strings.TrimSpace(string(out)))
				return 1
			}
		}
	}
	return 0
}

func assess(root string, access *repository.Access) (assessment, error) {
	a := assessment{shape: "empty", risk: "low"}
	hasSource, hasManifest, hasLayout, hasDocMarker := false, false, false, false
	files := []string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		files = append(files, rel)
		ext := strings.ToLower(filepath.Ext(rel))
		base := filepath.Base(rel)
		switch ext {
		case ".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".rs", ".java", ".kt", ".swift", ".c", ".cc", ".cpp", ".cs":
			a.code++
			hasSource = true
		case ".md", ".mdx":
			a.doc++
		}
		switch base {
		case "go.mod", "package.json", "pyproject.toml", "Cargo.toml", "pom.xml", "build.gradle", "Makefile", "Dockerfile":
			a.code += 3
			hasManifest = true
		case "mkdocs.yml", "mkdocs.yaml":
			a.doc += 3
			hasDocMarker = true
		case "README.md", "AGENTS.md", "CLAUDE.md", "arch.md", "plan.md":
			a.doc++
		}
		top, _, _ := strings.Cut(rel, "/")
		if top == "cmd" || top == "internal" || top == "pkg" || top == "src" {
			hasLayout = true
		}
		return nil
	})
	if err != nil {
		return a, err
	}
	if len(files) > 0 {
		switch {
		case (hasManifest || hasLayout) && hasSource:
			a.shape = "likely CODE"
		case hasDocMarker && !hasSource && !hasManifest:
			a.shape = "likely DOC"
		case a.code > a.doc && a.code > 0:
			a.shape = "likely CODE"
		case a.doc > a.code && a.doc > 0:
			a.shape = "likely DOC"
		case a.code > 0 && a.doc > 0:
			a.shape = "mixed"
		default:
			a.shape = "unclear"
		}
	}
	expected := []string{"AGENTS.md", "CLAUDE.md"}
	if a.shape == "likely CODE" {
		expected = append(expected, "README.md", "arch.md", "plan.md", "CHANGELOG.md", "govna/README.md", "govna/development-cycle.md", "govna/ac-template.md", "govna/build-release.md", "govna/metadata.txt")
	}
	if a.shape == "likely DOC" {
		expected = append(expected, "plan.md", "govna/metadata.txt")
	}
	nonempty := 0
	for _, rel := range expected {
		info, err := access.Lstat(rel)
		if err != nil {
			continue
		}
		a.existingArtifacts = append(a.existingArtifacts, rel)
		if info.Mode().IsRegular() && info.Size() > 0 {
			nonempty++
		}
	}
	if nonempty >= 3 {
		a.risk = "high"
	} else if nonempty > 0 {
		a.risk = "medium"
	}
	return a, nil
}

func parse(args []string) (Config, error) {
	var c Config
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--flavor":
			if i++; i >= len(args) {
				return c, fmt.Errorf("apply: -f, --flavor <code|doc> requires a value")
			}
			c.Flavor = args[i]
		case "-s", "--stack":
			if i++; i >= len(args) {
				return c, fmt.Errorf("apply: -s, --stack <name> requires a value")
			}
			c.Stack = strings.TrimSpace(args[i])
			if c.Stack == "" {
				return c, fmt.Errorf("apply: -s, --stack <name> requires a non-empty value")
			}
		case "-n", "--repo-name":
			if i++; i >= len(args) {
				return c, fmt.Errorf("apply: -n, --repo-name <name> requires a value")
			}
			c.RepoName = args[i]
		case "-m", "--module-path":
			if i++; i >= len(args) {
				return c, fmt.Errorf("apply: -m, --module-path <path> requires a value")
			}
			c.ModulePath = args[i]
		case "-g", "--init-git":
			c.InitGit = true
		default:
			return c, fmt.Errorf("apply: no positional arguments accepted; run from the target repo root (got: [%s])", args[i])
		}
	}
	if c.Flavor != "" && c.Flavor != "code" && c.Flavor != "doc" {
		return c, fmt.Errorf("apply: --flavor must be code or doc, got %q", c.Flavor)
	}
	return c, nil
}

// merge joins the rendered canon zone above the registered boundary of path with the
// existing file's boundary line and every byte after it, in either line-ending style.
func merge(old, fresh []byte, path string) ([]byte, bool) {
	head, ok := canon.ComparisonRegion(path, fresh)
	if !ok {
		return nil, false
	}
	tail, ok := canon.ProtectedRegion(path, old)
	if !ok {
		return nil, false
	}
	return append(append([]byte{}, head...), tail...), true
}
func adoption(n int, name, flavor, programVersion string, out []Outcome, symlink string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# AC%d Review Files Added by Govna\n\nGovna executable v%s added its embedded governance files (canon v%s) for the %s repository %s.\n\n## Summary\n\nGovna executable v%s added its embedded governance files (canon v%s). The list below records whether each file was written, merged, or preserved.\n\n## In Scope\n\nFiles Govna processed:\n\n", n, programVersion, canon.Version, flavor, name, programVersion, canon.Version)
	for _, o := range out {
		fmt.Fprintf(&b, "- `%s` (%s)\n", o.Path, o.Label)
	}
	if symlink == "created" {
		b.WriteString("- `CLAUDE.md` (agent alias link)\n")
	} else {
		b.WriteString("- `CLAUDE.md` (existing regular file preserved — not a symlink, see warning)\n")
	}
	b.WriteString("\n## Out Of Scope\n\n- Files not listed above.\n\n## Migration findings\n\n- None.\n\n## Acceptance Tests\n\n**AT1** [Manual] [Pre-release gate] — Verify AGENTS.md reflects the repository's actual practices.\n\n**AT2** [Manual] [Pre-release gate] — Verify govna/roles.md reflects the repository's delivery model (Operator + Director).\n\n")
	if symlink == "created" {
		b.WriteString("**AT3** [Manual] [Pre-release gate] — Verify CLAUDE.md is a symlink to AGENTS.md.\n\n")
	} else {
		b.WriteString("**AT3** [Manual] [Pre-release gate] — Verify CLAUDE.md remains the existing regular file instead of a symlink to AGENTS.md.\n\n")
	}
	b.WriteString("## Status\n\n`PENDING` — apply emission; awaiting explicit Director Audit.\n")
	return b.String()
}
func exists(p string) bool                               { _, e := os.Lstat(p); return e == nil }
func present(access *repository.Access, rel string) bool { _, e := access.Lstat(rel); return e == nil }
func fail(w io.Writer, e error) int                      { fmt.Fprintf(w, "apply: %v\n", e); return 1 }
