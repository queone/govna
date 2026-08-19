use rust_embed::RustEmbed;
use std::collections::BTreeMap;

/// Embedded canon version, independent of `PROGRAM_VERSION`. Bumped by
/// hand during release prep until `build.sh prep` automates it.
pub const CANON_VERSION: &str = "0.22.1";

#[derive(RustEmbed)]
#[folder = "templates/"]
pub struct Templates;

/// Reads an embedded template file and substitutes every placeholder key
/// (sorted) with its value.
pub fn read_and_render(
    path: &str,
    placeholders: &BTreeMap<&'static str, String>,
) -> Result<String, String> {
    let file =
        Templates::get(path).ok_or_else(|| format!("read template file {path}: not found"))?;
    let mut out = String::from_utf8(file.data.into_owned())
        .map_err(|e| format!("read template file {path}: invalid utf8: {e}"))?;
    for (key, value) in placeholders {
        out = out.replace(key, value);
    }
    Ok(out)
}

/// Reads an embedded file verbatim, no placeholder substitution. Used for
/// stack-ignores/stack-guidelines fragments, which are stitched into an
/// already-rendered file rather than rendered standalone.
pub fn read_raw(path: &str) -> Option<String> {
    let file = Templates::get(path)?;
    String::from_utf8(file.data.into_owned()).ok()
}
