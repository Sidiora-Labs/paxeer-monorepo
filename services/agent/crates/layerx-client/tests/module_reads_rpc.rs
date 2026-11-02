use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::time::Duration;

use layerx_client::rpc::module_state::{module_state, ModuleReadContext, ModuleReadError};
use layerx_client::sequencer::{SequencerEndpoint, SequencerError};
use serde_json::{json, Value};

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}

fn context() -> ModuleReadContext {
    ModuleReadContext {
        precompile: [0x12; 20],
        committed_height: 0x1234,
        block_hash: [0x9a; 32],
        app_hash: [0x56; 32],
    }
}

fn serve(
    exchanges: Vec<(&'static str, Value, Value)>,
) -> (SequencerEndpoint, std::thread::JoinHandle<()>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let endpoint = must(SequencerEndpoint::new(
        &format!("http://{}/", must(listener.local_addr())),
        &[],
    ));
    let handle = std::thread::spawn(move || {
        for (method, params, result) in exchanges {
            let (stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            let mut reader = BufReader::new(stream);
            let mut length = 0;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = must(value.trim().parse::<usize>());
                }
            }
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            let answer = json!({"jsonrpc":"2.0", "id": request["id"], "result":result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",answer.len()));
            must(stream.flush());
        }
    });
    (endpoint, handle)
}

fn exchanges(repeated: Value, hash: u8, root: u8) -> Vec<(&'static str, Value, Value)> {
    let call = json!([{"to":format!("0x{}","12".repeat(20)), "data":"0x01020304"}, "0x1234"]);
    vec![
        ("eth_call", call.clone(), json!("0x80ab")),
        ("eth_call", call, repeated),
        (
            "eth_getBlockByNumber",
            json!(["0x1234", true]),
            json!({
                "number":"0x1234", "hash":format!("0x{}",format!("{hash:02x}").repeat(32)),
                "parentHash":format!("0x{}","bc".repeat(32)), "timestamp":"0x42",
                "transactionsRoot":format!("0x{}","de".repeat(32)),
                "receiptsRoot":format!("0x{}","f0".repeat(32)), "transactions":[]
            }),
        ),
        (
            "eth_getBlockByNumber",
            json!(["0x1234", false]),
            json!({
                "number":"0x1234", "stateRoot":format!("0x{}",format!("{root:02x}").repeat(32))
            }),
        ),
    ]
}

#[test]
fn module_state_reads_the_storage_slot_proof_at_the_committed_height() {
    let (endpoint, handle) = serve(exchanges(json!("0x80ab"), 0x9a, 0x56));
    let value = must(module_state(&endpoint, &[1, 2, 3, 4], context()));
    assert_eq!(value.canonical_bytes(), &[0x80, 0xab]);
    assert_eq!(value.context(), &context());
    must(handle.join());
}

#[test]
fn module_state_refuses_a_storage_proof_for_another_slot() {
    for (result, hash, root) in [
        (json!("0x80ac"), 0x9a, 0x56),
        (json!("0x80ab"), 0x9b, 0x56),
        (json!("0x80ab"), 0x9a, 0x57),
    ] {
        let (endpoint, handle) = serve(exchanges(result, hash, root));
        assert_eq!(
            module_state(&endpoint, &[1, 2, 3, 4], context()),
            Err(ModuleReadError::CommittedChanged)
        );
        must(handle.join());
    }
    let (endpoint, handle) = serve(vec![]);
    let mut zero = context();
    zero.committed_height = 0;
    assert_eq!(
        module_state(&endpoint, &[1, 2, 3, 4], zero),
        Err(ModuleReadError::SelectorMismatch)
    );
    assert_eq!(
        module_state(&endpoint, &[1, 2, 3], context()),
        Err(ModuleReadError::SelectorMismatch)
    );
    must(handle.join());
    let mut malformed = exchanges(json!("0x80ab"), 0x9a, 0x56);
    malformed.truncate(1);
    malformed[0].2 = json!("0xgg");
    let (endpoint, handle) = serve(malformed);
    assert_eq!(
        module_state(&endpoint, &[1, 2, 3, 4], context()),
        Err(ModuleReadError::Rpc(SequencerError::Malformed))
    );
    must(handle.join());
}
