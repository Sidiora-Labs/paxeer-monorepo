use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use base64::Engine as _;
use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_service::custody::{KeyId, Operation};
use layerx_human_service::journeys::*;
use layerx_human_service::notify::JourneyId;
use layerx_human_service::store::{AgentTenantId, PrincipalStore, RowKey, Table, TenancyDigest};
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, EvmAddress, NetworkId, ProtocolVersion, PublicKey,
    SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

fn account(text: &str) -> AccountId {
    AccountId::parse(text).or_fail("protocol account")
}
fn owner() -> AccountId {
    account("agent:did:layerx:alice:main")
}
fn asset() -> AssetId {
    AssetId::new(Sha256::digest(b"uhpx").into())
}
fn unhex(text: &str) -> Vec<u8> {
    text.strip_prefix("0x")
        .unwrap_or(text)
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).or_fail("hex text"), 16)
                .or_fail("hex byte")
        })
        .collect()
}
fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut text = String::from("0x");
    for byte in bytes {
        write!(text, "{byte:02x}").or_fail("hex output");
    }
    text
}
fn transcript() -> Value {
    serde_json::from_str(include_str!(
        "../src/journeys/repair_22_31_153_transcript.json"
    ))
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "committed node/sequencer and Comet producer transcript"
        )
    })
}
fn keys() -> Vec<ReceiptKeyEntry> {
    let document = transcript();
    let key = &document["key"];
    vec![ReceiptKeyEntry {
        key_id: unhex(key["keyId"].as_str().or_fail("key id"))
            .try_into()
            .or_fail("key id width"),
        public_key: unhex(key["publicKey"].as_str().or_fail("public key"))
            .try_into()
            .or_fail("key width"),
        first_height: key["firstHeight"].as_u64().or_fail("first height"),
        last_height: key["lastHeight"].as_u64().or_fail("last height"),
    }]
}
fn sender() -> [u8; 20] {
    unhex(transcript()["sender"].as_str().or_fail("recorded sender"))
        .try_into()
        .or_fail("sender width")
}
fn send_route(nonce: u64) -> SendRoute {
    SendRoute {
        account_sequence: Sequence::from_u64(nonce),
        idempotency_key: IdempotencyKey::new([0x31; 32]),
        expires_at: TimestampSeconds::from_u64(u64::MAX),
        context_hash: ContextHash::new([0; 32]),
        authorization: SendAuthorization::new(
            SendAuthorizationKind::Owner,
            PublicKey::new([0x11; 32]),
            AuthorizationSignature::new([0x22; 64]),
        ),
        network_id: NetworkId::new(125).or_fail("network"),
        protocol_version: ProtocolVersion::new(3).or_fail("protocol"),
    }
}
fn bank_request(nonce: u64) -> RouteRequest {
    RouteRequest {
        source: Endpoint::PaxeerWallet,
        destination: Endpoint::PaxeerWallet,
        relationship: Relationship::BankTransfer {
            from: EvmAddress::new(sender()),
            to: EvmAddress::new([0x22; 20]),
            route: send_route(nonce),
            pointer: EvmAddress::new(BANK_ADDRESS),
            denom: "uhpx".to_owned(),
            chain_id: 125,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1_000_000_000_000,
            gas_limit: 1_000_000,
        },
        asset: asset(),
        amount: Amount::from_u128(90),
    }
}
fn fees() -> FeeSchedule {
    FeeSchedule::new(vec![
        (LegMechanism::Protocol(Mechanism::Send), 5),
        (LegMechanism::Protocol(Mechanism::BudgetFund), 2),
        (LegMechanism::Protocol(Mechanism::BudgetDefund), 2),
    ])
    .or_fail("fees")
}
fn identity() -> MovementExecutionIdentity {
    MovementExecutionIdentity {
        principal: principal("alice"),
        tenant: AgentTenantId::new("tenant-alice").or_fail("tenant"),
        account: layerx_intents::canonical::account_id_for_protocol(&owner(), 3)
            .or_fail("account id"),
        wallet: EvmAddress::new(sender()),
        plan_id: [0x51; 32],
    }
}

struct StoreFixture {
    root: PathBuf,
    digest: TenancyDigest,
    store: Option<PrincipalStore>,
}
impl StoreFixture {
    fn new(label: &str) -> Self {
        let root = directory(label);
        let (store, digest) = install_and_open(
            &root,
            &tenancy(&[("alice", "tenant-alice"), ("bob", "tenant-bob")]),
            retention_uniform(10_000),
        );
        Self {
            root,
            digest,
            store: Some(store),
        }
    }
    fn store(&mut self) -> &mut PrincipalStore {
        self.store.as_mut().or_fail("open store")
    }
    fn restart(&mut self) {
        drop(self.store.take());
        self.store = Some(
            PrincipalStore::open(&self.root, retention_uniform(10_000), self.digest)
                .or_fail("reopen durable store"),
        );
    }
    fn row(&mut self) -> (RowKey, Value) {
        let scope = self
            .store()
            .principal(&principal("alice"))
            .or_fail("owner scope");
        let key = scope
            .keys(Table::Journeys)
            .into_iter()
            .find(|key| key.as_str().starts_with("journey-"))
            .or_fail("engine row");
        let record = serde_json::from_slice(
            scope
                .get(Table::Journeys, &key)
                .or_fail("engine record")
                .bytes(),
        )
        .or_fail("record encoding");
        (key, record)
    }
    fn put(&mut self, key: RowKey, record: &Value) {
        self.store()
            .principal(&principal("alice"))
            .or_fail("scope")
            .put(
                Table::Journeys,
                key,
                100,
                serde_json::to_vec(record).or_fail("record bytes"),
            )
            .or_fail("authenticated store write");
    }
    fn load(&mut self) -> Result<Option<JourneyEngine>, JourneyError> {
        JourneyEngine::load(
            &self.store().principal(&principal("alice")).or_fail("scope"),
            &JourneyId::new("jrn_repairreceipt").or_fail("id"),
        )
    }
    fn submitted(&mut self) {
        let registry = ModuleRegistry::new(&[ModuleRegistration::new(
            ModuleId::Asset,
            &[ActivityType::new(ModuleId::Asset, 5).or_fail("send")],
        )
        .or_fail("module")])
        .or_fail("registry");
        let request = bank_request(7);
        let execution = MoveLegExecution::new(
            [0x31; 32],
            AgentDid::new("did:layerx:alice").or_fail("actor"),
            AuthorityRef::new("custody-human-primary").or_fail("authority"),
            7,
            0,
            u64::MAX,
            1_000_000_000_000_000_000,
        )
        .or_fail("execution");
        let plan = MovePlan::new_bank_transfers(
            JourneyId::new("jrn_repairreceipt").or_fail("id"),
            [0x44; 32],
            KeyId::new("human-primary").or_fail("key"),
            Operation::ProtocolMutation,
            vec![request],
            vec![execution],
            100,
            "HPX",
            "a few seconds",
        )
        .or_fail("production bank plan");
        MoveJourney::commit(
            &mut self.store().principal(&principal("alice")).or_fail("scope"),
            &plan,
            MoveAuthorization::Allowed,
            &registry,
            100,
        )
        .or_fail("production engine record");
        // Restore the exact previously submitted pointer transaction from the
        // committed pre-migration node transcript. Recovery must authenticate
        // already-signed history without resolving it as a new current route.
        let document = transcript();
        let raw = unhex(
            document["records"][0]["raw"]
                .as_str()
                .or_fail("producer transaction"),
        );
        let (call, signature) = decode_signed(&raw).or_fail("real signed producer call");
        assert_eq!(
            recover_sender(&call, &signature).or_fail("sender"),
            sender()
        );
        let unsigned = call.encode_unsigned();
        let digest: [u8; 32] = Sha256::digest(&unsigned).into();
        let (key, mut record) = self.row();
        let leg = &mut record["legs"][0];
        leg["payload"] = json!(unsigned);
        leg["payload_hash"] = json!(digest);
        leg["bank"] = json!({"unsigned": unsigned, "sender": sender(), "denom":"uhpx"});
        leg["phase"] = json!("submitted");
        leg["prepared_ref"] = json!("bank-recorded-submission");
        leg["prepared_digest"] = json!(digest);
        leg["signed"] = json!({"preparation_ref":"bank-recorded-submission", "canonical_digest":digest,
            "signature": signature.as_bytes().to_vec(), "signer_public_key":vec![0x61;32], "raw_transaction": raw});
        leg["submission_ref"] = json!("bank-recorded-submission");
        leg["activity_id"] = json!(transaction_hash(&raw));
        self.put(key, &record);
        self.restart();
        assert_eq!(
            self.load()
                .or_fail("load submitted migration")
                .or_fail("journey")
                .status()
                .or_fail("status")
                .state(),
            JourneyState::Processing
        );
    }
}
impl Drop for StoreFixture {
    fn drop(&mut self) {
        drop(self.store.take());
        fs::remove_dir_all(&self.root).or_fail("remove private fixture");
    }
}

struct RpcPeer {
    endpoint: SequencerEndpoint,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}
impl RpcPeer {
    fn start(answer: impl Fn(&Value) -> Value + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").or_fail("loopback RPC");
        listener
            .set_nonblocking(true)
            .or_fail("nonblocking listener");
        let endpoint = SequencerEndpoint::new(
            &format!("http://{}/", listener.local_addr().or_fail("address")),
            &[],
        )
        .or_fail("production client");
        let stop = Arc::new(AtomicBool::new(false));
        let shutdown = stop.clone();
        let calls = Arc::new(Mutex::new(Vec::new()));
        let observed = calls.clone();
        let worker = thread::spawn(move || {
            while !shutdown.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(value) => value,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .or_fail("bounded socket");
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).or_fail("HTTP header"), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().or_fail("length"));
                    }
                }
                let mut body = vec![0; length.or_fail("content length")];
                assert!(body.len() <= 65_536);
                reader.read_exact(&mut body).or_fail("RPC body");
                let request: Value = serde_json::from_slice(&body).or_fail("RPC JSON");
                let result = answer(&request);
                observed
                    .lock()
                    .or_fail("call inventory")
                    .push(request.clone());
                let body =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).or_fail("response");
                stream.flush().or_fail("flush");
            }
        });
        Self {
            endpoint,
            calls,
            stop,
            worker: Some(worker),
        }
    }
    fn receipts() -> Self {
        let document = transcript();
        Self::start(move |request| {
            let record = &document["records"][0];
            match request["method"].as_str().or_fail("method") {
                "pax_sequencerReceipt" => {
                    assert_eq!(request["params"][0], record["view"]["txHash"]);
                    record["view"].clone()
                }
                "eth_getTransactionReceipt" => {
                    assert_eq!(request["params"][0], record["view"]["txHash"]);
                    record["landing"].clone()
                }
                "px_getCommit" => record["material"].clone(),
                "px_getValidators" => record["validators"].clone(),
                "eth_getBlockByNumber" if request["params"][1] == false => {
                    json!({"number":request["params"][0], "stateRoot":record["appHash"]})
                }
                "eth_getBlockByNumber" => record["block"].clone(),
                other => panic!("unrecorded producer method {other}"),
            }
        })
    }
}
impl Drop for RpcPeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .or_fail("worker")
            .join()
            .or_fail("RPC peer thread");
    }
}

fn commit_proof() -> BankCommitEvidence {
    let document = transcript();
    let record = &document["records"][0];
    let proof = &record["proof"];
    let batch = unhex(record["batch"].as_str().or_fail("signed batch"));
    let mut leaf = b"LXAR\x01".to_vec();
    leaf.extend(
        proof["total"]
            .as_str()
            .or_fail("total")
            .parse::<i64>()
            .or_fail("total integer")
            .to_be_bytes(),
    );
    leaf.extend(
        proof["index"]
            .as_str()
            .or_fail("index")
            .parse::<i64>()
            .or_fail("index integer")
            .to_be_bytes(),
    );
    leaf.extend(Sha256::digest(&batch));
    let mut fields = vec![leaf];
    fields.extend(
        proof["aunts"]
            .as_array()
            .or_fail("aunts")
            .iter()
            .map(|aunt| {
                base64::engine::general_purpose::STANDARD
                    .decode(aunt.as_str().or_fail("aunt"))
                    .or_fail("base64 proof")
            }),
    );
    BankCommitEvidence {
        chain_id: record["material"]["signedHeader"]["header"]["chain_id"]
            .as_str()
            .or_fail("chain")
            .to_owned(),
        trusted_validators: record["validators"].clone(),
        committed_header: record["material"]["signedHeader"].clone(),
        covering_height: record["height"].as_u64().or_fail("height"),
        batch_transaction: batch,
        transaction_proof: fields,
        activity_ids: vec![unhex(record["view"]["txHash"].as_str().or_fail("hash"))
            .try_into()
            .or_fail("hash width")],
    }
}

#[test]
fn receipt_authority_retained_chain_lookup_and_reopen() {
    let mut fixture = StoreFixture::new("chain-receipt-recovery");
    fixture.submitted();
    let node = RpcPeer::receipts();
    let mut engine = fixture.load().or_fail("load").or_fail("journey");
    let status = engine
        .refresh_bank_receipt(
            &mut fixture
                .store()
                .principal(&principal("alice"))
                .or_fail("scope"),
            &node.endpoint,
            &keys(),
            0,
            101,
        )
        .or_fail("retrieve without caller batch");
    assert_eq!(status.state(), JourneyState::Done);
    let evidence = engine
        .verified_leg_evidence(0)
        .or_fail("evidence")
        .or_fail("verified");
    assert_eq!(evidence.rung, Rung::Receipted);
    assert_eq!(evidence.receipt_key, keys()[0]);
    assert_eq!(
        evidence.tx_hash,
        transaction_hash(engine.signed_transaction(0).or_fail("signed transaction"))
    );
    assert_eq!(evidence.signed_receipt, evidence.canonical_receipt);
    assert!(
        fixture.row().1["legs"][0]
            .get("receipt_authority")
            .is_none(),
        "current evidence never persists caller-supplied authority"
    );
    drop(engine);
    fixture.restart();
    let recovered = fixture.load().or_fail("reopen receipt").or_fail("journey");
    assert_eq!(recovered.status().or_fail("recovered status"), status);
    assert_eq!(
        recovered
            .verified_leg_evidence(0)
            .or_fail("recovered evidence"),
        Some(evidence)
    );
    assert!(node
        .calls
        .lock()
        .or_fail("calls")
        .iter()
        .any(|request| request["method"] == "pax_sequencerReceipt"));
    assert!(JourneyEngine::load(
        &fixture
            .store()
            .principal(&principal("bob"))
            .or_fail("other tenant"),
        &JourneyId::new("jrn_repairreceipt").or_fail("id")
    )
    .or_fail("isolated tenant lookup")
    .is_none());
}

#[test]
fn verified_evidence_reload_refuses_missing_or_changed_evidence_before_done() {
    let mut fixture = StoreFixture::new("verified-invariant-recovery");
    fixture.submitted();
    let node = RpcPeer::receipts();
    let mut engine = fixture.load().or_fail("load").or_fail("journey");
    engine
        .refresh_bank_receipt(
            &mut fixture
                .store()
                .principal(&principal("alice"))
                .or_fail("scope"),
            &node.endpoint,
            &keys(),
            0,
            101,
        )
        .or_fail("complete real receipt");
    drop(engine);
    let (row, valid) = fixture.row();
    for damage in 0..17 {
        let mut damaged = valid.clone();
        let leg = &mut damaged["legs"][0];
        match damage {
            0 => {
                leg.as_object_mut().or_fail("leg").remove("rung_evidence");
            }
            1 => leg["receipt"] = Value::Null,
            2 => leg["receipt_digest"] = Value::Null,
            3 => {
                let original = leg["rung_evidence"]["signed_receipt"][0]
                    .as_u64()
                    .or_fail("receipt byte");
                leg["rung_evidence"]["signed_receipt"][0] = json!(original ^ 1);
            }
            4 => leg["rung_evidence"]["public_key"] = json!(vec![0x99; 32]),
            5 => leg["rung_evidence"]["key_id"] = json!(vec![0x99; 32]),
            6 => leg["rung_evidence"]["inclusion_height"] = json!(0),
            7 => leg["rung_evidence"]["first_height"] = json!(9),
            8 => leg["rung_evidence"]["last_height"] = json!(7),
            9 => leg["rung_evidence"]["tx_hash"] = json!(vec![0x99; 32]),
            10 => leg["rung_evidence"]["rung"] = json!(0),
            11 => leg["rung_evidence"]["rung"] = json!(2),
            12 => leg["signed"]["raw_transaction"] = Value::Null,
            13 => leg["preparation"]["pending_nonce"] = json!(8),
            14 => leg["preparation"]["fee_limit"] = json!(0),
            15 => leg["preparation"]["not_after"] = json!(1),
            _ => leg["receipt_digest"] = json!(vec![0x99; 32]),
        }
        fixture.put(row.clone(), &damaged);
        fixture.restart();
        assert!(
            fixture.load().is_err(),
            "damage {damage} must be refused during load, before status can expose Done"
        );
        assert!(
            JourneyEngine::list(
                &fixture
                    .store()
                    .principal(&principal("alice"))
                    .or_fail("scope")
            )
            .is_err(),
            "list must also refuse damaged completed evidence"
        );
    }
    fixture.put(row.clone(), &valid);
    fixture.restart();
    assert_eq!(
        fixture
            .load()
            .or_fail("valid restoration")
            .or_fail("journey")
            .status()
            .or_fail("status")
            .state(),
        JourneyState::Done
    );
    // Authenticated migration retains complete chain-key evidence, ignoring the
    // retired authority column. Evidence-less retired history stays closed.
    let mut historical = valid;
    historical["kind"] = json!("withdraw");
    historical["legs"][0]
        .as_object_mut()
        .or_fail("leg")
        .remove("rung_evidence");
    fixture.put(row, &historical);
    fixture.restart();
    assert_eq!(
        fixture
            .load()
            .or_fail("read-only history")
            .or_fail("history")
            .status()
            .or_fail("closed status")
            .state(),
        JourneyState::Closed
    );
}

#[test]
fn committed_evidence_reload_rechecks_quorum_batch_and_inclusion() {
    let mut fixture = StoreFixture::new("committed-invariant-recovery");
    fixture.submitted();
    let node = RpcPeer::receipts();
    let proof = commit_proof();
    let mut engine = fixture.load().or_fail("load").or_fail("journey");
    engine
        .refresh_bank_receipt_with_commit(
            &mut fixture
                .store()
                .principal(&principal("alice"))
                .or_fail("scope"),
            &node.endpoint,
            &keys(),
            Some(&proof),
            0,
            101,
        )
        .or_fail("real consensus producer proof");
    assert_eq!(
        engine
            .verified_leg_evidence(0)
            .or_fail("evidence")
            .or_fail("verified")
            .rung,
        Rung::Committed
    );
    drop(engine);
    fixture.restart();
    assert_eq!(
        fixture
            .load()
            .or_fail("reopen committed")
            .or_fail("journey")
            .verified_leg_evidence(0)
            .or_fail("evidence")
            .or_fail("verified")
            .rung,
        Rung::Committed
    );
    let (row, valid) = fixture.row();
    for damage in 0..5 {
        let mut bad = valid.clone();
        let proof = &mut bad["legs"][0]["bank_commit"];
        match damage {
            0 => proof["batch_transaction"][100] = json!(0),
            1 => proof["activity_ids"][0][0] = json!(0xff),
            2 => proof["transaction_proof"][0][22] = json!(0xff),
            3 => proof["trusted_validators"]["validators"][0]["votingPower"] = json!("11"),
            _ => proof["committed_header"]["header"]["app_hash"] = json!("00".repeat(32)),
        }
        fixture.put(row.clone(), &bad);
        fixture.restart();
        assert!(
            fixture.load().is_err(),
            "retained consensus damage {damage} must be refused"
        );
    }
}

fn assert_wrong_route_identity_refused(
    scope: &layerx_human_service::store::PrincipalScope<'_>,
    route: &RouteRequest,
) {
    let mut wrong = identity();
    wrong.tenant = AgentTenantId::new("tenant-bob").or_fail("tenant");
    assert!(matches!(
        ObservedStateBuilder::for_route(
            scope,
            &wrong,
            owner(),
            TimestampSeconds::from_u64(100),
            route
        ),
        Err(ObservationError::IdentityMismatch)
    ));
    wrong = identity();
    wrong.principal = principal("bob");
    assert!(matches!(
        ObservedStateBuilder::for_route(
            scope,
            &wrong,
            owner(),
            TimestampSeconds::from_u64(100),
            route
        ),
        Err(ObservationError::IdentityMismatch)
    ));
    wrong = identity();
    wrong.account = [0x99; 32];
    assert!(matches!(
        ObservedStateBuilder::for_route(
            scope,
            &wrong,
            owner(),
            TimestampSeconds::from_u64(100),
            route
        ),
        Err(ObservationError::IdentityMismatch)
    ));
    wrong = identity();
    wrong.wallet = EvmAddress::new([0x99; 20]);
    assert!(matches!(
        ObservedStateBuilder::for_route(
            scope,
            &wrong,
            owner(),
            TimestampSeconds::from_u64(100),
            route
        ),
        Err(ObservationError::IdentityMismatch)
    ));
}

fn bank_balance_peer() -> RpcPeer {
    RpcPeer::start(
        |request| match request["method"].as_str().or_fail("method") {
            "eth_getTransactionCount" => {
                assert_eq!(request["params"], json!([hex(&sender()), "pending"]));
                json!("0x7")
            }
            "eth_call" => {
                assert_eq!(request["params"][1], "pending");
                assert_eq!(request["params"][0]["to"], hex(&BANK_ADDRESS));
                let data = unhex(request["params"][0]["data"].as_str().or_fail("data"));
                assert_eq!(
                    &data[..4],
                    &layerx_crypto::disclosure::call::ledger_selector("balance(address,string)")
                );
                assert_eq!(&data[16..36], &sender());
                assert_eq!(&data[100..104], b"uhpx");
                let mut amount = [0; 32];
                amount[30..].copy_from_slice(&4096_u16.to_be_bytes());
                json!(hex(&amount))
            }
            method => panic!("unexpected bank read {method}"),
        },
    )
}

#[test]
fn observation_requires_current_route_and_authenticated_pending_bank_read() {
    let mut fixture = StoreFixture::new("route-observations");
    let node = bank_balance_peer();
    let scope = fixture
        .store()
        .principal(&principal("alice"))
        .or_fail("scope");
    let route = bank_request(7);
    let observed = ObservedStateBuilder::for_route(
        &scope,
        &identity(),
        owner(),
        TimestampSeconds::from_u64(100),
        &route,
    )
    .or_fail("authenticated route")
    .read_pending_bank_balance(&node.endpoint, "uhpx")
    .or_fail("production pending bank client")
    .with_allowances(vec![])
    .with_budget_bindings(vec![])
    .with_fees(fees())
    .build()
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "bank route without reserve, withdrawals or joined wallet"
        )
    });
    assert_eq!(
        observed
            .available(&Endpoint::PaxeerWallet, asset())
            .require()
            .or_fail("observed"),
        Amount::from_u128(4096)
    );
    assert_eq!(node.calls.lock().or_fail("calls").len(), 3);
    assert_wrong_route_identity_refused(&scope, &route);
    let internal = RouteRequest {
        source: home(),
        destination: agent(),
        relationship: Relationship::Direct(send_route(7)),
        asset: asset(),
        amount: Amount::from_u128(90),
    };
    let ledger = ObservedStateBuilder::for_route(
        &scope,
        &identity(),
        owner(),
        TimestampSeconds::from_u64(100),
        &internal,
    )
    .or_fail("internal route")
    .with_ledger_balances(vec![balance(home(), 1000)])
    .with_allowances(vec![])
    .with_budget_bindings(vec![])
    .with_fees(fees())
    .build()
    .or_fail("ledger route needs no joined wallet");
    let intent = UnifiedIntent::new(
        home(),
        agent(),
        asset(),
        Amount::from_u128(90),
        Constraints::new(TimestampSeconds::from_u64(2000), 100, false),
    )
    .or_fail("send intent");
    assert_eq!(
        plan(&intent, &ledger)
            .or_fail("real current-route planner")
            .legs()
            .len(),
        1
    );
    let absent = RpcPeer::start(|request| {
        if request["method"] == "eth_getTransactionCount" {
            json!("0x7")
        } else {
            Value::Null
        }
    });
    assert!(matches!(
        ObservedStateBuilder::for_route(
            &scope,
            &identity(),
            owner(),
            TimestampSeconds::from_u64(100),
            &route
        )
        .or_fail("route")
        .read_pending_bank_balance(&absent.endpoint, "uhpx"),
        Err(ObservationError::ChainReadUnavailable)
    ));
}

#[test]
fn bank_target_drift_producer_matches_bank_consumer_at_pending_nonce() {
    let node = RpcPeer::start(|request| {
        assert_eq!(request["method"], "eth_getTransactionCount");
        assert_eq!(request["params"][1], "pending");
        json!("0x7")
    });
    let nonce = node
        .endpoint
        .pending_nonce(&sender())
        .or_fail("pending nonce");
    let request = bank_request(nonce);
    let route = RouteResolver::resolve(&request).or_fail("canonical bank route");
    let [call] = route.bank_transfers() else {
        panic!("one bank call")
    };
    assert_eq!(call.to, BANK_ADDRESS);
    assert_eq!(call.nonce, nonce);
    let mut amount = [0; 32];
    amount[31] = 90;
    let disclosure = bind_call(call, sender()).or_fail("real bank call consumer");
    assert_eq!(
        disclosure.construct_method().or_fail("decode bank ABI"),
        Some(ConstructMethod::BankSend {
            from: sender(),
            to: [0x22; 20],
            denom: "uhpx".to_owned(),
            amount
        })
    );
    // The consumer is the production signing/disclosure decoder, and its ABI
    // agrees with the actual bank precompile's frozen interface.
    let bank_abi: Value =
        serde_json::from_str(include_str!("../../../../../precompiles/bank/abi.json"))
            .or_fail("bank ABI");
    let method = bank_abi
        .as_array()
        .or_fail("ABI methods")
        .iter()
        .find(|method| {
            method["name"] == "send" && method["inputs"].as_array().or_fail("inputs").len() == 4
        })
        .or_fail("actual bank send consumer");
    let types: Vec<_> = method["inputs"]
        .as_array()
        .or_fail("inputs")
        .iter()
        .map(|input| input["type"].as_str().or_fail("ABI type"))
        .collect();
    assert_eq!(
        &call.data[..4],
        &layerx_crypto::disclosure::call::ledger_selector(&format!("send({})", types.join(",")))
    );
    let raw = call
        .sign(
            &unhex(
                transcript()["secret"]
                    .as_str()
                    .or_fail("synthetic producer secret"),
            )
            .try_into()
            .or_fail("secret width"),
        )
        .or_fail("real transaction signer");
    let (decoded, signature) = decode_signed(&raw).or_fail("consumer transaction codec");
    assert_eq!(decoded, *call);
    assert_eq!(
        recover_sender(&decoded, &signature).or_fail("sender"),
        sender()
    );
    let mut wrong = request.clone();
    wrong.asset = AssetId::new([0x99; 32]);
    assert!(
        RouteResolver::resolve(&wrong).is_err(),
        "denomination and asset must agree"
    );
    let Relationship::BankTransfer { pointer, .. } = &mut wrong.relationship else {
        panic!("bank")
    };
    *pointer = EvmAddress::new([0x88; 20]);
    wrong.asset = asset();
    assert!(
        RouteResolver::resolve(&wrong).is_err(),
        "arbitrary pointer cannot acquire canonical bank authority"
    );
}

fn home() -> Endpoint {
    Endpoint::Human(owner())
}
fn budget() -> Endpoint {
    Endpoint::AgentBudget(account("agent:did:layerx:bob:budget:one"))
}
fn agent() -> Endpoint {
    Endpoint::Agent(account("agent:did:layerx:bob:main"))
}
fn top_up_allowance() -> SignedAllowance {
    SignedAllowance::new(
        AllowanceId::new([0x61; 32]).or_fail("allowance id"),
        AllowanceKind::BudgetAllowance,
        AllowanceScope::new(
            home(),
            budget(),
            asset(),
            LegMechanism::Protocol(Mechanism::BudgetFund),
        ),
        Amount::from_u128(1000),
        Amount::from_u128(1000),
        TimestampSeconds::from_u64(5000),
    )
    .or_fail("signed inventory")
}
fn ledger(balances: Vec<BalanceEntry>, allowances: Vec<SignedAllowance>) -> ObservedState {
    ObservedState::new(
        TimestampSeconds::from_u64(100),
        owner(),
        None,
        balances,
        allowances,
        vec![
            BudgetBinding::new(account("agent:did:layerx:bob:budget:one"), owner())
                .or_fail("budget owner"),
        ],
        fees(),
    )
    .or_fail("real snapshot")
}
fn balance(endpoint: Endpoint, amount: u128) -> BalanceEntry {
    BalanceEntry::new(endpoint, asset(), Amount::from_u128(amount))
}
#[test]
fn missing_balance_is_refused_separately_from_zero_and_authorized_top_up() {
    let intent = UnifiedIntent::new(
        budget(),
        agent(),
        asset(),
        Amount::from_u128(100),
        Constraints::new(TimestampSeconds::from_u64(2000), 1000, true),
    )
    .or_fail("intent");
    let missing = ledger(vec![balance(home(), 1000)], vec![top_up_allowance()]);
    assert_eq!(
        missing.available(&budget(), asset()),
        BalanceObservation::Unknown {
            endpoint: budget(),
            asset: asset()
        }
    );
    assert_eq!(
        plan(&intent, &missing),
        Err(Refusal::BalanceUnknown {
            endpoint: budget(),
            asset: asset()
        })
    );
    let zero = ledger(vec![balance(home(), 1000), balance(budget(), 0)], vec![]);
    assert_eq!(
        zero.available(&budget(), asset())
            .require()
            .or_fail("known zero"),
        Amount::ZERO
    );
    assert_eq!(
        plan(&intent, &zero),
        Err(Refusal::TopUpNotAuthorized { shortfall: 105 })
    );
    let authorized = ledger(
        vec![balance(home(), 1000), balance(budget(), 0)],
        vec![top_up_allowance()],
    );
    let planned = plan(&intent, &authorized).or_fail("authorized top-up after a genuine zero");
    assert_eq!(planned.legs().len(), 2);
    assert!(planned.legs()[0].top_up().is_some());
    assert_eq!(planned.legs()[0].source(), &home());
    assert_eq!(planned.legs()[0].destination(), &budget());
    assert_eq!(
        authorized.allowances()[0].remaining_total(),
        Amount::from_u128(1000),
        "pure planning never consumes allowance headroom"
    );
    let missing_origin = ledger(vec![balance(budget(), 0)], vec![top_up_allowance()]);
    assert_eq!(
        plan(&intent, &missing_origin),
        Err(Refusal::BalanceUnknown {
            endpoint: home(),
            asset: asset()
        })
    );
    let mut fixture = StoreFixture::new("missing-ledger-builder");
    let scope = fixture
        .store()
        .principal(&principal("alice"))
        .or_fail("scope");
    let request = RouteRequest {
        source: budget(),
        destination: agent(),
        relationship: Relationship::Direct(send_route(7)),
        asset: asset(),
        amount: Amount::from_u128(100),
    };
    assert_eq!(
        ObservedStateBuilder::for_route(
            &scope,
            &identity(),
            owner(),
            TimestampSeconds::from_u64(100),
            &request
        )
        .or_fail("route")
        .with_ledger_balances(vec![balance(home(), 1000)])
        .with_allowances(vec![top_up_allowance()])
        .with_budget_bindings(vec![])
        .with_fees(fees())
        .build(),
        Err(ObservationError::Invalid(Refusal::BalanceUnknown {
            endpoint: budget(),
            asset: asset()
        }))
    );
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
