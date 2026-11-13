//! In-process peers for the bounded repair gate; the hosted engine, Redis codec,
//! KMS client, HTTP framing and receiver verifier are the production components.
use super::*;
use crate::http::{read_request, write_reply, Reply, Request};
use ed25519_dalek::{Signer, SigningKey};
use rustls::pki_types::{PrivateKeyDer, PrivatePkcs8KeyDer};
use std::net::TcpListener;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::thread::{self, JoinHandle};

/// Unwraps a peer operation in the test harness.
///
/// # Panics
/// Panics when the peer operation returns an error.
pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

#[derive(Clone)]
pub struct TlsMaterial {
    pub ca: Vec<u8>,
    pub identity: Identity,
    pub server: Arc<rustls::ServerConfig>,
}

#[must_use]
pub fn tls_material() -> TlsMaterial {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let ca_key = must(rcgen::KeyPair::generate());
    let mut params = must(rcgen::CertificateParams::new(Vec::<String>::new()));
    params.is_ca = rcgen::IsCa::Ca(rcgen::BasicConstraints::Unconstrained);
    params
        .distinguished_name
        .push(rcgen::DnType::CommonName, "repair-1328-ca");
    params.key_usages = vec![
        rcgen::KeyUsagePurpose::KeyCertSign,
        rcgen::KeyUsagePurpose::CrlSign,
    ];
    let ca = must(params.self_signed(&ca_key));
    let issuer = rcgen::Issuer::from_params(&params, &ca_key);
    let key = must(rcgen::KeyPair::generate());
    let mut params = must(rcgen::CertificateParams::new(vec!["localhost".into()]));
    params
        .distinguished_name
        .push(rcgen::DnType::CommonName, "localhost");
    params.key_usages = vec![rcgen::KeyUsagePurpose::DigitalSignature];
    params.extended_key_usages = vec![
        rcgen::ExtendedKeyUsagePurpose::ServerAuth,
        rcgen::ExtendedKeyUsagePurpose::ClientAuth,
    ];
    let leaf = must(params.signed_by(&key, &issuer));
    let server = must(
        rustls::ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(
                vec![leaf.der().clone()],
                PrivateKeyDer::from(PrivatePkcs8KeyDer::from(key.serialize_der())),
            ),
    );
    TlsMaterial {
        ca: ca.der().to_vec(),
        identity: must(Identity::from_pkcs8(
            format!("{}{}", leaf.pem(), ca.pem()).as_bytes(),
            key.serialize_pem().as_bytes(),
        )),
        server: Arc::new(server),
    }
}

pub struct Peer {
    pub url: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            must(worker.join());
        }
    }
}
impl Peer {
    fn tls(
        tls: &TlsMaterial,
        handler: impl Fn(&mut rustls::StreamOwned<rustls::ServerConnection, TcpStream>)
            + Send
            + Sync
            + 'static,
    ) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let url = format!("https://localhost:{}", must(listener.local_addr()).port());
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let tls = Arc::clone(&tls.server);
        let handler = Arc::new(handler);
        let worker = thread::spawn(move || {
            let mut connections = Vec::new();
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((socket, _)) => {
                        let tls = Arc::clone(&tls);
                        let handler = Arc::clone(&handler);
                        connections.push(thread::spawn(move || {
                            must(socket.set_read_timeout(Some(Duration::from_secs(5))));
                            must(socket.set_write_timeout(Some(Duration::from_secs(5))));
                            let mut stream = rustls::StreamOwned::new(
                                must(rustls::ServerConnection::new(tls)),
                                socket,
                            );
                            handler(&mut stream);
                            stream.conn.send_close_notify();
                            let _ = stream.conn.complete_io(&mut stream.sock);
                        }));
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("{error}"),
                }
            }
            for connection in connections {
                must(connection.join());
            }
        });
        Self {
            url,
            stop,
            worker: Some(worker),
        }
    }

    pub fn http(
        tls: &TlsMaterial,
        handler: impl Fn(Request) -> Reply + Send + Sync + 'static,
    ) -> Self {
        Self::tls(tls, move |stream| {
            let request = must(read_request(stream));
            must(write_reply(stream, &handler(request)));
        })
    }
}

/// The peer owns real revisioned state and executes the Redis operations used by
/// the repository; it does not synthesize hosted shards or delivery transitions.
#[derive(Default)]
pub struct MemoryRedis {
    pub shards: BTreeMap<String, (u64, String)>,
    pub members: std::collections::BTreeSet<String>,
}
impl MemoryRedis {
    fn command(&mut self, args: &[String]) -> Resp {
        match args[0].as_str() {
            "PING" => Resp::Simple("PONG".into()),
            "SMEMBERS" => Resp::Array(
                self.members
                    .iter()
                    .map(|id| Resp::Bulk(Some(id.as_bytes().to_vec())))
                    .collect(),
            ),
            "HMGET" => {
                let value = self.shards.get(&args[1]);
                Resp::Array(vec![
                    Resp::Bulk(value.map(|v| v.0.to_string().into_bytes())),
                    Resp::Bulk(value.map(|v| v.1.as_bytes().to_vec())),
                ])
            }
            "HGET" => Resp::Bulk(self.shards.get(&args[1]).map(|v| v.1.as_bytes().to_vec())),
            "EVAL" => {
                assert_eq!(args[1], CAS_SCRIPT);
                assert_eq!(args[2], "2");
                assert_eq!(args[4], "webhooks:principals");
                let expected = must(args[5].parse::<u64>());
                let current = self.shards.get(&args[3]).map_or(0, |v| v.0);
                if current != expected {
                    return Resp::Integer(0);
                }
                self.shards
                    .insert(args[3].clone(), (current + 1, args[6].clone()));
                self.members.insert(args[7].clone());
                Resp::Integer(1)
            }
            other => panic!("unsupported Redis operation {other}"),
        }
    }

    /// Replaces a committed shard with malformed JSON.
    ///
    /// # Panics
    /// Panics when the principal has no committed shard.
    pub fn corrupt(&mut self, principal: &Principal) -> String {
        let key = shard_key(principal);
        let old = self
            .shards
            .get_mut(&key)
            .unwrap_or_else(|| panic!("producer committed shard"));
        std::mem::replace(&mut old.1, "{broken durable shard".into())
    }
    /// Restores the principal's committed shard.
    ///
    /// # Panics
    /// Panics when the principal has no committed shard.
    pub fn restore(&mut self, principal: &Principal, json: String) {
        self.shards
            .get_mut(&shard_key(principal))
            .unwrap_or_else(|| panic!("shard"))
            .1 = json;
    }
}
fn reply_resp(stream: &mut impl Write, response: Resp) {
    match response {
        Resp::Simple(value) => must(write!(stream, "+{value}\r\n")),
        Resp::Integer(value) => must(write!(stream, ":{value}\r\n")),
        Resp::Bulk(None) => must(stream.write_all(b"$-1\r\n")),
        Resp::Bulk(Some(value)) => {
            must(write!(stream, "${}\r\n", value.len()));
            must(stream.write_all(&value));
            must(stream.write_all(b"\r\n"));
        }
        Resp::Array(values) => {
            must(write!(stream, "*{}\r\n", values.len()));
            for value in values {
                reply_resp(stream, value);
            }
        }
    }
    must(stream.flush());
}
fn arguments(stream: &mut impl Read) -> Vec<String> {
    let Resp::Array(values) = must(read_resp(stream, 0)) else {
        panic!("Redis command array");
    };
    values
        .iter()
        .map(|v| resp_text(v).unwrap_or_else(|| panic!("string command argument")))
        .collect()
}

pub struct Harness {
    pub tls: TlsMaterial,
    pub store: Arc<Mutex<MemoryRedis>>,
    redis: Peer,
    kms: Peer,
}
impl Harness {
    /// Starts the Redis and KMS peers for a hosted service.
    ///
    /// # Panics
    /// Panics when a peer fails to start or rejects its expected request.
    #[must_use]
    pub fn new() -> Self {
        let tls = tls_material();
        let store = Arc::new(Mutex::new(MemoryRedis::default()));
        let redis_store = Arc::clone(&store);
        let redis = Peer::tls(&tls, move |stream| {
            assert_eq!(arguments(stream), ["AUTH", "webhooks", "fixture-secret"]);
            reply_resp(stream, Resp::Simple("OK".into()));
            let args = arguments(stream);
            let response = must(redis_store.lock()).command(&args);
            reply_resp(stream, response);
        });
        let kms = Peer::http(&tls, |request| {
            assert_eq!(request.header("authorization"), Some("Bearer fixture-kms"));
            let key = SigningKey::from_bytes(&[91; 32]);
            let response = match request.path.as_str() {
                "/readyz" => serde_json::json!({"ready":true,"ed25519_non_exportable":true}),
                "/v1/signing-keys" => {
                    assert!(request.header("idempotency-key").is_some());
                    serde_json::json!({"key_id":"whk_repair","handle":"fixture-key","public_key":base64_encode(key.verifying_key().as_bytes())})
                }
                "/v1/signatures" => {
                    let body: serde_json::Value = must(serde_json::from_slice(&request.body));
                    assert_eq!(body["algorithm"], "ed25519");
                    assert_eq!(body["key_handle"], "fixture-key");
                    let message = must(base64_decode(
                        body["message"]
                            .as_str()
                            .unwrap_or_else(|| panic!("message")),
                    ));
                    serde_json::json!({"signature":base64_encode(&key.sign(&message).to_bytes())})
                }
                path => panic!("unexpected KMS route {path}"),
            };
            Reply::json(200, response.to_string())
        });
        Self {
            tls,
            store,
            redis,
            kms,
        }
    }

    #[must_use]
    pub fn service(&self, instance: &str, policy: RetryPolicy) -> HostedService {
        let ca = must(Certificate::from_der(&self.tls.ca));
        let identity = ClientIdentity::new(ca.clone(), Some(self.tls.identity.clone()));
        HostedService {
            repository: RedisRepository {
                endpoint: must(RedisEndpoint::parse(&self.redis.url.replacen(
                    "https://",
                    "rediss://",
                    1,
                ))),
                ca,
                username: Zeroizing::new("webhooks".into()),
                password: Zeroizing::new("fixture-secret".into()),
            },
            kms: KmsClient {
                client: Client::trusted(identity.clone()),
                endpoint: must(Endpoint::parse(&self.kms.url)),
                token: Zeroizing::new("fixture-kms".into()),
            },
            // Only the test's transport routes to its loopback peers. The production
            // constructor always installs Client::public and retains its IP checks.
            outbound: Client::trusted(identity),
            policy: must(policy.validate()),
            instance: instance.into(),
            cursor_key: Zeroizing::new([17; 32]),
            key_overlap_seconds: 86_400,
            worker_failures: Mutex::new(BTreeMap::new()),
        }
    }
}

impl Default for Harness {
    fn default() -> Self {
        Self::new()
    }
}
