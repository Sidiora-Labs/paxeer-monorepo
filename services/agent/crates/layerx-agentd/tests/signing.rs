#[path = "support/send_authorization.rs"]
mod send_authorization;

use std::fs;
use std::future::Future;
use std::os::unix::fs::{MetadataExt, PermissionsExt};
use std::path::PathBuf;
use std::pin::pin;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

use k256::ecdsa::SigningKey;
use layerx_agentd::session_keys::SessionKeyRegistry;
use layerx_agentd::sign::{external, self_sign, ProvisionedSessionKey, SigningError, SigningMode};
use layerx_agentd::sign::{validate_issued_session, SigningPreparation};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::secp256k1::evm_address;
use layerx_crypto::session::{issue_session_key, IssuedSessionKey, SessionKeyRequest};
use layerx_crypto::signer::Signer;
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

fn other_type() -> ActivityType {
    ActivityType::new(ModuleId::Asset, 6).unwrap_or_else(|error| panic!("activity: {error:?}"))
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
    send_authorization::sign(encoder.finish())
}

fn issued(seed: [u8; 32], permitted: ActivityType, expires_at: u64) -> IssuedSessionKey {
    let mut public_key = [0; 32];
    public_key[12..].copy_from_slice(&session_address(seed));
    let grant = issue_session_key(&SessionKeyRequest {
        fee_budget: None,
        purpose: layerx_crypto::session::SessionPurpose::Activity,
        grantor: [1; 32],
        session_public_key: public_key,
        not_before: 900,
        expires_at: Some(expires_at),
        permitted_activity_types: vec![permitted],
        revocation_sequence: Some(5),
    })
    .unwrap_or_else(|error| panic!("issue session key: {error:?}"));
    let decoded = validate_issued_session(
        &grant.registration_payload,
        [1; 32],
        public_key,
        900,
        expires_at,
        5,
        &[permitted.ordinal()],
    )
    .unwrap_or_else(|error| panic!("validate auth grant: {error:?}"));
    assert_eq!(decoded, grant);
    assert_eq!(decoded.permitted_activity_types, vec![permitted]);
    decoded
}

static NEXT_REGISTRY: AtomicU64 = AtomicU64::new(1);

struct RegistryDirectory(PathBuf);

impl RegistryDirectory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "layerx-signing-registry-{}-{}",
            std::process::id(),
            NEXT_REGISTRY.fetch_add(1, Ordering::Relaxed),
        ));
        fs::create_dir(&path).unwrap();
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700)).unwrap();
        Self(path)
    }

    fn open(&self) -> SessionKeyRegistry {
        SessionKeyRegistry::open(
            self.0.clone(),
            vec![0x61; 32],
            17,
            fs::metadata(&self.0).unwrap().uid(),
        )
        .unwrap_or_else(|error| panic!("registry: {error:?}"))
    }

    fn provision(&self, seed: [u8; 32], issued: &IssuedSessionKey) -> ProvisionedSessionKey {
        let registry = self.open();
        registry.provision(issued.grant_id, &seed, issued).unwrap();
        drop(registry);
        self.open().load(issued.grant_id, issued.clone()).unwrap()
    }
}

impl Drop for RegistryDirectory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

fn session_address(seed: [u8; 32]) -> [u8; 20] {
    let key = SigningKey::from_slice(&seed).unwrap_or_else(|error| panic!("key: {error:?}"));
    evm_address(key.verifying_key().to_encoded_point(false).as_bytes())
        .unwrap_or_else(|error| panic!("address: {error:?}"))
}

fn prepared() -> SigningPreparation {
    let actor_address = session_address([0xa5; 32]);
    let transaction = layerx_agentd::prepare::prepare_bound(
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
    )
    .unwrap();
    SigningPreparation {
        transaction,
        actor_address,
    }
}

#[test]
fn external_signing_is_the_key_free_default() {
    let prepared = prepared();
    let package = external(&prepared).unwrap_or_else(|error| panic!("external: {error:?}"));
    assert_eq!(package.mode, SigningMode::External);
    assert_eq!(
        package.canonical_bytes,
        prepared.transaction.call.encode_unsigned()
    );
    assert_eq!(package.signing_preimage, prepared.transaction.signing_hash);
    assert_eq!(
        package.disclosure_digest.0,
        prepared.transaction.disclosure_digest
    );
}

#[test]
fn unprovisioned_scope_exceeding_and_expired_self_signing_are_refused() {
    let seed = [0xa5; 32];
    let valid_prepared = prepared();
    assert_eq!(
        ready(self_sign(None, &valid_prepared, &registry(), 1_000, 5)),
        Err(SigningError::NotProvisioned)
    );

    let other_issued = issued(seed, other_type(), 1_100);
    let other_prepared = prepared();
    let other_directory = RegistryDirectory::new();
    let other = other_directory.provision(seed, &other_issued);
    assert_eq!(
        ready(self_sign(
            Some(&other),
            &other_prepared,
            &registry(),
            1_000,
            5
        )),
        Err(SigningError::ScopeDenied)
    );

    let expired_issued = issued(seed, send_type(), 999);
    let expired_prepared = prepared();
    let expired_directory = RegistryDirectory::new();
    let expired = expired_directory.provision(seed, &expired_issued);
    assert_eq!(
        ready(self_sign(
            Some(&expired),
            &expired_prepared,
            &registry(),
            1_000,
            5
        )),
        Err(SigningError::Expired)
    );
}

#[test]
fn session_key_self_signing_is_distinct_and_audited() {
    let seed = [0xa5; 32];
    let issued = issued(seed, send_type(), 1_100);
    let prepared = prepared();
    let directory = RegistryDirectory::new();
    let provisioned = directory.provision(seed, &issued);
    assert_eq!(provisioned.evm_address(), session_address(seed));
    let signed = ready(self_sign(
        Some(&provisioned),
        &prepared,
        &registry(),
        1_000,
        5,
    ))
    .unwrap_or_else(|error| panic!("self sign: {error:?}"));
    assert_eq!(signed.mode, SigningMode::ProtocolSessionKey);
    assert_eq!(signed.audit.mode, SigningMode::ProtocolSessionKey);
    assert_eq!(signed.audit.activity_type, send_type());
    assert_eq!(signed.audit.session_public_key, issued.session_public_key);
    assert_eq!(signed.audit.authority_bytes, issued.authority.as_bytes());
    assert_eq!(signed.audit.expires_at, issued.expires_at);
    assert_eq!(signed.audit.revocation_sequence, issued.revocation_sequence);
    assert_eq!(issued.permitted_activity_types, vec![send_type()]);
    assert_eq!(
        signed.audit.disclosure_digest.0,
        prepared.transaction.disclosure_digest
    );
    assert_eq!(
        signed.canonical_bytes,
        prepared.transaction.call.encode_unsigned()
    );
    assert_eq!(signed.signature.len(), 65);
    assert_ne!(signed.signature, [0; 65]);
    let (call, signature) = decode_signed(&signed.signed_canonical_bytes)
        .unwrap_or_else(|error| panic!("decode: {error:?}"));
    assert_eq!(call, prepared.transaction.call);
    assert_eq!(signature.as_bytes(), &signed.signature);
    assert_eq!(
        recover_sender(&call, &signature).unwrap(),
        session_address(seed)
    );
    assert_eq!(signed.signer_address, session_address(seed));
}

#[test]
fn authentication_only_sessions_cannot_be_imported_as_native_signers() {
    let seed = [0xa6; 32];
    let mut authentication = issue_session_key(&SessionKeyRequest {
        grantor: [1; 32],
        session_public_key: LocalSigner::new(seed).public_key(),
        not_before: 900,
        expires_at: Some(1_100),
        permitted_activity_types: Vec::new(),
        revocation_sequence: Some(5),
        fee_budget: None,
        purpose: layerx_crypto::session::SessionPurpose::Authentication,
    })
    .unwrap_or_else(|error| panic!("issue authentication key: {error:?}"));
    assert!(matches!(
        ProvisionedSessionKey::new(seed, authentication.clone()),
        Err(SigningError::InvalidProvisioning)
    ));
    authentication.permitted_activity_types.push(send_type());
    assert!(matches!(
        ProvisionedSessionKey::new(seed, authentication),
        Err(SigningError::InvalidProvisioning)
    ));
}

#[test]
fn durable_revocation_refuses_loaded_signers_and_survives_reopen() {
    let seed = [0xa5; 32];
    let issued = issued(seed, send_type(), 1_100);
    let directory = RegistryDirectory::new();
    let loaded = directory.provision(seed, &issued);
    let prepared = prepared();
    assert_eq!(
        ready(self_sign(Some(&loaded), &prepared, &registry(), 1_000, 6)),
        Err(SigningError::Revoked)
    );
    assert!(ready(self_sign(Some(&loaded), &prepared, &registry(), 1_000, 5)).is_ok());
    let registry_store = directory.open();
    registry_store.revoke(issued.grant_id).unwrap();
    assert_eq!(
        ready(self_sign(Some(&loaded), &prepared, &registry(), 1_000, 5)),
        Err(SigningError::Revoked)
    );
    drop(registry_store);
    let reopened = directory.open();
    assert!(reopened.load(issued.grant_id, issued.clone()).is_err());
    assert!(reopened.provision(issued.grant_id, &seed, &issued).is_err());
    assert_eq!(
        ready(self_sign(Some(&loaded), &prepared, &registry(), 1_000, 5)),
        Err(SigningError::Revoked)
    );
    assert_eq!(
        fs::read(
            directory
                .0
                .join(format!("{}.revoked", hex::encode(issued.grant_id)))
        )
        .unwrap(),
        b"LXSRV1"
    );
}
