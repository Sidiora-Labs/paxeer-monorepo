use std::io::{Read, Write};
use std::net::TcpListener;
use std::sync::{Arc, Mutex};

use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

use crate::boot::InferenceBootGate;
use crate::config::InferenceConfig;
use crate::degraded::Reference;

pub struct InferenceMarket {
    pub signal_listener: TcpListener,
    pub completion_listener: TcpListener,
    pub gate: Arc<Mutex<InferenceBootGate>>,
    pub config: InferenceConfig,
    pub room_profile: Vec<u8>,
}

/// # Errors
/// Returns an error if availability cannot be fetched or validated, or the inference server
/// cannot start or serve its listener.
pub fn serve_inference_market(
    config: InferenceConfig,
    gate: InferenceBootGate,
) -> Result<InferenceMarket, String> {
    if gate.status().mode != crate::degraded::Mode::Healthy {
        return Err("inference boot dependencies are not ready".to_owned());
    }
    let room_profile = config.load_room_profile()?;
    let signal_listener = TcpListener::bind(config.signal_listen)
        .map_err(|error| format!("inference signalling listener failed: {error}"))?;
    let completion_listener = TcpListener::bind(config.serve_listen)
        .map_err(|error| format!("inference completion listener failed: {error}"))?;
    Ok(InferenceMarket {
        signal_listener,
        completion_listener,
        gate: Arc::new(Mutex::new(gate)),
        config,
        room_profile,
    })
}

#[must_use]
pub fn market_id(chain_id: u64, program: &[u8; 32]) -> [u8; 32] {
    let mut chain = Sha256::new();
    chain.update(b"PAXAI/chain-domain/v1");
    chain.update(chain_id.to_be_bytes());
    let mut market = Sha256::new();
    market.update(b"PAXAI/market/v1\0");
    market.update(chain.finalize());
    market.update(program);
    market.finalize().into()
}

/// # Errors
/// Returns an error if availability cannot be fetched or validated, or the inference server
/// cannot start or serve its listener.
pub fn read_availability(
    agent: &ureq::Agent,
    endpoint: &str,
    bearer: &str,
    chain_id: u64,
    program: &[u8; 32],
    reference: Reference,
) -> Result<Value, String> {
    let document = availability_document(agent, endpoint, bearer, chain_id, program)?;
    validate_availability(&document, chain_id, program, reference)
}

/// Read one gateway publication and independently verify its claimed node height.
///
/// # Errors
/// Returns an error if acquisition, node verification, exact binding checks or
/// component availability checks fail.
pub fn read_verified_availability(
    agent: &ureq::Agent,
    endpoint: &str,
    bearer: &str,
    chain_id: u64,
    program: &[u8; 32],
    verify_height: impl FnOnce(u64) -> Result<Reference, String>,
) -> Result<(Reference, Value), String> {
    let document = availability_document(agent, endpoint, bearer, chain_id, program)?;
    let height = document["result"]["binding"]["block_height"]
        .as_str()
        .filter(|text| {
            (text.len() == 1 || !text.starts_with('0'))
                && text.bytes().all(|byte| byte.is_ascii_digit())
        })
        .and_then(|text| text.parse::<u64>().ok())
        .ok_or("inference availability is not bound to the current committed program")?;
    let reference = verify_height(height)?;
    let components = validate_availability(&document, chain_id, program, reference)?;
    Ok((reference, components))
}

fn availability_document(
    agent: &ureq::Agent,
    endpoint: &str,
    bearer: &str,
    chain_id: u64,
    program: &[u8; 32],
) -> Result<Value, String> {
    let market = layerx_programs::hex::encode(&market_id(chain_id, program));
    let url = format!(
        "{}/v1/ai/markets/0x{market}/snapshot",
        endpoint.trim_end_matches('/')
    );
    let mut response = agent
        .get(&url)
        .header("Authorization", &format!("Bearer {bearer}"))
        .call()
        .map_err(|_| "inference availability read failed".to_owned())?;
    if response.status().as_u16() != 200 {
        return Err("inference availability read refused".to_owned());
    }
    let bytes = response
        .body_mut()
        .with_config()
        .limit(65_536)
        .read_to_vec()
        .map_err(|_| "inference availability response exceeded its bound".to_owned())?;
    let document: Value = serde_json::from_slice(&bytes)
        .map_err(|_| "inference availability response is malformed".to_owned())?;
    Ok(document)
}

fn validate_availability(
    document: &Value,
    chain_id: u64,
    program: &[u8; 32],
    reference: Reference,
) -> Result<Value, String> {
    let market = layerx_programs::hex::encode(&market_id(chain_id, program));
    let snapshot = &document["result"];
    let binding = &snapshot["binding"];
    let mut chain = Sha256::new();
    chain.update(b"PAXAI/chain-domain/v1");
    chain.update(chain_id.to_be_bytes());
    if document["ok"] != true
        || snapshot["committed"] != true
        || snapshot["projection"] != "finalized-publishable"
        || binding["chain"] != format!("0x{}", layerx_programs::hex::encode(&chain.finalize()))
        || binding["program"] != format!("0x{}", layerx_programs::hex::encode(program))
        || binding["market"] != format!("0x{market}")
        || binding["block_height"]
            .as_str()
            .filter(|text| {
                (text.len() == 1 || !text.starts_with('0'))
                    && text.bytes().all(|byte| byte.is_ascii_digit())
            })
            .and_then(|text| text.parse::<u64>().ok())
            != Some(reference.head_sequence)
        || binding["block_hash"]
            != format!("0x{}", layerx_programs::hex::encode(&reference.checkpoint))
        || binding["rank"] != 4
    {
        return Err(
            "inference availability is not bound to the current committed program".to_owned(),
        );
    }
    let components = snapshot["components"].clone();
    InferenceBootGate::require_f11(&components)
        .map_err(|error| format!("inference availability refused: {error:?}"))?;
    Ok(components)
}

/// # Errors
/// Returns an error if availability cannot be fetched or validated, or the inference server
/// cannot start or serve its listener.
pub fn observe_availability(
    gate: &Arc<Mutex<InferenceBootGate>>,
    observation: Result<(Reference, Value), String>,
) -> Result<(), String> {
    let mut gate = gate.lock().map_err(|_| "inference gate is unavailable")?;
    match observation {
        Ok((reference, components)) => gate
            .ready(reference, Rung::Committed, &components)
            .map_err(|error| format!("inference readiness refused: {error:?}")),
        Err(error) => {
            gate.disconnected();
            Err(error)
        }
    }
}

/// # Errors
/// Returns an error if availability cannot be fetched or validated, or the inference server
/// cannot start or serve its listener.
pub fn serve_health(
    stream: &mut (impl Read + Write),
    gate: &Arc<Mutex<InferenceBootGate>>,
) -> Result<(), String> {
    let mut headers = Vec::new();
    while !headers.ends_with(b"\r\n\r\n") {
        if headers.len() == 16_384 {
            return Err("inference request headers exceeded their bound".to_owned());
        }
        let mut byte = [0];
        stream
            .read_exact(&mut byte)
            .map_err(|error| error.to_string())?;
        headers.push(byte[0]);
    }
    let request = std::str::from_utf8(&headers)
        .map_err(|_| "inference request headers are invalid".to_owned())?;
    let ready = gate
        .lock()
        .map_err(|_| "inference gate is unavailable")?
        .status()
        .mode
        == crate::degraded::Mode::Healthy;
    let (status, body) = if request.starts_with("GET /healthz HTTP/1.1\r\n") {
        (if ready { 200 } else { 503 }, json!({"ready": ready}))
    } else {
        (404, json!({"error": "not_found"}))
    };
    let body = body.to_string();
    write!(stream, "HTTP/1.1 {status} Response\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\nCache-Control: no-store\r\n\r\n{body}", body.len())
        .map_err(|error| error.to_string())
}
