use std::sync::Mutex;

use layerx_gas_station::rate::PublisherConfig;
use layerx_gas_station::signer::LocalSigner;
use serde_json::Value;

pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error>>;

pub struct Tape {
    pub fixture: Value,
}

impl Tape {
    pub fn load() -> Result<Self> {
        Ok(Self {
            fixture: serde_json::from_str(include_str!("../fixtures/producer_22_33_71.json"))?,
        })
    }

    pub fn config(&self) -> Result<PublisherConfig> {
        Ok(PublisherConfig::parse(
            &self.fixture["publisher_config"].to_string(),
        )?)
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
