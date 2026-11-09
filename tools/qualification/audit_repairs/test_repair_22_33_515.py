"""Source qualification and in-process provenance checks; no backend/toolchain run."""

import ast
import hashlib
import importlib.util
import json
from pathlib import Path
import shlex
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
CLIENT = ROOT / "services/explorer/services/libs/blockscout-client"
RECORD = Path(__file__).with_name("repair_22_33_515.kvx")


def source(relative):
    return (CLIENT / relative).read_text()


def generator():
    spec = importlib.util.spec_from_file_location(
        "blockscout_generation", CLIENT / "scripts/generate.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_local_contract_documentation():
    readme = source("README.md")
    manifest = json.loads(source("package.json"))
    command = shlex.split(manifest["scripts"]["test:local"])
    assert command == [
        "cargo", "run", "--locked", "--manifest-path", "crate/Cargo.toml",
        "--example", "local_backend_contract", "--",
    ]
    assert (CLIENT / "crate/examples/local_backend_contract.rs").is_file()
    for fact in (
        "yarn test:local /private/healthy.json /private/empty.json",
        "BlockscoutClient.ContractState.export(",
        "running backend's BEAM", "HTTP 500", "HTTP 200", "5002",
        "source-only pytest", "no actual backend run is claimed",
        "yarn generate FULL_BACKEND_SCHEMA_GIT_SHA",
        "crate/generation-provenance.json",
        "yarn record-stubs http://127.0.0.1:4001",
        "metadata.source_revision",
    ):
        assert fact in readme, fact
    assert "writes every response with status 200" not in readme


def test_state_export_reads_backend_database_and_health():
    export = source("scripts/export_contract_state.exs")
    for fact in (
        "defmodule BlockscoutClient.ContractState", "def export(origin, output, source_revision)",
        "Explorer.Repo.all(", "from(block in Explorer.Chain.Block,",
        "where: block.consensus == true", "order_by: [desc: block.number]",
        "limit: 51", "if length(blocks) > 50",
        "BlockScoutWeb.API.HealthController.get_indexing_status()",
        "source_revision: source_revision", "health: health",
        "height: block.height", "hash: to_string(block.hash)",
        "parent_hash: to_string(block.parent_hash)", "Jason.encode!(state, pretty: true)",
    ):
        assert fact in export, fact
    for mutation in ("Repo.insert", "Repo.delete", "Repo.update", "Mox", "Bypass"):
        assert mutation not in export
    backend = ROOT / "services/explorer/backend/apps/block_scout_web/lib/block_scout_web"
    health = (backend / "controllers/api/health_controller.ex").read_text()
    blocks = (backend / "controllers/api/v2/block_controller.ex").read_text()
    view = (backend / "views/api/v2/block_view.ex").read_text()
    assert "def get_indexing_status do" in health
    assert "HealthHelper.get_indexing_health_data()" in health
    assert "blocks: indexing_status.blocks" in health
    assert "Chain.list_blocks()" in blocks
    assert '"height" => block.number' in view
    assert '"hash" => block.hash' in view
    assert '"parent_hash" => block.parent_hash' in view


def test_rust_contract_consumes_actual_http_and_typed_state():
    harness = source("crate/examples/local_backend_contract.rs")
    for fact in (
        "struct BackendState", "health: HealthBlocks", "blocks: Vec<ExpectedBlock>",
        "serde_json::from_slice(&fs::read(path).unwrap())",
        "ApiClient::new(Configuration::builder().base_path(&state.origin).build())",
        "client.health_api().health().await", "Err(Error::ResponseError(response))",
        "assert_eq!(response.status.as_u16(), 200)",
        "assert_eq!(response.status.as_u16(), 500)",
        "response.try_parse_as_success_model().unwrap()",
        "assert_eq!(response.try_as_success().unwrap(), &model)",
        "HealthError::Status500(typed) => assert_eq!(typed.as_ref(), &model)",
        "assert_eq!(model.healthy, Some(state.health.healthy))",
        "assert_eq!(model.error, state.health.error)",
        "assert_eq!(actual, &state.health)",
        "actual.latest_block.db.as_ref().unwrap().number.is_some()",
        "actual.latest_block.cache.as_ref().unwrap().number.is_some()",
        "actual.latest_block.node.is_some()",
        'assert_eq!(error.message, "There are no blocks in the DB.")',
        "assert_eq!(error.code, 5002)",
        'r#type: Some("block".into())',
        "assert_eq!(response.try_as_success().unwrap(), &page)",
        "assert!(page.next_page_params.is_null())",
        "assert_eq!(page.items.len(), state.blocks.len())",
        "assert_eq!(actual.height, Decimal::from(expected.height))",
        "assert_eq!(actual.hash, expected.hash)",
        "assert_eq!(actual.parent_hash, expected.parent_hash)",
        "assert!(!state.blocks.is_empty())", "assert!(state.blocks.is_empty())",
        "assert_eq!(healthy.source_revision, empty.source_revision)",
        "assert_ne!(healthy.origin, empty.origin)",
        "check_backend(&healthy, &mut evidence).await",
        "check_backend(&empty, &mut evidence).await",
        '"source_revision": state.source_revision', '"status": status.as_u16()',
        '"body": content', '"body": response.content',
    ):
        # rustfmt may split chained expressions and assertions across lines.
        assert "".join(fact.split()) in "".join(harness.split()), fact
    for replacement in ("TcpListener", "stubr", "mockall", "#[ignore]", "env::var("):
        assert replacement not in harness
    assert '"{}/api/health"' in source("crate/src/apis/health_api.rs")
    assert '"{}/api/v2/blocks"' in source("crate/src/apis/blocks_api.rs")
    assert "pub blocks: models::BlockIndexingStatus" in source("crate/src/models/health_check_response_metadata.rs")
    assert "pub latest_block:" in source("crate/src/models/block_indexing_status.rs")
    assert "pub items: Vec<models::Block>" in source("crate/src/models/get_blocks_200_response.rs")


def test_recorder_preserves_status_body_query_and_source_revision():
    recorder = source("ts-src/record-stubs.ts")
    for fact in (
        "export async function recordResponses(", "sourceRevision: string",
        "process.argv.slice(2)", "args.length < 3", "/^[0-9a-f]{40}$/",
        "['127.0.0.1', '[::1]'].includes(origin.hostname)",
        "origin.username || origin.password", "url.origin !== origin.origin",
        "redirect: 'error'", "timeout: 30000", "size: 8 * 1024 * 1024",
        "const body = await response.text()", "status: response.status",
        "source_revision: sourceRevision", "rest_origin: origin.origin",
        "url: url.pathname + url.search", "response.headers.get('content-type')",
        "{ 'Content-Type': contentType }", "flag: 'wx'", "process.exitCode = 1",
        "['/api/health', '/api/v2/blocks']",
    ):
        assert fact in recorder, fact
    assert "status: 200" not in recorder
    assert "eth.blockscout.com" not in recorder
    assert "response.json()" not in recorder
    assert "console.error" in recorder
    assert json.loads(source("package.json"))["scripts"]["record-stubs"] == "ts-node ts-src/record-stubs.ts"


def test_manifest_pin_matches_effective_executable():
    manifest = json.loads(source("package.json"))
    config = json.loads(source("openapitools.json"))
    version = config["generator-cli"]["version"]
    assert version == "7.6.0"
    assert config["generator-cli"]["useDocker"] is True
    command = shlex.split(manifest["scripts"]["generate"])
    assert command == [
        "python3", "scripts/generate.py", f"openapitools/openapi-generator-cli:v{version}"
    ]
    assert (CLIENT / command[1]).is_file()


def test_generation_provenance_records_real_schema_content():
    module = generator()
    image = shlex.split(json.loads(source("package.json"))["scripts"]["generate"])[2]
    revision = "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    inputs = module.generation_inputs(CLIENT, image, revision)
    assert inputs == {
        "generator_image": image,
        "generator_version": "7.6.0",
        "input_schema_revision": revision,
        "input_schema_sha256": hashlib.sha256(
            (CLIENT / "swaggers/blockscout-api-final.yaml").read_bytes()
        ).hexdigest(),
    }
    # Unit input for the validator; this does not claim Docker was executed.
    image_id = "sha256:" + "a" * 64
    qualified = module.qualify_generation(inputs, image_id, "7.6.0\n")
    assert qualified == {**inputs, "resolved_generator": image_id}


@pytest.mark.parametrize("image,revision", [
    ("openapitools/openapi-generator-cli", "a" * 40),
    ("openapitools/openapi-generator-cli:latest", "a" * 40),
    ("openapitools/openapi-generator-cli:v7.7.0", "a" * 40),
    ("openapitools/openapi-generator-cli:v7.6.0", "main"),
    ("openapitools/openapi-generator-cli:v7.6.0", ""),
])
def test_generator_rejects_pin_or_schema_revision_drift(image, revision):
    with pytest.raises(ValueError):
        generator().generation_inputs(CLIENT, image, revision)


@pytest.mark.parametrize("image_id,version", [
    ("openapitools/openapi-generator-cli:v7.6.0", "7.6.0"),
    ("sha256:short", "7.6.0"),
    ("sha256:" + "a" * 64, "7.7.0"),
])
def test_generator_rejects_unresolved_or_wrong_executable(image_id, version):
    with pytest.raises(ValueError):
        generator().qualify_generation({"generator_version": "7.6.0"}, image_id, version)


def test_generation_executes_resolved_id_and_writes_provenance_after_success():
    script = source("scripts/generate.py")
    tree = ast.parse(script)
    main = next(node for node in tree.body if isinstance(node, ast.FunctionDef) and node.name == "main")
    calls = [node for node in ast.walk(main) if isinstance(node, ast.Call)]
    external = [
        call for call in calls
        if isinstance(call.func, ast.Attribute)
        and isinstance(call.func.value, ast.Name)
        and call.func.value.id == "subprocess"
    ]
    assert len(external) == 4
    run_calls = [call for call in external if call.func.attr == "run"]
    assert len(run_calls) == 2
    assert all(any(k.arg == "check" and isinstance(k.value, ast.Constant) and k.value.value is True for k in call.keywords) for call in run_calls)
    docker_runs = [
        call for call in external
        if isinstance(call.args[0], ast.List)
        and isinstance(call.args[0].elts[1], ast.Constant)
        and call.args[0].elts[1].value == "run"
    ]
    assert len(docker_runs) == 2
    assert all(any(isinstance(item, ast.Name) and item.id == "resolved" for item in call.args[0].elts) for call in docker_runs)
    for fact in (
        '"docker", "image", "inspect", "--format", "{{.Id}}", args.image',
        "qualify_generation(inputs, resolved, version)",
        "inputs != generation_inputs(ROOT, args.image, args.schema_revision)",
        '"crate/generation-provenance.json"', "json.dumps(provenance, indent=2)",
        '"/local/openapi-generator-config.json"', '"/local/swaggers/blockscout-api-final.yaml"',
        '"/local/rust-template-handlebars"',
    ):
        assert fact in script, fact
    assert script.index('"crate/generation-provenance.json"') > script.index('"--type-mappings=integer=i64"')


def test_remote_refresh_propagates_revision_and_fails_closed():
    manifest = json.loads(source("package.json"))
    assert manifest["scripts"]["generate-from-remote"] == "bash scripts/generate-from-remote.sh"
    path = CLIENT / "scripts/generate-from-remote.sh"
    result = subprocess.run(["bash", "-n", str(path)], capture_output=True, text=True)
    assert result.returncode == 0, result.stderr
    script = path.read_text()
    assert "set -euo pipefail" in script
    assert "[[ $# -ne 1 || ! $1 =~ ^[0-9a-f]{40}$ ]]" in script
    assert "exit 2" in script
    assert ['yarn getV2', 'yarn merge', 'yarn generate "$1"', 'yarn format'] == [
        line for line in script.splitlines() if line.startswith("yarn ")
    ]


def test_closure_record_covers_both_findings_and_proof_limits():
    record = tomllib.loads(RECORD.read_text())
    assert set(record["finding"]) == {"client-contract-tests", "generator-pin"}
    declared_tests = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")
    }
    for finding in record["finding"].values():
        assert finding["tests"]
        assert set(finding["tests"]) <= declared_tests
        assert finding["source_proof"] and finding["review_only"]
    assert record["baseline"]["verify_runs"] == 0
    assert "implementation-first" in record["baseline"]["reason"]
    assert "not measured" in record["baseline"]["failures"]
    assert record["qualification"]["verify_command"] == "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_515.py"
    assert "domain 14" in record["wiring"]["local_backend"]
    assert "services/explorer/backend" in record["wiring"]["local_backend"]
    assert "not run" in record["qualification"]["operational_status"]
