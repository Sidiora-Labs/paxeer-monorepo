pub mod fetch;

use serde::{Deserialize, Serialize};

/// One published artifact bound to the source revision it was built from.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct ArtifactManifestEntry {
    pub name: String,
    pub version: String,
    pub registry: String,
    pub distribution: String,
    pub target: String,
    pub artifact: String,
    pub retained: String,
    pub digest: String,
    pub digest_of: String,
    pub signature: String,
    pub sbom: String,
    pub attestation: String,
    pub location: String,
    pub source_revision: String,
    pub rollback_version: Option<String>,
    pub published: bool,
    pub install_check: bool,
}
