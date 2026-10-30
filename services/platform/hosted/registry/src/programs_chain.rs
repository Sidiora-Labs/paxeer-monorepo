use std::io::Read as _;
use std::time::{Duration, Instant};

use layerx_crypto::evm_transaction::transaction_hash as keccak256;
use layerx_programs::hex;
use serde_json::{json, Value};

const PROGRAMS: &str = "0x000000000000000000000000000000000000101d";
const MAX_RESPONSE_BYTES: u64 = 2 * 1024 * 1024 + 1024;
const MAX_STATE_BYTES: usize = 1024 * 1024;
const READ_TIMEOUT: Duration = Duration::from_secs(15);

pub struct ProgramsChain {
    endpoint: String,
    agent: ureq::Agent,
}

pub struct ProgramHead {
    pub sequence: u64,
    pub observed_at: u64,
    pub state_root: [u8; 32],
}

impl ProgramsChain {
    pub fn new(endpoint: &str) -> Result<Self, String> {
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
        let config = ureq::Agent::config_builder()
            .max_redirects(0)
            .timeout_global(Some(READ_TIMEOUT))
            .build();
        Ok(Self {
            endpoint: endpoint.to_owned(),
            agent: config.into(),
        })
    }

    pub fn head(&self, program: [u8; 32], deadline: Instant) -> Result<ProgramHead, String> {
        let deadline = deadline.min(Instant::now() + READ_TIMEOUT);
        let sequence = quantity(&self.rpc("eth_blockNumber", json!([]), deadline)?)?;
        if sequence == 0 {
            return Err("programs chain has no committed head".to_owned());
        }
        let tag = format!("0x{sequence:x}");
        let block = self.rpc("eth_getBlockByNumber", json!([tag, false]), deadline)?;
        if quantity(&block["number"])? != sequence {
            return Err("programs chain returned a different block height".to_owned());
        }
        let observed_at = quantity(&block["timestamp"])?
            .checked_mul(1000)
            .filter(|time| *time != 0)
            .ok_or("invalid programs chain block time")?;
        let record = self.view("getProgram(bytes32)", program, &tag, deadline)?;
        if record.len() != 192 {
            return Err("invalid getProgram ABI record length".to_owned());
        }
        word(&record[..32], 1)?;
        word(&record[96..128], 2)?;
        let version = word(&record[128..160], 4)?;
        let active = word(&record[160..192], 1)?;
        if version == 0 || active > 1 || record[64..96].iter().all(|byte| *byte == 0) {
            return Err("invalid or absent programs chain program record".to_owned());
        }
        let encoded = self.view("getStateHead(bytes32)", program, &tag, deadline)?;
        if encoded.len() < 64 || word(&encoded[..32], 8)? != 32 {
            return Err("invalid getStateHead ABI offset".to_owned());
        }
        let length = usize::try_from(word(&encoded[32..64], 8)?)
            .map_err(|_| "invalid getStateHead length")?;
        if length == 0 || length > MAX_STATE_BYTES {
            return Err("getStateHead record exceeds its bounds".to_owned());
        }
        let padded = length.div_ceil(32) * 32;
        if encoded.len() != 64 + padded || encoded[64 + length..].iter().any(|byte| *byte != 0) {
            return Err("noncanonical getStateHead ABI bytes".to_owned());
        }
        Ok(ProgramHead {
            sequence,
            observed_at,
            state_root: keccak256(&encoded[64..64 + length]),
        })
    }

    fn view(
        &self,
        signature: &str,
        program: [u8; 32],
        tag: &str,
        deadline: Instant,
    ) -> Result<Vec<u8>, String> {
        let selector = keccak256(signature.as_bytes());
        let data = format!("0x{}{}", hex::encode(&selector[..4]), hex::encode(&program));
        let result = self.rpc(
            "eth_call",
            json!([{"to": PROGRAMS, "data": data}, tag]),
            deadline,
        )?;
        let text = result
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .ok_or("programs chain call did not return hex bytes")?;
        hex::decode(text).map_err(|error| format!("programs chain call: {error}"))
    }

    fn rpc(&self, method: &str, params: Value, deadline: Instant) -> Result<Value, String> {
        let remaining = deadline
            .checked_duration_since(Instant::now())
            .filter(|remaining| !remaining.is_zero())
            .ok_or("programs chain read deadline expired")?;
        let mut response = self
            .agent
            .post(&self.endpoint)
            .config()
            .timeout_global(Some(remaining))
            .build()
            .send_json(json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}))
            .map_err(|error| format!("programs chain {method}: {error}"))?;
        let mut body = Vec::new();
        response
            .body_mut()
            .as_reader()
            .take(MAX_RESPONSE_BYTES + 1)
            .read_to_end(&mut body)
            .map_err(|error| format!("programs chain {method} response: {error}"))?;
        if body.len() as u64 > MAX_RESPONSE_BYTES || Instant::now() >= deadline {
            return Err("programs chain response exceeded its byte or time bound".to_owned());
        }
        let document: Value = serde_json::from_slice(&body)
            .map_err(|error| format!("programs chain {method} JSON: {error}"))?;
        if document["jsonrpc"] != "2.0" || document["id"] != 1 || document.get("error").is_some() {
            return Err(format!(
                "programs chain {method} refused or returned an invalid RPC envelope"
            ));
        }
        document
            .get("result")
            .cloned()
            .ok_or_else(|| format!("programs chain {method} omitted result"))
    }
}

fn quantity(value: &Value) -> Result<u64, String> {
    let text = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .ok_or("invalid programs chain quantity")?;
    if text.is_empty() || (text.len() > 1 && text.starts_with('0')) {
        return Err("noncanonical programs chain quantity".to_owned());
    }
    u64::from_str_radix(text, 16).map_err(|_| "programs chain quantity exceeds u64".to_owned())
}

fn word(bytes: &[u8], width: usize) -> Result<u64, String> {
    if bytes.len() != 32 || bytes[..32 - width].iter().any(|byte| *byte != 0) {
        return Err("programs chain ABI integer exceeds its type".to_owned());
    }
    Ok(bytes[32 - width..]
        .iter()
        .fold(0, |value, byte| (value << 8) | u64::from(*byte)))
}
