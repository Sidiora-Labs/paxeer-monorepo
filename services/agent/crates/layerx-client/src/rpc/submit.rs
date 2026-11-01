//! Transaction submission over JSON-RPC.

use layerx_types::result::ResultCode;
use layerx_types::rung::Rung;

use crate::sequencer::{RpcRefusal, SequencerEndpoint, SequencerError};

pub type KnownResult = layerx_types::result::KnownResult;

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum Submission {
    Submitted { tx_hash: [u8; 32], rung: Rung },
    Unknown(Unknown),
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Unknown {
    raw: Vec<u8>,
}

impl Unknown {
    #[must_use]
    pub fn retry_bytes(&self) -> &[u8] {
        &self.raw
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum SubmitError {
    Refused {
        result: ResultCode,
        refusal: RpcRefusal,
    },
    Sequencer(SequencerError),
}

pub fn submit_signed(endpoint: &SequencerEndpoint, raw: &[u8]) -> Result<Submission, SubmitError> {
    match endpoint.send_raw_transaction(raw) {
        Ok(tx_hash) => Ok(Submission::Submitted {
            tx_hash,
            rung: Rung::Submitted,
        }),
        Err(SequencerError::Unavailable) => Ok(Submission::Unknown(Unknown { raw: raw.to_vec() })),
        Err(SequencerError::Rpc(refusal)) => {
            if let Some(result) = refusal_result(&refusal) {
                Err(SubmitError::Refused { result, refusal })
            } else {
                Err(SubmitError::Sequencer(SequencerError::Rpc(refusal)))
            }
        }
        Err(error) => Err(SubmitError::Sequencer(error)),
    }
}

pub fn resubmit_unknown(
    endpoint: &SequencerEndpoint,
    unknown: &Unknown,
) -> Result<Submission, SubmitError> {
    submit_signed(endpoint, unknown.retry_bytes())
}

fn refusal_result(refusal: &RpcRefusal) -> Option<ResultCode> {
    let data = refusal.data.as_ref()?.as_str()?;
    if data.starts_with("LXP_") {
        return KnownResult::from_revert_reason(data).map(ResultCode::from);
    }
    let digits = data.strip_prefix("0x")?.as_bytes();
    if digits.len() % 2 != 0 {
        return None;
    }
    let bytes = digits
        .chunks_exact(2)
        .map(|pair| Some(hex_digit(pair[0])? * 16 + hex_digit(pair[1])?))
        .collect::<Option<Vec<_>>>()?;
    let body = bytes.strip_prefix(&[0x08, 0xc3, 0x79, 0xa0])?;
    if abi_length(body.get(..32)?)? != 32 {
        return None;
    }
    let length = abi_length(body.get(32..64)?)?;
    let text = body.get(64..)?;
    if text.len() != length.checked_next_multiple_of(32)?
        || text.get(length..)?.iter().any(|byte| *byte != 0)
    {
        return None;
    }
    let reason = std::str::from_utf8(text.get(..length)?).ok()?;
    KnownResult::from_revert_reason(reason).map(ResultCode::from)
}

fn hex_digit(byte: u8) -> Option<u8> {
    match byte {
        b'0'..=b'9' => Some(byte - b'0'),
        b'a'..=b'f' => Some(byte - b'a' + 10),
        b'A'..=b'F' => Some(byte - b'A' + 10),
        _ => None,
    }
}

fn abi_length(word: &[u8]) -> Option<usize> {
    if word.get(..24)?.iter().any(|byte| *byte != 0) {
        return None;
    }
    usize::try_from(u64::from_be_bytes(word.get(24..)?.try_into().ok()?)).ok()
}
