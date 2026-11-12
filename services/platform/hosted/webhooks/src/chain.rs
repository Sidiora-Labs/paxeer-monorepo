//! Chain certificates and native namespace membership at the webhook boundary.
use ed25519_dalek::{Signature, VerifyingKey};
use layerx_indexer::ai_market::{BlockCommitProof, FinalityAuthority, VerifiedCommit};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeSet;

use crate::encoding::digest;
use crate::error::WebhookError;

type Result<T> = std::result::Result<T, WebhookError>;

/// Receipt versions published by `node/sequencer/receipt.go` and `rpc/sequencer_receipt.go`.
pub const NODE_RECEIPT_PROTOCOL_VERSION: u16 = 2;
pub const NODE_RECEIPT_LAYOUT_VERSION: u32 = 1;
pub const NODE_RECEIPT_PARAMETER_VERSION: u32 = 0;

struct SequenceReceipt {
    sequence: u64,
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
}

fn rejected<T>() -> Result<T> {
    Err(WebhookError::VerificationRequired)
}

pub fn varint(mut n: u64, out: &mut Vec<u8>) {
    while n >= 128 {
        out.push(n.to_le_bytes()[0] | 128);
        n >>= 7;
    }
    out.push(n.to_le_bytes()[0]);
}
pub fn bytes(field: u8, value: &[u8], out: &mut Vec<u8>) {
    out.push(field << 3 | 2);
    varint(value.len() as u64, out);
    out.extend(value);
}
pub fn number(field: u8, value: u64, out: &mut Vec<u8>) {
    if value != 0 {
        out.push(field << 3);
        varint(value, out);
    }
}
fn wrapped(value: &[u8]) -> Vec<u8> {
    let mut out = Vec::new();
    if !value.is_empty() {
        bytes(1, value, &mut out);
    }
    out
}
fn read_varint(input: &mut &[u8]) -> Result<u64> {
    let mut n = 0;
    for shift in (0..70).step_by(7) {
        let (&byte, rest) = input
            .split_first()
            .ok_or(WebhookError::VerificationRequired)?;
        *input = rest;
        if shift == 63 && byte > 1 {
            return rejected();
        }
        n |= u64::from(byte & 127) << shift;
        if byte & 128 == 0 {
            if shift != 0 && byte == 0 {
                return rejected();
            }
            return Ok(n);
        }
    }
    rejected()
}
#[derive(Clone, Copy)]
enum Value<'a> {
    Number(u64),
    Bytes(&'a [u8]),
}
fn fields(mut input: &[u8]) -> Result<Vec<(u64, Value<'_>)>> {
    if input.len() > 1_048_576 {
        return rejected();
    }
    let mut out = Vec::new();
    let mut previous = 0;
    while !input.is_empty() {
        let tag = read_varint(&mut input)?;
        let field = tag >> 3;
        if field == 0 || field < previous || field > 0x1fff_ffff || out.len() >= 132 {
            return rejected();
        }
        previous = field;
        let value = match tag & 7 {
            0 => Value::Number(read_varint(&mut input)?),
            2 => {
                let n = usize::try_from(read_varint(&mut input)?)
                    .map_err(|_| WebhookError::VerificationRequired)?;
                let value = input.get(..n).ok_or(WebhookError::VerificationRequired)?;
                input = &input[n..];
                Value::Bytes(value)
            }
            _ => return rejected(),
        };
        out.push((field, value));
    }
    Ok(out)
}
fn field_bytes<'a>(field: &(u64, Value<'a>), n: u64) -> Result<&'a [u8]> {
    match field {
        (number, Value::Bytes(value)) if *number == n => Ok(value),
        _ => rejected(),
    }
}
fn one_bytes(input: &[u8], n: u64) -> Result<&[u8]> {
    let f = fields(input)?;
    if f.len() != 1 {
        return rejected();
    }
    field_bytes(&f[0], n)
}
fn one_number(input: &[u8], n: u64) -> Result<u64> {
    let f = fields(input)?;
    match f.as_slice() {
        [(field, Value::Number(value))] if *field == n => Ok(*value),
        _ => rejected(),
    }
}

#[must_use]
pub fn merkle(items: &[Vec<u8>]) -> [u8; 32] {
    match items.len() {
        0 => digest(&[]),
        1 => digest(&[&[0][..], items[0].as_slice()].concat()),
        n => {
            let split = 1 << (usize::BITS - 1 - (n - 1).leading_zeros());
            digest(&[&[1][..], &merkle(&items[..split]), &merkle(&items[split..])].concat())
        }
    }
}

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ValidatorPin {
    pub public_key: [u8; 32],
    pub voting_power: u64,
}
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ChainPolicy {
    pub chain_id: String,
    pub validators: Vec<ValidatorPin>,
}
impl ChainPolicy {
    ///
    /// # Errors
    /// Returns an error when chain evidence is malformed or fails verification.
    pub fn from_native(authority: &FinalityAuthority) -> Result<Self> {
        Ok(Self {
            chain_id: authority.chain_id.clone(),
            validators: authority
                .validators
                .iter()
                .map(|v| {
                    Ok(ValidatorPin {
                        public_key: v.pub_key,
                        voting_power: u64::try_from(v.voting_power)
                            .map_err(|_| WebhookError::VerificationRequired)?,
                    })
                })
                .collect::<Result<_>>()?,
        })
    }
    ///
    /// # Errors
    /// Returns an error when chain evidence is malformed or fails verification.
    pub fn validate(&self) -> Result<()> {
        if self.chain_id.is_empty()
            || self.chain_id.len() > 50
            || self.validators.is_empty()
            || self.validators.len() > 256
        {
            return rejected();
        }
        let mut seen = BTreeSet::new();
        let mut total = 0u64;
        for v in &self.validators {
            if v.voting_power == 0
                || !seen.insert(v.public_key)
                || VerifyingKey::from_bytes(&v.public_key).is_err()
            {
                return rejected();
            }
            total = total
                .checked_add(v.voting_power)
                .filter(|v| *v <= (i64::MAX / 8) as u64)
                .ok_or(WebhookError::VerificationRequired)?;
        }
        Ok(())
    }
    fn validators_hash(&self) -> [u8; 32] {
        merkle(
            &self
                .validators
                .iter()
                .map(|v| {
                    let mut key = Vec::new();
                    bytes(1, &v.public_key, &mut key);
                    let mut out = Vec::new();
                    bytes(1, &key, &mut out);
                    number(2, v.voting_power, &mut out);
                    out
                })
                .collect::<Vec<_>>(),
        )
    }
}

/// Transport of the exact fourteen canonical header leaves and consensus votes.
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CommitCertificate {
    pub header_fields: Vec<Vec<u8>>,
    pub round: i32,
    pub parts_total: u32,
    pub parts_hash: [u8; 32],
    pub votes: Vec<CommitVote>,
}
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CommitVote {
    pub flag: u8,
    pub address: Vec<u8>,
    pub seconds: i64,
    pub nanos: u32,
    pub signature: Vec<u8>,
}
impl CommitCertificate {
    /// Encodes an actual producer commit; no asserted finality flag is transported.
    ///
    /// # Errors
    /// Returns an error when chain evidence is malformed or fails verification.
    pub fn from_proof(proof: &BlockCommitProof<'_>) -> Result<Self> {
        let h = proof.header;
        if !(-62_135_596_800..=253_402_300_799).contains(&h.time.seconds)
            || h.time.nanos >= 1_000_000_000
        {
            return rejected();
        }
        let mut version = Vec::new();
        number(1, h.version.block, &mut version);
        number(2, h.version.app, &mut version);
        let mut height = Vec::new();
        number(
            1,
            u64::try_from(h.height).map_err(|_| WebhookError::VerificationRequired)?,
            &mut height,
        );
        let mut time = Vec::new();
        number(
            1,
            u64::from_ne_bytes(h.time.seconds.to_ne_bytes()),
            &mut time,
        );
        number(2, u64::from(h.time.nanos), &mut time);
        let mut parts = Vec::new();
        number(
            1,
            u64::from(h.last_block_id.part_set_header.total),
            &mut parts,
        );
        if !h.last_block_id.part_set_header.hash.is_empty() {
            bytes(2, &h.last_block_id.part_set_header.hash, &mut parts);
        }
        let mut last = Vec::new();
        if !h.last_block_id.hash.is_empty() {
            bytes(1, &h.last_block_id.hash, &mut last);
        }
        bytes(2, &parts, &mut last);
        if proof.commit.height
            != u64::try_from(h.height).map_err(|_| WebhookError::VerificationRequired)?
            || proof.commit.block_id.hash != h.hash()
        {
            return rejected();
        }
        Ok(Self {
            header_fields: vec![
                version,
                wrapped(h.chain_id.as_bytes()),
                height,
                time,
                last,
                wrapped(&h.last_commit_hash),
                wrapped(&h.data_hash),
                wrapped(&h.validators_hash),
                wrapped(&h.next_validators_hash),
                wrapped(&h.consensus_hash),
                wrapped(&h.app_hash),
                wrapped(&h.last_results_hash),
                wrapped(&h.evidence_hash),
                wrapped(&h.proposer_address),
            ],
            round: proof.commit.round,
            parts_total: proof.commit.block_id.part_set_header.total,
            parts_hash: proof
                .commit
                .block_id
                .part_set_header
                .hash
                .as_slice()
                .try_into()
                .map_err(|_| WebhookError::VerificationRequired)?,
            votes: proof
                .commit
                .signatures
                .iter()
                .map(|v| CommitVote {
                    flag: v.block_id_flag as u8,
                    address: v.validator_address.clone(),
                    seconds: v.timestamp.seconds,
                    nanos: v.timestamp.nanos,
                    signature: v.signature.map_or_else(Vec::new, |v| v.to_vec()),
                })
                .collect(),
        })
    }
    pub(crate) fn height(&self) -> Result<u64> {
        let field = self
            .header_fields
            .get(2)
            .ok_or(WebhookError::VerificationRequired)?;
        let height = one_number(field, 1)?;
        if height == 0 || height > i64::MAX as u64 {
            return rejected();
        }
        Ok(height)
    }
    ///
    /// # Errors
    /// Returns an error when chain evidence is malformed or fails verification.
    pub fn verify(&self, policy: &ChainPolicy, store_proof: &[u8]) -> Result<VerifiedCommit> {
        policy.validate()?;
        if self.header_fields.len() != 14
            || self.header_fields.iter().any(|v| v.len() > 4096)
            || self.votes.len() != policy.validators.len()
            || self.round < 0
            || !(1..=101).contains(&self.parts_total)
            || store_proof.is_empty()
            || store_proof.len() > 1_048_576
        {
            return rejected();
        }
        let f = &self.header_fields;
        if one_bytes(&f[1], 1)? != policy.chain_id.as_bytes()
            || one_bytes(&f[7], 1)? != policy.validators_hash()
        {
            return rejected();
        }
        let height = self.height()?;
        let app_hash = one_bytes(&f[10], 1)?
            .try_into()
            .map_err(|_| WebhookError::VerificationRequired)?;
        let block_hash = merkle(f);
        let mut total = 0u64;
        let mut signed = 0u64;
        for (pin, vote) in policy.validators.iter().zip(&self.votes) {
            total += pin.voting_power;
            if vote.flag == 0 {
                if !vote.signature.is_empty()
                    || !vote.address.is_empty()
                    || vote.seconds != -62_135_596_800
                    || vote.nanos != 0
                {
                    return rejected();
                }
                continue;
            }
            if !matches!(vote.flag, 1 | 2)
                || !(-62_135_596_800..=253_402_300_799).contains(&vote.seconds)
                || vote.nanos >= 1_000_000_000
                || vote.address != Sha256::digest(pin.public_key)[..20]
            {
                return rejected();
            }
            let mut message = Vec::new();
            number(1, 2, &mut message);
            message.push(0x11);
            message.extend(height.to_le_bytes());
            if self.round != 0 {
                message.push(0x19);
                message.extend(i64::from(self.round).to_le_bytes());
            }
            if vote.flag == 1 {
                let mut parts = Vec::new();
                number(1, self.parts_total.into(), &mut parts);
                bytes(2, &self.parts_hash, &mut parts);
                let mut block = Vec::new();
                bytes(1, &block_hash, &mut block);
                bytes(2, &parts, &mut block);
                bytes(4, &block, &mut message);
            }
            let mut time = Vec::new();
            number(1, u64::from_ne_bytes(vote.seconds.to_ne_bytes()), &mut time);
            number(2, vote.nanos.into(), &mut time);
            bytes(5, &time, &mut message);
            bytes(6, policy.chain_id.as_bytes(), &mut message);
            let mut sign_bytes = Vec::new();
            varint(message.len() as u64, &mut sign_bytes);
            sign_bytes.extend(message);
            let signature: [u8; 64] = vote
                .signature
                .as_slice()
                .try_into()
                .map_err(|_| WebhookError::VerificationRequired)?;
            VerifyingKey::from_bytes(&pin.public_key)
                .map_err(|_| WebhookError::VerificationRequired)?
                .verify_strict(&sign_bytes, &Signature::from_bytes(&signature))
                .map_err(|_| WebhookError::VerificationRequired)?;
            if vote.flag == 1 {
                signed += pin.voting_power;
            }
        }
        if u128::from(signed) * 3 <= u128::from(total) * 2 {
            return rejected();
        }
        Ok(VerifiedCommit {
            chain_id: policy.chain_id.clone(),
            height,
            block_hash,
            app_hash,
            store_proof_digest: digest(store_proof),
        })
    }
}

struct Existence<'a> {
    key: &'a [u8],
    value: &'a [u8],
    leaf: &'a [u8],
    path: Vec<&'a [u8]>,
}
fn existence(input: &[u8]) -> Result<Existence<'_>> {
    let f = fields(input)?;
    if f.len() < 3 || f.len() > 131 {
        return rejected();
    }
    Ok(Existence {
        key: field_bytes(&f[0], 1)?,
        value: field_bytes(&f[1], 2)?,
        leaf: field_bytes(&f[2], 3)?,
        path: f[3..]
            .iter()
            .map(|v| field_bytes(v, 4))
            .collect::<Result<_>>()?,
    })
}
fn leaf_prefix(input: &[u8]) -> Result<&[u8]> {
    let f = fields(input)?;
    let mut ops = [0u64; 4];
    let mut prefix = None;
    let mut seen = BTreeSet::new();
    for (n, v) in f {
        if !seen.insert(n) {
            return rejected();
        }
        match (n, v) {
            (1..=4, Value::Number(value)) => {
                ops[usize::try_from(n).map_err(|_| WebhookError::VerificationRequired)? - 1] =
                    value;
            }
            (5, Value::Bytes(value)) => prefix = Some(value),
            _ => return rejected(),
        }
    }
    if ops != [1, 0, 1, 1] {
        return rejected();
    }
    prefix.ok_or(WebhookError::VerificationRequired)
}
fn metadata(input: &mut &[u8]) -> Result<u64> {
    let mut height = 0;
    for i in 0..3 {
        let n = read_varint(input)?;
        if n & 1 != 0 {
            return rejected();
        }
        if i == 0 {
            height = n >> 1;
        }
    }
    Ok(height)
}
fn existence_root(proof: &Existence<'_>, iavl: bool) -> Result<[u8; 32]> {
    if proof.key.is_empty() || proof.value.is_empty() || proof.path.len() > 128 {
        return rejected();
    }
    let prefix = leaf_prefix(proof.leaf)?;
    if iavl {
        let mut tail = prefix;
        if prefix.first() != Some(&0) || metadata(&mut tail)? != 0 || !tail.is_empty() {
            return rejected();
        }
    } else if prefix != [0] {
        return rejected();
    }
    let mut leaf = prefix.to_vec();
    varint(proof.key.len() as u64, &mut leaf);
    leaf.extend(proof.key);
    varint(32, &mut leaf);
    leaf.extend(digest(proof.value));
    let mut root = digest(&leaf);
    for (depth, op) in proof.path.iter().enumerate() {
        let f = fields(op)?;
        let (prefix, suffix) = match f.as_slice() {
            [(1, Value::Number(1)), (2, Value::Bytes(prefix))] => (*prefix, &[][..]),
            [(1, Value::Number(1)), (2, Value::Bytes(prefix)), (3, Value::Bytes(suffix))] => {
                (*prefix, *suffix)
            }
            _ => return rejected(),
        };
        if iavl {
            let left = match suffix.len() {
                33 => 0,
                0 => 1,
                _ => return rejected(),
            };
            let length = prefix
                .len()
                .checked_sub(left * 33)
                .ok_or(WebhookError::VerificationRequired)?;
            if !(4..=12).contains(&length) || prefix.first() == Some(&0) {
                return rejected();
            }
            let mut tail = prefix;
            if metadata(&mut tail)? < depth as u64 + 1 {
                return rejected();
            }
            match (left, tail) {
                (0, [32]) if suffix[0] == 32 => {}
                (1, [32, ..]) if tail.len() == 34 && tail[33] == 32 => {}
                _ => return rejected(),
            }
        } else if !((prefix == [1] && suffix.len() == 32)
            || (prefix.len() == 33 && prefix[0] == 1 && suffix.is_empty()))
        {
            return rejected();
        }
        root = digest(&[prefix, &root, suffix].concat());
    }
    Ok(root)
}

/// An IAVL existence proof and the multistore path linking it to the app hash.
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Membership {
    pub proof: Vec<u8>,
    pub multistore: Vec<u8>,
}
impl Membership {
    ///
    /// # Errors
    /// Returns an error when chain evidence is malformed or fails verification.
    pub fn verify(&self, app_hash: [u8; 32], store: &[u8], key: &[u8], value: &[u8]) -> Result<()> {
        let proof = existence(&self.proof)?;
        if proof.key != key || proof.value != value {
            return rejected();
        }
        let root = existence_root(&proof, true)?;
        let outer = existence(one_bytes(&self.multistore, 1)?)?;
        if outer.key != store || outer.value != root || existence_root(&outer, false)? != app_hash {
            return rejected();
        }
        Ok(())
    }
    ///
    /// # Errors
    /// Returns an error when chain evidence is malformed or fails verification.
    pub fn value(&self) -> Result<&[u8]> {
        Ok(existence(&self.proof)?.value)
    }
}

/// Authentic native shared-storage head plus its content-addressed manifest.
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct ProgramStateProof {
    pub membership: Membership,
    pub manifest: Vec<u8>,
}
impl ProgramStateProof {
    ///
    /// # Errors
    /// Returns an error when chain evidence is malformed or fails verification.
    pub fn verify(&self, app_hash: [u8; 32], program: &[u8; 32], state: &[u8]) -> Result<()> {
        let mut key = b"\x01\x00\x09progstor".to_vec();
        key.extend(program);
        key.push(1);
        let value = self.membership.value()?;
        if value.len() != 38
            || value[..2] != [0, 1]
            || self.manifest.len() > 1_048_576
            || digest(&self.manifest) != value[6..]
        {
            return rejected();
        }
        self.membership.verify(app_hash, b"lxphost", &key, value)?;
        let count = u32::from_be_bytes(
            value[2..6]
                .try_into()
                .map_err(|_| WebhookError::VerificationRequired)?,
        );
        if count == 0 || count > 65_536 || self.manifest.get(..6) != Some(&value[..6]) {
            return rejected();
        }
        let mut input = &self.manifest[6..];
        let mut previous: Option<Vec<u8>> = None;
        let mut found = false;
        for _ in 0..count {
            let n = u16::from_be_bytes(
                take(&mut input, 2)?
                    .try_into()
                    .map_err(|_| WebhookError::VerificationRequired)?,
            ) as usize;
            if n == 0 || n > 1024 {
                return rejected();
            }
            let key = take(&mut input, n)?;
            if previous.as_ref().is_some_and(|v| v.as_slice() >= key) {
                return rejected();
            }
            previous = Some(key.to_vec());
            let hash = take(&mut input, 32)?;
            let length = u32::from_be_bytes(
                take(&mut input, 4)?
                    .try_into()
                    .map_err(|_| WebhookError::VerificationRequired)?,
            ) as usize;
            if key == layerx_programs_ai_market::SHARED_STATE_KEY {
                if length != state.len() || hash != digest(state) {
                    return rejected();
                }
                found = true;
            }
        }
        if !input.is_empty() || !found {
            return rejected();
        }
        Ok(())
    }
}
fn take<'a>(input: &mut &'a [u8], n: usize) -> Result<&'a [u8]> {
    let value = input.get(..n).ok_or(WebhookError::VerificationRequired)?;
    *input = &input[n..];
    Ok(value)
}

/// Node-produced receipt with the committed block and governed key witness.
#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct NodeReceiptEvidence {
    pub receipt: Vec<u8>,
    pub commit: CommitCertificate,
    pub governed_keys: Membership,
    pub transactions: Vec<Vec<u8>>,
    pub activity_ids: Vec<[u8; 32]>,
}
impl NodeReceiptEvidence {
    ///
    /// # Errors
    /// Returns an error when chain evidence is malformed or fails verification.
    pub fn verify(
        &self,
        policy: &ChainPolicy,
        activity: [u8; 32],
    ) -> Result<layerx_platform_gateway::VerifiedOperation> {
        let keys_value = self.governed_keys.value()?;
        if keys_value.is_empty()
            || keys_value.len() % 80 != 0
            || keys_value.len() > 80 * 256
            || self.transactions.is_empty()
            || self.transactions.len() > 65_536
            || self.activity_ids.len() > 65_536
            || self.transactions.iter().map(Vec::len).sum::<usize>() > 16 * 1_048_576
        {
            return rejected();
        }
        let committed = self.commit.verify(policy, &self.governed_keys.proof)?;
        self.governed_keys
            .verify(committed.app_hash, b"sequencer", &[3], keys_value)?;
        let pins: Vec<_> = keys_value.chunks_exact(80).map(|entry| serde_json::json!({
            "keyId": crate::encoding::base64_encode(&entry[..32]), "publicKey": crate::encoding::base64_encode(&entry[32..64]),
            "firstHeight": u64::from_be_bytes(std::array::from_fn(|i| entry[64+i])),
            "lastHeight": u64::from_be_bytes(std::array::from_fn(|i| entry[72+i])),
        })).collect();
        let keys = layerx_platform_gateway::ReceiptKeyList::from_config(
            &serde_json::to_string(&pins).map_err(|_| WebhookError::VerificationRequired)?,
        )
        .map_err(|_| WebhookError::VerificationRequired)?;
        let decoded = keys.verify_receipt(&self.receipt)?;
        let protocol = decoded
            .protocol()
            .ok_or(WebhookError::VerificationRequired)?;
        let receipt_height = u64::from_be_bytes(
            protocol.batch_id()[24..]
                .try_into()
                .map_err(|_| WebhookError::VerificationRequired)?,
        );
        if protocol.protocol_version() != NODE_RECEIPT_PROTOCOL_VERSION
            || protocol.module_version() != NODE_RECEIPT_LAYOUT_VERSION
            || protocol.parameter_version() != NODE_RECEIPT_PARAMETER_VERSION
            || protocol.activity_id() != activity
            || receipt_height != committed.height
        {
            return rejected();
        }
        let data = merkle(
            &self
                .transactions
                .iter()
                .map(|tx| digest(tx).to_vec())
                .collect::<Vec<_>>(),
        );
        if one_bytes(&self.commit.header_fields[6], 1)? != data {
            return rejected();
        }
        self.verify_batch(
            &keys,
            committed.height,
            &SequenceReceipt {
                sequence: protocol.global_sequence(),
                previous_state_root: protocol.previous_state_root(),
                resulting_state_root: protocol.resulting_state_root(),
            },
            activity,
            policy,
        )?;
        layerx_platform_gateway::verify_live_receipt(
            &self.receipt,
            layerx_platform_gateway::AuthorityFacts::new(
                protocol.batch_id(),
                protocol.asset(),
                protocol.previous_state_root(),
                protocol.resulting_state_root(),
                keys.authorization_for_header(protocol.context_hash(), receipt_height)?
                    .public_key(),
            ),
            &keys,
            Some(activity),
        )
        .map_err(WebhookError::from)
    }
    fn verify_batch(
        &self,
        keys: &layerx_platform_gateway::ReceiptKeyList,
        height: u64,
        receipt: &SequenceReceipt,
        activity: [u8; 32],
        policy: &ChainPolicy,
    ) -> Result<()> {
        const DOMAIN: &[u8] = b"PAXEER/v1/sequence-batch\0";
        let raw = &self.transactions[0];
        if raw.len() != DOMAIN.len() + 24 + 128 + 64 || !raw.starts_with(DOMAIN) {
            return rejected();
        }
        let b = &raw[DOMAIN.len()..];
        let at = |offset| -> Result<u64> {
            Ok(u64::from_be_bytes(
                b[offset..offset + 8]
                    .try_into()
                    .map_err(|_| WebhookError::VerificationRequired)?,
            ))
        };
        let sequence = receipt.sequence;
        let from = at(8)?;
        let to = at(16)?;
        if at(0)? != height
            || from == 0
            || to < from
            || self.activity_ids.len() as u64 != to - from + 1
            || sequence < from
            || sequence > to
            || self.activity_ids[usize::try_from(sequence - from)
                .map_err(|_| WebhookError::VerificationRequired)?]
                != activity
        {
            return rejected();
        }
        let mut previous: [u8; 32] = b[24..56]
            .try_into()
            .map_err(|_| WebhookError::VerificationRequired)?;
        let mut tx_root = Sha256::new();
        for (offset, id) in self.activity_ids.iter().enumerate() {
            let position = from + offset as u64;
            let next = digest(
                &[
                    b"PAXEER/v1/sequence\0".as_slice(),
                    &previous,
                    &position.to_be_bytes(),
                    id,
                ]
                .concat(),
            );
            if position == sequence
                && (previous != receipt.previous_state_root || next != receipt.resulting_state_root)
            {
                return rejected();
            }
            previous = next;
            tx_root.update(id);
        }
        if previous != b[56..88] {
            return rejected();
        }
        if tx_root.finalize().as_slice() != &b[88..120] {
            return rejected();
        }
        let id = b[120..152]
            .try_into()
            .map_err(|_| WebhookError::VerificationRequired)?;
        let pin = keys.authorization_for_header(id, height)?;
        let mut preimage = DOMAIN.to_vec();
        preimage.extend(policy.chain_id.as_bytes());
        preimage.extend(&b[..120]);
        let signature = b[152..216]
            .try_into()
            .map_err(|_| WebhookError::VerificationRequired)?;
        VerifyingKey::from_bytes(&pin.public_key())
            .map_err(|_| WebhookError::VerificationRequired)?
            .verify_strict(&digest(&preimage), &Signature::from_bytes(&signature))
            .map_err(|_| WebhookError::VerificationRequired)
    }
}

impl CommitCertificate {
    /// Decodes the `CometBFT` `commit` RPC result without trusting its block ID.
    ///
    /// # Errors
    /// Returns an error when chain evidence is malformed or fails verification.
    pub fn from_comet(result: &serde_json::Value) -> Result<Self> {
        let signed = &result["signed_header"];
        let h = &signed["header"];
        let c = &signed["commit"];
        let text = |v: &serde_json::Value| -> Result<String> {
            v.as_str()
                .map(str::to_owned)
                .ok_or(WebhookError::VerificationRequired)
        };
        let integer = |v: &serde_json::Value| -> Result<u64> {
            if let Some(n) = v.as_u64() {
                Ok(n)
            } else {
                text(v)?
                    .parse()
                    .map_err(|_| WebhookError::VerificationRequired)
            }
        };
        let hex =
            |v: &serde_json::Value| -> Result<Vec<u8>> { crate::encoding::hex_decode(&text(v)?) };
        let mut version = Vec::new();
        number(1, integer(&h["version"]["block"])?, &mut version);
        // Comet's protobuf JSON omits a zero application version.
        let app_version = match h["version"].get("app") {
            None => 0,
            Some(value) => integer(value)?,
        };
        number(2, app_version, &mut version);
        let chain = text(&h["chain_id"])?;
        let height = integer(&h["height"])?;
        if height != integer(&c["height"])? {
            return rejected();
        }
        let mut height_bytes = Vec::new();
        number(1, height, &mut height_bytes);
        let (seconds, nanos) = timestamp(&text(&h["time"])?)?;
        let mut time = Vec::new();
        number(1, u64::from_ne_bytes(seconds.to_ne_bytes()), &mut time);
        number(2, nanos.into(), &mut time);
        let encode_block = |v: &serde_json::Value| -> Result<Vec<u8>> {
            let mut parts = Vec::new();
            number(1, integer(&v["parts"]["total"])?, &mut parts);
            let hash = hex(&v["parts"]["hash"])?;
            if !hash.is_empty() {
                bytes(2, &hash, &mut parts);
            }
            let mut block = Vec::new();
            let hash = hex(&v["hash"])?;
            if !hash.is_empty() {
                bytes(1, &hash, &mut block);
            }
            bytes(2, &parts, &mut block);
            Ok(block)
        };
        let mut header_fields = vec![
            version,
            wrapped(chain.as_bytes()),
            height_bytes,
            time,
            encode_block(&h["last_block_id"])?,
        ];
        for name in [
            "last_commit_hash",
            "data_hash",
            "validators_hash",
            "next_validators_hash",
            "consensus_hash",
            "app_hash",
            "last_results_hash",
            "evidence_hash",
            "proposer_address",
        ] {
            header_fields.push(wrapped(&hex(&h[name])?));
        }
        if hex(&c["block_id"]["hash"])? != merkle(&header_fields) {
            return rejected();
        }
        let votes = decode_comet_votes(c)?;
        Ok(Self {
            header_fields,
            round: i32::try_from(integer(&c["round"])?)
                .map_err(|_| WebhookError::VerificationRequired)?,
            parts_total: u32::try_from(integer(&c["block_id"]["parts"]["total"])?)
                .map_err(|_| WebhookError::VerificationRequired)?,
            parts_hash: hex(&c["block_id"]["parts"]["hash"])?
                .try_into()
                .map_err(|_| WebhookError::VerificationRequired)?,
            votes,
        })
    }
}
fn decode_comet_votes(c: &serde_json::Value) -> Result<Vec<CommitVote>> {
    let text = |v: &serde_json::Value| -> Result<String> {
        v.as_str()
            .map(str::to_owned)
            .ok_or(WebhookError::VerificationRequired)
    };
    let integer = |v: &serde_json::Value| -> Result<u64> {
        if let Some(n) = v.as_u64() {
            Ok(n)
        } else {
            text(v)?
                .parse()
                .map_err(|_| WebhookError::VerificationRequired)
        }
    };
    let hex = |v: &serde_json::Value| -> Result<Vec<u8>> { crate::encoding::hex_decode(&text(v)?) };
    let votes = c["signatures"]
        .as_array()
        .ok_or(WebhookError::VerificationRequired)?;
    if votes.len() > 256 {
        return rejected();
    }
    votes
        .iter()
        .map(|v| {
            let flag = integer(&v["block_id_flag"])?;
            let (seconds, nanos) = timestamp(&text(&v["timestamp"])?)?;
            Ok(CommitVote {
                flag: u8::try_from(
                    flag.checked_sub(1)
                        .ok_or(WebhookError::VerificationRequired)?,
                )
                .map_err(|_| WebhookError::VerificationRequired)?,
                address: hex(&v["validator_address"])?,
                seconds,
                nanos,
                signature: if v["signature"].is_null() {
                    Vec::new()
                } else {
                    crate::encoding::base64_decode(&text(&v["signature"])?)?
                },
            })
        })
        .collect::<Result<_>>()
}

fn timestamp(input: &str) -> Result<(i64, u32)> {
    let s = input
        .strip_suffix('Z')
        .ok_or(WebhookError::VerificationRequired)?;
    if s.len() < 19
        || s.as_bytes()[4] != b'-'
        || s.as_bytes()[7] != b'-'
        || s.as_bytes()[10] != b'T'
        || s.as_bytes()[13] != b':'
        || s.as_bytes()[16] != b':'
    {
        return rejected();
    }
    let n = |a, b| -> Result<i64> {
        s.get(a..b)
            .ok_or(WebhookError::VerificationRequired)?
            .parse()
            .map_err(|_| WebhookError::VerificationRequired)
    };
    let year = n(0, 4)?;
    let month = n(5, 7)?;
    let day = n(8, 10)?;
    let hour = n(11, 13)?;
    let minute = n(14, 16)?;
    let second = n(17, 19)?;
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
        || day > days[usize::try_from(month - 1).map_err(|_| WebhookError::VerificationRequired)?]
        || !(0..24).contains(&hour)
        || !(0..60).contains(&minute)
        || !(0..60).contains(&second)
    {
        return rejected();
    }
    let y = year - i64::from(month <= 2);
    let era = y / 400;
    let yoe = y - era * 400;
    let doy = (153 * (month + if month > 2 { -3 } else { 9 }) + 2) / 5 + day - 1;
    let days = era * 146_097 + yoe * 365 + yoe / 4 - yoe / 100 + doy - 719_468;
    let nanos = if s.len() == 19 {
        0
    } else {
        let fraction = s
            .strip_prefix(&format!("{}.", &s[..19]))
            .ok_or(WebhookError::VerificationRequired)?;
        if fraction.is_empty()
            || fraction.len() > 9
            || !fraction.bytes().all(|v| v.is_ascii_digit())
        {
            return rejected();
        }
        fraction
            .parse::<u32>()
            .map_err(|_| WebhookError::VerificationRequired)?
            * 10u32.pow(
                9 - u32::try_from(fraction.len())
                    .map_err(|_| WebhookError::VerificationRequired)?,
            )
    };
    Ok((days * 86400 + hour * 3600 + minute * 60 + second, nanos))
}

impl Membership {
    /// Decodes an actual ABCI proof-ops answer; commitment wrappers stay strict.
    ///
    /// # Errors
    /// Returns an error when chain evidence is malformed or fails verification.
    pub fn from_abci(response: &serde_json::Value) -> Result<Self> {
        match response.get("code") {
            None => {}
            Some(value) if value.as_u64() == Some(0) => {}
            _ => return rejected(),
        }
        let ops = response
            .get("proofOps")
            .or_else(|| response.get("proof_ops"))
            .and_then(|v| v["ops"].as_array())
            .ok_or(WebhookError::VerificationRequired)?;
        if ops.len() != 2 || ops[0]["type"] != "ics23:iavl" || ops[1]["type"] != "ics23:simple" {
            return rejected();
        }
        let inner = crate::encoding::base64_decode(
            ops[0]["data"]
                .as_str()
                .ok_or(WebhookError::VerificationRequired)?,
        )?;
        let outer = crate::encoding::base64_decode(
            ops[1]["data"]
                .as_str()
                .ok_or(WebhookError::VerificationRequired)?,
        )?;
        Ok(Self {
            proof: one_bytes(&inner, 1)?.to_vec(),
            multistore: outer,
        })
    }
}
