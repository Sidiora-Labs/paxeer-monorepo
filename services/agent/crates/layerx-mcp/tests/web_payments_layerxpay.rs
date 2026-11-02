use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::TcpListener;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};

use ed25519_dalek::{Signer as _, SigningKey};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use layerx_agentd::prepare::evm::PrecompileTarget;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_crypto::payments::Grant;
use layerx_mcp::binding::Binding;
use layerx_mcp::catalogue::WEB_TOOLS;
use layerx_mcp::tools::web::{validate_web_challenge, GrantTerms, WebCall, WebConfig};
use layerx_x402::model::{account_identifiers, PaymentRequired, PaymentRequirements};
use serde_json::{json, Value};

fn config(endpoint: &str) -> WebConfig {
    WebConfig::new(
        endpoint,
        "did:layerx:payer",
        "0xabababababababababababababababababababab",
        [0xcd; 20],
        "eip155:125",
        [0x5e; 32],
        [0x2a; 32],
        1,
        100,
        Duration::from_secs(5),
        3,
    )
    .expect("valid web configuration")
}

fn exchange() -> Value {
    serde_json::from_str(include_str!(
        "../../../../../services/interop/crates/x-websearch/tests/fixtures/client-exchange.json"
    ))
    .expect("recorded seller exchange")
}

fn quote() -> PaymentRequirements {
    let mut offer = exchange()["exchange"][0]["response"]["headers"]["PAYMENT-REQUIRED"]["accepts"]
        .as_array()
        .expect("recorded offers")
        .iter()
        .find(|offer| offer["scheme"] == "exact" && offer["extra"]["layerx"]["currency"] == "USDC")
        .expect("recorded USDC exact offer")
        .clone();
    offer["amount"] = json!("2");
    offer["maxTimeoutSeconds"] = json!(30);
    assert!(offer["extra"]["layerx"].get("receiverDid").is_none());
    serde_json::from_value(offer).expect("exact quote")
}

struct RecordedLedger {
    endpoint: String,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}

impl RecordedLedger {
    fn start() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("replay listener");
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let endpoint = format!("http://{}", listener.local_addr().expect("replay address"));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let thread = thread::spawn(move || {
            let gateway: Value = serde_json::from_str(include_str!(
                "../../../../../services/interop/crates/x-websearch/tests/fixtures/gateway/metered.json"
            )).expect("recorded ledger answers");
            while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("replay accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("read timeout");
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                reader.read_line(&mut line).expect("request line");
                assert_eq!(line, "POST /rpc HTTP/1.1\r\n");
                let mut length = None;
                loop {
                    line.clear();
                    assert!(reader.read_line(&mut line).expect("header") > 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().expect("content length"));
                    }
                }
                let mut body = vec![0; length.expect("request length")];
                reader.read_exact(&mut body).expect("request body");
                let request: Value = serde_json::from_slice(&body).expect("accountId request");
                assert_eq!(request["jsonrpc"], "2.0");
                assert_eq!(request["method"], "eth_call");
                let data = request["params"][0]["data"].as_str().expect("call data");
                let prefix = format!("0x3f050cbc{}{}", "00".repeat(12), "ab".repeat(20));
                let asset = data
                    .strip_prefix(&prefix)
                    .expect("accountId owner and selector");
                assert_eq!(asset.len(), 64);
                assert_eq!(
                    request["params"],
                    json!([{
                    "to": "0x000000000000000000000000000000000000101a", "data": data
                }, "pending"])
                );
                let grant = gateway["ledger"]["grants"]
                    .as_array()
                    .expect("recorded grants")
                    .iter()
                    .find(|grant| {
                        grant["canonical"]
                            .as_str()
                            .expect("canonical grant")
                            .get(192..256)
                            == Some(asset)
                    })
                    .expect("recorded asset account");
                let canonical = grant["canonical"].as_str().expect("canonical grant");
                let body = json!({"jsonrpc": "2.0", "id": request["id"],
                    "result": format!("0x{}", &canonical[64..128])})
                .to_string();
                write!(
                    reader.get_mut(),
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                )
                .expect("recorded response");
            }
        });
        Self {
            endpoint,
            stop,
            thread: Some(thread),
        }
    }
}

impl Drop for RecordedLedger {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.thread
            .take()
            .expect("replay thread")
            .join()
            .expect("recorded ledger replay");
    }
}

#[test]
fn web_fetch_pays_with_pay_exact() {
    let call = WebCall::from_arguments(
        "web.fetch",
        &json!({
            "url": "https://paxeer.app/data.json", "currency": "USDC", "scheme": "exact",
            "idempotency_key": "09".repeat(32),
        }),
    )
    .expect("web.fetch invocation");
    let replay = RecordedLedger::start();
    let config = config(&replay.endpoint);
    let required: PaymentRequired = serde_json::from_value(
        exchange()["exchange"][0]["response"]["headers"]["PAYMENT-REQUIRED"].clone(),
    )
    .expect("recorded search challenge");
    validate_web_challenge(&required).expect("32-byte ledger challenge without receiverDid");
    for offers in required.accepts.chunks_exact(2) {
        let metered = &offers[0];
        let exact = &offers[1];
        let currency = exact.extra.as_ref().expect("profile")["layerx"]["currency"]
            .as_str()
            .expect("currency");
        let call = WebCall::from_arguments(
            "web.fetch",
            &json!({
                "url": "https://paxeer.app/data.json", "currency": currency,
                "scheme": "exact", "idempotency_key": "09".repeat(32),
            }),
        )
        .expect("exact call");
        let terms = call
            .exact_terms(&config, exact, 7)
            .expect("recorded ledger offer");
        assert!(account_identifiers(&terms.recipient_account)
            .expect("recipient ids")
            .contains(&terms.recipient));
        let payer = terms
            .payer_account
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>();
        assert_eq!(
            payer,
            metered.extra.as_ref().expect("metered profile")["layerx"]["payer"]
                .as_str()
                .expect("payer id")
        );
    }
    let mut quote = quote();
    let terms = call
        .exact_terms(&config, &quote, 7)
        .expect("bind exact web price");
    let prepared = terms.prepare_pay_exact();
    let data = prepared.encode().expect("payExact ABI encoding");
    assert_eq!(prepared.target, PrecompileTarget::Ledger);
    assert_eq!(
        prepared.target.address(),
        [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a]
    );
    assert_eq!(
        &data[..4],
        &transaction_hash(b"payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")[..4]
    );
    assert_eq!(data.len(), 4 + 6 * 32);
    for (index, word) in [
        (0, terms.payer_account),
        (1, terms.recipient),
        (2, terms.asset),
    ] {
        assert_eq!(&data[4 + index * 32..4 + (index + 1) * 32], &word);
        assert_ne!(
            &word[..12],
            &[0; 12],
            "ledger ids retain their leading bytes"
        );
    }
    assert_eq!(&data[100..116], &[0; 16]);
    assert_eq!(&data[116..132], &2_u128.to_be_bytes());
    assert_eq!(&data[132..156], &[0; 24]);
    assert_eq!(&data[156..164], &terms.expires_at_ms.to_be_bytes());
    assert_eq!(&data[164..196], &[9; 32]);
    assert_eq!(terms.amount, 2);
    assert!(account_identifiers(&terms.recipient_account)
        .expect("account ids")
        .contains(&terms.recipient));
    let expected_payer = exchange()["exchange"][0]["response"]["headers"]["PAYMENT-REQUIRED"]
        ["accepts"][4]["extra"]["layerx"]["payer"]
        .as_str()
        .expect("recorded USDC payer")
        .to_owned();
    assert_eq!(
        terms
            .payer_account
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>(),
        expected_payer
    );
    let evm = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 3,
        gas_limit: 100_000,
        to: prepared.target.address(),
        data: data.clone(),
    };
    let signed = evm.sign(&[0xa5; 32]).expect("sign real EIP-1559 call");
    let (decoded, signature) = decode_signed(&signed).expect("decode signed call");
    assert_eq!(decoded, evm);
    assert_ne!(
        recover_sender(&decoded, &signature).expect("recover EVM signer"),
        [0; 20]
    );
    quote.pay_to = format!("0x{}", "aa".repeat(32));
    assert!(
        call.exact_terms(&config, &quote, 7).is_err(),
        "recipient substitution must fail"
    );
    quote.pay_to = format!("0x{}", "cd".repeat(20));
    assert!(
        call.exact_terms(&config, &quote, 7).is_err(),
        "20-byte addresses cannot name ledger accounts"
    );

    let grant = GrantTerms {
        payer_did: terms.payer_did,
        payer_account: terms.payer_account,
        recipient: terms.recipient,
        asset: terms.asset,
        expires_at_ms: 1_900_000_000_000,
        amount: 2,
        purpose_hash: [4; 32],
        idempotency_key: [9; 32],
    };
    let signer = SigningKey::from_bytes(&[0x33; 32]);
    let mut signed_grant = Grant {
        id: [0; 32],
        from: grant.payer_account,
        recipient: grant.recipient,
        asset: grant.asset,
        per_draw_maximum: 2,
        allowance: 2,
        recurring: false,
        window_length: 0,
        expiration: grant.expires_at_ms,
        purpose_hash: [4; 32],
        has_reference: true,
        reference_hash: [9; 32],
        revocation_sequence: 0,
        public_key: signer.verifying_key().to_bytes(),
        signature: [0; 64],
    };
    signed_grant.id = signed_grant.signing_digest().expect("grant digest");
    signed_grant.signature = signer.sign(&signed_grant.id).to_bytes();
    let issued = grant
        .prepare_grant(&signed_grant)
        .expect("authenticated grant");
    assert_eq!(issued.target, PrecompileTarget::Ledger);
    let grant_data = issued.encode().expect("issueGrant calldata");
    assert_eq!(&grant_data[..4], &[0xd0, 0xd5, 0x34, 0x80]);
    assert_eq!(grant_data.len(), 4 + 2 * 32 + 352);
    assert_eq!(&grant_data[68..100], &signed_grant.id);
    assert_eq!(&grant_data[100..132], &grant.payer_account);
    assert_eq!(&grant_data[132..164], &grant.recipient);
    assert_eq!(&grant_data[164..196], &grant.asset);
    assert_eq!(&grant_data[237..245], &grant.expires_at_ms.to_be_bytes());
    let mut altered = signed_grant.clone();
    altered.expiration += 1;
    assert!(grant.prepare_grant(&altered).is_err());
    altered = signed_grant;
    altered.signature[0] ^= 1;
    assert!(grant.prepare_grant(&altered).is_err());
    let sequence_call = grant.next_sequence_call();
    assert_eq!(sequence_call.target, PrecompileTarget::Ledger);
    let sequence_data = sequence_call.encode().expect("nextSequence calldata");
    assert_eq!(&sequence_data[..4], &[0xce, 0x2f, 0x92, 0x1f]);
    assert_eq!(&sequence_data[4..36], &grant.recipient);
    assert_eq!(&sequence_data[36..68], &grant.asset);
    let draw = grant.prepare_draw([8; 32], 19);
    assert_eq!(draw.target, PrecompileTarget::Ledger);
    let draw_data = draw.encode().expect("draw calldata");
    assert_eq!(
        &draw_data[..4],
        &transaction_hash(b"draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)")[..4]
    );
    assert_eq!(&draw_data[4..36], &[8; 32]);
    assert_eq!(&draw_data[36..68], &grant.recipient);
    assert_eq!(&draw_data[84..100], &2_u128.to_be_bytes());
    assert_eq!(&draw_data[132..156], &[0; 24]);
    assert_eq!(&draw_data[156..164], &19_u64.to_be_bytes());
    for tool in WEB_TOOLS {
        assert_eq!(
            tool.required_scope,
            format!("write:{}", tool.name.replace('.', ":"))
        );
        assert!(tool.mutation.contains("payExact"));
    }
}

#[test]
fn web_fetch_pay_exact_expiry_is_the_disclosure_expiry_in_milliseconds() {
    let call = WebCall::from_arguments(
        "web.fetch",
        &json!({
            "url": "https://paxeer.app/data.json", "currency": "USDC", "scheme": "exact",
            "idempotency_key": "09".repeat(32),
        }),
    )
    .expect("web.fetch invocation");
    let replay = RecordedLedger::start();
    let config = config(&replay.endpoint);
    for timeout in [30_u64, 900] {
        let mut quote = quote();
        quote.max_timeout_seconds = timeout.try_into().expect("timeout");
        let before = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("unix clock")
            .as_secs();
        let terms = call
            .exact_terms(&config, &quote, 7)
            .expect("bind exact web price");
        let after = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("unix clock")
            .as_secs();
        let window = timeout.min(layerx_mcp::tools::web::APPROVAL_WINDOW);
        assert!((before + window..=after + window).contains(&terms.disclosure.expiry.0));
        let data = terms
            .prepare_pay_exact()
            .encode()
            .expect("payExact calldata");
        let expires_at_ms = u64::from_be_bytes(data[156..164].try_into().expect("expiry word"));
        assert_eq!(expires_at_ms, terms.disclosure.expiry.0 * 1_000);
        assert_eq!(expires_at_ms, terms.expires_at_ms);
        assert!(
            expires_at_ms > 1_790_000_000_000,
            "expiry must exceed a realistic block time in milliseconds"
        );
        assert!(expires_at_ms > after * 1_000);
        assert_ne!(expires_at_ms, 7);
        assert_ne!(expires_at_ms, (7 + window) * 1_000);
    }
}

fn document() -> Value {
    json!({
        "mode": "full", "tenant": "tenant-a", "store": "/tmp/web-store", "audit_root": "/tmp/web-audit",
        "session_id": "07".repeat(32), "session_token_file": "/tmp/web-session-token", "session_generation": 1,
        "capability_id": "09".repeat(32), "core_sequence": 7, "deadline_ms": 5000,
        "agent": {"endpoint": "http://127.0.0.1:9500", "bearer_file": "/tmp/web-agent-bearer", "probe_program": "55".repeat(32)},
        "limit": {"id": "09".repeat(16), "name": "mcp-limit", "scope": "tenant", "scope_id": "01".repeat(32), "ceiling": "1000", "consumed": "0"},
        "web": {"endpoint": "http://127.0.0.1:9450", "payer_address": format!("0x{}", "ab".repeat(20)),
            "receiver_address": format!("0x{}", "cd".repeat(20)), "network": "eip155:125",
            "receipt_key_index": [{"keyId": "2a".repeat(32), "publicKey": "5e".repeat(32), "firstHeight": 1, "lastHeight": 100}],
            "sequencer_id": "2a".repeat(32), "first_batch_number": 1, "last_batch_number": 100,
            "timeout_ms": 5000, "pending_attempts": 3, "approval_threshold": "1000"}
    })
}

#[test]
fn web_binding_names_a_receipt_key_index() {
    let valid = document();
    assert!(
        Binding::parse(&valid.to_string()).is_ok(),
        "receipt key index binds"
    );
    let mut missing = valid.clone();
    missing["web"]
        .as_object_mut()
        .expect("web block")
        .remove("receipt_key_index");
    let error = Binding::parse(&missing.to_string()).expect_err("index is required");
    assert!(error.detail().contains("receipt_key_index"));
    let mut retired = missing.clone();
    retired["web"]["sequencer_public_key"] = json!("5e".repeat(32));
    let error = Binding::parse(&retired.to_string()).expect_err("retired key is refused");
    assert!(error.detail().contains("sequencer_public_key"));
    retired["web"]["receipt_key_index"] = valid["web"]["receipt_key_index"].clone();
    assert!(
        Binding::parse(&retired.to_string()).is_err(),
        "retired key is refused alongside an index"
    );
    for index in [
        json!([]),
        json!("5e".repeat(32)),
        json!([{"keyId": "00".repeat(32), "publicKey": "5e".repeat(32), "firstHeight": 1, "lastHeight": 100}]),
    ] {
        let mut invalid = valid.clone();
        invalid["web"]["receipt_key_index"] = index;
        assert!(
            Binding::parse(&invalid.to_string()).is_err(),
            "malformed key index refused"
        );
    }
    let mut uncovered = valid;
    uncovered["web"]["receipt_key_index"][0]["lastHeight"] = json!(6);
    assert!(
        Binding::parse(&uncovered.to_string()).is_err(),
        "key must cover the configured heights"
    );
}
