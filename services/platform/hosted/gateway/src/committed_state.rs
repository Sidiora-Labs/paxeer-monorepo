//! Current node state authenticated by the configured consensus validator set.

use crate::committed_proof::{
    commit::{verify_commit, BlockIdFlag, Commit, CommitSig},
    header::{BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator},
    verify_header,
};
use serde::Deserialize;
use serde_json::Value;

const MAX_AGE_SECONDS: u64 = 120;

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ValidatorConfig {
    public_key: String,
    voting_power: i64,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct AuthorityConfig {
    chain_id: String,
    validators: Vec<ValidatorConfig>,
}

pub struct Authority {
    pub chain_id: String,
    validators: Vec<Validator>,
}

pub struct CurrentState {
    pub height: u64,
    pub block_hash: [u8; 32],
    pub state_root: [u8; 32],
    pub timestamp_ms: u64,
}

impl Authority {
    ///
    /// # Errors
    ///
    /// Returns an error when the validator configuration cannot be read or validated.
    pub fn configured() -> Result<Option<Self>, String> {
        let Some(path) = std::env::var_os("LAYERX_GATEWAY_STATE_VALIDATORS_FILE") else {
            return Ok(None);
        };
        let file = std::fs::File::open(path).map_err(|e| e.to_string())?;
        let mut bytes = Vec::new();
        std::io::Read::read_to_end(&mut std::io::Read::take(file, 65_537), &mut bytes).map_err(|e| e.to_string())?;
        Self::from_config(&bytes).map(Some).map_err(str::to_owned)
    }

    ///
    /// # Errors
    ///
    /// Returns an error when the validator configuration is invalid.
    pub fn from_config(bytes: &[u8]) -> Result<Self, &'static str> {
        if bytes.len() > 65_536 {
            return Err("invalid state authority");
        }
        let document: AuthorityConfig = serde_json::from_slice(bytes).map_err(|_| "invalid state authority")?;
        if document.chain_id.is_empty() || document.chain_id.len() > 50 || document.validators.is_empty() || document.validators.len() > 256 {
            return Err("invalid state authority");
        }
        let mut validators = Vec::new();
        let mut keys = std::collections::BTreeSet::new();
        let mut total = 0_i64;
        for validator in document.validators {
            let public_key: [u8; 32] = hex(&validator.public_key)?.try_into().map_err(|_| "invalid state authority")?;
            if validator.voting_power <= 0 || !keys.insert(public_key) || ed25519_dalek::VerifyingKey::from_bytes(&public_key).is_err() {
                return Err("invalid state authority");
            }
            total = total.checked_add(validator.voting_power).filter(|v| *v <= i64::MAX / 8).ok_or("invalid state authority")?;
            validators.push(Validator {
                pub_key: public_key,
                voting_power: validator.voting_power,
            });
        }
        Ok(Self { chain_id: document.chain_id, validators })
    }

    ///
    /// # Errors
    ///
    /// Returns an error when the committed state proof cannot be verified.
    pub fn verify(&self, status: &Value, document: &Value, now: u64) -> Result<CurrentState, &'static str> {
        let bad = "state_proof_unverified";
        let signed = &document["signed_header"];
        let header = header(&signed["header"]).ok_or(bad)?;
        let commit = commit(&signed["commit"]).ok_or(bad)?;
        if header.chain_id != self.chain_id || header.height <= 0 || header.app_hash.len() != 32 {
            return Err(bad);
        }
        let height = u64::try_from(header.height).map_err(|_| bad)?;
        let block_hash = verify_header(&header, &self.validators).map_err(|_| bad)?;
        verify_commit(&self.chain_id, &self.validators, block_hash, height, &commit).map_err(|_| bad)?;
        let sync = &status["sync_info"];
        if sync["catching_up"] != false || number(&sync["latest_block_height"]) != Some(height) || bytes(&sync["latest_block_hash"]).as_deref() != Some(block_hash.as_slice()) || bytes(&sync["latest_app_hash"]).as_deref() != Some(header.app_hash.as_slice()) {
            return Err("state_proof_stale");
        }
        let timestamp = u64::try_from(header.time.seconds).map_err(|_| bad)?;
        if timestamp > now.saturating_add(5) || now.saturating_sub(timestamp) > MAX_AGE_SECONDS {
            return Err("state_proof_stale");
        }
        Ok(CurrentState {
            height,
            block_hash,
            state_root: header.app_hash.try_into().map_err(|_| bad)?,
            timestamp_ms: timestamp.checked_mul(1000).and_then(|n| n.checked_add(u64::from(header.time.nanos / 1_000_000))).ok_or(bad)?,
        })
    }
}

fn hex(value: &str) -> Result<Vec<u8>, &'static str> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    if value.len() > 128 || !value.len().is_multiple_of(2) || !value.bytes().all(|b| b.is_ascii_hexdigit()) {
        return Err("invalid proof hex");
    }
    (0..value.len()).step_by(2).map(|i| u8::from_str_radix(&value[i..i + 2], 16).map_err(|_| "invalid proof hex")).collect()
}
fn bytes(value: &Value) -> Option<Vec<u8>> {
    hex(value.as_str()?).ok()
}
fn number(value: &Value) -> Option<u64> {
    let s = value.as_str()?;
    let n = s.parse::<u64>().ok()?;
    (n.to_string() == s).then_some(n)
}
fn block_id(value: &Value) -> Option<BlockId> {
    Some(BlockId {
        hash: bytes(&value["hash"])?,
        part_set_header: PartSetHeader {
            total: u32::try_from(value["parts"]["total"].as_u64()?).ok()?,
            hash: bytes(&value["parts"]["hash"])?,
        },
    })
}

fn header(value: &Value) -> Option<Header> {
    Some(Header {
        version: ConsensusVersion {
            block: number(&value["version"]["block"])?,
            app: number(&value["version"]["app"])?,
        },
        chain_id: value["chain_id"].as_str()?.to_owned(),
        height: i64::try_from(number(&value["height"])?).ok()?,
        time: timestamp(value["time"].as_str()?)?,
        last_block_id: block_id(&value["last_block_id"])?,
        last_commit_hash: bytes(&value["last_commit_hash"])?,
        data_hash: bytes(&value["data_hash"])?,
        validators_hash: bytes(&value["validators_hash"])?.try_into().ok()?,
        next_validators_hash: bytes(&value["next_validators_hash"])?,
        consensus_hash: bytes(&value["consensus_hash"])?,
        app_hash: bytes(&value["app_hash"])?,
        last_results_hash: bytes(&value["last_results_hash"])?,
        evidence_hash: bytes(&value["evidence_hash"])?,
        proposer_address: bytes(&value["proposer_address"])?,
    })
}
fn commit(value: &Value) -> Option<Commit> {
    let signatures = value["signatures"].as_array()?;
    if signatures.len() > 256 {
        return None;
    }
    let mut votes = Vec::with_capacity(signatures.len());
    for vote in signatures {
        let flag = match vote["block_id_flag"].as_u64()? {
            1 => BlockIdFlag::Absent,
            2 => BlockIdFlag::Commit,
            3 => BlockIdFlag::Nil,
            _ => return None,
        };
        votes.push(CommitSig {
            block_id_flag: flag,
            validator_address: bytes(&vote["validator_address"])?,
            timestamp: timestamp(vote["timestamp"].as_str()?)?,
            signature: if vote["signature"].is_null() { None } else { Some(layerx_platform_internal::base64::decode(vote["signature"].as_str()?)?.try_into().ok()?) },
        });
    }
    Some(Commit {
        height: number(&value["height"])?,
        round: i32::try_from(value["round"].as_i64()?).ok()?,
        block_id: block_id(&value["block_id"])?,
        signatures: votes,
    })
}

fn timestamp(value: &str) -> Option<Timestamp> {
    if !value.is_ascii() || value.len() > 30 {
        return None;
    }
    let (date, time) = value.strip_suffix('Z')?.split_once('T')?;
    let date: Vec<u32> = date.split('-').map(str::parse).collect::<Result<_, _>>().ok()?;
    let [year, month, day] = date.as_slice() else {
        return None;
    };
    if !(1..=9999).contains(year) || !(1..=12).contains(month) {
        return None;
    }
    let months = [31, if year % 4 == 0 && (year % 100 != 0 || year % 400 == 0) { 29 } else { 28 }, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
    if *day == 0 || *day > months[*month as usize - 1] {
        return None;
    }
    let (whole, fraction) = time.split_once('.').unwrap_or((time, ""));
    if fraction.len() > 9 || !fraction.bytes().all(|b| b.is_ascii_digit()) {
        return None;
    }
    let time: Vec<u32> = whole.split(':').map(str::parse).collect::<Result<_, _>>().ok()?;
    let [hour, minute, second] = time.as_slice() else {
        return None;
    };
    if *hour > 23 || *minute > 59 || *second > 59 {
        return None;
    }
    let before = |y: u32| {
        let p = i64::from(y) - 1;
        365 * p + p / 4 - p / 100 + p / 400
    };
    let days = before(*year) - before(1970) + i64::from(months[..*month as usize - 1].iter().sum::<u32>()) + i64::from(*day - 1);
    Some(Timestamp {
        seconds: days * 86_400 + i64::from(hour * 3600 + minute * 60 + second),
        nanos: if fraction.is_empty() { 0 } else { fraction.parse::<u32>().ok()? * 10_u32.pow(9 - u32::try_from(fraction.len()).ok()?) },
    })
}
