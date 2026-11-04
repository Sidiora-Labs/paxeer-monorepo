# Historical distribution rewards

The v6.4.0 executor at `0x0000000000000000000000000000000000001007`
is frozen for historical replay. Although its ABI declares `rewards(address)`
as `view`, a raw EVM CALL with nonzero value succeeds and transfers that value
from the caller to the precompile. The query does not refund the payment or
convert it into staking rewards. This includes individual wei and payments
with a fractional native denomination remainder.

Clients must send zero value when querying rewards. Use `eth_call` or a
zero-value STATICCALL for reads; do not infer runtime payment rejection from
the historical ABI's `view` annotation. Never attach a payment to this query.

The current distribution executor rejects nonzero rewards CALL payments from
the recorded `v7.0` done height, inclusive. A scheduled upgrade or a newer
binary alone does not activate this rule. Calls below that height retain the
historical behavior. Historical tracing selects v640 from its recorded
`v6.4.0` activation until a later version takes over; normal execution uses
the current executor, whose payment check is gated by the recorded merge
height. The repair tests use in-memory activation heights 10 and 30; these
are fixtures, not deployment heights.
