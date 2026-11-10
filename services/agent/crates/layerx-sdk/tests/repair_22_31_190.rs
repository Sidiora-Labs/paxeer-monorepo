use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::ReceiptKeys;
use layerx_sdk::program_lifecycle::{
    deploy_calldata, programs_module_registry, upgrade_calldata, verify_lifecycle_receipt,
    wind_down_calldata, NativeProgramLifecycleRequest,
};
use layerx_sdk::programs::{HttpProgramTransport, ProgramLifecycleSubmission, ProgramTransport};
use layerx_sdk::rpc::{LogFilter, LogPosition, RpcClient, RpcError, SubscriptionTopic};
use layerx_types::clock::{Clock, ClockError, ClockReading};
use layerx_types::program_lifecycle::{
    NativeProgramDeploy, NativeProgramUpgrade, NativeProgramWindDown, ProgramWindDownOperation,
};
use serde_json::{json, Value};
use tungstenite::{Message, WebSocket};

const PROGRAMS: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1d,
];

fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut text = String::from("0x");
    for byte in bytes {
        write!(&mut text, "{byte:02x}").unwrap_or_else(|error| panic!("hex: {error:?}"));
    }
    text
}

fn bytes(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair).unwrap_or_else(|error| panic!("hex UTF-8: {error:?}")),
                16,
            )
            .unwrap_or_else(|error| panic!("hex byte: {error:?}"))
        })
        .collect()
}

fn lifecycle_fixture(name: &str) -> Value {
    let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../sdk/platform/conformance/fixtures")
        .join(format!("native-program-{name}-v4.json"));
    serde_json::from_slice(
        &std::fs::read(path).unwrap_or_else(|error| panic!("native producer fixture: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("fixture JSON: {error:?}"))
}

fn sequencer_vectors() -> Value {
    let document: Value = serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(|error| panic!("sequencer producer vectors: {error:?}"));
    assert_eq!(
        document["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    document
}

fn transaction(data: Vec<u8>) -> Eip1559Call {
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 2_000_000,
        to: PROGRAMS,
        data,
    }
}

fn bind(
    fixture: &Value,
    signed: &[u8],
) -> Result<NativeProgramLifecycleRequest, layerx_sdk::programs::ProgramOperationError> {
    let registry = programs_module_registry()?;
    let payload = bytes(
        fixture["payload_hex"]
            .as_str()
            .unwrap_or_else(|| panic!("native payload")),
    );
    match fixture["ordinal"]
        .as_u64()
        .unwrap_or_else(|| panic!("ordinal"))
    {
        1 => NativeProgramLifecycleRequest::deploy(
            &registry,
            NativeProgramDeploy::decode(&payload)
                .unwrap_or_else(|error| panic!("native deploy codec: {error:?}")),
            signed,
        ),
        2 => NativeProgramLifecycleRequest::upgrade_with_fee_limit(
            &registry,
            NativeProgramUpgrade::decode(&payload)
                .unwrap_or_else(|error| panic!("native upgrade codec: {error:?}")),
            1000,
            signed,
        ),
        7 => NativeProgramLifecycleRequest::wind_down(
            &registry,
            NativeProgramWindDown::decode(&payload)
                .unwrap_or_else(|error| panic!("native wind-down codec: {error:?}")),
            signed,
        ),
        ordinal => panic!("unexpected ordinal {ordinal}"),
    }
}

#[test]
fn finding_017_lifecycle_abi_round_trips_signed_chain_125_transactions() {
    for name in [
        "deploy",
        "upgrade",
        "wind-down-route",
        "wind-down-deprecate",
        "wind-down-tombstone",
        "wind-down-exit",
    ] {
        let fixture = lifecycle_fixture(name);
        assert_eq!(fixture["evm"]["chainId"], 125);
        let payload = bytes(
            fixture["payload_hex"]
                .as_str()
                .unwrap_or_else(|| panic!("native payload")),
        );
        let expected = bytes(
            fixture["evm"]["calldata"]
                .as_str()
                .unwrap_or_else(|| panic!("merged ABI producer bytes")),
        );
        let encoded = encode_lifecycle_case(name, &payload);
        assert_eq!(encoded, expected, "{name}");
        let call = transaction(encoded);
        let signed = call
            .sign(&[0x42; 32])
            .unwrap_or_else(|error| panic!("sign EIP-1559: {error:?}"));
        let request = bind(&fixture, &signed)
            .unwrap_or_else(|error| panic!("bind the merged ABI: {error:?}"));
        let (decoded, signature) = decode_signed(request.signed_activity())
            .unwrap_or_else(|error| panic!("transport codec: {error:?}"));
        assert_eq!(decoded, call);
        assert_ne!(
            recover_sender(&decoded, &signature)
                .unwrap_or_else(|error| panic!("recover sender: {error:?}")),
            [0; 20]
        );
        assert_eq!(request.bound_activity_id(), transaction_hash(&signed));
        assert_eq!(request.bound_idempotency_key(), transaction_hash(&signed));
        assert!(bind(
            &fixture,
            &transaction(payload.clone())
                .sign(&[0x42; 32])
                .unwrap_or_else(|error| panic!("sign historical payload: {error:?}"))
        )
        .is_err());
        for chain_id in [1, 124, 126] {
            let wrong = Eip1559Call {
                chain_id,
                ..call.clone()
            }
            .sign(&[0x42; 32])
            .unwrap_or_else(|error| panic!("sign wrong chain: {error:?}"));
            assert!(bind(&fixture, &wrong).is_err(), "{name} chain {chain_id}");
        }
        let wrong = Eip1559Call {
            to: [0x11; 20],
            ..call.clone()
        }
        .sign(&[0x42; 32])
        .unwrap_or_else(|error| panic!("sign wrong destination: {error:?}"));
        assert!(bind(&fixture, &wrong).is_err());
        let mut changed = call.clone();
        changed.data.push(0);
        assert!(bind(
            &fixture,
            &changed
                .sign(&[0x42; 32])
                .unwrap_or_else(|error| panic!("sign trailing data: {error:?}"))
        )
        .is_err());
        for length in [0, 1, signed.len() - 1] {
            assert!(bind(&fixture, &signed[..length]).is_err());
        }
        if name == "upgrade" {
            let value = NativeProgramUpgrade::decode(&payload)
                .unwrap_or_else(|error| panic!("upgrade: {error:?}"));
            assert!(NativeProgramLifecycleRequest::upgrade_with_fee_limit(
                &programs_module_registry().unwrap_or_else(|error| panic!("registry: {error:?}")),
                value,
                999,
                &signed
            )
            .is_err());
            assert_eq!(
                NativeProgramLifecycleRequest::upgrade(
                    &programs_module_registry()
                        .unwrap_or_else(|error| panic!("registry: {error:?}")),
                    value,
                    &signed
                )
                .unwrap_or_else(|error| panic!("retained upgrade entry point: {error:?}")),
                request
            );
        }
    }
}

#[test]
fn finding_017_upgrade_flags_and_bounded_exit_are_bound() {
    let fixture = lifecycle_fixture("upgrade");
    let payload = bytes(
        fixture["payload_hex"]
            .as_str()
            .unwrap_or_else(|| panic!("payload")),
    );
    let original =
        NativeProgramUpgrade::decode(&payload).unwrap_or_else(|error| panic!("upgrade: {error:?}"));
    let registry = programs_module_registry().unwrap_or_else(|error| panic!("registry: {error:?}"));
    for value in [
        NativeProgramUpgrade {
            migration_hook: b"migrate",
            ..original
        },
        NativeProgramUpgrade {
            clear_interface: true,
            interface: Some(&[]),
            ..original
        },
    ] {
        let data = upgrade_calldata(value, u128::MAX)
            .unwrap_or_else(|error| panic!("upgrade ABI: {error:?}"));
        assert_eq!(
            data[4 + 2 * 32 + 31],
            if value.clear_interface { 2 } else { 1 }
        );
        let signed = transaction(data)
            .sign(&[0x42; 32])
            .unwrap_or_else(|error| panic!("sign: {error:?}"));
        assert!(NativeProgramLifecycleRequest::upgrade_with_fee_limit(
            &registry,
            value,
            u128::MAX,
            &signed
        )
        .is_ok());
        assert!(NativeProgramLifecycleRequest::upgrade_with_fee_limit(
            &registry,
            original,
            u128::MAX,
            &signed
        )
        .is_err());
    }
    let wind = NativeProgramWindDown {
        program_id: original.program_id,
        operation: ProgramWindDownOperation::BoundedExit {
            account: [0x33; 32],
            maximum_exit_amount: 1,
        },
    };
    let data =
        wind_down_calldata(wind).unwrap_or_else(|error| panic!("bounded exit ABI: {error:?}"));
    assert_eq!(&data[..4], &[0xad, 0xe5, 0xd9, 0xe6]);
    assert_eq!(data.len(), 4 + 3 * 32);
    assert_eq!(data[data.len() - 1], 1);
    let signed = transaction(data)
        .sign(&[0x42; 32])
        .unwrap_or_else(|error| panic!("sign: {error:?}"));
    assert!(NativeProgramLifecycleRequest::wind_down(&registry, wind, &signed).is_ok());
    assert!(NativeProgramLifecycleRequest::wind_down(
        &registry,
        NativeProgramWindDown {
            operation: ProgramWindDownOperation::Exit {
                account: [0x33; 32]
            },
            ..wind
        },
        &signed
    )
    .is_err());
    assert!(wind_down_calldata(NativeProgramWindDown {
        operation: ProgramWindDownOperation::BoundedExit {
            account: [0x33; 32],
            maximum_exit_amount: 0
        },
        ..wind
    })
    .is_err());
}

fn authorization(case: &Value) -> SequencerAuthorization {
    let key = &case["pinnedKey"];
    SequencerAuthorization::new(
        bytes(key["keyId"].as_str().unwrap_or_else(|| panic!("key id")))
            .try_into()
            .unwrap_or_else(|error| panic!("id length: {error:?}")),
        bytes(key["publicKey"].as_str().unwrap_or_else(|| panic!("key")))
            .try_into()
            .unwrap_or_else(|error| panic!("key length: {error:?}")),
        key["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("first height")),
        key["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("last height")),
    )
}

fn programs_case(document: &Value) -> &Value {
    document["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("cases"))
        .iter()
        .find(|case| case["id"] == "V1-layerxprograms-call")
        .unwrap_or_else(|| panic!("Programs sequencer producer"))
}

#[test]
fn finding_018_real_sequencer_receipt_requires_height_pins_and_programs_identity() {
    let vectors = sequencer_vectors();
    let case = programs_case(&vectors);
    let receipt = bytes(
        case["signedReceipt"]
            .as_str()
            .unwrap_or_else(|| panic!("signed receipt")),
    );
    let raw = bytes(
        case["rawTransaction"]
            .as_str()
            .unwrap_or_else(|| panic!("real signed transaction")),
    );
    let hash = transaction_hash(&raw);
    let pin = authorization(case);
    let pins = [pin];
    let verified = verify_lifecycle_receipt(&receipt, hash, ReceiptKeys::HeightRanges(&pins))
        .unwrap_or_else(|error| panic!("retained sequencer verification: {error:?}"));
    let protocol = verified.protocol().unwrap_or_else(|| panic!("protocol"));
    assert_eq!(protocol.protocol_version(), 2);
    assert_eq!(protocol.module_id(), 12);
    assert_eq!(protocol.operation(), 2);
    assert_eq!(protocol.activity_id(), hash);
    assert_eq!(protocol.to()[12..], PROGRAMS);
    assert!(verify_lifecycle_receipt(&receipt, [0x55; 32], &pins).is_err());
    assert!(verify_lifecycle_receipt(&receipt, hash, pin.public_key()).is_err());
    for pins in [
        vec![],
        vec![pin, pin],
        vec![SequencerAuthorization::new(
            pin.sequencer_id(),
            [0x51; 32],
            pin.first_batch_number(),
            pin.last_batch_number(),
        )],
        vec![SequencerAuthorization::new(
            pin.sequencer_id(),
            pin.public_key(),
            0,
            999,
        )],
    ] {
        assert!(verify_lifecycle_receipt(&receipt, hash, pins.as_slice()).is_err());
    }
    let mut tampered = receipt.clone();
    let last = tampered.len() - 1;
    tampered[last] ^= 1;
    assert!(verify_lifecycle_receipt(&tampered, hash, &pins).is_err());
    let unrelated = &vectors["cases"][0];
    let unrelated_raw = bytes(
        unrelated["rawTransaction"]
            .as_str()
            .unwrap_or_else(|| panic!("PAX producer transaction")),
    );
    assert!(verify_lifecycle_receipt(
        &bytes(
            unrelated["signedReceipt"]
                .as_str()
                .unwrap_or_else(|| panic!("PAX receipt"))
        ),
        transaction_hash(&unrelated_raw),
        &[authorization(unrelated)]
    )
    .is_err());
}

fn http_listener(replies: Vec<(&'static str, Value)>) -> (String, JoinHandle<Vec<Value>>) {
    let listener =
        TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("node listener: {error:?}"));
    let endpoint = format!(
        "http://{}/",
        listener
            .local_addr()
            .unwrap_or_else(|error| panic!("address: {error:?}"))
    );
    let server = thread::spawn(move || {
        let mut seen = Vec::new();
        for (method, result) in replies {
            let (stream, _) = listener
                .accept()
                .unwrap_or_else(|error| panic!("node accept: {error:?}"));
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap_or_else(|error| panic!("read deadline: {error:?}"));
            let mut reader = BufReader::new(stream);
            let mut line = String::new();
            reader
                .read_line(&mut line)
                .unwrap_or_else(|error| panic!("request line: {error:?}"));
            assert_eq!(line, "POST / HTTP/1.1\r\n");
            let mut length = None;
            loop {
                line.clear();
                assert_ne!(
                    reader
                        .read_line(&mut line)
                        .unwrap_or_else(|error| panic!("header: {error:?}")),
                    0
                );
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(
                        value
                            .trim()
                            .parse::<usize>()
                            .unwrap_or_else(|error| panic!("body length: {error:?}")),
                    );
                }
            }
            let mut body = vec![0; length.unwrap_or_else(|| panic!("content length"))];
            reader
                .read_exact(&mut body)
                .unwrap_or_else(|error| panic!("request body: {error:?}"));
            let request: Value = serde_json::from_slice(&body)
                .unwrap_or_else(|error| panic!("request JSON: {error:?}"));
            assert_eq!(request["method"], method);
            let response =
                json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
            let mut socket = reader.into_inner();
            write!(socket, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()).unwrap_or_else(|error| panic!("node response: {error:?}"));
            seen.push(request);
        }
        seen
    });
    (endpoint, server)
}

#[test]
fn finding_018_lifecycle_transport_uses_node_rpc_and_rejects_an_unrelated_real_receipt() {
    let vectors = sequencer_vectors();
    let case = programs_case(&vectors);
    let authorization = authorization(case);
    let keys = [ReceiptKeyEntry {
        key_id: authorization.sequencer_id(),
        public_key: authorization.public_key(),
        first_height: authorization.first_batch_number(),
        last_height: authorization.last_batch_number(),
    }];
    let fixture = lifecycle_fixture("deploy");
    let signed = transaction(bytes(
        fixture["evm"]["calldata"]
            .as_str()
            .unwrap_or_else(|| panic!("calldata")),
    ))
    .sign(&[0x42; 32])
    .unwrap_or_else(|error| panic!("sign: {error:?}"));
    let request = bind(&fixture, &signed).unwrap_or_else(|error| panic!("request: {error:?}"));
    let (endpoint, server) = http_listener(vec![
        (
            "eth_sendRawTransaction",
            json!(hex(&request.bound_activity_id())),
        ),
        ("pax_sequencerReceipt", Value::Null),
        ("pax_sequencerReceipt", case["receiptJson"].clone()),
    ]);
    let transport = HttpProgramTransport::connect_node(&endpoint, &[], &keys, || Ok(0))
        .unwrap_or_else(|error| panic!("node transport: {error:?}"));
    let result = transport
        .deploy(&request, request.bound_idempotency_key())
        .unwrap_or_else(|error| panic!("submit: {error:?}"));
    assert_eq!(
        result,
        ProgramLifecycleSubmission::Unknown {
            activity_id: request.bound_activity_id(),
            idempotency_key: request.bound_idempotency_key(),
            retained_signed_activity: signed.clone()
        }
    );
    drop(transport);
    let reopened = HttpProgramTransport::connect_node(&endpoint, &[], &keys, || Ok(0))
        .unwrap_or_else(|error| panic!("reopen transport: {error:?}"));
    assert!(reopened.lifecycle_receipt(&request).is_err());
    let seen = server
        .join()
        .unwrap_or_else(|error| panic!("node listener: {error:?}"));
    assert_eq!(seen[0]["params"], json!([hex(&signed)]));
    assert_eq!(
        seen[1]["params"],
        json!([hex(&request.bound_activity_id())])
    );
    assert_eq!(seen[2]["params"], seen[1]["params"]);
    assert!(HttpProgramTransport::connect_node(&endpoint, &[], &[], || Ok(0)).is_err());
    let unpinned =
        HttpProgramTransport::connect(&endpoint, None, authorization.public_key(), || Ok(0))
            .unwrap_or_else(|error| panic!("historical transport: {error:?}"));
    assert!(unpinned
        .deploy(&request, request.bound_idempotency_key())
        .is_err());
}

struct MonotonicClock {
    start: Instant,
}
impl Clock for MonotonicClock {
    fn sample(&self, _budget: Duration) -> Result<ClockReading, ClockError> {
        Ok(ClockReading {
            generation: [1; 16],
            unix_milliseconds: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map_err(|_| ClockError::Invalid)?
                .as_millis()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
            monotonic_nanoseconds: self
                .start
                .elapsed()
                .as_nanos()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
        })
    }
}

fn client(endpoint: &str) -> RpcClient {
    RpcClient::connect_node(endpoint, None)
        .unwrap_or_else(|error| panic!("node client: {error:?}"))
        .with_clock(Arc::new(MonotonicClock {
            start: Instant::now(),
        }))
}

fn receive(socket: &mut WebSocket<TcpStream>) -> Value {
    match socket
        .read()
        .unwrap_or_else(|error| panic!("read WebSocket: {error:?}"))
    {
        Message::Text(text) => {
            serde_json::from_str(&text).unwrap_or_else(|error| panic!("JSON-RPC: {error:?}"))
        }
        message => panic!("unexpected frame {message:?}"),
    }
}
fn send(socket: &mut WebSocket<TcpStream>, value: &Value) {
    socket
        .send(Message::Text(value.to_string().into()))
        .unwrap_or_else(|error| panic!("send WebSocket: {error:?}"));
}
fn notify(socket: &mut WebSocket<TcpStream>, log: &Value) {
    send(
        socket,
        &json!({"jsonrpc":"2.0", "method":"eth_subscription", "params":{"subscription":"0xlogs", "result":log}}),
    );
}
fn accept(listener: &TcpListener) -> WebSocket<TcpStream> {
    let (stream, _) = listener
        .accept()
        .unwrap_or_else(|error| panic!("accept WebSocket: {error:?}"));
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(|error| panic!("read budget: {error:?}"));
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(|error| panic!("write budget: {error:?}"));
    let mut socket = tungstenite::accept_hdr(
        stream,
        |request: &tungstenite::handshake::server::Request, response| {
            assert_eq!(request.uri().path(), "/node");
            Ok(response)
        },
    )
    .unwrap_or_else(|error| panic!("real WebSocket handshake: {error:?}"));
    let request = receive(&mut socket);
    assert_eq!(
        request,
        json!({"jsonrpc":"2.0", "id":"1", "method":"eth_subscribe", "params":["logs", {}]})
    );
    send(
        &mut socket,
        &json!({"jsonrpc":"2.0", "id":"1", "result":"0xlogs"}),
    );
    socket
}

fn producer_logs() -> Vec<Value> {
    let vectors = sequencer_vectors();
    let mut logs = Vec::new();
    for case in vectors["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("cases"))
        .iter()
        .take(5)
    {
        let view = &case["receiptJson"];
        let verified = layerx_proof::receipt::verify_sequencer_signature(
            &bytes(
                case["signedReceipt"]
                    .as_str()
                    .unwrap_or_else(|| panic!("receipt")),
            ),
            &[authorization(case)],
        )
        .unwrap_or_else(|error| panic!("producer signature: {error:?}"));
        let served = view["logs"]
            .as_array()
            .unwrap_or_else(|| panic!("logs"))
            .iter()
            .map(|log| {
                layerx_proof::sequencer_logs::SequencerLog::new(
                    bytes(log["address"].as_str().unwrap_or_else(|| panic!("address")))
                        .try_into()
                        .unwrap_or_else(|error| panic!("address length: {error:?}")),
                    log["topics"]
                        .as_array()
                        .unwrap_or_else(|| panic!("topics"))
                        .iter()
                        .map(|topic| {
                            bytes(topic.as_str().unwrap_or_else(|| panic!("topic")))
                                .try_into()
                                .unwrap_or_else(|error| panic!("topic length: {error:?}"))
                        })
                        .collect(),
                    bytes(log["data"].as_str().unwrap_or_else(|| panic!("data"))),
                    u64::from_str_radix(
                        log["logIndex"]
                            .as_str()
                            .unwrap_or_else(|| panic!("log index"))
                            .trim_start_matches("0x"),
                        16,
                    )
                    .unwrap_or_else(|error| panic!("index: {error:?}")),
                )
            })
            .collect();
        layerx_proof::sequencer_logs::verify_logs(&verified, served)
            .unwrap_or_else(|error| panic!("real producer log commitments: {error:?}"));
        for log in view["logs"].as_array().unwrap_or_else(|| panic!("logs")) {
            let mut log = log.clone();
            log["blockNumber"] = view["height"].clone();
            log["transactionIndex"] = view["index"].clone();
            log["transactionHash"] = view["txHash"].clone();
            log["blockHash"] = json!(hex(&verified
                .protocol()
                .unwrap_or_else(|| panic!("protocol"))
                .batch_id()));
            log["removed"] = json!(false);
            logs.push(log);
        }
    }
    logs
}

#[test]
fn finding_019_disconnect_and_listener_restart_replay_by_block_transaction_and_log() {
    let logs = producer_logs();
    assert_eq!(logs.len(), 4);
    let listener =
        TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("listener: {error:?}"));
    let address = listener
        .local_addr()
        .unwrap_or_else(|error| panic!("address: {error:?}"));
    let endpoint = format!("http://{address}/node");
    let first = logs[..2].to_vec();
    let (shutdown, stopped) = std::sync::mpsc::channel();
    let server = initial_log_listener(listener, first, stopped);
    let rpc = client(&endpoint);
    let mut subscription = rpc
        .subscribe(SubscriptionTopic::Logs(LogFilter::default()), None)
        .unwrap_or_else(|error| panic!("subscribe: {error:?}"));
    assert_eq!(subscription.cursor(), None);
    assert_eq!(
        subscription
            .next_event()
            .unwrap_or_else(|error| panic!("first event: {error:?}")),
        Some(logs[0].clone())
    );
    assert_eq!(
        subscription
            .next_event()
            .unwrap_or_else(|error| panic!("second event: {error:?}")),
        Some(logs[1].clone())
    );
    let cursor = subscription
        .cursor()
        .unwrap_or_else(|| panic!("resume position"));
    assert_eq!(
        cursor,
        LogPosition {
            block_number: 1001,
            transaction_index: 1,
            log_index: 0
        }
    );
    subscription
        .close()
        .unwrap_or_else(|error| panic!("disconnect subscription: {error:?}"));
    assert!(matches!(
        subscription.next_event(),
        Err(RpcError::Transport)
    ));
    assert_eq!(subscription.cursor(), Some(cursor));
    drop(subscription);
    drop(rpc);
    shutdown
        .send(cursor)
        .unwrap_or_else(|error| panic!("release listener with cursor: {error:?}"));
    server
        .join()
        .unwrap_or_else(|error| panic!("first listener stopped: {error:?}"));

    let listener = TcpListener::bind(address)
        .unwrap_or_else(|error| panic!("restart listener at same address: {error:?}"));
    let replay = logs[1..3].to_vec();
    let overlap = logs[2].clone();
    let live = logs[3].clone();
    let server = replay_log_listener(listener, replay, overlap, live);
    let rpc = client(&endpoint);
    let mut resumed = rpc
        .subscribe_from(SubscriptionTopic::Logs(LogFilter::default()), None, cursor)
        .unwrap_or_else(|error| panic!("resume on restarted listener: {error:?}"));
    assert_eq!(resumed.cursor(), Some(cursor));
    assert_eq!(
        resumed
            .next_event()
            .unwrap_or_else(|error| panic!("same block next transaction: {error:?}")),
        Some(logs[2].clone())
    );
    assert_eq!(
        resumed.cursor(),
        Some(LogPosition {
            block_number: 1001,
            transaction_index: 2,
            log_index: 0
        })
    );
    assert_eq!(
        resumed
            .next_event()
            .unwrap_or_else(|error| panic!("next block: {error:?}")),
        Some(logs[3].clone())
    );
    let last_delivered = resumed.cursor();
    resumed
        .unsubscribe()
        .unwrap_or_else(|error| panic!("eth_unsubscribe acknowledgement: {error:?}"));
    assert_eq!(resumed.cursor(), last_delivered);
    server
        .join()
        .unwrap_or_else(|error| panic!("restarted listener: {error:?}"));
}

#[test]
fn finding_019_malformed_removed_and_regressing_logs_never_advance_the_cursor() {
    let logs = producer_logs();
    for mutation in [
        "blockNumber",
        "transactionIndex",
        "logIndex",
        "removed",
        "address",
        "topics",
        "data",
        "transactionHash",
        "blockHash",
        "regression",
    ] {
        let listener =
            TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("listener: {error:?}"));
        let endpoint = format!(
            "http://{}/node",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("address: {error:?}"))
        );
        let first = logs[1].clone();
        let mut invalid = logs[2].clone();
        match mutation {
            "removed" => invalid[mutation] = json!(true),
            "topics" => invalid[mutation] = json!(["0x00"]),
            "data" => invalid[mutation] = json!("0x0"),
            "regression" => invalid = logs[0].clone(),
            _ => invalid[mutation] = json!("0x00"),
        }
        let server = thread::spawn(move || {
            let mut socket = accept(&listener);
            notify(&mut socket, &first);
            notify(&mut socket, &invalid);
        });
        let rpc = client(&endpoint);
        let mut subscription = rpc
            .subscribe(SubscriptionTopic::Logs(LogFilter::default()), None)
            .unwrap_or_else(|error| panic!("subscribe: {error:?}"));
        assert_eq!(
            subscription
                .next_event()
                .unwrap_or_else(|error| panic!("first: {error:?}")),
            Some(logs[1].clone())
        );
        let cursor = subscription.cursor();
        assert!(
            matches!(subscription.next_event(), Err(RpcError::InvalidResponse)),
            "{mutation}"
        );
        assert_eq!(subscription.cursor(), cursor, "{mutation}");
        server
            .join()
            .unwrap_or_else(|error| panic!("listener: {error:?}"));
    }
}

#[test]
fn finding_019_unsubscribe_does_not_acknowledge_undelivered_logs() {
    let logs = producer_logs();
    let listener =
        TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("listener: {error:?}"));
    let endpoint = format!(
        "http://{}/node",
        listener
            .local_addr()
            .unwrap_or_else(|error| panic!("address: {error:?}"))
    );
    let first = logs[0].clone();
    let second = logs[1].clone();
    let server = thread::spawn(move || {
        let mut socket = accept(&listener);
        notify(&mut socket, &first);
        assert_eq!(receive(&mut socket)["method"], "eth_unsubscribe");
        notify(&mut socket, &second);
        send(
            &mut socket,
            &json!({"jsonrpc":"2.0", "id":"2", "result":true}),
        );
    });
    let rpc = client(&endpoint);
    let mut subscription = rpc
        .subscribe(SubscriptionTopic::Logs(LogFilter::default()), None)
        .unwrap_or_else(|error| panic!("subscribe: {error:?}"));
    assert_eq!(
        subscription
            .next_event()
            .unwrap_or_else(|error| panic!("first log: {error:?}")),
        Some(logs[0].clone())
    );
    let delivered = subscription.cursor();
    subscription
        .unsubscribe()
        .unwrap_or_else(|error| panic!("unsubscribe: {error:?}"));
    assert_eq!(subscription.cursor(), delivered);
    server
        .join()
        .unwrap_or_else(|error| panic!("listener: {error:?}"));
}

fn encode_lifecycle_case(name: &str, payload: &[u8]) -> Vec<u8> {
    match name {
        "deploy" => deploy_calldata(
            NativeProgramDeploy::decode(payload)
                .unwrap_or_else(|error| panic!("deploy: {error:?}")),
        ),
        "upgrade" => upgrade_calldata(
            NativeProgramUpgrade::decode(payload)
                .unwrap_or_else(|error| panic!("upgrade: {error:?}")),
            1000,
        ),
        _ => wind_down_calldata(
            NativeProgramWindDown::decode(payload)
                .unwrap_or_else(|error| panic!("wind-down: {error:?}")),
        ),
    }
    .unwrap_or_else(|error| panic!("ABI encoding: {error:?}"))
}

fn replay_log_listener(
    listener: TcpListener,
    replay: Vec<Value>,
    overlap: Value,
    live: Value,
) -> JoinHandle<()> {
    thread::spawn(move || {
        let mut socket = accept(&listener);
        assert_eq!(
            receive(&mut socket),
            json!({"jsonrpc":"2.0", "id":"3", "method":"eth_blockNumber", "params":[]})
        );
        notify(&mut socket, &overlap);
        send(
            &mut socket,
            &json!({"jsonrpc":"2.0", "id":"3", "result":"0x3e9"}),
        );
        assert_eq!(
            receive(&mut socket),
            json!({"jsonrpc":"2.0", "id":"4", "method":"eth_getLogs", "params":[{"fromBlock":"0x3e9", "toBlock":"0x3e9"}]})
        );
        send(
            &mut socket,
            &json!({"jsonrpc":"2.0", "id":"4", "result":replay}),
        );
        notify(&mut socket, &live);
        assert_eq!(
            receive(&mut socket),
            json!({"jsonrpc":"2.0", "id":"2", "method":"eth_unsubscribe", "params":["0xlogs"]})
        );
        notify(&mut socket, &live);
        send(
            &mut socket,
            &json!({"jsonrpc":"2.0", "id":"2", "result":true}),
        );
    })
}

fn initial_log_listener(
    listener: TcpListener,
    first: Vec<Value>,
    stopped: std::sync::mpsc::Receiver<LogPosition>,
) -> JoinHandle<()> {
    thread::spawn(move || {
        let mut socket = accept(&listener);
        for log in &first {
            notify(&mut socket, log);
        }
        let cursor = stopped
            .recv_timeout(Duration::from_secs(5))
            .unwrap_or_else(|error| panic!("client shutdown with resume position: {error:?}"));
        assert_eq!(
            cursor,
            LogPosition {
                block_number: 1001,
                transaction_index: 1,
                log_index: 0
            }
        );
    })
}
