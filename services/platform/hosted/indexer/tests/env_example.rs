use std::collections::BTreeMap;

use layerx_indexer::config::BackfillConfig;

fn example_environment() -> BTreeMap<&'static str, &'static str> {
    include_str!("../railway.env.example")
        .lines()
        .map(str::trim)
        .filter(|line| !line.is_empty() && !line.starts_with('#'))
        .map(|line| line.split_once('=').expect("example must use NAME=value"))
        .collect()
}

fn filled_lookup(environment: &BTreeMap<&str, &str>, name: &str) -> Option<String> {
    environment.get(name).map(|value| {
        if value.is_empty() {
            match name {
                "PAXSCAN_DATABASE_PUBLIC_URL" => {
                    "postgres://readonly@localhost:5432/paxscan".to_owned()
                }
                "LAYERX_INDEXER_PAXSCAN_CERT_SHA256" => "ab".repeat(32),
                _ => (*value).to_owned(),
            }
        } else {
            (*value).to_owned()
        }
    })
}

#[test]
fn example_names_every_backfill_variable() {
    let environment = example_environment();
    assert_eq!(environment.get("PAXSCAN_DATABASE_PUBLIC_URL"), Some(&""));
    assert_eq!(
        environment.get("LAYERX_INDEXER_PAXSCAN_CERT_SHA256"),
        Some(&"")
    );

    let result = BackfillConfig::from_args(&["--cutover-height=1".to_owned()], &|name| {
        filled_lookup(&environment, name)
    });
    assert!(result.is_ok(), "example backfill configuration: {result:?}");
}

#[test]
fn example_without_the_pin_is_refused() {
    let mut environment = example_environment();
    environment.remove("LAYERX_INDEXER_PAXSCAN_CERT_SHA256");

    let result = BackfillConfig::from_args(&["--cutover-height=1".to_owned()], &|name| {
        filled_lookup(&environment, name)
    });
    assert!(result.is_err(), "backfill must refuse a missing pin");
}
