use std::error::Error;
use std::io::{Read as _, Write as _};
use std::net::TcpStream;
use std::path::PathBuf;
use std::sync::Arc;
use std::time::Duration;

use k256::ecdsa::SigningKey;
use serde_json::{json, Value};
use x_websearch::attest::recover_signer;
use x_websearch::config::FetchLimits;
use x_websearch::content::ContentStore;
use x_websearch::fetch::Fetcher;
use x_websearch::index::WebIndex;
use x_websearch::payment::hex;
use x_websearch::program::{register, ProgramAttestor, ProgramExchange, ProgramRequest};
use x_websearch::server::RouteError;
use x_websearch::watch::{
    decode_program_requested, hex0x, keccak, program_requested_topic, unhex0x, XWEB_PRECOMPILE,
};
use x_websearch::{Limits, RouteTable, RunningServer, Server};

type Checked<T = ()> = Result<T, Box<dyn Error>>;

struct Scratch(PathBuf);

impl Scratch {
    fn new(name: &str) -> Checked<Self> {
        let path = std::env::temp_dir().join(format!(
            "x-websearch-program-server-{}-{name}",
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

fn program_request() -> Checked<ProgramRequest> {
    let payload = b"paxeer network";
    let mut data = Vec::new();
    for word in [2_u64, 96, 41, u64::try_from(payload.len())?] {
        data.extend_from_slice(&[0; 24]);
        data.extend_from_slice(&word.to_be_bytes());
    }
    data.extend_from_slice(payload);
    data.resize(128 + payload.len().div_ceil(32) * 32, 0);
    Ok(decode_program_requested(&json!({
        "address": hex0x(&XWEB_PRECOMPILE),
        "topics": [
            hex0x(&program_requested_topic()),
            hex0x(&[0xab; 32]),
            format!("0x{:064x}", u64::MAX),
        ],
        "data": hex0x(&data),
        "blockNumber": "0x1c",
        "removed": false,
    }))?)
}

fn attestor(scratch: &Scratch) -> Checked<ProgramAttestor> {
    let index = Arc::new(WebIndex::open(&scratch.0.join("search"))?);
    index.put(
        "https://paxeer.app/",
        "Paxeer network",
        "The Paxeer network serves program web requests.",
    )?;
    index.commit()?;
    Ok(ProgramAttestor::new(
        SigningKey::from_slice(&[1; 32])?,
        9,
        Arc::new(Fetcher::new(FetchLimits {
            connect_timeout_ms: 3_000,
            total_timeout_ms: 10_000,
            max_body_bytes: 2_097_152,
            max_redirects: 3,
            allow_loopback: false,
        })?),
        index,
        Arc::new(ContentStore::open(&scratch.0.join("content"), &[])?),
    ))
}

fn serve(exchange: &Arc<ProgramExchange>) -> Checked<RunningServer> {
    let mut routes = RouteTable::new();
    register(&mut routes, exchange)?;
    assert_eq!(register(&mut routes, exchange), Err(RouteError::AlreadySet));
    Ok(Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?)
}

fn get(server: &RunningServer, program: &str, request_id: u64) -> Checked<(u16, Value)> {
    let mut stream = TcpStream::connect(server.local_addr())?;
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    stream.set_write_timeout(Some(Duration::from_secs(5)))?;
    write!(
        stream,
        "GET /program-attestations/{program}/{request_id} HTTP/1.1\r\nHost: localhost\r\n\r\n"
    )?;
    let mut bytes = Vec::new();
    stream.read_to_end(&mut bytes)?;
    let split = bytes
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .ok_or("missing HTTP response headers")?;
    let head = std::str::from_utf8(&bytes[..split])?;
    assert!(head
        .lines()
        .any(|line| line == "Content-Type: application/json"));
    let status = head
        .split_whitespace()
        .nth(1)
        .ok_or("missing status")?
        .parse()?;
    Ok((status, serde_json::from_slice(&bytes[split + 4..])?))
}

#[test]
fn a_peer_reads_the_attestation_of_an_evm_program_request() -> Checked {
    let scratch = Scratch::new("recorded")?;
    let exchange = Arc::new(ProgramExchange::open(&scratch.0.join("exchange"), &[])?);
    let server = serve(&exchange)?;
    let attestor = attestor(&scratch)?;
    let request = program_request()?;
    assert_eq!(request.request_id, u64::MAX);
    assert_eq!(request.sequence, 41);
    let other_program = ProgramRequest {
        program_id: [0xcd; 32],
        ..request.clone()
    };
    let other_request = ProgramRequest {
        request_id: request.request_id - 1,
        ..request.clone()
    };
    for request in [&request, &other_program, &other_request] {
        let answer = attestor.attest(request)?;
        assert!(!answer.response.is_empty());
        exchange.record(answer.clone());
        let (status, record) = get(&server, &hex(&request.program_id), request.request_id)?;
        assert_eq!(status, 200);
        assert_eq!(
            record,
            json!({
                "program_id": hex0x(&request.program_id),
                "request_id": request.request_id,
                "digest": hex0x(&answer.digest),
                "content_digest": hex0x(&answer.attestation.content_digest),
                "response_hash": hex0x(&keccak(&answer.response)),
                "full_length": answer.response.len(),
                "signer": hex0x(&answer.signer),
                "signature": hex0x(&answer.signature),
            })
        );
        let digest: [u8; 32] = unhex0x(record["digest"].as_str().ok_or("missing digest")?)
            .ok_or("invalid digest")?
            .try_into()
            .map_err(|_| "digest length")?;
        let signature: [u8; 65] = unhex0x(record["signature"].as_str().ok_or("missing signature")?)
            .ok_or("invalid signature")?
            .try_into()
            .map_err(|_| "signature length")?;
        assert_eq!(recover_signer(&digest, &signature)?, attestor.signer());
    }
    server.shutdown()?;
    Ok(())
}

#[test]
fn an_unknown_program_request_answers_not_found() -> Checked {
    let scratch = Scratch::new("unknown")?;
    let exchange = Arc::new(ProgramExchange::open(&scratch.0.join("exchange"), &[])?);
    let server = serve(&exchange)?;
    let request = program_request()?;
    assert_eq!(
        get(&server, &hex(&request.program_id), request.request_id)?,
        (404, json!({"error": "not_found"}))
    );
    exchange.record(attestor(&scratch)?.attest(&request)?);
    for (program, request_id) in [
        ([0xcd; 32], request.request_id),
        (request.program_id, request.request_id - 1),
    ] {
        assert_eq!(
            get(&server, &hex(&program), request_id)?,
            (404, json!({"error": "not_found"}))
        );
    }
    let program = hex(&request.program_id);
    for malformed in [
        String::new(),
        program[..63].to_owned(),
        format!("{program}0"),
        program.to_uppercase(),
        format!("0x{program}"),
        "g".repeat(64),
    ] {
        assert_eq!(
            get(&server, &malformed, request.request_id)?,
            (400, json!({"error": "malformed_program_request"})),
            "program id: {malformed}"
        );
    }
    server.shutdown()?;
    Ok(())
}
