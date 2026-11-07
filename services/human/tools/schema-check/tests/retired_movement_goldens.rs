use std::fs;
use std::path::Path;

use layerx_human_schema_check::{human_api_schema, parse_json, Json};

#[test]
fn retired_movement_failure_vectors_answer_operation_retired() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/human-api");
    let report = human_api_schema(&root)
        .unwrap_or_else(|violations| panic!("real schema conformance: {violations:#?}"));

    for operation in [
        "deposit.start",
        "deposit.confirm",
        "withdraw.start",
        "withdraw.claim",
        "exit.start",
        "exit.eligibility",
    ] {
        assert!(
            report
                .retired_operations
                .iter()
                .any(|name| name == operation),
            "{operation} must be declared retired"
        );
        let path = root.join(format!("golden/{operation}.failure.json"));
        let source = fs::read_to_string(&path)
            .unwrap_or_else(|error| panic!("read {}: {error}", path.display()));
        let vector =
            parse_json(&source).unwrap_or_else(|error| panic!("parse {}: {error}", path.display()));
        assert_eq!(
            vector.field("status"),
            Some(&Json::Number("410".to_owned())),
            "{operation} must answer HTTP 410"
        );
        let body = vector
            .field("body")
            .unwrap_or_else(|| panic!("{operation} must carry a failure envelope"));
        assert_eq!(body.field("ok"), Some(&Json::Bool(false)), "{operation}");
        let error = body
            .field("error")
            .unwrap_or_else(|| panic!("{operation} must carry a structured error"));
        for (field, expected) in [
            ("code", "operation-retired"),
            ("copy_key", "error.operation.retired"),
            ("retry", "final"),
        ] {
            assert_eq!(
                error.field(field),
                Some(&Json::String(expected.to_owned())),
                "{operation} error.{field}"
            );
        }
        assert_eq!(error.field("field"), None, "{operation}");
        assert_eq!(error.field("retry_after_ms"), None, "{operation}");
    }
}
