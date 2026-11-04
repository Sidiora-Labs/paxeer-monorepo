use crate::Error;
use reqwest::Client;
use std::{sync::OnceLock, time::Duration};

/// Reusable account transport. Clones share the connection pool and deadlines.
#[derive(Clone, Debug)]
pub struct AccountClient {
    pub(crate) http: Client,
}

impl AccountClient {
    /// The request deadline covers connection, response headers and the entire body.
    pub fn new(connect_timeout: Duration, request_timeout: Duration) -> Result<Self, Error> {
        if connect_timeout.is_zero() || request_timeout.is_zero() {
            return Err(Error::BlockscoutApi(
                "account deadlines must be nonzero".into(),
            ));
        }
        let http = Client::builder()
            .connect_timeout(connect_timeout)
            .timeout(request_timeout)
            .build()
            .map_err(upstream_error)?;
        Ok(Self { http })
    }
}

pub(crate) fn default_client() -> Result<&'static AccountClient, Error> {
    static CLIENT: OnceLock<Result<AccountClient, Error>> = OnceLock::new();
    CLIENT
        .get_or_init(|| AccountClient::new(Duration::from_secs(3), Duration::from_secs(10)))
        .as_ref()
        .map_err(|error| Error::BlockscoutApi(error.to_string()))
}

pub(crate) fn upstream_error(error: reqwest::Error) -> Error {
    if error.is_timeout() {
        Error::BlockscoutApi(format!("request timed out: {error}"))
    } else {
        Error::BlockscoutApi(error.to_string())
    }
}
