use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use base64::Engine as _;
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_service::custody::{KeyId, Operation};
use layerx_human_service::journeys::{
    BankCommitEvidence, Endpoint, MoveJourneyError, MoveLegExecution, MovePlan, Relationship,
    RouteRequest, SendRoute,
};
use layerx_human_service::notify::JourneyId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, EvmAddress, NetworkId, ProtocolVersion, PublicKey,
    SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::rung::Rung;
use serde_json::Value;
use sha2::{Digest as _, Sha256};

fn execution(key: u8, nonce: u64, ceiling: u128) -> MoveLegExecution {
    MoveLegExecution::new(
        [key; 32],
        AgentDid::new("did:layerx:alice").or_fail("actor"),
        AuthorityRef::new("custody-human-primary").or_fail("authority"),
        nonce,
        0,
        u64::MAX,
        ceiling,
    )
    .or_fail("execution")
}

fn request(nonce: u64, to: u8, amount: u128) -> RouteRequest {
    RouteRequest {
        source: Endpoint::PaxeerWallet,
        destination: Endpoint::PaxeerWallet,
        relationship: Relationship::BankTransfer {
            from: EvmAddress::new([0x11; 20]),
            to: EvmAddress::new([to; 20]),
            route: SendRoute {
                account_sequence: Sequence::from_u64(nonce),
                idempotency_key: IdempotencyKey::new([to; 32]),
                expires_at: TimestampSeconds::from_u64(2_000),
                context_hash: ContextHash::new([0; 32]),
                authorization: SendAuthorization::new(
                    SendAuthorizationKind::Owner,
                    PublicKey::new([0x11; 32]),
                    AuthorizationSignature::new([0x22; 64]),
                ),
                network_id: NetworkId::new(125).or_fail("network"),
                protocol_version: ProtocolVersion::new(2).or_fail("protocol"),
            },
            pointer: EvmAddress::new([0x88; 20]),
            denom: "uhpx".to_owned(),
            chain_id: 125,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
        },
        asset: AssetId::new(Sha256::digest(b"uhpx").into()),
        amount: Amount::from_u128(amount),
    }
}

fn plan(
    requests: Vec<RouteRequest>,
    executions: Vec<MoveLegExecution>,
) -> Result<MovePlan, MoveJourneyError> {
    MovePlan::new_bank_transfers(
        JourneyId::new("jrn_movebanksend").or_fail("journey"),
        [0x44; 32],
        KeyId::new("human-primary").or_fail("key"),
        Operation::ProtocolMutation,
        requests,
        executions,
        100,
        "HPX",
        "a few seconds",
    )
}

fn unhex(text: &str) -> Vec<u8> {
    text.strip_prefix("0x")
        .or_fail("hex prefix")
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).or_fail("hex text"), 16)
                .or_fail("hex byte")
        })
        .collect()
}

// Re-recorded with the production bank ABI, receipt/RPC, sequence-batch and
// Comet encoders. Producer source hashes are retained inside the recording.
fn transcript() -> Value {
    serde_json::from_str(BANK_TRANSCRIPT).or_fail("canonical bank producer transcript")
}

struct BankNode {
    endpoint: SequencerEndpoint,
    secret: [u8; 32],
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}

impl BankNode {
    fn balance_result(records: &[Value], sent: [bool; 2], params: &Value) -> Value {
        assert_eq!(params[1], "pending");
        let index = records
            .iter()
            .position(|record| {
                record["recipient"]
                    .as_str()
                    .or_fail("recipient")
                    .to_ascii_lowercase()
                    == params[0].as_str().or_fail("balance address")
            })
            .or_fail("recorded recipient balance");
        records[index][if sent[index] {
            "balanceAfter"
        } else {
            "balanceBefore"
        }]
        .clone()
    }

    fn rpc_result(
        request: &Value,
        fixture: &Value,
        records: &[Value],
        sent: &mut [bool; 2],
        lose_submit_ack: bool,
    ) -> (Value, bool) {
        let params = &request["params"];
        let mut drop_reply = false;
        let result = match request["method"].as_str().or_fail("RPC method") {
            "eth_chainId" => serde_json::json!("0x7d"),
            "eth_getTransactionCount" => {
                assert_eq!(
                    params[0],
                    fixture["sender"]
                        .as_str()
                        .or_fail("sender")
                        .to_ascii_lowercase()
                );
                assert_eq!(params[1], "pending");
                serde_json::json!(format!(
                    "{:#x}",
                    fixture["nonce"].as_u64().or_fail("nonce")
                        + sent.iter().filter(|sent| **sent).count() as u64
                ))
            }
            "eth_sendRawTransaction" => {
                let index = records
                    .iter()
                    .position(|record| record["raw"] == params[0])
                    .unwrap_or_else(|| {
                        panic!("{}", "engine must send the exact producer transaction")
                    });
                assert!(
                    !sent[index],
                    "recovery must not send a duplicate transaction"
                );
                assert!(
                    sent[..index].iter().all(|sent| *sent),
                    "consecutive nonce order"
                );
                sent[index] = true;
                drop_reply = lose_submit_ack && index == 0;
                records[index]["view"]["txHash"].clone()
            }
            "pax_sequencerReceipt" => {
                let index = records
                    .iter()
                    .position(|record| record["view"]["txHash"] == params[0])
                    .or_fail("lookup exact transaction hash");
                if sent[index] {
                    records[index]["view"].clone()
                } else {
                    Value::Null
                }
            }
            "eth_getTransactionReceipt" => {
                let index = records
                    .iter()
                    .position(|record| record["view"]["txHash"] == params[0])
                    .or_fail("lookup exact landing receipt");
                assert!(sent[index]);
                records[index]["landing"].clone()
            }
            "eth_getBalance" => Self::balance_result(records, *sent, params),
            "px_getCommit" | "px_getValidators" => {
                let record = records
                    .iter()
                    .find(|record| {
                        record["height"].as_u64().or_fail("height").to_string()
                            == params[0].as_str().or_fail("decimal height")
                    })
                    .or_fail("recorded committed height");
                record[if request["method"] == "px_getCommit" {
                    "material"
                } else {
                    "validators"
                }]
                .clone()
            }
            "eth_getBlockByNumber" => {
                let record = records
                    .iter()
                    .find(|record| {
                        format!("{:#x}", record["height"].as_u64().or_fail("height"))
                            == params[0].as_str().or_fail("height")
                    })
                    .or_fail("recorded committed block");
                if params[1] == false {
                    serde_json::json!({"number": params[0], "stateRoot":record["appHash"]})
                } else {
                    record["block"].clone()
                }
            }
            method => panic!("unrecorded RPC method {method}"),
        };
        (result, drop_reply)
    }

    fn serve(
        listener: &TcpListener,
        fixture: &Value,
        stopped: &Arc<AtomicBool>,
        lose_submit_ack: bool,
    ) {
        let records = fixture["records"].as_array().or_fail("two recorded sends");
        assert_eq!(records.len(), 2);
        let mut sent = [false; 2];
        while !stopped.load(Ordering::Acquire) {
            let (stream, _) = match listener.accept() {
                Ok(pair) => pair,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    thread::sleep(Duration::from_millis(1));
                    continue;
                }
                Err(error) => panic!("loopback accept: {error}"),
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(3)))
                .or_fail("peer read bound");
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(reader.read_line(&mut line).or_fail("HTTP request"), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(value.trim().parse::<usize>().or_fail("request length"));
                }
            }
            let length = length.or_fail("RPC body length");
            assert!(length <= 65_536);
            let mut body = vec![0; length];
            reader.read_exact(&mut body).or_fail("RPC body");
            let request: Value = serde_json::from_slice(&body).or_fail("JSON RPC request");
            let (result, drop_reply) =
                Self::rpc_result(&request, fixture, records, &mut sent, lose_submit_ack);
            if drop_reply {
                continue;
            }
            let answer = serde_json::json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
                .to_string();
            let mut stream = reader.into_inner();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",answer.len()).or_fail("RPC response");
            stream.flush().or_fail("flush RPC response");
        }
        assert!(
            sent.into_iter().all(|sent| sent),
            "both producer sends traversed the real RPC client"
        );
    }

    fn start(lose_submit_ack: bool) -> Self {
        let fixture = transcript();
        let listener = TcpListener::bind("127.0.0.1:0").or_fail("loopback peer");
        listener.set_nonblocking(true).or_fail("nonblocking peer");
        let endpoint = SequencerEndpoint::new(
            &format!("http://{}/", listener.local_addr().or_fail("peer address")),
            &[],
        )
        .or_fail("production RPC client");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let secret = unhex(fixture["secret"].as_str().or_fail("fixture signing seed"))
            .try_into()
            .or_fail("signing seed width");
        let worker =
            thread::spawn(move || Self::serve(&listener, &fixture, &stopped, lose_submit_ack));
        Self {
            endpoint,
            secret,
            stop,
            worker: Some(worker),
        }
    }

    fn receipt_keys() -> Vec<ReceiptKeyEntry> {
        let fixture = transcript();
        let key = &fixture["key"];
        vec![ReceiptKeyEntry {
            key_id: unhex(key["keyId"].as_str().or_fail("governed key id"))
                .try_into()
                .or_fail("key id"),
            public_key: unhex(key["publicKey"].as_str().or_fail("governed key"))
                .try_into()
                .or_fail("key"),
            first_height: key["firstHeight"].as_u64().or_fail("first height"),
            last_height: key["lastHeight"].as_u64().or_fail("last height"),
        }]
    }

    fn commit_evidence() -> Vec<BankCommitEvidence> {
        transcript()["records"]
            .as_array()
            .or_fail("records")
            .iter()
            .map(|record| {
                let batch = unhex(record["batch"].as_str().or_fail("node sequence batch"));
                let proof = &record["proof"];
                let mut leaf = b"LXAR\x01".to_vec();
                leaf.extend_from_slice(
                    &proof["total"]
                        .as_str()
                        .or_fail("proof total")
                        .parse::<i64>()
                        .or_fail("total")
                        .to_be_bytes(),
                );
                leaf.extend_from_slice(
                    &proof["index"]
                        .as_str()
                        .or_fail("proof index")
                        .parse::<i64>()
                        .or_fail("index")
                        .to_be_bytes(),
                );
                let transaction_hash = Sha256::digest(&batch);
                leaf.extend_from_slice(&transaction_hash);
                let expected_leaf_hash =
                    Sha256::digest([&[0_u8][..], &transaction_hash[..]].concat());
                assert_eq!(
                    base64::engine::general_purpose::STANDARD
                        .decode(
                            proof["leaf_hash"]
                                .as_str()
                                .or_fail("Comet transaction leaf hash")
                        )
                        .or_fail("base64 leaf hash"),
                    expected_leaf_hash.as_slice(),
                    "the recorded Comet transaction proof commits the transaction hash"
                );
                let mut fields = vec![leaf];
                fields.extend(
                    proof["aunts"]
                        .as_array()
                        .or_fail("Comet proof aunts")
                        .iter()
                        .map(|aunt| {
                            base64::engine::general_purpose::STANDARD
                                .decode(aunt.as_str().or_fail("aunt"))
                                .or_fail("base64 aunt")
                        }),
                );
                BankCommitEvidence {
                    chain_id: record["material"]["signedHeader"]["header"]["chain_id"]
                        .as_str()
                        .or_fail("chain")
                        .to_owned(),
                    trusted_validators: record["validators"].clone(),
                    committed_header: record["material"]["signedHeader"].clone(),
                    covering_height: record["height"].as_u64().or_fail("covering height"),
                    batch_transaction: batch,
                    transaction_proof: fields,
                    activity_ids: record["activityIds"]
                        .as_array()
                        .or_fail("activity ids")
                        .iter()
                        .map(|id| {
                            unhex(id.as_str().or_fail("id"))
                                .try_into()
                                .or_fail("id width")
                        })
                        .collect(),
                }
            })
            .collect()
    }
}

impl Drop for BankNode {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .or_fail("peer worker")
            .join()
            .or_fail("loopback peer shutdown");
    }
}

fn live_request(
    nonce: u64,
    to: u8,
    amount: u128,
    sender: [u8; 20],
    native_pointer: [u8; 20],
) -> RouteRequest {
    let mut request = request(nonce, to, amount);
    let Relationship::BankTransfer {
        from,
        route,
        pointer,
        max_fee_per_gas,
        gas_limit,
        ..
    } = &mut request.relationship
    else {
        panic!("bank request");
    };
    *from = EvmAddress::new(sender);
    *pointer = EvmAddress::new(native_pointer);
    route.expires_at = TimestampSeconds::from_u64(u64::MAX);
    *max_fee_per_gas = 1_000_000_000_000;
    *gas_limit = 1_000_000;
    request
}

use layerx_agentd::outbox::{Outbox, SubmissionState};
use layerx_agentd::store::{ObjectKind, Store as AgentStore, TenantId};
use layerx_human_service::custody::{
    CustodySigner, EnvelopeKms, KeyClass, KeyEntropy, Keystore, SigningLimits,
};
use layerx_human_service::journeys::{MoveAuthorization, MoveJourney, MoveStage};
use layerx_human_service::store::PrincipalStore;
use layerx_human_service::trace::TraceId;
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use std::future::Future;
use std::pin::pin;
use std::sync::atomic::{AtomicBool, Ordering};
use std::task::{Context, Poll, Wake, Waker};

struct ReadyWake;
impl Wake for ReadyWake {
    fn wake(self: Arc<Self>) {}
}
fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(ReadyWake));
    match future.as_mut().poll(&mut Context::from_waker(&waker)) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("local custody operation did not finish"),
    }
}
fn registry() -> ModuleRegistry {
    ModuleRegistry::new(&[ModuleRegistration::new(
        ModuleId::Asset,
        &[ActivityType::new(ModuleId::Asset, 5).or_fail("send activity")],
    )
    .or_fail("asset registration")])
    .or_fail("registry")
}

pub fn exact_custody_bank_transfer() {
    exercise(false);
}
pub fn producer_outbox_and_consumer_reopen() {
    exercise(true);
}

fn assert_recorded_calls(movement: &MovePlan, fixture: &Value, sender: [u8; 20]) {
    for (index, call) in movement.bank_transfers().iter().enumerate() {
        let raw = unhex(
            fixture["records"][index]["raw"]
                .as_str()
                .or_fail("recorded transaction"),
        );
        let (recorded, signature) = decode_signed(&raw).or_fail("recorded production transaction");
        assert_eq!(
            recover_sender(&recorded, &signature).or_fail("recorded sender"),
            sender
        );
        assert_eq!(
            &recorded, call,
            "the node recording must contain the exact current 0x1001 bank-send preparation"
        );
    }
}

fn capture_signed_bank_calls(
    journey: &MoveJourney,
    movement: &MovePlan,
    outbox: &mut Outbox,
    producer: &mut AgentStore,
    tenant: &TenantId,
    signed: &mut std::collections::BTreeMap<usize, Vec<u8>>,
    coordinates: ([u8; 20], u64),
) {
    let (sender, nonce) = coordinates;
    // Enqueue only bytes emitted and durably signed by the real journey/custody path.
    for index in 0..2 {
        if let Some(raw) = journey.signed_transaction(index) {
            let hash = transaction_hash(raw);
            signed.entry(index).or_insert_with(|| {
                assert_eq!(
                    outbox
                        .enqueue(producer, tenant.clone(), raw.to_vec())
                        .unwrap_or_else(|error| panic!(
                            "{}: {error:?}",
                            "enqueue exact verified custody bytes"
                        )),
                    hash
                );
                raw.to_vec()
            });
            assert_eq!(
                outbox
                    .exact_signed_bytes(hash)
                    .or_fail("durable outbox bytes"),
                raw
            );
            let (call, signature) = decode_signed(raw).or_fail("decode actual signed bytes");
            assert_eq!(call, movement.bank_transfers()[index]);
            assert_eq!(
                recover_sender(&call, &signature).or_fail("custody sender"),
                sender
            );
            assert_eq!(call.nonce, nonce + index as u64);
            let disclosure = layerx_crypto::disclosure::call::bind_call(&call, sender)
                .or_fail("bank disclosure");
            let mut amount = [0; 32];
            amount[16..].copy_from_slice(&(if index == 0 { 90_u128 } else { 40 }).to_be_bytes());
            assert_eq!(
                disclosure.construct_method().or_fail("decode bank method"),
                Some(layerx_crypto::disclosure::call::ConstructMethod::BankSend {
                    from: sender,
                    to: [if index == 0 { 0x22 } else { 0x55 }; 20],
                    denom: "uhpx".to_owned(),
                    amount,
                })
            );
        }
    }
}

fn assert_committed_bank_calls(
    journey: &MoveJourney,
    signed: &std::collections::BTreeMap<usize, Vec<u8>>,
    proofs: &[BankCommitEvidence],
    node: &BankNode,
    before: &[[u8; 32]],
    nonce: u64,
) {
    for index in 0..2 {
        let raw = signed.get(&index).or_fail("original custody bytes");
        assert_eq!(journey.signed_transaction(index), Some(raw.as_slice()));
        let evidence = journey
            .verified_leg_evidence(index)
            .or_fail("receipt lookup")
            .or_fail("verified receipt");
        assert_eq!(evidence.tx_hash, transaction_hash(raw));
        assert_eq!(evidence.rung, Rung::Committed);
        assert_eq!(journey.bank_commit_evidence(index), Some(&proofs[index]));
        let receipt = layerx_wire::receipt::decode(&evidence.canonical_receipt)
            .or_fail("decode real producer receipt");
        let protocol = receipt.protocol().or_fail("protocol receipt");
        assert_eq!(protocol.activity_id(), transaction_hash(raw));
        assert_eq!(protocol.debit_sequence(), nonce + index as u64);
        let after = node
            .endpoint
            .balance(&[if index == 0 { 0x22 } else { 0x55 }; 20])
            .or_fail("after balance");
        let amount =
            |bytes: &[u8]| u128::from_be_bytes(bytes[16..].try_into().or_fail("balance width"));
        assert_eq!(
            amount(&after) - amount(&before[index]),
            (if index == 0 { 90 } else { 40 }) * 1_000_000_000_000
        );
    }
}

struct BankCustodySetup {
    store: PrincipalStore,
    digest: layerx_human_service::store::TenancyDigest,
    custody_digest: layerx_human_service::store::TenancyDigest,
    owner: layerx_human_service::store::PrincipalId,
    retention: layerx_human_service::store::RetentionPolicy,
    custody: CustodySigner,
}

fn open_bank_keys(root: &std::path::Path) -> Keystore {
    let secret_path = root.join("kms-root");
    Keystore::open_development(
        root.join("custody"),
        125,
        EnvelopeKms::new("file-kms://repair-22-33-63", &secret_path)
            .or_fail("real envelope provider"),
    )
    .or_fail("keystore")
}

fn open_bank_custody(
    root: &std::path::Path,
    retention: layerx_human_service::store::RetentionPolicy,
    custody_digest: layerx_human_service::store::TenancyDigest,
) -> CustodySigner {
    CustodySigner::new(
        open_bank_keys(root),
        PrincipalStore::open(root.join("custody-store"), retention, custody_digest)
            .or_fail("custody audit store"),
        registry(),
        SigningLimits::new(100, 10_000).or_fail("sign limits"),
    )
}

fn create_bank_custody_setup(
    root: &std::path::Path,
    node: &BankNode,
    sender: [u8; 20],
) -> BankCustodySetup {
    let map = tenancy(&[("alice", "tenant-a")]);
    let retention = retention_uniform(10_000);
    let owner = principal("alice");
    let (store, digest) = install_and_open(&root.join("human"), &map, retention);
    fs::write(root.join("kms-root"), [0x42; 64]).or_fail("envelope root");
    let key = KeyId::new("human-primary").or_fail("key");
    let keystore = open_bank_keys(root);
    keystore
        .generate(
            &owner,
            &key,
            KeyClass::HumanPrimary,
            KeyEntropy::new(node.secret, [0x52; 16], [0x53; 24]).or_fail("key entropy"),
        )
        .or_fail("custody key");
    drop(keystore);
    let (custody_store, custody_digest) =
        install_and_open(&root.join("custody-store"), &map, retention);
    drop(custody_store);
    let custody = open_bank_custody(root, retention, custody_digest);
    assert_eq!(custody.evm_wallet(&owner, &key).or_fail("wallet"), sender);
    BankCustodySetup {
        store,
        digest,
        custody_digest,
        owner,
        retention,
        custody,
    }
}

fn restore_signed_outbox(
    producer: &AgentStore,
    outbox: &mut Outbox,
    tenant: &TenantId,
    signed: &std::collections::BTreeMap<usize, Vec<u8>>,
) {
    let ids = producer.list_object_ids(tenant, ObjectKind::Outbox);
    assert_eq!(ids.len(), signed.len());
    for id in ids {
        let hash = id.try_into().or_fail("submission hash");
        outbox
            .restore(producer, tenant.clone(), hash)
            .or_fail("reopen durable outbox");
        let raw = outbox
            .exact_signed_bytes(hash)
            .or_fail("restored exact bytes");
        assert_eq!(transaction_hash(raw), hash);
        assert!(signed.values().any(|bytes| bytes.as_slice() == raw));
    }
}

fn track_signed_bank_calls(
    node: &BankNode,
    keys: &[ReceiptKeyEntry],
    signed: &std::collections::BTreeMap<usize, Vec<u8>>,
    outbox: &mut Outbox,
    producer: &mut AgentStore,
) {
    for raw in signed.values() {
        let hash = transaction_hash(raw);
        if matches!(
            lookup_authenticated(&node.endpoint, hash, LookupContext { receipt_keys: keys })
                .or_fail("track exact transaction"),
            Lookup::Verified(_)
        ) && outbox.status(hash).or_fail("outbox status").state == SubmissionState::Queued
        {
            outbox
                .transition(
                    producer,
                    hash,
                    SubmissionState::Submitted,
                    "real RPC accepted exact custody transaction",
                    None,
                )
                .or_fail("persist acknowledgement");
        }
    }
}

fn verify_reopened_bank_state(
    roots: (&std::path::Path, &std::path::Path),
    durability: (
        layerx_human_service::store::RetentionPolicy,
        layerx_human_service::store::TenancyDigest,
    ),
    owner: &layerx_human_service::store::PrincipalId,
    id: &JourneyId,
    tenant: &TenantId,
    signed: &std::collections::BTreeMap<usize, Vec<u8>>,
    final_status: &layerx_human_service::journeys::MoveStatus,
) {
    let (producer_root, store_root) = roots;
    let (retention, digest) = durability;
    let producer = AgentStore::open(producer_root).or_fail("reopen final producer");
    let mut outbox = Outbox::default();
    for raw in signed.values() {
        let hash = transaction_hash(raw);
        outbox
            .restore(&producer, tenant.clone(), hash)
            .or_fail("final outbox reopen");
        assert_eq!(outbox.exact_signed_bytes(hash).or_fail("final bytes"), raw);
        assert_eq!(
            outbox.status(hash).or_fail("submission state").state,
            SubmissionState::Submitted
        );
    }
    let mut store =
        PrincipalStore::open(store_root, retention, digest).or_fail("final consumer reopen");
    let recovered = MoveJourney::load(&store.principal(owner).or_fail("scope"), id)
        .or_fail("load final evidence")
        .or_fail("journey");
    assert_eq!(
        &recovered.status().or_fail("recovered action"),
        final_status
    );
    assert_eq!(
        recovered.rung().or_fail("recovered finality"),
        Rung::Committed
    );
    drop(recovered);
    drop(store);
    drop(outbox);
    drop(producer);
}

fn recorded_bank_movement() -> (MovePlan, [u8; 20], u64) {
    let fixture = transcript();
    let sender: [u8; 20] = unhex(fixture["sender"].as_str().or_fail("sender"))
        .try_into()
        .or_fail("sender width");
    let mut pointer = [0; 20];
    pointer[18..].copy_from_slice(&0x1001_u16.to_be_bytes());
    let nonce = fixture["nonce"].as_u64().or_fail("nonce");
    let ceiling = 1_000_000_000_000_000_000;
    let movement = plan(
        vec![
            live_request(nonce, 0x22, 90, sender, pointer),
            live_request(nonce + 1, 0x55, 40, sender, pointer),
        ],
        vec![
            execution(1, nonce, ceiling),
            execution(2, nonce + 1, ceiling),
        ],
    )
    .or_fail("current bank plan");
    assert_recorded_calls(&movement, &fixture, sender);
    (movement, sender, nonce)
}

fn bank_exercise_root(restart_every_phase: bool) -> std::path::PathBuf {
    let root = directory(if restart_every_phase {
        "repair-22-33-63-reopen"
    } else {
        "repair-22-33-63-exact"
    });
    fs::create_dir_all(&root).or_fail("fixture directory");
    root
}

fn assert_completed_bank_journey(
    journey: &MoveJourney,
    signed: &std::collections::BTreeMap<usize, Vec<u8>>,
    restart_every_phase: bool,
    lost_acknowledgements: usize,
) {
    assert_eq!(journey.rung().or_fail("final rung"), Rung::Committed);
    assert_eq!(
        journey.status().or_fail("final status").stage(),
        MoveStage::Done
    );
    assert_eq!(
        journey.status().or_fail("amount").actual_amount(),
        Some(130)
    );
    assert_eq!(signed.len(), 2);
    assert_eq!(lost_acknowledgements, usize::from(restart_every_phase));
}

type BankCycleResources = (
    MoveJourney,
    PrincipalStore,
    CustodySigner,
    Outbox,
    AgentStore,
);

fn reopen_bank_cycle(
    resources: BankCycleResources,
    roots: (&std::path::Path, &std::path::Path, &std::path::Path),
    durability: (
        layerx_human_service::store::RetentionPolicy,
        layerx_human_service::store::TenancyDigest,
        layerx_human_service::store::TenancyDigest,
    ),
    coordinates: (
        &layerx_human_service::store::PrincipalId,
        &JourneyId,
        &TenantId,
    ),
    signed: &std::collections::BTreeMap<usize, Vec<u8>>,
) -> BankCycleResources {
    let (journey, store, custody, outbox, producer) = resources;
    let (root, store_root, producer_root) = roots;
    let (retention, digest, custody_digest) = durability;
    let (owner, id, tenant) = coordinates;
    drop(journey);
    drop(store);
    drop(custody);
    drop(outbox);
    drop(producer);
    let producer = AgentStore::open(producer_root).or_fail("reopen producer");
    let mut outbox = Outbox::default();
    restore_signed_outbox(&producer, &mut outbox, tenant, signed);
    let custody = open_bank_custody(root, retention, custody_digest);
    let mut store =
        PrincipalStore::open(store_root, retention, digest).or_fail("reopen Human consumer");
    let journey = MoveJourney::load(&store.principal(owner).or_fail("scope"), id)
        .or_fail("reopen journey")
        .or_fail("durable journey");
    (journey, store, custody, outbox, producer)
}

fn assert_bank_outcome(
    journey: &MoveJourney,
    signed: &std::collections::BTreeMap<usize, Vec<u8>>,
    restart: (bool, usize),
    chain: (&BankNode, &[BankCommitEvidence], &[[u8; 32]], u64, [u8; 20]),
) -> layerx_human_service::journeys::MoveStatus {
    let (restart_every_phase, lost_acknowledgements) = restart;
    let (node, proofs, before, nonce, sender) = chain;
    assert_completed_bank_journey(journey, signed, restart_every_phase, lost_acknowledgements);
    let final_status = journey.status().or_fail("final status");
    assert_committed_bank_calls(journey, signed, proofs, node, before, nonce);
    assert_eq!(
        node.endpoint
            .pending_nonce(&sender)
            .or_fail("pending nonce after recovery"),
        nonce + 2
    );
    final_status
}

fn exercise(restart_every_phase: bool) {
    let root = bank_exercise_root(restart_every_phase);
    let (movement, sender, nonce) = recorded_bank_movement();
    let node = BankNode::start(restart_every_phase);
    let keys = BankNode::receipt_keys();
    let proofs = BankNode::commit_evidence();
    let store_root = root.join("human");
    let BankCustodySetup {
        mut store,
        digest,
        custody_digest,
        owner,
        retention,
        mut custody,
    } = create_bank_custody_setup(&root, &node, sender);
    let now = 1_700_000_000;
    let trace = TraceId::mint([0x63; 16]);
    let id = JourneyId::new("jrn_movebanksend").or_fail("id");
    let mut journey = MoveJourney::commit(
        &mut store.principal(&owner).or_fail("scope"),
        &movement,
        MoveAuthorization::Allowed,
        &registry(),
        now,
    )
    .or_fail("commit original economic plan");
    let before: Vec<_> = [[0x22; 20], [0x55; 20]]
        .iter()
        .map(|to| node.endpoint.balance(to).or_fail("before balance"))
        .collect();
    let tenant = TenantId::new("tenant-a").or_fail("tenant");
    let producer_root = root.join("producer");
    let mut producer = AgentStore::open(&producer_root).or_fail("producer store");
    let mut outbox = Outbox::default();
    let mut observed_phases = std::collections::BTreeSet::new();
    let mut signed = std::collections::BTreeMap::new();
    let mut lost_acknowledgements = 0;
    for _ in 0..48 {
        capture_signed_bank_calls(
            &journey,
            &movement,
            &mut outbox,
            &mut producer,
            &tenant,
            &mut signed,
            (sender, nonce),
        );
        observed_phases.insert(format!("{:?}", journey.status().or_fail("stage").stage()));
        if journey.rung().or_fail("rung") == Rung::Committed {
            break;
        }
        if restart_every_phase {
            (journey, store, custody, outbox, producer) = reopen_bank_cycle(
                (journey, store, custody, outbox, producer),
                (&root, &store_root, &producer_root),
                (retention, digest, custody_digest),
                (&owner, &id, &tenant),
                &signed,
            );
        }
        let advance = ready(journey.advance_bank_with_commit(
            &mut store.principal(&owner).or_fail("scope"),
            &node.endpoint,
            &keys,
            &proofs,
            &custody,
            &trace,
            now,
        ));
        if let Err(error) = advance {
            assert!(
                restart_every_phase && lost_acknowledgements == 0,
                "unexpected boundary failure: {error:?}"
            );
            assert!(
                error.to_string().contains("agent boundary failure"),
                "submission acknowledgement: {error:?}"
            );
            lost_acknowledgements += 1;
        }
        track_signed_bank_calls(&node, &keys, &signed, &mut outbox, &mut producer);
    }
    let final_status = assert_bank_outcome(
        &journey,
        &signed,
        (restart_every_phase, lost_acknowledgements),
        (&node, &proofs, &before, nonce, sender),
    );
    drop((journey, store, custody, outbox, producer));
    verify_reopened_bank_state(
        (&producer_root, &store_root),
        (retention, digest),
        &owner,
        &id,
        &tenant,
        &signed,
        &final_status,
    );
    assert!(!observed_phases.is_empty());
    drop(node);
    fs::remove_dir_all(root).or_fail("remove private durable fixtures");
}

const BANK_TRANSCRIPT: &str = r#"{
  "key": {
    "firstHeight": 1,
    "keyId": "0x2b61f0ec7a2b23523274ba3f5b0a4a9e66f75a1165e11ced25862d8a1692761d",
    "lastHeight": 1000,
    "publicKey": "0xa7f6dfaf8f38b89ba8ce649b594f91e4d01fdc57f9c9493df43b5e50a9987367"
  },
  "nonce": 7,
  "pointer": "0x0000000000000000000000000000000000001001",
  "producer": "node/sequencer ReceiptKey.SignReceipt, rpc decodeSequencerReceipt and SequencerReceiptResult.Verify (unchanged extracted production declarations); bank ABI encoder, EncodeSequenceBatchTx, Comet Header.Hash, VoteSignBytes and Txs.Proof",
  "records": [
    {
      "activityIds": [
        "0x2c6f9011c36194321c05f13b17758624ecd7cbea2a9f9ceba1700ca13062b5e5"
      ],
      "appHash": "0x4a5f7f615738280ec996e9a5bec458d5b3997ab8f3b4bfe194d0d12d94f99428",
      "balanceAfter": "0x51dac207a000",
      "balanceBefore": "0x0",
      "batch": "0x5041584545522f76312f73657175656e63652d626174636800000000000000001200000000000000150000000000000015128d2f802cddb92f94af5505868c3c55089dd3760a25ba9a58b68f507144efbf86b022f4da302cc6c3148e0508d67a488635c9f76eab9aa9f9c3ba76099e407e8e9da4c7d6c28b16514a3ba7a345e3295ec325212e939a0c7de8452f99ff6dd92b61f0ec7a2b23523274ba3f5b0a4a9e66f75a1165e11ced25862d8a1692761df86d25e940cf18311f50496e145e48275c2c2e7106e4d94d1114bcadfbc0647e747dcd79e7920e28312dd6597b00d8461cc6790180bb6de4b1ad470bcf079b03",
      "block": {
        "hash": "0x3df853ed0255adc7de2d810998ff635c3513c100e9cba936dd5cfeaeaf49256f",
        "number": "0x12",
        "parentHash": "0x3333333333333333333333333333333333333333333333333333333333333333",
        "receiptsRoot": "0x3333333333333333333333333333333333333333333333333333333333333333",
        "timestamp": "0x3e8",
        "transactions": [],
        "transactionsRoot": "0x3cd3dfb6a596af1567297c96b0fe59f79d017bf44123fd535e4d8e171e26d323"
      },
      "hash": "0x3df853ed0255adc7de2d810998ff635c3513c100e9cba936dd5cfeaeaf49256f",
      "height": 18,
      "landing": {
        "blockHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "blockNumber": "0x8",
        "contractAddress": "0x0000000000000000000000000000000000000000",
        "cumulativeGasUsed": "0x0",
        "effectiveGasPrice": null,
        "gasUsed": "0xc350",
        "logs": [],
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "root": "0x",
        "status": "0x1",
        "transactionHash": "0x2c6f9011c36194321c05f13b17758624ecd7cbea2a9f9ceba1700ca13062b5e5",
        "transactionIndex": "0x0",
        "type": "0x2"
      },
      "material": {
        "height": "18",
        "signedHeader": {
          "commit": {
            "block_id": {
              "hash": "3DF853ED0255ADC7DE2D810998FF635C3513C100E9CBA936DD5CFEAEAF49256F",
              "parts": {
                "hash": "3333333333333333333333333333333333333333333333333333333333333333",
                "total": 1
              }
            },
            "height": "18",
            "round": 0,
            "signatures": [
              {
                "block_id_flag": 2,
                "signature": "BWNDdMm6HdwKaT+WBbFfDQ1DvDXm+cca7loVkyg47LFA8koPIX9qpu+9QEK/YuLBfmvv89ZsKezU9YBt6WpFDA==",
                "timestamp": "1970-01-01T00:16:40Z",
                "validator_address": "C0742A797BD5374378F26A371F35E61B32C0D0CC"
              }
            ]
          },
          "header": {
            "app_hash": "4A5F7F615738280EC996E9A5BEC458D5B3997AB8F3B4BFE194D0D12D94F99428",
            "chain_id": "hyperpax_125-1",
            "consensus_hash": "3333333333333333333333333333333333333333333333333333333333333333",
            "data_hash": "3CD3DFB6A596AF1567297C96B0FE59F79D017BF44123FD535E4D8E171E26D323",
            "evidence_hash": "3333333333333333333333333333333333333333333333333333333333333333",
            "height": "18",
            "last_block_id": {
              "hash": "3333333333333333333333333333333333333333333333333333333333333333",
              "parts": {
                "hash": "3333333333333333333333333333333333333333333333333333333333333333",
                "total": 1
              }
            },
            "last_commit_hash": "3333333333333333333333333333333333333333333333333333333333333333",
            "last_results_hash": "3333333333333333333333333333333333333333333333333333333333333333",
            "next_validators_hash": "28C70F937A83334B14CAD61AB38EFDCE1A3CA5A8FECD3DF72BAB18027D1D6C01",
            "proposer_address": "C0742A797BD5374378F26A371F35E61B32C0D0CC",
            "time": "1970-01-01T00:16:40Z",
            "validators_hash": "28C70F937A83334B14CAD61AB38EFDCE1A3CA5A8FECD3DF72BAB18027D1D6C01",
            "version": {
              "app": "1",
              "block": "11"
            }
          }
        }
      },
      "proof": {
        "aunts": [],
        "index": "0",
        "leaf_hash": "PNPftqWWrxVnKXyWsP5Z950Be/RBI/1TXk2OFx4m0yM=",
        "total": "1"
      },
      "raw": "0x02f9012d7d070185e8d4a51000830f424094000000000000000000000000000000000000100180b8c45c05961b00000000000000000000000017c5185167401ed00cf5f5b2fc97d9bbfdb7d02500000000000000000000000022222222222222222222222222222222222222220000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000005a00000000000000000000000000000000000000000000000000000000000000047568707800000000000000000000000000000000000000000000000000000000c080a044bea57c7332819a1e4848437ce5416572aecc98f21009632b8472b6bd6600fca049ef1310225c83ecb8457d3d80610e4d6e989b81a39aed424aef01533ad59632",
      "recipient": "0x2222222222222222222222222222222222222222",
      "validators": {
        "height": "18",
        "total": "1",
        "validators": [
          {
            "address": "c0742a797bd5374378f26a371f35e61b32c0d0cc",
            "pubKey": {
              "type": "tendermint/PubKeyEd25519",
              "value": "W2SJycf9Dc9QVF58FkiG70BJHsBsfxsSMEF5foEXU14="
            },
            "votingPower": "10"
          }
        ]
      },
      "view": {
        "contractAddress": "0x0000000000000000000000000000000000000000",
        "cumulativeGasUsed": "0xc350",
        "effectiveGasPrice": "0x1",
        "feeAmount": "0xc350",
        "feeDenom": "uhpx",
        "from": "0x17c5185167401ed00cf5f5b2fc97d9bbfdb7d025",
        "gasUsed": "0xc350",
        "height": "0x8",
        "index": "0x0",
        "keyId": "0x2b61f0ec7a2b23523274ba3f5b0a4a9e66f75a1165e11ced25862d8a1692761d",
        "logs": [],
        "nonce": "0x7",
        "paymentDenom": "uhpx",
        "receipt": "0x000252010002000000202c6f9011c36194321c05f13b17758624ecd7cbea2a9f9ceba1700ca13062b5e5000000000000001500000020128d2f802cddb92f94af5505868c3c55089dd3760a25ba9a58b68f507144efbf0000002086b022f4da302cc6c3148e0508d67a488635c9f76eab9aa9f9c3ba76099e407e00000020e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8550000000000000001000c00000000010000000020000000000000000000000000000000000000000000000000000000000000000000000071000000000000c350000000000000c35000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855000000000000000004756870780000000000000000000000000000c350000000200000000000000000000000000000000000000000000000000000000000000008000c000000010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73000000000000000000000000000000000000002000000000000000000000000017c5185167401ed00cf5f5b2fc97d9bbfdb7d0250000000000000000000000003b9aca000000000000000000000000003b9a06b000000000000000070000002000000000000000000000000000000000000000000000000000000000000010010000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000202b61f0ec7a2b23523274ba3f5b0a4a9e66f75a1165e11ced25862d8a1692761d00000000000f424001000000409f763003cf8e5853ba5b7d4435f9eeb5b4ed03b8330f08f40ab9a725436ff48029d163ae6fe8791c17f4a8dcd011bd41f5b9ba10855137550db74da5d6a51400",
        "returnData": "0x",
        "rung": "receipted",
        "seq": "0x15",
        "signingKey": "0xa7f6dfaf8f38b89ba8ce649b594f91e4d01fdc57f9c9493df43b5e50a9987367",
        "sponsor": "0x0000000000000000000000000000000000000000",
        "status": "0x1",
        "timestamp": "0xf4240",
        "to": "0x0000000000000000000000000000000000001001",
        "txHash": "0x2c6f9011c36194321c05f13b17758624ecd7cbea2a9f9ceba1700ca13062b5e5",
        "vmError": ""
      }
    },
    {
      "activityIds": [
        "0xb44ed9ba71bb63f598848b468fffeb49a924b12af9f5b1092409f809b0528f36"
      ],
      "appHash": "0x0c15fc0438296d4057b909e9d3d0e7d2fdce99684c25ef8ec32d703275ad13a6",
      "balanceAfter": "0x246139ca8000",
      "balanceBefore": "0x0",
      "batch": "0x5041584545522f76312f73657175656e63652d62617463680000000000000000130000000000000016000000000000001686b022f4da302cc6c3148e0508d67a488635c9f76eab9aa9f9c3ba76099e407e602ab9efb754a5187eceac19bb4d5e1530c9b60d000bf7160340c0e23cc007b53a373d7c66102cb70e3d6d1f17dd168ea1f0c7bb540592f71691083214be0ce22b61f0ec7a2b23523274ba3f5b0a4a9e66f75a1165e11ced25862d8a1692761d7d385032cb3d539ac2066955d0478bbc601229e76fe6ca0c9bf7d6e430aa511dce896e9e9423e44b615c956d79debdc23c7e3af626cfd3ad0fcd368b0fb56000",
      "block": {
        "hash": "0x5592daa3a26a72cb569892916d3f1f692311b110c42a943961d8e2727dd0eb32",
        "number": "0x13",
        "parentHash": "0x3333333333333333333333333333333333333333333333333333333333333333",
        "receiptsRoot": "0x3333333333333333333333333333333333333333333333333333333333333333",
        "timestamp": "0x3e8",
        "transactions": [],
        "transactionsRoot": "0x17142e2d6d8eff93cfe5bb93beb8ddefaa7a3ddff79bdc674f41e6e465304f11"
      },
      "hash": "0x5592daa3a26a72cb569892916d3f1f692311b110c42a943961d8e2727dd0eb32",
      "height": 19,
      "landing": {
        "blockHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
        "blockNumber": "0x9",
        "contractAddress": "0x0000000000000000000000000000000000000000",
        "cumulativeGasUsed": "0x0",
        "effectiveGasPrice": null,
        "gasUsed": "0xc350",
        "logs": [],
        "logsBloom": "0x00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
        "root": "0x",
        "status": "0x1",
        "transactionHash": "0xb44ed9ba71bb63f598848b468fffeb49a924b12af9f5b1092409f809b0528f36",
        "transactionIndex": "0x0",
        "type": "0x2"
      },
      "material": {
        "height": "19",
        "signedHeader": {
          "commit": {
            "block_id": {
              "hash": "5592DAA3A26A72CB569892916D3F1F692311B110C42A943961D8E2727DD0EB32",
              "parts": {
                "hash": "3333333333333333333333333333333333333333333333333333333333333333",
                "total": 1
              }
            },
            "height": "19",
            "round": 0,
            "signatures": [
              {
                "block_id_flag": 2,
                "signature": "YDEgOD5nSvuqUj4S5IcS1As3SsBxnYPdJ9rCI1lHtg+4G1jhgPA76LHMBVRC517EhHimya1VvmTumq7UWqPSBw==",
                "timestamp": "1970-01-01T00:16:40Z",
                "validator_address": "C0742A797BD5374378F26A371F35E61B32C0D0CC"
              }
            ]
          },
          "header": {
            "app_hash": "0C15FC0438296D4057B909E9D3D0E7D2FDCE99684C25EF8EC32D703275AD13A6",
            "chain_id": "hyperpax_125-1",
            "consensus_hash": "3333333333333333333333333333333333333333333333333333333333333333",
            "data_hash": "17142E2D6D8EFF93CFE5BB93BEB8DDEFAA7A3DDFF79BDC674F41E6E465304F11",
            "evidence_hash": "3333333333333333333333333333333333333333333333333333333333333333",
            "height": "19",
            "last_block_id": {
              "hash": "3333333333333333333333333333333333333333333333333333333333333333",
              "parts": {
                "hash": "3333333333333333333333333333333333333333333333333333333333333333",
                "total": 1
              }
            },
            "last_commit_hash": "3333333333333333333333333333333333333333333333333333333333333333",
            "last_results_hash": "3333333333333333333333333333333333333333333333333333333333333333",
            "next_validators_hash": "28C70F937A83334B14CAD61AB38EFDCE1A3CA5A8FECD3DF72BAB18027D1D6C01",
            "proposer_address": "C0742A797BD5374378F26A371F35E61B32C0D0CC",
            "time": "1970-01-01T00:16:40Z",
            "validators_hash": "28C70F937A83334B14CAD61AB38EFDCE1A3CA5A8FECD3DF72BAB18027D1D6C01",
            "version": {
              "app": "1",
              "block": "11"
            }
          }
        }
      },
      "proof": {
        "aunts": [],
        "index": "0",
        "leaf_hash": "FxQuLW2O/5PP5buTvrjd76p6Pd/3m9xnT0Hm5GUwTxE=",
        "total": "1"
      },
      "raw": "0x02f9012d7d080185e8d4a51000830f424094000000000000000000000000000000000000100180b8c45c05961b00000000000000000000000017c5185167401ed00cf5f5b2fc97d9bbfdb7d02500000000000000000000000055555555555555555555555555555555555555550000000000000000000000000000000000000000000000000000000000000080000000000000000000000000000000000000000000000000000000000000002800000000000000000000000000000000000000000000000000000000000000047568707800000000000000000000000000000000000000000000000000000000c001a0c3f778e86fa0d4bcc0d29dde126d09070e9485aff835bcb62a6230e2020b70d8a0284be4905262a3d0b9e68593926eaa3737770dc8008639177076ab112984c96b",
      "recipient": "0x5555555555555555555555555555555555555555",
      "validators": {
        "height": "19",
        "total": "1",
        "validators": [
          {
            "address": "c0742a797bd5374378f26a371f35e61b32c0d0cc",
            "pubKey": {
              "type": "tendermint/PubKeyEd25519",
              "value": "W2SJycf9Dc9QVF58FkiG70BJHsBsfxsSMEF5foEXU14="
            },
            "votingPower": "10"
          }
        ]
      },
      "view": {
        "contractAddress": "0x0000000000000000000000000000000000000000",
        "cumulativeGasUsed": "0xc350",
        "effectiveGasPrice": "0x1",
        "feeAmount": "0xc350",
        "feeDenom": "uhpx",
        "from": "0x17c5185167401ed00cf5f5b2fc97d9bbfdb7d025",
        "gasUsed": "0xc350",
        "height": "0x9",
        "index": "0x0",
        "keyId": "0x2b61f0ec7a2b23523274ba3f5b0a4a9e66f75a1165e11ced25862d8a1692761d",
        "logs": [],
        "nonce": "0x8",
        "paymentDenom": "uhpx",
        "receipt": "0x00025201000200000020b44ed9ba71bb63f598848b468fffeb49a924b12af9f5b1092409f809b0528f3600000000000000160000002086b022f4da302cc6c3148e0508d67a488635c9f76eab9aa9f9c3ba76099e407e00000020602ab9efb754a5187eceac19bb4d5e1530c9b60d000bf7160340c0e23cc007b500000020e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b8550000000000000001000c00000000010000000020000000000000000000000000000000000000000000000000000000000000000000000071000000000000c350000000000000c35000000000000000010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855000000000000000004756870780000000000000000000000000000c350000000200000000000000000000000000000000000000000000000000000000000000009000c000000010000000002000000200e237584332fc3e1f1672d80a167a8d027597a64b34f4be18951e60babdd4f73000000000000000000000000000000000000002000000000000000000000000017c5185167401ed00cf5f5b2fc97d9bbfdb7d0250000000000000000000000003b9aca000000000000000000000000003b9a06b000000000000000080000002000000000000000000000000000000000000000000000000000000000000010010000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000202b61f0ec7a2b23523274ba3f5b0a4a9e66f75a1165e11ced25862d8a1692761d00000000000f42400100000040eb5f07925bb200e099ea3294e3e980dad8a7769a2dfe815e2f4e3af4c9438a863f8195f4c287b733ac62ed9c614b1d54d710b8c7c36113ca37717900619e5e0b",
        "returnData": "0x",
        "rung": "receipted",
        "seq": "0x16",
        "signingKey": "0xa7f6dfaf8f38b89ba8ce649b594f91e4d01fdc57f9c9493df43b5e50a9987367",
        "sponsor": "0x0000000000000000000000000000000000000000",
        "status": "0x1",
        "timestamp": "0xf4240",
        "to": "0x0000000000000000000000000000000000001001",
        "txHash": "0xb44ed9ba71bb63f598848b468fffeb49a924b12af9f5b1092409f809b0528f36",
        "vmError": ""
      }
    }
  ],
  "secret": "0x4242424242424242424242424242424242424242424242424242424242424242",
  "sender": "0x17c5185167401ed00cf5f5b2fc97d9bbfdb7d025",
  "sourceHashes": {
    "modules/sequencer/types/batch_tx.go": "b948fd8a9f0567b0aa392b58cadc6555df41eaab0a603bac1e9cf84916a7410e",
    "node/sequencer/executor.go": "6e9664fa63b7c49b53009710614f8c778dcf4662b53ff60127e406a450119141",
    "node/sequencer/receipt.go": "fd5e182f019b043139e16b69d2324ac736cc5406cfd0cae0478fdce568b5ec30",
    "node/sequencer/receiptkey.go": "dd9373d94b240a238cbcc802dc3f0e182d7293bf4f9007916f46a404d549695e",
    "precompiles/bank/abi.json": "3611222d01669a53e8eb40807ef801fc60bd3c386ddd034a900ebbad00b48422",
    "rpc/sequencer_receipt.go": "458ed4ad4b40a4301a6792388b1a3b595ec0c36340b3de514b1fb7e7d133c455"
  }
}
"#;

use crate::require_value::RequireValue as _;
