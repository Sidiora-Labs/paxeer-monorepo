use std::collections::BTreeMap;
use std::fmt::Debug;
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::process::Command;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use layerx_agentd::boot::inference_market::{
    observe_availability, serve_health, serve_inference_market,
};
use layerx_agentd::boot::{InferenceBootError, InferenceBootGate};
use layerx_agentd::config::{agent_mode::Mode, InferenceConfig};
use layerx_agentd::degraded::{Mode as DegradedMode, OperationError, Reference};
use layerx_types::rung::Rung;
use serde_json::{json, Value};

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("unexpected error: {error:?}"))
}

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let root = std::env::temp_dir().join(format!(
            "inference-profile-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::create_dir(&root));
        must(fs::write(root.join("room.json"), b"{}"));
        Self(root)
    }

    fn values(&self) -> BTreeMap<String, String> {
        BTreeMap::from([
            (
                "LAYERX_AGENT_INFERENCE_MODEL_DIGEST".to_owned(),
                "11".repeat(32),
            ),
            (
                "LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST".to_owned(),
                "22".repeat(32),
            ),
            (
                "LAYERX_AGENT_INFERENCE_ROOM_PROFILE".to_owned(),
                self.0.join("room.json").to_string_lossy().into_owned(),
            ),
        ])
    }

    fn config(&self) -> InferenceConfig {
        let mut values = self.values();
        for key in [
            "LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN",
            "LAYERX_AGENT_INFERENCE_SERVE_LISTEN",
        ] {
            values.insert(key.to_owned(), "127.0.0.1:0".to_owned());
        }
        must(InferenceConfig::from_values(&values))
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn reference(height: u64) -> Reference {
    Reference {
        head_sequence: height,
        checkpoint: [7; 32],
    }
}

fn ready_gate() -> InferenceBootGate {
    let mut gate = InferenceBootGate::default();
    must(gate.ready(reference(10), Rung::Committed, &json!({"F11": "available"})));
    gate
}

#[test]
fn inference_market_mode_parses_and_keeps_other_modes_closed() {
    assert_eq!(
        Mode::parse(Some("inference-market")),
        Ok(Mode::InferenceMarket)
    );
    assert_eq!(Mode::parse(None), Ok(Mode::Full));
    assert_eq!(Mode::parse(Some("full")), Ok(Mode::Full));
    assert_eq!(Mode::parse(Some("human-owner")), Ok(Mode::HumanOwner));
    for invalid in [
        "",
        "inference",
        "Inference-market",
        "inference-market ",
        " inference-market",
    ] {
        assert!(Mode::parse(Some(invalid)).is_err(), "accepted {invalid:?}");
    }
}

#[test]
fn inference_environment_loads_the_selected_profile_and_exact_settings() {
    const CHILD: &str = "LAYERX_INFERENCE_PROFILE_TEST_CHILD";
    if std::env::var(CHILD).as_deref() == Ok("1") {
        assert_eq!(Mode::from_environment(), Ok(Mode::InferenceMarket));
        let config = must(InferenceConfig::from_environment());
        assert_eq!(config.model_digest, [0x11; 32]);
        assert_eq!(config.runtime_digest, [0x22; 32]);
        assert_eq!(config.signal_listen.to_string(), "127.0.0.1:18790");
        assert_eq!(config.serve_listen.to_string(), "127.0.0.1:18791");
        assert_eq!(must(config.load_room_profile()), b"{}");
        return;
    }
    let directory = Directory::new();
    let output = must(
        Command::new(must(std::env::current_exe()))
            .args([
                "--exact",
                "inference_environment_loads_the_selected_profile_and_exact_settings",
            ])
            .env_clear()
            .envs(directory.values())
            .env(CHILD, "1")
            .env("LAYERX_AGENT_MODE", "inference-market")
            .env("LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN", "127.0.0.1:18790")
            .env("LAYERX_AGENT_INFERENCE_SERVE_LISTEN", "127.0.0.1:18791")
            .output(),
    );
    assert!(
        output.status.success(),
        "{}\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
}

#[test]
fn inference_config_refuses_each_missing_blank_zero_or_malformed_digest() {
    let directory = Directory::new();
    for setting in [
        "LAYERX_AGENT_INFERENCE_MODEL_DIGEST",
        "LAYERX_AGENT_INFERENCE_RUNTIME_DIGEST",
    ] {
        let mut missing = directory.values();
        missing.remove(setting);
        assert!(must(
            InferenceConfig::from_values(&missing)
                .err()
                .ok_or("missing refusal")
        )
        .contains(setting));
        for invalid in [
            "".to_owned(),
            " ".to_owned(),
            "00".repeat(32),
            "gg".repeat(32),
            "a".repeat(63),
            "a".repeat(65),
        ] {
            let mut values = directory.values();
            values.insert(setting.to_owned(), invalid);
            assert!(must(
                InferenceConfig::from_values(&values)
                    .err()
                    .ok_or("missing refusal")
            )
            .contains(setting));
        }
    }
}

#[test]
fn inference_config_defaults_overrides_and_room_profile_are_used() {
    let directory = Directory::new();
    let mut values = directory.values();
    let defaults = must(InferenceConfig::from_values(&values));
    assert_eq!(defaults.signal_listen.to_string(), "127.0.0.1:8790");
    assert_eq!(defaults.serve_listen.to_string(), "127.0.0.1:8791");
    assert_eq!(defaults.model_digest, [0x11; 32]);
    assert_eq!(defaults.runtime_digest, [0x22; 32]);
    assert_eq!(must(defaults.load_room_profile()), b"{}");
    values.insert(
        "LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN".to_owned(),
        "127.0.0.1:18790".to_owned(),
    );
    values.insert(
        "LAYERX_AGENT_INFERENCE_SERVE_LISTEN".to_owned(),
        "127.0.0.1:18791".to_owned(),
    );
    let configured = must(InferenceConfig::from_values(&values));
    assert_eq!(configured.signal_listen.port(), 18790);
    assert_eq!(configured.serve_listen.port(), 18791);
    values.insert(
        "LAYERX_AGENT_INFERENCE_SIGNAL_LISTEN".to_owned(),
        String::new(),
    );
    assert!(InferenceConfig::from_values(&values).is_err());
    values = directory.values();
    values.remove("LAYERX_AGENT_INFERENCE_ROOM_PROFILE");
    assert!(InferenceConfig::from_values(&values).is_err());
    must(fs::write(&defaults.room_profile, b""));
    assert!(defaults.load_room_profile().is_err());
}

#[test]
fn inference_profile_refuses_missing_or_unavailable_f11_before_binding() {
    let directory = Directory::new();
    let occupied = must(TcpListener::bind("127.0.0.1:0"));
    let mut config = directory.config();
    config.signal_listen = must(occupied.local_addr());
    let mut components = serde_json::Map::new();
    for family in 1..=10 {
        components.insert(format!("F{family:02}"), json!("available"));
    }
    let mut cases = vec![Value::Object(components)];
    for status in [
        "not-enabled",
        "not-yet-produced",
        "content-unavailable",
        "unsupported-version",
        "unknown",
    ] {
        cases.push(json!({"F11": status}));
    }
    for components in cases {
        let mut gate = InferenceBootGate::default();
        assert_eq!(
            gate.ready(reference(10), Rung::Committed, &components),
            Err(InferenceBootError::F11Unavailable)
        );
        assert_eq!(gate.status().mode, DegradedMode::Unreachable);
        let error = match serve_inference_market(config.clone(), gate) {
            Ok(_) => panic!("profile booted without F11"),
            Err(error) => error,
        };
        assert_eq!(error, "inference boot dependencies are not ready");
    }
}

#[test]
fn inference_listener_boot_binds_signal_first_and_cleans_up_on_completion_failure() {
    let directory = Directory::new();
    let completion = must(TcpListener::bind("127.0.0.1:0"));
    let signal = must(TcpListener::bind("127.0.0.1:0"));
    let mut config = directory.config();
    config.signal_listen = must(signal.local_addr());
    config.serve_listen = must(completion.local_addr());
    let signal_address = config.signal_listen;
    drop(signal);
    let error = match serve_inference_market(config, ready_gate()) {
        Ok(_) => panic!("profile booted with an occupied completion listener"),
        Err(error) => error,
    };
    assert!(
        error.starts_with("inference completion listener failed:"),
        "{error}"
    );
    let _released_signal = must(TcpListener::bind(signal_address));
}

fn health_exchange(listener: &TcpListener, gate: &Arc<Mutex<InferenceBootGate>>) -> (u16, Value) {
    let address = must(listener.local_addr());
    let listener = must(listener.try_clone());
    let gate = Arc::clone(gate);
    let server = thread::spawn(move || {
        let (mut stream, _) = must(listener.accept());
        must(stream.set_read_timeout(Some(Duration::from_secs(5))));
        must(serve_health(&mut stream, &gate));
    });
    let mut client = must(TcpStream::connect(address));
    must(client.set_read_timeout(Some(Duration::from_secs(5))));
    must(client.write_all(b"GET /healthz HTTP/1.1\r\nHost: localhost\r\n\r\n"));
    let mut response = String::new();
    must(client.read_to_string(&mut response));
    must(server.join());
    let (headers, body) = response
        .split_once("\r\n\r\n")
        .unwrap_or_else(|| panic!("missing HTTP framing: {response}"));
    let status = must(
        headers
            .split_ascii_whitespace()
            .nth(1)
            .ok_or("missing status")
            .and_then(|status| status.parse::<u16>().map_err(|_| "invalid status")),
    );
    (status, must(serde_json::from_str(body)))
}

#[test]
fn inference_listeners_report_live_degraded_and_recovered_readiness_over_tcp() {
    let directory = Directory::new();
    let market = must(serve_inference_market(directory.config(), ready_gate()));
    assert_ne!(
        must(market.signal_listener.local_addr()),
        must(market.completion_listener.local_addr())
    );
    for listener in [&market.signal_listener, &market.completion_listener] {
        assert_eq!(
            health_exchange(listener, &market.gate),
            (200, json!({"ready": true}))
        );
    }
    assert!(observe_availability(&market.gate, Err("node connection lost".to_owned())).is_err());
    for listener in [&market.signal_listener, &market.completion_listener] {
        assert_eq!(
            health_exchange(listener, &market.gate),
            (503, json!({"ready": false}))
        );
    }
    must(observe_availability(
        &market.gate,
        Ok((reference(11), json!({"F11": "available"}))),
    ));
    assert_eq!(
        health_exchange(&market.completion_listener, &market.gate),
        (200, json!({"ready": true}))
    );
}

#[test]
fn inference_degraded_refuses_new_deals_and_keeps_the_open_deal_serving() {
    let directory = Directory::new();
    let gate = Arc::new(Mutex::new(ready_gate()));
    let deal = [1; 32];
    let record = directory.0.join("open-deal");
    must(must(must(gate.lock()).admit_deal(deal, || {
        fs::write(&record, b"open-deal-tokens")
    })));
    assert!(observe_availability(&gate, Err("node connection lost".to_owned())).is_err());
    let new_record = directory.0.join("new-deal");
    assert!(matches!(
        must(gate.lock()).admit_deal([2; 32], || fs::write(&new_record, b"must-not-run")),
        Err(InferenceBootError::Degraded(
            OperationError::LiveCoreRequired(DegradedMode::Unreachable)
        ))
    ));
    assert!(!new_record.exists());
    assert_eq!(
        must(must(
            must(gate.lock()).serve_deal(&deal, || fs::read(&record))
        )),
        b"open-deal-tokens"
    );
    assert_eq!(
        must(gate.lock())
            .serve_deal(&[3; 32], || fs::read(&record))
            .err(),
        Some(InferenceBootError::UnknownDeal)
    );
    must(observe_availability(
        &gate,
        Ok((reference(11), json!({"F11": "available"}))),
    ));
    must(must(must(gate.lock()).admit_deal([2; 32], || {
        fs::write(&new_record, b"recovered")
    })));
    assert_eq!(must(fs::read(new_record)), b"recovered");
    must(must(gate.lock()).close_deal(&deal));
    assert_eq!(
        must(gate.lock())
            .serve_deal(&deal, || fs::read(&record))
            .err(),
        Some(InferenceBootError::UnknownDeal)
    );
}
