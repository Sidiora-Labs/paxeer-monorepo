//! Repair record: `tests/repair_22_33_44.kvx`.
//! These modules retain the original nonce, copy, mutation and budget assertions.

#[path = "decide_nonces.rs"]
mod decide_nonces;
#[path = "projection_rungs.rs"]
mod projection_rungs;
#[path = "render.rs"]
mod render;
pub mod runtime_clock {
    include!("support/repair_22_33_44_clock.rs");
}

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
    fs::create_dir_all(&root).or_fail("clock root");
    fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).or_fail("private root");
    let authority = runtime_clock::ClockAuthority::start(&root).or_fail("production clock server");
    let clock = authority.connect();
    let first = clock
        .sample(Duration::from_secs(1))
        .or_fail("first authenticated sample");
    let second = clock
        .sample(Duration::from_secs(1))
        .or_fail("second authenticated sample");
    assert_ne!(first.generation, [0; 16]);
    second
        .follows(first)
        .or_fail("monotonic authenticated samples");

    // Inspect the same explicit binding used by Fixture::gateway and Fixture::backend.
    // No child is launched: the focused gate requires no executable or toolchain peer.
    let mut command = std::process::Command::new("unused-child-path");
    command.env_clear();
    command
        .env("LAYERX_RUNTIME_CLOCK_SOCKET", &authority.binding.socket)
        .env(
            "LAYERX_RUNTIME_CLOCK_PID",
            authority.binding.process.to_string(),
        )
        .env(
            "LAYERX_RUNTIME_CLOCK_UID",
            authority.binding.user.to_string(),
        );
    let environment: BTreeMap<OsString, OsString> = command
        .get_envs()
        .map(|(key, value)| (key.to_owned(), value.or_fail("explicit binding").to_owned()))
        .collect();
    assert_eq!(environment.len(), 3);
    let binding = ClockBinding {
        socket: environment
            .get(&OsString::from("LAYERX_RUNTIME_CLOCK_SOCKET"))
            .or_fail("child socket")
            .into(),
        process: environment
            .get(&OsString::from("LAYERX_RUNTIME_CLOCK_PID"))
            .or_fail("child authority PID")
            .to_str()
            .or_fail("PID text")
            .parse()
            .or_fail("PID"),
        user: environment
            .get(&OsString::from("LAYERX_RUNTIME_CLOCK_UID"))
            .or_fail("child authority UID")
            .to_str()
            .or_fail("UID text")
            .parse()
            .or_fail("UID"),
    };
    let child_clock = RuntimeClock::connect(binding.clone()).or_fail("consumer of child binding");
    let child_sample = child_clock
        .sample(Duration::from_secs(1))
        .or_fail("child sample");
    assert_eq!(child_sample.generation, first.generation);
    assert!(child_sample.monotonic_nanoseconds >= second.monotonic_nanoseconds);
    let wrong_process = ClockBinding {
        process: binding.process.checked_add(1).or_fail("different PID"),
        ..binding
    };
    assert!(matches!(
        RuntimeClock::connect(wrong_process),
        Err(ClockError::Invalid)
    ));
    drop(child_clock);
    drop(clock);
    drop(authority);
    fs::remove_dir_all(root).or_fail("clock cleanup");
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
