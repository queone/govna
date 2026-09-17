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

func mapEnvironment(stdoutTerminal, stderrTerminal bool, values map[string]string) environment {
	return environment{stdoutTerminal: stdoutTerminal, stderrTerminal: stderrTerminal, lookupEnv: func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}}
}

func execute(args ...string) (string, string, int) {
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr, plainEnvironment())
	return stdout.String(), stderr.String(), code
}

func pageHeader() string {
	return fmt.Sprintf("govna v%s\nAdd and maintain Govna governance files\ngithub.com/queone/govna\n\n", programVersion)
}

func topLevelPage() string {
	return pageHeader() +
		"Usage\n" +
		"  govna COMMAND [options]\n" +
		"\n" +
		"Commands\n" +
		"  apply    add Govna governance files to a repository\n" +
		"  audit    check a repository with Govna for updates and local changes\n" +
		"  rm       write a reviewable AC for removing Govna files\n" +
		"  render   write the selected built-in Govna files to a directory\n" +
		"  version  print executable and embedded governance-file versions\n" +
		"  help     show this help\n" +
		"\n" +
		"  Run 'govna COMMAND -h' for command-specific options.\n" +
		"\n" +
		"Options\n" +
		"  -v, --version   print executable version\n" +
		"  -h, -?, --help  show this help\n"
}

func TestVersionAliases(t *testing.T) {
	for _, alias := range []string{"-v", "--version", "ver", "v"} {
		t.Run(alias, func(t *testing.T) {
			stdout, stderr, code := execute(alias)
			assertResult(t, stdout, stderr, code, fmt.Sprintf("govna v%s\n", programVersion), "", 0)
		})
	}
}

func TestCommandVersionFlags(t *testing.T) {
	for _, command := range []string{"apply", "audit", "rm", "render"} {
		for _, flag := range []string{"-v", "--version"} {
			t.Run(command+" "+flag, func(t *testing.T) {
				stdout, stderr, code := execute(command, flag)
				assertResult(t, stdout, stderr, code, fmt.Sprintf("govna v%s\n", programVersion), "", 0)
			})
		}
	}
}

func TestDetailedVersion(t *testing.T) {
	stdout, stderr, code := execute("version")
	assertResult(t, stdout, stderr, code, fmt.Sprintf("govna v%s\nEmbedded governance-file version (canon version): v%s\n", programVersion, canonVersion), "", 0)

	stdout, stderr, code = execute("version", "extra", "ignored")
	assertResult(t, stdout, stderr, code, "", "unexpected argument for version: extra\nUsage: govna version\n", 2)
}

func TestTopLevelUsage(t *testing.T) {
	expected := topLevelPage()

	stdout, stderr, code := execute()
	assertResult(t, stdout, stderr, code, "", expected, 2)
	for _, alias := range []string{"-h", "--help", "-?", "help", "h"} {
		t.Run(alias, func(t *testing.T) {
			stdout, stderr, code := execute(alias)
			assertResult(t, stdout, stderr, code, expected, "", 0)
		})
	}
	for _, hidden := range []string{"render-canon", "drift-scan", "ver,", " v ", " h "} {
		if strings.Contains(expected, hidden) {
			t.Fatalf("hidden alias %q must stay unlisted", hidden)
		}
	}

	stdout, stderr, code = execute("deps")
	assertResult(t, stdout, stderr, code, "", "unknown command: deps\n"+expected, 2)
}

func TestCommandHelpPages(t *testing.T) {
	render := pageHeader() +
		"Usage\n" +
		"  govna render [options] TARGET\n" +
		"\n" +
		"  Write the selected built-in Govna files to TARGET/ using repository-relative\n" +
		"  paths. This command does not add an adoption AC. Existing target files remain\n" +
		"  unless render replaces them; empty the directory first when you need only the\n" +
		"  rendered files.\n" +
		"\n" +
		"Options\n" +
		"  -f, --flavor code|doc   select Govna file set: CODE or DOC (default: inferred from cwd)\n" +
		"  -s, --stack NAME        select CODE stack (default: inferred from cwd manifests)\n" +
		"  -m, --module-path PATH  module path for Go CODE files (default: read from cwd's go.mod)\n" +
		"  -v, --version           print executable version\n" +
		"  -h, -?, --help          show this help\n"
	audit := pageHeader() +
		"Usage\n" +
		"  govna audit [options]\n" +
		"\n" +
		"  Compare a repository's Govna files with the files built into this executable.\n" +
		"  Run from the repository root with no positional arguments. Writes a reviewable\n" +
		"  AC under govna/ when updates or Director choices are needed.\n" +
		"\n" +
		"Options\n" +
		"  -f, --flavor code|doc  Govna file set (CODE or DOC; default: auto-detect)\n" +
		"  -s, --stack NAME       CODE stack (default: inferred from manifests)\n" +
		"  -j, --json             emit JSON report alongside markdown emission\n" +
		"  -l, --diff-lines N     diff truncation limit (default: 200)\n" +
		"  -n, --repo-name NAME   override repo name (default: basename of cwd)\n" +
		"  -v, --version          print executable version\n" +
		"  -h, -?, --help         show this help\n"
	apply := pageHeader() +
		"Usage\n" +
		"  govna apply [options]\n" +
		"\n" +
		"  Add Govna governance files to the current directory. Govna identifies the\n" +
		"  repository type, reports any required option it cannot determine, and writes an\n" +
		"  AC for review.\n" +
		"\n" +
		"Options\n" +
		"  -f, --flavor code|doc   Govna file set (CODE or DOC; default: auto-detect)\n" +
		"  -s, --stack NAME        CODE stack (default: inferred from manifests)\n" +
		"  -n, --repo-name NAME    repo name (default: basename of cwd)\n" +
		"  -m, --module-path PATH  module path for Go CODE files (default: read from go.mod)\n" +
		"  -g, --init-git          initialize git if the target is not a repo\n" +
		"  -v, --version           print executable version\n" +
		"  -h, -?, --help          show this help\n"
	rm := pageHeader() +
		"Usage\n" +
		"  govna rm [options]\n" +
		"\n" +
		"  Write an AC that lists which Govna files can be removed and which files\n" +
		"  need a Director choice. Run from the repository root with no positional\n" +
		"  arguments. This command deletes nothing.\n" +
		"\n" +
		"Options\n" +
		"  -f, --flavor code|doc  Govna file set (CODE or DOC; default: auto-detect)\n" +
		"  -s, --stack NAME       CODE stack (default: inferred from manifests)\n" +
		"  -n, --repo-name NAME   override repo name (default: basename of cwd)\n" +
		"  -v, --version          print executable version\n" +
		"  -h, -?, --help         show this help\n"
	for _, tc := range []struct {
		command string
		want    string
	}{
		{"render", render},
		{"render-canon", render},
		{"audit", audit},
		{"drift-scan", audit},
		{"apply", apply},
		{"rm", rm},
	} {
		for _, flag := range []string{"-h", "-?", "--help"} {
			t.Run(tc.command+" "+flag, func(t *testing.T) {
				stdout, stderr, code := execute(tc.command, flag)
				assertResult(t, stdout, stderr, code, tc.want, "", 0)
			})
		}
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
	wantApply := fmt.Sprintf("Govna executable v%s added its embedded governance files (canon v%s) for the DOC repository", programVersion, canonVersion)
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
	auditMatches, err := filepath.Glob(filepath.Join(root, "govna", "ac*-audit-v0.63.1.md"))
	if err != nil || len(auditMatches) != 1 {
		t.Fatalf("audit matches=%v err=%v", auditMatches, err)
	}
	assertGeneratedMarkerAxes(t, auditMatches[0], "<!-- audit: emitted-by govna ")

	stdout, stderr, code = execute("rm")
	if code != 0 {
		t.Fatalf("rm code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	removalMatches, err := filepath.Glob(filepath.Join(root, "govna", "ac*-govna-rm-v0.63.1.md"))
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
			var stdout, stderr bytes.Buffer
			if code := run([]string{"-h"}, &stdout, &stderr, mapEnvironment(tc.terminal, false, tc.env)); code != 0 || stderr.Len() != 0 {
				t.Fatalf("help exit=%d stderr=%q", code, stderr.String())
			}
			got := stdout.String()
			if !tc.colored {
				if strings.Contains(got, "\x1b[") {
					t.Fatalf("unexpected color: %q", got)
				}
				return
			}
			wantPrefix := fmt.Sprintf("\x1b[1;38;5;231mgovna\x1b[0m v%s\n\x1b[38;5;245mAdd and maintain Govna governance files\x1b[0m\n\x1b[38;5;242mgithub.com/queone/govna\x1b[0m\n\n\x1b[1;38;5;231mUsage\x1b[0m\n", programVersion)
			if !strings.HasPrefix(got, wantPrefix) {
				t.Fatalf("color prefix mismatch: %q", got)
			}
			for _, heading := range []string{"Commands", "Options"} {
				if !strings.Contains(got, "\n\x1b[1;38;5;231m"+heading+"\x1b[0m\n") {
					t.Fatalf("colored heading %s missing: %q", heading, got)
				}
			}
			if strings.Contains(got, "\x1b[1m") {
				t.Fatalf("bold must be one sequence with the color: %q", got)
			}
		})
	}
}

func TestColorFollowsWrittenStream(t *testing.T) {
	env := mapEnvironment(false, true, map[string]string{"TERM": "xterm-256color"})
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-h"}, &stdout, &stderr, env); code != 0 || strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("requested help on a non-terminal stdout must be plain: code=%d %q", code, stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := run(nil, &stdout, &stderr, env); code != 2 || stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "\x1b[1;38;5;231mgovna\x1b[0m v") {
		t.Fatalf("usage error on a color stderr must be colored: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func assertResult(t *testing.T, stdout, stderr string, code int, wantStdout, wantStderr string, wantCode int) {
	t.Helper()
	if stdout != wantStdout || stderr != wantStderr || code != wantCode {
		t.Fatalf("result mismatch\nstdout: %q\nwant:   %q\nstderr: %q\nwant:   %q\ncode: %d, want %d", stdout, wantStdout, stderr, wantStderr, code, wantCode)
	}
}
