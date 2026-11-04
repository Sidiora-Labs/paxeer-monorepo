<!--
order: 3
-->

# AnteHandlers

An `AnteHandler` checks transaction authentication and other prerequisites before
message execution. SDK decorators compose through `sdk.ChainAnteDecorators`.
Checks run for mempool admission (`CheckTx`) and committed execution
(`DeliverTx`); individual decorators may treat simulation and rechecking
differently. Passing a node's admission checks does not guarantee execution will
succeed, and block validation must enforce its own checks.

## Decorators

The default SDK constructor,
[`sdk/x/auth/ante.NewAnteHandler`](../ante/ante.go), chains these decorators in
this order:

1. `SetUpContextDecorator`, created by `NewDefaultSetUpContextDecorator`: installs
   the gas meter and recovers downstream out-of-gas panics.
2. `RejectExtensionOptionsDecorator`: rejects unsupported extension options.
3. `ValidateBasicDecorator`: runs transaction basic validation.
4. `TxTimeoutHeightDecorator`: checks the transaction's timeout height.
5. `ValidateMemoDecorator`: checks memo length against auth parameters.
6. `ConsumeTxSizeGasDecorator`, created by `NewConsumeGasForTxSizeDecorator`:
   charges gas for transaction bytes.
7. `DeductFeeDecorator`: invokes the configured fee checker, charges effective
   fees to the payer or approved fee granter, and sets transaction priority.
8. `SetPubKeyDecorator`: sets signer public keys before signature processing.
9. `ValidateSigCountDecorator`: enforces the configured signature limit.
10. `SigGasConsumeDecorator`: charges signature verification gas.
11. `SigVerificationDecorator`: checks signer metadata and signatures using the
    configured sign-mode handler.
12. `IncrementSequenceDecorator`: increments and persists each signer's sequence.

### Fee checking and deduction

`DeductFeeDecorator` uses `HandlerOptions.TxFeeChecker` when supplied, otherwise
`CheckTxFeeWithValidatorMinGasPrices`. Minimum-price filtering and fee-based
priority calculation occur inside that checker. There is no separate
`MempoolFeeDecorator` in this constructed chain. See
[Gas & Fees](01_concepts.md#gas--fees) for global/local price merging, eligible
fee denominations and priority precision.

The transaction must implement `sdk.FeeTx`. Fees are charged to `FeePayer()`
unless `FeeGranter()` is set. A distinct granter must authorize the payer through
`UseGrantedFees`; setting a granter when no feegrant keeper is configured returns
`ErrInvalidRequest`. The paying account must exist. The bank keeper's
`DeferredSendCoinsFromAccountToModule` moves fees to `fee_collector`;
insufficient funds return `ErrInsufficientFunds`. Successful deduction emits
`tx` event attributes `fee` and `fee_payer` and passes the priority through the
context. See [`ante/fee.go`](../ante/fee.go).

### Signatures and sequences

[`SigVerificationDecorator`](../ante/sigverify.go) rejects an explicit sequence
mismatch with `ErrWrongSequence` unless `DisableSeqnoCheck` is enabled. Signature
verification still uses signer data containing the SDK chain ID, account number
and stored sequence. Simulation and `ReCheckTx` skip cryptographic verification
in this decorator. Sequence increments are a separate decorator and are not
disabled by the parameter. See [Parameters](07_params.md#sequence-checking) for
the exact scope of that setting.

## Handlers

The SDK default above is a reusable constructor. Paxeer installs its own
application constructor from [`node/ante.go`](../../../../node/ante.go) in
[`node/app.go`](../../../../node/app.go).

The application's SDK chain starts with context setup, then a `GaslessDecorator`
wrapping `DeductFeeDecorator`, followed by simulation-gas limiting, extension
checks, oracle checks, basic validation, timeout, memo and transaction-size gas.
It applies `PriorityDecorator` before public-key and signature processing, then
increments sequences and runs EVM-address association, nested-authz, revocation
and IBC checks. Its fee checker wrapper also handles the configured fee-token
feature. Consequently, fee deduction is earlier in this chain than in the SDK
default list.

The specialized SDK handlers
[`CosmosCheckTxAnte`](../../../../node/ante/cosmos_checktx.go) and
[`CosmosDeliverTxAnte`](../../../../node/ante/cosmos_delivertx.go), called by
[`node/legacyabci/check_tx.go`](../../../../node/legacyabci/check_tx.go) and
[`deliver_tx.go`](../../../../node/legacyabci/deliver_tx.go), perform their checks
directly. They check memo, transaction size, public keys and signatures and update
signers before fee charging. The admission path uses `CheckAndChargeFees` and
`DecoratePriority`; the committed path uses `ChargeFees`.

EVM transactions use a separate ante chain selected inside the node. That chain
checks EVM fields, chain/domain identity, preprocessing, basic validity, fees,
signatures and gas. The SDK decorator list must not be used as the EVM validation
order. Both transaction forms are part of the same network.
