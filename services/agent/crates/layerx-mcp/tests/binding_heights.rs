use std::path::Path;
use std::time::Duration;

use layerx_mcp::binding::{Binding, BindingError};
use layerx_mcp::server::DeploymentMode;
use serde_json::{json, Value};

fn document(height: u64) -> Value {
    json!({
        "mode": "full",
        "tenant": "height-tenant",
        "store": "/var/lib/layerx/agent/store",
        "audit_root": "/var/lib/layerx/agent/audit",
        "session_id": "07".repeat(32),
        "session_token_file": "/var/lib/layerx/agent/session-token",
        "session_generation": 1,
        "capability_id": "09".repeat(32),
        "core_height": height,
        "deadline_ms": 5_000,
        "agent": {
            "endpoint": "127.0.0.1:9440",
            "bearer_file": "/var/lib/layerx/agent/bearer",
            "probe_program": "cc".repeat(32),
        },
        "limit": {
            "id": "0a".repeat(16),
            "name": "mcp",
            "scope": "tenant",
            "scope_id": "01".repeat(32),
            "ceiling": "1000",
            "consumed": "0",
        },
    })
}

#[test]
fn binding_document_with_core_sequence_is_refused() {
    for retired in [json!(50), Value::Null, json!("50")] {
        for keep_height in [false, true] {
            let mut value = document(50);
            let fields = value.as_object_mut().unwrap_or_else(|| panic!("object"));
            if !keep_height {
                fields.remove("core_height");
            }
            fields.insert("core_sequence".to_owned(), retired.clone());
            let refusal = Binding::parse(&value.to_string())
                .err()
                .unwrap_or_else(|| panic!("retired key was accepted"));
            assert_eq!(
                refusal,
                BindingError::Refused("mcp.binding_retired_key".to_owned())
            );
        }
    }
}

#[test]
fn binding_document_with_core_height_binds() {
    for height in [0, 50, 9_007_199_254_740_993, u64::MAX] {
        let value = document(height);
        let binding = Binding::parse(&value.to_string())
            .unwrap_or_else(|error| panic!("height {height} refused: {error:?}"));
        assert_eq!(binding.core_height(), height);
        assert_eq!(binding.mode(), DeploymentMode::Full);
        assert_eq!(binding.tenant(), "height-tenant");
        assert_eq!(binding.store(), Path::new("/var/lib/layerx/agent/store"));
        assert_eq!(binding.session_generation(), 1);
        assert_eq!(binding.deadline(), Duration::from_millis(5_000));
        assert_eq!(binding.agent_endpoint(), "127.0.0.1:9440");

        for key in value.as_object().unwrap_or_else(|| panic!("object")).keys() {
            let mut missing = value.clone();
            missing
                .as_object_mut()
                .unwrap_or_else(|| panic!("object"))
                .remove(key);
            assert!(
                matches!(
                    Binding::parse(&missing.to_string()),
                    Err(BindingError::Malformed(_))
                ),
                "missing {key} was accepted"
            );
        }
    }

    for invalid in [Value::Null, json!(-1), json!(1.5), json!("50")] {
        let mut value = document(50);
        value["core_height"] = invalid;
        assert!(matches!(
            Binding::parse(&value.to_string()),
            Err(BindingError::Malformed(_))
        ));
    }
    let mut value = document(50);
    value["unexpected"] = json!(true);
    assert!(matches!(
        Binding::parse(&value.to_string()),
        Err(BindingError::Malformed(_))
    ));
}
