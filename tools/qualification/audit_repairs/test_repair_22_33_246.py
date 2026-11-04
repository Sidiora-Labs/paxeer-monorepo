"""Python-only source qualification for audit 22.30.129 finding 014.

The Node/npm packed-consumer regression is committed, but is not executed by
this gate: it requires the authoring toolchain excluded by the task contract.
"""

import ast
import json
from pathlib import Path, PurePosixPath
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
SDK = ROOT / "programs/sdk/assemblyscript"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_246.kvx"


def read(relative):
    return (SDK / relative).read_text(encoding="utf-8")


def package():
    return json.loads(read("package.json"))


def test_014_package_ships_the_canonical_manifest():
    manifest = package()
    assert manifest["name"] == "@sidiora/layerx-program-sdk"
    assert manifest["private"] is False
    assert manifest["bin"] == {
        "layerx-determinism-lint": "tools/determinism-lint.mjs",
    }
    assert manifest["exports"]["./determinism-lint"] == "./tools/determinism-lint.mjs"
    assert manifest["files"] == [
        "assembly", "tools", "abi-manifest.tsv", "asconfig.json",
        "toolchain.json", "tsconfig.json",
    ]
    for entry in manifest["files"]:
        assert (SDK / entry).exists(), entry
    bundled = (SDK / "abi-manifest.tsv").read_bytes()
    assert bundled == (ROOT / "programs/sdk/abi-manifest.tsv").read_bytes()
    assert bundled.endswith(b"\n")
    rows = [line.split("\t") for line in bundled.decode("utf-8").splitlines()]
    assert len(rows) == 28
    versions = []
    names = set()
    for version, module, name, signature in rows:
        assert re.fullmatch(r"[1-4]", version)
        assert module == f"layerx_v{version}"
        assert re.fullmatch(r"[a-z][a-z0-9_]*", name)
        assert re.fullmatch(r"\((?:(?:i32|i64)(?:,(?:i32|i64))*)?\)->(?:i32|i64)", signature)
        assert (module, name) not in names
        names.add((module, name))
        versions.append(int(version))
    assert versions == sorted(versions)
    assert set(versions) == {1, 2, 3, 4}
    assert rows[0] == ["1", "layerx_v1", "storage_read", "(i32,i32,i32,i32)->i32"]


def test_014_default_manifest_resolves_within_the_installed_publish_set():
    source = read("tools/determinism-lint.mjs")
    main = source.split("function main(argv) {", 1)[1]
    paths = re.findall(r'new URL\("([^"]+)", import\.meta\.url\)', main)
    assert paths == ["../abi-manifest.tsv"]
    # Resolve from the installed bin, independently of the repository and cwd.
    installed = PurePosixPath("/consumer/node_modules/@sidiora/layerx-program-sdk")
    bin_path = installed / package()["bin"]["layerx-determinism-lint"]
    segments = list(bin_path.parent.parts)
    for segment in PurePosixPath(paths[0]).parts:
        if segment == "..":
            segments.pop()
        elif segment != ".":
            segments.append(segment)
    resolved = PurePosixPath(*segments)
    assert resolved == installed / "abi-manifest.tsv"
    assert resolved.relative_to(installed).as_posix() in package()["files"]
    assert "process.cwd" not in main
    assert "../../abi-manifest.tsv" not in source
    assert 'argv[index] === "--abi-manifest" && index + 1 < argv.length' in main
    assert "manifest = argv[index + 1];" in main
    assert "try { abiFunctions = loadManifest(manifest); }" in main
    failure = main.split("catch (error) {", 1)[1].split("}", 1)[0]
    # The interpolated diagnostic contains a brace, so check the whole handler
    # separately for its terminal failure rather than treating it as JS syntax.
    assert "ABI manifest:" in failure
    assert re.search(r"catch \(error\) \{\s*process\.stderr\.write\([^\n]+\);\s*return 2;\s*\}", main)


def test_014_manifest_reaches_both_production_lint_consumers():
    source = read("tools/determinism-lint.mjs")
    loader = source.split("function loadManifest(path) {", 1)[1].split("class Refusal", 1)[0]
    for fact in (
        'readFileSync(path, "utf8")',
        'if (!text.endsWith("\\n")) refuse(RULE_MALFORMED, "unterminated ABI manifest")',
        'if (functions.length !== 28 || previous !== 4) refuse(RULE_MALFORMED, "incomplete ABI manifest")',
        "return { introduced, module: fields[1], name: fields[2], signature: fields[3] };",
        "return functions;",
    ):
        assert fact in loader
    permitted = source.split("function permittedImport(module, name) {", 1)[1].split("function valueTypeName", 1)[0]
    assert "abiFunctions.find((entry) => entry.introduced <= selectedAbiVersion" in permitted
    assert "entry.module === module && entry.name === name" in permitted
    assert "let selectedAbiVersion = 1;" in source
    imports = source.split("function checkImportSection(reader, limit) {", 1)[1].split("function checkFunctionSection", 1)[0]
    assert "const functionDeclaration = permittedImport(module, name);" in imports
    assert 'if (kind !== 0x00 || !functionDeclaration)' in imports
    assert 'refuse(RULE_FORBIDDEN_IMPORT, `${module}::${name}`);' in imports
    assert "reader.types[typeIndex] !== functionDeclaration.signature" in imports
    assert 'refuse(RULE_IMPORT_SIGNATURE, `${module}::${name}`);' in imports
    check_source = source.split("function checkSource(text) {", 1)[1].split("function collectPaths", 1)[0]
    assert "if (!permittedImport(match[1], match[2]))" in check_source
    assert "refuse(RULE_IMPORT_DECLARATION" in check_source
    dispatch = source.split("function lintPath(path) {", 1)[1].split("function main", 1)[0]
    assert 'if (extname(path) === ".wasm") {' in dispatch
    assert "checkModule(new Uint8Array(bytes));" in dispatch
    assert 'checkSource(bytes.toString("utf8"));' in dispatch
    assert "process.exitCode = main(process.argv.slice(2));" in source


def test_014_packed_consumer_regression_drives_the_real_installed_bin():
    assert shlex.split(package()["scripts"]["test:package"]) == [
        "node", "--test", "tests/packed-package.test.mjs",
    ]
    source = read("tests/packed-package.test.mjs")
    assert "function installedLintUsesBundledManifestForSourceAndWasm(t) {" in source
    assert "test(installedLintUsesBundledManifestForSourceAndWasm);" in source
    for fact in (
        'mkdtempSync(join(tmpdir(), "layerx-sdk-package-"))',
        'for (const entry of ["package.json", ...manifest.files])',
        'cpSync(join(sdk, entry), join(producer, entry), { recursive: true });',
        'JSON.parse(execFileSync("npm", [',
        '"pack", "--offline", "--ignore-scripts", "--json", "--pack-destination", workspace',
        'packed.files.some((entry) => entry.path === "abi-manifest.tsv")',
        '"install", "--offline", "--ignore-scripts", "--no-audit", "--no-fund",',
        '"--package-lock=false", "--omit=dev", join(workspace, packed.filename)',
        'rmSync(producer, { recursive: true });',
        'join(consumer, "node_modules", "@sidiora", "layerx-program-sdk")',
        'assert.deepEqual(readFileSync(bundledManifest), readFileSync(join(sdk, "abi-manifest.tsv")))',
        'assert.equal(existsSync(join(consumer, "node_modules", "@sidiora", "abi-manifest.tsv")), false)',
        'join(consumer, "node_modules", ".bin", "layerx-determinism-lint")',
        'assert.equal(realpathSync(bin), join(installed, manifest.bin["layerx-determinism-lint"]))',
        'cpSync(new URL("fixtures/packed-package/guest.ts", import.meta.url), source)',
        'compiler, source, "--config", join(installed, "asconfig.json"), "--outFile", wasm',
        'assert.ok(WebAssembly.validate(readFileSync(wasm)))',
        'WebAssembly.Module.imports(module)',
        '{ module: "layerx_v1", name: "storage_read", kind: "function" }',
        'entry.name === "memory" && entry.kind === "memory"',
        'spawnSync(bin, [source, wasm], { cwd: consumer, encoding: "utf8" })',
        'assert.ifError(passed.error)',
        'assert.equal(passed.status, 0, passed.stderr)',
        'assert.equal(passed.stderr, "")',
        'assert.equal(passed.stdout, `determinism-lint: ${source}: passed\\ndeterminism-lint: ${wasm}: passed\\n`)',
        'assert.equal(refused.status, 1)',
        'assert.match(refused.stderr, /forbidden-import-declaration: layerx_v2::response_write/)',
        'compiler, wrongSource, "--config", join(installed, "asconfig.json"), "--outFile", wrongWasm',
        'assert.equal(wrongSignature.status, 1)',
        'assert.match(wrongSignature.stderr, /wrong-import-signature: layerx_v1::storage_read/)',
        'rmSync(bundledManifest)',
        'assert.equal(missing.status, 2)',
        'assert.match(missing.stderr, /ABI manifest: .*ENOENT/)',
        'assert.equal(corrupt.status, 2)',
        'assert.match(corrupt.stderr, /ABI manifest: malformed-module: invalid ABI manifest row/)',
    ):
        assert fact in source, fact
    assert "--abi-manifest" not in source
    assert "--abi-version" not in source
    assert not re.search(r"\b(?:skip|todo|mock)\s*[:.(]", source)
    guest = read("tests/fixtures/packed-package/guest.ts")
    assert '@external("layerx_v1", "storage_read")' in guest
    assert "declare function storageRead(key: i32, keyLength: i32, output: i32, capacity: i32): i32;" in guest
    assert "return storageRead(key, keyLength, output, capacity);" in guest
    config = json.loads(read("asconfig.json"))["options"]
    assert config["runtime"] == "stub"
    assert config["use"] == ["abort=", "trace=", "seed="]
    assert config["importMemory"] is False


def test_014_record_maps_proof_and_discloses_execution_limits():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["task"] == "22.33.246"
    assert repair["base_revision"] == "8a5302e1b13ed685ab2cd21b29cd69052c2a8f6a"
    assert repair["build_cmd"] == "python3 --version"
    assert repair["verify_cmd"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_246.py"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "not executed" in repair["review_only"]
    assert "npm run test:package" in repair["review_only"]
    baseline = record["baseline"]
    assert baseline["verification_run"] is False
    assert baseline["observed_failing_tests"] == []
    assert "explicit task order" in baseline["reason"]
    continuation = record["continuation"]
    assert continuation["original_base_revision"] == "0ba7cfa54c7f07b37201b1f12fcc2f1dd7ed0ed7"
    assert continuation["previous_verified_revision"] == "e17049be2cce7644354bddf89643b1873275d02e"
    assert "build_on_verified" in continuation["previous_result"]
    finding = record["finding"]["014"]
    assert finding["audit"] == "22.30.129"
    assert finding["file"] == "programs/sdk/assemblyscript/package.json"
    assert finding["class"] == "wiring-gap"
    assert finding["severity"] == "medium"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text(encoding="utf-8")).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared)
    assert finding["runtime_regression"] == "installedLintUsesBundledManifestForSourceAndWasm"
    requests = record["wiring"]["requests"]
    assert any("Domain 13" in request and "programs/sdk/abi-manifest.tsv" in request for request in requests)
    assert any("Domain 9" in request and ".github/workflows/" in request
               and "npm run test:package" in request for request in requests)
