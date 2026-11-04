//! Durable, bounded-size checkpoint for a Solana backfill. Relay observations
//! remain in the main journal; a checkpoint is saved only after they are synced.

use std::fs::{self, File, OpenOptions};
use std::io::{Read as _, Write as _};
use std::path::{Path, PathBuf};

use serde::{Deserialize, Serialize};

use super::observe::SolanaSettings;
use crate::hex;
use crate::journal::JournalError;

#[derive(Clone, Copy, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct SignatureCursor {
    #[serde(with = "hex::fixed_serde")]
    pub signature: [u8; 64],
    pub slot: u64,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct BackfillContinuation {
    pub chain_id: u64,
    #[serde(with = "hex::fixed_serde")]
    pub program_id: [u8; 32],
    #[serde(with = "hex::fixed_serde")]
    pub vault: [u8; 20],
    pub commitment: String,
    pub from: u64,
    pub to: u64,
    pub before: Option<SignatureCursor>,
}

impl BackfillContinuation {
    pub(super) fn new(settings: &SolanaSettings, from: u64, to: u64) -> Self {
        Self {
            chain_id: settings.chain_id,
            program_id: settings.program_id,
            vault: settings.vault,
            commitment: settings.commitment.as_str().to_owned(),
            from,
            to,
            before: None,
        }
    }

    pub(super) fn validate(&self, settings: &SolanaSettings) -> Result<(), JournalError> {
        if self.chain_id != settings.chain_id
            || self.program_id != settings.program_id
            || self.vault != settings.vault
            || self.commitment != settings.commitment.as_str()
            || self.from > self.to
            || self.to == u64::MAX
            || self.before.is_some_and(|before| before.slot < self.from)
        {
            return Err(JournalError::Conflict(
                "Solana backfill identity or window disagrees with settings".to_owned(),
            ));
        }
        Ok(())
    }
}

/// One atomic checkpoint beside the relay journal. It stores the program,
/// inclusive bounds and pagination cursor together, including before page one.
pub struct BackfillJournal {
    path: PathBuf,
    pending: Option<BackfillContinuation>,
}

fn io(error: std::io::Error) -> JournalError {
    JournalError::Io(error.to_string())
}

impl BackfillJournal {
    pub fn open(path: &Path) -> Result<Self, JournalError> {
        let pending = match File::open(path) {
            Ok(file) => {
                let mut bytes = Vec::new();
                file.take(4097).read_to_end(&mut bytes).map_err(io)?;
                if bytes.len() > 4096 {
                    return Err(JournalError::Corrupt { line: 1 });
                }
                serde_json::from_slice(&bytes).map_err(|_| JournalError::Corrupt { line: 1 })?
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => None,
            Err(error) => return Err(io(error)),
        };
        Ok(Self {
            path: path.to_path_buf(),
            pending,
        })
    }

    #[must_use]
    pub fn pending(&self) -> Option<&BackfillContinuation> {
        self.pending.as_ref()
    }

    pub(super) fn save(
        &mut self,
        pending: Option<BackfillContinuation>,
    ) -> Result<(), JournalError> {
        let bytes =
            serde_json::to_vec(&pending).map_err(|error| JournalError::Io(error.to_string()))?;
        let mut temporary = self.path.as_os_str().to_os_string();
        temporary.push(".tmp");
        let temporary = PathBuf::from(temporary);
        let mut file = OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .open(&temporary)
            .map_err(io)?;
        file.write_all(&bytes).map_err(io)?;
        file.sync_all().map_err(io)?;
        fs::rename(&temporary, &self.path).map_err(io)?;
        let parent = self
            .path
            .parent()
            .filter(|path| !path.as_os_str().is_empty())
            .unwrap_or_else(|| Path::new("."));
        File::open(parent)
            .and_then(|directory| directory.sync_all())
            .map_err(io)?;
        self.pending = pending;
        Ok(())
    }
}
