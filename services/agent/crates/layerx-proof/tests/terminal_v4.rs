use std::path::PathBuf;

use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptCheck, ReceiptKeys};
use layerx_proof::sequencer_logs::{verify_logs, LogField, SequencerLog, SequencerLogRefusal};
use layerx_types::json::{parse, JsonValue};
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::{decode, encode, encode_unsigned, Receipt};
use sha2::{Digest as _, Sha256};

fn load(name: &str) -> JsonValue {
    let directory = std::env::var_os("LAYERX_TERMINAL_FIXTURE_DIR").map_or_else(
        || {
            PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                .join("../../../../sdk/platform/conformance/fixtures")
        },
        PathBuf::from,
    );
    let document = std::fs::read_to_string(directory.join(name))
        .unwrap_or_else(|error| panic!("fixture {name}: {error}"));
    parse(&document).unwrap_or_else(|error| panic!("fixture {name}: {error}"))
}

fn bytes(document: &JsonValue, field: &str) -> Vec<u8> {
    document
        .hex_at(field)
        .unwrap_or_else(|error| panic!("{field}: {error}"))
}

fn array<const N: usize>(document: &JsonValue, field: &str) -> [u8; N] {
    document
        .hex_array_at(field)
        .unwrap_or_else(|error| panic!("{field}: {error}"))
}

fn quantity(document: &JsonValue, field: &str) -> u64 {
    let text = document
        .str_at(field)
        .unwrap_or_else(|error| panic!("{error}"));
    let digits = text
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("{field}: hex quantity"));
    u64::from_str_radix(digits, 16).unwrap_or_else(|error| panic!("{field}: {error}"))
}

fn cases(document: &JsonValue) -> &[JsonValue] {
    document
        .array_at("cases")
        .unwrap_or_else(|error| panic!("{error}"))
}

fn pin(case: &JsonValue) -> SequencerAuthorization {
    SequencerAuthorization::new(
        array(case, "pinnedKey.keyId"),
        array(case, "pinnedKey.publicKey"),
        case.u64_at("pinnedKey.firstHeight")
            .unwrap_or_else(|error| panic!("{error}")),
        case.u64_at("pinnedKey.lastHeight")
            .unwrap_or_else(|error| panic!("{error}")),
    )
}

fn verified(case: &JsonValue) -> Receipt {
    let pins = [pin(case)];
    verify_sequencer_signature(
        &bytes(case, "signedReceipt"),
        ReceiptKeys::HeightRanges(&pins),
    )
    .unwrap_or_else(|error| panic!("{error:?}"))
}

fn logs(case: &JsonValue) -> Vec<SequencerLog> {
    case.array_at("receiptJson.logs")
        .unwrap_or_else(|error| panic!("{error}"))
        .iter()
        .map(|log| {
            let topics = log
                .array_at("topics")
                .unwrap_or_else(|error| panic!("{error}"));
            SequencerLog::new(
                array(log, "address"),
                (0..topics.len())
                    .map(|index| array(log, &format!("topics.{index}")))
                    .collect(),
                bytes(log, "data"),
                quantity(log, "logIndex"),
            )
        })
        .collect()
}

#[test]
fn retained_positive_and_revert_receipts_verify_byte_exactly() {
    for (name, result_code, count) in [
        ("vector-v1-receipted-positive.json", 0, 5),
        ("vector-v2-receipted-revert.json", 1, 1),
    ] {
        let document = load(name);
        assert_eq!(cases(&document).len(), count);
        if result_code == 0 {
            assert_eq!(
                cases(&document)
                    .iter()
                    .map(|case| case.str_at("id").unwrap_or_else(|error| panic!("{error}")))
                    .collect::<Vec<_>>(),
                [
                    "V1-pax-transfer",
                    "V1-erc20-approve",
                    "V1-layerxledger-pay-exact",
                    "V1-layerxbridge-bridge-out",
                    "V1-layerxprograms-call",
                ]
            );
        }
        for case in cases(&document) {
            let canonical = bytes(case, "signedReceipt");
            let receipt = verified(case);
            let protocol = receipt
                .protocol()
                .unwrap_or_else(|| panic!("protocol receipt"));
            assert_eq!(protocol.protocol_version(), 2);
            assert_eq!(protocol.module_id(), 12);
            assert_eq!(protocol.result_code(), result_code);
            assert!(protocol.program_outcome().is_none());
            assert_eq!(protocol.activity_id(), array(case, "receiptJson.txHash"));
            assert_eq!(protocol.context_hash(), array(case, "pinnedKey.keyId"));
            assert_eq!(protocol.previous_state_root(), array(case, "prevSeqHash"));
            assert_eq!(
                protocol.activity_root(),
                <[u8; 32]>::from(Sha256::digest(bytes(case, "receiptJson.returnData")))
            );
            assert_eq!(encode(&receipt), Ok(canonical.clone()));
            assert_eq!(bytes(case, "receiptJson.receipt"), canonical);
            let unsigned = encode_unsigned(&receipt).unwrap_or_else(|error| panic!("{error:?}"));
            assert_eq!(receipt_digest(&unsigned), Ok(array(case, "receiptDigest")));
            let served = logs(case);
            let checked =
                verify_logs(&receipt, served.clone()).unwrap_or_else(|error| panic!("{error:?}"));
            assert_eq!(checked.logs(), served.as_slice());
            assert_eq!(
                usize::try_from(checked.summary().log_count()),
                Ok(served.len())
            );
            assert_eq!(
                checked.summary().gas_used(),
                quantity(case, "receiptJson.gasUsed")
            );
        }
    }
}

#[test]
fn retained_positive_mutated_transfer_leg_is_refused() {
    let document = load("vector-v1-receipted-positive.json");
    let case = cases(&document)
        .iter()
        .find(|case| case.str_at("id") == Ok("V1-layerxledger-pay-exact"))
        .unwrap_or_else(|| panic!("retained payExact positive vector"));
    let canonical = bytes(case, "signedReceipt");
    let receipt = verified(case);
    let served = logs(case);
    assert_eq!(served.len(), 1);
    let leg = &served[0];
    assert_eq!(leg.topics().len(), 4);
    assert_eq!(leg.data().len(), 64);
    assert!(verify_logs(&receipt, served.clone()).is_ok());
    for offset in 0..leg.data().len() {
        let mut data = leg.data().to_vec();
        data[offset] ^= 1;
        let changed =
            SequencerLog::new(leg.address(), leg.topics().to_vec(), data, leg.log_index());
        assert_eq!(
            verify_logs(&receipt, vec![changed]),
            Err(SequencerLogRefusal::Log {
                index: 0,
                field: LogField::Data
            })
        );
    }
    for topic in 0..leg.topics().len() {
        for offset in 0..32 {
            let mut topics = leg.topics().to_vec();
            topics[topic][offset] ^= 1;
            let changed =
                SequencerLog::new(leg.address(), topics, leg.data().to_vec(), leg.log_index());
            assert_eq!(
                verify_logs(&receipt, vec![changed]),
                Err(SequencerLogRefusal::Log {
                    index: 0,
                    field: LogField::Topics
                })
            );
        }
    }
    for offset in 0..20 {
        let mut address = leg.address();
        address[offset] ^= 1;
        let changed = SequencerLog::new(
            address,
            leg.topics().to_vec(),
            leg.data().to_vec(),
            leg.log_index(),
        );
        assert_eq!(
            verify_logs(&receipt, vec![changed]),
            Err(SequencerLogRefusal::Log {
                index: 0,
                field: LogField::Address
            })
        );
    }
    let protocol = receipt
        .protocol()
        .unwrap_or_else(|| panic!("protocol receipt"));
    let effect = protocol.effects()[1].body();
    let offsets: Vec<_> = canonical
        .windows(effect.len())
        .enumerate()
        .filter_map(|(offset, window)| (window == effect).then_some(offset))
        .collect();
    assert_eq!(offsets.len(), 1);
    let mut changed = canonical.clone();
    changed[offsets[0] + effect.len() - 1] ^= 1;
    assert!(decode(&changed).is_ok());
    let pins = [pin(case)];
    assert_eq!(
        verify_sequencer_signature(&changed, ReceiptKeys::HeightRanges(&pins))
            .err()
            .map(|error| error.check),
        Some(ReceiptCheck::SequencerSignature)
    );
    assert_eq!(encode(&verified(case)), Ok(canonical));
}

#[test]
fn retained_receipt_envelope_refuses_truncation_and_trailing_bytes() {
    let document = load("vector-v1-receipted-positive.json");
    for case in cases(&document) {
        let canonical = bytes(case, "signedReceipt");
        let pins = [pin(case)];
        assert!(verify_sequencer_signature(&canonical, ReceiptKeys::HeightRanges(&pins)).is_ok());
        for length in 0..canonical.len() {
            assert!(
                decode(&canonical[..length]).is_err(),
                "truncation at {length}"
            );
        }
        let mut changed = canonical.clone();
        changed.push(0);
        assert!(decode(&changed).is_err());
        changed = canonical.clone();
        changed[0] ^= 1;
        assert!(decode(&changed).is_err());
        changed = canonical;
        changed[5..9].copy_from_slice(&u32::MAX.to_be_bytes());
        assert!(decode(&changed).is_err());
    }
}

mod native_call_fixtures {
    //! Current native lifecycle fixtures carry signed activities, not terminal execution receipts.

    use std::path::Path;

    use layerx_crypto::ed25519;
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    use layerx_wire::activity::{decode_signed, encode_signed, Activity};
    use layerx_wire::hash::{activity_id, payload_hash};
    use layerx_wire::receipt::{decode, decode_applied_terminal};
    use layerx_wire::sign::preimage;
    use serde_json::Value;

    fn fixture(version: u8) -> (Value, Vec<u8>, ModuleRegistry) {
        let path = Path::new(env!("CARGO_MANIFEST_DIR")).join(format!(
            "../../../../sdk/platform/conformance/fixtures/native-program-call-v{version}.json"
        ));
        let document: Value = serde_json::from_str(
            &std::fs::read_to_string(path).unwrap_or_else(|error| panic!("fixture: {error}")),
        )
        .unwrap_or_else(|error| panic!("fixture JSON: {error}"));
        let kind = ActivityType::new(ModuleId::Programs, 3)
            .unwrap_or_else(|error| panic!("activity type: {error:?}"));
        let registration = ModuleRegistration::new(ModuleId::Programs, &[kind])
            .unwrap_or_else(|error| panic!("registration: {error:?}"));
        let registry = ModuleRegistry::new(&[registration])
            .unwrap_or_else(|error| panic!("registry: {error:?}"));
        let bytes = hex(&document, "signed_activity_hex");
        (document, bytes, registry)
    }

    fn hex(document: &Value, field: &str) -> Vec<u8> {
        let text = document[field]
            .as_str()
            .unwrap_or_else(|| panic!("field {field}"));
        assert_eq!(text.len() % 2, 0);
        text.as_bytes()
            .chunks_exact(2)
            .map(|pair| {
                let pair = std::str::from_utf8(pair).unwrap_or_else(|error| panic!("{error}"));
                u8::from_str_radix(pair, 16).unwrap_or_else(|error| panic!("{error}"))
            })
            .collect()
    }

    fn signature_is_valid(document: &Value, activity: &Activity) -> bool {
        let public_key: [u8; 32] = hex(document, "public_key_hex")
            .try_into()
            .unwrap_or_else(|_| panic!("public key width"));
        let signature: [u8; 64] = activity
            .signature()
            .unwrap_or_else(|| panic!("signature"))
            .try_into()
            .unwrap_or_else(|_| panic!("signature width"));
        let digest = preimage(activity).unwrap_or_else(|error| panic!("preimage: {error:?}"));
        ed25519::verify_digest(&public_key, &signature, digest.as_bytes()).is_ok()
    }

    #[test]
    fn current_native_call_fixtures_verify_signed_activity_bindings() {
        for version in [3, 4] {
            let (document, bytes, registry) = fixture(version);
            let activity =
                decode_signed(&bytes, &registry).unwrap_or_else(|error| panic!("{error:?}"));
            assert_eq!(activity.protocol_version(), 3);
            assert_eq!(activity.activity_type().module(), ModuleId::Programs);
            assert_eq!(activity.activity_type().ordinal(), 3);
            assert_eq!(activity.payload(), hex(&document, "payload_hex"));
            assert_eq!(payload_hash(&activity), Ok(activity.payload_hash()));
            assert_eq!(
                activity_id(&activity).map(|id| id.to_vec()),
                Ok(hex(&document, "activity_id_hex"))
            );
            assert_eq!(encode_signed(&activity), Ok(bytes));
            assert!(signature_is_valid(&document, &activity));
        }
    }

    #[test]
    fn current_native_call_fixtures_refuse_every_truncated_envelope() {
        for version in [3, 4] {
            let (_, bytes, registry) = fixture(version);
            for length in 0..bytes.len() {
                assert!(
                    decode_signed(&bytes[..length], &registry).is_err(),
                    "v{version}: {length}"
                );
            }
        }
    }

    #[test]
    fn current_native_call_fixtures_refuse_trailing_and_oversized_fields() {
        for version in [3, 4] {
            let (_, bytes, registry) = fixture(version);
            let mut changed = bytes.clone();
            changed.push(0);
            assert!(decode_signed(&changed, &registry).is_err());
            let mut changed = bytes;
            // Tag 4 at offset 18 precedes the four-byte actor DID length.
            changed[19..23].copy_from_slice(&u32::MAX.to_be_bytes());
            assert!(decode_signed(&changed, &registry).is_err());
        }
    }

    #[test]
    fn current_native_call_signatures_refuse_network_payload_and_signature_mutations() {
        for version in [3, 4] {
            let (document, bytes, registry) = fixture(version);
            let payload = hex(&document, "payload_hex");
            let payload_offset = bytes
                .windows(payload.len())
                .position(|window| window == payload)
                .unwrap_or_else(|| panic!("payload in envelope"));
            // The network ID occupies bytes 9..13; byte 13 is the next field tag.
            for offset in [12, payload_offset, bytes.len() - 1] {
                let mut changed = bytes.clone();
                changed[offset] ^= 1;
                let activity =
                    decode_signed(&changed, &registry).unwrap_or_else(|error| panic!("{error:?}"));
                // The signature binds the payload hash; payload tampering is refused by its separate hash binding.
                if offset == payload_offset {
                    assert_ne!(payload_hash(&activity), Ok(activity.payload_hash()));
                } else {
                    assert!(!signature_is_valid(&document, &activity));
                }
            }
        }
    }

    #[test]
    fn current_native_call_fixtures_cannot_be_used_as_terminal_execution_evidence() {
        for version in [3, 4] {
            let (document, bytes, _) = fixture(version);
            assert!(document.get("canonical_receipt_hex").is_none());
            assert!(document.get("terminal_payload_hex").is_none());
            assert!(decode(&bytes).is_err());
            assert!(decode_applied_terminal(&bytes).is_err());
            assert!(decode_applied_terminal(&hex(&document, "payload_hex")).is_err());
        }
    }
}
