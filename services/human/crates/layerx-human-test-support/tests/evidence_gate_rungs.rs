use std::collections::BTreeMap;

use ed25519_dalek::SigningKey;
use layerx_agentd::boot::Status;
use layerx_agentd::config::ReceiptKeySource;
use layerx_agentd::store::TenantId;
use layerx_human_test_support::{evidence_gate, evidence_startup_config};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;

#[test]
fn evidence_gate_defaults_tenant_to_committed_rung() {
    let signer = SigningKey::from_bytes(&[0x83; 32]);
    let gate = evidence_gate(&signer);
    let config = evidence_startup_config(&signer);
    let tenant =
        TenantId::new("human-evidence").unwrap_or_else(|error| panic!("evidence tenant: {error}"));

    assert_eq!(gate.status(), &Status::NotReady);
    assert_eq!(
        config.verification_defaults,
        BTreeMap::from([(tenant.clone(), VerificationLevel::CHECKPOINT_FINALISED)])
    );
    assert_eq!(
        config.verification_defaults[&tenant].rung(),
        Some(Rung::Committed)
    );

    let ReceiptKeySource::PinnedFile(authority_path) = config.receipt_key_source else {
        panic!("evidence gate must use a pinned authority file");
    };
    std::fs::remove_file(authority_path)
        .unwrap_or_else(|error| panic!("remove evidence authority: {error}"));
}
