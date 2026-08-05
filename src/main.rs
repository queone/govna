use std::env;
use std::process::ExitCode;

const PROGRAM_VERSION: &str = "0.1.0";
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
        "apply" | "drift-scan" | "rm" | "deps" | "render-canon" => {
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
    eprintln!(
        "  render-canon  render flavor-specific canon files into a target directory (not yet implemented)"
    );
    eprintln!("  --version     print version");
    eprintln!("  version, ver  print version and source info");
    eprintln!("  help, h       show this help");
}
