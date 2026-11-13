# Python SDK tests

Run the owning Python SDK gate from the package directory:

```sh
cd services/agent/sdk/python
python3 -m pytest -q
```

The default collection configured in `pyproject.toml` includes the central
conformance suite at `../../../../tests/agent/sdk/python` and the local
integration suite at `tests`.
`tests/test_collection_contract.py::test_default_sdk_collection_contract` checks
pytest's real default-root resolution and unchanged discovery patterns.

The local integration modules include:

- `test_ai_market_commit.py`
- `test_px_commit.py`
- `test_receipt_generated_current.py`
- `test_retired_kernel_receipts.py`
- `test_revert_retryable.py`
- `test_sequencer_receipt.py`
- `test_spot_call.py`
- `test_web_search_sequencer.py`
- `test_x402_draw_sequencer.py`
- `test_x402_http_sequencer.py`

New `test_*.py` and `*_test.py` modules in either directory enter default
discovery automatically.
The suite needs Python 3.11 or newer, pytest and the SDK's `derivation` extra
(`cryptography>=41`). `test_receipt_generated_current.py` invokes the Rust
generator through Cargo. Bind transport cases need `anvil` on `PATH` or
`ANVIL_BIN`. The central native effect and program HTTP cases use the inputs
named by `NATIVE_EFFECT_CROSS_LANGUAGE_REQUEST` and
`LAYERX_PYTHON_PROGRAM_HTTP_FIXTURE`, respectively. Collection eligibility is
defined in `tests/conftest.py`.
