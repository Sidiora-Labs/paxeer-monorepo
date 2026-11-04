use smart_contract_verifier_proto::{
    blockscout::smart_contract_verifier::v2 as proto,
    http_client::{
        mock, solidity_verifier_client, sourcify_verifier_client, vyper_verifier_client, Client,
        Config,
    },
};
use tonic::Response;

macro_rules! set_expectation {
    ($mock_service:expr, $expect:ident, $request:ty, $response:ty) => {
        $mock_service
            .$expect()
            .withf(|request| request.get_ref() == &<$request>::default())
            .returning(move |_| Ok(Response::new(<$response>::default())));
        // .once();
    };
}

async fn build_client(server: mock::SmartContractVerifierServer) -> Client {
    let server_addr = server.start().await;
    let config = Config::new(format!("http://{server_addr}"));
    Client::new(config).await
}

#[tokio::test]
async fn solidity_service() {
    let mock_service = {
        let mut mock_service = mock::MockSolidityVerifierService::default();
        set_expectation!(
            mock_service,
            expect_verify_multi_part,
            proto::VerifySolidityMultiPartRequest,
            proto::VerifyResponse
        );
        set_expectation!(
            mock_service,
            expect_verify_standard_json,
            proto::VerifySolidityStandardJsonRequest,
            proto::VerifyResponse
        );
        set_expectation!(
            mock_service,
            expect_batch_verify_multi_part,
            proto::BatchVerifySolidityMultiPartRequest,
            proto::BatchVerifyResponse
        );
        set_expectation!(
            mock_service,
            expect_batch_verify_standard_json,
            proto::BatchVerifySolidityStandardJsonRequest,
            proto::BatchVerifyResponse
        );
        set_expectation!(
            mock_service,
            expect_list_compiler_versions,
            proto::ListCompilerVersionsRequest,
            proto::ListCompilerVersionsResponse
        );
        set_expectation!(
            mock_service,
            expect_lookup_methods,
            proto::LookupMethodsRequest,
            proto::LookupMethodsResponse
        );

        mock_service
    };

    let client =
        build_client(mock::SmartContractVerifierServer::new().solidity_service(mock_service)).await;

    assert!(solidity_verifier_client::verify_multi_part(
        &client,
        proto::VerifySolidityMultiPartRequest::default()
    )
    .await
    .is_ok());
    assert!(solidity_verifier_client::verify_standard_json(
        &client,
        proto::VerifySolidityStandardJsonRequest::default()
    )
    .await
    .is_ok());
    assert!(solidity_verifier_client::batch_verify_multi_part(
        &client,
        proto::BatchVerifySolidityMultiPartRequest::default()
    )
    .await
    .is_ok());
    assert!(solidity_verifier_client::batch_verify_standard_json(
        &client,
        proto::BatchVerifySolidityStandardJsonRequest::default()
    )
    .await
    .is_ok());
    assert!(solidity_verifier_client::list_compiler_versions(
        &client,
        proto::ListCompilerVersionsRequest::default()
    )
    .await
    .is_ok());
    assert!(solidity_verifier_client::lookup_methods(
        &client,
        proto::LookupMethodsRequest::default()
    )
    .await
    .is_ok());
}

#[tokio::test]
async fn vyper_service() {
    let mock_service = {
        let mut mock_service = mock::MockVyperVerifierService::default();
        set_expectation!(
            mock_service,
            expect_verify_multi_part,
            proto::VerifyVyperMultiPartRequest,
            proto::VerifyResponse
        );
        set_expectation!(
            mock_service,
            expect_verify_standard_json,
            proto::VerifyVyperStandardJsonRequest,
            proto::VerifyResponse
        );
        set_expectation!(
            mock_service,
            expect_list_compiler_versions,
            proto::ListCompilerVersionsRequest,
            proto::ListCompilerVersionsResponse
        );

        mock_service
    };

    let client =
        build_client(mock::SmartContractVerifierServer::new().vyper_service(mock_service)).await;

    assert!(vyper_verifier_client::verify_multi_part(
        &client,
        proto::VerifyVyperMultiPartRequest::default()
    )
    .await
    .is_ok());
    assert!(vyper_verifier_client::verify_standard_json(
        &client,
        proto::VerifyVyperStandardJsonRequest::default()
    )
    .await
    .is_ok());
    assert!(vyper_verifier_client::list_compiler_versions(
        &client,
        proto::ListCompilerVersionsRequest::default()
    )
    .await
    .is_ok());
}

#[tokio::test]
async fn sourcify_service() {
    let mock_service = {
        let mut mock_service = mock::MockSourcifyVerifierService::default();
        set_expectation!(
            mock_service,
            expect_verify,
            proto::VerifySourcifyRequest,
            proto::VerifyResponse
        );
        set_expectation!(
            mock_service,
            expect_verify_from_etherscan,
            proto::VerifyFromEtherscanSourcifyRequest,
            proto::VerifyResponse
        );

        mock_service
    };

    let client =
        build_client(mock::SmartContractVerifierServer::new().sourcify_service(mock_service)).await;

    assert!(
        sourcify_verifier_client::verify(&client, proto::VerifySourcifyRequest::default())
            .await
            .is_ok()
    );
    assert!(sourcify_verifier_client::verify_from_etherscan(
        &client,
        proto::VerifyFromEtherscanSourcifyRequest::default()
    )
    .await
    .is_ok());
}

#[path = "../../../libs/blockscout-service-launcher/src/launcher/router.rs"]
mod launcher_router;
#[path = "../../smart-contract-verifier-server/src/services/health.rs"]
mod production_health;

mod populated {
    use super::{launcher_router, production_health, proto};
    use smart_contract_verifier_proto::http_client::{
        health_client, solidity_verifier_client as solidity, sourcify_verifier_client as sourcify,
        vyper_verifier_client as vyper, Client, Config, Error,
    };
    use std::{collections::BTreeMap, net::TcpListener, sync::Arc, time::Duration};

    const VERSION: &str = "v0.8.17+commit.8df45f5f";
    const SETTINGS: &str = r#"{"optimizer":{"enabled":true,"runs":200},"evmVersion":"london"}"#;
    const ADDRESS: &str = "0x0123456789012345678901234567890123456789";

    fn sources() -> BTreeMap<String, String> {
        BTreeMap::from([
            (
                "src/Storage.sol".into(),
                include_str!(
                    "../../smart-contract-verifier-server/tests/contracts/storage/source.sol"
                )
                .into(),
            ),
            (
                "interfaces/IStorage.sol".into(),
                "// Unicode: λ\ninterface IStorage { function store(uint256 value) external; }\n"
                    .into(),
            ),
        ])
    }

    fn metadata() -> proto::VerificationMetadata {
        proto::VerificationMetadata {
            chain_id: Some("125".into()),
            contract_address: Some(ADDRESS.into()),
        }
    }

    fn solidity_multi_part() -> proto::VerifySolidityMultiPartRequest {
        proto::VerifySolidityMultiPartRequest {
            bytecode: "0x6080604052600080fd".into(),
            bytecode_type: proto::BytecodeType::CreationInput as i32,
            compiler_version: VERSION.into(),
            evm_version: Some("london".into()),
            optimization_runs: Some(200),
            source_files: sources(),
            metadata: Some(metadata()),
            post_actions: vec!["lookup-methods".into()],
        }
    }

    fn solidity_standard_json() -> proto::VerifySolidityStandardJsonRequest {
        proto::VerifySolidityStandardJsonRequest {
            bytecode: "0x60006000f3".into(),
            bytecode_type: proto::BytecodeType::DeployedBytecode as i32,
            compiler_version: VERSION.into(),
            input: include_str!("../../smart-contract-verifier-server/tests/contracts/match_type_full/standard_input.json").into(),
            metadata: Some(metadata()),
            post_actions: vec!["lookup-methods".into()],
        }
    }

    fn contracts() -> Vec<proto::Contract> {
        vec![
            proto::Contract {
                creation_code: Some("0x6080604052600080fd".into()),
                runtime_code: Some("0x60006000f3".into()),
                metadata: Some(metadata()),
            },
            proto::Contract {
                creation_code: None,
                runtime_code: Some("0x60016000f3".into()),
                metadata: None,
            },
        ]
    }

    fn batch_multi_part() -> proto::BatchVerifySolidityMultiPartRequest {
        proto::BatchVerifySolidityMultiPartRequest {
            contracts: contracts(),
            compiler_version: VERSION.into(),
            sources: sources(),
            evm_version: Some("london".into()),
            optimization_runs: Some(0),
        }
    }

    fn batch_standard_json() -> proto::BatchVerifySolidityStandardJsonRequest {
        proto::BatchVerifySolidityStandardJsonRequest {
            contracts: contracts(),
            compiler_version: VERSION.into(),
            input: solidity_standard_json().input,
        }
    }

    fn vyper_multi_part() -> proto::VerifyVyperMultiPartRequest {
        proto::VerifyVyperMultiPartRequest {
            bytecode: "0x60006000f3".into(),
            bytecode_type: proto::BytecodeType::DeployedBytecode as i32,
            compiler_version: "v0.3.10+commit.9136169".into(),
            evm_version: Some("shanghai".into()),
            source_files: BTreeMap::from([(
                "storage.vy".into(),
                "stored: public(uint256)\n".into(),
            )]),
            interfaces: BTreeMap::from([(
                "IStorage".into(),
                "def store(value: uint256): nonpayable\n".into(),
            )]),
            metadata: Some(metadata()),
        }
    }

    fn vyper_standard_json() -> proto::VerifyVyperStandardJsonRequest {
        proto::VerifyVyperStandardJsonRequest {
            bytecode: "0x6080604052".into(),
            bytecode_type: proto::BytecodeType::CreationInput as i32,
            compiler_version: vyper_multi_part().compiler_version,
            input: r#"{"language":"Vyper","sources":{"storage.vy":{"content":"stored: public(uint256)\n"}},"settings":{"evmVersion":"shanghai"}}"#.into(),
            metadata: Some(proto::VerificationMetadata { chain_id: Some("125".into()), contract_address: None }),
        }
    }

    fn sourcify_request() -> proto::VerifySourcifyRequest {
        proto::VerifySourcifyRequest {
            address: ADDRESS.into(),
            chain: "125".into(),
            files: sources(),
            chosen_contract: Some(0),
        }
    }

    fn etherscan_request() -> proto::VerifyFromEtherscanSourcifyRequest {
        proto::VerifyFromEtherscanSourcifyRequest {
            address: ADDRESS.into(),
            chain: "125".into(),
        }
    }

    fn methods_request() -> proto::LookupMethodsRequest {
        proto::LookupMethodsRequest {
            bytecode: "0x63abcdef0114600057".into(),
            abi: r#"[{"type":"function","name":"store","inputs":[{"name":"value","type":"uint256"}]}]"#.into(),
            source_map: "0:12:0;13:20:7".into(),
            file_ids: BTreeMap::from([(0, "src/Storage.sol".into()), (7, "interfaces/IStorage.sol".into())]),
        }
    }

    fn methods_response() -> proto::LookupMethodsResponse {
        proto::LookupMethodsResponse {
            methods: BTreeMap::from([
                (
                    "6057361d".into(),
                    proto::lookup_methods_response::Method {
                        file_name: "src/Storage.sol".into(),
                        file_offset: 73,
                        length: 42,
                    },
                ),
                (
                    "2e64cec1".into(),
                    proto::lookup_methods_response::Method {
                        file_name: "interfaces/IStorage.sol".into(),
                        file_offset: 17,
                        length: 29,
                    },
                ),
            ]),
        }
    }

    fn versions() -> proto::ListCompilerVersionsResponse {
        proto::ListCompilerVersionsResponse {
            compiler_versions: vec![VERSION.into(), "v0.8.16+commit.07a7930e".into()],
        }
    }

    fn verify_response() -> proto::VerifyResponse {
        proto::VerifyResponse {
            message: "Verification succeeded: λ".into(),
            status: proto::verify_response::Status::Success as i32,
            source: Some(proto::Source {
                file_name: "src/Storage.sol".into(),
                contract_name: "Storage".into(),
                compiler_version: VERSION.into(),
                compiler_settings: SETTINGS.into(),
                source_type: proto::source::SourceType::Solidity as i32,
                source_files: sources(),
                abi: Some(methods_request().abi),
                constructor_arguments: Some(String::new()),
                match_type: proto::source::MatchType::Full as i32,
                compilation_artifacts: Some(r#"{"abi":[],"userdoc":{"notice":"λ"}}"#.into()),
                creation_input_artifacts: Some(r#"{"sourceMap":"0:12:0"}"#.into()),
                deployed_bytecode_artifacts: Some(r#"{"immutableReferences":{}}"#.into()),
                is_blueprint: true,
                libraries: BTreeMap::from([("src/Library.sol:Library".into(), ADDRESS.into())]),
            }),
            extra_data: Some(proto::verify_response::ExtraData {
                local_creation_input_parts: vec![
                    proto::verify_response::extra_data::BytecodePart {
                        r#type: "main".into(),
                        data: "6080604052".into(),
                    },
                ],
                local_deployed_bytecode_parts: vec![
                    proto::verify_response::extra_data::BytecodePart {
                        r#type: "main".into(),
                        data: "60006000f3".into(),
                    },
                    proto::verify_response::extra_data::BytecodePart {
                        r#type: "meta".into(),
                        data: "a164736f6c6343000811000a".into(),
                    },
                ],
            }),
            post_action_responses: Some(proto::verify_response::PostActionResponses {
                lookup_methods: Some(methods_response()),
            }),
        }
    }

    fn vyper_response() -> proto::VerifyResponse {
        let mut response = verify_response();
        let source = response.source.as_mut().unwrap();
        source.file_name = "storage.vy".into();
        source.compiler_version = vyper_multi_part().compiler_version;
        source.source_type = proto::source::SourceType::Vyper as i32;
        source.source_files = vyper_multi_part().source_files;
        source.match_type = proto::source::MatchType::Partial as i32;
        source.constructor_arguments = None;
        source.creation_input_artifacts = None;
        source.is_blueprint = false;
        response.post_action_responses = None;
        response
    }

    fn sourcify_response() -> proto::VerifyResponse {
        let mut response = verify_response();
        response.extra_data = None;
        response.post_action_responses = None;
        response
    }

    fn batch_results() -> proto::BatchVerifyResponse {
        use proto::{batch_verify_response as batch, contract_verification_result as item};
        proto::BatchVerifyResponse {
            verification_result: Some(batch::VerificationResult::ContractVerificationResults(batch::ContractVerificationResults {
                items: vec![
                    proto::ContractVerificationResult { verification_result: Some(item::VerificationResult::Success(proto::ContractVerificationSuccess {
                        creation_code: contracts()[0].creation_code.clone().unwrap(),
                        runtime_code: contracts()[0].runtime_code.clone().unwrap(),
                        compiler: proto::contract_verification_success::compiler::Compiler::Solc as i32,
                        compiler_version: VERSION.into(),
                        language: proto::contract_verification_success::language::Language::Solidity as i32,
                        file_name: "src/Storage.sol".into(),
                        contract_name: "Storage".into(),
                        sources: sources(),
                        compiler_settings: SETTINGS.into(),
                        compilation_artifacts: r#"{"abi":[]}"#.into(),
                        creation_code_artifacts: r#"{"sourceMap":"0:12:0"}"#.into(),
                        runtime_code_artifacts: r#"{"sourceMap":"13:20:7"}"#.into(),
                        creation_match_details: Some(proto::contract_verification_success::MatchDetails {
                            match_type: proto::contract_verification_success::MatchType::Full as i32,
                            values: r#"{"constructorArguments":"0x0001"}"#.into(),
                            transformations: r#"[{"type":"insert","reason":"constructorArguments"}]"#.into(),
                        }),
                        runtime_match_details: None,
                    })) },
                    proto::ContractVerificationResult { verification_result: Some(item::VerificationResult::Failure(proto::ContractVerificationFailure {})) },
                ],
            })),
        }
    }

    fn compilation_failure() -> proto::BatchVerifyResponse {
        proto::BatchVerifyResponse {
            verification_result: Some(
                proto::batch_verify_response::VerificationResult::CompilationFailure(
                    proto::CompilationFailure {
                        message: "ParserError: Expected identifier in src/Storage.sol:1:1".into(),
                    },
                ),
            ),
        }
    }

    // This peer supplies committed typed exchanges to the production generated HTTP adapter.
    // Compilation is outside this transport test; no transport or codec is replaced.
    struct Exchanges;

    macro_rules! exchanges {
        ($service:path; $($method:ident($request:ty) => $expected:expr, $response:ty => $reply:expr;)+) => {
            #[async_trait::async_trait]
            impl $service for Exchanges {
                $(async fn $method(&self, request: tonic::Request<$request>) -> Result<tonic::Response<$response>, tonic::Status> {
                    assert_eq!(request.into_inner(), $expected, stringify!($method));
                    Ok(tonic::Response::new($reply))
                })+
            }
        };
    }

    exchanges! { proto::solidity_verifier_server::SolidityVerifier;
        verify_multi_part(proto::VerifySolidityMultiPartRequest) => solidity_multi_part(), proto::VerifyResponse => verify_response();
        verify_standard_json(proto::VerifySolidityStandardJsonRequest) => solidity_standard_json(), proto::VerifyResponse => verify_response();
        batch_verify_multi_part(proto::BatchVerifySolidityMultiPartRequest) => batch_multi_part(), proto::BatchVerifyResponse => batch_results();
        batch_verify_standard_json(proto::BatchVerifySolidityStandardJsonRequest) => batch_standard_json(), proto::BatchVerifyResponse => compilation_failure();
        list_compiler_versions(proto::ListCompilerVersionsRequest) => proto::ListCompilerVersionsRequest {}, proto::ListCompilerVersionsResponse => versions();
        lookup_methods(proto::LookupMethodsRequest) => methods_request(), proto::LookupMethodsResponse => methods_response();
    }
    exchanges! { proto::vyper_verifier_server::VyperVerifier;
        verify_multi_part(proto::VerifyVyperMultiPartRequest) => vyper_multi_part(), proto::VerifyResponse => vyper_response();
        verify_standard_json(proto::VerifyVyperStandardJsonRequest) => vyper_standard_json(), proto::VerifyResponse => vyper_response();
        list_compiler_versions(proto::ListCompilerVersionsRequest) => proto::ListCompilerVersionsRequest {}, proto::ListCompilerVersionsResponse => versions();
    }
    exchanges! { proto::sourcify_verifier_server::SourcifyVerifier;
        verify(proto::VerifySourcifyRequest) => sourcify_request(), proto::VerifyResponse => sourcify_response();
        verify_from_etherscan(proto::VerifyFromEtherscanSourcifyRequest) => etherscan_request(), proto::VerifyResponse => sourcify_response();
    }

    struct Router;

    impl launcher_router::HttpRouter for Router {
        fn register_routes(&self, config: &mut actix_web::web::ServiceConfig) {
            let exchanges = Arc::new(Exchanges);
            proto::solidity_verifier_actix::route_solidity_verifier(config, exchanges.clone());
            proto::vyper_verifier_actix::route_vyper_verifier(config, exchanges.clone());
            proto::sourcify_verifier_actix::route_sourcify_verifier(config, exchanges);
            proto::health_actix::route_health(
                config,
                Arc::new(production_health::HealthService::default()),
            );
        }
    }

    async fn populated_round_trip(mount: Option<&'static str>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let server = actix_web::HttpServer::new(move || {
            actix_web::App::new().configure(launcher_router::configure_router(
                &Router,
                mount.map(str::to_owned),
            ))
        })
        .workers(1)
        .disable_signals()
        .listen(listener)
        .unwrap()
        .run();
        let handle = server.handle();
        let server = actix_web::rt::spawn(server);

        for slash in ["", "/"] {
            let client = Client::new(
                Config::new(format!(
                    "http://{address}{}{slash}?discard=old#fragment",
                    mount.unwrap_or_default()
                ))
                .probe_url(true),
            )
            .await;
            assert_eq!(
                solidity::verify_multi_part(&client, solidity_multi_part())
                    .await
                    .unwrap(),
                verify_response()
            );
            assert_eq!(
                solidity::verify_standard_json(&client, solidity_standard_json())
                    .await
                    .unwrap(),
                verify_response()
            );
            assert_eq!(
                solidity::batch_verify_multi_part(&client, batch_multi_part())
                    .await
                    .unwrap(),
                batch_results()
            );
            assert_eq!(
                solidity::batch_verify_standard_json(&client, batch_standard_json())
                    .await
                    .unwrap(),
                compilation_failure()
            );
            assert_eq!(
                solidity::list_compiler_versions(&client, proto::ListCompilerVersionsRequest {})
                    .await
                    .unwrap(),
                versions()
            );
            assert_eq!(
                solidity::lookup_methods(&client, methods_request())
                    .await
                    .unwrap(),
                methods_response()
            );
            assert_eq!(
                vyper::verify_multi_part(&client, vyper_multi_part())
                    .await
                    .unwrap(),
                vyper_response()
            );
            assert_eq!(
                vyper::verify_standard_json(&client, vyper_standard_json())
                    .await
                    .unwrap(),
                vyper_response()
            );
            assert_eq!(
                vyper::list_compiler_versions(&client, proto::ListCompilerVersionsRequest {})
                    .await
                    .unwrap(),
                versions()
            );
            assert_eq!(
                sourcify::verify(&client, sourcify_request()).await.unwrap(),
                sourcify_response()
            );
            assert_eq!(
                sourcify::verify_from_etherscan(&client, etherscan_request())
                    .await
                    .unwrap(),
                sourcify_response()
            );
        }

        handle.stop(true).await;
        tokio::time::timeout(Duration::from_secs(5), server)
            .await
            .unwrap()
            .unwrap()
            .unwrap();
    }

    async fn production_health_and_errors(mount: Option<&'static str>) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let server = actix_web::HttpServer::new(move || {
            actix_web::App::new().configure(launcher_router::configure_router(
                &Router,
                mount.map(str::to_owned),
            ))
        })
        .workers(1)
        .disable_signals()
        .listen(listener)
        .unwrap()
        .run();
        let handle = server.handle();
        let server = actix_web::rt::spawn(server);
        let base = format!("http://{address}{}", mount.unwrap_or_default());
        let client = Client::new(Config::new(base.clone()).probe_url(true)).await;
        let serving = proto::HealthCheckResponse {
            status: proto::health_check_response::ServingStatus::Serving as i32,
        };
        for service in ["", <proto::health_server::HealthServer<production_health::HealthService> as tonic::server::NamedService>::NAME] {
            assert_eq!(health_client::health(&client, proto::HealthCheckRequest { service: service.into() }).await.unwrap(), serving);
        }
        for service in [
            "missing.service",
            "blockscout.smartContractVerifier.v2.health",
            "Health.Check",
        ] {
            let error = health_client::health(
                &client,
                proto::HealthCheckRequest {
                    service: service.into(),
                },
            )
            .await
            .unwrap_err();
            match error {
                Error::StatusCode(response) => {
                    assert_eq!(response.status(), reqwest::StatusCode::NOT_FOUND);
                    let body = response.text().await.unwrap();
                    assert!(body.contains("unknown service"), "{body}");
                    assert!(!body.contains("SERVING"), "{body}");
                }
                error => panic!("expected a typed HTTP status error, got {error}"),
            }
        }

        let raw = reqwest::Client::builder()
            .no_proxy()
            .timeout(Duration::from_secs(5))
            .build()
            .unwrap();
        for body in ["{", r#"{"bytecode":"0x00","bytecodeType":"INVALID"}"#] {
            let response = raw
                .post(format!(
                    "{base}/api/v2/verifier/solidity/sources:verify-multi-part"
                ))
                .header("content-type", "application/json")
                .body(body)
                .send()
                .await
                .unwrap();
            assert_eq!(response.status(), reqwest::StatusCode::BAD_REQUEST);
            assert!(!response.text().await.unwrap().is_empty());
        }
        if mount.is_some() {
            let wrong_client = Client::new(Config::new(format!("http://{address}"))).await;
            match solidity::verify_multi_part(&wrong_client, solidity_multi_part())
                .await
                .unwrap_err()
            {
                Error::StatusCode(response) => {
                    assert_eq!(response.status(), reqwest::StatusCode::NOT_FOUND)
                }
                error => panic!("expected missing mount to return HTTP 404, got {error}"),
            }
        }
        assert_eq!(
            health_client::health(&client, proto::HealthCheckRequest::default())
                .await
                .unwrap(),
            serving
        );
        assert_eq!(
            solidity::verify_multi_part(&client, solidity_multi_part())
                .await
                .unwrap(),
            verify_response()
        );
        handle.stop(true).await;
        tokio::time::timeout(Duration::from_secs(5), server)
            .await
            .unwrap()
            .unwrap()
            .unwrap();
    }

    #[actix_web::test]
    async fn populated_root_round_trip() {
        populated_round_trip(None).await;
    }

    #[actix_web::test]
    async fn populated_non_root_round_trip() {
        populated_round_trip(Some("/nested/verifier")).await;
    }

    #[actix_web::test]
    async fn production_root_health_and_errors() {
        production_health_and_errors(None).await;
    }

    #[actix_web::test]
    async fn production_non_root_health_and_errors() {
        production_health_and_errors(Some("/nested/verifier")).await;
    }
}
