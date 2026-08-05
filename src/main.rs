use clap::Parser;

#[derive(Parser)]
#[command(name = "govna", version, about = "A gradual Rust port of governa.")]
struct Cli;

fn main() {
    Cli::parse();
}
