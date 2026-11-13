# Arbitrum confirmation processing

The Arbitrum indexer discovers `SendRootUpdated` events from the parent-chain
outbox and links confirmed rollup blocks to parent-chain lifecycle
transactions. This is Arbitrum settlement indexing; native Paxeer finality
comes from committed BFT blocks.

| Source | Responsibility |
| --- | --- |
| [tasks.ex](tasks.ex) | Schedule new and historical confirmation discovery |
| [events.ex](events.ex) | Fetch, filter and parse `SendRootUpdated` logs |
| [discovery.ex](discovery.ex) | Select rollup ranges and prepare confirmation imports |
| [rollup_blocks.ex](rollup_blocks.ex) | Associate rollup blocks with the transaction that first confirmed them |

Tasks call discovery, which correlates outbox events with indexed batches
and rollup blocks. A confirmation covers its rollup block and earlier blocks,
but the stored associations preserve the first confirming transaction for
each range. For example, when an earlier parent-chain transaction already
confirmed a lower range, a later event is linked only to the newly confirmed
blocks above that range.

Imports set each affected rollup block's `confirmation_id` and advance
included L2-to-L1 messages to `:confirmed`. Existing parent-chain transaction
records have their block numbers and timestamps updated when their placement
changes. Discovery uses safe block bounds or overlapping log ranges according
to its parent-chain configuration.

`Discovery.perform/5` returns `:ok` when the range is processed or
`:confirmation_missed` when required batches or rollup blocks are unavailable.
The task retains the range for retry when a confirmation is missed. Its
scheduling distinguishes normal polling, catch-up after success and waiting
for the required migration to become ready.
