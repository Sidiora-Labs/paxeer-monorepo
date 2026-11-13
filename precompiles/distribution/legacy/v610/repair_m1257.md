# Historical distribution rewards and CALL value

The v6.1.0 distribution executor at
`0x0000000000000000000000000000000000001007` accepts positive native value on
a raw EVM `CALL` to `rewards(address)`. The EVM transfers that value from the
caller to the precompile, and the returned rewards query does not refund it.
The ABI's `view` annotation does not enforce nonpayability. Clients must send
zero value; use `eth_call` or `STATICCALL` for reads.

Historical replay selects this executor from its recorded v6.1.0 upgrade
height until a later version supersedes it. The
[current executor](../../distribution.go) rejects positive query value from
the recorded `v7.0` upgrade done height, inclusive. The EVM then reverts the
payment. A scheduled or unapplied plan does not activate that check.
Zero-value queries return the keeper's rewards through either implementation.

[Routing and value tests](repair_m1257_test.go) cover historical payments,
the current activation boundary, persisted balances, and zero-value
`CALL` and `STATICCALL` results.
