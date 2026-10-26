use std::fmt;

use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys, VerificationFailure};
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog, SequencerLogRefusal};
use serde_json::Value;
use sha2::{Digest as _, Sha256};

use crate::hex;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Rung {
    Submitted,
    Receipted,
    Committed,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReceiptLog {
    pub address: [u8; 20],
    pub topics: Vec<[u8; 32]>,
    pub data: Vec<u8>,
    pub log_index: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReceiptView {
    pub seq: u64,
    pub height: u64,
    pub index: u64,
    pub timestamp: u64,
    pub status: u64,
    pub return_data: Vec<u8>,
    pub logs: Vec<ReceiptLog>,
    pub gas_used: u64,
    pub cumulative_gas_used: u64,
    pub effective_gas_price: u64,
    pub fee_denom: String,
    pub fee_amount: u128,
    pub sponsor: [u8; 20],
    pub from: [u8; 20],
    pub to: [u8; 20],
    pub contract_address: [u8; 20],
    pub vm_error: String,
    pub key_id: [u8; 32],
    pub receipt: Vec<u8>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SequencerReceipt {
    pub rung: Rung,
    pub tx_hash: [u8; 32],
    pub nonce: u64,
    pub view: Option<ReceiptView>,
    pub block: Option<(u64, [u8; 32])>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ReceiptError {
    Malformed,
    Verification(VerificationFailure),
    Mismatch(&'static str),
    Logs(SequencerLogRefusal),
}

impl fmt::Display for ReceiptError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Malformed => formatter.write_str("sequencer receipt answer malformed"),
            Self::Verification(failure) => {
                write!(formatter, "sequencer receipt refused: {failure:?}")
            }
            Self::Mismatch(field) => {
                write!(formatter, "sequencer receipt differs from JSON {field}")
            }
            Self::Logs(failure) => write!(formatter, "sequencer receipt logs refused: {failure:?}"),
        }
    }
}

impl std::error::Error for ReceiptError {}

const VIEW_FIELDS: [&str; 19] = [
    "seq",
    "height",
    "index",
    "timestamp",
    "status",
    "returnData",
    "logs",
    "gasUsed",
    "cumulativeGasUsed",
    "effectiveGasPrice",
    "feeDenom",
    "feeAmount",
    "sponsor",
    "from",
    "to",
    "contractAddress",
    "vmError",
    "keyId",
    "receipt",
];

fn text<'a>(value: &'a Value, field: &str) -> Result<&'a str, ReceiptError> {
    value
        .get(field)
        .and_then(Value::as_str)
        .ok_or(ReceiptError::Malformed)
}

fn quantity_text<'a>(value: &'a Value, field: &str) -> Result<&'a str, ReceiptError> {
    let quantity = text(value, field)?;
    let digits = quantity.strip_prefix("0x").ok_or(ReceiptError::Malformed)?;
    if digits.is_empty()
        || (digits.len() > 1 && digits.starts_with('0'))
        || !digits.bytes().all(|byte| byte.is_ascii_hexdigit())
    {
        return Err(ReceiptError::Malformed);
    }
    Ok(quantity)
}

fn quantity(value: &Value, field: &str) -> Result<u64, ReceiptError> {
    hex::parse_quantity(quantity_text(value, field)?).map_err(|_| ReceiptError::Malformed)
}

fn fixed<const N: usize>(value: &Value, field: &str) -> Result<[u8; N], ReceiptError> {
    hex::fixed(text(value, field)?).map_err(|_| ReceiptError::Malformed)
}

fn bytes(value: &Value, field: &str) -> Result<Vec<u8>, ReceiptError> {
    hex::decode(text(value, field)?).map_err(|_| ReceiptError::Malformed)
}

impl ReceiptLog {
    fn decode(value: &Value) -> Result<Self, ReceiptError> {
        let topics = value
            .get("topics")
            .and_then(Value::as_array)
            .ok_or(ReceiptError::Malformed)?;
        if topics.len() > 4 {
            return Err(ReceiptError::Malformed);
        }
        Ok(Self {
            address: fixed(value, "address")?,
            topics: topics
                .iter()
                .map(|topic| {
                    hex::fixed(topic.as_str().ok_or(ReceiptError::Malformed)?)
                        .map_err(|_| ReceiptError::Malformed)
                })
                .collect::<Result<_, _>>()?,
            data: bytes(value, "data")?,
            log_index: quantity(value, "logIndex")?,
        })
    }
}

impl ReceiptView {
    fn decode(value: &Value) -> Result<Self, ReceiptError> {
        let status = quantity(value, "status")?;
        if status > 1 {
            return Err(ReceiptError::Malformed);
        }
        Ok(Self {
            seq: quantity(value, "seq")?,
            height: quantity(value, "height")?,
            index: quantity(value, "index")?,
            timestamp: quantity(value, "timestamp")?,
            status,
            return_data: bytes(value, "returnData")?,
            logs: value
                .get("logs")
                .and_then(Value::as_array)
                .ok_or(ReceiptError::Malformed)?
                .iter()
                .map(ReceiptLog::decode)
                .collect::<Result<_, _>>()?,
            gas_used: quantity(value, "gasUsed")?,
            cumulative_gas_used: quantity(value, "cumulativeGasUsed")?,
            effective_gas_price: quantity(value, "effectiveGasPrice")?,
            fee_denom: text(value, "feeDenom")?.to_owned(),
            fee_amount: hex::parse_quantity_u128(quantity_text(value, "feeAmount")?)
                .map_err(|_| ReceiptError::Malformed)?,
            sponsor: fixed(value, "sponsor")?,
            from: fixed(value, "from")?,
            to: fixed(value, "to")?,
            contract_address: fixed(value, "contractAddress")?,
            vm_error: text(value, "vmError")?.to_owned(),
            key_id: fixed(value, "keyId")?,
            receipt: bytes(value, "receipt")?,
        })
    }
}

impl SequencerReceipt {
    pub fn decode(value: &Value) -> Result<Self, ReceiptError> {
        let rung = match text(value, "rung")? {
            "submitted" => Rung::Submitted,
            "receipted" => Rung::Receipted,
            "committed" => Rung::Committed,
            _ => return Err(ReceiptError::Malformed),
        };
        let view = if rung == Rung::Submitted {
            if VIEW_FIELDS.iter().any(|field| value.get(*field).is_some()) {
                return Err(ReceiptError::Malformed);
            }
            None
        } else {
            Some(ReceiptView::decode(value)?)
        };
        let block = if rung == Rung::Committed {
            Some((quantity(value, "blockNumber")?, fixed(value, "blockHash")?))
        } else {
            if value.get("blockNumber").is_some() || value.get("blockHash").is_some() {
                return Err(ReceiptError::Malformed);
            }
            None
        };
        let answer = Self {
            rung,
            tx_hash: fixed(value, "txHash")?,
            nonce: quantity(value, "nonce")?,
            view,
            block,
        };
        answer.check_shape()?;
        Ok(answer)
    }

    fn check_shape(&self) -> Result<(), ReceiptError> {
        match (self.rung, &self.view, self.block) {
            (Rung::Submitted, None, None) | (Rung::Receipted, Some(_), None) => Ok(()),
            (Rung::Committed, Some(view), Some((number, _))) if number >= view.height => Ok(()),
            _ => Err(ReceiptError::Malformed),
        }
    }

    pub fn verify(&self, keys: &[SequencerAuthorization]) -> Result<(), ReceiptError> {
        self.check_shape()?;
        let view = self.view.as_ref().ok_or(ReceiptError::Malformed)?;
        let verified = verify_sequencer_signature(&view.receipt, ReceiptKeys::HeightRanges(keys))
            .map_err(ReceiptError::Verification)?;
        let receipt = verified.protocol().ok_or(ReceiptError::Malformed)?;
        let height = u64::from_be_bytes(
            receipt.batch_id()[24..]
                .try_into()
                .map_err(|_| ReceiptError::Malformed)?,
        );
        let status = match receipt.result_code() {
            0 => 1,
            1 => 0,
            _ => return Err(ReceiptError::Mismatch("status")),
        };
        for (matches, field) in [
            (receipt.global_sequence() == view.seq, "seq"),
            (height == view.height, "height"),
            (status == view.status, "status"),
            (receipt.activity_id() == self.tx_hash, "txHash"),
            (receipt.debit_sequence() == self.nonce, "nonce"),
            (receipt.context_hash() == view.key_id, "keyId"),
            (receipt.timestamp() == view.timestamp, "timestamp"),
            (receipt.fee_charged() == view.fee_amount, "feeAmount"),
            (
                receipt.asset() == <[u8; 32]>::from(Sha256::digest(view.fee_denom.as_bytes())),
                "feeDenom",
            ),
            (
                receipt.from()[..12] == [0; 12] && receipt.from()[12..] == view.from,
                "from",
            ),
            (
                receipt.to()[..12] == [0; 12] && receipt.to()[12..] == view.to,
                "to",
            ),
            (
                receipt.activity_root() == <[u8; 32]>::from(Sha256::digest(&view.return_data)),
                "returnData",
            ),
        ] {
            if !matches {
                return Err(ReceiptError::Mismatch(field));
            }
        }
        let logs = view
            .logs
            .iter()
            .map(|log| {
                SequencerLog::new(
                    log.address,
                    log.topics.clone(),
                    log.data.clone(),
                    log.log_index,
                )
            })
            .collect();
        let verified_logs = verify_logs(&verified, logs).map_err(ReceiptError::Logs)?;
        let summary = verified_logs.summary();
        for (matches, field) in [
            (u64::from(summary.tx_index()) == view.index, "index"),
            (summary.gas_used() == view.gas_used, "gasUsed"),
            (
                summary.cumulative_gas_used() == view.cumulative_gas_used,
                "cumulativeGasUsed",
            ),
            (
                summary.effective_gas_price() == view.effective_gas_price,
                "effectiveGasPrice",
            ),
            (summary.fee_denom() == view.fee_denom, "feeDenom"),
            (summary.sponsor() == view.sponsor, "sponsor"),
            (
                summary.contract_address() == view.contract_address,
                "contractAddress",
            ),
            (
                summary.vm_error_hash()
                    == <[u8; 32]>::from(Sha256::digest(view.vm_error.as_bytes())),
                "vmError",
            ),
            (
                usize::try_from(summary.return_data_len()).ok() == Some(view.return_data.len()),
                "returnData",
            ),
        ] {
            if !matches {
                return Err(ReceiptError::Mismatch(field));
            }
        }
        Ok(())
    }
}
