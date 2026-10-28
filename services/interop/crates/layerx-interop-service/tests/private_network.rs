use openssl::asn1::Asn1Time;
use openssl::ec::{EcGroup, EcKey};
use openssl::hash::MessageDigest;
use openssl::nid::Nid;
use openssl::pkcs12::Pkcs12;
use openssl::pkey::PKey;
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::process::Command;
use std::sync::atomic::{AtomicUsize, Ordering};

type TestResult<T = ()> = Result<T, Box<dyn std::error::Error>>;

static FIXTURES: AtomicUsize = AtomicUsize::new(0);

fn startup_command(url: &str, flag: Option<&str>) -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-interop-gateway"));
    command.env_clear().env("LAYERX_INTEROP_SEQUENCER_URL", url);
    if let Some(flag) = flag {
        command.env("LAYERX_INTEROP_ALLOW_RAILWAY_PRIVATE_NETWORK", flag);
    }
    command
}

fn startup_error(url: &str, flag: Option<&str>) -> String {
    command_error(startup_command(url, flag))
}

fn command_error(mut command: Command) -> String {
    let output = command.output().expect("interop gateway runs");
    assert!(
        !output.status.success(),
        "gateway started without its configuration"
    );
    String::from_utf8_lossy(&output.stderr).into_owned()
}

struct EndpointFixture(PathBuf);

impl Drop for EndpointFixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

impl EndpointFixture {
    fn new() -> TestResult<Self> {
        let dir = std::env::temp_dir().join(format!(
            "layerx-interop-private-network-{}-{}",
            std::process::id(),
            FIXTURES.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&dir)?;
        let fixture = Self(dir);
        fs::set_permissions(&fixture.0, fs::Permissions::from_mode(0o700))?;
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
        fs::write(fixture.0.join("cert.der"), cert.to_der()?)?;
        let identity = Pkcs12::builder()
            .name("interop private network test")
            .pkey(&key)
            .cert(&cert)
            .build2("interop-test-password")?;
        fs::write(fixture.0.join("client.p12"), identity.to_der()?)?;
        for (name, contents) in [
            ("identity-password", "interop-test-password"),
            ("redis-username", "interop"),
            ("redis-password", "interop-test-redis-password"),
        ] {
            let path = fixture.0.join(name);
            fs::write(&path, contents)?;
            fs::set_permissions(path, fs::Permissions::from_mode(0o600))?;
        }
        let mut manifest: Value = serde_json::from_slice(include_bytes!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../deploy/gateway/config.example.json"
        )))?;
        let public_key: String = ed25519_dalek::SigningKey::from_bytes(&[7; 32])
            .verifying_key()
            .to_bytes()
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect();
        manifest["x402_supported"] = json!({
            "kinds": [{"x402Version": 2, "scheme": "exact", "network": "eip155:125"}],
            "extensions": [],
            "signers": {"eip155:125": [format!("did:layerx:{public_key}")]}
        });
        let ap2_key = p256::ecdsa::SigningKey::from_bytes((&[9; 32]).into())?;
        let ap2_public_key: String = ap2_key
            .verifying_key()
            .to_encoded_point(false)
            .as_bytes()
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect();
        for pin in manifest["ap2_keys"].as_array_mut().ok_or("AP2 keys")? {
            pin["public_key_sec1"] = json!(ap2_public_key);
        }
        manifest["visa_agents"][0]["public_key"] = json!(public_key);
        manifest["fiat_providers"][0]["public_key_ed25519"] = json!(public_key);
        fs::write(
            fixture.0.join("config.json"),
            serde_json::to_vec(&manifest)?,
        )?;
        Ok(fixture)
    }

    fn configure(&self, command: &mut Command) {
        for (variable, file) in [
            ("CONFIG", "config.json"),
            ("OUTBOUND_CA_DER", "cert.der"),
            ("CLIENT_IDENTITY_PKCS12", "client.p12"),
            ("CLIENT_IDENTITY_PASSWORD_FILE", "identity-password"),
            ("REDIS_USERNAME_FILE", "redis-username"),
            ("REDIS_PASSWORD_FILE", "redis-password"),
        ] {
            command.env(format!("LAYERX_INTEROP_{variable}"), self.0.join(file));
        }
        command.env("LAYERX_INTEROP_REDIS_URL", "rediss://localhost:1");
    }
}

const PRIVATE: &str = "http://router.railway.internal:9443";
const REFUSED: &str =
    "LAYERX_INTEROP_SEQUENCER_URL may use http:// only as http://<name>.railway.internal:<port>";
const PASSED: &str = "LAYERX_INTEROP_CONFIG is required";

#[test]
fn flag_off_refuses_plain_sequencer_url() {
    for flag in [None, Some("0")] {
        let stderr = startup_error(PRIVATE, flag);
        assert!(
            stderr.contains("component endpoint must use HTTPS"),
            "{stderr}"
        );
    }
}

#[test]
fn flag_on_accepts_private_plain_sequencer_url() -> TestResult {
    let stderr = startup_error(PRIVATE, Some("1"));
    assert!(stderr.contains(PASSED), "{stderr}");
    let fixture = EndpointFixture::new()?;
    let mut command = startup_command(PRIVATE, Some("1"));
    fixture.configure(&mut command);
    let stderr = command_error(command);
    assert_eq!(
        stderr,
        "layerx interoperability gateway refused startup: LAYERX_INTEROP_RECEIPT_KEYS_FILE is required\n"
    );
    Ok(())
}

#[test]
fn flag_on_refuses_public_plain_sequencer_url() {
    let stderr = startup_error("http://example.com:9443", Some("1"));
    assert!(stderr.contains(REFUSED), "{stderr}");
}

#[test]
fn flag_on_refuses_plain_sequencer_url_without_port() {
    for url in [
        "http://router.railway.internal",
        "http://router.railway.internal:",
        "http://router.railway.internal:9443/",
    ] {
        let stderr = startup_error(url, Some("1"));
        assert!(stderr.contains(REFUSED), "{url}: {stderr}");
    }
}

#[test]
fn flag_on_refuses_malformed_private_plain_sequencer_url() {
    for url in [
        "http://.railway.internal:9443",
        "http://-router.railway.internal:9443",
        "http://router-.railway.internal:9443",
        "http://Router.railway.internal:9443",
        "http://router_name.railway.internal:9443",
        "http://sub.router.railway.internal:9443",
        "http://router.railway.internal.example:9443",
        "http://router.railway.internal.:9443",
        "http://user@router.railway.internal:9443",
        "http://router.railway.internal:0",
        "http://router.railway.internal:65536",
        "http://router.railway.internal:000001",
        "http://router.railway.internal:+9443",
        "http://router.railway.internal:9443/rpc",
        "http://router.railway.internal:9443?query=1",
        "http://router.railway.internal:9443#fragment",
    ] {
        let stderr = startup_error(url, Some("1"));
        assert!(stderr.contains(REFUSED), "{url}: {stderr}");
    }
    let url = format!("http://{}.railway.internal:9443", "a".repeat(64));
    let stderr = startup_error(&url, Some("1"));
    assert!(stderr.contains(REFUSED), "{url}: {stderr}");
}

#[test]
fn flag_off_keeps_plain_loopback_sequencer_url() {
    for url in [
        "http://127.0.0.1:9443",
        "http://localhost:9443",
        "http://[::1]:9443",
    ] {
        for flag in [None, Some("0")] {
            let stderr = startup_error(url, flag);
            assert!(stderr.contains(PASSED), "{url}: {stderr}");
        }
    }
}

#[test]
fn malformed_flag_fails_closed() {
    for flag in ["", "true", "yes", " 1"] {
        let stderr = startup_error(PRIVATE, Some(flag));
        assert!(
            stderr.contains("LAYERX_INTEROP_ALLOW_RAILWAY_PRIVATE_NETWORK must be 0 or 1"),
            "{flag:?}: {stderr}"
        );
    }
}

#[test]
fn flag_on_keeps_https_sequencer_url() {
    let stderr = startup_error("https://router.railway.internal:9443", Some("1"));
    assert!(stderr.contains(PASSED), "{stderr}");
}

#[test]
fn arguments_are_refused_before_configuration() {
    for args in [
        vec!["--verify-readiness-receipt", "/missing/receipt.json"],
        vec!["--verify-readiness-receipt"],
        vec!["--unknown"],
        vec![""],
    ] {
        let output = Command::new(env!("CARGO_BIN_EXE_layerx-interop-gateway"))
            .env_clear()
            .args(&args)
            .output()
            .expect("interop gateway runs");
        assert_eq!(output.status.code(), Some(1), "{args:?}");
        assert_eq!(
            String::from_utf8_lossy(&output.stderr),
            "layerx interoperability gateway refused startup: invalid interoperability gateway arguments\n",
            "{args:?}"
        );
    }
}
