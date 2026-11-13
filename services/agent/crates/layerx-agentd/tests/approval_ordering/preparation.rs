use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_crypto::disclosure::CounterpartyRole;
use layerx_crypto::send::{send_context_hash, SendDebit};
use layerx_crypto::SignatureMessage;
use layerx_types::payload::ModuleId;
use layerx_wire::hash::Domain;
use sha2::{Digest as _, Sha256};

use crate::prepare::evm::{
    prepare_bound, EvmPreparationState, PrecompileCall, PreparationDefaults, PrepareRequest,
};
use crate::prepare::EvmPrepared;

pub(super) fn produced(id: u8, expiry: u64) -> EvmPrepared {
    let debit = SendDebit {
        from: [4; 32],
        to: [5; 32],
        asset: [6; 32],
        amount: 50,
        source_sequence: 10,
        idempotency_key: [id; 32],
        expires_at: expiry,
        context_hash: send_context_hash(&[4; 32], &[5; 32], &[6; 32], 50, &[id; 32]),
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: 125,
        protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
    };
    let key = SigningKey::from_bytes(&[0x66; 32]);
    let authorization = lint_success(debit.authorization_message(), "send authorization");
    let message = lint_success(
        SignatureMessage::new(
            Domain::SignaturePreimage,
            debit.protocol_version,
            debit.network_id,
            &authorization,
        ),
        "send signature domain",
    );
    let payload = lint_success(
        debit.encode_signed(
            key.verifying_key().to_bytes(),
            key.sign(&message.digest()).to_bytes(),
        ),
        "signed send payload",
    );
    lint_success(
        prepare_bound(
            EvmPreparationState {
                chain_id: 125,
                pending_nonce: u64::from(id),
                base_fee_per_gas: 0,
                head_height: 10,
            },
            PreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 2,
                maximum_payload_bytes: 1_024,
            },
            PrepareRequest {
                from: [4; 20],
                call: PrecompileCall::ledger_transfer(payload),
                expected_nonce: Some(u64::from(id)),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 1_024,
            },
            100_000,
        ),
        "ledger preparation and disclosure",
    )
}

pub(super) fn adapt(evm: &EvmPrepared) -> Prepared {
    lint_success(
        crate::prepare::verify_call_binding(evm),
        "producer call binding",
    );
    let disclosure = &evm.disclosure;
    assert_eq!(disclosure.activity_type.module(), ModuleId::Asset);
    let recipients = disclosure
        .counterparties
        .iter()
        .filter(|party| party.role == CounterpartyRole::Recipient)
        .collect::<Vec<_>>();
    assert_eq!(recipients.len(), 1, "ledger send has one recipient");
    assert_eq!(disclosure.amounts.len(), 1, "ledger send has one amount");
    let bytes = evm.call.encode_unsigned();
    let digest: [u8; 32] = Sha256::digest(&bytes).into();
    let expiry = TimestampSeconds(disclosure.expiry.payload_expires_at);
    Prepared {
        preparation_ref: lint_success(
            PreparationRef::new(hex::encode(digest)),
            "preparation reference",
        ),
        unsigned_canonical_bytes: lint_success(CanonicalBytes::new(bytes), "canonical call"),
        signing_preimage: lint_success(
            SigningPreimage::new(evm.signing_hash.to_vec()),
            "signing hash",
        ),
        disclosure: Disclosure {
            canonical_digest: digest,
            activity_type: ActivityType(disclosure.activity_type.ordinal()),
            actor: lint_success(AgentDid::new(hex::encode(&disclosure.actor)), "call actor"),
            authority: lint_success(
                AuthorityRef::new(hex::encode(&disclosure.authority)),
                "call authority",
            ),
            counterparties: ExplicitSet::allow(
                disclosure
                    .counterparties
                    .iter()
                    .map(|party| {
                        lint_success(AgentDid::new(hex::encode(party.account)), "counterparty")
                    })
                    .collect(),
            ),
            amounts: ExplicitSet::allow(vec![DisclosedAmount {
                counterparty: lint_success(
                    AgentDid::new(hex::encode(recipients[0].account)),
                    "send recipient",
                ),
                amount: Amount(disclosure.amounts[0].value),
            }]),
            asset: lint_success(Asset::new(hex::encode(disclosure.asset)), "send asset"),
            fee_limit: Amount(disclosure.fee_limit),
            expiry,
            idempotency_key: lint_success(
                IdempotencyRef::new(hex::encode(disclosure.idempotency_key)),
                "send idempotency",
            ),
        },
        expiry,
    }
}

pub(super) fn prepared(id: u8, expiry: u64) -> Prepared {
    adapt(&produced(id, expiry))
}

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}
