//! Paxeer chain reads.

use serde_json::{json, Map, Value};

use crate::sequencer::{SequencerEndpoint, SequencerError};

#[derive(Debug)]
pub struct PxEndpoint(SequencerEndpoint);

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxResolvedIdentities {
    pub evm_address: Option<[u8; 20]>,
    pub pax_address: Option<String>,
    pub layerx_did: Option<String>,
    pub layerx_account: Option<[u8; 32]>,
    pub bound: bool,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxPaxeerAccount {
    pub address: [u8; 20],
    pub balance: u128,
    pub nonce: u64,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxAccountJoin {
    pub account: PxResolvedIdentities,
    pub paxeer: Option<PxPaxeerAccount>,
    pub layerx: Option<Map<String, Value>>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxCustodyAsset {
    pub asset_id: [u8; 32],
    pub denom: String,
    pub pointer: [u8; 20],
    pub enabled: bool,
    pub paused: bool,
    pub minimum_deposit: u128,
    pub custody_cap: u128,
    pub custodied: u128,
    pub released: u128,
    pub pending: u128,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxPaxeerAssetBalance {
    pub denom: String,
    pub amount: u128,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxAssetBalance {
    pub asset_id: [u8; 32],
    pub denom: Option<String>,
    pub custody: Option<PxCustodyAsset>,
    pub paxeer: Option<PxPaxeerAssetBalance>,
    pub layerx: Option<Map<String, Value>>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxAccountBalances {
    pub account: PxResolvedIdentities,
    pub balances: Vec<PxAssetBalance>,
    pub joined_limit: u64,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxAssetEntry {
    pub asset_id: [u8; 32],
    pub layerx: Option<Map<String, Value>>,
    pub paxeer: Option<PxCustodyAsset>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxAssetTable {
    pub assets: Vec<PxAssetEntry>,
    pub joined_limit: u64,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxPaxeerHead {
    pub chain_id: u64,
    pub latest_block: u64,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxNetworkHead {
    pub network_id: String,
    pub paxeer: PxPaxeerHead,
    pub layerx: Option<Map<String, Value>>,
    pub latest_committed_height: u64,
}

impl From<SequencerEndpoint> for PxEndpoint {
    fn from(endpoint: SequencerEndpoint) -> Self {
        Self(endpoint)
    }
}

impl PxEndpoint {
    pub fn new(url: &str, trust_anchors: &[&[u8]]) -> Result<Self, SequencerError> {
        SequencerEndpoint::new(url, trust_anchors).map(Self)
    }

    pub fn account(&self, account: &str) -> Result<PxAccountJoin, SequencerError> {
        let result = self.0.call_raw("px_getAccount", &json!([account]))?;
        decode_account(&result).ok_or(SequencerError::Malformed)
    }

    pub fn balances(&self, account: &str) -> Result<PxAccountBalances, SequencerError> {
        let result = self.0.call_raw("px_getBalances", &json!([account]))?;
        decode_balances(&result).ok_or(SequencerError::Malformed)
    }

    pub fn list_assets(&self) -> Result<PxAssetTable, SequencerError> {
        let result = self.0.call_raw("px_listAssets", &json!([]))?;
        decode_assets(&result).ok_or(SequencerError::Malformed)
    }

    pub fn network(&self) -> Result<PxNetworkHead, SequencerError> {
        let result = self.0.call_raw("px_getNetwork", &json!([]))?;
        decode_network(&result).ok_or(SequencerError::Malformed)
    }
}

fn nullable<T>(value: &Value, decode: impl FnOnce(&Value) -> Option<T>) -> Option<Option<T>> {
    if value.is_null() {
        Some(None)
    } else {
        decode(value).map(Some)
    }
}

fn text(value: &Value) -> Option<String> {
    let text = value.as_str()?;
    (!text.is_empty() && text.len() <= 128).then(|| text.to_owned())
}

fn fixed<const N: usize>(text: &str) -> Option<[u8; N]> {
    if text.len() != N * 2 || !text.bytes().all(|byte| byte.is_ascii_hexdigit()) {
        return None;
    }
    let mut bytes = [0; N];
    for (byte, pair) in bytes.iter_mut().zip(text.as_bytes().chunks_exact(2)) {
        *byte = u8::from_str_radix(std::str::from_utf8(pair).ok()?, 16).ok()?;
    }
    Some(bytes)
}

fn address(value: &Value) -> Option<[u8; 20]> {
    fixed(value.as_str()?.strip_prefix("0x")?)
}

fn asset_id(value: &Value) -> Option<[u8; 32]> {
    fixed(value.as_str()?)
}

fn quantity(value: &Value) -> Option<u128> {
    if let Some(number) = value.as_u64() {
        return Some(u128::from(number));
    }
    let text = value.as_str()?;
    if let Some(hex) = text.strip_prefix("0x") {
        if hex.is_empty() || hex.len() > 32 || !hex.bytes().all(|byte| byte.is_ascii_hexdigit()) {
            return None;
        }
        u128::from_str_radix(hex, 16).ok()
    } else if text == "0"
        || (!text.starts_with('0') && !text.is_empty() && text.bytes().all(|b| b.is_ascii_digit()))
    {
        text.parse().ok()
    } else {
        None
    }
}

fn count(value: &Value) -> Option<u64> {
    u64::try_from(quantity(value)?).ok()
}

fn object(value: &Value) -> Option<Map<String, Value>> {
    value.as_object().cloned()
}

fn identities(value: &Value) -> Option<PxResolvedIdentities> {
    let fields = value.as_object()?;
    let layerx_did = nullable(fields.get("layerx_did")?, |value| {
        let did = value.as_str()?;
        fixed::<32>(did.strip_prefix("did:layerx:")?)?;
        Some(did.to_owned())
    })?;
    Some(PxResolvedIdentities {
        evm_address: nullable(fields.get("evm_address")?, address)?,
        pax_address: nullable(fields.get("pax_address")?, text)?,
        layerx_did,
        layerx_account: nullable(fields.get("layerx_account")?, asset_id)?,
        bound: fields.get("bound")?.as_bool()?,
    })
}

fn decode_account(value: &Value) -> Option<PxAccountJoin> {
    let fields = value.as_object()?;
    Some(PxAccountJoin {
        account: identities(fields.get("account")?)?,
        paxeer: nullable(fields.get("paxeer")?, |half| {
            Some(PxPaxeerAccount {
                address: address(half.get("address")?)?,
                balance: quantity(half.get("balance")?)?,
                nonce: count(half.get("nonce")?)?,
            })
        })?,
        layerx: nullable(fields.get("layerx")?, object)?,
    })
}

fn custody(value: &Value) -> Option<PxCustodyAsset> {
    Some(PxCustodyAsset {
        asset_id: asset_id(value.get("asset_id")?)?,
        denom: text(value.get("denom")?)?,
        pointer: address(value.get("pointer")?)?,
        enabled: value.get("enabled")?.as_bool()?,
        paused: value.get("paused")?.as_bool()?,
        minimum_deposit: quantity(value.get("minimum_deposit")?)?,
        custody_cap: quantity(value.get("custody_cap")?)?,
        custodied: quantity(value.get("custodied")?)?,
        released: quantity(value.get("released")?)?,
        pending: quantity(value.get("pending")?)?,
    })
}

fn rows<'a>(value: &'a Value, name: &str) -> Option<&'a Vec<Value>> {
    let rows = value.get(name)?.as_array()?;
    (rows.len() <= 1024).then_some(rows)
}

fn decode_balances(value: &Value) -> Option<PxAccountBalances> {
    let balances = rows(value, "balances")?
        .iter()
        .map(|row| {
            Some(PxAssetBalance {
                asset_id: asset_id(row.get("asset_id")?)?,
                denom: nullable(row.get("denom")?, text)?,
                custody: nullable(row.get("custody")?, custody)?,
                paxeer: nullable(row.get("paxeer")?, |half| {
                    Some(PxPaxeerAssetBalance {
                        denom: text(half.get("denom")?)?,
                        amount: quantity(half.get("amount")?)?,
                    })
                })?,
                layerx: nullable(row.get("layerx")?, object)?,
            })
        })
        .collect::<Option<Vec<_>>>()?;
    Some(PxAccountBalances {
        account: identities(value.get("account")?)?,
        balances,
        joined_limit: count(value.get("joined_limit")?)?,
    })
}

fn decode_assets(value: &Value) -> Option<PxAssetTable> {
    let assets = rows(value, "assets")?
        .iter()
        .map(|entry| {
            Some(PxAssetEntry {
                asset_id: asset_id(entry.get("asset_id")?)?,
                layerx: nullable(entry.get("layerx")?, object)?,
                paxeer: nullable(entry.get("paxeer")?, custody)?,
            })
        })
        .collect::<Option<Vec<_>>>()?;
    Some(PxAssetTable {
        assets,
        joined_limit: count(value.get("joined_limit")?)?,
    })
}

fn decode_network(value: &Value) -> Option<PxNetworkHead> {
    let fields = value.as_object()?;
    if fields.contains_key("anchor") || fields.keys().any(|key| key.starts_with("lx_")) {
        return None;
    }
    let paxeer = fields.get("paxeer")?;
    let latest_block = count(paxeer.get("latest_block")?)?;
    let latest_committed_height = count(fields.get("latest_committed_height")?)?;
    if latest_committed_height > latest_block {
        return None;
    }
    Some(PxNetworkHead {
        network_id: text(fields.get("network_id")?)?,
        paxeer: PxPaxeerHead {
            chain_id: count(paxeer.get("chain_id")?)?,
            latest_block,
        },
        layerx: nullable(fields.get("layerx")?, object)?,
        latest_committed_height,
    })
}
