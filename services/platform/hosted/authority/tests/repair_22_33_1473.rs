include!("../src/main.rs");

use chain_readiness_tests::{base64, must, ReceiptHarness};
use serde_json::{json as document, Value};
use sha2::{Digest as _, Sha256};
use std::os::unix::fs::PermissionsExt as _;
use std::path::Path;

const README: &str = include_str!("../README.md");

fn native_case() -> Value {
    let fixture: Value = must(serde_json::from_str(include_str!(
        "fixtures/real-program-deploy-receipt.json"
    )));
    let bytes = must(hex::decode(fixture["receipt_hex"].as_str().unwrap()));
    let receipt = must(layerx_wire::receipt::decode(&bytes));
    let protocol = receipt.protocol().unwrap();
    assert!((1..=11).contains(&protocol.module_id()));
    let public = must(hex::decode32(
        fixture["sequencer_public_key_hex"].as_str().unwrap(),
    ));
    let key_id: [u8; 32] = Sha256::digest(public).into();
    // Preserve the real producer's receipt bytes and signature; adapt only the
    // recorded JSON-RPC envelope to the node's current key-identity convention.
    document!({
        "pinnedKey": {
            "keyId": format!("0x{}", hex::encode(&key_id)),
            "publicKey": format!("0x{}", hex::encode(&public)),
            "firstHeight": 1, "lastHeight": u64::MAX,
        },
        "receiptJson": {
            "txHash": format!("0x{}", hex::encode(&protocol.activity_id())),
            "rung": "receipted", "height": "0x2",
            "keyId": format!("0x{}", hex::encode(&key_id)),
            "signingKey": format!("0x{}", hex::encode(&public)),
            "receipt": format!("0x{}", hex::encode(&bytes)),
        },
        "signedReceipt": format!("0x{}", hex::encode(&bytes)),
    })
}

fn tls_config(harness: &ReceiptHarness) -> Arc<Config> {
    let config = &harness.config;
    Arc::new(Config {
        listen: config.listen,
        tls: config.tls.clone(),
        tokens: config.tokens.clone(),
        human: None,
        replica_address: config.replica_address,
        replica_host: config.replica_host.clone(),
        replica_token: config.replica_token.clone(),
        evidence_read_token: None,
        replica_id: config.replica_id,
        chain_receipts: must(ChainReceipts::load(
            &config.chain_rpc_url,
            &harness.directory.join("receipt-keys.json"),
        )),
        chain_rpc_url: config.chain_rpc_url.clone(),
        protocol_network_id: config.protocol_network_id,
        network_id: config.network_id.clone(),
        wire_version: config.wire_version.clone(),
        authorization: config.authorization,
        sequencer_public_key: config.sequencer_public_key,
    })
}

fn tls_get(config: Arc<Config>, path: &str, token: Option<&str>) -> (u16, Value) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let address = must(listener.local_addr());
    let worker = thread::spawn(move || {
        let (stream, _) = must(listener.accept());
        must(handle_connection(&config, stream, &AtomicBool::new(false)));
    });
    let mut roots = RootCertStore::empty();
    must(roots.add(CertificateDer::from(
        include_bytes!("../src/human/fixtures/ca.der").to_vec(),
    )));
    let client = rustls::ClientConfig::builder()
        .with_root_certificates(roots)
        .with_no_client_auth();
    let connection = must(rustls::ClientConnection::new(
        Arc::new(client),
        must(rustls::pki_types::ServerName::try_from("localhost")),
    ));
    let tcp = must(TcpStream::connect_timeout(&address, IO_TIMEOUT));
    must(tcp.set_read_timeout(Some(IO_TIMEOUT)));
    must(tcp.set_write_timeout(Some(IO_TIMEOUT)));
    let mut stream = StreamOwned::new(connection, tcp);
    let authorization = token.map_or_else(String::new, |token| {
        format!("Authorization: Bearer {token}\r\n")
    });
    must(write!(
        stream,
        "GET {path} HTTP/1.1\r\nHost: localhost\r\n{authorization}Connection: close\r\n\r\n"
    ));
    let mut answer = Vec::new();
    must(stream.read_to_end(&mut answer));
    worker.join().unwrap();
    let end = answer.windows(4).position(|b| b == b"\r\n\r\n").unwrap();
    let header = std::str::from_utf8(&answer[..end]).unwrap();
    let status = must(header.split_whitespace().nth(1).unwrap().parse());
    (status, must(serde_json::from_slice(&answer[end + 4..])))
}

#[test]
fn readme_matches_service_sources_and_configuration() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"));
    for path in [
        "src/main.rs",
        "src/chain_receipt.rs",
        "src/trust.rs",
        "src/committed.rs",
        "../registry/src/committed_head_route.rs",
        "src/human.rs",
        "src/human/dynamic.rs",
        "src/human/budget_state.rs",
        "src/protected.rs",
        "tests/chain_receipts.rs",
        "tests/human_tls.rs",
        "tests/budget_proof_exports.rs",
        "tests/maintenance_identity.rs",
        "tests/repair_22_31_207.rs",
        "tests/repair_22_31_208.rs",
        "tests/repair_22_33_1473.rs",
    ] {
        assert!(
            root.join(path).is_file(),
            "absent documented source: {path}"
        );
        assert!(
            README.contains(&format!("]({path})")),
            "unlinked source: {path}"
        );
    }
    for source in [
        USAGE,
        include_str!("../src/human.rs"),
        include_str!("../src/human/dynamic.rs"),
    ] {
        for variable in source.split(|c: char| !c.is_ascii_alphanumeric() && c != '_') {
            if variable.starts_with("LAYERX_AUTHORITY_") && variable.len() > 17 {
                assert!(
                    README.contains(variable),
                    "undocumented setting: {variable}"
                );
            }
        }
    }
    for absent in ["LAYERX_AUTHORITY_LNI_SOCKET", "real_node.rs"] {
        assert!(!README.contains(absent), "retired instruction: {absent}");
    }
    for contract in [
        "pax_sequencerReceipt",
        "http://127.0.0.1:PORT/evm",
        "`/comet`",
        "SHA-256",
        "256 KiB",
        "editing it requires a restart",
        "Human integration evidence is mandatory",
        "A header at H authenticates\nstate at H-1",
        "not an independently bootstrapped light client",
        committed_head_route::HEAD_PATH,
        "authority-readme-lni",
        "readme_matches_service_sources_and_configuration",
        "documented_receipt_transport_and_tls_refusals",
        "documented_protected_key_list_and_restart_rules",
    ] {
        assert!(README.contains(contract), "missing contract: {contract}");
    }
}

#[test]
fn documented_receipt_transport_and_tls_refusals() {
    let case = native_case();
    let mut harness = ReceiptHarness::start(std::slice::from_ref(&case), &case);
    let activity = chain_readiness_tests::hex_bytes(&case["receiptJson"]["txHash"])
        .try_into()
        .unwrap();
    let expected = chain_readiness_tests::hex_bytes(&case["signedReceipt"]);
    match lookup_receipt(&harness.config, activity, false) {
        ReceiptSource::Found(bytes) => assert_eq!(bytes, expected),
        other => panic!("production receipt not admitted: {other:?}"),
    }
    assert!(
        harness.config.chain_receipts.committed(activity).is_err(),
        "a signed execution receipt must not invent block inclusion"
    );
    let config = tls_config(&harness);
    let path = format!(
        "/v1/authorized-batches/by-activity/{}",
        hex::encode(&activity)
    );
    assert_eq!(tls_get(config.clone(), &path, None).0, 401);
    let (status, answer) = tls_get(config.clone(), &path, Some("chain-receipts-test-token"));
    assert_eq!(status, 503);
    assert_eq!(
        answer["error"]["code"], "replica_evidence_unavailable",
        "admitted receipt still requires independent replica evidence"
    );
    let (status, answer) = tls_get(config.clone(), "/readyz", None);
    assert_eq!(status, 200);
    assert_eq!(answer["ready"], true);
    let (status, answer) = tls_get(
        config.clone(),
        "/v1/agent/registry?tenant=t&principal=p",
        None,
    );
    assert_eq!(status, 503);
    assert_eq!(answer["error"]["code"], "human_authority_unconfigured");
    let unknown = format!(
        "/v1/authorized-batches/by-activity/{}",
        hex::encode(&[0x42; 32])
    );
    let (status, answer) = tls_get(config.clone(), &unknown, Some("chain-receipts-test-token"));
    assert_eq!(status, 404);
    assert_eq!(answer["error"]["code"], "unknown_activity");
    {
        let calls = must(harness.calls.lock());
        let receipts: Vec<_> = calls
            .iter()
            .filter(|call| call["method"] == "pax_sequencerReceipt")
            .collect();
        assert_eq!(receipts.len(), 4);
        for call in &receipts[..3] {
            assert_eq!(call["params"], document!([case["receiptJson"]["txHash"]]));
        }
    }
    harness.stop_chain();
    let (status, answer) = tls_get(config, &path, Some("chain-receipts-test-token"));
    assert_eq!(status, 503);
    assert_eq!(answer["error"]["code"], "receipt_source_unavailable");
}

#[test]
fn documented_protected_key_list_and_restart_rules() {
    let case = native_case();
    let mut harness = ReceiptHarness::start(std::slice::from_ref(&case), &case);
    let key_file = harness.directory.join("receipt-keys.json");
    let original = must(fs::read(&key_file));
    let key = harness.config.chain_receipts.pinned_keys()[0];
    assert_eq!(
        key.sequencer_id(),
        <[u8; 32]>::from(Sha256::digest(key.public_key()))
    );
    // Compatibility startup pins do not replace the actual receipt-key list.
    harness.config.sequencer_public_key = [0x42; 32];
    harness.config.authorization = SequencerAuthorization::new([0x43; 32], [0x42; 32], 1, 1);
    assert_eq!(must(trust::current(&harness.config)), vec![key]);
    let changed = document!([{
        "key_id": base64(&key.sequencer_id()),
        "public_key": base64(&key.public_key()),
        "first_height": 3, "last_height": 4,
    }]);
    must(fs::write(&key_file, changed.to_string()));
    assert_eq!(must(trust::current(&harness.config)), vec![key]);
    let activity = chain_readiness_tests::hex_bytes(&case["receiptJson"]["txHash"])
        .try_into()
        .unwrap();
    assert!(matches!(
        harness.config.chain_receipts.lookup(activity),
        ReceiptSource::Found(_)
    ));
    // Reopen the real key reader to model the documented startup reload boundary.
    harness.config.chain_receipts = must(ChainReceipts::load(
        &harness.config.chain_rpc_url,
        &key_file,
    ));
    assert_eq!(
        harness.config.chain_receipts.pinned_keys()[0].first_batch_number(),
        3
    );
    assert!(matches!(
        harness.config.chain_receipts.lookup(activity),
        ReceiptSource::Unavailable(_)
    ));

    let entry: Value = must(serde_json::from_slice::<Value>(&original))[0].clone();
    let mut unknown = entry.clone();
    unknown["unexpected"] = document!(true);
    let mut reversed = entry.clone();
    reversed["firstHeight"] = document!(5);
    reversed["lastHeight"] = document!(4);
    for invalid in [
        document!([]),
        document!([entry.clone(), entry]),
        document!([unknown]),
        document!([reversed]),
    ] {
        must(fs::write(&key_file, invalid.to_string()));
        assert!(ChainReceipts::load(&harness.config.chain_rpc_url, &key_file).is_err());
    }
    must(fs::write(&key_file, &original));
    must(fs::set_permissions(
        &key_file,
        fs::Permissions::from_mode(0o644),
    ));
    assert!(ChainReceipts::load(&harness.config.chain_rpc_url, &key_file).is_err());
    must(fs::set_permissions(
        &key_file,
        fs::Permissions::from_mode(0o600),
    ));
    assert!(ChainReceipts::load(&harness.config.chain_rpc_url, &key_file).is_ok());
    for url in [
        "https://127.0.0.1:9445/evm",
        "http://192.0.2.1:9445/evm",
        "http://localhost:0/evm",
    ] {
        assert!(
            ChainReceipts::load(url, &key_file).is_err(),
            "accepted {url}"
        );
    }
}
