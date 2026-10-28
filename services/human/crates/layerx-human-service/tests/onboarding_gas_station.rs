#[path = "../../../../interop/crates/layerx-gas-station/src/lib.rs"]
mod station_core;
pub use station_core::*;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_human_service::server::production_components::onboarding_sponsor_command;
use layerx_platform_internal::{http, tls};
use serde_json::{json, Value};
use std::fs;
use std::net::TcpListener;
use std::path::PathBuf;
use std::process::Command;
use std::sync::Mutex;
use std::time::{Duration, Instant};

type TestResult = Result<(), Box<dyn std::error::Error>>;
static ENVIRONMENT: Mutex<()> = Mutex::new(());

struct Environment {
    directory: PathBuf,
    variables: Vec<(String, Option<std::ffi::OsString>)>,
}

impl Environment {
    fn new() -> Result<Self, Box<dyn std::error::Error>> {
        let directory = std::env::temp_dir().join(format!(
            "onboarding-gas-station-{}",
            layerx_platform_internal::secret::random_hex(12)?.as_str()
        ));
        fs::create_dir(&directory)?;
        Ok(Self {
            directory,
            variables: Vec::new(),
        })
    }

    fn set(&mut self, name: &str, value: impl AsRef<std::ffi::OsStr>) {
        if !self.variables.iter().any(|(key, _)| key == name) {
            self.variables
                .push((name.to_owned(), std::env::var_os(name)));
        }
        std::env::set_var(name, value);
    }

    fn unset(&mut self, name: &str) {
        if !self.variables.iter().any(|(key, _)| key == name) {
            self.variables
                .push((name.to_owned(), std::env::var_os(name)));
        }
        std::env::remove_var(name);
    }

    fn registry(&mut self, document: &Value) -> TestResult {
        let path = self.directory.join("onboarding-registry.json");
        fs::write(&path, serde_json::to_vec(document)?)?;
        self.set("LAYERX_HUMAN_ONBOARDING_REGISTRY_FILE", path);
        Ok(())
    }

    fn certificates(
        &mut self,
    ) -> Result<std::sync::Arc<rustls::ServerConfig>, Box<dyn std::error::Error>> {
        for args in [
            vec![
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-keyout",
                "server.key",
                "-out",
                "server.pem",
                "-days",
                "1",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost",
                "-addext",
                "basicConstraints=critical,CA:TRUE",
            ],
            vec![
                "x509",
                "-in",
                "server.pem",
                "-outform",
                "DER",
                "-out",
                "server.der",
            ],
            vec![
                "pkcs8",
                "-topk8",
                "-nocrypt",
                "-in",
                "server.key",
                "-outform",
                "DER",
                "-out",
                "server-key.der",
            ],
        ] {
            let output = Command::new("openssl")
                .current_dir(&self.directory)
                .args(args)
                .output()?;
            assert!(
                output.status.success(),
                "certificate generation: {}",
                String::from_utf8_lossy(&output.stderr)
            );
        }
        self.set(
            "ONBOARDING_TEST_TLS_CERT_DER",
            self.directory.join("server.der"),
        );
        self.set(
            "ONBOARDING_TEST_TLS_KEY_DER",
            self.directory.join("server-key.der"),
        );
        self.set(
            "LAYERX_HUMAN_GAS_STATION_UPSTREAM_CA_DER",
            self.directory.join("server.der"),
        );
        tls::server_config("ONBOARDING_TEST").map_err(Into::into)
    }
}

impl Drop for Environment {
    fn drop(&mut self) {
        for (name, value) in self.variables.iter().rev() {
            match value {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
        let _ = fs::remove_dir_all(&self.directory);
    }
}

fn hex(bytes: &[u8]) -> String {
    rpc::hex(bytes)
}

fn decimal(value: &[u8; 32]) -> Result<String, String> {
    if value[..16] != [0; 16] {
        return Err("quote word exceeds u128".to_owned());
    }
    let bytes: [u8; 16] = value[16..].try_into().map_err(|_| "quote word length")?;
    Ok(u128::from_be_bytes(bytes).to_string())
}

#[test]
fn onboarding_prepare_builds_a_bind_layerx_call() -> TestResult {
    let _guard = ENVIRONMENT
        .lock()
        .map_err(|_| "environment lock poisoned")?;
    let mut environment = Environment::new()?;
    environment.unset("LAYERX_HUMAN_GAS_STATION_UPSTREAM_TOKEN_FILE");
    environment.unset("LAYERX_HUMAN_GAS_STATION_UPSTREAM_CLIENT_IDENTITY_PKCS12");
    environment.unset("LAYERX_HUMAN_GAS_STATION_UPSTREAM_COOKIE_FILE");
    let tls = environment.certificates()?;
    let listener = TcpListener::bind((std::net::Ipv4Addr::LOCALHOST, 0))?;
    listener.set_nonblocking(true)?;
    let endpoint = format!("https://localhost:{}", listener.local_addr()?.port());
    environment.registry(&json!({"chain_id":125,"gas_station":endpoint}))?;
    environment.set("LAYERX_HUMAN_GAS_STATION_UPSTREAM_URL", &endpoint);
    environment.set("ONBOARDING_STATION_SIGNER", format!("{:064x}", 1));
    let mut config = config::tests::config();
    config.chain_id = 125;
    config.per_account_limit = 8_000_000;
    let signer = signer::LocalSigner::from_env("ONBOARDING_STATION_SIGNER")?;
    let mut station = Station::new(config, signer)?;
    let server = std::thread::spawn(move || -> Result<Vec<Value>, String> {
        let mut received = Vec::new();
        let deadline = Instant::now() + Duration::from_secs(15);
        while received.len() < 2 {
            let (tcp, _) = match listener.accept() {
                Ok(connection) => connection,
                Err(error)
                    if error.kind() == std::io::ErrorKind::WouldBlock
                        && Instant::now() < deadline =>
                {
                    std::thread::sleep(Duration::from_millis(5));
                    continue;
                }
                Err(error) => return Err(error.to_string()),
            };
            tcp.set_read_timeout(Some(Duration::from_secs(5)))
                .map_err(|error| error.to_string())?;
            tcp.set_write_timeout(Some(Duration::from_secs(5)))
                .map_err(|error| error.to_string())?;
            let connection =
                rustls::ServerConnection::new(tls.clone()).map_err(|error| error.to_string())?;
            let mut stream = rustls::StreamOwned::new(connection, tcp);
            let request = http::parse_client_request(&mut stream)?;
            assert_eq!(request.method, "POST");
            assert_eq!(request.path, "/quote");
            let body: Value =
                serde_json::from_slice(&request.body).map_err(|error| error.to_string())?;
            assert_eq!(body["chainId"], "125");
            let signed = station
                .quote(
                    &QuoteRequest {
                        account: rpc::bytes(body["account"].as_str().ok_or("account required")?)
                            .map_err(|error| error.to_string())?
                            .try_into()
                            .map_err(|_| "account length")?,
                        max_token_amount: body["maxTokenAmount"]
                            .as_str()
                            .ok_or("maximum required")?
                            .parse::<u128>()
                            .map_err(|error| error.to_string())?,
                        gas_cost: body["gasCost"]
                            .as_str()
                            .ok_or("gas cost required")?
                            .parse::<u128>()
                            .map_err(|error| error.to_string())?,
                        deadline: 1019,
                        quote_nonce: quote::word(
                            u128::try_from(received.len()).map_err(|error| error.to_string())?,
                        ),
                    },
                    &price::tests::RATE,
                    u128::MAX,
                    1000,
                )
                .map_err(|error| format!("{error:?}"))?;
            let q = signed.quote;
            let answer = json!({"quote":{
                "sponsor":hex(&q.sponsor),"token":hex(&q.token),
                "maxTokenAmount":decimal(&q.max_token_amount)?,"tokenAmount":decimal(&q.token_amount)?,
                "deadline":decimal(&q.deadline)?,"quoteNonce":decimal(&q.nonce)?,
                "gasCost":decimal(&q.gas_cost)?,"decimals":6
            },"relayerSignature":hex(&signed.signature)});
            http::write_response(&mut stream, &http::json(200, &answer))?;
            received.push(body);
        }
        Ok(received)
    });
    let key = SigningKey::from_bytes(&[0x42; 32]);
    let mut message = b"LX:PAXEER-BIND:v1".to_vec();
    message.extend_from_slice(&quote::word(125));
    message.extend_from_slice(&[0x11; 20]);
    message.extend_from_slice(&0_u64.to_be_bytes());
    let signature = key.sign(&message).to_bytes();
    let request = json!({
        "account":hex(&[0x11;20]), "did_public_key":hex(&key.verifying_key().to_bytes()),
        "signature":hex(&signature),"bind_nonce":0,"nonce":"0",
        "max_token_amount":2_400_000,"gas_cost":750_000_000_000_000_000_u64,
        "token":hex(&quote::SIDIORA),"decimals":6,
    });
    let input = serde_json::to_vec(&request)?;
    let output = onboarding_sponsor_command("prepare", &input)?;
    let output: Value = serde_json::from_slice(&output)?;
    let call = &output["batch"]["calls"][0];
    assert_eq!(call["to"], "0x0000000000000000000000000000000000001004");
    assert_eq!(call["value"], "0");
    let data = rpc::bytes(call["data"].as_str().ok_or("call data required")?)?;
    assert_eq!(&data[..4], &[0xdd, 0x9a, 0xa6, 0x28]);
    assert_eq!(&data[4..36], key.verifying_key().as_bytes());
    assert_eq!(&data[36..68], &quote::word(64));
    assert_eq!(&data[68..100], &quote::word(64));
    assert_eq!(&data[100..], &signature);
    assert_eq!(output["batch"]["quote"]["tokenAmount"], "2358855");
    let decoded =
        layerx_human_service::onboarding::gas_station::GasStationClient::decode_quote(&json!({
            "quote":output["batch"]["quote"], "relayerSignature":output["relayerSignature"]
        }))?;
    let q = quote::Quote {
        sponsor: decoded.quote.sponsor,
        token: decoded.quote.token,
        max_token_amount: decoded.quote.max_token_amount,
        token_amount: decoded.quote.token_amount,
        deadline: decoded.quote.deadline,
        nonce: decoded.quote.nonce,
        gas_cost: decoded.quote.gas_cost,
    };
    assert_eq!(
        tx::recover(
            quote::quote_digest(quote::word(125), [0x11; 20], &q),
            &decoded.signature
        )?,
        q.sponsor
    );
    environment.registry(&json!({"chain_id":125}))?;
    environment.set("LAYERX_HUMAN_GAS_STATION_ENDPOINT", &endpoint);
    let fallback: Value = serde_json::from_slice(&onboarding_sponsor_command("prepare", &input)?)?;
    assert_eq!(fallback["batch"]["calls"], output["batch"]["calls"]);
    let received = server.join().map_err(|_| "quote server panicked")??;
    assert_eq!(received.len(), 2);
    assert_eq!(received[0]["calls"], output["batch"]["calls"]);
    assert_eq!(received[0]["account"], request["account"]);
    environment.unset("LAYERX_HUMAN_GAS_STATION_ENDPOINT");
    let missing = onboarding_sponsor_command("prepare", &input)
        .err()
        .ok_or("missing station accepted")?;
    assert!(missing.contains("LAYERX_HUMAN_GAS_STATION_ENDPOINT"));
    Ok(())
}

#[test]
fn onboarding_refuses_the_module_registry_document() -> TestResult {
    let _guard = ENVIRONMENT
        .lock()
        .map_err(|_| "environment lock poisoned")?;
    let mut environment = Environment::new()?;
    for registry in [
        json!({"network_id":125,"protocol_version":3,"modules":[{"module_id":1,"activity_types":[1]}]}),
        json!({"chain_id":125,"gas_station":"https://localhost","protocol_version":3,"modules":[]}),
        json!({"chain_id":1,"gas_station":"https://localhost"}),
    ] {
        environment.registry(&registry)?;
        let error = onboarding_sponsor_command("prepare", b"{}")
            .err()
            .ok_or("retired registry accepted")?;
        assert!(error.contains("LAYERX_HUMAN_ONBOARDING_REGISTRY_FILE"));
        assert!(error.contains("onboarding-registry.json"));
    }
    Ok(())
}

#[test]
fn onboarding_settlement_recipient_operation_is_unknown() -> TestResult {
    let _guard = ENVIRONMENT
        .lock()
        .map_err(|_| "environment lock poisoned")?;
    let mut environment = Environment::new()?;
    environment.registry(&json!({"chain_id":125,"gas_station":"https://localhost"}))?;
    assert_eq!(
        onboarding_sponsor_command("settlement-recipient", b"{}"),
        Err("unknown sponsor operation".to_owned())
    );
    Ok(())
}
