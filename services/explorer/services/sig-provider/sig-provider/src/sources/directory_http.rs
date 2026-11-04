use anyhow::Context;
use reqwest::{Client, Response};
use std::time::Duration;

pub(super) const TOTAL_TIMEOUT: Duration = Duration::from_secs(10);
const MAX_RETRIES: usize = 3;

pub(super) fn client() -> Client {
    Client::builder()
        .timeout(TOTAL_TIMEOUT)
        .redirect(reqwest::redirect::Policy::none())
        .build()
        .expect("signature directory HTTP client configuration must be valid")
}

pub(super) async fn send(
    request: reqwest::RequestBuilder,
    max_retries: usize,
) -> anyhow::Result<Response> {
    for attempt in 0..=max_retries.min(MAX_RETRIES) {
        let response = request
            .try_clone()
            .context("signature directory request cannot be replayed")?
            .send()
            .await;
        let delay = match response {
            Ok(response) if response.status().is_success() => return Ok(response),
            Ok(response) => {
                let status = response.status();
                let retryable = matches!(status.as_u16(), 408 | 429 | 500 | 502 | 503 | 504);
                if !retryable || attempt == max_retries.min(MAX_RETRIES) {
                    anyhow::bail!(
                        "signature directory returned HTTP {status}; {}",
                        if retryable {
                            "retry later with backoff"
                        } else {
                            "check the upstream request or configuration before retrying"
                        }
                    );
                }
                match response.headers().get(reqwest::header::RETRY_AFTER) {
                    Some(value) => Duration::from_secs(
                        value
                            .to_str()
                            .context("upstream Retry-After is not text; retry later")?
                            .parse()
                            .context("upstream Retry-After must be seconds; retry later")?,
                    ),
                    None => Duration::from_millis(50 * (1 << attempt)),
                }
            }
            Err(error) => {
                if !(error.is_timeout() || error.is_connect())
                    || attempt == max_retries.min(MAX_RETRIES)
                {
                    return Err(error)
                        .context("signature directory transport failed; retry later with backoff");
                }
                Duration::from_millis(50 * (1 << attempt))
            }
        };
        tokio::time::sleep(delay).await;
    }
    unreachable!("bounded request loop always returns on its last attempt")
}

pub(super) async fn import(request: reqwest::RequestBuilder) -> anyhow::Result<Response> {
    send(request, MAX_RETRIES).await
}

pub(super) fn validate_abi(abi: &str) -> anyhow::Result<()> {
    ethabi::Contract::load(abi.as_bytes())
        .context("malformed contract ABI; correct the ABI before retrying")?;
    Ok(())
}
