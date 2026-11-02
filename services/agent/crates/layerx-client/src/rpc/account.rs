//! Account reads over JSON-RPC.

use layerx_types::amount::Amount;

use crate::sequencer::{SequencerEndpoint, SequencerError};

pub type Rung = layerx_types::rung::Rung;

const LAYERX_LEDGER: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
];

#[allow(non_upper_case_globals)]
const balanceOf: [u8; 4] = [0xb1, 0xff, 0xc0, 0x3e];

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Requested(Rung);

impl Requested {
    pub const fn new(rung: Rung) -> Result<Self, ReadError> {
        match rung {
            Rung::Submitted => Err(ReadError::Submitted),
            Rung::Receipted | Rung::Committed => Ok(Self(rung)),
        }
    }

    #[must_use]
    pub const fn level(self) -> Rung {
        self.0
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ReadContext {
    pub committed_height: u64,
    pub requested: Requested,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ReadError {
    Rpc(SequencerError),
    MalformedValue,
    Submitted,
}

impl From<SequencerError> for ReadError {
    fn from(error: SequencerError) -> Self {
        Self::Rpc(error)
    }
}

pub fn balance(
    endpoint: &SequencerEndpoint,
    account: [u8; 32],
    asset: [u8; 32],
    context: ReadContext,
) -> Result<Amount, ReadError> {
    let mut data = Vec::with_capacity(68);
    data.extend_from_slice(&balanceOf);
    data.extend_from_slice(&account);
    data.extend_from_slice(&asset);
    let word = endpoint.call_at(context.committed_height, &LAYERX_LEDGER, &data)?;
    if word.len() != 32 || word[..16].iter().any(|byte| *byte != 0) {
        return Err(ReadError::MalformedValue);
    }
    let amount = word[16..]
        .try_into()
        .map_err(|_| ReadError::MalformedValue)?;
    Ok(Amount::from_be_bytes(amount))
}
/// Freshness coordinates inseparable from a returned read value.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Freshness {
    pub global_sequence: u64,
    pub batch_number: u64,
    pub observed_head_sequence: u64,
    pub observed_checkpoint: [u8; 32],
}
