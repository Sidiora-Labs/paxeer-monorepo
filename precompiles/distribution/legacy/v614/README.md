# Historical distribution rewards and CALL value

The v6.1.4 distribution precompile at `0x0000000000000000000000000000000000001007`
declares `rewards(address)` as `view`, but a raw EVM `CALL` with positive value
can succeed. The caller pays that value to the precompile address while receiving
the rewards tuple. The query does not refund the payment or withdraw rewards.
This includes wei remainders as well as whole native units. Do not attach value
to a rewards query; use a zero-value call or `STATICCALL`.

The historical executor and ABI are frozen for replay. Historical routing uses
the persisted upgrade done heights to select v6.1.4. The current implementation
rejects positive value from the recorded `v7.0` done height, inclusive, and EVM
reversion restores both balances. Before that activation, or if it has never
been applied, historical value acceptance remains possible. A scheduled plan or
the binary version alone does not establish activation. Zero-value queries keep
returning the keeper's rewards through either route.

[Routing and value tests](repair_m1258_test.go) exercise the production version
registry, upgrade store, keeper routing, EVM CALL/STATICCALL, distribution keeper,
and ABI decoding. Their heights 10 and 20 are isolated store fixtures; clients
must obtain their chain's actual applied heights rather than treating these
test numbers as network data.
