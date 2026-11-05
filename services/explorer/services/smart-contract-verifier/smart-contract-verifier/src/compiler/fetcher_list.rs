use super::{
    fetcher::{FetchError, Fetcher, FileValidator, Version},
    fetcher_versions::{VersionsFetcher, VersionsRefresher},
};
use async_trait::async_trait;
use bytes::{Bytes, BytesMut};
use cron::Schedule;
use primitive_types::H256;
use sha2::{Digest, Sha256};
use std::{
    collections::HashMap,
    fmt::{Debug, Display},
    marker::PhantomData,
    path::{Path, PathBuf},
    str::FromStr,
    sync::Arc,
    time::Duration,
};
use thiserror::Error;
use tracing::{debug, instrument};
use url::Url;

type VersionsMap<Version> = HashMap<Version, FileInfo>;

#[derive(Clone, Debug, PartialEq)]
struct FileInfo {
    pub url: Url,
    pub sha256: H256,
}

#[derive(Error, Debug)]
enum ListError {
    #[error("fetching list json returned error: {0}")]
    ListJsonFetch(#[from] FetchError),
    #[error("cannot parse list json file: {0}")]
    ParseListJson(serde_json::Error),
    #[error("error parsing 'path' field: {0}")]
    Path(url::ParseError),
}

#[derive(Clone)]
struct DownloadLimits {
    connect_timeout: Duration,
    request_timeout: Duration,
    list_bytes: usize,
    binary_bytes: usize,
}

impl Default for DownloadLimits {
    fn default() -> Self {
        Self {
            connect_timeout: Duration::from_secs(10),
            request_timeout: Duration::from_secs(120),
            list_bytes: 8 * 1024 * 1024,
            binary_bytes: 128 * 1024 * 1024,
        }
    }
}

#[derive(Clone)]
struct Downloader {
    client: reqwest::Client,
    limits: DownloadLimits,
}

fn request_error(error: reqwest::Error) -> FetchError {
    if error.is_timeout() {
        FetchError::Timeout
    } else {
        FetchError::Fetch(error.into())
    }
}

impl Downloader {
    fn new(limits: DownloadLimits) -> Result<Self, FetchError> {
        if limits.connect_timeout.is_zero()
            || limits.request_timeout.is_zero()
            || limits.list_bytes == 0
            || limits.binary_bytes == 0
        {
            return Err(FetchError::Fetch(anyhow::anyhow!(
                "compiler download limits must be positive"
            )));
        }
        let client = reqwest::Client::builder()
            .connect_timeout(limits.connect_timeout)
            .timeout(limits.request_timeout)
            .build()
            .map_err(request_error)?;
        Ok(Self { client, limits })
    }

    async fn download(&self, url: Url, limit: usize) -> Result<(Bytes, H256), FetchError> {
        let mut response = self.client.get(url).send().await.map_err(request_error)?;
        if !response.status().is_success() {
            return Err(FetchError::HttpStatus {
                status: response.status().as_u16(),
            });
        }
        if response
            .content_length()
            .is_some_and(|length| length > limit as u64)
        {
            return Err(FetchError::BodyTooLarge { limit });
        }
        let mut data = BytesMut::new();
        let mut hasher = Sha256::new();
        while let Some(chunk) = response.chunk().await.map_err(request_error)? {
            if chunk.len() > limit.saturating_sub(data.len()) {
                return Err(FetchError::BodyTooLarge { limit });
            }
            hasher.update(&chunk);
            data.extend_from_slice(&chunk);
        }
        Ok((data.freeze(), H256::from_slice(&hasher.finalize())))
    }
}

/// Configures the shared discovery/download client. Defaults: 10s connection, 120s total,
/// 8 MiB catalogue, 128 MiB compiler. Cache directories must be owned by the service account.
pub struct ListFetcherBuilder<Ver> {
    list_url: Url,
    folder: PathBuf,
    refresh_schedule: Option<Schedule>,
    validator: Option<Arc<dyn FileValidator<Ver>>>,
    limits: DownloadLimits,
}

impl<Ver: Version> ListFetcherBuilder<Ver>
where
    <Ver as FromStr>::Err: Display,
{
    pub fn connect_timeout(mut self, timeout: Duration) -> Self {
        self.limits.connect_timeout = timeout;
        self
    }

    pub fn request_timeout(mut self, timeout: Duration) -> Self {
        self.limits.request_timeout = timeout;
        self
    }

    pub fn max_list_bytes(mut self, limit: usize) -> Self {
        self.limits.list_bytes = limit;
        self
    }

    pub fn max_binary_bytes(mut self, limit: usize) -> Self {
        self.limits.binary_bytes = limit;
        self
    }

    pub async fn build(self) -> anyhow::Result<ListFetcher<Ver>> {
        let downloader = Downloader::new(self.limits)?;
        let fetcher = Arc::new(ListVersionFetcher::new(self.list_url, downloader.clone()));
        let versions = VersionsRefresher::new(fetcher, self.refresh_schedule).await?;
        Ok(ListFetcher {
            versions,
            folder: self.folder,
            validator: self.validator,
            downloader,
        })
    }
}

struct ListVersionFetcher<Ver> {
    list_url: Url,
    downloader: Downloader,
    _phantom_data: PhantomData<Ver>,
}

impl<Ver> ListVersionFetcher<Ver> {
    fn new(list_url: Url, downloader: Downloader) -> Self {
        Self {
            list_url,
            downloader,
            _phantom_data: Default::default(),
        }
    }
}

impl<Ver: Version> ListVersionFetcher<Ver>
where
    <Ver as FromStr>::Err: Display,
{
    async fn fetch_json_versions(&self) -> Result<json::List<Ver>, ListError> {
        let (bytes, _) = self
            .downloader
            .download(self.list_url.clone(), self.downloader.limits.list_bytes)
            .await?;
        serde_json::from_slice(&bytes).map_err(ListError::ParseListJson)
    }

    fn parse_json_versions(
        &self,
        list_json: json::List<Ver>,
    ) -> Result<VersionsMap<Ver>, ListError> {
        let mut versions = HashMap::default();
        for json_compiler_info in list_json.builds {
            let version = json_compiler_info.long_version.clone();
            let file_info = json_compiler_info
                .into_compact(&self.list_url)
                .map_err(ListError::Path)?;
            versions.insert(version, file_info);
        }
        Ok(versions)
    }
}

#[async_trait]
impl<Ver: Version> VersionsFetcher for ListVersionFetcher<Ver>
where
    <Ver as FromStr>::Err: Display,
{
    type Versions = VersionsMap<Ver>;
    type Error = ListError;

    fn len(vers: &Self::Versions) -> usize {
        vers.len()
    }

    #[instrument(skip(self), level = "debug")]
    async fn fetch_versions(&self) -> Result<Self::Versions, Self::Error> {
        let list_json = self.fetch_json_versions().await?;
        debug!("found list json file of len = {}", list_json.builds.len());
        self.parse_json_versions(list_json)
    }
}

pub struct ListFetcher<Ver> {
    downloader: Downloader,
    versions: VersionsRefresher<VersionsMap<Ver>>,
    folder: PathBuf,
    validator: Option<Arc<dyn FileValidator<Ver>>>,
}

impl<Ver: Version> ListFetcher<Ver>
where
    <Ver as FromStr>::Err: Display,
{
    pub async fn new(
        list_url: Url,
        folder: PathBuf,
        refresh_schedule: Option<Schedule>,
        validator: Option<Arc<dyn FileValidator<Ver>>>,
    ) -> anyhow::Result<Self> {
        let mut builder = Self::builder(list_url, folder);
        builder.refresh_schedule = refresh_schedule;
        builder.validator = validator;
        builder.build().await
    }

    pub fn builder(list_url: Url, folder: PathBuf) -> ListFetcherBuilder<Ver> {
        ListFetcherBuilder {
            list_url,
            folder,
            refresh_schedule: None,
            validator: None,
            limits: DownloadLimits::default(),
        }
    }

    #[instrument(skip(self), level = "debug")]
    async fn fetch_file(&self, ver: &Ver) -> Result<(Bytes, H256), FetchError> {
        let file_info = {
            let versions = self.versions.read();
            versions
                .get(ver)
                .cloned()
                .ok_or_else(|| FetchError::NotFound(ver.clone().to_string()))?
        };

        let (data, hash) = self
            .downloader
            .download(file_info.url, self.downloader.limits.binary_bytes)
            .await?;
        if hash != file_info.sha256 {
            return Err(mismatch::Mismatch::new(file_info.sha256, hash).into());
        }
        Ok((data, file_info.sha256))
    }
}

#[async_trait]
impl<Ver: Version> Fetcher for ListFetcher<Ver>
where
    <Ver as FromStr>::Err: Display,
{
    type Version = Ver;

    async fn fetch(&self, ver: &Self::Version) -> Result<PathBuf, FetchError> {
        let (data, hash) = self.fetch_file(ver).await?;
        super::fetcher::write_executable(data, hash, &self.folder, ver, self.validator.as_deref())
            .await
    }

    async fn validate_cached(&self, ver: &Ver, path: &Path) -> Result<(), FetchError> {
        let expected = self
            .versions
            .read()
            .get(ver)
            .map(|info| info.sha256)
            .ok_or_else(|| FetchError::NotFound(ver.to_string()))?;
        super::fetcher::validate_cached_file(
            ver,
            path,
            &self.folder,
            expected,
            self.downloader.limits.binary_bytes,
            self.validator.as_deref(),
        )
        .await
    }

    fn all_versions(&self) -> Vec<Self::Version> {
        let versions = self.versions.read();
        versions.keys().map(Clone::clone).collect()
    }
}

mod json {
    use super::Version;
    use primitive_types::H256;
    use serde::{Deserialize, Serialize};
    use serde_with::{serde_as, DisplayFromStr};
    use std::{fmt::Display, str::FromStr};
    use url::Url;

    #[derive(Debug, Deserialize, Serialize, PartialEq, Eq)]
    pub struct List<Ver: Version>
    where
        <Ver as FromStr>::Err: Display,
    {
        #[serde(bound(deserialize = "Ver: FromStr, Ver::Err: Display"))]
        #[serde(bound(serialize = "Ver: Display"))]
        pub builds: Vec<FileInfo<Ver>>,
    }

    #[serde_as]
    #[derive(Debug, Deserialize, Serialize, PartialEq, Eq)]
    #[serde(rename_all = "camelCase")]
    pub struct FileInfo<Ver: Version>
    where
        <Ver as FromStr>::Err: Display,
    {
        pub path: DownloadPath,
        #[serde_as(as = "DisplayFromStr")]
        #[serde(bound(deserialize = "Ver: FromStr, Ver::Err: Display"))]
        #[serde(bound(serialize = "Ver: Display"))]
        pub long_version: Ver,
        pub sha256: H256,
    }

    impl<Ver: Version> FileInfo<Ver>
    where
        <Ver as FromStr>::Err: Display,
    {
        pub fn into_compact(self, download_url: &Url) -> Result<super::FileInfo, url::ParseError> {
            let url = match self.path {
                DownloadPath::Url(url) => url,
                // download_url ends with `.../list.json` but join() will replace this with `filename`
                DownloadPath::Filename(filename) => download_url.join(&filename)?,
            };
            Ok(super::FileInfo {
                url,
                sha256: self.sha256,
            })
        }
    }

    #[derive(Debug, Deserialize, Serialize, PartialEq, Eq)]
    #[serde(untagged)]
    pub enum DownloadPath {
        Url(Url),
        Filename(String),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        consts::DEFAULT_SOLIDITY_COMPILER_LIST, tests::parse::test_deserialize_ok, CompactVersion,
        DetailedVersion,
    };
    use foundry_compilers::solc::Solc;
    use pretty_assertions::assert_eq;
    use std::{env::temp_dir, str::FromStr};
    use wiremock::{
        matchers::{method, path},
        Mock, MockServer, ResponseTemplate,
    };

    const DEFAULT_LIST_JSON: &str = r#"{
        "builds": [
            {
                "path": "https://github.com/blockscout/solc-bin/releases/download/solc-v0.8.15-nightly.2022.5.27%2Bcommit.095cc647/solc",
                "longVersion": "0.8.15-nightly.2022.5.27+commit.095cc647",
                "sha256": "35708c1593f3daddae734065e361a839ee39d400825972fb3f50718495be82b1"
            },
            {
                "path": "https://binaries.soliditylang.org/linux-amd64/solc-linux-amd64-v0.4.13+commit.0fb4cb1a",
                "longVersion": "0.4.13+commit.0fb4cb1a",
                "sha256": "0x791ee3a20adf6c5ab76cc889f13cca102f76eb0b7cf0da4a0b5b11dc46edf349"
            },
            {
                "path": "https://binaries.soliditylang.org/linux-amd64/solc-linux-amd64-v0.4.14+commit.c2215d46",
                "longVersion": "0.4.14+commit.c2215d46",
                "sha256": "0x28ce35a0941d9ecd59a2b1a377c019110e79a6b38bdbf5a3bffea811f9c2a13b"
            },
            {
                "path": "solc-linux-amd64-v0.4.15+commit.8b45bddb",
                "longVersion": "0.4.15+commit.8b45bddb",
                "sha256": "0xc71ac6c28bf3b1a425e77e97f5df67a80da3e4c047261875206561c0a110c0cb"
            },
            {
                "path": "download/files/solc-linux-amd64-v0.4.16+commit.d7661dd9",
                "longVersion": "0.4.16+commit.d7661dd9",
                "sha256": "0x78e0da6cad24ab145a8d17420c4f094c8314418ca23cff4b050bb2bfd36f3af2"
            },
            {
                "path": "solc-linux-amd64-v10.8.9-nightly.2021.9.11+commit.e5eed63a",
                "longVersion": "10.8.9-nightly.2021.9.11+commit.e5eed63a",
                "sha256": "0x791ee3a20adf6c5ab76cc889f13cca102f76eb0b7cf0da4a0b5b11dc46edf349"
            }
        ]
    }"#;
    const DEFAULT_DOWNLOAD_PREFIX: &str = "https://binaries.soliditylang.org/linux-amd64/";

    #[test]
    fn parse_list_json() {
        let ver = |s| DetailedVersion::from_str(s).unwrap();
        test_deserialize_ok(vec![
            (DEFAULT_LIST_JSON,
             json::List {
                 builds: vec![
                     json::FileInfo {
                         path: json::DownloadPath::Url(Url::from_str("https://github.com/blockscout/solc-bin/releases/download/solc-v0.8.15-nightly.2022.5.27%2Bcommit.095cc647/solc").unwrap()),
                         long_version: ver("0.8.15-nightly.2022.5.27+commit.095cc647"),
                         sha256: H256::from_str("35708c1593f3daddae734065e361a839ee39d400825972fb3f50718495be82b1").unwrap(),
                     },
                     json::FileInfo {
                         path: json::DownloadPath::Url(Url::from_str("https://binaries.soliditylang.org/linux-amd64/solc-linux-amd64-v0.4.13+commit.0fb4cb1a").unwrap()),
                         long_version: ver("0.4.13+commit.0fb4cb1a"),
                         sha256: H256::from_str("0x791ee3a20adf6c5ab76cc889f13cca102f76eb0b7cf0da4a0b5b11dc46edf349").unwrap(),
                     },
                     json::FileInfo {
                         path: json::DownloadPath::Url(Url::from_str("https://binaries.soliditylang.org/linux-amd64/solc-linux-amd64-v0.4.14+commit.c2215d46").unwrap()),
                         long_version: ver("0.4.14+commit.c2215d46"),
                         sha256: H256::from_str("0x28ce35a0941d9ecd59a2b1a377c019110e79a6b38bdbf5a3bffea811f9c2a13b").unwrap(),
                     },
                     json::FileInfo {
                         path: json::DownloadPath::Filename("solc-linux-amd64-v0.4.15+commit.8b45bddb".to_string()),
                         long_version: ver("0.4.15+commit.8b45bddb"),
                         sha256: H256::from_str("0xc71ac6c28bf3b1a425e77e97f5df67a80da3e4c047261875206561c0a110c0cb").unwrap(),
                     },
                     json::FileInfo {
                         path: json::DownloadPath::Filename("download/files/solc-linux-amd64-v0.4.16+commit.d7661dd9".to_string()),
                         long_version: ver("0.4.16+commit.d7661dd9"),
                         sha256: H256::from_str("0x78e0da6cad24ab145a8d17420c4f094c8314418ca23cff4b050bb2bfd36f3af2").unwrap(),
                     },
                     json::FileInfo {
                         path: json::DownloadPath::Filename("solc-linux-amd64-v10.8.9-nightly.2021.9.11+commit.e5eed63a".to_string()),
                         long_version: ver("10.8.9-nightly.2021.9.11+commit.e5eed63a"),
                         sha256: H256::from_str("0x791ee3a20adf6c5ab76cc889f13cca102f76eb0b7cf0da4a0b5b11dc46edf349").unwrap(),
                     },
                 ]
             })
        ]);
    }

    fn assert_has_version(versions: &VersionsMap<DetailedVersion>, ver: &str, expect: &str) {
        let ver = DetailedVersion::from_str(ver).unwrap();
        let info = versions.get(&ver).unwrap();
        let url = info.url.to_string();
        assert_eq!(url, expect, "urls don't match");
    }

    #[test]
    fn parse_versions() {
        let list_json_file: json::List<DetailedVersion> =
            serde_json::from_str(DEFAULT_LIST_JSON).unwrap();
        let download_url = Url::from_str(DEFAULT_DOWNLOAD_PREFIX).expect("valid url");
        let fetcher = ListVersionFetcher::new(
            download_url,
            Downloader::new(DownloadLimits::default()).unwrap(),
        );
        let verions = fetcher.parse_json_versions(list_json_file).unwrap();
        assert_has_version(
            &verions,
            "0.8.15-nightly.2022.5.27+commit.095cc647",
            "https://github.com/blockscout/solc-bin/releases/download/solc-v0.8.15-nightly.2022.5.27%2Bcommit.095cc647/solc",
        );
        assert_has_version(
            &verions,
            "0.4.13+commit.0fb4cb1a",
            "https://binaries.soliditylang.org/linux-amd64/solc-linux-amd64-v0.4.13+commit.0fb4cb1a",
        );
        assert_has_version(&verions,
                           "10.8.9-nightly.2021.9.11+commit.e5eed63a",
                           "https://binaries.soliditylang.org/linux-amd64/solc-linux-amd64-v10.8.9-nightly.2021.9.11+commit.e5eed63a"
        );
        assert_has_version(&verions,
                           "0.4.16+commit.d7661dd9",
                           "https://binaries.soliditylang.org/linux-amd64/download/files/solc-linux-amd64-v0.4.16+commit.d7661dd9"
        );
    }

    #[tokio::test]
    async fn list_download_versions() {
        let list_url = Url::try_from(DEFAULT_SOLIDITY_COMPILER_LIST).expect("valid url");
        let fetcher: ListFetcher<DetailedVersion> = ListFetcher::new(
            list_url,
            temp_dir().join("blockscout/smart_contract_verifier/compiler_fetcher/test/"),
            None,
            None,
        )
        .await
        .expect("list.json file should be valid");

        for compiler_version in [
            DetailedVersion::from_str("0.7.0+commit.9e61f92b").unwrap(),
            DetailedVersion::from_str("0.8.9+commit.e5eed63a").unwrap(),
        ] {
            let file = fetcher.fetch(&compiler_version).await.unwrap();
            let solc = Solc::new(file).unwrap();
            let ver = solc.version;
            assert_eq!(
                (ver.major, ver.minor, ver.patch),
                (
                    compiler_version.version().major,
                    compiler_version.version().minor,
                    compiler_version.version().patch,
                )
            );
        }
    }

    #[tokio::test]
    async fn check_refresh_versions() {
        let mock_server = MockServer::start().await;

        // mock list.json server response with empty list
        Mock::given(method("GET"))
            .and(path("/"))
            .respond_with(ResponseTemplate::new(200).set_body_bytes("{\"builds\": []}"))
            .mount(&mock_server)
            .await;
        let fetcher: ListFetcher<DetailedVersion> = ListFetcher::new(
            Url::parse(&mock_server.uri()).unwrap(),
            temp_dir(),
            Some(Schedule::from_str("* * * * * * *").unwrap()),
            None,
        )
        .await
        .expect("cannot initialize fetcher");
        assert!(fetcher.all_versions().is_empty());

        // mock list.json server response with `DEFAULT_LIST_JSON`
        mock_server.reset().await;
        Mock::given(method("GET"))
            .and(path("/"))
            .respond_with(ResponseTemplate::new(200).set_body_bytes(DEFAULT_LIST_JSON))
            .mount(&mock_server)
            .await;
        // wait for refresher to do its job
        tokio::time::sleep(tokio::time::Duration::from_millis(1500)).await;
        let versions = fetcher.all_versions();
        assert!(
            versions.contains(&DetailedVersion::from_str("0.4.13+commit.0fb4cb1a").unwrap()),
            "versions list doesn't have 0.4.13: {versions:?}",
        );
    }

    const VYPER_LIST_JSON: &str = r#"{
        "builds": [
            {
                "path": "https://github.com/vyperlang/vyper/releases/download/v0.3.2/vyper.0.3.2%2Bcommit.3b6a4117.linux",
                "longVersion": "0.3.2+commit.3b6a4117",
                "sha256": "7101527cc0976468a07087e98438e88e372c02002a5b8c8c6c411517176c2592"
            }
        ]
    }"#;

    /// That's will try to download the Vyper compiler from the list.json file.
    /// It check's:
    /// 1) an access to a download link
    /// 2) Hash (mis)matching
    #[tokio::test]
    async fn download_vyper_versions() {
        let mock_server = MockServer::start().await;

        // mock list.json server response with empty list
        Mock::given(method("GET"))
            .and(path("/"))
            .respond_with(ResponseTemplate::new(200).set_body_bytes(VYPER_LIST_JSON))
            .mount(&mock_server)
            .await;
        let fetcher: ListFetcher<DetailedVersion> = ListFetcher::new(
            Url::parse(&mock_server.uri()).unwrap(),
            temp_dir(),
            None,
            None,
        )
        .await
        .expect("cannot initialize fetcher");

        let versions = fetcher.all_versions();
        assert!(
            versions.contains(&DetailedVersion::from_str("0.3.2+commit.3b6a4117").unwrap()),
            "versions list doesn't have 0.3.2: {versions:?}",
        );

        for compiler_version in versions {
            fetcher.fetch(&compiler_version).await.unwrap_or_else(|_| {
                panic!("fetcher: can't download vyper compiler {compiler_version}")
            });
        }
    }

    const ZKSOLC_LIST_JSON: &str = r#"{
        "builds": [
            {
              "path": "https://github.com/matter-labs/zksolc-bin/releases/download/v1.4.1/zksolc-macosx-arm64-v1.4.1",
              "version": "1.4.1",
              "longVersion": "1.4.1",
              "md5": "8736f04aaaca31dd78bbbe0bdd5c2443",
              "sha256": "f9991aa8c227d2ce35672cd33cf93370febce736b26beb047ede1919de12a3b8"
            }
        ]
    }"#;

    /// That's will try to download the ZkSolc compiler from the list.json file.
    /// It check's:
    /// 1) an access to a download link
    /// 2) Hash (mis)matching
    #[tokio::test]
    async fn download_zksolc_versions() {
        let mock_server = MockServer::start().await;

        // mock list.json server response with empty list
        Mock::given(method("GET"))
            .and(path("/"))
            .respond_with(ResponseTemplate::new(200).set_body_bytes(ZKSOLC_LIST_JSON))
            .mount(&mock_server)
            .await;
        let fetcher: ListFetcher<CompactVersion> = ListFetcher::new(
            Url::parse(&mock_server.uri()).unwrap(),
            temp_dir(),
            None,
            None,
        )
        .await
        .expect("cannot initialize fetcher");

        let versions = fetcher.all_versions();
        assert!(
            versions.contains(&CompactVersion::from_str("v1.4.1").unwrap()),
            "versions list doesn't have v1.4.1: {versions:?}",
        );

        for compiler_version in versions {
            fetcher.fetch(&compiler_version).await.unwrap_or_else(|_| {
                panic!("fetcher: can't download zksolc compiler {compiler_version}")
            });
        }
    }
}
