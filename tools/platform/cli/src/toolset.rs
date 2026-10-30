use std::collections::BTreeSet;

use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_mcp::catalogue;
use layerx_mcp::server::{DeploymentMode, ToolDefinition, ToolKind, FAUCET_REQUEST};
use serde_json::{json, Value};

use crate::config::Configuration;
use crate::credential;
use crate::encoding::{fixed_hex, hex_decode, hex_encode};
use crate::http::{validate_resource_id, Client};

const MAX_ARGUMENT_BYTES: usize = 256;
const SERVED: [ToolDefinition; 3] = [
    ToolDefinition {
        name: "receipt.get",
        kind: ToolKind::Read,
        required_scope: "receipt:read",
        mutation: "none",
        evidence: "canonical receipt verified by the hosted gateway authority boundary",
    },
    ToolDefinition {
        name: "activity.submit",
        kind: ToolKind::Write,
        required_scope: "activity:write",
        mutation: "signed EIP-1559 transaction through eth_sendRawTransaction",
        evidence:
            "transaction hash and submitted rung; receipt evidence requires a separate lookup",
    },
    FAUCET_REQUEST,
];

pub struct Runtime {
    client: Client,
    network_id: u32,
    environment: String,
    endpoint: String,
}

impl Runtime {
    #[cfg(test)]
    pub fn loopback(endpoint: &str, network_id: u32) -> Result<Runtime, String> {
        Ok(Self {
            client: Client::new(endpoint, None)?,
            network_id,
            environment: "test".to_owned(),
            endpoint: endpoint.to_owned(),
        })
    }

    pub fn new(
        configuration: &Configuration,
        gateway_credential: &str,
        key: &str,
        source: Option<&str>,
        asset: Option<&str>,
        mode: DeploymentMode,
    ) -> Result<Self, String> {
        let (environment_name, environment) = configuration.active_environment()?;
        let credential = credential::gateway(gateway_credential)?.ok_or_else(|| {
            format!(
                "gateway credential alias {gateway_credential} is absent; rerun layerx install for this runtime"
            )
        })?;
        if let Ok(expected) = std::env::var("LAYERX_GATEWAY_KEY_ID") {
            let actual = credential
                .split_once(':')
                .map(|(id, _)| id)
                .ok_or_else(|| "stored gateway credential is malformed".to_owned())?;
            if actual != expected {
                return Err(
                    "installed gateway credential does not match its non-secret key identity"
                        .into(),
                );
            }
        }
        let source = source
            .map(|value| fixed_hex::<32>("source account", value))
            .transpose()?;
        let asset = asset
            .map(|value| fixed_hex::<32>("asset", value))
            .transpose()?;
        if mode == DeploymentMode::Full && (source.is_none() || asset.is_none()) {
            return Err(
                "payment mode requires the installed source account and asset binding".into(),
            );
        }
        if mode == DeploymentMode::ReadOnly && (source.is_some() || asset.is_some()) {
            return Err("read-only mode cannot carry a payment source binding".into());
        }
        let metadata = configuration
            .keys
            .get(key)
            .ok_or_else(|| format!("key {key} does not exist"))?;
        fixed_hex::<32>("public key", &metadata.public_key)?;
        Ok(Self {
            client: Client::new_gateway(&environment.endpoint, credential)?,
            network_id: environment.network_id,
            environment: environment_name.to_owned(),
            endpoint: environment.endpoint.clone(),
        })
    }
}

pub fn surface(mode: DeploymentMode) -> Result<Vec<ToolDefinition>, String> {
    let mut tools = Vec::with_capacity(SERVED.len());
    for definition in SERVED {
        if mode == DeploymentMode::ReadOnly && definition.kind != ToolKind::Read {
            continue;
        }
        tools.push(definition);
    }
    if tools.is_empty() {
        return Err("the selected deployment mode would serve no tool".into());
    }
    Ok(tools)
}

pub fn scopes(tools: &[ToolDefinition]) -> Vec<String> {
    let mut unique = BTreeSet::new();
    for tool in tools {
        unique.insert(tool.required_scope.to_owned());
    }
    unique.into_iter().collect()
}

pub const fn mode_name(mode: DeploymentMode) -> &'static str {
    match mode {
        DeploymentMode::Full => "full",
        DeploymentMode::ReadOnly => "read-only",
    }
}

pub const fn kind_name(kind: ToolKind) -> &'static str {
    match kind {
        ToolKind::Read => "read",
        ToolKind::Write => "write",
    }
}

/// Returns the daemon-bound catalogue `layerx mcp serve` serves in one deployment mode.
pub fn daemon_surface(mode: DeploymentMode) -> Result<Vec<ToolDefinition>, String> {
    let tools = catalogue::surface(mode);
    if tools.is_empty() {
        return Err("the selected deployment mode would serve no daemon-bound tool".into());
    }
    Ok(tools)
}

/// Describes one daemon-bound catalogue tool for an installation record.
pub fn daemon_descriptor(tool: ToolDefinition) -> Result<Value, String> {
    let description = catalogue::description(tool.name)
        .ok_or_else(|| format!("tool {} carries no catalogue description", tool.name))?;
    let arguments = catalogue::input_schema(tool.name)
        .ok_or_else(|| format!("tool {} carries no catalogue argument schema", tool.name))?;
    Ok(json!({
        "name": tool.name,
        "kind": kind_name(tool.kind),
        "scope": tool.required_scope,
        "mutation": tool.mutation,
        "evidence": tool.evidence,
        "description": description,
        "arguments": arguments,
    }))
}

pub fn descriptor(tool: ToolDefinition) -> Value {
    json!({
        "name": tool.name,
        "kind": kind_name(tool.kind),
        "scope": tool.required_scope,
        "mutation": tool.mutation,
        "evidence": tool.evidence,
        "arguments": schema(tool.name),
    })
}

pub fn description(name: &str) -> &'static str {
    match name {
        "receipt.get" => "Fetch gateway-verified receipt material for one activity.",
        "activity.submit" => {
            "Submit a signed EIP-1559 transaction and return its transaction hash and submitted rung."
        }
        "faucet.request" => {
            "Claim one bounded beta faucet grant for the stored identity session, the named DID, and the named signer key."
        }
        _ => "This tool is not served by this deployment.",
    }
}

pub fn schema(name: &str) -> Value {
    match name {
        "receipt.get" => json!({
            "type": "object",
            "properties": {"activity_id": {"type": "string", "pattern": "^[0-9a-fA-F]{64}$"}},
            "required": ["activity_id"],
            "additionalProperties": false,
        }),
        "activity.submit" => json!({
            "type": "object",
            "properties": {
                "raw_transaction": {
                    "type": "string", "pattern": "^0x02([0-9a-f]{2})+$",
                    "maxLength": 1_048_578
                }
            },
            "required": ["raw_transaction"],
            "additionalProperties": false,
        }),
        "faucet.request" => json!({
            "type": "object",
            "properties": {
                "did": {"type": "string", "pattern": "^did:[0-9A-Za-z._:-]+$", "maxLength": 256},
                "public_key": {"type": "string", "pattern": "^[0-9a-fA-F]{64}$"}
            },
            "required": ["did", "public_key"],
            "additionalProperties": false,
        }),
        _ => json!({"type": "object", "additionalProperties": false}),
    }
}

pub fn invoke(runtime: &Runtime, tool: ToolDefinition, arguments: &Value) -> Result<Value, String> {
    match tool.name {
        "receipt.get" => {
            let activity = text(arguments, "activity_id")?;
            validate_hex32(&activity, "activity id")?;
            runtime
                .client
                .get(&format!("/v1/receipts/{}", activity.to_ascii_lowercase()))
        }
        "activity.submit" => submit(runtime, arguments),
        "faucet.request" => claim_faucet(runtime, arguments),
        _ => Err(format!(
            "tool {} is not served by this deployment",
            tool.name
        )),
    }
}

pub fn invoke_task(
    runtime: &Runtime,
    tool: ToolDefinition,
    arguments: &Value,
) -> Result<Value, String> {
    if tool.name != "activity.submit" {
        return invoke(runtime, tool, arguments);
    }
    match submit_classified(runtime, arguments) {
        Ok(value) => Ok(value),
        Err(SubmitFailure::Indeterminate { tx_hash, detail }) => {
            Ok(json!({"txHash": tx_hash, "rung": "unknown", "detail": detail}))
        }
        Err(SubmitFailure::Unsent(detail) | SubmitFailure::Refused(detail)) => Err(detail),
    }
}

fn claim_faucet(runtime: &Runtime, arguments: &Value) -> Result<Value, String> {
    let did = text(arguments, "did")?;
    if did
        .strip_prefix("did:")
        .and_then(|rest| rest.split_once(':'))
        .is_none_or(|(method, identifier)| method.is_empty() || identifier.is_empty())
    {
        return Err("argument did must be a LayerX decentralised identifier".into());
    }
    validate_resource_id(&did, "did")?;
    let public_key = text(arguments, "public_key")?;
    validate_hex32(&public_key, "public key")?;
    let public_key = public_key.to_ascii_lowercase();
    let token = credential::token(&runtime.environment)?.ok_or_else(|| {
        format!(
            "no identity session is stored for {}; run layerx auth set --environment {} before claiming faucet funds",
            runtime.environment, runtime.environment
        )
    })?;
    let client = Client::new(&runtime.endpoint, Some(token))?;
    let response = client.post("/rpc", &crate::faucet::request(&did, &public_key), None)?;
    let claim = crate::faucet::decode(&response)?;
    Ok(json!({
        "did": did,
        "public_key": public_key,
        "claim": claim,
    }))
}

pub enum SubmitFailure {
    Unsent(String),
    Refused(String),
    Indeterminate { tx_hash: String, detail: String },
}

fn submit(runtime: &Runtime, arguments: &Value) -> Result<Value, String> {
    submit_classified(runtime, arguments).map_err(|failure| match failure {
        SubmitFailure::Unsent(detail)
        | SubmitFailure::Refused(detail)
        | SubmitFailure::Indeterminate { detail, .. } => detail,
    })
}

fn submit_classified(runtime: &Runtime, arguments: &Value) -> Result<Value, SubmitFailure> {
    let object = arguments
        .as_object()
        .filter(|object| object.len() == 1)
        .ok_or_else(|| {
            SubmitFailure::Unsent("activity.submit requires only raw_transaction".into())
        })?;
    let raw = object
        .get("raw_transaction")
        .and_then(Value::as_str)
        .ok_or_else(|| {
            SubmitFailure::Unsent(
                "activity.submit requires a signed raw_transaction hex string".into(),
            )
        })?;
    let request = layerx_platform_cli::rpc::request("eth_sendRawTransaction", &json!([raw]))
        .map_err(SubmitFailure::Unsent)?;
    let bytes = hex_decode(
        "raw transaction",
        raw.strip_prefix("0x")
            .ok_or_else(|| SubmitFailure::Unsent("raw transaction requires 0x".into()))?,
    )
    .map_err(SubmitFailure::Unsent)?;
    let (call, signature) = decode_signed(&bytes).map_err(|error| {
        SubmitFailure::Unsent(format!("invalid signed EIP-1559 transaction: {error}"))
    })?;
    recover_sender(&call, &signature)
        .map_err(|error| SubmitFailure::Unsent(format!("invalid EIP-1559 signature: {error}")))?;
    if call.chain_id != u64::from(runtime.network_id) {
        return Err(SubmitFailure::Unsent(
            "raw transaction chain id does not match the active environment".into(),
        ));
    }
    let tx_hash = format!("0x{}", hex_encode(&transaction_hash(&bytes)));
    let response = runtime
        .client
        .post("/rpc", &request, None)
        .map_err(|detail| SubmitFailure::Indeterminate {
            tx_hash: tx_hash.clone(),
            detail,
        })?;
    submission_result(&bytes, &response).map_err(|detail| {
        if response.get("jsonrpc") == Some(&json!("2.0"))
            && response.get("id") == Some(&json!(1))
            && response.get("result").is_none()
            && response.get("error").is_some_and(|error| {
                error.get("code").and_then(Value::as_i64).is_some()
                    && error.get("message").and_then(Value::as_str).is_some()
            })
        {
            SubmitFailure::Refused(detail)
        } else {
            SubmitFailure::Indeterminate { tx_hash, detail }
        }
    })
}

fn submission_result(raw: &[u8], response: &Value) -> Result<Value, String> {
    let hash = layerx_platform_cli::rpc::decode_response("eth_sendRawTransaction", response)?;
    let expected = format!("0x{}", hex_encode(&transaction_hash(raw)));
    if hash.as_str() != Some(expected.as_str()) {
        return Err(
            "eth_sendRawTransaction returned a hash that does not match the raw transaction".into(),
        );
    }
    Ok(json!({"txHash": hash, "rung": "submitted"}))
}

fn text(arguments: &Value, field: &str) -> Result<String, String> {
    let value = arguments
        .get(field)
        .and_then(Value::as_str)
        .ok_or_else(|| format!("argument {field} must be a string"))?;
    if value.is_empty() || value.len() > MAX_ARGUMENT_BYTES {
        return Err(format!(
            "argument {field} must be 1-{MAX_ARGUMENT_BYTES} bytes"
        ));
    }
    Ok(value.to_owned())
}

fn validate_hex32(value: &str, name: &str) -> Result<(), String> {
    validate_resource_id(value, name)?;
    fixed_hex::<32>(name, value).map(|_| ())
}
