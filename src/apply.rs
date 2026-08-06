//! Implements the `govna apply` subcommand. Ported from governa's
//! `internal/governance` package (`runApply`, `AssessTarget`, `planCanonical`,
//! `applyOperations`, `renderApplyAC`, `maybeInitGit`).
//!
//! Runs against the current working directory (no positional arguments,
//! no `--target` flag — govna-consistent with `drift-scan`'s cwd-only
//! design, unlike governa's `-t/--target`). Writes the full canon set via
//! `governance::render_canonical_files` (already built by `render-canon`,
//! AC4), a `CLAUDE.md` symlink, and a `govna/ac<N>-govna-apply.md` adoption
//! record. No interactive prompting (govna's primary consumer is an AI
//! agent invoking the CLI non-interactively, not a terminal user) — an
//! unresolvable required param hard-fails with actionable guidance instead,
//! matching `render-canon`'s existing convention.

use crate::emission;
use crate::governance::{self, RepoType};
use crate::{SymlinkOutcome, write_canon_file, write_claude_symlink};
use std::path::{Path, PathBuf};
use std::process::{Command, ExitCode};

// ── config / CLI ────────────────────────────────────────────────────────────

pub struct Config {
    pub flavor: String,
    pub stack: String,
    pub repo_name: String,
    pub module_path: String,
    pub init_git: bool,
}

fn print_usage() {
    eprintln!("Usage: govna apply [flags]");
    eprintln!();
    eprintln!("Apply governance template to the current directory (new or existing repo).");
    eprintln!("Detects repo state, resolves missing parameters, and writes an adoption AC.");
    eprintln!();
    eprintln!("Flags:");
    eprintln!("  -f, --flavor code|doc      overlay flavor (default: auto-detect)");
    eprintln!("  -s, --stack <name>         CODE stack (default: inferred from manifests)");
    eprintln!("  -n, --repo-name <name>     repo name (default: basename of cwd)");
    eprintln!(
        "  -m, --module-path <path>   module path for Go CODE canon (default: read from go.mod)"
    );
    eprintln!("  -g, --init-git             initialize git if the target is not a repo");
    eprintln!("  -h, --help                 show this help");
}

pub fn parse_args(args: &[String]) -> Result<(Config, bool), String> {
    let mut cfg = Config {
        flavor: String::new(),
        stack: String::new(),
        repo_name: String::new(),
        module_path: String::new(),
        init_git: false,
    };

    let mut positional: Vec<String> = Vec::new();
    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "-h" | "--help" | "-?" => {
                print_usage();
                return Ok((cfg, true));
            }
            "-f" | "--flavor" => {
                let Some(v) = args.get(i + 1) else {
                    return Err("apply: -f, --flavor <code|doc> requires a value".to_string());
                };
                cfg.flavor = v.clone();
                i += 1;
            }
            "-s" | "--stack" => {
                let Some(v) = args.get(i + 1) else {
                    return Err("apply: -s, --stack <name> requires a value".to_string());
                };
                if v.trim().is_empty() {
                    return Err("apply: -s, --stack <name> requires a non-empty value".to_string());
                }
                cfg.stack = v.trim().to_string();
                i += 1;
            }
            "-n" | "--repo-name" => {
                let Some(v) = args.get(i + 1) else {
                    return Err("apply: -n, --repo-name <name> requires a value".to_string());
                };
                cfg.repo_name = v.clone();
                i += 1;
            }
            "-m" | "--module-path" => {
                let Some(v) = args.get(i + 1) else {
                    return Err("apply: -m, --module-path <path> requires a value".to_string());
                };
                cfg.module_path = v.clone();
                i += 1;
            }
            "-g" | "--init-git" => {
                cfg.init_git = true;
            }
            other => positional.push(other.to_string()),
        }
        i += 1;
    }

    if !positional.is_empty() {
        return Err(format!(
            "apply: no positional arguments accepted; run from the target repo root (got: {positional:?})"
        ));
    }

    if !cfg.flavor.is_empty() && cfg.flavor != "code" && cfg.flavor != "doc" {
        return Err(format!(
            "apply: --flavor must be code or doc, got {:?}",
            cfg.flavor
        ));
    }
    if cfg.flavor == "doc" && !cfg.stack.is_empty() {
        return Err(
            "apply: --stack applies only to CODE canon; remove --stack or select --flavor code"
                .to_string(),
        );
    }
    if cfg.flavor == "doc" && !cfg.module_path.is_empty() {
        return Err(
            "apply: --module-path applies only to Go CODE canon; remove --module-path or select --flavor code"
                .to_string(),
        );
    }

    Ok((cfg, false))
}

pub fn run_cli(args: &[String]) -> ExitCode {
    let (cfg, help) = match parse_args(args) {
        Ok(v) => v,
        Err(e) => {
            eprintln!("{e}");
            print_usage();
            return ExitCode::from(2);
        }
    };
    if help {
        return ExitCode::SUCCESS;
    }
    run(cfg)
}

// ── orchestration ───────────────────────────────────────────────────────────

fn run(cfg: Config) -> ExitCode {
    match run_inner(&cfg) {
        Ok(code) => code,
        Err(e) => {
            eprintln!("{e}");
            ExitCode::from(1)
        }
    }
}

fn run_inner(cfg: &Config) -> Result<ExitCode, String> {
    let cwd = std::env::current_dir().map_err(|e| format!("apply: get cwd: {e}"))?;

    // Order of operations is load-bearing: refuse_govna_source must run
    // before any filesystem walk (mode detection, assessment) touches a
    // target that might be govna's own source checkout.
    emission::refuse_govna_source(&cwd, "apply")?;

    let mode = detect_apply_mode(&cwd);
    if mode == "existing" {
        eprintln!("existing governance files detected; apply will overwrite them");
    }

    let assessment = assess_target(&cwd)?;
    print_assessment(&cwd, &assessment);

    let flavor_input = cfg.flavor.trim();
    let repo_type = if !flavor_input.is_empty() {
        if flavor_input == "code" {
            RepoType::Code
        } else {
            RepoType::Doc
        }
    } else {
        governance::detect_flavor(&cwd)
            .map_err(|e| format!("apply: infer flavor from cwd: {e} (use --flavor to override)"))?
    };

    let mut stack = cfg.stack.clone();
    let mut module_path = cfg.module_path.clone();
    if repo_type == RepoType::Code {
        if stack.is_empty() {
            stack = governance::infer_stack(&cwd)
                .unwrap_or_default()
                .to_string();
            if stack.is_empty() {
                return Err(format!(
                    "apply: could not infer CODE stack from cwd={}; pass --stack to override",
                    cwd.display()
                ));
            }
        }
        if stack.eq_ignore_ascii_case("go") || stack.eq_ignore_ascii_case("golang") {
            if module_path.is_empty() {
                module_path = governance::read_module_path(&cwd).unwrap_or_default();
            }
        } else if !module_path.is_empty() {
            return Err(
                "apply: --module-path applies only to Go CODE canon; remove --module-path or select --stack Go"
                    .to_string(),
            );
        }
    }

    let repo_name = if cfg.repo_name.is_empty() {
        governance::resolve_repo_name(&cwd, &module_path)
    } else {
        cfg.repo_name.clone()
    };

    let gcfg = governance::Config {
        repo_type,
        repo_name,
        stack,
        module_path,
    };

    let ops = governance::render_canonical_files(&gcfg).map_err(|e| format!("apply: {e}"))?;

    for op in &ops {
        let dest = cwd.join(&op.rel_path);
        write_canon_file(&dest, &op.content)
            .map_err(|e| format!("apply: write {}: {e}", dest.display()))?;
        println!("write {} (canon file)", op.rel_path);
    }

    match write_claude_symlink(&cwd, true)? {
        SymlinkOutcome::Created => println!("symlink CLAUDE.md -> AGENTS.md"),
        SymlinkOutcome::Skipped => eprintln!(
            "warning: CLAUDE.md exists as a regular file; expected symlink to AGENTS.md — delete the file and re-run to create the symlink"
        ),
    }

    let ac_num = emission::next_ac_number(&cwd)?;
    let stub_rel = format!("govna/ac{ac_num}-govna-apply.md");
    let stub_path = cwd.join(&stub_rel);
    let stub_body = render_apply_ac(ac_num, &gcfg, &ops);
    emission::ensure_docs_dir(&cwd, "apply")?;
    std::fs::write(&stub_path, stub_body).map_err(|e| format!("apply: write {stub_rel}: {e}"))?;
    println!("write {stub_rel} (adoption record)");

    if cfg.init_git {
        maybe_init_git(&cwd)?;
    }

    Ok(ExitCode::SUCCESS)
}

fn detect_apply_mode(target: &Path) -> &'static str {
    if target.join("AGENTS.md").is_file() || target.join("CLAUDE.md").exists() {
        "existing"
    } else {
        "new"
    }
}

fn maybe_init_git(target: &Path) -> Result<(), String> {
    if target.join(".git").exists() {
        println!("skip git init (git repo already present)");
        return Ok(());
    }
    println!("exec git init {}", target.display());
    let output = Command::new("git")
        .args(["init", &target.to_string_lossy()])
        .output()
        .map_err(|e| format!("apply: git init {}: {e}", target.display()))?;
    if !output.status.success() {
        return Err(format!(
            "apply: git init {}: {}",
            target.display(),
            String::from_utf8_lossy(&output.stderr).trim()
        ));
    }
    Ok(())
}

// ── assessment ───────────────────────────────────────────────────────────────

struct Assessment {
    repo_shape: &'static str,
    code_signals: u32,
    doc_signals: u32,
    existing_artifacts: Vec<String>,
    overwrite_risk: &'static str,
}

fn expected_artifact_paths(repo_type: Option<RepoType>) -> Vec<&'static str> {
    let mut paths = vec!["AGENTS.md", "CLAUDE.md"];
    match repo_type {
        Some(RepoType::Code) => paths.extend([
            "README.md",
            "arch.md",
            "plan.md",
            "CHANGELOG.md",
            "govna/README.md",
            "govna/development-cycle.md",
            "govna/ac-template.md",
            "govna/build-release.md",
            "govna/metadata.txt",
        ]),
        Some(RepoType::Doc) => paths.extend(["plan.md", "govna/metadata.txt"]),
        None => {}
    }
    paths
}

/// Ported from governa's `AssessTarget`: scans the target for repo-shape
/// signals (informational + overwrite-risk display only — actual flavor
/// resolution uses `governance::detect_flavor` for consistency with
/// `render-canon`/`drift-scan`, not this heuristic).
fn assess_target(root: &Path) -> Result<Assessment, String> {
    let mut files: Vec<PathBuf> = Vec::new();
    collect_files(root, root, &mut files)?;

    if files.is_empty() {
        return Ok(Assessment {
            repo_shape: "empty",
            code_signals: 0,
            doc_signals: 0,
            existing_artifacts: Vec::new(),
            overwrite_risk: "low",
        });
    }

    let mut code_signals = 0u32;
    let mut doc_signals = 0u32;
    let mut has_source_file = false;
    let mut has_code_manifest = false;
    let mut has_code_layout = false;
    let mut has_doc_planning_marker = false;

    for rel in &files {
        let rel_str = rel.to_string_lossy();
        let base = rel
            .file_name()
            .map(|n| n.to_string_lossy().to_string())
            .unwrap_or_default();
        let ext = rel
            .extension()
            .map(|e| e.to_string_lossy().to_lowercase())
            .unwrap_or_default();
        let top_level = rel_str.split('/').next().unwrap_or_default();

        if matches!(
            ext.as_str(),
            "go" | "py"
                | "js"
                | "ts"
                | "tsx"
                | "jsx"
                | "rs"
                | "java"
                | "kt"
                | "swift"
                | "c"
                | "cc"
                | "cpp"
                | "cs"
        ) {
            code_signals += 1;
            has_source_file = true;
        }
        if matches!(ext.as_str(), "md" | "mdx") {
            doc_signals += 1;
        }
        match base.as_str() {
            "go.mod" | "package.json" | "pyproject.toml" | "Cargo.toml" | "pom.xml"
            | "build.gradle" | "Makefile" | "Dockerfile" => {
                code_signals += 3;
                has_code_manifest = true;
            }
            "mkdocs.yml" | "mkdocs.yaml" => {
                doc_signals += 3;
                has_doc_planning_marker = true;
            }
            "README.md" | "AGENTS.md" | "CLAUDE.md" | "arch.md" | "plan.md" => {
                doc_signals += 1;
            }
            _ => {}
        }
        if matches!(top_level, "cmd" | "internal" | "pkg" | "src") {
            has_code_layout = true;
        }
    }

    let repo_shape = if (has_code_manifest || has_code_layout) && has_source_file {
        "likely CODE"
    } else if has_doc_planning_marker && !has_source_file && !has_code_manifest {
        "likely DOC"
    } else if code_signals > doc_signals && code_signals > 0 {
        "likely CODE"
    } else if doc_signals > code_signals && doc_signals > 0 {
        "likely DOC"
    } else if code_signals > 0 && doc_signals > 0 {
        "mixed"
    } else {
        "unclear"
    };

    let resolved_type = match repo_shape {
        "likely CODE" => Some(RepoType::Code),
        "likely DOC" => Some(RepoType::Doc),
        _ => None,
    };

    let expected = expected_artifact_paths(resolved_type);
    let mut existing = Vec::new();
    let mut overwrite_count = 0u32;
    for rel in &expected {
        let full = root.join(rel);
        if let Ok(meta) = std::fs::metadata(&full) {
            existing.push(rel.to_string());
            if meta.is_file() && meta.len() > 0 {
                overwrite_count += 1;
            }
        }
    }

    let overwrite_risk = if overwrite_count >= 3 {
        "high"
    } else if overwrite_count > 0 {
        "medium"
    } else {
        "low"
    };

    Ok(Assessment {
        repo_shape,
        code_signals,
        doc_signals,
        existing_artifacts: existing,
        overwrite_risk,
    })
}

fn collect_files(root: &Path, dir: &Path, out: &mut Vec<PathBuf>) -> Result<(), String> {
    let entries = std::fs::read_dir(dir)
        .map_err(|e| format!("apply: scan target repo {}: {e}", dir.display()))?;
    for entry in entries {
        let entry = entry.map_err(|e| format!("apply: scan target repo: {e}"))?;
        let path = entry.path();
        if path.file_name().map(|n| n == ".git").unwrap_or(false) {
            continue;
        }
        if path.is_dir() {
            collect_files(root, &path, out)?;
        } else {
            let rel = path
                .strip_prefix(root)
                .map_err(|e| format!("apply: relativize {}: {e}", path.display()))?;
            out.push(rel.to_path_buf());
        }
    }
    Ok(())
}

fn print_assessment(target: &Path, a: &Assessment) {
    println!("mode: apply");
    println!("target: {}", target.display());
    println!("repo-shape: {}", a.repo_shape);
    println!("signals: code={} doc={}", a.code_signals, a.doc_signals);
    let existing = if a.existing_artifacts.is_empty() {
        "none".to_string()
    } else {
        a.existing_artifacts.join(", ")
    };
    println!("existing-artifacts: {existing}");
    println!("overwrite-risk: {}", a.overwrite_risk);
}

// ── adoption AC ──────────────────────────────────────────────────────────────

fn render_apply_ac(ac_num: u32, cfg: &governance::Config, ops: &[governance::WriteOp]) -> String {
    let flavor = match cfg.repo_type {
        RepoType::Code => "CODE",
        RepoType::Doc => "DOC",
    };
    let mut b = String::new();
    b.push_str(&format!("# AC{ac_num} Govna Apply\n\n"));
    b.push_str(&format!(
        "Applied govna v{} governance template ({flavor} overlay) to {}.\n\n",
        crate::templates::TEMPLATE_VERSION,
        cfg.repo_name
    ));
    b.push_str("## Summary\n\n");
    b.push_str(&format!(
        "Applied govna v{} governance template ({flavor} overlay). All files below are now consumer-owned — modify freely to fit the repo's needs.\n\n",
        crate::templates::TEMPLATE_VERSION
    ));
    b.push_str("## In Scope\n\n");
    b.push_str("Files written by govna apply:\n\n");
    for op in ops {
        b.push_str(&format!("- `{}` (canon file)\n", op.rel_path));
    }
    b.push_str("- `CLAUDE.md` (agent alias link)\n");
    b.push_str("\n## Out Of Scope\n\n");
    b.push_str("- All applied files are consumer-owned and can be freely modified\n");
    b.push_str(
        "- govna is not a runtime dependency — this repo does not import or inherit from the template repo\n",
    );
    b.push_str(
        "- Future govna improvements can be adopted by having a coding agent read the govna repo and cherry-pick useful changes\n",
    );
    b.push_str("\n## Acceptance Tests\n\n");
    b.push_str("**AT1** [Manual] — Verify AGENTS.md exists and sections match repo needs.\n\n");
    b.push_str(
        "**AT2** [Manual] — Verify govna/roles.md reflects the repo's delivery model (Operator + Director).\n\n",
    );
    b.push_str("**AT3** [Manual] — Verify CLAUDE.md is a symlink to AGENTS.md.\n\n");
    b.push_str("## Status\n\n");
    b.push_str("`PENDING` — review applied governance and adapt to repo needs.\n");
    b
}
