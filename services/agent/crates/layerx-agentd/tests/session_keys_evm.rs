use k256::ecdsa::SigningKey;
use layerx_agentd::session_keys::{SessionKeyRegistry, SessionKeyRegistryError};
use layerx_crypto::keystore::{Keystore, KeystoreEntropy};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::secp256k1::evm_address;
use layerx_crypto::session::{
    issue_session_key, IssuedSessionKey, SessionKeyRequest, SessionPurpose,
};
use layerx_crypto::signer::Signer as _;
use layerx_types::payload::{ActivityType, ModuleId};
use std::fmt::Debug;
use std::fs::{self, OpenOptions};
use std::io::Write as _;
use std::os::unix::fs::{MetadataExt as _, OpenOptionsExt as _, PermissionsExt as _};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT: AtomicU64 = AtomicU64::new(1);
const OPERATOR_SECRET: [u8; 32] = [0x61; 32];
const NETWORK: u32 = 77;

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("real session registry operation: {error:?}"))
}

struct RegistryDirectory(PathBuf);

impl RegistryDirectory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "layerx-session-evm-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::create_dir(&path));
        must(fs::set_permissions(
            &path,
            fs::Permissions::from_mode(0o700),
        ));
        Self(path)
    }

    fn open(&self) -> SessionKeyRegistry {
        must(SessionKeyRegistry::open(
            self.0.clone(),
            OPERATOR_SECRET.to_vec(),
            NETWORK,
            must(fs::metadata(&self.0)).uid(),
        ))
    }

    fn entry(&self, grant_id: [u8; 32]) -> PathBuf {
        self.0.join(format!("{}.lxks", hex::encode(grant_id)))
    }
}

impl Drop for RegistryDirectory {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.0));
    }
}

fn issue(session_public_key: [u8; 32]) -> IssuedSessionKey {
    must(issue_session_key(&SessionKeyRequest {
        grantor: [0x31; 32],
        session_public_key,
        not_before: 100,
        expires_at: Some(1000),
        permitted_activity_types: vec![must(ActivityType::new(ModuleId::Asset, 5))],
        revocation_sequence: Some(3),
        fee_budget: None,
        purpose: SessionPurpose::Activity,
    }))
}

fn address_word(address: [u8; 20]) -> [u8; 32] {
    let mut word = [0; 32];
    word[12..].copy_from_slice(&address);
    word
}

fn address(secret: &[u8; 32]) -> [u8; 20] {
    let key = must(SigningKey::from_slice(secret));
    must(evm_address(
        key.verifying_key().to_encoded_point(false).as_bytes(),
    ))
}

#[test]
fn provisioned_session_key_reports_its_evm_address() {
    let directory = RegistryDirectory::new();
    let registry = directory.open();
    let mut secret = [0; 32];
    secret[31] = 1;
    let expected = address(&secret);
    assert_eq!(
        hex::encode(expected),
        "7e5f4552091a69125d5dfcb7b8c2659029395bdf"
    );
    let issued = issue(address_word(expected));
    must(registry.provision(issued.grant_id, &secret, &issued));
    let encrypted = must(fs::read(directory.entry(issued.grant_id)));
    assert!(!encrypted.windows(32).any(|bytes| bytes == secret));
    let envelope = must(Keystore::from_bytes(&encrypted));
    assert!(envelope.identity().ends_with(&expected));
    assert_eq!(
        must(fs::metadata(directory.entry(issued.grant_id))).mode() & 0o777,
        0o600
    );
    assert!(matches!(
        registry.provision(issued.grant_id, &secret, &issued),
        Err(SessionKeyRegistryError::Exists)
    ));
    let loaded = must(registry.load(issued.grant_id, issued.clone()));
    assert_eq!(loaded.evm_address(), expected);
    assert_eq!(must(loaded.with_secret(address)), expected);
    let wrong_key = issue(address_word(address(&[2; 32])));
    assert!(matches!(
        registry.provision(wrong_key.grant_id, &secret, &wrong_key),
        Err(SessionKeyRegistryError::Invalid)
    ));
    assert!(registry
        .provision(issued.grant_id, &[0; 32], &issued)
        .is_err());
    assert!(registry
        .provision(issued.grant_id, &[0xff; 32], &issued)
        .is_err());
    let mut changed = issued.clone();
    changed.expires_at += 1;
    assert!(registry.load(issued.grant_id, changed).is_err());
    let prepared = must(registry.prepare_seed(b"owner/session/evm", [7; 32]));
    assert_eq!(prepared.evm_address(), address(prepared.secret()));
    let prepared_issued = issue(address_word(prepared.evm_address()));
    must(registry.provision(
        prepared_issued.grant_id,
        prepared.secret(),
        &prepared_issued,
    ));
    assert_eq!(
        must(registry.load(prepared_issued.grant_id, prepared_issued)).evm_address(),
        prepared.evm_address()
    );
    must(registry.probe());
    drop(registry);
    let restarted = directory.open();
    assert_eq!(
        must(restarted.load(issued.grant_id, issued.clone())).evm_address(),
        expected
    );
    assert_eq!(
        must(restarted.prepare_seed(b"owner/session/evm", [7; 32])).evm_address(),
        prepared.evm_address()
    );
    must(restarted.revoke(issued.grant_id));
    must(restarted.revoke(issued.grant_id));
    assert!(loaded.with_secret(address).is_err());
    assert!(restarted.load(issued.grant_id, issued.clone()).is_err());
    assert!(restarted
        .provision(issued.grant_id, &secret, &issued)
        .is_err());
    let marker = directory
        .0
        .join(format!("{}.revoked", hex::encode(issued.grant_id)));
    assert_eq!(must(fs::read(&marker)), b"LXSRV1");
    drop(restarted);
    assert!(directory.open().load(issued.grant_id, issued).is_err());
    assert_eq!(must(fs::read(marker)), b"LXSRV1");
}

#[test]
fn legacy_ed25519_session_entry_is_refused_at_load() {
    let directory = RegistryDirectory::new();
    let seed = [0x42; 32];
    let issued = issue(LocalSigner::new(seed).public_key());
    let mut identity = b"layerx-agentd/session-key/v1\0".to_vec();
    identity.extend(issued.grant_id);
    identity.extend(issued.session_public_key);
    identity.extend(issued.revocation_sequence.to_be_bytes());
    identity.extend(issued.expires_at.to_be_bytes());
    let envelope = must(Keystore::seal(
        &seed,
        &OPERATOR_SECRET,
        &identity,
        NETWORK,
        must(KeystoreEntropy::new([1; 16], [2; 24])),
    ));
    assert_eq!(
        must(
            envelope.open_with(&OPERATOR_SECRET, &identity, NETWORK, |stored| {
                LocalSigner::new(*stored).public_key()
            })
        ),
        issued.session_public_key
    );
    let bytes = must(envelope.to_bytes());
    let path = directory.entry(issued.grant_id);
    let mut file = must(
        OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&path),
    );
    must(file.write_all(&bytes));
    must(file.sync_all());
    drop(file);
    let registry = directory.open();
    assert!(matches!(
        registry.load(issued.grant_id, issued.clone()),
        Err(SessionKeyRegistryError::KeyKind)
    ));
    assert!(matches!(
        registry.probe(),
        Err(SessionKeyRegistryError::KeyKind)
    ));
    assert_eq!(must(fs::read(&path)), bytes);
    drop(registry);
    assert!(matches!(
        directory.open().load(issued.grant_id, issued),
        Err(SessionKeyRegistryError::KeyKind)
    ));
    assert_eq!(must(fs::read(path)), bytes);
}
