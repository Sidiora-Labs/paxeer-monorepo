mod test_network {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/../test_network.rs"));
}
#[path = "support/ledger_assets.rs"]
mod ledger_assets;

use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use k256::ecdsa::SigningKey;
use serde_json::{json, Value};
use x_websearch::keys::{ATTESTOR_KEY_FILE, RECEIVER_KEY_FILE, SUBMITTER_KEY_FILE};
use x_websearch::payment::{hex, system_clock, PaymentGate, PAYMENT_REQUIRED, PAYMENT_SIGNATURE};
use x_websearch::watch::{hex0x, keccak, unhex0x, XWEB_PRECOMPILE};
use x_websearch::{Config, Request, Route};

type Outcome<T = ()> = Result<T, Box<dyn std::error::Error>>;
const SYMBOLS: [&str; 4] = ["SID", "PAX", "USDC", "USDL"];

#[derive(Clone, Copy)]
enum Record {
    Available,
    Refused,
    Paused,
    Transport,
    Malformed,
}

struct Ledger {
    address: SocketAddr,
    records: Arc<Mutex<[Record; 4]>>,
    reads: Arc<Mutex<Vec<usize>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

fn read_request(stream: &mut TcpStream) -> Outcome<Value> {
    stream.set_read_timeout(Some(Duration::from_secs(5)))?;
    let mut bytes = Vec::new();
    loop {
        if let Some(end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let head = std::str::from_utf8(&bytes[..end])?;
            let length: usize = head
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then_some(value.trim())
                })
                .ok_or("content length")?
                .parse()?;
            if bytes.len() >= end + 4 + length {
                return Ok(serde_json::from_slice(&bytes[end + 4..end + 4 + length])?);
            }
        }
        let mut chunk = [0; 4096];
        let count = stream.read(&mut chunk)?;
        if count == 0 {
            return Err("incomplete HTTP request".into());
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
}

impl Ledger {
    fn start(config: &Config, initial: [Record; 4]) -> Outcome<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let records = Arc::new(Mutex::new(initial));
        let reads = Arc::new(Mutex::new(Vec::new()));
        let stop = Arc::new(AtomicBool::new(false));
        let (served, recorded, stopped) =
            (Arc::clone(&records), Arc::clone(&reads), Arc::clone(&stop));
        let assets = config.assets.clone();
        let recording: Value = serde_json::from_str(include_str!("fixtures/gateway/metered.json"))?;
        let rules = recording["endpoints"]["gateway"]["*"]
            .as_array()
            .ok_or("ledger rules")?
            .clone();
        let worker = thread::spawn(move || {
            for connection in listener.incoming() {
                if stopped.load(Ordering::SeqCst) {
                    break;
                }
                let mut stream = connection.expect("settlement connection");
                let request = read_request(&mut stream).expect("settlement request");
                let params = &request["params"];
                let mut reply = json!({"jsonrpc":"2.0", "id":request["id"]});
                if request["method"] == "eth_call"
                    && ledger_assets::get_asset_answer(params).is_some()
                {
                    let data =
                        unhex0x(params[0]["data"].as_str().expect("calldata")).expect("call bytes");
                    let index = assets
                        .iter()
                        .position(|asset| asset.asset_id == data[4..])
                        .expect("configured asset");
                    recorded.lock().expect("reads").push(index);
                    match served.lock().expect("records")[index] {
                        Record::Transport => continue,
                        Record::Refused => {
                            reply["error"] =
                                json!({"code":3,"message":"execution reverted: LXP_NO_ASSET"})
                        }
                        Record::Malformed => reply["result"] = json!("0x00"),
                        state => {
                            reply["result"] = json!(hex0x(&ledger_assets::asset_record(
                                &data[4..],
                                SYMBOLS[index],
                                matches!(state, Record::Paused)
                            )))
                        }
                    }
                } else if request["method"] == "eth_call"
                    && params[0]["to"] == hex0x(&XWEB_PRECOMPILE)
                {
                    assert_eq!(
                        params[0]["data"],
                        hex0x(&keccak(b"getKernelNetworkId()")[..4])
                    );
                    reply["result"] = json!(format!("0x{:064x}", 1));
                } else if request["method"] == "eth_call"
                    && params[0]["data"].as_str().is_some_and(|data| {
                        data.starts_with(&hex0x(&keccak(b"getUnifiedAccount(address)")[..4]))
                    })
                {
                    let data = unhex0x(params[0]["data"].as_str().expect("address call"))
                        .expect("address bytes");
                    let identity = rules
                        .iter()
                        .find(|rule| {
                            rule["params"][0]["data"]
                                .as_str()
                                .is_some_and(|data| data.starts_with("0xd3600b24"))
                        })
                        .expect("identity rule");
                    let key = unhex0x(identity["result"].as_str().expect("identity record"))
                        .expect("identity bytes");
                    reply["result"] = json!(format!(
                        "0x{}{:064x}{}{}{:064x}",
                        hex(&data[4..]),
                        128,
                        hex(&key[..32]),
                        "00".repeat(32),
                        0
                    ));
                } else if request["method"] == "eth_getBalance" {
                    reply["result"] = json!("0x0");
                } else if request["method"] == "pax_sequencerReceipt" {
                    reply["result"] = Value::Null;
                } else {
                    let rule = rules
                        .iter()
                        .find(|rule| {
                            rule["method"] == request["method"] && rule["params"] == *params
                        })
                        .unwrap_or_else(|| panic!("recorded settlement call: {request}"));
                    reply["result"] = rule["result"].clone();
                }
                let body = reply.to_string();
                write!(
                    stream,
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                )
                .expect("settlement response");
            }
        });
        Ok(Self {
            address,
            records,
            reads,
            stop,
            worker: Some(worker),
        })
    }

    fn set(&self, records: [Record; 4]) {
        *self.records.lock().expect("records") = records;
    }
}

impl Drop for Ledger {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            assert!(worker.join().is_ok());
        }
    }
}

struct Harness {
    gate: PaymentGate,
    ledger: Ledger,
    config: Value,
    root: PathBuf,
}

impl Harness {
    fn new(records: [Record; 4]) -> Outcome<Self> {
        test_network::isolate()?;
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let root = std::env::temp_dir().join(format!(
            "x-websearch-payable-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::create_dir(&root)?;
        let mut config: Value = serde_json::from_str(include_str!("fixtures/binary/config.json"))?;
        let ledger = Ledger::start(&Config::parse(&config.to_string())?, records)?;
        config["data_dir"] = json!(root.join("gate"));
        config["gateway"]["endpoint"] = json!(format!("http://{}", ledger.address));
        config["evm"]["endpoint"] = config["gateway"]["endpoint"].clone();
        config["evm"]["chain_id"] = json!(125);
        let key = SigningKey::from_slice(&[&[0; 31][..], &[3]].concat())?;
        let gate = PaymentGate::new_with_network_id(
            &Config::parse(&config.to_string())?,
            1,
            &key,
            x_websearch::conformance_suite()?,
            system_clock,
        )?;
        Ok(Self {
            gate,
            ledger,
            config,
            root,
        })
    }

    fn request(&self) -> Request {
        Request {
            method: "GET".to_owned(),
            route: Route::Search,
            path: "/search".to_owned(),
            query: Some("q=ledger".to_owned()),
            headers: vec![("Host".to_owned(), "localhost".to_owned())],
            digest: None,
            peer: self.ledger.address,
        }
    }

    fn offers(&self) -> Outcome<Value> {
        let response = self
            .gate
            .settle(&self.request(), &|_| panic!("unpaid content release"));
        assert_eq!(response.status, 402);
        let body: Value = serde_json::from_slice(&response.body)?;
        let header = response
            .headers
            .iter()
            .find(|(name, _)| name == PAYMENT_REQUIRED)
            .ok_or("offer header")?;
        assert_eq!(
            serde_json::from_slice::<Value>(&STANDARD.decode(&header.1)?)?,
            body["paymentRequired"]
        );
        Ok(body["paymentRequired"]["accepts"].clone())
    }

    fn reports(&self) -> Outcome<Value> {
        let reserved = TcpListener::bind("127.0.0.1:0")?;
        let address = reserved.local_addr()?;
        let mut config = self.config.clone();
        config["listen"] = json!(address.to_string());
        config["data_dir"] = json!(self.root.join("sidecar"));
        let path = self.root.join("config.json");
        std::fs::write(&path, config.to_string())?;
        let key = self.root.join("receiver.key");
        std::fs::write(&key, format!("{:064x}", 3))?;
        std::fs::set_permissions(&key, std::fs::Permissions::from_mode(0o600))?;
        let command = || {
            let mut command = Command::new(env!("CARGO_BIN_EXE_x-websearch"));
            command
                .arg("--config")
                .arg(&path)
                .env(RECEIVER_KEY_FILE, &key)
                .env_remove(ATTESTOR_KEY_FILE)
                .env_remove(SUBMITTER_KEY_FILE)
                .stdin(Stdio::null());
            command
        };
        let status = command().arg("receiver-status").output()?;
        assert!(
            status.status.success(),
            "{}",
            String::from_utf8_lossy(&status.stderr)
        );
        let status: Value = serde_json::from_slice(&status.stdout)?;
        drop(reserved);
        let mut sidecar = Sidecar(
            command()
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()?,
        );
        let deadline = Instant::now() + Duration::from_secs(20);
        let readiness = loop {
            if let Ok(mut stream) = TcpStream::connect(address) {
                stream.set_read_timeout(Some(Duration::from_secs(5)))?;
                stream.write_all(b"GET /readyz HTTP/1.1\r\nHost: localhost\r\n\r\n")?;
                let mut bytes = Vec::new();
                stream.read_to_end(&mut bytes)?;
                let end = bytes
                    .windows(4)
                    .position(|part| part == b"\r\n\r\n")
                    .ok_or("readiness HTTP response")?;
                break serde_json::from_slice::<Value>(&bytes[end + 4..])?;
            }
            assert!(sidecar.0.try_wait()?.is_none(), "sidecar remains running");
            assert!(Instant::now() < deadline, "readiness listener starts");
            thread::sleep(Duration::from_millis(20));
        };
        for field in ["payable_assets", "withheld_assets"] {
            assert_eq!(readiness["payment"][field], status[field]);
            assert_eq!(
                readiness["payment"][field],
                serde_json::to_value(self.gate.payment_resolution(None))?[field]
            );
        }
        Ok(readiness)
    }
}

struct Sidecar(Child);
impl Drop for Sidecar {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}
impl Drop for Harness {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.root);
    }
}

#[test]
fn payable_assets_four_records_offer_all_assets() -> Outcome {
    let harness = Harness::new([Record::Available; 4])?;
    harness.gate.resolve_receiver()?;
    let offers = harness.offers()?;
    assert_eq!(offers.as_array().ok_or("offers")?.len(), 4);
    for (offer, symbol) in offers.as_array().ok_or("offers")?.iter().zip(SYMBOLS) {
        assert_eq!(offer["scheme"], "exact");
        assert_eq!(offer["network"], "eip155:125");
        assert_eq!(offer["asset"], harness.config["assets"][symbol]["asset_id"]);
        assert_eq!(offer["amount"], harness.config["assets"][symbol]["price"]);
        assert_eq!(offer["extra"]["layerx"]["currency"], symbol);
    }
    assert!(harness.gate.payment_ready());
    assert_eq!(
        *harness.ledger.reads.lock().expect("reads"),
        vec![0, 1, 2, 3]
    );
    Ok(())
}

#[test]
fn payable_assets_one_refused_record_is_withheld_in_readiness_and_status() -> Outcome {
    let harness = Harness::new([
        Record::Available,
        Record::Available,
        Record::Available,
        Record::Refused,
    ])?;
    harness.gate.resolve_receiver()?;
    let offers = harness.offers()?;
    assert_eq!(offers.as_array().ok_or("offers")?.len(), 3);
    assert!(offers
        .as_array()
        .ok_or("offers")?
        .iter()
        .all(|offer| offer["extra"]["layerx"]["currency"] != "USDL"));
    let readiness = harness.reports()?;
    assert_eq!(readiness["payment"]["state"], "ready");
    assert!(readiness["payment"]["reason"].is_null());
    assert_eq!(
        readiness["payment"]["payable_assets"],
        json!(["SID", "PAX", "USDC"])
    );
    assert_eq!(
        readiness["payment"]["withheld_assets"],
        json!([{"symbol":"USDL","reason":"ledger reports no usable record"}])
    );
    Ok(())
}

#[test]
fn payable_assets_no_records_leave_payment_unresolved() -> Outcome {
    let harness = Harness::new([Record::Refused; 4])?;
    let reason = harness
        .gate
        .resolve_receiver()
        .expect_err("no payable assets");
    assert!(reason.contains("ledger reports no usable record"));
    for symbol in SYMBOLS {
        assert!(reason.contains(symbol));
    }
    assert!(!harness.gate.payment_ready());
    for route in [Route::Search, Route::Fetch] {
        let mut request = harness.request();
        request.route = route;
        if route == Route::Fetch {
            request.path = "/fetch".to_owned();
            request.query = Some("url=https%3A%2F%2Fpaxeer.app%2F".to_owned());
        }
        let response = harness
            .gate
            .settle(&request, &|_| panic!("unavailable content release"));
        assert_eq!(response.status, 503);
        assert_eq!(
            serde_json::from_slice::<Value>(&response.body)?["error"],
            "payment_unavailable"
        );
        assert!(response
            .headers
            .iter()
            .all(|(name, _)| name != PAYMENT_REQUIRED));
    }
    let readiness = harness.reports()?;
    assert_eq!(readiness["payment"]["state"], "unresolved");
    assert_eq!(readiness["payment"]["reason"], reason);
    assert_eq!(readiness["payment"]["payable_assets"], json!([]));
    assert_eq!(
        readiness["payment"]["withheld_assets"]
            .as_array()
            .ok_or("withheld assets")?
            .len(),
        4
    );
    Ok(())
}

#[test]
fn payable_assets_paused_record_is_withheld() -> Outcome {
    let harness = Harness::new([
        Record::Paused,
        Record::Available,
        Record::Available,
        Record::Available,
    ])?;
    harness.gate.resolve_receiver()?;
    let offers = harness.offers()?;
    assert_eq!(offers.as_array().ok_or("offers")?.len(), 3);
    assert!(offers
        .as_array()
        .ok_or("offers")?
        .iter()
        .all(|offer| offer["extra"]["layerx"]["currency"] != "SID"));
    let payment = harness.gate.payment_resolution(None);
    assert_eq!(payment.withheld_assets[0].symbol, "SID");
    assert_eq!(
        payment.withheld_assets[0].reason,
        "ledger reports no usable record: asset is paused"
    );
    Ok(())
}

#[test]
fn payable_assets_refresh_observes_registration_pause_and_release() -> Outcome {
    let harness = Harness::new([Record::Refused; 4])?;
    assert!(harness.gate.resolve_receiver().is_err());
    harness.ledger.set([
        Record::Available,
        Record::Refused,
        Record::Refused,
        Record::Refused,
    ]);
    harness.gate.resolve_receiver()?;
    assert_eq!(harness.offers()?.as_array().ok_or("offers")?.len(), 1);
    harness.ledger.set([Record::Available; 4]);
    harness.gate.resolve_receiver()?;
    assert_eq!(harness.offers()?.as_array().ok_or("offers")?.len(), 4);
    harness.ledger.set([
        Record::Available,
        Record::Paused,
        Record::Available,
        Record::Available,
    ]);
    harness.gate.resolve_receiver()?;
    assert_eq!(harness.offers()?.as_array().ok_or("offers")?.len(), 3);
    harness.ledger.set([Record::Available; 4]);
    harness.gate.resolve_receiver()?;
    assert_eq!(harness.offers()?.as_array().ok_or("offers")?.len(), 4);
    harness.ledger.set([Record::Refused; 4]);
    assert!(harness.gate.resolve_receiver().is_err());
    assert!(!harness.gate.payment_ready());
    assert_eq!(
        *harness.ledger.reads.lock().expect("reads"),
        [0, 1, 2, 3].repeat(6)
    );
    Ok(())
}

#[test]
fn payable_assets_transport_error_preserves_the_previous_resolution() -> Outcome {
    let harness = Harness::new([
        Record::Available,
        Record::Available,
        Record::Available,
        Record::Transport,
    ])?;
    let initial = harness.gate.payment_resolution(None);
    assert!(harness.gate.resolve_receiver().is_err());
    assert_eq!(harness.gate.payment_resolution(None), initial);
    harness.ledger.set([Record::Available; 4]);
    harness.gate.resolve_receiver()?;
    let ready = harness.gate.payment_resolution(None);
    let offers = harness.offers()?;
    harness.ledger.set([
        Record::Refused,
        Record::Paused,
        Record::Available,
        Record::Transport,
    ]);
    assert!(harness.gate.resolve_receiver().is_err());
    assert_eq!(harness.gate.payment_resolution(None), ready);
    assert_eq!(harness.offers()?, offers);
    harness.ledger.set([
        Record::Paused,
        Record::Available,
        Record::Available,
        Record::Available,
    ]);
    harness.gate.resolve_receiver()?;
    let partial = harness.gate.payment_resolution(None);
    harness.ledger.set([
        Record::Available,
        Record::Available,
        Record::Available,
        Record::Transport,
    ]);
    assert!(harness.gate.resolve_receiver().is_err());
    assert_eq!(harness.gate.payment_resolution(None), partial);
    harness.ledger.set([
        Record::Available,
        Record::Available,
        Record::Available,
        Record::Malformed,
    ]);
    assert!(harness.gate.resolve_receiver().is_err());
    assert_eq!(harness.gate.payment_resolution(None), partial);
    Ok(())
}

#[test]
fn payable_assets_payment_in_a_withheld_asset_is_not_offered() -> Outcome {
    let harness = Harness::new([Record::Available; 4])?;
    harness.gate.resolve_receiver()?;
    let offer = harness.offers()?[0].clone();
    harness.ledger.set([
        Record::Refused,
        Record::Available,
        Record::Available,
        Record::Available,
    ]);
    harness.gate.resolve_receiver()?;
    let signature = STANDARD.encode(json!({"x402Version":2, "accepted":offer, "payload":{
        "txHash":hex(&[3;32]), "receiptDigest":hex(&[4;32]), "verificationLevel":"receipted", "idempotencyKey":hex(&[5;32])
    }}).to_string());
    let mut request = harness.request();
    request
        .headers
        .push((PAYMENT_SIGNATURE.to_owned(), signature));
    let response = harness
        .gate
        .settle(&request, &|_| panic!("withheld asset released content"));
    assert_eq!(response.status, 402);
    let body: Value = serde_json::from_slice(&response.body)?;
    assert_eq!(body["error"], "payment_not_offered");
    assert!(body["paymentRequired"]["accepts"]
        .as_array()
        .ok_or("offers")?
        .iter()
        .all(|offered| offered["asset"] != offer["asset"]));
    Ok(())
}
