use ed25519_dalek::{Signer as _, SigningKey as EdSigningKey};
use k256::ecdsa::{Signature, SigningKey};
use layerx_crypto::secp256k1;
use layerx_proof::availability::{AvailabilityClass, Chunk};
use layerx_proof::export::{
    verify_complete, CompleteExportError, IndependentOfflineTrust, OfflineTrustError,
};
use layerx_proof::export_codec::{
    parse_fact_set, AccountStateRecord, AccountStateVariant, ArtifactDecodeError,
    CompleteOfflineArtifact, ExportCodecError, FactRefError, FactSelector, HeaderRecord,
    InclusionRecord, InclusionRecordKind, ProofRecord, ReceiptRecord, MAX_FACT_REFS,
    MAX_FACT_REF_BYTES, MAX_RECORD_BYTES, RECORD_VERSION,
};
use layerx_proof::merkle::MerkleError;
use layerx_proof::merkle::{build_leaf_hash_proof, build_proof, encode_proof, Proof};
use layerx_proof::signed_authority::SignedAuthorityHistory;
use layerx_proof::state::AccountProofError;
use layerx_types::rung::Rung;
use layerx_wire::encode::Encoder;
use layerx_wire::handover::{decode_genesis_trust, sequencer_id};
use layerx_wire::hash::{
    availability_chunk_digest, batch_header_digest, program_execution_batch_id, receipt_digest,
};
use layerx_wire::limits::{PROTOCOL_VERSION, STATE_COMMITMENT_PROTOCOL_VERSION};
use layerx_wire::receipt::{decode as decode_receipt, decode_batch_header, encode_unsigned};
use sha2::{Digest as _, Sha256};

const GENESIS: &[u8] = include_bytes!("fixtures/signed-authority/genesis.bin");
const HEADERS: &[u8] = include_bytes!("fixtures/signed-authority/headers.bin");
const HANDOVER_PAYLOAD: &[u8] = include_bytes!("fixtures/signed-authority/handover.payload");
const HEADER_RECORD: usize = 418;
const HEADER_BYTES: usize = 354;
const NO_RECORDS: [Vec<u8>; 0] = [];

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("export fixture: {error:?}"))
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn id(value: u8) -> [u8; 32] {
    let mut out = [value; 32];
    out[0] = 0xa0 | (value & 0x0f);
    out
}

fn receipt_ref(activity: [u8; 32]) -> String {
    format!("receipt:{}", hex(&activity))
}

fn lp(out: &mut Vec<u8>, bytes: &[u8]) {
    out.extend_from_slice(&must(u32::try_from(bytes.len())).to_be_bytes());
    out.extend_from_slice(bytes);
}

// Signed native receipt in the layout the receipt verifier decodes.
fn receipt(activity: [u8; 32], signer: &EdSigningKey) -> Vec<u8> {
    receipt_with_batch(activity, signer, [2; 32], [4; 32], PROTOCOL_VERSION)
}

fn receipt_with_batch(
    activity: [u8; 32],
    signer: &EdSigningKey,
    previous_state_root: [u8; 32],
    batch_id: [u8; 32],
    protocol_version: u16,
) -> Vec<u8> {
    receipt_with_state(
        activity,
        signer,
        previous_state_root,
        [3; 32],
        batch_id,
        protocol_version,
        9,
    )
}

fn receipt_with_state(
    activity: [u8; 32],
    signer: &EdSigningKey,
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
    batch_id: [u8; 32],
    protocol_version: u16,
    sequence: u64,
) -> Vec<u8> {
    let encode = |signature: Option<[u8; 64]>| {
        let mut encoder = Encoder::new(4096);
        assert_eq!(
            encoder.structure_header_version(0x5201, protocol_version),
            Ok(())
        );
        assert_eq!(encoder.u16(protocol_version), Ok(()));
        assert_eq!(encoder.bytes(&activity, 32), Ok(()));
        assert_eq!(encoder.u64(sequence), Ok(()));
        assert_eq!(encoder.bytes(&previous_state_root, 32), Ok(()));
        assert_eq!(encoder.bytes(&resulting_state_root, 32), Ok(()));
        assert_eq!(encoder.bytes(&[8; 32], 32), Ok(()));
        assert_eq!(encoder.i32(0), Ok(()));
        assert_eq!(encoder.sequence_length(0, 512), Ok(()));
        assert_eq!(encoder.u128(1), Ok(()));
        assert_eq!(encoder.bytes(&batch_id, 32), Ok(()));
        assert_eq!(encoder.u16(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u8(1), Ok(()));
        assert_eq!(encoder.bytes(&[5; 32], 32), Ok(()));
        assert_eq!(encoder.u128(25), Ok(()));
        assert_eq!(encoder.bytes(&[6; 32], 32), Ok(()));
        assert_eq!(encoder.u128(100), Ok(()));
        assert_eq!(encoder.u128(75), Ok(()));
        assert_eq!(encoder.u64(1), Ok(()));
        assert_eq!(encoder.bytes(&[7; 32], 32), Ok(()));
        assert_eq!(encoder.u128(10), Ok(()));
        assert_eq!(encoder.u128(35), Ok(()));
        assert_eq!(encoder.bytes(&[9; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[10; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[11; 32], 32), Ok(()));
        assert_eq!(encoder.u64(1_000), Ok(()));
        assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
        if let Some(value) = signature {
            assert_eq!(encoder.bytes(&value, 64), Ok(()));
        }
        encoder.finish()
    };
    let digest = must(receipt_digest(&encode(None)));
    let signed = encode(Some(signer.sign(&digest).to_bytes()));
    must(layerx_wire::receipt::encode(&must(decode_receipt(&signed))))
}

fn ed_key(value: u8) -> EdSigningKey {
    EdSigningKey::from_bytes(&[value; 32])
}

struct Native {
    history: SignedAuthorityHistory,
    registry: layerx_types::payload::ModuleRegistry,
    first_key: [u8; 32],
}

// Trust is built only from the checked-in native genesis and its signed headers.
fn native() -> Native {
    let genesis = must(decode_genesis_trust(GENESIS));
    let mut history = must(SignedAuthorityHistory::from_genesis(
        genesis.network_id,
        genesis.canonical_state_root,
        genesis.initial_sequencer_key,
        genesis.governance_witness,
    ));
    for (index, record) in HEADERS.chunks_exact(HEADER_RECORD).enumerate() {
        let packet = (index >= 13).then_some(HANDOVER_PAYLOAD);
        must(history.advance(
            &record[..HEADER_BYTES],
            &must(record[HEADER_BYTES..].try_into()),
            packet,
        ));
    }
    Native {
        history,
        registry: genesis.registry,
        first_key: genesis.initial_sequencer_key,
    }
}

fn guarantor(value: u8) -> (SigningKey, [u8; 33], [u8; 32]) {
    let mut scalar = [0_u8; 32];
    scalar[31] = value;
    let signing = must(SigningKey::from_bytes((&scalar).into()));
    let public_key: [u8; 33] = must(
        signing
            .verifying_key()
            .to_encoded_point(true)
            .as_bytes()
            .try_into(),
    );
    let mut guarantor_id = [0_u8; 32];
    guarantor_id[0] = value;
    (signing, public_key, guarantor_id)
}

fn trust() -> IndependentOfflineTrust {
    let native = native();
    must(IndependentOfflineTrust::new(
        native.registry,
        native.history,
    ))
}

// Header record over the first real signed header with the trusted interval key.
fn header_record(index: usize, public_key: [u8; 32], first: u64, last: u64) -> HeaderRecord {
    let record = &HEADERS[index * HEADER_RECORD..(index + 1) * HEADER_RECORD];
    HeaderRecord {
        canonical_header: record[..HEADER_BYTES].to_vec(),
        signature: must(record[HEADER_BYTES..].try_into()),
        sequencer_id: must(sequencer_id(&public_key)),
        public_key,
        first_batch: first,
        last_batch: last,
    }
}

fn header_bytes(record: &HeaderRecord) -> Vec<u8> {
    let mut out = b"LXHD".to_vec();
    out.push(1);
    lp(&mut out, &record.canonical_header);
    out.extend_from_slice(&record.signature);
    out.extend_from_slice(&record.sequencer_id);
    out.extend_from_slice(&record.public_key);
    out.extend_from_slice(&record.first_batch.to_be_bytes());
    out.extend_from_slice(&record.last_batch.to_be_bytes());
    out
}

fn receipt_record_bytes(activity: [u8; 32], receipt: &[u8], sequencer: [u8; 32]) -> Vec<u8> {
    let mut out = b"LXRF".to_vec();
    out.push(1);
    lp(&mut out, receipt_ref(activity).as_bytes());
    lp(&mut out, receipt);
    let decoded = must(decode_receipt(receipt));
    let protocol = decoded.protocol().expect("protocol receipt fixture");
    out.extend_from_slice(&protocol.batch_id());
    out.extend_from_slice(&protocol.asset());
    out.extend_from_slice(&protocol.previous_state_root());
    out.extend_from_slice(&protocol.resulting_state_root());
    out.extend_from_slice(&sequencer);
    let unsigned_digest = must(receipt_digest(&must(encode_unsigned(&decoded))));
    out.extend_from_slice(&unsigned_digest);
    out
}

fn single_proof() -> Proof {
    must(Proof::new(0, 1, Vec::new()))
}

fn pair_proof(index: u32) -> Proof {
    must(Proof::new(index, 2, vec![[0x31; 32]]))
}

fn inclusion_bytes(
    kind: u8,
    reference: &str,
    leaf: &[u8],
    proof: &Proof,
    digest: [u8; 32],
) -> Vec<u8> {
    let mut out = b"LXIP".to_vec();
    out.push(1);
    out.push(kind);
    lp(&mut out, reference.as_bytes());
    lp(&mut out, leaf);
    lp(&mut out, &encode_proof(proof));
    out.extend_from_slice(&digest);
    out
}

fn account_state_bytes(
    reference: &str,
    variant: u8,
    receipt: &[u8],
    digest: [u8; 32],
    tail: Option<(u32, u32, [u8; 32])>,
) -> Vec<u8> {
    let mut out = b"LXAS".to_vec();
    out.push(1);
    lp(&mut out, reference.as_bytes());
    out.push(variant);
    out.extend_from_slice(&id(0x22));
    lp(&mut out, &[0x41; 48]);
    out.extend_from_slice(&[0x42; 32]);
    out.extend_from_slice(&[0x43; 32]);
    out.extend_from_slice(&[0x44; 32]);
    lp(&mut out, &encode_proof(&pair_proof(0)));
    lp(&mut out, &encode_proof(&pair_proof(1)));
    lp(&mut out, &encode_proof(&single_proof()));
    lp(&mut out, receipt);
    lp(&mut out, &encode_proof(&pair_proof(1)));
    out.extend_from_slice(&digest);
    if let Some((count, version, dependency)) = tail {
        out.extend_from_slice(&count.to_be_bytes());
        out.extend_from_slice(&version.to_be_bytes());
        out.extend_from_slice(&dependency);
    }
    out
}

// Frozen historical wire bytes; no retired proof API is needed to test refusal.
fn attestation(guarantor_value: u8) -> Vec<u8> {
    let (signing, _, guarantor_id) = guarantor(guarantor_value);
    let checkpoint = [0x61; 32];
    let attested_at_ms = 1_000 + u64::from(guarantor_value);
    let mut message = [0_u8; 189];
    message[..2].copy_from_slice(&PROTOCOL_VERSION.to_be_bytes());
    message[2..6].copy_from_slice(&42_u32.to_be_bytes());
    message[6..14].copy_from_slice(&31_337_u64.to_be_bytes());
    message[14..34].copy_from_slice(&[0x55; 20]);
    message[34..42].copy_from_slice(&7_u64.to_be_bytes());
    message[42..74].copy_from_slice(&checkpoint);
    message[74..106].copy_from_slice(&checkpoint);
    message[106..138].copy_from_slice(&guarantor_id);
    message[138..146].copy_from_slice(&8_u64.to_be_bytes());
    message[146..178].copy_from_slice(&[12; 32]);
    message[178] = 1;
    message[179] = 1;
    message[180] = 0x1f;
    message[181..].copy_from_slice(&attested_at_ms.to_be_bytes());
    let digest = must(layerx_wire::hash::checkpoint_attestation_digest(&message));
    let (signature, recovery): (Signature, _) = must(signing.sign_prehash_recoverable(&digest));
    let signer = must(secp256k1::evm_address(
        signing.verifying_key().to_encoded_point(true).as_bytes(),
    ));
    let mut bytes = message.to_vec();
    bytes.extend_from_slice(&signer);
    bytes.extend_from_slice(&signature.to_bytes());
    bytes.push(27 + u8::from(recovery));
    bytes
}

struct Chunks {
    chunks: Vec<Chunk>,
    proofs: Vec<Proof>,
}

fn availability() -> Chunks {
    let mut chunks: Vec<Chunk> = [
        (AvailabilityClass::Activities, b"activity".to_vec()),
        (AvailabilityClass::Receipts, b"receipt".to_vec()),
        (AvailabilityClass::Oracle, b"oracle".to_vec()),
    ]
    .into_iter()
    .enumerate()
    .map(|(index, (class, bytes))| Chunk {
        batch_number: 1,
        index: must(u32::try_from(index)),
        class,
        class_offset: 0,
        bytes,
        claimed_hash: [0; 32],
    })
    .collect();
    for chunk in &mut chunks {
        chunk.claimed_hash = must(availability_chunk_digest(
            chunk.batch_number,
            chunk.index,
            chunk.class as u8,
            chunk.class_offset,
            &chunk.bytes,
        ));
    }
    let hashes: Vec<_> = chunks.iter().map(|chunk| chunk.claimed_hash).collect();
    let proofs = (0..hashes.len())
        .map(|index| must(build_leaf_hash_proof(&hashes, index)).0)
        .collect();
    Chunks { chunks, proofs }
}

fn chunk_bytes(chunk: &Chunk, proof: &Proof) -> Vec<u8> {
    let mut out = chunk.batch_number.to_be_bytes().to_vec();
    out.extend_from_slice(&chunk.index.to_be_bytes());
    out.push(chunk.class as u8);
    out.extend_from_slice(&chunk.class_offset.to_be_bytes());
    lp(&mut out, &chunk.bytes);
    out.extend_from_slice(&chunk.claimed_hash);
    lp(&mut out, &encode_proof(proof));
    out
}

fn checkpoint_bytes(
    reference: &str,
    header: &[u8],
    order: &[usize],
    chunk_count: Option<u16>,
) -> Vec<u8> {
    let attestations = [attestation(1), attestation(2)];
    let mut certificate = Vec::new();
    lp(&mut certificate, header);
    lp(&mut certificate, b"validity");
    certificate.push(2);
    certificate.push(2);
    for item in &attestations {
        certificate.extend_from_slice(item);
    }
    certificate.push(0);
    let mut out = b"LXCP".to_vec();
    out.push(1);
    lp(&mut out, reference.as_bytes());
    lp(&mut out, &certificate);
    out.extend_from_slice(&1_u64.to_be_bytes());
    out.extend_from_slice(&3_u16.to_be_bytes());
    for value in [1, 2, 3] {
        let (_, public_key, guarantor_id) = guarantor(value);
        out.extend_from_slice(&guarantor_id);
        out.extend_from_slice(&public_key);
        out.push(1);
    }
    out.extend_from_slice(&[0x61; 32]);
    out.push(0);
    let data = availability();
    let count = chunk_count.unwrap_or(must(u16::try_from(order.len())));
    out.extend_from_slice(&count.to_be_bytes());
    for index in order {
        out.extend_from_slice(&chunk_bytes(&data.chunks[*index], &data.proofs[*index]));
    }
    out
}

fn checkpoint_ref(activity: [u8; 32], batch: u64) -> String {
    format!("checkpoint:{}:{batch}", hex(&activity))
}

fn state_ref(activity: [u8; 32], account: [u8; 32]) -> String {
    format!("state:{}:{}", hex(&activity), hex(&account))
}

// Every decode must also refuse each strict prefix, one appended byte and a later version.
fn assert_strict<T>(bytes: &[u8], decode: impl Fn(&[u8]) -> Result<T, ExportCodecError>) {
    for end in 0..bytes.len() {
        assert!(decode(&bytes[..end]).is_err(), "prefix {end} accepted");
    }
    let mut trailing = bytes.to_vec();
    trailing.push(0);
    assert_eq!(
        decode(&trailing).err(),
        Some(ExportCodecError::TrailingBytes)
    );
    let mut version = bytes.to_vec();
    version[4] = RECORD_VERSION + 1;
    assert_eq!(decode(&version).err(), Some(ExportCodecError::Version));
    let mut magic = bytes.to_vec();
    magic[0] ^= 0x20;
    assert_eq!(decode(&magic).err(), Some(ExportCodecError::Magic));
    let mut overflow = bytes.to_vec();
    overflow[5..9].copy_from_slice(&u32::MAX.to_be_bytes());
    assert!(decode(&overflow).is_err());
}

#[test]
fn fact_grammar_accepts_each_canonical_form_in_request_order() {
    let activity = id(1);
    let account = id(2);
    let texts = [state_ref(activity, account), receipt_ref(activity)];
    let parsed = must(parse_fact_set(&texts));
    assert_eq!(
        parsed,
        vec![
            FactSelector::State {
                activity_id: activity,
                account_id: account
            },
            FactSelector::Receipt {
                activity_id: activity
            },
        ]
    );
    for (text, selector) in texts.iter().zip(&parsed) {
        assert_eq!(&selector.canonical_text(), text);
        assert_eq!(selector.activity_id(), activity);
    }
    assert_eq!(state_ref(activity, account).len(), MAX_FACT_REF_BYTES);
    let sixteen: Vec<String> = (1..=16).map(|value| receipt_ref(id(value))).collect();
    assert_eq!(MAX_FACT_REFS, 16);
    assert_eq!(must(parse_fact_set(&sixteen)).len(), 16);
}

#[test]
fn fact_grammar_refuses_every_noncanonical_form() {
    let good = hex(&id(1));
    let zero = hex(&[0; 32]);
    let upper = good.to_uppercase();
    let refused = [
        String::new(),
        format!("receipt:{zero}"),
        format!("activity:{zero}"),
        format!("state:{zero}:{good}"),
        format!("state:{good}:{zero}"),
        format!("checkpoint:{zero}:1"),
        format!("receipt:{upper}"),
        format!("Receipt:{good}"),
        format!("receipt:0x{}", &good[2..]),
        format!("receipt:{}", &good[..63]),
        format!("receipt:{good}0"),
        format!(" receipt:{good}"),
        format!("receipt:{good} "),
        format!("receipt: {good}"),
        format!("receipt:{good}\n"),
        format!("receipt:{good}:1"),
        format!("account:{good}"),
        format!("balance:{good}"),
        format!("state:{good}"),
        format!("state:{good}:{good}:{good}"),
        format!("checkpoint:{good}"),
        format!("checkpoint:{good}:"),
        format!("checkpoint:{good}:01"),
        format!("checkpoint:{good}:+1"),
        format!("checkpoint:{good}:-1"),
        format!("checkpoint:{good}:1 "),
        format!("checkpoint:{good}:18446744073709551616"),
        format!("checkpoint:{good}:99999999999999999999"),
        format!("{}0", state_ref(id(1), id(2))),
    ];
    for text in &refused {
        assert!(FactSelector::parse(text).is_err(), "accepted {text:?}");
        assert!(
            parse_fact_set(&[text.as_str()]).is_err(),
            "set accepted {text:?}"
        );
    }
    assert_eq!(
        FactSelector::parse(&format!("receipt:{zero}")),
        Err(FactRefError::ZeroIdentifier)
    );
    assert_eq!(
        FactSelector::parse(&format!("checkpoint:{good}:0")),
        Err(FactRefError::UnknownKind)
    );
    assert_eq!(
        FactSelector::parse(&format!("checkpoint:{good}:18446744073709551616")),
        Err(FactRefError::UnknownKind)
    );
    assert_eq!(
        FactSelector::parse(&format!("checkpoint:{good}:01")),
        Err(FactRefError::UnknownKind)
    );
    assert_eq!(
        FactSelector::parse(&format!("ledger:{good}")),
        Err(FactRefError::UnknownKind)
    );
    let long = format!("{}0", state_ref(id(1), id(2)));
    assert_eq!(long.len(), MAX_FACT_REF_BYTES + 1);
    assert_eq!(
        FactSelector::parse(&long),
        Err(FactRefError::TooLong {
            length: MAX_FACT_REF_BYTES + 1
        })
    );
    let empty: [&str; 0] = [];
    assert_eq!(parse_fact_set(&empty), Err(FactRefError::NoFacts));
    let seventeen: Vec<String> = (1..=17).map(|value| receipt_ref(id(value))).collect();
    assert_eq!(
        parse_fact_set(&seventeen),
        Err(FactRefError::TooMany { count: 17 })
    );
    let repeated = [receipt_ref(id(1)), receipt_ref(id(2)), receipt_ref(id(1))];
    assert_eq!(
        parse_fact_set(&repeated),
        Err(FactRefError::Duplicate { index: 2 })
    );
}

#[test]
fn receipt_header_and_inclusion_records_round_trip_their_frozen_byte_layouts() {
    let native = native();
    let signer = ed_key(9);
    let activity = id(1);
    let receipt_bytes = receipt(activity, &signer);

    let header = header_record(0, native.first_key, 1, 13);
    let header_wire = header_bytes(&header);
    let decoded = must(HeaderRecord::decode(&header_wire));
    assert_eq!(decoded.canonical_header, header.canonical_header);
    assert_eq!(decoded.signature, header.signature);
    assert_eq!(decoded.sequencer_id, header.sequencer_id);
    assert_eq!(decoded.public_key, native.first_key);
    assert_eq!((decoded.first_batch, decoded.last_batch), (1, 13));
    assert_eq!(must(decoded.encode()), header_wire);
    assert_eq!(
        must(decoded.digest()),
        must(batch_header_digest(&HEADERS[..HEADER_BYTES]))
    );
    assert_strict(&header_wire, HeaderRecord::decode);

    let receipt_wire =
        receipt_record_bytes(activity, &receipt_bytes, signer.verifying_key().to_bytes());
    let record = must(ReceiptRecord::decode(&receipt_wire));
    assert_eq!(
        record.reference,
        FactSelector::Receipt {
            activity_id: activity
        }
    );
    assert_eq!(record.canonical_receipt, receipt_bytes);
    assert_eq!(must(record.encode()), receipt_wire);
    assert_strict(&receipt_wire, ReceiptRecord::decode);
    let mut wrong_kind = b"LXRF".to_vec();
    wrong_kind.push(1);
    lp(
        &mut wrong_kind,
        format!("activity:{}", hex(&activity)).as_bytes(),
    );
    wrong_kind.extend_from_slice(&receipt_wire[9 + receipt_ref(activity).len()..]);
    assert_eq!(
        ReceiptRecord::decode(&wrong_kind).err(),
        Some(ExportCodecError::Reference(FactRefError::UnknownKind))
    );

    let digest = must(header.digest());
    for (kind, reference, leaf) in [(2_u8, receipt_ref(activity), receipt_bytes.clone())] {
        let wire = inclusion_bytes(kind, &reference, &leaf, &pair_proof(1), digest);
        let decoded = must(InclusionRecord::decode(&wire));
        assert!(matches!(
            (&decoded.kind, kind),
            (layerx_proof::export_codec::InclusionRecordKind::Receipt, 2)
        ));
        assert_eq!(decoded.reference.canonical_text(), reference);
        assert_eq!(decoded.canonical_leaf, leaf);
        assert_eq!(decoded.proof, pair_proof(1));
        assert_eq!(decoded.header_digest, digest);
        assert_eq!(must(decoded.encode()), wire);
        let routed = must(ProofRecord::decode(&wire));
        assert_eq!(must(routed.encode()), wire);
        assert_strict(&wire, InclusionRecord::decode);
        for tag in [0_u8, 1, 3, 0xff] {
            let mut unknown = wire.clone();
            unknown[5] = tag;
            assert_eq!(
                InclusionRecord::decode(&unknown).err(),
                Some(ExportCodecError::Tag)
            );
        }
    }
    // Receipt inclusion must not travel under the activity kind.
    let swapped = inclusion_bytes(
        1,
        &receipt_ref(activity),
        &receipt_bytes,
        &pair_proof(1),
        digest,
    );
    assert_eq!(
        InclusionRecord::decode(&swapped).err(),
        Some(ExportCodecError::Tag)
    );
    // A Merkle path that does not match its tree size is not a canonical proof.
    let mut bad_proof = inclusion_bytes(
        2,
        &receipt_ref(activity),
        &receipt_bytes,
        &pair_proof(1),
        digest,
    );
    let proof_at = bad_proof.len() - 32 - encode_proof(&pair_proof(1)).len();
    bad_proof[proof_at + 5..proof_at + 9].copy_from_slice(&3_u32.to_be_bytes());
    assert!(InclusionRecord::decode(&bad_proof).is_err());
}

#[test]
fn account_state_records_round_trip_both_variants_and_refuse_unknown_variants() {
    let signer = ed_key(9);
    let activity = id(1);
    let account = id(0x22);
    let reference = state_ref(activity, account);
    let receipt_bytes = receipt(activity, &signer);
    let digest = [0x51; 32];
    let plain = account_state_bytes(&reference, 1, &receipt_bytes, digest, None);
    let decoded = must(AccountStateRecord::decode(&plain));
    assert_eq!(
        decoded.reference,
        FactSelector::State {
            activity_id: activity,
            account_id: account
        }
    );
    assert_eq!(decoded.account_id, account);
    assert_eq!(decoded.account_value, vec![0x41; 48]);
    assert_eq!(decoded.account_proof, pair_proof(0));
    assert_eq!(decoded.account_tree_proof, pair_proof(1));
    assert_eq!(decoded.universal_root_proof, single_proof());
    assert_eq!(decoded.receipt_bytes, receipt_bytes);
    assert_eq!(decoded.header_digest, digest);
    assert_eq!(must(decoded.encode()), plain);
    assert_eq!(must(must(ProofRecord::decode(&plain)).encode()), plain);
    assert_strict(&plain, AccountStateRecord::decode);

    let maintained = account_state_bytes(
        &reference,
        2,
        b"maintenance",
        digest,
        Some((3, 7, activity)),
    );
    let decoded = must(AccountStateRecord::decode(&maintained));
    assert!(matches!(
        decoded.variant,
        layerx_proof::export_codec::AccountStateVariant::Maintenance {
            activity_count: 3,
            parameter_version: 7,
            dependency_receipt_activity_id,
        } if dependency_receipt_activity_id == activity
    ));
    assert_eq!(must(decoded.encode()), maintained);
    assert_strict(&maintained, AccountStateRecord::decode);
    // Variant 2 without its dependency tail, and variant 1 with one, are not canonical.
    assert!(AccountStateRecord::decode(&maintained[..maintained.len() - 40]).is_err());
    let mut extended = plain.clone();
    extended.extend_from_slice(&maintained[maintained.len() - 40..]);
    assert!(AccountStateRecord::decode(&extended).is_err());
    let variant_at = 9 + reference.len();
    for tag in [0_u8, 3, 0xff] {
        let mut unknown = plain.clone();
        unknown[variant_at] = tag;
        assert_eq!(
            AccountStateRecord::decode(&unknown).err(),
            Some(ExportCodecError::Tag)
        );
    }
}

#[test]
fn record_size_is_capped_at_exactly_one_mebibyte() {
    let digest = [0x51; 32];
    let reference = receipt_ref(id(1));
    let empty = inclusion_bytes(2, &reference, &[], &single_proof(), digest).len();
    let fits = vec![0x5a; MAX_RECORD_BYTES - empty];
    let exact = inclusion_bytes(2, &reference, &fits, &single_proof(), digest);
    assert_eq!(exact.len(), MAX_RECORD_BYTES);
    let decoded = must(InclusionRecord::decode(&exact));
    assert_eq!(must(decoded.encode()).len(), MAX_RECORD_BYTES);
    let over = vec![0x5a; MAX_RECORD_BYTES - empty + 1];
    let oversized = inclusion_bytes(2, &reference, &over, &single_proof(), digest);
    assert_eq!(oversized.len(), MAX_RECORD_BYTES + 1);
    assert_eq!(
        InclusionRecord::decode(&oversized).err(),
        Some(ExportCodecError::Bound)
    );
    let mut record = decoded;
    record.canonical_leaf = over;
    assert_eq!(record.encode().err(), Some(ExportCodecError::Bound));
}

fn receipt_artifact(header: &HeaderRecord, activity: [u8; 32]) -> Vec<Vec<u8>> {
    let signer = ed_key(9);
    let receipt_bytes = receipt(activity, &signer);
    vec![
        receipt_record_bytes(activity, &receipt_bytes, signer.verifying_key().to_bytes()),
        inclusion_bytes(
            2,
            &receipt_ref(activity),
            &receipt_bytes,
            &single_proof(),
            must(header.digest()),
        ),
        header_bytes(header),
    ]
}

fn decode_artifact(
    facts: &[String],
    parts: &[Vec<u8>],
    certificates: &[Vec<u8>],
) -> CompleteOfflineArtifact {
    must(CompleteOfflineArtifact::decode(
        facts,
        &parts[..1],
        &parts[1..parts.len() - 1],
        certificates,
        &parts[parts.len() - 1..],
    ))
}

#[test]
fn artifact_container_round_trips_and_refuses_duplicate_or_malformed_buckets() {
    let native = native();
    let activity = id(1);
    let header = header_record(0, native.first_key, 1, 13);
    let parts = receipt_artifact(&header, activity);
    let facts = vec![receipt_ref(activity)];
    let artifact = decode_artifact(&facts, &parts, &[]);
    assert_eq!(
        artifact.facts,
        vec![FactSelector::Receipt {
            activity_id: activity
        }]
    );
    let encoded = must(artifact.encode());
    assert_eq!(encoded.facts, facts);
    assert_eq!(encoded.receipts, parts[..1].to_vec());
    assert_eq!(encoded.proofs, parts[1..2].to_vec());
    assert_eq!(encoded.headers, parts[2..].to_vec());
    assert!(encoded.certificates.is_empty());
    let no_certificates: [Vec<u8>; 0] = [];
    let doubled = [receipt_ref(activity), receipt_ref(activity)];
    assert_eq!(
        CompleteOfflineArtifact::decode(
            &doubled,
            &parts[..1],
            &parts[1..2],
            &no_certificates,
            &parts[2..]
        )
        .err(),
        Some(ArtifactDecodeError::Facts(FactRefError::Duplicate {
            index: 1
        }))
    );
    let mut trailing = parts[2].clone();
    trailing.push(0);
    assert_eq!(
        CompleteOfflineArtifact::decode(
            &facts,
            &parts[..1],
            &parts[1..2],
            &no_certificates,
            &[trailing]
        )
        .err(),
        Some(ArtifactDecodeError::Record {
            bucket: "headers",
            index: 0,
            error: ExportCodecError::TrailingBytes,
        })
    );
    // A header record in the receipt bucket is not reinterpreted.
    assert!(CompleteOfflineArtifact::decode(
        &facts,
        &parts[2..],
        &parts[1..2],
        &no_certificates,
        &parts[2..]
    )
    .is_err());
}

#[test]
fn independent_trust_refuses_unverified_authority() {
    let genesis = must(decode_genesis_trust(GENESIS));
    let authority = must(SignedAuthorityHistory::from_genesis(
        genesis.network_id,
        genesis.canonical_state_root,
        genesis.initial_sequencer_key,
        genesis.governance_witness,
    ));
    assert!(matches!(
        IndependentOfflineTrust::new(genesis.registry, authority),
        Err(OfflineTrustError::NoAuthority)
    ));
    assert_eq!(trust().network_id(), native().history.network_id());
}

fn signed_export_batch(
    receipts: &[Vec<u8>],
    previous: [u8; 32],
    resulting: [u8; 32],
    first_sequence: u64,
    last_sequence: u64,
) -> (HeaderRecord, IndependentOfflineTrust, Vec<Proof>) {
    let genesis = must(decode_genesis_trust(GENESIS));
    let signer = ed_key(9);
    let public_key = signer.verifying_key().to_bytes();
    let sequencer = must(sequencer_id(&public_key));
    let leaves: Vec<_> = receipts.iter().map(Vec::as_slice).collect();
    let receipt_root = must(build_proof(&leaves, 0)).1;
    let proofs = (0..receipts.len())
        .map(|index| must(build_proof(&leaves, index)).0)
        .collect();
    let mut encoder = Encoder::new(HEADER_BYTES);
    must(encoder.structure_header_version(0x1701, STATE_COMMITMENT_PROTOCOL_VERSION));
    must(encoder.u8(15));
    must(encoder.tag(1, 15));
    must(encoder.u16(STATE_COMMITMENT_PROTOCOL_VERSION));
    must(encoder.tag(2, 15));
    must(encoder.u32(genesis.network_id));
    for (tag, value) in [(3, 1), (4, 1), (5, first_sequence), (6, last_sequence)] {
        must(encoder.tag(tag, 15));
        must(encoder.u64(value));
    }
    for (tag, value) in [
        (7, previous),
        (8, resulting),
        (9, [8; 32]),
        (10, receipt_root),
        (11, [10; 32]),
        (12, [11; 32]),
        (13, [12; 32]),
    ] {
        must(encoder.tag(tag, 15));
        must(encoder.bytes(&value, 32));
    }
    must(encoder.tag(14, 15));
    must(encoder.u64(1_000));
    must(encoder.tag(15, 15));
    must(encoder.bytes(&sequencer, 32));
    let canonical_header = encoder.finish();
    let digest = must(batch_header_digest(&canonical_header));
    let signature = signer.sign(&digest).to_bytes();
    let mut history = must(SignedAuthorityHistory::from_genesis(
        genesis.network_id,
        genesis.canonical_state_root,
        public_key,
        genesis.governance_witness,
    ));
    must(history.advance(&canonical_header, &signature, None));
    let trust = must(IndependentOfflineTrust::new(genesis.registry, history));
    (
        HeaderRecord {
            canonical_header,
            signature,
            sequencer_id: sequencer,
            public_key,
            first_batch: 1,
            last_batch: 1,
        },
        trust,
        proofs,
    )
}

fn receipted_export_for(
    activities: &[[u8; 32]],
) -> (CompleteOfflineArtifact, IndependentOfflineTrust) {
    let signer = ed_key(9);
    let previous = must(decode_batch_header(&HEADERS[..HEADER_BYTES])).previous_state_root();
    let last = must(u64::try_from(activities.len()));
    let batch = must(program_execution_batch_id(previous, [8; 32], 1, last, 1));
    let receipts: Vec<_> = activities
        .iter()
        .enumerate()
        .map(|(index, activity)| {
            receipt_with_state(
                *activity,
                &signer,
                previous,
                [3; 32],
                batch,
                STATE_COMMITMENT_PROTOCOL_VERSION,
                must(u64::try_from(index)) + 1,
            )
        })
        .collect();
    let (header, trust, proofs) = signed_export_batch(&receipts, previous, [3; 32], 1, last);
    let digest = must(header.digest());
    let records: Vec<_> = activities
        .iter()
        .zip(&receipts)
        .map(|(activity, receipt)| {
            receipt_record_bytes(*activity, receipt, signer.verifying_key().to_bytes())
        })
        .collect();
    let inclusions: Vec<_> = activities
        .iter()
        .zip(&receipts)
        .zip(&proofs)
        .map(|((activity, receipt), proof)| {
            inclusion_bytes(2, &receipt_ref(*activity), receipt, proof, digest)
        })
        .collect();
    (
        must(CompleteOfflineArtifact::decode(
            &activities
                .iter()
                .map(|activity| receipt_ref(*activity))
                .collect::<Vec<_>>(),
            &records,
            &inclusions,
            &NO_RECORDS,
            &[must(header.encode())],
        )),
        trust,
    )
}

fn receipted_export() -> (CompleteOfflineArtifact, IndependentOfflineTrust) {
    receipted_export_for(&[id(1)])
}

fn transported(artifact: &CompleteOfflineArtifact) -> CompleteOfflineArtifact {
    let wire = must(artifact.encode());
    must(CompleteOfflineArtifact::decode(
        &wire.facts,
        &wire.receipts,
        &wire.proofs,
        &wire.certificates,
        &wire.headers,
    ))
}

fn assert_receipted(artifact: &CompleteOfflineArtifact, trust: &IndependentOfflineTrust) {
    let report = must(verify_complete(
        &transported(artifact),
        trust,
        Rung::Receipted,
    ));
    assert_eq!(report.achieved(), Rung::Receipted);
    assert_eq!(report.facts().len(), artifact.facts.len());
    for (verified, expected) in report.facts().iter().zip(&artifact.facts) {
        assert_eq!(verified.fact(), *expected);
        assert_eq!(verified.achieved(), Rung::Receipted);
    }
}

#[test]
fn complete_export_reports_the_requested_rung() {
    let (artifact, trust) = receipted_export();
    assert!(must(artifact.encode()).certificates.is_empty());
    let report = must(verify_complete(&artifact, &trust, Rung::Receipted));
    assert_eq!(report.achieved(), Rung::Receipted);
    assert_eq!(report.facts().len(), 1);
    assert_eq!(report.facts()[0].fact(), artifact.facts[0]);
    assert_eq!(report.facts()[0].achieved(), Rung::Receipted);
}

#[test]
fn complete_export_below_the_requested_rung_is_refused() {
    let (artifact, trust) = receipted_export();
    assert!(must(artifact.encode()).certificates.is_empty());
    assert_eq!(
        verify_complete(&artifact, &trust, Rung::Committed),
        Err(CompleteExportError::RungNotAchieved {
            fact: 0,
            achieved: Rung::Receipted,
            requested: Rung::Committed,
        })
    );
}

#[test]
fn rogue_sequencer_key_and_false_key_range_are_refused_against_independent_trust() {
    let (control, trust) = receipted_export();
    assert_receipted(&control, &trust);
    let mut reversed = must(control.encode());
    let mut header = control.headers[0].clone();
    header.last_batch = 0;
    reversed.headers[0] = header_bytes(&header);
    assert_eq!(
        CompleteOfflineArtifact::decode(
            &reversed.facts,
            &reversed.receipts,
            &reversed.proofs,
            &reversed.certificates,
            &reversed.headers
        )
        .err(),
        Some(ArtifactDecodeError::Record {
            bucket: "headers",
            index: 0,
            error: ExportCodecError::Bound
        })
    );
    // Each case changes exactly one field of the working signed batch.
    let mut candidates = Vec::new();
    let mut changed = control.clone();
    changed.headers[0].public_key = ed_key(0x77).verifying_key().to_bytes();
    candidates.push(changed);
    let mut changed = control.clone();
    changed.headers[0].public_key = native().history.intervals()[1].public_key();
    candidates.push(changed);
    let mut range_control = control.clone();
    range_control.headers[0].last_batch = 13;
    assert_receipted(&range_control, &trust);
    let mut changed = range_control;
    changed.headers[0].first_batch = 2;
    candidates.push(changed);
    let mut range_control = control.clone();
    range_control.headers[0].first_batch = 0;
    assert_receipted(&range_control, &trust);
    let mut changed = range_control;
    changed.headers[0].last_batch = 0;
    candidates.push(changed);
    let mut changed = control.clone();
    changed.headers[0].signature = ed_key(0x77)
        .sign(&must(changed.headers[0].digest()))
        .to_bytes();
    candidates.push(changed);
    let mut changed = control.clone();
    changed.headers[0].sequencer_id = [1; 32];
    candidates.push(changed);
    let mut changed = control.clone();
    changed.headers[0].signature[0] ^= 1;
    candidates.push(changed);
    for candidate in candidates {
        assert_eq!(
            verify_complete(&transported(&candidate), &trust, Rung::Receipted),
            Err(CompleteExportError::Header { fact: 0 })
        );
    }
    let mut forged = control.clone();
    forged.headers[0].canonical_header[100] ^= 1;
    let digest = must(forged.headers[0].digest());
    let ProofRecord::Inclusion(inclusion) = &mut forged.proofs[0] else {
        panic!("receipt inclusion")
    };
    inclusion.header_digest = digest;
    assert_eq!(
        verify_complete(&transported(&forged), &trust, Rung::Receipted),
        Err(CompleteExportError::Header { fact: 0 })
    );
}

#[test]
fn swapped_and_orphan_records_are_refused() {
    let (control, trust) = receipted_export_for(&[id(1), id(2)]);
    assert_receipted(&control, &trust);
    let mut changed = control.clone();
    changed.receipts[0].reference = control.receipts[1].reference;
    changed.receipts[1].reference = control.receipts[0].reference;
    assert_eq!(
        verify_complete(&transported(&changed), &trust, Rung::Receipted),
        Err(CompleteExportError::ReceiptBinding { fact: 0 })
    );
    let mut changed = control.clone();
    let first = changed.proofs[0].clone();
    let second = changed.proofs[1].clone();
    let (ProofRecord::Inclusion(first), ProofRecord::Inclusion(second)) = (first, second) else {
        panic!("receipt inclusions")
    };
    let ProofRecord::Inclusion(record) = &mut changed.proofs[0] else {
        panic!("inclusion")
    };
    record.reference = second.reference;
    let ProofRecord::Inclusion(record) = &mut changed.proofs[1] else {
        panic!("inclusion")
    };
    record.reference = first.reference;
    assert_eq!(
        verify_complete(&transported(&changed), &trust, Rung::Receipted),
        Err(CompleteExportError::ReceiptBinding { fact: 0 })
    );
    let mut changed = control.clone();
    changed.facts.pop();
    assert_eq!(
        verify_complete(&transported(&changed), &trust, Rung::Receipted),
        Err(CompleteExportError::OrphanRecord)
    );
    let mut changed = control.clone();
    changed.receipts.pop();
    assert_eq!(
        verify_complete(&transported(&changed), &trust, Rung::Receipted),
        Err(CompleteExportError::MissingRecord { fact: 1 })
    );
}

fn state_leaf(key: &[u8], value: &[u8]) -> [u8; 32] {
    let mut bytes = b"LXP/v1/state-leaf\0".to_vec();
    bytes.extend_from_slice(&must(u32::try_from(key.len())).to_be_bytes());
    bytes.extend_from_slice(&must(u32::try_from(value.len())).to_be_bytes());
    bytes.extend_from_slice(key);
    bytes.extend_from_slice(value);
    Sha256::digest(bytes).into()
}

fn occupancy_leaf(root: [u8; 32]) -> Vec<u8> {
    use layerx_programs_runtime::{
        meter::FeeSchedule, occupancy::OccupancyLedger, FeeScheduleParameters,
    };
    let schedule = FeeSchedule::new_complete(FeeScheduleParameters {
        version: 1,
        fee_units_per_cpu_fuel: 1,
        fee_units_per_memory_byte: 2,
        fee_units_per_storage_read_byte: 3,
        fee_units_per_storage_write_byte: 4,
        fee_units_per_output_value: 5,
        fee_units_per_output_byte: 6,
        fee_units_per_occupancy_byte_batch: 7,
    });
    let prepared = must(OccupancyLedger::activated_after(0).prepare_unchanged_batch(1, schedule));
    let settlement = prepared.settlement();
    let evidence = settlement.canonical_evidence();
    let usage = settlement.usage();
    let mut schedule_bytes = 1_u32.to_be_bytes().to_vec();
    for price in 1_u64..=7 {
        schedule_bytes.extend_from_slice(&price.to_be_bytes());
    }
    schedule_bytes.extend_from_slice(&[0x81; 32]);
    let mut commitment = b"LXP/v1/context-hash\0".to_vec();
    commitment.extend_from_slice(&schedule_bytes);
    let mut bytes = b"LXP/programs/occupancy-receipt/v2\0".to_vec();
    bytes.extend_from_slice(&1_u64.to_be_bytes());
    bytes.extend_from_slice(&2_u64.to_be_bytes());
    bytes.extend_from_slice(&1_u32.to_be_bytes());
    bytes.extend_from_slice(&schedule_bytes);
    for amount in [
        usage.byte_batches,
        usage.fee_units,
        usage.paid_fee_units,
        usage.arrears_fee_units,
    ] {
        bytes.extend_from_slice(&amount.to_be_bytes());
    }
    assert!(must(settlement.payer_dispositions()).is_empty());
    bytes.extend_from_slice(&0_u16.to_be_bytes());
    bytes.extend_from_slice(&Sha256::digest(commitment));
    lp(&mut bytes, &evidence);
    bytes.extend_from_slice(&Sha256::digest(&evidence));
    bytes.extend_from_slice(&[0x82; 32]);
    bytes.extend_from_slice(&must(settlement.transfer_root([0x81; 32])));
    bytes.extend_from_slice(&root);
    bytes.extend_from_slice(&root);
    bytes
}

fn state_export(maintained: bool) -> (CompleteOfflineArtifact, IndependentOfflineTrust) {
    let vectors = include_str!("../../../../../tests/vectors/program_account_state_v2.vec");
    let entry = |name: &str| {
        vectors
            .lines()
            .find_map(|line| {
                let (key, value) = line.split_once('=')?;
                (key == name).then_some(value)
            })
            .unwrap_or_else(|| panic!("missing native account vector {name}"))
    };
    let unhex = |text: &str| {
        text.as_bytes()
            .chunks_exact(2)
            .map(|pair| must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16)))
            .collect::<Vec<_>>()
    };
    let account: [u8; 32] = must(unhex(entry("account_id")).try_into());
    let account_value = unhex(entry("account_value"));
    let mut key = vec![4];
    key.extend_from_slice(&account);
    let leaf = state_leaf(&key, &account_value);
    assert_eq!(leaf.as_slice(), unhex(entry("account_leaf")));
    let sibling = state_leaf(b"other-account", b"recorded peer");
    let mut node = b"LXP/v1/state-node\0".to_vec();
    node.extend_from_slice(&leaf);
    node.extend_from_slice(&sibling);
    let account_root: [u8; 32] = Sha256::digest(node).into();
    let universal_root = state_leaf(b"account-tree", &account_root);
    let resulting = state_leaf(&0_u16.to_be_bytes(), &universal_root);
    let previous = must(decode_batch_header(&HEADERS[..HEADER_BYTES])).previous_state_root();
    let last = if maintained { 2 } else { 1 };
    let batch = must(program_execution_batch_id(previous, [8; 32], 1, last, 1));
    let signer = ed_key(9);
    let receipt = receipt_with_state(
        id(1),
        &signer,
        previous,
        resulting,
        batch,
        STATE_COMMITMENT_PROTOCOL_VERSION,
        1,
    );
    let mut receipts = vec![receipt.clone()];
    if maintained {
        receipts.push(occupancy_leaf(resulting));
    }
    let (header, trust, proofs) = signed_export_batch(&receipts, previous, resulting, 1, last);
    let digest = must(header.digest());
    let state = AccountStateRecord {
        reference: FactSelector::State {
            activity_id: id(1),
            account_id: account,
        },
        variant: if maintained {
            AccountStateVariant::Maintenance {
                activity_count: 1,
                parameter_version: 1,
                dependency_receipt_activity_id: id(1),
            }
        } else {
            AccountStateVariant::Activity
        },
        account_id: account,
        account_value,
        account_root,
        universal_root,
        resulting_state_root: resulting,
        account_proof: must(Proof::new(0, 2, vec![sibling])),
        account_tree_proof: single_proof(),
        universal_root_proof: single_proof(),
        receipt_bytes: receipts[usize::from(maintained)].clone(),
        receipt_proof: proofs[usize::from(maintained)].clone(),
        header_digest: digest,
    };
    let mut artifact = CompleteOfflineArtifact {
        facts: vec![state.reference],
        receipts: Vec::new(),
        proofs: vec![ProofRecord::AccountState(state)],
        headers: vec![header],
    };
    if maintained {
        artifact
            .receipts
            .push(must(ReceiptRecord::decode(&receipt_record_bytes(
                id(1),
                &receipt,
                signer.verifying_key().to_bytes(),
            ))));
        artifact
            .proofs
            .push(ProofRecord::Inclusion(InclusionRecord {
                kind: InclusionRecordKind::Receipt,
                reference: FactSelector::Receipt { activity_id: id(1) },
                canonical_leaf: receipt,
                proof: proofs[0].clone(),
                header_digest: digest,
            }));
    }
    (transported(&artifact), trust)
}

#[test]
fn state_path_tamper_and_missing_maintenance_link_are_refused() {
    for maintained in [false, true] {
        let (control, trust) = state_export(maintained);
        assert_receipted(&control, &trust);
        let mut tampered = control.clone();
        let ProofRecord::AccountState(state) = &mut tampered.proofs[0] else {
            panic!("account state")
        };
        let mut siblings = state.account_proof.siblings().to_vec();
        siblings[0][0] ^= 1;
        state.account_proof = must(Proof::new(0, 2, siblings));
        assert_eq!(
            verify_complete(&transported(&tampered), &trust, Rung::Receipted),
            Err(CompleteExportError::AccountState {
                fact: 0,
                error: AccountProofError::AccountProof(MerkleError::RootMismatch)
            })
        );
        if maintained {
            let mut missing = control.clone();
            missing.receipts.clear();
            assert_eq!(
                verify_complete(&transported(&missing), &trust, Rung::Receipted),
                Err(CompleteExportError::MissingRecord { fact: 0 })
            );
            let mut wrong = control.clone();
            let ProofRecord::AccountState(state) = &mut wrong.proofs[0] else {
                panic!("account state")
            };
            let AccountStateVariant::Maintenance {
                dependency_receipt_activity_id,
                ..
            } = &mut state.variant
            else {
                panic!("maintenance variant")
            };
            *dependency_receipt_activity_id = id(3);
            assert_eq!(
                verify_complete(&transported(&wrong), &trust, Rung::Receipted),
                Err(CompleteExportError::MaintenanceLink { fact: 0 })
            );
            let mut wrong = control.clone();
            wrong.receipts[0].reference = FactSelector::Receipt { activity_id: id(3) };
            assert_eq!(
                verify_complete(&transported(&wrong), &trust, Rung::Receipted),
                Err(CompleteExportError::MissingRecord { fact: 0 })
            );
        }
    }
}

#[test]
fn checkpoint_facts_refuse_wrong_domain_membership_and_incomplete_availability() {
    let native = native();
    let activity = id(1);
    let header = header_record(0, native.first_key, 1, 13);
    let parts = receipt_artifact(&header, activity);
    let reference = checkpoint_ref(activity, 1);
    for order in [&[0, 1, 2][..], &[0, 1][..], &[][..]] {
        let certificate = checkpoint_bytes(&reference, &HEADERS[..HEADER_BYTES], order, None);
        assert_eq!(
            CompleteOfflineArtifact::decode(
                &[receipt_ref(activity)],
                &parts[..1],
                &parts[1..2],
                &[certificate],
                &parts[2..],
            )
            .err(),
            Some(ArtifactDecodeError::Record {
                bucket: "certificates",
                index: 0,
                error: ExportCodecError::Tag,
            })
        );
    }
}

#[test]
fn complete_export_decoder_refuses_checkpoint_and_activity_records() {
    let (artifact, trust) = receipted_export();
    let encoded = must(artifact.encode());
    let decode = |facts: &[String], proofs: &[Vec<u8>], certificates: &[Vec<u8>]| {
        CompleteOfflineArtifact::decode(
            facts,
            &encoded.receipts,
            proofs,
            certificates,
            &encoded.headers,
        )
    };
    let control = must(decode(&encoded.facts, &encoded.proofs, &[]));
    let report = must(verify_complete(&control, &trust, Rung::Receipted));
    assert_eq!(report.achieved(), Rung::Receipted);
    assert_eq!(report.facts().len(), 1);
    let activity = artifact.facts[0].activity_id();
    for reference in [
        format!("activity:{}", hex(&activity)),
        checkpoint_ref(activity, 1),
    ] {
        assert_eq!(
            decode(&[reference.clone()], &encoded.proofs, &[]).err(),
            Some(ArtifactDecodeError::Facts(FactRefError::UnknownKind))
        );
        let record = inclusion_bytes(2, &reference, b"retired", &single_proof(), [1; 32]);
        assert_eq!(
            decode(&encoded.facts, &[record], &[]).err(),
            Some(ArtifactDecodeError::Record {
                bucket: "proofs",
                index: 0,
                error: ExportCodecError::Reference(FactRefError::UnknownKind)
            })
        );
    }
    let mut activity_record = encoded.proofs[0].clone();
    activity_record[5] = 1;
    assert_eq!(
        decode(&encoded.facts, &[activity_record], &[]).err(),
        Some(ArtifactDecodeError::Record {
            bucket: "proofs",
            index: 0,
            error: ExportCodecError::Tag
        })
    );
    let legacy = checkpoint_bytes(
        &checkpoint_ref(activity, 1),
        &HEADERS[..HEADER_BYTES],
        &[0, 1, 2],
        None,
    );
    for certificate in [legacy, encoded.receipts[0].clone(), Vec::new()] {
        assert_eq!(
            decode(&encoded.facts, &encoded.proofs, &[certificate]).err(),
            Some(ArtifactDecodeError::Record {
                bucket: "certificates",
                index: 0,
                error: ExportCodecError::Tag
            })
        );
    }
    assert_eq!(must(control.encode()), encoded);
}
