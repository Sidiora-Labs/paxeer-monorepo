"""Exercise the production output guard in-process and check its shell wiring.

hf-build supplies the declared bash -n parse. No subprocess or build toolchain
is used by this gate; Go generation and paxd transaction semantics are reviewed
source facts, not claimed runtime coverage.
"""

import json
import os
from pathlib import Path
import re
import runpy
import shlex
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
GOV = ROOT / "tools/paxeer-x/gov"
RECORD = Path(__file__).with_name("repair_22_31_428.kvx")
CONFIG = ROOT / "services/bridge/deploy/proposals/testdata/solana.json"
GENERATOR = ROOT / "services/bridge/deploy/proposals/proposals.go"


@pytest.fixture
def guard():
    return runpy.run_path(str(GOV / "output.py"))


@pytest.fixture
def workspace(tmp_path, guard):
    output = guard["prepare_output"](tmp_path / "out")
    outside = tmp_path / "operator"
    outside.mkdir()
    sentinel = outside / "important.json"
    sentinel.write_bytes(b"operator data must survive")
    return output, sentinel


def config_file(tmp_path, name):
    document = json.loads(CONFIG.read_text())
    document["name"] = name
    path = tmp_path / "config.json"
    path.write_text(json.dumps(document))
    return path


def staged_files(output, guard):
    # File-backed peer bytes from committed JSON fixtures. The guard transports
    # bytes without interpreting bridge messages; Go message validity is not
    # inferred from these filesystem tests.
    stage = output / ".bridge-render.fixture"
    (stage / "bodies").mkdir(parents=True)
    (stage / "proposals").mkdir()
    raw = CONFIG.read_bytes()
    names = list(guard["BODY_FILES"])
    names.append("03-set-cap-01-" + "a" * 40 + ".json")
    for name in names:
        (stage / "bodies" / name).write_bytes(raw)
    for name in guard["PROPOSAL_FILES"]:
        (stage / "proposals" / name).write_bytes(
            (GOV / "templates/03-xweb-fee.json").read_bytes()
        )
    return stage, {
        path.relative_to(stage): path.read_bytes()
        for section in ("bodies", "proposals")
        for path in (stage / section).iterdir()
    }


@pytest.mark.parametrize("name", ["solana", "Ethereum-main_1", "125", "a"])
def test_f004_safe_identifiers_reach_contained_directory(guard, workspace, tmp_path, name):
    output, sentinel = workspace
    directory = guard["bridge_directory"](output, config_file(tmp_path, name))
    assert directory == output / "bridge" / name
    assert directory.resolve().is_relative_to(output)
    assert directory.is_dir()
    assert sentinel.read_bytes() == b"operator data must survive"


@pytest.mark.parametrize("name", [
    "", ".", "..", "../../operator", "../out-sibling", "/operator",
    "chain/../../operator", "chain/child", "chain\\child", "-option",
    " chain", "chain ", "chain\n", "chain\t", "chain\x00", "$(rm)",
    "*", "chain.name", "cha\u0131n", None, 125, True, [], {},
])
def test_f004_rejects_traversal_and_nonstring_names(guard, workspace, tmp_path, name):
    output, sentinel = workspace
    with pytest.raises(ValueError, match="bridge name"):
        guard["bridge_directory"](output, config_file(tmp_path, name))
    assert not (output / "bridge").exists()
    assert sentinel.read_bytes() == b"operator data must survive"


@pytest.mark.parametrize("document", ["{}", "[]", "null", "{", '"solana"'])
def test_f004_rejects_malformed_configs(guard, workspace, tmp_path, document):
    output, sentinel = workspace
    config = tmp_path / "invalid.json"
    config.write_text(document)
    with pytest.raises(ValueError):
        guard["bridge_directory"](output, config)
    assert not (output / "bridge").exists()
    assert sentinel.read_bytes() == b"operator data must survive"


@pytest.mark.parametrize("component", ["bridge", "bridge/solana"])
@pytest.mark.parametrize("external", [True, False])
def test_f004_rejects_symlinked_directories(guard, workspace, tmp_path, component, external):
    output, sentinel = workspace
    link = output / component
    link.parent.mkdir(parents=True, exist_ok=True)
    target = sentinel.parent if external else output / "operator"
    target.mkdir(exist_ok=True)
    link.symlink_to(target, target_is_directory=True)
    with pytest.raises(ValueError):
        guard["bridge_directory"](output, CONFIG)
    assert link.is_symlink()
    assert list(target.iterdir()) == ([sentinel] if external else [])
    assert sentinel.read_bytes() == b"operator data must survive"


def test_f004_resolved_containment_rejects_prefix_siblings(guard, workspace, tmp_path):
    output, sentinel = workspace
    for path in (output, output / "../operator", tmp_path / "out-other/file"):
        with pytest.raises(ValueError, match="escapes selected directory"):
            guard["contained"](output, path)
    alias = tmp_path / "selected-output"
    alias.symlink_to(output, target_is_directory=True)
    assert guard["prepare_output"](alias) == output
    assert sentinel.read_bytes() == b"operator data must survive"


def test_f004_publish_replaces_only_generated_files(guard, workspace):
    output, sentinel = workspace
    destination = guard["bridge_directory"](output, CONFIG)
    for section in ("bodies", "proposals"):
        (destination / section).mkdir()
    preserved = [
        destination / "operator.json",
        destination / "bodies/03-set-cap-not-a-generated-file.json",
        destination / "proposals/review.json",
    ]
    for path in preserved:
        path.write_bytes(b"keep")
    (destination / "proposals/operator-directory").mkdir()
    (destination / "proposals/operator-link").symlink_to(sentinel)
    stale = destination / "bodies" / ("03-set-cap-02-" + "b" * 40 + ".json")
    stale.write_bytes(b"obsolete")
    for name in guard["PROPOSAL_FILES"]:
        for suffix in (".tx.json", ".err"):
            (destination / "proposals" / (name + suffix)).write_bytes(b"obsolete")
    stage, expected = staged_files(output, guard)
    for relative in expected:
        os.link(sentinel, destination / relative)
    guard["publish_bridge"](output, destination, stage)
    assert {relative: (destination / relative).read_bytes() for relative in expected} == expected
    assert not stale.exists()
    assert all(path.read_bytes() == b"keep" for path in preserved)
    assert (destination / "proposals/operator-directory").is_dir()
    assert (destination / "proposals/operator-link").is_symlink()
    assert not list(destination.glob("proposals/*.tx.json"))
    assert not list(destination.glob("proposals/*.err"))
    assert list((stage / "bodies").iterdir()) == []
    assert list((stage / "proposals").iterdir()) == []
    assert sentinel.read_bytes() == b"operator data must survive"


def test_f004_rerender_removes_unemitted_optional_proposal(guard, workspace):
    output, sentinel = workspace
    destination = guard["bridge_directory"](output, CONFIG)
    stage, expected = staged_files(output, guard)
    guard["publish_bridge"](output, destination, stage)
    assert all((destination / path).read_bytes() == raw for path, raw in expected.items())
    for section in ("bodies", "proposals"):
        (stage / section).rmdir()
    stage.rmdir()
    optional = destination / "proposals/05-proposal-sidiora-cap.json"
    (destination / "operator.json").write_bytes(b"keep between renders")
    stage, _ = staged_files(output, guard)
    (stage / "proposals" / optional.name).unlink()
    guard["publish_bridge"](output, destination, stage)
    assert not optional.exists()
    assert (destination / "proposals/04-proposal-open-chain.json").read_bytes() == (
        GOV / "templates/03-xweb-fee.json"
    ).read_bytes()
    assert (destination / "operator.json").read_bytes() == b"keep between renders"
    assert sentinel.read_bytes() == b"operator data must survive"


@pytest.mark.parametrize("attack", [
    "bodies-directory", "proposals-directory", "target-link", "target-directory",
    "sidecar-link", "source-link", "unknown-source", "incomplete-source",
])
def test_f004_publish_preflights_all_paths_before_replacement(guard, workspace, attack):
    output, sentinel = workspace
    destination = guard["bridge_directory"](output, CONFIG)
    for section in ("bodies", "proposals"):
        (destination / section).mkdir()
    first = destination / "bodies/01-register-chain.json"
    first.write_bytes(b"previous generated output")
    stage, _ = staged_files(output, guard)
    if attack.endswith("-directory") and attack != "target-directory":
        section = attack.removesuffix("-directory")
        (destination / section).rename(destination / (section + "-preserved"))
        (destination / section).symlink_to(sentinel.parent, target_is_directory=True)
    elif attack in ("target-link", "target-directory", "sidecar-link"):
        target = destination / "proposals/04-proposal-open-chain.json"
        if attack == "sidecar-link":
            target = Path(str(target) + ".tx.json")
        if attack == "target-directory":
            target.mkdir()
            (target / "operator.json").write_bytes(b"keep")
        else:
            target.symlink_to(sentinel)
    elif attack == "source-link":
        source = stage / "proposals/04-proposal-open-chain.json"
        source.unlink()
        source.symlink_to(sentinel)
    elif attack == "unknown-source":
        (stage / "proposals/operator.json").write_bytes(b"unexpected")
    elif attack == "incomplete-source":
        (stage / "proposals/04-proposal-open-chain.json").unlink()
    with pytest.raises(ValueError):
        guard["publish_bridge"](output, destination, stage)
    previous = (destination / "bodies-preserved/01-register-chain.json"
                if attack == "bodies-directory" else first)
    assert previous.read_bytes() == b"previous generated output"
    assert (stage / "bodies/01-register-chain.json").read_bytes() == CONFIG.read_bytes()
    assert sentinel.read_bytes() == b"operator data must survive"


def test_f004_publish_rejects_unselected_destinations_and_stages(guard, workspace, tmp_path):
    output, sentinel = workspace
    destination = guard["bridge_directory"](output, CONFIG)
    stage, _ = staged_files(output, guard)
    for invalid in (sentinel.parent, output / "bridge/../operator", output / "other/solana"):
        with pytest.raises(ValueError):
            guard["publish_bridge"](output, invalid, stage)
    for invalid in (sentinel.parent, output / "bridge", output / ".bridge-render.absent"):
        with pytest.raises(ValueError):
            guard["publish_bridge"](output, destination, invalid)
    linked = output / ".bridge-render.link"
    linked.symlink_to(stage, target_is_directory=True)
    with pytest.raises(ValueError):
        guard["publish_bridge"](output, destination, linked)
    assert sentinel.read_bytes() == b"operator data must survive"


def test_f004_top_level_replaces_names_without_following_hardlinks(guard, workspace):
    output, sentinel = workspace
    keep = output / "operator.json"
    keep.write_bytes(b"keep")
    for name in guard["TOP_LEVEL"]:
        for suffix in ("", ".tx.json", ".err"):
            os.link(sentinel, output / (name + suffix))
    assert guard["prepare_output"](output) == output
    assert sorted(path.name for path in output.iterdir()) == [keep.name]
    # Reopen with a normal shell-style write after preparation: the original
    # operator inode must remain unchanged.
    (output / guard["TOP_LEVEL"][0]).write_bytes(b"new proposal")
    assert sentinel.read_bytes() == b"operator data must survive"
    assert keep.read_bytes() == b"keep"


@pytest.mark.parametrize("kind", ["symlink", "dangling", "directory", "fifo"])
def test_f004_top_level_preflights_all_generated_targets(guard, workspace, kind):
    output, sentinel = workspace
    first = output / guard["TOP_LEVEL"][0]
    first.write_bytes(b"previous generated output")
    target = output / (guard["TOP_LEVEL"][-1] + ".err")
    if kind == "directory":
        target.mkdir()
    elif kind == "fifo":
        os.mkfifo(target)
    else:
        target.symlink_to(sentinel if kind == "symlink" else sentinel.parent / "absent")
    with pytest.raises(ValueError):
        guard["prepare_output"](output)
    assert first.read_bytes() == b"previous generated output"
    assert sentinel.read_bytes() == b"operator data must survive"


def test_f004_output_guard_cli_uses_real_operations(guard, workspace, tmp_path, capsys):
    output, sentinel = workspace
    guard["main"](["prepare", str(output)])
    assert capsys.readouterr().out == str(output) + "\n"
    guard["main"](["bridge", str(output), str(CONFIG)])
    directory = output / "bridge/solana"
    assert capsys.readouterr().out == str(directory) + "\n"
    stage, expected = staged_files(output, guard)
    guard["main"](["publish", str(output), str(directory), str(stage)])
    assert all((directory / relative).read_bytes() == raw for relative, raw in expected.items())
    with pytest.raises(ValueError, match="usage"):
        guard["main"](["unknown"])
    assert sentinel.read_bytes() == b"operator data must survive"


def test_f004_shell_wires_guarded_producer_transport_and_consumer(guard):
    source = (GOV / "render.sh").read_text()
    assert 'OUTPUT_GUARD="$ROOT/tools/paxeer-x/gov/output.py"' in source
    assert source.index('OUT=$(realpath -m "$OUT")') < source.index(
        'OUT=$(python3 "$OUTPUT_GUARD" prepare "$OUT")'
    ) < source.index('for template in "$TEMPLATES"/*.json; do')
    loop = source[source.index('for config in "${BRIDGE_CONFIGS[@]}"; do'):]
    ordered = [
        'dir=$(python3 "$OUTPUT_GUARD" bridge "$OUT" "$config")',
        'stage=$(mktemp -d "$OUT/.bridge-render.XXXXXXXX")',
        'go run ./services/bridge/deploy/proposals/cmd/paxeer-bridge-proposals',
        'python3 "$OUTPUT_GUARD" publish "$OUT" "$dir" "$stage"',
        'rmdir -- "$stage/bodies" "$stage/proposals" "$stage"',
        'dry_run "$file" tx gov submit-proposal layerxbridge-proposal "$file"',
    ]
    positions = [loop.index(command) for command in ordered]
    assert positions == sorted(positions)
    assert 'set -euo pipefail' in source
    assert 'rm ' not in loop
    assert 'name=$(jq' not in loop
    invocation = re.search(r'go run .*?\) >/dev/null', loop, re.S).group()
    argv = shlex.split(invocation.replace("\\\n", ""))
    assert argv[argv.index("-proposals") + 1] == "$stage/proposals"
    assert argv[-3:] == ["$config", "$stage/bodies)", ">/dev/null"]
    assert '--generate-only --offline' in source
    assert set(guard["TOP_LEVEL"]) == {path.name for path in (GOV / "templates").glob("*.json")}
    generator = GENERATOR.read_text()
    for name in (*guard["BODY_FILES"], *guard["PROPOSAL_FILES"]):
        assert '"' + name + '"' in generator
    assert '"03-set-cap-%02d-%s.json"' in generator
    assert 'hex.EncodeToString(msg.Asset[:])' in generator
    assert guard["generated_file"]("bodies", "03-set-cap-100-" + "a" * 40 + ".json")
    assert not guard["generated_file"]("bodies", "03-set-cap-00-" + "a" * 40 + ".json")
    assert not guard["generated_file"]("unknown", "01-register-chain.json")


def test_f004_record_maps_finding_to_executed_tests():
    record = tomllib.loads(RECORD.read_text())
    assert record["task"]["22"]["31"]["428"]["audit"] == "22.30.181"
    tests = record["finding"]["f004"]["tests"]
    assert len(tests) >= 10
    for name in tests:
        assert name in globals() and callable(globals()[name])
    assert record["task"]["22"]["31"]["428"]["wiring_requests"] == []
