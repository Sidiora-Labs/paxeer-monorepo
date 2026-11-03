//! Owner receipt lookup wire contract: presence 0 is Absent, 2 is Submitted
//! (transaction hash and nonce), and 1 is Found followed by ReceiptLookupFrame.
//! Verification uses the Human codes 1 SEQUENCER_SIGNED, 2 BATCH_INCLUDED,
//! 3 STATE_PROVEN, 4 CHECKPOINT_FINALISED, 5 SETTLEMENT_ANCHORED.
//! The rung is Rung::wire_rank (0 Submitted, 1 Receipted, 2 Committed).

use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::{result::KnownResult, rung::Rung, verify::VerificationLevel};
use layerx_wire::{decode::Decoder, encode::Encoder, WireError};

const MAX_BYTES: usize = 1_048_576;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReceiptLookupFrame {
    pub canonical_bytes: Vec<u8>,
    pub authorised_batch: AuthorizedBatch,
    pub verification_level: VerificationLevel,
    pub tx_hash: [u8; 32],
    pub receipt_hash: [u8; 32],
    pub block_height: u64,
    pub rung: Rung,
    pub signed_receipt: Vec<u8>,
    pub receipt_key_id: [u8; 32],
    pub public_key: [u8; 32],
    pub first_height: u64,
    pub last_height: u64,
    pub inclusion_height: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ReceiptLookupAnswer {
    Absent,
    Submitted { tx_hash: [u8; 32], nonce: u64 },
    Found(ReceiptLookupFrame),
}

fn invalid(offset: usize) -> WireError {
    WireError {
        result: KnownResult::NonCanonical.into(),
        offset,
    }
}

fn fixed(decoder: &mut Decoder<'_>) -> Result<[u8; 32], WireError> {
    let offset = decoder.offset();
    decoder.fixed(32)?.try_into().map_err(|_| invalid(offset))
}

impl ReceiptLookupFrame {
    pub fn encode(&self) -> Result<Vec<u8>, WireError> {
        let mut encoder = Encoder::new(MAX_BYTES);
        self.write(&mut encoder)?;
        Ok(encoder.finish())
    }

    fn write(&self, encoder: &mut Encoder) -> Result<(), WireError> {
        if self.canonical_bytes.is_empty()
            || self.signed_receipt.is_empty()
            || self.verification_level == VerificationLevel::UNVERIFIED
        {
            return Err(invalid(encoder.as_bytes().len()));
        }
        encoder.bytes(&self.canonical_bytes, MAX_BYTES)?;
        for field in [
            self.authorised_batch.batch_id(),
            self.authorised_batch.asset(),
            self.authorised_batch.previous_state_root(),
            self.authorised_batch.resulting_state_root(),
            self.authorised_batch.sequencer_public_key(),
        ] {
            encoder.fixed(&field)?;
        }
        encoder.u8(self.verification_level.wire_rank())?;
        encoder.fixed(&self.tx_hash)?;
        encoder.fixed(&self.receipt_hash)?;
        encoder.u64(self.block_height)?;
        encoder.u8(self.rung.wire_rank())?;
        encoder.bytes(&self.signed_receipt, MAX_BYTES)?;
        encoder.fixed(&self.receipt_key_id)?;
        encoder.fixed(&self.public_key)?;
        encoder.u64(self.first_height)?;
        encoder.u64(self.last_height)?;
        encoder.u64(self.inclusion_height)
    }

    pub fn decode(bytes: &[u8]) -> Result<Self, WireError> {
        let mut decoder = Decoder::new(bytes, MAX_BYTES);
        let value = Self::decode_from(&mut decoder)?;
        decoder.finish()?;
        Ok(value)
    }

    pub fn decode_from(decoder: &mut Decoder<'_>) -> Result<Self, WireError> {
        let canonical_bytes = decoder.bytes_owned(MAX_BYTES)?;
        let authorised_batch = AuthorizedBatch::new(
            fixed(decoder)?,
            fixed(decoder)?,
            fixed(decoder)?,
            fixed(decoder)?,
            fixed(decoder)?,
        );
        let verification_level = match decoder.u8()? {
            1 => VerificationLevel::SEQUENCER_SIGNED,
            2 => VerificationLevel::BATCH_INCLUDED,
            3 => VerificationLevel::STATE_PROVEN,
            4 => VerificationLevel::CHECKPOINT_FINALISED,
            5 => VerificationLevel::SETTLEMENT_ANCHORED,
            _ => return Err(invalid(decoder.offset())),
        };
        let tx_hash = fixed(decoder)?;
        let receipt_hash = fixed(decoder)?;
        let block_height = decoder.u64()?;
        let rung = match decoder.u8()? {
            0 => Rung::Submitted,
            1 => Rung::Receipted,
            2 => Rung::Committed,
            _ => return Err(invalid(decoder.offset())),
        };
        let signed_receipt = decoder.bytes_owned(MAX_BYTES)?;
        if canonical_bytes.is_empty() || signed_receipt.is_empty() {
            return Err(invalid(decoder.offset()));
        }
        Ok(Self {
            canonical_bytes,
            authorised_batch,
            verification_level,
            tx_hash,
            receipt_hash,
            block_height,
            rung,
            signed_receipt,
            receipt_key_id: fixed(decoder)?,
            public_key: fixed(decoder)?,
            first_height: decoder.u64()?,
            last_height: decoder.u64()?,
            inclusion_height: decoder.u64()?,
        })
    }
}

impl ReceiptLookupAnswer {
    pub fn encode(&self) -> Result<Vec<u8>, WireError> {
        let mut encoder = Encoder::new(MAX_BYTES);
        match self {
            Self::Absent => encoder.u8(0)?,
            Self::Submitted { tx_hash, nonce } => {
                encoder.u8(2)?;
                encoder.fixed(tx_hash)?;
                encoder.u64(*nonce)?;
            }
            Self::Found(frame) => {
                encoder.u8(1)?;
                frame.write(&mut encoder)?;
            }
        }
        Ok(encoder.finish())
    }

    pub fn decode(bytes: &[u8]) -> Result<Self, WireError> {
        let mut decoder = Decoder::new(bytes, MAX_BYTES);
        let value = match decoder.u8()? {
            0 => Self::Absent,
            2 => Self::Submitted {
                tx_hash: fixed(&mut decoder)?,
                nonce: decoder.u64()?,
            },
            1 => Self::Found(ReceiptLookupFrame::decode_from(&mut decoder)?),
            _ => return Err(invalid(decoder.offset())),
        };
        decoder.finish()?;
        Ok(value)
    }
}
