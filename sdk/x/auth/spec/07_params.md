<!--
order: 7
-->

# Parameters

The auth params subspace persists the fields defined by
[`types/params.go`](../types/params.go) and
[`auth.proto`](../../../proto/cosmos/auth/v1beta1/auth.proto). The table gives code
defaults; query the running chain for its current values using
[the auth client](07_client.md#params).

| Go field | Protobuf/JSON field | Stored key bytes (literal text) | Type | Default |
| --- | --- | --- | --- | --- |
| `MaxMemoCharacters` | `max_memo_characters` | `MaxMemoCharacters` | `uint64` | `256` |
| `TxSigLimit` | `tx_sig_limit` | `TxSigLimit` | `uint64` | `7` |
| `TxSizeCostPerByte` | `tx_size_cost_per_byte` | `TxSizeCostPerByte` | `uint64` | `10` |
| `SigVerifyCostED25519` | `sig_verify_cost_ed25519` | `SigVerifyCostED25519` | `uint64` | `590` |
| `SigVerifyCostSecp256k1` | `sig_verify_cost_secp256k1` | `SigVerifyCostSecp256k1` | `uint64` | `1000` |
| `DisableSeqnoCheck` | `disable_seqno_check` | `KeyDisableSeqnoCheck` | `bool` | `false` |

`KeyDisableSeqnoCheck` is both the Go key variable's name and the exact stored
string. The stored string includes the `Key` prefix, unlike the other keys. It
is stored in the params store under the auth subspace as
`auth/KeyDisableSeqnoCheck`; it is not an account-store key.

Memo validation uses the byte length of the memo string despite the field's
`Characters` name. Signature counting includes the underlying keys of multisig
public keys. `TxSizeCostPerByte` charges gas for encoded transaction bytes; the
signature-cost fields feed the signature gas consumer. These defaults are not a
promise that every key type is accepted by every signing path.

## Sequence Checking

With the default `DisableSeqnoCheck = false`, an explicit mismatch between the
transaction signature's sequence and the account's stored sequence produces
`sdkerrors.ErrWrongSequence`.

Setting the parameter to `true` suppresses that explicit mismatch rejection in:

- [`SigVerificationDecorator.AnteHandle`](../ante/sigverify.go) in the SDK
  decorator chain.
- [`CheckSignatures`](../../../../node/ante/cosmos_checktx.go), shared by the
  node's specialized SDK `CheckTx` and `DeliverTx` paths.

This setting does not remove cryptographic signature verification, account-number
or SDK-chain-ID binding, or signer sequence increments. These paths still build
signer data with the stored account sequence; the effect on a mismatched signed
transaction depends on its signing mode. It also does not disable EVM nonce
checks. Operators must evaluate the signing and replay implications of enabling
it rather than treating it as a client retry mechanism. Clients should continue
to query and manage sequences with the default protection enabled.

Global minimum gas prices and allowed fee denominations are fields of
`FeesParams` in `x/params`, not auth parameters. See
[Concepts](01_concepts.md#global-and-local-minimum-prices) and
[AnteHandlers](03_antehandlers.md) for fee admission and charging behavior.
