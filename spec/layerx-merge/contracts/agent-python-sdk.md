# Agent python sdk contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## px

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.9.381.1</code> | req | 9381.1 |
| <code>acceptance.9.381.1</code> | scenario | Given a px_getNetwork result carrying an anchor member, when the test decodes it, then it raises ValueError and the module has no PxAnchorHead attribute. |
| <code>acceptance.9.381.1</code> | test | services/agent/sdk/python/tests/test_px_commit.py::test_network_head_has_no_anchor (planned) |
| <code>acceptance.9.381.1</code> | qualification | UNRUN |
| <code>req.9381</code> | ac_1 | decode_px_network_head refuses an anchor member and PxAnchorHead no longer exists. |
| <code>acceptance.9.381.2</code> | req | 9381.2 |
| <code>acceptance.9.381.2</code> | scenario | Given block height 7, tx index 2 and log index 1, when the test builds the cursor and history params, then the cursor is 00000000000000070000000200000001 and params[1] equals it. |
| <code>acceptance.9.381.2</code> | test | services/agent/sdk/python/tests/test_px_commit.py::test_history_cursor_is_block_position (planned) |
| <code>acceptance.9.381.2</code> | qualification | UNRUN |
| <code>req.9381</code> | ac_2 | history_cursor encodes the block position and history_params accepts it. |
| <code>capability.9.36.1</code> | title | Drop the anchor head from the Python px client and decode block position cursors |
| <code>capability.9.36.1</code> | reqs | 9381.1<br>9381.2 |
