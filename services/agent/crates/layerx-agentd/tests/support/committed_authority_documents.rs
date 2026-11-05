use sha2::{Digest, Sha256};
use std::path::{Path, PathBuf};

pub const CORPUS_READ_EXPECTATION: &str = "committed authority corpus must be readable; first run the producer test: cargo test --locked --manifest-path services/platform/hosted/authority/Cargo.toml --test repair_22_31_535 repair_22_31_535_producers_emit_committed_documents";

pub fn corpus_path(repository: &Path) -> PathBuf {
    let repository = repository.canonicalize().expect("repository root");
    let mut identity = Sha256::new();
    identity.update(repository.as_os_str().as_encoded_bytes());
    for source in [
        "services/platform/hosted/authority/src/human.rs",
        "services/platform/hosted/authority/src/human/budget_state.rs",
        "services/platform/hosted/authority/tests/repair_22_31_535.rs",
    ] {
        identity.update(std::fs::read(repository.join(source)).expect("producer source"));
    }
    let digest: [u8; 32] = identity.finalize().into();
    let suffix: String = digest.iter().map(|byte| format!("{byte:02x}")).collect();
    std::env::temp_dir()
        .join(format!("layerx-repair-22-31-535-{suffix}"))
        .join("documents.json")
}
