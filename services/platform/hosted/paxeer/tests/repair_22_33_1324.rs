//! Finding-to-case and out-of-path owner handoffs: repair_22_33_1324.kvx.
//! These tests prove transport and provenance codecs, not BFT finality.

#[path = "tls_listen.rs"]
mod tls_listen;

use layerx_types::chain::{BlockReference, ValidatorSetTrust};
use std::sync::atomic::Ordering;
use tls_listen::node_tls::{self, EvidencePeer, ARCHIVE, TRUST};

#[test]
fn relay_retirement_26_chain_height_and_validator_set_evidence() {
    let trust = ValidatorSetTrust::parse(TRUST).expect("explicit current trust pin");
    assert_eq!(trust.chain_id(), 125);
    assert_eq!(trust.comet_chain_id(), "hyperpax_125-1");
    assert_eq!(trust.height(), 23_869_863);
    let producer = node_tls::reference_from_recording();
    let peer = EvidencePeer::start(producer);
    let consumer = node_tls::fetch(peer.address, "localhost", true)
        .expect("real producer through TLS into production decoder");
    trust
        .check_reference(&consumer)
        .expect("height-specific validator-set pin");
    assert_eq!(consumer, producer);
    assert_eq!(consumer.validator_set_hash(), trust.validator_set_hash());
    assert_eq!(consumer.covering_height(), trust.height());
    assert_eq!(consumer.execution_height(), 23_869_863);
    assert_eq!(consumer.encode(), producer.encode());
    assert!(
        ValidatorSetTrust::parse(ARCHIVE).is_err(),
        "retired guarantor domains cannot become current trust"
    );
    assert!(
        ValidatorSetTrust::parse(&TRUST.replace("\"chain_id\": 125", "\"chain_id\": 126")).is_err()
    );
    let mut other_chain = producer.encode();
    other_chain[12] = 126;
    assert!(BlockReference::decode(&other_chain).is_err());
    for invalid in [
        BlockReference::new(
            producer.execution_height(),
            producer.covering_height() + 1,
            producer.block_hash(),
            producer.validator_set_hash(),
            producer.state_root(),
        )
        .expect("different height producer"),
        BlockReference::new(
            producer.execution_height(),
            producer.covering_height(),
            producer.block_hash(),
            [0x99; 32],
            producer.state_root(),
        )
        .expect("different validator-set producer"),
    ] {
        let rejected_peer = EvidencePeer::start(invalid);
        let transported = node_tls::fetch(rejected_peer.address, "localhost", true)
            .expect("transport does not imply evidence authority");
        assert!(trust.check_reference(&transported).is_err());
        assert_eq!(rejected_peer.finish(), vec![Ok(())]);
    }
    assert_eq!(peer.finish(), vec![Ok(())]);
}

#[test]
fn relay_retirement_26_migration_v1_preserves_archive_without_finality() {
    use layerx_proof::checkpoint::{
        checkpoint_id, Attestation, Certificate, Checkpoint, CheckpointError, GuarantorKey,
        SettlementDomain,
    };
    use layerx_types::historical::settlement::DeclaredCheckpointSettlement;
    use layerx_types::vectors::checkpoint::load_checkpoint_vector;
    let archive =
        DeclaredCheckpointSettlement::parse(ARCHIVE).expect("version-one historical export reader");
    assert_eq!(archive.protocol_version(), 2);
    let domain = archive.domain("vectors").expect("archived vector domain");
    assert_eq!(domain.paxeer_chain_id(), 31_337);
    let vector = load_checkpoint_vector(
        std::path::Path::new(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../tests/vectors/checkpoint/fresh.json"
        )),
        "fresh",
    )
    .expect("recorded historical export");
    let checkpoint = Checkpoint::new(vector.header.bytes.clone(), vector.validity_proof.clone());
    assert_eq!(
        checkpoint_id(&checkpoint).expect("historical canonical checkpoint codec"),
        vector.expected_digest
    );
    let attestations: Vec<_> = vector
        .attestations
        .iter()
        .map(|item| {
            let attestation = Attestation::new(
                vector.header.protocol_version,
                vector.header.network_id,
                domain.paxeer_chain_id(),
                domain.settlement_contract(),
                vector.header.epoch,
                vector.expected_digest,
                vector.expected_digest,
                item.guarantor_id,
                vector.header.batch_number,
                vector.header.data_availability_root,
                item.replayed,
                item.data_possessed,
                item.availability_class_mask,
                item.attested_at_ms,
                item.signer,
                item.signature,
                item.signature_v,
            );
            assert_eq!(
                attestation.canonical_statement(),
                item.message,
                "historical statements retain every exact exported byte"
            );
            attestation
        })
        .collect();
    let certificate = Certificate::new(checkpoint, attestations, vector.threshold, None);
    let keys: Vec<_> = domain
        .guarantor_set()
        .iter()
        .map(|key| GuarantorKey::new(key.guarantor_id(), key.public_key(), true))
        .collect();
    #[allow(deprecated)]
    let outcome = layerx_proof::checkpoint::verify_certificate(
        &certificate,
        &keys,
        &vector.expected_digest,
        SettlementDomain::new(domain.paxeer_chain_id(), domain.settlement_contract()),
        None,
    );
    assert_eq!(outcome, Err(CheckpointError::RetiredCertificateVerifier));
    assert!(ValidatorSetTrust::parse(ARCHIVE).is_err());
}

#[test]
fn relay_retirement_27_listener_address_and_tls_identity() {
    node_tls::qualify_listener();
    node_tls::qualify_tls_identity();
}

#[test]
fn relay_retirement_28_bounded_tls_shutdown_and_resource_release() {
    let producer = node_tls::reference_from_recording();
    let peer = EvidencePeer::start(producer);
    let address = peer.address;
    std::thread::scope(|scope| {
        let workers: Vec<_> = (0..8)
            .map(|_| {
                scope.spawn(|| {
                    for _ in 0..125 {
                        assert_eq!(
                            node_tls::fetch(address, "localhost", true)
                                .expect("complete TLS response and close_notify"),
                            producer
                        );
                    }
                })
            })
            .collect();
        for worker in workers {
            worker
                .join()
                .expect("all eight clients complete without errors");
        }
    });
    assert!(
        peer.maximum.load(Ordering::SeqCst) <= 8,
        "bounded TLS workers"
    );
    let results = peer.finish();
    assert_eq!(
        results.len(),
        1000,
        "retain every request in the original TLS cleanup workload"
    );
    assert!(
        results.iter().all(Result::is_ok),
        "every response, TLS shutdown and connection release must succeed: {results:?}"
    );
}
