use crate::{chain_receipt::ChainReceipts, hex, AuthorityError, AuthorityResult};
use layerx_client::sequencer::SequencerEndpoint;
use serde_json::{json, Value};
use std::collections::BTreeMap;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::time::{Duration, Instant};

// Keep the strict ICS23 implementation identical to the registry verifier until
// layerx-client exposes its generic per-store verifier (see repair_22_31_207.kvx).
mod ics23;
mod multistore;
mod proto;

#[derive(Clone, Copy, Debug)]
enum ChainProofError {
    Truncated,
    NonCanonical,
    UnknownField,
    Mismatch,
}

pub const EXPECTED_CHAIN_ID: u64 = 125;

#[derive(Clone)]
pub struct CommittedHeader {
    pub height: u64,
    pub block_hash: [u8; 32],
    pub app_hash: [u8; 32],
    pub state_height: u64,
    rpc_url: String,
}

impl CommittedHeader {
    /// # Errors
    /// Returns an error when the committed header cannot be read or verified.
    pub fn open(rpc_url: &str) -> AuthorityResult<Self> {
        let endpoint = SequencerEndpoint::new(rpc_url, &[]).map_err(|_| AuthorityError)?;
        if endpoint.chain_id().map_err(|_| AuthorityError)? != EXPECTED_CHAIN_ID {
            return Err(AuthorityError);
        }
        let height = endpoint.block_number().map_err(|_| AuthorityError)?;
        let block = endpoint
            .block_by_number(height)
            .map_err(|_| AuthorityError)?
            .ok_or(AuthorityError)?;
        let app_hash = endpoint.state_root(height).map_err(|_| AuthorityError)?;
        // The configured loopback node is the authority for committed BFT headers.
        // Pin its Comet commit and EVM projection to the same hash and app root.
        let material = endpoint
            .commit_material(height)
            .map_err(|_| AuthorityError)?;
        let header = &material["signedHeader"]["header"];
        let commit = &material["signedHeader"]["commit"];
        let hash = |value: &Value| {
            hex::decode32(value.as_str().ok_or(AuthorityError)?).map_err(|_| AuthorityError)
        };
        if height <= 1
            || block.hash == [0; 32]
            || app_hash == [0; 32]
            || hash(&header["app_hash"])? != app_hash
            || hash(&commit["block_id"]["hash"])? != block.hash
            || header["chain_id"].as_str().is_none_or(str::is_empty)
        {
            return Err(AuthorityError);
        }
        let pinned = Self {
            height,
            block_hash: block.hash,
            app_hash,
            // A Comet header at H authenticates the state committed at H-1.
            state_height: height.checked_sub(1).ok_or(AuthorityError)?,
            rpc_url: rpc_url.to_owned(),
        };
        pinned.unchanged()?;
        Ok(pinned)
    }

    /// # Errors
    /// Returns an error when the receipt is not bound to the committed header.
    pub fn bind_receipt(
        &self,
        rpc_url: &str,
        chain_receipts: &ChainReceipts,
        bytes: &[u8],
    ) -> AuthorityResult<()> {
        let locator = crate::receipt_locator(bytes).map_err(|_| AuthorityError)?;
        let endpoint = SequencerEndpoint::new(rpc_url, &[]).map_err(|_| AuthorityError)?;
        let (receipt, block_number, block_hash) = chain_receipts
            .committed(locator.activity_id)
            .map_err(|_| AuthorityError)?;
        if receipt != bytes || block_number == 0 || block_number > self.state_height {
            return Err(AuthorityError);
        }
        let block = endpoint
            .block_by_number(block_number)
            .map_err(|_| AuthorityError)?
            .ok_or(AuthorityError)?;
        if block.hash != block_hash || block_hash == [0; 32] {
            return Err(AuthorityError);
        }
        self.unchanged()
    }

    /// # Errors
    /// Returns an error when the store proof does not match the committed root.
    pub fn prove(&self, module: u16, key: &[u8], value: &[u8]) -> AuthorityResult<()> {
        if module == 0 || key.is_empty() || key.len() > 129 || value.is_empty() {
            return Err(AuthorityError);
        }
        let mut store_key = vec![1];
        store_key.extend_from_slice(&module.to_be_bytes());
        store_key.extend_from_slice(key);
        let reply = query(&self.rpc_url, &store_key, self.state_height)?;
        if reply["code"].as_u64() != Some(0)
            || reply["height"]
                .as_str()
                .and_then(|height| height.parse::<u64>().ok())
                != Some(self.state_height)
            || base64(reply["key"].as_str().ok_or(AuthorityError)?)? != store_key
            || base64(reply["value"].as_str().ok_or(AuthorityError)?)? != value
        {
            return Err(AuthorityError);
        }
        let ops = reply["proofOps"]["ops"].as_array().ok_or(AuthorityError)?;
        if ops.len() != 2
            || ops[0]["type"] != "ics23:iavl"
            || ops[1]["type"] != "ics23:simple"
            || base64(ops[0]["key"].as_str().ok_or(AuthorityError)?)? != store_key
            || base64(ops[1]["key"].as_str().ok_or(AuthorityError)?)? != b"lxphost"
        {
            return Err(AuthorityError);
        }
        let inner = base64(ops[0]["data"].as_str().ok_or(AuthorityError)?)?;
        let outer = base64(ops[1]["data"].as_str().ok_or(AuthorityError)?)?;
        verify_membership(&store_key, value, &inner, &outer, self.app_hash)?;
        self.unchanged()
    }

    fn unchanged(&self) -> AuthorityResult<()> {
        let endpoint = SequencerEndpoint::new(&self.rpc_url, &[]).map_err(|_| AuthorityError)?;
        let block = endpoint
            .block_by_number(self.height)
            .map_err(|_| AuthorityError)?
            .ok_or(AuthorityError)?;
        if block.hash != self.block_hash
            || endpoint
                .state_root(self.height)
                .map_err(|_| AuthorityError)?
                != self.app_hash
        {
            return Err(AuthorityError);
        }
        Ok(())
    }

    #[must_use]
    pub fn document(&self) -> Value {
        json!({"height": self.height, "block_hash": hex::encode(&self.block_hash),
            "app_hash": hex::encode(&self.app_hash), "state_height": self.state_height})
    }
}

/// # Errors
/// Returns an error when either proof does not establish membership in the app root.
pub fn verify_membership(
    key: &[u8],
    value: &[u8],
    inner: &[u8],
    outer: &[u8],
    app_hash: [u8; 32],
) -> AuthorityResult<()> {
    fn existence(data: &[u8]) -> AuthorityResult<ics23::ExistenceProof> {
        let mut reader = proto::ProtoReader::new(data);
        let field = reader
            .next_field()
            .map_err(|_| AuthorityError)?
            .ok_or(AuthorityError)?;
        let (1, proto::ProtoValue::Bytes(bytes)) = (field.number, field.value) else {
            return Err(AuthorityError);
        };
        reader.finish().map_err(|_| AuthorityError)?;
        ics23::ExistenceProof::decode(bytes).map_err(|_| AuthorityError)
    }
    let root: [u8; 32] = existence(outer)?
        .value
        .try_into()
        .map_err(|_| AuthorityError)?;
    ics23::verify_existence(
        &existence(inner)?,
        &ics23::ProofSpec::iavl(),
        &root,
        key,
        value,
    )
    .map_err(|_| AuthorityError)?;
    multistore::verify_store_root("lxphost", root, outer, app_hash).map_err(|_| AuthorityError)
}

/// # Errors
/// Returns an error when the text is not canonical bounded base64.
pub fn base64(text: &str) -> AuthorityResult<Vec<u8>> {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    if text.is_empty() || !text.len().is_multiple_of(4) || text.len() > 2 * 1024 * 1024 {
        return Err(AuthorityError);
    }
    let mut output = Vec::new();
    for (index, chunk) in text.as_bytes().chunks_exact(4).enumerate() {
        let last = index == text.len() / 4 - 1;
        let padding = if chunk[2] == b'=' {
            2
        } else {
            usize::from(chunk[3] == b'=')
        };
        if padding != 0 && (!last || chunk[3] != b'=') {
            return Err(AuthorityError);
        }
        let mut value = 0_u32;
        for byte in &chunk[..4 - padding] {
            value = (value << 6)
                | u32::try_from(
                    ALPHABET
                        .iter()
                        .position(|entry| entry == byte)
                        .ok_or(AuthorityError)?,
                )
                .map_err(|_| AuthorityError)?;
        }
        if padding == 2 {
            if value & 15 != 0 {
                return Err(AuthorityError);
            }
            output.push((value >> 4).to_le_bytes()[0]);
        } else if padding == 1 {
            if value & 3 != 0 {
                return Err(AuthorityError);
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
    Ok(output)
}

fn query(url: &str, key: &[u8], height: u64) -> AuthorityResult<Value> {
    let base = url
        .trim_end_matches('/')
        .strip_suffix("/evm")
        .unwrap_or(url.trim_end_matches('/'));
    let (host, address) = super::loopback_http(base).ok_or(AuthorityError)?;
    let body = json!({"jsonrpc":"2.0", "id":1, "method":"abci_query", "params":{
        "path":"/store/lxphost/key", "data":format!("0x{}",hex::encode(key)),
        "height":height.to_string(), "prove":true}})
    .to_string();
    let deadline = Instant::now() + Duration::from_secs(8);
    let remaining = || {
        deadline
            .checked_duration_since(Instant::now())
            .filter(|d| !d.is_zero())
            .ok_or(AuthorityError)
    };
    let mut stream =
        TcpStream::connect_timeout(&address, remaining()?).map_err(|_| AuthorityError)?;
    stream
        .set_write_timeout(Some(remaining()?))
        .map_err(|_| AuthorityError)?;
    write!(stream, "POST /comet HTTP/1.1\r\nHost: {host}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).map_err(|_| AuthorityError)?;
    let mut raw = Vec::new();
    let mut chunk = [0; 4096];
    loop {
        stream
            .set_read_timeout(Some(remaining()?))
            .map_err(|_| AuthorityError)?;
        let count = stream.read(&mut chunk).map_err(|_| AuthorityError)?;
        if count == 0 {
            break;
        }
        raw.extend_from_slice(&chunk[..count]);
        if raw.len() > 2 * 1024 * 1024 {
            return Err(AuthorityError);
        }
    }
    let end = raw
        .windows(4)
        .position(|s| s == b"\r\n\r\n")
        .filter(|n| *n <= 32768)
        .ok_or(AuthorityError)?;
    let mut lines = std::str::from_utf8(&raw[..end])
        .map_err(|_| AuthorityError)?
        .split("\r\n");
    if lines.next().and_then(|s| s.split_whitespace().nth(1)) != Some("200") {
        return Err(AuthorityError);
    }
    let mut headers = BTreeMap::new();
    for line in lines {
        let (name, value) = line.split_once(':').ok_or(AuthorityError)?;
        if headers
            .insert(name.to_ascii_lowercase(), value.trim())
            .is_some()
        {
            return Err(AuthorityError);
        }
    }
    let body = &raw[end + 4..];
    if headers.contains_key("transfer-encoding")
        || headers
            .get("content-length")
            .and_then(|s| s.parse::<usize>().ok())
            != Some(body.len())
    {
        return Err(AuthorityError);
    }
    let document: Value = serde_json::from_slice(body).map_err(|_| AuthorityError)?;
    if document["jsonrpc"] != "2.0" || document["id"] != 1 || document.get("error").is_some() {
        return Err(AuthorityError);
    }
    document["result"]
        .get("response")
        .cloned()
        .ok_or(AuthorityError)
}
