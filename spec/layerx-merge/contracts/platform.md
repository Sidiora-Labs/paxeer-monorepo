# Platform contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## gateway-explorer-proxy

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.8.http-gateway-get-api-v2-paxeer-x-archive-receipts-id-j79</code> | <code>http_route</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs</code> (planned) | <code>GET /api/v2/paxeer-x/archive/receipts/:id (proxied to the explorer archive)</code> |
| <code>interface.8.http-gateway-get-api-v2-paxeer-x-archive-receipts-j79</code> | <code>http_route</code> | <code>new</code> | <code>services/platform/hosted/gateway/src/explorer_proxy.rs</code> (planned) | <code>GET /api/v2/paxeer-x/archive/receipts (proxied to the explorer archive)</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.8.http-gateway-get-api-v2-paxeer-x-archive-receipts-id-j79</code> | notes | replaces the receipts allowlist entries at services/platform/hosted/gateway/src/explorer_proxy.rs:540-544 and :920-924 (RULING J54.3, decision.891) |
| <code>interface.8.http-gateway-get-api-v2-paxeer-x-archive-receipts-j79</code> | notes | replaces the receipts allowlist entries at services/platform/hosted/gateway/src/explorer_proxy.rs:540-544 and :920-924 (RULING J54.3, decision.891) |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.8.990.1</code> | req | 8990.1 |
| <code>acceptance.8.990.1</code> | scenario | Given the explorer proxy allowlist, when an archive receipt list or receipt-by-id request arrives, then it is proxied and a retired paxeer-x receipt route is refused |
| <code>acceptance.8.990.1</code> | test | services/platform/hosted/gateway/src/explorer_proxy.rs::explorer_proxies_archive_receipt_views (planned) |
| <code>acceptance.8.990.1</code> | qualification | UNRUN |
| <code>req.8990</code> | ac_1 | explorer_proxies_archive_receipt_views allows the archive receipt list and receipt-by-id routes and refuses the retired paxeer-x receipt routes. |
