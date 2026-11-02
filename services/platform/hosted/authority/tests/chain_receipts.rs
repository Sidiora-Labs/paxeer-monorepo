include!("../src/main.rs");

use ed25519_dalek::Signer as _;
use sha2::Digest as _;
use std::os::unix::fs::{FileTypeExt as _, PermissionsExt as _};
use std::process::Command;

use chain_readiness_tests::{base64, hex_bytes, must, receipt_vectors, vector, ReceiptHarness};

struct AuthorityProcess {
    child: std::process::Child,
    address: SocketAddr,
}

impl AuthorityProcess {
    fn start(harness: &ReceiptHarness) -> Self {
        Self::start_with_replica(harness, None)
    }

    fn start_with_replica(harness: &ReceiptHarness, replica: Option<SocketAddr>) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        drop(listener);
        let replica_address = replica.unwrap_or_else(|| {
            let listener = must(TcpListener::bind("127.0.0.1:0"));
            must(listener.local_addr())
        });
        let token = harness.directory.join("authority-token");
        let replica_token = harness.directory.join("replica-token");
        for (path, value) in [
            (&token, "chain-receipts-test-token"),
            (&replica_token, "replica-test-token"),
        ] {
            must(fs::write(path, value));
            must(fs::set_permissions(path, fs::Permissions::from_mode(0o600)));
        }
        let log = must(fs::File::create(harness.directory.join("authority.log")));
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-receipt-authority"));
        command
            .env_clear()
            .envs(
                env::vars_os()
                    .filter(|(name, _)| !name.to_string_lossy().starts_with("LAYERX_AUTHORITY_")),
            )
            .env("LAYERX_AUTHORITY_LISTEN", address.to_string())
            .env(
                "LAYERX_AUTHORITY_TLS_CERT_DER",
                harness.directory.join("tls-cert.der"),
            )
            .env(
                "LAYERX_AUTHORITY_TLS_KEY_DER",
                harness.directory.join("tls-key.der"),
            )
            .env("LAYERX_AUTHORITY_TOKEN_FILES", token)
            .env(
                "LAYERX_AUTHORITY_REPLICA_URL",
                format!("http://{replica_address}"),
            )
            .env("LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE", replica_token)
            .env("LAYERX_AUTHORITY_REPLICA_ID", hex::encode(&[1; 32]))
            .env(
                "LAYERX_AUTHORITY_CHAIN_RPC_URL",
                format!("http://{}/evm", harness.config.replica_address),
            )
            .env(
                "LAYERX_AUTHORITY_RECEIPT_KEYS_FILE",
                harness.directory.join("receipt-keys.json"),
            )
            .env("LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID", "125")
            .env("LAYERX_AUTHORITY_NETWORK_ID", "chain-receipts-test")
            .env(
                "LAYERX_AUTHORITY_SEQUENCER_ID",
                hex::encode(&harness.config.authorization.sequencer_id()),
            )
            .env(
                "LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY",
                hex::encode(&harness.config.sequencer_public_key),
            )
            .env("LAYERX_AUTHORITY_FIRST_BATCH", "1")
            .env("LAYERX_AUTHORITY_LAST_BATCH", u64::MAX.to_string())
            .stdout(must(log.try_clone()))
            .stderr(log)
            .env_remove("LAYERX_AUTHORITY_LNI_SOCKET");
        assert!(command
            .get_envs()
            .all(|(name, value)| name != "LAYERX_AUTHORITY_LNI_SOCKET" || value.is_none()));
        let child = must(command.spawn());
        let mut process = Self { child, address };
        let deadline = Instant::now() + Duration::from_secs(5);
        loop {
            if TcpStream::connect(address).is_ok() {
                return process;
            }
            assert!(
                must(process.child.try_wait()).is_none(),
                "authority exited during startup: {}",
                must(fs::read_to_string(harness.directory.join("authority.log")))
            );
            assert!(Instant::now() < deadline, "authority did not start");
            thread::sleep(Duration::from_millis(10));
        }
    }

    fn receipt(&self, activity: &str) -> (u16, serde_json::Value) {
        self.get(&format!("/v1/authorized-batches/by-activity/{activity}"))
    }

    fn get(&self, path: &str) -> (u16, serde_json::Value) {
        let output = must(
            Command::new("curl")
                .args([
                    "--insecure",
                    "--silent",
                    "--show-error",
                    "--max-time",
                    "3",
                    "--header",
                    "Authorization: Bearer chain-receipts-test-token",
                    "--write-out",
                    "\n%{http_code}",
                ])
                .arg(format!("https://{}{path}", self.address))
                .output(),
        );
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        let text = must(String::from_utf8(output.stdout));
        let (body, status) = text.rsplit_once('\n').unwrap();
        (must(status.parse()), must(serde_json::from_str(body)))
    }
}

impl Drop for AuthorityProcess {
    fn drop(&mut self) {
        must(self.child.kill());
        must(self.child.wait());
    }
}

#[test]
fn receipt_lookup_reads_the_sequencer_receipt() {
    let document = receipt_vectors();
    let case = vector(&document, "V1-pax-transfer");
    let mut harness = ReceiptHarness::start(&[case.clone()], &case);
    let hash: [u8; 32] = hex_bytes(&case["receiptJson"]["txHash"])
        .try_into()
        .unwrap();
    let expected = hex_bytes(&case["signedReceipt"]);
    match lookup_receipt(&harness.config, hash, false) {
        ReceiptSource::Found(bytes) => assert_eq!(bytes, expected),
        other => panic!("pinned signed receipt was not admitted: {other:?}"),
    }
    assert!(!harness.directory.join("absent-lni.sock").exists());
    assert!(
        matches!(lookup_receipt(&harness.config, hash, true), ReceiptSource::Found(bytes) if bytes == expected)
    );
    let missing = [0x42; 32];
    assert!(matches!(
        lookup_receipt(&harness.config, missing, false),
        ReceiptSource::Unknown
    ));
    let absent = by_activity(&harness.config, &hex::encode(&missing), false);
    assert_eq!(absent.status, 404);
    let absent: serde_json::Value = must(serde_json::from_slice(&absent.body));
    assert_eq!(absent["error"]["code"], "unknown_activity");
    let calls = must(harness.calls.lock());
    assert_eq!(calls.len(), 4);
    assert_eq!(
        calls[0]["params"],
        serde_json::json!([case["receiptJson"]["txHash"]])
    );
    drop(calls);
    harness.stop_chain();
    assert!(matches!(
        lookup_receipt(&harness.config, hash, false),
        ReceiptSource::Unavailable(_)
    ));
    let unavailable = by_activity(&harness.config, &hex::encode(&hash), false);
    assert_eq!(unavailable.status, 503);
    let unavailable: serde_json::Value = must(serde_json::from_slice(&unavailable.body));
    assert_eq!(unavailable["error"]["code"], "receipt_source_unavailable");
}

#[test]
fn receipt_signed_by_an_unpinned_key_is_refused() {
    let document = receipt_vectors();
    let pinned = vector(&document, "V1-pax-transfer");
    let mut unpinned = vector(&document, "V3-key-not-pinned");
    let outside_range = vector(&document, "V3-height-outside-key-range");
    let mut receipt = hex_bytes(&unpinned["signedReceipt"]);
    let signer = ed25519_dalek::SigningKey::from_bytes(&[0x8c; 32]);
    let unsigned = must(layerx_wire::receipt::encode_unsigned(&must(
        layerx_wire::receipt::decode(&receipt),
    )));
    let digest = must(layerx_wire::hash::receipt_digest(&unsigned));
    let signature = signer.sign(&digest).to_bytes();
    let signature_offset = receipt.len() - signature.len();
    receipt[signature_offset..].copy_from_slice(&signature);
    unpinned["receiptJson"]["receipt"] = serde_json::json!(format!("0x{}", hex::encode(&receipt)));
    unpinned["signedReceipt"] = unpinned["receiptJson"]["receipt"].clone();
    unpinned["pinnedKey"]["publicKey"] = serde_json::json!(format!(
        "0x{}",
        hex::encode(&signer.verifying_key().to_bytes())
    ));
    assert_ne!(
        unpinned["pinnedKey"]["publicKey"],
        pinned["pinnedKey"]["publicKey"]
    );
    let own_key = SequencerAuthorization::new(
        hex_bytes(&unpinned["receiptJson"]["keyId"])
            .try_into()
            .unwrap(),
        hex_bytes(&unpinned["pinnedKey"]["publicKey"])
            .try_into()
            .unwrap(),
        1,
        u64::MAX,
    );
    assert!(
        layerx_proof::receipt::verify_sequencer_signature(
            &receipt,
            layerx_proof::receipt::ReceiptKeys::HeightRanges(&[own_key]),
        )
        .is_ok(),
        "the refused receipt must carry a real valid signature"
    );
    let harness = ReceiptHarness::start(&[unpinned.clone(), outside_range.clone()], &pinned);
    let before = harness.files();
    let hash: [u8; 32] = hex_bytes(&unpinned["receiptJson"]["txHash"])
        .try_into()
        .unwrap();
    assert!(matches!(
        lookup_receipt(&harness.config, hash, false),
        ReceiptSource::KeyMismatch
    ));
    let refused = by_activity(&harness.config, &hex::encode(&hash), false);
    assert_eq!(refused.status, 502);
    let refused: serde_json::Value = must(serde_json::from_slice(&refused.body));
    assert_eq!(refused["error"]["code"], "sequencer_key_mismatch");
    assert!(matches!(
        lookup_receipt(&harness.config, hash, true),
        ReceiptSource::KeyMismatch
    ));
    let outside: [u8; 32] = hex_bytes(&outside_range["receiptJson"]["txHash"])
        .try_into()
        .unwrap();
    assert!(matches!(
        lookup_receipt(&harness.config, outside, false),
        ReceiptSource::KeyMismatch
    ));
    assert_eq!(
        harness.files(),
        before,
        "refused receipts must not be persisted"
    );
    assert_eq!(
        must(harness.calls.lock()).len(),
        4,
        "every lookup reads the chain and no refused receipt is cached"
    );
}

#[test]
fn trust_accepts_genesis_receipt_key_ids() {
    let document = receipt_vectors();
    let template = vector(&document, "V1-pax-transfer");
    let original_bytes = hex_bytes(&template["signedReceipt"]);
    let decoded = must(layerx_wire::receipt::decode(&original_bytes));
    let original = decoded.protocol().unwrap();
    let signer = ed25519_dalek::SigningKey::from_bytes(&[0x51; 32]);
    let public_key = signer.verifying_key().to_bytes();
    let key_id = <[u8; 32]>::from(sha2::Sha256::digest(public_key));
    let handover_id = must(layerx_wire::handover::sequencer_id(&public_key));
    assert_ne!(key_id, handover_id);
    let mut cases = Vec::new();
    for (index, seed) in [0x51, 0x52, 0x53].into_iter().enumerate() {
        let signing_key = ed25519_dalek::SigningKey::from_bytes(&[seed; 32]);
        let claimed_public_key = if index == 1 {
            signing_key.verifying_key().to_bytes()
        } else {
            public_key
        };
        let claimed_id = <[u8; 32]>::from(sha2::Sha256::digest(claimed_public_key));
        let activity = [0x41 + index as u8; 32];
        let mut bytes = original_bytes.clone();
        for (old, new) in [
            (original.context_hash(), claimed_id),
            (original.activity_id(), activity),
        ] {
            let offset = bytes.windows(32).position(|value| value == old).unwrap();
            bytes[offset..offset + 32].copy_from_slice(&new);
        }
        let unsigned = must(layerx_wire::receipt::encode_unsigned(&must(
            layerx_wire::receipt::decode(&bytes),
        )));
        let signature = signing_key.sign(&must(layerx_wire::hash::receipt_digest(&unsigned)));
        let offset = bytes.len() - 64;
        bytes[offset..].copy_from_slice(&signature.to_bytes());
        let mut case = template.clone();
        case["pinnedKey"]["publicKey"] =
            serde_json::json!(format!("0x{}", hex::encode(&claimed_public_key)));
        case["pinnedKey"]["keyId"] = serde_json::json!(format!("0x{}", hex::encode(&claimed_id)));
        case["signedReceipt"] = serde_json::json!(format!("0x{}", hex::encode(&bytes)));
        case["receiptJson"]["receipt"] = case["signedReceipt"].clone();
        case["receiptJson"]["keyId"] = case["pinnedKey"]["keyId"].clone();
        case["receiptJson"]["txHash"] = serde_json::json!(format!("0x{}", hex::encode(&activity)));
        cases.push(case);
    }
    let mut harness = ReceiptHarness::start(&cases, &cases[0]);
    let pin = SequencerAuthorization::new(key_id, public_key, 1000, 1999);
    assert_eq!(must(trust::current(&harness.config)), vec![pin]);
    let receipt = hex_bytes(&cases[0]["signedReceipt"]);
    assert_eq!(
        must(layerx_wire::receipt::decode(&receipt))
            .protocol()
            .unwrap()
            .context_hash(),
        key_id
    );
    match lookup_receipt(&harness.config, [0x41; 32], false) {
        ReceiptSource::Found(bytes) => assert_eq!(bytes, receipt),
        other => panic!("genesis receipt key was not admitted: {other:?}"),
    }
    for activity in [[0x42; 32], [0x43; 32]] {
        assert!(matches!(
            lookup_receipt(&harness.config, activity, false),
            ReceiptSource::KeyMismatch
        ));
    }
    harness.config.protocol_network_id = 0;
    assert!(trust::current(&harness.config).is_err());
    harness.config.protocol_network_id = 125;

    let noncanonical = [0xff; 32];
    assert!(layerx_wire::handover::sequencer_id(&noncanonical).is_err());
    for (id, key) in [
        (handover_id, public_key),
        (
            <[u8; 32]>::from(sha2::Sha256::digest(noncanonical)),
            noncanonical,
        ),
    ] {
        let keys_file = harness.directory.join("receipt-keys.json");
        let entries = serde_json::json!([{
            "keyId": base64(&id), "publicKey": base64(&key),
            "firstHeight": 1000, "lastHeight": 1999,
        }]);
        must(fs::write(&keys_file, entries.to_string()));
        harness.config.chain_receipts = must(ChainReceipts::load(
            &harness.config.chain_rpc_url,
            &keys_file,
        ));
        assert!(trust::current(&harness.config).is_err());
    }
}

#[test]
fn trust_snapshot_is_the_pinned_key_list() {
    use std::os::unix::net::UnixListener;

    let document = receipt_vectors();
    let template = vector(&document, "V1-pax-transfer");
    let original_bytes = hex_bytes(&template["signedReceipt"]);
    let decoded = must(layerx_wire::receipt::decode(&original_bytes));
    let original = decoded.protocol().unwrap();
    let mut cases = Vec::new();
    let mut pins = Vec::new();
    let mut expected_keys = Vec::new();
    let mut headers = Vec::new();
    let native_fixture: serde_json::Value = must(serde_json::from_str(include_str!(
        "fixtures/real-program-deploy-receipt.json"
    )));
    for (index, seed) in [0x51, 0x52, 0x53].into_iter().enumerate() {
        let signer = ed25519_dalek::SigningKey::from_bytes(&[seed; 32]);
        let public_key = signer.verifying_key().to_bytes();
        let key_id = <[u8; 32]>::from(sha2::Sha256::digest(public_key));
        let pin = serde_json::json!({"pinnedKey": {
            "keyId": format!("0x{}", hex::encode(&key_id)),
            "publicKey": format!("0x{}", hex::encode(&public_key)),
            "firstHeight": 0, "lastHeight": u64::MAX,
        }});
        if index < 2 {
            pins.push(pin.clone());
            expected_keys.push(SequencerAuthorization::new(key_id, public_key, 0, u64::MAX));
        }
        for (height_index, height) in [0, 1, 1000, u64::MAX].into_iter().enumerate() {
            let mut bytes = original_bytes.clone();
            let mut batch = [0; 32];
            batch[24..].copy_from_slice(&height.to_be_bytes());
            let activity = [0x31 + (index * 4 + height_index) as u8; 32];
            for (old, new) in [
                (original.batch_id(), batch),
                (original.context_hash(), key_id),
                (original.activity_id(), activity),
            ] {
                let offset = bytes.windows(32).position(|value| value == old).unwrap();
                bytes[offset..offset + 32].copy_from_slice(&new);
            }
            let unsigned = must(layerx_wire::receipt::encode_unsigned(&must(
                layerx_wire::receipt::decode(&bytes),
            )));
            let signature = signer.sign(&must(layerx_wire::hash::receipt_digest(&unsigned)));
            let offset = bytes.len() - 64;
            bytes[offset..].copy_from_slice(&signature.to_bytes());
            let mut case = template.clone();
            case["pinnedKey"] = pin["pinnedKey"].clone();
            case["signedReceipt"] = serde_json::json!(format!("0x{}", hex::encode(&bytes)));
            case["receiptJson"]["receipt"] = case["signedReceipt"].clone();
            case["receiptJson"]["txHash"] =
                serde_json::json!(format!("0x{}", hex::encode(&activity)));
            case["receiptJson"]["keyId"] = pin["pinnedKey"]["keyId"].clone();
            case["receiptJson"]["height"] = serde_json::json!(format!("0x{height:x}"));
            cases.push(case);
            let mut header = must(hex::decode(native_fixture["header_hex"].as_str().unwrap()));
            let original_header = must(layerx_wire::receipt::decode_batch_header(&header));
            let offset = header
                .windows(32)
                .position(|bytes| bytes == original_header.sequencer_id())
                .unwrap();
            header[offset..offset + 32].copy_from_slice(&key_id);
            header[9..13].copy_from_slice(&125_u32.to_be_bytes());
            header[23..31].copy_from_slice(&height.to_be_bytes());
            let signature = signer
                .sign(&must(layerx_wire::hash::batch_header_digest(&header)))
                .to_bytes();
            headers.push((header, signature));
        }
    }
    let mut harness = ReceiptHarness::start_with_pins(&cases, &pins);
    let kernel = must(UnixListener::bind(
        harness.directory.join("absent-lni.sock"),
    ));
    must(kernel.set_nonblocking(true));
    let initial_key = harness.config.sequencer_public_key;
    harness.config.sequencer_public_key = [0x73; 32];
    assert!(!expected_keys
        .iter()
        .any(|key| key.public_key() == harness.config.sequencer_public_key));
    assert_eq!(harness.config.chain_receipts.pinned_keys(), expected_keys);
    let expected = expected_keys.clone();
    for (index, (header, signature)) in headers.iter().enumerate() {
        if index < 8 {
            assert_eq!(
                must(trust::authorization(&harness.config, header, signature)),
                expected[index / 4]
            );
            assert!(trust::authorization(&harness.config, header, &[0; 64]).is_err());
        } else {
            assert!(trust::authorization(&harness.config, header, signature).is_err());
        }
    }
    let unpinned = cases.split_off(8);
    assert_eq!(must(trust::current(&harness.config)), expected.clone());
    for key in &expected_keys {
        assert_eq!(
            must(trust::snapshot(&harness.config, key.public_key())),
            expected.clone()
        );
    }
    for claimed in [[0; 32], harness.config.sequencer_public_key] {
        assert!(trust::snapshot(&harness.config, claimed).is_err());
    }
    for case in &cases {
        let activity = hex_bytes(&case["receiptJson"]["txHash"])
            .try_into()
            .unwrap();
        let expected = hex_bytes(&case["signedReceipt"]);
        assert!(matches!(lookup_receipt(&harness.config, activity, false),
            ReceiptSource::Found(bytes) if bytes == expected));
    }
    assert_eq!(must(harness.calls.lock()).len(), cases.len());
    for case in &unpinned {
        let activity = hex_bytes(&case["receiptJson"]["txHash"])
            .try_into()
            .unwrap();
        assert!(matches!(
            lookup_receipt(&harness.config, activity, false),
            ReceiptSource::KeyMismatch
        ));
    }
    {
        let authority = AuthorityProcess::start(&harness);
        for case in &cases {
            let activity = case["receiptJson"]["txHash"]
                .as_str()
                .unwrap()
                .strip_prefix("0x")
                .unwrap();
            let (status, body) = authority.receipt(activity);
            assert_eq!(status, 503);
            assert_eq!(
                body["error"]["code"], "replica_unavailable",
                "a pinned receipt is admitted before the independent replica evidence is requested"
            );
        }
        for case in &unpinned {
            let activity = case["receiptJson"]["txHash"]
                .as_str()
                .unwrap()
                .strip_prefix("0x")
                .unwrap();
            let (status, body) = authority.receipt(activity);
            assert_eq!(status, 502);
            assert_eq!(body["error"]["code"], "sequencer_key_mismatch");
        }
    }

    let ranged_keys = [
        SequencerAuthorization::new(
            expected_keys[0].sequencer_id(),
            expected_keys[0].public_key(),
            10,
            19,
        ),
        SequencerAuthorization::new(
            expected_keys[1].sequencer_id(),
            expected_keys[1].public_key(),
            20,
            29,
        ),
    ];
    let keys_file = harness.directory.join("receipt-keys.json");
    let chain_url = format!("http://{}/evm", harness.config.replica_address);
    let load_keys = |keys: &[SequencerAuthorization]| {
        let entries: Vec<_> = keys
            .iter()
            .map(|key| {
                serde_json::json!({
                    "keyId": base64(&key.sequencer_id()),
                    "publicKey": base64(&key.public_key()),
                    "firstHeight": key.first_batch_number(),
                    "lastHeight": key.last_batch_number(),
                })
            })
            .collect();
        must(fs::write(&keys_file, serde_json::to_vec(&entries).unwrap()));
        ChainReceipts::load(&chain_url, &keys_file)
    };
    harness.config.chain_receipts = must(load_keys(&ranged_keys));
    let ranged = ranged_keys.to_vec();
    assert_eq!(must(trust::current(&harness.config)), ranged);
    for key in &ranged {
        assert_eq!(
            must(trust::snapshot(&harness.config, key.public_key())),
            ranged
        );
    }
    for (index, seed) in [0x51, 0x52, 0x53].into_iter().enumerate() {
        let signer = ed25519_dalek::SigningKey::from_bytes(&[seed; 32]);
        for number in [0, 9, 10, 19, 20, 29, 30, u64::MAX] {
            for network in [125_u32, 126] {
                let mut header = headers[index * 4].0.clone();
                header[9..13].copy_from_slice(&network.to_be_bytes());
                header[23..31].copy_from_slice(&number.to_be_bytes());
                let signature = signer
                    .sign(&must(layerx_wire::hash::batch_header_digest(&header)))
                    .to_bytes();
                let result = trust::authorization(&harness.config, &header, &signature);
                if index < 2
                    && network == 125
                    && (ranged[index].first_batch_number()..=ranged[index].last_batch_number())
                        .contains(&number)
                {
                    assert_eq!(must(result), ranged[index]);
                    assert!(trust::authorization(&harness.config, &header, &[0; 64]).is_err());
                    assert!(trust::authorization(
                        &harness.config,
                        &header[..header.len() - 1],
                        &signature
                    )
                    .is_err());
                } else {
                    assert!(
                        result.is_err(),
                        "key {index}, batch {number}, network {network}"
                    );
                }
            }
        }
    }
    harness.config.chain_receipts = must(load_keys(&expected));

    let rotated = serde_json::json!([{
        "keyId": base64(&expected_keys[0].sequencer_id()), "publicKey": base64(&initial_key),
        "firstHeight": 0, "lastHeight": u64::MAX,
    }]);
    must(fs::write(&keys_file, rotated.to_string()));
    assert_eq!(
        must(trust::current(&harness.config)),
        expected.clone(),
        "editing the file must not rotate the running authority"
    );
    harness.config.chain_receipts = must(ChainReceipts::load(
        &format!("http://{}/evm", harness.config.replica_address),
        &keys_file,
    ));
    assert_ne!(must(trust::current(&harness.config)), expected);
    assert!(trust::snapshot(&harness.config, expected_keys[1].public_key()).is_err());

    harness.config.protocol_network_id = 0;
    assert!(trust::current(&harness.config).is_err());
    assert!(trust::snapshot(&harness.config, initial_key).is_err());
    assert!(trust::authorization(&harness.config, &headers[0].0, &headers[0].1).is_err());
    harness.config.protocol_network_id = 125;
    let mut invalid_lists = vec![
        Vec::new(),
        vec![expected_keys[0], expected_keys[0]],
        vec![SequencerAuthorization::new(
            [0; 32],
            initial_key,
            0,
            u64::MAX,
        )],
        vec![SequencerAuthorization::new(
            expected_keys[0].sequencer_id(),
            initial_key,
            2,
            1,
        )],
        vec![SequencerAuthorization::new(
            [1; 32],
            initial_key,
            0,
            u64::MAX,
        )],
    ];
    for public_key in [[0; 32], [0xff; 32], {
        let mut identity = [0; 32];
        identity[0] = 1;
        identity
    }] {
        invalid_lists.push(vec![SequencerAuthorization::new(
            [1; 32],
            public_key,
            0,
            u64::MAX,
        )]);
    }
    for keys in invalid_lists {
        let result = match load_keys(&keys) {
            Ok(chain_receipts) => {
                harness.config.chain_receipts = chain_receipts;
                trust::current(&harness.config)
            }
            Err(_) => Err(()),
        };
        assert!(
            result.is_err(),
            "invalid pinned key list was admitted: {keys:?}"
        );
    }
    let error = kernel.accept().unwrap_err();
    assert_eq!(
        error.kind(),
        std::io::ErrorKind::WouldBlock,
        "trust snapshots and chain receipt admission must never open the kernel socket"
    );
}

#[test]
fn authority_starts_without_an_lni_socket() {
    let document = receipt_vectors();
    let case = vector(&document, "V1-pax-transfer");
    let harness = ReceiptHarness::start(&[case.clone()], &case);
    assert!(harness
        .files()
        .iter()
        .all(|path| !must(fs::metadata(path)).file_type().is_socket()));
    let authority =
        AuthorityProcess::start_with_replica(&harness, Some(harness.config.replica_address));
    let (status, body) = authority.get("/readyz");
    assert_eq!(status, 200);
    assert_eq!(
        body,
        serde_json::json!({
            "ready": true,
            "network_id": "chain-receipts-test",
            "protocol_network_id": 125,
            "wire_version": PROTOCOL_VERSION.to_string(),
        })
    );
    let activity = hex_bytes(&case["receiptJson"]["txHash"])
        .try_into()
        .unwrap();
    match lookup_receipt(&harness.config, activity, false) {
        ReceiptSource::Found(bytes) => assert_eq!(bytes, hex_bytes(&case["signedReceipt"])),
        other => panic!("chain receipt was not admitted without an LNI variable: {other:?}"),
    }
    let activity = hex::encode(&activity);
    let (status, body) = authority.receipt(&activity);
    assert_eq!(status, 503);
    assert_eq!(body["error"]["code"], "replica_evidence_unavailable",
        "the running authority must admit the pinned chain receipt before requesting independent replica evidence");
    let calls = must(harness.calls.lock());
    assert!(calls.iter().any(|call| call["method"] == "eth_chainId"));
    let receipts: Vec<_> = calls
        .iter()
        .filter(|call| call["method"] == "pax_sequencerReceipt")
        .collect();
    assert_eq!(receipts.len(), 2);
    for call in receipts {
        assert_eq!(call["params"], serde_json::json!([format!("0x{activity}")]));
    }
    assert!(harness
        .files()
        .iter()
        .all(|path| !must(fs::metadata(path)).file_type().is_socket()));
}

#[test]
fn receipt_admission_needs_no_kernel_socket() {
    chain_readiness_tests::actual_node_info_refuses_incompatible_receipt_admission();
    let document = receipt_vectors();
    let case = vector(&document, "V1-pax-transfer");
    let harness = ReceiptHarness::start(&[case.clone()], &case);
    let authority =
        AuthorityProcess::start_with_replica(&harness, Some(harness.config.replica_address));
    let (status, body) = authority.get("/readyz");
    assert_eq!(status, 200);
    assert_eq!(
        body,
        serde_json::json!({
            "ready": true,
            "network_id": "chain-receipts-test",
            "protocol_network_id": 125,
            "wire_version": PROTOCOL_VERSION.to_string(),
        })
    );
    let activity = hex_bytes(&case["receiptJson"]["txHash"])
        .try_into()
        .unwrap();
    let expected = hex_bytes(&case["signedReceipt"]);
    match lookup_receipt(&harness.config, activity, false) {
        ReceiptSource::Found(bytes) => assert_eq!(bytes, expected),
        other => panic!("chain receipt was not admitted without a kernel socket: {other:?}"),
    }
    let activity = hex::encode(&activity);
    let (status, body) = authority.receipt(&activity);
    assert_eq!(status, 503);
    assert_eq!(body["error"]["code"], "replica_evidence_unavailable",
        "the running authority must admit the pinned chain receipt before requesting independent replica evidence");
    assert_eq!(
        must(must(SequencerEndpoint::new(&harness.config.chain_rpc_url, &[])).chain_id()),
        125
    );
    let mut other = ReceiptHarness::start_with_chain_id(&[case.clone()], &[case], 126);
    let other_authority =
        AuthorityProcess::start_with_replica(&other, Some(other.config.replica_address));
    let (status, body) = other_authority.get("/readyz");
    assert_eq!(status, 503);
    assert_eq!(body["ready"], false);
    other.stop_chain();
    let (status, body) = other_authority.get("/readyz");
    assert_eq!(status, 503);
    assert_eq!(body["ready"], false);
    let calls = must(harness.calls.lock());
    assert!(calls.iter().any(|call| call["method"] == "eth_chainId"));
    let receipt_calls: Vec<_> = calls
        .iter()
        .filter(|call| call["method"] == "pax_sequencerReceipt")
        .collect();
    assert_eq!(receipt_calls.len(), 2);
    for call in receipt_calls {
        assert_eq!(call["params"], serde_json::json!([format!("0x{activity}")]));
    }
}
