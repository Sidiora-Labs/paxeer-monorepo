#[allow(dead_code)]
#[path = "../src/bin/layerx-mirror-verify.rs"]
mod verifier;

use std::io::{self, Cursor, Read};

use layerx_mirror::committed::{CommittedBlock, CommittedHeader};
use layerx_mirror::source::{
    MirrorCanonicalPosition, MirrorLocator, MirrorObservation, MirrorReadPolicy, MirrorSourceId,
    MirrorTargetIdentity,
};
use layerx_mirror::{archive_commitment, Archive, ArchiveCommitment, MirrorVerifier, NodeHead};
use serde_json::{json, Value};
use verifier::{Config, EvidenceRequest, Request, SourceConfig};

const CONFIG: &[u8] = include_bytes!("../../../deploy/mirror/verify-config.example.json");
const EXAMPLES: [&[u8]; 6] = [
    include_bytes!("../src/bin/verify-examples/receipt-exact.json"),
    include_bytes!("../src/bin/verify-examples/receipt-ordered-preference.json"),
    include_bytes!("../src/bin/verify-examples/receipt-agreement.json"),
    include_bytes!("../src/bin/verify-examples/committed-exact.json"),
    include_bytes!("../src/bin/verify-examples/committed-ordered-preference.json"),
    include_bytes!("../src/bin/verify-examples/committed-agreement.json"),
];

fn fixture() -> Value {
    serde_json::from_str(include_str!("fixtures/committed_blocks.json"))
        .unwrap_or_else(|error| panic!("recorded committed block: {error}"))
}

fn bytes(value: &Value) -> Vec<u8> {
    let text = value.as_str().unwrap_or_else(|| panic!("fixture hex"));
    let digits = text
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("hex prefix"));
    (0..digits.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&digits[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("hex byte: {error}"))
        })
        .collect()
}

fn recorded_verifier() -> MirrorVerifier {
    let fixture = fixture();
    let header = CommittedHeader::from_block_json(&fixture["committedBlock"])
        .unwrap_or_else(|error| panic!("header: {error:?}"));
    let height = header.height;
    let block_hash = header.block_hash;
    let receipts = fixture["receipts"]
        .as_array()
        .unwrap_or_else(|| panic!("receipts"))
        .iter()
        .map(|receipt| bytes(&receipt["receipt"]))
        .collect();
    let archive = Archive::from_committed(
        &CommittedBlock { header, receipts },
        125,
        NodeHead {
            latest_sealed_batch: height,
            latest_committed: None,
        },
    )
    .unwrap_or_else(|error| panic!("archive: {error:?}"));
    let observation = MirrorObservation::from_archive(
        MirrorSourceId::new("recorded-ethereum")
            .unwrap_or_else(|error| panic!("source: {error:?}")),
        MirrorTargetIdentity::Ethereum {
            chain_id: 11155111,
            genesis_hash: [1; 32],
            contract: [1; 20],
            code_hash: [1; 32],
            publisher: [1; 20],
        },
        archive.bytes(),
        125,
        height,
        archive_commitment(archive.bytes()),
        MirrorCanonicalPosition::Ethereum {
            block_number: height,
            block_hash,
            reference_head_number: height,
            reference_head_hash: block_hash,
        },
    )
    .unwrap_or_else(|error| panic!("observation: {error:?}"));
    MirrorVerifier::admit_committed(archive.bytes(), observation)
        .unwrap_or_else(|error| panic!("admit committed archive: {error:?}"))
}

#[test]
fn finding_9_example_loads_through_config_and_verifies_recorded_receipt() {
    let config = Config::decode(CONFIG).unwrap_or_else(|error| panic!("example config: {error}"));
    assert_eq!(config.layerx_network_id, 125);
    assert_eq!(config.receipt_keys.len(), 1);
    assert_eq!(config.receipt_keys[0].first_height, 1000);
    assert_eq!(config.receipt_keys[0].last_height, 1999);
    assert_eq!(config.sources.len(), 2);
    for source in &config.sources {
        let (id, rpc, pins) = match source {
            SourceConfig::Ethereum {
                id,
                rpc,
                chain_id,
                genesis_hash_hex,
                archive_contract_hex,
                archive_code_hash_hex,
                publisher_hex,
            } => {
                assert_eq!(*chain_id, 11155111);
                (
                    id,
                    rpc,
                    vec![
                        (genesis_hash_hex, 32),
                        (archive_contract_hex, 20),
                        (archive_code_hash_hex, 32),
                        (publisher_hex, 20),
                    ],
                )
            }
            SourceConfig::Solana {
                id,
                rpc,
                genesis_hash_hex,
                archive_program_hex,
                upgradeable_loader_hex,
                program_data_account_hex,
                program_code_hash_hex,
                publisher_hex,
            } => (
                id,
                rpc,
                vec![
                    (genesis_hash_hex, 32),
                    (archive_program_hex, 32),
                    (upgradeable_loader_hex, 32),
                    (program_data_account_hex, 32),
                    (program_code_hash_hex, 32),
                    (publisher_hex, 32),
                ],
            ),
        };
        assert!(!id.is_empty());
        assert_eq!(rpc.endpoints.len(), 3);
        assert_eq!(rpc.quorum, 2);
        assert_eq!(rpc.connect_timeout_ms, 3000);
        assert_eq!(rpc.request_timeout_ms, 15000);
        assert_eq!(rpc.maximum_response_bytes, 16 * 1024 * 1024);
        for endpoint in &rpc.endpoints {
            assert!(endpoint.url.starts_with("https://"));
            assert!(!endpoint.independent_backend.is_empty());
            assert!(endpoint.ca_certificate_der.is_absolute());
            assert!(endpoint.bearer_token_file.is_absolute());
        }
        for (pin, length) in pins {
            assert_eq!(pin.len(), length * 2);
            assert!(pin.bytes().all(|byte| byte.is_ascii_hexdigit()));
        }
    }
    let keys = config
        .receipt_authorizations()
        .unwrap_or_else(|error| panic!("keys: {error}"));
    let result = verifier::verify_evidence(
        recorded_verifier(),
        EvidenceRequest::Receipt { index: 0 },
        &keys,
    )
    .unwrap_or_else(|error| panic!("receipt verification: {error}"));
    assert_eq!(result["committedHeight"], "1000");
    assert_eq!(
        result["evidenceDigest"],
        fixture()["receipts"][0]["receiptDigest"]
            .as_str()
            .unwrap_or_else(|| panic!("digest"))
            .trim_start_matches("0x")
    );
}

#[test]
fn finding_9_legacy_unknown_and_invalid_key_windows_are_rejected() {
    let original: Value =
        serde_json::from_slice(CONFIG).unwrap_or_else(|error| panic!("config: {error}"));
    for field in [
        "sequencer_id_hex",
        "sequencer_public_key_hex",
        "first_batch_number",
        "last_batch_number",
    ] {
        let mut value = original.clone();
        value[field] = json!("legacy");
        assert!(Config::decode(
            &serde_json::to_vec(&value).unwrap_or_else(|error| panic!("encode: {error}"))
        )
        .is_err());
    }
    for keys in [
        json!([]),
        json!([{
            "sequencer_id_hex": "6b".repeat(32), "public_key_hex": "81".repeat(32),
            "first_height": 2000, "last_height": 1999
        }]),
    ] {
        let mut value = original.clone();
        value["receipt_keys"] = keys;
        let config = Config::decode(
            &serde_json::to_vec(&value).unwrap_or_else(|error| panic!("encode: {error}")),
        )
        .unwrap_or_else(|error| panic!("config: {error}"));
        assert!(matches!(
            config.receipt_authorizations(),
            Err("configuration")
        ));
    }
    let mut expired = Config::decode(CONFIG).unwrap_or_else(|error| panic!("config: {error}"));
    expired.receipt_keys[0].first_height = 1001;
    let keys = expired
        .receipt_authorizations()
        .unwrap_or_else(|error| panic!("keys: {error}"));
    assert_eq!(
        verifier::verify_evidence(
            recorded_verifier(),
            EvidenceRequest::Receipt { index: 0 },
            &keys
        ),
        Err("proof")
    );
}

#[test]
fn finding_27_canonical_examples_round_trip_real_codec_policy_and_evidence_consumer() {
    let config = Config::decode(CONFIG).unwrap_or_else(|error| panic!("config: {error}"));
    let keys = config
        .receipt_authorizations()
        .unwrap_or_else(|error| panic!("keys: {error}"));
    for (index, example) in EXAMPLES.iter().enumerate() {
        let request = Request::decode(Cursor::new(example))
            .unwrap_or_else(|error| panic!("example {index}: {error}"));
        let encoded =
            serde_json::to_vec(&request).unwrap_or_else(|error| panic!("producer: {error}"));
        let published: Value =
            serde_json::from_slice(example).unwrap_or_else(|error| panic!("example: {error}"));
        assert_eq!(
            serde_json::from_slice::<Value>(&encoded)
                .unwrap_or_else(|error| panic!("encoded: {error}")),
            published
        );
        let received = Request::decode(Cursor::new(encoded))
            .unwrap_or_else(|error| panic!("consumer: {error}"));
        assert_eq!(received.batch_number, "1000");
        let locator = |source_index| MirrorLocator {
            source_index,
            commitment: ArchiveCommitment::from_bytes([0x11; 32]),
        };
        let expected = match index % 3 {
            0 => MirrorReadPolicy::Exact(locator(0)),
            1 => MirrorReadPolicy::OrderedPreference(vec![locator(0), locator(1)]),
            _ => MirrorReadPolicy::Agreement {
                candidates: vec![locator(0), locator(1)],
                minimum: 2,
            },
        };
        assert_eq!(verifier::policy(&received.policy), Ok(expected));
        assert_eq!(
            matches!(received.evidence, EvidenceRequest::Receipt { index: 0 }),
            index < 3
        );
        let result = verifier::verify_evidence(recorded_verifier(), received.evidence, &keys)
            .unwrap_or_else(|error| panic!("verification: {error}"));
        assert_eq!(result["level"], "Committed");
        assert_eq!(result["sourceId"], "recorded-ethereum");
        assert_eq!(result["committedHeight"], "1000");
        assert_eq!(result["evidenceDigest"].is_string(), index < 3);
        if index >= 3 {
            assert!(result["evidenceDigest"].is_null());
        }
    }
}

#[test]
fn finding_27_unsupported_selectors_fields_and_receipt_bounds_are_rejected() {
    let original: Value =
        serde_json::from_slice(EXAMPLES[0]).unwrap_or_else(|error| panic!("example: {error}"));
    for evidence in [
        json!({"kind":"state"}),
        json!({"kind":"receipt"}),
        json!({"kind":"receipt","index":"0"}),
        json!({"kind":"receipt","index":-1}),
        json!({"kind":"receipt","index":0,"state":"arbitrary"}),
        json!({"kind":"committed","index":0}),
        json!({"kind":"committed","state":"arbitrary"}),
    ] {
        let mut request = original.clone();
        request["evidence"] = evidence;
        let encoded =
            serde_json::to_vec(&request).unwrap_or_else(|error| panic!("encode: {error}"));
        assert!(
            matches!(Request::decode(Cursor::new(encoded)), Err("malformed")),
            "{request}"
        );
    }
    let mut request = original.clone();
    request["evidence"]["index"] = json!(1_048_576);
    let encoded = serde_json::to_vec(&request).unwrap_or_else(|error| panic!("encode: {error}"));
    assert!(matches!(
        Request::decode(Cursor::new(encoded)),
        Err("bounds")
    ));
    request["evidence"]["index"] = json!(1_048_575);
    let encoded = serde_json::to_vec(&request).unwrap_or_else(|error| panic!("encode: {error}"));
    assert!(Request::decode(Cursor::new(encoded)).is_ok());
    assert!(matches!(
        Request::decode(io::repeat(b' ').take(40 * 1024 * 1024 + 1)),
        Err("bounds")
    ));
}
