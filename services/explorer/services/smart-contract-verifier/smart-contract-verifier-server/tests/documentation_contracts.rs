//! README contracts exercised with production loaders, matchers and wire codecs.
//! Compiler outputs are the committed integration fixtures, never a subprocess.

use blockscout_service_launcher::launcher::ConfigSettings;
use prost::Message;
use serde::{de::DeserializeOwned, Serialize};
use serde_json::{json, Value};
use smart_contract_verifier::{
    solidity, vyper, zksync, Error, FullyQualifiedName, Language, SolcInput, VerifyingContract,
};
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{self as v2};
use smart_contract_verifier_server::Settings;
use std::collections::{BTreeMap, HashMap};
use verification_common::verifier_alliance::{
    self, CompilationArtifacts, CreationCodeArtifacts, MatchBuilder, RuntimeCodeArtifacts,
};

// These private production modules are compiled directly so that documentation
// tests exercise the server's conversions without widening its public API.
#[path = "../src/proto.rs"]
mod proto;
#[path = "../src/settings.rs"]
mod settings;
#[path = "../src/types/mod.rs"]
mod types;
mod services {
    pub mod common {
        include!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/src/services/common.rs"
        ));
    }
}
mod zksync_response {
    include!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/src/services/zksync_solidity_verifier.rs"
    ));

    pub fn convert(
        result: Result<zksync::VerificationResult, zksync::Error>,
    ) -> Result<VerifyResponse, Status> {
        process_verification_result(result)
    }
}

const README: &str = include_str!("../README.md");
const MANUAL: &str = include_str!("test_cases_solidity/libraries_manually_linked.json");
const LINKED: &str = include_str!("test_cases_solidity/libraries_linked_by_compiler.json");
const BATCH: &str = include_str!("test_cases_batch_solidity/basic_standard_json.json");
const ZKSYNC: &str = include_str!("test_cases_zksync_solidity/simple_standard_json.json");
const LIBRARY: &str = "contracts/1_Storage.sol:Journal";
const ADDRESS: &str = "0x7d53f102f4d4aa014db4e10d6deec2009b3cda6b";

fn blocks(section: &str, language: &str) -> Vec<&'static str> {
    let section = README.split_once(section).expect("documented section").1;
    let section = section.split("\n## ").next().unwrap();
    let marker = format!("```{language}\n");
    section
        .split(&marker)
        .skip(1)
        .map(|block| block.split_once("```").unwrap().0.trim())
        .collect()
}

fn example(section: &str) -> Value {
    serde_json::from_str(blocks(section, "json")[0]).expect("valid README JSON")
}

fn parse<T: DeserializeOwned>(value: Value) -> T {
    serde_json::from_value(value).unwrap()
}

fn code(value: &Value) -> Vec<u8> {
    blockscout_display_bytes::decode_hex(value.as_str().unwrap()).unwrap()
}

// Real generated protobuf codecs followed by the generated HTTP JSON consumer.
// No response message or transport implementation is substituted by a double.
fn wire<T>(value: T) -> (T, Value)
where
    T: Message + Default + Serialize + DeserializeOwned,
{
    let encoded = value.encode_to_vec();
    let decoded = T::decode(encoded.as_slice()).unwrap();
    let http = serde_json::to_vec(&decoded).unwrap();
    let consumer: T = serde_json::from_slice(&http).unwrap();
    let json = serde_json::from_slice(&http).unwrap();
    (consumer, json)
}

fn assert_excerpt(expected: &Value, actual: &Value) {
    match expected {
        Value::Object(fields) => {
            for (key, value) in fields {
                assert!(actual.get(key).is_some(), "missing documented key {key}");
                assert_excerpt(value, &actual[key]);
            }
        }
        Value::Array(items) => {
            let actual = actual.as_array().expect("documented array");
            assert_eq!(items.len(), actual.len());
            for (expected, actual) in items.iter().zip(actual) {
                assert_excerpt(expected, actual);
            }
        }
        _ => assert_eq!(expected, actual),
    }
}

fn load_settings(file: &str, environment: HashMap<String, String>) -> Settings {
    // The same config sources, prefix and separator as launcher 0.17.0's
    // ConfigSettings::build, with an explicit source to isolate process env.
    let settings: Settings = config::Config::builder()
        .add_source(config::File::from_str(file, config::FileFormat::Toml))
        .add_source(
            config::Environment::with_prefix(Settings::SERVICE_NAME)
                .separator("__")
                .source(Some(environment)),
        )
        .build()
        .unwrap()
        .try_deserialize()
        .unwrap();
    settings.validate().unwrap();
    settings
}

#[test]
fn undocumented_enabled_zksync() {
    let toml = blocks("### Configuration file", "toml")[0];
    let actual = load_settings(toml, HashMap::new());
    let defaults = Settings::default();
    assert!(defaults.zksync_solidity.enabled);
    assert_eq!(actual.zksync_solidity, defaults.zksync_solidity);
    assert_eq!(actual.tracing, defaults.tracing);
    assert_eq!(actual.compilers.max_threads.get(), 8);
    assert!(actual.solidity.enabled && actual.vyper.enabled && actual.sourcify.enabled);
    assert!(actual
        .zksync_solidity
        .evm_compilers_dir
        .ends_with("zksync-solc-compilers"));
    assert!(actual
        .zksync_solidity
        .zk_compilers_dir
        .ends_with("zksync-zksolc-compilers"));
    assert_eq!(
        actual
            .zksync_solidity
            .evm_refresh_versions_schedule
            .to_string(),
        "0 0 * * * * *"
    );
    assert_eq!(
        actual
            .zksync_solidity
            .zk_refresh_versions_schedule
            .to_string(),
        "0 0 * * * * *"
    );

    let environment: HashMap<String, String> = blocks("### Environment variables", "sh")[0]
        .lines()
        .map(|line| {
            let (key, value) = line.split_once('=').unwrap();
            (key.into(), value.trim_matches('\'').into())
        })
        .collect();
    let env_settings = load_settings("", environment.clone());
    assert_eq!(env_settings.zksync_solidity, actual.zksync_solidity);
    assert_eq!(env_settings.tracing, actual.tracing);
    let mut overridden = environment;
    overridden.insert(
        "SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ENABLED".into(),
        "false".into(),
    );
    overridden.insert(
        "SMART_CONTRACT_VERIFIER__TRACING__FORMAT".into(),
        "json".into(),
    );
    let overridden = load_settings(toml, overridden);
    assert!(!overridden.zksync_solidity.enabled);
    assert_eq!(
        serde_json::to_value(overridden.tracing).unwrap()["format"],
        "json"
    );

    let s3 = blocks("#### List and S3 fetchers", "toml")[0];
    for fetcher in ["evm_fetcher", "era_evm_fetcher", "zk_fetcher"] {
        let s3 = s3.replace("zk_fetcher", fetcher);
        let settings = load_settings(&s3, HashMap::new());
        let mut expected: Value = json!({});
        expected["zksync_solidity"] = json!({fetcher: {"s3": {
            "bucket": "compiler-inventory", "region": "us-east-1"
        }}});
        let expected: Settings = parse(expected);
        assert_eq!(settings, expected);
        let prefix = format!(
            "SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__{}__S3__",
            fetcher.to_uppercase()
        );
        let env = HashMap::from([
            (format!("{prefix}BUCKET"), "compiler-inventory".into()),
            (format!("{prefix}REGION"), "us-east-1".into()),
        ]);
        assert_eq!(load_settings("", env), expected);
    }
    assert!(
        README.contains("Initial version-list or S3 fetch failures propagate as startup failures")
    );
    assert!(README.contains("omitting the\nsection leaves it enabled"));
}

#[test]
fn obsolete_request_examples() {
    let multipart = example("### Solidity Multi-Part files");
    assert!(multipart.get("libraries").is_none());
    let (request, _) = wire::<v2::VerifySolidityMultiPartRequest>(parse(multipart));
    let request: solidity::multi_part::VerificationRequest = request.try_into().unwrap();
    assert_eq!(request.contract.chain_id.as_deref(), Some("125"));
    let inputs: Vec<SolcInput> = request.content.into();
    assert_eq!(inputs.len(), 1);
    assert_eq!(inputs[0].0.settings.optimizer.enabled, Some(true));
    assert_eq!(inputs[0].0.settings.optimizer.runs, Some(200));
    assert!(inputs[0].0.settings.libraries.libs.is_empty());

    let (request, _) = wire::<v2::VerifySolidityStandardJsonRequest>(parse(example(
        "### Solidity Standard-JSON input",
    )));
    let request: solidity::standard_json::VerificationRequest = request.try_into().unwrap();
    assert_eq!(request.content.0.settings.optimizer.enabled, Some(true));
    assert!(request.content.0.settings.libraries.libs.is_empty());

    let multipart = example("### Vyper Multi-Part files");
    assert!(multipart.get("optimizations").is_none());
    let (request, _) = wire::<v2::VerifyVyperMultiPartRequest>(parse(multipart));
    let request: vyper::multi_part::VerificationRequest = request.try_into().unwrap();
    let input = smart_contract_verifier::VyperInput::try_from(request.content).unwrap();
    assert_eq!(input.settings.optimize, None);
    assert_eq!(input.settings.evm_version.unwrap().to_string(), "istanbul");
    let (request, _) =
        wire::<v2::VerifyVyperStandardJsonRequest>(parse(example("### Vyper Standard-JSON input")));
    let request: vyper::standard_json::VerificationRequest = request.try_into().unwrap();
    assert_eq!(request.content.settings.optimize, Some(false));
    assert_eq!(request.content.settings.bytecode_metadata, Some(true));

    let (request, _) = wire::<v2::BatchVerifySolidityMultiPartRequest>(parse(example(
        "### Solidity batch verification",
    )));
    let request: solidity::multi_part::BatchVerificationRequest = request.try_into().unwrap();
    assert_eq!(request.contracts.len(), 1);
    assert_eq!(request.content.optimization_runs, Some(200));
    let fixture: Value = serde_json::from_str(BATCH).unwrap();
    let (request, _) =
        wire::<v2::BatchVerifySolidityStandardJsonRequest>(parse(fixture["request"].clone()));
    let request: solidity::standard_json::BatchVerificationRequest = request.try_into().unwrap();
    assert_eq!(request.contracts.len(), 1);

    let (request, _) = wire::<v2::zksync::solidity::VerifyStandardJsonRequest>(parse(example(
        "### ZkSync Solidity Standard-JSON input",
    )));
    let request: zksync::VerificationRequest =
        types::zksolc_standard_json::VerifyStandardJsonRequestWrapper::from(request)
            .try_into()
            .unwrap();
    assert_eq!(request.code.as_ref(), &[0]);
    assert_eq!(request.zk_compiler.to_string(), "v1.4.0");
    assert_eq!(request.solc_compiler.to_string(), "v0.8.17+commit.8df45f5f");
    for link in [
        "../smart-contract-verifier-proto/proto/v2/smart-contract-verifier.proto",
        "../smart-contract-verifier-proto/proto/v2/zksync-solidity.proto",
        "tests/test_cases_vyper/standard_json_with_interfaces.json",
        "tests/test_cases_batch_solidity/basic_standard_json.json",
    ] {
        assert!(README.contains(link));
        assert!(std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join(link)
            .is_file());
    }
}

// Rehydrate the committed compiler artifacts and bytecode, then ask the real
// match engine to discover links. Never supply fixture expectations as MatchValues.
fn library_result(fixture: &Value) -> VerifyingContract {
    let creation = code(&fixture["creation_bytecode"]);
    let runtime = code(&fixture["deployed_bytecode"]);
    let compilation_artifacts: CompilationArtifacts =
        parse(fixture["expected_compiler_artifacts"].clone());
    let creation_code_artifacts: CreationCodeArtifacts =
        parse(fixture["expected_creation_input_artifacts"].clone());
    let runtime_code_artifacts: RuntimeCodeArtifacts =
        parse(fixture["expected_deployed_bytecode_artifacts"].clone());
    let creation_match = verifier_alliance::verify_creation_code(
        &creation,
        creation.clone(),
        &creation_code_artifacts,
        &compilation_artifacts,
    )
    .unwrap()
    .expect("committed creation code matches");
    let runtime_match =
        verifier_alliance::verify_runtime_code(&runtime, runtime.clone(), &runtime_code_artifacts)
            .unwrap()
            .expect("committed runtime code matches");
    assert!(creation_match.metadata_match && runtime_match.metadata_match);
    VerifyingContract {
        fully_qualified_name: FullyQualifiedName::from_file_and_contract_names(
            fixture["file_name"].as_str().unwrap(),
            fixture["contract_name"].as_str().unwrap(),
        ),
        language: Language::Solidity,
        compiler_version: fixture["compiler_version"].as_str().unwrap().into(),
        compiler_settings: fixture["input"]["settings"].clone(),
        sources: fixture["input"]["sources"]
            .as_object()
            .unwrap()
            .iter()
            .map(|(name, source)| (name.clone(), source["content"].as_str().unwrap().into()))
            .collect(),
        creation_code: creation,
        runtime_code: runtime,
        compilation_artifacts,
        creation_code_artifacts,
        runtime_code_artifacts,
        creation_match: Some(creation_match),
        runtime_match: Some(runtime_match),
        is_blueprint: false,
    }
}

#[test]
fn manual_library_response_docs() {
    let mut returned = Vec::new();
    for (fixture, compiler_linked) in [(MANUAL, false), (LINKED, true)] {
        let fixture: Value = serde_json::from_str(fixture).unwrap();
        let result = library_result(&fixture);
        assert_eq!(
            !result.compiler_settings["libraries"]
                .as_object()
                .unwrap()
                .is_empty(),
            compiler_linked
        );
        assert_eq!(
            result
                .creation_match
                .as_ref()
                .unwrap()
                .values
                .libraries
                .is_empty(),
            compiler_linked
        );
        assert_eq!(
            result
                .runtime_match
                .as_ref()
                .unwrap()
                .values
                .libraries
                .is_empty(),
            compiler_linked
        );
        // Exercise each match independently: neither creation-only nor runtime-only
        // consumers may lose a manually linked address.
        for (creation, runtime) in [(true, true), (true, false), (false, true)] {
            let mut result = result.clone();
            if !creation {
                result.creation_match = None;
            }
            if !runtime {
                result.runtime_match = None;
            }
            let response =
                types::verification_result::process_verification_result(vec![result], &[]).unwrap();
            let (response, json) = wire(response);
            assert_eq!(response.status(), v2::verify_response::Status::Success);
            let source = response.source.unwrap();
            let libraries = BTreeMap::from([(LIBRARY.to_string(), ADDRESS.to_string())]);
            assert_eq!(source.libraries, libraries);
            assert_eq!(
                json["source"]["libraries"],
                serde_json::to_value(&libraries).unwrap()
            );
            let settings: Value = serde_json::from_str(&source.compiler_settings).unwrap();
            assert_eq!(
                !settings["libraries"].as_object().unwrap().is_empty(),
                compiler_linked
            );
            returned.push(source.libraries);
        }
    }
    assert!(returned.windows(2).all(|pair| pair[0] == pair[1]));
    assert!(README.contains("Preserve `source.libraries`"));
    assert!(README.contains("empty compiler-settings library map does not mean"));
    assert!(README.contains(ADDRESS) && README.contains(LIBRARY));
}

fn batch_result() -> VerifyingContract {
    let fixture: Value = serde_json::from_str(BATCH).unwrap();
    let success = &fixture["response"]["success"];
    let creation = code(&success["creationCode"]);
    let runtime = code(&success["runtimeCode"]);
    let compilation_artifacts = parse(success["compilationArtifacts"].clone());
    let creation_code_artifacts = parse(success["creationCodeArtifacts"].clone());
    let runtime_code_artifacts = parse(success["runtimeCodeArtifacts"].clone());
    let creation_match = verifier_alliance::verify_creation_code(
        &creation,
        creation.clone(),
        &creation_code_artifacts,
        &compilation_artifacts,
    )
    .unwrap()
    .unwrap();
    let runtime_match =
        verifier_alliance::verify_runtime_code(&runtime, runtime.clone(), &runtime_code_artifacts)
            .unwrap()
            .unwrap();
    VerifyingContract {
        fully_qualified_name: FullyQualifiedName::from_file_and_contract_names(
            success["fileName"].as_str().unwrap(),
            success["contractName"].as_str().unwrap(),
        ),
        language: Language::Solidity,
        compiler_version: success["compilerVersion"].as_str().unwrap().into(),
        compiler_settings: success["compilerSettings"].clone(),
        sources: parse(success["sources"].clone()),
        creation_code: creation,
        runtime_code: runtime,
        compilation_artifacts,
        creation_code_artifacts,
        runtime_code_artifacts,
        creation_match: Some(creation_match),
        runtime_match: Some(runtime_match),
        is_blueprint: false,
    }
}

fn mismatching_results() -> smart_contract_verifier::VerificationResult {
    let mut result = batch_result();
    let mut deployed = result.runtime_code.clone();
    deployed[0] ^= 1;
    result.creation_match = None;
    result.runtime_match = verifier_alliance::verify_runtime_code(
        &deployed,
        result.runtime_code.clone(),
        &result.runtime_code_artifacts,
    )
    .unwrap();
    assert!(
        result.runtime_match.is_none(),
        "changed executable code must not match"
    );
    result
        .runtime_match
        .is_some()
        .then_some(result)
        .into_iter()
        .collect()
}

fn zk_result() -> zksync::VerificationResult {
    let fixture: Value = serde_json::from_str(ZKSYNC).unwrap();
    let runtime = code(&fixture["deployed_code"]);
    let mut creation = runtime.clone();
    creation.extend(code(&fixture["constructor_arguments"]));
    let compilation_artifacts = parse(fixture["expected_compilation_artifacts"].clone());
    let creation_code_artifacts = parse(fixture["expected_creation_code_artifacts"].clone());
    let runtime_code_artifacts = parse(fixture["expected_runtime_code_artifacts"].clone());
    let runtime_match = MatchBuilder::new(&runtime, runtime.clone())
        .unwrap()
        .set_has_cbor_auxdata(true)
        .apply_runtime_code_transformations(&runtime_code_artifacts)
        .unwrap()
        .verify_and_build()
        .unwrap();
    let creation_match = MatchBuilder::new(&creation, runtime)
        .unwrap()
        .set_has_cbor_auxdata(true)
        .apply_creation_code_transformations(&creation_code_artifacts, &compilation_artifacts)
        .unwrap()
        .verify_and_build()
        .unwrap();
    assert_eq!(
        Value::from(creation_match.values.clone()),
        fixture["expected_creation_values"]
    );
    assert_eq!(
        Value::from(runtime_match.values.clone()),
        fixture["expected_runtime_values"]
    );
    zksync::VerificationResult {
        zk_compiler: "zksolc".into(),
        zk_compiler_version: fixture["zk_compiler_version"]
            .as_str()
            .unwrap()
            .parse()
            .unwrap(),
        evm_compiler: "solc".into(),
        evm_compiler_version: fixture["evm_compiler_version"]
            .as_str()
            .unwrap()
            .parse()
            .unwrap(),
        language: v2::zksync::solidity::verification_success::Language::Solidity,
        compiler_settings: fixture["input"]["settings"].clone(),
        sources: parse(fixture["expected_sources"].clone()),
        successes: vec![zksync::VerificationSuccess {
            file_path: fixture["file_name"].as_str().unwrap().into(),
            contract_name: fixture["contract_name"].as_str().unwrap().into(),
            compilation_artifacts,
            creation_code_artifacts,
            runtime_code_artifacts,
            creation_match: Some(creation_match),
            runtime_match,
        }],
        failures: vec![],
    }
}

#[test]
fn response_family_docs() {
    let fixture: Value = serde_json::from_str(MANUAL).unwrap();
    let solo = types::verification_result::process_verification_result(
        vec![library_result(&fixture)],
        &[],
    )
    .unwrap();
    let (solo, solo_json) = wire(solo);
    assert_excerpt(&example("### Solo success"), &solo_json);
    assert_eq!(solo.status(), v2::verify_response::Status::Success);
    let source = solo.source.unwrap();
    for artifact in [
        source.compiler_settings,
        source.abi.unwrap(),
        source.compilation_artifacts.unwrap(),
        source.creation_input_artifacts.unwrap(),
        source.deployed_bytecode_artifacts.unwrap(),
    ] {
        serde_json::from_str::<Value>(&artifact).unwrap();
    }
    let failure =
        types::verification_result::process_verification_result(mismatching_results(), &[])
            .unwrap();
    let (failure, failure_json) = wire(failure);
    assert_excerpt(&example("### Solo verification failure"), &failure_json);
    assert_eq!(failure.status(), v2::verify_response::Status::Failure);
    assert!(failure.source.is_none());

    let batch = types::batch_verification::process_verification_results(vec![vec![batch_result()]])
        .unwrap();
    let (batch, batch_json) = wire(batch);
    let examples = blocks("### Batch responses", "json");
    assert_excerpt(&serde_json::from_str(examples[0]).unwrap(), &batch_json);
    assert!(batch_json.get("status").is_none() && batch_json.get("source").is_none());
    let v2::batch_verify_response::VerificationResult::ContractVerificationResults(results) =
        batch.verification_result.unwrap()
    else {
        panic!("batch results required")
    };
    assert_eq!(results.items.len(), 1);
    let v2::contract_verification_result::VerificationResult::Success(success) = results
        .items
        .into_iter()
        .next()
        .unwrap()
        .verification_result
        .unwrap()
    else {
        panic!("batch success required")
    };
    for artifact in [
        success.compiler_settings,
        success.compilation_artifacts,
        success.creation_code_artifacts,
        success.runtime_code_artifacts,
        success.creation_match_details.unwrap().values,
        success.runtime_match_details.unwrap().transformations,
    ] {
        serde_json::from_str::<Value>(&artifact).unwrap();
    }
    let (_, batch_failure) = wire(
        types::batch_verification::process_verification_results(vec![mismatching_results()])
            .unwrap(),
    );
    assert_excerpt(&serde_json::from_str(examples[1]).unwrap(), &batch_failure);
    let (_, mixed) = wire(
        types::batch_verification::process_verification_results(vec![
            vec![batch_result()],
            mismatching_results(),
        ])
        .unwrap(),
    );
    assert_eq!(
        mixed["contractVerificationResults"]["items"]
            .as_array()
            .unwrap()
            .len(),
        2
    );
    assert!(mixed["contractVerificationResults"]["items"][0]
        .get("success")
        .is_some());
    assert!(mixed["contractVerificationResults"]["items"][1]
        .get("failure")
        .is_some());
    let (_, compilation) = wire(
        types::batch_verification::process_error(Error::Compilation(vec![
            "Invalid standard json".into()
        ]))
        .unwrap(),
    );
    assert_excerpt(&serde_json::from_str(examples[2]).unwrap(), &compilation);

    let (zk, zk_json) = wire(zksync_response::convert(Ok(zk_result())).unwrap());
    let examples = blocks("### ZkSync responses", "json");
    assert_excerpt(&serde_json::from_str(examples[0]).unwrap(), &zk_json);
    assert!(zk_json.get("status").is_none());
    let v2::zksync::solidity::verify_response::VerifyResponse::VerificationSuccess(success) =
        zk.verify_response.unwrap()
    else {
        panic!("ZkSync success required")
    };
    for artifact in [
        success.compiler_settings,
        success.compilation_artifacts,
        success.creation_code_artifacts,
        success.runtime_code_artifacts,
        success.creation_match.unwrap().values,
        success.runtime_match.unwrap().transformations,
    ] {
        serde_json::from_str::<Value>(&artifact).unwrap();
    }
    let mut mismatch = zk_result();
    let fixture: Value = serde_json::from_str(ZKSYNC).unwrap();
    let compiled = code(&fixture["deployed_code"]);
    let mut deployed = compiled.clone();
    deployed[0] ^= 1;
    let artifacts: RuntimeCodeArtifacts = parse(fixture["expected_runtime_code_artifacts"].clone());
    let observed = MatchBuilder::new(&deployed, compiled)
        .unwrap()
        .set_has_cbor_auxdata(true)
        .apply_runtime_code_transformations(&artifacts)
        .unwrap()
        .verify_and_build();
    assert!(
        observed.is_none(),
        "changed ZkSync executable code must not match"
    );
    mismatch.successes.retain(|_| observed.is_some());
    let (_, failure) = wire(zksync_response::convert(Ok(mismatch)).unwrap());
    assert_excerpt(&serde_json::from_str(examples[1]).unwrap(), &failure);
    let (_, compilation) = wire(
        zksync_response::convert(Err(zksync::Error::Compilation(vec![
            "Invalid standard json".into(),
        ])))
        .unwrap(),
    );
    assert_excerpt(&serde_json::from_str(examples[2]).unwrap(), &compilation);

    let solo_error =
        types::verification_result::process_error(Error::CompilerNotFound("unavailable".into()))
            .unwrap_err();
    let batch_error =
        types::batch_verification::process_error(Error::CompilerNotFound("unavailable".into()))
            .unwrap_err();
    let zk_error =
        zksync_response::convert(Err(zksync::Error::ZkCompilerNotFound("unavailable".into())))
            .unwrap_err();
    for error in [solo_error, batch_error, zk_error] {
        assert_eq!(error.code(), tonic::Code::InvalidArgument);
    }
    let solo_error = types::verification_result::process_error(Error::Internal(anyhow::anyhow!(
        "download failed"
    )))
    .unwrap_err();
    let batch_error = types::batch_verification::process_error(Error::Internal(anyhow::anyhow!(
        "download failed"
    )))
    .unwrap_err();
    let zk_error = zksync_response::convert(Err(zksync::Error::Internal(anyhow::anyhow!(
        "download failed"
    ))))
    .unwrap_err();
    for error in [solo_error, batch_error, zk_error] {
        assert_eq!(error.code(), tonic::Code::Internal);
    }
    assert!(README.contains("HTTP 200 / gRPC OK"));
    assert!(README.contains("HTTP 400") && README.contains("HTTP 500"));
}

#[actix_web::test]
async fn undocumented_enabled_zksync_startup_and_transport() {
    use actix_web::{http::StatusCode, test, web, App, HttpRequest, HttpResponse, HttpServer};
    use std::sync::{Arc, Mutex};
    use tokio::sync::Semaphore;
    use v2::solidity_verifier_actix::route_solidity_verifier;
    use v2::zksync::solidity::zk_sync_solidity_verifier_actix::route_zk_sync_solidity_verifier;

    // The peer serves compiler catalogue data only. Both verifiers and their
    // generated HTTP routes are the production implementations.
    let calls = Arc::new(Mutex::new(Vec::<String>::new()));
    let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    let observed = calls.clone();
    let catalogue = HttpServer::new(move || {
        let observed = observed.clone();
        App::new().default_service(web::to(move |request: HttpRequest| {
            observed.lock().unwrap().push(request.path().to_owned());
            async move {
                if request.path() == "/unavailable" {
                    HttpResponse::ServiceUnavailable().finish()
                } else {
                    HttpResponse::Ok().json(json!({"builds": []}))
                }
            }
        }))
    })
    .workers(1)
    .listen(listener)
    .unwrap()
    .run();
    let handle = catalogue.handle();
    let server = actix_web::rt::spawn(catalogue);
    let directory = tempfile::tempdir().unwrap();
    let config = json!({
        "zksync_solidity": {
            "evm_compilers_dir": directory.path().join("solc"),
            "zk_compilers_dir": directory.path().join("zksolc"),
            "evm_fetcher": {"list": {"list_url": format!("http://{address}/solc")}},
            "era_evm_fetcher": {"list": {"list_url": format!("http://{address}/era-solc")}},
            "zk_fetcher": {"list": {"list_url": format!("http://{address}/zksolc")}}
        }
    });
    let settings: settings::Settings = parse(config.clone());
    let service = Arc::new(
        zksync_response::Service::new(settings.zksync_solidity, Arc::new(Semaphore::new(1)))
            .await
            .unwrap(),
    );
    assert_eq!(*calls.lock().unwrap(), ["/solc", "/era-solc", "/zksolc"]);
    for (fetcher, context) in [
        ("evm_fetcher", "zksync solc fetcher initialization"),
        ("era_evm_fetcher", "zksync era solc fetcher initialization"),
        ("zk_fetcher", "zksync zksolc fetcher initialization"),
    ] {
        let mut failed = config.clone();
        failed["zksync_solidity"][fetcher] =
            json!({"list": {"list_url": format!("http://{address}/unavailable")}});
        let settings: settings::Settings = parse(failed);
        let failure =
            zksync_response::Service::new(settings.zksync_solidity, Arc::new(Semaphore::new(1)))
                .await;
        let error = match failure {
            Ok(_) => panic!("initial catalogue failure must stop startup"),
            Err(error) => error,
        };
        assert!(format!("{error:#}").contains(context), "{error:#}");
    }
    let settings: Settings = parse(json!({"solidity": {
        "compilers_dir": directory.path().join("ordinary-solc"),
        "fetcher": {"list": {"list_url": format!("http://{address}/solc")}}
    }}));
    let solidity = Arc::new(
        smart_contract_verifier_server::SolidityVerifierService::new(
            settings.solidity,
            Arc::new(Semaphore::new(1)),
        )
        .await
        .unwrap(),
    );
    let app = test::init_service(App::new().configure(|config| {
        route_zk_sync_solidity_verifier(config, service.clone());
        route_solidity_verifier(config, solidity.clone());
    }))
    .await;
    let response = test::TestRequest::get()
        .uri("/api/v2/zksync-verifier/solidity/versions")
        .send_request(&app)
        .await;
    assert_eq!(response.status(), StatusCode::OK);
    let versions: v2::zksync::solidity::ListCompilersResponse =
        test::read_body_json(response).await;
    assert!(versions.zk_compilers.is_empty() && versions.solc_compilers.is_empty());

    for (section, route, family) in [
        (
            "### Solidity Standard-JSON input",
            "/api/v2/verifier/solidity/sources:verify-standard-json",
            "status",
        ),
        (
            "### ZkSync Solidity Standard-JSON input",
            "/api/v2/zksync-verifier/solidity/sources:verify-standard-json",
            "compilationFailure",
        ),
    ] {
        let mut request = example(section);
        request["input"] = json!("not JSON");
        let response = test::TestRequest::post()
            .uri(route)
            .set_json(&request)
            .send_request(&app)
            .await;
        assert_eq!(response.status(), StatusCode::OK);
        let body: Value = test::read_body_json(response).await;
        if family == "status" {
            assert_eq!(body[family], "FAILURE");
        } else {
            assert!(body[family]["message"]
                .as_str()
                .unwrap()
                .contains("valid standard json"));
        }
        request["input"] = example(section)["input"].clone();
        let code_key = if family == "status" {
            "bytecode"
        } else {
            "code"
        };
        request[code_key] = json!("not hex");
        let response = test::TestRequest::post()
            .uri(route)
            .set_json(&request)
            .send_request(&app)
            .await;
        assert_eq!(response.status(), StatusCode::BAD_REQUEST);
        let body: Value = test::read_body_json(response).await;
        assert_eq!(body["code"], 3);
        assert!(!body["message"].as_str().unwrap().is_empty());
    }
    let fixture: Value = serde_json::from_str(BATCH).unwrap();
    let mut request = fixture["request"].clone();
    request["input"] = json!("not JSON");
    let response = test::TestRequest::post()
        .uri("/api/v2/verifier/solidity/sources:batch-verify-standard-json")
        .set_json(&request)
        .send_request(&app)
        .await;
    assert_eq!(response.status(), StatusCode::OK);
    let body: v2::BatchVerifyResponse = test::read_body_json(response).await;
    assert!(matches!(
        body.verification_result,
        Some(v2::batch_verify_response::VerificationResult::CompilationFailure(_))
    ));
    handle.stop(true).await;
    server.await.unwrap().unwrap();
}
