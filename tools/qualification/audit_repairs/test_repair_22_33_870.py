"""Offline source proof for retiring the standalone platform-node image.

Read the production recipes and catalogue without executing Docker or Bash.
These checks prove source wiring, not image builds or native node recovery.
"""

import hashlib
import json
from pathlib import Path
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
RECORD = Path(__file__).with_name("repair_22_33_870.kvx")
RETIRED = "docker/platform-node/Dockerfile"
NODE = "docker/paxeer/Dockerfile.paxd-node"
PAXD = "docker/paxeer/Dockerfile.paxd"
CATALOGUE = "services/platform/hosted/tests/beta-images.sh"


def source(path):
    return (ROOT / path).read_text(encoding="utf-8")


def docker_instructions(path):
    instructions = []
    for line in source(path).replace("\\\n", " ").splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        match = re.fullmatch(r"([A-Z]+)\s+(.+)", line)
        assert match is not None, (path, line)
        instructions.append(match.groups())
    assert instructions, path
    return instructions


def test_f001_standalone_image_recipe_is_removed():
    recipe = ROOT / RETIRED
    assert not recipe.exists(), "Retired daemon/guarantor recipe must be deleted"
    assert not recipe.is_symlink(), "A dangling replacement link is not retirement"
    directory = recipe.parent
    assert not directory.is_symlink()
    if directory.exists():
        assert list(directory.iterdir()) == [], "Do not replace the retired image"


def test_f001_retains_real_paxd_producer_and_runtime_consumer():
    instructions = docker_instructions(NODE)
    stages = {}
    current = None
    for instruction, value in instructions:
        if instruction == "FROM":
            words = shlex.split(value)
            if len(words) == 3:
                assert words[1].upper() == "AS"
                current = words[2]
            else:
                assert len(words) == 1
                current = "runtime"
            assert current not in stages
            stages[current] = []
        else:
            assert current is not None
            stages[current].append((instruction, value))
    assert set(stages) == {"native", "builder", "runtime"}
    native_runs = [value for key, value in stages["native"] if key == "RUN"]
    assert any("make programs-release-build" in run for run in native_runs)
    assert any('make -j"$(nproc)" paxd-native-libs;' in run for run in native_runs)
    builder = stages["builder"]
    assert ("COPY", "--from=native /out/ ./") in builder
    assert ("ENV", "CGO_ENABLED=1") in builder
    build_runs = [value for key, value in builder if key == "RUN"]
    assert any(
        "go build -tags" in run
        and "-o /go/bin/paxd ./daemon/paxd;" in run
        and "test -x /go/bin/paxd" in run
        for run in build_runs
    )
    assert (ROOT / "daemon/paxd/main.go").is_file()
    assert ("COPY", "--from=builder /go/bin/paxd /usr/bin/paxd") in stages["runtime"]
    entrypoints = [value for key, value in stages["runtime"] if key == "ENTRYPOINT"]
    assert [json.loads(value) for value in entrypoints] == [["/usr/bin/paxd"]]

    consumer = docker_instructions(PAXD)
    assert consumer[0] == (
        "ARG", "PAXD_IMAGE=ghcr.io/sidiora-labs/paxd-node:0.1.0"
    )
    assert consumer[1] == ("FROM", "${PAXD_IMAGE}")
    assert ("USER", "4020:4020") in consumer
    assert [json.loads(value) for key, value in consumer if key == "ENTRYPOINT"] == [
        ["/usr/bin/paxd"]
    ]
    for path in (NODE, PAXD):
        assert not re.search(r"\b(?:layerxd|layerx-handover|layerx-guarantor)\b", source(path))


def test_f001_catalogue_routes_only_to_retained_paxd_recipes():
    catalogue = source(CATALOGUE)
    names_match = re.search(r"^IMAGE_NAMES=\((.*?)\)$", catalogue, re.M | re.S)
    assert names_match is not None
    names = shlex.split(names_match[1])
    assert {"paxd-node", "paxd"} <= set(names)
    assert not {"platform-node", "layerx-node"}.intersection(names)
    assert "platform-node" not in catalogue
    mappings = {}
    function = catalogue.split("image_source() {\n", 1)[1].split("\n}\n", 1)[0]
    for name, payload in re.findall(r"^\s*([\w-]+)\) printf ('[^']*') ;;$", function, re.M):
        assert name not in mappings
        words = shlex.split(shlex.split(payload)[0])
        assert len(words) == 2
        mappings[name] = words
    assert set(names) == set(mappings)
    assert mappings["paxd-node"] == ["ghcr.io/sidiora-labs/paxd-node:0.1.0", NODE]
    assert mappings["paxd"] == ["ghcr.io/sidiora-labs/paxd:0.1.0", PAXD]
    assert RETIRED not in {recipe for _, recipe in mappings.values()}


def test_f001_record_binds_proofs_preserved_packaging_and_wiring_request():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["task"]["id"] == "22.33.870"
    assert record["task"]["requirement"] == "22998.1"
    assert record["task"]["baseline_tests_run"] is False
    assert record["task"]["evidence_ledger"] == "/out/run/gates.jsonl"
    assert record["task"]["verified_revision"]
    assert set(record["finding"]) == {"f001"}
    finding = record["finding"]["f001"]
    assert finding["file"] == RETIRED
    assert finding["audit"] == "22.30.183"
    assert finding["class"] == "retired-reference"
    assert set(finding["tests"]) == {
        "test_f001_standalone_image_recipe_is_removed",
        "test_f001_retains_real_paxd_producer_and_runtime_consumer",
        "test_f001_catalogue_routes_only_to_retained_paxd_recipes",
        "test_f001_record_binds_proofs_preserved_packaging_and_wiring_request",
    }
    for path, digest in record["preserved_packaging"].items():
        assert hashlib.sha256((ROOT / path).read_bytes()).hexdigest() == digest
    assert set(record["preserved_packaging"]) == {NODE, PAXD}
    requests = record["task"]["wiring_requests"]
    assert len(requests) == 1
    assert all(
        fact in requests[0]
        for fact in (
            "tools/containers/check.sh", "tools/containers/test_out_of_scope_list.py",
            "tools-ops", "domain 16", "OUT_OF_SCOPE", RETIRED,
        )
    )
