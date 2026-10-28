//! JSON-RPC transport seam. Ethereum reads go through the strict-majority
//! HTTPS quorum of `layerx_mirror::rpc::RpcCluster`; Paxeer reads go through
//! the relayer's pinned-TLS `PaxeerTransport`, with a strict majority across
//! the configured Paxeer endpoints.

use std::collections::BTreeMap;
use std::fmt;

use layerx_mirror::rpc::{BroadcastResult, RpcCluster, RpcError};
use serde_json::{json, Value};

use crate::hex;
use crate::transport::{PaxeerEndpoint, PaxeerTransport, TransportFault};

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RpcFault {
    Configuration,
    Unavailable,
    RateLimited { retry_after_seconds: u64 },
    Divergence,
    Malformed,
    Rejected { code: i64, message: String },
}

impl fmt::Display for RpcFault {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Configuration => formatter.write_str("rpc configuration refused"),
            Self::Unavailable => formatter.write_str("rpc quorum unavailable"),
            Self::RateLimited {
                retry_after_seconds,
            } => write!(formatter, "rpc rate limited for {retry_after_seconds}s"),
            Self::Divergence => formatter.write_str("rpc endpoints diverge"),
            Self::Malformed => formatter.write_str("rpc response malformed"),
            Self::Rejected { code, message } => write!(formatter, "rpc rejected {code}: {message}"),
        }
    }
}

impl std::error::Error for RpcFault {}

impl From<RpcError> for RpcFault {
    fn from(value: RpcError) -> Self {
        match value {
            RpcError::Configuration => Self::Configuration,
            RpcError::Unavailable => Self::Unavailable,
            RpcError::RateLimited {
                retry_after_seconds,
            } => Self::RateLimited {
                retry_after_seconds,
            },
            RpcError::Divergence => Self::Divergence,
            RpcError::ResponseMismatch => Self::Malformed,
            RpcError::Rejected { code, message } => Self::Rejected { code, message },
        }
    }
}

/// Result of broadcasting already signed bytes. `Unknown` is never
/// permission to sign a replacement: the same bytes are rebroadcast.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SendOutcome {
    Accepted,
    Unknown,
}

/// One chain's JSON-RPC. Reads must be agreed values; sends take the exact
/// signed bytes and the transaction hash they must be acknowledged with.
pub trait JsonRpc {
    /// # Errors
    ///
    /// Returns the transport's fault when no agreed value is available.
    fn call(&self, method: &str, params: Value) -> Result<Value, RpcFault>;

    /// # Errors
    ///
    /// Returns the transport's fault when no agreed receipt is available.
    fn sequencer_receipt(&self, hash: &[u8; 32]) -> Result<Option<Value>, RpcFault> {
        self.call("pax_sequencerReceipt", json!([hex::prefixed(hash)]))
            .map(|value| if value.is_null() { None } else { Some(value) })
    }

    /// # Errors
    ///
    /// Returns `Rejected` when the endpoints deterministically refuse the
    /// transaction.
    fn send_raw_transaction(&self, raw: &[u8], hash: &[u8; 32]) -> Result<SendOutcome, RpcFault>;
}

impl JsonRpc for RpcCluster {
    fn call(&self, method: &str, params: Value) -> Result<Value, RpcFault> {
        RpcCluster::call(self, method, params).map_err(RpcFault::from)
    }

    fn send_raw_transaction(&self, raw: &[u8], hash: &[u8; 32]) -> Result<SendOutcome, RpcFault> {
        match self.broadcast(
            "eth_sendRawTransaction",
            json!([hex::prefixed(raw)]),
            &hex::prefixed(hash),
        ) {
            Ok(BroadcastResult::Accepted) => Ok(SendOutcome::Accepted),
            Ok(BroadcastResult::Unknown) => Ok(SendOutcome::Unknown),
            Err(error) => Err(RpcFault::from(error)),
        }
    }
}

/// Paxeer JSON-RPC over the endpoints the relayer's `PaxeerTransport` authenticates
/// (pinned TLS, or an explicit loopback emulator). Every request re-checks
/// the endpoint's chain id; a value is accepted only when a strict majority of
/// the configured endpoints return it byte for byte.
pub struct PaxeerRpc {
    endpoints: Vec<PaxeerEndpoint>,
}

impl PaxeerRpc {
    /// # Errors
    ///
    /// Refuses an empty or oversized endpoint list, or endpoints bound to
    /// different chain ids.
    pub fn new(endpoints: Vec<PaxeerEndpoint>) -> Result<Self, RpcFault> {
        let Some(first) = endpoints.first() else {
            return Err(RpcFault::Configuration);
        };
        if endpoints.len() > 8
            || endpoints
                .iter()
                .any(|endpoint| endpoint.expected_chain_id != first.expected_chain_id)
        {
            return Err(RpcFault::Configuration);
        }
        Ok(Self { endpoints })
    }

    fn majority(&self) -> usize {
        self.endpoints.len() / 2 + 1
    }

    fn request(&self, method: &str, params: &Value) -> Vec<Result<Value, RpcFault>> {
        self.endpoints
            .iter()
            .map(|endpoint| {
                PaxeerTransport::call(endpoint, method, params).map_err(|fault| match fault {
                    TransportFault::Rpc { code, message } => RpcFault::Rejected { code, message },
                    _ => RpcFault::Unavailable,
                })
            })
            .collect()
    }
}

impl JsonRpc for PaxeerRpc {
    fn call(&self, method: &str, params: Value) -> Result<Value, RpcFault> {
        let mut agreed: BTreeMap<String, (usize, Value)> = BTreeMap::new();
        let mut refusals: BTreeMap<(i64, String), usize> = BTreeMap::new();
        for outcome in self.request(method, &params) {
            match outcome {
                Ok(value) => {
                    let entry = agreed.entry(value.to_string()).or_insert((0, value));
                    entry.0 += 1;
                }
                Err(RpcFault::Rejected { code, message }) => {
                    *refusals.entry((code, message)).or_default() += 1;
                }
                Err(_) => {}
            }
        }
        let majority = self.majority();
        let divergent = agreed.len() > 1;
        if let Some((_, value)) = agreed.into_values().find(|(count, _)| *count >= majority) {
            return Ok(value);
        }
        if let Some(((code, message), _)) =
            refusals.into_iter().find(|(_, count)| *count >= majority)
        {
            return Err(RpcFault::Rejected { code, message });
        }
        Err(if divergent {
            RpcFault::Divergence
        } else {
            RpcFault::Unavailable
        })
    }

    fn send_raw_transaction(&self, raw: &[u8], hash: &[u8; 32]) -> Result<SendOutcome, RpcFault> {
        let expected = hex::prefixed(hash);
        let mut refusals: BTreeMap<(i64, String), usize> = BTreeMap::new();
        for outcome in self.request("eth_sendRawTransaction", &json!([hex::prefixed(raw)])) {
            match outcome {
                Ok(value) if value.as_str() == Some(expected.as_str()) => {
                    return Ok(SendOutcome::Accepted);
                }
                Err(RpcFault::Rejected { code, message }) => {
                    *refusals.entry((code, message)).or_default() += 1;
                }
                _ => {}
            }
        }
        if let Some(((code, message), _)) = refusals
            .into_iter()
            .find(|(_, count)| *count >= self.majority())
        {
            return Err(RpcFault::Rejected { code, message });
        }
        Ok(SendOutcome::Unknown)
    }
}
