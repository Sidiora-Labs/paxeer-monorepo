use super::types::{Files, Success};
use crate::MatchType;
use bytes::Bytes;
use serde::Deserialize;
use std::collections::BTreeMap;

const METADATA_FILE_NAME: &str = "metadata.json";
const SOURCES_PREFIX: &str = "sources/";

// There is struct for metadata in foundry_compilers::artifacts::Metadata
// however it is for standard json input of compiler and
// has different `libraries` field structure
#[derive(Debug, PartialEq, Eq, Deserialize)]
pub struct Metadata {
    pub settings: MetadataSettings,
    pub compiler: Compiler,
    pub output: Output,

    // Is not deserialized and should be filled manually
    #[serde(skip)]
    pub raw_settings: String,
}

#[derive(Debug, PartialEq, Eq, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MetadataSettings {
    pub compilation_target: BTreeMap<String, String>,
    pub optimizer: Optimizer,
    pub libraries: BTreeMap<String, String>,
    pub evm_version: Option<String>,
}

#[derive(Debug, PartialEq, Eq, Deserialize)]
pub struct Optimizer {
    pub enabled: Option<bool>,
    pub runs: Option<usize>,
}

#[derive(Debug, PartialEq, Eq, Deserialize)]
pub struct Compiler {
    pub version: String,
}

#[derive(Debug, PartialEq, Eq, Deserialize)]
pub struct Output {
    pub abi: serde_json::Value,
}

impl Files {
    fn extract_metadata_and_source_files(
        self,
    ) -> Result<(Metadata, BTreeMap<String, String>), anyhow::Error> {
        let metadata_content = self
            .0
            .get(METADATA_FILE_NAME)
            .ok_or_else(|| anyhow::anyhow!("file {} not found", METADATA_FILE_NAME))?;
        let metadata = Metadata::from_value(serde_json::from_str(metadata_content)?)?;

        let source_files: BTreeMap<String, String> = self
            .0
            .into_iter()
            .filter_map(|(name, content)| {
                name.strip_prefix(SOURCES_PREFIX)
                    .map(|s| (s.into(), content))
            })
            .collect();

        Ok((metadata, source_files))
    }
}

impl TryFrom<(Files, MatchType)> for Success {
    type Error = anyhow::Error;

    fn try_from((files, match_type): (Files, MatchType)) -> Result<Self, Self::Error> {
        let (metadata, source_files) = files.extract_metadata_and_source_files()?;

        metadata.into_success(source_files, None, match_type)
    }
}

impl Metadata {
    pub(super) fn from_value(value: serde_json::Value) -> Result<Self, anyhow::Error> {
        let mut metadata: Self = serde_json::from_value(value.clone())?;
        metadata.raw_settings = serde_json::to_string(&value["settings"])?;
        Ok(metadata)
    }

    pub(super) fn into_success(
        self,
        source_files: BTreeMap<String, String>,
        constructor_arguments: Option<Bytes>,
        match_type: MatchType,
    ) -> Result<Success, anyhow::Error> {
        let mut compiler_settings: serde_json::Map<String, serde_json::Value> =
            serde_json::from_str(&self.raw_settings)?;
        compiler_settings.remove("compilationTarget");
        let mut libraries: BTreeMap<String, BTreeMap<String, String>> = BTreeMap::new();
        for (name, address) in &self.settings.libraries {
            let (file, library) = name.rsplit_once(':').unwrap_or(("", name.as_str()));
            libraries
                .entry(file.to_string())
                .or_default()
                .insert(library.to_string(), address.clone());
        }
        compiler_settings.insert("libraries".into(), serde_json::to_value(libraries)?);

        let compiler_version = self.compiler.version;
        let (file_name, contract_name) = self
            .settings
            .compilation_target
            .into_iter()
            .next()
            .ok_or_else(|| anyhow::Error::msg("compilation target not found"))?;
        let evm_version = self.settings.evm_version;
        let optimization = self.settings.optimizer.enabled;
        let optimization_runs = self.settings.optimizer.runs;
        let contract_libraries: BTreeMap<String, String> = self.settings.libraries;
        let abi = serde_json::to_string(&self.output.abi)?;

        Ok(Success {
            file_name,
            contract_name,
            compiler_version,
            evm_version,
            // TODO: extract args
            constructor_arguments,
            contract_libraries,
            optimization,
            optimization_runs,
            abi,
            sources: source_files,
            compiler_settings: serde_json::to_string(&compiler_settings)?,
            match_type,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    const DEFAULT_METADATA: &str = r#"{
        "compiler": {
            "version": "0.8.14+commit.80d49f37"
        },
        "output": {
            "abi": [{"inputs":[],"name":"retrieve","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]
        },
        "settings": {
            "compilationTarget": {
                "example.sol": "Example"
            },
            "evmVersion": "london",
            "libraries": {
                "SafeMath": "0xFBe36e5cAD207d5fDee40E6568bb276a351f6713"
            },
            "optimizer": {
                "enabled": false,
                "runs": 200
            }
        }
    }"#;

    #[test]
    fn parse_metadata_from_files() {
        let files = Files(BTreeMap::from([
            (format!("{SOURCES_PREFIX}source.sol"), "content".into()),
            (METADATA_FILE_NAME.into(), DEFAULT_METADATA.into()),
        ]));
        let result = files.extract_metadata_and_source_files();

        let (metadata, files) = result.expect("parse metadata from files failed");
        assert_eq!(
            files,
            BTreeMap::from([("source.sol".into(), "content".into())]),
        );
        assert_eq!(
            metadata,
            Metadata {
                settings: MetadataSettings {
                    compilation_target: BTreeMap::from([("example.sol".into(), "Example".into())]),
                    optimizer: Optimizer { enabled: Some(false), runs: Some(200) },
                    libraries: BTreeMap::from([("SafeMath".into(), "0xFBe36e5cAD207d5fDee40E6568bb276a351f6713".into())]),
                    evm_version: Some("london".into()),
                },
                compiler: Compiler { version: "0.8.14+commit.80d49f37".to_string() },
                output: Output { abi: serde_json::json!([{"inputs":[],"name":"retrieve","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]) },
                raw_settings: "{\"compilationTarget\":{\"example.sol\":\"Example\"},\"evmVersion\":\"london\",\"libraries\":{\"SafeMath\":\"0xFBe36e5cAD207d5fDee40E6568bb276a351f6713\"},\"optimizer\":{\"enabled\":false,\"runs\":200}}".to_string(),
            }
        );

        let files = Files(BTreeMap::from([(
            format!("{SOURCES_PREFIX}source.sol"),
            "content".into(),
        )]));
        files
            .extract_metadata_and_source_files()
            .expect_err("Parsing files without metadata should fail");
    }

    #[test]
    fn parse_response_from_files() {
        let match_type = MatchType::Partial;
        let files = Files(BTreeMap::from([
            (format!("{SOURCES_PREFIX}source.sol"), "content".into()),
            (METADATA_FILE_NAME.into(), DEFAULT_METADATA.into()),
        ]));

        let verification_result =
            Success::try_from((files, match_type)).expect("parse response from files failed");
        assert_eq!(
            verification_result,
            Success {
                file_name: "example.sol".into(),
                contract_name: "Example".into(),
                compiler_version: "0.8.14+commit.80d49f37".into(),
                evm_version: Some("london".into()),
                constructor_arguments: None,
                contract_libraries: BTreeMap::from([("SafeMath".into(), "0xFBe36e5cAD207d5fDee40E6568bb276a351f6713".into())]),
                optimization: Some(false),
                optimization_runs: Some(200),
                abi: r#"[{"inputs":[],"name":"retrieve","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"}]"#.into(),
                sources: BTreeMap::from([("source.sol".into(), "content".into())]),
                compiler_settings: "{\"evmVersion\":\"london\",\"libraries\":{\"\":{\"SafeMath\":\"0xFBe36e5cAD207d5fDee40E6568bb276a351f6713\"}},\"optimizer\":{\"enabled\":false,\"runs\":200}}".to_string(),
                match_type,
            }
        );

        let files = Files(BTreeMap::from([(
            format!("{SOURCES_PREFIX}source.sol"),
            "content".into(),
        )]));
        Success::try_from((files, match_type))
            .expect_err("Parsing files without metadata should fail");
    }
}
