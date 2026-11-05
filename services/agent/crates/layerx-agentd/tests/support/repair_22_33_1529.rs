#![allow(dead_code)]
// Loopback transcript peers for the production TLS authority and runtime clock clients.
#[path = "send_authorization.rs"]
mod send_authorization;

use layerx_agentd::human::HumanPeer;
use layerx_agentd::human_runtime::{HumanAuthorityBoundary, RemoteHumanAuthority};
use layerx_agentd::identity::{CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_types::{clock::ClockReading, clock_protocol, ids::Did};
use native_tls::{Identity, TlsAcceptor};
use openssl::{
    asn1::Asn1Time,
    bn::{BigNum, MsbOption},
    hash::MessageDigest,
    pkey::{PKey, Private},
    rsa::Rsa,
    x509::{
        extension::{BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName},
        X509NameBuilder, X509,
    },
};
use serde_json::{json, Value};
use std::{
    fs,
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    os::unix::{
        fs::{MetadataExt as _, PermissionsExt as _},
        net::UnixListener,
    },
    path::Path,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc,
    },
    thread::{self, JoinHandle},
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};

pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("repair fixture: {error:?}"))
}
fn random<const N: usize>() -> [u8; N] {
    let mut bytes = [0; N];
    must(getrandom::fill(&mut bytes));
    bytes
}

pub struct Peer {
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Peer {
    fn spawn(work: impl FnOnce(Arc<AtomicBool>) + Send + 'static) -> Self {
        let stop = Arc::new(AtomicBool::new(false));
        let signal = Arc::clone(&stop);
        Self {
            stop,
            worker: Some(thread::spawn(move || work(signal))),
        }
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
    }
}

fn incoming(listener: &TcpListener) -> Option<TcpStream> {
    match listener.accept() {
        Ok((stream, _)) => {
            must(stream.set_read_timeout(Some(Duration::from_secs(3))));
            must(stream.set_write_timeout(Some(Duration::from_secs(3))));
            Some(stream)
        }
        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
            thread::sleep(Duration::from_millis(2));
            None
        }
        Err(error) => panic!("peer accept: {error}"),
    }
}

fn http_request(stream: &mut impl Read) -> (String, Vec<u8>) {
    let mut headers = Vec::new();
    while !headers.ends_with(b"\r\n\r\n") {
        assert!(headers.len() < 16_384);
        let mut byte = [0];
        must(stream.read_exact(&mut byte));
        headers.push(byte[0]);
    }
    let headers = must(String::from_utf8(headers));
    let length = headers
        .lines()
        .filter_map(|line| line.split_once(':'))
        .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
        .map_or(0, |(_, value)| must(value.trim().parse::<usize>()));
    assert!(length <= 1_048_576);
    let mut body = vec![0; length];
    must(stream.read_exact(&mut body));
    (headers, body)
}

fn http_response(stream: &mut impl Write, body: &Value) {
    let body = must(serde_json::to_vec(body));
    must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()));
    must(stream.write_all(&body));
    must(stream.flush());
}

fn certificate(issuer: Option<(&X509, &PKey<Private>)>) -> (X509, PKey<Private>) {
    let key = must(PKey::from_rsa(must(Rsa::generate(2048))));
    let mut name = must(X509NameBuilder::new());
    must(name.append_entry_by_text("CN", "localhost"));
    let name = name.build();
    let mut cert = must(X509::builder());
    must(cert.set_version(2));
    let mut serial = must(BigNum::new());
    must(serial.rand(128, MsbOption::MAYBE_ZERO, false));
    must(cert.set_serial_number(must(serial.to_asn1_integer()).as_ref()));
    must(cert.set_subject_name(&name));
    must(cert.set_issuer_name(issuer.map_or(&name, |(ca, _)| ca.subject_name())));
    must(cert.set_pubkey(&key));
    must(cert.set_not_before(must(Asn1Time::days_from_now(0)).as_ref()));
    must(cert.set_not_after(must(Asn1Time::days_from_now(1)).as_ref()));
    if let Some((ca, signing_key)) = issuer {
        must(cert.append_extension(must(BasicConstraints::new().critical().build())));
        must(
            cert.append_extension(must(
                KeyUsage::new()
                    .digital_signature()
                    .key_encipherment()
                    .build(),
            )),
        );
        must(cert.append_extension(must(ExtendedKeyUsage::new().server_auth().build())));
        let san = must(
            SubjectAlternativeName::new()
                .dns("localhost")
                .build(&cert.x509v3_context(Some(ca), None)),
        );
        must(cert.append_extension(san));
        must(cert.sign(signing_key, MessageDigest::sha256()));
    } else {
        must(cert.append_extension(must(BasicConstraints::new().critical().ca().build())));
        must(
            cert.append_extension(must(
                KeyUsage::new()
                    .critical()
                    .key_cert_sign()
                    .crl_sign()
                    .build(),
            )),
        );
        must(cert.sign(&key, MessageDigest::sha256()));
    }
    (cert.build(), key)
}

pub struct AuthorityIdentity<'a> {
    pub authority: &'a mut RemoteHumanAuthority,
    pub peer: &'a HumanPeer,
}
impl IdentityResolver for AuthorityIdentity<'_> {
    fn resolve(&mut self, did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        self.authority.core_identity(self.peer, did).map(Some)
    }
}

pub fn clock_peer(root: &Path) -> (Arc<RuntimeClock>, Peer) {
    let path = root.join("clock.sock");
    let listener = must(UnixListener::bind(&path));
    must(fs::set_permissions(
        &path,
        fs::Permissions::from_mode(0o600),
    ));
    must(listener.set_nonblocking(true));
    let generation = random::<16>();
    let worker = Peer::spawn(move |stop| {
        let start = Instant::now();
        while !stop.load(Ordering::Acquire) {
            match listener.accept() {
                Ok((mut stream, _)) => {
                    must(stream.set_read_timeout(Some(Duration::from_secs(3))));
                    let mut bytes = [0; clock_protocol::REQUEST_BYTES];
                    must(stream.read_exact(&mut bytes));
                    let request = must(clock_protocol::Request::decode(&bytes));
                    thread::sleep(Duration::from_nanos(request.wait_nanoseconds));
                    let reading = ClockReading {
                        generation,
                        unix_milliseconds: must(SystemTime::now().duration_since(UNIX_EPOCH))
                            .as_millis()
                            .try_into()
                            .expect("wall time"),
                        monotonic_nanoseconds: must(start.elapsed().as_nanos().try_into()),
                    };
                    must(stream.write_all(&clock_protocol::response(request.counter, Ok(reading))));
                }
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    thread::sleep(Duration::from_millis(2))
                }
                Err(error) => panic!("clock accept: {error}"),
            }
        }
    });
    let clock = must(RuntimeClock::connect(ClockBinding {
        socket: path,
        process: std::process::id(),
        user: must(fs::metadata(root)).uid(),
    }));
    (clock, worker)
}

pub fn peer() -> HumanPeer {
    HumanPeer {
        uid: must(fs::metadata(".")).uid(),
        principal: "operator:incident-response".into(),
        tenant: "tenant-a".into(),
        subject: None,
    }
}

pub fn identity_document(head: u64, frozen: bool, authorities: &[ProtocolAuthority]) -> Value {
    json!({"canonical_core_bytes":hex::encode(format!("verified-identity-at-{head}")),
        "head_sequence":head,"revocation_sequence":1,"frozen":frozen,
        "verification_level":"checkpoint_finalised", "authorities":authorities.iter().map(|authority| {
            let (kind,id) = match authority { ProtocolAuthority::PrimaryKey(id)=>("primary_key",id),
                ProtocolAuthority::SessionKey(id)=>("session_key",id), ProtocolAuthority::CapabilityGrant(id)=>("capability_grant",id) };
            json!({"kind":kind,"id":hex::encode(id)})
        }).collect::<Vec<_>>()})
}

pub fn authority_peer(exchanges: Vec<(u16, Value)>) -> (RemoteHumanAuthority, JoinHandle<()>) {
    let (ca, ca_key) = certificate(None);
    let (cert, key) = certificate(Some((&ca, &ca_key)));
    let acceptor = must(TlsAcceptor::new(must(Identity::from_pkcs8(
        &must(cert.to_pem()),
        &must(key.private_key_to_pem_pkcs8()),
    ))));
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    must(listener.set_nonblocking(true));
    let authority = must(RemoteHumanAuthority::connect(
        &format!("https://localhost:{}", must(listener.local_addr()).port()),
        "repair-authority-transcript-credential".into(),
        Duration::from_secs(3),
        1_048_576,
        &must(ca.to_der()),
    ));
    let worker = thread::spawn(move || {
        for (status, value) in exchanges {
            let expires = Instant::now() + Duration::from_secs(10);
            let socket = loop {
                if let Some(stream) = incoming(&listener) {
                    break stream;
                }
                assert!(Instant::now() < expires, "missing authority request");
            };
            let mut stream = must(acceptor.accept(socket));
            let (headers, body) = http_request(&mut stream);
            assert!(body.is_empty());
            assert!(headers.starts_with("GET /v1/agent/identity?"));
            assert!(headers
                .to_ascii_lowercase()
                .contains("authorization: bearer repair-authority-transcript-credential"));
            let body = must(serde_json::to_vec(&value));
            must(write!(stream,"HTTP/1.1 {status} Transcript\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",body.len()));
            must(stream.write_all(&body));
            must(stream.flush());
        }
    });
    (authority, worker)
}

pub fn rpc_peer(exchanges: Vec<(&'static str, Value)>) -> (String, JoinHandle<Vec<Value>>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    must(listener.set_nonblocking(true));
    let worker = thread::spawn(move || {
        let mut requests = Vec::new();
        for (method, result) in exchanges {
            let expires = Instant::now() + Duration::from_secs(10);
            let mut stream = loop {
                if let Some(s) = incoming(&listener) {
                    break s;
                }
                assert!(Instant::now() < expires, "missing {method}");
            };
            let (_, body) = http_request(&mut stream);
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            http_response(
                &mut stream,
                &json!({"jsonrpc":"2.0","id":request["id"],"result":result}),
            );
            requests.push(request);
        }
        requests
    });
    (url, worker)
}

// Produces signed sequence-window transcripts with the production envelope/receipt/Merkle codecs.
// A batch has N activity leaves plus the maintenance leaf at its last sequence.
pub fn cumulative_window(
    actor: &Did,
) -> (
    layerx_agentd::protocol_evidence::EvidenceAuthority,
    Vec<layerx_agentd::protocol_evidence::RawActivityReceiptEvidence>,
) {
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_agentd::protocol_evidence::{RawActivityReceiptEvidence, RawReceiptEvidence};
    use layerx_crypto::SignatureMessage;
    use layerx_proof::merkle::build_proof;
    use layerx_types::{
        activity::{Authority, EnvelopeBuilder, Signature, TimestampBound},
        amount::Amount,
        ids::IdempotencyKey,
        payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload},
    };
    use layerx_wire::{
        activity::{encode_signed_envelope, encode_unsigned_envelope},
        encode::Encoder,
        hash::{activity_id, batch_header_digest, payload_hash_for, Domain},
        limits::PROTOCOL_VERSION,
    };
    let owner = SigningKey::from_bytes(&[0x45; 32]);
    let kind = must(ActivityType::new(ModuleId::Asset, 5));
    let registry = must(ModuleRegistry::new(&[must(ModuleRegistration::new(
        ModuleId::Asset,
        &[kind],
    ))]));
    let mut activities = Vec::new();
    let mut receipts = Vec::new();
    for index in 0..99u64 {
        let mut payload = Encoder::new(1024);
        must(payload.u16(0x5301));
        must(payload.u16(10));
        for value in [[0x11; 32], [0x22; 32], [0x33; 32]] {
            must(payload.fixed(&value));
        }
        must(payload.u128(25));
        must(payload.u64(index + 1));
        let mut action = [0x44; 32];
        action[..8].copy_from_slice(&index.to_be_bytes());
        must(payload.fixed(&action));
        must(payload.u64(1_010));
        must(payload.fixed(&[0x55; 32]));
        must(payload.u8(0));
        must(payload.u8(1));
        must(payload.fixed(&[0x11; 32]));
        must(payload.fixed(&[0x66; 32]));
        must(payload.fixed(&[0x77; 64]));
        must(payload.fixed(&[0x55; 32]));
        must(payload.u32(17));
        must(payload.u16(PROTOCOL_VERSION));
        let payload = must(Payload::new(
            &registry,
            kind,
            &send_authorization::sign(payload.finish()),
        ));
        let mut builder = EnvelopeBuilder::new();
        must(builder.protocol_version(PROTOCOL_VERSION));
        must(builder.network_id(42));
        must(builder.activity_type(kind));
        must(builder.actor_did(if index == 0 {
            actor.clone()
        } else {
            must(Did::new(b"did:layerx:other-window-actor"))
        }));
        must(builder.authority(must(Authority::owner(&owner.verifying_key().to_bytes()))));
        must(builder.account_sequence(index + 1));
        must(builder.timestamp_bound(must(TimestampBound::new(995, 1_010))));
        must(builder.idempotency_key(IdempotencyKey::new(action)));
        must(builder.fee_limit(Amount::from_u128(7)));
        must(builder.payload_hash(must(payload_hash_for(&payload))));
        must(builder.payload(payload));
        let envelope = must(builder.build());
        let unsigned = must(encode_unsigned_envelope(&envelope));
        let digest = must(SignatureMessage::new(
            Domain::SignaturePreimage,
            PROTOCOL_VERSION,
            42,
            &unsigned,
        ))
        .digest();
        let signed =
            envelope.attach_signature(must(Signature::new(&owner.sign(&digest).to_bytes())));
        let canonical = must(encode_signed_envelope(&signed));
        let decoded = must(layerx_wire::activity::decode_signed(&canonical, &registry));
        let id = must(activity_id(&decoded));
        activities.push(canonical);
        receipts.push(
            super::support::raw_receipt_at(id, 0, 25, 20 + index)
                .canonical_receipt()
                .to_vec(),
        );
    }
    receipts.push(
        super::support::raw_receipt_at([0xfe; 32], 0, 0, 119)
            .canonical_receipt()
            .to_vec(),
    );
    let leaves: Vec<_> = receipts.iter().map(Vec::as_slice).collect();
    let (_, receipt_root) = must(build_proof(&leaves, 0));
    let activity_leaves: Vec<_> = activities.iter().map(Vec::as_slice).collect();
    let (_, activity_root) = must(build_proof(&activity_leaves, 0));
    let key = SigningKey::from_bytes(&[0x3a; 32]);
    let previous_state_root = [0x21; 32];
    let resulting_state_root = [0x22; 32];
    let first = 20u64;
    let last = 119u64;
    let sequencer_id = key.verifying_key().to_bytes();
    let mut encoder = Encoder::new(354);
    assert_eq!(
        encoder.structure_header_version(0x1701, layerx_wire::limits::PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u8(15), Ok(()));
    let fields: [(u8, Vec<u8>); 15] = [
        (
            1,
            layerx_wire::limits::PROTOCOL_VERSION.to_be_bytes().to_vec(),
        ),
        (2, 42_u32.to_be_bytes().to_vec()),
        (3, 2_u64.to_be_bytes().to_vec()),
        (4, 7_u64.to_be_bytes().to_vec()),
        (5, first.to_be_bytes().to_vec()),
        (6, last.to_be_bytes().to_vec()),
        (7, previous_state_root.to_vec()),
        (8, resulting_state_root.to_vec()),
        (9, activity_root.to_vec()),
        (10, receipt_root.to_vec()),
        (11, [0x34; 32].to_vec()),
        (12, [0x35; 32].to_vec()),
        (13, [0x36; 32].to_vec()),
        (14, 1_000_u64.to_be_bytes().to_vec()),
        (15, sequencer_id.to_vec()),
    ];
    for (field, value) in fields {
        assert_eq!(encoder.tag(field, 15), Ok(()));
        match field {
            1 => assert_eq!(
                encoder.u16(u16::from_be_bytes([value[0], value[1]])),
                Ok(())
            ),
            2 => assert_eq!(
                encoder.u32(u32::from_be_bytes(
                    value
                        .as_slice()
                        .try_into()
                        .unwrap_or_else(|_| panic!("u32 field"))
                )),
                Ok(())
            ),
            3..=6 | 14 => assert_eq!(
                encoder.u64(u64::from_be_bytes(
                    value
                        .as_slice()
                        .try_into()
                        .unwrap_or_else(|_| panic!("u64 field"))
                )),
                Ok(())
            ),
            _ => assert_eq!(encoder.bytes(&value, 32), Ok(())),
        }
    }
    let header = encoder.finish();
    let header_digest =
        batch_header_digest(&header).unwrap_or_else(|error| panic!("header digest: {error:?}"));

    let entries = activities
        .into_iter()
        .enumerate()
        .map(|(index, activity)| {
            let (proof, _) = must(build_proof(&leaves, index));
            RawActivityReceiptEvidence::from_signed_inclusion(
                activity,
                RawReceiptEvidence::new(
                    receipts[index].clone(),
                    proof,
                    header.clone(),
                    key.sign(&header_digest).to_bytes(),
                ),
            )
        })
        .collect();
    (super::support::pinned_evidence_verifier(), entries)
}

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_wire::{
    encode::Encoder,
    hash::receipt_digest,
    limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION},
};
use sha2::{Digest as _, Sha256};
const HEIGHT: u64 = 8;
const KEY_ID: [u8; 32] = [1; 32];
fn rpc_hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}
fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}
// Signed sequencer transcript producer using the committed support codec and ABI event vector.
fn encoded_node_receipt(
    tx_hash: [u8; 32],
    sender: [u8; 20],
    nonce: u64,
    destination: [u8; 20],
    emitted: &layerx_client::sequencer::SequencerLog,
    success: bool,
    signature: Option<[u8; 64]>,
) -> Vec<u8> {
    let previous = [4; 32];
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend_from_slice(&previous);
    sequence.extend_from_slice(&7_u64.to_be_bytes());
    sequence.extend_from_slice(&tx_hash);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&destination);
    let vm_error = if success { "" } else { "execution reverted" };
    let mut summary = Encoder::new(256);
    must(summary.u64(21_000));
    must(summary.u64(42_000));
    must(summary.u64(1));
    must(summary.u32(1));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&sha256(vm_error.as_bytes())));
    must(summary.u32(2));
    must(summary.u32(1));
    must(summary.u8(4));
    must(summary.fixed(b"upax"));
    let mut log = emitted.address.to_vec();
    log.push(must(u8::try_from(emitted.topics.len())));
    for topic in &emitted.topics {
        log.extend_from_slice(topic);
    }
    log.extend_from_slice(&sha256(&emitted.data));

    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    must(encoder.structure_header_version(0x5201, PROTOCOL_VERSION));
    must(encoder.u16(PROTOCOL_VERSION));
    must(encoder.bytes(&tx_hash, 32));
    must(encoder.u64(7));
    must(encoder.bytes(&previous, 32));
    must(encoder.bytes(&sha256(&sequence), 32));
    must(encoder.bytes(&sha256(&[0xab, 0xcd]), 32));
    must(encoder.i32(i32::from(!success)));
    must(encoder.sequence_length(2, 512));
    for (ordinal, kind, body) in [(0, 1, summary.finish()), (1, 3, log)] {
        must(encoder.u16(12));
        must(encoder.u16(ordinal));
        must(encoder.u16(0));
        must(encoder.u8(kind));
        must(encoder.u8(0));
        must(encoder.bytes(&[0; 32], 32));
        must(encoder.bytes(&body, 256));
    }
    must(encoder.u128(21_000));
    must(encoder.bytes(&height, 32));
    must(encoder.u16(12));
    must(encoder.u32(1));
    must(encoder.u32(0));
    must(encoder.u8(2));
    must(encoder.bytes(&sha256(b"upax"), 32));
    must(encoder.u128(1));
    must(encoder.bytes(&from, 32));
    must(encoder.u128(21_002));
    must(encoder.u128(if success { 1 } else { 2 }));
    must(encoder.u64(nonce));
    must(encoder.bytes(&to, 32));
    must(encoder.u128(0));
    must(encoder.u128(u128::from(success)));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&KEY_ID, 32));
    must(encoder.u64(1_000));
    must(encoder.u8(u8::from(signature.is_some())));
    if let Some(signature) = signature {
        must(encoder.bytes(&signature, 64));
    }
    encoder.finish()
}

pub fn signed_node_receipt(
    raw: &[u8],
    success: bool,
    emitted: &layerx_client::sequencer::SequencerLog,
) -> (Value, ReceiptKeyEntry) {
    let (transaction, signature) = must(decode_signed(raw));
    let sender = must(recover_sender(&transaction, &signature));
    let tx_hash = transaction_hash(raw);
    let nonce = transaction.nonce;
    let signer = SigningKey::from_bytes(&[8; 32]);
    let unsigned = encoded_node_receipt(
        tx_hash,
        sender,
        nonce,
        transaction.to,
        emitted,
        success,
        None,
    );
    let signature = signer.sign(&must(receipt_digest(&unsigned))).to_bytes();
    let bytes = encoded_node_receipt(
        tx_hash,
        sender,
        nonce,
        transaction.to,
        emitted,
        success,
        Some(signature),
    );
    let key = ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: signer.verifying_key().to_bytes(),
        first_height: 1,
        last_height: 199,
    };
    let authorizations = [layerx_proof::inclusion::SequencerAuthorization::new(
        key.key_id,
        key.public_key,
        key.first_height,
        key.last_height,
    )];
    let verified = must(layerx_proof::receipt::verify_sequencer_signature(
        &bytes,
        authorizations.as_slice(),
    ));
    let logs = must(layerx_proof::sequencer_logs::verify_logs(
        &verified,
        vec![layerx_proof::sequencer_logs::SequencerLog::new(
            emitted.address,
            emitted.topics.clone(),
            emitted.data.clone(),
            emitted.log_index,
        )],
    ));
    assert_eq!(logs.summary().gas_used(), 21_000);
    assert_eq!(logs.summary().return_data_len(), 2);
    let view = json!({
        "rung": "receipted", "txHash": rpc_hex(&tx_hash), "nonce": format!("{nonce:#x}"),
        "seq": "0x7", "height": format!("{HEIGHT:#x}"), "index": "0x1", "timestamp": "0x3e8",
        "status": if success { "0x1" } else { "0x0" }, "returnData": "0xabcd",
        "logs": [{"address": rpc_hex(&emitted.address), "topics": emitted.topics.iter().map(|topic| rpc_hex(topic)).collect::<Vec<_>>(),
            "data": rpc_hex(&emitted.data), "logIndex": "0x0"}],
        "gasUsed": "0x5208", "cumulativeGasUsed": "0xa410", "effectiveGasPrice": "0x1",
        "feeDenom": "upax", "paymentDenom": "upax", "feeAmount": "0x5208", "sponsor": rpc_hex(&[0; 20]),
        "from": rpc_hex(&sender), "to": rpc_hex(&transaction.to), "contractAddress": rpc_hex(&[0; 20]),
        "vmError": if success { "" } else { "execution reverted" },
        "keyId": rpc_hex(&KEY_ID), "receipt": rpc_hex(&bytes),
        "signingKey": rpc_hex(&key.public_key)
    });
    (view, key)
}
