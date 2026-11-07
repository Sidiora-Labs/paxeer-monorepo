import json
import subprocess
import tomllib
import xml.etree.ElementTree as ET
from functools import cache
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
VERSION = "0.1.0"
MANIFEST_NAMES = {"package.json", "Cargo.toml", "pyproject.toml", "pom.xml"}
VENDORED_TREES = (
    "apps/explorer",
    "apps/launchpad/web/public/charting_library",
    "contracts/bridge/solana/vendor",
    "core/deps",
    "programs/vendor",
    "sdk/chainflow/typescript/src/internal/qs",
    "services/explorer",
    "tools/benchmark/loadtest/contracts/evm/lib/openzeppelin-contracts",
    "tools/benchmark/loadtest/contracts/evm/lib/solmate",
    "tools/hyperfide",
    "tools/uci",
    "tools/wasm-tools",
    "wasm-runtime/libwasmvm",
)
EXCLUDED_DIRECTORIES = {
    "test",
    "tests",
    "testdata",
    "fixture",
    "fixtures",
    "test-support",
    "vendor",
    "node_modules",
}
VERSION_ALLOWLIST = {
    "package.json": (None, "Repository npm workspace coordinator has no release version"),
    "apps/launchpad/package.json": (
        None,
        "Private launchpad workspace coordinator has no release version",
    ),
    "apps/web/src/app/exchange/package.json": (
        None,
        "Private exchange integration dependency and ES module configuration",
    ),
    "sdk/platform/conformance-runner/package.json": (
        None,
        "Private ES module configuration for the SDK conformance runner",
    ),
    "services/bridge/deploy/appenv/package.json": (
        None,
        "Private deployment interface proof dependency environment",
    ),
    "tools/codebase-map/package.json": (
        None,
        "Private repository map generation and serving tool",
    ),
    "services/platform/examples/samples/first-payment-csharp/FirstPayment.csproj": (
        None,
        "Executable quickstart referencing the SDK, with no package version",
    ),
    "programs/sdk/rust/package.json": (
        "0.0.0",
        "Private TypeScript dependency harness for Rust SDK conformance",
    ),
    "services/agent/fuzz/Cargo.toml": (
        "0.0.0",
        "Unpublished cargo-fuzz harness is exempt from SDK/app releases",
    ),
    "services/human/crates/layerx-intents/fuzz/Cargo.toml": (
        "0.0.0",
        "Unpublished cargo-fuzz harness is exempt from SDK/app releases",
    ),
    "tools/benchmark/loadtest/contracts/pax-cosmwasm/Cargo.toml": (
        "0.4.9",
        "Unpublished load-test contract retains its benchmark version",
    ),
}


@cache
def _manifests():
    result = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=ROOT,
        capture_output=True,
        check=True,
    )
    paths = {
        Path(name.decode("utf-8", errors="surrogateescape"))
        for name in result.stdout.split(b"\0")
        if name
    }
    manifests = tuple(
        path
        for path in sorted(paths)
        if (path.name in MANIFEST_NAMES or path.suffix == ".csproj")
        and not any(path.is_relative_to(tree) for tree in VENDORED_TREES)
        and not EXCLUDED_DIRECTORIES.intersection(path.parts[:-1])
    )
    assert manifests, "No first-party manifests found"
    assert Path("sdk/js/package.json") in manifests, "SDK missing from inventory"
    assert Path("apps/wallet/package.json") in manifests, "App missing from inventory"
    assert MANIFEST_NAMES <= {path.name for path in manifests}
    assert any(path.suffix == ".csproj" for path in manifests)
    return manifests


@cache
def _toml(path):
    return tomllib.loads((ROOT / path).read_text(encoding="utf-8"))


def _cargo_workspace(path, package):
    if "workspace" in package:
        workspace = (ROOT / path.parent / package["workspace"] / "Cargo.toml").resolve()
        assert workspace.is_relative_to(ROOT), f"Workspace outside repository: {path}"
        data = _toml(workspace.relative_to(ROOT))
        assert "workspace" in data, f"Not a Cargo workspace: {workspace}"
        return data["workspace"].get("package", {})
    for directory in (path.parent, *path.parent.parents):
        candidate = directory / "Cargo.toml"
        if (ROOT / candidate).is_file():
            data = _toml(candidate)
            if "workspace" in data:
                return data["workspace"].get("package", {})
    raise AssertionError(f"No Cargo workspace for {path}")


def _cargo_version(path, package):
    version = package.get("version")
    if isinstance(version, dict):
        assert version == {"workspace": True}, f"Invalid version inheritance: {path}"
        version = _cargo_workspace(path, package).get("version")
        assert isinstance(version, str), f"Workspace version missing for {path}"
    return version


@cache
def _versions(path):
    if path.name == "package.json":
        return (json.loads((ROOT / path).read_text(encoding="utf-8")).get("version"),)
    if path.name == "pyproject.toml":
        return (_toml(path).get("project", {}).get("version"),)
    if path.name == "Cargo.toml":
        data = _toml(path)
        versions = []
        if "package" in data:
            versions.append(_cargo_version(path, data["package"]))
        if "package" in data.get("workspace", {}):
            versions.append(data["workspace"]["package"].get("version"))
        assert versions, f"No package or workspace version declared in {path}"
        return tuple(versions)
    root = ET.fromstring((ROOT / path).read_text(encoding="utf-8"))
    for element in root.iter():
        element.tag = element.tag.rsplit("}", 1)[-1]
    if path.name == "pom.xml":
        return (root.findtext("version"),)
    versions = tuple(
        (element.text or "").strip()
        for element in root.findall("./PropertyGroup/*")
        if element.tag in {"Version", "PackageVersion"}
    )
    return versions or (None,)


def _is_sdk(path):
    return "sdk" in path.parts or path.parent.name == "layerx-sdk"


def _assert_versions(paths):
    paths = tuple(paths)
    assert paths, "No manifests selected for version checking"
    violations = []
    for path in paths:
        expected = VERSION
        if path.as_posix() in VERSION_ALLOWLIST:
            expected, _ = VERSION_ALLOWLIST[path.as_posix()]
        versions = _versions(path)
        if any(version != expected for version in versions):
            violations.append(f"{path}: expected {expected!r}, found {versions!r}")
    assert not violations, "First-party package versions:\n" + "\n".join(violations)


def test_first_party_sdk_versions_are_0_1_0():
    _assert_versions(path for path in _manifests() if _is_sdk(path))


def test_first_party_app_versions_are_0_1_0():
    _assert_versions(path for path in _manifests() if not _is_sdk(path))


def test_version_allowlist_entries_exist():
    manifests = {path.as_posix() for path in _manifests()}
    for relative, (version, reason) in VERSION_ALLOWLIST.items():
        assert relative in manifests, f"Stale or excluded version exception: {relative}"
        assert (ROOT / relative).is_file(), f"Missing version exception: {relative}"
        assert isinstance(reason, str) and reason.strip(), f"Missing reason: {relative}"
        assert version != VERSION, f"Unnecessary version exception: {relative}"
        assert all(value == version for value in _versions(Path(relative))), (
            f"Version exception no longer matches {relative}"
        )
