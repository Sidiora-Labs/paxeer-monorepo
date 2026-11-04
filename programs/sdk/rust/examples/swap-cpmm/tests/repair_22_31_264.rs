#[path = "../src/token_call.rs"]
mod token_call;

use layerx_program_sdk::{
    lxt20::Request,
    payments::{PaymentGrant, PreparedProgramAccount},
    Amount, AssetId, Capability, GrantedCapabilities, Principal, ProgramId,
};
use token_call::{NestedTokenCall, TokenCall};

fn program(byte: u8) -> ProgramId {
    ProgramId::new([byte; 32]).unwrap()
}

fn principal(byte: u8) -> Principal {
    Principal::new([byte; 32]).unwrap()
}

fn asset(byte: u8) -> AssetId {
    AssetId::new([byte; 32]).unwrap()
}

#[test]
fn finding_001_pull_routes_canonical_transfer_from_with_pool_spender() {
    for (token, backing) in [(program(0x5a), asset(0x6a)), (program(0x5b), asset(0x6b))] {
        let pool = program(0x55);
        let owner = principal(0x71);
        let amount = Amount::from_u128(4_000);
        let call = TokenCall::pull(token, backing, pool, owner, amount).unwrap();
        let pool_seed = pool.bytes();
        let recipient = PreparedProgramAccount::new(token, &pool_seed, backing)
            .unwrap()
            .account();
        assert_eq!(call.program(), token);
        assert_eq!(
            Request::decode(call.input()).unwrap(),
            Request::TransferFrom {
                owner,
                to: recipient,
                amount
            }
        );
        let received = NestedTokenCall::decode(call.input(), Some(pool)).unwrap();
        assert_eq!(received.owner, owner);
        assert_eq!(received.spender, Some(principal(0x55)));
        assert_ne!(received.spender, Some(owner));
        assert_eq!(received.to, recipient);
        assert_eq!(received.amount, amount);

        let grants = call.capabilities().unwrap();
        let source_seed = owner.bytes();
        let source = PreparedProgramAccount::new(token, &source_seed, backing).unwrap();
        assert_eq!(grants.len(), 3);
        assert!(grants
            .grants()
            .contains(&PaymentGrant::Basic(Capability::SharedStorageRead)));
        assert!(grants
            .grants()
            .contains(&PaymentGrant::Basic(Capability::SharedStorageWrite)));
        assert!(grants.grants().contains(&PaymentGrant::ProgramSpend {
            account: source,
            to: recipient,
            maximum: amount,
        }));
        let mut encoded = [0; 256];
        let length = grants.encode_into(&mut encoded).unwrap();
        let transport = GrantedCapabilities::new(&encoded[..length]).unwrap();
        let mut expected = source
            .spend_grant(recipient, amount)
            .unwrap()
            .as_slice()
            .to_vec();
        expected[..2].copy_from_slice(&3_u16.to_be_bytes());
        expected.extend_from_slice(&[7, 8]);
        assert_eq!(transport.bytes(), expected);
    }
}

#[test]
fn finding_001_push_debits_immediate_pool_caller_without_user_allowance() {
    let pool = program(0x55);
    let recipient = principal(0x72);
    let token = program(0x5b);
    let backing = asset(0x6b);
    let amount = Amount::from_u128(908);
    let call = TokenCall::push(token, backing, pool, recipient, amount).unwrap();
    let recipient_seed = recipient.bytes();
    let to = PreparedProgramAccount::new(token, &recipient_seed, backing)
        .unwrap()
        .account();
    assert_eq!(
        Request::decode(call.input()).unwrap(),
        Request::Transfer { to, amount }
    );
    let received = NestedTokenCall::decode(call.input(), Some(pool)).unwrap();
    assert_eq!(received.owner.bytes(), pool.bytes());
    assert_ne!(received.owner, recipient);
    assert_eq!(received.spender, None);
    let pool_seed = pool.bytes();
    let source = PreparedProgramAccount::new(token, &pool_seed, backing).unwrap();
    let grants = call.capabilities().unwrap();
    assert_eq!(grants.len(), 3);
    assert!(grants.grants().contains(&PaymentGrant::ProgramSpend {
        account: source,
        to,
        maximum: amount,
    }));

    let other_pool = program(0x56);
    let received = NestedTokenCall::decode(call.input(), Some(other_pool)).unwrap();
    assert_eq!(received.owner.bytes(), other_pool.bytes());
    let pull = TokenCall::pull(token, backing, pool, recipient, amount).unwrap();
    let received = NestedTokenCall::decode(pull.input(), Some(other_pool)).unwrap();
    assert_eq!(received.spender.unwrap().bytes(), other_pool.bytes());
    assert_eq!(received.owner, recipient);
}

#[test]
fn finding_001_nested_route_refuses_non_settlement_and_noncanonical_requests() {
    let pool = program(0x55);
    let owner = principal(0x71);
    let call = TokenCall::pull(
        program(0x5a),
        asset(0x6a),
        pool,
        owner,
        Amount::from_u128(1),
    )
    .unwrap();
    assert!(NestedTokenCall::decode(call.input(), None).is_err());
    for request in [
        Request::Approve {
            spender: owner,
            amount: Amount::from_u128(10),
        },
        Request::BalanceOf { owner },
        Request::Allowance {
            owner,
            spender: owner,
        },
        Request::TotalSupply,
        Request::Metadata,
    ] {
        assert!(NestedTokenCall::decode(request.encode().unwrap().as_slice(), Some(pool)).is_err());
    }
    for length in 0..call.input().len() {
        assert!(NestedTokenCall::decode(&call.input()[..length], Some(pool)).is_err());
    }
    let mut trailing = call.input().to_vec();
    trailing.push(0);
    assert!(NestedTokenCall::decode(&trailing, Some(pool)).is_err());
    let mut zero_amount = call.input().to_vec();
    zero_amount[74..90].fill(0);
    assert!(NestedTokenCall::decode(&zero_amount, Some(pool)).is_err());
    let mut invalid_tag = call.input().to_vec();
    invalid_tag[5] = 0x21;
    assert!(NestedTokenCall::decode(&invalid_tag, Some(pool)).is_err());
    assert!(TokenCall::pull(program(0x5a), asset(0x6a), pool, owner, Amount::ZERO).is_err());
    assert!(TokenCall::push(program(0x5a), asset(0x6a), pool, owner, Amount::ZERO).is_err());
}
