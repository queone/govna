package remove

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/queone/govna/internal/audit"
	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/emission"
	"github.com/queone/govna/internal/repository"
)

type Config struct{ Flavor, Stack, RepoName string }

type Route struct{ Path, Kind, Reason string }

type Assessment struct {
	InScope, OutOfScope, Review []Route
	ControlState                *Route
}

func Run(args []string, stdout, stderr io.Writer, cwd, programVersion string) int {
	cfg, err := parse(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		fmt.Fprint(stderr, Help())
		return 2
	}
	if repository.IsSource(cwd) {
		fmt.Fprintf(stderr, "rm: target %s looks like a govna checkout — rm is for adopted repos, not the govna source\n", cwd)
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
		fmt.Fprintf(stderr, "rm: infer flavor from cwd: %v (use --flavor to override)\n", err)
		return 1
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
			fmt.Fprintf(stderr, "rm: could not infer CODE stack from cwd=%s; pass --stack to override\n", cwd)
			return 1
		}
		canonical, ok := canon.CanonicalStack(stack)
		if !ok {
			fmt.Fprintf(stderr, "rm: unsupported CODE stack %q\n", stack)
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
	body := []byte(buildAC(stub, programVersion, canon.Version, flavor, stack, assessment))
	content := emission.GuardedBody(emission.RemovalMarkerPrefix, programVersion, canon.Version, body)
	if reused && bytes.Equal(existing, content) {
		fmt.Fprintf(stdout, "wrote %s\n", stub)
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
	fmt.Fprintf(stdout, "wrote %s\n", stub)
	return 0
}

func Help() string {
	return "Usage: govna rm [flags]\n\n" +
		"Emit a Director-reviewed cleanup AC for removing govna canon from an\n" +
		"adopted repo. Run from the consumer repo root (no positional arguments).\n" +
		"Deletes nothing itself.\n\nFlags:\n" +
		"  -f, --flavor code|doc      overlay flavor (default: auto-detect)\n" +
		"  -s, --stack <name>         CODE stack (default: inferred from manifests)\n" +
		"  -n, --repo-name <name>     override repo name (default: basename of cwd)\n" +
		"  -h, --help                 show this help\n"
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
			out.OutOfScope = append(out.OutOfScope, Route{path, "keep", "repo-owned govna-adjacent content"})
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
			out.Review = append(out.Review, Route{path, "hybrid", "mixed canon-shape and consumer content"})
			continue
		}
		if _, mixed := canon.Boundary(path); mixed {
			out.Review = append(out.Review, Route{path, "hybrid", "mixed canon-shape and consumer content"})
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return out, fmt.Errorf("read %s: %w", path, err)
		}
		if bytes.Equal(data, current[path]) {
			out.InScope = append(out.InScope, Route{path, "delete file", "byte-equal govna canon"})
		} else {
			out.Review = append(out.Review, Route{path, "ambiguity", "consumer-edited canon file"})
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
			if rel == "CLAUDE.md" || rel == "govna/preserve.txt" || rel == eligible {
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
			routes = append(routes, Route{rel, "keep", "target-only repo-owned file"})
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
	fmt.Fprintf(&b, "# AC%s Govna Removal from v%s\n\n## Summary\n\nThis removal AC was emitted by govna executable v%s with embedded canon v%s. It removes Govna canon from this consumer repository without deleting consumer-owned content. Director-resolved routing protects every review path.\n\n### Removal Instructions\n\n", number, canonVersion, programVersion, canonVersion)
	recipe := "govna render --flavor " + strings.ToLower(string(flavor))
	if flavor == canon.Code {
		recipe += " --stack " + stack
	}
	recipe += " <scratch>"
	if len(a.Review) == 0 {
		b.WriteString("- Apply each in-scope route.\n")
	} else {
		fmt.Fprintf(&b, "- Render the selected canon into `<scratch>` with `%s`.\n", recipe)
		b.WriteString("- Preserve every routing-pending path until its route is resolved.\n- Resolve every routing decision in chat.\n- Apply each in-scope route and each Director-resolved review route.\n")
	}
	b.WriteString("\n### Routing Decisions\n\n")
	if len(a.Review) == 0 {
		b.WriteString("`None` — no review items.\n")
	} else {
		for i, r := range a.Review {
			fmt.Fprintf(&b, "%d. `%s` is %s.\n   - Compare `%s` with `diff -ru <scratch>/%s %s`.\n   - Choose one route for `%s`: canon-only deletion, full preservation, or full deletion.\n", i+1, r.Path, r.Reason, r.Path, r.Path, r.Path, r.Path)
		}
	}
	b.WriteString("\n## In Scope\n\n")
	writeRoutes(&b, a.InScope)
	if a.ControlState != nil {
		writeRoutes(&b, []Route{*a.ControlState})
	}
	b.WriteString("\n## Out Of Scope\n\n")
	writeRoutes(&b, a.OutOfScope)
	b.WriteString("\n## Migration findings\n\n- None.\n\n## Acceptance Tests\n\n**AT1** [Automated] [Pre-release gate] — Verify every resolved removal target under `## In Scope` is absent.\n\n**AT2** [Manual] [Pre-release gate] — Verify every routing-pending path matches its Director-resolved route.\n")
	if a.ControlState != nil {
		b.WriteString("\n**AT3** [Automated] [Pre-release gate] — Verify every preserve-registry decision is applied before the final removal of `govna/preserve.txt`.\n")
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
