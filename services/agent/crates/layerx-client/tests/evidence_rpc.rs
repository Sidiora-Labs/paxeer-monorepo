use std::fmt::Write as _;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::time::Duration;

use layerx_client::rpc::evidence::{self, EvidenceError, ProofBundle, Rung};
use layerx_client::sequencer::{
    AccountProof, RpcRefusal, SequencerBlock, SequencerEndpoint, SequencerError, StorageProof,
};
use serde_json::{json, Value};

trait Must<T> {
    fn must(self) -> T;
}

impl<T, E: std::fmt::Debug> Must<T> for Result<T, E> {
    fn must(self) -> T {
        self.unwrap_or_else(|error| panic!("{error:?}"))
    }
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::from("0x"), |mut text, byte| {
        write!(text, "{byte:02x}").must();
        text
    })
}

#[test]
fn proof_bundle_reads_eth_get_proof_and_the_landing_block() {
    type BundleRead =
        fn(&SequencerEndpoint, [u8; 20], &[[u8; 32]], u64) -> Result<ProofBundle, EvidenceError>;
    let bundle_reads: [BundleRead; 2] =
        [evidence::proof_bundle, evidence::proof_bundle_with_history];
    let height = 0x1234;
    let address = [0x12; 20];
    let slots = [[0x34; 32]];
    let proof = json!({
        "address": hex(&address),
        "balance": "0x1234",
        "nonce": "0x7",
        "codeHash": hex(&[0x56; 32]),
        "storageHash": hex(&[0x78; 32]),
        "accountProof": ["0xf843", "0xaabbcc"],
        "storageProof": [{"key": hex(&slots[0]), "value": "0xab", "proof": ["0xc0", "0x0102"]}]
    });
    let block = json!({
        "number": "0x1234",
        "hash": hex(&[0x9a; 32]),
        "parentHash": hex(&[0xbc; 32]),
        "timestamp": "0x67a1b2c3",
        "transactionsRoot": hex(&[0xde; 32]),
        "transactions": []
    });
    let proof_params = json!([hex(&address), [hex(&slots[0])], "0x1234"]);
    let block_params = json!(["0x1234", true]);
    let mut other_address = proof.clone();
    other_address["address"] = json!(hex(&[0x13; 20]));
    let mut other_height = block.clone();
    other_height["number"] = json!("0x1235");
    let mut malformed_proof = proof.clone();
    malformed_proof["accountProof"][0] = json!("0xgg");
    let refusal = RpcRefusal {
        code: -32000,
        message: "historical state unavailable".to_owned(),
        data: Some(json!({"height": "0x1234"})),
    };
    let error_response = json!({"error": {
        "code": refusal.code, "message": refusal.message, "data": refusal.data
    }});
    let exchanges = vec![
        (
            "eth_getProof",
            proof_params.clone(),
            json!({"result": proof}),
        ),
        (
            "eth_getBlockByNumber",
            block_params.clone(),
            json!({"result": block}),
        ),
        (
            "eth_getBlockByNumber",
            block_params.clone(),
            json!({"result": block}),
        ),
        (
            "eth_getProof",
            proof_params.clone(),
            json!({"result": proof}),
        ),
        (
            "eth_getBlockByNumber",
            block_params.clone(),
            json!({"result": null}),
        ),
        (
            "eth_getProof",
            proof_params.clone(),
            json!({"result": proof}),
        ),
        (
            "eth_getBlockByNumber",
            block_params.clone(),
            json!({"result": other_height}),
        ),
        (
            "eth_getProof",
            proof_params.clone(),
            json!({"result": other_address}),
        ),
        (
            "eth_getProof",
            proof_params.clone(),
            json!({"result": malformed_proof}),
        ),
        ("eth_getProof", proof_params.clone(), error_response.clone()),
        ("eth_getProof", proof_params, json!({"result": proof})),
        ("eth_getBlockByNumber", block_params, error_response),
    ];
    let exchanges: Vec<_> = (0..bundle_reads.len())
        .flat_map(|_| exchanges.clone())
        .collect();
    let expected_requests = exchanges.len();
    let listener = TcpListener::bind("127.0.0.1:0").must();
    let url = format!("http://{}/", listener.local_addr().must());
    let server = std::thread::spawn(move || {
        let mut requests = 0;
        for (method, params, mut response) in exchanges {
            let (stream, _) = listener.accept().must();
            stream.set_read_timeout(Some(Duration::from_secs(5))).must();
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(reader.read_line(&mut line).must(), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(value.trim().parse::<usize>().must());
                }
            }
            let mut body = vec![0; length.unwrap_or_else(|| panic!("missing content length"))];
            reader.read_exact(&mut body).must();
            let request: Value = serde_json::from_slice(&body).must();
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            response["jsonrpc"] = json!("2.0");
            response["id"] = request["id"].clone();
            let answer = response.to_string();
            let mut stream = reader.into_inner();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()).must();
            stream.flush().must();
            requests += 1;
        }
        requests
    });
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    let mut balance = [0; 32];
    balance[30..].copy_from_slice(&[0x12, 0x34]);
    let mut storage_value = [0; 32];
    storage_value[31] = 0xab;
    let expected_block = SequencerBlock {
        number: height,
        hash: [0x9a; 32],
        parent_hash: [0xbc; 32],
        timestamp: 0x67a1_b2c3,
        transactions_root: [0xde; 32],
        transactions: vec![],
    };
    let expected_bundle = ProofBundle {
        account_proof: AccountProof {
            address,
            balance,
            nonce: 7,
            code_hash: [0x56; 32],
            storage_hash: [0x78; 32],
            account_proof: vec![vec![0xf8, 0x43], vec![0xaa, 0xbb, 0xcc]],
            storage: vec![StorageProof {
                key: slots[0],
                value: storage_value,
                proof: vec![vec![0xc0], vec![1, 2]],
            }],
        },
        landing_block: expected_block.clone(),
    };
    for bundle_read in bundle_reads {
        assert_eq!(
            bundle_read(&endpoint, address, &slots, height),
            Ok(expected_bundle.clone())
        );
        assert_eq!(
            evidence::checkpoint(&endpoint, height),
            Ok(expected_block.clone())
        );
        assert_eq!(
            bundle_read(&endpoint, address, &slots, height),
            Err(EvidenceError::Rpc(SequencerError::Unavailable))
        );
        for _ in 0..3 {
            assert_eq!(
                bundle_read(&endpoint, address, &slots, height),
                Err(EvidenceError::Rpc(SequencerError::Malformed))
            );
        }
        for _ in 0..2 {
            assert_eq!(
                bundle_read(&endpoint, address, &slots, height),
                Err(EvidenceError::Rpc(SequencerError::Rpc(refusal.clone())))
            );
        }
    }
    assert_eq!(server.join().must(), expected_requests);
}

#[test]
fn verification_label_names_only_the_three_rungs() {
    let rungs = [Rung::Submitted, Rung::Receipted, Rung::Committed];
    let labels = rungs.map(evidence::verification_label);
    assert_eq!(labels, ["submitted", "receipted", "committed"]);
    for (rung, label) in rungs.into_iter().zip(labels) {
        assert_eq!(label, rung.name());
        assert_eq!(Rung::from_name(label), Some(rung));
    }
    for retired in [
        "state_proven",
        "checkpoint_finalised",
        "settlement_anchored",
    ] {
        assert!(!labels.contains(&retired));
        assert_eq!(Rung::from_name(retired), None);
    }
}
