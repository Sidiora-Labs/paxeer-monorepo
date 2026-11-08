"""Keep Go CI connected to tagged suites and the independent registry module."""

from pathlib import Path
import re

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ".github/workflows/paxeer-go-test.yml"
GUARD = "tests/ci/test_go_build_tags_wired.py"
TAG_PACKAGES = {
    "mock_block_validation": "./consensus/types/...",
    "mock_chain_validation": "./consensus/types/...",
    "libsecp256k1_sdk": "./sdk/crypto/keys/secp256k1/...",
    "release": "./consensus/rpc/jsonrpc/client/...",
    "gofuzz": "./consensus/test/fuzz/...",
}


def go_workflow():
    return yaml.load((ROOT / WORKFLOW).read_text(), Loader=yaml.BaseLoader)


def assert_required_job(job):
    assert job["runs-on"] == "ubuntu-latest"
    assert "if" not in job
    assert "continue-on-error" not in job
    for step in job["steps"]:
        assert "if" not in step
        assert "continue-on-error" not in step
        if "uses" in step:
            assert re.fullmatch(r"[\w-]+/[\w-]+@[0-9a-f]{40}", step["uses"])
    assert any(
        step.get("uses", "").startswith("actions/checkout@")
        for step in job["steps"]
    )
    assert any(
        step.get("uses", "").startswith("actions/setup-go@")
        for step in job["steps"]
    )


def test_go_test_matrix_covers_build_tags():
    workflow = go_workflow()
    for event in ("pull_request", "push"):
        assert {
            "consensus/**", "sdk/**", "tests/integration/**", "chain.mk",
            WORKFLOW, GUARD,
        } <= set(workflow["on"][event]["paths"])
    assert {"workflow_call", "merge_group"} <= workflow["on"].keys()

    job = workflow["jobs"]["tagged-tests"]
    assert_required_job(job)
    assert job["strategy"]["fail-fast"] == "false"
    matrix = job["strategy"]["matrix"]
    assert set(matrix) == {"include"}
    entries = matrix["include"]
    mock_tags = {"mock_block_validation", "mock_chain_validation"}
    for entry in entries:
        tags = set(re.split(r"[,\s]+", entry["tags"].strip()))
        selected_mocks = tags & mock_tags
        assert len(selected_mocks) <= 1, "mock validation tags are mutually exclusive"
        if selected_mocks:
            assert tags == selected_mocks, "each mock validation tag needs its own leg"
            assert entry["packages"] == "./consensus/types/...", (
                "mock validation tags must only run consensus/types tests"
            )
    assert len(entries) == len(TAG_PACKAGES)
    assert {entry["tags"]: entry["packages"] for entry in entries} == TAG_PACKAGES
    assert job["env"]["CGO_ENABLED"] == "1"
    assert job["env"]["GOFLAGS"] == (
        "-race -tags=ledger,test_ledger_mock,${{ matrix.tags }}"
    )
    commands = [step["run"] for step in job["steps"] if "run" in step]
    assert commands == [
        "go test -count=1 -timeout=${{ env.GO_TEST_TIMEOUT }} ${{ matrix.packages }}"
    ]
    setup = next(
        step for step in job["steps"]
        if step.get("uses", "").startswith("actions/setup-go@")
    )
    assert setup["with"]["go-version"] == "${{ env.GO_VERSION }}"

    # Each leg must still select actual tests behind its build constraint.
    for tag, packages in TAG_PACKAGES.items():
        directory = ROOT / packages.removeprefix("./").removesuffix("/...")
        sources = list(directory.rglob("*_test.go"))
        assert sources, packages
        assert any(
            re.search(rf"^//go:build {re.escape(tag)}$", source.read_text(), re.M)
            for source in sources
        ), tag

    fuzz_sources = list((ROOT / "consensus/test/fuzz/tests").glob("*_test.go"))
    assert len(fuzz_sources) == 3
    for source in fuzz_sources:
        content = source.read_text()
        assert re.search(r"func Fuzz\w+\(f \*testing.F\)", content), source
        assert "f.Add(" in content, f"{source}: go test needs seed inputs"
        assert "f.Fuzz(" in content, source
    release = (ROOT / "consensus/rpc/jsonrpc/client/integration_test.go").read_text()
    assert "func TestWSClientReconnectWithJitter(t *testing.T)" in release
    assert "repairM1102ReconnectWithJitter(t, 8, 3)" in release

    autobahn = workflow["jobs"]["autobahn-integration"]
    assert_required_job(autobahn)
    assert autobahn["env"] == {
        "DOCKER_PLATFORM": "linux/amd64",
        "GIGA_EXECUTOR": "true", "GIGA_STORAGE": "true", "GIGA_OCC": "true",
    }
    assert [step["run"] for step in autobahn["steps"] if "run" in step] == [
        "make -f chain.mk autobahn-integration-test "
        "AUTOBAHN_TEST_TIMEOUT=${{ env.GO_TEST_TIMEOUT }}"
    ]
    makefile = (ROOT / "chain.mk").read_text()
    target = re.search(
        r"^autobahn-integration-test:\n((?:\t.*\n)+)", makefile, re.M
    )
    assert target is not None
    assert "AUTOBAHN_TEST_TIMEOUT ?= 30m" in makefile
    assert (
        'GOWORK=off go -C "$(PAXEER_PROJECT_HOME)" test '
        '-tags autobahn_integration -v -count=1 '
        '-timeout "$(AUTOBAHN_TEST_TIMEOUT)" ./tests/integration/autobahn/...'
    ) in target[1]
    assert ".PHONY: autobahn-integration-test" in makefile


def test_hpx_registry_go_test_wired():
    workflow = go_workflow()
    for event in ("pull_request", "push"):
        assert {"tools/hpx/**", WORKFLOW, GUARD} <= set(
            workflow["on"][event]["paths"]
        )
    job = workflow["jobs"]["hpx-registry"]
    assert_required_job(job)
    steps = job["steps"]
    setup = next(
        step for step in steps
        if step.get("uses", "").startswith("actions/setup-go@")
    )
    assert setup["with"]["go-version-file"] == "tools/hpx/registry/go.mod"
    module = (ROOT / "tools/hpx/registry/go.mod").read_text()
    version = re.search(r"^go (\S+)$", module, re.M)
    assert version is not None
    assert version[1] == workflow["env"]["GO_VERSION"]
    (test,) = [step for step in steps if step.get("name") == "Go test registry module"]
    assert test["working-directory"] == "tools/hpx/registry"
    assert test["env"]["GOWORK"] == "off"
    assert test["run"] == "go test -count=1 -timeout=${{ env.GO_TEST_TIMEOUT }} ./..."
    (guard,) = [step for step in steps if step.get("name") == "Check Go test wiring"]
    assert guard["run"] == f"python3 -m pytest -q {GUARD}"
    assert steps.index(setup) < steps.index(test)
    assert steps.index(guard) < steps.index(test)
    assert list((ROOT / "tools/hpx/registry").glob("*_test.go"))
