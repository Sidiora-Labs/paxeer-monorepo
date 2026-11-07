"""The owning SDK gate must select both maintained suites by default."""

from pathlib import Path

import pytest


SDK_ROOT = Path(__file__).resolve().parents[1]
CENTRAL_TESTS = SDK_ROOT.parents[3] / "tests/agent/sdk/python"


def test_default_sdk_collection_contract(monkeypatch):
    monkeypatch.chdir(SDK_ROOT)
    config = pytest.Config.fromdictargs({}, [])
    try:
        assert config.inipath == SDK_ROOT / "pyproject.toml"
        assert config.args_source == pytest.Config.ArgsSource.TESTPATHS
        assert config.getini("testpaths") == [
            "../../../../tests/agent/sdk/python",
            "tests",
        ]
        assert [(SDK_ROOT / path).resolve() for path in config.args] == [
            CENTRAL_TESTS,
            SDK_ROOT / "tests",
        ]
        assert config.getini("pythonpath") == [SDK_ROOT]
        assert config.getini("python_files") == ["test_*.py", "*_test.py"]
        assert config.getini("python_functions") == ["test"]
        assert all(path.is_dir() for path in (CENTRAL_TESTS, SDK_ROOT / "tests"))
        assert config.getini("addopts") == []
        assert config.getoption("ignore") is None
        assert config.getoption("ignore_glob") is None
        assert config.getoption("deselect") is None
    finally:
        config._ensure_unconfigure()
