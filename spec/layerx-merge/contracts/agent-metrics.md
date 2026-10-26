# Agent metrics contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## degraded-state

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.57.2</code> | req | 9057.2 |
| <code>acceptance.9.57.2</code> | scenario | Given each DegradedState variant, when it is rendered as a metric label, then the four labels are node_behind, node_unavailable, chain_halted and block_unavailable. |
| <code>acceptance.9.57.2</code> | test | services/agent/crates/layerx-agentd/tests/metrics_rungs.rs::degraded_state_labels_name_node_conditions (planned) |
| <code>acceptance.9.57.2</code> | qualification | UNRUN |
| <code>req.9057</code> | ac_2 | DegradedState renders node_behind, node_unavailable, chain_halted and block_unavailable. |

## rung

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.57.1</code> | req | 9057.1 |
| <code>acceptance.9.57.1</code> | scenario | Given the metrics registry, when the rung metric family is rendered, then its label values are submitted, receipted and committed and none names sequencer_signed or settlement_anchored. |
| <code>acceptance.9.57.1</code> | test | services/agent/crates/layerx-agentd/tests/metrics_rungs.rs::rung_metric_labels_are_exactly_the_three_rungs (planned) |
| <code>acceptance.9.57.1</code> | qualification | UNRUN |
| <code>req.9057</code> | ac_1 | The rung metric exposes exactly the labels submitted, receipted and committed. |
| <code>capability.9.17.1</code> | title | Label metrics and tenant errors by rung and node availability |
| <code>capability.9.17.1</code> | reqs | 9057.1<br>9057.2 |
