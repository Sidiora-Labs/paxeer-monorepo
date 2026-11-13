"""Exercise the SDK receipt integrations through their owning test modules."""

import importlib
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parents[2]
sys.path[:0] = [
    str(ROOT / "services/agent/sdk/python"),
    str(ROOT / "services/agent/sdk/python/tests"),
    str(ROOT / "tests/agent/sdk/python"),
]

for module_name in (
    "test_ai_market_commit",
    "test_retired_kernel_receipts",
    "test_web_search_sequencer",
    "test_x402_draw_sequencer",
    "test_x402_http_sequencer",
):
    module = importlib.import_module(module_name)
    for name, value in vars(module).items():
        if name.startswith("test_"):
            globals()[name] = value


# Explicit fixture exports work with both marker-based and object-based pytest APIs.
from test_ai_market_commit import (
    signatures, receipt_producer, signed_operation, commit_producer, committed_operation,
)
from test_retired_kernel_receipts import program_execution_evidence
