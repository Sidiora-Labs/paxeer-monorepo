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
    /// # Errors
    ///
    /// Returns `InvalidRequest` for an empty or oversized chain ID or validator set, invalid height range, duplicate or zero keys, zero power, or total power above the Comet bound.
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
                .filter(|v| *v <= (i64::MAX / 8).unsigned_abs())
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
    /// Pins Comet trust; each committed wait acquires its own sequence-batch coverage.
    /// # Errors
    ///
    /// This setter is infallible; subsequent committed waits may refuse evidence outside the pinned authority.
    #[allow(
        clippy::unnecessary_wraps,
        reason = "Preserve the public fallible builder signature used by committed-wait clients."
    )]
    pub fn with_node_commit_authority(
        mut self,
        authority: NodeCommitAuthority,
    ) -> Result<Self, RpcError> {
        self.node_commit = Some((authority, NodeCommitCoverage::automatic()));
        Ok(self)
    }

    /// Overrides served coverage for debug/test fixtures. Comet trust is still required.
    /// # Errors
    ///
    /// Returns `MissingFinalityTrust` without pinned authority, or `InvalidRequest` for coverage outside the height, transaction, activity or proof bounds.
    #[cfg(debug_assertions)]
    #[doc(hidden)]
    pub fn with_node_commit_coverage_override(
        mut self,
        coverage: NodeCommitCoverage,
    ) -> Result<Self, RpcError> {
        let (authority, stored) = self
            .node_commit
            .as_mut()
            .ok_or(RpcError::MissingFinalityTrust)?;
        if coverage.covering_height < authority.first_height
            || coverage.covering_height > authority.last_height
            || coverage.activity_ids.is_empty()
            || coverage.activity_ids.len() > 65_536
            || coverage.batch_transaction.len() != 241
            || coverage.transaction_proof.aunts.len() > 64
        {
            return Err(RpcError::InvalidRequest);
        }
        *stored = coverage;
        Ok(self)
    }
}

impl NodeCommitCoverage {
    // Height zero cannot be valid coverage and denotes acquisition per wait in
    // RpcClient's existing authority/coverage storage slot.
    fn automatic() -> Self {
        Self {
            covering_height: 0,
            batch_transaction: Vec::new(),
            transaction_proof: NodeTransactionProof {
                total: 0,
                index: 0,
                aunts: Vec::new(),
            },
            activity_ids: Vec::new(),
        }
    }

    pub(super) fn is_override(&self) -> bool {
        self.covering_height != 0
    }

    pub(super) fn from_sequence_batch(
        value: &Value,
        height: u64,
        receipt: &layerx_wire::receipt::ProtocolReceipt,
    ) -> Result<Self, RpcError> {
        if value["height"].as_str() != Some(height.to_string().as_str()) {
            return Err(RpcError::Verification);
        }
        let batch_transaction = super::node_bytes(value, "tx", 241)?;
        let batch = layerx_client::rpc::receipt_commit::SequenceBatch::decode(&batch_transaction)
            .map_err(|_| RpcError::Verification)?;
        let count = batch
            .to_sequence
            .checked_sub(batch.from_sequence)
            .and_then(|n| n.checked_add(1))
            .filter(|n| *n > 0 && *n <= 65_536)
            .ok_or(RpcError::Verification)?;
        if batch.height != height
            || receipt.global_sequence() < batch.from_sequence
            || receipt.global_sequence() > batch.to_sequence
        {
            return Err(RpcError::Verification);
        }
        // The batch envelope commits to IDs by root, rather than encoding them.
        // The batch response supplies consensus order; verify_committed authenticates
        // every ID and sequence state against the signed batch below.
        let transactions = value["activityIds"]
            .as_array()
            .filter(|ids| ids.len() as u64 == count)
            .ok_or(RpcError::Verification)?;
        let activity_ids = transactions
            .iter()
            .map(|id| {
                if !id.as_str().is_some_and(|text| text.starts_with("0x")) {
                    return Err(RpcError::Verification);
                }
                hex::<32>(id).map_err(|_| RpcError::Verification)
            })
            .collect::<Result<Vec<_>, _>>()?;
        if activity_ids[usize::try_from(receipt.global_sequence() - batch.from_sequence)
            .map_err(|_| RpcError::Verification)?]
            != receipt.activity_id()
        {
            return Err(RpcError::Verification);
        }
        let proof = &value["proof"];
        let leaf: [u8; 32] = Sha256::new()
            .chain_update([0])
            .chain_update(Sha256::digest(&batch_transaction))
            .finalize()
            .into();
        if super::node_fixed::<32>(proof, "leafHash")? != leaf {
            return Err(RpcError::Verification);
        }
        let aunts = proof["aunts"]
            .as_array()
            .filter(|aunts| aunts.len() <= 64)
            .ok_or_else(|| super::invalid_response("proof.aunts"))?
            .iter()
            .map(|aunt| hex::<32>(aunt).map_err(|_| super::invalid_response("proof.aunts")))
            .collect::<Result<Vec<_>, _>>()?;
        Ok(Self {
            covering_height: height,
            batch_transaction,
            transaction_proof: NodeTransactionProof {
                total: field(proof, "total", number)?,
                index: field(proof, "index", number)?,
                aunts,
            },
            activity_ids,
        })
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
        out.push((n.to_le_bytes()[0] & 127) | 128);
        n >>= 7;
    }
    out.push(n.to_le_bytes()[0]);
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
fn timestamp(value: &Value) -> Result<Vec<u8>, RpcError> {
    let timestamp_text = text(value)?;
    let digits = timestamp_text
        .strip_suffix('Z')
        .ok_or(RpcError::InvalidResponse)?;
    let (date, fraction) = digits.split_once('.').unwrap_or((digits, ""));
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
    let parse_component = |start: usize, end: usize| {
        date[start..end]
            .parse::<i64>()
            .map_err(|_| RpcError::InvalidResponse)
    };
    let (year, month, day, hour, minute, second) = (
        parse_component(0, 4)?,
        parse_component(5, 7)?,
        parse_component(8, 10)?,
        parse_component(11, 13)?,
        parse_component(14, 16)?,
        parse_component(17, 19)?,
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
        || day > days[usize::try_from(month - 1).map_err(|_| RpcError::InvalidResponse)?]
        || !(0..24).contains(&hour)
        || !(0..60).contains(&minute)
        || !(0..60).contains(&second)
    {
        return Err(RpcError::InvalidResponse);
    }
    let adjusted_year = year - i64::from(month <= 2);
    let era = adjusted_year / 400;
    let year_of_era = adjusted_year - era * 400;
    let shifted_month = month + if month > 2 { -3 } else { 9 };
    let day_of_year = (153 * shifted_month + 2) / 5 + day - 1;
    let day_of_era = year_of_era * 365 + year_of_era / 4 - year_of_era / 100 + day_of_year;
    let seconds =
        (era * 146_097 + day_of_era - 719_468) * 86_400 + hour * 3600 + minute * 60 + second;
    let nanos = if fraction.is_empty() {
        0
    } else {
        fraction
            .parse::<u64>()
            .map_err(|_| RpcError::InvalidResponse)?
            * 10_u64.pow(u32::try_from(9 - fraction.len()).map_err(|_| RpcError::InvalidResponse)?)
    };
    let mut out = scalar(1, u64::from_ne_bytes(seconds.to_ne_bytes()));
    out.extend(scalar(2, nanos));
    Ok(out)
}
fn field<'a, T>(
    value: &'a Value,
    name: &str,
    decode: impl FnOnce(&'a Value) -> Result<T, RpcError>,
) -> Result<T, RpcError> {
    decode(&value[name]).map_err(|error| match error {
        RpcError::InvalidResponse => super::invalid_response(name),
        other => other,
    })
}

fn block_id(v: &Value) -> Result<Vec<u8>, RpcError> {
    let total = field(&v["parts"], "total", number)?;
    if total == 0 {
        if v["hash"] != "" || v["parts"]["hash"] != "" {
            return Err(super::invalid_response("last_block_id"));
        }
        return Ok(blob(2, &[]));
    }
    if total > 101 {
        return Err(super::invalid_response("parts.total"));
    }
    let mut parts = scalar(1, total);
    parts.extend(blob(2, &field(&v["parts"], "hash", hex::<32>)?));
    let mut out = blob(1, &field(v, "hash", hex::<32>)?);
    out.extend(blob(2, &parts));
    Ok(out)
}
fn header_hash(h: &Value) -> Result<[u8; 32], RpcError> {
    let mut version = scalar(1, field(&h["version"], "block", number)?);
    version.extend(scalar(2, field(&h["version"], "app", number)?));
    let mut fields = vec![
        version,
        blob(1, field(h, "chain_id", text)?.as_bytes()),
        scalar(1, field(h, "height", number)?),
        field(h, "time", timestamp)?,
        field(h, "last_block_id", block_id)?,
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
        if matches!(field, "app_hash" | "last_results_hash") && h[field] == "" {
            fields.push(Vec::new());
        } else {
            fields.push(blob(1, &self::field(h, field, hex::<32>)?));
        }
    }
    fields.push(blob(1, &field(h, "proposer_address", hex::<20>)?));
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
    (expected_height, expected_hash): (u64, [u8; 32]),
    authority: &NodeCommitAuthority,
    coverage: &NodeCommitCoverage,
    pin: &layerx_proof::inclusion::SequencerAuthorization,
) -> Result<(), RpcError> {
    let h = &signed["header"];
    let c = &signed["commit"];
    if expected_height > i64::MAX.unsigned_abs()
        || expected_height < authority.first_height
        || expected_height > authority.last_height
        || expected_height != coverage.covering_height
        || expected_height < execution_height
        || field(h, "height", number)? != expected_height
        || field(h, "chain_id", text)? != authority.chain_id
        || field(c, "height", number)? != expected_height
        || field(c, "round", number)? > u64::from(i32::MAX.unsigned_abs())
        || field(&c["block_id"], "hash", hex::<32>)? != expected_hash
        || header_hash(h)? != expected_hash
    {
        return Err(RpcError::Verification);
    }
    verify_commit_signatures(h, c, expected_height, authority)?;
    if inclusion(&coverage.transaction_proof, &coverage.batch_transaction)?
        != field(h, "data_hash", hex::<32>)?
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

fn verify_commit_signatures(
    header: &Value,
    commit: &Value,
    expected_height: u64,
    authority: &NodeCommitAuthority,
) -> Result<(), RpcError> {
    let mut validator_bytes = Vec::new();
    let mut total = 0_u64;
    for (key, power) in &authority.validators {
        let mut simple = blob(1, &blob(1, key));
        simple.extend(scalar(2, *power));
        validator_bytes.push(simple);
        total += power;
    }
    if field(header, "validators_hash", hex::<32>)? != root(&validator_bytes) {
        return Err(RpcError::Verification);
    }
    let signatures = commit["signatures"]
        .as_array()
        .filter(|v| v.len() == authority.validators.len())
        .ok_or_else(|| super::invalid_response("commit.signatures"))?;
    let mut seen = std::collections::BTreeSet::new();
    let mut signed_power = 0_u64;
    for signature in signatures {
        let flag = field(signature, "block_id_flag", number)?;
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
        let address = field(signature, "validator_address", hex::<20>)?;
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
        vote.extend_from_slice(
            &i64::try_from(expected_height)
                .map_err(|_| RpcError::Verification)?
                .to_le_bytes(),
        );
        let round = field(commit, "round", number)?;
        if round != 0 {
            vote.push(0x19);
            vote.extend_from_slice(
                &(i64::try_from(round).map_err(|_| RpcError::Verification)?).to_le_bytes(),
            );
        }
        if flag == 2 {
            vote.extend(blob(4, &field(commit, "block_id", block_id)?));
        }
        vote.extend(blob(5, &field(signature, "timestamp", timestamp)?));
        vote.extend(blob(6, authority.chain_id.as_bytes()));
        let mut message = Vec::new();
        varint(vote.len() as u64, &mut message);
        message.extend(vote);
        let signature: [u8; 64] = field(signature, "signature", base64)?
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
    Ok(())
}
