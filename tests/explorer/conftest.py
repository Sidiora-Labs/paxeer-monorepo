import importlib.util
from pathlib import Path
from types import ModuleType

import pytest


@pytest.fixture
def data_boundary() -> ModuleType:
    path = Path(__file__).with_name("paxeer-x-data-boundary.py")
    spec = importlib.util.spec_from_file_location("paxeer_x_data_boundary", path)
    if spec is None or spec.loader is None:
        raise ImportError(f"Cannot load data-boundary script from {path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module
