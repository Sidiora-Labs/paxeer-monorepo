#[path = "support/live_bank_node.rs"]
mod live_bank_node;
use live_bank_node::{native_pointer, pending, submit, BankNode};

use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_client::rpc::receipt::AuthenticatedReceipt;
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_crypto::disclosure::call::ledger_selector;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_service::custody::{KeyId, Operation};
use layerx_human_service::journeys::{
    Endpoint, MoveJourneyError, MoveLegExecution, MovePlan, Relationship, RouteRequest, SendRoute,
    BANK_ADDRESS,
};
use layerx_human_service::notify::JourneyId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, EvmAddress, NetworkId, ProtocolVersion, PublicKey,
    SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::rung::Rung;
use sha2::Digest as _;
use std::fs;
use std::path::Path;

fn execution(key: u8, nonce: u64, ceiling: u128) -> MoveLegExecution {
    MoveLegExecution::new(
        [key; 32],
        AgentDid::new("did:layerx:alice").or_fail("actor"),
        AuthorityRef::new("custody-human-primary").or_fail("authority"),
        nonce,
        0,
        u64::MAX,
        ceiling,
    )
    .or_fail("execution")
}

fn request(nonce: u64, to: u8, amount: u128) -> RouteRequest {
    RouteRequest {
        source: Endpoint::PaxeerWallet,
        destination: Endpoint::PaxeerWallet,
        relationship: Relationship::BankTransfer {
            from: EvmAddress::new([0x11; 20]),
            to: EvmAddress::new([to; 20]),
            route: SendRoute {
                account_sequence: Sequence::from_u64(nonce),
                idempotency_key: IdempotencyKey::new([to; 32]),
                expires_at: TimestampSeconds::from_u64(2_000),
                context_hash: ContextHash::new([0; 32]),
                authorization: SendAuthorization::new(
                    SendAuthorizationKind::Owner,
                    PublicKey::new([0x11; 32]),
                    AuthorizationSignature::new([0x22; 64]),
                ),
                network_id: NetworkId::new(125).or_fail("network"),
                protocol_version: ProtocolVersion::new(2).or_fail("protocol"),
            },
            pointer: EvmAddress::new([0x88; 20]),
            denom: "uhpx".to_owned(),
            chain_id: 125,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 10,
            gas_limit: 100_000,
        },
        asset: AssetId::new(sha2::Sha256::digest(b"uhpx").into()),
        amount: Amount::from_u128(amount),
    }
}

fn plan(
    requests: Vec<RouteRequest>,
    executions: Vec<MoveLegExecution>,
) -> Result<MovePlan, MoveJourneyError> {
    MovePlan::new_bank_transfers(
        JourneyId::new("jrn_movebanksend").or_fail("journey"),
        [0x44; 32],
        KeyId::new("human-primary").or_fail("key"),
        Operation::ProtocolMutation,
        requests,
        executions,
        100,
        "HPX",
        "a few seconds",
    )
}

#[test]
fn move_plan_is_one_bank_transfer_per_leg() {
    let requests = vec![request(7, 0x22, 90), request(8, 0x55, 40)];
    let executions = vec![execution(1, 7, 1_000_000), execution(2, 8, 1_000_000)];
    let movement = plan(requests.clone(), executions.clone()).or_fail("two-leg bank move");
    assert_eq!(movement.bank_transfers().len(), 2);
    assert!(movement.route().legs().is_empty());
    for (index, call) in movement.bank_transfers().iter().enumerate() {
        assert_eq!(call.to, [0x88; 20]);
        assert_eq!(call.nonce, 7 + index as u64);
        assert_eq!(movement.executions()[index].pending_nonce(), call.nonce);
        assert_eq!(call.chain_id, 125);
        assert_eq!(
            &call.data[..4],
            &ledger_selector("transfer(address,uint256)")
        );
        assert_eq!(call.data.len(), 68);
        assert_eq!(&call.data[4..16], &[0; 12]);
        assert_eq!(
            &call.data[16..36],
            &[if index == 0 { 0x22 } else { 0x55 }; 20]
        );
        let amount = if index == 0 { 90_u128 } else { 40_u128 };
        assert_eq!(&call.data[36..52], &[0; 16]);
        assert_eq!(&call.data[52..68], &amount.to_be_bytes());
        let raw = call.sign(&[0x66; 32]).or_fail("real EIP-1559 signing");
        assert_eq!(
            decode_signed(&raw).or_fail("decode signed bank call").0,
            *call
        );
    }
    let single = MovePlan::new(
        JourneyId::new("jrn_movebanksingle").or_fail("single journey"),
        [0x44; 32],
        KeyId::new("human-primary").or_fail("key"),
        Operation::ProtocolMutation,
        requests[0].clone(),
        vec![executions[0].clone()],
        100,
        "HPX",
        "a few seconds",
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "existing constructor plans a bank transfer"
        )
    });
    assert_eq!(single.bank_transfers(), &movement.bank_transfers()[..1]);
    assert_eq!(single.quote().amount(), 90);
    let quote = movement.quote();
    assert_eq!(quote.amount(), 130);
    assert_eq!(quote.asset_label(), "HPX");
    assert_eq!(quote.fee_estimate(), 100);
    assert_eq!(quote.fee_ceiling(), 2_000_000);
    assert_eq!(quote.arrival_expectation(), "a few seconds");
    assert!(quote.irreversibility().contains("cannot be cancelled"));
    assert!(quote.plain_language().contains("Move 130 HPX"));
    assert!(!movement.complete());
    assert!(!movement.finalised());

    let root = layerx_human_test_support::directory("move-bank-plan");
    std::fs::create_dir_all(&root).or_fail("plan directory");
    let path = root.join("plan");
    std::fs::write(&path, movement.canonical_encode()).or_fail("persist producer's plan");
    drop(movement);
    let loaded = MovePlan::canonical_decode(&std::fs::read(&path).or_fail("read plan"))
        .or_fail("recover complete bank plan");
    assert_eq!(
        loaded,
        plan(requests.clone(), executions.clone()).or_fail("same plan")
    );
    std::fs::remove_dir_all(root).or_fail("remove plan directory");

    assert!(plan(
        requests.clone(),
        vec![execution(1, 7, 1_000_000), execution(2, 9, 1_000_000)]
    )
    .is_err());
    assert!(plan(
        vec![request(u64::MAX, 0x22, 90), request(0, 0x55, 40)],
        vec![
            execution(1, u64::MAX, 1_000_000),
            execution(2, 0, 1_000_000)
        ]
    )
    .is_err());
    assert!(plan(
        requests.clone(),
        vec![execution(1, 7, 999_999), execution(2, 8, 1_000_000)]
    )
    .is_err());
    assert!(plan(
        requests,
        vec![execution(1, 7, 1_000_000), execution(1, 8, 1_000_000)]
    )
    .is_err());
}

fn live_request(
    nonce: u64,
    to: u8,
    amount: u128,
    sender: [u8; 20],
    native_pointer: [u8; 20],
) -> RouteRequest {
    let mut request = request(nonce, to, amount);
    let Relationship::BankTransfer {
        from,
        route,
        pointer,
        max_fee_per_gas,
        gas_limit,
        ..
    } = &mut request.relationship
    else {
        panic!("bank request");
    };
    *from = EvmAddress::new(sender);
    *pointer = EvmAddress::new(native_pointer);
    route.expires_at = TimestampSeconds::from_u64(u64::MAX);
    *max_fee_per_gas = 1_000_000_000_000;
    *gas_limit = 1_000_000;
    request
}

#[derive(Clone, Copy)]
struct BankTransferContext {
    nonce: u64,
    ceiling: u128,
    sender: [u8; 20],
    pointer: [u8; 20],
}

fn complete_receipted_leg(
    node: &BankNode,
    keys: &[ReceiptKeyEntry],
    movement: &mut MovePlan,
    index: usize,
    context: BankTransferContext,
    root: &Path,
) -> (Vec<u8>, AuthenticatedReceipt) {
    let BankTransferContext {
        nonce,
        ceiling,
        sender,
        pointer,
    } = context;
    let raw = movement.bank_transfers()[index]
        .sign(&node.secret)
        .or_fail("sign planned bank transfer");
    let hash = transaction_hash(&raw);
    assert_eq!(submit(node, &raw), hash);
    let receipted = node.receipt(hash, keys, Rung::Receipted);
    assert_eq!(receipted.rung(), Rung::Receipted);
    assert!(
        receipted.execution().success,
        "planned bank transfer reverted: {:?}; node evidence: {}",
        String::from_utf8_lossy(&receipted.execution().return_data),
        node.home.display()
    );
    let index_byte = u8::try_from(index).unwrap_or_else(|_| panic!("leg index fits u8"));
    let mut leg = execution(10 + index_byte, nonce + index as u64, ceiling);
    assert!(!leg.complete());
    assert!(!leg.finalised());
    assert!(leg.complete_from_receipt([0x99; 32], &receipted).is_err());
    assert!(execution(20, nonce + index as u64 + 1, ceiling)
        .complete_from_receipt(hash, &receipted)
        .is_err());
    assert!(execution(20, nonce + index as u64, 0)
        .complete_from_receipt(hash, &receipted)
        .is_err());
    leg.complete_from_receipt(hash, &receipted)
        .or_fail("receipt completes leg");
    assert!(leg.complete());
    assert_eq!(leg.rung(), Rung::Receipted);
    assert!(!leg.finalised());
    let mut unrelated = plan(
        vec![live_request(
            nonce + index as u64,
            0x77,
            90,
            sender,
            pointer,
        )],
        vec![execution(3, nonce + index as u64, ceiling)],
    )
    .or_fail("unrelated bank plan with same nonce");
    assert!(unrelated.complete_bank_leg(0, &raw, &receipted).is_err());
    assert!(!unrelated.executions()[0].complete());
    assert!(!unrelated.finalised());
    assert!(movement
        .complete_bank_leg(1 - index, &raw, &receipted)
        .is_err());
    movement
        .complete_bank_leg(index, &raw, &receipted)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "matching receipt completes planned leg"));
    assert!(movement.executions()[index].complete());
    assert_eq!(movement.complete(), index == 1);
    assert!(!movement.finalised());
    let (key, actor, authority, nonce, before, after, fee) = leg.to_wire_parts();
    let recovered = MoveLegExecution::from_wire_parts(
        key,
        actor.clone(),
        authority.clone(),
        nonce,
        before,
        after,
        fee,
    )
    .unwrap_or_else(|error| {
        panic!(
            "{}: {error:?}",
            "recreate receipt consumer from immutable context"
        )
    });
    drop(leg);
    assert!(!recovered.complete());
    fs::write(root.join(format!("transaction-{index}")), &raw)
        .or_fail("persist signed transaction");
    let committed = node.receipt(hash, keys, Rung::Committed);
    assert_eq!(committed.canonical_bytes(), receipted.canonical_bytes());
    (raw, receipted)
}

fn assert_reverted_transfer(
    node: &BankNode,
    keys: &[ReceiptKeyEntry],
    context: BankTransferContext,
) {
    let BankTransferContext {
        nonce,
        ceiling,
        sender,
        pointer,
    } = context;
    let reverted_plan = plan(
        vec![live_request(nonce + 2, 0x77, u128::MAX, sender, pointer)],
        vec![execution(4, nonce + 2, ceiling)],
    )
    .or_fail("insufficient balance transfer plan");
    let mut invalid_call = reverted_plan.bank_transfers()[0].clone();
    invalid_call.data[..4].copy_from_slice(&[0xff; 4]);
    let raw = invalid_call
        .sign(&node.secret)
        .or_fail("sign reverting bank call");
    let hash = transaction_hash(&raw);
    assert_eq!(submit(node, &raw), hash);
    let reverted = node.receipt(hash, keys, Rung::Receipted);
    assert!(!reverted.execution().success);
    assert!(execution(4, reverted.execution().nonce, ceiling)
        .complete_from_receipt(reverted.activity_id(), &reverted)
        .is_err());
}

#[test]
fn move_leg_completes_from_its_receipt() {
    let node = BankNode::start();
    assert_eq!(node.endpoint.chain_id().or_fail("chain ID"), 125);
    let probe = crate::request(0, 0x22, 90);
    let probe_plan =
        plan(vec![probe], vec![execution(1, 0, 1_000_000)]).or_fail("bank signing probe");
    let probe_raw = probe_plan.bank_transfers()[0]
        .sign(&node.secret)
        .or_fail("sign probe");
    let (probe_call, probe_signature) = decode_signed(&probe_raw).or_fail("decode probe");
    let sender = recover_sender(&probe_call, &probe_signature).or_fail("real sender");
    let keys = node.receipt_keys();
    assert!(!keys.is_empty());
    let native = native_pointer(&node, sender, &keys);
    assert_ne!(native, BANK_ADDRESS);
    let pointer = native;
    let nonce = pending(node.clock.as_ref(), || node.endpoint.pending_nonce(&sender));
    let ceiling = 1_000_000_000_000_000_000;
    let requests = vec![
        live_request(nonce, 0x22, 90, sender, pointer),
        live_request(nonce + 1, 0x55, 40, sender, pointer),
    ];
    let mut movement = plan(
        requests.clone(),
        vec![
            execution(1, nonce, ceiling),
            execution(2, nonce + 1, ceiling),
        ],
    )
    .or_fail("two-leg live bank plan");
    let root = layerx_human_test_support::directory("move-bank-recovery");
    fs::create_dir_all(&root).or_fail("recovery directory");
    let plan_path = root.join("plan");
    fs::write(&plan_path, movement.canonical_encode()).or_fail("persist real producer plan");
    let mut raw_transactions = Vec::new();
    let mut receipts = Vec::new();
    let mut before = Vec::new();
    for recipient in [[0x22; 20], [0x55; 20]] {
        before.push(pending(node.clock.as_ref(), || {
            node.endpoint.balance(&recipient)
        }));
    }
    let context = BankTransferContext {
        nonce,
        ceiling,
        sender,
        pointer,
    };
    for index in 0..2 {
        let (raw, receipted) =
            complete_receipted_leg(&node, &keys, &mut movement, index, context, &root);
        raw_transactions.push(raw);
        receipts.push(receipted);
    }
    assert_eq!(
        fs::read(&plan_path).or_fail("immutable plan"),
        movement.canonical_encode()
    );
    drop(movement);
    let mut recovered =
        MovePlan::canonical_decode(&fs::read(&plan_path).or_fail("read producer plan"))
            .or_fail("restart consumer from durable plan");
    for index in 0..2 {
        let raw = fs::read(root.join(format!("transaction-{index}")))
            .or_fail("recover original signed transaction");
        assert_eq!(raw, raw_transactions[index]);
        let hash = transaction_hash(&raw);
        let committed = node.receipt(hash, &keys, Rung::Committed);
        assert_eq!(committed.rung(), Rung::Committed);
        assert_eq!(
            committed.canonical_bytes(),
            receipts[index].canonical_bytes()
        );
        recovered
            .complete_bank_leg(index, &raw, &committed)
            .or_fail("receipt finalises leg after restart");
        assert!(recovered.executions()[index].complete());
        assert!(recovered.executions()[index].finalised());
        assert_eq!(recovered.executions()[index].rung(), Rung::Committed);
        assert!(recovered
            .complete_bank_leg(index, &raw, &receipts[index])
            .is_err());
        assert!(recovered.executions()[index].finalised());
        assert_eq!(recovered.finalised(), index == 1);
        let recipient = [if index == 0 { 0x22 } else { 0x55 }; 20];
        let after = pending(node.clock.as_ref(), || node.endpoint.balance(&recipient));
        let before =
            u128::from_be_bytes(before[index][16..].try_into().or_fail("recipient balance"));
        assert_eq!(
            u128::from_be_bytes(after[16..].try_into().or_fail("recipient balance")) - before,
            (if index == 0 { 90 } else { 40 }) * 1_000_000_000_000_u128
        );
    }
    assert!(recovered.complete());
    assert!(recovered.finalised());
    assert_reverted_transfer(&node, &keys, context);
    fs::remove_dir_all(root).or_fail("remove recovery directory");
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
