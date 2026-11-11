use std::fmt;
use std::sync::{mpsc, Arc};
use std::time::{Duration, Instant};

use layerx_client::sequencer::{
    CallRequest, ReceiptKeyEntry, RpcRefusal, SequencerBlock, SequencerEndpoint, SequencerError,
    SequencerReceipt, StorageWitness,
};
use serde_json::{json, Value};

pub const LAYERX_AGENT_NODE_RPC_URL: &str = "LAYERX_AGENT_NODE_RPC_URL";
pub const LAYERX_AGENT_NODE_CHAIN_ID: &str = "LAYERX_AGENT_NODE_CHAIN_ID";
const DEFAULT_DEADLINE: Duration = Duration::from_secs(30);

/// Environment decoding shared by the agentd host and its env transport proof.
pub mod environment {
    use super::NodeRpcConfig;

    /// # Errors
    /// Returns an error when the required environment variable is absent or invalid.
    pub fn required(name: &str) -> Result<String, String> {
        std::env::var(name)
            .ok()
            .filter(|value| !value.is_empty())
            .ok_or_else(|| format!("{name} is required"))
    }

    /// # Errors
    /// Returns an error when the configured value is missing, malformed or outside the supported
    /// range.
    pub fn parse_u64(name: &str) -> Result<u64, String> {
        required(name)?
            .parse()
            .map_err(|_| format!("{name} must be an unsigned integer"))
    }

    /// # Errors
    /// Returns an error when the configured value is missing, malformed or outside the supported
    /// range.
    pub fn parse_digest(name: &str) -> Result<[u8; 32], String> {
        layerx_programs::hex::decode_digest(&required(name)?)
            .map_err(|error| format!("{name} is invalid: {error}"))
    }

    /// # Errors
    /// Returns an error when the configured value is missing, malformed or outside the supported
    /// range.
    pub fn human_protocol_version() -> Result<u16, String> {
        let version = required("LAYERX_AGENT_HUMAN_PROTOCOL_VERSION")?
            .parse()
            .map_err(|_| "human protocol version is invalid")?;
        if !layerx_wire::limits::protocol_version_uses_occupancy(version)
            || version != layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION
        {
            return Err("human protocol version is not the current beta protocol".to_owned());
        }
        Ok(version)
    }

    #[derive(Debug, PartialEq, Eq)]
    pub struct HumanNodeEnvironment {
        pub protocol_version: u16,
        pub expected_network_id: u32,
        pub rpc: NodeRpcConfig,
    }

    impl HumanNodeEnvironment {
        /// # Errors
        /// Returns an error if the environment or protected configuration is missing, malformed,
        /// unsupported or unreadable.
        pub fn from_environment() -> Result<Self, String> {
            let protocol_version = human_protocol_version()?;
            let expected_network_id = required("LAYERX_AGENT_HUMAN_NETWORK_ID")?
                .parse::<u32>()
                .map_err(|_| "human network id is invalid")?;
            let rpc = NodeRpcConfig::from_environment()
                .map_err(|error| format!("human node RPC configuration is invalid: {error}"))?;
            if rpc.chain_id != u64::from(expected_network_id) {
                return Err("human node RPC network id mismatch".to_owned());
            }
            Ok(Self {
                protocol_version,
                expected_network_id,
                rpc,
            })
        }
    }

    #[derive(Debug, PartialEq, Eq)]
    pub struct McpHeights {
        pub core_height: u64,
        pub expiry_height: u64,
    }

    impl McpHeights {
        /// # Errors
        /// Returns an error if the environment or protected configuration is missing, malformed,
        /// unsupported or unreadable.
        pub fn from_environment() -> Result<Self, String> {
            Ok(Self {
                expiry_height: parse_u64("LAYERX_AGENT_MCP_EXPIRY_HEIGHT")?,
                core_height: parse_u64("LAYERX_AGENT_MCP_CORE_HEIGHT")?,
            })
        }
    }

    /// # Errors
    /// Returns an error when a retired configuration or input is present.
    pub fn refuse_retired_environment() -> Result<(), String> {
        // Presence alone is an error; retain the old names to reject stale configuration.
        for name in [
            "LAYERX_AGENT_HUMAN_EXPORT_SETTLEMENT_DOMAIN",
            "LAYERX_AGENT_SEQUENCER_TRUST_HISTORY",
            "LAYERX_AGENT_GENESIS_TRUST",
            "LAYERX_AGENT_HANDOVER_FINALITY",
            "LAYERX_AGENT_MCP_WEB_SEQUENCER_PUBLIC_KEY",
            "LAYERX_AGENT_MCP_CORE_SEQUENCE",
            "LAYERX_AGENT_MCP_EXPIRY_SEQUENCE",
        ] {
            if std::env::var_os(name).is_some() {
                return Err(format!("{name} is retired"));
            }
        }
        Ok(())
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct NodeRpcConfig {
    pub url: String,
    pub chain_id: u64,
    pub deadline: Duration,
}

impl NodeRpcConfig {
    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn from_environment() -> Result<Self, NodeRpcError> {
        let url = std::env::var(LAYERX_AGENT_NODE_RPC_URL)
            .map_err(|_| NodeRpcError::MissingVariable(LAYERX_AGENT_NODE_RPC_URL))?;
        if url.is_empty() || url.trim() != url {
            return Err(NodeRpcError::InvalidVariable(LAYERX_AGENT_NODE_RPC_URL));
        }
        let chain_id = std::env::var(LAYERX_AGENT_NODE_CHAIN_ID)
            .map_err(|_| NodeRpcError::MissingVariable(LAYERX_AGENT_NODE_CHAIN_ID))?;
        let chain_id = if chain_id.starts_with("0x") {
            quantity(&chain_id)
        } else if !chain_id.is_empty() && chain_id.bytes().all(|byte| byte.is_ascii_digit()) {
            chain_id.parse().ok()
        } else {
            None
        }
        .ok_or(NodeRpcError::InvalidVariable(LAYERX_AGENT_NODE_CHAIN_ID))?;
        Ok(Self {
            url,
            chain_id,
            deadline: DEFAULT_DEADLINE,
        })
    }
}

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum NodeRpcError {
    MissingVariable(&'static str),
    InvalidVariable(&'static str),
    InvalidDeadline,
    ChainIdMismatch { expected: u64, observed: u64 },
    Unavailable,
    Sequencer(SequencerError),
}

impl fmt::Display for NodeRpcError {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::MissingVariable(name) => write!(formatter, "missing {name}"),
            Self::InvalidVariable(name) => write!(formatter, "invalid {name}"),
            Self::InvalidDeadline => formatter.write_str("invalid node JSON-RPC deadline"),
            Self::ChainIdMismatch { .. } => formatter.write_str("node chain id mismatch"),
            Self::Unavailable => formatter.write_str("node JSON-RPC is unavailable"),
            Self::Sequencer(error) => write!(formatter, "node JSON-RPC error: {error:?}"),
        }
    }
}

impl std::error::Error for NodeRpcError {}

impl From<SequencerError> for NodeRpcError {
    fn from(error: SequencerError) -> Self {
        match error {
            SequencerError::Unavailable => Self::Unavailable,
            error => Self::Sequencer(error),
        }
    }
}

#[derive(Debug)]
pub struct NodeRpc {
    endpoint: Arc<SequencerEndpoint>,
    chain_id: u64,
    deadline: Duration,
}

/// The metered stream returned by the payment precompile at one node height.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AskStreamState {
    pub stream_id: [u8; 32],
    pub recipient: [u8; 32],
    pub mode: u8,
    pub rate: u128,
    pub rate_unit: u64,
    pub total_cap: u128,
    pub accrued_total: u128,
    pub settled_total: u128,
    pub remainder_carry: u128,
    pub cumulative_meter: u64,
    pub meter_authorities: Vec<[u8; 32]>,
    pub underfunded: bool,
    pub paused: bool,
    pub closed: bool,
}

impl AskStreamState {
    /// Decode the canonical ABI answer of `getStream(bytes32)`.
    ///
    /// # Errors
    /// Returns an error if the stream ID is zero, the ABI answer is malformed or noncanonical,
    /// or the decoded values violate the metered stream constraints.
    pub fn decode(stream_id: [u8; 32], bytes: &[u8]) -> Result<Self, NodeRpcError> {
        let malformed = || NodeRpcError::from(SequencerError::Malformed);
        let word = |index: usize| -> Result<&[u8], NodeRpcError> {
            bytes
                .get(index * 32..(index + 1) * 32)
                .ok_or_else(malformed)
        };
        let number = |index: usize| -> Result<u128, NodeRpcError> {
            let value = word(index)?;
            if value[..16] != [0; 16] {
                return Err(malformed());
            }
            Ok(u128::from_be_bytes(
                value[16..].try_into().map_err(|_| malformed())?,
            ))
        };
        let small = |index: usize| -> Result<u64, NodeRpcError> {
            u64::try_from(number(index)?).map_err(|_| malformed())
        };
        let boolean = |index: usize| -> Result<bool, NodeRpcError> {
            match number(index)? {
                0 => Ok(false),
                1 => Ok(true),
                _ => Err(malformed()),
            }
        };
        // The dynamic tuple has nineteen head words and one dynamic key array.
        if stream_id == [0; 32] || number(0)? != 32 || number(16)? != 19 * 32 {
            return Err(malformed());
        }
        let count = usize::try_from(number(20)?).map_err(|_| malformed())?;
        if !(1..=8).contains(&count) || bytes.len() != (21 + count) * 32 {
            return Err(malformed());
        }
        let mut keys = Vec::with_capacity(count);
        for index in 0..count {
            let key: [u8; 32] = word(21 + index)?.try_into().map_err(|_| malformed())?;
            if key == [0; 32] || keys.last().is_some_and(|previous| previous >= &key) {
                return Err(malformed());
            }
            keys.push(key);
        }
        let state = Self {
            stream_id,
            recipient: word(3)?.try_into().map_err(|_| malformed())?,
            mode: u8::try_from(number(5)?).map_err(|_| malformed())?,
            rate: number(6)?,
            rate_unit: small(7)?,
            total_cap: number(11)?,
            accrued_total: number(12)?,
            settled_total: number(13)?,
            remainder_carry: number(14)?,
            cumulative_meter: small(15)?,
            meter_authorities: keys,
            underfunded: boolean(17)?,
            paused: boolean(18)?,
            closed: boolean(19)?,
        };
        if state.recipient == [0; 32]
            || state.mode != 2
            || state.rate == 0
            || state.rate_unit == 0
            || state.total_cap == 0
            || state.accrued_total > state.total_cap
            || state.settled_total > state.accrued_total
            || state.remainder_carry >= u128::from(state.rate_unit)
        {
            return Err(malformed());
        }
        // Reject noncanonical uint64 fields even when they do not affect admission.
        for index in 8..=10 {
            small(index)?;
        }
        Ok(state)
    }
}

impl NodeRpc {
    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn connect(config: &NodeRpcConfig) -> Result<Self, NodeRpcError> {
        Self::connect_with_roots(config, None)
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn connect_with_ca(config: &NodeRpcConfig, ca_der: &[u8]) -> Result<Self, NodeRpcError> {
        Self::connect_with_roots(config, Some(ca_der.to_vec()))
    }

    fn connect_with_roots(
        config: &NodeRpcConfig,
        ca_der: Option<Vec<u8>>,
    ) -> Result<Self, NodeRpcError> {
        let config = config.clone();
        within_deadline(config.deadline, move |expires| {
            let uri: ureq::http::Uri = config.url.parse().map_err(|_| SequencerError::Endpoint)?;
            let certificates = if uri.scheme_str() == Some("https") {
                if let Some(ca) = ca_der {
                    if ca.is_empty() {
                        return Err(SequencerError::Tls.into());
                    }
                    vec![rustls::pki_types::CertificateDer::from(ca)]
                } else {
                    let loaded = rustls_native_certs::load_native_certs();
                    if loaded.certs.is_empty() || !loaded.errors.is_empty() {
                        return Err(SequencerError::Tls.into());
                    }
                    loaded.certs
                }
            } else {
                Vec::new()
            };
            let anchors: Vec<&[u8]> = certificates.iter().map(AsRef::as_ref).collect();
            let endpoint = SequencerEndpoint::new(&config.url, &anchors)?;
            let roots: Vec<_> = anchors
                .iter()
                .map(|anchor| ureq::tls::Certificate::from_der(anchor).to_owned())
                .collect();
            let agent: ureq::Agent = ureq::Agent::config_builder()
                .timeout_global(Some(remaining(expires)?))
                .http_status_as_error(false)
                .max_redirects(0)
                .tls_config(
                    ureq::tls::TlsConfig::builder()
                        .provider(ureq::tls::TlsProvider::Rustls)
                        .root_certs(ureq::tls::RootCerts::new_with_certs(&roots))
                        .build(),
                )
                .build()
                .into();
            let observed = chain_id(&agent, &config.url)?;
            if observed != config.chain_id {
                return Err(NodeRpcError::ChainIdMismatch {
                    expected: config.chain_id,
                    observed,
                });
            }
            Ok(Self {
                endpoint: Arc::new(endpoint),
                chain_id: observed,
                deadline: config.deadline,
            })
        })
    }

    #[must_use]
    pub const fn chain_id(&self) -> u64 {
        self.chain_id
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn pending_nonce(&self, address: &[u8; 20]) -> Result<u64, NodeRpcError> {
        let address = *address;
        self.request(move |endpoint| endpoint.pending_nonce(&address))
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn estimate_gas(&self, call: &CallRequest) -> Result<u64, NodeRpcError> {
        let call = call.clone();
        self.request(move |endpoint| endpoint.estimate_gas(&call))
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn call_at(
        &self,
        height: u64,
        to: &[u8; 20],
        data: &[u8],
    ) -> Result<Vec<u8>, NodeRpcError> {
        let to = *to;
        let data = data.to_vec();
        self.request(move |endpoint| endpoint.call_at(height, &to, &data))
    }

    /// Read the ask binding and stream at the same committed node height.
    ///
    /// # Errors
    /// Returns an error if the ask ID is zero, a node transport or RPC request fails or exceeds
    /// its deadline, or the returned binding or stream state is malformed or invalid.
    pub fn ask_stream(&self, ask: &[u8; 32]) -> Result<Option<AskStreamState>, NodeRpcError> {
        if *ask == [0; 32] {
            return Err(SequencerError::Malformed.into());
        }
        let height = self.block_number()?;
        let mut payment = [0; 20];
        payment[18..].copy_from_slice(&[0x10, 0x1b]);
        let call = |signature: &[u8], argument: &[u8; 32]| {
            let digest = layerx_crypto::evm_transaction::transaction_hash(signature);
            let mut data = digest[..4].to_vec();
            data.extend_from_slice(argument);
            self.call_at(height, &payment, &data)
        };
        let answer = call(b"askStreamOf(bytes32)", ask)?;
        let id: [u8; 32] = answer
            .as_slice()
            .try_into()
            .map_err(|_| SequencerError::Malformed)?;
        if id == [0; 32] {
            return Ok(None);
        }
        AskStreamState::decode(id, &call(b"getStream(bytes32)", &id)?).map(Some)
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn send_raw_transaction(&self, raw: &[u8]) -> Result<[u8; 32], NodeRpcError> {
        let raw = raw.to_vec();
        self.request(move |endpoint| endpoint.send_raw_transaction(&raw))
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn sequencer_receipt(
        &self,
        tx_hash: &[u8; 32],
    ) -> Result<Option<SequencerReceipt>, NodeRpcError> {
        let tx_hash = *tx_hash;
        self.request(move |endpoint| endpoint.sequencer_receipt(&tx_hash))
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn receipt_keys(&self) -> Result<Vec<ReceiptKeyEntry>, NodeRpcError> {
        self.request(SequencerEndpoint::receipt_keys)
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn block_by_number(&self, height: u64) -> Result<Option<SequencerBlock>, NodeRpcError> {
        self.request(move |endpoint| endpoint.block_by_number(height))
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn block_number(&self) -> Result<u64, NodeRpcError> {
        self.request(SequencerEndpoint::block_number)
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn storage_witness(
        &self,
        address: &[u8; 20],
        slots: &[[u8; 32]],
        height: u64,
    ) -> Result<Vec<StorageWitness>, NodeRpcError> {
        let address = *address;
        let slots = slots.to_vec();
        self.request(move |endpoint| endpoint.storage_witness(&address, &slots, height))
    }

    /// # Errors
    /// Returns an error if node configuration, transport or RPC fails, or required node evidence
    /// is missing or malformed.
    pub fn bound_address(&self, public_key: &[u8; 32]) -> Result<Option<[u8; 20]>, NodeRpcError> {
        let public_key = *public_key;
        self.request(move |endpoint| endpoint.bound_address(&public_key))
    }

    fn request<T: Send + 'static>(
        &self,
        request: impl FnOnce(&SequencerEndpoint) -> Result<T, SequencerError> + Send + 'static,
    ) -> Result<T, NodeRpcError> {
        let endpoint = Arc::clone(&self.endpoint);
        within_deadline(self.deadline, move |_| {
            request(&endpoint).map_err(Into::into)
        })
    }
}

fn remaining(expires: Instant) -> Result<Duration, NodeRpcError> {
    expires
        .checked_duration_since(Instant::now())
        .filter(|duration| !duration.is_zero())
        .ok_or(NodeRpcError::Unavailable)
}

// The client has a fixed transport timeout. Bound the caller's wait as well;
// a timed-out send has an unknown outcome and must be resolved by its receipt.
fn within_deadline<T: Send + 'static>(
    deadline: Duration,
    operation: impl FnOnce(Instant) -> Result<T, NodeRpcError> + Send + 'static,
) -> Result<T, NodeRpcError> {
    if deadline.is_zero() {
        return Err(NodeRpcError::InvalidDeadline);
    }
    let expires = Instant::now()
        .checked_add(deadline)
        .ok_or(NodeRpcError::InvalidDeadline)?;
    let (sender, receiver) = mpsc::sync_channel(1);
    std::thread::Builder::new()
        .name("node-rpc".to_owned())
        .spawn(move || {
            let result = remaining(expires).and_then(|_| operation(expires));
            let _ = sender.send(result);
        })
        .map_err(|_| NodeRpcError::Unavailable)?;
    receiver
        .recv_timeout(remaining(expires)?)
        .map_err(|_| NodeRpcError::Unavailable)?
}

fn chain_id(agent: &ureq::Agent, url: &str) -> Result<u64, NodeRpcError> {
    let bytes = rpc_response(agent, url, "eth_chainId", &json!([]), 64 * 1024)?;
    let response: Value = serde_json::from_slice(&bytes).map_err(|_| SequencerError::Malformed)?;
    response["result"]
        .as_str()
        .and_then(quantity)
        .ok_or_else(|| SequencerError::Malformed.into())
}

fn rpc_response(
    agent: &ureq::Agent,
    url: &str,
    method: &str,
    params: &Value,
    maximum_response_bytes: usize,
) -> Result<Vec<u8>, NodeRpcError> {
    let mut response = agent
        .post(url)
        .header("Content-Type", "application/json")
        .header("Accept", "application/json")
        .send_json(json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}))
        .map_err(|value| transport(&value))?;
    match response.status().as_u16() {
        200 => {}
        502..=504 => return Err(NodeRpcError::Unavailable),
        status => return Err(SequencerError::HttpStatus(status).into()),
    }
    let bytes = response
        .body_mut()
        .with_config()
        .limit(maximum_response_bytes as u64)
        .read_to_vec()
        .map_err(|value| transport(&value))?;
    let response: Value = serde_json::from_slice(&bytes).map_err(|_| SequencerError::Malformed)?;
    let object = response.as_object().ok_or(SequencerError::Malformed)?;
    if object.len() != 3 || response["jsonrpc"] != "2.0" || response["id"].as_u64() != Some(1) {
        return Err(SequencerError::Malformed.into());
    }
    if let Some(error) = object.get("error") {
        let error = error.as_object().ok_or(SequencerError::Malformed)?;
        if error
            .keys()
            .any(|key| !matches!(key.as_str(), "code" | "message" | "data"))
        {
            return Err(SequencerError::Malformed.into());
        }
        return Err(SequencerError::Rpc(RpcRefusal {
            code: error
                .get("code")
                .and_then(Value::as_i64)
                .ok_or(SequencerError::Malformed)?,
            message: error
                .get("message")
                .and_then(Value::as_str)
                .ok_or(SequencerError::Malformed)?
                .to_owned(),
            data: error.get("data").cloned(),
        })
        .into());
    }
    if !object.contains_key("result") {
        return Err(SequencerError::Malformed.into());
    }
    Ok(bytes)
}

fn quantity(text: &str) -> Option<u64> {
    let digits = text.strip_prefix("0x")?;
    if digits.is_empty()
        || (digits.len() > 1 && digits.starts_with('0'))
        || !digits.bytes().all(|byte| byte.is_ascii_hexdigit())
    {
        return None;
    }
    u64::from_str_radix(digits, 16).ok()
}

fn transport(error: &ureq::Error) -> NodeRpcError {
    match error {
        ureq::Error::Timeout(_)
        | ureq::Error::Io(_)
        | ureq::Error::HostNotFound
        | ureq::Error::ConnectionFailed
        | ureq::Error::Protocol(_)
        | ureq::Error::BodyStalled => NodeRpcError::Unavailable,
        ureq::Error::Tls(_)
        | ureq::Error::Pem(_)
        | ureq::Error::Rustls(_)
        | ureq::Error::TlsRequired => SequencerError::Tls.into(),
        ureq::Error::BodyExceedsLimit(_) | ureq::Error::LargeResponseHeader(..) => {
            SequencerError::Malformed.into()
        }
        _ => SequencerError::Endpoint.into(),
    }
}
