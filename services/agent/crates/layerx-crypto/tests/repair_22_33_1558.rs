//! Producer -> canonical transaction/SSH transport -> production consumer proofs.
//! Findings and out-of-scope migrations are recorded in src/repair_22_33_1558.kvx.

mod support;

use std::future::Future;
use std::io::{Read as _, Write as _};
use std::os::unix::net::UnixListener;
use std::path::{Path, PathBuf};
use std::pin::pin;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::thread::JoinHandle;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::binding::{BindAuthorization, BindMessage, BIND_MESSAGE_BYTES};
use layerx_crypto::disclosure::call::{bind_call, validate_call, ConstructMethod};
use layerx_crypto::evm_transaction::{
    assemble, decode_signed, decode_unsigned, recover_sender, Eip1559Call,
};
use layerx_crypto::keystore::{Keystore, KeystoreEntropy, KeystoreError};
use layerx_crypto::send::{
    ledger_payment_call, send_context_hash, CallOptions, DisclosedCall, SendCondition, SendDebit,
};
use layerx_crypto::signer::{
    sign_disclosed_call, KeystoreSigner, LocalSigner, SignError, Signer, SigningRequest,
};

struct ReadyWake;
impl Wake for ReadyWake {
    fn wake(self: Arc<Self>) {}
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(ReadyWake));
    match future.as_mut().poll(&mut Context::from_waker(&waker)) {
        Poll::Ready(result) => result,
        Poll::Pending => panic!("synchronous signer path unexpectedly yielded"),
    }
}

fn hex(text: &str) -> Vec<u8> {
    let text = text.trim();
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("fixture UTF-8"), 16)
                .expect("fixture hex")
        })
        .collect()
}

fn signer_and_options() -> (LocalSigner, CallOptions) {
    let account = derive_from_seed(&[7; 64], 0).expect("derived account");
    let signer = LocalSigner::from_derived_account(&account);
    let options = CallOptions {
        from: account.evm_address(),
        chain_id: 125,
        nonce: 9,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
    };
    (signer, options)
}

fn bind_message(options: &CallOptions, bind_nonce: u64) -> BindMessage {
    let mut chain = [0; 32];
    chain[24..].copy_from_slice(&options.chain_id.to_be_bytes());
    BindMessage::new(chain, options.from, bind_nonce)
}

fn debit() -> SendDebit {
    SendDebit {
        from: [1; 32],
        to: [2; 32],
        asset: [3; 32],
        amount: 123,
        source_sequence: 7,
        idempotency_key: [4; 32],
        expires_at: 100,
        context_hash: send_context_hash(&[1; 32], &[2; 32], &[3; 32], 123, &[4; 32]),
        conditions: vec![SendCondition {
            kind: 1,
            timestamp: 10,
        }],
        authorization_kind: 1,
        network_id: 17,
        protocol_version: 3,
    }
}

fn payment_round_trip(payload: &[u8]) -> DisclosedCall {
    let (signer, options) = signer_and_options();
    let produced = ledger_payment_call(payload, &options).expect("produce payment call");
    let bytes = produced.call.encode_unsigned();
    assert_eq!(bytes[0], 2);
    let received = decode_unsigned(&bytes).expect("consume type-2 bytes");
    let consumed = DisclosedCall::from_call(received.clone(), options.from).expect("bind call");
    assert_eq!(consumed.disclosure, produced.disclosure);
    assert_eq!(consumed.disclosure.reencode().expect("reencode"), bytes);
    consumed
        .disclosure
        .validate_canonical(&bytes)
        .expect("canonical validation");
    let signature = ready(sign_disclosed_call(
        &signer,
        &received,
        &consumed.disclosure,
        options.from,
    ))
    .expect("EVM signing");
    let wire = assemble(&received, &signature);
    let (committed_call, committed_signature) = decode_signed(&wire).expect("signed transport");
    let sender = recover_sender(&committed_call, &committed_signature).expect("recover sender");
    assert_eq!(sender, options.from);
    assert_eq!(
        bind_call(&committed_call, sender).expect("consumer disclosure"),
        produced.disclosure
    );
    consumed
}

// Compatibility is retained only with an actionable Rust deprecation notice.
fn assert_deprecated(source: &str, declaration: &str) {
    let offset = source
        .find(declaration)
        .expect("live compatibility declaration");
    let start = source[..offset].rfind("\n\n").map_or(0, |index| index + 2);
    assert!(
        source[start..offset].contains("#[deprecated("),
        "{declaration} must be deprecated"
    );
}

#[test]
fn activity_disclosure_still_public_is_closed_by_transaction_round_trip() {
    let source = include_str!("../src/disclosure.rs");
    for declaration in ["pub fn bind(", "pub fn bind_budget_mutation("] {
        assert!(
            !source.contains(declaration),
            "retired declaration {declaration}"
        );
    }
    assert!(!include_str!("../src/signer.rs").contains("pub fn sign_disclosed<'a>("));
    let payer = LocalSigner::new([5; 32]);
    let payload = ready(debit().sign(&payer)).expect("native authorization producer");
    let produced = payment_round_trip(&payload);
    assert_eq!(
        produced
            .disclosure
            .payload_sequence()
            .expect("source sequence"),
        Some(7)
    );
    assert_eq!(produced.disclosure.envelope_sequence(), 9);
    let (_, options) = signer_and_options();
    for index in 0..7 {
        let mut changed = produced.call.clone();
        match index {
            0 => changed.chain_id += 1,
            1 => changed.nonce += 1,
            2 => changed.max_priority_fee_per_gas += 1,
            3 => changed.max_fee_per_gas += 1,
            4 => changed.gas_limit += 1,
            5 => changed.to[19] ^= 1,
            _ => changed.data[68] ^= 1,
        }
        assert!(validate_call(&changed, &produced.disclosure, options.from).is_err());
        assert!(produced
            .disclosure
            .validate_canonical(&changed.encode_unsigned())
            .is_err());
    }
    assert!(validate_call(&produced.call, &produced.disclosure, [9; 20]).is_err());
    let retired_activity = support::archived_send(25);
    assert!(decode_unsigned(&retired_activity).is_err());
}

#[test]
fn api_contract_document_absent_is_closed_by_owned_contract_and_raw_bind_vector() {
    let contract = include_str!("../src/api-contract.md");
    assert!(include_str!("../src/lib.rs").contains("#![doc = include_str!(\"api-contract.md\")]"));
    for term in [
        "## Crypto",
        "## Proof",
        "receipt",
        "pinned",
        "height",
        "submitted",
        "receipted",
        "committed",
        "cannot establish this rung",
        "SignError",
        "sign_disclosed_call",
    ] {
        assert!(contract.contains(term), "missing contract term {term}");
    }
    // Published chain/client vector: a hash of this message would produce a
    // different signature, so this proves the raw-message signing contract.
    let message_bytes = hex("4c583a5041584545522d42494e443a763100000000000000000000000000000000000000000000000000000000000ae3f2102132435465768798a9bacbdcedfe0f1e2d3c4b0000000000000000");
    let binding = BindMessage::decode(&message_bytes).expect("published consent");
    assert_eq!(binding.as_bytes().len(), BIND_MESSAGE_BYTES);
    let signer = LocalSigner::new([0x61; 32]);
    let signature =
        ready(signer.sign(SigningRequest::bind(&binding))).expect("raw consent signing");
    assert_eq!(signature.as_bytes().as_slice(), hex("c815ff4b2c34fe06ed50e904133b8f4cc618255ce6493d1a1177480eb2e959ed16f055fbe0e12885c199bf662b430e4f9f131addb2aa705fde963fa5995ece0f"));
    binding
        .verify(&signer.public_key(), signature.as_bytes())
        .expect("chain-compatible verification");
}

struct TestDirectory(PathBuf);
impl TestDirectory {
    fn new() -> Self {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let directory = std::env::temp_dir().join(format!(
            "crypto-1558-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed),
        ));
        std::fs::create_dir(&directory).expect("isolated fixture directory");
        Self(directory)
    }
}
impl Drop for TestDirectory {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0).expect("remove fixture directory");
    }
}

#[test]
fn crypto_retired_exports_are_closed_while_key_storage_and_signing_reopen() {
    let exports = include_str!("../src/lib.rs");
    assert_deprecated(exports, "pub mod onboarding;");
    assert!(!exports.contains("pub mod settlement_recipient;"));
    assert_deprecated(
        include_str!("../src/payments.rs"),
        "pub use crate::onboarding",
    );
    let directory = TestDirectory::new();
    let path = directory.0.join("identity.encrypted");
    let seed = [0x61; 32];
    let secret = b"committed high-entropy fixture secret";
    let identity = b"did:layerx:af06a3e3291714e4f356c19c9b15cd1951ec6e6662aa77be07547f289383341d";
    let store = Keystore::seal(
        &seed,
        secret,
        identity,
        125,
        KeystoreEntropy::new([0x31; 16], [0x42; 24]).expect("fixture entropy"),
    )
    .expect("seal production store");
    std::fs::write(&path, store.to_bytes().expect("persist encrypted store")).expect("write store");
    drop(store);
    let recovered = Keystore::from_bytes(&std::fs::read(path).expect("read reopened store"))
        .expect("decode store");
    assert_eq!(
        recovered.open(secret, identity, 126).err(),
        Some(KeystoreError::NetworkMismatch)
    );
    let signer = recovered
        .open_with(secret, identity, 125, |seed| LocalSigner::new(*seed))
        .expect("recover signer");
    let (_, options) = signer_and_options();
    let consent = bind_message(&options, 2);
    let signature =
        ready(signer.sign(SigningRequest::bind(&consent))).expect("recovered bind signing");
    consent
        .verify(&signer.public_key(), signature.as_bytes())
        .expect("recovered signature");
    assert_eq!(
        ready(signer.sign_transaction(&Eip1559Call {
            chain_id: 125,
            nonce: 0,
            max_priority_fee_per_gas: 2,
            max_fee_per_gas: 30,
            gas_limit: 100_000,
            to: options.from,
            data: vec![],
        })),
        Err(SignError::EvmKeyUnavailable)
    );
}

#[test]
fn nested_activity_onboarding_is_closed_by_registration_transaction_and_consent() {
    let legacy = include_str!("../src/onboarding.rs");
    assert_deprecated(legacy, "pub struct OnboardingConsent");
    assert_deprecated(legacy, "pub struct SponsoredRegistration");
    let (signer, options) = signer_and_options();
    let consent = bind_message(&options, 2);
    let signature = ready(signer.sign(SigningRequest::bind(&consent))).expect("DID consent");
    let authorization = BindAuthorization {
        message: consent,
        public_key: signer.public_key(),
        signature: *signature.as_bytes(),
    };
    let call = authorization
        .to_call(&options)
        .expect("registration producer");
    assert_eq!(call.nonce, 9, "EVM nonce is independent from bind nonce 2");
    let tx_signature = ready(signer.sign_transaction(&call)).expect("EVM owner consent");
    let wire = assemble(&call, &tx_signature);
    let (received, signature) = decode_signed(&wire).expect("registration transaction");
    let sender = recover_sender(&received, &signature).expect("transaction sender");
    let consumed =
        BindAuthorization::from_call(&received, sender, 2).expect("registration consumer");
    assert_eq!(consumed, authorization);
    assert!(BindAuthorization::from_call(&received, sender, 3).is_err());
    assert!(BindAuthorization::from_call(&received, [0x32; 20], 2).is_err());
    for offset in 0..received.data.len() {
        let mut changed = received.clone();
        changed.data[offset] ^= 1;
        assert!(
            BindAuthorization::from_call(&changed, sender, 2).is_err(),
            "changed ABI byte {offset}"
        );
    }
    let mut other_chain = received.clone();
    other_chain.chain_id += 1;
    assert!(BindAuthorization::from_call(&other_chain, sender, 2).is_err());
    let mut wrong_target = received.clone();
    wrong_target.to[19] ^= 1;
    assert!(BindAuthorization::from_call(&wrong_target, sender, 2).is_err());
    for length in 0..received.data.len() {
        let mut truncated = received.clone();
        truncated.data.truncate(length);
        assert!(BindAuthorization::from_call(&truncated, sender, 2).is_err());
    }
    let mut trailing = received;
    trailing.data.push(0);
    assert!(BindAuthorization::from_call(&trailing, sender, 2).is_err());
}

#[test]
fn old_send_envelope_constructor_is_closed_with_native_payload_compatibility() {
    let source = include_str!("../src/send.rs");
    for declaration in [
        "pub struct EnvelopeOptions",
        "pub struct DisclosedEnvelope",
        "pub fn encode_send_envelope(",
    ] {
        assert!(
            !source.contains(declaration),
            "retired declaration {declaration}"
        );
    }
    let fixture = hex(include_str!("fixtures/payments/native-1-5.hex"));
    let consumed = payment_round_trip(&fixture);
    assert_eq!(
        consumed.disclosure.construct_method().expect("method"),
        Some(ConstructMethod::LedgerSend {
            payload: fixture.clone()
        })
    );
    assert_eq!(consumed.disclosure.amounts[0].value, 123);
    assert_eq!(consumed.disclosure.counterparties[0].account, [1; 32]);
    assert_eq!(consumed.disclosure.counterparties[1].account, [2; 32]);
    assert_eq!(&consumed.call.data[68..68 + fixture.len()], fixture);
    let (_, options) = signer_and_options();
    for offset in 0..fixture.len() {
        let mut changed = fixture.clone();
        changed[offset] ^= 1;
        assert!(
            ledger_payment_call(&changed, &options).is_err(),
            "changed payload byte {offset}"
        );
    }
    for length in 0..fixture.len() {
        assert!(ledger_payment_call(&fixture[..length], &options).is_err());
    }
    let mut trailing = fixture;
    trailing.push(0);
    assert!(ledger_payment_call(&trailing, &options).is_err());
}

#[test]
fn recipient_binding_not_signed_is_closed_by_surface_retirement() {
    assert!(!Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("src/settlement_recipient.rs")
        .exists());
    assert!(!include_str!("../src/lib.rs").contains("pub mod settlement_recipient;"));
    // The supported replacement binds all ownership facts in the actual DID
    // signature. There is no independently replaceable custody digest field.
    let (signer, options) = signer_and_options();
    let consent = bind_message(&options, 2);
    let signature = ready(signer.sign(SigningRequest::bind(&consent))).expect("binding signature");
    consent
        .verify(&signer.public_key(), signature.as_bytes())
        .expect("original consent");
    for offset in 0..BIND_MESSAGE_BYTES {
        let mut changed = *consent.as_bytes();
        changed[offset] ^= 1;
        match BindMessage::decode(&changed) {
            Ok(changed) => assert!(changed
                .verify(&signer.public_key(), signature.as_bytes())
                .is_err()),
            Err(error) => assert_eq!(error, SignError::InvalidDisclosure),
        }
    }
}

fn ssh_string(bytes: &[u8], out: &mut Vec<u8>) {
    out.extend_from_slice(
        &u32::try_from(bytes.len())
            .expect("bounded SSH string")
            .to_be_bytes(),
    );
    out.extend_from_slice(bytes);
}
fn read_string<'a>(bytes: &'a [u8], cursor: &mut usize) -> &'a [u8] {
    let length =
        u32::from_be_bytes(bytes[*cursor..*cursor + 4].try_into().expect("SSH length")) as usize;
    *cursor += 4;
    let value = &bytes[*cursor..*cursor + length];
    *cursor += length;
    value
}

/// A committed-key OpenSSH protocol peer fixture. The component under repair
/// is the unmodified production KeystoreSigner, including real Unix I/O and
/// strict Ed25519 response verification; no external ssh-agent binary is used.
struct AgentPeer {
    socket: PathBuf,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<Vec<Vec<u8>>>>,
}
impl AgentPeer {
    fn start(socket: &Path, seed: [u8; 32]) -> Self {
        let listener = UnixListener::bind(socket).expect("bind SSH agent peer");
        listener.set_nonblocking(true).expect("nonblocking accept");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let thread = std::thread::spawn(move || {
            let key = SigningKey::from_bytes(&seed);
            let mut expected_blob = Vec::new();
            ssh_string(b"ssh-ed25519", &mut expected_blob);
            ssh_string(&key.verifying_key().to_bytes(), &mut expected_blob);
            let mut transcript = Vec::new();
            while !stopped.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("SSH accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(2)))
                    .expect("read bound");
                stream
                    .set_write_timeout(Some(Duration::from_secs(2)))
                    .expect("write bound");
                let mut length = [0; 4];
                stream.read_exact(&mut length).expect("request length");
                let length = u32::from_be_bytes(length) as usize;
                assert!((1..=2048).contains(&length));
                let mut request = vec![0; length];
                stream.read_exact(&mut request).expect("SSH request");
                assert_eq!(request[0], 13);
                let mut cursor = 1;
                let requested_key = read_string(&request, &mut cursor);
                let message = read_string(&request, &mut cursor);
                assert_eq!(&request[cursor..], &[0; 4], "SSH flags");
                transcript.push(message.to_vec());
                let response = if requested_key == expected_blob {
                    let mut signature_blob = Vec::new();
                    ssh_string(b"ssh-ed25519", &mut signature_blob);
                    ssh_string(&key.sign(message).to_bytes(), &mut signature_blob);
                    let mut response = vec![14];
                    ssh_string(&signature_blob, &mut response);
                    response
                } else {
                    vec![5]
                };
                stream
                    .write_all(&(response.len() as u32).to_be_bytes())
                    .expect("response length");
                stream.write_all(&response).expect("SSH signature response");
            }
            transcript
        });
        Self {
            socket: socket.to_path_buf(),
            stop,
            thread: Some(thread),
        }
    }

    fn stop(&mut self) -> Vec<Vec<u8>> {
        self.stop.store(true, Ordering::Release);
        let transcript = self
            .thread
            .take()
            .expect("running agent peer")
            .join()
            .expect("SSH peer thread");
        std::fs::remove_file(&self.socket).expect("close agent socket");
        transcript
    }
}
impl Drop for AgentPeer {
    fn drop(&mut self) {
        if self.thread.is_some() {
            self.stop();
        }
    }
}

#[test]
fn keystore_accepts_activity_preimage_is_closed_before_and_after_reopening() {
    assert!(!include_str!("../src/signer.rs").contains("pub fn sign_disclosed<'a>("));
    let directory = TestDirectory::new();
    let socket = directory.0.join("agent.sock");
    let seed = [0x61; 32];
    let public_key = SigningKey::from_bytes(&seed).verifying_key().to_bytes();
    let (_, options) = signer_and_options();
    let consent = bind_message(&options, 2);
    let retired_activity = support::archived_send(25);
    let payment = support::send_call(25, 7);
    let transaction = payment.call.encode_unsigned();
    let request = support::signing::activity_request(&transaction, &payment.disclosure);
    let activity_digest = layerx_crypto::SignatureMessage::new(
        layerx_wire::hash::Domain::SignaturePreimage,
        3,
        17,
        &retired_activity,
    )
    .expect("retired preimage fixture")
    .digest();
    let mut first_signature = None;
    for _ in 0..2 {
        let mut agent = AgentPeer::start(&socket, seed);
        let signer = KeystoreSigner::new(&socket, public_key).expect("external signer");
        assert_eq!(
            ready(signer.sign(request)),
            Err(SignError::InvalidDisclosure)
        );
        let mut altered = payment.disclosure.clone();
        altered.amounts[0].value += 1;
        assert_eq!(
            SigningRequest::new(
                layerx_crypto::SignatureMessage::new(
                    layerx_wire::hash::Domain::SignaturePreimage,
                    support::PROTOCOL_VERSION,
                    support::NETWORK_ID,
                    &transaction,
                )
                .expect("activity preimage domain"),
                &altered,
            )
            .err(),
            Some(SignError::DisclosureMismatch("amounts"))
        );
        for retired in [retired_activity.as_slice(), activity_digest.as_slice()] {
            assert_eq!(
                ready(signer.sign_bind_bytes(retired)),
                Err(SignError::InvalidDisclosure)
            );
            assert_eq!(
                debit().signing_request(retired).err(),
                Some(SignError::DisclosureMismatch("send_authorization"))
            );
        }
        for length in 0..BIND_MESSAGE_BYTES {
            assert_eq!(
                ready(signer.sign_bind_bytes(&consent.as_bytes()[..length])),
                Err(SignError::InvalidDisclosure)
            );
        }
        let mut trailing = consent.as_bytes().to_vec();
        trailing.push(0);
        assert_eq!(
            ready(signer.sign_bind_bytes(&trailing)),
            Err(SignError::InvalidDisclosure)
        );
        let signature =
            ready(signer.sign_bind_bytes(consent.as_bytes())).expect("SSH raw bind signing");
        consent
            .verify(&public_key, signature.as_bytes())
            .expect("real returned signature");
        if let Some(first) = first_signature {
            assert_eq!(
                signature, first,
                "reopened signer preserves exact signing domain"
            );
        } else {
            first_signature = Some(signature);
        }
        let debit = debit();
        let canonical = debit.authorization_message().expect("native payer message");
        let request = debit
            .signing_request(&canonical)
            .expect("typed payer request");
        let payer_bytes = request.signing_bytes().expect("permitted payer digest");
        let payload = ready(debit.sign(&signer)).expect("SSH payer authorization");
        payment_round_trip(&payload);
        let absent_key = LocalSigner::new([0x62; 32]).public_key();
        let wrong_signer =
            KeystoreSigner::new(&socket, absent_key).expect("well-formed absent key");
        assert_eq!(
            ready(wrong_signer.sign_bind_bytes(consent.as_bytes())),
            Err(SignError::KeystoreRefused)
        );
        drop(wrong_signer);
        drop(signer);
        assert_eq!(
            agent.stop(),
            vec![
                consent.as_bytes().to_vec(),
                payer_bytes,
                consent.as_bytes().to_vec()
            ],
            "no retired request reached the agent"
        );
    }
    let unavailable = KeystoreSigner::new(&socket, public_key).expect("closed external signer");
    assert_eq!(
        ready(unavailable.sign_bind_bytes(&retired_activity)),
        Err(SignError::InvalidDisclosure)
    );
    assert_eq!(
        ready(unavailable.sign_bind_bytes(consent.as_bytes())),
        Err(SignError::KeystoreUnavailable)
    );
}
