use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer, SigningKey};
use layerx_agent_api::error::RequestId;
use layerx_agent_api::verify::{Rung, VerificationStatus};
use layerx_agentd::agent_rpc::{self, AgentRpcResponse};
use layerx_agentd::agent_rpc_dispatch::dispatch_program_receipt;
use layerx_agentd::finality::{
    augment_verified, validator_address, BlockCommit, CommitSignature, Validator, VerifiedBlock,
    VerifiedReceipt,
};
use layerx_agentd::human::HumanResponse;
use layerx_agentd::human_receipt_frame::{ReceiptLookupAnswer, ReceiptLookupFrame};
use layerx_agentd::human_runtime::{
    lookup_human_receipt_by_outbox_hash, receipt_lookup_found_bytes, receipt_lookup_not_ready_bytes,
};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::outbox::Outbox;
use layerx_agentd::read::{validator_set_hash, CheckpointHeader, CommitTimestamp, CommitValidator};
use layerx_agentd::receipt::{self, ReceiptLookupKey};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::rpc::receipt::Lookup;
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerBlock, SequencerEndpoint};
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_programs::chain_proof::merkle::hash_from_byte_slices;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_sdk::agent_envelope::{
    decode_response_status, AgentEnvelopeTransport, EnvelopeCredential, EnvelopeError,
    EnvelopeOutcome, NotReadyStatus,
};
use layerx_sdk::Operation;
use openssl::asn1::Asn1Time;
use openssl::hash::MessageDigest;
use openssl::pkey::PKey;
use openssl::rsa::Rsa;
use openssl::x509::extension::{BasicConstraints, ExtendedKeyUsage, SubjectAlternativeName};
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

static NEXT: AtomicU64 = AtomicU64::new(0);

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "receipt-rpc-22-31-220-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::create_dir_all(&path).unwrap();
        Self(path)
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0).unwrap();
    }
}

fn bytes(value: &Value) -> Vec<u8> {
    hex::decode(value.as_str().unwrap().trim_start_matches("0x")).unwrap()
}

fn accept(listener: &TcpListener) -> TcpStream {
    let deadline = Instant::now() + Duration::from_secs(10);
    loop {
        match listener.accept() {
            Ok((stream, _)) => {
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                return stream;
            }
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                assert!(Instant::now() < deadline, "missing HTTP exchange");
                thread::sleep(Duration::from_millis(1));
            }
            Err(error) => panic!("accept: {error}"),
        }
    }
}

fn read_request<S: Read>(stream: &mut S) -> (String, Value) {
    let mut reader = BufReader::new(stream);
    let mut route = String::new();
    reader.read_line(&mut route).unwrap();
    let mut length = None;
    loop {
        let mut line = String::new();
        assert_ne!(reader.read_line(&mut line).unwrap(), 0);
        if line == "\r\n" {
            break;
        }
        if let Some((name, value)) = line.split_once(':') {
            if name.eq_ignore_ascii_case("content-length") {
                length = Some(value.trim().parse::<usize>().unwrap());
            }
        }
    }
    let length = length.unwrap();
    assert!(length <= 1_048_576);
    let mut body = vec![0; length];
    reader.read_exact(&mut body).unwrap();
    (route, serde_json::from_slice(&body).unwrap())
}

fn respond(stream: &mut impl Write, status: u16, body: &[u8]) {
    write!(stream, "HTTP/1.1 {status} Response\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap();
    stream.write_all(body).unwrap();
    stream.flush().unwrap();
}

struct Recording {
    case: Value,
    key: ReceiptKeyEntry,
    raw: Vec<u8>,
    hash: [u8; 32],
    canonical: Vec<u8>,
}

impl Recording {
    fn new() -> Self {
        let recording: Value = serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))
        .unwrap();
        let case = recording["cases"][1].clone();
        let raw = bytes(&case["rawTransaction"]);
        let hash = transaction_hash(&raw);
        assert_eq!(hash.as_slice(), bytes(&case["receiptJson"]["txHash"]));
        let key = ReceiptKeyEntry {
            key_id: bytes(&case["pinnedKey"]["keyId"]).try_into().unwrap(),
            public_key: bytes(&case["pinnedKey"]["publicKey"]).try_into().unwrap(),
            first_height: case["pinnedKey"]["firstHeight"].as_u64().unwrap(),
            last_height: case["pinnedKey"]["lastHeight"].as_u64().unwrap(),
        };
        let canonical = bytes(&case["signedReceipt"]);
        Self {
            case,
            key,
            raw,
            hash,
            canonical,
        }
    }

    fn owner_lookup(&self, outbox: &Outbox, response: Value) -> Lookup {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let url = format!("http://{}/", listener.local_addr().unwrap());
        let key = self.key;
        let found = response.get("receipt").is_some();
        let mut exchanges = vec![
            ("eth_chainId", json!("0x7d")),
            (
                "px_getReceiptKeys",
                json!({"keys":[{
                    "keyId": format!("0x{}",hex::encode(key.key_id)),
                    "publicKey": format!("0x{}",hex::encode(key.public_key)),
                    "firstHeight":key.first_height,"lastHeight":key.last_height
                }]}),
            ),
            ("pax_sequencerReceipt", response.clone()),
            ("pax_sequencerReceipt", response),
        ];
        if found {
            exchanges.push(("eth_getTransactionReceipt", Value::Null));
        }
        let hash = self.hash;
        let server = thread::spawn(move || {
            for (method, result) in exchanges {
                let mut stream = accept(&listener);
                let (route, request) = read_request(&mut stream);
                assert_eq!(route, "POST / HTTP/1.1\r\n");
                assert_eq!(request["method"], method);
                if matches!(method, "pax_sequencerReceipt" | "eth_getTransactionReceipt") {
                    assert_eq!(
                        request["params"],
                        json!([format!("0x{}", hex::encode(hash))])
                    );
                }
                let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result});
                respond(&mut stream, 200, body.to_string().as_bytes());
            }
        });
        let node = NodeRpc::connect(&NodeRpcConfig {
            url: url.clone(),
            chain_id: 125,
            deadline: Duration::from_secs(3),
        })
        .unwrap();
        let endpoint = SequencerEndpoint::new(&url, &[]).unwrap();
        let (lookup, keys) =
            lookup_human_receipt_by_outbox_hash(&node, &endpoint, outbox, hash, hash).unwrap();
        assert_eq!(keys, vec![self.key]);
        server.join().unwrap();
        lookup
    }

    fn block(&self) -> VerifiedBlock {
        let signer = SigningKey::from_bytes(&[50; 32]);
        let public_key = signer.verifying_key().to_bytes();
        let validator = CommitValidator {
            address: validator_address(&public_key),
            public_key,
            voting_power: 1,
        };
        let transaction_hash = Sha256::digest(&self.raw);
        let root = hash_from_byte_slices(&[transaction_hash.as_slice()]);
        let header = CheckpointHeader {
            version_block: 11,
            version_app: 1,
            chain_id: "receipt-rpc".to_owned(),
            height: 1000,
            time: CommitTimestamp {
                seconds: 1000,
                nanos: 0,
            },
            last_block_id: None,
            last_commit_hash: None,
            data_hash: Some(root),
            validators_hash: validator_set_hash(&[validator]),
            next_validators_hash: validator_set_hash(&[validator]),
            consensus_hash: [4; 32],
            app_hash: [5; 32],
            last_results_hash: None,
            evidence_hash: None,
            proposer_address: validator.address,
        };
        let block = SequencerBlock {
            number: 1000,
            hash: header.block_hash(),
            parent_hash: [16; 32],
            timestamp: 1000,
            transactions_root: root,
            receipts_root: [20; 32],
            transactions: vec![self.raw.clone()],
        };
        let mut commit = BlockCommit {
            height: block.number,
            round: 0,
            block_hash: block.hash,
            part_set_total: 1,
            part_set_hash: [19; 32],
            signatures: vec![],
        };
        let vote = commit.vote_sign_bytes("receipt-rpc", 1000, 0).unwrap();
        commit.signatures.push(CommitSignature {
            validator_address: validator_address(&public_key),
            timestamp_seconds: 1000,
            timestamp_nanos: 0,
            signature: signer.sign(&vote).to_bytes(),
        });
        VerifiedBlock::verify_with_header(
            &block,
            &commit,
            &header,
            "receipt-rpc",
            &[Validator {
                public_key,
                voting_power: 1,
            }],
        )
        .unwrap()
    }

    fn answer(&self, store: &mut Store, tenant: &TenantId, committed: bool) -> Vec<u8> {
        let served = receipt::serve(
            store,
            tenant.clone(),
            ReceiptLookupKey::Idempotency(self.hash),
        )
        .unwrap();
        assert_eq!(served.canonical_bytes, self.canonical);
        let verified =
            VerifiedReceipt::verify(self.hash, &served.canonical_bytes, &[self.key]).unwrap();
        let block = committed.then(|| self.block());
        let evidence =
            augment_verified(store, tenant.clone(), self.hash, &verified, block.as_ref()).unwrap();
        let decoded = layerx_wire::receipt::decode(&served.canonical_bytes).unwrap();
        let protocol = decoded.protocol().unwrap();
        let authority = AuthorizedBatch::new(
            protocol.batch_id(),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
            self.key.public_key,
        );
        receipt_lookup_found_bytes(&served, &evidence, authority, &[self.key]).unwrap()
    }
}

fn tls(directory: &Directory) -> (native_tls::TlsAcceptor, PathBuf) {
    let key = PKey::from_rsa(Rsa::generate(2048).unwrap()).unwrap();
    let mut name = X509NameBuilder::new().unwrap();
    name.append_entry_by_text("CN", "Receipt RPC test CA")
        .unwrap();
    let name = name.build();
    let mut builder = X509::builder().unwrap();
    builder.set_version(2).unwrap();
    builder.set_subject_name(&name).unwrap();
    builder.set_issuer_name(&name).unwrap();
    builder.set_pubkey(&key).unwrap();
    builder
        .set_not_before(&Asn1Time::days_from_now(0).unwrap())
        .unwrap();
    builder
        .set_not_after(&Asn1Time::days_from_now(1).unwrap())
        .unwrap();
    builder
        .append_extension(BasicConstraints::new().critical().ca().build().unwrap())
        .unwrap();
    builder.sign(&key, MessageDigest::sha256()).unwrap();
    let ca = builder.build();
    let pem = ca.to_pem().unwrap();
    let path = directory.0.join("ca.pem");
    std::fs::write(&path, &pem).unwrap();
    let mut name = X509NameBuilder::new().unwrap();
    name.append_entry_by_text("CN", "localhost").unwrap();
    let mut leaf = X509::builder().unwrap();
    leaf.set_version(2).unwrap();
    leaf.set_subject_name(&name.build()).unwrap();
    leaf.set_issuer_name(ca.subject_name()).unwrap();
    leaf.set_pubkey(&key).unwrap();
    leaf.set_not_before(&Asn1Time::days_from_now(0).unwrap())
        .unwrap();
    leaf.set_not_after(&Asn1Time::days_from_now(1).unwrap())
        .unwrap();
    leaf.append_extension(BasicConstraints::new().critical().build().unwrap())
        .unwrap();
    leaf.append_extension(ExtendedKeyUsage::new().server_auth().build().unwrap())
        .unwrap();
    let san = SubjectAlternativeName::new()
        .dns("localhost")
        .build(&leaf.x509v3_context(Some(&ca), None))
        .unwrap();
    leaf.append_extension(san).unwrap();
    leaf.sign(&key, MessageDigest::sha256()).unwrap();
    let mut chain = leaf.build().to_pem().unwrap();
    chain.extend_from_slice(&pem);
    let identity =
        native_tls::Identity::from_pkcs8(&chain, &key.private_key_to_pem_pkcs8().unwrap()).unwrap();
    (native_tls::TlsAcceptor::new(identity).unwrap(), path)
}

fn rpc_peer(
    directory: &Directory,
    answers: Vec<Vec<u8>>,
) -> (AgentEnvelopeTransport, JoinHandle<()>) {
    let (tls, ca) = tls(directory);
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    let url = format!(
        "https://localhost:{}",
        listener.local_addr().unwrap().port()
    );
    let server = thread::spawn(move || {
        for answer in answers {
            let mut stream = tls.accept(accept(&listener)).unwrap();
            let (route, body) = read_request(&mut stream);
            assert_eq!(route, "POST /v1/agent/rpc HTTP/1.1\r\n");
            let envelope = agent_rpc::decode(&serde_json::to_vec(&body).unwrap()).unwrap();
            assert_eq!(envelope.operation, Operation::ProgramReceipt);
            let requested =
                Rung::from_name(envelope.request["requested_rung"].as_str().unwrap()).unwrap();
            let result = dispatch_program_receipt(
                &envelope.request,
                envelope.request_id,
                |key, activity| {
                    assert_ne!(key, [0; 32]);
                    assert_ne!(activity, [0; 32]);
                    Ok(HumanResponse::new(answer).unwrap())
                },
            );
            let response = match result.and_then(|answer| {
                agent_rpc::success_body(envelope.request_id, &answer, Some(Some(requested)))
            }) {
                Ok(response) => response,
                Err(error) => AgentRpcResponse {
                    status: agent_rpc::http_status(&error),
                    body: agent_rpc::error_body(&error),
                },
            };
            respond(&mut stream, response.status, &response.body);
        }
    });
    (
        AgentEnvelopeTransport::connect(&url, None, Some(&ca)).unwrap(),
        server,
    )
}

fn query(
    transport: &AgentEnvelopeTransport,
    hash: [u8; 32],
    requested: Rung,
) -> Result<layerx_agent_api::verify::ApiSuccess<Value>, EnvelopeError> {
    let credential = EnvelopeCredential::new("receipt-rpc", [1; 32], [2; 32], 1).unwrap();
    transport.send_operation(Operation::ProgramReceipt, RequestId(220), &json!({
        "idempotency_key":hex::encode(hash),"expected_activity_id":hex::encode(hash),"requested_rung":requested.name(),
    }), Some(&credential), None)
}

#[test]
fn receipt_rpc_roundtrip_preserves_bindings_and_explicit_rung_after_store_reopen() {
    let recording = Recording::new();
    for committed in [false, true] {
        let directory = Directory::new();
        let tenant = TenantId::new("receipt-rpc").unwrap();
        let mut store = Store::open(directory.0.join("store")).unwrap();
        let mut outbox = Outbox::default();
        assert_eq!(
            outbox
                .enqueue(&mut store, tenant.clone(), recording.raw.clone())
                .unwrap(),
            recording.hash
        );
        let Lookup::Verified(receipt) =
            recording.owner_lookup(&outbox, recording.case["receiptJson"].clone())
        else {
            panic!("verified recording required")
        };
        receipt::store_verified_if_absent(
            &mut store,
            tenant.clone(),
            recording.hash,
            receipt.canonical_bytes(),
            &receipt,
        )
        .unwrap();
        let before = recording.answer(&mut store, &tenant, committed);
        let finality_key = TenantKey::new(
            tenant.clone(),
            ObjectKind::Configuration,
            [b"finality:".as_slice(), &recording.hash].concat(),
        )
        .unwrap();
        let retained = store.get(&finality_key).unwrap().bytes().to_vec();
        drop(outbox);
        drop(store);
        let mut store = Store::open(directory.0.join("store")).unwrap();
        let mut outbox = Outbox::default();
        outbox
            .restore(&store, tenant.clone(), recording.hash)
            .unwrap();
        assert_eq!(
            outbox.exact_signed_bytes(recording.hash).unwrap(),
            recording.raw
        );
        assert_eq!(store.get(&finality_key).unwrap().bytes(), retained);
        let after = recording.answer(&mut store, &tenant, committed);
        assert_eq!(before, after);
        let ReceiptLookupAnswer::Found(frame) = ReceiptLookupAnswer::decode(&before).unwrap()
        else {
            panic!("found frame")
        };
        assert_eq!(
            frame.verification_level,
            layerx_types::verify::VerificationLevel::SEQUENCER_SIGNED
        );
        let expected_rung = if committed {
            Rung::Committed
        } else {
            Rung::Receipted
        };
        assert_eq!(frame.rung, expected_rung);
        let (transport, server) = rpc_peer(
            &directory,
            vec![before.clone(), before, after.clone(), after],
        );
        for _ in 0..2 {
            for requested in [Rung::Receipted, Rung::Committed] {
                let response = query(&transport, recording.hash, requested).unwrap();
                assert_eq!(response.value["found"], true);
                let value = &response.value["receipt"];
                assert_eq!(value["canonical_bytes"], hex::encode(&recording.canonical));
                assert_eq!(value["signed_receipt"], hex::encode(&recording.canonical));
                assert_eq!(value["tx_hash"], hex::encode(recording.hash));
                assert_eq!(
                    value["receipt_hash"],
                    hex::encode(Sha256::digest(&recording.canonical))
                );
                assert_eq!(value["receipt_key_id"], hex::encode(recording.key.key_id));
                assert_eq!(value["public_key"], hex::encode(recording.key.public_key));
                assert_eq!(
                    value["first_height"],
                    recording.key.first_height.to_string()
                );
                assert_eq!(value["last_height"], recording.key.last_height.to_string());
                assert_eq!(value["block_height"], "1000");
                assert_eq!(value["inclusion_height"], "1000");
                assert_eq!(value["verification_level"], "1");
                assert_eq!(value["rung"], expected_rung.name());
                for (name, expected) in [
                    ("batch_id", frame.authorised_batch.batch_id()),
                    ("asset", frame.authorised_batch.asset()),
                    (
                        "previous_state_root",
                        frame.authorised_batch.previous_state_root(),
                    ),
                    (
                        "resulting_state_root",
                        frame.authorised_batch.resulting_state_root(),
                    ),
                    (
                        "sequencer_public_key",
                        frame.authorised_batch.sequencer_public_key(),
                    ),
                ] {
                    assert_eq!(value["authorised_batch"][name], hex::encode(expected));
                }
                if requested <= expected_rung {
                    assert_eq!(
                        response.verification_status,
                        VerificationStatus::Achieved(expected_rung)
                    );
                } else {
                    assert!(matches!(
                        response.verification_status,
                        VerificationStatus::Below {
                            requested: Rung::Committed,
                            achieved: Rung::Receipted,
                            ..
                        }
                    ));
                }
            }
        }
        server.join().unwrap();
    }
}

#[test]
fn receipt_rpc_roundtrip_retains_absent_and_submitted_answers() {
    let directory = Directory::new();
    let recording = Recording::new();
    let mut store = Store::open(directory.0.join("store")).unwrap();
    let mut outbox = Outbox::default();
    outbox
        .enqueue(
            &mut store,
            TenantId::new("receipt-rpc").unwrap(),
            recording.raw.clone(),
        )
        .unwrap();
    let absent = recording.owner_lookup(&outbox, Value::Null);
    assert_eq!(absent, Lookup::Absent);
    let submitted = recording.owner_lookup(&outbox, json!({"rung":"submitted","txHash":format!("0x{}",hex::encode(recording.hash)),"nonce":"0x1"}));
    assert_eq!(
        submitted,
        Lookup::Submitted {
            tx_hash: recording.hash,
            nonce: 1
        }
    );
    let (transport, server) = rpc_peer(
        &directory,
        vec![
            receipt_lookup_not_ready_bytes(&absent).unwrap(),
            receipt_lookup_not_ready_bytes(&submitted).unwrap(),
        ],
    );
    let credential = EnvelopeCredential::new("receipt-rpc", [1; 32], [2; 32], 1).unwrap();
    for (value, status, expected_status) in [
        (
            json!({"found":false}),
            json!({"state":"absent","requested":"committed"}),
            NotReadyStatus::Absent {
                requested: Rung::Committed,
            },
        ),
        (
            json!({"found":false,"submitted":{"tx_hash":format!("0x{}",hex::encode(recording.hash)),"nonce":"1"}}),
            json!({"state":"pending","requested":"committed","achieved":"submitted"}),
            NotReadyStatus::Pending {
                requested: Rung::Committed,
                achieved: Rung::Submitted,
            },
        ),
    ] {
        let (received, outcome) = transport.send_operation_recorded(Operation::ProgramReceipt, RequestId(220), &json!({
            "idempotency_key":hex::encode(recording.hash),"expected_activity_id":hex::encode(recording.hash),"requested_rung":"committed"
        }), Some(&credential), None);
        assert_eq!(
            received,
            Some((
                200,
                json!({"request_id":"220","value":value,"verification_status":status})
            ))
        );
        // Recorded exchanges preserve not-ready success material through the
        // compatibility error API and the public typed outcome decoder.
        assert_eq!(
            outcome,
            Err(EnvelopeError::NotReady {
                request_id: RequestId(220),
                value: value.clone(),
                status: expected_status.clone(),
            })
        );
        let (http_status, document) = received.unwrap();
        assert_eq!(
            decode_response_status(http_status, &document),
            Some(Ok(EnvelopeOutcome::NotReady {
                request_id: RequestId(220),
                value,
                status: expected_status,
            }))
        );
    }
    server.join().unwrap();
}

#[test]
fn receipt_rpc_roundtrip_refuses_malformed_frames_and_changed_bindings() {
    let directory = Directory::new();
    let recording = Recording::new();
    let tenant = TenantId::new("receipt-rpc").unwrap();
    let mut store = Store::open(directory.0.join("store")).unwrap();
    let mut outbox = Outbox::default();
    outbox
        .enqueue(&mut store, tenant.clone(), recording.raw.clone())
        .unwrap();
    let Lookup::Verified(receipt) =
        recording.owner_lookup(&outbox, recording.case["receiptJson"].clone())
    else {
        panic!("verified receipt")
    };
    receipt::store_verified_if_absent(
        &mut store,
        tenant.clone(),
        recording.hash,
        receipt.canonical_bytes(),
        &receipt,
    )
    .unwrap();
    let original = recording.answer(&mut store, &tenant, false);
    let ReceiptLookupAnswer::Found(frame) = ReceiptLookupAnswer::decode(&original).unwrap() else {
        panic!("found frame")
    };
    let mut malformed = vec![
        vec![3],
        original[..1].to_vec(),
        original[..5].to_vec(),
        original[..original.len() - 1].to_vec(),
        [original.as_slice(), &[0]].concat(),
        vec![0, 0],
        vec![2],
        vec![2; 42],
    ];
    for change in [
        |f: &mut ReceiptLookupFrame| f.tx_hash[0] ^= 1,
        |f: &mut ReceiptLookupFrame| f.receipt_hash[0] ^= 1,
        |f: &mut ReceiptLookupFrame| f.block_height += 1,
        |f: &mut ReceiptLookupFrame| f.inclusion_height += 1,
        |f: &mut ReceiptLookupFrame| f.inclusion_height = 0,
        |f: &mut ReceiptLookupFrame| f.receipt_key_id[0] ^= 1,
        |f: &mut ReceiptLookupFrame| f.receipt_key_id = [0; 32],
        |f: &mut ReceiptLookupFrame| f.public_key[0] ^= 1,
        |f: &mut ReceiptLookupFrame| f.public_key = [0; 32],
        |f: &mut ReceiptLookupFrame| f.first_height = 1001,
        |f: &mut ReceiptLookupFrame| f.last_height = 999,
        |f: &mut ReceiptLookupFrame| f.rung = Rung::Submitted,
        |f: &mut ReceiptLookupFrame| f.signed_receipt[0] ^= 1,
        |f: &mut ReceiptLookupFrame| {
            let last = f.canonical_bytes.len() - 1;
            f.canonical_bytes[last] ^= 1;
            f.signed_receipt = f.canonical_bytes.clone();
            f.receipt_hash = Sha256::digest(&f.canonical_bytes).into();
        },
        |f: &mut ReceiptLookupFrame| {
            f.authorised_batch = AuthorizedBatch::new(
                [0; 32],
                f.authorised_batch.asset(),
                f.authorised_batch.previous_state_root(),
                f.authorised_batch.resulting_state_root(),
                f.public_key,
            )
        },
        |f: &mut ReceiptLookupFrame| {
            f.authorised_batch = AuthorizedBatch::new(
                f.authorised_batch.batch_id(),
                [0; 32],
                f.authorised_batch.previous_state_root(),
                f.authorised_batch.resulting_state_root(),
                f.public_key,
            )
        },
        |f: &mut ReceiptLookupFrame| {
            f.authorised_batch = AuthorizedBatch::new(
                f.authorised_batch.batch_id(),
                f.authorised_batch.asset(),
                [9; 32],
                f.authorised_batch.resulting_state_root(),
                f.public_key,
            )
        },
        |f: &mut ReceiptLookupFrame| {
            f.authorised_batch = AuthorizedBatch::new(
                f.authorised_batch.batch_id(),
                f.authorised_batch.asset(),
                f.authorised_batch.previous_state_root(),
                [9; 32],
                f.public_key,
            )
        },
        |f: &mut ReceiptLookupFrame| {
            f.authorised_batch = AuthorizedBatch::new(
                f.authorised_batch.batch_id(),
                f.authorised_batch.asset(),
                f.authorised_batch.previous_state_root(),
                f.authorised_batch.resulting_state_root(),
                [9; 32],
            )
        },
    ] {
        let mut changed = frame.clone();
        change(&mut changed);
        malformed.push(ReceiptLookupAnswer::Found(changed).encode().unwrap());
    }
    let verification_offset = 1 + 4 + frame.canonical_bytes.len() + 5 * 32;
    malformed.push(original[..verification_offset + 1].to_vec());
    for code in [0, 6, 255] {
        let mut changed = original.clone();
        changed[verification_offset] = code;
        malformed.push(changed);
    }
    let mut changed = original.clone();
    changed[verification_offset + 1 + 32 + 32 + 8] = 3;
    malformed.push(changed);
    let count = malformed.len();
    malformed.push(original);
    let (transport, server) = rpc_peer(&directory, malformed);
    for _ in 0..count {
        let error = query(&transport, recording.hash, Rung::Receipted).unwrap_err();
        assert!(
            matches!(error, EnvelopeError::Refused(ref error) if error.reason.as_str() == "owner.response_malformed"),
            "{error:?}"
        );
    }
    let error = query(&transport, [0x99; 32], Rung::Receipted).unwrap_err();
    assert!(
        matches!(error, EnvelopeError::Refused(ref error) if error.reason.as_str() == "owner.response_malformed")
    );
    server.join().unwrap();
}
