#![allow(dead_code, unused_imports)]
use base64::Engine as _;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::ai_market::{
    AiClientError, CommitFinality, ObservedSnapshot, OperationJournal, OperationRecord,
    OperationState, VerifiedProgramState,
};
use layerx_client::px::{PxEndpoint, Uint256};
use layerx_client::rpc::{
    commit::{acquire_committed_header, verify_commit_material},
    history, payments,
    receipt::{lookup_authenticated_with_commit, Lookup, LookupContext},
    receipt_commit::{ReceiptCommitEvidence, SequenceBatch},
};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint, SequencerError};
use layerx_crypto::evm_transaction::{self, Eip1559Call};
use layerx_programs_registry::chain_proof::{
    decode::ProofOp,
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    merkle::{hash_from_byte_slices, SimpleProof},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofVerifier, Commit, VerifiedChainHead,
};
use layerx_wire::{
    encode::Encoder,
    hash::receipt_digest,
    limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION},
};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::{
    atomic::{AtomicBool, AtomicU64, Ordering},
    Arc,
};
use std::time::Duration;

const HEIGHT: u64 = 150;
const KEY_ID: [u8; 32] = [1; 32];
const PROGRAM: [u8; 32] = [11; 32];
const OWNER: [u8; 32] = [12; 32];
const ASSET: [u8; 32] = [13; 32];
const REFUND: [u8; 32] = [14; 32];
const MAX256: &str =
    "115792089237316195423570985008687907853269984665640564039457584007913129639935";
const OVER256: &str =
    "115792089237316195423570985008687907853269984665640564039457584007913129639936";
const POW128: &str = "340282366920938463463374607431768211456";

pub fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|e| panic!("{e:?}"))
}
pub fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}
pub fn hex(bytes: &[u8]) -> String {
    format!(
        "0x{}",
        bytes.iter().map(|b| format!("{b:02x}")).collect::<String>()
    )
}
pub fn unhex(text: &str) -> Vec<u8> {
    text.strip_prefix("0x")
        .unwrap_or(text)
        .as_bytes()
        .chunks_exact(2)
        .map(|p| must(u8::from_str_radix(must(std::str::from_utf8(p)), 16)))
        .collect()
}
fn string(value: &Value) -> &str {
    value.as_str().unwrap_or_else(|| panic!("string: {value}"))
}

struct Server {
    url: String,
    stop: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<usize>>,
}
impl Server {
    fn start(handler: impl Fn(Value) -> Value + Send + 'static) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let url = format!("http://{}/", must(listener.local_addr()));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let worker = std::thread::spawn(move || {
            let mut count = 0;
            while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(v) => v,
                    Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(e) => panic!("{e}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(5))));
                let mut reader = BufReader::new(stream);
                let mut length = 0;
                loop {
                    let mut line = String::new();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(v) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = must(v.trim().parse::<usize>());
                    }
                }
                let mut body = vec![0; length];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                assert_eq!(request["jsonrpc"], "2.0");
                let mut answer = handler(request.clone());
                answer["id"] = request["id"].clone();
                answer["jsonrpc"] = json!("2.0");
                let body = answer.to_string();
                let mut stream = reader.into_inner();
                must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",body.len()));
                must(stream.flush());
                count += 1;
            }
            count
        });
        Self {
            url,
            stop,
            worker: Some(worker),
        }
    }
    fn endpoint(&self) -> SequencerEndpoint {
        must(SequencerEndpoint::new(&self.url, &[]))
    }
    fn finish(mut self) -> usize {
        self.stop.store(true, Ordering::Release);
        must(
            self.worker
                .take()
                .unwrap_or_else(|| panic!("worker"))
                .join(),
        )
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}
fn identity() -> Value {
    json!({"evm_address":hex(&[0x12;20]),"pax_address":"pax1exampleaccount","layerx_did":format!("did:layerx:{}","61".repeat(32)),"layerx_account":"7c".repeat(32),"bound":true})
}
fn encoded_receipt(
    tx_hash: [u8; 32],
    success: bool,
    fee_denom: &str,
    payment_denom: &str,
    signature: Option<[u8; 64]>,
    sender: [u8; 20],
) -> Vec<u8> {
    let previous = [4; 32];
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend_from_slice(&previous);
    sequence.extend_from_slice(&7_u64.to_be_bytes());
    sequence.extend_from_slice(&tx_hash);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].fill(7);
    let vm_error = if success { "" } else { "execution reverted" };
    let mut summary = Encoder::new(256);
    must(summary.u64(21_000));
    must(summary.u64(42_000));
    must(summary.u64(1));
    must(summary.u32(1));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&sha256(vm_error.as_bytes())));
    must(summary.u32(2));
    must(summary.u32(1));
    must(summary.u8(must(u8::try_from(fee_denom.len()))));
    must(summary.fixed(fee_denom.as_bytes()));
    let mut log = vec![8; 20];
    log.push(1);
    log.extend_from_slice(&[9; 32]);
    log.extend_from_slice(&sha256(&[10, 11]));

    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    must(encoder.structure_header_version(0x5201, PROTOCOL_VERSION));
    must(encoder.u16(PROTOCOL_VERSION));
    must(encoder.bytes(&tx_hash, 32));
    must(encoder.u64(7));
    must(encoder.bytes(&previous, 32));
    must(encoder.bytes(&sha256(&sequence), 32));
    must(encoder.bytes(&sha256(&[0xab, 0xcd]), 32));
    must(encoder.i32(i32::from(!success)));
    must(encoder.sequence_length(2, 512));
    for (ordinal, kind, body) in [(0, 1, summary.finish()), (1, 3, log)] {
        must(encoder.u16(12));
        must(encoder.u16(ordinal));
        must(encoder.u16(0));
        must(encoder.u8(kind));
        must(encoder.u8(0));
        must(encoder.bytes(&[0; 32], 32));
        must(encoder.bytes(&body, 256));
    }
    must(encoder.u128(21_000));
    must(encoder.bytes(&height, 32));
    must(encoder.u16(12));
    must(encoder.u32(1));
    must(encoder.u32(0));
    must(encoder.u8(2));
    must(encoder.bytes(&sha256(payment_denom.as_bytes()), 32));
    must(encoder.u128(1));
    must(encoder.bytes(&from, 32));
    must(encoder.u128(21_002));
    must(encoder.u128(if success { 1 } else { 2 }));
    must(encoder.u64(41));
    must(encoder.bytes(&to, 32));
    must(encoder.u128(0));
    must(encoder.u128(u128::from(success)));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&KEY_ID, 32));
    must(encoder.u64(1_000));
    must(encoder.u8(u8::from(signature.is_some())));
    if let Some(signature) = signature {
        must(encoder.bytes(&signature, 64));
    }
    encoder.finish()
}

fn signed_receipt(
    tx_hash: [u8; 32],
    success: bool,
    sender: [u8; 20],
) -> (Value, Vec<u8>, ReceiptKeyEntry) {
    signed_receipt_with_denoms(tx_hash, success, "uhpx", "uhpx", sender)
}

fn signed_receipt_with_denoms(
    tx_hash: [u8; 32],
    success: bool,
    fee_denom: &str,
    payment_denom: &str,
    sender: [u8; 20],
) -> (Value, Vec<u8>, ReceiptKeyEntry) {
    let signer = SigningKey::from_bytes(&[8; 32]);
    let unsigned = encoded_receipt(tx_hash, success, fee_denom, payment_denom, None, sender);
    let signature = signer.sign(&must(receipt_digest(&unsigned))).to_bytes();
    let bytes = encoded_receipt(
        tx_hash,
        success,
        fee_denom,
        payment_denom,
        Some(signature),
        sender,
    );
    let key = ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: signer.verifying_key().to_bytes(),
        first_height: 100,
        last_height: 199,
    };
    let view = json!({
        "rung": "receipted", "txHash": hex(&tx_hash), "nonce": "0x29",
        "seq": "0x7", "height": "0x96", "index": "0x1", "timestamp": "0x3e8",
        "status": if success { "0x1" } else { "0x0" }, "returnData": "0xabcd",
        "logs": [{"address": hex(&[8; 20]), "topics": [hex(&[9; 32])],
            "data": "0x0a0b", "logIndex": "0x0"}],
        "gasUsed": "0x5208", "cumulativeGasUsed": "0xa410", "effectiveGasPrice": "0x1",
        "feeDenom": fee_denom, "paymentDenom": payment_denom, "feeAmount": "0x5208", "sponsor": hex(&[0; 20]),
        "from": hex(&sender), "to": hex(&[7; 20]), "contractAddress": hex(&[0; 20]),
        "vmError": if success { "" } else { "execution reverted" },
        "keyId": hex(&KEY_ID), "receipt": hex(&bytes), "signingKey": hex(&key.public_key)
    });
    (view, bytes, key)
}

pub struct NodeCommit {
    pub head: VerifiedChainHead,
    pub verifier: ChainProofVerifier,
    pub material: Value,
    pub validators: Value,
}
pub fn node_commit(height: u64, app_hash: [u8; 32], data_hash: [u8; 32]) -> NodeCommit {
    let signer = SigningKey::from_bytes(&[0x74; 32]);
    let validator = TrustedValidator::new(signer.verifying_key().to_bytes(), 100);
    let address = validator.address;
    let validators = must(ValidatorSet::new(vec![validator]));
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-repair-125".to_owned(),
        height: height as i64,
        time: HeaderTime {
            seconds: 1700000000,
            nanos: 0,
        },
        last_block_id: HeaderBlockId {
            hash: vec![1; 32],
            part_set_total: 1,
            part_set_hash: vec![2; 32],
        },
        last_commit_hash: vec![3; 32],
        data_hash: data_hash.to_vec(),
        validators_hash: validators.hash().to_vec(),
        next_validators_hash: validators.hash().to_vec(),
        consensus_hash: vec![6; 32],
        app_hash: app_hash.to_vec(),
        last_results_hash: vec![8; 32],
        evidence_hash: vec![9; 32],
        proposer_address: address.to_vec(),
    };
    let block_id = BlockId {
        hash: must(header.hash()).to_vec(),
        part_set_header: PartSetHeader {
            total: 1,
            hash: vec![10; 32],
        },
    };
    let timestamp = Timestamp {
        seconds: 1700000000,
        nanos: 0,
    };
    let signature = signer
        .sign(&must(vote_sign_bytes(
            &header.chain_id,
            header.height,
            0,
            Some(&block_id),
            timestamp,
        )))
        .to_bytes();
    let commit = Commit {
        height: header.height,
        round: 0,
        block_id: block_id.clone(),
        signatures: vec![CommitSig {
            flag: BlockIdFlag::Commit,
            validator_address: address.to_vec(),
            timestamp,
            signature: signature.to_vec(),
        }],
    };
    let verifier = ChainProofVerifier {
        chain_id: header.chain_id.clone(),
        validators,
    };
    let head = must(verifier.verify_head(&header, &commit));
    let material = json!({"height":height.to_string(),"signedHeader":{"header":{"version":{"block":"11","app":"1"},"chain_id":header.chain_id,"height":height.to_string(),"time":"2023-11-14T22:13:20Z","last_block_id":{"hash":hex(&[1;32]),"parts":{"total":1,"hash":hex(&[2;32])}},"last_commit_hash":hex(&[3;32]),"data_hash":hex(&data_hash),"validators_hash":hex(&verifier.validators.hash()),"next_validators_hash":hex(&verifier.validators.hash()),"consensus_hash":hex(&[6;32]),"app_hash":hex(&app_hash),"last_results_hash":hex(&[8;32]),"evidence_hash":hex(&[9;32]),"proposer_address":hex(&address)},"commit":{"height":height.to_string(),"round":0,"block_id":{"hash":hex(&head.header_hash()),"parts":{"total":1,"hash":hex(&[10;32])}},"signatures":[{"block_id_flag":2,"validator_address":hex(&address),"timestamp":"2023-11-14T22:13:20Z","signature":base64::engine::general_purpose::STANDARD.encode(signature)}]}}});
    assert_eq!(
        must(verify_commit_material(&material["signedHeader"], &verifier)),
        head
    );
    let validators = json!({"height":height.to_string(),"total":"1","validators":[{"address":hex(&address)[2..].to_owned(),"pubKey":{"type":"tendermint/PubKeyEd25519","value":base64::engine::general_purpose::STANDARD.encode(signer.verifying_key().to_bytes())},"votingPower":"100"}]});
    NodeCommit {
        head,
        verifier,
        material,
        validators,
    }
}

fn receipt_server(raw: Vec<u8>, view: Value, node: &NodeCommit) -> Server {
    let material = node.material.clone();
    let validators = node.validators.clone();
    let head = node.head.clone();
    Server::start(move |r| {
        let result = match string(&r["method"]) {
            "eth_sendRawTransaction" => {
                assert_eq!(r["params"], json!([hex(&raw)]));
                json!(hex(&evm_transaction::transaction_hash(&raw)))
            }
            "pax_sequencerReceipt" => {
                assert_eq!(
                    r["params"],
                    json!([hex(&evm_transaction::transaction_hash(&raw))])
                );
                view.clone()
            }
            "eth_getTransactionReceipt" => Value::Null,
            "px_getCommit" => {
                assert_eq!(r["params"], json!([head.height().to_string()]));
                material.clone()
            }
            "px_getValidators" => validators.clone(),
            "eth_getBlockByNumber" => {
                assert_eq!(r["params"], json!([format!("{:#x}", head.height()), true]));
                json!({"number":format!("{:#x}",head.height()),"hash":hex(&head.header_hash()),"parentHash":hex(&[1;32]),"timestamp":"0x6553f100","transactionsRoot":hex(&head.data_hash()),"receiptsRoot":hex(&[2;32]),"transactions":[]})
            }
            other => panic!("unexpected node request {other}"),
        };
        json!({"result":result})
    })
}

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);
struct Directory(std::path::PathBuf);
impl Directory {
    fn open() -> Self {
        let id = NEXT_DIRECTORY.fetch_add(1, Ordering::SeqCst);
        let path =
            std::env::temp_dir().join(format!("repair-22-31-221-{}-{id}", std::process::id()));
        must(std::fs::create_dir(&path));
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

pub fn receipt_sequence_and_height_survive_reopen() {
    let raw = must(
        Eip1559Call {
            chain_id: 125,
            nonce: 41,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1,
            gas_limit: 100000,
            to: [7; 20],
            data: vec![],
        }
        .sign(&[0x46; 32]),
    );
    let (call, signature) = must(evm_transaction::decode_signed(&raw));
    let sender = must(evm_transaction::recover_sender(&call, &signature));
    let tx_hash = evm_transaction::transaction_hash(&raw);
    let (view, canonical, key) = signed_receipt(tx_hash, true, sender);
    let mut chain = b"PAXEER/v1/sequence\0".to_vec();
    chain.extend_from_slice(&[4; 32]);
    chain.extend_from_slice(&7_u64.to_be_bytes());
    chain.extend_from_slice(&tx_hash);
    let resulting_root = sha256(&chain);
    let mut batch = SequenceBatch {
        height: 150,
        from_sequence: 7,
        to_sequence: 7,
        previous_root: [4; 32],
        resulting_root,
        activity_root: sha256(&tx_hash),
        key_id: KEY_ID,
        signature: [0; 64],
    };
    batch.signature = SigningKey::from_bytes(&[8; 32])
        .sign(&must(batch.signing_digest("paxeer-repair-125")))
        .to_bytes();
    let batch_bytes = must(batch.encode());
    let batch_hash = sha256(&batch_bytes);
    let data_root = hash_from_byte_slices(&[&batch_hash]);
    let node = node_commit(150, [0x33; 32], data_root);
    let proof = SimpleProof {
        total: 1,
        index: 0,
        leaf_hash: data_root,
        aunts: vec![],
    };
    let evidence = ReceiptCommitEvidence {
        verifier: &node.verifier,
        covering_height: 150,
        batch_transaction: &batch_bytes,
        transaction_proof: &proof,
        activity_ids: &[tx_hash],
    };
    let server = receipt_server(raw.clone(), view.clone(), &node);
    let endpoint = server.endpoint();
    let registry = must(layerx_types::payload::ModuleRegistry::new(&[]));
    let directory = Directory::open();
    let journal = must(OperationJournal::open(&directory.0));
    let mut record = must(OperationRecord::from_signed_transaction(
        raw.clone(),
        [0x55; 32],
        10000,
    ));
    must(journal.record_signed(&record));
    assert_eq!(
        must(journal.submit(&mut record, &endpoint)),
        OperationState::Pending
    );
    assert_eq!(
        must(journal.resolve(
            &mut record,
            &endpoint,
            LookupContext {
                receipt_keys: &[key.clone()]
            }
        )),
        OperationState::Executed
    );
    assert_eq!(record.global_sequence(), Some(7));
    assert_eq!(record.execution_height(), Some(150));
    assert_eq!(record.receipt_root(), Some(resulting_root));
    drop(journal);
    drop(record);
    let journal = must(OperationJournal::open(&directory.0));
    let mut record = must(journal.load(&tx_hash, &registry));
    assert_eq!(record.execution_height(), Some(150));
    assert_eq!(record.global_sequence(), Some(7));
    for height in [7, 149, 150, 999] {
        let unverified = must(CommitFinality::new(height, [2; 32], [3; 32], [4; 32]));
        assert_eq!(
            journal.finalize(&mut record, &unverified),
            Err(AiClientError::FinalityMismatch)
        );
        assert_eq!(record.state(), OperationState::Executed);
        assert_eq!(must(journal.load(&tx_hash, &registry)), record);
    }
    let lookup = must(lookup_authenticated_with_commit(
        &endpoint,
        tx_hash,
        LookupContext {
            receipt_keys: &[key.clone()],
        },
        &evidence,
    ));
    let Lookup::Verified(receipt) = lookup else {
        panic!("receipt")
    };
    assert_eq!(receipt.canonical_bytes(), canonical);
    let finality = must(CommitFinality::from_verified_receipt(&receipt));
    // Inclusion alone is insufficient: a valid batch commit with a foreign receipt root refuses.
    let mut wrong = batch.clone();
    wrong.resulting_root = [99; 32];
    wrong.signature = SigningKey::from_bytes(&[8; 32])
        .sign(&must(wrong.signing_digest("paxeer-repair-125")))
        .to_bytes();
    let wrong_bytes = must(wrong.encode());
    let wrong_data = hash_from_byte_slices(&[&sha256(&wrong_bytes)]);
    let wrong_node = node_commit(150, [0x33; 32], wrong_data);
    let wrong_proof = SimpleProof {
        total: 1,
        index: 0,
        leaf_hash: wrong_data,
        aunts: vec![],
    };
    let wrong_server = receipt_server(raw.clone(), view.clone(), &wrong_node);
    let wrong_evidence = ReceiptCommitEvidence {
        verifier: &wrong_node.verifier,
        covering_height: 150,
        batch_transaction: &wrong_bytes,
        transaction_proof: &wrong_proof,
        activity_ids: &[tx_hash],
    };
    assert!(matches!(
        lookup_authenticated_with_commit(
            &wrong_server.endpoint(),
            tx_hash,
            LookupContext {
                receipt_keys: &[key.clone()]
            },
            &wrong_evidence
        ),
        Err(layerx_client::rpc::receipt::ReceiptError::Coverage(_))
    ));
    assert_eq!(wrong_server.finish(), 5);
    let old_node = node_commit(7, [0x33; 32], data_root);
    let old_server = receipt_server(raw.clone(), view.clone(), &old_node);
    let old_evidence = ReceiptCommitEvidence {
        verifier: &old_node.verifier,
        covering_height: 7,
        batch_transaction: &batch_bytes,
        transaction_proof: &proof,
        activity_ids: &[tx_hash],
    };
    assert!(matches!(
        lookup_authenticated_with_commit(
            &old_server.endpoint(),
            tx_hash,
            LookupContext {
                receipt_keys: &[key.clone()]
            },
            &old_evidence
        ),
        Err(layerx_client::rpc::receipt::ReceiptError::FieldMismatch(
            "coveringBlock"
        ))
    ));
    assert_eq!(old_server.finish(), 2);
    let second_directory = Directory::open();
    let second_journal = must(OperationJournal::open(&second_directory.0));
    let mut second = must(OperationRecord::from_signed_transaction(
        raw.clone(),
        [0x55; 32],
        10000,
    ));
    must(second_journal.submit(&mut second, &endpoint));
    assert_eq!(
        must(second_journal.resolve_with_commit(
            &mut second,
            &endpoint,
            LookupContext {
                receipt_keys: &[key]
            },
            &evidence
        )),
        OperationState::Executed
    );
    drop(second_journal);
    drop(second);
    let second_journal = must(OperationJournal::open(&second_directory.0));
    let mut second = must(second_journal.load(&tx_hash, &registry));
    assert_eq!(second.execution_height(), Some(150));
    assert_eq!(
        must(second_journal.finalize(&mut second, &finality)),
        OperationState::Finalized
    );
    assert_eq!(
        must(journal.finalize(&mut record, &finality)),
        OperationState::Finalized
    );
    let bytes = must(record.encode());
    assert_eq!(&bytes[..8], b"PAXAIOP3");
    drop(journal);
    drop(record);
    let journal = must(OperationJournal::open(&directory.0));
    let record = must(journal.load(&tx_hash, &registry));
    assert_eq!(record.state(), OperationState::Finalized);
    assert_eq!(record.execution_height(), Some(150));
    assert_eq!(record.checkpoint(), Some(node.head.header_hash()));
    assert_eq!(record.signed_bytes(), raw);
    let mut finalized_record = record.clone();
    assert_eq!(
        journal.finalize(&mut finalized_record, &finality),
        Err(AiClientError::InvalidTransition {
            from: OperationState::Finalized
        })
    );
    // A real older-format record carries the signed transaction but none of the new evidence.
    let end = bytes.len() - 4 - raw.len();
    let start = end - (9 + 33 + 33 + 153);
    let mut legacy = bytes[..start].to_vec();
    legacy[..8].copy_from_slice(b"PAXAIOP2");
    legacy.extend_from_slice(&bytes[end..]);
    must(std::fs::write(journal.record_path(&tx_hash), legacy));
    let mut migrated = must(journal.load(&tx_hash, &registry));
    assert_eq!(migrated.state(), OperationState::Executed);
    assert_eq!(migrated.global_sequence(), Some(7));
    assert_eq!(migrated.execution_height(), None);
    assert_eq!(migrated.receipt_root(), None);
    assert_eq!(migrated.checkpoint(), None);
    assert_eq!(
        journal.finalize(&mut migrated, &finality),
        Err(AiClientError::FinalityMismatch)
    );
    assert_eq!(server.finish(), 14);
}
pub fn market_state(rubric: u8) -> Vec<u8> {
    use layerx_programs_ai_market::{
        codec::{decode_envelope, derive_market, encode_envelope, Envelope},
        dispatch,
        policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
        registry::{derive_rewards_account, F01_SECTION_CAP},
        registry_ops::{self, Outcome},
        state::{self, Section},
        types::{
            AssetId, Authentication, ChainDomain, Digest32, Presence, PrincipalId, ProgramId,
            RequestId, RubricDigest,
        },
        MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES,
    };
    let chain = must(ChainDomain::new(
        layerx_programs_ai_market::MERGED_CHAIN_DOMAIN,
    ));
    let program = must(ProgramId::new(PROGRAM));
    let policy = must(TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: must(Digest32::new([1; 32])),
            dataset_artifact: [2; 32],
            benchmark_suite: must(Digest32::new([3; 32])),
            rubric: must(RubricDigest::new([rubric; 32])),
            task_schema: must(Digest32::new([5; 32])),
            result_schema: must(Digest32::new([6; 32])),
            service_terms: must(Digest32::new([7; 32])),
        },
        100,
        1,
    ));
    let mut encoded_policy = vec![0; TASK_POLICY_BYTES];
    must(policy.encode(&mut encoded_policy));
    let rewards = must(derive_rewards_account(program, must(AssetId::new(ASSET))));
    let mut payload = OWNER.to_vec();
    payload.extend_from_slice(&ASSET);
    payload.extend_from_slice(rewards.as_bytes());
    payload.extend_from_slice(&REFUND);
    payload.push(0);
    payload.extend_from_slice(&encoded_policy);
    payload.extend_from_slice(&[16; 32]);
    let envelope = Envelope {
        operation: dispatch::CREATE,
        chain,
        program,
        market: must(derive_market(chain, program)),
        actor: must(PrincipalId::new(OWNER)),
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: 1,
        expiry: 1000000,
        request: must(RequestId::new([1; 32])),
        payload: &payload,
        authentication: Authentication::Native,
    };
    let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
    let length = must(encode_envelope(&envelope, &mut encoded));
    let validated = must(decode_envelope(&encoded[..length]));
    let context = registry_ops::CallContext {
        chain,
        program,
        principal: must(PrincipalId::new(OWNER)),
        height: 1,
    };
    let mut section = vec![0; F01_SECTION_CAP];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let Outcome::Applied { state, .. } = must(registry_ops::apply(
        &context,
        None,
        &validated,
        &mut section,
        &mut event,
    )) else {
        panic!("fresh state")
    };
    let mut out = vec![0; must(state.encoded_len())];
    let mut scratch = vec![0; Section::Control.payload_cap()];
    let length = must(state::encode_shared_state(&state, &mut out, &mut scratch));
    out.truncate(length);
    out
}
fn varint(mut value: usize) -> Vec<u8> {
    let mut out = vec![];
    while value >= 128 {
        out.push((value as u8 & 127) | 128);
        value >>= 7;
    }
    out.push(value as u8);
    out
}
fn proto_bytes(field: u8, value: &[u8], out: &mut Vec<u8>) {
    out.push((field << 3) | 2);
    out.extend(varint(value.len()));
    out.extend_from_slice(value);
}
fn membership(key: &[u8], value: &[u8], prefix: &[u8]) -> (Vec<u8>, [u8; 32]) {
    let mut leaf = vec![8, 1, 24, 1, 32, 1];
    proto_bytes(5, prefix, &mut leaf);
    let mut existence = vec![];
    proto_bytes(1, key, &mut existence);
    proto_bytes(2, value, &mut existence);
    proto_bytes(3, &leaf, &mut existence);
    let mut commitment = vec![];
    proto_bytes(1, &existence, &mut commitment);
    let mut preimage = prefix.to_vec();
    preimage.extend(varint(key.len()));
    preimage.extend_from_slice(key);
    preimage.extend(varint(32));
    preimage.extend_from_slice(&sha256(value));
    (commitment, sha256(&preimage))
}
pub fn namespace_proof(state: &[u8]) -> (Vec<u8>, Vec<u8>, Vec<ProofOp>, [u8; 32], [u8; 32]) {
    let mut manifest = vec![0, 1];
    manifest.extend_from_slice(&1_u32.to_be_bytes());
    let key = layerx_programs_ai_market::SHARED_STATE_KEY;
    manifest.extend_from_slice(&(key.len() as u16).to_be_bytes());
    manifest.extend_from_slice(key);
    manifest.extend_from_slice(&sha256(state));
    manifest.extend_from_slice(&(state.len() as u32).to_be_bytes());
    let mut head = vec![0, 1];
    head.extend_from_slice(&1_u32.to_be_bytes());
    head.extend_from_slice(&sha256(&manifest));
    let mut key = vec![1, 0, 9];
    key.extend_from_slice(b"progstor");
    key.extend_from_slice(&PROGRAM);
    key.push(1);
    let (proof, store_root) = membership(&key, &head, &[0, 0, 2]);
    let (outer, app_hash) = membership(b"lxphost", &store_root, &[0]);
    let ops = vec![
        ProofOp {
            op_type: "ics23:iavl".to_owned(),
            key,
            data: proof,
        },
        ProofOp {
            op_type: "ics23:simple".to_owned(),
            key: b"lxphost".to_vec(),
            data: outer,
        },
    ];
    (head, manifest, ops, store_root, app_hash)
}
pub fn capture_market(
    state: &[u8],
    root: [u8; 32],
    block: [u8; 32],
    height: u64,
) -> ObservedSnapshot {
    use layerx_programs_ai_market::{
        queries::{read_state_chunk, ReadProof},
        types::{ChainDomain, Digest32, ProgramId},
    };
    let proof = ReadProof {
        chain: must(ChainDomain::new(
            layerx_programs_ai_market::MERGED_CHAIN_DOMAIN,
        )),
        program: must(ProgramId::new(PROGRAM)),
        native_state_root: must(Digest32::new(root)),
        observed_sequence: 7,
        execution_height: height,
        block_hash: must(Digest32::new(block)),
    };
    let mut chunks = vec![];
    let mut offset = 0_u32;
    let mut revision = 0_u64;
    let mut pinned = [0; 32];
    loop {
        let mut request = revision.to_be_bytes().to_vec();
        request.extend_from_slice(&pinned);
        request.extend_from_slice(&offset.to_be_bytes());
        request.extend_from_slice(&8192_u16.to_be_bytes());
        let mut out = vec![0; 8244];
        let length = must(read_state_chunk(state, &request, &mut out));
        out.truncate(length);
        let chunk = must(layerx_programs_ai_market::codec::decode_chunk_response(
            &out,
        ));
        revision = chunk.revision;
        pinned = chunk.digest.bytes();
        let total = chunk.total_bytes;
        chunks.push((proof, out));
        offset = offset.saturating_add(8192);
        if offset >= total {
            break;
        }
    }
    must(ObservedSnapshot::capture(&chunks))
}

pub fn market_finality_requires_same_root_node_evidence() {
    use layerx_programs_ai_market::{queries::QueryError, types::ProgramId};
    let state = market_state(4);
    let (head_value, manifest, ops, store_root, app_hash) = namespace_proof(&state);
    let node = node_commit(151, app_hash, [0x66; 32]);
    let server = receipt_server(vec![], Value::Null, &node);
    let head = must(acquire_committed_header(
        &server.endpoint(),
        151,
        &node.verifier,
    ));
    assert_eq!(head, node.head);
    let program = must(ProgramId::new(PROGRAM));
    let witness = must(VerifiedProgramState::verify(
        &head,
        &node.verifier.chain_id,
        program,
        &head_value,
        &manifest,
        &state,
        &ops,
    ));
    let finality = must(CommitFinality::from_verified_program(&head, &witness));
    let snapshot = capture_market(&state, store_root, head.header_hash(), 150);
    let unverified = must(CommitFinality::new(
        151,
        head.header_hash(),
        head.app_hash(),
        finality.store_proof_digest().bytes(),
    ));
    assert_eq!(
        snapshot.clone().finalize(Some(&unverified), 5000),
        Err(AiClientError::Query(QueryError::FinalityUnavailable))
    );
    let finalized = must(snapshot.finalize(Some(&finality), 5000));
    assert_eq!(*finalized.commit(), finality);
    assert_eq!(
        finalized.binding().rank,
        layerx_programs_ai_market::queries::FINALIZED_RANK
    );
    assert_eq!(
        capture_market(&state, [0xee; 32], head.header_hash(), 150).finalize(Some(&finality), 5000),
        Err(AiClientError::Query(QueryError::BindingMismatch))
    );
    assert_eq!(
        capture_market(&state, store_root, [0xee; 32], 150).finalize(Some(&finality), 5000),
        Err(AiClientError::Query(QueryError::BindingMismatch))
    );
    assert_eq!(
        capture_market(&state, store_root, head.header_hash(), 151).finalize(Some(&finality), 5000),
        Err(AiClientError::Query(QueryError::BindingMismatch))
    );
    let foreign = market_state(9);
    assert_eq!(
        VerifiedProgramState::verify(
            &head,
            &node.verifier.chain_id,
            program,
            &head_value,
            &manifest,
            &foreign,
            &ops
        ),
        Err(AiClientError::FinalityMismatch)
    );
    assert_eq!(
        VerifiedProgramState::verify(
            &head,
            "foreign-chain",
            program,
            &head_value,
            &manifest,
            &state,
            &ops
        ),
        Err(AiClientError::FinalityMismatch)
    );
    let foreign_node = node_commit(151, [0xaa; 32], [0x66; 32]);
    assert_eq!(
        VerifiedProgramState::verify(
            &foreign_node.head,
            &node.verifier.chain_id,
            program,
            &head_value,
            &manifest,
            &state,
            &ops
        ),
        Err(AiClientError::FinalityMismatch)
    );
    assert_eq!(
        CommitFinality::from_verified_program(&foreign_node.head, &witness),
        Err(AiClientError::FinalityMismatch)
    );
    let mut damaged = node.material.clone();
    damaged["signedHeader"]["commit"]["signatures"][0]["signature"] =
        json!(base64::engine::general_purpose::STANDARD.encode([0; 64]));
    assert!(verify_commit_material(&damaged["signedHeader"], &node.verifier).is_err());
    assert_eq!(server.finish(), 2);
}

/// A market snapshot whose finality comes from the node commit and namespace verifier.
pub fn verified_market(state: &[u8], execution_height: u64) -> (ObservedSnapshot, CommitFinality) {
    let (head_value, manifest, ops, store_root, app_hash) = namespace_proof(state);
    let node = node_commit(execution_height + 1, app_hash, [0x66; 32]);
    let server = receipt_server(vec![], Value::Null, &node);
    let head = must(acquire_committed_header(
        &server.endpoint(),
        execution_height + 1,
        &node.verifier,
    ));
    let witness = must(VerifiedProgramState::verify(
        &head,
        &node.verifier.chain_id,
        must(layerx_programs_ai_market::types::ProgramId::new(PROGRAM)),
        &head_value,
        &manifest,
        state,
        &ops,
    ));
    let finality = must(CommitFinality::from_verified_program(&head, &witness));
    let snapshot = capture_market(state, store_root, head.header_hash(), execution_height);
    assert_eq!(server.finish(), 2);
    (snapshot, finality)
}
