use std::fmt::Write as _;
use std::io::{self, Read, Write};

use layerx_client::px::PxEndpoint;
use layerx_client::sequencer::{SequencerEndpoint, ADDR_PRECOMPILE, GET_EVM_ADDR_BY_LAYERX};
use layerx_types::account::AccountId;
use layerx_types::ids::Did;
use serde::Deserialize;

const MAX_INPUT: u64 = 1_048_576;
const LEDGER: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
];

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Input {
    rpc_url: String,
    rpc_ca_der_hex: Vec<String>,
    chain_id: u64,
    owner_did: String,
    owner_account: String,
    asset_id: String,
    maximum_age_seconds: u64,
}

fn refused() -> io::Error {
    io::Error::new(io::ErrorKind::InvalidData, "account-state head refused")
}

fn hex(value: &str) -> io::Result<Vec<u8>> {
    if !value.len().is_multiple_of(2)
        || !value
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    {
        return Err(refused());
    }
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let text = std::str::from_utf8(pair).map_err(|_| refused())?;
            u8::from_str_radix(text, 16).map_err(|_| refused())
        })
        .collect()
}

fn fixed(value: &str) -> io::Result<[u8; 32]> {
    let bytes: [u8; 32] = hex(value)?.try_into().map_err(|_| refused())?;
    if bytes == [0; 32] {
        return Err(refused());
    }
    Ok(bytes)
}

fn encoded(bytes: &[u8]) -> String {
    let mut text = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        let _ = write!(text, "{byte:02x}");
    }
    text
}

fn account_id(name: &str) -> io::Result<[u8; 32]> {
    let account = AccountId::parse(name).map_err(|_| refused())?;
    layerx_intents::canonical::account_id_for_protocol(&account, 3).map_err(|_| refused())
}

fn call(
    endpoint: &SequencerEndpoint,
    height: u64,
    to: &[u8; 20],
    selector: [u8; 4],
    arguments: &[u8],
) -> io::Result<[u8; 32]> {
    let data = [&selector[..], arguments].concat();
    endpoint
        .call_at(height, to, &data)
        .map_err(|_| refused())?
        .try_into()
        .map_err(|_| refused())
}

/// Reads the current account through the configured authenticated chain endpoint.
/// `now_ms` must come from the caller's trusted clock, never the request document.
///
/// # Errors
/// Refuses untrusted transport, changed or stale heads, conflicting identities,
/// retired batch envelopes and any debit history whose limit cannot be recovered.
pub fn validate_account_head(
    input: impl Read,
    mut output: impl Write,
    now_ms: u64,
) -> io::Result<()> {
    let mut bytes = Vec::new();
    input.take(MAX_INPUT + 1).read_to_end(&mut bytes)?;
    if bytes.len() as u64 > MAX_INPUT {
        return Err(refused());
    }
    let input: Input = serde_json::from_slice(&bytes).map_err(|_| refused())?;
    if input.chain_id == 0 || !(1..=300).contains(&input.maximum_age_seconds) {
        return Err(refused());
    }
    let public = fixed(
        input
            .owner_did
            .strip_prefix("did:layerx:")
            .ok_or_else(refused)?,
    )?;
    let asset = fixed(&input.asset_id)?;
    let account = fixed(&input.owner_account)?;
    if account
        != account_id(&format!(
            "agent:{}:asset:{}",
            input.owner_did, input.asset_id
        ))?
    {
        return Err(refused());
    }
    let main = account_id(&format!("agent:{}:main", input.owner_did))?;
    let did = Did::new(input.owner_did.as_bytes()).map_err(|_| refused())?;
    let owner = layerx_intents::canonical::did_id_for_protocol(&did, 3).map_err(|_| refused())?;
    let anchors = input
        .rpc_ca_der_hex
        .iter()
        .map(|anchor| hex(anchor))
        .collect::<io::Result<Vec<_>>>()?;
    let anchors: Vec<_> = anchors.iter().map(Vec::as_slice).collect();
    let endpoint = SequencerEndpoint::new(&input.rpc_url, &anchors).map_err(|_| refused())?;
    let network = PxEndpoint::new(&input.rpc_url, &anchors).map_err(|_| refused())?;
    if endpoint.chain_id().map_err(|_| refused())? != input.chain_id {
        return Err(refused());
    }
    let height = endpoint.block_number().map_err(|_| refused())?;
    if height == 0 {
        return Err(refused());
    }
    // The pinned HTTPS node (or a local node on loopback) authenticates these
    // committed observations; caller-supplied headers are never authoritative.
    let block = endpoint
        .block_by_number(height)
        .map_err(|_| refused())?
        .ok_or_else(refused)?;
    let state_root = endpoint.state_root(height).map_err(|_| refused())?;
    let observed_at = block.timestamp.checked_mul(1000).ok_or_else(refused)?;
    if block.hash == [0; 32]
        || state_root == [0; 32]
        || observed_at == 0
        || now_ms
            .checked_sub(observed_at)
            .is_none_or(|age| age > input.maximum_age_seconds * 1000)
    {
        return Err(refused());
    }
    let join = network.account(&input.owner_did).map_err(|_| refused())?;
    let address = join.account.evm_address.ok_or_else(refused)?;
    if !join.account.bound
        || address == [0; 20]
        || join.account.layerx_did.as_deref() != Some(input.owner_did.as_str())
        || join.account.layerx_account != Some(main)
        || join
            .paxeer
            .as_ref()
            .is_none_or(|paxeer| paxeer.address != address)
    {
        return Err(refused());
    }
    let bound = call(
        &endpoint,
        height,
        &ADDR_PRECOMPILE,
        GET_EVM_ADDR_BY_LAYERX,
        &public,
    )?;
    let identified = call(
        &endpoint,
        height,
        &LEDGER,
        [0x3f, 0x05, 0x0c, 0xbc],
        &[public, asset].concat(),
    )?;
    let holder = call(
        &endpoint,
        height,
        &LEDGER,
        [0x37, 0x95, 0x5a, 0xb2],
        &account,
    )?;
    if bound[..12] != [0; 12] || bound[12..] != address || identified != account || holder != owner
    {
        return Err(refused());
    }
    let arguments = [account, asset].concat();
    let balance = call(
        &endpoint,
        height,
        &LEDGER,
        [0xb1, 0xff, 0xc0, 0x3e],
        &arguments,
    )?;
    let sequence = call(
        &endpoint,
        height,
        &LEDGER,
        [0xce, 0x2f, 0x92, 0x1f],
        &arguments,
    )?;
    // Only an account with no debits proves an unused limit. A later chain
    // height alone says nothing about consumption; unsupported history refuses.
    if balance[..16] != [0; 16] || sequence != [0; 32] {
        return Err(refused());
    }
    if network.account(&input.owner_did).map_err(|_| refused())? != join
        || endpoint.block_number().map_err(|_| refused())? != height
        || endpoint
            .block_by_number(height)
            .map_err(|_| refused())?
            .as_ref()
            != Some(&block)
        || endpoint.state_root(height).map_err(|_| refused())? != state_root
    {
        return Err(refused());
    }
    let balance = u128::from_be_bytes(balance[16..].try_into().map_err(|_| refused())?);
    let result = serde_json::json!({
        "schema": "layerx.human.account-head.v1",
        "chain_id": input.chain_id, "owner_did": input.owner_did,
        "account": input.owner_account, "asset_id": input.asset_id,
        "committed_height": height, "block_hash": encoded(&block.hash),
        "state_root": encoded(&state_root), "observed_at": observed_at,
        "balance": balance.to_string(), "consumed": 0
    });
    serde_json::to_writer(&mut output, &result)?;
    output.write_all(b"\n")
}
