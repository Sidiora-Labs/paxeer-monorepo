use std::{error, fmt};

#[derive(Debug, Clone)]
pub struct ResponseContent<T> {
    pub status: reqwest::StatusCode,
    pub content: String,
    pub entity: Option<T>,
}

#[derive(Debug)]
pub enum Error<T> {
    Reqwest(reqwest::Error),

    ReqwestMiddleware(reqwest_middleware::Error),

    Serde(serde_json::Error),
    Io(std::io::Error),
    ResponseError(ResponseContent<T>),
    Configuration(&'static str),
    UnsupportedCapability(&'static str),
    ResponseTooLarge {
        status: reqwest::StatusCode,
        content: Vec<u8>,
        limit: usize,
    },
    Decode {
        route: &'static str,
        status: reqwest::StatusCode,
        content: String,
        source: serde_json::Error,
    },
}

impl<T> fmt::Display for Error<T> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let (module, e) = match self {
            Error::Configuration(message) => ("configuration", message.to_string()),
            Error::UnsupportedCapability(message) => ("capability", message.to_string()),
            Error::ResponseTooLarge { status, limit, .. } => (
                "response",
                format!("status {status}, body exceeds {limit} bytes"),
            ),
            Error::Decode {
                route,
                status,
                source,
                ..
            } => (
                "decode",
                format!(
                    "{route}: status {status}: {:?} at line {}, column {}",
                    source.classify(),
                    source.line(),
                    source.column()
                ),
            ),
            Error::Reqwest(e) => ("reqwest", e.to_string()),

            Error::ReqwestMiddleware(e) => ("reqwest-middleware", e.to_string()),

            Error::Serde(e) => ("serde", e.to_string()),
            Error::Io(e) => ("IO", e.to_string()),
            Error::ResponseError(e) => ("response", format!("status code {}", e.status)),
        };
        write!(f, "error in {module}: {e}")
    }
}

impl<T: fmt::Debug> error::Error for Error<T> {
    fn source(&self) -> Option<&(dyn error::Error + 'static)> {
        Some(match self {
            Error::Reqwest(e) => e,

            Error::ReqwestMiddleware(e) => e,

            Error::Serde(e) => e,
            Error::Io(e) => e,
            Error::Decode { source, .. } => source,
            Error::ResponseError(_)
            | Error::Configuration(_)
            | Error::UnsupportedCapability(_)
            | Error::ResponseTooLarge { .. } => return None,
        })
    }
}

impl<T> From<reqwest::Error> for Error<T> {
    fn from(e: reqwest::Error) -> Self {
        Error::Reqwest(e)
    }
}

impl<T> From<reqwest_middleware::Error> for Error<T> {
    fn from(e: reqwest_middleware::Error) -> Self {
        Error::ReqwestMiddleware(e)
    }
}

impl<T> From<serde_json::Error> for Error<T> {
    fn from(e: serde_json::Error) -> Self {
        Error::Serde(e)
    }
}

impl<T> From<std::io::Error> for Error<T> {
    fn from(e: std::io::Error) -> Self {
        Error::Io(e)
    }
}

pub fn urlencode<T: AsRef<str>>(s: T) -> String {
    ::url::form_urlencoded::byte_serialize(s.as_ref().as_bytes()).collect()
}

/// Encode bracketed keys and indexed arrays, including scalar and null leaves.
pub fn parse_deep_object(prefix: &str, value: &serde_json::Value) -> Vec<(String, String)> {
    let mut pending = vec![(prefix.to_owned(), value)];
    let mut params = Vec::new();
    while let Some((key, value)) = pending.pop() {
        match value {
            serde_json::Value::Object(object) => {
                for (name, value) in object.iter().rev() {
                    pending.push((format!("{key}[{name}]"), value));
                }
            }
            serde_json::Value::Array(array) => {
                for (i, value) in array.iter().enumerate().rev() {
                    pending.push((format!("{key}[{i}]"), value));
                }
            }
            serde_json::Value::String(value) => params.push((key, value.clone())),
            _ => params.push((key, value.to_string())),
        }
    }
    params
}

/// Preserve at most `limit` response bytes, including error evidence. Request
/// deadlines remain attached to the reqwest response while its body is read.
pub(crate) async fn read_bounded<T>(
    mut response: reqwest::Response,
    limit: usize,
) -> Result<String, Error<T>> {
    let status = response.status();
    let mut body = Vec::new();
    while let Some(chunk) = response.chunk().await? {
        let remaining = limit.saturating_sub(body.len());
        body.extend_from_slice(&chunk[..chunk.len().min(remaining)]);
        if chunk.len() > remaining {
            return Err(Error::ResponseTooLarge {
                status,
                content: body,
                limit,
            });
        }
    }
    String::from_utf8(body)
        .map_err(|error| Error::Io(std::io::Error::new(std::io::ErrorKind::InvalidData, error)))
}

/// Decode only the declared HTTP 200 model; raw body and status remain available
/// in ResponseError / Decode for callers that need to inspect transport evidence.
pub(crate) fn decode_success<M, S, E>(
    status: reqwest::StatusCode,
    content: String,
    route: &'static str,
    wrap: impl FnOnce(Box<M>) -> S,
) -> Result<ResponseContent<S>, Error<E>>
where
    M: serde::de::DeserializeOwned,
    E: serde::de::DeserializeOwned,
{
    if status != reqwest::StatusCode::OK {
        let entity = serde_json::from_str(&content).ok();
        return Err(Error::ResponseError(ResponseContent {
            status,
            content,
            entity,
        }));
    }
    let model = serde_json::from_str::<M>(&content).map_err(|source| Error::Decode {
        route,
        status,
        content: content.clone(),
        source,
    })?;
    Ok(ResponseContent {
        status,
        content,
        entity: Some(wrap(Box::new(model))),
    })
}

pub mod addresses_api;

pub mod blocks_api;

pub mod celestia_service_api;

pub mod config_api;

pub mod health_api;

pub mod internal_transactions_api;

pub mod main_page_api;

pub mod proxy_api;

pub mod search_api;

pub mod smart_contracts_api;

pub mod stats_api;

pub mod token_transfers_api;

pub mod tokens_api;

pub mod transactions_api;

pub mod withdrawals_api;

pub mod configuration;

use std::sync::Arc;

pub trait Api {
    fn addresses_api(&self) -> &dyn addresses_api::AddressesApi;

    fn blocks_api(&self) -> &dyn blocks_api::BlocksApi;

    fn celestia_service_api(&self) -> &dyn celestia_service_api::CelestiaServiceApi;

    fn config_api(&self) -> &dyn config_api::ConfigApi;

    fn health_api(&self) -> &dyn health_api::HealthApi;

    fn internal_transactions_api(&self) -> &dyn internal_transactions_api::InternalTransactionsApi;

    fn main_page_api(&self) -> &dyn main_page_api::MainPageApi;

    fn proxy_api(&self) -> &dyn proxy_api::ProxyApi;

    fn search_api(&self) -> &dyn search_api::SearchApi;

    fn smart_contracts_api(&self) -> &dyn smart_contracts_api::SmartContractsApi;

    fn stats_api(&self) -> &dyn stats_api::StatsApi;

    fn token_transfers_api(&self) -> &dyn token_transfers_api::TokenTransfersApi;

    fn tokens_api(&self) -> &dyn tokens_api::TokensApi;

    fn transactions_api(&self) -> &dyn transactions_api::TransactionsApi;

    fn withdrawals_api(&self) -> &dyn withdrawals_api::WithdrawalsApi;
}

pub struct ApiClient {
    addresses_api: Box<dyn addresses_api::AddressesApi>,

    blocks_api: Box<dyn blocks_api::BlocksApi>,

    celestia_service_api: Box<dyn celestia_service_api::CelestiaServiceApi>,

    config_api: Box<dyn config_api::ConfigApi>,

    health_api: Box<dyn health_api::HealthApi>,

    internal_transactions_api: Box<dyn internal_transactions_api::InternalTransactionsApi>,

    main_page_api: Box<dyn main_page_api::MainPageApi>,

    proxy_api: Box<dyn proxy_api::ProxyApi>,

    search_api: Box<dyn search_api::SearchApi>,

    smart_contracts_api: Box<dyn smart_contracts_api::SmartContractsApi>,

    stats_api: Box<dyn stats_api::StatsApi>,

    token_transfers_api: Box<dyn token_transfers_api::TokenTransfersApi>,

    tokens_api: Box<dyn tokens_api::TokensApi>,

    transactions_api: Box<dyn transactions_api::TransactionsApi>,

    withdrawals_api: Box<dyn withdrawals_api::WithdrawalsApi>,
}

impl ApiClient {
    // changed
    pub fn new_arc(configuration: Arc<configuration::Configuration>) -> Self {
        Self {
            addresses_api: Box::new(addresses_api::AddressesApiClient::new(
                configuration.clone(),
            )),

            blocks_api: Box::new(blocks_api::BlocksApiClient::new(configuration.clone())),

            celestia_service_api: Box::new(celestia_service_api::CelestiaServiceApiClient::new(
                configuration.clone(),
            )),

            config_api: Box::new(config_api::ConfigApiClient::new(configuration.clone())),

            health_api: Box::new(health_api::HealthApiClient::new(configuration.clone())),

            internal_transactions_api: Box::new(
                internal_transactions_api::InternalTransactionsApiClient::new(
                    configuration.clone(),
                ),
            ),

            main_page_api: Box::new(main_page_api::MainPageApiClient::new(configuration.clone())),

            proxy_api: Box::new(proxy_api::ProxyApiClient::new(configuration.clone())),

            search_api: Box::new(search_api::SearchApiClient::new(configuration.clone())),

            smart_contracts_api: Box::new(smart_contracts_api::SmartContractsApiClient::new(
                configuration.clone(),
            )),

            stats_api: Box::new(stats_api::StatsApiClient::new(configuration.clone())),

            token_transfers_api: Box::new(token_transfers_api::TokenTransfersApiClient::new(
                configuration.clone(),
            )),

            tokens_api: Box::new(tokens_api::TokensApiClient::new(configuration.clone())),

            transactions_api: Box::new(transactions_api::TransactionsApiClient::new(
                configuration.clone(),
            )),

            withdrawals_api: Box::new(withdrawals_api::WithdrawalsApiClient::new(
                configuration.clone(),
            )),
        }
    }
    // changed
    pub fn new(configuration: configuration::Configuration) -> Self {
        Self::new_arc(Arc::new(configuration))
    }
}

impl Api for ApiClient {
    fn addresses_api(&self) -> &dyn addresses_api::AddressesApi {
        self.addresses_api.as_ref()
    }

    fn blocks_api(&self) -> &dyn blocks_api::BlocksApi {
        self.blocks_api.as_ref()
    }

    fn celestia_service_api(&self) -> &dyn celestia_service_api::CelestiaServiceApi {
        self.celestia_service_api.as_ref()
    }

    fn config_api(&self) -> &dyn config_api::ConfigApi {
        self.config_api.as_ref()
    }

    fn health_api(&self) -> &dyn health_api::HealthApi {
        self.health_api.as_ref()
    }

    fn internal_transactions_api(&self) -> &dyn internal_transactions_api::InternalTransactionsApi {
        self.internal_transactions_api.as_ref()
    }

    fn main_page_api(&self) -> &dyn main_page_api::MainPageApi {
        self.main_page_api.as_ref()
    }

    fn proxy_api(&self) -> &dyn proxy_api::ProxyApi {
        self.proxy_api.as_ref()
    }

    fn search_api(&self) -> &dyn search_api::SearchApi {
        self.search_api.as_ref()
    }

    fn smart_contracts_api(&self) -> &dyn smart_contracts_api::SmartContractsApi {
        self.smart_contracts_api.as_ref()
    }

    fn stats_api(&self) -> &dyn stats_api::StatsApi {
        self.stats_api.as_ref()
    }

    fn token_transfers_api(&self) -> &dyn token_transfers_api::TokenTransfersApi {
        self.token_transfers_api.as_ref()
    }

    fn tokens_api(&self) -> &dyn tokens_api::TokensApi {
        self.tokens_api.as_ref()
    }

    fn transactions_api(&self) -> &dyn transactions_api::TransactionsApi {
        self.transactions_api.as_ref()
    }

    fn withdrawals_api(&self) -> &dyn withdrawals_api::WithdrawalsApi {
        self.withdrawals_api.as_ref()
    }
}

#[cfg(feature = "mockall")]
pub struct MockApiClient {
    pub addresses_api_mock: addresses_api::MockAddressesApi,

    pub blocks_api_mock: blocks_api::MockBlocksApi,

    pub celestia_service_api_mock: celestia_service_api::MockCelestiaServiceApi,

    pub config_api_mock: config_api::MockConfigApi,

    pub health_api_mock: health_api::MockHealthApi,

    pub internal_transactions_api_mock: internal_transactions_api::MockInternalTransactionsApi,

    pub main_page_api_mock: main_page_api::MockMainPageApi,

    pub proxy_api_mock: proxy_api::MockProxyApi,

    pub search_api_mock: search_api::MockSearchApi,

    pub smart_contracts_api_mock: smart_contracts_api::MockSmartContractsApi,

    pub stats_api_mock: stats_api::MockStatsApi,

    pub token_transfers_api_mock: token_transfers_api::MockTokenTransfersApi,

    pub tokens_api_mock: tokens_api::MockTokensApi,

    pub transactions_api_mock: transactions_api::MockTransactionsApi,

    pub withdrawals_api_mock: withdrawals_api::MockWithdrawalsApi,
}

#[cfg(feature = "mockall")]
impl MockApiClient {
    pub fn new() -> Self {
        Self {
            addresses_api_mock: addresses_api::MockAddressesApi::new(),

            blocks_api_mock: blocks_api::MockBlocksApi::new(),

            celestia_service_api_mock: celestia_service_api::MockCelestiaServiceApi::new(),

            config_api_mock: config_api::MockConfigApi::new(),

            health_api_mock: health_api::MockHealthApi::new(),

            internal_transactions_api_mock:
                internal_transactions_api::MockInternalTransactionsApi::new(),

            main_page_api_mock: main_page_api::MockMainPageApi::new(),

            proxy_api_mock: proxy_api::MockProxyApi::new(),

            search_api_mock: search_api::MockSearchApi::new(),

            smart_contracts_api_mock: smart_contracts_api::MockSmartContractsApi::new(),

            stats_api_mock: stats_api::MockStatsApi::new(),

            token_transfers_api_mock: token_transfers_api::MockTokenTransfersApi::new(),

            tokens_api_mock: tokens_api::MockTokensApi::new(),

            transactions_api_mock: transactions_api::MockTransactionsApi::new(),

            withdrawals_api_mock: withdrawals_api::MockWithdrawalsApi::new(),
        }
    }
}

#[cfg(feature = "mockall")]
impl Default for MockApiClient {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(feature = "mockall")]
impl Api for MockApiClient {
    fn addresses_api(&self) -> &dyn addresses_api::AddressesApi {
        &self.addresses_api_mock
    }

    fn blocks_api(&self) -> &dyn blocks_api::BlocksApi {
        &self.blocks_api_mock
    }

    fn celestia_service_api(&self) -> &dyn celestia_service_api::CelestiaServiceApi {
        &self.celestia_service_api_mock
    }

    fn config_api(&self) -> &dyn config_api::ConfigApi {
        &self.config_api_mock
    }

    fn health_api(&self) -> &dyn health_api::HealthApi {
        &self.health_api_mock
    }

    fn internal_transactions_api(&self) -> &dyn internal_transactions_api::InternalTransactionsApi {
        &self.internal_transactions_api_mock
    }

    fn main_page_api(&self) -> &dyn main_page_api::MainPageApi {
        &self.main_page_api_mock
    }

    fn proxy_api(&self) -> &dyn proxy_api::ProxyApi {
        &self.proxy_api_mock
    }

    fn search_api(&self) -> &dyn search_api::SearchApi {
        &self.search_api_mock
    }

    fn smart_contracts_api(&self) -> &dyn smart_contracts_api::SmartContractsApi {
        &self.smart_contracts_api_mock
    }

    fn stats_api(&self) -> &dyn stats_api::StatsApi {
        &self.stats_api_mock
    }

    fn token_transfers_api(&self) -> &dyn token_transfers_api::TokenTransfersApi {
        &self.token_transfers_api_mock
    }

    fn tokens_api(&self) -> &dyn tokens_api::TokensApi {
        &self.tokens_api_mock
    }

    fn transactions_api(&self) -> &dyn transactions_api::TransactionsApi {
        &self.transactions_api_mock
    }

    fn withdrawals_api(&self) -> &dyn withdrawals_api::WithdrawalsApi {
        &self.withdrawals_api_mock
    }
}

pub mod feed;
pub mod pagination;
