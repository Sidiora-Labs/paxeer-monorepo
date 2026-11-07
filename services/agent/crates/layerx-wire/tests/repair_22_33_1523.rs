//! Repair record 22.33.1523, audit 22.30.92, acceptance 22998.1.
//!
//! Finding -> proving test:
//! - wire_hash_tests -> wire_hash_tests_native_identity_and_scope
//! - wire_rejection_tests -> wire_rejection_tests_native_mutations_and_disclosures
//! - wire_structure_tests -> wire_structure_tests_native_round_trips
//! - wallet_vector_tests -> wallet_vector_tests_signed_calls_reach_node_consumer
//!
//! Baseline: no gate ran before implementation. The ordered implementation-first,
//! hf-done-only instruction supersedes do_8; this target did not exist at start.
//! No pre-change test failure is claimed. Qualification revision, command, exit
//! and private evidence are recorded by the runner in /out/run/gates.jsonl.
//!
//! Attempt 1 failed to build because the crypto custody modules and client crate
//! were unavailable to these tests (gotcha 3404). Attempt 2 includes Cargo.toml:
//! the crypto dev-dependency enables its default custody feature and the client
//! dev-dependency supplies the real node JSON-RPC client. No out-of-path wiring
//! remains; the workspace lockfile is refreshed with those dependencies.
//!
//! Historical kernel wallet vectors remain in wallet_vectors.rs under the
//! historical_compatibility module. They establish archive codec compatibility,
//! never present-day signing authority or chain finality. The loopback peer below
//! is a committed transport fixture; transaction and disclosure handling use
//! production code. It does not simulate consensus or claim committed execution.

use std::fmt::{Debug, Write as _};
use std::future::Future;
use std::io::{Read as _, Write as _};
use std::net::{TcpListener, TcpStream};
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::time::{Duration, Instant};

use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::authority_grant::{authority_grant_call, AuthorityGrant, GrantKind, GrantScope};
use layerx_crypto::disclosure::call::{
    bind_call, encode_bank_send, encode_program_call, ledger_selector, reencode, validate_call,
    validate_canonical, ConstructMethod,
};
use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::evm_transaction::{
    assemble, decode_signed, decode_unsigned, recover_sender, transaction_hash, Eip1559Call,
    EvmTransactionError,
};
use layerx_crypto::send::{ledger_payment_call, send_context_hash, CallOptions, SendDebit};
use layerx_crypto::signer::LocalSigner;
use layerx_types::intent::ProgramId;
use layerx_types::json;
use layerx_types::program_call::{NativeProgramCall, Resources};
use layerx_types::result::{KnownResult, ResultCode};
use layerx_types::vectors::checkpoint::{load_checkpoint_vectors, CHECKPOINT_VECTOR_CASES};
use layerx_types::vectors::CodecVector;
use layerx_wire::decode::Decoder;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{checkpoint_attestation_digest, checkpoint_id, Domain};
use layerx_wire::receipt::{
    decode, decode_batch_header, decode_checkpoint, decode_merkle_proof, encode,
    encode_batch_header, encode_checkpoint, encode_merkle_proof,
};
use sha2::{Digest as _, Sha256};

fn must<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("production path refused fixture: {error:?}"))
}

/// Reads committed text fixtures directly; no generated replay corpus or toolchain.
pub fn codec_vectors(text: &str) -> Vec<CodecVector> {
    text.lines()
        .filter(|line| !line.trim().is_empty() && !line.starts_with('#'))
        .map(|line| {
            let fields: Vec<_> = line.split('|').collect();
            assert_eq!(fields.len(), 5);
            CodecVector {
                kind: fields[0].to_owned(),
                name: fields[1].to_owned(),
                bytes: must(json::decode_hex(&format!("0x{}", fields[2]))),
                expected_result: ResultCode::from_raw(must(fields[3].parse())),
                expected_sha256: if fields[4] == "-" {
                    None
                } else {
                    Some(must(
                        must(json::decode_hex(&format!("0x{}", fields[4]))).try_into(),
                    ))
                },
            }
        })
        .collect()
}

#[test]
fn wire_rejection_tests_primitive_taxonomy_and_bounds() {
    let vectors = codec_vectors(include_str!(
        "../../../../../tests/vectors/codec/adversarial.lxv"
    ));
    let mut codes = std::collections::BTreeSet::new();
    for vector in vectors {
        let mut decoder = Decoder::new(&vector.bytes, 0);
        let result = match vector.kind.as_str() {
            "u64" => decoder.u64().and_then(|_| decoder.finish()),
            "tag" => decoder.tag(3).map(|_| ()),
            "bytes4" => decoder.bytes(4).map(|_| ()),
            "seq" => (|| {
                let length = usize::from(decoder.u8()?);
                let first = decoder.fixed(length)?;
                let length = usize::from(decoder.u8()?);
                let second = decoder.fixed(length)?;
                layerx_wire::check_ordered_keys(&[first, second])
            })(),
            kind => panic!("unhandled committed vector: {kind}"),
        };
        let error = result.unwrap_err();
        assert_eq!(error.result, vector.expected_result, "{}", vector.name);
        codes.insert(error.result.raw());
    }
    assert_eq!(
        codes,
        std::collections::BTreeSet::from([-6, -5, -4, -2, -1])
    );
    for maximum in [4_usize, 32, 64, 256, 512, 65_535] {
        let prefix = must(u32::try_from(maximum)).to_be_bytes();
        let bytes = [prefix.to_vec(), vec![0; maximum]].concat();
        let mut decoder = Decoder::new(&bytes, maximum);
        assert_eq!(must(decoder.bytes_owned(maximum)).len(), maximum);
        assert_eq!(decoder.allocated(), maximum);
        assert_eq!(decoder.finish(), Ok(()));
        let over = must(u32::try_from(maximum + 1)).to_be_bytes();
        let mut decoder = Decoder::new(&over, maximum);
        assert_eq!(
            decoder.bytes_owned(maximum).unwrap_err().result,
            KnownResult::LengthLimit.into()
        );
        assert_eq!(decoder.allocated(), 0);
    }
}

// Public, deterministic test key, also used by the production crypto codec tests.
pub const SENDER: [u8; 20] = [
    0x1a, 0x64, 0x2f, 0x0e, 0x3c, 0x3a, 0xf5, 0x45, 0xe7, 0xac, 0xbd, 0x38, 0xb0, 0x72, 0x51, 0xb3,
    0x99, 0x09, 0x14, 0xf1,
];

struct WakeLocal;
impl Wake for WakeLocal {
    fn wake(self: Arc<Self>) {}
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(WakeLocal));
    match future.as_mut().poll(&mut Context::from_waker(&waker)) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("in-process native signer unexpectedly pending"),
    }
}

fn target(suffix: u16) -> [u8; 20] {
    let mut address = [0; 20];
    address[18..].copy_from_slice(&suffix.to_be_bytes());
    address
}

fn word(value: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&value.to_be_bytes());
    word
}

pub fn native_call() -> Eip1559Call {
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 3,
        gas_limit: 100_000,
        to: target(0x1001),
        data: encode_bank_send(SENDER, [2; 20], "uhpx", word(50)),
    }
}

/// Five current examples produced by native constructors, with real signatures
/// applied by the caller. ABI argument words for budgetFund are fixture input.
pub fn native_vectors() -> Vec<(&'static str, Eip1559Call)> {
    let native = native_call();
    let signer = LocalSigner::new([7; 32]);
    let debit = SendDebit {
        from: [11; 32],
        to: [12; 32],
        asset: [13; 32],
        amount: 75_000,
        source_sequence: 0,
        idempotency_key: [14; 32],
        expires_at: 1_800_000_600,
        context_hash: send_context_hash(&[11; 32], &[12; 32], &[13; 32], 75_000, &[14; 32]),
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: 125,
        protocol_version: 3,
    };
    let payload = must(ready(debit.sign(&signer)));
    let options = CallOptions {
        from: SENDER,
        chain_id: 125,
        nonce: 8,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 3,
        gas_limit: 100_000,
    };
    let token = must(ledger_payment_call(&payload, &options)).call;
    let grant = AuthorityGrant {
        grantor: [1; 32],
        grantee: [1; 32],
        kind: GrantKind::DelegatedCapability,
        delegate_key: [3; 32],
        scope: GrantScope {
            module_mask: 1 << 1,
            ordinal_min: 5,
            ordinal_max: 5,
            asset: [13; 32],
            maximum_per_activity: 10,
            maximum_total: 100,
            period_length: 0,
            maximum_per_period: 0,
            period_start: 0,
            purpose: [4; 32],
        },
        fee_budget: None,
        not_before: 1,
        not_after: 100,
        revocation_sequence: 1,
    };
    let approval = must(authority_grant_call(&grant, &options)).call;
    let budget = Eip1559Call {
        to: target(0x101b),
        data: [
            ledger_selector("budgetFund(bytes32,uint128)").to_vec(),
            [5; 32].to_vec(),
            word(250_000).to_vec(),
        ]
        .concat(),
        ..native.clone()
    };
    let program = NativeProgramCall {
        program_id: ProgramId::new([6; 32]),
        guest_abi: 1,
        entrypoint: b"run",
        calldata: b"current wallet vector",
        capabilities: &[],
        access_declaration: &[],
        response_capacity: 1024,
        resources: Resources([100; 7]),
    };
    let action = Eip1559Call {
        to: target(0x101d),
        data: encode_program_call(&program, 40_000),
        ..native.clone()
    };
    vec![
        ("native-send", native),
        ("token-send", token),
        ("approval", approval),
        ("budget-change", budget),
        ("agent-action", action),
    ]
}

#[test]
pub fn wire_hash_tests_native_identity_and_scope() {
    let call = native_call();
    let raw = must(call.sign(&[1; 32]));
    let original_hash = transaction_hash(&raw);
    assert_eq!(raw, must(call.sign(&[1; 32])));
    let (decoded, signature) = must(decode_signed(&raw));
    assert_eq!(must(recover_sender(&decoded, &signature)), SENDER);
    assert_eq!(assemble(&decoded, &signature), raw);
    assert_eq!(
        transaction_hash(&assemble(&decoded, &signature)),
        original_hash
    );
    assert_ne!(call.signing_hash(), original_hash);
    for changed in [
        Eip1559Call {
            chain_id: 126,
            ..call.clone()
        },
        Eip1559Call {
            nonce: 8,
            ..call.clone()
        },
        Eip1559Call {
            gas_limit: 100_001,
            ..call.clone()
        },
        Eip1559Call {
            max_fee_per_gas: 4,
            ..call.clone()
        },
        Eip1559Call {
            max_priority_fee_per_gas: 1,
            ..call.clone()
        },
        Eip1559Call {
            to: target(0x101a),
            ..call.clone()
        },
        Eip1559Call {
            data: encode_bank_send(SENDER, [3; 20], "uhpx", word(50)),
            ..call.clone()
        },
    ] {
        assert_ne!(changed.signing_hash(), call.signing_hash());
        assert_ne!(
            transaction_hash(&must(changed.sign(&[1; 32]))),
            original_hash
        );
    }
    let mut unsupported = raw;
    unsupported[0] = 4;
    assert_eq!(
        decode_signed(&unsupported),
        Err(EvmTransactionError::NotEip1559)
    );
}

// Relist mutated fixture fields; this is test input construction, never a decoder.
fn relist(fields: &[u8]) -> Vec<u8> {
    let length = must(u16::try_from(fields.len())).to_be_bytes();
    let mut raw = if fields.len() < 56 {
        vec![2, 0xc0 + must(u8::try_from(fields.len()))]
    } else if length[0] == 0 {
        vec![2, 0xf8, length[1]]
    } else {
        vec![2, 0xf9, length[0], length[1]]
    };
    raw.extend_from_slice(fields);
    raw
}

#[test]
pub fn wire_rejection_tests_native_mutations_and_disclosures() {
    let call = Eip1559Call {
        data: vec![0xde, 0xad, 0xbe, 0xef],
        chain_id: 1,
        gas_limit: 30_000,
        ..native_call()
    };
    let raw = must(call.sign(&[1; 32]));
    assert_eq!(raw[1], 0xf8);
    let fields = &raw[3..];
    assert_eq!(relist(fields), raw);
    assert_eq!(fields[0], 1);
    assert_eq!(fields[7], 0x94);
    assert_eq!(fields[28], 0x80);
    assert_eq!(fields[34], 0xc0);
    for length in 1..raw.len() {
        assert_eq!(
            decode_signed(&raw[..length]),
            Err(EvmTransactionError::MalformedRlp)
        );
    }
    assert_eq!(decode_signed(&[]), Err(EvmTransactionError::NotEip1559));
    for kind in [0, 1, 3, 4, u8::MAX] {
        let mut bytes = raw.clone();
        bytes[0] = kind;
        assert_eq!(decode_signed(&bytes), Err(EvmTransactionError::NotEip1559));
    }
    let mut trailing = raw.clone();
    trailing.push(0);
    assert_eq!(
        decode_signed(&trailing),
        Err(EvmTransactionError::MalformedRlp)
    );
    for nonminimal in [&[0x81, 1][..], &[0x82, 0, 1], &[0]] {
        let malformed = [nonminimal, &fields[1..]].concat();
        assert_eq!(
            decode_signed(&relist(&malformed)),
            Err(EvmTransactionError::MalformedRlp)
        );
    }
    let wide = [&[0x89, 1, 0, 0, 0, 0, 0, 0, 0, 0][..], &fields[1..]].concat();
    assert_eq!(
        decode_signed(&relist(&wide)),
        Err(EvmTransactionError::UnsupportedField)
    );
    let mut value = fields.to_vec();
    value[28] = 1;
    assert_eq!(
        decode_signed(&relist(&value)),
        Err(EvmTransactionError::UnsupportedField)
    );
    let access = [&fields[..34], &[0xc1, 0xc0], &fields[35..]].concat();
    assert_eq!(
        decode_signed(&relist(&access)),
        Err(EvmTransactionError::UnsupportedField)
    );
    // Invalid parity, canonical zero r, and high s retain signature classifications.
    let (decoded, signature) = must(decode_signed(&raw));
    assert_eq!(decoded, call);
    assert_eq!(raw[raw.len() - 66], 0xa0);
    assert_eq!(raw[raw.len() - 33], 0xa0);
    for mutation in 0..3 {
        let mut bad = raw.clone();
        let length = bad.len();
        match mutation {
            0 => bad[length - 67] = 2,
            1 => {
                let fields = [&raw[3..length - 66], &[0x80], &raw[length - 33..]].concat();
                bad = relist(&fields);
            }
            _ => bad[length - 32..].fill(0xff),
        }
        assert_eq!(decode_signed(&bad), Err(EvmTransactionError::BadSignature));
    }
    assert_eq!(assemble(&call, &signature), raw);

    // Exercise the production decoder's exact unsigned custody limit, including
    // framing overhead; a one-byte larger input must retain its typed refusal.
    let maximum = 1_048_576;
    let mut bounded = native_call();
    bounded.data = vec![0; maximum];
    let overhead = bounded.encode_unsigned().len() - maximum;
    bounded.data.truncate(maximum - overhead);
    let exact = bounded.encode_unsigned();
    assert_eq!(exact.len(), maximum);
    assert_eq!(decode_unsigned(&exact), Ok(bounded.clone()));
    bounded.data.push(0);
    let over = bounded.encode_unsigned();
    assert_eq!(over.len(), maximum + 1);
    assert_eq!(
        decode_unsigned(&over),
        Err(EvmTransactionError::UnsupportedField)
    );

    for (_, call) in native_vectors() {
        let original = must(bind_call(&call, SENDER));
        assert_eq!(validate_call(&call, &original, SENDER), Ok(()));
        assert_eq!(
            validate_canonical(&original, &call.encode_unsigned()),
            Ok(())
        );
        let mut changed = original.clone();
        changed.fee_limit += 1;
        assert_eq!(
            validate_call(&call, &changed, SENDER),
            Err(DisclosureError::FieldMismatch("call"))
        );
        assert_eq!(
            validate_call(&call, &original, [9; 20]),
            Err(DisclosureError::FieldMismatch("call"))
        );
        let different = Eip1559Call {
            nonce: call.nonce + 1,
            ..call.clone()
        };
        assert_eq!(
            validate_canonical(&original, &different.encode_unsigned()),
            Err(DisclosureError::FieldMismatch("canonical_bytes"))
        );
        let mut unknown = call.clone();
        unknown.data[..4].fill(0xff);
        assert_eq!(
            bind_call(&unknown, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
        let mut extra = call;
        extra.data.push(0);
        assert_eq!(
            bind_call(&extra, SENDER),
            Err(DisclosureError::MalformedPayload)
        );
    }
}

#[test]
pub fn wire_structure_tests_native_round_trips() {
    for (name, call) in native_vectors() {
        let unsigned = call.encode_unsigned();
        assert_eq!(must(decode_unsigned(&unsigned)), call, "{name}");
        let raw = must(call.sign(&[1; 32]));
        let (received, signature) = must(decode_signed(&raw));
        assert_eq!(received, call, "{name}");
        assert_eq!(assemble(&received, &signature), raw, "{name}");
        assert_eq!(received.encode_unsigned(), unsigned, "{name}");
        assert_eq!(
            must(recover_sender(&received, &signature)),
            SENDER,
            "{name}"
        );
        let disclosure = must(bind_call(&received, SENDER));
        assert_eq!(must(reencode(&disclosure)), unsigned, "{name}");
        assert_eq!(
            validate_call(&received, &disclosure, SENDER),
            Ok(()),
            "{name}"
        );
        match (name, must(disclosure.construct_method())) {
            ("native-send", Some(ConstructMethod::BankSend { denom, amount, .. })) => {
                assert_eq!(denom, "uhpx");
                assert_eq!(amount, word(50));
            }
            ("token-send", Some(ConstructMethod::LedgerSend { payload })) => {
                assert!(!payload.is_empty())
            }
            ("approval", Some(ConstructMethod::AuthLifecycle { payload, .. })) => {
                assert!(!payload.is_empty())
            }
            ("budget-change", Some(ConstructMethod::PayBudgetFund { budget, amount })) => {
                assert_eq!(budget, [5; 32]);
                assert_eq!(amount, 250_000);
            }
            ("agent-action", Some(ConstructMethod::ProgramCall { call, fee_limit })) => {
                assert_eq!(call.program_id, ProgramId::new([6; 32]));
                assert_eq!(fee_limit, 40_000);
            }
            other => panic!("native constructor changed: {other:?}"),
        }
    }
    // Committed protocol receipt fixtures qualify independently of any activity decoder.
    for bytes in [
        include_bytes!("../../../../../tests/fixtures/asset/supply-pause.receipt").as_slice(),
        include_bytes!("../../../../../tests/fixtures/asset/supply-unpause.receipt").as_slice(),
    ] {
        let receipt = must(decode(bytes));
        assert_eq!(must(encode(&receipt)), bytes);
        for length in 0..bytes.len() {
            assert!(decode(&bytes[..length]).is_err());
        }
    }
}

#[test]
fn wire_hash_tests_checkpoint_literals_and_headers() {
    assert_eq!(
        Domain::CheckpointCertificate.tag(),
        b"LXP/v2/checkpoint-certificate\0"
    );
    assert_eq!(
        Domain::GuarantorAttestation.tag(),
        b"LXP/v2/guarantor-attestation\0"
    );
    let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let vectors = must(load_checkpoint_vectors(&root));
    assert_eq!(vectors.len(), CHECKPOINT_VECTOR_CASES.len());
    for vector in vectors {
        let bytes = &vector.header.bytes;
        assert_eq!(bytes.len(), 354);
        assert_eq!(&bytes[..5], &[0, 2, 0x17, 1, 15]);
        let header = must(decode_batch_header(bytes));
        assert_eq!(header.protocol_version(), 2);
        assert_eq!(header.network_id(), vector.header.network_id);
        assert_eq!(header.epoch(), vector.header.epoch);
        assert_eq!(header.batch_number(), vector.header.batch_number);
        assert_eq!(header.first_sequence(), vector.header.first_sequence);
        assert_eq!(header.last_sequence(), vector.header.last_sequence);
        assert_eq!(
            header.previous_state_root(),
            vector.header.previous_state_root
        );
        assert_eq!(
            header.resulting_state_root(),
            vector.header.resulting_state_root
        );
        assert_eq!(
            header.activity_merkle_root(),
            vector.header.activity_merkle_root
        );
        assert_eq!(
            header.receipt_merkle_root(),
            vector.header.receipt_merkle_root
        );
        assert_eq!(header.event_merkle_root(), vector.header.event_merkle_root);
        assert_eq!(
            header.data_availability_root(),
            vector.header.data_availability_root
        );
        assert_eq!(header.oracle_root(), vector.header.oracle_root);
        assert_eq!(header.timestamp_ms(), vector.header.timestamp_ms);
        assert_eq!(header.sequencer_id(), vector.header.sequencer_id);
        assert_eq!(must(encode_batch_header(&header)), *bytes);
        let mut independent = Sha256::new();
        independent.update(b"LXP/v2/checkpoint-certificate\0");
        independent.update(bytes);
        independent.update(must(u32::try_from(vector.validity_proof.len())).to_be_bytes());
        independent.update(&vector.validity_proof);
        assert_eq!(
            <[u8; 32]>::from(independent.finalize()),
            vector.expected_digest
        );
        assert_eq!(
            checkpoint_id(bytes, &vector.validity_proof),
            Ok(vector.expected_digest)
        );
        for attestation in vector.attestations {
            let mut independent = Sha256::new();
            independent.update(b"LXP/v2/guarantor-attestation\0");
            independent.update(attestation.message);
            assert_eq!(<[u8; 32]>::from(independent.finalize()), attestation.digest);
            assert_eq!(
                checkpoint_attestation_digest(&attestation.message),
                Ok(attestation.digest)
            );
        }
    }
}

#[test]
fn wire_structure_tests_proofs_headers_and_checkpoints() {
    let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let vectors = must(load_checkpoint_vectors(&root));
    let mut header = vectors[0].header.bytes.clone();
    // Version-one certificate is a retained archive codec, independent of activity.
    header[..2].copy_from_slice(&1_u16.to_be_bytes());
    header[6..8].copy_from_slice(&1_u16.to_be_bytes());
    let decoded = must(decode_batch_header(&header));
    assert_eq!(must(encode_batch_header(&decoded)), header);
    let mut proof = Encoder::new(64);
    must(proof.structure_header(0x4d50));
    must(proof.u32(0));
    must(proof.u32(2));
    must(proof.u8(1));
    must(proof.bytes(&[3; 32], 1024));
    let proof = proof.finish();
    assert_eq!(
        must(encode_merkle_proof(&must(decode_merkle_proof(&proof)))),
        proof
    );
    let mut checkpoint = Encoder::new(2048);
    must(checkpoint.fixed(&header));
    must(checkpoint.bytes(&[1, 2, 3], 1_048_576));
    must(checkpoint.sequence_length(1, 32));
    must(checkpoint.bytes(&[4; 32], 32));
    must(checkpoint.bytes(&[5; 64], 64));
    must(checkpoint.u32(1));
    must(checkpoint.bytes(&[6, 7], 1024));
    let checkpoint = checkpoint.finish();
    assert_eq!(
        must(encode_checkpoint(&must(decode_checkpoint(&checkpoint)))),
        checkpoint
    );
    for length in 0..checkpoint.len() {
        assert!(decode_checkpoint(&checkpoint[..length]).is_err());
    }
    let mut trailing = checkpoint;
    trailing.push(0);
    assert!(decode_checkpoint(&trailing).is_err());
}

fn hex(bytes: &[u8]) -> String {
    let mut value = String::from("0x");
    for byte in bytes {
        must(write!(&mut value, "{byte:02x}"));
    }
    value
}

fn request(stream: &mut TcpStream) -> json::JsonValue {
    must(stream.set_read_timeout(Some(Duration::from_secs(5))));
    must(stream.set_write_timeout(Some(Duration::from_secs(5))));
    let mut bytes = Vec::new();
    let mut byte = [0];
    while !bytes.ends_with(b"\r\n\r\n") {
        assert!(bytes.len() < 8192, "bounded HTTP header");
        must(stream.read_exact(&mut byte));
        bytes.push(byte[0]);
    }
    let header = must(std::str::from_utf8(&bytes));
    assert!(header.starts_with("POST / HTTP/1.1\r\n"));
    let length = header
        .lines()
        .filter_map(|line| line.split_once(':'))
        .find(|(key, _)| key.eq_ignore_ascii_case("content-length"))
        .map(|(_, length)| must(length.trim().parse::<usize>()))
        .unwrap_or_else(|| panic!("missing HTTP length"));
    assert!(length <= 1_048_576);
    let mut body = vec![0; length];
    must(stream.read_exact(&mut body));
    must(json::parse(must(std::str::from_utf8(&body))))
}

#[test]
pub fn wallet_vector_tests_signed_calls_reach_node_consumer() {
    let vectors = native_vectors();
    assert_eq!(
        vectors.iter().map(|(name, _)| *name).collect::<Vec<_>>(),
        [
            "native-send",
            "token-send",
            "approval",
            "budget-change",
            "agent-action"
        ]
    );
    let signed: Vec<_> = vectors
        .iter()
        .map(|(_, call)| must(call.sign(&[1; 32])))
        .collect();
    let expected = signed.clone();
    let listener = must(TcpListener::bind(("127.0.0.1", 0)));
    let address = must(listener.local_addr());
    must(listener.set_nonblocking(true));
    let peer = std::thread::spawn(move || {
        for raw in expected {
            let deadline = Instant::now() + Duration::from_secs(5);
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(
                            Instant::now() < deadline,
                            "node client did not submit vector"
                        );
                        std::thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("loopback node: {error}"),
                }
            };
            let request = request(&mut stream);
            assert_eq!(must(request.str_at("jsonrpc")), "2.0");
            assert_eq!(must(request.str_at("method")), "eth_sendRawTransaction");
            assert_eq!(must(request.array_at("params")).len(), 1);
            assert_eq!(must(request.hex_at("params.0")), raw);
            // The consumer decodes transported bytes and independently rebinds the sender.
            let (received, signature) = must(decode_signed(&raw));
            let sender = must(recover_sender(&received, &signature));
            assert_eq!(sender, SENDER);
            let disclosure = must(bind_call(&received, sender));
            assert_eq!(validate_call(&received, &disclosure, sender), Ok(()));
            let body = format!(
                "{{\"jsonrpc\":\"2.0\",\"id\":{},\"result\":\"{}\"}}",
                must(request.u64_at("id")),
                hex(&transaction_hash(&raw))
            );
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()));
        }
    });
    let endpoint = must(SequencerEndpoint::new_node(
        &format!("http://{address}/"),
        &[],
    ));
    for raw in signed {
        assert_eq!(
            endpoint.send_raw_transaction(&raw),
            Ok(transaction_hash(&raw))
        );
    }
    peer.join()
        .unwrap_or_else(|error| std::panic::resume_unwind(error));
}
