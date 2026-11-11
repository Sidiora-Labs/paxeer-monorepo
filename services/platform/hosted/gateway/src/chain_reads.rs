use super::{public_reads, rpc, Config, Endpoint};
use layerx_platform_gateway::http::OutboundRequest;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::collections::BTreeSet;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::{Duration, Instant};

// consensus/types/vote_set.go: MaxVotesCount also bounds a valid validator set.
const MAX_VALIDATORS: u64 = 10_000;
const VALIDATORS_PER_PAGE: u64 = 100;
const MAX_VALIDATOR_PAGES: u64 = MAX_VALIDATORS / VALIDATORS_PER_PAGE;
const VALIDATORS_DEADLINE: Duration = Duration::from_secs(5);
const MAX_VALIDATOR_READS: usize = 16;
static VALIDATOR_READS: AtomicUsize = AtomicUsize::new(0);

struct ValidatorRead;
impl Drop for ValidatorRead {
    fn drop(&mut self) {
        VALIDATOR_READS.fetch_sub(1, Ordering::AcqRel);
    }
}

fn unavailable(id: &Value, code: &str) -> Value {
    let mut answer = rpc::error(id, -32003, "Chain read unavailable");
    answer["error"]["data"] = json!({"code": code});
    answer
}

fn decimal(value: &Value) -> Option<u64> {
    let text = value.as_str()?;
    if text.is_empty() || !text.bytes().all(|byte| byte.is_ascii_digit()) {
        return None;
    }
    text.parse().ok()
}

fn height(params: Option<&Value>) -> Option<u64> {
    let [value] = params?.as_array()?.as_slice() else {
        return None;
    };
    decimal(value).filter(|height| *height > 0)
}

pub(super) fn consensus(config: &Config, method: &str, id: &Value, params: Value) -> Option<Value> {
    let mut endpoint = std::env::var("LAYERX_GATEWAY_CONSENSUS_RPC_URL")
        .ok()
        .and_then(|url| Endpoint::parse(&url).ok())?;
    consensus_request(&config.client, &mut endpoint, method, id, params)
}

fn consensus_request(
    client: &layerx_platform_gateway::http::Client,
    endpoint: &mut Endpoint,
    method: &str,
    id: &Value,
    params: Value,
) -> Option<Value> {
    let path = if endpoint.base_path.is_empty() {
        "/".to_owned()
    } else {
        endpoint.base_path.clone()
    };
    let body = serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": id, "method": method, "params": params
    }))
    .ok()?;
    let mut target = endpoint.clone();
    target.base_path.clear();
    let response = client
        .request_unauthenticated(
            &target,
            &OutboundRequest {
                method: "POST",
                path: &path,
                idempotency: None,
                content_type: "application/json",
                body: &body,
            },
        )
        .ok()?;
    if response.status != 200 || response.content_type != "application/json" {
        return None;
    }
    let document: Value = serde_json::from_slice(&response.body).ok()?;
    if document["jsonrpc"] != "2.0"
        || document.get("id") != Some(id)
        || document.get("error").is_some()
    {
        return None;
    }
    document
        .get("result")
        .filter(|result| result.is_object())
        .cloned()
}

#[derive(Debug, Deserialize, Serialize)]
pub(super) struct ConsensusHead {
    #[serde(rename(deserialize = "latest_block_height"))]
    latest_height: String,
    latest_block_hash: String,
    latest_app_hash: String,
    catching_up: bool,
}

impl ConsensusHead {
    pub(super) fn from_status(status: &Value) -> Option<Self> {
        let head: Self = serde_json::from_value(status.get("sync_info")?.clone()).ok()?;
        decimal(&Value::String(head.latest_height.clone()))?;
        for hash in [&head.latest_block_hash, &head.latest_app_hash] {
            if hash.len() % 2 != 0 || !hash.bytes().all(|byte| byte.is_ascii_hexdigit()) {
                return None;
            }
        }
        Some(head)
    }
}

pub(super) fn consensus_head(config: &Config, id: &Value) -> Option<ConsensusHead> {
    ConsensusHead::from_status(&consensus(config, "status", id, json!({}))?)
}

#[allow(non_snake_case)]
fn px_getCommit(config: &Config, id: &Value, height: u64) -> Option<Value> {
    let result = consensus(config, "commit", id, json!({"height": height.to_string()}))?;
    let signed_header = result
        .get("signed_header")
        .filter(|header| header.is_object())?;
    if decimal(&signed_header["header"]["height"]) != Some(height)
        || !signed_header["commit"].is_object()
    {
        return None;
    }
    Some(json!({"height": height.to_string(), "signedHeader": signed_header}))
}

#[allow(non_snake_case)]
fn px_getValidators(config: &Config, id: &Value, height: u64) -> Option<Value> {
    let deadline = Instant::now().checked_add(VALIDATORS_DEADLINE)?;
    let mut endpoint = std::env::var("LAYERX_GATEWAY_CONSENSUS_RPC_URL")
        .ok()
        .and_then(|url| Endpoint::parse(&url).ok())?;
    VALIDATOR_READS
        .fetch_update(Ordering::AcqRel, Ordering::Acquire, |active| {
            (active < MAX_VALIDATOR_READS).then_some(active + 1)
        })
        .ok()?;
    let permit = ValidatorRead;
    let client = config.client.independent();
    let id = id.clone();
    let (send, receive) = std::sync::mpsc::sync_channel(1);
    // A bounded worker also covers DNS, connect and TLS time in the caller's
    // aggregate deadline. A timed-out worker retains its permit until I/O exits.
    std::thread::Builder::new()
        .name("gateway-validators".into())
        .spawn(move || {
            let _permit = permit;
            let result = validator_pages(height, deadline, |page| {
                consensus_request(
                    &client,
                    &mut endpoint,
                    "validators",
                    &id,
                    json!({
                        "height": height.to_string(), "page": page.to_string(),
                        "per_page": VALIDATORS_PER_PAGE.to_string()
                    }),
                )
            });
            let _ = send.send(result);
        })
        .ok()?;
    receive
        .recv_timeout(deadline.saturating_duration_since(Instant::now()))
        .ok()?
}

fn validator_pages(
    height: u64,
    deadline: Instant,
    mut read: impl FnMut(u64) -> Option<Value>,
) -> Option<Value> {
    let mut validators = Vec::new();
    let mut total = None;
    let mut addresses = BTreeSet::new();
    for page in 1..=MAX_VALIDATOR_PAGES {
        if Instant::now() >= deadline {
            return None;
        }
        let result = read(page)?;
        if Instant::now() >= deadline {
            return None;
        }
        if decimal(&result["block_height"]) != Some(height) {
            return None;
        }
        let count = decimal(&result["total"])?;
        if count == 0 || count > MAX_VALIDATORS {
            return None;
        }
        if total.is_some_and(|total| total != count) {
            return None;
        }
        total = Some(count);
        let entries = result["validators"].as_array()?;
        if decimal(&result["count"])? != entries.len() as u64
            || entries.len() as u64 > VALIDATORS_PER_PAGE
        {
            return None;
        }
        for entry in entries {
            let address = entry.get("address").filter(|value| value.is_string())?;
            if !addresses.insert(address.as_str()?.to_owned()) {
                return None;
            }
            let public_key = entry.get("pub_key").filter(|value| value.is_object())?;
            let power = entry.get("voting_power")?;
            decimal(power)?;
            validators
                .push(json!({"address": address, "pubKey": public_key, "votingPower": power}));
        }
        if validators.len() as u64 == count {
            return Some(
                json!({"height": height.to_string(), "validators": validators, "total": count.to_string()}),
            );
        }
        if entries.is_empty() || validators.len() as u64 > count {
            return None;
        }
    }
    None
}

fn consensus_inner(left: &[u8; 32], right: &[u8; 32]) -> [u8; 32] {
    let mut hash = Sha256::new();
    hash.update([1]);
    hash.update(left);
    hash.update(right);
    hash.finalize().into()
}

fn consensus_root(leaves: &[[u8; 32]]) -> [u8; 32] {
    match leaves.len() {
        0 => Sha256::digest([]).into(),
        1 => leaves[0],
        length => {
            let split = 1_usize << (length - 1).ilog2();
            consensus_inner(
                &consensus_root(&leaves[..split]),
                &consensus_root(&leaves[split..]),
            )
        }
    }
}

fn transaction_zero_aunts(leaves: &[[u8; 32]], aunts: &mut Vec<[u8; 32]>) {
    if leaves.len() > 1 {
        let split = 1_usize << (leaves.len() - 1).ilog2();
        transaction_zero_aunts(&leaves[..split], aunts);
        aunts.push(consensus_root(&leaves[split..]));
    }
}

fn is_sequence_batch(tx: &[u8], height: u64) -> Option<bool> {
    const DOMAIN: &[u8] = b"PAXEER/v1/sequence-batch\0";
    if !tx.starts_with(DOMAIN) {
        return Some(false);
    }
    let fields: &[u8; 3 * 8 + 4 * 32 + 64] = tx.get(DOMAIN.len()..)?.try_into().ok()?;
    let batch_height = u64::from_be_bytes(fields[..8].try_into().ok()?);
    let from_seq = u64::from_be_bytes(fields[8..16].try_into().ok()?);
    let to_seq = u64::from_be_bytes(fields[16..24].try_into().ok()?);
    let previous_seq = from_seq.checked_sub(1)?;
    if batch_height != height || to_seq < previous_seq {
        return None;
    }
    if to_seq == previous_seq
        && (fields[24..56] != fields[56..88] || fields[88..120] != Sha256::digest([])[..])
    {
        return None;
    }
    Some(true)
}

#[allow(non_snake_case)]
fn px_getSequenceBatch(config: &Config, id: &Value, height: u64) -> Value {
    let Some(result) = consensus(config, "block", id, json!({"height": height.to_string()})) else {
        return unavailable(id, "consensus_unavailable");
    };
    let invalid = || unavailable(id, "invalid_consensus_response");
    let block = &result["block"];
    if decimal(&block["header"]["height"]) != Some(height) {
        return invalid();
    }
    let Some(data_hash) = block["header"]["data_hash"]
        .as_str()
        .and_then(|text| super::parse_hex32(text).ok())
    else {
        return invalid();
    };
    let transactions = match block["data"].get("txs") {
        Some(Value::Null) => Some(Vec::new()),
        Some(Value::Array(txs)) => txs
            .iter()
            .map(|tx| layerx_platform_internal::base64::decode(tx.as_str()?))
            .collect::<Option<Vec<_>>>(),
        _ => None,
    };
    let Some(transactions) = transactions else {
        return invalid();
    };
    let leaves: Vec<[u8; 32]> = transactions
        .iter()
        .map(|tx| {
            let mut hash = Sha256::new();
            hash.update([0]);
            hash.update(Sha256::digest(tx));
            hash.finalize().into()
        })
        .collect();
    let mut aunts = Vec::new();
    transaction_zero_aunts(&leaves, &mut aunts);
    let root = leaves.first().map_or_else(
        || consensus_root(&[]),
        |leaf| {
            aunts
                .iter()
                .fold(*leaf, |left, right| consensus_inner(&left, right))
        },
    );
    if root != data_hash {
        return invalid();
    }
    let batch = transactions
        .first()
        .map_or(Some(false), |tx| is_sequence_batch(tx, height));
    match batch {
        None => invalid(),
        Some(false) => {
            let mut answer = rpc::error(id, -32602, "No sequence batch at this height");
            answer["error"]["data"] = json!({"code": "no_sequence_batch"});
            answer
        }
        Some(true) => {
            let aunts: Vec<_> = aunts
                .iter()
                .map(|aunt| format!("0x{}", super::hex(aunt)))
                .collect();
            json!({"jsonrpc": "2.0", "id": id, "result": {
                "height": height.to_string(), "tx": format!("0x{}", super::hex(&transactions[0])),
                "proof": {"total": transactions.len(), "index": 0,
                    "leafHash": format!("0x{}", super::hex(&leaves[0])), "aunts": aunts}
            }})
        }
    }
}

#[allow(non_snake_case)]
fn px_getReceiptKeys(config: &Config, id: &Value, params: Option<&Value>) -> Value {
    if !params.is_some_and(|params| params.as_array().is_some_and(Vec::is_empty)) {
        return rpc::error(id, -32602, "Invalid params");
    }
    let Some(list) = &config.receipt_keys else {
        return unavailable(id, "receipt_keys_unavailable");
    };
    let keys: Vec<_> = list
        .entries()
        .iter()
        .map(|key| {
            json!({
                "keyId": format!("0x{}", super::hex(&key.sequencer_id())),
                "publicKey": format!("0x{}", super::hex(&key.public_key())),
                "firstHeight": key.first_batch_number(),
                "lastHeight": key.last_batch_number(),
            })
        })
        .collect();
    json!({"jsonrpc": "2.0", "id": id, "result": {"keys": keys}})
}

pub(super) fn dispatch(
    config: &Config,
    method: &str,
    id: &Value,
    params: Option<&Value>,
) -> Option<Value> {
    if !matches!(
        method,
        "px_getCommit" | "px_getValidators" | "px_getReceiptKeys" | "px_getSequenceBatch"
    ) {
        return None;
    }
    if !public_reads::consume_read(&config.read_windows) {
        let mut answer = rpc::error(id, -32005, "Read unavailable");
        answer["error"]["data"] = json!({"code": "public_read_rate_limit"});
        return Some(answer);
    }
    if method == "px_getReceiptKeys" {
        return Some(px_getReceiptKeys(config, id, params));
    }
    let Some(height) = height(params) else {
        return Some(rpc::error(id, -32602, "Invalid params"));
    };
    if method == "px_getSequenceBatch" {
        return Some(px_getSequenceBatch(config, id, height));
    }
    let result = match method {
        "px_getCommit" => px_getCommit(config, id, height),
        "px_getValidators" => px_getValidators(config, id, height),
        _ => unreachable!(),
    };
    Some(result.map_or_else(
        || unavailable(id, "consensus_unavailable"),
        |result| json!({"jsonrpc": "2.0", "id": id, "result": result}),
    ))
}
