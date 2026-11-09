use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

use layerx_mcp_test_support::test_clock;

use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::TcpListener;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};

use ed25519_dalek::{Signer as _, SigningKey};
use std::time::Duration;

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
    result_assertion::required(
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
        ),
        "valid web configuration",
    )
}

fn exchange() -> Value {
    result_assertion::required(
        serde_json::from_str(include_str!(
        "../../../../../services/interop/crates/x-websearch/tests/fixtures/client-exchange.json"
    )),
        "recorded seller exchange",
    )
}

fn quote() -> PaymentRequirements {
    let mut offer = option_assertion::required(
        option_assertion::required(
            exchange()["exchange"][0]["response"]["headers"]["PAYMENT-REQUIRED"]["accepts"]
                .as_array(),
            "recorded offers",
        )
        .iter()
        .find(|offer| offer["scheme"] == "exact" && offer["extra"]["layerx"]["currency"] == "USDC"),
        "recorded USDC exact offer",
    )
    .clone();
    offer["amount"] = json!("2");
    offer["maxTimeoutSeconds"] = json!(30);
    assert!(offer["extra"]["layerx"].get("receiverDid").is_none());
    result_assertion::required(serde_json::from_value(offer), "exact quote")
}

struct RecordedLedger {
    endpoint: String,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}

impl RecordedLedger {
    fn start() -> Self {
        let listener =
            result_assertion::required(TcpListener::bind("127.0.0.1:0"), "replay listener");
        result_assertion::required(listener.set_nonblocking(true), "nonblocking listener");
        let endpoint = format!(
            "http://{}",
            result_assertion::required(listener.local_addr(), "replay address")
        );
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let thread = thread::spawn(move || {
            let gateway: Value = result_assertion::required(serde_json::from_str(include_str!(
                "../../../../../services/interop/crates/x-websearch/tests/fixtures/gateway/metered.json"
            )), "recorded ledger answers");
            while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("replay accept: {error}"),
                };
                result_assertion::required(
                    stream.set_read_timeout(Some(Duration::from_secs(5))),
                    "read timeout",
                );
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                result_assertion::required(reader.read_line(&mut line), "request line");
                assert_eq!(line, "POST / HTTP/1.1\r\n");
                let mut length = None;
                loop {
                    line.clear();
                    assert!(result_assertion::required(reader.read_line(&mut line), "header") > 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(result_assertion::required(
                            value.trim().parse::<usize>(),
                            "content length",
                        ));
                    }
                }
                let mut body = vec![0; option_assertion::required(length, "request length")];
                result_assertion::required(reader.read_exact(&mut body), "request body");
                let request: Value =
                    result_assertion::required(serde_json::from_slice(&body), "accountId request");
                assert_eq!(request["jsonrpc"], "2.0");
                assert_eq!(request["method"], "eth_call");
                let data =
                    option_assertion::required(request["params"][0]["data"].as_str(), "call data");
                let prefix = format!("0x3f050cbc{}{}", "00".repeat(12), "ab".repeat(20));
                let asset = option_assertion::required(
                    data.strip_prefix(&prefix),
                    "accountId owner and selector",
                );
                assert_eq!(asset.len(), 64);
                assert_eq!(
                    request["params"],
                    json!([{
                    "to": "0x000000000000000000000000000000000000101a", "data": data
                }, "pending"])
                );
                let grant = option_assertion::required(
                    option_assertion::required(
                        gateway["ledger"]["grants"].as_array(),
                        "recorded grants",
                    )
                    .iter()
                    .find(|grant| {
                        option_assertion::required(grant["canonical"].as_str(), "canonical grant")
                            .get(192..256)
                            == Some(asset)
                    }),
                    "recorded asset account",
                );
                let canonical =
                    option_assertion::required(grant["canonical"].as_str(), "canonical grant");
                let body = json!({"jsonrpc": "2.0", "id": request["id"],
                    "result": format!("0x{}", &canonical[64..128])})
                .to_string();
                result_assertion::required(
                    write!(
                        reader.get_mut(),
                        "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                        body.len()
                    ),
                    "recorded response",
                );
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
        result_assertion::required(
            option_assertion::required(self.thread.take(), "replay thread").join(),
            "recorded ledger replay",
        );
    }
}

#[test]
fn web_fetch_pays_with_pay_exact() {
    let call = result_assertion::required(
        WebCall::from_arguments(
            "web.fetch",
            &json!({
                "url": "https://paxeer.app/data.json", "currency": "USDC", "scheme": "exact",
                "idempotency_key": "09".repeat(32),
            }),
        ),
        "web.fetch invocation",
    );
    let mut replay = RecordedLedger::start();
    let config = config(&replay.endpoint);
    assert_exact_offer_accounts(&config, &mut replay);
    let mut quote = quote();
    let terms = result_assertion::required(
        call.exact_terms_with_payer(
            &config,
            &quote,
            &mut replay,
            &test_clock::TestClock::new(50),
        ),
        "bind exact web price",
    );
    assert_pay_exact_abi(&terms);
    quote.pay_to = format!("0x{}", "aa".repeat(32));
    assert!(
        call.exact_terms_with_payer(
            &config,
            &quote,
            &mut replay,
            &test_clock::TestClock::new(50)
        )
        .is_err(),
        "recipient substitution must fail"
    );
    quote.pay_to = format!("0x{}", "cd".repeat(20));
    assert!(
        call.exact_terms_with_payer(
            &config,
            &quote,
            &mut replay,
            &test_clock::TestClock::new(50)
        )
        .is_err(),
        "20-byte addresses cannot name ledger accounts"
    );

    assert_metered_grant_calls(terms);
}

#[test]
fn web_fetch_pay_exact_expiry_is_the_disclosure_expiry_in_milliseconds() {
    let call = result_assertion::required(
        WebCall::from_arguments(
            "web.fetch",
            &json!({
                "url": "https://paxeer.app/data.json", "currency": "USDC", "scheme": "exact",
                "idempotency_key": "09".repeat(32),
            }),
        ),
        "web.fetch invocation",
    );
    let mut replay = RecordedLedger::start();
    let config = config(&replay.endpoint);
    let clock = test_clock::TestClock::new(1_800_000_000);
    for timeout in [30_u64, 900] {
        let mut quote = quote();
        quote.max_timeout_seconds = result_assertion::required(timeout.try_into(), "timeout");
        let before = clock.now();
        let terms = result_assertion::required(
            call.exact_terms_with_payer(&config, &quote, &mut replay, &clock),
            "bind exact web price",
        );
        let after = clock.now();
        let window = timeout.min(layerx_mcp::tools::web::APPROVAL_WINDOW);
        assert!((before + window..=after + window).contains(&terms.disclosure.expiry.0));
        let data =
            result_assertion::required(terms.prepare_pay_exact().encode(), "payExact calldata");
        let expires_at_ms = u64::from_be_bytes(result_assertion::required(
            data[156..164].try_into(),
            "expiry word",
        ));
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

use layerx_mcp_test_support::payment_binding;

fn document() -> Value {
    payment_binding::document()
}

#[test]
fn web_binding_names_a_receipt_key_index() {
    let valid = document();
    assert!(
        Binding::parse(&valid.to_string()).is_ok(),
        "receipt key index binds"
    );
    let mut missing = valid.clone();
    option_assertion::required(missing["web"].as_object_mut(), "web block")
        .remove("receipt_key_index");
    let error = layerx_mcp_test_support::refusal_assertion::refused(
        Binding::parse(&missing.to_string()),
        "index is required",
    );
    assert!(error.detail().contains("receipt_key_index"));
    let mut retired = missing.clone();
    retired["web"]["sequencer_public_key"] = json!("5e".repeat(32));
    let error = layerx_mcp_test_support::refusal_assertion::refused(
        Binding::parse(&retired.to_string()),
        "retired key is refused",
    );
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

impl layerx_mcp::tools::web::WebPayer for RecordedLedger {
    fn view(
        &mut self,
        call: &layerx_agentd::prepare::evm::PrecompileCall,
    ) -> Result<Vec<u8>, layerx_mcp::tools::web::WebPayerError> {
        use layerx_mcp::tools::web::WebPayerError;
        let node = layerx_client::sequencer::SequencerEndpoint::new(&self.endpoint, &[])
            .map_err(|_| WebPayerError::Unavailable)?;
        node.call_pending(
            &call.target.address(),
            &call.encode().map_err(|_| WebPayerError::Refused)?,
        )
        .map_err(|_| WebPayerError::Unavailable)
    }
    fn sign_grant(
        &mut self,
        _: &GrantTerms,
    ) -> Result<Grant, layerx_mcp::tools::web::WebPayerError> {
        panic!("quote-only ledger must not sign")
    }
    fn submit(
        &mut self,
        _: &layerx_agentd::prepare::evm::PrecompileCall,
    ) -> Result<layerx_mcp::tools::web::ExactPayment, layerx_mcp::tools::web::WebPayerError> {
        panic!("quote-only ledger must not submit")
    }
}

use layerx_mcp_test_support::seller_exchange;

#[test]
fn payment_receipts_round_trip_through_seller_router_and_node_root() {
    seller_exchange::seller_routes_and_node_transport_complete_exact_and_metered();
    seller_exchange::node_account_failures_never_reach_settlement();
}

fn assert_exact_offer_accounts(config: &WebConfig, replay: &mut RecordedLedger) {
    let required: PaymentRequired = result_assertion::required(
        serde_json::from_value(
            exchange()["exchange"][0]["response"]["headers"]["PAYMENT-REQUIRED"].clone(),
        ),
        "recorded search challenge",
    );
    result_assertion::required(
        validate_web_challenge(&required),
        "32-byte ledger challenge without receiverDid",
    );
    for offers in required.accepts.chunks_exact(2) {
        let metered = &offers[0];
        let exact = &offers[1];
        let currency = option_assertion::required(
            option_assertion::required(exact.extra.as_ref(), "profile")["layerx"]["currency"]
                .as_str(),
            "currency",
        );
        let call = result_assertion::required(
            WebCall::from_arguments(
                "web.fetch",
                &json!({
                    "url": "https://paxeer.app/data.json", "currency": currency,
                    "scheme": "exact", "idempotency_key": "09".repeat(32),
                }),
            ),
            "exact call",
        );
        let terms = result_assertion::required(
            call.exact_terms_with_payer(config, exact, replay, &test_clock::TestClock::new(50)),
            "recorded ledger offer",
        );
        assert!(result_assertion::required(
            account_identifiers(&terms.recipient_account),
            "recipient ids"
        )
        .contains(&terms.recipient));
        let payer = layerx_programs::hex::encode(&terms.payer_account);
        assert_eq!(
            payer,
            option_assertion::required(
                option_assertion::required(metered.extra.as_ref(), "metered profile")["layerx"]
                    ["payer"]
                    .as_str(),
                "payer id"
            )
        );
    }
}

fn assert_pay_exact_abi(terms: &layerx_mcp::tools::web::ExactTerms) {
    let prepared = terms.prepare_pay_exact();
    let data = result_assertion::required(prepared.encode(), "payExact ABI encoding");
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
    assert!(result_assertion::required(
        account_identifiers(&terms.recipient_account),
        "account ids"
    )
    .contains(&terms.recipient));
    let expected_payer = option_assertion::required(
        exchange()["exchange"][0]["response"]["headers"]["PAYMENT-REQUIRED"]["accepts"][4]["extra"]
            ["layerx"]["payer"]
            .as_str(),
        "recorded USDC payer",
    )
    .to_owned();
    assert_eq!(
        layerx_programs::hex::encode(&terms.payer_account),
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
    let signed = result_assertion::required(evm.sign(&[0xa5; 32]), "sign real EIP-1559 call");
    let (decoded, signature) =
        result_assertion::required(decode_signed(&signed), "decode signed call");
    assert_eq!(decoded, evm);
    assert_ne!(
        result_assertion::required(recover_sender(&decoded, &signature), "recover EVM signer"),
        [0; 20]
    );
}

fn assert_metered_grant_calls(terms: layerx_mcp::tools::web::ExactTerms) {
    let receiver_did_key = decode_receiver_did_key(&terms.recipient_account);
    let grant = GrantTerms {
        payer_did: terms.payer_did,
        payer_account: terms.payer_account,
        recipient: terms.recipient,
        receiver_did_key,
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
    signed_grant.id = result_assertion::required(signed_grant.signing_digest(), "grant digest");
    signed_grant.signature = signer.sign(&signed_grant.id).to_bytes();
    let issued =
        result_assertion::required(grant.prepare_grant(&signed_grant), "authenticated grant");
    assert_eq!(issued.target, PrecompileTarget::Ledger);
    let grant_data = result_assertion::required(issued.encode(), "issueGrant calldata");
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
    let sequence_data = result_assertion::required(sequence_call.encode(), "nextSequence calldata");
    assert_eq!(&sequence_data[..4], &[0xce, 0x2f, 0x92, 0x1f]);
    assert_eq!(&sequence_data[4..36], &grant.recipient);
    assert_ne!(grant.receiver_did_key, grant.recipient);
    assert_eq!(&sequence_data[36..68], &grant.asset);
    let draw = grant.prepare_draw([8; 32], 19);
    assert_eq!(draw.target, PrecompileTarget::Ledger);
    let draw_data = result_assertion::required(draw.encode(), "draw calldata");
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

fn decode_receiver_did_key(account: &str) -> [u8; 32] {
    result_assertion::required(
        option_assertion::required(
            option_assertion::required(
                account.strip_prefix("agent:did:layerx:"),
                "receiver DID account",
            )
            .split(':')
            .next(),
            "receiver DID key",
        )
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            result_assertion::required(
                u8::from_str_radix(
                    result_assertion::required(std::str::from_utf8(pair), "hex pair"),
                    16,
                ),
                "receiver DID hex",
            )
        })
        .collect::<Vec<_>>()
        .try_into(),
        "32-byte receiver DID key",
    )
}
