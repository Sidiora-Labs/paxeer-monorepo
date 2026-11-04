use super::PrecompileSource;
#[path = "abi_inventory_test_support.rs"]
mod inventory;
use std::collections::{BTreeSet, HashMap};

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
    let expected = inventory::declared_abi_inventory();
    let abi = ethabi::Contract::load(serde_json::to_vec(&expected).unwrap().as_slice()).unwrap();
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
