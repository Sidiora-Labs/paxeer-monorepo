use layerx_platform_gateway::http::Endpoint;
use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::net::SocketAddr;
use std::path::Path;

const ROUTER_EXAMPLE: &str = include_str!("../railway.env.example");
const FILE_VARIABLES: &str = "FILE_VARIABLES = (";

fn configured(example: &str) -> BTreeMap<&str, &str> {
    let mut variables = BTreeMap::new();
    for line in example.lines().map(str::trim) {
        if line.is_empty() || line.starts_with('#') {
            continue;
        }
        let (name, value) = line
            .split_once('=')
            .unwrap_or_else(|| panic!("expected NAME=value: {line}"));
        assert!(!name.is_empty(), "environment variable must have a name");
        assert!(
            variables.insert(name, value).is_none(),
            "duplicate environment variable: {name}"
        );
    }
    variables
}

fn configured_endpoint(variables: &BTreeMap<&str, &str>, name: &str) -> Endpoint {
    Endpoint::parse(variables[name])
        .unwrap_or_else(|error| panic!("{name} must be a valid HTTPS endpoint: {error}"))
}

#[test]
fn router_example_drops_unread_sequencer_variables() {
    let variables = configured(ROUTER_EXAMPLE);
    for name in variables.keys() {
        assert!(
            !name.starts_with("LAYERX_GATEWAY_SEQUENCER_")
                && !name.starts_with("ENDPOINT_SEQUENCER_"),
            "unread sequencer variable: {name}"
        );
    }
    for name in [
        "LAYERX_GATEWAY_FAUCET_URL",
        "LAYERX_GATEWAY_FAUCET_SERVICE_TOKEN_FILE",
        "LAYERX_GATEWAY_FAUCET_CLAIM_AMOUNT",
        "LAYERX_GATEWAY_RECEIPT_KEYS_FILE",
    ] {
        assert!(!variables.contains_key(name), "{name} must remain optional");
        assert!(
            ROUTER_EXAMPLE
                .lines()
                .any(|line| line.starts_with(&format!("# {name}="))),
            "optional variable must be documented: {name}"
        );
    }
}

#[test]
fn router_example_names_the_indexer_listener() {
    let variables = configured(ROUTER_EXAMPLE);
    let endpoint = configured_endpoint(&variables, "LAYERX_GATEWAY_INDEXER_URL");
    let indexer_example = fs::read_to_string(
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../indexer/railway.env.example"),
    )
    .unwrap_or_else(|error| panic!("indexer example must be readable: {error}"));
    let indexer = configured(&indexer_example);
    let listener = indexer["LAYERX_INDEXER_LISTEN"]
        .parse::<SocketAddr>()
        .unwrap_or_else(|error| panic!("indexer listener must be a socket address: {error}"));
    assert_eq!(endpoint.host, "indexer.railway.internal");
    assert_eq!(endpoint.port, listener.port());
    assert!(endpoint.base_path.is_empty());
}

#[test]
fn router_example_file_secrets_have_table_rows() {
    let variables = configured(ROUTER_EXAMPLE);
    let table = fs::read_to_string(
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../docker/platform-gateway/files.tsv"),
    )
    .unwrap_or_else(|error| panic!("gateway file-secret table must be readable: {error}"));
    let mut endpoints = BTreeSet::new();
    let mut guest_paths = BTreeSet::new();
    for line in table.lines().filter(|line| !line.trim().is_empty()) {
        let fields: Vec<_> = line.split('\t').collect();
        assert_eq!(fields.len(), 4, "expected four file-secret columns: {line}");
        assert!(fields[0].starts_with("ENDPOINT_"));
        assert!(!fields[0].starts_with("ENDPOINT_SEQUENCER_"));
        assert!(fields[1].starts_with("/run/secrets/"));
        endpoints.insert(fields[0]);
        guest_paths.insert(fields[1]);
    }
    assert!(!endpoints.is_empty(), "file-secret table must not be empty");
    for (name, value) in &variables {
        if name.starts_with("ENDPOINT_") {
            assert!(endpoints.contains(name), "missing file-secret row: {name}");
        }
        if value.starts_with("/run/secrets/") {
            assert!(guest_paths.contains(value), "missing guest path: {value}");
        }
    }
    for kind in ["PAYMENT", "WEBHOOKS"] {
        for suffix in [
            "URL",
            "CA_DER",
            "TOKEN_FILE",
            "CLIENT_IDENTITY_PKCS12",
            "CLIENT_IDENTITY_PASSWORD_FILE",
        ] {
            let name = format!("LAYERX_EVENTS_{kind}_UPSTREAM_{suffix}");
            assert!(variables.contains_key(name.as_str()), "missing {name}");
        }
        let name = format!("ENDPOINT_EVENTS_{kind}_TOKEN");
        assert!(variables.contains_key(name.as_str()), "missing {name}");
    }
}

#[test]
fn native_file_rows_match_general_paths_for_shared_names_and_bindings_mode() {
    let container =
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../docker/platform-gateway");
    let general = fs::read_to_string(container.join("files.tsv"))
        .unwrap_or_else(|error| panic!("gateway file-secret table must be readable: {error}"));
    let native = fs::read_to_string(container.join("native-files.tsv")).unwrap_or_else(|error| {
        panic!("native gateway file-secret table must be readable: {error}")
    });
    let shim = fs::read_to_string(container.join("../common/env-files.sh"))
        .unwrap_or_else(|error| panic!("file staging shim must be readable: {error}"));
    assert!(
        shim.contains("mode=${mode:-0600}"),
        "empty file modes must default to 0600"
    );
    let rows = |table: &str| {
        table
            .lines()
            .filter(|line| !line.trim().is_empty() && !line.starts_with('#'))
            .map(|line| {
                let fields: Vec<_> = line.split('\t').collect();
                assert_eq!(fields.len(), 4, "expected four file-secret columns: {line}");
                let mode = (!fields[2].is_empty()).then(|| {
                    u32::from_str_radix(fields[2], 8)
                        .unwrap_or_else(|error| panic!("invalid file mode in {line}: {error}"))
                });
                (fields[0].to_owned(), fields[1].to_owned(), mode)
            })
            .collect::<Vec<_>>()
    };
    let general_rows = rows(&general);
    let native_rows = rows(&native);
    assert!(
        !general_rows.is_empty(),
        "general file-secret table must not be empty"
    );
    assert!(
        !native_rows.is_empty(),
        "native file-secret table must not be empty"
    );
    let general_names = general_rows
        .iter()
        .map(|(name, _, _)| name.as_str())
        .collect::<BTreeSet<_>>();
    for (name, path, mode) in native_rows {
        if general_names.contains(name.as_str()) {
            assert!(
                general_rows
                    .iter()
                    .any(|(general_name, general_path, _)| general_name == &name && general_path == &path),
                "native file-secret row has no general row with the same name and path: {name:?} {path:?}"
            );
        }
        if name == "ENDPOINT_ROUTE_BINDINGS" {
            let general_mode = general_rows
                .iter()
                .find(|(general_name, general_path, _)| {
                    general_name == &name && general_path == &path
                })
                .and_then(|(_, _, mode)| *mode);
            assert_eq!(
                mode, general_mode,
                "route-bindings rows must use the same mode in both file tables"
            );
        }
    }
}

#[test]
fn router_entrypoint_stages_every_table_backed_file_variable() {
    let entrypoint = fs::read_to_string(
        Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("../../../../docker/platform-gateway/entrypoint.sh"),
    )
    .unwrap_or_else(|error| panic!("gateway entrypoint must be readable: {error}"));
    let file_variables = entrypoint
        .split_once(FILE_VARIABLES)
        .unwrap_or_else(|| panic!("{}", "gateway entrypoint must declare FILE_VARIABLES"))
        .1
        .split_once(')')
        .unwrap_or_else(|| panic!("{}", "FILE_VARIABLES tuple must be closed"))
        .0
        .lines()
        .map(str::trim)
        .filter(|line| !line.is_empty())
        .map(|line| {
            line.trim_end_matches(',')
                .strip_prefix('"')
                .and_then(|name| name.strip_suffix('"'))
                .unwrap_or_else(|| panic!("expected double-quoted file variable: {line}"))
        })
        .collect::<BTreeSet<_>>();
    let bindings_variable = entrypoint
        .lines()
        .find_map(|line| line.strip_prefix("BINDINGS_VARIABLE = "))
        .and_then(|value| value.strip_prefix('"'))
        .and_then(|value| value.strip_suffix('"'))
        .unwrap_or_else(|| {
            panic!(
                "{}",
                "gateway entrypoint must declare a double-quoted BINDINGS_VARIABLE"
            )
        });
    assert!(
        file_variables.contains("LAYERX_GATEWAY_UI_BINDINGS_FILE"),
        "LAYERX_GATEWAY_UI_BINDINGS_FILE must be staged by FILE_VARIABLES"
    );

    let table = fs::read_to_string(
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../../docker/platform-gateway/files.tsv"),
    )
    .unwrap_or_else(|error| panic!("gateway file-secret table must be readable: {error}"));
    let mut guest_paths = BTreeSet::new();
    for line in table.lines().filter(|line| !line.trim().is_empty()) {
        let fields: Vec<_> = line.split('\t').collect();
        assert_eq!(fields.len(), 4, "expected four file-secret columns: {line}");
        assert!(fields[1].starts_with("/run/secrets/"));
        guest_paths.insert(fields[1]);
    }
    assert!(
        !guest_paths.is_empty(),
        "file-secret table must not be empty"
    );

    let variables = configured(ROUTER_EXAMPLE);
    let optional_variables = ROUTER_EXAMPLE.lines().map(str::trim).filter_map(|line| {
        line.strip_prefix("# ")
            .and_then(|line| line.split_once('='))
    });
    for (name, value) in variables.into_iter().chain(optional_variables) {
        if guest_paths.contains(value) {
            assert!(
                file_variables.contains(name) || name == bindings_variable,
                "table-backed file variable must be staged by the gateway entrypoint: {name}"
            );
        }
    }
}
