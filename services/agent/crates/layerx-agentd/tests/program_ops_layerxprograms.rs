use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::ops::program::{ProgramOperationError, ProgramOperations, ProgramRpcFailure};
use layerx_client::sequencer::{RpcRefusal, SequencerEndpoint, SequencerError};
use layerx_crypto::evm_transaction::transaction_hash;
use layerx_types::intent::ProgramId;
use layerx_types::program_call::{NativeProgramCall, Resources};
use serde_json::{json, Value};

fn serve(answer: Value) -> (String, JoinHandle<Value>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let url = format!("http://{}/", listener.local_addr().unwrap());
    listener.set_nonblocking(true).unwrap();
    let server = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(5);
        let stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(
                        Instant::now() < deadline,
                        "program operation sent no request"
                    );
                    thread::sleep(Duration::from_millis(1));
                }
                Err(error) => panic!("accept: {error}"),
            }
        };
        stream
            .set_read_timeout(Some(Duration::from_secs(2)))
            .unwrap();
        stream
            .set_write_timeout(Some(Duration::from_secs(2)))
            .unwrap();
        let mut reader = BufReader::new(stream);
        let mut length = None;
        loop {
            let mut line = String::new();
            assert_ne!(reader.read_line(&mut line).unwrap(), 0);
            if line == "\r\n" {
                break;
            }
            if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                length = Some(value.trim().parse::<usize>().unwrap());
            }
        }
        let length = length.expect("request content length");
        assert!(length < 64 * 1024);
        let mut body = vec![0; length];
        reader.read_exact(&mut body).unwrap();
        let request: Value = serde_json::from_slice(&body).unwrap();
        let mut response = answer;
        response["jsonrpc"] = json!("2.0");
        response["id"] = request["id"].clone();
        let text = response.to_string();
        let mut stream = reader.into_inner();
        write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{text}", text.len()).unwrap();
        stream.flush().unwrap();
        request
    });
    (url, server)
}

fn call() -> NativeProgramCall<'static> {
    NativeProgramCall {
        program_id: ProgramId::new([0x31; 32]),
        guest_abi: 1,
        entrypoint: b"layerx_call",
        calldata: &[0x01, 0x02, 0xff],
        capabilities: &[0, 0],
        access_declaration: b"LayerX/programs/access-declaration/v1\0\0",
        response_capacity: 1024,
        resources: Resources([1_000_000, 16_777_216, 100, 200, 4, 1024, 4096]),
    }
}

fn word(value: u128) -> [u8; 32] {
    let mut bytes = [0; 32];
    bytes[16..].copy_from_slice(&value.to_be_bytes());
    bytes
}

fn output(result: i32, kind: u8, response: &[u8]) -> Vec<u8> {
    let mut bytes = vec![if result < 0 { 0xff } else { 0 }; 32];
    bytes[28..].copy_from_slice(&result.to_be_bytes());
    bytes.extend_from_slice(&word(u128::from(kind)));
    bytes.extend_from_slice(&word(96));
    bytes.extend_from_slice(&word(response.len() as u128));
    bytes.extend_from_slice(response);
    bytes.resize(128 + response.len().div_ceil(32) * 32, 0);
    bytes
}

fn assert_request(request: &Value) {
    assert_eq!(request["jsonrpc"], "2.0");
    assert_eq!(request["method"], "eth_call");
    assert_eq!(request["params"][1], "pending");
    assert_eq!(
        request["params"][0]["to"],
        "0x000000000000000000000000000000000000101d"
    );
    let data = hex::decode(
        request["params"][0]["data"]
            .as_str()
            .unwrap()
            .strip_prefix("0x")
            .unwrap(),
    )
    .unwrap();
    let abi: Value = serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxprograms/abi.json"
    ))
    .unwrap();
    let function = abi
        .as_array()
        .unwrap()
        .iter()
        .find(|v| v["name"] == "call" && v["type"] == "function")
        .unwrap();
    let types = function["inputs"]
        .as_array()
        .unwrap()
        .iter()
        .map(|v| v["type"].as_str().unwrap())
        .collect::<Vec<_>>()
        .join(",");
    let selector = transaction_hash(format!("call({types})").as_bytes());
    assert_eq!(&data[..4], &selector[..4]);
    let args = &data[4..];
    assert_eq!(&args[..32], &[0x31; 32]);
    assert_eq!(&args[32..64], &word(1));
    // The fixed uint64[7] occupies seven words in the tuple head.
    assert_eq!(&args[64..96], &word(480));
    assert_eq!(&args[96..128], &word(544));
    assert_eq!(&args[128..160], &word(608));
    assert_eq!(&args[160..192], &word(672));
    assert_eq!(&args[192..224], &word(1024));
    for (index, resource) in call().resources.0.iter().enumerate() {
        assert_eq!(
            &args[224 + index * 32..256 + index * 32],
            &word(u128::from(*resource))
        );
    }
    assert_eq!(&args[448..480], &word(987));
    for (offset, body) in [
        (480, call().entrypoint),
        (544, call().calldata),
        (608, call().capabilities),
        (672, call().access_declaration),
    ] {
        assert_eq!(&args[offset..offset + 32], &word(body.len() as u128));
        assert_eq!(&args[offset + 32..offset + 32 + body.len()], body);
    }
    assert_eq!(args.len(), 768);
}

#[test]
fn program_simulation_uses_eth_call_at_pending() {
    for (native, code, kind, response) in [
        (false, 0, 0, vec![0x00, 0xab, 0xff]),
        (true, -7, 2, (0..65).collect::<Vec<u8>>()),
        (false, 0, 1, Vec::new()),
    ] {
        let (url, server) =
            serve(json!({"result": format!("0x{}", hex::encode(output(code, kind, &response)))}));
        let operations =
            ProgramOperations::from_endpoint(SequencerEndpoint::new(&url, &[]).unwrap());
        let result = if native {
            operations.simulate_native(call(), 987)
        } else {
            operations.simulate(call(), 987)
        }
        .unwrap();
        assert_eq!(result.terminal_result, code);
        assert_eq!(result.terminal_kind, kind);
        assert_eq!(result.response, response);
        assert_request(&server.join().unwrap());
    }
    for malformed in [
        vec![],
        {
            let mut value = output(0, 0, &[1]);
            value[95] = 64;
            value
        },
        {
            let mut value = output(0, 0, &[1]);
            *value.last_mut().unwrap() = 1;
            value
        },
        output(0, 0, &vec![1; 1025]),
    ] {
        let (url, server) = serve(json!({"result": format!("0x{}", hex::encode(malformed))}));
        let operations = ProgramOperations::connect(&url, &[]).unwrap();
        assert_eq!(
            operations.simulate(call(), 987),
            Err(ProgramOperationError::UnverifiedReceipt)
        );
        assert_request(&server.join().unwrap());
    }
}

#[test]
fn program_revert_maps_to_a_typed_operation_error() {
    for data in [
        Some(json!("0x08c379a00000deadbeef")),
        Some(json!({"data":"0xdeadbeef", "reason":"LXP_PROGRAM_REFUSED"})),
        Some(Value::Null),
        None,
    ] {
        let refusal = RpcRefusal {
            code: 3,
            message: "execution reverted: LXP_PROGRAM_REFUSED".to_owned(),
            data,
        };
        let mut error = json!({"code": refusal.code, "message": refusal.message});
        if let Some(data) = &refusal.data {
            error["data"] = data.clone();
        }
        let (url, server) = serve(json!({"error": error}));
        let operations = ProgramOperations::connect(&url, &[]).unwrap();
        assert_eq!(
            operations.simulate(call(), 987),
            Err(ProgramOperationError::Submit(
                ProgramRpcFailure::ExecutionReverted(refusal)
            ))
        );
        assert_request(&server.join().unwrap());
    }
    let refusal = RpcRefusal {
        code: -32602,
        message: "invalid params".to_owned(),
        data: Some(json!("bad call")),
    };
    let (url, server) = serve(
        json!({"error": {"code":refusal.code, "message":refusal.message, "data":refusal.data}}),
    );
    let operations = ProgramOperations::connect(&url, &[]).unwrap();
    assert_eq!(
        operations.simulate(call(), 987),
        Err(ProgramOperationError::Submit(ProgramRpcFailure::Sequencer(
            SequencerError::Rpc(refusal)
        )))
    );
    assert_request(&server.join().unwrap());
}
