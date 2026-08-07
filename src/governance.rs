use crate::templates::{self, Templates};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use std::path::Path;

#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum RepoType {
    Code,
    Doc,
}

pub struct Config {
    pub repo_type: RepoType,
    pub repo_name: String,
    /// Raw stack string (whatever the user passed, or whatever `infer_stack`
    /// returned) — canonicalized inside `render_canonical_files`. Empty for
    /// DOC.
    pub stack: String,
    /// Empty unless CODE flavor with the Go stack.
    pub module_path: String,
}

pub struct WriteOp {
    pub rel_path: String,
    pub content: String,
}

pub const BASELINE_PATH: &str = "govna/canon-baseline.txt";
const BASELINE_SCHEMA: &str = "govna-canon-baseline-v1";

pub fn mixed_content_boundary(relpath: &str) -> Option<&'static str> {
    match relpath {
        "AGENTS.md" => Some("## Project Rules"),
        "govna/build-release.md"
        | "govna/development-guidelines.md"
        | "govna/editing-guidelines.md" => Some("## Project Practices"),
        _ => None,
    }
}

pub fn extract_canon_zone(content: &str, boundary: &str) -> Option<String> {
    let mut acc = String::new();
    for line in content.split_inclusive('\n') {
        let trimmed = line.trim_end_matches(['\n', '\r']);
        if trimmed == boundary {
            return Some(acc);
        }
        acc.push_str(line);
    }
    None
}

fn sha256(content: &str) -> String {
    Sha256::digest(content.as_bytes())
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect()
}

fn render_baseline(canon: &BTreeMap<String, String>) -> Result<String, String> {
    let mut baseline = format!(
        "{BASELINE_SCHEMA}\ncanon_version = v{}\n",
        templates::CANON_VERSION
    );
    for (relpath, content) in canon {
        let (scope, region) = if let Some(boundary) = mixed_content_boundary(relpath) {
            let region = extract_canon_zone(content, boundary).ok_or_else(|| {
                format!("render baseline: {relpath} is missing registered boundary {boundary:?}")
            })?;
            (format!("before:{boundary}"), region)
        } else {
            ("full".to_string(), content.clone())
        };
        baseline.push_str(&format!("{relpath}\t{scope}\t{}\n", sha256(&region)));
    }
    Ok(baseline)
}

/// Produces the full set of canon files a render would write, keyed by
/// repo-relative slash path via `WriteOp`. Pure, in-memory — no filesystem
/// writes. Output-precedence rule: ops are queued in
/// order (base -> flavor overlay -> stack overlay) and a later op silently
/// wins over an earlier one at the same rel path. Concretely, the DOC overlay
/// ships its own `AGENTS.md.tmpl`, so DOC's real output is the overlay's
/// AGENTS.md, not base's; CODE has no such override file, so CODE keeps base's.
pub fn render_canonical_files(cfg: &Config) -> Result<Vec<WriteOp>, String> {
    let canonical_stack = if cfg.repo_type == RepoType::Code {
        canonical_stack(&cfg.stack).map(|s| s.to_string()).ok_or_else(|| {
            format!(
                "unsupported CODE stack {:?}: use Go, Rust, Swift, Terraform, Node, Python, or Java",
                cfg.stack
            )
        })?
    } else {
        cfg.stack.trim().to_string()
    };

    let module_path = if cfg.module_path.is_empty() {
        cfg.repo_name.clone()
    } else {
        cfg.module_path.clone()
    };

    let mut placeholders: BTreeMap<&'static str, String> = BTreeMap::new();
    placeholders.insert("{{REPO_NAME}}", cfg.repo_name.clone());
    placeholders.insert(
        "{{STACK_OR_PLATFORM}}",
        value_or_default(&canonical_stack, "TBD"),
    );
    placeholders.insert("{{MODULE_PATH}}", module_path);
    placeholders.insert(
        "{{CANON_VERSION}}",
        format!("v{}", templates::CANON_VERSION),
    );
    placeholders.insert("{{CODE_STACK}}", value_or_default(&canonical_stack, "TBD"));
    let stack_build_release = if canonical_stack.eq_ignore_ascii_case("rust") {
        templates::read_raw("stack-build-release/rust.md")
            .ok_or_else(|| {
                "compose stack build/release guidance: Rust block not found".to_string()
            })?
            .trim()
            .to_string()
    } else {
        String::new()
    };
    placeholders.insert("{{STACK_BUILD_RELEASE_GUIDANCE}}", stack_build_release);

    let mut out: BTreeMap<String, String> = BTreeMap::new();

    let base_content = templates::read_and_render("base/AGENTS.md", &placeholders)?;
    out.insert("AGENTS.md".to_string(), base_content);

    let overlay_prefix = match cfg.repo_type {
        RepoType::Code => "overlays/code/files/",
        RepoType::Doc => "overlays/doc/files/",
    };
    for path in Templates::iter() {
        let path_str: &str = &path;
        if let Some(rel) = path_str.strip_prefix(overlay_prefix) {
            let target_rel = rel.strip_suffix(".tmpl").unwrap_or(rel);
            let mut content = templates::read_and_render(path_str, &placeholders)?;
            if target_rel == ".gitignore"
                && let Some(block) = stack_ignore_block(&canonical_stack)
            {
                content.push('\n');
                content.push_str(&block);
            }
            if target_rel == "govna/development-guidelines.md"
                && let Some(block) = stack_guideline_block(&canonical_stack)
            {
                content = insert_stack_guidelines(&content, &block)?;
            }
            out.insert(target_rel.to_string(), content);
        }
    }

    if cfg.repo_type == RepoType::Code && !canonical_stack.is_empty() {
        let stack_prefix = format!("overlays/code/stacks/{}/", canonical_stack.to_lowercase());
        for path in Templates::iter() {
            let path_str: &str = &path;
            if let Some(rel) = path_str.strip_prefix(stack_prefix.as_str()) {
                let target_rel = rel.strip_suffix(".tmpl").unwrap_or(rel);
                let content = templates::read_and_render(path_str, &placeholders)?;
                out.insert(target_rel.to_string(), content);
            }
        }
    }

    let baseline = render_baseline(&out)?;
    out.insert(BASELINE_PATH.to_string(), baseline);

    Ok(out
        .into_iter()
        .map(|(rel_path, content)| WriteOp { rel_path, content })
        .collect())
}

/// Flavor resolution, read from `dir` (the caller's cwd, not the render
/// target). `govna/metadata.txt` wins when present; otherwise a fallback
/// heuristic (Jekyll marker vs. a strong CODE manifest).
pub fn detect_flavor(dir: &Path) -> Result<RepoType, String> {
    Ok(detect_flavor_with_source(dir)?.0)
}

/// Same resolution as `detect_flavor`, also reporting which tier resolved
/// it (`"metadata"` or `"fallback"`) — public surface for `audit`,
/// which reports the source in its emitted report header.
pub fn detect_flavor_with_source(dir: &Path) -> Result<(RepoType, &'static str), String> {
    if let Some(metadata) = read_repo_metadata(dir)? {
        let repo_type = metadata
            .get("repo_type")
            .ok_or_else(|| "invalid govna/metadata.txt: missing repo_type".to_string())?;
        return match repo_type.as_str() {
            "CODE" => Ok((RepoType::Code, "metadata")),
            "DOC" => Ok((RepoType::Doc, "metadata")),
            other => Err(format!(
                "invalid govna/metadata.txt: unknown repo_type {other:?}"
            )),
        };
    }
    detect_fallback_flavor(dir).map(|t| (t, "fallback"))
}

/// Reads and parses `<dir>/govna/metadata.txt`. Public surface for
/// `audit`, which needs the full parsed record (canon_version,
/// code_stack) for its report header, not just the repo_type `detect_flavor` uses.
pub fn read_repo_metadata(dir: &Path) -> Result<Option<BTreeMap<String, String>>, String> {
    let path = dir.join("govna").join("metadata.txt");
    let content = match std::fs::read_to_string(&path) {
        Ok(c) => c,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(e) => return Err(format!("read {}: {e}", path.display())),
    };
    if !content.ends_with('\n') {
        return Err(format!(
            "invalid {}: require a final newline",
            path.display()
        ));
    }
    let mut values = BTreeMap::new();
    for line in content.trim_end_matches('\n').split('\n') {
        let (key, value) = line.split_once(" = ").ok_or_else(|| {
            format!(
                "invalid {}: each line must use `key = value`",
                path.display()
            )
        })?;
        if key.is_empty() || value.is_empty() {
            return Err(format!(
                "invalid {}: each line must use `key = value`",
                path.display()
            ));
        }
        values.insert(key.to_string(), value.to_string());
    }
    Ok(Some(values))
}

fn detect_fallback_flavor(dir: &Path) -> Result<RepoType, String> {
    let has_jekyll = dir.join("_config.yml").exists();
    let has_strong_code = [
        "go.mod",
        "Cargo.toml",
        "Package.swift",
        ".terraform.lock.hcl",
    ]
    .iter()
    .any(|name| dir.join(name).exists());
    match (has_strong_code, has_jekyll) {
        (true, true) => Err(
            "conflicting flavor signals: target has _config.yml and a strong CODE manifest; pass --flavor code or --flavor doc"
                .to_string(),
        ),
        (true, false) => Ok(RepoType::Code),
        (false, true) => Ok(RepoType::Doc),
        (false, false) => {
            Err("could not infer flavor: add govna/metadata.txt, pass --flavor code|doc, or add a recognized flavor manifest".to_string())
        }
    }
}

/// Stack inference, read from `dir`. Exact manifest-check precedence:
/// Go/Terraform/Rust checked first in that literal order, then Swift, then
/// the remaining manifests, then a `*.tf` glob fallback.
pub fn infer_stack(dir: &Path) -> Option<&'static str> {
    if dir.join("go.mod").exists() {
        return Some("Go");
    }
    if dir.join(".terraform.lock.hcl").exists() {
        return Some("Terraform");
    }
    if dir.join("Cargo.toml").exists() {
        return Some("Rust");
    }
    if dir.join("Package.swift").exists() {
        return Some("Swift");
    }
    if dir.join("package.json").exists() {
        return Some("Node");
    }
    if dir.join("pyproject.toml").exists() {
        return Some("Python");
    }
    if dir.join("pom.xml").exists() || dir.join("build.gradle").exists() {
        return Some("Java");
    }
    if let Ok(entries) = std::fs::read_dir(dir) {
        for entry in entries.flatten() {
            if entry.path().extension().map(|e| e == "tf").unwrap_or(false) {
                return Some("Terraform");
            }
        }
    }
    None
}

/// Public surface for audit and render command handling.
pub fn canonical_stack(stack: &str) -> Option<&'static str> {
    match stack.trim().to_lowercase().as_str() {
        "go" => Some("Go"),
        "rust" => Some("Rust"),
        "swift" => Some("Swift"),
        "terraform" => Some("Terraform"),
        "node" => Some("Node"),
        "python" => Some("Python"),
        "java" => Some("Java"),
        _ => None,
    }
}

/// Reads the Go module path from `<dir>/go.mod`, or `None` if absent.
pub fn read_module_path(dir: &Path) -> Option<String> {
    let content = std::fs::read_to_string(dir.join("go.mod")).ok()?;
    for line in content.split('\n') {
        let line = line.trim();
        if let Some(rest) = line.strip_prefix("module ") {
            return Some(rest.trim().to_string());
        }
    }
    None
}

fn value_or_default(value: &str, fallback: &str) -> String {
    let trimmed = value.trim();
    if trimmed.is_empty() {
        fallback.to_string()
    } else {
        trimmed.to_string()
    }
}

fn stack_ignore_block(stack: &str) -> Option<String> {
    let file = if stack.eq_ignore_ascii_case("go") {
        "stack-ignores/go.txt"
    } else if stack.eq_ignore_ascii_case("rust") {
        "stack-ignores/rust.txt"
    } else if stack.eq_ignore_ascii_case("swift") {
        "stack-ignores/swift.txt"
    } else if stack.eq_ignore_ascii_case("terraform") {
        "stack-ignores/terraform.txt"
    } else {
        return None;
    };
    templates::read_raw(file)
}

/// Stack-specific development guidance to insert above the consumer-owned
/// `## Project Practices` boundary.
fn stack_guideline_block(stack: &str) -> Option<String> {
    let file = if stack.eq_ignore_ascii_case("go") {
        "stack-guidelines/go.md"
    } else if stack.eq_ignore_ascii_case("rust") {
        "stack-guidelines/rust.md"
    } else if stack.eq_ignore_ascii_case("swift") {
        "stack-guidelines/swift.md"
    } else {
        return None;
    };
    templates::read_raw(file).map(|s| s.trim().to_string())
}

fn insert_stack_guidelines(content: &str, block: &str) -> Result<String, String> {
    const BOUNDARY: &str = "## Project Practices";
    let marker = format!("\n{BOUNDARY}\n");
    let marker_index = content
        .find(&marker)
        .ok_or_else(|| format!("compose stack guidelines: {BOUNDARY} boundary not found"))?;
    let split_at = marker_index + 1; // skip the leading '\n' (1 byte), land on '#'
    let prefix = content[..split_at].trim_end_matches('\n');
    let suffix = &content[split_at..];
    Ok(format!("{prefix}\n\n{block}\n\n{suffix}"))
}

/// Resolves a render target's repo name: for CODE with a module path,
/// the module path's final slash-separated component (module paths always
/// use `/`, regardless of host OS — matching Go's `path.Base`, not
/// `filepath.Base`); otherwise the basename of `cwd` (never the render
/// target's basename — see `render`'s doc comment).
pub fn resolve_repo_name(cwd: &Path, module_path: &str) -> String {
    if !module_path.is_empty() {
        module_path
            .rsplit('/')
            .next()
            .unwrap_or(module_path)
            .to_string()
    } else {
        cwd.file_name()
            .map(|n| n.to_string_lossy().to_string())
            .unwrap_or_default()
    }
}
