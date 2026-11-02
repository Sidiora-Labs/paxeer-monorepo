use layerx_platform_gateway::http;
use openssl::asn1::Asn1Time;
use openssl::ec::{EcGroup, EcKey};
use openssl::hash::MessageDigest;
use openssl::nid::Nid;
use openssl::pkcs12::Pkcs12;
use openssl::pkey::PKey;
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::thread;
use std::time::{Duration, Instant};

type TestResult<T = ()> = Result<T, Box<dyn std::error::Error>>;

static FIXTURES: AtomicUsize = AtomicUsize::new(0);
const TIMEOUT: Duration = Duration::from_secs(10);

struct Fixture {
    dir: PathBuf,
    listen: Option<TcpListener>,
    receipt_keys: Value,
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.dir);
    }
}

struct Gateway(Child);

impl Drop for Gateway {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

impl Fixture {
    fn new() -> TestResult<Self> {
        let dir = std::env::temp_dir().join(format!(
            "layerx-interop-sequencer-start-{}-{}",
            std::process::id(),
            FIXTURES.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&dir)?;
        fs::set_permissions(&dir, fs::Permissions::from_mode(0o700))?;
        let group = EcGroup::from_curve_name(Nid::X9_62_PRIME256V1)?;
        let key = PKey::from_ec_key(EcKey::generate(&group)?)?;
        let mut name = X509NameBuilder::new()?;
        name.append_entry_by_text("CN", "localhost")?;
        let name = name.build();
        let mut cert = X509::builder()?;
        cert.set_version(2)?;
        cert.set_subject_name(&name)?;
        cert.set_issuer_name(&name)?;
        cert.set_pubkey(&key)?;
        let not_before = Asn1Time::days_from_now(0)?;
        let not_after = Asn1Time::days_from_now(1)?;
        cert.set_not_before(&not_before)?;
        cert.set_not_after(&not_after)?;
        cert.sign(&key, MessageDigest::sha256())?;
        let cert = cert.build();
        fs::write(dir.join("cert.der"), cert.to_der()?)?;
        let identity = Pkcs12::builder()
            .name("interop startup test")
            .pkey(&key)
            .cert(&cert)
            .build2("interop-test-password")?;
        fs::write(dir.join("client.p12"), identity.to_der()?)?;

        let public_key = hex(&ed25519_dalek::SigningKey::from_bytes(&[7; 32])
            .verifying_key()
            .to_bytes());
        let receipt_keys = json!([{
            "keyId": format!("0x{}", "11".repeat(32)),
            "publicKey": format!("0x{public_key}"),
            "firstHeight": 1,
            "lastHeight": 100
        }]);
        fs::write(
            dir.join("receipt-keys.json"),
            serde_json::to_vec(&receipt_keys)?,
        )?;
        for (name, contents) in [
            ("identity-password", "interop-test-password".to_owned()),
            ("redis-username", "interop".to_owned()),
            ("redis-password", "interop-test-redis-password".to_owned()),
            ("authority-token", "interop-test-authority-token".to_owned()),
            ("sequencer-public-key", public_key.clone()),
            ("sequencer-id", "11".repeat(32)),
            ("sequencer-first-batch", "1".to_owned()),
            ("sequencer-last-batch", "100".to_owned()),
            (
                "registry.json",
                r#"{"modules":[{"module":1,"ordinals":[1]}]}"#.to_owned(),
            ),
        ] {
            let path = dir.join(name);
            fs::write(&path, contents)?;
            fs::set_permissions(path, fs::Permissions::from_mode(0o600))?;
        }
        let mut manifest: Value = serde_json::from_slice(include_bytes!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../deploy/gateway/config.example.json"
        )))?;
        manifest["x402_supported"] = json!({
            "kinds": [{"x402Version": 2, "scheme": "exact", "network": "eip155:125"}],
            "extensions": [],
            "signers": {"eip155:125": [format!("did:layerx:{public_key}")]}
        });
        let ap2_key = p256::ecdsa::SigningKey::from_bytes((&[9; 32]).into())?;
        let ap2_public_key = hex(ap2_key.verifying_key().to_encoded_point(false).as_bytes());
        for pin in manifest["ap2_keys"].as_array_mut().ok_or("AP2 keys")? {
            pin["public_key_sec1"] = json!(ap2_public_key);
        }
        manifest["visa_agents"][0]["public_key"] = json!(public_key);
        manifest["fiat_providers"][0]["public_key_ed25519"] = json!(public_key);
        fs::write(dir.join("config.json"), serde_json::to_vec(&manifest)?)?;
        Ok(Self {
            dir,
            listen: Some(TcpListener::bind("127.0.0.1:0")?),
            receipt_keys,
        })
    }

    fn command(&self, sequencer: SocketAddr) -> TestResult<Command> {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-interop-gateway"));
        command.env_clear();
        for (variable, file) in [
            ("CONFIG", "config.json"),
            ("OUTBOUND_CA_DER", "cert.der"),
            ("CLIENT_IDENTITY_PKCS12", "client.p12"),
            ("CLIENT_IDENTITY_PASSWORD_FILE", "identity-password"),
            ("REDIS_USERNAME_FILE", "redis-username"),
            ("REDIS_PASSWORD_FILE", "redis-password"),
            ("SEQUENCER_PUBLIC_KEY_FILE", "sequencer-public-key"),
            ("SEQUENCER_ID_FILE", "sequencer-id"),
            ("SEQUENCER_FIRST_BATCH_FILE", "sequencer-first-batch"),
            ("SEQUENCER_LAST_BATCH_FILE", "sequencer-last-batch"),
            ("RECEIPT_KEYS_FILE", "receipt-keys.json"),
            ("MODULE_REGISTRY_FILE", "registry.json"),
            ("RECEIPT_AUTHORITY_TOKEN_FILE", "authority-token"),
        ] {
            command.env(format!("LAYERX_INTEROP_{variable}"), self.dir.join(file));
        }
        for (variable, value) in [
            ("LISTENER", "plain"),
            ("REDIS_URL", "rediss://localhost:1"),
            ("HOSTED_GATEWAY_URL", "https://localhost:2"),
            ("RECEIPT_AUTHORITY_URL", "https://localhost:3"),
            ("NETWORK_ID", "eip155-125"),
            ("WIRE_VERSION", "3"),
            ("PROTOCOL_NETWORK_ID", "402"),
            ("CHAIN_ID", "125"),
            ("READINESS_MAX_AGE_MS", "30000"),
            ("TAP_CLOCK_SKEW_SECONDS", "60"),
        ] {
            command.env(format!("LAYERX_INTEROP_{variable}"), value);
        }
        command
            .env(
                "LAYERX_INTEROP_LISTEN",
                self.listen
                    .as_ref()
                    .ok_or("reserved listener")?
                    .local_addr()?
                    .to_string(),
            )
            .env(
                "LAYERX_INTEROP_SEQUENCER_URL",
                format!("http://{sequencer}"),
            )
            .stdout(Stdio::null())
            .stderr(Stdio::piped());
        Ok(command)
    }
}

fn stderr(gateway: &mut Gateway) -> TestResult<String> {
    let mut text = String::new();
    gateway
        .0
        .stderr
        .take()
        .ok_or("gateway stderr")?
        .read_to_string(&mut text)?;
    Ok(text)
}

#[test]
fn service_starts_against_a_loopback_sequencer() -> TestResult {
    let mut fixture = Fixture::new()?;
    let sequencer = TcpListener::bind("127.0.0.1:0")?;
    sequencer.set_nonblocking(true)?;
    let endpoint = sequencer.local_addr()?;
    let mut receipt_keys = fixture.receipt_keys.clone();
    for key in receipt_keys.as_array_mut().ok_or("receipt keys")? {
        for field in ["firstHeight", "lastHeight"] {
            let height = key[field].as_u64().ok_or("receipt key height")?;
            key[field] = json!(format!("{height:#x}"));
        }
    }
    let rpc = thread::spawn(move || -> Result<(), String> {
        let deadline = Instant::now() + TIMEOUT;
        let mut stream = loop {
            match sequencer.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    if Instant::now() >= deadline {
                        return Err("sequencer received no startup request".to_owned());
                    }
                    thread::sleep(Duration::from_millis(10));
                }
                Err(error) => return Err(error.to_string()),
            }
        };
        stream
            .set_read_timeout(Some(TIMEOUT))
            .map_err(|error| error.to_string())?;
        stream
            .set_write_timeout(Some(TIMEOUT))
            .map_err(|error| error.to_string())?;
        let request = http::read_request(&mut stream, 64 * 1024)?;
        let rpc: Value =
            serde_json::from_slice(&request.body).map_err(|error| error.to_string())?;
        assert_eq!(request.method, "POST");
        assert_eq!(rpc["jsonrpc"], "2.0");
        assert_eq!(rpc["method"], "px_getReceiptKeys");
        assert_eq!(rpc["params"], json!([]));
        http::write_response(
            &mut stream,
            &http::OutgoingResponse {
                status: 200,
                content_type: "application/json".to_owned(),
                headers: Vec::new(),
                body: serde_json::to_vec(&json!({
                    "jsonrpc": "2.0", "id": rpc["id"], "result": receipt_keys
                }))
                .map_err(|error| error.to_string())?,
                retry_after: None,
            },
        )
    });
    let address = fixture
        .listen
        .as_ref()
        .ok_or("reserved listener")?
        .local_addr()?;
    let mut command = fixture.command(endpoint)?;
    drop(fixture.listen.take());
    let mut gateway = Gateway(command.spawn()?);
    rpc.join().map_err(|_| "sequencer listener panicked")??;
    let deadline = Instant::now() + TIMEOUT;
    let mut stream = loop {
        if let Some(status) = gateway.0.try_wait()? {
            return Err(format!("gateway exited {status}: {}", stderr(&mut gateway)?).into());
        }
        match TcpStream::connect_timeout(&address, Duration::from_millis(100)) {
            Ok(stream) => break stream,
            Err(_) if Instant::now() < deadline => thread::sleep(Duration::from_millis(10)),
            Err(error) => return Err(error.into()),
        }
    };
    stream.set_read_timeout(Some(TIMEOUT))?;
    stream.set_write_timeout(Some(TIMEOUT))?;
    stream.write_all(b"GET /readyz HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")?;
    let mut response = String::new();
    stream.read_to_string(&mut response)?;
    let (headers, body) = response.split_once("\r\n\r\n").ok_or("HTTP response")?;
    assert!(headers.starts_with("HTTP/1.1 503"), "{response}");
    let ready: Value = serde_json::from_str(body)?;
    assert_eq!(ready["service"], "layerx-interop-gateway");
    assert_eq!(ready["status"], "degraded");
    assert_eq!(ready["network_id"], "eip155-125");
    assert_eq!(ready["capabilities"]["chain_serving"]["chain_id"], 125);
    assert_eq!(ready["components"]["durable_gateway_store"], "unavailable");
    assert!(
        gateway.0.try_wait()?.is_none(),
        "gateway must remain running"
    );
    Ok(())
}

#[test]
fn service_refuses_start_without_a_sequencer() -> TestResult {
    let fixture = Fixture::new()?;
    let sequencer = TcpListener::bind("127.0.0.1:0")?;
    let endpoint = sequencer.local_addr()?;
    let mut command = fixture.command(endpoint)?;
    drop(sequencer);
    let mut gateway = Gateway(command.spawn()?);
    let deadline = Instant::now() + TIMEOUT;
    let status = loop {
        if let Some(status) = gateway.0.try_wait()? {
            break status;
        }
        assert!(Instant::now() < deadline, "gateway did not refuse startup");
        thread::sleep(Duration::from_millis(10));
    };
    assert!(
        !status.success(),
        "gateway accepted an unreachable sequencer"
    );
    assert_eq!(stderr(&mut gateway)?, "layerx interoperability gateway refused startup: sequencer px_getReceiptKeys failed: Unavailable\n");
    Ok(())
}
