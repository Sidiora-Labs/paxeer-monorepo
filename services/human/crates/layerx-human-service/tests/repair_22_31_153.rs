use layerx_client::runtime_clock::RuntimeClock;
use layerx_types::clock::{Clock, Deadline};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::path::Path;
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use base64::Engine as _;
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint, SequencerError};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_human_service::custody::{KeyId, Operation};
use layerx_human_service::journeys::{
    BankCommitEvidence, BankCommittedReceipt, Endpoint, MoveJourneyError, MoveLegExecution,
    MovePlan, Relationship, RouteRequest, SendRoute,
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
        AgentDid::new("did:layerx:alice").expect("actor"),
        AuthorityRef::new("custody-human-primary").expect("authority"),
        nonce,
        0,
        u64::MAX,
        ceiling,
    )
    .expect("execution")
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
                network_id: NetworkId::new(125).expect("network"),
                protocol_version: ProtocolVersion::new(2).expect("protocol"),
            },
            pointer: EvmAddress::new([0x88; 20]),
            denom: "uhpx".to_owned(),
            chain_id: 125,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
        },
        asset: AssetId::new([0x33; 32]),
        amount: Amount::from_u128(amount),
    }
}

fn plan(
    requests: Vec<RouteRequest>,
    executions: Vec<MoveLegExecution>,
) -> Result<MovePlan, MoveJourneyError> {
    MovePlan::new_bank_transfers(
        JourneyId::new("jrn_movebanksend").expect("journey"),
        [0x44; 32],
        KeyId::new("human-primary").expect("key"),
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
        .expect("hex prefix")
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16).expect("hex byte")
        })
        .collect()
}

// Recorded once using node/sequencer's signing/receipt codecs, node RPC view
// validation, and Comet's header, vote, batch and transaction-proof producers.
fn transcript() -> Value {
    serde_json::from_str(include_str!(
        "../src/journeys/repair_22_31_153_transcript.json"
    ))
    .expect("committed Go producer transcript")
}

struct BankNode {
    clock: Arc<RuntimeClock>,
    endpoint: SequencerEndpoint,
    secret: [u8; 32],
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}

impl BankNode {
    fn start(clock: Arc<RuntimeClock>) -> Self {
        let fixture = transcript();
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback peer");
        listener.set_nonblocking(true).expect("nonblocking peer");
        let endpoint = SequencerEndpoint::new(
            &format!("http://{}/", listener.local_addr().expect("peer address")),
            &[],
        )
        .expect("production RPC client");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = stop.clone();
        let secret = unhex(fixture["secret"].as_str().expect("fixture signing seed"))
            .try_into()
            .expect("signing seed width");
        let worker = thread::spawn(move || {
            let records = fixture["records"].as_array().expect("two recorded sends");
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
                    .expect("peer read bound");
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).expect("HTTP request"), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().expect("request length"));
                    }
                }
                let length = length.expect("RPC body length");
                assert!(length <= 65_536);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).expect("RPC body");
                let request: Value = serde_json::from_slice(&body).expect("JSON RPC request");
                let params = &request["params"];
                let result = match request["method"].as_str().expect("RPC method") {
                    "eth_chainId" => serde_json::json!("0x7d"),
                    "eth_getTransactionCount" => {
                        assert_eq!(
                            params[0],
                            fixture["sender"]
                                .as_str()
                                .expect("sender")
                                .to_ascii_lowercase()
                        );
                        assert_eq!(params[1], "pending");
                        serde_json::json!(format!(
                            "{:#x}",
                            fixture["nonce"].as_u64().expect("nonce")
                                + sent.iter().filter(|sent| **sent).count() as u64
                        ))
                    }
                    "eth_sendRawTransaction" => {
                        let index = records
                            .iter()
                            .position(|record| record["raw"] == params[0])
                            .expect("engine must send the exact producer transaction");
                        assert!(
                            !sent[index],
                            "recovery must not send a duplicate transaction"
                        );
                        assert!(
                            sent[..index].iter().all(|sent| *sent),
                            "consecutive nonce order"
                        );
                        sent[index] = true;
                        records[index]["view"]["txHash"].clone()
                    }
                    "pax_sequencerReceipt" => {
                        let index = records
                            .iter()
                            .position(|record| record["view"]["txHash"] == params[0])
                            .expect("lookup exact transaction hash");
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
                            .expect("lookup exact landing receipt");
                        assert!(sent[index]);
                        records[index]["landing"].clone()
                    }
                    "eth_getBalance" => {
                        assert_eq!(params[1], "pending");
                        let index = records
                            .iter()
                            .position(|record| {
                                record["recipient"]
                                    .as_str()
                                    .expect("recipient")
                                    .to_ascii_lowercase()
                                    == params[0].as_str().expect("balance address")
                            })
                            .expect("recorded recipient balance");
                        records[index][if sent[index] {
                            "balanceAfter"
                        } else {
                            "balanceBefore"
                        }]
                        .clone()
                    }
                    "px_getCommit" | "px_getValidators" => {
                        let record = records
                            .iter()
                            .find(|record| {
                                record["height"].as_u64().expect("height").to_string()
                                    == params[0].as_str().expect("decimal height")
                            })
                            .expect("recorded committed height");
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
                                format!("{:#x}", record["height"].as_u64().expect("height"))
                                    == params[0].as_str().expect("height")
                            })
                            .expect("recorded committed block");
                        if params[1] == false {
                            serde_json::json!({"number": params[0], "stateRoot":record["appHash"]})
                        } else {
                            record["block"].clone()
                        }
                    }
                    method => panic!("unrecorded RPC method {method}"),
                };
                let answer =
                    serde_json::json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
                        .to_string();
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",answer.len()).expect("RPC response");
                stream.flush().expect("flush RPC response");
            }
            assert!(
                sent.into_iter().all(|sent| sent),
                "both producer sends traversed the real RPC client"
            );
        });
        Self {
            clock,
            endpoint,
            secret,
            stop,
            worker: Some(worker),
        }
    }

    fn receipt_keys(&self) -> Vec<ReceiptKeyEntry> {
        let fixture = transcript();
        let key = &fixture["key"];
        vec![ReceiptKeyEntry {
            key_id: unhex(key["keyId"].as_str().expect("governed key id"))
                .try_into()
                .expect("key id"),
            public_key: unhex(key["publicKey"].as_str().expect("governed key"))
                .try_into()
                .expect("key"),
            first_height: key["firstHeight"].as_u64().expect("first height"),
            last_height: key["lastHeight"].as_u64().expect("last height"),
        }]
    }

    fn commit_evidence(&self) -> Vec<BankCommitEvidence> {
        transcript()["records"]
            .as_array()
            .expect("records")
            .iter()
            .map(|record| {
                let batch = unhex(record["batch"].as_str().expect("node sequence batch"));
                let proof = &record["proof"];
                let mut leaf = b"LXAR\x01".to_vec();
                leaf.extend_from_slice(
                    &proof["total"]
                        .as_str()
                        .expect("proof total")
                        .parse::<i64>()
                        .expect("total")
                        .to_be_bytes(),
                );
                leaf.extend_from_slice(
                    &proof["index"]
                        .as_str()
                        .expect("proof index")
                        .parse::<i64>()
                        .expect("index")
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
                                .expect("Comet transaction leaf hash")
                        )
                        .expect("base64 leaf hash"),
                    expected_leaf_hash.as_slice(),
                    "the recorded Comet transaction proof commits the transaction hash"
                );
                let mut fields = vec![leaf];
                fields.extend(
                    proof["aunts"]
                        .as_array()
                        .expect("Comet proof aunts")
                        .iter()
                        .map(|aunt| {
                            base64::engine::general_purpose::STANDARD
                                .decode(aunt.as_str().expect("aunt"))
                                .expect("base64 aunt")
                        }),
                );
                BankCommitEvidence {
                    chain_id: record["material"]["signedHeader"]["header"]["chain_id"]
                        .as_str()
                        .expect("chain")
                        .to_owned(),
                    trusted_validators: record["validators"].clone(),
                    committed_header: record["material"]["signedHeader"].clone(),
                    covering_height: record["height"].as_u64().expect("covering height"),
                    batch_transaction: batch,
                    transaction_proof: fields,
                    activity_ids: record["activityIds"]
                        .as_array()
                        .expect("activity ids")
                        .iter()
                        .map(|id| {
                            unhex(id.as_str().expect("id"))
                                .try_into()
                                .expect("id width")
                        })
                        .collect(),
                }
            })
            .collect()
    }

    fn receipt(
        &self,
        hash: [u8; 32],
        keys: &[ReceiptKeyEntry],
        rung: Rung,
    ) -> BankCommittedReceipt {
        let proof = self
            .commit_evidence()
            .into_iter()
            .find(|proof| proof.activity_ids.contains(&hash))
            .expect("recorded coverage");
        let receipt = proof
            .authenticate(&self.endpoint, hash, keys)
            .expect("verify signed receipt and committed batch");
        assert!(receipt.rung() >= rung);
        receipt
    }
}

impl Drop for BankNode {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .expect("peer worker")
            .join()
            .expect("loopback peer shutdown");
    }
}

fn pending<T>(clock: &dyn Clock, mut operation: impl FnMut() -> Result<T, SequencerError>) -> T {
    let mut deadline = Deadline::start(clock, Duration::from_secs(20)).expect("pending deadline");
    loop {
        match operation() {
            Ok(value) => return value,
            Err(SequencerError::Rpc(refusal))
                if refusal.message == "pending block unavailable: sequencer: no block is open"
                    || refusal.message == "sequencer: no block is open" =>
            {
                assert!(
                    !deadline.remaining(clock).expect("pending clock").is_zero(),
                    "sequencer did not open a block"
                );
                thread::sleep(Duration::from_millis(10));
            }
            Err(error) => panic!("real sequencer refused bank operation: {error:?}"),
        }
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

#[path = "../../../../platform/hosted/runtime-clock/src/server.rs"]
mod clock_server;

use layerx_client::runtime_clock::ClockBinding;
use layerx_human_service::custody::{
    CustodySigner, EnvelopeKms, KeyClass, KeyEntropy, Keystore, SigningLimits,
};
use layerx_human_service::journeys::{
    JourneyEngine, JourneyKind, JourneyPhase, JourneyState, MoveAuthorization, MoveJourney,
    MoveStage,
};
use layerx_human_service::store::{PrincipalStore, Table};
use layerx_human_service::trace::TraceId;
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use std::future::Future;
use std::pin::pin;
use std::sync::atomic::{AtomicBool, Ordering};
use std::task::{Context, Poll, Wake, Waker};

struct RunningClock {
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<std::io::Result<()>>>,
    clock: Arc<RuntimeClock>,
}
impl RunningClock {
    fn start(root: &Path) -> Self {
        let mut server = clock_server::Server::bind(root).expect("real runtime clock");
        let binding = ClockBinding {
            socket: server.socket().to_owned(),
            process: std::process::id(),
            user: rustix::process::geteuid().as_raw(),
        };
        let stop = Arc::new(AtomicBool::new(false));
        let shutdown = stop.clone();
        let worker = thread::spawn(move || {
            while !shutdown.load(Ordering::Acquire) {
                server.poll()?;
                thread::sleep(Duration::from_millis(1));
            }
            Ok(())
        });
        let clock = RuntimeClock::connect(binding).expect("connect real runtime clock");
        Self {
            stop,
            worker: Some(worker),
            clock,
        }
    }
    fn now(&self) -> u64 {
        self.clock
            .sample(Duration::from_secs(1))
            .expect("clock sample")
            .unix_seconds()
    }
}
impl Drop for RunningClock {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .expect("clock worker")
            .join()
            .expect("clock thread")
            .expect("clock shutdown");
    }
}
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
        &[ActivityType::new(ModuleId::Asset, 5).expect("send activity")],
    )
    .expect("asset registration")])
    .expect("registry")
}

#[test]
fn bank_plan_constructor_commit_reopen_two_sends() {
    let root = layerx_human_test_support::directory("repair-bank-durable");
    fs::create_dir_all(&root).expect("bank test root");
    let clock = RunningClock::start(&root);
    let node = BankNode::start(clock.clock.clone());
    let sender_probe = Eip1559Call {
        chain_id: 125,
        nonce: 0,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 21_000,
        to: [0x42; 20],
        data: vec![],
    };
    let raw = sender_probe
        .sign(&node.secret)
        .expect("real funded node identity");
    let (call, signature) = decode_signed(&raw).expect("sender probe");
    let sender = recover_sender(&call, &signature).expect("node sender");
    let keys = node.receipt_keys();
    let pointer = unhex(transcript()["pointer"].as_str().expect("recorded pointer"))
        .try_into()
        .expect("pointer width");
    let nonce = pending(node.clock.as_ref(), || node.endpoint.pending_nonce(&sender));
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
    .expect("accepted two-transfer constructor");
    assert!(movement.route().legs().is_empty());
    let map = tenancy(&[("alice", "tenant-a")]);
    let store_root = root.join("store");
    let retention = retention_uniform(10_000);
    let (mut store, digest) = install_and_open(&store_root, &map, retention);
    let owner = principal("alice");
    let secret_path = root.join("kms-root");
    fs::write(&secret_path, [0x42; 64]).expect("local envelope root");
    let keystore = Keystore::open_development(
        root.join("custody"),
        125,
        EnvelopeKms::new("file-kms://repair", &secret_path).expect("real envelope KMS"),
    )
    .expect("keystore");
    let key = KeyId::new("human-primary").expect("key");
    keystore
        .generate(
            &owner,
            &key,
            KeyClass::HumanPrimary,
            KeyEntropy::new(node.secret, [0x52; 16], [0x53; 24]).expect("entropy"),
        )
        .expect("generate actual signer");
    let (custody_store, _) = install_and_open(&root.join("custody-store"), &map, retention);
    let custody = CustodySigner::new(
        keystore,
        custody_store,
        registry(),
        SigningLimits::new(100, 10_000).expect("signing limits"),
    );
    assert_eq!(
        custody.evm_wallet(&owner, &key).expect("custody wallet"),
        sender
    );
    let id = JourneyId::new("jrn_movebanksend").expect("id");
    let trace = TraceId::mint([0x45; 16]);
    let mut journey = {
        let mut scope = store.principal(&owner).expect("owner scope");
        let journey = MoveJourney::commit(
            &mut scope,
            &movement,
            MoveAuthorization::Allowed,
            &registry(),
            clock.now(),
        )
        .expect("commit accepted constructor");
        assert_eq!(journey.status().expect("status").legs().len(), 2);
        assert_eq!(journey.rung().expect("initial rung"), Rung::Submitted);
        assert_eq!(
            MoveJourney::commit(
                &mut scope,
                &movement,
                MoveAuthorization::Allowed,
                &registry(),
                clock.now()
            )
            .expect("idempotent commit"),
            journey
        );
        journey
    };
    let mut before = Vec::new();
    for to in [[0x22; 20], [0x55; 20]] {
        before.push(pending(node.clock.as_ref(), || node.endpoint.balance(&to)));
    }
    // Stop at durable Signed for leg one, before any send, then reopen the actual store.
    for _ in 0..3 {
        let mut scope = store.principal(&owner).expect("scope");
        ready(journey.advance_bank(
            &mut scope,
            &node.endpoint,
            &keys,
            &custody,
            &trace,
            clock.now(),
        ))
        .expect("prepare and sign bank leg");
    }
    let first_raw = journey
        .signed_transaction(0)
        .expect("durable signed first leg")
        .to_vec();
    assert_eq!(
        decode_signed(&first_raw).expect("signed call").0,
        movement.bank_transfers()[0]
    );
    assert!(journey.signed_transaction(1).is_none());
    drop(journey);
    drop(store);
    store = PrincipalStore::open(&store_root, retention, digest).expect("reopen real store");
    journey = MoveJourney::load(&store.principal(&owner).expect("recovered scope"), &id)
        .expect("load")
        .expect("committed bank journey");
    assert_eq!(journey.signed_transaction(0), Some(first_raw.as_slice()));
    let mut deadline =
        Deadline::start(clock.clock.as_ref(), Duration::from_secs(90)).expect("advance deadline");
    loop {
        let mut scope = store.principal(&owner).expect("advance scope");
        ready(journey.advance_bank(
            &mut scope,
            &node.endpoint,
            &keys,
            &custody,
            &trace,
            clock.now(),
        ))
        .expect("real bank transport and receipt verification");
        if journey.status().expect("status").stage() == MoveStage::Done {
            break;
        }
        assert!(
            !deadline
                .remaining(clock.clock.as_ref())
                .expect("clock")
                .is_zero(),
            "bank journey did not reach verified receipts"
        );
        thread::sleep(Duration::from_millis(10));
    }
    assert!(journey.rung().expect("receipted rung") >= Rung::Receipted);
    let raws: Vec<_> = (0..2)
        .map(|index| {
            journey
                .signed_transaction(index)
                .expect("signed leg")
                .to_vec()
        })
        .collect();
    let receipt_bytes: Vec<_> = (0..2)
        .map(|index| {
            journey
                .verified_leg_evidence(index)
                .expect("evidence")
                .expect("verified")
                .canonical_receipt
        })
        .collect();
    drop(journey);
    drop(store);
    store = PrincipalStore::open(&store_root, retention, digest).expect("reopen after receipts");
    journey = MoveJourney::load(&store.principal(&owner).expect("scope"), &id)
        .expect("reopen receipts")
        .expect("journey");
    let committed_evidence = node.commit_evidence();
    let hash = transaction_hash(&raws[0]);
    let Lookup::Verified(unsigned) = lookup_authenticated(
        &node.endpoint,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )
    .expect("receipt lookup") else {
        panic!("recorded receipt")
    };
    assert_eq!(
        unsigned.rung(),
        Rung::Receipted,
        "RPC landing alone never establishes commitment"
    );
    for damage in 0..4 {
        let mut invalid = committed_evidence[0].clone();
        match damage {
            0 => invalid.batch_transaction[100] ^= 1,
            1 => invalid.activity_ids[0][0] ^= 1,
            2 => invalid.covering_height += 1,
            _ => {
                invalid.trusted_validators["validators"][0]["votingPower"] = serde_json::json!("11")
            }
        }
        assert!(
            invalid.authenticate(&node.endpoint, hash, &keys).is_err(),
            "damaged consensus coverage must be refused"
        );
    }
    while journey.rung().expect("rung") != Rung::Committed {
        let mut scope = store.principal(&owner).expect("scope");
        ready(journey.advance_bank_with_commit(
            &mut scope,
            &node.endpoint,
            &keys,
            &committed_evidence,
            &custody,
            &trace,
            clock.now(),
        ))
        .expect("recover committed receipts");
        assert!(
            !deadline
                .remaining(clock.clock.as_ref())
                .expect("clock")
                .is_zero(),
            "bank journey did not commit"
        );
        thread::sleep(Duration::from_millis(10));
    }
    let status = journey.status().expect("completed status");
    assert_eq!(status.actual_amount(), Some(130));
    assert!(status.actual_fees().expect("actual fees") <= 2 * ceiling);
    assert_eq!(status.receipt_references().len(), 2);
    for index in 0..2 {
        assert_eq!(
            journey.signed_transaction(index),
            Some(raws[index].as_slice())
        );
        let (call, signature) = decode_signed(&raws[index]).expect("recovered signed transaction");
        assert_eq!(call, movement.bank_transfers()[index]);
        assert_eq!(call.nonce, nonce + index as u64);
        assert_eq!(
            recover_sender(&call, &signature).expect("signature"),
            sender
        );
        let evidence = journey
            .verified_leg_evidence(index)
            .expect("evidence")
            .expect("verified leg");
        assert_eq!(evidence.rung, Rung::Committed);
        assert_eq!(evidence.tx_hash, transaction_hash(&raws[index]));
        assert_eq!(evidence.canonical_receipt, receipt_bytes[index]);
        let protocol =
            layerx_wire::receipt::decode(&evidence.canonical_receipt).expect("node receipt");
        let protocol = protocol.protocol().expect("protocol");
        assert_eq!(protocol.module_id(), 12);
        assert_eq!(
            protocol.operation(),
            2,
            "EVM operation is the envelope type, separately from native send ordinal 5"
        );
        assert_eq!(protocol.debit_sequence(), call.nonce);
        assert_eq!(protocol.activity_id(), transaction_hash(&raws[index]));
        let to = [if index == 0 { 0x22 } else { 0x55 }; 20];
        let after = pending(node.clock.as_ref(), || node.endpoint.balance(&to));
        let before = u128::from_be_bytes(before[index][16..].try_into().expect("balance"));
        assert_eq!(
            u128::from_be_bytes(after[16..].try_into().expect("balance")) - before,
            (if index == 0 { 90 } else { 40 }) * 1_000_000_000_000_u128
        );
    }
    let committed = node.receipt(transaction_hash(&raws[0]), &keys, Rung::Committed);
    assert_eq!(committed.rung(), Rung::Committed);
    let first = committed.receipt();
    let mut checked_plan = movement.clone();
    assert!(
        checked_plan.complete_bank_leg(1, &raws[0], &first).is_err(),
        "cross-leg receipt reuse is refused"
    );
    let mut wrong_call = movement.bank_transfers()[0].clone();
    wrong_call.data[35] ^= 1;
    let wrong_raw = wrong_call
        .sign(&node.secret)
        .expect("genuine signature on a different bank call");
    assert!(
        checked_plan
            .complete_bank_leg(0, &wrong_raw, &first)
            .is_err(),
        "same nonce and real signature do not authorize a different bank call"
    );
    let wrong_signer = movement.bank_transfers()[0]
        .sign(&[0x66; 32])
        .expect("different real signer");
    assert!(
        checked_plan
            .complete_bank_leg(0, &wrong_signer, &first)
            .is_err(),
        "identity substitution is refused"
    );
    assert!(
        execution(9, nonce + 1, ceiling)
            .complete_from_receipt(transaction_hash(&raws[0]), &first)
            .is_err(),
        "receipt nonce is bound"
    );
    assert!(
        execution(9, nonce, 0)
            .complete_from_receipt(transaction_hash(&raws[0]), &first)
            .is_err(),
        "fee ceiling is retained"
    );
    checked_plan
        .complete_bank_leg(0, &raws[0], &first)
        .expect("exact bank transfer and authenticated receipt");
    assert_eq!(
        pending(node.clock.as_ref(), || node.endpoint.pending_nonce(&sender)),
        nonce + 2,
        "recovery did not create a duplicate send"
    );
    drop(journey);
    drop(store);
    let mut store = PrincipalStore::open(&store_root, retention, digest).expect("reopen committed");
    let recovered = MoveJourney::load(&store.principal(&owner).expect("scope"), &id)
        .expect("load committed")
        .expect("journey");
    assert_eq!(
        recovered.rung().expect("durable committed rung"),
        Rung::Committed
    );
    assert_eq!(recovered.status().expect("durable status"), status);
    for (index, proof) in committed_evidence.iter().enumerate() {
        assert_eq!(recovered.bank_commit_evidence(index), Some(proof),
            "reopen retains the exact validator quorum, signed batch and transaction inclusion proof");
    }
    drop(recovered);
    drop(store);
    drop(custody);
    drop(node);
    drop(clock);
    fs::remove_dir_all(root).expect("remove private test state");
}

use layerx_human_service::journeys::{JourneyError, JourneyLeg, JourneyPlan};
use layerx_intents::{BudgetDefund, Intent, IntentKind};
use layerx_types::account::AccountId;
use layerx_types::intent::BudgetId;
#[test]
fn history_state_schema_get_list_after_reopen() {
    let root = directory("journey-retired-kind");
    let retention = retention_uniform(10_000);
    let map = tenancy(&[("alice", "alice-tenant")]);
    let (mut store, digest) = install_and_open(&root, &map, retention);
    let owner = principal("alice");
    let id = JourneyId::new("jrn_retiredwithdraw").expect("journey id");
    let activity = ActivityType::new(ModuleId::Budget, 8).expect("activity");
    let registry = ModuleRegistry::new(&[
        ModuleRegistration::new(ModuleId::Budget, &[activity]).expect("module")
    ])
    .expect("registry");
    let intent = Intent::v1(IntentKind::BudgetDefund(
        BudgetDefund::new(
            BudgetId::new([0x21; 32]),
            AccountId::parse("agent:did:layerx:alice:budget:travel").expect("budget account"),
            AccountId::parse("agent:did:layerx:alice:main").expect("owner account"),
            AssetId::new([0x22; 32]),
            Amount::from_u128(37),
            Sequence::from_u64(7),
            IdempotencyKey::new([0x31; 32]),
        )
        .expect("intent"),
    ));
    let plan = JourneyPlan::new(
        id.clone(),
        JourneyKind::Move,
        [0x41; 32],
        KeyId::new("human-primary").expect("custody key"),
        Operation::ProtocolMutation,
        vec![JourneyLeg::new(
            intent,
            [0x31; 32],
            AgentDid::new("did:layerx:alice").expect("actor"),
            AuthorityRef::new("custody-human-primary").expect("authority"),
            7,
            19,
            100,
            200,
            1_000,
        )
        .expect("leg")],
    )
    .expect("plan");
    {
        let mut scope = store.principal(&owner).expect("scope");
        let live = JourneyEngine::start(&mut scope, &plan, &registry, 100).expect("persist plan");
        assert_eq!(
            live.status().expect("live status").state(),
            JourneyState::GettingReady
        );
        let row = scope
            .keys(Table::Journeys)
            .into_iter()
            .find(|key| key.as_str().starts_with("journey-"))
            .expect("journey row");
        let mut historical: Value = serde_json::from_slice(
            scope
                .get(Table::Journeys, &row)
                .expect("stored plan")
                .bytes(),
        )
        .expect("production record");
        assert_eq!(historical["legs"][0]["preparation"]["account_sequence"], 7);
        assert_eq!(historical["legs"][0]["preparation"]["pending_nonce"], 19);
        historical["kind"] = Value::String("withdraw".to_owned());
        historical["legs"][0]["preparation"]
            .as_object_mut()
            .expect("preparation")
            .remove("pending_nonce");
        scope
            .put(
                Table::Journeys,
                row,
                100,
                serde_json::to_vec(&historical).expect("legacy encoding"),
            )
            .expect("persist historical record");
    }
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, digest).expect("restart store");
    {
        let mut scope = store.principal(&owner).expect("reopened scope");
        let historical = JourneyEngine::load(&scope, &id)
            .expect("load historical")
            .expect("journey");
        assert_eq!(historical.kind(), None);
        assert_eq!(historical.historical_kind(), Some("withdraw"));
        let status = historical.status().expect("historical status");
        assert_eq!(status.state(), JourneyState::Closed);
        assert_eq!(status.phases(), &[JourneyPhase::Compiled]);
        assert_eq!(status.receipt_digests(), &[None]);
        let listed = JourneyEngine::list(&scope).expect("list historical");
        assert_eq!(listed.len(), 1);
        assert_eq!(
            listed[0].status().expect("listed status").state(),
            JourneyState::Closed
        );
        assert!(matches!(
            JourneyEngine::start(&mut scope, &plan, &registry, 101),
            Err(JourneyError::IdempotencyConflict)
        ));
        assert_eq!(
            JourneyEngine::load(&scope, &id)
                .expect("reload")
                .expect("retained")
                .status()
                .expect("still closed")
                .state(),
            JourneyState::Closed
        );
    }
    {
        let scope = store.principal(&owner).expect("HTTP encoder scope");
        let historical = JourneyEngine::load(&scope, &id)
            .expect("get")
            .expect("history");
        let get = historical
            .historical_response()
            .expect("encode closed history")
            .expect("retired response");
        assert_eq!(get["state"], "closed");
        assert_eq!(
            serde_json::from_value::<JourneyState>(get["state"].clone()).expect("state consumer"),
            JourneyState::Closed
        );
        let journeys: Vec<_> = JourneyEngine::list(&scope)
            .expect("list")
            .iter()
            .map(|journey| {
                journey
                    .historical_response()
                    .expect("encode list member")
                    .expect("closed history")
            })
            .collect();
        let list = serde_json::json!({"journeys": journeys, "next_cursor": ""});
        let schema = layerx_human_service::server::schema::ApiSchema::v1()
            .expect("actual embedded HTTP schema");
        let get_result = schema.encode_response(
            schema.operation("journey.get").expect("get operation"),
            &get,
        );
        let list_result = schema.encode_response(
            schema.operation("journey.list").expect("list operation"),
            &list,
        );
        assert!(get_result.is_ok() && list_result.is_ok(),
            "closed history must pass the real HTTP get/list encoder after reopen: get={get_result:?}, list={list_result:?}");
    }
    for retired in ["wallet-binding", "deposit", "withdraw", "exit"] {
        assert!(
            serde_json::from_value::<JourneyKind>(Value::String(retired.to_owned())).is_err(),
            "retired kind {retired} must not construct a new plan"
        );
    }
    assert_eq!(
        serde_json::from_str::<JourneyKind>("\"move\"").expect("live kind"),
        JourneyKind::Move
    );
    drop(store);
    std::fs::remove_dir_all(root).expect("remove test store");
}
