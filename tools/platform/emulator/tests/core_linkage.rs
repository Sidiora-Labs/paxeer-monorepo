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
fn head_read_without_a_sealed_batch_is_refused() -> Result<(), String> {
    let address = boot_protocol(3)?;
    let public = ed25519_dalek::SigningKey::from_bytes(&EMULATOR_SEED)
        .verifying_key()
        .to_bytes();
    let public_hex = hex_encode(&public)?;
    for (did, amount) in [("source", 1000), ("destination", 0)] {
        let body = serde_json::json!({
            "did": format!("did:layerx:head-{did}"),
            "public_key": public_hex,
            "amount_lo": amount,
        })
        .to_string();
        let reply = post_json(&address, "/__emulator/accounts/prefund", &body)?;
        assert_eq!(reply.status, 200, "prefund: {}", reply.text());
    }

    let head = request(&address, "GET", "/v1/protocol/account-state/head", "", &[])?;
    assert_eq!(head.status, 503, "head: {}", head.text());
    assert_eq!(head.content_type, "application/json");
    let body: serde_json::Value =
        serde_json::from_slice(&head.body).map_err(|error| error.to_string())?;
    assert_eq!(body["ok"], false, "head: {}", head.text());
    assert_eq!(
        body["error"]["code"],
        "principal_state_proof_unavailable",
        "head: {}",
        head.text()
    );

    let health = request(&address, "GET", "/healthz", "", &[])?;
    assert_eq!(health.status, 200, "health: {}", health.text());
    Ok(())
}
