//! In-process integration coverage for the local emulator gateway.
//!
//! These tests boot the real `layerx_platform_emulator::run` listener, which
//! links the production `LayerX` core transition and receipt machinery through
//! the C bridge, and drive it over its HTTP surface exactly as an SDK or the
//! middleware would. They assert the production gateway surface
//! (`/v1/surface`, `/v1/state`, `/v1/receipts/<id>`) and the emulator-only
//! control hooks (`/__emulator/*`) that live clearly outside the deterministic
//! transition path.

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
fn boot() -> Result<String, String> {
    boot_protocol(2)
}

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

fn post_json(address: &str, path: &str, body: &str) -> Result<Reply, String> {
    request(address, "POST", path, "application/json", body.as_bytes())
}

fn error_code(reply: &Reply) -> Option<String> {
    let text = reply.text();
    let marker = "\"code\":\"";
    let start = text.find(marker)? + marker.len();
    let rest = &text[start..];
    let end = rest.find('"')?;
    Some(rest[..end].to_string())
}

#[test]
fn healthz_reports_the_layerx_core() -> Result<(), String> {
    let address = boot()?;
    let reply = request(&address, "GET", "/healthz", "", &[])?;
    assert_eq!(reply.status, 200);
    let text = reply.text();
    assert!(
        text.contains("\"status\":\"ready\""),
        "unexpected body: {text}"
    );
    assert!(
        text.contains("\"core\":\"layerx\""),
        "unexpected body: {text}"
    );
    Ok(())
}

#[test]
fn state_advertises_the_emulator_with_instant_batching() -> Result<(), String> {
    let address = boot()?;
    let reply = request(&address, "GET", "/v1/state", "", &[])?;
    assert_eq!(reply.status, 200);
    let text = reply.text();
    assert!(
        text.contains("\"network_mode\":\"emulator\""),
        "unexpected body: {text}"
    );
    assert!(
        text.contains("\"batch_cadence\":\"instant\""),
        "unexpected body: {text}"
    );
    assert!(text.contains("\"accounts\":[]"), "unexpected body: {text}");
    Ok(())
}

#[test]
fn prefunded_accounts_appear_in_state() -> Result<(), String> {
    let address = boot()?;
    let public_key = "11".repeat(32);
    let body = format!("{{\"did\":\"did:layerx:agent-alpha\",\"public_key\":\"{public_key}\",\"amount_lo\":123456}}");
    let reply = post_json(&address, "/__emulator/accounts/prefund", &body)?;
    assert_eq!(reply.status, 200, "prefund failed: {}", reply.text());
    assert!(reply.text().contains("\"prefunded\":true"));

    let state = request(&address, "GET", "/v1/state", "", &[])?;
    assert_eq!(state.status, 200);
    let text = state.text();
    assert!(
        text.contains("agent:did:layerx:agent-alpha:main"),
        "prefunded account missing from state: {text}"
    );
    assert!(
        text.contains("\"balance_lo\":123456"),
        "prefunded balance missing from state: {text}"
    );
    Ok(())
}

#[test]
fn time_control_is_monotonic() -> Result<(), String> {
    let address = boot()?;
    let target = 1_800_000_000_000_u64;
    let set = post_json(
        &address,
        "/__emulator/time/set",
        &format!("{{\"timestamp_ms\":{target}}}"),
    )?;
    assert_eq!(set.status, 200, "time set refused: {}", set.text());

    let state = request(&address, "GET", "/v1/state", "", &[])?;
    assert!(
        state.text().contains(&format!("\"timestamp_ms\":{target}")),
        "state did not adopt the controlled time: {}",
        state.text()
    );

    let advanced = post_json(&address, "/__emulator/time/advance", "{\"delta_ms\":1000}")?;
    assert_eq!(advanced.status, 200);
    let after = request(&address, "GET", "/v1/state", "", &[])?;
    assert!(
        after
            .text()
            .contains(&format!("\"timestamp_ms\":{}", target + 1000)),
        "advance did not move the clock: {}",
        after.text()
    );

    let backward = post_json(&address, "/__emulator/time/set", "{\"timestamp_ms\":1}")?;
    assert_eq!(backward.status, 400, "non-monotonic set was accepted");
    Ok(())
}

#[test]
fn snapshots_round_trip_through_the_core() -> Result<(), String> {
    let address = boot()?;
    let public_key = "22".repeat(32);
    let body = format!(
        "{{\"did\":\"did:layerx:agent-beta\",\"public_key\":\"{public_key}\",\"amount_lo\":777}}"
    );
    assert_eq!(
        post_json(&address, "/__emulator/accounts/prefund", &body)?.status,
        200
    );

    let exported = request(&address, "GET", "/__emulator/snapshot", "", &[])?;
    assert_eq!(exported.status, 200, "export refused: {}", exported.text());
    assert_eq!(
        exported.content_type,
        "application/vnd.layerx.emulator-snapshot"
    );
    assert!(!exported.body.is_empty(), "snapshot body was empty");

    let imported = request(
        &address,
        "PUT",
        "/__emulator/snapshot",
        "application/octet-stream",
        &exported.body,
    )?;
    assert_eq!(imported.status, 200, "import refused: {}", imported.text());
    assert!(imported.text().contains("\"imported\":true"));

    let state = request(&address, "GET", "/v1/state", "", &[])?;
    assert!(
        state.text().contains("agent:did:layerx:agent-beta:main"),
        "restored snapshot lost the prefunded account: {}",
        state.text()
    );
    Ok(())
}

#[test]
fn gateway_surface_matches_production_verbs() -> Result<(), String> {
    let address = boot()?;

    let discovery = request(&address, "GET", "/v1/surface", "", &[])?;
    assert_eq!(discovery.status, 200);
    let document: serde_json::Value =
        serde_json::from_slice(&discovery.body).map_err(|error| error.to_string())?;
    assert_eq!(document["result"]["payments"]["supported"], false);
    assert_eq!(
        document["result"]["historical_imports"]["mode"],
        "inspection_only"
    );
    let retired_settlement = post_json(&address, "/v1/settle", "{}")?;
    assert_eq!(retired_settlement.status, 410);
    assert_eq!(
        error_code(&retired_settlement).as_deref(),
        Some("settlement_authority_retired")
    );

    let wrong_verb = post_json(&address, "/v1/state", "{}")?;
    assert_eq!(wrong_verb.status, 405, "state accepted a write verb");
    assert_eq!(
        error_code(&wrong_verb).as_deref(),
        Some("method_not_allowed")
    );

    let missing_receipt = request(&address, "GET", "/v1/receipts/unknown-id", "", &[])?;
    assert_eq!(missing_receipt.status, 404);
    assert_eq!(error_code(&missing_receipt).as_deref(), Some("not_found"));

    let unknown_route = request(&address, "GET", "/v1/does-not-exist", "", &[])?;
    assert_eq!(unknown_route.status, 404);
    assert_eq!(error_code(&unknown_route).as_deref(), Some("not_found"));
    Ok(())
}

#[test]
fn retired_kernel_routes_answer_not_found() -> Result<(), String> {
    for protocol in [2, 3] {
        let address = boot_protocol(protocol)?;
        let id = "a".repeat(64);
        let routes = [
            ("POST", "/v1/activities".to_owned()),
            ("POST", "/v1/moves".to_owned()),
            ("POST", "/v1/moves/quote".to_owned()),
            ("POST", "/v1/programs".to_owned()),
            ("POST", "/v1/programs/read".to_owned()),
            ("POST", "/v1/programs/call".to_owned()),
            ("POST", "/v1/programs/deploy".to_owned()),
            ("POST", "/v1/programs/upgrade".to_owned()),
            ("POST", "/v1/programs/wind-down".to_owned()),
            ("POST", "/v1/programs/simulate".to_owned()),
            ("GET", "/v1/programs/registry".to_owned()),
            ("GET", format!("/v1/programs/registry/{id}/source")),
            ("GET", format!("/v1/programs/registry/{id}")),
            ("GET", format!("/v1/programs/registry/{id}/interface")),
            ("GET", format!("/v1/programs/receipts/by-idempotency/{id}")),
            ("GET", format!("/v1/programs/activities/{id}")),
            ("GET", "/v1/programs/unknown".to_owned()),
            ("GET", "/v1/activities".to_owned()),
            ("GET", "/v1/moves".to_owned()),
            ("GET", "/v1/moves/quote".to_owned()),
        ];
        for (method, path) in routes {
            let body: &[u8] = if method == "POST" { b"{}" } else { b"" };
            let reply = request(&address, method, &path, "application/json", body)?;
            assert_eq!(reply.status, 404, "{method} {path}: {}", reply.text());
            assert_eq!(
                error_code(&reply).as_deref(),
                Some("not_found"),
                "{method} {path}"
            );
        }
        assert_eq!(request(&address, "GET", "/healthz", "", &[])?.status, 200);
    }
    Ok(())
}
