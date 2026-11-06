//! Shared hosted wallet query producers. Native calls must be pinned to `height` by the caller.
use serde_json::{json, Value};

#[path = "../../../../services/platform/hosted/gateway/src/evm.rs"]
pub mod evm;

pub const ASSET_PAGE_LIMIT: usize = 16;

pub fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

pub fn key(text: &str) -> Result<[u8; 32], &'static str> {
    if text.len() != 64
        || !text
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    {
        return Err("invalid_identifier");
    }
    let mut bytes = [0; 32];
    for (i, pair) in text.as_bytes().chunks_exact(2).enumerate() {
        bytes[i] = u8::from_str_radix(
            std::str::from_utf8(pair).map_err(|_| "invalid_identifier")?,
            16,
        )
        .map_err(|_| "invalid_identifier")?;
    }
    if bytes == [0; 32] {
        return Err("invalid_identifier");
    }
    Ok(bytes)
}

pub fn ledger_account(
    did: &str,
    asset_text: &str,
    height: u64,
    call: &mut dyn FnMut(&[u8]) -> Result<Vec<u8>, &'static str>,
) -> Result<Value, &'static str> {
    let owner = key(did.strip_prefix("did:layerx:").ok_or("invalid_did")?)?;
    let asset = key(asset_text)?;
    let mut data = vec![0x3f, 0x05, 0x0c, 0xbc];
    data.extend_from_slice(&owner);
    data.extend_from_slice(&asset);
    let id: [u8; 32] = call(&data)?
        .try_into()
        .map_err(|_| "invalid_ledger_account")?;
    let mut native = [0; 32];
    native[0] = 1;
    let name = layerx_wire::account::account_name_for_asset(did, asset, native)
        .map_err(|_| "invalid_ledger_account")?;
    let expected = layerx_wire::hash::account_id_for_protocol(&name, 3)
        .map_err(|_| "invalid_ledger_account")?;
    if id != expected {
        return Err("conflicting_ledger_account");
    }
    let mut query = vec![0x37, 0x95, 0x5a, 0xb2];
    query.extend_from_slice(&id);
    if call(&query)?.as_slice() != owner {
        return Err("conflicting_ledger_owner");
    }
    let mut query = vec![0xce, 0x2f, 0x92, 0x1f];
    query.extend_from_slice(&id);
    query.extend_from_slice(&asset);
    let sequence = call(&query)?;
    if sequence.len() != 32 || sequence[..24].iter().any(|b| *b != 0) {
        return Err("invalid_ledger_sequence");
    }
    let next = u64::from_be_bytes(
        sequence[24..]
            .try_into()
            .map_err(|_| "invalid_ledger_sequence")?,
    );
    query[..4].copy_from_slice(&[0xb1, 0xff, 0xc0, 0x3e]);
    let balance = call(&query)?;
    if balance.len() != 32 || balance[..16].iter().any(|b| *b != 0) {
        return Err("invalid_ledger_balance");
    }
    let balance = u128::from_be_bytes(
        balance[16..]
            .try_into()
            .map_err(|_| "invalid_ledger_balance")?,
    );
    Ok(
        json!({"did":did,"account_id":hex(&id),"asset_id":asset_text,"owner":hex(&owner),
        "next_sequence":next.to_string(),"balance":balance.to_string(),"height":height,
        "evidence":"committed_eth_call"}),
    )
}

pub fn account_document(
    evm_address: Option<[u8; 20]>,
    pax_address: Option<String>,
    did_public_key: Option<[u8; 32]>,
    layerx_account: Option<[u8; 32]>,
) -> Value {
    json!({"evm_address":evm_address.as_ref().map(evm::address_hex),"pax_address":pax_address,
        "layerx_did":did_public_key.map(|key| format!("did:layerx:{}",hex(&key))),
        "layerx_account":layerx_account.as_ref().map(|id|hex(id)),
        "bound":evm_address.is_some() && did_public_key.is_some()})
}

pub fn balances(account: Value, height: u64, balances: Vec<Value>) -> Value {
    json!({"account":account,"height":height,"balances":balances,"units":"base",
        "evidence":"committed_eth_call"})
}

pub fn registration_cursor(text: &str) -> Result<u64, &'static str> {
    text.parse::<u64>()
        .ok()
        .filter(|value| *value > 0 && i64::try_from(*value).is_ok())
        .filter(|value| value.to_string() == text)
        .ok_or("invalid_page")
}

pub fn page_params(params: Option<&Value>) -> Result<(Option<u64>, usize), &'static str> {
    let empty = Vec::new();
    let args = match params {
        None | Some(Value::Null) => &empty,
        Some(Value::Array(args)) => args,
        _ => return Err("invalid_page"),
    };
    if args.len() > 2 {
        return Err("invalid_page");
    }
    let cursor = match args.first() {
        None | Some(Value::Null) => None,
        Some(Value::String(cursor)) => Some(registration_cursor(cursor)?),
        _ => return Err("invalid_page"),
    };
    let limit = match args.get(1) {
        None => ASSET_PAGE_LIMIT,
        Some(value) => value
            .as_u64()
            .and_then(|v| usize::try_from(v).ok())
            .filter(|v| (1..=ASSET_PAGE_LIMIT).contains(v))
            .ok_or("invalid_page")?,
    };
    Ok((cursor, limit))
}

pub fn assets_page(
    assets: Vec<Value>,
    height: u64,
    cursor: Option<u64>,
    limit: usize,
    next_cursor: Option<u64>,
) -> Result<Value, &'static str> {
    if !(1..=ASSET_PAGE_LIMIT).contains(&limit) || assets.len() > limit {
        return Err("invalid_page");
    }
    if let Some(cursor) = cursor {
        registration_cursor(&cursor.to_string())?;
    }
    if let Some(next) = next_cursor {
        registration_cursor(&next.to_string())?;
        if next <= cursor.unwrap_or(0) || assets.len() != limit {
            return Err("invalid_page");
        }
    }
    let mut seen = std::collections::BTreeSet::new();
    for asset in &assets {
        let id = key(asset["asset_id"].as_str().ok_or("invalid_asset")?)?;
        if !seen.insert(id) {
            return Err("duplicate_asset");
        }
    }
    Ok(json!({"height":height,"assets":assets,
        "next_cursor":next_cursor.map(|cursor| cursor.to_string()),
        "joined_limit":ASSET_PAGE_LIMIT,"order":"registration",
        "units":"base","evidence":"committed_eth_call"}))
}

pub fn ledger_asset_metadata(answer: &[u8], expected: [u8; 32]) -> Result<Value, &'static str> {
    let asset = evm::decode_ledger_asset(answer).map_err(|_| "invalid_ledger_asset")?;
    if asset.asset_id != expected {
        return Err("invalid_ledger_asset");
    }
    Ok(
        json!({"name":asset.name,"symbol":asset.symbol,"decimals":asset.decimals,
        "supply":asset.total_units.to_string(),"total_units":asset.total_units.to_string(),
        "supply_cap":asset.supply_cap.to_string(),"paused":asset.paused,
        "issuer_did":hex(&asset.issuer_did),"salt":hex(&asset.salt),"issuer_kind":asset.issuer_kind,
        "custody_kind":asset.custody_kind,"custody_reference":hex(&asset.custody_reference)}),
    )
}
