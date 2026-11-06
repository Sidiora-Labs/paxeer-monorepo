//! Parity evidence comes from authenticated operation envelopes emitted by a real daemon.
use std::collections::BTreeMap;
use std::fs;
use std::io::{Read, Write};
use std::path::Path;

use layerx_agent_api::error::RequestId;
use layerx_agent_api::idempotency::Key;
use layerx_agentd::agent_rpc;
use layerx_agentd::human_runtime::{HumanAuthorityBoundary, SharedAgentOwner};
use layerx_sdk::agent_envelope::{AgentEnvelopeTransport, EnvelopeCredential, EnvelopeError};
use layerx_sdk::Operation;
use serde_json::{json, Value};
use ureq::tls::{Certificate, ClientCert, PrivateKey};

pub struct DaemonParity {
    transport: AgentEnvelopeTransport,
    credential: EnvelopeCredential,
}

pub fn digest(value: &Value) -> Result<[u8; 32], String> {
    let text = value
        .as_str()
        .ok_or("expected a 64-digit lowercase hex identifier")?;
    if text.len() != 64
        || !text
            .bytes()
            .all(|b| b.is_ascii_digit() || (b'a'..=b'f').contains(&b))
    {
        return Err("expected a 64-digit lowercase hex identifier".to_owned());
    }
    let mut bytes = [0; 32];
    for (slot, pair) in bytes.iter_mut().zip(text.as_bytes().chunks_exact(2)) {
        *slot = u8::from_str_radix(
            std::str::from_utf8(pair).map_err(|_| "identifier encoding")?,
            16,
        )
        .map_err(|_| "identifier encoding")?;
    }
    Ok(bytes)
}

impl DaemonParity {
    pub fn connect(
        endpoint: &str,
        certificate: &[u8],
        private_key: &[u8],
        trust_anchors: &Path,
        credential: EnvelopeCredential,
    ) -> Result<Self, String> {
        let certificate =
            Certificate::from_pem(certificate).map_err(|_| "invalid parity client certificate")?;
        let key = PrivateKey::from_pem(private_key).map_err(|_| "invalid parity client key")?;
        let identity = ClientCert::new_with_certs(&[certificate], key);
        let transport = AgentEnvelopeTransport::connect_daemon(endpoint, identity, trust_anchors)
            .map_err(|error| format!("daemon envelope transport: {error:?}"))?;
        Ok(Self {
            transport,
            credential,
        })
    }

    pub fn from_document(document: &Value, plan_path: &Path) -> Result<Self, String> {
        let text = |name: &str| {
            document[name]
                .as_str()
                .ok_or_else(|| format!("daemon plan omitted {name}"))
        };
        let root = plan_path
            .parent()
            .ok_or("provisioning plan has no directory")?;
        let cert = fs::read(root.join(text("client_certificate")?))
            .map_err(|_| "read daemon client certificate")?;
        let key = fs::read(root.join(text("client_key")?)).map_err(|_| "read daemon client key")?;
        let generation = text("generation")?
            .parse()
            .map_err(|_| "invalid credential generation")?;
        let credential = EnvelopeCredential::new(
            text("tenant")?,
            digest(&document["session_id"])?,
            digest(&document["token_id"])?,
            generation,
        )
        .map_err(|_| "invalid daemon session credential")?;
        Self::connect(
            text("endpoint")?,
            &cert,
            &key,
            &root.join(text("trust_anchors")?),
            credential,
        )
    }

    /// Retains only actual, schema-checked daemon responses. Transport or decode failures
    /// cannot become comparison evidence, even if an edge returned parseable JSON.
    pub fn observe(
        &self,
        operation: Operation,
        id: RequestId,
        request: &Value,
        key: Option<Key>,
    ) -> Result<Value, String> {
        let (received, result) = self.transport.send_operation_recorded(
            operation,
            id,
            request,
            Some(&self.credential),
            key,
        );
        match result {
            Ok(_) | Err(EnvelopeError::Refused(_)) | Err(EnvelopeError::NotReady { .. }) => {}
            Err(error) => return Err(format!("{} envelope failed: {error:?}", operation.name())),
        }
        let (status, document) = received.ok_or("daemon returned no envelope evidence")?;
        Ok(json!({"status": status, "document": document}))
    }
}

pub fn compare_evidence(
    language: &str,
    reference: &BTreeMap<String, Value>,
    observed: &BTreeMap<String, Value>,
) -> Result<(), String> {
    if reference.keys().ne(observed.keys()) {
        return Err(format!("{language} envelope scenario coverage diverged"));
    }
    for (scenario, actual) in reference {
        if observed.get(scenario) != Some(actual) {
            // Do not print raw owner data or credentials in a public report.
            return Err(format!(
                "scenario={scenario} language={language} daemon envelope diverged"
            ));
        }
    }
    Ok(())
}

/// HTTP framing shared by an in-process provisioned daemon and its loopback proof.
/// This function passes the exact request bytes to the production owner, including
/// authentication, admission, durable idempotency and response rendering.
pub fn serve_daemon_connection<A: HumanAuthorityBoundary>(
    owner: &SharedAgentOwner<A>,
    stream: &mut (impl Read + Write),
) -> Result<(), String> {
    let body = read_rpc_body(stream)?;
    let response = agent_rpc::handle_rpc(owner, &body);
    write_rpc_response(stream, response.status, &response.body)
}

pub fn read_rpc_body(stream: &mut impl Read) -> Result<Vec<u8>, String> {
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        if header.len() >= 16_384 {
            return Err("RPC header exceeds bound".to_owned());
        }
        let mut byte = [0];
        stream
            .read_exact(&mut byte)
            .map_err(|_| "RPC header read failed")?;
        header.push(byte[0]);
    }
    let text = std::str::from_utf8(&header).map_err(|_| "RPC header encoding")?;
    let mut lines = text.split("\r\n");
    if lines.next() != Some("POST /rpc HTTP/1.1") {
        return Err("expected current daemon POST /rpc route".to_owned());
    }
    let mut length = None;
    for line in lines.filter(|line| !line.is_empty()) {
        let (name, value) = line.split_once(':').ok_or("invalid RPC header")?;
        if name.eq_ignore_ascii_case("authorization")
            || name.eq_ignore_ascii_case("transfer-encoding")
        {
            return Err("unsupported RPC principal or framing header".to_owned());
        }
        if name.eq_ignore_ascii_case("content-length") {
            if length.is_some() {
                return Err("duplicate RPC length".to_owned());
            }
            length = Some(
                value
                    .trim()
                    .parse::<usize>()
                    .map_err(|_| "invalid RPC length")?,
            );
        }
    }
    let length = length.ok_or("missing RPC length")?;
    if length > 1_048_576 {
        return Err("RPC body exceeds bound".to_owned());
    }
    let mut body = vec![0; length];
    stream
        .read_exact(&mut body)
        .map_err(|_| "RPC body read failed")?;
    Ok(body)
}

pub fn write_rpc_response(stream: &mut impl Write, status: u16, body: &[u8]) -> Result<(), String> {
    write!(stream, "HTTP/1.1 {status} Response\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len())
        .and_then(|()| stream.write_all(body)).and_then(|()| stream.flush()).map_err(|_| "RPC response write failed".to_owned())
}
