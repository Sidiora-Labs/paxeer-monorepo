# Pay contract: layerxpay 0x101B

The payment construct at `0x000000000000000000000000000000000000101B` serves escrow, budget, stream and service operations through the shared kernel host. [The dispatcher](../../../precompiles/layerxpay/layerxpay.go) registers `NewMergePrecompile`; [node/upgrades.go](../../../node/upgrades.go) imports it and gates the address on `layerxpay` under `v7.0`.

Paxeer is one BFT network, with EVM chain ID `125` (`0x7d`). The kernel executes inside the node, and receipts are final when their blocks commit. Calls use signed EVM transactions or `eth_call` at the configured node root.

## layerxpay-0x101b

The [ABI](../../../precompiles/layerxpay/abi.json) defines the methods below. Writes are nonpayable, refuse delegatecall and cannot run under staticcall. Argument encoding is checked before execution. `budgetPull` is retained as a refusal selector and does not execute a payment.

| Method | Mutability | Returns | Selector |
|---|---|---|---|
| `askStreamOf(bytes32)` | view | `bytes32` | `0x929c1012` |
| `budgetAmend(bytes32,uint128,uint128,uint64,uint8)` | nonpayable | `none` | `0x90ce21c0` |
| `budgetClose(bytes32,uint64)` | nonpayable | `none` | `0xa139eb39` |
| `budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)` | nonpayable | `none` | `0xa863162b` |
| `budgetCreateNative(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8,bytes32,uint64)` | nonpayable | `none` | `0x9cca862b` |
| `budgetDefund(bytes32,uint128)` | nonpayable | `none` | `0x3d054fb3` |
| `budgetDelegateAdd(bytes32,bytes32)` | nonpayable | `none` | `0xd7a025b7` |
| `budgetDelegateRemove(bytes32,bytes32)` | nonpayable | `none` | `0xe90bac70` |
| `budgetDelegateSpend(bytes32,bytes32,uint128)` | nonpayable | `none` | `0xf9708fc1` |
| `budgetFund(bytes32,uint128)` | nonpayable | `none` | `0x5051eb50` |
| `budgetFundNative(bytes32,uint128,uint64)` | nonpayable | `none` | `0x10b28d90` |
| `budgetPull(bytes32,bytes32,uint128,bytes)` | nonpayable | `none` | `0xbc2548e7` |
| `budgetRemaining(bytes32)` | view | `uint128` | `0xd7778749` |
| `budgetRevoke(bytes32,uint64)` | nonpayable | `none` | `0x62da81cd` |
| `budgetSpend(bytes32,bytes32,uint128)` | nonpayable | `none` | `0x8608d32b` |
| `escrowCapture(bytes32,uint128,bytes32)` | nonpayable | `none` | `0xb291ec43` |
| `escrowDisputeOpen(bytes32)` | nonpayable | `none` | `0xeafee6d1` |
| `escrowDisputeResolve(bytes32,uint32,bytes32)` | nonpayable | `none` | `0x3d00261e` |
| `escrowOpen(bytes32,bytes32,bytes32,bytes32,bytes32,bytes32,uint128,uint64,uint64,bytes32,bytes32)` | nonpayable | `none` | `0xa179121e` |
| `escrowPartialCapture(bytes32,uint128,bytes32)` | nonpayable | `none` | `0xb7a902ab` |
| `escrowRelease(bytes32,bytes32)` | nonpayable | `none` | `0x50e0d150` |
| `escrowRemaining(bytes32)` | view | `uint128` | `0x9c328bc9` |
| `escrowTimeout(bytes32,bytes32)` | nonpayable | `none` | `0xa49e626f` |
| `getAgreement(bytes32)` | view | `(bytes32,bytes32,bytes32,bytes32,bytes32,bytes32,uint64,uint64,uint64,uint8,uint8,uint64,uint16,bytes32[],bool,uint64,uint64)` | `0xf42eb765` |
| `getBudget(bytes32)` | view | `(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8,bool,bool,bytes32[],bytes32)` | `0x546887a3` |
| `getCommitment(bytes32)` | view | `(bytes32,bytes32,bytes32,bytes32,bytes32,uint64,uint64,bytes32,uint64,bool,uint16)` | `0x7795820c` |
| `getDelivery(bytes32)` | view | `(bytes32,bytes32,(bytes32,uint64,bytes32)[],uint64)` | `0xedab743e` |
| `getDispute(bytes32)` | view | `(bytes32,bytes32,bytes32,bytes32,bytes32[],uint64,bool,uint16,uint32,bytes32,uint64)` | `0x136ba6aa` |
| `getEscrow(bytes32)` | view | `(bytes32,bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint8,uint64,uint64,bytes32,bytes32)` | `0xf023b811` |
| `getEscrowResult(bytes32)` | view | `bytes` | `0x068833a7` |
| `getExecution(bytes32)` | view | `bytes` | `0x0fe11ab3` |
| `getOffer(bytes32)` | view | `(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32,uint64,uint64,uint64,uint8,uint64,uint64,bool,bool)` | `0xd597b8c8` |
| `getProgress(bytes32)` | view | `(bytes32,bytes32,bytes32,bytes32,bytes32,bytes32,bytes32,uint32,uint64,uint64)` | `0x0f89ff41` |
| `getStream(bytes32)` | view | `(bytes32,bytes32,bytes32,bytes32,uint8,uint128,uint64,uint64,uint64,uint64,uint128,uint128,uint128,uint128,uint64,bytes32[],bool,bool,bool)` | `0x1290abe8` |
| `getStreamResult(bytes32)` | view | `(bytes32,bytes32,uint128,uint128,uint16,uint8)` | `0xb3d268df` |
| `latestDelivery(bytes32)` | view | `(bytes32,bytes32,(bytes32,uint64,bytes32)[],uint64)` | `0x1e0ac80d` |
| `progressHighWater(bytes32)` | view | `uint32` | `0x2c4e8866` |
| `serviceAccept(bytes32)` | nonpayable | `none` | `0x9f8dfee6` |
| `serviceAgreementAccept(bytes32)` | nonpayable | `none` | `0x85997911` |
| `serviceAgreementPropose(bytes32,bytes32,bytes32,bytes32)` | nonpayable | `none` | `0xe5589018` |
| `serviceCommitAbandon(bytes32,uint16)` | nonpayable | `none` | `0x3415adc7` |
| `serviceCommitTask(bytes32,bytes32,bytes32,bytes32,uint64,uint64)` | nonpayable | `none` | `0xdbc88a65` |
| `serviceDeliver(bytes32,bytes32,(bytes32,uint64,bytes32)[])` | nonpayable | `none` | `0xbda35baa` |
| `serviceDisputeOpen(bytes32,bytes32,bytes32[])` | nonpayable | `none` | `0x1274face` |
| `serviceDisputeResolve(bytes32,uint16,uint32,bytes32)` | nonpayable | `none` | `0x696b2d17` |
| `serviceOfferPublish(bytes32,bytes32,uint128,bytes32,bytes32,uint64,uint64,uint64,uint8,uint64)` | nonpayable | `none` | `0x34569433` |
| `serviceOfferWithdraw(bytes32)` | nonpayable | `none` | `0xd5584d7c` |
| `serviceProgressReport(bytes32,bytes32,bytes32,bytes32,uint32)` | nonpayable | `none` | `0xa203571a` |
| `serviceReject(bytes32,uint16,bytes32[])` | nonpayable | `none` | `0x4d71a096` |
| `serviceToolExecAttest(bytes)` | nonpayable | `none` | `0x5169415d` |
| `streamAccrued(bytes32)` | view | `uint128,uint128` | `0xb750cb8e` |
| `streamClose(bytes32,bytes32)` | nonpayable | `none` | `0x78725d86` |
| `streamMeter(bytes32,uint64,bytes32,bytes)` | nonpayable | `none` | `0xb96ba914` |
| `streamOpenForAsk(bytes32,address,uint256,uint8,uint256,bytes32)` | nonpayable | `bytes32` | `0x382e3492` |
| `streamOpen(bytes32,bytes32,bytes32,bytes32,uint8,uint128,uint64,uint64,uint64,uint128,uint128,bytes32[])` | nonpayable | `none` | `0x89fdbd3b` |
| `streamPause(bytes32)` | nonpayable | `none` | `0x71ed5358` |
| `streamResume(bytes32)` | nonpayable | `none` | `0x59aa40a2` |
| `streamSettle(bytes32,bytes32)` | nonpayable | `none` | `0x00a56e5f` |
| `streamTopUp(bytes32,uint128)` | nonpayable | `none` | `0x0b69f222` |

## escrow

Escrow operations open a hold, capture all or part of it, release it, apply a timeout, or resolve a dispute. [The keeper](../../../modules/layerxpay/keeper/escrow_open.go) submits these operations to kernel module 2. Escrow reads return a record or amount, a found flag and an error; malformed stored holds and result links return `ErrEscrowCorruptState`.

## budget

[Budget operations](../../../modules/layerxpay/keeper/budget_create.go) create, fund, amend, spend, delegate, close, defund and revoke budgets through kernel module 3. `budgetSpend` at 0x101B is the customer authorization path used by the ramp toolkit. The `budgetPull` selector is retired.

`BudgetRemaining` projects allowance at the query block time in milliseconds, caps it by the actual budget account balance, and returns zero for expired, revoked or closed budgets. The projection is read-only.

## stream

[Stream operations](../../../modules/layerxpay/keeper/stream_open.go) use kernel module 4. They open, top up, meter, settle, pause, resume and close streams. Agent streams at 0x0906 share these keeper records. Agent-stream time conversion uses milliseconds internally.

## service

[Service operations](../../../modules/layerxpay/keeper/service_offer.go) use kernel module 5 for offers, agreements, commitments, tool execution attestations, progress, delivery, acceptance, rejection and disputes.

## views

Views read the corresponding kernel records through the host. They do not change state or require the module to be unpaused. Read errors propagate; absence and malformed state are distinct outcomes.

## events

The ABI declares these events. Event declarations describe log shapes; successful emission is determined by the dispatcher and receipt decoding.

| Event | Indexed fields | Topic |
|---|---|---|
| `AgreementAccepted(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0x9de5935a09572b051c9194e7e83c1e5e340c717e25a06c66268b8892121474d4` |
| `AgreementProposed(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0x3e9d8e46bd4c26cf3e14abb64acb712a44dfdc616aaf61b4b2e1bac16aedee39` |
| `BudgetAmended(bytes32,uint128,uint64)` | `budgetId` | `0x31c93d31b53c28bc0fa050fb1c3062f782ba9a79bddc2e03ce2fd9b1fe6beff1` |
| `BudgetClosed(bytes32,uint128,uint64)` | `budgetId` | `0xa75bf542dcea5f85d998297a562512ea52af43a7dbf6b2e52283ac9e7269a064` |
| `BudgetCreated(bytes32,bytes32,uint128)` | `budgetId` | `0x4ffe02fb44ac10fea9c67d108d6137f20d66416c427018cd7c5c85134367a2b3` |
| `BudgetDefunded(bytes32,uint128,uint128)` | `budgetId` | `0xddf7b24a523c211d8e64a31f112821e871669f996eac19ae03a6ca742cf33dd3` |
| `BudgetDelegateAdded(bytes32,bytes32)` | `budgetId` | `0xfb88d261a6feb94af7f84c47e1ee79be8f7a17b4592b3cfaacfd30e7fcc8de3b` |
| `BudgetDelegateRemoved(bytes32,bytes32)` | `budgetId` | `0x5cde5fb6325cdde9badecb2d773ddcbd29bbd1ab44f7fa976e9b824ddccf2dbc` |
| `BudgetFunded(bytes32,uint128)` | `budgetId` | `0xb5395e31b1001c1a3db7533e5433ded6d549d5831c06882d39e8747947506d3f` |
| `BudgetRevoked(bytes32,uint128,uint64)` | `budgetId` | `0x6940118bfee95d3acf3b067b81e5d0eed108c543c9b0e0a462b502a81de64586` |
| `BudgetSpent(bytes32,bytes32,uint128)` | `budgetId` | `0xe7061a9872a2a196198f5fc257e098280fc29533217100b36a8a45113481624a` |
| `CommitAbandoned(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0xf82b0eee8867ff42a37f93f9ff76ac9f49c6d9d87212401d309bfe38d8784080` |
| `DefaultOutcomeApplied(bytes32,uint8,bool,uint64,uint64)` | `agreementId` | `0x52cdc71e077aa63bc67e6a932084f167c9160b7b857a3e3c1d26d3ec4923da98` |
| `Delivered(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0x8e7c33a038e5e59bac9f44a0eb4bb592c7d0cc1c7271360f73f232a8c9511552` |
| `DisputeOpened(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0xbf7c0c01d9d1cca84d5e6e3c398fd19d7b9bf4aa1eb2344706c0cd9605734d07` |
| `DisputeResolved(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0xa4df9ab184633d8a0e83b36ceccf4a447aab7b5d6c0687d9cbec51e2eb54192a` |
| `EscrowCaptured(bytes32,uint16,uint8,uint128,uint128)` | `escrowId` | `0xad68d39439fc7b036c914e06aff1e364d909efc098ae0a50a48f10dfb4246e34` |
| `EscrowDisputed(bytes32,uint16,uint8,uint128,uint128)` | `escrowId` | `0x01030c369649c257abfaf58265fd8e25cfd284243d7e111bf34ae8df760ef1a6` |
| `EscrowOpened(bytes32,uint16,uint8,uint128,uint128)` | `escrowId` | `0xeb55494fa55a2ce6e323b2b8776e5e352d2964877c0f5bc592679325749bfc2f` |
| `EscrowPartiallyCaptured(bytes32,uint16,uint8,uint128,uint128)` | `escrowId` | `0xc84c61f008e334a19c74068689bf3a54f2ae0ecfec049d62b819f5f68d7a1a9d` |
| `EscrowReleased(bytes32,uint16,uint8,uint128,uint128)` | `escrowId` | `0x65407a7b21a4dbbe609b634d05221c6beab59a52ac8e17b8be2bb7fa6e399654` |
| `EscrowResolved(bytes32,uint16,uint8,uint128,uint128)` | `escrowId` | `0xe3a9e891d675b5ae5ea35ed61028e3f15be7ab2dad8b4bb7ad51ac5af1737c73` |
| `EscrowTimedOut(bytes32,uint16,uint8,uint128,uint128)` | `escrowId` | `0x3cb7cfeaeaccd65cfb44ed6f0f50c4ecd0d08b5b9b7c507c8554672ea0e2df98` |
| `ExecutionAttested(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0x3fd39a11ff049c70f0c6c1e3e311f2bd0d00607756c7fae5156587bf82a19467` |
| `OfferPublished(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0x094bfb6e853a36faae49604b89e04fef60a8ce2f9e4c1f9d0c53c6166d7f6b43` |
| `OfferWithdrawn(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0x1dc29fd3e5c1c7ecf9bbd0e84ede6461886d0dafe96843e82dd32efd004c31b4` |
| `OutcomeAccepted(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0x5d654e0e0378bd06e4e6fcd1728c8954e62e820aa2147d3a6a90a133bffaf1de` |
| `OutcomeRejected(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0x6b6c89470b91617fc6fa2384b31e88021ddf3bcb78491fc0e82748dc84b13518` |
| `ProgressReported(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0x819ea4c59366246bb925fe1c7273f9af36a52b0ff8d393e63e789762e328c0fa` |
| `AskStreamOpened(bytes32,bytes32,address)` | `askRef`, `streamId` | `0x27aefe1fca15d7f0c14d95e40221cc603fbd590174202d2c2f670480d7023a91` |
| `StreamClosed(bytes32,uint128,uint128)` | `streamId` | `0x8063cfde1c85c4898f9fff27df374f273806733152313f92e8a5914423678a9f` |
| `StreamMetered(bytes32,uint64,uint128,uint128)` | `streamId` | `0x932366bfeca08e9b804274a1a7eb67bd4d66f03e9290b961e60b8eeaf5d78b02` |
| `StreamOpened(bytes32,bytes32,bytes32,bytes32,bytes32,uint8,uint128)` | `streamId` | `0x8277b76c675fc9dab63f492a7ac5479a66d6552828f1fdbc9a76e2340ada9448` |
| `StreamPaused(bytes32,uint64,uint128)` | `streamId` | `0x9032eddf8dcc6800a443fad0f2630b6edbeb66a914d9cb09347325363cf7fe1e` |
| `StreamRateUpdated(bytes32,uint128,uint128,uint128)` | `streamId` | `0x6006a657a93ec26224beb6993b55b31a96ca5b9e8719252665286ab2b7f58e60` |
| `StreamResumed(bytes32,uint64,uint128)` | `streamId` | `0x90f70f76c4cceddd91668722f7b46eb8b8df7612a6f9c6087fc49b97b9758a14` |
| `StreamSettled(bytes32,uint128,uint128,bool)` | `streamId` | `0xa87dc98d52838f3eb3a565c28fa8ccfb54ffcc925bc81b759eb3e64fb5185f69` |
| `StreamToppedUp(bytes32,uint128,bool)` | `streamId` | `0x643cbe429fc24e5e4f0f1ddb6f5d583d651e5558832d390e9b65557734ce47c9` |
| `TaskCommitted(bytes32,bytes32,uint8,uint64)` | `primary`, `secondary` | `0x6e00443b450170a849061291f9a4797138ee198a5cc8de5f4c30f1aa79e501f1` |

## errors

The precompile uses Solidity `Error(string)` with canonical `LXP_` reasons. [Module errors](../../../modules/layerxpay/types/errors.go) and kernel errors retain their typed causes. Clients accept a canonical code with or without a diagnostic suffix.

## gas

The served precompile charges [methodGas](../../../precompiles/layerxpay/gas.go) over arguments after the four-byte selector, then meters the call's store work. The method row is a base charge, rather than a constant total independent of state access. An unknown selector is refused and charged `UnknownMethodCallGas`.

## authority

[AuthorityFor](../../../modules/layerxpay/keeper/authority.go) resolves the EVM caller's DID binding. `CallerAuthority` carries `ActorDID`, `AuthorityKey` and `Kernel`. `Submit` derives the bound main-account principal and enters the host; call arguments cannot supply a different principal.

## execution

[Submit](../../../modules/layerxpay/keeper/execute.go) checks the method's family, pause and enable flags, resolves the bound caller, and constructs a protocol-v3 kernel activity using block time. Execution occurs in a cached context; refusal discards the branch. The activity's method mapping is [MethodActivity](../../../modules/layerxpay/types/activity.go).

## pause

`CheckWrite(ctx, method)` derives the kernel family from the method table. It checks the family parameter, governance pause/exit policy and shared-host enable record. Read methods do not pass this write gate. The permitted exits are defined by [CheckPauseScope](../../../modules/layerxgov/types/proposal.go).

## genesis

The [module genesis type](../../../modules/layerxpay/types/params.go) contains `Params`, `KernelRecords`, `EscrowReceipts` and `AgentStreamMetadata`. Default params are paused with all four family flags false. The [keeper genesis implementation](../../../modules/layerxpay/keeper/genesis.go) validates the full envelope before writing, restores kernel records to the host store, restores metadata to the pay store and rebuilds service indexes.

`ExportRecoveryGenesis` and `InitRecoveryGenesis` use a versioned envelope with bounded chunks, record counts, ordered keys and a checksum. Validation includes record links across chunks. Pay state recovery includes more than module params.

## queries

[The module](../../../modules/layerxpay/module.go) registers its Msg and Query services. [The query handler](../../../modules/layerxpay/keeper/grpc_query.go) serves params without writes; a nil request is `InvalidArgument`. Kernel payment records are read through the EVM views above.

## Parameters and messages

[tx.proto](../../../api/layerxpay/tx.proto) declares `Params` with `paused`, `escrow_enabled`, `budget_enabled`, `stream_enabled` and `service_enabled`. `Msg/UpdateParams` accepts `MsgUpdateParams { authority, params }` and returns an empty `MsgUpdateParamsResponse`. [The message server](../../../modules/layerxpay/keeper/msg_server.go) requires the configured governance authority.

<a id="pay-contract-layerxpay-0x101b-and-the-layerxpay-module"></a>

<a id="contents"></a>

<a id="idempotency"></a>

<a id="pay-genesis"></a>

<a id="msg-layerxpay-updateparams"></a>

<a id="proto-layerxpay-params"></a>

<a id="proto-layerxpay-msgupdateparams"></a>

<a id="proto-layerxpay-msgupdateparamsresponse"></a>

<a id="decisions"></a>

## pay-keeper

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `func (k Keeper) Submit(ctx sdk.Context, caller common.Address, method string, payload []byte, idempotencyKey [32]byte) (lxphost.Receipt, error)` | [modules/layerxpay/keeper/execute.go](../../../modules/layerxpay/keeper/execute.go) |
| `go_exported` | `type Keeper struct {` | [modules/layerxpay/keeper/keeper.go](../../../modules/layerxpay/keeper/keeper.go) |
| `go_exported` | `func NewKeeper(cdc codec.BinaryCodec, storeKey, hostKey sdk.StoreKey, host *lxphost.Host, evm EVMBinding, custody CustodyReader, authority string) (Keeper, error)` | [modules/layerxpay/keeper/keeper.go](../../../modules/layerxpay/keeper/keeper.go) |

## pay-types

| Kind | Declaration | Source |
|---|---|---|
| `go_exported` | `ModuleName                 = "layerxpay"` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `StoreKey                   = ModuleName` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `RouterKey                  = ModuleName` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `QuerierRoute               = ModuleName` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `EscrowModuleID      uint16 = 2` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `BudgetModuleID      uint16 = 3` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `StreamModuleID      uint16 = 4` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `ServiceModuleID     uint16 = 5` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `GovernanceModuleID  uint16 = 7` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `MaxKernelRecords           = 512` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `MaxKernelKeyBytes          = 128` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `MaxKernelValueBytes        = 1024` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `func KernelKVKey(moduleID uint16, key []byte) []byte` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `func KernelModuleEnableKey(moduleID uint16) ([]byte, error)` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `go_exported` | `var ErrInvalidGenesis = sdkerrors.Register(ModuleName, 3, "invalid genesis")` | [modules/layerxpay/types/params.go](../../../modules/layerxpay/types/params.go) |
| `go_exported` | `func DefaultParams() Params` | [modules/layerxpay/types/params.go](../../../modules/layerxpay/types/params.go) |
| `go_exported` | `type GenesisState struct` | [modules/layerxpay/types/params.go](../../../modules/layerxpay/types/params.go) |
| `go_exported` | `func DefaultGenesis() *GenesisState` | [modules/layerxpay/types/params.go](../../../modules/layerxpay/types/params.go) |
| `go_exported` | `func (g GenesisState) Validate() error` | [modules/layerxpay/types/params.go](../../../modules/layerxpay/types/params.go) |
| `go_exported` | `BudgetCreatePayloadBytes        = 211` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetCreateNativePayloadBytes  = 251` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetFundPayloadBytes          = 50` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetFundNativePayloadBytes    = 58` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetAmendPayloadBytes         = 75` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetDelegatePayloadBytes      = 66` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetSpendPayloadBytes         = 82` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetClosePayloadBytes         = 42` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetDefundPayloadBytes        = 50` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetRevokePayloadBytes        = 42` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetDelegateSpendPayloadBytes = 114` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `PayerGrantBytes                 = 346` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `BudgetPullPayloadBytes          = BudgetDelegateSpendPayloadBytes + PayerGrantBytes` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetCreate(budgetID, budgetAccount, assetID, purposeHash [32]byte, perPeriodLimit, carryCap, amount codec.U128, periodLength, periodStart, expiry, revocationSequence uint64, rolloverPolicy uint8) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetCreateNative(budgetID, budgetAccount, assetID, purposeHash [32]byte, perPeriodLimit, carryCap, amount codec.U128, periodLength, periodStart, expiry, revocationSequence uint64, rolloverPolicy uint8, sourceAccount [32]byte, sourceSequence uint64) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetFund(budgetID [32]byte, amount codec.U128) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetFundNative(budgetID [32]byte, amount codec.U128, sourceSequence uint64) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetAmend(budgetID [32]byte, perPeriodLimit, carryCap codec.U128, expiry uint64, rolloverPolicy uint8) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetDelegate(budgetID, delegate [32]byte) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetSpend(budgetID, recipient [32]byte, amount codec.U128) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetClose(budgetID [32]byte, revocationSequence uint64) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetDefund(budgetID [32]byte, amount codec.U128) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetRevoke(budgetID [32]byte, revocationSequence uint64) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetDelegateSpend(budgetID, recipient [32]byte, amount codec.U128, idempotencyKey [32]byte) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `func EncodeBudgetPull(budgetID, recipient [32]byte, amount codec.U128, idempotencyKey [32]byte, payerGrant []byte) ([]byte, error)` | [modules/layerxpay/types/payload_budget.go](../../../modules/layerxpay/types/payload_budget.go) |
| `go_exported` | `EscrowOpenPayloadBytes           = 288` | [modules/layerxpay/types/payload_escrow.go](../../../modules/layerxpay/types/payload_escrow.go) |
| `go_exported` | `EscrowCapturePayloadBytes        = 80` | [modules/layerxpay/types/payload_escrow.go](../../../modules/layerxpay/types/payload_escrow.go) |
| `go_exported` | `EscrowReleasePayloadBytes        = 64` | [modules/layerxpay/types/payload_escrow.go](../../../modules/layerxpay/types/payload_escrow.go) |
| `go_exported` | `EscrowDisputeOpenPayloadBytes    = 32` | [modules/layerxpay/types/payload_escrow.go](../../../modules/layerxpay/types/payload_escrow.go) |
| `go_exported` | `EscrowDisputeResolvePayloadBytes = 68` | [modules/layerxpay/types/payload_escrow.go](../../../modules/layerxpay/types/payload_escrow.go) |
| `go_exported` | `func EncodeEscrowOpen(escrowID, owner, escrowAccount, beneficiary, arbiter, assetID [32]byte, amount codec.U128, expiryMs, disputeWindowMs uint64, termsHash, agreementReference [32]byte) []byte` | [modules/layerxpay/types/payload_escrow.go](../../../modules/layerxpay/types/payload_escrow.go) |
| `go_exported` | `func EncodeEscrowCapture(escrowID [32]byte, amount codec.U128, idempotencyKey [32]byte) []byte` | [modules/layerxpay/types/payload_escrow.go](../../../modules/layerxpay/types/payload_escrow.go) |
| `go_exported` | `func EncodeEscrowRelease(escrowID, idempotencyKey [32]byte) []byte` | [modules/layerxpay/types/payload_escrow.go](../../../modules/layerxpay/types/payload_escrow.go) |
| `go_exported` | `func EncodeEscrowDisputeOpen(escrowID [32]byte) []byte` | [modules/layerxpay/types/payload_escrow.go](../../../modules/layerxpay/types/payload_escrow.go) |
| `go_exported` | `func EncodeEscrowDisputeResolve(escrowID [32]byte, beneficiaryBps uint32, idempotencyKey [32]byte) []byte` | [modules/layerxpay/types/payload_escrow.go](../../../modules/layerxpay/types/payload_escrow.go) |
| `go_exported` | `ServiceMaxDeliverables = 16` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `ServiceExecutionBytes  = 416` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `type ServiceDeliverable struct` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServiceOffer(offerID, assetID [32]byte, price codec.U128, termsHash, deliverableSpecHash [32]byte, deliveryDeadline, acceptanceWindow, disputeWindow uint64, defaultOutcome uint8, offerExpiry uint64) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServiceIdentifier(identifier [32]byte) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServicePropose(agreementID, offerID, termsHash, escrowID [32]byte) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServiceCommit(commitmentID, agreementID, taskHash, escrowID [32]byte, deadline, resourceBound uint64) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServiceAbandon(commitmentID [32]byte, reason uint16) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServiceProgress(reportID, commitmentID, noteHash, availabilityReference [32]byte, progressBPS uint32) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServiceDeliver(deliveryID, agreementID [32]byte, deliverables []ServiceDeliverable) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServiceReject(agreementID [32]byte, reason uint16, contestedHashes [][32]byte) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServiceDisputeOpen(disputeID, agreementID [32]byte, evidenceHashes [][32]byte) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServiceDisputeResolve(disputeID [32]byte, ruling uint16, providerBPS uint32, escrowResolutionID [32]byte) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `func EncodeServiceToolExecAttest(execution []byte) ([]byte, error)` | [modules/layerxpay/types/payload_service.go](../../../modules/layerxpay/types/payload_service.go) |
| `go_exported` | `StreamMaxMeterAuthorities          = 8` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `StreamOpenPayloadFixedBytes        = 204` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `StreamOpenPayloadMaxBytes          = StreamOpenPayloadFixedBytes + StreamMaxMeterAuthorities*32` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `StreamTopUpPayloadBytes            = 50` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `StreamMeterPayloadBytes            = 138` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `StreamKeyedPayloadBytes            = 66` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `StreamRateChangePayloadBytes       = 82` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `StreamIDPayloadBytes               = 34` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `StreamModeTime               uint8 = 1` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `StreamModeMetered            uint8 = 2` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `type StreamOpen struct` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `func EncodeStreamOpen(open StreamOpen) ([]byte, error)` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `func EncodeStreamTopUp(streamID [32]byte, amount codec.U128) ([]byte, error)` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `func EncodeStreamMeter(streamID [32]byte, cumulativeReading uint64, authorityKey [32]byte, signature [64]byte) ([]byte, error)` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `func EncodeStreamKeyed(streamID, idempotencyKey [32]byte) ([]byte, error)` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `func EncodeStreamID(streamID [32]byte) ([]byte, error)` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `go_exported` | `func EncodeStreamRateChange(streamID [32]byte, newRate codec.U128, idempotencyKey [32]byte) ([]byte, error)` | [modules/layerxpay/types/payload_stream.go](../../../modules/layerxpay/types/payload_stream.go) |
| `storage_key` | `ParamsKey          = []byte{0x05}` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `storage_key` | `KernelKVPrefix     = []byte{0x01}` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `storage_key` | `EscrowKVPrefix     = []byte{0x01, 0x00, 0x02}` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `storage_key` | `BudgetKVPrefix     = []byte{0x01, 0x00, 0x03}` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `storage_key` | `StreamKVPrefix     = []byte{0x01, 0x00, 0x04}` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `storage_key` | `ServiceKVPrefix    = []byte{0x01, 0x00, 0x05}` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
| `storage_key` | `GovernanceKVPrefix = []byte{0x01, 0x00, 0x07}` | [modules/layerxpay/types/keys.go](../../../modules/layerxpay/types/keys.go) |
