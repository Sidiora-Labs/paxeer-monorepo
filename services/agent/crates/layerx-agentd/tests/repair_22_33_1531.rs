//! TLS exchange and input-bound cases run under the library's outbound_tls::tests filter.

use layerx_agentd::human::HumanOperationError;
use layerx_agentd::human_runtime::RemoteHumanAuthority;
use layerx_agentd::test_support::private_ca;
use std::time::Duration;

#[test]
fn private_ca_configuration_and_authority_refuse_empty_trust_roots() {
    assert!(private_ca(&[]).is_none());
    assert!(matches!(
        RemoteHumanAuthority::connect(
            "https://localhost",
            "a".repeat(32),
            Duration::from_secs(1),
            1024,
            &[],
        ),
        Err(HumanOperationError::Refused)
    ));
}
