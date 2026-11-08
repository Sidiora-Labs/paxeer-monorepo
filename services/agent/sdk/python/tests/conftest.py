"""Apply the SDK's explicit tool requirements and recorded repair baseline."""

import json
import os
from pathlib import Path
import shutil

import pytest


ROOT = Path(__file__).resolve().parents[5]
BASELINE = ROOT / "tests/ci/data/agent_sdk_python_baseline.json"
SDK_TREES = ("tests/agent/sdk/python", "services/agent/sdk/python/tests")
FIXTURE_REQUIREMENTS = {
    ("test_native_effect.py", "NativeEffectTest"): "NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST",
    ("test_program_http.py", "ProgramHttp"): "LAYERX_PYTHON_PROGRAM_HTTP_FIXTURE",
}


def pytest_collection_modifyitems(items):
    # This session hook also covers the central suite, whose tests moved out of
    # the SDK package. Use repository-relative ids regardless of pytest rootdir.
    baseline = {entry["nodeid"]: entry for entry in json.loads(BASELINE.read_text())}
    for item in items:
        relative = item.path.resolve().relative_to(ROOT).as_posix()
        if not any(relative.startswith(tree + "/") for tree in SDK_TREES):
            continue
        nodeid = relative + "::" + item.nodeid.split("::", 1)[1]
        class_name = item.cls.__name__ if item.cls is not None else None
        requirement = FIXTURE_REQUIREMENTS.get((item.path.name, class_name))
        if requirement and not os.environ.get(requirement):
            item.add_marker(pytest.mark.skip(reason=f"requires {requirement}"))
        elif (
            item.path.name == "test_account_derivation.py"
            and class_name == "BindTransportTest"
            and not os.environ.get("ANVIL_BIN")
            and shutil.which("anvil") is None
        ):
            item.add_marker(pytest.mark.skip(reason="requires ANVIL_BIN or anvil on PATH"))
        elif nodeid in baseline:
            entry = baseline[nodeid]
            item.add_marker(
                pytest.mark.skip(
                    reason=f"baseline retired by {entry['retiring_task']}: {entry['first_error_line']}"
                )
            )
