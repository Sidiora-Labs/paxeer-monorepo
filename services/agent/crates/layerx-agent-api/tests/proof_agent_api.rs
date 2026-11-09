//! Proves: `ProofBundle/OfflineExport/VerifiedRead` -> public receipt-proof-v2 JSON codecs -> `proof_bundle_operation_value/serialize_export/deserialize_export` and RPC request checks.
//!
//! The consumer codecs live on the public contract and are called directly by
//! `agent_rpc_wire::ProofBundleWire`, `agent_rpc_adapters::read_proof_bundle`,
//! `human_runtime::proof_bundle_operation_value`, `export::serialize_export`, and
//! `export::deserialize_export`. This proof executes those shared implementations rather
//! than duplicating a wire decoder or replacing the transport with a double.
//! The producer material is the recorded, signed native conformance corpus;
//! only the public API constructs the messages consumed below.
//!
//! Repairs: `human_runtime.rs::proof_bundle_operation_value` discarded the producer's
//! block hash and relative height; `export.rs::deserialize_export` accepted duplicate
//! JSON keys through `serde_json::Value`. Both now use the shared strict codec.
//! `stream.rs::EventDelivery::new` also accepted a Submitted reference as Verified;
//! `agent_rpc_adapters.rs::read_proof_bundle` failed to check the owner's requested rung.
//!
//! Inventory finding: error.rs's historical Level has been replaced by Rung.
//! The local Level alias below pins the current three-rung contract without
//! restoring retired public vocabulary. These are structural contract checks;
//! signature, inclusion and commit authentication remain separate consumer duties.
//!
//! The additional `committed_landing` corpus below carries actual protobuf
//! Comet headers/commits/validators and encoded EVM ABCI results from one node
//! landing block. Its independent verifier checks receipt signatures,
//! transaction/batch/result inclusion, commit power/signatures and the successor
//! link. The original structural corpus and all malformed-field checks remain.
//!
//! Historical structural corpus: sdk/platform/conformance/fixtures/vector-v5-portable.json,
//! bundles[1].{`blockHeader`,`receiptInclusion`,commit,`validatorSet`}, stores the native
//! recorder's structured JSON rather than encoded protobuf/EVM proof strings.
//! This structural API proof preserves those original producer bytes as opaque
//! `CanonicalBytes` and asserts exact preservation; it does not claim that the
//! independent Comet or EVM evidence verifier accepts that representation.

#[path = "support/committed_landing.rs"]
pub(crate) mod committed;

use layerx_agent_api::export::{
    check_export_response, validate_export_request, FactRef, OfflineExport,
};
use layerx_agent_api::identity::ContractError;
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::proof::{ProofBundle, ProofBundleTarget};
use layerx_agent_api::read::{Freshness, ReadRequest, RelativeTo, VerifiedRead};
use layerx_agent_api::subscription::{Cursor, EventDelivery, EventIdentity, ReceiptReference};
use layerx_agent_api::track::ReceiptRef;
use layerx_agent_api::verify::{ReasonCode, Rung, RungStatus};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};
use layerx_types::json::{parse, JsonValue};

type Level = Rung;

const RECORDED: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/vector-v5-portable.json");

fn quantity(value: &JsonValue, path: &str) -> u64 {
    u64::from_str_radix(
        value
            .str_at(path)
            .unwrap_or_else(|error| panic!("recorded quantity: {error:?}"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("hex quantity")),
        16,
    )
    .unwrap_or_else(|error| panic!("bounded recorded quantity: {error:?}"))
}

fn canonical(value: &JsonValue, path: &str) -> CanonicalBytes {
    CanonicalBytes::new(
        value
            .hex_at(path)
            .unwrap_or_else(|error| panic!("recorded native bytes: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("nonempty native bytes: {error:?}"))
}

// Read an exact producer-emitted JSON fragment, without inventing proof bytes
// or re-encoding the native recorder's evidence into another proof format.
fn recorded_evidence(name: &str) -> CanonicalBytes {
    let key = format!("\"{name}\":");
    let start = RECORDED
        .find(&key)
        .unwrap_or_else(|| panic!("recorded evidence field"))
        + key.len();
    let source = RECORDED[start..].trim_start();
    assert!(matches!(source.as_bytes()[0], b'{' | b'['));
    let mut depth = 0_usize;
    let mut quoted = false;
    let mut escaped = false;
    for (offset, byte) in source.bytes().enumerate() {
        if quoted {
            if escaped {
                escaped = false;
            } else if byte == b'\\' {
                escaped = true;
            } else if byte == b'"' {
                quoted = false;
            }
            continue;
        }
        match byte {
            b'"' => quoted = true,
            b'{' | b'[' => depth += 1,
            b'}' | b']' => {
                depth -= 1;
                if depth == 0 {
                    let fragment = &source[..=offset];
                    parse(fragment).unwrap_or_else(|error| {
                        panic!("complete original evidence record: {error:?}")
                    });
                    return CanonicalBytes::new(fragment.as_bytes().to_vec())
                        .unwrap_or_else(|error| panic!("nonempty producer record: {error:?}"));
                }
            }
            _ => {}
        }
    }
    panic!("truncated producer evidence");
}

fn producer(index: usize) -> (ProofBundle, [u8; 32], u64) {
    // Reopen the recorded native output on every call: the API holds no cache.
    let document = parse(RECORDED)
        .unwrap_or_else(|error| panic!("recorded portable native material: {error:?}"));
    let value = &document
        .array_at("bundles")
        .unwrap_or_else(|error| panic!("native bundles: {error:?}"))[index];
    let optional = |name| value.field(name).map(|_| recorded_evidence(name));
    let bundle = ProofBundle {
        format: value
            .str_at("format")
            .unwrap_or_else(|error| panic!("format: {error:?}"))
            .into(),
        verification_level: Level::from_name(
            value
                .str_at("verificationLevel")
                .unwrap_or_else(|error| panic!("rung: {error:?}")),
        )
        .unwrap_or_else(|| panic!("current rung")),
        raw_transaction: canonical(value, "rawTransaction"),
        signed_receipt: canonical(value, "signedReceipt"),
        receipt_digest: value
            .hex_array_at("receiptDigest")
            .unwrap_or_else(|error| panic!("receipt digest: {error:?}")),
        receipt_key: ReceiptKeyEntry {
            key_id: value
                .hex_array_at("receiptKey.keyId")
                .unwrap_or_else(|error| panic!("key identifier: {error:?}")),
            public_key: value
                .hex_array_at("receiptKey.publicKey")
                .unwrap_or_else(|error| panic!("public key: {error:?}")),
            first_height: quantity(value, "receiptKey.firstHeight"),
            last_height: quantity(value, "receiptKey.lastHeight"),
        },
        block_header: optional("blockHeader"),
        receipt_inclusion: optional("receiptInclusion"),
        commit: optional("commit"),
        validator_set: optional("validatorSet"),
        account_proof: None,
    };
    let id = value
        .hex_array_at("receiptJson.txHash")
        .unwrap_or_else(|error| panic!("native transaction identifier: {error:?}"));
    (bundle, id, quantity(value, "receiptJson.height"))
}

fn coordinate(head: u64, height: u64, marker: u8) -> Freshness {
    Freshness {
        chain_head: head,
        value_height: height,
        relative_to: RelativeTo::Block {
            height: head,
            hash: [marker; 32],
        },
    }
}

#[test]
fn proof_agent_api_receipt_producer_crosses_rpc_contract_twice_and_refuses_malformed_output(
) -> Result<(), String> {
    let mut observations = Vec::new();
    for marker in [4, 5] {
        let (bundle, id, height) = producer(0);
        let key = bundle.receipt_key;
        let pins = [SequencerAuthorization::new(
            key.key_id,
            key.public_key,
            key.first_height,
            key.last_height,
        )];
        assert!(verify_sequencer_signature(
            bundle.signed_receipt.as_bytes(),
            ReceiptKeys::HeightRanges(&pins)
        )
        .is_ok());
        let wrong_pins = [SequencerAuthorization::new(
            key.key_id,
            [1; 32],
            key.first_height,
            key.last_height,
        )];
        assert!(verify_sequencer_signature(
            bundle.signed_receipt.as_bytes(),
            ReceiptKeys::HeightRanges(&wrong_pins)
        )
        .is_err());

        let selector = ProofBundleTarget::Receipt(id)
            .encode()
            .map_err(|error| format!("real selector producer: {error:?}"))?;
        let request =
            ReadRequest::proof_bundle(selector.as_bytes().to_vec(), Level::Receipted.name())
                .map_err(|error| format!("RPC selector consumer: {error:?}"))?;
        assert_eq!(request.selector, selector);
        let produced = VerifiedRead::new(
            bundle,
            Level::Receipted,
            coordinate(height + u64::from(marker), height, marker),
        );
        request
            .check_proof_response(&produced)
            .map_err(|error| format!("RPC owner response check: {error:?}"))?;
        let transport = produced
            .encode_json()
            .map_err(|error| format!("real RPC codec: {error:?}"))?;
        // Drop the producer before decoding to prove the consumer owns its result.
        let expected = produced.clone();
        drop(produced);
        let observed = VerifiedRead::<ProofBundle>::decode_json(&transport)
            .map_err(|error| format!("RPC response consumer: {error:?}"))?;
        assert_eq!(observed, expected);
        request
            .check_proof_response(&observed)
            .map_err(|error| format!("consumer target and rung: {error:?}"))?;
        assert_eq!(
            Freshness::decode_json(&observed.freshness.encode_json()),
            Ok(observed.freshness.clone())
        );
        let envelope = RungStatus::Achieved(observed.achieved_rung);
        assert_eq!(envelope.validate(), Ok(()));
        observations.push(observed.freshness);

        let malformed = transport.replace(
            "\"verificationLevel\":\"receipted\"",
            "\"verificationLevel\":\"submitted\"",
        );
        assert_ne!(malformed, transport);
        assert!(VerifiedRead::<ProofBundle>::decode_json(&malformed).is_err());
        let inconsistent = transport.replace(
            "\"achieved_verification_level\":\"receipted\"",
            "\"achieved_verification_level\":\"committed\"",
        );
        assert_ne!(inconsistent, transport);
        assert!(VerifiedRead::<ProofBundle>::decode_json(&inconsistent).is_err());
        let duplicate =
            transport.replacen("\"chain_head\":", "\"chain_head\":\"0\",\"chain_head\":", 1);
        assert_ne!(duplicate, transport);
        assert!(VerifiedRead::<ProofBundle>::decode_json(&duplicate).is_err());
        let too_high = ReadRequest {
            requested_rung: Level::Committed,
            ..request
        };
        assert!(too_high.check_proof_response(&expected).is_err());
        let mut bad_selector = selector.as_bytes().to_vec();
        bad_selector[2] = 255;
        assert!(ReadRequest::proof_bundle(bad_selector, "receipted").is_err());
        assert!(
            ReadRequest::proof_bundle(selector.as_bytes().to_vec(), "sequencer-signed").is_err()
        );
    }
    assert_ne!(
        observations[0], observations[1],
        "the second consumer must see the new producer coordinate"
    );

    Ok(())
}

#[test]
fn proof_agent_api_committed_export_crosses_retained_codec_twice_and_refuses_duplicate_keys(
) -> Result<(), String> {
    for _ in 0..2 {
        let (bundle, id, height) = producer(1);
        // The corpus publishes structured evidence; assert its exact original
        // bytes before the API's opaque-byte crossing, not a fabricated proof.
        for (name, evidence) in [
            ("blockHeader", &bundle.block_header),
            ("receiptInclusion", &bundle.receipt_inclusion),
            ("commit", &bundle.commit),
            ("validatorSet", &bundle.validator_set),
        ] {
            assert_eq!(evidence.as_ref(), Some(&recorded_evidence(name)));
        }
        let facts = vec![FactRef::new(format!(
            "receipt:{}",
            id.iter()
                .flat_map(|byte| {
                    let digits = b"0123456789abcdef";
                    [
                        char::from(digits[usize::from(byte >> 4)]),
                        char::from(digits[usize::from(byte & 15)]),
                    ]
                })
                .collect::<String>()
        ))
        .map_err(|error| format!("real fact selector: {error:?}"))?];
        let request = validate_export_request(ReadRequest {
            selector: facts.clone(),
            requested_rung: Level::Committed,
        })
        .map_err(|error| format!("export request consumer: {error:?}"))?;
        let produced = OfflineExport {
            facts,
            bundles: vec![bundle],
            height,
        };
        let transport = produced
            .encode_json()
            .map_err(|error| format!("retained export producer: {error:?}"))?;
        let expected = produced.clone();
        drop(produced);
        let observed = OfflineExport::decode_json(&transport)
            .map_err(|error| format!("retained export consumer: {error:?}"))?;
        assert_eq!(observed, expected);
        let response = VerifiedRead::new(observed, Level::Committed, coordinate(height, height, 8));
        assert_eq!(check_export_response(&request, &response), Ok(()));
        let duplicate = transport.replacen("\"height\":", "\"height\":\"0\",\"height\":", 1);
        assert_ne!(duplicate, transport);
        assert!(OfflineExport::decode_json(&duplicate).is_err());
        let malformed = transport.replace("\"commit\":", "\"unknownCommit\":");
        assert_ne!(malformed, transport);
        assert!(OfflineExport::decode_json(&malformed).is_err());
        let mut wrong = response;
        wrong.value.height += 1;
        assert_eq!(
            check_export_response(&request, &wrong),
            Err(ContractError::Mismatch("export_height"))
        );
    }

    Ok(())
}

#[test]
fn proof_agent_api_receipt_reference_consumer_requires_receipted_evidence() -> Result<(), String> {
    let (bundle, id, height) = producer(0);
    let identity = EventIdentity::new(id);
    let reference = ReceiptRef::new(format!("receipt:{height}"))
        .map_err(|error| format!("receipt reference producer: {error:?}"))?;
    for rung in [Level::Receipted, Level::Committed] {
        let observed = EventDelivery::new(
            identity,
            bundle.signed_receipt.as_bytes().to_vec(),
            Cursor {
                block_height: height,
                tx_index: 0,
                log_index: 0,
            },
            ReceiptReference::Verified {
                receipt_ref: reference.clone(),
                rung,
            },
        )
        .map_err(|error| format!("event consumer: {error:?}"))?;
        assert_eq!(
            observed.receipt_reference,
            ReceiptReference::Verified {
                receipt_ref: reference.clone(),
                rung
            }
        );
        assert_eq!(observed.event_bytes, bundle.signed_receipt.as_bytes());
        assert!(EventDelivery::new(
            identity,
            observed.event_bytes,
            observed.cursor,
            ReceiptReference::Verified {
                receipt_ref: reference.clone(),
                rung: Level::Submitted
            }
        )
        .is_err());
    }
    let reason = ReasonCode::new("verification.below_requested")
        .map_err(|error| format!("stable reason: {error:?}"))?;
    assert_eq!(
        RungStatus::Below {
            requested: Level::Committed,
            achieved: Level::Receipted,
            reason: reason.clone()
        }
        .validate(),
        Ok(())
    );
    assert!(RungStatus::Below {
        requested: Level::Receipted,
        achieved: Level::Committed,
        reason
    }
    .validate()
    .is_err());

    Ok(())
}

#[test]
fn proof_agent_api_freshness_consumer_preserves_full_width_heights_and_refuses_lossy_forms() {
    let produced = coordinate(u64::MAX, 9_007_199_254_740_993, 9);
    let transport = produced.encode_json();
    for _ in 0..2 {
        assert_eq!(Freshness::decode_json(&transport), Ok(produced.clone()));
    }
    for malformed in [
        transport.replace("9007199254740993", "09007199254740993"),
        transport.replace("9007199254740993", "18446744073709551616"),
        transport.replace("\"9007199254740993\"", "9007199254740993"),
        transport.replace("relative_to", "relativeTo"),
        format!("{transport} null"),
    ] {
        assert_ne!(malformed, transport);
        assert!(Freshness::decode_json(&malformed).is_err());
    }
}

#[test]
fn proof_agent_api_encoded_node_evidence_refuses_tampering() -> Result<(), String> {
    let produced = committed::produced();
    let wire = produced
        .encode_json()
        .map_err(|error| format!("daemon shared response encoder: {error:?}"))?;
    let observed = VerifiedRead::<ProofBundle>::decode_json(&wire)
        .map_err(|error| format!("SDK shared response decoder: {error:?}"))?;
    assert_eq!(observed, produced);
    committed::verify(&observed)
        .map_err(|error| format!("independent authenticated committed evidence: {error:?}"))?;
    for field in 0..4 {
        let mut changed = observed.clone();
        let evidence = match field {
            0 => &mut changed.value.block_header,
            1 => &mut changed.value.receipt_inclusion,
            2 => &mut changed.value.commit,
            _ => &mut changed.value.validator_set,
        };
        let mut bytes = evidence
            .as_ref()
            .ok_or_else(|| "required value".to_owned())?
            .as_bytes()
            .to_vec();
        let last = bytes.len() - 1;
        bytes[last] ^= 1;
        *evidence =
            Some(CanonicalBytes::new(bytes).map_err(|error| format!("required value: {error:?}"))?);
        // Structural opacity must never be mistaken for authentication.
        let crossed = changed
            .encode_json()
            .map_err(|error| format!("required value: {error:?}"))?;
        let crossed = VerifiedRead::<ProofBundle>::decode_json(&crossed)
            .map_err(|error| format!("required value: {error:?}"))?;
        assert!(
            committed::verify(&crossed).is_err(),
            "tampered committed field {field}"
        );
    }
    let inclusion = std::str::from_utf8(
        observed
            .value
            .receipt_inclusion
            .as_ref()
            .ok_or_else(|| "required value".to_owned())?
            .as_bytes(),
    )
    .map_err(|error| format!("required value: {error:?}"))?;
    for (from, to) in [
        ("\"index\":1", "\"index\":0"),
        ("\"height\":10", "\"height\":9"),
        ("\"total\":2", "\"total\":3"),
        ("\"result\":\"12", "\"result\":\"13"),
        ("comet-abci-results-v1", "comet-abci-results-v0"),
    ] {
        let changed = inclusion.replacen(from, to, 1);
        assert_ne!(changed, inclusion);
        let mut bad_inclusion = observed.clone();
        bad_inclusion.value.receipt_inclusion = Some(
            CanonicalBytes::new(changed.into_bytes())
                .map_err(|error| format!("required value: {error:?}"))?,
        );
        assert!(committed::verify(&bad_inclusion).is_err());
    }
    assert_committed_tampering(&observed)?;
    for (from, to) in [
        ("\"commit\":", "\"unknownCommit\":"),
        (
            "\"achieved_verification_level\":\"committed\"",
            "\"achieved_verification_level\":\"receipted\"",
        ),
        ("\"chain_head\":", "\"chain_head\":\"0\",\"chain_head\":"),
    ] {
        let malformed = wire.replacen(from, to, 1);
        assert_ne!(malformed, wire);
        assert!(VerifiedRead::<ProofBundle>::decode_json(&malformed).is_err());
    }

    Ok(())
}

fn assert_committed_tampering(observed: &VerifiedRead<ProofBundle>) -> Result<(), String> {
    let mut insufficient = observed.clone();
    insufficient.value.commit = Some(
        CanonicalBytes::new(
            committed::document()
                .hex_at("insufficientCommit")
                .map_err(|error| format!("required value: {error:?}"))?,
        )
        .map_err(|error| format!("required value: {error:?}"))?,
    );
    assert!(
        committed::verify(&insufficient).is_err(),
        "absent voting power cannot establish commitment"
    );
    let mut bad_receipt = observed.clone();
    let mut bytes = bad_receipt.value.signed_receipt.as_bytes().to_vec();
    let last = bytes.len() - 1;
    bytes[last] ^= 1;
    bad_receipt.value.signed_receipt =
        CanonicalBytes::new(bytes).map_err(|error| format!("required value: {error:?}"))?;
    assert!(committed::verify(&bad_receipt).is_err());
    let mut bad_digest = observed.clone();
    bad_digest.value.receipt_digest[0] ^= 1;
    assert!(committed::verify(&bad_digest).is_err());
    let mut bad_key = observed.clone();
    bad_key.value.receipt_key.public_key[0] ^= 1;
    assert!(committed::verify(&bad_key).is_err());
    let mut bad_raw = observed.clone();
    let mut raw = bad_raw.value.raw_transaction.as_bytes().to_vec();
    raw[4] ^= 1;
    bad_raw.value.raw_transaction =
        CanonicalBytes::new(raw).map_err(|error| format!("required value: {error:?}"))?;
    assert!(committed::verify(&bad_raw).is_err());
    let mut bad_freshness = observed.clone();
    bad_freshness.freshness.value_height += 1;
    assert!(committed::verify(&bad_freshness).is_err());
    let mut bad_hash = observed.clone();
    let RelativeTo::Block { hash, .. } = &mut bad_hash.freshness.relative_to;
    hash[0] ^= 1;
    assert!(committed::verify(&bad_hash).is_err());
    Ok(())
}
