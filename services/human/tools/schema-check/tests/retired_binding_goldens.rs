use std::fs;
use std::path::{Path, PathBuf};

use layerx_human_schema_check::{parse_json, Json};

fn real_schema_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/human-api")
}

fn read_schema(file: &str) -> String {
    fs::read_to_string(real_schema_root().join(file))
        .unwrap_or_else(|error| panic!("read {file}: {error}"))
}

fn golden(file: &str) -> Json {
    parse_json(&read_schema(&format!("golden/{file}")))
        .unwrap_or_else(|error| panic!("parse {file}: {error}"))
}

fn field<'a>(value: &'a Json, name: &str) -> &'a Json {
    value
        .field(name)
        .unwrap_or_else(|| panic!("missing {name} in {value:?}"))
}

fn declared_version(key: &str) -> Json {
    let source = read_schema("v1.kvx");
    let mut lines = source
        .lines()
        .map(str::trim)
        .skip_while(|line| *line != "[schema]");
    assert_eq!(lines.next(), Some("[schema]"), "missing schema section");
    let values: Vec<_> = lines
        .take_while(|line| !line.starts_with('['))
        .filter_map(|line| line.split_once('='))
        .filter_map(|(name, value)| (name.trim() == key).then_some(value.trim()))
        .collect();
    assert_eq!(values.len(), 1, "expected exactly one schema.{key}");
    let number = values[0]
        .parse::<u64>()
        .unwrap_or_else(|error| panic!("invalid schema.{key}: {error}"));
    Json::Number(number.to_string())
}

struct VersionInfo<'a> {
    schema: &'a Json,
    service: &'a str,
}

impl<'a> VersionInfo<'a> {
    fn from_json(result: &'a Json) -> Self {
        let Json::String(service) = field(result, "service") else {
            panic!("VersionInfo.service must be a string");
        };
        Self {
            schema: field(result, "schema"),
            service,
        }
    }
}

#[test]
fn retired_binding_failure_vectors_answer_operation_retired() {
    for operation in [
        "binding.statement",
        "binding.submit",
        "binding.status",
        "binding.rebind.action",
        "binding.rebind",
    ] {
        let vector = golden(&format!("{operation}.failure.json"));
        assert_eq!(
            field(&vector, "status"),
            &Json::Number("410".to_owned()),
            "{operation}"
        );
        let body = field(&vector, "body");
        assert_eq!(field(body, "ok"), &Json::Bool(false), "{operation}");
        assert!(
            body.field("result").is_none(),
            "{operation} returned a result"
        );
        assert_eq!(
            field(field(body, "error"), "code"),
            &Json::String("operation-retired".to_owned()),
            "{operation}"
        );
    }
}

#[test]
fn version_golden_answers_the_declared_minor() {
    let vector = golden("version.response.json");
    assert_eq!(field(&vector, "status"), &Json::Number("200".to_owned()));
    let body = field(&vector, "body");
    assert_eq!(field(body, "ok"), &Json::Bool(true));
    let version = VersionInfo::from_json(field(body, "result"));
    assert!(!version.service.is_empty(), "missing service version");
    assert_eq!(declared_version("minor"), Json::Number("10".to_owned()));
    for key in ["major", "minor"] {
        assert_eq!(
            field(version.schema, key),
            &declared_version(key),
            "schema.{key}"
        );
    }
}
