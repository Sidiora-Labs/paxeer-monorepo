# Historical distribution rewards and CALL value

The v6.2.0 executor at `0x0000000000000000000000000000000000001007`
remains frozen for historical replay. Its `rewards(address)` ABI is `view`,
but the executor does not enforce a zero CALL value. A successful raw CALL
with positive value returns rewards and transfers the attached native value
from the caller to the precompile address. It does not treat that value as a
reward deposit or refund it. This includes a one-wei payment and payments with
a fractional bank-unit remainder. Clients must send zero value when querying
historical rewards; use STATICCALL for contract queries.

The live executor in `precompiles/distribution/distribution.go` enforces zero
value once the `v7.0` upgrade is recorded as applied and the execution height
reaches that recorded height. Positive-value calls then revert and retain the
caller's value. Zero-value CALL and STATICCALL continue to return the keeper's
rewards. Before that boundary, or without an applied upgrade marker, the live
executor preserves its earlier payment behavior. The mere passage of a block
height does not activate this check.

Historical tracing selects the version recorded at the requested height. It
must continue selecting v620 during its historical interval even when the
store already contains the later merge marker. Never fix historical replay by
adding a nonpayable check to `distribution.go` in this directory.

`repair_m1259_test.go` drives the production distribution registry, keeper version selection,
EVM CALL/STATICCALL, distribution query and ABI response. Its isolated in-memory
upgrade history records v6.2.0 at height 20, v6.3.0 at 30, and v7.0 at 100.
These are fixture heights, not published network activation heights. The tests
check exact activation boundaries, positive and zero values, nonempty rewards,
and balances both immediately and after finalizing and reopening the EVM DB.
