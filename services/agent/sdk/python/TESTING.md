Run the owning Python SDK gate from the package directory:

```sh
cd services/agent/sdk/python
python3 -m pytest -q
```

The default collection contract is the union of the central conformance suite at
`../../../../tests/agent/sdk/python` and the local integration suite at `tests`.
Do not replace this invocation with an explicit path to just one suite, or add
ignore, deselect, skip or expected-failure options to meet this contract.
`tests/test_collection_contract.py::test_default_sdk_collection_contract` checks
pytest's real default-root resolution and unchanged discovery patterns.

The local inventory includes all ten existing modules:

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

Future `test_*.py` modules in either directory enter default discovery automatically.
The default invocation preserves all receipt, restart and rejection assertions.
The suite needs pytest and the SDK's `derivation` extra (cryptography). Some existing
cases also use native receipt generators, local EVM tooling or explicit integration
inputs; adding discovery does not provision those prerequisites.

The focused repair qualification runs the collection contract in process, checks
both source inventories and proves existing test files were preserved. It does
not execute the SDK suites or certify their runtime behavior. The separately
owned gate and CI wiring requests are in
`tools/qualification/audit_repairs/repair_22_33_19.kvx` at the repository root.
