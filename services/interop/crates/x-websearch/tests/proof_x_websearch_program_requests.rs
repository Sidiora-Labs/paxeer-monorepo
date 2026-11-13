//! Proves: `RequestWatcher::program_work` + `ProgramAttestor` -> `ProgramExchange` HTTP + `observation_bytes` -> `ProgramRelay::prepare_program` + `Submitter::submit_program/decode_program_observation`.
//!
//! The test is the first public Rust consumer: its initial input is persisted
//! watcher state, not an RPC recording. Answers, HTTP records, signatures and
//! observations are emitted by the production implementations. It reopens the
//! watcher, index and exchanges after signing before quorum, then changes the
//! index and recovers the same answer. A distinct producer-issued request proves
//! fresh content without allowing a conflicting signature for the original key.
//! It proves the submission decoding boundary; it
//! does not claim an EVM broadcast or receipt without a real chain.
//!
//! Repairs: `program.rs::ProgramExchange::accept` previously ignored contradictory
//! `content_digest`, `response_hash` and `full_length` fields in signed peer records;
//! `watch.rs::RequestWatcher::open` trusted malformed persisted program requests;
//! `submit.rs::Submitter::submit_program` resumed journals before decoding input
//! and did not validate the canonical header, response bounds or signer order.

use std::error::Error;
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::sync::Arc;

use k256::ecdsa::SigningKey;
use serde_json::{json, Value};
use x_websearch::attest::{signer_address, Answer, AttestorSet, Discard, Ready};
use x_websearch::config::FetchLimits;
use x_websearch::content::ContentStore;
use x_websearch::fetch::Fetcher;
use x_websearch::index::WebIndex;
use x_websearch::program::{
    decode_request_record, observation_bytes, program_record, register, ProgramAttestor,
    ProgramExchange, ProgramRelay, ProgramRequest, OBSERVATION_HEADER_BYTES,
};
use x_websearch::submit::{
    decode_program_observation, fulfil_program_calldata, SubmitError, Submitter,
};
use x_websearch::watch::{hex0x, keccak, EvmError, EvmRpc, RequestWatcher};
use x_websearch::{Limits, RouteTable, RunningServer, Server};

type Checked<T = ()> = Result<T, Box<dyn Error>>;
const NETWORK: u32 = 9;
const CHAIN: u64 = 713_714;
const PROGRAM: [u8; 32] = [0xa0; 32];
const REQUEST: u64 = 7;

struct Scratch(PathBuf);

impl Scratch {
    fn new(name: &str) -> Checked<Self> {
        let path = std::env::temp_dir().join(format!(
            "proof-x-websearch-program-{}-{name}",
            std::process::id()
        ));
        std::fs::create_dir(&path)?;
        Ok(Self(path))
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn key(number: u8) -> Checked<SigningKey> {
    let mut scalar = [0; 32];
    scalar[31] = number;
    Ok(SigningKey::from_slice(&scalar)?)
}

// Initial producer state. No answer, peer record, observation or RPC response
// is supplied by this setup; those must all come from production functions.
fn initial_progress() -> Value {
    json!({
        "version": 1,
        "next_block": 31,
        "chain_id": CHAIN,
        "entries": [],
        "program_entries": [{
            "request": {
                "program_id": PROGRAM.to_vec(), "request_id": REQUEST,
                "kind": 2, "payload": b"paxeer".to_vec(), "sequence": 41
            },
            "block_number": 28,
            "source": {
                "chain_id": CHAIN, "block_hash": vec![0x31; 32],
                "transaction_hash": vec![0x71; 32], "log_index": 1
            }
        }]
    })
}

fn seed_watcher(directory: &Path, progress: &Value) -> Checked {
    std::fs::create_dir_all(directory)?;
    std::fs::write(
        directory.join("pending.json"),
        serde_json::to_vec(progress)?,
    )?;
    Ok(())
}

fn attestor(
    number: u8,
    index: &Arc<WebIndex>,
    store: &Arc<ContentStore>,
) -> Checked<ProgramAttestor> {
    Ok(ProgramAttestor::new(
        key(number)?,
        NETWORK,
        Arc::new(Fetcher::new(FetchLimits {
            connect_timeout_ms: 3_000,
            total_timeout_ms: 10_000,
            max_body_bytes: 2_097_152,
            max_redirects: 3,
            allow_loopback: true,
        })?),
        Arc::clone(index),
        Arc::clone(store),
    ))
}

#[test]
fn proof_x_websearch_program_requests() -> Checked {
    let scratch = Scratch::new("crossing")?;
    let watch_dir = scratch.0.join("watch");
    seed_watcher(&watch_dir, &initial_progress())?;
    // A closed local port is an actual unavailable endpoint, not an RPC double.
    // All successful assertions below happen at the public Rust boundary.
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let rpc = EvmRpc::new(&format!("http://{}", listener.local_addr()?))?;
    drop(listener);
    let expected = ProgramRequest {
        program_id: PROGRAM,
        request_id: REQUEST,
        kind: 2,
        payload: b"paxeer".to_vec(),
        sequence: 41,
    };
    let mut signers = vec![
        signer_address(&key(1)?),
        signer_address(&key(2)?),
        signer_address(&key(4)?),
    ];
    signers.sort_unstable();
    let set = AttestorSet {
        signers,
        threshold: 3,
    };
    let producer: Value = serde_json::from_str(include_str!("fixtures/kernel/watch.json"))?;
    let event = &producer["endpoints"]["gateway"]["*"][0]["result"]["events"][0];
    let fresh = decode_request_record(
        x_websearch::payment::unhex(event["program_id"].as_str().ok_or("producer program id")?)
            .ok_or("producer program hex")?
            .try_into()
            .map_err(|_| "producer program width")?,
        event["sequence"].as_u64().ok_or("producer sequence")?,
        &x_websearch::payment::unhex(event["data"].as_str().ok_or("producer request bytes")?)
            .ok_or("producer request hex")?,
    )?;
    assert_ne!(fresh.request_id, expected.request_id);
    let requests = [expected.clone(), fresh];
    let mut previous_digest = None;
    let mut previous_response = None;

    for (round, expected) in requests.iter().enumerate() {
        let context = RoundContext {
            scratch: &scratch,
            watch_dir: &watch_dir,
            rpc: &rpc,
            set: &set,
            expected,
            round,
        };
        run_round(&context, &mut previous_digest, &mut previous_response)?;
    }
    let watcher = RequestWatcher::open(rpc.clone(), 2, &watch_dir, None)?.with_chain_id(CHAIN)?;
    assert!(watcher.program_work().is_empty());
    drop(watcher);
    assert!(RequestWatcher::open(rpc, 2, &watch_dir, None)?
        .with_chain_id(CHAIN)?
        .program_work()
        .is_empty());
    Ok(())
}

type ProgramKey = ([u8; 32], u64);

struct RoundContext<'a> {
    scratch: &'a Scratch,
    watch_dir: &'a Path,
    rpc: &'a EvmRpc,
    set: &'a AttestorSet,
    expected: &'a ProgramRequest,
    round: usize,
}

struct PreparedRound {
    request: ProgramRequest,
    program_key: ProgramKey,
    title: String,
    peer_answer: Answer,
    unknown: Answer,
    partial_set: AttestorSet,
    partial: Ready,
    local_answer: Answer,
    third_answer: Answer,
}

struct ReopenedRound {
    relay: ProgramRelay,
    server: RunningServer,
    third_server: RunningServer,
    observation: Vec<u8>,
    ready: Ready,
}

fn run_round(
    ctx: &RoundContext<'_>,
    previous_digest: &mut Option<[u8; 32]>,
    previous_response: &mut Option<Vec<u8>>,
) -> Checked {
    let prepared = prepare_round(ctx)?;
    let reopened = reopen_round(ctx, &prepared, previous_digest, previous_response)?;
    assert_submitter_refusals(&prepared, reopened)
}

fn prepare_round(ctx: &RoundContext<'_>) -> Checked<PreparedRound> {
    let RoundContext {
        scratch,
        watch_dir,
        rpc,
        set,
        expected,
        round,
    } = *ctx;
    seed_round_watcher(watch_dir, expected, round)?;
    let index = Arc::new(WebIndex::open(&scratch.0.join("search"))?);
    let store = Arc::new(ContentStore::open(&scratch.0.join("content"), &[])?);
    let title = format!("Paxeer observation round {round}");
    index.put(
        "https://paxeer.app/proof",
        &title,
        "paxeer network program requests",
    )?;
    index.commit()?;
    let peer_attestor = attestor(2, &index, &store)?;
    let peer = Arc::new(ProgramExchange::open(&scratch.0.join("peer"), &[])?);
    peer.bind_chain(CHAIN, NETWORK, peer_attestor.signer())?;
    let third_attestor = attestor(4, &index, &store)?;
    let third = Arc::new(ProgramExchange::open(&scratch.0.join("third"), &[])?);
    third.bind_chain(CHAIN, NETWORK, third_attestor.signer())?;
    let mut third_routes = RouteTable::new();
    register(&mut third_routes, &third)?;
    let third_server =
        Server::bind("127.0.0.1:0".parse()?, Limits::default(), third_routes)?.spawn()?;
    let mut routes = RouteTable::new();
    register(&mut routes, &peer)?;
    let server = Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?;
    let exchange = Arc::new(ProgramExchange::open(
        &scratch.0.join("local"),
        &[
            format!("http://{}", server.local_addr()),
            format!("http://{}", third_server.local_addr()),
        ],
    )?);
    exchange.bind_chain(CHAIN, NETWORK, signer_address(&key(1)?))?;
    let relay = ProgramRelay {
        watcher: RequestWatcher::open((*rpc).clone(), 2, watch_dir, None)?.with_chain_id(CHAIN)?,
        attestor: attestor(1, &index, &store)?,
        exchange,
        set: set.clone(),
        submitter: Submitter::open((*rpc).clone(), key(4)?, CHAIN, &scratch.0.join("submit"))?,
    };
    assert_eq!(relay.watcher.program_work(), vec![expected.clone()]);
    // A returned snapshot cannot mutate the producer's work queue.
    let mut snapshot = relay.watcher.program_work();
    snapshot[0].payload.clear();
    assert_eq!(relay.watcher.program_work(), vec![expected.clone()]);
    let request = relay.watcher.program_work().remove(0);
    let program_key = (request.program_id, request.request_id);
    assert!(
        relay.prepare_program(&request)?.is_none(),
        "a missing peer cannot reach quorum"
    );
    let peer_answer = peer_attestor.attest(&request)?;
    let unknown = attestor(3, &index, &store)?.attest(&request)?;
    peer.record(peer_answer.clone())?;
    assert!(
        relay.prepare_program(&request)?.is_none(),
        "two signatures cannot reach a three-member quorum"
    );
    let partial_set = AttestorSet {
        signers: set.signers.clone(),
        threshold: 2,
    };
    let partial = relay
        .exchange
        .ready(program_key, &partial_set)
        .ok_or("peer signature did not cross HTTP")?;
    let local_answer = relay
        .exchange
        .answer(program_key)
        .ok_or("local signed answer")?;
    let third_answer = third_attestor.attest(&request)?;
    third.record(third_answer.clone())?;
    assert_eq!(partial.digest, peer_answer.digest);
    assert_eq!(partial.signers.len(), 2);
    // Stop all producers and transports before reopening their durable state.
    server.shutdown()?;
    third_server.shutdown()?;
    drop(relay);
    drop(peer);
    drop(third);
    drop(peer_attestor);
    drop(third_attestor);
    drop(index);
    drop(store);
    Ok(PreparedRound {
        request,
        program_key,
        title,
        peer_answer,
        unknown,
        partial_set,
        partial,
        local_answer,
        third_answer,
    })
}

fn seed_round_watcher(watch_dir: &Path, expected: &ProgramRequest, round: usize) -> Checked {
    if round != 0 {
        let mut progress = initial_progress();
        progress["program_entries"][0]["request"] = json!({
            "program_id": expected.program_id.to_vec(), "request_id": expected.request_id,
            "kind": expected.kind, "payload": expected.payload, "sequence": expected.sequence,
        });
        seed_watcher(watch_dir, &progress)?;
    }
    Ok(())
}

fn reopen_round(
    ctx: &RoundContext<'_>,
    prepared: &PreparedRound,
    previous_digest: &mut Option<[u8; 32]>,
    previous_response: &mut Option<Vec<u8>>,
) -> Checked<ReopenedRound> {
    let RoundContext {
        scratch,
        watch_dir,
        rpc,
        set,
        round,
        ..
    } = *ctx;
    let PreparedRound {
        program_key,
        peer_answer,
        unknown,
        partial_set,
        partial,
        local_answer,
        third_answer,
        ..
    } = prepared;
    let index = Arc::new(WebIndex::open(&scratch.0.join("search"))?);
    let changed_title = format!("Paxeer changed index after signing round {round}");
    index.put(
        "https://paxeer.app/proof",
        &changed_title,
        "paxeer network program requests",
    )?;
    index.commit()?;
    let store = Arc::new(ContentStore::open(&scratch.0.join("content"), &[])?);
    let peer = Arc::new(ProgramExchange::open(&scratch.0.join("peer"), &[])?);
    peer.bind_chain(CHAIN, NETWORK, signer_address(&key(2)?))?;
    assert_eq!(peer.answer(*program_key), Some(peer_answer.clone()));
    let third = Arc::new(ProgramExchange::open(&scratch.0.join("third"), &[])?);
    third.bind_chain(CHAIN, NETWORK, signer_address(&key(4)?))?;
    assert_eq!(third.answer(*program_key), Some(third_answer.clone()));
    let mut routes = RouteTable::new();
    register(&mut routes, &peer)?;
    let server = Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?;
    let mut routes = RouteTable::new();
    register(&mut routes, &third)?;
    let third_server = Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?;
    let exchange = Arc::new(ProgramExchange::open(
        &scratch.0.join("local"),
        &[
            format!("http://{}", server.local_addr()),
            format!("http://{}", third_server.local_addr()),
        ],
    )?);
    exchange.bind_chain(CHAIN, NETWORK, signer_address(&key(1)?))?;
    assert_eq!(exchange.answer(*program_key), Some(local_answer.clone()));
    assert_eq!(
        exchange.ready(*program_key, partial_set),
        Some(partial.clone())
    );
    assert!(exchange.ready(*program_key, set).is_none());
    let relay = ProgramRelay {
        watcher: RequestWatcher::open((*rpc).clone(), 2, watch_dir, None)?.with_chain_id(CHAIN)?,
        attestor: attestor(1, &index, &store)?,
        exchange,
        set: set.clone(),
        submitter: Submitter::open((*rpc).clone(), key(4)?, CHAIN, &scratch.0.join("submit"))?,
    };
    let (ready, observation) = assert_reopened_response(
        &relay,
        ctx,
        prepared,
        &store,
        previous_digest,
        previous_response,
    )?;
    assert_discarded_responses(&relay, *program_key, set, peer_answer, unknown, &ready)?;
    Ok(ReopenedRound {
        relay,
        server,
        third_server,
        observation,
        ready,
    })
}

fn assert_reopened_response(
    relay: &ProgramRelay,
    ctx: &RoundContext<'_>,
    prepared: &PreparedRound,
    store: &Arc<ContentStore>,
    previous_digest: &mut Option<[u8; 32]>,
    previous_response: &mut Option<Vec<u8>>,
) -> Checked<(Ready, Vec<u8>)> {
    let request = &prepared.request;
    let program_key = prepared.program_key;
    let set = ctx.set;
    let peer_answer = &prepared.peer_answer;
    let title = &prepared.title;

    assert_eq!(relay.watcher.program_work(), vec![request.clone()]);
    let changed = relay.attestor.attest(request)?;
    assert_ne!(
        changed.response, peer_answer.response,
        "the reopened index really changed"
    );
    assert_ne!(changed.digest, peer_answer.digest);
    assert!(
        relay.exchange.record(changed).is_err(),
        "a conflicting answer cannot replace the persisted signature"
    );
    let ready = relay
        .prepare_program(request)?
        .ok_or("real peer did not reach quorum")?;
    assert_eq!(ready.signers, set.signers);
    assert_eq!(ready.digest, peer_answer.digest);
    assert_eq!(ready.response, peer_answer.response);
    let response: Value = serde_json::from_slice(&ready.response)?;
    assert_eq!(response[0]["url"], "https://paxeer.app/proof");
    assert_eq!(response[0]["title"], title.as_str());
    assert!(store.get(&ready.content_digest)?.is_some());
    if let Some(digest) = *previous_digest {
        assert_ne!(
            ready.digest, digest,
            "the second crossing must observe changed content"
        );
        assert_ne!(Some(ready.response.clone()), *previous_response);
    }
    *previous_digest = Some(ready.digest);
    *previous_response = Some(ready.response.clone());

    // The observation is emitted by the real encoder and decoded by the
    // exact consumer that submit_program calls before journal/RPC access.
    let observation = observation_bytes(NETWORK, request, &ready)?;
    assert_eq!(
        decode_program_observation(program_key, &observation, 3)?,
        ready
    );
    let calldata = fulfil_program_calldata(&observation);
    assert_eq!(&calldata[..4], &keccak(b"fulfilProgram(bytes)")[..4]);
    assert_eq!(&calldata[68..68 + observation.len()], observation);
    assert!(calldata[68 + observation.len()..]
        .iter()
        .all(|byte| *byte == 0));
    Ok((ready, observation))
}

fn assert_discarded_responses(
    relay: &ProgramRelay,
    program_key: ProgramKey,
    set: &AttestorSet,
    peer_answer: &Answer,
    unknown: &Answer,
    ready: &Ready,
) -> Checked {
    // Mutations are of real producer output, never replacement records.
    for field in ["content_digest", "response_hash", "full_length"] {
        let mut malformed = program_record(peer_answer);
        malformed[field] = if field == "full_length" {
            json!(peer_answer.attestation.full_length + 1)
        } else {
            json!(hex0x(&[0xff; 32]))
        };
        assert_eq!(
            relay
                .exchange
                .accept("altered-peer", program_key, &malformed, set),
            Err(Discard::DifferentDigest)
        );
    }
    assert_eq!(
        relay
            .exchange
            .accept("unknown", program_key, &program_record(unknown), set),
        Err(Discard::UnknownSigner)
    );
    let mut malformed_record = program_record(peer_answer);
    malformed_record["signature"] = json!("0x00");
    assert_eq!(
        relay
            .exchange
            .accept("malformed", program_key, &malformed_record, set),
        Err(Discard::Malformed)
    );
    assert_eq!(relay.exchange.ready(program_key, set), Some(ready.clone()));
    assert_eq!(relay.exchange.discarded().len(), 5);
    let discards = std::fs::read_to_string(relay.exchange.log_path())?;
    assert!(discards.contains("different_digest"));
    assert!(discards.contains("unknown_signer"));
    assert!(discards.contains("malformed"));
    Ok(())
}

fn assert_submitter_refusals(prepared: &PreparedRound, reopened: ReopenedRound) -> Checked {
    let ReopenedRound {
        relay,
        server,
        third_server,
        observation,
        ready,
    } = reopened;
    let request = &prepared.request;
    let program_key = prepared.program_key;
    let mut submitter = relay.submitter;
    // Prove bad input is refused even when the journal would itself fail:
    // a cached/resumable entry must not bypass the decoder.
    let journal_path = submitter.journal().path(program_key);
    std::fs::write(&journal_path, b"{")?;
    let mut malformed_observations = Vec::new();
    for offset in [0, 1, 73] {
        let mut malformed = observation.clone();
        malformed[offset] = 0xff;
        malformed_observations.push(malformed);
    }
    let mut truncated = observation.clone();
    truncated.pop();
    malformed_observations.push(truncated);
    let mut short_full_length = observation.clone();
    short_full_length[138..142].fill(0);
    malformed_observations.push(short_full_length);
    let mut reversed = observation.clone();
    let signatures_at = OBSERVATION_HEADER_BYTES + ready.response.len() + 1;
    let (first, rest) = reversed[signatures_at..].split_at_mut(65);
    first.swap_with_slice(&mut rest[..65]);
    malformed_observations.push(reversed);
    let mut bad_signature = observation.clone();
    bad_signature[signatures_at + 64] = 0;
    malformed_observations.push(bad_signature);
    for malformed in malformed_observations {
        assert_eq!(
            decode_program_observation(program_key, &malformed, 3),
            Err(SubmitError::Encode)
        );
        assert_eq!(
            submitter.submit_program(program_key, &malformed, 3),
            Err(SubmitError::Encode)
        );
    }
    assert_eq!(
        submitter.submit_program(([0xb0; 32], REQUEST), &observation, 3),
        Err(SubmitError::Encode)
    );
    for count in [0, 1, 2, 4, 17, usize::MAX] {
        assert_eq!(
            submitter.submit_program(program_key, &observation, count),
            Err(SubmitError::Encode)
        );
    }
    std::fs::remove_file(journal_path)?;
    assert_eq!(
        submitter.submit_program(program_key, &observation, 3),
        Err(SubmitError::Evm(EvmError::Unavailable))
    );
    assert!(submitter.journal().load(program_key)?.is_none());
    server.shutdown()?;
    third_server.shutdown()?;
    // Dropping the watcher releases its real file lock before reopening.
    let mut watcher = relay.watcher;
    watcher.retire_program(request.program_id, request.request_id)?;
    drop(watcher);
    Ok(())
}

#[test]
fn malformed_persisted_program_producer_state_is_refused() -> Checked {
    let scratch = Scratch::new("malformed-state")?;
    let rpc = EvmRpc::new("http://127.0.0.1:1")?;
    for (field, value) in [
        ("/program_entries/0/request/kind", json!(3)),
        ("/program_entries/0/request/program_id", json!(vec![0; 32])),
        ("/program_entries/0/request/payload", json!([])),
        ("/program_entries/0/request/payload", json!(vec![0; 65_537])),
        ("/program_entries/0/source/chain_id", json!(CHAIN + 1)),
        ("/program_entries/0/source/block_hash", json!(vec![0; 32])),
        (
            "/program_entries/0/source/transaction_hash",
            json!(vec![0; 32]),
        ),
        ("/program_entries/0/block_number", json!(31)),
        ("/next_block", Value::Null),
    ] {
        let mut progress = initial_progress();
        *progress.pointer_mut(field).ok_or("invalid mutation path")? = value;
        seed_watcher(&scratch.0, &progress)?;
        let error = RequestWatcher::open(rpc.clone(), 2, &scratch.0, None)
            .err()
            .ok_or("malformed program state was accepted")?;
        assert_eq!(error.kind(), std::io::ErrorKind::InvalidData, "{field}");
    }
    Ok(())
}
