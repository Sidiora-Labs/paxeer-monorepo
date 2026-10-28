use crate::node_rpc::{NodeRpc, NodeRpcConfig, NodeRpcError};
use layerx_agent_api::error::{ApiError, ErrorClass, ReasonCode, RequestId, Retriability};
use layerx_types::amount::Amount;
use serde_json::{json, Value};

pub type Rung = layerx_agent_api::verify::Rung;

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Block {
    Pending,
    Height(u64),
}

impl Block {
    pub fn tag(self) -> String {
        match self {
            Self::Pending => "pending".to_owned(),
            Self::Height(height) => format!("{height:#x}"),
        }
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Freshness {
    pub chain_head: u64,
    pub value_height: u64,
    pub relative_to: Block,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct BalanceRead {
    pub account: [u8; 20],
    pub amount: Amount,
    pub canonical_bytes: Vec<u8>,
    pub achieved_rung: Option<Rung>,
    pub freshness: Freshness,
}

pub fn balance(
    config: &NodeRpcConfig,
    account: [u8; 20],
    height: Option<u64>,
) -> Result<BalanceRead, ApiError> {
    let _node = NodeRpc::connect(config).map_err(node_error)?;
    let certificates = if config.url.starts_with("https://") {
        let loaded = rustls_native_certs::load_native_certs();
        if loaded.certs.is_empty() || !loaded.errors.is_empty() {
            return Err(refusal(ErrorClass::TransportFailure, "read.node_tls"));
        }
        loaded.certs
    } else {
        Vec::new()
    };
    let roots: Vec<_> = certificates
        .iter()
        .map(|certificate| ureq::tls::Certificate::from_der(certificate.as_ref()).to_owned())
        .collect();
    let agent: ureq::Agent = ureq::Agent::config_builder()
        .timeout_global(Some(config.deadline))
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
    let head = rpc(&agent, config, "eth_blockNumber", json!([]), false)?;
    let chain_head = quantity(&head)
        .and_then(|digits| u64::from_str_radix(digits, 16).ok())
        .ok_or_else(malformed)?;
    let (relative_to, value_height) = match height {
        Some(height) if height > chain_head => {
            return Err(refusal(ErrorClass::CoreRejection, "read.height_ahead"));
        }
        Some(height) => (Block::Height(height), height),
        None => (
            Block::Pending,
            chain_head.checked_add(1).ok_or_else(malformed)?,
        ),
    };
    let address: String = account.iter().map(|byte| format!("{byte:02x}")).collect();
    let value = rpc(
        &agent,
        config,
        "eth_getBalance",
        json!([format!("0x{address}"), relative_to.tag()]),
        height.is_some(),
    )?;
    let digits = quantity(&value).ok_or_else(malformed)?;
    let amount = u128::from_str_radix(digits, 16)
        .map(Amount::from_u128)
        .map_err(|_| refusal(ErrorClass::CoreRejection, "read.amount_overflow"))?;
    Ok(BalanceRead {
        account,
        amount,
        canonical_bytes: amount.to_be_bytes().to_vec(),
        achieved_rung: None,
        freshness: Freshness {
            chain_head,
            value_height,
            relative_to,
        },
    })
}

fn quantity(value: &Value) -> Option<&str> {
    let digits = value.as_str()?.strip_prefix("0x")?;
    (!digits.is_empty()
        && (digits.len() == 1 || !digits.starts_with('0'))
        && digits.bytes().all(|byte| byte.is_ascii_hexdigit()))
    .then_some(digits)
}

fn rpc(
    agent: &ureq::Agent,
    config: &NodeRpcConfig,
    method: &str,
    params: Value,
    historical: bool,
) -> Result<Value, ApiError> {
    let mut response = agent
        .post(&config.url)
        .header("Content-Type", "application/json")
        .header("Accept", "application/json")
        .send_json(json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}))
        .map_err(transport)?;
    if response.status().as_u16() != 200 {
        return Err(refusal(ErrorClass::TransportFailure, "read.node_http"));
    }
    let body = response
        .body_mut()
        .with_config()
        .limit(64 * 1024)
        .read_to_vec()
        .map_err(transport)?;
    let document: Value = serde_json::from_slice(&body).map_err(|_| malformed())?;
    let object = document.as_object().ok_or_else(malformed)?;
    if object.len() != 3 || document["jsonrpc"] != "2.0" || document["id"].as_u64() != Some(1) {
        return Err(malformed());
    }
    if let Some(result) = object.get("result") {
        return Ok(result.clone());
    }
    let error = document["error"].as_object().ok_or_else(malformed)?;
    if error
        .keys()
        .any(|key| !matches!(key.as_str(), "code" | "message" | "data"))
        || error.get("code").and_then(Value::as_i64).is_none()
    {
        return Err(malformed());
    }
    let message = error
        .get("message")
        .and_then(Value::as_str)
        .ok_or_else(malformed)?;
    let message = message.to_ascii_lowercase();
    if historical
        && (message.contains("pruned")
            || message.starts_with("missing trie node")
            || message.starts_with("historical state")
            || message == "header not found"
            || message == "required data unavailable"
            || message == "resource not found"
            || message == "unknown block"
            || message.starts_with("blockoutofrangeerror:")
            || message.starts_with("height is not available"))
    {
        return Err(refusal(ErrorClass::CoreRejection, "read.height_pruned"));
    }
    Err(refusal(ErrorClass::CoreRejection, "read.node_refusal"))
}

fn node_error(error: NodeRpcError) -> ApiError {
    match error {
        NodeRpcError::ChainIdMismatch { .. } => refusal(
            ErrorClass::ProtocolIncompatibility,
            "read.chain_id_mismatch",
        ),
        NodeRpcError::Unavailable => refusal(ErrorClass::TransportFailure, "read.node_unavailable"),
        _ => refusal(ErrorClass::TransportFailure, "read.node_connection"),
    }
}

fn transport(error: ureq::Error) -> ApiError {
    match error {
        ureq::Error::BodyExceedsLimit(_) | ureq::Error::LargeResponseHeader(..) => malformed(),
        _ => refusal(ErrorClass::TransportFailure, "read.node_unavailable"),
    }
}

fn malformed() -> ApiError {
    refusal(ErrorClass::ProtocolIncompatibility, "read.node_malformed")
}

fn refusal(class: ErrorClass, reason: &str) -> ApiError {
    ApiError {
        class,
        protocol_result_code: None,
        retriability: Retriability::Terminal,
        request_id: RequestId(0),
        reason: ReasonCode::new(reason).unwrap_or_else(|_| unreachable!("static reason")),
    }
}
