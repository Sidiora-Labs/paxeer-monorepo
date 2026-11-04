use super::{EmbeddedPrecompiles, PrecompileSource};
use std::{
    collections::{BTreeSet, HashMap},
    path::Path,
    process::Command,
};

fn canonical_signature(name: &str, inputs: impl Iterator<Item = String>) -> String {
    format!("{name}({})", inputs.collect::<Vec<_>>().join(","))
}

fn table_entries(table: &HashMap<String, Vec<String>>) -> BTreeSet<(String, String)> {
    table
        .iter()
        .flat_map(|(selector, signatures)| {
            signatures
                .iter()
                .map(move |signature| (selector.clone(), signature.clone()))
        })
        .collect()
}

#[test]
fn embedded_table_matches_abi_files() {
    let generator =
        Path::new(env!("CARGO_MANIFEST_DIR")).join("src/sources/generate_precompiles.py");
    let generated = Command::new("python3")
        .arg(generator)
        .arg("--stdout")
        .output()
        .unwrap();
    assert!(
        generated.status.success(),
        "{}",
        String::from_utf8_lossy(&generated.stderr)
    );
    let expected: serde_json::Value = serde_json::from_slice(&generated.stdout).unwrap();
    let actual: serde_json::Value = serde_json::from_str(include_str!("precompiles.json")).unwrap();
    assert_eq!(actual, expected, "embedded inventory and ABI must match the node's versioned registry and historical snapshot");
    let embedded: EmbeddedPrecompiles = serde_json::from_value(expected).unwrap();
    let abi = embedded.into_contract();
    let mut expected_functions = BTreeSet::new();
    let mut expected_events = BTreeSet::new();
    for function in abi.functions() {
        expected_functions.insert((
            hex::encode(function.short_signature()),
            canonical_signature(
                &function.name,
                function.inputs.iter().map(|input| input.kind.to_string()),
            ),
        ));
    }
    for event in abi.events() {
        expected_events.insert((
            hex::encode(event.signature()),
            canonical_signature(
                &event.name,
                event.inputs.iter().map(|input| input.kind.to_string()),
            ),
        ));
    }
    let embedded = PrecompileSource::signatures();
    assert_eq!(
        table_entries(&embedded.functions),
        expected_functions,
        "embedded functions must equal the versioned and historical ABI union"
    );
    assert_eq!(
        table_entries(&embedded.events),
        expected_events,
        "embedded events must equal the versioned and historical ABI union"
    );
}
