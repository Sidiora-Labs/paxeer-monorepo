fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

use sha2::{Digest, Sha256};
use std::path::{Path, PathBuf};

pub const CORPUS_READ_EXPECTATION: &str = "committed authority corpus must be readable; first run the producer test: cargo test --locked --manifest-path services/platform/hosted/authority/Cargo.toml --test repair_22_31_535 repair_22_31_535_producers_emit_committed_documents";

pub fn corpus_path(repository: &Path) -> PathBuf {
    let repository = require_ok(repository.canonicalize(), "repository root");
    let mut identity = Sha256::new();
    identity.update(repository.as_os_str().as_encoded_bytes());
    for source in [
        "services/platform/hosted/authority/src/human.rs",
        "services/platform/hosted/authority/src/human/budget_state.rs",
        "services/platform/hosted/authority/tests/repair_22_31_535.rs",
    ] {
        identity.update(require_ok(
            std::fs::read(repository.join(source)),
            "producer source",
        ));
    }
    let digest: [u8; 32] = identity.finalize().into();
    let suffix = layerx_platform_authority::hex::encode(&digest);
    std::env::temp_dir()
        .join(format!("layerx-repair-22-31-535-{suffix}"))
        .join("documents.json")
}
