//! Real MemIAVL/SDK and Comet/node codec producer -> HTTP -> repaired RPC readers.
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::path::Path;
use std::process::Command;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, OnceLock,
};
use std::thread::JoinHandle;
use std::time::Duration;

use base64::Engine as _;
use layerx_client::rpc::commit::{acquire_committed_header, decode_validators, CommitError};
use layerx_client::rpc::evidence::{self, EvidenceError};
use layerx_client::rpc::module_state::{self, ModuleReadContext, ModuleReadError};
use layerx_client::rpc::receipt::{
    self, AuthenticatedReceipt, Lookup, LookupContext, ReceiptError, Rung,
};
use layerx_client::rpc::receipt_commit::{CoverageError, ReceiptCommitEvidence, SequenceBatch};
use layerx_client::sequencer::{decode_receipt_keys, SequencerEndpoint};
use layerx_client::storage_witness::StorageWitnessError;
use layerx_programs_registry::chain_proof::{merkle::SimpleProof, ChainProofVerifier};
use serde_json::{json, Value};

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}
fn text(value: &Value) -> &str {
    value.as_str().unwrap_or_else(|| panic!("string: {value}"))
}
fn number(value: &Value) -> u64 {
    value.as_u64().unwrap_or_else(|| must(text(value).parse()))
}
fn unhex(value: &Value) -> Vec<u8> {
    let digits = text(value)
        .strip_prefix("0x")
        .unwrap_or_else(|| text(value));
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16)))
        .collect()
}
fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    must(unhex(value).try_into())
}
fn producer() -> &'static Value {
    static PRODUCER: OnceLock<Value> = OnceLock::new();
    PRODUCER.get_or_init(|| {
        let root = must(
            Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("../../../..")
                .canonicalize(),
        );
        let output = must(
            Command::new("go")
                .current_dir(root)
                .args([
                    "run",
                    "./services/agent/crates/layerx-client/src/rpc/repair_22_31_222_producer",
                ])
                .output(),
        );
        assert!(
            output.status.success(),
            "real producer: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        must(serde_json::from_slice(&output.stdout))
    })
}
fn verifier(record: &Value) -> ChainProofVerifier {
    ChainProofVerifier {
        chain_id: text(&record["material"]["signedHeader"]["header"]["chain_id"]).to_owned(),
        validators: must(decode_validators(&record["validators"])),
    }
}

#[derive(Clone, Copy, Debug)]
enum Damage {
    None,
    Slot,
    Value,
    StoreRoot,
    AppHash,
    Signature,
    MissingWitness,
    MissingCommit,
    BlockHash,
}

struct Crossing {
    endpoint: SequencerEndpoint,
    stop: Arc<AtomicBool>,
    server: Option<JoinHandle<()>>,
}
impl Crossing {
    fn open(record: Value, damage: Damage) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        must(listener.set_nonblocking(true));
        let endpoint = must(SequencerEndpoint::new(
            &format!("http://{}/", must(listener.local_addr())),
            &[],
        ));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let server = std::thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(stream) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("{error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(10))));
                let mut reader = BufReader::new(stream);
                let mut length = 0;
                loop {
                    let mut line = String::new();
                    assert_ne!(must(reader.read_line(&mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = must(value.trim().parse::<usize>());
                    }
                }
                let mut body = vec![0; length];
                must(reader.read_exact(&mut body));
                let request: Value = must(serde_json::from_slice(&body));
                let method = text(&request["method"]);
                let params = &request["params"];
                let height = number(&record["height"]);
                let result = match method {
                    "px_getCommit" => {
                        assert_eq!(params, &json!([height.to_string()]));
                        let mut material = record["material"].clone();
                        if matches!(damage, Damage::Signature) {
                            material["signedHeader"]["commit"]["signatures"][0]["signature"] =
                                json!(base64::engine::general_purpose::STANDARD.encode([0_u8; 64]));
                        }
                        material
                    }
                    "px_getValidators" => {
                        assert_eq!(params, &json!([height.to_string()]));
                        record["validators"].clone()
                    }
                    "eth_getBlockByNumber" => {
                        assert_eq!(params[0], format!("{height:#x}"));
                        if params[1] == false {
                            let mut root = fixed::<32>(&record["appHash"]);
                            if matches!(damage, Damage::AppHash) {
                                root[0] ^= 1;
                            }
                            json!({"number":format!("{height:#x}"),"stateRoot":format!("0x{}",root.iter().map(|v|format!("{v:02x}")).collect::<String>())})
                        } else {
                            let mut block = record["block"].clone();
                            if matches!(damage, Damage::BlockHash) {
                                block["hash"] = json!(format!("0x{}", "ff".repeat(32)));
                            }
                            block
                        }
                    }
                    "eth_getProof" => {
                        assert_eq!(params[0], record["address"]);
                        assert_eq!(params[2], format!("{height:#x}"));
                        let mut result = record["responses"][0].clone();
                        result["hexValues"] = json!([]);
                        result["storageProof"] = json!([]);
                        for slot in params[1].as_array().unwrap_or_else(|| panic!("slots")) {
                            let index = record["slots"]
                                .as_array()
                                .unwrap_or_else(|| panic!("slots"))
                                .iter()
                                .position(|known| known == slot)
                                .unwrap_or_else(|| panic!("unknown slot"));
                            let index = if matches!(damage, Damage::Slot) {
                                1 - index
                            } else {
                                index
                            };
                            result["hexValues"]
                                .as_array_mut()
                                .unwrap_or_else(|| panic!("values"))
                                .push(record["responses"][index]["hexValues"][0].clone());
                            result["storageProof"]
                                .as_array_mut()
                                .unwrap_or_else(|| panic!("proofs"))
                                .push(record["responses"][index]["storageProof"][0].clone());
                        }
                        match damage {
                            Damage::Value => result["hexValues"][0] = json!("ff".repeat(32)),
                            Damage::StoreRoot => {
                                let encoded = &result["storageProof"][0]["ops"][1]["data"];
                                let mut data = must(
                                    base64::engine::general_purpose::STANDARD.decode(text(encoded)),
                                );
                                let root = unhex(&record["storeRoot"]);
                                let offset = data
                                    .windows(root.len())
                                    .position(|part| part == root)
                                    .unwrap_or_else(|| panic!("root"));
                                data[offset] ^= 1;
                                result["storageProof"][0]["ops"][1]["data"] =
                                    json!(base64::engine::general_purpose::STANDARD.encode(data));
                            }
                            Damage::MissingWitness => result["storageProof"] = json!([]),
                            _ => {}
                        }
                        result
                    }
                    "pax_sequencerReceipt" => {
                        assert_eq!(params, &json!([record["view"]["txHash"]]));
                        record["view"].clone()
                    }
                    "eth_getTransactionReceipt" => record["landing"].clone(),
                    other => panic!("unrequested producer method {other}"),
                };
                let mut answer = json!({"jsonrpc":"2.0","id":request["id"],"result":result});
                if matches!(damage, Damage::MissingCommit) && method == "px_getCommit" {
                    answer = json!({"jsonrpc":"2.0","id":request["id"],"error":{"code":-32601,"message":"commit unavailable"}});
                }
                let answer = answer.to_string();
                let mut stream = reader.into_inner();
                must(write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",answer.len()));
                must(stream.flush());
            }
        });
        Self {
            endpoint,
            stop,
            server: Some(server),
        }
    }
}
impl Drop for Crossing {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(server) = self.server.take() {
            must(server.join());
        }
    }
}
fn context(record: &Value) -> ModuleReadContext {
    ModuleReadContext {
        precompile: fixed(&record["address"]),
        committed_height: number(&record["height"]),
        block_hash: fixed(&record["hash"]),
        app_hash: fixed(&record["appHash"]),
    }
}
fn authenticated(value: Lookup) -> AuthenticatedReceipt {
    let Lookup::Verified(receipt) = value else {
        panic!("verified receipt")
    };
    receipt
}
fn inclusion(record: &Value) -> SimpleProof {
    SimpleProof {
        total: number(&record["proof"]["total"]) as i64,
        index: number(&record["proof"]["index"]) as i64,
        leaf_hash: must(
            base64::engine::general_purpose::STANDARD.decode(text(&record["proof"]["leaf_hash"])),
        )
        .try_into()
        .unwrap_or_else(|_| panic!("leaf")),
        aunts: record["proof"]["aunts"]
            .as_array()
            .map(|items| {
                items
                    .iter()
                    .map(|v| {
                        must(
                            must(base64::engine::general_purpose::STANDARD.decode(text(v)))
                                .try_into(),
                        )
                    })
                    .collect()
            })
            .unwrap_or_default(),
    }
}

#[test]
fn f02_module_state_returns_only_the_derived_slot_witness() {
    let record = &producer()["storage"];
    let crossing = Crossing::open(record.clone(), Damage::None);
    for (index, key) in [b"record-A".as_slice(), b"record-B".as_slice()]
        .into_iter()
        .enumerate()
    {
        assert_eq!(
            must(module_state::module_slot(fixed(&record["address"]), key)),
            fixed::<32>(&record["slots"][index])
        );
        let value = must(module_state::module_state(
            &crossing.endpoint,
            key,
            context(record),
        ));
        assert_eq!(value.canonical_bytes(), unhex(&record["values"][index]));
        assert_eq!(value.context(), &context(record));
        assert_eq!(
            must(module_state::module_state_with_history(
                &crossing.endpoint,
                key,
                context(record)
            )),
            value
        );
    }
}

#[test]
fn f02_module_state_refuses_slot_value_store_root_and_app_hash_substitution() {
    let record = &producer()["storage"];
    for damage in [
        Damage::Slot,
        Damage::Value,
        Damage::StoreRoot,
        Damage::Signature,
        Damage::MissingWitness,
    ] {
        let crossing = Crossing::open(record.clone(), damage);
        let error = module_state::module_state(&crossing.endpoint, b"record-A", context(record))
            .err()
            .unwrap_or_else(|| panic!("accepted {damage:?}"));
        match damage {
            Damage::Slot => assert_eq!(error, ModuleReadError::Witness(StorageWitnessError::Key)),
            Damage::Value => {
                assert_eq!(error, ModuleReadError::Witness(StorageWitnessError::Value))
            }
            Damage::StoreRoot => assert_eq!(
                error,
                ModuleReadError::Witness(StorageWitnessError::StoreRoot)
            ),
            Damage::Signature => assert!(matches!(
                error,
                ModuleReadError::Commit(CommitError::Proof(_))
            )),
            Damage::MissingWitness => assert!(matches!(error, ModuleReadError::Rpc(_))),
            _ => panic!("case"),
        }
    }
    let crossing = Crossing::open(record.clone(), Damage::None);
    let mut foreign = context(record);
    foreign.app_hash[0] ^= 1;
    assert_eq!(
        module_state::module_state(&crossing.endpoint, b"record-A", foreign),
        Err(ModuleReadError::Commit(CommitError::Invalid))
    );
    let mut foreign = context(record);
    foreign.block_hash[0] ^= 1;
    assert_eq!(
        module_state::module_state(&crossing.endpoint, b"record-A", foreign),
        Err(ModuleReadError::Commit(CommitError::Invalid))
    );
}

#[test]
fn f03_proof_bundle_retains_every_requested_slot_identity_and_value() {
    let record = &producer()["storage"];
    let crossing = Crossing::open(record.clone(), Damage::None);
    let head = must(acquire_committed_header(
        &crossing.endpoint,
        number(&record["height"]),
        &verifier(record),
    ));
    let address = fixed(&record["address"]);
    let slots = [fixed(&record["slots"][0]), fixed(&record["slots"][1])];
    for bundle in [
        must(evidence::proof_bundle(
            &crossing.endpoint,
            address,
            &slots,
            &head,
        )),
        must(evidence::proof_bundle_with_history(
            &crossing.endpoint,
            address,
            &slots,
            &head,
        )),
    ] {
        assert_eq!(bundle.landing_block.hash, head.header_hash());
        assert_eq!(bundle.committed_header, head);
        assert_eq!(bundle.witnesses.len(), 2);
        for (index, witness) in bundle.witnesses.iter().enumerate() {
            assert_eq!(witness.address, address);
            assert_eq!(witness.slot, slots[index]);
            assert_eq!(witness.value.to_vec(), unhex(&record["values"][index]));
        }
    }
    let first = must(evidence::proof_bundle(
        &crossing.endpoint,
        address,
        &slots[..1],
        &head,
    ));
    let second = must(evidence::proof_bundle(
        &crossing.endpoint,
        address,
        &slots[1..],
        &head,
    ));
    assert_ne!(first, second);
    assert_eq!(
        evidence::proof_bundle(
            &crossing.endpoint,
            address,
            &slots,
            number(&record["height"])
        ),
        Err(EvidenceError::MissingCommittedHeader)
    );
}

#[test]
fn f03_proof_bundle_refuses_absent_mismatched_and_uncommitted_material() {
    let record = &producer()["storage"];
    let crossing = Crossing::open(record.clone(), Damage::None);
    let head = must(acquire_committed_header(
        &crossing.endpoint,
        number(&record["height"]),
        &verifier(record),
    ));
    for damage in [
        Damage::Slot,
        Damage::Value,
        Damage::StoreRoot,
        Damage::MissingWitness,
        Damage::AppHash,
        Damage::BlockHash,
    ] {
        let crossing = Crossing::open(record.clone(), damage);
        assert!(
            evidence::proof_bundle(
                &crossing.endpoint,
                fixed(&record["address"]),
                &[fixed(&record["slots"][0])],
                &head
            )
            .is_err(),
            "{damage:?}"
        );
    }
}

#[test]
fn f05_receipt_requires_commit_and_signed_sequence_root_coverage() {
    let record = &producer()["receipt"];
    let pins = must(decode_receipt_keys(&json!({"keys":[record["key"]]})));
    let context = LookupContext {
        receipt_keys: &pins,
    };
    let tx_hash = fixed(&record["view"]["txHash"]);
    let trust = verifier(record);
    let batch = unhex(&record["batch"]);
    let proof = inclusion(record);
    let ids: Vec<[u8; 32]> = record["activityIds"]
        .as_array()
        .unwrap_or_else(|| panic!("ids"))
        .iter()
        .map(fixed)
        .collect();
    let evidence = ReceiptCommitEvidence {
        verifier: &trust,
        covering_height: number(&record["height"]),
        batch_transaction: &batch,
        transaction_proof: &proof,
        activity_ids: &ids,
    };
    let crossing = Crossing::open(record.clone(), Damage::None);
    let unsigned = authenticated(must(receipt::lookup_authenticated(
        &crossing.endpoint,
        tx_hash,
        context,
    )));
    assert_eq!(unsigned.rung(), Rung::Receipted);
    assert!(unsigned.landing().is_some());
    assert_eq!(unsigned.committed_header(), None);
    let committed = authenticated(must(receipt::lookup_authenticated_with_commit(
        &crossing.endpoint,
        tx_hash,
        context,
        &evidence,
    )));
    assert_eq!(committed.rung(), Rung::Committed);
    assert!(number(&record["height"]) > committed.height());
    assert_eq!(
        committed.committed_header().map(|head| head.header_hash()),
        Some(fixed(&record["hash"]))
    );
    assert_eq!(
        committed.canonical_bytes(),
        unhex(&record["view"]["receipt"])
    );
    for damage in [Damage::MissingCommit, Damage::Signature, Damage::BlockHash] {
        let crossing = Crossing::open(record.clone(), damage);
        let answer = receipt::lookup_authenticated_with_commit(
            &crossing.endpoint,
            tx_hash,
            context,
            &evidence,
        );
        if matches!(damage, Damage::MissingCommit) {
            assert_eq!(authenticated(must(answer)).rung(), Rung::Receipted);
        } else {
            assert!(answer.is_err(), "{damage:?}");
        }
    }
    for (kind, expected) in [
        ("sequence", CoverageError::Sequence),
        ("root", CoverageError::StateRoot),
    ] {
        let variant = &producer()["variants"][kind];
        let crossing = Crossing::open(variant.clone(), Damage::None);
        let batch = unhex(&variant["batch"]);
        let proof = inclusion(variant);
        let invalid = ReceiptCommitEvidence {
            batch_transaction: &batch,
            transaction_proof: &proof,
            ..evidence
        };
        assert_eq!(
            receipt::lookup_authenticated_with_commit(
                &crossing.endpoint,
                tx_hash,
                context,
                &invalid
            ),
            Err(ReceiptError::Coverage(expected))
        );
    }
    let mut wrong_ids = ids.clone();
    wrong_ids[0][0] ^= 1;
    let wrong = ReceiptCommitEvidence {
        activity_ids: &wrong_ids,
        ..evidence
    };
    assert!(matches!(
        receipt::lookup_authenticated_with_commit(&crossing.endpoint, tx_hash, context, &wrong),
        Err(ReceiptError::Coverage(CoverageError::StateRoot))
    ));
    let mut wrong_proof = proof.clone();
    wrong_proof.leaf_hash[0] ^= 1;
    let wrong = ReceiptCommitEvidence {
        transaction_proof: &wrong_proof,
        ..evidence
    };
    assert!(matches!(
        receipt::lookup_authenticated_with_commit(&crossing.endpoint, tx_hash, context, &wrong),
        Err(ReceiptError::Coverage(CoverageError::Inclusion))
    ));
    let mut foreign = record.clone();
    foreign["view"]["rung"] = json!("committed");
    foreign["view"]["blockNumber"] = json!(format!("{:#x}", evidence.covering_height));
    foreign["view"]["blockHash"] = json!(format!("0x{}", "ff".repeat(32)));
    let crossing = Crossing::open(foreign, Damage::None);
    assert_eq!(
        receipt::lookup_authenticated_with_commit(&crossing.endpoint, tx_hash, context, &evidence),
        Err(ReceiptError::FieldMismatch("blockHash"))
    );
}

#[test]
fn f05_sequence_batch_round_trips_the_node_producer_codec() {
    let record = &producer()["receipt"];
    let bytes = unhex(&record["batch"]);
    let batch = must(SequenceBatch::decode(&bytes));
    assert_eq!(must(batch.encode()), bytes);
    assert_eq!(batch.height, number(&record["height"]));
    for bytes in [&bytes[..240], &bytes[1..]] {
        assert_eq!(SequenceBatch::decode(bytes), Err(CoverageError::Encoding));
    }
}
