use layerx_agentd::human::{HumanOperationError, HumanPeer};
use layerx_agentd::human_runtime::{HumanAuthorityBoundary, RemoteHumanAuthority};
use layerx_types::{ids::Did, verify::VerificationLevel};
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
use std::io::{Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::PermissionsExt;
use std::path::Path;
use std::time::{Duration, Instant};

#[path = "support/committed_authority_documents.rs"]
mod committed_documents;

// Execute the in-crate proof verifier tests in this declared integration gate too.
pub use layerx_agentd::budget::ProtocolBudgetState;
pub use layerx_agentd::read;
#[path = "../src/budget/budget_proof.rs"]
mod budget_proof;

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
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

fn documents() -> Value {
    let repository = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let path = committed_documents::corpus_path(&repository);
    let metadata = must(std::fs::metadata(&path));
    assert!(metadata.is_file());
    assert_eq!(metadata.permissions().mode() & 0o077, 0);
    must(serde_json::from_slice(&must(std::fs::read(path))))
}

#[test]
fn repair_22_31_535_remote_consumers_require_committed_producer_documents() {
    let corpus = documents();
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    must(listener.set_nonblocking(true));
    let endpoint = format!("https://localhost:{}", must(listener.local_addr()).port());
    let (ca, key) = certificate(None);
    let (leaf, leaf_key) = certificate(Some((&ca, &key)));
    let identity = must(Identity::from_pkcs8(
        &must(leaf.to_pem()),
        &must(leaf_key.private_key_to_pem_pkcs8()),
    ));
    let tls = must(TlsAcceptor::new(identity));
    let produced = corpus.clone();
    let server = std::thread::spawn(move || {
        let mut requests = 0;
        for rank in [
            VerificationLevel::COMMITTED.wire_rank(),
            4,
            5,
            0,
            1,
            2,
            3,
            7,
            u8::MAX,
        ] {
            for (name, route) in [
                ("budget", "budget-state"),
                ("policy", "key-policy"),
                ("scope", "capability-scope"),
            ] {
                let deadline = Instant::now() + Duration::from_secs(10);
                let tcp = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(
                                Instant::now() < deadline,
                                "missing {route} request for rank {rank}"
                            );
                            std::thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("accept: {error}"),
                    }
                };
                must(tcp.set_read_timeout(Some(Duration::from_secs(5))));
                must(tcp.set_write_timeout(Some(Duration::from_secs(5))));
                let mut stream = must(tls.accept(tcp));
                let mut request = Vec::new();
                while !request.ends_with(b"\r\n\r\n") {
                    let mut byte = [0];
                    must(stream.read_exact(&mut byte));
                    request.push(byte[0]);
                    assert!(request.len() < 16384);
                }
                let request = must(std::str::from_utf8(&request));
                assert!(
                    request.starts_with(&format!(
                        "GET /v1/agent/{route}?tenant=repair-tenant&principal=repair-principal&"
                    )),
                    "{request}"
                );
                assert!(request
                    .to_ascii_lowercase()
                    .contains(&format!("authorization: bearer {}", "a".repeat(32))));
                let mut document = produced[name].clone();
                if rank != VerificationLevel::COMMITTED.wire_rank() {
                    document["verification"] = json!(rank);
                }
                let body = must(serde_json::to_vec(&document));
                must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()));
                must(stream.write_all(&body));
                requests += 1;
            }
        }
        requests
    });
    let mut authority = must(RemoteHumanAuthority::connect(
        &endpoint,
        "a".repeat(32),
        Duration::from_secs(5),
        1_048_576,
        &must(ca.to_der()),
    ));
    let peer = HumanPeer {
        uid: 1000,
        tenant: "repair-tenant".into(),
        principal: "repair-principal".into(),
        subject: None,
    };
    let id = |value: &Value| -> [u8; 32] {
        must(hex::decode(value.as_str().unwrap()))
            .try_into()
            .unwrap()
    };
    let policy_did = must(Did::new(corpus["policy_did"].as_str().unwrap().as_bytes()));
    let scope_did = must(Did::new(corpus["scope_did"].as_str().unwrap().as_bytes()));
    for rank in [
        VerificationLevel::COMMITTED.wire_rank(),
        4,
        5,
        0,
        1,
        2,
        3,
        7,
        u8::MAX,
    ] {
        let budget = authority.budget_state(&peer, id(&corpus["budget_id"]));
        let policy = authority.key_rotation_policy(&peer, &policy_did, false);
        let scope = authority.capability_scope(
            &peer,
            &scope_did,
            id(&corpus["scope"]["authority"]),
            id(&corpus["scope"]["action_key"]),
            id(&corpus["scope"]["capability_id"]),
        );
        if rank == VerificationLevel::COMMITTED.wire_rank() {
            let budget = must(budget);
            let policy = must(policy);
            let scope = must(scope);
            for label in [budget.verification, policy.verification, scope.verification] {
                assert_eq!(label, rank);
                assert_eq!(
                    VerificationLevel::from_wire_rank(label),
                    Ok(VerificationLevel::COMMITTED)
                );
            }
            assert_eq!(
                budget.remaining.to_string(),
                corpus["budget"]["remaining"].as_str().unwrap()
            );
            assert_eq!(
                policy.policy_revision,
                corpus["policy"]["policy_revision"].as_u64().unwrap()
            );
            assert_eq!(
                scope.observed_sequence,
                corpus["scope"]["observed_sequence"].as_u64().unwrap()
            );
        } else {
            assert!(
                matches!(budget, Err(HumanOperationError::Refused)),
                "budget rank {rank}"
            );
            assert!(
                matches!(policy, Err(HumanOperationError::Refused)),
                "policy rank {rank}"
            );
            assert!(
                matches!(scope, Err(HumanOperationError::Refused)),
                "scope rank {rank}"
            );
        }
    }
    assert_eq!(must(server.join()), 27);
}
