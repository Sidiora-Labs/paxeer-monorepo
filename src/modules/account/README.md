# Account read authority

The account read interface at the node root is `rpc/px_account.go`. Native
account storage remains in `src/ledger/lx_account_registry.c` and
`src/ledger/lx_account_read.c`; this module path does not introduce a second
registry. DID binding state belongs to the EVM addr precompile at `0x1004`.

`px_resolveAccount`, `px_getAccount` and `px_getBalances` preserve the EVM or DID
selector and pin every read to one committed watermark. The addr view supplies
the forward identity, reverse binding and derived main account ID. Native
account IDs are references, not reverse lookup selectors. An unbound DID has
no EVM balance list; a conflicting join is an explicit unavailable error.
