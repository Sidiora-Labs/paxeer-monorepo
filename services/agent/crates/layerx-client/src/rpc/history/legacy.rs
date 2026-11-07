//! Compatibility decoder for the daemon's offline legacy batch evidence.
//! No registered gateway history response produces this format.

use crate::sequencer::SequencerError;
use layerx_proof::merkle::{Proof, MAX_DEPTH};

pub struct HistoryProof {
    pub proof: Proof,
    pub header: Vec<u8>,
    pub header_signature: [u8; 64],
}

impl HistoryProof {
    /// # Errors
    /// Refuses malformed, truncated or noncanonical inclusion proof fields.
    pub fn decode(bytes: &[u8]) -> Result<Self, SequencerError> {
        let mut reader = Reader::new(bytes);
        let _asserted_level = reader.u8()?;
        let _root: [u8; 32] = reader.array()?;
        let leaf_index = reader.u32()?;
        let leaf_count = reader.u32()?;
        let sibling_count = usize::from(reader.u8()?);
        if sibling_count > MAX_DEPTH {
            return Err(SequencerError::Malformed);
        }
        let mut siblings = Vec::with_capacity(sibling_count);
        for _ in 0..sibling_count {
            siblings.push(reader.array()?);
        }
        let header_length =
            usize::try_from(reader.u32()?).map_err(|_| SequencerError::Malformed)?;
        let header = reader.bytes(header_length)?.to_vec();
        let header_signature = reader.array()?;
        reader.finish()?;
        let proof =
            Proof::new(leaf_index, leaf_count, siblings).map_err(|_| SequencerError::Malformed)?;
        Ok(Self {
            proof,
            header,
            header_signature,
        })
    }
}

struct Reader<'a> {
    bytes: &'a [u8],
    offset: usize,
}

impl<'a> Reader<'a> {
    const fn new(bytes: &'a [u8]) -> Self {
        Self { bytes, offset: 0 }
    }

    fn bytes(&mut self, length: usize) -> Result<&'a [u8], SequencerError> {
        let end = self
            .offset
            .checked_add(length)
            .ok_or(SequencerError::Malformed)?;
        let value = self
            .bytes
            .get(self.offset..end)
            .ok_or(SequencerError::Malformed)?;
        self.offset = end;
        Ok(value)
    }

    fn array<const LENGTH: usize>(&mut self) -> Result<[u8; LENGTH], SequencerError> {
        self.bytes(LENGTH)?
            .try_into()
            .map_err(|_| SequencerError::Malformed)
    }

    fn u8(&mut self) -> Result<u8, SequencerError> {
        self.array().map(u8::from_be_bytes)
    }

    fn u32(&mut self) -> Result<u32, SequencerError> {
        self.array().map(u32::from_be_bytes)
    }

    fn finish(self) -> Result<(), SequencerError> {
        if self.offset == self.bytes.len() {
            Ok(())
        } else {
            Err(SequencerError::Malformed)
        }
    }
}
