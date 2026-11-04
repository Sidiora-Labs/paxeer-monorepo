# Governance proposal types

Paxeer has one merged network: chain governance runs in `paxd`, and the
kernel is part of that network and proposes blocks. A receipt is final when
its block is committed.

This reference covers Go proposal declarations and their execution handlers,
plus the kernel governance constants. A retained declaration does not by itself
mean that a proposal can execute on the merged network.

## Chain proposal types

The application registers proposal handlers in [node/app.go](../../node/app.go).
The tables catalogue `ProposalType*` constants in `modules/` and `sdk/x/`;
IBC and conditionally enabled CosmWasm proposal handlers are also registered
by the application. Active types still require valid content, governance
approval, and the target module's authority and lifecycle checks.

### Active proposal types

| Type | Constant | Declaration source |
|---|---|---|
| `AddERCNativePointerV2` | `ProposalTypeAddERCNativePointerV2` | [modules/evm/types/gov.go](../../modules/evm/types/gov.go) |
| `CancelSoftwareUpgrade` | `ProposalTypeCancelSoftwareUpgrade` | [sdk/x/upgrade/types/proposal.go](../../sdk/x/upgrade/types/proposal.go) |
| `CommunityPoolSpend` | `ProposalTypeCommunityPoolSpend` | [sdk/x/distribution/types/proposal.go](../../sdk/x/distribution/types/proposal.go) |
| `LayerXBridge` | `ProposalTypeBridge` | [modules/layerxbridge/types/proposal.go](../../modules/layerxbridge/types/proposal.go) |
| `LayerX` | `ProposalTypeLayerX` | [modules/layerxgov/types/proposal.go](../../modules/layerxgov/types/proposal.go) |
| `ParameterChange` | `ProposalTypeChange` | [sdk/x/params/types/proposal/proposal.go](../../sdk/x/params/types/proposal/proposal.go) |
| `PointerBinding` | `ProposalTypePointerBinding` | [modules/evm/types/gov.go](../../modules/evm/types/gov.go) |
| `SoftwareUpgrade` | `ProposalTypeSoftwareUpgrade` | [sdk/x/upgrade/types/proposal.go](../../sdk/x/upgrade/types/proposal.go) |
| `Text` | `ProposalTypeText` | [sdk/x/gov/types/proposal.go](../../sdk/x/gov/types/proposal.go) |
| `UpdateMinter` | `ProposalTypeUpdateMinter` | [modules/mint/types/gov.go](../../modules/mint/types/gov.go) |

`Text` is a signaling proposal: its handler makes no state change.
The [upgrade handler](../../sdk/x/upgrade/handler.go) schedules the supplied
plan for `SoftwareUpgrade` and clears the pending plan for
`CancelSoftwareUpgrade`.

`LayerXBridge` executes the bridge module's authority-checked messages through
the [bridge handler](../../modules/layerxbridge/handler.go). `LayerX` validates
its carried messages and dispatches them to registered message services through
the [LayerX governance handler](../../modules/layerxgov/handler.go). Each
handler applies its messages atomically. The `LayerX` proposal type remains
active, but it cannot bypass a target module's retirement checks.

### Historical declarations

These constants remain in the source and proposal codecs for compatibility.
They do not describe executable operations on the merged network.

| Type | Constant | Declaration source | Execution status |
|---|---|---|---|
| `AddCWERC1155Pointer` | `ProposalTypeAddCWERC1155Pointer` | [modules/evm/types/gov.go](../../modules/evm/types/gov.go) | Deprecated; always refused |
| `AddCWERC20Pointer` | `ProposalTypeAddCWERC20Pointer` | [modules/evm/types/gov.go](../../modules/evm/types/gov.go) | Deprecated; always refused |
| `AddCWERC721Pointer` | `ProposalTypeAddCWERC721Pointer` | [modules/evm/types/gov.go](../../modules/evm/types/gov.go) | Deprecated; always refused |
| `AddERCCW1155Pointer` | `ProposalTypeAddERCCW1155Pointer` | [modules/evm/types/gov.go](../../modules/evm/types/gov.go) | Deprecated; always refused |
| `AddERCCW20Pointer` | `ProposalTypeAddERCCW20Pointer` | [modules/evm/types/gov.go](../../modules/evm/types/gov.go) | Deprecated; always refused |
| `AddERCCW721Pointer` | `ProposalTypeAddERCCW721Pointer` | [modules/evm/types/gov.go](../../modules/evm/types/gov.go) | Deprecated; always refused |
| `AddERCNativePointer` | `ProposalTypeAddERCNativePointer` | [modules/evm/types/gov.go](../../modules/evm/types/gov.go) | Deprecated; always refused |
| `LayerXCustody` | `ProposalTypeCustody` | [modules/layerxcustody/types/proposal.go](../../modules/layerxcustody/types/proposal.go) | Retired at merge activation |

The seven deprecated pointer handlers in [modules/evm/gov.go](../../modules/evm/gov.go)
return `proposal type deprecated`. The active native-pointer proposal is
`AddERCNativePointerV2`; `PointerBinding` has a separate active handler in
[modules/evm/handler.go](../../modules/evm/handler.go).

The [custody handler](../../modules/layerxcustody/handler.go) remains registered,
but checks retirement before validating or executing content. Its
[retirement check](../../modules/layerxcustody/keeper/retire.go) observes the
recorded `v7.0` activation at the current height, or an imported retirement
marker. Once retired, it returns `ErrModuleRetired` in codespace
`layerxcustody`, code `29`, without executing proposal messages. Historical
execution before retirement also requires the recorded `v6.12.0` activation;
without that activation it returns `ErrGovernanceNotActive` (code `25`).
See the [custody error declarations](../../modules/layerxcustody/types/errors.go)
and [activation check](../../modules/layerxcustody/keeper/keeper.go).

### Governance precompile submission

The governance precompile at `0x0000000000000000000000000000000000001006`
accepts a smaller set through `submitProposal(string)`: `Text`,
`ParameterChange`, `SoftwareUpgrade`, `CancelSoftwareUpgrade`, and
`CommunityPoolSpend`. An empty JSON `type` selects `Text`. Other chain proposal
types require their native submission path; appearing in the tables above
does not make them accepted by this precompile.

Source: [precompiles/gov/handler.go](../../precompiles/gov/handler.go) and
[Gov.sol](../../precompiles/gov/Gov.sol). See also the
[precompile reference](precompiles.md) and [CLI reference](cli.md).

## Kernel governance (`include/layerx/lxp_governance.h`)

The [kernel governance header](../../include/layerx/lxp_governance.h) declares
the following activity identifiers, effect bits, and rollout scopes. These are
kernel constants within the same network, distinct from the Go proposal type
strings above. Effect bits describe attempted changes; they do not grant
permission to perform those changes. For example,
[`lxp_gov_module_enable`](../../src/modules/governance/lxp_emergency.c) refuses
any nonzero attempted effect mask with `LXP_ERR_AUTH_SCOPE`.

| Constant | Value |
|---|---|
| `LXP_GOVERNANCE_MIGRATION_BUDGET` | `0x0007000a` |
| `LXP_GOVERNANCE_PROGRAMS_FEE` | `0x0007000b` |
| `LXP_GOV_EFFECT_BALANCE_WRITE` | `1U << 0` |
| `LXP_GOV_EFFECT_BATCH_REWRITE` | `1U << 4` |
| `LXP_GOV_EFFECT_BURN` | `1U << 2` |
| `LXP_GOV_EFFECT_FINALIZED_HISTORY_REASSIGN` | `1U << 6` |
| `LXP_GOV_EFFECT_MINT` | `1U << 1` |
| `LXP_GOV_EFFECT_RECEIPT_REWRITE` | `1U << 3` |
| `LXP_GOV_EFFECT_STATE_ROOT_SUBSTITUTE` | `1U << 5` |
| `LXP_GOV_ROLLOUT_ACCOUNT_SET` | `4` |
| `LXP_GOV_ROLLOUT_ALL` | `1` |
| `LXP_GOV_ROLLOUT_MARKET` | `3` |
| `LXP_GOV_ROLLOUT_MODULE` | `2` |
