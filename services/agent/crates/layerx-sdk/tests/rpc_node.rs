use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
use layerx_sdk::rpc_verification::ReceiptPolicy;
use serde_json::{json, Value};

const ADDRESS: &str = "0x1111111111111111111111111111111111111111";

fn listener(results: Vec<Value>) -> (RpcClient, JoinHandle<Vec<Value>>) {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind loopback RPC");
    let endpoint = format!(
        "http://{}",
        listener.local_addr().expect("listener address")
    );
    listener
        .set_nonblocking(true)
        .expect("nonblocking listener");
    let server = thread::spawn(move || {
        let mut requests = Vec::new();
        for result in results {
            let deadline = Instant::now() + Duration::from_secs(10);
            let mut socket = loop {
                match listener.accept() {
                    Ok((socket, _)) => break socket,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing RPC request");
                        thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => panic!("accept RPC: {error}"),
                }
            };
            socket
                .set_read_timeout(Some(Duration::from_secs(5)))
                .expect("read timeout");
            socket
                .set_write_timeout(Some(Duration::from_secs(5)))
                .expect("write timeout");
            let mut header = Vec::new();
            while !header.ends_with(b"\r\n\r\n") {
                assert!(header.len() < 8192, "HTTP header bound");
                let mut byte = [0];
                socket.read_exact(&mut byte).expect("HTTP header");
                header.push(byte[0]);
            }
            let header = std::str::from_utf8(&header).expect("UTF-8 header");
            assert!(header.starts_with("POST /rpc HTTP/1.1\r\n"));
            let length = header
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse::<usize>().expect("content length"))
                })
                .expect("content-length header");
            assert!(length <= 1_048_576 + 4096, "HTTP request bound");
            let mut body = vec![0; length];
            socket.read_exact(&mut body).expect("HTTP body");
            let request: Value = serde_json::from_slice(&body).expect("JSON-RPC request");
            assert_eq!(request["jsonrpc"], "2.0");
            assert!(request["method"].is_string());
            let body = serde_json::to_vec(&json!({
                "jsonrpc": "2.0", "id": request["id"], "result": result,
            }))
            .expect("JSON-RPC response");
            write!(socket, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).expect("response header");
            socket.write_all(&body).expect("response body");
            requests.push(request);
        }
        requests
    });
    (
        RpcClient::connect(&endpoint, None).expect("connect RPC"),
        server,
    )
}

fn call() -> Eip1559Call {
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [0x1a; 20],
        data: vec![0x12, 0x34],
    }
}

fn asset(id: u8) -> Value {
    json!({
        "asset_id": format!("{id:02x}").repeat(32),
        "symbol": "USD", "name": "Test Dollar", "decimals": 6,
        "custody_kind": 1, "custody_reference": "", "paused": false,
        "supply_cap": "1000000", "issuer_did": "22".repeat(32),
        "issuer_kind": 1, "total_units": "100", "salt": "33".repeat(32),
    })
}

fn page(ids: &[u8], next: Option<u8>) -> Value {
    json!({
        "assets": ids.iter().copied().map(asset).collect::<Vec<_>>(),
        "next_cursor": next.map(|id| format!("{id:02x}").repeat(32)),
        "observed_head_sequence": "9", "state_root": "44".repeat(32),
        "verification": "authenticated_committed_snapshot",
    })
}

#[test]
fn reads_use_only_eth_and_px_methods() {
    let (rpc, server) = listener(vec![
        json!({"account": "joined"}),
        json!("0x10"),
        json!("0x7"),
        json!({"status": "0x1"}),
        json!({"number": "0x9"}),
        json!("0x7"),
        json!({"balances": []}),
        json!({"chainId": "0x7d"}),
        page(&[0x11], None),
        page(&[0x22], None),
        page(&[0x11], Some(0x11)),
        page(&[0x22], None),
        json!("0x5208"),
        json!({"accountProof": [], "storageProof": []}),
    ]);
    assert_eq!(
        rpc.get_account("did:layerx:alice").expect("account")["account"],
        "joined"
    );
    assert_eq!(rpc.get_balance(ADDRESS).expect("balance"), "0x10");
    assert_eq!(rpc.get_sequence(ADDRESS).expect("sequence"), "0x7");
    assert_eq!(
        rpc.get_receipt(&format!("0x{}", "ab".repeat(32)))
            .expect("receipt")["status"],
        "0x1"
    );
    assert_eq!(rpc.get_batch_header("0x9").expect("block")["number"], "0x9");
    assert_eq!(
        rpc.get_identity_sequence(ADDRESS).expect("pending nonce"),
        7
    );
    assert_eq!(
        rpc.get_balances("did:layerx:alice")
            .expect("balances")
            .into_value(),
        json!({"balances": []})
    );
    assert_eq!(rpc.get_node_info().expect("network")["chainId"], "0x7d");
    assert_eq!(
        rpc.list_assets().expect("assets").assets[0].asset_id,
        [0x11; 32]
    );
    assert_eq!(
        rpc.list_assets_page(Some([0x11; 32]), Some(1))
            .expect("asset page")
            .assets[0]
            .asset_id,
        [0x22; 32]
    );
    let selected = rpc.get_asset([0x22; 32]).expect("asset on second page");
    assert_eq!(selected.asset.asset_id, [0x22; 32]);
    assert_eq!(selected.asset.symbol, "USD");
    assert_eq!(selected.observed_head_sequence, 9);
    assert_eq!(selected.state_root, [0x44; 32]);
    assert_eq!(rpc.estimate_fee(&call()).expect("gas estimate"), 21_000);
    assert_eq!(
        rpc.get_proof(ADDRESS, &[[0x55; 32]], "0x9").expect("proof")["storageProof"],
        json!([])
    );

    let requests = server.join().expect("RPC listener");
    let methods: Vec<_> = requests
        .iter()
        .map(|request| request["method"].as_str().expect("method name"))
        .collect();
    assert!(
        methods
            .iter()
            .all(|method| method.starts_with("eth_") || method.starts_with("px_")),
        "{methods:?}"
    );
    assert_eq!(
        methods,
        [
            "px_getAccount",
            "eth_getBalance",
            "eth_getTransactionCount",
            "eth_getTransactionReceipt",
            "eth_getBlockByNumber",
            "eth_getTransactionCount",
            "px_getBalances",
            "px_getNetwork",
            "px_listAssets",
            "px_listAssets",
            "px_listAssets",
            "px_listAssets",
            "eth_estimateGas",
            "eth_getProof",
        ]
    );
    assert_eq!(requests[0]["params"], json!(["did:layerx:alice"]));
    for index in [1, 2, 5] {
        assert_eq!(requests[index]["params"], json!([ADDRESS, "pending"]));
    }
    assert_eq!(
        requests[3]["params"],
        json!([format!("0x{}", "ab".repeat(32))])
    );
    assert_eq!(requests[4]["params"], json!(["0x9", false]));
    assert_eq!(requests[6]["params"], json!(["did:layerx:alice"]));
    assert_eq!(requests[7]["params"], json!([]));
    assert_eq!(requests[8]["params"], json!([]));
    assert_eq!(requests[9]["params"], json!(["11".repeat(32), 1]));
    assert_eq!(requests[10]["params"], json!([]));
    assert_eq!(requests[11]["params"], json!(["11".repeat(32)]));
    assert_eq!(
        requests[12]["params"],
        json!([{
        "type": "0x2", "chainId": "0x7d", "nonce": "0x7",
        "maxPriorityFeePerGas": "0x1", "maxFeePerGas": "0x2", "gas": "0x186a0",
        "to": format!("0x{}", "1a".repeat(20)), "value": "0x0", "data": "0x1234", "accessList": [],
    }, "pending"])
    );
    assert_eq!(
        requests[13]["params"],
        json!([ADDRESS, [format!("0x{}", "55".repeat(32))], "0x9"])
    );
}

#[test]
fn finalised_wait_is_refused_without_reading_a_checkpoint() {
    let (rpc, server) = listener(vec![json!({"chainId": "0x7d"})]);
    let policy = ReceiptPolicy {
        protocol_version: 3,
        network_id: 125,
        sequencer: SequencerAuthorization::new([1; 32], [2; 32], 1, 100),
        trusted_checkpoint_context_digest: Some([0x33; 32]),
    };
    assert!(matches!(
        rpc.wait_for(
            [0xab; 32],
            Commitment::Finalised,
            &policy,
            Duration::from_secs(1)
        ),
        Err(RpcError::MissingFinalityTrust)
    ));
    assert_eq!(rpc.get_node_info().expect("network")["chainId"], "0x7d");
    let requests = server.join().expect("RPC listener");
    assert!(requests.iter().all(|request| {
        request["method"] != "lx_getNodeInfo" && request["method"] != "lx_getCheckpoint"
    }));
    assert_eq!(requests.len(), 1);
    assert_eq!(requests[0]["method"], "px_getNetwork");
    assert_eq!(requests[0]["params"], json!([]));
}

#[test]
fn send_raw_transaction_returns_the_hash_at_the_submitted_rung() {
    let hash = [0xab; 32];
    let hash_text = format!("0x{}", "ab".repeat(32));
    let raw = call().sign(&[1; 32]).expect("sign EIP-1559 transaction");
    let (rpc, server) = listener(vec![
        json!(hash_text),
        json!({"txHash": hash_text, "rung": "submitted"}),
        json!("ab".repeat(32)),
        json!("0xab"),
        json!(format!("0x{}", "gg".repeat(32))),
        json!({"txHash": hash_text}),
        Value::Null,
    ]);
    assert_eq!(
        rpc.send_raw_transaction(&raw).expect("submitted hash"),
        hash
    );
    let receipt = rpc.sequencer_receipt(hash).expect("sequencer status");
    assert_eq!(receipt["rung"], "submitted");
    assert_eq!(receipt["txHash"], hash_text);
    assert!(receipt.get("receipt").is_none());
    for _ in 0..5 {
        assert!(matches!(
            rpc.send_raw_transaction(&raw),
            Err(RpcError::InvalidResponse)
        ));
    }
    assert!(matches!(
        rpc.send_raw_transaction(&[]),
        Err(RpcError::InvalidRequest)
    ));
    assert!(matches!(
        rpc.send_raw_transaction(&vec![0; 524_289]),
        Err(RpcError::InvalidRequest)
    ));
    let requests = server.join().expect("RPC listener");
    let raw_hex = raw
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>();
    assert_eq!(requests.len(), 7);
    for (index, request) in requests.iter().enumerate() {
        if index == 1 {
            assert_eq!(request["method"], "pax_sequencerReceipt");
            assert_eq!(request["params"], json!([hash_text]));
        } else {
            assert_eq!(request["method"], "eth_sendRawTransaction");
            assert_eq!(request["params"], json!([format!("0x{raw_hex}")]));
        }
    }
}

#[test]
fn identity_sequence_reads_the_pending_nonce() {
    let (rpc, server) = listener(vec![
        json!("0x2a"),
        json!("0x0"),
        json!("0xffffffffffffffff"),
        json!("0x00"),
        json!("0x10000000000000000"),
        json!("42"),
        json!(42),
        json!({"next_sequence": "42"}),
        Value::Null,
    ]);
    for expected in [42, 0, u64::MAX] {
        assert_eq!(
            rpc.get_identity_sequence(ADDRESS).expect("pending nonce"),
            expected
        );
    }
    for _ in 0..6 {
        assert!(matches!(
            rpc.get_identity_sequence(ADDRESS),
            Err(RpcError::InvalidResponse)
        ));
    }
    for invalid in [
        "did:layerx:alice",
        "",
        "0x11",
        "1111111111111111111111111111111111111111",
        "0xgg11111111111111111111111111111111111111",
    ] {
        assert!(matches!(
            rpc.get_identity_sequence(invalid),
            Err(RpcError::InvalidRequest)
        ));
    }
    let requests = server.join().expect("RPC listener");
    assert_eq!(requests.len(), 9);
    for request in requests {
        assert_eq!(request["method"], "eth_getTransactionCount");
        assert_eq!(request["params"], json!([ADDRESS, "pending"]));
    }
}
