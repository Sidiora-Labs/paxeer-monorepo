The row recording is copied from `apps/web/e2e/fixtures/px-get-unified-history.json`,
which `unified_history_answer_matches_the_web_fixture` in the gateway history
module records from the real Rust indexer store and API. `responses.json`
projects its original indexer fields, asset records and readiness document into
HTTP responses; the pages retain the original row positions and exact amounts.
The registry record comes from that producer's `indexer()` fixture. The empty
holder envelope is the store's recorded empty-list result asserted by
`px_list_holder_records_pages_the_indexer_holder_route_to_the_end`.

Tests replay these documents over loopback HTTP. Native registry records and
bindings are produced separately by the real node keepers and committed before
RPC reads. Account creation emits no transfer, so those accounts replay the
canonical empty history page. No executable, external endpoint or credential
is needed to replay the records.
