<!--
order: 2
-->

# State

The transfer module stores its bound port and denomination traces in the
`transfer` KV store. [Keeper accessors](../keeper/keeper.go) and
[key definitions](../types/keys.go) define these encodings:

| Entry | Key bytes | Value encoding |
|-------|-----------|----------------|
| Port | `0x01` | Raw `[]byte(portID)`, normally the UTF-8 bytes of `transfer` |
| DenomTrace | `0x02` followed by the 32 raw SHA-256 hash bytes | Protobuf-encoded `DenomTrace` with `path` and `base_denom` |

`SetPort` writes raw string bytes and `GetPort` converts them directly to a
string; the port value has no protobuf string wrapper or length prefix.
For `transfer`, the value is exactly `74 72 61 6e 73 66 65 72` in hexadecimal.
`SetDenomTrace` uses the binary codec to marshal the trace and indexes it by
`DenomTrace.Hash()`, not by the ASCII hexadecimal hash or the `ibc/` prefix.
The trace codec boundary is [MarshalDenomTrace / UnmarshalDenomTrace](../keeper/encoding.go);
`TestRepairM_1206_PortStorage` checks these bytes through the real store and codec.
See [Denomination Trace](01_concepts.md#denomination-trace) and
[ADR 001](../../../../docs/architecture/adr-001-coin-source-tracing.md).

[Genesis initialization](../keeper/genesis.go) restores the port and traces,
binds the port and claims its capability if needed, and sets the transfer
[parameters](07_params.md). `SendEnabled` and `ReceiveEnabled` live in the
module's parameter subspace rather than at either KV key above. Port/channel
capabilities and core packet commitments belong to their respective keepers;
balances, minted vouchers and escrow balances are bank state.
