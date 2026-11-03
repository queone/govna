package remove

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/queone/govna/internal/audit"
	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/emission"
	"github.com/queone/govna/internal/help"
	"github.com/queone/govna/internal/repository"
)

type Config struct{ Flavor, Stack, RepoName, Child string }

type Route struct{ Path, Kind, Reason string }

type Assessment struct {
	InScope, OutOfScope, Review []Route
	ControlState                *Route
	orphans                     int
}

func Run(args []string, stdout, stderr io.Writer, cwd, programVersion string) int {
	cfg, err := parse(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprint(stderr, Page(programVersion).Render(false))
		return 2
	}
	if repository.IsSource(cwd) {
		fmt.Fprintf(stderr, "rm: a Govna removal AC cannot be created inside the Govna source checkout at %s; run this command from the target repository\n", cwd)
		return 1
	}
	if err := repository.RequireAdopted(cwd); err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	if err := repository.RequireGitWorktree(cwd); err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	preserve, err := audit.ParsePreserve(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	flavor, err := repository.Flavor(cwd, cfg.Flavor)
	if err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	children, err := readChildren(cwd)
	if err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	if children != nil && flavor == canon.Doc {
		fmt.Fprintf(stderr, "rm: %s is present in a DOC repository, and DOC repositories have no child repositories; delete %s and re-run rm\n", repository.ChildRegistryPath, repository.ChildRegistryPath)
		return 1
	}
	if cfg.Child != "" {
		return runChild(cfg.Child, children, flavor, stdout, stderr, cwd, programVersion)
	}
	stack := cfg.Stack
	if flavor == canon.Doc && stack != "" {
		fmt.Fprintln(stderr, "rm: --stack applies only to CODE canon; remove --stack or select --flavor code")
		return 2
	}
	if flavor == canon.Code {
		if stack == "" {
			stack = metadataStack(cwd)
		}
		if stack == "" {
			stack = repository.Stack(cwd)
		}
		if stack == "" {
			fmt.Fprintf(stderr, "rm: could not infer a supported CODE stack from cwd=%s; use %s with --stack\n", cwd, canon.SupportedStackChoices)
			return 1
		}
		canonical, ok := canon.CanonicalStack(stack)
		if !ok {
			fmt.Fprintf(stderr, "rm: unsupported CODE stack %q: use %s\n", stack, canon.SupportedStackChoices)
			return 1
		}
		stack = canonical
	}
	module := ""
	if stack == "Go" {
		module = repository.ModulePath(cwd)
	}
	name := repository.Name(cwd, module, cfg.RepoName)
	files, err := canon.Render(canon.Config{Flavor: flavor, RepoName: name, Stack: stack, ModulePath: module})
	if err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	version := "v" + canon.Version
	stub, reused, err := emission.GuardedPath(cwd, "govna-rm", version, nil)
	if err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	full := filepath.Join(cwd, filepath.FromSlash(stub))
	var existing []byte
	if reused {
		existing, err = os.ReadFile(full)
		if err != nil || !emission.VerifyGuardedBody(existing, emission.RemovalMarkerPrefix) {
			fmt.Fprintf(stderr, "rm: %s has been edited since last emission — delete or rename the emitted file before re-running\n", stub)
			return 1
		}
	}
	assessment, err := classify(cwd, files, preserve, stub)
	if err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	if err := addChildRoutes(&assessment, children, stack); err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	body := []byte(buildAC(stub, programVersion, canon.Version, flavor, stack, assessment))
	return writeStub(stub, full, reused, existing, programVersion, body, stdout, stderr)
}

func writeStub(stub, full string, reused bool, existing []byte, programVersion string, body []byte, stdout, stderr io.Writer) int {
	content := emission.GuardedBody(emission.RemovalMarkerPrefix, programVersion, canon.Version, body)
	if reused && bytes.Equal(existing, content) {
		fmt.Fprintf(stdout, "Wrote %s for review.\n", stub)
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	if err := os.WriteFile(full, content, 0o644); err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Wrote %s for review.\n", stub)
	return 0
}

func readChildren(root string) ([]string, error) {
	access, err := repository.Open(root)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", root, err)
	}
	defer access.Close()
	children, present, err := access.ReadChildren()
	if err != nil || !present {
		return nil, err
	}
	if children == nil {
		children = []string{}
	}
	return children, nil
}

// addChildRoutes rewrites each declared child's AGENTS.md to the orphan note,
// deletes the registry, and keeps every other child file.
func addChildRoutes(a *Assessment, children []string, stack string) error {
	if children == nil {
		return nil
	}
	files, err := canon.RenderChild(stack)
	if err != nil {
		return err
	}
	for _, entry := range children {
		a.InScope = append(a.InScope, Route{entry + "/AGENTS.md", "rewrite to the orphan note", "the child repository loses its parent"})
		for _, file := range files {
			if file.Path != "AGENTS.md" {
				a.OutOfScope = append(a.OutOfScope, Route{entry + "/" + file.Path, "keep", "a child file stays after its parent drops the child"})
			}
		}
	}
	a.InScope = append(a.InScope, Route{repository.ChildRegistryPath, "delete file", "child registry; delete it after every orphan rewrite"})
	a.orphans = len(children)
	sortRoutes(a.InScope)
	sortRoutes(a.OutOfScope)
	return nil
}

// runChild writes a removal AC that drops one declared child from the parent.
func runChild(arg string, children []string, flavor canon.Flavor, stdout, stderr io.Writer, cwd, programVersion string) int {
	if flavor != canon.Code {
		fmt.Fprintln(stderr, "rm: --child applies only to a CODE parent; a DOC repository has no child repositories")
		return 1
	}
	entry, err := declaredChild(cwd, arg, children)
	if err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	version := "v" + canon.Version
	stub, reused, err := emission.GuardedPath(cwd, "govna-rm-child-"+childSlug(entry), version, nil)
	if err != nil {
		fmt.Fprintf(stderr, "rm: %v\n", err)
		return 1
	}
	full := filepath.Join(cwd, filepath.FromSlash(stub))
	var existing []byte
	if reused {
		existing, err = os.ReadFile(full)
		if err != nil || !emission.VerifyGuardedBody(existing, emission.RemovalMarkerPrefix) {
			fmt.Fprintf(stderr, "rm: %s has been edited since last emission — delete or rename the emitted file before re-running\n", stub)
			return 1
		}
	}
	body := []byte(buildChildAC(stub, programVersion, entry))
	return writeStub(stub, full, reused, existing, programVersion, body, stdout, stderr)
}

// declaredChild matches arg against the registry, first literally and then by
// its resolved directory.
func declaredChild(root, arg string, children []string) (string, error) {
	if children == nil {
		return "", fmt.Errorf("%s is absent, so this repository declares no child repositories; check the parent path", repository.ChildRegistryPath)
	}
	literal := strings.TrimSuffix(strings.TrimPrefix(filepath.ToSlash(arg), "./"), "/")
	if slices.Contains(children, literal) {
		return literal, nil
	}
	access, err := repository.Open(root)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", root, err)
	}
	defer access.Close()
	if dir, err := repository.ResolveChild(access.Dir(), arg); err == nil {
		if entry, err := repository.ChildEntry(access.Dir(), dir); err == nil && slices.Contains(children, entry) {
			return entry, nil
		}
	}
	return "", fmt.Errorf("child %s is not declared in %s; pass one of its listed paths", arg, repository.ChildRegistryPath)
}

func childSlug(entry string) string {
	base := strings.ToLower(path.Base(entry))
	var b strings.Builder
	for _, r := range base {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func buildChildAC(stub, programVersion, entry string) string {
	number := ""
	if rest, ok := strings.CutPrefix(filepath.Base(stub), "ac"); ok {
		if before, _, ok := strings.Cut(rest, "-"); ok {
			number = before
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# AC%s Drop Child Repository %s\n\n## Summary\n\nGovna executable v%s created this plan from its embedded governance files (canon v%s). This AC drops the child repository `%s` from this parent: it rewrites the child's `AGENTS.md` to the orphan note and removes the child from `%s`. Every other file stays unchanged.\n\n### Removal Instructions\n\n", number, entry, programVersion, canon.Version, entry, repository.ChildRegistryPath)
	b.WriteString("- Create the orphan note with `govna render --orphan <orphan-scratch>`.\n")
	fmt.Fprintf(&b, "- Copy `<orphan-scratch>/AGENTS.md` to `%s/AGENTS.md`.\n", entry)
	fmt.Fprintf(&b, "- Remove the `%s` line from `%s`.\n", entry, repository.ChildRegistryPath)
	b.WriteString("\n### Routing Decisions\n\n`None` — no review items.\n\n## In Scope\n\n")
	writeRoutes(&b, []Route{
		{entry + "/AGENTS.md", "rewrite to the orphan note", "the child repository loses its parent"},
		{repository.ChildRegistryPath, "remove entry `" + entry + "`", "the parent stops governing the child"},
	})
	b.WriteString("\n## Out Of Scope\n\n- Every other parent and child file.\n\n## Migration findings\n\n- None.\n\n## Acceptance Tests\n\n")
	fmt.Fprintf(&b, "**AT1** [Automated] [Pre-release gate] — Verify `%s/AGENTS.md` matches `govna render --orphan`.\n\n", entry)
	fmt.Fprintf(&b, "**AT2** [Automated] [Pre-release gate] — Verify `%s` no longer lists `%s`.\n", repository.ChildRegistryPath, entry)
	b.WriteString("\n## Status\n\n`PENDING` — removal emission; awaiting explicit Director Audit.\n")
	return b.String()
}

// Page returns the rm help page rendered through the shared help layout.
func Page(version string) help.Page {
	return help.Utility(version,
		help.Section{Title: "Usage", Rows: []help.Row{{Form: "govna rm [options]"}}, Note: "Write an AC that lists which Govna files can be removed and which files\nneed a Director choice. Run from the repository root with no positional\narguments. This command deletes nothing."},
		help.Section{Title: "Options", Rows: []help.Row{
			{Form: "-f, --flavor code|doc", Meaning: "Govna file set (CODE or DOC; default: auto-detect)"},
			{Form: "-s, --stack NAME", Meaning: "CODE stack (default: inferred from manifests)"},
			{Form: "-n, --repo-name NAME", Meaning: "override repo name (default: basename of cwd)"},
			{Form: "-c, --child PATH", Meaning: "write an AC that drops one declared child repository"},
		}},
	)
}

func parse(args []string) (Config, error) {
	var cfg Config
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--flavor":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("rm: -f, --flavor <code|doc> requires a value")
			}
			cfg.Flavor = args[i]
		case "-s", "--stack":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("rm: -s, --stack <name> requires a value")
			}
			cfg.Stack = strings.TrimSpace(args[i])
			if cfg.Stack == "" {
				return cfg, fmt.Errorf("rm: -s, --stack <name> requires a non-empty value")
			}
		case "-n", "--repo-name":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("rm: -n, --repo-name <name> requires a value")
			}
			cfg.RepoName = args[i]
		case "-c", "--child":
			i++
			if i >= len(args) {
				return cfg, fmt.Errorf("rm: -c, --child <path> requires a value")
			}
			cfg.Child = strings.TrimSpace(args[i])
			if cfg.Child == "" {
				return cfg, fmt.Errorf("rm: -c, --child <path> requires a non-empty value")
			}
		default:
			return cfg, fmt.Errorf("rm: no positional arguments accepted; run from the consumer repo root (got: [%s])", args[i])
		}
	}
	if cfg.Flavor != "" && cfg.Flavor != "code" && cfg.Flavor != "doc" {
		return cfg, fmt.Errorf("rm: --flavor must be code or doc, got %q", cfg.Flavor)
	}
	if cfg.Flavor == "doc" && cfg.Stack != "" {
		return cfg, fmt.Errorf("rm: --stack applies only to CODE canon; remove --stack or select --flavor code")
	}
	if cfg.Child != "" && (cfg.Flavor != "" || cfg.Stack != "" || cfg.RepoName != "") {
		return cfg, fmt.Errorf("rm: --child drops one child repository; remove the other options")
	}
	return cfg, nil
}

func metadataStack(root string) string {
	data, _ := os.ReadFile(filepath.Join(root, "govna", "metadata.txt"))
	for line := range strings.SplitSeq(string(data), "\n") {
		if value, ok := strings.CutPrefix(line, "code_stack = "); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func classify(root string, files []canon.File, preserve map[string]bool, eligible string) (Assessment, error) {
	var out Assessment
	current := map[string][]byte{}
	for _, file := range files {
		current[file.Path] = file.Content
	}
	paths := make([]string, 0, len(current))
	for path := range current {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return out, fmt.Errorf("inspect %s: %w", path, err)
		}
		if path == "plan.md" || path == "arch.md" {
			out.OutOfScope = append(out.OutOfScope, Route{path, "keep", "a repository-owned planning file that Govna never manages"})
			continue
		}
		if preserve[path] {
			out.OutOfScope = append(out.OutOfScope, Route{path, "keep", "registered in govna/preserve.txt"})
			continue
		}
		if !info.Mode().IsRegular() {
			out.Review = append(out.Review, Route{path, "ambiguity", "non-regular canon path"})
			continue
		}
		if path == "README.md" || path == "CHANGELOG.md" {
			out.Review = append(out.Review, Route{path, "hybrid", "contains both Govna-managed and repository-owned content"})
			continue
		}
		if _, mixed := canon.Boundary(path); mixed {
			out.Review = append(out.Review, Route{path, "hybrid", "contains both Govna-managed and repository-owned content"})
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return out, fmt.Errorf("read %s: %w", path, err)
		}
		if bytes.Equal(data, current[path]) {
			out.InScope = append(out.InScope, Route{path, "delete file", "matches the current Govna file exactly"})
		} else {
			out.Review = append(out.Review, Route{path, "ambiguity", "Govna-managed file has local edits"})
		}
	}
	claude := filepath.Join(root, "CLAUDE.md")
	if info, err := os.Lstat(claude); err == nil && info.Mode()&os.ModeSymlink != 0 {
		out.InScope = append(out.InScope, Route{"CLAUDE.md", "delete symlink", "govna compatibility link"})
	}
	targetOnly, err := targetOnly(root, current, eligible)
	if err != nil {
		return out, err
	}
	out.OutOfScope = append(out.OutOfScope, targetOnly...)
	sortRoutes(out.InScope)
	sortRoutes(out.OutOfScope)
	sortRoutes(out.Review)
	if info, err := os.Stat(filepath.Join(root, filepath.FromSlash("govna/preserve.txt"))); err == nil && info.Mode().IsRegular() {
		route := Route{"govna/preserve.txt", "delete control state last", "preserve decisions applied before registry removal"}
		out.ControlState = &route
	}
	return out, nil
}

func targetOnly(root string, current map[string][]byte, eligible string) ([]Route, error) {
	var routes []Route
	var walk func(string) error
	walk = func(dir string) error {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("read target directory: %w", err)
		}
		for _, entry := range entries {
			if entry.Name() == ".git" {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if rel == "CLAUDE.md" || rel == "govna/preserve.txt" || rel == repository.ChildRegistryPath || rel == eligible {
				continue
			}
			if info.IsDir() {
				if err := walk(path); err != nil {
					return err
				}
				continue
			}
			if _, ok := current[rel]; ok {
				continue
			}
			routes = append(routes, Route{rel, "keep", "repository-owned file with no matching entry in Govna's current canon"})
		}
		return nil
	}
	if err := walk(root); err != nil {
		return nil, err
	}
	sortRoutes(routes)
	return routes, nil
}

func sortRoutes(routes []Route) {
	sort.Slice(routes, func(i, j int) bool { return routes[i].Path < routes[j].Path })
}

func buildAC(stub, programVersion, canonVersion string, flavor canon.Flavor, stack string, a Assessment) string {
	number := ""
	base := filepath.Base(stub)
	if rest, ok := strings.CutPrefix(base, "ac"); ok {
		if before, _, ok := strings.Cut(rest, "-"); ok {
			number = before
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# AC%s Review Removal of Govna Files\n\n## Summary\n\nGovna executable v%s created this removal plan from its embedded governance files (canon v%s). This AC removes Govna-managed content without deleting repository-owned content. Files needing a choice stay unchanged until the Director decides what to do.\n\n### Removal Instructions\n\n", number, programVersion, canonVersion)
	recipe := "govna render --flavor " + strings.ToLower(string(flavor))
	if flavor == canon.Code {
		recipe += " --stack " + stack
	}
	recipe += " <scratch>"
	if a.orphans > 0 {
		b.WriteString("- Create the orphan note with `govna render --orphan <orphan-scratch>`.\n")
		b.WriteString("- Copy `<orphan-scratch>/AGENTS.md` over each child `AGENTS.md` under `## In Scope`.\n")
	}
	if len(a.Review) == 0 {
		b.WriteString("- Apply each in-scope removal.\n")
	} else {
		fmt.Fprintf(&b, "- Create a temporary copy of the selected Govna files with `%s`.\n", recipe)
		b.WriteString("- Preserve every file under Routing Decisions until the Director resolves it.\n- Resolve every Director choice in chat.\n- Apply each in-scope removal and Director choice.\n")
	}
	b.WriteString("\n### Routing Decisions\n\n")
	if len(a.Review) == 0 {
		b.WriteString("`None` — no review items.\n")
	} else {
		for i, r := range a.Review {
			fmt.Fprintf(&b, "%d. `%s`: %s.\n   - Compare `%s` with `diff -ru <scratch>/%s %s`.\n   - Choose what to remove from `%s`: only its Govna-managed section, nothing, or the whole file.\n", i+1, r.Path, r.Reason, r.Path, r.Path, r.Path, r.Path)
		}
	}
	b.WriteString("\n## In Scope\n\n")
	writeRoutes(&b, a.InScope)
	if a.ControlState != nil {
		writeRoutes(&b, []Route{*a.ControlState})
	}
	b.WriteString("\n## Out Of Scope\n\n")
	writeRoutes(&b, a.OutOfScope)
	b.WriteString("\n## Migration findings\n\n- None.\n\n## Acceptance Tests\n\n**AT1** [Automated] [Pre-release gate] — Verify every resolved removal target under `## In Scope` is absent.\n\n**AT2** [Manual] [Pre-release gate] — Verify every file under Routing Decisions matches its Director-resolved action.\n")
	if a.ControlState != nil {
		b.WriteString("\n**AT3** [Automated] [Pre-release gate] — Verify every keep-local choice is applied before the final removal of `govna/preserve.txt`.\n")
	}
	b.WriteString("\n## Status\n\n`PENDING` — removal emission; awaiting explicit Director Audit.\n")
	return b.String()
}

func writeRoutes(b *strings.Builder, routes []Route) {
	if len(routes) == 0 {
		b.WriteString("- None.\n")
		return
	}
	for _, r := range routes {
		fmt.Fprintf(b, "- `%s` — %s; %s.\n", r.Path, r.Kind, r.Reason)
	}
}
