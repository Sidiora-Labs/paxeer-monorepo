import ast
from pathlib import Path
from types import ModuleType

import pytest


def test_history_readiness_uses_evm_url(data_boundary: ModuleType, tmp_path: Path):
    inherited = {
        "PATH": "/usr/bin",
        "LAYERX_INDEXER_RELAY_URL": "http://127.0.0.1:7000",
        "LAYERX_INDEXER_EVM_URL": "http://127.0.0.1:8544",
        "LAYERX_INDEXER_COMET_URL": "http://127.0.0.1:26657",
    }
    database = tmp_path / "history.sqlite"
    evm_url = "http://127.0.0.1:8545"

    start_block = 6120
    abi_dir = tmp_path / "precompiles"
    environment = data_boundary.history_readiness_environment(
        inherited, database, 9000, evm_url, start_block=start_block, abi_dir=abi_dir
    )

    assert environment["LAYERX_INDEXER_EVM_URL"] == evm_url
    assert "LAYERX_INDEXER_RELAY_URL" not in environment
    assert "LAYERX_INDEXER_COMET_URL" not in environment
    assert environment["LAYERX_INDEXER_START_BLOCK"] == str(start_block)
    assert environment["LAYERX_INDEXER_EVM_CHAIN_ID"] == "125"
    assert environment["LAYERX_INDEXER_ABI_DIR"] == str(abi_dir)
    assert environment["LAYERX_INDEXER_DB"] == str(database)
    assert environment["LAYERX_INDEXER_LISTEN"] == "127.0.0.1:9000"
    assert environment["LAYERX_INDEXER_POLL_MS"] == "100"
    assert environment["LAYERX_INDEXER_TIMEOUT_MS"] == "400"
    assert environment["LAYERX_INDEXER_STALL_SECS"] == "5"
    assert environment["LAYERX_INDEXER_MAX_UNITS_PER_STEP"] == "1"
    assert environment["PATH"] == inherited["PATH"]
    assert inherited["LAYERX_INDEXER_RELAY_URL"] == "http://127.0.0.1:7000"
    assert inherited["LAYERX_INDEXER_EVM_URL"] == "http://127.0.0.1:8544"


def test_no_explorer_index_binary_probe(data_boundary: ModuleType):
    assert not hasattr(data_boundary, "SR_BINARY_NAME")
    assert not hasattr(data_boundary, "sr_binary")
    assert not hasattr(data_boundary, "sr_prerequisites")
    assert not hasattr(data_boundary, "case_standalone_receipts")
    assert "standalone-receipts" not in data_boundary.CASES


def test_history_readiness_evm_source_is_the_local_node(data_boundary: ModuleType, tmp_path: Path):
    tree = ast.parse(Path(data_boundary.__file__).read_text())
    literals = [node.value for node in ast.walk(tree)
                if isinstance(node, ast.Constant) and isinstance(node.value, str)]
    for retired in ("--relay-archive", "relay-archive/e2e.py", "LAYERX_HISTORY_RELAY_FIXTURE_CONFIG", "layerxd"):
        assert not any(retired in literal for literal in literals), retired
    assert "tools/qualification/localnode/v6120-node.sh" in literals
    assert any("LOCAL_NODE_RPC" in literal for literal in literals)

    env_file = tmp_path / "history-readiness.env"
    evm_url = "http://127.0.0.1:18545"
    env_file.write_text(f"LOCAL_NODE_RPC={evm_url}\nLOCAL_NODE_KEY=0x1234\n"
                        f"PERPS_DATA_TEST_RPC={evm_url}\nPERPS_DATA_TEST_COMET=http://127.0.0.1:26657\n"
                        "NEXT_PUBLIC_PERPS_GATEWAY_URL=http://127.0.0.1:0\n")
    assert data_boundary.history_local_node_rpc(env_file) == evm_url
    env_file.write_text("LOCAL_NODE_RPC=http://paxeer.example:8545\n")
    with pytest.raises(ValueError, match="LOCAL_NODE_RPC"):
        data_boundary.history_local_node_rpc(env_file)
    env_file.write_text(f"PERPS_DATA_TEST_RPC={evm_url}\n")
    with pytest.raises(ValueError, match="LOCAL_NODE_RPC"):
        data_boundary.history_local_node_rpc(env_file)
