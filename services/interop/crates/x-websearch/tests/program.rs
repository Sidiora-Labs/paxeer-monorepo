use std::error::Error;
use std::path::PathBuf;
use std::sync::Arc;

use k256::ecdsa::SigningKey;
use serde_json::Value;
use x_websearch::attest::{recover_signer, signer_address, Answer, AttestorSet, Discard, Ready};
use x_websearch::config::FetchLimits;
use x_websearch::content::ContentStore;
use x_websearch::fetch::Fetcher;
use x_websearch::index::WebIndex;
use x_websearch::kernel::decode_request_record;
use x_websearch::payment::{hex, unhex};
use x_websearch::program::{
    observation_bytes, program_record, register, ProgramAttestor, ProgramExchange, ProgramRequest,
};
use x_websearch::watch::keccak;
use x_websearch::{Limits, RouteTable, RunningServer, Server};

type Checked<T = ()> = Result<T, Box<dyn Error>>;

const NETWORK: u32 = 9;

struct Scratch(PathBuf);

impl Scratch {
    fn new(name: &str) -> Checked<Self> {
        let path =
            std::env::temp_dir().join(format!("x-websearch-program-{}-{name}", std::process::id()));
        std::fs::create_dir(&path)?;
        Ok(Self(path))
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn recorded_request(name: &str) -> Checked<ProgramRequest> {
    let recording: Value = serde_json::from_str(name)?;
    let event = recording
        .pointer("/endpoints/gateway/*/0/result/events/0")
        .ok_or("recorded request missing")?;
    let program_id = unhex(event["program_id"].as_str().ok_or("program id missing")?)
        .ok_or("program id malformed")?
        .try_into()
        .map_err(|_| "program id length")?;
    let data = unhex(event["data"].as_str().ok_or("record missing")?).ok_or("record malformed")?;
    Ok(decode_request_record(
        program_id,
        event["sequence"].as_u64().ok_or("sequence missing")?,
        &data,
    )?)
}

fn signing_key(index: u8) -> Checked<SigningKey> {
    let mut scalar = [0; 32];
    scalar[31] = index;
    Ok(SigningKey::from_slice(&scalar)?)
}

fn attestor(scratch: &Scratch, index: u8) -> Checked<ProgramAttestor> {
    Ok(ProgramAttestor::new(
        signing_key(index)?,
        NETWORK,
        Arc::new(Fetcher::new(FetchLimits {
            connect_timeout_ms: 3_000,
            total_timeout_ms: 10_000,
            max_body_bytes: 2_097_152,
            max_redirects: 3,
            allow_loopback: true,
        })?),
        Arc::new(WebIndex::open(&scratch.0.join(format!("index-{index}")))?),
        Arc::new(ContentStore::open(
            &scratch.0.join(format!("content-{index}")),
            &[],
        )?),
    ))
}

fn roster(answers: &[&Answer]) -> AttestorSet {
    let mut signers: Vec<_> = answers.iter().map(|answer| answer.signer).collect();
    signers.sort_unstable();
    AttestorSet {
        signers,
        threshold: 2,
    }
}

fn serve_exchange(exchange: &Arc<ProgramExchange>) -> Checked<RunningServer> {
    let mut routes = RouteTable::new();
    register(&mut routes, exchange)?;
    Ok(Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?)
}

#[test]
fn observation_bytes_are_unchanged_by_the_move() -> Checked {
    let scratch = Scratch::new("observation")?;
    let request = recorded_request(include_str!("fixtures/kernel/relay.json"))?;
    assert_eq!(request.request_id, 0x0102_0304_0506_0708);
    assert_eq!(request.kind, 2);
    assert_eq!(request.payload, b"paxeer network");
    let local = attestor(&scratch, 1)?.attest(&request)?;
    let peer = attestor(&scratch, 2)?.attest(&request)?;
    assert_eq!(local.response, b"[]");
    assert_eq!(local.digest, peer.digest);
    assert_eq!(local.callback_gas, 0);
    assert_eq!(local.timeout_height, u64::MAX);
    assert_eq!(local.signer, signer_address(&signing_key(1)?));
    let set = roster(&[&local, &peer]);
    let signatures = [
        (local.signer, local.signature),
        (peer.signer, peer.signature),
    ]
    .into_iter()
    .collect();
    let ready = Ready::from_signatures(&local, &set, &signatures).ok_or("not ready")?;
    assert_eq!(ready.signatures.len(), 2);

    let expected = format!(
        "02\
         0000000000000000000000000000000000000000000000000000000000000009\
         a0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebf\
         0102030405060708\
         02{}{}00000002000000025b5d02{}{}",
        hex(&keccak(b"paxeer network")),
        hex(&ready.content_digest),
        hex(&ready.signatures[0]),
        hex(&ready.signatures[1]),
    );
    let observation = observation_bytes(NETWORK, &request, &ready)?;
    assert_eq!(hex(&observation), expected);
    assert_eq!(observation.len(), 146 + 2 + 1 + 2 * 65);
    assert_eq!(ready.signers, set.signers);
    for (signer, signature) in ready.signers.iter().zip(&ready.signatures) {
        assert_eq!(recover_signer(&ready.digest, signature)?, *signer);
    }
    Ok(())
}

#[test]
fn the_program_exchange_collects_peer_signatures_by_program_and_request() -> Checked {
    let scratch = Scratch::new("exchange")?;
    let recorded = recorded_request(include_str!("fixtures/kernel/watch.json"))?;
    let request = ProgramRequest {
        request_id: 4,
        ..recorded
    };
    let next_request = ProgramRequest {
        request_id: 5,
        ..request.clone()
    };
    let mut other_program = request.clone();
    other_program.program_id[0] ^= 1;
    let local_attestor = attestor(&scratch, 1)?;
    let peer_attestor = attestor(&scratch, 2)?;
    let unknown_attestor = attestor(&scratch, 3)?;
    let local = local_attestor.attest(&request)?;
    let peer = peer_attestor.attest(&request)?;
    let unknown = unknown_attestor.attest(&request)?;
    let set = roster(&[&local, &peer]);
    let peer_exchange = Arc::new(ProgramExchange::open(&scratch.0.join("peer"), &[])?);
    let unknown_exchange = Arc::new(ProgramExchange::open(&scratch.0.join("unknown"), &[])?);
    peer_exchange.record(peer.clone());
    unknown_exchange.record(unknown.clone());
    let peer_server = serve_exchange(&peer_exchange)?;
    let unknown_server = serve_exchange(&unknown_exchange)?;
    let exchange = ProgramExchange::open(
        &scratch.0.join("local"),
        &[
            format!("http://{}", peer_server.local_addr()),
            format!("http://{}", unknown_server.local_addr()),
        ],
    )?;
    let key = (request.program_id, request.request_id);
    let next_key = (next_request.program_id, next_request.request_id);
    let other_key = (other_program.program_id, other_program.request_id);
    exchange.record(local.clone());
    exchange.record(peer.clone());
    assert_eq!(
        exchange.answer(key).ok_or("no answer")?.record(),
        local.record()
    );
    assert!(exchange.ready(key, &set).is_none());
    assert_eq!(exchange.pending(), vec![key]);
    assert_eq!(
        exchange.accept("peer", next_key, &program_record(&peer), &set),
        Ok(None)
    );
    exchange.record(local_attestor.attest(&next_request)?);
    exchange.record(local_attestor.attest(&other_program)?);
    let next_peer = peer_attestor.attest(&next_request)?;
    let other_peer = peer_attestor.attest(&other_program)?;
    assert_eq!(
        exchange.accept("peer", key, &program_record(&next_peer), &set),
        Err(Discard::WrongRequest)
    );
    assert_eq!(
        exchange.accept("peer", key, &program_record(&other_peer), &set),
        Err(Discard::WrongRequest)
    );
    assert_eq!(
        exchange.accept("unknown", key, &program_record(&unknown), &set),
        Err(Discard::UnknownSigner)
    );
    assert!(exchange.ready(key, &set).is_none());
    assert_eq!(exchange.collect(key, &set), 2);
    let ready = exchange.ready(key, &set).ok_or("threshold not reached")?;
    assert_eq!(ready.signers, set.signers);
    assert_eq!(ready.signatures.len(), 2);
    assert_eq!(ready.digest, local.digest);
    assert_eq!(ready.response, local.response);
    for (signer, signature) in ready.signers.iter().zip(&ready.signatures) {
        assert_eq!(recover_signer(&ready.digest, signature)?, *signer);
    }
    assert!(exchange.ready(next_key, &set).is_none());
    assert!(exchange.ready(other_key, &set).is_none());
    assert_eq!(
        exchange.accept("peer", key, &program_record(&peer), &set),
        Ok(Some(peer.signer))
    );
    assert_eq!(
        exchange
            .ready(key, &set)
            .ok_or("not ready")?
            .signatures
            .len(),
        2
    );
    assert_eq!(
        exchange.accept("peer", next_key, &program_record(&next_peer), &set),
        Ok(Some(peer.signer))
    );
    assert_eq!(
        exchange
            .ready(next_key, &set)
            .ok_or("request 5 not ready")?
            .digest,
        next_peer.digest
    );
    assert_ne!(next_peer.digest, ready.digest);
    assert!(exchange.ready(other_key, &set).is_none());
    assert_eq!(
        exchange.accept("peer", other_key, &program_record(&other_peer), &set),
        Ok(Some(peer.signer))
    );
    let other_ready = exchange
        .ready(other_key, &set)
        .ok_or("other program not ready")?;
    assert_eq!(other_ready.signers, set.signers);
    assert_eq!(other_ready.signatures.len(), 2);
    assert_eq!(other_ready.digest, other_peer.digest);
    assert_ne!(other_ready.digest, ready.digest);
    assert_eq!(exchange.ready(key, &set), Some(ready));
    let discarded = exchange.discarded();
    assert_eq!(discarded.len(), 4);
    assert!(discarded
        .iter()
        .all(|entry| (entry.program_id, entry.request_id) == key));
    let log = std::fs::read_to_string(exchange.log_path())?;
    assert!(log.contains("wrong_request"));
    assert!(log.contains("unknown_signer"));
    peer_server.shutdown()?;
    unknown_server.shutdown()?;
    Ok(())
}
