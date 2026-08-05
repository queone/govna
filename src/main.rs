mod governance;
mod templates;

use governance::{Config, RepoType};
use std::env;
use std::path::{Path, PathBuf};
use std::process::ExitCode;

const PROGRAM_VERSION: &str = "0.2.0";
const SOURCE_REPO: &str = "github.com/queone/govna";

fn main() -> ExitCode {
    let args: Vec<String> = env::args().collect();

    let Some(subcmd) = args.get(1) else {
        print_usage();
        return ExitCode::from(2);
    };

    match subcmd.as_str() {
        // Required by build.sh's compiled-utility validation: exactly
        // "govna v<version>" plus a newline on stdout, nothing on stderr.
        "--version" => {
            println!("govna v{PROGRAM_VERSION}");
            ExitCode::SUCCESS
        }
        "version" | "ver" => {
            println!("govna v{PROGRAM_VERSION}\nsource: {SOURCE_REPO}");
            ExitCode::SUCCESS
        }
        "render-canon" => run_render_canon(&args[2..]),
        "apply" | "drift-scan" | "rm" | "deps" => {
            eprintln!("govna {subcmd}: not yet implemented");
            ExitCode::from(1)
        }
        "-h" | "--help" | "-?" | "help" | "h" => {
            print_usage();
            ExitCode::SUCCESS
        }
        _ => {
            eprintln!("unknown command: {subcmd}");
            print_usage();
            ExitCode::from(2)
        }
    }
}

fn print_usage() {
    eprintln!("govna v{PROGRAM_VERSION}");
    eprintln!("Repo governance templates — {SOURCE_REPO}");
    eprintln!();
    eprintln!("Usage: govna <command> [options]");
    eprintln!();
    eprintln!("  apply         apply governance template to a repo (not yet implemented)");
    eprintln!("  drift-scan    scan an adopted repo against govna canon (not yet implemented)");
    eprintln!("  rm            emit cleanup AC for removing govna canon (not yet implemented)");
    eprintln!("  deps          report direct dependency freshness (not yet implemented)");
    eprintln!("  render-canon  render flavor-specific canon files into a target directory");
    eprintln!("  --version     print version");
    eprintln!("  version, ver  print version and source info");
    eprintln!("  help, h       show this help");
}

fn print_render_canon_usage() {
    eprintln!(
        "Usage: govna render-canon [--flavor code|doc] [--stack <name>] [--module-path <path>] <target>"
    );
    eprintln!();
    eprintln!("  -f, --flavor code|doc    select consumer flavor (default: inferred from cwd)");
    eprintln!(
        "  -s, --stack <name>       select CODE stack (default: inferred from cwd manifests)"
    );
    eprintln!(
        "  -m, --module-path <path> module path for Go CODE canon (default: read from cwd's go.mod)"
    );
    eprintln!();
    eprintln!("Render canon files into <target>/ in flat repo-relative layout. Canon files only —");
    eprintln!(
        "no adoption record. Target is not pre-cleaned; remove or empty it beforehand if you"
    );
    eprintln!("need a fresh tree.");
}

/// render-canon: render flavor-specific canon files into <target>/, flat
/// repo-relative layout (e.g. <target>/AGENTS.md, <target>/govna/ac-template.md).
/// Flavor/stack/module-path are read from cwd, not the render target.
fn run_render_canon(args: &[String]) -> ExitCode {
    let mut flavor: Option<String> = None;
    let mut target: Option<String> = None;
    let mut module_path: Option<String> = None;
    let mut stack: Option<String> = None;

    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "-h" | "--help" | "-?" => {
                print_render_canon_usage();
                return ExitCode::SUCCESS;
            }
            "-f" | "--flavor" => {
                let Some(v) = args.get(i + 1) else {
                    eprintln!("--flavor requires a value");
                    return ExitCode::from(2);
                };
                flavor = Some(v.clone());
                i += 1;
            }
            "-m" | "--module-path" => {
                let Some(v) = args.get(i + 1) else {
                    eprintln!("--module-path requires a value");
                    return ExitCode::from(2);
                };
                module_path = Some(v.clone());
                i += 1;
            }
            "-s" | "--stack" => {
                let Some(v) = args.get(i + 1) else {
                    eprintln!("--stack requires a value");
                    return ExitCode::from(2);
                };
                let trimmed = v.trim();
                if trimmed.is_empty() {
                    eprintln!("--stack requires a non-empty value");
                    return ExitCode::from(2);
                }
                stack = Some(trimmed.to_string());
                i += 1;
            }
            other => {
                if let Some(existing) = &target {
                    eprintln!("unexpected argument: {other} (target already set to {existing:?})");
                    return ExitCode::from(2);
                }
                target = Some(other.to_string());
            }
        }
        i += 1;
    }

    let Some(target) = target else {
        eprintln!("render-canon requires a positional <target> argument");
        return ExitCode::from(2);
    };

    if let Some(f) = &flavor
        && f != "code"
        && f != "doc"
    {
        eprintln!("invalid --flavor: {f:?} (must be 'code' or 'doc')");
        return ExitCode::from(2);
    }

    let cwd = match env::current_dir() {
        Ok(c) => c,
        Err(e) => {
            eprintln!("get cwd: {e}");
            return ExitCode::from(1);
        }
    };

    let repo_type = match flavor.as_deref() {
        Some("code") => RepoType::Code,
        Some("doc") => RepoType::Doc,
        _ => match governance::detect_flavor(&cwd) {
            Ok(t) => t,
            Err(e) => {
                eprintln!("infer flavor from cwd: {e} (use --flavor to override)");
                return ExitCode::from(1);
            }
        },
    };

    if repo_type == RepoType::Doc && stack.is_some() {
        eprintln!("--stack applies only to CODE canon; remove --stack or select --flavor code");
        return ExitCode::from(1);
    }
    if repo_type == RepoType::Doc && module_path.is_some() {
        eprintln!(
            "--module-path applies only to Go CODE canon; remove --module-path or select --flavor code"
        );
        return ExitCode::from(1);
    }

    let mut resolved_stack = String::new();
    let mut resolved_module_path = String::new();

    if repo_type == RepoType::Code {
        resolved_stack = match stack {
            Some(s) => s,
            None => match governance::infer_stack(&cwd) {
                Some(s) => s.to_string(),
                None => {
                    eprintln!(
                        "could not infer CODE stack from cwd={}; pass --stack to override",
                        cwd.display()
                    );
                    return ExitCode::from(1);
                }
            },
        };
        if resolved_stack.eq_ignore_ascii_case("go")
            || resolved_stack.eq_ignore_ascii_case("golang")
        {
            resolved_module_path = match module_path {
                Some(m) => m,
                None => match governance::read_module_path(&cwd) {
                    Some(m) => m,
                    None => {
                        eprintln!(
                            "could not read module path from cwd's go.mod (cwd={}); pass --module-path to override",
                            cwd.display()
                        );
                        return ExitCode::from(1);
                    }
                },
            };
        } else if module_path.is_some() {
            eprintln!(
                "--module-path applies only to Go CODE canon; remove --module-path or select --stack Go"
            );
            return ExitCode::from(1);
        }
    }

    let target_path = PathBuf::from(&target);
    let abs_target = if target_path.is_absolute() {
        target_path
    } else {
        cwd.join(&target_path)
    };
    if let Err(e) = std::fs::create_dir_all(&abs_target) {
        eprintln!("create target {}: {e}", abs_target.display());
        return ExitCode::from(1);
    }

    let repo_name = governance::resolve_repo_name(&cwd, &resolved_module_path);
    let cfg = Config {
        repo_type,
        repo_name,
        stack: resolved_stack,
        module_path: resolved_module_path,
    };

    let ops = match governance::render_canonical_files(&cfg) {
        Ok(ops) => ops,
        Err(e) => {
            eprintln!("render canon: {e}");
            return ExitCode::from(1);
        }
    };

    for op in &ops {
        let dest = abs_target.join(&op.rel_path);
        if let Err(e) = write_canon_file(&dest, &op.content) {
            eprintln!("write {}: {e}", dest.display());
            return ExitCode::from(1);
        }
    }

    // Deliberate divergence from governa (AC4 Refine): governa's render-canon
    // never creates this symlink (that's apply-only there); govna's does,
    // since render-canon is usable standalone well before apply exists.
    let claude_path = abs_target.join("CLAUDE.md");
    let _ = std::fs::remove_file(&claude_path);
    if let Err(e) = std::os::unix::fs::symlink("AGENTS.md", &claude_path) {
        eprintln!("create symlink {}: {e}", claude_path.display());
        return ExitCode::from(1);
    }

    println!("{}", abs_target.display());
    ExitCode::SUCCESS
}

fn write_canon_file(dest: &Path, content: &str) -> std::io::Result<()> {
    if let Some(parent) = dest.parent() {
        std::fs::create_dir_all(parent)?;
    }
    std::fs::write(dest, content)?;
    let mode = if dest.extension().map(|e| e == "sh").unwrap_or(false) {
        0o755
    } else {
        0o644
    };
    use std::os::unix::fs::PermissionsExt;
    std::fs::set_permissions(dest, std::fs::Permissions::from_mode(mode))
}
