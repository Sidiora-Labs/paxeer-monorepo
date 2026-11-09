import subprocess
import tomllib
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]


def _assert_lockfile_resolves_locked(directory, *, base=ROOT / "services/agent"):
    result = subprocess.run(
        [
            "cargo",
            "metadata",
            "--locked",
            "--format-version",
            "1",
            "--manifest-path",
            str(base / directory / "Cargo.toml"),
        ],
        cwd=ROOT,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    assert result.returncode == 0, f"{directory}: {result.stderr}"


def test_fuzz_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("fuzz")


def test_qualification_pay2_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("qualification/pay2")


def test_tests_boundary_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tests/boundary")


def test_tests_isolation_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tests/isolation")


def test_tests_parity_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tests/parity")


def test_tests_qualify_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tests/qualify")


def test_tools_audit_verify_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tools/audit-verify")


def test_tools_policy_harness_lockfile_resolves_locked():
    _assert_lockfile_resolves_locked("tools/policy-harness")


def test_sdk_registry_is_dev_only_and_consumers_resolve_locked():
    with (ROOT / "services/agent/crates/layerx-sdk/Cargo.toml").open("rb") as source:
        manifest = tomllib.load(source)

    registry_alias = "layerx_programs_registry"
    assert manifest["dev-dependencies"][registry_alias] == {
        "package": "layerx-programs-registry",
        "version": "=0.1.0",
        "path": "../../../../programs/crates/layerx-programs-registry",
    }
    assert registry_alias not in manifest["dependencies"]
    assert registry_alias not in manifest.get("build-dependencies", {})
    for target in manifest.get("target", {}).values():
        assert registry_alias not in target.get("dependencies", {})
        assert registry_alias not in target.get("build-dependencies", {})
    assert manifest["dependencies"]["ed25519-dalek"] == "=2.2.0"

    for directory in (
        "services/agent/fuzz",
        "services/agent/qualification/pay2",
        "services/agent/tests/isolation",
        "services/agent/tests/parity",
        "services/agent/tests/qualify",
        "services/agent/tools/audit-verify",
        "services/agent/tools/policy-harness",
        "services/human",
        "services/human/crates/layerx-intents/fuzz",
        "services/interop",
        "services/platform",
        "services/platform/examples/samples/verify-receipt-rust",
        "services/platform/hosted/node/tests/probe",
        "tools/platform",
    ):
        _assert_lockfile_resolves_locked(directory, base=ROOT)
