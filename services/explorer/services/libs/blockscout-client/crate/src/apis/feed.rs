use crate::models;
use serde::{Deserialize, Deserializer, Serialize};

/// The transaction feed and homepage share the producer's type discriminator.
#[derive(Debug, Clone, Serialize)]
#[serde(untagged)]
pub enum FeedItem {
    Evm(Box<models::Transaction>),
    Native(Box<NativePriceUpdate>),
}

impl<'de> Deserialize<'de> for FeedItem {
    fn deserialize<D: Deserializer<'de>>(deserializer: D) -> Result<Self, D::Error> {
        let value = serde_json::Value::deserialize(deserializer)?;
        let is_native = value
            .get("type")
            .is_some_and(|kind| kind.as_u64() == Some(127) || kind.as_str() == Some("127"));
        if is_native {
            serde_json::from_value(value)
                .map(Self::Native)
                .map_err(serde::de::Error::custom)
        } else {
            serde_json::from_value(value)
                .map(Self::Evm)
                .map_err(serde::de::Error::custom)
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionFeedPage {
    pub items: Vec<FeedItem>,
    pub next_page_params: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NativePriceUpdate {
    pub hash: String,
    pub block_number: u64,
    pub block_hash: String,
    pub timestamp: String,
    pub index: u64,
    pub from: NativeFeeder,
    pub to: (),
    pub value: ZeroValue,
    pub fee: NativeFee,
    pub gas_used: String,
    pub gas_limit: String,
    pub status: NativeStatus,
    pub result: NativeResult,
    pub method: NativeMethod,
    pub transaction_types: Vec<NativeActivity>,
    pub r#type: NativeType,
    pub confirmations: u64,
    pub price_update: PriceUpdate,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NativeFeeder {
    pub hash: String,
    pub is_contract: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NativeFee {
    pub r#type: ActualFee,
    pub value: ZeroValue,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ZeroValue {
    #[serde(rename = "0")]
    Zero,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ActualFee {
    #[serde(rename = "actual")]
    Actual,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum NativeStatus {
    Ok,
    Error,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum NativeResult {
    Success,
    Failed,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum NativeMethod {
    #[serde(rename = "submitPrices")]
    SubmitPrices,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum NativeActivity {
    #[serde(rename = "price_update")]
    PriceUpdate,
}

#[derive(Debug, Clone, serde_repr::Serialize_repr, serde_repr::Deserialize_repr)]
#[repr(u8)]
pub enum NativeType {
    PriceUpdate = 127,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceUpdate {
    pub feeder_bech32: String,
    pub observation_count: u64,
    pub observations: Vec<PriceObservation>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceObservation {
    pub market_id: u64,
    pub accepted: bool,
    pub symbol: Option<String>,
    pub index_price: String,
    pub mark_price: String,
    pub source_timestamp_ms: u64,
    pub sequence: u64,
}
