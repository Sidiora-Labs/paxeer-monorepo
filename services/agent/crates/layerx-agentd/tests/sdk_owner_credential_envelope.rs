use layerx_agent_api::error::RequestId;
use layerx_agent_api::idempotency::Key;
use layerx_agentd::agent_rpc::decode;
use layerx_sdk::agent_envelope::{encode_envelope, EnvelopeCredential, EnvelopeError};
use layerx_sdk::Operation;
use serde_json::json;

#[test]
fn sdk_bootstrap_operations_carry_owner_credential_to_daemon_decode() {
    let token_id = [0xab; 32];
    let credential = EnvelopeCredential::new("tenant-a", [0xcd; 32], token_id, 7)
        .expect("valid owner session credential");
    let key = Key::new([0x12; 32]).expect("valid idempotency key");
    let request = json!({});

    for operation in [Operation::AgentRegister, Operation::SessionOpen] {
        let body = encode_envelope(
            operation,
            RequestId(42),
            &request,
            Some(&credential),
            Some(key),
        )
        .expect("bootstrap operation carries the owner session credential");
        let encoded = serde_json::to_vec(&body).expect("serialize SDK envelope");
        let decoded = decode(&encoded).expect("daemon accepts the SDK envelope");
        assert_eq!(decoded.operation, operation);
        assert_eq!(decoded.request_id, RequestId(42));
        assert_eq!(decoded.idempotency_key, Some(key.bytes()));
        let owner = decoded
            .credential
            .expect("owner session credential is present");
        assert_eq!(owner.tenant().as_str(), credential.tenant());
        assert_eq!(owner.session_id().0, credential.session_id());
        assert_eq!(owner.token_id(), token_id);
        assert_eq!(owner.generation(), credential.generation());

        assert_eq!(
            encode_envelope(operation, RequestId(42), &request, None, Some(key)),
            Err(EnvelopeError::CredentialPresence { operation })
        );
    }
}
