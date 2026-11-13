# Agent MCP contract

The MCP server exposes JSON-RPC over stdio or its configured Unix listener. Tool calls pass through a tenant-, session- and capability-bound daemon boundary. Read-only mode omits write tools. Tool names and operation mappings are defined in [server.rs](../../../services/agent/crates/layerx-mcp/src/server.rs); argument schemas are in [catalogue.rs](../../../services/agent/crates/layerx-mcp/src/catalogue.rs).

## activity-disclose

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-activity-disclose</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=activity.disclose -&gt; agent operation prepare</code> |


## activity-prepare

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-activity-prepare</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=activity.prepare -&gt; agent operation prepare</code> |


## activity-sign

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-activity-sign</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=activity.sign -&gt; agent operation sign</code> |


## activity-submit

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-activity-submit</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=activity.submit -&gt; agent operation submit</code> |


## activity-track

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-activity-track</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=activity.track -&gt; agent operation track</code> |


## activity-wait

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-activity-wait</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=activity.wait -&gt; agent operation wait</code> |


## approval-approve

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-approval-approve</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=approval.approve -&gt; agent operation approval.approve</code> |


## approval-get

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-approval-get</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=approval.get -&gt; agent operation approval.get</code> |


## approval-list

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-approval-list</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=approval.list -&gt; agent operation approval.list</code> |


## approval-reject

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-approval-reject</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=approval.reject -&gt; agent operation approval.reject</code> |


## availability-get

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-availability-get</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=availability.get -&gt; agent operation availability.fetch</code> |


## balance-get

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-balance-get</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=balance.get -&gt; agent operation read.balance</code> |


## checkpoint-get

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-checkpoint-get</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=checkpoint.get -&gt; agent operation read.checkpoint</code> |


## faucet-request

`faucet.request` uses the dedicated faucet execution path, authorized with `write:faucet:claim`. It is not mapped to the retired `faucet.claim` agent operation. The tool definition describes a testnet funding request through `px_requestFunds`, returning its transaction hash or a refusal.

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-faucet-request</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=faucet.request; dedicated funding route</code> |


## grant-draw

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-grant-draw</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=grant.draw -&gt; agent operation submit</code> |


## grant-issue

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-grant-issue</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=grant.issue -&gt; agent operation submit</code> |


## history-list

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-history-list</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=history.list -&gt; agent operation read.history</code> |


## jsonrpc-errors

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.err-mcp-jsonrpc-32600</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>MCP JSON-RPC error -32600 (invalid request)</code> |
| <code>interface.9.err-mcp-jsonrpc-32601</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>MCP JSON-RPC error -32601 (method not found)</code> |
| <code>interface.9.err-mcp-jsonrpc-32602</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>MCP JSON-RPC error -32602 (invalid params)</code> |
| <code>interface.9.err-mcp-jsonrpc-32700</code> | <code>error_code</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>MCP JSON-RPC error -32700 (parse error)</code> |

## jsonrpc-initialize

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.rpc-mcp-initialize</code> | <code>jsonrpc_method</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>MCP JSON-RPC 2.0 method initialize (standalone stdio server; daemon-bound server at services/agent/crates/layerx-mcp/src/stdio.rs)</code> |


## jsonrpc-ping

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.rpc-mcp-ping</code> | <code>jsonrpc_method</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>MCP JSON-RPC 2.0 method ping (standalone stdio server; daemon-bound server at services/agent/crates/layerx-mcp/src/stdio.rs)</code> |


## jsonrpc-tools-call

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.rpc-mcp-tools-call</code> | <code>jsonrpc_method</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>MCP JSON-RPC 2.0 method tools/call (standalone stdio server; daemon-bound server at services/agent/crates/layerx-mcp/src/stdio.rs)</code> |


## jsonrpc-tools-list

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.rpc-mcp-tools-list</code> | <code>jsonrpc_method</code> | <code>services/agent/crates/layerx-mcp/src/stdio.rs</code> | <code>MCP JSON-RPC 2.0 method tools/list (standalone stdio server; daemon-bound server at services/agent/crates/layerx-mcp/src/stdio.rs)</code> |


## proof-get

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-proof-get</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=proof.get -&gt; agent operation read.proof_bundle</code> |


## receipt-get

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-receipt-get</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=receipt.get -&gt; agent operation program.receipt</code> |


## subscription-acknowledge

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-subscription-acknowledge</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=subscription.acknowledge -&gt; agent operation subscription.acknowledge</code> |


## subscription-create

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-subscription-create</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=subscription.create -&gt; agent operation subscription.create</code> |


## subscription-delete

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-subscription-delete</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=subscription.delete -&gt; agent operation subscription.delete</code> |


## subscription-health

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-subscription-health</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=subscription.health -&gt; agent operation subscription.health</code> |


## subscription-list

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-subscription-list</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=subscription.list -&gt; agent operation subscription.list</code> |


## subscription-pause

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-subscription-pause</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=subscription.pause -&gt; agent operation subscription.pause</code> |


## subscription-resume

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-subscription-resume</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=subscription.resume -&gt; agent operation subscription.resume</code> |


## tenant-readiness

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-tenant-readiness</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=tenant.readiness -&gt; agent operation tenant.readiness</code> |

## token-create

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-token-create</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=token.create -&gt; agent operation submit</code> |


## token-mint

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-token-mint</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=token.mint -&gt; agent operation submit</code> |


## token-transfer

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-token-transfer</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=token.transfer -&gt; agent operation submit</code> |


## wallet-accounts

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-wallet-accounts</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=wallet.accounts -&gt; agent operation read.account</code> |


## wallet-balance

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-wallet-balance</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=wallet.balance -&gt; agent operation read.balance</code> |


## wallet-send

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-wallet-send</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/server.rs</code> | <code>MCP tools/call name=wallet.send -&gt; agent operation submit</code> |


## web-content

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-web-content</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=web.content -&gt; agent operation submit</code> |


## web-fetch

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-web-fetch</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=web.fetch -&gt; agent operation submit</code> |


## web-search

| Interface | Kind | Source | Signature |
| --- | --- | --- | --- |
| <code>interface.9.http-mcp-tool-web-search</code> | <code>http_route</code> | <code>services/agent/crates/layerx-mcp/src/catalogue.rs</code> | <code>MCP tools/call name=web.search -&gt; agent operation submit</code> |

