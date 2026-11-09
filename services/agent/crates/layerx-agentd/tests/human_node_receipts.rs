use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::proof::ProofBundleTarget;
use layerx_agentd::finality::{
    augment_verified, validator_address, BlockCommit, CommitSignature, Validator, VerifiedBlock,
    VerifiedReceipt,
};
use layerx_agentd::human::HumanOperationError;
use layerx_agentd::human_node::HumanNode;
use layerx_agentd::human_receipt_frame::{ReceiptLookupAnswer, ReceiptLookupFrame};
use layerx_agentd::human_runtime::{
    lookup_human_receipt_by_outbox_hash, receipt_lookup_found_bytes,
    receipt_lookup_not_ready_bytes, receipt_proof_operation,
};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::outbox::Outbox;
use layerx_agentd::read::{validator_set_hash, CheckpointHeader, CommitTimestamp, CommitValidator};
use layerx_agentd::store::{Store, TenantId};
use layerx_client::rpc::receipt::Lookup;
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_programs::chain_proof::merkle::hash_from_byte_slices;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

#[track_caller]
fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{error:?}"),
    }
}

fn serve(exchanges: Vec<(&'static str, Value)>) -> (String, JoinHandle<Vec<Value>>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    must(listener.set_nonblocking(true));
    let handle = thread::spawn(move || {
        let mut requests = Vec::new();
        for (method, result) in exchanges {
            let expires = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < expires, "missing {method} request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(2))));
            must(stream.set_write_timeout(Some(Duration::from_secs(2))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(must(value.trim().parse::<usize>()));
                }
            }
            let length = length.unwrap_or_else(|| panic!("content length required"));
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            let answer =
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
            requests.push(request);
        }
        requests
    });
    (url, handle)
}

fn rpc_hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn unhex(value: &Value) -> Vec<u8> {
    must(hex::decode(
        value
            .as_str()
            .expect("fixture hex")
            .strip_prefix("0x")
            .expect("RPC hex prefix"),
    ))
}

fn fixture_key(case: &Value) -> ReceiptKeyEntry {
    let key = &case["pinnedKey"];
    ReceiptKeyEntry {
        key_id: must(unhex(&key["keyId"]).try_into()),
        public_key: must(unhex(&key["publicKey"]).try_into()),
        first_height: key["firstHeight"].as_u64().expect("first height"),
        last_height: key["lastHeight"].as_u64().expect("last height"),
    }
}

/// Replay the shared node producer's exact JSON, signed receipt and EVM bytes.
fn signed_receipt(success: bool) -> (Vec<u8>, Value, Vec<u8>, ReceiptKeyEntry) {
    let fixtures: Value = must(serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    assert_eq!(
        fixtures["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let id = if success {
        "V1-layerxledger-pay-exact"
    } else {
        "V2-layerxledger-revert"
    };
    let case = fixtures["cases"]
        .as_array()
        .expect("node cases")
        .iter()
        .find(|case| case["id"] == id)
        .expect("ledger case");
    assert_eq!(case["receiptJson"]["receipt"], case["signedReceipt"]);
    assert_eq!(case["receiptJson"]["paymentDenom"], "uhpx");
    (
        unhex(&case["rawTransaction"]),
        case["receiptJson"].clone(),
        unhex(&case["signedReceipt"]),
        fixture_key(case),
    )
}

fn published_key(key: ReceiptKeyEntry) -> Value {
    json!({"keys": [{
        "keyId": rpc_hex(&key.key_id),
        "publicKey": rpc_hex(&key.public_key),
        "firstHeight": key.first_height,
        "lastHeight": key.last_height,
    }]})
}

fn lookup_server(
    keys: Value,
    receipt: Value,
    landing: bool,
) -> (
    Box<dyn HumanNode>,
    SequencerEndpoint,
    JoinHandle<Vec<Value>>,
) {
    let mut exchanges = vec![
        ("eth_chainId", json!("0x7d")),
        ("px_getReceiptKeys", keys),
        ("pax_sequencerReceipt", receipt.clone()),
        ("pax_sequencerReceipt", receipt),
    ];
    if landing {
        exchanges.push(("eth_getTransactionReceipt", Value::Null));
    }
    let (url, server) = serve(exchanges);
    let endpoint = must(SequencerEndpoint::new(&url, &[]));
    let node = must(NodeRpc::connect(&NodeRpcConfig {
        url,
        chain_id: 125,
        deadline: Duration::from_secs(2),
    }));
    (Box::new(node), endpoint, server)
}

fn rotated_keys(key: ReceiptKeyEntry) -> Value {
    let mut entries = published_key(key)["keys"].as_array().unwrap().clone();
    let old = SigningKey::from_bytes(&[42; 32]);
    let current = SigningKey::from_bytes(&[43; 32]);
    entries.insert(
        0,
        json!({
            "keyId": rpc_hex(&[2; 32]), "publicKey": rpc_hex(&old.verifying_key().to_bytes()),
            "firstHeight": 1, "lastHeight": 999
        }),
    );
    entries.push(json!({
        "keyId": rpc_hex(&[3; 32]), "publicKey": rpc_hex(&current.verifying_key().to_bytes()),
        "firstHeight": 2000, "lastHeight": 3000
    }));
    json!({"keys": entries})
}

#[test]
fn human_receipt_lookup_reads_pax_sequencer_receipt_by_outbox_hash() {
    for success in [true, false] {
        check_human_receipt_lookup(success);
    }
}

fn check_human_receipt_lookup(success: bool) {
    let directory = std::env::temp_dir().join(format!(
        "layerx-human-node-receipts-{}-{success}",
        std::process::id(),
    ));
    let tenant = must(TenantId::new("human-node-receipts"));
    let (raw, response, canonical, key) = signed_receipt(success);
    let height = u64::from_str_radix(
        response["height"]
            .as_str()
            .unwrap()
            .trim_start_matches("0x"),
        16,
    )
    .unwrap();
    let (transaction, signature) = must(decode_signed(&raw));
    let sender = must(recover_sender(&transaction, &signature));
    let hash = transaction_hash(&raw);
    assert_eq!(response["from"], rpc_hex(&sender));
    assert_eq!(response["txHash"], rpc_hex(&hash));
    let mut store = must(Store::open(&directory));
    let mut outbox = Outbox::default();
    let idempotency_key = must(outbox.enqueue(&mut store, tenant.clone(), raw.clone()));
    assert_eq!(idempotency_key, hash);
    assert_eq!(outbox.status(idempotency_key).unwrap().activity_id, hash);
    drop(store);
    let mut store = must(Store::open(&directory));
    let mut outbox = Outbox::default();
    must(outbox.restore(&store, tenant.clone(), idempotency_key));
    assert_eq!(must(outbox.exact_signed_bytes(idempotency_key)), raw);

    {
        for (first, last) in [(1000, 1999), (height, height)] {
            let key = ReceiptKeyEntry {
                first_height: first,
                last_height: last,
                ..key
            };
            let published = rotated_keys(key);
            layerx_client::sequencer::decode_receipt_keys(&published)
                .unwrap_or_else(|reason| panic!("receipt lookup key fixture refused: {reason:?}"));
            let (node, endpoint, server) = lookup_server(published, response.clone(), true);
            let (lookup, keys) = must(lookup_human_receipt_by_outbox_hash(
                node.as_ref(),
                &endpoint,
                &outbox,
                idempotency_key,
                hash,
            ));
            let Lookup::Verified(receipt) = lookup else {
                panic!("authenticated receipt required: {lookup:?}")
            };
            assert_eq!(receipt.activity_id(), hash);
            assert_eq!(receipt.canonical_bytes(), canonical);
            assert_eq!(receipt.height(), height);
            assert_eq!(receipt.execution().success, success);
            assert_eq!(receipt.execution().signing_key, key.public_key);
            assert_eq!(receipt.result_code().raw(), i32::from(!success));
            assert_eq!(receipt.rung(), Rung::Receipted);
            let receipt_tenant = must(TenantId::new(format!(
                "human-receipt-{success}-{first}-{last}"
            )));
            let served = must(layerx_agentd::receipt::store_verified_if_absent(
                &mut store,
                receipt_tenant.clone(),
                idempotency_key,
                receipt.canonical_bytes(),
                &receipt,
            ));
            let protocol = receipt.receipt().protocol().unwrap();
            let authority = AuthorizedBatch::new(
                protocol.batch_id(),
                protocol.asset(),
                protocol.previous_state_root(),
                protocol.resulting_state_root(),
                key.public_key,
            );
            let verified = must(VerifiedReceipt::verify(hash, &canonical, &keys));
            let block = committed_block(&raw, height);
            for committed in [false, true] {
                let evidence = must(augment_verified(
                    &mut store,
                    receipt_tenant.clone(),
                    idempotency_key,
                    &verified,
                    committed.then_some(&block),
                ));
                let answer = must(receipt_lookup_found_bytes(
                    &served, &evidence, authority, &keys,
                ));
                let ReceiptLookupAnswer::Found(frame) = must(ReceiptLookupAnswer::decode(&answer))
                else {
                    panic!("found receipt answer required");
                };
                assert_eq!(
                    frame,
                    ReceiptLookupFrame {
                        canonical_bytes: served.canonical_bytes.clone(),
                        authorised_batch: authority,
                        verification_level: served.metadata.verification_level,
                        tx_hash: evidence.tx_hash,
                        receipt_hash: sha256(&canonical),
                        block_height: evidence.height,
                        rung: if committed {
                            Rung::Committed
                        } else {
                            Rung::Receipted
                        },
                        signed_receipt: evidence.receipt_bytes.clone(),
                        receipt_key_id: key.key_id,
                        public_key: key.public_key,
                        first_height: first,
                        last_height: last,
                        inclusion_height: height,
                    }
                );
                assert_eq!(
                    must(ReceiptLookupFrame::decode(&must(frame.encode()))),
                    frame
                );
                let mut mismatched = evidence.clone();
                mismatched.tx_hash[0] ^= 1;
                assert!(matches!(
                    receipt_lookup_found_bytes(&served, &mismatched, authority, &keys),
                    Err(HumanOperationError::Refused)
                ));
            }
            assert_eq!(served.canonical_bytes, canonical);
            assert_eq!(served.metadata.activity_id, hash);
            assert_eq!(served.metadata.idempotency_key, idempotency_key);
            assert_eq!(served.metadata.result.code.raw(), i32::from(!success));
            for selector in [
                layerx_agentd::receipt::ReceiptLookupKey::Activity(hash),
                layerx_agentd::receipt::ReceiptLookupKey::Idempotency(idempotency_key),
                layerx_agentd::receipt::ReceiptLookupKey::GlobalSequence(receipt.global_sequence()),
            ] {
                assert_eq!(
                    must(layerx_agentd::receipt::serve(
                        &store,
                        receipt_tenant.clone(),
                        selector
                    )),
                    served
                );
            }
            assert!(layerx_agentd::receipt::store_verified_if_absent(
                &mut store,
                receipt_tenant.clone(),
                [0x88; 32],
                receipt.canonical_bytes(),
                &receipt,
            )
            .is_err());
            assert!(layerx_agentd::receipt::store_verified_if_absent(
                &mut store,
                receipt_tenant,
                idempotency_key,
                b"altered",
                &receipt,
            )
            .is_err());
            let proof = must(receipt_proof_operation(
                ProofBundleTarget::Receipt(hash),
                Rung::Receipted,
                &raw,
                receipt.canonical_bytes(),
                &keys,
                125,
                2000,
                [0x55; 32],
            ));
            assert_eq!(proof.value.receipt_key, key);
            assert_eq!(proof.value.raw_transaction.as_bytes(), raw);
            assert_eq!(proof.value.signed_receipt.as_bytes(), canonical);
            assert_eq!(proof.freshness.value_height, height);
            assert_eq!(proof.freshness.chain_head, 2000);
            assert_eq!(proof.achieved_rung, Rung::Receipted);
            assert!(proof.value.commit.is_none());
            let requests = must(server.join());
            assert_eq!(requests.len(), 5);
            assert_eq!(requests[1]["params"], json!([]));
            for request in &requests[2..] {
                assert_eq!(request["params"], json!([rpc_hex(&hash)]));
            }
        }
    }

    for wrong_key in [
        ReceiptKeyEntry {
            last_height: height - 1,
            ..key
        },
        ReceiptKeyEntry {
            first_height: height + 1,
            ..key
        },
        ReceiptKeyEntry {
            public_key: SigningKey::from_bytes(&[99; 32]).verifying_key().to_bytes(),
            ..key
        },
    ] {
        let (node, endpoint, server) =
            lookup_server(rotated_keys(wrong_key), response.clone(), false);
        assert!(matches!(
            lookup_human_receipt_by_outbox_hash(
                node.as_ref(),
                &endpoint,
                &outbox,
                idempotency_key,
                hash
            ),
            Err(HumanOperationError::Refused)
        ));
        assert_eq!(must(server.join()).len(), 4);
    }

    let mut altered = response.clone();
    altered["seq"] = json!("0x8");
    let (node, endpoint, server) = lookup_server(rotated_keys(key), altered, false);
    assert!(matches!(
        lookup_human_receipt_by_outbox_hash(
            node.as_ref(),
            &endpoint,
            &outbox,
            idempotency_key,
            hash
        ),
        Err(HumanOperationError::Refused)
    ));
    assert_eq!(must(server.join()).len(), 4);

    for (id, expected) in [(idempotency_key, [0x88; 32]), ([0x88; 32], hash)] {
        let (url, server) = serve(vec![("eth_chainId", json!("0x7d"))]);
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let node = must(NodeRpc::connect(&NodeRpcConfig {
            url,
            chain_id: 125,
            deadline: Duration::from_secs(2),
        }));
        assert!(matches!(
            lookup_human_receipt_by_outbox_hash(&node, &endpoint, &outbox, id, expected),
            Err(HumanOperationError::Refused)
        ));
        assert_eq!(must(server.join()).len(), 1);
    }

    let (node, endpoint, server) = lookup_server(rotated_keys(key), Value::Null, false);
    let (lookup, _) = must(lookup_human_receipt_by_outbox_hash(
        node.as_ref(),
        &endpoint,
        &outbox,
        idempotency_key,
        hash,
    ));
    assert_eq!(lookup, Lookup::Absent);
    assert_eq!(
        must(ReceiptLookupAnswer::decode(
            &receipt_lookup_not_ready_bytes(&lookup).unwrap()
        )),
        ReceiptLookupAnswer::Absent
    );
    assert_eq!(must(server.join()).len(), 4);
    let submitted = json!({"rung": "submitted", "txHash": rpc_hex(&hash), "nonce": format!("{:#x}", transaction.nonce)});
    let (node, endpoint, server) = lookup_server(rotated_keys(key), submitted, false);
    let (lookup, _) = must(lookup_human_receipt_by_outbox_hash(
        node.as_ref(),
        &endpoint,
        &outbox,
        idempotency_key,
        hash,
    ));
    assert_eq!(
        lookup,
        Lookup::Submitted {
            tx_hash: hash,
            nonce: transaction.nonce
        }
    );
    assert_eq!(
        must(ReceiptLookupAnswer::decode(
            &receipt_lookup_not_ready_bytes(&lookup).unwrap()
        )),
        ReceiptLookupAnswer::Submitted {
            tx_hash: hash,
            nonce: transaction.nonce
        }
    );
    assert_eq!(must(server.join()).len(), 4);
    assert_eq!(must(outbox.exact_signed_bytes(idempotency_key)), raw);
    drop(store);
    must(std::fs::remove_dir_all(directory));
}

fn committed_block(raw: &[u8], height: u64) -> VerifiedBlock {
    let signer = SigningKey::from_bytes(&[50; 32]);
    let public_key = signer.verifying_key().to_bytes();
    let validator = CommitValidator {
        address: validator_address(&public_key),
        public_key,
        voting_power: 1,
    };
    let root = hash_from_byte_slices(&[&sha256(raw)]);
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "paxeer-test-125".to_owned(),
        height,
        time: CommitTimestamp {
            seconds: 1_000,
            nanos: 0,
        },
        last_block_id: None,
        last_commit_hash: None,
        data_hash: Some(root),
        validators_hash: validator_set_hash(&[validator]),
        next_validators_hash: validator_set_hash(&[validator]),
        consensus_hash: [17; 32],
        app_hash: [5; 32],
        last_results_hash: None,
        evidence_hash: None,
        proposer_address: validator.address,
    };
    let block = layerx_client::sequencer::SequencerBlock {
        number: height,
        hash: header.block_hash(),
        parent_hash: [16; 32],
        timestamp: 1_000,
        transactions_root: root,
        receipts_root: [20; 32],
        transactions: vec![raw.to_vec()],
    };
    let validators = [Validator {
        public_key,
        voting_power: 1,
    }];
    let mut commit = BlockCommit {
        height,
        round: 0,
        block_hash: block.hash,
        part_set_total: 1,
        part_set_hash: [19; 32],
        signatures: Vec::new(),
    };
    let vote = must(commit.vote_sign_bytes("paxeer-test-125", 1_000, 0));
    commit.signatures.push(CommitSignature {
        validator_address: validator_address(&public_key),
        timestamp_seconds: 1_000,
        timestamp_nanos: 0,
        signature: signer.sign(&vote).to_bytes(),
    });
    must(VerifiedBlock::verify_with_header(
        &block,
        &commit,
        &header,
        "paxeer-test-125",
        &validators,
    ))
}

#[test]
fn human_receipt_fixture_missing_payment() {
    // Committed derivation of the shared producer's zero-value ledger vectors:
    // payment stays uhpx while the independently signed gas denomination is usid.
    let fixtures: Value = must(serde_json::from_str(include_str!(
        "../../layerx-agentd-host/src/repair_22_31_363_receipts.json"
    )));
    assert_eq!(
        fixtures["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    for outcome in ["success", "reverted"] {
        let case = &fixtures[outcome];
        let raw = unhex(&case["rawTransaction"]);
        let hash = transaction_hash(&raw);
        let view = case["receiptJson"].clone();
        let key = fixture_key(case);
        assert_eq!(view["paymentDenom"], "uhpx");
        assert_eq!(view["feeDenom"], "usid");
        let directory =
            std::env::temp_dir().join(format!("human-payment-{}-{outcome}", std::process::id()));
        let tenant = must(TenantId::new("human-payment"));
        let mut store = must(Store::open(&directory));
        let mut outbox = Outbox::default();
        let id = must(outbox.enqueue(&mut store, tenant, raw.clone()));
        let (node, endpoint, server) = lookup_server(published_key(key), view.clone(), true);
        let (lookup, _) = must(lookup_human_receipt_by_outbox_hash(
            node.as_ref(),
            &endpoint,
            &outbox,
            id,
            hash,
        ));
        let Lookup::Verified(receipt) = lookup else {
            panic!("authenticated payment receipt")
        };
        assert_eq!(receipt.canonical_bytes(), unhex(&case["signedReceipt"]));
        assert_eq!(receipt.execution().payment_denom(), "uhpx");
        assert_eq!(receipt.execution().fee_denom, "usid");
        assert_eq!(
            receipt.receipt().protocol().unwrap().asset(),
            sha256(b"uhpx")
        );
        assert_eq!(receipt.execution().success, outcome == "success");
        assert_eq!(must(server.join()).len(), 5);
        // Keep every other field and the authentic signature intact; omission and
        // substituting the fee denomination must each refuse at the Human reader.
        for missing in [true, false] {
            let mut invalid = view.clone();
            if missing {
                assert!(invalid
                    .as_object_mut()
                    .unwrap()
                    .remove("paymentDenom")
                    .is_some());
            } else {
                invalid["paymentDenom"] = invalid["feeDenom"].clone();
            }
            let (url, server) = serve(vec![
                ("eth_chainId", json!("0x7d")),
                ("px_getReceiptKeys", published_key(key)),
                ("pax_sequencerReceipt", invalid),
            ]);
            let endpoint = must(SequencerEndpoint::new(&url, &[]));
            let node = must(NodeRpc::connect(&NodeRpcConfig {
                url,
                chain_id: 125,
                deadline: Duration::from_secs(2),
            }));
            assert!(matches!(
                lookup_human_receipt_by_outbox_hash(&node, &endpoint, &outbox, id, hash),
                Err(HumanOperationError::Refused)
            ));
            assert_eq!(must(server.join()).len(), 3);
            assert_eq!(must(outbox.exact_signed_bytes(id)), raw);
        }
        drop(store);
        must(std::fs::remove_dir_all(directory));
    }
}
