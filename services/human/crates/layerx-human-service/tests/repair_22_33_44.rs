//! Repair record: tests/repair_22_33_44.kvx.
//! These modules retain the original nonce, copy, mutation and budget assertions.

#[path = "decide_nonces.rs"]
mod decide_nonces;
#[path = "projection_rungs.rs"]
mod projection_rungs;
#[path = "render.rs"]
mod render;
#[path = "support/repair_22_33_44_clock.rs"]
mod runtime_clock;

#[path = "support/repair_22_33_44_kms.rs"]
mod in_process_gateway;

use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_types::clock::{Clock, ClockError};
use std::collections::BTreeMap;
use std::ffi::OsString;
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::time::Duration;

#[test]
fn kms_clock_authenticated_binding_survives_a_cleared_child_environment() {
    let root = layerx_human_test_support::directory("repair-kms-clock");
    fs::create_dir_all(&root).expect("clock root");
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).expect("private root");
    let authority = runtime_clock::ClockAuthority::start(&root).expect("production clock server");
    let clock = authority.connect();
    let first = clock
        .sample(Duration::from_secs(1))
        .expect("first authenticated sample");
    let second = clock
        .sample(Duration::from_secs(1))
        .expect("second authenticated sample");
    assert_ne!(first.generation, [0; 16]);
    second
        .follows(first)
        .expect("monotonic authenticated samples");

    // Inspect the same explicit binding used by Fixture::gateway and Fixture::backend.
    // No child is launched: the focused gate requires no executable or toolchain peer.
    let mut command = std::process::Command::new("unused-child-path");
    command.env_clear();
    authority.bind_command(&mut command);
    let environment: BTreeMap<OsString, OsString> = command
        .get_envs()
        .map(|(key, value)| (key.to_owned(), value.expect("explicit binding").to_owned()))
        .collect();
    assert_eq!(environment.len(), 3);
    let binding = ClockBinding {
        socket: environment
            .get(&OsString::from("LAYERX_RUNTIME_CLOCK_SOCKET"))
            .expect("child socket")
            .into(),
        process: environment
            .get(&OsString::from("LAYERX_RUNTIME_CLOCK_PID"))
            .expect("child authority PID")
            .to_str()
            .expect("PID text")
            .parse()
            .expect("PID"),
        user: environment
            .get(&OsString::from("LAYERX_RUNTIME_CLOCK_UID"))
            .expect("child authority UID")
            .to_str()
            .expect("UID text")
            .parse()
            .expect("UID"),
    };
    let child_clock = RuntimeClock::connect(binding.clone()).expect("consumer of child binding");
    let child_sample = child_clock
        .sample(Duration::from_secs(1))
        .expect("child sample");
    assert_eq!(child_sample.generation, first.generation);
    assert!(child_sample.monotonic_nanoseconds >= second.monotonic_nanoseconds);
    let wrong_process = ClockBinding {
        process: binding.process.checked_add(1).expect("different PID"),
        ..binding
    };
    assert!(matches!(
        RuntimeClock::connect(wrong_process),
        Err(ClockError::Invalid)
    ));
    drop(child_clock);
    drop(clock);
    drop(authority);
    fs::remove_dir_all(root).expect("clock cleanup");
}
