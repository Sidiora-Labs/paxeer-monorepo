#[path = "sourcify_from_etherscan.rs"]
mod etherscan_import_success_untested;
#[path = "sourcify.rs"]
mod sourcify_route_tests_commented;
#[path = "support/repair_22_33_596/mod.rs"]
mod support;

use actix_web::{http::StatusCode, test, App};
use serde_json::{json, Value};
use smart_contract_verifier::{DetailedVersion, SolcValidator};
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{
    solidity_verifier_actix::route_solidity_verifier, ListCompilerVersionsResponse,
};

#[actix_web::test]
async fn nightly_catalogue_reaches_service_and_rejects_wrong_date() {
    let catalogue: Value = serde_json::from_str(support::NIGHTLY_LIST).unwrap();
    assert_eq!(catalogue["builds"].as_array().unwrap().len(), 1);
    assert_eq!(
        catalogue["builds"][0]["sha256"],
        "db60947a952c9e7db5cea948189e17fd524a4c25ea7c2b252b92a2e396c3142c"
    );
    assert_eq!(catalogue["builds"][0]["path"], "https://github.com/blockscout/solc-bin/releases/download/v0.8.14-nightly.2022.4.13%2Bcommit.25923c1f/solc");
    let version: DetailedVersion = support::NIGHTLY.parse().unwrap();
    SolcValidator::default()
        .validate_version(
            &version,
            &"0.8.14-nightly.2022.4.13+commit.25923c1f".parse().unwrap(),
        )
        .unwrap();
    let (service, _directory) = support::nightly_service().await;
    let app = test::init_service(
        App::new().configure(|config| route_solidity_verifier(config, service.clone())),
    )
    .await;
    let response = test::TestRequest::get()
        .uri("/api/v2/verifier/solidity/versions")
        .send_request(&app)
        .await;
    assert_eq!(response.status(), StatusCode::OK);
    let versions: ListCompilerVersionsResponse = test::read_body_json(response).await;
    assert_eq!(
        versions.compiler_versions,
        vec![support::NIGHTLY.to_string()]
    );
    // Both prerelease date and commit are part of catalogue identity. An unknown
    // nightly must be rejected before invoking a compiler, even for valid source.
    for version in [
        "v0.8.14-nightly.2022.4.14+commit.25923c1f",
        "v0.8.14-nightly.2022.4.13+commit.00000000",
    ] {
        let response = test::TestRequest::post()
            .uri("/api/v2/verifier/solidity/sources:verify-multi-part")
            .set_json(json!({
                "compilerVersion":version,"bytecodeType":"CREATION_INPUT",
                "bytecode":include_str!("contracts/issue_5430_5434/creation_tx_input").trim(),
                "sourceFiles":{"tests/contracts/issue_5430_5434/source.sol":include_str!("contracts/issue_5430_5434/source.sol")}
            })).send_request(&app).await;
        assert_eq!(response.status(), StatusCode::BAD_REQUEST);
        let body = test::read_body(response).await;
        assert!(std::str::from_utf8(&body)
            .unwrap()
            .contains(&format!("Compiler version not found: {version}")));
    }
}
