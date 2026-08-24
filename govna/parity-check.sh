#!/bin/sh

set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
index_path="$repo_root/govna/parity-index.txt"
contract_path="$repo_root/govna/parity.md"

die() {
  printf '%s\n' "parity-check: $*" >&2
  exit 1
}

integration_names() {
  awk '
    /^#\[test\]$/ { want_name = 1; next }
    want_name && /^fn [A-Za-z0-9_]+\(\)/ {
      name = $0
      sub(/^fn /, "", name)
      sub(/\(.*/, "", name)
      print name
      want_name = 0
    }
  ' "$1/tests/govna_cli.rs"
}

build_tool_names() {
  awk '/^test_[A-Za-z0-9_]+$/ { print }' "$1/tests/build_cli.sh"
}

generate_index() {
  reference=$1
  [ -f "$reference/tests/govna_cli.rs" ] ||
    die "missing Rust integration tests under $reference"
  [ -f "$reference/tests/build_cli.sh" ] ||
    die "missing Rust build-tool tests under $reference"

  commit=$(git -C "$reference" rev-parse HEAD 2>/dev/null) ||
    die "cannot resolve Rust reference commit under $reference"
  [ "$commit" = 7416cb919b48284f2db45adc99875bddfdb87564 ] ||
    die "Rust reference commit is $commit, expected 7416cb919b48284f2db45adc99875bddfdb87564"
  git -C "$reference" tag --points-at HEAD | grep -Fxq v0.37.1 ||
    die "Rust reference HEAD is not tagged v0.37.1"

  integration=$(integration_names "$reference")
  build_tools=$(build_tool_names "$reference")
  integration_count=$(printf '%s\n' "$integration" | awk 'NF { count++ } END { print count + 0 }')
  build_count=$(printf '%s\n' "$build_tools" | awk 'NF { count++ } END { print count + 0 }')
  [ "$integration_count" -eq 128 ] ||
    die "found $integration_count Rust integration tests, expected 128"
  [ "$build_count" -eq 18 ] ||
    die "found $build_count invoked build-tool tests, expected 18"

  printf '%s\n' \
    'govna-parity-index-v1' \
    'repository queone/govna-rust' \
    'tag v0.37.1' \
    'commit 7416cb919b48284f2db45adc99875bddfdb87564' \
    'integration-count 128' \
    'build-tool-count 18' \
    '' \
    '[integration-tests]'
  printf '%s\n' "$integration"
  printf '%s\n' '' '[build-tool-tests]'
  printf '%s\n' "$build_tools"
}

if [ "${1:-}" = --generate ]; then
  [ "$#" -eq 2 ] || die 'usage: govna/parity-check.sh --generate <rust-checkout>'
  generate_index "$2"
  exit 0
fi
[ "$#" -eq 0 ] || die 'usage: govna/parity-check.sh [--generate <rust-checkout>]'

[ -f "$index_path" ] || die 'missing govna/parity-index.txt'
[ -f "$contract_path" ] || die 'missing govna/parity.md'

grep -Fxq 'govna-parity-index-v1' "$index_path" || die 'invalid parity-index schema'
grep -Fxq 'repository queone/govna-rust' "$index_path" || die 'invalid reference repository'
grep -Fxq 'tag v0.37.1' "$index_path" || die 'invalid reference tag'
grep -Fxq 'commit 7416cb919b48284f2db45adc99875bddfdb87564' "$index_path" ||
  die 'invalid reference commit'
grep -Fxq 'integration-count 128' "$index_path" || die 'invalid integration-test count header'
grep -Fxq 'build-tool-count 18' "$index_path" || die 'invalid build-tool-test count header'

actual_integration=$(awk '
  /^\[integration-tests\]$/ { section = 1; next }
  /^\[build-tool-tests\]$/ { section = 0 }
  section && NF { count++ }
  END { print count + 0 }
' "$index_path")
actual_build=$(awk '
  /^\[build-tool-tests\]$/ { section = 1; next }
  section && NF { count++ }
  END { print count + 0 }
' "$index_path")
[ "$actual_integration" -eq 128 ] ||
  die "parity index contains $actual_integration integration tests, expected 128"
[ "$actual_build" -eq 18 ] ||
  die "parity index contains $actual_build build-tool tests, expected 18"

grep -Fq 'queone/govna-rust' "$contract_path" || die 'contract omits reference repository'
grep -Fq 'v0.37.1' "$contract_path" || die 'contract omits reference tag'
grep -Fq '7416cb919b48284f2db45adc99875bddfdb87564' "$contract_path" ||
  die 'contract omits reference commit'
grep -Fq 'current `programVersion`' "$contract_path" ||
  die 'contract omits successor program-version substitution'
grep -Fq 'reported embedded-canon version' "$contract_path" ||
  die 'contract omits successor canon-version substitution'
grep -Fq 'source repository `github.com/queone/govna`' "$contract_path" ||
  die 'contract omits successor source-repository substitution'

require_record_text() {
  record_id=$1
  expected=$2
  record_count=$(grep -Fc "| $record_id |" "$contract_path")
  [ "$record_count" -eq 1 ] || die "expected one $record_id record, found $record_count"
  grep -F "| $record_id |" "$contract_path" | grep -Fq -- "$expected" ||
    die "$record_id omits required contract text: $expected"
}

# AT4: complete command interface, stream/status, mutation, and artifact records.
require_record_text IF-001 'Write usage to stderr and exit 2.'
require_record_text IF-001 'Make no filesystem mutation.'
require_record_text IF-002 '`-h`, `--help`, `-?`, `help`, `h`'
require_record_text IF-002 'stdout and exit 0'
require_record_text IF-003 'Any unrecognized subcommand'
require_record_text IF-003 'stderr and exit 2'
require_record_text IF-004 '`--version`, `ver`, `v`'
require_record_text IF-004 'stdout and exit 0'
require_record_text IF-005 '`version`; reject additional arguments'
require_record_text IF-005 'stderr and exit 2'
require_record_text IF-006 '`render`; legacy alias `render-canon`'
require_record_text IF-006 '`-f`, `--flavor`, `-s`, `--stack`, `-m`, `--module-path`, `-h`, `--help`, `-?`'
require_record_text IF-006 'stderr and exit 0; write argument errors to stderr and exit 2; write runtime errors to stderr and exit 1'
require_record_text IF-006 'canon set, `govna/canon-baseline.txt`, and the `CLAUDE.md` symlink'
require_record_text IF-006 'do not pre-clean'
require_record_text IF-007 '`-f`, `--flavor`, `-s`, `--stack`, `-n`, `--repo-name`, `-m`, `--module-path`, `-g`, `--init-git`, `-h`, `--help`, `-?`'
require_record_text IF-007 'reject positional arguments'
require_record_text IF-007 'stderr and exit 0; write argument errors to stderr and exit 2; write runtime errors and warnings to stderr and exit 1'
require_record_text IF-007 'canon set and baseline'
require_record_text IF-007 'emit one adoption AC'
require_record_text IF-007 'initialize Git on `main` only with `-g`'
require_record_text IF-007 'never delete the legacy `governa/` tree'
require_record_text IF-008 '`audit`; legacy alias `drift-scan`'
require_record_text IF-008 '`-f`, `--flavor`, `-s`, `--stack`, `-j`, `--json`, `-l`, `--diff-lines`, `-n`, `--repo-name`, `-h`, `--help`, `-?`'
require_record_text IF-008 'reject positional arguments'
require_record_text IF-008 'stderr and exit 0; write argument errors to stderr and exit 2; write runtime or prerequisite errors to stderr and exit 1'
require_record_text IF-008 'summary or JSON to stdout and exit 0'
require_record_text IF-008 'Emit one deterministic audit AC only for actionable results'
require_record_text IF-008 'emit no AC for a clean result'
require_record_text IF-008 'mutate no pre-existing governed consumer artifact other than an eligible emitted-AC stub'
require_record_text IF-009 '`-f`, `--flavor`, `-s`, `--stack`, `-n`, `--repo-name`, `-h`, `--help`, `-?`'
require_record_text IF-009 'reject positional arguments'
require_record_text IF-009 'stderr and exit 0; write argument errors to stderr and exit 2; write runtime or prerequisite errors to stderr and exit 1'
require_record_text IF-009 'emitted-AC path to stdout and exit 0'
require_record_text IF-009 'Emit or reuse one deterministic removal AC'
require_record_text IF-009 'mutate no pre-existing governed consumer artifact other than an eligible emitted-AC stub'
printf '%s\n' 'AT4 command-surface automation: pass'

ids_path=$(mktemp "${TMPDIR:-/tmp}/govna-parity-ids.XXXXXX") ||
  die 'create temporary requirement-ID file failed'
trap 'rm -f "$ids_path"' EXIT HUP INT TERM
if ! awk -F '|' '
  function trim(value) {
    sub(/^[[:space:]]+/, "", value)
    sub(/[[:space:]]+$/, "", value)
    return value
  }
  /^\| (VER|CLI|RND|AUD|APL|REM|BLD|DIF)-[0-9][0-9][0-9] / {
    rows++
    if (NF != 9) {
      print "parity-check: malformed matrix column count at line " NR > "/dev/stderr"
      failed = 1
      next
    }
    id = trim($2)
    surface = trim($3)
    reference = trim($4)
    contract = trim($5)
    disposition = trim($6)
    stage = trim($7)
    verification = trim($8)
    if (id !~ /^[A-Z][A-ZA-Z]*-[0-9][0-9][0-9]$/ || surface == "" ||
        reference == "" || contract == "" || verification == "") {
      print "parity-check: incomplete matrix row " id > "/dev/stderr"
      failed = 1
    }
    if (disposition != "byte-exact" && disposition != "semantic" &&
        disposition != "intentional-difference" &&
        disposition != "implementation-specific" &&
        disposition != "not-applicable") {
      print "parity-check: invalid disposition for " id > "/dev/stderr"
      failed = 1
    }
    if (stage !~ /^S[1-6]$/) {
      print "parity-check: invalid primary stage for " id > "/dev/stderr"
      failed = 1
    }
    if ((disposition == "intentional-difference" ||
         disposition == "implementation-specific" ||
         disposition == "not-applicable") && verification !~ /^Reason:/) {
      print "parity-check: missing disposition reason for " id > "/dev/stderr"
      failed = 1
    }
    print id
  }
  END {
    if (rows == 0) {
      print "parity-check: traceability matrix has no rows" > "/dev/stderr"
      failed = 1
    }
    if (failed) exit 1
  }
' "$contract_path" >"$ids_path"; then
  exit 1
fi
duplicate=$(LC_ALL=C sort "$ids_path" | uniq -d | awk 'NR == 1 { print; exit }')
[ -z "$duplicate" ] || die "duplicate requirement ID $duplicate"

# AT8: preserve outcomes while excluding Rust-only product mechanics and
# leaving implementation architecture to later ACs.
require_record_text BND-001 'Go product tooling'
require_record_text BND-001 'isolated temporary-output ownership'
require_record_text BND-001 'Defer Go commands, package layout, dependency choices, build-cache layout, and tool selection to S6.'
require_record_text BND-002 'Emitted Rust consumer tooling'
require_record_text BND-002 'externally observable consumer output'
require_record_text BND-003 'Cargo manifest parsing, shared Cargo target ownership, and Cargo target cleanup'
require_record_text BND-003 'Classify those mechanics as `implementation-specific`'
require_record_text BND-004 'Defer packages, dependencies, CLI libraries, embedding strategy, data structures, concurrency, and internal control flow'
require_record_text BND-005 'Initialize `apply -g` repositories on `main`'
require_record_text BND-005 'Permit no other intentional difference'
for implementation_id in BLD-006 BLD-013 BLD-021; do
  require_record_text "$implementation_id" '| implementation-specific | S6 | Reason:'
done
implementation_count=$(grep -c '| implementation-specific |' "$contract_path")
[ "$implementation_count" -eq 3 ] ||
  die "expected 3 implementation-specific requirements, found $implementation_count"
intentional_count=$(grep -c '| intentional-difference |' "$contract_path")
[ "$intentional_count" -eq 1 ] ||
  die "expected 1 intentional difference, found $intentional_count"
if awk '/^- \*\*S[1-6]:/{print}' "$contract_path" |
  grep -Eiq 'package layout|dependenc|CLI librar|embedding strategy|data structure|concurrency|Cargo|Rust'; then
  die 'future stage definitions contain an implementation-architecture choice'
fi
printf '%s\n' 'AT8 disposition-boundary automation: pass'

section=''
while IFS= read -r line || [ -n "$line" ]; do
  case "$line" in
    '[integration-tests]') section=integration; continue ;;
    '[build-tool-tests]') section=build; continue ;;
    '['*']'|'') continue ;;
  esac
  [ -n "$section" ] || continue
  grep -Fq "| $section:$line |" "$contract_path" ||
    die "unmapped $section test $line"
done <"$index_path"

grep -Fq '| DIF-001 | Apply | integration:apply_init_git_then_skips_on_rerun |' "$contract_path" ||
  die 'missing apply -g intentional-difference requirement'
grep -F '| DIF-001 |' "$contract_path" | grep -Fq '`main`' ||
  die 'intentional difference does not require main'
grep -F '| DIF-001 |' "$contract_path" | grep -Fq '`master`' ||
  die 'intentional difference does not prohibit master'
grep -F '| DIF-001 |' "$contract_path" | grep -Fq '| S3 |' ||
  die 'intentional difference is not owned by S3'

grep -Fxq -- '- Keep Go tests beside the packages they exercise.' "$repo_root/AGENTS.md" ||
  die 'AGENTS.md does not keep Go tests beside their packages'
grep -Fxq -- '- Keep repository-specific governance verification under `govna/`.' "$repo_root/AGENTS.md" ||
  die 'AGENTS.md does not keep governance verification under govna/'
grep -Fxq -- '- Prohibit a top-level `tests/` directory.' "$repo_root/AGENTS.md" ||
  die 'AGENTS.md does not prohibit a top-level tests/ directory'
[ ! -d "$repo_root/tests" ] || die 'top-level tests/ directory exists'

printf '%s\n' "parity contract: $actual_integration integration tests and $actual_build build-tool tests mapped"
