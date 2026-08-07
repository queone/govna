//! Shared helpers for govna CLI commands that emit AC artifacts into
//! consumer repositories.

use regex::Regex;
use sha2::{Digest, Sha256};
use std::collections::BTreeSet;
use std::path::Path;

const MARKER_INFIX: &str = "; emission-sha=";
const MARKER_SUFFIX: &str = " -->";

fn file_exists(p: &Path) -> bool {
    p.is_file()
}

/// Reports whether `dir` looks like a govna source checkout: `Cargo.toml`
/// declares `name = "govna"`, `templates/base/AGENTS.md` exists, and
/// `src/main.rs` carries the literal `SOURCE_REPO` declaration. The third
/// check is load-bearing: checking only the bare package name would
/// false-positive on any coincidentally-named `govna` Cargo package
/// elsewhere, so this also requires the literal `SOURCE_REPO` declaration
/// unique to this repo.
pub fn is_govna_checkout(dir: &Path) -> bool {
    let cargo_toml_ok = std::fs::read_to_string(dir.join("Cargo.toml"))
        .map(|s| s.lines().any(|l| l.trim() == "name = \"govna\""))
        .unwrap_or(false);
    if !cargo_toml_ok {
        return false;
    }
    if !dir.join("templates/base/AGENTS.md").is_file() {
        return false;
    }
    std::fs::read_to_string(dir.join("src/main.rs"))
        .map(|s| s.contains(r#"SOURCE_REPO: &str = "github.com/queone/govna""#))
        .unwrap_or(false)
}

/// Prevents consumer-run commands from targeting govna's own source repo.
pub fn refuse_govna_source(target: &Path, tool: &str) -> Result<(), String> {
    if is_govna_checkout(target) {
        return Err(format!(
            "{tool}: target {} looks like a govna checkout — {tool} is for adopted repos, not the govna source",
            target.display()
        ));
    }
    Ok(())
}

/// Verifies target carries govna adoption signals.
pub fn require_govna_adopted(target: &Path, tool: &str) -> Result<(), String> {
    if !file_exists(&target.join("AGENTS.md")) {
        return Err(format!(
            "{tool}: {} is not a govna-adopted repo (AGENTS.md not found); run from the consumer repo root after `govna render`",
            target.display()
        ));
    }
    for sig in [
        "govna/ac-template.md",
        "govna/release.md",
        "govna/build-release.md",
    ] {
        if file_exists(&target.join(sig)) {
            return Ok(());
        }
    }
    if let Ok(changelog) = std::fs::read_to_string(target.join("CHANGELOG.md")) {
        let re = Regex::new(r"(?i)govna\s+(apply|render|render-canon)").unwrap();
        if re.is_match(&changelog) {
            return Ok(());
        }
    }
    Err(format!(
        "{tool}: {} has AGENTS.md but no govna adoption signal (expected one of: govna/ac-template.md, govna/release.md, govna/build-release.md, or a CHANGELOG row referencing 'govna apply' or 'govna render'); ensure you are running from a govna-adopted repo root",
        target.display()
    ))
}

/// Creates the artifact directory used by emitted AC files.
pub fn ensure_docs_dir(target: &Path, tool: &str) -> Result<(), String> {
    std::fs::create_dir_all(target.join("govna"))
        .map_err(|e| format!("{tool}: ensure govna/ exists: {e}"))
}

pub const PRESERVE_PATH: &str = "govna/preserve.txt";
const PRESERVE_SCHEMA: &str = "govna-preserve-v1";

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct LegacyPreserveMarker {
    pub relpath: String,
    pub phrase: String,
}

/// Reads the optional consumer-owned preserve registry. Absence is empty;
/// presence is strict so malformed durable routing state cannot be ignored.
pub fn preserve_registry(target_root: &Path) -> Result<BTreeSet<String>, String> {
    let path = target_root.join(PRESERVE_PATH);
    let content = match std::fs::read_to_string(&path) {
        Ok(content) => content,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(BTreeSet::new()),
        Err(error) => return Err(format!("read {PRESERVE_PATH}: {error}")),
    };
    if !content.ends_with('\n') {
        return Err(format!("invalid {PRESERVE_PATH}: require a final newline"));
    }
    let mut lines = content.strip_suffix('\n').unwrap().split('\n');
    if lines.next() != Some(PRESERVE_SCHEMA) {
        return Err(format!(
            "invalid {PRESERVE_PATH}: first line must be {PRESERVE_SCHEMA}"
        ));
    }
    let mut entries = BTreeSet::new();
    let mut previous: Option<String> = None;
    for relpath in lines {
        validate_preserve_path(relpath)?;
        if previous.as_deref().is_some_and(|prior| prior >= relpath) {
            return Err(format!(
                "invalid {PRESERVE_PATH}: entries must be unique and byte-sorted"
            ));
        }
        previous = Some(relpath.to_string());
        entries.insert(relpath.to_string());
    }
    Ok(entries)
}

fn validate_preserve_path(relpath: &str) -> Result<(), String> {
    let invalid = relpath.is_empty()
        || relpath == PRESERVE_PATH
        || relpath.starts_with('/')
        || relpath.ends_with('/')
        || relpath.contains('\\')
        || relpath.contains('\t')
        || relpath
            .split('/')
            .any(|part| part.is_empty() || part == "." || part == "..");
    if invalid {
        return Err(format!(
            "invalid {PRESERVE_PATH}: invalid repo-relative path {relpath:?}"
        ));
    }
    Ok(())
}

/// Finds accepted legacy phrases only in the Unreleased CHANGELOG Summary.
/// They are migration evidence, never preserve authority.
pub fn legacy_preserve_markers(target_root: &Path) -> Vec<LegacyPreserveMarker> {
    let Ok(changelog) = std::fs::read_to_string(target_root.join("CHANGELOG.md")) else {
        return Vec::new();
    };
    let Some(summary) = changelog.lines().find_map(|line| {
        let cells: Vec<&str> = line.split('|').collect();
        (cells.len() >= 4 && cells[1].trim() == "Unreleased").then(|| cells[2].trim())
    }) else {
        return Vec::new();
    };
    let mut markers = Vec::new();
    for segment in summary.split(';').map(str::trim).filter(|s| !s.is_empty()) {
        let relpath = if let Some(rest) = segment.strip_prefix("preserve ") {
            rest.split_whitespace().next()
        } else if let Some(rest) = segment.strip_prefix("do not sync ") {
            rest.split_whitespace().next()
        } else if let Some(rest) = segment.strip_prefix("intentional divergence: ") {
            rest.split_whitespace().next()
        } else {
            segment.strip_suffix(": keep local").map(str::trim)
        };
        let Some(relpath) = relpath else { continue };
        if validate_preserve_path(relpath).is_ok() {
            markers.push(LegacyPreserveMarker {
                relpath: relpath.to_string(),
                phrase: segment.to_string(),
            });
        }
    }
    markers.sort_by(|a, b| (&a.relpath, &a.phrase).cmp(&(&b.relpath, &b.phrase)));
    markers.dedup();
    markers
}

/// Reuses a same-canon-version stub's AC number if one exists, else
/// allocates the next monotonic AC number from `<target>/govna/ac*.md`
/// filenames plus `AC\d+` references in `git log --all --pretty=%B`.
/// Returns `(number, reused)`.
pub fn allocate_ac_number(
    target: &Path,
    slug_stem: &str,
    canon_version: &str,
) -> Result<(u32, bool), String> {
    let docs_dir = target.join("govna");
    let suffix = format!("-{slug_stem}-{canon_version}.md");
    let mut stubs = Vec::new();
    if let Ok(entries) = std::fs::read_dir(&docs_dir) {
        for entry in entries.flatten() {
            let name = entry.file_name().to_string_lossy().to_string();
            if name.starts_with("ac") && name.ends_with(&suffix) {
                stubs.push(name);
            }
        }
    }

    let stub_re = Regex::new(r"^ac(\d+)-").unwrap();
    match stubs.len() {
        1 => {
            let base = &stubs[0];
            let caps = stub_re
                .captures(base)
                .ok_or_else(|| format!("unexpected emitted AC filename: {base}"))?;
            let n: u32 = caps[1]
                .parse()
                .map_err(|e| format!("parse AC number from {base}: {e}"))?;
            return Ok((n, true));
        }
        0 => {}
        _ => {
            return Err(format!(
                "multiple emitted AC stubs for {slug_stem} {canon_version}: {stubs:?}"
            ));
        }
    }

    Ok((next_ac_number(target)?, false))
}

/// Computes the next monotonic AC number for `target`: the max of (a) AC
/// numbers in `<target>/govna/ac*.md` filenames and (b) `AC\d+` references
/// in `git log --all --pretty=%B`, plus one. A target with no `.git/` at
/// all (e.g. a brand-new `apply` bootstrap target) is treated the same as
/// one with no commits yet — empty history, not an error.
pub fn next_ac_number(target: &Path) -> Result<u32, String> {
    let docs_dir = target.join("govna");
    let stub_re = Regex::new(r"^ac(\d+)-").unwrap();

    let mut max_n = 0u32;
    if let Ok(entries) = std::fs::read_dir(&docs_dir) {
        for entry in entries.flatten() {
            if entry.path().is_dir() {
                continue;
            }
            let name = entry.file_name();
            let name = name.to_string_lossy();
            if let Some(caps) = stub_re.captures(&name)
                && let Ok(n) = caps[1].parse::<u32>()
            {
                max_n = max_n.max(n);
            }
        }
    }

    let output = std::process::Command::new("git")
        .args([
            "-C",
            &target.to_string_lossy(),
            "log",
            "--all",
            "--pretty=%B",
        ])
        .output()
        .map_err(|e| {
            format!(
                "read git log for AC-number allocation in {}: {e}",
                target.display()
            )
        })?;
    let log_text = if output.status.success() {
        String::from_utf8_lossy(&output.stdout).to_string()
    } else {
        let stderr = String::from_utf8_lossy(&output.stderr);
        if stderr.contains("does not have any commits")
            || stderr.contains("bad default revision")
            || stderr.contains("Not a valid object name")
            || stderr.contains("not a git repository")
        {
            String::new()
        } else {
            return Err(format!(
                "read git log for AC-number allocation in {}: git exited {}: {}",
                target.display(),
                output.status.code().unwrap_or(-1),
                stderr.trim()
            ));
        }
    };

    let ac_ref_re = Regex::new(r"\bAC(\d+)\b").unwrap();
    for caps in ac_ref_re.captures_iter(&log_text) {
        if let Ok(n) = caps[1].parse::<u32>() {
            max_n = max_n.max(n);
        }
    }
    Ok(max_n + 1)
}

/// Checks whether an emitted file body still matches its marker hash.
pub fn verify_unedited(path: &Path, marker_prefix: &str) -> Result<bool, String> {
    let data = std::fs::read_to_string(path).map_err(|e| e.to_string())?;
    let Some(idx) = data.find('\n') else {
        return Ok(false);
    };
    let stored = parse_marker(&data[..idx], marker_prefix);
    if stored.is_empty() {
        return Ok(false);
    }
    Ok(stored == body_sha(&data[idx + 1..]))
}

/// Writes marker + body, preserving edit-detection metadata.
pub fn write_with_marker(
    path: &Path,
    marker_prefix: &str,
    canon_version: &str,
    body: &str,
) -> Result<(), String> {
    let marker = format!(
        "{marker_prefix}{canon_version}{MARKER_INFIX}{}{MARKER_SUFFIX}",
        body_sha(body)
    );
    std::fs::write(path, format!("{marker}\n{body}")).map_err(|e| e.to_string())
}

fn parse_marker(line: &str, marker_prefix: &str) -> String {
    if !line.starts_with(marker_prefix) || !line.ends_with(MARKER_SUFFIX) {
        return String::new();
    }
    let inner = &line[marker_prefix.len()..line.len() - MARKER_SUFFIX.len()];
    match inner.split_once(MARKER_INFIX) {
        Some((_, sha)) => sha.to_string(),
        None => String::new(),
    }
}

fn body_sha(body: &str) -> String {
    let mut hasher = Sha256::new();
    hasher.update(body.as_bytes());
    hasher
        .finalize()
        .iter()
        .map(|b| format!("{b:02x}"))
        .collect()
}
