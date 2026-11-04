"""Offline source contracts and real, fail-closed Bash argument parsing.

No Rust/Foundry toolchain, credentials or endpoint is used. The child PATH
contains only the real Bash and the coreutils needed to locate the script.
"""

import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import tomllib
from urllib.parse import urlsplit

import pytest


ROOT = Path(__file__).resolve().parents[3]
DEPLOY = ROOT / "services/bridge/deploy"
VERIFY = DEPLOY / "verify-evm-chain.sh"
OVERLAY = DEPLOY / "overlay.sh"
EXAMPLE = DEPLOY / "relayer-solana.example.json"
README = ROOT / "contracts/bridge/README.md"
RELAYER = ROOT / "services/interop/crates/layerx-bridge-relayer/src"
RECORD = Path(__file__).with_name("repair_22_33_420.kvx")
CHAINS = (
    "ethereum", "base", "arbitrum", "optimism", "bnb", "polygon",
    "avalanche", "hyperevm",
)


@pytest.fixture
def shell(tmp_path):
    # Symlink real runtime programs, never substitutes for jq, forge or cast.
    bin_dir = tmp_path / "bin"
    bin_dir.mkdir()
    for name in ("bash", "dirname", "basename"):
        executable = shutil.which(name)
        assert executable is not None, name
        (bin_dir / name).symlink_to(executable)
    assert {path.name for path in bin_dir.iterdir()} == {
        "bash", "dirname", "basename",
    }
    env = {
        "PATH": str(bin_dir),
        "LC_ALL": "C",
        "PAXEER_BRIDGE_EVM_CHAINS_ROOT": str(tmp_path / "chains"),
    }
    return bin_dir / "bash", env


def invoke(shell, arguments, extra_env=None):
    bash, env = shell
    return subprocess.run(
        [str(bash), str(VERIFY), *arguments],
        cwd=ROOT,
        env={**env, **(extra_env or {})},
        capture_output=True,
        text=True,
        timeout=10,
        check=False,
    )


def test_relayer_paxeer_root_and_all_other_example_facts_are_preserved():
    example = json.loads(EXAMPLE.read_text(encoding="utf-8"))
    endpoint, = example["paxeer"]["endpoints"]
    assert endpoint["url"] == "https://paxeer.example.invalid/"
    url = urlsplit(endpoint["url"])
    assert (url.scheme, url.hostname, url.path, url.query, url.fragment) == (
        "https", "paxeer.example.invalid", "/", "", "",
    )
    assert endpoint["trust_anchor_der"] == "/EXAMPLE_ONLY/operator-a/paxeer-ca.der"
    assert endpoint["request_timeout_ms"] == 5000
    assert example["paxeer"]["chain_id"] == 125
    foreign = example["chains"][0]["rpc"]["endpoints"] + example["solana"]["rpc"]["endpoints"]
    assert len(foreign) == 6
    for peer in foreign:
        url = urlsplit(peer["url"])
        assert url.scheme == "https"
        assert url.hostname.endswith(".example.invalid")
        assert url.path == "/rpc"
        assert peer["ca_certificate_der"].startswith("/EXAMPLE_ONLY/")
        assert peer["bearer_token_file"].startswith("/EXAMPLE_ONLY/")
    # Freeze every other original JSON value, including fixture keys and handles.
    endpoint["url"] = "<PAXEER_NODE_ORIGIN>"
    canonical = json.dumps(example, sort_keys=True, separators=(",", ":"))
    assert hashlib.sha256(canonical.encode()).hexdigest() == (
        "05d4da7d4580afc03f5f7bd25100881632b9a5076abf3983fc8528bc2012fa22"
    )


def test_relayer_example_loader_transport_and_operator_contract_agree():
    loader = (RELAYER / "config.rs").read_text(encoding="utf-8")
    loader = loader.split("pub fn paxeer_endpoints(", 1)[1].split("\n    pub fn ", 1)[0]
    assert "PaxeerEndpoint::new(\n                    endpoint.url.clone()," in loader
    assert "std::fs::read(path)" in loader
    assert "trust_anchor_der," in loader
    transport = (RELAYER / "transport.rs").read_text(encoding="utf-8")
    assert "let target = parse_url(&endpoint.url)?;" in transport
    assert '.map_or((rest, "/"), |index| rest.split_at(index))' in transport
    assert "path: path.to_owned()," in transport
    assert 'write!(stream, "POST {} HTTP/1.1' in transport
    assert "target.path, target.authority, body.len(), body)" in transport
    assert "trust_anchor(self.trust_anchor_der.as_deref())?;" in transport
    readme = README.read_text(encoding="utf-8")
    for fact in (
        "`paxeer.endpoints[].url` (`https://paxeer.example.invalid/` in the example)",
        "approved Paxeer node origin using the root path `/`",
        "passes this URL unchanged to its HTTP transport; do not append `/rpc`",
        "approved DER trust anchor through `trust_anchor_der` and retain TLS verification",
        "Foreign EVM and Solana endpoint paths belong to their respective providers",
        "Domain 11 owns the relayer loader and transport",
        "fixture value, not an approved deployment",
        "Do not fund them or use them as operator identities",
    ):
        assert fact in readme


@pytest.mark.parametrize("script", [VERIFY, OVERLAY], ids=["verifier", "overlay"])
def test_shell_sources_parse_with_bash_n(shell, script):
    bash, env = shell
    result = subprocess.run(
        [str(bash), "-n", str(script)], env=env, capture_output=True,
        text=True, timeout=10, check=False,
    )
    assert result.returncode == 0, result.stderr


def test_bridge_all_preserves_deployment_checks_preflight_and_order():
    source = VERIFY.read_text(encoding="utf-8")
    # The complete verifier must differ from the inspected source by this case only.
    all_case = "--all)\n    ;;\n"
    assert source.count(all_case) == 1
    original = source.replace(all_case, "", 1)
    assert hashlib.sha256(original.encode()).hexdigest() == (
        "a8c56a613b576c6641bfac57ed3905efcb1130e339c1e8b78bbf6a0ace897a93"
    )
    assert source.index(all_case) < source.index("-*)\n    usage")
    cardinality = source.index("[ $# -eq 1 ] || usage")
    dispatch = source.index('if [ "$1" = --all ]; then')
    assert cardinality < dispatch < source.index("chain=$1")
    assert 'run_all "$SCRIPT_DIR/verify-evm-chain.sh" "${EVM_CHAINS[@]}" -- "${preflight[@]}"' in source
    assert '[ "$preflight_only" -eq 0 ] || preflight=(--preflight)' in source
    record_checks = source.index('recorded_chain=$(jq -r')
    encode = source.index("constructor_arguments=$(cast abi-encode")
    preflight = source.index('if [ "$preflight_only" -eq 1 ]; then')
    submit = source.index("forge verify-contract")
    assert record_checks < encode < preflight < submit
    assert "exit 0\nfi" in source[preflight:submit]
    assert 'export ETHERSCAN_API_KEY="${!key_variable}"' in source
    assert "successfully verified|already verified" in source
    overlay = OVERLAY.read_text(encoding="utf-8")
    declared = re.search(r"^EVM_CHAINS=\(([^)]+)\)$", overlay, re.MULTILINE)
    assert declared is not None
    assert tuple(declared.group(1).split()) == CHAINS
    assert 'for chain in "${chains[@]}"; do' in overlay
    assert 'PAXEER_BRIDGE_DEPLOYMENT_RECORD="$PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR/$chain.json"' in overlay
    assert 'bash "$script" "${options[@]}" "$chain" || fail "$chain failed; the chains after it were not run"' in overlay


@pytest.mark.parametrize("prefix", [[], ["--preflight"]], ids=["submission", "preflight"])
@pytest.mark.parametrize("directory", ["unset", "missing", "empty"])
def test_bridge_all_reaches_real_run_all_and_stops_before_explorer(shell, tmp_path, prefix, directory):
    extra = {}
    if directory != "unset":
        records = tmp_path / "records"
        if directory == "empty":
            records.mkdir()
        extra["PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR"] = str(records)
    result = invoke(shell, [*prefix, "--all"], extra)
    assert result.returncode == 1, result.stderr
    assert "usage:" not in result.stderr
    if directory == "unset":
        assert "PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR is required with --all and is not set" in result.stderr
    elif directory == "missing":
        assert result.stderr == f"verify-evm-chain: error: {records} does not exist\n"
    else:
        visits = re.findall(r"^verify-evm-chain.sh: (\w+)$", result.stderr, re.MULTILINE)
        assert visits == [CHAINS[0]]
        assert "ethereum/config.json is not readable; ethereum is not a bridge EVM chain" in result.stderr
        assert "ethereum failed; the chains after it were not run" in result.stderr
    assert result.stdout == ""


@pytest.mark.parametrize("arguments", [
    [], ["--preflight"], ["--unknown"], ["--all", "ethereum"],
    ["--all", "--preflight"], ["ethereum", "base"],
    ["--preflight", "--all", "ethereum"], ["--preflight", "ethereum", "base"],
])
def test_bridge_option_cardinality_and_unknown_leading_option_are_rejected(shell, arguments):
    result = invoke(shell, arguments)
    assert result.returncode == 2, result.stderr
    assert result.stderr == "usage: verify-evm-chain.sh [--preflight] <chain|--all>\n"
    assert result.stdout == ""


@pytest.mark.parametrize("prefix", [[], ["--preflight"]])
@pytest.mark.parametrize("chain", ["ethereum", "BAD", "--unknown"])
def test_bridge_single_chain_validation_remains_fail_closed(shell, prefix, chain):
    result = invoke(shell, [*prefix, chain])
    if not prefix and chain.startswith("-"):
        assert result.returncode == 2
        assert "usage:" in result.stderr
    else:
        assert result.returncode == 1, result.stderr
        if chain == "ethereum":
            assert "ethereum/config.json is not readable; ethereum is not a bridge EVM chain" in result.stderr
        else:
            assert f"{chain} is not a chain name" in result.stderr
    assert result.stdout == ""


def test_repair_record_maps_both_findings_and_external_ownership():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    assert record["repair"]["task"] == "22.33.420"
    assert record["repair"]["audit"] == "22.30.145"
    assert set(record["finding"]) == {"relayer-paxeer-path", "bridge-all-option"}
    functions = set(re.findall(r"^def (test_\w+)\(", Path(__file__).read_text(), re.MULTILINE))
    for finding in record["finding"].values():
        assert finding["tests"]
        assert set(finding["tests"]) <= functions
    assert "Domain 11" in record["wiring"]["relayer"]
    assert "services/interop/crates/layerx-bridge-relayer" in record["wiring"]["relayer"]
