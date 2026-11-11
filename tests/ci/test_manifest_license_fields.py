import importlib.util
import json
import re
import subprocess
import tomllib
import xml.etree.ElementTree as ET
import zipfile
from email import policy
from email.parser import BytesParser
from functools import cache
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
ORGANISATION = "Sidiora Labs"
REPOSITORY_URL = "https://github.com/Sidiora-Labs/paxeer-monorepo"
MANIFEST_NAMES = {
    "package.json",
    "Cargo.toml",
    "pyproject.toml",
    "pom.xml",
    "Package.swift",
    "build.gradle",
}
VENDORED_TREES = {
    "apps/explorer": "Blockscout frontend and its upstream deploy tools",
    "apps/launchpad/web/public/charting_library": "TradingView charting library",
    "contracts/bridge/solana/vendor": "Agave dependencies",
    "core/deps": "Imported chain dependencies",
    "programs/vendor": "Pinned Cargo dependencies",
    "sdk/chainflow/typescript/src/internal/qs": "Upstream qs implementation",
    "services/explorer": "Blockscout backend and services",
    "tests/fixtures": "Imported contract and dependency fixtures",
    "tools/benchmark/loadtest/contracts/evm/lib/openzeppelin-contracts": "OpenZeppelin",
    "tools/benchmark/loadtest/contracts/evm/lib/solmate": "Solmate",
    "tools/hyperfide": "Vendored Hyperfide project and its dependencies",
    "tools/uci": "Vendored UCI project",
    "tools/wasm-tools": "Upstream Bytecode Alliance wasm-tools",
    "wasm-runtime/libwasmvm": "Upstream CosmWasm wasmvm; retain its author",
}
VENDORED_DIRECTORY_NAMES = {"testdata": "Imported test data and dependency fixtures"}
REQUIRED_AUTHORS = {
    "contracts/package.json",
    "contracts/launchpad/package.json",
    "sdk/chainflow/python/pyproject.toml",
    "sdk/chainflow/rust/Cargo.toml",
    "sdk/chainflow/typescript/package.json",
    "tests/integration/dapp_tests/package.json",
}
MAVEN_LICENSE_MANIFESTS = {
    "sdk/integrations/android/pom.xml",
    "sdk/integrations/spring/pom.xml",
    "sdk/platform/jvm/pom.xml",
}
SPDX_HEADER = re.compile(r"SPDX-License-Identifier:\s*([^\r\n]+)")
SPDX_EXPRESSION = re.compile(
    r"\(*[A-Za-z0-9][A-Za-z0-9.+-]*\)*"
    r"(?: (?:AND|OR|WITH) \(*[A-Za-z0-9][A-Za-z0-9.+-]*\)*)*"
)
LICENSE_FILENAMES = (
    "LICENSE",
    "LICENSE.txt",
    "LICENSE.md",
    "LICENCE",
    "LICENCE.txt",
    "LICENCE.md",
)


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
    manifests = []
    for path in sorted(paths):
        if path.name not in MANIFEST_NAMES and path.suffix != ".csproj":
            continue
        if any(path.is_relative_to(tree) for tree in VENDORED_TREES):
            continue
        if any(part in VENDORED_DIRECTORY_NAMES for part in path.parts[:-1]):
            continue
        manifests.append(ROOT / path)
    assert manifests, "No first-party manifests found"
    assert ROOT / "package.json" in manifests, "Root manifest missing from inventory"
    return tuple(manifests)


@cache
def _toml(path):
    return tomllib.loads(path.read_text(encoding="utf-8"))


def _cargo_workspace(path):
    package = _toml(path).get("package", {})
    if "workspace" in package:
        workspace = (path.parent / package["workspace"] / "Cargo.toml").resolve()
        assert workspace.is_relative_to(ROOT), f"Workspace outside repository: {workspace}"
        assert "workspace" in _toml(workspace), f"Not a Cargo workspace: {workspace}"
        return _toml(workspace)["workspace"].get("package", {})
    for directory in (path.parent, *path.parent.parents):
        if not directory.is_relative_to(ROOT):
            break
        candidate = directory / "Cargo.toml"
        if candidate.is_file() and "workspace" in _toml(candidate):
            return _toml(candidate)["workspace"].get("package", {})
    raise AssertionError(f"No workspace for {path.relative_to(ROOT)}")


def _cargo_field(path, package, field):
    value = package.get(field)
    if isinstance(value, dict):
        assert value == {"workspace": True}, f"Invalid {field} inheritance: {value}"
        value = _cargo_workspace(path).get(field)
        assert value, f"Workspace does not declare {field}"
    return value


@cache
def _xml(path):
    root = ET.fromstring(path.read_text(encoding="utf-8"))
    for element in root.iter():
        element.tag = element.tag.rsplit("}", 1)[-1]
    return root


@cache
def _metadata(path):
    if path.name == "package.json":
        package = json.loads(path.read_text(encoding="utf-8"))
        license_value = package.get("license")
        author = package.get("author")
        authors = [] if author is None else [author]
    elif path.name in {"Cargo.toml", "pyproject.toml"}:
        data = _toml(path)
        if path.name == "Cargo.toml":
            package = data.get("package", data.get("workspace", {}).get("package", {}))
            license_value = _cargo_field(path, package, "license")
            authors = _cargo_field(path, package, "authors") or []
        else:
            package = data.get("project", {})
            license_value = package.get("license")
            authors = package.get("authors", [])
    elif path.name == "pom.xml":
        root = _xml(path)
        licenses = tuple(
            node.findtext("name", default="").strip()
            for node in root.findall("./licenses/license")
        )
        authors = [
            node.findtext("organization") or node.findtext("name", default="")
            for node in root.findall("./developers/developer")
        ]
        return licenses, tuple(authors)
    elif path.suffix == ".csproj":
        root = _xml(path)
        licenses = tuple(
            (node.text or "").strip()
            for node in root.findall("./PropertyGroup/PackageLicenseExpression")
        )
        authors = [
            author.strip()
            for node in root.findall("./PropertyGroup/Authors")
            for author in (node.text or "").split(";")
        ]
        return licenses, tuple(authors)
    else:
        text = path.read_text(encoding="utf-8")
        licenses = tuple(match.group(1).strip() for match in SPDX_HEADER.finditer(text))
        if path.name == "build.gradle":
            licenses += tuple(
                re.findall(r"\bext\.license\s*=\s*['\"]([^'\"]+)['\"]", text)
            )
        return licenses, ()
    licenses = () if license_value is None else (license_value,)
    return licenses, tuple(
        author.get("name", "") if isinstance(author, dict) else author
        for author in authors
    )


def _nearest_license(path):
    for directory in (path.parent, *path.parent.parents):
        if not directory.is_relative_to(ROOT):
            break
        for name in LICENSE_FILENAMES:
            candidate = directory / name
            if candidate.is_file():
                return candidate
    raise AssertionError(f"No governing LICENSE for {path.relative_to(ROOT)}")


def _license_identifier(path):
    text = path.read_text(encoding="utf-8")
    header = SPDX_HEADER.search(text)
    if header:
        return header.group(1).strip()
    if "Apache License" in text and "Version 2.0" in text:
        return "Apache-2.0"
    if "Permission is hereby granted, free of charge" in text:
        return "MIT"
    if "GNU GENERAL PUBLIC LICENSE" in text and "Version 3" in text:
        return "GPL-3.0"
    raise AssertionError(f"Unrecognised governing license: {path.relative_to(ROOT)}")


def _normalise(expression):
    return expression.replace("(", "").replace(")", "").strip()


def test_first_party_manifests_declare_license():
    violations = []
    for path in _manifests():
        try:
            licenses, _ = _metadata(path)
            assert licenses, "Missing licence declaration"
            for value in licenses:
                assert isinstance(value, str) and SPDX_EXPRESSION.fullmatch(value), (
                    f"Licence must be an SPDX identifier or expression: {value!r}"
                )
        except (AssertionError, ValueError, ET.ParseError) as error:
            violations.append(f"{path.relative_to(ROOT)}: {error}")
    assert not violations, "First-party manifest licences:\n" + "\n".join(violations)


def test_manifest_license_matches_nearest_license_file():
    violations = []
    for path in _manifests():
        try:
            licenses, _ = _metadata(path)
            governing = _nearest_license(path)
            expected = _license_identifier(governing)
            engine_license = path.parent / "LICENSE-ENGINE"
            if engine_license.is_file():
                expected += " AND " + _license_identifier(engine_license)
            assert licenses, "Missing licence declaration"
            assert all(
                isinstance(value, str) and _normalise(value) == expected
                for value in licenses
            ), f"{licenses!r} disagrees with {governing.relative_to(ROOT)} ({expected})"
            if path.relative_to(ROOT).as_posix() in MAVEN_LICENSE_MANIFESTS:
                entries = _xml(path).findall("./licenses/license")
                assert len(entries) == 1, "Expected one Maven licence"
                url = entries[0].findtext("url", default="").strip()
                assert url == f"{REPOSITORY_URL}/blob/main/LICENSE", (
                    f"Noncanonical repository LICENSE URL: {url}"
                )
        except (AssertionError, ValueError, ET.ParseError) as error:
            violations.append(f"{path.relative_to(ROOT)}: {error}")
    assert not violations, "Manifest/governing LICENSE disagreements:\n" + "\n".join(violations)


def test_python_wheels_emit_declared_license(tmp_path):
    projects = (
        (
            "fastapi",
            ROOT / "sdk/integrations/fastapi",
            ["fastapi>=0.110", "layerx-sdk>=0.1.0"],
            [],
        ),
        (
            "sdk",
            ROOT / "services/agent/sdk/python",
            ["cryptography>=41; extra == 'derivation'"],
            ["derivation"],
        ),
    )
    for name, project_root, expected_requires, expected_extras in projects:
        spec = importlib.util.spec_from_file_location(
            f"license_{name}_build_backend", project_root / "build_backend.py"
        )
        assert spec is not None and spec.loader is not None
        backend = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(backend)
        with (project_root / "pyproject.toml").open("rb") as manifest:
            declared_license = tomllib.load(manifest)["project"]["license"]

        wheels = []
        for build in range(2):
            output = tmp_path / name / str(build)
            wheel_path = output / backend.build_wheel(str(output))
            wheels.append(wheel_path.read_bytes())
            with zipfile.ZipFile(wheel_path) as archive:
                metadata_paths = [
                    path
                    for path in archive.namelist()
                    if path.endswith(".dist-info/METADATA")
                ]
                assert len(metadata_paths) == 1, name
                metadata = BytesParser(policy=policy.default).parsebytes(
                    archive.read(metadata_paths[0])
                )
            assert metadata.get_all("Metadata-Version") == ["2.4"], name
            assert metadata.get_all("License-Expression") == [declared_license], name
            assert "License" not in metadata, name
            assert metadata.get_all("Requires-Dist", []) == expected_requires, name
            assert metadata.get_all("Provides-Extra", []) == expected_extras, name
        assert wheels[0] == wheels[1], name


def test_first_party_authors_are_organisation():
    violations = []
    checked = set()
    for path in _manifests():
        relative = path.relative_to(ROOT).as_posix()
        checked.add(relative)
        try:
            _, authors = _metadata(path)
            if relative in REQUIRED_AUTHORS:
                assert authors, "Missing organisation author"
            assert all(author == ORGANISATION for author in authors), (
                f"Expected {ORGANISATION!r}, found {authors!r}"
            )
        except (AssertionError, ValueError, ET.ParseError) as error:
            violations.append(f"{relative}: {error}")
    assert REQUIRED_AUTHORS <= checked, "Required author manifests missing from inventory"
    upstream = _toml(ROOT / "wasm-runtime/libwasmvm/Cargo.toml")["package"]["authors"]
    assert upstream == ["Ethan Frey <ethanfrey@users.noreply.github.com>"], (
        "Retain the vendored wasm library's upstream author"
    )
    assert not violations, "First-party manifest authors:\n" + "\n".join(violations)
