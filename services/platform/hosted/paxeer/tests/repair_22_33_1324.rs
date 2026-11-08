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
    use layerx_types::historical::checkpoint::GUARANTOR_ATTESTATION_FIELDS;
    use layerx_types::historical::settlement::DeclaredCheckpointSettlement;
    use layerx_types::vectors::checkpoint::load_checkpoint_vector;
    use sha2::{Digest, Sha256};

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
    let mut checkpoint_digest = Sha256::new();
    checkpoint_digest.update(b"LXP/v2/checkpoint-certificate\0");
    checkpoint_digest.update(&vector.header.bytes);
    checkpoint_digest.update(
        u32::try_from(vector.validity_proof.len())
            .expect("archived proof length")
            .to_be_bytes(),
    );
    checkpoint_digest.update(&vector.validity_proof);
    assert_eq!(
        <[u8; 32]>::from(checkpoint_digest.finalize()),
        vector.expected_digest
    );
    assert!(!vector.attestations.is_empty());
    for item in &vector.attestations {
        let fields = [
            (
                "protocol_version",
                vector.header.protocol_version.to_be_bytes().to_vec(),
            ),
            (
                "network_id",
                vector.header.network_id.to_be_bytes().to_vec(),
            ),
            (
                "paxeer_chain_id",
                domain.paxeer_chain_id().to_be_bytes().to_vec(),
            ),
            (
                "paxeer_settlement_contract",
                domain.settlement_contract().to_vec(),
            ),
            ("epoch", vector.header.epoch.to_be_bytes().to_vec()),
            ("checkpoint_id", vector.expected_digest.to_vec()),
            ("checkpoint_hash", vector.expected_digest.to_vec()),
            ("guarantor_id", item.guarantor_id.to_vec()),
            (
                "batch_number",
                vector.header.batch_number.to_be_bytes().to_vec(),
            ),
            (
                "data_availability_root",
                vector.header.data_availability_root.to_vec(),
            ),
            ("replayed", vec![u8::from(item.replayed)]),
            ("da_possessed", vec![u8::from(item.data_possessed)]),
            (
                "availability_class_mask",
                vec![item.availability_class_mask],
            ),
            ("attested_at_ms", item.attested_at_ms.to_be_bytes().to_vec()),
            ("signer", item.signer.to_vec()),
            ("signature", item.signature.to_vec()),
            ("signature_v", vec![item.signature_v]),
        ];
        assert_eq!(
            fields.each_ref().map(|(name, _)| *name),
            GUARANTOR_ATTESTATION_FIELDS
        );
        let statement: Vec<u8> = fields[..14]
            .iter()
            .flat_map(|(_, bytes)| bytes.iter().copied())
            .collect();
        assert_eq!(
            statement, item.message,
            "historical statements retain every exact exported byte"
        );
        let mut digest = Sha256::new();
        digest.update(b"LXP/v2/guarantor-attestation\0");
        digest.update(&statement);
        assert_eq!(<[u8; 32]>::from(digest.finalize()), item.digest);
    }
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
