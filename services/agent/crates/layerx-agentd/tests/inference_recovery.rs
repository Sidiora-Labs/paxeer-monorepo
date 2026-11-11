#![forbid(unsafe_code)]

#[allow(
    dead_code,
    reason = "Including targets do not use every signal item; decision 3218 permits dead_code on the including module"
)]
mod signal {
    include!("../src/inference_market/signal.rs");
    pub mod deals {
        include!("../src/inference_market/deals.rs");
        pub mod laps {
            include!("../src/inference_market/laps.rs");
            pub mod recovery {
                include!("../src/inference_market/recovery.rs");
            }
        }
        pub mod serve {
            include!("../src/inference_market/serve.rs");
            pub mod recovery {
                include!("../src/inference_market/recovery_serve.rs");
            }
        }
    }
}

#[allow(
    dead_code,
    reason = "Including targets do not use every producer item; decision 3218 permits dead_code on the including module"
)]
mod producers {
    include!("inference_deals.rs");

    pub(super) struct Market(NodeServer);
    impl Market {
        pub(super) fn new() -> Self {
            Self(NodeServer::start(1000))
        }
        pub(super) fn snapshot(&self) -> FinalizedSnapshot {
            self.0.snapshot()
        }
        pub(super) fn manifest(&self, index: u8) -> Vec<u8> {
            must(self.0.node.lock()).fixture.published_manifest(index)
        }
        pub(super) fn endpoint(&self) -> &SequencerEndpoint {
            &self.0.endpoint
        }
        pub(super) fn commit(&self) {
            must(self.0.node.lock()).commit_pending();
        }
        pub(super) fn assert_recorded_recovery(
            &self,
            previous: [u8; 32],
            replacement: [u8; 32],
            ask: [u8; 32],
        ) {
            let snapshot = self.snapshot();
            let shared = must(state::decode_shared_state(snapshot.state_bytes()));
            let book = must(rooms::RoomBook::decode(must(shared.room_market()).1));
            let previous = must(book.deal(must(Digest32::new(previous))))
                .unwrap_or_else(|| panic!("previous deal remains recorded"));
            let replacement = must(book.deal(must(Digest32::new(replacement))))
                .unwrap_or_else(|| panic!("replacement deal is recorded"));
            assert_ne!(previous.deal_id, replacement.deal_id);
            assert_eq!(previous.ask_id.bytes(), ask);
            assert_eq!(replacement.ask_id.bytes(), ask);
            assert_ne!(previous.layers, replacement.layers);
            assert_eq!(
                must(book.ask(must(Digest32::new(ask))))
                    .unwrap_or_else(|| panic!("matched ask remains recorded"))
                    .state,
                rooms::AskState::Matched
            );
        }
        pub(super) fn height(&self) -> u64 {
            must(self.0.node.lock()).height
        }
    }

    pub(super) fn signed(envelope: &[u8]) -> OperationRecord {
        let env = must(codec::decode_envelope(envelope)).envelope;
        let signer = LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)));
        must(chain_fixture::ready(sign_market_envelope(
            envelope,
            &NativeTerms {
                network_id: 125,
                actor_did: b"did:layerx:recovery-host",
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
        )))
    }
}

use std::collections::BTreeMap;
use std::fmt::Debug;
use std::fs;
use std::io::{Read, Write};
use std::net::{Shutdown, TcpListener, TcpStream};
use std::os::unix::fs::DirBuilderExt as _;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::ai_market::{inference_market::HostMeterKey, OperationJournal, OperationState};
use signal::deals::laps::recovery::{LossReason, PeerLoss, RecoveryEngine, RecoveryState};
use signal::deals::laps::{
    receive_countersignature, send_lap, LapAcknowledgement, LapEngine, LapFrame, LapServerFrame,
};
use signal::deals::{
    receive_deal, record_deal, send_deal, DealClientFrame, DealConfig, DealEngine, DealFrame,
    DealServerFrame,
};
use signal::{ClientFrame, HelloFrame};
use tungstenite::{Message, WebSocket};

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "inference-recovery-{}-{}",
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

struct Peer {
    index: u8,
    socket: WebSocket<TcpStream>,
    server: TcpStream,
    writer: Arc<Mutex<TcpStream>>,
}
impl Peer {
    fn join(index: u8, market: &producers::Market, engine: &mut DealEngine) -> Self {
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        let accept = thread::spawn(move || {
            let (stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            must(must(tungstenite::accept(stream)).get_ref().try_clone())
        });
        let client = must(TcpStream::connect(address));
        let (mut socket, _) = must(tungstenite::client(format!("ws://{address}/"), client));
        must(
            socket
                .get_mut()
                .set_read_timeout(Some(Duration::from_secs(5))),
        );
        let key = SigningKey::from_bytes(&[60 + index; 32]);
        let nonce = [90 + index; 32];
        let hello = HelloFrame {
            room_id: [60; 32],
            peer_key: key.verifying_key().to_bytes(),
            nonce,
            signature: key
                .sign(&HelloFrame::signing_bytes(&[60; 32], &nonce))
                .to_bytes()
                .to_vec(),
            manifest: market.manifest(index),
        };
        must(socket.send(Message::Text(
            must(serde_json::to_string(&ClientFrame::Hello(hello))).into(),
        )));
        let mut server = must(accept.join());
        let writer = Arc::new(Mutex::new(must(server.try_clone())));
        let DealClientFrame::Signal(ClientFrame::Hello(hello)) =
            must(receive_deal(&mut server, &writer, &mut None))
        else {
            panic!("hello ingress");
        };
        must(engine.admit(u64::from(index) + 1, &hello, &nonce));
        Self {
            index,
            socket,
            server,
            writer,
        }
    }
    fn key(&self) -> HostMeterKey {
        HostMeterKey::from_seed(&[60 + self.index; 32])
    }
    fn public(&self) -> [u8; 32] {
        self.key().public_key()
    }
    fn acknowledge(&mut self, frame: &DealFrame, engine: &mut DealEngine, now: u64) {
        must(send_deal(
            &self.writer,
            &DealServerFrame::Deal {
                deal: frame.clone(),
            },
        ));
        let received: DealServerFrame = must(serde_json::from_slice(
            &must(self.socket.read()).into_data(),
        ));
        let DealServerFrame::Deal { deal } = received else {
            panic!("deal frame");
        };
        assert_eq!(deal, *frame);
        let key = SigningKey::from_bytes(&[60 + self.index; 32]);
        let ack = must(deal.acknowledgement(
            &self.public(),
            key.sign(&must(deal.acknowledgement_bytes())).to_bytes(),
        ));
        must(
            self.socket
                .send(Message::Text(must(serde_json::to_string(&ack)).into())),
        );
        let DealClientFrame::Acknowledgement(ack) =
            must(receive_deal(&mut self.server, &self.writer, &mut None))
        else {
            panic!("deal acknowledgement ingress");
        };
        must(engine.acknowledge(&self.public(), &ack, now));
    }
    fn countersign(&mut self, frame: &LapFrame, engine: &mut LapEngine, now: u64) {
        must(send_lap(&self.writer, frame));
        let received: LapServerFrame = must(serde_json::from_slice(
            &must(self.socket.read()).into_data(),
        ));
        let LapServerFrame::LapReceipt { lap } = received else {
            panic!("expected a lap receipt");
        };
        let ack = must(lap.countersign(&self.key()));
        must(
            self.socket.send(Message::Text(
                must(serde_json::to_string(&LapAcknowledgement::LapCountersign(
                    ack,
                )))
                .into(),
            )),
        );
        let ack = must(receive_countersignature(&mut self.server, &self.writer));
        must(engine.countersign(self.public(), &ack, now));
    }
}

struct Room {
    market: producers::Market,
    directory: Directory,
    deals: DealEngine,
    peers: Vec<Peer>,
    frame: DealFrame,
    laps: BTreeMap<[u8; 32], LapEngine>,
    recovery: RecoveryEngine,
    host: HostMeterKey,
    journal: OperationJournal,
}
impl Room {
    fn open(indices: &[u8]) -> Self {
        let market = producers::Market::new();
        let directory = Directory::new();
        let mut deals = must(DealEngine::open(&directory.0.join("deals.json"), config()));
        must(deals.poll(market.snapshot(), 1000));
        let mut peers = indices
            .iter()
            .map(|index| Peer::join(*index, &market, &mut deals))
            .collect::<Vec<_>>();
        must(deals.poll(market.snapshot(), 1001));
        let frame = deals
            .current()
            .cloned()
            .unwrap_or_else(|| panic!("initial complete plan"));
        for split in &frame.plan.layers {
            peers
                .iter_mut()
                .find(|p| p.public() == split.peer_key)
                .unwrap_or_else(|| panic!("planned peer"))
                .acknowledge(&frame, &mut deals, 1002);
        }
        let journal = must(OperationJournal::open(&directory.0.join("operations")));
        let mut record = producers::signed(&must(deals.envelope(350)));
        assert_eq!(
            must(record_deal(
                &mut deals,
                &journal,
                market.endpoint(),
                &mut record
            )),
            OperationState::Pending
        );
        market.commit();
        must(deals.poll(market.snapshot(), 1003));
        assert!(must(deals.live(&frame.deal_id)));
        let host = HostMeterKey::from_seed(&[7; 32]);
        let lap_engine = must(LapEngine::open(
            &directory.0.join(format!(
                "lap-{}.json",
                layerx_programs::hex::encode(&frame.deal_id)
            )),
            market.snapshot().state_bytes(),
            frame.clone(),
            host.public_key(),
            8,
        ));
        let recovery = must(RecoveryEngine::open(
            &directory.0.join("recovery.json"),
            config(),
        ));
        let laps = BTreeMap::from([(frame.deal_id, lap_engine)]);
        Self {
            market,
            directory,
            deals,
            peers,
            frame,
            laps,
            recovery,
            host,
            journal,
        }
    }
    fn countersigned_lap(&mut self, now: u64, prompt: u64) -> LapFrame {
        let engine = self
            .laps
            .get_mut(&self.frame.deal_id)
            .unwrap_or_else(|| panic!("live lap engine"));
        let frame = must(engine.issue_lap(
            prompt,
            32,
            vec![[8; 32]; self.frame.plan.layers.len()],
            false,
            now,
            &self.host,
        ));
        for split in &self.frame.plan.layers {
            self.peers
                .iter_mut()
                .find(|p| p.public() == split.peer_key)
                .unwrap_or_else(|| panic!("lap peer"))
                .countersign(&frame, engine, now + 1);
        }
        assert!(must(engine.finish_lap(now + 2, &self.host)));
        frame
    }
    fn lose(&mut self, index: u8) {
        let peer = self
            .peers
            .iter()
            .find(|p| p.index == index)
            .unwrap_or_else(|| panic!("lost peer"));
        must(peer.server.shutdown(Shutdown::Both));
        must(self.recovery.replan_after_loss(
            &mut self.deals,
            &mut self.laps,
            PeerLoss {
                deal_id: self.frame.deal_id,
                peer_key: peer.public(),
            },
            LossReason::DataChannel,
            &self.market.snapshot(),
            2000,
        ));
    }
}

#[test]
fn loss_replans_all_layers_using_the_waiting_peer_and_retains_deal_lineage() {
    let mut room = Room::open(&[0, 1, 2]);
    let last = room.countersigned_lap(1100, 5);
    room.lose(1);
    let record = &room.recovery.records()[0];
    let replacement = record
        .replacement
        .as_ref()
        .unwrap_or_else(|| panic!("replacement plan"));
    must(replacement.plan.validate());
    assert_eq!(record.previous_deal.deal_id, room.frame.deal_id);
    assert_ne!(replacement.deal_id, room.frame.deal_id);
    assert_eq!(replacement.ask_id, room.frame.ask_id);
    assert_eq!(
        replacement
            .plan
            .layers
            .first()
            .unwrap_or_else(|| panic!("first range"))
            .first_layer,
        0
    );
    assert_eq!(
        replacement
            .plan
            .layers
            .last()
            .unwrap_or_else(|| panic!("last range"))
            .end_layer,
        8
    );
    assert!(replacement
        .plan
        .layers
        .iter()
        .any(|p| p.peer_key == HostMeterKey::from_seed(&[62; 32]).public_key()));
    assert!(replacement
        .plan
        .layers
        .iter()
        .all(|p| p.peer_key != HostMeterKey::from_seed(&[61; 32]).public_key()));
    assert_eq!(record.resume.next_lap_index, last.receipt.lap_index + 1);
    assert_eq!(
        record.resume.cumulative_tokens,
        last.receipt.cumulative_tokens
    );
    assert!(!record.recompute.is_empty());
    for split in &replacement.plan.layers {
        if !room.frame.plan.layers.contains(split) {
            assert!(record.recompute.iter().any(|r| r.peer_key == split.peer_key
                && r.first_layer == split.first_layer
                && r.end_layer == split.end_layer
                && r.through_lap == Some(last.receipt.lap_index)));
        }
    }
    assert!(
        room.deals.ready_to_record().is_none(),
        "replacement requires new signatures"
    );
    assert!(
        !must(room.deals.live(&room.frame.deal_id)),
        "superseded deal cannot admit completions"
    );
    assert!(
        room.recovery.paused().is_some(),
        "replacement must reach finality before serving"
    );
    let restored = must(RecoveryEngine::open(
        &room.directory.0.join("recovery.json"),
        config(),
    ));
    assert_eq!(
        restored.records()[0]
            .replacement
            .as_ref()
            .unwrap_or_else(|| panic!("retained replacement")),
        replacement
    );
    assert_eq!(restored.records()[0].resume, record.resume);
}

#[test]
fn resume_discards_the_uncountersigned_tail_and_keeps_the_last_signed_lap() {
    let mut room = Room::open(&[0, 1, 2]);
    room.countersigned_lap(1100, 5);
    let last = room.countersigned_lap(1200, 0);
    let engine = room
        .laps
        .get_mut(&room.frame.deal_id)
        .unwrap_or_else(|| panic!("live engine"));
    let pending = must(engine.issue_lap(
        0,
        32,
        vec![[9; 32]; room.frame.plan.layers.len()],
        false,
        1300,
        &room.host,
    ));
    let survivor = room
        .peers
        .iter_mut()
        .find(|p| p.index == 0)
        .unwrap_or_else(|| panic!("survivor"));
    survivor.countersign(&pending, engine, 1301);
    room.lose(1);
    let checkpoint = &room.recovery.records()[0].resume;
    assert_eq!(
        checkpoint.last_countersigned_lap,
        Some(last.receipt.lap_index)
    );
    assert_eq!(checkpoint.next_lap_index, last.receipt.lap_index + 1);
    assert_eq!(checkpoint.next_lap_index, pending.receipt.lap_index);
    assert_eq!(checkpoint.cumulative_tokens, last.receipt.cumulative_tokens);
    assert_eq!(checkpoint.generated_tokens, 64);
}

#[test]
fn an_uncoverable_range_pauses_both_completion_routes_until_a_peer_joins() {
    let mut room = Room::open(&[0, 1]);
    room.countersigned_lap(1100, 5);
    room.lose(1);
    assert_eq!(room.recovery.records()[0].state, RecoveryState::Paused);
    assert!(room.recovery.records()[0].replacement.is_none());
    for route in ["/v1/chat/completions", "/v1/messages"] {
        assert!(room.recovery.paused().is_some());
        let recovery = must(RecoveryEngine::open(
            &room.directory.0.join("recovery.json"),
            config(),
        ));
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        let server = thread::spawn(move || {
            let (mut stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            let mut completion = must(signal::deals::serve::CompletionConnection::read(
                &mut stream,
            ));
            assert!(must(recovery.refuse_paused_completion(&mut completion)));
        });
        let mut client = must(TcpStream::connect(address));
        must(client.set_read_timeout(Some(Duration::from_secs(5))));
        must(write!(
            client,
            "POST {route} HTTP/1.1\r\nContent-Length: 0\r\n\r\n"
        ));
        let mut response = String::new();
        must(client.read_to_string(&mut response));
        must(server.join());
        assert!(response.starts_with("HTTP/1.1 503 Service Unavailable\r\n"));
        assert!(response.contains("\r\nRetry-After: 1\r\n"));
        assert!(response.contains("room_paused"));
    }
    let peer = Peer::join(2, &room.market, &mut room.deals);
    room.peers.push(peer);
    must(room.recovery.refresh(
        &mut room.deals,
        &mut room.laps,
        &room.market.snapshot(),
        3000,
    ));
    assert_eq!(room.recovery.records()[0].state, RecoveryState::Negotiating);
    must(
        room.recovery.records()[0]
            .replacement
            .as_ref()
            .unwrap_or_else(|| panic!("joining peer fills the gap"))
            .plan
            .validate(),
    );
    assert!(
        room.recovery.paused().is_some(),
        "joining does not bypass finality"
    );
}

#[test]
fn the_recorded_replacement_resumes_generation_at_the_last_countersigned_lap_plus_one() {
    let mut room = Room::open(&[0, 1, 2]);
    let last = room.countersigned_lap(1100, 5);
    room.lose(1);
    let replacement = room.recovery.records()[0]
        .replacement
        .clone()
        .unwrap_or_else(|| panic!("replacement"));
    for split in &replacement.plan.layers {
        room.peers
            .iter_mut()
            .find(|p| p.public() == split.peer_key)
            .unwrap_or_else(|| panic!("replacement peer"))
            .acknowledge(&replacement, &mut room.deals, 2001);
    }
    let mut signed = producers::signed(&must(room.deals.envelope(350)));
    assert_eq!(
        must(record_deal(
            &mut room.deals,
            &room.journal,
            room.market.endpoint(),
            &mut signed
        )),
        OperationState::Pending
    );
    room.market.commit();
    room.market.assert_recorded_recovery(
        room.frame.deal_id,
        replacement.deal_id,
        room.frame.ask_id,
    );
    must(room.deals.poll(room.market.snapshot(), 2002));
    assert!(
        must(room.deals.live(&replacement.deal_id)),
        "replacement must be recorded through the real program"
    );
    let mut engine = must(LapEngine::open(
        &room.directory.0.join("replacement.json"),
        room.market.snapshot().state_bytes(),
        replacement.clone(),
        room.host.public_key(),
        8,
    ));
    must(room.recovery.records()[0].resume_lap_engine(&room.deals, &mut engine));
    let resumed = must(engine.issue_lap(
        0,
        32,
        vec![[10; 32]; replacement.plan.layers.len()],
        false,
        2003,
        &room.host,
    ));
    assert_eq!(resumed.receipt.lap_index, last.receipt.lap_index + 1);
    assert_eq!(
        resumed.receipt.cumulative_tokens,
        last.receipt.cumulative_tokens + 32
    );
    assert!(room.market.height() > 258);
}

#[test]
fn liveness_uses_the_room_deadline_and_resumes_from_the_last_fully_signed_lap() {
    use layerx_client::ai_market::market_program::{rooms::RoomBook, state, types::Digest32};
    let mut room = Room::open(&[0, 1, 2]);
    let last = room.countersigned_lap(1100, 5);
    let snapshot = room.market.snapshot();
    let shared = must(state::decode_shared_state(snapshot.state_bytes()));
    let book = must(RoomBook::decode(must(shared.room_market()).1));
    let timeout = must(book.room(must(Digest32::new(room.frame.room_id))))
        .unwrap_or_else(|| panic!("room profile"))
        .profile
        .liveness_timeout_ms;
    let engine = room
        .laps
        .get_mut(&room.frame.deal_id)
        .unwrap_or_else(|| panic!("live engine"));
    let pending = must(engine.issue_lap(
        0,
        32,
        vec![[9; 32]; room.frame.plan.layers.len()],
        false,
        1300,
        &room.host,
    ));
    room.peers
        .iter_mut()
        .find(|p| p.index == 0)
        .unwrap_or_else(|| panic!("responsive peer"))
        .countersign(&pending, engine, 1301);
    assert!(must(engine.finish_lap(pending.deadline_ms, &room.host)));
    must(room.recovery.refresh(
        &mut room.deals,
        &mut room.laps,
        &snapshot,
        1300 + timeout - 1,
    ));
    assert!(
        room.recovery.records().is_empty(),
        "the room profile's liveness interval has not elapsed"
    );
    must(
        room.recovery
            .refresh(&mut room.deals, &mut room.laps, &snapshot, 1300 + timeout),
    );
    let record = &room.recovery.records()[0];
    assert_eq!(
        record.lost_peers.get(&layerx_programs::hex::encode(
            &HostMeterKey::from_seed(&[61; 32]).public_key()
        )),
        Some(&LossReason::LivenessTimeout)
    );
    assert_eq!(record.resume.next_lap_index, last.receipt.lap_index + 1);
    assert_eq!(
        record.resume.cumulative_tokens,
        last.receipt.cumulative_tokens
    );
    must(
        record
            .replacement
            .as_ref()
            .unwrap_or_else(|| panic!("liveness replacement"))
            .plan
            .validate(),
    );
}

#[test]
fn restart_retains_the_checkpoint_and_requires_replacement_finality() {
    let mut room = Room::open(&[0, 1, 2]);
    let last = room.countersigned_lap(1100, 5);
    room.lose(1);
    room.recovery = must(RecoveryEngine::open(
        &room.directory.0.join("recovery.json"),
        config(),
    ));
    room.deals = must(DealEngine::open(
        &room.directory.0.join("deals.json"),
        config(),
    ));
    must(room.deals.poll(room.market.snapshot(), 2100));
    for index in [0, 2] {
        room.peers
            .push(Peer::join(index, &room.market, &mut room.deals));
    }
    let old = room
        .laps
        .remove(&room.frame.deal_id)
        .unwrap_or_else(|| panic!("original lap engine"));
    drop(old);
    let original = must(LapEngine::open(
        &room.directory.0.join(format!(
            "lap-{}.json",
            layerx_programs::hex::encode(&room.frame.deal_id)
        )),
        room.market.snapshot().state_bytes(),
        room.frame.clone(),
        room.host.public_key(),
        8,
    ));
    room.laps.insert(room.frame.deal_id, original);
    must(room.recovery.refresh(
        &mut room.deals,
        &mut room.laps,
        &room.market.snapshot(),
        2200,
    ));
    assert_eq!(
        room.recovery.records()[0].resume.next_lap_index,
        last.receipt.lap_index + 1
    );
    assert!(room.recovery.paused().is_some());
    let replacement = room.recovery.records()[0]
        .replacement
        .as_ref()
        .unwrap_or_else(|| panic!("restored replacement"));
    assert!(!must(room.deals.live(&replacement.deal_id)));
    assert!(
        LapEngine::open(
            &room.directory.0.join("unrecorded.json"),
            room.market.snapshot().state_bytes(),
            replacement.clone(),
            room.host.public_key(),
            8
        )
        .is_err(),
        "a journal entry must not bypass chain finality"
    );
}

#[test]
fn resume_moves_the_cursor_of_the_route_to_the_further_deal() {
    let mut room = Room::open(&[0, 1, 2]);
    let last = room.countersigned_lap(1100, 5);
    room.lose(1);
    let record = &room.recovery.records()[0];
    assert_eq!(record.resume.next_lap_index, last.receipt.lap_index + 1);
    assert_eq!(
        record.resume.cumulative_tokens,
        last.receipt.cumulative_tokens
    );
    signal::deals::serve::recovery::tests::assert_resume_cursor(record);
}
