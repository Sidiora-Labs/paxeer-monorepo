#[path = "support/producer_redis.rs"]
mod producer_redis;
#[path = "../../../../../programs/crates/layerx-programs-registry/tests/support/mod.rs"]
pub mod support;
#[path = "../../internal/src/producer_test_support.rs"]
mod transport;

use layerx_platform_internal::{
    events::{enrollment_snapshot_mac, Kind, ProducerCredential, Service, ENROLLMENT_VERSION},
    gateway_http, http, kms,
    principal::PrincipalClient,
    producer::{Client, Outbox},
    seal::SealKey,
    secret::sha256_hex,
};
use layerx_platform_registry::event_producer::ProgramOutbox;
use std::collections::BTreeMap;
use std::fs;
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::sync::Arc;
use std::time::Duration;
use zeroize::Zeroizing;

struct IdentityServer(std::process::Child);
impl Drop for IdentityServer {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

fn service_binary(package: &str, binary: &str) -> PathBuf {
    let manifest = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../Cargo.toml");
    let output = std::process::Command::new(env!("CARGO"))
        .args(["build", "--locked", "--manifest-path"])
        .arg(manifest)
        .args(["-p", package, "--bin", binary, "--message-format=json"])
        .output()
        .unwrap_or_else(|error| panic!("build {binary} fixture: {error}"));
    assert!(
        output.status.success(),
        "build {binary} fixture failed: {}\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    String::from_utf8(output.stdout)
        .unwrap_or_else(|error| panic!("{binary} build output: {error}"))
        .lines()
        .map(|line| {
            serde_json::from_str::<serde_json::Value>(line)
                .unwrap_or_else(|error| panic!("{binary} build artifact: {error}"))
        })
        .find_map(|artifact| {
            (artifact["reason"] == "compiler-artifact" && artifact["target"]["name"] == binary)
                .then(|| artifact["executable"].as_str().map(PathBuf::from))
                .flatten()
        })
        .unwrap_or_else(|| panic!("{binary} build did not produce the service executable"))
}

fn identity(root: &Path, tls: &transport::Tls) -> (IdentityServer, PrincipalClient) {
    let tokens = root.join("identity-tokens");
    fs::create_dir_all(&tokens).unwrap_or_else(|error| panic!("tokens directory: {error:?}"));
    for service in [
        "gateway",
        "registry",
        "webhooks",
        "dashboard",
        "faucet",
        "testnet",
        "ramp",
        "provisioning",
        "registrar",
    ] {
        fs::write(tokens.join(service), format!("{service}-integration-token"))
            .unwrap_or_else(|error| panic!("service token: {error:?}"));
    }
    fs::write(root.join("store-key"), "identity-integration-store-key")
        .unwrap_or_else(|error| panic!("store key: {error:?}"));
    let listener = std::net::TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("port: {error:?}"));
    let port = listener
        .local_addr()
        .unwrap_or_else(|error| panic!("address: {error:?}"))
        .port();
    drop(listener);
    let child = std::process::Command::new(service_binary(
        "layerx-platform-identity",
        "layerx-identity",
    ))
    .env_clear()
    .env("LAYERX_IDENTITY_LISTEN", format!("127.0.0.1:{port}"))
    .env("LAYERX_IDENTITY_HEALTH_ADDR", "127.0.0.1:0")
    .env("LAYERX_IDENTITY_TLS_CERT_DER", root.join("cert.der"))
    .env("LAYERX_IDENTITY_TLS_KEY_DER", root.join("key.der"))
    .env("LAYERX_IDENTITY_CLIENT_CA_DER", root.join("cert.der"))
    .env("LAYERX_IDENTITY_STATE_DIR", root.join("identity-state"))
    .env("LAYERX_IDENTITY_SERVICE_TOKENS_DIR", tokens)
    .env("LAYERX_IDENTITY_STORE_KEY_FILE", root.join("store-key"))
    .stdout(std::process::Stdio::null())
    .stderr(std::process::Stdio::inherit())
    .spawn()
    .unwrap_or_else(|error| panic!("identity spawn: {error:?}"));
    let mut server = IdentityServer(child);
    let mut ready = false;
    for _ in 0..100 {
        assert!(server
            .0
            .try_wait()
            .unwrap_or_else(|error| panic!("identity status: {error:?}"))
            .is_none());
        if tls
            .upstream(port, "registry-integration-token")
            .get("/livez")
            .is_ok_and(|reply| reply.status == 200)
        {
            ready = true;
            break;
        }
        std::thread::sleep(Duration::from_millis(50));
    }
    assert!(ready, "identity did not become live within 100 attempts");
    let endpoint = gateway_http::Endpoint::parse(&format!("https://localhost:{port}"))
        .unwrap_or_else(|error| panic!("identity endpoint: {error:?}"));
    let client = gateway_http::Client::new(tls.ca.clone(), tls.identity.clone());
    for (path, body, key) in [
        ("/v1/principals", serde_json::json!({"tenant":"beta", "sub":"program-principal", "allowed_signer_public_keys":["ab".repeat(32)]}).to_string(), None),
        ("/v1/publication-keys", serde_json::json!({"sub":"program-principal", "revoked":false}).to_string(), Some("program-key:publication-integration")),
    ] {
        let response = client.request_with_principal(&endpoint, "Bearer provisioning-integration-token", &gateway_http::OutboundRequest { method:"POST", path, idempotency:None, content_type:"application/json", body:body.as_bytes() }, None, key).unwrap_or_else(|error| panic!("provisioning response: {error:?}"));
        assert_eq!(response.status, 200);
    }
    (
        server,
        PrincipalClient::new(
            client,
            endpoint,
            Zeroizing::new("registry-integration-token".to_owned()),
        ),
    )
}

fn gateway(
    tls: &transport::Tls,
    store: Arc<layerx_platform_gateway::store::RedisStore>,
) -> transport::Listener {
    transport::Listener::start(Arc::clone(&tls.config), 0, move |stream| {
        if let Ok(request) = http::parse_client_request(stream) {
            if request.method == "GET" && request.path == "/readyz" {
                let ready = store.ready();
                let _ = http::write_response(
                    stream,
                    &http::json(
                        if ready { 200 } else { 503 },
                        &serde_json::json!({"ready": ready}),
                    ),
                );
                return;
            }
            let response = request
                .headers
                .get("authorization")
                .ok_or(layerx_platform_gateway::AccessError::Unauthenticated)
                .and_then(|authorization| {
                    layerx_platform_gateway::gateway_principal(&store, authorization)
                })
                .map_or_else(
                    |_| http::refusal(401, "api_key_required", None),
                    |value| http::json(200, &value),
                );
            let _ = http::write_response(stream, &response);
        }
    })
}

fn signing_service(tls: &transport::Tls, root: &Path) -> transport::Listener {
    let store = kms::KeyStore::open(
        &root.join("kms"),
        SealKey::derive(kms::SEAL_LABEL, b"producer-integration-seal-secret"),
    )
    .unwrap_or_else(|error| panic!("KMS store: {error}"));
    let service = kms::Service::new(store, Zeroizing::new("kms-integration".to_owned()), true);
    transport::Listener::start(Arc::clone(&tls.config), 0, move |stream| {
        if let Ok(mut request) = http::parse_client_request(stream) {
            request.peer_verified = stream
                .conn
                .peer_certificates()
                .is_some_and(|certs| !certs.is_empty());
            let _ = http::write_response(stream, &service.route(&request));
        }
    })
}

fn publication() -> String {
    let fixture = support::deploy_fixture(
        support::WASM_V1,
        layerx_programs_runtime::UpgradePolicy::Authority(support::AUTHORITY),
        70,
        support::NOW,
    );
    let verifier = support::verifier_for_fixture(&fixture, 1000);
    let evidence = verifier
        .verify_deployment(&fixture.proof, support::NOW)
        .unwrap_or_else(|error| panic!("verified publication: {error:?}"));
    serde_json::json!({"program_id":layerx_programs::hex::encode(&evidence.program().bytes()), "lifecycle":"active", "versions":[{"version":evidence.version(), "code_hash":layerx_programs::hex::encode(&evidence.code_hash()), "deployment_receipt_digest":layerx_programs::hex::encode(&evidence.receipt_digest())}]}).to_string()
}

#[test]
fn registry_publication_uses_identity_and_delivers_over_tls_with_durable_recovery() {
    let root = std::env::temp_dir().join(format!("registry-producer-tls-{}", std::process::id()));
    let tls = producer_tls(&root);
    let (_identity, identity) = identity(&root, &tls);
    let principal = identity
        .resolve("program-key:publication-integration")
        .unwrap_or_else(|error| panic!("identity resolution: {error:?}"));
    assert_eq!(principal, sha256_hex(b"program-principal"));
    assert!(identity.resolve("unknown-publication-key").is_err());
    let redis = producer_redis::RedisProcess::start();
    let store = Arc::new(redis.store());
    let secret = format!("lxp_live_{}", "12".repeat(32));
    store
        .issue_key(
            &layerx_platform_gateway::store::KeyRecord {
                key_id: "program-key".to_owned(),
                principal_digest: principal.clone(),
                salt: "program-salt".to_owned(),
                secret_digest: layerx_platform_gateway::gateway_digest(&[
                    b"gateway-key-v1",
                    b"program-salt",
                    secret.as_bytes(),
                ]),
                signer_public_key: "ab".repeat(32),
                scopes: "program:read".to_owned(),
                quota_requests: 1000,
                quota_window_seconds: 60,
                epoch: 1,
                disabled: false,
            },
            "program-issue",
        )
        .unwrap_or_else(|error| panic!("gateway key: {error:?}"));
    let gateway = gateway(&tls, store);
    qualify_delivery(&tls, &root, gateway.port, &secret, &principal);
}

fn source(
    tls: &transport::Tls,
    root: &Path,
    gateway_port: u16,
    secret: &str,
    port: u16,
) -> transport::Listener {
    let credential = Zeroizing::new(format!("program-key:{secret}"));
    let credential_path = root.join("source-principal.credential");
    let snapshot_path = root.join("source-enrollment.json");
    let enrollment_key = "0123456789abcdef0123456789abcdef";
    let snapshot = serde_json::json!({
        "version": ENROLLMENT_VERSION,
        "generation": 1,
        "principals": [{
            "principal": "program-principal",
            "credential_file": credential_path,
        }],
        "mac": enrollment_snapshot_mac(
            Kind::Program,
            1,
            &[("program-principal", credential.as_str())],
            enrollment_key,
        ),
    });
    for (path, contents) in [
        (&credential_path, credential.as_str().to_owned()),
        (&snapshot_path, snapshot.to_string()),
    ] {
        fs::write(path, contents)
            .unwrap_or_else(|error| panic!("source enrollment file: {error:?}"));
        fs::set_permissions(path, fs::Permissions::from_mode(0o600))
            .unwrap_or_else(|error| panic!("source enrollment permissions: {error:?}"));
    }
    let service = Service::open(
        Kind::Program,
        tls.upstream(gateway_port, "unused"),
        &snapshot_path,
        enrollment_key,
        Zeroizing::new("consumer-token".to_owned()),
        &root.join("source"),
    )
    .and_then(|service| {
        service.with_producers(vec![ProducerCredential {
            token: Zeroizing::new("producer-token".to_owned()),
            allow_principal_digest: true,
        }])
    })
    .unwrap_or_else(|error| panic!("program source: {error:?}"));
    service
        .refresh()
        .unwrap_or_else(|error| panic!("program source enrollment: {error:?}"));
    transport::Listener::start(Arc::clone(&tls.config), port, move |stream| {
        if let Ok(mut request) = http::parse_client_request(stream) {
            request.peer_verified = stream
                .conn
                .peer_certificates()
                .is_some_and(|certs| !certs.is_empty());
            let _ = http::write_response(stream, &service.route(&request));
        }
    })
}

fn qualify_delivery(
    tls: &transport::Tls,
    root: &Path,
    gateway_port: u16,
    secret: &str,
    principal: &str,
) {
    let source = |port| source(tls, root, gateway_port, secret, port);
    let sink = source(0);
    let kms = signing_service(tls, root);
    let mut webhook = Webhooks::start(root, sink.port, kms.port);
    let client = Client::new(
        BTreeMap::from([(
            "program".to_owned(),
            tls.upstream(sink.port, "producer-token"),
        )]),
        tls.upstream(webhook.port, "notification-token"),
    )
    .unwrap_or_else(|error| panic!("producer: {error:?}"));
    let outbox = ProgramOutbox::new(&root.join("registry"));
    outbox
        .enqueue_publication(&publication(), principal, support::NOW)
        .unwrap_or_else(|error| panic!("publication enqueue: {error:?}"));
    let first = outbox
        .pending()
        .unwrap_or_else(|error| panic!("pending: {error:?}"))
        .unwrap_or_else(|| panic!("publication"));
    let mut foreign = first.observation.clone();
    foreign.principal_digest = Some(sha256_hex(b"foreign-principal"));
    assert_eq!(
        tls.upstream(sink.port, "producer-token")
            .post(
                "/internal/v1/observe",
                &foreign
                    .encode()
                    .unwrap_or_else(|error| panic!("foreign encoding: {error:?}"))
            )
            .unwrap_or_else(|error| panic!("refusal: {error:?}"))
            .status,
        403
    );
    let port = sink.port;
    drop(sink);
    assert!(client.step(&outbox, &outbox.health).is_err());
    std::thread::sleep(Duration::from_secs(31));
    assert!(!outbox.health.ready());
    assert_eq!(
        outbox
            .pending()
            .unwrap_or_else(|error| panic!("queue retained: {error:?}")),
        Some(first.clone())
    );
    let sink = source(port);
    client
        .step(&outbox, &outbox.health)
        .unwrap_or_else(|error| panic!("recovered observation: {error:?}"));
    assert!(outbox.health.ready());
    let observed = outbox
        .pending()
        .unwrap_or_else(|error| panic!("observed: {error:?}"))
        .unwrap_or_else(|| panic!("notification"));
    assert!(observed.observed);
    webhook.stop();
    assert!(client.step(&outbox, &outbox.health).is_err());
    drop(outbox);
    let outbox = ProgramOutbox::new(&root.join("registry"));
    assert_eq!(
        outbox
            .pending()
            .unwrap_or_else(|error| panic!("restart pending: {error:?}")),
        Some(observed.clone())
    );
    webhook.restart();
    assert_eq!(client.deliver(&observed), Ok(false));
    assert_eq!(client.deliver(&observed), Ok(false));
    client
        .step(&outbox, &outbox.health)
        .unwrap_or_else(|error| panic!("notification ack: {error:?}"));
    assert!(outbox
        .pending()
        .unwrap_or_else(|error| panic!("delivered: {error:?}"))
        .is_none());
    let reply = tls
        .upstream(sink.port, "consumer-token")
        .get(&format!("/internal/v1/events/{}", first.observation.id))
        .unwrap_or_else(|error| panic!("event read: {error:?}"));
    assert_eq!(reply.status, 200);
    let record: layerx_platform_internal::events::Record =
        serde_json::from_slice(&reply.body).unwrap_or_else(|error| panic!("record: {error:?}"));
    assert_eq!(
        record,
        first.observation.record("program-principal".to_owned())
    );
}

struct Webhooks {
    redis: std::process::Child,
    child: Option<std::process::Child>,
    command: Command,
    pub port: u16,
}

impl Webhooks {
    fn start(root: &Path, program_source_port: u16, kms_port: u16) -> Self {
        let executable = service_binary("layerx-platform-webhooks", "layerx-webhooks");
        let port = free_port();
        let redis_port = free_port();
        fs::write(
            root.join("redis.acl"),
            "user default off\nuser producer on >integration-only ~* &* +@all\n",
        )
        .unwrap_or_else(|error| panic!("{error}"));
        fs::write(root.join("redis.conf"), format!("bind 127.0.0.1\nport 0\ntls-port {redis_port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\naclfile {}\nappendonly yes\nappendfsync always\ndir {}\n", root.join("cert.pem").display(), root.join("key.pem").display(), root.join("cert.pem").display(), root.join("redis.acl").display(), root.display())).unwrap_or_else(|error| panic!("{error}"));
        let redis = Command::new("redis-server")
            .arg(root.join("redis.conf"))
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::null())
            .spawn()
            .unwrap_or_else(|error| panic!("{error}"));
        let mut command = Command::new(executable);
        command
            .env_clear()
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::inherit());
        let endpoint = format!("https://localhost:{program_source_port}");
        for (name, value) in [
            ("ROLE", "ingress".to_owned()),
            ("LISTEN", format!("127.0.0.1:{port}")),
            ("REDIS_URL", format!("rediss://localhost:{redis_port}")),
            ("KMS_URL", format!("https://localhost:{kms_port}")),
            ("COMPONENT_URL", endpoint.clone()),
            ("AUTHORITY_URL", endpoint.clone()),
            ("INSTANCE_ID", "producer-integration".to_owned()),
            ("LXP_WIRE_VERSION", "3".to_owned()),
            ("NETWORK_ID", "77".to_owned()),
        ] {
            command.env(format!("LAYERX_WEBHOOKS_{name}"), value);
        }
        for (name, file) in [
            ("TLS_CERT_DER", "cert.der"),
            ("TLS_KEY_DER", "key.der"),
            ("INGRESS_CLIENT_CA_DER", "cert.der"),
            ("INTERNAL_CA_DER", "cert.der"),
            ("PUBLIC_CA_DER", "cert.der"),
            ("CLIENT_IDENTITY_PKCS12", "client.p12"),
        ] {
            command.env(format!("LAYERX_WEBHOOKS_{name}"), root.join(file));
        }
        for (name, value) in [
            ("CLIENT_IDENTITY_PASSWORD", "integration-only".to_owned()),
            ("REDIS_USERNAME", "producer".to_owned()),
            ("REDIS_PASSWORD", "integration-only".to_owned()),
            ("CURSOR_KEY", "11".repeat(32)),
            ("KMS_TOKEN", "kms-integration".to_owned()),
            ("COMPONENT_TOKEN", "component-integration".to_owned()),
            ("AUTHORITY_TOKEN", "authority-integration".to_owned()),
            ("SOURCE_TRIGGER_TOKEN", "notification-token".to_owned()),
            ("OPERATOR_TOKEN", "operator-integration".to_owned()),
            (
                "SEQUENCER_PUBLIC_KEY",
                "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a".to_owned(),
            ),
            ("SEQUENCER_ID", "11".repeat(32)),
            ("SEQUENCER_FIRST_BATCH", "0".to_owned()),
            ("SEQUENCER_LAST_BATCH", u64::MAX.to_string()),
        ] {
            fs::write(root.join(name), value).unwrap_or_else(|error| panic!("{error}"));
            command.env(format!("LAYERX_WEBHOOKS_{name}_FILE"), root.join(name));
        }
        fs::write(root.join("consumer-token"), "consumer-token")
            .unwrap_or_else(|error| panic!("{error}"));
        for kind in ["JOURNEY", "APPROVAL", "PROGRAM", "PAYMENT"] {
            command.env(
                format!("LAYERX_WEBHOOKS_{kind}_SOURCE_URL"),
                endpoint.clone(),
            );
            command.env(
                format!("LAYERX_WEBHOOKS_{kind}_SOURCE_TOKEN_FILE"),
                root.join("consumer-token"),
            );
        }
        let mut result = Self {
            redis,
            child: None,
            command,
            port,
        };
        result.restart();
        result
    }

    pub fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            let _ = child.kill();
            let _ = child.wait();
        }
    }

    pub fn restart(&mut self) {
        assert!(self.child.is_none());
        self.child = Some(
            self.command
                .spawn()
                .unwrap_or_else(|error| panic!("{error}")),
        );
        for _ in 0..100 {
            assert!(
                self.child
                    .as_mut()
                    .unwrap_or_else(|| panic!("child"))
                    .try_wait()
                    .unwrap_or_else(|error| panic!("child status: {error:?}"))
                    .is_none(),
                "webhook exited before listening"
            );
            if TcpStream::connect(("127.0.0.1", self.port)).is_ok() {
                return;
            }
            std::thread::sleep(Duration::from_millis(50));
        }
        panic!("webhook did not listen");
    }
}

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("{error}"))
        .local_addr()
        .unwrap_or_else(|error| panic!("{error}"))
        .port()
}

impl Drop for Webhooks {
    fn drop(&mut self) {
        self.stop();
        let _ = self.redis.kill();
        let _ = self.redis.wait();
    }
}

fn producer_tls(root: &Path) -> transport::Tls {
    let mut tls = transport::Tls::new(root);
    fs::write(
        root.join("producer.ext"),
        "basicConstraints=critical,CA:FALSE\nextendedKeyUsage=clientAuth\nsubjectAltName=URI:urn:layerx:webhooks:role:producer\n",
    )
    .unwrap_or_else(|error| panic!("producer certificate extensions: {error}"));
    for args in [
        vec![
            "x509",
            "-req",
            "-in",
            "client.csr",
            "-CA",
            "cert.pem",
            "-CAkey",
            "key.pem",
            "-CAcreateserial",
            "-out",
            "producer.pem",
            "-days",
            "2",
            "-extfile",
            "producer.ext",
        ],
        vec![
            "pkcs12",
            "-export",
            "-inkey",
            "client.key",
            "-in",
            "producer.pem",
            "-out",
            "producer.p12",
            "-passout",
            "pass:integration-only",
        ],
    ] {
        let output = Command::new("openssl")
            .current_dir(root)
            .args(args)
            .output()
            .unwrap_or_else(|error| panic!("producer certificate: {error}"));
        assert!(
            output.status.success(),
            "producer certificate failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
    }
    tls.identity = native_tls::Identity::from_pkcs12(
        &fs::read(root.join("producer.p12"))
            .unwrap_or_else(|error| panic!("producer identity: {error}")),
        "integration-only",
    )
    .unwrap_or_else(|error| panic!("producer identity: {error}"));
    tls
}
