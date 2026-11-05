//! Task 22.31.162, finding failed-sponsor:
//! node_revert_vector_preserves_evidence proves the committed producer vector;
//! failed_sponsor_survives_submit_status_and_reopen proves submit, status and
//! disk-journal recovery over bounded loopback HTTPS, including foreign-sponsor
//! rejection. No external endpoint, credential, toolchain or native build is used.
//! Baseline: the supplied attempt failed with Malformed and exceeded 60 seconds.
//! No additional pre-change gate is claimed; the ordered gates run after implementation.
//! Wiring requests: none; producer and consumer paths are both in task scope.

use std::fs;
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::fs::DirBuilderExt as _;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::JoinHandle;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_gas_station::config::{ServiceConfig, StationConfig};
use layerx_gas_station::journal::{Entry, Journal, Key, QuoteRecord};
use layerx_gas_station::price::PaymasterRateSource;
use layerx_gas_station::quote::{
    address_word, keccak, quote_digest, word, Address, Quote, Word, SIDIORA,
};
use layerx_gas_station::rpc::{
    bytes, decode_full_sequencer_receipt, hex, quantity, ConfiguredRpc, Exchange, Execution,
    HttpsExchange, RpcFault, Rung, SequencerEndpoint,
};
use layerx_gas_station::service::{Limits, Service, ServiceError};
use layerx_gas_station::signer::{QuoteSigner, SignerError};
use layerx_gas_station::station::{prepare_submission, GasStation};
use layerx_gas_station::tx::{
    authorization_digest, batch_digest, encode_sponsored, Authorization, Call, Fees,
    TransactionRequest,
};
use layerx_gas_station::SignedQuote;
use layerx_platform_internal::evm_sender::EvmSender;
use serde_json::{json, Value};

type TestResult<T = ()> = Result<T, Box<dyn std::error::Error>>;
const CHAIN_ID: u64 = 713_714;
const NOW: u64 = 1_750_000_400;
const RPC_BUDGET: Duration = Duration::from_secs(2);

fn fixed<const N: usize>(value: &Value) -> TestResult<[u8; N]> {
    Ok(bytes(value.as_str().ok_or("missing hexadecimal field")?)?
        .try_into()
        .map_err(|_| "incorrect field length")?)
}

#[test]
fn node_revert_vector_preserves_evidence() -> TestResult {
    let vector: Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))?;
    let cases = vector["cases"].as_array().ok_or("missing producer cases")?;
    let case = cases
        .iter()
        .find(|case| case["id"] == "V2-layerxledger-revert")
        .ok_or("missing genuine reverted node receipt")?;
    let answer = &case["receiptJson"];
    let hash: Word = fixed(&answer["txHash"])?;
    let receipt = decode_full_sequencer_receipt(answer, &hash)?;
    assert_eq!(answer["sponsor"], hex(&[0; 20]));
    assert_eq!(receipt.sponsor, None);
    assert!(matches!(
        receipt.rung,
        Rung::Receipted(Execution {
            succeeded: false,
            sponsor: None,
            ..
        })
    ));
    assert_eq!(receipt.status, Some(0));
    assert_eq!(receipt.vm_error.as_deref(), Some("execution reverted"));
    assert_eq!(
        receipt.receipt,
        bytes(
            case["signedReceipt"]
                .as_str()
                .ok_or("missing signed receipt")?
        )?
    );
    assert_eq!(
        receipt.return_data,
        bytes(answer["returnData"].as_str().ok_or("missing revert data")?)?
    );

    // Invalid address evidence must not acquire the zero-address meaning.
    for bad in [json!("0x00"), json!("0xzz"), Value::Null] {
        let mut invalid = answer.clone();
        invalid["sponsor"] = bad;
        assert_eq!(
            decode_full_sequencer_receipt(&invalid, &hash),
            Err(RpcFault::Malformed)
        );
    }
    for sponsor in [[0; 20], [0x77; 20]] {
        let mut successful = cases[0]["receiptJson"].clone();
        successful["sponsor"] = json!(hex(&sponsor));
        let decoded = decode_full_sequencer_receipt(&successful, &fixed(&successful["txHash"])?)?;
        assert_eq!(decoded.sponsor, Some(sponsor));
        assert!(matches!(decoded.rung, Rung::Receipted(Execution {
            succeeded: true, sponsor: Some(address), ..
        }) if address == sponsor));
    }
    Ok(())
}

// Use the production signer without loading credentials from the environment.
struct Signer(EvmSender);
impl Signer {
    fn new(secret: u8) -> TestResult<Self> {
        Ok(Self(EvmSender::from_hex_secret(&format!("{secret:064x}"))?))
    }
}
impl QuoteSigner for Signer {
    fn address(&self) -> Address {
        self.0.address()
    }
    fn sign_digest(&self, digest: Word) -> Result<[u8; 65], SignerError> {
        self.0.sign_prehash(&digest).map_err(Into::into)
    }
}

struct TestRuntime {
    directory: PathBuf,
    previous_ca: Option<std::ffi::OsString>,
    previous_budget: Option<std::ffi::OsString>,
}
impl TestRuntime {
    fn new(certificate: &str) -> TestResult<Self> {
        let directory = std::env::temp_dir().join(format!(
            "repair-22-31-162-{}-{}",
            std::process::id(),
            SystemTime::now().duration_since(UNIX_EPOCH)?.as_nanos()
        ));
        fs::DirBuilder::new().mode(0o700).create(&directory)?;
        let runtime = Self {
            directory,
            previous_ca: std::env::var_os("SSL_CERT_FILE"),
            previous_budget: std::env::var_os("GAS_STATION_DID_GAS_LIMIT"),
        };
        fs::write(runtime.directory.join("ca.pem"), certificate)?;
        // The production constructor requires this setting. Supply and restore it
        // inside the test so the gate requires no environment configuration.
        std::env::set_var("SSL_CERT_FILE", runtime.directory.join("ca.pem"));
        std::env::set_var("GAS_STATION_DID_GAS_LIMIT", "1000000");
        Ok(runtime)
    }
}
impl Drop for TestRuntime {
    fn drop(&mut self) {
        for (name, previous) in [
            ("SSL_CERT_FILE", &self.previous_ca),
            ("GAS_STATION_DID_GAS_LIMIT", &self.previous_budget),
        ] {
            match previous {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
        let _ = fs::remove_dir_all(&self.directory);
    }
}

// Apply a short deadline to the production TLS/HTTP transport, including calls
// outside recovery (whose deadline is set by GasStation itself).
struct BoundedHttps;
impl Exchange for BoundedHttps {
    fn request(&self, endpoint: &str, method: &str, params: &Value) -> Result<Value, RpcFault> {
        self.request_until(endpoint, method, params, Instant::now() + RPC_BUDGET)
    }
    fn request_until(
        &self,
        endpoint: &str,
        method: &str,
        params: &Value,
        deadline: Instant,
    ) -> Result<Value, RpcFault> {
        HttpsExchange.request_until(
            endpoint,
            method,
            params,
            deadline.min(Instant::now() + RPC_BUDGET),
        )
    }
}

struct Transcript {
    raw: Vec<u8>,
    hash: Word,
    nonce: u64,
    receipt: Value,
    sponsor: Address,
    account: Address,
    paymaster: Address,
    authorization_nonce: u64,
}
impl Transcript {
    fn response(&self, request: &Value) -> Value {
        assert_eq!(request["jsonrpc"], "2.0");
        assert_eq!(request["id"], 1);
        let params = &request["params"];
        let result = match request["method"].as_str().expect("RPC method") {
            "eth_chainId" => {
                assert_eq!(*params, json!([]));
                json!(format!("0x{CHAIN_ID:x}"))
            }
            "eth_getBlockByNumber" => {
                assert!(
                    params == &json!(["latest", false]) || params == &json!(["finalized", false])
                );
                json!({"number":"0x3ea", "hash":hex(&[0xab; 32]), "timestamp":format!("0x{NOW:x}")})
            }
            "eth_getBalance" => {
                assert_eq!(*params, json!([hex(&self.sponsor), "pending"]));
                json!("0x56bc75e2d63100000")
            }
            "eth_getTransactionCount" => {
                assert_eq!(params[1], "pending");
                if params[0] == hex(&self.sponsor) {
                    json!(format!("0x{:x}", self.nonce))
                } else {
                    assert_eq!(params[0], hex(&self.account));
                    json!(format!("0x{:x}", self.authorization_nonce))
                }
            }
            "eth_getCode" => {
                assert_eq!(params[0], hex(&self.account));
                assert!(
                    params[1] == "pending"
                        || params[1]
                            == json!({"blockHash":hex(&[0xab; 32]), "requireCanonical":true})
                );
                json!(hex(&[&[0xef, 0x01, 0x00][..], &self.paymaster].concat()))
            }
            "eth_call" => {
                let did = [
                    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x04,
                ];
                if params[0]["to"] == hex(&did) {
                    assert_eq!(params[1], "pending");
                    let data = params[0]["data"].as_str().expect("DID calldata");
                    let call = |signature: &[u8]| {
                        hex(&[&keccak(signature)[..4], &address_word(self.account)].concat())
                    };
                    if data == call(b"getLayerXDid(address)") {
                        // Recorded reverted DID lookup; the real station must use
                        // getUnifiedAccount's unbound account representation.
                        return json!({"jsonrpc":"2.0", "id":1, "error":{
                            "code":3, "message":"execution reverted: Malformed", "data":hex(&keccak(b"Malformed()")[..4])
                        }});
                    }
                    assert_eq!(data, call(b"getUnifiedAccount(address)"));
                    json!(hex(&[
                        address_word(self.account),
                        word(128),
                        word(0),
                        word(0),
                        word(0)
                    ]
                    .concat()))
                } else {
                    assert_eq!(params[0]["to"], hex(&self.account));
                    let data = bytes(params[0]["data"].as_str().expect("account calldata"))
                        .expect("hex calldata");
                    if data == keccak(b"nonce()")[..4] {
                        assert_eq!(params[1], "pending");
                    } else {
                        assert_eq!(
                            data,
                            [
                                &keccak(b"usedQuoteNonces(address,uint256)")[..4],
                                &address_word(self.sponsor),
                                &word(u128::from(self.nonce + 1))
                            ]
                            .concat()
                        );
                        assert_eq!(
                            params[1],
                            json!({"blockHash":hex(&[0xab; 32]), "requireCanonical":true})
                        );
                    }
                    json!(hex(&word(0)))
                }
            }
            "eth_sendRawTransaction" => {
                assert_eq!(*params, json!([hex(&self.raw)]));
                json!(hex(&self.hash))
            }
            "eth_getTransactionReceipt" => {
                assert_eq!(*params, json!([hex(&self.hash)]));
                Value::Null
            }
            "pax_sequencerReceipt" => {
                assert_eq!(*params, json!([hex(&self.hash)]));
                self.receipt.clone()
            }
            other => panic!("unexpected fixture RPC: {other}: {params}"),
        };
        json!({"jsonrpc":"2.0", "id":1, "result":result})
    }
}

struct Producer {
    endpoint: String,
    stop: Arc<AtomicBool>,
    server: Option<JoinHandle<Result<Vec<Value>, String>>>,
}
impl Producer {
    fn start(tls: &Value, transcript: Transcript) -> TestResult<Self> {
        let identity = native_tls::Identity::from_pkcs8(
            tls["certificate"]
                .as_str()
                .ok_or("missing TLS certificate")?
                .as_bytes(),
            tls["privateKey"]
                .as_str()
                .ok_or("missing TLS fixture key")?
                .as_bytes(),
        )?;
        let acceptor = native_tls::TlsAcceptor::new(identity)?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let endpoint = format!("https://{}", listener.local_addr()?);
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let server = std::thread::spawn(move || {
            let deadline = Instant::now() + Duration::from_secs(20);
            let mut requests = Vec::new();
            while !stopped.load(Ordering::SeqCst) {
                if Instant::now() >= deadline {
                    return Err("loopback fixture exceeded 20 seconds".into());
                }
                let (socket, _) = match listener.accept() {
                    Ok(pair) => pair,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => return Err(error.to_string()),
                };
                socket
                    .set_read_timeout(Some(RPC_BUDGET))
                    .map_err(|e| e.to_string())?;
                socket
                    .set_write_timeout(Some(RPC_BUDGET))
                    .map_err(|e| e.to_string())?;
                let mut stream = acceptor.accept(socket).map_err(|e| e.to_string())?;
                let mut header = Vec::new();
                let mut byte = [0; 1];
                while !header.ends_with(b"\r\n\r\n") {
                    if header.len() >= 8192 {
                        return Err("oversized fixture request headers".into());
                    }
                    stream.read_exact(&mut byte).map_err(|e| e.to_string())?;
                    header.push(byte[0]);
                }
                let header = std::str::from_utf8(&header).map_err(|e| e.to_string())?;
                assert!(header.starts_with("POST / HTTP/1.1\r\n"));
                let length = header
                    .lines()
                    .find_map(|line| line.strip_prefix("Content-Length: "))
                    .ok_or("missing fixture Content-Length")?
                    .parse::<usize>()
                    .map_err(|e| e.to_string())?;
                if length > 65536 {
                    return Err("oversized fixture request body".into());
                }
                let mut body = vec![0; length];
                stream.read_exact(&mut body).map_err(|e| e.to_string())?;
                let request: Value = serde_json::from_slice(&body).map_err(|e| e.to_string())?;
                let body = transcript.response(&request).to_string();
                requests.push(request);
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).map_err(|e| e.to_string())?;
                stream.flush().map_err(|e| e.to_string())?;
                stream.shutdown().map_err(|e| e.to_string())?;
            }
            Ok(requests)
        });
        Ok(Self {
            endpoint,
            stop,
            server: Some(server),
        })
    }
    fn finish(&mut self) -> TestResult<Vec<Value>> {
        self.stop.store(true, Ordering::SeqCst);
        self.server
            .take()
            .ok_or("fixture already stopped")?
            .join()
            .map_err(|_| "loopback fixture panicked")?
            .map_err(Into::into)
    }
}
impl Drop for Producer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(server) = self.server.take() {
            let _ = server.join();
        }
    }
}

#[test]
fn failed_sponsor_survives_submit_status_and_reopen() -> TestResult {
    let fixture: Value = serde_json::from_str(include_str!("fixtures/sequencer_receipts.json"))?;
    let runtime = TestRuntime::new(
        fixture["tls"]["certificate"]
            .as_str()
            .ok_or("missing TLS certificate")?,
    )?;
    let sponsor = Signer::new(1)?;
    let account = Signer::new(2)?;
    let now = NOW;
    let fees = Fees {
        gas_limit: 200_000,
        max_fee_per_gas: 500_000_000_000,
        max_priority_fee_per_gas: 1_000_000_000,
    };
    let calls = vec![Call {
        to: [0x55; 20],
        value: word(0),
        data: vec![1, 2, 3],
    }];
    for index in 0..2 {
        let paymaster: Address = [0x44; 20];
        let mut config = ServiceConfig {
            station: StationConfig {
                chain_id: CHAIN_ID,
                endpoints: vec!["https://127.0.0.1:1".into()],
                paymaster,
                token: SIDIORA,
                decimals: 6,
                max_rate_age: 300,
                spread_bps: 500,
                margin_bps: 100,
                per_account_limit: 8_000_000,
                per_interval_limit: 16_000_000,
                per_quote_limit: 4_000_000,
                interval_seconds: 60,
                balance_floor: 1,
                relayer_key_env: "REPAIR_2231162_SPONSOR".into(),
            },
            listen: "127.0.0.1:12345".parse()?,
            gas_limit: fees.gas_limit,
            max_priority_fee_per_gas: fees.max_priority_fee_per_gas,
            registered_senders: vec![],
        };
        let key = Key {
            sponsor: sponsor.address(),
            quote_nonce: word(index as u128 + 1),
        };
        let quote = Quote {
            sponsor: sponsor.address(),
            token: SIDIORA,
            max_token_amount: word(10),
            token_amount: word(1),
            deadline: word(u128::from(now + 600)),
            nonce: key.quote_nonce,
            gas_cost: word(fees.gas_cost()?),
        };
        let digest = quote_digest(word(u128::from(CHAIN_ID)), account.address(), &quote);
        let signed = SignedQuote {
            signature: sponsor.sign_digest(digest)?,
            digest,
            quote,
        };
        let account_signature = account.sign_digest(batch_digest(
            CHAIN_ID,
            account.address(),
            word(0),
            &calls,
            &signed.quote,
        )?)?;
        let mut authorization = Authorization {
            chain_id: CHAIN_ID,
            delegate: paymaster,
            nonce: index as u64 + 2,
            signature: [0; 65],
        };
        authorization.signature = account.sign_digest(authorization_digest(&authorization)?)?;
        let journal_path = runtime.directory.join(format!("station-{index}.jsonl"));
        let mut journal = Journal::open(&journal_path)?;
        let reservation = Entry::QuoteReserved {
            quote: Box::new(QuoteRecord {
                key,
                did: hex(&account.address()),
                batch_nonce: word(0),
                chain_id: CHAIN_ID,
                paymaster,
                account: account.address(),
                token: SIDIORA,
                maximum: word(10),
                amount: word(1),
                deadline: now + 600,
                gas_cost: signed.quote.gas_cost,
                issued_at: now,
                signature: signed.signature.to_vec(),
                fees,
            }),
        };
        journal.append(&reservation)?;
        let prepared = prepare_submission(
            &mut journal,
            TransactionRequest {
                chain_id: CHAIN_ID,
                account: account.address(),
                paymaster,
                nonce: index as u64,
                batch_nonce: word(0),
                fees,
                calls: &calls,
                authorizations: &[authorization.clone()],
                quote: &signed,
                account_signature: &account_signature,
            },
            &hex(&account.address()),
            &sponsor,
        )?;
        drop(journal);
        // The committed producer receipt supplies all execution evidence. Bind
        // its request identity to the genuine type-4 bytes prepared by the crate;
        // node_revert_vector_preserves_evidence separately checks the unchanged
        // signed vector. This service test does not claim signature verification.
        let mut produced = fixture[if index == 0 { "reverted" } else { "receipted" }].clone();
        produced["txHash"] = json!(hex(&prepared.hash));
        produced["nonce"] = json!(format!("0x{:x}", prepared.nonce));
        if index == 1 {
            produced["sponsor"] = json!(hex(&[0x77; 20]));
        }
        let mut producer = Producer::start(
            &fixture["tls"],
            Transcript {
                raw: prepared.raw.clone(),
                hash: prepared.hash,
                nonce: prepared.nonce,
                receipt: produced.clone(),
                sponsor: sponsor.address(),
                account: account.address(),
                paymaster,
                authorization_nonce: authorization.nonce,
            },
        )?;
        config.station.endpoints = vec![producer.endpoint.clone()];
        // Only the quote is durable initially: /submit must prepare and broadcast
        // exactly the same transaction itself through the production path.
        let initial_path = runtime.directory.join(format!("initial-{index}.jsonl"));
        let mut initial = Journal::open(&initial_path)?;
        initial.append(&reservation)?;
        drop(initial);
        assert_eq!(fixed::<32>(&produced["txHash"])?, prepared.hash);
        assert_eq!(
            quantity(produced["nonce"].as_str().ok_or("missing node nonce")?)?,
            index as u128
        );
        let wire_quote = json!({"sponsor": hex(&sponsor.address()), "token": hex(&SIDIORA),
            "maxTokenAmount": "10", "tokenAmount": "1", "deadline": (now + 600).to_string(),
            "quoteNonce": (index + 1).to_string(), "gasCost": fees.gas_cost()?.to_string(), "decimals": 6});
        let submit = serde_json::to_vec(&json!({
            "call": {"to": hex(&account.address()), "value": "0", "data": hex(&encode_sponsored(&calls, &signed, &account_signature)?)},
            "authorization": {"chainId": CHAIN_ID.to_string(), "address": hex(&paymaster),
                "nonce": authorization.nonce.to_string(), "yParity": authorization.signature[64] - 27,
                "r": hex(&authorization.signature[..32]), "s": hex(&authorization.signature[32..64])},
            "batch": {"chainId": CHAIN_ID.to_string(), "account": hex(&account.address()), "nonce": "0",
                "calls": [{"to": hex(&calls[0].to), "value": "0", "data": hex(&calls[0].data)}], "quote": wire_quote},
            "accountSignature": hex(&account_signature), "relayerSignature": hex(&signed.signature),
        }))?;
        let status = serde_json::to_vec(
            &json!({"sponsor": hex(&key.sponsor), "quoteNonce": (index + 1).to_string(),
            "account": hex(&account.address()), "relayerSignature": hex(&signed.signature)}),
        )?;
        let rpc = || ConfiguredRpc::new(&config.station, BoundedHttps);
        let endpoint = SequencerEndpoint::new(rpc()?);
        let full = endpoint
            .full_receipt(&prepared.hash)?
            .ok_or("loopback producer lost receipt")?;
        let expected = if index == 0 {
            ServiceError::Refused
        } else {
            ServiceError::Unavailable
        };
        if index == 0 {
            assert_eq!(produced["status"], "0x0");
            assert_eq!(produced["sponsor"], hex(&[0; 20]));
            assert_eq!(full.sponsor, None);
            assert_eq!(full.vm_error.as_deref(), Some("execution reverted"));
        } else {
            assert_eq!(produced["status"], "0x1");
            assert_eq!(full.sponsor, Some([0x77; 20]));
            assert_ne!(full.sponsor, Some(sponsor.address()));
        }
        // Drop and reopen the real service and its exclusively locked journal.
        for reopen in 0..2 {
            let station = GasStation::new(
                config.station.clone(),
                Signer::new(1)?,
                rpc()?,
                PaymasterRateSource::new(rpc()?, paymaster),
                Journal::open(&initial_path)?,
            )?;
            let mut service = Service::new(&config, station, move || Some(now), Limits::default())
                .with_sequencer(SequencerEndpoint::new(rpc()?));
            if reopen == 1 {
                let recovered = service.recover(Duration::from_secs(5))?;
                assert_eq!(recovered.pending, 1);
                assert_eq!(recovered.completed, 0);
                assert_eq!(recovered.unreachable, 0);
                assert_eq!(recovered.deferred, 0);
                assert_eq!(
                    service.station().journal().state().items[&key]
                        .submission
                        .as_ref(),
                    Some(&prepared)
                );
            }
            assert_eq!(service.respond("/submit", &submit), Err(expected));
            assert_eq!(expected.status(), if index == 0 { 422 } else { 503 });
            assert_eq!(
                service.station().journal().state().items[&key]
                    .submission
                    .as_ref(),
                Some(&prepared)
            );
            if index == 0 {
                let answer = service.respond("/status", &status)?;
                assert_eq!(answer["state"], "completed");
                assert_eq!(
                    answer["completion"],
                    json!({"outcome": "reverted", "rung": "receipted", "transactionHash": hex(&prepared.hash)})
                );
            } else {
                assert_eq!(
                    service.respond("/status", &status),
                    Err(ServiceError::Unavailable)
                );
            }
        }
        let requests = producer.finish()?;
        assert_eq!(
            requests
                .iter()
                .filter(|request| request["method"] == "eth_sendRawTransaction")
                .count(),
            1,
            "submit broadcasts once; reopening and recovery must retain the receipted transaction"
        );
        assert!(requests
            .iter()
            .any(|request| request["method"] == "eth_call"
                && request["params"][0]["data"].as_str().is_some_and(
                    |data| data.starts_with(&hex(&keccak(b"getLayerXDid(address)")[..4]))
                )));
        assert!(requests
            .iter()
            .any(|request| request["method"] == "eth_call"
                && request["params"][0]["data"].as_str().is_some_and(
                    |data| data.starts_with(&hex(&keccak(b"getUnifiedAccount(address)")[..4]))
                )));
    }
    Ok(())
}
