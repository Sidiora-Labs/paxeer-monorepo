//! Module for compiling solidity contracts using cmd args.
//! foundry_compilers compiler uses --standard-json method of input,
//! because it's very easy and convient, however --standard-json flag
//! was added only since 0.4.10 version. So, to compile older versions
//! we need convert functions for CompilerInput and CompilerOutput.

use super::solc_compiler::SolcInput;
use foundry_compilers::{
    artifacts::solc,
    error::{SolcError, SolcIoError},
};
use std::{
    collections::BTreeMap,
    path::{Component, Path},
    process::Stdio,
};
use tokio::process::Command;

pub(crate) fn validate_source_name(name: &Path) -> Result<(), SolcError> {
    let text = name.as_os_str().to_string_lossy();
    let bytes = text.as_bytes();
    let windows_prefix = bytes.len() >= 2 && bytes[0].is_ascii_alphabetic() && bytes[1] == b':';
    if text.is_empty()
        || text.contains('\0')
        || name.has_root()
        || text.starts_with('\\')
        || windows_prefix
        || text.split(['/', '\\']).any(|part| part == "..")
        || name.components().any(|component| {
            matches!(
                component,
                Component::Prefix(_) | Component::RootDir | Component::ParentDir
            )
        })
        || !matches!(name.components().next_back(), Some(Component::Normal(_)))
    {
        return Err(SolcError::Message(format!(
            "invalid source name {}: expected a relative file path without root, prefix or parent components",
            name.display()
        )));
    }
    Ok(())
}

pub async fn compile_using_cli(
    compiler_path: &Path,
    input: &SolcInput,
) -> Result<solc::CompilerOutput, SolcError> {
    let input = &input.0;
    let input_args = types::InputArgs::from(input);
    let input_files = types::InputFiles::try_from_compiler_input(input).await?;
    let output = Command::new(compiler_path)
        .args(input_args.build())
        .args(input_files.build()?)
        .stderr(Stdio::piped())
        .stdout(Stdio::piped())
        .output()
        .await
        .map_err(|err| SolcError::Io(SolcIoError::new(err, compiler_path)))?;

    let stderr = String::from_utf8_lossy(&output.stderr).to_string();
    let compiler_output = if output.status.success() && !output.stdout.is_empty() {
        let output_json: serde_json::Value = serde_json::from_slice(output.stdout.as_slice())?;
        if output_json.get("contracts").is_none() {
            return Ok(solc::CompilerOutput {
                errors: vec![compiler_error(format!(
                    "Compiler produced no contracts: {stderr}"
                ))],
                sources: BTreeMap::new(),
                contracts: BTreeMap::new(),
            });
        }
        let output_json: types::OutputJson = serde_json::from_value(output_json)?;
        if output_json.contracts.is_empty() {
            return Ok(solc::CompilerOutput {
                errors: vec![compiler_error(stderr)],
                sources: BTreeMap::new(),
                contracts: BTreeMap::new(),
            });
        }
        let mut compiler_output = solc::CompilerOutput::try_from(types::ConvertibleOutput {
            output_json,
            parent_dir: input_files.files_dir.path(),
            file_names: input_files.file_names,
        })?;
        if !stderr.is_empty() {
            let mut warning = compiler_error(stderr);
            warning.severity = solc::Severity::Warning;
            compiler_output.errors.push(warning);
        }
        compiler_output
    } else {
        solc::CompilerOutput {
            errors: vec![compiler_error(stderr)],
            sources: BTreeMap::new(),
            contracts: BTreeMap::new(),
        }
    };
    Ok(compiler_output)
}

fn compiler_error(message: String) -> solc::error::Error {
    solc::Error {
        source_location: None,
        secondary_source_locations: vec![],
        r#type: "".to_string(),
        component: "".to_string(),
        severity: solc::Severity::Error,
        error_code: None,
        message,
        formatted_message: None,
    }
}

mod types {
    use super::{serde_helpers, validate_source_name};
    use foundry_compilers::{artifacts::solc, error::SolcError};
    use serde::{Deserialize, Serialize};
    use std::{
        collections::{BTreeMap, HashMap},
        path::{Path, PathBuf},
    };
    use tempfile::TempDir;
    use tokio::io::AsyncWriteExt;

    #[derive(Debug, PartialEq, Eq)]
    pub struct InputArgs {
        pub optimize: bool,
        pub optimize_runs: Option<usize>,
        pub libs: BTreeMap<String, String>,
    }

    fn merge_libs(libraries: solc::Libraries) -> BTreeMap<String, String> {
        let mut result = BTreeMap::new();
        libraries
            .libs
            .into_iter()
            .for_each(|(_, libs)| result.extend(libs));
        result
    }

    impl From<&solc::SolcInput> for InputArgs {
        fn from(input: &solc::SolcInput) -> Self {
            let libs = merge_libs(input.settings.libraries.clone());
            InputArgs {
                optimize: input.settings.optimizer.enabled.unwrap_or(false),
                optimize_runs: input.settings.optimizer.runs,
                libs,
            }
        }
    }

    impl InputArgs {
        pub fn build(&self) -> Vec<String> {
            let mut vec: Vec<String> = vec![
                "--combined-json".to_string(),
                OutputContract::keys().join(","),
            ];
            if self.optimize {
                vec.push("--optimize".to_string());
            }
            if let Some(runs) = self.optimize_runs {
                // Note: --optimize-runs doesn't affect bytecode without --optimize
                vec.push("--optimize-runs".to_string());
                vec.push(runs.to_string());
            }
            if !self.libs.is_empty() {
                vec.push("--libraries".to_string());
                let libs: Vec<String> = self
                    .libs
                    .iter()
                    .map(|(name, address)| format!("{name}:{address}"))
                    .collect();
                vec.push(libs.join(","))
            }
            vec
        }
    }

    #[derive(Debug)]
    pub struct InputFiles {
        pub files_dir: TempDir,
        pub file_names: Vec<PathBuf>,
    }

    impl InputFiles {
        pub async fn try_from_compiler_input(input: &solc::SolcInput) -> Result<Self, SolcError> {
            if !input.sources.is_empty() {
                for name in input.sources.0.keys() {
                    validate_source_name(name)?;
                }
                let files_dir =
                    tempfile::tempdir().map_err(|e| SolcError::Message(e.to_string()))?;
                let mut file_names = Vec::new();
                for (name, source) in input.sources.iter() {
                    let file_path = files_dir.path().join(name);

                    if let Some(parent) = name.parent() {
                        let mut directory = files_dir.path().to_path_buf();
                        for component in parent.components() {
                            directory.push(component);
                            match tokio::fs::create_dir(&directory).await {
                                Ok(()) => {}
                                Err(error) if error.kind() == std::io::ErrorKind::AlreadyExists => {
                                    let metadata = tokio::fs::symlink_metadata(&directory)
                                        .await
                                        .map_err(|e| SolcError::Message(e.to_string()))?;
                                    if !metadata.is_dir() || metadata.file_type().is_symlink() {
                                        return Err(SolcError::Message(format!(
                                            "source directory is not a real directory: {}",
                                            directory.display()
                                        )));
                                    }
                                }
                                Err(error) => return Err(SolcError::Message(error.to_string())),
                            }
                        }
                    }
                    let mut file = tokio::fs::OpenOptions::new()
                        .write(true)
                        .create_new(true)
                        .open(&file_path)
                        .await
                        .map_err(|e| SolcError::Message(e.to_string()))?;
                    file_names.push(file_path);
                    file.write_all(source.content.as_bytes())
                        .await
                        .map_err(|e| SolcError::Message(e.to_string()))?;
                }

                Ok(InputFiles {
                    files_dir,
                    file_names,
                })
            } else {
                Err(SolcError::Message("no files were provided".to_string()))
            }
        }

        pub fn build(&self) -> Result<&Vec<PathBuf>, SolcError> {
            self.files_dir
                .path()
                .exists()
                .then_some(&self.file_names)
                .ok_or_else(|| {
                    SolcError::Message("temp dir with contracts doesn't exist".to_string())
                })
        }
    }

    #[derive(Debug, Serialize, Deserialize, PartialEq, Eq, Clone)]
    pub struct OutputContract {
        #[serde(deserialize_with = "serde_helpers::deserialize_abi_string")]
        pub abi: Vec<serde_json::Value>,
        pub bin: String,
        #[serde(rename = "bin-runtime")]
        pub bin_runtime: String,
    }

    impl OutputContract {
        fn keys() -> Vec<String> {
            vec![
                "abi".to_string(),
                "bin".to_string(),
                "bin-runtime".to_string(),
            ]
        }
    }

    impl TryFrom<OutputContract> for solc::Contract {
        type Error = SolcError;

        fn try_from(output_contract: OutputContract) -> Result<Self, Self::Error> {
            let contract = serde_json::json!({
                "abi": output_contract.abi,
                "evm": {
                    "bytecode": {
                        "object": output_contract.bin,
                    },
                    "deployedBytecode": {
                        "object": output_contract.bin_runtime,
                    },
                },
            });
            let contract: solc::Contract = serde_json::from_value(contract)?;
            Ok(contract)
        }
    }

    #[derive(Debug, Serialize, Deserialize, PartialEq, Eq, Clone)]
    pub struct OutputJson {
        pub contracts: HashMap<String, OutputContract>,
    }

    #[derive(Debug, PartialEq, Eq, Clone)]
    pub struct ConvertibleOutput<'a> {
        pub output_json: OutputJson,
        pub parent_dir: &'a Path,
        pub file_names: Vec<PathBuf>,
    }

    fn split_file_and_contract_names(name: String) -> (Option<String>, String) {
        name.rsplit_once(':')
            .map(|(file_name, contract_name)| {
                (Some(file_name.to_string()), contract_name.to_string())
            })
            .unwrap_or((None, name))
    }

    impl TryFrom<ConvertibleOutput<'_>> for solc::CompilerOutput {
        type Error = SolcError;

        fn try_from(convertible_output: ConvertibleOutput) -> Result<Self, Self::Error> {
            // The easiest option - we always know the compiled contract name
            let contracts = if convertible_output.file_names.len() == 1 {
                debug_assert!(convertible_output.output_json.contracts.len() == 1);

                let full_file_path = convertible_output.file_names[0].clone();
                let file_name = full_file_path
                    .strip_prefix(convertible_output.parent_dir)
                    // must never be the case, as we derive that full_file_path during InputFiles derivation
                    .map_err(|_| SolcError::Message("compiled file has unexpected prefix".into()))?
                    .to_string_lossy()
                    .to_string();

                let (name, output) = convertible_output
                    .output_json
                    .contracts
                    .into_iter()
                    .next()
                    .expect(
                    "number of output-json contracts must correspond to the number of input files (=1)",
                );
                let (_, contract_name) = split_file_and_contract_names(name);
                let contract: solc::Contract = output.try_into()?;

                BTreeMap::from([(file_name, BTreeMap::from([(contract_name, contract)]))])
            } else {
                // The main issue with several files is that some of the compilers (e.g., at least v0.4.8 and below)
                // return only contract names (and omit file names) in the output. E.g., they output
                // ```
                // "contracts": {
                //     "A": {...}
                // }
                // ```
                // But for verification we need those file names, as blockscout expects for the verified contract `file_name`
                // to correspond to some input file. Thus, for compilers which do not return file names it is impossible to verify
                // multi-file inputs in an easy way.
                //
                // Notice that at least v0.4.10 returns fully qualified names. E.g,
                // ```
                // "contracts": {
                //     "a.sol:A": {...}
                // }
                // ```
                // For those, we can derive both file and contract names, however we omit supporting such cases for now.
                // At the moment (2025) we have seen no verified contracts with compiler version v0.4.10 and below
                // which has more than a single file. Besides, if such contracts were found they could be verified by flattening
                // initial files. We can support them later if required.
                return Err(SolcError::Message(
                    "convertion of output with more than one file is not currently supported"
                        .into(),
                ));
            };

            let contracts_raw = serde_json::to_value(&contracts)?;
            let compiler_output = serde_json::json!(
                {
                    "contracts": contracts_raw
                }
            );
            let compiler_output = serde_json::from_value(compiler_output)?;
            Ok(compiler_output)
        }
    }
}

mod serde_helpers {
    use serde::de;

    pub fn deserialize_abi_string<'de, D>(
        deserializer: D,
    ) -> Result<Vec<serde_json::Value>, D::Error>
    where
        D: de::Deserializer<'de>,
    {
        let s: String = de::Deserialize::deserialize(deserializer)?;
        serde_json::from_str(&s).map_err(de::Error::custom)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::compiler::{DetailedVersion, Fetcher, ListFetcher};
    use foundry_compilers::Artifact;
    use hex::ToHex;
    use pretty_assertions::assert_eq;
    use std::{collections::HashSet, env::temp_dir, path::PathBuf, str::FromStr};
    use url::Url;
    use wiremock::{
        matchers::{method, path},
        Mock, MockServer, ResponseTemplate,
    };

    const DEFAULT_COMPILER_INPUT: &str = r#"
    {
        "language": "Solidity",
        "sources": {
            "a.sol": {
                "content": "pragma solidity >=0.4.5;\n\n\ncontract A {\n   function get_a() public returns (uint256) {\n        return 88888888888;\n    }\n}"
            }
        },
        "settings": {
            "optimizer": {
                "enabled": false,
                "runs": 200
            },
            "libraries": {
                "main.sol": {
                    "MyLib": "0x1234567890123456789012345678901234567890",
                    "OurLib": "0x0987654321098765432109876543210987654321"
                }
            },
            "outputSelection": {
                "*": {
                    "*": ["*"]
                }
            }
        }
    }
    "#;

    const DEFAULT_COMPILER_OUTPUT: &str = r#"
    {
        "contracts": {
            "A": {
                "abi": "[{\"inputs\":[],\"name\":\"get_a\",\"outputs\":[{\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
                "bin": "6060604052346000575b6096806100176000396000f30060606040526000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680634815918114603c575b6000565b346000576046605c565b6040518082815260200191505060405180910390f35b60006414b230ce3890505b905600a165627a7a7230582062ac15c74e3af0aec92b47f64d9c8909939b731732d5ee4163c6ed3af70806550029",
                "bin-runtime": "60606040526000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680634815918114603c575b6000565b346000576046605c565b6040518082815260200191505060405180910390f35b60006414b230ce3890505b905600a165627a7a7230582062ac15c74e3af0aec92b47f64d9c8909939b731732d5ee4163c6ed3af70806550029"
            }
        },
        "version": "0.4.8+commit.60cc1668"
    }
    "#;

    #[cfg(target_os = "linux")]
    pub const LIST_JSON: &str = r#"
    {
        "builds": [
            {
                "path": "https://github.com/blockscout/solc-bin/releases/download/v0.4.8%2Bcommit.60cc1668/solc",
                "longVersion": "v0.4.8+commit.60cc1668",
                "sha256": "9c64d0ea8373346342f462d0ee5a5a50f1946e209e971f8339af5722c5d65144"
            },
            {
                "path": "https://github.com/blockscout/solc-bin/releases/download/v0.4.10%2Bcommit.f0d539ae/solc",
                "longVersion": "v0.4.10+commit.f0d539ae",
                "sha256": "e1897b1985e5091555d97178bf4bd48e85b56d617561d0d5928414e4f007195b"
            }
        ]
    }
    "#;
    #[cfg(target_os = "macos")]
    pub const LIST_JSON: &str = r#"
        {
            "builds": [
                {
                    "path": "https://binaries.soliditylang.org/macosx-amd64/solc-macosx-amd64-v0.4.8+commit.60cc1668",
                    "longVersion": "v0.4.8+commit.60cc1668",
                    "sha256": "ebb64b8b8dd465bd53a52fa7063569115df176c7561ac4feb47004513e1df74b"
                },
                {
                    "path": "https://binaries.soliditylang.org/macosx-amd64/solc-macosx-amd64-v0.4.10+commit.f0d539ae",
                    "longVersion": "v0.4.10+commit.f0d539ae",
                    "sha256": "0x40f179e4d27201ab726669dd26d594cfe10bf4dd6117495ee49d26f0dda9ef42"
                }
            ]
        }
    "#;

    #[test]
    fn correct_input_args() {
        let input: SolcInput = serde_json::from_str(DEFAULT_COMPILER_INPUT).unwrap();

        let input_args = types::InputArgs::from(&input.0);
        let expected_args = types::InputArgs {
            optimize: false,
            optimize_runs: Some(200),
            libs: BTreeMap::from_iter([
                (
                    "MyLib".to_string(),
                    "0x1234567890123456789012345678901234567890".to_string(),
                ),
                (
                    "OurLib".to_string(),
                    "0x0987654321098765432109876543210987654321".to_string(),
                ),
            ]),
        };
        assert_eq!(input_args, expected_args);
        assert_eq!(
            input_args.build(),
            vec![
                "--combined-json",
                "abi,bin,bin-runtime",
                "--optimize-runs",
                "200",
                "--libraries",
                "MyLib:0x1234567890123456789012345678901234567890,OurLib:0x0987654321098765432109876543210987654321"
            ]
        );
    }

    #[tokio::test]
    async fn correct_input_files() {
        let input: SolcInput = serde_json::from_str(DEFAULT_COMPILER_INPUT).unwrap();

        let input_files = types::InputFiles::try_from_compiler_input(&input.0)
            .await
            .expect("failed to convert files");
        assert!(input_files.files_dir.path().exists());

        let expected_files: Vec<PathBuf> = vec!["a.sol"]
            .into_iter()
            .map(|name| input_files.files_dir.path().join(name))
            .collect();
        assert_eq!(input_files.file_names, expected_files);
        let string_args = input_files.build().expect("failed to build string args");
        assert_eq!(string_args, &expected_files);
    }

    #[tokio::test]
    async fn fails_if_parent_directory_component_exists() {
        let mut input: SolcInput = serde_json::from_str(DEFAULT_COMPILER_INPUT).unwrap();

        let file = input.0.sources.0.keys().next().unwrap();
        let content = input.0.sources.0.get(file).unwrap();

        let mut traversed_file = PathBuf::from("a/../");
        traversed_file.push(file);
        input
            .0
            .sources
            .0
            .insert(traversed_file.clone(), content.clone());

        types::InputFiles::try_from_compiler_input(&input.0)
            .await
            .expect_err("should fail");
    }

    #[test]
    fn correct_output() {
        let parent_dir = PathBuf::from("parent_dir");
        let file_names = vec![PathBuf::from("parent_dir/a.sol")];
        let output_json: types::OutputJson = serde_json::from_str(DEFAULT_COMPILER_OUTPUT).unwrap();
        let convertible_output = types::ConvertibleOutput {
            output_json: output_json.clone(),
            parent_dir: &parent_dir,
            file_names,
        };
        let compiler_output = solc::CompilerOutput::try_from(convertible_output)
            .expect("failed to convert output json");
        assert_eq!(compiler_output.contracts.len(), 1);
        let filename = compiler_output.contracts.iter().next().unwrap().0;
        assert_eq!(filename, &PathBuf::from("a.sol"));

        for (name, contract) in compiler_output.contracts_iter() {
            let initial_contract = output_json
                .contracts
                .get(name)
                .expect("invalid contract name");
            let abi = serde_json::to_value(&contract.abi).unwrap();
            let expected_abi = serde_json::to_value(&initial_contract.abi).unwrap();
            assert_eq!(abi, expected_abi);

            for (name, actual_bytecode, expected_bytecode) in [
                (
                    "creation bytecode",
                    contract.get_bytecode_bytes(),
                    &initial_contract.bin,
                ),
                (
                    "deployed bytecode",
                    contract.get_deployed_bytecode_bytes(),
                    &initial_contract.bin_runtime,
                ),
            ] {
                let bytecode: String = actual_bytecode
                    .unwrap_or_else(|| panic!("unlinked {name}"))
                    .encode_hex();
                assert_eq!(&bytecode, expected_bytecode, "wrong {}", name);
            }
        }
    }

    async fn get_solc(ver: &DetailedVersion) -> PathBuf {
        let mock_server = MockServer::start().await;
        Mock::given(method("GET"))
            .and(path("/"))
            .respond_with(ResponseTemplate::new(200).set_body_bytes(LIST_JSON))
            .mount(&mock_server)
            .await;
        let fetcher = ListFetcher::new(
            Url::parse(&mock_server.uri()).unwrap(),
            temp_dir(),
            None,
            None,
        )
        .await
        .expect("failed to build fetcher");
        fetcher
            .fetch(ver)
            .await
            .unwrap_or_else(|err| panic!("cannot fetch {ver} version: {err}"))
    }

    fn source(file: &str, content: &str) -> (PathBuf, solc::Source) {
        (file.into(), solc::Source::new(content))
    }

    #[tokio::test]
    async fn compile() {
        for ver in &["v0.4.8+commit.60cc1668", "v0.4.10+commit.f0d539ae"] {
            let version = DetailedVersion::from_str(ver).expect("valid version");
            let solc = get_solc(&version).await;

            let input: SolcInput = serde_json::from_str(DEFAULT_COMPILER_INPUT).unwrap();
            let output: solc::CompilerOutput = compile_using_cli(&solc, &input)
                .await
                .unwrap_or_else(|_| panic!("failed to compile contracts with {ver}"));
            assert!(
                !output.has_error(),
                "errors during compilation: {:?}",
                output.errors
            );
            let names: HashSet<String> =
                output.contracts_into_iter().map(|(name, _)| name).collect();
            let expected_names = HashSet::from_iter(["A".into()]);
            assert_eq!(names, expected_names);

            let input = SolcInput(solc::SolcInput {
                language: solc::SolcLanguage::Solidity,
                sources: solc::Sources(BTreeMap::from_iter([source(
                    "main.sol",
                    "contract A { function get_a() public returns (uint256) { return 1; } }",
                )])),
                settings: solc::Settings::default(),
            });
            let output = compile_using_cli(&solc, &input)
                .await
                .expect("should compile contracts with warnings");
            assert!(!output.has_error(), "{:?}", output.errors);
            assert!(output.errors.iter().any(|error| {
                error.severity == solc::Severity::Warning
                    && error.message.contains("compiler version")
            }));
            assert!(output.contracts[Path::new("main.sol")].contains_key("A"));

            for sources in [
                BTreeMap::from_iter([source("main.sol", "")]),
                BTreeMap::from_iter([source("main.sol", "some string")]),
            ] {
                let input = SolcInput(solc::SolcInput {
                    language: solc::SolcLanguage::Solidity,
                    sources: solc::Sources(sources),
                    settings: solc::Settings::default(),
                });
                let output: solc::CompilerOutput = compile_using_cli(&solc, &input)
                    .await
                    .expect("shouldn't return Err, but Ok with errors field");
                assert!(output.has_error());
                assert!(output.contracts.is_empty());
            }

            let input = SolcInput(solc::SolcInput {
                language: solc::SolcLanguage::Solidity,
                sources: solc::Sources(BTreeMap::new()),
                settings: solc::Settings::default(),
            });
            compile_using_cli(&solc, &input)
                .await
                .expect_err("should not compile empty files");
        }
    }
}
