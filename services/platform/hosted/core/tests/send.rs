use layerx_platform_core::{
    build_send_with_identity_sequence, main_account, SeedSigner, SendRequest, TreasurySigner,
};

#[test]
fn beta_send_uses_native_account_ids_in_signed_payload_and_authorization() {
    let expected = "efc9802f76722dfc48ebfed35bfd8b20dbc2775fe2f027d6cbd595aff1307454";
    let source = main_account("did:key:alice").unwrap_or_else(|error| panic!("{error}"));
    let encoded = layerx_platform_core::hex_encode(&source);
    assert_eq!(encoded, expected);
    let signed = build_send_with_identity_sequence(
        &[9; 32],
        11,
        &SendRequest {
            network_id: 42,
            source_did: "did:key:alice".into(),
            destination_did: "did:key:bob".into(),
            asset: [2; 32],
            amount: 25,
            account_sequence: 0,
            idempotency_key: [3; 32],
            not_before_ms: 1000,
            expires_at_ms: 2000,
            fee_limit: 1000,
        },
    )
    .unwrap_or_else(|error| panic!("{error}"));
    let (call, signature) = layerx_crypto::evm_transaction::decode_signed(&signed.canonical)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let sender = layerx_crypto::evm_transaction::recover_sender(&call, &signature).unwrap();
    assert_eq!(
        Some(sender),
        SeedSigner::new(&[9; 32]).transaction_signer().evm_address()
    );
    assert_eq!(
        call.to,
        [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a]
    );
    let disclosure = layerx_crypto::payments::bind_call(&call, sender).unwrap();
    let Some(layerx_crypto::disclosure::call::ConstructMethod::LedgerSend { payload }) =
        disclosure.construct_method().unwrap()
    else {
        panic!("ledger send calldata required")
    };
    assert_eq!(signed.source_account, source);
    assert_eq!(&payload[4..36], source);
    assert_eq!(&payload[36..68], signed.destination_account);
    assert_eq!(&payload[198..230], source);
    assert_eq!(call.nonce, 11);
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.gas_limit, 1000);
    assert_eq!(signed.canonical[0], 2);
    assert_eq!(
        signed.activity_id,
        layerx_crypto::evm_transaction::transaction_hash(&signed.canonical)
    );
}

mod boundary {
    include!("../src/main.rs");

    fn test_config(root: &Path, seed: &[u8; 32]) -> Arc<Config> {
        let certificate = fs::read(root.join("certificate.der")).unwrap();
        let key = fs::read(root.join("key.der")).unwrap();
        let tls = Arc::new(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(
                    vec![CertificateDer::from(certificate.clone())],
                    PrivateKeyDer::from(PrivatePkcs8KeyDer::from(key)),
                )
                .unwrap(),
        );
        let treasury = layerx_platform_core::SeedSigner::new(seed);
        Arc::new(Config {
            listen: "127.0.0.1:0".parse().unwrap(),
            admin_listen: "127.0.0.1:0".parse().unwrap(),
            tls: Arc::clone(&tls),
            admin_tls: tls,
            sequencer: SequencerEndpoint::new("https://localhost:1", &[&certificate]).unwrap(),
            network: PxEndpoint::new("https://localhost:1", &[&certificate]).unwrap(),
            network_id: 42,
            node: NodeEndpoint {
                host: "127.0.0.1".into(),
                port: 1,
            },
            node_token: Zeroizing::new("test-node-token".into()),
            receipt_events_token: None,
            wallet_caps_token: None,
            replica: NodeEndpoint {
                host: "127.0.0.1".into(),
                port: 1,
            },
            replica_token: Zeroizing::new("test-replica-token".into()),
            admin_token: Zeroizing::new("test-admin-token".into()),
            treasury_did: did_for_public_key(&treasury.public_key()),
            treasury: Arc::new(treasury),
            treasury_asset: [2; 32],
            funding: chain_fund::FundingSlot::from_env(),
            sequencer_id: [4; 32],
            supervisor_socket: root.join("supervisor.sock"),
            state_dir: root.to_path_buf(),
            fee_limit: 1000,
            receipt_deadline: Duration::from_secs(1),
            admin_lock: Mutex::new(()),
            journal_lock: Mutex::new(()),
        })
    }

    fn exchange(
        config: Arc<Config>,
        method: &str,
        path: &str,
        body: &[u8],
    ) -> (u16, serde_json::Value) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let certificate = fs::read(config.state_dir.join("certificate.der")).unwrap();
        let plane = if path.starts_with("/v1/") {
            Plane::Core
        } else {
            Plane::Admin
        };
        let worker = thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            handle_connection(&config, plane, stream).unwrap();
        });
        let connector = native_tls::TlsConnector::builder()
            .add_root_certificate(native_tls::Certificate::from_der(&certificate).unwrap())
            .build()
            .unwrap();
        let tcp = TcpStream::connect(address).unwrap();
        tcp.set_read_timeout(Some(Duration::from_secs(5))).unwrap();
        tcp.set_write_timeout(Some(Duration::from_secs(5))).unwrap();
        let mut stream = connector.connect("localhost", tcp).unwrap();
        write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer test-admin-token\r\nContent-Type: application/json\r\nIdempotency-Key: eip1559-round-trip\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap();
        stream.write_all(body).unwrap();
        stream.flush().unwrap();
        let response = read_http_message(&mut stream, MAX_REQUEST_BYTES).unwrap();
        let status = response.headers[""]
            .split_whitespace()
            .nth(1)
            .unwrap()
            .parse()
            .unwrap();
        let body = serde_json::from_slice(&response.body).unwrap();
        worker.join().unwrap();
        (status, body)
    }

    pub(super) fn round_trip() {
        install_provider().unwrap();
        let root = std::env::temp_dir().join(format!("layerx-core-eip1559-{}", std::process::id()));
        fs::DirBuilder::new().mode(0o700).create(&root).unwrap();
        struct Cleanup(PathBuf);
        impl Drop for Cleanup {
            fn drop(&mut self) {
                fs::remove_dir_all(&self.0).unwrap();
            }
        }
        let _cleanup = Cleanup(root.clone());
        for directory in ["journal", "funding-intents", "funding-receipts"] {
            fs::DirBuilder::new()
                .mode(0o700)
                .create(root.join(directory))
                .unwrap();
        }
        for arguments in [
            vec![
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-keyout",
                "key.pem",
                "-out",
                "certificate.pem",
                "-days",
                "1",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost",
            ],
            vec![
                "x509",
                "-in",
                "certificate.pem",
                "-outform",
                "DER",
                "-out",
                "certificate.der",
            ],
            vec![
                "pkcs8", "-topk8", "-nocrypt", "-in", "key.pem", "-outform", "DER", "-out",
                "key.der",
            ],
        ] {
            let output = std::process::Command::new("openssl")
                .args(arguments)
                .current_dir(&root)
                .output()
                .unwrap();
            assert!(
                output.status.success(),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
        }
        let seed = [9; 32];
        let config = test_config(&root, &seed);
        let signer = layerx_platform_core::SeedSigner::new(&seed);
        let destination = layerx_platform_core::SeedSigner::new(&[7; 32]);
        let command = FundingCommand {
            funding_id: "eip1559-round-trip".into(),
            did: did_for_public_key(&destination.public_key()),
            public_key: hex_encode(&destination.public_key()),
            amount: 25,
        };
        let key = "eip1559-round-trip";
        let signed = layerx_platform_core::build_send_with_signer(
            &signer,
            11,
            &layerx_platform_core::SendRequest {
                network_id: config.network_id,
                source_did: config.treasury_did.clone(),
                destination_did: command.did.clone(),
                asset: config.treasury_asset,
                amount: u128::from(command.amount),
                account_sequence: 3,
                idempotency_key: send_idempotency(key),
                not_before_ms: 1000,
                expires_at_ms: 2000,
                fee_limit: 1000,
            },
        )
        .unwrap();
        let (call, signature) =
            layerx_crypto::evm_transaction::decode_signed(&signed.canonical).unwrap();
        let transaction_hash = layerx_crypto::evm_transaction::transaction_hash(&signed.canonical);
        let request_body = serde_json::to_vec(&command).unwrap();
        let digest = request_digest(&Request {
            method: "POST".into(),
            path: "/admin/v1/testnet/fund".into(),
            query: None,
            headers: BTreeMap::new(),
            body: request_body.clone(),
        });
        let mut intent = FundingIntent {
            version: 1,
            key: key.into(),
            request_digest: digest,
            request_body,
            command,
            network_id: config.network_id,
            asset: config.treasury_asset,
            sequencer_id: config.sequencer_id,
            sequencer_key: [5; 32],
            signed: PreparedFunding {
                canonical: signed.canonical.clone(),
                transaction_hash,
                sender: layerx_crypto::evm_transaction::recover_sender(&call, &signature).unwrap(),
            },
        };
        let body = serde_json::to_vec(&intent).unwrap();
        let (status, response) = exchange(
            Arc::clone(&config),
            "POST",
            "/admin/v1/testnet/funding-intents",
            &body,
        );
        assert_eq!(status, 200, "{response}");
        assert_eq!(
            response["result"]["transaction_hash"],
            hex_encode(&transaction_hash)
        );
        let unsigned = call.encode_unsigned();
        let fee_body =
            serde_json::to_vec(&serde_json::json!({"canonical_hex": hex_encode(&unsigned)}))
                .unwrap();
        let (status, estimate) =
            exchange(Arc::clone(&config), "POST", "/v1/fees/estimate", &fee_body);
        assert_eq!(status, 200, "{estimate}");
        assert_eq!(estimate["result"]["fee"], "1000");
        assert_eq!(estimate["result"]["canonical_bytes"], unsigned.len());
        let signed_fee = serde_json::to_vec(
            &serde_json::json!({"canonical_hex": hex_encode(&signed.canonical)}),
        )
        .unwrap();
        let (status, _) = exchange(
            Arc::clone(&config),
            "POST",
            "/v1/fees/estimate",
            &signed_fee,
        );
        assert_eq!(status, 400);
        intent.signed.transaction_hash[0] ^= 1;
        let (status, _) = exchange(
            Arc::clone(&config),
            "POST",
            "/admin/v1/testnet/funding-intents",
            &serde_json::to_vec(&intent).unwrap(),
        );
        assert_eq!(status, 400);
        drop(config);
        let recovered = test_config(&root, &seed);
        let (status, archive) = exchange(
            recovered,
            "GET",
            &format!("/admin/v1/testnet/funding-intents/{key}"),
            b"",
        );
        assert_eq!(status, 200, "{archive}");
        assert_eq!(
            archive["result"]["transaction_hash"],
            hex_encode(&transaction_hash)
        );
        assert_eq!(
            archive["result"]["canonical_hex"],
            hex_encode(&signed.canonical)
        );
        let archived_bytes =
            hex_decode(archive["result"]["canonical_hex"].as_str().unwrap()).unwrap();
        assert_eq!(
            layerx_crypto::evm_transaction::transaction_hash(&archived_bytes),
            transaction_hash
        );
    }
}

#[test]
fn funding_intent_round_trips_as_eip1559_send() {
    boundary::round_trip();
}
