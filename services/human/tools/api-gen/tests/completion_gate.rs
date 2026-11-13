use std::collections::{BTreeMap, BTreeSet};
use std::error::Error;
use std::fs;
use std::io::ErrorKind;
use std::path::Path;

type Sections = BTreeMap<String, BTreeMap<String, String>>;

const WAVE_SIX_TASKS: [&str; 13] = [
    "10.1.2", "10.1.3", "10.1.4", "10.10.1", "10.10.2", "10.14.1", "10.16.2", "10.21.4", "10.21.5",
    "10.21.6", "10.24.3", "10.25.4", "10.97.1",
];

const RETIRED_EVIDENCE_TESTS: [&str; 2] = [
    "services/human/crates/layerx-human-test-support/tests/evidence_node_frames.rs",
    "services/human/crates/layerx-human-test-support/tests/evidence_node_transactions.rs",
];

fn sections(source: &str, prefix: &str) -> Sections {
    let mut result = Sections::new();
    let mut current = None;
    for line in source.lines().map(str::trim) {
        if let Some(name) = line
            .strip_prefix('[')
            .and_then(|name| name.strip_suffix(']'))
        {
            current = name.starts_with(prefix).then(|| name.to_owned());
            if let Some(name) = &current {
                assert!(result.insert(name.clone(), BTreeMap::new()).is_none());
            }
        } else if let (Some(name), Some((key, value))) = (&current, line.split_once('=')) {
            if matches!(
                key.trim(),
                "interfaces"
                    | "planned_paths"
                    | "status"
                    | "kind"
                    | "path"
                    | "signature"
                    | "retired"
                    | "method"
            ) {
                assert!(result
                    .get_mut(name)
                    .unwrap_or_else(|| panic!("missing section {name}"))
                    .insert(key.trim().to_owned(), value.trim().to_owned())
                    .is_none());
            }
        }
    }
    result
}

fn field<'a>(section: &'a BTreeMap<String, String>, key: &str) -> &'a str {
    section
        .get(key)
        .unwrap_or_else(|| panic!("inventory record lacks {key}"))
}

fn quoted(value: &str) -> String {
    value
        .strip_prefix('"')
        .and_then(|value| value.strip_suffix('"'))
        .unwrap_or_else(|| panic!("invalid KVX string: {value}"))
        .replace("\\\"", "\"")
}

fn list(value: &str) -> Vec<String> {
    let body = value
        .strip_prefix('[')
        .and_then(|value| value.strip_suffix(']'))
        .unwrap_or_else(|| panic!("invalid KVX list: {value}"));
    if body.trim().is_empty() {
        return Vec::new();
    }
    body.split(',').map(|value| quoted(value.trim())).collect()
}

fn compact(source: &str) -> String {
    source.chars().filter(|c| !c.is_whitespace()).collect()
}

fn has_identifier(source: &str, identifier: &str) -> bool {
    source
        .split(|c: char| !c.is_alphanumeric() && c != '_')
        .any(|token| token == identifier)
}

fn retired_surface_present(kind: &str, signature: &str, source: &str) -> bool {
    match kind {
        "env_var" => {
            let (name, _) = signature
                .split_once('=')
                .unwrap_or_else(|| panic!("invalid environment signature: {signature}"));
            let name = name.trim();
            let suffix = name
                .strip_prefix("LAYERX_HUMAN_MOVEMENT_PROVIDER_")
                .unwrap_or_else(|| panic!("unexpected retired environment variable: {name}"));
            let source = compact(source);
            [
                "std::env::var",
                "std::env::var_os",
                "env::var",
                "env::var_os",
                "required",
                "bounded",
                "path",
            ]
            .iter()
            .any(|reader| {
                [name, suffix]
                    .iter()
                    .any(|name| source.contains(&format!("{reader}(\"{name}\"")))
            })
        }
        "cli_command" | "file_format" => true,
        "rust_pub_item" => {
            if signature.starts_with("pub use layerx_wire::activity::") {
                return compact(source).contains("layerx_wire::activity")
                    || [
                        "decode_signed_activity",
                        "decode_unsigned_activity",
                        "signed_envelope_bytes",
                        "unsigned_activity_bytes",
                        "unsigned_envelope_bytes",
                        "Activity",
                    ]
                    .iter()
                    .any(|identifier| has_identifier(source, identifier));
            }
            if signature.starts_with("pub use credit::") {
                return has_identifier(source, "credit_receipt")
                    || has_identifier(source, "CreditReceiptFields");
            }
            let identifier = ["pub struct ", "pub enum ", "pub fn ", "pub mod "]
                .iter()
                .find_map(|prefix| signature.strip_prefix(prefix))
                .and_then(|tail| {
                    tail.split(|c: char| !c.is_alphanumeric() && c != '_')
                        .next()
                })
                .filter(|identifier| !identifier.is_empty())
                .unwrap_or_else(|| panic!("unsupported retirement signature: {signature}"));
            has_identifier(source, identifier)
        }
        _ => panic!("unsupported retired interface kind: {kind}"),
    }
}

fn schema_route_absent(source: &str, signature: &str, production: &str) -> bool {
    let (method, path) = signature
        .split_once(' ')
        .unwrap_or_else(|| panic!("invalid route signature: {signature}"));
    let operations = sections(source, "operation.");
    let matching: Vec<_> = operations
        .iter()
        .filter(|(_, operation)| {
            quoted(field(operation, "method")) == method && quoted(field(operation, "path")) == path
        })
        .collect();
    assert_eq!(
        matching.len(),
        1,
        "missing or duplicate schema route: {signature}"
    );
    let (section, operation) = matching[0];
    let name = section
        .strip_prefix("operation.")
        .unwrap_or_else(|| panic!("invalid operation section: {section}"));
    operation
        .get("retired")
        .is_some_and(|value| quoted(value) == "true")
        && !production.contains(&format!("\"{name}\""))
        && !production.contains(&format!("\"{path}\""))
}

#[test]
fn human_completion_retired_surfaces_absent() -> Result<(), Box<dyn Error>> {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .ok_or("api-gen manifest is outside the repository layout")?;
    let tasks = sections(
        &fs::read_to_string(root.join("spec/layerx-merge/spec.kvx"))?,
        "task.10.",
    );
    let inventory = sections(
        &fs::read_to_string(root.join("spec/layerx-merge/interfaces.kvx"))?,
        "interface.10.",
    );
    let mut interfaces = BTreeSet::new();
    let mut planned_paths = BTreeSet::new();
    for id in WAVE_SIX_TASKS {
        let name = format!("task.{id}");
        let task = tasks.get(&name).unwrap_or_else(|| panic!("missing {name}"));
        interfaces.extend(list(field(task, "interfaces")));
        planned_paths.extend(list(field(task, "planned_paths")));
    }
    assert_eq!(planned_paths.len(), 12, "incomplete planned-path inventory");
    assert!(
        planned_paths.remove(RETIRED_EVIDENCE_TESTS[1]),
        "retired transactions evidence test is missing from the planned-path inventory"
    );
    assert_eq!(
        planned_paths.len(),
        11,
        "incomplete required-path inventory"
    );
    let retired: BTreeSet<_> = interfaces
        .into_iter()
        .filter(|id| {
            let interface = inventory.get(id).unwrap_or_else(|| panic!("missing {id}"));
            matches!(
                quoted(field(interface, "status")).as_str(),
                "retire" | "retired"
            )
        })
        .collect();
    assert_eq!(retired.len(), 52, "incomplete retired-interface inventory");
    let schema_routes: BTreeSet<_> = inventory
        .iter()
        .filter(|(_, interface)| {
            matches!(
                quoted(field(interface, "status")).as_str(),
                "retire" | "retired"
            ) && quoted(field(interface, "kind")) == "http_route"
                && quoted(field(interface, "path")).starts_with("services/human/schema/human-api/")
        })
        .map(|(id, _)| id.clone())
        .collect();
    assert_eq!(
        schema_routes.len(),
        11,
        "incomplete retired-schema-route inventory"
    );
    let production = fs::read_to_string(
        root.join("services/human/crates/layerx-human-service/src/server/production_components.rs"),
    )?;
    let mut failures = Vec::new();
    let mut sources = BTreeMap::new();
    for id in retired.union(&schema_routes) {
        let interface = &inventory[id];
        let relative = quoted(field(interface, "path"));
        let path = root.join(&relative);
        let kind = quoted(field(interface, "kind"));
        match fs::symlink_metadata(&path) {
            Err(error) if error.kind() == ErrorKind::NotFound && kind != "http_route" => continue,
            Err(error) => return Err(format!("cannot inspect {id} at {relative}: {error}").into()),
            Ok(_) => {}
        }
        let source = sources
            .entry(relative.clone())
            .or_insert(fs::read_to_string(&path)?);
        let signature = quoted(field(interface, "signature"));
        let present = if kind == "http_route" {
            !schema_route_absent(source, &signature, &production)
        } else {
            retired_surface_present(&kind, &signature, source)
        };
        if present {
            failures.push(format!("retired surface {id} remains at {relative}"));
        }
    }
    for relative in planned_paths {
        match fs::metadata(root.join(&relative)) {
            Ok(metadata) if metadata.is_file() => {}
            Ok(_) => failures.push(format!("planned path is not a regular file: {relative}")),
            Err(error) => failures.push(format!("missing planned path {relative}: {error}")),
        }
    }
    for relative in RETIRED_EVIDENCE_TESTS {
        match fs::symlink_metadata(root.join(relative)) {
            Err(error) if error.kind() == ErrorKind::NotFound => {}
            Err(error) => {
                return Err(
                    format!("cannot inspect retired evidence test {relative}: {error}").into(),
                )
            }
            Ok(_) => failures.push(format!("retired evidence test remains at {relative}")),
        }
    }
    assert!(
        failures.is_empty(),
        "domain 10 completion blocked:\n{}",
        failures.join("\n")
    );
    Ok(())
}
