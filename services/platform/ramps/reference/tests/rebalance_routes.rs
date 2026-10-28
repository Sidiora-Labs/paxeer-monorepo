use std::fs;
use std::net::SocketAddr;
use std::os::unix::fs::PermissionsExt as _;
use std::path::PathBuf;
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

use serde_json::{json, Value};

struct Workspace(PathBuf);

impl Workspace {
    fn new() -> Self {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("clock")
            .as_nanos();
        let workspace = Self(std::env::temp_dir().join(format!(
            "layerx-ramp-rebalance-{}-{nanos}",
            std::process::id()
        )));
        fs::create_dir(&workspace.0).expect("create workspace");
        fs::set_permissions(&workspace.0, fs::Permissions::from_mode(0o700))
            .expect("private workspace");
        for args in [
            vec![
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-days",
                "2",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost,IP:127.0.0.1",
                "-keyout",
                "key.pem",
                "-out",
                "cert.pem",
            ],
            vec![
                "pkcs12",
                "-export",
                "-inkey",
                "key.pem",
                "-in",
                "cert.pem",
                "-passout",
                "pass:ramp-test-password",
                "-out",
                "identity.p12",
            ],
            vec![
                "x509",
                "-in",
                "cert.pem",
                "-outform",
                "DER",
                "-out",
                "anchor.der",
            ],
        ] {
            let output = Command::new("openssl")
                .args(args)
                .current_dir(&workspace.0)
                .output()
                .expect("generate client TLS material");
            assert!(
                output.status.success(),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
        }
        fs::set_permissions(
            workspace.path("identity.p12"),
            fs::Permissions::from_mode(0o600),
        )
        .expect("private TLS identity");
        workspace.private_file("password", b"ramp-test-password\n");
        workspace.private_file("service-token", b"work-route-service-token\n");
        workspace.private_file("operator-token", b"work-route-operator-token\n");
        workspace
    }

    fn path(&self, name: &str) -> PathBuf {
        self.0.join(name)
    }

    fn private_file(&self, name: &str, bytes: &[u8]) -> PathBuf {
        let path = self.path(name);
        fs::write(&path, bytes).expect("write protected file");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o600)).expect("protect file");
        path
    }

    fn config(&self, address: SocketAddr) -> Value {
        let p = |name: &str| self.path(name);
        let did = "did:layerx:ramp-work-test";
        let account = format!("agent:{did}:main");
        let config = json!({
            "listen": address.to_string(),
            "listener": "plain",
            "journal_path": p("journal.jsonl"),
            "worker_id": "ramp-work-worker",
            "lease_seconds": 60,
            "reconcile_seconds": 5,
            "operator": {
                "principal_id": "ramp-work-operator",
                "account": account,
                "signer_key_handle": "kms://ramp-work/layerx"
            },
            "quotes": [{
                "quote_id": "work-quote",
                "direction": "on_ramp",
                "layerx_asset": ([1_u8; 32]),
                "layerx_amount": 1_000_000,
                "external_currency": "EUR",
                "external_amount_minor": 100,
                "rate_numerator": 1_000_000,
                "rate_denominator": 100,
                "fee_minor": 2,
                "maximum_slippage_bps": 25,
                "context": ([8_u8; 32]),
                "provider_token": "provider-product",
                "payout_token": "beneficiary-token",
                "expires_at": 4_000_000_000_u64
            }],
            "client_tls": {
                "ca_pem": p("cert.pem"),
                "identity_pkcs12": p("identity.p12"),
                "identity_password_file": p("password"),
                "timeout_seconds": 8
            },
            "identity": {
                "endpoint": "https://identity.example.test",
                "service_token_file": p("service-token"),
                "audience": "layerx-ramp"
            },
            "compliance": {
                "endpoint": "https://compliance.example.test",
                "service_token_file": p("service-token"),
                "public_key": "22".repeat(32)
            },
            "provider": {
                "endpoint": "https://provider.example.test",
                "credential_file": p("service-token"),
                "settlement_path": "/layerx-ramp-v1/settlements",
                "status_path": "/layerx-ramp-v1/settlements"
            },
            "layerx": {
                "rpc_endpoint": "https://rpc.example.test:8545",
                "rpc_trust_anchor_der": p("anchor.der"),
                "chain_id": 125,
                "operator_address": format!("0x{}", "12".repeat(20)),
                "receipt_keys": [{
                    "keyId": "01".repeat(32),
                    "publicKey": "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
                    "firstHeight": 1,
                    "lastHeight": 10
                }],
                "max_fee_per_gas": 1000,
                "max_priority_fee_per_gas": 2,
                "gas_limit_cap": 100000,
                "signer_endpoint": "https://signer.example.test",
                "signer_token_file": p("service-token"),
                "actor_did": did
            },
            "paxeer": {
                "custody_endpoint": "https://custody.example.test",
                "custody_credential_file": p("service-token"),
                "broadcast_path": "/layerx-paxeer-v1/rebalances",
                "status_path": "/layerx-paxeer-v1/rebalances",
                "operator_account": account,
                "wallet_address": "operator-wallet",
                "vault_id": "operator-vault",
                "signer_key_handle": "kms://ramp-work/paxeer",
                "rpc_endpoints": ["https://rpc-a.example.test:8545", "https://rpc-b.example.test:8545"],
                "rpc_trust_anchor_der": p("anchor.der"),
                "rpc_chain_id": 229,
                "rpc_minimum_agreement": 2,
                "required_confirmations": 12,
                "poll_cadence_seconds": 5,
                "delayed_after_polls": 12
            },
            "provider_callback_public_key": "44".repeat(32),
            "operator_control_token_file": p("operator-token")
        });
        config
    }
}

impl Drop for Workspace {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[path = "../src/main.rs"]
mod reference;

use layerx_ramp_toolkit::journal::PaxeerObservation;
use layerx_types::rung::Rung;

#[test]
fn rebalance_presentation_reports_rung() {
    for rung in [Rung::Submitted, Rung::Receipted, Rung::Committed] {
        let workspace = Workspace::new();
        let config = workspace.config("127.0.0.1:0".parse().expect("listener address"));
        let key = [7; 32];
        let transaction = [9; 32];
        let committed = rung == Rung::Committed;
        let (status, body) = reference::rebalance_route_test_response(
            config,
            |journal| {
                journal
                    .plan_paxeer(key, [1; 32], 100, 1)
                    .expect("plan custody transfer");
                journal
                    .observe_paxeer(
                        key,
                        PaxeerObservation {
                            operation_id: "custody-operation",
                            transaction_hash: transaction,
                            rung,
                            block_hash: committed.then_some([8; 32]),
                            block_height: committed.then_some(42),
                        },
                        2,
                    )
                    .expect("record custody observation");
            },
            &format!("/internal/v1/rebalances/{}", "07".repeat(32)),
        );
        assert_eq!(status, 200, "{body}");
        assert_eq!(body["rung"], rung.name());
        assert_eq!(body["status"], rung.name());
        assert_eq!(body["transaction_hash"], format!("0x{}", "09".repeat(32)));
        assert!(body
            .as_object()
            .expect("rebalance object")
            .contains_key("block_height"));
        assert_eq!(body["block_height"], json!(committed.then_some(42)));
        assert_eq!(
            body["block_hash"],
            json!(committed.then(|| format!("0x{}", "08".repeat(32))))
        );
        assert!(body.get("confirmations").is_none(), "{body}");
    }
}

#[test]
fn readiness_reports_sequencer_chain_id() {
    let workspace = Workspace::new();
    let config = workspace.config("127.0.0.1:0".parse().expect("listener address"));
    assert_ne!(
        config["layerx"]["chain_id"],
        config["paxeer"]["rpc_chain_id"]
    );
    let (status, body) = reference::rebalance_route_test_response(config, |_| {}, "/readyz");
    assert_eq!(status, 200, "{body}");
    assert_eq!(body["ready"], true);
    assert_eq!(body["chain_id"], 125);
    for field in [
        "agreement",
        "endpoint_agreement",
        "minimum_agreement",
        "rpc_minimum_agreement",
        "endpoints",
        "chain_signal",
        "endpoint_signal",
        "confirmations",
    ] {
        assert!(body.get(field).is_none(), "retired field {field}: {body}");
    }
}
