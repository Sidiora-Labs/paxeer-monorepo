//! Durable migration-head producer contract for the hosted authority (domain 8).
//! The HTTPS router supplies an authenticated principal, never a caller-selected
//! identity. Independent majority backends each retain their own durable volume;
//! neither this store nor the journal may share rollback snapshots.

use std::fs::{self, File, OpenOptions};
use std::io::{Read, Write};
use std::os::unix::fs::{
    DirBuilderExt as _, MetadataExt as _, OpenOptionsExt as _, PermissionsExt as _,
};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_interop_gateway::principal::PrincipalId;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

use crate::source_codec::{decode_fixed_hex, hex};
use crate::MigrationError;

pub const GET_MIGRATION_JOURNAL_HEAD: &str = "layerx_getMigrationJournalHead";
pub const ADVANCE_MIGRATION_JOURNAL_HEAD: &str = "layerx_advanceMigrationJournalHead";
static TEMPORARY: AtomicU64 = AtomicU64::new(1);

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct MigrationJournalHead {
    pub sequence: u64,
    pub digest: String,
}

impl MigrationJournalHead {
    pub(crate) fn decode(&self) -> Result<(u64, [u8; 32]), MigrationError> {
        let digest = decode_fixed_hex(&format!("0x{}", self.digest))?;
        if hex(&digest) != self.digest || (self.sequence == 0) != (digest == [0; 32]) {
            return Err(MigrationError::CheckpointIntegrity);
        }
        Ok((self.sequence, digest))
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct AdvanceMigrationJournalHead {
    pub anchor_id: String,
    pub expected_sequence: u64,
    pub expected_digest: String,
    pub sequence: u64,
    pub digest: String,
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct DurableHead {
    version: u8,
    owner: String,
    anchor_id: String,
    head: MigrationJournalHead,
    previous: Option<MigrationJournalHead>,
}

/// One explicitly enrolled namespace. Enrolment belongs to the hosted control
/// plane; RPC requests cannot create namespaces or change their owner.
/// Atomic replacement plus file and directory fsync precede every CAS reply.
/// A stable OS file lock serializes independent handles and processes.
pub struct MigrationHeadAuthority {
    directory: PathBuf,
    owner: PrincipalId,
    anchor_id: String,
}

impl MigrationHeadAuthority {
    ///
    /// # Errors
    /// Returns an error if the requested operation cannot be completed.
    pub fn open(
        directory: &Path,
        owner: PrincipalId,
        anchor_id: String,
    ) -> Result<Self, MigrationError> {
        if !directory.is_absolute() || !valid_anchor(&anchor_id) {
            return Err(MigrationError::Configuration);
        }
        let created = match fs::DirBuilder::new().mode(0o700).create(directory) {
            Ok(()) => true,
            Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => false,
            Err(_) => return Err(MigrationError::Configuration),
        };
        let metadata =
            fs::symlink_metadata(directory).map_err(|_| MigrationError::Configuration)?;
        if !metadata.is_dir()
            || metadata.permissions().mode() & 0o077 != 0
            || fs::canonicalize(directory).ok().as_deref() != Some(directory)
        {
            return Err(MigrationError::Configuration);
        }
        let store = Self {
            directory: directory.to_owned(),
            owner,
            anchor_id,
        };
        if created {
            let lock = OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(store.directory.join("lock"))
                .map_err(|_| MigrationError::CheckpointIntegrity)?;
            lock.lock()
                .map_err(|_| MigrationError::CheckpointIntegrity)?;
            store.persist(
                &MigrationJournalHead {
                    sequence: 0,
                    digest: hex(&[0; 32]),
                },
                None,
            )?;
            File::open(directory.parent().ok_or(MigrationError::Configuration)?)
                .and_then(|parent| parent.sync_all())
                .map_err(|_| MigrationError::CheckpointIntegrity)?;
        } else {
            store.current()?;
        }
        Ok(store)
    }

    fn lock(&self) -> Result<File, MigrationError> {
        let path = self.directory.join("lock");
        self.validate_file(&path)?;
        let file = OpenOptions::new()
            .read(true)
            .write(true)
            .open(path)
            .map_err(|_| MigrationError::CheckpointIntegrity)?;
        file.lock()
            .map_err(|_| MigrationError::CheckpointIntegrity)?;
        Ok(file)
    }

    fn validate_file(&self, path: &Path) -> Result<(), MigrationError> {
        let metadata =
            fs::symlink_metadata(path).map_err(|_| MigrationError::CheckpointIntegrity)?;
        let owner = fs::metadata(&self.directory)
            .map_err(|_| MigrationError::CheckpointIntegrity)?
            .uid();
        if !metadata.is_file()
            || metadata.nlink() != 1
            || metadata.uid() != owner
            || metadata.permissions().mode() & 0o077 != 0
        {
            return Err(MigrationError::CheckpointIntegrity);
        }
        Ok(())
    }

    fn load(&self) -> Result<DurableHead, MigrationError> {
        let path = self.directory.join("head.json");
        self.validate_file(&path)?;
        let mut bytes = Vec::new();
        File::open(path)
            .and_then(|file| file.take(4097).read_to_end(&mut bytes))
            .map_err(|_| MigrationError::CheckpointIntegrity)?;
        if bytes.len() > 4096 {
            return Err(MigrationError::CheckpointIntegrity);
        }
        let record: DurableHead =
            serde_json::from_slice(&bytes).map_err(|_| MigrationError::CheckpointIntegrity)?;
        if record.version != 1
            || record.owner != self.owner.as_str()
            || record.anchor_id != self.anchor_id
        {
            return Err(MigrationError::CheckpointIntegrity);
        }
        record.head.decode()?;
        match &record.previous {
            None if record.head.sequence == 0 => {}
            Some(previous)
                if previous.sequence.checked_add(1) == Some(record.head.sequence)
                    && previous.digest != record.head.digest =>
            {
                previous.decode()?;
            }
            _ => return Err(MigrationError::CheckpointIntegrity),
        }
        Ok(record)
    }

    fn persist(
        &self,
        head: &MigrationJournalHead,
        previous: Option<MigrationJournalHead>,
    ) -> Result<(), MigrationError> {
        let bytes = serde_json::to_vec(&DurableHead {
            version: 1,
            owner: self.owner.as_str().to_owned(),
            anchor_id: self.anchor_id.clone(),
            head: head.clone(),
            previous,
        })
        .map_err(|_| MigrationError::CheckpointIntegrity)?;
        let temporary = self.directory.join(format!(
            ".head-{}-{}",
            std::process::id(),
            TEMPORARY.fetch_add(1, Ordering::Relaxed)
        ));
        let mut file = OpenOptions::new()
            .create_new(true)
            .write(true)
            .mode(0o600)
            .open(&temporary)
            .map_err(|_| MigrationError::CheckpointIntegrity)?;
        file.write_all(&bytes)
            .and_then(|()| file.sync_all())
            .and_then(|()| fs::rename(&temporary, self.directory.join("head.json")))
            .and_then(|()| File::open(&self.directory)?.sync_all())
            .map_err(|_| MigrationError::CheckpointIntegrity)
    }

    ///
    /// # Errors
    /// Returns an error if the requested operation cannot be completed.
    pub fn current(&self) -> Result<MigrationJournalHead, MigrationError> {
        let _lock = self.lock()?;
        Ok(self.load()?.head)
    }

    ///
    /// # Errors
    /// Returns an error if the requested operation cannot be completed.
    pub fn advance(
        &self,
        principal: &PrincipalId,
        request: &AdvanceMigrationJournalHead,
    ) -> Result<MigrationJournalHead, MigrationError> {
        self.authorize(principal, &request.anchor_id)?;
        let expected = MigrationJournalHead {
            sequence: request.expected_sequence,
            digest: request.expected_digest.clone(),
        };
        let next = MigrationJournalHead {
            sequence: request.sequence,
            digest: request.digest.clone(),
        };
        expected.decode()?;
        next.decode()?;
        if expected.sequence.checked_add(1) != Some(next.sequence) || expected.digest == next.digest
        {
            return Err(MigrationError::CheckpointConflict);
        }
        let _lock = self.lock()?;
        let current = self.load()?;
        // A quorum retry can reach a backend that durably accepted the first CAS.
        if current.head == next && current.previous.as_ref() == Some(&expected) {
            return Ok(current.head);
        }
        if current.head != expected {
            return Err(MigrationError::CheckpointConflict);
        }
        self.persist(&next, Some(expected))?;
        Ok(next)
    }

    fn authorize(&self, principal: &PrincipalId, anchor_id: &str) -> Result<(), MigrationError> {
        if principal != &self.owner || anchor_id != self.anchor_id {
            return Err(MigrationError::PlaneRefused);
        }
        Ok(())
    }

    /// JSON-RPC handler for a router that has already authenticated HTTPS bearer
    /// credentials to a principal. Invalid/unowned requests never touch state.
    ///
    /// # Errors
    /// Returns an error if the requested operation cannot be completed.
    pub fn handle(
        &self,
        principal: &PrincipalId,
        request: &Value,
    ) -> Result<Value, MigrationError> {
        if request.get("jsonrpc").and_then(Value::as_str) != Some("2.0")
            || request.get("id").and_then(Value::as_u64).is_none()
        {
            return Err(MigrationError::RpcResponseMismatch);
        }
        let params = request
            .get("params")
            .and_then(Value::as_array)
            .filter(|params| params.len() == 1)
            .ok_or(MigrationError::RpcResponseMismatch)?;
        let head = match request.get("method").and_then(Value::as_str) {
            Some(GET_MIGRATION_JOURNAL_HEAD) => {
                let anchor = params[0]
                    .as_str()
                    .ok_or(MigrationError::RpcResponseMismatch)?;
                self.authorize(principal, anchor)?;
                self.current()?
            }
            Some(ADVANCE_MIGRATION_JOURNAL_HEAD) => {
                let advance = serde_json::from_value(params[0].clone())
                    .map_err(|_| MigrationError::RpcResponseMismatch)?;
                self.advance(principal, &advance)?
            }
            _ => return Err(MigrationError::RpcResponseMismatch),
        };
        Ok(json!({"jsonrpc":"2.0", "id":request["id"], "result":head}))
    }
}

fn valid_anchor(value: &str) -> bool {
    !value.is_empty()
        && value.len() <= 512
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b':' | b'.'))
}
