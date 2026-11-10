#[path = "../src/inference_market/signal.rs"]
mod signal;

use std::collections::BTreeMap;
use std::fmt::Debug;
use std::fs;
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::boot::InferenceBootGate;
use layerx_agentd::config::InferenceConfig;
use layerx_agentd::degraded::Reference;
use layerx_client::ai_market::market_program::{queries, workers};
use serde_json::{json, Value};
use signal::{
    ClientFrame, DescriptionKind, HelloFrame, IceCandidate, LayerAssignment, RoomOffer,
    ServerFrame, SessionDescription, SignalAuthority, SignalChain, SignalError, SignalListener,
    MAX_CONNECTIONS, MAX_FRAME_BYTES,
};
use tungstenite::{Message, WebSocket};

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
}

fn agent(ca: Option<&[u8]>) -> ureq::Agent {
    let mut builder = ureq::Agent::config_builder()
        .timeout_global(Some(Duration::from_secs(3)))
        .max_redirects(0)
        .http_status_as_error(false);
    if let Some(ca) = ca {
        let roots = [ureq::tls::Certificate::from_der(ca).to_owned()];
        builder = builder.tls_config(
            ureq::tls::TlsConfig::builder()
                .provider(ureq::tls::TlsProvider::Rustls)
                .root_certs(ureq::tls::RootCerts::new_with_certs(&roots))
                .build(),
        );
    }
    builder.build().into()
}

fn local_offer() -> RoomOffer {
    RoomOffer {
        room_id: [1; 32],
        model_digest: [2; 32],
        runtime_digest: [3; 32],
        layer_plan: vec![LayerAssignment {
            peer_key: SigningKey::from_bytes(&[4; 32]).verifying_key().to_bytes(),
            first_layer: 0,
            end_layer: 32,
        }],
        price: "1000".to_owned(),
    }
}

fn boundary_listener() -> SignalListener {
    let authority = must(SignalAuthority::new(
        agent(None),
        "http://127.0.0.1:9/".to_owned(),
        String::new(),
        125,
        [5; 32],
        Arc::new(Mutex::new(InferenceBootGate::default())),
    ));
    must(SignalListener::start(
        must(TcpListener::bind("127.0.0.1:0")),
        local_offer(),
        authority,
    ))
}

fn connect(listener: &SignalListener) -> (WebSocket<TcpStream>, [u8; 32]) {
    let socket = must(TcpStream::connect(listener.local_addr()));
    must(socket.set_read_timeout(Some(Duration::from_secs(3))));
    must(socket.set_write_timeout(Some(Duration::from_secs(3))));
    let (mut socket, response) = must(tungstenite::client(
        format!("ws://{}/", listener.local_addr()),
        socket,
    ));
    assert_eq!(response.status(), 101);
    let ServerFrame::Challenge { nonce } = receive(&mut socket) else {
        panic!("expected a fresh server challenge");
    };
    (socket, nonce)
}

fn receive(socket: &mut WebSocket<TcpStream>) -> ServerFrame {
    let Message::Text(text) = must(socket.read()) else {
        panic!("expected a JSON signalling frame");
    };
    must(serde_json::from_str(text.as_str()))
}

fn send(socket: &mut WebSocket<TcpStream>, frame: &ClientFrame) {
    must(socket.send(Message::Text(must(serde_json::to_string(frame)).into())));
}

fn hello(room_id: [u8; 32], nonce: [u8; 32], key: &SigningKey, manifest: Vec<u8>) -> HelloFrame {
    HelloFrame {
        room_id,
        peer_key: key.verifying_key().to_bytes(),
        nonce,
        signature: key
            .sign(&HelloFrame::signing_bytes(&room_id, &nonce))
            .to_bytes()
            .to_vec(),
        manifest,
    }
}

fn refused(socket: &mut WebSocket<TcpStream>, expected: SignalError) {
    assert_eq!(receive(socket), ServerFrame::Error { code: expected });
    let Message::Close(Some(_)) = must(socket.read()) else {
        panic!("refused peer retained its connection");
    };
}

#[test]
fn bad_signature_refused_before_any_chain_read() {
    let listener = boundary_listener();
    let (mut socket, nonce) = connect(&listener);
    let mut frame = hello(
        local_offer().room_id,
        nonce,
        &SigningKey::from_bytes(&[4; 32]),
        vec![0, 2],
    );
    frame.signature[0] ^= 1;
    send(&mut socket, &ClientFrame::Hello(frame));
    refused(&mut socket, SignalError::BadSignature);
    must(listener.check());
}

#[test]
fn cap_refuses_the_65th_connection_before_hello() {
    let listener = boundary_listener();
    let mut peers = Vec::new();
    for _ in 0..MAX_CONNECTIONS {
        peers.push(connect(&listener).0);
    }
    assert_eq!(peers.len(), 64);
    let socket = must(TcpStream::connect(listener.local_addr()));
    must(socket.set_read_timeout(Some(Duration::from_secs(3))));
    let refusal = tungstenite::client(format!("ws://{}/", listener.local_addr()), socket);
    match refusal {
        Err(tungstenite::HandshakeError::Failure(tungstenite::Error::Http(response))) => {
            assert_eq!(response.status(), 503);
        }
        other => panic!("65th connection was not refused: {other:?}"),
    }
    must(listener.check());
}

#[test]
fn frames_are_capped_at_exactly_65536_bytes() {
    let listener = boundary_listener();
    for length in [MAX_FRAME_BYTES, MAX_FRAME_BYTES + 1] {
        let (mut socket, nonce) = connect(&listener);
        let mut frame = hello(
            local_offer().room_id,
            nonce,
            &SigningKey::from_bytes(&[4; 32]),
            vec![0, 2],
        );
        frame.signature[0] ^= 1;
        let mut text = must(serde_json::to_string(&ClientFrame::Hello(frame)));
        text.extend(std::iter::repeat_n(' ', length - text.len()));
        assert_eq!(text.len(), length);
        must(socket.send(Message::Text(text.into())));
        refused(
            &mut socket,
            if length == MAX_FRAME_BYTES {
                SignalError::BadSignature
            } else {
                SignalError::FrameTooLarge
            },
        );
    }
}

#[test]
fn captured_hello_cannot_be_replayed_on_a_new_connection() {
    let listener = boundary_listener();
    let (first, nonce) = connect(&listener);
    let (mut second, second_nonce) = connect(&listener);
    assert_ne!(nonce, second_nonce);
    let frame = hello(
        local_offer().room_id,
        nonce,
        &SigningKey::from_bytes(&[4; 32]),
        vec![0, 2],
    );
    send(&mut second, &ClientFrame::Hello(frame));
    refused(&mut second, SignalError::WrongChallenge);
    drop(first);
}

#[test]
fn room_offer_hash_binds_every_advertised_term() {
    let offer = local_offer();
    let digest = must(offer.digest());
    let mut mutations = Vec::new();
    let mut changed = offer.clone();
    changed.room_id[0] ^= 1;
    mutations.push(changed);
    let mut changed = offer.clone();
    changed.model_digest[0] ^= 1;
    mutations.push(changed);
    let mut changed = offer.clone();
    changed.runtime_digest[0] ^= 1;
    mutations.push(changed);
    let mut changed = offer.clone();
    changed.layer_plan[0].end_layer += 1;
    mutations.push(changed);
    let mut changed = offer.clone();
    changed.layer_plan[0].peer_key[0] ^= 1;
    mutations.push(changed);
    let mut changed = offer.clone();
    changed.price = "1001".to_owned();
    mutations.push(changed);
    for changed in mutations {
        assert_ne!(must(changed.digest()), digest);
    }
}

struct LiveContext {
    authority: SignalAuthority,
    offer: RoomOffer,
    manifests: Vec<Vec<u8>>,
    seeds: Vec<[u8; 32]>,
    chain: Arc<TestChain>,
}

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "inference-signal-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::create_dir(&path));
        Self(path)
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[derive(Clone)]
struct TestChain {
    state: Vec<u8>,
    room: Value,
    reference: Reference,
}

impl SignalChain for TestChain {
    fn reference(&self) -> Result<Reference, SignalError> {
        Ok(self.reference)
    }

    fn read(&self, method: &str, params: Value) -> Result<Value, SignalError> {
        match method {
            "eth_chainId" => {
                assert_eq!(params, json!([]));
                Ok(json!("0x7d"))
            }
            "eth_getBlockByNumber" => {
                assert_eq!(params, json!(["latest", false]));
                Ok(
                    json!({"number": format!("0x{:x}", self.reference.head_sequence),
                    "hash": format!("0x{}", hex::encode(self.reference.checkpoint))}),
                )
            }
            "px_getInferenceRoom" => {
                assert_eq!(params, json!([self.room["room_id"]]));
                let key = must(hex::decode(lint_present(params[0].as_str(), "room id")));
                let mut out = vec![0; queries::ROOM_VIEW_BYTES];
                let length = must(queries::answer_room_view(&self.state, &key, &mut out));
                let view = must(queries::RoomView::decode(&out[..length]));
                assert_eq!(
                    hex::encode(view.room.room_id.as_bytes()),
                    self.room["room_id"]
                );
                assert_eq!(
                    hex::encode(view.room.model_digest.as_bytes()),
                    self.room["model_digest"]
                );
                assert_eq!(
                    hex::encode(view.room.runtime_digest.as_bytes()),
                    self.room["runtime_digest"]
                );
                assert_eq!(
                    view.room.state as u8,
                    lint_success(
                        u8::try_from(lint_present(self.room["state"].as_u64(), "room state")),
                        "integer fits u8"
                    )
                );
                assert_eq!(room_answer(&view), self.room);
                Ok(self.room.clone())
            }
            "px_getInferencePeer" => {
                let key = must(hex::decode(lint_present(
                    params[0].as_str(),
                    "peer principal",
                )));
                assert_eq!(key.len(), 32);
                let mut out = vec![0; queries::PEER_VIEW_BYTES];
                let length = match queries::answer_peer_view(&self.state, &key, &mut out) {
                    Ok(length) => length,
                    Err(error) if error == layerx_client::ai_market::market_program::errors::F11_ROOM_UNKNOWN => return Ok(Value::Null),
                    Err(error) => panic!("real F11 peer read refused: {error:?}"),
                };
                let peer = must(queries::PeerView::decode(&out[..length]));
                Ok(json!({"peer_key": hex::encode(peer.peer_key.as_bytes()),
                    "delegate": hex::encode(peer.delegate.0), "enrolled": peer.enrolled,
                    "metadata": hex::encode(peer.metadata.as_bytes()),
                    "worker": hex::encode(peer.worker.as_bytes()),
                    "generation": peer.generation.to_string(), "key_version": peer.key_version.to_string(),
                    "metadata_revision": peer.metadata_revision.to_string(),
                    "valid_from": peer.valid_from.to_string(), "expiry": peer.expiry.to_string()}))
            }
            _ => panic!("unexpected signal chain read {method}"),
        }
    }
}

fn room_answer(view: &queries::RoomView) -> Value {
    assert!(view.open_asks.iter().all(Option::is_none));
    let room = &view.room;
    let profile = &room.profile;
    json!({
        "room_id": hex::encode(room.room_id.as_bytes()),
        "host_key": hex::encode(room.host_key.as_bytes()),
        "model_digest": hex::encode(room.model_digest.as_bytes()),
        "runtime_digest": hex::encode(room.runtime_digest.as_bytes()),
        "state": room.state as u8,
        "opened_at_epoch": room.opened_at_epoch.to_string(),
        "profile": {
            "minimum_bond": profile.minimum_bond.to_string(),
            "canary_ratio_bps": profile.canary_ratio_bps,
            "lap_size_tokens": profile.lap_size_tokens.to_string(),
            "challenge_window_blocks": profile.challenge_window_blocks.to_string(),
            "slash_fraction_bps": profile.slash_fraction_bps,
            "host_share_bps": profile.host_share_bps,
            "protocol_share_bps": profile.protocol_share_bps,
            "settlement_account": hex::encode(profile.settlement_account.as_bytes()),
        },
        "open_asks": view.open_asks.iter().flatten()
            .map(|id| hex::encode(id.as_bytes())).collect::<Vec<_>>(),
        "open_bids": view.open_bids.iter().flatten()
            .map(|id| hex::encode(id.as_bytes())).collect::<Vec<_>>(),
        "ask_streams": [],
    })
}

impl LiveContext {
    fn load() -> Self {
        Self::load_room(false)
    }

    fn load_room(closed: bool) -> Self {
        let (state, manifests, seeds) = enrolled_chain::peers(closed);
        let mut offer = local_offer();
        offer.room_id = [60; 32];
        offer.model_digest = [20; 32];
        offer.runtime_digest = [13; 32];
        offer.layer_plan = seeds
            .iter()
            .take(3)
            .enumerate()
            .map(|(index, seed)| LayerAssignment {
                peer_key: SigningKey::from_bytes(seed).verifying_key().to_bytes(),
                first_layer: (lint_success(u32::try_from(index), "integer fits u32")) * 16,
                end_layer: (lint_success(u32::try_from(index), "integer fits u32") + 1) * 16,
            })
            .collect();
        let directory = Directory::new();
        let profile = directory.0.join("room.json");
        must(fs::write(&profile, must(serde_json::to_vec(&offer))));
        let values = BTreeMap::from([
            (
                "LAYERX_AGENT_INFERENCE_MODEL_DIGEST".to_owned(),
                hex::encode(offer.model_digest),
            ),
            (
                "LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST".to_owned(),
                hex::encode(offer.runtime_digest),
            ),
            (
                "LAYERX_AGENT_INFERENCE_ROOM_PROFILE".to_owned(),
                profile.to_string_lossy().into_owned(),
            ),
            (
                "LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN".to_owned(),
                "127.0.0.1:0".to_owned(),
            ),
            (
                "LAYERX_AGENT_INFERENCE_SERVE_LISTEN".to_owned(),
                "127.0.0.1:0".to_owned(),
            ),
        ]);
        let config = must(InferenceConfig::from_values(&values));
        let offer: RoomOffer = must(serde_json::from_slice(&must(config.load_room_profile())));
        assert_eq!(offer.model_digest, config.model_digest);
        assert_eq!(offer.runtime_digest, config.runtime_digest);
        let reference = Reference {
            head_sequence: 183,
            checkpoint: [7; 32],
        };
        let mut out = vec![0; queries::ROOM_VIEW_BYTES];
        let length = must(queries::answer_room_view(&state, &offer.room_id, &mut out));
        let view = must(queries::RoomView::decode(&out[..length]));
        let room = room_answer(&view);
        let chain = Arc::new(TestChain {
            state,
            room,
            reference,
        });
        let authority = SignalAuthority::with_chain(chain.clone(), 125, [51; 32]);
        Self {
            authority,
            offer,
            manifests,
            seeds,
            chain,
        }
    }

    fn listener(&self, offer: RoomOffer) -> SignalListener {
        must(SignalListener::start(
            must(TcpListener::bind("127.0.0.1:0")),
            offer,
            self.authority.clone(),
        ))
    }

    fn admitted(&self, listener: &SignalListener, index: usize) -> WebSocket<TcpStream> {
        let (mut socket, nonce) = connect(listener);
        send(
            &mut socket,
            &ClientFrame::Hello(hello(
                self.offer.room_id,
                nonce,
                &SigningKey::from_bytes(&self.seeds[index]),
                self.manifests[index].clone(),
            )),
        );
        assert_eq!(
            receive(&mut socket),
            ServerFrame::Offer {
                offer: self.offer.clone()
            }
        );
        socket
    }
}

#[test]
fn hello_admitted_with_the_chain_anchored_offer() {
    let context = LiveContext::load();
    let listener = context.listener(context.offer.clone());
    let _peer = context.admitted(&listener, 0);
    must(listener.check());
}

#[test]
fn hello_is_admitted_against_the_room_fields_the_node_serves() {
    let context = LiveContext::load();
    let fields = context
        .chain
        .room
        .as_object()
        .unwrap_or_else(|| panic!("room answer"));
    assert_eq!(
        fields.keys().map(String::as_str).collect::<Vec<_>>(),
        vec![
            "ask_streams",
            "host_key",
            "model_digest",
            "open_asks",
            "open_bids",
            "opened_at_epoch",
            "profile",
            "room_id",
            "runtime_digest",
            "state",
        ]
    );
    let listener = context.listener(context.offer.clone());
    let _peer = context.admitted(&listener, 0);
    must(listener.check());

    for field in ["model_digest", "runtime_digest", "state"] {
        let mut changed = LiveContext::load_room(field == "state");
        match field {
            "model_digest" => changed.offer.model_digest[0] ^= 1,
            "runtime_digest" => changed.offer.runtime_digest[0] ^= 1,
            "state" => assert_eq!(changed.chain.room["state"], 1),
            _ => unreachable!(),
        }
        let listener = changed.listener(changed.offer.clone());
        let (mut socket, nonce) = connect(&listener);
        send(
            &mut socket,
            &ClientFrame::Hello(hello(
                changed.offer.room_id,
                nonce,
                &SigningKey::from_bytes(&changed.seeds[0]),
                changed.manifests[0].clone(),
            )),
        );
        refused(&mut socket, SignalError::OfferMismatch);
        must(listener.check());
    }
}

#[test]
fn unenrolled_peer_refused() {
    let context = LiveContext::load();
    let listener = context.listener(context.offer.clone());
    let (mut socket, nonce) = connect(&listener);
    let mut seed = [0; 32];
    must(getrandom::fill(&mut seed));
    send(
        &mut socket,
        &ClientFrame::Hello(hello(
            context.offer.room_id,
            nonce,
            &SigningKey::from_bytes(&seed),
            context.manifests[0].clone(),
        )),
    );
    refused(&mut socket, SignalError::Unenrolled);
}

#[test]
fn f11_unadmitted_peer_refused() {
    let context = LiveContext::load();
    let listener = context.listener(context.offer.clone());
    let (mut socket, nonce) = connect(&listener);
    send(
        &mut socket,
        &ClientFrame::Hello(hello(
            context.offer.room_id,
            nonce,
            &SigningKey::from_bytes(&context.seeds[3]),
            context.manifests[3].clone(),
        )),
    );
    refused(&mut socket, SignalError::Unenrolled);
}

#[test]
fn unknown_peer_principal_refused() {
    let context = LiveContext::load();
    let listener = context.listener(context.offer.clone());
    let (mut socket, nonce) = connect(&listener);
    let mut manifest = context.manifests[0].clone();
    manifest[66..98].copy_from_slice(&[88; 32]);
    must(workers::decode_manifest(&manifest));
    send(
        &mut socket,
        &ClientFrame::Hello(hello(
            context.offer.room_id,
            nonce,
            &SigningKey::from_bytes(&context.seeds[0]),
            manifest,
        )),
    );
    refused(&mut socket, SignalError::Unenrolled);
}

#[test]
fn room_without_open_state_refused() {
    let context = LiveContext::load_room(true);
    let authority = context.authority.clone();
    let listener = must(SignalListener::start(
        must(TcpListener::bind("127.0.0.1:0")),
        context.offer.clone(),
        authority,
    ));
    let (mut socket, nonce) = connect(&listener);
    send(
        &mut socket,
        &ClientFrame::Hello(hello(
            context.offer.room_id,
            nonce,
            &SigningKey::from_bytes(&context.seeds[0]),
            context.manifests[0].clone(),
        )),
    );
    refused(&mut socket, SignalError::OfferMismatch);
}

#[test]
fn malformed_v2_manifest_refused() {
    let context = LiveContext::load();
    let listener = context.listener(context.offer.clone());
    let (mut socket, nonce) = connect(&listener);
    send(
        &mut socket,
        &ClientFrame::Hello(hello(
            context.offer.room_id,
            nonce,
            &SigningKey::from_bytes(&context.seeds[0]),
            vec![0, 2],
        )),
    );
    refused(&mut socket, SignalError::InvalidManifest);
}

#[test]
fn enrolled_manifest_digest_mismatch_refused() {
    let context = LiveContext::load();
    let listener = context.listener(context.offer.clone());
    let (mut socket, nonce) = connect(&listener);
    let mut manifest = context.manifests[0].clone();
    let last = manifest.len() - 1;
    manifest[last] ^= 1;
    send(
        &mut socket,
        &ClientFrame::Hello(hello(
            context.offer.room_id,
            nonce,
            &SigningKey::from_bytes(&context.seeds[0]),
            manifest,
        )),
    );
    refused(&mut socket, SignalError::ManifestMismatch);
}

#[test]
fn changed_offer_refused_against_the_chain_record() {
    let context = LiveContext::load();
    let mut offer = context.offer.clone();
    offer.model_digest[0] ^= 1;
    let listener = context.listener(offer);
    let (mut socket, nonce) = connect(&listener);
    send(
        &mut socket,
        &ClientFrame::Hello(hello(
            context.offer.room_id,
            nonce,
            &SigningKey::from_bytes(&context.seeds[0]),
            context.manifests[0].clone(),
        )),
    );
    refused(&mut socket, SignalError::OfferMismatch);
}

#[test]
fn relay_description_and_ice_deliver_to_the_named_peer_only() {
    let context = LiveContext::load();
    let listener = context.listener(context.offer.clone());
    let mut sender = context.admitted(&listener, 0);
    let mut recipient = context.admitted(&listener, 1);
    let mut bystander = context.admitted(&listener, 2);
    let from = SigningKey::from_bytes(&context.seeds[0])
        .verifying_key()
        .to_bytes();
    let to = SigningKey::from_bytes(&context.seeds[1])
        .verifying_key()
        .to_bytes();
    let description = SessionDescription {
        kind: DescriptionKind::Offer,
        sdp: "v=0\r\ns=inference\r\nt=0 0\r\n".to_owned(),
    };
    send(
        &mut sender,
        &ClientFrame::Description {
            to,
            description: description.clone(),
        },
    );
    assert_eq!(
        receive(&mut recipient),
        ServerFrame::Description { from, description }
    );
    let candidate = IceCandidate {
        candidate: "candidate:1 1 UDP 2122260223 127.0.0.1 12345 typ host".to_owned(),
        sdp_mid: Some("0".to_owned()),
        sdp_m_line_index: Some(0),
    };
    send(
        &mut sender,
        &ClientFrame::Candidate {
            to,
            candidate: candidate.clone(),
        },
    );
    assert_eq!(
        receive(&mut recipient),
        ServerFrame::Candidate { from, candidate }
    );
    for socket in [&mut sender, &mut bystander] {
        must(
            socket
                .get_mut()
                .set_read_timeout(Some(Duration::from_millis(100))),
        );
        match socket.read() {
            Err(tungstenite::Error::Io(error))
                if matches!(
                    error.kind(),
                    std::io::ErrorKind::WouldBlock | std::io::ErrorKind::TimedOut
                ) => {}
            other => panic!("relay reached an unnamed peer: {other:?}"),
        }
    }
}

mod enrolled_chain {
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

    fn ready<F: std::future::Future>(future: F) -> F::Output {
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

    struct Fixture {
        header: MarketHeader,
        owner: LocalSigner,
        peer: LocalSigner,
        delegate: SigningKey,
        peer_principal: PrincipalId,
        legacy: Vec<u8>,
        manifest: Vec<u8>,
        state: Vec<u8>,
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

    fn principal(address: [u8; 20]) -> PrincipalId {
        let mut bytes = [0; 32];
        bytes[12..].copy_from_slice(&address);
        must(PrincipalId::new(bytes))
    }

    impl Fixture {
        fn program() -> Self {
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
                183,
            ));
            fixture
        }

        fn next_sequence(&self, actor: PrincipalId) -> u64 {
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

        fn apply_signed(&mut self, record: &OperationRecord, height: u64) {
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
        }

        fn call(
            &mut self,
            operation: Operation,
            payload: &[u8],
            signer: &LocalSigner,
            height: u64,
        ) {
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
                    capabilities: capabilities(1),
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

        fn bootstrap_roster(&mut self) {
            let mut bootstrap = Self::new(1);
            bootstrap.state.clone_from(&self.state);
            bootstrap.header = self.header;
            bootstrap.bootstrap_peer(0);
            let owner = LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)));
            for nonce in [2_u8, 3, 4] {
                let signer =
                    LocalSigner::from_derived_account(&must(derive_from_seed(&[nonce; 32], 0)));
                let actor = principal(signer.evm_address().unwrap_or_else(|| panic!("evaluator")));
                let key = SigningKey::from_bytes(&[nonce; 32]);
                let public = PublicKey32(key.verifying_key().to_bytes());
                let rubric = must(PolicySection::decode(must(
                    must(state::decode_shared_state(&bootstrap.state))
                        .section(Section::PolicyLifecycle),
                )))
                .current
                .commitments
                .rubric;
                let grant = must(market::evaluators::model::EvaluatorGrant::nominate(
                    bootstrap.header.market_id,
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
                bootstrap.call(dispatch::ScheduleEvaluator, &payload, &owner, 151);
                let mut bytes = [0; 256];
                let length = must(market::evaluators::codec::encode_grant(&grant, &mut bytes));
                let identity = must(codec::domain_hash(
                    "PAXAI/evaluator-grant/v1",
                    &bytes[..length],
                ));
                let approval = bootstrap.approve(
                    Participant::Evaluator(grant.evaluator),
                    actor,
                    public,
                    nonce,
                    identity,
                    151,
                );
                let mut request = [1; 32];
                request[..2].copy_from_slice(&dispatch::AdmitEvaluator.selector().to_be_bytes());
                request[2..10].copy_from_slice(&bootstrap.next_sequence(actor).to_be_bytes());
                let consent = EvaluatorConsent {
                    chain: bootstrap.header.deployment_chain_domain,
                    program: bootstrap.header.program_id,
                    market: bootstrap.header.market_id,
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
                bootstrap.call(dispatch::AdmitEvaluator, &payload, &signer, 151);
            }
            let mut funding = 1000_u128.to_be_bytes().to_vec();
            funding.extend_from_slice(bootstrap.header.refund_recipient_account.as_bytes());
            funding.extend_from_slice(&1_u64.to_be_bytes());
            funding.push(1);
            bootstrap.call(dispatch::FUND, &funding, &owner, 151);
            bootstrap.call(dispatch::OPEN_EPOCH, &[], &owner, 152);
            for index in 1..4 {
                bootstrap.bootstrap_peer(index);
            }
            self.state = bootstrap.state;
            self.header = bootstrap.header;
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
            }
        }

        fn sign(
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
            let (epoch, roster) = if matches!(operation, dispatch::LockBond | dispatch::PostBid) {
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
                let roster = must(market::epoch::captured_roster(&state))
                    .map_or(Presence::Absent, |roster| {
                        Presence::Present(roster.binding.roster)
                    });
                (0, roster)
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

    pub fn peers(closed: bool) -> (Vec<u8>, Vec<Vec<u8>>, Vec<[u8; 32]>) {
        let mut fixture = Fixture::program();
        if closed {
            let close = fixture.sign(
                dispatch::CloseRoom,
                &[60; 32],
                fixture.header.owner_principal,
                (fixture.next_sequence(fixture.header.owner_principal), 21),
                &fixture.owner,
                false,
            );
            fixture.state = must(apply_program(
                &fixture.header,
                Some(&fixture.state),
                close.signed_bytes(),
                183,
            ));
        }
        let mut manifests = Vec::new();
        let mut seeds = Vec::new();
        for index in 0..4 {
            let peer =
                LocalSigner::from_derived_account(&must(derive_from_seed(&[40 + index; 32], 0)));
            let owner = principal(super::lint_present(peer.evm_address(), "peer owner"));
            let worker = must(codec::derive_worker(
                fixture.header.market_id,
                owner,
                [21; 32],
            ));
            let legacy = legacy_manifest(fixture.header.market_id, worker, owner);
            let mut bytes = vec![0; workers::MAX_MANIFEST_BYTES];
            let length = must(codec::encode_manifest_v2(
                &ManifestV2 {
                    manifest_v1: &legacy,
                    capabilities: capabilities(1),
                },
                &mut bytes,
            ));
            bytes.truncate(length);
            let mut answer = vec![0; market::queries::PEER_VIEW_BYTES];
            let length = must(market::queries::answer_peer_view(
                &fixture.state,
                owner.as_bytes(),
                &mut answer,
            ));
            let view = must(market::queries::PeerView::decode(&answer[..length]));
            assert_eq!(view.enrolled, index < 3);
            assert_eq!(view.peer_key, owner);
            assert_eq!(view.metadata, must(workers::decode_manifest(&bytes)).digest);
            assert_eq!(
                view.delegate.0,
                SigningKey::from_bytes(&[60 + index; 32])
                    .verifying_key()
                    .to_bytes()
            );
            manifests.push(bytes);
            seeds.push([60 + index; 32]);
        }
        (fixture.state, manifests, seeds)
    }
}

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}

#[track_caller]
fn lint_present<T>(value: Option<T>, context: &str) -> T {
    match value {
        Some(value) => value,
        None => panic!("{context}: required value is absent"),
    }
}
