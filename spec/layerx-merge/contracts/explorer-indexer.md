# Explorer indexer contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## paxeer-x-ingestion

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.14.storage-lx-account-bindings-j54</code> | <code>storage_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/account_binding.ex:42</code> | <code>table lx_account_bindings</code> |
| <code>interface.14.storage-lx-anchors-j54</code> | <code>storage_key</code> | <code>change</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/anchor.ex:49</code> | <code>table lx_anchors</code> |
| <code>interface.14.storage-lx-custody-events-j54</code> | <code>storage_key</code> | <code>change</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/custody_event.ex:52</code> | <code>table lx_custody_events</code> |
| <code>interface.14.storage-lx-deposit-roots-j54</code> | <code>storage_key</code> | <code>change</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/deposit_root.ex:49</code> | <code>table lx_deposit_roots</code> |
| <code>interface.14.storage-lx-guarantor-events-j54</code> | <code>storage_key</code> | <code>change</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/guarantor_event.ex:73</code> | <code>table lx_guarantor_events</code> |
| <code>interface.14.storage-lx-kernel-receipt-cursors-j54</code> | <code>storage_key</code> | <code>change</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/receipt.ex:305</code> | <code>table lx_kernel_receipt_cursors</code> |
| <code>interface.14.storage-lx-market-events-j54</code> | <code>storage_key</code> | <code>stable</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/market_event.ex:44</code> | <code>table lx_market_events</code> |
| <code>interface.14.storage-lx-receipts-j54</code> | <code>storage_key</code> | <code>change</code> | <code>services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/receipt.ex:72</code> | <code>table lx_receipts</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.14.storage-lx-account-bindings-j54</code> | notes | Still written by the bindings runner. |
| <code>interface.14.storage-lx-anchors-j54</code> | notes | Kept as a read-only archive; no writer after the runner is deleted. |
| <code>interface.14.storage-lx-custody-events-j54</code> | notes | Table kept, ingestion stops (decision.1421). |
| <code>interface.14.storage-lx-deposit-roots-j54</code> | notes | Table kept, ingestion stops (decision.1421). |
| <code>interface.14.storage-lx-guarantor-events-j54</code> | notes | Table kept, ingestion stops (decision.1421). |
| <code>interface.14.storage-lx-kernel-receipt-cursors-j54</code> | notes | Table kept; its only writer, the kernel receipts fetcher, is deleted. |
| <code>interface.14.storage-lx-market-events-j54</code> | notes | Still written by the market events runner. |
| <code>interface.14.storage-lx-receipts-j54</code> | notes | Kept as a read-only archive; no writer after the runner and fetcher are deleted. |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.255.1</code> | req | 14255.1 |
| <code>acceptance.14.255.1</code> | scenario | Given the compiled explorer app, when function_exported?/3 is called for changeset/2 on Anchor and Receipt, then both return false |
| <code>acceptance.14.255.1</code> | test | services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/archive_readonly_test.exs::archive_schemas_export_no_changeset (planned) |
| <code>acceptance.14.255.1</code> | qualification | UNRUN |
| <code>req.14255</code> | ac_1 | Explorer.Chain.PaxeerX.Anchor (services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/anchor.ex:1) and Receipt export no changeset/2. |
| <code>acceptance.14.256.1</code> | req | 14256.1 |
| <code>acceptance.14.256.1</code> | scenario | Given INDEXER_PAXEER_X_KERNEL_RECEIPTS_RELAY_URL set to a relay URL, when the indexer children list is built, then no child id is Indexer.Fetcher.PaxeerXKernelReceipts |
| <code>acceptance.14.256.1</code> | test | services/explorer/backend/apps/indexer/test/indexer/application_test.exs::indexer_children_ignore_kernel_receipts_env (planned) |
| <code>acceptance.14.256.1</code> | qualification | UNRUN |
| <code>req.14256</code> | ac_1 | With INDEXER_PAXEER_X_KERNEL_RECEIPTS_RELAY_URL set, Indexer.Application starts no kernel receipts child (services/explorer/backend/apps/indexer/lib/indexer/application.ex:90-94 deleted). |
| <code>acceptance.14.257.1</code> | req | 14257.1 |
| <code>acceptance.14.257.1</code> | scenario | Given the two READMEs after the change, when they are searched for the fetcher module, child name and variable prefix, then none is found |
| <code>acceptance.14.257.1</code> | test | services/explorer/tests/test_paxeer_x_readme_retirements.py::test_readmes_name_no_retired_fetcher (planned) |
| <code>acceptance.14.257.1</code> | qualification | UNRUN |
| <code>req.14257</code> | ac_1 | services/explorer/backend/README.md:35 and services/explorer/backend/apps/indexer/README.md:42 no longer name the retired fetcher or its variables. |
| <code>acceptance.14.258.1</code> | req | 14258.1 |
| <code>acceptance.14.258.1</code> | scenario | Given one custody-deposit log at 0x0000000000000000000000000000000000001013 and one checkpoint log at 0x0000000000000000000000000000000000001014, when PaxeerXLogs.parse/1 runs, then the result has only account_bindings and market_events, both empty |
| <code>acceptance.14.258.1</code> | test | services/explorer/backend/apps/indexer/test/indexer/transform/paxeer_x_logs_retired_test.exs::retired_precompile_logs_yield_no_rows (planned) |
| <code>acceptance.14.258.1</code> | qualification | UNRUN |
| <code>req.14258</code> | ac_1 | PaxeerXLogs.parse/1 (services/explorer/backend/apps/indexer/lib/indexer/transform/paxeer_x_logs.ex:704) returns no custody, deposit-root, anchor, receipt or guarantor rows for logs at 0x1013 and 0x1014. |
| <code>acceptance.14.259.1</code> | req | 14259.1 |
| <code>acceptance.14.259.1</code> | scenario | Given a parsed result with one binding and one market event, when the paxeer_x import options are built, then they hold exactly the two keys for bindings and market events |
| <code>acceptance.14.259.1</code> | test | services/explorer/backend/apps/indexer/test/indexer/block/fetcher_paxeer_x_options_test.exs::paxeer_x_import_options_carry_two_keys (planned) |
| <code>acceptance.14.259.1</code> | qualification | UNRUN |
| <code>req.14259</code> | ac_1 | do_import_options(:paxeer_x, ...) (services/explorer/backend/apps/indexer/lib/indexer/block/fetcher.ex:401-410) passes only the bindings and market event rows to the importer. |
| <code>acceptance.14.260.1</code> | req | 14260.1 |
| <code>acceptance.14.260.1</code> | scenario | Given the chain type paxeer_x, when the stage runner list is read, then it equals [Runner.PaxeerX.AccountBindings, Runner.PaxeerX.MarketEvents] |
| <code>acceptance.14.260.1</code> | test | services/explorer/backend/apps/explorer/test/explorer/chain/import/stage/paxeer_x_runners_test.exs::paxeer_x_stage_lists_two_runners (planned) |
| <code>acceptance.14.260.1</code> | qualification | UNRUN |
| <code>req.14260</code> | ac_1 | The paxeer_x stage at services/explorer/backend/apps/explorer/lib/explorer/chain/import/stage/chain_type_specific.ex:72-80 runs AccountBindings and MarketEvents only. |
| <code>acceptance.14.261.1</code> | req | 14261.1 |
| <code>acceptance.14.261.1</code> | scenario | Given a block import with one binding and one market event, when Import.all/1 runs, then lx_custody_events, lx_deposit_roots and lx_guarantor_events each hold zero rows |
| <code>acceptance.14.261.1</code> | test | services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/import_test.exs::import_writes_no_retired_tables (planned) |
| <code>acceptance.14.261.1</code> | qualification | UNRUN |
| <code>req.14261</code> | ac_1 | An import through services/explorer/backend/apps/explorer/test/explorer/chain/paxeer_x/import_test.exs:20 writes no row to lx_custody_events, lx_deposit_roots or lx_guarantor_events. |
| <code>acceptance.14.262.1</code> | req | 14262.1 |
| <code>acceptance.14.262.1</code> | scenario | Given the four precompile abi.json files, when each definition topic is compared with the event selectors of its address, then every topic is found |
| <code>acceptance.14.262.1</code> | test | services/explorer/backend/apps/indexer/test/indexer/transform/paxeer_x_logs_abi_test.exs::kept_topics_match_precompile_abis (planned) |
| <code>acceptance.14.262.1</code> | qualification | UNRUN |
| <code>req.14262</code> | ac_1 | Every topic in PaxeerXLogs.definitions/0 (services/explorer/backend/apps/indexer/lib/indexer/transform/paxeer_x_logs.ex:685) equals keccak256 of an event in the ABI of its precompile. |
| <code>acceptance.14.262.2</code> | req | 14262.2 |
| <code>acceptance.14.262.2</code> | scenario | Given PaxeerXLogs.definitions/0, when the addresses are collected, then neither 0x0000000000000000000000000000000000001013 nor 0x0000000000000000000000000000000000001014 is among them |
| <code>acceptance.14.262.2</code> | test | services/explorer/backend/apps/indexer/test/indexer/transform/paxeer_x_logs_abi_test.exs::no_definition_targets_retired_address (planned) |
| <code>acceptance.14.262.2</code> | qualification | UNRUN |
| <code>req.14262</code> | ac_2 | No PaxeerXLogs definition targets 0x1013 or 0x1014. |
| <code>acceptance.14.300.1</code> | req | 14300.1 |
| <code>acceptance.14.300.1</code> | scenario | Given one layerxexchange log and one launchpad log, when PaxeerXLogs.parse/1 runs, then market_events has two rows |
| <code>acceptance.14.300.1</code> | test | services/explorer/backend/apps/indexer/test/indexer/transform/paxeer_x_logs_market_test.exs::market_logs_still_parse (planned) |
| <code>acceptance.14.300.1</code> | qualification | UNRUN |
| <code>req.14300</code> | ac_1 | PaxeerXLogs.parse/1 (services/explorer/backend/apps/indexer/lib/indexer/transform/paxeer_x_logs.ex:704) still yields market_events rows for exchange and launchpad logs. |
| <code>acceptance.14.300.2</code> | req | 14300.2 |
| <code>acceptance.14.300.2</code> | scenario | Given one addr binding log, when PaxeerXLogs.parse/1 runs, then account_bindings has one row |
| <code>acceptance.14.300.2</code> | test | services/explorer/backend/apps/indexer/test/indexer/transform/paxeer_x_logs_market_test.exs::binding_log_still_parses (planned) |
| <code>acceptance.14.300.2</code> | qualification | UNRUN |
| <code>req.14300</code> | ac_2 | PaxeerXLogs.parse/1 still yields an account_bindings row for an addr binding log. |
| <code>capability.14.23.6</code> | title | Make the Anchor and Receipt chain schemas read-only archive records |
| <code>capability.14.23.6</code> | reqs | 14255.1 |
| <code>capability.14.24.1</code> | title | Remove the kernel receipts fetcher and its INDEXER_PAXEER_X_KERNEL_RECEIPTS_* settings |
| <code>capability.14.24.1</code> | reqs | 14256.1 |
| <code>capability.14.24.2</code> | title | Drop the kernel receipts fetcher from the backend and indexer READMEs |
| <code>capability.14.24.2</code> | reqs | 14257.1 |
| <code>capability.14.25.1</code> | title | Stop parsing custody, deposit-root, anchor and guarantor logs in PaxeerXLogs |
| <code>capability.14.25.1</code> | reqs | 14258.1 |
| <code>capability.14.25.2</code> | title | Import only bindings and market events from the block fetcher |
| <code>capability.14.25.2</code> | reqs | 14259.1 |
| <code>capability.14.25.3</code> | title | Run only the bindings and market runners in the paxeer_x import stage |
| <code>capability.14.25.3</code> | reqs | 14260.1 |
| <code>capability.14.25.4</code> | title | Delete the custody, deposit-root and guarantor chain schemas |
| <code>capability.14.25.4</code> | reqs | 14261.1 |
| <code>capability.14.25.5</code> | title | Check every kept PaxeerXLogs topic against the precompile ABI files |
| <code>capability.14.25.5</code> | reqs | 14262.1<br>14262.2 |
| <code>capability.14.38.1</code> | title | Check market and binding rows still parse after the retired definitions leave |
| <code>capability.14.38.1</code> | reqs | 14300.1<br>14300.2 |
| <code>resolution.17.946</code> | title | decision.1421 binds: Keep the lx_* tables and stop writing the retired ones |
| <code>resolution.17.946</code> | applies_to | decision.1421<br>task.14.25.1<br>task.14.25.3<br>task.14.25.4 |
| <code>resolution.17.946</code> | source | decision.1421 in fragments/J54/decision.kvx; tasks from affects |

## rpc-parity

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.14.267.1</code> | req | 14267.1 |
| <code>acceptance.14.267.1</code> | scenario | Given new_head_full.json, when new_head_matches_block?/2 compares its notification and block, then it returns true |
| <code>acceptance.14.267.1</code> | test | services/explorer/backend/apps/ethereum_jsonrpc/test/ethereum_jsonrpc/paxeer_x_new_head_parity_test.exs::full_new_head_matches_block_body (planned) |
| <code>acceptance.14.267.1</code> | qualification | UNRUN |
| <code>req.14267</code> | ac_1 | new_head_matches_block?/2 (services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/paxeer_x.ex:247-254) accepts a newHeads header whose fields equal the block body. |
| <code>acceptance.14.268.1</code> | req | 14268.1 |
| <code>acceptance.14.268.1</code> | scenario | Given new_head_full.json with one byte of receiptsRoot flipped, when new_head_matches_block?/2 runs, then it returns false |
| <code>acceptance.14.268.1</code> | test | services/explorer/backend/apps/ethereum_jsonrpc/test/ethereum_jsonrpc/paxeer_x_new_head_fields_test.exs::new_head_with_other_receipts_root_is_rejected (planned) |
| <code>acceptance.14.268.1</code> | qualification | UNRUN |
| <code>req.14268</code> | ac_1 | new_head_matches_block?/2 (services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/paxeer_x.ex:247-254) returns false when receiptsRoot or parentHash differ. |
| <code>acceptance.14.269.1</code> | req | 14269.1 |
| <code>acceptance.14.269.1</code> | scenario | Given tx_count_parity.json, when the block is decoded and the count method answer read, then both counts are 2 |
| <code>acceptance.14.269.1</code> | test | services/explorer/backend/apps/ethereum_jsonrpc/test/ethereum_jsonrpc/paxeer_x_tx_count_parity_test.exs::block_tx_count_matches_count_method (planned) |
| <code>acceptance.14.269.1</code> | qualification | UNRUN |
| <code>req.14269</code> | ac_1 | The number of transactions the indexer decodes for a block equals eth_getBlockTransactionCountByNumber for that height. |
| <code>acceptance.14.270.1</code> | req | 14270.1 |
| <code>acceptance.14.270.1</code> | scenario | Given call_tracer_block.json, when fetch_block_internal_transactions/2 runs over a transport answering it, then two internal transactions with types call and create are returned |
| <code>acceptance.14.270.1</code> | test | services/explorer/backend/apps/ethereum_jsonrpc/test/ethereum_jsonrpc/paxeer_x_trace_parity_test.exs::call_tracer_block_decodes_internal_transactions (planned) |
| <code>acceptance.14.270.1</code> | qualification | UNRUN |
| <code>req.14270</code> | ac_1 | A callTracer block trace decodes into one call and one create internal transaction with the trace indexes the node assigns. |
| <code>acceptance.14.271.1</code> | req | 14271.1 |
| <code>acceptance.14.271.1</code> | scenario | Given a transport answering eth_syncing with currentBlock and highestBlock, when fetch_syncing_status/1 runs, then it reports syncing with both heights |
| <code>acceptance.14.271.1</code> | test | services/explorer/backend/apps/ethereum_jsonrpc/test/ethereum_jsonrpc/paxeer_x_syncing_parity_test.exs::syncing_object_reports_not_synced (planned) |
| <code>acceptance.14.271.1</code> | qualification | UNRUN |
| <code>req.14271</code> | ac_1 | fetch_syncing_status/1 (services/explorer/backend/apps/ethereum_jsonrpc/lib/ethereum_jsonrpc/paxeer_x.ex:299) reports not synced for an eth_syncing object answer. |
| <code>acceptance.14.271.2</code> | req | 14271.2 |
| <code>acceptance.14.271.2</code> | scenario | Given a transport answering false, when fetch_syncing_status/1 runs, then it reports synced and the recorded method is eth_syncing |
| <code>acceptance.14.271.2</code> | test | services/explorer/backend/apps/ethereum_jsonrpc/test/ethereum_jsonrpc/paxeer_x_syncing_parity_test.exs::syncing_false_reports_synced (planned) |
| <code>acceptance.14.271.2</code> | qualification | UNRUN |
| <code>req.14271</code> | ac_2 | fetch_syncing_status/1 reports synced for an eth_syncing false answer and sends method eth_syncing. |
| <code>capability.14.28.1</code> | title | Freeze a newHeads fixture whose header fields match eth_getBlockByNumber |
| <code>capability.14.28.1</code> | reqs | 14267.1 |
| <code>capability.14.28.2</code> | title | Compare receiptsRoot and parentHash when matching a newHeads header to its block |
| <code>capability.14.28.2</code> | reqs | 14268.1 |
| <code>capability.14.28.3</code> | title | Check block transaction counts against eth_getBlockTransactionCountByNumber |
| <code>capability.14.28.3</code> | reqs | 14269.1 |
| <code>capability.14.28.4</code> | title | Check call_tracer traces decode into internal transactions |
| <code>capability.14.28.4</code> | reqs | 14270.1 |
| <code>capability.14.28.5</code> | title | Check eth_syncing answers decode after the method rename |
| <code>capability.14.28.5</code> | reqs | 14271.1<br>14271.2 |
| <code>resolution.17.954</code> | title | decision.1429 binds: newHeads headers are compared on parentHash and receiptsRoot as well |
| <code>resolution.17.954</code> | applies_to | decision.1429<br>task.14.28.1<br>task.14.28.2 |
| <code>resolution.17.954</code> | source | decision.1429 in fragments/J54/decision.kvx; tasks from affects |
