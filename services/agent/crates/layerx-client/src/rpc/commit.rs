use base64::Engine as _;
use layerx_programs_registry::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    validators::{TrustedValidator, ValidatorSet},
    vote::{BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofError, ChainProofVerifier, Commit, VerifiedChainHead,
};
use serde_json::Value;

use crate::sequencer::{SequencerEndpoint, SequencerError};

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum CommitError {
    Rpc(SequencerError),
    Invalid,
    Proof(ChainProofError),
}

impl From<SequencerError> for CommitError {
    fn from(error: SequencerError) -> Self {
        Self::Rpc(error)
    }
}

pub fn acquire_committed_header(
    endpoint: &SequencerEndpoint,
    height: u64,
    verifier: &ChainProofVerifier,
) -> Result<VerifiedChainHead, CommitError> {
    let material = endpoint.commit_material(height)?;
    let validators = decode_validators(&endpoint.validator_set(height)?)?;
    if validators != verifier.validators {
        return Err(CommitError::Invalid);
    }
    let head = verify_commit_material(&material["signedHeader"], verifier)?;
    if u64::try_from(head.height()).ok() != Some(height) {
        return Err(CommitError::Invalid);
    }
    Ok(head)
}

pub(crate) fn acquire_pinned_header(
    endpoint: &SequencerEndpoint,
    height: u64,
    block_hash: [u8; 32],
    app_hash: [u8; 32],
) -> Result<VerifiedChainHead, CommitError> {
    let material = endpoint.commit_material(height)?;
    let validators = decode_validators(&endpoint.validator_set(height)?)?;
    let verifier = ChainProofVerifier {
        chain_id: text(&material["signedHeader"]["header"]["chain_id"])?.to_owned(),
        validators,
    };
    let head = verify_commit_material(&material["signedHeader"], &verifier)?;
    if u64::try_from(head.height()).ok() != Some(height)
        || head.header_hash() != block_hash
        || head.app_hash() != app_hash
    {
        return Err(CommitError::Invalid);
    }
    Ok(head)
}

pub fn decode_validators(value: &Value) -> Result<ValidatorSet, CommitError> {
    let entries = value["validators"].as_array().ok_or(CommitError::Invalid)?;
    if entries.is_empty() {
        return Err(CommitError::Invalid);
    }
    let mut validators = Vec::with_capacity(entries.len());
    for entry in entries {
        if entry["pubKey"]["type"] != "tendermint/PubKeyEd25519" {
            return Err(CommitError::Invalid);
        }
        let key: [u8; 32] = base64::engine::general_purpose::STANDARD
            .decode(text(&entry["pubKey"]["value"])?)
            .map_err(|_| CommitError::Invalid)?
            .try_into()
            .map_err(|_| CommitError::Invalid)?;
        let power = number(&entry["votingPower"])?;
        let validator = TrustedValidator::new(key, power);
        if power == 0 || hex(&entry["address"])? != validator.address {
            return Err(CommitError::Invalid);
        }
        validators.push(validator);
    }
    ValidatorSet::new(validators).map_err(CommitError::Proof)
}

pub fn verify_commit_material(
    signed: &Value,
    verifier: &ChainProofVerifier,
) -> Result<VerifiedChainHead, CommitError> {
    let h = &signed["header"];
    let time = timestamp(&h["time"])?;
    let last = block_id(&h["last_block_id"])?;
    let header = ChainHeader {
        version: HeaderVersion {
            block: number(&h["version"]["block"])?,
            app: number(&h["version"]["app"])?,
        },
        chain_id: text(&h["chain_id"])?.to_owned(),
        height: i64::try_from(number(&h["height"])?).map_err(|_| CommitError::Invalid)?,
        time: HeaderTime {
            seconds: time.seconds,
            nanos: time.nanos,
        },
        last_block_id: HeaderBlockId {
            hash: last.hash,
            part_set_total: last.part_set_header.total,
            part_set_hash: last.part_set_header.hash,
        },
        last_commit_hash: hex::<32>(&h["last_commit_hash"])?.to_vec(),
        data_hash: hex::<32>(&h["data_hash"])?.to_vec(),
        validators_hash: hex::<32>(&h["validators_hash"])?.to_vec(),
        next_validators_hash: hex::<32>(&h["next_validators_hash"])?.to_vec(),
        consensus_hash: hex::<32>(&h["consensus_hash"])?.to_vec(),
        app_hash: hex::<32>(&h["app_hash"])?.to_vec(),
        last_results_hash: hex::<32>(&h["last_results_hash"])?.to_vec(),
        evidence_hash: hex::<32>(&h["evidence_hash"])?.to_vec(),
        proposer_address: hex::<20>(&h["proposer_address"])?.to_vec(),
    };
    let c = &signed["commit"];
    let entries = c["signatures"].as_array().ok_or(CommitError::Invalid)?;
    if entries.len() != verifier.validators.validators().len() {
        return Err(CommitError::Invalid);
    }
    let mut signatures = Vec::with_capacity(entries.len());
    for entry in entries {
        let flag =
            BlockIdFlag::try_from(number(&entry["block_id_flag"])?).map_err(CommitError::Proof)?;
        let (validator_address, time, signature) = if flag == BlockIdFlag::Absent {
            if entry["validator_address"] != "" || !entry["signature"].is_null() {
                return Err(CommitError::Invalid);
            }
            (Vec::new(), timestamp(&entry["timestamp"])?, Vec::new())
        } else {
            (
                hex::<20>(&entry["validator_address"])?.to_vec(),
                timestamp(&entry["timestamp"])?,
                base64::engine::general_purpose::STANDARD
                    .decode(text(&entry["signature"])?)
                    .map_err(|_| CommitError::Invalid)?,
            )
        };
        signatures.push(CommitSig {
            flag,
            validator_address,
            timestamp: time,
            signature,
        });
    }
    let commit = Commit {
        height: i64::try_from(number(&c["height"])?).map_err(|_| CommitError::Invalid)?,
        round: i32::try_from(number(&c["round"])?).map_err(|_| CommitError::Invalid)?,
        block_id: block_id(&c["block_id"])?,
        signatures,
    };
    verifier
        .verify_head(&header, &commit)
        .map_err(CommitError::Proof)
}

fn block_id(value: &Value) -> Result<BlockId, CommitError> {
    Ok(BlockId {
        hash: hex::<32>(&value["hash"])?.to_vec(),
        part_set_header: PartSetHeader {
            total: u32::try_from(number(&value["parts"]["total"])?)
                .map_err(|_| CommitError::Invalid)?,
            hash: hex::<32>(&value["parts"]["hash"])?.to_vec(),
        },
    })
}

fn text(value: &Value) -> Result<&str, CommitError> {
    value.as_str().ok_or(CommitError::Invalid)
}
fn number(value: &Value) -> Result<u64, CommitError> {
    value
        .as_u64()
        .or_else(|| value.as_str()?.parse().ok())
        .ok_or(CommitError::Invalid)
}
fn hex<const N: usize>(value: &Value) -> Result<[u8; N], CommitError> {
    let digits = text(value)?;
    let digits = digits.strip_prefix("0x").unwrap_or(digits);
    if digits.len() != N * 2 {
        return Err(CommitError::Invalid);
    }
    let mut result = [0; N];
    for (index, pair) in digits.as_bytes().chunks_exact(2).enumerate() {
        result[index] = u8::from_str_radix(
            std::str::from_utf8(pair).map_err(|_| CommitError::Invalid)?,
            16,
        )
        .map_err(|_| CommitError::Invalid)?;
    }
    Ok(result)
}

fn timestamp(value: &serde_json::Value) -> Result<Timestamp, CommitError> {
    let text = value.as_str().ok_or(CommitError::Invalid)?;
    let bad = || CommitError::Invalid;
    let number = |start, end| {
        let digits = text.get(start..end).ok_or_else(bad)?;
        if !digits.bytes().all(|byte| byte.is_ascii_digit()) {
            return Err(bad());
        }
        digits.parse::<i64>().map_err(|_| bad())
    };
    if text.len() < 20
        || text.get(4..5) != Some("-")
        || text.get(7..8) != Some("-")
        || text.get(10..11) != Some("T")
        || text.get(13..14) != Some(":")
        || text.get(16..17) != Some(":")
        || !text.ends_with('Z')
    {
        return Err(bad());
    }
    let (mut year, month, day) = (number(0, 4)?, number(5, 7)?, number(8, 10)?);
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
    if !(1..=12).contains(&month) || day < 1 || day > days[(month - 1) as usize] {
        return Err(bad());
    }
    let (hour, minute, second) = (number(11, 13)?, number(14, 16)?, number(17, 19)?);
    if hour > 23 || minute > 59 || second > 59 {
        return Err(bad());
    }
    let nanos = if text.len() == 20 {
        0
    } else {
        let fraction = text.get(20..text.len() - 1).ok_or_else(bad)?;
        if text.get(19..20) != Some(".")
            || fraction.is_empty()
            || fraction.len() > 9
            || !fraction.bytes().all(|b| b.is_ascii_digit())
        {
            return Err(bad());
        }
        fraction.parse::<u32>().map_err(|_| bad())? * 10_u32.pow(9 - fraction.len() as u32)
    };
    year -= i64::from(month <= 2);
    let era = year.div_euclid(400);
    let yoe = year - era * 400;
    let mp = month + if month > 2 { -3 } else { 9 };
    let doy = (153 * mp + 2) / 5 + day - 1;
    let days = era * 146097 + yoe * 365 + yoe / 4 - yoe / 100 + doy - 719468;
    let timestamp = Timestamp {
        seconds: days * 86400 + hour * 3600 + minute * 60 + second,
        nanos: nanos as i32,
    };
    if !(-62_135_596_800..=253_402_300_799).contains(&timestamp.seconds)
        || !(0..1_000_000_000).contains(&timestamp.nanos)
    {
        return Err(bad());
    }
    Ok(timestamp)
}
