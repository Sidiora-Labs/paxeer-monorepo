use std::collections::BTreeSet;
use std::fs::{File, OpenOptions};
use std::io::{Read as _, Write as _};
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::path::{Path, PathBuf};

use layerx_intents::canonical::MAX_MESSAGE_BYTES;
use layerx_types::result::ResultCode;
use sha2::{Digest as _, Sha256};

const JOURNAL_NAME: &str = "admission.journal";
const JOURNAL_MAGIC: &[u8; 4] = b"LXHA";
const JOURNAL_VERSION: u16 = 1;
const JOURNAL_SUPERBLOCK_BYTES: usize = 10;
const RECORD_MAGIC: &[u8; 4] = b"LXAR";
const RECORD_HEADER_BYTES: usize = 40;
const RECORD_DIGEST_BYTES: usize = 32;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Refusal {
    pub class: u8,
    pub result: ResultCode,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AdmissionRecord {
    pub tx_hash: [u8; 32],
    pub raw_transaction: Vec<u8>,
}

#[derive(Debug)]
pub struct AdmissionJournal {
    directory: PathBuf,
    file: File,
    records: Vec<AdmissionRecord>,
    admitted: BTreeSet<[u8; 32]>,
}

impl AdmissionJournal {
    /// # Panics
    /// Panics if journal creation, permissions, persistence, or recovery fails.
    #[must_use]
    pub fn open(directory: &Path, network_id: u32) -> Self {
        std::fs::create_dir_all(directory)
            .unwrap_or_else(|error| panic!("admission directory: {error}"));
        std::fs::set_permissions(directory, std::fs::Permissions::from_mode(0o700))
            .unwrap_or_else(|error| panic!("admission directory permissions: {error}"));
        let path = directory.join(JOURNAL_NAME);
        let existed = path.exists();
        let mut file = OpenOptions::new()
            .read(true)
            .append(true)
            .create(true)
            .mode(0o600)
            .open(&path)
            .unwrap_or_else(|error| panic!("admission journal: {error}"));
        let records = if existed {
            Self::recover_from(&mut file, network_id)
        } else {
            file.write_all(&superblock(network_id))
                .unwrap_or_else(|error| panic!("admission superblock: {error}"));
            file.sync_data()
                .unwrap_or_else(|error| panic!("admission superblock fdatasync: {error}"));
            File::open(directory)
                .and_then(|parent| parent.sync_all())
                .unwrap_or_else(|error| panic!("admission directory fsync: {error}"));
            Vec::new()
        };
        let admitted = records.iter().map(|record| record.tx_hash).collect();
        Self {
            directory: directory.to_path_buf(),
            file,
            records,
            admitted,
        }
    }

    /// # Panics
    /// Panics if the journal cannot be read or its superblock does not match the network.
    #[must_use]
    pub fn recover(directory: &Path, network_id: u32) -> Vec<AdmissionRecord> {
        let mut file = File::open(directory.join(JOURNAL_NAME))
            .unwrap_or_else(|error| panic!("recover admission journal: {error}"));
        Self::recover_from(&mut file, network_id)
    }

    fn recover_from(file: &mut File, network_id: u32) -> Vec<AdmissionRecord> {
        let mut bytes = Vec::new();
        file.read_to_end(&mut bytes)
            .unwrap_or_else(|error| panic!("read admission journal: {error}"));
        assert!(
            bytes.len() >= JOURNAL_SUPERBLOCK_BYTES
                && bytes[..JOURNAL_SUPERBLOCK_BYTES] == superblock(network_id),
            "admission journal superblock mismatch"
        );
        let mut records = Vec::new();
        let mut cursor = JOURNAL_SUPERBLOCK_BYTES;
        while cursor + RECORD_HEADER_BYTES <= bytes.len() {
            let header = &bytes[cursor..cursor + RECORD_HEADER_BYTES];
            if &header[..4] != RECORD_MAGIC {
                break;
            }
            let length = usize::try_from(u32::from_be_bytes([
                header[4], header[5], header[6], header[7],
            ]))
            .unwrap_or_else(|_| panic!("admission record length overflow"));
            let mut tx_hash = [0_u8; 32];
            tx_hash.copy_from_slice(&header[8..40]);
            let body_end = cursor + RECORD_HEADER_BYTES + length;
            let record_end = body_end + RECORD_DIGEST_BYTES;
            if length == 0 || length > MAX_MESSAGE_BYTES || record_end > bytes.len() {
                break;
            }
            let raw_transaction = &bytes[cursor + RECORD_HEADER_BYTES..body_end];
            if bytes[body_end..record_end] != record_digest(&header[4..40], raw_transaction) {
                break;
            }
            records.push(AdmissionRecord {
                tx_hash,
                raw_transaction: raw_transaction.to_vec(),
            });
            cursor = record_end;
        }
        records
    }

    #[must_use]
    pub fn contains(&self, tx_hash: &[u8; 32]) -> bool {
        self.admitted.contains(tx_hash)
    }

    #[must_use]
    pub fn records(&self) -> &[AdmissionRecord] {
        &self.records
    }

    #[must_use]
    pub fn directory(&self) -> &Path {
        &self.directory
    }

    fn admit(&mut self, tx_hash: [u8; 32], raw_transaction: &[u8]) -> std::io::Result<()> {
        let mut record =
            Vec::with_capacity(RECORD_HEADER_BYTES + raw_transaction.len() + RECORD_DIGEST_BYTES);
        record.extend_from_slice(RECORD_MAGIC);
        let length = u32::try_from(raw_transaction.len())
            .map_err(|_| std::io::Error::other("admission record length overflow"))?;
        record.extend_from_slice(&length.to_be_bytes());
        record.extend_from_slice(&tx_hash);
        record.extend_from_slice(raw_transaction);
        let digest = record_digest(&record[4..RECORD_HEADER_BYTES], raw_transaction);
        record.extend_from_slice(&digest);
        self.file.write_all(&record)?;
        self.file.sync_data()?;
        self.records.push(AdmissionRecord {
            tx_hash,
            raw_transaction: raw_transaction.to_vec(),
        });
        self.admitted.insert(tx_hash);
        Ok(())
    }
}

fn superblock(network_id: u32) -> [u8; JOURNAL_SUPERBLOCK_BYTES] {
    let mut bytes = [0_u8; JOURNAL_SUPERBLOCK_BYTES];
    bytes[..4].copy_from_slice(JOURNAL_MAGIC);
    bytes[4..6].copy_from_slice(&JOURNAL_VERSION.to_be_bytes());
    bytes[6..10].copy_from_slice(&network_id.to_be_bytes());
    bytes
}

fn record_digest(header_tail: &[u8], raw_transaction: &[u8]) -> [u8; RECORD_DIGEST_BYTES] {
    let mut digest = Sha256::new();
    digest.update(b"LXH/v1/admission-record\0");
    digest.update(header_tail);
    digest.update(raw_transaction);
    digest.finalize().into()
}
