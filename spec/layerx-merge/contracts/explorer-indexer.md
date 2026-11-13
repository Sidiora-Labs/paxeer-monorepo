# Explorer indexer contract

The chain-specific `paxeer_x` import stage runs `AccountBindings` and `MarketEvents`. [Block import](../../../services/explorer/backend/apps/indexer/lib/indexer/block/fetcher.ex) supplies those decoded records. Anchor and receipt schemas support archive reads; receipt cursors identify archive ingestion progress.

## paxeer-x-ingestion

| Interface | Kind | Source | Signature |
|---|---|---|---|
| <code>interface.14.storage-lx-account-bindings-j54</code> | <code>storage_key</code> | [services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/account_binding.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/account_binding.ex) | <code>table lx_account_bindings</code> |
| <code>interface.14.storage-lx-anchors-j54</code> | <code>storage_key</code> | [services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/anchor.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/anchor.ex) | <code>table lx_anchors</code> |
| <code>interface.14.storage-lx-kernel-receipt-cursors-j54</code> | <code>storage_key</code> | [services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/receipt.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/receipt.ex) | <code>table lx_kernel_receipt_cursors</code> |
| <code>interface.14.storage-lx-market-events-j54</code> | <code>storage_key</code> | [services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/market_event.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/market_event.ex) | <code>table lx_market_events</code> |
| <code>interface.14.storage-lx-receipts-j54</code> | <code>storage_key</code> | [services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/receipt.ex](../../../services/explorer/backend/apps/explorer/lib/explorer/chain/paxeer_x/receipt.ex) | <code>table lx_receipts</code> |
