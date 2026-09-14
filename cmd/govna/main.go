package main

import (
	"fmt"
	"io"
	"os"

	"github.com/queone/govna/internal/apply"
	"github.com/queone/govna/internal/audit"
	"github.com/queone/govna/internal/help"
	"github.com/queone/govna/internal/remove"
	"github.com/queone/govna/internal/render"
)

const programVersion = "0.24.0"
const canonVersion = "0.59.0"

type environment struct {
	stdoutTerminal bool
	stderrTerminal bool
	lookupEnv      func(string) (string, bool)
}

func main() {
	env := environment{stdoutTerminal: isTerminal(os.Stdout), stderrTerminal: isTerminal(os.Stderr), lookupEnv: os.LookupEnv}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, env))
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func run(args []string, stdout, stderr io.Writer, env environment) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, topPage().Render(env.stderrColor()))
		return 2
	}

	switch args[0] {
	case "-v", "--version", "ver", "v":
		fmt.Fprintf(stdout, "govna v%s\n", programVersion)
		return 0
	case "version":
		if len(args) > 1 {
			fmt.Fprintf(stderr, "unexpected argument for version: %s\nUsage: govna version\n", args[1])
			return 2
		}
		fmt.Fprintf(stdout, "govna v%s\nEmbedded governance-file version (canon version): v%s\n", programVersion, canonVersion)
		return 0
	case "-h", "--help", "-?", "help", "h":
		fmt.Fprint(stdout, topPage().Render(env.stdoutColor()))
		return 0
	case "render", "render-canon":
		if code, done := commandRequest(args, renderPage(), stdout, env); done {
			return code
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "get cwd: %v\n", err)
			return 1
		}
		return render.Run(args[1:], stdout, stderr, cwd)
	case "audit", "drift-scan":
		if code, done := commandRequest(args, auditPage(), stdout, env); done {
			return code
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "audit: get cwd: %v\n", err)
			return 1
		}
		return audit.Run(args[1:], stdout, stderr, cwd, programVersion)
	case "apply":
		if code, done := commandRequest(args, applyPage(), stdout, env); done {
			return code
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "apply: get cwd: %v\n", err)
			return 1
		}
		return apply.Run(args[1:], stdout, stderr, cwd, programVersion, nil)
	case "rm":
		if code, done := commandRequest(args, remove.Page(programVersion), stdout, env); done {
			return code
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "rm: get cwd: %v\n", err)
			return 1
		}
		return remove.Run(args[1:], stdout, stderr, cwd, programVersion)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		fmt.Fprint(stderr, topPage().Render(env.stderrColor()))
		return 2
	}
}

func isHelp(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "-?"
}

func isVersion(arg string) bool {
	return arg == "-v" || arg == "--version"
}

// commandRequest answers a command's help or version request when that request
// is the command's only argument; done reports whether it answered.
func commandRequest(args []string, page help.Page, stdout io.Writer, env environment) (code int, done bool) {
	if len(args) != 2 {
		return 0, false
	}
	switch {
	case isHelp(args[1]):
		fmt.Fprint(stdout, page.Render(env.stdoutColor()))
		return 0, true
	case isVersion(args[1]):
		fmt.Fprintf(stdout, "govna v%s\n", programVersion)
		return 0, true
	}
	return 0, false
}

func (env environment) stdoutColor() bool { return help.Enabled(env.stdoutTerminal, env.lookupEnv) }

func (env environment) stderrColor() bool { return help.Enabled(env.stderrTerminal, env.lookupEnv) }

func topPage() help.Page {
	return help.Utility(programVersion,
		help.Section{Title: "Usage", Rows: []help.Row{{Form: "govna COMMAND [options]"}}},
		help.Section{Title: "Commands", Rows: []help.Row{
			{Form: "apply", Meaning: "add Govna governance files to a repository"},
			{Form: "audit", Meaning: "check a repository with Govna for updates and local changes"},
			{Form: "rm", Meaning: "write a reviewable AC for removing Govna files"},
			{Form: "render", Meaning: "write the selected built-in Govna files to a directory"},
			{Form: "version", Meaning: "print executable and embedded governance-file versions"},
			{Form: "help", Meaning: "show this help"},
		}, Note: "Run 'govna COMMAND -h' for command-specific options."},
	)
}

func renderPage() help.Page {
	return help.Utility(programVersion,
		help.Section{Title: "Usage", Rows: []help.Row{{Form: "govna render [options] TARGET"}}, Note: "Write the selected built-in Govna files to TARGET/ using repository-relative\npaths. This command does not add an adoption AC. Existing target files remain\nunless render replaces them; empty the directory first when you need only the\nrendered files."},
		help.Section{Title: "Options", Rows: []help.Row{
			{Form: "-f, --flavor code|doc", Meaning: "select Govna file set: CODE or DOC (default: inferred from cwd)"},
			{Form: "-s, --stack NAME", Meaning: "select CODE stack (default: inferred from cwd manifests)"},
			{Form: "-m, --module-path PATH", Meaning: "module path for Go CODE files (default: read from cwd's go.mod)"},
		}},
	)
}

func auditPage() help.Page {
	return help.Utility(programVersion,
		help.Section{Title: "Usage", Rows: []help.Row{{Form: "govna audit [options]"}}, Note: "Compare a repository's Govna files with the files built into this executable.\nRun from the repository root with no positional arguments. Writes a reviewable\nAC under govna/ when updates or Director choices are needed."},
		help.Section{Title: "Options", Rows: []help.Row{
			{Form: "-f, --flavor code|doc", Meaning: "Govna file set (CODE or DOC; default: auto-detect)"},
			{Form: "-s, --stack NAME", Meaning: "CODE stack (default: inferred from manifests)"},
			{Form: "-j, --json", Meaning: "emit JSON report alongside markdown emission"},
			{Form: "-l, --diff-lines N", Meaning: "diff truncation limit (default: 200)"},
			{Form: "-n, --repo-name NAME", Meaning: "override repo name (default: basename of cwd)"},
		}},
	)
}

func applyPage() help.Page {
	return help.Utility(programVersion,
		help.Section{Title: "Usage", Rows: []help.Row{{Form: "govna apply [options]"}}, Note: "Add Govna governance files to the current directory. Govna identifies the\nrepository type, reports any required option it cannot determine, and writes an\nAC for review."},
		help.Section{Title: "Options", Rows: []help.Row{
			{Form: "-f, --flavor code|doc", Meaning: "Govna file set (CODE or DOC; default: auto-detect)"},
			{Form: "-s, --stack NAME", Meaning: "CODE stack (default: inferred from manifests)"},
			{Form: "-n, --repo-name NAME", Meaning: "repo name (default: basename of cwd)"},
			{Form: "-m, --module-path PATH", Meaning: "module path for Go CODE files (default: read from go.mod)"},
			{Form: "-g, --init-git", Meaning: "initialize git if the target is not a repo"},
		}},
	)
}
