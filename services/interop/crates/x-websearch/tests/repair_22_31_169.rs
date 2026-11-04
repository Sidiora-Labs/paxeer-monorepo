use std::collections::BTreeMap;
use std::error::Error;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Barrier};
use std::time::Duration;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use k256::ecdsa::SigningKey;
use serde_json::{json, Value};
use x_websearch::attest::{signer_address, AttestorSet, Ready};
use x_websearch::config::FetchLimits;
use x_websearch::content::ContentStore;
use x_websearch::fetch::Fetcher;
use x_websearch::index::WebIndex;
use x_websearch::payment::{decode_payment_exact, hex, PaymentGate, EXACT};
use x_websearch::program::{
    observation_bytes, register, ProgramAttestor, ProgramExchange, ProgramRelay, ProgramRequest,
};
use x_websearch::settlement::SettlementRpc;
use x_websearch::submit::{decode_program_observation, Fees, Submitter};
use x_websearch::watch::{unhex0x, EvmRpc, RequestWatcher};
use x_websearch::{Config, Limits, Response, Route, RouteTable, RunningServer, Server};

type Checked<T = ()> = Result<T, Box<dyn Error>>;
const CHAIN: u64 = 125;
const NETWORK: u32 = 125;
const PROGRAM: [u8; 32] = [0x91; 32];

struct Scratch(PathBuf);
impl Scratch {
    fn new(label: &str) -> Checked<Self> {
        let directory =
            std::env::temp_dir().join(format!("repair-22-31-169-{}-{label}", std::process::id()));
        std::fs::create_dir(&directory)?;
        Ok(Self(directory))
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
fn rpc_unavailable() -> Checked<EvmRpc> {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let rpc = EvmRpc::new(&format!("http://{}", listener.local_addr()?))?;
    drop(listener);
    Ok(rpc)
}
fn request(id: u64) -> ProgramRequest {
    ProgramRequest {
        program_id: PROGRAM,
        request_id: id,
        kind: 2,
        payload: b"paxeer".to_vec(),
        sequence: id,
    }
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
            connect_timeout_ms: 3000,
            total_timeout_ms: 10000,
            max_body_bytes: 2_097_152,
            max_redirects: 3,
            allow_loopback: true,
        })?),
        Arc::clone(index),
        Arc::clone(store),
    ))
}
fn set() -> Checked<AttestorSet> {
    let mut signers = vec![signer_address(&key(1)?), signer_address(&key(2)?)];
    signers.sort_unstable();
    Ok(AttestorSet {
        signers,
        threshold: 2,
    })
}
fn serve(exchange: &Arc<ProgramExchange>) -> Checked<RunningServer> {
    let mut routes = RouteTable::new();
    register(&mut routes, exchange)?;
    Ok(Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?)
}
fn get(address: SocketAddr, path: &str, headers: &str) -> Checked<(u16, Value)> {
    let mut stream = TcpStream::connect(address)?;
    stream.set_read_timeout(Some(Duration::from_secs(10)))?;
    write!(
        stream,
        "GET {path} HTTP/1.1\r\nHost: localhost\r\n{headers}\r\n"
    )?;
    let mut bytes = Vec::new();
    stream.read_to_end(&mut bytes)?;
    let split = bytes
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .ok_or("HTTP head")?;
    let status = std::str::from_utf8(&bytes[..split])?
        .split_whitespace()
        .nth(1)
        .ok_or("HTTP status")?
        .parse()?;
    Ok((status, serde_json::from_slice(&bytes[split + 4..])?))
}

#[test]
fn program_answer_recovery_preserves_emitted_bytes_and_quorum() -> Checked {
    let scratch = Scratch::new("program")?;
    let index = Arc::new(WebIndex::open(&scratch.0.join("index"))?);
    index.put(
        "https://paxeer.app/",
        "Paxeer original answer",
        "paxeer original content",
    )?;
    index.commit()?;
    let store = Arc::new(ContentStore::open(&scratch.0.join("content"), &[])?);
    let request = request(7);
    let program_key = (request.program_id, request.request_id);
    let peer = Arc::new(ProgramExchange::open(&scratch.0.join("peer"), &[])?);
    peer.bind_chain(CHAIN, NETWORK, signer_address(&key(2)?))?;
    peer.record(attestor(2, &index, &store)?.attest(&request)?)?;
    let peer_server = serve(&peer)?;
    let peer_url = format!("http://{}", peer_server.local_addr());
    let exchange = Arc::new(ProgramExchange::open(&scratch.0.join("local"), &[])?);
    exchange.bind_chain(CHAIN, NETWORK, signer_address(&key(1)?))?;
    let mut relay = ProgramRelay {
        watcher: RequestWatcher::open(rpc_unavailable()?, 2, &scratch.0.join("watch"), None)?
            .with_chain_id(CHAIN)?,
        attestor: attestor(1, &index, &store)?,
        exchange,
        set: set()?,
        submitter: Submitter::open(
            rpc_unavailable()?,
            key(4)?,
            CHAIN,
            &scratch.0.join("submit"),
        )?,
    };
    assert!(relay.prepare_program(&request)?.is_none());
    let original = relay
        .exchange
        .answer(program_key)
        .ok_or("missing local answer")?;
    let local_server = serve(&relay.exchange)?;
    let path = format!("/program-attestations/{}/7", hex(&PROGRAM));
    let (status, emitted) = get(local_server.local_addr(), &path, "")?;
    assert_eq!(status, 200);
    assert_eq!(
        emitted["digest"],
        x_websearch::watch::hex0x(&original.digest)
    );
    local_server.shutdown()?;
    drop(relay);
    index.put(
        "https://paxeer.app/",
        "Paxeer changed answer",
        "paxeer changed content after restart",
    )?;
    index.commit()?;
    assert_ne!(
        attestor(1, &index, &store)?.attest(&request)?.digest,
        original.digest
    );
    let exchange = Arc::new(ProgramExchange::open(
        &scratch.0.join("local"),
        &[peer_url],
    )?);
    exchange.bind_chain(CHAIN, NETWORK, signer_address(&key(1)?))?;
    assert!(exchange
        .bind_chain(CHAIN + 1, NETWORK, signer_address(&key(1)?))
        .is_err());
    assert!(exchange
        .bind_chain(CHAIN, NETWORK + 1, signer_address(&key(1)?))
        .is_err());
    assert!(exchange
        .bind_chain(CHAIN, NETWORK, signer_address(&key(2)?))
        .is_err());
    relay = ProgramRelay {
        watcher: RequestWatcher::open(rpc_unavailable()?, 2, &scratch.0.join("watch"), None)?
            .with_chain_id(CHAIN)?,
        attestor: attestor(1, &index, &store)?,
        exchange,
        set: set()?,
        submitter: Submitter::open(
            rpc_unavailable()?,
            key(4)?,
            CHAIN,
            &scratch.0.join("submit"),
        )?,
    };
    let mut changed_request = request.clone();
    changed_request.payload = b"changed request binding".to_vec();
    assert!(relay.prepare_program(&changed_request).is_err());
    let ready = relay
        .prepare_program(&request)?
        .ok_or("quorum not recovered")?;
    assert_eq!(ready.digest, original.digest);
    assert_eq!(ready.response, original.response);
    assert_eq!(ready.signatures.len(), 2);
    let observation = observation_bytes(NETWORK, &request, &ready)?;
    assert_eq!(
        decode_program_observation(program_key, &observation, 2)?.response,
        original.response
    );
    let local_server = serve(&relay.exchange)?;
    assert_eq!(get(local_server.local_addr(), &path, "")?.1, emitted);
    local_server.shutdown()?;
    peer_server.shutdown()?;
    drop(peer);
    drop(relay);
    let recovered = ProgramExchange::open(&scratch.0.join("local"), &[])?;
    recovered.bind_chain(CHAIN, NETWORK, signer_address(&key(1)?))?;
    let ready_after_quorum = recovered
        .ready(program_key, &set()?)
        .ok_or("persisted quorum missing")?;
    assert_eq!(
        observation_bytes(NETWORK, &request, &ready_after_quorum)?,
        observation
    );
    assert!(recovered
        .record(attestor(1, &index, &store)?.attest(&request)?)
        .is_err());
    recovered.forget(program_key)?;
    drop(recovered);
    let completed = ProgramExchange::open(&scratch.0.join("local"), &[])?;
    assert!(completed.answer(program_key).is_none());
    assert!(completed.pending().is_empty());
    assert!(completed.record(original).is_err());
    // Corruption of signed content must stop recovery rather than reattest.
    drop(completed);
    let state = scratch.0.join("local/answers.json");
    let mut value: Value = serde_json::from_slice(&std::fs::read(&state)?)?;
    value["answers"][0]["held"]["response"] = json!(hex(b"corrupt"));
    std::fs::write(state, serde_json::to_vec(&value)?)?;
    assert!(ProgramExchange::open(&scratch.0.join("local"), &[]).is_err());
    Ok(())
}

fn fulfilments(scratch: &Scratch) -> Checked<(Ready, Vec<u8>)> {
    let index = Arc::new(WebIndex::open(&scratch.0.join("index"))?);
    index.put("https://paxeer.app/", "Paxeer", "paxeer fulfilment")?;
    index.commit()?;
    let store = Arc::new(ContentStore::open(&scratch.0.join("content"), &[])?);
    let request = request(17);
    let first = attestor(1, &index, &store)?.attest(&request)?;
    let second = attestor(2, &index, &store)?.attest(&request)?;
    let signatures = BTreeMap::from([
        (first.signer, first.signature),
        (second.signer, second.signature),
    ]);
    let ready = Ready::from_signatures(&first, &set()?, &signatures).ok_or("quorum")?;
    Ok((ready.clone(), observation_bytes(NETWORK, &request, &ready)?))
}
const FEES: Fees = Fees {
    max_fee_per_gas: 10,
    max_priority_fee_per_gas: 1,
};

#[test]
fn fulfilment_nonces_are_shared_across_kinds_and_reopen_before_broadcast() -> Checked {
    let scratch = Scratch::new("nonce")?;
    let (ordinary, observation) = fulfilments(&scratch)?;
    let normal = Submitter::open(
        rpc_unavailable()?,
        key(4)?,
        CHAIN,
        &scratch.0.join("submit"),
    )?;
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let program_rpc = EvmRpc::new(&format!("http://{}", listener.local_addr()?))?;
    let program = Submitter::open(
        program_rpc,
        key(4)?,
        CHAIN,
        &scratch.0.join("program-submit"),
    )?;
    let barrier = Arc::new(Barrier::new(2));
    let (web, origin2) = std::thread::scope(|scope| {
        let waiter = Arc::clone(&barrier);
        let normal_ref = &normal;
        let ordinary_ref = &ordinary;
        let web = scope.spawn(move || {
            waiter.wait();
            normal_ref.stage(ordinary_ref, 7, FEES)
        });
        let origin2 = scope.spawn(|| {
            barrier.wait();
            program.stage_program((PROGRAM, 17), &observation, 2, 7, FEES)
        });
        (
            web.join().expect("web worker"),
            origin2.join().expect("program worker"),
        )
    });
    let web = web?;
    let origin2 = origin2?;
    let web_entry = normal.journal().load(17)?.ok_or("web journal")?;
    let program_entry = program
        .journal()
        .load((PROGRAM, 17))?
        .ok_or("program journal")?;
    let mut nonces = vec![
        web_entry.transaction.as_ref().ok_or("web tx")?.0,
        program_entry.transaction.as_ref().ok_or("program tx")?.0,
    ];
    nonces.sort_unstable();
    assert_eq!(nonces, vec![7, 8]);
    assert_ne!(web.raw, origin2.raw);
    // Receive the actual production RPC send, then lose the response. No
    // fabricated receipt, status or nonce response is supplied by this fault.
    let receiver = std::thread::spawn(move || -> Result<Value, String> {
        let (mut stream, _) = listener.accept().map_err(|e| e.to_string())?;
        stream
            .set_read_timeout(Some(Duration::from_secs(10)))
            .map_err(|e| e.to_string())?;
        let mut reader = BufReader::new(&mut stream);
        let mut length = None;
        loop {
            let mut line = String::new();
            reader.read_line(&mut line).map_err(|e| e.to_string())?;
            if line == "\r\n" {
                break;
            }
            if let Some((name, value)) = line.split_once(':') {
                if name.eq_ignore_ascii_case("content-length") {
                    length = Some(value.trim().parse::<usize>().map_err(|e| e.to_string())?);
                }
            }
        }
        let mut body = vec![0; length.ok_or("missing content length")?];
        reader.read_exact(&mut body).map_err(|e| e.to_string())?;
        serde_json::from_slice(&body).map_err(|e| e.to_string())
    });
    assert!(program.broadcast_program((PROGRAM, 17)).is_err());
    let sent = receiver.join().map_err(|_| "receiver panicked")??;
    assert_eq!(sent["method"], "eth_sendRawTransaction");
    assert_eq!(sent["params"][0], x_websearch::watch::hex0x(&origin2.raw));
    drop(normal);
    drop(program);
    let normal = Submitter::open(
        rpc_unavailable()?,
        key(4)?,
        CHAIN,
        &scratch.0.join("submit"),
    )?;
    let mut program = Submitter::open(
        rpc_unavailable()?,
        key(4)?,
        CHAIN,
        &scratch.0.join("program-submit"),
    )?;
    assert_eq!(normal.stage(&ordinary, 7, FEES)?, web);
    assert_eq!(
        program.stage_program((PROGRAM, 17), &observation, 2, 7, FEES)?,
        origin2
    );
    // Actual failed transports leave the signed allocations owned, byte for byte.
    assert!(normal.submit(&ordinary).is_err());
    assert!(program
        .submit_program((PROGRAM, 17), &observation, 2)
        .is_err());
    assert_eq!(normal.journal().load(17)?, Some(web_entry));
    assert_eq!(program.journal().load((PROGRAM, 17))?, Some(program_entry));
    let mut next = ordinary.clone();
    next.request_id = 18;
    normal.stage(&next, 7, FEES)?;
    assert_eq!(
        normal
            .journal()
            .load(18)?
            .ok_or("next journal")?
            .transaction
            .ok_or("next tx")?
            .0,
        9
    );
    assert!(Submitter::open(
        rpc_unavailable()?,
        key(5)?,
        CHAIN,
        &scratch.0.join("submit")
    )
    .is_err());
    assert!(Submitter::open(
        rpc_unavailable()?,
        key(4)?,
        CHAIN + 1,
        &scratch.0.join("submit")
    )
    .is_err());
    Ok(())
}

#[test]
fn fulfilment_nonces_recover_a_reservation_before_journal_publication() -> Checked {
    let scratch = Scratch::new("reservation")?;
    let (ready, observation) = fulfilments(&scratch)?;
    let directory = scratch.0.join("submit");
    let normal = Submitter::open(rpc_unavailable()?, key(4)?, CHAIN, &directory)?;
    let obstruction = directory.join("17.tmp");
    std::fs::create_dir(&obstruction)?;
    assert!(normal.stage(&ready, 7, FEES).is_err());
    let coordinator = scratch
        .0
        .join("fulfilment-nonces")
        .join(format!("{CHAIN}-{}", hex(&normal.address())))
        .join("state.json");
    let state: Value = serde_json::from_slice(&std::fs::read(coordinator)?)?;
    let reserved_raw = state["reservation"]["entry"]["raw"]
        .as_str()
        .ok_or("reservation not durable")?
        .to_owned();
    assert_eq!(state["next"], 8);
    drop(normal);
    std::fs::remove_dir(obstruction)?;
    let program = Submitter::open(
        rpc_unavailable()?,
        key(4)?,
        CHAIN,
        &scratch.0.join("program-submit"),
    )?;
    program.stage_program((PROGRAM, 17), &observation, 2, 7, FEES)?;
    assert_eq!(
        program
            .journal()
            .load((PROGRAM, 17))?
            .ok_or("program")?
            .transaction
            .ok_or("transaction")?
            .0,
        8
    );
    let normal = Submitter::open(rpc_unavailable()?, key(4)?, CHAIN, &directory)?;
    let recovered = normal.stage(&ready, 7, FEES)?;
    assert_eq!(x_websearch::watch::hex0x(&recovered.raw), reserved_raw);
    assert_eq!(
        normal
            .journal()
            .load(17)?
            .ok_or("web")?
            .transaction
            .ok_or("transaction")?
            .0,
        7
    );
    Ok(())
}

// Replays only committed request/response pairs. It does not implement ledger
// semantics, sign receipts, or replace any websearch payment component.
struct LedgerTranscript {
    address: SocketAddr,
    stop: Arc<AtomicBool>,
    worker: Option<std::thread::JoinHandle<Result<Vec<Value>, String>>>,
}
impl LedgerTranscript {
    fn start(recording: &Value) -> Checked<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let exchanges = recording["exchanges"]
            .as_array()
            .ok_or("exchanges")?
            .clone();
        let worker = std::thread::spawn(move || {
            let mut requests = Vec::new();
            loop {
                let (mut stream, _) = listener.accept().map_err(|e| e.to_string())?;
                if stopping.load(Ordering::Acquire) {
                    return Ok(requests);
                }
                let request = read_rpc(&mut stream).map_err(|e| e.to_string())?;
                let exchange = exchanges
                    .iter()
                    .find(|exchange| {
                        exchange["method"] == request["method"]
                            && exchange["params"] == request["params"]
                    })
                    .ok_or_else(|| format!("unrecorded ledger request: {request}"))?;
                let body = json!({"jsonrpc":"2.0", "id":request["id"],
                    "result":exchange["result"]})
                .to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len())
                    .map_err(|e| e.to_string())?;
                requests.push(request);
            }
        });
        Ok(Self {
            address,
            stop,
            worker: Some(worker),
        })
    }
    fn finish(mut self) -> Checked<Vec<Value>> {
        self.stop.store(true, Ordering::Release);
        TcpStream::connect(self.address)?;
        self.worker
            .take()
            .ok_or("transcript worker")?
            .join()
            .map_err(|_| "transcript worker panicked")?
            .map_err(Into::into)
    }
}
impl Drop for LedgerTranscript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}
fn read_rpc(stream: &mut TcpStream) -> std::io::Result<Value> {
    stream.set_read_timeout(Some(Duration::from_secs(10)))?;
    let mut reader = BufReader::new(stream);
    let mut length = None;
    loop {
        let mut line = String::new();
        if reader.read_line(&mut line)? == 0 {
            return Err(std::io::ErrorKind::UnexpectedEof.into());
        }
        if line == "\r\n" {
            break;
        }
        if let Some((name, value)) = line.split_once(':') {
            if name.eq_ignore_ascii_case("content-length") {
                length = Some(
                    value
                        .trim()
                        .parse::<usize>()
                        .map_err(std::io::Error::other)?,
                );
            }
        }
    }
    let mut bytes = vec![0; length.ok_or(std::io::ErrorKind::InvalidData)?];
    reader.read_exact(&mut bytes)?;
    serde_json::from_slice(&bytes).map_err(std::io::Error::other)
}

#[test]
fn metered_retired_offers_execute_exact_on_the_application_ledger() -> Checked {
    let scratch = Scratch::new("ledger")?;
    let produced: Value =
        serde_json::from_str(include_str!("../src/repair_22_31_169_ledger.json"))?;
    let transcript = LedgerTranscript::start(&produced)?;
    let endpoint = format!("http://{}", transcript.address);
    let config = Config::parse(&json!({
        "listen":"127.0.0.1:8482", "data_dir":scratch.0.join("data"),"seeds":["https://paxeer.app/"],
        "crawl":{"pages_per_cycle":10,"pages_per_host":10,"max_depth":1,"politeness_delay_ms":1000},
        "fetch":{"connect_timeout_ms":3000,"total_timeout_ms":10000,"max_body_bytes":2097152,"max_redirects":3,"allow_loopback":true},
        "assets":produced["assets"],"gateway":{"endpoint":endpoint,"receipt_keys":[produced["receipt_key"]]},
        "evm":{"endpoint":endpoint,"chain_id":CHAIN,"confirmations":12},"kernel_network_id":NETWORK,"peers":[],"payment":{}
    }).to_string())?;
    let gate = Arc::new(PaymentGate::new(
        &config,
        &key(3)?,
        x_websearch::conformance_suite()?,
        || 1_000_000_000_000,
    )?);
    let mut routes = RouteTable::new();
    PaymentGate::install(&gate, &mut routes, Route::Search, |_| {
        Response::json(200, br#"{"paid":true}"#.to_vec())
    })?;
    let server = Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?;
    let headers = format!(
        "LAYERX-PAYER-ADDRESS: {}\r\n",
        produced["payer"].as_str().ok_or("payer")?
    );
    let target = "/search?q=paxeer";
    let (status, challenge) = get(server.local_addr(), target, &headers)?;
    assert_eq!(status, 402);
    let accepts = challenge["paymentRequired"]["accepts"]
        .as_array()
        .ok_or("offers")?;
    assert_eq!(accepts.len(), 4);
    // Naming a payer must not advertise the retired grant scheme.
    assert_eq!(
        get(server.local_addr(), target, "")?.1["paymentRequired"]["accepts"],
        json!(accepts)
    );
    let rpc = SettlementRpc::new(&endpoint, CHAIN)?;
    for offer in accepts {
        assert_eq!(offer["scheme"], EXACT);
        let currency = offer["extra"]["layerx"]["currency"]
            .as_str()
            .ok_or("currency")?;
        let recorded = &produced["payments"][currency];
        let hash = unhex0x(recorded["txHash"].as_str().ok_or("txHash")?)
            .ok_or("hash hex")?
            .try_into()
            .map_err(|_| "hash length")?;
        let receipt = rpc.sequencer_receipt(&hash)?.ok_or("recorded receipt")?;
        assert_eq!(receipt.status, Some(1));
        let payments = receipt
            .logs
            .iter()
            .filter_map(decode_payment_exact)
            .collect::<Vec<_>>();
        assert_eq!(payments.len(), 1);
        assert_eq!(
            hex(&payments[0].payee),
            offer["payTo"].as_str().ok_or("payTo")?
        );
        assert_eq!(
            hex(&payments[0].asset),
            offer["asset"].as_str().ok_or("asset")?
        );
        assert_eq!(
            payments[0].amount.to_string(),
            offer["amount"].as_str().ok_or("amount")?
        );
        assert_eq!(hex(&payments[0].reference), recorded["idempotencyKey"]);
        // Feed the recorded receipt back through the actual seller. Its
        // signature, log commitments, amount and payee must all verify before
        // the handler releases the paid response.
        let mut payload = recorded.clone();
        payload
            .as_object_mut()
            .ok_or("payment object")?
            .remove("rawTransaction");
        let signature = STANDARD
            .encode(json!({"x402Version":2,"accepted":offer,"payload":payload}).to_string());
        let (status, body) = get(
            server.local_addr(),
            target,
            &format!("PAYMENT-SIGNATURE: {signature}\r\n"),
        )?;
        assert_eq!((status, body), (200, json!({"paid":true})), "{currency}");
    }
    server.shutdown()?;
    let requests = transcript.finish()?;
    assert_eq!(
        requests
            .iter()
            .filter(|r| r["method"] == "pax_sequencerReceipt")
            .count(),
        8
    );
    assert!(requests.iter().all(|r| r["method"] == "eth_chainId"
        || r["method"] == "eth_call"
        || r["method"] == "pax_sequencerReceipt"));
    Ok(())
}
