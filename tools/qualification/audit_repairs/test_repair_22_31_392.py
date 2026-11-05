"""Exercise bridge-all-rejected through the real CLI, runner and child script."""

import json
import os
from pathlib import Path
import re
import shlex
import subprocess

import pytest


ROOT = Path(__file__).resolve().parents[3]
DEPLOY = ROOT / "services/bridge/deploy/deploy-evm-chain.sh"
OVERLAY = DEPLOY.with_name("overlay.sh")
CHAINS_ROOT = ROOT / "contracts/bridge/evm/chains"
CHAINS = (
    "ethereum",
    "base",
    "arbitrum",
    "optimism",
    "bnb",
    "polygon",
    "avalanche",
    "hyperevm",
)
ALL_FORMS = [("--all",), ("--preflight", "--all")]


def cli(arguments, environment, cwd, trace=False):
    return subprocess.run(
        ["bash", *(["-x"] if trace else []), str(DEPLOY), *arguments],
        cwd=cwd,
        env=environment,
        text=True,
        capture_output=True,
        timeout=30,
        check=False,
    )


@pytest.fixture
def environment():
    # Exclude inherited overlays, endpoints, keys, BASH_ENV and shell options.
    return {"PATH": os.environ["PATH"], "LC_ALL": "C"}


def test_bridge_all_rejected_source_contract():
    for script in (DEPLOY, OVERLAY):
        parsed = subprocess.run(
            ["bash", "-n", str(script)], text=True, capture_output=True, check=False
        )
        assert parsed.returncode == 0, parsed.stderr

    source = DEPLOY.read_text()
    parser = source.split("case ${1:-} in\n", 1)[1].split("\nesac", 1)[0]
    assert re.search(r"(?m)^--all\)\n\s*;;$", parser)
    assert parser.index("--all)") < parser.index("-*)")
    assert "[ $# -eq 1 ] || usage" in source
    assert 'if [ "$1" = --all ]; then' in source
    assert '[ "$preflight_only" -eq 0 ] || preflight=(--preflight)' in source
    assert (
        'run_all "$SCRIPT_DIR/deploy-evm-chain.sh" "${EVM_CHAINS[@]}" '
        '-- "${preflight[@]}"' in source
    )
    assert source.index('if [ "$preflight_only" -eq 1 ]; then') < source.index(
        "confirmed_chain_id=$(cast chain-id)"
    )

    runner = OVERLAY.read_text()
    declared = re.search(r"(?m)^EVM_CHAINS=\(([^\n]*)\)$", runner)
    assert declared is not None
    assert tuple(shlex.split(declared.group(1))) == CHAINS
    assert (
        'PAXEER_BRIDGE_DEPLOYMENT_RECORD='
        '"$PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR/$chain.json"' in runner
    )
    assert (
        'bash "$script" "${options[@]}" "$chain" || fail '
        '"$chain failed; the chains after it were not run"' in runner
    )


@pytest.mark.parametrize("arguments", ALL_FORMS)
@pytest.mark.parametrize("missing_directory", [False, True])
def test_bridge_all_rejected_cli_record_guard(
    tmp_path, environment, arguments, missing_directory
):
    if missing_directory:
        environment["PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR"] = str(tmp_path / "missing")
        reason = f"{tmp_path / 'missing'} does not exist"
    else:
        reason = (
            "PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR is required with --all and is not set"
        )
    result = cli(arguments, environment, tmp_path)
    assert result.returncode == 1, result.stderr
    assert f"deploy-evm-chain: error: {reason}\n" == result.stderr
    assert result.stdout == ""


@pytest.mark.parametrize("arguments", ALL_FORMS)
def test_bridge_all_rejected_cli_stops_first_failure(tmp_path, environment, arguments):
    records = tmp_path / "records"
    records.mkdir()
    chains = tmp_path / "missing-chains"
    environment.update(
        PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR=str(records),
        PAXEER_BRIDGE_EVM_CHAINS_ROOT=str(chains),
    )
    result = cli(arguments, environment, tmp_path)
    assert result.returncode == 1, result.stderr
    assert result.stderr.splitlines() == [
        "deploy-evm-chain.sh: ethereum",
        f"deploy-evm-chain: error: {chains / 'ethereum/config.json'} is not readable; "
        "ethereum is not a bridge EVM chain",
        "deploy-evm-chain: error: ethereum failed; the chains after it were not run",
    ]
    assert result.stdout == ""
    assert list(records.iterdir()) == []


@pytest.mark.parametrize("failure_chain", [None, *CHAINS])
def test_bridge_all_rejected_preflight_all(tmp_path, environment, failure_chain):
    records = tmp_path / "records with spaces"
    records.mkdir()
    overlay = tmp_path / "offline-overlay.json"
    entries = {}
    key_variables = {}
    for chain in CHAINS:
        configuration = json.loads((CHAINS_ROOT / chain / "config.json").read_text())
        assert configuration["chain"] == chain
        names = configuration["environment"]
        # These offline inputs are consumed by the production overlay and checks.
        # Preflight must exit before an RPC or signing command can use them.
        environment[names["rpc_url"]] = "offline://audit-repair-no-rpc"
        environment[names["deploy_key"]] = "offline-preflight-no-signing-key"
        key_variables[chain] = names["deploy_key"]
        entries[chain] = {
            "owner": f"0x{1:040x}",
            "attestors": [f"0x{number:040x}" for number in range(1, 6)],
        }
        if chain == "hyperevm":
            entries[chain]["big_blocks_acknowledged"] = True
    overlay.write_text(json.dumps({"chains": entries}))
    environment.update(
        BRIDGE_DEPLOY_OVERLAY=str(overlay),
        PAXEER_BRIDGE_EVM_CHAINS_ROOT=str(CHAINS_ROOT),
        PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR=str(records),
    )
    if failure_chain is not None:
        del environment[key_variables[failure_chain]]

    result = cli(("--preflight", "--all"), environment, tmp_path, trace=True)
    count = len(CHAINS) if failure_chain is None else CHAINS.index(failure_chain) + 1
    visited = CHAINS[:count]
    assert re.findall(r"(?m)^deploy-evm-chain\.sh: (\w+)$", result.stderr) == list(
        visited
    )
    assignments = re.findall(
        r"(?m)^\+ PAXEER_BRIDGE_DEPLOYMENT_RECORD=(.*)$", result.stderr
    )
    assert [shlex.split(value)[0] for value in assignments] == [
        str(records / f"{chain}.json") for chain in visited
    ]
    ready = re.findall(
        r"(?m)^deploy-evm-chain: (\w+) \(chain \d+\) is ready to deploy:",
        result.stderr,
    )
    if failure_chain is None:
        assert result.returncode == 0, result.stderr
        assert ready == list(CHAINS)
        assert "deploy-evm-chain: error:" not in result.stderr
    else:
        assert result.returncode == 1, result.stderr
        assert ready == list(visited[:-1])
        assert (
            f"deploy-evm-chain: error: {key_variables[failure_chain]} "
            "is required and is not set\n"
            in result.stderr
        )
        assert (
            f"deploy-evm-chain: error: {failure_chain} failed; "
            "the chains after it were not run\n"
            in result.stderr
        )
    assert result.stdout == ""
    assert list(records.iterdir()) == []


@pytest.mark.parametrize(
    "arguments",
    [(), ("--unknown",), ("--all", "ethereum"), ("--preflight", "--all", "base")],
)
def test_bridge_all_rejected_cli_usage(tmp_path, environment, arguments):
    result = cli(arguments, environment, tmp_path)
    assert result.returncode == 2, result.stderr
    assert result.stderr == "usage: deploy-evm-chain.sh [--preflight] <chain|--all>\n"
    assert result.stdout == ""
