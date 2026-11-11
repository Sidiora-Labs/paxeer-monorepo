"""Offline workflow/source proofs and real retention/checksum helper execution.

No image, native build, live API, credential or subprocess toolchain is used.
Bash is invoked only with -n to parse the production shell as required by the task.
"""

import ast
import base64
from contextlib import contextmanager
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import importlib.util
import io
import json
from pathlib import Path
import re
import shlex
import subprocess
import threading
import tomllib

import pytest
import yaml

ROOT = Path(__file__).resolve().parents[3]
WORKFLOWS = ROOT / ".github/workflows"
RECORD = Path(__file__).with_name("repair_22_33_699.kvx")


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def workflow(name):
    return yaml.load((WORKFLOWS / name).read_text(encoding="utf-8"), Loader=yaml.BaseLoader)


def steps(name, job):
    return workflow(name)["jobs"][job]["steps"]


def step_named(name, job, label):
    return next(step for step in steps(name, job) if step.get("name") == label)


def shell_syntax(script):
    # GitHub substitutes these expressions before handing scripts to Bash.
    script = re.sub(r"\$\{\{.*?\}\}", "workflow_value", script, flags=re.S)
    result = subprocess.run(
        ["bash", "--noprofile", "--norc", "-n"], input=script,
        text=True, capture_output=True, check=False,
    )
    assert result.returncode == 0, result.stderr


def helper(name):
    path = WORKFLOWS / "scripts" / (name + ".py")
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def test_node_pr_input_coverage():
    config = workflow("node-build.yml")
    assert config["on"]["pull_request"] == ""
    assert config["on"]["push"]["branches"] == ["main"]
    assert set(config["jobs"]) == {"native-libs", "paxd", "lxphost", "node-tests", "node-build-gate"}
    commands = {}
    for job, data in config["jobs"].items():
        commands[job] = [step["run"] for step in data["steps"] if "run" in step]
        for command in commands[job]:
            shell_syntax(command)
    for job in ("native-libs", "paxd", "lxphost", "node-tests"):
        joined = "\n".join(commands[job])
        for prerequisite in ("build-essential", "clang", "libssl-dev", "libsqlite3-dev", "rustup toolchain install 1.91.1"):
            assert prerequisite in joined
    assert "make paxd-native-libs" in commands["native-libs"]
    assert "make paxd-build" in commands["paxd"]
    assert "make paxd-version-check" in commands["paxd"]
    assert "make -f chain.mk paxd-native-digest-check" in commands["paxd"]
    assert "make test-lxphost" in commands["lxphost"]
    tests = step_named("node-build.yml", "node-tests", "Test node packages")
    assert tests["run"] == "go test -timeout 15m -count=1 ./node/..."
    assert tests["env"]["CGO_ENABLED"] == "1"
    assert commands["node-tests"].index("make paxd-native-libs") < commands["node-tests"].index("go test -timeout 15m -count=1 ./node/...")
    gate = config["jobs"]["node-build-gate"]
    assert gate["if"] == "always()"
    assert set(gate["needs"]) == {"native-libs", "paxd", "lxphost", "node-tests"}
    assert 'if [ "$result" != success ]; then' in "\n".join(commands["node-build-gate"])
    assert 'exit "$status"' in "\n".join(commands["node-build-gate"])


def image_contract(name, job, expected, reference):
    builds = [step["with"] for step in steps(name, job) if step.get("uses", "").startswith("docker/build-push-action@")]
    assert len(builds) == len(expected)
    for build, (tag, tags) in zip(builds, expected):
        assert build["context"] == "."
        assert build["file"] == "docker/layerx/Dockerfile"
        assert build["platforms"] == "linux/amd64"
        assert build["push"] == "true"
        assert build["tags"] == "${{ steps.login-ecr.outputs.registry }}/pax/pax-chain:" + tag
        args = dict(line.split("=", 1) for line in build["build-args"].splitlines() if line)
        assert args == dict(PAX_CHAIN_REF=reference, **({"GO_BUILD_TAGS": tags} if tags else {}))
    recipe = source("docker/layerx/Dockerfile").replace("\\\n", " ")
    directives = [line.strip().split(None, 1) for line in recipe.splitlines() if line and not line.startswith("#")]
    assert ["ARG", 'PAX_CHAIN_REF=""'] in directives
    assert ["ARG", 'GO_BUILD_TAGS=""'] in directives
    assert ["ENV", "CGO_ENABLED=1"] in directives
    assert ["COPY", "--from=native /out/ ./"] in directives
    assert ["COPY", "--from=builder /go/bin/paxd /usr/bin/"] in directives
    assert ["COPY", "--from=builder /go/lib/*.so /usr/lib/"] in directives
    assert ["ENTRYPOINT", '["/usr/bin/paxd"]'] in directives
    assert 'BUILD_TAGS="netgo ledger ${GO_BUILD_TAGS}"' in recipe
    assert '-X ${VERSION_PKG}.Commit=${PAX_CHAIN_REF:-unknown}' in recipe
    assert 'go build -tags "${BUILD_TAGS}"' in recipe
    assert '-o /go/bin/paxd ./daemon/paxd' in recipe
    assert 'NativeArchiveDigest=$(cat build/liblayerx.a programs/target/release/liblayerx_programs_sandbox.a | sha256sum' in recipe
    assert 'sha256sum --check --strict libwasmvm.sha256' in recipe


def test_ecr_build_file():
    suffix = "${{ inputs.tag || inputs.ref || github.sha }}"
    image_contract("paxeer-ecr.yml", "publisher", [
        ("mock-" + suffix, "mock_balances"),
        ("mock_block_validation-" + suffix, "mock_block_validation"),
        ("mock_chain_validation-" + suffix, "mock_balances mock_chain_validation"),
        (suffix, ""),
    ], "${{ inputs.ref || github.sha }}")
    checkout = steps("paxeer-ecr.yml", "publisher")[0]
    assert checkout["with"]["ref"] == "${{ inputs.ref || github.sha }}"


def test_nightly_build_file():
    image_contract("paxeer-nightly-ecr.yml", "publish", [
        ("${{ steps.tag.outputs.nightly }}", ""),
        ("${{ steps.tag.outputs.mock }}", "mock_balances"),
        ("${{ steps.tag.outputs.mock_chain }}", "mock_balances mock_chain_validation"),
    ], "${{ steps.tag.outputs.sha }}")
    tag = step_named("paxeer-nightly-ecr.yml", "publish", "Compute tag components")["run"]
    shell_syntax(tag)
    for output in ("nightly=nightly-${DATE}-${SHA7}", "mock=mock-nightly-${DATE}-${SHA7}", "mock_chain=mock_chain_validation-nightly-${DATE}-${SHA7}"):
        assert output in tag
    assert 'SHA=$(git rev-parse HEAD)' in tag
    assert steps("paxeer-nightly-ecr.yml", "publish")[0]["with"]["ref"] == "main"


def test_ghcr_cleanup_owner_wiring():
    registry = helper("integration_registry")
    config = registry.registry_config()
    assert config["owner"] == "sidiora-labs"
    for job in ("prepare-cluster", "integration-tests"):
        job_steps = steps("paxeer-integration-test.yml", job)
        configure = next(i for i, step in enumerate(job_steps) if step.get("run") == "python3 .github/workflows/scripts/integration_registry.py environment")
        login = next(i for i, step in enumerate(job_steps) if step.get("name") == "Login to GitHub Container Registry")
        assert configure < login
        assert job_steps[login]["with"] == {
            "registry": "ghcr.io", "username": "${{ env.GHCR_OWNER }}", "password": "${{ secrets.GITHUB_TOKEN }}"
        }
    publisher = step_named("paxeer-integration-test.yml", "prepare-cluster", "Push images to GHCR for test jobs")["run"]
    consumer = step_named("paxeer-integration-test.yml", "integration-tests", "Load prebuilt paxd and pull Docker images")["run"]
    for role in ("LOCALNODE", "RPCNODE"):
        assert 'docker push "${GHCR_' + role + '_RUN_IMAGE}"' in publisher
        assert 'docker pull "${GHCR_' + role + '_RUN_IMAGE}"' in consumer
    for script in (publisher, consumer):
        shell_syntax(script)
    cleanup = workflow("paxeer-ghcr-integration-test-cleanup.yml")
    assert cleanup["jobs"]["cleanup"]["permissions"] == {"contents": "read", "actions": "read", "packages": "write"}
    assert cleanup["on"]["workflow_dispatch"]["inputs"]["dry_run"]["default"] == "true"
    run = cleanup["jobs"]["cleanup"]["steps"][-1]
    assert run["env"]["DRY_RUN"] == "${{ github.event_name != 'workflow_dispatch' || inputs.dry_run }}"
    assert run["env"]["GH_TOKEN"] == "${{ secrets.GITHUB_TOKEN }}"
    assert run["run"] == "python3 .github/workflows/scripts/integration_registry.py cleanup"
    for event in ("push", "pull_request"):
        assert ".github/workflows/integration-registry.json" in workflow("paxeer-integration-test.yml")["on"][event]["paths"]
    with pytest.raises(ValueError, match="owner differs"):
        registry.image_environment(config, "unrelated-org/repository", "12345")
    for run_id in ("", "cache", "12/34", "0", "1\n2"):
        with pytest.raises(ValueError, match="run id"):
            registry.image_environment(config, "Sidiora-Labs/Paxeer-X-Network", run_id)


@contextmanager
def registry_server(documents):
    """Serve committed GitHub API response documents over real HTTP."""
    requests = []

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_GET(self):
            requests.append(("GET", self.path, self.headers.get("Authorization")))
            document = documents.get(self.path)
            body = json.dumps(document).encode()
            self.send_response(200 if document is not None else 404)
            self.end_headers()
            self.wfile.write(body)

        def do_DELETE(self):
            requests.append(("DELETE", self.path, self.headers.get("Authorization")))
            self.send_response(204)
            self.end_headers()

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}", requests
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


def registry_documents(config, images, repository):
    old = "2025-01-01T00:00:00Z"
    recent = "2026-11-01T00:00:00Z"
    documents = {}
    for role, package in config["packages"].items():
        # The production publisher emits these full references; transport tags
        # are decoded from them, not separately constructed by the test.
        tag = images[f"GHCR_{role.upper()}_RUN_IMAGE"].rsplit(":", 1)[1]
        documents[f"/orgs/{config['owner']}/packages/container/{package}/versions?per_page=100&page=1"] = [
            {"id": 41, "created_at": old, "metadata": {"container": {"tags": [tag]}}},
            {"id": 42, "created_at": old, "metadata": {"container": {"tags": [tag, "cache"]}}},
            {"id": 43, "created_at": recent, "metadata": {"container": {"tags": [tag]}}},
            {"id": 44, "created_at": old, "metadata": {"container": {"tags": []}}},
        ]
        documents[f"/repos/{repository}/actions/runs/{tag}"] = {
            "id": int(tag), "repository": {"full_name": repository},
            "path": config["publisher_workflow"], "status": "completed",
        }
    return documents


def test_ghcr_cleanup_owner_dry_run_and_deletion():
    registry = helper("integration_registry")
    config = registry.registry_config()
    repository = "Sidiora-Labs/Paxeer-X-Network"
    images = registry.image_environment(config, repository, "12345")
    documents = registry_documents(config, images, repository)
    now = datetime(2026, 11, 4, tzinfo=timezone.utc)
    with registry_server(documents) as (url, requests):
        api = registry.RegistryAPI("loopback-test-token", url)
        output = io.StringIO()
        dry_plan = registry.cleanup(config, repository, api, 14, True, now, output)
        assert len(dry_plan) == 2
        assert all(entry["version_id"] == 41 and entry["tags"] == ["12345"] for entry in dry_plan)
        assert {entry["package"] for entry in dry_plan} == set(config["packages"].values())
        assert not [request for request in requests if request[0] == "DELETE"]
        assert [json.loads(line) for line in output.getvalue().splitlines()] == [dict(entry, dry_run=True) for entry in dry_plan]
        live_plan = registry.cleanup(config, repository, api, 14, False, now, io.StringIO())
        assert live_plan == dry_plan
        deleted = [request[1] for request in requests if request[0] == "DELETE"]
        assert deleted == [f"/orgs/{config['owner']}/packages/container/{entry['package']}/versions/41" for entry in dry_plan]
        assert {request[2] for request in requests} == {"Bearer loopback-test-token"}
    assert "loopback-test-token" not in output.getvalue()


@pytest.mark.parametrize("field,value", [
    ("path", ".github/workflows/unrelated.yml"),
    ("status", "in_progress"),
    ("id", 98765),
    ("repository", {"full_name": "other/repo"}),
])
def test_ghcr_cleanup_rejects_unproven_run_before_delete(field, value):
    registry = helper("integration_registry")
    config = registry.registry_config()
    repository = "Sidiora-Labs/Paxeer-X-Network"
    documents = registry_documents(config, registry.image_environment(config, repository, "12345"), repository)
    documents[f"/repos/{repository}/actions/runs/12345"][field] = value
    with registry_server(documents) as (url, requests):
        with pytest.raises(ValueError, match="publisher run"):
            registry.cleanup(config, repository, registry.RegistryAPI("loopback-test-token", url), 14, False, datetime(2026, 11, 4, tzinfo=timezone.utc), io.StringIO())
        assert not [request for request in requests if request[0] == "DELETE"]


def test_review_upload_url_log():
    config = workflow("paxeer-pr-to-slack-codex.yml")
    upload_steps = [step for job in config["jobs"].values() for step in job["steps"] if "getUploadURLExternal" in step.get("run", "")]
    assert len(upload_steps) == 1
    script = upload_steps[0]["run"]
    shell_syntax(script)
    assert "set -euo pipefail" in script
    extract = script.index("upload_url=$(printf")
    mask = script.index("printf '::add-mask::%s\\n'")
    upload = script.index('curl --fail -sS -X POST "$upload_url"')
    assert extract < mask < upload
    assert "masked_url=${upload_url//'%'/'%25'}" in script
    assert "masked_url=${masked_url//$'\\r'/'%0D'}" in script
    assert "masked_url=${masked_url//$'\\n'/'%0A'}" in script
    assert "jq -c '{ok, error: (.error // null), file_id: (.file_id // null)}'" in script
    assert "jq -c '{ok, error: (.error // null)}'" in script
    assert 'echo "$ticket"' not in script
    assert 'echo "$resp"' not in script
    assert 'failed: $ticket' not in script and 'failed: $resp' not in script
    assert 'test "$(printf \'%s\' "$ticket" | jq -r \'.ok\')" = "true"' in script
    assert '-a -n "$upload_url" -a -n "$file_id"' in script
    assert 'files.completeUploadExternal failed" >&2; exit 1;' in script
    assert 'getUploadURLExternal failed" >&2; exit 1;' in script
    assert '"filename=@review.md;type=text/markdown"' in script
    assert 'files:[{id:$fid, title:$title}], channel_id:$ch, thread_ts:$ts' in script


def release_source_contract():
    script = source("tools/release/build-paxd.sh")
    shell_syntax(script)
    assert "set -euo pipefail" in script
    assert 'if [ "${PAXD_RELEASE_LAYERXD:-0}" != 0 ]; then' in script
    assert "PAXD_RELEASE_LAYERXD is retired; release the merged paxd package" in script
    assert re.search(r"release_assets\(\).*?return 2", script, re.S)
    assert not re.search(r"(?:go|cargo) build[^\n]*layerxd", script)
    libs = re.search(r"^native_libs=\((.+)\)$", script, re.M)
    assert shlex.split(libs[1]) == ["build/liblayerx.a", "programs/target/release/liblayerx_programs_sandbox.a"]
    native = script.index('paxd-native-libs\n')
    manifest = script.index('sha256sum "${native_libs[@]}" > "$out/native-libs.sha256"')
    check = script.index('sha256sum --check --strict "$out/native-libs.sha256"')
    build = script.index('go build -trimpath')
    assert native < manifest < check < build
    assert 'native_digest=$(cat "${native_libs[@]}" | sha256sum' in script
    assert 'ldflags="$ldflags -X $version_pkg.NativeArchiveDigest=$native_digest"' in script
    assert "version --long --output json | jq -er '.native_archive_digest'" in script
    assert 'echo "build-paxd: embedded native archive digest differs" >&2\n  exit 1' in script
    assert '-o "$out/$paxd" ./daemon/paxd' in script
    assert 'paxd="paxd-$version-$goos-$goarch"' in script
    assert 'cp "$platform_target/release/price-sidecar" "$out/price-sidecar-$version-$goos-$goarch"' in script
    assert 'sha256sum --check --strict --quiet' in script
    assert 'write_sums\npython3 tools/release/upgrade_info.py release --sums "$out/SHA256SUMS" --tag "$tag" --out "$out/upgrade-info.json"\nwrite_sums\n(cd "$out" && sha256sum --check --strict SHA256SUMS)' in script
    assert "-name SHA256SUMS" in script and "sort | xargs sha256sum > SHA256SUMS" in script
    return script


def test_release_check_daemon():
    release_source_contract()
    name = "paxeer-release-check.yml"
    assert "PAXD_RELEASE_LAYERXD" not in (WORKFLOWS / name).read_text()
    step = step_named(name, "release-check", "Check reproducible release build")
    assert step["run"] == "tools/release/reproducible-check.sh"
    assert "env" not in step
    consumer = source("tools/release/reproducible-check.sh")
    assert consumer.count('"$root/tools/release/build-paxd.sh"') == 2
    assert 'cmp -s "$first/native-libs.sha256" "$second/native-libs.sha256"' in consumer
    assert 'cmp -s "$first/SHA256SUMS" "$second/SHA256SUMS"' in consumer


def test_release_publish_daemon():
    release_source_contract()
    name = "paxeer-release-publish.yml"
    assert "PAXD_RELEASE_LAYERXD" not in (WORKFLOWS / name).read_text()
    build = step_named(name, "releaser", "Build release binaries")
    assert build["run"] == "tools/release/build-paxd.sh"
    assert "env" not in build
    publish = step_named(name, "releaser", "Publish GitHub release")["run"]
    shell_syntax(publish)
    assert 'gh release create "$RELEASE_TAG"' in publish
    assert '"dist/$RELEASE_TAG"/*' in publish
    assert 'cat "dist/$RELEASE_TAG/SHA256SUMS"' in publish
    info = source("tools/release/upgrade_info.py")
    assert 'asset = f"paxd-{identity[\'version\']}-linux-amd64"' in info
    assert '"binaries": {' in info


def test_go_publication_checksum_transport_and_evidence(tmp_path):
    checksum = helper("go_publication_checksum")
    module = "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/platform/go"
    version = "v1.2.3"
    digest = "h1:" + base64.b64encode(bytes(range(32))).decode()
    exact = f"{module}\t{version}  {digest}"
    sums = tmp_path / "go.sum"
    sums.write_text(f"{module} {version}/go.mod {digest}\n{exact}\n{module} v1.2.4 {digest}\n", encoding="utf-8")
    download = tmp_path / "download.json"
    download.write_text(json.dumps({"Path": module, "Version": version, "Sum": digest}), encoding="utf-8")
    evidence = tmp_path / "registry-digest.txt"
    evidence.write_text("zip-sha256  module.zip\n", encoding="utf-8")
    assert checksum.retain_checksum(module, version, sums, download, evidence) == exact
    assert evidence.read_text() == "zip-sha256  module.zip\n" + exact + "\n"
    script = step_named("platform.yml", "publish-go-modules", "Install the module from the proxy through the checksum database and verify it byte for byte")["run"]
    shell_syntax(script)
    assert 'go mod download -json "$module@v$LAYERX_RELEASE_VERSION" > download.json' in script
    assert 'python3 "$GITHUB_WORKSPACE/.github/workflows/scripts/go_publication_checksum.py"' in script
    assert '"$module" "v$LAYERX_RELEASE_VERSION" go.sum download.json "$output/registry-digest.txt"' in script
    assert 'go-checksum-database:sum.golang.org $checksum' in script
    assert 'test -n "$checksum"' in script
    assert 'grep " $module' not in script


@pytest.mark.parametrize("case", ["missing", "other-module", "other-version", "go-mod-only", "duplicate", "extra-field", "bad-prefix", "bad-base64", "short-digest", "different-sum", "different-path", "different-version", "download-error"])
def test_go_publication_checksum_rejects_invalid_evidence(case, tmp_path):
    checksum = helper("go_publication_checksum")
    module, version = "example.org/module", "v1.2.3"
    digest = "h1:" + base64.b64encode(bytes(range(32))).decode()
    line = f"{module} {version} {digest}"
    download = {"Path": module, "Version": version, "Sum": digest}
    records = {
        "missing": "", "other-module": line.replace(module, "example.org/other"),
        "other-version": line.replace(version, "v1.2.4"),
        "go-mod-only": line.replace(version, version + "/go.mod"),
        "duplicate": line + "\n" + line, "extra-field": line + " extra",
        "bad-prefix": line.replace("h1:", "h2:"), "bad-base64": line.replace(digest, "h1:!"),
        "short-digest": line.replace(digest, "h1:YQ=="),
    }
    if case == "different-sum":
        download["Sum"] = "h1:" + base64.b64encode(b"x" * 32).decode()
    elif case == "different-path":
        download["Path"] = "example.org/other"
    elif case == "different-version":
        download["Version"] = "v1.2.4"
    elif case == "download-error":
        download["Error"] = "checksum mismatch"
    sums = tmp_path / "go.sum"
    sums.write_text(records.get(case, line) + "\n")
    metadata = tmp_path / "download.json"
    metadata.write_text(json.dumps(download))
    evidence = tmp_path / "evidence.txt"
    with pytest.raises(ValueError):
        checksum.retain_checksum(module, version, sums, metadata, evidence)
    assert not evidence.exists()


def test_repair_record_maps_all_findings():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    expected = {
        "node-pr-input-coverage": "test_node_pr_input_coverage",
        "ecr-build-file": "test_ecr_build_file",
        "nightly-build-file": "test_nightly_build_file",
        "ghcr-cleanup-owner": "test_ghcr_cleanup_owner_dry_run_and_deletion",
        "review-upload-url-log": "test_review_upload_url_log",
        "release-check-daemon": "test_release_check_daemon",
        "release-publish-daemon": "test_release_publish_daemon",
        "go-publication-checksum": "test_go_publication_checksum_transport_and_evidence",
    }
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    assert set(record["finding"]) == set(expected)
    for finding, test in expected.items():
        assert test in record["finding"][finding]["tests"]
        assert set(record["finding"][finding]["tests"]) <= declared
        assert record["finding"][finding]["audit"] == "22.30.169"
    repair = record["repair"]
    assert repair["task"] == "22.33.699"
    assert repair["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert repair["evidence_ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in repair["evidence"]
    assert "No baseline failures are claimed" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    assert any("tools/release/tests/build.test.sh" in request and "domain 16" in request for request in repair["wiring_requests"])
    assert all("layerxd" not in name for name in record["release_inventory"]["linux_amd64"])
