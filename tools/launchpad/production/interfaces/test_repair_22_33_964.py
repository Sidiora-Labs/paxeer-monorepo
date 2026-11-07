"""Run original regressions through the production file codecs.

Committed SDK producer records supply explicit source authorities. The complete
materializer suite derives its input from the admitted operation selection and
checks every schema assertion through real generation and catalog replay.
"""

import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import tomllib
import unittest

import pytest
from jsonschema import Draft202012Validator, ValidationError


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
RECORDS = HERE / "testdata/repair_22_33_963"


def load_suite_module(name):
    spec = importlib.util.spec_from_file_location(name, HERE / (name + ".py"))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def assert_suite_passes(case):
    suite = unittest.defaultTestLoader.loadTestsFromTestCase(case)
    expected = suite.countTestCases()
    assert expected > 0
    result = unittest.TestResult()
    suite.run(result)
    details = "\n".join(trace for _, trace in result.errors + result.failures)
    assert result.wasSuccessful(), details
    assert result.testsRun == expected
    assert not result.skipped
    assert not result.expectedFailures
    assert not result.unexpectedSuccesses


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")


def test_capability_test_authority():
    module = load_suite_module("test_capability_check")
    assert module.SPEC == ROOT / "spec/layerx-merge/spec.kvx"
    assert module.CONTRACTS == ROOT / "contracts/launchpad"
    record = json.loads((RECORDS / "sdk-placement.json").read_bytes())
    for relative, expected in record["sources"].items():
        assert hashlib.sha256((ROOT / relative).read_bytes()).hexdigest() == expected
    # This is an explicit recorded source-only selection, never runtime proof.
    with tempfile.TemporaryDirectory(prefix="capability-authority-", dir=HERE) as directory:
        bundle = Path(directory) / "contracts/v1"
        evidence = Path(directory) / "evidence"
        evidence.mkdir(parents=True)
        not_before = evidence.stat().st_mtime
        catalog = bundle / "catalog.json"
        write_json(catalog, record["catalog"])
        schema = ROOT / record["schemaSource"]
        definitions = json.loads(schema.read_bytes())["$defs"]
        shutil.copyfile(schema, bundle / "schema-definitions.json")
        mapping = copy.deepcopy(record["mapping"])
        # Resolve each recorded catalog ID against the actual SDK declarations;
        # ambiguity or absence is a hard failure, never a generic definition.
        for schema_id in record["catalog"]["schema"]:
            key = schema_id.removeprefix("IF_").replace("_", "").lower()
            names = [name for name in definitions if name.lower() == key]
            assert len(names) == 1, (schema_id, names)
            name = names[0]
            mapping["schemas"][schema_id] = {
                "wireName": name, "ref": "schema-definitions.json#/$defs/" + name,
            }
        write_json(bundle / "catalog-map.json", mapping)
        write_json(bundle / "openapi.json", record["openapi"])
        document = json.loads((RECORDS / "capabilities.json").read_bytes())
        document["catalogInputSha256"] = hashlib.sha256(catalog.read_bytes()).hexdigest()
        document["requiredArtifacts"] = [
            str((bundle / name).relative_to(ROOT))
            for name in ("schema-definitions.json", "openapi.json", "asyncapi.json", "catalog-map.json")
        ] + [
            str((bundle / "chain" / name).relative_to(ROOT))
            for name in ("abi-catalog.json", "event-authorities.json", "golden-fixtures.json",
                         "mainnet-attestation.json", "files-manifest.json")
        ]
        write_json(bundle / "capabilities.json", document)
        module.bind_inputs(bundle, catalog, bundle, evidence, not_before)
        assert_suite_passes(module.CapabilityTests)
        changed = copy.deepcopy(document)
        changed["catalogInputSha256"] = "0" * 64
        errors = module.check.validate(
            changed, ROOT, module.SPEC,
            {"workspace": ROOT, "contracts": module.CONTRACTS, "evidence": evidence},
            require_materialized=False, bundle=bundle, chain_bundle=bundle,
            catalog_input=catalog, not_before=not_before,
        )
        assert "selected catalog source digest mismatch" in errors
        before = {path: path.read_bytes() for path in bundle.rglob("*") if path.is_file()}
        report = Path(directory) / "report.json"
        assert module.check.main([
            "--check", "--root", str(ROOT), "--spec", str(module.SPEC),
            "--bundle", str(bundle), "--catalog", str(catalog), "--chain-bundle", str(bundle),
            "--contracts-root", str(module.CONTRACTS), "--evidence-root", str(evidence),
            "--not-before", "1970-01-01T00:00:01Z", "--report", str(report),
        ]) == 1
        result = json.loads(report.read_bytes())
        assert result["passed"] is False
        assert result["releaseQualified"] is False
        assert "fresh run chain attestation and --not-before required" in result["errors"]
        assert {path: path.read_bytes() for path in bundle.rglob("*") if path.is_file()} == before


def test_codegen_drift_test_target():
    assert_suite_passes(load_suite_module("test_codegen").CodegenTests)


def test_materializer_recorded_file_path(tmp_path):
    module = load_suite_module("test_materialize")
    out = tmp_path / "contracts/v1"
    args = [
        "--spec", str(RECORDS / "selected-catalog.kvx"),
        "--supplementary", str(RECORDS / "supplementary.json"),
        "--out", str(out), "--frozen-v1-out", str(out),
    ]
    module.m.main(args)
    before = {path: path.read_bytes() for path in out.rglob("*") if path.is_file()}
    module.m.main(args + ["--check"])
    assert {path: path.read_bytes() for path in out.rglob("*") if path.is_file()} == before
    # Consume the catalog the real spec producer just emitted, then reopen a
    # separate output bundle and compare every artifact without substituting
    # the selected fixture for the full-suite authority below.
    catalog = out / "inputs/catalog.json"
    replay = tmp_path / "replayed/v1"
    replay_args = [
        "--catalog", str(catalog),
        "--supplementary", str(RECORDS / "supplementary.json"),
        "--out", str(replay), "--frozen-v1-out", str(replay),
    ]
    module.m.main(replay_args)
    module.m.main(replay_args + ["--check"])
    assert {
        out / path.relative_to(replay): path.read_bytes()
        for path in replay.rglob("*") if path.is_file()
    } == before
    selected = tomllib.loads((RECORDS / "selected-catalog.kvx").read_text())
    assert json.loads(catalog.read_bytes()) == module.m.semantic_input(selected)
    definitions = json.loads((out / "schema-definitions.json").read_bytes())
    examples = load_suite_module("examples_check")
    samples = examples.Samples(definitions)
    request = samples.sample({"$ref": "#/$defs/StreamHeartbeatRequest"})
    validator = Draft202012Validator({
        "$ref": "#/$defs/StreamHeartbeatRequest", "$defs": definitions["$defs"],
    })
    validator.validate(json.loads(json.dumps(request)))
    request["unknownField"] = True
    with pytest.raises(ValidationError):
        validator.validate(request)
    with pytest.raises(module.m.ContractError, match="cannot overwrite v1"):
        module.m.main(args + ["--major", "2"])
    assert {path: path.read_bytes() for path in out.rglob("*") if path.is_file()} == before


def test_materializer_test_authority(tmp_path):
    module = load_suite_module("test_materialize")
    assert module.SPEC is None
    assert module.CATALOG is None
    assert module.SUPPLEMENTARY is None
    assert module.OUT == ROOT / "sdk/launchpad/protocol/contracts/v1"
    catalog_input, supplementary = module.derive_authority(tmp_path)
    selected = json.loads(catalog_input.read_bytes())
    sections = selected["sections"]
    assert tuple(len(sections[key]) for key in ("schema", "endpoint", "stream", "error")) == (59, 108, 12, 16)
    record = json.loads(module.RECORD.read_bytes())
    assert set(sections["endpoint"]) == set(record["catalog"]["endpoint"])
    for name, endpoint in sections["endpoint"].items():
        recorded = record["mapping"]["endpoints"][name]
        assert endpoint["path"] == recorded["path"].replace("/v2/", "/v1/", 1)
        assert endpoint["method"] == recorded["method"]
    out = tmp_path / "authority/v1"
    args = [
        "--catalog", str(catalog_input), "--supplementary", str(supplementary),
        "--out", str(out), "--frozen-v1-out", str(out),
    ]
    module.m.main(args)
    module.m.main(args + ["--check"])
    catalog = out / "inputs/catalog.json"
    assert json.loads(catalog.read_bytes()) == module.m.semantic_input(sections)
    before = {path: path.read_bytes() for path in out.rglob("*") if path.is_file()}
    # Default setUpClass must derive the same complete authority in a clean root.
    assert_suite_passes(module.MaterializeTests)
    derived_files = module.MaterializeTests.files
    module.bind_inputs(supplementary, catalog=catalog)
    assert_suite_passes(module.MaterializeTests)
    assert module.MaterializeTests.files == derived_files
    assert {
        str(path.relative_to(out)): content for path, content in before.items()
    } == derived_files
    assert {path: path.read_bytes() for path in out.rglob("*") if path.is_file()} == before
    assert module.main(["--catalog", str(catalog), "--supplementary", str(supplementary)]) == 0


def test_materializer_source_digest_is_required(tmp_path):
    module = load_suite_module("test_materialize")
    record = json.loads(module.RECORD.read_bytes())
    source = record["schemaSource"]
    record["sources"][source] = "0" * 64
    altered = tmp_path / "altered-record.json"
    write_json(altered, record)
    before = {path: path.read_bytes() for path in tmp_path.rglob("*") if path.is_file()}
    with pytest.raises(module.m.ContractError, match="Recorded SDK source digest mismatch"):
        module.derive_authority(tmp_path, record_path=altered)
    assert {path: path.read_bytes() for path in tmp_path.rglob("*") if path.is_file()} == before


def test_suite_commands_require_explicit_authority():
    for name in ("test_capability_check", "test_materialize"):
        module = load_suite_module(name)
        with pytest.raises(SystemExit) as error:
            module.main([])
        assert error.value.code == 2
