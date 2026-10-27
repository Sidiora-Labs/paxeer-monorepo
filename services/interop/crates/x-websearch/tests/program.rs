use std::collections::VecDeque;
use std::error::Error;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use k256::ecdsa::SigningKey;
use serde_json::{json, Value};
use x_websearch::attest::{recover_signer, signer_address, Answer, AttestorSet, Discard, Ready};
use x_websearch::config::FetchLimits;
use x_websearch::content::ContentStore;
use x_websearch::fetch::Fetcher;
use x_websearch::index::WebIndex;
use x_websearch::kernel::decode_request_record;
use x_websearch::payment::{hex, unhex};
use x_websearch::program::{
    observation_bytes, program_record, register, ProgramAttestor, ProgramExchange, ProgramRelay,
    ProgramRequest, Step,
};
use x_websearch::submit::{
    fulfil_program_calldata, fulfil_program_gas_limit, Fees, Journal, JournalState, Outcome,
    Submitter, Transaction,
};
use x_websearch::watch::{
    decode_program_requested, hex0x, keccak, program_requested_topic, requested_topic, EvmRpc,
    RequestWatcher, XWEB_PRECOMPILE,
};
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

const CHAIN_ID: u64 = 713_714;

type RpcScript = Arc<Mutex<VecDeque<(Value, Value)>>>;

struct FixtureServer {
    address: SocketAddr,
    thread: Option<thread::JoinHandle<Result<(), String>>>,
}

fn read_http(stream: &mut TcpStream) -> Checked<(String, Vec<u8>)> {
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    let mut bytes = Vec::new();
    let mut chunk = [0; 4096];
    loop {
        if let Some(end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let head = std::str::from_utf8(&bytes[..end])?;
            let length = head
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse::<usize>().ok())?
                })
                .unwrap_or(0);
            if bytes.len() >= end + 4 + length {
                return Ok((head.to_owned(), bytes[end + 4..end + 4 + length].to_vec()));
            }
        }
        let read = stream.read(&mut chunk)?;
        if read == 0 {
            return Err("closed HTTP request".into());
        }
        bytes.extend_from_slice(&chunk[..read]);
    }
}

impl FixtureServer {
    fn start(
        handler: impl Fn(&str, &[u8]) -> Checked<(&'static str, String)> + Send + 'static,
    ) -> Checked<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let handle = thread::spawn(move || {
            let serve = || -> Checked {
                for stream in listener.incoming() {
                    let mut stream = stream?;
                    let (head, body) = read_http(&mut stream)?;
                    if head.starts_with("GET /__stop ") {
                        return Ok(());
                    }
                    let (content_type, answer) = handler(&head, &body)?;
                    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len())?;
                }
                Ok(())
            };
            serve().map_err(|error| error.to_string())
        });
        Ok(Self {
            address,
            thread: Some(handle),
        })
    }

    fn site() -> Checked<Self> {
        Self::start(|head, _| {
            let path = head.split_whitespace().nth(1).ok_or("no HTTP path")?;
            let body = match path {
                "/robots.txt" => include_str!("fixtures/site/robots.txt"),
                "/plain.txt" => include_str!("fixtures/site/plain.txt"),
                _ => return Err(format!("unexpected site fetch: {path}").into()),
            };
            Ok(("text/plain; charset=utf-8", body.to_owned()))
        })
    }

    fn rpc(script: &RpcScript) -> Checked<Self> {
        let script = Arc::clone(script);
        Self::start(move |_, body| {
            let request: Value = serde_json::from_slice(body)?;
            let (expected, answer) = script
                .lock()
                .map_err(|_| "RPC script lock")?
                .pop_front()
                .ok_or("unexpected RPC beyond recording")?;
            let actual = json!({"method": request["method"], "params": request["params"]});
            if actual != expected {
                return Err(
                    format!("RPC differs from recording: {actual}, expected {expected}").into(),
                );
            }
            Ok((
                "application/json",
                json!({"jsonrpc":"2.0", "id":request["id"], "result":answer}).to_string(),
            ))
        })
    }

    fn endpoint(&self) -> String {
        format!("http://{}", self.address)
    }
}

impl Drop for FixtureServer {
    fn drop(&mut self) {
        if let Ok(mut stream) = TcpStream::connect(self.address) {
            let _ = stream.write_all(b"GET /__stop HTTP/1.1\r\n\r\n");
        }
        if let Some(handle) = self.thread.take() {
            let result = handle.join();
            if !thread::panicking() {
                assert!(
                    matches!(&result, Ok(Ok(()))),
                    "fixture server failed: {result:?}"
                );
            }
        }
    }
}

fn rpc_call(method: &str, params: Value, result: Value) -> (Value, Value) {
    (json!({"method":method, "params":params}), result)
}

fn relay_log(url: &str) -> Checked<Value> {
    let recording: Value = serde_json::from_str(include_str!("fixtures/evm/watch.json"))?;
    let mut log = recording["endpoints"]["evm"]["*"][1]["result"][0].clone();
    log["blockHash"] = json!(hex0x(&[0x31; 32]));
    log["topics"] = json!([
        hex0x(&program_requested_topic()),
        hex0x(&[0xa0; 32]),
        format!("0x{:064x}", 7),
    ]);
    let mut data = Vec::new();
    for value in [1, 96, 41, u64::try_from(url.len())?] {
        data.extend_from_slice(&[0; 24]);
        data.extend_from_slice(&value.to_be_bytes());
    }
    data.extend_from_slice(url.as_bytes());
    data.resize(128 + url.len().div_ceil(32) * 32, 0);
    log["data"] = json!(hex0x(&data));
    Ok(log)
}

fn poll_script(log: Value) -> VecDeque<(Value, Value)> {
    VecDeque::from([
        rpc_call("eth_blockNumber", json!([]), json!("0x20")),
        rpc_call("eth_chainId", json!([]), json!("0xae3f2")),
        rpc_call(
            "eth_getLogs",
            json!([{
                "address":hex0x(&XWEB_PRECOMPILE), "fromBlock":"0x19", "toBlock":"0x1e",
                "topics":[[hex0x(&requested_topic()), hex0x(&program_requested_topic())]],
            }]),
            json!([log]),
        ),
    ])
}

fn append_idle_poll(script: &mut VecDeque<(Value, Value)>) {
    script.extend([
        rpc_call("eth_blockNumber", json!([]), json!("0x20")),
        rpc_call("eth_chainId", json!([]), json!("0xae3f2")),
    ]);
}

fn make_relay(
    scratch: &Scratch,
    rpc: &EvmRpc,
    exchange: Arc<ProgramExchange>,
    set: AttestorSet,
) -> Checked<ProgramRelay> {
    Ok(ProgramRelay {
        watcher: RequestWatcher::open(rpc.clone(), 2, &scratch.0.join("watch"), Some(0x19))?
            .with_chain_id(CHAIN_ID)?,
        attestor: attestor(scratch, 1)?,
        exchange,
        set,
        submitter: Submitter::open(
            rpc.clone(),
            signing_key(4)?,
            CHAIN_ID,
            &scratch.0.join("submit"),
        )?
        .with_confirmations(2),
    })
}

#[test]
fn a_program_request_is_attested_exchanged_and_fulfilled() -> Checked {
    for threshold in [1, 2] {
        let scratch = Scratch::new(&format!("relay-{threshold}"))?;
        let site = FixtureServer::site()?;
        let log = relay_log(&format!("{}/plain.txt", site.endpoint()))?;
        let request = decode_program_requested(&log)?;
        let key = (request.program_id, request.request_id);
        let local = attestor(&scratch, 1)?.attest(&request)?;
        let peer = attestor(&scratch, 2)?.attest(&request)?;
        assert_eq!(local.digest, peer.digest);
        let set = if threshold == 1 {
            AttestorSet {
                signers: vec![local.signer],
                threshold,
            }
        } else {
            roster(&[&local, &peer])
        };
        let signatures = [
            (local.signer, local.signature),
            (peer.signer, peer.signature),
        ]
        .into_iter()
        .collect();
        let ready = Ready::from_signatures(&local, &set, &signatures).ok_or("not ready")?;
        let observation = observation_bytes(NETWORK, &request, &ready)?;
        assert_eq!(&observation[33..65], &request.program_id);
        assert_eq!(&observation[65..73], &request.request_id.to_be_bytes());
        let data = fulfil_program_calldata(&observation);
        assert_eq!(&data[..4], &keccak(b"fulfilProgram(bytes)")[..4]);
        assert_eq!(&data[68..68 + observation.len()], observation);
        let signed = Transaction {
            chain_id: CHAIN_ID,
            nonce: 3,
            fees: Fees {
                max_fee_per_gas: 21_000_000_000,
                max_priority_fee_per_gas: 1_000_000_000,
            },
            gas_limit: fulfil_program_gas_limit(&data, ready.signatures.len()),
            to: XWEB_PRECOMPILE,
            data,
        }
        .sign(&signing_key(4)?)?;
        let peer_exchange = Arc::new(ProgramExchange::open(&scratch.0.join("peer"), &[])?);
        let peer_server = serve_exchange(&peer_exchange)?;
        let exchange = Arc::new(ProgramExchange::open(
            &scratch.0.join("exchange"),
            &[format!("http://{}", peer_server.local_addr())],
        )?);
        let mut script = poll_script(log);
        if threshold == 2 {
            append_idle_poll(&mut script);
        }
        let chain: Value = serde_json::from_str(include_str!("fixtures/evm/chain.json"))?;
        script.push_back(rpc_call(
            "eth_getTransactionCount",
            json!([hex0x(&signer_address(&signing_key(4)?)), "pending"]),
            json!("0x3"),
        ));
        for rule in chain["endpoints"]["evm"]["*"]
            .as_array()
            .ok_or("chain rules missing")?[2..]
            .iter()
        {
            script.push_back((
                json!({"method":rule["method"], "params":rule["params"]}),
                rule["result"].clone(),
            ));
        }
        script.push_back(rpc_call(
            "eth_sendRawTransaction",
            json!([hex0x(&signed.raw)]),
            json!(hex0x(&signed.hash)),
        ));
        append_idle_poll(&mut script);
        script.push_back(rpc_call("eth_getTransactionReceipt", json!([hex0x(&signed.hash)]), json!({
            "transactionHash":hex0x(&signed.hash), "status":"0x1", "blockNumber":"0x1e", "blockHash":hex0x(&[0x31;32]),
        })));
        script.push_back(rpc_call("eth_blockNumber", json!([]), json!("0x20")));
        script.push_back(rpc_call(
            "eth_getBlockByNumber",
            json!(["0x1e", false]),
            json!({"hash":hex0x(&[0x31;32])}),
        ));
        append_idle_poll(&mut script);
        let script = Arc::new(Mutex::new(script));
        let node = FixtureServer::rpc(&script)?;
        let rpc = EvmRpc::new(&node.endpoint())?;
        let mut relay = make_relay(&scratch, &rpc, Arc::clone(&exchange), set)?;
        if threshold == 2 {
            assert!(relay.step()?.is_empty());
            assert_eq!(relay.watcher.program_work(), vec![request.clone()]);
            assert_eq!(
                exchange.answer(key).ok_or("local answer missing")?.digest,
                local.digest
            );
            assert!(exchange.ready(key, &relay.set).is_none());
            assert!(Journal::open(&scratch.0.join("submit"))?
                .load(key)?
                .is_none());
            peer_exchange.record(peer);
        }
        assert_eq!(
            relay.step()?,
            vec![Step::Submitted {
                program_id: key.0,
                request_id: key.1,
                outcome: Outcome::Sent { hash: signed.hash }
            }]
        );
        assert_eq!(exchange.ready(key, &relay.set).ok_or("no quorum")?, ready);
        assert_eq!(relay.watcher.program_work(), vec![request]);
        let journal = Journal::open(&scratch.0.join("submit"))?;
        let entry = journal.load(key)?.ok_or("signed journal missing")?;
        assert_eq!(entry.state, JournalState::Signed);
        assert_eq!(
            entry.transaction.ok_or("signed transaction missing")?.1,
            signed
        );
        drop(relay);
        let restored_exchange = Arc::new(ProgramExchange::open(&scratch.0.join("exchange"), &[])?);
        let mut relay = make_relay(&scratch, &rpc, restored_exchange, AttestorSet::default())?;
        assert_eq!(
            relay.step()?,
            vec![Step::Submitted {
                program_id: key.0,
                request_id: key.1,
                outcome: Outcome::Fulfilled
            }]
        );
        assert!(relay.watcher.program_work().is_empty());
        assert!(relay.exchange.pending().is_empty());
        assert_eq!(
            journal.load(key)?.ok_or("completed journal missing")?.state,
            JournalState::Fulfilled
        );
        assert!(relay.step()?.is_empty());
        assert!(script.lock().map_err(|_| "RPC script lock")?.is_empty());
        assert!(
            RequestWatcher::open(rpc, 2, &scratch.0.join("watch"), None)?
                .program_work()
                .is_empty()
        );
        peer_server.shutdown()?;
    }
    Ok(())
}

#[test]
fn a_program_request_refused_by_the_attestor_is_retired_with_its_code() -> Checked {
    let scratch = Scratch::new("relay-refused")?;
    let site = FixtureServer::site()?;
    let log = relay_log(&format!("{}/private/secret.html", site.endpoint()))?;
    let request = decode_program_requested(&log)?;
    let reason = attestor(&scratch, 2)?
        .attest(&request)
        .expect_err("robots must refuse fetch");
    assert_eq!(
        reason,
        x_websearch::attest::AttestError::Fetch(x_websearch::fetch::FetchError::RobotsDisallowed)
    );
    let mut script = poll_script(log);
    append_idle_poll(&mut script);
    let script = Arc::new(Mutex::new(script));
    let node = FixtureServer::rpc(&script)?;
    let rpc = EvmRpc::new(&node.endpoint())?;
    let exchange = Arc::new(ProgramExchange::open(&scratch.0.join("exchange"), &[])?);
    let mut relay = make_relay(
        &scratch,
        &rpc,
        exchange,
        AttestorSet {
            signers: vec![signer_address(&signing_key(1)?)],
            threshold: 1,
        },
    )?;
    assert_eq!(
        relay.step()?,
        vec![Step::Discarded {
            program_id: request.program_id,
            request_id: request.request_id,
            code: "robots_disallowed".to_owned()
        }]
    );
    assert!(relay.watcher.program_work().is_empty());
    assert!(relay.exchange.pending().is_empty());
    let discarded = relay.discarded()?;
    assert_eq!(discarded.len(), 1);
    assert_eq!(discarded[0].program_id, request.program_id);
    assert_eq!(discarded[0].request_id, request.request_id);
    assert_eq!(discarded[0].code, "robots_disallowed");
    let journal = Journal::open(&scratch.0.join("submit"))?;
    assert!(journal
        .load((request.program_id, request.request_id))?
        .is_none());
    drop(relay);
    let exchange = Arc::new(ProgramExchange::open(&scratch.0.join("exchange"), &[])?);
    let mut restored = make_relay(&scratch, &rpc, exchange, AttestorSet::default())?;
    assert_eq!(restored.discarded()?, discarded);
    assert!(restored.watcher.program_work().is_empty());
    assert!(restored.step()?.is_empty());
    assert!(script.lock().map_err(|_| "RPC script lock")?.is_empty());
    assert!(journal
        .load((request.program_id, request.request_id))?
        .is_none());
    Ok(())
}
