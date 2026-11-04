use super::{hex, Config};
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

#[derive(Clone)]
pub(crate) struct CommittedHeader {
    pub height: u64,
    pub block_hash: [u8; 32],
    pub app_hash: [u8; 32],
    pub state_height: u64,
    rpc_url: String,
}

impl CommittedHeader {
    pub(crate) fn open(config: &Config) -> Result<Self, ()> {
        let endpoint = SequencerEndpoint::new(&config.chain_rpc_url, &[]).map_err(|_| ())?;
        if endpoint.chain_id().map_err(|_| ())? != super::trust::EXPECTED_CHAIN_ID {
            return Err(());
        }
        let height = endpoint.block_number().map_err(|_| ())?;
        let block = endpoint
            .block_by_number(height)
            .map_err(|_| ())?
            .ok_or(())?;
        let app_hash = endpoint.state_root(height).map_err(|_| ())?;
        // The configured loopback node is the authority for committed BFT headers.
        // Pin its Comet commit and EVM projection to the same hash and app root.
        let material = endpoint.commit_material(height).map_err(|_| ())?;
        let header = &material["signedHeader"]["header"];
        let commit = &material["signedHeader"]["commit"];
        let hash = |value: &Value| hex::decode32(value.as_str().ok_or(())?).map_err(|_| ());
        if height <= 1
            || block.hash == [0; 32]
            || app_hash == [0; 32]
            || hash(&header["app_hash"])? != app_hash
            || hash(&commit["block_id"]["hash"])? != block.hash
            || header["chain_id"].as_str().is_none_or(str::is_empty)
        {
            return Err(());
        }
        let pinned = Self {
            height,
            block_hash: block.hash,
            app_hash,
            // A Comet header at H authenticates the state committed at H-1.
            state_height: height.checked_sub(1).ok_or(())?,
            rpc_url: config.chain_rpc_url.clone(),
        };
        pinned.unchanged()?;
        Ok(pinned)
    }

    pub(crate) fn bind_receipt(&self, config: &Config, bytes: &[u8]) -> Result<(), ()> {
        let locator = layerx_platform_authority::receipt_locator(bytes).map_err(|_| ())?;
        let endpoint = SequencerEndpoint::new(&config.chain_rpc_url, &[]).map_err(|_| ())?;
        let (receipt, block_number, block_hash) = config
            .chain_receipts
            .committed(locator.activity_id)
            .map_err(|_| ())?;
        if receipt != bytes || block_number == 0 || block_number > self.state_height {
            return Err(());
        }
        let block = endpoint
            .block_by_number(block_number)
            .map_err(|_| ())?
            .ok_or(())?;
        if block.hash != block_hash || block_hash == [0; 32] {
            return Err(());
        }
        self.unchanged()
    }

    pub(crate) fn prove(&self, module: u16, key: &[u8], value: &[u8]) -> Result<(), ()> {
        if module == 0 || key.is_empty() || key.len() > 129 || value.is_empty() {
            return Err(());
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
            || base64(reply["key"].as_str().ok_or(())?)? != store_key
            || base64(reply["value"].as_str().ok_or(())?)? != value
        {
            return Err(());
        }
        let ops = reply["proofOps"]["ops"].as_array().ok_or(())?;
        if ops.len() != 2
            || ops[0]["type"] != "ics23:iavl"
            || ops[1]["type"] != "ics23:simple"
            || base64(ops[0]["key"].as_str().ok_or(())?)? != store_key
            || base64(ops[1]["key"].as_str().ok_or(())?)? != b"lxphost"
        {
            return Err(());
        }
        let inner = base64(ops[0]["data"].as_str().ok_or(())?)?;
        let outer = base64(ops[1]["data"].as_str().ok_or(())?)?;
        verify_membership(&store_key, value, &inner, &outer, self.app_hash)?;
        self.unchanged()
    }

    fn unchanged(&self) -> Result<(), ()> {
        let endpoint = SequencerEndpoint::new(&self.rpc_url, &[]).map_err(|_| ())?;
        let block = endpoint
            .block_by_number(self.height)
            .map_err(|_| ())?
            .ok_or(())?;
        if block.hash != self.block_hash
            || endpoint.state_root(self.height).map_err(|_| ())? != self.app_hash
        {
            return Err(());
        }
        Ok(())
    }

    pub(crate) fn document(&self) -> Value {
        json!({"height": self.height, "block_hash": hex::encode(&self.block_hash),
            "app_hash": hex::encode(&self.app_hash), "state_height": self.state_height})
    }
}

pub(crate) fn verify_membership(
    key: &[u8],
    value: &[u8],
    inner: &[u8],
    outer: &[u8],
    app_hash: [u8; 32],
) -> Result<(), ()> {
    fn existence(data: &[u8]) -> Result<ics23::ExistenceProof, ()> {
        let mut reader = proto::ProtoReader::new(data);
        let field = reader.next_field().map_err(|_| ())?.ok_or(())?;
        let bytes = match (field.number, field.value) {
            (1, proto::ProtoValue::Bytes(bytes)) => bytes,
            _ => return Err(()),
        };
        reader.finish().map_err(|_| ())?;
        ics23::ExistenceProof::decode(bytes).map_err(|_| ())
    }
    let root: [u8; 32] = existence(outer)?.value.try_into().map_err(|_| ())?;
    ics23::verify_existence(
        &existence(inner)?,
        &ics23::ProofSpec::iavl(),
        &root,
        key,
        value,
    )
    .map_err(|_| ())?;
    multistore::verify_store_root("lxphost", root, outer, app_hash).map_err(|_| ())
}

pub(crate) fn base64(text: &str) -> Result<Vec<u8>, ()> {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    if text.is_empty() || !text.len().is_multiple_of(4) || text.len() > 2 * 1024 * 1024 {
        return Err(());
    }
    let mut output = Vec::new();
    for (index, chunk) in text.as_bytes().chunks_exact(4).enumerate() {
        let last = index == text.len() / 4 - 1;
        let padding = if chunk[2] == b'=' {
            2
        } else if chunk[3] == b'=' {
            1
        } else {
            0
        };
        if padding != 0 && (!last || chunk[3] != b'=') {
            return Err(());
        }
        let mut value = 0_u32;
        for byte in &chunk[..4 - padding] {
            value = (value << 6)
                | u32::try_from(ALPHABET.iter().position(|entry| entry == byte).ok_or(())?)
                    .map_err(|_| ())?;
        }
        if padding == 2 {
            if value & 15 != 0 {
                return Err(());
            }
            output.push((value >> 4) as u8);
        } else if padding == 1 {
            if value & 3 != 0 {
                return Err(());
            }
            output.extend_from_slice(&[(value >> 10) as u8, (value >> 2) as u8]);
        } else {
            output.extend_from_slice(&[(value >> 16) as u8, (value >> 8) as u8, value as u8]);
        }
    }
    Ok(output)
}

fn query(url: &str, key: &[u8], height: u64) -> Result<Value, ()> {
    let base = url
        .trim_end_matches('/')
        .strip_suffix("/evm")
        .unwrap_or(url.trim_end_matches('/'));
    let (host, address) = super::loopback_http(base).ok_or(())?;
    let body = json!({"jsonrpc":"2.0", "id":1, "method":"abci_query", "params":{
        "path":"/store/lxphost/key", "data":format!("0x{}",hex::encode(key)),
        "height":height.to_string(), "prove":true}})
    .to_string();
    let deadline = Instant::now() + Duration::from_secs(8);
    let remaining = || {
        deadline
            .checked_duration_since(Instant::now())
            .filter(|d| !d.is_zero())
            .ok_or(())
    };
    let mut stream = TcpStream::connect_timeout(&address, remaining()?).map_err(|_| ())?;
    stream
        .set_write_timeout(Some(remaining()?))
        .map_err(|_| ())?;
    write!(stream, "POST /comet HTTP/1.1\r\nHost: {host}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).map_err(|_| ())?;
    let mut raw = Vec::new();
    let mut chunk = [0; 4096];
    loop {
        stream
            .set_read_timeout(Some(remaining()?))
            .map_err(|_| ())?;
        let count = stream.read(&mut chunk).map_err(|_| ())?;
        if count == 0 {
            break;
        }
        raw.extend_from_slice(&chunk[..count]);
        if raw.len() > 2 * 1024 * 1024 {
            return Err(());
        }
    }
    let end = raw
        .windows(4)
        .position(|s| s == b"\r\n\r\n")
        .filter(|n| *n <= 32768)
        .ok_or(())?;
    let mut lines = std::str::from_utf8(&raw[..end])
        .map_err(|_| ())?
        .split("\r\n");
    if lines.next().and_then(|s| s.split_whitespace().nth(1)) != Some("200") {
        return Err(());
    }
    let mut headers = BTreeMap::new();
    for line in lines {
        let (name, value) = line.split_once(':').ok_or(())?;
        if headers
            .insert(name.to_ascii_lowercase(), value.trim())
            .is_some()
        {
            return Err(());
        }
    }
    let body = &raw[end + 4..];
    if headers.contains_key("transfer-encoding")
        || headers
            .get("content-length")
            .and_then(|s| s.parse::<usize>().ok())
            != Some(body.len())
    {
        return Err(());
    }
    let document: Value = serde_json::from_slice(body).map_err(|_| ())?;
    if document["jsonrpc"] != "2.0" || document["id"] != 1 || document.get("error").is_some() {
        return Err(());
    }
    document["result"].get("response").cloned().ok_or(())
}
