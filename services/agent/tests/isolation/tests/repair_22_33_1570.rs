#[path = "../src/main.rs"]
mod isolation;

use layerx_agentd::store::TenantId;

#[test]
fn isolation_surface_gap_provisions_real_authority_and_drives_the_mcp_sdk_transport() {
    let report = isolation::agent_isolation_suite()
        .unwrap_or_else(|error| panic!("real isolation routes: {error}"));
    assert_eq!(report.surfaces_checked, 7);
    assert_eq!(report.mcp_transport_requests, 5);
    assert_eq!(report.mcp_injection_cases, 10);
    assert_eq!(report.tenant_values_removed, 14);
    assert_eq!(report.legal_audits_retained, 1);
    assert_eq!(report.retained_protocol_values, 0);
}

#[test]
fn isolation_surface_gap_reopen_preserves_only_legal_metadata_and_every_other_tenant_value() {
    let alpha = TenantId::new("alpha").unwrap_or_else(|error| panic!("alpha: {error}"));
    let beta = TenantId::new("beta").unwrap_or_else(|error| panic!("beta: {error}"));
    let (removed, retained_legal, retained_protocol) =
        isolation::tenant_deletion_suite(&alpha, &beta)
            .unwrap_or_else(|error| panic!("real deletion and reopen: {error}"));
    assert_eq!(removed, 14);
    assert_eq!(retained_legal, 1);
    assert_eq!(retained_protocol, 0);
}
