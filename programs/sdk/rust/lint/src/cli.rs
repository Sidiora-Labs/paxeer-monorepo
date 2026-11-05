//! ABI argument and project metadata handling used by the lint command.

use std::fs;
use std::path::Path;

use crate::{lint_project_for_abi, DeterminismViolation};
use serde::de::{self, Deserialize, Deserializer, IgnoredAny, MapAccess, Visitor};

/// Lints a project using its recorded ABI, refusing a conflicting command argument.
#[must_use]
pub fn project_violations(
    project: &Path,
    artifact: Option<&Path>,
    requested_version: Option<&str>,
) -> Vec<DeterminismViolation> {
    let recorded = match recorded_abi_version(project) {
        Ok(version) => version,
        Err(reason) => return vec![metadata_refusal(project, reason)],
    };
    if let Some(requested) = requested_version {
        let Some(requested) = parse_version(requested) else {
            return vec![metadata_refusal(
                project,
                "ABI version must be exactly 1, 2, 3, 4 or 5",
            )];
        };
        if requested != recorded {
            return vec![metadata_refusal(
                project,
                "requested ABI version differs from layerx-program.json",
            )];
        }
    }
    lint_project_for_abi(project, artifact, recorded)
}

struct ProgramManifestAbi {
    abi_version: u16,
}

impl<'de> Deserialize<'de> for ProgramManifestAbi {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        struct ManifestVisitor;

        impl<'de> Visitor<'de> for ManifestVisitor {
            type Value = ProgramManifestAbi;

            fn expecting(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
                formatter.write_str("a program manifest object with one integer abi_version")
            }

            fn visit_map<A>(self, mut map: A) -> Result<Self::Value, A::Error>
            where
                A: MapAccess<'de>,
            {
                let mut abi_version = None;
                while let Some(key) = map.next_key::<String>()? {
                    if key == "abi_version" {
                        if abi_version.is_some() {
                            return Err(de::Error::duplicate_field("abi_version"));
                        }
                        abi_version = Some(map.next_value::<u16>()?);
                    } else {
                        map.next_value::<IgnoredAny>()?;
                    }
                }
                Ok(ProgramManifestAbi {
                    abi_version: abi_version
                        .ok_or_else(|| de::Error::missing_field("abi_version"))?,
                })
            }
        }

        deserializer.deserialize_map(ManifestVisitor)
    }
}

fn recorded_abi_version(project: &Path) -> Result<u16, String> {
    let manifest = fs::read_to_string(project.join("layerx-program.json"))
        .map_err(|error| format!("missing or unreadable layerx-program.json: {error}"))?;
    let recorded = serde_json::from_str::<ProgramManifestAbi>(&manifest)
        .map_err(|error| format!("invalid layerx-program.json: {error}"))?
        .abi_version;
    parse_version_number(recorded)
        .ok_or_else(|| format!("unsupported LayerX ABI version {recorded}"))
}

/// Parses a command argument through the shared runtime admission policy.
#[must_use]
pub fn parse_version(value: &str) -> Option<u16> {
    value.parse::<u16>().ok().and_then(parse_version_number)
}

const fn parse_version_number(value: u16) -> Option<u16> {
    match layerx_programs_runtime::admit_abi_version(value) {
        Ok(()) => Some(value),
        Err(_) => None,
    }
}

fn metadata_refusal(project: &Path, reason: impl Into<String>) -> DeterminismViolation {
    DeterminismViolation::UnreadablePath {
        path: project.join("layerx-program.json"),
        reason: reason.into(),
    }
}

#[cfg(test)]
mod tests {
    use super::{parse_version, recorded_abi_version};
    use std::fs;

    #[test]
    fn project_metadata_requires_one_supported_abi_version() {
        let root = std::env::temp_dir().join(format!("layerx-lint-abi-{}", std::process::id()));
        fs::create_dir_all(&root).unwrap_or_else(|error| panic!("fixture directory: {error}"));
        fs::write(
            root.join("layerx-program.json"),
            "{\n  \"abi_version\": 2\n}\n",
        )
        .unwrap_or_else(|error| panic!("fixture manifest: {error}"));
        assert_eq!(recorded_abi_version(&root), Ok(2));
        fs::write(root.join("layerx-program.json"), "{\"abi_version\":1}\n")
            .unwrap_or_else(|error| panic!("one-line fixture manifest: {error}"));
        assert_eq!(recorded_abi_version(&root), Ok(1));
        fs::write(
            root.join("layerx-program.json"),
            "{\"abi_version\":1,\"abi_version\":2}\n",
        )
        .unwrap_or_else(|error| panic!("duplicate fixture manifest: {error}"));
        assert!(recorded_abi_version(&root).is_err());
        fs::write(root.join("layerx-program.json"), "{\"abi_version\":2\n")
            .unwrap_or_else(|error| panic!("malformed fixture manifest: {error}"));
        assert!(recorded_abi_version(&root).is_err());
        fs::write(
            root.join("layerx-program.json"),
            "{\"description\":\"\\\"abi_version\\\":2\"}\n",
        )
        .unwrap_or_else(|error| panic!("embedded fixture manifest: {error}"));
        assert!(recorded_abi_version(&root).is_err());
        fs::write(
            root.join("layerx-program.json"),
            "{\"metadata\":{\"abi_version\":2}}\n",
        )
        .unwrap_or_else(|error| panic!("nested fixture manifest: {error}"));
        assert!(recorded_abi_version(&root).is_err());
        fs::write(
            root.join("layerx-program.json"),
            "{\"abi_version\":\"2\"}\n",
        )
        .unwrap_or_else(|error| panic!("string fixture manifest: {error}"));
        assert!(recorded_abi_version(&root).is_err());
        fs::write(root.join("layerx-program.json"), "{\"abi_version\":2.0}\n")
            .unwrap_or_else(|error| panic!("floating fixture manifest: {error}"));
        assert!(recorded_abi_version(&root).is_err());
        fs::write(root.join("layerx-program.json"), "{\"abi_version\":-1}\n")
            .unwrap_or_else(|error| panic!("negative fixture manifest: {error}"));
        assert!(recorded_abi_version(&root).is_err());
        fs::write(root.join("layerx-program.json"), "{\"abi_version\":3}\n")
            .unwrap_or_else(|error| panic!("v3 fixture manifest: {error}"));
        assert_eq!(recorded_abi_version(&root), Ok(3));
        fs::write(root.join("layerx-program.json"), "{\"abi_version\":4}\n")
            .unwrap_or_else(|error| panic!("v4 fixture manifest: {error}"));
        assert_eq!(recorded_abi_version(&root), Ok(4));
        fs::write(root.join("layerx-program.json"), "{\"abi_version\":5}\n")
            .unwrap_or_else(|error| panic!("v5 fixture manifest: {error}"));
        assert_eq!(recorded_abi_version(&root), Ok(5));
        fs::write(root.join("layerx-program.json"), "{\"abi_version\":6}\n")
            .unwrap_or_else(|error| panic!("unknown fixture manifest: {error}"));
        assert!(recorded_abi_version(&root).is_err());
        fs::write(root.join("layerx-program.json"), "{}\n")
            .unwrap_or_else(|error| panic!("fixture manifest: {error}"));
        assert!(recorded_abi_version(&root).is_err());
        fs::remove_dir_all(&root).unwrap_or_else(|error| panic!("fixture cleanup: {error}"));
        assert_eq!(parse_version("1"), Some(1));
        assert_eq!(parse_version("2"), Some(2));
        assert_eq!(parse_version("3"), Some(3));
        assert_eq!(parse_version("4"), Some(4));
        assert_eq!(parse_version("5"), Some(5));
        assert_eq!(parse_version("6"), None);
        assert_eq!(parse_version("65535"), None);
        assert_eq!(parse_version("0"), None);
    }
}
