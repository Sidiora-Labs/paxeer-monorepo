//! Task 22.33.1497 / audit 22.30.89 repair record.
//!
//! Finding -> proving test (a failing test keeps the finding open):
//! `wallet_bind_success_gap` -> `wallet_bind_success_gap`, `binding_vectors_and_nonce_recovery`
//! `test_retired_endpoint_wallet_fee` -> `test_retired_endpoint_wallet_fee`
//! `test_retired_endpoint_wallet_writes` -> `test_retired_endpoint_wallet_writes`
//! `wallet_third_shape_test` -> `wallet_third_shape_test`, `recorded_submission_and_receipt`
//! `test_retired_endpoint_wallet_reads` -> `test_retired_endpoint_wallet_reads`
//! `wallet_read_capture_gap` -> `wallet_read_capture_gap`
//! `test_retired_endpoint_wallet_watch` -> `test_retired_endpoint_wallet_watch`
//!
//! Baseline: no gate was run before implementation: this target did not exist and
//! the explicit ordered instructions require hf-build before hf-done. No baseline
//! test failure is claimed. The runner records the qualified revision and evidence.
//!
//! Qualification finding: revision 71b855417e0d3a7ad2b7e66839106e406de1c39f;
//! command cg spec done 22.33.1497 -f layerx-merge --root /work/paxeer-monorepo;
//! exit 1; private evidence /out/logs/verify-1.log.
//! Observed: five root transport cases fail with "RPC URL must end in /rpc".
//! Assumption: domain 8 must repair the production CLI transport outside this scope.
//! Harness repair: compare exact SDK wire-format addresses and pin the recorded
//! payment receipt's single height 1001; retain a negative check for height 1000.
//! Continuation: the root-admission repairs in 22.33.1493 and 22.33.1494 are present;
//! compare derived addresses as exactly 20 decoded bytes per decision 2624.
//! The subscription transport upgrades the configured root URL unchanged; assert
//! the exact root path and authenticated upgrade rather than inventing /ws.
//!
//! Wiring request: tools/platform/cli/src/rpc.rs and `rpc_subscription.rs`, domain 8:
//! accept the node root and declared WebSocket URL independently of hosted /rpc;
//! preserve authentication and lx_ refusal. The root regressions must pass unchanged.
//! Wiring request: rpc/ and precompiles/addr, domain 2: expose a Rust-gate-compatible
//! real node/precompile listener; the committed ABI/receipt peers below prove the
//! Rust producer/transport/consumer contracts, not Go state execution or persistence.
//! Wiring request: tools/platform/cli/src/wallet.rs, domain 8: accept separate node
//! and hosted-service URLs for signed writes; `px_getAccount` belongs on explicit /rpc.
//! Wiring request: tools/platform/cli/src/output.rs, domain 8: expose serialization
//! to a writer so the complete CLI envelope can be asserted without a subprocess.

use ed25519_dalek::Signer as _;
use layerx_platform_cli::{
    rpc::RpcClient as CliRpc, wallet_gateway as producer, wallet_reads, wallet_send,
    wallet_signing::SigningFacts,
};
use layerx_sdk::rpc::RpcClient as Sdk;
use serde_json::{json, Value};
use sha2::Digest as _;
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::fs::OpenOptionsExt as _;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::time::Duration;
use zeroize::Zeroizing;

type Outcome = Result<(), Box<dyn std::error::Error>>;

fn debug(error: impl std::fmt::Debug) -> Box<dyn std::error::Error> {
    std::io::Error::other(format!("{error:?}")).into()
}

/// HTTP framing for committed remote fixtures. No production client/codec is replaced.
struct FixturePeer {
    url: String,
    requests: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<()>>,
}

impl FixturePeer {
    fn start(
        path: &'static str,
        mut response: impl FnMut(&Value) -> Value + Send + 'static,
    ) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(panic_error);
        listener.set_nonblocking(true).unwrap_or_else(panic_error);
        let url = format!(
            "http://{}{}",
            listener.local_addr().unwrap_or_else(panic_error),
            path
        );
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let requests = Arc::new(Mutex::new(Vec::new()));
        let observed = requests.clone();
        let worker = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(stream) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("fixture peer accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .unwrap_or_else(panic_error);
                stream
                    .set_write_timeout(Some(Duration::from_secs(3)))
                    .unwrap_or_else(panic_error);
                let mut header = Vec::new();
                while !header.ends_with(b"\r\n\r\n") {
                    assert!(header.len() < 8192, "bounded HTTP header");
                    let mut byte = [0];
                    stream.read_exact(&mut byte).unwrap_or_else(panic_error);
                    header.push(byte[0]);
                }
                let header = std::str::from_utf8(&header).unwrap_or_else(panic_error);
                assert!(header.starts_with(&format!("POST {path} HTTP/1.1\r\n")));
                let length = header
                    .lines()
                    .find_map(|line| {
                        let (key, value) = line.split_once(':')?;
                        key.eq_ignore_ascii_case("content-length")
                            .then(|| value.trim().parse::<usize>().unwrap_or_else(panic_error))
                    })
                    .unwrap_or_else(panic_missing);
                assert!(length <= 1_048_576);
                let mut body = vec![0; length];
                stream.read_exact(&mut body).unwrap_or_else(panic_error);
                let request: Value = serde_json::from_slice(&body).unwrap_or_else(panic_error);
                let authorization = header.lines().find_map(|line| {
                    let (key, value) = line.split_once(':')?;
                    key.eq_ignore_ascii_case("authorization")
                        .then(|| value.trim())
                });
                let (status, reply) = if authorization == Some(authorization_header().as_str()) {
                    observed
                        .lock()
                        .unwrap_or_else(panic_error)
                        .push(request.clone());
                    (
                        "200 OK",
                        json!({"jsonrpc":"2.0","id":request["id"],"result":response(&request)}),
                    )
                } else {
                    ("401 Unauthorized", json!({"error":"unauthenticated"}))
                };
                let body = serde_json::to_vec(&reply).unwrap_or_else(panic_error);
                write!(stream, "HTTP/1.1 {status}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).unwrap_or_else(panic_error);
                stream.write_all(&body).unwrap_or_else(panic_error);
            }
        });
        Self {
            url,
            requests,
            stop,
            worker: Some(worker),
        }
    }

    fn requests(&self) -> Vec<Value> {
        self.requests.lock().unwrap_or_else(panic_error).clone()
    }
}

impl Drop for FixturePeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            if std::thread::panicking() {
                let _ = worker.join();
            } else {
                worker.join().unwrap_or_else(panic_error);
            }
        }
    }
}

fn credential() -> Zeroizing<String> {
    Zeroizing::new(format!("repair1497:lxp_live_{}", "a".repeat(64)))
}
fn authorization_header() -> String {
    format!("LayerX-Key {}", credential().as_str())
}
fn sdk(url: &str) -> Result<Sdk, Box<dyn std::error::Error>> {
    let credential = layerx_sdk::programs::LayerXKeyCredential::new(
        "repair1497",
        layerx_sdk::production::SecretBytes::new(format!("lxp_live_{}", "a".repeat(64)).as_bytes())
            .map_err(debug)?,
    )
    .map_err(debug)?;
    Sdk::connect_hosted(url, Some(credential)).map_err(debug)
}
fn bytes(text: &str) -> Vec<u8> {
    layerx_platform_cli::encoding::hex_decode(
        "committed fixture",
        text.strip_prefix("0x").unwrap_or(text),
    )
    .unwrap_or_else(panic_error)
}
fn abi(name: &str) -> Value {
    let vectors: Value = serde_json::from_str(include_str!(
        "../../../../services/platform/hosted/gateway/tests/fixtures/paxeer-abi-vectors.json"
    ))
    .unwrap_or_else(panic_error);
    vectors["cases"]
        .as_array()
        .unwrap_or_else(panic_missing)
        .iter()
        .find(|row| row["name"] == name)
        .unwrap_or_else(panic_missing)
        .clone()
}
fn derivation() -> Value {
    serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/account-derivation-v1.json"
    ))
    .unwrap_or_else(panic_error)
}
fn receipts() -> Value {
    serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap_or_else(panic_error)
}
fn switched_off() {
    for method in [
        "lx_getBalances",
        "lx_send",
        "lx_estimateFee",
        "lx_subscribe",
    ] {
        let error = layerx_platform_cli::rpc::request(method, &json!([]))
            .err()
            .unwrap_or_else(|| panic!("expected error"));
        assert_eq!(
            error,
            format!("rpc_method_unavailable: {method} is switched off")
        );
    }
}
fn word(value: u128) -> Vec<u8> {
    let mut word = vec![0; 16];
    word.extend_from_slice(&value.to_be_bytes());
    word
}
fn did() -> String {
    format!("did:layerx:{}", "61".repeat(32))
}
fn native() -> [u8; 32] {
    let mut asset = [0; 32];
    asset[0] = 1;
    asset
}
fn account_id(actor: &str, asset: [u8; 32]) -> [u8; 32] {
    let name = layerx_wire::account::account_name_for_asset(actor, asset, native())
        .unwrap_or_else(panic_error);
    layerx_wire::hash::account_id_for_protocol(&name, 3).unwrap_or_else(panic_error)
}
fn balance_document() -> Value {
    let unified = abi("addr.getUnifiedAccount");
    let account = producer::evm::decode_unified_account(&bytes(
        unified["result"].as_str().unwrap_or_else(panic_missing),
    ))
    .unwrap_or_else(panic_error);
    let coins = abi("bank.all_balances");
    let rows = producer::evm::decode_coins(&bytes(
        coins["result"].as_str().unwrap_or_else(panic_missing),
    ))
    .unwrap_or_else(panic_error)
    .into_iter()
    .map(|(denom, amount)| json!({"denom":denom,"amount":producer::evm::uint256_decimal(&amount)}))
    .collect();
    producer::balances(
        producer::account_document(
            Some(account.evm),
            Some(account.pax_address),
            Some(account.did_public_key),
            Some(account.layerx_main_account_id),
        ),
        91,
        rows,
    )
}

#[test]
fn binding_vectors_and_nonce_recovery() -> Outcome {
    use layerx_client::account_binding::{
        decode_bind_nonce, plan_bind, sign_bind_transaction, BindError, BindFees, BindPlan,
    };
    use layerx_crypto::{
        account_derivation::derive_from_mnemonic,
        evm_transaction::{decode_signed, recover_sender},
    };
    let corpus = derivation();
    let vector = &corpus["mnemonic_vectors"][0];
    for expected in vector["accounts"].as_array().unwrap_or_else(panic_missing) {
        let index = u32::try_from(expected["index"].as_u64().unwrap_or_else(panic_missing))
            .unwrap_or_else(panic_error);
        let account = derive_from_mnemonic(
            vector["mnemonic"].as_str().unwrap_or_else(panic_missing),
            "",
            index,
        )?;
        assert_eq!(
            account.evm_address(),
            layerx_platform_cli::encoding::fixed_hex::<20>(
                "fixture EVM address",
                expected["evm_address"]
                    .as_str()
                    .unwrap_or_else(panic_missing)
                    .strip_prefix("0x")
                    .unwrap_or_else(panic_missing),
            )?
        );
        assert_eq!(account.did(), expected["did"]);
        let BindPlan::Bind(call) = plan_bind(&account, 125, None, 0)? else {
            panic!("unbound pair")
        };
        assert_eq!(producer::hex(call.data()), expected["bind"]["calldata"]);
        let raw = sign_bind_transaction(
            &account,
            125,
            &call,
            BindFees {
                evm_nonce: 0,
                max_priority_fee_per_gas: 1_000_000_000,
                max_fee_per_gas: 2_000_000_000,
                gas_limit: 200_000,
            },
        )?;
        assert_eq!(producer::hex(&raw), expected["bind"]["transaction"]["raw"]);
        let (transaction, signature) = decode_signed(&raw).map_err(debug)?;
        assert_eq!(
            recover_sender(&transaction, &signature).map_err(debug)?,
            account.evm_address()
        );
        assert!(matches!(
            plan_bind(&account, 125, Some(account.layerx_public_key()), 1)?,
            BindPlan::AlreadyBound
        ));
        assert_eq!(
            plan_bind(&account, 125, Some([0x61; 32]), 1)
                .err()
                .unwrap_or_else(|| panic!("expected error")),
            BindError::BoundToDifferentDid { bound: [0x61; 32] }
        );
        let nonce = decode_bind_nonce(&bytes(
            abi("addr.layerXBindNonce")["result"]
                .as_str()
                .unwrap_or_else(panic_missing),
        ))?;
        assert_eq!(nonce, 7);
        let BindPlan::Bind(recovered) = plan_bind(&account, 125, None, nonce)? else {
            panic!("recovered nonce")
        };
        recovered.signed_binding().verify().map_err(debug)?;
        assert_eq!(recovered.signed_binding().binding().nonce(), 7);
        assert!(recovered
            .signed_binding()
            .binding()
            .verify(
                &call.signed_binding().public_key(),
                &call.signed_binding().signature()
            )
            .is_err());
        assert_ne!(recovered.data(), call.data());
    }
    Ok(())
}

#[test]
fn wallet_bind_success_gap() -> Outcome {
    switched_off();
    let fixture = derivation();
    let account = layerx_crypto::account_derivation::derive_from_mnemonic(
        fixture["mnemonic_vectors"][0]["mnemonic"]
            .as_str()
            .unwrap_or_else(panic_missing),
        "",
        0,
    )?;
    let peer = FixturePeer::start("/", |request| {
        match request["method"].as_str().unwrap_or_else(panic_missing) {
            "eth_chainId" => json!("0x7d"),
            "eth_gasPrice" => json!("0x2"),
            "eth_maxPriorityFeePerGas" => json!("0x1"),
            method => panic!("unexpected binding method {method}"),
        }
    });
    assert_eq!(
        sdk(&peer.url)?.eip1559_parameters().map_err(debug)?,
        (125, 1, 5)
    );
    let client = CliRpc::new(&peer.url, Some(credential())).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "wallet bind must accept the node root URL; rpc.rs is outside this task's scope"
        )
    });
    assert_eq!(client.call("eth_chainId", &json!([]))?, "0x7d");
    assert_eq!(
        account.evm_address(),
        layerx_platform_cli::encoding::fixed_hex::<20>(
            "fixture EVM address",
            fixture["mnemonic_vectors"][0]["accounts"][0]["evm_address"]
                .as_str()
                .unwrap_or_else(panic_missing)
                .strip_prefix("0x")
                .unwrap_or_else(panic_missing),
        )?
    );
    assert_eq!(
        account.did(),
        fixture["mnemonic_vectors"][0]["accounts"][0]["did"]
    );
    Ok(())
}

#[test]
fn test_retired_endpoint_wallet_fee() -> Outcome {
    switched_off();
    let peer = FixturePeer::start("/", |request| {
        match request["method"].as_str().unwrap_or_else(panic_missing) {
            "eth_estimateGas" => {
                assert_eq!(request["params"][1], "pending");
                assert_eq!(
                    request["params"][0]["from"],
                    "0x1111111111111111111111111111111111111111"
                );
                json!("0x5208")
            }
            "eth_gasPrice" => json!("0x2"),
            method => panic!("unexpected fee method {method}"),
        }
    });
    let call = layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 21_000,
        to: [0x22; 20],
        data: vec![0xde, 0xad, 0xbe, 0xef],
    };
    assert_eq!(
        sdk(&peer.url)?
            .estimate_fee_from(&call, [0x11; 20])
            .map_err(debug)?,
        21_000
    );
    assert!(Sdk::connect_node(&peer.url, None)
        .map_err(debug)?
        .estimate_fee_from(&call, [0x11; 20])
        .is_err());
    let client = CliRpc::new(&peer.url, Some(credential())).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "wallet fee must use the public node root URL"
        )
    });
    assert_eq!(client.call("eth_gasPrice", &json!([]))?, "0x2");
    assert_eq!(
        peer.requests().last().unwrap_or_else(panic_missing)["method"],
        "eth_gasPrice"
    );
    Ok(())
}

#[test]
fn recorded_submission_and_receipt() -> Outcome {
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
    let vectors = receipts();
    let vector = &vectors["cases"][2];
    let raw = bytes(
        vector["rawTransaction"]
            .as_str()
            .unwrap_or_else(panic_missing),
    );
    let (call, signature) = decode_signed(&raw).map_err(debug)?;
    let sender = recover_sender(&call, &signature).map_err(debug)?;
    assert_eq!(
        layerx_sdk::rpc::ethereum_address(&sender),
        vector["receiptJson"]["from"]
    );
    let hash = transaction_hash(&raw);
    let expected = vector["receiptJson"].clone();
    assert_eq!(format!("0x{}", producer::hex(&hash)), expected["txHash"]);
    let expected_raw = vector["rawTransaction"].clone();
    let response = expected.clone();
    let peer = FixturePeer::start("/", move |request| {
        match request["method"].as_str().unwrap_or_else(panic_missing) {
            "eth_sendRawTransaction" => {
                assert_eq!(request["params"], json!([expected_raw]));
                response["txHash"].clone()
            }
            "pax_sequencerReceipt" => {
                assert_eq!(request["params"], json!([response["txHash"]]));
                response.clone()
            }
            method => panic!("unexpected submission method {method}"),
        }
    });
    let sdk = sdk(&peer.url)?;
    assert_eq!(sdk.send_raw_transaction(&raw).map_err(debug)?, hash);
    let receipt = sdk.sequencer_receipt(hash).map_err(debug)?;
    assert_eq!(receipt, expected);
    let policy = layerx_sdk::rpc_verification::ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: layerx_proof::inclusion::SequencerAuthorization::new(
            [0x6b; 32],
            layerx_platform_cli::encoding::fixed_hex(
                "pinned key",
                "8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e",
            )?,
            1001,
            1001,
        ),
        trusted_checkpoint_context_digest: None,
    };
    assert_eq!(receipt["height"], "0x3e9");
    let wrong_height = layerx_sdk::rpc_verification::ReceiptPolicy {
        protocol_version: 2,
        network_id: 125,
        sequencer: layerx_proof::inclusion::SequencerAuthorization::new(
            [0x6b; 32],
            policy.sequencer.public_key(),
            1000,
            1000,
        ),
        trusted_checkpoint_context_digest: None,
    };
    assert!(
        layerx_sdk::rpc_verification::verify_node_receipt(&receipt, hash, &wrong_height).is_err()
    );
    let (_, canonical) = layerx_sdk::rpc_verification::verify_node_receipt(&receipt, hash, &policy)
        .map_err(debug)?;
    assert_eq!(
        canonical,
        bytes(expected["receipt"].as_str().unwrap_or_else(panic_missing))
    );
    let mut altered = receipt;
    altered["gasUsed"] = json!("0x1");
    assert!(layerx_sdk::rpc_verification::verify_node_receipt(&altered, hash, &policy).is_err());
    assert_eq!(peer.requests().len(), 2);
    Ok(())
}

#[test]
fn test_retired_endpoint_wallet_writes() -> Outcome {
    switched_off();
    let peer = FixturePeer::start("/", |request| {
        assert_eq!(request["method"], "eth_chainId");
        json!("0x7d")
    });
    let client = CliRpc::new(&peer.url, Some(credential())).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "wallet writes must accept the node root URL without adding /rpc"
        )
    });
    assert_eq!(client.call("eth_chainId", &json!([]))?, "0x7d");
    Ok(())
}

fn account_peer(actor: String, owner: [u8; 32], asset: [u8; 32], id: [u8; 32]) -> FixturePeer {
    let producer_actor = actor;
    FixturePeer::start("/rpc", move |request| {
        assert_eq!(request["method"], "px_getAccount");
        assert_eq!(
            request["params"],
            json!([producer_actor, producer::hex(&asset)])
        );
        producer::ledger_account(&producer_actor, &producer::hex(&asset), 73, &mut |data| {
            match &data[..4] {
                [0x3f, 0x05, 0x0c, 0xbc] => {
                    assert_eq!(&data[4..36], &owner);
                    assert_eq!(&data[36..], &asset);
                    Ok(id.to_vec())
                }
                [0x37, 0x95, 0x5a, 0xb2] => {
                    assert_eq!(&data[4..], &id);
                    Ok(owner.to_vec())
                }
                [0xce, 0x2f, 0x92, 0x1f] => {
                    assert_eq!(&data[4..36], &id);
                    assert_eq!(&data[36..], &asset);
                    Ok(word(19))
                }
                [0xb1, 0xff, 0xc0, 0x3e] => {
                    assert_eq!(&data[4..36], &id);
                    assert_eq!(&data[36..], &asset);
                    Ok(word(1000))
                }
                selector => panic!("unexpected ledger ABI {selector:?}"),
            }
        })
        .unwrap_or_else(panic_error)
    })
}

fn assert_account_snapshot(
    client: &Sdk,
    actor: &str,
    asset: [u8; 32],
    peer: &FixturePeer,
) -> Outcome {
    let snapshot = client
        .get_ledger_account(actor, asset)
        .map_err(debug)?
        .into_value();
    assert_eq!(snapshot["evidence"], "committed_eth_call");
    assert_eq!(snapshot["height"], 73);
    assert_eq!(snapshot["balance"], "1000");
    assert!(snapshot.get("proof_material").is_none());
    assert!(snapshot.get("verification").is_none());
    for (field, bad) in [
        ("owner", json!("44".repeat(32))),
        ("account_id", json!("44".repeat(32))),
        ("asset_id", json!("44".repeat(32))),
        ("next_sequence", json!("019")),
    ] {
        let mut corrupted = snapshot.clone();
        corrupted[field] = bad;
        assert!(
            layerx_sdk::rpc::LedgerAccountSnapshot::decode(corrupted, actor, asset).is_err(),
            "{field}"
        );
    }
    assert!(peer
        .requests()
        .iter()
        .all(|request| request["method"] == "px_getAccount"));
    Ok(())
}

#[test]
fn wallet_third_shape_test() -> Outcome {
    let account = layerx_crypto::account_derivation::derive_from_seed(&[7; 64], 0)?;
    let actor = account.did();
    let owner = account.layerx_public_key();
    let asset = [0x22; 32];
    let id = account_id(&actor, asset);
    let peer = account_peer(actor.clone(), owner, asset, id);
    let client = sdk(&peer.url)?;
    let facts = SigningFacts {
        actor: &actor,
        public_key: owner,
        network_id: 402,
        identity_next_sequence: 7,
        not_before_ms: 1000,
        expires_at_ms: 2000,
        fee_limit: 200_000,
        idempotency_key: [4; 32],
    };
    let debit = wallet_send::native_send_debit(&client, &facts, asset, [0x33; 32], 25)?;
    assert_eq!(debit.from, id);
    assert_eq!(debit.source_sequence, 19);
    assert_eq!(facts.identity_next_sequence, 7);
    assert_ne!(facts.identity_next_sequence, debit.source_sequence);
    let signing_key = ed25519_dalek::SigningKey::from_bytes(account.layerx_seed());
    let mut digest = sha2::Sha256::new();
    digest.update(layerx_wire::hash::Domain::SignaturePreimage.tag());
    digest.update(debit.authorization_message().map_err(debug)?);
    let signature = signing_key.sign(&digest.finalize()).to_bytes();
    let payload = debit.encode_signed(owner, signature).map_err(debug)?;
    let options = layerx_crypto::payments::CallOptions {
        from: account.evm_address(),
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 0,
        max_fee_per_gas: 2,
        gas_limit: 21_000,
    };
    let prepared = layerx_platform_cli::wallet_signing::PreparedPayment::from_send(
        &payload, &facts, &options,
    )?;
    assert_eq!(prepared.confirmation()["source_account_sequence"], "19");
    assert_eq!(prepared.confirmation()["nonce"], "7");
    let directory = tempfile::tempdir()?;
    let path = directory.path().join("derived.json");
    let mut file = std::fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(&path)?;
    serde_json::to_writer(
        &mut file,
        &json!({"index":0,"did":actor,"evm_address":account.evm_address_text(),
        "evm_private_key":producer::hex(account.evm_secret().unwrap_or_else(panic_missing)),"layerx_seed":producer::hex(account.layerx_seed())}),
    )?;
    let key = layerx_platform_cli::wallet_signing::EvmSigningKey::read(&path, &actor, owner)?;
    let (signed, hash) = prepared.sign_with_id(&key)?;
    let (decoded, signature) =
        layerx_crypto::evm_transaction::decode_signed(&signed).map_err(debug)?;
    assert_eq!(decoded.nonce, 7);
    assert_eq!(
        layerx_crypto::evm_transaction::recover_sender(&decoded, &signature).map_err(debug)?,
        account.evm_address()
    );
    let disclosure =
        layerx_crypto::payments::bind_call(&decoded, account.evm_address()).map_err(debug)?;
    assert_eq!(disclosure.counterparties[0].account, id);
    assert_eq!(disclosure.counterparties[1].account, [0x33; 32]);
    assert_eq!(disclosure.amounts[0].value, 25);
    let expected_raw = signed.clone();
    let node_peer = FixturePeer::start("/", move |request| {
        assert_eq!(request["method"], "eth_sendRawTransaction");
        assert_eq!(
            request["params"],
            json!([format!("0x{}", producer::hex(&expected_raw))])
        );
        json!(format!(
            "0x{}",
            producer::hex(&layerx_crypto::evm_transaction::transaction_hash(
                &expected_raw
            ))
        ))
    });
    assert_eq!(
        sdk(&node_peer.url)?
            .send_raw_transaction(&signed)
            .map_err(debug)?,
        hash
    );
    assert_account_snapshot(&client, &actor, asset, &peer)?;
    Ok(())
}

fn assert_custody_asset() -> Outcome {
    let asset_fixture: Value =
        serde_json::from_str(include_str!("fixtures/wallet-assets-v1.json"))?;
    let asset = producer::key(
        asset_fixture["asset_id"]
            .as_str()
            .unwrap_or_else(panic_missing),
    )?;
    let issuer = bytes(
        asset_fixture["issuer"]
            .as_str()
            .unwrap_or_else(panic_missing),
    );
    let salt = bytes(asset_fixture["salt"].as_str().unwrap_or_else(panic_missing));
    let mut abi = Vec::new();
    for field in [
        asset.to_vec(),
        word(384),
        word(448),
        word(38),
        word(u128::MAX),
        issuer,
        salt,
        word(1),
        word(0),
        word(512),
        word(0),
        word(
            asset_fixture["amount"]
                .as_str()
                .unwrap_or_else(panic_missing)
                .parse::<u128>()?,
        ),
    ] {
        abi.extend(field);
    }
    for text in ["PAY", "Payment €"] {
        abi.extend(word(text.len() as u128));
        abi.extend(text.as_bytes());
        abi.resize(abi.len().div_ceil(32) * 32, 0);
    }
    abi.extend(word(0));
    let mut row = json!({"asset_id":producer::hex(&asset),"kind":"ledger","denom":format!("layerx/{}",producer::hex(&asset))});
    row.as_object_mut().unwrap_or_else(panic_missing).extend(
        producer::ledger_asset_metadata(&abi, asset)?
            .as_object()
            .unwrap_or_else(panic_missing)
            .clone(),
    );
    assert_eq!(row["supply"], asset_fixture["amount"]);
    assert_eq!(row["symbol"], "PAY");
    assert_eq!(row["name"], "Payment €");
    let rows = vec![row.clone()];
    let assets = FixturePeer::start("/rpc", move |request| {
        assert_eq!(request["method"], "px_listAssets");
        let (cursor, limit) =
            producer::page_params(Some(&request["params"])).unwrap_or_else(panic_error);
        producer::assets_page(rows.clone(), 94, cursor, limit, None).unwrap_or_else(panic_error)
    });
    let client = sdk(&assets.url)?;
    let page = client
        .list_custody_assets_page(None, None)
        .map_err(debug)?
        .into_value();
    assert_eq!(page["height"], 94);
    assert_eq!(page["assets"], json!([row]));
    assert_eq!(page["units"], "base");
    assert_eq!(page["order"], "registration");
    assert_eq!(page["next_cursor"], Value::Null);
    let info = client.get_custody_asset(asset).map_err(debug)?;
    assert_eq!(info["asset"], row);
    assert_eq!(info["height"], 94);
    assert_eq!(assets.requests().len(), 2);
    Ok(())
}

#[test]
fn wallet_read_capture_gap() -> Outcome {
    let expected = balance_document();
    assert_eq!(
        expected["balances"],
        abi("bank.all_balances")["out"]["response"]
    );
    let response = expected.clone();
    let peer = FixturePeer::start("/rpc", move |request| {
        assert_eq!(request["method"], "px_getBalances");
        assert_eq!(request["params"], json!([did()]));
        response.clone()
    });
    let client = sdk(&peer.url)?;
    let data = wallet_reads::balances(
        client.get_balances(&did()).map_err(debug)?.into_value(),
        &did(),
        None,
    )?;
    assert_eq!(data, expected);
    assert_eq!(data["height"], 91);
    assert_eq!(data["units"], "base");
    assert_eq!(data["evidence"], "committed_eth_call");
    assert_eq!(data["balances"][0]["amount"], "123456789");
    assert_eq!(
        data["balances"][1]["amount"],
        "1606938044258990275541962092341162602522202993782792835301376"
    );
    assert!(data.get("accounts").is_none());
    assert!(data.get("verification").is_none());
    assert!(wallet_reads::balances(data.clone(), "did:layerx:other", None).is_err());
    for (field, bad) in [
        ("height", json!("91")),
        ("evidence", json!("state_proven")),
        ("units", json!("display")),
    ] {
        let mut altered = data.clone();
        altered[field] = bad;
        assert!(
            wallet_reads::balances(altered, &did(), None).is_err(),
            "{field}"
        );
    }
    assert!(Sdk::connect_hosted(&peer.url, None)
        .map_err(debug)?
        .get_balances(&did())
        .is_err());
    assert_custody_asset()?;
    Ok(())
}

#[test]
fn test_retired_endpoint_wallet_reads() -> Outcome {
    switched_off();
    let peer = FixturePeer::start("/", |request| {
        assert_eq!(request["method"], "eth_chainId");
        json!("0x7d")
    });
    let client = CliRpc::new(&peer.url, Some(credential())).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "wallet read transport must preserve the node root URL"
        )
    });
    assert_eq!(client.call("eth_chainId", &json!([]))?, "0x7d");
    Ok(())
}

#[test]
fn test_retired_endpoint_wallet_watch() -> Outcome {
    switched_off();
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let root = format!("http://{}", listener.local_addr()?);
    let client = CliRpc::new(&root, Some(credential())).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "wallet watch must upgrade the configured root endpoint unchanged"
        )
    });
    let worker = std::thread::spawn(move || {
        listener.set_nonblocking(true).unwrap_or_else(panic_error);
        let deadline = std::time::Instant::now() + Duration::from_secs(3);
        let stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(
                        std::time::Instant::now() < deadline,
                        "watch never connected"
                    );
                    std::thread::sleep(Duration::from_millis(1));
                }
                Err(error) => panic!("watch peer: {error}"),
            }
        };
        stream
            .set_read_timeout(Some(Duration::from_secs(3)))
            .unwrap_or_else(panic_error);
        let mut socket = tungstenite::accept_hdr(
            stream,
            |request: &tungstenite::handshake::server::Request,
             response: tungstenite::handshake::server::Response| {
                assert_eq!(request.uri().path(), "/");
                assert_eq!(request.headers()["authorization"], authorization_header());
                Ok(response)
            },
        )
        .unwrap_or_else(panic_error);
        let request: Value = serde_json::from_str(
            socket
                .read()
                .unwrap_or_else(panic_error)
                .to_text()
                .unwrap_or_else(panic_error),
        )
        .unwrap_or_else(panic_error);
        assert_eq!(request["method"], "eth_subscribe");
        assert_eq!(
            request["params"],
            json!(["logs",{"address":"0x1234567890123456789012345678901234567890","topics":[]}])
        );
        socket
            .send(tungstenite::Message::Text(
                json!({"jsonrpc":"2.0","id":request["id"],"result":"0x123abc"})
                    .to_string()
                    .into(),
            ))
            .unwrap_or_else(panic_error);
        // A retired notification is a refusal, never commitment evidence.
        socket.send(tungstenite::Message::Text(json!({"jsonrpc":"2.0","method":"lx_subscription","params":{"subscription":"0x123abc","result":{}}}).to_string().into())).unwrap_or_else(panic_error);
    });
    let result = client.subscribe(
        &json!(["account", "0x1234567890123456789012345678901234567890"]),
        Duration::from_secs(2),
    );
    worker.join().unwrap_or_else(panic_error);
    assert_eq!(
        result.err().unwrap_or_else(|| panic!("expected error")),
        "unexpected subscription notification method"
    );
    Ok(())
}

fn panic_error<T, E: std::fmt::Debug>(error: E) -> T {
    panic!("{error:?}")
}

fn panic_missing<T>() -> T {
    panic!("expected value")
}
