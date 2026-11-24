use smart_contract_verifier::{
    solidity, DetailedVersion, EvmCompilersPool, ListFetcher, RequestParseError, SolcCompiler,
    SolcInput,
};
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{
    BatchVerifySolidityMultiPartRequest, BatchVerifySolidityStandardJsonRequest, BytecodeType,
    VerifySolidityMultiPartRequest, VerifySolidityStandardJsonRequest,
};
use std::{collections::BTreeMap, path::Path, sync::Arc};
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpListener,
    sync::Semaphore,
};

const VERSIONS: [&str; 2] = ["v0.4.8+commit.60cc1668", "v0.4.10+commit.f0d539ae"];
const SOURCE: &str = "pragma solidity ^0.4.0; contract Sentinel {}";
const SENTINEL: &[u8] = b"persistent service state must survive verification";

fn standard_input(name: &str) -> String {
    serde_json::json!({
        "language": "Solidity",
        "sources": {name: {"content": SOURCE}},
        "settings": {}
    })
    .to_string()
}

fn multipart_request(name: &str, version: &str) -> VerifySolidityMultiPartRequest {
    VerifySolidityMultiPartRequest {
        bytecode: "0x00".into(),
        bytecode_type: BytecodeType::DeployedBytecode as i32,
        compiler_version: version.into(),
        source_files: BTreeMap::from([(name.into(), SOURCE.into())]),
        ..Default::default()
    }
}

fn standard_request(name: &str, version: &str) -> VerifySolidityStandardJsonRequest {
    VerifySolidityStandardJsonRequest {
        bytecode: "0x00".into(),
        bytecode_type: BytecodeType::DeployedBytecode as i32,
        compiler_version: version.into(),
        input: standard_input(name),
        ..Default::default()
    }
}

fn assert_invalid_name(error: RequestParseError) {
    assert!(matches!(error, RequestParseError::BadRequest(_)));
    assert!(error.to_string().contains("invalid source name"), "{error}");
}

// absolute-source-path is closed by the multipart, JSON and legacy CLI sentinel regressions.
#[test]
fn multipart_absolute_source_path_preserves_sentinel() {
    let outside = tempfile::tempdir().unwrap();
    let sentinel = outside.path().join("state.sol");
    std::fs::write(&sentinel, SENTINEL).unwrap();
    for version in VERSIONS {
        let result = solidity::multi_part::VerificationRequest::try_from(multipart_request(
            sentinel.to_str().unwrap(),
            version,
        ));
        assert_invalid_name(result.err().expect("absolute source must be rejected"));
        assert_eq!(std::fs::read(&sentinel).unwrap(), SENTINEL);
    }
}

#[test]
fn standard_json_absolute_source_path_preserves_sentinel() {
    let outside = tempfile::tempdir().unwrap();
    let sentinel = outside.path().join("state.sol");
    std::fs::write(&sentinel, SENTINEL).unwrap();
    for version in VERSIONS {
        let result = solidity::standard_json::VerificationRequest::try_from(standard_request(
            sentinel.to_str().unwrap(),
            version,
        ));
        assert_invalid_name(result.err().expect("absolute source must be rejected"));
        assert_eq!(std::fs::read(&sentinel).unwrap(), SENTINEL);
    }
}

#[test]
fn all_request_boundaries_reject_root_prefix_and_parent_components() {
    for name in [
        "/root.sol",
        "//server/share/root.sol",
        r"\root.sol",
        r"\\server\share\root.sol",
        r"C:\root.sol",
        "C:root.sol",
        "../parent.sol",
        "nested/../../parent.sol",
        r"nested\..\parent.sol",
        "",
        ".",
        "bad\0name.sol",
    ] {
        for version in VERSIONS {
            let multipart = solidity::multi_part::VerificationRequest::try_from(multipart_request(
                name, version,
            ));
            assert_invalid_name(
                multipart
                    .err()
                    .unwrap_or_else(|| panic!("accepted {name:?}")),
            );
            let standard = solidity::standard_json::VerificationRequest::try_from(
                standard_request(name, version),
            );
            assert_invalid_name(
                standard
                    .err()
                    .unwrap_or_else(|| panic!("accepted {name:?}")),
            );
            let multipart = solidity::multi_part::BatchVerificationRequest::try_from(
                BatchVerifySolidityMultiPartRequest {
                    compiler_version: version.into(),
                    sources: BTreeMap::from([(name.into(), SOURCE.into())]),
                    ..Default::default()
                },
            );
            assert_invalid_name(
                multipart
                    .err()
                    .unwrap_or_else(|| panic!("batch accepted {name:?}")),
            );
            let standard = solidity::standard_json::BatchVerificationRequest::try_from(
                BatchVerifySolidityStandardJsonRequest {
                    compiler_version: version.into(),
                    input: standard_input(name),
                    ..Default::default()
                },
            );
            assert_invalid_name(
                standard
                    .err()
                    .unwrap_or_else(|| panic!("batch accepted {name:?}")),
            );
        }
    }
}

#[test]
fn relative_nested_sources_keep_their_names_and_contents() {
    for name in [
        "Contract.sol",
        "contracts/nested/Contract.sol",
        "./Contract.sol",
    ] {
        for version in VERSIONS {
            let request = solidity::multi_part::VerificationRequest::try_from(multipart_request(
                name, version,
            ))
            .unwrap();
            assert_eq!(request.content.sources[Path::new(name)], SOURCE);
            let inputs: Vec<SolcInput> = request.content.into();
            assert_eq!(inputs.len(), 1);
            assert_eq!(
                inputs[0].0.sources.0[Path::new(name)].content.as_str(),
                SOURCE
            );
            let request = solidity::standard_json::VerificationRequest::try_from(standard_request(
                name, version,
            ))
            .unwrap();
            assert_eq!(
                request.content.0.sources.0[Path::new(name)]
                    .content
                    .as_str(),
                SOURCE
            );
        }
    }
}

// A loopback catalogue drives the real fetcher; the rejecting CLI path needs no compiler executable.
async fn compiler_pool() -> (EvmCompilersPool<SolcCompiler>, tempfile::TempDir) {
    let directory = tempfile::tempdir().unwrap();
    let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = listener.local_addr().unwrap();
    let server = tokio::spawn(async move {
        let (mut stream, _) = listener.accept().await.unwrap();
        let mut request = Vec::new();
        let mut buffer = [0; 1024];
        while !request.windows(4).any(|bytes| bytes == b"\r\n\r\n") {
            let count = stream.read(&mut buffer).await.unwrap();
            assert_ne!(count, 0);
            request.extend_from_slice(&buffer[..count]);
        }
        let body = r#"{"builds":[]}"#;
        let response = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()
        );
        stream.write_all(response.as_bytes()).await.unwrap();
    });
    let fetcher = ListFetcher::<DetailedVersion>::new(
        format!("http://{address}/list.json").parse().unwrap(),
        directory.path().to_path_buf(),
        None,
        None,
    )
    .await
    .unwrap();
    server.await.unwrap();
    (
        EvmCompilersPool::new(Arc::new(fetcher), Arc::new(Semaphore::new(1))),
        directory,
    )
}

#[tokio::test]
async fn legacy_cli_rejects_multipart_and_json_sources_before_writing() {
    let (pool, compiler_directory) = compiler_pool().await;
    let outside = tempfile::tempdir().unwrap();
    let sentinel = outside.path().join("state.sol");
    let new_file = outside.path().join("must-not-exist/nested.sol");
    std::fs::write(&sentinel, SENTINEL).unwrap();
    let compiler = compiler_directory.path().join("no-compiler-needed");
    assert!(!compiler.exists());
    for version in VERSIONS {
        let version: DetailedVersion = version.parse().unwrap();
        for name in [
            sentinel.to_str().unwrap(),
            new_file.to_str().unwrap(),
            "a/../parent.sol",
        ] {
            let multipart = solidity::multi_part::Content {
                sources: BTreeMap::from([(name.into(), SOURCE.into())]),
                evm_version: None,
                optimization_runs: None,
            };
            let inputs: Vec<SolcInput> = multipart.into();
            let json_input: SolcInput = serde_json::from_str(&standard_input(name)).unwrap();
            for mut input in [inputs.into_iter().next().unwrap(), json_input] {
                input.0.sources.0.insert(
                    "safe.sol".into(),
                    foundry_compilers::artifacts::Source::new(SOURCE),
                );
                let error = pool
                    .compile(&compiler, &version, &input)
                    .await
                    .err()
                    .expect("legacy CLI must reject the source name");
                assert!(error.to_string().contains("invalid source name"), "{error}");
                assert_eq!(std::fs::read(&sentinel).unwrap(), SENTINEL);
                assert!(!new_file.parent().unwrap().exists());
            }
        }
    }
}

#[cfg(unix)]
#[tokio::test]
async fn legacy_cli_rejects_absolute_symlink_destination() {
    let (pool, compiler_directory) = compiler_pool().await;
    let outside = tempfile::tempdir().unwrap();
    let sentinel = outside.path().join("state.sol");
    let link = outside.path().join("link.sol");
    std::fs::write(&sentinel, SENTINEL).unwrap();
    std::os::unix::fs::symlink(&sentinel, &link).unwrap();
    for version in VERSIONS {
        let input: SolcInput =
            serde_json::from_str(&standard_input(link.to_str().unwrap())).unwrap();
        let error = pool
            .compile(
                &compiler_directory.path().join("no-compiler-needed"),
                &version.parse().unwrap(),
                &input,
            )
            .await
            .err()
            .expect("symlink destination must be rejected");
        assert!(error.to_string().contains("invalid source name"), "{error}");
        assert_eq!(std::fs::read(&sentinel).unwrap(), SENTINEL);
        assert!(std::fs::symlink_metadata(&link)
            .unwrap()
            .file_type()
            .is_symlink());
    }
}
