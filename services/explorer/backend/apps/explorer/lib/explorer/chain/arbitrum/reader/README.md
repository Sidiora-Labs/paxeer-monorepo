# Arbitrum reader modules

These modules query the explorer's Arbitrum rollup data. They support the
Arbitrum chain configuration; native Paxeer finality follows committed BFT
blocks as described in the [backend overview](../../../../../../../README.md).

| Source | Queries |
| --- | --- |
| [api/messages.ex](api/messages.ex) | Cross-chain messages for API responses |
| [api/settlement.ex](api/settlement.ex) | Batches, DA records and rollup block associations |
| [api/general.ex](api/general.ex) | Transaction logs for API responses |
| [common.ex](common.ex) | Queries shared by API and indexer readers, with repository selection |
| [indexer/messages.ex](indexer/messages.ex) | Message progress and missing message data |
| [indexer/parent_chain_transactions.ex](indexer/parent_chain_transactions.ex) | Parent-chain transaction lifecycle records |
| [indexer/settlement.ex](indexer/settlement.ex) | Batch, confirmation and DA data |
| [indexer/general.ex](indexer/general.ex) | Rollup blocks and blocks with missing fields |

The indexer calls these readers through
[Indexer.Fetcher.Arbitrum.Utils.Db](../../../../../../indexer/lib/indexer/fetcher/arbitrum/utils/db/README.md)
wrappers. The wrappers convert database records into the maps required by
indexer consumers and prepare import data.
