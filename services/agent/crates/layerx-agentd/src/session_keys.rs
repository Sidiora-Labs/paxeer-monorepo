//! Durable encrypted protocol-session signer registry.

use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use layerx_crypto::keystore::{Keystore, KeystoreEntropy};
use layerx_crypto::session::IssuedSessionKey;
use sha2::{Digest as _, Sha256};
use std::fs::{self, OpenOptions};
use std::io::Write;
use std::os::unix::fs::{MetadataExt, OpenOptionsExt, PermissionsExt};
use std::path::{Path, PathBuf};
use zeroize::Zeroizing;

pub struct SessionKeyRegistry {
    root: PathBuf,
    operator_secret: Zeroizing<Vec<u8>>,
    network_id: u32,
    owner_uid: u32,
}
#[derive(Debug)]
pub enum SessionKeyRegistryError {
    Io,
    Unprotected,
    Invalid,
    Exists,
    Crypto,
    KeyKind,
}

pub struct PreparedSessionSeed {
    secret: Zeroizing<[u8; 32]>,
    address: [u8; 20],
}

impl PreparedSessionSeed {
    pub fn secret(&self) -> &[u8; 32] {
        &self.secret
    }

    pub const fn evm_address(&self) -> [u8; 20] {
        self.address
    }
}

impl AsRef<[u8]> for PreparedSessionSeed {
    fn as_ref(&self) -> &[u8] {
        self.secret.as_ref()
    }
}

pub struct ProvisionedEvmSessionKey {
    key: PreparedSessionSeed,
    revocation_marker: PathBuf,
    owner_uid: u32,
}

impl ProvisionedEvmSessionKey {
    pub const fn evm_address(&self) -> [u8; 20] {
        self.key.evm_address()
    }

    pub fn with_secret<R>(
        &self,
        operation: impl FnOnce(&[u8; 32]) -> R,
    ) -> Result<R, SessionKeyRegistryError> {
        match fs::symlink_metadata(&self.revocation_marker) {
            Ok(_) => {
                validate_revocation_marker(&self.revocation_marker, self.owner_uid)?;
                Err(SessionKeyRegistryError::Invalid)
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                Ok(operation(self.key.secret()))
            }
            Err(_) => Err(SessionKeyRegistryError::Io),
        }
    }
}

const SESSION_KEY_KIND: &[u8] = b"layerx-agentd/session-key/secp256k1/v2\0";
const PREPARATION_KEY_KIND: &[u8] = b"layerx-agentd/session-preparation/secp256k1/v2\0";

impl SessionKeyRegistry {
    ///
    /// # Errors
    ///
    /// Returns an error if key material, protected files, or registry state cannot be validated or accessed.
    pub fn open(
        root: PathBuf,
        operator_secret: Vec<u8>,
        network_id: u32,
        owner_uid: u32,
    ) -> Result<Self, SessionKeyRegistryError> {
        if !root.is_absolute() || operator_secret.len() < 32 || network_id == 0 {
            return Err(SessionKeyRegistryError::Invalid);
        }
        fs::create_dir_all(&root).map_err(|_| SessionKeyRegistryError::Io)?;
        let meta = fs::symlink_metadata(&root).map_err(|_| SessionKeyRegistryError::Io)?;
        if !meta.file_type().is_dir()
            || meta.file_type().is_symlink()
            || meta.uid() != owner_uid
            || meta.permissions().mode() & 0o077 != 0
        {
            return Err(SessionKeyRegistryError::Unprotected);
        }
        for entry in fs::read_dir(&root).map_err(|_| SessionKeyRegistryError::Io)? {
            let path = entry.map_err(|_| SessionKeyRegistryError::Io)?.path();
            if path.extension().is_some_and(|value| value == "tmp") {
                validate_file(&path, owner_uid)?;
                fs::remove_file(path).map_err(|_| SessionKeyRegistryError::Io)?;
            }
        }
        fs::File::open(&root)
            .and_then(|file| file.sync_all())
            .map_err(|_| SessionKeyRegistryError::Io)?;
        Ok(Self {
            root,
            operator_secret: Zeroizing::new(operator_secret),
            network_id,
            owner_uid,
        })
    }
    /// Prepares one durable encrypted session seed for a bound issuance request.
    ///
    /// # Errors
    ///
    /// Refuses changed request bindings, invalid inputs or unprotected storage.
    pub fn prepare_seed(
        &self,
        namespace: &[u8],
        request_digest: [u8; 32],
    ) -> Result<PreparedSessionSeed, SessionKeyRegistryError> {
        if namespace.is_empty() || namespace.len() > 1024 || request_digest == [0; 32] {
            return Err(SessionKeyRegistryError::Invalid);
        }
        let mut hash = Sha256::new();
        hash.update(b"layerx-agentd/session-preparation/v1\0");
        hash.update(namespace);
        let id: [u8; 32] = hash.finalize().into();
        let mut binding = PREPARATION_KEY_KIND.to_vec();
        binding.extend(id);
        binding.extend(request_digest);
        let path = self.root.join(format!("pending-{}.lxks", hex(id)));
        if !path.exists() {
            let mut seed = Zeroizing::new([0; 32]);
            let mut salt = [0; 16];
            let mut nonce = [0; 24];
            getrandom::fill(seed.as_mut()).map_err(|_| SessionKeyRegistryError::Crypto)?;
            getrandom::fill(&mut salt).map_err(|_| SessionKeyRegistryError::Crypto)?;
            getrandom::fill(&mut nonce).map_err(|_| SessionKeyRegistryError::Crypto)?;
            session_address(&seed, self.network_id)?;
            let envelope = Keystore::seal(
                &seed,
                &self.operator_secret,
                &binding,
                self.network_id,
                KeystoreEntropy::new(salt, nonce).map_err(|_| SessionKeyRegistryError::Crypto)?,
            )
            .map_err(|_| SessionKeyRegistryError::Crypto)?;
            let bytes = envelope
                .to_bytes()
                .map_err(|_| SessionKeyRegistryError::Crypto)?;
            let temp = self.root.join(format!(
                "pending-{}.{}.tmp",
                hex(id),
                hex(Sha256::digest(nonce).into())
            ));
            let mut file = OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(&temp)
                .map_err(|_| SessionKeyRegistryError::Io)?;
            if file.write_all(&bytes).is_err() || file.sync_all().is_err() {
                let _ = fs::remove_file(&temp);
                return Err(SessionKeyRegistryError::Io);
            }
            drop(file);
            let published = fs::hard_link(&temp, &path);
            let _ = fs::remove_file(&temp);
            if let Err(error) = published {
                if error.kind() != std::io::ErrorKind::AlreadyExists {
                    return Err(SessionKeyRegistryError::Io);
                }
            }
            fs::File::open(&self.root)
                .and_then(|file| file.sync_all())
                .map_err(|_| SessionKeyRegistryError::Io)?;
        }
        let bytes = crate::config::read_protected_source(&path, 8192)
            .map_err(|_| SessionKeyRegistryError::Unprotected)?;
        let envelope = Keystore::from_bytes(&bytes).map_err(|_| SessionKeyRegistryError::Crypto)?;
        if !envelope.identity().starts_with(PREPARATION_KEY_KIND) {
            return Err(SessionKeyRegistryError::KeyKind);
        }
        envelope
            .open_with(&self.operator_secret, &binding, self.network_id, |seed| {
                Ok(PreparedSessionSeed {
                    address: session_address(seed, self.network_id)?,
                    secret: Zeroizing::new(*seed),
                })
            })
            .map_err(|_| SessionKeyRegistryError::Crypto)?
    }

    /// # Errors
    ///
    /// Returns an error if key material, protected files, or registry state cannot be validated or accessed.
    pub fn provision(
        &self,
        grant_id: [u8; 32],
        seed: &[u8; 32],
        issued: &IssuedSessionKey,
    ) -> Result<(), SessionKeyRegistryError> {
        if self.revoked_path(grant_id).exists() {
            return Err(SessionKeyRegistryError::Invalid);
        }
        validate_issued(grant_id, issued)?;
        let address = session_address(seed, self.network_id)?;
        validate_address(address, issued)?;
        let identity = identity(grant_id, issued, address);
        let mut salt = [0; 16];
        let mut nonce = [0; 24];
        getrandom::fill(&mut salt).map_err(|_| SessionKeyRegistryError::Crypto)?;
        getrandom::fill(&mut nonce).map_err(|_| SessionKeyRegistryError::Crypto)?;
        let envelope = Keystore::seal(
            seed,
            &self.operator_secret,
            &identity,
            self.network_id,
            KeystoreEntropy::new(salt, nonce).map_err(|_| SessionKeyRegistryError::Crypto)?,
        )
        .map_err(|_| SessionKeyRegistryError::Crypto)?;
        let bytes = envelope
            .to_bytes()
            .map_err(|_| SessionKeyRegistryError::Crypto)?;
        let path = self.path(grant_id);
        let temp = self
            .root
            .join(format!("{}.{}.tmp", hex(grant_id), std::process::id()));
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&temp)
            .map_err(|error| {
                if error.kind() == std::io::ErrorKind::AlreadyExists {
                    SessionKeyRegistryError::Exists
                } else {
                    SessionKeyRegistryError::Io
                }
            })?;
        if file.write_all(&bytes).is_err() || file.sync_all().is_err() {
            let _ = fs::remove_file(&temp);
            return Err(SessionKeyRegistryError::Io);
        }
        drop(file);
        let published = fs::hard_link(&temp, &path);
        let _ = fs::remove_file(&temp);
        match published {
            Ok(()) => {}
            Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => {
                return Err(SessionKeyRegistryError::Exists)
            }
            Err(_) => return Err(SessionKeyRegistryError::Io),
        }
        fs::File::open(&self.root)
            .and_then(|file| file.sync_all())
            .map_err(|_| SessionKeyRegistryError::Io)?;
        Ok(())
    }
    ///
    /// # Errors
    ///
    /// Returns an error if key material, protected files, or registry state cannot be validated or accessed.
    pub fn revoke(&self, grant_id: [u8; 32]) -> Result<(), SessionKeyRegistryError> {
        let marker = self.revoked_path(grant_id);
        if marker.exists() {
            validate_revocation_marker(&marker, self.owner_uid)?;
            return Ok(());
        }
        let temp = self.root.join(format!(
            "{}.revoked.{}.tmp",
            hex(grant_id),
            std::process::id()
        ));
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&temp)
            .map_err(|error| {
                if error.kind() == std::io::ErrorKind::AlreadyExists {
                    SessionKeyRegistryError::Exists
                } else {
                    SessionKeyRegistryError::Io
                }
            })?;
        if file.write_all(b"LXSRV1").is_err() || file.sync_all().is_err() {
            let _ = fs::remove_file(&temp);
            return Err(SessionKeyRegistryError::Io);
        }
        drop(file);
        let published = fs::hard_link(&temp, &marker);
        let _ = fs::remove_file(&temp);
        match published {
            Ok(()) => {}
            Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => {
                validate_revocation_marker(&marker, self.owner_uid)?;
            }
            Err(_) => return Err(SessionKeyRegistryError::Io),
        }
        fs::File::open(&self.root)
            .and_then(|file| file.sync_all())
            .map_err(|_| SessionKeyRegistryError::Io)?;
        Ok(())
    }
    ///
    /// # Errors
    ///
    /// Returns an error if key material, protected files, or registry state cannot be validated or accessed.
    pub fn load(
        &self,
        grant_id: [u8; 32],
        issued: IssuedSessionKey,
    ) -> Result<ProvisionedEvmSessionKey, SessionKeyRegistryError> {
        let marker = self.revoked_path(grant_id);
        if marker.exists() {
            return Err(SessionKeyRegistryError::Invalid);
        }
        let bytes = crate::config::read_protected_source(&self.path(grant_id), 8192)
            .map_err(|_| SessionKeyRegistryError::Unprotected)?;
        let envelope = Keystore::from_bytes(&bytes).map_err(|_| SessionKeyRegistryError::Crypto)?;
        if !envelope.identity().starts_with(SESSION_KEY_KIND) {
            return Err(SessionKeyRegistryError::KeyKind);
        }
        validate_issued(grant_id, &issued)?;
        let address: [u8; 20] = issued.session_public_key[12..]
            .try_into()
            .map_err(|_| SessionKeyRegistryError::Invalid)?;
        validate_address(address, &issued)?;
        let identity = identity(grant_id, &issued, address);
        envelope
            .open_with(&self.operator_secret, &identity, self.network_id, |seed| {
                if session_address(seed, self.network_id)? != address {
                    return Err(SessionKeyRegistryError::Invalid);
                }
                Ok(ProvisionedEvmSessionKey {
                    key: PreparedSessionSeed {
                        secret: Zeroizing::new(*seed),
                        address,
                    },
                    revocation_marker: marker,
                    owner_uid: self.owner_uid,
                })
            })
            .map_err(|_| SessionKeyRegistryError::Crypto)?
    }
    ///
    /// # Errors
    ///
    /// Returns an error if key material, protected files, or registry state cannot be validated or accessed.
    pub fn probe(&self) -> Result<(), SessionKeyRegistryError> {
        let meta = fs::symlink_metadata(&self.root).map_err(|_| SessionKeyRegistryError::Io)?;
        if !meta.file_type().is_dir()
            || meta.file_type().is_symlink()
            || meta.uid() != self.owner_uid
            || meta.permissions().mode() & 0o077 != 0
        {
            return Err(SessionKeyRegistryError::Unprotected);
        }
        for entry in fs::read_dir(&self.root).map_err(|_| SessionKeyRegistryError::Io)? {
            let path = entry.map_err(|_| SessionKeyRegistryError::Io)?.path();
            validate_file(&path, self.owner_uid)?;
            let bytes = crate::config::read_protected_source(&path, 8192)
                .map_err(|_| SessionKeyRegistryError::Unprotected)?;
            if path.extension().is_some_and(|value| value == "revoked") {
                if bytes != b"LXSRV1" {
                    return Err(SessionKeyRegistryError::Invalid);
                }
                continue;
            }
            let envelope =
                Keystore::from_bytes(&bytes).map_err(|_| SessionKeyRegistryError::Crypto)?;
            if !envelope.identity().starts_with(SESSION_KEY_KIND)
                && !envelope.identity().starts_with(PREPARATION_KEY_KIND)
            {
                return Err(SessionKeyRegistryError::KeyKind);
            }
            envelope
                .open_with(
                    &self.operator_secret,
                    envelope.identity(),
                    self.network_id,
                    |_| (),
                )
                .map_err(|_| SessionKeyRegistryError::Crypto)?;
        }
        Ok(())
    }
    fn path(&self, id: [u8; 32]) -> PathBuf {
        self.root.join(format!("{}.lxks", hex(id)))
    }
    fn revoked_path(&self, id: [u8; 32]) -> PathBuf {
        self.root.join(format!("{}.revoked", hex(id)))
    }
}
fn validate_file(path: &Path, owner_uid: u32) -> Result<(), SessionKeyRegistryError> {
    let meta = fs::symlink_metadata(path).map_err(|_| SessionKeyRegistryError::Io)?;
    if !meta.file_type().is_file()
        || meta.file_type().is_symlink()
        || meta.uid() != owner_uid
        || meta.permissions().mode() & 0o077 != 0
    {
        Err(SessionKeyRegistryError::Unprotected)
    } else {
        Ok(())
    }
}
fn validate_revocation_marker(path: &Path, owner_uid: u32) -> Result<(), SessionKeyRegistryError> {
    validate_file(path, owner_uid)?;
    let bytes = crate::config::read_protected_source(path, 16)
        .map_err(|_| SessionKeyRegistryError::Unprotected)?;
    if bytes == b"LXSRV1" {
        Ok(())
    } else {
        Err(SessionKeyRegistryError::Invalid)
    }
}
fn identity(grant: [u8; 32], issued: &IssuedSessionKey, address: [u8; 20]) -> Vec<u8> {
    let mut out = SESSION_KEY_KIND.to_vec();
    out.extend(grant);
    out.extend(issued.session_public_key);
    out.extend(issued.revocation_sequence.to_be_bytes());
    out.extend(issued.expires_at.to_be_bytes());
    out.extend(address);
    out
}
fn hex(bytes: [u8; 32]) -> String {
    const H: &[u8; 16] = b"0123456789abcdef";
    bytes
        .iter()
        .flat_map(|b| [H[(b >> 4) as usize] as char, H[(b & 15) as usize] as char])
        .collect()
}

fn validate_issued(
    grant_id: [u8; 32],
    issued: &IssuedSessionKey,
) -> Result<(), SessionKeyRegistryError> {
    if grant_id != issued.grant_id
        || issued.purpose != layerx_crypto::session::SessionPurpose::Activity
        || issued.permitted_activity_types.is_empty()
        || issued.expires_at == 0
        || issued.revocation_sequence == 0
    {
        return Err(SessionKeyRegistryError::Invalid);
    }
    Ok(())
}

fn validate_address(
    address: [u8; 20],
    issued: &IssuedSessionKey,
) -> Result<(), SessionKeyRegistryError> {
    if issued.session_public_key[..12] != [0; 12] || issued.session_public_key[12..] != address {
        return Err(SessionKeyRegistryError::Invalid);
    }
    Ok(())
}

fn session_address(
    secret: &[u8; 32],
    network_id: u32,
) -> Result<[u8; 20], SessionKeyRegistryError> {
    let proof = Eip1559Call {
        chain_id: u64::from(network_id),
        nonce: 0,
        max_priority_fee_per_gas: 0,
        max_fee_per_gas: 0,
        gas_limit: 0,
        to: [0; 20],
        data: SESSION_KEY_KIND.to_vec(),
    };
    let signed = proof
        .sign(secret)
        .map_err(|_| SessionKeyRegistryError::Crypto)?;
    let (call, signature) = decode_signed(&signed).map_err(|_| SessionKeyRegistryError::Crypto)?;
    recover_sender(&call, &signature).map_err(|_| SessionKeyRegistryError::Crypto)
}
