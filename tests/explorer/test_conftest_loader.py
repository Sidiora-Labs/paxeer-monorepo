from pathlib import Path
import sys
from types import ModuleType

import pytest


def test_loader_exposes_cases(request: pytest.FixtureRequest):
    path = Path(__file__).with_name("paxeer-x-data-boundary.py")
    calls = []

    def record_calls(frame, event, arg):
        if event == "call" and frame.f_code.co_filename == str(path):
            calls.append(frame.f_code.co_name)

    previous_profile = sys.getprofile()
    sys.setprofile(record_calls)
    try:
        data_boundary = request.getfixturevalue("data_boundary")
    finally:
        sys.setprofile(previous_profile)

    assert isinstance(data_boundary, ModuleType)
    assert data_boundary.__name__ == "paxeer_x_data_boundary"
    assert Path(data_boundary.__file__) == path
    assert isinstance(data_boundary.CASES, tuple)
    assert data_boundary.CASES
    assert all(isinstance(case, str) for case in data_boundary.CASES)
    assert callable(data_boundary.prerequisites)
    assert data_boundary.prerequisites.__module__ == data_boundary.__name__
    assert callable(data_boundary.main)
    assert "<module>" in calls
    assert "main" not in calls
    assert "prerequisites" not in calls
