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
    AccountId::parse(text).expect("protocol account")
}
fn owner() -> AccountId {
    account("agent:did:layerx:alice:main")
}
fn asset() -> AssetId {
    AssetId::new(Sha256::digest(b"uhpx").into())
}
pub fn unhex(text: &str) -> Vec<u8> {
    text.strip_prefix("0x")
        .unwrap_or(text)
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16).expect("hex byte")
        })
        .collect()
}
pub fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut text = String::from("0x");
    for byte in bytes {
        write!(text, "{byte:02x}").expect("hex output");
    }
    text
}
pub fn transcript() -> Value {
    serde_json::from_str(include_str!(
        "../../src/journeys/repair_22_31_153_transcript.json"
    ))
    .expect("committed node/sequencer and Comet producer transcript")
}
pub fn keys() -> Vec<ReceiptKeyEntry> {
    let document = transcript();
    let key = &document["key"];
    vec![ReceiptKeyEntry {
        key_id: unhex(key["keyId"].as_str().expect("key id"))
            .try_into()
            .expect("key id width"),
        public_key: unhex(key["publicKey"].as_str().expect("public key"))
            .try_into()
            .expect("key width"),
        first_height: key["firstHeight"].as_u64().expect("first height"),
        last_height: key["lastHeight"].as_u64().expect("last height"),
    }]
}
fn sender() -> [u8; 20] {
    unhex(transcript()["sender"].as_str().expect("recorded sender"))
        .try_into()
        .expect("sender width")
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
        network_id: NetworkId::new(125).expect("network"),
        protocol_version: ProtocolVersion::new(3).expect("protocol"),
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
    .expect("fees")
}
fn identity() -> MovementExecutionIdentity {
    MovementExecutionIdentity {
        principal: principal("act_alice"),
        tenant: AgentTenantId::new("tenant-alice").expect("tenant"),
        account: layerx_intents::canonical::account_id_for_protocol(&owner(), 3)
            .expect("account id"),
        wallet: EvmAddress::new(sender()),
        plan_id: [0x51; 32],
    }
}

pub struct StoreFixture {
    pub root: PathBuf,
    pub digest: TenancyDigest,
    pub store: Option<PrincipalStore>,
}
impl StoreFixture {
    pub fn new(label: &str) -> Self {
        let root = directory(label);
        let (store, digest) = install_and_open(
            &root,
            &tenancy(&[("act_alice", "tenant-alice"), ("bob", "tenant-bob")]),
            retention_uniform(10_000),
        );
        Self {
            root,
            digest,
            store: Some(store),
        }
    }
    pub fn store(&mut self) -> &mut PrincipalStore {
        self.store.as_mut().expect("open store")
    }
    pub fn restart(&mut self) {
        drop(self.store.take());
        self.store = Some(
            PrincipalStore::open(&self.root, retention_uniform(10_000), self.digest)
                .expect("reopen durable store"),
        );
    }
    fn row(&mut self) -> (RowKey, Value) {
        let scope = self
            .store()
            .principal(&principal("act_alice"))
            .expect("owner scope");
        let key = scope
            .keys(Table::Journeys)
            .into_iter()
            .find(|key| key.as_str().starts_with("journey-"))
            .expect("engine row");
        let record = serde_json::from_slice(
            scope
                .get(Table::Journeys, &key)
                .expect("engine record")
                .bytes(),
        )
        .expect("record encoding");
        (key, record)
    }
    fn put(&mut self, key: RowKey, record: &Value) {
        self.store()
            .principal(&principal("act_alice"))
            .expect("scope")
            .put(
                Table::Journeys,
                key,
                100,
                serde_json::to_vec(record).expect("record bytes"),
            )
            .expect("authenticated store write");
    }
    pub fn load(&mut self) -> Result<Option<JourneyEngine>, JourneyError> {
        JourneyEngine::load(
            &self
                .store()
                .principal(&principal("act_alice"))
                .expect("scope"),
            &JourneyId::new("jrn_repairreceipt").expect("id"),
        )
    }
    pub fn submitted(&mut self) {
        let registry = ModuleRegistry::new(&[ModuleRegistration::new(
            ModuleId::Asset,
            &[ActivityType::new(ModuleId::Asset, 5).expect("send")],
        )
        .expect("module")])
        .expect("registry");
        let request = bank_request(7);
        let execution = MoveLegExecution::new(
            [0x31; 32],
            AgentDid::new("did:layerx:alice").expect("actor"),
            AuthorityRef::new("custody-human-primary").expect("authority"),
            7,
            0,
            u64::MAX,
            1_000_000_000_000_000_000,
        )
        .expect("execution");
        let plan = MovePlan::new_bank_transfers(
            JourneyId::new("jrn_repairreceipt").expect("id"),
            [0x44; 32],
            KeyId::new("human-primary").expect("key"),
            Operation::ProtocolMutation,
            vec![request],
            vec![execution],
            100,
            "HPX",
            "a few seconds",
        )
        .expect("production bank plan");
        MoveJourney::commit(
            &mut self
                .store()
                .principal(&principal("act_alice"))
                .expect("scope"),
            &plan,
            MoveAuthorization::Allowed,
            &registry,
            100,
        )
        .expect("production engine record");
        // Restore the exact previously submitted pointer transaction from the
        // committed pre-migration node transcript. Recovery must authenticate
        // already-signed history without resolving it as a new current route.
        let document = transcript();
        let raw = unhex(
            document["records"][0]["raw"]
                .as_str()
                .expect("producer transaction"),
        );
        let (call, signature) = decode_signed(&raw).expect("real signed producer call");
        assert_eq!(recover_sender(&call, &signature).expect("sender"), sender());
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
                .expect("load submitted migration")
                .expect("journey")
                .status()
                .expect("status")
                .state(),
            JourneyState::Processing
        );
    }
}
impl Drop for StoreFixture {
    fn drop(&mut self) {
        drop(self.store.take());
        fs::remove_dir_all(&self.root).expect("remove private fixture");
    }
}

pub struct RpcPeer {
    pub endpoint: SequencerEndpoint,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}
impl RpcPeer {
    fn start(answer: impl Fn(&Value) -> Value + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback RPC");
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let endpoint = SequencerEndpoint::new(
            &format!("http://{}/", listener.local_addr().expect("address")),
            &[],
        )
        .expect("production client");
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
                    .expect("bounded socket");
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).expect("HTTP header"), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(value.trim().parse::<usize>().expect("length"));
                    }
                }
                let mut body = vec![0; length.expect("content length")];
                assert!(body.len() <= 65_536);
                reader.read_exact(&mut body).expect("RPC body");
                let request: Value = serde_json::from_slice(&body).expect("RPC JSON");
                let result = answer(&request);
                observed
                    .lock()
                    .expect("call inventory")
                    .push(request.clone());
                let body =
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                let mut stream = reader.into_inner();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).expect("response");
                stream.flush().expect("flush");
            }
        });
        Self {
            endpoint,
            calls,
            stop,
            worker: Some(worker),
        }
    }
    pub fn receipts() -> Self {
        let document = transcript();
        Self::start(move |request| {
            let record = &document["records"][0];
            match request["method"].as_str().expect("method") {
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
            .expect("worker")
            .join()
            .expect("RPC peer thread");
    }
}

pub fn commit_proof() -> BankCommitEvidence {
    let document = transcript();
    let record = &document["records"][0];
    let proof = &record["proof"];
    let batch = unhex(record["batch"].as_str().expect("signed batch"));
    let mut leaf = b"LXAR\x01".to_vec();
    leaf.extend(
        proof["total"]
            .as_str()
            .expect("total")
            .parse::<i64>()
            .expect("total integer")
            .to_be_bytes(),
    );
    leaf.extend(
        proof["index"]
            .as_str()
            .expect("index")
            .parse::<i64>()
            .expect("index integer")
            .to_be_bytes(),
    );
    leaf.extend(Sha256::digest(&batch));
    let mut fields = vec![leaf];
    fields.extend(
        proof["aunts"]
            .as_array()
            .expect("aunts")
            .iter()
            .map(|aunt| {
                base64::engine::general_purpose::STANDARD
                    .decode(aunt.as_str().expect("aunt"))
                    .expect("base64 proof")
            }),
    );
    BankCommitEvidence {
        chain_id: record["material"]["signedHeader"]["header"]["chain_id"]
            .as_str()
            .expect("chain")
            .to_owned(),
        trusted_validators: record["validators"].clone(),
        committed_header: record["material"]["signedHeader"].clone(),
        covering_height: record["height"].as_u64().expect("height"),
        batch_transaction: batch,
        transaction_proof: fields,
        activity_ids: vec![unhex(record["view"]["txHash"].as_str().expect("hash"))
            .try_into()
            .expect("hash width")],
    }
}
