use serde_json::Value;

#[test]
fn health_response_shape_requires_complete_models() {
    use blockscout_client::models::{HealthCheckResponse, UnhealthyHealthCheckResponse};

    let responses: Vec<Value> = serde_json::from_str(include_str!(
        "../blockscout-client/crate/tests/health/repair_22_31_269.json"
    ))
    .unwrap();
    let healthy = &responses[0]["body"];
    for (object, field) in [
        ("", "healthy"),
        ("/metadata", "blocks"),
        ("/metadata/blocks", "healthy"),
        ("/metadata/blocks", "latest_block"),
        ("/metadata/blocks/latest_block", "cache"),
        ("/metadata/blocks/latest_block", "db"),
        ("/metadata/blocks/latest_block", "node"),
        ("/metadata/blocks/latest_block/cache", "number"),
        ("/metadata/blocks/latest_block/cache", "timestamp"),
        ("/metadata/blocks/latest_block/db", "number"),
        ("/metadata/blocks/latest_block/db", "timestamp"),
        ("/metadata/blocks/latest_block/node", "number"),
    ] {
        let mut incomplete = healthy.clone();
        incomplete
            .pointer_mut(object)
            .unwrap()
            .as_object_mut()
            .unwrap()
            .remove(field)
            .unwrap();
        assert!(
            serde_json::from_value::<HealthCheckResponse>(incomplete).is_err(),
            "generated success model must require {object}/{field}"
        );
    }
    let unhealthy = &responses[1]["body"];
    for (object, field) in [
        ("", "healthy"),
        ("", "metadata"),
        ("", "error"),
        ("/error", "code"),
        ("/error", "message"),
    ] {
        let mut incomplete = unhealthy.clone();
        incomplete
            .pointer_mut(object)
            .unwrap()
            .as_object_mut()
            .unwrap()
            .remove(field)
            .unwrap();
        assert!(
            serde_json::from_value::<UnhealthyHealthCheckResponse>(incomplete).is_err(),
            "generated failure model must require {object}/{field}"
        );
    }
    let mut media = responses[4]["body"].clone();
    media["data"]["unexpected"] = serde_json::json!(true);
    assert!(
        serde_json::from_value::<HealthCheckResponse>(media).is_err(),
        "standalone media data must obey additionalProperties: false"
    );
}
