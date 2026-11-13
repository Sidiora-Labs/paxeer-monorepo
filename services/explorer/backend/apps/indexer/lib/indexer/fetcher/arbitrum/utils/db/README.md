# Arbitrum database utilities

These modules wrap
[Explorer.Chain.Arbitrum.Reader](../../../../../../../explorer/lib/explorer/chain/arbitrum/reader/README.md)
queries and prepare records for indexer consumers and imports.

| Source | Responsibility |
| --- | --- |
| [common.ex](common.ex) | Indexed-block checks, timestamp lookup, rollup blocks and blocks with missing fields |
| [messages.ex](messages.ex) | Message progress and missing message/execution data |
| [parent_chain_transactions.ex](parent_chain_transactions.ex) | Lifecycle transaction conversion and identifier allocation |
| [settlement.ex](settlement.ex) | Batches, confirmations, gaps and DA lookups |
| [tools.ex](tools.ex) | Record-to-map conversion |

`Common.indexed_blocks?/2` checks indexed ranges;
`Common.closest_block_after_timestamp/1` finds a block by timestamp.
`Common.rollup_blocks/1` and `Common.blocks_with_missing_fields/2` return
rollup block data used by discovery. These queries operate on Arbitrum data.

`Tools.db_record_to_map/3` selects record fields and optionally encodes
binary hashes; its third argument defaults to `false`. The other wrappers
apply conversions appropriate to the record type before returning data to
fetchers.
