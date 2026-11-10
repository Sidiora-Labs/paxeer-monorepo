use std::future::Future;
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

use k256::ecdsa::SigningKey;
use layerx_agentd::sign::SigningPreparation;
use layerx_agentd::sign::{self_sign, ProvisionedSessionKey, SigningError, SigningMode};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
use layerx_crypto::secp256k1::evm_address;
use layerx_crypto::session::{issue_session_key, IssuedSessionKey, SessionKeyRequest};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::encode::Encoder;

struct NoopWake;

impl Wake for NoopWake {
    fn wake(self: Arc<Self>) {}
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(NoopWake));
    let mut context = Context::from_waker(&waker);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("local signing unexpectedly blocked"),
    }
}

fn send_type() -> ActivityType {
    ActivityType::new(ModuleId::Asset, 5).unwrap_or_else(|error| panic!("activity: {error:?}"))
}

fn registry() -> ModuleRegistry {
    ModuleRegistry::new(&[ModuleRegistration::new(ModuleId::Asset, &[send_type()])
        .unwrap_or_else(|error| panic!("registration: {error:?}"))])
    .unwrap_or_else(|error| panic!("registry: {error:?}"))
}

fn send_payload() -> Vec<u8> {
    let mut encoder = Encoder::new(512);
    for result in [encoder.u16(0x5301), encoder.u16(10)] {
        result.unwrap_or_else(|error| panic!("header: {error:?}"));
    }
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("from: {error:?}"));
    encoder
        .fixed(&[0x22; 32])
        .unwrap_or_else(|error| panic!("to: {error:?}"));
    encoder
        .fixed(&[0x33; 32])
        .unwrap_or_else(|error| panic!("asset: {error:?}"));
    encoder
        .u128(25)
        .unwrap_or_else(|error| panic!("amount: {error:?}"));
    encoder
        .u64(5)
        .unwrap_or_else(|error| panic!("sequence: {error:?}"));
    encoder
        .fixed(&[4; 32])
        .unwrap_or_else(|error| panic!("idempotency: {error:?}"));
    encoder
        .u64(1_010)
        .unwrap_or_else(|error| panic!("expiry: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("context: {error:?}"));
    encoder
        .u8(0)
        .unwrap_or_else(|error| panic!("conditions: {error:?}"));
    encoder
        .u8(1)
        .unwrap_or_else(|error| panic!("authority kind: {error:?}"));
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("controller: {error:?}"));
    encoder
        .fixed(&[0x66; 32])
        .unwrap_or_else(|error| panic!("payload key: {error:?}"));
    encoder
        .fixed(&[0x77; 64])
        .unwrap_or_else(|error| panic!("payload signature: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("signed context: {error:?}"));
    encoder
        .u32(17)
        .unwrap_or_else(|error| panic!("network: {error:?}"));
    encoder
        .u16(layerx_wire::limits::PROTOCOL_VERSION)
        .unwrap_or_else(|error| panic!("version: {error:?}"));
    crate::send_authorization::sign(encoder.finish())
}

fn issued(seed: [u8; 32], permitted: ActivityType, expires_at: u64) -> IssuedSessionKey {
    let public_key = session_public_key(seed);
    issue_session_key(&SessionKeyRequest {
        fee_budget: None,
        purpose: layerx_crypto::session::SessionPurpose::Activity,
        grantor: [1; 32],
        session_public_key: public_key,
        not_before: 900,
        expires_at: Some(expires_at),
        permitted_activity_types: vec![permitted],
        revocation_sequence: Some(5),
    })
    .unwrap_or_else(|error| panic!("issue session key: {error:?}"))
}

fn session_public_key(seed: [u8; 32]) -> [u8; 32] {
    let mut public_key = [0; 32];
    public_key[12..].copy_from_slice(&session_address(seed));
    public_key
}

fn session_address(seed: [u8; 32]) -> [u8; 20] {
    let key = SigningKey::from_slice(&seed).unwrap_or_else(|error| panic!("key: {error:?}"));
    evm_address(key.verifying_key().to_encoded_point(false).as_bytes())
        .unwrap_or_else(|error| panic!("address: {error:?}"))
}

struct SessionFixture {
    root: std::path::PathBuf,
    keys: layerx_agentd::session_keys::SessionKeyRegistry,
}
impl SessionFixture {
    fn new() -> Self {
        use std::os::unix::fs::{DirBuilderExt, MetadataExt};
        static NEXT: std::sync::atomic::AtomicU64 = std::sync::atomic::AtomicU64::new(1);
        let root = std::env::temp_dir().join(format!(
            "evm-signing-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, std::sync::atomic::Ordering::Relaxed)
        ));
        assert_ok(
            std::fs::DirBuilder::new().mode(0o700).create(&root),
            "unexpected error",
        );
        let uid = assert_ok(std::fs::metadata(&root), "unexpected error").uid();
        let keys = assert_ok(
            layerx_agentd::session_keys::SessionKeyRegistry::open(
                root.clone(),
                vec![0x81; 32],
                17,
                uid,
            ),
            "unexpected error",
        );
        Self { root, keys }
    }
    fn provision(&self, seed: [u8; 32], grant: &IssuedSessionKey) -> ProvisionedSessionKey {
        // Decode the actual authorization registration before persisting its EVM key.
        assert_eq!(
            assert_ok(
                layerx_crypto::session::decode_session_key(&grant.registration_payload),
                "unexpected error"
            ),
            *grant
        );
        assert_ok(
            self.keys.provision(grant.grant_id, &seed, grant),
            "unexpected error",
        );
        assert_ok(
            self.keys.load(grant.grant_id, grant.clone()),
            "unexpected error",
        )
    }
    fn reopen(&mut self) {
        use std::os::unix::fs::MetadataExt;
        self.keys = assert_ok(
            layerx_agentd::session_keys::SessionKeyRegistry::open(
                self.root.clone(),
                vec![0x81; 32],
                17,
                assert_ok(std::fs::metadata(&self.root), "unexpected error").uid(),
            ),
            "unexpected error",
        );
    }
}
impl Drop for SessionFixture {
    fn drop(&mut self) {
        assert_ok(std::fs::remove_dir_all(&self.root), "unexpected error");
    }
}

fn prepared() -> SigningPreparation {
    let actor_address = session_address([0xa5; 32]);
    let transaction = assert_ok(
        layerx_agentd::prepare::prepare_bound(
            layerx_agentd::prepare::EvmPreparationState {
                chain_id: 17,
                pending_nonce: 5,
                base_fee_per_gas: 3,
                head_height: 88,
            },
            layerx_agentd::prepare::EvmPreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 7,
                maximum_payload_bytes: 1_024,
            },
            layerx_agentd::prepare::EvmPrepareRequest {
                from: actor_address,
                call: layerx_agentd::prepare::PrecompileCall::ledger_transfer(send_payload()),
                expected_nonce: Some(5),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 1_024,
            },
            100_000,
        ),
        "unexpected error",
    );
    SigningPreparation {
        transaction,
        actor_address,
    }
}

#[test]
fn self_signed_transaction_recovers_to_the_session_address() {
    let seed = [0xa5; 32];
    let session_issued = issued(seed, send_type(), 1_100);
    let prepared = prepared();
    let mut fixture = SessionFixture::new();
    let session = fixture.provision(seed, &session_issued);
    assert_eq!(session.evm_address(), session_address(seed));
    drop(session);
    fixture.reopen();
    let session = assert_ok(
        fixture
            .keys
            .load(session_issued.grant_id, session_issued.clone()),
        "unexpected error",
    );
    let signed = assert_ok(
        ready(self_sign(Some(&session), &prepared, &registry(), 1_000, 5)),
        "unexpected error",
    );
    assert_eq!(signed.signature.len(), 65);
    let (call, signature) = assert_ok(
        decode_signed(&signed.signed_canonical_bytes),
        "unexpected error",
    );
    assert_eq!(call, prepared.transaction.call);
    assert_eq!(signature.as_bytes(), &signed.signature);
    let expected = session_address(seed);
    assert_eq!(
        assert_ok(recover_sender(&call, &signature), "unexpected error"),
        expected
    );
    assert_eq!(signed.signer_address, expected);
    assert_eq!(expected, prepared.actor_address);
    assert_eq!(signed.mode, SigningMode::ProtocolSessionKey);
    let verified = assert_ok(
        layerx_agentd::sign::verify_before_submit(
            &signed.signed_canonical_bytes,
            &prepared,
            &expected,
            &registry(),
        ),
        "unexpected error",
    );
    assert_eq!(verified.exact_bytes(), signed.signed_canonical_bytes);
    assert_eq!(signed.audit.session_public_key, session_public_key(seed));
    assert_eq!(signed.audit.revocation_sequence, 5);
    assert_eq!(signed.audit.expires_at, 1_100);
    assert_eq!(
        ready(self_sign(Some(&session), &prepared, &registry(), 1_000, 6)),
        Err(SigningError::Revoked),
    );
    let mut changed_hash = prepared.clone();
    changed_hash.transaction.signing_hash[0] ^= 1;
    assert_eq!(
        ready(self_sign(
            Some(&session),
            &changed_hash,
            &registry(),
            1_000,
            5
        )),
        Err(SigningError::PreparedBytesChanged),
    );
    let mint = assert_authority_and_scope(seed, &prepared);
    assert_eq!(session_issued.permitted_activity_types, vec![send_type()]);
    let mut changed_grant = session_issued.clone();
    changed_grant.permitted_activity_types = vec![mint];
    assert!(fixture
        .keys
        .load(changed_grant.grant_id, changed_grant)
        .is_err());
    assert_ok(
        fixture.keys.revoke(session_issued.grant_id),
        "unexpected error",
    );
    assert_eq!(
        ready(self_sign(Some(&session), &prepared, &registry(), 1_000, 5)),
        Err(SigningError::Revoked)
    );
    drop(session);
    fixture.reopen();
    assert!(fixture
        .keys
        .load(session_issued.grant_id, session_issued)
        .is_err());
}

#[test]
fn external_signature_from_another_key_is_refused() {
    let prepared = prepared();
    let foreign_raw = assert_ok(
        prepared.transaction.call.sign(&[0xa6; 32]),
        "unexpected error",
    );
    let (call, signature) = assert_ok(decode_signed(&foreign_raw), "unexpected error");
    assert_eq!(signature.as_bytes().len(), 65);
    assert_eq!(
        assert_ok(recover_sender(&call, &signature), "unexpected error"),
        session_address([0xa6; 32])
    );
    assert_ne!(session_address([0xa6; 32]), prepared.actor_address);
    assert_eq!(
        layerx_agentd::sign::attach_external_signature(&prepared, *signature.as_bytes()),
        Err(SigningError::SignerMismatch),
    );
    assert_eq!(
        layerx_agentd::sign::attach_external_signature(&prepared, [0; 65]),
        Err(SigningError::SignatureInvalid),
    );
    let valid_raw = assert_ok(
        prepared.transaction.call.sign(&[0xa5; 32]),
        "unexpected error",
    );
    let (_, valid_signature) = assert_ok(decode_signed(&valid_raw), "unexpected error");
    assert_eq!(
        layerx_agentd::sign::verify_before_submit(
            &valid_raw,
            &prepared,
            &session_address([0xa6; 32]),
            &registry(),
        ),
        Err(SigningError::SignerMismatch),
    );
    let mut trailing_bytes = valid_raw.clone();
    trailing_bytes.push(0);
    assert_eq!(
        layerx_agentd::sign::verify_before_submit(
            &trailing_bytes,
            &prepared,
            &prepared.actor_address,
            &registry(),
        ),
        Err(SigningError::SignatureInvalid),
    );

    assert_eq!(
        assert_ok(
            layerx_agentd::sign::attach_external_signature(&prepared, *valid_signature.as_bytes()),
            "unexpected error"
        ),
        valid_raw,
    );
    assert_eq!(
        layerx_agentd::sign::verify_before_submit(
            &foreign_raw,
            &prepared,
            &prepared.actor_address,
            &registry(),
        ),
        Err(SigningError::SignerMismatch),
    );
}

#[test]
fn verify_before_submit_rejects_mutated_calldata() {
    let prepared = prepared();
    let mut changed = prepared.transaction.call.clone();
    changed.data[0] ^= 1;
    // Sign the changed call with the correct key, so rejection proves call binding.
    let mutated_raw = assert_ok(changed.sign(&[0xa5; 32]), "unexpected error");
    let (decoded, signature) = assert_ok(decode_signed(&mutated_raw), "unexpected error");
    assert_ne!(decoded.data, prepared.transaction.call.data);
    assert_eq!(
        assert_ok(recover_sender(&decoded, &signature), "unexpected error"),
        prepared.actor_address
    );
    assert_eq!(
        layerx_agentd::sign::verify_before_submit(
            &mutated_raw,
            &prepared,
            &prepared.actor_address,
            &registry(),
        ),
        Err(SigningError::PreparedBytesChanged),
    );
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}

fn assert_authority_and_scope(seed: [u8; 32], prepared: &SigningPreparation) -> ActivityType {
    let other_fixture = SessionFixture::new();
    let mismatched_authority =
        other_fixture.provision([0xa6; 32], &issued([0xa6; 32], send_type(), 1_100));
    assert_eq!(
        ready(self_sign(
            Some(&mismatched_authority),
            prepared,
            &registry(),
            1_000,
            5
        )),
        Err(SigningError::AuthorityMismatch),
    );
    let denied_fixture = SessionFixture::new();
    let mint = assert_ok(ActivityType::new(ModuleId::Asset, 10), "unexpected error");
    let denied = denied_fixture.provision(seed, &issued(seed, mint, 1_100));
    assert_eq!(
        ready(self_sign(Some(&denied), prepared, &registry(), 1_000, 5)),
        Err(SigningError::ScopeDenied)
    );
    mint
}
