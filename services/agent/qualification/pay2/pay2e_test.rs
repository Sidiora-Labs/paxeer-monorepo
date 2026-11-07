#[path = "tests/support/registered_scenario.rs"]
mod registered_scenario;
#[path = "tests/support/funded_scenario.rs"]
mod scenario;

use scenario::{funded_inputs, funding, required::Required, sequence};
use std::path::PathBuf;

#[test]
fn pay2e_signed_payments_survive_restart() {
    registered_scenario::pay2_signed_payments_survive_restart();
}

#[test]
fn pay2e_funded_sdk_send_and_mcp_grant_draw() {
    let inputs = funded_inputs::ProvisionedInputs::read();
    let cluster = &inputs.cluster;
    let funding = &inputs.funding;
    let payer_key = &inputs.payer_key;
    let recipient_key = &inputs.recipient_key;
    let endpoint = &inputs.gateway_url;
    let payer_rpc = layerx_sdk::rpc::RpcClient::connect_with_ca_der(
        &endpoint,
        Some(scenario::pay2_credential(&payer_key)),
        &inputs.ca_der,
    )
    .required("PAY2 payer RPC")
    .with_clock(
        layerx_client::runtime_clock::RuntimeClock::from_environment().required("PAY2 clock"),
    );
    let recipient_rpc = layerx_sdk::rpc::RpcClient::connect_with_ca_der(
        &endpoint,
        Some(scenario::pay2_credential(&recipient_key)),
        &inputs.ca_der,
    )
    .required("PAY2 recipient RPC")
    .with_clock(
        layerx_client::runtime_clock::RuntimeClock::from_environment().required("PAY2 clock"),
    );
    let payer_signer =
        layerx_crypto::local::LocalSigner::from_derived_account(&inputs.payer_account);
    let recipient_signer =
        layerx_crypto::local::LocalSigner::from_derived_account(&inputs.recipient_account);
    let authority = || {
        sequence::Pay2CommittedAuthority::from_environment()
            .required("PAY2 committed identity authority")
    };
    let committed_rpc = |activity, key| {
        let pinned = authority();
        let coverage = funded_inputs::read_commit_coverage(&cluster.root, activity)?;
        let trust = layerx_sdk::rpc_verification::node_commit::NodeCommitAuthority::new(
            pinned.chain_id,
            pinned
                .validators
                .into_iter()
                .map(|v| (v.public_key, v.voting_power))
                .collect(),
            coverage.covering_height,
            coverage.covering_height,
        )?;
        layerx_sdk::rpc::RpcClient::connect_with_ca_der(
            endpoint,
            Some(scenario::pay2_credential(key)),
            &inputs.ca_der,
        )?
        .with_clock(
            layerx_client::runtime_clock::RuntimeClock::from_environment().required("PAY2 clock"),
        )
        .with_node_commit_authority(trust)
    };
    let committed_payer_rpc = |activity| committed_rpc(activity, payer_key);
    let committed_recipient_rpc = |activity| committed_rpc(activity, recipient_key);
    let output =
        PathBuf::from(std::env::var_os("PAY2_EVIDENCE_OUT").required("PAY2 evidence output path"));
    scenario::qualify(scenario::Scenario {
        cluster,
        funding,
        payer_rpc: &payer_rpc,
        recipient_rpc: &recipient_rpc,
        payer_signer: &payer_signer,
        recipient_signer: &recipient_signer,
        identity_authority: &authority,
        committed_payer_rpc: &committed_payer_rpc,
        committed_recipient_rpc: &committed_recipient_rpc,
        evidence_output: &output,
    })
    .required("PAY2 funded scenario");
}

#[test]
fn pay2_did_is_not_an_evm_nonce_selector() {
    let rpc = layerx_sdk::rpc::RpcClient::connect("http://127.0.0.1:1", None)
        .required("PAY2 local RPC configuration");
    assert!(matches!(
        rpc.get_identity_sequence("did:layerx:treasury"),
        Err(layerx_sdk::rpc::RpcError::InvalidRequest)
    ));
}
