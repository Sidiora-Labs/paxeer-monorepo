use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};

use layerx_human_schema_check::{
    baseline_manifest, human_api_schema, parse_json, Json, SchemaReport, BASELINE_FILE,
};

#[allow(non_upper_case_globals)]
const JourneyKind: &str = "type.JourneyKind";
#[allow(non_upper_case_globals)]
const VerificationLevel: &str = "type.VerificationLevel";
#[allow(non_upper_case_globals)]
const EvidenceRef: &str = "type.EvidenceRef";

fn real_schema_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/human-api")
}

fn declaration(section: &str, key: &str) -> Json {
    let source = fs::read_to_string(real_schema_root().join("journeys.kvx"))
        .unwrap_or_else(|error| panic!("read journeys.kvx: {error}"));
    let header = format!("[{section}]");
    let mut lines = source.lines().skip_while(|line| *line != header);
    assert_eq!(lines.next(), Some(header.as_str()), "missing {header}");
    let value = lines
        .take_while(|line| !line.starts_with('['))
        .filter_map(|line| line.split_once('='))
        .find_map(|(name, value)| (name.trim() == key).then(|| value.trim()))
        .unwrap_or_else(|| panic!("missing journeys.kvx:{section}.{key}"));
    parse_json(value).unwrap_or_else(|error| panic!("{section}.{key}: {error}"))
}

fn string_list(section: &str, key: &str) -> Vec<String> {
    let Json::Array(values) = declaration(section, key) else {
        panic!("{section}.{key} must be an array");
    };
    values
        .into_iter()
        .map(|value| match value {
            Json::String(value) => value,
            other => panic!("{section}.{key} must contain strings: {other:?}"),
        })
        .collect()
}

fn real_schema_report() -> SchemaReport {
    let root = real_schema_root();
    let report = human_api_schema(&root)
        .unwrap_or_else(|violations| panic!("real schema conformance: {violations:#?}"));
    let baseline = baseline_manifest(&root)
        .unwrap_or_else(|violations| panic!("real schema baseline: {violations:#?}"));
    assert_eq!(
        fs::read_to_string(root.join(BASELINE_FILE))
            .unwrap_or_else(|error| panic!("read baseline: {error}")),
        baseline,
        "baseline must be refreshed"
    );
    assert_eq!(report.additions_beyond_baseline, 0);
    report
}

fn assert_retired_variants(report: &SchemaReport, section: &str, expected: &[&str]) {
    let retired = string_list(section, "retired_variants");
    assert_eq!(retired, expected, "{section} retired variants");
    assert_eq!(
        report.retired_variants.get(
            section
                .strip_prefix("type.")
                .unwrap_or_else(|| panic!("expected type section: {section}"))
        ),
        Some(&retired),
        "conformance must report the real schema's retired variants"
    );
    let variants = string_list(section, "variants");
    for value in &retired {
        assert_eq!(
            variants.iter().filter(|variant| *variant == value).count(),
            1,
            "{section} must keep {value} decodable exactly once"
        );
    }
}

fn rung_mapping() -> BTreeMap<String, String> {
    let mut mapping = BTreeMap::new();
    for entry in string_list(VerificationLevel, "rung_mapping") {
        let (rung, verification) = entry
            .split_once(':')
            .unwrap_or_else(|| panic!("invalid rung mapping: {entry}"));
        assert!(
            mapping
                .insert(rung.to_owned(), verification.to_owned())
                .is_none(),
            "duplicate rung: {rung}"
        );
    }
    mapping
}

#[test]
fn journey_kind_retires_binding_deposit_withdraw_and_exit() {
    let report = real_schema_report();
    assert_retired_variants(
        &report,
        JourneyKind,
        &["wallet-binding", "deposit", "withdraw", "exit"],
    );
    assert_eq!(
        string_list(JourneyKind, "variants"),
        [
            "onboarding",
            "wallet-binding",
            "deposit",
            "withdraw",
            "exit",
            "move",
            "agent-create",
            "agent-fund",
            "agent-pause",
            "agent-retire",
        ]
    );
    let retired = string_list(JourneyKind, "retired_variants");
    let active: Vec<_> = string_list(JourneyKind, "variants")
        .into_iter()
        .filter(|kind| !retired.contains(kind))
        .collect();
    assert_eq!(
        active,
        [
            "onboarding",
            "move",
            "agent-create",
            "agent-fund",
            "agent-pause",
            "agent-retire",
        ]
    );
}

#[test]
fn verification_level_retires_settlement_values_and_maps_rungs() {
    let report = real_schema_report();
    assert_retired_variants(
        &report,
        VerificationLevel,
        &["paxeer-finalised", "settlement-anchored"],
    );
    assert_eq!(
        string_list(VerificationLevel, "variants"),
        [
            "unverified",
            "receipt-verified",
            "checkpoint-finalised",
            "paxeer-finalised",
            "settlement-anchored",
        ]
    );
    assert_eq!(
        declaration(VerificationLevel, "ordering"),
        Json::String("declaration_order".to_owned())
    );
    let mapping = rung_mapping();
    assert_eq!(
        mapping,
        BTreeMap::from([
            ("submitted".to_owned(), "unverified".to_owned()),
            ("receipted".to_owned(), "receipt-verified".to_owned()),
            ("committed".to_owned(), "checkpoint-finalised".to_owned()),
        ])
    );
    let retired = string_list(VerificationLevel, "retired_variants");
    let active: Vec<_> = string_list(VerificationLevel, "variants")
        .into_iter()
        .filter(|level| !retired.contains(level))
        .collect();
    assert_eq!(
        active,
        ["unverified", "receipt-verified", "checkpoint-finalised"]
    );
    for level in mapping.values() {
        assert!(
            active.contains(level),
            "rung maps to inactive level: {level}"
        );
    }

    assert_retired_variants(&report, "type.EvidenceClass", &["paxeer-finality"]);
    for section in [EvidenceRef, "type.EvidenceMaterial"] {
        assert_eq!(
            string_list(section, "optional"),
            ["settlement_domain:SettlementDomain"]
        );
        assert_eq!(
            declaration(section, "settlement_domain.retired"),
            Json::String("true".to_owned())
        );
        assert_eq!(
            declaration(section, "settlement_domain"),
            Json::String("paxeer".to_owned())
        );
    }
    assert_eq!(
        string_list(EvidenceRef, "required"),
        [
            "evidence_id:EvidenceId",
            "class:EvidenceClass",
            "verification:VerificationLevel"
        ]
    );
    assert_eq!(
        string_list("type.EvidenceMaterial", "required"),
        [
            "evidence_id:EvidenceId",
            "class:EvidenceClass",
            "verification:VerificationLevel",
            "content_type:string",
            "bytes_base64:string"
        ]
    );
    for (operation, path, response) in [
        (
            "evidence.get",
            "/v1/evidence/{evidence_id}",
            "EvidenceMaterial",
        ),
        ("journey.get", "/v1/journeys/{journey_id}", "Journey"),
        ("journey.list", "/v1/journeys", "JourneyPage"),
        ("journey.page", "/v1/journeys/page/{cursor}", "JourneyPage"),
    ] {
        let section = format!("operation.{operation}");
        for (key, expected) in [
            ("authorization_class", "read"),
            ("method", "GET"),
            ("path", path),
            ("request", "Empty"),
            ("response", response),
        ] {
            assert_eq!(
                declaration(&section, key),
                Json::String(expected.to_owned()),
                "{section}.{key}"
            );
        }
    }
}
