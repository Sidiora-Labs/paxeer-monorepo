# Historical rewards value hazard

The frozen v6.3.0 distribution precompile at `0x0000000000000000000000000000000000001007` accepts native value on a raw `CALL` to `rewards(address)`, even though its ABI declares the method `view`. The EVM transfers that value to the precompile address while the call returns rewards. Querying rewards does not refund the payment or withdraw rewards.

Clients must set the transaction or CALL value to zero. Contracts can use zero-value `STATICCALL` for this query. ABI mutability metadata alone does not prevent a raw caller from supplying value.

Historical replay retains these exact semantics. The current distribution executor rejects nonzero rewards value from the recorded `v7.0` completed upgrade height, inclusive; a scheduled plan or the current binary version alone does not activate that refusal. Historical tracing selects the executor recorded at the requested height, while ordinary execution uses the current executor with its height gate. Clients should continue to send zero value at every height.

`repair_m1260_test.go` proves historical transfers, current routing and rollback at the activation boundary, and zero-value CALL/STATICCALL results against real in-memory keepers and stores. Its heights 10 and 20 are test fixtures, not network activation heights. The legacy implementation and ABI remain unchanged for replay.
