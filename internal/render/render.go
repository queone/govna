package render

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/queone/govna/internal/canon"
	"github.com/queone/govna/internal/repository"
	"github.com/queone/govna/internal/usererr"
)

func Run(args []string, stdout, stderr io.Writer, cwd string) int {
	flavor, stack, modulePath, target, code := parse(args, stderr)
	if code != 0 {
		return code
	}
	resolvedFlavor, err := repository.Flavor(cwd, flavor)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if resolvedFlavor == canon.Doc && stack != "" {
		fmt.Fprintln(stderr, "--stack applies only to CODE canon; remove --stack or select --flavor code")
		return 1
	}
	if resolvedFlavor == canon.Doc && modulePath != "" {
		fmt.Fprintln(stderr, "--module-path applies only to Go CODE canon; remove --module-path or select --flavor code")
		return 1
	}
	if resolvedFlavor == canon.Code {
		if stack == "" {
			stack = repository.Stack(cwd)
			if stack == "" {
				fmt.Fprintf(stderr, "could not infer a supported CODE stack from cwd=%s; use %s with --stack\n", cwd, canon.SupportedStackChoices)
				return 1
			}
		}
		canonical, ok := canon.CanonicalStack(stack)
		if !ok {
			fmt.Fprintf(stderr, "render canon: unsupported CODE stack %q: use %s\n", stack, canon.SupportedStackChoices)
			return 1
		}
		stack = canonical
		if stack == "Go" {
			if modulePath == "" {
				modulePath = repository.ModulePath(cwd)
				if modulePath == "" {
					fmt.Fprintf(stderr, "could not read module path from cwd's go.mod (cwd=%s); pass --module-path to override\n", cwd)
					return 1
				}
			}
		} else if modulePath != "" {
			fmt.Fprintln(stderr, "--module-path applies only to Go CODE canon; remove --module-path or select --stack Go")
			return 1
		}
	}
	absTarget := target
	if !filepath.IsAbs(absTarget) {
		absTarget = filepath.Join(cwd, target)
	}
	repoName := filepath.Base(cwd)
	if modulePath != "" {
		repoName = path.Base(modulePath)
	}
	files, err := canon.Render(canon.Config{Flavor: resolvedFlavor, RepoName: repoName, Stack: stack, ModulePath: modulePath})
	if err != nil {
		fmt.Fprintf(stderr, "render canon: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(absTarget, 0o755); err != nil {
		fmt.Fprintf(stderr, "create target %s: %v\n", absTarget, err)
		return 1
	}
	for _, file := range files {
		destination := filepath.Join(absTarget, filepath.FromSlash(file.Path))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			fmt.Fprintf(stderr, "write %s: %v\n", destination, err)
			return 1
		}
		mode := os.FileMode(0o644)
		if filepath.Ext(destination) == ".sh" {
			mode = 0o755
		}
		if err := os.WriteFile(destination, file.Content, mode); err != nil {
			fmt.Fprintf(stderr, "write %s: %v\n", destination, err)
			return 1
		}
		if err := os.Chmod(destination, mode); err != nil {
			fmt.Fprintf(stderr, "write %s: %v\n", destination, err)
			return 1
		}
	}
	claude := filepath.Join(absTarget, "CLAUDE.md")
	_ = os.Remove(claude)
	if err := os.Symlink("AGENTS.md", claude); err != nil {
		fmt.Fprintf(stderr, "create symlink %s: %v\n", claude, err)
		return 1
	}
	fmt.Fprintln(stdout, absTarget)
	return 0
}

func parse(args []string, stderr io.Writer) (flavor, stack, modulePath, target string, code int) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--flavor":
			if i+1 == len(args) {
				fmt.Fprintln(stderr, "--flavor requires a value")
				return "", "", "", "", 2
			}
			i++
			flavor = args[i]
		case "-s", "--stack":
			if i+1 == len(args) {
				fmt.Fprintln(stderr, "--stack requires a value")
				return "", "", "", "", 2
			}
			i++
			stack = strings.TrimSpace(args[i])
			if stack == "" {
				fmt.Fprintln(stderr, "--stack requires a non-empty value")
				return "", "", "", "", 2
			}
		case "-m", "--module-path":
			if i+1 == len(args) {
				fmt.Fprintln(stderr, "--module-path requires a value")
				return "", "", "", "", 2
			}
			i++
			modulePath = args[i]
		default:
			if target != "" {
				fmt.Fprintf(stderr, "unexpected argument: %s (target already set to %q)\n", args[i], target)
				return "", "", "", "", 2
			}
			target = args[i]
		}
	}
	if target == "" {
		fmt.Fprintln(stderr, "render requires a positional <target> argument")
		return "", "", "", "", 2
	}
	if flavor != "" && flavor != "code" && flavor != "doc" {
		fmt.Fprintf(stderr, "invalid --flavor: %q (must be 'code' or 'doc')\n", flavor)
		return "", "", "", "", 2
	}
	return flavor, stack, modulePath, target, 0
}

func resolveFlavor(cwd, explicit string) (canon.Flavor, error) {
	if explicit == "code" {
		return canon.Code, nil
	}
	if explicit == "doc" {
		return canon.Doc, nil
	}
	metadata, err := os.ReadFile(filepath.Join(cwd, "govna", "metadata.txt"))
	if err == nil {
		text := string(metadata)
		if !strings.HasSuffix(text, "\n") {
			return "", fmt.Errorf("invalid %s: require a final newline", filepath.Join(cwd, "govna", "metadata.txt"))
		}
		values := map[string]string{}
		for line := range strings.SplitSeq(strings.TrimSuffix(text, "\n"), "\n") {
			key, value, ok := strings.Cut(line, " = ")
			if !ok || key == "" || value == "" {
				return "", fmt.Errorf("invalid %s: each line must use `key = value`", filepath.Join(cwd, "govna", "metadata.txt"))
			}
			values[key] = value
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
		return "", fmt.Errorf("read %s: %v", filepath.Join(cwd, "govna", "metadata.txt"), err)
	}
	_, jekyll := os.Stat(filepath.Join(cwd, "_config.yml"))
	hasJekyll := jekyll == nil
	hasCode := false
	for _, name := range []string{"go.mod", "Cargo.toml", "Package.swift", ".terraform.lock.hcl"} {
		if regularFile(filepath.Join(cwd, name)) {
			hasCode = true
		}
	}
	if hasRegularMatch(filepath.Join(cwd, "*.tf")) {
		hasCode = true
	}
	if hasCode && hasJekyll {
		return "", usererr.Errorf("Govna found both CODE and DOC evidence: the repository has _config.yml and a CODE project manifest; pass --flavor code or --flavor doc")
	}
	if hasCode {
		return canon.Code, nil
	}
	if hasJekyll {
		return canon.Doc, nil
	}
	for _, item := range []struct{ name, stack string }{{"package.json", "Node"}, {"pyproject.toml", "Python"}, {"pom.xml", "Java"}, {"build.gradle", "Java"}} {
		if regularFile(filepath.Join(cwd, item.name)) {
			return "", usererr.Errorf("Govna found %s for the unsupported %s CODE stack; use %s", item.name, item.stack, canon.SupportedStackChoices)
		}
	}
	return "", usererr.Errorf("Govna could not determine whether this is a CODE or DOC repository; add govna/metadata.txt, pass --flavor code|doc, or add a recognized project manifest")
}

func inferStack(cwd string) string {
	for _, item := range []struct{ name, stack string }{{"go.mod", "Go"}, {".terraform.lock.hcl", "Terraform"}, {"Cargo.toml", "Rust"}, {"Package.swift", "Swift"}} {
		if regularFile(filepath.Join(cwd, item.name)) {
			return item.stack
		}
	}
	if hasRegularMatch(filepath.Join(cwd, "*.tf")) {
		return "Terraform"
	}
	return ""
}

func regularFile(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.Mode().IsRegular()
}

func hasRegularMatch(pattern string) bool {
	matches, _ := filepath.Glob(pattern)
	return slices.ContainsFunc(matches, regularFile)
}
