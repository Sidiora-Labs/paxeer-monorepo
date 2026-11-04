//! Paxeer chain reads. Each public read crosses the sequencer JSON-RPC transport
//! again; this endpoint retains neither account state nor a cached network head.

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
    pub balance: Uint256,
    pub nonce: u64,
    pub height: u64,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxAccountJoin {
    pub account: PxResolvedIdentities,
    pub paxeer: Option<PxPaxeerAccount>,
    pub layerx: Option<Map<String, Value>>,
}

/// An exact unsigned on-chain quantity. Construction rejects overflow and noncanonical input.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Uint256([u8; 32]);

impl std::fmt::Display for Uint256 {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.pad_integral(true, "", &self.decimal())
    }
}

impl Uint256 {
    pub const fn bytes(self) -> [u8; 32] {
        self.0
    }

    pub fn decimal(self) -> String {
        let mut value = self.0;
        let mut digits = Vec::new();
        while value != [0; 32] {
            let mut remainder = 0_u16;
            for byte in &mut value {
                let part = remainder * 256 + u16::from(*byte);
                *byte = (part / 10) as u8;
                remainder = part % 10;
            }
            digits.push((b'0' + remainder as u8) as char);
        }
        if digits.is_empty() {
            return "0".to_owned();
        }
        digits.into_iter().rev().collect()
    }

    pub fn from_decimal(text: &str) -> Option<Self> {
        if text.is_empty()
            || text.len() > 78
            || (text.len() > 1 && text.starts_with('0'))
            || !text.bytes().all(|b| b.is_ascii_digit())
        {
            return None;
        }
        let mut out = [0_u8; 32];
        for digit in text.bytes() {
            let mut carry = u16::from(digit - b'0');
            for byte in out.iter_mut().rev() {
                let part = u16::from(*byte) * 10 + carry;
                *byte = part as u8;
                carry = part >> 8;
            }
            if carry != 0 {
                return None;
            }
        }
        Some(Self(out))
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxAssetBalance {
    pub denom: String,
    pub amount: Uint256,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxAccountBalances {
    pub account: PxResolvedIdentities,
    pub height: u64,
    pub balances: Vec<PxAssetBalance>,
}

/// Flat metadata returned by the gateway. Optional metadata remains explicitly absent.
/// `verification` is the producer's label, not a client-verified proof.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxAssetEntry {
    pub asset_id: String,
    pub kind: String,
    pub denom: Option<String>,
    pub name: Option<String>,
    pub symbol: Option<String>,
    pub decimals: Option<u8>,
    pub supply: Option<Uint256>,
    pub paused: Option<bool>,
    pub verification: Option<String>,
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct PxAssetTable {
    pub assets: Vec<PxAssetEntry>,
    pub height: u64,
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
        if result.get("consensus") == Some(&Value::Null)
            && result.get("reason").and_then(Value::as_str) == Some("consensus_unavailable")
        {
            return Err(SequencerError::Unavailable);
        }
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

fn quantity(value: &Value) -> Option<Uint256> {
    let text = value.as_str()?;
    if let Some(hex) = text.strip_prefix("0x") {
        if hex.is_empty()
            || hex.len() > 64
            || (hex.len() > 1 && hex.starts_with('0'))
            || !hex.bytes().all(|b| b.is_ascii_hexdigit())
        {
            return None;
        }
        let padded = format!("{hex:0>64}");
        fixed(&padded).map(Uint256)
    } else {
        Uint256::from_decimal(text)
    }
}

fn count(value: &Value) -> Option<u64> {
    if let Some(number) = value.as_u64() {
        return Some(number);
    }
    let bytes = quantity(value)?.bytes();
    if bytes[..24] != [0; 24] {
        return None;
    }
    Some(u64::from_be_bytes(bytes[24..].try_into().ok()?))
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
                height: half.get("height")?.as_u64()?,
            })
        })?,
        layerx: match fields.get("layerx") {
            None => None,
            Some(value) => nullable(value, object)?,
        },
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
            if row.as_object()?.len() != 2 {
                return None;
            }
            Some(PxAssetBalance {
                denom: text(row.get("denom")?)?,
                amount: Uint256::from_decimal(row.get("amount")?.as_str()?)?,
            })
        })
        .collect::<Option<Vec<_>>>()?;
    if balances.len() > 256 {
        return None;
    }
    Some(PxAccountBalances {
        account: identities(value.get("account")?)?,
        height: value.get("height")?.as_u64()?,
        balances,
    })
}

fn decode_assets(value: &Value) -> Option<PxAssetTable> {
    let assets = rows(value, "assets")?
        .iter()
        .map(|entry| {
            Some(PxAssetEntry {
                asset_id: text(entry.get("asset_id")?)?,
                kind: text(entry.get("kind")?)?,
                denom: nullable(entry.get("denom")?, text)?,
                name: nullable(entry.get("name")?, |v| v.as_str().map(str::to_owned))?,
                symbol: nullable(entry.get("symbol")?, |v| v.as_str().map(str::to_owned))?,
                decimals: nullable(entry.get("decimals")?, |v| u8::try_from(v.as_u64()?).ok())?,
                supply: nullable(entry.get("supply")?, |v| Uint256::from_decimal(v.as_str()?))?,
                paused: entry.get("paused").map(Value::as_bool).transpose_option()?,
                verification: entry
                    .get("verification")
                    .map(|v| {
                        (v.as_str()? == "authenticated_committed_snapshot")
                            .then(|| "authenticated_committed_snapshot".to_owned())
                    })
                    .transpose_option()?,
            })
        })
        .collect::<Option<Vec<_>>>()?;
    let joined_limit = value.get("joined_limit")?.as_u64()?;
    if joined_limit != 16 || assets.len() > 16 {
        return None;
    }
    Some(PxAssetTable {
        assets,
        height: value.get("height")?.as_u64()?,
        joined_limit,
    })
}

trait TransposeOption<T> {
    fn transpose_option(self) -> Option<Option<T>>;
}
impl<T> TransposeOption<T> for Option<Option<T>> {
    fn transpose_option(self) -> Option<Option<T>> {
        match self {
            None => Some(None),
            Some(value) => value.map(Some),
        }
    }
}

fn decode_network(value: &Value) -> Option<PxNetworkHead> {
    let fields = value.as_object()?;
    if fields.contains_key("anchor") || fields.keys().any(|key| key.starts_with("lx_")) {
        return None;
    }
    let paxeer = fields.get("paxeer")?;
    let latest_block = count(paxeer.get("latest_block")?)?;
    let consensus = fields.get("consensus")?;
    let latest_committed_height = count(consensus.get("latest_height")?)?;
    if latest_committed_height > latest_block {
        return None;
    }
    Some(PxNetworkHead {
        network_id: text(fields.get("network_id")?)?,
        paxeer: PxPaxeerHead {
            chain_id: count(paxeer.get("chain_id")?)?,
            latest_block,
        },
        layerx: match fields.get("layerx") {
            Some(value) => nullable(value, object)?,
            None => None,
        },
        latest_committed_height,
    })
}
