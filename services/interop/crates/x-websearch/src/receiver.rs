//! Receiver identity inspection and first binding through the settlement endpoint.

use std::collections::BTreeMap;
use std::path::Path;
use std::thread;
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use serde_json::{json, Value};

use crate::attest::signer_address;
use crate::keys::{load_identity, KeyFiles};
use crate::payment::{
    hex, read_asset_availability, read_receiver_did, LedgerTransaction, WithheldAsset,
    LEDGER_PRECOMPILE,
};
use crate::settlement::SettlementRpc;
use crate::watch::{hex0x, keccak, parse_quantity, unhex0x};
use crate::Config;

pub const ADDR_PRECOMPILE: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x04,
];

const RECEIPT_TIMEOUT: Duration = Duration::from_secs(60);
const RECEIPT_POLL_INTERVAL: Duration = Duration::from_millis(250);

/// The Ed25519 consent message reconstructed by the address precompile.
#[must_use]
pub fn binding_message(chain_id: u64, address: &[u8; 20], nonce: u64) -> Vec<u8> {
    [
        b"LX:PAXEER-BIND:v1".as_slice(),
        &[0; 24],
        &chain_id.to_be_bytes(),
        address,
        &nonce.to_be_bytes(),
    ]
    .concat()
}

/// Encodes `bindLayerX(bytes32,bytes)` with a 64-byte Ed25519 signature.
#[must_use]
pub fn binding_calldata(
    identity: &SigningKey,
    chain_id: u64,
    address: &[u8; 20],
    nonce: u64,
) -> Vec<u8> {
    let signature = identity.sign(&binding_message(chain_id, address, nonce));
    [
        &keccak(b"bindLayerX(bytes32,bytes)")[..4],
        identity.verifying_key().as_bytes(),
        &[0; 31],
        &[64],
        &[0; 31],
        &[64],
        signature.to_bytes().as_slice(),
    ]
    .concat()
}

fn address_call(signature: &[u8], address: &[u8; 20]) -> Vec<u8> {
    [&keccak(signature)[..4], &[0; 12], address].concat()
}

fn word_u64(word: &[u8]) -> Option<u64> {
    if word.len() != 32 || word[..24].iter().any(|byte| *byte != 0) {
        return None;
    }
    Some(u64::from_be_bytes(word[24..].try_into().ok()?))
}

fn bound_identity(rpc: &SettlementRpc, address: &[u8; 20]) -> Result<Option<[u8; 32]>, String> {
    let answer = rpc
        .eth_call(
            ADDR_PRECOMPILE,
            &address_call(b"getUnifiedAccount(address)", address),
        )
        .map_err(|_| "receiver identity read failed")?;
    let malformed = || "receiver identity answer is malformed".to_owned();
    if answer.len() < 160
        || answer[..12] != [0; 12]
        || answer[12..32] != *address
        || word_u64(&answer[32..64]) != Some(128)
    {
        return Err(malformed());
    }
    let pax_len = word_u64(&answer[128..160])
        .and_then(|length| usize::try_from(length).ok())
        .ok_or_else(malformed)?;
    let end = 160_usize.checked_add(pax_len).ok_or_else(malformed)?;
    let padded = end.checked_add(31).ok_or_else(malformed)? / 32 * 32;
    if answer.len() != padded || answer[end..].iter().any(|byte| *byte != 0) {
        return Err(malformed());
    }
    let key: [u8; 32] = answer[64..96].try_into().map_err(|_| malformed())?;
    Ok((key != [0; 32]).then_some(key))
}

fn quantity(rpc: &SettlementRpc, method: &str, params: &Value) -> Result<u128, String> {
    rpc.call(method, params)
        .map_err(|_| format!("{method} read failed"))?
        .as_str()
        .and_then(parse_quantity)
        .ok_or_else(|| format!("{method} answer is malformed"))
}

fn status(config: &Config, rpc: &SettlementRpc, address: [u8; 20]) -> Result<Value, String> {
    let owner = bound_identity(rpc, &address)?;
    let mut accounts = BTreeMap::new();
    let mut payable_assets = Vec::new();
    let mut withheld_assets = Vec::new();
    if let Some(owner) = owner {
        for symbol in crate::config::AssetSymbol::ALL {
            let asset = config
                .assets
                .iter()
                .find(|asset| asset.symbol == symbol)
                .ok_or_else(|| format!("asset {} is missing", symbol.code()))?;
            match read_asset_availability(rpc, &asset.asset_id).map_err(|error| {
                format!("ledger asset read failed for {}: {error}", symbol.code())
            })? {
                Some(reason) => withheld_assets.push(WithheldAsset {
                    symbol: symbol.code(),
                    reason,
                }),
                None => payable_assets.push(symbol.code()),
            }
        }
        for asset in &config.assets {
            let data = [
                &keccak(b"accountId(bytes32,bytes32)")[..4],
                owner.as_slice(),
                &asset.asset_id,
            ]
            .concat();
            let account = rpc
                .eth_call(LEDGER_PRECOMPILE, &data)
                .map_err(|_| format!("ledger account read failed for {}", asset.symbol.code()))?;
            if account.len() != 32 || account.iter().all(|byte| *byte == 0) {
                return Err(format!(
                    "ledger account answer is malformed for {}",
                    asset.symbol.code()
                ));
            }
            accounts.insert(asset.symbol.code(), hex0x(&account));
        }
    }
    let balance = quantity(rpc, "eth_getBalance", &json!([hex0x(&address), "latest"]))?;
    Ok(json!({
        "address": hex0x(&address),
        "bound": owner.is_some(),
        "identity": owner.map(|key| format!("did:layerx:{}", hex(&key))),
        "ledger_accounts": accounts,
        "payable_assets": payable_assets,
        "withheld_assets": withheld_assets,
        "balance": balance.to_string(),
    }))
}

fn wait_for_binding(
    rpc: &SettlementRpc,
    address: &[u8; 20],
    identity: &[u8; 32],
    hash: &[u8; 32],
) -> Result<(), String> {
    let deadline = Instant::now() + RECEIPT_TIMEOUT;
    loop {
        let receipt = rpc
            .call("eth_getTransactionReceipt", &json!([hex0x(hash)]))
            .map_err(|_| "transaction receipt read failed")?;
        if !receipt.is_null() {
            let reported_hash = receipt["transactionHash"].as_str().and_then(unhex0x);
            if reported_hash.as_deref() != Some(hash.as_slice()) {
                return Err("transaction receipt hash is malformed or mismatched".to_owned());
            }
            match receipt["status"].as_str().and_then(parse_quantity) {
                Some(0) => return Err("binding transaction receipt failed".to_owned()),
                Some(1) => {}
                _ => return Err("transaction receipt status is malformed".to_owned()),
            }
            let bound =
                read_receiver_did(rpc, address).map_err(|_| "binding confirmation read failed")?;
            if &bound != identity {
                return Err("binding confirmation returned a different identity".to_owned());
            }
            return Ok(());
        }
        if Instant::now() >= deadline {
            return Err("timed out waiting for the binding transaction receipt".to_owned());
        }
        thread::sleep(RECEIPT_POLL_INTERVAL);
    }
}

fn bind(
    config: &Config,
    rpc: &SettlementRpc,
    receiver: &k256::ecdsa::SigningKey,
    identity: &SigningKey,
) -> Result<Value, String> {
    let address = signer_address(receiver);
    let public = identity.verifying_key().to_bytes();
    let did = format!("did:layerx:{}", hex(&public));
    if let Some(bound) = bound_identity(rpc, &address)? {
        if bound != public {
            return Err(format!(
                "receiver {} is bound to another identity did:layerx:{}",
                hex0x(&address),
                hex(&bound)
            ));
        }
        return Ok(
            json!({"address": hex0x(&address), "identity": did, "bound": true,
            "already_bound": true, "transaction_hash": null}),
        );
    }
    let nonce_word = rpc
        .eth_call(
            ADDR_PRECOMPILE,
            &address_call(b"layerXBindNonce(address)", &address),
        )
        .map_err(|_| "binding nonce read failed")?;
    let bind_nonce = word_u64(&nonce_word).ok_or("binding nonce answer is malformed")?;
    let data = binding_calldata(identity, config.evm.chain_id, &address, bind_nonce);
    let gas_price = quantity(rpc, "eth_gasPrice", &json!([]))?;
    let gas_limit = quantity(
        rpc,
        "eth_estimateGas",
        &json!([{
            "from": hex0x(&address), "to": hex0x(&ADDR_PRECOMPILE),
            "data": hex0x(&data), "value": "0x0", "gasPrice": "0x0",
        }]),
    )?;
    let gas_limit = u64::try_from(gas_limit).map_err(|_| "gas estimate exceeds 64 bits")?;
    if gas_price == 0 || gas_limit == 0 {
        return Err("gas price and gas estimate must be positive".to_owned());
    }
    let needed = gas_price
        .checked_mul(u128::from(gas_limit))
        .ok_or("binding transaction cost exceeds 128 bits")?;
    let balance = quantity(rpc, "eth_getBalance", &json!([hex0x(&address), "pending"]))?;
    if balance < needed {
        return Err(format!("receiver {} needs {needed} base units for the binding transaction; balance is {balance}", hex0x(&address)));
    }
    let nonce = rpc
        .pending_nonce(address)
        .map_err(|_| "receiver transaction nonce read failed")?;
    let signed = LedgerTransaction {
        chain_id: config.evm.chain_id,
        nonce,
        gas_price,
        gas_limit,
        data,
    }
    .sign_to(receiver, ADDR_PRECOMPILE)
    .map_err(|_| "receiver transaction signing failed")?;
    let context = format!(
        "receiver {} identity {did} transaction {}",
        hex0x(&address),
        hex0x(&signed.hash)
    );
    let hash = rpc
        .send_raw_transaction(&signed.raw)
        .map_err(|_| format!("{context}: broadcast failed"))?;
    if hash != signed.hash {
        return Err(format!(
            "{context}: broadcast returned a different transaction hash"
        ));
    }
    wait_for_binding(rpc, &address, &public, &hash)
        .map_err(|reason| format!("{context}: {reason}"))?;
    Ok(
        json!({"address": hex0x(&address), "identity": did, "bound": true,
        "already_bound": false, "transaction_hash": hex0x(&hash)}),
    )
}

/// Runs a receiver command without opening the sidecar's stores or listener.
/// A supplied identity path selects binding; otherwise this reports status.
///
/// # Errors
/// Reports key refusals, failed chain reads, unsafe bindings and failed transactions.
pub fn run(config: &Config, identity_path: Option<&Path>) -> Result<Value, String> {
    let receiver = KeyFiles::load_receiver_from_env().map_err(|error| error.to_string())?;
    let identity = identity_path
        .map(load_identity)
        .transpose()
        .map_err(|error| error.to_string())?;
    if identity
        .as_ref()
        .is_some_and(|key| key.as_bytes().as_slice() == receiver.to_bytes().as_slice())
    {
        return Err("identity key must differ from the receiver key".to_owned());
    }
    let rpc = SettlementRpc::new(&config.gateway.endpoint, config.evm.chain_id)
        .map_err(|_| "settlement endpoint is malformed")?;
    match identity {
        Some(identity) => bind(config, &rpc, &receiver, &identity),
        None => status(config, &rpc, signer_address(&receiver)),
    }
}
