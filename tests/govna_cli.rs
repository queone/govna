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

fn write_preserve_registry(dir: &Path, paths: &[&str]) {
    let mut entries = paths.to_vec();
    entries.sort_unstable();
    fs::write(
        dir.join("govna/preserve.txt"),
        format!(
            "govna-preserve-v1\n{}",
            entries
                .iter()
                .map(|path| format!("{path}\n"))
                .collect::<String>()
        ),
    )
    .unwrap();
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
        assert_ne!(fields[0], "govna/preserve.txt");
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

#[test]
fn rendered_agents_define_active_ac_exceptions() {
    let repo = Path::new(env!("CARGO_MANIFEST_DIR"));
    let override_instruction = "Follow an explicit Director workflow override without requiring contract-amendment language.";
    let stop_instruction =
        "Stop and ask when a request lacks authorization, scope, or required context.";
    let ac_first_instruction = "Treat every non-trivial change as AC-first work unless the Director explicitly overrides it.";
    let superseded_stop = "Stop and ask when a request bypasses a required govna gate or lacks required authorization, scope, or context.";

    for path in [
        "AGENTS.md",
        "templates/base/AGENTS.md",
        "templates/overlays/doc/files/AGENTS.md.tmpl",
    ] {
        let agents = read(&repo.join(path));
        for expected in [override_instruction, stop_instruction, ac_first_instruction] {
            assert_eq!(agents.matches(expected).count(), 1, "{path}: {expected}");
        }
        assert!(!agents.contains(superseded_stop), "{path}: {agents}");
        assert!(
            !agents.contains("### Director Workflow Override"),
            "{path}: {agents}"
        );
    }

    let mut rendered = Vec::new();

    for flavor in ["code", "doc"] {
        let cwd = new_fixture();
        if flavor == "code" {
            fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
        }
        let target = new_fixture();
        let out = govna()
            .args(["render", "--flavor", flavor, target.to_str().unwrap()])
            .current_dir(&cwd)
            .output()
            .unwrap();
        assert!(
            out.status.success(),
            "{flavor}: {}",
            String::from_utf8_lossy(&out.stderr)
        );
        rendered.push((flavor, target));
    }

    for (flavor, target) in &rendered {
        let agents = read(&target.join("AGENTS.md"));
        for expected in [override_instruction, stop_instruction, ac_first_instruction] {
            assert_eq!(agents.matches(expected).count(), 1, "{flavor}: {expected}");
        }
        assert!(!agents.contains(superseded_stop), "{flavor}: {agents}");
        assert!(
            !agents.contains("### Director Workflow Override"),
            "{flavor}: {agents}"
        );
        for expected in [
            "Treat changed-content integrity, AC-template structure, Instruction Style, and applicable Pre-Implementation Verification as the tests-in-the-same-pass gate when a change pass creates or edits only an active AC document.",
            "Validate AC-document-only Draft and Refine edits with the required document checks.",
            "Keep AC-document-only Draft and Refine edits outside canonical validation cycles.",
            "Reserve bare AC and AT identifiers for CHANGELOG rows, commit messages, active `govna/ac<N>-<slug>.md` documents, literal examples in `govna/ac-template.md`, and `Historical:` comments.",
            "Treat every other Markdown documentation file as out of bounds for bare AC, AT, Class, Part, Round, and IE identifiers.",
        ] {
            assert!(agents.contains(expected), "{flavor}: {agents}");
        }

        let metadata = read(&target.join("govna/metadata.txt"));
        let baseline = read(&target.join("govna/canon-baseline.txt"));
        assert!(metadata.contains("canon_version = v0.25.0"), "{metadata}");
        assert!(baseline.contains("canon_version = v0.25.0"), "{baseline}");
    }

    let code_agents = read(&rendered[0].1.join("AGENTS.md"));
    assert!(
        code_agents.contains(
            "Treat a change pass that creates or edits only an active AC document as outside a canonical validation cycle."
        ),
        "{code_agents}"
    );
    assert!(
        code_agents.contains(
            "Run `./build.sh` as the first validation command in every validation cycle."
        ),
        "{code_agents}"
    );

    let doc_agents = read(&rendered[1].1.join("AGENTS.md"));
    assert!(
        !doc_agents.contains(
            "Run `./build.sh` as the first validation command in every validation cycle."
        ),
        "{doc_agents}"
    );
}

#[test]
fn rendered_contracts_define_concise_reporting_and_ceremony_triage() {
    let repo = Path::new(env!("CARGO_MANIFEST_DIR"));
    let agent_rules = [
        "Evaluate AC ceremony during initial request triage when the Director has not selected Draft.",
        "Treat changes exceeding eight counted files as presumptively AC-worthy.",
        "Track the primary repository and current phase internally.",
        "Report the primary repository or current phase only for ancillary work, repository or phase ambiguity, a phase correction, or a repository switch.",
        "Assign Audit findings sequential identifiers in the form `F<#> [High|Medium|Low|Nit]`.",
        "Keep one finding sequence for the active AC lifecycle.",
        "Start a separate finding sequence for each standalone contract-integrity report.",
        "Open every substantive phase completion with one short outcome sentence.",
        "Suppress routine repository labels, phase labels, phase mechanics, expected skips, duplicated status, and expected no-action confirmations.",
        "Keep independently useful results, corrections, findings, and risks in separate bullets.",
        "Color `Verified:`, `Red-teamed:`, and `Not checked:` cyan only when the response channel explicitly supports native color or ANSI color.",
        "Place a section's sole item on the heading line without a bullet.",
    ];
    let role_rules = [
        "Assign stable severity-qualified finding identifiers under `AGENTS.md` Review Style.",
        "Keep each independently useful self-review item distinct.",
        "Place a sole self-review item on its heading line.",
        "Use terse flat bullets for multiple self-review items.",
        "Keep substantive summaries focused on task results and actionable exceptions.",
    ];

    for path in [
        "AGENTS.md",
        "templates/base/AGENTS.md",
        "templates/overlays/doc/files/AGENTS.md.tmpl",
    ] {
        let contents = read(&repo.join(path));
        for expected in agent_rules {
            assert!(contents.contains(expected), "{path}: {expected}");
        }
    }

    for path in [
        "govna/roles.md",
        "templates/overlays/code/files/govna/roles.md.tmpl",
        "templates/overlays/doc/files/govna/roles.md.tmpl",
    ] {
        let contents = read(&repo.join(path));
        for expected in role_rules {
            assert!(contents.contains(expected), "{path}: {expected}");
        }
    }

    for flavor in ["code", "doc"] {
        let cwd = new_fixture();
        if flavor == "code" {
            fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
        }
        let target = new_fixture();
        let out = govna()
            .args(["render", "--flavor", flavor, target.to_str().unwrap()])
            .current_dir(&cwd)
            .output()
            .unwrap();
        assert!(
            out.status.success(),
            "{flavor}: {}",
            String::from_utf8_lossy(&out.stderr)
        );

        let agents = read(&target.join("AGENTS.md"));
        for expected in agent_rules {
            assert!(agents.contains(expected), "{flavor}: {expected}");
        }
        let roles = read(&target.join("govna/roles.md"));
        for expected in role_rules {
            assert!(roles.contains(expected), "{flavor}: {expected}");
        }
        let readme = read(&target.join("README.md"));
        for contents in [&agents, &roles, &readme] {
            for name in [
                "Claude Code",
                "Anthropic",
                "OpenAI Codex",
                "ChatGPT",
                "GitHub Copilot",
                "Google Gemini",
            ] {
                assert!(!contents.contains(name), "{flavor}: {name}");
            }
        }
    }
}

#[test]
fn documentation_avoids_unnecessary_coding_agent_names() {
    let repo = Path::new(env!("CARGO_MANIFEST_DIR"));
    let prohibited = [
        "Claude Code",
        "Anthropic",
        "OpenAI Codex",
        "ChatGPT",
        "GitHub Copilot",
        "Google Gemini",
    ];

    for path in [
        "README.md",
        "AGENTS.md",
        "templates/base/AGENTS.md",
        "templates/overlays/doc/files/AGENTS.md.tmpl",
    ] {
        let contents = read(&repo.join(path));
        for name in prohibited {
            assert!(!contents.contains(name), "{path}: {name}");
        }
    }

    assert!(read(&repo.join("AGENTS.md")).contains("`CLAUDE.md`"));
    assert!(
        read(&repo.join("README.md"))
            .contains("The primary validation surface so far has been CLI-type coding agents.")
    );
}

#[test]
fn rendered_contract_bundle_is_compact_and_preserves_authority() {
    let repo = Path::new(env!("CARGO_MANIFEST_DIR"));
    for (source, template) in [
        (
            "govna/roles.md",
            "templates/overlays/code/files/govna/roles.md.tmpl",
        ),
        (
            "govna/development-cycle.md",
            "templates/overlays/code/files/govna/development-cycle.md.tmpl",
        ),
        (
            "govna/ac-template.md",
            "templates/overlays/code/files/govna/ac-template.md.tmpl",
        ),
        (
            "govna/audit.md",
            "templates/overlays/code/files/govna/audit.md.tmpl",
        ),
        (
            "govna/audit.md",
            "templates/overlays/doc/files/govna/audit.md.tmpl",
        ),
        (
            "govna/canon-cycle.md",
            "templates/overlays/code/files/govna/canon-cycle.md.tmpl",
        ),
        (
            "govna/canon-cycle.md",
            "templates/overlays/doc/files/govna/canon-cycle.md.tmpl",
        ),
        (
            "govna/code-stacks.md",
            "templates/overlays/code/files/govna/code-stacks.md.tmpl",
        ),
    ] {
        assert_eq!(
            read(&repo.join(source)),
            read(&repo.join(template)),
            "{template}"
        );
    }

    let root_agents = read(&repo.join("AGENTS.md"));
    let base_agents = read(&repo.join("templates/base/AGENTS.md"));
    assert_eq!(
        root_agents.split("## Project Rules").next(),
        base_agents.split("## Project Rules").next()
    );

    let mut bundles = Vec::new();
    for flavor in ["code", "doc"] {
        let cwd = new_fixture();
        if flavor == "code" {
            fs::write(cwd.join("Cargo.toml"), "[package]\nname = \"x\"\n").unwrap();
        }
        let target = new_fixture();
        let out = govna()
            .args(["render", "--flavor", flavor, target.to_str().unwrap()])
            .current_dir(&cwd)
            .output()
            .unwrap();
        assert!(
            out.status.success(),
            "{flavor}: {}",
            String::from_utf8_lossy(&out.stderr)
        );
        bundles.push((flavor, target));
    }

    let code_paths = [
        "AGENTS.md",
        "govna/roles.md",
        "govna/development-cycle.md",
        "govna/development-guidelines.md",
        "govna/build-release.md",
        "govna/audit.md",
        "govna/ac-template.md",
        "govna/canon-cycle.md",
        "govna/code-stacks.md",
        "govna/operator-contract-rationale.md",
    ];
    let doc_paths = [
        "AGENTS.md",
        "govna/roles.md",
        "govna/editing-cycle.md",
        "govna/editing-guidelines.md",
        "govna/release.md",
        "govna/audit.md",
        "govna/ac-template.md",
        "govna/canon-cycle.md",
        "govna/operator-contract-rationale.md",
    ];
    let counts: Vec<usize> = bundles
        .iter()
        .zip([&code_paths[..], &doc_paths[..]])
        .map(|((_, target), paths)| {
            paths
                .iter()
                .map(|path| read(&target.join(path)).split_whitespace().count())
                .sum()
        })
        .collect();
    assert!(
        counts[0] + counts[1] <= 22_880,
        "CODE={} DOC={} combined={}",
        counts[0],
        counts[1],
        counts[0] + counts[1]
    );

    struct AtomicInstruction {
        id: &'static str,
        original: &'static str,
        replacements: &'static [&'static str],
        targets: &'static [&'static str],
    }

    fn is_atomic(document: &str, instruction: &AtomicInstruction) -> bool {
        instruction
            .replacements
            .iter()
            .all(|replacement| document.lines().any(|line| line == *replacement))
            && !document.contains(instruction.original)
    }

    const ATOMIC_INSTRUCTIONS: &[AtomicInstruction] = &[
        AtomicInstruction {
            id: "A01",
            original: "- Classify repository-specific tools, architecture, release, content, or operating-preference findings as `Consumer-local`; route corrections to `## Project Rules` or the owning repo document.",
            replacements: &[
                "- Classify repository-specific tools, architecture, release, content, or operating-preference findings as `Consumer-local`.",
                "- Route `Consumer-local` corrections to `## Project Rules` or the owning repo document.",
            ],
            targets: &["source:AGENTS.md", "code:AGENTS.md", "doc:AGENTS.md"],
        },
        AtomicInstruction {
            id: "A02",
            original: "- Classify shared phase, approval, scope, role, canon, or template findings as `Govna canon`; route corrections to the authoritative source and every applicable consumer path.",
            replacements: &[
                "- Classify shared phase, approval, scope, role, canon, or template findings as `Govna canon`.",
                "- Route `Govna canon` corrections to the authoritative source and every applicable consumer path.",
            ],
            targets: &["source:AGENTS.md", "code:AGENTS.md", "doc:AGENTS.md"],
        },
        AtomicInstruction {
            id: "A03",
            original: "- Pause before any unnamed action; treat ambiguous, unrelated, or implicit replies as non-advancing feedback.",
            replacements: &[
                "- Pause before any unnamed action.",
                "- Treat ambiguous, unrelated, or implicit replies as non-advancing feedback.",
            ],
            targets: &["source:AGENTS.md", "code:AGENTS.md", "doc:AGENTS.md"],
        },
        AtomicInstruction {
            id: "T01",
            original: "Copy this file to `govna/ac<N>-<slug>.md`; use a kebab-case slug and `# AC<N> Title` heading.",
            replacements: &[
                "Copy this file to `govna/ac<N>-<slug>.md`.",
                "Use a kebab-case slug and `# AC<N> Title` heading.",
            ],
            targets: &[
                "source:govna/ac-template.md",
                "code:govna/ac-template.md",
                "doc:govna/ac-template.md",
            ],
        },
        AtomicInstruction {
            id: "T02",
            original: "Set `N` to one above the highest AC number in `govna/` or `git log --all --pretty=%B`; count every reference because release-prep deletions do not reset numbering.",
            replacements: &[
                "Set `N` to one above the highest AC number in `govna/` or `git log --all --pretty=%B`.",
                "Count every reference because release-prep deletions do not reset numbering.",
            ],
            targets: &[
                "source:govna/ac-template.md",
                "code:govna/ac-template.md",
                "doc:govna/ac-template.md",
            ],
        },
        AtomicInstruction {
            id: "T04",
            original: "List concrete changes and exact paths under useful groupings. Treat this list as authoritative; apply only the effective-scope and emitted-routing exceptions defined in `AGENTS.md`.",
            replacements: &[
                "List concrete changes and exact paths under useful groupings.",
                "Treat this list as authoritative.",
                "Apply only the effective-scope and emitted-routing exceptions defined in `AGENTS.md`.",
            ],
            targets: &[
                "source:govna/ac-template.md",
                "code:govna/ac-template.md",
                "doc:govna/ac-template.md",
            ],
        },
        AtomicInstruction {
            id: "T06",
            original: "Label every AT `[Automated]` or `[Manual]` and `[Pre-release gate]` or `[Post-release verification]`. Prefer automated pre-release coverage; use post-release only when automated regression coverage already gates the behavior class.",
            replacements: &[
                "Label every AT `[Automated]` or `[Manual]` and `[Pre-release gate]` or `[Post-release verification]`.",
                "Prefer automated pre-release coverage.",
                "Use post-release only when automated regression coverage already gates the behavior class.",
            ],
            targets: &[
                "source:govna/ac-template.md",
                "code:govna/ac-template.md",
                "doc:govna/ac-template.md",
            ],
        },
        AtomicInstruction {
            id: "C04",
            original: "4. **Alerting.** Surface updates through audit and hard-fail incoherent canon for maintainer correction.",
            replacements: &[
                "4. **Alerting.**",
                "   - Surface updates through audit.",
                "   - Hard-fail incoherent canon for maintainer correction.",
            ],
            targets: &[
                "source:govna/canon-cycle.md",
                "code:govna/canon-cycle.md",
                "doc:govna/canon-cycle.md",
            ],
        },
        AtomicInstruction {
            id: "C08",
            original: "4. **Boundaries.** Replace canon above `## Project Rules` in `AGENTS.md` and above `## Project Practices` in development/editing guidelines and CODE build-release; keep the boundary and local tail. Keep DOC release full canon.",
            replacements: &[
                "4. **Boundaries.**",
                "   - Replace canon above `## Project Rules` in `AGENTS.md`.",
                "   - Replace canon above `## Project Practices` in development/editing guidelines and CODE build-release.",
                "   - Keep each boundary and local tail.",
                "   - Keep DOC release full canon.",
            ],
            targets: &[
                "source:govna/canon-cycle.md",
                "code:govna/canon-cycle.md",
                "doc:govna/canon-cycle.md",
            ],
        },
        AtomicInstruction {
            id: "C10",
            original: "6. **Baseline.** Install and verify the baseline from the same scratch render only after other tests, routes, and validation pass; skip an immediate audit rerun.",
            replacements: &[
                "6. **Baseline.**",
                "   - Install the baseline from the same scratch render only after other tests, routes, and validation pass.",
                "   - Verify the baseline from that scratch render after installation.",
                "   - Skip an immediate audit rerun.",
            ],
            targets: &[
                "source:govna/canon-cycle.md",
                "code:govna/canon-cycle.md",
                "doc:govna/canon-cycle.md",
            ],
        },
        AtomicInstruction {
            id: "S02",
            original: "- Identify each utility by its stack-selected canonical target and require one strict stable SemVer declaration.",
            replacements: &[
                "- Identify each utility by its stack-selected canonical target.",
                "- Require one strict stable SemVer declaration for each utility.",
            ],
            targets: &["source:govna/code-stacks.md", "code:govna/code-stacks.md"],
        },
        AtomicInstruction {
            id: "S03",
            original: "- Validate declarations before compilation and compiled versions before installation or release metadata.",
            replacements: &[
                "- Validate declarations before compilation.",
                "- Validate compiled versions before installation.",
                "- Validate compiled versions before writing release metadata.",
            ],
            targets: &["source:govna/code-stacks.md", "code:govna/code-stacks.md"],
        },
        AtomicInstruction {
            id: "D04",
            original: "4. **Implement.** Deliver, test, verify, correct, and closure-audit the settled scope.",
            replacements: &[
                "4. **Implement.**",
                "   - Deliver the settled scope.",
                "   - Test the settled scope.",
                "   - Verify the settled scope.",
                "   - Correct implementation defects.",
                "   - Closure-audit the settled scope.",
            ],
            targets: &[
                "source:govna/development-cycle.md",
                "code:govna/development-cycle.md",
            ],
        },
        AtomicInstruction {
            id: "D05",
            original: "5. **Ratify.** Perform the Director-triggered final review and bounded correction behavior.",
            replacements: &[
                "5. **Ratify.**",
                "   - Perform the Director-triggered final review.",
                "   - Apply bounded correction behavior.",
            ],
            targets: &[
                "source:govna/development-cycle.md",
                "code:govna/development-cycle.md",
            ],
        },
        AtomicInstruction {
            id: "D08",
            original: "- Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`, architecture in `arch.md`, and repo governance in `AGENTS.md`.",
            replacements: &[
                "- Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.",
                "- Keep architecture in `arch.md`.",
                "- Keep repo governance in `AGENTS.md`.",
            ],
            targets: &[
                "source:govna/development-cycle.md",
                "code:govna/development-cycle.md",
            ],
        },
        AtomicInstruction {
            id: "D10",
            original: "- Keep ACs in `govna/ac<N>-<slug>.md` and summarize rather than reproduce them in chat.",
            replacements: &[
                "- Keep ACs in `govna/ac<N>-<slug>.md`.",
                "- Summarize ACs rather than reproduce them in chat.",
            ],
            targets: &[
                "source:govna/development-cycle.md",
                "code:govna/development-cycle.md",
            ],
        },
        AtomicInstruction {
            id: "D11",
            original: "- Mark an unscoped stub in `## Summary`, keep scope and tests TBD, and leave it `PENDING` until scoped.",
            replacements: &[
                "- Mark an unscoped stub in `## Summary`.",
                "- Keep an unscoped stub's scope and tests TBD.",
                "- Leave an unscoped stub `PENDING` until scoped.",
            ],
            targets: &[
                "source:govna/development-cycle.md",
                "code:govna/development-cycle.md",
            ],
        },
        AtomicInstruction {
            id: "G01",
            original: "Use these durable coding practices; use `AGENTS.md`, `development-cycle.md`, and `build-release.md` for workflow, validation, and Package.",
            replacements: &[
                "Use these durable coding practices.",
                "Use `AGENTS.md`, `development-cycle.md`, and `build-release.md` for workflow, validation, and Package.",
            ],
            targets: &[
                "source:govna/development-guidelines.md",
                "code:govna/development-guidelines.md",
            ],
        },
        AtomicInstruction {
            id: "G03",
            original: "- Ship behavior docs with code, verify every referenced symbol or path, and keep `arch.md` limited to built architecture.",
            replacements: &[
                "- Ship behavior docs with code.",
                "- Verify every referenced symbol or path.",
                "- Keep `arch.md` limited to built architecture.",
            ],
            targets: &[
                "source:govna/development-guidelines.md",
                "code:govna/development-guidelines.md",
            ],
        },
        AtomicInstruction {
            id: "R05",
            original: "- Present release commands for the Director; never execute them.",
            replacements: &[
                "- Present release commands for the Director.",
                "- Never execute release commands.",
            ],
            targets: &[
                "source:govna/roles.md",
                "code:govna/roles.md",
                "doc:govna/roles.md",
            ],
        },
        AtomicInstruction {
            id: "R07",
            original: "- Red-team completed work and challenge assumptions or underspecified behavior.",
            replacements: &[
                "- Red-team completed work.",
                "- Challenge assumptions or underspecified behavior.",
            ],
            targets: &[
                "source:govna/roles.md",
                "code:govna/roles.md",
                "doc:govna/roles.md",
            ],
        },
        AtomicInstruction {
            id: "R08",
            original: "- Cite findings by file and line, order them by severity, and use objective language.",
            replacements: &[
                "- Cite findings by file and line.",
                "- Order findings by severity.",
                "- Use objective review language.",
            ],
            targets: &[
                "source:govna/roles.md",
                "code:govna/roles.md",
                "doc:govna/roles.md",
            ],
        },
        AtomicInstruction {
            id: "R14",
            original: "- Cite non-trivial findings and state explicitly when a section has none.",
            replacements: &[
                "- Cite non-trivial findings.",
                "- State explicitly when a section has no findings.",
            ],
            targets: &[
                "source:govna/roles.md",
                "code:govna/roles.md",
                "doc:govna/roles.md",
            ],
        },
        AtomicInstruction {
            id: "R20",
            original: "- Use one-line acknowledgments for trivial signals and structured summaries for substantive completions or Director decisions.",
            replacements: &[
                "- Use one-line acknowledgments for trivial signals.",
                "- Use structured summaries for substantive completions or Director decisions.",
            ],
            targets: &[
                "source:govna/roles.md",
                "code:govna/roles.md",
                "doc:govna/roles.md",
            ],
        },
        AtomicInstruction {
            id: "E04",
            original: "4. **Implement.** Deliver, verify, correct, and closure-audit the settled content scope.",
            replacements: &[
                "4. **Implement.**",
                "   - Deliver the settled content scope.",
                "   - Verify the settled content scope.",
                "   - Correct content defects.",
                "   - Closure-audit the settled content scope.",
            ],
            targets: &["doc:govna/editing-cycle.md"],
        },
        AtomicInstruction {
            id: "E05",
            original: "5. **Ratify.** Perform the Director-triggered final review and bounded correction behavior.",
            replacements: &[
                "5. **Ratify.**",
                "   - Perform the Director-triggered final review.",
                "   - Apply bounded correction behavior.",
            ],
            targets: &["doc:govna/editing-cycle.md"],
        },
        AtomicInstruction {
            id: "E08",
            original: "- Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md` and repo governance in `AGENTS.md`.",
            replacements: &[
                "- Keep roadmap decisions and follow-on `IE<N>:` items in `plan.md`.",
                "- Keep repo governance in `AGENTS.md`.",
            ],
            targets: &["doc:govna/editing-cycle.md"],
        },
        AtomicInstruction {
            id: "E10",
            original: "- Keep ACs in `govna/ac<N>-<slug>.md` and summarize rather than reproduce them in chat.",
            replacements: &[
                "- Keep ACs in `govna/ac<N>-<slug>.md`.",
                "- Summarize ACs rather than reproduce them in chat.",
            ],
            targets: &["doc:govna/editing-cycle.md"],
        },
        AtomicInstruction {
            id: "E11",
            original: "- Mark an unscoped stub in `## Summary`, keep scope and tests TBD, and leave it `PENDING` until scoped.",
            replacements: &[
                "- Mark an unscoped stub in `## Summary`.",
                "- Keep an unscoped stub's scope and tests TBD.",
                "- Leave an unscoped stub `PENDING` until scoped.",
            ],
            targets: &["doc:govna/editing-cycle.md"],
        },
        AtomicInstruction {
            id: "O01",
            original: "Use these durable content practices; use `AGENTS.md`, `editing-cycle.md`, and `release.md` for workflow, validation, and Package.",
            replacements: &[
                "Use these durable content practices.",
                "Use `AGENTS.md`, `editing-cycle.md`, and `release.md` for workflow, validation, and Package.",
            ],
            targets: &["doc:govna/editing-guidelines.md"],
        },
        AtomicInstruction {
            id: "O02",
            original: "- Ship docs with content changes, verify referenced paths and headings, and update every affected doc in one pass.",
            replacements: &[
                "- Ship docs with content changes.",
                "- Verify referenced paths and headings.",
                "- Update every affected doc in one pass.",
            ],
            targets: &["doc:govna/editing-guidelines.md"],
        },
        AtomicInstruction {
            id: "O04",
            original: "Run `./build.sh vX.Y.Z \"release message\"` to show status and planned Git steps, then prompt before `git add → commit → annotated tag → push tag → push branch`.",
            replacements: &[
                "Run `./build.sh vX.Y.Z \"release message\"` to show status and planned Git steps.",
                "Require its confirmation prompt before `git add → commit → annotated tag → push tag → push branch`.",
            ],
            targets: &["doc:govna/release.md"],
        },
        AtomicInstruction {
            id: "O05",
            original: "Keep DOC release prep repository-wide, reject CODE target selection, and limit release messages to 80 characters.",
            replacements: &[
                "Keep DOC release prep repository-wide.",
                "Reject CODE target selection.",
                "Limit release messages to 80 characters.",
            ],
            targets: &["doc:govna/release.md"],
        },
        AtomicInstruction {
            id: "S04",
            original: "- Infer Go from `go.mod`; select it explicitly with `--stack Go`.",
            replacements: &[
                "- Infer Go from `go.mod`.",
                "- Select Go explicitly with `--stack Go`.",
            ],
            targets: &["source:govna/code-stacks.md", "code:govna/code-stacks.md"],
        },
        AtomicInstruction {
            id: "S05",
            original: "- Infer Rust from `Cargo.toml`; select it explicitly with `--stack Rust`.",
            replacements: &[
                "- Infer Rust from `Cargo.toml`.",
                "- Select Rust explicitly with `--stack Rust`.",
            ],
            targets: &["source:govna/code-stacks.md", "code:govna/code-stacks.md"],
        },
        AtomicInstruction {
            id: "S06",
            original: "- Infer Terraform from `.terraform.lock.hcl` or root Terraform files; select it explicitly with `--stack Terraform`.",
            replacements: &[
                "- Infer Terraform from `.terraform.lock.hcl` or root Terraform files.",
                "- Select Terraform explicitly with `--stack Terraform`.",
            ],
            targets: &["source:govna/code-stacks.md", "code:govna/code-stacks.md"],
        },
        AtomicInstruction {
            id: "S07",
            original: "- Infer Swift from a root `Package.swift`; select it explicitly with `--stack Swift`.",
            replacements: &[
                "- Infer Swift from a root `Package.swift`.",
                "- Select Swift explicitly with `--stack Swift`.",
            ],
            targets: &["source:govna/code-stacks.md", "code:govna/code-stacks.md"],
        },
        AtomicInstruction {
            id: "D12",
            original: "- Treat standalone `Ratify` or `ratify` as the Director acceptance action that initiates the final review and completes Ratify when that review is clean.",
            replacements: &[
                "- Treat standalone `Ratify` or `ratify` as the Director acceptance action.",
                "- Initiate the final review on that action.",
                "- Complete Ratify when that review is clean.",
            ],
            targets: &[
                "source:govna/development-cycle.md",
                "code:govna/development-cycle.md",
                "doc:govna/editing-cycle.md",
            ],
        },
        AtomicInstruction {
            id: "R21",
            original: "- Run `./build.sh` only when reviewing code changes or when build output is itself part of the claim under review. Skip it for AC critique, doc-only review, and design discussion.",
            replacements: &[
                "- Run `./build.sh` only when reviewing code changes or build-output claims.",
                "- Skip `./build.sh` for AC critique, doc-only review, and design discussion.",
            ],
            targets: &["source:govna/roles.md", "code:govna/roles.md"],
        },
        AtomicInstruction {
            id: "R22",
            original: "- Run `./build.sh` only when reviewing build-relevant changes or when build output is itself part of the claim under review. Skip it for AC critique, doc-only review, and design discussion.",
            replacements: &[
                "- Run `./build.sh` only when reviewing build-relevant changes or build-output claims.",
                "- Skip `./build.sh` for AC critique, doc-only review, and design discussion.",
            ],
            targets: &["doc:govna/roles.md"],
        },
        AtomicInstruction {
            id: "B01",
            original: "4. **Detect and validate version targets.** Follow this repository's Project Practices and stack build implementation. Reject missing, malformed, duplicate, or unsafe targets before any write.",
            replacements: &[
                "4. **Process version targets.**",
                "   - Detect every version target.",
                "   - Validate every version target.",
                "   - Follow this repository's Project Practices.",
                "   - Follow the stack build implementation.",
                "   - Reject missing, malformed, duplicate, or unsafe targets before any write.",
            ],
            targets: &[
                "source:govna/build-release.md",
                "code:govna/build-release.md",
            ],
        },
        AtomicInstruction {
            id: "B02",
            original: "- Summaries are single-line, ≤ 500 characters; lead with the AC reference if any.",
            replacements: &[
                "- Keep summaries single-line and no longer than 500 characters.",
                "- Lead summaries with the AC reference when one exists.",
            ],
            targets: &[
                "source:govna/build-release.md",
                "code:govna/build-release.md",
                "doc:govna/release.md",
            ],
        },
        AtomicInstruction {
            id: "B03",
            original: "5. **Detect CHANGELOG targets + fail-fast idempotency guard.** Root `CHANGELOG.md`. If it already contains a row for the target version, prep exits with a fatal error before any writes.",
            replacements: &[
                "5. **Guard CHANGELOG idempotency.**",
                "   - Detect the root `CHANGELOG.md` target.",
                "   - Reject an existing row for the target version before any write.",
            ],
            targets: &[
                "source:govna/build-release.md",
                "code:govna/build-release.md",
            ],
        },
        AtomicInstruction {
            id: "O06",
            original: "1. **Verify all in-scope AC work is complete.** Every AT in the AC has been run and passes.",
            replacements: &[
                "1. **Verify completion.**",
                "   - Verify all in-scope AC work is complete.",
                "   - Verify every AC acceptance test passes.",
            ],
            targets: &["doc:govna/release.md"],
        },
    ];

    assert_eq!(ATOMIC_INSTRUCTIONS.len(), 44);
    for (index, instruction) in ATOMIC_INSTRUCTIONS.iter().enumerate() {
        for other in &ATOMIC_INSTRUCTIONS[index + 1..] {
            assert_ne!(instruction.id, other.id, "duplicate atomic instruction ID");
        }
        assert!(instruction.replacements.len() >= 2, "{}", instruction.id);
        for target in instruction.targets {
            let document = if let Some(path) = target.strip_prefix("source:") {
                read(&repo.join(path))
            } else if let Some(path) = target.strip_prefix("code:") {
                read(&bundles[0].1.join(path))
            } else if let Some(path) = target.strip_prefix("doc:") {
                read(&bundles[1].1.join(path))
            } else {
                panic!("{}: invalid target {target}", instruction.id);
            };
            assert!(
                is_atomic(&document, instruction),
                "{}: {target}",
                instruction.id
            );

            let mut mutant =
                document.replacen(instruction.replacements[0], instruction.original, 1);
            for replacement in &instruction.replacements[1..] {
                mutant = mutant.replacen(replacement, "", 1);
            }
            assert!(
                !is_atomic(&mutant, instruction),
                "{} mutant passed: {target}",
                instruction.id
            );
        }
    }

    const LEGACY_AGENT_REWRITES: &[(&str, &str, &str)] = &[
        (
            "A04",
            "- Edit sections in place; change section order or the `##` section list only when the user explicitly requests a contract amendment.",
            "- Edit sections in place.\n- Change section order or the `##` section list only when the user explicitly requests a contract amendment.",
        ),
        (
            "A05",
            "- Use `##` for top-level sections and `###` for thematic groupings inside a section; cap header nesting at `###`.",
            "- Use `##` for top-level sections.\n- Use `###` for thematic groupings inside a section.\n- Cap header nesting at `###`.",
        ),
        (
            "A06",
            "- Treat AGENTS.md as the authoritative source for the rules it describes; conform overlay templates and other canon files to it — `govna audit` catches violations (see `### Audit Adoption`).",
            "- Treat AGENTS.md as the authoritative source for the rules it describes.\n- Conform overlay templates and other canon files to AGENTS.md.",
        ),
        (
            "A07",
            "- Place each structured deliverable (AC, plan, doc draft, scope card) in its target file; never paste the full body in chat.",
            "- Place each structured deliverable (AC, plan, doc draft, scope card) in its target file.\n- Never paste a structured deliverable's full body in chat.",
        ),
        (
            "A08",
            "- Treat each authorization as scope-limited; require fresh approval for any new action, even when similar to a prior approved one.",
            "- Treat each authorization as scope-limited.\n- Require fresh approval for every new action.",
        ),
        (
            "A09",
            "- Draft `govna/ac<N>-<slug>.md` before implementation using `govna/ac-template.md`; define scope, out-of-scope, and acceptance tests.",
            "- Draft `govna/ac<N>-<slug>.md` before implementation using `govna/ac-template.md`.\n- Define scope, out-of-scope, and acceptance tests in the AC.",
        ),
        (
            "A10",
            "- Keep Audit non-mutating; do not edit the AC or repository during Audit.",
            "- Keep Audit non-mutating.\n- Do not edit the AC or repository during Audit.",
        ),
        (
            "A11",
            "- Start `Package` only after an explicit Director request; do not infer it from Ratify acceptance.",
            "- Start `Package` only after an explicit Director request.\n- Do not infer Package from Ratify acceptance.",
        ),
        (
            "A12",
            "- Treat standalone `Draft` or `draft` as the pre-cycle action that creates the active AC; require the Director to authorize it before creating the AC.",
            "- Treat standalone `Draft` or `draft` as the pre-cycle action that creates the active AC.\n- Require the Director to authorize Draft before creating the AC.",
        ),
        (
            "A13",
            "- Require explicit operational wording such as `run ./build.sh` before executing a repository command; never infer a shell command from an action name.",
            "- Require explicit operational wording such as `run ./build.sh` before executing a repository command.\n- Never infer a shell command from an action name.",
        ),
        (
            "A14",
            "- List ✓ for each check and flag any gaps; authorize implementation only when clean.",
            "- List ✓ for each check.\n- Flag every gap.\n- Authorize implementation only when the checklist is clean.",
        ),
        (
            "A15",
            "- Complete every mid-implementation decision change in one pass — files, docs, and tests together; never leave a half-migrated state.",
            "- Complete every mid-implementation decision change in one pass across files, docs, and tests.\n- Never leave a half-migrated state.",
        ),
        (
            "A16",
            "- Record follow-on improvements in `plan.md` (or note them to the user if no planning artifact exists); keep the current task strictly within its authorized scope.",
            "- Record follow-on improvements in `plan.md`.\n- Note follow-on improvements to the user when no planning artifact exists.\n- Keep the current task strictly within its authorized scope.",
        ),
        (
            "A17",
            "- **Record every authorized correction about repo behavior as an edit to the governance doc that owns the topic; never as a memory entry, `feedback.md`, or session note.**",
            "- **Record every authorized repository-behavior correction in its owning governance document.**\n- **Never record a repository-behavior correction as a memory entry, `feedback.md`, or session note.**",
        ),
        (
            "A18",
            "- Lead each review with findings and cite file paths and concrete behavior; skip preamble summaries.",
            "- Lead each review with findings.\n- Cite file paths and concrete behavior.\n- Skip preamble summaries.",
        ),
        (
            "A19",
            "- Report \"no issues\" directly when none are found; note any residual risk or verification gaps.",
            "- Report \"no issues\" directly when none are found.\n- Note every residual risk or verification gap.",
        ),
        (
            "A20",
            "- Never prescribe commit, push, or release actions in Ratify; the Director triggers those — Ratify names what's pending, not what to do.",
            "- Never prescribe commit, push, or release actions in Ratify.",
        ),
        (
            "A21",
            "- Default to plain text and simple bullets; reach for tables or richer structure only when content clearly benefits.",
            "- Default to plain text and simple bullets.\n- Use tables or richer structure only when content clearly benefits.",
        ),
        (
            "A22",
            "- Run required validation gates, but report successful routine gates only when they materially affect confidence; always report failures and skipped required gates.",
            "- Run every required validation gate.\n- Report successful routine gates only when they materially affect confidence.\n- Always report failures and skipped required gates.",
        ),
        (
            "A23",
            "- Keep `Verified:`, `Red-teamed:`, `Not checked:`, and `Run below to release:` in the Package completion report; state `No commit or release command executed.` and present the exact drafted release command.",
            "- Keep `Verified:`, `Red-teamed:`, `Not checked:`, and `Run below to release:` in the Package completion report.\n- State `No commit or release command executed.`.\n- Present the exact drafted release command.",
        ),
        (
            "A24",
            "- Pin dependencies to explicit versions; document any reason to stay on an older version.",
            "- Pin dependencies to explicit versions.\n- Document every reason to stay on an older dependency version.",
        ),
        (
            "A25",
            "- Use the `Historical:` prefix on a comment only when it references a shipped AC and the context aids the reader; delete the reference if no longer relevant.",
            "- Use the `Historical:` prefix only for a relevant shipped-AC comment.\n- Delete an irrelevant shipped-AC reference.",
        ),
        (
            "A26",
            "- Pair every new CLI flag with a one-letter short form (standard, leads help output) and a long-form alias; migrate existing flags when their code is next touched.",
            "- Pair every new CLI flag with a leading one-letter short form and a long-form alias.\n- Migrate existing flags when their code is next touched.",
        ),
        (
            "A27",
            "- Reuse content from files already in conversation context; reach for `Read` only to fetch unseen content or check for recent changes.",
            "- Reuse content from files already in conversation context.\n- Reach for `Read` only to fetch unseen content or check for recent changes.",
        ),
    ];

    for (id, original, replacement_block) in LEGACY_AGENT_REWRITES {
        let mut documents = vec![
            read(&repo.join("AGENTS.md")),
            read(&bundles[0].1.join("AGENTS.md")),
        ];
        if !matches!(*id, "A06" | "A15" | "A24" | "A26") {
            documents.push(read(&bundles[1].1.join("AGENTS.md")));
        }
        for document in documents {
            let replacements: Vec<&str> = replacement_block.lines().collect();
            assert!(!document.contains(original), "{id}: original remains");
            assert!(
                replacements
                    .iter()
                    .all(|replacement| document.lines().any(|line| line == *replacement)),
                "{id}: replacement missing"
            );
            let mut mutant = document.replacen(replacements[0], original, 1);
            for replacement in &replacements[1..] {
                mutant = mutant.replacen(replacement, "", 1);
            }
            assert!(mutant.contains(original), "{id}: mutation failed");
        }
    }

    for (id, original, replacement_block) in [
        (
            "A06D",
            "- Treat AGENTS.md as the authoritative source for the rules it describes; conform overlay templates and other canon files to it — `govna audit` catches violations.",
            "- Treat AGENTS.md as the authoritative source for the rules it describes.\n- Conform overlay templates and other canon files to AGENTS.md.",
        ),
        (
            "A15D",
            "- Complete every mid-implementation decision change in one pass — files and docs together; never leave a half-migrated state.",
            "- Complete every mid-implementation decision change in one pass across files and docs.\n- Never leave a half-migrated state.",
        ),
    ] {
        let document = read(&bundles[1].1.join("AGENTS.md"));
        let replacements: Vec<&str> = replacement_block.lines().collect();
        assert!(!document.contains(original), "{id}: original remains");
        assert!(
            replacements
                .iter()
                .all(|replacement| document.lines().any(|line| line == *replacement)),
            "{id}: replacement missing"
        );
        let mut mutant = document.replacen(replacements[0], original, 1);
        for replacement in &replacements[1..] {
            mutant = mutant.replacen(replacement, "", 1);
        }
        assert!(mutant.contains(original), "{id}: mutation failed");
    }

    for (id, original, replacement_block, path) in [
        (
            "C11",
            "- Require `schema_version`, `canon_version`, and `repo_type`; require `code_stack` only for CODE consumers.",
            "- Require `schema_version`, `canon_version`, and `repo_type`.\n- Require `code_stack` only for CODE consumers.",
            "govna/canon-cycle.md",
        ),
        (
            "S08",
            "- Bump the single detected `programVersion` during release prep; validate and preserve independent utility versions in multi-utility repositories.",
            "- Bump the single detected `programVersion` during release prep.\n- Validate independent utility versions in multi-utility repositories.\n- Preserve independent utility versions in multi-utility repositories.",
            "govna/code-stacks.md",
        ),
        (
            "S09",
            "- Prefer Go, Terraform, and Rust manifests over Swift; prefer Swift over Node, Python, and Java manifests.",
            "- Prefer Go, Terraform, and Rust manifests over Swift.\n- Prefer Swift over Node, Python, and Java manifests.",
            "govna/code-stacks.md",
        ),
        (
            "S10",
            "- Keep `Package.resolved` tracked for leaf packages with dependencies; treat it as optional for dependency libraries.",
            "- Keep `Package.resolved` tracked for leaf packages with dependencies.\n- Treat `Package.resolved` as optional for dependency libraries.",
            "govna/code-stacks.md",
        ),
        (
            "G04",
            "- Prefer surrogate keys for internal identity; keep external IDs as indexed attributes",
            "- Prefer surrogate keys for internal identity\n- Keep external IDs as indexed attributes",
            "govna/development-guidelines.md",
        ),
        (
            "G05",
            "- Validate external data at the boundary; do not trust upstream shape or completeness",
            "- Validate external data at the boundary\n- Treat upstream shape and completeness as untrusted",
            "govna/development-guidelines.md",
        ),
        (
            "G06",
            "- Cache external data locally with explicit TTL or versioning; never silently serve stale data as fresh",
            "- Cache external data locally with explicit TTL or versioning\n- Never silently serve stale data as fresh",
            "govna/development-guidelines.md",
        ),
        (
            "G07",
            "- Keep `build.sh` self-contained; do not add sourced production helper modules.",
            "- Keep `build.sh` self-contained.\n- Do not add sourced production helper modules.",
            "govna/development-guidelines.md",
        ),
        (
            "G08",
            "- Validate at system boundaries (user input, external APIs, file I/O); trust internal code",
            "- Validate at system boundaries (user input, external APIs, file I/O)\n- Trust internal code",
            "govna/development-guidelines.md",
        ),
        (
            "G09",
            "- Each flag line is indented 2 spaces; descriptions align at column 38",
            "- Indent each flag line by 2 spaces\n- Align descriptions at column 38",
            "govna/development-guidelines.md",
        ),
    ] {
        for document in [read(&repo.join(path)), read(&bundles[0].1.join(path))] {
            let replacements: Vec<&str> = replacement_block.lines().collect();
            assert!(!document.contains(original), "{id}: original remains");
            assert!(
                replacements
                    .iter()
                    .all(|replacement| document.lines().any(|line| line == *replacement)),
                "{id}: replacement missing"
            );
            let mut mutant = document.replacen(replacements[0], original, 1);
            for replacement in &replacements[1..] {
                mutant = mutant.replacen(replacement, "", 1);
            }
            assert!(mutant.contains(original), "{id}: mutation failed");
        }
    }

    for (id, original, replacement_block, path) in [
        (
            "O07",
            "- Verify links are not stale before publishing; flag broken links as findings during review",
            "- Verify links are not stale before publishing\n- Flag broken links as findings during review",
            "govna/editing-guidelines.md",
        ),
        (
            "O08",
            "- Keep one topic per file; split when a file covers unrelated concerns",
            "- Keep one topic per file\n- Split files that cover unrelated concerns",
            "govna/editing-guidelines.md",
        ),
        (
            "O09",
            "- Use flat `##` sections with inline bullets; avoid deep nesting unless structure genuinely requires it",
            "- Use flat `##` sections with inline bullets\n- Avoid deep nesting unless structure genuinely requires it",
            "govna/editing-guidelines.md",
        ),
        (
            "O10",
            "- Use consistent terminology throughout the repo; define terms in one place and reference that definition",
            "- Use consistent terminology throughout the repo\n- Define each term in one place\n- Reference the canonical definition",
            "govna/editing-guidelines.md",
        ),
    ] {
        let document = read(&bundles[1].1.join(path));
        let replacements: Vec<&str> = replacement_block.lines().collect();
        assert!(!document.contains(original), "{id}: original remains");
        assert!(
            replacements
                .iter()
                .all(|replacement| document.lines().any(|line| line == *replacement)),
            "{id}: replacement missing"
        );
        let mut mutant = document.replacen(replacements[0], original, 1);
        for replacement in &replacements[1..] {
            mutant = mutant.replacen(replacement, "", 1);
        }
        assert!(mutant.contains(original), "{id}: mutation failed");
    }

    for (id, original, replacement_block, target) in [
        (
            "B04",
            "1. **Run the stack-defined `./build.sh prep vX.Y.Z \"message\"` invocation.** Stages version bumps, inserts the CHANGELOG row, deletes completed AC files, sweeps matching AC-pointer IE lines from `plan.md`, runs stack-defined validation, and prints the canonical release command. The agent determines the version (semver classification from the AC's scope) and drafts the release message (≤ 80 characters) before invoking prep. Flags: `--validation-token`/`-t` passes current validation evidence when supported; `--dry-run`/`-n` prints intended writes without touching the working tree; `--no-build`/`-B` follows the applicable stack policy.",
            "1. **Run prep.**\n   - Classify the AC scope under semver.\n   - Draft a release message no longer than 80 characters.\n   - Run the stack-defined `./build.sh prep vX.Y.Z \"message\"` invocation.\n   - Pass current validation evidence with `--validation-token` or `-t` when supported.\n   - Use `--dry-run` or `-n` to inspect without writes.\n   - Use `--no-build` or `-B` only under the applicable stack policy.",
            "code",
        ),
        (
            "B05",
            "2. **Run the printed release command (`./build.sh vX.Y.Z \"message\"`).** Shows `git status --short`, lists every git step it will execute, and prompts for interactive confirmation. On approval it orchestrates `git add → commit → tag → push tag → push branch`.",
            "2. **Run the printed release command.**\n   - Run `./build.sh vX.Y.Z \"message\"`.\n   - Confirm the displayed status and Git steps.\n   - Approve the interactive prompt to execute the displayed sequence.",
            "code",
        ),
        (
            "B06",
            "7. **Apply writes.** Version bumps (per-file idempotent no-op when the file already has the target value); CHANGELOG row insertion under `| Unreleased | |`; AC file deletions (AC files are deleted whole; there are no separate companion files); AC-pointer IE-line sweep from `plan.md` (lines matching `→ govna/ac<N>-` for each released AC). Skipped when `--dry-run`/`-n`. Idempotent re-runs leave already-swept lines alone.",
            "7. **Apply writes.**\n   - Apply idempotent version bumps.\n   - Insert the CHANGELOG row under `| Unreleased | |`.\n   - Delete each released AC file whole.\n   - Sweep matching AC-pointer IE lines from `plan.md`.\n   - Skip writes under `--dry-run` or `-n`.\n   - Leave already-swept lines unchanged on rerun.",
            "code",
        ),
        (
            "O11",
            "2. **Determine the version.** Classify the change set using semver: PATCH (formatting, fixes, refactors invisible to users) or MINOR (structure, navigation, schema changes visible to users). Bump from the latest tag accordingly.",
            "2. **Determine the version.**\n   - Classify the change set using semver.\n   - Use PATCH for formatting, fixes, or user-invisible refactors.\n   - Use MINOR for user-visible structure, navigation, or schema changes.\n   - Bump from the latest tag.",
            "doc",
        ),
        (
            "O12",
            "3. **Derive the release message.** Summarize the change set in ≤ 80 characters. Lead with the AC reference if any (e.g., `AC1: adopt govna v0.1.0 DOC overlay`).",
            "3. **Derive the release message.**\n   - Summarize the change set in no more than 80 characters.\n   - Lead with the AC reference when one exists.",
            "doc",
        ),
        (
            "O13",
            "4. **Run release prep.** Run `./build.sh prep vX.Y.Z \"derived message\"`. It inserts the CHANGELOG row, deletes completed AC files referenced by the release message, sweeps their `plan.md` IE entries, and prints the release command. Use `--dry-run`/`-n` to inspect without writes.",
            "4. **Run release prep.**\n   - Run `./build.sh prep vX.Y.Z \"derived message\"`.\n   - Use `--dry-run` or `-n` to inspect without writes.",
            "doc",
        ),
    ] {
        let documents = if target == "code" {
            vec![
                read(&repo.join("govna/build-release.md")),
                read(&bundles[0].1.join("govna/build-release.md")),
            ]
        } else {
            vec![read(&bundles[1].1.join("govna/release.md"))]
        };
        for document in documents {
            let replacements: Vec<&str> = replacement_block.lines().collect();
            assert!(!document.contains(original), "{id}: original remains");
            assert!(
                replacements
                    .iter()
                    .all(|replacement| document.lines().any(|line| line == *replacement)),
                "{id}: replacement missing"
            );
            let mut mutant = document.replacen(replacements[0], original, 1);
            for replacement in &replacements[1..] {
                mutant = mutant.replacen(replacement, "", 1);
            }
            assert!(mutant.contains(original), "{id}: mutation failed");
        }
    }

    let code_agents = read(&bundles[0].1.join("AGENTS.md"));
    for sentinel in [
        "Leave every `git commit` for the user to execute",
        "Pause after Audit and await explicit Director instruction to Refine.",
        "Edit only the files listed in the AC's `## In Scope` section",
        "Run `./build.sh` as the first validation command in every validation cycle.",
        "Start `Package` only after an explicit Director request",
    ] {
        assert!(
            code_agents.contains(sentinel),
            "missing sentinel: {sentinel}"
        );
    }
    let code_roles = read(&bundles[0].1.join("govna/roles.md"));
    assert!(code_roles.contains("Report `Verified`, `Red-teamed`, and `Not checked`"));
    let audit = read(&bundles[0].1.join("govna/audit.md"));
    for sentinel in [
        "`clear-sync`",
        "`migration-required`",
        "`target-has-no-canon`",
        "Pass `--json`",
    ] {
        assert!(
            audit.contains(sentinel),
            "missing audit sentinel: {sentinel}"
        );
    }
    let doc_agents = read(&bundles[1].1.join("AGENTS.md"));
    assert!(!doc_agents.contains("Use `./build.sh` for repository-wide formatting validation"));
}

#[test]
fn rendered_contract_defines_bounded_completeness_scenarios() {
    let repo = Path::new(env!("CARGO_MANIFEST_DIR"));
    let code = new_fixture();
    let doc = new_fixture();
    for (target, flavor) in [(&code, "code"), (&doc, "doc")] {
        let output = govna()
            .args(["render", "--flavor", flavor, target.to_str().unwrap()])
            .output()
            .unwrap();
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
    }

    let contracts = [
        read(&repo.join("AGENTS.md")),
        read(&code.join("AGENTS.md")),
        read(&doc.join("AGENTS.md")),
    ];
    let scenarios = [
        (
            "qualifying gap",
            "- Treat the original Implement authorization as continuing authority for an eligible correction round.",
        ),
        (
            "repository evidence",
            "- Require repository evidence of the missed path or instruction.",
        ),
        (
            "active requirement",
            "- Cite the active requirement that the gap violates.",
        ),
        (
            "single outcome",
            "- Explain why the correction has only one materially valid outcome.",
        ),
        (
            "initial phase rejection",
            "- Prevent the exception from authorizing initial Implement, Audit, Ratify, Package, release preparation, publication, delegation, or commits.",
        ),
        (
            "failed eligibility",
            "- Pause immediately when an eligibility condition fails.",
        ),
        (
            "director decision",
            "- Pause immediately when a Director-owned decision appears.",
        ),
        (
            "counter reset",
            "- Reset the round counter only when the Director authorizes Implement again.",
        ),
        (
            "fourth round",
            "- Pause for the Director before a fourth correction round.",
        ),
    ];

    for contract in contracts {
        for (scenario, required_line) in scenarios {
            assert!(
                contract.lines().any(|line| line == required_line),
                "missing bounded-completeness scenario {scenario}: {required_line}"
            );
        }
    }
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

fn audit_json_output(dir: &Path) -> (serde_json::Value, std::process::Output) {
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
    (report, out)
}

fn audit_json(dir: &Path) -> serde_json::Value {
    let (report, _) = audit_json_output(dir);
    let stub = read(&dir.join(report["emitted"]["ac_stub"].as_str().unwrap()));
    assert_at_axes(&stub);
    report
}

fn audit_stub_names(dir: &Path) -> Vec<String> {
    let mut names: Vec<_> = fs::read_dir(dir.join("govna"))
        .unwrap()
        .map(|entry| entry.unwrap().file_name().to_string_lossy().into_owned())
        .filter(|name| name.starts_with("ac") && name.contains("-audit-v"))
        .collect();
    names.sort();
    names
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

#[test]
fn audit_rejects_unreadable_agents_path_before_emission() {
    let dir = new_fixture();
    fs::create_dir(dir.join("AGENTS.md")).unwrap();
    fs::create_dir_all(dir.join("govna")).unwrap();
    fs::write(dir.join("govna/ac-template.md"), "template\n").unwrap();
    let out = govna().arg("audit").current_dir(&dir).output().unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("AGENTS.md not found"));
    assert!(audit_stub_names(&dir).is_empty());
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

// Fresh, unmodified CODE and DOC fixtures are clean and do not create no-op ACs.
#[test]
fn audit_fresh_fixture_all_match() {
    for dir in [rendered_code_fixture(), rendered_doc_fixture()] {
        let out = govna().arg("audit").current_dir(&dir).output().unwrap();
        assert!(
            out.status.success(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        let stdout = String::from_utf8_lossy(&out.stdout);
        assert!(stdout.starts_with("clean ("), "{stdout}");
        assert!(stdout.ends_with("); no AC emitted\n"), "{stdout}");
        assert!(
            out.stderr.is_empty(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        assert!(audit_stub_names(&dir).is_empty());

        let (report, json_out) = audit_json_output(&dir);
        for file in report["files"].as_array().unwrap() {
            assert_eq!(file["classification"], "match", "{file}");
        }
        assert!(report["emitted"].is_null(), "{report}");
        assert!(json_out.stderr.is_empty());
        assert!(audit_stub_names(&dir).is_empty());
    }
}

#[test]
fn audit_expected_divergence_only_does_not_emit() {
    let dir = rendered_code_fixture();
    fs::write(dir.join("plan.md"), "# Local plan\n").unwrap();
    git(&dir, &["add", "plan.md"]);
    git(&dir, &["commit", "-q", "-m", "customize plan"]);

    let (report, out) = audit_json_output(&dir);
    assert_eq!(
        file_result(&report, "plan.md").unwrap()["classification"],
        "expected-divergence"
    );
    assert!(report["emitted"].is_null());
    assert!(out.stderr.is_empty());
    assert!(audit_stub_names(&dir).is_empty());
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

fn rewrite_baseline_version(dir: &Path, version: &str) {
    let path = dir.join("govna/canon-baseline.txt");
    let baseline = read(&path);
    let current = baseline
        .lines()
        .find(|line| line.starts_with("canon_version = "))
        .expect("baseline canon_version missing");
    fs::write(
        path,
        baseline.replacen(current, &format!("canon_version = {version}"), 1),
    )
    .unwrap();
}

fn replace_baseline_entry(dir: &Path, relpath: &str, scope: &str, hash: &str) {
    let path = dir.join("govna/canon-baseline.txt");
    let baseline = read(&path);
    let replacement = baseline
        .lines()
        .map(|line| {
            if line.starts_with(&format!("{relpath}\t")) {
                format!("{relpath}\t{scope}\t{hash}")
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
fn audit_accepts_only_eligible_legacy_build_release_full_scope() {
    for with_marker in [false, true] {
        let dir = rendered_code_fixture();
        let legacy = "# Legacy Swift Build and Release\n\nUse SkitVersion.current.\n";
        fs::write(dir.join("govna/build-release.md"), legacy).unwrap();
        rewrite_baseline_version(&dir, "v0.10.0");
        replace_baseline_entry(&dir, "govna/build-release.md", "full", &sha256(legacy));
        if with_marker {
            let changelog = read(&dir.join("CHANGELOG.md"));
            fs::write(
                dir.join("CHANGELOG.md"),
                changelog.replacen(
                    "| Unreleased | |",
                    "| Unreleased | preserve govna/build-release.md |",
                    1,
                ),
            )
            .unwrap();
        }
        let baseline_before = read(&dir.join("govna/canon-baseline.txt"));
        let report = audit_json(&dir);
        let result = file_result(&report, "govna/build-release.md").unwrap();
        assert_eq!(result["classification"], "ambiguity", "{result}");
        assert!(
            result["compare_command"]
                .as_str()
                .unwrap()
                .contains("full file")
        );
        if with_marker {
            assert_eq!(
                result["legacy_preserve_markers"][0],
                "preserve govna/build-release.md"
            );
        }
        assert_eq!(
            file_result(&report, "govna/canon-baseline.txt").unwrap()["classification"],
            "migration-required"
        );
        assert_eq!(
            read(&dir.join("govna/canon-baseline.txt")),
            baseline_before,
            "audit must not rewrite the accepted legacy baseline"
        );
        let stub = read(&dir.join(report["emitted"]["ac_stub"].as_str().unwrap()));
        assert!(stub.contains("verified as the final audit-adoption step"));
    }

    let current_code = rendered_code_fixture();
    let current_content = read(&current_code.join("govna/build-release.md"));
    replace_baseline_entry(
        &current_code,
        "govna/build-release.md",
        "full",
        &sha256(&current_content),
    );
    assert_audit_fails_scope_validation(&current_code);

    let doc = rendered_doc_fixture();
    rewrite_baseline_version(&doc, "v0.10.0");
    add_baseline_entry(&doc, "govna/build-release.md", "# legacy\n");
    assert_audit_fails_scope_validation(&doc);

    let other_path = rendered_code_fixture();
    rewrite_baseline_version(&other_path, "v0.10.0");
    let agents = read(&other_path.join("AGENTS.md"));
    replace_baseline_entry(&other_path, "AGENTS.md", "full", &sha256(&agents));
    assert_audit_fails_scope_validation(&other_path);

    let wrong_boundary = rendered_code_fixture();
    rewrite_baseline_version(&wrong_boundary, "v0.10.0");
    let build_release = read(&wrong_boundary.join("govna/build-release.md"));
    replace_baseline_entry(
        &wrong_boundary,
        "govna/build-release.md",
        "before:## Wrong Boundary",
        &sha256(&build_release),
    );
    assert_audit_fails_scope_validation(&wrong_boundary);

    let bad_hash = rendered_code_fixture();
    rewrite_baseline_version(&bad_hash, "v0.10.0");
    replace_baseline_entry(&bad_hash, "govna/build-release.md", "full", "bad");
    let out = govna()
        .arg("audit")
        .current_dir(&bad_hash)
        .output()
        .unwrap();
    assert!(!out.status.success());
    assert!(String::from_utf8_lossy(&out.stderr).contains("invalid SHA-256"));
    assert!(audit_stub_names(&bad_hash).is_empty());
}

fn assert_audit_fails_scope_validation(dir: &Path) {
    let out = govna().arg("audit").current_dir(dir).output().unwrap();
    assert!(!out.status.success());
    assert!(
        String::from_utf8_lossy(&out.stderr).contains("scope"),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(audit_stub_names(dir).is_empty());
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
    assert!(stub.contains("### Validation disposition"));
    assert!(stub.contains("`./build.sh` — inferred from target `AGENTS.md`"));
    assert!(stub.contains("No Director confirmation is required"));
    assert!(!stub.contains("Director must confirm it or override it in chat"));
    assert!(!stub.contains("[Manual]"), "{stub}");
    assert!(stub.contains("1 migration item and 0 entries"), "{stub}");
    assert!(!stub.contains("1 migration items"), "{stub}");
    assert!(!stub.contains("1 entries"), "{stub}");
    assert!(stub.contains("except `govna/canon-baseline.txt`"));
    assert!(stub.contains("every other applicable automated AT"));
    assert!(stub.contains("verified as the final audit-adoption step"));
    assert!(!stub.contains("leave this emitted stub unchanged"));
    assert!(
        stub.find("inferred `./build.sh` validation disposition")
            .unwrap()
            < stub
                .find("verified as the final audit-adoption step")
                .unwrap()
    );

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
    let stub = read(&missing_entry.join(report["emitted"]["ac_stub"].as_str().unwrap()));
    assert!(stub.contains("1 migration item and 1 entry"), "{stub}");
    let manual = stub
        .find("Director resolved every `### Routing Decisions`")
        .unwrap();
    let conditional = stub.find("Every resolved routing outcome").unwrap();
    let validation = stub
        .find("inferred `./build.sh` validation disposition")
        .unwrap();
    let baseline = stub
        .find("verified as the final audit-adoption step")
        .unwrap();
    assert!(manual < conditional && conditional < validation && validation < baseline);
}

#[test]
fn audit_doc_baseline_migration_infers_evidenced_no_validation() {
    let dir = rendered_doc_fixture();
    fs::remove_file(dir.join("govna/canon-baseline.txt")).unwrap();

    let report = audit_json(&dir);
    let stub = read(&dir.join(report["emitted"]["ac_stub"].as_str().unwrap()));
    assert!(stub.contains("### Validation disposition"));
    assert!(stub.contains("`Not applicable` — inferred from target `govna/release.md`"));
    assert!(stub.contains("No Director confirmation is required"));
    assert!(!stub.contains("Director must confirm"));
    assert!(!stub.contains("[Manual]"), "{stub}");
    assert!(stub.contains("inferred `Not applicable` validation disposition"));
}

fn remove_agents_rule(dir: &Path, rule: &str) {
    let path = dir.join("AGENTS.md");
    let agents = read(&path);
    assert!(agents.contains(rule), "missing fixture rule: {rule}");
    fs::write(path, agents.replacen(rule, "", 1)).unwrap();
}

fn unresolved_validation_stub(dir: &Path) -> String {
    fs::remove_file(dir.join("govna/canon-baseline.txt")).unwrap();
    let report = audit_json(dir);
    let stub = read(&dir.join(report["emitted"]["ac_stub"].as_str().unwrap()));
    assert!(
        stub.contains("**Validation disposition**: proposed"),
        "{stub}"
    );
    assert!(stub.contains("Director must confirm"), "{stub}");
    assert!(stub.contains("[Manual]"), "{stub}");
    assert!(stub.contains("Every resolved routing outcome"), "{stub}");
    assert!(
        stub.find("Director resolved every `### Routing Decisions`")
            .unwrap()
            < stub.find("Every resolved routing outcome").unwrap()
    );
    stub
}

#[test]
fn audit_code_validation_evidence_failures_remain_unresolved() {
    const RUN_RULE: &str =
        "- Run `./build.sh` as the first validation command in every validation cycle.\n";
    const USE_RULE: &str = "- Use `./build.sh` for repository-wide formatting validation, testing, vetting, linting, static analysis, and compilation checks.\n";

    for mutation in 0..6 {
        let dir = rendered_code_fixture();
        match mutation {
            0 => remove_agents_rule(&dir, RUN_RULE),
            1 => {
                let path = dir.join("AGENTS.md");
                let agents = read(&path);
                fs::write(path, format!("{agents}{RUN_RULE}")).unwrap();
            }
            2 => {
                let path = dir.join("AGENTS.md");
                let agents = read(&path).replacen(USE_RULE, "- Use `make validate` for repository-wide formatting validation, testing, vetting, linting, static analysis, and compilation checks.\n", 1);
                fs::write(path, agents).unwrap();
            }
            3 => {
                let path = dir.join("AGENTS.md");
                let agents = read(&path)
                    .replace(RUN_RULE, "- Run `make validate` as the first validation command in every validation cycle.\n")
                    .replace(USE_RULE, "- Use `make validate` for repository-wide formatting validation, testing, vetting, linting, static analysis, and compilation checks.\n");
                fs::write(path, agents).unwrap();
            }
            4 => fs::remove_file(dir.join("build.sh")).unwrap(),
            5 => {
                fs::remove_file(dir.join("build.sh")).unwrap();
                fs::create_dir(dir.join("build.sh")).unwrap();
            }
            _ => unreachable!(),
        }
        let stub = unresolved_validation_stub(&dir);
        assert!(stub.contains("proposed `./build.sh` based on the CODE flavor"));
    }
}

#[test]
fn audit_doc_validation_evidence_failures_remain_unresolved() {
    const RUN_RULE: &str =
        "- Run `./build.sh` as the first validation command in every validation cycle.\n";
    for mutation in 0..4 {
        let dir = rendered_doc_fixture();
        match mutation {
            0 => fs::remove_file(dir.join("govna/release.md")).unwrap(),
            1 => {
                let path = dir.join("govna/release.md");
                let release = read(&path).replace(
                    "define no automated content-validation command",
                    "define no standard content check",
                );
                fs::write(path, release).unwrap();
            }
            2 => {
                let path = dir.join("AGENTS.md");
                fs::write(&path, format!("{}{RUN_RULE}", read(&path))).unwrap();
            }
            3 => {
                let path = dir.join("AGENTS.md");
                fs::write(&path, format!("{}{RUN_RULE}{RUN_RULE}", read(&path))).unwrap();
            }
            _ => unreachable!(),
        }
        let stub = unresolved_validation_stub(&dir);
        assert!(stub.contains("proposed `Not applicable`"));
    }
}

#[test]
fn audit_validation_ignores_unrecognized_evidence_loci() {
    let dir = rendered_code_fixture();
    const RUN_RULE: &str =
        "- Run `./build.sh` as the first validation command in every validation cycle.\n";
    remove_agents_rule(&dir, RUN_RULE);
    fs::write(dir.join("Makefile"), "validate:\n\t@true\n").unwrap();
    fs::create_dir_all(dir.join(".github/workflows")).unwrap();
    fs::write(
        dir.join(".github/workflows/test.yml"),
        "run: make validate\n",
    )
    .unwrap();
    let roles_path = dir.join("govna/roles.md");
    fs::write(&roles_path, format!("{}{RUN_RULE}", read(&roles_path))).unwrap();

    let stub = unresolved_validation_stub(&dir);
    assert!(stub.contains("Director must confirm"));
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

// A preserve-registry entry cannot pin the canon-owned freshness field when it is
// the only metadata difference.
#[test]
fn audit_stale_metadata_version_overrides_preserve_registry() {
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
    write_preserve_registry(&dir, &["govna/metadata.txt"]);
    git(&dir, &["add", "-A"]);
    git(
        &dir,
        &["commit", "-q", "-m", "mark stale metadata preserved"],
    );
    let report = audit_json(&dir);
    let fr = file_result(&report, "govna/metadata.txt").unwrap();
    assert_eq!(fr["classification"], "clear-sync");
    assert!(fr.get("preserve_entries").is_none(), "{fr}");
}

#[test]
fn audit_stale_metadata_legacy_phrase_emits_explicit_removal_decision() {
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
    let report = audit_json(&dir);
    let result = file_result(&report, "govna/metadata.txt").unwrap();
    assert_eq!(result["classification"], "clear-sync", "{result}");
    let stub = read(&dir.join(report["emitted"]["ac_stub"].as_str().unwrap()));
    assert!(
        stub.contains("**`govna/metadata.txt` legacy preserve phrase**"),
        "{stub}"
    );
    assert!(
        stub.contains("normal classification remains `clear-sync`"),
        "{stub}"
    );
    assert!(
        stub.contains("remove only the exact legacy phrase"),
        "{stub}"
    );
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
    assert!(stub.contains("`govna/preserve.txt` joins it"), "{stub}");
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
        "preserve targets remain and their exact paths occur in `govna/preserve.txt`",
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

// A preserve-registry entry suppresses sync and remains non-actionable.
#[test]
fn audit_preserve_registry_is_non_actionable() {
    let dir = rendered_code_fixture();
    fs::write(
        dir.join("govna/roles.md"),
        format!("{}\nextra line\n", read(&dir.join("govna/roles.md"))),
    )
    .unwrap();
    write_preserve_registry(&dir, &["govna/roles.md"]);
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "preserve registry"]);
    let (report, out) = audit_json_output(&dir);
    let fr = file_result(&report, "govna/roles.md").unwrap();
    assert_eq!(fr["classification"], "preserve");
    assert_eq!(fr["preserve_entries"][0], "govna/roles.md");
    assert!(report["emitted"].is_null());
    assert!(out.stderr.is_empty());
    assert!(audit_stub_names(&dir).is_empty());
}

#[test]
fn audit_accepts_empty_registry_and_suppresses_registered_missing_target() {
    let empty = rendered_code_fixture();
    write_preserve_registry(&empty, &[]);
    let (clean, _) = audit_json_output(&empty);
    assert!(clean["emitted"].is_null(), "{clean}");

    let missing = rendered_code_fixture();
    fs::remove_file(missing.join("govna/roles.md")).unwrap();
    write_preserve_registry(&missing, &["govna/roles.md"]);
    let (report, _) = audit_json_output(&missing);
    let result = file_result(&report, "govna/roles.md").unwrap();
    assert_eq!(result["classification"], "match", "{result}");
    assert_eq!(result["preserve_entries"][0], "govna/roles.md");
    assert!(report["emitted"].is_null(), "{report}");
}

#[test]
fn audit_and_rm_reject_malformed_preserve_registry_before_emission() {
    for invalid in [
        "wrong-header\n",
        "govna-preserve-v1",
        "govna-preserve-v1\n\n",
        "govna-preserve-v1\n/absolute.md\n",
        "govna-preserve-v1\ntrailing/\n",
        "govna-preserve-v1\na\\b.md\n",
        "govna-preserve-v1\na\tb.md\n",
        "govna-preserve-v1\na/./b.md\n",
        "govna-preserve-v1\na/../b.md\n",
        "govna-preserve-v1\nb.md\na.md\n",
        "govna-preserve-v1\na.md\na.md\n",
        "govna-preserve-v1\ngovna/preserve.txt\n",
    ] {
        for command in ["audit", "rm"] {
            let dir = rendered_code_fixture();
            fs::write(dir.join("govna/preserve.txt"), invalid).unwrap();
            let output = govna().arg(command).current_dir(&dir).output().unwrap();
            assert!(!output.status.success(), "{command}: {invalid:?}");
            assert!(
                String::from_utf8_lossy(&output.stderr).contains("govna/preserve.txt"),
                "{command}: {}",
                String::from_utf8_lossy(&output.stderr)
            );
            assert!(audit_stub_names(&dir).is_empty(), "{command}: {invalid:?}");
        }
    }
}

#[test]
fn audit_ignores_legacy_phrases_outside_unreleased_summary() {
    let dir = rendered_code_fixture();
    fs::write(
        dir.join("govna/ac1-old.md"),
        "# Historical decision\n\npreserve govna/roles.md\n",
    )
    .unwrap();
    let changelog = read(&dir.join("CHANGELOG.md"));
    fs::write(
        dir.join("CHANGELOG.md"),
        format!("{changelog}\n| 0.0.1 | preserve govna/roles.md |\n"),
    )
    .unwrap();
    fs::write(
        dir.join("govna/roles.md"),
        format!("{}\nlocal edit\n", read(&dir.join("govna/roles.md"))),
    )
    .unwrap();
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "historical preserve prose"]);
    let report = audit_json(&dir);
    let result = file_result(&report, "govna/roles.md").unwrap();
    assert_eq!(result["classification"], "ambiguity", "{result}");
    assert!(result.get("legacy_preserve_markers").is_none(), "{result}");
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
    let (report, _) = audit_json_output(&dir);
    let fr = file_result(&report, "govna/development-guidelines.md").unwrap();
    assert_eq!(fr["classification"], "match", "{fr}");
    assert!(report["emitted"].is_null());
}

#[test]
fn audit_boundaryless_build_release_requires_review_even_with_preserve_marker() {
    for with_marker in [false, true] {
        let dir = rendered_code_fixture();
        fs::write(
            dir.join("govna/build-release.md"),
            "# Local Build and Release\n\nUse SkitVersion.current.\n",
        )
        .unwrap();
        if with_marker {
            let changelog = read(&dir.join("CHANGELOG.md"));
            fs::write(
                dir.join("CHANGELOG.md"),
                changelog.replacen(
                    "| Unreleased | |",
                    "| Unreleased | preserve govna/build-release.md |",
                    1,
                ),
            )
            .unwrap();
        }
        let report = audit_json(&dir);
        let fr = file_result(&report, "govna/build-release.md").unwrap();
        assert_eq!(fr["classification"], "ambiguity", "{fr}");
        assert!(
            fr["compare_command"]
                .as_str()
                .unwrap()
                .contains("full file"),
            "{fr}"
        );
        if with_marker {
            assert_eq!(
                fr["legacy_preserve_markers"][0],
                "preserve govna/build-release.md"
            );
        } else {
            assert!(fr.get("legacy_preserve_markers").is_none(), "{fr}");
        }
    }
}

#[test]
fn audit_build_release_boundary_scopes_local_and_canon_changes() {
    let local_dir = rendered_code_fixture();
    let local_path = local_dir.join("govna/build-release.md");
    fs::write(
        &local_path,
        format!(
            "{}- Keep a repository-specific release marker.\n",
            read(&local_path)
        ),
    )
    .unwrap();
    let (local_report, _) = audit_json_output(&local_dir);
    let local_result = file_result(&local_report, "govna/build-release.md").unwrap();
    assert_eq!(local_result["classification"], "match", "{local_result}");
    assert_eq!(local_result["boundary"], "## Project Practices");

    let canon_dir = rendered_code_fixture();
    let canon_path = canon_dir.join("govna/build-release.md");
    let content = read(&canon_path);
    fs::write(
        &canon_path,
        content.replacen(
            "Reuse the canonical build color policy",
            "Change the canonical build color policy",
            1,
        ),
    )
    .unwrap();
    let canon_report = audit_json(&canon_dir);
    let canon_result = file_result(&canon_report, "govna/build-release.md").unwrap();
    assert_ne!(canon_result["classification"], "match", "{canon_result}");
    assert_eq!(canon_result["boundary"], "## Project Practices");
    let stub = read(
        &canon_dir.join(
            canon_report["emitted"]["ac_stub"]
                .as_str()
                .expect("audit stub missing"),
        ),
    );
    assert!(
        stub.contains("canon zone above `## Project Practices`"),
        "{stub}"
    );
}

#[test]
fn audit_clean_run_leaves_existing_edited_stub_untouched() {
    let dir = rendered_code_fixture();
    let stub_path = dir.join("govna/ac1-audit-v0.14.0.md");
    let edited = "director-owned edited audit stub\n";
    fs::write(&stub_path, edited).unwrap();

    let out = govna().arg("audit").current_dir(&dir).output().unwrap();
    assert!(
        out.status.success(),
        "{}",
        String::from_utf8_lossy(&out.stderr)
    );
    assert!(String::from_utf8_lossy(&out.stdout).contains("no AC emitted"));
    assert_eq!(read(&stub_path), edited);
    assert_eq!(audit_stub_names(&dir), ["ac1-audit-v0.14.0.md"]);
}

#[test]
fn audit_clean_run_does_not_consume_next_ac_number() {
    let dir = rendered_code_fixture();
    let clean = govna().arg("audit").current_dir(&dir).output().unwrap();
    assert!(clean.status.success());
    assert!(audit_stub_names(&dir).is_empty());

    fs::remove_file(dir.join("govna/roles.md")).unwrap();
    let report = audit_json(&dir);
    assert_eq!(report["emitted"]["ac_stub"], "govna/ac1-audit-v0.25.0.md");
    assert_eq!(audit_stub_names(&dir), ["ac1-audit-v0.25.0.md"]);
}

// re-running immediately (unedited stub) reuses the same AC number;
// editing the stub's body then re-running fails with the edit-detection
// guard's exact wording.
#[test]
fn audit_idempotent_reuse_and_edit_detection_guard() {
    let dir = rendered_code_fixture();
    fs::remove_file(dir.join("govna/roles.md")).unwrap();
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
    let (report, out) = audit_json_output(&dir);
    assert!(report["header"]["canon_sha"].is_string());
    assert!(report["files"].is_array());
    assert!(!report["files"].as_array().unwrap().is_empty());
    assert!(report["emitted"].is_null());
    assert!(out.stderr.is_empty());
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

// A preserve-registry entry routes that file to Out Of Scope.
#[test]
fn rm_preserve_registry_routes_to_keep() {
    let dir = rendered_code_fixture();
    write_preserve_registry(&dir, &["govna/roles.md"]);
    git(&dir, &["add", "-A"]);
    git(&dir, &["commit", "-q", "-m", "preserve registry"]);
    let stub = rm_stub(&dir);
    assert!(
        stub.contains("- `govna/roles.md` — keep; registered in govna/preserve.txt."),
        "{stub}"
    );
    assert!(
        stub.contains("- `govna/preserve.txt` — delete control state last;"),
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

#[test]
fn apply_preserves_boundaryless_build_release_for_manual_migration() {
    let dir = new_fixture();
    govna()
        .args(["apply", "-f", "code", "-s", "swift"])
        .current_dir(&dir)
        .output()
        .unwrap();
    let legacy = "# Local Swift Build and Release\n\nUse SkitVersion.current.\n";
    fs::write(dir.join("govna/build-release.md"), legacy).unwrap();

    let out = govna()
        .args(["apply", "-f", "code", "-s", "swift"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(out.status.success());
    assert_eq!(read(&dir.join("govna/build-release.md")), legacy);
    assert!(
        String::from_utf8_lossy(&out.stderr)
            .contains("existing content preserved for manual migration")
    );
    let ac = read(&dir.join("govna/ac2-govna-apply.md"));
    assert!(ac.contains("manual boundary migration required"), "{ac}");
    assert!(
        ac.contains("Merge rendered canon above `## Project Practices`"),
        "{ac}"
    );
}

#[test]
fn apply_merges_build_release_project_practices() {
    let dir = new_fixture();
    govna()
        .args(["apply", "-f", "code", "-s", "swift"])
        .current_dir(&dir)
        .output()
        .unwrap();
    let path = dir.join("govna/build-release.md");
    let customized = format!("{}- Keep SkitVersion.current aligned.\n", read(&path));
    fs::write(&path, customized).unwrap();
    govna()
        .args(["apply", "-f", "code", "-s", "swift"])
        .current_dir(&dir)
        .output()
        .unwrap();
    assert!(read(&path).ends_with("## Project Practices\n- Keep SkitVersion.current aligned.\n"));
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
        let canon_cycle = read(&dir.join("govna/canon-cycle.md"));
        assert!(
            canon_cycle.contains("above `## Project Practices` in development/editing guidelines and CODE build-release"),
            "{}: {canon_cycle}",
            dir.display()
        );
        assert!(
            canon_cycle.contains("Keep DOC release full canon"),
            "{}: {canon_cycle}",
            dir.display()
        );
        assert!(!canon_cycle.contains("DOC `govna/release.md` use"));
        let agents = read(&dir.join("AGENTS.md"));
        for rule in [
            "Treat every Director-resolved routing target as effective implementation scope",
            "Treat each explicitly named migration destination as effective implementation scope",
            "Treat `govna/preserve.txt` as effective implementation scope",
            "Require the Director to name every migration destination",
            "Verify each resolved sync target",
            "Verify each migration source",
            "Verify each canon-backed migration destination",
            "Verify each repo-owned migration destination",
            "Verify each resolved delete target",
            "Verify each resolved preserve target",
            "Confirm or override an unresolved emitted validation disposition in chat",
            "Run the resolved validation command after all selected sync, migration, and deletion work",
            "Cite repository evidence when resolving validation as `Not applicable`",
            "Install or replace `govna/canon-baseline.txt` from the scratch render only after every other applicable acceptance test",
            "Treat standalone `Ratify` or `ratify` after successful Implement completion as the Director's acceptance action",
            "Perform the final review during the same Ratify turn",
            "Complete Ratify in that turn when the review finds no issue",
            "Apply the Approval Boundaries > General Gates and roles.md `What the Operator Must Defer` boundaries to classify any other Director-owned Ratify finding",
            "Return Ratify feedback to Refine, without completing Ratify, for a contract, scope, product, security, destructive, publication, or release finding",
            "Auto-correct an implementation-only finding inline during Ratify",
            "Rerun applicable validation after an inline Ratify correction",
            "Skip `./build.sh` in that revalidation only when the correction is documentation-only and not covered by this repo's own build validation",
            "Repeat the correct-validate-review cycle automatically for at least 3 rounds before treating an implementation-only finding as unresolved",
            "Return Ratify feedback to Implement, without completing Ratify, for an implementation-only finding still unresolved after 3 rounds",
            "Skip requests for a second acceptance signal after a clean Ratify review",
            "Apply these rules during Implement, closure-audit correction, and Ratify's implementation-only correction loop",
            "Require the omitted artifact to fail compilation, execution, rendering, regeneration, settled-behavior verification, or exact-fact accuracy without the change",
            "Require every omitted production-reference change to have no materially distinct valid outcome",
            "Update an omitted lockfile only as deterministic output from an explicitly in-scope dependency decision",
            "Return to Refine when lockfile resolution exposes an unexpected dependency, source, version, feature, or graph choice",
            "Update an omitted documentation reference only for an exact settled identifier, signature, command, path, or output",
            "Exempt only that eligible verification artifact from a second file-creation authorization",
            "Prohibit new production behavior, production files, interfaces, dependency decisions, migrations, or architectural choices",
            "Return to Refine when more than one materially distinct valid outcome exists",
            "Record each effective-scope path, triggering in-scope change, and eligibility rule in the applicable Implement or Ratify completion report",
            "Apply contract-integrity reporting when governance instructions are contradictory, circular, unexecutable, or repeatedly produce a workflow loop",
            "Define a repeated workflow loop as the same conflict forcing at least two unnecessary phase returns, correction cycles, or Director round-trips",
            "Report a directly demonstrated contradiction, circular dependency, or unexecutable instruction without waiting for repetition",
            "Require repository evidence, an observed workflow consequence, or a directly demonstrable consequence for every finding",
            "Avoid executing a broken or unsafe path solely to produce finding evidence",
            "Exclude wording preferences, harmless redundancy, speculative conflicts, and disagreement with a settled Director decision",
            "Cite each source path, section heading, short targeted instruction snippet, and operational effect",
            "Classify repository-specific tools, architecture, release, content, or operating-preference findings as `Consumer-local`",
            "Route `Consumer-local` corrections to `## Project Rules` or the owning repo document",
            "Classify shared phase, approval, scope, role, canon, or template findings as `Govna canon`",
            "Route `Govna canon` corrections to the authoritative source and every applicable consumer path",
            "Apply an unnumbered action instruction to the sole AC under `govna/`",
            "Require the AC number when multiple ACs are present",
            "Pause before any unnamed action",
            "Treat ambiguous, unrelated, or implicit replies as non-advancing feedback",
            "Classify a finding as `Unclear` when repository evidence supports both destinations",
            "Pair a blocking `Govna canon` recommendation with a temporary consumer mitigation only when the mitigation remains compatible with canon",
            "Mark every temporary consumer mitigation explicitly and state its removal condition",
            "Prohibit a temporary consumer mitigation from overriding or contradicting canon",
            "Continue unaffected authorized work when a finding is non-blocking",
            "Report a non-blocking finding in the next substantive response",
            "Recheck an acknowledged or deferred finding silently while its evidence, impact, classification, and recommended correction remain unchanged",
            "Report an acknowledged or deferred finding again only when one of those elements changes",
            "Keep an unauthorized finding in chat and the active session",
            "Record an authorized correction in the governance document that owns the topic",
            "Prevent the governance-record rule from bypassing authorization for a governance edit",
            "Prevent contract-integrity reporting from authorizing a new AC phase, governance edit, delegation, commit, publication, or release action",
            "Record every authorized repository-behavior correction in its owning governance document",
            "Recheck new or unresolved contract-integrity findings before completing Audit",
            "Recheck new or unresolved contract-integrity findings before completing Implement and during the closure audit",
            "Recheck new or unresolved contract-integrity findings during Ratify",
        ] {
            assert!(agents_authority.contains(rule), "source AGENTS.md: {rule}");
            assert!(agents.contains(rule), "{}: {rule}", dir.display());
        }
        for rule in [
            "Classify repository-specific tools, architecture, release, content, or operating-preference findings as `Consumer-local`.",
            "Route `Consumer-local` corrections to `## Project Rules` or the owning repo document.",
            "Classify shared phase, approval, scope, role, canon, or template findings as `Govna canon`.",
            "Route `Govna canon` corrections to the authoritative source and every applicable consumer path.",
            "Apply an unnumbered action instruction to the sole AC under `govna/`.",
            "Require the AC number when multiple ACs are present.",
            "Pause before any unnamed action.",
            "Treat ambiguous, unrelated, or implicit replies as non-advancing feedback.",
        ] {
            let bullet = format!("- {rule}");
            assert!(
                agents_authority.lines().any(|line| line == bullet),
                "source AGENTS.md atomic instruction: {bullet}"
            );
            assert!(
                agents.lines().any(|line| line == bullet),
                "{} atomic instruction: {bullet}",
                dir.display()
            );
        }
        for compound in [
            "Classify repository-specific tools, architecture, release, content, or operating-preference findings as `Consumer-local`; route corrections",
            "Classify shared phase, approval, scope, role, canon, or template findings as `Govna canon`; route corrections",
            "Apply an unnumbered action instruction to the sole AC under `govna/`; require the AC number",
            "Pause before any unnamed action; treat ambiguous, unrelated, or implicit replies",
        ] {
            assert!(
                !agents_authority.contains(compound),
                "source AGENTS.md compound instruction: {compound}"
            );
            assert!(
                !agents.contains(compound),
                "{} compound instruction: {compound}",
                dir.display()
            );
        }
        assert!(!agents.contains("Keep Ratify complete only after the Director accepts"));
        assert!(!agents.contains("Confirm or override the emitted validation disposition in chat"));
        assert!(read(&dir.join("govna/metadata.txt")).contains("canon_version = v0.25.0\n"));
        let audit_doc = read(&dir.join("govna/audit.md"));
        for contract in [
            "only when the completed report contains actionable work",
            "ordinary preserve results are non-actionable",
            "no AC emitted",
            "never deletes, overwrites, or validates an existing audit stub",
            "`emitted` is `null`",
        ] {
            assert!(authority.contains(contract), "authority: {contract}");
            assert!(
                audit_doc.contains(contract),
                "{}: {contract}",
                dir.display()
            );
        }
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
            "Complete Ratify when that review is clean",
            "Perform the Director-triggered final review",
            "Apply bounded correction behavior",
            "only after separate Director authorization",
            "Apply the complete phase, scope, correction, contract-integrity, and advancement rules in `AGENTS.md`",
        ] {
            assert!(workflow.contains(rule), "{rule}: {workflow}");
        }
    }
    for rule in [
        "Skip requests for a second acceptance signal after a clean Ratify review",
        "Return Ratify feedback to Refine, without completing Ratify",
        "Apply `### Effective Implementation Scope` to eligible omitted artifacts during Ratify correction",
        "Recheck new or unresolved contract-integrity findings during Ratify",
        "Start `Package` only after an explicit Director request",
    ] {
        assert!(
            agents_authority.contains(rule),
            "{rule}: {agents_authority}"
        );
    }
    for roles in [
        read(&repo_root.join("govna/roles.md")),
        read(&code_dir.join("govna/roles.md")),
        read(&doc_dir.join("govna/roles.md")),
    ] {
        assert!(roles.contains(
            "Follow AC, phase, scope, correction, completion, and Package rules in `AGENTS.md`"
        ));
        assert!(roles.contains(
            "Do not treat effective implementation scope as authority to resolve a Director-owned decision"
        ));
        assert!(roles.contains(
            "Keep contract-integrity reports from authorizing governance edits or phase advancement"
        ));
    }
    let ac_template = read(&repo_root.join("govna/ac-template.md"));
    assert_at_axes(&ac_template);
    assert!(ac_template.contains("Label every AT `[Automated]` or `[Manual]` and `[Pre-release gate]` or `[Post-release verification]`"));
    assert!(ac_template.contains(
        "Apply only the effective-scope and emitted-routing exceptions defined in `AGENTS.md`"
    ));
    let authority_acceptance_tests = markdown_section(&ac_template, "Acceptance Tests");
    for dir in [&code_dir, &doc_dir] {
        let rendered_template = read(&dir.join("govna/ac-template.md"));
        assert_eq!(
            markdown_section(&rendered_template, "Acceptance Tests"),
            authority_acceptance_tests
        );
        assert_at_axes(&rendered_template);
        assert!(rendered_template.contains("Label every AT `[Automated]` or `[Manual]` and `[Pre-release gate]` or `[Post-release verification]`"));
        assert!(rendered_template.contains(
            "Apply only the effective-scope and emitted-routing exceptions defined in `AGENTS.md`"
        ));
    }
    let rationale_authority = read(&repo_root.join("govna/operator-contract-rationale.md"));
    for rationale in [
        &rationale_authority,
        &read(&code_dir.join("govna/operator-contract-rationale.md")),
        &read(&doc_dir.join("govna/operator-contract-rationale.md")),
    ] {
        for boundary in [
            "## Why Effective Implementation Scope Is Bounded",
            "directly broken, deterministic fallout",
            "preserves behavior and intent",
            "returns to Refine wherever product, scope, security",
        ] {
            assert!(rationale.contains(boundary), "{boundary}: {rationale}");
        }
        for boundary in [
            "## Why Contract Integrity Reporting Is Evidence-Triggered",
            "distinguishes contract defects from implementation defects",
            "Classification routes consumer-local, canon, or unclear findings but never grants editing authority",
            "Blocking findings stop unsafe or decision-bearing work",
            "unchanged acknowledged findings stay silent",
        ] {
            assert!(rationale.contains(boundary), "{boundary}: {rationale}");
        }
    }
    {
        let relpath = "govna/ac-template.md";
        let content = read(&repo_root.join(relpath));
        assert!(!content.contains("During a audit"), "{relpath}: {content}");
        assert!(
            !content.contains("resolves a audit"),
            "{relpath}: {content}"
        );
    }
}

#[test]
fn render_code_build_release_is_stack_aware_and_bounded() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let authority = read(&repo_root.join("govna/build-release.md"));
    let doc_dir = new_fixture();
    let authority_zone = authority
        .split_once("## Project Practices\n")
        .expect("source build-release boundary missing")
        .0;
    for stack in ["go", "rust", "swift", "terraform", "node", "python", "java"] {
        let code_dir = new_fixture();
        let mut args = vec!["render", "--flavor", "code", "--stack", stack];
        if stack == "go" {
            args.extend(["--module-path", "example.com/build-release-test"]);
        }
        args.push(code_dir.to_str().unwrap());
        let output = govna().args(args).output().unwrap();
        assert!(
            output.status.success(),
            "{stack}: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        let rendered = read(&code_dir.join("govna/build-release.md"));
        let (canon_zone, tail) = rendered
            .split_once("## Project Practices\n")
            .expect("rendered build-release boundary missing");
        assert_eq!(rendered.matches("## Project Practices").count(), 1);
        assert!(tail.trim().is_empty(), "{stack}: {tail}");
        if stack == "rust" {
            assert_eq!(canon_zone, authority_zone);
            assert!(canon_zone.contains("## Rust Compilation Reuse"));
        } else {
            for excluded in [
                "Rust Compilation Reuse",
                "Cargo",
                "Clippy",
                "PROGRAM_VERSION",
            ] {
                assert!(
                    !canon_zone.contains(excluded),
                    "{stack}: {excluded}: {canon_zone}"
                );
            }
        }
        let baseline = read(&code_dir.join("govna/canon-baseline.txt"));
        assert!(baseline.contains("govna/build-release.md\tbefore:## Project Practices\t"));
        assert!(read(&code_dir.join("govna/metadata.txt")).contains("canon_version = v0.25.0\n"));
    }
    assert!(
        govna()
            .args(["render", "--flavor", "doc", doc_dir.to_str().unwrap()])
            .output()
            .unwrap()
            .status
            .success()
    );

    for expected in [
        "build duration becomes materially costly",
        "stable Cargo or Clippy behavior offers measurable artifact reuse",
        "compiler-cache evaluation only with Director authorization",
        "toolchain version, exact commands, isolated target-directory conditions, repeated timings, and unchanged validation coverage",
    ] {
        assert!(authority_zone.contains(expected), "{expected}");
    }
    assert!(!read(&doc_dir.join("govna/release.md")).contains("## Rust Compilation Reuse"));
    assert!(!doc_dir.join("govna/build-release.md").exists());
    assert!(read(&doc_dir.join("govna/metadata.txt")).contains("canon_version = v0.25.0\n"));
}

#[test]
fn rendered_rust_prep_validation_token_contract_matches_source() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let rendered_dir = new_fixture();
    let output = govna()
        .args([
            "render",
            "--flavor",
            "code",
            "--stack",
            "rust",
            rendered_dir.to_str().unwrap(),
        ])
        .output()
        .unwrap();
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );

    let source_build = read(&repo_root.join("build.sh"));
    let rendered_build = read(&rendered_dir.join("build.sh"));
    let source_tests = read(&repo_root.join("tests/build_cli.sh"));
    let rendered_tests = read(&rendered_dir.join("tests/build_cli.sh"));
    let rendered_release = read(&rendered_dir.join("govna/build-release.md"));
    let rendered_guidelines = read(&rendered_dir.join("govna/development-guidelines.md"));
    for expected in [
        "-t, --validation-token <TOKEN>",
        "selected_token=\"$cli_token\"",
        "selected_token=\"${GOVNA_PREP_VALIDATION_TOKEN:-}\"",
        "validation token option may be used only once",
    ] {
        assert!(source_build.contains(expected), "source: {expected}");
        assert!(rendered_build.contains(expected), "rendered: {expected}");
    }
    for expected in [
        "test_prep_evidence_routing",
        "test_prep_validation_token_cli",
        "assert_not_contains \"$output\" \"$token\"",
    ] {
        assert!(source_tests.contains(expected), "source tests: {expected}");
        assert!(
            rendered_tests.contains(expected),
            "rendered tests: {expected}"
        );
    }
    assert!(rendered_release.contains("./build.sh prep -t '<token>'"));
    assert!(rendered_release.contains("compatibility fallback"));
    assert!(rendered_guidelines.contains("through `-t, --validation-token`"));
    assert!(rendered_guidelines.contains("only as a compatibility fallback"));

    let shell_tests = Command::new("bash")
        .arg("tests/build_cli.sh")
        .current_dir(&rendered_dir)
        .output()
        .unwrap();
    assert!(
        shell_tests.status.success(),
        "stdout: {}\nstderr: {}",
        String::from_utf8_lossy(&shell_tests.stdout),
        String::from_utf8_lossy(&shell_tests.stderr)
    );
}

#[test]
fn rendered_agents_scope_rust_validation_token_contract() {
    let conditional_rules = [
        "Pass the full build's validation token to Rust prep during `Package` only when the repository provides Rust validation-token support.",
        "Fall back to a pre-change full build only when Rust validation-token support exists and its prep evidence is missing or stale.",
        "Refresh Rust validation evidence from the same scratch baseline only when the repository provides Rust validation-token support and the installed `govna/canon-baseline.txt` is verified.",
        "Use the refreshed Rust validation token as Package evidence only when the repository provides Rust validation-token support.",
    ];
    let unconditional_rules = [
        "Pass the full build's validation token to Rust prep during `Package`.",
        "Fall back to a pre-change full build when Rust prep evidence is missing or stale.",
        "Refresh Rust validation evidence from the same scratch baseline only after installing and verifying `govna/canon-baseline.txt`.",
        "Use the refreshed Rust validation token as Package evidence.",
    ];

    for stack in ["go", "rust", "swift", "terraform", "node", "python", "java"] {
        let rendered_dir = new_fixture();
        let mut args = vec!["render", "--flavor", "code", "--stack", stack];
        if stack == "go" {
            args.extend(["--module-path", "example.com/validation-token-scope"]);
        }
        args.push(rendered_dir.to_str().unwrap());
        let output = govna().args(args).output().unwrap();
        assert!(
            output.status.success(),
            "{stack}: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        let agents = read(&rendered_dir.join("AGENTS.md"));
        for rule in conditional_rules {
            assert!(agents.contains(rule), "{stack}: {rule}: {agents}");
        }
        for rule in unconditional_rules {
            assert!(
                !agents.lines().any(|line| line == format!("- {rule}")),
                "{stack}: {rule}: {agents}"
            );
        }
    }

    let doc_dir = rendered_doc_fixture();
    let doc_agents = read(&doc_dir.join("AGENTS.md"));
    for excluded in [
        "current pre-change Package evidence",
        "Rust prep",
        "Rust validation evidence",
        "Rust validation token",
    ] {
        assert!(!doc_agents.contains(excluded), "{excluded}: {doc_agents}");
    }
}

#[test]
fn rendered_refine_no_change_contract_matches_authority() {
    let repo_root = Path::new(env!("CARGO_MANIFEST_DIR"));
    let authority_agents = read(&repo_root.join("AGENTS.md"));
    let authority_cycle = read(&repo_root.join("govna/development-cycle.md"));
    let code_dir = rendered_code_fixture();
    let doc_dir = rendered_doc_fixture();
    let code_agents = read(&code_dir.join("AGENTS.md"));
    let doc_agents = read(&doc_dir.join("AGENTS.md"));
    let code_cycle = read(&code_dir.join("govna/development-cycle.md"));
    let conditional_edit = "Edit the AC during Refine when an Audit finding or settled Director decision requires an AC change.";
    let no_change = "Complete Refine without editing the AC when no Audit finding or settled Director decision requires an AC change and no Director-specific decision remains unresolved.";
    let no_implement = "Do not begin implementation during Refine.";
    let immutable_stub =
        "Apply each resolved routing action while leaving the emitted AC stub unchanged.";

    for agents in [&authority_agents, &code_agents, &doc_agents] {
        for rule in [conditional_edit, no_change, no_implement, immutable_stub] {
            assert!(agents.contains(&format!("- {rule}")), "{rule}: {agents}");
        }
        assert!(!agents.contains("Edit the AC during Refine;"), "{agents}");
    }
    for cycle in [&authority_cycle, &code_cycle] {
        assert!(
            cycle.contains("3. **Refine.** Resolve findings and Director decisions in the AC."),
            "{cycle}"
        );
        assert!(
            cycle.contains("Apply the complete phase, scope, correction, contract-integrity, and advancement rules in `AGENTS.md`"),
            "{cycle}"
        );
    }
    assert!(!doc_dir.join("govna/development-cycle.md").exists());
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
