//! Task 22.31.214 repair record (the gate ledger supplies the verified revision).
//! native-send-owner: rotated_owner_consent_crosses_transport_signing_and_reopen;
//! native-send-owner: native_send_refuses_legacy_domains_and_noncanonical_frames.
//! proof-rung: committed_activity_and_receipt_cross_shared_rpc_codecs;
//! proof-rung: proof_rung_refuses_downgrades_and_inconsistent_evidence.
//! Baseline: no existing repair test or prior gate ledger; unchanged verification
//! was not run because the enforced gate order requires complete implementation first.
//! Wiring request, agent domain 9: services/agent/schema/agent-api/v1.kvx must declare NativeSendPurposeV2 and SignedNativeSendPurposeV2 byte layouts and explicit V1 refusal.
//! Wiring request, agent domain 9: services/agent/crates/layerx-agentd/src/{agent_rpc_wire.rs,human_runtime.rs,capability/binding.rs} must adopt V2, resolve current stable owner/key/address, verify consent and recovered sender, and persist/revalidate V2 across daemon restart.
//! Wiring request, agent domain 9: services/agent/crates/layerx-sdk/src/{native_effect.rs,agent_envelope.rs} must adopt the V2 carrier and prove a real committed receipt through daemon RPC and SDK end to end.
//! Wiring request, Human domain 10: services/human/crates/layerx-human-service/src/{server/native_send.rs,server/agent_runtime.rs,journeys/engine.rs,custody/provider.rs} and services/human/crates/layerx-human-kms/src/attestor.rs must adopt V2, sign SHA256(V2 canonical purpose) with the current consent key and independently recover the type-2 sender on chain 125 before release, including owner rotation and process restart.
//! Wiring request, agent domain 9: services/agent/crates/layerx-agent-api/tests/read_heights.rs still asserts achieved == selector minimum; update its obsolete expectation in its owning task without weakening the minimum or requested-rung checks.
//! These tests prove the real shared contract codecs, pinned receipt signatures,
//! current-owner/preparation comparisons and genuine Ed25519 consent signatures.
//! They do not claim daemon/Human/custody migration or independent Comet finality
//! authentication, which need the out-of-scope consumers above.

use std::fs;
use std::io::{Read as _, Write as _};
use std::net::{Shutdown, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Command, Output};
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agent_api::identity::{
    AgentDid, CapabilityId, ContractError, NativeActivity, NativeSendPurposeV1,
    NativeSendPurposeV2, SessionId, SignedNativeSendPurposeV1, SignedNativeSendPurposeV2, TenantId,
};
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::proof::{ProofBundle, ProofBundleTarget};
use layerx_agent_api::read::{Freshness, ReadRequest, RelativeTo, VerifiedRead};
use layerx_agent_api::verify::Rung;
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};
use layerx_types::json::{parse, JsonValue};

const RECORDED: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/vector-v5-portable.json");

static NEXT_TEMP: AtomicU64 = AtomicU64::new(0);

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "repair-22-31-214-{}-{}",
            std::process::id(),
            NEXT_TEMP.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).expect("private test directory");
        Self(path)
    }

    fn path(&self, name: &str) -> PathBuf {
        self.0.join(name)
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove this test's temporary files");
    }
}

fn openssl(arguments: &[&str], paths: &[&Path]) -> Output {
    let mut command = Command::new("openssl");
    command.args(arguments);
    for path in paths {
        command.arg(path);
    }
    command
        .output()
        .expect("installed OpenSSL crypto implementation")
}

fn successful(output: Output) -> Vec<u8> {
    assert!(
        output.status.success(),
        "OpenSSL failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    output.stdout
}

fn sha256(directory: &Directory, bytes: &[u8]) -> [u8; 32] {
    let path = directory.path("digest-input");
    fs::write(&path, bytes).expect("hash input");
    successful(openssl(&["dgst", "-sha256", "-binary"], &[&path]))
        .try_into()
        .expect("SHA256 digest")
}

fn consent_key(directory: &Directory, name: &str, seed: u8) -> ([u8; 32], PathBuf, PathBuf) {
    // Public deterministic test seeds, encoded as RFC 8410 PKCS#8 Ed25519 keys.
    let mut der = vec![
        0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x04, 0x22, 0x04,
        0x20,
    ];
    der.extend_from_slice(&[seed; 32]);
    let private = directory.path(&format!("{name}.der"));
    fs::write(&private, der).expect("public test key");
    let public_der = successful(openssl(
        &[
            "pkey", "-inform", "DER", "-pubout", "-outform", "DER", "-in",
        ],
        &[&private],
    ));
    assert_eq!(public_der.len(), 44);
    let public = directory.path(&format!("{name}-public.der"));
    fs::write(&public, &public_der).expect("public verification key");
    (
        public_der[12..].try_into().expect("Ed25519 public key"),
        private,
        public,
    )
}

fn sign(
    directory: &Directory,
    key: &Path,
    purpose: NativeSendPurposeV2,
) -> SignedNativeSendPurposeV2 {
    let digest = sha256(
        directory,
        &purpose.canonical_bytes().expect("canonical purpose"),
    );
    let path = directory.path("consent-digest");
    fs::write(&path, digest).expect("consent digest");
    let output = Command::new("openssl")
        .args(["pkeyutl", "-sign", "-rawin", "-keyform", "DER", "-inkey"])
        .arg(key)
        .arg("-in")
        .arg(path)
        .output()
        .expect("real Ed25519 signer");
    SignedNativeSendPurposeV2 {
        purpose,
        signature: successful(output).try_into().expect("Ed25519 signature"),
    }
    .validate()
    .expect("signed consent carrier")
}

fn signature_valid(directory: &Directory, key: &Path, signed: &SignedNativeSendPurposeV2) -> bool {
    let digest = sha256(
        directory,
        &signed.purpose.canonical_bytes().expect("canonical purpose"),
    );
    let path = directory.path("verify-digest");
    let signature = directory.path("signature");
    fs::write(&path, digest).expect("verification digest");
    fs::write(&signature, signed.signature).expect("verification signature");
    Command::new("openssl")
        .args([
            "pkeyutl", "-verify", "-rawin", "-pubin", "-keyform", "DER", "-inkey",
        ])
        .arg(key)
        .arg("-in")
        .arg(path)
        .arg("-sigfile")
        .arg(signature)
        .output()
        .expect("real Ed25519 verifier")
        .status
        .success()
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn signed_transport(signed: &SignedNativeSendPurposeV2) -> SignedNativeSendPurposeV2 {
    let listener = TcpListener::bind(("127.0.0.1", 0)).expect("bind real transport");
    let address = listener.local_addr().expect("transport address");
    let consumer = std::thread::spawn(move || {
        let (mut stream, _) = listener.accept().expect("accept consent");
        let mut bytes = Vec::new();
        stream
            .read_to_end(&mut bytes)
            .expect("receive canonical consent");
        SignedNativeSendPurposeV2::from_canonical_bytes(&bytes).expect("real consumer codec")
    });
    let mut stream = TcpStream::connect(address).expect("connect real transport");
    stream
        .write_all(&signed.canonical_bytes().expect("real producer codec"))
        .expect("send consent");
    stream
        .shutdown(Shutdown::Write)
        .expect("complete consent frame");
    consumer.join().expect("consumer completed")
}

fn purpose(
    key: [u8; 32],
    owner: AgentDid,
    digest: [u8; 32],
    sender: [u8; 20],
) -> NativeSendPurposeV2 {
    NativeSendPurposeV2 {
        tenant: TenantId::new("rotating-owner").unwrap(),
        agent_did: AgentDid::new(format!("did:layerx:{}", hex(&[0x33; 32]))).unwrap(),
        owner_did: owner,
        consent_public_key: key,
        evm_sender: sender,
        session_id: SessionId::new(hex(&[4; 32])).unwrap(),
        generation: 2,
        expires_at_ms: 1_900_000_000_000,
        capability_id: CapabilityId::new(hex(&[5; 32])).unwrap(),
        transaction_type: 2,
        chain_id: 125,
        activity: NativeActivity::new(1, 5).unwrap(),
        preparation_id: [6; 32],
        canonical_digest: digest,
        economic_action: [7; 32],
        idempotency_key: [8; 32],
        commitment: [9; 32],
    }
}

#[test]
fn rotated_owner_consent_crosses_transport_signing_and_reopen() {
    let directory = Directory::new();
    let (old_key, old_private, old_public) = consent_key(&directory, "original", 0x11);
    let (current_key, current_private, current_public) = consent_key(&directory, "rotated", 0x22);
    let stable_owner = AgentDid::new(format!("did:layerx:{}", hex(&old_key))).unwrap();
    assert_ne!(
        stable_owner.as_str(),
        format!("did:layerx:{}", hex(&current_key))
    );

    let corpus = parse(RECORDED).unwrap();
    let recorded = &corpus.array_at("bundles").unwrap()[0];
    let sender = recorded.hex_array_at("receiptJson.from").unwrap();
    let raw = recorded.hex_at("rawTransaction").unwrap();
    assert_eq!(&raw[..4], &[2, 0xf8, 0x72, 125]);
    // The recorded transaction has a 47-byte unsigned field list followed by
    // yParity and two 32-byte signature scalars. Preserve those original fields.
    assert_eq!(raw.len(), 117);
    assert_eq!(&raw[50..53], &[0x80, 0xa0, 0xc1]);
    let mut unsigned = vec![2, 0xef];
    unsigned.extend_from_slice(&raw[3..50]);
    let digest = sha256(&directory, &unsigned);
    let expected = purpose(current_key, stable_owner.clone(), digest, sender);
    assert_eq!(expected.clone().validate(), Ok(expected.clone()));
    let signed = sign(&directory, &current_private, expected.clone());
    assert!(signature_valid(&directory, &current_public, &signed));
    assert!(!signature_valid(&directory, &old_public, &signed));
    let received = signed_transport(&signed);
    assert_eq!(received, signed);
    received
        .purpose
        .check_current_owner(&stable_owner, &current_key, &sender)
        .unwrap();
    received
        .purpose
        .check_prepared_transaction(2, 125, &sender, &digest)
        .unwrap();

    let record = directory.path("retained-send");
    fs::write(&record, received.canonical_bytes().unwrap()).unwrap();
    drop(received);
    drop(signed);
    // Reopen the persisted production codec frame with no live consumer state.
    let mut file = fs::File::open(&record).unwrap();
    let mut retained = Vec::new();
    file.read_to_end(&mut retained).unwrap();
    drop(file);
    let recovered = SignedNativeSendPurposeV2::from_canonical_bytes(&retained).unwrap();
    assert_eq!(recovered.purpose, expected);
    assert!(signature_valid(&directory, &current_public, &recovered));
    recovered
        .purpose
        .check_current_owner(&stable_owner, &current_key, &sender)
        .unwrap();
    recovered
        .purpose
        .check_prepared_transaction(2, 125, &sender, &digest)
        .unwrap();

    let stale = sign(
        &directory,
        &old_private,
        NativeSendPurposeV2 {
            consent_public_key: old_key,
            ..expected.clone()
        },
    );
    assert!(signature_valid(&directory, &old_public, &stale));
    assert_eq!(
        stale
            .purpose
            .check_current_owner(&stable_owner, &current_key, &sender),
        Err(ContractError::Mismatch("native_send.current_owner"))
    );
    let other_owner = AgentDid::new(format!("did:layerx:{}", hex(&[0x44; 32]))).unwrap();
    assert!(recovered
        .purpose
        .check_current_owner(&other_owner, &current_key, &sender)
        .is_err());
    assert!(recovered
        .purpose
        .check_current_owner(&stable_owner, &current_key, &[1; 20])
        .is_err());
    for (kind, chain, address, hash) in [
        (3, 125, sender, digest),
        (2, 126, sender, digest),
        (2, 125, [1; 20], digest),
        (2, 125, sender, [1; 32]),
    ] {
        assert!(recovered
            .purpose
            .check_prepared_transaction(kind, chain, &address, &hash)
            .is_err());
    }
    let mutations: [fn(&mut NativeSendPurposeV2); 13] = [
        |p| p.tenant = TenantId::new("other-tenant").unwrap(),
        |p| p.agent_did = AgentDid::new(format!("did:layerx:{}", hex(&[0x55; 32]))).unwrap(),
        |p| p.owner_did = AgentDid::new(format!("did:layerx:{}", hex(&[0x66; 32]))).unwrap(),
        |p| p.consent_public_key = [1; 32],
        |p| p.evm_sender = [1; 20],
        |p| p.session_id = SessionId::new(hex(&[1; 32])).unwrap(),
        |p| p.generation += 1,
        |p| p.expires_at_ms += 1,
        |p| p.capability_id = CapabilityId::new(hex(&[1; 32])).unwrap(),
        |p| p.preparation_id[0] ^= 1,
        |p| p.canonical_digest[0] ^= 1,
        |p| p.economic_action[0] ^= 1,
        |p| p.idempotency_key[0] ^= 1,
    ];
    for mutate in mutations {
        let mut changed = recovered.clone();
        mutate(&mut changed.purpose);
        assert!(!signature_valid(&directory, &current_public, &changed));
    }
    let mut changed = recovered;
    changed.purpose.commitment[0] ^= 1;
    assert!(!signature_valid(&directory, &current_public, &changed));
}

#[test]
fn native_send_refuses_legacy_domains_and_noncanonical_frames() {
    let directory = Directory::new();
    let (key, private, _) = consent_key(&directory, "owner", 0x11);
    let owner = AgentDid::new(format!("did:layerx:{}", hex(&key))).unwrap();
    let current = purpose(key, owner.clone(), [1; 32], [2; 20]);
    let signed = sign(&directory, &private, current.clone());
    let bytes = current.canonical_bytes().unwrap();
    let frame = signed.canonical_bytes().unwrap();
    for end in 0..bytes.len() {
        assert!(NativeSendPurposeV2::from_canonical_bytes(&bytes[..end]).is_err());
    }
    for end in 0..frame.len() {
        assert!(SignedNativeSendPurposeV2::from_canonical_bytes(&frame[..end]).is_err());
    }
    let mut trailing = bytes.clone();
    trailing.push(0);
    assert!(NativeSendPurposeV2::from_canonical_bytes(&trailing).is_err());
    let mut trailing = frame.clone();
    trailing.push(0);
    assert!(SignedNativeSendPurposeV2::from_canonical_bytes(&trailing).is_err());
    let mut wrong_version = bytes.clone();
    wrong_version[NativeSendPurposeV2::DOMAIN.len()] = 1;
    assert!(NativeSendPurposeV2::from_canonical_bytes(&wrong_version).is_err());
    assert!(NativeSendPurposeV1::from_canonical_bytes(&bytes).is_err());
    for kind in [0, 1, 3, 4] {
        assert!(NativeSendPurposeV2 {
            transaction_type: kind,
            ..current.clone()
        }
        .validate()
        .is_err());
    }
    for chain in [0, 1, 126, u64::MAX] {
        assert!(NativeSendPurposeV2 {
            chain_id: chain,
            ..current.clone()
        }
        .validate()
        .is_err());
    }
    for activity in [
        NativeActivity::new(1, 1).unwrap(),
        NativeActivity::new(2, 5).unwrap(),
    ] {
        assert!(NativeSendPurposeV2 {
            activity,
            ..current.clone()
        }
        .validate()
        .is_err());
    }
    assert!(NativeSendPurposeV2 {
        consent_public_key: [0; 32],
        ..current.clone()
    }
    .validate()
    .is_err());
    assert!(NativeSendPurposeV2 {
        evm_sender: [0; 20],
        ..current.clone()
    }
    .validate()
    .is_err());
    assert!(NativeSendPurposeV2 {
        generation: 0,
        ..current.clone()
    }
    .validate()
    .is_err());
    assert!(NativeSendPurposeV2 {
        expires_at_ms: 0,
        ..current.clone()
    }
    .validate()
    .is_err());
    assert!(NativeSendPurposeV2 {
        owner_did: AgentDid::new("not-a-did").unwrap(),
        ..current.clone()
    }
    .validate()
    .is_err());
    assert!(NativeSendPurposeV2 {
        tenant: TenantId::new("x".repeat(256)).unwrap(),
        ..current.clone()
    }
    .validate()
    .is_err());
    let invalid: [fn(&mut NativeSendPurposeV2); 10] = [
        |p| p.preparation_id = [0; 32],
        |p| p.canonical_digest = [0; 32],
        |p| p.economic_action = [0; 32],
        |p| p.idempotency_key = [0; 32],
        |p| p.commitment = [0; 32],
        |p| p.session_id = SessionId::new("00").unwrap(),
        |p| p.capability_id = CapabilityId::new("00").unwrap(),
        |p| p.tenant = TenantId::new("tenant\0alias").unwrap(),
        |p| p.owner_did = AgentDid::new(format!("did:layerx:{}", "AB".repeat(32))).unwrap(),
        |p| p.agent_did = AgentDid::new(format!("did:layerx:{}", "AB".repeat(32))).unwrap(),
    ];
    for mutate in invalid {
        let mut changed = current.clone();
        mutate(&mut changed);
        assert!(changed.clone().validate().is_err());
        assert!(changed.canonical_bytes().is_err());
    }
    let mut oversized = bytes.clone();
    oversized.resize(NativeSendPurposeV2::MAX_CANONICAL_BYTES + 1, 0);
    assert!(NativeSendPurposeV2::from_canonical_bytes(&oversized).is_err());
    let mut oversized = frame.clone();
    oversized.resize(SignedNativeSendPurposeV2::MAX_CANONICAL_BYTES + 1, 0);
    assert!(SignedNativeSendPurposeV2::from_canonical_bytes(&oversized).is_err());
    let mut bad_length = frame;
    let offset = SignedNativeSendPurposeV2::DOMAIN.len();
    bad_length[offset..offset + 4].copy_from_slice(&u32::MAX.to_be_bytes());
    assert!(SignedNativeSendPurposeV2::from_canonical_bytes(&bad_length).is_err());
    assert!(SignedNativeSendPurposeV2 {
        signature: [0; 64],
        ..signed
    }
    .validate()
    .is_err());

    let legacy = NativeSendPurposeV1 {
        tenant: current.tenant,
        agent_did: current.agent_did,
        owner_did: owner,
        owner_public_key: key,
        session_id: current.session_id,
        generation: current.generation,
        expires_at_ms: current.expires_at_ms,
        capability_id: current.capability_id,
        protocol_version: 3,
        network_id: 125,
        activity: current.activity,
        preparation_id: current.preparation_id,
        canonical_digest: current.canonical_digest,
        economic_action: current.economic_action,
        idempotency_key: current.idempotency_key,
        commitment: current.commitment,
    };
    assert_eq!(
        legacy.clone().validate(),
        Err(ContractError::Mismatch("native_send.legacy_version"))
    );
    assert!(legacy.canonical_bytes().is_err());
    assert!(SignedNativeSendPurposeV1 {
        purpose: legacy,
        owner_public_key: key,
        signature: [1; 64]
    }
    .validate()
    .is_err());
    let mut legacy_domain = NativeSendPurposeV1::DOMAIN.to_vec();
    legacy_domain.extend_from_slice(&bytes[NativeSendPurposeV2::DOMAIN.len()..]);
    assert!(NativeSendPurposeV2::from_canonical_bytes(&legacy_domain).is_err());
}

fn quantity(value: &JsonValue, field: &str) -> u64 {
    u64::from_str_radix(value.str_at(field).unwrap().strip_prefix("0x").unwrap(), 16).unwrap()
}

// Preserve the native recorder's original proof fragments as opaque codec data.
fn recorded_fragment(name: &str) -> CanonicalBytes {
    let key = format!("\"{name}\":");
    let start = RECORDED.find(&key).unwrap() + key.len();
    let text = RECORDED[start..].trim_start();
    let mut quoted = false;
    let mut escaped = false;
    let mut depth = 0;
    for (index, byte) in text.bytes().enumerate() {
        if quoted {
            if escaped {
                escaped = false;
            } else if byte == b'\\' {
                escaped = true;
            } else if byte == b'"' {
                quoted = false;
            }
        } else {
            match byte {
                b'"' => quoted = true,
                b'{' | b'[' => depth += 1,
                b'}' | b']' => {
                    depth -= 1;
                    if depth == 0 {
                        parse(&text[..=index]).unwrap();
                        return CanonicalBytes::new(text.as_bytes()[..=index].to_vec()).unwrap();
                    }
                }
                _ => {}
            }
        }
    }
    panic!("incomplete native evidence {name}");
}

fn recorded_bundle(index: usize) -> (ProofBundle, [u8; 32], Freshness) {
    let corpus = parse(RECORDED).unwrap();
    let value = &corpus.array_at("bundles").unwrap()[index];
    let height = quantity(value, "receiptJson.height");
    let optional = |field| value.field(field).map(|_| recorded_fragment(field));
    let bytes = |field| CanonicalBytes::new(value.hex_at(field).unwrap()).unwrap();
    let bundle = ProofBundle {
        format: value.str_at("format").unwrap().into(),
        verification_level: Rung::from_name(value.str_at("verificationLevel").unwrap()).unwrap(),
        raw_transaction: bytes("rawTransaction"),
        signed_receipt: bytes("signedReceipt"),
        receipt_digest: value.hex_array_at("receiptDigest").unwrap(),
        receipt_key: ReceiptKeyEntry {
            key_id: value.hex_array_at("receiptKey.keyId").unwrap(),
            public_key: value.hex_array_at("receiptKey.publicKey").unwrap(),
            first_height: quantity(value, "receiptKey.firstHeight"),
            last_height: quantity(value, "receiptKey.lastHeight"),
        },
        block_header: optional("blockHeader"),
        receipt_inclusion: optional("receiptInclusion"),
        commit: optional("commit"),
        validator_set: optional("validatorSet"),
        account_proof: None,
    };
    let pins = [SequencerAuthorization::new(
        bundle.receipt_key.key_id,
        bundle.receipt_key.public_key,
        bundle.receipt_key.first_height,
        bundle.receipt_key.last_height,
    )];
    let receipt = verify_sequencer_signature(
        bundle.signed_receipt.as_bytes(),
        ReceiptKeys::HeightRanges(&pins),
    )
    .expect("genuine native producer receipt");
    let id = value.hex_array_at("receiptJson.txHash").unwrap();
    assert_eq!(receipt.protocol().unwrap().activity_id(), id);
    let hash = if index == 1 {
        // Recorded execution header's native block hash, from the successor link.
        // The API transports this coordinate; finality authentication is a consumer duty.
        [
            0x06, 0x6b, 0x3b, 0x86, 0xeb, 0x6f, 0x62, 0x43, 0x7b, 0x65, 0x9c, 0x1d, 0x07, 0x5f,
            0x28, 0xeb, 0x22, 0x24, 0x6d, 0x23, 0x3f, 0xe8, 0x6b, 0x3d, 0x5c, 0xac, 0xbd, 0xb5,
            0xe1, 0x82, 0x16, 0xe9,
        ]
    } else {
        [0; 32]
    };
    (
        bundle,
        id,
        Freshness {
            chain_head: height,
            value_height: height,
            relative_to: RelativeTo::Block { height, hash },
        },
    )
}

#[test]
fn committed_activity_and_receipt_cross_shared_rpc_codecs() {
    let (bundle, id, freshness) = recorded_bundle(1);
    assert_eq!(bundle.verification_level, Rung::Committed);
    let pinned = bundle.receipt_key;
    for target in [
        ProofBundleTarget::Activity(id),
        ProofBundleTarget::Receipt(id),
    ] {
        for requested in [Rung::Submitted, Rung::Receipted, Rung::Committed] {
            let request = ReadRequest {
                selector: target,
                requested_rung: requested,
            };
            let produced = VerifiedRead::new(bundle.clone(), Rung::Committed, freshness.clone());
            ProofBundle::check_response(&request, &produced)
                .expect("strong evidence satisfies both minima");
            let selector = target.encode().unwrap();
            let consumer_request =
                ReadRequest::proof_bundle(selector.as_bytes().to_vec(), requested.name()).unwrap();
            let listener = TcpListener::bind(("127.0.0.1", 0)).unwrap();
            let address = listener.local_addr().unwrap();
            let consumer = std::thread::spawn(move || {
                let (mut stream, _) = listener.accept().unwrap();
                let mut wire = String::new();
                stream.read_to_string(&mut wire).unwrap();
                let read = VerifiedRead::<ProofBundle>::decode_json(&wire).unwrap();
                consumer_request
                    .check_proof_response(&read)
                    .expect("RPC request consumer accepts committed evidence");
                let pins = [SequencerAuthorization::new(
                    pinned.key_id,
                    pinned.public_key,
                    pinned.first_height,
                    pinned.last_height,
                )];
                let receipt = verify_sequencer_signature(
                    read.value.signed_receipt.as_bytes(),
                    ReceiptKeys::HeightRanges(&pins),
                )
                .unwrap();
                assert_eq!(
                    receipt.protocol().unwrap().activity_id(),
                    target.activity_id()
                );
                read
            });
            let mut stream = TcpStream::connect(address).unwrap();
            stream
                .write_all(produced.encode_json().unwrap().as_bytes())
                .unwrap();
            stream.shutdown(Shutdown::Write).unwrap();
            let read = consumer.join().unwrap();
            assert_eq!(read, produced);
            ProofBundle::check_response(&request, &read).unwrap();
        }
    }
}

#[test]
fn proof_rung_refuses_downgrades_and_inconsistent_evidence() {
    let (bundle, id, freshness) = recorded_bundle(1);
    let committed = VerifiedRead::new(bundle, Rung::Committed, freshness);
    let mut receipted = committed.clone();
    receipted.achieved_rung = Rung::Receipted;
    receipted.value.verification_level = Rung::Receipted;
    receipted.value.block_header = None;
    receipted.value.receipt_inclusion = None;
    receipted.value.commit = None;
    receipted.value.validator_set = None;
    receipted
        .value
        .validate()
        .expect("valid lower-rung field set");
    for target in [
        ProofBundleTarget::Activity(id),
        ProofBundleTarget::Receipt(id),
    ] {
        let request = ReadRequest {
            selector: target,
            requested_rung: Rung::Committed,
        };
        assert_eq!(
            ProofBundle::check_response(&request, &receipted),
            Err(ContractError::OutOfRange("proof_bundle_rung"))
        );
        for requested in [Rung::Submitted, Rung::Receipted] {
            ProofBundle::check_response(
                &ReadRequest {
                    requested_rung: requested,
                    ..request.clone()
                },
                &receipted,
            )
            .unwrap();
        }
        let mut submitted = receipted.clone();
        submitted.achieved_rung = Rung::Submitted;
        submitted.value.verification_level = Rung::Submitted;
        assert!(ProofBundle::check_response(
            &ReadRequest {
                requested_rung: Rung::Submitted,
                ..request.clone()
            },
            &submitted
        )
        .is_err());
        let mut inconsistent = committed.clone();
        inconsistent.achieved_rung = Rung::Receipted;
        assert!(ProofBundle::check_response(&request, &inconsistent).is_err());
        assert!(inconsistent.encode_json().is_err());
        let wire = committed.encode_json().unwrap().replace(
            "\"achieved_verification_level\":\"committed\"",
            "\"achieved_verification_level\":\"receipted\"",
        );
        assert!(VerifiedRead::<ProofBundle>::decode_json(&wire).is_err());
        for field in 0..4 {
            let mut incomplete = committed.clone();
            match field {
                0 => incomplete.value.block_header = None,
                1 => incomplete.value.receipt_inclusion = None,
                2 => incomplete.value.commit = None,
                _ => incomplete.value.validator_set = None,
            }
            assert!(ProofBundle::check_response(&request, &incomplete).is_err());
        }
        let mut leaked = receipted.clone();
        leaked.value.commit = committed.value.commit.clone();
        assert!(ProofBundle::check_response(&request, &leaked).is_err());
    }
    let account = ReadRequest {
        selector: ProofBundleTarget::AccountState {
            activity_id: id,
            account_id: [1; 32],
        },
        requested_rung: Rung::Submitted,
    };
    assert!(ProofBundle::check_response(&account, &receipted).is_err());
    assert_eq!(
        ProofBundle::check_response(&account, &committed),
        Err(ContractError::Empty("account_proof"))
    );
}
