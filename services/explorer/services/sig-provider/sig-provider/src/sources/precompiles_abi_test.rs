use super::PrecompileSource;
use std::{
    collections::{BTreeSet, HashMap},
    fs::File,
    path::Path,
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
    let precompiles = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../../precompiles");
    let mut expected_functions = BTreeSet::new();
    let mut expected_events = BTreeSet::new();

    for relative_path in [
        "addr/abi.json",
        "layerxexchange/abi.json",
        "layerxbridge/abi.json",
        "launchpad/abi.json",
    ] {
        let path = precompiles.join(relative_path);
        let file = File::open(&path)
            .unwrap_or_else(|error| panic!("cannot read {}: {error}", path.display()));
        let abi = ethabi::Contract::load(file)
            .unwrap_or_else(|error| panic!("invalid ABI {}: {error}", path.display()));

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
    }

    let embedded = PrecompileSource::signatures();
    assert_eq!(
        table_entries(&embedded.functions),
        expected_functions,
        "embedded function signatures must equal the union of the four precompile ABIs"
    );
    assert_eq!(
        table_entries(&embedded.events),
        expected_events,
        "embedded event signatures must equal the union of the four precompile ABIs"
    );
}
