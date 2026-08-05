use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::atomic::{AtomicU32, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

#[test]
fn version_flag_is_exact() {
    let output = Command::new(env!("CARGO_BIN_EXE_govna"))
        .arg("--version")
        .output()
        .unwrap();
    assert!(output.status.success());
    assert_eq!(output.stdout, b"govna v0.2.0\n");
    assert!(output.stderr.is_empty());
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
        .arg("apply")
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
