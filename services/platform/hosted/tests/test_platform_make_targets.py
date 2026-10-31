import re
from pathlib import Path


def test_platform_targets_build_no_kernel_binaries():
    makefile = (Path(__file__).resolve().parents[2] / "Makefile.inc").read_text()

    for retired in (
        "layerxd",
        "layerx-genesis-build",
        "layerx-archive-codec",
        "tests/relay-archive",
        "node-artifacts.py",
        "platform-build-node-artifacts",
        "platform-test-node",
    ):
        assert retired not in makefile, f"Platform targets still reference {retired}"

    target = re.search(
        r"^platform-test-plain-listeners:\n((?:\t[^\n]*\n)+)",
        makefile,
        re.MULTILINE,
    )
    assert target is not None, "Plain listener target must have no prerequisites"

    runner = "\tsh $(CURDIR)/tools/runtime/run-with-clock.sh "
    platform_test = (
        "$(PLATFORM_CARGO) test --offline --manifest-path $(PLATFORM_MANIFEST) "
        "--locked -p "
    )
    assert target.group(1).splitlines() == [
        runner + platform_test + "layerx-platform-dashboard --test plain_listener",
        runner + platform_test + "layerx-platform-webhooks --test plain_listener",
        runner + platform_test + "layerx-reference-ramp --test plain_listener",
        runner
        + "cargo test --offline --manifest-path services/interop/Cargo.toml "
        + "--locked -p layerx-interop-service --test plain_listener",
    ]
