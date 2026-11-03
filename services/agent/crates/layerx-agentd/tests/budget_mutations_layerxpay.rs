use layerx_agentd::budget::{
    budget_create_identity, BudgetCreateIdentity, BudgetCreationError, BudgetKind, BudgetRequest,
    ProtocolBudgetState,
};
use layerx_agentd::prepare::evm::PrecompileTarget;
use layerx_agentd::store::TenantId;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_types::payload::ModuleRegistry;
use serde_json::json;

pub fn registry() -> ModuleRegistry {
    ModuleRegistry::new(&[]).expect("empty registry: EVM calls do not use kernel ordinals")
}

pub fn request() -> BudgetRequest {
    BudgetRequest {
        tenant: TenantId::new("budget-owner").expect("tenant"),
        request_id: [9; 32],
        kind: BudgetKind::ProtocolBudget,
        asset: [3; 32],
        ceiling: 5_000,
        expiry_ms: 90_000,
        core_time_ms: 1_000,
        canonical_activity: Vec::new(),
        verified_submission: None,
    }
}

pub fn identity() -> BudgetCreateIdentity {
    BudgetCreateIdentity {
        encoding_version: 1,
        budget_id: [1; 32],
        sender: [0; 20],
        budget_account: [2; 32],
        asset: [3; 32],
        purpose: [4; 32],
        per_period_limit: 5_000,
        carry_cap: 0,
        initial_amount: 5_000,
        period_length_ms: 60_000,
        period_start_ms: 1_000,
        expiry_ms: 90_000,
        revocation_sequence: 7,
        rollover: 1,
        source_account: [0; 32],
        source_sequence: 0,
    }
}

pub fn signed_call(request: &BudgetRequest, identity: &BudgetCreateIdentity) -> Vec<u8> {
    let prepared = request
        .create_protocol_budget(identity)
        .expect("budget calldata");
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 3,
        gas_limit: 100_000,
        to: prepared.target.address(),
        data: prepared.encode().expect("ABI encoding"),
    }
    .sign(&[0xa5; 32])
    .expect("real secp256k1 signature")
}

pub fn budget_result() -> Vec<u8> {
    let mut result = vec![0; 20 * 32];
    for (index, value) in [
        (0, 32_u128),
        (5, 5_000),
        (6, 5_000),
        (10, 60_000),
        (11, 1_000),
        (12, 90_000),
        (13, 7),
        (14, 1),
        (17, 18 * 32),
    ] {
        result[index * 32 + 16..(index + 1) * 32].copy_from_slice(&value.to_be_bytes());
    }
    for (index, value) in [(1, 0x11), (2, 2), (3, 3), (4, 4)] {
        result[index * 32..(index + 1) * 32].fill(value);
    }
    result
}

#[test]
fn budget_create_prepares_create_budget_calldata() {
    let request = request();
    let mut template = identity();
    template.asset = [0xee; 32];
    template.per_period_limit = 1;
    template.expiry_ms = 2;
    let prepared = request
        .create_protocol_budget(&template)
        .expect("createBudget preparation");
    assert_eq!(
        prepared.target.address(),
        [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1b]
    );
    let abi: serde_json::Value = serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxpay/abi.json"
    ))
    .expect("deployed layerxpay ABI");
    let method = abi
        .as_array()
        .expect("ABI array")
        .iter()
        .find(|entry| entry["name"] == "budgetCreate")
        .expect("deployed create method");
    let inputs = method["inputs"].as_array().expect("ABI inputs");
    let signature = format!(
        "budgetCreate({})",
        inputs
            .iter()
            .map(|argument| argument["type"].as_str().expect("ABI type"))
            .collect::<Vec<_>>()
            .join(",")
    );
    assert_eq!(
        prepared.selector,
        transaction_hash(signature.as_bytes())[..4]
    );
    let data = prepared.encode().expect("calldata");
    assert_eq!(data.len(), 4 + inputs.len() * 32);
    assert_eq!(&data[4 + 2 * 32..4 + 3 * 32], &request.asset);
    assert_eq!(&data[4 + 4 * 32..4 + 4 * 32 + 16], &[0; 16]);
    assert_eq!(
        &data[4 + 4 * 32 + 16..4 + 5 * 32],
        &request.ceiling.to_be_bytes()
    );
    assert_eq!(&data[4 + 9 * 32..4 + 9 * 32 + 24], &[0; 24]);
    assert_eq!(
        &data[4 + 9 * 32 + 24..4 + 10 * 32],
        &request.expiry_ms.to_be_bytes()
    );
    let signed = signed_call(&request, &template);
    let identity =
        budget_create_identity(signed.as_slice(), &registry()).expect("signed EVM identity");
    assert_eq!(identity.asset, request.asset);
    assert_eq!(identity.per_period_limit, request.ceiling);
    assert_eq!(identity.expiry_ms, request.expiry_ms);
    let (call, signature) = decode_signed(&signed).expect("signed transaction");
    assert_eq!(
        identity.sender,
        recover_sender(&call, &signature).expect("recover sender")
    );
    assert_eq!(call.to, PrecompileTarget::Pay.address());
    let context = layerx_crypto::disclosure::BudgetStateContext {
        budget_id: [1; 32],
        owner: [0x11; 32],
        budget_account: [2; 32],
        asset: request.asset,
        source_account: [0x11; 32],
        native_source: false,
        revocation_sequence: 7,
        balance: 5_000,
        state_digest: transaction_hash(&budget_result()),
        observed_head_sequence: 42,
        purpose_hash: [4; 32],
    };
    for (signature, value) in [
        ("budgetFund(bytes32,uint128)", 100_u128),
        ("budgetRevoke(bytes32,uint64)", 42),
    ] {
        let mut mutation_call = call.clone();
        mutation_call.data = transaction_hash(signature.as_bytes())[..4].to_vec();
        mutation_call.data.extend_from_slice(&context.budget_id);
        mutation_call.data.extend_from_slice(&[0; 16]);
        mutation_call.data.extend_from_slice(&value.to_be_bytes());
        let signed_mutation = mutation_call.sign(&[0xa5; 32]).expect("signed mutation");
        let mutation = layerx_agentd::budget::budget_mutation_identity(
            &signed_mutation,
            &registry(),
            &context,
        )
        .expect("layerxpay mutation identity");
        match mutation {
            layerx_agentd::budget::BudgetMutation::Fund {
                amount,
                source_height,
                ..
            } => {
                assert_eq!(amount, 100);
                assert_eq!(source_height, None);
            }
            layerx_agentd::budget::BudgetMutation::Revoke {
                revocation_height, ..
            } => assert_eq!(revocation_height, 42),
        }
    }
    let mut invalid = request.clone();
    invalid.ceiling = 0;
    assert_eq!(
        invalid.create_protocol_budget(&template),
        Err(BudgetCreationError::InvalidLimit)
    );
    invalid = request.clone();
    invalid.expiry_ms = invalid.core_time_ms;
    assert_eq!(
        invalid.create_protocol_budget(&template),
        Err(BudgetCreationError::Expired)
    );
}
