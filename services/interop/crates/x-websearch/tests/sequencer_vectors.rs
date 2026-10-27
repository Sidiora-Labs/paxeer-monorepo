use std::cell::Cell;
use std::collections::BTreeSet;
use std::error::Error;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::merkle::leaf_hash;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptCheck, ReceiptKeys};
use serde::Deserialize;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use sha3::Keccak256;
use x_websearch::payment::{PaymentGate, PAYMENT_RESPONSE, PAYMENT_SIGNATURE};
use x_websearch::settlement::{SettlementRpc, SettlementRung};
use x_websearch::{Config, Request, Response, Route};

type Outcome<T = ()> = Result<T, Box<dyn Error + Send + Sync>>;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Vectors {
    name: String,
    chain_id: u64,
    cases: Vec<Case>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct Case {
    id: String,
    vector: String,
    receipt_json: Value,
    raw_transaction: String,
    prev_seq_hash: String,
    signed_receipt: String,
    pinned_key: PinnedKey,
    expected: Value,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
struct PinnedKey {
    key_id: String,
    public_key: String,
    first_height: u64,
    last_height: u64,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum Classification {
    Answer,
    SignatureRefusal,
    DecodeRefusal,
    TransactionRefusal,
    SequenceRefusal,
    LogRefusal,
}

fn classify(case: &Case) -> Classification {
    let (vector, classification, verifier) = match case.id.as_str() {
        "V1-pax-transfer"
        | "V1-erc20-approve"
        | "V1-layerxledger-pay-exact"
        | "V1-layerxbridge-bridge-out" => ("V1", Classification::Answer, "accepts"),
        "V2-layerxledger-revert" => ("V2", Classification::Answer, "accepts"),
        "V3-key-not-pinned" | "V3-height-outside-key-range" => {
            ("V3", Classification::SignatureRefusal, "SequencerSignature")
        }
        "V3-non-canonical-bytes" => ("V3", Classification::DecodeRefusal, "Decode"),
        "V3-activity-id-not-raw-hash" => ("V3", Classification::TransactionRefusal, "accepts"),
        "V3-seq-hash-mismatch" => ("V3", Classification::SequenceRefusal, "accepts"),
        "V3-tampered-log" => ("V3", Classification::LogRefusal, "accepts"),
        id => panic!("unclassified shared sequencer-receipt case: {id}"),
    };
    assert_eq!(case.vector, vector, "{}", case.id);
    assert_eq!(case.expected["verifier"], verifier, "{}", case.id);
    classification
}

fn fixtures() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures")
}

fn shared_vectors() -> Outcome<Vectors> {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let path = root.join("sdk/platform/conformance/fixtures/sequencer-receipt-v1.json");
    let vectors: Vectors = serde_json::from_slice(&std::fs::read(path)?)?;
    assert_eq!(vectors.name, "sequencer-receipt-v1");
    assert_eq!(vectors.chain_id, 125);
    assert_eq!(vectors.cases.len(), 11, "all shared cases must execute");
    let mut ids = BTreeSet::new();
    for case in &vectors.cases {
        assert!(ids.insert(&case.id), "duplicate shared case: {}", case.id);
        classify(case);
        assert_eq!(
            case.receipt_json["receipt"], case.signed_receipt,
            "{}",
            case.id
        );
    }
    Ok(vectors)
}

fn bytes(text: &str) -> Vec<u8> {
    let digits = text.strip_prefix("0x").expect("fixture hex prefix");
    assert_eq!(digits.len() % 2, 0);
    (0..digits.len())
        .step_by(2)
        .map(|offset| u8::from_str_radix(&digits[offset..offset + 2], 16).expect("fixture hex"))
        .collect()
}

fn hash(text: &str) -> [u8; 32] {
    bytes(text).try_into().expect("fixture 32-byte hash")
}

fn quantity(value: &Value) -> u64 {
    let text = value.as_str().expect("fixture quantity string");
    u64::from_str_radix(text.strip_prefix("0x").expect("quantity prefix"), 16)
        .expect("fixture u64 quantity")
}

fn authorization(case: &Case) -> [SequencerAuthorization; 1] {
    let key = &case.pinned_key;
    assert!(key.first_height <= key.last_height, "{}", case.id);
    [SequencerAuthorization::new(
        hash(&key.key_id),
        hash(&key.public_key),
        key.first_height,
        key.last_height,
    )]
}

/// Replays only exact recorded requests. Receipt answers and signatures come
/// unchanged from the shared Go vectors; account lookups come from the existing
/// ledger recording. Unexpected requests fail instead of receiving a fallback.
struct Replay {
    address: SocketAddr,
    thread: Option<JoinHandle<Outcome<()>>>,
}

impl Replay {
    fn start(chain_id: u64, case: &Case, ledger: bool) -> Outcome<Self> {
        let mut rules = vec![
            json!({"method": "eth_chainId", "params": [], "result": format!("0x{chain_id:x}")}),
            json!({"method": "pax_sequencerReceipt", "params": [case.receipt_json["txHash"]], "result": case.receipt_json}),
        ];
        if ledger {
            let recording: Value =
                serde_json::from_slice(&std::fs::read(fixtures().join("gateway/metered.json"))?)?;
            let calls = recording["endpoints"]["gateway"]["*"]
                .as_array()
                .ok_or("recorded ledger calls")?;
            let accounts: Vec<_> = calls
                .iter()
                .filter(|call| {
                    call["method"] == "eth_call"
                        && call["params"][0]["data"]
                            .as_str()
                            .is_some_and(|data| data.starts_with("0x3f050cbc"))
                })
                .cloned()
                .collect();
            assert_eq!(
                accounts.len(),
                4,
                "recorded receiver's four account lookups"
            );
            rules.extend(accounts);
        }
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let handle = thread::spawn(move || -> Outcome<()> {
            for connection in listener.incoming() {
                let mut stream = connection?;
                stream.set_read_timeout(Some(Duration::from_secs(10)))?;
                stream.set_write_timeout(Some(Duration::from_secs(10)))?;
                let Some(request) = read_request(&mut stream)? else {
                    return Ok(());
                };
                let rule = rules
                    .iter()
                    .find(|rule| {
                        rule["method"] == request["method"] && rule["params"] == request["params"]
                    })
                    .ok_or_else(|| format!("unrecorded RPC request: {request}"))?;
                let body = json!({
                    "jsonrpc": "2.0", "id": request["id"], "result": rule["result"],
                })
                .to_string();
                write!(
                    stream,
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                )?;
            }
            Ok(())
        });
        Ok(Self {
            address,
            thread: Some(handle),
        })
    }

    fn endpoint(&self) -> String {
        format!("http://{}", self.address)
    }

    fn finish(mut self) -> Outcome {
        TcpStream::connect(self.address)?.shutdown(std::net::Shutdown::Both)?;
        self.thread
            .take()
            .ok_or("replay already stopped")?
            .join()
            .map_err(|_| "replay thread panicked")??;
        Ok(())
    }
}

fn read_request(stream: &mut TcpStream) -> Outcome<Option<Value>> {
    let mut head = Vec::new();
    while !head.ends_with(b"\r\n\r\n") {
        let mut byte = [0];
        if stream.read(&mut byte)? == 0 {
            return if head.is_empty() {
                Ok(None)
            } else {
                Err("truncated replay request".into())
            };
        }
        head.push(byte[0]);
        if head.len() > 16_384 {
            return Err("oversized replay request headers".into());
        }
    }
    let head = std::str::from_utf8(&head)?;
    let length = head
        .split("\r\n")
        .filter_map(|line| line.split_once(':'))
        .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
        .ok_or("replay request missing content length")?
        .1
        .trim()
        .parse::<usize>()?;
    if length > 1_048_576 {
        return Err("oversized replay request body".into());
    }
    let mut body = vec![0; length];
    stream.read_exact(&mut body)?;
    Ok(Some(serde_json::from_slice(&body)?))
}

mod settlement {
    use super::{bytes, hash, quantity, Case, Outcome, SettlementRung};
    use x_websearch::settlement::SettlementReceipt;

    pub fn decode(case: &Case) -> Outcome<SettlementReceipt> {
        let answer = x_websearch::settlement::decode(&case.receipt_json)?;
        assert_eq!(case.receipt_json["rung"], "receipted", "{}", case.id);
        assert_eq!(answer.rung, SettlementRung::Receipted, "{}", case.id);
        assert_eq!(
            answer.seq,
            Some(quantity(&case.receipt_json["seq"])),
            "{}",
            case.id
        );
        assert_eq!(
            answer.height,
            Some(quantity(&case.receipt_json["height"])),
            "{}",
            case.id
        );
        assert_eq!(
            answer.status,
            Some(quantity(&case.receipt_json["status"])),
            "{}",
            case.id
        );
        assert_eq!(
            answer.key_id,
            Some(hash(case.receipt_json["keyId"].as_str().ok_or("keyId")?)),
            "{}",
            case.id
        );
        assert_eq!(
            answer.tx_hash,
            hash(case.receipt_json["txHash"].as_str().ok_or("txHash")?),
            "{}",
            case.id
        );
        assert_eq!(
            answer.nonce,
            quantity(&case.receipt_json["nonce"]),
            "{}",
            case.id
        );
        assert_eq!(
            answer.receipt.as_deref(),
            Some(bytes(&case.signed_receipt).as_slice()),
            "{}",
            case.id
        );

        Ok(answer)
    }
}

#[test]
fn every_shared_answer_case_decodes_through_the_settlement_client() -> Outcome {
    let vectors = shared_vectors()?;
    let mut checked = 0;
    let mut verified_answers = 0;
    for case in &vectors.cases {
        let classification = classify(case);
        let answer = settlement::decode(case)?;

        let replay = Replay::start(vectors.chain_id, case, false)?;
        let client = SettlementRpc::new(&replay.endpoint(), vectors.chain_id)?;
        assert_eq!(
            client.sequencer_receipt(&answer.tx_hash)?,
            Some(answer.clone()),
            "{}",
            case.id
        );
        replay.finish()?;

        if classification == Classification::Answer {
            assert_eq!(case.expected["rung"], "receipted", "{}", case.id);
            let keys = authorization(case);
            let receipt = verify_sequencer_signature(
                answer.receipt.as_deref().ok_or("signed receipt")?,
                ReceiptKeys::HeightRanges(&keys),
            )
            .map_err(|failure| format!("{}: {failure:?}", case.id))?;
            let facts = receipt.protocol().ok_or("protocol receipt")?;
            assert_eq!(answer.seq, Some(facts.global_sequence()), "{}", case.id);
            assert_eq!(
                answer.height,
                Some(u64::from_be_bytes(facts.batch_id()[24..].try_into()?)),
                "{}",
                case.id
            );
            assert_eq!(
                answer.status,
                Some(u64::from(facts.result_code() == 0)),
                "{}",
                case.id
            );
            assert_eq!(answer.key_id, Some(facts.context_hash()), "{}", case.id);
            verified_answers += 1;
        }
        checked += 1;
    }
    assert_eq!(checked, 11);
    assert_eq!(
        verified_answers, 5,
        "four successes and one execution revert"
    );
    Ok(())
}

struct Scratch(PathBuf);

impl Scratch {
    fn new() -> Outcome<Self> {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let path = std::env::temp_dir().join(format!(
            "x-websearch-sequencer-vectors-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::create_dir(&path)?;
        Ok(Self(path))
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn clock() -> u64 {
    1_000_000_000_000
}

#[test]
fn shared_refusal_cases_do_not_release_a_payment() -> Outcome {
    let vectors = shared_vectors()?;
    let mut checked = 0;
    let mut wrong_refusals = Vec::new();
    for case in &vectors.cases {
        let classification = classify(case);
        if classification == Classification::Answer {
            continue;
        }
        let expected_refusal = match case.expected["verifier"].as_str() {
            Some("SequencerSignature" | "Decode") => "receipt_unverified",
            Some("accepts") => "receipt_mismatch",
            verifier => panic!(
                "{}: unclassified verifier expectation: {verifier:?}",
                case.id
            ),
        };
        let keys = authorization(case);
        let signed = bytes(&case.signed_receipt);
        let verification = verify_sequencer_signature(&signed, ReceiptKeys::HeightRanges(&keys));
        match classification {
            Classification::SignatureRefusal | Classification::DecodeRefusal => {
                let expected = if classification == Classification::SignatureRefusal {
                    ReceiptCheck::SequencerSignature
                } else {
                    ReceiptCheck::Decode
                };
                assert_eq!(
                    verification.map_err(|failure| failure.check),
                    Err(expected),
                    "{}",
                    case.id
                );
            }
            Classification::TransactionRefusal
            | Classification::SequenceRefusal
            | Classification::LogRefusal => {
                let receipt =
                    verification.map_err(|failure| format!("{}: {failure:?}", case.id))?;
                let facts = receipt.protocol().ok_or("protocol receipt")?;
                match classification {
                    Classification::TransactionRefusal => {
                        assert_eq!(case.expected["refusedAt"], "R4");
                        let transaction_hash: [u8; 32] =
                            Keccak256::digest(bytes(&case.raw_transaction)).into();
                        assert_ne!(facts.activity_id(), transaction_hash, "{}", case.id);
                    }
                    Classification::SequenceRefusal => {
                        assert_eq!(case.expected["refusedAt"], "R6");
                        let previous = hash(&case.prev_seq_hash);
                        let mut sequence = Sha256::new();
                        sequence.update(b"PAXEER/v1/sequence\0");
                        sequence.update(previous);
                        sequence.update(facts.global_sequence().to_be_bytes());
                        sequence.update(facts.activity_id());
                        let expected: [u8; 32] = sequence.finalize().into();
                        assert!(
                            facts.previous_state_root() != previous
                                || facts.resulting_state_root() != expected,
                            "{}",
                            case.id
                        );
                    }
                    Classification::LogRefusal => {
                        assert_eq!(case.expected["refusedAt"], "R5");
                        let answer = settlement::decode(case)?;
                        let logs = answer
                            .logs
                            .into_iter()
                            .map(|log| {
                                layerx_proof::sequencer_logs::SequencerLog::new(
                                    log.address,
                                    log.topics,
                                    log.data,
                                    log.log_index,
                                )
                            })
                            .collect();
                        assert!(
                            layerx_proof::sequencer_logs::verify_logs(&receipt, logs).is_err(),
                            "{}",
                            case.id
                        );
                    }
                    _ => unreachable!(),
                }
            }
            Classification::Answer => unreachable!(),
        }

        let scratch = Scratch::new()?;
        let replay = Replay::start(vectors.chain_id, case, true)?;
        let mut configuration: Value =
            serde_json::from_slice(&std::fs::read(fixtures().join("config/valid.json"))?)?;
        let gateway = configuration["gateway"]
            .as_object_mut()
            .ok_or("gateway configuration")?;
        let key_id = gateway.remove("sequencer_id").ok_or("recorded key id")?;
        let public_key = gateway
            .remove("sequencer_public_key")
            .ok_or("recorded public key")?;
        gateway.insert(
            "receipt_keys".to_owned(),
            json!([{
                "key_id": key_id,
                "public_key": public_key,
                "first_height": case.pinned_key.first_height,
                "last_height": case.pinned_key.last_height,
            }]),
        );
        configuration["evm"]["endpoint"] = json!(replay.endpoint());
        configuration["evm"]["chain_id"] = json!(vectors.chain_id);
        let mut config = Config::parse(&configuration.to_string())?;
        config.data_dir = scratch.0.clone();
        // The shared vector's repeated-byte key id is a config placeholder.
        config.gateway.receipt_keys = keys.to_vec();
        assert_eq!(config.gateway.receipt_keys, keys, "{}", case.id);
        let mut secret = [0; 32];
        secret[31] = 3; // Receiver of the existing recorded account lookups.
        let receiver = k256::ecdsa::SigningKey::from_slice(&secret)?;
        let gate = PaymentGate::new(&config, &receiver, x_websearch::conformance_suite()?, clock)?;
        let mut request = Request {
            method: "GET".to_owned(),
            route: Route::Search,
            path: "/search".to_owned(),
            query: Some(format!("q={}", case.id)),
            headers: vec![("Host".to_owned(), "127.0.0.1".to_owned())],
            digest: None,
            peer: "127.0.0.1:1".parse()?,
        };
        let required = gate
            .payment_required(&request, None)
            .map_err(|response| format!("payment offer: {response:?}"))?;
        let required = serde_json::to_value(required)?;
        let offer = required["accepts"]
            .as_array()
            .ok_or("payment offers")?
            .iter()
            .find(|offer| offer["scheme"] == "exact")
            .ok_or("exact offer")?;
        let payload = json!({
            "x402Version": 2,
            "accepted": offer,
            "payload": {
                "txHash": case.receipt_json["txHash"],
                "receiptDigest": x_websearch::payment::hex(&leaf_hash(&signed).map_err(|error| format!("receipt leaf: {error:?}"))?),
                "verificationLevel": "receipted",
            },
        });
        request.headers.push((
            PAYMENT_SIGNATURE.to_owned(),
            STANDARD.encode(serde_json::to_vec(&payload)?),
        ));
        let releases = Cell::new(0);
        let response = gate.settle(&request, &|_| {
            releases.set(releases.get() + 1);
            Response::json(200, b"{}".to_vec())
        });
        assert_eq!(response.status, 402, "{}: {response:?}", case.id);
        assert_eq!(releases.get(), 0, "{} released paid content", case.id);
        let body: Value = serde_json::from_slice(&response.body)?;
        if body["error"] != expected_refusal {
            wrong_refusals.push(format!(
                "{}: expected {expected_refusal}, got {}",
                case.id, body["error"]
            ));
        }
        let header = response
            .headers
            .iter()
            .find(|(name, _)| name.eq_ignore_ascii_case(PAYMENT_RESPONSE))
            .ok_or("payment response header")?;
        let payment: Value = serde_json::from_slice(&STANDARD.decode(&header.1)?)?;
        assert_eq!(payment["success"], false, "{}", case.id);
        assert_eq!(payment["errorReason"], body["error"], "{}", case.id);
        for directory in ["receipts", "deliveries"] {
            assert_eq!(
                std::fs::read_dir(scratch.0.join("payments").join(directory))?.count(),
                0,
                "{}: {directory}",
                case.id
            );
        }
        for entry in std::fs::read_dir(scratch.0.join("payments/requests"))? {
            let record: x_websearch::payment::PaymentRecord =
                serde_json::from_slice(&std::fs::read(entry?.path())?)?;
            assert!(!record.released, "{} persisted a payment release", case.id);
            assert!(
                record.receipt.is_none(),
                "{} claimed an unverified receipt",
                case.id
            );
            assert!(
                record.receipt_digest.is_none(),
                "{} saved an unverified digest",
                case.id
            );
        }
        drop(gate);
        replay.finish()?;
        checked += 1;
    }
    assert_eq!(checked, 6, "all shared V3 cases executed");
    assert!(wrong_refusals.is_empty(), "{}", wrong_refusals.join("\n"));
    Ok(())
}
