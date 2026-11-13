# Historical rewards queries and CALL value

The distribution precompile at `0x0000000000000000000000000000000000001007`
has a historical v5.8.0 `rewards(address)` executor whose ABI says `view`,
but whose raw EVM `CALL` accepts nonzero value. The EVM transfers that value
from the caller to the precompile address while returning the rewards query
result. The payment does not withdraw rewards or purchase a service. The
query does not refund it. Do not treat a successful query as evidence that
no funds moved, and do not assume those funds can be recovered.

Always send zero value for rewards queries. Prefer `eth_call` for an RPC
read or `STATICCALL` for an on-chain read. A submitted transaction containing
a zero-value query can still incur its normal transaction gas fee.

Historical executors and their embedded ABIs stay unchanged for replay.
Historical tracing selects v580 from its recorded v5.8.0 upgrade height
until a later recorded version supersedes it. The current executor rejects
nonzero query value with `sending funds to a non-payable function` from the
recorded `v7.0` merge **done height**, inclusive; the EVM reverts the value
transfer. A scheduled plan alone does not enable this guard. Ordinary
execution before that height, or without a recorded merge activation,
retains the prior value behavior. Do not infer this protection merely from
the node binary version or ABI mutability.

[Routing and value tests](repair_m1254_test.go) exercise the application's
version routing, raw EVM calls, nonempty distribution keeper results, and
stored payment balances. Their recorded heights 5 and 10 are isolated fixture
heights, not network activation heights.
