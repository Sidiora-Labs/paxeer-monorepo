#[path = "support/funded_peer.rs"]
mod funded_peer;
#[path = "support/funded_scenario.rs"]
mod scenario;

#[test]
fn pay2_funded_scenario_has_an_executable_cargo_target() {
    let manifest = include_str!("../Cargo.toml");
    let targets: Vec<_> = manifest
        .split("[[test]]")
        .skip(1)
        .map(|section| section.split("\n[").next().expect("test target section"))
        .collect();
    assert_eq!(
        targets
            .iter()
            .filter(|section| {
                section
                    .lines()
                    .any(|line| line.trim() == "name = \"pay2e\"")
                    && section
                        .lines()
                        .any(|line| line.trim() == "path = \"pay2e_test.rs\"")
            })
            .count(),
        1,
        "the funded scenario must be a registered executable Cargo target"
    );
    let workflow = include_str!("../pay2e_test.rs");
    assert!(workflow.contains("#[test]\nfn pay2e_funded_sdk_send_and_mcp_grant_draw()"));
    assert!(workflow.contains("scenario::qualify(scenario::Scenario {"));
    let _implementation: for<'a> fn(
        scenario::Scenario<'a>,
    ) -> Result<(), layerx_sdk::rpc::RpcError> = scenario::qualify;
}

#[test]
fn pay2_input_test_gap_executes_the_funded_sdk_and_mcp_workflow() {
    assert_eq!(scenario::funded_inputs::PROTOCOL_VERSION, 2);
    let mut peer = funded_peer::Peer::start();
    let payer_rpc = peer.rpc();
    let recipient_rpc = peer.rpc();
    let authority = || peer.identity_authority();
    let committed_payer_rpc = |activity| peer.committed_rpc(activity);
    let output = peer.cluster.root.join("funded-evidence.json");
    let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        scenario::qualify(scenario::Scenario {
            cluster: &peer.cluster,
            funding: &peer.funding,
            payer_rpc: &payer_rpc,
            recipient_rpc: &recipient_rpc,
            payer_signer: &peer.payer,
            recipient_signer: &peer.recipient,
            identity_authority: &authority,
            committed_payer_rpc: &committed_payer_rpc,
            committed_recipient_rpc: &committed_payer_rpc,
            evidence_output: &output,
        })
    }));
    peer.finish();
    let result = result.unwrap_or_else(|payload| std::panic::resume_unwind(payload));
    if let Err(error) = result {
        let version = peer
            .receipt_version
            .load(std::sync::atomic::Ordering::Acquire);
        record_send_failure(
            "send",
            &error,
            version,
            scenario::funded_inputs::PROTOCOL_VERSION,
        );
    }
    let evidence: serde_json::Value = serde_json::from_slice(
        &std::fs::read(output).expect("the real funded workflow must write its evidence"),
    )
    .expect("funded evidence JSON");
    for operation in [
        "send_executed",
        "send_batched",
        "sdk_grant_issue",
        "sdk_grant_draw",
        "mcp_grant_issue",
        "mcp_grant_draw",
    ] {
        assert_eq!(evidence[operation]["result_code"], 0, "funded {operation}");
    }
    let mut covered_activities = std::collections::BTreeSet::new();
    let mut covering_heights = std::collections::BTreeSet::new();
    for (index, operation) in [
        "send_batched",
        "sdk_grant_issue",
        "sdk_grant_draw",
        "mcp_grant_issue",
        "mcp_grant_draw",
    ]
    .into_iter()
    .enumerate()
    {
        assert_eq!(
            evidence[operation]["global_sequence"],
            2 * u64::try_from(index).expect("funded operation index") + 1,
            "funded {operation} must reserve the next sequence for maintenance",
        );
        assert_eq!(
            evidence[operation]["commitment"], "batched",
            "funded {operation}"
        );
        let activity = evidence[operation]["activity_id"]
            .as_str()
            .expect("verified activity identity");
        assert!(
            covered_activities.insert(activity),
            "distinct funded activity"
        );
        let coverage: serde_json::Value = serde_json::from_slice(
            &std::fs::read(
                peer.cluster
                    .root
                    .join(format!("commit-coverage-{activity}.json")),
            )
            .expect("activity-specific commitment coverage"),
        )
        .expect("committed coverage JSON");
        assert_eq!(coverage["activity_ids"], serde_json::json!([activity]));
        assert!(
            coverage["covering_height"]
                .as_u64()
                .expect("covering height")
                > 0
        );
        covering_heights.insert(
            coverage["covering_height"]
                .as_u64()
                .expect("covering height")
                .to_string(),
        );
    }
    let requests = peer.requests.lock().expect("observed real SDK transport");
    assert!(requests
        .iter()
        .any(|request| request["method"] == "eth_sendRawTransaction"));
    assert!(requests
        .iter()
        .any(|request| request["method"] == "pax_sequencerReceipt"));
    let batch_heights: std::collections::BTreeSet<_> = requests
        .iter()
        .filter(|request| request["method"] == "px_getSequenceBatch")
        .map(|request| {
            let params = request["params"].as_array().expect("batch parameters");
            assert_eq!(params.len(), 1);
            params[0].as_str().expect("decimal batch height").to_owned()
        })
        .collect();
    assert_eq!(
        batch_heights, covering_heights,
        "every funded Batched wait must acquire its coverage through the real SDK transport"
    );
}

fn rpc_error_variant(error: &layerx_sdk::rpc::RpcError) -> &'static str {
    use layerx_sdk::rpc::RpcError;
    match error {
        RpcError::Clock(_) => "Clock",
        RpcError::Transport => "Transport",
        RpcError::Verification => "Verification",
        RpcError::Signing(_) => "Signing",
        RpcError::StaleSourceSequence => "StaleSourceSequence",
        RpcError::Reverted { .. } => "Reverted",
        RpcError::MissingFinalityTrust => "MissingFinalityTrust",
        RpcError::Pending { .. } => "Pending",
        RpcError::InvalidResponse => "InvalidResponse",
        RpcError::InvalidRequest => "InvalidRequest",
        RpcError::Configuration(_) => "Configuration",
        RpcError::Remote { .. } => "Remote",
    }
}

fn record_send_failure(
    stage: &str,
    error: &layerx_sdk::rpc::RpcError,
    producer_version: u16,
    policy_version: u16,
) -> ! {
    let variant = rpc_error_variant(error);
    let kind = if matches!(error, layerx_sdk::rpc::RpcError::Verification)
        && producer_version != 0
        && producer_version != policy_version
    {
        "receipt-policy-version-mismatch"
    } else {
        "rpc-error"
    };
    eprintln!(
        "PAY2 diagnostic stage={stage} kind={kind} error_type=layerx_sdk::rpc::RpcError variant={variant} producer_version={producer_version} policy_version={policy_version}; contents omitted; finding=22.30.99/pay2-input-test-gap"
    );
    panic!("PAY2 {stage} failed: {kind}, layerx_sdk::rpc::RpcError::{variant}; contents omitted");
}
