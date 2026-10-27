import re
import shlex

import pytest

from programme.parity import ParityError, command_for


def test_go_selectors_build_anchored_run() -> None:
    row = {"selectors": ["node/a_test.go::TestA", "node/b_test.go::TestB"]}
    command = command_for(row)
    assert command == "go test -count=1 ./node/ -run '^(TestA|TestB)$'"
    pattern = shlex.split(command)[-1]
    assert re.search(pattern, "TestA")
    assert re.search(pattern, "TestB")
    for unselected in ("TestAA", "TestBB", "OtherTestA", "TestC"):
        assert re.search(pattern, unselected) is None
    assert row["selectors"] == ["node/a_test.go::TestA", "node/b_test.go::TestB"]

    assert command_for({"selectors": ["node/a_test.go::TestA"]}) == (
        "go test -count=1 ./node/ -run '^(TestA)$'"
    )
    assert command_for(
        {
            "selectors": [
                "node/sequencer/a_test.go::TestA",
                "node/sequencer/b_test.go::TestB",
            ]
        }
    ) == "go test -count=1 ./node/sequencer/ -run '^(TestA|TestB)$'"

    assert command_for(
        {"selectors": ["tests/test_a.py::test_a", "tests/test_b.py::test_b"]}
    ) == "python3 -m pytest tests/test_a.py::test_a tests/test_b.py::test_b -q"
    assert command_for(
        {
            "selectors": [
                "services/agent/crates/layerx-sdk/tests/native_programs.rs"
                "::native_request_binds_real_signed_fixture"
            ]
        }
    ) == (
        "cargo test --locked --manifest-path services/agent/Cargo.toml "
        "-p layerx-sdk --test native_programs -- --exact "
        "native_request_binds_real_signed_fixture"
    )
    rust_file = "programs/crates/layerx-programs-runtime/tests/lifecycle.rs"
    rust_names = [
        "deploy_returns_an_opaque_receipt_without_a_runtime_trust_bypass",
        "immutable_is_default_and_upgrade_records_hash_history",
    ]
    assert command_for(
        {"selectors": [f"{rust_file}::{name}" for name in rust_names]}
    ) == (
        "cargo test --locked --manifest-path programs/Cargo.toml "
        "-p layerx-programs-runtime --test lifecycle -- --exact "
        + " ".join(rust_names)
    )


def test_two_packages_are_refused() -> None:
    selectors = ["node/a_test.go::TestA", "rpc/b_test.go::TestB"]
    for selection in (selectors, selectors[::-1]):
        with pytest.raises(ParityError, match="Go selectors span two packages"):
            command_for({"selectors": selection})

    with pytest.raises(ParityError, match="Rust selectors span two crates"):
        command_for(
            {
                "selectors": [
                    "programs/crates/layerx-programs-runtime/tests/lifecycle.rs"
                    "::deploy_returns_an_opaque_receipt_without_a_runtime_trust_bypass",
                    "programs/crates/layerx-programs-ai-market/tests/market_lifecycle.rs"
                    "::a01_create_registers_once_at_committed_origin",
                ]
            }
        )
    for row in (
        {},
        {"selectors": []},
        {"selectors": "node/a_test.go::TestA"},
        {"selectors": ["node/a_test.go"]},
        {"selectors": ["node/a_test.go::Test.*"]},
        {"selectors": ["../node/a_test.go::TestA"]},
        {"selectors": ["node/a_test.go::TestA", "tests/test_a.py::test_a"]},
    ):
        with pytest.raises(ParityError):
            command_for(row)
