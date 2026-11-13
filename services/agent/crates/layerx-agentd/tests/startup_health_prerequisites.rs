//! Task 22.33.1532, audit 22.30.94, acceptance 22998.1.
//! Finding `readme_startup` -> proving tests:
//! `readme_startup_documents_current_prerequisites_and_fresh_health_checks`;
//! `readme_startup_node_checks_chain_on_every_connection`;
//! `readme_startup_authority_reads_each_peer_and_refuses_invalid_registry`.
//! These exercise the production `NodeRpc` and `RemoteHumanAuthority` consumers
//! over loopback HTTP and authenticated TLS with typed registry production.
//! Baseline: no pre-change test failure is claimed. The explicit gate order
//! prohibits verification before complete implementation and reserves it for
//! hf-done. The initial runner ledger contained no build or verify runs.
//! Wiring requests: none; the proposal is entirely within the allowed paths.
//! Exact revision/command/exit/private log are retained by hf-build/hf-done.

fn require_err<T: std::fmt::Debug, E>(result: Result<T, E>, message: &str) -> E {
    result
        .err()
        .unwrap_or_else(|| panic!("{message}: expected an error"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use std::fmt::Debug;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::human::HumanPeer;
use layerx_agentd::human_node::HumanStateError;
use layerx_agentd::human_runtime::{HumanAuthorityBoundary, RemoteHumanAuthority};
use layerx_agentd::node_rpc::{
    NodeRpc, NodeRpcConfig, NodeRpcError, LAYERX_AGENT_NODE_CHAIN_ID, LAYERX_AGENT_NODE_RPC_URL,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use native_tls::{Identity, TlsAcceptor};
use openssl::asn1::Asn1Time;
use openssl::bn::{BigNum, MsbOption};
use openssl::hash::MessageDigest;
use openssl::pkey::{PKey, Private};
use openssl::rsa::Rsa;
use openssl::x509::extension::{
    BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName,
};
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};

const DEADLINE: Duration = Duration::from_secs(5);
const README: &str = include_str!("../README.md");

fn must<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("unexpected failure: {error:?}"))
}

fn listener() -> TcpListener {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    must(listener.set_nonblocking(true));
    listener
}

fn accept(listener: &TcpListener) -> TcpStream {
    let expires = Instant::now() + DEADLINE;
    let stream = loop {
        match listener.accept() {
            Ok((stream, _)) => break stream,
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                assert!(Instant::now() < expires, "missing dependency request");
                thread::sleep(Duration::from_millis(1));
            }
            Err(error) => panic!("accept failed: {error}"),
        }
    };
    must(stream.set_read_timeout(Some(DEADLINE)));
    must(stream.set_write_timeout(Some(DEADLINE)));
    stream
}

fn request(stream: &mut (impl Read + Write)) -> (String, Vec<(String, String)>, Vec<u8>) {
    let mut reader = BufReader::new(stream);
    let mut route = String::new();
    must(reader.read_line(&mut route));
    let mut headers = Vec::new();
    let mut length = 0;
    loop {
        let mut line = String::new();
        assert_ne!(must(reader.read_line(&mut line)), 0);
        if line == "\r\n" {
            break;
        }
        assert!(headers.len() < 64);
        let (name, value) = require_some(line.trim_end().split_once(':'), "required test value");
        if name.eq_ignore_ascii_case("content-length") {
            length = must(value.trim().parse::<usize>());
        }
        headers.push((name.to_ascii_lowercase(), value.trim().to_owned()));
    }
    assert!(length <= 4096);
    let mut body = vec![0; length];
    must(reader.read_exact(&mut body));
    (route, headers, body)
}

fn respond(stream: &mut impl Write, status: u16, body: &Value) {
    let body = must(serde_json::to_vec(body));
    must(write!(stream, "HTTP/1.1 {status} Response\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()));
    must(stream.write_all(&body));
    must(stream.flush());
}

fn node_peer() -> (NodeRpcConfig, JoinHandle<()>) {
    let listener = listener();
    let config = NodeRpcConfig {
        url: format!("http://{}/", must(listener.local_addr())),
        chain_id: 125,
        deadline: DEADLINE,
    };
    let server = thread::spawn(move || {
        for (status, result) in [
            (200, json!("0x7d")),
            (200, json!("0x7e")),
            (503, Value::Null),
        ] {
            let mut stream = accept(&listener);
            let (route, headers, body) = request(&mut stream);
            assert_eq!(route, "POST / HTTP/1.1\r\n");
            assert!(!headers.iter().any(|(name, _)| name == "authorization"));
            let value: Value = must(serde_json::from_slice(&body));
            assert_eq!(
                value,
                json!({"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]})
            );
            respond(
                &mut stream,
                status,
                &json!({"jsonrpc":"2.0","id":1,"result":result}),
            );
        }
    });
    (config, server)
}

#[test]
fn readme_startup_node_checks_chain_on_every_connection() {
    let (config, server) = node_peer();
    assert_eq!(must(NodeRpc::connect(&config)).chain_id(), 125);
    assert_eq!(
        require_err(NodeRpc::connect(&config), "required test value"),
        NodeRpcError::ChainIdMismatch {
            expected: 125,
            observed: 126
        }
    );
    assert_eq!(
        require_err(NodeRpc::connect(&config), "required test value"),
        NodeRpcError::Unavailable
    );
    must(server.join());
}

fn certificate(issuer: Option<(&X509, &PKey<Private>)>) -> (X509, PKey<Private>) {
    let key = must(PKey::from_rsa(must(Rsa::generate(2048))));
    let mut name = must(X509NameBuilder::new());
    must(name.append_entry_by_text(
        "CN",
        if issuer.is_some() {
            "localhost"
        } else {
            "readme startup CA"
        },
    ));
    let name = name.build();
    let mut builder = must(X509::builder());
    must(builder.set_version(2));
    let mut serial = must(BigNum::new());
    must(serial.rand(128, MsbOption::MAYBE_ZERO, false));
    must(builder.set_serial_number(must(serial.to_asn1_integer()).as_ref()));
    must(builder.set_subject_name(&name));
    must(builder.set_issuer_name(issuer.map_or(&name, |(ca, _)| ca.subject_name())));
    must(builder.set_pubkey(&key));
    must(builder.set_not_before(must(Asn1Time::days_from_now(0)).as_ref()));
    must(builder.set_not_after(must(Asn1Time::days_from_now(1)).as_ref()));
    if let Some((ca, signing_key)) = issuer {
        must(builder.append_extension(must(BasicConstraints::new().critical().build())));
        must(
            builder.append_extension(must(
                KeyUsage::new()
                    .digital_signature()
                    .key_encipherment()
                    .build(),
            )),
        );
        must(builder.append_extension(must(ExtendedKeyUsage::new().server_auth().build())));
        let san = must(
            SubjectAlternativeName::new()
                .dns("localhost")
                .build(&builder.x509v3_context(Some(ca), None)),
        );
        must(builder.append_extension(san));
        must(builder.sign(signing_key, MessageDigest::sha256()));
    } else {
        must(builder.append_extension(must(BasicConstraints::new().critical().ca().build())));
        must(
            builder.append_extension(must(
                KeyUsage::new()
                    .critical()
                    .key_cert_sign()
                    .crl_sign()
                    .build(),
            )),
        );
        must(builder.sign(&key, MessageDigest::sha256()));
    }
    (builder.build(), key)
}

#[test]
fn readme_startup_authority_reads_each_peer_and_refuses_invalid_registry() {
    let activity = must(ActivityType::new(ModuleId::Asset, 1));
    let registry = must(ModuleRegistry::new(&[must(ModuleRegistration::new(
        ModuleId::Asset,
        &[activity],
    ))]));
    // The fixture producer serializes the real registry's typed registrations.
    let body = json!({"modules": registry.registrations().iter().map(|registration| json!({
        "module_id": registration.module() as u16,
        "activity_types": registration.activity_types().iter().map(|activity| activity.value()).collect::<Vec<_>>()
    })).collect::<Vec<_>>()});
    let (ca, key) = certificate(None);
    let (leaf, leaf_key) = certificate(Some((&ca, &key)));
    let identity = must(Identity::from_pkcs8(
        &must(leaf.to_pem()),
        &must(leaf_key.private_key_to_pem_pkcs8()),
    ));
    let tls = must(TlsAcceptor::new(identity));
    let listener = listener();
    let endpoint = format!("https://localhost:{}", must(listener.local_addr()).port());
    let server = thread::spawn(move || {
        for (tenant, principal, status, body) in [
            ("tenant-a", "principal-a", 200, body.clone()),
            ("tenant-b", "principal-b", 200, body.clone()),
            (
                "tenant-a",
                "principal-a",
                503,
                json!({"error":"unavailable"}),
            ),
            ("tenant-a", "principal-a", 200, json!({"modules":[]})),
            ("tenant-a", "principal-a", 200, body),
        ] {
            let mut stream = must(tls.accept(accept(&listener)));
            let (route, headers, bytes) = request(&mut stream);
            assert_eq!(
                route,
                format!(
                    "GET /v1/agent/registry?tenant={tenant}&principal={principal} HTTP/1.1\r\n"
                )
            );
            assert!(bytes.is_empty());
            assert!(headers.contains(&(
                "authorization".to_owned(),
                format!("Bearer {}", "a".repeat(32))
            )));
            respond(&mut stream, status, &body);
        }
    });
    let authority = must(RemoteHumanAuthority::connect(
        &endpoint,
        "a".repeat(32),
        DEADLINE,
        4096,
        &must(ca.to_der()),
    ));
    let peer = |suffix: &str| HumanPeer {
        uid: 1000,
        tenant: format!("tenant-{suffix}"),
        principal: format!("principal-{suffix}"),
        subject: None,
    };
    assert_eq!(must(authority.registry(&peer("a"))), registry);
    assert_eq!(must(authority.registry(&peer("b"))), registry);
    assert_eq!(
        authority.registry(&peer("a")),
        Err(HumanStateError::Unverified)
    );
    assert_eq!(
        authority.registry(&peer("a")),
        Err(HumanStateError::Unverified)
    );
    assert_eq!(must(authority.registry(&peer("a"))), registry);
    must(server.join());
}

#[test]
fn readme_startup_documents_current_prerequisites_and_fresh_health_checks() {
    for key in [
        LAYERX_AGENT_NODE_RPC_URL,
        LAYERX_AGENT_NODE_CHAIN_ID,
        "LAYERX_AGENT_HUMAN_AUTHORITY_ENDPOINT",
        "LAYERX_AGENT_HUMAN_AUTHORITY_BEARER",
        "LAYERX_AGENT_HUMAN_AUTHORITY_CA_DER",
        "LAYERX_AGENT_HUMAN_AUTHORITY_MAX_BYTES",
        "LAYERX_AGENT_HUMAN_DEADLINE_MS",
        "LAYERX_AGENT_HUMAN_PEERS",
    ] {
        assert!(
            README.contains(&format!("| `{key}` |")),
            "missing prerequisite {key}"
        );
    }
    for claim in [
        "No LNI connection is\nrequired.",
        "checks `eth_chainId` and refuses a\nmismatch",
        "authority registry read for every configured peer",
        "Each admitted probe reconnects to the node and queries the registry again",
        "node failure before contacting the authority",
        "200 with `{\"ready\":true}`",
        "503 with `{\"ready\":false}`",
    ] {
        assert!(
            README.contains(claim),
            "missing startup/health contract: {claim}"
        );
    }
    assert!(!README.contains("LAYERX_AGENT_HUMAN_LNI"));
    assert!(!README.contains("fresh node LNI handshake"));
}
