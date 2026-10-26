use base64::engine::general_purpose::{STANDARD, URL_SAFE_NO_PAD};
use base64::Engine as _;
use layerx_human_kms::attestor::AttestorClient;
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

pub type Result<T> = std::result::Result<T, Box<dyn Error>>;

pub const CHAIN_ID: &str = "125";
pub const ISSUER: &str = "attestor-test-issuer";
pub const AUDIENCE: &str = "attestor-test-audience";
pub const NODES: usize = 5;
pub const SIGNERS: [&str; 3] = ["node-1", "node-3", "node-5"];

static SCRATCH: AtomicU64 = AtomicU64::new(0);

pub fn checked<T, E: std::fmt::Debug>(value: std::result::Result<T, E>) -> Result<T> {
    value.map_err(|error| format!("{error:?}").into())
}

pub fn scratch(label: &str) -> Result<PathBuf> {
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

pub fn openssl(dir: &Path, args: &[&str]) -> Result<()> {
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

pub fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

pub struct Pki {
    dir: PathBuf,
    serial: u64,
}

impl Pki {
    pub fn new(dir: &Path) -> Self {
        Self {
            dir: dir.to_path_buf(),
            serial: 1,
        }
    }

    pub fn authority(&self, name: &str) -> Result<()> {
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

    pub fn leaf(&mut self, authority: &str, name: &str, server: bool) -> Result<()> {
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

    pub fn certificate_der(&self, name: &str) -> Result<Vec<u8>> {
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

    pub fn key_der(&self, name: &str) -> Result<Vec<u8>> {
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

    pub fn pin(&self, name: &str) -> Result<String> {
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

    pub fn path(&self, file: &str) -> PathBuf {
        self.dir.join(file)
    }
}

pub fn free_addresses(count: usize) -> Result<Vec<SocketAddr>> {
    let listeners = (0..count)
        .map(|_| TcpListener::bind("127.0.0.1:0"))
        .collect::<std::io::Result<Vec<_>>>()?;
    listeners
        .iter()
        .map(|listener| listener.local_addr().map_err(Into::into))
        .collect()
}

pub fn operator_client(pki: &Pki, nodes: &[(String, SocketAddr)]) -> Result<AttestorClient> {
    Ok(AttestorClient::new(
        nodes,
        &[pki.certificate_der("node-ca")?],
        &[pki.certificate_der("operator")?],
        &pki.key_der("operator")?,
        Duration::from_secs(120),
    )?)
}

pub fn gateway_client(pki: &Pki, nodes: &[(String, SocketAddr)]) -> Result<AttestorClient> {
    Ok(AttestorClient::new(
        nodes,
        &[pki.certificate_der("node-ca")?],
        &[pki.certificate_der("gateway")?],
        &pki.key_der("gateway")?,
        Duration::from_secs(120),
    )?)
}

pub struct KeySetServer {
    address: SocketAddr,
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl KeySetServer {
    pub fn serve(document: String) -> Result<Self> {
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
                        let _ = respond(&mut stream, &document);
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

    pub fn url(&self) -> String {
        format!("http://{}/jwks.json", self.address)
    }
}

impl Drop for KeySetServer {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

fn respond(stream: &mut TcpStream, document: &str) -> Result<()> {
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    let mut request = Vec::new();
    let mut buffer = [0_u8; 1024];
    while !request.windows(4).any(|window| window == b"\r\n\r\n") {
        let read = stream.read(&mut buffer)?;
        if read == 0 || request.len() > 16 * 1024 {
            return Ok(());
        }
        request.extend_from_slice(&buffer[..read]);
    }
    write!(
        stream,
        "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
        document.len(),
        document
    )?;
    stream.flush()?;
    Ok(())
}

pub struct TokenSigner {
    key_id: String,
    key: SigningKey,
}

impl TokenSigner {
    fn public_pem(&self) -> String {
        let mut spki = vec![
            0x30, 0x59, 0x30, 0x13, 0x06, 0x07, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x02, 0x01, 0x06,
            0x08, 0x2a, 0x86, 0x48, 0xce, 0x3d, 0x03, 0x01, 0x07, 0x03, 0x42, 0x00,
        ];
        spki.extend_from_slice(self.key.verifying_key().to_encoded_point(false).as_bytes());
        format!(
            "-----BEGIN PUBLIC KEY-----\n{}\n-----END PUBLIC KEY-----\n",
            STANDARD.encode(spki)
        )
    }

    fn sign_inventory(&self, claims: &Value) -> String {
        let header = json!({"alg": "ES256", "typ": "wallet-custody-inventory+jwt"});
        let input = format!(
            "{}.{}",
            URL_SAFE_NO_PAD.encode(header.to_string()),
            URL_SAFE_NO_PAD.encode(claims.to_string())
        );
        let signature: Signature = self.key.sign(input.as_bytes());
        format!("{input}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes()))
    }

    pub fn generate(key_id: &str) -> Result<Self> {
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

    pub fn jwk(&self) -> Result<Value> {
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

    pub fn mint(&self, subject: &str) -> Result<String> {
        let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs();
        let header = json!({ "alg": "ES256", "typ": "JWT", "kid": self.key_id });
        let claims = json!({
            "iss": ISSUER,
            "sub": subject,
            "aud": AUDIENCE,
            "exp": now + 3600,
            "iat": now,
            "nbf": now - 5,
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

pub struct Cluster {
    root: PathBuf,
    pub pki: Pki,
    pub nodes: Vec<(String, SocketAddr)>,
    children: Vec<(String, Child)>,
    pub tokens: TokenSigner,
    keys: KeySetServer,
    inventory_members: Vec<Value>,
    inventory_keys: Vec<Value>,
    inventory_sequence: u64,
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
    pub fn start(activity_types: &[String]) -> Result<Self> {
        Self::start_with_kernel_policy(
            activity_types,
            &json!({"version": 1, "defaults": {"modules": {}}}),
        )
    }

    pub fn start_with_kernel_policy(
        activity_types: &[String],
        kernel_policy: &Value,
    ) -> Result<Self> {
        let root = scratch("attestor-cluster")?;
        let daemon = build_daemon(&root)?;
        let mut pki = Pki::new(&root);
        pki.authority("node-ca")?;
        pki.authority("operator-ca")?;
        pki.leaf("operator-ca", "operator", false)?;
        pki.authority("gateway-ca")?;
        pki.leaf("gateway-ca", "gateway", false)?;
        fs::write(
            pki.path("api-ca.pem"),
            [
                fs::read(pki.path("node-ca.pem"))?,
                fs::read(pki.path("gateway-ca.pem"))?,
            ]
            .concat(),
        )?;
        let ids: Vec<String> = (1..=NODES).map(|index| format!("node-{index}")).collect();
        let mut pins = Vec::new();
        for id in &ids {
            pki.leaf("node-ca", id, true)?;
            pins.push(pki.pin(id)?);
        }
        let tokens = TokenSigner::generate("attestor-test-key")?;
        fs::write(root.join("authority.pem"), tokens.public_pem())?;
        let keys = KeySetServer::serve(json!({ "keys": [tokens.jwk()?] }).to_string())?;
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
            serde_json::to_vec(kernel_policy)?,
        )?;
        let addresses = free_addresses(NODES * 2 + 1)?;
        let chain_rpc = format!("http://{}", addresses[NODES * 2]);
        let nodes: Vec<(String, SocketAddr)> = ids
            .iter()
            .enumerate()
            .map(|(index, id)| (id.clone(), addresses[index]))
            .collect();
        let peers: Vec<SocketAddr> = addresses[NODES..NODES * 2].to_vec();
        let mut cluster = Self {
            root: root.clone(),
            pki,
            nodes,
            children: Vec::new(),
            tokens,
            keys,
            inventory_members: ids
                .iter()
                .zip(&pins)
                .map(|(id, pin)| json!({"id": id, "spki_sha256": pin}))
                .collect(),
            inventory_keys: Vec::new(),
            inventory_sequence: 0,
        };
        cluster.publish_inventory()?;
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
                .env("ATTESTOR_JWKS_URL", cluster.keys.url())
                .env("ATTESTOR_JWT_ISSUER", ISSUER)
                .env("ATTESTOR_JWT_AUDIENCE", AUDIENCE)
                .env(
                    "ATTESTOR_AUTHORITY_PUBLIC_KEY_FILE",
                    root.join("authority.pem"),
                )
                .env("ATTESTOR_AUTHORITY_ISSUER", ISSUER)
                .env("ATTESTOR_AUTHORITY_TENANT", "attestor-test")
                .env(
                    "ATTESTOR_INVENTORY_PUBLIC_KEY_FILE",
                    root.join("authority.pem"),
                )
                .env("ATTESTOR_INVENTORY_FILE", root.join("inventory.jwt"))
                .env("ATTESTOR_POLICY_FILE", root.join("policy.json"))
                .env(
                    "ATTESTOR_TLS_CERT_FILE",
                    cluster.pki.path(&format!("{id}.pem")),
                )
                .env(
                    "ATTESTOR_TLS_KEY_FILE",
                    cluster.pki.path(&format!("{id}.key")),
                )
                .env("ATTESTOR_TLS_CA_FILE", cluster.pki.path("api-ca.pem"))
                .env(
                    "ATTESTOR_OPERATOR_CA_FILE",
                    cluster.pki.path("operator-ca.pem"),
                )
                .env("ATTESTOR_ACTIVITY_TYPES", activity_types.join(","))
                .env(
                    "ATTESTOR_KERNEL_POLICY_FILE",
                    root.join("kernel-policy.json"),
                )
                .env("ATTESTOR_RPC_URL", &chain_rpc)
                .stdin(Stdio::null())
                .stdout(log.try_clone()?)
                .stderr(log)
                .spawn()?;
            cluster.children.push((id.clone(), child));
        }
        cluster.wait_ready()?;
        Ok(cluster)
    }

    pub fn client(&self) -> Result<AttestorClient> {
        operator_client(&self.pki, &self.nodes)
    }

    fn publish_inventory(&mut self) -> Result<()> {
        self.inventory_sequence += 1;
        let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs();
        let claims = json!({
            "version": 1, "iss": ISSUER, "aud": "wallet-custody-inventory",
            "tenant": "attestor-test", "sequence": self.inventory_sequence.to_string(),
            "iat": now - 1, "exp": now + 3600, "protocol": "wallet", "threshold": 3,
            "members": self.inventory_members, "keys": self.inventory_keys,
        });
        let pending = self.root.join("inventory.pending");
        fs::write(&pending, self.tokens.sign_inventory(&claims))?;
        fs::set_permissions(&pending, fs::Permissions::from_mode(0o600))?;
        fs::rename(pending, self.root.join("inventory.jwt"))?;
        Ok(())
    }

    pub fn generate_key(
        &mut self,
        key_id: &str,
        curve: &str,
        owner: &str,
        account: &str,
    ) -> Result<Vec<u8>> {
        self.inventory_keys.push(json!({
            "key_id": key_id, "epoch": 0, "curve": curve, "public_key": "",
            "owner": owner, "operations": ["generate"],
        }));
        self.publish_inventory()?;
        let trusted =
            ureq::tls::Certificate::from_der(&self.pki.certificate_der("node-ca")?).to_owned();
        let certificate =
            ureq::tls::Certificate::from_der(&self.pki.certificate_der("gateway")?).to_owned();
        let private_key =
            ureq::tls::PrivateKey::from_pem(&fs::read(self.pki.path("gateway.key"))?)?;
        let agent: ureq::Agent = ureq::Agent::config_builder()
            .tls_config(
                ureq::tls::TlsConfig::builder()
                    .provider(ureq::tls::TlsProvider::Rustls)
                    .root_certs(ureq::tls::RootCerts::new_with_certs(&[trusted]))
                    .client_cert(Some(ureq::tls::ClientCert::new_with_certs(
                        &[certificate],
                        private_key,
                    )))
                    .build(),
            )
            .timeout_global(Some(Duration::from_secs(120)))
            .max_redirects(0)
            .build()
            .into();
        let body = serde_json::to_vec(&json!({
            "session_id": format!("generate-{key_id}"), "key_id": key_id,
            "curve": curve, "owner": owner, "account": account,
        }))?;
        let responses: Vec<Result<Value>> = thread::scope(|scope| {
            let handles: Vec<_> = self
                .nodes
                .iter()
                .map(|(_, address)| {
                    let agent = &agent;
                    let body = &body;
                    scope.spawn(move || -> std::result::Result<Value, String> {
                        let mut response = agent
                            .post(format!("https://{address}/v1/keys/generate"))
                            .header("content-type", "application/json")
                            .send(body.as_slice())
                            .map_err(|error| error.to_string())?;
                        let bytes = response
                            .body_mut()
                            .with_config()
                            .limit(65_536)
                            .read_to_vec()
                            .map_err(|error| error.to_string())?;
                        serde_json::from_slice(&bytes).map_err(|error| error.to_string())
                    })
                })
                .collect();
            handles
                .into_iter()
                .map(|handle| match handle.join() {
                    Ok(result) => result.map_err(Into::into),
                    Err(_) => Err("key generation worker panicked".into()),
                })
                .collect()
        });
        let mut public = None;
        for ((node, _), response) in self.nodes.iter().zip(responses) {
            let response = response?;
            assert_eq!(response["node_id"], *node);
            assert_eq!(response["key_id"], key_id);
            assert_eq!(response["curve"], curve);
            assert!(response["audit_sequence"]
                .as_u64()
                .is_some_and(|value| value > 0));
            let encoded = response["public_key"]
                .as_str()
                .ok_or("missing public key")?;
            let key = (0..encoded.len())
                .step_by(2)
                .map(|offset| u8::from_str_radix(&encoded[offset..offset + 2], 16))
                .collect::<std::result::Result<Vec<_>, _>>()?;
            if let Some(agreed) = &public {
                assert_eq!(agreed, &key);
            } else {
                public = Some(key);
            }
        }
        let public = public.ok_or("no key generation responses")?;
        assert_eq!(public.len(), if curve == "secp256k1" { 65 } else { 32 });
        let entry = self
            .inventory_keys
            .last_mut()
            .ok_or("missing inventory key")?;
        entry["public_key"] = json!(hex(&public));
        entry["operations"] = json!(["sign"]);
        self.publish_inventory()?;
        Ok(public)
    }

    pub fn gateway(&self) -> Result<AttestorClient> {
        gateway_client(&self.pki, &self.nodes)
    }

    pub fn key_set_url(&self) -> String {
        self.keys.url()
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

    pub fn logs(&self) -> String {
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

    pub fn audit(&self, client: &AttestorClient) -> Result<Vec<u64>> {
        SIGNERS
            .iter()
            .map(|id| Ok(checked(client.health(id))?.audit_sequence))
            .collect()
    }
}

pub fn build_daemon(root: &Path) -> Result<PathBuf> {
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
