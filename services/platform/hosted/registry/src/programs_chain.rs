use std::io::Read as _;
use std::time::{Duration, Instant};

use layerx_crypto::evm_transaction::transaction_hash as keccak256;
use layerx_programs::hex;
use serde::{Deserialize, Serialize};
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

#[derive(Debug)]
pub enum DeploymentReadError {
    Unavailable(String),
    Refused(String),
}

#[derive(Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct ChainDeployment {
    pub transaction_hash: String,
    pub block_hash: String,
    pub block_number: u64,
    pub program_id: String,
    pub code_hash: String,
    pub abi_version: u16,
    pub version: u32,
    pub policy: u8,
    pub authority: String,
    pub active: bool,
}

impl ChainDeployment {
    pub fn journal(&self, root: &std::path::Path) -> Result<(), String> {
        use nix::fcntl::{Flock, FlockArg};
        use std::fs::{self, OpenOptions};
        use std::os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _};

        let root = root.join("chain-deployments");
        fs::DirBuilder::new()
            .recursive(true)
            .mode(0o700)
            .create(&root)
            .map_err(|error| error.to_string())?;
        let lock = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .mode(0o600)
            .custom_flags(nix::libc::O_NOFOLLOW)
            .open(root.join(".lock"))
            .map_err(|error| error.to_string())?;
        let _guard =
            Flock::lock(lock, FlockArg::LockExclusive).map_err(|(_, error)| error.to_string())?;
        let path = root.join(format!("{}.json", self.transaction_hash));
        match fs::read(&path) {
            Ok(bytes) => {
                let stored: Self = serde_json::from_slice(&bytes)
                    .map_err(|error| format!("invalid chain deployment journal: {error}"))?;
                if stored != *self {
                    return Err("chain deployment differs from its journaled record".to_owned());
                }
                Ok(())
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                let bytes = serde_json::to_vec(self).map_err(|error| error.to_string())?;
                crate::write_atomic(&path, &bytes).map_err(|error| error.to_string())
            }
            Err(error) => Err(error.to_string()),
        }
    }
}

impl ProgramsChain {
    pub fn deployment(
        &self,
        transaction: [u8; 32],
        deadline: Instant,
    ) -> Result<ChainDeployment, DeploymentReadError> {
        use DeploymentReadError::{Refused, Unavailable};

        let deadline = deadline.min(Instant::now() + READ_TIMEOUT);
        let receipt = self
            .rpc(
                "eth_getTransactionReceipt",
                json!([format!("0x{}", hex::encode(&transaction))]),
                deadline,
            )
            .map_err(Unavailable)?;
        let refused = |error: &str| Refused(error.to_owned());
        if receipt.is_null() || quantity(&receipt["status"]).map_err(Refused)? != 1 {
            return Err(refused("deployment transaction has no successful receipt"));
        }
        if digest(&receipt["transactionHash"]).map_err(Refused)? != transaction {
            return Err(refused("deployment receipt belongs to another transaction"));
        }
        let block_hash = digest(&receipt["blockHash"]).map_err(Refused)?;
        let block_number = quantity(&receipt["blockNumber"]).map_err(Refused)?;
        if block_hash == [0; 32] || block_number == 0 {
            return Err(refused("deployment receipt is not committed in a block"));
        }
        let event = keccak256(b"ProgramDeployed(bytes32,bytes32)");
        let logs = receipt["logs"]
            .as_array()
            .ok_or_else(|| refused("deployment receipt has no logs"))?;
        let mut deployed = None;
        for log in logs {
            if !log["address"]
                .as_str()
                .is_some_and(|address| address.eq_ignore_ascii_case(PROGRAMS))
                || log["topics"]
                    .as_array()
                    .and_then(|topics| topics.first())
                    .and_then(|topic| digest(topic).ok())
                    != Some(event)
            {
                continue;
            }
            if deployed.is_some()
                || log["topics"].as_array().map(Vec::len) != Some(2)
                || log["removed"] == true
                || digest(&log["transactionHash"]).map_err(Refused)? != transaction
                || digest(&log["blockHash"]).map_err(Refused)? != block_hash
                || quantity(&log["blockNumber"]).map_err(Refused)? != block_number
            {
                return Err(refused("ambiguous or inconsistent deployment event"));
            }
            let program = digest(&log["topics"][1]).map_err(Refused)?;
            let code_hash = digest(&log["data"]).map_err(Refused)?;
            if program == [0; 32] || code_hash == [0; 32] {
                return Err(refused("deployment event has an empty identity"));
            }
            deployed = Some((program, code_hash));
        }
        let (program, deployed_hash) =
            deployed.ok_or_else(|| refused("transaction has no Programs deployment event"))?;
        let record = self.view_at(
            "getProgram(bytes32)",
            program,
            json!({"blockHash": format!("0x{}", hex::encode(&block_hash)), "requireCanonical": true}),
            deadline,
        ).map_err(Unavailable)?;
        if record.len() != 192 {
            return Err(refused("invalid getProgram ABI record length"));
        }
        let policy = u8::try_from(word(&record[..32], 1).map_err(Refused)?)
            .map_err(|_| refused("invalid program policy"))?;
        let authority = &record[32..64];
        let code_hash = &record[64..96];
        let abi_version = u16::try_from(word(&record[96..128], 2).map_err(Refused)?)
            .map_err(|_| refused("invalid program ABI version"))?;
        let version = u32::try_from(word(&record[128..160], 4).map_err(Refused)?)
            .map_err(|_| refused("invalid program version"))?;
        let active = word(&record[160..192], 1).map_err(Refused)?;
        if policy > 1
            || (policy == 0) != authority.iter().all(|byte| *byte == 0)
            || abi_version == 0
            || version == 0
            || active != 1
            || code_hash != deployed_hash.as_slice()
        {
            return Err(refused(
                "getProgram record does not match an active deployment",
            ));
        }
        Ok(ChainDeployment {
            transaction_hash: hex::encode(&transaction),
            block_hash: hex::encode(&block_hash),
            block_number,
            program_id: hex::encode(&program),
            code_hash: hex::encode(code_hash),
            abi_version,
            version,
            policy,
            authority: hex::encode(authority),
            active: true,
        })
    }

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
        self.view_at(signature, program, json!(tag), deadline)
    }

    fn view_at(
        &self,
        signature: &str,
        program: [u8; 32],
        tag: Value,
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

fn digest(value: &Value) -> Result<[u8; 32], String> {
    let text = value
        .as_str()
        .and_then(|text| text.strip_prefix("0x"))
        .ok_or("programs chain digest must be hexadecimal bytes32")?;
    hex::decode_digest(text).map_err(|error| format!("programs chain digest: {error}"))
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
