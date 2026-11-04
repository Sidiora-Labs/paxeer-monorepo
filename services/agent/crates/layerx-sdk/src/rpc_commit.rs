//! Comet committed-block authority and sequence-batch coverage for node receipts.
use crate::rpc::{RpcClient, RpcError};
use serde_json::Value;
use sha2::{Digest as _, Sha256};

#[derive(Clone)]
pub struct NodeCommitAuthority {
    chain_id: String,
    validators: Vec<([u8; 32], u64)>,
    first_height: u64,
    last_height: u64,
}
impl NodeCommitAuthority {
    /// Pins the chain and validator set independently of the RPC response.
    pub fn new(
        chain_id: String,
        validators: Vec<([u8; 32], u64)>,
        first_height: u64,
        last_height: u64,
    ) -> Result<Self, RpcError> {
        let mut total = 0_u64;
        if chain_id.is_empty()
            || chain_id.len() > 128
            || validators.is_empty()
            || validators.len() > 1024
            || first_height == 0
            || first_height > last_height
        {
            return Err(RpcError::InvalidRequest);
        }
        for (index, (key, power)) in validators.iter().enumerate() {
            if *key == [0; 32]
                || *power == 0
                || validators[..index].iter().any(|(prior, _)| prior == key)
            {
                return Err(RpcError::InvalidRequest);
            }
            total = total
                .checked_add(*power)
                .filter(|v| *v <= (i64::MAX / 8) as u64)
                .ok_or(RpcError::InvalidRequest)?;
        }
        Ok(Self {
            chain_id,
            validators,
            first_height,
            last_height,
        })
    }
}

#[derive(Clone)]
pub struct NodeTransactionProof {
    pub total: u64,
    pub index: u64,
    pub aunts: Vec<[u8; 32]>,
}
#[derive(Clone)]
pub struct NodeCommitCoverage {
    pub covering_height: u64,
    pub batch_transaction: Vec<u8>,
    pub transaction_proof: NodeTransactionProof,
    pub activity_ids: Vec<[u8; 32]>,
}
impl RpcClient {
    /// Supplies raw sequence-batch inclusion material and independently pinned Comet trust.
    /// The live signed header is still acquired and authenticated by `wait_for`.
    pub fn with_node_commit_authority(
        mut self,
        authority: NodeCommitAuthority,
        coverage: NodeCommitCoverage,
    ) -> Result<Self, RpcError> {
        if coverage.covering_height < authority.first_height
            || coverage.covering_height > authority.last_height
            || coverage.activity_ids.is_empty()
            || coverage.activity_ids.len() > 65_536
            || coverage.batch_transaction.len() != 241
            || coverage.transaction_proof.aunts.len() > 64
        {
            return Err(RpcError::InvalidRequest);
        }
        self.node_commit = Some((authority, coverage));
        Ok(self)
    }
}

fn number(v: &Value) -> Result<u64, RpcError> {
    match v {
        Value::String(s)
            if !s.is_empty()
                && (s == "0" || !s.starts_with('0'))
                && s.bytes().all(|b| b.is_ascii_digit()) =>
        {
            s.parse().map_err(|_| RpcError::InvalidResponse)
        }
        Value::Number(n) => n.as_u64().ok_or(RpcError::InvalidResponse),
        _ => Err(RpcError::InvalidResponse),
    }
}
fn text(v: &Value) -> Result<&str, RpcError> {
    v.as_str().ok_or(RpcError::InvalidResponse)
}
fn hex<const N: usize>(v: &Value) -> Result<[u8; N], RpcError> {
    let s = text(v)?;
    let s = s.strip_prefix("0x").unwrap_or(s);
    if s.len() != N * 2 {
        return Err(RpcError::InvalidResponse);
    }
    let mut bytes = [0; N];
    for (target, pair) in bytes.iter_mut().zip(s.as_bytes().chunks_exact(2)) {
        *target = u8::from_str_radix(
            std::str::from_utf8(pair).map_err(|_| RpcError::InvalidResponse)?,
            16,
        )
        .map_err(|_| RpcError::InvalidResponse)?;
    }
    Ok(bytes)
}
fn base64(v: &Value) -> Result<Vec<u8>, RpcError> {
    let s = text(v)?.as_bytes();
    if s.len() > 4096 || s.len() % 4 != 0 {
        return Err(RpcError::InvalidResponse);
    }
    let mut bytes = Vec::new();
    for (index, chunk) in s.chunks_exact(4).enumerate() {
        let mut word = 0_u32;
        let mut padding = 0;
        for byte in chunk {
            let n = match byte {
                b'A'..=b'Z' => byte - b'A',
                b'a'..=b'z' => byte - b'a' + 26,
                b'0'..=b'9' => byte - b'0' + 52,
                b'+' => 62,
                b'/' => 63,
                b'=' => {
                    padding += 1;
                    0
                }
                _ => return Err(RpcError::InvalidResponse),
            };
            if padding > 0 && *byte != b'=' {
                return Err(RpcError::InvalidResponse);
            }
            word = word << 6 | u32::from(n);
        }
        if padding > 2
            || (padding > 0 && index + 1 != s.len() / 4)
            || (padding == 1 && word & 0xff != 0)
            || (padding == 2 && word & 0xffff != 0)
        {
            return Err(RpcError::InvalidResponse);
        }
        bytes.extend_from_slice(&word.to_be_bytes()[1..4 - padding]);
    }
    Ok(bytes)
}
fn varint(mut n: u64, out: &mut Vec<u8>) {
    while n >= 128 {
        out.push((n as u8 & 127) | 128);
        n >>= 7;
    }
    out.push(n as u8);
}
fn scalar(field: u64, n: u64) -> Vec<u8> {
    let mut out = Vec::new();
    if n != 0 {
        varint(field << 3, &mut out);
        varint(n, &mut out);
    }
    out
}
fn blob(field: u64, bytes: &[u8]) -> Vec<u8> {
    let mut out = Vec::new();
    varint(field << 3 | 2, &mut out);
    varint(bytes.len() as u64, &mut out);
    out.extend_from_slice(bytes);
    out
}
fn root(items: &[Vec<u8>]) -> [u8; 32] {
    match items {
        [] => Sha256::digest([]).into(),
        [one] => Sha256::new()
            .chain_update([0])
            .chain_update(one)
            .finalize()
            .into(),
        _ => {
            let split = 1_usize << ((usize::BITS - 1) - (items.len() - 1).leading_zeros());
            Sha256::new()
                .chain_update([1])
                .chain_update(root(&items[..split]))
                .chain_update(root(&items[split..]))
                .finalize()
                .into()
        }
    }
}
fn timestamp(v: &Value) -> Result<Vec<u8>, RpcError> {
    let s = text(v)?;
    let digits = s.strip_suffix('Z').ok_or(RpcError::InvalidResponse)?;
    let (date, fraction) = digits.split_once('.').map_or((digits, ""), |(a, b)| (a, b));
    if !date.is_ascii()
        || date.len() != 19
        || &date[4..5] != "-"
        || &date[7..8] != "-"
        || &date[10..11] != "T"
        || &date[13..14] != ":"
        || &date[16..17] != ":"
        || fraction.len() > 9
        || !fraction.bytes().all(|b| b.is_ascii_digit())
    {
        return Err(RpcError::InvalidResponse);
    }
    let n = |start: usize, end: usize| {
        date[start..end]
            .parse::<i64>()
            .map_err(|_| RpcError::InvalidResponse)
    };
    let (year, month, day, hour, minute, second) = (
        n(0, 4)?,
        n(5, 7)?,
        n(8, 10)?,
        n(11, 13)?,
        n(14, 16)?,
        n(17, 19)?,
    );
    let leap = year % 4 == 0 && (year % 100 != 0 || year % 400 == 0);
    let days = [
        31,
        if leap { 29 } else { 28 },
        31,
        30,
        31,
        30,
        31,
        31,
        30,
        31,
        30,
        31,
    ];
    if !(1..=9999).contains(&year)
        || !(1..=12).contains(&month)
        || day < 1
        || day > days[(month - 1) as usize]
        || !(0..24).contains(&hour)
        || !(0..60).contains(&minute)
        || !(0..60).contains(&second)
    {
        return Err(RpcError::InvalidResponse);
    }
    let y = year - i64::from(month <= 2);
    let era = y / 400;
    let yoe = y - era * 400;
    let m = month + if month > 2 { -3 } else { 9 };
    let doy = (153 * m + 2) / 5 + day - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    let seconds = (era * 146097 + doe - 719468) * 86400 + hour * 3600 + minute * 60 + second;
    let nanos = if fraction.is_empty() {
        0
    } else {
        fraction
            .parse::<u64>()
            .map_err(|_| RpcError::InvalidResponse)?
            * 10_u64.pow((9 - fraction.len()) as u32)
    };
    let mut out = scalar(1, seconds as u64);
    out.extend(scalar(2, nanos));
    Ok(out)
}
fn block_id(v: &Value) -> Result<Vec<u8>, RpcError> {
    let total = number(&v["parts"]["total"])?;
    if total == 0 || total > 101 {
        return Err(RpcError::InvalidResponse);
    }
    let mut parts = scalar(1, total);
    parts.extend(blob(2, &hex::<32>(&v["parts"]["hash"])?));
    let mut out = blob(1, &hex::<32>(&v["hash"])?);
    out.extend(blob(2, &parts));
    Ok(out)
}
fn header_hash(h: &Value) -> Result<[u8; 32], RpcError> {
    let mut version = scalar(1, number(&h["version"]["block"])?);
    version.extend(scalar(2, number(&h["version"]["app"])?));
    let mut fields = vec![
        version,
        blob(1, text(&h["chain_id"])?.as_bytes()),
        scalar(1, number(&h["height"])?),
        timestamp(&h["time"])?,
        block_id(&h["last_block_id"])?,
    ];
    for field in [
        "last_commit_hash",
        "data_hash",
        "validators_hash",
        "next_validators_hash",
        "consensus_hash",
        "app_hash",
        "last_results_hash",
        "evidence_hash",
    ] {
        fields.push(blob(1, &hex::<32>(&h[field])?));
    }
    fields.push(blob(1, &hex::<20>(&h["proposer_address"])?));
    Ok(root(&fields))
}
fn inclusion(proof: &NodeTransactionProof, raw: &[u8]) -> Result<[u8; 32], RpcError> {
    fn fold(
        total: u64,
        index: u64,
        leaf: [u8; 32],
        aunts: &[[u8; 32]],
    ) -> Result<[u8; 32], RpcError> {
        if total == 0 || index >= total {
            return Err(RpcError::Verification);
        }
        if total == 1 {
            return if aunts.is_empty() {
                Ok(leaf)
            } else {
                Err(RpcError::Verification)
            };
        }
        let (last, rest) = aunts.split_last().ok_or(RpcError::Verification)?;
        let split = 1_u64 << (63 - (total - 1).leading_zeros());
        let (left, right) = if index < split {
            (fold(split, index, leaf, rest)?, *last)
        } else {
            (*last, fold(total - split, index - split, leaf, rest)?)
        };
        Ok(Sha256::new()
            .chain_update([1])
            .chain_update(left)
            .chain_update(right)
            .finalize()
            .into())
    }
    if proof.index != 0 || proof.aunts.len() > 64 {
        return Err(RpcError::Verification);
    }
    fold(
        proof.total,
        proof.index,
        Sha256::new()
            .chain_update([0])
            .chain_update(Sha256::digest(raw))
            .finalize()
            .into(),
        &proof.aunts,
    )
}

pub(super) fn verify_committed(
    signed: &Value,
    receipt: &layerx_wire::receipt::ProtocolReceipt,
    execution_height: u64,
    expected_height: u64,
    expected_hash: [u8; 32],
    authority: &NodeCommitAuthority,
    coverage: &NodeCommitCoverage,
    pin: &layerx_proof::inclusion::SequencerAuthorization,
) -> Result<(), RpcError> {
    let h = &signed["header"];
    let c = &signed["commit"];
    if expected_height > i64::MAX as u64
        || expected_height < authority.first_height
        || expected_height > authority.last_height
        || expected_height != coverage.covering_height
        || expected_height < execution_height
        || number(&h["height"])? != expected_height
        || text(&h["chain_id"])? != authority.chain_id
        || number(&c["height"])? != expected_height
        || number(&c["round"])? > i32::MAX as u64
        || hex::<32>(&c["block_id"]["hash"])? != expected_hash
        || header_hash(h)? != expected_hash
    {
        return Err(RpcError::Verification);
    }
    let mut validator_bytes = Vec::new();
    let mut total = 0_u64;
    for (key, power) in &authority.validators {
        let mut simple = blob(1, &blob(1, key));
        simple.extend(scalar(2, *power));
        validator_bytes.push(simple);
        total += power;
    }
    if hex::<32>(&h["validators_hash"])? != root(&validator_bytes) {
        return Err(RpcError::Verification);
    }
    let signatures = c["signatures"]
        .as_array()
        .filter(|v| v.len() == authority.validators.len())
        .ok_or(RpcError::InvalidResponse)?;
    let mut seen = std::collections::BTreeSet::new();
    let mut signed_power = 0_u64;
    for signature in signatures {
        let flag = number(&signature["block_id_flag"])?;
        if flag == 1 {
            if signature["validator_address"] != ""
                || !signature["signature"].is_null()
                || signature["timestamp"] != "0001-01-01T00:00:00Z"
            {
                return Err(RpcError::Verification);
            }
            continue;
        }
        if !matches!(flag, 2 | 3) {
            return Err(RpcError::InvalidResponse);
        }
        let address = hex::<20>(&signature["validator_address"])?;
        let (key, power) = authority
            .validators
            .iter()
            .find(|(key, _)| Sha256::digest(key)[..20] == address)
            .ok_or(RpcError::Verification)?;
        if !seen.insert(address) {
            return Err(RpcError::Verification);
        }
        let mut vote = scalar(1, 2);
        vote.push(0x11);
        vote.extend_from_slice(&(expected_height as i64).to_le_bytes());
        let round = number(&c["round"])?;
        if round != 0 {
            vote.push(0x19);
            vote.extend_from_slice(&(round as i64).to_le_bytes());
        }
        if flag == 2 {
            vote.extend(blob(4, &block_id(&c["block_id"])?));
        }
        vote.extend(blob(5, &timestamp(&signature["timestamp"])?));
        vote.extend(blob(6, authority.chain_id.as_bytes()));
        let mut message = Vec::new();
        varint(vote.len() as u64, &mut message);
        message.extend(vote);
        let signature: [u8; 64] = base64(&signature["signature"])?
            .try_into()
            .map_err(|_| RpcError::InvalidResponse)?;
        layerx_crypto::ed25519::verify_message(key, &signature, &message)
            .map_err(|_| RpcError::Verification)?;
        if flag == 2 {
            signed_power = signed_power
                .checked_add(*power)
                .ok_or(RpcError::Verification)?;
        }
    }
    if u128::from(signed_power) * 3 <= u128::from(total) * 2 {
        return Err(RpcError::Verification);
    }
    if inclusion(&coverage.transaction_proof, &coverage.batch_transaction)?
        != hex::<32>(&h["data_hash"])?
    {
        return Err(RpcError::Verification);
    }
    let batch =
        layerx_client::rpc::receipt_commit::SequenceBatch::decode(&coverage.batch_transaction)
            .map_err(|_| RpcError::Verification)?;
    if batch.height != expected_height
        || batch.key_id != pin.sequencer_id()
        || batch.height < pin.first_batch_number()
        || batch.height > pin.last_batch_number()
        || receipt.global_sequence() < batch.from_sequence
        || receipt.global_sequence() > batch.to_sequence
        || batch
            .to_sequence
            .checked_sub(batch.from_sequence)
            .and_then(|n| n.checked_add(1))
            != Some(coverage.activity_ids.len() as u64)
    {
        return Err(RpcError::Verification);
    }
    layerx_crypto::ed25519::verify_digest(
        &pin.public_key(),
        &batch.signature,
        &batch
            .signing_digest(&authority.chain_id)
            .map_err(|_| RpcError::Verification)?,
    )
    .map_err(|_| RpcError::Verification)?;
    let mut activity_hash = Sha256::new();
    let mut state = batch.previous_root;
    for (offset, id) in coverage.activity_ids.iter().enumerate() {
        activity_hash.update(id);
        let sequence = batch
            .from_sequence
            .checked_add(offset as u64)
            .ok_or(RpcError::Verification)?;
        let previous = state;
        state = Sha256::new()
            .chain_update(b"PAXEER/v1/sequence\0")
            .chain_update(previous)
            .chain_update(sequence.to_be_bytes())
            .chain_update(id)
            .finalize()
            .into();
        if sequence == receipt.global_sequence()
            && (*id != receipt.activity_id()
                || previous != receipt.previous_state_root()
                || state != receipt.resulting_state_root())
        {
            return Err(RpcError::Verification);
        }
    }
    if state != batch.resulting_root
        || <[u8; 32]>::from(activity_hash.finalize()) != batch.activity_root
    {
        return Err(RpcError::Verification);
    }
    Ok(())
}
