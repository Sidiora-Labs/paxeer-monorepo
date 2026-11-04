use blockscout_client::{Api, ApiClient, Configuration};
use std::{error::Error, io, ops::Deref, path::Path, time::Duration};
use stubr::Stubr;

pub type HarnessResult<T> = Result<T, Box<dyn Error + Send + Sync>>;
pub const DEADLINE: Duration = Duration::from_secs(5);

pub struct OwnedApiClient {
    client: ApiClient,
    // The server lives on this test's runtime and is dropped with its client.
    _server: Stubr,
}

impl OwnedApiClient {
    pub async fn start() -> HarnessResult<Self> {
        Self::start_from(
            Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/recorded/eth_blockscout_com"),
        )
        .await
    }

    pub async fn start_from(path: impl AsRef<Path>) -> HarnessResult<Self> {
        let path = path.as_ref();
        std::fs::read_dir(path).map_err(|error| {
            io::Error::new(
                error.kind(),
                format!("recording directory {}: {error}", path.display()),
            )
        })?;
        let server = tokio::time::timeout(DEADLINE, Stubr::try_start(path.to_path_buf()))
            .await
            .map_err(|error| {
                io::Error::new(
                    io::ErrorKind::TimedOut,
                    format!("recording server startup: {error}"),
                )
            })??;
        let http = reqwest::Client::builder()
            .no_proxy()
            .connect_timeout(DEADLINE)
            .timeout(DEADLINE)
            .build()?;
        let client = ApiClient::new(
            Configuration::builder()
                .base_path(server.uri())
                .request_timeout(DEADLINE)
                .client(reqwest_middleware::ClientBuilder::new(http).build())
                .build(),
        );
        let ready =
            client.health_api().health().await.map_err(|error| {
                io::Error::other(format!("recording server readiness: {error}"))
            })?;
        let health = ready.try_parse_as_success_model()?;
        if ready.status != reqwest::StatusCode::OK || !health.healthy {
            return Err(io::Error::other(format!("recording server readiness: {ready:?}")).into());
        }
        Ok(Self {
            client,
            _server: server,
        })
    }
}

impl Deref for OwnedApiClient {
    type Target = ApiClient;

    fn deref(&self) -> &Self::Target {
        &self.client
    }
}
