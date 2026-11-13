# Account read authority

The account read interface at the node root is implemented in
[`rpc/px_account.go`](../../../rpc/px_account.go). Native account storage uses
[`lx_account_registry.c`](../../ledger/lx_account_registry.c) and
[`lx_account_read.c`](../../ledger/lx_account_read.c). DID binding state belongs
to the EVM addr precompile at `0x1004`.

`px_resolveAccount`, `px_getAccount` and `px_getBalances` preserve the EVM or DID
selector and pin every read to one committed watermark. Selectors accept an EVM
address or an explicit `did:layerx:` DID. The addr view supplies the forward
identity, reverse binding and derived main account ID. Native account IDs are
references, not reverse lookup selectors. For an unbound DID, `px_getAccount`
returns the native identity with `paxeer: null`; `px_resolveAccount` and
`px_getBalances` return an unavailable error. A conflicting join also returns
an unavailable error.
