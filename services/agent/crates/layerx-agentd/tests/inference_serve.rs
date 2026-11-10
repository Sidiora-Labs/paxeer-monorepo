mod signal {
    include!("../src/inference_market/signal.rs");
    pub mod deals {
        include!("../src/inference_market/deals.rs");
        pub mod serve {
            include!("../src/inference_market/serve.rs");
        }
    }
}

use std::fmt::Debug;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::DirBuilderExt as _;
use std::os::unix::net::UnixListener;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{mpsc, Arc, Mutex};
use std::thread;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::boot::InferenceBootGate;
use layerx_agentd::degraded::Reference;
use layerx_agentd::node_rpc::AskStreamState;
use layerx_client::ai_market::market_program::{
    laps::LapReceipt,
    policy::RoomProfile,
    rooms::{DealRecord, DealState, LayerSplit, RoomRecord, RoomState, MAX_DEAL_PEERS},
    types::{AccountId, Digest32, PrincipalId, RosterDigest, Signature64},
};
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use signal::deals::serve::{
    write_error, AskGate, CompletionBinding, CompletionConnection, CompletionWriter, FinishReason,
    ServeRoutes, TokenPiece, BODY_LIMIT,
};

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("inference completion proof failed: {error:?}"))
}
fn digest(value: u8) -> Digest32 {
    must(Digest32::new([value; 32]))
}
fn host() -> SigningKey {
    SigningKey::from_bytes(&[9; 32])
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "inference-serve-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::DirBuilder::new().mode(0o700).create(&path));
        Self(path)
    }
    fn routes(&self) -> ServeRoutes {
        must(ServeRoutes::new(
            [1; 32],
            [2; 32],
            Some(self.0.join("runtime.sock")),
            Some(self.0.join("laps.jsonl")),
            healthy(),
        ))
    }
    fn worker(&self) -> UnixListener {
        must(UnixListener::bind(self.0.join("runtime.sock")))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
fn healthy() -> Arc<Mutex<InferenceBootGate>> {
    let mut gate = InferenceBootGate::default();
    must(gate.ready(
        Reference {
            head_sequence: 10,
            checkpoint: [7; 32],
        },
        Rung::Committed,
        &json!({"F11":"available"}),
    ));
    Arc::new(Mutex::new(gate))
}
fn binding(maximum: u64) -> CompletionBinding {
    let profile = RoomProfile {
        minimum_bond: 1,
        canary_ratio_bps: 0,
        lap_size_tokens: 2,
        challenge_window_blocks: 10,
        slash_fraction_bps: 100,
        host_share_bps: 1000,
        protocol_share_bps: 100,
        settlement_account: must(AccountId::new([4; 32])),
        max_generated_tokens: maximum,
        queue_depth: 32,
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
    let mut layers = [None; MAX_DEAL_PEERS];
    layers[0] = Some(LayerSplit {
        bid_id: digest(6),
        first_layer: 0,
        end_layer: 2,
    });
    let deal = DealRecord {
        room_id: digest(1),
        deal_id: digest(8),
        ask_id: digest(7),
        agreed_price_per_thousand_tokens: 1000,
        roster_digest: must(RosterDigest::new([10; 32])),
        layers,
        state: DealState::Open,
    };
    // Use the canonical program records on both sides of the serving boundary.
    let mut bytes = vec![0; 4096];
    let length = must(room.encode(&mut bytes));
    let room = must(RoomRecord::decode(&bytes[..length]));
    let length = must(deal.encode(&mut bytes));
    let deal = must(DealRecord::decode(&bytes[..length]));
    must(CompletionBinding::new(room, deal))
}
fn funded(cap: u128) -> AskStreamState {
    AskStreamState {
        stream_id: [11; 32],
        recipient: [4; 32],
        mode: 2,
        rate: 1,
        rate_unit: 1,
        total_cap: cap,
        accrued_total: 0,
        settled_total: 0,
        remainder_carry: 0,
        cumulative_meter: 0,
        meter_authorities: vec![host().verifying_key().to_bytes()],
        underfunded: false,
        paused: false,
        closed: false,
    }
}
fn body(routes: &ServeRoutes, streaming: bool) -> Value {
    json!({"model": routes.model_alias(), "messages":[{"role":"user","content":"Hello"}],
        "max_tokens":3,"stream":streaming})
}
fn wire(path: &str, body: &Value) -> Vec<u8> {
    let bytes = must(serde_json::to_vec(body));
    let mut request = format!("POST {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", "07".repeat(32), bytes.len()).into_bytes();
    request.extend_from_slice(&bytes);
    request
}

// Node I/O ends at the concrete AskStreamState boundary. This listener drives
// the same parser, admission predicate and responder as ServeRoutes::serve.
fn listener(
    routes: Arc<ServeRoutes>,
    state: Option<AskStreamState>,
    bound: CompletionBinding,
) -> (TcpStream, thread::JoinHandle<()>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let address = must(listener.local_addr());
    let server = thread::spawn(move || {
        let (mut stream, _) = must(listener.accept());
        must(stream.set_read_timeout(Some(Duration::from_secs(5))));
        must(stream.set_write_timeout(Some(Duration::from_secs(5))));
        let mut stream = must(CompletionConnection::read(&mut stream));
        assert!(stream.is_completion());
        match routes.read_request(&mut stream) {
            Ok(Some(request)) => match AskGate::check(state) {
                Ok(gate) => {
                    let _ = routes.respond(&mut stream, request, gate, bound);
                }
                Err(error) => must(write_error(&mut stream, &error)),
            },
            Ok(None) => (),
            Err(error) => must(write_error(&mut stream, &error)),
        }
    });
    let client = must(TcpStream::connect(address));
    must(client.set_read_timeout(Some(Duration::from_secs(5))));
    (client, server)
}
fn exchange(
    routes: Arc<ServeRoutes>,
    state: Option<AskStreamState>,
    bound: CompletionBinding,
    bytes: &[u8],
) -> String {
    let (mut client, server) = listener(routes, state, bound);
    must(client.write_all(bytes));
    let mut client = BufReader::new(client);
    let mut response = String::new();
    let mut length = None;
    loop {
        let mut line = String::new();
        assert!(must(client.read_line(&mut line)) > 0);
        if let Some(value) = line.strip_prefix("Content-Length: ") {
            length = Some(must(value.trim().parse::<usize>()));
        }
        response.push_str(&line);
        if line == "\r\n" {
            break;
        }
    }
    if let Some(length) = length {
        let mut bytes = vec![0; length];
        must(client.read_exact(&mut bytes));
        response.push_str(&must(String::from_utf8(bytes)));
    } else {
        must(client.read_to_string(&mut response));
    }
    must(server.join());
    response
}
fn status(response: &str, expected: u16) {
    assert!(
        response.starts_with(&format!("HTTP/1.1 {expected} ")),
        "{response}"
    );
}
fn document(response: &str) -> Value {
    must(serde_json::from_str(
        must(response.split_once("\r\n\r\n").ok_or("HTTP body")).1,
    ))
}
fn events(response: &str) -> Vec<Value> {
    let body = must(response.split_once("\r\n\r\n").ok_or("SSE body")).1;
    body.lines()
        .filter_map(|line| line.strip_prefix("data: "))
        .filter(|line| *line != "[DONE]")
        .map(|line| must(serde_json::from_str(line)))
        .collect()
}

fn publish<W: Read + Write>(
    writer: &mut CompletionWriter<W>,
    index: u64,
    prompt: u64,
    cumulative: u64,
    pieces: &[&str],
    signer: &SigningKey,
) -> (u64, u64) {
    let ranges = [digest(12)];
    let mut receipt = LapReceipt {
        deal_id: digest(8),
        lap_index: index,
        prompt_tokens: prompt,
        generated_tokens: pieces.len() as u64,
        cumulative_tokens: cumulative,
        layer_range_digests: &ranges,
        host_signature: Signature64([1; 64]),
        peer_countersignatures: 1,
    };
    receipt.host_signature = Signature64(
        signer
            .sign(must(receipt.signing_digest()).as_bytes())
            .to_bytes(),
    );
    let tokens = pieces
        .iter()
        .enumerate()
        .map(|(index, text)| TokenPiece {
            token_id: index as u32,
            text: (*text).to_owned(),
        })
        .collect();
    must(writer.lap(&receipt, tokens));
    (receipt.prompt_tokens, receipt.generated_tokens)
}
fn completion_worker(
    worker: UnixListener,
    maximum: u64,
    system: bool,
) -> thread::JoinHandle<(u64, u64)> {
    thread::spawn(move || {
        let (socket, _) = must(worker.accept());
        must(socket.set_read_timeout(Some(Duration::from_secs(5))));
        let mut writer = must(CompletionWriter::accept(socket));
        let request = writer.request();
        assert_eq!(request.ask_id, [7; 32]);
        assert_eq!(request.room_id, [1; 32]);
        assert_eq!(request.deal_id, [8; 32]);
        assert_eq!(request.model_digest, [2; 32]);
        assert_eq!(request.max_generated_tokens, maximum);
        assert_eq!(
            request
                .messages
                .last()
                .map(|message| message.content.as_str()),
            Some("Hello")
        );
        if system {
            assert_eq!(request.messages[0].role, "system");
            assert_eq!(request.messages[0].content, "Be concise");
        }
        let first = publish(&mut writer, 0, 2, 4, &["Good", " morning"], &host());
        let last = publish(&mut writer, 1, 0, 5, &["!"], &host());
        must(writer.finish(FinishReason::Length));
        (first.0 + last.0, first.1 + last.1)
    })
}

#[test]
fn completion_dispatch_preserves_internal_lap_and_health_requests() {
    for path in ["/internal/inference/laps", "/healthz", "/v1/messages/other"] {
        let request = format!(
            "{} {path} HTTP/1.1\r\nContent-Length: 2\r\n\r\n{{}}",
            if path == "/healthz" { "GET" } else { "POST" }
        );
        let listener = must(TcpListener::bind("127.0.0.1:0"));
        let address = must(listener.local_addr());
        let expected = request.clone();
        let server = thread::spawn(move || {
            let (mut socket, _) = must(listener.accept());
            must(socket.set_read_timeout(Some(Duration::from_secs(5))));
            let mut connection = must(CompletionConnection::read(&mut socket));
            assert!(!connection.is_completion());
            let mut received = vec![0; expected.len()];
            must(connection.read_exact(&mut received));
            assert_eq!(received, expected.as_bytes());
            must(connection.write_all(b"HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n"));
            must(connection.flush());
        });
        let mut client = must(TcpStream::connect(address));
        must(client.set_read_timeout(Some(Duration::from_secs(5))));
        must(client.write_all(request.as_bytes()));
        let mut response = String::new();
        must(client.read_to_string(&mut response));
        status(&response, 200);
        must(server.join());
    }
}

#[test]
fn unfunded_absent_closed_paused_underfunded_and_capped_asks_answer_402() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    let mut states = vec![None];
    let mut closed = funded(20);
    closed.closed = true;
    states.push(Some(closed));
    let mut capped = funded(20);
    capped.accrued_total = 20;
    states.push(Some(capped));
    let mut paused = funded(20);
    paused.paused = true;
    states.push(Some(paused));
    let mut underfunded = funded(20);
    underfunded.underfunded = true;
    states.push(Some(underfunded));
    for path in ["/v1/chat/completions", "/v1/messages"] {
        for state in &states {
            let response = exchange(
                Arc::clone(&routes),
                state.clone(),
                binding(3),
                &wire(path, &body(&routes, true)),
            );
            status(&response, 402);
            assert_eq!(document(&response)["error"]["type"], "ask_stream_unfunded");
            assert!(!response.contains("text/event-stream"));
        }
    }
    assert_eq!(must(fs::metadata(directory.0.join("laps.jsonl"))).len(), 0);
}

#[test]
fn funded_ask_streams_tokens_and_usage_equals_the_canonical_lap_counts() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    let worker = directory.worker();
    let (release, proceed) = mpsc::channel();
    let (first_lap, produced) = mpsc::channel();
    let producer = thread::spawn(move || {
        let (socket, _) = must(worker.accept());
        let mut writer = must(CompletionWriter::accept(socket));
        assert_eq!(writer.request().max_generated_tokens, 3);
        assert_eq!(writer.request().token_budget, 20);
        let first = publish(&mut writer, 0, 2, 4, &["Good", " morning"], &host());
        must(first_lap.send(()));
        must(proceed.recv_timeout(Duration::from_secs(5)));
        let last = publish(&mut writer, 1, 0, 5, &["!"], &host());
        must(writer.finish(FinishReason::Length));
        (first.0 + last.0, first.1 + last.1)
    });
    let (mut client, server) = listener(Arc::clone(&routes), Some(funded(20)), binding(3));
    must(client.write_all(&wire("/v1/chat/completions", &body(&routes, true))));
    must(produced.recv_timeout(Duration::from_secs(5)));
    let mut client = BufReader::new(client);
    let mut response = String::new();
    loop {
        let mut line = String::new();
        assert!(must(client.read_line(&mut line)) > 0);
        response.push_str(&line);
        if line.contains(" morning") {
            break;
        }
    }
    assert!(!response.contains("[DONE]"));
    must(release.send(()));
    must(client.read_to_string(&mut response));
    must(server.join());
    let counts = must(producer.join());
    status(&response, 200);
    assert!(response.contains("Content-Type: text/event-stream"));
    assert!(response.ends_with("data: [DONE]\n\n"));
    let events = events(&response);
    let text: String = events
        .iter()
        .filter_map(|event| event["choices"][0]["delta"]["content"].as_str())
        .collect();
    assert_eq!(text, "Good morning!");
    let usage = &must(events.last().ok_or("usage chunk"))["usage"];
    assert_eq!(usage["prompt_tokens"], counts.0);
    assert_eq!(usage["completion_tokens"], counts.1);
    assert_eq!(usage["total_tokens"], counts.0 + counts.1);
    assert_eq!(
        events[events.len() - 2]["choices"][0]["finish_reason"],
        "length"
    );
    let retained = must(fs::read_to_string(directory.0.join("laps.jsonl")));
    let records: Vec<Value> = retained
        .lines()
        .map(|line| must(serde_json::from_str(line)))
        .collect();
    assert_eq!(records.len(), 2);
    assert_eq!(records[0]["ask"], json!(vec![7; 32]));
    assert_eq!(records[1]["stream"], json!(vec![11; 32]));
}

#[test]
fn foreign_model_id_answers_404_before_funding_or_generation() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    for path in ["/v1/chat/completions", "/v1/messages"] {
        let mut input = body(&routes, true);
        input["model"] = json!("foreign-model");
        let response = exchange(Arc::clone(&routes), None, binding(3), &wire(path, &input));
        status(&response, 404);
        assert_eq!(document(&response)["error"]["type"], "model_not_found");
    }
    assert_eq!(must(fs::metadata(directory.0.join("laps.jsonl"))).len(), 0);
}

#[test]
fn anthropic_shape_maps_the_same_completion_and_lap_usage() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    let producer = completion_worker(directory.worker(), 3, true);
    let mut input = body(&routes, false);
    input["system"] = json!([{"type":"text","text":"Be concise"}]);
    input["messages"][0]["content"] = json!([{"type":"text","text":"Hello"}]);
    let response = exchange(
        Arc::clone(&routes),
        Some(funded(20)),
        binding(3),
        &wire("/v1/messages", &input),
    );
    let counts = must(producer.join());
    status(&response, 200);
    let message = document(&response);
    assert_eq!(message["type"], "message");
    assert_eq!(message["role"], "assistant");
    assert_eq!(message["model"], routes.model_alias());
    assert_eq!(
        message["content"],
        json!([{"type":"text","text":"Good morning!"}])
    );
    assert_eq!(message["stop_reason"], "max_tokens");
    assert_eq!(message["usage"]["input_tokens"], counts.0);
    assert_eq!(message["usage"]["output_tokens"], counts.1);
}

#[test]
fn anthropic_stream_has_ordered_message_content_and_usage_events() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    let producer = completion_worker(directory.worker(), 3, false);
    let response = exchange(
        Arc::clone(&routes),
        Some(funded(20)),
        binding(3),
        &wire("/v1/messages", &body(&routes, true)),
    );
    let counts = must(producer.join());
    status(&response, 200);
    let events = events(&response);
    let kinds: Vec<_> = events.iter().map(|event| event["type"].as_str()).collect();
    assert_eq!(
        kinds,
        vec![
            Some("message_start"),
            Some("content_block_start"),
            Some("content_block_delta"),
            Some("content_block_delta"),
            Some("content_block_delta"),
            Some("content_block_stop"),
            Some("message_delta"),
            Some("message_stop")
        ]
    );
    assert_eq!(events[0]["message"]["usage"]["input_tokens"], counts.0);
    assert_eq!(events[6]["usage"]["output_tokens"], counts.1);
    assert_eq!(events[6]["delta"]["stop_reason"], "max_tokens");
    let text: String = events
        .iter()
        .filter_map(|event| event["delta"]["text"].as_str())
        .collect();
    assert_eq!(text, "Good morning!");
}

#[test]
fn openai_nonstreaming_maps_messages_and_usage_and_caps_the_worker_request() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    let producer = completion_worker(directory.worker(), 3, false);
    let mut input = body(&routes, false);
    input["max_tokens"] = json!(1000);
    let response = exchange(
        Arc::clone(&routes),
        Some(funded(20)),
        binding(3),
        &wire("/v1/chat/completions", &input),
    );
    let counts = must(producer.join());
    status(&response, 200);
    let completion = document(&response);
    assert_eq!(completion["object"], "chat.completion");
    assert_eq!(
        completion["choices"][0]["message"],
        json!({"role":"assistant","content":"Good morning!"})
    );
    assert_eq!(
        completion["usage"],
        json!({"prompt_tokens":counts.0,"completion_tokens":counts.1,"total_tokens":counts.0 + counts.1})
    );
    assert_eq!(completion["choices"][0]["finish_reason"], "length");
}

#[test]
fn exact_body_limit_is_accepted_and_one_more_byte_is_413() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    for path in ["/v1/chat/completions", "/v1/messages"] {
        let mut bytes = must(serde_json::to_vec(&body(&routes, false)));
        bytes.resize(BODY_LIMIT, b' ');
        let mut request = format!("POST {path} HTTP/1.1\r\nAuthorization: Bearer {}\r\nContent-Type: application/json\r\nContent-Length: {BODY_LIMIT}\r\n\r\n", "07".repeat(32)).into_bytes();
        request.extend_from_slice(&bytes);
        let response = exchange(Arc::clone(&routes), None, binding(3), &request);
        status(&response, 402);
        let oversized = format!(
            "POST {path} HTTP/1.1\r\nContent-Length: {}\r\n\r\n",
            BODY_LIMIT + 1
        );
        let response = exchange(
            Arc::clone(&routes),
            Some(funded(20)),
            binding(3),
            oversized.as_bytes(),
        );
        status(&response, 413);
    }
}

#[test]
fn ambiguous_framing_invalid_bearer_and_invalid_token_bounds_are_refused() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    let request = wire("/v1/chat/completions", &body(&routes, false));
    let request = must(String::from_utf8(request));
    for (changed, expected) in [
        (
            request.replacen(
                "Content-Length:",
                "Transfer-Encoding: chunked\r\nContent-Length:",
                1,
            ),
            400,
        ),
        (
            request.replacen("Content-Length:", "Content-Length: 0\r\nContent-Length:", 1),
            400,
        ),
        (request.replace(&"07".repeat(32), "invalid"), 401),
        (request.replace("application/json", "text/plain"), 415),
    ] {
        status(
            &exchange(
                Arc::clone(&routes),
                Some(funded(20)),
                binding(3),
                changed.as_bytes(),
            ),
            expected,
        );
    }
    for value in [json!(0), json!(-1), json!("3")] {
        let mut input = body(&routes, false);
        input["max_tokens"] = value;
        status(
            &exchange(
                Arc::clone(&routes),
                Some(funded(20)),
                binding(3),
                &wire("/v1/chat/completions", &input),
            ),
            400,
        );
    }
}

#[test]
fn retained_unmetered_laps_prevent_reusing_the_cap_after_restart() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    let producer = completion_worker(directory.worker(), 3, false);
    let response = exchange(
        Arc::clone(&routes),
        Some(funded(5)),
        binding(3),
        &wire("/v1/chat/completions", &body(&routes, false)),
    );
    status(&response, 200);
    assert_eq!(must(producer.join()), (2, 3));
    drop(routes);
    let restarted = Arc::new(directory.routes());
    let response = exchange(
        Arc::clone(&restarted),
        Some(funded(5)),
        binding(3),
        &wire("/v1/chat/completions", &body(&restarted, false)),
    );
    status(&response, 402);
    assert_eq!(
        must(fs::read_to_string(directory.0.join("laps.jsonl")))
            .lines()
            .count(),
        2
    );
}

#[test]
fn a_foreign_receipt_signer_is_rejected_before_streaming_tokens() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    let worker = directory.worker();
    let producer = thread::spawn(move || {
        let (socket, _) = must(worker.accept());
        let mut writer = must(CompletionWriter::accept(socket));
        publish(
            &mut writer,
            0,
            2,
            4,
            &["Good", " morning"],
            &SigningKey::from_bytes(&[99; 32]),
        );
    });
    let response = exchange(
        Arc::clone(&routes),
        Some(funded(20)),
        binding(3),
        &wire("/v1/chat/completions", &body(&routes, true)),
    );
    must(producer.join());
    status(&response, 502);
    assert!(!response.contains("Good"));
    assert_eq!(must(fs::metadata(directory.0.join("laps.jsonl"))).len(), 0);
}

#[test]
fn a_lap_that_exceeds_the_room_generation_limit_is_rejected() {
    let directory = Directory::new();
    let routes = Arc::new(directory.routes());
    let worker = directory.worker();
    let producer = thread::spawn(move || {
        let (socket, _) = must(worker.accept());
        let mut writer = must(CompletionWriter::accept(socket));
        assert_eq!(writer.request().max_generated_tokens, 1);
        publish(&mut writer, 0, 2, 4, &["Good", " morning"], &host());
    });
    let response = exchange(
        Arc::clone(&routes),
        Some(funded(20)),
        binding(1),
        &wire("/v1/chat/completions", &body(&routes, true)),
    );
    must(producer.join());
    status(&response, 502);
    assert!(!response.contains("Good"));
    assert_eq!(must(fs::metadata(directory.0.join("laps.jsonl"))).len(), 0);
}

#[test]
fn fractional_stream_rates_and_pending_meter_counts_bound_admission() {
    let mut state = funded(1);
    state.rate = 3;
    state.rate_unit = 2;
    state.remainder_carry = 1;
    assert_eq!(
        must(
            AskGate::check(Some(state))
                .err()
                .ok_or("fractional cap refusal")
        )
        .status,
        402
    );
    let mut malformed = funded(20);
    malformed.rate = 0;
    assert_eq!(
        must(
            AskGate::check(Some(malformed))
                .err()
                .ok_or("invalid rate refusal")
        )
        .status,
        502
    );
}
