//! The emulator entry point retains the real core linkage.

/// The emulator library publishes the build-script link directives for the
/// `LayerX` C core, so the bridge entry points these tests call resolve only when
/// the library itself is part of this binary. Driving its public entry keeps
/// that dependency explicit and pins the refusal an empty command line earns.
#[test]
fn emulator_entry_refuses_an_empty_command_line() {
    assert!(layerx_platform_emulator::run(Vec::<String>::new()).is_err());
}
