use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicU32, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

#[test]
fn version_aliases_are_all_single_line_and_identical() {
    for arg in ["--version", "version", "ver", "v"] {
        let output = Command::new(env!("CARGO_BIN_EXE_govna"))
            .arg(arg)
            .output()
            .unwrap();
        assert!(output.status.success(), "arg={arg}");
        assert_eq!(output.stdout, b"govna v0.6.0\n", "arg={arg}");
        assert!(output.stderr.is_empty(), "arg={arg}");
    }
}

#[test]
fn no_args_exits_with_usage_error() {
    let output = Command::new(env!("CARGO_BIN_EXE_govna")).output().unwrap();
    assert_eq!(output.status.code(), Some(2));
    assert!(output.stdout.is_empty());
    assert!(!output.stderr.is_empty());
}

#[test]
fn unimplemented_subcommand_exits_one() {
    let output = Command::new(env!("CARGO_BIN_EXE_govna"))
        .arg("deps")
        .output()
        .unwrap();
    assert_eq!(output.status.code(), Some(1));
    assert!(output.stdout.is_empty());
    assert!(String::from_utf8_lossy(&output.stderr).contains("not yet implemented"));
}

// ── render-canon fixtures ──────────────────────────────────────────────────

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

// AT1: DOC flavor renders; metadata has repo_type = DOC, no code_stack, a govna_version line.
#[test]
fn render_canon_doc_flavor_metadata() {
    let cwd = new_fixture();
    let target = new_fixture();
    let output = govna()
        .args(["render-canon", "--flavor", "doc", target.to_str().unwrap()])
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
    assert!(metadata.contains("govna_version = v"), "{metadata}");
}

// AT2: cwd with Cargo.toml infers Rust; case-insensitive --stack override matches.
#[test]
fn render_canon_infers_rust_and_accepts_case_insensitive_override() {
    let cwd = new_fixture();
    fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();

    let inferred_target = new_fixture();
    let out = govna()
        .args([
            "render-canon",
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
            "render-canon",
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

// AT3: cwd with Package.swift infers Swift; case-insensitive --stack override matches.
#[test]
fn render_canon_infers_swift_and_accepts_case_insensitive_override() {
    let cwd = new_fixture();
    fs::write(cwd.join("Package.swift"), "// swift-tools-version:6.0\n").unwrap();

    let inferred_target = new_fixture();
    let out = govna()
        .args([
            "render-canon",
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
            "render-canon",
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

// AT4: DOC flavor rejects --stack.
#[test]
fn render_canon_doc_rejects_stack() {
    let cwd = new_fixture();
    let target = new_fixture();
    let out = govna()
        .args([
            "render-canon",
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

// AT5: module-path is Go-only — rejected for DOC and for non-Go CODE stacks.
#[test]
fn render_canon_module_path_rejected_outside_go_code() {
    let cwd = new_fixture();

    let doc_target = new_fixture();
    let out = govna()
        .args([
            "render-canon",
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
            "render-canon",
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

// AT6: Go module path read from go.mod; explicit --module-path overrides it.
#[test]
fn render_canon_go_module_path_and_override() {
    let cwd = new_fixture();
    fs::write(cwd.join("go.mod"), "module example.com/thing\n\ngo 1.22\n").unwrap();

    let inferred_target = new_fixture();
    let out = govna()
        .args([
            "render-canon",
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
            "render-canon",
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

// AT7: .gitignore carries the stack ignore block; development-guidelines.md carries the
// stack guideline block above ## Project Practices.
#[test]
fn render_canon_stitches_gitignore_and_guidelines() {
    let cwd = new_fixture();
    fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
    let target = new_fixture();
    let out = govna()
        .args(["render-canon", "--flavor", "code", target.to_str().unwrap()])
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

// AT8: help output documents --stack and --module-path.
#[test]
fn render_canon_help_documents_flags() {
    let out = govna().args(["render-canon", "--help"]).output().unwrap();
    assert!(out.status.success());
    let stderr = String::from_utf8_lossy(&out.stderr);
    assert!(stderr.contains("-s, --stack <name>"), "{stderr}");
    assert!(stderr.contains("-m, --module-path <path>"), "{stderr}");
}

// AT9: AGENTS.md and govna/*.md are fully substituted — no leftover {{...}} tokens.
// Deliberately scoped to these two paths, not all rendered output: the Go stack's
// build.sh legitimately contains `{{.Path}}` (Go's own `go list -f` syntax).
#[test]
fn render_canon_output_is_fully_substituted() {
    let cwd = new_fixture();
    fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
    let target = new_fixture();
    let out = govna()
        .args(["render-canon", "--flavor", "code", target.to_str().unwrap()])
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

// AT10: DOC's rendered AGENTS.md differs from CODE's and references DOC-specific docs —
// proves the DOC overlay's AGENTS.md.tmpl overrides base/AGENTS.md, per the
// last-write-wins output-precedence rule, rather than base silently winning for both flavors.
#[test]
fn render_canon_doc_agents_overrides_base() {
    let doc_cwd = new_fixture();
    let doc_target = new_fixture();
    let out = govna()
        .args([
            "render-canon",
            "--flavor",
            "doc",
            doc_target.to_str().unwrap(),
        ])
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
        .args([
            "render-canon",
            "--flavor",
            "code",
            code_target.to_str().unwrap(),
        ])
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

// AT11: CLAUDE.md is a symlink to AGENTS.md, for both flavors (govna's deliberate
// divergence from governa parity — governa's own render-canon never creates this).
#[test]
fn render_canon_creates_claude_symlink() {
    let cwd = new_fixture();
    let target = new_fixture();
    let out = govna()
        .args(["render-canon", "--flavor", "doc", target.to_str().unwrap()])
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

// AT12: govna's own root docs no longer carry stale governa Go-implementation tokens;
// .gitignore and development-guidelines.md carry the Rust stitching; README shows
// render-canon as implemented.
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
        "drift-scan",
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
    assert!(
        readme.contains("| `render-canon` | implemented |"),
        "{readme}"
    );
}

// AT14: govna's own repo root carries no self-referential govna/metadata.txt,
// matching governa's own precedent (verified via governa's git history: it has
// never once committed a self-referential metadata.txt at its own root — the
// only copies that exist anywhere in its history are the two template sources).
#[test]
fn root_has_no_self_referential_metadata() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    assert!(!repo_root.join("govna/metadata.txt").exists());
}

// AT13 [Manual] — Director reads the rewritten govna/*.md root docs end-to-end and
// confirms the prose is accurate for govna's actual implementation. No automated
// coverage possible; tracked here as a marker only.

// ── drift-scan (AC5) fixtures ───────────────────────────────────────────────

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

/// A fresh `render-canon --flavor code --stack Rust` output, `git init`'d
/// but with nothing committed yet — so `git log` is empty for every path,
/// giving any subsequently-edited file a true zero-commit-history state
/// (`ClearSync`-eligible). A single full commit would *not* achieve this:
/// every file in that commit already has one entry in its own history.
fn rendered_code_fixture_no_commit() -> PathBuf {
    let dir = new_fixture();
    let out = govna()
        .args(["render-canon", "--flavor", "code", "--stack", "Rust", "."])
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

fn drift_scan_json(dir: &Path) -> serde_json::Value {
    let out = govna()
        .args(["drift-scan", "--json"])
        .current_dir(dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    serde_json::from_slice(&out.stdout).unwrap_or_else(|e| {
        panic!(
            "invalid JSON: {e}\n{}",
            String::from_utf8_lossy(&out.stdout)
        )
    })
}

fn file_result<'a>(report: &'a serde_json::Value, relpath: &str) -> Option<&'a serde_json::Value> {
    report["files"]
        .as_array()
        .unwrap()
        .iter()
        .find(|f| f["relpath"] == relpath)
}

// AT1: drift-scan refuses to run against govna's own source checkout —
// proves refuse_govna_source runs before require_govna_adopted, even though
// this repo would otherwise pass the positive adoption check (it has
// AGENTS.md + govna/ac-template.md). Safe against the real repo: the
// self-check is the very first thing run_inner does, before any writes.
#[test]
fn drift_scan_refuses_govna_source() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let out = govna()
        .arg("drift-scan")
        .current_dir(repo_root)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("looks like a govna checkout"));
}

// AT2: no AGENTS.md at all fails require_govna_adopted's exact wording.
#[test]
fn drift_scan_requires_agents_md() {
    let dir = new_fixture();
    git(&dir, &["init", "-q"]);
    let out = govna()
        .arg("drift-scan")
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("is not a govna-adopted repo"));
}

// AT3: passes require_govna_adopted (AGENTS.md + govna/ac-template.md) but
// has no .git/ — fails on the git-worktree requirement before classification.
#[test]
fn drift_scan_requires_git_worktree() {
    let dir = new_fixture();
    fs::write(dir.join("AGENTS.md"), "# AGENTS.md\n").unwrap();
    fs::create_dir_all(dir.join("govna")).unwrap();
    fs::write(dir.join("govna/ac-template.md"), "template\n").unwrap();
    let out = govna()
        .arg("drift-scan")
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("not a git worktree"));
}

// AT4: fresh, unmodified fixture — everything Match (or, byte-equal right
// after a fresh render, plan.md/arch.md also Match; they only classify
// ExpectedDivergence once actually customized), zero sync/migration/routing
// entries, "No sync items." in the emitted stub.
#[test]
fn drift_scan_fresh_fixture_all_match() {
    let dir = rendered_code_fixture();
    let report = drift_scan_json(&dir);
    for f in report["files"].as_array().unwrap() {
        assert_eq!(f["classification"], "match", "{f}");
    }
    let stub_path = dir.join(report["emitted"]["ac_stub"].as_str().unwrap());
    let stub = read(&stub_path);
    assert!(stub.contains("No sync items."), "{stub}");
}

// AT5: a committed edit to a non-format-defining file with git history
// classifies Ambiguity, routed to Routing Decisions (not silently synced).
#[test]
fn drift_scan_ambiguity_routes_to_review() {
    let dir = rendered_code_fixture();
    fs::write(
        dir.join("govna/roles.md"),
        format!("{}\nextra line\n", read(&dir.join("govna/roles.md"))),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "edit roles"]);
    let report = drift_scan_json(&dir);
    let fr = file_result(&report, "govna/roles.md").unwrap();
    assert_eq!(fr["classification"], "ambiguity");
    let stub_path = dir.join(report["emitted"]["ac_stub"].as_str().unwrap());
    let stub = read(&stub_path);
    assert!(stub.contains("### Routing Decisions"));
    assert!(stub.contains("govna/roles.md"));
}

// AT6: format-defining override. AGENTS.md edited above its canon-zone
// boundary, with committed history (raw classification Ambiguity — a
// zero-history ClearSync raw classification would ALSO force to sync, but
// the "Format-defining file routing" note only fires when the override
// actually changes the outcome, i.e. raw != ClearSync/MissingTarget; this
// scenario exercises that real branch, not a vacuous one).
#[test]
fn drift_scan_format_defining_forces_sync() {
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
    let report = drift_scan_json(&dir);
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
}

// AT7: a preserve marker in CHANGELOG.md suppresses sync — classifies
// Preserve, routed to Out Of Scope with the marker citation shown.
#[test]
fn drift_scan_preserve_marker_routes_to_out_of_scope() {
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
    let report = drift_scan_json(&dir);
    let fr = file_result(&report, "govna/roles.md").unwrap();
    assert_eq!(fr["classification"], "preserve");
    assert_eq!(fr["preserve_markers"][0], "preserve govna/roles.md");
    let stub_path = dir.join(report["emitted"]["ac_stub"].as_str().unwrap());
    let stub = read(&stub_path);
    assert!(stub.contains("## Out Of Scope"));
    assert!(stub.contains("preserve govna/roles.md"));
}

// AT8: mixed-content boundary. An edit strictly below `## Project Practices`
// (the repo-owned tail) classifies Match — canon-zone byte-equal, not a
// false divergence.
#[test]
fn drift_scan_mixed_content_below_boundary_matches() {
    let dir = rendered_code_fixture();
    let guidelines = read(&dir.join("govna/development-guidelines.md"));
    fs::write(
        dir.join("govna/development-guidelines.md"),
        format!("{guidelines}\nextra local note\n"),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "edit below boundary"]);
    let report = drift_scan_json(&dir);
    let fr = file_result(&report, "govna/development-guidelines.md").unwrap();
    assert_eq!(fr["classification"], "match", "{fr}");
}

// AT9: re-running immediately (unedited stub) reuses the same AC number;
// editing the stub's body then re-running fails with the edit-detection
// guard's exact wording.
#[test]
fn drift_scan_idempotent_reuse_and_edit_detection_guard() {
    let dir = rendered_code_fixture();
    let report1 = drift_scan_json(&dir);
    let stub_rel = report1["emitted"]["ac_stub"].as_str().unwrap().to_string();
    let report2 = drift_scan_json(&dir);
    assert_eq!(
        report1["emitted"]["ac_stub"], report2["emitted"]["ac_stub"],
        "AC number should be reused"
    );

    let stub_path = dir.join(&stub_rel);
    let tampered = format!("{}\ntampered\n", read(&stub_path));
    fs::write(&stub_path, tampered).unwrap();
    let out = govna()
        .arg("drift-scan")
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(
        String::from_utf8_lossy(&out.stderr)
            .contains("has been edited since last drift-scan emission")
    );
}

// AT10: cross-flavor orphan detection. A repo rendered DOC then re-rendered
// CODE over the same directory leaves DOC-only files orphaned; scanning
// with --flavor code classifies all of them target-has-no-canon.
#[test]
fn drift_scan_cross_flavor_orphans() {
    let dir = new_fixture();
    let out = govna()
        .args(["render-canon", "--flavor", "doc", "."])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let out = govna()
        .args(["render-canon", "--flavor", "code", "--stack", "Rust", "."])
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

    let report = drift_scan_json(&dir);
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

// AT11: name-reference body scan. A target-only file (no canon counterpart
// in either flavor) referenced via a backticked path from a divergent,
// git-tracked file classifies target-has-no-canon via the name-reference
// branch, not silently dropped.
#[test]
fn drift_scan_name_referenced_target_only_file() {
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

    let report = drift_scan_json(&dir);
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

// AT12: --json output is valid JSON, deserializes to the expected shape,
// and canon_content never appears in it.
#[test]
fn drift_scan_json_output_shape() {
    let dir = rendered_code_fixture();
    let report = drift_scan_json(&dir);
    assert!(report["header"]["canon_sha"].is_string());
    assert!(report["files"].is_array());
    assert!(!report["files"].as_array().unwrap().is_empty());
    let raw = serde_json::to_string(&report).unwrap();
    assert!(!raw.contains("canon_content"), "{raw}");
}

// AT13: `drift-scan extra-arg` fails with the exact "no positional
// arguments accepted" wording.
#[test]
fn drift_scan_rejects_positional_args() {
    let dir = rendered_code_fixture();
    let out = govna()
        .args(["drift-scan", "extra-arg"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("no positional arguments accepted"));
}

// AT14 [Manual] — Director runs drift-scan against a real, organically-
// drifted consumer repo and confirms the emitted AC stub reads sensibly
// end-to-end. No automated fixture substitutes for a genuinely messy real
// repo; tracked here as a marker only.

// CLI surface beyond the ATs above (closure-audit gap, not a named AT):
// --repo-name overrides the basename-of-cwd default.
#[test]
fn drift_scan_repo_name_override() {
    let dir = rendered_code_fixture();
    let out = govna()
        .args([
            "drift-scan",
            "--json",
            "--repo-name",
            "totally-different-name",
        ])
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
fn drift_scan_diff_lines_truncates() {
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
        .args(["drift-scan", "--json", "--diff-lines", "5"])
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
fn render_canon_metadata_txt_wins_over_manifest_inference() {
    let cwd = new_fixture();
    fs::create_dir_all(cwd.join("govna")).unwrap();
    fs::write(
        cwd.join("govna/metadata.txt"),
        "schema_version = 1\ngovna_version = v0.1.0\nrepo_type = DOC\n",
    )
    .unwrap();
    let target = new_fixture();
    let out = govna()
        .args(["render-canon", target.to_str().unwrap()])
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
fn render_canon_fallback_flavor_conflict_errors() {
    let cwd = new_fixture();
    fs::write(cwd.join("_config.yml"), "").unwrap();
    fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
    let target = new_fixture();
    let out = govna()
        .args(["render-canon", target.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("conflicting flavor signals"));
}

// Fallback flavor heuristic: no signals at all errors rather than defaulting.
#[test]
fn render_canon_fallback_flavor_absent_errors() {
    let cwd = new_fixture();
    let target = new_fixture();
    let out = govna()
        .args(["render-canon", target.to_str().unwrap()])
        .current_dir(&cwd)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("could not infer flavor"));
}

// Stack inference: *.tf glob fallback (no canonical Terraform manifest file present).
#[test]
fn render_canon_infers_terraform_from_tf_glob() {
    let cwd = new_fixture();
    fs::write(cwd.join("main.tf"), "resource \"null_resource\" \"x\" {}\n").unwrap();
    let target = new_fixture();
    let out = govna()
        .args(["render-canon", "--flavor", "code", target.to_str().unwrap()])
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

// ── apply (AC7) fixtures ────────────────────────────────────────────────────

// AT1: fresh empty target, apply -f code -s rust: writes AGENTS.md, CLAUDE.md
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

// AT2: fresh empty target, apply -f doc: writes the DOC-overlay set (no
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

// AT3: no flags, target has CODE manifests present — infers flavor and stack
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

// AT4: no resolvable flavor signal and no -f flag exits non-zero with an
// actionable error.
#[test]
fn apply_unresolvable_flavor_errors() {
    let dir = new_fixture();
    let out = govna().arg("apply").current_dir(&dir).output().unwrap();
    assert!(!out.status.success());
    let stderr = String::from_utf8_lossy(&out.stderr);
    assert!(stderr.contains("--flavor"), "{stderr}");
}

// AT5: re-running apply against a target that already has AGENTS.md prints
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

// AT5b: a fresh target with no .git/ at all (no --init-git passed) still
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

// AT6: CLAUDE.md already present as a regular (non-symlink) file with
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

// AT7: --init-git on a target with no .git/ runs git init; a second run
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

// AT8: apply against govna's own source checkout refuses with a non-zero
// exit, mirroring drift-scan's existing refusal test. Safe against the
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

// ── govna-source-only content exclusion (AC8) ───────────────────────────────

// AT5: a freshly apply'd CODE target's rendered AGENTS.md and
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

// AT9: the written govna/ac<N>-govna-apply.md contains all five required
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
    for heading in [
        "## Summary",
        "## In Scope",
        "## Out Of Scope",
        "## Acceptance Tests",
        "## Status",
    ] {
        assert!(ac.contains(heading), "{heading} missing:\n{ac}");
    }
    assert!(ac.contains("- `AGENTS.md` (canon file)"), "{ac}");
    assert!(ac.contains("- `CLAUDE.md` (agent alias link)"), "{ac}");
}

// ── rm (AC9) fixtures ────────────────────────────────────────────────────────
//
// Reuses `rendered_code_fixture()` (render-canon-based, no adoption AC) as
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

// AT1: fresh unmodified fixture — pure-canon files list as `delete file`,
// CLAUDE.md lists as `delete symlink`. No companion diffs file (AC10 Part C
// — rm no longer emits one).
#[test]
fn rm_fresh_fixture_pure_canon_deletes() {
    let dir = rendered_code_fixture();
    let out = govna().arg("rm").current_dir(&dir).output().unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    let stdout = String::from_utf8_lossy(&out.stdout);
    assert!(stdout.contains("ac1-govna-rm-"), "{stdout}");
    assert!(!stdout.contains("-diffs.md"), "{stdout}");
    assert!(dir.join("govna/ac1-govna-rm-v0.1.0.md").is_file());
    assert!(!dir.join("govna/ac1-govna-rm-v0.1.0-diffs.md").exists());

    let stub = read(&dir.join("govna/ac1-govna-rm-v0.1.0.md"));
    assert!(
        stub.contains("- `govna/roles.md` — delete file; byte-equal govna canon."),
        "{stub}"
    );
    assert!(
        stub.contains("- `CLAUDE.md` — delete symlink; govna compatibility link."),
        "{stub}"
    );
}

// AT2: hybrid files always route to Routing Decisions, even unmodified.
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

// AT3: plan.md/arch.md list under Out Of Scope as repo-owned govna-adjacent content.
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

// AT4: a preserve marker routes that file to Out Of Scope, not In Scope or Review.
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

// AT5: a target-only file lists under Out Of Scope as target-only repo-owned file.
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

// AT6: an edited non-hybrid canon file routes to Review as ambiguity, with
// an on-demand comparison recipe embedded in the bullet (AC10 Part C — no
// pre-computed diff, no companion diffs file).
#[test]
fn rm_edited_canon_file_routes_to_ambiguity() {
    let dir = rendered_code_fixture();
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
    let stub = read(&dir.join("govna/ac1-govna-rm-v0.1.0.md"));
    assert!(
        stub.contains("`govna/roles.md` is consumer-edited canon file"),
        "{stub}"
    );
    assert!(
        stub.contains(
            "Compare with: `govna render-canon --flavor code --stack Rust <scratch> && diff -ru <scratch>/govna/roles.md govna/roles.md`"
        ),
        "{stub}"
    );
    assert!(!dir.join("govna/ac1-govna-rm-v0.1.0-diffs.md").exists());
}

// AT7: re-running unedited reuses the same AC number for both files;
// editing either fails with the edit-detection guard's exact wording.
#[test]
fn rm_idempotent_reuse_and_edit_detection_guard() {
    let dir = rendered_code_fixture();
    let out1 = govna().arg("rm").current_dir(&dir).output().unwrap();
    assert!(out1.status.success());
    let stdout1 = String::from_utf8_lossy(&out1.stdout).to_string();

    let out2 = govna().arg("rm").current_dir(&dir).output().unwrap();
    assert!(out2.status.success());
    let stdout2 = String::from_utf8_lossy(&out2.stdout).to_string();
    assert_eq!(stdout1, stdout2, "AC number should be reused");

    let stub_path = dir.join("govna/ac1-govna-rm-v0.1.0.md");
    let tampered = format!("{}\ntampered\n", read(&stub_path));
    fs::write(&stub_path, tampered).unwrap();
    let out3 = govna().arg("rm").current_dir(&dir).output().unwrap();
    assert!(!out3.status.success());
    assert!(String::from_utf8_lossy(&out3.stderr).contains("has been edited since last emission"));
}

// AT8: rm against govna's own source checkout refuses with a non-zero exit.
#[test]
fn rm_refuses_govna_source() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let out = govna().arg("rm").current_dir(repo_root).output().unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("looks like a govna checkout"));
}

// AT9: no AGENTS.md fails require_govna_adopted's exact wording; no .git/
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

// AT10: --flavor override is honored — forcing --flavor doc on an
// otherwise CODE-shaped fixture drives classification off DOC's smaller
// canon set (govna/editing-guidelines.md, not development-guidelines.md).
#[test]
fn rm_flavor_override_changes_canon_set() {
    let dir = rendered_code_fixture();
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
    let stub = read(&dir.join("govna/ac1-govna-rm-v0.1.0.md"));
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

// AT11: `rm extra-arg` fails with the exact "no positional arguments
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

// ── migrate-from-governa (AC10 Part A) + hunk-merge (AC10 Part B) ──────────

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
/// `PATH` to make it discoverable). `render_canon_ok`: when true, `render-canon`
/// writes `governa/roles.md` with `roles_content` into the target and exits 0;
/// when false, `--version` still succeeds but `render-canon` exits 1
/// (simulating an unsupported stack or bad-flag failure).
fn fake_governa_binary(dir: &Path, render_canon_ok: bool, roles_content: &str) {
    use std::os::unix::fs::PermissionsExt;
    let script_path = dir.join("governa");
    // Matches governa's real CLI: `version` is a subcommand, not a `--version`
    // flag — `governa --version` actually exits non-zero on the real binary.
    let script = if render_canon_ok {
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

// AT1: governa/metadata.txt carries repo_type/code_stack; apply (no flags,
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

// AT2: precise tier. A fake `governa` binary on PATH renders roles.md;
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
    let ac = read(&dir.join("govna/ac2-govna-migrate-from-governa-v0.1.0.md"));
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
    let ac2 = read(&dir2.join("govna/ac2-govna-migrate-from-governa-v0.1.0.md"));
    assert!(
        ac2.contains(
            "Compare with: `governa render-canon --flavor code --stack Rust <scratch> && diff -ru <scratch>/governa/roles.md governa/roles.md`"
        ),
        "{ac2}"
    );
}

// AT3: crude tier. No `governa` binary on PATH at all: governa/roles.md
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
    let ac = read(&dir.join("govna/ac2-govna-migrate-from-governa-v0.1.0.md"));
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

// AT4: a `governa` binary that succeeds on --version but fails on
// render-canon (e.g. unsupported stack) falls back to the crude path —
// apply does not error or crash.
#[test]
fn apply_migration_falls_back_when_render_canon_fails() {
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
    let ac = read(&dir.join("govna/ac2-govna-migrate-from-governa-v0.1.0.md"));
    assert!(ac.contains("likely superseded by `govna/roles.md`"), "{ac}");
}

// AT5: re-running apply unedited reuses the same migration-AC number;
// editing it and re-running fails with the edit-detection guard's wording.
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
    assert!(
        dir.join("govna/ac2-govna-migrate-from-governa-v0.1.0.md")
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
        !dir.join("govna/ac3-govna-migrate-from-governa-v0.1.0.md")
            .exists(),
        "should reuse ac2, not allocate ac3"
    );

    let ac_path = dir.join("govna/ac2-govna-migrate-from-governa-v0.1.0.md");
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

fn count_migration_acs(dir: &Path) -> usize {
    fs::read_dir(dir.join("govna"))
        .unwrap()
        .filter_map(|e| e.ok())
        .filter(|e| {
            e.file_name()
                .to_string_lossy()
                .contains("migrate-from-governa")
        })
        .count()
}

// AT6: no governa/ directory at all — no migration AC emitted, behavior
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

// AT7: Part B — existing-mode apply on a repo whose AGENTS.md has extra
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

// AT8: Part B — existing-mode apply on an unmodified repo is a no-op merge:
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

// AT9: Part B — existing-mode apply leaves a customized README.md/CHANGELOG.md
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

// AT10: Part B — new-mode apply (fresh empty dir) is unaffected: writes
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
}

// AT11: Part B — an AGENTS.md missing the ## Project Rules boundary
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
