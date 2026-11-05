//! Task 22.33.1556 repair record, finding doc_crate.
//! Proofs: doc_crate_examples, doc_crate_receipt_keys, doc_crate_readiness,
//! and doc_crate_catalogue exercise the producer shape and production MCP consumers.
//! Baseline: no prior gate ledger or repair target existed; the implementation-first,
//! wrapper-only gate order prohibits an extra unchanged-tree verify. No baseline
//! failure is claimed. No out-of-path wiring is needed; hf-build/hf-done record
//! the exact qualified revision, command, exit and private evidence location.

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::os::unix::fs::{MetadataExt as _, PermissionsExt as _};
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::time::Duration;

use layerx_agentd::enrolment::binding_shape::BindingDocument;
use layerx_mcp::binding::{Binding, BindingError};
use layerx_mcp::catalogue;
use layerx_mcp::listener::Listener;
use layerx_mcp::server::{tool_operation, DeploymentMode, ToolKind};
use serde_json::{json, Value};

const README: &str = include_str!("../README.md");
static NEXT: AtomicUsize = AtomicUsize::new(0);

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "mcp-doc-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).expect("create private fixture directory");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700))
            .expect("protect fixture directory");
        Self(fs::canonicalize(path).expect("canonical fixture directory"))
    }

    fn secret(&self, name: &str, contents: &str) -> PathBuf {
        let path = self.0.join(name);
        fs::write(&path, contents).expect("write fixture credential");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o600))
            .expect("protect fixture credential");
        path
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove private fixture directory");
    }
}

fn examples() -> Vec<Value> {
    let blocks: Vec<_> = README
        .split("```json\n")
        .skip(1)
        .map(|block| {
            serde_json::from_str(block.split_once("```").expect("closed JSON fence").0)
                .expect("README JSON example")
        })
        .collect();
    assert_eq!(blocks.len(), 3, "base, optional listener and optional web");
    blocks
}

// Serialize with the exact shape the daemon publisher uses before handing the
// bytes to the MCP parser. No replacement parser or parallel binding is used.
fn produced(document: &Value) -> Binding {
    let producer: BindingDocument =
        serde_json::from_value(document.clone()).expect("current daemon producer shape");
    let bytes = serde_json::to_vec(&producer).expect("serialize daemon producer");
    assert_eq!(serde_json::from_slice::<Value>(&bytes).unwrap(), *document);
    let binding = Binding::parse(std::str::from_utf8(&bytes).expect("JSON transport"))
        .expect("production MCP binding consumer");
    assert_eq!(binding.core_height(), producer.core_height);
    assert_eq!(binding.agent_endpoint(), producer.agent.endpoint);
    assert_eq!(binding.session_generation(), producer.session_generation);
    binding
}

fn web_document() -> Value {
    let blocks = examples();
    let mut document = blocks[0].clone();
    document["web"] = blocks[2]["web"].clone();
    produced(&document);
    document
}

#[test]
fn doc_crate_examples() {
    let blocks = examples();
    let base = &blocks[0];
    let binding = produced(base);
    assert_eq!(binding.core_height(), 120);
    assert_eq!(binding.mode(), DeploymentMode::Full);
    assert_eq!(binding.deadline(), Duration::from_millis(10_000));
    assert!(binding.listener().is_none(), "stdio needs no listener");
    assert!(base.get("core_sequence").is_none());

    let mut socket_document = base.clone();
    socket_document["listener"] = blocks[1]["listener"].clone();
    let config = produced(&socket_document).listener().cloned().unwrap();
    assert_eq!(config.endpoint, PathBuf::from("/run/layerx/mcp.sock"));
    assert_eq!(config.mode, 0o660);
    assert_eq!(config.admitted_uids, vec![0]);

    // Substitute only deployment-specific ownership and path, then exercise the
    // real listener with the configuration decoded from the documented example.
    let directory = Directory::new();
    let owner = fs::metadata(&directory.0).unwrap();
    socket_document["listener"]["socket"] = json!(directory.0.join("mcp.sock"));
    socket_document["listener"]["owner_uid"] = json!(owner.uid());
    socket_document["listener"]["owner_gid"] = json!(owner.gid());
    socket_document["listener"]["admitted_uids"] = json!([owner.uid()]);
    socket_document["web"] = blocks[2]["web"].clone();
    let socket_binding = produced(&socket_document);
    let config = socket_binding.listener().unwrap().clone();
    let endpoint = config.endpoint.clone();
    let listener = Listener::bind(config).expect("documented listener binds");
    assert_eq!(fs::metadata(&endpoint).unwrap().mode() & 0o777, 0o660);
    drop(listener);
    assert!(
        !endpoint.exists(),
        "production listener cleans up its socket"
    );
}

#[test]
fn doc_crate_receipt_keys() {
    let valid = web_document();
    assert!(valid["web"].get("sequencer_public_key").is_none());
    let fields: BTreeSet<_> = valid["web"]["receipt_key_index"][0]
        .as_object()
        .unwrap()
        .keys()
        .map(String::as_str)
        .collect();
    assert_eq!(
        fields,
        BTreeSet::from(["keyId", "publicKey", "firstHeight", "lastHeight"])
    );

    for field in fields {
        let mut invalid = valid.clone();
        invalid["web"]["receipt_key_index"][0]
            .as_object_mut()
            .unwrap()
            .remove(field);
        assert!(
            Binding::parse(&invalid.to_string()).is_err(),
            "missing {field}"
        );
    }
    for index in [
        json!([]),
        json!([{"keyId": "00".repeat(32), "publicKey": "5e".repeat(32), "firstHeight": 1, "lastHeight": 1000}]),
        json!([{"keyId": "2a".repeat(32), "publicKey": "00".repeat(32), "firstHeight": 1, "lastHeight": 1000}]),
        json!([
            valid["web"]["receipt_key_index"][0],
            valid["web"]["receipt_key_index"][0]
        ]),
    ] {
        let mut invalid = valid.clone();
        invalid["web"]["receipt_key_index"] = index;
        assert!(Binding::parse(&invalid.to_string()).is_err());
    }
    let mut uncovered = valid.clone();
    uncovered["web"]["receipt_key_index"][0]["lastHeight"] = json!(119);
    assert!(Binding::parse(&uncovered.to_string())
        .unwrap_err()
        .detail()
        .contains("cover"));
    let mut retired = valid.clone();
    retired["web"]["sequencer_public_key"] = json!("5e".repeat(32));
    assert!(Binding::parse(&retired.to_string())
        .unwrap_err()
        .detail()
        .contains("retired"));
    let mut retired = valid;
    retired["core_sequence"] = retired
        .as_object_mut()
        .unwrap()
        .remove("core_height")
        .unwrap();
    assert_eq!(
        Binding::parse(&retired.to_string()).unwrap_err(),
        BindingError::Refused("mcp.binding_retired_key".to_owned())
    );
}

#[test]
fn doc_crate_readiness() {
    let directory = Directory::new();
    let mut document = examples()[0].clone();
    document["session_token_file"] = json!(directory.secret("session", &"03".repeat(32)));
    document["agent"]["readiness"]["gateway_key_file"] =
        json!(directory.secret("gateway", &format!("mcp-doc:lxp_live_{}", "ab".repeat(32))));
    let binding = produced(&document);
    // This constructs the real authenticated HTTPS transport without sending a
    // request to the illustrative origin or requiring a live gateway.
    binding
        .daemon_transport()
        .expect("documented authenticated HTTPS configuration");
    let credential = binding
        .daemon_credential()
        .expect("protected session credential");
    assert_eq!(credential.tenant().as_str(), "beta");
    assert!(binding.matches_session(&credential));

    let mut missing = document.clone();
    missing["agent"]
        .as_object_mut()
        .unwrap()
        .remove("readiness");
    assert!(produced(&missing)
        .daemon_transport()
        .err()
        .unwrap()
        .detail()
        .contains("authenticated unified gateway"));
    let mut insecure = document.clone();
    insecure["agent"]["readiness"]["endpoint"] = json!("http://127.0.0.1:8090");
    assert!(produced(&insecure).daemon_transport().is_err());
    let mut malformed = document.clone();
    malformed["agent"]["readiness"]["gateway_key_file"] =
        json!(directory.secret("invalid", "invalid"));
    assert!(produced(&malformed).daemon_transport().is_err());
    let token_path = PathBuf::from(document["session_token_file"].as_str().unwrap());
    fs::set_permissions(token_path, fs::Permissions::from_mode(0o644)).unwrap();
    assert!(
        binding.daemon_transport().is_err(),
        "world-readable credentials are refused"
    );
}

#[test]
fn doc_crate_catalogue() {
    let rows: BTreeMap<_, _> = README
        .lines()
        .filter(|line| line.starts_with("| `"))
        .map(|line| {
            let cells: Vec<_> = line
                .trim_matches('|')
                .split('|')
                .map(|cell| cell.trim().trim_matches('`'))
                .collect();
            assert_eq!(cells.len(), 4);
            (cells[0], (cells[1], cells[2], cells[3]))
        })
        .collect();
    let tools = catalogue::surface(DeploymentMode::Full)
        .into_iter()
        .chain(catalogue::web_surface())
        .collect::<Vec<_>>();
    assert_eq!(rows.len(), tools.len());
    assert_eq!(tools.len(), 36);
    for tool in tools {
        let kind = if tool.kind == ToolKind::Read {
            "read"
        } else {
            "write"
        };
        let operation = tool_operation(tool.name).map_or_else(
            || "Dedicated faucet route".to_owned(),
            |operation| format!("{operation:?}"),
        );
        assert_eq!(
            rows.get(tool.name),
            Some(&(kind, tool.required_scope, operation.as_str())),
            "{}",
            tool.name
        );
        let listing = catalogue::listing(tool).expect("production tools/list encoder");
        let wire = serde_json::to_vec(&listing).unwrap();
        let received: Value = serde_json::from_slice(&wire).unwrap();
        assert_eq!(received["name"], tool.name);
        assert_eq!(received["_meta"]["layerx/scope"], tool.required_scope);
        assert_eq!(
            received["annotations"]["readOnlyHint"],
            tool.kind == ToolKind::Read
        );
        if tool.name.starts_with("web.") {
            assert_eq!(received["_meta"]["layerx/output"], "untrusted");
        }
    }
    let expected_reads: BTreeSet<_> = rows
        .iter()
        .filter(|(_, (kind, _, _))| *kind == "read")
        .map(|(name, _)| *name)
        .collect();
    let actual_reads: BTreeSet<_> = catalogue::surface(DeploymentMode::ReadOnly)
        .into_iter()
        .map(|tool| tool.name)
        .collect();
    assert_eq!(actual_reads, expected_reads);
}
