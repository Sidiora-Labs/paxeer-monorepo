use super::directory_http;
use crate::SignatureSource;
use anyhow::Context;
use reqwest_middleware::ClientWithMiddleware;

pub struct Source {
    host: url::Url,
    client: ClientWithMiddleware,
    import_client: reqwest::Client,
}

impl Source {
    pub fn new(host: url::Url) -> Source {
        Source {
            host,
            client: super::new_client(),
            import_client: directory_http::client(),
        }
    }

    async fn fetch(&self, path: &str) -> Result<json::GetResponse, anyhow::Error> {
        let response = self
            .client
            .get(self.host.join(path).context("invalid sigeth lookup URL")?)
            .send()
            .await
            .map_err(anyhow::Error::msg)?;
        match response.status() {
            reqwest::StatusCode::OK => Ok(response.json().await?),
            status => Err(anyhow::anyhow!(
                "invalid status code got as a result: {}",
                status
            )),
        }
    }

    fn convert(sigs: Option<json::SigMap>, hash: &str) -> Vec<String> {
        // TODO: sort using "filtered" field
        sigs.and_then(|mut sigs| {
            sigs.remove(hash)
                .map(|sigs| sigs.into_iter().map(|sig| sig.name).collect())
        })
        .unwrap_or_default()
    }
}

#[async_trait::async_trait]
impl SignatureSource for Source {
    async fn create_signatures(&self, abi: &str) -> Result<(), anyhow::Error> {
        directory_http::validate_abi(abi)?;
        let abi = serde_json::from_str(abi).context("malformed sigeth ABI")?;
        let request = self
            .import_client
            .post(
                self.host
                    .join("/api/v1/import")
                    .context("invalid sigeth import URL")?,
            )
            .json(&json::CreateRequest {
                kind: "abi",
                data: vec![abi],
            });
        tokio::time::timeout(directory_http::TOTAL_TIMEOUT, async {
            let response = directory_http::import(request).await?;
            let response: json::CreateResponse = response
                .json()
                .await
                .context("malformed sigeth import result; check upstream before retrying")?;
            anyhow::ensure!(
                response.ok,
                "sigeth import rejected by upstream; check ABI before retrying"
            );
            let result = response.result.context(
                "sigeth import acceptance result missing; check upstream before retrying",
            )?;
            for result in [result.function, result.event] {
                anyhow::ensure!(
                    result.invalid.is_null()
                        || result.invalid.as_array().is_some_and(Vec::is_empty)
                        || result
                            .invalid
                            .as_object()
                            .is_some_and(serde_json::Map::is_empty),
                    "sigeth import rejected invalid signatures; correct ABI before retrying"
                );
            }
            Ok(())
        })
        .await
        .context("sigeth import exceeded total deadline; retry later with backoff")?
    }

    async fn get_function_signatures(&self, hex: &str) -> Result<Vec<String>, anyhow::Error> {
        let hash = super::hash(hex);
        let resp = self
            .fetch(&format!("/api/v1/signatures?function={hash}&all"))
            .await?;
        let signatures = Self::convert(resp.result.function, &hash);
        Ok(signatures)
    }

    async fn get_event_signatures(&self, hex: &str) -> Result<Vec<String>, anyhow::Error> {
        let hash = super::hash(hex);
        let resp = self
            .fetch(&format!("/api/v1/signatures?event={hash}&all"))
            .await?;
        let signatures = Self::convert(resp.result.event, &hash);
        Ok(signatures)
    }

    fn source(&self) -> String {
        self.host.to_string()
    }
}

mod json {
    use std::collections::HashMap;

    use serde::{Deserialize, Serialize};

    #[derive(Debug, Serialize)]
    pub struct CreateRequest {
        #[serde(rename = "type")]
        pub kind: &'static str,
        pub data: Vec<serde_json::Value>,
    }

    #[derive(Debug, Deserialize)]
    pub struct CreateResponse {
        pub ok: bool,
        pub result: Option<ImportResult>,
    }

    #[derive(Debug, Deserialize)]
    pub struct ImportResult {
        pub function: ImportSignatures,
        pub event: ImportSignatures,
    }

    #[derive(Debug, Deserialize)]
    pub struct ImportSignatures {
        #[serde(rename = "imported")]
        pub _imported: HashMap<String, String>,
        #[serde(rename = "duplicated")]
        pub _duplicated: HashMap<String, String>,
        pub invalid: serde_json::Value,
    }

    #[derive(Debug, Deserialize)]
    pub struct Signature {
        pub name: String,
    }

    pub type SigMap = HashMap<String, Vec<Signature>>;

    #[derive(Debug, Deserialize)]
    pub struct SigTypes {
        pub function: Option<SigMap>,
        pub event: Option<SigMap>,
        pub _error: Option<SigMap>,
    }

    #[derive(Debug, Deserialize)]
    pub struct GetResponse {
        pub result: SigTypes,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::str::FromStr;

    const DEFAULT_HOST: &str = "https://sig.eth.samczsun.com/";

    #[rstest::fixture]
    fn source() -> Source {
        let host = url::Url::from_str(DEFAULT_HOST).expect("default host is not an url");
        Source::new(host)
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
