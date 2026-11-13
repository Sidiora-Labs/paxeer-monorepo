use super::support;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use layerx_human_identity_provider::{AssertionConfig, AssertionVerifier, Policy, Server, State};
use layerx_human_service::server::{
    IdentityDispatchError, IdentityProviderConfig, RemoteIdentityProvider,
};
use layerx_human_service::store::{
    AgentTenantId, PrincipalId, PrincipalStore, PrincipalTenancyAuthority, RetentionPeriod,
    RetentionPolicy, RowKey, StoreError, Table, TenancyDigest, TenancyMap,
};
use layerx_identity_binding::{Client, Config};
use layerx_types::clock::Clock as _;
use p256::ecdsa::signature::Signer as _;
use p256::ecdsa::{Signature, SigningKey};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::error::Error;
use std::fs;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::Path;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::thread::{self, JoinHandle};
use std::time::Duration;

pub type Result<T = ()> = std::result::Result<T, Box<dyn Error>>;

fn encoded(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut text = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        let _ = write!(text, "{byte:02x}");
    }
    text
}

const ISSUER: &str = "https://identity.example.test/auth/v1";
const AUDIENCE: &str = "authenticated";
const TENANT: &str = "human-provider";
const WALLET_DID: &str =
    "did:layerx:3f1c0a9e5b7d2468ace013579bdf2468ace013579bdf2468ace013579bdf2468";

struct KeySetServer {
    address: SocketAddr,
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl KeySetServer {
    fn serve(keys: &[Value]) -> Result<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let body = json!({ "keys": keys }).to_string();
        let shutdown = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&shutdown);
        let worker = thread::spawn(move || {
            for stream in listener.incoming() {
                if stop.load(Ordering::Acquire) {
                    break;
                }
                if let Ok(mut stream) = stream {
                    let _ = respond(&mut stream, &body);
                }
            }
        });
        Ok(Self {
            address,
            shutdown,
            worker: Some(worker),
        })
    }

    fn url(&self) -> String {
        format!("http://{}/auth/v1/.well-known/jwks.json", self.address)
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

fn respond(stream: &mut TcpStream, body: &str) -> Result {
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    let mut request = Vec::new();
    let mut buffer = [0u8; 1024];
    while !request.windows(4).any(|window| window == b"\r\n\r\n") {
        let read = stream.read(&mut buffer)?;
        if read == 0 || request.len() > 16 * 1024 {
            return Ok(());
        }
        request.extend_from_slice(&buffer[..read]);
    }
    if !request.starts_with(b"GET /auth/v1/.well-known/jwks.json ") {
        stream.write_all(
            b"HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n",
        )?;
        return Ok(());
    }
    write!(
        stream,
        "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
        body.len(),
        body
    )?;
    stream.flush()?;
    Ok(())
}

struct Signer(SigningKey);

impl Signer {
    fn generate() -> Result<Self> {
        loop {
            let mut scalar = [0u8; 32];
            getrandom::fill(&mut scalar)?;
            if let Ok(key) = SigningKey::from_slice(&scalar) {
                return Ok(Self(key));
            }
        }
    }

    fn jwk(&self, key_id: &str) -> Result<Value> {
        let point = self.0.verifying_key().to_encoded_point(false);
        Ok(json!({
            "kty": "EC", "crv": "P-256", "kid": key_id, "use": "sig", "alg": "ES256",
            "x": URL_SAFE_NO_PAD.encode(point.x().ok_or("missing x")?),
            "y": URL_SAFE_NO_PAD.encode(point.y().ok_or("missing y")?),
        }))
    }

    fn mint(&self, header: &Value, claims: &Value) -> String {
        let input = format!(
            "{}.{}",
            URL_SAFE_NO_PAD.encode(header.to_string()),
            URL_SAFE_NO_PAD.encode(claims.to_string())
        );
        let signature: Signature = self.0.sign(input.as_bytes());
        format!("{input}.{}", URL_SAFE_NO_PAD.encode(signature.to_bytes()))
    }

    fn assertion(&self, subject: &str, now: u64) -> String {
        self.mint(
            &json!({"alg": "ES256", "typ": "JWT", "kid": "supabase-1"}),
            &json!({"iss": ISSUER, "sub": subject, "aud": AUDIENCE, "exp": now + 3600,
                "iat": now, "nbf": now - 5, "role": "authenticated"}),
        )
    }
}

fn authority_now() -> Result<u64> {
    Ok(support::clock()?
        .sample(Duration::from_secs(1))?
        .unix_seconds())
}

fn identity(root: &Path, peer_gid: u32) -> Result<RemoteIdentityProvider> {
    Ok(RemoteIdentityProvider::new(IdentityProviderConfig {
        socket: root.join("identity.sock"),
        deadline: Duration::from_secs(1),
        maximum_frame_bytes: 65_536,
        peer_uid: rustix::process::geteuid().as_raw(),
        peer_gid,
    })?)
}

#[derive(Debug)]
struct Authority(Client);
impl PrincipalTenancyAuthority for Authority {
    fn tenant_for(
        &self,
        principal: &PrincipalId,
    ) -> std::result::Result<AgentTenantId, StoreError> {
        let binding = self.0.lookup(principal.as_str())?;
        AgentTenantId::new(binding.agent_tenant())
    }
}

struct Running {
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::io::Result<()>>>,
}
impl Running {
    fn start_with(root: &Path, assertion: Option<AssertionVerifier>) -> Result<Self> {
        let uid = rustix::process::geteuid().as_raw();
        let mut state = State::open(
            &root.join("identity"),
            Policy {
                root: [0x43; 32],
                threshold: 1,
                delay_seconds: 86_400,
            },
        )?;
        if let Some(verifier) = assertion {
            state.enable_assertion(verifier)?;
        }
        let server = Server::bind(
            &root.join("identity.sock"),
            state,
            uid,
            Duration::from_secs(1),
            support::clock()?,
        )?
        .with_binding_reader(&root.join("binding.sock"), "human-provider", &[uid])?;
        let shutdown = Arc::new(AtomicBool::new(false));
        let flag = Arc::clone(&shutdown);
        Ok(Self {
            shutdown,
            worker: Some(thread::spawn(move || server.run(&flag))),
        })
    }
    fn stop(mut self) -> Result {
        self.shutdown.store(true, Ordering::Release);
        self.worker
            .take()
            .ok_or("worker missing")?
            .join()
            .map_err(|_| "provider panicked")??;
        Ok(())
    }
}
impl Drop for Running {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

fn client(root: &Path) -> Result<Client> {
    tenant_client(root, TENANT)
}

fn tenant_client(root: &Path, tenant: &str) -> Result<Client> {
    Ok(Client::new(
        Config {
            socket: root.join("binding.sock"),
            tenant: tenant.into(),
            peer_uid: rustix::process::geteuid().as_raw(),
            peer_gid: rustix::process::getegid().as_raw(),
            deadline: Duration::from_secs(1),
        },
        support::clock()?,
    )?)
}

fn open(root: &Path, digest: TenancyDigest, client: Client) -> Result<PrincipalStore> {
    let period = RetentionPeriod::new(1_000);
    Ok(PrincipalStore::open_with_authority(
        root,
        RetentionPolicy {
            journeys: period,
            notifications: period,
            audit: period,
            telemetry: period,
            cache: period,
        },
        digest,
        Arc::new(Authority(client)),
    )?)
}

fn recorded() -> Result<Value> {
    let record: Value =
        serde_json::from_str(include_str!("../fixtures/wallet_binding_22_33_35.json"))?;
    for (path, source) in [
        (
            "services/wallet/gateway/src/provision/state.ts",
            include_str!(concat!(
                env!("CARGO_MANIFEST_DIR"),
                "/../../../wallet/gateway/src/provision/state.ts"
            )),
        ),
        (
            "services/wallet/gateway/src/provision/bind.ts",
            include_str!(concat!(
                env!("CARGO_MANIFEST_DIR"),
                "/../../../wallet/gateway/src/provision/bind.ts"
            )),
        ),
        (
            "services/wallet/gateway/src/rpc/pool.ts",
            include_str!(concat!(
                env!("CARGO_MANIFEST_DIR"),
                "/../../../wallet/gateway/src/rpc/pool.ts"
            )),
        ),
        (
            "services/wallet/gateway/test/support/chain.ts",
            include_str!(concat!(
                env!("CARGO_MANIFEST_DIR"),
                "/../../../wallet/gateway/test/support/chain.ts"
            )),
        ),
    ] {
        let digest = encoded(&Sha256::digest(source.as_bytes()));
        assert_eq!(
            record["sources"][path], digest,
            "regenerate the recording after changing its producer"
        );
    }
    assert_eq!(
        record["producer"],
        "services/wallet/gateway/src/provision/state.ts:produceIdentityBinding"
    );
    assert_eq!(record["issuer"], ISSUER);
    assert_eq!(record["tenant"], TENANT);
    assert_eq!(record["did"], WALLET_DID);
    Ok(record)
}

fn text<'a>(record: &'a Value, field: &str) -> Result<&'a str> {
    record[field]
        .as_str()
        .ok_or_else(|| format!("recorded {field} missing").into())
}

fn verifier(keys: &KeySetServer, record: &Value) -> Result<AssertionVerifier> {
    Ok(AssertionVerifier::new_with_clock(
        AssertionConfig {
            jwks_url: keys.url(),
            issuer: ISSUER.to_owned(),
            audience: AUDIENCE.to_owned(),
            clock_skew_seconds: 30,
            refresh_interval_seconds: 300,
        },
        support::clock()?,
    )?
    .with_binding_producer_key(text(record, "publicKey")?)?)
}

fn refused(value: &std::result::Result<impl std::fmt::Debug, IdentityDispatchError>) {
    assert!(
        matches!(value, Err(IdentityDispatchError::ProviderRefused)),
        "binding must be refused: {value:?}"
    );
}

fn forged_binding(binding: &str) -> Result<String> {
    let (signed, _) = binding
        .rsplit_once('.')
        .ok_or("binding signature missing")?;
    let forger = Signer::generate()?;
    let signature: Signature = forger.0.sign(signed.as_bytes());
    Ok(format!(
        "{signed}.{}",
        URL_SAFE_NO_PAD.encode(signature.to_bytes())
    ))
}

pub fn wallet_gateway_binding_producer_transport_and_reopen() -> Result {
    let record = recorded()?;
    let directory = tempfile::Builder::new()
        .permissions(fs::Permissions::from_mode(0o700))
        .tempdir()?;
    let root = directory.path();
    let issuer = Signer::generate()?;
    let keys = KeySetServer::serve(&[issuer.jwk("supabase-1")?])?;
    let running = Running::start_with(root, Some(verifier(&keys, &record)?))?;
    let now = authority_now()?;
    let token = issuer.assertion(text(&record, "subject")?, now);
    let other_token = issuer.assertion("supabase-user-b", now);
    let identity = self::identity(root, rustix::process::getegid().as_raw())?;
    let binding = text(&record, "binding")?;
    let reader = client(root)?;
    let pending = identity.resolve_assertion(&token)?;
    assert!(pending.binding_pending());
    assert!(reader.lookup(pending.principal.as_str()).is_err());

    for offered in [
        forged_binding(binding)?,
        text(&record, "crossTenant")?.to_owned(),
    ] {
        refused(&identity.resolve_assertion_with_binding(&token, Some(&offered)));
        assert_eq!(identity.resolve_assertion(&token)?, pending);
        assert!(reader.lookup(pending.principal.as_str()).is_err());
    }
    refused(&identity.resolve_assertion_with_binding(&other_token, Some(binding)));
    let untrusted = Signer::generate()?.assertion(text(&record, "subject")?, now);
    refused(&identity.resolve_assertion_with_binding(&untrusted, Some(binding)));
    assert!(reader.lookup(pending.principal.as_str()).is_err());

    // This is the gateway's recorded signature, passed unchanged through LXIP operation 4.
    let account = identity.resolve_assertion_with_binding(&token, Some(binding))?;
    assert_eq!(account.principal, pending.principal);
    assert_eq!(account.did.as_deref(), Some(WALLET_DID));
    assert!(!account.binding_pending());
    let lookup = reader.lookup(account.principal.as_str())?;
    assert_eq!(lookup.did().as_bytes(), WALLET_DID.as_bytes());
    assert_eq!(lookup.tenant(), TENANT);
    assert_eq!(
        identity.resolve_assertion_with_binding(&token, Some(binding))?,
        account
    );
    let other = identity.resolve_assertion(&other_token)?;
    assert!(other.binding_pending());
    assert_ne!(other.principal, account.principal);

    // PrincipalStore is first opened only after the producer binding is accepted.
    let store_root = root.join("store");
    let digest = TenancyMap::new([])?.install(&store_root)?;
    let mut store = open(&store_root, digest, reader.clone())?;
    let row = RowKey::new("producer-binding")?;
    {
        let mut scope = store.principal(&account.principal)?;
        assert_eq!(scope.tenant().as_str(), lookup.agent_tenant());
        scope.put(
            Table::Journeys,
            row.clone(),
            now,
            WALLET_DID.as_bytes().to_vec(),
        )?;
    }
    assert!(store.principal(&other.principal).is_err());
    assert!(tenant_client(root, "other-tenant")?
        .lookup(account.principal.as_str())
        .is_err());
    drop(store);
    drop(identity);
    drop(reader);
    running.stop()?;
    assert!(client(root)?.lookup(account.principal.as_str()).is_err());

    let running = Running::start_with(root, Some(verifier(&keys, &record)?))?;
    let identity = self::identity(root, rustix::process::getegid().as_raw())?;
    let reader = client(root)?;
    // Recovery succeeds without resubmitting a wallet binding.
    assert_eq!(identity.resolve_assertion(&token)?, account);
    let recovered = reader.lookup(account.principal.as_str())?;
    assert_eq!(recovered.did().as_bytes(), lookup.did().as_bytes());
    assert_eq!(recovered.tenant(), lookup.tenant());
    assert_eq!(recovered.agent_tenant(), lookup.agent_tenant());
    let mut store = open(&store_root, digest, reader.clone())?;
    let scope = store.principal(&account.principal)?;
    assert_eq!(scope.tenant().as_str(), recovered.agent_tenant());
    assert_eq!(
        scope
            .get(Table::Journeys, &row)
            .ok_or("recovered binding row missing")?
            .bytes(),
        WALLET_DID.as_bytes()
    );
    drop(scope);
    assert!(store.principal(&other.principal).is_err());
    refused(&identity.resolve_assertion_with_binding(&token, Some(&forged_binding(binding)?)));
    refused(&identity.resolve_assertion_with_binding(&other_token, Some(binding)));
    refused(&identity.resolve_assertion_with_binding(&token, Some(text(&record, "crossTenant")?)));
    assert_eq!(identity.resolve_assertion(&token)?, account);
    assert_eq!(
        reader.lookup(account.principal.as_str())?.agent_tenant(),
        lookup.agent_tenant()
    );
    drop(store);
    running.stop()?;
    Ok(())
}

fn unhex(value: &str) -> Result<Vec<u8>> {
    let value = value.strip_prefix("0x").unwrap_or(value);
    if !value.len().is_multiple_of(2) {
        return Err("odd hexadecimal width".into());
    }
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| Ok(u8::from_str_radix(std::str::from_utf8(pair)?, 16)?))
        .collect()
}

struct AccountPeer {
    address: SocketAddr,
    shutdown: Arc<AtomicBool>,
    worker: Option<JoinHandle<std::result::Result<(), String>>>,
}

impl AccountPeer {
    fn start(record: Value) -> Result<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let address = listener.local_addr()?;
        let shutdown = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&shutdown);
        let worker = thread::spawn(move || -> std::result::Result<(), String> {
            let serve = || -> Result {
                while !stop.load(Ordering::Acquire) {
                    let (stream, _) = match listener.accept() {
                        Ok(pair) => pair,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(1));
                            continue;
                        }
                        Err(error) => return Err(error.into()),
                    };
                    stream.set_read_timeout(Some(Duration::from_secs(3)))?;
                    stream.set_write_timeout(Some(Duration::from_secs(3)))?;
                    let mut reader = BufReader::new(stream);
                    let mut length = None;
                    loop {
                        let mut line = String::new();
                        if reader.read_line(&mut line)? == 0 {
                            return Err("truncated HTTP headers".into());
                        }
                        if line == "\r\n" {
                            break;
                        }
                        if let Some(value) =
                            line.to_ascii_lowercase().strip_prefix("content-length:")
                        {
                            length = Some(value.trim().parse::<usize>()?);
                        }
                    }
                    let length = length.ok_or("HTTP request length missing")?;
                    assert!(length <= 65_536);
                    let mut body = vec![0; length];
                    reader.read_exact(&mut body)?;
                    let request: Value = serde_json::from_slice(&body)?;
                    assert_eq!(request["jsonrpc"], "2.0");
                    assert_eq!(request["method"], "eth_call");
                    let observation = record["account"]["observations"]
                        .as_array()
                        .ok_or("observations missing")?
                        .iter()
                        .find(|observation| observation["params"] == request["params"])
                        .ok_or("unrecorded account request")?;
                    let body = json!({"jsonrpc":"2.0", "id":request["id"], "result":observation["result"]}).to_string();
                    let mut stream = reader.into_inner();
                    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len())?;
                    stream.flush()?;
                }
                Ok(())
            };
            serve().map_err(|error| error.to_string())
        });
        Ok(Self {
            address,
            shutdown,
            worker: Some(worker),
        })
    }

    fn endpoint(&self) -> Result<layerx_client::sequencer::SequencerEndpoint> {
        layerx_client::sequencer::SequencerEndpoint::new(&format!("http://{}/", self.address), &[])
            .map_err(|error| format!("account endpoint: {error:?}").into())
    }
}

impl Drop for AccountPeer {
    fn drop(&mut self) {
        self.shutdown.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            assert_eq!(
                worker
                    .join()
                    .unwrap_or_else(|_| panic!("account peer panicked")),
                Ok(())
            );
        }
    }
}

pub fn merged_account_head_transport_validator_and_reopen() -> Result {
    use layerx_client::account_binding::{bound_did_call, decode_bound_did, ADDR_PRECOMPILE};
    let record = recorded()?;
    let directory = tempfile::Builder::new()
        .permissions(fs::Permissions::from_mode(0o700))
        .tempdir()?;
    let root = directory.path();
    let issuer = Signer::generate()?;
    let keys = KeySetServer::serve(&[issuer.jwk("supabase-1")?])?;
    let running = Running::start_with(root, Some(verifier(&keys, &record)?))?;
    let token = issuer.assertion(text(&record, "subject")?, authority_now()?);
    let identity = self::identity(root, rustix::process::getegid().as_raw())?;
    let account =
        identity.resolve_assertion_with_binding(&token, Some(text(&record, "binding")?))?;
    let reader = client(root)?;
    let lookup = reader.lookup(account.principal.as_str())?;
    let address: [u8; 20] = unhex(text(&record, "address")?)?
        .try_into()
        .map_err(|_| "address width")?;
    let public: [u8; 32] = unhex(WALLET_DID.strip_prefix("did:layerx:").ok_or("DID prefix")?)?
        .try_into()
        .map_err(|_| "DID width")?;
    let peer = AccountPeer::start(record.clone())?;
    let endpoint = peer.endpoint()?;
    let store_root = root.join("store");
    let digest = TenancyMap::new([])?.install(&store_root)?;
    let mut store = open(&store_root, digest, reader)?;
    let observations = record["account"]["observations"]
        .as_array()
        .ok_or("observations missing")?;
    assert_eq!(observations.len(), 2);
    let mut previous_height = 1;
    for observation in observations {
        let height = observation["height"].as_u64().ok_or("height missing")?;
        assert!(height > previous_height);
        previous_height = height;
        let result = endpoint
            .call_at(height, &ADDR_PRECOMPILE, &bound_did_call(&address))
            .map_err(|error| format!("merged account transport: {error:?}"))?;
        assert_eq!(result, unhex(text(observation, "result")?)?);
        assert_eq!(decode_bound_did(&result)?, Some(public));
        assert_eq!(
            lookup.did().as_bytes(),
            format!("did:layerx:{}", encoded(&result[64..96])).as_bytes()
        );
        assert_eq!(&result[12..32], &address);
        assert_eq!(&result[96..128], unhex(text(&record, "mainAccountId")?)?);
        for malformed in [&result[..127], &[result.as_slice(), &[0]].concat()] {
            assert!(decode_bound_did(malformed).is_err());
        }
        let mut wrong_key = result.clone();
        wrong_key[64] ^= 1;
        assert_ne!(decode_bound_did(&wrong_key)?, Some(public));
        store.principal(&account.principal)?.put(
            Table::Cache,
            RowKey::new(format!("head-{height}"))?,
            height,
            result,
        )?;
    }
    drop(endpoint);
    drop(peer);
    drop(store);
    drop(identity);
    running.stop()?;

    let running = Running::start_with(root, Some(verifier(&keys, &record)?))?;
    let identity = self::identity(root, rustix::process::getegid().as_raw())?;
    assert_eq!(identity.resolve_assertion(&token)?, account);
    let reader = client(root)?;
    let recovered = reader.lookup(account.principal.as_str())?;
    assert_eq!(recovered.did().as_bytes(), lookup.did().as_bytes());
    assert_eq!(recovered.agent_tenant(), lookup.agent_tenant());
    let mut store = open(&store_root, digest, reader)?;
    let peer = AccountPeer::start(record.clone())?;
    let endpoint = peer.endpoint()?;
    let scope = store.principal(&account.principal)?;
    for observation in observations {
        let height = observation["height"].as_u64().ok_or("height missing")?;
        let key = RowKey::new(format!("head-{height}"))?;
        let row = scope
            .get(Table::Cache, &key)
            .ok_or("recovered account observation missing")?;
        assert_eq!(decode_bound_did(row.bytes())?, Some(public));
        assert_eq!(
            endpoint
                .call_at(height, &ADDR_PRECOMPILE, &bound_did_call(&address))
                .map_err(|error| format!("reopened account transport: {error:?}"))?,
            row.bytes()
        );
    }
    drop(scope);
    drop(store);
    running.stop()?;
    Ok(())
}
