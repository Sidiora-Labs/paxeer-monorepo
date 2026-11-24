use actix_web::{http::StatusCode, test, App};
use foundry_compilers::{
    artifacts::{CompilerOutput, EvmVersion, SolcInput, Source},
    solc::Solc,
};
use smart_contract_verifier::{DetailedVersion, Fetcher, ListFetcher};
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{
    self as proto, solidity_verifier_actix::route_solidity_verifier,
    solidity_verifier_client::SolidityVerifierClient,
    solidity_verifier_server::SolidityVerifierServer,
};
use smart_contract_verifier_server::{Settings, SolidityVerifierService};
use std::{sync::Arc, time::Duration};
use tokio::sync::{OnceCell, Semaphore};

const COMPILER_VERSION: &str = "v0.8.17+commit.8df45f5f";
const COMPILER_LIST: &str =
    "https://raw.githubusercontent.com/ethereum/solc-bin/gh-pages/linux-amd64/list.json";
const MULTIPART: &str = "/api/v2/verifier/solidity/sources:verify-multi-part";
const STANDARD_JSON: &str = "/api/v2/verifier/solidity/sources:verify-standard-json";
const BASE: &str = "pragma solidity ^0.8.17;\ncontract Base { function inherited() public pure returns (uint256) { return 7; } }";
const CHOSEN: &str = "pragma solidity ^0.8.17;\nimport \"Base.sol\";\ncontract Chosen is Base { function answer() public pure returns (uint256) { return 42; } }\ncontract Decoy { function wrong() public pure returns (uint256) { return 99; } }";

struct Fixture {
    service: Arc<SolidityVerifierService>,
    input: SolcInput,
    output: CompilerOutput,
    _compilers_dir: tempfile::TempDir,
}

async fn fixture() -> &'static Fixture {
    static FIXTURE: OnceCell<Fixture> = OnceCell::const_new();
    FIXTURE
        .get_or_init(|| async {
            let compilers_dir = tempfile::tempdir().unwrap();
            let fetcher = ListFetcher::<DetailedVersion>::new(
                COMPILER_LIST.parse().unwrap(),
                compilers_dir.path().into(),
                None,
                None,
            )
            .await
            .unwrap();
            let compiler = fetcher
                .fetch(&COMPILER_VERSION.parse().unwrap())
                .await
                .unwrap();
            let sources = [
                ("Base.sol".into(), Source::new(BASE)),
                ("Chosen.sol".into(), Source::new(CHOSEN)),
            ]
            .into_iter()
            .collect();
            let input = SolcInput::resolve_and_build(sources, Default::default())
                .into_iter()
                .next()
                .unwrap()
                .evm_version(EvmVersion::London);
            let compile_input = input.clone();
            let output = tokio::task::spawn_blocking(move || {
                Solc::new(compiler)
                    .unwrap()
                    .compile(&compile_input)
                    .unwrap()
            })
            .await
            .unwrap();
            assert!(!output.has_error(), "compiler errors: {:?}", output.errors);
            let settings: Settings = serde_json::from_value(serde_json::json!({
                "solidity": {
                    "compilers_dir": compilers_dir.path(),
                    "fetcher": { "list": { "list_url": COMPILER_LIST } }
                }
            }))
            .unwrap();
            let service = Arc::new(
                SolidityVerifierService::new(settings.solidity, Arc::new(Semaphore::new(1)))
                    .await
                    .unwrap(),
            );
            Fixture {
                service,
                input,
                output,
                _compilers_dir: compilers_dir,
            }
        })
        .await
}

fn bytecode(fixture: &Fixture, creation: bool) -> String {
    let contract = fixture
        .output
        .contracts_iter()
        .find(|(name, _)| *name == "Chosen")
        .unwrap()
        .1;
    let evm = contract.evm.as_ref().unwrap();
    let bytecode = if creation {
        evm.bytecode.as_ref().unwrap()
    } else {
        evm.deployed_bytecode
            .as_ref()
            .unwrap()
            .bytecode
            .as_ref()
            .unwrap()
    };
    bytecode.object.clone().into_bytes().unwrap().to_string()
}

fn multipart(
    fixture: &Fixture,
    creation: bool,
    actions: Vec<String>,
) -> proto::VerifySolidityMultiPartRequest {
    proto::VerifySolidityMultiPartRequest {
        bytecode: bytecode(fixture, creation),
        bytecode_type: if creation {
            proto::BytecodeType::CreationInput
        } else {
            proto::BytecodeType::DeployedBytecode
        } as i32,
        compiler_version: COMPILER_VERSION.into(),
        evm_version: Some("london".into()),
        source_files: [
            ("Base.sol".into(), BASE.into()),
            ("Chosen.sol".into(), CHOSEN.into()),
        ]
        .into_iter()
        .collect(),
        post_actions: actions,
        ..Default::default()
    }
}

fn standard_json(
    fixture: &Fixture,
    creation: bool,
    actions: Vec<String>,
) -> proto::VerifySolidityStandardJsonRequest {
    proto::VerifySolidityStandardJsonRequest {
        bytecode: bytecode(fixture, creation),
        bytecode_type: if creation {
            proto::BytecodeType::CreationInput
        } else {
            proto::BytecodeType::DeployedBytecode
        } as i32,
        compiler_version: COMPILER_VERSION.into(),
        input: serde_json::to_string(&fixture.input).unwrap(),
        post_actions: actions,
        ..Default::default()
    }
}

fn requested_action_sets() -> [Vec<String>; 3] {
    [
        vec![],
        vec!["lookup-methods".into()],
        vec!["lookup-methods".into(), "lookup-methods".into()],
    ]
}

fn assert_post_actions(response: &proto::VerifyResponse, requested: &[String]) {
    assert_eq!(
        response.status(),
        proto::verify_response::Status::Success,
        "{response:?}"
    );
    let returned: std::collections::BTreeSet<&str> = response
        .post_action_responses
        .as_ref()
        .expect("successful verification must include post-action responses")
        .lookup_methods
        .as_ref()
        .map(|_| "lookup-methods")
        .into_iter()
        .collect();
    let requested: std::collections::BTreeSet<&str> =
        requested.iter().map(String::as_str).collect();
    assert_eq!(
        returned, requested,
        "post-actions-dropped: echoed actions must equal exactly the requested set"
    );
}

fn assert_lookup(fixture: &Fixture, response: proto::VerifyResponse) {
    assert_eq!(
        response.status(),
        proto::verify_response::Status::Success,
        "{response:?}"
    );
    let source = response.source.unwrap();
    assert_eq!(source.file_name, "Chosen.sol");
    assert_eq!(source.contract_name, "Chosen");
    let methods = response
        .post_action_responses
        .unwrap()
        .lookup_methods
        .expect("post-actions-dropped: lookup-methods was requested")
        .methods;
    let expected = &fixture
        .output
        .contracts_iter()
        .find(|(name, _)| *name == "Chosen")
        .unwrap()
        .1
        .evm
        .as_ref()
        .unwrap()
        .method_identifiers;
    assert_eq!(methods.len(), 2);
    assert_eq!(
        methods.keys().collect::<std::collections::BTreeSet<_>>(),
        expected.values().collect::<std::collections::BTreeSet<_>>()
    );
    for (signature, selector) in expected {
        let (filename, text, declaration) = match signature.as_str() {
            "answer()" => (
                "Chosen.sol",
                CHOSEN,
                "function answer() public pure returns (uint256) { return 42; }",
            ),
            "inherited()" => (
                "Base.sol",
                BASE,
                "function inherited() public pure returns (uint256) { return 7; }",
            ),
            other => panic!("unexpected method: {other}"),
        };
        let method = methods.get(selector).unwrap();
        assert_eq!(method.file_name, filename);
        assert_eq!(method.file_offset as usize, text.find(declaration).unwrap());
        assert_eq!(method.length as usize, declaration.len());
    }
}

async fn grpc(
    service: Arc<SolidityVerifierService>,
) -> (
    SolidityVerifierClient<tonic::transport::Channel>,
    tokio::task::JoinHandle<()>,
) {
    let listener = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
    let address = listener.local_addr().unwrap();
    let incoming = tonic::codegen::tokio_stream::wrappers::TcpListenerStream::new(listener);
    let server = tokio::spawn(async move {
        tonic::transport::Server::builder()
            .add_service(SolidityVerifierServer::from_arc(service))
            .serve_with_incoming(incoming)
            .await
            .unwrap();
    });
    let client = tokio::time::timeout(Duration::from_secs(10), async {
        loop {
            match SolidityVerifierClient::connect(format!("http://{address}")).await {
                Ok(client) => break client,
                Err(_) => {
                    assert!(
                        !server.is_finished(),
                        "gRPC server stopped before becoming ready"
                    );
                    tokio::time::sleep(Duration::from_millis(10)).await;
                }
            }
        }
    })
    .await
    .unwrap();
    (client, server)
}

#[actix_web::test]
async fn post_actions_dropped_http_multipart() {
    let fixture = fixture().await;
    let app = test::init_service(
        App::new().configure(|config| route_solidity_verifier(config, fixture.service.clone())),
    )
    .await;
    for creation in [false, true] {
        for actions in requested_action_sets() {
            let request = multipart(fixture, creation, actions.clone());
            let response = test::TestRequest::post()
                .uri(MULTIPART)
                .set_json(&request)
                .send_request(&app)
                .await;
            assert_eq!(response.status(), StatusCode::OK);
            let response: proto::VerifyResponse = test::read_body_json(response).await;
            assert_post_actions(&response, &actions);
            if !actions.is_empty() {
                assert_lookup(fixture, response);
            }
        }
    }
}

#[actix_web::test]
async fn post_actions_dropped_http_standard_json() {
    let fixture = fixture().await;
    let app = test::init_service(
        App::new().configure(|config| route_solidity_verifier(config, fixture.service.clone())),
    )
    .await;
    for creation in [false, true] {
        for actions in requested_action_sets() {
            let request = standard_json(fixture, creation, actions.clone());
            let response = test::TestRequest::post()
                .uri(STANDARD_JSON)
                .set_json(&request)
                .send_request(&app)
                .await;
            assert_eq!(response.status(), StatusCode::OK);
            let response: proto::VerifyResponse = test::read_body_json(response).await;
            assert_post_actions(&response, &actions);
            if !actions.is_empty() {
                assert_lookup(fixture, response);
            }
        }
    }
}

#[tokio::test]
async fn post_actions_dropped_grpc_multipart() {
    let fixture = fixture().await;
    let (mut client, server) = grpc(fixture.service.clone()).await;
    for creation in [false, true] {
        for actions in requested_action_sets() {
            let response = client
                .verify_multi_part(multipart(fixture, creation, actions.clone()))
                .await
                .unwrap()
                .into_inner();
            assert_post_actions(&response, &actions);
            if !actions.is_empty() {
                assert_lookup(fixture, response);
            }
        }
    }
    server.abort();
}

#[tokio::test]
async fn post_actions_dropped_grpc_standard_json() {
    let fixture = fixture().await;
    let (mut client, server) = grpc(fixture.service.clone()).await;
    for creation in [false, true] {
        for actions in requested_action_sets() {
            let response = client
                .verify_standard_json(standard_json(fixture, creation, actions.clone()))
                .await
                .unwrap()
                .into_inner();
            assert_post_actions(&response, &actions);
            if !actions.is_empty() {
                assert_lookup(fixture, response);
            }
        }
    }
    server.abort();
}

#[actix_web::test]
async fn post_actions_reject_unsupported_http() {
    let fixture = fixture().await;
    let app = test::init_service(
        App::new().configure(|config| route_solidity_verifier(config, fixture.service.clone())),
    )
    .await;
    for action in ["unknown-action", "", "LOOKUP-METHODS"] {
        let request = multipart(fixture, false, vec!["lookup-methods".into(), action.into()]);
        let response = test::TestRequest::post()
            .uri(MULTIPART)
            .set_json(&request)
            .send_request(&app)
            .await;
        assert_eq!(response.status(), StatusCode::BAD_REQUEST);
        assert!(String::from_utf8(test::read_body(response).await.to_vec())
            .unwrap()
            .contains("unsupported post-action"));
        let request = standard_json(fixture, false, vec![action.into()]);
        let response = test::TestRequest::post()
            .uri(STANDARD_JSON)
            .set_json(&request)
            .send_request(&app)
            .await;
        assert_eq!(response.status(), StatusCode::BAD_REQUEST);
        assert!(String::from_utf8(test::read_body(response).await.to_vec())
            .unwrap()
            .contains("unsupported post-action"));
    }
}

#[tokio::test]
async fn post_actions_reject_unsupported_grpc() {
    let fixture = fixture().await;
    let (mut client, server) = grpc(fixture.service.clone()).await;
    for action in ["unknown-action", "", "LOOKUP-METHODS"] {
        let error = client
            .verify_multi_part(multipart(
                fixture,
                false,
                vec!["lookup-methods".into(), action.into()],
            ))
            .await
            .unwrap_err();
        assert_eq!(error.code(), tonic::Code::InvalidArgument);
        assert!(error.message().contains("unsupported post-action"));
        let error = client
            .verify_standard_json(standard_json(fixture, false, vec![action.into()]))
            .await
            .unwrap_err();
        assert_eq!(error.code(), tonic::Code::InvalidArgument);
        assert!(error.message().contains("unsupported post-action"));
    }
    server.abort();
}

#[tokio::test]
async fn post_actions_omitted_and_failed_verification() {
    let fixture = fixture().await;
    let (mut client, server) = grpc(fixture.service.clone()).await;
    let response = client
        .verify_multi_part(multipart(fixture, false, vec![]))
        .await
        .unwrap()
        .into_inner();
    assert_eq!(response.status(), proto::verify_response::Status::Success);
    assert!(response
        .post_action_responses
        .unwrap()
        .lookup_methods
        .is_none());
    let response = client
        .verify_standard_json(standard_json(fixture, false, vec![]))
        .await
        .unwrap()
        .into_inner();
    assert_eq!(response.status(), proto::verify_response::Status::Success);
    assert!(response
        .post_action_responses
        .unwrap()
        .lookup_methods
        .is_none());
    let mut request = standard_json(fixture, false, vec!["lookup-methods".into()]);
    request.bytecode = "0x00".into();
    let response = client
        .verify_standard_json(request)
        .await
        .unwrap()
        .into_inner();
    assert_eq!(response.status(), proto::verify_response::Status::Failure);
    assert!(response.post_action_responses.is_none());
    server.abort();
}

#[tokio::test]
async fn post_actions_conversion_preserves_supported_action() {
    let fixture = fixture().await;
    for creation in [false, true] {
        for actions in requested_action_sets() {
            let (_, multipart_actions) = smart_contract_verifier::solidity::multi_part::VerificationRequest::try_from_with_post_actions(
                multipart(fixture, creation, actions.clone()),
            ).unwrap();
            assert_eq!(multipart_actions, actions);
            let (_, standard_json_actions) = smart_contract_verifier::solidity::standard_json::VerificationRequest::try_from_with_post_actions(
                standard_json(fixture, creation, actions.clone()),
            ).unwrap();
            assert_eq!(standard_json_actions, actions);
        }
    }
}
