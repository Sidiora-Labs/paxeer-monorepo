//! Task 22.33.70 repair record:
//! registered-budget-unwired: registered_caps_survive_service_restart.
//! gas-journal-growth: sustained_rotation_preserves_accounting_and_recovery and
//! authenticated_snapshot_and_replay_index_reject_tampering, and
//! rate_rotation_preserves_daily_costs_and_unresolved_bytes.
//! rate-unbound-view: producer_receipts_bind_every_execution_field and
//! publisher_rejects_json_only_outcome_forgery.
//! Baseline: no pre-change test run is claimed; the explicit ordered gate rule
//! requires completing implementation before hf-build and hf-done.
//! Inherited attempt 1 failed registered_caps_survive_service_restart,
//! sustained_rotation_preserves_accounting_and_recovery, and
//! authenticated_snapshot_and_replay_index_reject_tampering with Rpc(Malformed),
//! and publisher_rejects_json_only_outcome_forgery with Rpc(Configuration).
//! Wiring requests: none. All repairs are within this crate's declared scope.

use std::fs;
use std::os::unix::fs::DirBuilderExt as _;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex, MutexGuard};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_gas_station::config::{RegisteredSender, ServiceConfig, StationConfig};
use layerx_gas_station::journal::{
    Completion, Entry, Journal, JournalError, Key, LiabilityRelease, Publication,
    PublicationTransaction, QuoteRecord, Submission,
};
use layerx_gas_station::price::PaymasterRateSource;
use layerx_gas_station::quote::{keccak, word, Address, Word, SIDIORA};
use layerx_gas_station::rate::{
    verify_sequencer_view, PublisherConfig, RatePublisher, RateRefusal,
};
use layerx_gas_station::rpc::{bytes, hex, ConfiguredRpc, Exchange, RpcFault};
use layerx_gas_station::service::{Limits, Service, ServiceError};
use layerx_gas_station::signer::{QuoteSigner, SignerError};
use layerx_gas_station::station::{prepare_submission, GasStation};
use layerx_gas_station::tx::{
    authorization_digest, batch_digest, Authorization, Call, Fees, TransactionRequest,
};
use layerx_platform_internal::evm_sender::EvmSender;
use layerx_proof::inclusion::SequencerAuthorization;
use serde_json::{json, Value};

type Result<T = ()> = std::result::Result<T, Box<dyn std::error::Error>>;
const PAYMASTER: Address = [0x44; 20];
const AMOUNT: u128 = 3_145_140;
const GAS_COST: u128 = 1_000_000_000_000_000_000;
static ENVIRONMENT: Mutex<()> = Mutex::new(());

struct Runtime {
    directory: PathBuf,
    previous: Option<std::ffi::OsString>,
    _guard: MutexGuard<'static, ()>,
}
impl Runtime {
    fn new() -> Result<Self> {
        let guard = ENVIRONMENT
            .lock()
            .map_err(|_| "environment lock poisoned")?;
        let directory = std::env::temp_dir().join(format!(
            "repair-22-33-70-{}-{}",
            std::process::id(),
            SystemTime::now().duration_since(UNIX_EPOCH)?.as_nanos()
        ));
        fs::DirBuilder::new().mode(0o700).create(&directory)?;
        let previous = std::env::var_os("GAS_STATION_DID_GAS_LIMIT");
        std::env::set_var("GAS_STATION_DID_GAS_LIMIT", "1000000000");
        Ok(Self {
            directory,
            previous,
            _guard: guard,
        })
    }
    fn journal(&self) -> PathBuf {
        self.directory.join("station.jsonl")
    }
}
impl Drop for Runtime {
    fn drop(&mut self) {
        match &self.previous {
            Some(value) => std::env::set_var("GAS_STATION_DID_GAS_LIMIT", value),
            None => std::env::remove_var("GAS_STATION_DID_GAS_LIMIT"),
        }
        let _ = fs::remove_dir_all(&self.directory);
    }
}
struct Signer(EvmSender);
impl Signer {
    fn new(secret: u8) -> Result<Self> {
        Ok(Self(EvmSender::from_hex_secret(&format!("{secret:064x}"))?))
    }
}
impl QuoteSigner for Signer {
    fn address(&self) -> Address {
        self.0.address()
    }
    fn sign_digest(&self, digest: Word) -> std::result::Result<[u8; 65], SignerError> {
        self.0.sign_prehash(&digest).map_err(Into::into)
    }
}
fn config() -> ServiceConfig {
    ServiceConfig {
        station: StationConfig {
            chain_id: 1325,
            endpoints: vec!["https://recorded.invalid".into()],
            paymaster: PAYMASTER,
            token: SIDIORA,
            decimals: 6,
            max_rate_age: 300,
            spread_bps: 500,
            margin_bps: 100,
            per_account_limit: 7_000_000,
            per_interval_limit: 100_000_000,
            per_quote_limit: 4_000_000,
            interval_seconds: 60,
            balance_floor: 100,
            relayer_key_env: "REPAIR_RELAYER_KEY".into(),
        },
        listen: "127.0.0.1:9000".parse().expect("literal listen address"),
        gas_limit: 200_000,
        max_priority_fee_per_gas: 1_000_000_000,
        registered_senders: vec![
            RegisteredSender {
                address: [0x11; 20],
                budget: 10_000_000,
            },
            RegisteredSender {
                address: [0x22; 20],
                budget: 4_000_000,
            },
        ],
    }
}

// A parameterized replay of the committed cycle and governed-price transcripts.
// The component under repair is the real Service/GasStation/Journal, while this
// peer supplies deterministic node observations through the production RPC client.
#[derive(Clone)]
struct Transcript {
    now: Arc<AtomicU64>,
    nonce: Arc<AtomicU64>,
    held: Option<Submission>,
    answer: Option<Value>,
    chain: u64,
}
impl Transcript {
    fn new() -> Self {
        Self {
            now: Arc::new(AtomicU64::new(1000)),
            nonce: Arc::new(AtomicU64::new(0)),
            held: None,
            answer: None,
            chain: 1325,
        }
    }
}
impl Exchange for Transcript {
    fn request_until(
        &self,
        endpoint: &str,
        method: &str,
        params: &Value,
        deadline: Instant,
    ) -> std::result::Result<Value, RpcFault> {
        if Instant::now() >= deadline {
            return Err(RpcFault::Unavailable);
        }
        self.request(endpoint, method, params)
    }
    fn request(
        &self,
        _endpoint: &str,
        method: &str,
        params: &Value,
    ) -> std::result::Result<Value, RpcFault> {
        let now = self.now.load(Ordering::SeqCst);
        let result = match method {
            "eth_chainId" => {
                assert_eq!(*params, json!([]));
                json!(format!("0x{:x}", self.chain))
            }
            "eth_getBalance" => json!("0xffffffffffffffffffffffffffffffff"),
            "eth_getBlockByNumber" => {
                assert_eq!(params[1], false);
                json!({"number":"0x10", "hash":hex(&[0xab;32]), "timestamp":format!("0x{now:x}")})
            }
            "eth_getCode" => json!(hex(&[&[0xef, 0x01, 0x00][..], &PAYMASTER].concat())),
            "eth_call" => {
                let data = bytes(params[0]["data"].as_str().ok_or(RpcFault::Malformed)?)?;
                let selector = data.get(..4).ok_or(RpcFault::Malformed)?;
                if selector == &keccak(b"nonce()")[..4] {
                    assert_eq!(params[1], "pending");
                    json!(hex(&word(self.nonce.load(Ordering::SeqCst).into())))
                } else if selector == &keccak(b"usedQuoteNonces(address,uint256)")[..4] {
                    assert_eq!(
                        params[1],
                        json!({"blockHash":hex(&[0xab;32]),"requireCanonical":true})
                    );
                    json!(hex(&word(0)))
                } else if selector == &keccak(b"getLayerXDid(address)")[..4] {
                    assert_eq!(params[1], "pending");
                    return Err(RpcFault::Rejected { code: 3 });
                } else if selector == &keccak(b"getUnifiedAccount(address)")[..4] {
                    let account = data.get(4..36).ok_or(RpcFault::Malformed)?;
                    json!(hex(
                        &[account, &word(128), &word(0), &word(0), &word(0)].concat()
                    ))
                } else {
                    assert_eq!(params[0]["to"], hex(&PAYMASTER));
                    assert_eq!(params[1], "latest");
                    let fixture: Value =
                        serde_json::from_str(include_str!("fixtures/paymaster_rate.json"))
                            .map_err(|_| RpcFault::Malformed)?;
                    if selector == &keccak(b"rateUpdatedAt()")[..4] {
                        // Advance the recorded governed-rate timestamp with chain time.
                        json!(hex(&word(now.into())))
                    } else {
                        assert_eq!(selector, &keccak(b"currentRate()")[..4]);
                        fixture["fresh"]["currentRate()"]["result"].clone()
                    }
                }
            }
            "eth_getTransactionReceipt" => Value::Null,
            "pax_sequencerReceipt" => {
                if let Some(answer) = &self.answer {
                    answer.clone()
                } else {
                    let held = self.held.as_ref().ok_or(RpcFault::Malformed)?;
                    assert_eq!(*params, json!([hex(&held.hash)]));
                    json!({"rung":"submitted", "txHash":hex(&held.hash),"nonce":format!("0x{:x}",held.nonce)})
                }
            }
            _ => panic!("unexpected transcript request: {method} {params}"),
        };
        Ok(result)
    }
}
type Rpc = ConfiguredRpc<Transcript>;
type TestService = Service<Signer, Rpc, PaymasterRateSource<Rpc>, fn() -> Option<u64>>;
fn clock() -> Option<u64> {
    Some(1000)
}
fn open_service(
    config: &ServiceConfig,
    path: &Path,
    transcript: &Transcript,
) -> Result<TestService> {
    let rpc = ConfiguredRpc::new(&config.station, transcript.clone())
        .map_err(|error| format!("service RPC configuration: {error:?}"))?;
    let prices = PaymasterRateSource::new(
        ConfiguredRpc::new(&config.station, transcript.clone())?,
        PAYMASTER,
    );
    let station = GasStation::new(
        config.station.clone(),
        Signer::new(1)?,
        rpc,
        prices,
        Journal::open(path)?,
    )
    .map_err(|error| format!("station startup: {error:?}"))?;
    Ok(Service::new(
        config,
        station,
        clock as fn() -> Option<u64>,
        Limits::default(),
    ))
}
fn request(account: Address, nonce: u64) -> Vec<u8> {
    serde_json::to_vec(&json!({"account":hex(&account),"nonce":nonce.to_string(),"calls":[],
        "maxTokenAmount":"4000000","gasCost":GAS_COST.to_string(),"chainId":"1325","token":hex(&SIDIORA),"decimals":6})).expect("serializable request")
}
fn quote(
    service: &mut TestService,
    transcript: &Transcript,
    account: Address,
    nonce: u64,
) -> std::result::Result<Value, ServiceError> {
    transcript.nonce.store(nonce, Ordering::SeqCst);
    service.respond("/quote", &request(account, nonce))
}
fn key(answer: &Value) -> Result<Key> {
    Ok(Key {
        sponsor: Signer::new(1)?.address(),
        quote_nonce: word(
            answer["quote"]["quoteNonce"]
                .as_str()
                .ok_or("missing quote nonce")?
                .parse()?,
        ),
    })
}

#[test]
fn registered_caps_survive_service_restart() -> Result {
    let runtime = Runtime::new()?;
    let mut config = config();
    // Exercise the actual strict configuration parser as the producer.
    let station = &config.station;
    let encoded = json!({"chain_id":station.chain_id,"endpoints":station.endpoints,"paymaster":hex(&station.paymaster),"token":hex(&station.token),
        "decimals":6,"max_rate_age":300,"spread_bps":500,"margin_bps":100,"per_account_limit":7000000,"per_interval_limit":100000000,
        "per_quote_limit":4000000,"interval_seconds":60,"balance_floor":100,"relayer_key_env":"REPAIR_RELAYER_KEY",
        "listen":"127.0.0.1:9000","gas_limit":200000,"max_priority_fee_per_gas":1000000000,
        "registered_senders":[{"address":hex(&[0x11;20]),"budget":10000000},{"address":hex(&[0x22;20]),"budget":4000000}]});
    config = ServiceConfig::parse(&serde_json::to_string(&encoded)?)?;
    let transcript = Transcript::new();
    let mut service = open_service(&config, &runtime.journal(), &transcript)?;
    let high = [0x11; 20];
    let low = [0x22; 20];
    let ordinary = [0x33; 20];
    let high_first = quote(&mut service, &transcript, high, 0)?;
    assert_eq!(high_first["quote"]["tokenAmount"], AMOUNT.to_string());
    quote(&mut service, &transcript, high, 1)?;
    quote(&mut service, &transcript, low, 0)?;
    quote(&mut service, &transcript, ordinary, 0)?;
    let before = service.station().journal().state().clone();
    assert_eq!(
        quote(&mut service, &transcript, low, 1),
        Err(ServiceError::Refused)
    );
    assert_eq!(service.station().journal().state(), &before);
    assert_eq!(
        service.station().policy().sender_used(high, 1000),
        2 * AMOUNT
    );
    drop(service);

    let mut service = open_service(&config, &runtime.journal(), &transcript)?;
    assert_eq!(
        service.station().policy().sender_used(high, 1000),
        2 * AMOUNT
    );
    assert_eq!(
        service.station().policy().used(&hex(&ordinary), 1000),
        AMOUNT
    );
    assert_eq!(quote(&mut service, &transcript, high, 0)?, high_first);
    quote(&mut service, &transcript, high, 2)?;
    assert_eq!(
        quote(&mut service, &transcript, high, 3),
        Err(ServiceError::Refused)
    );
    quote(&mut service, &transcript, ordinary, 1)?;
    assert_eq!(
        quote(&mut service, &transcript, ordinary, 2),
        Err(ServiceError::Refused)
    );
    assert_eq!(
        quote(&mut service, &transcript, low, 1),
        Err(ServiceError::Refused)
    );
    assert_eq!(service.station().policy().outstanding_pax(), 6 * GAS_COST);
    drop(service);

    let mut changed = config.clone();
    changed.registered_senders[0].budget += 1;
    let mut refused = open_service(&changed, &runtime.journal(), &transcript)?;
    assert_eq!(
        quote(&mut refused, &transcript, high, 3),
        Err(ServiceError::Conflict)
    );
    assert!(refused.recover(Duration::from_secs(1)).is_err());
    drop(refused);
    let mut service = open_service(&config, &runtime.journal(), &transcript)?;
    transcript.now.store(1020, Ordering::SeqCst);
    quote(&mut service, &transcript, low, 0)?;
    assert_eq!(service.station().policy().sender_used(low, 1020), AMOUNT);
    Ok(())
}

fn prepare(
    journal: &mut Journal,
    quote: QuoteRecord,
    nonce: u64,
    secret: u8,
) -> Result<Submission> {
    let sponsor = Signer::new(1)?;
    let account = Signer::new(secret)?;
    assert_eq!(quote.account, account.address());
    let signed = quote.signed_quote()?;
    let calls: Vec<Call> = Vec::new();
    let mut authorization = Authorization {
        chain_id: 1325,
        delegate: PAYMASTER,
        nonce: 0,
        signature: [0; 65],
    };
    authorization.signature = account.sign_digest(authorization_digest(&authorization)?)?;
    let account_signature = account.sign_digest(batch_digest(
        1325,
        account.address(),
        quote.batch_nonce,
        &calls,
        &signed.quote,
    )?)?;
    Ok(prepare_submission(
        journal,
        TransactionRequest {
            chain_id: 1325,
            account: account.address(),
            paymaster: PAYMASTER,
            nonce,
            batch_nonce: quote.batch_nonce,
            fees: quote.fees,
            calls: &calls,
            authorizations: &[authorization],
            quote: &signed,
            account_signature: &account_signature,
        },
        &quote.did,
        &sponsor,
    )?)
}

#[test]
fn sustained_rotation_preserves_accounting_and_recovery() -> Result {
    let runtime = Runtime::new()?;
    let config = config();
    let mut transcript = Transcript::new();
    let first_account = Signer::new(2)?.address();
    let second_account = Signer::new(3)?.address();
    let mut service = open_service(&config, &runtime.journal(), &transcript)?;
    let first = key(&quote(&mut service, &transcript, first_account, 0)?)?;
    let unresolved = key(&quote(&mut service, &transcript, second_account, 0)?)?;
    let first_quote = service.station().journal().state().items[&first]
        .quote
        .clone()
        .ok_or("first quote")?;
    let second_quote = service.station().journal().state().items[&unresolved]
        .quote
        .clone()
        .ok_or("second quote")?;
    drop(service);
    let mut journal = Journal::open(&runtime.journal())?;
    let settled = prepare(&mut journal, first_quote, 5, 2)?;
    let prepared = prepare(&mut journal, second_quote, 6, 3)?;
    // Finalized Ethereum receipt shape from the committed cycle transcript.
    let cycle: Value = serde_json::from_str(include_str!("fixtures/cycle.json"))?;
    let mut receipt = cycle["phases"]["included"][0]["result"].clone();
    receipt["transactionHash"] = json!(hex(&settled.hash));
    receipt["from"] = json!(hex(&first.sponsor));
    receipt["to"] = json!(hex(&first_account));
    receipt["status"] = json!("0x0");
    receipt["blockHash"] = json!(hex(&[0xab; 32]));
    receipt["logs"] = json!([]);
    let canonical = json!({"number":"0x10","hash":hex(&[0xab;32]),"timestamp":"0x3e8"});
    journal.append(&Entry::ReceiptObserved {
        key: first,
        hash: settled.hash,
        receipt,
        canonical: canonical.clone(),
        finalized: canonical,
    })?;
    journal.append(&Entry::Completed {
        key: first,
        account: first_account,
        completion: Completion::Reverted { hash: settled.hash },
    })?;
    journal.append(&Entry::LiabilityReleased {
        key: first,
        proof: LiabilityRelease::Settled { hash: settled.hash },
    })?;
    let spent = 100_000 * 5_000_000_000_000u128;
    assert_eq!(journal.state().finalized_spend()?, spent);
    drop(journal);
    transcript.held = Some(prepared.clone());
    let mut service = open_service(&config, &runtime.journal(), &transcript)?;
    let traffic = [0x55; 20];
    let mut retired = None;
    for interval in 0..384u64 {
        transcript.now.store(1020 + 60 * interval, Ordering::SeqCst);
        let response = quote(&mut service, &transcript, traffic, interval)?;
        if interval == 0 {
            retired = Some(key(&response)?);
        }
        assert!(service.station().journal().state().items.len() <= 132);
    }
    let state = service.station().journal().state();
    assert_eq!(state.finalized_spend()?, spent);
    assert_eq!(
        state.items[&unresolved].submission.as_ref(),
        Some(&prepared)
    );
    assert_eq!(service.station().policy().outstanding_pax(), 2 * GAS_COST);
    let used = service
        .station()
        .policy()
        .did_budget()
        .used(&hex(&traffic), 0);
    assert_eq!(used, 384 * config.gas_limit);
    let before_nonce = state.quote_nonce_high_water;
    drop(service);
    let mut journal = Journal::open(&runtime.journal())?;
    journal.compact()?;
    assert!(journal.state().items.len() <= 3);
    assert!(journal.retired(first)?);
    assert!(journal.retired(retired.ok_or("retired traffic key")?)?);
    assert_eq!(journal.state().next_nonce(first.sponsor, 0), Some(7));
    assert!(journal.state().holds(first.sponsor, 5));
    assert_eq!(journal.state().finalized_spend()?, spent);
    assert_eq!(journal.state().quote_nonce_high_water, before_nonce);
    let state = journal.state().clone();
    drop(journal);
    let journal = Journal::open(&runtime.journal())?;
    assert_eq!(journal.state(), &state);
    assert_eq!(
        journal.state().items[&unresolved].submission.as_ref(),
        Some(&prepared)
    );
    drop(journal);
    let mut service = open_service(&config, &runtime.journal(), &transcript)?;
    assert_eq!(
        service
            .station()
            .policy()
            .did_budget()
            .used(&hex(&traffic), 0),
        used
    );
    assert_eq!(service.station().policy().outstanding_pax(), 2 * GAS_COST);
    assert_eq!(service.station().policy().finalized_spend(), spent);
    let recovery = service.recover(Duration::from_secs(2))?;
    assert_eq!(recovery.pending, 1);
    assert_eq!(recovery.unreachable, 0);
    assert_eq!(
        service.station().journal().state().items[&unresolved]
            .submission
            .as_ref(),
        Some(&prepared)
    );
    let last_nonce = 383;
    let saved = quote(&mut service, &transcript, traffic, last_nonce)?;
    assert_eq!(key(&saved)?.quote_nonce, before_nonce);
    assert!(fs::metadata(runtime.journal())?.len() < 512 * 1024);
    assert_eq!(fs::read_dir(&runtime.directory)?.count(), 5);
    Ok(())
}

#[test]
fn authenticated_snapshot_and_replay_index_reject_tampering() -> Result {
    let runtime = Runtime::new()?;
    let path = runtime.journal();
    let config = config();
    let transcript = Transcript::new();
    let mut service = open_service(&config, &path, &transcript)?;
    quote(&mut service, &transcript, [0x55; 20], 0)?;
    transcript.now.store(1020, Ordering::SeqCst);
    quote(&mut service, &transcript, [0x55; 20], 1)?;
    drop(service);
    let mut journal = Journal::open(&path)?;
    journal.compact()?;
    assert!(matches!(Journal::open(&path), Err(JournalError::Locked)));
    drop(journal);
    let original = fs::read(&path)?;
    let mut changed: Value = serde_json::from_slice(&original)?;
    let mut payload: Value =
        serde_json::from_str(changed["payload"].as_str().ok_or("snapshot payload")?)?;
    payload["state"]["policy_identity"]["registered_senders"][0]["budget"] = json!(1);
    changed["payload"] = json!(serde_json::to_string(&payload)?);
    let mut changed = serde_json::to_vec(&changed)?;
    changed.push(b'\n');
    fs::write(&path, changed)?;
    assert!(matches!(Journal::open(&path), Err(JournalError::Corrupt)));
    fs::write(&path, &original)?;
    let index = fs::read_dir(&runtime.directory)?
        .collect::<std::result::Result<Vec<_>, _>>()?
        .into_iter()
        .find(|entry| entry.file_name().to_string_lossy().contains(".replay."))
        .ok_or("replay index")?
        .path();
    let original_index = fs::read(&index)?;
    assert!(!original_index.is_empty());
    let mut changed_index = original_index.clone();
    changed_index[0] ^= 1;
    fs::write(&index, changed_index)?;
    assert!(matches!(Journal::open(&path), Err(JournalError::Corrupt)));
    fs::write(&index, original_index)?;
    let journal = Journal::open(&path)?;
    drop(journal);
    let mut journal = Journal::open(&path)?;
    journal.append(&Entry::Completed {
        key: Key {
            sponsor: Signer::new(1)?.address(),
            quote_nonce: word(999),
        },
        account: [0x77; 20],
        completion: Completion::Consumed,
    })?;
    drop(journal);
    // Dropping a whole acknowledged record must not pass merely because its
    // predecessor remains a valid HMAC chain.
    fs::write(&path, &original)?;
    assert!(matches!(Journal::open(&path), Err(JournalError::Corrupt)));
    let mut torn = original;
    torn.pop();
    fs::write(&path, torn)?;
    assert!(matches!(Journal::open(&path), Err(JournalError::Corrupt)));
    Ok(())
}

fn vectors() -> Result<Value> {
    Ok(serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))?)
}
fn fixed<const N: usize>(value: &Value) -> Result<[u8; N]> {
    Ok(bytes(value.as_str().ok_or("hex field")?)?
        .try_into()
        .map_err(|_| "field width")?)
}
fn pins(case: &Value) -> Result<Vec<SequencerAuthorization>> {
    let pin = &case["pinnedKey"];
    Ok(vec![SequencerAuthorization::new(
        fixed(&pin["keyId"])?,
        fixed(&pin["publicKey"])?,
        pin["firstHeight"].as_u64().ok_or("first height")?,
        pin["lastHeight"].as_u64().ok_or("last height")?,
    )])
}

#[test]
fn producer_receipts_bind_every_execution_field() -> Result {
    let fixtures = vectors()?;
    for case in fixtures["cases"]
        .as_array()
        .ok_or("producer vectors")?
        .iter()
        .filter(|case| {
            case["id"]
                .as_str()
                .is_some_and(|id| id.starts_with("V1-") || id == "V2-layerxledger-revert")
        })
    {
        let view = &case["receiptJson"];
        let hash = keccak(&bytes(
            case["rawTransaction"].as_str().ok_or("raw transaction")?,
        )?);
        assert_eq!(hash, fixed(&view["txHash"])?);
        let pins = pins(case)?;
        let expected = view["status"] == "0x1";
        assert_eq!(
            verify_sequencer_view(view, &hash, &pins),
            Ok(expected),
            "{}",
            case["id"]
        );
        for (field, replacement) in [
            ("status", json!(if expected { "0x0" } else { "0x1" })),
            ("gasUsed", json!("0x1")),
            ("effectiveGasPrice", json!("0x1")),
            ("from", json!(hex(&[0x99; 20]))),
            ("to", json!(hex(&[0x99; 20]))),
            ("nonce", json!("0xffff")),
            ("timestamp", json!("0x1")),
            ("cumulativeGasUsed", json!("0x1")),
            ("returnData", json!("0x01")),
            ("vmError", json!("changed")),
            ("paymentDenom", json!("other")),
            ("feeDenom", json!("other")),
            ("feeAmount", json!("0x1")),
            ("sponsor", json!(hex(&[0x99; 20]))),
            ("contractAddress", json!(hex(&[0x99; 20]))),
            ("signingKey", json!(hex(&[0x99; 32]))),
        ] {
            let mut changed = view.clone();
            changed[field] = replacement;
            assert_eq!(changed["receipt"], view["receipt"]);
            assert_eq!(
                verify_sequencer_view(&changed, &hash, &pins),
                Err(RpcFault::Malformed),
                "{} {field}",
                case["id"]
            );
        }
        if !view["logs"].as_array().ok_or("logs")?.is_empty() {
            for (field, replacement) in [
                ("data", json!(hex(&word(2)))),
                ("address", json!(hex(&[0x99; 20]))),
                ("topics", json!([])),
                ("logIndex", json!("0x1")),
            ] {
                let mut changed = view.clone();
                changed["logs"][0][field] = replacement;
                assert_eq!(
                    verify_sequencer_view(&changed, &hash, &pins),
                    Err(RpcFault::Malformed),
                    "{} log {field}",
                    case["id"]
                );
            }
        }
    }
    Ok(())
}

#[test]
fn publisher_rejects_json_only_outcome_forgery() -> Result {
    let runtime = Runtime::new()?;
    let fixtures = vectors()?;
    let case = fixtures["cases"]
        .as_array()
        .ok_or("vectors")?
        .iter()
        .find(|case| case["id"] == "V2-layerxledger-revert")
        .ok_or("node revert vector")?;
    let view = &case["receiptJson"];
    let raw = bytes(case["rawTransaction"].as_str().ok_or("raw transaction")?)?;
    let transaction = PublicationTransaction {
        version: 1,
        publication: Publication {
            owner: fixed(&view["from"])?,
            nonce: layerx_gas_station::rpc::quantity(view["nonce"].as_str().ok_or("nonce")?)?
                .try_into()?,
            hash: keccak(&raw),
            rate: word(1),
            gas_limit: 200_000,
            max_fee_per_gas: 5_000_000_000_000,
            signed_at: 1000,
        },
        chain_id: 125,
        paymaster: fixed(&view["to"])?,
        max_priority_fee_per_gas: 1_000_000_000,
        raw,
        cancellation: false,
    };
    let mut station_config = config().station;
    station_config.chain_id = 125;
    let publisher_config = PublisherConfig {
        station: station_config.clone(),
        owner_key_env: "REPAIR_OWNER_KEY".into(),
        cadence_seconds: 30,
        gas_budget_per_day: 1_000_000,
        balance_floor: 100,
        max_priority_fee_per_gas: 1_000_000_000,
        max_fee_per_gas: 5_000_000_000_000,
        confirmation_retry_seconds: 1,
        sequencer_receipt_keys: pins(case)?,
    };
    let mut transcript = Transcript::new();
    transcript.chain = 125;
    for (index, field) in [None, Some("status"), Some("gasUsed"), Some("logs")]
        .into_iter()
        .enumerate()
    {
        let mut answer = view.clone();
        if let Some(field) = field {
            answer[field] = match field {
                "status" => json!("0x1"),
                "gasUsed" => json!("0x1"),
                _ => {
                    json!([{"address":hex(&transaction.paymaster),"topics":[],"data":"0x","logIndex":"0x0"}])
                }
            };
        }
        transcript.answer = Some(answer);
        publisher_config.validate()?;
        let rpc = ConfiguredRpc::new(&station_config, transcript.clone())?;
        let mut publisher = RatePublisher::new(
            publisher_config.clone(),
            Signer::new(1)?,
            rpc,
            Journal::open(&runtime.directory.join(format!("rate-{index}.jsonl")))?,
            Duration::from_millis(1),
        )
        .map_err(|error| format!("publisher startup: {error:?}"))?;
        let outcome = publisher.confirm_by_receipt(&transaction);
        assert_eq!(
            outcome,
            Err(if field.is_none() {
                RateRefusal::Reverted
            } else {
                RateRefusal::Rpc(RpcFault::Malformed)
            })
        );
        assert!(publisher.journal().state().publications.is_empty());
    }
    Ok(())
}

#[test]
fn rate_rotation_preserves_daily_costs_and_unresolved_bytes() -> Result {
    let runtime = Runtime::new()?;
    let mut journal = Journal::open(&runtime.journal())?;
    let signer = Signer::new(4)?;
    let fees = Fees {
        gas_limit: 60_000,
        max_fee_per_gas: 3_000_000_000,
        max_priority_fee_per_gas: 1_000_000_000,
    };
    let data = [&keccak(b"setRate(uint256)")[..4], &word(7)].concat();
    let mut pending = None;
    let canonical = json!({"number":"0x10","hash":hex(&[0x77;32]),"timestamp":"0x3e8"});
    let fixtures = vectors()?;
    let produced = fixtures["cases"][0]["receiptJson"].clone();
    for nonce in 0..193u64 {
        let envelope =
            layerx_gas_station::tx::sign_call(1325, nonce, fees, PAYMASTER, &data, &signer)?;
        let transaction = PublicationTransaction {
            version: 2,
            publication: Publication {
                owner: signer.address(),
                nonce,
                hash: envelope.hash,
                rate: word(7),
                gas_limit: fees.gas_limit,
                max_fee_per_gas: fees.max_fee_per_gas,
                signed_at: 1000 + nonce,
            },
            chain_id: 1325,
            paymaster: PAYMASTER,
            max_priority_fee_per_gas: fees.max_priority_fee_per_gas,
            raw: envelope.raw,
            cancellation: false,
        };
        journal.append(&Entry::RatePrepared {
            transaction: transaction.clone(),
        })?;
        journal.append(&Entry::RateBroadcast {
            hash: transaction.publication.hash,
        })?;
        if nonce == 0 {
            pending = Some(transaction);
            continue;
        }
        let mut receipt = produced.clone();
        receipt["transactionHash"] = json!(hex(&transaction.publication.hash));
        receipt["blockNumber"] = json!("0x10");
        receipt["blockHash"] = canonical["hash"].clone();
        receipt["from"] = json!(hex(&signer.address()));
        receipt["to"] = json!(hex(&PAYMASTER));
        receipt["status"] = json!("0x0");
        receipt["logs"] = json!([]);
        journal.append(&Entry::RateFinalized {
            hash: transaction.publication.hash,
            settlement: layerx_gas_station::journal::Settlement {
                block_number: 16,
                gas_used: 21_000,
                cost_wei: 21_000 * 2_000_000_000,
                succeeded: false,
            },
            receipt,
            canonical: canonical.clone(),
            finalized: canonical.clone(),
        })?;
        assert!(journal.state().publications.len() <= 45);
    }
    journal.compact()?;
    let pending = pending.ok_or("pending publication")?;
    assert_eq!(journal.state().publications.len(), 1);
    assert_eq!(journal.state().publication_gas(0), 192 * 21_000 + 60_000);
    assert_eq!(
        journal.state().publication_wei(0),
        192 * 21_000 * 2_000_000_000
    );
    assert_eq!(journal.state().reserved_wei(), 60_000 * 3_000_000_000);
    assert_eq!(
        journal.state().rate_transactions[&pending.publication.hash],
        pending
    );
    let before = journal.state().clone();
    drop(journal);
    let mut journal = Journal::open(&runtime.journal())?;
    assert_eq!(journal.state(), &before);
    assert_eq!(
        journal.append(&Entry::RatePrepared {
            transaction: pending.clone()
        }),
        Ok(())
    );
    let retired_nonce = 100;
    let envelope =
        layerx_gas_station::tx::sign_call(1325, retired_nonce, fees, PAYMASTER, &data, &signer)?;
    let mut replay = pending.clone();
    replay.publication.nonce = retired_nonce;
    replay.publication.hash = envelope.hash;
    replay.publication.signed_at = 2000;
    replay.raw = envelope.raw;
    assert_eq!(
        journal.append(&Entry::RatePrepared {
            transaction: replay
        }),
        Err(JournalError::Conflict)
    );
    assert_eq!(journal.state(), &before);
    Ok(())
}
