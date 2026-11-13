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
        asset: AssetId::new([0x33; 32]),
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

// Recorded once using node/sequencer's signing/receipt codecs, node RPC view
// validation, and Comet's header, vote, batch and transaction-proof producers.
fn transcript() -> Value {
    serde_json::from_str(include_str!(
        "../src/journeys/repair_22_31_153_transcript.json"
    ))
    .or_fail("committed Go producer transcript")
}

struct BankNode {
    clock: Arc<RuntimeClock>,
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
    ) -> Value {
        let params = &request["params"];
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
        result
    }

    fn serve(listener: &TcpListener, fixture: &Value, stopped: &Arc<AtomicBool>) {
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
            let result = Self::rpc_result(&request, fixture, records, &mut sent);
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

    fn start(clock: Arc<RuntimeClock>) -> Self {
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
        let worker = thread::spawn(move || Self::serve(&listener, &fixture, &stopped));
        Self {
            clock,
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

    fn receipt(
        &self,
        hash: [u8; 32],
        keys: &[ReceiptKeyEntry],
        rung: Rung,
    ) -> BankCommittedReceipt {
        let proof = Self::commit_evidence()
            .into_iter()
            .find(|proof| proof.activity_ids.contains(&hash))
            .or_fail("recorded coverage");
        let receipt = proof
            .authenticate(&self.endpoint, hash, keys)
            .unwrap_or_else(|error| {
                panic!("{}: {error:?}", "verify signed receipt and committed batch")
            });
        assert!(receipt.rung() >= rung);
        receipt
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

fn pending<T>(clock: &dyn Clock, mut operation: impl FnMut() -> Result<T, SequencerError>) -> T {
    let mut deadline = Deadline::start(clock, Duration::from_secs(20)).or_fail("pending deadline");
    loop {
        match operation() {
            Ok(value) => return value,
            Err(SequencerError::Rpc(refusal))
                if refusal.message == "pending block unavailable: sequencer: no block is open"
                    || refusal.message == "sequencer: no block is open" =>
            {
                assert!(
                    !deadline.remaining(clock).or_fail("pending clock").is_zero(),
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
        let mut server = clock_server::Server::bind(root).or_fail("real runtime clock");
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
        let clock = RuntimeClock::connect(binding).or_fail("connect real runtime clock");
        Self {
            stop,
            worker: Some(worker),
            clock,
        }
    }
    fn now(&self) -> u64 {
        self.clock
            .sample(Duration::from_secs(1))
            .or_fail("clock sample")
            .unix_seconds()
    }
}
impl Drop for RunningClock {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .or_fail("clock worker")
            .join()
            .or_fail("clock thread")
            .or_fail("clock shutdown");
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
        &[ActivityType::new(ModuleId::Asset, 5).or_fail("send activity")],
    )
    .or_fail("asset registration")])
    .or_fail("registry")
}

struct BankSetup {
    root: std::path::PathBuf,
    clock: RunningClock,
    node: BankNode,
    sender: [u8; 20],
    keys: Vec<ReceiptKeyEntry>,
    nonce: u64,
    ceiling: u128,
    movement: MovePlan,
    store: PrincipalStore,
    digest: layerx_human_service::store::TenancyDigest,
    owner: layerx_human_service::store::PrincipalId,
    custody: CustodySigner,
    id: JourneyId,
    trace: TraceId,
    retention: layerx_human_service::store::RetentionPolicy,
    store_root: std::path::PathBuf,
}

fn bank_setup() -> BankSetup {
    let root = layerx_human_test_support::directory("repair-bank-durable");
    fs::create_dir_all(&root).or_fail("bank test root");
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
        .or_fail("real funded node identity");
    let (call, signature) = decode_signed(&raw).or_fail("sender probe");
    let sender = recover_sender(&call, &signature).or_fail("node sender");
    let keys = BankNode::receipt_keys();
    let pointer = unhex(transcript()["pointer"].as_str().or_fail("recorded pointer"))
        .try_into()
        .or_fail("pointer width");
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
    .or_fail("accepted two-transfer constructor");
    assert!(movement.route().legs().is_empty());
    let map = tenancy(&[("alice", "tenant-a")]);
    let store_root = root.join("store");
    let retention = retention_uniform(10_000);
    let (store, digest) = install_and_open(&store_root, &map, retention);
    let owner = principal("alice");
    let secret_path = root.join("kms-root");
    fs::write(&secret_path, [0x42; 64]).or_fail("local envelope root");
    let keystore = Keystore::open_development(
        root.join("custody"),
        125,
        EnvelopeKms::new("file-kms://repair", &secret_path).or_fail("real envelope KMS"),
    )
    .or_fail("keystore");
    let key = KeyId::new("human-primary").or_fail("key");
    keystore
        .generate(
            &owner,
            &key,
            KeyClass::HumanPrimary,
            KeyEntropy::new(node.secret, [0x52; 16], [0x53; 24]).or_fail("entropy"),
        )
        .or_fail("generate actual signer");
    let (custody_store, _) = install_and_open(&root.join("custody-store"), &map, retention);
    let custody = CustodySigner::new(
        keystore,
        custody_store,
        registry(),
        SigningLimits::new(100, 10_000).or_fail("signing limits"),
    );
    assert_eq!(
        custody.evm_wallet(&owner, &key).or_fail("custody wallet"),
        sender
    );
    let id = JourneyId::new("jrn_movebanksend").or_fail("id");
    let trace = TraceId::mint([0x45; 16]);
    BankSetup {
        root,
        clock,
        node,
        sender,
        keys,
        nonce,
        ceiling,
        movement,
        store,
        digest,
        owner,
        custody,
        id,
        trace,
        retention,
        store_root,
    }
}

#[derive(Clone, Copy)]
struct BankChecks<'a> {
    journey: &'a MoveJourney,
    node: &'a BankNode,
    keys: &'a [ReceiptKeyEntry],
    raws: &'a [Vec<u8>],
    receipt_bytes: &'a [Vec<u8>],
    movement: &'a MovePlan,
    before: &'a [[u8; 32]],
    nonce: u64,
    ceiling: u128,
    sender: [u8; 20],
}

fn assert_committed_receipts(checks: BankChecks<'_>) -> layerx_human_service::journeys::MoveStatus {
    let BankChecks {
        journey,
        node,
        raws,
        receipt_bytes,
        movement,
        before,
        nonce,
        ceiling,
        sender,
        ..
    } = checks;
    let status = journey.status().or_fail("completed status");
    assert_eq!(status.actual_amount(), Some(130));
    assert!(status.actual_fees().or_fail("actual fees") <= 2 * ceiling);
    assert_eq!(status.receipt_references().len(), 2);
    for index in 0..2 {
        assert_eq!(
            journey.signed_transaction(index),
            Some(raws[index].as_slice())
        );
        let (call, signature) = decode_signed(&raws[index]).or_fail("recovered signed transaction");
        assert_eq!(call, movement.bank_transfers()[index]);
        assert_eq!(call.nonce, nonce + index as u64);
        assert_eq!(
            recover_sender(&call, &signature).or_fail("signature"),
            sender
        );
        let evidence = journey
            .verified_leg_evidence(index)
            .or_fail("evidence")
            .or_fail("verified leg");
        assert_eq!(evidence.rung, Rung::Committed);
        assert_eq!(evidence.tx_hash, transaction_hash(&raws[index]));
        assert_eq!(evidence.canonical_receipt, receipt_bytes[index]);
        let protocol =
            layerx_wire::receipt::decode(&evidence.canonical_receipt).or_fail("node receipt");
        let protocol = protocol.protocol().or_fail("protocol");
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
        let before = u128::from_be_bytes(before[index][16..].try_into().or_fail("balance"));
        assert_eq!(
            u128::from_be_bytes(after[16..].try_into().or_fail("balance")) - before,
            (if index == 0 { 90 } else { 40 }) * 1_000_000_000_000_u128
        );
    }
    status
}

fn assert_bank_receipt_binding(checks: BankChecks<'_>) {
    let BankChecks {
        node,
        keys,
        raws,
        movement,
        nonce,
        ceiling,
        sender,
        ..
    } = checks;
    let committed = node.receipt(transaction_hash(&raws[0]), keys, Rung::Committed);
    assert_eq!(committed.rung(), Rung::Committed);
    let first = committed.receipt();
    let mut checked_plan = movement.clone();
    assert!(
        checked_plan.complete_bank_leg(1, &raws[0], first).is_err(),
        "cross-leg receipt reuse is refused"
    );
    let mut wrong_call = movement.bank_transfers()[0].clone();
    wrong_call.data[35] ^= 1;
    let wrong_raw = wrong_call.sign(&node.secret).unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "genuine signature on a different bank call"
        )
    });
    assert!(
        checked_plan
            .complete_bank_leg(0, &wrong_raw, first)
            .is_err(),
        "same nonce and real signature do not authorize a different bank call"
    );
    let wrong_signer = movement.bank_transfers()[0]
        .sign(&[0x66; 32])
        .or_fail("different real signer");
    assert!(
        checked_plan
            .complete_bank_leg(0, &wrong_signer, first)
            .is_err(),
        "identity substitution is refused"
    );
    assert!(
        execution(9, nonce + 1, ceiling)
            .complete_from_receipt(transaction_hash(&raws[0]), first)
            .is_err(),
        "receipt nonce is bound"
    );
    assert!(
        execution(9, nonce, 0)
            .complete_from_receipt(transaction_hash(&raws[0]), first)
            .is_err(),
        "fee ceiling is retained"
    );
    checked_plan
        .complete_bank_leg(0, &raws[0], first)
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "exact bank transfer and authenticated receipt"
            )
        });
    assert_eq!(
        pending(node.clock.as_ref(), || node.endpoint.pending_nonce(&sender)),
        nonce + 2,
        "recovery did not create a duplicate send"
    );
}

struct BankAdvance<'a> {
    store: &'a mut PrincipalStore,
    journey: &'a mut MoveJourney,
    node: &'a BankNode,
    keys: &'a [ReceiptKeyEntry],
    custody: &'a CustodySigner,
    trace: &'a TraceId,
    owner: &'a layerx_human_service::store::PrincipalId,
    clock: &'a RunningClock,
    movement: &'a MovePlan,
}

fn assert_first_bank_leg(context: BankAdvance<'_>) -> (Vec<[u8; 32]>, Vec<u8>) {
    let BankAdvance {
        store,
        journey,
        node,
        keys,
        custody,
        trace,
        owner,
        clock,
        movement,
    } = context;
    let mut before = Vec::new();
    for to in [[0x22; 20], [0x55; 20]] {
        before.push(pending(node.clock.as_ref(), || node.endpoint.balance(&to)));
    }
    // Stop at durable Signed for leg one, before any send, then reopen the actual store.
    for _ in 0..3 {
        let mut scope = store.principal(owner).or_fail("scope");
        ready(journey.advance_bank(
            &mut scope,
            &node.endpoint,
            keys,
            custody,
            trace,
            clock.now(),
        ))
        .or_fail("prepare and sign bank leg");
    }
    let first_raw = journey
        .signed_transaction(0)
        .or_fail("durable signed first leg")
        .to_vec();
    assert_eq!(
        decode_signed(&first_raw).or_fail("signed call").0,
        movement.bank_transfers()[0]
    );
    assert!(journey.signed_transaction(1).is_none());
    (before, first_raw)
}

fn advance_to_bank_receipts(context: BankAdvance<'_>) -> (Deadline, Vec<Vec<u8>>, Vec<Vec<u8>>) {
    let BankAdvance {
        store,
        journey,
        node,
        keys,
        custody,
        trace,
        owner,
        clock,
        ..
    } = context;
    let mut deadline =
        Deadline::start(clock.clock.as_ref(), Duration::from_secs(90)).or_fail("advance deadline");
    loop {
        let mut scope = store.principal(owner).or_fail("advance scope");
        ready(journey.advance_bank(
            &mut scope,
            &node.endpoint,
            keys,
            custody,
            trace,
            clock.now(),
        ))
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "real bank transport and receipt verification"
            )
        });
        if journey.status().or_fail("status").stage() == MoveStage::Done {
            break;
        }
        assert!(
            !deadline
                .remaining(clock.clock.as_ref())
                .or_fail("clock")
                .is_zero(),
            "bank journey did not reach verified receipts"
        );
        thread::sleep(Duration::from_millis(10));
    }
    assert!(journey.rung().or_fail("receipted rung") >= Rung::Receipted);
    let raws: Vec<_> = (0..2)
        .map(|index| {
            journey
                .signed_transaction(index)
                .or_fail("signed leg")
                .to_vec()
        })
        .collect();
    let receipt_bytes: Vec<_> = (0..2)
        .map(|index| {
            journey
                .verified_leg_evidence(index)
                .or_fail("evidence")
                .or_fail("verified")
                .canonical_receipt
        })
        .collect();
    (deadline, raws, receipt_bytes)
}

fn assert_commit_coverage(
    context: BankAdvance<'_>,
    committed_evidence: &[BankCommitEvidence],
    raws: &[Vec<u8>],
    deadline: &mut Deadline,
) {
    let BankAdvance {
        store,
        journey,
        node,
        keys,
        custody,
        trace,
        owner,
        clock,
        ..
    } = context;
    let hash = transaction_hash(&raws[0]);
    let Lookup::Verified(unsigned) =
        lookup_authenticated(&node.endpoint, hash, LookupContext { receipt_keys: keys })
            .or_fail("receipt lookup")
    else {
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
                invalid.trusted_validators["validators"][0]["votingPower"] =
                    serde_json::json!("11");
            }
        }
        assert!(
            invalid.authenticate(&node.endpoint, hash, keys).is_err(),
            "damaged consensus coverage must be refused"
        );
    }
    while journey.rung().or_fail("rung") != Rung::Committed {
        let mut scope = store.principal(owner).or_fail("scope");
        ready(journey.advance_bank_with_commit(
            &mut scope,
            &node.endpoint,
            keys,
            committed_evidence,
            custody,
            trace,
            clock.now(),
        ))
        .or_fail("recover committed receipts");
        assert!(
            !deadline
                .remaining(clock.clock.as_ref())
                .or_fail("clock")
                .is_zero(),
            "bank journey did not commit"
        );
        thread::sleep(Duration::from_millis(10));
    }
}

fn commit_bank_journey(
    store: &mut PrincipalStore,
    owner: &layerx_human_service::store::PrincipalId,
    movement: &MovePlan,
    clock: &RunningClock,
) -> MoveJourney {
    let mut scope = store.principal(owner).or_fail("owner scope");
    let journey = MoveJourney::commit(
        &mut scope,
        movement,
        MoveAuthorization::Allowed,
        &registry(),
        clock.now(),
    )
    .or_fail("commit accepted constructor");
    assert_eq!(journey.status().or_fail("status").legs().len(), 2);
    assert_eq!(journey.rung().or_fail("initial rung"), Rung::Submitted);
    assert_eq!(
        MoveJourney::commit(
            &mut scope,
            movement,
            MoveAuthorization::Allowed,
            &registry(),
            clock.now()
        )
        .or_fail("idempotent commit"),
        journey
    );
    journey
}

fn assert_committed_reopen(
    store_root: &Path,
    retention: layerx_human_service::store::RetentionPolicy,
    digest: layerx_human_service::store::TenancyDigest,
    owner: &layerx_human_service::store::PrincipalId,
    id: &JourneyId,
    status: &layerx_human_service::journeys::MoveStatus,
    proofs: &[BankCommitEvidence],
) {
    let mut store = PrincipalStore::open(store_root, retention, digest).or_fail("reopen committed");
    let recovered = MoveJourney::load(&store.principal(owner).or_fail("scope"), id)
        .or_fail("load committed")
        .or_fail("journey");
    assert_eq!(
        recovered.rung().or_fail("durable committed rung"),
        Rung::Committed
    );
    assert_eq!(recovered.status().or_fail("durable status"), *status);
    for (index, proof) in proofs.iter().enumerate() {
        assert_eq!(recovered.bank_commit_evidence(index), Some(proof),
            "reopen retains the exact validator quorum, signed batch and transaction inclusion proof");
    }
    drop(recovered);
    drop(store);
}

#[test]
fn bank_plan_constructor_commit_reopen_two_sends() {
    let BankSetup {
        root,
        clock,
        node,
        sender,
        keys,
        nonce,
        ceiling,
        movement,
        mut store,
        digest,
        owner,
        custody,
        id,
        trace,
        retention,
        store_root,
    } = bank_setup();
    let mut journey = commit_bank_journey(&mut store, &owner, &movement, &clock);
    let (before, first_raw) = assert_first_bank_leg(BankAdvance {
        store: &mut store,
        journey: &mut journey,
        node: &node,
        keys: &keys,
        custody: &custody,
        trace: &trace,
        owner: &owner,
        clock: &clock,
        movement: &movement,
    });
    drop(journey);
    drop(store);
    store = PrincipalStore::open(&store_root, retention, digest).or_fail("reopen real store");
    journey = MoveJourney::load(&store.principal(&owner).or_fail("recovered scope"), &id)
        .or_fail("load")
        .or_fail("committed bank journey");
    assert_eq!(journey.signed_transaction(0), Some(first_raw.as_slice()));
    let (mut deadline, raws, receipt_bytes) = advance_to_bank_receipts(BankAdvance {
        store: &mut store,
        journey: &mut journey,
        node: &node,
        keys: &keys,
        custody: &custody,
        trace: &trace,
        owner: &owner,
        clock: &clock,
        movement: &movement,
    });
    drop(journey);
    drop(store);
    store = PrincipalStore::open(&store_root, retention, digest).or_fail("reopen after receipts");
    journey = MoveJourney::load(&store.principal(&owner).or_fail("scope"), &id)
        .or_fail("reopen receipts")
        .or_fail("journey");
    let committed_evidence = BankNode::commit_evidence();
    assert_commit_coverage(
        BankAdvance {
            store: &mut store,
            journey: &mut journey,
            node: &node,
            keys: &keys,
            custody: &custody,
            trace: &trace,
            owner: &owner,
            clock: &clock,
            movement: &movement,
        },
        &committed_evidence,
        &raws,
        &mut deadline,
    );
    let checks = BankChecks {
        journey: &journey,
        node: &node,
        keys: &keys,
        raws: &raws,
        receipt_bytes: &receipt_bytes,
        movement: &movement,
        before: &before,
        nonce,
        ceiling,
        sender,
    };
    let status = assert_committed_receipts(checks);
    assert_bank_receipt_binding(checks);
    drop(journey);
    drop(store);
    assert_committed_reopen(
        &store_root,
        retention,
        digest,
        &owner,
        &id,
        &status,
        &committed_evidence,
    );
    drop(custody);
    drop(node);
    drop(clock);
    fs::remove_dir_all(root).or_fail("remove private test state");
}

use layerx_human_service::journeys::{JourneyError, JourneyLeg, JourneyPlan};
use layerx_intents::{BudgetDefund, Intent, IntentKind};
use layerx_types::account::AccountId;
use layerx_types::intent::BudgetId;
fn historical_plan(id: &JourneyId) -> (ModuleRegistry, JourneyPlan) {
    let activity = ActivityType::new(ModuleId::Budget, 8).or_fail("activity");
    let registry = ModuleRegistry::new(&[
        ModuleRegistration::new(ModuleId::Budget, &[activity]).or_fail("module")
    ])
    .or_fail("registry");
    let intent = Intent::v1(IntentKind::BudgetDefund(
        BudgetDefund::new(
            BudgetId::new([0x21; 32]),
            AccountId::parse("agent:did:layerx:alice:budget:travel").or_fail("budget account"),
            AccountId::parse("agent:did:layerx:alice:main").or_fail("owner account"),
            AssetId::new([0x22; 32]),
            Amount::from_u128(37),
            Sequence::from_u64(7),
            IdempotencyKey::new([0x31; 32]),
        )
        .or_fail("intent"),
    ));
    let plan = JourneyPlan::new(
        id.clone(),
        JourneyKind::Move,
        [0x41; 32],
        KeyId::new("human-primary").or_fail("custody key"),
        Operation::ProtocolMutation,
        vec![JourneyLeg::new(
            intent,
            [0x31; 32],
            AgentDid::new("did:layerx:alice").or_fail("actor"),
            AuthorityRef::new("custody-human-primary").or_fail("authority"),
            7,
            19,
            100,
            200,
            1_000,
        )
        .or_fail("leg")],
    )
    .or_fail("plan");
    (registry, plan)
}

fn assert_historical_response(
    store: &mut PrincipalStore,
    owner: &layerx_human_service::store::PrincipalId,
    id: &JourneyId,
) {
    {
        let scope = store.principal(owner).or_fail("HTTP encoder scope");
        let historical = JourneyEngine::load(&scope, id)
            .or_fail("get")
            .or_fail("history");
        let get = historical
            .historical_response()
            .or_fail("encode closed history")
            .or_fail("retired response");
        assert_eq!(get["state"], "closed");
        assert_eq!(
            serde_json::from_value::<JourneyState>(get["state"].clone()).or_fail("state consumer"),
            JourneyState::Closed
        );
        let journeys: Vec<_> = JourneyEngine::list(&scope)
            .or_fail("list")
            .iter()
            .map(|journey| {
                journey
                    .historical_response()
                    .or_fail("encode list member")
                    .or_fail("closed history")
            })
            .collect();
        let list = serde_json::json!({"journeys": journeys, "next_cursor": ""});
        let schema = layerx_human_service::server::schema::ApiSchema::v1()
            .or_fail("actual embedded HTTP schema");
        let get_result = schema.encode_response(
            schema.operation("journey.get").or_fail("get operation"),
            &get,
        );
        let list_result = schema.encode_response(
            schema.operation("journey.list").or_fail("list operation"),
            &list,
        );
        assert!(get_result.is_ok() && list_result.is_ok(),
            "closed history must pass the real HTTP get/list encoder after reopen: get={get_result:?}, list={list_result:?}");
    }
}

#[test]
fn history_state_schema_get_list_after_reopen() {
    let root = directory("journey-retired-kind");
    let retention = retention_uniform(10_000);
    let map = tenancy(&[("alice", "alice-tenant")]);
    let (mut store, digest) = install_and_open(&root, &map, retention);
    let owner = principal("alice");
    let id = JourneyId::new("jrn_retiredwithdraw").or_fail("journey id");
    let (registry, plan) = historical_plan(&id);
    {
        let mut scope = store.principal(&owner).or_fail("scope");
        let live = JourneyEngine::start(&mut scope, &plan, &registry, 100).or_fail("persist plan");
        assert_eq!(
            live.status().or_fail("live status").state(),
            JourneyState::GettingReady
        );
        let row = scope
            .keys(Table::Journeys)
            .into_iter()
            .find(|key| key.as_str().starts_with("journey-"))
            .or_fail("journey row");
        let mut historical: Value = serde_json::from_slice(
            scope
                .get(Table::Journeys, &row)
                .or_fail("stored plan")
                .bytes(),
        )
        .or_fail("production record");
        assert_eq!(historical["legs"][0]["preparation"]["account_sequence"], 7);
        assert_eq!(historical["legs"][0]["preparation"]["pending_nonce"], 19);
        historical["kind"] = Value::String("withdraw".to_owned());
        historical["legs"][0]["preparation"]
            .as_object_mut()
            .or_fail("preparation")
            .remove("pending_nonce");
        scope
            .put(
                Table::Journeys,
                row,
                100,
                serde_json::to_vec(&historical).or_fail("legacy encoding"),
            )
            .or_fail("persist historical record");
    }
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, digest).or_fail("restart store");
    {
        let mut scope = store.principal(&owner).or_fail("reopened scope");
        let historical = JourneyEngine::load(&scope, &id)
            .or_fail("load historical")
            .or_fail("journey");
        assert_eq!(historical.kind(), None);
        assert_eq!(historical.historical_kind(), Some("withdraw"));
        let status = historical.status().or_fail("historical status");
        assert_eq!(status.state(), JourneyState::Closed);
        assert_eq!(status.phases(), &[JourneyPhase::Compiled]);
        assert_eq!(status.receipt_digests(), &[None]);
        let listed = JourneyEngine::list(&scope).or_fail("list historical");
        assert_eq!(listed.len(), 1);
        assert_eq!(
            listed[0].status().or_fail("listed status").state(),
            JourneyState::Closed
        );
        assert!(matches!(
            JourneyEngine::start(&mut scope, &plan, &registry, 101),
            Err(JourneyError::IdempotencyConflict)
        ));
        assert_eq!(
            JourneyEngine::load(&scope, &id)
                .or_fail("reload")
                .or_fail("retained")
                .status()
                .or_fail("still closed")
                .state(),
            JourneyState::Closed
        );
    }
    assert_historical_response(&mut store, &owner, &id);
    for retired in ["wallet-binding", "deposit", "withdraw", "exit"] {
        assert!(
            serde_json::from_value::<JourneyKind>(Value::String(retired.to_owned())).is_err(),
            "retired kind {retired} must not construct a new plan"
        );
    }
    assert_eq!(
        serde_json::from_str::<JourneyKind>("\"move\"").or_fail("live kind"),
        JourneyKind::Move
    );
    drop(store);
    std::fs::remove_dir_all(root).or_fail("remove test store");
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
