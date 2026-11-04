//! History reads over JSON-RPC.

use crate::rpc::{evidence::RootSelector, Head, Version};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::merkle::{Proof, MAX_DEPTH};
use layerx_types::verify::VerificationLevel;
use serde_json::{json, Value};

use crate::sequencer::{SequencerEndpoint, SequencerError};

pub type HistoryPage = crate::sequencer::HistoryPage;
pub type PxEndpoint = crate::px::PxEndpoint;

pub fn history(
    endpoint: &SequencerEndpoint,
    account: &str,
    cursor: Option<&str>,
    limit: Option<u16>,
) -> Result<HistoryPage, SequencerError> {
    endpoint.history(account, cursor, u64::from(limit.unwrap_or(50)))
}

pub fn did_evm_address(
    endpoint: &PxEndpoint,
    did: &str,
) -> Result<Option<[u8; 20]>, SequencerError> {
    let account = endpoint.account(did)?;
    if account.account.bound && account.account.layerx_did.as_deref() == Some(did) {
        Ok(account.account.evm_address)
    } else {
        Ok(None)
    }
}

pub fn did_accounts(endpoint: &PxEndpoint, did: &str) -> Result<Vec<Value>, SequencerError> {
    let account = endpoint.account(did)?.account;
    let Some(address) = account.evm_address else {
        return Ok(vec![]);
    };
    if !account.bound || account.layerx_did.as_deref() != Some(did) {
        return Ok(vec![]);
    }
    let hex = |bytes: &[u8]| {
        bytes
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    };
    Ok(vec![json!({
        "evm_address": format!("0x{}", hex(&address)),
        "pax_address": account.pax_address,
        "layerx_did": account.layerx_did,
        "layerx_account": account.layerx_account.as_ref().map(|id| hex(id)),
        "bound": account.bound
    })])
}

/// A caller-requested evidence floor. Returned values carry only the level
/// actually established by local verification.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Requested(VerificationLevel);

impl Requested {
    #[must_use]
    pub const fn new(level: VerificationLevel) -> Self {
        Self(level)
    }

    #[must_use]
    pub const fn level(self) -> VerificationLevel {
        self.0
    }
}

/// Trusted client context for state and history proof verification.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct ReadContext {
    pub interface_version: Version,
    pub correlation_id: u64,
    pub expected_protocol_version: u16,
    pub expected_network_id: u32,
    pub requested: Requested,
    pub head: Head,
    pub sequencer_authorization: SequencerAuthorization,
    pub handshake_sequencer_key: [u8; 32],
    pub root_selector: RootSelector,
}

/// Exact core history record class.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum HistoryKind {
    Activity,
    Receipt,
    Event,
}

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
