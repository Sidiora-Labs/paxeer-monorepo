use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use base64::Engine as _;
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_service::journeys::{
    BankCommitEvidence, Endpoint, MoveLegExecution, Relationship, RouteRequest, RouteResolver,
    SendRoute, BANK_ADDRESS,
};
use layerx_human_service::server::production_components::web_identity;
use layerx_human_service::server::schema::ApiSchema;
use layerx_human_service::server::Listener;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, EvmAddress, NetworkId, ProtocolVersion, PublicKey,
    SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const README: &str = include_str!("../README.md");

fn unhex(text: &str) -> Vec<u8> {
    text.strip_prefix("0x")
        .unwrap_or(text)
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).or_fail("hex text"), 16)
                .or_fail("hex byte")
        })
        .collect()
}
fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut text = String::from("0x");
    for byte in bytes {
        write!(text, "{byte:02x}").or_fail("hex output");
    }
    text
}
fn transcript() -> Value {
    serde_json::from_str(include_str!(
        "../src/journeys/repair_22_31_153_transcript.json"
    ))
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "committed node/sequencer and Comet producer transcript"
        )
    })
}
fn keys() -> Vec<ReceiptKeyEntry> {
    let document = transcript();
    let key = &document["key"];
    vec![ReceiptKeyEntry {
        key_id: unhex(key["keyId"].as_str().or_fail("key id"))
            .try_into()
            .or_fail("key id width"),
        public_key: unhex(key["publicKey"].as_str().or_fail("public key"))
            .try_into()
            .or_fail("key width"),
        first_height: key["firstHeight"].as_u64().or_fail("first height"),
        last_height: key["lastHeight"].as_u64().or_fail("last height"),
    }]
}
fn sender() -> [u8; 20] {
    unhex(transcript()["sender"].as_str().or_fail("recorded sender"))
        .try_into()
        .or_fail("sender width")
}
struct RpcPeer {
    endpoint: SequencerEndpoint,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}
impl RpcPeer {
    fn start(answer: impl Fn(&Value) -> Value + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").or_fail("loopback RPC");
        listener
            .set_nonblocking(true)
            .or_fail("nonblocking listener");
        let endpoint = SequencerEndpoint::new(
            &format!("http://{}/", listener.local_addr().or_fail("address")),
            &[],
        )
        .or_fail("production client");
        let stop = Arc::new(AtomicBool::new(false));
        let shutdown = stop.clone();
        let calls = Arc::new(Mutex::new(Vec::new()));
        let observed = calls.clone();
        let worker = thread::spawn(move || {
            while !shutdown.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(value) => value,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .or_fail("bounded socket");
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).or_fail("HTTP header"), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().or_fail("length"));
                    }
                }
                let mut body = vec![0; length.or_fail("content length")];
                assert!(body.len() <= 65_536);
                reader.read_exact(&mut body).or_fail("RPC body");
                let request: Value = serde_json::from_slice(&body).or_fail("RPC JSON");
                let result = answer(&request);
                observed
                    .lock()
                    .or_fail("call inventory")
                    .push(request.clone());
                let body =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).or_fail("response");
                stream.flush().or_fail("flush");
            }
        });
        Self {
            endpoint,
            calls,
            stop,
            worker: Some(worker),
        }
    }
    fn receipts() -> Self {
        let document = transcript();
        Self::start(move |request| {
            let record = &document["records"][0];
            match request["method"].as_str().or_fail("method") {
                "pax_sequencerReceipt" => {
                    assert_eq!(request["params"][0], record["view"]["txHash"]);
                    record["view"].clone()
                }
                "eth_getTransactionReceipt" => {
                    assert_eq!(request["params"][0], record["view"]["txHash"]);
                    record["landing"].clone()
                }
                "px_getCommit" => record["material"].clone(),
                "px_getValidators" => record["validators"].clone(),
                "eth_getBlockByNumber" if request["params"][1] == false => {
                    json!({"number":request["params"][0], "stateRoot":record["appHash"]})
                }
                "eth_getBlockByNumber" => record["block"].clone(),
                other => panic!("unrecorded producer method {other}"),
            }
        })
    }
}
impl Drop for RpcPeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .or_fail("worker")
            .join()
            .or_fail("RPC peer thread");
    }
}

fn commit_proof() -> BankCommitEvidence {
    let document = transcript();
    let record = &document["records"][0];
    let proof = &record["proof"];
    let batch = unhex(record["batch"].as_str().or_fail("signed batch"));
    let mut leaf = b"LXAR\x01".to_vec();
    leaf.extend(
        proof["total"]
            .as_str()
            .or_fail("total")
            .parse::<i64>()
            .or_fail("total integer")
            .to_be_bytes(),
    );
    leaf.extend(
        proof["index"]
            .as_str()
            .or_fail("index")
            .parse::<i64>()
            .or_fail("index integer")
            .to_be_bytes(),
    );
    leaf.extend(Sha256::digest(&batch));
    let mut fields = vec![leaf];
    fields.extend(
        proof["aunts"]
            .as_array()
            .or_fail("aunts")
            .iter()
            .map(|aunt| {
                base64::engine::general_purpose::STANDARD
                    .decode(aunt.as_str().or_fail("aunt"))
                    .or_fail("base64 proof")
            }),
    );
    BankCommitEvidence {
        chain_id: record["material"]["signedHeader"]["header"]["chain_id"]
            .as_str()
            .or_fail("chain")
            .to_owned(),
        trusted_validators: record["validators"].clone(),
        committed_header: record["material"]["signedHeader"].clone(),
        covering_height: record["height"].as_u64().or_fail("height"),
        batch_transaction: batch,
        transaction_proof: fields,
        activity_ids: vec![unhex(record["view"]["txHash"].as_str().or_fail("hash"))
            .try_into()
            .or_fail("hash width")],
    }
}

#[test]
fn doc_13_native_bank_call_uses_the_sender_pending_nonce() {
    let node = RpcPeer::start(|request| {
        assert_eq!(request["method"], "eth_getTransactionCount");
        assert_eq!(request["params"], json!([hex(&sender()), "pending"]));
        json!("0x7")
    });
    let nonce = node
        .endpoint
        .pending_nonce(&sender())
        .or_fail("real pending-nonce client");
    let request = RouteRequest {
        source: Endpoint::PaxeerWallet,
        destination: Endpoint::PaxeerWallet,
        relationship: Relationship::BankTransfer {
            from: EvmAddress::new(sender()),
            to: EvmAddress::new([0x22; 20]),
            route: SendRoute {
                account_sequence: Sequence::from_u64(nonce),
                idempotency_key: IdempotencyKey::new([0x31; 32]),
                expires_at: TimestampSeconds::from_u64(u64::MAX),
                context_hash: ContextHash::new([0; 32]),
                authorization: SendAuthorization::new(
                    SendAuthorizationKind::Owner,
                    PublicKey::new([0x11; 32]),
                    AuthorizationSignature::new([0x22; 64]),
                ),
                network_id: NetworkId::new(125).or_fail("network"),
                protocol_version: ProtocolVersion::new(3).or_fail("protocol"),
            },
            pointer: EvmAddress::new(BANK_ADDRESS),
            denom: "uhpx".to_owned(),
            chain_id: 125,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1_000_000_000_000,
            gas_limit: 1_000_000,
        },
        asset: AssetId::new(Sha256::digest(b"uhpx").into()),
        amount: Amount::from_u128(90),
    };
    let route = RouteResolver::resolve(&request).or_fail("production bank call producer");
    let [call] = route.bank_transfers() else {
        panic!("exactly one native call")
    };
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, nonce);
    assert_eq!(call.to, BANK_ADDRESS);
    assert_eq!(hex(&call.to), "0x0000000000000000000000000000000000001001");
    assert_eq!(
        &call.data[..4],
        &layerx_crypto::disclosure::call::ledger_selector("send(address,address,string,uint256)")
    );
    let mut amount = [0; 32];
    amount[31] = 90;
    let disclosure = bind_call(call, sender()).or_fail("production signing consumer");
    assert_eq!(
        disclosure.construct_method().or_fail("bank ABI"),
        Some(ConstructMethod::BankSend {
            from: sender(),
            to: [0x22; 20],
            denom: "uhpx".to_owned(),
            amount,
        })
    );
    let secret = unhex(
        transcript()["secret"]
            .as_str()
            .or_fail("synthetic recorded key"),
    );
    let raw = call
        .sign(&secret.try_into().or_fail("key width"))
        .or_fail("real signing");
    let (decoded, signature) = decode_signed(&raw).or_fail("real transaction consumer");
    assert_eq!(decoded, *call);
    assert_eq!(
        recover_sender(&decoded, &signature).or_fail("recover sender"),
        sender()
    );
    assert_eq!(
        layerx_crypto::evm_transaction::assemble(&decoded, &signature),
        raw,
        "canonical zero-value type-2 envelope"
    );
    assert_eq!(raw.first(), Some(&2));
    assert_eq!(node.calls.lock().or_fail("calls").len(), 1);
    for text in [
        "pending nonce",
        "eth_getTransactionCount",
        "`pending`",
        "125",
        &hex(&call.to),
        "send(address,address,string,uint256)",
    ] {
        assert!(
            README.contains(text),
            "README must explain the executed bank path: {text}"
        );
    }
}

#[test]
fn doc_13_committed_receipt_requires_batch_quorum_and_inclusion() {
    let node = RpcPeer::receipts();
    let document = transcript();
    let raw = unhex(
        document["records"][0]["raw"]
            .as_str()
            .or_fail("recorded signed transaction"),
    );
    let hash = transaction_hash(&raw);
    let Lookup::Verified(receipted) = lookup_authenticated(
        &node.endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys(),
        },
    )
    .or_fail("real receipt transport and authentication") else {
        panic!("signed producer receipt")
    };
    assert_eq!(receipted.rung(), Rung::Receipted);
    let mut leg = MoveLegExecution::new(
        [0x31; 32],
        AgentDid::new("did:layerx:alice").or_fail("actor"),
        AuthorityRef::new("custody-human-primary").or_fail("authority"),
        document["nonce"].as_u64().or_fail("nonce"),
        0,
        u64::MAX,
        1_000_000_000_000_000_000,
    )
    .or_fail("production receipt consumer");
    assert!(!leg.complete());
    assert!(!leg.finalised());
    leg.complete_from_receipt(hash, &receipted)
        .or_fail("matching receipted execution");
    assert!(leg.complete());
    assert!(
        !leg.finalised(),
        "a receipt signature alone is not economic finality"
    );
    let proof = commit_proof();
    let committed = proof
        .authenticate(&node.endpoint, hash, &keys())
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "real batch, quorum and inclusion verification"
            )
        });
    assert_eq!(committed.rung(), Rung::Committed);
    assert_eq!(
        committed.receipt().canonical_bytes(),
        receipted.canonical_bytes()
    );
    assert_eq!(committed.receipt().activity_id(), hash);
    for damage in 0..5 {
        let mut changed = proof.clone();
        match damage {
            0 => changed.batch_transaction[100] ^= 1,
            1 => changed.activity_ids[0][0] ^= 1,
            2 => changed.transaction_proof[0][22] ^= 1,
            3 => changed.trusted_validators["validators"][0]["votingPower"] = json!("11"),
            _ => changed.committed_header["header"]["app_hash"] = json!("00".repeat(32)),
        }
        assert!(
            changed.authenticate(&node.endpoint, hash, &keys()).is_err(),
            "corrupt consensus evidence {damage}"
        );
    }
    for method in [
        "pax_sequencerReceipt",
        "eth_getTransactionReceipt",
        "px_getCommit",
        "px_getValidators",
        "eth_getBlockByNumber",
    ] {
        assert!(
            node.calls
                .lock()
                .or_fail("calls")
                .iter()
                .any(|call| call["method"] == method),
            "real receipt path must call {method}"
        );
    }
    for text in [
        "**Committed**",
        "**Submitted**",
        "**Receipted**",
        "BankCommitEvidence::authenticate",
        "BankCommittedReceipt",
        "trusted validator quorum",
        "transaction inclusion",
    ] {
        assert!(
            README.contains(text),
            "README must distinguish submission from finality: {text}"
        );
    }
}

#[test]
fn doc_13_settings_targets_and_retirement_match_production() {
    let schema = ApiSchema::v1().or_fail("real executable operation inventory");
    for name in [
        "binding.statement",
        "binding.submit",
        "binding.status",
        "binding.rebind.action",
        "binding.rebind",
        "deposit.start",
        "deposit.confirm",
        "withdraw.start",
        "withdraw.claim",
        "exit.start",
        "exit.eligibility",
    ] {
        assert!(schema
            .operation(name)
            .or_fail("retained compatibility operation")
            .retired());
    }
    for name in ["move.quote", "move.commit"] {
        let operation = schema
            .operation(name)
            .or_fail("active bank transfer operation");
        assert!(!operation.retired());
        assert!(README.contains(&format!("{} {}", operation.method, operation.path)));
    }
    assert!(README.contains("410 `operation-retired`"));
    assert_eq!(Listener::parse(None, None, None), Ok(Listener::Tls));
    assert_eq!(
        Listener::parse(Some("plain"), None, None),
        Ok(Listener::Plain)
    );
    assert!(Listener::parse(Some("plain"), Some("certificate.der"), None).is_err());
    let (rp_id, origins) = web_identity(|_| None);
    assert!(README.contains(&format!("`{rp_id}`")));
    assert!(README.contains(&format!("`{origins}`")));
    let production = include_str!("../src/server/production_components.rs");
    for name in [
        "LAYERX_HUMAN_STORE_ROOT",
        "LAYERX_HUMAN_CUSTODY_ROOT",
        "LAYERX_HUMAN_AUTH_INDEX_ROOT",
        "LAYERX_HUMAN_TENANCY_DIGEST",
        "LAYERX_HUMAN_AUTH_INDEX_KEY",
        "LAYERX_HUMAN_STREAM_CURSOR_KEY",
        "LAYERX_HUMAN_AGENT_SOCKET",
        "LAYERX_HUMAN_IDENTITY_SOCKET",
        "LAYERX_HUMAN_SECURITY_SOCKET",
        "LAYERX_HUMAN_NETWORK_ID",
        "LAYERX_HUMAN_PROTOCOL_VERSION",
        "LAYERX_HUMAN_EVM_GAS_LIMIT",
        "LAYERX_HUMAN_EVM_MAX_FEE_PER_GAS",
        "LAYERX_HUMAN_EVM_MAX_PRIORITY_FEE_PER_GAS",
        "LAYERX_HUMAN_KMS_ENDPOINT",
        "LAYERX_HUMAN_KMS_PROVIDER_REFERENCE",
        "LAYERX_HUMAN_KMS_SERVER_NAME",
        "LAYERX_HUMAN_KMS_ROOT_CERTIFICATE_DER",
        "LAYERX_HUMAN_KMS_CLIENT_CERTIFICATE_DER",
        "LAYERX_HUMAN_KMS_CLIENT_PRIVATE_KEY_DER",
        "LAYERX_HUMAN_KMS_MAX_FRAME_BYTES",
        "LAYERX_HUMAN_KMS_MAX_CONNECTIONS",
        "LAYERX_HUMAN_KMS_MAX_STREAMS",
        "LAYERX_HUMAN_KMS_MAX_QUEUED_BYTES",
        "LAYERX_HUMAN_KMS_DEADLINE_SECONDS",
    ] {
        assert!(
            production.contains(&format!("\"{name}\"")),
            "production setting exists: {name}"
        );
        assert!(
            README.contains(&format!("`{name}`")),
            "document the real setting: {name}"
        );
    }
    for (target, source) in [
        ("repair_22_33_45", include_str!("repair_22_33_45.rs")),
        (
            "agent_runtime_eip1559",
            include_str!("agent_runtime_eip1559.rs"),
        ),
        (
            "agent_runtime_nonces",
            include_str!("agent_runtime_nonces.rs"),
        ),
    ] {
        assert!(
            source.contains("#[test]"),
            "documented target has executable cases"
        );
        assert!(README.contains(&format!("cargo test --locked --manifest-path services/human/crates/layerx-human-service/Cargo.toml --test {target}")));
    }
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
