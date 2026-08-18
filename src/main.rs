mod apply;
mod driftscan;
mod emission;
mod governance;
mod rm;
mod templates;

use governance::{Config, RepoType};
use std::env;
use std::io::IsTerminal;
use std::path::{Path, PathBuf};
use std::process::ExitCode;

const PROGRAM_VERSION: &str = "0.23.0";
const SOURCE_REPO: &str = "github.com/queone/govna";

fn main() -> ExitCode {
    let args: Vec<String> = env::args().collect();

    let Some(subcmd) = args.get(1) else {
        print_usage();
        return ExitCode::from(2);
    };

    match subcmd.as_str() {
        // Also required by build.sh's compiled-utility validation: exactly
        // "govna v<version>" plus a newline on stdout, nothing on stderr.
        "--version" | "version" | "ver" | "v" => {
            println!("govna v{PROGRAM_VERSION}");
            ExitCode::SUCCESS
        }
        "render" | "render-canon" => run_render(&args[2..]),
        "audit" | "drift-scan" => driftscan::run_cli(&args[2..]),
        "apply" => apply::run_cli(&args[2..]),
        "rm" => rm::run_cli(&args[2..]),
        "-h" | "--help" | "-?" | "help" | "h" => {
            print_usage_stdout();
            ExitCode::SUCCESS
        }
        _ => {
            eprintln!("unknown command: {subcmd}");
            print_usage();
            ExitCode::from(2)
        }
    }
}

// ── color ────────────────────────────────────────────────────────────────

// Color gating: enabled when NO_COLOR is unset, TERM isn't dumb, and
// stderr is a TTY, AND the terminal is 256-color capable (COLORTERM is
// truecolor/24bit, or TERM contains 256color).
fn color_enabled() -> bool {
    if std::env::var_os("NO_COLOR").is_some() {
        return false;
    }
    if std::env::var("TERM").map(|t| t == "dumb").unwrap_or(false) {
        return false;
    }
    if !std::io::stderr().is_terminal() {
        return false;
    }
    matches!(
        std::env::var("COLORTERM").as_deref(),
        Ok("truecolor") | Ok("24bit")
    ) || std::env::var("TERM")
        .map(|t| t.contains("256color"))
        .unwrap_or(false)
}

fn colorize(code: &str, s: &str) -> String {
    if color_enabled() {
        format!("\x1b[{code}m{s}\x1b[0m")
    } else {
        s.to_string()
    }
}

// 256-color white (231), bold.
fn bold_white(s: &str) -> String {
    if color_enabled() {
        format!("\x1b[1m\x1b[38;5;231m{s}\x1b[0m")
    } else {
        s.to_string()
    }
}

// 256-color index 245.
fn dark_gray(s: &str) -> String {
    colorize("38;5;245", s)
}

/// 2-space indent, description aligned at column 32 — matches
/// `govna/development-guidelines.md`'s CLI Usage Formatting convention and
/// build.sh's own `_emit_usage_line`.
fn usage_line(flag: &str, desc: &str) -> String {
    let prefix_len = 2 + flag.chars().count();
    let pad = if prefix_len < 32 { 32 - prefix_len } else { 2 };
    format!("  {flag}{}{desc}", " ".repeat(pad))
}

fn print_usage() {
    eprint!("{}", usage_text());
}

fn print_usage_stdout() {
    print!("{}", usage_text());
}

fn usage_text() -> String {
    format!(
        "{} v{PROGRAM_VERSION}\n{}\n\n{} govna <command> [options]\n\n{}\n{}\n{}\n{}\n{}\n{}\n\nRun 'govna <command> -h' for command-specific flags.\n",
        bold_white("govna"),
        dark_gray(&format!("Repo governance templates — {SOURCE_REPO}")),
        bold_white("Usage:"),
        usage_line("apply", "apply governance template to a repo"),
        usage_line("audit", "drift scan an adopted repo against govna canon"),
        usage_line("rm", "emit cleanup AC for removing govna canon"),
        usage_line(
            "render",
            "render flavor-specific canon files into a target directory"
        ),
        usage_line("ver, v, --version", "print version"),
        usage_line("help, h", "show this help"),
    )
}

fn print_render_usage() {
    eprintln!(
        "{} govna render [--flavor code|doc] [--stack <name>] [--module-path <path>] <target>",
        bold_white("Usage:")
    );
    eprintln!();
    eprintln!(
        "{}",
        usage_line(
            "-f, --flavor code|doc",
            "select consumer flavor (default: inferred from cwd)"
        )
    );
    eprintln!(
        "{}",
        usage_line(
            "-s, --stack <name>",
            "select CODE stack (default: inferred from cwd manifests)"
        )
    );
    eprintln!(
        "{}",
        usage_line(
            "-m, --module-path <path>",
            "module path for Go CODE canon (default: read from cwd's go.mod)"
        )
    );
    eprintln!();
    eprintln!("Render canon files into <target>/ in flat repo-relative layout. Canon files only —");
    eprintln!(
        "no adoption record. Target is not pre-cleaned; remove or empty it beforehand if you"
    );
    eprintln!("need a fresh tree.");
}

/// render: render flavor-specific canon files into <target>/, flat
/// repo-relative layout (e.g. <target>/AGENTS.md, <target>/govna/ac-template.md).
/// Flavor/stack/module-path are read from cwd, not the render target.
fn run_render(args: &[String]) -> ExitCode {
    let mut flavor: Option<String> = None;
    let mut target: Option<String> = None;
    let mut module_path: Option<String> = None;
    let mut stack: Option<String> = None;

    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "-h" | "--help" | "-?" => {
                print_render_usage();
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
        eprintln!("render requires a positional <target> argument");
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

    // render creates the CLAUDE.md symlink even standalone, since it's
    // usable well before apply exists — unlike apply, which uses the
    // preserve_regular_file variant instead since it runs against real repos
    // with real user content to protect. render's typical target is a
    // scratch dir, so this uses an unconditional remove+recreate.
    if let Err(e) = write_claude_symlink(&abs_target, false) {
        eprintln!("{e}");
        return ExitCode::from(1);
    }

    println!("{}", abs_target.display());
    ExitCode::SUCCESS
}

pub(crate) enum SymlinkOutcome {
    Created,
    Skipped,
}

/// Writes `<target>/CLAUDE.md` as a symlink to `AGENTS.md`. When
/// `preserve_regular_file` is set, a `CLAUDE.md` that already exists as a
/// regular (non-symlink) file is left untouched instead of being clobbered;
/// otherwise (missing, or already a symlink) it's removed and recreated.
pub(crate) fn write_claude_symlink(
    target: &Path,
    preserve_regular_file: bool,
) -> Result<SymlinkOutcome, String> {
    let claude_path = target.join("CLAUDE.md");
    if preserve_regular_file
        && let Ok(meta) = std::fs::symlink_metadata(&claude_path)
        && !meta.file_type().is_symlink()
    {
        return Ok(SymlinkOutcome::Skipped);
    }
    let _ = std::fs::remove_file(&claude_path);
    std::os::unix::fs::symlink("AGENTS.md", &claude_path)
        .map_err(|e| format!("create symlink {}: {e}", claude_path.display()))?;
    Ok(SymlinkOutcome::Created)
}

pub(crate) fn write_canon_file(dest: &Path, content: &str) -> std::io::Result<()> {
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
