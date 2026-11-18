# --- bare identifier scan ({{BARE_ID_SCAN_LABEL}}) ---
# Flags bare AC, AT, and IE identifiers in governed Markdown outside the places
# that the AGENTS.md bare-identifier rules allow. Governed Markdown is the root
# {{BARE_ID_SCAN_FILE_NAMES}}, plus
# every Markdown file under govna/. A link counts by its own path. Every other
# Markdown file is outside the scan. A child repository gets no allowance, as
# its parent's AGENTS.md requires. Class, Part, and Round numbers are not
# scanned because canon defines no written form for them.

# _bare_id_is_child succeeds when AGENTS.md is the Govna child pointer.
_bare_id_is_child() {
  [ -f AGENTS.md ] && LC_ALL=C grep -q '^This repository is a Govna child repository\.' AGENTS.md
}

# _bare_id_list_files prints each tracked or unignored *.md path, NUL-delimited.
# Outside a Git work tree it lists every *.md file under the current directory.
_bare_id_list_files() {
  if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    git ls-files -z --cached --others --exclude-standard -- '*.md'
  else
    find . \( -name .git -o -name node_modules -o -name vendor \) -prune -o \
      -name '*.md' -type f -print0
  fi
}

# _bare_id_scan prints one "<path>:<line>: bare identifier <ID>; ..." line per
# finding in the current repository and fails only when a file cannot be read.
_bare_id_scan() {
  local child=0 rel ac_doc='^govna/ac[0-9]+-[^/]+\.md$' files=()
  if _bare_id_is_child; then child=1; fi
  while IFS= read -r -d '' rel; do
    rel="${rel#./}"
    [ -f "$rel" ] || continue
    case "$rel" in tests/fixtures/* | testdata/* | */testdata/*) continue ;; esac
    case "$rel" in
      {{BARE_ID_SCAN_FILE_PATTERNS}} | govna/*) ;;
      *) continue ;;
    esac
    if [ "$child" -eq 0 ]; then
      [ "$rel" != CHANGELOG.md ] || continue
      if [[ "$rel" =~ $ac_doc ]]; then continue; fi
    fi
    files+=("./$rel")
  done < <(_bare_id_list_files)
  [ "${#files[@]}" -gt 0 ] || return 0
  LC_ALL=C awk -v child="$child" '
    function strip_historical(s,   out, i, rest, j) {
      out = ""
      while ((i = index(s, "<!--")) > 0) {
        rest = substr(s, i + 4)
        j = index(rest, "-->")
        if (j > 0 && rest ~ /^[ \t]*Historical:/) {
          out = out substr(s, 1, i - 1) " "
          s = substr(rest, j + 3)
        } else {
          out = out substr(s, 1, i + 3)
          s = rest
        }
      }
      return out s
    }
    {
      path = FILENAME
      sub(/^\.\//, "", path)
      line = $0
      template = 0
      plan = 0
      if (!child) {
        line = strip_historical(line)
        template = (path == "govna/ac-template.md")
        plan = (path == "plan.md")
      }
      rest = line
      last = ""
      while (match(rest, /(AC|AT|IE)[0-9]+/)) {
        before = (RSTART > 1) ? substr(rest, RSTART - 1, 1) : last
        after = substr(rest, RSTART + RLENGTH, 1)
        id = substr(rest, RSTART, RLENGTH)
        last = substr(id, length(id), 1)
        rest = substr(rest, RSTART + RLENGTH)
        if (before ~ /[A-Za-z0-9_]/ || after ~ /[A-Za-z0-9_]/) continue
        if (template && id !~ /^IE/) continue
        if (plan && id ~ /^IE/) continue
        printf "%s:%d: bare identifier %s; replace it with plain words or a placeholder such as AC<N>, or move it to a place the AGENTS.md bare-identifier rules allow\n", path, FNR, id
      }
    }' "${files[@]}"
}

# _bare_id_check runs the scan as one build or prep step and fails on a finding.
_bare_id_check() {
  local findings count
  printf '%s\n' "$(yel7 '==> Check markdown for bare AC, AT, and IE identifiers')"
  findings=$(_bare_id_scan) || {
    printf 'bare identifier scan: reading Markdown failed; make every listed Markdown file readable and retry\n' >&2
    return 1
  }
  if [ -n "$findings" ]; then
    printf '%s\n' "$findings" | sed 's/^/    /'
    count=$(printf '%s\n' "$findings" | LC_ALL=C wc -l | tr -d ' ')
    printf 'bare identifier scan found %s issue(s)\n' "$count" >&2
    return 1
  fi
  printf '    %s\n' 'No bare AC, AT, or IE identifiers found.'
}
# --- end bare identifier scan ---
