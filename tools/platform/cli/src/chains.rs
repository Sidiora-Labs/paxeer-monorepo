//! The chain registry ABI exposes no next-id read. Listing scans upward until
//! chainCount records have been collected, skipping unknown-chain refusals.

use std::path::Path;
use std::time::{Duration, Instant};

use layerx_client::chains::{
    chain_rpc_url, decode_event, decode_get_chain, decode_params, encode_call, virtual_chain_id,
    ChainCall, ChainEvent, ChainRecord, CHAINS_PRECOMPILE,
};
use layerx_crypto::evm_transaction::transaction_hash;
use serde_json::{json, Value};

use crate::commands::ChainTier;
use crate::http::Client;

fn hex(bytes: &[u8]) -> String {
    let mut text = String::from("0x");
    for byte in bytes {
        use std::fmt::Write as _;
        write!(text, "{byte:02x}").unwrap_or_else(|_| unreachable!());
    }
    text
}

fn unhex(text: &str) -> Result<Vec<u8>, String> {
    let digits = text
        .strip_prefix("0x")
        .ok_or("expected 0x-prefixed hexadecimal data")?;
    if !digits.len().is_multiple_of(2) || !digits.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err("invalid hexadecimal data".into());
    }
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let digits = std::str::from_utf8(pair).map_err(|_| "invalid hexadecimal data")?;
            u8::from_str_radix(digits, 16).map_err(|_| "invalid hexadecimal data".into())
        })
        .collect()
}

fn rpc(client: &Client, method: &str, params: Value) -> Result<Value, String> {
    let response = client.post(
        "/",
        &json!({"jsonrpc":"2.0", "id":1, "method":method, "params":params}),
        None,
    )?;
    if response["jsonrpc"] != "2.0"
        || response["id"] != 1
        || response.get("result").is_some() == response.get("error").is_some()
    {
        return Err(format!("invalid JSON-RPC response for {method}"));
    }
    if let Some(error) = response.get("error") {
        return Err(format!("{method} refused: {error}"));
    }
    Ok(response["result"].clone())
}

fn read(client: &Client, call: &ChainCall) -> Result<Vec<u8>, String> {
    let result = rpc(
        client,
        "eth_call",
        json!([{"to":hex(&CHAINS_PRECOMPILE), "data":hex(&encode_call(call))}, "pending"]),
    )?;
    unhex(
        result
            .as_str()
            .ok_or("eth_call returned non-hexadecimal data")?,
    )
}

fn read_id(client: &Client, call: &ChainCall) -> Result<u32, String> {
    let bytes = read(client, call)?;
    if bytes.len() != 32 || bytes[..28].iter().any(|byte| *byte != 0) {
        return Err("chain registry returned an invalid uint32".into());
    }
    Ok(u32::from_be_bytes(
        bytes[28..].try_into().map_err(|_| "invalid uint32")?,
    ))
}

fn decimal(bytes: &[u8; 32]) -> String {
    let mut digits = vec![0_u16];
    for byte in bytes {
        let mut carry = u16::from(*byte);
        for digit in &mut digits {
            let value = *digit * 256 + carry;
            *digit = value % 10;
            carry = value / 10;
        }
        while carry != 0 {
            digits.push(carry % 10);
            carry /= 10;
        }
    }
    digits
        .iter()
        .rev()
        .map(|digit| char::from(b'0' + u8::try_from(*digit).unwrap_or_else(|_| unreachable!())))
        .collect()
}

fn record(client: &Client, base: &str, id: u32) -> Result<Value, String> {
    if id == 0 {
        return Err("chain_not_found: chain id must be positive".into());
    }
    let chain: ChainRecord = decode_get_chain(&read(client, &ChainCall::GetChain(id))?)
        .map_err(|error| error.to_string())?;
    if chain.id != id || chain.chain_id != virtual_chain_id(id) {
        return Err("chain registry returned a record for a different chain".into());
    }
    Ok(json!({
        "id":chain.id, "name":chain.name, "operator":hex(&chain.operator), "status":chain.status,
        "chainId":chain.chain_id, "gasAllowance":chain.gas_allowance, "byteAllowance":chain.byte_allowance,
        "feeShareBps":chain.fee_share_bps, "bond":decimal(&chain.bond),
        "baseFeePerGas":decimal(&chain.base_fee_per_gas), "createdHeight":chain.created_height,
        "rpcUrl":chain_rpc_url(base, id),
    }))
}

fn unknown_chain_refusal(error: &str) -> bool {
    let Some(refusal) = error.strip_prefix("eth_call refused: ") else {
        return false;
    };
    let Ok(refusal) = serde_json::from_str::<Value>(refusal) else {
        return false;
    };
    refusal["code"] == 3
        && refusal["message"]
            == "execution reverted: LXP_UNKNOWN_FIELD: unknown field; unknown domain"
}

pub fn list(client: &Client, base: &str) -> Result<Value, String> {
    let count = read_id(client, &ChainCall::ChainCount)?;
    let mut chains = Vec::new();
    let mut remaining = count;
    let mut id = 1_u32;
    while remaining > 0 {
        match record(client, base, id) {
            Ok(chain) => {
                chains.push(chain);
                remaining -= 1;
            }
            Err(error) if unknown_chain_refusal(&error) => {}
            Err(error) => return Err(error),
        }
        if remaining > 0 {
            id = id
                .checked_add(1)
                .ok_or("chain registry count exceeds the records in the uint32 id range")?;
        }
    }
    Ok(json!({"chains":chains}))
}

pub fn show(client: &Client, base: &str, id_or_name: &str) -> Result<Value, String> {
    let id = if !id_or_name.is_empty() && id_or_name.bytes().all(|byte| byte.is_ascii_digit()) {
        id_or_name
            .parse::<u32>()
            .map_err(|_| "chain id exceeds uint32")?
    } else {
        read_id(client, &ChainCall::ChainByName(id_or_name.to_owned()))?
    };
    record(client, base, id)
}

fn quantity(client: &Client, method: &str, params: Value) -> Result<u128, String> {
    let value = rpc(client, method, params)?;
    let digits = value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .filter(|value| {
            !value.is_empty()
                && value.len() <= 32
                && (value.len() == 1 || !value.starts_with('0'))
                && value.bytes().all(|byte| byte.is_ascii_hexdigit())
        })
        .ok_or_else(|| format!("{method} returned an invalid hexadecimal quantity"))?;
    u128::from_str_radix(digits, 16).map_err(|_| format!("{method} quantity exceeds 128 bits"))
}

fn add_value(bond: [u8; 32], fee: [u8; 32]) -> Result<[u8; 32], String> {
    let mut result = [0; 32];
    let mut carry = 0_u16;
    for index in (0..32).rev() {
        let sum = u16::from(bond[index]) + u16::from(fee[index]) + carry;
        result[index] = sum.to_be_bytes()[1];
        carry = sum >> 8;
    }
    if carry != 0 {
        return Err("chain bond plus fee exceeds uint256".into());
    }
    Ok(result)
}

fn value_quantity(value: &[u8; 32]) -> String {
    let text = hex(value);
    let digits = text[2..].trim_start_matches('0');
    format!("0x{}", if digits.is_empty() { "0" } else { digits })
}

fn created(
    receipt: &Value,
    name: &str,
    operator: &[u8; 20],
    base: &str,
    hash: &str,
) -> Result<Value, String> {
    if receipt["transactionHash"] != hash || receipt["status"] != "0x1" {
        return Err("chain creation receipt is mismatched or the transaction reverted".into());
    }
    let logs = receipt["logs"]
        .as_array()
        .ok_or("chain creation receipt has no logs")?;
    let topic0 = hex(&transaction_hash(
        b"ChainCreated(uint32,string,address,uint64)",
    ));
    let mut result = None;
    for log in logs {
        if log["address"]
            .as_str()
            .map(str::to_ascii_lowercase)
            .as_deref()
            != Some(hex(&CHAINS_PRECOMPILE).as_str())
            || log["topics"][0] != topic0
        {
            continue;
        }
        let topics = log["topics"]
            .as_array()
            .ok_or("invalid chain event topics")?
            .iter()
            .map(|topic| {
                unhex(topic.as_str().ok_or("invalid chain event topic")?)?
                    .try_into()
                    .map_err(|_| "invalid chain event topic width".to_owned())
            })
            .collect::<Result<Vec<[u8; 32]>, String>>()?;
        let data = unhex(log["data"].as_str().ok_or("invalid chain event data")?)?;
        if let ChainEvent::Created {
            id,
            name: event_name,
            operator: event_operator,
            chain_id,
        } = decode_event(&topics, &data).map_err(|error| error.to_string())?
        {
            if id == 0
                || event_name != name
                || event_operator != *operator
                || chain_id != virtual_chain_id(id)
                || result.is_some()
            {
                return Err("ChainCreated event does not match the submitted creation".into());
            }
            result = Some(
                json!({"id":id, "name":event_name, "chainId":chain_id, "rpcUrl":chain_rpc_url(base, id), "transactionHash":hash}),
            );
        }
    }
    result.ok_or_else(|| "chain creation receipt contains no ChainCreated event".into())
}

pub fn create(
    client: &Client,
    base: &str,
    name: &str,
    tier: ChainTier,
    fee_share_bps: u16,
    key_file: &Path,
) -> Result<Value, String> {
    if !(3..=32).contains(&name.len())
        || name.starts_with('-')
        || name.ends_with('-')
        || !name
            .bytes()
            .all(|byte| byte.is_ascii_lowercase() || byte.is_ascii_digit() || byte == b'-')
    {
        return Err("chain name must be 3-32 lowercase letters, digits or hyphens with no leading or trailing hyphen".into());
    }
    if fee_share_bps > 5000 {
        return Err("fee share must be between 0 and 5000 bps".into());
    }
    let params =
        decode_params(&read(client, &ChainCall::Params)?).map_err(|error| error.to_string())?;
    let (gas_allowance, byte_allowance) = tier.allowances();
    if gas_allowance < params.min_gas_allowance
        || gas_allowance > params.max_gas_allowance
        || gas_allowance > params.total_gas_allowance_left
    {
        return Err("chain tier exceeds the available gas allowance bounds".into());
    }
    let value = add_value(params.min_bond, params.creation_fee)?;
    let data = encode_call(&ChainCall::CreateChain {
        name: name.to_owned(),
        gas_allowance,
        byte_allowance,
        fee_share_bps,
    });
    let (secret, address) = read_key(key_file)?;
    let chain_id = quantity(client, "eth_chainId", json!([]))?;
    if chain_id != 125 {
        return Err("chain creation requires chainId 125".into());
    }
    let nonce = u64::try_from(quantity(
        client,
        "eth_getTransactionCount",
        json!([hex(&address), "pending"]),
    )?)
    .map_err(|_| "transaction count exceeds 64 bits")?;
    let gas_price = quantity(client, "eth_gasPrice", json!([]))?;
    let priority = quantity(client, "eth_maxPriorityFeePerGas", json!([]))?;
    let max_fee = gas_price
        .checked_mul(2)
        .ok_or("gas price exceeds fee range")?;
    if max_fee == 0 || priority > max_fee {
        return Err("invalid EIP-1559 fee bounds".into());
    }
    let gas = u64::try_from(quantity(client, "eth_estimateGas", json!([{
        "from":hex(&address), "to":hex(&CHAINS_PRECOMPILE), "data":hex(&data), "value":value_quantity(&value),
    }, "pending"]))?).map_err(|_| "gas estimate exceeds 64 bits")?;
    if gas == 0 {
        return Err("gas estimate must be positive".into());
    }
    let call = PayableCall {
        nonce,
        priority,
        max_fee,
        gas,
        value,
        data,
    };
    let raw = call.sign(&secret, &address)?;
    let hash = hex(&transaction_hash(&raw));
    match rpc(client, "eth_sendRawTransaction", json!([hex(&raw)])) {
        Ok(returned) if returned == hash => {}
        outcome => {
            return Ok(json!({
                "transactionHash": hash, "state": "unknown", "name": name,
                "operator": hex(&address),
                "detail": match outcome {
                    Ok(_) => "RPC transaction hash differs from signed chain creation".to_owned(),
                    Err(detail) => detail,
                },
            }))
        }
    }
    let deadline = Instant::now() + Duration::from_secs(60);
    loop {
        let receipt = match rpc(client, "eth_getTransactionReceipt", json!([hash])) {
            Ok(receipt) => receipt,
            Err(detail) => {
                return Ok(
                    json!({"transactionHash":hash,"state":"pending","name":name,"operator":hex(&address),"detail":detail}),
                )
            }
        };
        if !receipt.is_null() {
            return created(&receipt, name, &address, base, &hash);
        }
        if Instant::now() >= deadline {
            return Ok(
                json!({"transactionHash":hash,"state":"pending","name":name,"operator":hex(&address)}),
            );
        }
        std::thread::sleep(Duration::from_millis(250));
    }
}

/// Reconciles an original creation without signing or submitting a second transaction.
pub fn reconcile_creation(
    client: &Client,
    base: &str,
    hash: &str,
    name: &str,
    operator: &str,
) -> Result<Value, String> {
    if unhex(hash)?.len() != 32 {
        return Err("transaction hash must contain 32 bytes".into());
    }
    let operator: [u8; 20] = unhex(operator)?
        .try_into()
        .map_err(|_| "operator must contain 20 bytes")?;
    let receipt = rpc(client, "eth_getTransactionReceipt", json!([hash]))
        .map_err(|error| format!("chain creation {hash}: {error}"))?;
    if receipt.is_null() {
        return Ok(json!({"transactionHash":hash,"state":"pending"}));
    }
    let result = created(&receipt, name, &operator, base, hash)?;
    let id = result["id"]
        .as_u64()
        .and_then(|id| u32::try_from(id).ok())
        .ok_or("invalid recovered chain id")?;
    let domain = record(client, base, id)?;
    if domain["name"] != name || domain["operator"] != hex(&operator) {
        return Err(format!(
            "chain creation {hash}: recovered domain contradicts receipt"
        ));
    }
    Ok(json!({"state":"completed","transactionHash":hash,"chain":domain}))
}

/// Public node endpoints use the origin root; hosted services are configured separately.
pub fn node_endpoint(url: &str) -> Result<&str, String> {
    let uri: ureq::http::Uri = url.parse().map_err(|_| "invalid node endpoint")?;
    if !matches!(uri.scheme_str(), Some("http" | "https"))
        || uri.authority().is_none()
        || uri.authority().is_some_and(|a| a.as_str().contains('@'))
        || uri.path_and_query().is_some_and(|p| p.as_str() != "/")
        || url.contains('#')
    {
        return Err("node endpoint must be an http(s) origin at the root".into());
    }
    Client::new(url, None)?;
    Ok(url.trim_end_matches('/'))
}

#[derive(serde::Deserialize)]
struct KeyExport {
    evm_address: String,
    evm_private_key: String,
}

impl Drop for KeyExport {
    fn drop(&mut self) {
        use zeroize::Zeroize as _;
        self.evm_private_key.zeroize();
    }
}

fn read_key(path: &Path) -> Result<(zeroize::Zeroizing<[u8; 32]>, [u8; 20]), String> {
    use std::io::Read as _;
    use std::os::unix::fs::{MetadataExt as _, OpenOptionsExt as _};
    let flags = i32::try_from((rustix::fs::OFlags::NOFOLLOW | rustix::fs::OFlags::NONBLOCK).bits())
        .map_err(|_| "key file flags are unavailable")?;
    let file = std::fs::OpenOptions::new()
        .read(true)
        .custom_flags(flags)
        .open(path)
        .map_err(|error| format!("could not open EVM key file: {error}"))?;
    let metadata = file.metadata().map_err(|error| error.to_string())?;
    if !metadata.is_file()
        || metadata.mode() & 0o077 != 0
        || metadata.uid() != rustix::process::geteuid().as_raw()
    {
        return Err("EVM key file must be a caller-owned, owner-only regular file".into());
    }
    let mut bytes = zeroize::Zeroizing::new(Vec::new());
    file.take(4097)
        .read_to_end(&mut bytes)
        .map_err(|error| error.to_string())?;
    if bytes.len() > 4096 {
        return Err("EVM key file exceeds 4096 bytes".into());
    }
    let exported: KeyExport = serde_json::from_slice(&bytes)
        .map_err(|_| "expected an EVM key export from wallet derive")?;
    let address = unhex(&exported.evm_address)?
        .try_into()
        .map_err(|_| "EVM address must have 20 bytes")?;
    let secret_bytes = zeroize::Zeroizing::new(unhex(&format!("0x{}", exported.evm_private_key))?);
    let mut secret = zeroize::Zeroizing::new([0; 32]);
    if secret_bytes.len() != 32 {
        return Err("EVM private key must have 32 bytes".into());
    }
    secret.copy_from_slice(&secret_bytes);
    Ok((secret, address))
}

fn rlp_bytes(bytes: &[u8], list: bool) -> Vec<u8> {
    if !list && bytes.len() == 1 && bytes[0] < 0x80 {
        return bytes.to_vec();
    }
    let short = if list { 0xc0 } else { 0x80 };
    let mut result = Vec::new();
    if bytes.len() < 56 {
        result.push(short + u8::try_from(bytes.len()).unwrap_or_else(|_| unreachable!()));
    } else {
        let length = bytes.len().to_be_bytes();
        let start = length
            .iter()
            .position(|byte| *byte != 0)
            .unwrap_or(length.len());
        result.push(
            short + 55 + u8::try_from(length.len() - start).unwrap_or_else(|_| unreachable!()),
        );
        result.extend_from_slice(&length[start..]);
    }
    result.extend_from_slice(bytes);
    result
}

fn rlp_uint(bytes: &[u8]) -> Vec<u8> {
    let start = bytes
        .iter()
        .position(|byte| *byte != 0)
        .unwrap_or(bytes.len());
    rlp_bytes(&bytes[start..], false)
}

struct PayableCall {
    nonce: u64,
    priority: u128,
    max_fee: u128,
    gas: u64,
    value: [u8; 32],
    data: Vec<u8>,
}

impl PayableCall {
    fn fields(&self) -> Vec<u8> {
        [
            rlp_uint(&125_u64.to_be_bytes()),
            rlp_uint(&self.nonce.to_be_bytes()),
            rlp_uint(&self.priority.to_be_bytes()),
            rlp_uint(&self.max_fee.to_be_bytes()),
            rlp_uint(&self.gas.to_be_bytes()),
            rlp_bytes(&CHAINS_PRECOMPILE, false),
            rlp_uint(&self.value),
            rlp_bytes(&self.data, false),
            vec![0xc0],
        ]
        .concat()
    }

    fn sign(&self, secret: &[u8; 32], address: &[u8; 20]) -> Result<Vec<u8>, String> {
        let fields = self.fields();
        let mut unsigned = vec![2];
        unsigned.extend_from_slice(&rlp_bytes(&fields, true));
        let digest = transaction_hash(&unsigned);
        let key =
            k256::ecdsa::SigningKey::from_slice(secret).map_err(|_| "invalid EVM private key")?;
        let public_key = key.verifying_key().to_encoded_point(false);
        if layerx_crypto::secp256k1::evm_address(public_key.as_bytes())
            .map_err(|error| format!("{error:?}"))?
            != *address
        {
            return Err("EVM key file address does not match its private key".into());
        }
        let (signature, recovery) = key
            .sign_prehash_recoverable(&digest)
            .map_err(|_| "EVM transaction signing failed")?;
        let compact: [u8; 64] = signature.to_bytes().into();
        let recovery_v = 27 + recovery.to_byte();
        layerx_crypto::secp256k1::verify_recoverable_digest(
            public_key.as_bytes(),
            &compact,
            recovery_v,
            &digest,
        )
        .map_err(|error| format!("EVM signature verification failed: {error:?}"))?;
        let mut signed = fields;
        signed.extend_from_slice(&rlp_uint(&[recovery.to_byte()]));
        signed.extend_from_slice(&rlp_uint(&compact[..32]));
        signed.extend_from_slice(&rlp_uint(&compact[32..]));
        let mut raw = vec![2];
        raw.extend_from_slice(&rlp_bytes(&signed, true));
        Ok(raw)
    }
}
