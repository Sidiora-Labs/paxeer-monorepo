use layerx_client::ai_market::{market_call_terms, market_program, AiClientError};
use layerx_programs_runtime::abi::{Capability, CapabilitySet};
use layerx_programs_runtime::access::AccessDeclaration;

const PROGRAM: [u8; 32] = [1; 32];
const PRINCIPAL: [u8; 32] = [2; 32];

#[test]
fn market_mutation_terms_match_the_kernel_case() {
    let terms = market_call_terms(market_program::dispatch::RecordDeal, PROGRAM, PRINCIPAL)
        .unwrap_or_else(|error| panic!("mutation terms: {error:?}"));
    assert_eq!(terms.capabilities, [0x00, 0x03, 0x03, 0x07, 0x08]);
    assert_eq!(
        terms.access_declaration,
        b"LayerX/programs/access-declaration/v1\x00\x00"
    );
}

#[test]
fn market_read_terms_hold_the_shared_read_grant() {
    let terms = market_call_terms(market_program::dispatch::READ_HEADER, PROGRAM, PRINCIPAL)
        .unwrap_or_else(|error| panic!("read terms: {error:?}"));
    assert_eq!(terms.capabilities, [0x00, 0x01, 0x07]);
}

#[test]
fn market_call_terms_are_admitted_by_the_runtime_decoders() {
    for (operation, expected) in [
        (
            market_program::dispatch::READ_HEADER,
            vec![Capability::SharedStorageRead],
        ),
        (
            market_program::dispatch::RecordDeal,
            vec![
                Capability::EmitEvent,
                Capability::SharedStorageRead,
                Capability::SharedStorageWrite,
            ],
        ),
    ] {
        let terms = market_call_terms(operation, PROGRAM, PRINCIPAL)
            .unwrap_or_else(|error| panic!("{} terms: {error:?}", operation.metadata().name));
        assert_eq!(
            CapabilitySet::decode_canonical(&terms.capabilities),
            Ok(expected)
        );
        assert_eq!(
            AccessDeclaration::canonical_decode(&terms.access_declaration),
            Ok(AccessDeclaration::Absent)
        );
    }
}

#[test]
fn market_value_operation_terms_are_refused() {
    for operation in [
        market_program::dispatch::FUND,
        market_program::dispatch::CLAIM,
        market_program::dispatch::REFUND_FREE,
        market_program::dispatch::ADVANCE_CLOSE,
        market_program::dispatch::LockBond,
        market_program::dispatch::RefundBond,
        market_program::dispatch::ChallengeLap,
        market_program::dispatch::ResolveChallenge,
    ] {
        assert_eq!(
            market_call_terms(operation, PROGRAM, PRINCIPAL),
            Err(AiClientError::MarketValueOperation {
                operation: operation.metadata().name,
            })
        );
    }
}
