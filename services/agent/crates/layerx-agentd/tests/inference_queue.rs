pub mod signal {
    include!("../src/inference_market/signal.rs");
    pub mod deals {
        include!("../src/inference_market/deals.rs");
        pub mod serve {
            include!("../src/inference_market/serve.rs");
        }
        pub mod queue {
            include!("../src/inference_market/queue.rs");
        }
    }
}

use std::fmt::Debug;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use layerx_agentd::boot::InferenceBootGate;
use layerx_agentd::degraded::Reference;
use layerx_client::ai_market::market_program::{
    codec::{self, Writer},
    policy::{RoomProfile, DEFAULT_QUEUE_DEPTH},
    rooms::{DealRecord, DealState, RoomRecord, RoomState},
    types::{AccountId, Digest32, PrincipalId},
    workers::{self, CapabilityFields, ManifestV2, MAX_MANIFEST_BYTES},
};
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use signal::deals::queue::{
    DraftPeer, GenerationSlot, QueueError, RequestQueue, MAX_SPECULATIVE_SLOTS,
};
use signal::deals::serve::{CompletionBinding, CompletionConnection, ServeRoutes};
use signal::deals::{plan_layers, DealFrame, PlanPeer};

fn must<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("inference queue proof: {error:?}"))
}
fn present<T>(value: Option<T>, context: &str) -> T {
    value.unwrap_or_else(|| panic!("inference queue proof: {context}"))
}
fn digest(value: u8) -> Digest32 {
    must(Digest32::new([value; 32]))
}

fn manifest(runtime_kind: u8) -> Vec<u8> {
    let mut bytes = vec![0; MAX_MANIFEST_BYTES];
    let mut writer = Writer::new(&mut bytes);
    must(writer.u16(1));
    for value in [10, 11, 12] {
        must(writer.put(&[value; 32]));
    }
    for value in [2, 3, 5, 300, 428] {
        must(writer.u64(value));
    }
    must(writer.put(&[13; 32]));
    must(writer.u16(1));
    must(writer.u8(1));
    must(writer.u8(1));
    for value in 20..25 {
        must(writer.put(&[value; 32]));
    }
    for value in [1024, 2048, 4096, 8192] {
        must(writer.u32(value));
    }
    must(writer.u8(1));
    must(writer.u32(60_000));
    must(writer.u16(4));
    must(writer.u8(1));
    must(writer.u16(1));
    must(writer.u8(1));
    must(writer.bytes(b"https://worker.example/paxai/v1", workers::MAX_URI_BYTES));
    must(writer.put(&[30; 32]));
    must(writer.u8(1));
    must(writer.u16(1));
    must(writer.u16(1));
    must(writer.put(&[31; 32]));
    must(writer.put(&[32; 32]));
    must(writer.u32(0));
    let length = writer.len();
    bytes.truncate(length);
    let mut out = vec![0; MAX_MANIFEST_BYTES];
    let length = must(codec::encode_manifest_v2(
        &ManifestV2 {
            manifest_v1: &bytes,
            capabilities: CapabilityFields {
                accelerator_memory_mib: 24_576,
                system_memory_mib: 65_536,
                bandwidth_class: 4,
                region_code: *b"US",
                attestation_family: 255,
                runtime_kind,
            },
        },
        &mut out,
    ));
    out.truncate(length);
    must(workers::decode_manifest(&out));
    out
}

fn binding(ask: u8, depth: u16, runtime_kind: u8) -> (CompletionBinding, DealFrame, Vec<u8>) {
    let manifest = manifest(runtime_kind);
    let summary = must(workers::decode_manifest(&manifest));
    let profile = RoomProfile {
        minimum_bond: 1,
        canary_ratio_bps: 0,
        lap_size_tokens: 2,
        challenge_window_blocks: 10,
        slash_fraction_bps: 100,
        host_share_bps: 1000,
        protocol_share_bps: 100,
        settlement_account: must(AccountId::new([4; 32])),
        max_generated_tokens: 1024,
        queue_depth: depth,
        liveness_timeout_ms: 1000,
        acknowledgement_window_ms: 1000,
        canary_relative_tolerance_bps: 10,
    };
    let room = RoomRecord {
        room_id: digest(1),
        host_key: must(PrincipalId::new([5; 32])),
        model_digest: digest(2),
        runtime_digest: digest(3),
        profile,
        state: RoomState::Open,
        opened_at_epoch: 1,
        closed_height: 0,
    };
    let plan = must(plan_layers(
        [5; 32],
        2,
        1,
        &[PlanPeer {
            peer_key: [14; 32],
            owner: summary.owner.bytes(),
            bid_id: [6; 32],
            manifest_digest: summary.digest.bytes(),
            accelerator_memory_mib: 24_576,
            layer_capacity: 2,
            minimum_price: 1000,
        }],
    ));
    let frame = DealFrame {
        chain: [15; 32],
        program: [16; 32],
        market: summary.market.bytes(),
        room_id: [1; 32],
        model_digest: [2; 32],
        runtime_digest: [3; 32],
        deal_id: [ask.wrapping_add(64); 32],
        ask_id: [ask; 32],
        roster_digest: [10; 32],
        epoch: 1,
        config: 1,
        round: 1,
        price: "1000".to_owned(),
        plan,
    };
    let request = must(frame.request());
    let deal = DealRecord {
        room_id: request.room_id,
        deal_id: request.deal_id,
        ask_id: request.ask_id,
        agreed_price_per_thousand_tokens: request.agreed_price_per_thousand_tokens,
        roster_digest: request.roster_digest,
        layers: request.layers,
        state: DealState::Open,
    };
    let mut bytes = vec![0; 4096];
    let length = must(room.encode(&mut bytes));
    let room = must(RoomRecord::decode(&bytes[..length]));
    let length = must(deal.encode(&mut bytes));
    let deal = must(DealRecord::decode(&bytes[..length]));
    (must(CompletionBinding::new(room, deal)), frame, manifest)
}

fn draft(ask: u8) -> DraftPeer {
    let (binding, frame, manifest) = binding(ask, 32, 1);
    present(
        must(DraftPeer::from_deal_manifest(
            &binding,
            &frame,
            &frame.plan.layers[0],
            &manifest,
            1,
        )),
        "explicit draft runtime-kind advertisement in the assigned peer manifest",
    )
}

#[test]
fn ordering_is_fair_across_two_asks() {
    let queue = RequestQueue::default();
    let active = must(must(queue.enqueue([1; 32], [7; 32], None, None)).wait(Duration::ZERO));
    let a1 = must(queue.enqueue([1; 32], [7; 32], None, None));
    let a2 = must(queue.enqueue([1; 32], [7; 32], None, None));
    let a3 = must(queue.enqueue([1; 32], [7; 32], None, None));
    let b1 = must(queue.enqueue([1; 32], [8; 32], None, None));
    let b2 = must(queue.enqueue([1; 32], [8; 32], None, None));
    let b3 = must(queue.enqueue([1; 32], [8; 32], None, None));
    assert_eq!(must(a1.slot()), None);
    assert_eq!(must(b1.slot()), None);
    drop(active);
    let mut pending = [a1, a2, a3, b1, b2, b3]
        .into_iter()
        .map(Some)
        .collect::<Vec<_>>();
    for expected in [3, 0, 4, 1, 5, 2] {
        for (index, ticket) in pending.iter().enumerate() {
            if let Some(ticket) = ticket {
                assert_eq!(
                    must(ticket.slot()),
                    if index == expected {
                        Some(GenerationSlot::Generation)
                    } else {
                        None
                    }
                );
            }
        }
        let permit = must(present(pending[expected].take(), "queued request").wait(Duration::ZERO));
        assert_eq!(permit.room(), [1; 32]);
        drop(permit);
    }
}

#[test]
fn default_depth_is_32_and_rooms_are_independent() {
    assert_eq!(DEFAULT_QUEUE_DEPTH, 32);
    let queue = RequestQueue::default();
    let active = must(queue.enqueue([1; 32], [7; 32], None, None));
    let pending: Vec<_> = (0..32)
        .map(|_| must(queue.enqueue([1; 32], [7; 32], None, None)))
        .collect();
    assert!(matches!(
        queue.enqueue([1; 32], [8; 32], None, None),
        Err(QueueError::Overflow)
    ));
    let other = must(queue.enqueue([2; 32], [7; 32], None, None));
    assert_eq!(must(other.slot()), Some(GenerationSlot::Generation));
    drop(pending);
    drop(active);
    let next = must(queue.enqueue([1; 32], [8; 32], Some(2), None));
    assert_eq!(must(next.slot()), Some(GenerationSlot::Generation));
}

fn healthy_routes() -> ServeRoutes {
    let mut health = InferenceBootGate::default();
    must(health.ready(
        Reference {
            head_sequence: 10,
            checkpoint: [7; 32],
        },
        Rung::Committed,
        &json!({"F11":"available"}),
    ));
    must(ServeRoutes::new(
        [1; 32],
        [2; 32],
        None,
        None,
        Arc::new(Mutex::new(health)),
    ))
}

#[test]
fn overflow_answers_429_with_retry_after_on_both_completion_routes() {
    for path in ["/v1/chat/completions", "/v1/messages"] {
        let queue = RequestQueue::default();
        let (binding, _, _) = binding(7, 2, 2);
        let active = must(queue.enqueue(
            [1; 32],
            [7; 32],
            Some(binding.room.profile.queue_depth),
            None,
        ));
        let first = must(queue.enqueue([1; 32], [7; 32], Some(2), None));
        let second = must(queue.enqueue([1; 32], [7; 32], Some(2), None));
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        let server = thread::spawn(move || {
            let (mut stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            let routes = healthy_routes();
            let mut connection = must(CompletionConnection::read(&mut stream));
            assert!(connection.is_completion());
            let request = present(
                must(routes.read_request(&mut connection)),
                "completion request",
            );
            assert_eq!(request.ask, binding.deal.ask_id.bytes());
            let Err(error) = queue.enqueue(
                binding.room.room_id.bytes(),
                request.ask,
                Some(binding.room.profile.queue_depth),
                None,
            ) else {
                panic!("full room admitted a request");
            };
            assert_eq!(error, QueueError::Overflow);
            must(error.respond(&mut connection));
        });
        let mut client = must(TcpStream::connect(address));
        must(client.set_read_timeout(Some(Duration::from_secs(5))));
        let body = must(serde_json::to_vec(
            &json!({"model":"02".repeat(32), "messages":[{"role":"user","content":"Hello"}], "max_tokens":3}),
        ));
        must(write!(client, "POST {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", "07".repeat(32), body.len()));
        must(client.write_all(&body));
        let mut response = String::new();
        must(client.read_to_string(&mut response));
        must(server.join());
        let (head, body) = present(response.split_once("\r\n\r\n"), "HTTP framing");
        assert!(
            head.starts_with("HTTP/1.1 429 Too Many Requests\r\n"),
            "{response}"
        );
        assert!(head.contains("\r\nRetry-After: 1\r\n"));
        assert!(head.contains(&format!("\r\nContent-Length: {}\r\n", body.len())));
        let value: Value = must(serde_json::from_str(body));
        assert_eq!(value["error"]["type"], "rate_limit_error");
        assert_eq!(value["error"]["message"], "room_queue_full");
        drop((active, first, second));
    }
}

#[test]
fn speculative_slots_require_a_draft_capable_peer_in_the_deal() {
    let (bound, frame, native) = binding(7, 32, 1);
    let split = &frame.plan.layers[0];
    let capability = present(
        must(DraftPeer::from_deal_manifest(
            &bound, &frame, split, &native, 1,
        )),
        "advertised draft runtime kind",
    );
    assert_eq!(capability, draft(7));
    assert_eq!(
        must(DraftPeer::from_deal_manifest(
            &bound, &frame, split, &native, 2
        )),
        None
    );
    assert!(DraftPeer::from_deal_manifest(&bound, &frame, split, &native, 3).is_err());
    let (_, unrelated_frame, _) = binding(8, 32, 1);
    assert!(DraftPeer::from_deal_manifest(
        &bound,
        &unrelated_frame,
        &unrelated_frame.plan.layers[0],
        &native,
        1
    )
    .is_err());
    let mut unassigned = split.clone();
    unassigned.peer_key = [99; 32];
    assert!(DraftPeer::from_deal_manifest(&bound, &frame, &unassigned, &native, 1).is_err());
    let daemon = manifest(2);
    assert!(DraftPeer::from_deal_manifest(&bound, &frame, split, &daemon, 1).is_err());
    let (daemon_bound, daemon_frame, daemon_manifest) = binding(9, 32, 2);
    assert_eq!(
        must(DraftPeer::from_deal_manifest(
            &daemon_bound,
            &daemon_frame,
            &daemon_frame.plan.layers[0],
            &daemon_manifest,
            1
        )),
        None
    );

    let queue = RequestQueue::default();
    assert!(matches!(
        queue.enqueue([2; 32], [7; 32], None, Some(capability)),
        Err(QueueError::Unavailable)
    ));
    assert!(matches!(
        queue.enqueue([1; 32], [8; 32], None, Some(capability)),
        Err(QueueError::Unavailable)
    ));
    let active = must(queue.enqueue([1; 32], [7; 32], None, None));
    let ordinary = must(queue.enqueue([1; 32], [8; 32], None, None));
    assert_eq!(must(ordinary.slot()), None);
    let same_ask = must(queue.enqueue([1; 32], [7; 32], None, Some(capability)));
    assert_eq!(must(same_ask.slot()), None);
    let mut speculative = Vec::new();
    for index in 0..MAX_SPECULATIVE_SLOTS {
        let ask = 20 + must(u8::try_from(index));
        let peer = draft(ask);
        let ticket = must(queue.enqueue([1; 32], [ask; 32], None, Some(peer)));
        match present(must(ticket.slot()), "draft admission") {
            GenerationSlot::Speculative(slot) => {
                assert_eq!(slot.index, index);
                assert_eq!(slot.peer, peer);
            }
            GenerationSlot::Generation => panic!("second primary generation"),
        }
        speculative.push(ticket);
    }
    let fifth = must(queue.enqueue([1; 32], [30; 32], None, Some(draft(30))));
    assert_eq!(must(fifth.slot()), None);
    let released = speculative.remove(1);
    drop(released);
    assert!(
        matches!(must(fifth.slot()), Some(GenerationSlot::Speculative(slot)) if slot.index == 1)
    );
    assert_eq!(must(ordinary.slot()), None);
    drop(active);
    assert_eq!(must(ordinary.slot()), Some(GenerationSlot::Generation));
}

#[test]
fn cancellation_and_timeout_release_capacity_and_wake_waiters() {
    let queue = RequestQueue::default();
    let active = must(queue.enqueue([1; 32], [7; 32], Some(1), None));
    let cancelled = must(queue.enqueue([1; 32], [8; 32], Some(1), None));
    assert!(matches!(
        queue.enqueue([1; 32], [9; 32], Some(1), None),
        Err(QueueError::Overflow)
    ));
    drop(cancelled);
    let expired = must(queue.enqueue([1; 32], [8; 32], Some(1), None));
    assert!(matches!(
        expired.wait(Duration::ZERO),
        Err(QueueError::Timeout)
    ));
    let waiting = must(queue.enqueue([1; 32], [9; 32], Some(1), None));
    let thread = thread::spawn(move || {
        let permit = must(waiting.wait(Duration::from_secs(5)));
        assert_eq!(permit.slot, GenerationSlot::Generation);
    });
    drop(active);
    must(thread.join());
    let next = must(queue.enqueue([1; 32], [8; 32], Some(1), None));
    assert_eq!(must(next.slot()), Some(GenerationSlot::Generation));
}
