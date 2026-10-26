use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};

use layerx_human_schema_check::{
    baseline_manifest, human_api_schema, parse_json, Json, SchemaReport, BASELINE_FILE,
};

fn real_schema_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/human-api")
}

fn read_schema(file: &str) -> String {
    fs::read_to_string(real_schema_root().join(file))
        .unwrap_or_else(|error| panic!("read {file}: {error}"))
}

fn declaration(file: &str, section: &str, key: &str) -> String {
    let source = read_schema(file);
    let header = format!("[{section}]");
    let mut lines = source.lines().skip_while(|line| *line != header);
    assert_eq!(lines.next(), Some(header.as_str()), "missing {header}");
    lines
        .take_while(|line| !line.starts_with('['))
        .filter_map(|line| line.split_once('='))
        .find_map(|(name, value)| (name.trim() == key).then(|| value.trim().to_owned()))
        .unwrap_or_else(|| panic!("missing {file}:{section}.{key}"))
}

fn real_schema_report() -> SchemaReport {
    let root = real_schema_root();
    let report = human_api_schema(&root)
        .unwrap_or_else(|violations| panic!("real schema conformance: {violations:#?}"));
    let manifest = baseline_manifest(&root)
        .unwrap_or_else(|violations| panic!("real schema baseline: {violations:#?}"));
    assert_eq!(
        read_schema(BASELINE_FILE),
        manifest,
        "baseline must be refreshed"
    );
    assert_eq!(report.additions_beyond_baseline, 0);
    report
}

fn retired(file: &str, section: &str) {
    assert_eq!(
        declaration(file, section, "retired"),
        "\"true\"",
        "{section}"
    );
}

fn retired_variants() -> Vec<String> {
    match parse_json(&declaration(
        "errors.kvx",
        "type.ErrorCode",
        "retired_variants",
    )) {
        Ok(Json::Array(values)) => values
            .into_iter()
            .map(|value| match value {
                Json::String(value) => value,
                other => panic!("retired variant must be a string: {other:?}"),
            })
            .collect(),
        other => panic!("retired variants must be a string list: {other:?}"),
    }
}

#[test]
fn real_schema_marks_eleven_operations_retired() {
    let report = real_schema_report();
    let expected = [
        "binding.rebind",
        "binding.rebind.action",
        "binding.statement",
        "binding.status",
        "binding.submit",
        "deposit.confirm",
        "deposit.start",
        "exit.eligibility",
        "exit.start",
        "withdraw.claim",
        "withdraw.start",
    ];
    assert_eq!(report.retired_operations, expected);

    for name in [
        "SettlementDomain",
        "DepositStartRequest",
        "DepositConfirmRequest",
        "WithdrawStartRequest",
        "WithdrawClaimRequest",
        "ExitStartRequest",
        "ExitEligibility",
    ] {
        retired("movement.kvx", &format!("type.{name}"));
    }
    for name in [
        "BindingStatementRequest",
        "BindingStatement",
        "BindingSubmission",
        "BindingState",
        "WalletBinding",
        "RebindingSubmission",
        "BindingRebindAction",
    ] {
        retired("identity.kvx", &format!("type.{name}"));
    }
}

#[test]
fn real_schema_declares_operation_retired_error_code() {
    let report = real_schema_report();
    let expected = vec!["wallet-not-bound".to_owned(), "exit-unavailable".to_owned()];
    assert_eq!(retired_variants(), expected);
    assert_eq!(report.retired_variants.get("ErrorCode"), Some(&expected));
    assert_eq!(report.retired_variants.len(), 1);

    let variants = parse_json(&declaration("errors.kvx", "type.ErrorCode", "variants"))
        .unwrap_or_else(|error| panic!("ErrorCode variants: {error}"));
    let Json::Array(variants) = variants else {
        panic!("ErrorCode variants must be a list");
    };
    for code in ["operation-retired", "wallet-not-bound", "exit-unavailable"] {
        assert_eq!(
            variants
                .iter()
                .filter(|value| **value == Json::String(code.to_owned()))
                .count(),
            1,
            "ErrorCode must retain exactly one {code} variant"
        );
    }
    assert_eq!(
        variants.last(),
        Some(&Json::String("operation-retired".to_owned()))
    );
}

#[test]
fn compatibility_rows_cover_every_minor_through_ten() {
    real_schema_report();
    assert_eq!(declaration("v1.kvx", "schema", "major"), "1");
    assert_eq!(declaration("v1.kvx", "schema", "minor"), "10");

    let source = read_schema("compatibility.kvx");
    let rows: Vec<_> = source
        .lines()
        .filter_map(|line| {
            line.strip_prefix('[')
                .and_then(|line| line.strip_suffix(']'))
        })
        .filter(|section| section.starts_with("compatibility.row."))
        .collect();
    assert_eq!(rows.len(), 11, "one compatibility row per schema minor");
    let versions: Vec<_> = rows
        .iter()
        .map(|section| declaration("compatibility.kvx", section, "schema"))
        .collect();
    let unique: BTreeSet<_> = versions.iter().cloned().collect();
    assert_eq!(unique.len(), versions.len(), "duplicate schema minor row");
    assert_eq!(
        unique,
        (0..=10).map(|minor| format!("\"1.{minor}\"")).collect()
    );
}
