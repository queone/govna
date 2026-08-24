package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/queone/govna/internal/apply"
	"github.com/queone/govna/internal/audit"
	"github.com/queone/govna/internal/remove"
	"github.com/queone/govna/internal/render"
)

const programVersion = "0.7.1"
const canonVersion = "0.31.0"
const sourceRepo = "github.com/queone/govna"

type environment struct {
	stderrTerminal bool
	lookupEnv      func(string) (string, bool)
}

func main() {
	terminal := false
	if info, err := os.Stderr.Stat(); err == nil {
		terminal = info.Mode()&os.ModeCharDevice != 0
	}
	env := environment{stderrTerminal: terminal, lookupEnv: os.LookupEnv}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, env))
}

func run(args []string, stdout, stderr io.Writer, env environment) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText(env))
		return 2
	}

	switch args[0] {
	case "--version", "ver", "v":
		fmt.Fprintf(stdout, "govna v%s\n", programVersion)
		return 0
	case "version":
		if len(args) > 1 {
			fmt.Fprintf(stderr, "unexpected argument for version: %s\nUsage: govna version\n", args[1])
			return 2
		}
		fmt.Fprintf(stdout, "govna binary: v%s\nembedded canon: v%s\n", programVersion, canonVersion)
		return 0
	case "-h", "--help", "-?", "help", "h":
		fmt.Fprint(stdout, usageText(env))
		return 0
	case "render", "render-canon":
		if len(args) == 2 && isHelp(args[1]) {
			fmt.Fprint(stderr, renderHelp(env))
			return 0
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "get cwd: %v\n", err)
			return 1
		}
		return render.Run(args[1:], stdout, stderr, cwd)
	case "audit", "drift-scan":
		if len(args) == 2 && isHelp(args[1]) {
			fmt.Fprint(stderr, auditHelp())
			return 0
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "audit: get cwd: %v\n", err)
			return 1
		}
		return audit.Run(args[1:], stdout, stderr, cwd)
	case "apply":
		if len(args) == 2 && isHelp(args[1]) {
			fmt.Fprint(stderr, applyHelp())
			return 0
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "apply: get cwd: %v\n", err)
			return 1
		}
		return apply.Run(args[1:], stdout, stderr, cwd, nil)
	case "rm":
		if len(args) == 2 && isHelp(args[1]) {
			fmt.Fprint(stderr, remove.Help())
			return 0
		}
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "rm: get cwd: %v\n", err)
			return 1
		}
		return remove.Run(args[1:], stdout, stderr, cwd)
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		fmt.Fprint(stderr, usageText(env))
		return 2
	}
}

func isHelp(arg string) bool {
	return arg == "-h" || arg == "--help" || arg == "-?"
}

func colorEnabled(env environment) bool {
	if !env.stderrTerminal || env.lookupEnv == nil {
		return false
	}
	if _, exists := env.lookupEnv("NO_COLOR"); exists {
		return false
	}
	if term, _ := env.lookupEnv("TERM"); term == "dumb" {
		return false
	}
	colorTerm, _ := env.lookupEnv("COLORTERM")
	term, _ := env.lookupEnv("TERM")
	return colorTerm == "truecolor" || colorTerm == "24bit" || strings.Contains(term, "256color")
}

func colorize(env environment, code, text string) string {
	if !colorEnabled(env) {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func boldWhite(env environment, text string) string {
	if !colorEnabled(env) {
		return text
	}
	return "\x1b[1m\x1b[38;5;231m" + text + "\x1b[0m"
}

func usageLine(flag, description string) string {
	padding := max(32-2-len([]rune(flag)), 2)
	return "  " + flag + strings.Repeat(" ", padding) + description
}

func usageText(env environment) string {
	return fmt.Sprintf("%s v%s\n%s\n\n%s govna <command> [options]\n\n%s\n%s\n%s\n%s\n%s\n%s\n%s\n\nRun 'govna <command> -h' for command-specific flags.\n",
		boldWhite(env, "govna"), programVersion,
		colorize(env, "38;5;245", "Repo governance templates — "+sourceRepo),
		boldWhite(env, "Usage:"),
		usageLine("apply", "apply governance template to a repo"),
		usageLine("audit", "drift scan an adopted repo against govna canon"),
		usageLine("rm", "emit cleanup AC for removing govna canon"),
		usageLine("render", "render flavor-specific canon files into a target directory"),
		usageLine("version", "print binary and embedded canon versions"),
		usageLine("ver, v, --version", "print binary version"),
		usageLine("help, h", "show this help"))
}

func renderHelp(env environment) string {
	return fmt.Sprintf("%s govna render [--flavor code|doc] [--stack <name>] [--module-path <path>] <target>\n\n%s\n%s\n%s\n\nRender canon files into <target>/ in flat repo-relative layout. Canon files only —\nno adoption record. Target is not pre-cleaned; remove or empty it beforehand if you\nneed a fresh tree.\n",
		boldWhite(env, "Usage:"),
		usageLine("-f, --flavor code|doc", "select consumer flavor (default: inferred from cwd)"),
		usageLine("-s, --stack <name>", "select CODE stack (default: inferred from cwd manifests)"),
		usageLine("-m, --module-path <path>", "module path for Go CODE canon (default: read from cwd's go.mod)"))
}

func auditHelp() string {
	return "Usage: govna audit [options]\n\n" +
		"Scan an adopted-govna repo against canon. Run from the consumer repo root\n" +
		"(no positional arguments). Emits an AC stub under govna/.\n\n" +
		"Flags:\n" +
		"  -f, --flavor code|doc      overlay flavor (default: auto-detect)\n" +
		"  -s, --stack <name>         CODE stack (default: inferred from manifests)\n" +
		"  -j, --json                 emit JSON report alongside markdown emission\n" +
		"  -l, --diff-lines <N>       diff truncation limit (default: 200)\n" +
		"  -n, --repo-name <name>     override repo name (default: basename of cwd)\n" +
		"  -h, --help                 show this help\n"
}

func applyHelp() string {
	return "Usage: govna apply [flags]\n\nApply governance template to the current directory (new or existing repo).\nDetects repo state, resolves missing parameters, and writes an adoption AC.\n\nFlags:\n  -f, --flavor code|doc      overlay flavor (default: auto-detect)\n  -s, --stack <name>         CODE stack (default: inferred from manifests)\n  -n, --repo-name <name>     repo name (default: basename of cwd)\n  -m, --module-path <path>   module path for Go CODE canon (default: read from go.mod)\n  -g, --init-git             initialize git if the target is not a repo\n  -h, --help                 show this help\n"
}
