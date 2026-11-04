use std::path::Path;

pub const MANIFEST_ENV: &str = "LAYERX_BRIDGE_CAPTURE_MANIFEST";
pub const PIN_ENV: &str = "LAYERX_BRIDGE_CAPTURE_MANIFEST_SHA256";
pub const REQUIRED_ENV: &str = "LAYERX_BRIDGE_CAPTURE_REQUIRED";
pub const MISSING: &str = "missing LAYERX_BRIDGE_CAPTURE_MANIFEST and independent LAYERX_BRIDGE_CAPTURE_MANIFEST_SHA256; genuine captured production-boundary qualification is NOT PROVEN";

#[derive(Debug, Eq, PartialEq)]
pub enum Disposition<'a> {
    MissingPrerequisite(&'static str),
    Required { manifest: &'a Path, pin: &'a str },
}

pub fn prerequisite<'a>(
    manifest: Option<&'a Path>,
    pin: Option<&'a str>,
    required: bool,
) -> Result<Disposition<'a>, &'static str> {
    match (manifest, pin) {
        (None, None) if !required => Ok(Disposition::MissingPrerequisite(MISSING)),
        (None, _) => Err(
            "missing LAYERX_BRIDGE_CAPTURE_MANIFEST; genuine captured boundary corpus is required",
        ),
        (_, None) => Err("missing independent LAYERX_BRIDGE_CAPTURE_MANIFEST_SHA256 pin"),
        (Some(manifest), Some(pin)) => {
            if !manifest.is_absolute()
                || pin.len() != 64
                || !pin.bytes().all(|byte| byte.is_ascii_hexdigit())
            {
                return Err("capture manifest path or independent SHA256 pin refused");
            }
            Ok(Disposition::Required { manifest, pin })
        }
    }
}
