use std::collections::BTreeMap;

use layerx_agent_api::{agent_api_compat_gate, agent_api_schema_v1, AGENT_API_V1_SOURCE};

const BASELINE: &str = include_str!("../../../schema/agent-api/golden/v1.kvx");
const READ_SOURCE: &str = include_str!("../../../schema/agent-api/read.kvx");

fn declarations(source: &str) -> BTreeMap<String, String> {
    let mut section = "";
    let mut values = BTreeMap::new();
    for raw in source.lines() {
        let line = raw.trim();
        if line.is_empty() || line.starts_with('#') || line.starts_with("//") {
            continue;
        }
        if let Some(name) = line
            .strip_prefix('[')
            .and_then(|value| value.strip_suffix(']'))
        {
            section = name;
        } else {
            let (key, value) = line.split_once('=').expect("schema declaration");
            assert!(!section.is_empty(), "declaration must belong to a section");
            let path = format!("{section}.{}", key.trim());
            assert!(
                values
                    .insert(path.clone(), value.trim().to_owned())
                    .is_none(),
                "duplicate {path}"
            );
        }
    }
    values
}

#[test]
fn v1_schema_declares_no_kernel_verification_level() {
    let current = declarations(AGENT_API_V1_SOURCE);
    let baseline = declarations(BASELINE);
    assert_eq!(
        baseline, current,
        "major-two golden declarations must match"
    );
    let contract = agent_api_schema_v1();
    assert_eq!(contract.version.major, 2);
    let previous: Vec<_> = baseline
        .iter()
        .map(|(key, value)| (key.as_str(), value.as_str()))
        .collect();
    let next: Vec<_> = current
        .iter()
        .map(|(key, value)| (key.as_str(), value.as_str()))
        .collect();
    agent_api_compat_gate(
        contract.version.major,
        contract.version.major,
        &previous,
        &next,
    )
    .expect("the golden schema is compatible within major two");

    let operations: Vec<_> = current
        .iter()
        .filter(|(key, _)| key.starts_with("operation.") || key.contains(".operation."))
        .collect();
    assert!(!operations.is_empty(), "operation declarations must exist");
    for (key, value) in operations {
        for retired in [
            "sequencer-signed",
            "BatchIncluded",
            "StateProven",
            "PB1",
            "protocol3",
        ] {
            assert!(
                !key.contains(retired) && !value.contains(retired),
                "retired kernel vocabulary {retired} at {key}"
            );
        }
    }
    assert_eq!(
        current["transport.agent_http.operation.program.interface.request"],
        "\"ProgramDiscoverWire: program_id and requested_rung=receipted; unknown fields refused.\""
    );
    assert_eq!(
        current["transport.agent_http.errors.\"verification.inconsistent\""],
        "\"VerificationFailure Terminal. Verification status violates the rung ordering.\""
    );
    let validation = &current["record.NativeSendPurposeV1.validation"];
    for required in [
        "chain id",
        "EIP-1559 type 2",
        "owner address exactly 20 bytes",
    ] {
        assert!(
            validation.contains(required),
            "native send validation must declare {required}"
        );
    }
    for retired in ["version1", "protocol3", "kernel network", "did:layerx:"] {
        assert!(
            !validation.contains(retired),
            "native send validation retains {retired}"
        );
    }
}

#[test]
fn proof_bundle_operation_declares_receipt_proof_v2() {
    let current = declarations(AGENT_API_V1_SOURCE);
    let read = declarations(READ_SOURCE);
    let request = &current["transport.agent_http.operation.read.proof_bundle.request"];
    let response = &current["transport.agent_http.operation.read.proof_bundle.response"];
    let verification = &current["transport.agent_http.operation.read.proof_bundle.verification"];
    assert_eq!(
        read["type.ProofBundle.format"],
        "\"layerx-receipt-proof-v2\""
    );
    assert!(response.contains("layerx-receipt-proof-v2"));
    assert!(response.contains("read.kvx type.ProofBundle"));
    assert!(response.contains("VerifiedRead<ProofBundle>"));
    for field_set in ["required", "required_committed", "optional"] {
        for field in read[&format!("type.ProofBundle.{field_set}")]
            .trim_start_matches('[')
            .trim_end_matches(']')
            .split(',')
        {
            let field = field.trim().trim_matches('"');
            assert!(!field.is_empty());
            assert!(
                response.contains(field),
                "response must declare ProofBundle field {field}"
            );
        }
    }
    assert!(request.contains("requested_verification_level, a Rung"));
    assert!(request.contains("retained raw signed transaction hash"));
    assert!(request.contains(
        "AccountState additionally binds the subject account at the requested block height"
    ));
    assert!(verification.contains("Activity and Receipt targets require at least Receipted"));
    assert!(verification.contains("AccountState targets require Committed"));
    assert!(verification.contains("Requested rung cannot exceed achieved rung"));
    assert!(verification.contains("Envelope Achieved status and response verificationLevel agree"));
    assert!(verification.contains("independently pinned ReceiptKeys"));
    assert!(verification.contains("EVM receipt inclusion"));
    assert!(verification.contains("CometBFT commit"));
    assert!(response.contains("Bundle <=262144 bytes before hex encoding"));
    assert!(response.contains("complete JSON body <=1048576 bytes"));
}
