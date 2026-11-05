//! Proves: FAUCET_REQUEST, ExactTerms, GrantTerms, request_funds -> ToolDefinition, PrecompileCall, WebPayer -> CLI descriptors, host payer preparation, web payment validation.
//!
//! The Rust items are the crossing layer: these are the same descriptor,
//! unsigned-grant builder, sequence decoder and payment validators called by
//! tools/platform/cli/src/toolset.rs, layerx-agentd-host/src/payer.rs and WebPlane.
//! Real signing and payment codecs produce the grant bytes; no payer or transport
//! is replaced. These interfaces own no persistent state. Each is consumed twice,
//! with changed input between observations to expose an accidental cached result.
//!
//! Repairs tools/faucet.rs::request_funds accepting overflowing and noncanonical
//! funded amounts by sharing the SDK's strict decoder with the CLI consumer.
//! Repairs WebPayer::draw accepting a zero grant or purpose, which the production
//! host already refused; both now use GrantTerms::draw_from_sequence_result.
//! request_funds currently refuses HTTPS endpoints (tools/faucet.rs::post); this
//! proof preserves that explicit refusal rather than inventing a funding service.

use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{Disclosure, IdempotencyRef};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::prepare::evm::{AbiArgument, PrecompileTarget};
use layerx_mcp::catalogue;
use layerx_mcp::server::{DeploymentMode, InvocationOutcome, FAUCET_REQUEST};
use layerx_mcp::tools::faucet::{
    decode_funding_response, invocation_outcome, request_funds, FaucetError,
};
use layerx_mcp::tools::web::{
    ExactTerms, GrantTerms, WebConfig, WebPayer, WebPayerError, WebToolError,
};
use serde_json::{json, Value};

fn exact_terms(amount: u128, key: [u8; 32]) -> ExactTerms {
    let did = "did:layerx:mcp-proof";
    ExactTerms {
        payer_did: did.to_owned(),
        payer_account: [1; 32],
        recipient: [2; 32],
        recipient_account: "agent:did:layerx:receiver:main".to_owned(),
        asset: [3; 32],
        expires_at_ms: 2_000_000_000_000,
        disclosure: Disclosure {
            canonical_digest: [4; 32],
            activity_type: ActivityType(7),
            actor: AgentDid::new(did).unwrap_or_else(|error| panic!("actor: {error:?}")),
            authority: AuthorityRef::new("x402:exact:eip155:125")
                .unwrap_or_else(|error| panic!("authority: {error:?}")),
            counterparties: ExplicitSet::allow(Vec::new()),
            amounts: ExplicitSet::allow(Vec::new()),
            asset: Asset::new("SID").unwrap_or_else(|error| panic!("asset: {error:?}")),
            fee_limit: Amount(0),
            expiry: TimestampSeconds(2_000_000_000),
            idempotency_key: IdempotencyRef::new("mcp-proof")
                .unwrap_or_else(|error| panic!("key: {error:?}")),
        },
        amount,
        idempotency_key: key,
    }
}

fn grant_terms(amount: u128, key: [u8; 32]) -> GrantTerms {
    let exact = exact_terms(amount, key);
    GrantTerms {
        payer_did: exact.payer_did,
        payer_account: exact.payer_account,
        recipient: exact.recipient,
        receiver_did_key: [6; 32],
        asset: exact.asset,
        expires_at_ms: exact.expires_at_ms,
        amount: exact.amount,
        purpose_hash: [5; 32],
        idempotency_key: exact.idempotency_key,
    }
}

#[test]
fn proof_mcp_catalogue_reaches_the_cli_descriptor_consumer_twice() {
    for _ in 0..2 {
        let tools = catalogue::surface(DeploymentMode::Full);
        let definition = tools
            .iter()
            .find(|tool| tool.name == FAUCET_REQUEST.name)
            .unwrap_or_else(|| panic!("faucet is not served"));
        assert_eq!(*definition, FAUCET_REQUEST);
        let schema =
            catalogue::input_schema(definition.name).unwrap_or_else(|| panic!("faucet schema"));
        let descriptor = definition.descriptor(schema.clone());
        let bytes = serde_json::to_vec(&descriptor)
            .unwrap_or_else(|error| panic!("descriptor encode: {error}"));
        let installed: Value = serde_json::from_slice(&bytes)
            .unwrap_or_else(|error| panic!("installation decode: {error}"));
        assert_eq!(installed["name"], "faucet.request");
        assert_eq!(installed["kind"], "write");
        assert_eq!(installed["scope"], FAUCET_REQUEST.required_scope);
        assert_eq!(installed["mutation"], FAUCET_REQUEST.mutation);
        assert_eq!(installed["evidence"], FAUCET_REQUEST.evidence);
        assert_eq!(installed["arguments"], schema);
        let valid = json!({"did": "did:layerx:mcp-proof", "public_key": "ab".repeat(32)});
        assert!(catalogue::validate(definition.name, &valid).is_ok());
        let mut malformed = valid;
        malformed["public_key"] = json!("zz".repeat(32));
        assert!(catalogue::validate(definition.name, &malformed).is_err());
        assert!(!catalogue::surface(DeploymentMode::ReadOnly).contains(definition));
    }
}

#[test]
fn proof_mcp_exact_terms_reach_the_web_consumer_twice_without_a_cache() {
    let mut previous = None;
    for (amount, key) in [(17, [6; 32]), (29, [7; 32])] {
        let terms = exact_terms(amount, key);
        // ProductionWebPayer::prepare_payment calls this very producer.
        let prepared = terms.prepare_pay_exact();
        assert_eq!(prepared.target, PrecompileTarget::Ledger);
        let encoded = prepared
            .encode()
            .unwrap_or_else(|error| panic!("ABI: {error:?}"));
        assert_eq!(&encoded[..4], &[0x78, 0xea, 0x11, 0xdb]);
        assert_eq!(encoded.len(), 4 + 6 * 32);
        assert_eq!(&encoded[4..36], &terms.payer_account);
        assert_eq!(&encoded[36..68], &terms.recipient);
        assert_eq!(&encoded[68..100], &terms.asset);
        assert_eq!(&encoded[116..132], &amount.to_be_bytes());
        assert_eq!(&encoded[164..196], &key);
        assert!(<dyn WebPayer>::validate_exact_call(&terms, &prepared).is_ok());
        if let Some(previous) = previous.replace(encoded.clone()) {
            assert_ne!(previous, encoded);
        }
        let mut malformed = prepared.clone();
        malformed.arguments[3] = AbiArgument::Word([0; 32]);
        assert!(matches!(
            <dyn WebPayer>::validate_exact_call(&terms, &malformed),
            Err(WebToolError::PaymentArtifact("pay_exact_call"))
        ));
        malformed = prepared;
        malformed.target = PrecompileTarget::Pay;
        assert!(matches!(
            <dyn WebPayer>::validate_exact_call(&terms, &malformed),
            Err(WebToolError::PaymentArtifact("pay_exact_call"))
        ));
    }
}

#[test]
fn proof_mcp_signed_grant_reaches_the_web_decoder_twice_without_a_cache() {
    let signer = SigningKey::from_bytes(&[8; 32]);
    let mut previous = None;
    for (amount, key) in [(17, [6; 32]), (29, [7; 32])] {
        let terms = grant_terms(amount, key);
        // This builder is also used by ProductionWebPayer::sign_grant.
        let mut grant = terms.unsigned_grant(signer.verifying_key().to_bytes());
        grant.id = grant
            .signing_digest()
            .unwrap_or_else(|error| panic!("digest: {error:?}"));
        grant.signature = signer.sign(&grant.id).to_bytes();
        let prepared = terms
            .prepare_grant(&grant)
            .unwrap_or_else(|error| panic!("grant preparation: {error:?}"));
        let observed = <dyn WebPayer>::decode_grant_call(&terms, &prepared)
            .unwrap_or_else(|error| panic!("web consumer: {error:?}"));
        assert_eq!(observed, grant);
        assert_eq!(observed.from, terms.payer_account);
        assert_eq!(observed.recipient, terms.recipient);
        assert_eq!(observed.asset, terms.asset);
        assert_eq!(observed.allowance, amount);
        assert_eq!(observed.reference_hash, key);
        if let Some(previous) = previous.replace(observed.id) {
            assert_ne!(previous, observed.id);
        }
        let mut malformed = prepared.clone();
        let [AbiArgument::Bytes(payload)] = malformed.arguments.as_mut_slice() else {
            panic!("grant ABI payload")
        };
        let last = payload
            .last_mut()
            .unwrap_or_else(|| panic!("signed payload"));
        *last ^= 1;
        assert!(matches!(
            <dyn WebPayer>::decode_grant_call(&terms, &malformed),
            Err(WebToolError::PaymentArtifact("grant_call"))
        ));
        let mut wrong_recipient = terms.clone();
        wrong_recipient.recipient = [9; 32];
        assert!(matches!(
            <dyn WebPayer>::decode_grant_call(&wrong_recipient, &prepared),
            Err(WebToolError::Payer(WebPayerError::Refused))
        ));
    }
}

#[test]
fn proof_mcp_sequence_result_is_decoded_at_the_host_and_trait_boundary() {
    let terms = grant_terms(17, [6; 32]);
    for sequence in [11_u64, 12] {
        // The public draw producer emits the canonical sequence word. The host
        // and trait consumer decode that word with the shared strict decoder.
        let producer = terms.prepare_draw([9; 32], sequence);
        let encoded = producer
            .encode()
            .unwrap_or_else(|error| panic!("sequence ABI: {error:?}"));
        let result = &encoded[4 + 4 * 32..4 + 5 * 32];
        let observed = terms
            .draw_from_sequence_result([9; 32], result)
            .unwrap_or_else(|error| panic!("host decoder: {error:?}"));
        assert_eq!(observed, terms.prepare_draw([9; 32], sequence));
        let AbiArgument::Word(word) = &observed.arguments[4] else {
            panic!("sequence word")
        };
        assert_eq!(&word[24..], &sequence.to_be_bytes());
        assert_eq!(
            terms.draw_from_sequence_result([0; 32], result),
            Err(WebPayerError::Refused)
        );
        let mut malformed = result.to_vec();
        malformed[0] = 1;
        assert_eq!(
            terms.draw_from_sequence_result([9; 32], &malformed),
            Err(WebPayerError::Refused)
        );
        assert_eq!(
            terms.draw_from_sequence_result([9; 32], &result[..31]),
            Err(WebPayerError::Refused)
        );
    }
}

#[test]
fn proof_mcp_request_funds_refusals_reach_the_outcome_consumer_twice() {
    for _ in 0..2 {
        let refusal = request_funds(
            "http://127.0.0.1:1/rpc",
            "did:layerx:mcp-proof",
            &[0; 32],
            "session",
        );
        assert!(matches!(&refusal, Err(FaucetError::InvalidRequest)));
        assert_eq!(invocation_outcome(&refusal), InvocationOutcome::Refused);
        let refusal = request_funds(
            "https://127.0.0.1/rpc",
            "did:layerx:mcp-proof",
            &[8; 32],
            "session",
        );
        assert!(matches!(&refusal, Err(FaucetError::Endpoint)));
        assert_eq!(invocation_outcome(&refusal), InvocationOutcome::Refused);
    }
    // Malformed producer results are injected only at the response decoder.
    for amount in ["01", "+1", "340282366920938463463374607431768211456"] {
        let malformed = json!({"jsonrpc": "2.0", "id": 1, "result": {
            "funded": true, "funding_id": "ab".repeat(32),
            "transaction_id": "cd".repeat(32), "amount": amount, "network": "testnet"
        }});
        let refusal = decode_funding_response(&malformed, &json!(1));
        assert!(matches!(&refusal, Err(FaucetError::InvalidResponse)));
        assert_eq!(invocation_outcome(&refusal), InvocationOutcome::Unknown);
    }
    let config = WebConfig::new(
        "http://127.0.0.1:1",
        "did:layerx:mcp-proof",
        "0x1111111111111111111111111111111111111111",
        [2; 20],
        "eip155:125",
        [3; 32],
        [4; 32],
        1,
        2,
        Duration::from_secs(1),
        1,
    );
    assert!(config.is_ok());
}
