# Mcp tools contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## activity-sign

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.49.1</code> | req | 9049.1 |
| <code>acceptance.9.49.1</code> | scenario | Given a bound MCP server and a prepared activity, when activity.sign is invoked with a 64-byte hex signature, then the refusal reason is mcp.external_signature and the daemon receives no sign request. |
| <code>acceptance.9.49.1</code> | test | services/agent/crates/layerx-mcp/tests/activity_evm_signatures.rs::activity_sign_refuses_a_64_byte_signature (planned) |
| <code>acceptance.9.49.1</code> | qualification | UNRUN |
| <code>req.9049</code> | ac_1 | activity.sign with external_signature_v1 refuses a 64-byte signature with reason mcp.external_signature (services/agent/crates/layerx-mcp/src/binding.rs:411). |
| <code>capability.9.14.1</code> | title | Accept 65-byte external signatures in the MCP activity tools |
| <code>capability.9.14.1</code> | reqs | 9049.1<br>9049.2 |

## activity-submit

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.49.2</code> | req | 9049.2 |
| <code>acceptance.9.49.2</code> | scenario | Given the MCP catalogue, when the activity.submit input schema is read, then signature has 130 hex characters, signer_address has 40 and signer_public_key is absent. |
| <code>acceptance.9.49.2</code> | test | services/agent/crates/layerx-mcp/tests/activity_evm_signatures.rs::activity_submit_schema_declares_signer_address (planned) |
| <code>acceptance.9.49.2</code> | qualification | UNRUN |
| <code>req.9049</code> | ac_2 | The native submit schema requires a 65-byte signature and a 20-byte signer_address and no signer_public_key. |

## approval-get

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.53.2</code> | req | 9053.2 |
| <code>acceptance.9.53.2</code> | scenario | Given a pending approval for a prepared transfer, when approval.get is invoked, then the result carries tx_hash null before signing and rung null, and no activity_id key. |
| <code>acceptance.9.53.2</code> | test | services/agent/crates/layerx-mcp/tests/session_tools_major_two.rs::approval_get_tool_reports_tx_hash_and_rung (planned) |
| <code>acceptance.9.53.2</code> | qualification | UNRUN |
| <code>req.9053</code> | ac_2 | approval.get returns the approval's tx_hash and rung. |

## balance-get

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.50.2</code> | req | 9050.2 |
| <code>acceptance.9.50.2</code> | scenario | Given a daemon answering read.balance with achieved_rung null, when balance.get is invoked, then the tool result rung is None and freshness names chain_head and value_height. |
| <code>acceptance.9.50.2</code> | test | services/agent/crates/layerx-mcp/tests/read_tools_rungs.rs::balance_tool_has_no_rung_for_ordinary_reads (planned) |
| <code>acceptance.9.50.2</code> | qualification | UNRUN |
| <code>req.9050</code> | ac_2 | balance.get returns rung None and the height freshness for an ordinary read. |

## binding-document

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.54.1</code> | req | 9054.1 |
| <code>acceptance.9.54.1</code> | scenario | Given a binding document with core_sequence = 5, when the MCP server binds it, then the refusal reason is mcp.binding_retired_key. |
| <code>acceptance.9.54.1</code> | test | services/agent/crates/layerx-mcp/tests/binding_heights.rs::binding_document_with_core_sequence_is_refused (planned) |
| <code>acceptance.9.54.1</code> | qualification | UNRUN |
| <code>req.9054</code> | ac_1 | A binding document holding core_sequence is refused with reason mcp.binding_retired_key. |
| <code>acceptance.9.54.2</code> | req | 9054.2 |
| <code>acceptance.9.54.2</code> | scenario | Given a binding document with core_height = 5 and the other required keys, when the MCP server binds it, then binding succeeds and the session reports core height 5. |
| <code>acceptance.9.54.2</code> | test | services/agent/crates/layerx-mcp/tests/binding_heights.rs::binding_document_with_core_height_binds (planned) |
| <code>acceptance.9.54.2</code> | qualification | UNRUN |
| <code>req.9054</code> | ac_2 | A binding document with core_height binds and reports that height. |
| <code>acceptance.9.55.2</code> | req | 9055.2 |
| <code>acceptance.9.55.2</code> | scenario | Given a binding web block with sequencer_public_key, when the MCP server binds it, then it is refused naming sequencer_public_key; with receipt_key_index it binds. |
| <code>acceptance.9.55.2</code> | test | services/agent/crates/layerx-mcp/tests/web_payments_layerxpay.rs::web_binding_names_a_receipt_key_index (planned) |
| <code>acceptance.9.55.2</code> | qualification | UNRUN |
| <code>req.9055</code> | ac_2 | The web binding requires receipt_key_index and refuses sequencer_public_key. |
| <code>capability.9.15.4</code> | title | Bind MCP sessions by block height |
| <code>capability.9.15.4</code> | reqs | 9054.1<br>9054.2 |

## faucet-request

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.51.1</code> | req | 9051.1 |
| <code>acceptance.9.51.1</code> | scenario | Given a loopback node answering px_requestFunds with a hash, when faucet.request is invoked under scope write:faucet:claim, then the request names the session address and the tool result carries that hash. |
| <code>acceptance.9.51.1</code> | test | services/agent/crates/layerx-mcp/tests/faucet_request_funds.rs::faucet_request_calls_px_request_funds (planned) |
| <code>acceptance.9.51.1</code> | qualification | UNRUN |
| <code>req.9051</code> | ac_1 | faucet.request sends px_requestFunds with the session address and returns the funding transaction hash. |
| <code>acceptance.9.51.2</code> | req | 9051.2 |
| <code>acceptance.9.51.2</code> | scenario | Given the MCP server, when tool_operation is asked for faucet.request, then it returns None. |
| <code>acceptance.9.51.2</code> | test | services/agent/crates/layerx-mcp/tests/faucet_request_funds.rs::faucet_request_maps_to_no_agent_operation (planned) |
| <code>acceptance.9.51.2</code> | qualification | UNRUN |
| <code>req.9051</code> | ac_2 | tool_operation returns None for faucet.request (services/agent/crates/layerx-mcp/src/server.rs:817). |
| <code>capability.9.15.1</code> | title | Serve faucet.request through px_requestFunds |
| <code>capability.9.15.1</code> | reqs | 9051.1<br>9051.2 |

## grant-draw

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.52.2</code> | req | 9052.2 |
| <code>acceptance.9.52.2</code> | scenario | Given a budget b-1, when grant.draw is invoked for 3 units, then the prepared call target is 0x101B with selector draw and arguments b-1 and 3. |
| <code>acceptance.9.52.2</code> | test | services/agent/crates/layerx-mcp/tests/alias_precompiles.rs::grant_draw_targets_the_pay_precompile (planned) |
| <code>acceptance.9.52.2</code> | qualification | UNRUN |
| <code>req.9052</code> | ac_2 | grant.draw prepares a draw call to 0x101B. |

## receipt-get

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.50.1</code> | req | 9050.1 |
| <code>acceptance.9.50.1</code> | scenario | Given a daemon answering read.receipt with achieved_rung receipted at height 12, when receipt.get is invoked, then the tool result carries rung Receipted and value_height 12. |
| <code>acceptance.9.50.1</code> | test | services/agent/crates/layerx-mcp/tests/read_tools_rungs.rs::receipt_tool_reports_the_receipted_rung (planned) |
| <code>acceptance.9.50.1</code> | qualification | UNRUN |
| <code>req.9050</code> | ac_1 | receipt.get returns rung receipted and the receipt's block height for a sequencer-signed receipt. |
| <code>capability.9.14.2</code> | title | Report rungs and block heights from the MCP read tools |
| <code>capability.9.14.2</code> | reqs | 9050.1<br>9050.2 |

## subscription-acknowledge

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.53.1</code> | req | 9053.1 |
| <code>acceptance.9.53.1</code> | scenario | Given the MCP catalogue, when subscription.acknowledge is invoked with cursor 7, then it is refused as malformed; with a triple it reaches the daemon. |
| <code>acceptance.9.53.1</code> | test | services/agent/crates/layerx-mcp/tests/session_tools_major_two.rs::subscription_acknowledge_tool_takes_a_cursor_triple (planned) |
| <code>acceptance.9.53.1</code> | qualification | UNRUN |
| <code>req.9053</code> | ac_1 | subscription.acknowledge requires block_height, tx_index and log_index and refuses a single integer cursor. |
| <code>capability.9.15.3</code> | title | Serve approval and subscription tools with major-2 shapes |
| <code>capability.9.15.3</code> | reqs | 9053.1<br>9053.2 |

## wallet-send

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.52.1</code> | req | 9052.1 |
| <code>acceptance.9.52.1</code> | scenario | Given a bound MCP server, when wallet.send is invoked for 5 units to address R, then the prepared call target is 0x101A with selector transfer and arguments R and 5. |
| <code>acceptance.9.52.1</code> | test | services/agent/crates/layerx-mcp/tests/alias_precompiles.rs::wallet_send_targets_the_ledger_precompile (planned) |
| <code>acceptance.9.52.1</code> | qualification | UNRUN |
| <code>req.9052</code> | ac_1 | wallet.send prepares a transfer call to 0x101A. |
| <code>capability.9.15.2</code> | title | Map wallet, token and grant aliases onto ledger and pay precompiles |
| <code>capability.9.15.2</code> | reqs | 9052.1<br>9052.2 |

## web-fetch

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.55.1</code> | req | 9055.1 |
| <code>acceptance.9.55.1</code> | scenario | Given a web endpoint quoting an exact price of 2 units, when web.fetch is invoked, then the payer prepares payExact to 0x101B with amount 2 and the recipient address. |
| <code>acceptance.9.55.1</code> | test | services/agent/crates/layerx-mcp/tests/web_payments_layerxpay.rs::web_fetch_pays_with_pay_exact (planned) |
| <code>acceptance.9.55.1</code> | qualification | UNRUN |
| <code>req.9055</code> | ac_1 | web.fetch with an exact price pays through a payExact call to 0x101B. |
| <code>acceptance.9.56.1</code> | req | 9056.1 |
| <code>acceptance.9.56.1</code> | scenario | Given a payer with receipt key set K and a receipt signed by a key not in K, when receipt verification runs, then the result is a WebPayerError and no payment is reported. |
| <code>acceptance.9.56.1</code> | test | services/agent/crates/layerx-agentd-host/tests/payer.rs::web_payment_receipt_must_be_signed_by_a_published_key (planned) |
| <code>acceptance.9.56.1</code> | qualification | UNRUN |
| <code>req.9056</code> | ac_1 | A payExact receipt signed by a key outside px_getReceiptKeys is refused with WebPayerError. |
| <code>acceptance.9.56.2</code> | req | 9056.2 |
| <code>acceptance.9.56.2</code> | scenario | Given a receipt signed by a published key with a PaidExact log of 2 units to R, when receipt verification runs, then the payment reports amount 2 and recipient R. |
| <code>acceptance.9.56.2</code> | test | services/agent/crates/layerx-agentd-host/tests/payer.rs::web_payment_reports_the_pay_exact_log_amount (planned) |
| <code>acceptance.9.56.2</code> | qualification | UNRUN |
| <code>req.9056</code> | ac_2 | A verified payExact receipt reports the amount and recipient from its log. |
| <code>capability.9.16.1</code> | title | Pay for web tools with layerxpay payExact and draw |
| <code>capability.9.16.1</code> | reqs | 9055.1<br>9055.2 |
| <code>capability.9.16.2</code> | title | Settle agentd-host web payments through layerxpay receipts |
| <code>capability.9.16.2</code> | reqs | 9056.1<br>9056.2 |
