#[path = "support/attestor_cluster.rs"]
mod attestor_cluster;
#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod runtime_clock_server;

use layerx_client::lni::framing::{read_frame, write_frame};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, ServerName};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use sha3::Keccak256;
use zeroize::Zeroizing;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::disclosure::Disclosure;
use layerx_crypto::signer::SignError;
use layerx_human_kms::attestor::{
    new_session_id, preimage, Approval, AttestorCallSignature, AttestorClient, AttestorError,
    AttestorSigner,
};
use p256::ecdsa::signature::Signer as _;
use p256::ecdsa::{Signature, SigningKey};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::error::Error;
use std::fs;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

type Result<T> = std::result::Result<T, Box<dyn Error>>;

const NETWORK: u32 = 125;
const CHAIN_ID: &str = "125";
const ISSUER: &str = "attestor-test-issuer";
const AUDIENCE: &str = "attestor-test-audience";
const OWNER: &str = "user-0001";
const STRANGER: &str = "user-0002";
const ACCOUNT: &str = "0x00000000000000000000000000000000000000a1";
const NODES: usize = 5;
const SIGNERS: [&str; 3] = ["node-1", "node-3", "node-5"];
const JWT_MAX_AGE: &str = "10m";
const BIND_NONCE: u64 = 1;
const TRANSFER: u128 = 5_000;

const MAX_FRAME: usize = 2_097_152;
const PROVIDER: &str = "evm-attestor-test";
const BINDING: [u8; 32] = [3; 32];
const ACTION: [u8; 32] = [2; 32];
const EVM_KEY: &str = "evm-owner";
const PENDING_NONCE: u64 = 7;

#[derive(serde::Serialize, serde::Deserialize)]
#[serde(deny_unknown_fields)]
struct ChainSnapshot {
    chain_id: u64,
}

fn registry() -> ChainSnapshot {
    ChainSnapshot {
        chain_id: u64::from(NETWORK),
    }
}

static SCRATCH: AtomicU64 = AtomicU64::new(0);

fn checked<T, E: std::fmt::Debug>(value: std::result::Result<T, E>) -> Result<T> {
    value.map_err(|error| format!("{error:?}").into())
}

fn scratch(label: &str) -> Result<PathBuf> {
    let nanos = SystemTime::now().duration_since(UNIX_EPOCH)?.as_nanos();
    let root = std::env::temp_dir().join(format!(
        "{label}-{}-{nanos}-{}",
        std::process::id(),
        SCRATCH.fetch_add(1, Ordering::AcqRel)
    ));
    fs::create_dir(&root)?;
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700))?;
    Ok(root)
}

fn openssl(dir: &Path, args: &[&str]) -> Result<()> {
    let output = Command::new("openssl")
        .current_dir(dir)
        .args(args)
        .stdin(Stdio::null())
        .output()?;
    if !output.status.success() {
        return Err(format!(
            "openssl {} failed: {}",
            args.join(" "),
            String::from_utf8_lossy(&output.stderr)
        )
        .into());
    }
    Ok(())
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

struct Pki {
    dir: PathBuf,
    serial: u64,
}

impl Pki {
    fn new(dir: &Path) -> Self {
        Self {
            dir: dir.to_path_buf(),
            serial: 1,
        }
    }

    fn authority(&self, name: &str) -> Result<()> {
        openssl(
            &self.dir,
            &[
                "req",
                "-x509",
                "-newkey",
                "ec",
                "-pkeyopt",
                "ec_paramgen_curve:prime256v1",
                "-nodes",
                "-keyout",
                &format!("{name}.key"),
                "-out",
                &format!("{name}.pem"),
                "-days",
                "1",
                "-subj",
                &format!("/CN={name}"),
                "-addext",
                "basicConstraints=critical,CA:TRUE",
            ],
        )
    }

    fn leaf(&mut self, authority: &str, name: &str, server: bool) -> Result<()> {
        openssl(
            &self.dir,
            &[
                "req",
                "-new",
                "-newkey",
                "ec",
                "-pkeyopt",
                "ec_paramgen_curve:prime256v1",
                "-nodes",
                "-keyout",
                &format!("{name}.key"),
                "-out",
                &format!("{name}.csr"),
                "-subj",
                &format!("/CN={name}"),
            ],
        )?;
        let extensions = if server {
            "basicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=serverAuth,clientAuth\nsubjectAltName=IP:127.0.0.1\n"
        } else {
            "basicConstraints=CA:FALSE\nkeyUsage=critical,digitalSignature\nextendedKeyUsage=clientAuth\n"
        };
        fs::write(self.dir.join(format!("{name}.ext")), extensions)?;
        self.serial += 1;
        openssl(
            &self.dir,
            &[
                "x509",
                "-req",
                "-in",
                &format!("{name}.csr"),
                "-CA",
                &format!("{authority}.pem"),
                "-CAkey",
                &format!("{authority}.key"),
                "-set_serial",
                &self.serial.to_string(),
                "-days",
                "1",
                "-sha256",
                "-extfile",
                &format!("{name}.ext"),
                "-out",
                &format!("{name}.pem"),
            ],
        )
    }

    fn certificate_der(&self, name: &str) -> Result<Vec<u8>> {
        openssl(
            &self.dir,
            &[
                "x509",
                "-in",
                &format!("{name}.pem"),
                "-outform",
                "DER",
                "-out",
                &format!("{name}.der"),
            ],
        )?;
        Ok(fs::read(self.dir.join(format!("{name}.der")))?)
    }

    fn key_der(&self, name: &str) -> Result<Vec<u8>> {
        openssl(
            &self.dir,
            &[
                "pkcs8",
                "-topk8",
                "-nocrypt",
                "-in",
                &format!("{name}.key"),
                "-outform",
                "DER",
                "-out",
                &format!("{name}.pk8"),
            ],
        )?;
        Ok(fs::read(self.dir.join(format!("{name}.pk8")))?)
    }

    fn pin(&self, name: &str) -> Result<String> {
        openssl(
            &self.dir,
            &[
                "x509",
                "-in",
                &format!("{name}.pem"),
                "-noout",
                "-pubkey",
                "-out",
                &format!("{name}.pub"),
            ],
        )?;
        openssl(
            &self.dir,
            &[
                "pkey",
                "-pubin",
                "-in",
                &format!("{name}.pub"),
                "-outform",
                "DER",
                "-out",
                &format!("{name}.spki"),
            ],
        )?;
        Ok(hex(&Sha256::digest(fs::read(
            self.dir.join(format!("{name}.spki")),
        )?)))
    }

    fn path(&self, file: &str) -> PathBuf {
        self.dir.join(file)
    }
}

fn free_addresses(count: usize) -> Result<Vec<SocketAddr>> {
    let listeners = (0..count)
        .map(|_| TcpListener::bind("127.0.0.1:0"))
        .collect::<std::io::Result<Vec<_>>>()?;
    listeners
        .iter()
        .map(|listener| listener.local_addr().map_err(Into::into))
        .collect()
}

fn identity_client(
    pki: &Pki,
    nodes: &[(String, SocketAddr)],
    identity: &str,
) -> Result<AttestorClient> {
    Ok(AttestorClient::new(
        nodes,
        &[pki.certificate_der("gateway-ca")?],
        &[pki.certificate_der(identity)?],
        &pki.key_der(identity)?,
        Duration::from_secs(120),
    )?)
}

fn gateway_client(pki: &Pki, nodes: &[(String, SocketAddr)]) -> Result<AttestorClient> {
    identity_client(pki, nodes, "gateway")
}

fn operator_client(pki: &Pki, nodes: &[(String, SocketAddr)]) -> Result<AttestorClient> {
    identity_client(pki, nodes, "operator")
}

type Responder = Arc<dyn Fn(&[u8]) -> (u16, String) + Send + Sync>;

struct JsonServer {
    address: SocketAddr,
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl JsonServer {
    fn serve(responder: Responder) -> Result<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let shutdown = Arc::new(AtomicBool::new(false));
        let worker = {
            let shutdown = Arc::clone(&shutdown);
            thread::spawn(move || {
                for stream in listener.incoming() {
                    if shutdown.load(Ordering::Acquire) {
                        break;
                    }
                    if let Ok(mut stream) = stream {
                        let responder = Arc::clone(&responder);
                        thread::spawn(move || {
                            let _ = respond(&mut stream, responder.as_ref());
                        });
                    }
                }
            })
        };
        Ok(Self {
            address,
            shutdown,
            worker: Some(worker),
        })
    }

    fn key_set(document: String) -> Result<Self> {
        Self::serve(Arc::new(move |_: &[u8]| (200, document.clone())))
    }

    fn bind_nonces(nonce: u64) -> Result<Self> {
        Self::serve(Arc::new(move |body: &[u8]| bind_nonce_reply(body, nonce)))
    }

    fn url(&self, path: &str) -> String {
        format!("http://{}{path}", self.address)
    }
}

fn bind_nonce_reply(body: &[u8], nonce: u64) -> (u16, String) {
    let Ok(request) = serde_json::from_slice::<Value>(body) else {
        return (400, json!({ "error": "bad request" }).to_string());
    };
    if request.get("method").and_then(Value::as_str) != Some("eth_call") {
        return (400, json!({ "error": "bad request" }).to_string());
    }
    let mut word = [0_u8; 32];
    word[24..].copy_from_slice(&nonce.to_be_bytes());
    (
        200,
        json!({
            "jsonrpc": "2.0",
            "id": request.get("id").cloned().unwrap_or(Value::Null),
            "result": format!("0x{}", hex(&word)),
        })
        .to_string(),
    )
}

impl Drop for JsonServer {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

fn respond(
    stream: &mut TcpStream,
    responder: &(dyn Fn(&[u8]) -> (u16, String) + Send + Sync),
) -> Result<()> {
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    let mut request = Vec::new();
    let mut buffer = [0_u8; 1024];
    let header_end = loop {
        if let Some(end) = request.windows(4).position(|window| window == b"\r\n\r\n") {
            break end + 4;
        }
        let read = stream.read(&mut buffer)?;
        if read == 0 || request.len() > 16 * 1024 {
            return Ok(());
        }
        request.extend_from_slice(&buffer[..read]);
    };
    let headers = String::from_utf8_lossy(&request[..header_end]).to_ascii_lowercase();
    let length = headers
        .lines()
        .find_map(|line| line.strip_prefix("content-length:"))
        .map(|value| value.trim().parse::<usize>())
        .transpose()?
        .unwrap_or(0);
    if length > 64 * 1024 {
        return Ok(());
    }
    while request.len() < header_end + length {
        let read = stream.read(&mut buffer)?;
        if read == 0 {
            return Ok(());
        }
        request.extend_from_slice(&buffer[..read]);
    }
    let (status, document) = responder(&request[header_end..header_end + length]);
    let reason = if status == 200 { "OK" } else { "Bad Request" };
    write!(
        stream,
        "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
        document.len(),
        document
    )?;
    stream.flush()?;
    Ok(())
}

struct TokenSigner {
    key_id: String,
    key: SigningKey,
}

impl TokenSigner {
    fn generate(key_id: &str) -> Result<Self> {
        loop {
            let mut scalar = [0_u8; 32];
            getrandom::fill(&mut scalar)?;
            if let Ok(key) = SigningKey::from_slice(&scalar) {
                return Ok(Self {
                    key_id: key_id.to_owned(),
                    key,
                });
            }
        }
    }

    fn jwk(&self) -> Result<Value> {
        let point = self.key.verifying_key().to_encoded_point(false);
        Ok(json!({
            "kty": "EC",
            "crv": "P-256",
            "kid": self.key_id,
            "use": "sig",
            "alg": "ES256",
            "x": URL_SAFE_NO_PAD.encode(point.x().ok_or("missing x")?),
            "y": URL_SAFE_NO_PAD.encode(point.y().ok_or("missing y")?),
        }))
    }

    fn mint(&self, subject: &str) -> Result<String> {
        let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs();
        let mut id = [0_u8; 16];
        getrandom::fill(&mut id)?;
        let header = json!({ "alg": "ES256", "typ": "JWT", "kid": self.key_id });
        let claims = json!({
            "iss": ISSUER,
            "sub": subject,
            "aud": AUDIENCE,
            "exp": now + 600,
            "iat": now,
            "nbf": now - 5,
            "jti": hex(&id),
        });
        let input = format!(
            "{}.{}",
            URL_SAFE_NO_PAD.encode(header.to_string()),
            URL_SAFE_NO_PAD.encode(claims.to_string())
        );
        let signature: Signature = self.key.sign(input.as_bytes());
        Ok(format!(
            "{input}.{}",
            URL_SAFE_NO_PAD.encode(signature.to_bytes())
        ))
    }
}

struct Cluster {
    root: PathBuf,
    pki: Pki,
    nodes: Vec<(String, SocketAddr)>,
    children: Vec<(String, Child)>,
    tokens: TokenSigner,
    keys: JsonServer,
    chain: JsonServer,
}

impl Drop for Cluster {
    fn drop(&mut self) {
        for (_, child) in &mut self.children {
            let _ = child.kill();
            let _ = child.wait();
        }
        let _ = fs::remove_dir_all(&self.root);
    }
}

impl Cluster {
    fn start() -> Result<Self> {
        let root = scratch("attestor-cluster")?;
        let daemon = build_daemon(&root)?;
        let mut pki = Pki::new(&root);
        pki.authority("gateway-ca")?;
        pki.authority("operator-ca")?;
        pki.leaf("gateway-ca", "gateway", false)?;
        pki.leaf("operator-ca", "operator", false)?;
        let ids: Vec<String> = (1..=NODES).map(|index| format!("node-{index}")).collect();
        let mut pins = Vec::new();
        for id in &ids {
            pki.leaf("gateway-ca", id, true)?;
            pins.push(pki.pin(id)?);
        }
        let tokens = TokenSigner::generate("attestor-test-key")?;
        let keys = JsonServer::key_set(json!({ "keys": [tokens.jwk()?] }).to_string())?;
        let chain = JsonServer::bind_nonces(BIND_NONCE)?;
        fs::write(
            root.join("policy.json"),
            serde_json::to_vec(&json!({
                "version": 1,
                "defaults": {
                    "chain_id": 125,
                    "kinds": ["lx_activity", "evm_tx"],
                    "caps": {"native": {"per_transaction": "1000000000000000000", "daily": "10000000000000000000"}},
                    "rate_per_minute": 1000
                }
            }))?,
        )?;
        fs::write(
            root.join("kernel-policy.json"),
            serde_json::to_vec(&json!({
                "version": 1,
                "defaults": {
                    "modules": {"asset": [5], "programs": [5]},
                    "caps": {"native": {"per_operation": "1000000", "daily": "8000000"}}
                }
            }))?,
        )?;
        let activity_types: Vec<String> = activity_registry()?
            .registrations()
            .iter()
            .flat_map(|module| {
                module
                    .activity_types()
                    .iter()
                    .map(|kind| kind.value().to_string())
            })
            .collect();
        let addresses = free_addresses(NODES * 2)?;
        let nodes: Vec<(String, SocketAddr)> = ids
            .iter()
            .enumerate()
            .map(|(index, id)| (id.clone(), addresses[index]))
            .collect();
        let peers: Vec<SocketAddr> = addresses[NODES..].to_vec();
        let mut cluster = Self {
            root: root.clone(),
            pki,
            nodes,
            children: Vec::new(),
            tokens,
            keys,
            chain,
        };
        for (index, id) in ids.iter().enumerate() {
            let others: Vec<usize> = (0..NODES).filter(|other| *other != index).collect();
            let peer_list = others
                .iter()
                .map(|other| format!("{}={}", ids[*other], peers[*other]))
                .collect::<Vec<_>>()
                .join(",");
            let pin_list = others
                .iter()
                .map(|other| format!("{}={}", ids[*other], pins[*other]))
                .collect::<Vec<_>>()
                .join(",");
            let mut node_key = [0_u8; 32];
            getrandom::fill(&mut node_key)?;
            let key_file = root.join(format!("{id}.node-key"));
            fs::write(&key_file, hex(&node_key))?;
            fs::set_permissions(&key_file, fs::Permissions::from_mode(0o600))?;
            let log = fs::File::create(root.join(format!("{id}.log")))?;
            let child = Command::new(&daemon)
                .env_clear()
                .env("ATTESTOR_NODE_ID", id)
                .env("ATTESTOR_REGION", "test")
                .env("ATTESTOR_LISTEN_ADDR", cluster.nodes[index].1.to_string())
                .env("ATTESTOR_PEER_LISTEN_ADDR", peers[index].to_string())
                .env("ATTESTOR_PEERS", peer_list)
                .env("ATTESTOR_PEER_PINS", pin_list)
                .env("ATTESTOR_NODE_KEY_FILE", &key_file)
                .env("ATTESTOR_DATA_DIR", root.join(format!("{id}.data")))
                .env("ATTESTOR_CHAIN_ID", CHAIN_ID)
                .env("ATTESTOR_JWKS_URL", cluster.keys.url("/jwks.json"))
                .env("ATTESTOR_JWT_ISSUER", ISSUER)
                .env("ATTESTOR_JWT_AUDIENCE", AUDIENCE)
                .env("ATTESTOR_JWT_MAX_AGE", JWT_MAX_AGE)
                .env("ATTESTOR_POLICY_FILE", root.join("policy.json"))
                .env(
                    "ATTESTOR_KERNEL_POLICY_FILE",
                    root.join("kernel-policy.json"),
                )
                .env("ATTESTOR_RPC_URL", cluster.chain.url("/"))
                .env(
                    "ATTESTOR_TLS_CERT_FILE",
                    cluster.pki.path(&format!("{id}.pem")),
                )
                .env(
                    "ATTESTOR_TLS_KEY_FILE",
                    cluster.pki.path(&format!("{id}.key")),
                )
                .env("ATTESTOR_TLS_CA_FILE", cluster.pki.path("gateway-ca.pem"))
                .env(
                    "ATTESTOR_OPERATOR_CA_FILE",
                    cluster.pki.path("operator-ca.pem"),
                )
                .env("ATTESTOR_ACTIVITY_TYPES", activity_types.join(","))
                .stdin(Stdio::null())
                .stdout(log.try_clone()?)
                .stderr(log)
                .spawn()?;
            cluster.children.push((id.clone(), child));
        }
        cluster.wait_ready()?;
        Ok(cluster)
    }

    fn client(&self) -> Result<AttestorClient> {
        gateway_client(&self.pki, &self.nodes)
    }

    fn operator(&self) -> Result<AttestorClient> {
        operator_client(&self.pki, &self.nodes)
    }

    fn wait_ready(&mut self) -> Result<()> {
        let client = self.client()?;
        let started = Instant::now();
        let mut pending: Vec<String> = self.nodes.iter().map(|(id, _)| id.clone()).collect();
        while !pending.is_empty() {
            let mut exited = None;
            for (id, child) in &mut self.children {
                if let Some(status) = child.try_wait()? {
                    exited = Some(format!("{id} exited with {status}"));
                    break;
                }
            }
            if let Some(exit) = exited {
                return Err(format!("{exit}: {}", self.logs()).into());
            }
            pending.retain(|id| {
                !matches!(client.health(id), Ok(health) if health.ready && health.reachable_peers == 4)
            });
            if started.elapsed() > Duration::from_secs(120) {
                return Err(
                    format!("attestors {pending:?} never became ready: {}", self.logs()).into(),
                );
            }
            thread::sleep(Duration::from_millis(200));
        }
        Ok(())
    }

    fn logs(&self) -> String {
        self.nodes
            .iter()
            .map(|(id, _)| {
                format!(
                    "\n{id}: {}",
                    fs::read_to_string(self.root.join(format!("{id}.log"))).unwrap_or_default()
                )
            })
            .collect()
    }

    fn sign_raw(&self, body: &[u8], token: &str) -> Result<Vec<(String, u16, Value)>> {
        self.post_raw("/v1/sign", body, Some(token))
    }

    fn post_raw(
        &self,
        path: &str,
        body: &[u8],
        token: Option<&str>,
    ) -> Result<Vec<(String, u16, Value)>> {
        let root_der = self.pki.certificate_der("gateway-ca")?;
        let chain_der = self.pki.certificate_der("gateway")?;
        let key_pem = fs::read(self.pki.path("gateway.key"))?;
        let root = ureq::tls::Certificate::from_der(&root_der).to_owned();
        let chain = ureq::tls::Certificate::from_der(&chain_der).to_owned();
        let key = ureq::tls::PrivateKey::from_pem(&key_pem)?;
        let agent: ureq::Agent = ureq::Agent::config_builder()
            .tls_config(
                ureq::tls::TlsConfig::builder()
                    .provider(ureq::tls::TlsProvider::Rustls)
                    .root_certs(ureq::tls::RootCerts::new_with_certs(&[root]))
                    .client_cert(Some(ureq::tls::ClientCert::new_with_certs(&[chain], key)))
                    .build(),
            )
            .timeout_global(Some(Duration::from_secs(120)))
            .http_status_as_error(false)
            .build()
            .into();
        let targets: Vec<(String, SocketAddr)> = self
            .nodes
            .iter()
            .filter(|(id, _)| path != "/v1/sign" || SIGNERS.contains(&id.as_str()))
            .cloned()
            .collect();
        thread::scope(|scope| {
            let handles: Vec<_> = targets
                .iter()
                .map(|(id, address)| {
                    let agent = agent.clone();
                    scope.spawn(
                        move || -> std::result::Result<(String, u16, Value), String> {
                            let request = agent
                                .post(format!("https://{address}{path}"))
                                .header("content-type", "application/json");
                            let request = if let Some(token) = token {
                                request.header("authorization", format!("Bearer {token}"))
                            } else {
                                request
                            };
                            let mut response = request
                                .send(body)
                                .map_err(|error| format!("{id}: {error}"))?;
                            let status = response.status().as_u16();
                            let text = response
                                .body_mut()
                                .read_to_string()
                                .map_err(|error| format!("{id}: {error}"))?;
                            let value = serde_json::from_str(&text)
                                .map_err(|error| format!("{id}: {error}: {text}"))?;
                            Ok((id.clone(), status, value))
                        },
                    )
                })
                .collect();
            handles
                .into_iter()
                .map(|handle| {
                    handle
                        .join()
                        .map_err(|_| "raw sign thread panicked".to_owned())?
                        .map_err(Into::into)
                })
                .collect()
        })
    }

    fn audit(&self, client: &AttestorClient) -> Result<Vec<u64>> {
        SIGNERS
            .iter()
            .map(|id| Ok(checked(client.health(id))?.audit_sequence))
            .collect()
    }
}

fn build_daemon(root: &Path) -> Result<PathBuf> {
    let source = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../services/wallet/attestor");
    let binary = root.join("attestor");
    let output = Command::new("go")
        .current_dir(&source)
        .arg("build")
        .arg("-o")
        .arg(&binary)
        .arg("./cmd/attestor")
        .stdin(Stdio::null())
        .output()?;
    if !output.status.success() {
        return Err(format!(
            "attestor build failed: {}",
            String::from_utf8_lossy(&output.stderr)
        )
        .into());
    }
    Ok(binary)
}

fn activity_registry() -> Result<layerx_types::payload::ModuleRegistry> {
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    checked(ModuleRegistry::new(&[
        checked(ModuleRegistration::new(
            ModuleId::Asset,
            &[
                checked(ActivityType::new(ModuleId::Asset, 1))?,
                checked(ActivityType::new(ModuleId::Asset, 4))?,
                checked(ActivityType::new(ModuleId::Asset, 5))?,
            ],
        ))?,
        checked(ModuleRegistration::new(
            ModuleId::Budget,
            &[checked(ActivityType::new(ModuleId::Budget, 1))?],
        ))?,
        checked(ModuleRegistration::new(
            ModuleId::Governance,
            &[
                checked(ActivityType::new(ModuleId::Governance, 1))?,
                checked(ActivityType::new(ModuleId::Governance, 2))?,
                checked(ActivityType::new(ModuleId::Governance, 3))?,
                checked(ActivityType::new(ModuleId::Governance, 5))?,
                checked(ActivityType::new(ModuleId::Governance, 8))?,
            ],
        ))?,
        checked(ModuleRegistration::new(
            ModuleId::Programs,
            &[checked(ActivityType::new(ModuleId::Programs, 5))?],
        ))?,
    ]))
}

fn actor(public: [u8; 32]) -> String {
    format!("did:layerx:{}", hex(&public))
}

fn main_account(did: &str) -> Result<[u8; 32]> {
    let account = checked(layerx_types::account::AccountId::parse(&format!(
        "agent:{did}:main"
    )))?;
    checked(layerx_intents::canonical::account_id_for_protocol(
        &account, 3,
    ))
}

fn transfer(public: [u8; 32]) -> Result<layerx_types::payload::Payload> {
    use layerx_crypto::payments::{Payment, TransferLeg};
    use layerx_types::payload::{ActivityType, ModuleId, Payload};
    let did = actor(public);
    let transfer = Payment::ProgramTransfer {
        program: [21; 32],
        legs: vec![TransferLeg {
            from: main_account(&did)?,
            asset: [0; 32],
            to: main_account(&actor([0x52; 32]))?,
            amount: TRANSFER,
        }],
    };
    checked(Payload::new(
        &activity_registry()?,
        checked(ActivityType::new(ModuleId::Programs, 5))?,
        &checked(transfer.encode(did.as_bytes()))?,
    ))
}

fn activity(
    seed: [u8; 32],
    from: [u8; 20],
    fee: u128,
) -> Result<(layerx_crypto::evm_transaction::Eip1559Call, Disclosure)> {
    use layerx_crypto::send::{ledger_payment_call, CallOptions, SendDebit};
    use ring::signature::KeyPair as _;
    let key = ring::signature::Ed25519KeyPair::from_seed_unchecked(&seed)
        .map_err(|_| "invalid test signing seed")?;
    let public: [u8; 32] = key.public_key().as_ref().try_into()?;
    let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs();
    let debit = SendDebit {
        from: main_account(&actor(public))?,
        to: main_account(&actor([0x52; 32]))?,
        asset: [0; 32],
        amount: TRANSFER,
        source_sequence: PENDING_NONCE,
        idempotency_key: [4; 32],
        expires_at: now + 600,
        context_hash: [5; 32],
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: NETWORK,
        protocol_version: 3,
    };
    let message = checked(debit.authorization_message())?;
    let signature: [u8; 64] = key.sign(&preimage(&message)).as_ref().try_into()?;
    let payload = checked(debit.encode_signed(public, signature))?;
    let disclosed = checked(ledger_payment_call(
        &payload,
        &CallOptions {
            from,
            chain_id: u64::from(NETWORK),
            nonce: PENDING_NONCE,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: fee,
            gas_limit: 100_000,
        },
    ))?;
    let call = disclosed.call;
    let unsigned = layerx_human_kms::attestor::unsigned_evm_transaction(&call);
    assert_eq!(Keccak256::digest(&unsigned).as_slice(), call.signing_hash());
    assert_eq!(
        checked(layerx_human_kms::attestor::decode_unsigned_evm_transaction(
            &unsigned
        ))?,
        call
    );
    let disclosure = checked(bind_call(&call, from))?;
    assert_eq!(disclosure, disclosed.disclosure);
    Ok((call, disclosure))
}

fn assert_verified(
    signer: &AttestorSigner,
    call: &layerx_crypto::evm_transaction::Eip1559Call,
    signature: &AttestorCallSignature,
) -> Result<()> {
    let unsigned = layerx_human_kms::attestor::unsigned_evm_transaction(call);
    let expected: [u8; 32] = Keccak256::digest(&unsigned).into();
    assert_eq!(signature.digest(), &expected);
    assert_eq!(signature.digest(), &call.signing_hash());
    assert_eq!(signature.signature().len(), 65);
    let recovered = checked(layerx_crypto::signer::AgentSignature::new(
        *signature.signature(),
    ))?;
    assert_eq!(recover_sender(call, &recovered)?, signer.evm_address());
    let mut tampered = *signature.signature();
    tampered[0] ^= 1;
    let invalid = layerx_crypto::signer::AgentSignature::new(tampered).and_then(|signature| {
        recover_sender(call, &signature).map_err(|_| layerx_crypto::VerifyError::BadSignature)
    });
    assert!(invalid.is_err() || invalid? != signer.evm_address());
    Ok(())
}

/// Plays the approval boundary the way the Human service `sign_now` does: the stored key
/// owner, a fresh request session and the approved call's payload expiry.
fn sign_approved(
    signer: &AttestorSigner,
    call: &layerx_crypto::evm_transaction::Eip1559Call,
    disclosure: &Disclosure,
    from: [u8; 20],
    assertion: &str,
) -> Result<std::result::Result<AttestorCallSignature, AttestorError>> {
    let session = checked(new_session_id("activity"))?;
    let approval = Approval {
        principal: OWNER,
        session_id: &session,
        expires_at: disclosure
            .expiry
            .not_after
            .min(disclosure.expiry.payload_expires_at),
    };
    Ok(signer.sign_activity(call, disclosure, from, &approval, assertion))
}

#[test]
fn attestor_signs_a_disclosed_activity_through_the_real_daemon_quorum() -> Result<()> {
    let cluster = Cluster::start()?;
    let denied = cluster
        .operator()?
        .generate_ed25519("lx-operator-key", OWNER, ACCOUNT);
    assert!(
        matches!(
            &denied,
            Err(AttestorError::Refused { status: 403, code, .. }) if code == "operator_required"
        ),
        "{denied:?}"
    );
    let generated = checked(
        cluster
            .client()?
            .generate_ed25519("lx-user-key", OWNER, ACCOUNT),
    )?;
    assert_eq!(generated.audit.len(), NODES);
    assert!(generated.audit.values().all(|sequence| *sequence > 0));
    let generated_evm = serde_json::to_vec(&json!({
        "session_id": "generate-evm-owner", "key_id": EVM_KEY,
        "curve": "secp256k1", "owner": OWNER, "account": "",
    }))?;
    let mut wallet_public = None;
    for (node, status, body) in cluster.post_raw("/v1/keys/generate", &generated_evm, None)? {
        assert_eq!(status, 200, "{node}: {body}");
        let encoded = body["public_key"]
            .as_str()
            .ok_or("missing EVM public key")?;
        let public = (0..encoded.len())
            .step_by(2)
            .map(|offset| u8::from_str_radix(&encoded[offset..offset + 2], 16))
            .collect::<std::result::Result<Vec<_>, _>>()?;
        if let Some(agreed) = &wallet_public {
            assert_eq!(agreed, &public);
        } else {
            wallet_public = Some(public);
        }
    }
    let public = wallet_public.ok_or("missing EVM key generation responses")?;
    let point = k256::ecdsa::VerifyingKey::from_sec1_bytes(&public)?.to_encoded_point(false);
    let wallet: [u8; 20] = Keccak256::digest(&point.as_bytes()[1..])[12..].try_into()?;
    cluster.pki.certificate_der("gateway-ca")?;
    cluster.pki.certificate_der("gateway")?;
    cluster.pki.key_der("gateway")?;
    let config = serde_json::from_value(json!({
        "nodes": cluster.nodes.iter().map(|(id, address)| json!({"id":id,"address":address})).collect::<Vec<_>>(),
        "trust_anchors": [cluster.pki.path("gateway-ca.der")],
        "client_certificate_chain": [cluster.pki.path("gateway.der")],
        "client_private_key_pkcs8": cluster.pki.path("gateway.pk8"),
        "key_id": "lx-user-key", "public_key": generated.public_key,
        "evm_key_id": EVM_KEY, "evm_address": wallet,
        "signers": SIGNERS, "network_id": NETWORK, "timeout_seconds": 60,
    }))?;
    let signer = checked(AttestorSigner::from_config(&config))?;
    let observer = cluster.client()?;
    let assertion = cluster.tokens.mint(OWNER)?;

    let (canonical, disclosure) = activity(generated.public_key, wallet, 1)?;
    let first = checked(sign_approved(
        &signer,
        &canonical,
        &disclosure,
        wallet,
        &assertion,
    )?)?;
    assert_verified(&signer, &canonical, &first)?;
    assert_eq!(
        first.audit().keys().map(String::as_str).collect::<Vec<_>>(),
        SIGNERS
    );
    assert_eq!(&signer.audit_sequences(), first.audit());
    for (node, sequence) in first.audit() {
        assert!(*sequence > generated.audit[node]);
    }

    let before = cluster.audit(&observer)?;
    let (other_canonical, other_disclosure) = activity(generated.public_key, wallet, 2)?;
    assert_eq!(
        sign_approved(&signer, &canonical, &other_disclosure, wallet, &assertion)?,
        Err(AttestorError::Disclosure(SignError::DisclosureMismatch(
            "fee_limit"
        )))
    );
    let mut altered = disclosure.clone();
    altered.amounts[0].value = altered.amounts[0]
        .value
        .checked_add(1)
        .ok_or("amount overflow")?;
    assert_eq!(
        sign_approved(&signer, &canonical, &altered, wallet, &assertion)?,
        Err(AttestorError::Disclosure(SignError::DisclosureMismatch(
            "amounts"
        )))
    );
    let mut foreign = canonical.clone();
    foreign.chain_id += 1;
    let foreign_disclosure = checked(bind_call(&foreign, wallet))?;
    assert_eq!(
        sign_approved(&signer, &foreign, &foreign_disclosure, wallet, &assertion)?,
        Err(AttestorError::WrongNetwork {
            expected: NETWORK,
            actual: NETWORK + 1
        })
    );
    let other_wallet = [0x55; 20];
    let other_sender = checked(bind_call(&canonical, other_wallet))?;
    assert_eq!(
        sign_approved(&signer, &canonical, &other_sender, other_wallet, &assertion)?,
        Err(AttestorError::Disclosure(SignError::DisclosureMismatch(
            "from"
        )))
    );
    assert_eq!(cluster.audit(&observer)?, before);
    assert_eq!(&signer.audit_sequences(), first.audit());

    let refused = sign_approved(
        &signer,
        &other_canonical,
        &other_disclosure,
        wallet,
        &cluster.tokens.mint(STRANGER)?,
    )?;
    assert!(
        matches!(
            &refused,
            Err(AttestorError::Refused { status: 401, code, .. }) if code == "token_not_owner"
        ),
        "{refused:?}"
    );
    assert_eq!(&signer.audit_sequences(), first.audit());

    let shared = checked(sign_approved(
        &signer,
        &other_canonical,
        &other_disclosure,
        wallet,
        &assertion,
    )?)?;
    assert_verified(&signer, &other_canonical, &shared)?;
    for (node, sequence) in shared.audit() {
        assert!(*sequence > first.audit()[node]);
    }
    assert_eq!(&signer.audit_sequences(), shared.audit());

    let (repeat_canonical, repeat_disclosure) = activity(generated.public_key, wallet, 3)?;
    let repeat_session = "activity-repeated-under-one-token";
    let repeat = serde_json::to_vec(&json!({
        "session_id": repeat_session,
        "key_id": EVM_KEY,
        "kind": "evm_tx",
        "signers": SIGNERS,
        "transaction": hex(&layerx_human_kms::attestor::unsigned_evm_transaction(&repeat_canonical)),
    }))?;
    assert_eq!(
        repeat_disclosure.signing_digest()?,
        repeat_canonical.signing_hash()
    );
    for (node, status, body) in cluster.sign_raw(&repeat, &assertion)? {
        assert_eq!(status, 200, "{node}: {body}");
    }
    let before = cluster.audit(&observer)?;
    for (node, status, body) in cluster.sign_raw(&repeat, &assertion)? {
        assert_eq!(status, 401, "{node}: {body}");
        assert_eq!(body["error"]["code"], "token_invalid", "{node}: {body}");
    }
    for (index, sequence) in cluster.audit(&observer)?.iter().enumerate() {
        assert!(*sequence > before[index]);
    }

    let second = checked(sign_approved(
        &signer,
        &other_canonical,
        &other_disclosure,
        wallet,
        &cluster.tokens.mint(OWNER)?,
    )?)?;
    assert_verified(&signer, &other_canonical, &second)?;
    for (node, sequence) in second.audit() {
        assert!(*sequence > shared.audit()[node]);
    }
    assert_eq!(&signer.audit_sequences(), second.audit());
    Ok(())
}

struct ClockAuthority {
    socket: PathBuf,
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::io::Result<()>>>,
}

impl ClockAuthority {
    fn start() -> Result<Self> {
        let mut server = runtime_clock_server::Server::bind(&std::env::temp_dir())?;
        let socket = server.socket().to_owned();
        let shutdown = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&shutdown);
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                server.poll()?;
                std::thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        Ok(Self {
            socket,
            shutdown,
            worker: Some(worker),
        })
    }
}

impl Drop for ClockAuthority {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

struct SendFixture {
    child: Option<Child>,
    clock: ClockAuthority,
    cluster: attestor_cluster::Cluster,
    root: PathBuf,
    address: SocketAddr,
    wallet: [u8; 20],
    reference: Vec<u8>,
}

impl Drop for SendFixture {
    fn drop(&mut self) {
        self.stop();
        let _ = fs::remove_dir_all(&self.root);
    }
}

impl SendFixture {
    fn new() -> Result<Self> {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let snapshot = registry();
        let mut cluster = attestor_cluster::Cluster::start(&[])?;
        let public = cluster.generate_key(
            "ed25519-owner",
            "ed25519",
            OWNER,
            "0x1111111111111111111111111111111111111111",
        )?;
        let evm_public = cluster.generate_key(EVM_KEY, "secp256k1", OWNER, "")?;
        let point =
            k256::ecdsa::VerifyingKey::from_sec1_bytes(&evm_public)?.to_encoded_point(false);
        let wallet = Keccak256::digest(&point.as_bytes()[1..])[12..].try_into()?;
        cluster.pki.leaf("node-ca", "kms", true)?;
        cluster.pki.leaf("gateway-ca", "service", false)?;
        for name in ["node-ca", "gateway-ca", "gateway", "kms", "service"] {
            cluster.pki.certificate_der(name)?;
        }
        for name in ["gateway", "kms", "service"] {
            cluster.pki.key_der(name)?;
            fs::set_permissions(
                cluster.pki.path(&format!("{name}.pk8")),
                fs::Permissions::from_mode(0o600),
            )?;
        }
        let root = scratch("lxkp-send-authorization")?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let mut seal = Zeroizing::new([0; 32]);
        getrandom::fill(&mut *seal)?;
        fs::write(root.join("seal"), &*seal)?;
        fs::write(root.join("snapshot.json"), serde_json::to_vec(&snapshot)?)?;
        fs::write(root.join("assertion"), cluster.tokens.mint(OWNER)?)?;
        let nodes: Vec<_> = cluster
            .nodes
            .iter()
            .map(|(id, address)| json!({"id":id,"address":address}))
            .collect();
        fs::write(
            root.join("attestor.json"),
            serde_json::to_vec(&json!({
                "nodes": nodes,
                "trust_anchors": [cluster.pki.path("node-ca.der")],
                "client_certificate_chain": [cluster.pki.path("gateway.der")],
                "client_private_key_pkcs8": cluster.pki.path("gateway.pk8"),
                "key_id": "ed25519-owner", "public_key": public,
                "evm_key_id": EVM_KEY, "evm_address": wallet,
                "signers": attestor_cluster::SIGNERS, "network_id":snapshot.chain_id, "timeout_seconds":60
            }))?,
        )?;
        for entry in fs::read_dir(&root)? {
            fs::set_permissions(entry?.path(), fs::Permissions::from_mode(0o600))?;
        }
        let mut fixture = Self {
            child: None,
            clock: ClockAuthority::start()?,
            cluster,
            root,
            address,
            wallet,
            reference: Vec::new(),
        };
        drop(listener);
        fixture.start()?;
        let description = fixture.call(1, 1, &[], None)?;
        let length = usize::try_from(u32::from_be_bytes(description[..4].try_into()?))?;
        assert_eq!(length, 32);
        fixture.reference = description[4..4 + length].to_vec();
        assert_eq!(&description[4 + length + 32..4 + length + 64], BINDING);
        assert_eq!(
            fixture.call(3, 6, &fixture.reference, None)?,
            fixture.wallet
        );
        Ok(fixture)
    }

    fn start(&mut self) -> Result<()> {
        let log = fs::File::create(self.root.join("kms.log"))?;
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-human-kms"));
        command
            .env("LAYERX_HUMAN_KMS_LISTEN", self.address.to_string())
            .env("LAYERX_HUMAN_KMS_PROVIDER_REFERENCE", PROVIDER)
            .env("LAYERX_HUMAN_KMS_STATE_DIR", self.root.join("state"))
            .env("LAYERX_HUMAN_KMS_DEADLINE_SECONDS", "60");
        for (suffix, path) in [
            ("REGISTRY_FILE", self.root.join("snapshot.json")),
            ("SEAL_SECRET_FILE", self.root.join("seal")),
            ("ATTESTOR_CONFIG_FILE", self.root.join("attestor.json")),
            ("EVM_ASSERTION_FILE", self.root.join("assertion")),
            ("CLIENT_CA_DER", self.cluster.pki.path("gateway-ca.der")),
            ("TLS_CERT_DER", self.cluster.pki.path("kms.der")),
            ("TLS_KEY_DER", self.cluster.pki.path("kms.pk8")),
            ("CLIENT_CERT_DER", self.cluster.pki.path("service.der")),
        ] {
            command.env(format!("LAYERX_HUMAN_KMS_{suffix}"), path);
        }
        command.env_remove("LAYERX_HUMAN_KMS_EVM_CLIENT_CERT_DER");
        command
            .env("LAYERX_RUNTIME_CLOCK_SOCKET", &self.clock.socket)
            .env("LAYERX_RUNTIME_CLOCK_PID", std::process::id().to_string())
            .env(
                "LAYERX_RUNTIME_CLOCK_UID",
                rustix::process::geteuid().as_raw().to_string(),
            );
        self.child = Some(
            command
                .stdin(Stdio::null())
                .stdout(log.try_clone()?)
                .stderr(log)
                .spawn()?,
        );
        let started = Instant::now();
        loop {
            if self.call(1, 0, &[], None).is_ok() {
                return Ok(());
            }
            if self
                .child
                .as_mut()
                .ok_or("missing KMS process")?
                .try_wait()?
                .is_some()
                || started.elapsed() > Duration::from_secs(10)
            {
                return Err(format!(
                    "KMS startup failed: {}",
                    fs::read_to_string(self.root.join("kms.log"))?
                )
                .into());
            }
            std::thread::sleep(Duration::from_millis(20));
        }
    }

    fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }

    fn exchange(
        &self,
        version: u16,
        operation: u8,
        reference: &[u8],
        payload: Option<&[u8]>,
    ) -> Result<Vec<u8>> {
        let mut roots = RootCertStore::empty();
        roots.add(CertificateDer::from(fs::read(
            self.cluster.pki.path("node-ca.der"),
        )?))?;
        let config = ClientConfig::builder()
            .with_root_certificates(roots)
            .with_client_auth_cert(
                vec![CertificateDer::from(fs::read(
                    self.cluster.pki.path("service.der"),
                )?)],
                PrivateKeyDer::try_from(fs::read(self.cluster.pki.path("service.pk8"))?)?,
            )?;
        let tcp = TcpStream::connect(self.address)?;
        tcp.set_read_timeout(Some(Duration::from_secs(60)))?;
        tcp.set_write_timeout(Some(Duration::from_secs(60)))?;
        let mut tls = StreamOwned::new(
            ClientConnection::new(
                Arc::new(config),
                ServerName::IpAddress(self.address.ip().into()),
            )?,
            tcp,
        );
        let mut request = b"LXKP".to_vec();
        request.extend(version.to_be_bytes());
        request.push(operation);
        blob(&mut request, PROVIDER.as_bytes())?;
        if operation != 0 {
            request.extend(BINDING);
            request.extend(125_u32.to_be_bytes());
            request.push(1);
            blob(&mut request, reference)?;
        }
        if let Some(payload) = payload {
            blob(&mut request, payload)?;
        }
        checked(write_frame(&mut tls, &request, MAX_FRAME))?;
        let response = checked(read_frame(&mut tls, MAX_FRAME))?;
        assert_eq!(&response[..7], &request[..7]);
        Ok(response)
    }

    fn call(
        &self,
        version: u16,
        operation: u8,
        reference: &[u8],
        payload: Option<&[u8]>,
    ) -> Result<Vec<u8>> {
        let response = self.exchange(version, operation, reference, payload)?;
        if response[7] != 0 {
            return Err(format!(
                "LXKP operation {operation} refused with status {}",
                response[7]
            )
            .into());
        }
        Ok(response[8..].to_vec())
    }
}

fn blob(out: &mut Vec<u8>, bytes: &[u8]) -> Result<()> {
    out.extend(u32::try_from(bytes.len())?.to_be_bytes());
    out.extend(bytes);
    Ok(())
}

fn authorization(fixture: &SendFixture) -> Result<Value> {
    let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs();
    let recipient = [0x44; 20];
    let mut amount = [0; 32];
    amount[16..].copy_from_slice(&400_000_u128.to_be_bytes());
    let denom = "apax";
    let mut bank = [0; 20];
    bank[18..].copy_from_slice(&[0x10, 0x01]);
    let mut data =
        layerx_crypto::disclosure::call::ledger_selector("send(address,address,string,uint256)")
            .to_vec();
    for address in [fixture.wallet, recipient] {
        data.extend([0; 12]);
        data.extend(address);
    }
    let mut offset = [0; 32];
    offset[31] = 128;
    data.extend(offset);
    data.extend(amount);
    let mut length = [0; 32];
    length[31] = 4;
    data.extend(length);
    data.extend(denom.as_bytes());
    data.extend([0; 28]);
    Ok(json!({
        "authorization": {
            "plan_id": ([1; 32]), "action_key": ACTION,
            "tenant": "tenant", "principal": OWNER,
            "binding_digest": BINDING, "wallet": fixture.wallet,
            "not_before": now - 5, "not_after": now + 300,
            "transaction": {
                "chain_id": registry().chain_id, "nonce": PENDING_NONCE,
                "max_priority_fee_per_gas": 1, "max_fee_per_gas": 2,
                "gas_limit": 100_000, "to": bank, "value": ([0; 32]),
                "calldata": data,
            }
        },
        "disclosure": { "recipient": recipient, "denom": denom, "amount": amount }
    }))
}

fn native_value_transaction(value: u64) -> Vec<u8> {
    let amount = value.to_be_bytes();
    let amount = &amount[amount.iter().take_while(|byte| **byte == 0).count()..];
    let mut fields = vec![0x7d, 0x80, 0x01, 0x02, 0x82, 0x52, 0x08, 0x94];
    fields.extend([0x44; 20]);
    fields.push(0x80 + u8::try_from(amount.len()).expect("u64 RLP width"));
    fields.extend(amount);
    fields.extend([0x80, 0xc0]);
    let mut raw = vec![
        2,
        0xc0 + u8::try_from(fields.len()).expect("short native transfer"),
    ];
    raw.extend(fields);
    raw
}

#[test]
fn attestor_signs_a_send_authorization_then_the_completed_send() -> Result<()> {
    let mut fixture = SendFixture::new()?;
    let request = authorization(&fixture)?;
    let payload = serde_json::to_vec(&request)?;
    let client = fixture.cluster.client()?;
    let before = fixture.cluster.audit(&client)?;
    let raw = fixture.call(3, 11, &fixture.reference, Some(&payload))?;
    assert_eq!(raw.first(), Some(&2));
    let (call, signature) = decode_signed(&raw)?;
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, PENDING_NONCE);
    assert_eq!(call.max_priority_fee_per_gas, 1);
    assert_eq!(call.max_fee_per_gas, 2);
    assert_eq!(call.gas_limit, 100_000);
    assert_eq!(
        layerx_crypto::evm_transaction::assemble(&call, &signature),
        raw
    );
    assert_eq!(
        json!(call.to),
        request["authorization"]["transaction"]["to"]
    );
    assert_eq!(
        json!(call.data),
        request["authorization"]["transaction"]["calldata"]
    );
    assert_eq!(recover_sender(&call, &signature)?, fixture.wallet);
    let disclosure = checked(bind_call(&call, fixture.wallet))?;
    let method = checked(disclosure.construct_method())?;
    let Some(layerx_crypto::disclosure::call::ConstructMethod::BankSend {
        from,
        to,
        denom,
        amount,
    }) = method
    else {
        return Err("signed transaction is not a bank transfer".into());
    };
    assert_eq!(from, fixture.wallet);
    assert_eq!(json!(to), request["disclosure"]["recipient"]);
    assert_eq!(json!(denom), request["disclosure"]["denom"]);
    assert_eq!(json!(amount), request["disclosure"]["amount"]);
    let after = fixture.cluster.audit(&client)?;
    assert_eq!(fixture.cluster.nodes.len(), NODES);
    assert_eq!(before.len(), SIGNERS.len());
    assert_eq!(after.len(), before.len());
    for (previous, current) in before.iter().zip(&after) {
        assert!(
            current > previous,
            "every attestor must record the EVM signing"
        );
    }
    assert_eq!(
        fixture.call(3, 11, &fixture.reference, Some(&payload))?,
        raw
    );
    assert_eq!(fixture.cluster.audit(&client)?, after);
    let persisted: Value = serde_json::from_slice(&fixture.call(
        3,
        10,
        &fixture.reference,
        Some(&serde_json::to_vec(&ACTION)?),
    )?)?;
    assert_eq!(persisted["raw_transaction"], json!(raw));
    assert_eq!(persisted["transaction_hash"], json!(transaction_hash(&raw)));
    fixture.stop();
    fixture.start()?;
    assert_eq!(
        fixture.call(3, 11, &fixture.reference, Some(&payload))?,
        raw
    );
    assert_eq!(fixture.cluster.audit(&client)?, after);
    let mut conflict = request.clone();
    conflict["authorization"]["transaction"]["nonce"] = json!(1);
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&conflict)?)
        )?,
        b"LXKP\0\x03\x0b\x03"
    );
    let mut foreign = request.clone();
    foreign["authorization"]["action_key"] = json!(([8; 32]));
    foreign["authorization"]["wallet"] = json!(([0x61; 20]));
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&foreign)?)
        )?,
        b"LXKP\0\x03\x0b\x01"
    );
    assert_eq!(fixture.cluster.audit(&client)?, after);

    let config = serde_json::from_slice(&fs::read(fixture.root.join("attestor.json"))?)?;
    let mut signer = checked(layerx_human_kms::attestor::AttestorSigner::from_config(
        &config,
    ))?;
    checked(signer.set_evm_assertion(&fs::read_to_string(fixture.root.join("assertion"))?))?;
    let unsigned = layerx_human_kms::attestor::unsigned_evm_transaction(&call);
    let mut signature = checked(signer.sign_evm_transaction(&unsigned, call.chain_id))?;
    signature[64] += 27;
    let signature = checked(layerx_crypto::signer::AgentSignature::new(signature))?;
    assert_eq!(recover_sender(&call, &signature)?, fixture.wallet);
    let assembled = layerx_crypto::evm_transaction::assemble(&call, &signature);
    assert_eq!(decode_signed(&assembled)?.0, call);
    let first_audit = signer.audit_sequences();
    assert_eq!(
        first_audit.keys().map(String::as_str).collect::<Vec<_>>(),
        SIGNERS
    );
    assert!(first_audit.values().all(|sequence| *sequence > 0));
    let first_assertion = fs::read_to_string(fixture.root.join("assertion"))?;
    std::thread::sleep(Duration::from_secs(1));
    let second_assertion = fixture.cluster.tokens.mint(OWNER)?;
    assert_ne!(first_assertion, second_assertion);
    checked(signer.set_evm_assertion(&second_assertion))?;
    let mut next = call.clone();
    next.nonce += 1;
    let unsigned = layerx_human_kms::attestor::unsigned_evm_transaction(&next);
    let mut signature = checked(signer.sign_evm_transaction(&unsigned, next.chain_id))?;
    signature[64] += 27;
    let signature = checked(layerx_crypto::signer::AgentSignature::new(signature))?;
    assert_eq!(recover_sender(&next, &signature)?, fixture.wallet);
    for (node, sequence) in signer.audit_sequences() {
        assert!(sequence > first_audit[&node]);
    }
    checked(signer.set_evm_assertion(&fixture.cluster.tokens.mint(STRANGER)?))?;
    let refused = signer.sign_evm_transaction(&unsigned, next.chain_id);
    assert!(
        matches!(&refused, Err(AttestorError::Refused {status: 401, code, ..}) if code == "token_not_owner"),
        "{refused:?}"
    );
    checked(signer.set_evm_assertion(&fixture.cluster.tokens.mint(OWNER)?))?;
    // The EVM policy caps native transaction value at 10^18; exercise its next value.
    let over = native_value_transaction(1_000_000_000_000_000_001);
    let refused = signer.sign_evm_transaction(&over, call.chain_id);
    assert!(
        matches!(&refused, Err(error) if error.refusal_code() == Some("value_cap")),
        "{refused:?}"
    );
    assert!(refused
        .as_ref()
        .err()
        .is_some_and(|error| error.to_string().contains("value_cap")));
    Ok(())
}

#[test]
fn attestor_refuses_a_mismatched_disclosure_before_contacting_any_node() -> Result<()> {
    refuses_mismatched_activity_disclosure_offline()?;
    let fixture = SendFixture::new()?;
    let request = authorization(&fixture)?;
    let client = fixture.cluster.client()?;
    let before = fixture.cluster.audit(&client)?;
    let mut mismatch = request.clone();
    mismatch["disclosure"]["recipient"] = json!(([0x55; 20]));
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&mismatch)?)
        )?,
        b"LXKP\0\x03\x0b\x01"
    );
    mismatch = request.clone();
    mismatch["disclosure"]["amount"][31] = json!(43);
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&mismatch)?)
        )?,
        b"LXKP\0\x03\x0b\x01"
    );
    mismatch = request.clone();
    mismatch["disclosure"]["denom"] = json!("other");
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&mismatch)?)
        )?,
        b"LXKP\0\x03\x0b\x01"
    );
    mismatch = request.clone();
    mismatch["authorization"]["wallet"] = json!(([0x61; 20]));
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&mismatch)?)
        )?,
        b"LXKP\0\x03\x0b\x01"
    );
    mismatch = request.clone();
    mismatch["authorization"]["transaction"]["max_priority_fee_per_gas"] = json!(3);
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&mismatch)?)
        )?,
        b"LXKP\0\x03\x0b\x01"
    );
    mismatch = request.clone();
    mismatch["authorization"]["transaction"]["chain_id"] = json!(126);
    assert_eq!(
        fixture.exchange(
            3,
            11,
            &fixture.reference,
            Some(&serde_json::to_vec(&mismatch)?)
        )?,
        b"LXKP\0\x03\x0b\x01"
    );
    assert!(AttestorSigner::new(
        fixture.cluster.gateway()?,
        "ed25519-owner",
        [9; 32],
        &["node-1", "node-2"],
        NETWORK
    )
    .is_err());
    assert_eq!(fixture.cluster.audit(&client)?, before);
    assert_eq!(
        fixture.exchange(
            3,
            10,
            &fixture.reference,
            Some(&serde_json::to_vec(&ACTION)?)
        )?,
        b"LXKP\0\x03\x0a\x02"
    );
    Ok(())
}

fn refuses_mismatched_activity_disclosure_offline() -> Result<()> {
    let root = scratch("attestor-offline")?;
    let outcome = (|| -> Result<()> {
        let mut pki = Pki::new(&root);
        pki.authority("gateway-ca")?;
        pki.leaf("gateway-ca", "gateway", false)?;
        let nodes: Vec<(String, SocketAddr)> = free_addresses(NODES)?
            .into_iter()
            .enumerate()
            .map(|(index, address)| (format!("node-{}", index + 1), address))
            .collect();
        let wallet = [0x42; 20];
        let public = [9; 32];
        pki.certificate_der("gateway-ca")?;
        pki.certificate_der("gateway")?;
        pki.key_der("gateway")?;
        let mut config: layerx_human_kms::attestor::AttestorSignerConfig = serde_json::from_value(
            json!({
                "nodes": nodes.iter().map(|(id, address)| json!({"id": id, "address": address})).collect::<Vec<_>>(),
                "trust_anchors": [pki.path("gateway-ca.der")],
                "client_certificate_chain": [pki.path("gateway.der")],
                "client_private_key_pkcs8": pki.path("gateway.pk8"),
                "key_id": "lx-user-key", "public_key": public,
                "evm_key_id": EVM_KEY, "evm_address": wallet,
                "signers": SIGNERS, "network_id": NETWORK, "timeout_seconds": 1,
            }),
        )?;
        let signer = checked(AttestorSigner::from_config(&config))?;
        let (canonical, disclosure) = activity(public, wallet, 1)?;
        let (_, other) = activity(public, wallet, 2)?;
        assert_eq!(
            sign_approved(&signer, &canonical, &other, wallet, "assertion")?,
            Err(AttestorError::Disclosure(SignError::DisclosureMismatch(
                "fee_limit"
            )))
        );
        let mut altered = disclosure.clone();
        altered.actor = b"did:layerx:mallory".to_vec();
        assert_eq!(
            sign_approved(&signer, &canonical, &altered, wallet, "assertion")?,
            Err(AttestorError::Disclosure(SignError::DisclosureMismatch(
                "actor"
            )))
        );
        assert!(matches!(
            sign_approved(&signer, &canonical, &disclosure, wallet, "assertion")?,
            Err(AttestorError::Unavailable { .. })
        ));
        assert!(signer.audit_sequences().is_empty());
        config.network_id = NETWORK + 1;
        let foreign = checked(AttestorSigner::from_config(&config))?;
        assert_eq!(
            sign_approved(&foreign, &canonical, &disclosure, wallet, "assertion")?,
            Err(AttestorError::WrongNetwork {
                expected: NETWORK + 1,
                actual: NETWORK
            })
        );
        assert!(AttestorSigner::new(
            gateway_client(&pki, &nodes)?,
            "lx-user-key",
            public,
            &["node-1", "node-2"],
            NETWORK,
        )
        .is_err());
        Ok(())
    })();
    let _ = fs::remove_dir_all(&root);
    outcome
}
