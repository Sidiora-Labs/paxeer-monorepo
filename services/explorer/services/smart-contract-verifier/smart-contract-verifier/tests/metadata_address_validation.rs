// Repair 22.33.584: metadata-address-parse is proven by every test below.
// Baseline verification was not run: the task's ordered gates prohibit testing before implementation.
// Wiring requests: none; all metadata conversions are inside the declared source scope.

use alloy_core::primitives::Address;
use serde::{de::DeserializeOwned, Serialize};
use smart_contract_verifier::{solidity, vyper, OnChainContract, RequestParseError};
use smart_contract_verifier_proto::blockscout::smart_contract_verifier::v2::{
    BatchVerifySolidityMultiPartRequest, BatchVerifySolidityStandardJsonRequest, BytecodeType,
    Contract, VerificationMetadata, VerifySolidityMultiPartRequest,
    VerifySolidityStandardJsonRequest, VerifyVyperMultiPartRequest, VerifyVyperStandardJsonRequest,
};
use std::collections::BTreeMap;

const CHAIN_ID: &str = "125";
const ADDRESS: &str = "0x0123456789abcdef0123456789abcdef01234567";
const SOLIDITY_VERSION: &str = "v0.8.24+commit.e11b9ed9";
const VYPER_VERSION: &str = "v0.3.10+commit.9136169";

#[derive(Clone, Copy, Debug)]
enum RequestPath {
    Contract,
    SolidityMultiPart,
    SolidityStandardJson,
    SolidityBatchMultiPart,
    SolidityBatchStandardJson,
    VyperMultiPart,
    VyperStandardJson,
}

const REQUEST_PATHS: [RequestPath; 7] = [
    RequestPath::Contract,
    RequestPath::SolidityMultiPart,
    RequestPath::SolidityStandardJson,
    RequestPath::SolidityBatchMultiPart,
    RequestPath::SolidityBatchStandardJson,
    RequestPath::VyperMultiPart,
    RequestPath::VyperStandardJson,
];

// Use the generated request types and the production HTTP JSON codec before conversion.
fn transport<T: Serialize + DeserializeOwned>(request: T) -> T {
    let encoded = serde_json::to_vec(&request).unwrap();
    serde_json::from_slice(&encoded).unwrap()
}

fn standard_input(language: &str, name: &str, source: &str) -> String {
    serde_json::json!({
        "language": language,
        "sources": {name: {"content": source}},
        "settings": {}
    })
    .to_string()
}

fn contract(metadata: Option<VerificationMetadata>) -> Contract {
    Contract {
        runtime_code: Some("0x6000".into()),
        creation_code: Some("0x60016000".into()),
        metadata,
    }
}

impl RequestPath {
    fn convert(
        self,
        metadata: Option<VerificationMetadata>,
    ) -> Result<OnChainContract, RequestParseError> {
        let solidity_sources = BTreeMap::from([("Contract.sol".into(), "contract C {}".into())]);
        let solidity_input = standard_input("Solidity", "Contract.sol", "contract C {}");
        let vyper_sources =
            BTreeMap::from([("Contract.vy".into(), "value: public(uint256)".into())]);
        let vyper_input = standard_input("Vyper", "Contract.vy", "value: public(uint256)");
        match self {
            Self::Contract => OnChainContract::try_from(transport(contract(metadata))),
            Self::SolidityMultiPart => {
                let request = transport(VerifySolidityMultiPartRequest {
                    bytecode: "0x6000".into(),
                    bytecode_type: BytecodeType::DeployedBytecode as i32,
                    compiler_version: SOLIDITY_VERSION.into(),
                    source_files: solidity_sources,
                    metadata,
                    ..Default::default()
                });
                Ok(solidity::multi_part::VerificationRequest::try_from(request)?.contract)
            }
            Self::SolidityStandardJson => {
                let request = transport(VerifySolidityStandardJsonRequest {
                    bytecode: "0x60016000".into(),
                    bytecode_type: BytecodeType::CreationInput as i32,
                    compiler_version: SOLIDITY_VERSION.into(),
                    input: solidity_input,
                    metadata,
                    ..Default::default()
                });
                Ok(solidity::standard_json::VerificationRequest::try_from(request)?.contract)
            }
            Self::SolidityBatchMultiPart => {
                let request = transport(BatchVerifySolidityMultiPartRequest {
                    contracts: vec![contract(None), contract(metadata)],
                    compiler_version: SOLIDITY_VERSION.into(),
                    sources: solidity_sources,
                    ..Default::default()
                });
                let mut converted =
                    solidity::multi_part::BatchVerificationRequest::try_from(request)?;
                assert_eq!(converted.contracts.len(), 2);
                assert_eq!(converted.contracts[0].chain_id, None);
                assert_eq!(converted.contracts[0].address, None);
                Ok(converted.contracts.pop().unwrap())
            }
            Self::SolidityBatchStandardJson => {
                let request = transport(BatchVerifySolidityStandardJsonRequest {
                    contracts: vec![contract(None), contract(metadata)],
                    compiler_version: SOLIDITY_VERSION.into(),
                    input: solidity_input,
                });
                let mut converted =
                    solidity::standard_json::BatchVerificationRequest::try_from(request)?;
                assert_eq!(converted.contracts.len(), 2);
                assert_eq!(converted.contracts[0].chain_id, None);
                assert_eq!(converted.contracts[0].address, None);
                Ok(converted.contracts.pop().unwrap())
            }
            Self::VyperMultiPart => {
                let request = transport(VerifyVyperMultiPartRequest {
                    bytecode: "0x6000".into(),
                    bytecode_type: BytecodeType::DeployedBytecode as i32,
                    compiler_version: VYPER_VERSION.into(),
                    source_files: vyper_sources,
                    metadata,
                    ..Default::default()
                });
                Ok(vyper::multi_part::VerificationRequest::try_from(request)?.contract)
            }
            Self::VyperStandardJson => {
                let request = transport(VerifyVyperStandardJsonRequest {
                    bytecode: "0x60016000".into(),
                    bytecode_type: BytecodeType::CreationInput as i32,
                    compiler_version: VYPER_VERSION.into(),
                    input: vyper_input,
                    metadata,
                });
                Ok(vyper::standard_json::VerificationRequest::try_from(request)?.contract)
            }
        }
    }
}

#[test]
fn metadata_address_parse_preserves_absent_metadata_and_optional_identity() {
    for path in REQUEST_PATHS {
        for metadata in [None, Some(VerificationMetadata::default())] {
            let converted = path.convert(metadata).unwrap();
            assert_eq!(converted.chain_id, None, "{path:?}");
            assert_eq!(converted.address, None, "{path:?}");
        }
        let converted = path
            .convert(Some(VerificationMetadata {
                chain_id: Some(CHAIN_ID.into()),
                contract_address: None,
            }))
            .unwrap();
        assert_eq!(converted.chain_id.as_deref(), Some(CHAIN_ID), "{path:?}");
        assert_eq!(converted.address, None, "{path:?}");
    }
}

#[test]
fn metadata_address_parse_preserves_valid_twenty_byte_addresses() {
    let expected = Address::from([
        0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd,
        0xef, 0x01, 0x23, 0x45, 0x67,
    ]);
    for path in REQUEST_PATHS {
        for address in [
            ADDRESS.to_string(),
            ADDRESS.to_uppercase().replacen("0X", "0x", 1),
            format!("0x{}", "00".repeat(20)),
        ] {
            for chain_id in [None, Some(CHAIN_ID.to_string())] {
                let converted = path
                    .convert(Some(VerificationMetadata {
                        chain_id: chain_id.clone(),
                        contract_address: Some(address.clone()),
                    }))
                    .unwrap();
                assert_eq!(converted.chain_id, chain_id, "{path:?}");
                let expected_address = if address == format!("0x{}", "00".repeat(20)) {
                    Address::ZERO
                } else {
                    expected
                };
                assert_eq!(
                    converted.address,
                    Some(expected_address),
                    "{path:?}: {address}"
                );
            }
        }
    }
}

#[test]
fn metadata_address_parse_rejects_malformed_short_and_overlong_addresses() {
    let invalid = [
        String::new(),
        "0x".into(),
        "not-an-address".into(),
        format!("0x{}gg", "12".repeat(19)),
        "0x01".into(),
        format!("0x{}", "12".repeat(19)),
        format!("0x{}1", "12".repeat(19)),
        format!("0x{}1", "12".repeat(20)),
        format!("0x{}", "12".repeat(21)),
        format!(" {ADDRESS}"),
        format!("{ADDRESS} "),
    ];
    for path in REQUEST_PATHS {
        for address in &invalid {
            for chain_id in [None, Some(CHAIN_ID.to_string())] {
                let error = path
                    .convert(Some(VerificationMetadata {
                        chain_id,
                        contract_address: Some(address.clone()),
                    }))
                    .expect_err(&format!("{path:?} accepted invalid address {address:?}"));
                assert!(
                    matches!(error, RequestParseError::BadRequest(_)),
                    "{path:?}: {error}"
                );
                assert!(
                    error
                        .to_string()
                        .contains("invalid metadata contract_address"),
                    "{path:?}: {error}"
                );
                if let RequestParseError::BadRequest(error) = error {
                    assert!(
                        error.source().is_some(),
                        "address parser's cause must be retained"
                    );
                }
            }
        }
    }
}
