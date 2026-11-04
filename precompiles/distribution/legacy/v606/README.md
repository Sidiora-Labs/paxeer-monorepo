# Historical rewards value hazard

The distribution precompile at `0x0000000000000000000000000000000000001007`
exposes `rewards(address)` as a `view` query. The v6.0.6 executor nevertheless
accepts a raw EVM `CALL` with positive native value: the EVM transfers that
value to the precompile and the executor returns rewards without refunding
the payment. This includes amounts smaller than one bank unit and amounts
with a wei remainder. A successful rewards response does not mean the
attached value was returned, credited as a delegation, or used to claim rewards.

Clients must send zero value for rewards queries. Prefer `STATICCALL` in
contracts and zero-value `eth_call` requests when reading rewards.

`distribution.go` and `abi.json` remain unchanged for historical replay.
Historical tracing selects v6.0.6 using the recorded upgrade height; the
regression fixture pins its activation at height 151573570. Live routing
uses the current distribution implementation, which rejects positive value
from the recorded `v7.0` done height, inclusive. A scheduled or absent merge
plan does not activate that refusal. Tests use a committed in-memory merge
height of 151573580; this is not a network deployment height.

`repair_m1256_test.go` exercises the production keeper routing, EVM value
transfer, distribution query, ABI response and persisted balances. It also
checks zero-value `CALL` and `STATICCALL` on both implementations.
