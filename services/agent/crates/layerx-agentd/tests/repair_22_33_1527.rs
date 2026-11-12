fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_agentd::ops::program::{
    EmulatorProgramSimulationTransport, NativeProgramSimulationTransport,
    NodeProgramSimulationTransport, ProgramOperationError, ProgramOperations,
    ProgramSimulationEvidence,
};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_types::payload::ModuleRegistry;
use layerx_types::program_call::NativeProgramCall;
use layerx_wire::limits::MAX_MESSAGE_BYTES;
use serde_json::{json, Value};

enum Request {
    Simulation(Vec<u8>),
    Rpc(&'static str, Value),
}

fn serve(exchanges: Vec<(Request, Value)>) -> (String, JoinHandle<()>) {
    let listener = require_ok(TcpListener::bind("127.0.0.1:0"), "required test value");
    require_ok(listener.set_nonblocking(true), "required test value");
    let endpoint = format!(
        "http://{}",
        require_ok(listener.local_addr(), "required test value")
    );
    let worker = thread::spawn(move || {
        for (expected, mut answer) in exchanges {
            let deadline = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing production request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            require_ok(
                stream.set_read_timeout(Some(Duration::from_secs(5))),
                "required test value",
            );
            require_ok(
                stream.set_write_timeout(Some(Duration::from_secs(5))),
                "required test value",
            );
            let mut reader = BufReader::new(stream);
            let mut line = String::new();
            assert_ne!(
                require_ok(reader.read_line(&mut line), "required test value"),
                0
            );
            let path = match &expected {
                Request::Simulation(_) => "/v1/programs/simulate",
                Request::Rpc(_, _) => "/",
            };
            assert_eq!(line, format!("POST {path} HTTP/1.1\r\n"));
            let mut length = None;
            loop {
                line.clear();
                assert_ne!(
                    require_ok(reader.read_line(&mut line), "required test value"),
                    0
                );
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(require_ok(
                        value.trim().parse::<usize>(),
                        "required test value",
                    ));
                }
            }
            let length = require_some(length, "required test value");
            assert!(length < 64 * 1024);
            let mut body = vec![0; length];
            require_ok(reader.read_exact(&mut body), "required test value");
            match expected {
                Request::Simulation(signed) => assert_eq!(body, signed),
                Request::Rpc(method, params) => {
                    let request: Value =
                        require_ok(serde_json::from_slice(&body), "required test value");
                    assert_eq!(request["jsonrpc"], "2.0");
                    assert_eq!(request["method"], method);
                    assert_eq!(request["params"], params);
                    answer["jsonrpc"] = json!("2.0");
                    answer["id"] = request["id"].clone();
                }
            }
            let text = answer.to_string();
            let mut stream = reader.into_inner();
            require_ok(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{text}", text.len()), "required test value");
            require_ok(stream.flush(), "required test value");
        }
    });
    (endpoint, worker)
}

fn fixture() -> (Value, Vec<u8>, Vec<u8>) {
    let value: Value = require_ok(
        serde_json::from_str(include_str!(
            "../../../../../sdk/platform/conformance/fixtures/native-program-call-v3.json"
        )),
        "required test value",
    );
    let payload = require_ok(
        hex::decode(require_some(
            value["payload_hex"].as_str(),
            "required test value",
        )),
        "required test value",
    );
    let signed = require_ok(
        hex::decode(require_some(
            value["signed_activity_hex"].as_str(),
            "required test value",
        )),
        "required test value",
    );
    (value, payload, signed)
}

fn simulation_document() -> Value {
    json!({
        "verification_status": {"state": "Achieved", "level": "SequencerSigned"},
        "value": {
            "committed": false,
            "execution": {"receipt": "00aAfF", "terminal_payload": "0123456789aBcDeF", "call_graph": ""},
            "simulation_evidence": {
                "boundary_id": "11".repeat(32), "activity_id": "22".repeat(32),
                "previous_state_root": "33".repeat(32), "hypothetical_state_root": "44".repeat(32),
                "observed_sequence": "7", "observed_at": "100", "committed": false,
                "signature": "55".repeat(64)
            }
        }
    })
}

#[test]
fn program_hex_unicode_panic_simulation_fields_refuse_and_owner_remains_available() {
    let (_, payload, signed) = fixture();
    let native = require_ok(NativeProgramCall::decode(&payload), "required test value");
    let mut exchanges = Vec::new();
    let mut invalid = Vec::new();
    for field in ["receipt", "terminal_payload", "call_graph"] {
        for text in ["aéa", "0€", "😀", "é", "0", "0g", " 0", "+0", "0\n", "0\0"] {
            let mut document = simulation_document();
            document["value"]["execution"][field] = json!(text);
            invalid.push(document);
        }
    }
    for field in [
        "boundary_id",
        "activity_id",
        "previous_state_root",
        "hypothetical_state_root",
        "signature",
    ] {
        for text in [
            "aéa".to_owned(),
            "00".repeat(if field == "signature" { 65 } else { 33 }),
        ] {
            let mut document = simulation_document();
            document["value"]["simulation_evidence"][field] = json!(text);
            invalid.push(document);
        }
    }
    let cases = invalid.len();
    for document in invalid {
        exchanges.push((Request::Simulation(signed.clone()), document));
        exchanges.push((Request::Simulation(signed.clone()), simulation_document()));
    }
    let (endpoint, server) = serve(exchanges);
    let mut owner = require_ok(
        EmulatorProgramSimulationTransport::connect(&endpoint),
        "required test value",
    );
    for _ in 0..cases {
        assert!(matches!(
            owner.simulate_native_exact(native, 1000, &signed),
            Err(ProgramOperationError::UnverifiedReceipt)
        ));
        let recovered = require_ok(
            owner.simulate_native_exact(native, 1000, &signed),
            "required test value",
        );
        assert_eq!(recovered.receipt, [0, 0xaa, 0xff]);
        assert_eq!(
            recovered.terminal_payload,
            [1, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef]
        );
        assert!(recovered.call_graph.is_empty());
        assert_eq!(recovered.evidence_signature, [0x55; 64]);
    }
    require_ok(server.join(), "required test value");
}

#[test]
fn program_hex_simulation_bounds_are_enforced_before_decoding() {
    let (_, payload, signed) = fixture();
    let native = require_ok(NativeProgramCall::decode(&payload), "required test value");
    let mut exchanges = Vec::new();
    for field in ["receipt", "terminal_payload", "call_graph"] {
        for length in [MAX_MESSAGE_BYTES + 1, MAX_MESSAGE_BYTES] {
            let mut document = simulation_document();
            document["value"]["execution"][field] = json!("aF".repeat(length));
            exchanges.push((Request::Simulation(signed.clone()), document));
        }
    }
    let (endpoint, server) = serve(exchanges);
    let mut owner = require_ok(
        EmulatorProgramSimulationTransport::connect(&endpoint),
        "required test value",
    );
    for field in ["receipt", "terminal_payload", "call_graph"] {
        assert!(matches!(
            owner.simulate_native_exact(native, 1000, &signed),
            Err(ProgramOperationError::UnverifiedReceipt)
        ));
        let recovered = require_ok(
            owner.simulate_native_exact(native, 1000, &signed),
            "required test value",
        );
        let bytes = match field {
            "receipt" => recovered.receipt,
            "terminal_payload" => recovered.terminal_payload,
            _ => recovered.call_graph,
        };
        assert_eq!(bytes.len(), MAX_MESSAGE_BYTES);
        assert!(bytes.iter().all(|byte| *byte == 0xaf));
    }
    require_ok(server.join(), "required test value");
}

fn word(value: usize) -> [u8; 32] {
    let mut bytes = [0; 32];
    bytes[16..].copy_from_slice(&(value as u128).to_be_bytes());
    bytes
}

fn revert(reason: &str) -> String {
    let mut bytes = vec![0x08, 0xc3, 0x79, 0xa0];
    bytes.extend_from_slice(&word(32));
    bytes.extend_from_slice(&word(reason.len()));
    bytes.extend_from_slice(reason.as_bytes());
    bytes.resize(68 + reason.len().div_ceil(32) * 32, 0);
    format!("0x{}", hex::encode(bytes))
}

fn successful_call() -> Value {
    let mut bytes = word(0).to_vec();
    bytes.extend_from_slice(&word(1));
    bytes.extend_from_slice(&word(96));
    bytes.extend_from_slice(&word(6));
    bytes.extend_from_slice(b"answer");
    bytes.resize(160, 0);
    json!({"result": format!("0x{}", hex::encode(bytes))})
}

#[test]
fn program_hex_unicode_panic_node_reverts_refuse_and_owner_remains_available() {
    let (fixture, payload, _) = fixture();
    let native = require_ok(NativeProgramCall::decode(&payload), "required test value");
    let precompile = require_ok(
        ProgramOperations::<SequencerEndpoint>::call_precompile(native, 1000),
        "required test value",
    );
    let transaction = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 1_000_000,
        to: precompile.target.address(),
        data: require_ok(precompile.encode(), "required test value"),
    };
    assert_eq!(
        format!("0x{}", hex::encode(&transaction.data)),
        fixture["evm"]["calldata"]
    );
    let signed = require_ok(transaction.sign(&[0xa5; 32]), "required test value");
    let (_, signature) = require_ok(decode_signed(&signed), "required test value");
    let sender = require_ok(
        recover_sender(&transaction, &signature),
        "required test value",
    );
    let params = json!([{"from": format!("0x{}", hex::encode(sender)),
        "to": format!("0x{}", hex::encode(transaction.to)),
        "data": format!("0x{}", hex::encode(&transaction.data))}, "pending"]);
    let mut cases = ["aéa", "0€", "😀", "é", "0", "0g", " 0", "+0", "0\0"]
        .map(|text| {
            (
                format!("0x{text}"),
                ProgramOperationError::UnverifiedReceipt,
            )
        })
        .to_vec();
    cases.push((
        format!("0x{}", "00".repeat(MAX_MESSAGE_BYTES + 1)),
        ProgramOperationError::UnverifiedReceipt,
    ));
    // Error(string) is four selector bytes plus two words plus padded text.
    let near_bound_reason = format!("LXP_STALE: {}", "x".repeat(MAX_MESSAGE_BYTES - 100 - 11));
    let near_bound = revert(&near_bound_reason);
    assert_eq!((near_bound.len() - 2) / 2, MAX_MESSAGE_BYTES - 28);
    cases.push((near_bound, ProgramOperationError::Stale));
    let over_bound = revert(&format!("{near_bound_reason}{}", "x".repeat(32)));
    assert_eq!((over_bound.len() - 2) / 2, MAX_MESSAGE_BYTES + 4);
    cases.push((over_bound, ProgramOperationError::UnverifiedReceipt));
    for (reason, expected) in [
        (
            "LXP_NOT_FOUND: program",
            ProgramOperationError::ProgramStateAbsent,
        ),
        (
            "LXP_HEAD_ADVANCED: read",
            ProgramOperationError::HeadAdvanced,
        ),
        ("LXP_STALE: read", ProgramOperationError::Stale),
    ] {
        cases.push((revert(reason), expected.clone()));
        cases.push((
            format!("0x{}", revert(reason)[2..].to_ascii_uppercase()),
            expected,
        ));
    }
    let mut exchanges = Vec::new();
    for estimate in [true, false] {
        for (data, _) in &cases {
            if !estimate {
                exchanges.push((
                    Request::Rpc("eth_estimateGas", params.clone()),
                    json!({"result":"0x5208"}),
                ));
            }
            exchanges.push((
                Request::Rpc(
                    if estimate {
                        "eth_estimateGas"
                    } else {
                        "eth_call"
                    },
                    params.clone(),
                ),
                json!({"error":{"code":3,"message":"execution reverted","data":data}}),
            ));
            exchanges.push((
                Request::Rpc("eth_estimateGas", params.clone()),
                json!({"result":"0x5208"}),
            ));
            exchanges.push((Request::Rpc("eth_call", params.clone()), successful_call()));
        }
    }
    continue_program_hex_unicode_panic_node_reverts_refuse_and_owner_remains_available_1_repair_22_33_1527(ContinueProgramHexUnicodePanicNodeRevertsRefuseAndOwnerRemainsAvailable1Repair22331527Context { native, transaction, signed, sender, cases, exchanges });
}

struct ContinueProgramHexUnicodePanicNodeRevertsRefuseAndOwnerRemainsAvailable1Repair22331527Context<
    'a,
> {
    native: NativeProgramCall<'a>,
    transaction: Eip1559Call,
    signed: Vec<u8>,
    sender: [u8; 20],
    cases: Vec<(String, ProgramOperationError)>,
    exchanges: Vec<(Request, Value)>,
}

fn continue_program_hex_unicode_panic_node_reverts_refuse_and_owner_remains_available_1_repair_22_33_1527(
    context: ContinueProgramHexUnicodePanicNodeRevertsRefuseAndOwnerRemainsAvailable1Repair22331527Context<'_>,
) {
    let ContinueProgramHexUnicodePanicNodeRevertsRefuseAndOwnerRemainsAvailable1Repair22331527Context { native, transaction, signed, sender, cases, exchanges } = context;

    let (endpoint, server) = serve(exchanges);
    let mut endpoint = require_ok(
        SequencerEndpoint::new(&endpoint, &[]),
        "required test value",
    );
    let mut owner = NodeProgramSimulationTransport::new(
        &mut endpoint,
        require_ok(ModuleRegistry::new(&[]), "required test value"),
        42,
    );
    for _ in [true, false] {
        for (_, expected) in &cases {
            assert!(
                matches!(owner.simulate_native_exact(native, 1000, &signed), Err(actual) if actual == *expected)
            );
            let recovered = require_ok(
                owner.simulate_native_exact(native, 1000, &signed),
                "required test value",
            );
            match recovered.evidence {
                ProgramSimulationEvidence::EvmPending {
                    activity_id,
                    sender: actual,
                    to,
                    gas_estimate,
                    result,
                } => {
                    assert_eq!(activity_id, transaction_hash(&signed));
                    assert_eq!(actual, sender);
                    assert_eq!(to, transaction.to);
                    assert_eq!(gas_estimate, 0x5208);
                    assert_eq!(result.terminal_result, 0);
                    assert_eq!(result.terminal_kind, 1);
                    assert_eq!(result.response, b"answer");
                }
                other @ ProgramSimulationEvidence::SequencerSigned { .. } => {
                    panic!("unexpected evidence: {other:?}")
                }
            }
        }
    }
    require_ok(server.join(), "required test value");
}
