//! Implements the `govna audit` subcommand.
//!
//! Runs against the current working directory (no positional arguments)
//! after a positive govna-adoption check, walks the canon overlay embedded
//! in the binary, byte-compares each governed file against the cwd,
//! classifies divergences, collects evidence (preserve markers, git log),
//! emits an AC stub when actionable drift exists, and reports a clean result
//! without writing when only match, expected-divergence, and ordinary preserve
//! classifications remain. Per-file diffs are not snapshotted — adopters use
//! `govna render` + standard `diff -ru` to inspect changes.

use crate::emission;
use crate::governance::{self, RepoType};
use crate::templates;
use regex::Regex;
use serde::Serialize;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use std::path::{Path, PathBuf};
use std::process::{Command, ExitCode};

// ── classification ─────────────────────────────────────────────────────────

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize)]
pub enum Classification {
    #[serde(rename = "match")]
    Match,
    #[serde(rename = "preserve")]
    Preserve,
    #[serde(rename = "ambiguity")]
    Ambiguity,
    #[serde(rename = "clear-sync")]
    ClearSync,
    #[serde(rename = "missing-in-target")]
    MissingTarget,
    #[serde(rename = "target-has-no-canon")]
    TargetNoCanon,
    #[serde(rename = "migration-required")]
    MigrationRequired,
    #[serde(rename = "expected-divergence")]
    ExpectedDivergence,
}

impl Classification {
    pub fn as_str(&self) -> &'static str {
        match self {
            Classification::Match => "match",
            Classification::Preserve => "preserve",
            Classification::Ambiguity => "ambiguity",
            Classification::ClearSync => "clear-sync",
            Classification::MissingTarget => "missing-in-target",
            Classification::TargetNoCanon => "target-has-no-canon",
            Classification::MigrationRequired => "migration-required",
            Classification::ExpectedDivergence => "expected-divergence",
        }
    }
}

impl std::fmt::Display for Classification {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(self.as_str())
    }
}

const REACHABILITY_HEADER_REMINDER: &str = "Reachability check: verify divergent canon-code branches reach this consumer's structure before treating as drift.";
const ROUTING_RESOLUTION_REMINDER: &str = "Routing resolution: resolve every routing decision in chat and leave this emitted stub unchanged. Every Director-resolved routing target is effective implementation scope even when absent from `## In Scope`; an explicitly named migration destination joins that scope, and `CHANGELOG.md` joins it when a preserve marker is required. Do not infer an unnamed migration destination.";
const AUDIT_MARKER_PREFIX: &str = "<!-- audit: emitted-by govna ";
const BASELINE_SCHEMA: &str = "govna-canon-baseline-v1";
const RETIRED_CANON_PATHS: &[(&str, &str)] = &[("govna/drift-scan.md", "govna/audit.md")];

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
struct StableVersion(u64, u64, u64);

fn parse_canon_version(value: &str, source: &str) -> Result<StableVersion, String> {
    let Some(raw) = value.strip_prefix('v') else {
        return Err(format!(
            "audit: invalid {source} canon_version {value:?}; restore a strict vMAJOR.MINOR.PATCH value before re-running"
        ));
    };
    let parts: Vec<&str> = raw.split('.').collect();
    if parts.len() != 3
        || parts.iter().any(|part| {
            part.is_empty()
                || !part.bytes().all(|byte| byte.is_ascii_digit())
                || (part.len() > 1 && part.starts_with('0'))
        })
    {
        return Err(format!(
            "audit: invalid {source} canon_version {value:?}; restore a strict vMAJOR.MINOR.PATCH value before re-running"
        ));
    }
    let mut parsed = [0_u64; 3];
    for (index, part) in parts.iter().enumerate() {
        parsed[index] = part.parse::<u64>().map_err(|_| {
            format!(
                "audit: invalid {source} canon_version {value:?}; restore a strict vMAJOR.MINOR.PATCH value before re-running"
            )
        })?;
    }
    Ok(StableVersion(parsed[0], parsed[1], parsed[2]))
}

fn metadata_value<'a>(content: &'a str, key: &str) -> Option<&'a str> {
    content
        .lines()
        .find_map(|line| line.strip_prefix(&format!("{key} = ")))
}

fn validate_metadata_versions(
    target_value: &str,
    embedded_value: &str,
) -> Result<(StableVersion, StableVersion), String> {
    let target = parse_canon_version(target_value, "target")?;
    let embedded = parse_canon_version(embedded_value, "embedded")?;
    if target > embedded {
        return Err(format!(
            "audit: target canon_version {target_value} is newer than embedded canon {embedded_value}; upgrade govna before auditing so consumer metadata is not downgraded"
        ));
    }
    Ok((target, embedded))
}

#[derive(Debug)]
struct BaselineEntry {
    hash: String,
}

#[derive(Debug)]
struct Baseline {
    entries: BTreeMap<String, BaselineEntry>,
}

fn parse_baseline(content: &str, embedded_version: &str, flavor: &str) -> Result<Baseline, String> {
    if !content.ends_with('\n') {
        return Err("audit: invalid govna/canon-baseline.txt: require a final newline".to_string());
    }
    let mut lines = content.trim_end_matches('\n').split('\n');
    if lines.next() != Some(BASELINE_SCHEMA) {
        return Err(format!(
            "audit: invalid govna/canon-baseline.txt: first line must be {BASELINE_SCHEMA}"
        ));
    }
    let version_line = lines.next().ok_or_else(|| {
        "audit: invalid govna/canon-baseline.txt: missing canon_version".to_string()
    })?;
    let version = version_line.strip_prefix("canon_version = ").ok_or_else(|| {
        "audit: invalid govna/canon-baseline.txt: second line must be canon_version = vMAJOR.MINOR.PATCH".to_string()
    })?;
    let baseline_version = parse_canon_version(version, "baseline")?;
    let embedded_version_parsed = parse_canon_version(embedded_version, "embedded")?;
    if baseline_version > embedded_version_parsed {
        return Err(format!(
            "audit: baseline canon_version {version} is newer than embedded canon {embedded_version}; upgrade govna before auditing"
        ));
    }

    let mut entries = BTreeMap::new();
    let mut previous = None::<String>;
    for line in lines {
        let fields: Vec<&str> = line.split('\t').collect();
        if fields.len() != 3 || fields.iter().any(|field| field.is_empty()) {
            return Err("audit: invalid govna/canon-baseline.txt: each entry must be <path><TAB><scope><TAB><sha256>".to_string());
        }
        let (relpath, scope, hash) = (fields[0], fields[1], fields[2]);
        if relpath == governance::BASELINE_PATH {
            return Err(
                "audit: invalid govna/canon-baseline.txt: manifest must exclude itself".to_string(),
            );
        }
        if previous.as_deref().is_some_and(|path| path >= relpath) {
            return Err(
                "audit: invalid govna/canon-baseline.txt: paths must be unique and sorted"
                    .to_string(),
            );
        }
        previous = Some(relpath.to_string());
        if hash.len() != 64
            || !hash
                .bytes()
                .all(|byte| byte.is_ascii_hexdigit() && !byte.is_ascii_uppercase())
        {
            return Err(format!(
                "audit: invalid govna/canon-baseline.txt: invalid SHA-256 for {relpath}"
            ));
        }
        let expected_boundary = governance::mixed_content_boundary(relpath);
        match (scope, expected_boundary) {
            ("full", None) => {}
            (scope, Some(boundary)) if scope == format!("before:{boundary}") => {}
            ("full", Some(_))
                if flavor == "code"
                    && relpath == "govna/build-release.md"
                    && baseline_version < StableVersion(0, 11, 0) => {}
            (scope, Some(boundary)) => {
                return Err(format!(
                    "audit: invalid govna/canon-baseline.txt: {relpath} scope {scope:?} does not match registered boundary {boundary:?}"
                ));
            }
            (scope, None) if scope.starts_with("before:") => {
                return Err(format!(
                    "audit: invalid govna/canon-baseline.txt: {relpath} declares an unregistered boundary scope {scope:?}"
                ));
            }
            _ => {
                return Err(format!(
                    "audit: invalid govna/canon-baseline.txt: unknown scope {scope:?} for {relpath}"
                ));
            }
        }
        entries.insert(
            relpath.to_string(),
            BaselineEntry {
                hash: hash.to_string(),
            },
        );
    }
    Ok(Baseline { entries })
}

fn region_hash(content: &str, relpath: &str) -> Option<String> {
    let region = if let Some(boundary) = governance::mixed_content_boundary(relpath) {
        governance::extract_canon_zone(content, boundary)?
    } else {
        content.to_string()
    };
    Some(
        Sha256::digest(region.as_bytes())
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect(),
    )
}

// ── report types ────────────────────────────────────────────────────────────

#[derive(Debug, Clone, Serialize)]
pub struct FileResult {
    pub relpath: String,
    pub classification: Classification,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub diff: String,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub commits: Vec<String>,
    #[serde(rename = "preserve_markers", skip_serializing_if = "Vec::is_empty")]
    pub markers: Vec<String>,
    #[serde(rename = "canon_ref", skip_serializing_if = "String::is_empty")]
    pub canon_ref: String,
    #[serde(rename = "compare_command", skip_serializing_if = "String::is_empty")]
    pub compare_command: String,
    /// Rendered canon body. Not serialized — used internally by
    /// `build_ac_stub` for missing-in-target previews.
    #[serde(skip)]
    pub canon_content: String,
    /// Declared for JSON-shape compatibility with governa's own `--json`
    /// output (some consumers may parse either tool's output uniformly);
    /// never populated, matching governa's own current behavior.
    #[serde(rename = "coupled_local_only", skip_serializing_if = "Vec::is_empty")]
    pub coupled_local_only: Vec<String>,
    #[serde(skip_serializing_if = "String::is_empty")]
    pub boundary: String,
}

impl FileResult {
    fn new(relpath: impl Into<String>) -> Self {
        FileResult {
            relpath: relpath.into(),
            classification: Classification::Match,
            diff: String::new(),
            commits: Vec::new(),
            markers: Vec::new(),
            canon_ref: String::new(),
            compare_command: String::new(),
            canon_content: String::new(),
            coupled_local_only: Vec::new(),
            boundary: String::new(),
        }
    }
}

#[derive(Debug, Clone, Serialize)]
pub struct ReportHeader {
    pub invocation: String,
    #[serde(rename = "canon_sha")]
    pub canon_sha: String,
    pub target: String,
    pub flavor: String,
    #[serde(rename = "flavor_source")]
    pub flavor_source: String,
    #[serde(rename = "repo_name")]
    pub repo_name: String,
    #[serde(rename = "canon_version", skip_serializing_if = "String::is_empty")]
    pub canon_version: String,
    #[serde(rename = "code_stack", skip_serializing_if = "String::is_empty")]
    pub code_stack: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct EmittedPaths {
    #[serde(rename = "ac_stub")]
    pub ac_stub: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct Report {
    pub header: ReportHeader,
    pub files: Vec<FileResult>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub emitted: Option<EmittedPaths>,
}

// ── config / CLI ────────────────────────────────────────────────────────────

pub struct Config {
    pub target: PathBuf,
    pub flavor: String,
    pub stack: String,
    pub json: bool,
    pub diff_lines: usize,
    pub repo_name: String,
    pub invocation: String,
    pub override_canon_id: String,
}

fn print_usage() {
    eprintln!("Usage: govna audit [options]");
    eprintln!();
    eprintln!("Scan an adopted-govna repo against canon. Run from the consumer repo root");
    eprintln!("(no positional arguments). Emits an AC stub under govna/.");
    eprintln!();
    eprintln!("Flags:");
    eprintln!("  -f, --flavor code|doc      overlay flavor (default: auto-detect)");
    eprintln!("  -s, --stack <name>         CODE stack (default: inferred from manifests)");
    eprintln!("  -j, --json                 emit JSON report alongside markdown emission");
    eprintln!("  -l, --diff-lines <N>       diff truncation limit (default: 200)");
    eprintln!("  -n, --repo-name <name>     override repo name (default: basename of cwd)");
    eprintln!("  -h, --help                 show this help");
}

pub fn parse_args(args: &[String]) -> Result<(Config, bool), String> {
    let mut cfg = Config {
        target: PathBuf::new(),
        flavor: String::new(),
        stack: String::new(),
        json: false,
        diff_lines: 200,
        repo_name: String::new(),
        invocation: String::new(),
        override_canon_id: String::new(),
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
                    return Err("audit: -f, --flavor <code|doc> requires a value".to_string());
                };
                cfg.flavor = v.clone();
                i += 1;
            }
            "-s" | "--stack" => {
                let Some(v) = args.get(i + 1) else {
                    return Err("audit: -s, --stack <name> requires a value".to_string());
                };
                if v.trim().is_empty() {
                    return Err("audit: -s, --stack <name> requires a non-empty value".to_string());
                }
                cfg.stack = v.trim().to_string();
                i += 1;
            }
            "-j" | "--json" => {
                cfg.json = true;
            }
            "-l" | "--diff-lines" => {
                let Some(v) = args.get(i + 1) else {
                    return Err("audit: -l, --diff-lines <N> requires a value".to_string());
                };
                cfg.diff_lines = v.parse().map_err(|_| {
                    format!("audit: --diff-lines must be a non-negative integer, got {v:?}")
                })?;
                i += 1;
            }
            "-n" | "--repo-name" => {
                let Some(v) = args.get(i + 1) else {
                    return Err("audit: -n, --repo-name <name> requires a value".to_string());
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
            "audit: no positional arguments accepted; run from the consumer repo root (got: {positional:?})"
        ));
    }

    if !cfg.flavor.is_empty() && cfg.flavor != "code" && cfg.flavor != "doc" {
        return Err(format!(
            "audit: --flavor must be code or doc, got {:?}",
            cfg.flavor
        ));
    }
    if cfg.flavor == "doc" && !cfg.stack.is_empty() {
        return Err(
            "audit: --stack applies only to CODE canon; remove --stack or select --flavor code"
                .to_string(),
        );
    }

    let cwd = std::env::current_dir().map_err(|e| format!("audit: get cwd: {e}"))?;
    cfg.target = cwd;
    cfg.invocation = format!("govna audit {}", args.join(" "));

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

pub fn run(cfg: Config) -> ExitCode {
    match run_inner(&cfg) {
        Ok(code) => code,
        Err(e) => {
            eprintln!("{e}");
            ExitCode::from(1)
        }
    }
}

fn run_inner(cfg: &Config) -> Result<ExitCode, String> {
    if !cfg.target.is_dir() {
        return Err(format!(
            "audit: target {} is not a directory",
            cfg.target.display()
        ));
    }

    emission::refuse_govna_source(&cfg.target, "audit")?;
    emission::require_govna_adopted(&cfg.target, "audit")?;

    if !cfg.target.join(".git").exists() {
        return Err(format!(
            "audit: target {} is not a git worktree (no .git/) — audit needs git history for migration fallback; run `git init` and commit, or pass an already-rendered target",
            cfg.target.display()
        ));
    }
    if Command::new("git").arg("--version").output().is_err() {
        return Err(
            "audit: git binary not found on PATH; install git before running audit".to_string(),
        );
    }

    let sha = if cfg.override_canon_id.is_empty() {
        format!("v{}", templates::CANON_VERSION)
    } else {
        cfg.override_canon_id.clone()
    };

    let flavor_input = cfg.flavor.trim();
    let stack_input = cfg.stack.trim();
    let (flavor, flavor_source) = if !flavor_input.is_empty() {
        (flavor_input.to_string(), "explicit".to_string())
    } else {
        let (repo_type, source) = governance::detect_flavor_with_source(&cfg.target)?;
        let flavor = match repo_type {
            RepoType::Code => "code",
            RepoType::Doc => "doc",
        };
        (flavor.to_string(), source.to_string())
    };
    if flavor != "code" && flavor != "doc" {
        return Err(format!(
            "audit: --flavor must be code or doc, got {flavor:?}"
        ));
    }
    if flavor == "doc" && !stack_input.is_empty() {
        return Err(
            "audit: --stack applies only to CODE canon; remove --stack or select --flavor code"
                .to_string(),
        );
    }

    let repo_name = if cfg.repo_name.is_empty() {
        cfg.target
            .file_name()
            .map(|n| n.to_string_lossy().to_string())
            .unwrap_or_default()
    } else {
        cfg.repo_name.clone()
    };

    let metadata = governance::read_repo_metadata(&cfg.target)?.unwrap_or_default();
    let metadata_present = !metadata.is_empty();

    let repo_type = if flavor == "code" {
        RepoType::Code
    } else {
        RepoType::Doc
    };
    let mut stack = stack_input.to_string();
    if flavor == "code" && stack.is_empty() {
        stack = governance::infer_stack(&cfg.target)
            .unwrap_or_default()
            .to_string();
        if stack.is_empty() && metadata_present {
            stack = metadata.get("code_stack").cloned().unwrap_or_default();
        }
        if stack.is_empty() {
            return Err(format!(
                "audit: cannot resolve CODE stack for target {}; pass -s, --stack <name> or add a recognized stack manifest",
                cfg.target.display()
            ));
        }
    }

    let gcfg = governance::Config {
        repo_type,
        repo_name: repo_name.clone(),
        stack: stack.clone(),
        module_path: String::new(),
    };
    let canon_ops = governance::render_canonical_files(&gcfg)
        .map_err(|e| format!("audit: render canon: {e}"))?;
    let canon: BTreeMap<String, String> = canon_ops
        .into_iter()
        .map(|op| (op.rel_path, op.content))
        .collect();

    let canon_metadata = canon
        .get("govna/metadata.txt")
        .ok_or_else(|| "audit: rendered canon is missing govna/metadata.txt".to_string())?;
    let embedded_canon_version = metadata_value(canon_metadata, "canon_version")
        .ok_or_else(|| {
            "audit: invalid embedded canon_version: govna/metadata.txt is missing the field; restore a strict vMAJOR.MINOR.PATCH value before re-running"
                .to_string()
        })?;
    let metadata_versions = if metadata_present {
        let target_canon_version = metadata.get("canon_version").ok_or_else(|| {
            "audit: invalid target canon_version: govna/metadata.txt is missing the field; restore a strict vMAJOR.MINOR.PATCH value before re-running"
                .to_string()
        })?;
        Some(validate_metadata_versions(
            target_canon_version,
            embedded_canon_version,
        )?)
    } else {
        parse_canon_version(embedded_canon_version, "embedded")?;
        None
    };

    let canon_baseline = canon
        .get(governance::BASELINE_PATH)
        .ok_or_else(|| "audit: rendered canon is missing govna/canon-baseline.txt".to_string())?;
    let target_baseline_content =
        match std::fs::read_to_string(cfg.target.join(governance::BASELINE_PATH)) {
            Ok(content) => Some(content),
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => None,
            Err(error) => {
                return Err(format!(
                    "audit: read {}: {error}",
                    governance::BASELINE_PATH
                ));
            }
        };
    let baseline = target_baseline_content
        .as_deref()
        .map(|content| parse_baseline(content, embedded_canon_version, &flavor))
        .transpose()?;

    let coherence_failures = check_canon_coherence(&canon);
    if !coherence_failures.is_empty() {
        return Ok(write_coherence_failure_report(&coherence_failures));
    }

    let invocation = if cfg.invocation.is_empty() {
        let mut s = format!("govna audit --flavor {flavor}");
        if !stack.is_empty() {
            s.push_str(&format!(" --stack {stack:?}"));
        }
        s.push_str(&format!(" {} (programmatic)", cfg.target.display()));
        s
    } else {
        cfg.invocation.clone()
    };

    let mut report = Report {
        header: ReportHeader {
            invocation,
            canon_sha: sha.clone(),
            target: cfg.target.display().to_string(),
            flavor: flavor.clone(),
            flavor_source,
            repo_name: repo_name.clone(),
            canon_version: metadata.get("canon_version").cloned().unwrap_or_default(),
            code_stack: metadata.get("code_stack").cloned().unwrap_or_default(),
        },
        files: Vec::new(),
        emitted: None,
    };

    for relpath in canon.keys() {
        if relpath == governance::BASELINE_PATH {
            continue;
        }
        let canon_content = &canon[relpath];
        let mut fr = classify_file(
            &cfg.target,
            relpath,
            canon_content,
            baseline.as_ref(),
            &sha,
            cfg.diff_lines,
        );
        if relpath == "govna/metadata.txt" && !metadata_present {
            fr.classification = Classification::MigrationRequired;
            fr.compare_command = format!("metadata absent; migration required (canon @ {sha})");
        } else if relpath == "govna/metadata.txt"
            && let Some((target_version, embedded_version)) = metadata_versions
            && target_version < embedded_version
        {
            let target_canon_version = metadata.get("canon_version").unwrap();
            let target_content = std::fs::read_to_string(cfg.target.join(relpath))
                .map_err(|e| format!("audit: read {relpath}: {e}"))?;
            let replaced = target_content.replacen(
                &format!("canon_version = {target_canon_version}"),
                &format!("canon_version = {embedded_canon_version}"),
                1,
            );
            if replaced == *canon_content {
                fr.classification = Classification::ClearSync;
                fr.markers.clear();
                fr.compare_command = format!(
                    "stale canon_version {target_canon_version}; automatic sync to {embedded_canon_version} (canon @ {sha})"
                );
            } else if fr.classification != Classification::Preserve {
                fr.classification = Classification::Ambiguity;
                fr.compare_command = format!(
                    "stale canon_version {target_canon_version} plus other metadata divergence; whole-file review required (canon @ {sha})"
                );
            }
        }
        report.files.push(fr);
    }

    if target_baseline_content.as_deref() != Some(canon_baseline.as_str()) {
        let mut fr = FileResult::new(governance::BASELINE_PATH);
        fr.classification = Classification::MigrationRequired;
        fr.canon_ref = format!("govna @ {sha}: generated baseline manifest");
        fr.canon_content = canon_baseline.clone();
        fr.compare_command = if target_baseline_content.is_some() {
            format!("replace baseline after all routing and sync succeeds (canon @ {sha})")
        } else {
            format!("baseline absent; install after Director-reviewed migration (canon @ {sha})")
        };
        report.files.push(fr);
    }

    // Target-only evidence is merged before report emission so one path can
    // retain its strongest explanation without appearing more than once.
    let other_flavor = if flavor == "doc" { "code" } else { "doc" };
    let other_canon =
        other_flavor_canon_paths(other_flavor, &repo_name, &cfg.target).unwrap_or_default();
    let mut target_only =
        target_only_evidence(&cfg.target, &canon, baseline.as_ref(), &other_canon);

    let divergent_for_scan: Vec<FileResult> = report
        .files
        .iter()
        .filter(|f| is_divergent_class(f.classification))
        .cloned()
        .collect();
    for rel in name_referenced_target_only_files(&cfg.target, &divergent_for_scan, &canon) {
        target_only.entry(rel).or_default().name_reference = true;
    }
    for (rel, evidence) in target_only {
        let mut fr = FileResult::new(rel.clone());
        fr.classification = Classification::TargetNoCanon;
        fr.canon_ref = evidence.canon_ref(&flavor);
        fr.compare_command = evidence.routing_explanation(&cfg.target);
        if let Ok(bytes) = std::fs::read_to_string(cfg.target.join(&rel)) {
            fr.diff = unified_diff("", &bytes, &rel, cfg.diff_lines);
        }
        report.files.push(fr);
    }

    if !report.files.iter().any(is_actionable_file) {
        if cfg.json {
            let json = serde_json::to_string_pretty(&report)
                .map_err(|e| format!("audit: encode JSON report: {e}"))?;
            println!("{json}");
        } else {
            println!(
                "clean ({}); no AC emitted",
                tally_classifications(&report.files)
            );
        }
        return Ok(ExitCode::SUCCESS);
    }

    let (ac_num, reused) = emission::allocate_ac_number(&cfg.target, "audit", &sha)?;
    let stub_rel = format!("govna/ac{ac_num}-audit-{sha}.md");
    let stub_path = cfg.target.join(&stub_rel);

    if reused && stub_path.exists() {
        let unedited = emission::verify_unedited(&stub_path, AUDIT_MARKER_PREFIX)?;
        if !unedited {
            return Err(format!(
                "audit: {stub_rel} has been edited since last audit emission — to re-run, commit edits and delete the stub to regenerate, or rename the stub off the audit-{sha} slug"
            ));
        }
    }

    let validation = infer_validation_disposition(&cfg.target, &flavor);
    let stub_body = build_ac_stub(&report, ac_num, &sha, &validation);

    emission::ensure_docs_dir(&cfg.target, "audit")?;
    emission::write_with_marker(&stub_path, AUDIT_MARKER_PREFIX, &sha, &stub_body)?;

    report.emitted = Some(EmittedPaths {
        ac_stub: stub_rel.clone(),
    });

    if cfg.json {
        let json = serde_json::to_string_pretty(&report)
            .map_err(|e| format!("audit: encode JSON report: {e}"))?;
        println!("{json}");
    } else {
        println!(
            "wrote {stub_rel} ({})",
            tally_classifications(&report.files)
        );
    }
    Ok(ExitCode::SUCCESS)
}

// ── classification algorithm ────────────────────────────────────────────────

pub(crate) const EXPECTED_DIVERGENCE_PATHS: &[&str] = &["plan.md", "arch.md"];
const FORMAT_DEFINING_CANON_PATHS: &[&str] = &["govna/ac-template.md", "AGENTS.md"];

pub(crate) fn mixed_content_boundary(relpath: &str) -> Option<&'static str> {
    governance::mixed_content_boundary(relpath)
}

fn is_format_defining(relpath: &str) -> bool {
    FORMAT_DEFINING_CANON_PATHS.contains(&relpath)
}

pub(crate) fn extract_canon_zone(content: &str, boundary: &str) -> Option<String> {
    governance::extract_canon_zone(content, boundary)
}

fn classify_file(
    target: &Path,
    relpath: &str,
    canon: &str,
    baseline: Option<&Baseline>,
    sha: &str,
    diff_lines: usize,
) -> FileResult {
    let mut fr = FileResult::new(relpath);
    fr.canon_ref = format!("govna @ {sha}: templates/overlays/<flavor>/files/{relpath}");
    fr.canon_content = canon.to_string();

    let target_path = target.join(relpath);
    let target_bytes = match std::fs::read_to_string(&target_path) {
        Ok(s) => s,
        Err(_) => {
            let markers = emission::preserve_markers(target, relpath);
            if !markers.is_empty() {
                fr.classification = Classification::Match;
                fr.compare_command = format!(
                    "absent from target; preserve marker found — suppressed (canon @ {sha})"
                );
                return fr;
            }
            fr.classification = Classification::MissingTarget;
            fr.diff = unified_diff(canon, "", relpath, diff_lines);
            return fr;
        }
    };

    if target_bytes == canon {
        fr.classification = Classification::Match;
        fr.compare_command = format!("byte-equal (canon @ {sha} vs {relpath})");
        return fr;
    }

    if let Some(boundary) = mixed_content_boundary(relpath) {
        let canon_zone = extract_canon_zone(canon, boundary);
        let target_zone = extract_canon_zone(&target_bytes, boundary);
        if let (Some(cz), Some(tz)) = (&canon_zone, &target_zone) {
            fr.boundary = boundary.to_string();
            if cz == tz {
                fr.classification = Classification::Match;
                fr.compare_command =
                    format!("canon-zone byte-equal above {boundary} (canon @ {sha} vs {relpath})");
                return fr;
            }
        }
        if relpath == "govna/build-release.md" && canon_zone.is_some() && target_zone.is_none() {
            fr.diff = unified_diff(canon, &target_bytes, relpath, diff_lines);
            fr.markers = emission::preserve_markers(target, relpath);
            fr.classification = Classification::Ambiguity;
            fr.compare_command = format!(
                "target lacks registered {boundary} boundary; review the full file and migrate repository-owned practices below the boundary (canon @ {sha})"
            );
            return fr;
        }
    }

    if EXPECTED_DIVERGENCE_PATHS.contains(&relpath) {
        fr.classification = Classification::ExpectedDivergence;
        fr.compare_command = format!(
            "expected per-repo divergence (canon @ {sha} is a content stub; {relpath} carries repo-specific content)"
        );
        return fr;
    }

    fr.diff = unified_diff(canon, &target_bytes, relpath, diff_lines);
    fr.markers = emission::preserve_markers(target, relpath);
    if !fr.markers.is_empty() {
        fr.classification = Classification::Preserve;
        return fr;
    }

    if let Some(baseline) = baseline {
        fr.classification = match baseline.entries.get(relpath) {
            Some(entry) if region_hash(&target_bytes, relpath).as_deref() == Some(&entry.hash) => {
                fr.compare_command = format!(
                    "target comparison region matches stored baseline; canon changed (canon @ {sha})"
                );
                Classification::ClearSync
            }
            Some(_) => Classification::Ambiguity,
            None => {
                fr.compare_command = format!(
                    "valid baseline has no entry for {relpath}; review required (canon @ {sha})"
                );
                Classification::Ambiguity
            }
        };
    } else {
        fr.commits = git_log_n(target, relpath, 5);
        fr.classification = if !fr.commits.is_empty() {
            Classification::Ambiguity
        } else {
            Classification::ClearSync
        };
    }
    fr
}

fn is_divergent_class(c: Classification) -> bool {
    matches!(
        c,
        Classification::Preserve | Classification::Ambiguity | Classification::ClearSync
    )
}

// ── canon-coherence precondition ────────────────────────────────────────────

struct CoherenceFailure {
    rule: String,
    path: String,
    expected: String,
    preview: String,
}

struct CoherenceConformant {
    path: &'static str,
    regex: Regex,
}

struct CoherenceRule {
    name: &'static str,
    authority_path: &'static str,
    authority_regex: Regex,
    conformants: Vec<CoherenceConformant>,
}

/// Registry-driven, canon-only precondition. **Starts empty** — this ships
/// the mechanism, not any specific rule. `Regex` isn't `const`-constructible,
/// so this is a function rather than a `const`/`static` slice; a future rule
/// is added by pushing a `CoherenceRule` into the returned `Vec`.
fn coherence_rules() -> Vec<CoherenceRule> {
    vec![]
}

/// Walks `coherence_rules()` and returns failures. Empty return means canon
/// is internally coherent on all registered rules. Runs canon-only — does
/// not read the target.
fn check_canon_coherence(canon: &BTreeMap<String, String>) -> Vec<CoherenceFailure> {
    let mut failures = Vec::new();
    for rule in coherence_rules() {
        let mut sites: Vec<(&str, &Regex)> = vec![(rule.authority_path, &rule.authority_regex)];
        sites.extend(rule.conformants.iter().map(|c| (c.path, &c.regex)));
        for (path, regex) in sites {
            match canon.get(path) {
                None => failures.push(CoherenceFailure {
                    rule: rule.name.to_string(),
                    path: path.to_string(),
                    expected: regex.as_str().to_string(),
                    preview: "[file not in canon for this flavor]".to_string(),
                }),
                Some(content) if !regex.is_match(content) => failures.push(CoherenceFailure {
                    rule: rule.name.to_string(),
                    path: path.to_string(),
                    expected: regex.as_str().to_string(),
                    preview: preview_canon_content(content, 6),
                }),
                Some(_) => {}
            }
        }
    }
    failures
}

fn write_coherence_failure_report(failures: &[CoherenceFailure]) -> ExitCode {
    println!("# Canon-Coherence Precondition Failed");
    println!();
    println!(
        "This is a **govna-side** defect requiring canon reconciliation. Consumer Director's action is \"ping govna maintainer,\" not \"route a divergence.\" Drift-scan refused to emit; no files were staged in the target."
    );
    println!();
    let mut by_rule: BTreeMap<&str, Vec<&CoherenceFailure>> = BTreeMap::new();
    for f in failures {
        by_rule.entry(&f.rule).or_default().push(f);
    }
    for (rule, items) in &by_rule {
        println!("## Rule: {rule}");
        println!();
        println!("**Authoritative source:** `AGENTS.md` per the `## Governed Sections` clause.");
        println!();
        println!("**Conflicting sites:**");
        println!();
        for f in items {
            println!(
                "- `{}` — expected canonical pattern `{}` not found. First lines of canon content:",
                f.path, f.expected
            );
            println!();
            println!("  ```");
            for line in f.preview.split('\n') {
                println!("  {line}");
            }
            println!("  ```");
            println!();
        }
    }
    println!("Reconcile canon-side and re-run audit.");
    ExitCode::from(1)
}

// ── diff / git evidence ─────────────────────────────────────────────────────

fn unified_diff(canon: &str, target: &str, relpath: &str, max_lines: usize) -> String {
    if Command::new("diff").arg("--version").output().is_err() {
        return "[diff unavailable: install GNU/BSD diff and re-run]".to_string();
    }
    let mut canon_f = match tempfile_write(canon) {
        Ok(p) => p,
        Err(e) => return format!("[diff failed: create canon tmp: {e}]"),
    };
    let mut target_f = match tempfile_write(target) {
        Ok(p) => p,
        Err(e) => {
            let _ = std::fs::remove_file(&canon_f);
            return format!("[diff failed: create target tmp: {e}]");
        }
    };

    let output = Command::new("diff")
        .args([
            "-u",
            "-L",
            &format!("canon/{relpath}"),
            "-L",
            &format!("target/{relpath}"),
        ])
        .arg(&canon_f)
        .arg(&target_f)
        .output();
    let _ = std::fs::remove_file(&canon_f);
    let _ = std::fs::remove_file(&target_f);
    canon_f.clear();
    target_f.clear();

    let output = match output {
        Ok(o) => o,
        Err(e) => return format!("[diff failed: {e}]"),
    };
    if let Some(code) = output.status.code()
        && code >= 2
    {
        return format!(
            "[diff failed: exit {code}: {}]",
            String::from_utf8_lossy(&output.stdout).trim()
        );
    }
    let text = String::from_utf8_lossy(&output.stdout);
    let mut lines: Vec<&str> = text.trim_end_matches('\n').split('\n').collect();
    if max_lines > 0 && lines.len() > max_lines {
        let extra = lines.len() - max_lines;
        lines.truncate(max_lines);
        let marker = format!("[... {extra} more lines truncated ...]");
        let joined = lines.join("\n");
        return format!("{joined}\n{marker}");
    }
    lines.join("\n")
}

fn tempfile_write(content: &str) -> std::io::Result<PathBuf> {
    let path = std::env::temp_dir().join(format!(
        "govna-driftscan-{}-{}.tmp",
        std::process::id(),
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_nanos()
    ));
    std::fs::write(&path, content)?;
    Ok(path)
}

static ADOPTION_COMMIT_RE: std::sync::LazyLock<Regex> =
    std::sync::LazyLock::new(|| Regex::new(r"(?i)\bgovna\b|^govern[a-z]*\b").unwrap());

fn annotate_commit(line: &str) -> String {
    let subject = line.split_once(' ').map(|(_, s)| s).unwrap_or(line);
    if ADOPTION_COMMIT_RE.is_match(subject) {
        format!("{line} (adoption)")
    } else {
        line.to_string()
    }
}

fn git_log_n(target_root: &Path, relpath: &str, n: u32) -> Vec<String> {
    let output = Command::new("git")
        .args([
            "-C",
            &target_root.to_string_lossy(),
            "log",
            &format!("-n{n}"),
            "--follow",
            "--pretty=oneline",
            "--",
            relpath,
        ])
        .output();
    let Ok(output) = output else {
        return Vec::new();
    };
    if !output.status.success() {
        return Vec::new();
    }
    String::from_utf8_lossy(&output.stdout)
        .trim_end_matches('\n')
        .split('\n')
        .filter(|l| !l.is_empty())
        .map(annotate_commit)
        .collect()
}

fn tally_classifications(files: &[FileResult]) -> String {
    let order = [
        Classification::Match,
        Classification::ExpectedDivergence,
        Classification::Preserve,
        Classification::Ambiguity,
        Classification::ClearSync,
        Classification::MissingTarget,
        Classification::TargetNoCanon,
        Classification::MigrationRequired,
    ];
    let mut counts: BTreeMap<&str, usize> = BTreeMap::new();
    for f in files {
        *counts.entry(f.classification.as_str()).or_insert(0) += 1;
    }
    let parts: Vec<String> = order
        .iter()
        .filter_map(|c| counts.get(c.as_str()).map(|n| format!("{n} {c}")))
        .collect();
    if parts.is_empty() {
        "0 files".to_string()
    } else {
        parts.join(", ")
    }
}

fn preview_canon_content(s: &str, max_lines: usize) -> String {
    let lines: Vec<&str> = s.trim_end_matches('\n').split('\n').collect();
    if max_lines == 0 || lines.len() <= max_lines {
        return lines.join("\n");
    }
    let extra = lines.len() - max_lines;
    format!(
        "{}\n[... {extra} more lines truncated ...]",
        lines[..max_lines].join("\n")
    )
}

// ── cross-flavor orphan detection (Part B) ──────────────────────────────────

fn other_flavor_canon_paths(
    other_flavor: &str,
    repo_name: &str,
    target: &Path,
) -> Result<std::collections::HashSet<String>, String> {
    let repo_type = match other_flavor {
        "code" => RepoType::Code,
        "doc" => RepoType::Doc,
        other => return Err(format!("unknown flavor {other:?}")),
    };
    let gcfg = governance::Config {
        repo_type,
        repo_name: repo_name.to_string(),
        // Best-effort placeholder — any non-empty stack lets the renderer
        // succeed; the exact stack only affects which stack-overlay paths
        // land in the resulting set (e.g. Rust's tests/build_cli.sh).
        stack: if other_flavor == "code" {
            "Rust".to_string()
        } else {
            String::new()
        },
        module_path: String::new(),
    };
    let _ = target;
    let ops = governance::render_canonical_files(&gcfg)?;
    Ok(ops.into_iter().map(|op| op.rel_path).collect())
}

static AC_STUB_PREFIX_RE: std::sync::LazyLock<Regex> =
    std::sync::LazyLock::new(|| Regex::new(r"^ac\d+-").unwrap());

#[derive(Default)]
struct TargetOnlyEvidence {
    replacement: Option<&'static str>,
    prior_baseline: bool,
    other_flavor: bool,
    name_reference: bool,
}

impl TargetOnlyEvidence {
    fn canon_ref(&self, flavor: &str) -> String {
        if let Some(replacement) = self.replacement {
            format!("(retired canon path; replacement: {replacement})")
        } else if self.prior_baseline {
            "(present in prior canon baseline; absent from current canon)".to_string()
        } else if self.other_flavor {
            format!("(no canon path for flavor {flavor}; present in other flavor canon)")
        } else {
            format!(
                "(no canon path for flavor {flavor}; name-referenced from a divergent target file)"
            )
        }
    }

    fn routing_explanation(&self, target: &Path) -> String {
        if let Some(replacement) = self.replacement {
            if target.join(replacement).is_file() {
                format!(
                    "retired canon path replaced by {replacement}; replacement is present, so delete or preserve this retired path"
                )
            } else {
                format!(
                    "retired canon path replaced by {replacement}; replacement is missing, so restore or migrate it before deleting this retired path"
                )
            }
        } else if self.prior_baseline {
            "path existed in the prior canon baseline but has no current canon counterpart"
                .to_string()
        } else if self.other_flavor {
            "path belongs to the other flavor canon".to_string()
        } else {
            "path is name-referenced from a divergent governed file".to_string()
        }
    }
}

fn target_only_evidence(
    target: &Path,
    our_canon: &BTreeMap<String, String>,
    baseline: Option<&Baseline>,
    other_canon: &std::collections::HashSet<String>,
) -> BTreeMap<String, TargetOnlyEvidence> {
    let mut out = BTreeMap::<String, TargetOnlyEvidence>::new();

    if let Some(baseline) = baseline {
        for relpath in baseline.entries.keys() {
            if !our_canon.contains_key(relpath) && target.join(relpath).is_file() {
                out.entry(relpath.clone()).or_default().prior_baseline = true;
            }
        }
    }
    for &(relpath, replacement) in RETIRED_CANON_PATHS {
        if !our_canon.contains_key(relpath) && target.join(relpath).is_file() {
            out.entry(relpath.to_string()).or_default().replacement = Some(replacement);
        }
    }

    let docs_dir = target.join("govna");
    if let Ok(entries) = walk_dir(&docs_dir) {
        for path in entries {
            let Ok(rel) = path.strip_prefix(target) else {
                continue;
            };
            let rel = rel.to_string_lossy().replace('\\', "/");
            let base = path.file_name().unwrap_or_default().to_string_lossy();
            if base.starts_with("ac") && base.ends_with(".md") && AC_STUB_PREFIX_RE.is_match(&base)
            {
                continue;
            }
            if our_canon.contains_key(&rel) {
                continue;
            }
            if other_canon.contains(&rel) {
                out.entry(rel).or_default().other_flavor = true;
            }
        }
    }
    if let Ok(entries) = std::fs::read_dir(target) {
        for entry in entries.flatten() {
            if entry.path().is_dir() {
                continue;
            }
            let rel = entry.file_name().to_string_lossy().to_string();
            if our_canon.contains_key(&rel) {
                continue;
            }
            if other_canon.contains(&rel) {
                out.entry(rel).or_default().other_flavor = true;
            }
        }
    }
    out
}

fn walk_dir(dir: &Path) -> std::io::Result<Vec<PathBuf>> {
    let mut out = Vec::new();
    if !dir.is_dir() {
        return Ok(out);
    }
    for entry in std::fs::read_dir(dir)? {
        let entry = entry?;
        let path = entry.path();
        if path.is_dir() {
            out.extend(walk_dir(&path)?);
        } else {
            out.push(path);
        }
    }
    Ok(out)
}

// Excludes whitespace, not just the closing delimiter, so a match can never
// cross a line/sentence boundary. Real path references never contain
// whitespace; excluding only the delimiter would let a *closing* backtick
// immediately followed by unrelated punctuation (e.g. `` `Package complete.`. ``
// — very common prose, and present in govna's own real `roles.md`) get
// misread as a new match's opening delimiter, then greedily consume
// everything up to the next literal backtick anywhere in the file.
static BACKTICKED_PATH_RE: std::sync::LazyLock<Regex> =
    std::sync::LazyLock::new(|| Regex::new(r"`([./][^`\s]+)`").unwrap());
static QUOTED_PATH_RE: std::sync::LazyLock<Regex> =
    std::sync::LazyLock::new(|| Regex::new(r#""([./][^"\s]+)""#).unwrap());
static GO_RUN_OR_EXEC_TAIL_RE: std::sync::LazyLock<Regex> =
    std::sync::LazyLock::new(|| Regex::new(r"(?m)(?:go run|exec)\s+(.+)$").unwrap());

fn extract_path_references(content: &str) -> Vec<String> {
    let mut refs = Vec::new();
    for caps in BACKTICKED_PATH_RE.captures_iter(content) {
        refs.push(caps[1].to_string());
    }
    for caps in QUOTED_PATH_RE.captures_iter(content) {
        refs.push(caps[1].to_string());
    }
    for caps in GO_RUN_OR_EXEC_TAIL_RE.captures_iter(content) {
        for tok in caps[1].split_whitespace() {
            if tok.starts_with("./") || tok.starts_with('/') {
                refs.push(tok.to_string());
            }
        }
    }
    refs
}

/// Lexically resolves `..`/`.` components, matching Go's `filepath.Join`,
/// which cleans internally — unlike Rust's `Path::join`, which does not.
/// Without this, a reference like `../scripts.sh` from a file inside
/// `govna/` would resolve to the literal (uncleaned, wrong) `govna/../scripts.sh`.
fn clean_path(p: &str) -> String {
    let mut parts: Vec<&str> = Vec::new();
    for comp in p.split('/') {
        match comp {
            "" | "." => continue,
            ".." => {
                if parts.last().is_some_and(|last| *last != "..") {
                    parts.pop();
                } else {
                    parts.push("..");
                }
            }
            other => parts.push(other),
        }
    }
    parts.join("/")
}

fn normalize_ref_path(reference: &str, referer_rel: &str) -> String {
    let referer_dir = Path::new(referer_rel).parent().unwrap_or(Path::new(""));
    let referer_dir = referer_dir.to_string_lossy();
    if let Some(stripped) = reference.strip_prefix('/') {
        clean_path(stripped)
    } else if let Some(stripped) = reference.strip_prefix("./") {
        if referer_dir.is_empty() {
            clean_path(stripped)
        } else {
            clean_path(&format!("{referer_dir}/{stripped}"))
        }
    } else if referer_dir.is_empty() {
        clean_path(reference)
    } else {
        clean_path(&format!("{referer_dir}/{reference}"))
    }
}

fn name_referenced_target_only_files(
    target: &Path,
    divergent: &[FileResult],
    our_canon: &BTreeMap<String, String>,
) -> Vec<String> {
    let mut found = std::collections::HashSet::new();
    for f in divergent {
        let Ok(content) = std::fs::read_to_string(target.join(&f.relpath)) else {
            continue;
        };
        for reference in extract_path_references(&content) {
            let resolved = normalize_ref_path(&reference, &f.relpath);
            if resolved.is_empty() || resolved == f.relpath {
                continue;
            }
            let abs_path = target.join(&resolved);
            if !abs_path.is_file() {
                continue;
            }
            if our_canon.contains_key(&resolved) {
                continue;
            }
            found.insert(resolved);
        }
    }
    let mut out: Vec<String> = found.into_iter().collect();
    out.sort();
    out
}

// ── AC-stub emission ─────────────────────────────────────────────────────────

fn build_ac_stub(
    r: &Report,
    ac_num: u32,
    canon_version: &str,
    validation: &ValidationDisposition,
) -> String {
    let mut sync_entries: Vec<&FileResult> = Vec::new();
    let mut migration_entries: Vec<&FileResult> = Vec::new();
    let mut oos_entries: Vec<&FileResult> = Vec::new();
    let mut review_entries: Vec<&FileResult> = Vec::new();
    let mut format_defining_forced: Vec<&FileResult> = Vec::new();

    for f in &r.files {
        if is_format_defining(&f.relpath)
            && f.classification != Classification::Match
            && f.classification != Classification::ExpectedDivergence
        {
            sync_entries.push(f);
            if f.classification != Classification::ClearSync
                && f.classification != Classification::MissingTarget
            {
                format_defining_forced.push(f);
            }
            continue;
        }
        match f.classification {
            Classification::ClearSync | Classification::MissingTarget => sync_entries.push(f),
            Classification::MigrationRequired => migration_entries.push(f),
            Classification::Preserve | Classification::ExpectedDivergence => oos_entries.push(f),
            Classification::Ambiguity | Classification::TargetNoCanon => {
                review_entries.push(f);
            }
            Classification::Match => {}
        }
    }
    let baseline_migration = migration_entries
        .iter()
        .copied()
        .find(|f| f.relpath == governance::BASELINE_PATH);
    let unresolved_validation = baseline_migration.is_some() && validation.is_unresolved();
    let has_routing_decisions = !review_entries.is_empty() || unresolved_validation;

    let mut b = String::new();
    b.push_str(&format!(
        "# AC{ac_num} Audit Adoption from govna {canon_version}\n\n"
    ));
    if migration_entries.is_empty() {
        b.push_str(&format!(
            "Adopt {} canon-owned changes from govna {canon_version}; {} {} require routing decisions.\n\n",
            sync_entries.len(),
            review_entries.len(),
            count_noun(review_entries.len(), "entry", "entries")
        ));
    } else {
        b.push_str(&format!(
            "Adopt {} canon-owned changes from govna {canon_version}; {} {} and {} {} require routing decisions.\n\n",
            sync_entries.len(),
            migration_entries.len(),
            count_noun(migration_entries.len(), "migration item", "migration items"),
            review_entries.len(),
            count_noun(review_entries.len(), "entry", "entries")
        ));
    }

    b.push_str("## Summary\n\n");
    b.push_str(&format!(
        "Sync this repo to govna @ {canon_version} canon as part of the recurring audit cycle. Audit surfaced {}. Use `govna render` to render canon and standard `diff -ru` to inspect per-file changes (see AGENTS.md `### Audit Adoption`).\n\n",
        tally_classifications(&r.files)
    ));
    if has_routing_decisions {
        b.push_str(ROUTING_RESOLUTION_REMINDER);
        b.push_str("\n\n");
    }
    if r.header.flavor == "code" {
        b.push_str(REACHABILITY_HEADER_REMINDER);
        b.push_str("\n\n");
    }

    if !format_defining_forced.is_empty() {
        b.push_str("### Format-defining file routing\n\n");
        b.push_str("The following files were routed to `## In Scope` as sync items because they are in the format-defining registry; the raw classification (preserve / ambiguity / etc.) is overridden because the file's content defines the form the AC instantiates:\n\n");
        for f in &format_defining_forced {
            b.push_str(&format!(
                "- `{}` — raw classification: {}; forced to sync.\n",
                f.relpath, f.classification
            ));
        }
        b.push('\n');
    }
    b.push_str("### Routing Decisions\n\n");
    if !has_routing_decisions {
        b.push_str("`None` — no ambiguities or target-only files surfaced.\n");
    } else {
        for (i, f) in review_entries.iter().enumerate() {
            match f.classification {
                Classification::Ambiguity if f.relpath == "govna/metadata.txt" => {
                    b.push_str(&format!(
                        "{}. **`{}`**: diverges from canon, most likely a stale `canon_version` marker — sync to update it; if `repo_type`/`code_stack` were also hand-edited here, preserve those and sync `canon_version` separately.\n",
                        i + 1,
                        f.relpath
                    ))
                }
                Classification::Ambiguity => b.push_str(&format!(
                    "{}. **`{}`**: diverges from canon — sync to canon, preserve as repo-owned, or pin via preserve marker?\n",
                    i + 1,
                    f.relpath
                )),
                Classification::TargetNoCanon => b.push_str(&format!(
                    "{}. **`{}`**: file exists in target but not in canon for this flavor — keep, delete, or migrate to an explicitly named destination? {}.\n",
                    i + 1,
                    f.relpath,
                    f.compare_command
                )),
                _ => {}
            }
        }
        if unresolved_validation {
            let item = review_entries.len() + 1;
            if r.header.flavor == "code" {
                b.push_str(&format!(
                    "{item}. **Validation disposition**: proposed `./build.sh` based on the CODE flavor. Director must confirm it or override it in chat when this repository declares a different validation command; leave this emitted stub unchanged.\n"
                ));
            } else {
                b.push_str(&format!(
                    "{item}. **Validation disposition**: proposed `Not applicable` because standard DOC canon defines no automated content-validation command. Director must confirm it with repository evidence or override it in chat when this repository declares a validation command; leave this emitted stub unchanged.\n"
                ));
            }
        }
    }
    b.push('\n');

    if baseline_migration.is_some() && !unresolved_validation {
        b.push_str("### Validation disposition\n\n");
        b.push_str(validation.evidence());
        b.push_str("\n\n");
    }

    b.push_str("## In Scope\n\n");
    if sync_entries.is_empty() && migration_entries.is_empty() {
        b.push_str("No sync items.\n");
    } else {
        if !sync_entries.is_empty() {
            b.push_str("Sync to canon:\n\n");
            for f in &sync_entries {
                let suffix = if is_format_defining(&f.relpath) {
                    " (format-defining)"
                } else {
                    ""
                };
                b.push_str(&format!(
                    "- `{}` — {}{}\n",
                    f.relpath, f.classification, suffix
                ));
            }
        }
        if !migration_entries.is_empty() {
            b.push_str("Migration items:\n\n");
            for f in &migration_entries {
                b.push_str(&format!("- `{}` — migration-required\n", f.relpath));
            }
        }
    }
    b.push('\n');

    b.push_str("## Out Of Scope\n\n");
    if oos_entries.is_empty() {
        b.push_str("No preserved or expected-divergence entries.\n");
    } else {
        for f in &oos_entries {
            b.push_str(&format!("- `{}` — {}", f.relpath, f.classification));
            if !f.markers.is_empty() {
                b.push_str(&format!(" (markers: {})", f.markers.join("; ")));
            }
            b.push('\n');
        }
    }
    b.push('\n');

    b.push_str("## Migration findings\n\n");
    if migration_entries.is_empty() {
        b.push_str("`None`.\n");
    } else {
        for f in &migration_entries {
            b.push_str(&format!("- `{}` — {}\n", f.relpath, f.compare_command));
        }
    }
    b.push('\n');

    b.push_str("## Acceptance Tests\n\n");
    b.push_str(
        "**AT1** [Automated] [Pre-release gate] — Canon-coherence precondition passed (emission was not blocked).\n\n",
    );
    let mut at_num = 2;
    for f in &sync_entries {
        if !f.boundary.is_empty() {
            b.push_str(&format!(
                "**AT{at_num}** [Automated] [Pre-release gate] — canon zone of `{}` (above `{}`) matches canon byte-for-byte after sync.\n\n",
                f.relpath, f.boundary
            ));
        } else {
            b.push_str(&format!(
                "**AT{at_num}** [Automated] [Pre-release gate] — `{}` matches canon byte-for-byte after sync.\n\n",
                f.relpath
            ));
        }
        at_num += 1;
    }
    for f in &migration_entries {
        if f.relpath != governance::BASELINE_PATH {
            b.push_str(&format!(
                "**AT{at_num}** [Automated] [Pre-release gate] — `{}` is explicitly migrated to the metadata contract before audit adoption completes.\n\n",
                f.relpath
            ));
            at_num += 1;
        }
    }
    if has_routing_decisions {
        b.push_str(&format!(
            "**AT{at_num}** [Manual] [Pre-release gate] — Director resolved every `### Routing Decisions` item listed above, and the resolution is reflected in the repo.\n\n"
        ));
        at_num += 1;
        b.push_str(&format!(
            "**AT{at_num}** [Automated] [Pre-release gate] — Every resolved routing outcome is verified conditionally: sync targets match their rendered canon region; migration sources are absent unless explicitly preserved; canon-backed migration destinations match rendered canon; repo-owned migration destinations satisfy the Director's stated result; delete targets are absent; preserve targets remain and `CHANGELOG.md` carries the required preserve marker.\n\n"
        ));
        at_num += 1;
        for f in &review_entries {
            if !f.boundary.is_empty() {
                b.push_str(&format!(
                    "**AT{at_num}** [Automated] [Pre-release gate] — When `{}` is resolved as sync, its canon zone above `{}` matches canon byte-for-byte; otherwise verify its resolved preserve outcome.\n\n",
                    f.relpath, f.boundary
                ));
                at_num += 1;
            }
        }
    }
    if baseline_migration.is_some() {
        b.push_str(&format!(
            "**AT{at_num}** [Automated] [Pre-release gate] — {}\n\n",
            validation.acceptance_test()
        ));
        at_num += 1;
    }
    if !sync_entries.is_empty() || !migration_entries.is_empty() || !review_entries.is_empty() {
        b.push_str(&format!(
            "**AT{at_num}** [Automated] [Pre-release gate] — For each file listed under `## In Scope` except `govna/canon-baseline.txt`, each routing target resolved as sync, and each canon-backed migration destination, `govna render` (per the recipe in `## Summary`) plus `diff -ru` against rendered canon shows no remaining diff — scoped to the canon zone above the boundary heading for any file whose AT above names a boundary.\n\n"
        ));
        at_num += 1;
    }
    if baseline_migration.is_some() {
        b.push_str(&format!(
            "**AT{at_num}** [Automated] [Pre-release gate] — After every other applicable automated AT, resolved routing outcome, and the resolved validation disposition passes, `govna/canon-baseline.txt` is installed or replaced from the same scratch render and verified as the final audit-adoption step.\n\n"
        ));
    }

    b.push_str("## Status\n\n");
    b.push_str(
        "`PENDING` — audit emission; awaiting Director review and implementation authorization.\n",
    );
    b
}

fn count_noun<'a>(count: usize, singular: &'a str, plural: &'a str) -> &'a str {
    if count == 1 { singular } else { plural }
}

#[derive(Debug, Clone, PartialEq, Eq)]
enum ValidationDisposition {
    InferredBuild,
    InferredNotApplicable,
    UnresolvedCode,
    UnresolvedDoc,
}

impl ValidationDisposition {
    fn is_unresolved(&self) -> bool {
        matches!(self, Self::UnresolvedCode | Self::UnresolvedDoc)
    }

    fn evidence(&self) -> &'static str {
        match self {
            Self::InferredBuild => {
                "`./build.sh` — inferred from target `AGENTS.md`: exactly one `Run` declaration names it as the first validation command, exactly one `Use` declaration names it for repository-wide validation, and root `build.sh` is a regular file. No Director confirmation is required."
            }
            Self::InferredNotApplicable => {
                "`Not applicable` — inferred from target `govna/release.md`, which contains the exact canon declaration that DOC repositories define no automated content-validation command; target `AGENTS.md` contains no recognized positive validation declaration. No Director confirmation is required."
            }
            Self::UnresolvedCode | Self::UnresolvedDoc => "",
        }
    }

    fn acceptance_test(&self) -> &'static str {
        match self {
            Self::InferredBuild => {
                "The inferred `./build.sh` validation disposition is satisfied after all selected sync, migration, and deletion work: `./build.sh` succeeds before baseline installation."
            }
            Self::InferredNotApplicable => {
                "The inferred `Not applicable` validation disposition remains supported after all selected sync, migration, and deletion work: `govna/release.md` declares that DOC repositories define no automated content-validation command and no recognized positive AGENTS.md declaration exists."
            }
            Self::UnresolvedCode | Self::UnresolvedDoc => {
                "The Director-confirmed validation disposition is satisfied after all selected sync, migration, and deletion work: the resolved command succeeds, or `Not applicable` cites repository evidence that no automated content-validation command is declared."
            }
        }
    }
}

fn infer_validation_disposition(target: &Path, flavor: &str) -> ValidationDisposition {
    const DOC_NO_VALIDATION: &str = "DOC repositories do not need a compiler toolchain for release preparation or release orchestration and define no automated content-validation command.";

    let agents = std::fs::read_to_string(target.join("AGENTS.md")).unwrap_or_default();
    let run_pattern =
        Regex::new(r"(?m)^- Run `([^`\n]+)` as the first validation command(?:\s|\.|$)[^\n]*$")
            .unwrap();
    let use_pattern =
        Regex::new(r"(?m)^- Use `([^`\n]+)` for repository-wide [^\n]*validation[^\n]*$").unwrap();
    let run_commands: Vec<_> = run_pattern
        .captures_iter(&agents)
        .map(|captures| captures[1].to_string())
        .collect();
    let use_commands: Vec<_> = use_pattern
        .captures_iter(&agents)
        .map(|captures| captures[1].to_string())
        .collect();
    let release = std::fs::read_to_string(target.join("govna/release.md")).unwrap_or_default();
    let negative_count = release.matches(DOC_NO_VALIDATION).count();
    let build_is_regular = std::fs::symlink_metadata(target.join("build.sh"))
        .map(|metadata| metadata.file_type().is_file())
        .unwrap_or(false);

    if flavor == "code" {
        if run_commands == ["./build.sh"]
            && use_commands == ["./build.sh"]
            && negative_count == 0
            && build_is_regular
        {
            ValidationDisposition::InferredBuild
        } else {
            ValidationDisposition::UnresolvedCode
        }
    } else if run_commands.is_empty() && use_commands.is_empty() && negative_count == 1 {
        ValidationDisposition::InferredNotApplicable
    } else {
        ValidationDisposition::UnresolvedDoc
    }
}

fn is_actionable_file(file: &FileResult) -> bool {
    if is_format_defining(&file.relpath) {
        return file.classification != Classification::Match
            && file.classification != Classification::ExpectedDivergence;
    }
    matches!(
        file.classification,
        Classification::Ambiguity
            | Classification::ClearSync
            | Classification::MissingTarget
            | Classification::TargetNoCanon
            | Classification::MigrationRequired
    )
}

#[cfg(test)]
mod tests {
    use super::{parse_canon_version, validate_metadata_versions};

    #[test]
    fn canon_version_parser_rejects_non_strict_embedded_values() {
        for value in ["0.3.0", "v1.2", "v01.2.3", "v1.2.3-beta"] {
            let error = parse_canon_version(value, "embedded").unwrap_err();
            assert!(error.contains("strict vMAJOR.MINOR.PATCH"), "{error}");
        }
    }

    #[test]
    fn metadata_version_validation_rejects_newer_target() {
        let error = validate_metadata_versions("v1.0.0", "v0.4.0").unwrap_err();
        assert!(error.contains("upgrade govna"), "{error}");
        assert!(error.contains("not downgraded"), "{error}");
    }
}
