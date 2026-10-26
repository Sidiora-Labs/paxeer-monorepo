use std::fmt;
use std::sync::{mpsc, Arc};
use std::time::{Duration, Instant};

use layerx_client::sequencer::{
    CallRequest, ReceiptKeyEntry, RpcRefusal, SequencerBlock, SequencerEndpoint, SequencerError,
    SequencerReceipt,
};
use serde_json::{json, Value};

const RPC_URL: &str = "LAYERX_AGENT_NODE_RPC_URL";
const CHAIN_ID: &str = "LAYERX_AGENT_NODE_CHAIN_ID";
const DEFAULT_DEADLINE: Duration = Duration::from_secs(30);

#[derive(Clone, Debug, PartialEq, Eq)]
pub struct NodeRpcConfig {
    pub url: String,
    pub chain_id: u64,
    pub deadline: Duration,
}

impl NodeRpcConfig {
    pub fn from_environment() -> Result<Self, NodeRpcError> {
        let url = std::env::var(RPC_URL).map_err(|_| NodeRpcError::MissingVariable(RPC_URL))?;
        if url.is_empty() || url.trim() != url {
            return Err(NodeRpcError::InvalidVariable(RPC_URL));
        }
        let chain_id =
            std::env::var(CHAIN_ID).map_err(|_| NodeRpcError::MissingVariable(CHAIN_ID))?;
        let chain_id = if chain_id.starts_with("0x") {
            quantity(&chain_id)
        } else if !chain_id.is_empty() && chain_id.bytes().all(|byte| byte.is_ascii_digit()) {
            chain_id.parse().ok()
        } else {
            None
        }
        .ok_or(NodeRpcError::InvalidVariable(CHAIN_ID))?;
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

impl NodeRpc {
    pub fn connect(config: &NodeRpcConfig) -> Result<Self, NodeRpcError> {
        let config = config.clone();
        within_deadline(config.deadline, move |expires| {
            let uri: ureq::http::Uri = config.url.parse().map_err(|_| SequencerError::Endpoint)?;
            let certificates = if uri.scheme_str() == Some("https") {
                let loaded = rustls_native_certs::load_native_certs();
                if loaded.certs.is_empty() || !loaded.errors.is_empty() {
                    return Err(SequencerError::Tls.into());
                }
                loaded.certs
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

    pub const fn chain_id(&self) -> u64 {
        self.chain_id
    }

    pub fn pending_nonce(&self, address: &[u8; 20]) -> Result<u64, NodeRpcError> {
        let address = *address;
        self.request(move |endpoint| endpoint.pending_nonce(&address))
    }

    pub fn estimate_gas(&self, call: &CallRequest) -> Result<u64, NodeRpcError> {
        let call = call.clone();
        self.request(move |endpoint| endpoint.estimate_gas(&call))
    }

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

    pub fn send_raw_transaction(&self, raw: &[u8]) -> Result<[u8; 32], NodeRpcError> {
        let raw = raw.to_vec();
        self.request(move |endpoint| endpoint.send_raw_transaction(&raw))
    }

    pub fn sequencer_receipt(
        &self,
        tx_hash: &[u8; 32],
    ) -> Result<Option<SequencerReceipt>, NodeRpcError> {
        let tx_hash = *tx_hash;
        self.request(move |endpoint| endpoint.sequencer_receipt(&tx_hash))
    }

    pub fn receipt_keys(&self) -> Result<Vec<ReceiptKeyEntry>, NodeRpcError> {
        self.request(SequencerEndpoint::receipt_keys)
    }

    pub fn block_by_number(&self, height: u64) -> Result<Option<SequencerBlock>, NodeRpcError> {
        self.request(move |endpoint| endpoint.block_by_number(height))
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
    let mut response = agent
        .post(url)
        .header("Content-Type", "application/json")
        .header("Accept", "application/json")
        .send_json(json!({"jsonrpc": "2.0", "id": 1, "method": "eth_chainId", "params": []}))
        .map_err(transport)?;
    match response.status().as_u16() {
        200 => {}
        502..=504 => return Err(NodeRpcError::Unavailable),
        status => return Err(SequencerError::HttpStatus(status).into()),
    }
    let bytes = response
        .body_mut()
        .with_config()
        .limit(64 * 1024)
        .read_to_vec()
        .map_err(transport)?;
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
    response["result"]
        .as_str()
        .and_then(quantity)
        .ok_or_else(|| SequencerError::Malformed.into())
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

fn transport(error: ureq::Error) -> NodeRpcError {
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
