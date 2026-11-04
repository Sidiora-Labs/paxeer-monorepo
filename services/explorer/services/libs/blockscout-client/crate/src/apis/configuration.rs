use bon::Builder;
use std::{fmt, time::Duration};

pub const DEFAULT_CONNECT_TIMEOUT: Duration = Duration::from_secs(5);
pub const DEFAULT_REQUEST_TIMEOUT: Duration = Duration::from_secs(30);
pub const DEFAULT_MAX_RESPONSE_BYTES: usize = 8 * 1024 * 1024;

/// Configure the explorer REST origin explicitly. Before using deployment data,
/// validate the separately configured root node's eth_chainId (Paxeer: 0x7d)
/// against deployment identity; a REST URL alone does not establish chain identity.
#[derive(Clone, Builder)]
pub struct Configuration {
    #[builder(into)]
    pub base_path: String,
    /// Root JSON-RPC origin supplied by the deployment, never inferred from REST.
    #[builder(into)]
    pub node_rpc_origin: Option<String>,
    #[builder(into)]
    pub user_agent: Option<String>,
    #[builder(default = default_client())]
    pub client: reqwest_middleware::ClientWithMiddleware,
    #[builder(default = DEFAULT_REQUEST_TIMEOUT)]
    pub request_timeout: Duration,
    #[builder(default = DEFAULT_MAX_RESPONSE_BYTES)]
    pub max_response_bytes: usize,
    pub basic_auth: Option<BasicAuth>,
    #[builder(into)]
    pub oauth_access_token: Option<String>,
    #[builder(into)]
    pub bearer_access_token: Option<String>,
    pub api_key: Option<ApiKey>,
}

pub type BasicAuth = (String, Option<String>);

#[derive(Clone)]
pub struct ApiKey {
    pub prefix: Option<String>,
    pub key: String,
}

impl fmt::Debug for ApiKey {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.debug_struct("ApiKey")
            .field("credentials", &"[REDACTED]")
            .finish()
    }
}

impl fmt::Debug for Configuration {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // Even URLs, user-agent and middleware may contain user-supplied secrets.
        let origin = reqwest::Url::parse(&self.base_path)
            .ok()
            .map(|url| url.origin().ascii_serialization());
        f.debug_struct("Configuration")
            .field("rest_origin", &origin)
            .field("request_timeout", &self.request_timeout)
            .field("max_response_bytes", &self.max_response_bytes)
            .field("credentials", &"[REDACTED]")
            .finish()
    }
}

fn default_client() -> reqwest_middleware::ClientWithMiddleware {
    reqwest_middleware::ClientBuilder::new(
        reqwest::Client::builder()
            .connect_timeout(DEFAULT_CONNECT_TIMEOUT)
            .timeout(DEFAULT_REQUEST_TIMEOUT)
            .redirect(reqwest::redirect::Policy::none())
            .build()
            .expect("valid bounded HTTP client configuration"),
    )
    .build()
}

impl Configuration {
    pub fn new() -> Configuration {
        Configuration::default()
    }

    pub(crate) fn rest_origin<T>(&self) -> Result<&str, super::Error<T>> {
        validate_origin(&self.base_path).map_err(super::Error::Configuration)?;
        if self.request_timeout.is_zero() || self.max_response_bytes == 0 {
            return Err(super::Error::Configuration(
                "HTTP deadline and response limit must be positive",
            ));
        }
        Ok(self.base_path.trim_end_matches('/'))
    }

    pub fn node_origin(&self) -> Result<&str, super::Error<()>> {
        let origin = self
            .node_rpc_origin
            .as_deref()
            .ok_or(super::Error::Configuration(
                "explicit root node JSON-RPC origin required",
            ))?;
        validate_origin(origin).map_err(super::Error::Configuration)?;
        Ok(origin.trim_end_matches('/'))
    }
}

fn validate_origin(origin: &str) -> Result<(), &'static str> {
    let url = reqwest::Url::parse(origin).map_err(|_| "explicit HTTP(S) origin required")?;
    if !matches!(url.scheme(), "http" | "https")
        || url.host_str().is_none()
        || !url.username().is_empty()
        || url.password().is_some()
        || url.path() != "/"
        || url.query().is_some()
        || url.fragment().is_some()
    {
        return Err("expected an HTTP(S) origin without credentials, path, query or fragment");
    }
    Ok(())
}

impl Default for Configuration {
    fn default() -> Self {
        Self {
            base_path: String::new(),
            node_rpc_origin: None,
            user_agent: Some("OpenAPI-Generator/1.0.0/rust".to_owned()),
            client: default_client(),
            request_timeout: DEFAULT_REQUEST_TIMEOUT,
            max_response_bytes: DEFAULT_MAX_RESPONSE_BYTES,
            basic_auth: None,
            oauth_access_token: None,
            bearer_access_token: None,
            api_key: None,
        }
    }
}
