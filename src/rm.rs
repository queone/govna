//! Implements the `govna rm` subcommand.
//!
//! Runs against the current working directory (no positional arguments,
//! no `--target` flag — matches `audit`'s cwd-only design). Renders
//! the repo's canon (to know what *should* be there), classifies every
//! canon file into In Scope (safe to delete), Out Of Scope (repo-owned,
//! keep), or Review (needs a Director routing decision), and emits one
//! file under `govna/`: the removal AC. `rm` deletes nothing itself — the
//! actual removal is a later, separate Director-approved implementation
//! pass against the emitted AC. Review items don't carry a pre-computed
//! diff — each Routing Decision bullet embeds a ready-to-run
//! comparison command instead, matching the "generate it when you need it"
//! pattern `AGENTS.md`'s Drift-Scan Adoption section already documents.

use crate::driftscan;
use crate::emission;
use crate::governance::{self, RepoType};
use crate::templates;
use std::collections::{BTreeMap, BTreeSet};
use std::path::Path;
use std::process::ExitCode;

const RM_MARKER_PREFIX: &str = "<!-- govna-rm: emitted-by govna ";

// ── config / CLI ────────────────────────────────────────────────────────────

pub struct Config {
    pub flavor: String,
    pub stack: String,
    pub repo_name: String,
}

fn print_usage() {
    eprintln!("Usage: govna rm [flags]");
    eprintln!();
    eprintln!("Emit a Director-reviewed cleanup AC for removing govna canon from an");
    eprintln!("adopted repo. Run from the consumer repo root (no positional arguments).");
    eprintln!("Deletes nothing itself.");
    eprintln!();
    eprintln!("Flags:");
    eprintln!("  -f, --flavor code|doc      overlay flavor (default: auto-detect)");
    eprintln!("  -s, --stack <name>         CODE stack (default: inferred from manifests)");
    eprintln!("  -n, --repo-name <name>     override repo name (default: basename of cwd)");
    eprintln!("  -h, --help                 show this help");
}

pub fn parse_args(args: &[String]) -> Result<(Config, bool), String> {
    let mut cfg = Config {
        flavor: String::new(),
        stack: String::new(),
        repo_name: String::new(),
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
                    return Err("rm: -f, --flavor <code|doc> requires a value".to_string());
                };
                cfg.flavor = v.clone();
                i += 1;
            }
            "-s" | "--stack" => {
                let Some(v) = args.get(i + 1) else {
                    return Err("rm: -s, --stack <name> requires a value".to_string());
                };
                if v.trim().is_empty() {
                    return Err("rm: -s, --stack <name> requires a non-empty value".to_string());
                }
                cfg.stack = v.trim().to_string();
                i += 1;
            }
            "-n" | "--repo-name" => {
                let Some(v) = args.get(i + 1) else {
                    return Err("rm: -n, --repo-name <name> requires a value".to_string());
                };
                cfg.repo_name = v.clone();
                i += 1;
            }
            other => positional.push(other.to_string()),
        }
        i += 1;
    }

    if !positional.is_empty() {
        return Err(format!(
            "rm: no positional arguments accepted; run from the consumer repo root (got: {positional:?})"
        ));
    }

    if !cfg.flavor.is_empty() && cfg.flavor != "code" && cfg.flavor != "doc" {
        return Err(format!(
            "rm: --flavor must be code or doc, got {:?}",
            cfg.flavor
        ));
    }
    if cfg.flavor == "doc" && !cfg.stack.is_empty() {
        return Err(
            "rm: --stack applies only to CODE canon; remove --stack or select --flavor code"
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
    let cwd = std::env::current_dir().map_err(|e| format!("rm: get cwd: {e}"))?;

    emission::refuse_govna_source(&cwd, "rm")?;
    emission::require_govna_adopted(&cwd, "rm")?;
    if !cwd.join(".git").exists() {
        return Err(format!(
            "rm: target {} is not a git worktree (no .git/) — rm needs git history to allocate the cleanup AC number",
            cwd.display()
        ));
    }

    let flavor_input = cfg.flavor.trim();
    let repo_type = if !flavor_input.is_empty() {
        if flavor_input == "code" {
            RepoType::Code
        } else {
            RepoType::Doc
        }
    } else {
        governance::detect_flavor(&cwd)
            .map_err(|e| format!("rm: infer flavor from cwd: {e} (use --flavor to override)"))?
    };

    let mut stack = cfg.stack.clone();
    if repo_type == RepoType::Code && stack.is_empty() {
        stack = governance::infer_stack(&cwd)
            .unwrap_or_default()
            .to_string();
        if stack.is_empty()
            && let Ok(Some(metadata)) = governance::read_repo_metadata(&cwd)
        {
            stack = metadata.get("code_stack").cloned().unwrap_or_default();
        }
        if stack.is_empty() {
            return Err(format!(
                "rm: could not infer CODE stack from cwd={}; pass --stack to override",
                cwd.display()
            ));
        }
    }

    let repo_name = if cfg.repo_name.is_empty() {
        governance::resolve_repo_name(&cwd, "")
    } else {
        cfg.repo_name.clone()
    };

    let gcfg = governance::Config {
        repo_type,
        repo_name,
        stack,
        module_path: String::new(),
    };
    let ops = governance::render_canonical_files(&gcfg).map_err(|e| format!("rm: {e}"))?;
    let canon: BTreeMap<String, String> = ops
        .into_iter()
        .map(|op| (op.rel_path, op.content))
        .collect();
    let preserve_registry =
        emission::preserve_registry(&cwd).map_err(|error| format!("rm: {error}"))?;

    let canon_version = format!("v{}", templates::CANON_VERSION);
    let (ac_num, reused) = emission::allocate_ac_number(&cwd, "govna-rm", &canon_version)?;
    let stub_rel = format!("govna/ac{ac_num}-govna-rm-{canon_version}.md");
    let stub_path = cwd.join(&stub_rel);

    if reused && stub_path.exists() {
        let unedited = emission::verify_unedited(&stub_path, RM_MARKER_PREFIX)?;
        if !unedited {
            return Err(format!(
                "rm: {stub_rel} has been edited since last emission — delete or rename the emitted file before re-running"
            ));
        }
    }

    let (in_scope, out_of_scope, review) = classify(&cwd, &canon, &preserve_registry);
    let stub_body = build_stub(
        ac_num,
        &canon_version,
        &gcfg,
        &in_scope,
        &out_of_scope,
        &review,
    );

    emission::ensure_docs_dir(&cwd, "rm")?;
    emission::write_with_marker(&stub_path, RM_MARKER_PREFIX, &canon_version, &stub_body)?;

    println!("wrote {stub_rel}");
    Ok(ExitCode::SUCCESS)
}

// ── classification ───────────────────────────────────────────────────────────

struct Routing {
    path: String,
    kind: &'static str,
    reason: String,
}

fn classify(
    target: &Path,
    canon: &BTreeMap<String, String>,
    preserve_registry: &BTreeSet<String>,
) -> (Vec<Routing>, Vec<Routing>, Vec<Routing>) {
    let mut in_scope = Vec::new();
    let mut out_of_scope = Vec::new();
    let mut review = Vec::new();

    for (relpath, canon_content) in canon {
        let target_path = target.join(relpath);
        let Ok(content) = std::fs::read_to_string(&target_path) else {
            continue;
        };

        if driftscan::EXPECTED_DIVERGENCE_PATHS.contains(&relpath.as_str()) {
            out_of_scope.push(Routing {
                path: relpath.clone(),
                kind: "keep",
                reason: "repo-owned govna-adjacent content".to_string(),
            });
            continue;
        }

        if preserve_registry.contains(relpath) {
            out_of_scope.push(Routing {
                path: relpath.clone(),
                kind: "keep",
                reason: format!("registered in {}", emission::PRESERVE_PATH),
            });
            continue;
        }

        let is_hybrid = matches!(relpath.as_str(), "README.md" | "CHANGELOG.md")
            || driftscan::mixed_content_boundary(relpath).is_some();
        if is_hybrid {
            review.push(Routing {
                path: relpath.clone(),
                kind: "hybrid",
                reason: "mixed canon-shape and consumer content".to_string(),
            });
            continue;
        }

        if content == *canon_content {
            in_scope.push(Routing {
                path: relpath.clone(),
                kind: "delete file",
                reason: "byte-equal govna canon".to_string(),
            });
            continue;
        }

        review.push(Routing {
            path: relpath.clone(),
            kind: "ambiguity",
            reason: "consumer-edited canon file".to_string(),
        });
    }

    if let Ok(meta) = std::fs::symlink_metadata(target.join("CLAUDE.md"))
        && meta.file_type().is_symlink()
    {
        in_scope.push(Routing {
            path: "CLAUDE.md".to_string(),
            kind: "delete symlink",
            reason: "govna compatibility link".to_string(),
        });
    }

    out_of_scope.extend(target_only_routes(target, canon));
    if target.join(emission::PRESERVE_PATH).is_file() {
        in_scope.push(Routing {
            path: emission::PRESERVE_PATH.to_string(),
            kind: "delete control state last",
            reason: "preserve decisions applied before registry removal".to_string(),
        });
    }

    (in_scope, out_of_scope, review)
}

fn target_only_routes(target: &Path, canon: &BTreeMap<String, String>) -> Vec<Routing> {
    let mut routes = Vec::new();
    collect_target_only(target, target, canon, &mut routes);
    routes.sort_by(|a, b| a.path.cmp(&b.path));
    routes
}

fn collect_target_only(
    root: &Path,
    dir: &Path,
    canon: &BTreeMap<String, String>,
    out: &mut Vec<Routing>,
) {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return;
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path.file_name().map(|n| n == ".git").unwrap_or(false) {
            continue;
        }
        if path.is_dir() {
            collect_target_only(root, &path, canon, out);
            continue;
        }
        let Ok(rel) = path.strip_prefix(root) else {
            continue;
        };
        let rel_str = rel.to_string_lossy().replace('\\', "/");
        if rel_str == "CLAUDE.md" || rel_str == emission::PRESERVE_PATH {
            continue;
        }
        if canon.contains_key(&rel_str) {
            continue;
        }
        out.push(Routing {
            path: rel_str,
            kind: "keep",
            reason: "target-only repo-owned file".to_string(),
        });
    }
}

// ── emission ─────────────────────────────────────────────────────────────────

fn build_stub(
    ac_num: u32,
    canon_version: &str,
    gcfg: &governance::Config,
    in_scope: &[Routing],
    out_of_scope: &[Routing],
    review: &[Routing],
) -> String {
    let mut b = String::new();
    b.push_str(&format!(
        "# AC{ac_num} Govna Removal from {canon_version}\n\n"
    ));
    b.push_str("Remove govna canon from this repo through a Director-reviewed cleanup pass.\n");
    b.push_str("\n## Summary\n\n");
    b.push_str(&format!(
        "Extricate govna canon from this consumer repo without deleting consumer-owned content. Emitted by `govna rm` against canon {canon_version}. Implement only after the Director resolves the routing decisions below.\n"
    ));
    b.push_str(
        "\nCompare each routing-pending file yourself (see the command in each bullet below) before choosing how to route it. Do not auto-delete routing-pending files until the Director chooses their routing.\n",
    );
    b.push_str("\n### Routing Decisions\n\n");
    if review.is_empty() {
        b.push_str("`None` — no review items.\n");
    } else {
        let recipe = render_canon_recipe(gcfg);
        for (i, route) in review.iter().enumerate() {
            b.push_str(&format!(
                "{}. `{}` is {}. Compare with: `{recipe} && diff -ru <scratch>/{} {}`. Choose: delete canon-shape only, keep entirely, or delete entirely.\n",
                i + 1,
                route.path,
                route.reason,
                route.path,
                route.path
            ));
        }
    }
    b.push_str("\n## In Scope\n\n");
    write_routing_list(&mut b, in_scope);
    b.push_str("\n## Out Of Scope\n\n");
    write_routing_list(&mut b, out_of_scope);
    b.push_str("\n## Acceptance Tests\n\n");
    b.push_str("**AT1** [Automated] [Pre-release gate] — Removed files listed under `## In Scope` no longer exist.\n");
    b.push_str("**AT2** [Manual] [Pre-release gate] — Director confirms every routing-pending file under `### Routing Decisions` was routed exactly as decided.\n");
    if in_scope
        .iter()
        .any(|route| route.path == emission::PRESERVE_PATH)
    {
        b.push_str("**AT3** [Automated] [Pre-release gate] — Every preserve-registry decision is applied and verified before `govna/preserve.txt` is deleted as the final control-state removal.\n");
    }
    b.push_str("\n## Status\n\n");
    b.push_str("`PENDING` — Emitted by `govna rm`; awaiting Director review.\n");
    b
}

/// Builds the `govna render` half of a Routing Decision's on-demand
/// comparison command — the flag exactly as it would need to be re-run to
/// regenerate the canon this repo was compared against.
fn render_canon_recipe(cfg: &governance::Config) -> String {
    let flavor = match cfg.repo_type {
        RepoType::Code => "code",
        RepoType::Doc => "doc",
    };
    let mut cmd = format!("govna render --flavor {flavor}");
    if cfg.repo_type == RepoType::Code && !cfg.stack.is_empty() {
        let canonical = governance::canonical_stack(&cfg.stack).unwrap_or(&cfg.stack);
        cmd.push_str(&format!(" --stack {canonical}"));
    }
    cmd.push_str(" <scratch>");
    cmd
}

fn write_routing_list(b: &mut String, routes: &[Routing]) {
    if routes.is_empty() {
        b.push_str("- None.\n");
        return;
    }
    for route in routes {
        b.push_str(&format!(
            "- `{}` — {}; {}.\n",
            route.path, route.kind, route.reason
        ));
    }
}
