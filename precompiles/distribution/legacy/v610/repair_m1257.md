# Task 22.33.1257 — historical rewards payment behavior

Finding: `historical-rewards-value-v610` (audit 22.30.62, domain 6).

`distribution.go` and `abi.json` remain byte-for-byte unchanged. The historical
`v6.1.0` executor accepts positive EVM value on a raw CALL to `rewards(address)`.
The value is transferred from the caller to `0x0000000000000000000000000000000000001007`;
the returned rewards query does not refund it. The ABI's `view` annotation does
not enforce nonpayability. Clients must send zero value, preferably use a static
query, and never treat ABI mutability as protection against a raw payment.

The repaired live executor rejects positive value once the recorded `v7.0`
upgrade is active at the requested block height. A scheduled or unapplied plan
does not activate the repair. Historical replay must retain the old behavior.
Test heights 5 (`v6.1.0`) and 10 (`v7.0`) are isolated in-memory upgrade records,
not mainnet activation-height assertions.

| Proving test | Criterion |
| --- | --- |
| `TestRepairM_1257_HistoricalRawCALL` | Real version routing selects v610 at its recorded activation and before the merge; raw CALL returns keeper-produced rewards and transfers sub-bank-unit, whole-bank-unit and mixed payments, also visible after state DB finalization and reopening. |
| `TestRepairM_1257_LiveActivationBoundary` | Real live and replay routing serves the repaired executor at/after activation and refuses payment without changing either balance; before activation or with an unapplied plan, historical payment behavior remains. |
| `TestRepairM_1257_ZeroValueQueries` | Both versions return actual nonempty keeper rewards through CALL and STATICCALL with zero value and unchanged balances. |

Baseline: no `repair_m1257_test.go` existed in the initial tree and no prior task
gate ledger existed. No unchanged-tree verification was run: the explicit gate
order permits verification only through `hf-done` after implementation and
`hf-build`. No baseline test failure is claimed. Qualification revision, command,
exit code and private log are recorded by `hf-build`/`hf-done` in the runner's
gate ledger and final task evidence.

Qualification finding: revision `dc10fd93e64c61f0792b390b91b231abaf6fa042`, `cg spec done 22.33.1257 -f layerx-merge --root /work/paxeer-monorepo`, exit 1, private log `/out/logs/verify-1.log` (compiler details `/tmp/repair-m1257.log` at that run). The full node fixture imported pre-existing auth, staking and tokenfactory CLI call sites incompatible with `tx.NewFactoryCLI`'s two return values; the repaired fixture assembles the real keepers directly over an in-memory store, avoiding those CLI packages without changing them or any assertion.

Attempt 3 retains the attempt-2 fixture and every assertion on base
`ca3015b61333a7ebcaedf0914337d4460107fe1a`, with prerequisite `22.31.463`
recorded done. The package's existing tests still compile alongside this fixture;
the prerequisite carries their transitive CLI callers through the fallible factory
interface. Qualification uses this attempt's gate ledger and does not reuse the
earlier failed verification as evidence of closure.

Wiring request — domain 6, `precompiles/distribution`: retain these historical/repaired routing assertions in the owning package's activation regression coverage; that directory is outside this task's editable scope.

Wiring request — domain 6, `docs/system/precompiles.md`: publish the historical rewards CALL payment hazard and zero-value client guidance above with the `v7.0` activation boundary; public documentation is outside this task's editable scope.
