# Perps ABI change history

This history corrects the implementation claim in the subject of commit
`c1f5f4d31ef0f4885db9b4203821d350fd249cb1`, as recorded by
[issue #1558's triage report](../spec/layerx-merge/programme/issues/1558.kvx)
and decision 2438. Follow-up statuses and source line references below describe
revision `210639d0360c53f5c2fdd81c9c7c5e84b24b8c84`.

## Decision commit

c1f5f4d31ef0f4885db9b4203821d350fd249cb1 records decisions 2387-2393 and follow-up tasks; it does not implement the perps callers, boundary manifest-rule retirement or served-methods golden refresh.

Its three-file diff changes only `GOTCHA.kvx`,
`spec/layerx-merge/qualification.kvx` and `spec/layerx-merge/spec.kvx`.
The subject's description of SDK, wallet, web, boundary and golden changes
therefore overstates what this commit delivered.

## Separate SDK implementation and observed web signatures

Commit `c31ded3a33b5c890e15e40b05be99c5b763307e9` supplies the later SDK
implementation under task `22.9.37`. In
[`services/agent/sdk/typescript/src/perps.ts:740-753`](../services/agent/sdk/typescript/src/perps.ts#L740-L753),
`perpsOpenPositionCall` encodes
`openPosition(uint256,address,uint256,uint256,bool)` with market ID, collateral
token, collateral amount, leverage and direction. `perpsCancelOrderCall` encodes
`cancelOrder(uint256)` with the order ID.

That implementation commit also updates the ABI-based tests at
[`services/agent/sdk/typescript/test/perps-v2.test.ts:33-88`](../services/agent/sdk/typescript/test/perps-v2.test.ts#L33-L88).
They derive selectors from `precompiles/perps/abi/abi.json`, assert the encoded
argument words and reject invalid inputs. These code fixes and their tests
remain owned by their existing task.

At the cited revision, the web page's `perpsMarketPositionCall` and
`perpsOrderCancelCall` already declare the same open and cancel signatures at
[`apps/web/src/app/exchange/exchange-client.tsx:72-77`](../apps/web/src/app/exchange/exchange-client.tsx#L72-L77).
Those declarations alone do not establish completion of the pending web proof.

## Outstanding follow-up work

All four tasks below are explicitly **pending** at revision
`210639d0360c53f5c2fdd81c9c7c5e84b24b8c84`; their changes remain outstanding
in that snapshot.

| Existing task | Outstanding change | Pending status at the cited revision |
| --- | --- | --- |
| `22.24.26` | Wallet compatibility: update `sdk/wallet/src/modules/exchange.ts:54`, the wallet form and their tests to use the SDK's ABI shapes, including cancellation without an account ID. | `spec/layerx-merge/spec.kvx:119043` |
| `22.23.13` | Web proof: remove the substituted open/cancel declarations at `apps/web/e2e/exchange-perps.test.ts:50-55` and prove the real precompile ABI through the web flow. | `spec/layerx-merge/spec.kvx:119012` |
| `10.1.4` | Boundary manifest-rule retirement: remove the obsolete payload-authority manifest rule at `services/human/tools/boundary-check/src/lib.rs:255-259` and retain the boundary task's proof obligations. | `spec/layerx-merge/spec.kvx:175311` |
| `22.7.3` | Served-methods golden refresh: add the missing `px_getReceiptKeys` row between `rpc/testdata/served_methods_http.golden:100-101`. | `spec/layerx-merge/spec.kvx:191684` |

## Focused documentation check

The `commit_scope_correction` check asserts the complete standalone correction
sentence above and the link from [the documentation index](README.md).
Implementation changes and their tests remain under their existing tasks;
this documentation check supplies no evidence that pending work has completed.
