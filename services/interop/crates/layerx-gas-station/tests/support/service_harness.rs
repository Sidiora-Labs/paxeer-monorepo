use std::collections::BTreeMap;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::mpsc;
use std::sync::{Arc, Mutex, PoisonError};
use std::time::Duration;

use k256::ecdsa::SigningKey;
use k256::elliptic_curve::rand_core::OsRng;
use layerx_gas_station::config::{RegisteredSender, ServiceConfig, StationConfig};
use layerx_gas_station::journal::Journal;
use layerx_gas_station::price::PaymasterRateSource;
use layerx_gas_station::quote::{
    address_word, keccak, quote_digest, word, Address, Quote, SIDIORA,
};
use layerx_gas_station::rpc::{
    bytes, hex, ConfiguredRpc, Exchange, HttpsExchange, RpcFault, SequencerEndpoint,
};
use layerx_gas_station::service::{serve, Limits, Service};
use layerx_gas_station::signer::{LocalSigner, QuoteSigner};
use layerx_gas_station::station::GasStation;
use layerx_gas_station::tx::{
    authorization_digest, batch_digest, encode_sponsored, Authorization, Call,
};
use layerx_gas_station::SignedQuote;
use serde_json::{json, Value};

type TestResult = Result<(), Box<dyn std::error::Error>>;

/// Held by every test that sets or removes `GAS_STATION_DID_GAS_LIMIT` while
/// a station starts, since `GasStation::new` reads it from the process
/// environment.
static DID_GAS_LIMIT_ENV: Mutex<()> = Mutex::new(());

const CHAIN_ID: u64 = 1325;
const PAYMASTER: Address = [0x44; 20];
const AMOUNT: u128 = 3_145_140;
const GAS_COST: &str = "1000000000000000000";
const DID_PRECOMPILE: Address = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x04,
];

/// The ABI answer of `getLayerXDid(address)` for an account bound to the
/// identity `key`: the key, then the string `did:layerx:<key>`.
fn did_binding(key: &[u8; 32]) -> Result<Vec<u8>, std::num::TryFromIntError> {
    let did = format!("did:layerx:{}", &hex(key)[2..]);
    let mut padded = did.into_bytes();
    let length = padded.len();
    padded.resize(length.div_ceil(32) * 32, 0);
    Ok([
        key.as_slice(),
        word(64).as_slice(),
        word(u128::try_from(length)?).as_slice(),
        &padded,
    ]
    .concat())
}

struct Recording {
    fixture: Value,
    phase: Mutex<String>,
    bindings: Mutex<BTreeMap<String, Value>>,
    sent: Mutex<Vec<Vec<u8>>>,
}
impl Recording {
    fn phase(&self, phase: &str) {
        if let Ok(mut current) = self.phase.lock() {
            *current = phase.into();
        }
    }
    fn bind(&self, name: &str, value: Value) {
        if let Ok(mut bindings) = self.bindings.lock() {
            bindings.insert(name.into(), value);
        }
    }
    fn expect_quote_nonce(&self, sponsor: Address, nonce: u128) {
        let consumed = [
            keccak(b"usedQuoteNonces(address,uint256)")[..4].to_vec(),
            address_word(sponsor).to_vec(),
            word(nonce).to_vec(),
        ]
        .concat();
        self.bind("$consumed_call", json!(hex(&consumed)));
    }
    fn sent(&self) -> Vec<Vec<u8>> {
        self.sent
            .lock()
            .map(|sent| sent.clone())
            .unwrap_or_default()
    }
}
fn substitute(value: &Value, bindings: &BTreeMap<String, Value>) -> Value {
    match value {
        Value::String(s) if s.starts_with('$') => bindings
            .get(s)
            .cloned()
            .unwrap_or_else(|| panic!("unbound fixture field")),
        Value::Array(values) => {
            Value::Array(values.iter().map(|v| substitute(v, bindings)).collect())
        }
        Value::Object(values) => Value::Object(
            values
                .iter()
                .map(|(k, v)| (k.clone(), substitute(v, bindings)))
                .collect(),
        ),
        _ => value.clone(),
    }
}

/// The recorded node: every exchange is answered from tests/fixtures/cycle.json,
/// except in the `unreachable` phase, where the request goes to a real HTTPS
/// exchange aimed at a closed loopback port. The `did_unreachable` phase sends
/// only the DID read to that port.
struct ReplayExchange(Arc<Recording>);
impl Exchange for ReplayExchange {
    fn request_until(
        &self,
        endpoint: &str,
        method: &str,
        params: &Value,
        deadline: std::time::Instant,
    ) -> Result<Value, RpcFault> {
        if std::time::Instant::now() >= deadline {
            return Err(RpcFault::Unavailable);
        }
        let phase = self
            .0
            .phase
            .lock()
            .map_err(|_| RpcFault::Unavailable)?
            .clone();
        if phase == "unreachable"
            || (phase == "did_unreachable"
                && method == "eth_call"
                && params[0]["to"] == hex(&DID_PRECOMPILE))
        {
            return HttpsExchange.request_until("https://127.0.0.1:1", method, params, deadline);
        }
        // Recorded exchanges finish synchronously; recovery supplies the same
        // deadline contract as ordinary production transport calls.
        self.request(endpoint, method, params)
    }

    fn request(&self, _endpoint: &str, method: &str, params: &Value) -> Result<Value, RpcFault> {
        let phase = self
            .0
            .phase
            .lock()
            .map_err(|_| RpcFault::Unavailable)?
            .clone();
        if phase == "unreachable"
            || (phase == "did_unreachable"
                && method == "eth_call"
                && params[0]["to"] == hex(&DID_PRECOMPILE))
        {
            return HttpsExchange.request("https://127.0.0.1:1", method, params);
        }
        if method == "eth_sendRawTransaction" {
            let raw = bytes(params[0].as_str().ok_or(RpcFault::Malformed)?)?;
            self.0.bind("$raw", json!(hex(&raw)));
            self.0.bind("$hash", json!(hex(&keccak(&raw))));
            self.0
                .sent
                .lock()
                .map_err(|_| RpcFault::Unavailable)?
                .push(raw);
        }
        let bindings = self
            .0
            .bindings
            .lock()
            .map_err(|_| RpcFault::Unavailable)?
            .clone();
        for name in [phase.as_str(), "*"] {
            let rules = self.0.fixture["phases"][name]
                .as_array()
                .ok_or(RpcFault::Malformed)?;
            for rule in rules {
                if rule["method"] == method && substitute(&rule["params"], &bindings) == *params {
                    if rule["fault"] == "unavailable" {
                        return Err(RpcFault::Unavailable);
                    }
                    if let Some(code) = rule["error"]["code"].as_i64() {
                        return Err(RpcFault::Rejected { code });
                    }
                    return Ok(substitute(&rule["result"], &bindings));
                }
            }
        }
        panic!("unmatched recorded exchange: {method}")
    }
}

/// Recorded exchanges the sequencer endpoint adds to tests/fixtures/cycle.json:
/// the anchored block reads the station makes, the delegated account code, and
/// per phase the send and the `pax_sequencerReceipt` answer. Each phase's rules
/// are matched before the fixture's own.
fn sequencer_endpoint_rules() -> Value {
    let latest = json!({"blockHash": "$latest_block", "requireCanonical": true});
    let finalized = json!({"blockHash": "$finalized_block", "requireCanonical": true});
    let reverted =
        serde_json::from_str::<Value>(include_str!("../fixtures/producer_22_33_71.json"))
            .unwrap_or_else(|error| panic!("committed producer recording: {error:?}"))["service"]
            ["receipt"]
            .clone();
    let executed = |rung: &str, status: &str, sponsor: Value| {
        json!({"rung": rung, "txHash": "$hash", "seq": "0x2a", "height": "0x11", "index": "0x0",
            "timestamp": "0x3e8", "status": status, "returnData": "0x", "logs": [],
            "gasUsed": "0x186a0", "cumulativeGasUsed": "0x186a0",
            "effectiveGasPrice": "0x48c27395000", "feeDenom": "ahpx", "feeAmount": "0x0",
            "sponsor": sponsor, "from": "$sponsor", "to": "$account", "contractAddress": hex(&[0; 20]),
            "nonce": "0x5", "vmError": "", "keyId": hex(&word(1)), "receipt": "0x"})
    };
    let sent = json!({"method": "eth_sendRawTransaction", "params": ["$raw"], "result": "$hash"});
    let receipt = |result: Value| json!({"method": "pax_sequencerReceipt", "params": ["$hash"], "result": result});
    let rate = |updated: &str| {
        json!([{"method": "eth_call",
            "params": [{"to": "$paymaster", "data": "$rate_updated_call"}, "latest"],
            "result": updated}])
    };
    let mut rules = json!({
        "*": [
            {"method": "eth_getBalance", "params": ["$sponsor", "pending"],
                "result": "$balance"},
            {"method": "eth_call", "params": [{"to": "$account", "data": "$batch_nonce_call"}, "pending"],
                "result": "$batch_nonce"},
            {"method": "eth_getBlockByNumber", "params": ["latest", false],
                "result": {"number": "0x10", "hash": "$latest_block", "timestamp": "0x3e8"}},
            {"method": "eth_getBlockByNumber", "params": ["finalized", false],
                "result": {"number": "0xf", "hash": "$finalized_block", "timestamp": "0x3e0"}},
            {"method": "eth_getBlockByNumber", "params": ["0xf", false],
                "result": {"number": "0xf", "hash": "$finalized_block", "timestamp": "0x3e0"}},
            {"method": "eth_getCode", "params": ["$account", latest], "result": "$delegated_code"},
            {"method": "eth_getCode", "params": ["$account", finalized], "result": "$delegated_code"},
            {"method": "eth_getCode", "params": ["$account", "pending"], "result": "$delegated_code"},
            {"method": "eth_call", "params": [{"to": "$account", "data": "$batch_nonce_call"}, latest],
                "result": "$zero"},
            {"method": "eth_call", "params": [{"to": "$did_precompile", "data": "$did_call"}, "pending"],
                "result": "$did_binding"},
            {"method": "eth_call", "params": [{"to": "$account", "data": "$consumed_call"}, finalized],
                "result": "$zero"},
        ],
        "did_unreachable": [],
        "receipted": [sent.clone(), receipt(executed("receipted", "0x1", json!("$sponsor")))],
        "reverted": [sent.clone(), receipt(reverted),
            {"method": "eth_getTransactionReceipt", "params": ["$hash"], "result": null}],
        "foreign_sponsor": [sent.clone(), receipt(executed("receipted", "0x1", json!("$paymaster")))],
        "held": [sent, receipt(json!({"rung": "submitted", "txHash": "$hash", "nonce": "0x5"}))],
        "refused": [receipt(Value::Null)],
        "rate_age_150": rate("0x0000000000000000000000000000000000000000000000000000000000000352"),
        "rate_age_120": rate("0x0000000000000000000000000000000000000000000000000000000000000370"),
    });
    for nonce in 2..=4 {
        rules["*"].as_array_mut().unwrap_or_else(|| panic!("recorded RPC rules")).push(json!({
            "method": "eth_call",
            "params": [{"to": "$account", "data": format!("$consumed_call_{nonce}")}, finalized],
            "result": "$zero",
        }));
    }
    rules
}

fn with_sequencer_endpoint_rules(mut fixture: Value) -> Result<Value, Box<dyn std::error::Error>> {
    let phases = fixture["phases"]
        .as_object_mut()
        .ok_or("fixture without phases")?;
    if let Value::Object(added) = sequencer_endpoint_rules() {
        for (phase, rules) in added {
            let mut merged = rules.as_array().cloned().ok_or("phase rules not a list")?;
            if let Some(existing) = phases.get(&phase).and_then(Value::as_array) {
                merged.extend(existing.iter().cloned());
            }
            phases.insert(phase, Value::Array(merged));
        }
    }
    let receipts: Value =
        serde_json::from_str(include_str!("../fixtures/sequencer_receipts.json"))?;
    for (phase, rung) in [
        ("full_committed", "committed"),
        ("full_receipted", "receipted"),
    ] {
        let mut receipt = receipts[rung].clone();
        receipt["txHash"] = json!("$hash");
        receipt["nonce"] = json!("0x5");
        receipt["sponsor"] = json!("$sponsor");
        receipt["from"] = json!("$sponsor");
        receipt["to"] = json!("$account");
        phases.insert(
            phase.into(),
            json!([
                {"method": "eth_sendRawTransaction", "params": ["$raw"], "result": "$hash"},
                {"method": "pax_sequencerReceipt", "params": ["$hash"], "result": receipt},
            ]),
        );
    }
    Ok(fixture)
}

#[derive(Clone)]
struct SharedLog(Arc<Mutex<Vec<u8>>>);
impl std::io::Write for SharedLog {
    fn write(&mut self, data: &[u8]) -> std::io::Result<usize> {
        self.0
            .lock()
            .map_err(|_| std::io::Error::other("log poisoned"))?
            .extend_from_slice(data);
        Ok(data.len())
    }
    fn flush(&mut self) -> std::io::Result<()> {
        Ok(())
    }
}

fn wire_calls() -> Value {
    json!([{"to": hex(&[0x55; 20]), "data": "0x010203", "value": "0"}])
}

struct Harness {
    address: SocketAddr,
    recorded: Arc<Recording>,
    account: LocalSigner,
    sponsor: Address,
    secrets: Vec<String>,
    log: SharedLog,
    journal: PathBuf,
    clock: Arc<AtomicU64>,
    config: ServiceConfig,
    listener_control: TcpListener,
    worker: Option<std::thread::JoinHandle<()>>,
    did_gas_limit: u64,
}

fn station_config(name: &str) -> StationConfig {
    StationConfig {
        chain_id: CHAIN_ID,
        endpoints: vec![format!("https://{}", std::process::id())],
        paymaster: PAYMASTER,
        token: SIDIORA,
        decimals: 6,
        max_rate_age: 300,
        spread_bps: 500,
        margin_bps: 100,
        per_account_limit: 8_000_000,
        per_interval_limit: 16_000_000,
        per_quote_limit: 4_000_000,
        interval_seconds: 60,
        balance_floor: 100,
        relayer_key_env: format!("PAXEER_SERVICE_{name}_KEY"),
    }
}

fn ephemeral_signer(
    config: &StationConfig,
) -> Result<(LocalSigner, String), Box<dyn std::error::Error>> {
    let secret = hex(&SigningKey::random(&mut OsRng).to_bytes());
    std::env::set_var(&config.relayer_key_env, &secret);
    let signer = LocalSigner::from_env(&config.relayer_key_env);
    std::env::remove_var(&config.relayer_key_env);
    Ok((signer?, secret))
}

/// Builds the real service over the recorded node inside its own thread and
/// serves it on `listener`, returning once it is ready or refused to start.
fn spawn_service(
    config: ServiceConfig,
    signer: LocalSigner,
    recorded: Arc<Recording>,
    journal: PathBuf,
    listener: TcpListener,
    clock: Arc<AtomicU64>,
    mut log: SharedLog,
) -> Result<std::thread::JoinHandle<()>, Box<dyn std::error::Error>> {
    let (ready, started) = mpsc::channel();
    let worker = std::thread::spawn(move || {
        let built = (|| -> Result<_, Box<dyn std::error::Error>> {
            let rpc = ConfiguredRpc::new(&config.station, ReplayExchange(Arc::clone(&recorded)))?;
            let rates = PaymasterRateSource::new(
                ConfiguredRpc::new(&config.station, ReplayExchange(Arc::clone(&recorded)))?,
                config.station.paymaster,
            );
            let sequencer = SequencerEndpoint::new(ConfiguredRpc::new(
                &config.station,
                ReplayExchange(recorded),
            )?);
            let station = GasStation::new(
                config.station.clone(),
                signer,
                rpc,
                rates,
                Journal::open(&journal)?,
            )?;
            Ok(Service::new(
                &config,
                station,
                move || Some(clock.load(Ordering::SeqCst)),
                Limits {
                    max_body: 65_536,
                    read_time: Duration::from_millis(500),
                },
            )
            .with_sequencer(sequencer))
        })();
        match built {
            Ok(mut service) => {
                if ready.send(Ok(())).is_ok() {
                    let _ = serve(&listener, &mut service, &mut log);
                }
            }
            Err(error) => {
                let _ = ready.send(Err(error.to_string()));
            }
        }
    });
    started.recv_timeout(Duration::from_secs(10))??;
    Ok(worker)
}

fn start(name: &str) -> Result<Harness, Box<dyn std::error::Error>> {
    start_with_rate_age(name, 300)
}

fn start_with_rate_age(
    name: &str,
    max_rate_age: u64,
) -> Result<Harness, Box<dyn std::error::Error>> {
    start_configured(name, |config, _| config.station.max_rate_age = max_rate_age)
}

/// The recorded node answering from `fixture` for the station whose sponsor
/// is `sponsor`, quoting for `account`.
fn recording(
    sponsor: Address,
    account: Address,
    fixture: Value,
) -> Result<Arc<Recording>, Box<dyn std::error::Error>> {
    let mut bindings: BTreeMap<String, Value> = [
        ("$sponsor", hex(&sponsor)),
        ("$paymaster", hex(&PAYMASTER)),
        ("$current_rate_call", hex(&keccak(b"currentRate()")[..4])),
        ("$rate_updated_call", hex(&keccak(b"rateUpdatedAt()")[..4])),
        ("$account", hex(&account)),
        ("$zero", hex(&word(0))),
        ("$batch_nonce", hex(&word(0))),
        ("$balance", "0x3782dace9d900000".into()),
        ("$one", hex(&word(1))),
        ("$batch_nonce_call", hex(&keccak(b"nonce()")[..4])),
        ("$latest_block", hex(&[0x10; 32])),
        ("$finalized_block", hex(&[0x0f; 32])),
        ("$did_precompile", hex(&DID_PRECOMPILE)),
        (
            "$did_call",
            hex(&[
                &keccak(b"getLayerXDid(address)")[..4],
                address_word(account).as_slice(),
            ]
            .concat()),
        ),
        ("$did_binding", hex(&did_binding(&[0xd1; 32])?)),
        (
            "$delegated_code",
            hex(&[[0xef, 0x01, 0x00].as_slice(), PAYMASTER.as_slice()].concat()),
        ),
    ]
    .into_iter()
    .map(|(k, v)| (k.to_owned(), json!(v)))
    .collect();
    for nonce in 2..=4 {
        let data = [
            keccak(b"usedQuoteNonces(address,uint256)")[..4].to_vec(),
            address_word(sponsor).to_vec(),
            word(nonce).to_vec(),
        ]
        .concat();
        bindings.insert(format!("$consumed_call_{nonce}"), json!(hex(&data)));
    }
    let recorded = Arc::new(Recording {
        fixture,
        phase: Mutex::new("submit".into()),
        bindings: Mutex::new(bindings),
        sent: Mutex::new(vec![]),
    });
    recorded.expect_quote_nonce(sponsor, 1);
    Ok(recorded)
}

/// Starts the service with its configuration changed by `adjust`, which also
/// receives the address of the account the harness quotes for.
fn start_configured(
    name: &str,
    adjust: impl FnOnce(&mut ServiceConfig, Address),
) -> Result<Harness, Box<dyn std::error::Error>> {
    let station = station_config(name);
    let (signer, relayer_secret) = ephemeral_signer(&station)?;
    let (account, account_secret) = ephemeral_signer(&station)?;
    let sponsor = signer.address();
    let journal = std::env::temp_dir().join(format!(
        "paxeer-service-{}-{}.jsonl",
        name.to_ascii_lowercase(),
        std::process::id()
    ));
    if journal.exists() {
        std::fs::remove_file(&journal)?;
    }
    let recorded = recording(
        sponsor,
        account.address(),
        with_sequencer_endpoint_rules(serde_json::from_str(include_str!(
            "../fixtures/cycle.json"
        ))?)?,
    )?;
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let address = listener.local_addr()?;
    let mut config = ServiceConfig {
        station,
        listen: address,
        gas_limit: 200_000,
        max_priority_fee_per_gas: 1_000_000_000,
        registered_senders: Vec::new(),
    };
    adjust(&mut config, account.address());
    let log = SharedLog(Arc::new(Mutex::new(Vec::new())));
    let _did_env = DID_GAS_LIMIT_ENV
        .lock()
        .unwrap_or_else(PoisonError::into_inner);
    std::env::set_var("GAS_STATION_DID_GAS_LIMIT", "1000000");
    let clock = Arc::new(AtomicU64::new(1000));
    let listener_control = listener.try_clone()?;
    let worker = spawn_service(
        config.clone(),
        signer,
        Arc::clone(&recorded),
        journal.clone(),
        listener,
        Arc::clone(&clock),
        log.clone(),
    )?;
    Ok(Harness {
        address,
        recorded,
        account,
        sponsor,
        secrets: vec![relayer_secret, account_secret],
        log,
        journal,
        clock,
        config,
        listener_control,
        worker: Some(worker),
        did_gas_limit: 1_000_000,
    })
}

impl Harness {
    fn exchange(&self, request: &[u8]) -> Result<(u16, Value), Box<dyn std::error::Error>> {
        let mut stream = TcpStream::connect(self.address)?;
        stream.set_read_timeout(Some(Duration::from_secs(10)))?;
        stream.write_all(request)?;
        let mut response = Vec::new();
        stream.read_to_end(&mut response)?;
        let split = response
            .windows(4)
            .position(|w| w == b"\r\n\r\n")
            .ok_or("response without head")?;
        let head = std::str::from_utf8(&response[..split])?;
        let status = head
            .split(' ')
            .nth(1)
            .ok_or("response without status")?
            .parse()?;
        let body = &response[split + 4..];
        assert!(head.contains(&format!("Content-Length: {}", body.len())));
        Ok((status, serde_json::from_slice(body)?))
    }

    fn post(&self, path: &str, body: &Value) -> Result<(u16, Value), Box<dyn std::error::Error>> {
        let body = body.to_string();
        self.exchange(
            format!(
                "POST {path} HTTP/1.1\r\nHost: station\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{body}",
                body.len()
            )
            .as_bytes(),
        )
    }

    /// The body `requestGasQuote` in the TypeScript SDK sends.
    fn quote_body(&self) -> Value {
        json!({
            "account": hex(&self.account.address()),
            "nonce": "0",
            "calls": wire_calls(),
            "maxTokenAmount": "3200000",
            "gasCost": GAS_COST,
            "chainId": "1325",
            "token": "0x21f7b20a555199fa73A238B1a91FD0f549068fEe",
            "decimals": 6,
        })
    }

    fn quoted(&self, response: &Value) -> Result<SignedQuote, Box<dyn std::error::Error>> {
        let quote = &response["quote"];
        let number = |name: &str| -> Result<u128, Box<dyn std::error::Error>> {
            Ok(quote[name]
                .as_str()
                .ok_or("numeric field not a string")?
                .parse()?)
        };
        let address = |name: &str| -> Result<Address, Box<dyn std::error::Error>> {
            Ok(bytes(quote[name].as_str().ok_or("address not a string")?)?
                .try_into()
                .map_err(|_| "address length")?)
        };
        let quote = Quote {
            sponsor: address("sponsor")?,
            token: address("token")?,
            max_token_amount: word(number("maxTokenAmount")?),
            token_amount: word(number("tokenAmount")?),
            deadline: word(number("deadline")?),
            nonce: word(number("quoteNonce")?),
            gas_cost: word(number("gasCost")?),
        };
        let signature: [u8; 65] = bytes(
            response["relayerSignature"]
                .as_str()
                .ok_or("signature not a string")?,
        )?
        .try_into()
        .map_err(|_| "signature length")?;
        Ok(SignedQuote {
            digest: quote_digest(word(u128::from(CHAIN_ID)), self.account.address(), &quote),
            quote,
            signature,
        })
    }

    /// The body `sendSponsoredBatch` in the web application sends.
    fn submission_body(&self, quote_response: &Value) -> Result<Value, Box<dyn std::error::Error>> {
        let signed = self.quoted(quote_response)?;
        let calls = vec![Call {
            to: [0x55; 20],
            value: word(0),
            data: vec![1, 2, 3],
        }];
        let account_signature = self.account.sign_digest(batch_digest(
            CHAIN_ID,
            self.account.address(),
            word(0),
            &calls,
            &signed.quote,
        )?)?;
        let mut authorization = Authorization {
            chain_id: CHAIN_ID,
            delegate: PAYMASTER,
            nonce: 0,
            signature: [0; 65],
        };
        let auth_signature = self
            .account
            .sign_digest(authorization_digest(&authorization)?)?;
        authorization.signature = auth_signature;
        let data = encode_sponsored(&calls, &signed, &account_signature)?;
        Ok(json!({
            "call": {"to": hex(&self.account.address()), "value": "0", "data": hex(&data)},
            "authorization": {
                "chainId": "1325",
                "address": hex(&PAYMASTER),
                "nonce": "0",
                "yParity": auth_signature[64] - 27,
                "r": hex(&auth_signature[..32]),
                "s": hex(&auth_signature[32..64]),
            },
            "batch": {
                "chainId": "1325",
                "account": hex(&self.account.address()),
                "nonce": "0",
                "calls": wire_calls(),
                "quote": quote_response["quote"],
            },
            "accountSignature": hex(&account_signature),
            "relayerSignature": quote_response["relayerSignature"],
        }))
    }

    /// The body a client polls `/status` with for the quote it holds.
    fn status_body(&self, quote_response: &Value) -> Value {
        json!({
            "sponsor": quote_response["quote"]["sponsor"],
            "quoteNonce": quote_response["quote"]["quoteNonce"],
            "account": hex(&self.account.address()),
            "relayerSignature": quote_response["relayerSignature"],
        })
    }

    fn log(&self) -> String {
        self.log
            .0
            .lock()
            .map(|log| String::from_utf8_lossy(&log).into_owned())
            .unwrap_or_default()
    }

    /// The service writes a request's log line after it delivered the
    /// response, so the log is read once it holds `lines` lines or after a
    /// bounded wait.
    fn assert_log_clean(&self, lines: &[&str]) {
        let waited = std::time::Instant::now();
        while self.log().lines().count() < lines.len() && waited.elapsed() < Duration::from_secs(5)
        {
            std::thread::sleep(Duration::from_millis(10));
        }
        let log = self.log();
        assert_eq!(log.lines().collect::<Vec<_>>(), lines);
        for secret in &self.secrets {
            assert!(!log.contains(secret.trim_start_matches("0x")));
        }
        for raw in self.recorded.sent() {
            assert!(!log.contains(hex(&raw).trim_start_matches("0x")));
        }
    }

    fn journal_is_empty(&self) -> Result<bool, Box<dyn std::error::Error>> {
        // Startup binds the service policy before it handles any request.
        // A refused quote must add no records beyond that binding.
        let lines = std::fs::read_to_string(&self.journal)?;
        let records = lines
            .lines()
            .map(
                |line| -> Result<layerx_gas_station::journal::Entry, Box<dyn std::error::Error>> {
                    let record: Value = serde_json::from_str(line)?;
                    Ok(serde_json::from_str(
                        record["payload"].as_str().ok_or("authenticated payload")?,
                    )?)
                },
            )
            .collect::<Result<Vec<_>, _>>()?;
        Ok(matches!(
            records.as_slice(),
            [layerx_gas_station::journal::Entry::PolicyBound { .. }]
        ))
    }

    fn stop(&mut self) -> TestResult {
        if let Some(worker) = self.worker.take() {
            self.listener_control.set_nonblocking(true)?;
            // Wake a blocking accept, then let the production serve loop return
            // on its next nonblocking accept so it drops the journal lock.
            self.exchange(b"GET /healthz HTTP/1.1\r\nHost: station\r\n\r\n")?;
            worker.join().map_err(|_| "service worker panicked")?;
        }
        Ok(())
    }

    fn restart(&mut self) -> TestResult {
        self.stop()?;
        let listener = TcpListener::bind("127.0.0.1:0")?;
        self.address = listener.local_addr()?;
        self.listener_control = listener.try_clone()?;
        self.config.listen = self.address;
        let name = &self.config.station.relayer_key_env;
        std::env::set_var(name, &self.secrets[0]);
        let signer = LocalSigner::from_env(name);
        std::env::remove_var(name);
        let _did_env = DID_GAS_LIMIT_ENV
            .lock()
            .unwrap_or_else(PoisonError::into_inner);
        std::env::set_var("GAS_STATION_DID_GAS_LIMIT", self.did_gas_limit.to_string());
        self.worker = Some(spawn_service(
            self.config.clone(),
            signer?,
            Arc::clone(&self.recorded),
            self.journal.clone(),
            listener,
            Arc::clone(&self.clock),
            self.log.clone(),
        )?);
        Ok(())
    }

    fn finish(mut self) -> TestResult {
        self.stop()?;
        std::fs::remove_file(&self.journal)?;
        Ok(())
    }
}

fn sequencer_endpoint_submission(
    name: &str,
    phase: &str,
) -> Result<SequencerEndpointSubmission, Box<dyn std::error::Error>> {
    let harness = start(name)?;
    harness.recorded.phase(phase);
    let (status, quote) = harness.post("/quote", &harness.quote_body())?;
    assert_eq!(status, 200);
    let answer = harness.post("/submit", &harness.submission_body(&quote)?)?;
    assert_eq!(harness.recorded.sent().len(), 1);
    Ok((harness, quote, answer))
}

type SequencerEndpointSubmission = (Harness, Value, (u16, Value));
