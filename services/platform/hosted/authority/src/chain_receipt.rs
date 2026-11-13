use crate::hex;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptCheck, ReceiptKeys};
use serde::Deserialize;
use sha2::Digest as _;
use std::collections::BTreeMap;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpStream};
use std::path::Path;
use std::time::{Duration, Instant};

const MAX_KEYS_BYTES: u64 = 256 * 1024;
const MAX_RESPONSE_BYTES: usize = 2 * 1024 * 1024;
const MAX_HEADER_BYTES: usize = 32 * 1024;
const TIMEOUT: Duration = Duration::from_secs(8);

type ReceiptReply = (Vec<u8>, [u8; 32], u64, Option<(u64, [u8; 32])>);

#[derive(Debug)]
pub enum ReceiptSource {
    Found(Vec<u8>),
    Unknown,
    Unavailable(String),
    KeyMismatch,
}

pub struct ChainReceipts {
    host: String,
    address: SocketAddr,
    path: String,
    keys: Vec<SequencerAuthorization>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ReceiptKeyEntry {
    #[serde(rename = "keyId", alias = "key_id")]
    key_id: String,
    #[serde(rename = "publicKey", alias = "public_key")]
    public_key: String,
    #[serde(rename = "firstHeight", alias = "first_height")]
    first_height: u64,
    #[serde(rename = "lastHeight", alias = "last_height")]
    last_height: u64,
}

impl ChainReceipts {
    #[must_use]
    pub fn pinned_keys(&self) -> &[SequencerAuthorization] {
        &self.keys
    }

    /// # Errors
    /// Returns an error when the loopback endpoint or receipt keys are invalid.
    pub fn load(url: &str, keys_file: &Path) -> Result<Self, String> {
        let source = url
            .strip_prefix("http://")
            .ok_or("chain RPC requires http://loopback:port")?;
        let (authority, suffix) = source.split_once('/').unwrap_or((source, ""));
        let (host, address) = super::loopback_http(&format!("http://{authority}"))
            .ok_or("chain RPC requires http://loopback:port")?;
        if suffix.bytes().any(|byte| !(0x21..=0x7e).contains(&byte))
            || suffix.contains(['#', '?', '\\'])
        {
            return Err("chain RPC path is invalid".to_owned());
        }
        let bytes = super::protected::read(keys_file, MAX_KEYS_BYTES)
            .map_err(|crate::AuthorityError| "receipt key list is unavailable or unprotected")?;
        let entries: Vec<ReceiptKeyEntry> =
            serde_json::from_slice(&bytes).map_err(|_| "receipt key list is invalid")?;
        if entries.is_empty() {
            return Err("receipt key list is empty".to_owned());
        }
        let mut keys = Vec::with_capacity(entries.len());
        for entry in entries {
            let id = base64_key(&entry.key_id).ok_or("receipt key ID is invalid")?;
            let key = base64_key(&entry.public_key).ok_or("receipt public key is invalid")?;
            if id == [0; 32]
                || key == [0; 32]
                || entry.first_height > entry.last_height
                || keys
                    .iter()
                    .any(|existing: &SequencerAuthorization| existing.sequencer_id() == id)
            {
                return Err("receipt key identity or range is invalid".to_owned());
            }
            keys.push(SequencerAuthorization::new(
                id,
                key,
                entry.first_height,
                entry.last_height,
            ));
        }
        Ok(Self {
            host,
            address,
            path: format!("/{suffix}"),
            keys,
        })
    }

    #[must_use]
    pub fn lookup(&self, activity_id: [u8; 32]) -> ReceiptSource {
        match self.read(activity_id) {
            Ok(None) => ReceiptSource::Unknown,
            Ok(Some((bytes, signing_key, height, _))) => {
                let decoded = layerx_wire::receipt::decode(&bytes);
                let native = decoded
                    .as_ref()
                    .ok()
                    .and_then(|receipt| receipt.protocol())
                    .is_some_and(|receipt| (1..=11).contains(&receipt.module_id()));
                let receipt_keys = if native {
                    // Native execution receipts carry a content-addressed batch ID;
                    // the signed replica header independently checks the same key range.
                    let Some(key) = self.keys.iter().find(|key| {
                        key.public_key() == signing_key
                            && key.sequencer_id()
                                == <[u8; 32]>::from(sha2::Sha256::digest(signing_key))
                            && (key.first_batch_number()..=key.last_batch_number())
                                .contains(&height)
                    }) else {
                        return ReceiptSource::KeyMismatch;
                    };
                    ReceiptKeys::HistoricalKernel(key.public_key())
                } else {
                    ReceiptKeys::HeightRanges(&self.keys)
                };
                match verify_sequencer_signature(&bytes, receipt_keys) {
                    Ok(receipt)
                        if receipt
                            .protocol()
                            .is_some_and(|protocol| protocol.activity_id() == activity_id) =>
                    {
                        let Some(protocol) = receipt.protocol() else {
                            return ReceiptSource::Unavailable(
                                "chain receipt shape invalid".to_owned(),
                            );
                        };
                        if self.keys.iter().any(|key| {
                            (native || key.sequencer_id() == protocol.context_hash())
                                && key.public_key() == signing_key
                        }) {
                            ReceiptSource::Found(bytes)
                        } else {
                            ReceiptSource::KeyMismatch
                        }
                    }
                    Ok(_) => ReceiptSource::Unavailable(
                        "chain receipt identifies another transaction".to_owned(),
                    ),
                    Err(failure) if failure.check == ReceiptCheck::SequencerSignature => {
                        ReceiptSource::KeyMismatch
                    }
                    Err(failure) => ReceiptSource::Unavailable(format!(
                        "chain receipt verification: {:?}",
                        failure.check
                    )),
                }
            }
            Err(reason) => ReceiptSource::Unavailable(reason),
        }
    }

    /// # Errors
    /// Returns an error when the receipt is missing, uncommitted, or invalid.
    pub fn committed(&self, id: [u8; 32]) -> Result<(Vec<u8>, u64, [u8; 32]), String> {
        let (bytes, _, _, committed) = self.read(id)?.ok_or("committed receipt missing")?;
        let (height, hash) = committed.ok_or("receipt is not committed")?;
        Ok((bytes, height, hash))
    }

    fn read(&self, activity_id: [u8; 32]) -> Result<Option<ReceiptReply>, String> {
        let raw = self.request_receipt(activity_id)?;
        let document: serde_json::Value =
            serde_json::from_slice(&raw).map_err(|error| error.to_string())?;
        if document["jsonrpc"] != "2.0" || document["id"] != 1 || document.get("error").is_some() {
            return Err("chain RPC response refused or malformed".to_owned());
        }
        let result = document.get("result").ok_or("chain RPC result missing")?;
        if result.is_null() {
            return Ok(None);
        }
        let tx_hash = rpc_hex32(&result["txHash"]).ok_or("chain RPC transaction hash missing")?;
        if tx_hash != activity_id {
            return Err("chain RPC transaction hash mismatch".to_owned());
        }
        match result["rung"].as_str() {
            Some("submitted") if result.get("receipt").is_none() => return Ok(None),
            Some("receipted" | "committed") => {}
            _ => return Err("chain RPC receipt rung invalid".to_owned()),
        }
        let bytes = result["receipt"]
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .and_then(|text| hex::decode(text).ok())
            .ok_or("chain RPC receipt bytes invalid")?;
        let decoded =
            layerx_wire::receipt::decode(&bytes).map_err(|_| "chain RPC receipt decode failed")?;
        let protocol = decoded
            .protocol()
            .ok_or("chain RPC receipt shape invalid")?;
        let signing_key =
            rpc_hex32(&result["signingKey"]).ok_or("chain RPC signing key invalid")?;
        let native = (1..=11).contains(&protocol.module_id());
        let key_id = if native {
            <[u8; 32]>::from(sha2::Sha256::digest(signing_key))
        } else {
            protocol.context_hash()
        };
        if protocol.activity_id() != activity_id || rpc_hex32(&result["keyId"]) != Some(key_id) {
            return Err("chain RPC receipt identity mismatch".to_owned());
        }
        let height = result["height"]
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .and_then(|text| u64::from_str_radix(text, 16).ok())
            .filter(|height| *height > 0)
            .ok_or("chain RPC receipt height invalid")?;
        if native {
            let key = self
                .keys
                .iter()
                .find(|key| {
                    key.sequencer_id() == key_id
                        && key.public_key() == signing_key
                        && (key.first_batch_number()..=key.last_batch_number()).contains(&height)
                })
                .ok_or("chain RPC native receipt key or height mismatch")?;
            verify_sequencer_signature(&bytes, ReceiptKeys::HistoricalKernel(key.public_key()))
                .map_err(|_| "chain RPC native receipt signature invalid")?;
        }
        let committed = if result["rung"] == "committed" {
            let block = result["blockNumber"]
                .as_str()
                .and_then(|text| text.strip_prefix("0x"))
                .and_then(|text| u64::from_str_radix(text, 16).ok())
                .filter(|block| *block >= height)
                .ok_or("chain RPC committed height invalid")?;
            let hash = rpc_hex32(&result["blockHash"])
                .filter(|hash| *hash != [0; 32])
                .ok_or("chain RPC committed hash invalid")?;
            Some((block, hash))
        } else {
            None
        };
        Ok(Some((bytes, signing_key, height, committed)))
    }

    fn request_receipt(&self, activity_id: [u8; 32]) -> Result<Vec<u8>, String> {
        let deadline = Instant::now()
            .checked_add(TIMEOUT)
            .ok_or("chain RPC deadline unavailable")?;
        let remaining = || {
            deadline
                .checked_duration_since(Instant::now())
                .filter(|duration| !duration.is_zero())
                .ok_or("chain RPC deadline elapsed".to_owned())
        };
        let body = serde_json::json!({"jsonrpc": "2.0", "id": 1,
            "method": "pax_sequencerReceipt", "params": [format!("0x{}", hex::encode(&activity_id))]}).to_string();
        let mut stream = TcpStream::connect_timeout(&self.address, remaining()?)
            .map_err(|error| error.to_string())?;
        stream
            .set_write_timeout(Some(remaining()?))
            .map_err(|error| error.to_string())?;
        let request = format!(
            "POST {} HTTP/1.0\r\nHost: {}\r\nContent-Type: application/json\r\nAccept: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
            self.path,
            self.host,
            body.len(),
            body
        );
        stream
            .write_all(request.as_bytes())
            .map_err(|error| error.to_string())?;
        let mut raw = Vec::new();
        let mut chunk = [0; 4096];
        let mut header = None;
        loop {
            stream
                .set_read_timeout(Some(remaining()?))
                .map_err(|error| error.to_string())?;
            let count = stream.read(&mut chunk).map_err(|error| error.to_string())?;
            if count == 0 {
                break;
            }
            if raw.len().saturating_add(count) > MAX_RESPONSE_BYTES + MAX_HEADER_BYTES {
                return Err("chain RPC response exceeds bound".to_owned());
            }
            raw.extend_from_slice(&chunk[..count]);
            if header.is_none() {
                if let Some(end) = raw.windows(4).position(|bytes| bytes == b"\r\n\r\n") {
                    if end > MAX_HEADER_BYTES {
                        return Err("chain RPC headers exceed bound".to_owned());
                    }
                    header = Some((end + 4, response_length(&raw[..end])?));
                } else if raw.len() > MAX_HEADER_BYTES {
                    return Err("chain RPC headers exceed bound".to_owned());
                }
            }
            if let Some((end, length)) = header {
                if length.is_some_and(|length| raw.len() >= end + length) {
                    break;
                }
            }
        }
        let (end, length) = header.ok_or("chain RPC headers missing")?;
        if length.is_some_and(|length| raw.len() != end + length)
            || raw.len() - end > MAX_RESPONSE_BYTES
        {
            return Err("chain RPC content length mismatch".to_owned());
        }
        Ok(raw[end..].to_vec())
    }
}

fn rpc_hex32(value: &serde_json::Value) -> Option<[u8; 32]> {
    hex::decode32(value.as_str()?.strip_prefix("0x")?).ok()
}

fn response_length(head: &[u8]) -> Result<Option<usize>, String> {
    let head = std::str::from_utf8(head).map_err(|_| "chain RPC headers invalid")?;
    let mut lines = head.split("\r\n");
    if !matches!(
        lines.next().and_then(|line| line.split_whitespace().nth(1)),
        Some("200")
    ) {
        return Err("chain RPC HTTP call failed".to_owned());
    }
    let mut headers = BTreeMap::new();
    for line in lines {
        let (name, value) = line.split_once(':').ok_or("chain RPC header invalid")?;
        if headers
            .insert(name.to_ascii_lowercase(), value.trim())
            .is_some()
        {
            return Err("chain RPC duplicate header".to_owned());
        }
    }
    if headers.contains_key("transfer-encoding") {
        return Err("chain RPC transfer encoding refused".to_owned());
    }
    match headers.get("content-length") {
        Some(value) => value
            .parse::<usize>()
            .ok()
            .filter(|length| *length <= MAX_RESPONSE_BYTES)
            .map(Some)
            .ok_or("chain RPC content length invalid".to_owned()),
        None => Ok(None),
    }
}

fn base64_key(text: &str) -> Option<[u8; 32]> {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let bytes = text.as_bytes();
    if bytes.len() != 44 || bytes[43] != b'=' {
        return None;
    }
    let mut output = Vec::with_capacity(32);
    for (index, chunk) in bytes.chunks_exact(4).enumerate() {
        let mut value = 0_u32;
        for byte in &chunk[..if index == 10 { 3 } else { 4 }] {
            value = (value << 6)
                | u32::try_from(ALPHABET.iter().position(|item| item == byte)?).ok()?;
        }
        if index == 10 {
            if value & 3 != 0 {
                return None;
            }
            output.extend_from_slice(&[
                (value >> 10).to_le_bytes()[0],
                (value >> 2).to_le_bytes()[0],
            ]);
        } else {
            output.extend_from_slice(&[
                (value >> 16).to_le_bytes()[0],
                (value >> 8).to_le_bytes()[0],
                value.to_le_bytes()[0],
            ]);
        }
    }
    output.try_into().ok()
}
