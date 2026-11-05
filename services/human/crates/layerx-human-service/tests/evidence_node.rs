use layerx_types::clock::Deadline;
use std::collections::BTreeMap;
use std::fs::OpenOptions;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_client::rpc::evidence::{checkpoint, proof_bundle};
use layerx_client::rpc::submit::{submit_signed, Submission, SubmitError};
use layerx_client::sequencer::{RpcRefusal, SequencerEndpoint, SequencerReceipt};
use layerx_human_test_support as support;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use sha3::Keccak256;

use ed25519_dalek::{Signer as _, SigningKey};
use k256::ecdsa::SigningKey as EvmSigningKey;
use layerx_crypto::evm_transaction::{
    decode_signed as decode_transaction, recover_sender, Eip1559Call, EvmTransactionError,
};
use layerx_crypto::send::{ledger_payment_call, send_context_hash, CallOptions, SendDebit};
use layerx_crypto::SignatureMessage;
use layerx_intents::canonical::MAX_MESSAGE_BYTES;
use layerx_types::result::{KnownResult, ResultCode};
use layerx_wire::hash::Domain;
use layerx_wire::limits::PROTOCOL_VERSION;

use support::evidence_node::AdmissionJournal;
use support::EVIDENCE_NETWORK_ID;

fn sender(signer: &EvmSigningKey) -> [u8; 20] {
    checked(layerx_crypto::secp256k1::evm_address(
        signer.verifying_key().to_encoded_point(false).as_bytes(),
    ))
}

fn signed_activity(signer: &EvmSigningKey, sequence: u64, idempotency: u8) -> Vec<u8> {
    let from = [1; 32];
    let to = [2; 32];
    let asset = [3; 32];
    let idempotency_key = [idempotency; 32];
    let debit = SendDebit {
        from,
        to,
        asset,
        amount: 3,
        source_sequence: sequence,
        idempotency_key,
        expires_at: 1_000,
        context_hash: send_context_hash(&from, &to, &asset, 3, &idempotency_key),
        conditions: vec![],
        authorization_kind: 1,
        network_id: EVIDENCE_NETWORK_ID,
        protocol_version: PROTOCOL_VERSION,
    };
    let authorization_key = SigningKey::from_bytes(&[0x41; 32]);
    let authorization = checked(debit.authorization_message());
    let message = checked(SignatureMessage::new(
        Domain::SignaturePreimage,
        PROTOCOL_VERSION,
        EVIDENCE_NETWORK_ID,
        &authorization,
    ));
    let payload = checked(debit.encode_signed(
        authorization_key.verifying_key().to_bytes(),
        authorization_key.sign(&message.digest()).to_bytes(),
    ));
    let disclosed = checked(ledger_payment_call(
        &payload,
        &CallOptions {
            from: sender(signer),
            chain_id: 125,
            nonce: sequence,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
        },
    ));
    let call: Eip1559Call = disclosed.call;
    checked(call.sign(&signer.to_bytes().into()))
}

fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("sequencer fixture: {error:?}"))
}

fn hex(bytes: &[u8]) -> String {
    let mut value = String::from("0x");
    for byte in bytes {
        use std::fmt::Write as _;
        checked(write!(value, "{byte:02x}"));
    }
    value
}

fn unhex(value: &Value) -> Vec<u8> {
    let text = value.as_str().expect("hex RPC parameter");
    let digits = text.strip_prefix("0x").expect("hex prefix");
    assert_eq!(digits.len() % 2, 0);
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| checked(u8::from_str_radix(checked(std::str::from_utf8(pair)), 16)))
        .collect()
}

struct AdmissionSequencer {
    journal: AdmissionJournal,
    identities: BTreeMap<Vec<u8>, [u8; 20]>,
    senders: BTreeMap<[u8; 20], Vec<u8>>,
    queue: Vec<[u8; 32]>,
    authentication_refusals: u64,
    fail_stopped: bool,
    capacity: usize,
    receipt_key: [u8; 32],
    admitted: BTreeMap<[u8; 32], u64>,
}

impl AdmissionSequencer {
    fn new(signer: &SigningKey, label: &str) -> Self {
        Self {
            journal: AdmissionJournal::open(&support::directory(label), EVIDENCE_NETWORK_ID),
            identities: BTreeMap::new(),
            senders: BTreeMap::new(),
            queue: Vec::new(),
            authentication_refusals: 0,
            fail_stopped: false,
            capacity: 1,
            receipt_key: signer.verifying_key().to_bytes(),
            admitted: BTreeMap::new(),
        }
    }

    fn register_identity(&mut self, did: &[u8], sender: [u8; 20]) {
        self.identities.insert(did.to_vec(), sender);
        self.senders.insert(sender, did.to_vec());
    }

    fn admit(&mut self, raw: &[u8]) -> Result<[u8; 32], KnownResult> {
        let authenticated = (|| {
            if raw.is_empty() || raw.len() > MAX_MESSAGE_BYTES {
                return Err(KnownResult::MalformedEnvelope);
            }
            let (transaction, signature) =
                decode_transaction(raw).map_err(|error| match error {
                    EvmTransactionError::BadSignature => KnownResult::BadSignature,
                    _ => KnownResult::MalformedEnvelope,
                })?;
            let sender =
                recover_sender(&transaction, &signature).map_err(|_| KnownResult::BadSignature)?;
            if transaction.chain_id != 125 {
                return Err(KnownResult::WrongNetwork);
            }
            let did = self.senders.get(&sender).ok_or(KnownResult::BadSignature)?;
            if self.identities.get(did) != Some(&sender) {
                return Err(KnownResult::UnknownDid);
            }
            Ok(transaction)
        })();
        let transaction = authenticated.inspect_err(|_| self.authentication_refusals += 1)?;
        let tx_hash: [u8; 32] = Keccak256::digest(raw).into();
        if self.journal.contains(&tx_hash) {
            return Ok(tx_hash);
        }
        if self.queue.len() >= self.capacity {
            return Err(KnownResult::LengthLimit);
        }
        let mut record = b"LXAR".to_vec();
        record.extend_from_slice(&checked(u32::try_from(raw.len())).to_be_bytes());
        record.extend_from_slice(&tx_hash);
        record.extend_from_slice(raw);
        let mut digest = Sha256::new();
        digest.update(b"LXH/v1/admission-record\0");
        digest.update(&record[4..40]);
        digest.update(raw);
        record.extend_from_slice(&digest.finalize());
        let mut file = checked(
            OpenOptions::new()
                .append(true)
                .open(self.journal.directory().join("admission.journal")),
        );
        if file
            .write_all(&record)
            .and_then(|()| file.sync_data())
            .is_err()
        {
            self.fail_stopped = true;
            panic!("admission failed before durable response");
        }
        self.journal = AdmissionJournal::open(self.journal.directory(), EVIDENCE_NETWORK_ID);
        self.queue.push(tx_hash);
        self.admitted.insert(tx_hash, transaction.nonce);
        Ok(tx_hash)
    }

    fn serve(
        mut self,
        requests: usize,
        interrupt_first_submit: bool,
    ) -> (SequencerEndpoint, NodeRpcConfig, JoinHandle<Self>) {
        let listener = checked(TcpListener::bind("127.0.0.1:0"));
        let config = NodeRpcConfig {
            url: format!("http://{}/", checked(listener.local_addr())),
            chain_id: 125,
            deadline: Duration::from_secs(2),
        };
        let endpoint = checked(SequencerEndpoint::new(&config.url, &[]));
        checked(listener.set_nonblocking(true));
        let clock = support::StepClock::new(50_000);
        let server = thread::spawn(move || {
            let mut interrupted = false;
            for _ in 0..requests {
                let mut deadline = checked(Deadline::start(&clock, Duration::from_secs(5)));
                let stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(
                                !checked(deadline.remaining(&clock)).is_zero(),
                                "missing sequencer request"
                            );
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("sequencer accept: {error}"),
                    }
                };
                checked(stream.set_read_timeout(Some(Duration::from_secs(2))));
                checked(stream.set_write_timeout(Some(Duration::from_secs(2))));
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(checked(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(checked(value.trim().parse::<usize>()));
                    }
                }
                let length = length.expect("RPC content length");
                assert!(length <= 2_097_152);
                let mut body = vec![0; length];
                checked(reader.read_exact(&mut body));
                let request: Value = checked(serde_json::from_slice(&body));
                assert_eq!(request["jsonrpc"], "2.0");
                let params = &request["params"];
                let response = match request["method"].as_str().expect("RPC method") {
                    "eth_chainId" => {
                        assert_eq!(params, &json!([]));
                        json!({"result": "0x7d"})
                    }
                    "eth_sendRawTransaction" => {
                        assert_eq!(params.as_array().expect("submit params").len(), 1);
                        match self.admit(&unhex(&params[0])) {
                            Ok(hash) => {
                                if interrupt_first_submit && !interrupted {
                                    interrupted = true;
                                    continue;
                                }
                                json!({"result": hex(&hash)})
                            }
                            Err(result) => json!({"error": refusal(result)}),
                        }
                    }
                    "pax_sequencerReceipt" => {
                        assert_eq!(params.as_array().expect("receipt params").len(), 1);
                        let hash: [u8; 32] = checked(unhex(&params[0]).try_into());
                        json!({"result": self.admitted.get(&hash).map(|nonce| json!({
                            "rung": "submitted", "txHash": hex(&hash), "nonce": format!("{nonce:#x}")
                        }))})
                    }
                    "px_getReceiptKeys" => {
                        assert_eq!(params, &json!([]));
                        json!({"result": [{"keyId": hex(&self.receipt_key),
                            "publicKey": hex(&self.receipt_key), "firstHeight": "0x7", "lastHeight": "0x7"}]})
                    }
                    "eth_getBlockByNumber" => {
                        assert_eq!(params, &json!(["0x7", true]));
                        json!({"result": {"number": "0x7", "hash": hex(&[0x91; 32]),
                            "parentHash": hex(&[0x90; 32]), "timestamp": "0x32",
                            "transactionsRoot": hex(&[0; 32]), "receiptsRoot": hex(&[0; 32]),
                            "transactions": []}})
                    }
                    "eth_getProof" => {
                        assert_eq!(params, &json!([hex(&[0x52; 20]), [], "0x7"]));
                        json!({"result": {"address": hex(&[0x52; 20]), "balance": "0x0", "nonce": "0x0",
                            "codeHash": hex(&[0; 32]), "storageHash": hex(&[0; 32]),
                            "accountProof": [], "storageProof": []}})
                    }
                    method => panic!("unexpected sequencer RPC: {method}"),
                };
                let mut response = response;
                response["jsonrpc"] = json!("2.0");
                response["id"] = request["id"].clone();
                let answer = response.to_string();
                let mut stream = reader.into_inner();
                checked(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
                checked(stream.flush());
            }
            self
        });
        (endpoint, config, server)
    }
}

fn refusal(result: KnownResult) -> Value {
    let data = match result {
        KnownResult::MalformedEnvelope => "LXP_MALFORMED_ENVELOPE: authentication refused",
        KnownResult::WrongNetwork => "LXP_WRONG_NETWORK: authentication refused",
        KnownResult::BadSignature => "LXP_BAD_SIGNATURE: authentication refused",
        KnownResult::UnknownDid => "LXP_UNKNOWN_DID: authentication refused",
        KnownResult::LengthLimit => "LXP_LENGTH_LIMIT: admission capacity exhausted",
        other => panic!("unexpected admission refusal: {other:?}"),
    };
    json!({"code": 3, "message": "execution reverted", "data": data})
}

fn refused(result: KnownResult) -> SubmitError {
    let value = refusal(result);
    SubmitError::Refused {
        result: ResultCode::from_raw(result.raw()),
        refusal: RpcRefusal {
            code: 3,
            message: "execution reverted".to_owned(),
            data: Some(value["data"].clone()),
        },
    }
}

#[test]
fn advertised_durable_submit_is_backed_by_authenticated_fdatasync_admission() {
    let receipt_signer = SigningKey::from_bytes(&[0x51; 32]);
    let actor = checked(EvmSigningKey::from_slice(&[0x52; 32]));
    let mut gate = support::evidence_gate(&receipt_signer);
    let mut node = AdmissionSequencer::new(&receipt_signer, "durable-admission");
    node.register_identity(b"did:layerx:actor", sender(&actor));
    let admission_directory = node.journal.directory().to_path_buf();
    let (endpoint, config, server) = node.serve(11, true);
    let readiness = checked(NodeRpc::connect(&config));
    let ready = checked(gate.readiness_gate(&readiness));
    assert_eq!(ready.chain_id, 125);
    assert_eq!(ready.receipt_keys.len(), 1);
    assert_eq!(
        ready.receipt_keys[0].public_key,
        receipt_signer.verifying_key().to_bytes()
    );
    assert!(gate.evidence_authority().is_ok());
    assert!(gate.guard_write(|| ()).is_ok());
    let keys = checked(endpoint.receipt_keys());
    assert_eq!(keys.len(), 1);
    assert_eq!(
        keys[0].public_key,
        receipt_signer.verifying_key().to_bytes()
    );
    assert_eq!((keys[0].first_height, keys[0].last_height), (7, 7));

    let first = signed_activity(&actor, 1, 0x11);
    let (transaction, signature) = checked(decode_transaction(&first));
    assert_eq!(
        checked(recover_sender(&transaction, &signature)),
        sender(&actor)
    );
    assert_eq!(transaction.chain_id, 125);
    assert_eq!(transaction.nonce, 1);
    assert_eq!(&transaction.data[..4], &[0x2b, 0xb1, 0xae, 0x7c]);
    let first_hash: [u8; 32] = Keccak256::digest(&first).into();
    let Submission::Unknown(unknown) = checked(submit_signed(&endpoint, &first)) else {
        panic!("interrupted durable admission must have an unknown outcome");
    };
    assert_eq!(unknown.retry_bytes(), first);
    assert_eq!(
        AdmissionJournal::recover(&admission_directory, EVIDENCE_NETWORK_ID)[0].raw_transaction,
        first
    );
    let admitted = checked(submit_signed(&endpoint, unknown.retry_bytes()));
    assert_eq!(
        admitted,
        Submission::Submitted {
            tx_hash: first_hash,
            rung: Rung::Submitted
        }
    );
    let retried = checked(submit_signed(&endpoint, &first));
    assert_eq!(
        retried,
        Submission::Submitted {
            tx_hash: first_hash,
            rung: Rung::Submitted
        }
    );
    assert_eq!(
        checked(endpoint.sequencer_receipt(&first_hash)),
        Some(SequencerReceipt::Submitted {
            tx_hash: first_hash,
            nonce: 1
        })
    );
    assert_eq!(checked(endpoint.sequencer_receipt(&[0xff; 32])), None);
    let block = checked(checkpoint(&endpoint, 7));
    assert_eq!(block.number, 7);
    assert_eq!(block.hash, [0x91; 32]);
    let bundle = checked(proof_bundle(&endpoint, [0x52; 20], &[], 7));
    assert_eq!(bundle.landing_block, block);
    assert_eq!(bundle.landing_block.number, 7);
    let second = signed_activity(&actor, 2, 0x22);
    assert_eq!(
        submit_signed(&endpoint, &second),
        Err(refused(KnownResult::LengthLimit))
    );

    let node = checked(server.join());
    assert!(!node.fail_stopped);
    assert_eq!(node.authentication_refusals, 0);
    assert_eq!(node.queue, vec![first_hash]);
    let records = node.journal.records();
    assert_eq!(records.len(), 1);
    assert_eq!(records[0].tx_hash, first_hash);
    assert_eq!(records[0].raw_transaction, first);
    drop(node);
    let recovered = AdmissionJournal::recover(&admission_directory, EVIDENCE_NETWORK_ID);
    assert_eq!(recovered.len(), 1);
    assert_eq!(recovered[0].tx_hash, first_hash);
    assert_eq!(recovered[0].raw_transaction, first);
    let reopened = AdmissionJournal::open(&admission_directory, EVIDENCE_NETWORK_ID);
    assert!(reopened.contains(&first_hash));
}

#[test]
fn authentication_refusals_are_typed_and_consume_no_admission_capacity() {
    let receipt_signer = SigningKey::from_bytes(&[0x61; 32]);
    let actor = checked(EvmSigningKey::from_slice(&[0x62; 32]));
    let impostor = checked(EvmSigningKey::from_slice(&[0x63; 32]));
    let stranger = checked(EvmSigningKey::from_slice(&[0x64; 32]));
    let mut gate = support::evidence_gate(&receipt_signer);
    let mut node = AdmissionSequencer::new(&receipt_signer, "refused-admission");
    node.register_identity(b"did:layerx:actor", sender(&actor));
    node.register_identity(b"did:layerx:stranger", sender(&stranger));
    assert!(node
        .identities
        .remove(b"did:layerx:stranger".as_slice())
        .is_some());
    let admission_directory = node.journal.directory().to_path_buf();
    let (endpoint, config, server) = node.serve(8, false);
    let readiness = checked(NodeRpc::connect(&config));
    checked(gate.readiness_gate(&readiness));
    assert!(gate.evidence_authority().is_ok());
    assert!(gate.guard_write(|| ()).is_ok());
    let genuine = signed_activity(&actor, 1, 0x33);
    let mut malformed = genuine.clone();
    malformed[0] = 0x01;
    assert_eq!(
        submit_signed(&endpoint, &malformed),
        Err(refused(KnownResult::MalformedEnvelope))
    );
    let mut bad_signature = genuine.clone();
    let parity = bad_signature.len() - 67;
    assert_eq!(bad_signature[parity + 1], 0xa0);
    assert_eq!(bad_signature[parity + 34], 0xa0);
    bad_signature[parity] = 0x02;
    assert_eq!(
        submit_signed(&endpoint, &bad_signature),
        Err(refused(KnownResult::BadSignature))
    );
    let forged = signed_activity(&impostor, 1, 0x31);
    assert_eq!(
        submit_signed(&endpoint, &forged),
        Err(refused(KnownResult::BadSignature))
    );
    let (mut foreign, _) = checked(decode_transaction(&genuine));
    foreign.chain_id = 126;
    let wrong_network = checked(foreign.sign(&actor.to_bytes().into()));
    assert_eq!(
        submit_signed(&endpoint, &wrong_network),
        Err(refused(KnownResult::WrongNetwork))
    );
    let unknown = signed_activity(&stranger, 1, 0x32);
    assert_eq!(
        submit_signed(&endpoint, &unknown),
        Err(refused(KnownResult::UnknownDid))
    );
    let hash: [u8; 32] = Keccak256::digest(&genuine).into();
    assert_eq!(
        checked(submit_signed(&endpoint, &genuine)),
        Submission::Submitted {
            tx_hash: hash,
            rung: Rung::Submitted
        }
    );
    let node = checked(server.join());
    assert!(!node.fail_stopped);
    assert_eq!(node.authentication_refusals, 5);
    assert_eq!(node.queue.len(), 1);
    assert_eq!(node.journal.records().len(), 1);
    drop(node);
    assert_eq!(
        AdmissionJournal::recover(&admission_directory, EVIDENCE_NETWORK_ID).len(),
        1
    );
}
