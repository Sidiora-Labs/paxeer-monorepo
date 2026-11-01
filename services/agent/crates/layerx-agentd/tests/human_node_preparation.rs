use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::human_node::HumanNode;
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::prepare::legacy::{
    CorePreparationBoundary, CoreStateError, ProductionCorePreparationBoundary,
};
use layerx_client::sequencer::{ADDR_PRECOMPILE, GET_EVM_ADDR_BY_LAYERX};
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::result::KnownResult;
use serde_json::{json, Value};

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
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

fn block(number: u64, timestamp: u64) -> Value {
    json!({
        "number": format!("{number:#x}"), "hash": format!("0x{}", "11".repeat(32)),
        "parentHash": format!("0x{}", "22".repeat(32)), "timestamp": format!("{timestamp:#x}"),
        "transactionsRoot": format!("0x{}", "33".repeat(32)),
        "receiptsRoot": format!("0x{}", "44".repeat(32)), "transactions": []
    })
}

#[test]
fn preparation_state_reads_chain_id_pending_nonce_and_block_time() {
    let public_key = ed25519_dalek::SigningKey::from_bytes(&[42; 32])
        .verifying_key()
        .to_bytes();
    let actor = must(Did::new(
        format!("did:layerx:{}", hex::encode(public_key)).as_bytes(),
    ));
    let address = [0x52; 20];
    let address_word = format!("0x{}{}", "00".repeat(12), hex::encode(address));
    let registry = must(ModuleRegistry::new(&[must(ModuleRegistration::new(
        ModuleId::Asset,
        &[must(ActivityType::new(ModuleId::Asset, 5))],
    ))]));
    let (url, server) = serve(vec![
        ("eth_chainId", json!("0xe5")),
        ("eth_call", json!(address_word)),
        ("eth_getTransactionCount", json!("0x7")),
        ("eth_blockNumber", json!("0x96")),
        ("eth_getBlockByNumber", block(150, 1_700_000_000)),
        ("eth_call", json!(address_word)),
        ("eth_getTransactionCount", json!("0x0")),
        ("eth_blockNumber", json!("0x97")),
        ("eth_getBlockByNumber", block(151, 1_700_000_012)),
    ]);
    let node: Box<dyn HumanNode> = Box::new(must(NodeRpc::connect(&NodeRpcConfig {
        url,
        chain_id: 229,
        deadline: Duration::from_secs(2),
    })));
    let mut boundary = ProductionCorePreparationBoundary::new(node.as_ref(), registry.clone());
    assert!(boundary.last_state().is_none());
    let state = must(boundary.preparation_state(&actor));
    assert_eq!(state.network_id, 229);
    assert_eq!(state.account_sequence, 7);
    assert_eq!(state.protocol_timestamp, 1_700_000_000);
    assert_eq!(state.observed_head_sequence, 150);
    assert_eq!(state.module_registry, registry);
    assert_eq!(boundary.last_state(), Some(&state));
    let next = must(boundary.preparation_state(&actor));
    assert_eq!(next.network_id, 229);
    assert_eq!(next.account_sequence, 0);
    assert_eq!(next.protocol_timestamp, 1_700_000_012);
    assert_eq!(next.observed_head_sequence, 151);
    assert_eq!(next.module_registry, registry);
    assert_eq!(boundary.last_state(), Some(&next));
    assert_eq!(
        boundary.preparation_state(&must(Did::new(b"did:layerx:invalid"))),
        Err(CoreStateError::Unverified)
    );
    assert!(boundary.last_state().is_none());

    let requests = must(server.join());
    assert_eq!(requests.len(), 9);
    assert_eq!(requests[0]["params"], json!([]));
    let data = [GET_EVM_ADDR_BY_LAYERX.as_slice(), public_key.as_slice()].concat();
    for index in [1, 5] {
        assert_eq!(
            requests[index]["params"],
            json!([{
            "to": format!("0x{}", hex::encode(ADDR_PRECOMPILE)),
            "data": format!("0x{}", hex::encode(&data)),
        }, "pending"])
        );
        assert_eq!(
            requests[index + 1]["params"],
            json!([format!("0x{}", hex::encode(address)), "pending"])
        );
        assert_eq!(requests[index + 2]["params"], json!([]));
    }
    assert_eq!(requests[4]["params"], json!(["0x96", true]));
    assert_eq!(requests[8]["params"], json!(["0x97", true]));

    let (url, server) = serve(vec![("eth_chainId", json!("0x100000000"))]);
    let node = must(NodeRpc::connect(&NodeRpcConfig {
        url,
        chain_id: u64::from(u32::MAX) + 1,
        deadline: Duration::from_secs(2),
    }));
    let mut boundary = ProductionCorePreparationBoundary::new(&node, registry);
    assert_eq!(
        boundary.preparation_state(&actor),
        Err(CoreStateError::Refused {
            class: 1,
            result: KnownResult::Overflow.into(),
        })
    );
    assert!(boundary.last_state().is_none());
    assert_eq!(must(server.join()).len(), 1);
}
