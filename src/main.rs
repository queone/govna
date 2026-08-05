use std::env;
use std::process::ExitCode;

const VERSION: &str = env!("CARGO_PKG_VERSION");
const SOURCE_REPO: &str = "github.com/queone/govna";

fn main() -> ExitCode {
    let args: Vec<String> = env::args().collect();

    let Some(subcmd) = args.get(1) else {
        print_usage();
        return ExitCode::from(2);
    };

    match subcmd.as_str() {
        "version" | "ver" => {
            println!("govna v{VERSION}\nsource: {SOURCE_REPO}");
            ExitCode::SUCCESS
        }
        "apply" | "drift-scan" | "rm" | "deps" | "render-canon" => {
            eprintln!("govna {subcmd}: not yet implemented");
            ExitCode::from(1)
        }
        "-h" | "--help" | "-?" | "help" | "h" => {
            print_usage();
            ExitCode::SUCCESS
        }
        // TODO: `--version`/`-V` deliberately falls through to the
        // unknown-command branch below rather than being handled as a bare
        // flag (clap's derive API would give that for free). governa itself
        // has no bare version flag, only the `version`/`ver` subcommand.
        // Revisit adding a bare `--version` as a convenience once govna
        // reaches parity with governa — not now.
        _ => {
            eprintln!("unknown command: {subcmd}");
            print_usage();
            ExitCode::from(2)
        }
    }
}

fn print_usage() {
    eprintln!("govna v{VERSION}");
    eprintln!("Repo governance templates — {SOURCE_REPO}");
    eprintln!();
    eprintln!("Usage: govna <command> [options]");
    eprintln!();
    eprintln!("  apply         apply governance template to a repo (not yet implemented)");
    eprintln!("  drift-scan    scan an adopted repo against governa canon (not yet implemented)");
    eprintln!("  rm            emit cleanup AC for removing Governa canon (not yet implemented)");
    eprintln!("  deps          report direct dependency freshness (not yet implemented)");
    eprintln!(
        "  render-canon  render flavor-specific canon files into a target directory (not yet implemented)"
    );
    eprintln!("  version, ver  print version and source info");
    eprintln!("  help, h       show this help");
}
