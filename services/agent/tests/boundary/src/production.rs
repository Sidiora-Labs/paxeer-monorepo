use std::fs::{self, OpenOptions};
use std::io::Write;
use std::path::PathBuf;
use std::thread;
use std::time::Duration;

use layerx_client::sequencer::{
    CallRequest, ExecutedReceipt, SequencerEndpoint, SequencerError, SequencerReceipt,
};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};

const NODE_RPC_URL: &str = "LAYERX_QUALIFY_NODE_RPC_URL";
const RETIRED_VARIABLES: [&str; 7] = [
    "LAYERX_QUALIFY_LNI_SOCKET",
    "LAYERX_QUALIFY_SIGNED_ACTIVITY",
    "LAYERX_QUALIFY_LNI_DAEMON_UID",
    "LAYERX_QUALIFY_LNI_CLIENT_GID",
    "LAYERX_QUALIFY_MAX_SIGNED_ACTIVITY",
    "LAYERX_QUALIFY_LNI_DEADLINE_MS",
    "LAYERX_QUALIFY_UNKNOWN_SIGNED_ACTIVITY",
];

pub fn refuse_retired_environment() -> Result<(), String> {
    for name in RETIRED_VARIABLES {
        if std::env::var_os(name).is_some() {
            return Err(format!("{name} is retired; use {NODE_RPC_URL}"));
        }
    }
    Ok(())
}

fn required(name: &str) -> Result<String, String> {
    std::env::var(name).map_err(|_| format!("{name} is required and must be valid UTF-8"))
}

fn endpoint() -> Result<SequencerEndpoint, String> {
    refuse_retired_environment()?;
    let url = required(NODE_RPC_URL)?;
    let certificate = std::env::var_os("LAYERX_QUALIFY_NODE_RPC_CA_DER")
        .map(|path| {
            fs::read(path)
                .map_err(|error| format!("could not read LAYERX_QUALIFY_NODE_RPC_CA_DER: {error}"))
        })
        .transpose()?;
    let anchors: Vec<&[u8]> = certificate.iter().map(Vec::as_slice).collect();
    SequencerEndpoint::new_with_railway_private_network(&url, &anchors)
        .map_err(|error| format!("{NODE_RPC_URL} endpoint refused: {error:?}"))
}

fn secret() -> Result<[u8; 32], String> {
    let value = required("LAYERX_QUALIFY_EVM_PRIVATE_KEY")?;
    let hex = value.strip_prefix("0x").unwrap_or(&value);
    if hex.len() != 64 || !hex.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return Err("LAYERX_QUALIFY_EVM_PRIVATE_KEY must be a 32-byte hex key".to_owned());
    }
    let mut secret = [0; 32];
    for (index, byte) in secret.iter_mut().enumerate() {
        *byte = u8::from_str_radix(&hex[index * 2..index * 2 + 2], 16)
            .map_err(|_| "invalid LAYERX_QUALIFY_EVM_PRIVATE_KEY".to_owned())?;
    }
    Ok(secret)
}

fn fee(name: &str) -> Result<u128, String> {
    required(name)?
        .parse()
        .map_err(|_| format!("{name} must be a decimal wei amount"))
}

fn transfer(endpoint: &SequencerEndpoint) -> Result<(Eip1559Call, [u8; 32]), String> {
    let secret = secret()?;
    let mut call = Eip1559Call {
        chain_id: endpoint
            .chain_id()
            .map_err(|error| format!("eth_chainId failed: {error:?}"))?,
        nonce: 0,
        max_priority_fee_per_gas: fee("LAYERX_QUALIFY_MAX_PRIORITY_FEE_PER_GAS")?,
        max_fee_per_gas: fee("LAYERX_QUALIFY_MAX_FEE_PER_GAS")?,
        gas_limit: 21_000,
        to: [0; 20],
        data: Vec::new(),
    };
    if call.max_priority_fee_per_gas > call.max_fee_per_gas || call.max_fee_per_gas == 0 {
        return Err(
            "qualification fee caps must be positive and cover the priority fee".to_owned(),
        );
    }
    let raw = call.sign(&secret).map_err(|error| error.to_string())?;
    let (_, signature) = decode_signed(&raw).map_err(|error| error.to_string())?;
    let sender = recover_sender(&call, &signature).map_err(|error| error.to_string())?;
    call.to = sender;
    call.nonce = endpoint
        .pending_nonce(&sender)
        .map_err(|error| format!("eth_getTransactionCount failed: {error:?}"))?;
    call.gas_limit = endpoint
        .estimate_gas(&CallRequest {
            from: Some(sender),
            to: Some(sender),
            value: 0,
            data: Vec::new(),
        })
        .map_err(|error| format!("eth_estimateGas failed: {error:?}"))?;
    Ok((call, secret))
}

fn retained_path() -> Result<PathBuf, String> {
    std::env::var_os("LAYERX_QUALIFY_TRANSACTION_FILE")
        .map(PathBuf::from)
        .ok_or_else(|| {
            "LAYERX_QUALIFY_TRANSACTION_FILE is required to retain exact signed bytes".to_owned()
        })
}

fn retain(raw: &[u8]) -> Result<(), String> {
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(retained_path()?)
        .map_err(|error| format!("could not create LAYERX_QUALIFY_TRANSACTION_FILE: {error}"))?;
    file.write_all(raw)
        .and_then(|()| file.sync_all())
        .map_err(|error| format!("could not persist signed transaction before submission: {error}"))
}

fn retained(endpoint: &SequencerEndpoint) -> Result<Vec<u8>, String> {
    let raw = fs::read(retained_path()?)
        .map_err(|error| format!("could not read LAYERX_QUALIFY_TRANSACTION_FILE: {error}"))?;
    let (call, signature) =
        decode_signed(&raw).map_err(|error| format!("retained transaction refused: {error}"))?;
    let sender = recover_sender(&call, &signature).map_err(|error| error.to_string())?;
    let chain_id = endpoint
        .chain_id()
        .map_err(|error| format!("eth_chainId failed: {error:?}"))?;
    if call.chain_id != chain_id || call.to != sender || !call.data.is_empty() {
        return Err(
            "retained transaction is not the signed qualification transfer for this chain"
                .to_owned(),
        );
    }
    Ok(raw)
}

fn submit(endpoint: &SequencerEndpoint, raw: &[u8]) -> Result<[u8; 32], String> {
    let expected = transaction_hash(raw);
    let answered = endpoint.send_raw_transaction(raw)
        .map_err(|error| format!("eth_sendRawTransaction failed; retained bytes must be resolved through pax_sequencerReceipt: {error:?}"))?;
    if answered != expected {
        return Err("eth_sendRawTransaction returned a different transaction hash".to_owned());
    }
    Ok(answered)
}

fn require_receipt(endpoint: &SequencerEndpoint, raw: &[u8]) -> Result<ExecutedReceipt, String> {
    let hash = transaction_hash(raw);
    let (call, signature) = decode_signed(raw).map_err(|error| error.to_string())?;
    let sender = recover_sender(&call, &signature).map_err(|error| error.to_string())?;
    for _ in 0..100 {
        let candidate = endpoint
            .sequencer_receipt(&hash)
            .map_err(|error| format!("pax_sequencerReceipt failed: {error:?}"))?;
        let receipt = match candidate {
            Some(SequencerReceipt::Receipted(receipt))
            | Some(SequencerReceipt::Committed { receipt, .. }) => Some(receipt),
            Some(SequencerReceipt::Submitted { nonce, .. }) => {
                if nonce != call.nonce {
                    return Err("submitted receipt changed the retained nonce".to_owned());
                }
                None
            }
            None => None,
        };
        if let Some(receipt) = receipt {
            if receipt.tx_hash != hash
                || receipt.nonce != call.nonce
                || receipt.from != sender
                || receipt.to != call.to
                || !receipt.success
                || !receipt.vm_error.is_empty()
            {
                return Err(
                    "pax_sequencerReceipt did not bind the successful signed transfer".to_owned(),
                );
            }
            return Ok(receipt);
        }
        thread::sleep(Duration::from_millis(20));
    }
    Err(
        "pax_sequencerReceipt did not resolve the retained transaction within the lookup bound"
            .to_owned(),
    )
}

fn qualify_required() -> Result<String, String> {
    let endpoint = endpoint()?;
    let report = crate::cases::agent_boundary_conformance_suite(&endpoint)?;
    let (call, secret) = transfer(&endpoint)?;
    let raw = call.sign(&secret).map_err(|error| error.to_string())?;
    retain(&raw)?;
    submit(&endpoint, &raw)?;
    require_receipt(&endpoint, &raw)?;
    Ok(format!(
        "{report}\nRPC_CASE signed-transfer-admission-and-receipt"
    ))
}

pub fn run_if_configured() -> Result<Option<String>, String> {
    refuse_retired_environment()?;
    if std::env::var_os(NODE_RPC_URL).is_none() {
        return Ok(None);
    }
    qualify_required().map(Some)
}

pub fn run_required() -> Result<(), String> {
    println!("{}", qualify_required()?);
    Ok(())
}

pub fn run_recovery() -> Result<(), String> {
    let endpoint = endpoint()?;
    let raw = retained(&endpoint)?;
    require_receipt(&endpoint, &raw)?;
    println!("RPC_CASE retained-transaction-receipt-recovery");
    Ok(())
}

pub fn run_duplicate() -> Result<(), String> {
    let endpoint = endpoint()?;
    let raw = retained(&endpoint)?;
    let before = require_receipt(&endpoint, &raw)?;
    let first = submit(&endpoint, &raw)?;
    let second = submit(&endpoint, &raw)?;
    if first != second || require_receipt(&endpoint, &raw)? != before {
        return Err("duplicate raw transaction changed its hash or original receipt".to_owned());
    }
    println!("RPC_CASE identical-raw-transaction-preserves-hash-and-receipt");
    Ok(())
}

pub fn run_peer_refusal() -> Result<(), String> {
    let endpoint = endpoint()?;
    let (mut call, secret) = transfer(&endpoint)?;
    call.chain_id ^= 1;
    let raw = call.sign(&secret).map_err(|error| error.to_string())?;
    match endpoint.send_raw_transaction(&raw) {
        Err(SequencerError::Rpc(_)) => {}
        result => {
            return Err(format!(
                "wrong chain id did not receive a JSON-RPC error: {result:?}"
            ))
        }
    }
    println!("RPC_CASE wrong-chain-id-refusal");
    Ok(())
}
