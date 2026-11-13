//! Task 22.33.1496 / audit 22.30.89. Finding -> executable proof:
//! `test_retired_endpoint_program_calls` -> `test_retired_endpoint_program_calls`
//! `program_capture_path_gap` -> `program_capture_path_gap`
//! `test_retired_endpoint_program_lifecycle` -> `test_retired_endpoint_program_lifecycle`
//! `program_lifecycle_third_shape` -> `program_lifecycle_third_shape` proves calldata,
//! receipt authentication and ordinary-call refusal; lifecycle execution and
//! resultant state still require the producer transcripts requested below.
//! `test_retired_endpoint_program_registry` -> `test_retired_endpoint_program_registry`
//! `proof_rpc_expected_failure` -> `proof_rpc_expected_failure`
//! `test_retired_endpoint_rpc_identity` -> `test_retired_endpoint_rpc_identity`
//! `test_retired_endpoint_wallet_commands` -> `test_retired_endpoint_wallet_commands`
//!
//! Baseline: no gate was run before implementation, as required by the explicit
//! gate order; the new test target did not exist on the supplied base.
//! These assertions do not declare the findings closed before qualification.
//! Attempt 1: 42 passed / 9 failed; the recorded failures were root admission,
//! POST /rpc route absence and prefixed decoding of the unprefixed bank vector.
//! Attempt 2 uses the landed root transports and library module types; bank ABI
//! bytes remain unprefixed while node bytes and numeric height pins stay intact.
//! Gate evidence: hf-build/hf-done retain the verified revision, command, exit
//! and private log location; no unexecuted baseline or gate is claimed.
//! Wiring request (node/kernel, domains 1/2): publish a committed deploy,
//! upgrade, wind-down, pending and reverted-program execution transcript with
//! state reads; the existing native-program-v4 vectors explicitly claim encoding
//! only, and are not evidence of deployed execution. No lifecycle result is
//! invented here to fill that missing producer interface.
//! Wiring request (node/kernel, domains 1/2): `rpc/sequencer_receipt_vectors_test.go`
//! must publish a Programs revert as well as the existing Ledger revert; the
//! latter is verified as a genuine revert and refused as lifecycle evidence.

#![allow(dead_code)]

use layerx_platform_cli::{encoding, http};
#[path = "../src/programs.rs"]
mod programs;

use layerx_client::sequencer::{SequencerEndpoint, SequencerReceipt};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_platform_cli::{rpc::RpcClient, wallet_gateway, wallet_reads};
use serde_json::{json, Value};
use std::io::{BufRead, Read, Write};
use std::net::TcpListener;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::time::Duration;
use zeroize::Zeroizing;

const CORPUS: &str =
    include_str!("../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
const PROGRAM: &str = "V1-layerxprograms-call";
const REVERT: &str = "V2-layerxledger-revert";

type Proof = Result<(), String>;

fn bytes(value: &Value) -> Vec<u8> {
    encoding::hex_decode(
        "producer bytes",
        value
            .as_str()
            .unwrap_or_else(|| panic!("{}", "hex string"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("{}", "hex prefix")),
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "producer hex"))
}

fn case(id: &str) -> Value {
    let corpus: Value = serde_json::from_str(CORPUS)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "node producer corpus"));
    assert_eq!(
        corpus["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    corpus["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("{}", "producer cases"))
        .iter()
        .find(|case| case["id"] == id)
        .unwrap_or_else(|| panic!("{}", "recorded node case"))
        .clone()
}

fn authentic(case: &Value) {
    use layerx_proof::{inclusion::SequencerAuthorization, receipt::verify_sequencer_signature};
    let raw = bytes(&case["rawTransaction"]);
    assert_eq!(
        case["receiptJson"]["txHash"],
        format!("0x{}", encoding::hex_encode(&transaction_hash(&raw)))
    );
    assert_eq!(case["receiptJson"]["receipt"], case["signedReceipt"]);
    let pin = &case["pinnedKey"];
    let pins = [SequencerAuthorization::new(
        bytes(&pin["keyId"])
            .try_into()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "key id")),
        bytes(&pin["publicKey"])
            .try_into()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "public key")),
        pin["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("{}", "first height")),
        pin["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("{}", "last height")),
    )];
    let receipt = bytes(&case["signedReceipt"]);
    let verified = verify_sequencer_signature(&receipt, &pins).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "independently pinned authentic node receipt"
        )
    });
    assert_eq!(
        verified
            .protocol()
            .unwrap_or_else(|| panic!("{}", "protocol receipt"))
            .activity_id(),
        transaction_hash(&raw)
    );
    let mut damaged = receipt;
    *damaged
        .last_mut()
        .unwrap_or_else(|| panic!("{}", "signature byte")) ^= 1;
    assert!(verify_sequencer_signature(&damaged, &pins).is_err());
}

#[derive(Clone, Debug)]
struct Exchange {
    method: &'static str,
    params: Value,
    result: Value,
}

#[derive(Clone, Debug)]
struct Observed {
    path: String,
    authorization: Option<String>,
    request: Value,
}

/// A bounded HTTP replay peer, not a replacement node implementation. Every
/// successful result comes from the committed producer or a production gateway
/// codec. Unrecorded RPC operations are errors, never canned success replies.
struct Transcript {
    endpoint: String,
    observed: Arc<Mutex<Vec<Observed>>>,
    stop: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<()>>,
}

impl Transcript {
    fn start(path: &'static str, authorization: Option<String>, exchanges: Vec<Exchange>) -> Self {
        Self::start_with_capture(path, authorization, exchanges, None)
    }

    fn capture(authorization: String, method: &'static str, params: Value) -> Self {
        Self::start_with_capture("/", Some(authorization), vec![], Some((method, params)))
    }

    fn start_with_capture(
        path: &'static str,
        authorization: Option<String>,
        exchanges: Vec<Exchange>,
        capture: Option<(&'static str, Value)>,
    ) -> Self {
        let exchanges: std::collections::HashMap<_, _> = exchanges
            .into_iter()
            .map(|exchange| {
                (
                    (exchange.method.to_owned(), exchange.params.to_string()),
                    exchange.result,
                )
            })
            .collect();
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("{}: {error:?}", "loopback peer"));
        let endpoint = format!(
            "http://{}",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "peer address"))
        );
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "nonblocking peer"));
        let observed = Arc::new(Mutex::new(Vec::new()));
        let seen = observed.clone();
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (socket, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("peer accept: {error}"),
                };
                let (mut reader, request_path, bearer, request) = read_transcript_request(socket);
                seen.lock()
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "observation lock"))
                    .push(Observed {
                        path: request_path.clone(),
                        authorization: bearer.clone(),
                        request: request.clone(),
                    });
                let (status, body) = if request_path != path {
                    (
                        404,
                        serde_json::to_vec(&json!({"error":"route absent"}))
                            .unwrap_or_else(|error| panic!("{}: {error:?}", "route refusal")),
                    )
                } else if bearer != authorization {
                    (
                        401,
                        serde_json::to_vec(&json!({"error":"authentication required"}))
                            .unwrap_or_else(|error| {
                                panic!("{}: {error:?}", "authentication refusal")
                            }),
                    )
                } else if capture.as_ref().is_some_and(|(method, params)| {
                    request["method"] == *method && request["params"] == *params
                }) {
                    reader
                        .get_ref()
                        .shutdown(std::net::Shutdown::Both)
                        .unwrap_or_else(|error| panic!("{}: {error:?}", "drop captured reply"));
                    continue;
                } else if let Some(result) = exchanges.get(&(
                    request["method"]
                        .as_str()
                        .unwrap_or_else(|| panic!("{}", "RPC method"))
                        .to_owned(),
                    request["params"].to_string(),
                )) {
                    (
                        200,
                        serde_json::to_vec(
                            &json!({"jsonrpc":"2.0", "id":request["id"], "result":result}),
                        )
                        .unwrap_or_else(|error| panic!("{}: {error:?}", "recorded reply")),
                    )
                } else {
                    panic!("unrecorded producer operation: {request}");
                };
                let socket = reader.get_mut();
                write!(socket, "HTTP/1.1 {status} Reply\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap_or_else(|error| panic!("{}: {error:?}", "response header"));
                socket
                    .write_all(&body)
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "response body"));
            }
        });
        Self {
            endpoint,
            observed,
            stop,
            worker: Some(worker),
        }
    }

    fn receipt(case: &Value) -> Self {
        Self::start(
            "/",
            None,
            vec![Exchange {
                method: "pax_sequencerReceipt",
                params: json!([case["receiptJson"]["txHash"]]),
                result: case["receiptJson"].clone(),
            }],
        )
    }

    fn observations(&self) -> Vec<Observed> {
        self.observed
            .lock()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "observations"))
            .clone()
    }
}

fn read_transcript_request(
    socket: std::net::TcpStream,
) -> (
    std::io::BufReader<std::net::TcpStream>,
    String,
    Option<String>,
    Value,
) {
    socket
        .set_read_timeout(Some(Duration::from_secs(3)))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "read deadline"));
    socket
        .set_write_timeout(Some(Duration::from_secs(3)))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "write deadline"));
    let mut reader = std::io::BufReader::new(socket);
    let mut line = String::new();
    assert!(
        reader
            .read_line(&mut line)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "request line"))
            > 0
    );
    let request_path = line
        .split_whitespace()
        .nth(1)
        .unwrap_or_else(|| panic!("{}", "request path"))
        .to_owned();
    assert!(line.starts_with("POST "));
    let mut length = None;
    let mut bearer = None;
    let mut header_bytes = line.len();
    loop {
        line.clear();
        assert!(
            reader
                .read_line(&mut line)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "header"))
                > 0
        );
        header_bytes += line.len();
        assert!(header_bytes <= 16_384);
        if line == "\r\n" {
            break;
        }
        let (key, value) = line
            .split_once(':')
            .unwrap_or_else(|| panic!("{}", "header delimiter"));
        if key.eq_ignore_ascii_case("content-length") {
            length = Some(
                value
                    .trim()
                    .parse::<usize>()
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "content length")),
            );
        }
        if key.eq_ignore_ascii_case("authorization") {
            bearer = Some(value.trim().to_owned());
        }
    }
    let length = length.unwrap_or_else(|| panic!("{}", "bounded body length"));
    assert!(length <= 4 * 1024 * 1024);
    let mut body = vec![0; length];
    reader
        .read_exact(&mut body)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "request body"));
    let request: Value = serde_json::from_slice(&body)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "RPC request"));
    assert_eq!(request["jsonrpc"], "2.0");
    (reader, request_path, bearer, request)
}

impl Drop for Transcript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .unwrap_or_else(|| panic!("{}", "peer worker"))
            .join()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "peer assertions"));
    }
}

fn switched_off(endpoint: &str) -> Proof {
    let node = RpcClient::new(endpoint, None)?;
    let gateway = RpcClient::new_gateway(&format!("{endpoint}/services/wallet"), None)?;
    for method in [
        "lx_getReceipt",
        "lx_sendActivity",
        "lx_getProgram",
        "lx_deployProgram",
        "lx_upgradeProgram",
        "lx_windDownProgram",
        "lx_listPrograms",
        "lx_getBalances",
        "lx_futureMethod",
    ] {
        for client in [&node, &gateway] {
            let refusal = client
                .call(method, &json!([]))
                .err()
                .unwrap_or_else(|| panic!("{}", "retired method must be refused before transport"));
            assert_eq!(
                refusal,
                format!("rpc_method_unavailable: {method} is switched off")
            );
        }
    }
    Ok(())
}

fn receipt_policy(recorded: &Value) -> layerx_sdk::rpc_verification::ReceiptPolicy {
    let pin = &recorded["pinnedKey"];
    layerx_sdk::rpc_verification::ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: layerx_proof::inclusion::SequencerAuthorization::new(
            bytes(&pin["keyId"])
                .try_into()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "key id")),
            bytes(&pin["publicKey"])
                .try_into()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "public key")),
            pin["firstHeight"]
                .as_u64()
                .unwrap_or_else(|| panic!("{}", "decimal first height")),
            pin["lastHeight"]
                .as_u64()
                .unwrap_or_else(|| panic!("{}", "decimal last height")),
        ),
        trusted_checkpoint_context_digest: None,
    }
}

fn authenticated_root_receipt(recorded: &Value) -> Proof {
    let credential = format!("nodekey:lxp_live_{}", "b".repeat(64));
    let peer = Transcript::start(
        "/",
        Some(format!("LayerX-Key {credential}")),
        vec![Exchange {
            method: "pax_sequencerReceipt",
            params: json!([recorded["receiptJson"]["txHash"]]),
            result: recorded["receiptJson"].clone(),
        }],
    );
    let params = json!([recorded["receiptJson"]["txHash"]]);
    let unauthenticated = RpcClient::new(&peer.endpoint, None)?;
    let refused = unauthenticated
        .call("pax_sequencerReceipt", &params)
        .err()
        .unwrap_or_else(|| panic!("{}", "node authentication must be preserved"));
    assert!(refused.contains("401"), "{refused}");
    let client = RpcClient::new(&peer.endpoint, Some(Zeroizing::new(credential.clone())))?;
    let returned = client.call("pax_sequencerReceipt", &params)?;
    assert_eq!(returned, recorded["receiptJson"]);
    let hash = transaction_hash(&bytes(&recorded["rawTransaction"]));
    let (_, canonical) = layerx_sdk::rpc_verification::verify_node_receipt(
        &returned,
        hash,
        &receipt_policy(recorded),
    )
    .map_err(|e| format!("{e:?}"))?;
    assert_eq!(canonical, bytes(&recorded["signedReceipt"]));
    switched_off(&peer.endpoint)?;
    let seen = peer.observations();
    assert_eq!(seen.len(), 2);
    assert!(seen.iter().all(|seen| seen.path == "/"));
    assert!(seen[0].authorization.is_none());
    assert_eq!(
        seen[1].authorization,
        Some(format!("LayerX-Key {credential}"))
    );
    assert!(RpcClient::new(&format!("{}/rpc", peer.endpoint), None).is_err());
    Ok(())
}

fn root_receipt() -> Proof {
    let recorded = case(PROGRAM);
    authentic(&recorded);
    let peer = Transcript::receipt(&recorded);
    let hash = transaction_hash(&bytes(&recorded["rawTransaction"]));
    // Establish that the same root peer is consumable by the production node client.
    let node = SequencerEndpoint::new(&peer.endpoint, &[]).map_err(|e| format!("{e:?}"))?;
    let receipt = node
        .sequencer_receipt(&hash)
        .map_err(|e| format!("{e:?}"))?;
    let Some(SequencerReceipt::Receipted(receipt)) = receipt else {
        panic!("authentic receipted program call")
    };
    assert!(receipt.success);
    assert_eq!(receipt.receipt, bytes(&recorded["signedReceipt"]));
    switched_off(&peer.endpoint)?;
    assert_eq!(
        peer.observations().len(),
        1,
        "lx_ must never reach the peer"
    );
    let cli = crate::http::Client::new(&peer.endpoint, None)?;
    let returned = programs::sequencer_receipt(
        &cli,
        recorded["receiptJson"]["txHash"]
            .as_str()
            .unwrap_or_else(|| panic!("{}", "hash")),
    )?;
    assert_eq!(
        returned, recorded["receiptJson"],
        "CLI must preserve authentic receipt output"
    );
    assert!(peer
        .observations()
        .iter()
        .all(|seen| seen.path == "/" && seen.authorization.is_none()));
    authenticated_root_receipt(&recorded)?;
    hosted_wallet()
}

/// # Errors
/// Returns an error if the CLI or receipt response violates its contract.
///
/// # Panics
/// Panics if a required fixture assertion is false.
#[test]
pub fn test_retired_endpoint_program_calls() -> Proof {
    root_receipt()
}

/// # Errors
/// Returns an error if the CLI or receipt response violates its contract.
///
/// # Panics
/// Panics if a required fixture assertion is false.
#[test]
pub fn program_capture_path_gap() -> Proof {
    let recorded = case(PROGRAM);
    authentic(&recorded);
    let raw = bytes(&recorded["rawTransaction"]);
    let (transaction, signature) = decode_signed(&raw).map_err(|e| e.to_string())?;
    let from = recover_sender(&transaction, &signature).map_err(|e| e.to_string())?;
    assert_eq!(
        format!("0x{}", encoding::hex_encode(&from)),
        recorded["receiptJson"]["from"]
    );
    // Re-sign exactly the Go producer's transaction through the CLI signing path.
    let secret = encoding::fixed_hex(
        "committed fixture secret",
        "8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63",
    )?;
    assert_eq!(
        programs::sign_program_transaction(&transaction, &secret, &from)?,
        raw
    );
    assert_eq!(&transaction.data[..4], &[0x54, 0xbd, 0x2b, 0x48]);
    let mut simulated = transaction.data.clone();
    simulated[..4].copy_from_slice(&[0x30, 0x44, 0xbd, 0x56]);
    let return_data = recorded["receiptJson"]["returnData"].clone();
    let expected = programs::simulated_program_result(&return_data)?;
    assert_eq!(expected["outcome"]["terminal_result"], 0);
    let peer = Transcript::start(
        "/",
        None,
        vec![
            Exchange {
                method: "eth_call",
                params: json!([{"to":recorded["receiptJson"]["to"],"data":format!("0x{}", encoding::hex_encode(&simulated))}, "pending"]),
                result: return_data,
            },
            Exchange {
                method: "eth_sendRawTransaction",
                params: json!([recorded["rawTransaction"]]),
                result: recorded["receiptJson"]["txHash"].clone(),
            },
            Exchange {
                method: "pax_sequencerReceipt",
                params: json!([recorded["receiptJson"]["txHash"]]),
                result: recorded["receiptJson"].clone(),
            },
        ],
    );
    let client = crate::http::Client::new(&peer.endpoint, None)?;
    let answer = programs::rpc_call(
        &client,
        "eth_call",
        &json!([{"to":recorded["receiptJson"]["to"],"data":format!("0x{}", encoding::hex_encode(&simulated))},"pending"]),
    )?;
    assert_eq!(programs::simulated_program_result(&answer)?, expected);
    let submitted = programs::submit_program_transaction(&client, &raw)?;
    assert_eq!(submitted["tx_hash"], recorded["receiptJson"]["txHash"]);
    assert_eq!(submitted["rung"], "submitted");
    assert_eq!(
        programs::sequencer_receipt(
            &client,
            submitted["tx_hash"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "hash"))
        )?,
        recorded["receiptJson"]
    );
    assert_eq!(peer.observations().len(), 3);
    assert!(peer.observations().iter().all(|seen| seen.path == "/"));
    let reverted = case(REVERT);
    authentic(&reverted);
    let revert_peer = Transcript::receipt(&reverted);
    let revert_client = crate::http::Client::new(&revert_peer.endpoint, None)?;
    let hash = reverted["receiptJson"]["txHash"]
        .as_str()
        .unwrap_or_else(|| panic!("{}", "reverted hash"));
    let result = programs::sequencer_receipt(&revert_client, hash)?;
    assert_eq!(result, reverted["receiptJson"]);
    assert_eq!(result["status"], "0x0");
    assert_eq!(result["vmError"], "execution reverted");
    assert!(result["returnData"]
        .as_str()
        .unwrap_or_else(|| panic!("{}", "Error(string)"))
        .starts_with("0x08c379a0"));
    assert!(
        programs::simulated_program_result(&result["returnData"]).is_err(),
        "an EVM revert is not a successful Programs ABI result"
    );
    Ok(())
}

/// # Errors
/// Returns an error if the CLI or receipt response violates its contract.
///
/// # Panics
/// Panics if a required fixture assertion is false.
#[test]
pub fn test_retired_endpoint_program_lifecycle() -> Proof {
    use layerx_crypto::evm_transaction::Eip1559Call;

    let account = layerx_crypto::account_derivation::derive_from_mnemonic(
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
        "",
        0,
    ).map_err(|error| error.to_string())?;
    let credential = format!("lifecycle:lxp_live_{}", "c".repeat(64));
    let invalid = format!("lifecycle:lxp_live_{}", "d".repeat(64));
    for (ordinal, fee, source) in [
        (
            1,
            "0",
            include_str!(
                "../../../../sdk/platform/conformance/fixtures/native-program-deploy-v4.json"
            ),
        ),
        (
            2,
            "1000",
            include_str!(
                "../../../../sdk/platform/conformance/fixtures/native-program-upgrade-v4.json"
            ),
        ),
        (
            7,
            "0",
            include_str!(
                "../../../../sdk/platform/conformance/fixtures/native-program-wind-down-tombstone-v4.json"
            ),
        ),
    ] {
        let vector: Value = serde_json::from_str(source).map_err(|error| error.to_string())?;
        let payload = encoding::hex_decode(
            "lifecycle payload",
            vector["payload_hex"].as_str().unwrap_or_else(|| panic!("{}", "producer payload")),
        )?;
        let data = programs::lifecycle_calldata(ordinal, &payload, fee)?;
        assert_eq!(
            format!("0x{}", encoding::hex_encode(&data)),
            vector["evm"]["calldata"]
        );
        let transaction = Eip1559Call {
            chain_id: vector["evm"]["chainId"].as_u64().unwrap_or_else(|| panic!("{}", "producer chain")),
            nonce: 7,
            max_priority_fee_per_gas: 2,
            max_fee_per_gas: 200,
            gas_limit: 1_000_000,
            to: encoding::fixed_hex(
                "Programs address",
                "000000000000000000000000000000000000101d",
            )?,
            data,
        };
        let raw = programs::sign_program_transaction(
            &transaction,
            account.evm_secret().unwrap_or_else(|| panic!("{}", "derived EVM secret")),
            &account.evm_address(),
        )?;
        let params = json!([format!("0x{}", encoding::hex_encode(&raw))]);
        let hash = format!("0x{}", encoding::hex_encode(&transaction_hash(&raw)));
        let peer = Transcript::capture(
            format!("LayerX-Key {credential}"),
            "eth_sendRawTransaction",
            params.clone(),
        );
        let mut expected_headers = Vec::new();
        for key in [None, Some(invalid.clone()), Some(credential.clone())] {
            let client = match key.clone() {
                Some(key) => http::Client::new_gateway(&peer.endpoint, Zeroizing::new(key))?,
                None => http::Client::new(&peer.endpoint, None)?,
            };
            let outcome = programs::submit_lifecycle_transaction(&client, &raw)?;
            assert_eq!(outcome["txHash"], hash);
            assert_eq!(outcome["tx_hash"], hash);
            assert_eq!(outcome["rung"], "unknown");
            let detail = outcome["detail"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "recoverable submission detail"));
            if key.as_deref() == Some(credential.as_str()) {
                assert!(detail.contains("POST / failed:"), "{detail}");
                assert!(!detail.contains("401"), "authenticated capture: {detail}");
            } else {
                assert!(detail.contains("401"), "authentication refusal: {detail}");
            }
            assert!(outcome.get("receipt").is_none());
            expected_headers.push(key.map(|key| format!("LayerX-Key {key}")));
        }
        assert_lifecycle_capture(&peer, &params, &transaction, account.evm_address(), expected_headers)?;
    }
    hosted_wallet()
}

fn assert_lifecycle_capture(
    peer: &Transcript,
    params: &Value,
    transaction: &layerx_crypto::evm_transaction::Eip1559Call,
    signer: [u8; 20],
    expected_headers: Vec<Option<String>>,
) -> Proof {
    let count = peer.observations().len();
    switched_off(&peer.endpoint)?;
    let seen = peer.observations();
    assert_eq!(count, 3, "each lifecycle invocation sends exactly once");
    assert_eq!(seen.len(), count, "lx_ refuses before lifecycle transport");
    for (seen, authorization) in seen.iter().zip(expected_headers) {
        assert_eq!(seen.path, "/");
        assert_eq!(seen.authorization, authorization);
        assert_eq!(
            seen.request,
            json!({"jsonrpc":"2.0", "id":1, "method":"eth_sendRawTransaction", "params":params})
        );
        let (sent, signature) =
            decode_signed(&bytes(&seen.request["params"][0])).map_err(|error| error.to_string())?;
        assert_eq!(&sent, transaction);
        assert_eq!(
            recover_sender(&sent, &signature).map_err(|error| error.to_string())?,
            signer
        );
    }
    Ok(())
}

fn assert_revert_lifecycle(
    hash: &str,
    receipt: &Value,
    policy: &layerx_sdk::rpc_verification::ReceiptPolicy,
) {
    assert_eq!(receipt["status"], "0x0");
    assert_eq!(receipt["vmError"], "execution reverted");
    assert!(receipt["returnData"]
        .as_str()
        .unwrap_or_else(|| panic!("{}", "revert data"))
        .starts_with("0x08c379a0"));
    let refusal = programs::lifecycle_receipt_result_verified(hash, receipt, policy)
        .err()
        .unwrap_or_else(|| {
            panic!(
                "{}",
                "a Ledger revert cannot authenticate a Programs lifecycle"
            )
        });
    assert!(
        refusal.contains("outside the Programs execution scope"),
        "{refusal}"
    );
}

/// # Errors
/// Returns an error if the CLI or receipt response violates its contract.
///
/// # Panics
/// Panics if a required fixture assertion is false.
#[test]
pub fn program_lifecycle_third_shape() -> Proof {
    let lifecycle_topics = validated_lifecycle_topics()?;
    // These are authentic execution receipts, not the old txHash/rung-only objects.
    for id in [PROGRAM, REVERT] {
        let recorded = case(id);
        authentic(&recorded);
        let peer = Transcript::receipt(&recorded);
        let client = crate::http::Client::new(&peer.endpoint, None)?;
        let hash = recorded["receiptJson"]["txHash"]
            .as_str()
            .unwrap_or_else(|| panic!("{}", "hash"));
        let receipt = programs::sequencer_receipt(&client, hash)?;
        assert_eq!(receipt, recorded["receiptJson"]);
        let policy = receipt_policy(&recorded);
        let (verified, canonical) = layerx_sdk::rpc_verification::verify_node_receipt(
            &receipt,
            transaction_hash(&bytes(&recorded["rawTransaction"])),
            &policy,
        )
        .map_err(|e| format!("{e:?}"))?;
        assert_eq!(canonical, bytes(&recorded["signedReceipt"]));
        assert_eq!(
            verified
                .protocol()
                .unwrap_or_else(|| panic!("{}", "execution facts"))
                .result_code(),
            i32::from(id == REVERT)
        );
        if id == REVERT {
            assert_revert_lifecycle(hash, &receipt, &policy);
        } else {
            let raw = bytes(&recorded["rawTransaction"]);
            let (call, signature) = decode_signed(&raw).map_err(|e| e.to_string())?;
            assert_eq!(call.chain_id, 125);
            assert_eq!(
                format!("0x{}", encoding::hex_encode(&call.to)),
                receipt["to"]
            );
            assert_eq!(
                format!(
                    "0x{}",
                    encoding::hex_encode(
                        &recover_sender(&call, &signature).map_err(|e| e.to_string())?
                    )
                ),
                receipt["from"]
            );
            assert_eq!(
                &call.data[..4],
                &[0x54, 0xbd, 0x2b, 0x48],
                "Programs call selector"
            );
            assert!(programs::lifecycle_event_topic(&call.data).is_err());
            let logs = receipt["logs"]
                .as_array()
                .unwrap_or_else(|| panic!("{}", "authenticated logs"));
            assert_eq!(logs.len(), 1);
            assert_eq!(
                logs[0]["address"],
                "0x000000000000000000000000000000000000101d"
            );
            assert_eq!(
                logs[0]["topics"],
                json!([
                    "0xcfe5297f03a2951065e463e9e0c2e3686bc0204f727eeec1a52357b5dd4a27a0",
                    format!("0x{}", encoding::hex_encode(&call.data[4..36])),
                ]),
                "ProgramCallOutcome for the called program"
            );
            let protocol = verified
                .protocol()
                .unwrap_or_else(|| panic!("{}", "authenticated Programs execution"));
            assert_eq!(protocol.module_id(), 12);
            assert_eq!(protocol.operation(), 2);
            assert_eq!(receipt["status"], "0x1");
            assert_eq!(receipt["vmError"], "");
            let refusal = programs::lifecycle_receipt_result_verified(hash, &receipt, &policy)
                .err()
                .unwrap_or_else(|| {
                    panic!(
                        "{}",
                        "an ordinary Programs call cannot prove lifecycle completion"
                    )
                });
            assert!(
                refusal.contains("omitted an authenticated program lifecycle event"),
                "{refusal}"
            );
            assert_pending_and_tampered_lifecycle(
                hash,
                &receipt,
                &policy,
                canonical,
                &lifecycle_topics,
            )?;
        }
    }
    Ok(())
}

fn validated_lifecycle_topics() -> Result<Vec<&'static str>, String> {
    let mut lifecycle_topics = Vec::new();
    for (ordinal, fee, source) in [
        (
            1,
            "0",
            include_str!(
                "../../../../sdk/platform/conformance/fixtures/native-program-deploy-v4.json"
            ),
        ),
        (
            2,
            "1000",
            include_str!(
                "../../../../sdk/platform/conformance/fixtures/native-program-upgrade-v4.json"
            ),
        ),
        (
            7,
            "0",
            include_str!(
                "../../../../sdk/platform/conformance/fixtures/native-program-wind-down-route-v4.json"
            ),
        ),
        (
            7,
            "0",
            include_str!(
                "../../../../sdk/platform/conformance/fixtures/native-program-wind-down-deprecate-v4.json"
            ),
        ),
        (
            7,
            "0",
            include_str!(
                "../../../../sdk/platform/conformance/fixtures/native-program-wind-down-tombstone-v4.json"
            ),
        ),
        (
            7,
            "0",
            include_str!(
                "../../../../sdk/platform/conformance/fixtures/native-program-wind-down-exit-v4.json"
            ),
        ),
    ] {
        let vector: Value = serde_json::from_str(source).map_err(|e| e.to_string())?;
        let payload = encoding::hex_decode(
            "lifecycle payload",
            vector["payload_hex"].as_str().unwrap_or_else(|| panic!("{}", "unprefixed payload")),
        )?;
        let calldata = programs::lifecycle_calldata(ordinal, &payload, fee)?;
        lifecycle_topics.push(programs::lifecycle_event_topic(&calldata)?);
        assert_eq!(
            format!("0x{}", encoding::hex_encode(&calldata)),
            vector["evm"]["calldata"]
        );
    }
    Ok(lifecycle_topics)
}

fn assert_pending_and_tampered_lifecycle(
    hash: &str,
    receipt: &Value,
    policy: &layerx_sdk::rpc_verification::ReceiptPolicy,
    canonical: Vec<u8>,
    lifecycle_topics: &[&str],
) -> Proof {
    let pending = programs::lifecycle_receipt_result_verified(hash, &Value::Null, policy)?;
    assert_eq!(
        pending,
        json!({"txHash":hash,"rung":"submitted","state":"pending"})
    );
    let submitted = json!({"txHash":hash,"rung":"submitted"});
    assert_eq!(
        programs::lifecycle_receipt_result_verified(hash, &submitted, policy)?,
        pending
    );
    let submitted = json!({"txHash":hash,"rung":"submitted","nonce":receipt["nonce"]});
    assert_eq!(
        programs::lifecycle_receipt_result_verified(hash, &submitted, policy)?,
        json!({"txHash":hash,"rung":"submitted","state":"pending","nonce":receipt["nonce"]})
    );
    assert!(programs::lifecycle_receipt_result_verified(
        hash,
        &json!({"txHash":hash,"rung":"submitted","nonce":"0x00"}),
        policy,
    )
    .is_err());
    for rung in ["receipted", "committed"] {
        assert!(
            programs::lifecycle_receipt_result_verified(
                hash,
                &json!({"txHash":hash,"rung":rung}),
                policy,
            )
            .is_err(),
            "terminal labels are not authenticated execution"
        );
    }
    let mut damaged = canonical;
    *damaged
        .last_mut()
        .unwrap_or_else(|| panic!("{}", "signature byte")) ^= 1;
    for (field, replacement) in [
        ("status", json!("0x0")),
        ("txHash", json!(format!("0x{}", "00".repeat(32)))),
        (
            "receipt",
            json!(format!("0x{}", encoding::hex_encode(&damaged))),
        ),
    ] {
        let mut changed = (*receipt).clone();
        changed[field] = replacement;
        let refusal = programs::lifecycle_receipt_result_verified(hash, &changed, policy)
            .err()
            .unwrap_or_else(|| panic!("{}", "tampered execution facts must fail authentication"));
        assert!(
            refusal.contains("lifecycle receipt verification failed"),
            "{refusal}"
        );
    }
    for topic in lifecycle_topics {
        let mut changed = (*receipt).clone();
        changed["logs"][0]["topics"][0] = json!(format!("0x{topic}"));
        let refusal = programs::lifecycle_receipt_result_verified(hash, &changed, policy)
            .err()
            .unwrap_or_else(|| {
                panic!(
                    "{}",
                    "relabelling a signed call log cannot authenticate a lifecycle event",
                )
            });
        assert!(
            refusal.contains("lifecycle receipt verification failed"),
            "{refusal}"
        );
    }
    Ok(())
}

/// # Errors
/// Returns an error if the CLI or receipt response violates its contract.
///
/// # Panics
/// Panics if a required fixture assertion is false.
#[test]
pub fn test_retired_endpoint_program_registry() -> Proof {
    let recorded_head: Value = serde_json::from_str(include_str!(
        "../../../../services/explorer/deploy/tools/fixtures/eth_blockNumber.json"
    ))
    .map_err(|error| error.to_string())?;
    let recorded_logs: Value = serde_json::from_str(include_str!(
        "../../../../services/explorer/deploy/tools/fixtures/eth_getLogs.json"
    ))
    .map_err(|error| error.to_string())?;
    assert_eq!(recorded_head["method"], "eth_blockNumber");
    assert_eq!(recorded_logs["method"], "eth_getLogs");
    for record in [&recorded_head, &recorded_logs] {
        assert_eq!(
            record["recorded_from"],
            "the Paxeer X Network public JSON-RPC surface"
        );
        assert_eq!(
            record["responses"]
                .as_array()
                .unwrap_or_else(|| panic!("{}", "recorded responses"))
                .len(),
            1
        );
        assert_eq!(record["responses"][0]["match"], json!([]));
    }
    let (head, logs, exchanges) = registry_exchanges(&recorded_head, &recorded_logs)?;
    let credential = format!("registry:lxp_live_{}", "e".repeat(64));
    let invalid = format!("registry:lxp_live_{}", "f".repeat(64));
    let peer = Transcript::start(
        "/",
        Some(format!("LayerX-Key {credential}")),
        exchanges.clone(),
    );
    for key in [None, Some(invalid.clone())] {
        let client = match key {
            Some(key) => http::Client::new_gateway(&peer.endpoint, Zeroizing::new(key))?,
            None => http::Client::new(&peer.endpoint, None)?,
        };
        let refusal = programs::registry_list(&client)
            .err()
            .unwrap_or_else(|| panic!("{}", "registry requires node authentication"));
        assert!(refusal.contains("401"), "{refusal}");
    }
    let client = http::Client::new_gateway(&peer.endpoint, Zeroizing::new(credential.clone()))?;
    let output = programs::registry_list(&client)?;
    assert_eq!(
        output,
        json!({
            "program_ids":[], "events":logs, "source":"eth_getLogs",
            "from_block":0, "to_block":head, "history_complete":true, "earliest_scanned":0
        })
    );
    let count = peer.observations().len();
    switched_off(&peer.endpoint)?;
    let seen = peer.observations();
    assert_eq!(
        count,
        exchanges.len() + 2,
        "registry must query the head and every bounded log window"
    );
    assert_eq!(seen.len(), count, "lx_ refuses before registry transport");
    assert!(seen.iter().all(|seen| seen.path == "/"));
    assert_eq!(seen[0].authorization, None);
    assert_eq!(seen[1].authorization, Some(format!("LayerX-Key {invalid}")));
    for seen in &seen[..2] {
        assert_eq!(
            seen.request,
            json!({"jsonrpc":"2.0", "id":1, "method":"eth_blockNumber", "params":[]})
        );
    }
    for (seen, exchange) in seen[2..].iter().zip(&exchanges) {
        assert_eq!(seen.authorization, Some(format!("LayerX-Key {credential}")));
        assert_eq!(
            seen.request,
            json!({"jsonrpc":"2.0", "id":1, "method":exchange.method, "params":exchange.params})
        );
    }
    hosted_wallet()
}

fn registry_exchanges(
    recorded_head: &Value,
    recorded_logs: &Value,
) -> Result<(u64, Value, Vec<Exchange>), String> {
    let head_result = recorded_head["responses"][0]["result"].clone();
    let head = u64::from_str_radix(
        head_result
            .as_str()
            .unwrap_or_else(|| panic!("{}", "recorded head"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("{}", "quantity prefix")),
        16,
    )
    .map_err(|error| error.to_string())?;
    let logs = recorded_logs["responses"][0]["result"].clone();
    assert_eq!(logs, json!([]));
    let mut exchanges = vec![Exchange {
        method: "eth_blockNumber",
        params: json!([]),
        result: head_result,
    }];
    let mut upper = head;
    loop {
        let lower = upper.saturating_sub(1999);
        exchanges.push(Exchange {
            method: "eth_getLogs",
            params: json!([{
                "address":"0x000000000000000000000000000000000000101d",
                "fromBlock":format!("0x{lower:x}"), "toBlock":format!("0x{upper:x}"),
                "topics":["0x2765c1020c93cab47e389edaa5b5c68834bdd76f7a5d384adb460fc86e688709"]
            }]),
            result: logs.clone(),
        });
        if lower == 0 {
            break;
        }
        upper = lower - 1;
    }
    Ok((head, logs, exchanges))
}

fn assert_service_observations(
    service: &Transcript,
    did: &str,
    invalid_key: &str,
    service_key: &str,
) {
    let seen = service.observations();
    assert_eq!(
        seen.len(),
        3,
        "retired and root-hosted methods must refuse before transport"
    );
    for (seen, authorization) in seen.iter().zip([
        None,
        Some(format!("LayerX-Key {invalid_key}")),
        Some(format!("LayerX-Key {service_key}")),
    ]) {
        assert_eq!(seen.path, "/services/wallet");
        assert_eq!(seen.authorization, authorization);
        assert_eq!(
            seen.request,
            json!({"jsonrpc":"2.0", "id":1, "method":"px_getBalances", "params":[did]})
        );
    }
}

fn hosted_wallet() -> Proof {
    let (rows, expected_balances) = bank_fixture()?;
    let did = format!("did:layerx:{}", "61".repeat(32));
    let account = wallet_gateway::account_document(
        Some([0x42; 20]),
        Some("pax1account".into()),
        Some([0x61; 32]),
        None,
    );
    let output = wallet_gateway::balances(account, 91, rows);
    let key = format!("key:lxp_live_{}", "a".repeat(64));
    let peer = Transcript::start(
        "/rpc",
        Some(format!("LayerX-Key {key}")),
        vec![Exchange {
            method: "px_getBalances",
            params: json!([did]),
            result: output.clone(),
        }],
    );
    let gateway = crate::http::Client::new_gateway(&peer.endpoint, Zeroizing::new(key))?;
    let response = gateway.post(
        "/rpc",
        &json!({"jsonrpc":"2.0","id":1,"method":"px_getBalances","params":[did]}),
        None,
    )?;
    let rendered = wallet_reads::balances(response["result"].clone(), &did, None)?;
    assert_eq!(rendered, output);
    assert_eq!(rendered["balances"], expected_balances);
    let uncredentialed = crate::http::Client::new(&peer.endpoint, None)?;
    let refused = uncredentialed
        .post(
            "/rpc",
            &json!({"jsonrpc":"2.0","id":1,"method":"px_getBalances","params":[did]}),
            None,
        )
        .err()
        .unwrap_or_else(|| panic!("{}", "hosted service needs authentication"));
    assert!(refused.contains("401"), "{refused}");
    switched_off(&peer.endpoint)?;
    assert_eq!(
        peer.observations().len(),
        2,
        "lx_ refuses before hosted transport"
    );
    assert!(peer.observations().iter().all(|seen| seen.path == "/rpc"));
    let service_key = format!("service:lxp_live_{}", "9".repeat(64));
    let invalid_key = format!("service:lxp_live_{}", "8".repeat(64));
    let service = Transcript::start(
        "/services/wallet",
        Some(format!("LayerX-Key {service_key}")),
        vec![Exchange {
            method: "px_getBalances",
            params: json!([did]),
            result: output.clone(),
        }],
    );
    let node = RpcClient::new(&service.endpoint, None)?;
    let refused = node
        .call("px_requestFunds", &json!([did, "61".repeat(32)]))
        .err()
        .unwrap_or_else(|| panic!("{}", "hosted method needs an explicit service route"));
    assert!(
        refused.starts_with("gateway_endpoint_required:"),
        "{refused}"
    );
    assert!(service.observations().is_empty());
    let endpoint = format!("{}/services/wallet", service.endpoint);
    assert!(RpcClient::new(&endpoint, None).is_err());
    let request = json!({"jsonrpc":"2.0", "id":1, "method":"px_getBalances", "params":[did]});
    for key in [None, Some(invalid_key.clone())] {
        let client = match key {
            Some(key) => http::Client::new_gateway(&service.endpoint, Zeroizing::new(key))?,
            None => http::Client::new(&service.endpoint, None)?,
        };
        let refused = client
            .post("/services/wallet", &request, None)
            .err()
            .unwrap_or_else(|| panic!("{}", "service authentication required"));
        assert!(refused.contains("401"), "{refused}");
    }
    let client = http::Client::new_gateway(&service.endpoint, Zeroizing::new(service_key.clone()))?;
    let response = client.post("/services/wallet", &request, None)?;
    assert_eq!(
        wallet_reads::balances(response["result"].clone(), &did, None)?,
        output
    );
    switched_off(&service.endpoint)?;
    assert_service_observations(&service, &did, &invalid_key, &service_key);
    Ok(())
}

fn bank_fixture() -> Result<(Vec<Value>, Value), String> {
    let vectors: Value = serde_json::from_str(include_str!(
        "../../../../services/platform/hosted/gateway/tests/fixtures/paxeer-abi-vectors.json"
    ))
    .map_err(|e| e.to_string())?;
    let vector = vectors["cases"]
        .as_array()
        .unwrap_or_else(|| panic!("{}", "bank ABI cases"))
        .iter()
        .find(|v| v["name"] == "bank.all_balances")
        .unwrap_or_else(|| panic!("{}", "committed bank producer"));
    let rows = wallet_gateway::evm::decode_coins(&encoding::hex_decode(
        "bank ABI result",
        vector["result"].as_str().unwrap_or_else(|| panic!("{}", "unprefixed bank ABI hex")),
    )?).map_err(|e| format!("{e:?}"))?.into_iter()
        .map(|(denom, amount)| json!({"denom":denom, "amount":wallet_gateway::evm::uint256_decimal(&amount)})).collect::<Vec<Value>>();
    Ok((rows, vector["out"]["response"].clone()))
}

/// # Errors
/// Returns an error if the CLI or receipt response violates its contract.
///
/// # Panics
/// Panics if a required fixture assertion is false.
#[test]
pub fn proof_rpc_expected_failure() -> Proof {
    hosted_wallet()?;
    root_receipt()
}

/// # Errors
/// Returns an error if the CLI or receipt response violates its contract.
///
/// # Panics
/// Panics if a required fixture assertion is false.
#[test]
pub fn test_retired_endpoint_rpc_identity() -> Proof {
    let recorded = case(PROGRAM);
    let peer = Transcript::receipt(&recorded);
    switched_off(&peer.endpoint)?;
    assert!(peer.observations().is_empty());
    let root = RpcClient::new(&peer.endpoint, None)?;
    let receipt = root.call(
        "pax_sequencerReceipt",
        &json!([recorded["receiptJson"]["txHash"]]),
    )?;
    assert_eq!(receipt, recorded["receiptJson"]);
    authenticated_root_receipt(&recorded)?;
    hosted_wallet()
}

/// # Errors
/// Returns an error if the CLI or receipt response violates its contract.
///
/// # Panics
/// Panics if a required fixture assertion is false.
#[test]
pub fn test_retired_endpoint_wallet_commands() -> Proof {
    hosted_wallet()?;
    root_receipt()
}

#[test]
fn hosted_wallet_authentication_and_retired_methods() -> Proof {
    hosted_wallet()
}

#[test]
fn recorded_node_receipts_remain_authentic_through_root_transport() -> Proof {
    for (id, success) in [(PROGRAM, true), (REVERT, false)] {
        let recorded = case(id);
        authentic(&recorded);
        let peer = Transcript::receipt(&recorded);
        let hash = transaction_hash(&bytes(&recorded["rawTransaction"]));
        let node = SequencerEndpoint::new(&peer.endpoint, &[]).map_err(|e| format!("{e:?}"))?;
        let Some(SequencerReceipt::Receipted(receipt)) = node
            .sequencer_receipt(&hash)
            .map_err(|e| format!("{e:?}"))?
        else {
            panic!("producer supplied an executed receipt")
        };
        assert_eq!(receipt.success, success);
        assert_eq!(
            receipt.return_data,
            bytes(&recorded["receiptJson"]["returnData"])
        );
        assert_eq!(receipt.receipt, bytes(&recorded["signedReceipt"]));
        assert_eq!(peer.observations().len(), 1);
        assert_eq!(peer.observations()[0].path, "/");
    }
    Ok(())
}
