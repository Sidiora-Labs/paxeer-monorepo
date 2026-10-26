# Explorer tests contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## data-boundary

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.272.1</code> | req | 14272.1 |
| <code>acceptance.14.272.1</code> | scenario | Given the fixture built at HEAD, when it runs with the human-test arguments, then its output equals the golden file |
| <code>acceptance.14.272.1</code> | test | tests/explorer_fixture_test.py::test_fixture_output_matches_golden (planned) |
| <code>acceptance.14.272.1</code> | qualification | UNRUN |
| <code>req.14272</code> | ac_1 | The fixture binary built from tests/explorer_fixture.c by Makefile:1395 prints the bytes frozen in explorer_fixture_golden.json. |
| <code>acceptance.14.273.1</code> | req | 14273.1 |
| <code>acceptance.14.273.1</code> | scenario | Given the fixture rebuilt without lxp_batch_sign, when it runs, then its output equals the golden file and the signature verifies under the fixture public key |
| <code>acceptance.14.273.1</code> | test | tests/explorer_fixture_test.py::test_fixture_signature_verifies (planned) |
| <code>acceptance.14.273.1</code> | qualification | UNRUN |
| <code>req.14273</code> | ac_1 | tests/explorer_fixture.c:197 signs without lxp_batch_sign and its output still matches the golden bytes. |
| <code>acceptance.14.274.1</code> | req | 14274.1 |
| <code>acceptance.14.274.1</code> | scenario | Given the data-boundary script, when its history-readiness environment is built, then LAYERX_INDEXER_EVM_URL is set and LAYERX_INDEXER_RELAY_URL is absent |
| <code>acceptance.14.274.1</code> | test | tests/explorer/test_paxeer_x_data_boundary_cases.py::test_history_readiness_uses_evm_url (planned) |
| <code>acceptance.14.274.1</code> | qualification | UNRUN |
| <code>req.14274</code> | ac_1 | The history-readiness case (tests/explorer/paxeer-x-data-boundary.py:1123) starts the indexer with LAYERX_INDEXER_EVM_URL. |
| <code>acceptance.14.274.2</code> | req | 14274.2 |
| <code>acceptance.14.274.2</code> | scenario | Given the data-boundary script loaded through importlib, when its attributes and CASES are read, then SR_BINARY_NAME, sr_binary and case_standalone_receipts are absent and CASES has no standalone-receipts entry |
| <code>acceptance.14.274.2</code> | test | tests/explorer/test_paxeer_x_data_boundary_cases.py::test_no_explorer_index_binary_probe (planned) |
| <code>acceptance.14.274.2</code> | qualification | UNRUN |
| <code>req.14274</code> | ac_2 | The data-boundary script no longer probes or launches layerx-explorer-index (SR_BINARY_NAME at tests/explorer/paxeer-x-data-boundary.py:577). |
| <code>acceptance.14.275.1</code> | req | 14275.1 |
| <code>acceptance.14.275.1</code> | scenario | Given the data-boundary script loaded through importlib, when CASES is read, then it equals ('history-readiness', 'unified-pagination') |
| <code>acceptance.14.275.1</code> | test | tests/explorer/test_paxeer_x_data_boundary_retired_cases.py::test_cases_are_history_and_pagination_only (planned) |
| <code>acceptance.14.275.1</code> | qualification | UNRUN |
| <code>req.14275</code> | ac_1 | CASES at tests/explorer/paxeer-x-data-boundary.py:24 is exactly history-readiness and unified-pagination. |
| <code>acceptance.14.275.2</code> | req | 14275.2 |
| <code>acceptance.14.275.2</code> | scenario | Given the data-boundary script source, when its prerequisite list and environment reads are collected, then bin/layerxd, bin/layerx-archive-codec and the 31 retired variables are absent |
| <code>acceptance.14.275.2</code> | test | tests/explorer/test_paxeer_x_data_boundary_retired_cases.py::test_no_retired_binary_prerequisite (planned) |
| <code>acceptance.14.275.2</code> | qualification | UNRUN |
| <code>req.14275</code> | ac_2 | The prerequisites at tests/explorer/paxeer-x-data-boundary.py:91 no longer name bin/layerxd or bin/layerx-archive-codec, and no retired variable is read. |
| <code>acceptance.14.297.1</code> | req | 14297.1 |
| <code>acceptance.14.297.1</code> | scenario | Given the conftest loader, when the data_boundary fixture is requested, then the module loads, main is not called and CASES is a tuple of strings |
| <code>acceptance.14.297.1</code> | test | tests/explorer/test_conftest_loader.py::test_loader_exposes_cases (planned) |
| <code>acceptance.14.297.1</code> | qualification | UNRUN |
| <code>req.14297</code> | ac_1 | The loader fixture imports tests/explorer/paxeer-x-data-boundary.py without running it and exposes CASES (tests/explorer/paxeer-x-data-boundary.py:24). |
| <code>capability.14.29.1</code> | title | Freeze the explorer fixture output as a golden file |
| <code>capability.14.29.1</code> | reqs | 14272.1 |
| <code>capability.14.29.2</code> | title | Sign the explorer fixture batch header through EVP instead of lxp_batch_sign |
| <code>capability.14.29.2</code> | reqs | 14273.1 |
| <code>capability.14.29.3</code> | title | Start the history indexer from the EVM URL and drop the index binary probe |
| <code>capability.14.29.3</code> | reqs | 14274.1<br>14274.2 |
| <code>capability.14.29.4</code> | title | Drop the kernel receipt cases and the layerxd prerequisites from the data-boundary test |
| <code>capability.14.29.4</code> | reqs | 14275.1<br>14275.2 |
| <code>capability.14.35.1</code> | title | Add an importlib loader fixture for the hyphenated data-boundary script |
| <code>capability.14.35.1</code> | reqs | 14297.1 |
