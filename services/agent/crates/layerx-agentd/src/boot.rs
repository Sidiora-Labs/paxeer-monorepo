//! Startup and reconnection readiness for the node JSON-RPC connection.

use layerx_client::sequencer::ReceiptKeyEntry;

use crate::config::StartupConfig;
use crate::node_rpc::{NodeRpc, NodeRpcError};
use crate::protocol_evidence::{EvidenceAuthority, ProtocolEvidenceVerifier, VerifierPolicyError};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ReadinessStatus {
    pub generation: u64,
    pub chain_id: u64,
    pub receipt_keys: Vec<ReceiptKeyEntry>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Status {
    NotReady,
    Ready(ReadinessStatus),
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum WriteGateError {
    NotReady,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum GateError {
    NodeRpc(NodeRpcError),
    NoReceiptKeys,
    Evidence(VerifierPolicyError),
}

impl From<NodeRpcError> for GateError {
    fn from(value: NodeRpcError) -> Self {
        Self::NodeRpc(value)
    }
}

#[derive(Clone, Debug)]
pub struct Gate {
    status: Status,
    generation: u64,
    evidence: EvidenceAuthority,
}

impl Gate {
    /// Loads the protected evidence policy before opening any write gate.
    ///
    /// # Errors
    ///
    /// Refuses an unavailable or malformed configured authority source.
    pub fn new(config: &StartupConfig) -> Result<Self, GateError> {
        let verifier = ProtocolEvidenceVerifier::load(config).map_err(GateError::Evidence)?;
        Ok(Self {
            status: Status::NotReady,
            generation: 0,
            evidence: EvidenceAuthority::new(verifier),
        })
    }

    #[must_use]
    pub const fn status(&self) -> &Status {
        &self.status
    }

    pub fn disconnected(&mut self) {
        self.status = Status::NotReady;
    }

    /// Refreshes readiness using a chain-checked connection and its current receipt keys.
    ///
    /// # Errors
    ///
    /// Leaves the daemon not ready on every RPC or receipt-key failure, including
    /// a failed refresh of a previously ready connection.
    pub fn readiness_gate(&mut self, node: &NodeRpc) -> Result<&ReadinessStatus, GateError> {
        self.disconnected();
        let mut status = readiness_gate(node)?;
        if !status.receipt_keys.iter().all(|key| {
            self.evidence
                .verifier()
                .accepts_handshake_key(key.first_height, key.public_key)
                && self
                    .evidence
                    .verifier()
                    .accepts_handshake_key(key.last_height, key.public_key)
        }) {
            return Err(GateError::Evidence(VerifierPolicyError::HandshakeKey));
        }
        self.generation = self.generation.saturating_add(1);
        status.generation = self.generation;
        self.status = Status::Ready(status);
        match &self.status {
            Status::Ready(status) => Ok(status),
            Status::NotReady => unreachable!(),
        }
    }

    /// Executes a write only after the current node readiness check succeeds.
    ///
    /// # Errors
    ///
    /// Returns `NotReady` before startup or after disconnect or refusal.
    pub fn guard_write<T>(&self, operation: impl FnOnce() -> T) -> Result<T, WriteGateError> {
        match &self.status {
            Status::Ready(_) => Ok(operation()),
            Status::NotReady => Err(WriteGateError::NotReady),
        }
    }

    /// Returns the configured evidence authority while the node is ready and its
    /// receipt keys agree with the protected evidence policy.
    ///
    /// # Errors
    ///
    /// Refuses a disconnected node or keys absent from the configured authority.
    pub fn evidence_authority(&self) -> Result<&EvidenceAuthority, WriteGateError> {
        match &self.status {
            Status::Ready(status)
                if status.receipt_keys.iter().all(|key| {
                    self.evidence
                        .verifier()
                        .accepts_handshake_key(key.first_height, key.public_key)
                        && self
                            .evidence
                            .verifier()
                            .accepts_handshake_key(key.last_height, key.public_key)
                }) =>
            {
                Ok(&self.evidence)
            }
            Status::Ready(_) | Status::NotReady => Err(WriteGateError::NotReady),
        }
    }
}

/// Checks readiness over the real node connection. `NodeRpc::connect` has already
/// compared `eth_chainId` with the configured chain id; an unvalidated connection
/// cannot be constructed through its public API. Keys are fetched on every call.
///
/// # Errors
///
/// Refuses unavailable or malformed RPC answers and an empty receipt-key set.
pub fn readiness_gate(node: &NodeRpc) -> Result<ReadinessStatus, GateError> {
    let receipt_keys = node.receipt_keys()?;
    if receipt_keys.is_empty() {
        return Err(GateError::NoReceiptKeys);
    }
    Ok(ReadinessStatus {
        generation: 0,
        chain_id: node.chain_id(),
        receipt_keys,
    })
}
