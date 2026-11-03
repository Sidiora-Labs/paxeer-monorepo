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
        ("vector-v1-receipted-positive.json", 0, 4),
        ("vector-v2-receipted-revert.json", 1, 1),
    ] {
        let document = load(name);
        assert_eq!(cases(&document).len(), count);
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
