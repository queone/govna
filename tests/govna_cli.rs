use sha2::{Digest, Sha256};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicU32, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

#[test]
// Compares against Cargo.toml's version (env!("CARGO_PKG_VERSION")), not a
// hardcoded duplicate, so a forgotten PROGRAM_VERSION bump fails loudly
// instead of two stale copies silently agreeing with each other. This only
// holds because govna is single-utility (one [[bin]], package version and
// PROGRAM_VERSION kept in lockstep by design) — a multi-utility repo must
// compare a printed version against that specific utility's own
// PROGRAM_VERSION declaration, not the shared package version (AGENTS.md's
// Project Rules and build.sh's own multi-utility handling treat those as
// deliberately independent).
fn version_aliases_are_all_single_line_and_identical() {
    let expected = format!("govna v{}\n", env!("CARGO_PKG_VERSION"));
    for arg in ["--version", "version", "ver", "v"] {
        let output = Command::new(env!("CARGO_BIN_EXE_govna"))
            .arg(arg)
            .output()
            .unwrap();
        assert!(output.status.success(), "arg={arg}");
        assert_eq!(output.stdout, expected.as_bytes(), "arg={arg}");
        assert!(output.stderr.is_empty(), "arg={arg}");
    }
}

#[test]
fn no_args_exits_with_usage_error() {
    let output = Command::new(env!("CARGO_BIN_EXE_govna")).output().unwrap();
    assert_eq!(output.status.code(), Some(2));
    assert!(output.stdout.is_empty());
    assert!(!output.stderr.is_empty());
    let expected = format!(
        "govna v{}\nRepo governance templates — github.com/queone/govna\n\nUsage: govna <command> [options]\n\n  apply                         apply governance template to a repo\n  audit                         drift scan an adopted repo against govna canon\n  rm                            emit cleanup AC for removing govna canon\n  render                        render flavor-specific canon files into a target directory\n  ver, v, --version             print version\n  help, h                       show this help\n\nRun 'govna <command> -h' for command-specific flags.\n",
        env!("CARGO_PKG_VERSION")
    );
    assert_eq!(output.stderr, expected.as_bytes());
}

#[test]
fn top_level_help_aliases_use_stdout() {
    for alias in ["help", "h"] {
        let output = govna().arg(alias).output().unwrap();
        assert!(output.status.success(), "{alias}");
        assert!(output.stderr.is_empty(), "{alias}");
        assert!(String::from_utf8_lossy(&output.stdout).contains("  audit"));
        assert!(!String::from_utf8_lossy(&output.stdout).contains("drift-scan"));
        assert!(!String::from_utf8_lossy(&output.stdout).contains("render-canon"));
    }
}

#[test]
fn legacy_command_aliases_remain_functional_but_hidden() {
    for alias in ["render-canon", "drift-scan"] {
        let output = govna().args([alias, "--help"]).output().unwrap();
        assert!(output.status.success(), "{alias}");
    }
}

#[test]
fn unrecognized_subcommand_exits_two() {
    let output = Command::new(env!("CARGO_BIN_EXE_govna"))
        .arg("deps")
        .output()
        .unwrap();
    assert_eq!(output.status.code(), Some(2));
    assert!(output.stdout.is_empty());
    assert!(String::from_utf8_lossy(&output.stderr).contains("unknown command: deps"));
}

// ── render fixtures ──────────────────────────────────────────────────

static FIXTURE_COUNTER: AtomicU32 = AtomicU32::new(0);

fn new_fixture() -> PathBuf {
    let n = FIXTURE_COUNTER.fetch_add(1, Ordering::SeqCst);
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    let dir = std::env::temp_dir().join(format!(
        "govna-cli-test-{}-{}-{}",
        std::process::id(),
        nanos,
        n
    ));
    fs::create_dir_all(&dir).unwrap();
    dir
}

fn govna() -> Command {
    Command::new(env!("CARGO_BIN_EXE_govna"))
}

fn read(path: &Path) -> String {
    fs::read_to_string(path).unwrap_or_else(|e| panic!("read {}: {e}", path.display()))
}

// DOC flavor renders; metadata has repo_type = DOC, no code_stack, a canon_version line.
#[test]
fn render_doc_flavor_metadata() {
    let cwd = new_fixture();
    let target = new_fixture();
    let output = govna()
        .args(["render", "--flavor", "doc", target.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );

    let metadata = read(&target.join("govna/metadata.txt"));
    assert!(metadata.contains("repo_type = DOC\n"), "{metadata}");
    assert!(!metadata.contains("code_stack"), "{metadata}");
    assert!(metadata.contains("canon_version = v"), "{metadata}");
}

fn sha256(content: &str) -> String {
    Sha256::digest(content.as_bytes())
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect()
}

fn validate_baseline(dir: &Path) -> String {
    let baseline = read(&dir.join("govna/canon-baseline.txt"));
    let mut lines = baseline.lines();
    assert_eq!(lines.next(), Some("govna-canon-baseline-v1"));
    assert!(lines.next().unwrap().starts_with("canon_version = v"));
    let mut previous = "";
    for line in lines {
        let fields: Vec<&str> = line.split('\t').collect();
        assert_eq!(fields.len(), 3, "{line}");
        assert!(previous < fields[0], "manifest is not sorted: {line}");
        previous = fields[0];
        assert_ne!(fields[0], "govna/canon-baseline.txt");
        let content = read(&dir.join(fields[0]));
        let region = match fields[1].strip_prefix("before:") {
            Some(boundary) => content.split(&format!("{boundary}\n")).next().unwrap(),
            None => {
                assert_eq!(fields[1], "full");
                content.as_str()
            }
        };
        assert_eq!(fields[2], sha256(region), "{}", fields[0]);
    }
    baseline
}

#[test]
fn render_baselines_are_valid_flavor_specific_and_deterministic() {
    let cwd = new_fixture();
    let code = new_fixture();
    let code_again = new_fixture();
    let doc = new_fixture();
    for target in [&code, &code_again] {
        let out = govna()
            .args([
                "render",
                "--flavor",
                "code",
                "--stack",
                "Rust",
                target.to_str().unwrap(),
            ])
            .current_dir(&cwd)
            .output()
            .unwrap();
        assert!(
            out.status.success(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
    }
    let out = govna()
        .args(["render", "--flavor", "doc", doc.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let code_baseline = validate_baseline(&code);
    assert_eq!(code_baseline, validate_baseline(&code_again));
    assert_ne!(code_baseline, validate_baseline(&doc));
}

// cwd with Cargo.toml infers Rust; case-insensitive --stack override matches.
#[test]
fn render_infers_rust_and_accepts_case_insensitive_override() {
    let cwd = new_fixture();
    fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();

    let inferred_target = new_fixture();
    let out = govna()
        .args([
            "render",
            "--flavor",
            "code",
            inferred_target.to_str().unwrap(),
        ])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(read(&inferred_target.join("govna/metadata.txt")).contains("code_stack = Rust"));

    let explicit_target = new_fixture();
    let out = govna()
        .args([
            "render",
            "--flavor",
            "code",
            "--stack",
            "rUsT",
            explicit_target.to_str().unwrap(),
        ])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(read(&explicit_target.join("govna/metadata.txt")).contains("code_stack = Rust"));
}

// cwd with Package.swift infers Swift; case-insensitive --stack override matches.
#[test]
fn render_infers_swift_and_accepts_case_insensitive_override() {
    let cwd = new_fixture();
    fs::write(cwd.join("Package.swift"), "// swift-tools-version:6.0\n").unwrap();

    let inferred_target = new_fixture();
    let out = govna()
        .args([
            "render",
            "--flavor",
            "code",
            inferred_target.to_str().unwrap(),
        ])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(read(&inferred_target.join("govna/metadata.txt")).contains("code_stack = Swift"));

    let explicit_target = new_fixture();
    let out = govna()
        .args([
            "render",
            "--flavor",
            "code",
            "--stack",
            "sWiFt",
            explicit_target.to_str().unwrap(),
        ])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(read(&explicit_target.join("govna/metadata.txt")).contains("code_stack = Swift"));
}

// DOC flavor rejects --stack.
#[test]
fn render_doc_rejects_stack() {
    let cwd = new_fixture();
    let target = new_fixture();
    let out = govna()
        .args([
            "render",
            "--flavor",
            "doc",
            "--stack",
            "Rust",
            target.to_str().unwrap(),
        ])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("applies only to CODE canon"));
}

// module-path is Go-only — rejected for DOC and for non-Go CODE stacks.
#[test]
fn render_module_path_rejected_outside_go_code() {
    let cwd = new_fixture();

    let doc_target = new_fixture();
    let out = govna()
        .args([
            "render",
            "--flavor",
            "doc",
            "--module-path",
            "example.com/x",
            doc_target.to_str().unwrap(),
        ])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("applies only to Go CODE canon"));

    let rust_target = new_fixture();
    let out = govna()
        .args([
            "render",
            "--flavor",
            "code",
            "--stack",
            "Rust",
            "--module-path",
            "example.com/x",
            rust_target.to_str().unwrap(),
        ])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("applies only to Go CODE canon"));
}

// Go module path read from go.mod; explicit --module-path overrides it.
#[test]
fn render_go_module_path_and_override() {
    let cwd = new_fixture();
    fs::write(cwd.join("go.mod"), "module example.com/thing\n\ngo 1.22\n").unwrap();

    let inferred_target = new_fixture();
    let out = govna()
        .args([
            "render",
            "--flavor",
            "code",
            "--stack",
            "Go",
            inferred_target.to_str().unwrap(),
        ])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let arch = read(&inferred_target.join("arch.md"));
    assert!(!arch.contains("{{"), "{arch}");
    let readme = read(&inferred_target.join("README.md"));
    assert!(readme.starts_with("# thing"), "{readme}");

    let override_target = new_fixture();
    let out = govna()
        .args([
            "render",
            "--flavor",
            "code",
            "--stack",
            "Go",
            "--module-path",
            "example.com/override",
            override_target.to_str().unwrap(),
        ])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let readme = read(&override_target.join("README.md"));
    assert!(readme.starts_with("# override"), "{readme}");
}

// .gitignore carries the stack ignore block; development-guidelines.md carries the
// stack guideline block above ## Project Practices.
#[test]
fn render_stitches_gitignore_and_guidelines() {
    let cwd = new_fixture();
    fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
    let target = new_fixture();
    let out = govna()
        .args(["render", "--flavor", "code", target.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );

    let gitignore = read(&target.join(".gitignore"));
    assert!(gitignore.contains("# Rust"), "{gitignore}");
    assert!(gitignore.contains("/target/"), "{gitignore}");

    let guidelines = read(&target.join("govna/development-guidelines.md"));
    let rust_pos = guidelines
        .find("## Rust Practices")
        .expect("Rust Practices block missing");
    // Match the heading itself, not the intro sentence's prose mention of it.
    let boundary_pos = guidelines
        .find("\n## Project Practices\n")
        .expect("boundary heading missing");
    assert!(rust_pos < boundary_pos, "{guidelines}");
}

// help output documents --stack and --module-path.
#[test]
fn render_help_documents_flags() {
    let out = govna().args(["render", "--help"]).output().unwrap();
    assert!(out.status.success());
    let stderr = String::from_utf8_lossy(&out.stderr);
    assert!(stderr.contains("-s, --stack <name>"), "{stderr}");
    assert!(stderr.contains("-m, --module-path <path>"), "{stderr}");
}

// AGENTS.md and govna/*.md are fully substituted — no leftover {{...}} tokens.
// Deliberately scoped to these two paths, not all rendered output: the Go stack's
// build.sh legitimately contains `{{.Path}}` (Go's own `go list -f` syntax).
#[test]
fn render_output_is_fully_substituted() {
    let cwd = new_fixture();
    fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
    let target = new_fixture();
    let out = govna()
        .args(["render", "--flavor", "code", target.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );

    let agents = read(&target.join("AGENTS.md"));
    assert!(!agents.contains("{{"), "{agents}");

    for entry in fs::read_dir(target.join("govna")).unwrap() {
        let path = entry.unwrap().path();
        if path.extension().map(|e| e == "md").unwrap_or(false)
            || path.file_name().unwrap() == "metadata.txt"
        {
            let content = read(&path);
            assert!(!content.contains("{{"), "{}: {}", path.display(), content);
        }
    }
}

// DOC's rendered AGENTS.md differs from CODE's and references DOC-specific docs —
// proves the DOC overlay's AGENTS.md.tmpl overrides base/AGENTS.md, per the
// last-write-wins output-precedence rule, rather than base silently winning for both flavors.
#[test]
fn render_doc_agents_overrides_base() {
    let doc_cwd = new_fixture();
    let doc_target = new_fixture();
    let out = govna()
        .args(["render", "--flavor", "doc", doc_target.to_str().unwrap()])
        .current_dir(&doc_cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );

    let code_cwd = new_fixture();
    fs::write(code_cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
    let code_target = new_fixture();
    let out = govna()
        .args(["render", "--flavor", "code", code_target.to_str().unwrap()])
        .current_dir(&code_cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );

    let doc_agents = read(&doc_target.join("AGENTS.md"));
    let code_agents = read(&code_target.join("AGENTS.md"));
    assert_ne!(doc_agents, code_agents);
    assert!(
        doc_agents.contains("govna/editing-guidelines.md"),
        "{doc_agents}"
    );
    assert!(
        code_agents.contains("govna/development-guidelines.md"),
        "{code_agents}"
    );
}

// CLAUDE.md is a symlink to AGENTS.md, for both flavors (govna's deliberate
// divergence from governa parity — governa's own render never creates this).
#[test]
fn render_creates_claude_symlink() {
    let cwd = new_fixture();
    let target = new_fixture();
    let out = govna()
        .args(["render", "--flavor", "doc", target.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let claude = target.join("CLAUDE.md");
    let link_target = fs::read_link(&claude).unwrap();
    assert_eq!(link_target, Path::new("AGENTS.md"));
}

// govna's own root docs no longer carry stale governa Go-implementation tokens;
// .gitignore and development-guidelines.md carry the Rust stitching; README shows
// render as implemented.
#[test]
fn root_docs_have_no_stale_governa_tokens() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let docs = [
        "ac-template",
        "build-release",
        "canon-cycle",
        "code-stacks",
        "development-cycle",
        "development-guidelines",
        "audit",
        "operator-contract-rationale",
        "README",
        "roles",
    ];
    for name in docs {
        let path = repo_root.join("govna").join(format!("{name}.md"));
        let content = read(&path);
        for stale in ["cmd/governa", "internal/templates", "embedded FS"] {
            assert!(
                !content.contains(stale),
                "{}: found stale token {:?}",
                path.display(),
                stale
            );
        }
    }

    let gitignore = read(&repo_root.join(".gitignore"));
    assert!(gitignore.contains("# Rust"), "{gitignore}");

    let guidelines = read(&repo_root.join("govna/development-guidelines.md"));
    let rust_pos = guidelines
        .find("## Rust Practices")
        .expect("Rust Practices block missing");
    // Match the heading itself, not the intro sentence's prose mention of it.
    let boundary_pos = guidelines
        .find("\n## Project Practices\n")
        .expect("boundary heading missing");
    assert!(rust_pos < boundary_pos, "{guidelines}");

    let readme = read(&repo_root.join("README.md"));
    assert!(readme.contains("| `render` | implemented |"), "{readme}");
}

// govna's own repo root carries no self-referential govna/metadata.txt,
// matching governa's own precedent (verified via governa's git history: it has
// never once committed a self-referential metadata.txt at its own root — the
// only copies that exist anywhere in its history are the two template sources).
#[test]
fn root_has_no_self_referential_metadata() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    assert!(!repo_root.join("govna/metadata.txt").exists());
}

// [Manual] — Director reads the rewritten govna/*.md root docs end-to-end and
// confirms the prose is accurate for govna's actual implementation. No automated
// coverage possible; tracked here as a marker only.

// ── audit fixtures ─────────────────────────────────────────────────────

fn git(dir: &Path, args: &[&str]) {
    let out = Command::new("git")
        .args(args)
        .current_dir(dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "git {args:?} failed: {}",
        String::from_utf8_lossy(&out.stderr)
    );
}

/// A fresh `render --flavor code --stack Rust` output, `git init`'d
/// but with nothing committed yet — so `git log` is empty for every path,
/// giving any subsequently-edited file a true zero-commit-history state
/// (`ClearSync`-eligible). A single full commit would *not* achieve this:
/// every file in that commit already has one entry in its own history.
fn rendered_code_fixture_no_commit() -> PathBuf {
    let dir = new_fixture();
    let out = govna()
        .args(["render", "--flavor", "code", "--stack", "Rust", "."])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    git(&dir, &["init", "-q"]);
    dir
}

/// Same as `rendered_code_fixture_no_commit`, plus one initial commit of
/// the full render — the baseline most ATs build on.
fn rendered_code_fixture() -> PathBuf {
    let dir = rendered_code_fixture_no_commit();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "AC0: initial fixture render"]);
    dir
}

fn rendered_doc_fixture() -> PathBuf {
    let dir = new_fixture();
    let out = govna()
        .args(["render", "--flavor", "doc", "."])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    git(&dir, &["init", "-q"]);
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "AC0: initial fixture render"]);
    dir
}

fn markdown_section<'a>(content: &'a str, heading: &str) -> &'a str {
    let after_heading = content
        .split_once(&format!("## {heading}\n"))
        .unwrap_or_else(|| panic!("{heading} section missing"))
        .1;
    after_heading
        .split_once("\n## ")
        .map_or(after_heading, |(section, _)| section)
}

fn assert_at_axes(stub: &str) {
    let lines: Vec<_> = stub
        .lines()
        .filter(|line| line.starts_with("**AT"))
        .collect();
    assert!(!lines.is_empty(), "no AT lines: {stub}");
    for line in lines {
        let source_axes = line.matches("[Automated]").count() + line.matches("[Manual]").count();
        let timing_axes = line.matches("[Pre-release gate]").count()
            + line.matches("[Post-release verification]").count();
        assert_eq!(source_axes, 1, "source axis: {line}");
        assert_eq!(timing_axes, 1, "timing axis: {line}");
        assert!(!line.contains("[Post-release verification]"), "{line}");
    }
}

fn audit_json(dir: &Path) -> serde_json::Value {
    let out = govna()
        .args(["audit", "--json"])
        .current_dir(dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let report: serde_json::Value = serde_json::from_slice(&out.stdout).unwrap_or_else(|e| {
        panic!(
            "invalid JSON: {e}\n{}",
            String::from_utf8_lossy(&out.stdout)
        )
    });
    let stub = read(&dir.join(report["emitted"]["ac_stub"].as_str().unwrap()));
    assert_at_axes(&stub);
    report
}

fn file_result<'a>(report: &'a serde_json::Value, relpath: &str) -> Option<&'a serde_json::Value> {
    report["files"]
        .as_array()
        .unwrap()
        .iter()
        .find(|f| f["relpath"] == relpath)
}

/// Reads the live-rendered canon_version from `dir`'s own govna/metadata.txt
/// (e.g. "v0.4.0") — avoids hardcoding CANON_VERSION's current value in
/// filename/content assertions, which would otherwise need manual updating
/// in every affected test on every bump.
fn canon_version(dir: &Path) -> String {
    let metadata = read(&dir.join("govna/metadata.txt"));
    metadata
        .lines()
        .find_map(|l| l.strip_prefix("canon_version = "))
        .expect("canon_version line missing")
        .to_string()
}

// audit refuses to run against govna's own source checkout —
// proves refuse_govna_source runs before require_govna_adopted, even though
// this repo would otherwise pass the positive adoption check (it has
// AGENTS.md + govna/ac-template.md). Safe against the real repo: the
// self-check is the very first thing run_inner does, before any writes.
#[test]
fn audit_refuses_govna_source() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let out = govna()
        .arg("audit")
        .current_dir(repo_root)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("looks like a govna checkout"));
}

// no AGENTS.md at all fails require_govna_adopted's exact wording.
#[test]
fn audit_requires_agents_md() {
    let dir = new_fixture();
    git(&dir, &["init", "-q"]);
    let out = govna().arg("audit").current_dir(&dir).output().unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("is not a govna-adopted repo"));
}

// passes require_govna_adopted (AGENTS.md + govna/ac-template.md) but
// has no .git/ — fails on the git-worktree requirement before classification.
#[test]
fn audit_requires_git_worktree() {
    let dir = new_fixture();
    fs::write(dir.join("AGENTS.md"), "# AGENTS.md\n").unwrap();
    fs::create_dir_all(dir.join("govna")).unwrap();
    fs::write(dir.join("govna/ac-template.md"), "template\n").unwrap();
    let out = govna().arg("audit").current_dir(&dir).output().unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("not a git worktree"));
}

// fresh, unmodified fixture — everything Match (or, byte-equal right
// after a fresh render, plan.md/arch.md also Match; they only classify
// ExpectedDivergence once actually customized), zero sync/migration/routing
// entries, "No sync items." in the emitted stub.
#[test]
fn audit_fresh_fixture_all_match() {
    let dir = rendered_code_fixture();
    let report = audit_json(&dir);
    for f in report["files"].as_array().unwrap() {
        assert_eq!(f["classification"], "match", "{f}");
    }
    let stub_path = dir.join(report["emitted"]["ac_stub"].as_str().unwrap());
    let stub = read(&stub_path);
    assert!(stub.contains("No sync items."), "{stub}");
    assert_eq!(
        markdown_section(&stub, "Migration findings").trim(),
        "`None`."
    );
    assert!(!stub.contains("**Validation disposition**"), "{stub}");
    assert!(stub.find("## Out Of Scope").unwrap() < stub.find("## Migration findings").unwrap());
    assert!(
        stub.find("## Migration findings").unwrap() < stub.find("## Acceptance Tests").unwrap()
    );
    // Nothing to sync/migrate/review — no vacuous "verify via diff" AT.
    assert!(!stub.contains("**AT2**"), "{stub}");
}

fn replace_baseline_hash(dir: &Path, relpath: &str, content: &str) {
    let path = dir.join("govna/canon-baseline.txt");
    let baseline = read(&path);
    let replacement = baseline
        .lines()
        .map(|line| {
            if line.starts_with(&format!("{relpath}\t")) {
                let mut fields = line.split('\t');
                let path = fields.next().unwrap();
                let scope = fields.next().unwrap();
                format!("{path}\t{scope}\t{}", sha256(content))
            } else {
                line.to_string()
            }
        })
        .collect::<Vec<_>>()
        .join("\n")
        + "\n";
    fs::write(path, replacement).unwrap();
}

fn add_baseline_entry(dir: &Path, relpath: &str, content: &str) {
    let path = dir.join("govna/canon-baseline.txt");
    let baseline = read(&path);
    let mut lines = baseline.lines();
    let schema = lines.next().unwrap();
    let version = lines.next().unwrap();
    let mut entries: Vec<String> = lines.map(str::to_string).collect();
    entries.push(format!("{relpath}\tfull\t{}", sha256(content)));
    entries.sort();
    fs::write(
        path,
        format!("{schema}\n{version}\n{}\n", entries.join("\n")),
    )
    .unwrap();
}

#[test]
fn audit_baseline_distinguishes_untouched_prior_canon_from_consumer_edit() {
    let clear_dir = rendered_code_fixture();
    let relpath = "govna/roles.md";
    let prior_canon = "# Prior canon roles\n";
    fs::write(clear_dir.join(relpath), prior_canon).unwrap();
    replace_baseline_hash(&clear_dir, relpath, prior_canon);
    git(&clear_dir, &["add", "-A"]);
    git(&clear_dir, &["commit", "-q", "-m", "adopt prior canon"]);
    let clear = audit_json(&clear_dir);
    assert_eq!(
        file_result(&clear, relpath).unwrap()["classification"],
        "clear-sync"
    );

    let edited_dir = rendered_code_fixture();
    let baseline_before = read(&edited_dir.join("govna/canon-baseline.txt"));
    fs::write(edited_dir.join(relpath), "# Consumer edit\n").unwrap();
    git(&edited_dir, &["add", "-A"]);
    git(&edited_dir, &["commit", "-q", "-m", "consumer edit"]);
    let edited = audit_json(&edited_dir);
    assert_eq!(
        file_result(&edited, relpath).unwrap()["classification"],
        "ambiguity"
    );
    assert_eq!(
        read(&edited_dir.join("govna/canon-baseline.txt")),
        baseline_before,
        "audit must not advance trust state"
    );
}

#[test]
fn audit_missing_baseline_and_entry_route_without_silent_trust() {
    let missing_metadata = rendered_code_fixture();
    fs::write(
        missing_metadata.join("Cargo.toml"),
        "[package]\nname = \"fixture\"\nversion = \"0.1.0\"\n",
    )
    .unwrap();
    fs::remove_file(missing_metadata.join("govna/metadata.txt")).unwrap();
    let report = audit_json(&missing_metadata);
    assert_eq!(
        file_result(&report, "govna/metadata.txt").unwrap()["classification"],
        "migration-required"
    );
    let stub = read(&missing_metadata.join(report["emitted"]["ac_stub"].as_str().unwrap()));
    let migration = markdown_section(&stub, "Migration findings");
    assert_eq!(migration.matches("`govna/metadata.txt`").count(), 1);
    assert!(migration.contains("metadata absent; migration required"));
    assert!(stub.contains("Migration items:\n\n- `govna/metadata.txt`"));

    let missing_manifest = rendered_code_fixture();
    fs::remove_file(missing_manifest.join("govna/canon-baseline.txt")).unwrap();
    let report = audit_json(&missing_manifest);
    assert_eq!(
        file_result(&report, "govna/canon-baseline.txt").unwrap()["classification"],
        "migration-required"
    );
    let stub = read(&missing_manifest.join(report["emitted"]["ac_stub"].as_str().unwrap()));
    let migration = markdown_section(&stub, "Migration findings");
    assert_eq!(migration.matches("`govna/canon-baseline.txt`").count(), 1);
    assert!(migration.contains("install after Director-reviewed migration"));
    assert!(stub.contains("Migration items:\n\n- `govna/canon-baseline.txt`"));
    assert!(stub.contains("**Validation disposition**: proposed `./build.sh`"));
    assert!(stub.contains("Director must confirm it or override it in chat"));
    assert!(stub.contains("except `govna/canon-baseline.txt`"));
    assert!(stub.contains("every other applicable automated AT"));
    assert!(stub.contains("verified as the final audit-adoption step"));
    assert!(stub.contains("leave this emitted stub unchanged"));

    let missing_entry = rendered_code_fixture();
    let path = missing_entry.join("govna/canon-baseline.txt");
    let baseline = read(&path)
        .lines()
        .filter(|line| !line.starts_with("govna/roles.md\t"))
        .collect::<Vec<_>>()
        .join("\n")
        + "\n";
    fs::write(path, baseline).unwrap();
    fs::write(missing_entry.join("govna/roles.md"), "# changed\n").unwrap();
    let report = audit_json(&missing_entry);
    assert_eq!(
        file_result(&report, "govna/roles.md").unwrap()["classification"],
        "ambiguity"
    );
}

#[test]
fn audit_doc_baseline_migration_proposes_evidenced_no_validation() {
    let dir = rendered_doc_fixture();
    fs::remove_file(dir.join("govna/canon-baseline.txt")).unwrap();

    let report = audit_json(&dir);
    let stub = read(&dir.join(report["emitted"]["ac_stub"].as_str().unwrap()));
    assert!(stub.contains("**Validation disposition**: proposed `Not applicable`"));
    assert!(stub.contains("standard DOC canon defines no automated content-validation command"));
    assert!(stub.contains("confirm it with repository evidence or override it in chat"));
    assert!(stub.contains("repository declares a validation command"));
}

#[test]
fn audit_routes_prebaseline_retired_path_without_mutation() {
    let dir = rendered_code_fixture();
    let retired = "# Retired drift-scan documentation\n";
    fs::write(dir.join("govna/drift-scan.md"), retired).unwrap();
    fs::remove_file(dir.join("govna/canon-baseline.txt")).unwrap();
    let replacement_before = read(&dir.join("govna/audit.md"));

    let report = audit_json(&dir);
    let matching: Vec<_> = report["files"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|file| file["relpath"] == "govna/drift-scan.md")
        .collect();
    assert_eq!(matching.len(), 1, "{report}");
    assert_eq!(matching[0]["classification"], "target-has-no-canon");
    assert!(
        matching[0]["canon_ref"]
            .as_str()
            .unwrap()
            .contains("replacement: govna/audit.md")
    );
    assert!(
        matching[0]["compare_command"]
            .as_str()
            .unwrap()
            .contains("replacement is present")
    );
    assert_eq!(read(&dir.join("govna/drift-scan.md")), retired);
    assert_eq!(read(&dir.join("govna/audit.md")), replacement_before);
}

#[test]
fn audit_routes_prior_baseline_retirement_and_ignores_unrelated_local_doc() {
    let dir = rendered_code_fixture();
    let retired_path = "govna/retired-example.md";
    let retired = "retired canon content\n";
    fs::write(dir.join(retired_path), retired).unwrap();
    add_baseline_entry(&dir, retired_path, retired);
    fs::write(dir.join("govna/local-notes.md"), "consumer owned\n").unwrap();

    let report = audit_json(&dir);
    assert_eq!(
        file_result(&report, retired_path).unwrap()["classification"],
        "target-has-no-canon"
    );
    assert!(file_result(&report, "govna/local-notes.md").is_none());
    assert_eq!(read(&dir.join(retired_path)), retired);
}

#[test]
fn audit_merges_retired_path_evidence_and_retains_tombstone_replacement() {
    let dir = rendered_code_fixture();
    let retired_path = "govna/drift-scan.md";
    let retired = "retired canon content\n";
    fs::write(dir.join(retired_path), retired).unwrap();
    add_baseline_entry(&dir, retired_path, retired);
    let later_path = "govna/z-retired.md";
    fs::write(dir.join(later_path), retired).unwrap();
    add_baseline_entry(&dir, later_path, retired);
    let roles = read(&dir.join("govna/roles.md"));
    fs::write(
        dir.join("govna/roles.md"),
        format!("{roles}\nSee `drift-scan.md`.\n"),
    )
    .unwrap();

    let report = audit_json(&dir);
    let matching: Vec<_> = report["files"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|file| file["relpath"] == retired_path)
        .collect();
    assert_eq!(matching.len(), 1, "{report}");
    assert!(
        matching[0]["canon_ref"]
            .as_str()
            .unwrap()
            .contains("replacement: govna/audit.md")
    );
    let routed_paths: Vec<_> = report["files"]
        .as_array()
        .unwrap()
        .iter()
        .filter(|file| file["classification"] == "target-has-no-canon")
        .map(|file| file["relpath"].as_str().unwrap())
        .collect();
    let mut sorted_paths = routed_paths.clone();
    sorted_paths.sort_unstable();
    assert_eq!(routed_paths, sorted_paths);
    let stub = read(&dir.join(report["emitted"]["ac_stub"].as_str().unwrap()));
    assert_eq!(stub.matches("**`govna/drift-scan.md`**").count(), 1);
}

#[test]
fn audit_requires_replacement_before_recommending_retired_path_deletion() {
    let dir = rendered_code_fixture();
    fs::write(dir.join("govna/drift-scan.md"), "retired\n").unwrap();
    fs::remove_file(dir.join("govna/audit.md")).unwrap();
    fs::remove_file(dir.join("govna/canon-baseline.txt")).unwrap();

    let report = audit_json(&dir);
    let retired = file_result(&report, "govna/drift-scan.md").unwrap();
    assert!(
        retired["compare_command"]
            .as_str()
            .unwrap()
            .contains("replacement is missing")
    );
    assert_eq!(read(&dir.join("govna/drift-scan.md")), "retired\n");
    assert!(!dir.join("govna/audit.md").exists());
}

#[test]
fn audit_rejects_malformed_baseline_before_emission() {
    let valid_hash = "0".repeat(64);
    let cases = [
        "govna-canon-baseline-v1\ncanon_version = v0.4.0\nbad-fields\n".to_string(),
        "govna-canon-baseline-v1\ncanon_version = v0.4.0\ngovna/roles.md\tfull\tnot-a-hash\n"
            .to_string(),
        format!(
            "govna-canon-baseline-v1\ncanon_version = v0.4.0\ngovna/roles.md\tfull\t{valid_hash}\ngovna/roles.md\tfull\t{valid_hash}\n"
        ),
        format!("govna-canon-baseline-v1\ncanon_version = v0.4.0\nAGENTS.md\tfull\t{valid_hash}\n"),
        format!(
            "govna-canon-baseline-v1\ncanon_version = v0.4.0\ngovna/roles.md\tbogus\t{valid_hash}\n"
        ),
        format!(
            "govna-canon-baseline-v1\ncanon_version = v99.0.0\ngovna/roles.md\tfull\t{valid_hash}\n"
        ),
    ];
    for content in cases {
        let dir = rendered_code_fixture();
        fs::write(dir.join("govna/canon-baseline.txt"), content).unwrap();
        let out = govna().arg("audit").current_dir(&dir).output().unwrap();
        assert!(!out.status.success());
        let stderr = String::from_utf8_lossy(&out.stderr);
        assert!(
            stderr.contains("invalid govna/canon-baseline.txt")
                || stderr.contains("newer than embedded canon"),
            "{stderr}"
        );
        assert!(fs::read_dir(dir.join("govna")).unwrap().all(|entry| {
            !entry
                .unwrap()
                .file_name()
                .to_string_lossy()
                .contains("audit-v")
        }));
    }
}

// A committed stale canon_version is bookkeeping drift when replacing that
// field makes metadata byte-equal to canon. Git history does not turn it into
// a routing decision.
#[test]
fn audit_stale_metadata_version_forces_clear_sync() {
    let dir = rendered_code_fixture();
    let metadata = read(&dir.join("govna/metadata.txt"));
    let current = format!("canon_version = {}", canon_version(&dir));
    let stale: String = metadata
        .lines()
        .map(|l| {
            if l.starts_with("canon_version = ") {
                "canon_version = v0.1.0"
            } else {
                l
            }
        })
        .collect::<Vec<_>>()
        .join("\n")
        + "\n";
    assert_ne!(metadata, stale, "canon_version line not found in fixture");
    assert!(metadata.contains(&current), "{metadata}");
    fs::write(dir.join("govna/metadata.txt"), stale).unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "stale version"]);
    let report = audit_json(&dir);
    let fr = file_result(&report, "govna/metadata.txt").unwrap();
    assert_eq!(fr["classification"], "clear-sync");
    let stub_path = dir.join(report["emitted"]["ac_stub"].as_str().unwrap());
    let stub = read(&stub_path);
    assert!(
        stub.contains("- `govna/metadata.txt` — clear-sync"),
        "{stub}"
    );
    assert!(
        stub.contains("`None` — no ambiguities or target-only files surfaced."),
        "{stub}"
    );
    assert!(!stub.contains("**`govna/metadata.txt`**"), "{stub}");
}

// A preserve marker cannot pin the canon-owned freshness field when it is
// the only metadata difference.
#[test]
fn audit_stale_metadata_version_overrides_preserve_marker() {
    let dir = rendered_code_fixture();
    let metadata = read(&dir.join("govna/metadata.txt"));
    fs::write(
        dir.join("govna/metadata.txt"),
        metadata.replacen(
            &format!("canon_version = {}", canon_version(&dir)),
            "canon_version = v0.1.0",
            1,
        ),
    )
    .unwrap();
    let changelog = read(&dir.join("CHANGELOG.md"));
    fs::write(
        dir.join("CHANGELOG.md"),
        changelog.replacen(
            "| Unreleased | |",
            "| Unreleased | preserve govna/metadata.txt |",
            1,
        ),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(
        &dir,
        &["commit", "-q", "-m", "mark stale metadata preserved"],
    );
    let report = audit_json(&dir);
    let fr = file_result(&report, "govna/metadata.txt").unwrap();
    assert_eq!(fr["classification"], "clear-sync");
    assert!(fr.get("preserve_markers").is_none(), "{fr}");
}

// A lower canon_version does not authorize a field-level merge. Any other
// metadata difference keeps the whole file in review.
#[test]
fn audit_stale_metadata_with_other_difference_routes_to_review() {
    let dir = rendered_code_fixture();
    let metadata = read(&dir.join("govna/metadata.txt"));
    let stale = metadata.replacen(
        &format!("canon_version = {}", canon_version(&dir)),
        "canon_version = v0.1.0",
        1,
    );
    fs::write(
        dir.join("govna/metadata.txt"),
        format!("{stale}local_field = keep\n"),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "edit another metadata field"]);
    let report = audit_json(&dir);
    let fr = file_result(&report, "govna/metadata.txt").unwrap();
    assert_eq!(fr["classification"], "ambiguity");
    let stub_path = dir.join(report["emitted"]["ac_stub"].as_str().unwrap());
    let stub = read(&stub_path);
    assert!(stub.contains("**`govna/metadata.txt`**"), "{stub}");
    assert!(
        !stub.contains("- `govna/metadata.txt` — clear-sync"),
        "{stub}"
    );
}

// A newer or malformed target marker fails before AC allocation/emission.
#[test]
fn audit_rejects_non_adoptable_metadata_versions_before_emission() {
    for (version, expected) in [
        ("v99.0.0", "upgrade govna"),
        ("v1.2", "strict vMAJOR.MINOR.PATCH"),
    ] {
        let dir = rendered_code_fixture();
        let metadata = read(&dir.join("govna/metadata.txt"));
        fs::write(
            dir.join("govna/metadata.txt"),
            metadata.replacen(
                &format!("canon_version = {}", canon_version(&dir)),
                &format!("canon_version = {version}"),
                1,
            ),
        )
        .unwrap();
        let out = govna().arg("audit").current_dir(&dir).output().unwrap();
        assert!(!out.status.success());
        assert!(
            String::from_utf8_lossy(&out.stderr).contains(expected),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        assert!(
            fs::read_dir(dir.join("govna"))
                .unwrap()
                .filter_map(|entry| entry.ok())
                .all(|entry| !entry.file_name().to_string_lossy().contains("audit-v"))
        );
    }
}

// a committed edit to a non-format-defining file with git history
// classifies Ambiguity, routed to Routing Decisions (not silently synced).
#[test]
fn audit_ambiguity_routes_to_review() {
    let dir = rendered_code_fixture();
    fs::write(
        dir.join("govna/roles.md"),
        format!("{}\nextra line\n", read(&dir.join("govna/roles.md"))),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "edit roles"]);
    let report = audit_json(&dir);
    let fr = file_result(&report, "govna/roles.md").unwrap();
    assert_eq!(fr["classification"], "ambiguity");
    let stub_path = dir.join(report["emitted"]["ac_stub"].as_str().unwrap());
    let stub = read(&stub_path);
    assert!(stub.contains("### Routing Decisions"));
    assert!(stub.contains("govna/roles.md"));
    // Generic ambiguity wording no longer overstates "local commits" — the
    // divergence might just be staleness, not necessarily a local edit.
    assert!(
        stub.contains("`govna/roles.md`**: diverges from canon"),
        "{stub}"
    );
    assert!(!stub.contains("local commits diverge"), "{stub}");
    // A non-empty Routing Decisions section gets a Manual AT confirming
    // every listed item was actually resolved.
    assert!(
        stub.contains("Director resolved every `### Routing Decisions` item"),
        "{stub}"
    );
    assert!(
        stub.contains("Every Director-resolved routing target is effective implementation scope"),
        "{stub}"
    );
    assert!(
        stub.contains(
            "resolve every routing decision in chat and leave this emitted stub unchanged"
        ),
        "{stub}"
    );
    assert!(
        stub.contains("explicitly named migration destination"),
        "{stub}"
    );
    assert!(stub.contains("`CHANGELOG.md` joins it"), "{stub}");
    assert!(
        stub.contains("Do not infer an unnamed migration destination"),
        "{stub}"
    );
    assert!(
        stub.contains("Every resolved routing outcome is verified conditionally"),
        "{stub}"
    );
    for outcome in [
        "sync targets match their rendered canon region",
        "migration sources are absent unless explicitly preserved",
        "canon-backed migration destinations match rendered canon",
        "repo-owned migration destinations satisfy the Director's stated result",
        "delete targets are absent",
        "preserve targets remain and `CHANGELOG.md` carries the required preserve marker",
    ] {
        assert!(stub.contains(outcome), "missing {outcome}:\n{stub}");
    }
}

// format-defining override. AGENTS.md edited above its canon-zone
// boundary, with committed history (raw classification Ambiguity — a
// zero-history ClearSync raw classification would ALSO force to sync, but
// the "Format-defining file routing" note only fires when the override
// actually changes the outcome, i.e. raw != ClearSync/MissingTarget; this
// scenario exercises that real branch, not a vacuous one).
#[test]
fn audit_format_defining_forces_sync() {
    let dir = rendered_code_fixture();
    let agents = read(&dir.join("AGENTS.md"));
    let edited = agents.replacen(
        "## Governed Sections",
        "## Governed Sections\nextra canon-zone line",
        1,
    );
    fs::write(dir.join("AGENTS.md"), edited).unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "edit AGENTS.md canon zone"]);
    let report = audit_json(&dir);
    let fr = file_result(&report, "AGENTS.md").unwrap();
    assert_eq!(fr["classification"], "ambiguity");
    let stub_path = dir.join(report["emitted"]["ac_stub"].as_str().unwrap());
    let stub = read(&stub_path);
    assert!(stub.contains("### Format-defining file routing"), "{stub}");
    assert!(
        stub.contains("`AGENTS.md` — raw classification: ambiguity; forced to sync."),
        "{stub}"
    );
    assert!(
        stub.contains("- `AGENTS.md` — ambiguity (format-defining)"),
        "{stub}"
    );
    // Something landed in ## In Scope (AGENTS.md, forced to sync) — the
    // final AT is the non-mutating render + diff -ru recipe, not the
    // old self-defeating "re-run audit" instruction.
    assert!(
        stub.contains(
            "For each file listed under `## In Scope` except `govna/canon-baseline.txt`, each routing target resolved as sync, and each canon-backed migration destination, `govna render` (per the recipe in `## Summary`) plus `diff -ru` against rendered canon shows no remaining diff — scoped to the canon zone above the boundary heading for any file whose AT above names a boundary."
        ),
        "{stub}"
    );
    assert!(!stub.contains("Re-running `govna audit`"), "{stub}");
}

// a preserve marker in CHANGELOG.md suppresses sync — classifies
// Preserve, routed to Out Of Scope with the marker citation shown.
#[test]
fn audit_preserve_marker_routes_to_out_of_scope() {
    let dir = rendered_code_fixture();
    fs::write(
        dir.join("govna/roles.md"),
        format!("{}\nextra line\n", read(&dir.join("govna/roles.md"))),
    )
    .unwrap();
    let changelog = read(&dir.join("CHANGELOG.md"));
    fs::write(
        dir.join("CHANGELOG.md"),
        format!("{changelog}\n| 0.0.1 | preserve govna/roles.md |\n"),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "preserve marker"]);
    let report = audit_json(&dir);
    let fr = file_result(&report, "govna/roles.md").unwrap();
    assert_eq!(fr["classification"], "preserve");
    assert_eq!(fr["preserve_markers"][0], "preserve govna/roles.md");
    let stub_path = dir.join(report["emitted"]["ac_stub"].as_str().unwrap());
    let stub = read(&stub_path);
    assert!(stub.contains("## Out Of Scope"));
    assert!(stub.contains("preserve govna/roles.md"));
}

// mixed-content boundary. An edit strictly below `## Project Practices`
// (the repo-owned tail) classifies Match — canon-zone byte-equal, not a
// false divergence.
#[test]
fn audit_mixed_content_below_boundary_matches() {
    let dir = rendered_code_fixture();
    let guidelines = read(&dir.join("govna/development-guidelines.md"));
    fs::write(
        dir.join("govna/development-guidelines.md"),
        format!("{guidelines}\nextra local note\n"),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "edit below boundary"]);
    let report = audit_json(&dir);
    let fr = file_result(&report, "govna/development-guidelines.md").unwrap();
    assert_eq!(fr["classification"], "match", "{fr}");
}

// re-running immediately (unedited stub) reuses the same AC number;
// editing the stub's body then re-running fails with the edit-detection
// guard's exact wording.
#[test]
fn audit_idempotent_reuse_and_edit_detection_guard() {
    let dir = rendered_code_fixture();
    let report1 = audit_json(&dir);
    let stub_rel = report1["emitted"]["ac_stub"].as_str().unwrap().to_string();
    let report2 = audit_json(&dir);
    assert_eq!(
        report1["emitted"]["ac_stub"], report2["emitted"]["ac_stub"],
        "AC number should be reused"
    );

    let stub_path = dir.join(&stub_rel);
    let tampered = format!("{}\ntampered\n", read(&stub_path));
    fs::write(&stub_path, tampered).unwrap();
    let out = govna().arg("audit").current_dir(&dir).output().unwrap();
    assert!(!out.status.success());
    assert!(
        String::from_utf8_lossy(&out.stderr).contains("has been edited since last audit emission")
    );
}

// cross-flavor orphan detection. A repo rendered DOC then re-rendered
// CODE over the same directory leaves DOC-only files orphaned; scanning
// with --flavor code classifies all of them target-has-no-canon.
#[test]
fn audit_cross_flavor_orphans() {
    let dir = new_fixture();
    let out = govna()
        .args(["render", "--flavor", "doc", "."])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let out = govna()
        .args(["render", "--flavor", "code", "--stack", "Rust", "."])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    git(&dir, &["init", "-q"]);
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "doc-then-code fixture"]);

    let report = audit_json(&dir);
    for orphan in [
        "govna/editing-cycle.md",
        "govna/editing-guidelines.md",
        "govna/release.md",
    ] {
        let fr =
            file_result(&report, orphan).unwrap_or_else(|| panic!("{orphan} missing from report"));
        assert_eq!(
            fr["classification"], "target-has-no-canon",
            "{orphan}: {fr}"
        );
    }
}

// name-reference body scan. A target-only file (no canon counterpart
// in either flavor) referenced via a backticked path from a divergent,
// git-tracked file classifies target-has-no-canon via the name-reference
// branch, not silently dropped.
#[test]
fn audit_name_referenced_target_only_file() {
    let dir = rendered_code_fixture();
    fs::write(dir.join("scripts.sh"), "custom helper script\n").unwrap();
    let roles = read(&dir.join("govna/roles.md"));
    fs::write(
        dir.join("govna/roles.md"),
        format!("{roles}\nSee `../scripts.sh` for details.\n"),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(
        &dir,
        &["commit", "-q", "-m", "add referenced target-only file"],
    );

    let report = audit_json(&dir);
    let fr = file_result(&report, "scripts.sh")
        .unwrap_or_else(|| panic!("scripts.sh missing from report: {report}"));
    assert_eq!(fr["classification"], "target-has-no-canon");
    assert!(
        fr["canon_ref"]
            .as_str()
            .unwrap()
            .contains("name-referenced")
    );
}

// --json output is valid JSON, deserializes to the expected shape,
// and canon_content never appears in it.
#[test]
fn audit_json_output_shape() {
    let dir = rendered_code_fixture();
    let report = audit_json(&dir);
    assert!(report["header"]["canon_sha"].is_string());
    assert!(report["files"].is_array());
    assert!(!report["files"].as_array().unwrap().is_empty());
    let raw = serde_json::to_string(&report).unwrap();
    assert!(!raw.contains("canon_content"), "{raw}");
}

// `audit extra-arg` fails with the exact "no positional
// arguments accepted" wording.
#[test]
fn audit_rejects_positional_args() {
    let dir = rendered_code_fixture();
    let out = govna()
        .args(["audit", "extra-arg"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("no positional arguments accepted"));
}

// [Manual] — Director runs audit against a real, organically-
// drifted consumer repo and confirms the emitted AC stub reads sensibly
// end-to-end. No automated fixture substitutes for a genuinely messy real
// repo; tracked here as a marker only.

// CLI surface beyond the ATs above (closure-audit gap, not a named AT):
// --repo-name overrides the basename-of-cwd default.
#[test]
fn audit_repo_name_override() {
    let dir = rendered_code_fixture();
    let out = govna()
        .args(["audit", "--json", "--repo-name", "totally-different-name"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let report: serde_json::Value = serde_json::from_slice(&out.stdout).unwrap();
    assert_eq!(report["header"]["repo_name"], "totally-different-name");
}

// CLI surface beyond the ATs above (closure-audit gap, not a named AT):
// --diff-lines truncates a long diff and reports the omitted-line count.
#[test]
fn audit_diff_lines_truncates() {
    let dir = rendered_code_fixture();
    let long_addition: String = (0..50).map(|i| format!("extra line {i}\n")).collect();
    fs::write(
        dir.join("govna/roles.md"),
        format!("{}\n{long_addition}", read(&dir.join("govna/roles.md"))),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "long edit"]);

    let out = govna()
        .args(["audit", "--json", "--diff-lines", "5"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let report: serde_json::Value = serde_json::from_slice(&out.stdout).unwrap();
    let fr = file_result(&report, "govna/roles.md").unwrap();
    let diff = fr["diff"].as_str().unwrap();
    assert!(diff.contains("more lines truncated"), "{diff}");
    assert!(
        diff.split('\n').count() < 55,
        "expected truncation, got {} lines",
        diff.split('\n').count()
    );
}

// Flavor resolution beyond the ATs above (closure-audit gap, not a named AT):
// govna/metadata.txt takes priority over manifest inference when present.
#[test]
fn render_metadata_txt_wins_over_manifest_inference() {
    let cwd = new_fixture();
    fs::create_dir_all(cwd.join("govna")).unwrap();
    fs::write(
        cwd.join("govna/metadata.txt"),
        "schema_version = 1\ncanon_version = v0.1.0\nrepo_type = DOC\n",
    )
    .unwrap();
    let target = new_fixture();
    let out = govna()
        .args(["render", target.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(read(&target.join("govna/metadata.txt")).contains("repo_type = DOC\n"));
}

// Fallback flavor heuristic: conflicting signals (Jekyll marker + a strong CODE
// manifest) error rather than guessing.
#[test]
fn render_fallback_flavor_conflict_errors() {
    let cwd = new_fixture();
    fs::write(cwd.join("_config.yml"), "").unwrap();
    fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
    let target = new_fixture();
    let out = govna()
        .args(["render", target.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("conflicting flavor signals"));
}

// Fallback flavor heuristic: no signals at all errors rather than defaulting.
#[test]
fn render_fallback_flavor_absent_errors() {
    let cwd = new_fixture();
    let target = new_fixture();
    let out = govna()
        .args(["render", target.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("could not infer flavor"));
}

// Stack inference: *.tf glob fallback (no canonical Terraform manifest file present).
#[test]
fn render_infers_terraform_from_tf_glob() {
    let cwd = new_fixture();
    fs::write(cwd.join("main.tf"), "resource \"null_resource\" \"x\" {}\n").unwrap();
    let target = new_fixture();
    let out = govna()
        .args(["render", "--flavor", "code", target.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(read(&target.join("govna/metadata.txt")).contains("code_stack = Terraform"));
}

// ── apply fixtures ──────────────────────────────────────────────────────────

// fresh empty target, apply -f code -s rust: writes AGENTS.md, CLAUDE.md
// symlink, metadata.txt (repo_type = CODE / code_stack = Rust), and
// govna/ac1-govna-apply.md; exits 0.
#[test]
fn apply_fresh_code_target() {
    let dir = new_fixture();
    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(dir.join("AGENTS.md").is_file());
    let link_target = fs::read_link(dir.join("CLAUDE.md")).unwrap();
    assert_eq!(link_target, Path::new("AGENTS.md"));
    let metadata = read(&dir.join("govna/metadata.txt"));
    assert!(metadata.contains("repo_type = CODE\n"), "{metadata}");
    assert!(metadata.contains("code_stack = Rust\n"), "{metadata}");
    assert!(dir.join("govna/ac1-govna-apply.md").is_file());
}

// fresh empty target, apply -f doc: writes the DOC-overlay set (no
// --stack required) and govna/ac1-govna-apply.md; exits 0.
#[test]
fn apply_fresh_doc_target() {
    let dir = new_fixture();
    let out = govna()
        .args(["apply", "-f", "doc"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(dir.join("AGENTS.md").is_file());
    assert!(dir.join("govna/editing-guidelines.md").is_file());
    assert!(dir.join("govna/ac1-govna-apply.md").is_file());
}

// no flags, target has CODE manifests present — infers flavor and stack
// without erroring.
#[test]
fn apply_infers_flavor_and_stack_from_manifest() {
    let dir = new_fixture();
    fs::write(dir.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
    let out = govna().arg("apply").current_dir(&dir).output().unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(read(&dir.join("govna/metadata.txt")).contains("code_stack = Rust"));
}

// no resolvable flavor signal and no -f flag exits non-zero with an
// actionable error.
#[test]
fn apply_unresolvable_flavor_errors() {
    let dir = new_fixture();
    let out = govna().arg("apply").current_dir(&dir).output().unwrap();
    assert!(!out.status.success());
    let stderr = String::from_utf8_lossy(&out.stderr);
    assert!(stderr.contains("--flavor"), "{stderr}");
}

// re-running apply against a target that already has AGENTS.md prints
// the existing-governance-files warning and allocates the adoption AC's
// number above the pre-existing govna/ac1-govna-apply.md (not ac1 again).
#[test]
fn apply_reapply_bumps_ac_number() {
    let dir = new_fixture();
    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(dir.join("govna/ac1-govna-apply.md").is_file());

    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(
        String::from_utf8_lossy(&out.stderr).contains("existing governance files detected"),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(dir.join("govna/ac2-govna-apply.md").is_file());
}

// a fresh target with no .git/ at all (no --init-git passed) still
// succeeds and allocates ac1 — regression coverage for next_ac_number's
// git-error tolerance (verified during Audit: a target with no .git/ at
// all produces a different git stderr than "no commits yet", and the
// original allocate_ac_number tolerance list didn't cover it).
#[test]
fn apply_fresh_target_without_git_succeeds() {
    let dir = new_fixture();
    assert!(!dir.join(".git").exists());
    let out = govna()
        .args(["apply", "-f", "doc"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(dir.join("govna/ac1-govna-apply.md").is_file());
}

// CLAUDE.md already present as a regular (non-symlink) file with
// content — apply warns and leaves it untouched rather than deleting it.
#[test]
fn apply_preserves_existing_regular_claude_file() {
    let dir = new_fixture();
    fs::write(dir.join("CLAUDE.md"), "hand-written content\n").unwrap();
    let out = govna()
        .args(["apply", "-f", "doc"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(
        String::from_utf8_lossy(&out.stderr).contains("expected symlink to AGENTS.md"),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert_eq!(read(&dir.join("CLAUDE.md")), "hand-written content\n");
}

// --init-git on a target with no .git/ runs git init; a second run
// against a target that already has .git/ prints a skip line and does not
// re-init.
#[test]
fn apply_init_git_then_skips_on_rerun() {
    let dir = new_fixture();
    let out = govna()
        .args(["apply", "-f", "doc", "--init-git"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(dir.join(".git").exists());

    let out = govna()
        .args(["apply", "-f", "doc", "--init-git"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(
        String::from_utf8_lossy(&out.stdout).contains("skip git init"),
        "{}",
        String::from_utf8_lossy(&out.stdout)
    );
}

// apply against govna's own source checkout refuses with a non-zero
// exit, mirroring audit's existing refusal test. Safe against the
// real repo: refuse_govna_source is the very first thing run_inner does.
#[test]
fn apply_refuses_govna_source() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let out = govna()
        .arg("apply")
        .current_dir(repo_root)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("looks like a govna checkout"));
}

// ── govna-source-only content exclusion ─────────────────────────────────────

// a freshly apply'd CODE target's rendered AGENTS.md and
// development-guidelines.md don't carry govna-source-only content that
// references paths no consumer repo has.
#[test]
fn apply_excludes_govna_source_only_content() {
    let dir = new_fixture();
    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );

    let agents = read(&dir.join("AGENTS.md"));
    assert!(
        !agents.contains("Mirror every AGENTS.md change"),
        "{agents}"
    );
    assert!(
        !agents.contains("templates/overlays/doc/files/govna/"),
        "{agents}"
    );

    let guidelines = read(&dir.join("govna/development-guidelines.md"));
    assert!(
        !guidelines.contains("templates/overlays/code/stacks/"),
        "{guidelines}"
    );
}

// the written govna/ac<N>-govna-apply.md contains all five required
// sections and lists every written file path under ## In Scope.
#[test]
fn apply_adoption_ac_has_required_sections() {
    let dir = new_fixture();
    let out = govna()
        .args(["apply", "-f", "doc"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let ac = read(&dir.join("govna/ac1-govna-apply.md"));
    assert_at_axes(&ac);
    for heading in [
        "## Summary",
        "## In Scope",
        "## Out Of Scope",
        "## Acceptance Tests",
        "## Status",
    ] {
        assert!(ac.contains(heading), "{heading} missing:\n{ac}");
    }
    assert!(ac.contains("- `AGENTS.md` (written)"), "{ac}");
    assert!(ac.contains("- `CLAUDE.md` (agent alias link)"), "{ac}");
}

// ── rm fixtures ──────────────────────────────────────────────────────────────
//
// Reuses `rendered_code_fixture()` (render-based, no adoption AC) as
// the baseline rather than an `apply`-based fixture: `apply` would write its
// own `govna/ac1-govna-apply.md`, which itself matches the `^ac(\d+)-`
// AC-numbering scan and would bump rm's first allocation to ac2 instead of
// ac1 — a real cross-command interaction, not just a test-fixture quirk.

fn rm_stub(dir: &Path) -> String {
    let out = govna().arg("rm").current_dir(dir).output().unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let stdout = String::from_utf8_lossy(&out.stdout);
    let stub_rel = stdout
        .split_whitespace()
        .find(|w| w.contains("govna-rm-") && !w.contains("-diffs"))
        .unwrap_or_else(|| panic!("no stub filename in stdout: {stdout}"));
    read(&dir.join(stub_rel))
}

// fresh unmodified fixture — pure-canon files list as `delete file`,
// CLAUDE.md lists as `delete symlink`. No companion diffs file (rm no
// longer emits one).
#[test]
fn rm_fresh_fixture_pure_canon_deletes() {
    let dir = rendered_code_fixture();
    let version = canon_version(&dir);
    let out = govna().arg("rm").current_dir(&dir).output().unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let stdout = String::from_utf8_lossy(&out.stdout);
    assert!(stdout.contains("ac1-govna-rm-"), "{stdout}");
    assert!(!stdout.contains("-diffs.md"), "{stdout}");
    assert!(
        dir.join(format!("govna/ac1-govna-rm-{version}.md"))
            .is_file()
    );
    assert!(
        !dir.join(format!("govna/ac1-govna-rm-{version}-diffs.md"))
            .exists()
    );

    let stub = read(&dir.join(format!("govna/ac1-govna-rm-{version}.md")));
    assert_at_axes(&stub);
    assert!(
        stub.contains("- `govna/roles.md` — delete file; byte-equal govna canon."),
        "{stub}"
    );
    assert!(
        stub.contains("- `CLAUDE.md` — delete symlink; govna compatibility link."),
        "{stub}"
    );
}

// hybrid files always route to Routing Decisions, even unmodified.
#[test]
fn rm_hybrid_files_always_route_to_review() {
    let dir = rendered_code_fixture();
    let stub = rm_stub(&dir);
    for path in [
        "AGENTS.md",
        "README.md",
        "CHANGELOG.md",
        "govna/development-guidelines.md",
    ] {
        assert!(
            stub.contains(&format!(
                "`{path}` is mixed canon-shape and consumer content"
            )),
            "{path} missing from Routing Decisions:\n{stub}"
        );
    }
}

// plan.md/arch.md list under Out Of Scope as repo-owned govna-adjacent content.
#[test]
fn rm_expected_divergence_files_kept() {
    let dir = rendered_code_fixture();
    let stub = rm_stub(&dir);
    for path in ["plan.md", "arch.md"] {
        assert!(
            stub.contains(&format!(
                "- `{path}` — keep; repo-owned govna-adjacent content."
            )),
            "{path}:\n{stub}"
        );
    }
}

// a preserve marker routes that file to Out Of Scope, not In Scope or Review.
#[test]
fn rm_preserve_marker_routes_to_keep() {
    let dir = rendered_code_fixture();
    let changelog = read(&dir.join("CHANGELOG.md"));
    fs::write(
        dir.join("CHANGELOG.md"),
        format!("{changelog}\n| 0.0.1 | preserve govna/roles.md |\n"),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "preserve marker"]);
    let stub = rm_stub(&dir);
    assert!(
        stub.contains("- `govna/roles.md` — keep; preserve marker: preserve govna/roles.md."),
        "{stub}"
    );
}

// a target-only file lists under Out Of Scope as target-only repo-owned file.
#[test]
fn rm_target_only_file_kept() {
    let dir = rendered_code_fixture();
    fs::write(dir.join("custom-notes.md"), "local notes\n").unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "add local file"]);
    let stub = rm_stub(&dir);
    assert!(
        stub.contains("- `custom-notes.md` — keep; target-only repo-owned file."),
        "{stub}"
    );
}

// an edited non-hybrid canon file routes to Review as ambiguity, with
// an on-demand comparison recipe embedded in the bullet (no pre-computed
// diff, no companion diffs file).
#[test]
fn rm_edited_canon_file_routes_to_ambiguity() {
    let dir = rendered_code_fixture();
    let version = canon_version(&dir);
    fs::write(
        dir.join("govna/roles.md"),
        format!("{}\nextra line\n", read(&dir.join("govna/roles.md"))),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "edit roles"]);

    let out = govna().arg("rm").current_dir(&dir).output().unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let stub = read(&dir.join(format!("govna/ac1-govna-rm-{version}.md")));
    assert!(
        stub.contains("`govna/roles.md` is consumer-edited canon file"),
        "{stub}"
    );
    assert!(
        stub.contains(
            "Compare with: `govna render --flavor code --stack Rust <scratch> && diff -ru <scratch>/govna/roles.md govna/roles.md`"
        ),
        "{stub}"
    );
    assert!(
        !dir.join(format!("govna/ac1-govna-rm-{version}-diffs.md"))
            .exists()
    );
}

// re-running unedited reuses the same AC number for both files;
// editing either fails with the edit-detection guard's exact wording.
#[test]
fn rm_idempotent_reuse_and_edit_detection_guard() {
    let dir = rendered_code_fixture();
    let version = canon_version(&dir);
    let out1 = govna().arg("rm").current_dir(&dir).output().unwrap();
    assert!(out1.status.success());
    let stdout1 = String::from_utf8_lossy(&out1.stdout).to_string();

    let out2 = govna().arg("rm").current_dir(&dir).output().unwrap();
    assert!(out2.status.success());
    let stdout2 = String::from_utf8_lossy(&out2.stdout).to_string();
    assert_eq!(stdout1, stdout2, "AC number should be reused");

    let stub_path = dir.join(format!("govna/ac1-govna-rm-{version}.md"));
    let tampered = format!("{}\ntampered\n", read(&stub_path));
    fs::write(&stub_path, tampered).unwrap();
    let out3 = govna().arg("rm").current_dir(&dir).output().unwrap();
    assert!(!out3.status.success());
    assert!(String::from_utf8_lossy(&out3.stderr).contains("has been edited since last emission"));
}

// rm against govna's own source checkout refuses with a non-zero exit.
#[test]
fn rm_refuses_govna_source() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let out = govna().arg("rm").current_dir(repo_root).output().unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("looks like a govna checkout"));
}

// no AGENTS.md fails require_govna_adopted's exact wording; no .git/
// fails the git-worktree requirement.
#[test]
fn rm_requires_adoption_and_git_worktree() {
    let dir = new_fixture();
    git(&dir, &["init", "-q"]);
    let out = govna().arg("rm").current_dir(&dir).output().unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("is not a govna-adopted repo"));

    let dir2 = new_fixture();
    let out = govna()
        .args(["apply", "-f", "doc"])
        .current_dir(&dir2)
        .output()
        .unwrap();
    assert!(out.status.success());
    let out = govna().arg("rm").current_dir(&dir2).output().unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("not a git worktree"));
}

// --flavor override is honored — forcing --flavor doc on an
// otherwise CODE-shaped fixture drives classification off DOC's smaller
// canon set (govna/editing-guidelines.md, not development-guidelines.md).
#[test]
fn rm_flavor_override_changes_canon_set() {
    let dir = rendered_code_fixture();
    let version = canon_version(&dir);
    let out = govna()
        .args(["rm", "--flavor", "doc"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let stub = read(&dir.join(format!("govna/ac1-govna-rm-{version}.md")));
    // Under DOC's canon (forced via --flavor), development-guidelines.md
    // isn't a canon path at all — it becomes target-only, not a hybrid
    // Routing Decision the way it is under CODE's canon.
    assert!(
        stub.contains("- `govna/development-guidelines.md` — keep; target-only repo-owned file."),
        "{stub}"
    );
    assert!(
        !stub.contains("`govna/development-guidelines.md` is mixed canon-shape"),
        "{stub}"
    );
}

// `rm extra-arg` fails with the exact "no positional arguments
// accepted" wording.
#[test]
fn rm_rejects_positional_args() {
    let dir = rendered_code_fixture();
    let out = govna()
        .args(["rm", "extra-arg"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("no positional arguments accepted"));
}

// ── migrate-from-governa + hunk-merge ───────────────────────────────────────

fn governa_metadata_fixture(code_stack: Option<&str>) -> PathBuf {
    let dir = new_fixture();
    fs::create_dir_all(dir.join("governa")).unwrap();
    let mut content = "schema_version = 1\ngovna_version = v0.1.0\nrepo_type = CODE\n".to_string();
    if let Some(stack) = code_stack {
        content.push_str(&format!("code_stack = {stack}\n"));
    }
    fs::write(dir.join("governa/metadata.txt"), content).unwrap();
    dir
}

/// Writes an executable `governa` stub script into `dir` (prepend `dir` to
/// `PATH` to make it discoverable). `render_ok`: when true, `render`
/// writes `governa/roles.md` with `roles_content` into the target and exits 0;
/// when false, `--version` still succeeds but `render` exits 1
/// (simulating an unsupported stack or bad-flag failure).
fn fake_governa_binary(dir: &Path, render_ok: bool, roles_content: &str) {
    use std::os::unix::fs::PermissionsExt;
    let script_path = dir.join("governa");
    // Matches governa's real CLI: `version` is a subcommand, not a `--version`
    // flag — `governa --version` actually exits non-zero on the real binary.
    let script = if render_ok {
        format!(
            "#!/bin/bash\nif [ \"$1\" = \"version\" ]; then\n  echo 'governa v0.160.2'\n  exit 0\nfi\nif [ \"$1\" = \"render-canon\" ]; then\n  target=\"${{@: -1}}\"\n  mkdir -p \"$target/governa\"\n  printf '%s' '{roles_content}' > \"$target/governa/roles.md\"\n  exit 0\nfi\nexit 1\n"
        )
    } else {
        "#!/bin/bash\nif [ \"$1\" = \"version\" ]; then\n  echo 'governa v0.160.2'\n  exit 0\nfi\nexit 1\n"
            .to_string()
    };
    fs::write(&script_path, script).unwrap();
    let mut perms = fs::metadata(&script_path).unwrap().permissions();
    perms.set_mode(0o755);
    fs::set_permissions(&script_path, perms).unwrap();
}

fn path_with(prefix: &Path) -> String {
    format!(
        "{}:{}",
        prefix.display(),
        std::env::var("PATH").unwrap_or_default()
    )
}

/// The real PATH with any directory containing a `governa` binary filtered
/// out — used for crude-tier tests. A plain empty-directory PATH override
/// would also hide `git` (and every other tool `govna` shells out to), so
/// this only removes what's specifically being tested as absent.
fn path_without_governa() -> String {
    let real_path = std::env::var_os("PATH").unwrap_or_default();
    let filtered: Vec<_> = std::env::split_paths(&real_path)
        .filter(|p| !p.join("governa").exists())
        .collect();
    std::env::join_paths(filtered)
        .unwrap()
        .to_string_lossy()
        .into_owned()
}

// governa/metadata.txt carries repo_type/code_stack; apply (no flags,
// no manifest file) resolves CODE/Rust from it instead of failing to infer.
#[test]
fn apply_migration_carries_over_legacy_metadata() {
    let dir = governa_metadata_fixture(Some("Rust"));
    let out = govna().arg("apply").current_dir(&dir).output().unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let metadata = read(&dir.join("govna/metadata.txt"));
    assert!(metadata.contains("repo_type = CODE\n"), "{metadata}");
    assert!(metadata.contains("code_stack = Rust\n"), "{metadata}");
}

// a governa-managed apply emits exactly one AC file (not two,
// adoption + migration merged), with a ## Migration findings section.
#[test]
fn apply_migration_emits_single_merged_ac() {
    let dir = governa_metadata_fixture(Some("Rust"));
    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .env("PATH", path_without_governa())
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );

    let ac_files: Vec<_> = fs::read_dir(dir.join("govna"))
        .unwrap()
        .filter_map(|e| e.ok())
        .filter(|e| {
            let name = e.file_name().to_string_lossy().into_owned();
            name.starts_with("ac") && name.chars().nth(2).is_some_and(|c| c.is_ascii_digit())
        })
        .collect();
    assert_eq!(
        ac_files.len(),
        1,
        "expected exactly one AC file: {ac_files:?}"
    );

    let version = canon_version(&dir);
    let ac = read(&dir.join(format!("govna/ac1-govna-apply-{version}.md")));
    assert_at_axes(&ac);
    assert!(ac.contains("## Migration findings"), "{ac}");
    assert!(ac.contains("## In Scope"), "{ac}");
    assert!(ac.contains("### In Scope (legacy governa/ tree)"), "{ac}");
}

// precise tier. A fake `governa` binary on PATH renders roles.md;
// byte-identical target file classifies confirmed-safe, a differing one
// classifies needs-review with the exact on-demand recipe, not a diff.
#[test]
fn apply_migration_precise_tier_classifies_via_fake_governa() {
    let stub_dir = new_fixture();
    fake_governa_binary(&stub_dir, true, "stock roles content\n");

    let dir = governa_metadata_fixture(Some("Rust"));
    fs::write(dir.join("governa/roles.md"), "stock roles content\n").unwrap();
    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .env("PATH", path_with(&stub_dir))
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let version = canon_version(&dir);
    let ac = read(&dir.join(format!("govna/ac1-govna-apply-{version}.md")));
    assert!(
        ac.contains("- `governa/roles.md` — confirmed safe; confirmed byte-identical"),
        "{ac}"
    );

    let dir2 = governa_metadata_fixture(Some("Rust"));
    fs::write(dir2.join("governa/roles.md"), "edited roles content\n").unwrap();
    let out2 = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir2)
        .env("PATH", path_with(&stub_dir))
        .output()
        .unwrap();
    assert!(
        out2.status.success(),
        "{}",
        String::from_utf8_lossy(&out2.stderr)
    );
    let version2 = canon_version(&dir2);
    let ac2 = read(&dir2.join(format!("govna/ac1-govna-apply-{version2}.md")));
    assert!(
        ac2.contains(
            "Compare with: `governa render-canon --flavor code --stack Rust <scratch> && diff -ru <scratch>/governa/roles.md governa/roles.md`"
        ),
        "{ac2}"
    );
}

// crude tier. No `governa` binary on PATH at all: governa/roles.md
// (has a govna/ equivalent, since apply just wrote one) flags "likely
// superseded"; a file with no govna equivalent flags "no equivalent".
#[test]
fn apply_migration_crude_tier_fallback_no_governa_binary() {
    let dir = governa_metadata_fixture(Some("Rust"));
    fs::write(dir.join("governa/roles.md"), "old roles\n").unwrap();
    fs::write(dir.join("governa/ac3-custom-thing.md"), "custom\n").unwrap();
    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .env("PATH", path_without_governa())
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let version = canon_version(&dir);
    let ac = read(&dir.join(format!("govna/ac1-govna-apply-{version}.md")));
    assert!(
        ac.contains("- `governa/roles.md` — likely superseded; likely superseded by `govna/roles.md`; compare manually before removing."),
        "{ac}"
    );
    assert!(
        ac.contains(
            "- `governa/ac3-custom-thing.md` — keep; no govna equivalent; may be repo-owned content."
        ),
        "{ac}"
    );
}

// a `governa` binary that succeeds on --version but fails on
// render (e.g. unsupported stack) falls back to the crude path —
// apply does not error or crash.
#[test]
fn apply_migration_falls_back_when_render_fails() {
    let stub_dir = new_fixture();
    fake_governa_binary(&stub_dir, false, "");

    let dir = governa_metadata_fixture(Some("Rust"));
    fs::write(dir.join("governa/roles.md"), "old roles\n").unwrap();
    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .env("PATH", path_with(&stub_dir))
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let version = canon_version(&dir);
    let ac = read(&dir.join(format!("govna/ac1-govna-apply-{version}.md")));
    assert!(ac.contains("likely superseded by `govna/roles.md`"), "{ac}");
}

// re-running apply unedited reuses the same merged apply+migration-AC
// number; editing it and re-running fails with the edit-detection guard's
// wording.
#[test]
fn apply_migration_idempotent_reuse_and_edit_detection_guard() {
    let dir = governa_metadata_fixture(Some("Rust"));
    fs::write(dir.join("governa/roles.md"), "old roles\n").unwrap();
    let out1 = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .env("PATH", path_without_governa())
        .output()
        .unwrap();
    assert!(out1.status.success());
    let version = canon_version(&dir);
    assert!(
        dir.join(format!("govna/ac1-govna-apply-{version}.md"))
            .is_file()
    );

    let out2 = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .env("PATH", path_without_governa())
        .output()
        .unwrap();
    assert!(out2.status.success());
    assert!(
        !dir.join(format!("govna/ac2-govna-apply-{version}.md"))
            .exists(),
        "should reuse ac1, not allocate ac2"
    );

    let ac_path = dir.join(format!("govna/ac1-govna-apply-{version}.md"));
    let tampered = format!("{}\ntampered\n", read(&ac_path));
    fs::write(&ac_path, tampered).unwrap();
    let out3 = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .env("PATH", path_without_governa())
        .output()
        .unwrap();
    assert!(!out3.status.success());
    assert!(String::from_utf8_lossy(&out3.stderr).contains("has been edited since last emission"));
}

/// Counts merged apply+migration ACs — the versioned `govna-apply-v...`
/// filename shape, distinct from the unversioned `govna-apply.md` a plain
/// (non-governa-managed) apply writes.
fn count_migration_acs(dir: &Path) -> usize {
    fs::read_dir(dir.join("govna"))
        .unwrap()
        .filter_map(|e| e.ok())
        .filter(|e| e.file_name().to_string_lossy().contains("govna-apply-v"))
        .count()
}

// no governa/ directory at all — no migration AC emitted, behavior
// identical to a plain apply. Self-terminating: once governa/ is removed,
// re-running apply emits no further migration AC. Counts migration-AC
// files rather than asserting specific numbers, since each re-`apply` also
// allocates a new (non-reused) adoption AC, shifting subsequent numbers.
#[test]
fn apply_migration_noop_without_governa_dir_and_self_terminates() {
    let dir = new_fixture();
    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert_eq!(count_migration_acs(&dir), 0);

    fs::create_dir_all(dir.join("governa")).unwrap();
    fs::write(
        dir.join("governa/metadata.txt"),
        "schema_version = 1\ngovna_version = v0.1.0\nrepo_type = CODE\ncode_stack = Rust\n",
    )
    .unwrap();
    let out2 = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(out2.status.success());
    assert_eq!(count_migration_acs(&dir), 1);

    fs::remove_dir_all(dir.join("governa")).unwrap();
    let out3 = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(out3.status.success());
    assert_eq!(
        count_migration_acs(&dir),
        1,
        "no new migration AC once governa/ is gone"
    );
}

// Part B — existing-mode apply on a repo whose AGENTS.md has extra
// bullets under ## Project Rules preserves them byte-for-byte; everything
// above the boundary matches fresh canon.
#[test]
fn apply_hunk_merges_agents_md_preserving_extra_bullets() {
    let dir = new_fixture();
    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(out.status.success());

    let original = read(&dir.join("AGENTS.md"));
    let customized = format!("{original}- A repo-specific extra rule.\n");
    fs::write(dir.join("AGENTS.md"), &customized).unwrap();

    let out2 = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(out2.status.success());
    let merged = read(&dir.join("AGENTS.md"));
    assert!(
        merged.contains("- A repo-specific extra rule.\n"),
        "{merged}"
    );
    let boundary_pos = merged.find("## Project Rules").unwrap();
    assert!(
        merged[..boundary_pos].contains("## Governed Sections"),
        "{merged}"
    );
}

// Part B — existing-mode apply on an unmodified repo is a no-op merge:
// output byte-identical to a fresh write.
#[test]
fn apply_hunk_merge_idempotent_when_unmodified() {
    let dir = new_fixture();
    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    let first = read(&dir.join("AGENTS.md"));

    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    let second = read(&dir.join("AGENTS.md"));
    assert_eq!(first, second);
}

// Part B — existing-mode apply leaves a customized README.md/CHANGELOG.md
// completely untouched (no boundary to merge on for either).
#[test]
fn apply_skips_readme_and_changelog_when_existing() {
    let dir = new_fixture();
    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    fs::write(dir.join("README.md"), "my custom readme\n").unwrap();
    fs::write(dir.join("CHANGELOG.md"), "my custom changelog\n").unwrap();

    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(out.status.success());
    assert_eq!(read(&dir.join("README.md")), "my custom readme\n");
    assert_eq!(read(&dir.join("CHANGELOG.md")), "my custom changelog\n");
}

// Part B — new-mode apply (fresh empty dir) is unaffected: writes
// AGENTS.md/README.md/CHANGELOG.md/development-guidelines.md fresh.
#[test]
fn apply_new_mode_unaffected_by_hunk_merge_logic() {
    let dir = new_fixture();
    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(out.status.success());
    assert!(dir.join("AGENTS.md").is_file());
    assert!(dir.join("README.md").is_file());
    assert!(dir.join("CHANGELOG.md").is_file());
    assert!(dir.join("govna/development-guidelines.md").is_file());
    assert!(dir.join("arch.md").is_file());
    assert!(dir.join("plan.md").is_file());
}

// Part B — an AGENTS.md missing the ## Project Rules boundary
// entirely falls back to blind overwrite with a printed warning, rather
// than silently skipping or crashing.
#[test]
fn apply_falls_back_to_overwrite_when_boundary_missing() {
    let dir = new_fixture();
    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    fs::write(dir.join("AGENTS.md"), "no boundary heading here\n").unwrap();

    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(out.status.success());
    assert!(
        String::from_utf8_lossy(&out.stderr).contains("has no `## Project Rules` boundary"),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let agents = read(&dir.join("AGENTS.md"));
    assert!(agents.contains("## Governed Sections"), "{agents}");
}

// ── apply must not overwrite EXPECTED_DIVERGENCE_PATHS files ───────────────

// existing-mode apply preserves pre-existing arch.md/plan.md
// content instead of blindly overwriting it with the fresh canon stub.
#[test]
fn apply_preserves_existing_arch_and_plan_content() {
    let dir = new_fixture();
    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    fs::write(dir.join("arch.md"), "my custom architecture notes\n").unwrap();
    fs::write(dir.join("plan.md"), "my custom roadmap\n").unwrap();

    let out = govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(out.status.success());
    assert_eq!(read(&dir.join("arch.md")), "my custom architecture notes\n");
    assert_eq!(read(&dir.join("plan.md")), "my custom roadmap\n");
    let stdout = String::from_utf8_lossy(&out.stdout);
    assert!(
        stdout.contains("skip arch.md (existing content preserved)"),
        "{stdout}"
    );
    assert!(
        stdout.contains("skip plan.md (existing content preserved)"),
        "{stdout}"
    );
}

// ── apply-AC fidelity, DOC closure-audit wording, single migration AC ──────

// new-mode apply labels every canon file "(written)" in the
// emitted AC's ## In Scope — the common case, no skips/merges to report yet.
#[test]
fn apply_new_mode_labels_every_file_written() {
    let dir = new_fixture();
    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    let ac = read(&dir.join("govna/ac1-govna-apply.md"));
    assert!(ac.contains("- `AGENTS.md` (written)"), "{ac}");
    assert!(ac.contains("- `README.md` (written)"), "{ac}");
    assert!(!ac.contains("existing content preserved"), "{ac}");
    assert!(!ac.contains("merged"), "{ac}");
}

// existing-mode apply's emitted AC correctly labels
// README.md/CHANGELOG.md/arch.md/plan.md as preserved, not written —
// regression test for the exact defect the `bits` audit found (the emitted
// AC previously claimed these were written when the run actually skipped
// them).
#[test]
fn apply_existing_mode_ac_labels_preserved_files_correctly() {
    let dir = new_fixture();
    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    fs::write(dir.join("README.md"), "my custom readme\n").unwrap();
    fs::write(dir.join("CHANGELOG.md"), "my custom changelog\n").unwrap();
    fs::write(dir.join("arch.md"), "my custom architecture notes\n").unwrap();
    fs::write(dir.join("plan.md"), "my custom roadmap\n").unwrap();

    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    let ac = read(&dir.join("govna/ac2-govna-apply.md"));
    assert_at_axes(&ac);
    for path in ["README.md", "CHANGELOG.md", "arch.md", "plan.md"] {
        assert!(
            ac.contains(&format!("- `{path}` (existing content preserved)")),
            "{path} not labeled preserved:\n{ac}"
        );
    }
    assert!(ac.contains("- `AGENTS.md` (canon zone merged"), "{ac}");
}

// The reworded manual-review wording, and the two distinct wordings
// depending on the real CLAUDE.md symlink outcome.
#[test]
fn apply_ac_at1_is_manual_review_wording() {
    let dir = new_fixture();
    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    let ac = read(&dir.join("govna/ac1-govna-apply.md"));
    assert_at_axes(&ac);
    assert!(
        ac.contains(
            "Director reads AGENTS.md and confirms it reflects this repo's actual practices"
        ),
        "{ac}"
    );
    assert!(!ac.contains("match repo needs"), "{ac}");
}

#[test]
fn apply_ac_at3_reflects_symlink_conflict() {
    let dir = new_fixture();
    fs::create_dir_all(&dir).unwrap();
    fs::write(dir.join("CLAUDE.md"), "not a symlink\n").unwrap();
    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    let ac = read(&dir.join("govna/ac1-govna-apply.md"));
    assert_at_axes(&ac);
    assert!(
        ac.contains("CLAUDE.md exists as a regular file, not a symlink to AGENTS.md"),
        "{ac}"
    );
    assert!(
        !ac.contains("Verify CLAUDE.md is a symlink to AGENTS.md."),
        "{ac}"
    );
}

// the closure-audit bullet forks by flavor — DOC no longer
// contains CODE/data-pipeline vocabulary, and DOC's render differs from
// CODE's at that exact bullet.
#[test]
fn render_doc_closure_audit_bullet_has_no_code_vocabulary() {
    let code_dir = new_fixture();
    let doc_dir = new_fixture();
    govna()
        .args([
            "render",
            "--flavor",
            "code",
            "--stack",
            "rust",
            code_dir.to_str().unwrap(),
        ])
        .output()
        .unwrap();
    govna()
        .args(["render", "--flavor", "doc", doc_dir.to_str().unwrap()])
        .output()
        .unwrap();
    let code_agents = read(&code_dir.join("AGENTS.md"));
    let doc_agents = read(&doc_dir.join("AGENTS.md"));
    for term in [
        "provider/API fetch",
        "normalized-table write",
        "durable snapshot",
        "freshness gate",
    ] {
        assert!(
            !doc_agents.contains(term),
            "DOC should not contain {term}:\n{doc_agents}"
        );
        assert!(
            code_agents.contains(term),
            "CODE should still contain {term}"
        );
    }
    assert!(doc_agents.contains("published page"), "{doc_agents}");
}

// The authoritative audit documentation is mirrored byte-for-byte into
// both consumer flavors, and the rendered identity record carries this canon
// behavior's version.
#[test]
fn render_audit_docs_and_version_match_authority() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let authority = read(&repo_root.join("govna/audit.md"));
    let canon_cycle_authority = read(&repo_root.join("govna/canon-cycle.md"));
    let agents_authority = read(&repo_root.join("AGENTS.md"));
    let code_dir = new_fixture();
    let doc_dir = new_fixture();
    let code_out = govna()
        .args([
            "render",
            "--flavor",
            "code",
            "--stack",
            "rust",
            code_dir.to_str().unwrap(),
        ])
        .output()
        .unwrap();
    assert!(code_out.status.success());
    let doc_out = govna()
        .args(["render", "--flavor", "doc", doc_dir.to_str().unwrap()])
        .output()
        .unwrap();
    assert!(doc_out.status.success());

    for dir in [&code_dir, &doc_dir] {
        assert_eq!(read(&dir.join("govna/audit.md")), authority);
        assert_eq!(
            read(&dir.join("govna/canon-cycle.md")),
            canon_cycle_authority
        );
        let agents = read(&dir.join("AGENTS.md"));
        for rule in [
            "Treat every Director-resolved routing target as effective implementation scope",
            "Treat each explicitly named migration destination as effective implementation scope",
            "Treat `CHANGELOG.md` as effective implementation scope",
            "Require the Director to name every migration destination",
            "Verify each resolved sync target",
            "Verify each migration source",
            "Verify each canon-backed migration destination",
            "Verify each repo-owned migration destination",
            "Verify each resolved delete target",
            "Verify each resolved preserve target",
            "Confirm or override the emitted validation disposition in chat",
            "Run the resolved validation command after all selected sync, migration, and deletion work",
            "Cite repository evidence when resolving validation as `Not applicable`",
            "Install or replace `govna/canon-baseline.txt` from the scratch render only after every other applicable acceptance test",
            "Treat standalone `Ratify` or `ratify` after successful Implement completion as the Director's acceptance action",
            "Perform the final non-mutating review during the same Ratify turn",
            "Complete Ratify in that turn when the review finds no issue",
            "Return Ratify feedback to Refine for contract or scope changes without completing Ratify",
            "Return Ratify feedback to Implement for implementation-only corrections without completing Ratify",
            "Skip requests for a second acceptance signal after a clean Ratify review",
        ] {
            assert!(agents_authority.contains(rule), "source AGENTS.md: {rule}");
            assert!(agents.contains(rule), "{}: {rule}", dir.display());
        }
        assert!(!agents.contains("Keep Ratify complete only after the Director accepts"));
        assert!(read(&dir.join("govna/metadata.txt")).contains("canon_version = v0.8.0\n"));
        for relpath in [
            "govna/ac-template.md",
            "govna/build-release.md",
            "govna/release.md",
        ] {
            let path = dir.join(relpath);
            if path.is_file() {
                let content = read(&path);
                assert!(!content.contains("During a audit"), "{}", path.display());
                assert!(!content.contains("resolves a audit"), "{}", path.display());
            }
        }
    }
    let development_cycle = read(&repo_root.join("govna/development-cycle.md"));
    let code_cycle = read(&code_dir.join("govna/development-cycle.md"));
    let doc_cycle = read(&doc_dir.join("govna/editing-cycle.md"));
    assert_eq!(code_cycle, development_cycle);
    for workflow in [&development_cycle, &code_cycle, &doc_cycle] {
        for rule in [
            "Treat standalone `Ratify` or `ratify` as the Director acceptance action",
            "completes Ratify when that review is clean",
            "request no second acceptance signal",
            "without completing Ratify",
            "Perform Package only when explicitly requested",
        ] {
            assert!(workflow.contains(rule), "{rule}: {workflow}");
        }
    }
    for roles in [
        read(&repo_root.join("govna/roles.md")),
        read(&code_dir.join("govna/roles.md")),
        read(&doc_dir.join("govna/roles.md")),
    ] {
        assert!(roles.contains("Treat Ratify as the director's acceptance of delivered AC work"));
        assert!(roles.contains("do not begin Package without a separate explicit request"));
    }
    let ac_template = read(&repo_root.join("govna/ac-template.md"));
    assert_at_axes(&ac_template);
    assert!(ac_template.contains("always write the selected label explicitly"));
    let authority_acceptance_tests = markdown_section(&ac_template, "Acceptance Tests");
    for dir in [&code_dir, &doc_dir] {
        let rendered_template = read(&dir.join("govna/ac-template.md"));
        assert_eq!(
            markdown_section(&rendered_template, "Acceptance Tests"),
            authority_acceptance_tests
        );
        assert_at_axes(&rendered_template);
        assert!(rendered_template.contains("always write the selected label explicitly"));
    }
    for relpath in ["govna/ac-template.md", "govna/build-release.md"] {
        let content = read(&repo_root.join(relpath));
        assert!(!content.contains("During a audit"), "{relpath}: {content}");
        assert!(
            !content.contains("resolves a audit"),
            "{relpath}: {content}"
        );
        assert!(content.contains("an audit"), "{relpath}: {content}");
    }
}

#[test]
fn render_code_build_reuse_rationale_matches_authority() {
    fn section(content: &str) -> &str {
        content
            .split_once("## Rust Compilation Reuse\n")
            .expect("Rust Compilation Reuse section missing")
            .1
            .split_once("\n## ")
            .expect("section terminator missing")
            .0
    }

    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let authority = read(&repo_root.join("govna/build-release.md"));
    let code_dir = new_fixture();
    let doc_dir = new_fixture();
    assert!(
        govna()
            .args([
                "render",
                "--flavor",
                "code",
                "--stack",
                "rust",
                code_dir.to_str().unwrap(),
            ])
            .output()
            .unwrap()
            .status
            .success()
    );
    assert!(
        govna()
            .args(["render", "--flavor", "doc", doc_dir.to_str().unwrap()])
            .output()
            .unwrap()
            .status
            .success()
    );

    let rendered = read(&code_dir.join("govna/build-release.md"));
    assert_eq!(section(&rendered), section(&authority));
    for expected in [
        "build duration becomes materially costly",
        "stable Cargo or Clippy behavior offers measurable artifact reuse",
        "compiler-cache evaluation only with Director authorization",
        "toolchain version, exact commands, isolated target-directory conditions, repeated timings, and unchanged validation coverage",
    ] {
        assert!(section(&authority).contains(expected), "{expected}");
    }
    assert!(read(&code_dir.join("govna/metadata.txt")).contains("canon_version = v0.8.0\n"));
    assert!(!read(&doc_dir.join("govna/release.md")).contains("## Rust Compilation Reuse"));
}

// Fresh CODE and DOC renders both seed ## Project Rules with just the one
// generic bullet — no govna-specific utility-versioning or IE-tracking
// content leaked into what a fresh consumer gets.
#[test]
fn render_project_rules_seed_has_no_govna_specific_content() {
    let code_dir = new_fixture();
    let doc_dir = new_fixture();
    govna()
        .args([
            "render",
            "--flavor",
            "code",
            "--stack",
            "rust",
            code_dir.to_str().unwrap(),
        ])
        .output()
        .unwrap();
    govna()
        .args(["render", "--flavor", "doc", doc_dir.to_str().unwrap()])
        .output()
        .unwrap();
    for dir in [&code_dir, &doc_dir] {
        let agents = read(&dir.join("AGENTS.md"));
        let tail = agents
            .split("## Project Rules\n\n")
            .nth(1)
            .expect("Project Rules section missing");
        assert_eq!(
            tail.trim_end(),
            "- Follow existing repo patterns unless an approved improvement says otherwise.",
            "{agents}"
        );
    }
}

// govna's own root AGENTS.md keeps its real Project Rules bullets
// (utility-versioning, IE-tracking) unchanged — only the shipped seed was
// stripped, not govna's own contract.
#[test]
fn root_agents_project_rules_unchanged() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let agents = read(&repo_root.join("AGENTS.md"));
    assert!(
        agents.contains(
            "Keep the repository/package release version separate from each installable utility version"
        ),
        "{agents}"
    );
    assert!(
        agents.contains("Track forward-looking work in `plan.md` only via IE entries"),
        "{agents}"
    );
}

// prep's canon-version validation gate — no other reasonable place to unit
// test build.sh logic from Rust, exercised directly by invoking it via bash
// instead.
#[test]
fn build_sh_validates_canon_version_bump() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let out = Command::new("bash")
        .arg("-c")
        .arg("source build.sh; _failure() { echo \"FAILURE: $*\" >&2; return 1; }; _validate_canon_version_bump")
        .current_dir(repo_root)
        .output()
        .unwrap();
    // Whatever the current real repo state is, the function must run
    // without a bash error (syntax/undefined-variable failures would show
    // up as a non-0/1 exit or stderr noise unrelated to the check itself).
    assert!(
        out.status.code() == Some(0) || out.status.code() == Some(1),
        "unexpected exit: {:?}\nstderr: {}",
        out.status.code(),
        String::from_utf8_lossy(&out.stderr)
    );
}

// The canon-version validation gate is govna-source-only (CANON_VERSION and
// templates/ don't exist in an ordinary consumer repo) and deliberately not
// shipped in the Rust stack template — locks in the one intentional
// divergence between build.sh and its template mirror.
#[test]
fn rust_stack_template_omits_canon_version_check() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let build_sh = read(&repo_root.join("build.sh"));
    let template = read(&repo_root.join("templates/overlays/code/stacks/rust/build.sh.tmpl"));
    assert!(
        build_sh.contains("_validate_canon_version_bump"),
        "{build_sh}"
    );
    assert!(
        !template.contains("_validate_canon_version_bump"),
        "{template}"
    );
}

// a mixed_content_boundary file with no matching boundary falls
// back to a blind overwrite, and the emitted AC labels it distinctly from a
// clean fresh write.
#[test]
fn apply_boundary_fallback_labeled_distinctly_in_ac() {
    let dir = new_fixture();
    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    fs::write(dir.join("AGENTS.md"), "no boundary heading here\n").unwrap();

    govna()
        .args(["apply", "-f", "code", "-s", "rust"])
        .current_dir(&dir)
        .output()
        .unwrap();
    let ac = read(&dir.join("govna/ac2-govna-apply.md"));
    assert!(
        ac.contains("- `AGENTS.md` (written — no boundary found, blind overwrite; see warning)"),
        "{ac}"
    );
}
