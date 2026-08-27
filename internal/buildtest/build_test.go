package buildtest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func run(t *testing.T, dir string, input string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("/bin/bash", args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestGoBuildRejectsValidationTokens(t *testing.T) {
	root := repoRoot(t)
	script, err := os.ReadFile(filepath.Join(root, "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"_validation_token()", "_validation_fingerprints()", "refresh_validation_token_run()", "==> Validation token:"} {
		if strings.Contains(string(script), forbidden) {
			t.Fatalf("Go build contains token behavior %q", forbidden)
		}
	}
	out, err := run(t, root, "", "./build.sh", "prep", "--validation-token", "secret-evidence", "v9.9.9", "test")
	if err == nil || !strings.Contains(out, "unsupported for Go") || strings.Contains(out, "secret-evidence") {
		t.Fatalf("token rejection: %v: %s", err, out)
	}
}

func TestRenderedGoBuildMatchesRoot(t *testing.T) {
	root := repoRoot(t)
	a, err := os.ReadFile(filepath.Join(root, "build.sh"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "internal/canon/assets/overlays/code/stacks/go/build.sh.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"Validate compiled utility versions", "Install validated utilities", "prep: running pre-check build", "prep: running post-check build"} {
		if !strings.Contains(string(a), marker) || !strings.Contains(string(b), marker) {
			t.Fatalf("root/rendered Go build scripts lack shared marker %q", marker)
		}
	}
	for _, marker := range []string{
		"for d in cmd/*/; do\n    [ -f \"${d}main.go\" ] || continue\n    discovered_targets+=(\"$(basename \"$d\")\")\n  done",
		"discovered_list=$(printf '%s\\n' \"${discovered_targets[@]}\" | LC_ALL=C sort)",
		"_install_validated_utility()",
		"trap '_build_signal_exit 143' TERM",
	} {
		if !strings.Contains(string(a), marker) || !strings.Contains(string(b), marker) {
			t.Fatalf("root/rendered Go target discovery lacks shared marker %q", marker)
		}
	}
	if !strings.Contains(string(a), "_validate_root_canon_version") || strings.Contains(string(b), "_validate_root_canon_version") {
		t.Fatal("root-only canon-version boundary is incorrect")
	}
}

func TestRenderedGoHelperClosure(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal/canon/assets/overlays/code/stacks/go/build.sh.tmpl")
	script := string(mustRead(t, path))
	closure := scanShellHelperClosure(script)
	if err := closure.validate(); err != nil {
		t.Fatal(err)
	}
	for _, helper := range []string{"_build_signal_exit", "_print_coverage_summary", "_domain_coverage", "_extract_program_version", "_install_validated_utility", "_is_strict_stable_semver"} {
		if closure.definitions[helper] != 1 {
			t.Errorf("%s definitions=%d", helper, closure.definitions[helper])
		}
	}
	wantReferences := []string{
		"_byte_len",
		"_cleanup_build_owned",
		"_collect_test_files",
		"_color_init",
		"_count_program_version_declarations",
		"_domain_coverage",
		"_emit_usage_line",
		"_ensure_git_repo",
		"_ensure_staticcheck",
		"_extract_program_version",
		"_go_quote",
		"_install_validated_utility",
		"_is_blank",
		"_is_strict_stable_semver",
		"_join_comma_space",
		"_lint_regex_hits",
		"_lint_test_naming",
		"_md_files",
		"_next_patch_tag",
		"_prep_apply_changelog_insert",
		"_prep_apply_version_bump",
		"_prep_build",
		"_prep_detect_changelog_targets",
		"_prep_detect_version_targets",
		"_prep_emit_release_command",
		"_prep_find_ac_files",
		"_prep_find_ie_lines",
		"_prep_module_basename",
		"_prep_parse_ac_refs",
		"_prep_print_dry_run",
		"_prep_remove_ie_lines",
		"_prep_validate_git_state",
		"_prep_validate_multi_utility_versions",
		"_print_coverage_summary",
		"_recovery_error",
		"_rel_step",
		"_run_git",
		"_scan_nested_fences",
		"_trim",
		"_validate_utility_version_output",
		"_wrap",
	}
	if got := closure.referenceNames(); strings.Join(got, "\n") != strings.Join(wantReferences, "\n") {
		t.Fatalf("helper references:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(wantReferences, "\n"))
	}
	if out, err := exec.Command("/bin/bash", "-n", path).CombinedOutput(); err != nil {
		t.Fatalf("bash syntax: %v: %s", err, out)
	}
}

func TestRenderedGoHelperClosureRecognizesCallForms(t *testing.T) {
	forms := []struct {
		name string
		call string
	}{
		{"direct call", "_known_helper"},
		{"command-boundary call", "true && _known_helper"},
		{"conditional call", "if ! _known_helper; then :; fi"},
		{"command substitution", "value=$(_known_helper)"},
		{"nested command substitution", `value=$(printf '%s' "$(_known_helper)")`},
		{"input process substitution", "cat < <(_known_helper)"},
		{"output process substitution", "cat > >(_known_helper)"},
		{"active heredoc substitution", "cat <<EOF\n$(_known_helper)\nEOF"},
	}
	for _, form := range forms {
		t.Run(form.name, func(t *testing.T) {
			closure := scanShellHelperClosure("_known_helper() { :; }\n" + form.call + "\n")
			if err := closure.validate(); err != nil {
				t.Fatal(err)
			}
			if got := closure.referenceNames(); len(got) != 1 || got[0] != "_known_helper" {
				t.Fatalf("helper references=%v", got)
			}
		})
	}
}

func TestRenderedGoHelperClosureRejectsMutations(t *testing.T) {
	root := repoRoot(t)
	script := string(mustRead(t, filepath.Join(root, "internal/canon/assets/overlays/code/stacks/go/build.sh.tmpl")))
	removed := strings.Replace(script, "_extract_program_version() {", "extract_program_version_removed() {", 1)
	if removed == script {
		t.Fatal("definition-removal mutation did not change the script")
	}
	mutations := map[string]string{
		"definition removed":          removed,
		"definition duplicated":       script + "\n_extract_program_version() { :; }\n",
		"direct call":                 script + "\n_missing_helper\n",
		"command-boundary call":       script + "\ntrue && _missing_helper\n",
		"conditional call":            script + "\nif ! _missing_helper; then :; fi\n",
		"command substitution":        script + "\nvalue=$(_missing_helper)\n",
		"nested command substitution": script + "\nvalue=$(printf '%s' \"$(_missing_helper)\")\n",
		"input process substitution":  script + "\ncat < <(_missing_helper)\n",
		"output process substitution": script + "\ncat > >(_missing_helper)\n",
		"active heredoc substitution": script + "\ncat <<EOF\n$(_missing_helper)\nEOF\n",
	}
	for name, mutated := range mutations {
		t.Run(name, func(t *testing.T) {
			if err := scanShellHelperClosure(mutated).validate(); err == nil {
				t.Fatal("mutation passed helper-closure validation")
			}
		})
	}
}

func TestRenderedGoHelperClosureIgnoresNonCalls(t *testing.T) {
	corpus := `_known_helper() { :; }
_shell_variable=value
$_variable_command argument
"$_quoted_variable_command" argument
# _comment_only
pattern='(_single_quoted_data)'
pattern="(_double_quoted_data)"
awk 'BEGIN { _embedded_language() }'
cat <<'EOF'
_quoted_heredoc_data
$(_quoted_heredoc_substitution)
EOF
cat <<EOF
_unquoted_heredoc_data
EOF
`
	closure := scanShellHelperClosure(corpus)
	if err := closure.validate(); err != nil {
		t.Fatal(err)
	}
	if got := closure.referenceNames(); len(got) != 0 {
		t.Fatalf("non-calls classified as helpers: %v", got)
	}
	if closure.definitions["_known_helper"] != 1 {
		t.Fatalf("definition count=%d", closure.definitions["_known_helper"])
	}
}

func TestRenderedGoBuildExecutesWithoutNetwork(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	script := mustRead(t, filepath.Join(root, "internal/canon/assets/overlays/code/stacks/go/build.sh.tmpl"))
	writeBuildFixture(t, filepath.Join(dir, "build.sh"), script, 0o755)
	writeBuildFixture(t, filepath.Join(dir, "go.mod"), []byte("module example.com/widget\n\ngo 1.27.0\n"), 0o644)
	writeBuildFixture(t, filepath.Join(dir, "cmd/alpha/main.go"), []byte("package main\n\nconst programVersion = \"1.0.0\"\n\nfunc main() {}\n"), 0o644)
	writeBuildFixture(t, filepath.Join(dir, "cmd/widget/main.go"), []byte("package main\n\nconst programVersion = \"1.2.3\"\n\nfunc main() {}\n"), 0o644)
	writeBuildFixture(t, filepath.Join(dir, "cmd/zeta/main.go"), []byte("package main\n\nconst programVersion = \"2.0.0\"\n\nfunc main() {}\n"), 0o644)
	writeBuildFixture(t, filepath.Join(dir, "cmd/shared/shared.go"), []byte("package shared\n\nfunc Value() string { return \"shared\" }\n"), 0o644)
	writeBuildFixture(t, filepath.Join(dir, "internal/domain/domain.go"), []byte("package domain\n"), 0o644)

	fakeBin := filepath.Join(dir, "fakebin")
	gopath := filepath.Join(dir, "gopath")
	tmpRoot := filepath.Join(dir, "tmp")
	trace := filepath.Join(dir, "go.trace")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(tmpRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	fakeGo := `#!/bin/bash
set -euo pipefail
printf '%s\n' "$*" >>"$FAKE_TRACE"
case "$1" in
list)
  printf '%s\n' 'example.com/widget'
  ;;
env)
  case "$2" in
  GOPATH) printf '%s\n' "$FAKE_GOPATH" ;;
  GOEXE) printf '\n' ;;
  *) exit 2 ;;
  esac
  ;;
mod)
  [ "$2" = tidy ]
  ;;
fmt|fix|vet)
  ;;
test)
  cover=''
  for arg in "$@"; do
    case "$arg" in -coverprofile=*) cover="${arg#-coverprofile=}" ;; esac
  done
  [ -n "$cover" ]
  printf '%s\n' 'mode: set' 'example.com/widget/internal/domain/domain.go:1.1,1.2 1 1' >"$cover"
  if [ "${FAKE_TEST_FAIL:-}" = 1 ]; then
    printf 'injected test failure\n' >&2
    exit 8
  fi
  ;;
tool)
  [ "$2" = cover ]
  if [ "${FAKE_COVER_FAIL:-}" = 1 ]; then
    printf 'injected coverage failure\n' >&2
    exit 8
  fi
  printf '%s\n' 'example.com/widget/internal/domain/domain.go:1: f 100.0%' 'total: (statements) 100.0%'
  ;;
install)
  [ "$2" = 'honnef.co/go/tools/cmd/staticcheck@v0.8.1' ]
  mkdir -p "$FAKE_GOPATH/bin"
  printf '%s\n' '#!/bin/bash' 'printf '\''staticcheck %s\n'\'' "$*" >>"$FAKE_TRACE"' 'exit 0' >"$FAKE_GOPATH/bin/staticcheck"
  chmod +x "$FAKE_GOPATH/bin/staticcheck"
  ;;
build)
  output=''
  target=''
  shift
  while [ "$#" -gt 0 ]; do
    if [ "$1" = -o ]; then
      shift
      output="$1"
    else
      target="$1"
    fi
    shift
  done
  [ -n "$output" ]
  case "$target" in
  ./cmd/alpha) utility=alpha; version=1.0.0 ;;
  ./cmd/widget) utility=widget; version=1.2.3 ;;
  ./cmd/zeta) utility=zeta; version=2.0.0 ;;
  *) printf 'unexpected fake go build target: %s\n' "$target" >&2; exit 2 ;;
  esac
  if [ "${FAKE_BUILD_FAIL_TARGET:-}" = "$utility" ]; then
    printf 'injected build failure for %s\n' "$utility" >&2
    exit 9
  fi
  reported_version="$version"
  if [ "${FAKE_BAD_VERSION_TARGET:-}" = "$utility" ]; then
    reported_version=9.9.9
  fi
  mkdir -p "$(dirname "$output")"
  cat >"$output" <<EOF
#!/bin/bash
if [ "\${1:-}" = --version ]; then
  if [ -n "\${FAKE_TRACE:-}" ]; then printf 'version $utility\n' >>"\${FAKE_TRACE}"; fi
  printf '$utility $reported_version\n'
  exit 0
fi
exit 2
EOF
  chmod +x "$output"
  ;;
*)
  printf 'unexpected fake go command: %s\n' "$*" >&2
  exit 2
  ;;
esac
`
	writeBuildFixture(t, filepath.Join(fakeBin, "go"), []byte(fakeGo), 0o755)
	fakeMv := "#!/bin/bash\nprintf 'mv %s\\n' \"$*\" >>\"$FAKE_TRACE\"\nif [ \"${FAKE_MV_FAIL:-}\" = 1 ]; then printf 'injected install failure\\n' >&2; exit 9; fi\nexec /bin/mv \"$@\"\n"
	writeBuildFixture(t, filepath.Join(fakeBin, "mv"), []byte(fakeMv), 0o755)
	realMktemp, err := exec.LookPath("mktemp")
	if err != nil {
		t.Fatal(err)
	}
	fakeMktemp := fmt.Sprintf("#!/bin/bash\nprintf 'mktemp %%s\\n' \"$*\" >>\"$FAKE_TRACE\"\nexec %q \"$@\"\n", realMktemp)
	writeBuildFixture(t, filepath.Join(fakeBin, "mktemp"), []byte(fakeMktemp), 0o755)
	if err := os.MkdirAll(filepath.Join(gopath, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("/bin/bash", "./build.sh")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"NO_COLOR=1",
		"TERM=dumb",
		"GOVNA_FORCE_TTY=0",
		"FAKE_GOPATH="+gopath,
		"FAKE_TRACE="+trace,
		"TMPDIR="+tmpRoot,
		"PATH="+fakeBin+":"+os.Getenv("PATH"),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rendered build: %v:\n%s", err, out)
	}
	output := string(out)
	for _, want := range []string{"domain coverage: 100.0%", `programVersion = "1.0.0"`, `programVersion = "1.2.3"`, `programVersion = "2.0.0"`, "installed:"} {
		if !strings.Contains(output, want) {
			t.Errorf("rendered build output omits %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "cmd/shared: programVersion") {
		t.Fatalf("rendered full build treated shared package as a utility:\n%s", output)
	}
	if strings.Contains(output, "command not found") {
		t.Fatalf("rendered build has undefined command:\n%s", output)
	}
	traceBody := string(mustRead(t, trace))
	for _, validation := range []string{"fmt ./...\n", "fix ./...\n", "vet ./...\n"} {
		if !strings.Contains(traceBody, validation) {
			t.Fatalf("full build trace omits repository-wide validation %q:\n%s", validation, traceBody)
		}
	}
	if got := utilityBuildTargets(traceBody); strings.Join(got, ",") != "./cmd/alpha,./cmd/widget,./cmd/zeta" {
		t.Fatalf("full-build utility order=%v trace:\n%s", got, traceBody)
	}
	buildOutputs := utilityBuildOutputs(traceBody)
	if len(buildOutputs) != 3 {
		t.Fatalf("full-build outputs=%v trace:\n%s", buildOutputs, traceBody)
	}
	ownedDir := filepath.Dir(buildOutputs[0])
	if filepath.Dir(ownedDir) != tmpRoot || !strings.HasPrefix(filepath.Base(ownedDir), "govna-go-build.") {
		t.Fatalf("full-build output is not in an invocation-owned directory: %s", buildOutputs[0])
	}
	seenOutputs := map[string]bool{}
	for _, buildOutput := range buildOutputs {
		if filepath.Dir(buildOutput) != ownedDir || seenOutputs[buildOutput] {
			t.Fatalf("full-build output is outside shared unique staging: %v", buildOutputs)
		}
		seenOutputs[buildOutput] = true
	}
	if strings.Count(traceBody, "staticcheck ./...\n") != 1 {
		t.Fatalf("full build did not invoke the pinned Staticcheck binary exactly once:\n%s", traceBody)
	}
	firstInstall := strings.Index(traceBody, "mv ")
	if firstInstall < 0 {
		t.Fatalf("full-build trace omits atomic installation:\n%s", traceBody)
	}
	installMoves := utilityInstallMoves(traceBody)
	if len(installMoves) != 3 {
		t.Fatalf("full-build install moves=%v trace:\n%s", installMoves, traceBody)
	}
	seenInstallSources := map[string]bool{}
	for utility, move := range installMoves {
		if filepath.Dir(move[0]) != filepath.Join(gopath, "bin") || filepath.Dir(move[1]) != filepath.Join(gopath, "bin") ||
			!strings.HasPrefix(filepath.Base(move[0]), ".govna-install-"+utility+".") || filepath.Base(move[1]) != utility || seenInstallSources[move[0]] {
			t.Fatalf("%s installation was not a unique adjacent atomic move: %v", utility, move)
		}
		seenInstallSources[move[0]] = true
	}
	for _, utility := range []string{"alpha", "widget", "zeta"} {
		versionCheck := strings.Index(traceBody, "version "+utility+"\n")
		if versionCheck < 0 || versionCheck > firstInstall {
			t.Fatalf("%s version validation did not precede every installation:\n%s", utility, traceBody)
		}
	}
	if _, statErr := os.Stat(filepath.Join(gopath, "bin", "shared")); !os.IsNotExist(statErr) {
		t.Fatalf("shared package produced an installed utility: %v", statErr)
	}

	scoped := exec.Command("/bin/bash", "./build.sh", "zeta")
	scoped.Dir = dir
	scoped.Env = cmd.Env
	scopedOut, scopedErr := scoped.CombinedOutput()
	if scopedErr != nil {
		t.Fatalf("rendered scoped build: %v:\n%s", scopedErr, scopedOut)
	}
	if !strings.Contains(string(scopedOut), "Building specific utilities: zeta") || !strings.Contains(string(scopedOut), "cmd/zeta: programVersion") || strings.Contains(string(scopedOut), "cmd/alpha: programVersion") || strings.Contains(string(scopedOut), "cmd/widget: programVersion") || strings.Contains(string(scopedOut), "command not found") {
		t.Fatalf("rendered scoped build output:\n%s", scopedOut)
	}
	traceBody = string(mustRead(t, trace))
	if got := utilityBuildTargets(traceBody); strings.Join(got, ",") != "./cmd/alpha,./cmd/widget,./cmd/zeta,./cmd/zeta" {
		t.Fatalf("scoped utility order=%v trace:\n%s", got, traceBody)
	}
	if !strings.Contains(traceBody, "install honnef.co/go/tools/cmd/staticcheck@v0.8.1\n") {
		t.Fatalf("go trace omits exact Staticcheck pin:\n%s", traceBody)
	}
	if strings.Count(traceBody, "staticcheck ./cmd/zeta\n") != 1 {
		t.Fatalf("scoped build did not invoke the pinned Staticcheck binary exactly once:\n%s", traceBody)
	}
	installedBeforeFailures := map[string]string{}
	for utility, version := range map[string]string{"alpha": "1.0.0", "widget": "1.2.3", "zeta": "2.0.0"} {
		installed := filepath.Join(gopath, "bin", utility)
		if versionOut, versionErr := exec.Command(installed, "--version").CombinedOutput(); versionErr != nil || string(versionOut) != utility+" "+version+"\n" {
			t.Fatalf("installed %s version: %v: %q", utility, versionErr, versionOut)
		}
		installedBeforeFailures[utility] = string(mustRead(t, installed))
	}

	traceBeforeRejected := string(mustRead(t, trace))
	goModBeforeRejected := string(mustRead(t, filepath.Join(dir, "go.mod")))
	for _, rejected := range []struct {
		name string
		args []string
		want string
	}{
		{"non-command directory", []string{"shared"}, "unknown target"},
		{"unknown target", []string{"missing"}, "unknown target"},
		{"duplicate target", []string{"zeta", "zeta"}, "duplicate target"},
		{"path-containing target", []string{"../cmd/zeta"}, "invalid target"},
	} {
		t.Run(rejected.name, func(t *testing.T) {
			invalid := exec.Command("/bin/bash", append([]string{"./build.sh"}, rejected.args...)...)
			invalid.Dir = dir
			invalid.Env = cmd.Env
			invalidOut, invalidErr := invalid.CombinedOutput()
			if invalidErr == nil || !strings.Contains(string(invalidOut), rejected.want) {
				t.Fatalf("rejected selector result: %v:\n%s", invalidErr, invalidOut)
			}
			if after := string(mustRead(t, trace)); after != traceBeforeRejected {
				t.Fatalf("rejected selector ran mutable build work:\n%s", after[len(traceBeforeRejected):])
			}
			if after := string(mustRead(t, filepath.Join(dir, "go.mod"))); after != goModBeforeRejected {
				t.Fatal("rejected selector changed go.mod")
			}
			assertInstalledUtilitiesUnchanged(t, gopath, installedBeforeFailures)
		})
	}
	if _, statErr := os.Stat(filepath.Join(gopath, "bin", "shared")); !os.IsNotExist(statErr) {
		t.Fatalf("scoped shared package produced an installed utility: %v", statErr)
	}

	widgetMain := filepath.Join(dir, "cmd/widget/main.go")
	validWidgetMain := mustRead(t, widgetMain)
	writeBuildFixture(t, widgetMain, []byte("package main\n\nconst programVersion = \"invalid\"\n\nfunc main() {}\n"), 0o644)
	traceBeforeDeclarationFailure := string(mustRead(t, trace))
	declarationFailure := exec.Command("/bin/bash", "./build.sh")
	declarationFailure.Dir = dir
	declarationFailure.Env = cmd.Env
	declarationOut, declarationErr := declarationFailure.CombinedOutput()
	writeBuildFixture(t, widgetMain, validWidgetMain, 0o644)
	if declarationErr == nil || !strings.Contains(string(declarationOut), "invalid utility version") {
		t.Fatalf("rendered invalid declaration: %v:\n%s", declarationErr, declarationOut)
	}
	if after := string(mustRead(t, trace)); after != traceBeforeDeclarationFailure {
		t.Fatalf("invalid declaration ran mutable build work:\n%s", after[len(traceBeforeDeclarationFailure):])
	}
	assertInstalledUtilitiesUnchanged(t, gopath, installedBeforeFailures)
	for _, failure := range []struct {
		name string
		env  string
		want string
	}{
		{"test", "FAKE_TEST_FAIL=1", "injected test failure"},
		{"coverage", "FAKE_COVER_FAIL=1", "injected coverage failure"},
		{"installation", "FAKE_MV_FAIL=1", "injected install failure"},
	} {
		t.Run(failure.name+" failure cleanup", func(t *testing.T) {
			failed := exec.Command("/bin/bash", "./build.sh")
			failed.Dir = dir
			failed.Env = append(cmd.Env, failure.env)
			failedOut, failedErr := failed.CombinedOutput()
			if failedErr == nil || !strings.Contains(string(failedOut), failure.want) {
				t.Fatalf("%s failure result: %v:\n%s", failure.name, failedErr, failedOut)
			}
			assertInstalledUtilitiesUnchanged(t, gopath, installedBeforeFailures)
			assertNoGoBuildArtifacts(t, tmpRoot, gopath)
		})
	}

	compileFailure := exec.Command("/bin/bash", "./build.sh")
	compileFailure.Dir = dir
	compileFailure.Env = append(cmd.Env, "FAKE_BUILD_FAIL_TARGET=widget")
	compileOut, compileErr := compileFailure.CombinedOutput()
	if compileErr == nil || !strings.Contains(string(compileOut), "injected build failure") {
		t.Fatalf("rendered compilation failure: %v:\n%s", compileErr, compileOut)
	}
	assertInstalledUtilitiesUnchanged(t, gopath, installedBeforeFailures)

	invalidVersion := exec.Command("/bin/bash", "./build.sh")
	invalidVersion.Dir = dir
	invalidVersion.Env = append(cmd.Env, "FAKE_BAD_VERSION_TARGET=widget")
	invalidOut, invalidErr := invalidVersion.CombinedOutput()
	if invalidErr == nil || !strings.Contains(string(invalidOut), "--version output") {
		t.Fatalf("rendered invalid compiled version: %v:\n%s", invalidErr, invalidOut)
	}
	assertInstalledUtilitiesUnchanged(t, gopath, installedBeforeFailures)
	assertNoGoBuildArtifacts(t, tmpRoot, gopath)
}

func assertInstalledUtilitiesUnchanged(t *testing.T, gopath string, expected map[string]string) {
	t.Helper()
	for utility, want := range expected {
		if got := string(mustRead(t, filepath.Join(gopath, "bin", utility))); got != want {
			t.Fatalf("failed build replaced installed %s", utility)
		}
	}
}

func assertNoGoBuildArtifacts(t *testing.T, tmpRoot, gopath string) {
	t.Helper()
	var artifacts []string
	if err := filepath.Walk(tmpRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := info.Name()
		if strings.HasPrefix(name, "govna-go-build.") || strings.HasPrefix(name, "build-cover.") || strings.HasPrefix(name, "govna-version.") {
			artifacts = append(artifacts, path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	installArtifacts, err := filepath.Glob(filepath.Join(gopath, "bin", ".govna-install-*"))
	if err != nil {
		t.Fatal(err)
	}
	artifacts = append(artifacts, installArtifacts...)
	if len(artifacts) != 0 {
		t.Fatalf("owned build artifacts remain: %v", artifacts)
	}
}

func TestGoBuildSignalsTerminateAndCleanOwnedArtifacts(t *testing.T) {
	root := repoRoot(t)
	scripts := map[string][]byte{
		"root":     mustRead(t, filepath.Join(root, "build.sh")),
		"rendered": mustRead(t, filepath.Join(root, "internal/canon/assets/overlays/code/stacks/go/build.sh.tmpl")),
	}
	interruptions := []struct {
		phase      string
		signal     syscall.Signal
		exitStatus int
	}{
		{"coverage", syscall.SIGHUP, 129},
		{"version", syscall.SIGINT, 130},
		{"install", syscall.SIGTERM, 143},
	}
	for scriptName, script := range scripts {
		for _, interruption := range interruptions {
			t.Run(scriptName+"_"+interruption.phase, func(t *testing.T) {
				dir := t.TempDir()
				writeBuildFixture(t, filepath.Join(dir, "build.sh"), script, 0o755)
				writeBuildFixture(t, filepath.Join(dir, "go.mod"), []byte("module example.com/widget\n\ngo 1.27.0\n"), 0o644)
				writeBuildFixture(t, filepath.Join(dir, "cmd/widget/main.go"), []byte("package main\n\nconst programVersion = \"1.2.3\"\n\nfunc main() {}\n"), 0o644)
				writeBuildFixture(t, filepath.Join(dir, "internal/domain/domain.go"), []byte("package domain\n"), 0o644)

				fakeBin := filepath.Join(dir, "fakebin")
				gopath := filepath.Join(dir, "gopath")
				tmpRoot := filepath.Join(dir, "tmp")
				ready := filepath.Join(dir, "ready")
				for _, path := range []string{fakeBin, filepath.Join(gopath, "bin"), tmpRoot} {
					if err := os.MkdirAll(path, 0o755); err != nil {
						t.Fatal(err)
					}
				}
				fakeGo := `#!/bin/bash
set -euo pipefail
pause_phase() {
  [ "${PAUSE_PHASE:-}" = "$1" ] || return 0
  : >"$READY_FILE"
  while :; do /bin/sleep 1; done
}
case "$1" in
list) printf '%s\n' 'example.com/widget' ;;
env)
  case "$2" in
  GOPATH) printf '%s\n' "$FAKE_GOPATH" ;;
  GOEXE) printf '\n' ;;
  *) exit 2 ;;
  esac
  ;;
mod) [ "$2" = tidy ] ;;
fmt|fix|vet) ;;
test)
  cover=''
  for arg in "$@"; do case "$arg" in -coverprofile=*) cover="${arg#-coverprofile=}" ;; esac; done
  [ -n "$cover" ]
  printf '%s\n' 'mode: set' 'example.com/widget/internal/domain/domain.go:1.1,1.2 1 1' >"$cover"
  pause_phase coverage
  ;;
tool)
  [ "$2" = cover ]
  printf '%s\n' 'example.com/widget/internal/domain/domain.go:1: f 100.0%' 'total: (statements) 100.0%'
  ;;
install)
  [ "$2" = 'honnef.co/go/tools/cmd/staticcheck@v0.8.1' ]
  printf '%s\n' '#!/bin/bash' 'exit 0' >"$FAKE_GOPATH/bin/staticcheck"
  chmod 0755 "$FAKE_GOPATH/bin/staticcheck"
  ;;
build)
  output=''
  shift
  while [ "$#" -gt 0 ]; do
    if [ "$1" = -o ]; then shift; output="$1"; fi
    shift
  done
  [ -n "$output" ]
  cat >"$output" <<'EOF'
#!/bin/bash
if [ "${1:-}" = --version ]; then
  if [ "${PAUSE_PHASE:-}" = version ]; then
    : >"$READY_FILE"
    while :; do /bin/sleep 1; done
  fi
  printf 'widget 1.2.3\n'
  exit 0
fi
exit 2
EOF
  chmod 0755 "$output"
  ;;
*) exit 2 ;;
esac
`
				fakeChmod := `#!/bin/bash
if [ "${PAUSE_PHASE:-}" = install ]; then
  case "$*" in
  *'.govna-install-'*)
    : >"$READY_FILE"
    while :; do /bin/sleep 1; done
    ;;
  esac
fi
exec /bin/chmod "$@"
`
				writeBuildFixture(t, filepath.Join(fakeBin, "go"), []byte(fakeGo), 0o755)
				writeBuildFixture(t, filepath.Join(fakeBin, "chmod"), []byte(fakeChmod), 0o755)

				cmd := exec.Command("/bin/bash", "./build.sh", "widget")
				cmd.Dir = dir
				cmd.Env = append(os.Environ(),
					"NO_COLOR=1",
					"TERM=dumb",
					"GOVNA_FORCE_TTY=0",
					"FAKE_GOPATH="+gopath,
					"PAUSE_PHASE="+interruption.phase,
					"READY_FILE="+ready,
					"TMPDIR="+tmpRoot,
					"PATH="+fakeBin+":"+os.Getenv("PATH"),
				)
				cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				var output bytes.Buffer
				cmd.Stdout = &output
				cmd.Stderr = &output
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				waited := make(chan error, 1)
				go func() { waited <- cmd.Wait() }()
				deadline := time.Now().Add(5 * time.Second)
				for {
					if _, err := os.Stat(ready); err == nil {
						break
					} else if !os.IsNotExist(err) {
						t.Fatal(err)
					}
					select {
					case err := <-waited:
						t.Fatalf("build exited before %s interruption: %v:\n%s", interruption.phase, err, output.String())
					default:
					}
					if time.Now().After(deadline) {
						_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
						<-waited
						t.Fatalf("build did not reach %s interruption point:\n%s", interruption.phase, output.String())
					}
					time.Sleep(10 * time.Millisecond)
				}
				if err := syscall.Kill(-cmd.Process.Pid, interruption.signal); err != nil {
					t.Fatal(err)
				}
				var runErr error
				select {
				case runErr = <-waited:
				case <-time.After(5 * time.Second):
					_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
					<-waited
					t.Fatalf("build did not terminate after %s:\n%s", interruption.signal, output.String())
				}
				exitErr, ok := runErr.(*exec.ExitError)
				if !ok {
					t.Fatalf("signal result=%v, want exit %d:\n%s", runErr, interruption.exitStatus, output.String())
				}
				if exitErr.ExitCode() != interruption.exitStatus {
					t.Fatalf("signal exit=%d, want %d:\n%s", exitErr.ExitCode(), interruption.exitStatus, output.String())
				}
				if strings.Contains(output.String(), "installed:") {
					t.Fatalf("build continued through installation after signal:\n%s", output.String())
				}
				assertNoGoBuildArtifacts(t, tmpRoot, gopath)
			})
		}
	}
}

func utilityBuildTargets(trace string) []string {
	targets := []string{}
	for line := range strings.SplitSeq(trace, "\n") {
		if !strings.HasPrefix(line, "build ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 0 {
			targets = append(targets, fields[len(fields)-1])
		}
	}
	return targets
}

func utilityBuildOutputs(trace string) []string {
	outputs := []string{}
	for line := range strings.SplitSeq(trace, "\n") {
		if !strings.HasPrefix(line, "build ") {
			continue
		}
		fields := strings.Fields(line)
		for index, field := range fields {
			if field == "-o" && index+1 < len(fields) {
				outputs = append(outputs, fields[index+1])
				break
			}
		}
	}
	return outputs
}

func utilityInstallMoves(trace string) map[string][2]string {
	moves := map[string][2]string{}
	for line := range strings.SplitSeq(trace, "\n") {
		if !strings.HasPrefix(line, "mv ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 4 || fields[1] != "-f" {
			continue
		}
		moves[filepath.Base(fields[3])] = [2]string{fields[2], fields[3]}
	}
	return moves
}

func TestRenderedGoVersionHelpers(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	writeBuildFixture(t, filepath.Join(dir, "build.sh"), mustRead(t, filepath.Join(root, "internal/canon/assets/overlays/code/stacks/go/build.sh.tmpl")), 0o755)
	writeBuildFixture(t, filepath.Join(dir, "single.go"), []byte("package main\nconst programVersion string = \"1.2.3\"\n"), 0o644)
	writeBuildFixture(t, filepath.Join(dir, "grouped.go"), []byte("package main\nconst (\n\tprogramVersion = \"2.3.4\"\n)\n"), 0o644)
	check := `source ./build.sh
[ "$(_extract_program_version single.go)" = 1.2.3 ]
[ "$(_extract_program_version grouped.go)" = 2.3.4 ]
`
	if out, err := run(t, dir, "", "-c", check); err != nil {
		t.Fatalf("version extraction: %v: %s", err, out)
	}
	for _, version := range []string{"0.0.0", "1.2.3", "10.20.30"} {
		if out, err := run(t, dir, "", "-c", `source ./build.sh; _is_strict_stable_semver "$1"`, "fixture", version); err != nil {
			t.Errorf("valid SemVer %q rejected: %v: %s", version, err, out)
		}
	}
	for _, version := range []string{"01.2.3", "1.02.3", "1.2.03", "1.2.3-rc.1", "1.2.3+meta", "1.2", "v1.2.3", ""} {
		if _, err := run(t, dir, "", "-c", `source ./build.sh; _is_strict_stable_semver "$1"`, "fixture", version); err == nil {
			t.Errorf("invalid SemVer %q accepted", version)
		}
	}
}

func TestRenderedGoPrepDryRunValidatesEveryUtility(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	writeBuildFixture(t, filepath.Join(dir, "build.sh"), mustRead(t, filepath.Join(root, "internal/canon/assets/overlays/code/stacks/go/build.sh.tmpl")), 0o755)
	writeBuildFixture(t, filepath.Join(dir, "go.mod"), []byte("module example.com/widget\n\ngo 1.27.0\n"), 0o644)
	writeBuildFixture(t, filepath.Join(dir, "cmd/alpha/main.go"), []byte("package main\nconst programVersion = \"1.2.3\"\n"), 0o644)
	writeBuildFixture(t, filepath.Join(dir, "cmd/beta/main.go"), []byte("package main\nconst (\n\tprogramVersion string = \"2.3.4\"\n)\n"), 0o644)
	writeBuildFixture(t, filepath.Join(dir, "CHANGELOG.md"), []byte("# Changelog\n\n| Version | Summary |\n|---|---|\n| Unreleased | |\n"), 0o644)
	gitFixture(t, dir, "init", "-q")
	gitFixture(t, dir, "config", "user.name", "Fixture")
	gitFixture(t, dir, "config", "user.email", "fixture@example.com")
	gitFixture(t, dir, "add", ".")
	gitFixture(t, dir, "commit", "-qm", "fixture")
	before, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	out, runErr := run(t, dir, "", "./build.sh", "prep", "v3.0.0", "AC14 fixture", "--dry-run", "--no-build")
	if runErr != nil {
		t.Fatalf("prep dry-run: %v: %s", runErr, out)
	}
	for _, want := range []string{"multi-utility repo detected (2 programVersion targets", "cmd/alpha/main.go", "cmd/beta/main.go", "release command:"} {
		if !strings.Contains(out, want) {
			t.Errorf("prep output omits %q: %s", want, out)
		}
	}
	after, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("prep dry-run changed fixture: before=%q after=%q", before, after)
	}
}

func TestReleaseCancellationIsNonMutating(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	script, _ := os.ReadFile(filepath.Join(root, "build.sh"))
	if err := os.WriteFile(filepath.Join(dir, "build.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	before, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	out, err := run(t, dir, "n\n", "./build.sh", "v1.0.0", "cancel")
	if err == nil || !strings.Contains(out, "release aborted") {
		t.Fatalf("unexpected cancellation: %v: %s", err, out)
	}
	after, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if string(before) != string(after) {
		t.Fatalf("cancellation changed Git state")
	}
}

func TestReleaseApprovalAndFailureOrdering(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	script, _ := os.ReadFile(filepath.Join(root, "build.sh"))
	if err := os.WriteFile(filepath.Join(dir, "build.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("init: %v: %s", err, out)
	}
	approved := `source ./build.sh
_run_git() { shift; printf '%s\n' "$*" >> operations; return 0; }
_release_rebuild_and_verify() { printf '%s\n' provenance >> operations; return 0; }
printf 'y\n' | rel_run v1.2.3 release
`
	out, err := run(t, dir, "", "-c", approved)
	if err != nil {
		t.Fatalf("approved release fixture: %v: %s", err, out)
	}
	operations, err := os.ReadFile(filepath.Join(dir, "operations"))
	if err != nil {
		t.Fatal(err)
	}
	want := "status --short\nadd .\ncommit -m release\ntag v1.2.3\nprovenance\npush origin v1.2.3\npush origin\n"
	if string(operations) != want {
		t.Fatalf("operations:\n%s\nwant:\n%s", operations, want)
	}

	failing := `source ./build.sh
_run_git() { name="$1"; shift; printf '%s\n' "$*" >> failed-operations; if [ "$name" = 'git tag' ]; then _git_err='git tag failed: exit status 9'; return 1; fi; return 0; }
printf 'y\n' | rel_run v1.2.3 release
`
	out, err = run(t, dir, "", "-c", failing)
	if err == nil || !strings.Contains(out, "completed before failure: git add, git commit") {
		t.Fatalf("failed release fixture: %v: %s", err, out)
	}
	failed, err := os.ReadFile(filepath.Join(dir, "failed-operations"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(failed), "push origin") {
		t.Fatalf("release continued after failure: %s", failed)
	}

	provenanceFailure := `source ./build.sh
_run_git() { shift; printf '%s\n' "$*" >> provenance-failed-operations; return 0; }
_release_rebuild_and_verify() { return 1; }
printf 'y\n' | rel_run v1.2.3 release
`
	if out, err = run(t, dir, "", "-c", provenanceFailure); err == nil {
		t.Fatalf("provenance failure accepted: %s", out)
	}
	provenanceFailed, err := os.ReadFile(filepath.Join(dir, "provenance-failed-operations"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(provenanceFailed), "push origin") {
		t.Fatalf("release pushed after provenance failure: %s", provenanceFailed)
	}
}

func TestCanonAssetChangesRequireVersionIncrease(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	for path, content := range map[string]string{
		"build.sh":                    string(mustRead(t, filepath.Join(root, "build.sh"))),
		"internal/canon/canon.go":     "package canon\nconst Version = \"1.0.0\"\n",
		"cmd/govna/main.go":           "package main\nconst canonVersion = \"1.0.0\"\n",
		"internal/canon/assets/a.txt": "one\n",
	} {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitFixture(t, dir, "init", "-q")
	gitFixture(t, dir, "config", "user.name", "Fixture")
	gitFixture(t, dir, "config", "user.email", "fixture@example.com")
	gitFixture(t, dir, "add", ".")
	gitFixture(t, dir, "commit", "-qm", "baseline")
	gitFixture(t, dir, "tag", "v1.0.0")
	if err := os.WriteFile(filepath.Join(dir, "internal/canon/assets/a.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, dir, "", "-c", "source ./build.sh; _color_init; _validate_root_canon_version")
	if err == nil || !strings.Contains(out, "increase internal/canon.Version") {
		t.Fatalf("unchanged version accepted: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "internal/canon/canon.go"), []byte("package canon\nconst Version = \"1.1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "cmd/govna/main.go"), []byte("package main\nconst canonVersion = \"1.1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, dir, "", "-c", "source ./build.sh; _color_init; _validate_root_canon_version"); err != nil {
		t.Fatalf("increased version rejected: %v: %s", err, out)
	}
}

func TestTaggedBinaryProvenanceVerification(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	tooling := mustRead(t, filepath.Join(root, "build.sh"))
	if err := os.WriteFile(filepath.Join(dir, "tooling.sh"), tooling, 0o644); err != nil {
		t.Fatal(err)
	}
	gopath := filepath.Join(dir, "gopath")
	fakebin := filepath.Join(dir, "fakebin")
	if err := os.MkdirAll(fakebin, 0o755); err != nil {
		t.Fatal(err)
	}
	build := "#!/bin/bash\nmkdir -p \"$FAKE_GOPATH/bin\"\nprintf '#!/bin/bash\\nprintf \\\"govna v1.2.3\\\\n\\\"\\n' > \"$FAKE_GOPATH/bin/govna\"\nchmod +x \"$FAKE_GOPATH/bin/govna\"\n"
	if err := os.WriteFile(filepath.Join(dir, "build.sh"), []byte(build), 0o755); err != nil {
		t.Fatal(err)
	}
	goTool := "#!/bin/bash\nif [ \"$1\" = env ]; then printf '%s\\n' \"$FAKE_GOPATH\"; exit; fi\nif [ \"$1\" = version ]; then printf 'path\\tgovna\\nbuild\\tvcs.revision=%s\\nbuild\\tvcs.modified=false\\n' \"$(git rev-parse HEAD)\"; exit; fi\nexit 1\n"
	if err := os.WriteFile(filepath.Join(fakebin, "go"), []byte(goTool), 0o755); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, dir, "init", "-q")
	gitFixture(t, dir, "config", "user.name", "Fixture")
	gitFixture(t, dir, "config", "user.email", "fixture@example.com")
	gitFixture(t, dir, "add", ".")
	gitFixture(t, dir, "commit", "-qm", "release")
	cmd := exec.Command("/bin/bash", "-c", "source ./tooling.sh; _color_init; _release_rebuild_and_verify v1.2.3")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb", "FAKE_GOPATH="+gopath, "PATH="+fakebin+":"+os.Getenv("PATH"))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("provenance rejected: %v: %s", err, out)
	}
}

func TestUtilityDeclarationValidationAndAtomicInstall(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tooling.sh"), mustRead(t, filepath.Join(root, "build.sh")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nconst programVersion = \"1.2.3\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	compiled := filepath.Join(dir, "compiled")
	if err := os.WriteFile(compiled, []byte("#!/bin/bash\nprintf 'widget 1.2.3\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "bin", "widget")
	if err := os.Mkdir(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	script := "source ./tooling.sh; _color_init; [ \"$(_extract_program_version main.go)\" = 1.2.3 ]; _install_compiled_utility ./compiled ./bin/widget widget 1.2.3"
	if out, err := run(t, dir, "", "-c", script); err != nil {
		t.Fatalf("install failed: %v: %s", err, out)
	}
	if out, err := exec.Command(destination, "--version").CombinedOutput(); err != nil || string(out) != "widget 1.2.3\n" {
		t.Fatalf("installed output: %v: %s", err, out)
	}
	installedBeforeFailure := string(mustRead(t, destination))
	if err := os.WriteFile(compiled, []byte("#!/bin/bash\nprintf 'widget 9.9.9\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, dir, "", "-c", "source ./tooling.sh; _color_init; _install_compiled_utility ./compiled ./bin/widget widget 1.2.3"); err == nil || !strings.Contains(out, "--version output") {
		t.Fatalf("invalid compiled version accepted: %v: %s", err, out)
	}
	if installedAfterFailure := string(mustRead(t, destination)); installedAfterFailure != installedBeforeFailure {
		t.Fatal("failed root validation replaced the installed utility")
	}
	if err := os.WriteFile(compiled, []byte("#!/bin/bash\nprintf 'widget 1.2.3\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, failure := range []struct {
		name    string
		command string
	}{
		{"staging copy", "cp"},
		{"staging permission", "chmod"},
		{"atomic rename", "mv"},
	} {
		t.Run(failure.name, func(t *testing.T) {
			fixture := "source ./tooling.sh; _color_init; " + failure.command + "() { return 9; }; _install_compiled_utility ./compiled ./bin/widget widget 1.2.3"
			if out, err := run(t, dir, "", "-c", fixture); err == nil {
				t.Fatalf("injected %s failure accepted: %s", failure.name, out)
			}
			if got := string(mustRead(t, destination)); got != installedBeforeFailure {
				t.Fatalf("%s failure replaced the installed utility", failure.name)
			}
			matches, err := filepath.Glob(filepath.Join(dir, "bin", ".govna-install-*"))
			if err != nil || len(matches) != 0 {
				t.Fatalf("%s staging files=%v err=%v", failure.name, matches, err)
			}
		})
	}
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("compiled", destination); err != nil {
		t.Fatal(err)
	}
	if out, err := run(t, dir, "", "-c", "source ./tooling.sh; _color_init; _install_compiled_utility ./compiled ./bin/widget widget 1.2.3"); err == nil || !strings.Contains(out, "must be absent or a regular file") {
		t.Fatalf("unsafe destination accepted: %v: %s", err, out)
	}
}

func TestPreparationMutationFailureAndCleanup(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tooling.sh"), mustRead(t, filepath.Join(root, "build.sh")), 0o644); err != nil {
		t.Fatal(err)
	}
	versionFile := filepath.Join(dir, "main.go")
	if err := os.WriteFile(versionFile, []byte("package main\nconst programVersion = \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	owned := filepath.Join(dir, "owned")
	if err := os.Mkdir(owned, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "source ./tooling.sh; _color_init; _prep_apply_version_bump main.go programVersion 1.1.0; _build_owned_dir=owned; _cleanup_build_owned; ! _prep_apply_version_bump main.go unknown 2.0.0"
	if out, err := run(t, dir, "", "-c", script); err != nil {
		t.Fatalf("prep fixture failed: %v: %s", err, out)
	}
	content := string(mustRead(t, versionFile))
	if !strings.Contains(content, `programVersion = "1.1.0"`) || strings.Contains(content, "2.0.0") {
		t.Fatalf("version mutation=%s", content)
	}
	if _, err := os.Stat(owned); !os.IsNotExist(err) {
		t.Fatalf("owned scratch remains: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".govna-install-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("install temporaries=%v err=%v", matches, err)
	}
}

type shellHelperClosure struct {
	definitions map[string]int
	references  map[string]bool
}

type shellLexState struct {
	quote            byte
	heredoc          string
	heredocQuoted    bool
	heredocStripTabs bool
}

var shellFunctionDefinitionRE = regexp.MustCompile(`^[\t ]*(_[A-Za-z][A-Za-z0-9_]*)[\t ]*\(\)[\t ]*\{`)
var shellDirectStartRE = regexp.MustCompile(`^[\t ]*((if|elif|while|until|then|do|else)[\t ]+)?(![\t ]+)?(?P<helper>_[A-Za-z][A-Za-z0-9_]*)`)
var shellDirectBoundaryRE = regexp.MustCompile(`[;|&{}(][\t ]*((if|elif|while|until|then|do|else)[\t ]+)?(![\t ]+)?(?P<helper>_[A-Za-z][A-Za-z0-9_]*)`)

func scanShellHelperClosure(script string) shellHelperClosure {
	closure := shellHelperClosure{definitions: map[string]int{}, references: map[string]bool{}}
	state := shellLexState{}
	for line := range strings.SplitSeq(script, "\n") {
		if state.heredoc != "" {
			candidate := line
			if state.heredocStripTabs {
				candidate = strings.TrimLeft(candidate, "\t")
			}
			if candidate == state.heredoc {
				state.heredoc = ""
				state.heredocQuoted = false
				state.heredocStripTabs = false
				continue
			}
			if !state.heredocQuoted {
				scanHeredocSubstitutions(line, closure.references)
			}
			continue
		}

		code := maskShellLine(line, &state.quote, closure.references)
		if match := shellFunctionDefinitionRE.FindStringSubmatch(code); match != nil {
			closure.definitions[match[1]]++
		}
		addDirectShellHelpers(code, closure.references)
		if delimiter, quoted, stripTabs, ok := shellHeredocStart(line, code); ok {
			state.heredoc = delimiter
			state.heredocQuoted = quoted
			state.heredocStripTabs = stripTabs
		}
	}
	return closure
}

func (closure shellHelperClosure) validate() error {
	issues := []string{}
	definitionNames := make([]string, 0, len(closure.definitions))
	for name := range closure.definitions {
		definitionNames = append(definitionNames, name)
	}
	sort.Strings(definitionNames)
	for _, name := range definitionNames {
		if closure.definitions[name] > 1 {
			issues = append(issues, fmt.Sprintf("duplicate helper %s has %d definitions", name, closure.definitions[name]))
		}
	}
	for _, name := range closure.referenceNames() {
		if closure.definitions[name] == 0 {
			issues = append(issues, "undefined helper "+name)
		}
	}
	if len(issues) != 0 {
		return fmt.Errorf("shell helper closure: %s", strings.Join(issues, "; "))
	}
	return nil
}

func (closure shellHelperClosure) referenceNames() []string {
	names := make([]string, 0, len(closure.references))
	for name := range closure.references {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func maskShellLine(line string, quote *byte, references map[string]bool) string {
	masked := []byte(strings.Repeat(" ", len(line)))
	for index := 0; index < len(line); index++ {
		char := line[index]
		if *quote != 0 {
			if *quote == '"' && char == '$' && index+1 < len(line) && line[index+1] == '(' {
				end := shellMatchingParen(line, index+1)
				if end == len(line) {
					continue
				}
				if index+2 >= len(line) || line[index+2] != '(' {
					scanShellFragment(line[index+2:end], references)
				}
				index = end
				continue
			}
			if char == '\\' && *quote != '\'' && index+1 < len(line) {
				index++
				continue
			}
			if char == *quote {
				*quote = 0
			}
			continue
		}

		if char == '#' && shellCommentStart(line, index) {
			break
		}
		if char == '\'' || char == '"' || char == '`' {
			*quote = char
			continue
		}
		if char == '\\' && index+1 < len(line) {
			index++
			continue
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '(' {
			end := shellMatchingParen(line, index+1)
			if end == len(line) {
				masked[index] = char
				masked[index+1] = line[index+1]
				index++
				continue
			}
			if index+2 >= len(line) || line[index+2] != '(' {
				scanShellFragment(line[index+2:end], references)
			}
			index = end
			continue
		}
		if (char == '<' || char == '>') && index+1 < len(line) && line[index+1] == '(' {
			end := shellMatchingParen(line, index+1)
			if end == len(line) {
				masked[index] = char
				masked[index+1] = line[index+1]
				index++
				continue
			}
			scanShellFragment(line[index+2:end], references)
			index = end
			continue
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '{' {
			if end := shellMatchingBrace(line, index+1); end < len(line) {
				index = end
				continue
			}
		}
		masked[index] = char
	}
	return string(masked)
}

func scanShellFragment(fragment string, references map[string]bool) {
	quote := byte(0)
	code := maskShellLine(fragment, &quote, references)
	addDirectShellHelpers(code, references)
}

func scanHeredocSubstitutions(line string, references map[string]bool) {
	for index := 0; index+1 < len(line); index++ {
		if line[index] == '\\' {
			index++
			continue
		}
		if line[index] != '$' || line[index+1] != '(' {
			continue
		}
		end := shellMatchingParen(line, index+1)
		if index+2 >= len(line) || line[index+2] != '(' {
			scanShellFragment(line[index+2:end], references)
		}
		index = end
	}
}

func shellMatchingParen(line string, open int) int {
	depth := 0
	quote := byte(0)
	for index := open; index < len(line); index++ {
		char := line[index]
		if quote != 0 {
			if char == '\\' && quote != '\'' && index+1 < len(line) {
				index++
				continue
			}
			if char == quote {
				quote = 0
			}
			continue
		}
		if char == '\'' || char == '"' || char == '`' {
			quote = char
			continue
		}
		if char == '\\' && index+1 < len(line) {
			index++
			continue
		}
		switch char {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return len(line)
}

func shellMatchingBrace(line string, open int) int {
	depth := 0
	for index := open; index < len(line); index++ {
		if line[index] == '\\' && index+1 < len(line) {
			index++
			continue
		}
		switch line[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return len(line)
}

func shellCommentStart(line string, index int) bool {
	if index == 0 {
		return true
	}
	return strings.ContainsRune(" \t;|&(){}", rune(line[index-1]))
}

func addDirectShellHelpers(code string, references map[string]bool) {
	for _, expression := range []*regexp.Regexp{shellDirectStartRE, shellDirectBoundaryRE} {
		helperGroup := expression.SubexpIndex("helper")
		for _, match := range expression.FindAllStringSubmatchIndex(code, -1) {
			start := match[helperGroup*2]
			end := match[helperGroup*2+1]
			if start < 0 || end < 0 {
				continue
			}
			if end < len(code) && strings.ContainsRune("=([+-", rune(code[end])) {
				continue
			}
			references[code[start:end]] = true
		}
	}
}

func shellHeredocStart(line, code string) (string, bool, bool, bool) {
	for index := 0; index+1 < len(code); index++ {
		if code[index] != '<' || code[index+1] != '<' || index+2 < len(code) && code[index+2] == '<' {
			continue
		}
		cursor := index + 2
		stripTabs := false
		if cursor < len(line) && line[cursor] == '-' {
			stripTabs = true
			cursor++
		}
		for cursor < len(line) && (line[cursor] == ' ' || line[cursor] == '\t') {
			cursor++
		}
		if cursor >= len(line) {
			return "", false, false, false
		}
		quoted := false
		quote := byte(0)
		if line[cursor] == '\'' || line[cursor] == '"' {
			quoted = true
			quote = line[cursor]
			cursor++
		} else if line[cursor] == '\\' {
			quoted = true
			cursor++
		}
		start := cursor
		for cursor < len(line) {
			if quote != 0 {
				if line[cursor] == quote {
					break
				}
			} else if strings.ContainsRune(" \t;|&<>", rune(line[cursor])) {
				break
			}
			cursor++
		}
		if cursor > start {
			return line[start:cursor], quoted, stripTabs, true
		}
	}
	return "", false, false, false
}

func writeBuildFixture(t *testing.T, path string, content []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, mode); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func gitFixture(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
