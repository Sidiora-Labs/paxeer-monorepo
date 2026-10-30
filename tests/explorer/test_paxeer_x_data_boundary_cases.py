from pathlib import Path
from types import ModuleType


def test_history_readiness_uses_evm_url(data_boundary: ModuleType, tmp_path: Path):
    inherited = {
        "PATH": "/usr/bin",
        "LAYERX_INDEXER_RELAY_URL": "http://127.0.0.1:7000",
        "LAYERX_INDEXER_EVM_URL": "http://127.0.0.1:8544",
    }
    database = tmp_path / "history.sqlite"
    evm_url = "http://127.0.0.1:8545"

    environment = data_boundary.history_readiness_environment(inherited, database, 9000, evm_url)

    assert environment["LAYERX_INDEXER_EVM_URL"] == evm_url
    assert "LAYERX_INDEXER_RELAY_URL" not in environment
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
