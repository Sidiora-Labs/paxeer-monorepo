use super::directory_http;
use crate::SignatureSource;
use anyhow::Context;
use std::collections::HashSet;

const MAX_PAGES: usize = 64;

pub struct Source {
    host: url::Url,
    client: reqwest::Client,
    n_retries: usize,
}

impl Source {
    pub fn new(host: url::Url) -> Source {
        Source {
            host,
            client: directory_http::client(),
            n_retries: 3,
        }
    }

    #[cfg(test)]
    pub fn n_retries(mut self, n_retries: usize) -> Source {
        self.n_retries = n_retries;
        self
    }

    async fn fetch(&self, path: String) -> anyhow::Result<Vec<String>> {
        tokio::time::timeout(directory_http::TOTAL_TIMEOUT, self.fetch_pages(&path))
            .await
            .context("fourbyte pagination exceeded total deadline; retry later with backoff")?
    }

    async fn fetch_pages(&self, path: &str) -> anyhow::Result<Vec<String>> {
        let mut page = self
            .host
            .join(path)
            .context("invalid fourbyte lookup URL")?;
        let mut visited = HashSet::new();
        let mut signatures = Vec::new();
        for _ in 0..MAX_PAGES {
            page.set_fragment(None);
            anyhow::ensure!(
                visited.insert(page.clone()),
                "fourbyte pagination cycle; retry later after upstream repair"
            );
            let response =
                directory_http::send(self.client.get(page.clone()), self.n_retries).await?;
            let response: json::GetResponse = response
                .json()
                .await
                .context("malformed fourbyte page; retry later after upstream repair")?;
            signatures.extend(response.results.into_iter().map(|sig| sig.text_signature));
            let Some(next) = response.next else {
                return Ok(signatures);
            };
            let next = page
                .join(&next)
                .context("malformed fourbyte next link; retry later after upstream repair")?;
            anyhow::ensure!(
                next.origin() == self.host.origin()
                    && matches!(next.scheme(), "http" | "https")
                    && next.username().is_empty()
                    && next.password().is_none(),
                "unexpected-origin fourbyte next link; check upstream pagination before retrying"
            );
            page = next;
        }
        anyhow::bail!("fourbyte pagination exceeded {MAX_PAGES} pages; narrow the lookup or retry after upstream repair")
    }
}

#[async_trait::async_trait]
impl SignatureSource for Source {
    async fn create_signatures(&self, abi: &str) -> Result<(), anyhow::Error> {
        directory_http::validate_abi(abi)?;
        let request = self
            .client
            .post(
                self.host
                    .join("/api/v1/import-solidity/")
                    .context("invalid fourbyte import URL")?,
            )
            .json(&json::CreateRequest { contract_abi: abi });
        tokio::time::timeout(directory_http::TOTAL_TIMEOUT, async {
            let response = directory_http::import(request).await?;
            let result: json::CreateResponse = response
                .json()
                .await
                .context("malformed fourbyte import result; check upstream before retrying")?;
            let accounted = result
                .num_imported
                .checked_add(result.num_duplicates)
                .and_then(|count| count.checked_add(result.num_ignored));
            anyhow::ensure!(
                accounted == Some(result.num_processed),
                "fourbyte import rejected: inconsistent import counts; check ABI before retrying"
            );
            Ok(())
        })
        .await
        .context("fourbyte import exceeded total deadline; retry later with backoff")?
    }

    async fn get_function_signatures(&self, hex: &str) -> Result<Vec<String>, anyhow::Error> {
        self.fetch(format!("/api/v1/signatures/?hex_signature={hex}"))
            .await
    }

    async fn get_event_signatures(&self, hex: &str) -> Result<Vec<String>, anyhow::Error> {
        self.fetch(format!("/api/v1/event-signatures/?hex_signature={hex}"))
            .await
    }

    fn source(&self) -> String {
        self.host.to_string()
    }
}

mod json {
    use serde::{Deserialize, Serialize};

    #[derive(Debug, Serialize)]
    pub struct CreateRequest<'a> {
        pub contract_abi: &'a str,
    }

    #[derive(Debug, Deserialize)]
    pub struct CreateResponse {
        pub num_processed: u64,
        pub num_imported: u64,
        pub num_duplicates: u64,
        pub num_ignored: u64,
    }

    #[derive(Debug, Deserialize)]
    pub struct Signature {
        pub text_signature: String,
    }

    #[derive(Debug, Deserialize)]
    pub struct GetResponse {
        pub next: Option<String>,
        pub results: Vec<Signature>,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::str::FromStr;

    const DEFAULT_HOST: &str = "https://www.4byte.directory/";

    #[rstest::fixture]
    fn source() -> Source {
        let host = url::Url::from_str(DEFAULT_HOST).expect("default host is not an url");
        Source::new(host).n_retries(6) // We increase n_retries to avoid most blinking tests
    }

    #[rstest::rstest]
    #[tokio::test]
    async fn create(source: Source) {
        let abi = r#"[{"constant":false,"inputs":[],"name":"f","outputs":[],"type":"function"},{"inputs":[],"type":"constructor"},{"anonymous":false,"inputs":[{"name":"","type":"string","indexed":true}],"name":"E","type":"event"}]"#;
        source
            .create_signatures(abi)
            .await
            .expect("error while submitting a new signature");
    }

    #[rstest::rstest]
    #[tokio::test]
    async fn get_function_signatures(source: Source) {
        let (signature, hex) = ("f()", "0x26121ff0");
        let result = source
            .get_function_signatures(hex)
            .await
            .expect("error while getting function signature");
        assert!(result.contains(&signature.into()))
    }

    #[rstest::rstest]
    #[tokio::test]
    async fn get_event_signatures(source: Source) {
        let (signature, hex) = (
            "E(string)",
            "0x3e9992c940c54ea252d3a34557cc3d3014281525c43d694f89d5f3dfd820b07d",
        );
        let result = source
            .get_event_signatures(hex)
            .await
            .expect("error while getting event signature");
        assert!(result.contains(&signature.into()))
    }
}
