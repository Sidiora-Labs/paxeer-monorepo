"""Keep independently packaged artifacts synchronized with their declared sources."""

import ast
from collections import defaultdict
from functools import lru_cache
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import zipfile

import pytest


ROOT = Path(__file__).resolve().parents[2]
ALLOWLIST = ROOT / "tests/ci/data/intentional_copies.txt"
MIN_COPY_BYTES = 20_000
ATTESTOR_ABIS = {
    "launchpad": "precompiles/launchpad/abi.json",
    "perps": "precompiles/perps/abi/abi.json",
    "layerxprograms": "precompiles/layerxprograms/abi.json",
}
DEPLOYMENT_SOURCE = "contracts/launchpad/deployments/paxeer-x-network-addresses.json"
DEPLOYMENT_COPY = "services/launchpad/indexer/internal/receipt/deployment_source.json"
INVENTORIES = "tools/launchpad/production/feature-acceptance"
# Upstream trees have their own copy policies. Explorer provenance is pinned in
# docs/explorer/UPSTREAM.md; the remaining imported trees carry VENDOR.kvx.
UPSTREAM_TREES = (
    "core/deps/",
    "programs/vendor/",
    "tools/hyperfide/",
    "tools/wasm-tools/",
    "tools/uci/",
    "tools/paxctl/",
    "tools/loadtest/",
    "services/explorer/backend/",
    "services/explorer/services/",
    "services/interop/specs/vendor/",
)
# These are engineering records and generated dependency/build outputs, not
# first-party product artifacts. Do not exclude application fixtures or assets.
NON_PRODUCT_TREES = ("spec/", ".hyperfide/", ".codify/", ".git/")
BUILD_DIRECTORIES = {"node_modules", "__pycache__", ".venv", "target", ".pytest_cache"}
DEPENDENCY_FILES = {
    "go.sum", "go.work.sum", "Cargo.lock", "pnpm-lock.yaml",
    "package-lock.json", "yarn.lock",
}


def _json(path):
    return json.loads((ROOT / path).read_text(encoding="utf-8"))


@lru_cache(maxsize=1)
def _listed_copies():
    rows = []
    lines = ALLOWLIST.read_text(encoding="utf-8").splitlines()
    for number, line in enumerate(lines, 1):
        if not line.strip() or line.startswith("#"):
            continue
        fields = tuple(field.strip() for field in line.split("|"))
        assert len(fields) == 5, f"{ALLOWLIST.name}:{number}: expected five fields"
        source, copy, comparison, test, reason = fields
        assert comparison in {"bytes", "json"}, fields
        assert source != copy and reason, fields
        for name in (source, copy):
            path = Path(name)
            assert not path.is_absolute() and ".." not in path.parts, fields
            assert (ROOT / path).is_file(), f"Missing intentional artifact: {name}"
            assert (ROOT / path).resolve().is_relative_to(ROOT), fields
        assert test.startswith("test_") and callable(globals().get(test)), fields
        rows.append(fields)
    assert rows, "The intentional-copy inventory must not be empty"
    pairs = [(source, copy) for source, copy, *_ in rows]
    assert len(set(pairs)) == len(pairs), "Duplicate intentional-copy declarations"
    return tuple(rows)


def _assert_copy(source, copy, comparison):
    if comparison == "json":
        assert _json(source) == _json(copy), f"{copy} differs from {source}"
    else:
        assert (ROOT / copy).read_bytes() == (ROOT / source).read_bytes(), (
            f"{copy} differs from {source}"
        )


@pytest.mark.parametrize("name", ATTESTOR_ABIS)
def test_attestor_abis_match_precompile_abis(name):
    source = ATTESTOR_ABIS[name]
    copy = f"services/wallet/attestor/internal/policy/evm/abi/{name}.json"
    abi = _json(source)
    assert isinstance(abi, list) and abi, f"Empty or invalid ABI: {source}"
    assert any(entry.get("type") == "function" for entry in abi), source
    _assert_copy(source, copy, "bytes")
    assert (source, copy, "bytes", "test_attestor_abis_match_precompile_abis") in {
        row[:4] for row in _listed_copies()
    }
    consumer = (
        ROOT / "services/wallet/attestor/internal/policy/evm/decode.go"
    ).read_text()
    assert "//go:embed abi/*.json" in consumer
    assert 'abiFiles.ReadFile("abi/" + name + ".json")' in consumer


def test_explorer_openapi_matches_backend_swagger():
    public = ROOT / "apps/explorer/public/openapi.yaml"
    backend = ROOT / "services/explorer/swagger.yaml"
    assert not public.is_symlink(), (
        "The frontend container must ship a regular public document"
    )
    assert backend.is_symlink(), (
        "The backend generator path must alias the sole stored document"
    )
    assert backend.resolve(strict=True) == public
    assert public.read_bytes() == backend.read_bytes()
    assert b"openapi: 3.0.3" in backend.read_bytes()
    consumer = (ROOT / "apps/explorer/ui/apiDocs/utils.ts").read_text()
    assert re.search(
        r"CORE_API_SPEC_PUBLIC_URL\s*=\s*['\"]/openapi\.yaml['\"]", consumer
    )
    dockerfile = (ROOT / "docker/explorer-frontend/Dockerfile").read_text()
    assert "COPY apps/explorer ." in dockerfile


def test_deployment_source_matches_launchpad_addresses():
    manifest = _json(DEPLOYMENT_SOURCE)
    assert manifest["_meta"]["network"] == "125"
    _assert_copy(DEPLOYMENT_SOURCE, DEPLOYMENT_COPY, "json")
    assert (
        DEPLOYMENT_SOURCE, DEPLOYMENT_COPY, "json",
        "test_deployment_source_matches_launchpad_addresses",
    ) in {row[:4] for row in _listed_copies()}


def _inventory_builder():
    path = ROOT / INVENTORIES / "build_inventory.py"
    spec = importlib.util.spec_from_file_location("copied_artifacts_inventory", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_retired_inventory_differs_from_live():
    historical_path = f"{INVENTORIES}/fixtures/retired-inventory.json"
    live_path = f"{INVENTORIES}/inventory.json"
    assert (ROOT / historical_path).read_bytes() != (ROOT / live_path).read_bytes()
    historical = _json(historical_path)
    live = _json(live_path)
    builder = _inventory_builder()
    specification = builder.specification()
    reference_policy = builder.reference_policy()
    assert historical["version"] == 1
    assert {feature["id"] for feature in historical["features"]} == set(builder.OWNERS)
    sections = set()
    for feature in historical["features"]:
        assert feature["state"] == "unqualified", feature["id"]
        assert feature["requirements"], feature["id"]
        sections.add("coverage." + feature["id"])
        sections.update("req." + rid for rid in feature["requirements"])
    assert sections
    for section in sorted(sections):
        assert section not in specification, (
            f"Current entry in retired inventory: {section}"
        )
        reference = reference_policy.reference(specification, section)
        assert reference["class"] == "retired-reference", reference
        assert "decision.1803" in reference["reason"], reference
    # The existing producer intentionally republishes historical declarations
    # when their release is retired. Preserve its ownership/lineage contract.
    assert builder.build() == live == historical
    assert (
        historical_path, live_path, "json", "test_retired_inventory_differs_from_live"
    ) in {row[:4] for row in _listed_copies()}


def test_statik_consumers_share_one_bundle():
    canonical = (ROOT / "docs/swagger/statik.go").read_text(encoding="utf-8")
    sdk = (ROOT / "sdk/client/docs/statik/statik.go").read_text(encoding="utf-8")
    assert 'import _ "github.com/Sidiora-Labs/Paxeer-X-Network/docs/swagger"' in sdk
    assert "data :=" not in sdk and "fs.Register" not in sdk
    assert 'const swaggerNamespace = "swagger"' in canonical
    assert "fs.Register(data)" in canonical
    assert "fs.RegisterWithNamespace(swaggerNamespace, data)" in canonical
    assert canonical.count("\tdata := ") == 1, (
        "Exactly one archive must initialize both namespaces"
    )
    literal = canonical.split("\tdata := ", 1)[1].split("\n\tfs.Register(data)", 1)[0]
    archive = ast.literal_eval("b" + literal)
    with zipfile.ZipFile(io.BytesIO(archive)) as bundle:
        assert bundle.testzip() is None
        names = bundle.namelist()
        assert len(names) == len(set(names)), "Duplicate archive members"
        assert {
            "index.html", "swagger-initializer.js", "swagger.yml", "swagger.yaml",
            "swagger-ui-bundle.js",
        } <= set(names)
        assert bundle.read("swagger.yml") == bundle.read("swagger.yaml")
        assert bundle.read("swagger.yaml")
        assert b"swagger.yml" in bundle.read("swagger-initializer.js")
        assert b"swagger-ui-bundle.js" in bundle.read("index.html")
    node = (ROOT / "node/app.go").read_text()
    server = (ROOT / "sdk/server/api/server.go").read_text()
    assert '_ "github.com/Sidiora-Labs/Paxeer-X-Network/docs/swagger"' in node
    assert 'fs.NewWithNamespace("swagger")' in node
    assert (
        '_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/docs/statik"' in server
    )


def test_shared_swagger_generation_round_trip(tmp_path):
    checkout = tmp_path / "checkout"
    checkout.mkdir()
    paths = (
        "scripts/swagger_assets.py", "scripts/update-swagger-ui-statik.sh",
        "sdk/scripts/update-swagger-ui-statik.sh", "sdk/Makefile",
        "sdk/contrib/devtools/Makefile", "sdk/client/docs/statik/statik.go",
        "consensus/version/version.go", "docs/swagger/statik.go",
    )
    for name in paths:
        destination = checkout / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, destination)
    shutil.copytree(ROOT / "docs/swagger-ui", checkout / "docs/swagger-ui")
    module = "github.com/Sidiora-Labs/Paxeer-X-Network"
    (checkout / "go.mod").write_text(
        f"module {module}\n\ngo 1.20\n\nrequire github.com/rakyll/statik v0.1.7\n"
    )
    (checkout / "go.sum").write_text("".join(
        line for line in (ROOT / "go.sum").read_text().splitlines(keepends=True)
        if line.startswith("github.com/rakyll/statik v0.1.7")
    ))
    binaries = tmp_path / "bin"
    binaries.mkdir()
    env = dict(os.environ, GOWORK="off", GOTOOLCHAIN="local", GOBIN=str(binaries))
    env["PATH"] = str(binaries) + os.pathsep + env["PATH"]
    env["STATIK"] = str(binaries / "statik")

    def run(*command, cwd=checkout, check=True):
        result = subprocess.run(
            command, cwd=cwd, env=env, capture_output=True, text=True, timeout=180,
        )
        if check:
            assert result.returncode == 0, (
                f"{command}: {result.returncode}\n{result.stdout}\n{result.stderr}"
            )
        return result

    run("go", "install", "github.com/rakyll/statik@v0.1.7")
    checker = checkout / "checker"
    checker.mkdir()
    (checker / "main.go").write_text('''package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"

    _ "github.com/Sidiora-Labs/Paxeer-X-Network/docs/swagger"
    _ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/docs/statik"
    "github.com/rakyll/statik/fs"
)

func readAssets(files http.FileSystem) map[string]string {
    values := make(map[string]string)
    for _, name := range []string{
        "swagger.yml", "swagger.yaml", "index.html",
        "swagger-initializer.js", "swagger-ui-bundle.js",
    } {
        file, err := files.Open("/" + name)
        if err != nil { panic(err) }
        data, err := io.ReadAll(file)
        if err != nil { panic(err) }
        if err := file.Close(); err != nil { panic(err) }
        if len(data) == 0 { panic(fmt.Sprintf("empty asset: %s", name)) }
        digest := sha256.Sum256(data)
        values[name] = hex.EncodeToString(digest[:])
    }
    return values
}

func main() {
    ordinary, err := fs.New()
    if err != nil { panic(err) }
    named, err := fs.NewWithNamespace("swagger")
    if err != nil { panic(err) }
    if err := json.NewEncoder(os.Stdout).Encode(map[string]map[string]string{
        "default": readAssets(ordinary), "swagger": readAssets(named),
    }); err != nil { panic(err) }
}
''')
    archive = checkout / "docs/swagger/statik.go"
    shim = checkout / "sdk/client/docs/statik/statik.go"
    original_shim = shim.read_bytes()
    schema = (checkout / "docs/swagger-ui/swagger.yml").read_bytes()
    assert schema
    expected = {
        name: hashlib.sha256((checkout / "docs/swagger-ui" / name).read_bytes()).hexdigest()
        for name in (
            "swagger.yml", "index.html", "swagger-initializer.js", "swagger-ui-bundle.js",
        )
    }
    expected["swagger.yaml"] = expected["swagger.yml"]

    def assert_bundle():
        source = archive.read_text()
        assert source.count("\tdata := ") == 1
        assert source.count("\tfs.Register(data)") == 1
        assert source.count("\tfs.RegisterWithNamespace(swaggerNamespace, data)") == 1
        assert shim.read_bytes() == original_shim
        assert list((checkout / "sdk/client/docs/statik").glob("*.go")) == [shim]
        literal = re.search(r'(?m)^\tdata := (.*)$', source)[1]
        with zipfile.ZipFile(io.BytesIO(ast.literal_eval("b" + literal))) as bundle:
            assert len(bundle.namelist()) == len(set(bundle.namelist()))
            assert bundle.testzip() is None
            assert bundle.read("swagger.yml") == bundle.read("swagger.yaml") == schema
        run("go", "build", "-mod=mod", "-o", str(binaries / "consumer"), "./checker")
        assert json.loads(run(str(binaries / "consumer")).stdout) == {
            "default": expected, "swagger": expected,
        }
        assert not list((checkout / "docs").glob(".swagger-statik.*"))

    # Bundle the existing canonical schema explicitly; full OpenAPI regeneration
    # still requires the real Ignite project configuration in the root script.
    run("sh", "scripts/update-swagger-ui-statik.sh", "--assets-only", cwd=checkout)
    assert_bundle()
    first = archive.read_bytes()
    run("sh", str(checkout / "sdk/scripts/update-swagger-ui-statik.sh"), cwd=tmp_path)
    assert_bundle()
    assert archive.read_bytes() == first

    # A real temporary Git repository gives the Make drift check a tracked HEAD
    # without changing the working repository or replacing Git with a test double.
    run("git", "init", "-q")
    run("git", "add", ".")
    run("git", "-c", "user.name=Swagger test", "-c", "user.email=swagger@example.invalid",
        "-c", "core.hooksPath=/dev/null", "commit", "-qm", "Canonical Swagger baseline")
    make = (
        "make", "-C", str(checkout / "sdk"), "update-swagger-docs",
        f"TOOLS_DESTDIR={binaries}", "LEDGER_ENABLED=false",
    )
    run(*make)
    assert_bundle()
    assert archive.read_bytes() == first

    # Schema drift reaches the published archive and must fail synchronization.
    schema += b"\n# round-trip schema change\n"
    (checkout / "docs/swagger-ui/swagger.yml").write_bytes(schema)
    expected["swagger.yml"] = expected["swagger.yaml"] = hashlib.sha256(schema).hexdigest()
    drift = run(*make, check=False)
    assert drift.returncode != 0
    assert "Swagger docs are out of sync" in drift.stdout + drift.stderr
    assert_bundle()
    assert archive.read_bytes() != first

    # Staged and unstaged shim changes must also remain covered by the check.
    (checkout / "docs/swagger-ui/swagger.yml").write_bytes(
        (ROOT / "docs/swagger-ui/swagger.yml").read_bytes()
    )
    for staged in (False, True):
        shim.write_bytes(original_shim + b"\n// drift\n")
        if staged:
            run("git", "add", "sdk/client/docs/statik/statik.go")
        drift = run(*make, check=False)
        assert drift.returncode != 0
        assert "Swagger docs are out of sync" in drift.stdout + drift.stderr
    shim.write_bytes(original_shim)
    run("git", "add", "sdk/client/docs/statik/statik.go")
    run(*make)

    # Fail the actual statik executable and canonical-asset preparation, then
    # invoke the actual publisher with malformed output: none may replace bytes.
    previous = archive.read_bytes()
    statik = binaries / "statik"
    statik.chmod(0o644)
    try:
        for command in (
            ("sh", "scripts/update-swagger-ui-statik.sh", "--assets-only"),
            ("sh", "sdk/scripts/update-swagger-ui-statik.sh"), make,
        ):
            assert run(*command, check=False).returncode != 0
            assert archive.read_bytes() == previous
            assert shim.read_bytes() == original_shim
            assert not list((checkout / "docs").glob(".swagger-statik.*"))
    finally:
        statik.chmod(0o755)
    (checkout / "docs/swagger-ui/swagger.yml").write_bytes(b"")
    assert run("sh", "scripts/update-swagger-ui-statik.sh", "--assets-only", check=False).returncode != 0
    assert archive.read_bytes() == previous
    malformed = tmp_path / "malformed.go"
    malformed.write_bytes(previous[:100])
    assert run("python3", "scripts/swagger_assets.py", str(malformed), str(archive), check=False).returncode != 0
    assert archive.read_bytes() == previous
    assert shim.read_bytes() == original_shim
    assert not list(archive.parent.glob(".statik-*"))
    assert not list((checkout / "docs").glob(".swagger-statik.*"))


def test_other_intentional_copies_match_sources():
    for source, copy, comparison, _, _ in _listed_copies():
        _assert_copy(source, copy, comparison)


def _first_party_files():
    inventory = subprocess.run(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
        cwd=ROOT,
        check=True,
        capture_output=True,
    ).stdout
    paths = sorted({
        name.decode("utf-8", errors="surrogateescape")
        for name in inventory.split(b"\0") if name
    })
    assert paths, "No repository files inventoried"
    for name in paths:
        relative = Path(name)
        if name.startswith(UPSTREAM_TREES + NON_PRODUCT_TREES):
            continue
        if BUILD_DIRECTORIES.intersection(relative.parts) or "vendor" in relative.parts:
            continue
        if relative.name in DEPENDENCY_FILES or relative.suffix in {".pyc", ".pyo"}:
            continue
        path = ROOT / relative
        # A symlink serves its source without storing another artifact.
        if not path.is_symlink() and path.is_file():
            yield relative


def test_no_unlisted_byte_identical_copies_over_20kb():
    by_size = defaultdict(list)
    for path in _first_party_files():
        size = (ROOT / path).stat().st_size
        if size > MIN_COPY_BYTES:
            by_size[size].append(path.as_posix())
    allowed_edges = {
        frozenset((source, copy)) for source, copy, *_ in _listed_copies()
    }
    unlisted = []
    for paths in by_size.values():
        if len(paths) < 2:
            continue
        by_digest = defaultdict(list)
        for name in paths:
            with (ROOT / name).open("rb") as artifact:
                digest = hashlib.file_digest(artifact, "sha256").digest()
            by_digest[digest].append(name)
        for group in by_digest.values():
            if len(group) < 2:
                continue
            connected = {group[0]}
            while True:
                reached = {
                    name for name in group if any(
                        frozenset((name, known)) in allowed_edges for known in connected
                    )
                } - connected
                if not reached:
                    break
                connected.update(reached)
            if connected != set(group):
                unlisted.append(" | ".join(group))
    assert not unlisted, (
        "Byte-identical first-party artifacts over 20000 bytes need a source, copy, "
        "purpose and executing sync test in tests/ci/data/intentional_copies.txt:\n"
        + "\n".join(sorted(unlisted))
    )
