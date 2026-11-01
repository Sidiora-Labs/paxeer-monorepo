from pathlib import Path
import tomllib


Required = {
    "gateway-lifecycle-receipt-verify": "verifier.rs",
    "gateway-lifecycle-sdk-verify": "lifecycle_sdk.rs",
}


def test_gateway_local_harness_has_no_lifecycle_test():
    tests = Path(__file__).resolve().parents[1] / "gateway" / "tests"
    local = tests / "local"
    manifest = tomllib.loads((local / "Cargo.toml").read_text(encoding="utf-8"))

    for target in manifest.get("test", []):
        assert target.get("name") != "lifecycle", "Retired lifecycle test is declared"
        assert target.get("path") != "lifecycle.rs", "Retired lifecycle test is wired"

    for name in (
        "lifecycle.rs",
        "events.rs",
        "funding.rs",
        "withdrawal.rs",
        "registry_runtime.rs",
        "build.rs",
    ):
        path = local / name
        assert not path.exists(), f"Retired lifecycle harness file remains: {name}"
        assert not path.is_symlink(), f"Retired lifecycle harness symlink remains: {name}"

    assert not manifest.get("dev-dependencies"), "Lifecycle dev-dependencies remain"
    binaries = {target["name"]: target["path"] for target in manifest["bin"]}
    for name, source in Required.items():
        assert binaries.get(name) == source, f"Required binary declaration changed: {name}"
        assert (local / source).is_file(), f"Required binary source is missing: {source}"
    assert (local / "required.rs").is_file(), "Shared Required trait must remain"

    for name in ("retired_client_modules.rs", "retired_client_modules_w2.rs"):
        path = tests / name
        assert not path.exists(), f"Retired lifecycle module scan remains: {name}"
        assert not path.is_symlink(), f"Retired lifecycle module scan symlink remains: {name}"
