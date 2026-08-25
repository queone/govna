package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func plainEnvironment() environment {
	return environment{lookupEnv: func(string) (string, bool) { return "", false }}
}

func mapEnvironment(terminal bool, values map[string]string) environment {
	return environment{stderrTerminal: terminal, lookupEnv: func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}}
}

func execute(args ...string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr, plainEnvironment())
	return stdout.String(), stderr.String(), code
}

func TestVersionAliases(t *testing.T) {
	for _, alias := range []string{"--version", "ver", "v"} {
		t.Run(alias, func(t *testing.T) {
			stdout, stderr, code := execute(alias)
			assertResult(t, stdout, stderr, code, fmt.Sprintf("govna v%s\n", programVersion), "", 0)
		})
	}
}

func TestDetailedVersion(t *testing.T) {
	stdout, stderr, code := execute("version")
	assertResult(t, stdout, stderr, code, fmt.Sprintf("govna binary: v%s\nembedded canon: v0.35.0\n", programVersion), "", 0)

	stdout, stderr, code = execute("version", "extra", "ignored")
	assertResult(t, stdout, stderr, code, "", "unexpected argument for version: extra\nUsage: govna version\n", 2)
}

func TestTopLevelUsage(t *testing.T) {
	expected := fmt.Sprintf("govna v%s\nRepo governance templates — github.com/queone/govna\n\nUsage: govna <command> [options]\n\n", programVersion) +
		"  apply                         apply governance template to a repo\n" +
		"  audit                         drift scan an adopted repo against govna canon\n" +
		"  rm                            emit cleanup AC for removing govna canon\n" +
		"  render                        render flavor-specific canon files into a target directory\n" +
		"  version                       print binary and embedded canon versions\n" +
		"  ver, v, --version             print binary version\n" +
		"  help, h                       show this help\n\n" +
		"Run 'govna <command> -h' for command-specific flags.\n"

	stdout, stderr, code := execute()
	assertResult(t, stdout, stderr, code, "", expected, 2)
	for _, alias := range []string{"-h", "--help", "-?", "help", "h"} {
		t.Run(alias, func(t *testing.T) {
			stdout, stderr, code := execute(alias)
			assertResult(t, stdout, stderr, code, expected, "", 0)
		})
	}
	if strings.Contains(expected, "render-canon") || strings.Contains(expected, "drift-scan") {
		t.Fatal("legacy aliases must remain hidden")
	}

	stdout, stderr, code = execute("deps")
	assertResult(t, stdout, stderr, code, "", "unknown command: deps\n"+expected, 2)
}

func TestReservedCommandHelp(t *testing.T) {
	render := "Usage: govna render [--flavor code|doc] [--stack <name>] [--module-path <path>] <target>\n\n" +
		"  -f, --flavor code|doc         select consumer flavor (default: inferred from cwd)\n" +
		"  -s, --stack <name>            select CODE stack (default: inferred from cwd manifests)\n" +
		"  -m, --module-path <path>      module path for Go CODE canon (default: read from cwd's go.mod)\n\n" +
		"Render canon files into <target>/ in flat repo-relative layout. Canon files only —\n" +
		"no adoption record. Target is not pre-cleaned; remove or empty it beforehand if you\n" +
		"need a fresh tree.\n"
	audit := "Usage: govna audit [options]\n\n" +
		"Scan an adopted-govna repo against canon. Run from the consumer repo root\n" +
		"(no positional arguments). Emits an AC stub under govna/.\n\nFlags:\n" +
		"  -f, --flavor code|doc      overlay flavor (default: auto-detect)\n" +
		"  -s, --stack <name>         CODE stack (default: inferred from manifests)\n" +
		"  -j, --json                 emit JSON report alongside markdown emission\n" +
		"  -l, --diff-lines <N>       diff truncation limit (default: 200)\n" +
		"  -n, --repo-name <name>     override repo name (default: basename of cwd)\n" +
		"  -h, --help                 show this help\n"
	rm := "Usage: govna rm [flags]\n\n" +
		"Emit a Director-reviewed cleanup AC for removing govna canon from an\n" +
		"adopted repo. Run from the consumer repo root (no positional arguments).\n" +
		"Deletes nothing itself.\n\nFlags:\n" +
		"  -f, --flavor code|doc      overlay flavor (default: auto-detect)\n" +
		"  -s, --stack <name>         CODE stack (default: inferred from manifests)\n" +
		"  -n, --repo-name <name>     override repo name (default: basename of cwd)\n" +
		"  -h, --help                 show this help\n"
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"render", []string{"render", "--help"}, render},
		{"render alias", []string{"render-canon", "--help"}, render},
		{"audit", []string{"audit", "--help"}, audit},
		{"audit alias", []string{"drift-scan", "--help"}, audit},
		{"rm", []string{"rm", "--help"}, rm},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := execute(tc.args...)
			assertResult(t, stdout, stderr, code, "", tc.want, 0)
		})
	}
}

func TestApplyHelpAliases(t *testing.T) {
	want := applyHelp()
	for _, alias := range []string{"-h", "--help", "-?"} {
		stdout, stderr, code := execute("apply", alias)
		assertResult(t, stdout, stderr, code, "", want, 0)
	}
}

func TestRenderAliasesOperational(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "Cargo.toml"), []byte("[package]\nname = \"widget\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)
	for _, command := range []string{"render", "render-canon"} {
		target := command + "-out"
		stdout, stderr, code := execute(command, target)
		if code != 0 || stderr != "" || stdout != filepath.Join(cwd, target)+"\n" {
			t.Fatalf("%s: stdout=%q stderr=%q code=%d", command, stdout, stderr, code)
		}
	}
}

func TestTopLevelGeneratedVersionAxes(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	stdout, stderr, code := execute("apply", "--flavor", "doc")
	if code != 0 {
		t.Fatalf("apply code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	applyBody, err := os.ReadFile(filepath.Join(root, "govna", "ac1-govna-apply.md"))
	if err != nil {
		t.Fatal(err)
	}
	wantApply := fmt.Sprintf("govna executable v%s applied embedded canon v%s (DOC overlay)", programVersion, canonVersion)
	if !strings.Contains(string(applyBody), wantApply) {
		t.Fatalf("apply body omits top-level version axes: %s", applyBody)
	}

	gitMainTest(t, root, "init", "-q")
	gitMainTest(t, root, "config", "user.email", "fixture@example.invalid")
	gitMainTest(t, root, "config", "user.name", "Fixture")
	gitMainTest(t, root, "add", ".")
	gitMainTest(t, root, "commit", "-qm", "govna apply")
	if err := os.Remove(filepath.Join(root, "govna", "roles.md")); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code = execute("audit")
	if code != 0 {
		t.Fatalf("audit code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	auditMatches, err := filepath.Glob(filepath.Join(root, "govna", "ac*-audit-v0.35.0.md"))
	if err != nil || len(auditMatches) != 1 {
		t.Fatalf("audit matches=%v err=%v", auditMatches, err)
	}
	assertGeneratedMarkerAxes(t, auditMatches[0], "<!-- audit: emitted-by govna ")

	stdout, stderr, code = execute("rm")
	if code != 0 {
		t.Fatalf("rm code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	removalMatches, err := filepath.Glob(filepath.Join(root, "govna", "ac*-govna-rm-v0.35.0.md"))
	if err != nil || len(removalMatches) != 1 {
		t.Fatalf("removal matches=%v err=%v", removalMatches, err)
	}
	assertGeneratedMarkerAxes(t, removalMatches[0], "<!-- govna-rm: emitted-by govna ")
}

func assertGeneratedMarkerAxes(t *testing.T, path, markerPrefix string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%sexecutable v%s with embedded canon v%s sha256:", markerPrefix, programVersion, canonVersion)
	if !strings.HasPrefix(string(content), want) {
		t.Fatalf("%s marker omits top-level version axes: %s", path, content)
	}
}

func gitMainTest(t *testing.T, root string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", root}, args...)
	if output, err := exec.Command("git", commandArgs...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", commandArgs, err, output)
	}
}

func TestColorGating(t *testing.T) {
	tests := []struct {
		name     string
		terminal bool
		env      map[string]string
		colored  bool
	}{
		{"non-terminal", false, map[string]string{"TERM": "xterm-256color"}, false},
		{"NO_COLOR empty", true, map[string]string{"NO_COLOR": "", "TERM": "xterm-256color"}, false},
		{"TERM dumb", true, map[string]string{"TERM": "dumb", "COLORTERM": "truecolor"}, false},
		{"unsupported", true, map[string]string{"TERM": "xterm"}, false},
		{"truecolor", true, map[string]string{"COLORTERM": "truecolor"}, true},
		{"24bit", true, map[string]string{"COLORTERM": "24bit"}, true},
		{"256color", true, map[string]string{"TERM": "screen-256color"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			env := mapEnvironment(tc.terminal, tc.env)
			got := usageText(env)
			if tc.colored {
				wantPrefix := fmt.Sprintf("\x1b[1m\x1b[38;5;231mgovna\x1b[0m v%s\n\x1b[38;5;245mRepo governance templates — github.com/queone/govna\x1b[0m", programVersion)
				if !strings.HasPrefix(got, wantPrefix) {
					t.Fatalf("color prefix mismatch: %q", got)
				}
				if !strings.Contains(got, "\x1b[1m\x1b[38;5;231mUsage:\x1b[0m govna") {
					t.Fatalf("colored Usage missing: %q", got)
				}
			} else if strings.Contains(got, "\x1b[") {
				t.Fatalf("unexpected color: %q", got)
			}
		})
	}
}

func assertResult(t *testing.T, stdout, stderr string, code int, wantStdout, wantStderr string, wantCode int) {
	t.Helper()
	if stdout != wantStdout || stderr != wantStderr || code != wantCode {
		t.Fatalf("result mismatch\nstdout: %q\nwant:   %q\nstderr: %q\nwant:   %q\ncode: %d, want %d", stdout, wantStdout, stderr, wantStderr, code, wantCode)
	}
}
