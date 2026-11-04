use layerx_platform_authority::{authorized_batch_by_activity, parse_replica_evidence};
use layerx_platform_webhooks::{
    deployment::bind_network,
    encoding::{fixed_hex, hex_decode, hex_encode},
    http::{read_request, write_reply, Reply},
    trusted::AuthorityReceiptVerifier,
    WebhookError,
};
use rustls::pki_types::{PrivateKeyDer, PrivatePkcs8KeyDer};
use serde_json::{json, Value};
use std::{
    net::TcpListener,
    sync::Arc,
    thread::{self, JoinHandle},
    time::{Duration, Instant},
};

const NODE: &str = include_str!("../../node/deployment.yaml");
const WEBHOOKS: &str = include_str!("../deployment.yaml");

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn configured_network(manifest: &str) -> String {
    let bindings: Vec<_> = manifest
        .lines()
        .filter_map(|line| line.strip_prefix("  LAYERX_WEBHOOKS_NETWORK_ID: "))
        .collect();
    assert_eq!(bindings.len(), 1);
    if bindings[0].starts_with('"') {
        must(serde_json::from_str(bindings[0]))
    } else {
        bindings[0].to_owned()
    }
}

#[test]
fn finding_3_network_is_sourced_from_the_merged_authority_deployment() {
    let rendered = must(bind_network(NODE, WEBHOOKS));
    assert_eq!(configured_network(&rendered), "layerx-testnet");
    assert_eq!(configured_network(WEBHOOKS), configured_network(&rendered));
    assert_eq!(rendered.lines().count(), WEBHOOKS.lines().count());
    for (before, after) in WEBHOOKS.lines().zip(rendered.lines()) {
        if !before.starts_with("  LAYERX_WEBHOOKS_NETWORK_ID:") {
            assert_eq!(before, after);
        }
    }

    let merged = NODE.replace(
        "network-name: layerx-testnet",
        "network-name: merged-network",
    );
    assert_eq!(
        configured_network(&must(bind_network(&merged, WEBHOOKS))),
        "merged-network"
    );
    let block_style = merged.replace(
        "- {name: LAYERX_AUTHORITY_NETWORK_ID, valueFrom: {configMapKeyRef: {name: layerx-node-config, key: network-name}}}",
        "- name: LAYERX_AUTHORITY_NETWORK_ID\n              valueFrom:\n                configMapKeyRef:\n                  name: layerx-node-config\n                  key: network-name",
    );
    assert_ne!(block_style, merged);
    assert_eq!(
        configured_network(&must(bind_network(&block_style, WEBHOOKS))),
        "merged-network"
    );
    for quoted in [
        "'merged-network'",
        "\"merged-network\" # authority identity",
    ] {
        let quoted_node = merged.replace(
            "network-name: merged-network",
            &format!("network-name: {quoted}"),
        );
        assert_eq!(
            configured_network(&must(bind_network(&quoted_node, WEBHOOKS))),
            "merged-network"
        );
    }
    for invalid in [
        NODE.replace("  network-name: layerx-testnet\n", ""),
        NODE.replace("network-name: layerx-testnet", "network-name: bad/name"),
        NODE.replace("network-name: layerx-testnet", "network-name: layerx-testnet\n  network-name: other"),
        NODE.replace("key: network-name", "key: network-id"),
        format!("{NODE}\n---\napiVersion: v1\nkind: ConfigMap\nmetadata: {{name: layerx-node-config}}\ndata:\n  network-name: other\n"),
        NODE.replace("network-name: layerx-testnet", "network-name: ''"),
        NODE.replace("network-name: layerx-testnet", &format!("network-name: {}", "n".repeat(65))),
        NODE.replace("name: LAYERX_AUTHORITY_NETWORK_ID, valueFrom:", "name: LAYERX_AUTHORITY_NETWORK_ID, value: override, valueFrom:"),
    ] {
        assert!(bind_network(&invalid, WEBHOOKS).is_err(), "ambiguous authority configuration was accepted");
    }
    let overridden = format!("{WEBHOOKS}\n  LAYERX_WEBHOOKS_NETWORK_ID: override\n");
    assert!(bind_network(NODE, &overridden).is_err());
}

struct TlsMaterial {
    ca: Vec<u8>,
    identity: native_tls::Identity,
    server: Arc<rustls::ServerConfig>,
}

fn tls_material() -> TlsMaterial {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let ca_key = must(rcgen::KeyPair::generate());
    let mut ca_params = must(rcgen::CertificateParams::new(Vec::<String>::new()));
    ca_params.is_ca = rcgen::IsCa::Ca(rcgen::BasicConstraints::Unconstrained);
    ca_params
        .distinguished_name
        .push(rcgen::DnType::CommonName, "webhooks-test-ca");
    ca_params.key_usages = vec![
        rcgen::KeyUsagePurpose::KeyCertSign,
        rcgen::KeyUsagePurpose::CrlSign,
    ];
    let ca = must(ca_params.self_signed(&ca_key));
    let issuer = rcgen::Issuer::from_params(&ca_params, &ca_key);
    let leaf_key = must(rcgen::KeyPair::generate());
    let mut leaf_params = must(rcgen::CertificateParams::new(vec!["localhost".into()]));
    leaf_params
        .distinguished_name
        .push(rcgen::DnType::CommonName, "localhost");
    leaf_params.key_usages = vec![rcgen::KeyUsagePurpose::DigitalSignature];
    leaf_params.extended_key_usages = vec![
        rcgen::ExtendedKeyUsagePurpose::ServerAuth,
        rcgen::ExtendedKeyUsagePurpose::ClientAuth,
    ];
    let leaf = must(leaf_params.signed_by(&leaf_key, &issuer));
    let mut roots = rustls::RootCertStore::empty();
    must(roots.add(ca.der().clone()));
    let verifier = must(rustls::server::WebPkiClientVerifier::builder(Arc::new(roots)).build());
    let server = must(
        rustls::ServerConfig::builder()
            .with_client_cert_verifier(verifier)
            .with_single_cert(
                vec![leaf.der().clone()],
                PrivateKeyDer::from(PrivatePkcs8KeyDer::from(leaf_key.serialize_der())),
            ),
    );
    let chain = format!("{}{}", leaf.pem(), ca.pem());
    TlsMaterial {
        ca: ca.der().to_vec(),
        identity: must(native_tls::Identity::from_pkcs8(
            chain.as_bytes(),
            leaf_key.serialize_pem().as_bytes(),
        )),
        server: Arc::new(server),
    }
}

// The peer replays the retained authority transcript; the real verifier owns
// HTTPS, authentication, response decoding and cryptographic receipt validation.
fn serve_transcripts(
    tls: Arc<rustls::ServerConfig>,
    path: String,
    token: &'static str,
    bodies: Vec<String>,
) -> (String, JoinHandle<usize>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    must(listener.set_nonblocking(true));
    let endpoint = format!("https://localhost:{}", must(listener.local_addr()).port());
    let worker = thread::spawn(move || {
        let count = bodies.len();
        for body in bodies {
            let deadline = Instant::now() + Duration::from_secs(10);
            let socket = loop {
                match listener.accept() {
                    Ok((socket, _)) => break socket,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "transcript request timed out");
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("{error}"),
                }
            };
            must(socket.set_read_timeout(Some(Duration::from_secs(5))));
            must(socket.set_write_timeout(Some(Duration::from_secs(5))));
            let mut stream = rustls::StreamOwned::new(
                must(rustls::ServerConnection::new(Arc::clone(&tls))),
                socket,
            );
            let request = must(read_request(&mut stream));
            assert!(stream
                .conn
                .peer_certificates()
                .is_some_and(|certificates| !certificates.is_empty()));
            assert_eq!(request.method, "GET");
            assert_eq!(request.path, path);
            assert_eq!(
                request.header("authorization"),
                Some(format!("Bearer {token}").as_str())
            );
            assert!(request.body.is_empty());
            must(write_reply(&mut stream, &Reply::json(200, body)));
            stream.conn.send_close_notify();
            must(stream.conn.complete_io(&mut stream.sock));
        }
        count
    });
    (endpoint, worker)
}

#[test]
fn finding_3_authority_receipt_is_accepted_and_a_foreign_network_is_refused() {
    let network = configured_network(&must(bind_network(NODE, WEBHOOKS)));
    let fixture: Value = must(serde_json::from_str(include_str!(
        "../../gateway/tests/fixtures/maintained-authority.json"
    )));
    let receipt = must(hex_decode(fixture["receipt_hex"].as_str().unwrap()));
    let pin = must(layerx_platform_gateway::configured_sequencer(
        fixture["sequencer_id"].as_str().unwrap(),
        fixture["sequencer_public_key"].as_str().unwrap(),
        fixture["first_batch"].as_str().unwrap(),
        fixture["last_batch"].as_str().unwrap(),
    ));
    let activity = fixture["authority"]["activity_id"].as_str().unwrap();
    let evidence = must(parse_replica_evidence(
        &must(serde_json::to_vec(&json!({
            "authority_replica_id": "01".repeat(32),
            "sequencer_public_key": fixture["sequencer_public_key"],
            "batch_evidence": fixture["authority"]["batch_evidence"],
        }))),
        [1; 32],
        pin.public_key(),
    ));
    let facts = must(authorized_batch_by_activity(
        must(fixed_hex(activity)),
        &receipt,
        &evidence,
        &pin,
    ));
    let verified_header = must(layerx_proof::inclusion::verify_header(
        &evidence.header,
        &evidence.header_signature,
        &pin,
    ));
    let header = verified_header.header();
    let protocol_network = header.network_id();
    let component = must(serde_json::to_string(
        &json!({"activity_id": activity, "receipt": fixture["receipt_hex"]}),
    ));
    let mut response = fixture["authority"].clone();
    response["receipt"] = fixture["receipt_hex"].clone();
    response["protocol_network_id"] = json!(protocol_network);
    response["network_id"] = json!(network);
    // The retained envelope must agree with the real authority producer before
    // crossing the transport. The deployment name is independent of signed LXP network bytes.
    for (field, expected) in [
        ("batch_id", facts.batch_id),
        ("asset", facts.asset),
        ("previous_state_root", facts.previous_state_root),
        ("resulting_state_root", facts.resulting_state_root),
        ("sequencer_public_key", facts.sequencer_public_key),
    ] {
        assert_eq!(response[field], hex_encode(&expected));
    }
    let foreign_networks = ["testnet", "foreign-network", "layerx-testnet-other"];
    let mut authority_bodies = vec![must(serde_json::to_string(&response))];
    for foreign in foreign_networks {
        response["network_id"] = json!(foreign);
        authority_bodies.push(must(serde_json::to_string(&response)));
    }
    let tls = tls_material();
    let (component_url, component_worker) = serve_transcripts(
        Arc::clone(&tls.server),
        format!("/internal/v1/receipts/{activity}"),
        "component",
        vec![component; 4],
    );
    let (authority_url, authority_worker) = serve_transcripts(
        Arc::clone(&tls.server),
        format!("/internal/v1/activities/{activity}/authority"),
        "authority",
        authority_bodies,
    );
    let verifier = must(AuthorityReceiptVerifier::new(
        &component_url,
        &authority_url,
        "component".into(),
        "authority".into(),
        &tls.ca,
        tls.identity,
        pin,
        network,
        "3".into(),
        protocol_network,
    ));
    let operation = must(verifier.verify(activity));
    assert_eq!(operation.activity_id(), facts.activity_id);
    assert_eq!(operation.receipt(), receipt);
    let decoded = must(layerx_proof::receipt::verify_sequencer_signature(
        &receipt,
        layerx_platform_gateway::ReceiptKeys::HistoricalKernel(pin.public_key()),
    ));
    let protocol = decoded.protocol().unwrap();
    let result: Value = must(serde_json::from_slice(operation.response()));
    assert_eq!(result["batch_id"], hex_encode(&facts.batch_id));
    assert_eq!(
        result["state_root"],
        hex_encode(&protocol.resulting_state_root())
    );
    assert_eq!(facts.resulting_state_root, header.resulting_state_root());
    assert_ne!(
        protocol.resulting_state_root(),
        header.resulting_state_root()
    );
    assert_eq!(result["global_sequence"], facts.global_sequence);
    for foreign in foreign_networks {
        assert!(
            matches!(
                verifier.verify(activity),
                Err(WebhookError::VerificationRequired)
            ),
            "foreign network {foreign} must not establish receipt authority",
        );
    }
    assert_eq!(must(component_worker.join()), 4);
    assert_eq!(must(authority_worker.join()), 4);
}
