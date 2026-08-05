use std::process::Command;

#[test]
fn version_flag_is_exact() {
    let output = Command::new(env!("CARGO_BIN_EXE_govna"))
        .arg("--version")
        .output()
        .unwrap();
    assert!(output.status.success());
    assert_eq!(output.stdout, b"govna v0.1.0\n");
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
