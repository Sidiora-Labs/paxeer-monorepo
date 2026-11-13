#[allow(
    dead_code,
    reason = "The host binary uses these signalling, deal and lap items; this test target exercises a subset."
)]
mod signal {
    include!("../src/inference_market/signal.rs");
    pub mod deals {
        include!("../src/inference_market/deals.rs");
    }
}

use std::fmt::Debug;
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::DirBuilderExt as _;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::ai_market::{
    inference_market as client, market_program as market, sign_market_envelope, FinalizedSnapshot,
    NativeTerms, OperationJournal, OperationRecord, OperationState,
};
use layerx_client::rpc::commit::verify_commit_material;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::evm_transaction;
use layerx_crypto::signer::{LocalSigner, Signer};
use layerx_programs::chain_proof::{
    decode::ProofOp,
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofVerifier, Commit, VerifiedChainHead,
};
use layerx_types::program_call::Resources;
use market::{codec, dispatch, queries, rooms, state, types::*, workers};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use signal::deals::{
    receive_deal, record_deal, send_deal, DealAcknowledgement, DealClientFrame, DealConfig,
    DealEngine, DealFrame, DealServerFrame,
};
use signal::{ClientFrame, HelloFrame, ServerFrame};
use tungstenite::{Message, WebSocket};

fn must<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("unexpected failure: {error:?}"))
}
fn now_ms() -> u64 {
    must(u64::try_from(
        must(SystemTime::now().duration_since(UNIX_EPOCH)).as_millis(),
    ))
}
fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}
fn hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}
fn b64(bytes: impl AsRef<[u8]>) -> String {
    const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut out = String::new();
    for bytes in bytes.as_ref().chunks(3) {
        let word = (u32::from(bytes[0]) << 16)
            | (u32::from(*bytes.get(1).unwrap_or(&0)) << 8)
            | u32::from(*bytes.get(2).unwrap_or(&0));
        out.push(ALPHABET[((word >> 18) & 63) as usize] as char);
        out.push(ALPHABET[((word >> 12) & 63) as usize] as char);
        out.push(if bytes.len() > 1 {
            ALPHABET[((word >> 6) & 63) as usize] as char
        } else {
            '='
        });
        out.push(if bytes.len() > 2 {
            ALPHABET[(word & 63) as usize] as char
        } else {
            '='
        });
    }
    out
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "inference-deals-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::DirBuilder::new().mode(0o700).create(&path));
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

struct ProgramNode {
    fixture: chain_fixture::Fixture,
    height: u64,
    pending: Vec<Vec<u8>>,
    submissions: usize,
    corrupt_proof: bool,
}
impl ProgramNode {
    fn new(maximum_price: u128) -> Self {
        let mut fixture = chain_fixture::Fixture::program();
        let owner = LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)));
        let mut payload = vec![0; 128];
        let length = must(
            rooms::PostAskRequest {
                room_id: must(Digest32::new([60; 32])),
                ask_id: must(Digest32::new([70; 32])),
                model_digest: must(Digest32::new([20; 32])),
                maximum_price_per_thousand_tokens: maximum_price,
                stream_id: 7,
            }
            .encode(&mut payload),
        );
        fixture.call(dispatch::PostAsk, &payload[..length], &owner, 258);
        for index in 0..3_u8 {
            let peer =
                LocalSigner::from_derived_account(&must(derive_from_seed(&[40 + index; 32], 0)));
            let principal = chain_fixture::principal(
                peer.evm_address()
                    .unwrap_or_else(|| panic!("peer EVM address")),
            );
            let shared = must(state::decode_shared_state(&fixture.state));
            let table = must(workers::WorkerTable::decode(
                must(market::evaluators::authority::split_identity_section(must(
                    shared.section(state::Section::IdentityRoster),
                )))
                .0,
            ));
            let worker = table
                .iter()
                .find(|worker| worker.owner == principal)
                .unwrap_or_else(|| panic!("published peer"));
            let digest = worker.metadata;
            let room = must(Digest32::new([60; 32]));
            let bid = must(Digest32::new([80 + index; 32]));
            let payload = must(
                market::bonds::LockBondRequest {
                    room,
                    bid,
                    amount: 100,
                }
                .encode(),
            );
            fixture.call(dispatch::LockBond, &payload, &peer, 258);
            let request = rooms::PostBidRequest {
                room_id: room,
                bid_id: bid,
                capability_digest: must(Digest32::new(digest.bytes())),
                minimum_price_per_thousand_tokens: 100 + u128::from(index),
                bond_amount: 100,
                layer_capacity: 4,
            };
            let mut bytes = vec![0; 130];
            let length = must(request.encode(&mut bytes));
            fixture.call(dispatch::PostBid, &bytes[..length], &peer, 258);
        }
        Self {
            fixture,
            height: 258,
            pending: Vec::new(),
            submissions: 0,
            corrupt_proof: false,
        }
    }

    fn namespace(&self) -> NamespaceProof {
        namespace_proof(&self.fixture.state, self.fixture.header.program_id.bytes())
    }
    fn commit(&self) -> NodeCommit {
        node_commit(self.height + 1, self.namespace().4, [0x66; 32])
    }
    fn commit_pending(&mut self) {
        let pending = std::mem::take(&mut self.pending);
        assert!(!pending.is_empty(), "the node has no submitted RecordDeal");
        self.height += 1;
        for raw in pending {
            let envelope = must(layerx_crypto::evm_transaction::decode_signed(&raw));
            let (native, _) = must(layerx_crypto::disclosure::call::decode_program_call(
                &envelope.0.data,
            ));
            let validated = must(codec::decode_envelope(&native.calldata));
            let intent = must(validated.request_digest()).bytes();
            let expiry = validated.envelope.expiry;
            let record = must(OperationRecord::from_signed_transaction(
                raw, intent, expiry,
            ));
            self.fixture.apply_signed(&record, self.height);
        }
    }

    fn deal_view(&self, request: &Value) -> Value {
        let text = request["params"][0]
            .as_str()
            .unwrap_or_else(|| panic!("deal id"));
        let id = must(hex::decode(text.strip_prefix("0x").unwrap_or(text)));
        let mut bytes = vec![0; queries::DEAL_VIEW_BYTES];
        let length = must(queries::answer_deal_view(
            &self.fixture.state,
            &id,
            &mut bytes,
        ));
        let view = must(client::decode_deal_view(&bytes[..length]));
        let deal = view.deal;
        let layers = deal
            .layers
            .iter()
            .flatten()
            .map(|split| {
                json!({
                    "bid_id": hex::encode(split.bid_id.as_bytes()),
                    "first_layer": split.first_layer,
                    "end_layer": split.end_layer,
                })
            })
            .collect::<Vec<_>>();
        json!({
            "room_id": hex::encode(deal.room_id.as_bytes()),
            "deal_id": hex::encode(deal.deal_id.as_bytes()),
            "ask_id": hex::encode(deal.ask_id.as_bytes()),
            "agreed_price_per_thousand_tokens": deal.agreed_price_per_thousand_tokens.to_string(),
            "roster_digest": hex::encode(deal.roster_digest.as_bytes()),
            "state": deal.state as u8,
            "layers": layers,
            "settled_tokens": view.settled_tokens.to_string(),
            "settled_amount_owed": view.settled_amount_owed.to_string(),
        })
    }

    fn rpc(&mut self, request: &Value) -> Value {
        let result = match request["method"]
            .as_str()
            .unwrap_or_else(|| panic!("RPC method"))
        {
            "eth_chainId" => json!("0x7d"),
            "eth_blockNumber" => json!(format!("0x{:x}", self.height + 1)),
            "px_getCommit" => {
                assert_eq!(request["params"], json!([(self.height + 1).to_string()]));
                self.commit().material
            }
            "px_getValidators" => self.commit().validators,
            "px_getInferenceDeal" => self.deal_view(request),
            "abci_query" => {
                let params = &request["params"];
                assert_eq!(params["path"], "/store/lxphost/key");
                assert_eq!(params["height"], self.height.to_string());
                let text = params["data"]
                    .as_str()
                    .unwrap_or_else(|| panic!("ABCI key"));
                let key = must(hex::decode(text.strip_prefix("0x").unwrap_or(text)));
                let (head, manifest, mut ops, _, _) = self.namespace();
                let mut manifest_key = vec![5, 0, 9];
                manifest_key.extend_from_slice(&sha256(&manifest));
                let mut state_key = vec![5, 0, 9];
                state_key.extend_from_slice(&sha256(&self.fixture.state));
                let value = if key == ops[0].key {
                    head
                } else if key == manifest_key {
                    manifest
                } else {
                    assert_eq!(key, state_key);
                    self.fixture.state.clone()
                };
                if self.corrupt_proof {
                    ops[0].data[8] ^= 1;
                }
                json!({"response":{"code":0,"height":self.height.to_string(),"key":b64(&key),"value":b64(&value),
                    "proofOps":{"ops": if params["prove"] == true { ops.iter().map(|op| json!({"type":op.op_type,"key":b64(&op.key),"data":b64(&op.data)})).collect::<Vec<_>>() } else { Vec::new() }}}})
            }
            "eth_sendRawTransaction" => {
                let text = request["params"][0]
                    .as_str()
                    .unwrap_or_else(|| panic!("signed transaction"));
                let raw = must(hex::decode(text.strip_prefix("0x").unwrap_or(text)));
                let (call, signature) = must(evm_transaction::decode_signed(&raw));
                let actor = chain_fixture::principal(must(evm_transaction::recover_sender(
                    &call, &signature,
                )));
                assert_eq!(actor, self.fixture.header.owner_principal);
                let (native, _) = must(layerx_crypto::disclosure::call::decode_program_call(
                    &call.data,
                ));
                assert_eq!(
                    must(codec::decode_envelope(&native.calldata))
                        .envelope
                        .operation,
                    dispatch::RecordDeal
                );
                let hash = evm_transaction::transaction_hash(&raw);
                self.pending.push(raw);
                self.submissions += 1;
                json!(hex(&hash))
            }
            method => panic!("unexpected real-node harness method {method}"),
        };
        json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
    }
}

struct NodeServer {
    endpoint: SequencerEndpoint,
    node: Arc<Mutex<ProgramNode>>,
    verifier: ChainProofVerifier,
    stop: Arc<AtomicBool>,
    thread: Option<thread::JoinHandle<()>>,
}
impl NodeServer {
    fn start(maximum_price: u128) -> Self {
        let node = Arc::new(Mutex::new(ProgramNode::new(maximum_price)));
        let verifier = must(node.lock()).commit().verifier;
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        must(listener.set_nonblocking(true));
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let state = Arc::clone(&node);
        let worker = thread::spawn(move || {
            while !stopping.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((mut stream, _)) => {
                        must(stream.set_read_timeout(Some(Duration::from_secs(3))));
                        let mut headers = Vec::new();
                        while !headers.ends_with(b"\r\n\r\n") {
                            assert!(headers.len() < 16_384);
                            let mut byte = [0];
                            must(stream.read_exact(&mut byte));
                            headers.push(byte[0]);
                        }
                        let headers = must(std::str::from_utf8(&headers));
                        let length = headers
                            .split("\r\n")
                            .find_map(|line| {
                                line.split_once(':')
                                    .filter(|(name, _)| name.eq_ignore_ascii_case("content-length"))
                            })
                            .map_or_else(
                                || panic!("HTTP content length"),
                                |(_, value)| must(value.trim().parse::<usize>()),
                            );
                        assert!(length <= 1_048_576);
                        let mut body = vec![0; length];
                        must(stream.read_exact(&mut body));
                        let request: Value = must(serde_json::from_slice(&body));
                        let result = must(state.lock()).rpc(&request).to_string();
                        must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\nContent-Length: {}\r\n\r\n{result}", result.len()));
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("node listener failed: {error:?}"),
                }
            }
        });
        Self {
            endpoint: must(SequencerEndpoint::new_node(
                &format!("http://{address}/"),
                &[],
            )),
            node,
            verifier,
            stop,
            thread: Some(worker),
        }
    }
    fn snapshot(&self) -> FinalizedSnapshot {
        must(client::poll_finalized_market(
            &self.endpoint,
            &self.verifier,
            must(ProgramId::new([51; 32])),
            must(self.endpoint.block_number()),
            now_ms(),
        ))
    }
    fn manifest(index: u8) -> Vec<u8> {
        // The publication envelope in the real program fixture retains the exact emitted manifest.
        let mut fixture = chain_fixture::Fixture::program();
        fixture.published_manifest(index)
    }
}
impl Drop for NodeServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.thread.take() {
            must(worker.join());
        }
    }
}

struct WirePeer {
    key: SigningKey,
    socket: WebSocket<TcpStream>,
    server: TcpStream,
    writer: Arc<Mutex<TcpStream>>,
}
impl WirePeer {
    fn admitted(index: u8, manifest: Vec<u8>, engine: &mut DealEngine) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        let mut nonce = [0; 32];
        must(getrandom::fill(&mut nonce));
        let server = thread::spawn(move || {
            let (stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(3))));
            let mut socket = must(tungstenite::accept(stream));
            must(socket.send(Message::Text(
                must(serde_json::to_string(&ServerFrame::Challenge { nonce })).into(),
            )));
            must(socket.get_ref().try_clone())
        });
        let stream = must(TcpStream::connect(address));
        must(stream.set_read_timeout(Some(Duration::from_secs(3))));
        let (mut socket, _) = must(tungstenite::client(format!("ws://{address}/"), stream));
        let server = must(server.join());
        let Message::Text(challenge) = must(socket.read()) else {
            panic!("challenge frame");
        };
        let ServerFrame::Challenge { nonce } = must(serde_json::from_str(challenge.as_str()))
        else {
            panic!("nonce challenge");
        };
        let key = SigningKey::from_bytes(&[60 + index; 32]);
        let hello = HelloFrame {
            room_id: [60; 32],
            peer_key: key.verifying_key().to_bytes(),
            nonce,
            signature: key
                .sign(&HelloFrame::signing_bytes(&[60; 32], &nonce))
                .to_bytes()
                .to_vec(),
            manifest,
        };
        must(socket.send(Message::Text(
            must(serde_json::to_string(&ClientFrame::Hello(hello))).into(),
        )));
        let writer = Arc::new(Mutex::new(must(server.try_clone())));
        let mut peer = Self {
            key,
            socket,
            server,
            writer,
        };
        let DealClientFrame::Signal(ClientFrame::Hello(hello)) =
            must(receive_deal(&mut peer.server, &peer.writer, &mut None))
        else {
            panic!("hello ingress");
        };
        must(engine.admit(u64::from(index) + 1, &hello, &nonce));
        peer
    }
    fn public_key(&self) -> [u8; 32] {
        self.key.verifying_key().to_bytes()
    }
    fn receive_offer(&mut self, frame: &DealFrame) -> DealFrame {
        must(send_deal(
            &self.writer,
            &DealServerFrame::Deal {
                deal: frame.clone(),
            },
        ));
        let Message::Text(text) = must(self.socket.read()) else {
            panic!("deal frame");
        };
        let DealServerFrame::Deal { deal } = must(serde_json::from_str(text.as_str())) else {
            panic!("deal proposal");
        };
        assert_eq!(&deal, frame);
        deal
    }
    fn acknowledge(
        &mut self,
        frame: &DealFrame,
        engine: &mut DealEngine,
        now: u64,
    ) -> DealAcknowledgement {
        let frame = self.receive_offer(frame);
        let signature = self
            .key
            .sign(&must(frame.acknowledgement_bytes()))
            .to_bytes();
        let acknowledgement = must(frame.acknowledgement(&self.public_key(), signature));
        must(self.socket.send(Message::Text(
            must(serde_json::to_string(&acknowledgement)).into(),
        )));
        let DealClientFrame::Acknowledgement(received) =
            must(receive_deal(&mut self.server, &self.writer, &mut None))
        else {
            panic!("acknowledgement ingress");
        };
        must(engine.acknowledge(&self.public_key(), &received, now));
        acknowledgement
    }

    fn confirm_recorded(
        &mut self,
        frame: &DealFrame,
        snapshot: &FinalizedSnapshot,
        endpoint: &SequencerEndpoint,
    ) {
        must(send_deal(
            &self.writer,
            &DealServerFrame::DealLive {
                deal_id: frame.deal_id,
            },
        ));
        let Message::Text(text) = must(self.socket.read()) else {
            panic!("live deal frame");
        };
        let DealServerFrame::DealLive { deal_id } = must(serde_json::from_str(text.as_str()))
        else {
            panic!("live deal announcement");
        };
        assert_eq!(deal_id, frame.deal_id);
        must(frame.verify_recorded(endpoint, snapshot));
    }
}

fn config() -> DealConfig {
    DealConfig {
        room_id: [60; 32],
        model_digest: [20; 32],
        runtime_digest: [13; 32],
        program: [51; 32],
        layer_count: 8,
        layer_memory_mib: 4096,
    }
}
fn setup(server: &NodeServer, directory: &Directory) -> (DealEngine, Vec<WirePeer>, u64) {
    let mut engine = must(DealEngine::open(
        &directory.0.join("session.json"),
        config(),
    ));
    let now = now_ms();
    must(engine.poll(server.snapshot(), now));
    assert!(
        engine.current().is_none(),
        "unadmitted peers must not be planned"
    );
    let mut peers = Vec::new();
    for index in 0..3 {
        peers.push(WirePeer::admitted(
            index,
            NodeServer::manifest(index),
            &mut engine,
        ));
    }
    must(engine.poll(server.snapshot(), now));
    (engine, peers, now)
}
fn all_acknowledge(engine: &mut DealEngine, peers: &mut [WirePeer], now: u64) -> DealFrame {
    let frame = engine
        .current()
        .cloned()
        .unwrap_or_else(|| panic!("crossing pair has no proposed deal"));
    for split in &frame.plan.layers {
        peers
            .iter_mut()
            .find(|peer| peer.public_key() == split.peer_key)
            .unwrap_or_else(|| panic!("assigned participant"))
            .acknowledge(&frame, engine, now);
    }
    assert_eq!(engine.ready_to_record(), Some(&frame));
    frame
}
pub(crate) fn host_evm_address() -> [u8; 20] {
    LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)))
        .evm_address()
        .unwrap_or_else(|| panic!("host EVM signing address"))
}

fn sign_record(engine: &DealEngine) -> OperationRecord {
    let envelope = must(engine.envelope(350));
    let env = must(codec::decode_envelope(&envelope)).envelope;
    let signer = LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)));
    chain_fixture::ready(sign_market_envelope(
        &envelope,
        &NativeTerms {
            network_id: 125,
            actor_did: b"did:layerx:inference-host",
            owner_public_key: signer.public_key(),
            account_sequence: env.sequence,
            idempotency_key: env.request.bytes(),
            not_before: 0,
            not_after: env.expiry,
            fee_limit: 1_000_000,
            capabilities: &[],
            access_declaration: &[],
            response_capacity: 16_384,
            resources: Resources([10_000_000; 7]),
            gas_limit: 100_000,
            max_fee_per_gas: 10,
            max_priority_fee_per_gas: 1,
        },
        &signer,
    ))
    .unwrap_or_else(|error| panic!("host signing failed: {error:?}"))
}

fn confirm_finalized_deal(
    server: &NodeServer,
    frame: &DealFrame,
    peer: &mut WirePeer,
) -> FinalizedSnapshot {
    must(server.node.lock()).commit_pending();
    let snapshot = server.snapshot();
    let mut bytes = vec![0; queries::DEAL_VIEW_BYTES];
    let length = must(queries::answer_deal_view(
        snapshot.state_bytes(),
        &frame.deal_id,
        &mut bytes,
    ));
    let recorded = must(client::decode_deal_view(&bytes[..length]));
    assert_eq!(recorded.deal.deal_id.bytes(), frame.deal_id);
    assert_eq!(recorded.deal.layers, must(frame.request()).layers);
    peer.confirm_recorded(frame, &snapshot, &server.endpoint);
    let mut altered_price = frame.clone();
    altered_price.price = (must(frame.price.parse::<u128>()) + 1).to_string();
    assert!(altered_price
        .verify_recorded(&server.endpoint, &snapshot)
        .is_err());
    let mut altered_plan = frame.clone();
    altered_plan.plan.layers[0].end_layer -= 1;
    altered_plan.plan.layers[1].first_layer -= 1;
    assert!(altered_plan
        .verify_recorded(&server.endpoint, &snapshot)
        .is_err());
    snapshot
}

fn confirm_reopened_deal(server: &NodeServer, directory: &Directory, frame: &DealFrame, now: u64) {
    let mut reopened = must(DealEngine::open(
        &directory.0.join("session.json"),
        config(),
    ));
    assert!(
        !must(reopened.live(&frame.deal_id)),
        "restart must reacquire chain authority"
    );
    must(reopened.poll(server.snapshot(), now));
    assert!(must(reopened.live(&frame.deal_id)));
}

#[test]
fn crossing_pair_becomes_recorded_only_after_acknowledgements_and_finality() {
    let server = NodeServer::start(1000);
    let directory = Directory::new();
    let (mut engine, mut peers, now) = setup(&server, &directory);
    let frame = engine
        .current()
        .cloned()
        .unwrap_or_else(|| panic!("crossing deal"));
    assert!(engine.ready_to_record().is_none());
    assert!(engine.envelope(250).is_err());
    assert!(!must(engine.live(&frame.deal_id)));
    assert!(frame
        .verify_recorded(&server.endpoint, &server.snapshot())
        .is_err());
    let frame = all_acknowledge(&mut engine, &mut peers, now + 1);
    assert!(
        !must(engine.live(&frame.deal_id)),
        "signatures are not chain finality"
    );
    let journal_path = directory.0.join("operations");
    let journal = must(OperationJournal::open(&journal_path));
    let mut record = sign_record(&engine);
    assert!(record_deal(
        &mut engine,
        &journal,
        &server.endpoint,
        &mut record,
        [0x99; 20],
    )
    .is_err());
    assert_eq!(must(server.node.lock()).submissions, 0);
    assert_eq!(
        must(record_deal(
            &mut engine,
            &journal,
            &server.endpoint,
            &mut record,
            host_evm_address(),
        )),
        OperationState::Pending
    );
    assert_eq!(must(server.node.lock()).submissions, 1);
    must(engine.poll(server.snapshot(), now + 2));
    assert!(
        !must(engine.live(&frame.deal_id)),
        "submitted bytes are not chain finality"
    );
    drop(journal);
    drop(engine);
    let journal = must(OperationJournal::open(&journal_path));
    let registry = must(layerx_types::payload::ModuleRegistry::new(&[]));
    assert_eq!(
        must(journal.load(&record.activity_id(), &registry)).signed_bytes(),
        record.signed_bytes()
    );
    let mut engine = must(DealEngine::open(
        &directory.0.join("session.json"),
        config(),
    ));
    signal::deals::resume_submission(&engine, &journal, &server.endpoint)
        .unwrap_or_else(|error| panic!("resume failed: {error:?}"));
    assert_eq!(
        must(server.node.lock()).submissions,
        1,
        "pending delivery must not be repeated"
    );
    assert!(!must(engine.live(&frame.deal_id)));
    let snapshot = confirm_finalized_deal(&server, &frame, &mut peers[0]);
    must(engine.poll(snapshot, now + 3));
    assert!(must(engine.live(&frame.deal_id)));
    drop(engine);
    confirm_reopened_deal(&server, &directory, &frame, now + 4);
}

#[test]
fn layer_plan_covers_every_layer_once_largest_memory_first_and_retains_host_endpoints() {
    let server = NodeServer::start(1000);
    let directory = Directory::new();
    let (engine, peers, _) = setup(&server, &directory);
    let plan = &engine
        .current()
        .unwrap_or_else(|| panic!("layer plan"))
        .plan;
    must(plan.validate());
    assert_eq!(
        plan.embedding_host,
        must(server.node.lock())
            .fixture
            .header
            .owner_principal
            .bytes()
    );
    assert_eq!(plan.head_host, plan.embedding_host);
    assert_eq!(
        plan.layers[0].peer_key,
        peers[1].public_key(),
        "largest published accelerator memory must receive the first range"
    );
    assert_eq!(plan.layers[0].first_layer, 0);
    assert_eq!(
        plan.layers[0].end_layer, 4,
        "the bid capacity must constrain accelerator memory"
    );
    assert_eq!(plan.layers[1].peer_key, peers[0].public_key());
    let mut coverage = vec![0; usize::from(plan.layer_count)];
    for split in &plan.layers {
        for layer in split.first_layer..split.end_layer {
            coverage[usize::from(layer)] += 1;
        }
    }
    assert_eq!(coverage, vec![1; 8]);
    let mut gap = plan.clone();
    gap.layers[1].first_layer += 1;
    assert!(gap.validate().is_err());
    let mut overlap = plan.clone();
    overlap.layers[1].first_layer -= 1;
    assert!(overlap.validate().is_err());
}

#[test]
fn missing_acknowledgement_drops_the_peer_and_replans_with_fresh_signatures() {
    let server = NodeServer::start(1000);
    let directory = Directory::new();
    let (mut engine, mut peers, now) = setup(&server, &directory);
    let first = engine
        .current()
        .cloned()
        .unwrap_or_else(|| panic!("initial negotiation"));
    let old_ack = peers[0].acknowledge(&first, &mut engine, now + 1);
    assert!(engine.ready_to_record().is_none());
    must(engine.poll(server.snapshot(), now + 4999));
    assert_eq!(
        engine.current(),
        Some(&first),
        "the complete room acknowledgement window must be honoured"
    );
    let dropped = must(engine.poll(server.snapshot(), now + 5000));
    assert_eq!(dropped, vec![peers[1].public_key()]);
    assert!(engine.excluded(&peers[1].public_key()));
    let replacement = engine
        .current()
        .cloned()
        .unwrap_or_else(|| panic!("replacement negotiation"));
    assert_ne!(replacement.deal_id, first.deal_id);
    assert_ne!(must(replacement.digest()), must(first.digest()));
    assert_eq!(replacement.round, first.round + 1);
    assert_eq!(replacement.plan.layers[0].peer_key, peers[0].public_key());
    assert_eq!(replacement.plan.layers[1].peer_key, peers[2].public_key());
    assert!(engine
        .acknowledge(&peers[0].public_key(), &old_ack, now + 5001)
        .is_err());
    assert!(
        engine.ready_to_record().is_none(),
        "an old acknowledgement must not authorize a new plan"
    );
    let replacement = all_acknowledge(&mut engine, &mut peers, now + 5001);
    let journal = must(OperationJournal::open(&directory.0.join("operations")));
    let mut record = sign_record(&engine);
    must(record_deal(
        &mut engine,
        &journal,
        &server.endpoint,
        &mut record,
        host_evm_address(),
    ));
    must(server.node.lock()).commit_pending();
    must(engine.poll(server.snapshot(), now + 5002));
    assert!(must(engine.live(&replacement.deal_id)));
    assert!(!must(engine.live(&first.deal_id)));
}

#[test]
fn roster_digest_matches_the_finalized_chain_value_and_includes_admitted_evaluators() {
    let server = NodeServer::start(1000);
    let directory = Directory::new();
    let (mut engine, mut peers, now) = setup(&server, &directory);
    let frame = all_acknowledge(&mut engine, &mut peers, now + 1);
    let snapshot = server.snapshot();
    let shared = must(state::decode_shared_state(snapshot.state_bytes()));
    let table = must(market::admission::AdmissionTable::decode(
        must(market::reputation_transition::split_joint(must(
            shared.section(state::Section::ReputationAdmission),
        )))
        .1,
    ));
    let epoch = table
        .current_epoch()
        .unwrap_or_else(|| panic!("admission epoch"));
    let mut preimage = epoch.to_be_bytes().to_vec();
    let mut evaluators = 0;
    let mut workers = 0;
    for member in table
        .iter()
        .filter(|member| member.admitted_epoch.is_some())
    {
        match member.participant {
            market::admission::Participant::Worker(_) => workers += 1,
            market::admission::Participant::Evaluator(_) => evaluators += 1,
        }
        preimage.push(member.participant.role() as u8);
        preimage.extend_from_slice(&member.participant.bytes());
        preimage.extend_from_slice(member.owner.as_bytes());
        preimage.extend_from_slice(&member.membership_generation.to_be_bytes());
        preimage.extend_from_slice(&member.delegate_generation.to_be_bytes());
    }
    assert_eq!((workers, evaluators), (3, 3));
    let independently_computed = must(codec::domain_hash("PAXAI/admission-roster/v1", &preimage));
    assert_eq!(frame.roster_digest, independently_computed.bytes());
    let journal = must(OperationJournal::open(&directory.0.join("operations")));
    let mut record = sign_record(&engine);
    must(record_deal(
        &mut engine,
        &journal,
        &server.endpoint,
        &mut record,
        host_evm_address(),
    ));
    must(server.node.lock()).commit_pending();
    let snapshot = server.snapshot();
    let mut bytes = vec![0; queries::DEAL_VIEW_BYTES];
    let length = must(queries::answer_deal_view(
        snapshot.state_bytes(),
        &frame.deal_id,
        &mut bytes,
    ));
    assert_eq!(
        must(queries::DealView::decode(&bytes[..length]))
            .deal
            .roster_digest
            .bytes(),
        independently_computed.bytes()
    );
    must(engine.poll(snapshot, now + 2));
    assert!(must(engine.live(&frame.deal_id)));
}

#[test]
fn noncrossing_orders_and_unverified_state_cannot_create_deals() {
    let server = NodeServer::start(50);
    let directory = Directory::new();
    let (engine, _, _) = setup(&server, &directory);
    assert!(engine.current().is_none());
    assert!(engine.ready_to_record().is_none());
    must(server.node.lock()).corrupt_proof = true;
    assert!(
        client::poll_finalized_market(
            &server.endpoint,
            &server.verifier,
            must(ProgramId::new([51; 32])),
            must(server.endpoint.block_number()),
            now_ms()
        )
        .is_err(),
        "a tampered native membership proof must never produce finality"
    );
    assert_eq!(must(server.node.lock()).submissions, 0);
}

#[test]
fn forged_or_wrong_participant_acknowledgements_cannot_authorize_submission() {
    let server = NodeServer::start(1000);
    let directory = Directory::new();
    let (mut engine, peers, now) = setup(&server, &directory);
    let frame = engine
        .current()
        .cloned()
        .unwrap_or_else(|| panic!("proposal"));
    let signature = peers[0]
        .key
        .sign(&must(frame.acknowledgement_bytes()))
        .to_bytes();
    let valid = must(frame.acknowledgement(&peers[0].public_key(), signature));
    assert!(engine
        .acknowledge(&peers[1].public_key(), &valid, now + 1)
        .is_err());
    assert!(engine
        .acknowledge(&peers[2].public_key(), &valid, now + 1)
        .is_err());
    let mut forged = valid.clone();
    let DealAcknowledgement::DealAck { signature, .. } = &mut forged;
    signature[0] ^= 1;
    assert!(engine
        .acknowledge(&peers[0].public_key(), &forged, now + 1)
        .is_err());
    assert!(engine.ready_to_record().is_none());
    assert!(engine.envelope(250).is_err());
    assert_eq!(must(server.node.lock()).submissions, 0);
}

pub struct NodeCommit {
    pub head: VerifiedChainHead,
    pub verifier: ChainProofVerifier,
    pub material: Value,
    pub validators: Value,
}
/// # Panics
/// Panics if the supplied height or signed fixture material is invalid.
#[must_use]
pub fn node_commit(height: u64, app_hash: [u8; 32], data_hash: [u8; 32]) -> NodeCommit {
    let signer = SigningKey::from_bytes(&[0x74; 32]);
    let validator = TrustedValidator::new(signer.verifying_key().to_bytes(), 100);
    let address = validator.address;
    let validators = must(ValidatorSet::new(vec![validator]));
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-repair-125".to_owned(),
        height: must(i64::try_from(height)),
        time: HeaderTime {
            seconds: 1_700_000_000,
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
        seconds: 1_700_000_000,
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
    let material = json!({"height":height.to_string(),"signedHeader":{"header":{"version":{"block":"11","app":"1"},"chain_id":header.chain_id,"height":height.to_string(),"time":"2023-11-14T22:13:20Z","last_block_id":{"hash":hex(&[1;32]),"parts":{"total":1,"hash":hex(&[2;32])}},"last_commit_hash":hex(&[3;32]),"data_hash":hex(&data_hash),"validators_hash":hex(&verifier.validators.hash()),"next_validators_hash":hex(&verifier.validators.hash()),"consensus_hash":hex(&[6;32]),"app_hash":hex(&app_hash),"last_results_hash":hex(&[8;32]),"evidence_hash":hex(&[9;32]),"proposer_address":hex(&address)},"commit":{"height":height.to_string(),"round":0,"block_id":{"hash":hex(&head.header_hash()),"parts":{"total":1,"hash":hex(&[10;32])}},"signatures":[{"block_id_flag":2,"validator_address":hex(&address),"timestamp":"2023-11-14T22:13:20Z","signature":b64(signature)}]}}});
    assert_eq!(
        must(verify_commit_material(&material["signedHeader"], &verifier)),
        head
    );
    let validators = json!({"height":height.to_string(),"total":"1","validators":[{"address":hex(&address)[2..].to_owned(),"pubKey":{"type":"tendermint/PubKeyEd25519","value":b64(signer.verifying_key().to_bytes())},"votingPower":"100"}]});
    NodeCommit {
        head,
        verifier,
        material,
        validators,
    }
}

fn varint(mut value: usize) -> Vec<u8> {
    let mut out = vec![];
    while value >= 128 {
        out.push((value.to_le_bytes()[0] & 127) | 128);
        value >>= 7;
    }
    out.push(must(u8::try_from(value)));
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
pub type NamespaceProof = (Vec<u8>, Vec<u8>, Vec<ProofOp>, [u8; 32], [u8; 32]);

#[must_use]
pub fn namespace_proof(state: &[u8], program: [u8; 32]) -> NamespaceProof {
    let mut manifest = vec![0, 1];
    manifest.extend_from_slice(&1_u32.to_be_bytes());
    let key = market::SHARED_STATE_KEY;
    manifest.extend_from_slice(&must(u16::try_from(key.len())).to_be_bytes());
    manifest.extend_from_slice(key);
    manifest.extend_from_slice(&sha256(state));
    manifest.extend_from_slice(&must(u32::try_from(state.len())).to_be_bytes());
    let mut head = vec![0, 1];
    head.extend_from_slice(&1_u32.to_be_bytes());
    head.extend_from_slice(&sha256(&manifest));
    let mut key = vec![1, 0, 9];
    key.extend_from_slice(b"progstor");
    key.extend_from_slice(&program);
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

mod chain_fixture {
    use std::sync::Arc;
    use std::task::{Context, Poll, Wake, Waker};
    use std::thread;

    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_agentd::finality::inference_enrol::SignedManifest;
    use layerx_client::ai_market::market_program as market;
    use layerx_client::ai_market::{sign_market_envelope, NativeTerms, OperationRecord};
    use layerx_crypto::account_derivation::derive_from_seed;
    use layerx_crypto::disclosure::call::decode_program_call;
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
    use layerx_crypto::signer::{LocalSigner, Signer};
    use layerx_types::program_call::Resources;
    use market::admission::{ApprovalTerms, EvaluatorConsent, Participant};
    use market::codec::{self, Envelope, Writer};
    use market::dispatch::{self, Operation};
    use market::policy::{PolicyCommitments, RoomProfile, TaskPolicyV1, TASK_POLICY_BYTES};
    use market::registry::{derive_rewards_account, MarketHeader};
    use market::registry_ops::{CallContext, PolicySection};
    use market::state::{self, ActorSlot, Control, ReplayTable, Section, SharedState};
    use market::types::*;
    use market::workers::{self, CapabilityFields, ManifestV2, WorkerTable};

    fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
        value.unwrap_or_else(|error| panic!("{error:?}"))
    }

    pub(super) fn ready<F: std::future::Future>(future: F) -> F::Output {
        struct ThreadWake(thread::Thread);
        impl Wake for ThreadWake {
            fn wake(self: Arc<Self>) {
                self.0.unpark();
            }
            fn wake_by_ref(self: &Arc<Self>) {
                self.0.unpark();
            }
        }
        let waker = Waker::from(Arc::new(ThreadWake(thread::current())));
        let mut context = Context::from_waker(&waker);
        let mut future = std::pin::pin!(future);
        loop {
            match future.as_mut().poll(&mut context) {
                Poll::Ready(value) => return value,
                Poll::Pending => thread::park(),
            }
        }
    }

    pub(super) struct Fixture {
        pub(super) header: MarketHeader,
        pub(super) owner: LocalSigner,
        peer: LocalSigner,
        delegate: SigningKey,
        peer_principal: PrincipalId,
        legacy: Vec<u8>,
        pub(super) manifest: Vec<u8>,
        pub(super) state: Vec<u8>,
        height: u64,
    }

    fn capabilities(runtime_kind: u8) -> CapabilityFields {
        CapabilityFields {
            accelerator_memory_mib: 24_576,
            system_memory_mib: 65_536,
            bandwidth_class: 3,
            region_code: *b"GB",
            attestation_family: 255,
            runtime_kind,
        }
    }

    fn peer_capabilities(index: u8) -> CapabilityFields {
        let mut fields = capabilities(1);
        fields.accelerator_memory_mib = match index {
            0 => 24_576,
            1 => 32_768,
            _ => 16_384,
        };
        fields
    }

    fn legacy_manifest(market: MarketId, worker: WorkerId, owner: PrincipalId) -> Vec<u8> {
        let mut bytes = vec![0; workers::MAX_MANIFEST_BYTES];
        let mut w = Writer::new(&mut bytes);
        must(w.u16(1));
        must(w.put(market.as_bytes()));
        must(w.put(worker.as_bytes()));
        must(w.put(owner.as_bytes()));
        for value in [1, 1, 2, 150, 350] {
            must(w.u64(value));
        }
        must(w.put(&[13; 32]));
        must(w.u16(1));
        must(w.u8(1));
        must(w.u8(1));
        for value in 20..25 {
            must(w.put(&[value; 32]));
        }
        for value in [1024, 2048, 4096, 8192] {
            must(w.u32(value));
        }
        must(w.u8(1));
        must(w.u32(60_000));
        must(w.u16(4));
        must(w.u8(1));
        must(w.u16(1));
        must(w.u8(1));
        must(w.bytes(b"https://worker.example/paxai/v1", workers::MAX_URI_BYTES));
        must(w.put(&[30; 32]));
        must(w.u8(1));
        must(w.u16(1));
        must(w.u16(1));
        must(w.put(&[31; 32]));
        must(w.put(&[32; 32]));
        must(w.u32(0));
        let length = w.len();
        bytes.truncate(length);
        bytes
    }

    pub(super) fn principal(address: [u8; 20]) -> PrincipalId {
        let mut bytes = [0; 32];
        bytes[12..].copy_from_slice(&address);
        must(PrincipalId::new(bytes))
    }

    impl Fixture {
        pub(super) fn program() -> Self {
            let mut fixture = Self::new(1);
            let digest = |byte| must(Digest32::new([byte; 32]));
            let policy = must(TaskPolicyV1::bounded_default(
                1,
                1,
                PolicyCommitments {
                    model_artifact: digest(20),
                    dataset_artifact: [21; 32],
                    benchmark_suite: digest(22),
                    rubric: must(RubricDigest::new([23; 32])),
                    task_schema: digest(24),
                    result_schema: digest(25),
                    service_terms: digest(26),
                },
                1000,
                1,
            ));
            let mut policy_bytes = vec![0; TASK_POLICY_BYTES];
            let length = must(policy.encode(&mut policy_bytes));
            policy_bytes.truncate(length);
            let mut payload = Vec::new();
            payload.extend_from_slice(fixture.header.owner_principal.as_bytes());
            payload.extend_from_slice(fixture.header.funding_asset.as_bytes());
            payload.extend_from_slice(fixture.header.rewards_account.as_bytes());
            payload.extend_from_slice(fixture.header.refund_recipient_account.as_bytes());
            payload.push(0);
            payload.extend_from_slice(&policy_bytes);
            payload.extend_from_slice(fixture.header.metadata_digest.as_bytes());
            let create = fixture.sign(
                dispatch::CREATE,
                &payload,
                fixture.header.owner_principal,
                (1, 0),
                &fixture.owner,
                false,
            );
            fixture.state = must(apply_program(
                &fixture.header,
                None,
                create.signed_bytes(),
                128,
            ));
            fixture.header = must(PolicySection::decode(must(
                must(state::decode_shared_state(&fixture.state)).section(Section::PolicyLifecycle),
            )))
            .header;
            fixture.bootstrap_roster();
            let mut payload = vec![0; 256];
            let length = must(
                market::rooms::OpenRoomRequest {
                    room_id: digest(60),
                    model_digest: digest(20),
                    runtime_digest: digest(13),
                    profile: RoomProfile {
                        minimum_bond: 100,
                        canary_ratio_bps: 250,
                        lap_size_tokens: 32,
                        challenge_window_blocks: 16,
                        slash_fraction_bps: 1000,
                        host_share_bps: 1000,
                        protocol_share_bps: 1000,
                        settlement_account: must(AccountId::new([80; 32])),
                        max_generated_tokens: 4096,
                        queue_depth: market::policy::DEFAULT_QUEUE_DEPTH,
                        liveness_timeout_ms: 30_000,
                        acknowledgement_window_ms: 5_000,
                        canary_relative_tolerance_bps: 100,
                    },
                }
                .encode(&mut payload),
            );
            payload.truncate(length);
            let open = fixture.sign(
                dispatch::OpenRoom,
                &payload,
                fixture.header.owner_principal,
                (fixture.next_sequence(fixture.header.owner_principal), 20),
                &fixture.owner,
                false,
            );
            fixture.state = must(apply_program(
                &fixture.header,
                Some(&fixture.state),
                open.signed_bytes(),
                257,
            ));
            fixture
        }

        pub(super) fn published_manifest(&mut self, index: u8) -> Vec<u8> {
            let peer =
                LocalSigner::from_derived_account(&must(derive_from_seed(&[40 + index; 32], 0)));
            let owner = principal(peer.evm_address().unwrap_or_else(|| panic!("peer owner")));
            let worker = must(codec::derive_worker(self.header.market_id, owner, [21; 32]));
            let legacy = legacy_manifest(self.header.market_id, worker, owner);
            let mut bytes = vec![0; workers::MAX_MANIFEST_BYTES];
            let length = must(codec::encode_manifest_v2(
                &ManifestV2 {
                    manifest_v1: &legacy,
                    capabilities: peer_capabilities(index),
                },
                &mut bytes,
            ));
            bytes.truncate(length);
            let state = must(state::decode_shared_state(&self.state));
            let workers = must(WorkerTable::decode(
                must(market::evaluators::authority::split_identity_section(must(
                    state.section(Section::IdentityRoster),
                )))
                .0,
            ));
            assert_eq!(
                workers
                    .get(worker)
                    .unwrap_or_else(|| panic!("published worker"))
                    .metadata,
                must(workers::decode_manifest(&bytes)).digest
            );
            bytes
        }

        pub(super) fn next_sequence(&self, actor: PrincipalId) -> u64 {
            let state = must(state::decode_shared_state(&self.state));
            (0..43)
                .filter_map(|index| {
                    state
                        .control
                        .replay
                        .actor(must(ActorSlot::from_index(index)))
                })
                .find(|entry| entry.principal == actor)
                .map_or(1, |entry| entry.last.map_or(1, |last| last.sequence + 1))
        }

        pub(super) fn apply_signed(&mut self, record: &OperationRecord, height: u64) {
            self.state = apply_program(
                &self.header,
                Some(&self.state),
                record.signed_bytes(),
                height,
            )
            .unwrap_or_else(|error| {
                panic!(
                    "{} at {height}: {error:?}",
                    must(codec::decode_envelope(&must(
                        layerx_client::ai_market::market_envelope(record)
                    )))
                    .envelope
                    .operation
                    .metadata()
                    .name
                )
            });
            self.header = must(PolicySection::decode(must(
                must(state::decode_shared_state(&self.state)).section(Section::PolicyLifecycle),
            )))
            .header;
            self.height = height;
        }

        pub(super) fn call(
            &mut self,
            operation: Operation,
            payload: &[u8],
            signer: &LocalSigner,
            height: u64,
        ) {
            self.height = height;
            let actor = principal(
                signer
                    .evm_address()
                    .unwrap_or_else(|| panic!("native actor")),
            );
            let sequence = if operation.metadata().sequence == dispatch::SequencePolicy::ObjectLocal
            {
                0
            } else {
                self.next_sequence(actor)
            };
            let record = self.sign(
                operation,
                payload,
                actor,
                (sequence, sequence),
                signer,
                false,
            );
            self.apply_signed(&record, height);
        }

        fn approve(
            &mut self,
            participant: Participant,
            owner: PrincipalId,
            key: PublicKey32,
            nonce: u8,
            identity: Digest32,
            height: u64,
        ) -> Digest32 {
            let effective_epoch = self.admission_epoch(height);
            let mut terms = ApprovalTerms {
                participant,
                owner,
                enrollment_nonce_commitment: must(codec::domain_hash(
                    "PAXAI/enrollment-nonce/v1",
                    &[nonce; 32],
                )),
                delegate: key,
                delegate_generation: 1,
                identity_commitment: identity,
                effective_epoch,
                config_version: 1,
                request: must(RequestId::new([1; 32])),
                expiry_height: 192,
            };
            let mut payload = vec![participant.role() as u8];
            for bytes in [
                participant.bytes().as_slice(),
                owner.as_bytes(),
                terms.enrollment_nonce_commitment.as_bytes(),
                &key.0,
            ] {
                payload.extend_from_slice(bytes);
            }
            payload.extend_from_slice(&1_u64.to_be_bytes());
            payload.extend_from_slice(identity.as_bytes());
            for value in [effective_epoch, 1, terms.expiry_height] {
                payload.extend_from_slice(&value.to_be_bytes());
            }
            let owner_signer =
                LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)));
            let sequence = self.next_sequence(self.header.owner_principal);
            let record = self.sign(
                dispatch::ApproveAdmission,
                &payload,
                self.header.owner_principal,
                (sequence, sequence),
                &owner_signer,
                false,
            );
            let envelope = must(layerx_client::ai_market::market_envelope(&record));
            terms.request = must(codec::decode_envelope(&envelope)).envelope.request;
            let digest = must(terms.digest(&self.header));
            self.apply_signed(&record, height);
            digest
        }

        fn admission_epoch(&self, height: u64) -> u64 {
            let shared = must(state::decode_shared_state(&self.state));
            let table = must(market::admission::AdmissionTable::decode(
                must(market::reputation_transition::split_joint(must(
                    shared.section(Section::ReputationAdmission),
                )))
                .1,
            ));
            must(
                table.required_effective_epoch(&market::admission::AdmissionContext {
                    market: &self.header,
                    invoking_principal: self.header.owner_principal,
                    height,
                }),
            )
        }

        fn bootstrap_peer(&mut self, index: u8) {
            let bootstrap = self;
            bootstrap.peer =
                LocalSigner::from_derived_account(&must(derive_from_seed(&[40 + index; 32], 0)));
            bootstrap.peer_principal = principal(
                bootstrap
                    .peer
                    .evm_address()
                    .unwrap_or_else(|| panic!("bootstrap peer")),
            );
            bootstrap.delegate = SigningKey::from_bytes(&[60 + index; 32]);
            let worker = must(codec::derive_worker(
                bootstrap.header.market_id,
                bootstrap.peer_principal,
                [21; 32],
            ));
            bootstrap.legacy =
                legacy_manifest(bootstrap.header.market_id, worker, bootstrap.peer_principal);
            let mut bytes = vec![0; workers::MAX_MANIFEST_BYTES];
            let length = must(codec::encode_manifest_v2(
                &ManifestV2 {
                    manifest_v1: &bootstrap.legacy,
                    capabilities: peer_capabilities(index),
                },
                &mut bytes,
            ));
            bytes.truncate(length);
            bootstrap.manifest = bytes;
            let plan = bootstrap.plan();
            must(plan.validate());
            let start = if index == 0 {
                140
            } else {
                144 + u64::from(index) * 10
            };
            let publication = if index == 0 { 150 } else { start + 8 };
            for (record, height) in [
                (&plan.proposal, start),
                (&plan.acceptance, start + 1),
                (&plan.publication, publication),
            ] {
                bootstrap.apply_signed(record, height);
            }
            if index == 3 {
                return;
            }
            let state = must(state::decode_shared_state(&bootstrap.state));
            let workers = must(WorkerTable::decode(
                must(market::evaluators::authority::split_identity_section(must(
                    state.section(Section::IdentityRoster),
                )))
                .0,
            ));
            let identity = must(
                workers
                    .get(worker)
                    .unwrap_or_else(|| panic!("bootstrap worker"))
                    .proposal_digest(&bootstrap.header),
            );
            let approval = bootstrap.approve(
                Participant::Worker(worker),
                bootstrap.peer_principal,
                PublicKey32(bootstrap.delegate.verifying_key().to_bytes()),
                21,
                identity,
                publication + 1,
            );
            let mut payload = worker.bytes().to_vec();
            let effective = bootstrap.admission_epoch(publication + 1);
            for value in [1_u64, effective, 1] {
                payload.extend_from_slice(&value.to_be_bytes());
            }
            payload.extend_from_slice(approval.as_bytes());
            let peer =
                LocalSigner::from_derived_account(&must(derive_from_seed(&[40 + index; 32], 0)));
            bootstrap.call(dispatch::AdmitWorker, &payload, &peer, publication + 1);
        }

        fn admit_bootstrap_evaluators(&mut self, owner: &LocalSigner) {
            for nonce in [2_u8, 3, 4] {
                let signer =
                    LocalSigner::from_derived_account(&must(derive_from_seed(&[nonce; 32], 0)));
                let actor = principal(signer.evm_address().unwrap_or_else(|| panic!("evaluator")));
                let key = SigningKey::from_bytes(&[nonce; 32]);
                let public = PublicKey32(key.verifying_key().to_bytes());
                let rubric = must(PolicySection::decode(must(
                    must(state::decode_shared_state(&self.state)).section(Section::PolicyLifecycle),
                )))
                .current
                .commitments
                .rubric;
                let grant = must(market::evaluators::model::EvaluatorGrant::nominate(
                    self.header.market_id,
                    actor,
                    [nonce; 32],
                    market::evaluators::model::GrantTerms {
                        rubric,
                        grant_version: must(Version::new(1)),
                        key_version: must(Version::new(1)),
                        signing_key: public,
                        effective_epoch: 0,
                        expiry_epoch_exclusive: 32,
                    },
                ));
                let mut payload = actor.bytes().to_vec();
                payload.extend_from_slice(&[nonce; 32]);
                payload.extend_from_slice(rubric.as_bytes());
                payload.extend_from_slice(&public.0);
                for value in [1_u64, 1, 0, 32] {
                    payload.extend_from_slice(&value.to_be_bytes());
                }
                self.call(dispatch::ScheduleEvaluator, &payload, owner, 151);
                let mut bytes = [0; 256];
                let length = must(market::evaluators::codec::encode_grant(&grant, &mut bytes));
                let identity = must(codec::domain_hash(
                    "PAXAI/evaluator-grant/v1",
                    &bytes[..length],
                ));
                let approval = self.approve(
                    Participant::Evaluator(grant.evaluator),
                    actor,
                    public,
                    nonce,
                    identity,
                    151,
                );
                let mut request = [1; 32];
                request[..2].copy_from_slice(&dispatch::AdmitEvaluator.selector().to_be_bytes());
                request[2..10].copy_from_slice(&self.next_sequence(actor).to_be_bytes());
                let consent = EvaluatorConsent {
                    chain: self.header.deployment_chain_domain,
                    program: self.header.program_id,
                    market: self.header.market_id,
                    evaluator: grant.evaluator,
                    owner: actor,
                    signing_key: public,
                    enrollment_nonce: [nonce; 32],
                    rubric,
                    approval_digest: approval,
                    request: must(RequestId::new(request)),
                    grant_version: 1,
                    key_version: 1,
                    effective_epoch: 0,
                    config_version: 1,
                    expiry_height: 192,
                };
                let mut body = [0; 362];
                must(consent.encode(&mut body));
                let mut payload = body.to_vec();
                payload.extend_from_slice(&key.sign(must(consent.digest()).as_bytes()).to_bytes());
                self.call(dispatch::AdmitEvaluator, &payload, &signer, 151);
            }
        }

        fn bootstrap_roster(&mut self) {
            let mut bootstrap = Self::new(1);
            bootstrap.state.clone_from(&self.state);
            bootstrap.header = self.header;
            bootstrap.bootstrap_peer(0);
            let owner = LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)));
            bootstrap.admit_bootstrap_evaluators(&owner);
            let mut funding = 1000_u128.to_be_bytes().to_vec();
            funding.extend_from_slice(bootstrap.header.refund_recipient_account.as_bytes());
            funding.extend_from_slice(&1_u64.to_be_bytes());
            funding.push(1);
            bootstrap.call(dispatch::FUND, &funding, &owner, 151);
            bootstrap.call(dispatch::OPEN_EPOCH, &[], &owner, 151);
            for index in 1..4 {
                bootstrap.bootstrap_peer(index);
            }
            bootstrap.call(dispatch::BeginAggregation, &[], &owner, 224);
            loop {
                let progress = must(market::aggregation::progress(&bootstrap.state));
                if progress.cursor == progress.worker_count {
                    break;
                }
                let Presence::Present(input) = progress.input else {
                    panic!("bootstrap aggregation has no sealed input");
                };
                let mut payload = input.bytes().to_vec();
                payload.extend_from_slice(&progress.cursor.to_be_bytes());
                bootstrap.call(dispatch::ProcessAggregation, &payload, &owner, 224);
            }
            let Presence::Present(input) =
                must(market::aggregation::progress(&bootstrap.state)).input
            else {
                panic!("bootstrap aggregation has no sealed input");
            };
            bootstrap.call(dispatch::FinalizeAggregation, input.as_bytes(), &owner, 224);
            assert_eq!(
                must(market::aggregation::progress(&bootstrap.state)).phase,
                market::aggregation_codec::AggregationPhase::Terminal,
            );
            bootstrap.call(dispatch::OPEN_EPOCH, &[], &owner, 256);
            self.state = bootstrap.state;
            self.header = bootstrap.header;
            self.height = bootstrap.height;
        }

        fn new(runtime: u8) -> Self {
            let owner = LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)));
            let peer = LocalSigner::from_derived_account(&must(derive_from_seed(&[12; 32], 0)));
            let owner_principal = principal(
                owner
                    .evm_address()
                    .unwrap_or_else(|| panic!("owner address")),
            );
            let peer_principal =
                principal(peer.evm_address().unwrap_or_else(|| panic!("peer address")));
            let chain = must(ChainDomain::new(market::merged_chain_domain(125)));
            let program = must(ProgramId::new([51; 32]));
            let market = must(codec::derive_market(chain, program));
            let asset = must(AssetId::new([52; 32]));
            let header = MarketHeader {
                format_version: 1,
                market_id: market,
                deployment_chain_domain: chain,
                program_id: program,
                owner_principal,
                funding_asset: asset,
                rewards_account: must(derive_rewards_account(program, asset)),
                refund_recipient_account: must(AccountId::new([54; 32])),
                treasury_principal: Presence::Absent,
                origin_height: 0,
                lifecycle: 2,
                state_revision: 1,
                highest_config_version: 1,
                active_config_version: 1,
                activation_epoch: 0,
                activation_scheduled: false,
                closure_requested_at: 0,
                close_phase: 0,
                close_cursor: 0,
                suspension_reason_digest: [0; 32],
                metadata_digest: must(MetadataDigest::new([55; 32])),
                closing_request_digest: [0; 32],
                reserved: [0; 8],
            };
            let worker = must(codec::derive_worker(market, peer_principal, [21; 32]));
            let legacy = legacy_manifest(market, worker, peer_principal);
            let mut manifest = vec![0; workers::MAX_MANIFEST_BYTES];
            let length = must(codec::encode_manifest_v2(
                &ManifestV2 {
                    manifest_v1: &legacy,
                    capabilities: capabilities(runtime),
                },
                &mut manifest,
            ));
            manifest.truncate(length);
            let mut replay = ReplayTable::new();
            must(replay.bind(ActorSlot::OWNER, owner_principal, must(Version::new(1))));
            let shared = SharedState {
                revision: 1,
                feature_sections: [&[], &[], &[], &[], &[]],
                control: Control {
                    replay,
                    feature_bytes: &[],
                },
            };
            let mut state = vec![0; market::MAX_STATE_BYTES];
            let mut scratch = vec![0; workers::CONTROL_SCRATCH_BYTES];
            let len = must(state::encode_shared_state(
                &shared,
                &mut state,
                &mut scratch,
            ));
            state.truncate(len);
            Self {
                header,
                owner,
                peer,
                delegate: SigningKey::from_bytes(&[56; 32]),
                peer_principal,
                legacy,
                manifest,
                state,
                height: 128,
            }
        }

        pub(super) fn sign(
            &self,
            operation: Operation,
            payload: &[u8],
            actor: PrincipalId,
            (sequence, nonce): (u64, u64),
            signer: &LocalSigner,
            delegate: bool,
        ) -> OperationRecord {
            let mut request = [1; 32];
            request[..2].copy_from_slice(&operation.selector().to_be_bytes());
            request[2..10].copy_from_slice(&sequence.to_be_bytes());
            let state = must(state::decode_shared_state(&self.state));
            let (epoch, roster) = if operation == dispatch::OPEN_EPOCH {
                let frozen = must(market::epoch::preview_open(
                    &self.state,
                    self.height,
                    &mut vec![0; market::MAX_STATE_BYTES],
                    &mut vec![0; market::epoch::OPEN_SCRATCH_BYTES],
                ));
                (frozen.epoch, Presence::Present(frozen.roster))
            } else if matches!(
                operation,
                dispatch::LockBond | dispatch::PostBid | dispatch::RecordDeal
            ) {
                let admission = must(market::admission::AdmissionTable::decode(
                    must(market::reputation_transition::split_joint(must(
                        state.section(Section::ReputationAdmission),
                    )))
                    .1,
                ));
                let epoch = admission
                    .current_epoch()
                    .unwrap_or_else(|| panic!("open admission epoch"));
                let roster = must(market::roster::roster_digest(&admission, epoch));
                (
                    epoch,
                    Presence::Present(must(RosterDigest::new(roster.bytes()))),
                )
            } else if must(state.section(Section::PolicyLifecycle)).is_empty() {
                (0, Presence::Absent)
            } else {
                must(market::epoch::captured_roster(&state)).map_or(
                    (0, Presence::Absent),
                    |roster| {
                        (
                            roster.binding.epoch,
                            Presence::Present(roster.binding.roster),
                        )
                    },
                )
            };
            let mut env = Envelope {
                operation,
                chain: self.header.deployment_chain_domain,
                program: self.header.program_id,
                market: self.header.market_id,
                actor,
                epoch,
                config: 1,
                roster,
                sequence,
                expiry: 1000,
                request: must(RequestId::new(request)),
                payload,
                authentication: Authentication::Native,
            };
            let mut bytes = vec![0; market::MAX_ENVELOPE_BYTES];
            if delegate {
                let key = PublicKey32(self.delegate.verifying_key().to_bytes());
                env.authentication = Authentication::Delegate {
                    key,
                    signature: Signature64([0; 64]),
                };
                let len = must(codec::encode_envelope(&env, &mut bytes));
                let digest = must(must(codec::decode_envelope(&bytes[..len])).request_digest());
                env.authentication = Authentication::Delegate {
                    key,
                    signature: Signature64(self.delegate.sign(digest.as_bytes()).to_bytes()),
                };
            }
            let len = must(codec::encode_envelope(&env, &mut bytes));
            must(ready(sign_market_envelope(
                &bytes[..len],
                &NativeTerms {
                    network_id: 125,
                    actor_did: b"did:layerx:inference-test",
                    owner_public_key: signer.public_key(),
                    account_sequence: nonce,
                    idempotency_key: request,
                    not_before: 0,
                    not_after: 1000,
                    fee_limit: 1_000_000,
                    capabilities: &[],
                    access_declaration: &[],
                    response_capacity: 16_384,
                    resources: Resources([10_000_000; 7]),
                    gas_limit: 100_000,
                    max_fee_per_gas: 10,
                    max_priority_fee_per_gas: 1,
                },
                signer,
            )))
        }

        fn plan(&self) -> SignedManifest {
            let state = must(state::decode_shared_state(&self.state));
            let owner_sequence = state
                .control
                .replay
                .actor(ActorSlot::OWNER)
                .unwrap_or_else(|| panic!("owner replay authority"))
                .last
                .map_or(1, |last| last.sequence + 1);
            let summary = must(workers::decode_manifest(&self.manifest));
            let key = PublicKey32(self.delegate.verifying_key().to_bytes());
            let mut proposal = Vec::new();
            proposal.extend_from_slice(self.peer_principal.as_bytes());
            proposal.extend_from_slice(&[21; 32]);
            proposal.extend_from_slice(&key.0);
            proposal.extend_from_slice(summary.digest.as_bytes());
            proposal.extend_from_slice(&200_u64.to_be_bytes());
            let digest = must(workers::consent_digest(
                &self.header,
                summary.worker,
                summary.owner,
                key,
                1,
                1,
                summary.digest,
                200,
            ));
            let mut acceptance = Vec::new();
            acceptance.extend_from_slice(summary.worker.as_bytes());
            acceptance.extend_from_slice(&1_u64.to_be_bytes());
            acceptance.extend_from_slice(&1_u64.to_be_bytes());
            acceptance.extend_from_slice(summary.digest.as_bytes());
            acceptance.extend_from_slice(&self.delegate.sign(digest.as_bytes()).to_bytes());
            let mut publication = vec![0; workers::MAX_MANIFEST_BYTES + 100];
            let mut w = Writer::new(&mut publication);
            must(w.put(summary.worker.as_bytes()));
            must(w.u64(1));
            must(w.u64(2));
            must(w.put(summary.digest.as_bytes()));
            must(w.u64(summary.valid_from));
            must(w.u64(summary.expiry));
            must(w.bytes(&self.manifest, workers::MAX_MANIFEST_BYTES));
            let len = w.len();
            publication.truncate(len);
            SignedManifest {
                manifest: self.manifest.clone(),
                proposal: self.sign(
                    dispatch::EnrollWorker,
                    &proposal,
                    self.header.owner_principal,
                    (owner_sequence, owner_sequence - 1),
                    &self.owner,
                    false,
                ),
                acceptance: self.sign(
                    dispatch::AcceptEnrollment,
                    &acceptance,
                    self.peer_principal,
                    (1, 0),
                    &self.peer,
                    false,
                ),
                publication: self.sign(
                    dispatch::PublishMetadata,
                    &publication,
                    self.peer_principal,
                    (2, 1),
                    &self.peer,
                    true,
                ),
            }
        }
    }

    fn apply_program(
        header: &MarketHeader,
        current: Option<&[u8]>,
        raw: &[u8],
        height: u64,
    ) -> Result<Vec<u8>, market::errors::ApplicationError> {
        let (call, signature) = must(decode_signed(raw));
        let (native, _) = must(decode_program_call(&call.data));
        let mut next = vec![0; market::MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0; market::MAX_EVENT_BYTES];
        let mut result = vec![0; market::MAX_RESULT_BYTES];
        match dispatch::route(
            &CallContext {
                chain: header.deployment_chain_domain,
                program: header.program_id,
                principal: principal(must(recover_sender(&call, &signature))),
                height,
            },
            &native.calldata,
            current,
            dispatch::Buffers {
                next: &mut next,
                scratch: &mut scratch,
                event: &mut event,
                result: &mut result,
            },
        )? {
            dispatch::Routed::Applied { state_len, .. } => {
                next.truncate(state_len);
                Ok(next)
            }
            dispatch::Routed::Refused { result_len } => {
                Err(codec::decode_result(&result[..result_len])?
                    .error
                    .unwrap_or_else(|| panic!("program refusal must name an error")))
            }
            dispatch::Routed::Unchanged { .. } => panic!("unexpected replay"),
        }
    }
}
