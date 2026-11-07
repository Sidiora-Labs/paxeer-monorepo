use std::path::PathBuf;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use layerx_gas_station::journal::{Journal, PublicationTransaction};
use layerx_gas_station::quote::Word;
use layerx_gas_station::rate::{PublisherConfig, RatePublisher};
use layerx_gas_station::rpc::{bytes, hex, ConfiguredRpc, Exchange, RpcFault};
use layerx_gas_station::signer::LocalSigner;
use serde_json::{json, Value};

pub type Result<T = ()> = std::result::Result<T, Box<dyn std::error::Error>>;

#[derive(Clone)]
pub struct Tape {
    pub fixture: Arc<Value>,
    state: Arc<Mutex<State>>,
}
struct State {
    phase: String,
    sent: Vec<Vec<u8>>,
    withheld: bool,
    tamper: Option<&'static str>,
}
impl Tape {
    pub fn load(phase: &str) -> Result<Self> {
        let fixture = serde_json::from_str(include_str!("../fixtures/producer_22_33_71.json"))?;
        Ok(Self {
            fixture: Arc::new(fixture),
            state: Arc::new(Mutex::new(State {
                phase: phase.into(),
                sent: vec![],
                withheld: false,
                tamper: None,
            })),
        })
    }
    pub fn phase(&self, phase: &str) {
        self.state.lock().unwrap().phase = phase.into();
    }
    pub fn withhold(&self, value: bool) {
        self.state.lock().unwrap().withheld = value;
    }
    pub fn tamper(&self, field: Option<&'static str>) {
        self.state.lock().unwrap().tamper = field;
    }
    pub fn sent(&self) -> Vec<Vec<u8>> {
        self.state.lock().unwrap().sent.clone()
    }
    pub fn config(&self) -> Result<PublisherConfig> {
        Ok(PublisherConfig::parse(
            &self.fixture["publisher_config"].to_string(),
        )?)
    }
    pub fn transaction(&self, name: &str) -> Result<PublicationTransaction> {
        Ok(serde_json::from_value(self.fixture[name].clone())?)
    }
    pub fn answer(&self, phase: &str, method: &str, params: &Value) -> Result<Value> {
        let rules = self.fixture["phases"][phase]
            .as_array()
            .ok_or("missing capture phase")?;
        let rule = rules
            .iter()
            .rev()
            .find(|r| r["method"] == method && r["params"] == *params)
            .ok_or_else(|| format!("unrecorded {phase}: {method} {params}"))?;
        if let Some(code) = rule["error"]["code"].as_i64() {
            return Err(RpcFault::Rejected { code }.into());
        }
        Ok(rule["result"].clone())
    }
    pub fn receipt(&self, phase: &str, hash: &Word) -> Result<Value> {
        self.answer(phase, "pax_sequencerReceipt", &json!([hex(hash)]))
    }
}
impl Exchange for Tape {
    fn request_until(
        &self,
        endpoint: &str,
        method: &str,
        params: &Value,
        deadline: Instant,
    ) -> std::result::Result<Value, RpcFault> {
        if method == "pax_sequencerReceipt"
            && self
                .state
                .lock()
                .map_err(|_| RpcFault::Unavailable)?
                .withheld
        {
            std::thread::sleep(deadline.saturating_duration_since(Instant::now()));
            return Err(RpcFault::Unavailable);
        }
        if Instant::now() >= deadline {
            return Err(RpcFault::Unavailable);
        }
        self.request(endpoint, method, params)
    }
    fn request(
        &self,
        _: &str,
        method: &str,
        params: &Value,
    ) -> std::result::Result<Value, RpcFault> {
        let mut state = self.state.lock().map_err(|_| RpcFault::Unavailable)?;
        let phase = state.phase.clone();
        let rule = self.fixture["phases"][&phase]
            .as_array()
            .ok_or(RpcFault::Malformed)?
            .iter()
            .rev()
            .find(|r| r["method"] == method && r["params"] == *params)
            .unwrap_or_else(|| panic!("unrecorded {phase}: {method} {params}"));
        if let Some(code) = rule["error"]["code"].as_i64() {
            return Err(RpcFault::Rejected { code });
        }
        let mut result = rule["result"].clone();
        if method == "eth_sendRawTransaction" {
            state
                .sent
                .push(bytes(params[0].as_str().ok_or(RpcFault::Malformed)?)?);
            if phase.ends_with("_pre") {
                state.phase = phase.replace("_pre", "_receipted");
            }
        }
        if method == "pax_sequencerReceipt" && result["receipt"].is_string() {
            match state.tamper {
                Some("gas") => result["gasUsed"] = json!("0x1"),
                Some("status") => result["status"] = json!("0x0"),
                Some("time") => {
                    let mut data = bytes(
                        result["logs"][0]["data"]
                            .as_str()
                            .ok_or(RpcFault::Malformed)?,
                    )?;
                    *data.last_mut().ok_or(RpcFault::Malformed)? ^= 1;
                    result["logs"][0]["data"] = json!(hex(&data));
                }
                _ => {}
            }
        }
        Ok(result)
    }
}

static SIGNER_ENV: Mutex<()> = Mutex::new(());

pub fn signer(index: u8) -> Result<LocalSigner> {
    let _guard = SIGNER_ENV
        .lock()
        .map_err(|_| "signer environment lock poisoned")?;
    let name = format!("REPAIR_22_33_71_TEST_KEY_{index}");
    std::env::set_var(&name, format!("{index:064x}"));
    let signer = LocalSigner::from_env(&name);
    std::env::remove_var(&name);
    Ok(signer?)
}

pub struct Lane {
    pub tape: Tape,
    pub dir: PathBuf,
}
impl Lane {
    pub fn new(name: &str, phase: &str) -> Result<Self> {
        use std::os::unix::fs::DirBuilderExt;
        let dir =
            std::env::temp_dir().join(format!("gas-station-node-{name}-{}", std::process::id()));
        std::fs::DirBuilder::new().mode(0o700).create(&dir)?;
        Ok(Self {
            tape: Tape::load(phase)?,
            dir,
        })
    }
    pub fn publisher(&self) -> Result<RatePublisher<LocalSigner, ConfiguredRpc<Tape>>> {
        let mut config = self.tape.config()?;
        config.confirmation_retry_seconds = 1;
        let rpc = ConfiguredRpc::new(&config.station, self.tape.clone())?;
        Ok(RatePublisher::new(
            config,
            signer(1)?,
            rpc,
            Journal::open(&self.dir.join("rate.jsonl"))?,
            Duration::from_millis(10),
        )?)
    }
    pub fn file(&self, rate: u128) -> Result<PathBuf> {
        use std::os::unix::fs::PermissionsExt;
        let path = self.dir.join("rate.toml");
        std::fs::write(&path, format!("rate = {rate}\nset_at = 1\n"))?;
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600))?;
        Ok(path)
    }
}
impl Drop for Lane {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.dir);
    }
}
