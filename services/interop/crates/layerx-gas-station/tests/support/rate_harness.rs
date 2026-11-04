use std::cell::RefCell;
use std::path::PathBuf;
use std::rc::Rc;
use std::time::Duration;

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use ed25519_dalek::Signer as _;
use k256::ecdsa::SigningKey;
use k256::elliptic_curve::rand_core::OsRng;
use layerx_gas_station::config::ConfigError;
use layerx_gas_station::journal::{Entry, Journal, Publication};
use layerx_gas_station::quote::{address_word, keccak, word, Address, Word};
use layerx_gas_station::rate::{
    next_wait, PublisherConfig, RateFile, RatePublisher, RateRefusal, RATE_GAS_LIMIT,
};
use layerx_gas_station::rpc::{bytes, hex, ConfiguredRpc, Exchange, RpcFault};
use layerx_gas_station::signer::{LocalSigner, QuoteSigner};
use layerx_gas_station::tx::recover;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::receipt::{decode, encode};
use layerx_wire::WireError;
use serde_json::{json, Value};

type TestResult = Result<(), Box<dyn std::error::Error>>;

const PAYMASTER: Address = [0x44; 20];
const NOW: u64 = 1_900_000_000;

const BLOCK: u64 = 0x10;
const BLOCK_HASH: Word = [0x77; 32];
const FEE_TOKEN: Address = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x18,
];
/// The pinned sequencer receipt key: its keyId, ed25519 seed and the
/// inclusive height range it signs for.
const RECEIPT_KEY_ID: Word = [0x5a; 32];
const RECEIPT_KEY_SEED: [u8; 32] = [0x42; 32];
const RECEIPT_FIRST_HEIGHT: u64 = 1;
const RECEIPT_LAST_HEIGHT: u64 = 0x100;

/// How the sequencer corrupts the signed receipt it serves.
#[derive(Clone, Copy, Debug)]
enum Tamper {
    None,
    Signature,
    UnpinnedKeyId,
    HeightOutsideRange,
    OtherActivity,
    Missing,
    ViewGas,
    ViewStatus,
    ViewUpdatedAt,
}

fn receipt_key() -> ed25519_dalek::SigningKey {
    ed25519_dalek::SigningKey::from_bytes(&RECEIPT_KEY_SEED)
}

fn wire<T>(result: Result<T, WireError>) -> Result<T, Box<dyn std::error::Error>> {
    result.map_err(|error| format!("{error:?}").into())
}

fn account(address: &Address) -> Word {
    let mut word = [0; 32];
    word[12..].copy_from_slice(address);
    word
}

/// The `ProtocolReceipt` v2 the sequencer signs for an EVM transaction
/// (contracts/sequencer.md a1), encoded by the canonical wire encoder and
/// signed by the pinned receipt key unless `tamper` corrupts it.
fn signed_receipt(
    tamper: Tamper,
    activity_id: Word,
    nonce: u64,
    from: Address,
    to: Address,
    status: u64,
    timestamp: u64,
) -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    let (key_id, height, activity_id) = match tamper {
        Tamper::UnpinnedKeyId => ([0x5b; 32], BLOCK, activity_id),
        Tamper::HeightOutsideRange => (RECEIPT_KEY_ID, RECEIPT_LAST_HEIGHT + 1, activity_id),
        Tamper::OtherActivity => (RECEIPT_KEY_ID, BLOCK, word(0xbad)),
        Tamper::None
        | Tamper::Signature
        | Tamper::Missing
        | Tamper::ViewGas
        | Tamper::ViewStatus
        | Tamper::ViewUpdatedAt => (RECEIPT_KEY_ID, BLOCK, activity_id),
    };
    let fields = |signature: Option<[u8; 64]>| -> Result<Vec<u8>, WireError> {
        let mut encoder = Encoder::new(4096);
        encoder.structure_header_version(0x5201, layerx_wire::limits::PROTOCOL_VERSION)?;
        encoder.u16(layerx_wire::limits::PROTOCOL_VERSION)?;
        encoder.bytes(&activity_id, 32)?;
        encoder.u64(nonce + 1)?;
        encoder.bytes(&[0x21; 32], 32)?;
        encoder.bytes(&[0x22; 32], 32)?;
        encoder.bytes(&[0x23; 32], 32)?;
        encoder.i32(i32::from(status != 1))?;
        encoder.sequence_length(0, 512)?;
        encoder.u128(1_500_000_000 * 35_000)?;
        let mut batch_id = [0; 32];
        batch_id[24..].copy_from_slice(&height.to_be_bytes());
        encoder.bytes(&batch_id, 32)?;
        encoder.u16(12)?;
        encoder.u32(1)?;
        encoder.u32(0)?;
        encoder.u8(2)?;
        encoder.bytes(&[0x24; 32], 32)?;
        encoder.u128(0)?;
        encoder.bytes(&account(&from), 32)?;
        encoder.u128(10_u128.pow(18))?;
        encoder.u128(10_u128.pow(18) - 1_500_000_000 * 35_000)?;
        encoder.u64(nonce)?;
        encoder.bytes(&account(&to), 32)?;
        encoder.u128(0)?;
        encoder.u128(0)?;
        encoder.bytes(&[0; 32], 32)?;
        encoder.bytes(&[0; 32], 32)?;
        encoder.bytes(&key_id, 32)?;
        encoder.u64(timestamp * 1_000)?;
        encoder.u8(u8::from(signature.is_some()))?;
        if let Some(signature) = signature {
            encoder.bytes(&signature, 64)?;
        }
        Ok(encoder.finish())
    };
    let digest = wire(receipt_digest(&wire(fields(None))?))?;
    let mut signature = receipt_key().sign(&digest).to_bytes();
    if matches!(tamper, Tamper::Signature) {
        signature[0] ^= 1;
    }
    let signed = wire(fields(Some(signature)))?;
    let canonical = wire(encode(&wire(decode(&signed))?))?;
    assert_eq!(canonical, signed);
    Ok(canonical)
}

/// The paymaster, the owner account and the sequencer as the node reports
/// them: `rung` is how far the sequencer has taken every sent transaction
/// ("unknown", "submitted", "receipted" or "committed"); a receipted
/// transaction advances the pending nonce, a committed one the finalized nonce.
/// `clock` is the chain time of the latest block; a successful setRate sets
/// the paymaster's rate and `updated_at` to the clock once the sequencer
/// receipts it, and `applied` counts the sent transactions it has receipted.
struct Chain {
    clock: u64,
    applied: usize,
    rate: u128,
    updated_at: u64,
    owner: Address,
    balance: u128,
    nonce: u64,
    finalized: u64,
    status: u64,
    gas_used: u64,
    effective_gas_price: u128,
    rung: &'static str,
    sent: Vec<Vec<u8>>,
    nonces: Vec<(Word, u64)>,
    tamper: Tamper,
}
impl Chain {
    fn sequencer_receipt(&self, hash: &Word) -> Result<Value, RpcFault> {
        let Some(nonce) = self.nonce_of(hash) else {
            return Ok(Value::Null);
        };
        Ok(match self.rung {
            "submitted" => json!({"rung": "submitted", "txHash": hex(hash),
                "nonce": quantity(nonce.into())}),
            rung => {
                let raw = self
                    .sent
                    .iter()
                    .find(|raw| keccak(raw) == *hash)
                    .ok_or(RpcFault::Malformed)?;
                let (to, rate, from) = published_rate(raw).map_err(|_| RpcFault::Malformed)?;
                let logs = if self.status == 1 {
                    json!([{"address": hex(&to),
                        "topics": [hex(&keccak(b"RateUpdated(uint256,uint256)"))],
                        "data": hex(&[rate, word(self.clock.into())].concat()),
                        "logIndex": "0x0"}])
                } else {
                    json!([])
                };
                let mut receipt = json!({"rung": rung, "txHash": hex(hash),
                    "seq": quantity(nonce.into()), "height": quantity(BLOCK.into()),
                    "index": "0x1", "timestamp": quantity(self.clock.into()),
                    "status": quantity(self.status.into()), "returnData": "0x",
                    "logs": logs, "gasUsed": quantity(self.gas_used.into()),
                    "cumulativeGasUsed": quantity(self.gas_used.into()),
                    "effectiveGasPrice": quantity(self.effective_gas_price),
                    "from": hex(&from), "to": hex(&to), "contractAddress": Value::Null,
                    "nonce": quantity(nonce.into()), "vmError": ""});
                if rung == "committed" {
                    receipt["blockHash"] = Value::String(hex(&BLOCK_HASH));
                }
                if !matches!(self.tamper, Tamper::Missing) {
                    let signed = signed_receipt(
                        self.tamper,
                        *hash,
                        nonce,
                        from,
                        to,
                        self.status,
                        self.clock,
                    )
                    .map_err(|_| RpcFault::Malformed)?;
                    receipt["keyId"] = Value::String(hex(&RECEIPT_KEY_ID));
                    receipt["receipt"] = Value::String(hex(&signed));
                }
                // Alter only the unsigned view: the signed receipt remains byte
                // identical to the unmodified fixture for these refusal cases.
                match self.tamper {
                    Tamper::ViewGas => receipt["gasUsed"] = quantity(u128::from(self.gas_used + 1)),
                    Tamper::ViewStatus => receipt["status"] = quantity(u128::from(1 - self.status)),
                    Tamper::ViewUpdatedAt => {
                        receipt["logs"][0]["data"] =
                            json!(hex(&[rate, word(u128::from(self.clock + 1))].concat()));
                    }
                    _ => {}
                }
                receipt
            }
        })
    }

    fn executed(&self) -> Option<u64> {
        matches!(self.rung, "receipted" | "committed")
            .then(|| self.nonces.iter().map(|(_, n)| n + 1).max())
            .flatten()
    }
    fn nonce_of(&self, hash: &Word) -> Option<u64> {
        self.nonces
            .iter()
            .find_map(|(h, n)| (h == hash).then_some(*n))
    }
    /// Applies every accepted setRate the sequencer has receipted since the
    /// last request, as the paymaster does: a successful one sets the rate and
    /// stamps `updated_at` with the clock.
    fn receipt_sent_rates(&mut self) -> Result<(), RpcFault> {
        if !matches!(self.rung, "receipted" | "committed") {
            return Ok(());
        }
        while let Some((hash, _)) = self.nonces.get(self.applied).copied() {
            self.applied += 1;
            if self.status != 1 {
                continue;
            }
            let raw = self
                .sent
                .iter()
                .find(|raw| keccak(raw) == hash)
                .ok_or(RpcFault::Malformed)?;
            let (_, rate, _) = published_rate(raw).map_err(|_| RpcFault::Malformed)?;
            self.rate =
                u128::from_be_bytes(rate[16..].try_into().map_err(|_| RpcFault::Malformed)?);
            self.updated_at = self.clock;
        }
        Ok(())
    }
}
fn quantity(v: u128) -> Value {
    Value::String(format!("{v:#x}"))
}

struct ChainExchange(Rc<RefCell<Chain>>);
impl Exchange for ChainExchange {
    fn request_until(
        &self,
        endpoint: &str,
        method: &str,
        params: &Value,
        _: std::time::Instant,
    ) -> Result<Value, RpcFault> {
        self.request(endpoint, method, params)
    }

    fn request(&self, _: &str, method: &str, params: &Value) -> Result<Value, RpcFault> {
        let mut chain = self.0.borrow_mut();
        chain.receipt_sent_rates()?;
        let hash_param = || -> Result<Word, RpcFault> {
            bytes(params[0].as_str().ok_or(RpcFault::Malformed)?)?
                .try_into()
                .map_err(|_| RpcFault::Malformed)
        };
        Ok(match method {
            "eth_chainId" => quantity(1325),
            "eth_getBlockByNumber" => match params[0].as_str() {
                Some("latest") => {
                    json!({"timestamp": quantity(chain.clock.into()), "baseFeePerGas": quantity(1_000_000_000)})
                }
                Some("finalized" | "0x10") => {
                    json!({"number": quantity(BLOCK.into()), "hash": hex(&BLOCK_HASH)})
                }
                _ => return Err(RpcFault::Rejected { code: -32602 }),
            },
            "eth_call" => {
                assert_eq!(params[0]["to"], hex(&PAYMASTER));
                let selector = |name: &[u8]| hex(&keccak(name)[..4]);
                let data = &params[0]["data"];
                let value = if *data == selector(b"rate()") {
                    word(chain.rate)
                } else if *data == selector(b"rateUpdatedAt()") {
                    word(chain.updated_at.into())
                } else if *data == selector(b"owner()") {
                    address_word(chain.owner)
                } else {
                    return Err(RpcFault::Rejected { code: 3 });
                };
                Value::String(hex(&value))
            }
            "eth_getBalance" => quantity(chain.balance),
            "eth_getTransactionCount" => {
                let executed = chain.executed().unwrap_or(0);
                match params[1].as_str() {
                    Some("pending") => quantity(chain.nonce.max(executed).into()),
                    Some("finalized") if chain.rung == "committed" => {
                        quantity(chain.finalized.max(executed).into())
                    }
                    Some("finalized") => quantity(chain.finalized.into()),
                    _ => return Err(RpcFault::Rejected { code: -32602 }),
                }
            }
            "eth_sendRawTransaction" => {
                let raw = bytes(params[0].as_str().ok_or(RpcFault::Malformed)?)?;
                let hash = keccak(&raw);
                chain.sent.push(raw);
                if chain.rung == "unknown" {
                    return Err(RpcFault::Rejected { code: -32000 });
                }
                if chain.nonce_of(&hash).is_none() {
                    let nonce = chain.nonce.max(chain.executed().unwrap_or(0));
                    chain.nonces.push((hash, nonce));
                }
                Value::String(hex(&hash))
            }
            "pax_sequencerReceipt" => chain.sequencer_receipt(&hash_param()?)?,
            "eth_getTransactionReceipt" => {
                let hash = hash_param()?;
                if chain.rung != "committed" || chain.nonce_of(&hash).is_none() {
                    return Ok(Value::Null);
                }
                json!({"transactionHash": hex(&hash), "from": hex(&chain.owner), "to": hex(&PAYMASTER),
                    "blockNumber": quantity(BLOCK.into()), "blockHash": hex(&BLOCK_HASH),
                    "gasUsed": quantity(chain.gas_used.into()),
                    "effectiveGasPrice": quantity(chain.effective_gas_price),
                    "status": quantity(chain.status.into())})
            }
            _ => return Err(RpcFault::Rejected { code: -32601 }),
        })
    }
}

struct Lane {
    dir: PathBuf,
    chain: Rc<RefCell<Chain>>,
    owner: Address,
    key: Vec<u8>,
}
impl Lane {
    fn new(name: &str) -> Result<Self, Box<dyn std::error::Error>> {
        let dir = std::env::temp_dir().join(format!("paxeer-rate-{name}-{}", std::process::id()));
        if dir.exists() {
            std::fs::remove_dir_all(&dir)?;
        }
        std::fs::create_dir_all(&dir)?;
        let key = SigningKey::random(&mut OsRng).to_bytes().to_vec();
        let owner = signer(&key)?.address();
        Ok(Self {
            dir,
            chain: Rc::new(RefCell::new(Chain {
                clock: NOW,
                applied: 0,
                rate: 3_114_000,
                updated_at: NOW - 120,
                owner,
                balance: 10_u128.pow(18),
                nonce: 7,
                finalized: 7,
                status: 1,
                gas_used: 35_000,
                effective_gas_price: 1_500_000_000,
                rung: "committed",
                sent: Vec::new(),
                nonces: Vec::new(),
                tamper: Tamper::None,
            })),
            owner,
            key,
        })
    }
    fn rate_file(&self, text: &str) -> Result<PathBuf, std::io::Error> {
        use std::os::unix::fs::PermissionsExt as _;
        let path = self.dir.join("rate.toml");
        std::fs::write(&path, text)?;
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600))?;
        Ok(path)
    }
    fn publisher(
        &self,
        budget: u128,
    ) -> Result<RatePublisher<LocalSigner, ConfiguredRpc<ChainExchange>>, Box<dyn std::error::Error>>
    {
        self.publisher_with(budget, 5_000_000_000)
    }
    fn publisher_with(
        &self,
        budget: u128,
        ceiling: u128,
    ) -> Result<RatePublisher<LocalSigner, ConfiguredRpc<ChainExchange>>, Box<dyn std::error::Error>>
    {
        let mut value = config_json();
        value["rate_gas_budget_per_day"] = json!(budget);
        value["rate_max_fee_per_gas"] = json!(ceiling);
        self.publisher_from(&value, Duration::ZERO)
    }
    /// A publisher that waits `retry` seconds for a sequencer receipt,
    /// polling every 10 ms.
    fn publisher_confirming_within(
        &self,
        retry: u64,
    ) -> Result<RatePublisher<LocalSigner, ConfiguredRpc<ChainExchange>>, Box<dyn std::error::Error>>
    {
        let mut value = config_json();
        value["rate_confirmation_retry_seconds"] = json!(retry);
        self.publisher_from(&value, Duration::from_millis(10))
    }
    fn publisher_from(
        &self,
        value: &Value,
        poll: Duration,
    ) -> Result<RatePublisher<LocalSigner, ConfiguredRpc<ChainExchange>>, Box<dyn std::error::Error>>
    {
        let config = PublisherConfig::parse(&value.to_string())?;
        let rpc = ConfiguredRpc::new(&config.station, ChainExchange(self.chain.clone()))?;
        let journal = Journal::open(&self.dir.join("rate.jsonl"))?;
        Ok(RatePublisher::new(
            config,
            signer(&self.key)?,
            rpc,
            journal,
            poll,
        )?)
    }
    fn sent(&self) -> usize {
        self.chain.borrow().sent.len()
    }
}
impl Drop for Lane {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.dir);
    }
}

fn signer(key: &[u8]) -> Result<LocalSigner, Box<dyn std::error::Error>> {
    let name = format!(
        "PAXEER_RATE_TEST_KEY_{}",
        hex(&keccak(key)[..8])[2..].to_uppercase()
    );
    std::env::set_var(&name, hex(key));
    let signer = LocalSigner::from_env(&name);
    std::env::remove_var(&name);
    Ok(signer?)
}

fn config_json() -> Value {
    json!({"chain_id":1325,"endpoints":["https://paxeer.app"],
        "paymaster":hex(&PAYMASTER),
        "token":"0x21f7b20a555199fa73A238B1a91FD0f549068fEe","decimals":6,
        "max_rate_age":300,"spread_bps":500,"margin_bps":100,"per_account_limit":4_000_000,
        "per_interval_limit":8_000_000,"per_quote_limit":3_000_000,"interval_seconds":60,
        "balance_floor":100,"relayer_key_env":"GAS_STATION_RELAYER_KEY",
        "max_priority_fee_per_gas":1_000_000_000,"rate_owner_key_env":"GAS_STATION_RATE_OWNER_KEY",
        "rate_cadence_seconds":120,"rate_gas_budget_per_day":1_000_000,
        "rate_balance_floor":10_u64.pow(15),"rate_max_fee_per_gas":5_000_000_000_u64,
        "rate_confirmation_retry_seconds":60,
        "sequencer_receipt_keys":[receipt_key_entry(RECEIPT_KEY_ID, RECEIPT_FIRST_HEIGHT, RECEIPT_LAST_HEIGHT)]})
}

/// One pinned key in the shape `px_getReceiptKeys` returns.
fn receipt_key_entry(key_id: Word, first: u64, last: u64) -> Value {
    json!({"keyId": STANDARD.encode(key_id),
        "publicKey": STANDARD.encode(receipt_key().verifying_key().to_bytes()),
        "firstHeight": first.to_string(), "lastHeight": last.to_string()})
}

/// The paymaster, the rate and the recovered signer of a type-2 setRate
/// envelope: 0x02, the list header, the fields up to the data, an empty
/// access list, then y parity, r and s.
fn published_rate(raw: &[u8]) -> Result<(Address, Word, Address), Box<dyn std::error::Error>> {
    assert_eq!(&raw[..2], &[2, 0xf8]);
    let selector = &keccak(b"setRate(uint256)")[..4];
    let at = raw
        .windows(4)
        .position(|w| w == selector)
        .ok_or("setRate selector absent")?;
    assert_eq!((raw[at - 23], raw[at - 2], raw[at - 1]), (0x94, 0x80, 0xa4));
    let to: Address = raw[at - 22..at - 2].try_into()?;
    let rate: Word = raw[at + 4..at + 36].try_into()?;
    assert_eq!(raw[at + 36], 0xc0);
    let fields = &raw[3..at + 37];
    let mut signature = [0_u8; 65];
    let mut cursor = at + 37;
    signature[64] = match raw[cursor] {
        0x80 => 27,
        1 => 28,
        _ => return Err("invalid parity".into()),
    };
    for end in [32, 64] {
        cursor += 1;
        let length = usize::from(raw[cursor] - 0x80);
        signature[end - length..end].copy_from_slice(&raw[cursor + 1..cursor + 1 + length]);
        cursor += length;
    }
    assert_eq!(cursor + 1, raw.len());
    let mut unsigned = vec![2, 0xf8, u8::try_from(fields.len())?];
    unsigned.extend_from_slice(fields);
    Ok((to, rate, recover(keccak(&unsigned), &signature)?))
}
