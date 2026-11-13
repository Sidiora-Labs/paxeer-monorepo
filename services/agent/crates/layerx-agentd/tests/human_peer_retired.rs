use layerx_agent_api::error::ErrorClass;
use layerx_agentd::human::{encode_response, HumanOperationError, HumanRefusal};
use layerx_types::rung::Rung;

fn field(bytes: &mut Vec<u8>, value: &[u8]) {
    bytes.extend_from_slice(&assert_ok(u32::try_from(value.len()), "bounded field").to_be_bytes());
    bytes.extend_from_slice(value);
}

#[test]
fn export_above_committed_is_rung_unattainable() {
    assert_eq!(Rung::from_name("committed"), Some(Rung::Committed));
    assert_eq!(Rung::Committed.wire_rank(), 2);
    for retired in ["settled", "anchored", "settlement_anchored"] {
        assert_eq!(Rung::from_name(retired), None, "accepted {retired}");
    }

    let refusal = HumanRefusal::ExportLevelUnattainable;
    assert_eq!(refusal.reason(), "export.rung_unattainable");
    assert_eq!(refusal.class(), ErrorClass::VerificationFailure);
    let mut expected = b"LXHAGT01\x01".to_vec();
    field(&mut expected, refusal.reason().as_bytes());
    assert_eq!(
        encode_response(Err(HumanOperationError::Typed(refusal))),
        expected
    );

    for source in [
        include_str!("../src/human.rs"),
        include_str!("../src/export.rs"),
        include_str!("../src/human_runtime.rs"),
        include_str!("../src/agent_rpc_dispatch.rs"),
    ] {
        assert!(!source.contains("SettlementAnchoringUnavailable"));
        assert!(!source.contains("export.settlement_anchoring_unavailable"));
        assert!(!source.contains("export.level_unattainable"));
    }
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}
