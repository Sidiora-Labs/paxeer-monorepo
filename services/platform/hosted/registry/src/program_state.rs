use std::fs;
use std::io::Read as _;
use std::path::{Path, PathBuf};
use std::time::{Duration, Instant};

use layerx_crypto::evm_transaction::transaction_hash as keccak256;
use layerx_programs::{hex, ProgramId};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

use crate::write_atomic;
use crate::ProgramStateCursor;

const RECORD_SUFFIX: &str = ".program-state";
const CURSOR_FILE: &str = "canonical.cursor";
const PROGRAMS: &str = "0x000000000000000000000000000000000000101d";
const MAX_RESPONSE_BYTES: u64 = 2 * 1024 * 1024 + 1024;
const READ_TIMEOUT: Duration = Duration::from_secs(15);

/// Durable cache of native Programs storage heads read through eth_call.
pub struct FileProgramStateJournal {
    root: PathBuf,
}

impl FileProgramStateJournal {
    ///
    /// # Errors
    /// Returns a filesystem error when the journal directory cannot be created.
    pub fn open(root: PathBuf) -> Result<Self, String> {
        fs::create_dir_all(&root).map_err(|error| {
            format!(
                "cannot create program-state journal {}: {error}",
                root.display()
            )
        })?;
        Ok(Self { root })
    }

    /// # Errors
    /// Refuses invalid endpoints, unavailable calls, malformed state and persistence failures.
    pub fn store(
        &self,
        endpoint: &str,
        program: ProgramId,
        sequence: u64,
        deadline: Instant,
    ) -> Result<Vec<u8>, String> {
        if sequence == 0 {
            return Err("program-state reads require a committed height".to_owned());
        }
        if endpoint.len() > 8192 || !endpoint.is_ascii() || endpoint.contains(['@', '#']) {
            return Err("invalid programs chain RPC URL".to_owned());
        }
        let uri: ureq::http::Uri = endpoint
            .parse()
            .map_err(|_| "invalid programs chain RPC URL".to_owned())?;
        let host = uri.host().ok_or("programs chain RPC URL needs a host")?;
        match uri.scheme_str() {
            Some("https") => {}
            Some("http") if matches!(host, "127.0.0.1" | "localhost" | "[::1]") => {}
            _ => return Err("programs chain RPC requires HTTPS or loopback HTTP".to_owned()),
        }
        let deadline = deadline.min(Instant::now() + READ_TIMEOUT);
        let remaining = deadline
            .checked_duration_since(Instant::now())
            .filter(|remaining| !remaining.is_zero())
            .ok_or("program-state read deadline expired")?;
        let selector = keccak256(b"getStateHead(bytes32)");
        let data = format!(
            "0x{}{}",
            hex::encode(&selector[..4]),
            hex::encode(&program.bytes())
        );
        let agent: ureq::Agent = ureq::Agent::config_builder()
            .max_redirects(0)
            .timeout_global(Some(remaining))
            .build()
            .into();
        let mut response = agent
            .post(endpoint)
            .send_json(json!({
                "jsonrpc": "2.0", "id": 1, "method": "eth_call",
                "params": [{"to": PROGRAMS, "data": data}, format!("0x{sequence:x}")],
            }))
            .map_err(|error| format!("program-state eth_call failed: {error}"))?;
        let mut bytes = Vec::new();
        response
            .body_mut()
            .as_reader()
            .take(MAX_RESPONSE_BYTES + 1)
            .read_to_end(&mut bytes)
            .map_err(|error| format!("program-state eth_call response failed: {error}"))?;
        if Instant::now() >= deadline {
            return Err("program-state read deadline expired".to_owned());
        }
        self.store_response(program, &bytes)
    }

    /// # Errors
    /// Refuses malformed RPC/ABI encodings and native storage heads; cached bytes are not read authority.
    pub fn store_response(&self, program: ProgramId, response: &[u8]) -> Result<Vec<u8>, String> {
        if response.len() as u64 > MAX_RESPONSE_BYTES {
            return Err("program-state response exceeds its byte bound".to_owned());
        }
        let document: Value = serde_json::from_slice(response)
            .map_err(|error| format!("program-state RPC JSON: {error}"))?;
        if document["jsonrpc"] != "2.0" || document["id"] != 1 || document.get("error").is_some() {
            return Err(
                "program-state eth_call refused or returned an invalid envelope".to_owned(),
            );
        }
        let encoded = document["result"]
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .ok_or("program-state eth_call did not return hex bytes")?;
        let encoded =
            hex::decode(encoded).map_err(|error| format!("program-state eth_call hex: {error}"))?;
        let mut offset = [0; 32];
        offset[31] = 32;
        let mut length = [0; 32];
        length[31] = 38;
        if encoded.len() != 128
            || encoded[..32] != offset
            || encoded[32..64] != length
            || encoded[102..].iter().any(|byte| *byte != 0)
        {
            return Err("noncanonical getStateHead ABI bytes".to_owned());
        }
        let bytes = &encoded[64..102];
        if bytes[..2] != [0, 1] || bytes[6..].iter().all(|byte| *byte == 0) {
            return Err("invalid native program storage head".to_owned());
        }
        if bytes[2..6] == [0; 4] && bytes[6..] != Sha256::digest(&bytes[..6])[..] {
            return Err("invalid empty program storage manifest digest".to_owned());
        }
        let digest: [u8; 32] = Sha256::digest(bytes).into();
        let path = self.record_path(program, digest);
        write_atomic(&path, bytes).map_err(|error| {
            format!(
                "cannot persist program-state record {}: {error}",
                path.display()
            )
        })?;
        Ok(bytes.to_vec())
    }

    /// Hash-checks every local cache candidate. Cached bytes never authorize a read;
    /// restart publication requires a fresh native call at the current head.
    ///
    /// # Errors
    /// Returns filesystem errors and refuses cached records whose names do not match their content digests.
    pub fn audit(&self) -> Result<(), String> {
        let mut paths = fs::read_dir(&self.root)
            .map_err(|error| {
                format!(
                    "cannot read program-state journal {}: {error}",
                    self.root.display()
                )
            })?
            .map(|entry| entry.map(|entry| entry.path()))
            .collect::<Result<Vec<_>, _>>()
            .map_err(|error| format!("cannot enumerate program-state journal: {error}"))?;
        paths.retain(|path| is_record(path));
        paths.sort();

        for path in paths {
            let bytes = fs::read(&path).map_err(|error| {
                format!(
                    "cannot read program-state record {}: {error}",
                    path.display()
                )
            })?;
            let digest: [u8; 32] = Sha256::digest(&bytes).into();
            let named = path
                .file_stem()
                .and_then(|name| name.to_str())
                .and_then(|name| name.rsplit_once('.'))
                .map(|(_, digest)| digest);
            let expected = hex::encode(&digest);
            if named != Some(expected.as_str()) {
                return Err(format!(
                    "program-state cache {} does not match its content digest",
                    path.display()
                ));
            }
        }
        Ok(())
    }

    ///
    /// # Errors
    /// Returns filesystem errors other than an absent cursor and refuses malformed cursor contents.
    pub fn cursor(&self) -> Result<ProgramStateCursor, String> {
        let path = self.root.join(CURSOR_FILE);
        let text = match fs::read_to_string(&path) {
            Ok(text) => text,
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                return Ok(ProgramStateCursor::default())
            }
            Err(error) => {
                return Err(format!(
                    "cannot read program-state cursor {}: {error}",
                    path.display()
                ))
            }
        };
        let (sequence, ordinal) = text
            .trim()
            .split_once('\t')
            .ok_or_else(|| format!("program-state cursor {} is corrupt", path.display()))?;
        Ok(ProgramStateCursor {
            sequence: sequence
                .parse()
                .map_err(|_| format!("program-state cursor {} is corrupt", path.display()))?,
            ordinal: ordinal
                .parse()
                .map_err(|_| format!("program-state cursor {} is corrupt", path.display()))?,
        })
    }

    ///
    /// # Errors
    /// Refuses event ordinals, regressing or unreadable cursors and atomic persistence failures.
    pub fn advance(&self, cursor: ProgramStateCursor) -> Result<(), String> {
        if cursor.ordinal != 0 {
            return Err("program-state scan cursor cannot carry an event ordinal".to_owned());
        }
        if cursor < self.cursor()? {
            return Err("program-state cursor cannot regress".to_owned());
        }
        let path = self.root.join(CURSOR_FILE);
        write_atomic(
            &path,
            format!("{}\t{}\n", cursor.sequence, cursor.ordinal).as_bytes(),
        )
        .map_err(|error| {
            format!(
                "cannot persist program-state cursor {}: {error}",
                path.display()
            )
        })
    }

    fn record_path(&self, program: ProgramId, digest: [u8; 32]) -> PathBuf {
        self.root.join(format!(
            "{}.{}{}",
            hex::encode(&program.bytes()),
            hex::encode(&digest),
            RECORD_SUFFIX
        ))
    }
}

fn is_record(path: &Path) -> bool {
    path.is_file()
        && path
            .file_name()
            .and_then(|name| name.to_str())
            .is_some_and(|name| name.ends_with(RECORD_SUFFIX))
}
