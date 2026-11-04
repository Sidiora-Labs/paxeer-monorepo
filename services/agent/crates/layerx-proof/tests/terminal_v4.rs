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

mod program_call_transactions {
    use super::*;
    use layerx_programs_runtime::crypto::{hash_bytes, HashAlgorithm};
    use layerx_proof::level::achieved_rung;
    use layerx_proof::program::{
        verify_program_execution, ProgramExecutionCheck, ProgramExecutionExpectation,
    };
    use layerx_proof::receipt::{verify_outcome, AuthorizedBatch};
    use layerx_types::intent::{ProgramCallOutcome, ProgramId};
    use layerx_types::payload::{
        ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload,
    };
    use layerx_types::program_call::{InvalidNativeCall, NativeProgramCall, Resources};
    use layerx_types::rung::Rung;
    use layerx_wire::hash::payload_hash_for;
    use layerx_wire::receipt::decode_applied_terminal;

    fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
        value.unwrap_or_else(|error| panic!("program-call producer: {error:?}"))
    }

    fn unhex(text: &str) -> Vec<u8> {
        assert_eq!(text.len() % 2, 0);
        text.as_bytes()
            .chunks_exact(2)
            .map(|pair| must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16)))
            .collect()
    }

    fn fixed(text: &str) -> [u8; 32] {
        must(unhex(text).try_into())
    }

    fn program_call() -> NativeProgramCall<'static> {
        NativeProgramCall {
            program_id: ProgramId::new(fixed(
                "865c09696f3a22198f922f38291b97aaa48acc49035887349a1788decb583bb6",
            )),
            guest_abi: 1,
            entrypoint: b"layerx_call",
            calldata: &[],
            capabilities: &[0, 0],
            access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
            response_capacity: 64,
            resources: Resources([
                1_000_000, 16_777_216, 1_048_576, 1_048_576, 64, 1_048_576, 4096,
            ]),
        }
    }

    fn payload_hash(bytes: &[u8]) -> [u8; 32] {
        let kind = must(ActivityType::new(ModuleId::Programs, 3));
        let registration = must(ModuleRegistration::new(ModuleId::Programs, &[kind]));
        let registry = must(ModuleRegistry::new(&[registration]));
        must(payload_hash_for(&must(Payload::new(
            &registry, kind, bytes,
        ))))
    }

    fn transaction_case(document: &JsonValue) -> &JsonValue {
        cases(document)
            .iter()
            .find(|case| case.str_at("id") == Ok("V1-layerxprograms-call"))
            .unwrap_or_else(|| panic!("recorded program-call transaction"))
    }

    #[test]
    fn program_call_transaction_binds_pinned_identity_and_payload_hash() {
        let document = load("vector-v1-receipted-positive.json");
        let case = transaction_case(&document);
        let raw = bytes(case, "rawTransaction");
        let tx_hash = fixed("ca7dcc65ea0997273944305fbb282c9926c53a1fd801cf5f442c550b46579674");
        assert_eq!(must(hash_bytes(HashAlgorithm::Keccak256, &raw)), tx_hash);
        assert_eq!(array::<32>(case, "receiptJson.txHash"), tx_hash);
        assert_eq!(
            array::<20>(case, "receiptJson.to"),
            [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1d]
        );
        let call = program_call();
        let payload = must(call.encode());
        assert_eq!(payload.len(), 158);
        assert_eq!(must(NativeProgramCall::decode(&payload)), call);
        assert_eq!(
            payload_hash(&payload),
            fixed("e7b220f3ed688a175ad35b983099ea7958626d06db8956faa1724683556cdf77")
        );
        // This committed transaction is the real node producer's transcript.
        // Pin its ABI fields to the native call the production codec emits.
        let data = &raw[46..46 + 740];
        assert_eq!(&data[..4], &[0x54, 0xbd, 0x2b, 0x48]);
        let args = &data[4..];
        assert_eq!(&args[..32], &call.program_id.bytes());
        let word = |index: usize| -> u64 {
            assert!(args[index * 32..index * 32 + 24]
                .iter()
                .all(|byte| *byte == 0));
            u64::from_be_bytes(must(args[index * 32 + 24..(index + 1) * 32].try_into()))
        };
        assert_eq!(word(1), u64::from(call.guest_abi));
        assert_eq!(word(6), u64::from(call.response_capacity));
        for (index, ceiling) in call.resources.0.iter().enumerate() {
            assert_eq!(word(index + 7), *ceiling);
        }
        assert_eq!(word(14), 100_000_000);
        for (index, body) in [
            (2, call.entrypoint),
            (3, call.calldata),
            (4, call.capabilities),
            (5, call.access_declaration),
        ] {
            let offset = must(usize::try_from(word(index)));
            let length = must(usize::try_from(u64::from_be_bytes(must(
                args[offset + 24..offset + 32].try_into(),
            ))));
            assert_eq!(&args[offset + 32..offset + 32 + length], body);
        }
        let pins = [pin(case)];
        let canonical = bytes(case, "signedReceipt");
        let receipt = verified(case);
        let protocol = receipt
            .protocol()
            .unwrap_or_else(|| panic!("protocol receipt"));
        assert_eq!(protocol.activity_id(), tx_hash);
        let batch = must(AuthorizedBatch::from_receipt_keys(
            &canonical,
            &pins,
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
        ));
        let outcome = must(verify_outcome(&canonical, &batch));
        assert_eq!(achieved_rung(outcome.evidence()), Rung::Receipted);
        assert_eq!(must(verify_logs(&receipt, logs(case))).logs(), logs(case));
        assert_eq!(encode(&receipt), Ok(canonical));
    }

    #[test]
    fn program_call_payload_refuses_every_truncation() {
        let payload = must(program_call().encode());
        assert_eq!(must(NativeProgramCall::decode(&payload)), program_call());
        for length in 0..payload.len() {
            assert_eq!(
                NativeProgramCall::decode(&payload[..length]),
                Err(InvalidNativeCall),
                "{length}"
            );
        }
    }

    #[test]
    fn program_call_payload_refuses_trailing_and_oversized_fields() {
        let payload = must(program_call().encode());
        assert_eq!(
            must(NativeProgramCall::decode(&payload)).encode(),
            Ok(payload.clone())
        );
        let mut changed = payload.clone();
        changed.push(0);
        assert_eq!(NativeProgramCall::decode(&changed), Err(InvalidNativeCall));
        let mut changed = payload;
        changed[36..40].copy_from_slice(&u32::MAX.to_be_bytes());
        assert_eq!(NativeProgramCall::decode(&changed), Err(InvalidNativeCall));
    }

    #[test]
    fn program_call_transaction_refuses_identity_payload_and_signature_substitution() {
        let document = load("vector-v1-receipted-positive.json");
        let case = transaction_case(&document);
        let raw = bytes(case, "rawTransaction");
        let identity = array::<32>(case, "receiptJson.txHash");
        assert_eq!(must(hash_bytes(HashAlgorithm::Keccak256, &raw)), identity);
        for offset in [4, 50, raw.len() - 1] {
            let mut changed = raw.clone();
            changed[offset] ^= 1;
            assert_ne!(
                must(hash_bytes(HashAlgorithm::Keccak256, &changed)),
                identity
            );
        }
        let payload = must(program_call().encode());
        let expected = fixed("e7b220f3ed688a175ad35b983099ea7958626d06db8956faa1724683556cdf77");
        assert_eq!(payload_hash(&payload), expected);
        for offset in 0..payload.len() {
            let mut changed = payload.clone();
            changed[offset] ^= 1;
            assert_ne!(payload_hash(&changed), expected);
        }
        let canonical = bytes(case, "signedReceipt");
        let pins = [pin(case)];
        assert_eq!(encode(&verified(case)), Ok(canonical.clone()));
        let mut changed = canonical.clone();
        let last = changed.len() - 1;
        changed[last] ^= 1;
        assert_eq!(
            verify_sequencer_signature(&changed, ReceiptKeys::HeightRanges(&pins))
                .err()
                .map(|failure| failure.check),
            Some(ReceiptCheck::SequencerSignature)
        );
        let wrong = [SequencerAuthorization::new(
            pins[0].sequencer_id(),
            [0x77; 32],
            1000,
            1999,
        )];
        assert_eq!(
            verify_sequencer_signature(&canonical, ReceiptKeys::HeightRanges(&wrong))
                .err()
                .map(|failure| failure.check),
            Some(ReceiptCheck::SequencerSignature)
        );
    }

    fn execution_fixture(name: &str) -> Vec<u8> {
        let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("../../../../tests/fixtures/programs/emulator-response-code");
        unhex(must(std::fs::read_to_string(root.join(name))).trim())
    }

    fn executed_expectation() -> ProgramExecutionExpectation {
        let call = NativeProgramCall {
            program_id: ProgramId::new([0x71; 32]),
            guest_abi: 2,
            entrypoint: b"layerx_call",
            calldata: b"echo",
            capabilities: &[0, 0],
            access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
            response_capacity: 4096,
            resources: Resources([100_000, 65_536, 0, 0, 2, 4096, 0]),
        };
        let payload = must(call.encode());
        let expected_hash =
            fixed("c208b24dd53c657a59810dee53186bafe390c85b74f409833eadc660654be583");
        assert_eq!(payload_hash(&payload), expected_hash);
        ProgramExecutionExpectation {
            sequencer_public_key: must(execution_fixture("sequencer-public.hex").try_into()),
            previous_state_root: fixed(
                "84c2029a102fd72553719bc187b9e8d2f4acf77919fe63b72430184ad2463c6d",
            ),
            activity_id: fixed("a790ebe72b3431e6bd06d85a26db8809c73b2f713102b47923208a76d80e0eab"),
            payload_hash: expected_hash,
            program_id: call.program_id.bytes(),
            guest_abi_version: call.guest_abi,
        }
    }

    #[test]
    fn program_call_transaction_is_distinct_from_executed_terminal_evidence() {
        let document = load("vector-v1-receipted-positive.json");
        let case = transaction_case(&document);
        for bytes in [bytes(case, "rawTransaction"), must(program_call().encode())] {
            assert!(decode(&bytes).is_err());
            assert!(decode_applied_terminal(&bytes).is_err());
        }
        let receipt = execution_fixture("receipt.hex");
        let terminal = execution_fixture("terminal.hex");
        let graph = execution_fixture("call-graph.hex");
        let expected = executed_expectation();
        let checked = must(verify_program_execution(
            &receipt, &terminal, &graph, expected,
        ));
        assert_eq!(achieved_rung(checked.receipt().evidence()), Rung::Submitted);
        assert_eq!(checked.result_code(), 0);
        let ProgramCallOutcome::Completed(response) = checked.outcome() else {
            panic!("executed terminal")
        };
        assert_eq!(response.code(), 7);
        assert_eq!(
            checked.terminal_payload_root(),
            <[u8; 32]>::from(Sha256::digest(&terminal))
        );
        let mut wrong = expected;
        wrong.activity_id[0] ^= 1;
        assert_eq!(
            verify_program_execution(&receipt, &terminal, &graph, wrong)
                .err()
                .map(|failure| failure.check),
            Some(ProgramExecutionCheck::Activity)
        );
        for offset in 0..terminal.len() {
            let mut changed = terminal.clone();
            changed[offset] ^= 1;
            assert_eq!(
                verify_program_execution(&receipt, &changed, &graph, expected)
                    .err()
                    .map(|failure| failure.check),
                Some(ProgramExecutionCheck::TerminalPayload),
                "terminal byte {offset}"
            );
        }
    }
}
