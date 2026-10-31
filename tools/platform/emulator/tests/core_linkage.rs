use std::fmt::Write as _;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::thread;
use std::time::{Duration, Instant};

const EMULATOR_SEED: [u8; 32] = [0x42; 32];

struct Reply {
    status: u16,
    content_type: String,
    body: Vec<u8>,
}

impl Reply {
    fn text(&self) -> String {
        String::from_utf8_lossy(&self.body).into_owned()
    }
}

fn free_port() -> Result<u16, String> {
    let probe = TcpListener::bind("127.0.0.1:0").map_err(|error| error.to_string())?;
    let port = probe
        .local_addr()
        .map_err(|error| error.to_string())?
        .port();
    drop(probe);
    Ok(port)
}

fn request(
    address: &str,
    method: &str,
    path: &str,
    content_type: &str,
    body: &[u8],
) -> Result<Reply, String> {
    request_with_idempotency(address, method, path, content_type, body, None)
}

fn request_with_idempotency(
    address: &str,
    method: &str,
    path: &str,
    content_type: &str,
    body: &[u8],
    idempotency_key: Option<&str>,
) -> Result<Reply, String> {
    let mut stream = TcpStream::connect(address).map_err(|error| error.to_string())?;
    stream
        .set_read_timeout(Some(Duration::from_secs(10)))
        .map_err(|error| error.to_string())?;
    let mut head = format!(
        "{method} {path} HTTP/1.1\r\nHost: {address}\r\nConnection: close\r\nContent-Length: {}\r\n",
        body.len()
    );
    if !content_type.is_empty() {
        write!(head, "Content-Type: {content_type}\r\n").map_err(|error| error.to_string())?;
    }
    if let Some(value) = idempotency_key {
        write!(head, "Idempotency-Key: {value}\r\n").map_err(|error| error.to_string())?;
    }
    head.push_str("\r\n");
    stream
        .write_all(head.as_bytes())
        .map_err(|error| error.to_string())?;
    stream.write_all(body).map_err(|error| error.to_string())?;
    stream.flush().map_err(|error| error.to_string())?;
    let mut raw = Vec::new();
    stream
        .read_to_end(&mut raw)
        .map_err(|error| error.to_string())?;
    parse_reply(&raw)
}

fn parse_reply(raw: &[u8]) -> Result<Reply, String> {
    let split = raw
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .ok_or_else(|| "response is missing a header terminator".to_string())?;
    let header_text =
        std::str::from_utf8(&raw[..split]).map_err(|_| "response headers are not UTF-8")?;
    let mut lines = header_text.split("\r\n");
    let status_line = lines.next().ok_or("response is missing a status line")?;
    let status = status_line
        .split_whitespace()
        .nth(1)
        .ok_or("status line is missing a code")?
        .parse::<u16>()
        .map_err(|_| "status code is not numeric".to_string())?;
    let mut content_type = String::new();
    for line in lines {
        if let Some((name, value)) = line.split_once(':') {
            if name.eq_ignore_ascii_case("content-type") {
                content_type = value.trim().to_string();
            }
        }
    }
    Ok(Reply {
        status,
        content_type,
        body: raw[split + 4..].to_vec(),
    })
}

/// Boots the emulator on a private port and returns its loopback address once
/// the real core reports readiness on `/healthz`.
fn boot_protocol(protocol_version: u16) -> Result<String, String> {
    let port = free_port()?;
    let address = format!("127.0.0.1:{port}");
    let listen = address.clone();
    let seed_path = PathBuf::from(format!(
        "/tmp/layerx-emulator-gateway-seed-{}-{port}",
        std::process::id()
    ));
    std::fs::write(&seed_path, EMULATOR_SEED).map_err(|error| error.to_string())?;
    let seed_argument = seed_path.to_string_lossy().into_owned();
    thread::Builder::new()
        .stack_size(8 * 1024 * 1024)
        .spawn(move || {
            let _ = layerx_platform_emulator::run(vec![
                "up".to_string(),
                "--listen".to_string(),
                listen,
                "--sequencer-seed-file".to_string(),
                seed_argument,
                "--protocol-version".to_string(),
                protocol_version.to_string(),
            ]);
        })
        .map_err(|error| error.to_string())?;
    let deadline = Instant::now() + Duration::from_secs(15);
    loop {
        if let Ok(reply) = request(&address, "GET", "/healthz", "", &[]) {
            if reply.status == 200 {
                let _ = std::fs::remove_file(&seed_path);
                return Ok(address);
            }
        }
        if Instant::now() >= deadline {
            return Err("emulator did not become ready".to_string());
        }
        thread::sleep(Duration::from_millis(20));
    }
}

fn response_result(reply: &Reply) -> Result<serde_json::Value, String> {
    serde_json::from_slice::<serde_json::Value>(&reply.body)
        .map_err(|error| error.to_string())?
        .get("result")
        .cloned()
        .ok_or_else(|| format!("response omitted result: {}", reply.text()))
}

fn post_json(address: &str, path: &str, body: &str) -> Result<Reply, String> {
    request(address, "POST", path, "application/json", body.as_bytes())
}

fn hex_encode(bytes: &[u8]) -> Result<String, String> {
    let mut encoded = String::new();
    for byte in bytes {
        write!(encoded, "{byte:02x}").map_err(|error| error.to_string())?;
    }
    Ok(encoded)
}

fn hex_decode(value: &str) -> Result<Vec<u8>, String> {
    if !value.len().is_multiple_of(2) {
        return Err("hex value has odd length".to_string());
    }
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let text = std::str::from_utf8(pair).map_err(|_| "hex value is not UTF-8")?;
            u8::from_str_radix(text, 16).map_err(|_| "hex value has a non-hex byte".to_string())
        })
        .collect()
}

#[test]
fn emulator_core_builds_without_batch_and_history_symbols() -> Result<(), String> {
    let archive = PathBuf::from(env!("OUT_DIR")).join("liblayerx_emulator_core.a");
    assert!(
        archive.is_file(),
        "core archive missing: {}",
        archive.display()
    );
    let output = std::process::Command::new("nm")
        .arg("-g")
        .arg(&archive)
        .output()
        .map_err(|error| error.to_string())?;
    assert!(
        output.status.success(),
        "nm failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let symbols = String::from_utf8(output.stdout).map_err(|error| error.to_string())?;
    assert!(
        symbols
            .lines()
            .any(|line| line.split_whitespace().last() == Some("platform_emulator_execute")),
        "archive did not contain the emulator core"
    );
    for retired in [
        "lxp_batch_seal",
        "lxp_batch_sign",
        "lxp_history_open",
        "lxp_history_close",
    ] {
        assert!(
            !symbols
                .lines()
                .any(|line| line.split_whitespace().last() == Some(retired)),
            "core archive defines or references {retired}: {symbols}"
        );
    }
    Ok(())
}

#[test]
fn emulator_head_proof_still_verifies() -> Result<(), String> {
    use sha2::{Digest as _, Sha256};
    let address = boot_protocol(3)?;
    let public = ed25519_dalek::SigningKey::from_bytes(&EMULATOR_SEED)
        .verifying_key()
        .to_bytes();
    let public_hex = hex_encode(&public)?;
    for (did, amount) in [("source", 1000), ("destination", 0)] {
        let body = format!(
            "{{\"did\":\"did:layerx:head-{did}\",\"public_key\":\"{public_hex}\",\"amount_lo\":{amount}}}"
        );
        let reply = post_json(&address, "/__emulator/accounts/prefund", &body)?;
        assert_eq!(reply.status, 200, "prefund: {}", reply.text());
    }
    let quoted = post_json(
        &address,
        "/v1/moves/quote",
        r#"{"source":"agent:did:layerx:head-source:main","destination":"agent:did:layerx:head-destination:main","money":{"currency":"LXP","amount":"250"}}"#,
    )?;
    assert_eq!(quoted.status, 200, "quote: {}", quoted.text());
    let quote = response_result(&quoted)?;
    let quote_id = quote["quote_id"].as_str().ok_or("quote id missing")?;
    let committed = request_with_idempotency(
        &address,
        "POST",
        "/v1/moves",
        "application/json",
        format!("{{\"quote_id\":\"{quote_id}\"}}").as_bytes(),
        Some("head-proof-transfer-0001"),
    )?;
    assert_eq!(committed.status, 200, "transfer: {}", committed.text());
    assert_eq!(response_result(&committed)?["state"], "done");
    let head_reply = request(&address, "GET", "/v1/protocol/account-state/head", "", &[])?;
    assert_eq!(head_reply.status, 200, "head: {}", head_reply.text());
    let head = response_result(&head_reply)?;
    assert_eq!(head["current"], true);
    let bytes = |pointer: &str| -> Result<Vec<u8>, String> {
        hex_decode(
            head.pointer(pointer)
                .and_then(serde_json::Value::as_str)
                .ok_or_else(|| format!("head omitted {pointer}"))?,
        )
    };
    let receipt = bytes("/receipt_hex")?;
    let header_bytes = bytes("/batch_evidence/header_hex")?;
    let signature: [u8; 64] = bytes("/batch_evidence/header_signature")?
        .try_into()
        .map_err(|_| "signature length")?;
    let encoded_proof = bytes("/batch_evidence/receipt_proof_hex")?;
    let path = layerx_wire::receipt::decode_merkle_proof(&encoded_proof)
        .map_err(|error| format!("{error:?}"))?;
    assert_eq!(path.leaf_count(), 2);
    assert_eq!(path.leaf_index(), 1);
    let proof = layerx_proof::merkle::Proof::new(
        path.leaf_index(),
        path.leaf_count(),
        path.siblings().to_vec(),
    )
    .map_err(|error| format!("{error:?}"))?;
    let authorization = layerx_proof::inclusion::SequencerAuthorization::new(
        Sha256::digest(public).into(),
        public,
        1,
        u64::MAX,
    );
    let evidence = layerx_proof::inclusion::verify_receipt(
        &receipt,
        &proof,
        &header_bytes,
        &signature,
        &authorization,
    )
    .map_err(|error| format!("head verification failed: {error:?}"))?;
    let header = evidence.header().header();
    assert_eq!(header.protocol_version(), 3);
    assert_eq!(header.network_id(), 402);
    let maintenance = layerx_wire::batch_maintenance::decode_maintenance(&receipt)
        .map_err(|error| format!("{error:?}"))?;
    maintenance
        .verify_header(header)
        .map_err(|error| format!("{error:?}"))?;
    assert_eq!(bytes("/state_root")?, header.resulting_state_root());
    assert_eq!(head["observed_sequence"], header.last_sequence());
    assert_eq!(head["observed_at"], header.timestamp_ms());
    assert_eq!(
        bytes("/receipt_digest")?,
        Sha256::digest(&receipt).as_slice()
    );
    let state_reply = request(&address, "GET", "/v1/state", "", &[])?;
    assert_eq!(state_reply.status, 200);
    assert_eq!(
        response_result(&state_reply)?["canonical_state_root"],
        head["state_root"]
    );
    let mut changed_signature = signature;
    changed_signature[0] ^= 1;
    assert!(layerx_proof::inclusion::verify_receipt(
        &receipt,
        &proof,
        &header_bytes,
        &changed_signature,
        &authorization
    )
    .is_err());
    let mut changed_receipt = receipt.clone();
    *changed_receipt.last_mut().ok_or("empty receipt")? ^= 1;
    assert!(layerx_proof::inclusion::verify_receipt(
        &changed_receipt,
        &proof,
        &header_bytes,
        &signature,
        &authorization
    )
    .is_err());
    let mut changed_header = header_bytes.clone();
    *changed_header.last_mut().ok_or("empty header")? ^= 1;
    assert!(layerx_proof::inclusion::verify_receipt(
        &receipt,
        &proof,
        &changed_header,
        &signature,
        &authorization
    )
    .is_err());
    let wrong_public = ed25519_dalek::SigningKey::from_bytes(&[0x24; 32])
        .verifying_key()
        .to_bytes();
    let wrong_authorization = layerx_proof::inclusion::SequencerAuthorization::new(
        Sha256::digest(wrong_public).into(),
        wrong_public,
        1,
        u64::MAX,
    );
    assert!(layerx_proof::inclusion::verify_receipt(
        &receipt,
        &proof,
        &header_bytes,
        &signature,
        &wrong_authorization
    )
    .is_err());
    Ok(())
}
