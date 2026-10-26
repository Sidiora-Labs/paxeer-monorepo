# Agent http contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## healthz

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.43.1</code> | req | 9043.1 |
| <code>acceptance.9.43.1</code> | scenario | Given the binary started against a loopback node that answers eth_chainId only after 300 ms, when /healthz is polled, then the first answer is 503 and a later answer is 200. |
| <code>acceptance.9.43.1</code> | test | services/agent/crates/layerx-agentd/host-tests/serve_routes.rs::healthz_reports_not_ready_until_the_node_answers (planned) |
| <code>acceptance.9.43.1</code> | qualification | UNRUN |
| <code>req.9043</code> | ac_1 | GET /healthz answers 503 until the readiness gate passes and 200 afterwards. |
| <code>capability.9.12.4</code> | title | Serve health and program balance routes from node JSON-RPC readiness |
| <code>capability.9.12.4</code> | reqs | 9043.1<br>9043.2 |

## programs-balances

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.26.1</code> | req | 9026.1 |
| <code>acceptance.9.26.1</code> | scenario | Given a node answering eth_call for program p-1 at height 15 with value 70, when the reader reads p-1 at height 15, then the value is 70 and freshness.value_height is 15. |
| <code>acceptance.9.26.1</code> | test | services/agent/crates/layerx-agentd/tests/program_balances_eth_call.rs::program_balance_is_read_by_eth_call_at_a_committed_height (planned) |
| <code>acceptance.9.26.1</code> | qualification | UNRUN |
| <code>req.9026</code> | ac_1 | A program balance read issues eth_call to 0x101D at the requested committed height and returns the decoded value with that height. |
| <code>acceptance.9.43.2</code> | req | 9043.2 |
| <code>acceptance.9.43.2</code> | scenario | Given a running binary, when /v1/programs/zz/balances is requested, then the response status is 400 with error invalid_program. |
| <code>acceptance.9.43.2</code> | test | services/agent/crates/layerx-agentd/host-tests/serve_routes.rs::unknown_program_balance_route_is_invalid_program (planned) |
| <code>acceptance.9.43.2</code> | qualification | UNRUN |
| <code>req.9043</code> | ac_2 | GET /v1/programs/&lt;malformed&gt;/balances answers invalid_program. |
| <code>capability.9.5.8</code> | title | Read program balances through eth_call on layerxprograms |
| <code>capability.9.5.8</code> | reqs | 9026.1 |

## reads-receipt

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.25.1</code> | req | 9025.1 |
| <code>acceptance.9.25.1</code> | scenario | Given a receipted transaction, when the native route reads receipt/&lt;hash&gt;, then the value carries the receipt, the key index and rung receipted. |
| <code>acceptance.9.25.1</code> | test | services/agent/crates/layerx-agentd/tests/native_routes_rpc.rs::native_receipt_route_serves_a_signed_receipt (planned) |
| <code>acceptance.9.25.1</code> | qualification | UNRUN |
| <code>req.9025</code> | ac_1 | GET /v1/reads/receipt/&lt;hash&gt; returns the sequencer receipt with the signing key index and rung Receipted. |
| <code>acceptance.9.25.2</code> | req | 9025.2 |
| <code>acceptance.9.25.2</code> | scenario | Given a NativeReadRoute, when it reads batch/1, then the result is NativeReadError::InvalidRequest. |
| <code>acceptance.9.25.2</code> | test | services/agent/crates/layerx-agentd/tests/native_routes_rpc.rs::native_route_rejects_an_unknown_kind (planned) |
| <code>acceptance.9.25.2</code> | qualification | UNRUN |
| <code>req.9025</code> | ac_2 | A native read of an unknown kind is refused with NativeReadError::InvalidRequest (services/agent/crates/layerx-agentd/src/read/native.rs:291). |
| <code>capability.9.5.7</code> | title | Serve the native read routes from node JSON-RPC |
| <code>capability.9.5.7</code> | reqs | 9025.1<br>9025.2 |
