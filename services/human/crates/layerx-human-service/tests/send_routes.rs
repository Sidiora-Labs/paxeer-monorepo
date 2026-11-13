use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
use layerx_human_service::journeys::{
    CustodyRoute, Endpoint, EndpointKind, ExecutableShape, LegMechanism, Mechanism, Relationship,
    RouteError, RouteRequest, RouteResolver, SendRoute,
};
use layerx_intents::{Intent, IntentKind};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, CheckpointId, IdempotencyKey};
use layerx_types::intent::{
    AuthorizationSignature, ContextHash, DepositProofId, EvmAddress, NetworkId, ProtocolVersion,
    PublicKey, SendAuthorization, SendAuthorizationKind, Sequence, TimestampSeconds,
};
use sha3::{Digest as _, Keccak256};

fn send_route(pending_nonce: u64) -> SendRoute {
    let signer = SigningKey::from_bytes(&[7; 32]);
    let context = [8; 32];
    SendRoute {
        account_sequence: Sequence::from_u64(77),
        idempotency_key: IdempotencyKey::new([9; 32]),
        expires_at: TimestampSeconds::from_u64(10_000),
        context_hash: ContextHash::new(context),
        authorization: SendAuthorization::new(
            SendAuthorizationKind::Owner,
            PublicKey::new(signer.verifying_key().to_bytes()),
            AuthorizationSignature::new(signer.sign(&context).to_bytes()),
        ),
        network_id: NetworkId::new(125).or_fail("network"),
        protocol_version: ProtocolVersion::new(3).or_fail("protocol"),
    }
    .with_pending_nonce(pending_nonce)
}

fn assert_rejected_bank_variants(request: &RouteRequest) {
    let mut no_pointer = request.clone();
    if let Relationship::BankTransfer { pointer, .. } = &mut no_pointer.relationship {
        *pointer = EvmAddress::new([0; 20]);
    } else {
        panic!("bank transfer relationship");
    }
    assert!(matches!(
        RouteResolver::resolve(&no_pointer),
        Err(RouteError::Unavailable { .. })
    ));
    assert!(matches!(
        RouteRequest::canonical_decode(&no_pointer.canonical_encode()),
        Err(RouteError::Unavailable { .. })
    ));
    let mut zero = request.clone();
    zero.amount = Amount::ZERO;
    assert!(matches!(
        RouteResolver::resolve(&zero),
        Err(RouteError::Unavailable { .. })
    ));
    let mut self_transfer = request.clone();
    if let Relationship::BankTransfer { from, to, .. } = &mut self_transfer.relationship {
        *to = *from;
    } else {
        panic!("bank transfer relationship");
    }
    assert!(matches!(
        RouteResolver::resolve(&self_transfer),
        Err(RouteError::Unavailable { .. })
    ));
}

#[test]
fn send_to_an_evm_address_resolves_to_a_bank_transfer() {
    let secret = [3; 32];
    let signer = k256::ecdsa::SigningKey::from_slice(&secret).or_fail("EVM signer");
    let from = layerx_crypto::secp256k1::evm_address(
        signer.verifying_key().to_encoded_point(false).as_bytes(),
    )
    .or_fail("sender address");
    let to = [4; 20];
    let pointer = [6; 20];
    for (pending_nonce, amount, denom) in [
        (0, 1, "upax"),
        (1, 50, "upax"),
        (
            42,
            25,
            "ibc/0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
        ),
        (u64::MAX, u128::MAX, "upax"),
    ] {
        let request = RouteRequest::from_wire_parts(
            Endpoint::PaxeerWallet,
            Endpoint::PaxeerWallet,
            Relationship::BankTransfer {
                from: EvmAddress::new(from),
                to: EvmAddress::new(to),
                route: send_route(pending_nonce),
                pointer: EvmAddress::new(pointer),
                denom: denom.to_owned(),
                chain_id: 125,
                max_priority_fee_per_gas: 2,
                max_fee_per_gas: 30,
                gas_limit: 100_000,
            },
            AssetId::new([1; 32]),
            Amount::from_u128(amount),
        )
        .or_fail("bank route admitted through public API");
        if let Relationship::BankTransfer { route, .. } = &request.relationship {
            assert_eq!(route.pending_nonce(), pending_nonce);
        } else {
            panic!("bank transfer relationship");
        }
        let wire = request.canonical_encode();
        let decoded = RouteRequest::canonical_decode(&wire).or_fail("canonical request");
        assert_eq!(decoded, request);
        let route = RouteResolver::resolve(&decoded).or_fail("EVM destination route");
        assert!(route.legs().is_empty());
        assert_eq!(route.bank_transfers().len(), 1);
        let call = &route.bank_transfers()[0];
        assert_eq!(call.to, pointer);
        assert_eq!(call.nonce, pending_nonce);
        assert_eq!(call.chain_id, 125);
        assert_eq!(call.max_priority_fee_per_gas, 2);
        assert_eq!(call.max_fee_per_gas, 30);
        assert_eq!(call.gas_limit, 100_000);
        assert_eq!(
            &call.data[..4],
            &Keccak256::digest(b"transfer(address,uint256)")[..4]
        );
        let mut amount_word = [0; 32];
        amount_word[16..].copy_from_slice(&amount.to_be_bytes());
        assert_eq!(call.data.len(), 68);
        assert_eq!(&call.data[4..16], &[0; 12]);
        assert_eq!(&call.data[16..36], &to);
        assert_eq!(&call.data[36..68], &amount_word);
        let disclosure = bind_call(call, from).or_fail("real pointer calldata consumer");
        assert_eq!(
            disclosure.construct_method().or_fail("bank ABI"),
            Some(ConstructMethod::PointerTransfer {
                pointer,
                to,
                amount: amount_word,
            })
        );
        let signed_call = call.sign(&secret).or_fail("signed executable bank call");
        let (transaction, signature) =
            decode_signed(&signed_call).or_fail("canonical signed transaction");
        assert_eq!(&transaction, call);
        assert_eq!(
            recover_sender(&transaction, &signature).or_fail("transaction sender"),
            from
        );
        assert_eq!(
            disclosure.signing_digest().or_fail("signing digest"),
            transaction.signing_hash()
        );

        assert_rejected_bank_variants(&request);
    }
}

#[test]
fn bridge_only_destination_is_refused() {
    let human =
        Endpoint::human(AccountId::parse("agent:did:layerx:human:main").or_fail("human account"))
            .or_fail("human endpoint");
    let request = RouteRequest {
        source: Endpoint::PaxeerWallet,
        destination: human.clone(),
        relationship: Relationship::Custody(CustodyRoute::Deposit {
            deposit_proof: DepositProofId::new([1; 32]),
            checkpoint: CheckpointId::new([2; 32]),
            reserve: AccountId::parse("system:treasury").or_fail("old deposit reserve"),
            idempotency_key: IdempotencyKey::new([3; 32]),
        }),
        asset: AssetId::new([4; 32]),
        amount: Amount::from_u128(50),
    };
    let expected = RouteError::Unavailable {
        source: EndpointKind::PaxeerWallet,
        destination: EndpointKind::Human,
        relationship: "custody-deposit",
    };
    assert_eq!(RouteResolver::resolve(&request), Err(expected.clone()));
    assert_eq!(
        RouteRequest::from_wire_parts(
            request.source.clone(),
            request.destination.clone(),
            request.relationship.clone(),
            request.asset,
            request.amount,
        ),
        Err(expected.clone())
    );
    assert_eq!(
        RouteRequest::canonical_decode(&request.canonical_encode()),
        Err(expected)
    );
    let withdrawal = RouteRequest {
        source: human,
        destination: Endpoint::PaxeerWallet,
        relationship: Relationship::Custody(CustodyRoute::Withdrawal {
            request_anchor: CheckpointId::new([5; 32]),
            fee_limit: 100,
            withdrawals_account: AccountId::parse("system:paxeer-withdrawals")
                .or_fail("old withdrawal account"),
            payout_address: EvmAddress::new([6; 20]),
            idempotency_key: IdempotencyKey::new([7; 32]),
        }),
        ..request
    };
    assert_eq!(
        RouteResolver::resolve(&withdrawal),
        Err(RouteError::Unavailable {
            source: EndpointKind::Human,
            destination: EndpointKind::PaxeerWallet,
            relationship: "custody-withdrawal",
        })
    );
    for retired in [
        LegMechanism::PaxeerCustodyDeposit,
        LegMechanism::PaxeerWithdrawFinalise,
    ] {
        assert_eq!(ExecutableShape::of_mechanisms(&[retired]), None);
        assert_eq!(
            ExecutableShape::of_mechanisms(&[retired, LegMechanism::Protocol(Mechanism::Send)]),
            None
        );
        assert_eq!(
            ExecutableShape::of_mechanisms(&[LegMechanism::Protocol(Mechanism::Send), retired]),
            None
        );
    }
    assert_eq!(
        ExecutableShape::of_mechanisms(&[LegMechanism::Protocol(Mechanism::Send)]),
        Some(ExecutableShape::Kernel)
    );
    let direct = RouteRequest::from_wire_parts(
        Endpoint::human(AccountId::parse("agent:did:layerx:human:main").or_fail("human"))
            .or_fail("human endpoint"),
        Endpoint::agent(AccountId::parse("agent:did:layerx:agent:main").or_fail("agent"))
            .or_fail("agent endpoint"),
        Relationship::Direct(send_route(42)),
        AssetId::new([4; 32]),
        Amount::from_u128(50),
    )
    .or_fail("agent-runtime route");
    let route = RouteResolver::resolve(&direct).or_fail("direct route");
    assert!(route.bank_transfers().is_empty());
    assert_eq!(route.legs().len(), 1);
    let intent: &Intent = route.legs()[0].intent();
    let IntentKind::LxpSend(send) = intent.kind() else {
        panic!("agent-runtime send intent");
    };
    assert_eq!(send.to_wire_parts().4.value(), 42);
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
