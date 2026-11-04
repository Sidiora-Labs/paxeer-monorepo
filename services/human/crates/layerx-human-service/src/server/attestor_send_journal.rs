use std::collections::BTreeMap;
use std::fs::{self, File, OpenOptions};
use std::io::{Read, Write};
use std::os::unix::fs::{MetadataExt as _, OpenOptionsExt as _, PermissionsExt as _};
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};
use subtle::ConstantTimeEq as _;
use zeroize::Zeroizing;

use super::production_components::AttestorSendAuthorization;
use crate::custody::KmsError;

pub const ATTESTOR_SEND_RETENTION_CAPACITY: usize = 4_096;
const MAX_BYTES: u64 = 64 * 1024 * 1024;
const FILE_NAME: &str = "attestor-sends.v1";
const TEMP_NAME: &str = "attestor-sends.tmp";

#[derive(Clone, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct Entry {
    request: AttestorSendAuthorization,
    raw_transaction: Option<Vec<u8>>,
}
#[derive(Clone, Default, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct State {
    observed_at: u64,
    sends: BTreeMap<String, Entry>,
    nonce_floors: BTreeMap<String, u64>,
}

/// Durable signing reservations, exact-request results and per-wallet nonce floors.
/// A reservation is committed before the quorum is contacted. An expired reservation
/// may be reconciled only with its durable floor; its authorization can never sign again.
pub struct AttestorSendJournal {
    root: PathBuf,
    key: Zeroizing<[u8; 32]>,
    state: State,
    _lock: File,
}

impl AttestorSendJournal {
    pub fn open(root: impl AsRef<Path>, key: [u8; 32]) -> Result<Self, KmsError> {
        if key == [0; 32] || !root.as_ref().is_absolute() {
            return Err(KmsError::InvalidConfiguration);
        }
        let root = root.as_ref().to_path_buf();
        if !root.exists() {
            fs::create_dir_all(&root).map_err(|_| KmsError::Unavailable)?;
            fs::set_permissions(&root, fs::Permissions::from_mode(0o700))
                .map_err(|_| KmsError::Unavailable)?;
        }
        let metadata = fs::symlink_metadata(&root).map_err(|_| KmsError::Unavailable)?;
        if !metadata.is_dir() || metadata.mode() & 0o077 != 0 {
            return Err(KmsError::InvalidConfiguration);
        }
        let lock = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .mode(0o600)
            .custom_flags(rustix::fs::OFlags::NOFOLLOW.bits() as i32)
            .open(root.join("attestor-sends.lock"))
            .map_err(|_| KmsError::Unavailable)?;
        rustix::fs::flock(&lock, rustix::fs::FlockOperation::NonBlockingLockExclusive)
            .map_err(|_| KmsError::Unavailable)?;
        let path = root.join(FILE_NAME);
        let state = if path.exists() {
            let file = OpenOptions::new()
                .read(true)
                .custom_flags(rustix::fs::OFlags::NOFOLLOW.bits() as i32)
                .open(path)
                .map_err(|_| KmsError::Integrity)?;
            let metadata = file.metadata().map_err(|_| KmsError::Integrity)?;
            if !metadata.is_file() || metadata.mode() & 0o077 != 0 || metadata.len() > MAX_BYTES {
                return Err(KmsError::Integrity);
            }
            let mut bytes = Vec::new();
            file.take(MAX_BYTES + 1)
                .read_to_end(&mut bytes)
                .map_err(|_| KmsError::Integrity)?;
            if bytes.len() < 37 || bytes.len() as u64 > MAX_BYTES || &bytes[..5] != b"LXAS1" {
                return Err(KmsError::Integrity);
            }
            let cut = bytes.len() - 32;
            if !bool::from(super::production_auth::hmac(&key, &bytes[..cut]).ct_eq(&bytes[cut..])) {
                return Err(KmsError::Integrity);
            }
            let state: State =
                serde_json::from_slice(&bytes[5..cut]).map_err(|_| KmsError::Integrity)?;
            if state.sends.len() > ATTESTOR_SEND_RETENTION_CAPACITY
                || state.nonce_floors.len() > ATTESTOR_SEND_RETENTION_CAPACITY
            {
                return Err(KmsError::Integrity);
            }
            for entry in state.sends.values() {
                if state
                    .nonce_floors
                    .get(&wallet_key(&entry.request))
                    .is_none_or(|floor| *floor < entry.request.authorization.transaction.nonce)
                {
                    return Err(KmsError::Integrity);
                }
            }
            state
        } else {
            State::default()
        };
        match fs::remove_file(root.join(TEMP_NAME)) {
            Ok(()) => (),
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => (),
            Err(_) => return Err(KmsError::Unavailable),
        }
        Ok(Self {
            root,
            key: Zeroizing::new(key),
            state,
            _lock: lock,
        })
    }

    /// Looks up a completed exact request without reserving unauthenticated work.
    pub fn replay(&self, payload: &[u8], now: u64) -> Result<Option<Vec<u8>>, KmsError> {
        let request: AttestorSendAuthorization =
            serde_json::from_slice(payload).map_err(|_| KmsError::Refused)?;
        if now < self.state.observed_at
            || request.authorization.not_before > now
            || request.authorization.not_after < now
        {
            return Err(KmsError::Refused);
        }
        match self.state.sends.get(&action_key(&request)) {
            Some(entry) if entry.request == request => Ok(entry.raw_transaction.clone()),
            Some(_) => Err(KmsError::Conflict),
            None => Ok(None),
        }
    }

    /// Returns an exact signed replay or durably reserves the request before signing.
    pub fn reserve(&mut self, payload: &[u8], now: u64) -> Result<Option<Vec<u8>>, KmsError> {
        let request: AttestorSendAuthorization =
            serde_json::from_slice(payload).map_err(|_| KmsError::Refused)?;
        let authorization = &request.authorization;
        if now < self.state.observed_at
            || authorization.not_before > now
            || authorization.not_after < now
            || authorization.not_after <= authorization.not_before
            || authorization.action_key == [0; 32]
            || authorization.wallet == [0; 20]
        {
            return Err(KmsError::Refused);
        }
        let id = action_key(&request);
        if let Some(entry) = self.state.sends.get(&id) {
            return if entry.request == request {
                Ok(entry.raw_transaction.clone())
            } else {
                Err(KmsError::Conflict)
            };
        }
        let mut next = self.state.clone();
        next.observed_at = now;
        reconcile(&mut next, now)?;
        let wallet = wallet_key(&request);
        if next
            .nonce_floors
            .get(&wallet)
            .is_some_and(|floor| *floor >= authorization.transaction.nonce)
        {
            return Err(KmsError::Conflict);
        }
        if next.sends.len() >= ATTESTOR_SEND_RETENTION_CAPACITY
            || (!next.nonce_floors.contains_key(&wallet)
                && next.nonce_floors.len() >= ATTESTOR_SEND_RETENTION_CAPACITY)
        {
            return Err(KmsError::Unavailable);
        }
        next.nonce_floors
            .insert(wallet, authorization.transaction.nonce);
        next.sends.insert(
            id,
            Entry {
                request,
                raw_transaction: None,
            },
        );
        self.commit(next)?;
        Ok(None)
    }

    /// Completes only an existing exact reservation with the matching signed transaction.
    pub fn complete(&mut self, payload: &[u8], raw: Vec<u8>) -> Result<Vec<u8>, KmsError> {
        let request: AttestorSendAuthorization =
            serde_json::from_slice(payload).map_err(|_| KmsError::Refused)?;
        let (call, signature) = layerx_crypto::evm_transaction::decode_signed(&raw)
            .map_err(|_| KmsError::InvalidResponse)?;
        let tx = &request.authorization.transaction;
        if call.chain_id != tx.chain_id
            || call.nonce != tx.nonce
            || call.to != tx.to
            || call.data != tx.calldata
            || call.gas_limit != tx.gas_limit
            || call.max_fee_per_gas != u128::from(tx.max_fee_per_gas)
            || call.max_priority_fee_per_gas != u128::from(tx.max_priority_fee_per_gas)
            || layerx_crypto::evm_transaction::recover_sender(&call, &signature)
                .map_err(|_| KmsError::Integrity)?
                != request.authorization.wallet
        {
            return Err(KmsError::Integrity);
        }
        let mut next = self.state.clone();
        let entry = next
            .sends
            .get_mut(&action_key(&request))
            .ok_or(KmsError::Conflict)?;
        if entry.request != request
            || entry
                .raw_transaction
                .as_ref()
                .is_some_and(|prior| *prior != raw)
        {
            return Err(KmsError::Conflict);
        }
        entry.raw_transaction = Some(raw.clone());
        self.commit(next)?;
        Ok(raw)
    }

    pub fn retained_requests(&self) -> usize {
        self.state.sends.len()
    }

    fn commit(&mut self, next: State) -> Result<(), KmsError> {
        let mut bytes = b"LXAS1".to_vec();
        bytes.extend_from_slice(&serde_json::to_vec(&next).map_err(|_| KmsError::Integrity)?);
        bytes.extend_from_slice(&super::production_auth::hmac(&self.key, &bytes));
        if bytes.len() as u64 > MAX_BYTES {
            return Err(KmsError::Unavailable);
        }
        let temp = self.root.join(TEMP_NAME);
        let result = (|| {
            let mut file = OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(&temp)
                .map_err(|_| KmsError::Unavailable)?;
            file.write_all(&bytes)
                .and_then(|()| file.sync_all())
                .map_err(|_| KmsError::Unavailable)?;
            fs::rename(&temp, self.root.join(FILE_NAME)).map_err(|_| KmsError::Unavailable)?;
            // Publish memory before directory fsync: a failed fsync must never allow
            // the running process to forget a reservation already renamed on disk.
            self.state = next;
            File::open(&self.root)
                .and_then(|file| file.sync_all())
                .map_err(|_| KmsError::Unavailable)
        })();
        if result.is_err() {
            let _ = fs::remove_file(temp);
        }
        result
    }
}

fn action_key(request: &AttestorSendAuthorization) -> String {
    request
        .authorization
        .action_key
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect()
}
fn wallet_key(request: &AttestorSendAuthorization) -> String {
    format!(
        "{}:{}",
        request.authorization.transaction.chain_id,
        request
            .authorization
            .wallet
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    )
}
fn reconcile(state: &mut State, now: u64) -> Result<(), KmsError> {
    for entry in state.sends.values() {
        if state
            .nonce_floors
            .get(&wallet_key(&entry.request))
            .is_none_or(|floor| *floor < entry.request.authorization.transaction.nonce)
        {
            return Err(KmsError::Integrity);
        }
    }
    state
        .sends
        .retain(|_, entry| entry.request.authorization.not_after >= now);
    Ok(())
}
