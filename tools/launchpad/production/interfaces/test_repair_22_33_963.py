"""Exercise explicit authority bindings through the production file codecs.

The peers are committed SDK producer records and source inventories. Files are
the transport used by these offline clients; no provider or toolchain is needed.
"""

import contextlib
import copy
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import shutil
import sys
import tomllib
import unittest

import pytest
from jsonschema import Draft202012Validator, ValidationError


HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[3]
RECORDS = HERE / "testdata/repair_22_33_963"


def module(name):
    source = importlib.util.spec_from_file_location(name, HERE / (name + ".py"))
    loaded = importlib.util.module_from_spec(source)
    sys.modules[name] = loaded
    source.loader.exec_module(loaded)
    return loaded


capability = module("capability_check")
codegen = module("codegen")
compatibility = module("compatibility_check")
compatibility_suite = module("compatibility_check_test")
examples = module("examples_check")
examples_suite = module("examples_check_test")
materializer = module("materialize")
persistence = module("persistence_check")


def write_json(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n")


def snapshot(root):
    return {
        str(path.relative_to(root)): (path.read_bytes(), path.stat().st_mtime_ns)
        for path in root.rglob("*")
        if path.is_file()
    }


def run_suite(case):
    output = io.StringIO()
    result = unittest.TextTestRunner(stream=output).run(
        unittest.defaultTestLoader.loadTestsFromTestCase(case)
    )
    assert result.testsRun == len(unittest.defaultTestLoader.getTestCaseNames(case))
    assert result.testsRun > 0
    assert not result.skipped
    assert not result.expectedFailures
    assert result.wasSuccessful(), output.getvalue()


@pytest.fixture(scope="module")
def authority(tmp_path_factory):
    """Reopen a recorded producer selection using real SDK schemas and routes."""
    root = tmp_path_factory.mktemp("reviewed-source")
    bundle = root / "reviewed/contracts/v1"
    record = json.loads((RECORDS / "sdk-placement.json").read_text())
    for relative, expected in record["sources"].items():
        assert hashlib.sha256((ROOT / relative).read_bytes()).hexdigest() == expected
    write_json(bundle / "catalog.json", record["catalog"])
    write_json(bundle / "catalog-map.json", record["mapping"])
    write_json(bundle / "openapi.json", record["openapi"])
    shutil.copyfile(ROOT / record["schemaSource"], bundle / "schema-definitions.json")
    assert len(record["catalog"]["endpoint"]) == 108
    assert len(record["catalog"]["schema"]) == 59
    assert len(record["mapping"]["streams"]) == 12
    return root, bundle


def test_capability_authority_path(authority, tmp_path):
    root, bundle = authority
    document = json.loads((RECORDS / "capabilities.json").read_text())
    chain_bundle = root / "reviewed/chain-contracts/v1"
    chain_bundle.mkdir(parents=True)
    document["requiredArtifacts"] = [
        str((bundle / name).relative_to(root))
        for name in ("schema-definitions.json", "openapi.json", "asyncapi.json", "catalog-map.json")
    ] + [
        str((chain_bundle / "chain" / name).relative_to(root))
        for name in (
            "abi-catalog.json", "event-authorities.json", "golden-fixtures.json",
            "mainnet-attestation.json", "files-manifest.json",
        )
    ]
    document["catalogInputSha256"] = hashlib.sha256((bundle / "catalog.json").read_bytes()).hexdigest()
    for source in document["sources"]:
        if source.get("authority") == "workspace":
            target = root / source["path"]
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(ROOT / source["path"], target)
    write_json(bundle / "capabilities.json", document)
    authorities = {"workspace": root, "contracts": ROOT / "contracts/launchpad"}
    arguments = dict(
        require_materialized=False,
        bundle=bundle,
        chain_bundle=chain_bundle,
        catalog_input=bundle / "catalog.json",
    )
    spec = bundle / "catalog.json"
    assert capability.validate(document, root, spec, authorities, **arguments) == []
    assert capability.validate(document, root, spec, authorities) == [
        "explicit authoritative spec, bundle and chain bundle required"
    ]
    escaped = dict(arguments, bundle=tmp_path)
    assert capability.validate(document, root, spec, authorities, **escaped) == [
        "contract bundles must belong to source root"
    ]
    changed = copy.deepcopy(document)
    changed["sources"][0]["sha256"] = "0" * 64
    assert any("stale source hash" in error for error in capability.validate(changed, root, spec, authorities, **arguments))
    changed = copy.deepcopy(document)
    changed["requiredArtifacts"].pop()
    assert "required artifact set removed" in capability.validate(changed, root, spec, authorities, **arguments)
    changed = copy.deepcopy(document)
    changed["catalogInputSha256"] = "0" * 64
    assert "selected catalog source digest mismatch" in capability.validate(changed, root, spec, authorities, **arguments)
    before = snapshot(root)
    report = tmp_path / "capability-report.json"
    assert capability.main([
        "--check", "--root", str(root), "--spec", str(spec),
        "--bundle", str(bundle), "--chain-bundle", str(chain_bundle),
        "--catalog", str(spec), "--contracts-root", str(ROOT / "contracts/launchpad"),
        "--evidence-root", str(tmp_path / "evidence"), "--not-before", "1970-01-01T00:00:01Z",
        "--report", str(report),
    ]) == 1
    outcome = json.loads(report.read_text())
    assert outcome["passed"] is False
    assert outcome["releaseQualified"] is False
    assert "fresh run chain attestation and --not-before required" in outcome["errors"]
    assert any("missing required frozen artifact" in error for error in outcome["errors"])
    assert snapshot(root) == before


def selected_schema():
    """Take a dependency-closed slice of the committed production SDK schema."""
    document = json.loads((ROOT / "sdk/launchpad/packages/production-contracts/src/schemas.gen.json").read_text())
    selected = {}

    def collect(value):
        if isinstance(value, dict):
            if "$ref" in value:
                name = value["$ref"].removeprefix("#/$defs/")
                if name not in selected:
                    selected[name] = document["$defs"][name]
                    collect(selected[name])
            for child in value.values():
                collect(child)
        elif isinstance(value, list):
            for child in value:
                collect(child)

    for name in ("LoginBody", "QuoteRequest"):
        collect({"$ref": "#/$defs/" + name})
    return {"$schema": document["$schema"], "$defs": selected}


def codegen_arguments(root, source, manifest, major=2):
    return [
        "--root", str(root), "--schema", str(source), "--manifest", str(manifest),
        "--major", str(major), "--no-format",
    ]


def test_codegen_authority_path(tmp_path):
    source = tmp_path / "reviewed/contracts/v2/schema-definitions.json"
    manifest_path = source.with_name("codegen-manifest.json")
    write_json(source, selected_schema())
    args = codegen_arguments(tmp_path, source, manifest_path)
    assert codegen.main(args) == 0
    manifest = json.loads(manifest_path.read_text())
    assert manifest["schemaPath"] == str(source.relative_to(tmp_path))
    assert manifest["schemaSha256"] == hashlib.sha256(source.read_bytes()).hexdigest()
    assert manifest["schemaMajor"] == 2
    assert manifest["generator"] == "tools/launchpad/production/interfaces/codegen.py"
    assert len(manifest["outputs"]) == 4
    for relative, digest in manifest["outputs"].items():
        assert hashlib.sha256((tmp_path / relative).read_bytes()).hexdigest() == digest
    assert "type LoginBody" in (tmp_path / "sdk/launchpad/shared/interfaces/v2/types.gen.go").read_text()
    before = snapshot(tmp_path)
    assert codegen.main(args + ["--check"]) == 0
    assert snapshot(tmp_path) == before
    consumer_schema = json.loads((tmp_path / "sdk/launchpad/packages/production-contracts-v2/src/schemas.gen.json").read_text())
    samples = examples.Samples(consumer_schema)
    login = samples.sample({"$ref": "#/$defs/LoginBody"})
    samples.validate({"$ref": "#/$defs/LoginBody"}, json.loads(json.dumps(login)))
    del login["challengeId"]
    with pytest.raises(ValidationError):
        samples.validate({"$ref": "#/$defs/LoginBody"}, login)
    target = tmp_path / "sdk/launchpad/shared/interfaces/v2/types.gen.go"
    target.write_text(target.read_text() + "// drift\n")
    drifted = snapshot(tmp_path)
    assert codegen.main(args + ["--check"]) == 1
    assert snapshot(tmp_path) == drifted


def test_codegen_authority_traversal_and_version_protection(tmp_path):
    source = tmp_path / "reviewed/v2/schema-definitions.json"
    write_json(source, selected_schema())
    for manifest in (
        tmp_path / "unreviewed/codegen-manifest.json",
        source.parent / "../codegen-manifest.json",
        source.with_name("other.json"),
        tmp_path.parent / "codegen-manifest.json",
    ):
        before = snapshot(tmp_path)
        assert codegen.main(codegen_arguments(tmp_path, source, manifest)) == 1
        assert snapshot(tmp_path) == before
    for output in ("/tmp/output", "sdk/launchpad/shared/interfaces/../escaped", "outside/v2"):
        with pytest.raises(codegen.GenerationError):
            codegen.Generator(selected_schema(), major=2, go_out=output, format_output=False)
    v1 = tmp_path / "reviewed/v1/schema-definitions.json"
    write_json(v1, selected_schema())
    assert codegen.main(codegen_arguments(tmp_path, v1, v1.with_name("codegen-manifest.json"))) == 1
    escape = tmp_path / "outside-generated"
    escape.mkdir()
    link = tmp_path / "sdk/launchpad/shared/interfaces/v2"
    link.parent.mkdir(parents=True)
    link.symlink_to(escape, target_is_directory=True)
    before = snapshot(tmp_path)
    assert codegen.main(codegen_arguments(tmp_path, source, source.with_name("codegen-manifest.json"))) == 1
    assert snapshot(tmp_path) == before
    assert codegen.frozen_v1_generator() == codegen.FROZEN_V1_GENERATOR_SOURCE


def test_codegen_immutable_v1_reproduction(tmp_path):
    source = tmp_path / "reviewed/v1/schema-definitions.json"
    write_json(source, selected_schema())
    outputs = codegen.Generator(json.loads(source.read_text()), format_output=False).emit()
    frozen = {
        "schemaSha256": hashlib.sha256(source.read_bytes()).hexdigest(),
        "outputs": {path: hashlib.sha256(value.encode()).hexdigest() for path, value in outputs.items()},
    }
    for relative, content in outputs.items():
        target = tmp_path / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(content)
    manifest = source.with_name("codegen-manifest.json")
    write_json(manifest, frozen)
    args = codegen_arguments(tmp_path, source, manifest, major=1)
    before = snapshot(tmp_path)
    assert codegen.main(args + ["--check"]) == 0
    assert snapshot(tmp_path) == before
    source.write_text(source.read_text() + " ")
    before = snapshot(tmp_path)
    assert codegen.main(args) == 1
    assert snapshot(tmp_path) == before


def test_compatibility_authority_path(authority, capsys):
    _, bundle = authority
    manifest = RECORDS / "compatibility.json"
    args = [
        "--check", "--root", str(ROOT), "--manifest", str(manifest),
        "--schemas", str(bundle / "schema-definitions.json"), "--schemas-required",
    ]
    assert compatibility.main(args) == 0
    result = json.loads(capsys.readouterr().out)
    assert result["status"] == "passed"
    assert result["contracts"] == 10
    assert result["generated_schema_refs_checked"] is True
    with pytest.raises(SystemExit) as error:
        compatibility.main(["--check", "--root", str(ROOT)])
    assert error.value.code == 2


def test_compatibility_test_authority(authority):
    _, bundle = authority
    assert compatibility_suite.ROOT == ROOT
    compatibility_suite.bind_inputs(RECORDS / "compatibility.json", bundle / "schema-definitions.json")
    run_suite(compatibility_suite.CompatibilityFreezeTests)


def test_examples_authority_path(authority):
    root, bundle = authority
    args = ["--catalog", str(bundle / "catalog.json"), "--contracts", str(bundle)]
    examples.main(args + ["--write"])
    actual = json.loads((bundle / "examples.json").read_text())
    assert len(actual["endpoints"]) == 108
    assert actual["label"] == examples.LABEL
    before = snapshot(root)
    examples.main(args + ["--check"])
    assert snapshot(root) == before
    bad = copy.deepcopy(actual)
    bad["inputs"]["semanticCatalogSha256"] = "0" * 64
    write_json(bundle / "drift.json", bad)
    before = snapshot(root)
    with pytest.raises(examples.ExampleError, match="Stale example input hashes"):
        examples.main(args + ["--check", "--examples", str(bundle / "drift.json")])
    assert snapshot(root) == before
    duplicate = bundle / "duplicate.json"
    duplicate.write_bytes((bundle / "examples.json").read_bytes().replace(b'"IF_LOGIN":', b'"IF_LOGIN": {}, "IF_LOGIN":', 1))
    before = snapshot(root)
    with pytest.raises(examples.ExampleError, match="Duplicate JSON key"):
        examples.main(args + ["--check", "--examples", str(duplicate)])
    assert snapshot(root) == before
    with pytest.raises(SystemExit) as error:
        examples.main(["--check", "--contracts", str(bundle)])
    assert error.value.code == 2


def test_examples_test_authority(authority):
    _, bundle = authority
    examples_suite.bind_inputs(bundle, catalog=bundle / "catalog.json")
    run_suite(examples_suite.ExamplesTests)


def test_materializer_authority_path(tmp_path):
    source = RECORDS / "selected-catalog.kvx"
    supplementary = RECORDS / "supplementary.json"
    out = tmp_path / "reviewed/contracts/v1"
    args = [
        "--spec", str(source), "--out", str(out), "--frozen-v1-out", str(out),
        "--supplementary", str(supplementary),
    ]
    materializer.main(args)
    before = snapshot(tmp_path)
    materializer.main(args + ["--check"])
    assert snapshot(tmp_path) == before
    catalog = tomllib.loads(source.read_text())
    mapping = json.loads((out / "catalog-map.json").read_text())
    assert mapping["inputs"]["catalogSha256"] == materializer.semantic_digest(catalog)
    document = json.loads((out / "schema-definitions.json").read_text())
    samples = examples.Samples(document)
    request = samples.sample({"$ref": "#/$defs/StreamHeartbeatRequest"})
    samples.validate({"$ref": "#/$defs/StreamHeartbeatRequest"}, json.loads(json.dumps(request)))
    assert request.keys() == {"viewerToken", "playbackSessionId", "streamId"}
    request["unknownField"] = True
    with pytest.raises(ValidationError):
        samples.validate({"$ref": "#/$defs/StreamHeartbeatRequest"}, request)
    generated_source = out / "schema-definitions.json"
    generated_manifest = out / "codegen-manifest.json"
    # The immutable-v1 input is not silently overwritten by the v2 generator.
    assert codegen.main(codegen_arguments(tmp_path, generated_source, generated_manifest)) == 1
    changed = copy.deepcopy(catalog)
    changed["endpoint"]["IF_STREAM_HEARTBEAT"]["path"] = "/v2/streams/{streamId}/viewers:heartbeat"
    next_catalog = tmp_path / "next-catalog.json"
    write_json(next_catalog, changed)
    with pytest.raises(materializer.ContractError, match="cannot overwrite v1"):
        materializer.main([
            "--catalog", str(next_catalog), "--out", str(out), "--major", "2",
            "--frozen-v1-out", str(out), "--supplementary", str(supplementary),
        ])
    next_out = tmp_path / "reviewed/contracts/v2"
    materializer.main([
        "--catalog", str(next_catalog), "--out", str(next_out), "--major", "2",
        "--frozen-v1-out", str(out), "--supplementary", str(supplementary),
    ])
    assert json.loads((next_out / "openapi.json").read_text())["info"]["version"] == "2.0.0"
    assert codegen.main(codegen_arguments(tmp_path, next_out / "schema-definitions.json", next_out / "codegen-manifest.json")) == 0
    generated = json.loads((tmp_path / "sdk/launchpad/packages/production-contracts-v2/src/schemas.gen.json").read_text())
    Draft202012Validator.check_schema(generated)
    target = out / "openapi.json"
    target.write_text(target.read_text() + " ")
    before = snapshot(out)
    with pytest.raises(materializer.ContractError, match="missing or stale"):
        materializer.main(args + ["--check"])
    assert snapshot(out) == before
    with pytest.raises(SystemExit) as error:
        materializer.main(["--out", str(out), "--frozen-v1-out", str(out)])
    assert error.value.code == 2


def test_persistence_authority_path(capsys, tmp_path):
    register_path = RECORDS / "migration-register.json"
    runtime = ROOT / "sdk/launchpad/shared/interfaces/v1/persistence.go"
    register, histories, bound_runtime = persistence.load_inputs(register_path, runtime)
    assert bound_runtime == runtime
    assert len(histories) == 8
    assert len(register["ranges"]) == 50
    assert sum(len(history["current_migrations"]) for history in histories) > 8
    persistence.main([
        "--self-test", "--register", str(register_path), "--runtime-source", str(runtime),
    ])
    result = json.loads(capsys.readouterr().out)
    assert result == {"status": "pass", "identifier_rejections": 8, "docker_access": False}
    for field, value in (("sha256", "0" * 64), ("version", 2)):
        changed = copy.deepcopy(register)
        changed["histories"][0]["current_migrations"][0][field] = value
        with pytest.raises(ValueError, match="checksum mismatch|version must match"):
            persistence.validate_register(changed)
    changed = copy.deepcopy(register)
    changed["runtime_role_policy"]["superuser"] = True
    with pytest.raises(ValueError, match="runtime role contract changed"):
        persistence.validate_register(changed)
    with pytest.raises(ValueError, match="runtime persistence source"):
        persistence.load_inputs(register_path, tmp_path / "absent.go")
    with pytest.raises(ValueError, match="migration register"):
        persistence.load_inputs(tmp_path / "absent.json", runtime)
    with pytest.raises(SystemExit) as error:
        persistence.main(["--self-test", "--register", str(register_path)])
    assert error.value.code == 2
    denial = persistence.denied("CREATE TABLE public.denied(id integer)")
    assert "WHEN insufficient_privilege THEN NULL" in denial
    assert "privilege violation: unexpected success" in denial


@pytest.mark.parametrize("client,args", [
    (capability.main, ["--check", "--evidence-root", ".", "--not-before", "1"]),
    (codegen.main, ["--check"]),
    (examples.main, ["--check"]),
    (compatibility_suite.main, []),
    (examples_suite.main, []),
])
def test_missing_authority_is_explicit(client, args):
    with pytest.raises(SystemExit) as error:
        client(args)
    assert error.value.code == 2
