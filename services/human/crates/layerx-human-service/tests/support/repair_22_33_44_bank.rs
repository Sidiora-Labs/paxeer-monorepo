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
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_service::custody::{KeyId, Operation};
use layerx_human_service::journeys::*;
use layerx_human_service::notify::JourneyId;
use layerx_human_service::store::{PrincipalStore, RowKey, Table, TenancyDigest};
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, EvmAddress, NetworkId, ProtocolVersion, PublicKey,
    SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

fn asset() -> AssetId {
    AssetId::new(Sha256::digest(b"uhpx").into())
}
pub fn unhex(text: &str) -> Vec<u8> {
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
pub fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    let mut text = String::from("0x");
    for byte in bytes {
        write!(text, "{byte:02x}").or_fail("hex output");
    }
    text
}
pub fn transcript() -> Value {
    serde_json::from_str(include_str!(
        "../../src/journeys/repair_22_31_153_transcript.json"
    ))
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "committed node/sequencer and Comet producer transcript"
        )
    })
}
pub fn keys() -> Vec<ReceiptKeyEntry> {
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
        self.store.as_mut().or_fail("open store")
    }
    pub fn restart(&mut self) {
        drop(self.store.take());
        self.store = Some(
            PrincipalStore::open(&self.root, retention_uniform(10_000), self.digest)
                .or_fail("reopen durable store"),
        );
    }
    fn row(&mut self) -> (RowKey, Value) {
        let scope = self
            .store()
            .principal(&principal("act_alice"))
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
            .principal(&principal("act_alice"))
            .or_fail("scope")
            .put(
                Table::Journeys,
                key,
                100,
                serde_json::to_vec(record).or_fail("record bytes"),
            )
            .or_fail("authenticated store write");
    }
    pub fn load(&mut self) -> Result<Option<JourneyEngine>, JourneyError> {
        JourneyEngine::load(
            &self
                .store()
                .principal(&principal("act_alice"))
                .or_fail("scope"),
            &JourneyId::new("jrn_repairreceipt").or_fail("id"),
        )
    }
    pub fn submitted(&mut self) {
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
            &mut self
                .store()
                .principal(&principal("act_alice"))
                .or_fail("scope"),
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

pub struct RpcPeer {
    pub endpoint: SequencerEndpoint,
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
            stop,
            worker: Some(worker),
        }
    }
    pub fn receipts() -> Self {
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

pub fn commit_proof() -> BankCommitEvidence {
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

use crate::require_value::RequireValue as _;
