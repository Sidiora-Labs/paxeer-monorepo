# Explorer data boundary contract

## data-boundary

The [data-boundary driver](../../../tests/explorer/paxeer-x-data-boundary.py) checks history readiness and unified pagination. Its `CASES` tuple is exactly `("history-readiness", "unified-pagination")`.

History readiness starts the indexer with `LAYERX_INDEXER_EVM_URL`, chain ID `125`, a start block and the precompile ABI directory. It compares the indexer's progress with the source chain. Unified pagination exercises the explorer's combined account history.

The driver checks for Docker, OpenSSL, the Python `cryptography` package, prebuilt explorer dependencies and the local toolchain and PostgreSQL images before running its cases. Its argument parser defines the supported case selection and execution options.

The [explorer fixture](../../../tests/explorer_fixture.c) produces canonical fixture data. [Fixture checks](../../../tests/explorer_fixture_test.py) compare it with [the golden JSON](../../../tests/explorer_fixture_golden.json) and verify its signature.
