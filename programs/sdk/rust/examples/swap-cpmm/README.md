# Constant-product swap reference

This ABI-v2 guest implements constant-product pool accounting for two LXT-20
programs and tracks shares backed by a third asset in swap-derived accounts.
Pool settlement requires tokens that accept authenticated nested calls. The
supplied [LXT-20 settlement reference](../token-lxt20/README.md) accepts direct
calls only and cannot serve as a pooled token.

## Token contract

[`src/lib.rs`](src/lib.rs) moves pooled tokens through `program_call`, using
canonical SDK `lxt20::Request` calldata assembled by
[`TokenCall`](src/token_call.rs). The runtime dispatches nested calls to
`layerx_call`. A compatible token must support this settlement route:

- A pull sends `TransferFrom { owner, to, amount }`. The owner is the invoking
  user; the recipient is the token account derived from the swap program's
  id32. The token must authenticate the immediate calling program as the
  allowance spender and subtract the exact amount from that user's allowance
  to the pool.
- A push sends `Transfer { to, amount }`. The recipient is the token account
  derived from the invoking user's id32. The token must debit the account
  derived from the immediate calling pool's id32, without requiring an allowance
  from the user receiving the payout.
- Each request carries shared-storage read/write grants and an exact
  `ProgramSpend` grant for the token program, source seed, backing asset,
  recipient and amount. These grants must be held by the caller and admitted
  by the token's registered interface; a nested call cannot expand authority.
  Token balances, allowances and backing transfers must stage in the same
  native transaction as the pool's changes, so a refused leg rolls back all
  prior effects.

`NestedTokenCall::decode` specifies that settlement-only route using an
immediate caller supplied by the runtime, never a caller identity in calldata.
It is currently a helper in this example, not a handler installed in the token.
The supplied token's `layerx_call` unconditionally refuses, and its named
`transfer` and `transfer_from` exports also reject nested callers through
`direct()`. Its registry interface binds named direct-call entries, not the
required nested route. Reusing the canonical request bytes alone does not
make that token compatible.

The required wire contract is:

| Pool operation | Canonical method | Ordinal | Request bytes | Debit seed | Allowance spender | Recipient seed |
| --- | --- | --- | --- | --- | --- | --- |
| Pull | `transfer_from` | 3 | 90 | user id32 | immediate pool id32 | pool id32 |
| Push | `transfer` | 1 | 58 | immediate pool id32 | none | user id32 |

Both frames start with `LX`, token discriminator byte 20 and the ordinal,
followed by the bounded-bytes envelope. The authenticated caller is absent
from these frames: `TransferFrom.owner` identifies the debited user, not the
spender. For a push, neither the pool owner nor a spender is supplied in calldata.

The supplied token uses `REFERENCE_ASSET` (byte `0x44` repeated 32 times),
which differs from both swap backing constants. Its fixture uses program
`[0x55; 32]`, while the pool expects `[0x5a; 32]` and `[0x5b; 32]`.
A deployment must bind each token id, backing asset and registered interface
consistently. Program-owned pool recipients need registration in both token
storage and the native Programs account module; the supplied token's direct
`approve` registers only its invoking user in token storage.

## Accounts and shares

`TOKEN_A` is program `[0x5a; 32]` with backing asset `[0x6a; 32]`; `TOKEN_B` is
program `[0x5b; 32]` with backing asset `[0x6b; 32]`. These are source constants
that must agree with deployed token interfaces and registered backing assets.
For each token, the pool's account uses the swap program's id32 as the seed;
a user's account uses that user's id32. Token recipients need registration in
both token storage and the native Programs account module.

Share balances live in swap shared storage. Adding liquidity transfers existing
`SHARE_ASSET` units (`[0x5c; 32]`) from the swap-derived treasury with seed
`lx.ref.swap.treasury` to the holder account derived from the invoking user's
id32; removing liquidity returns them. The treasury must be registered and
funded, and holder accounts must be registered for this asset before settlement.
The guest updates share accounting but does not mint backing units or expose the
seven-method LXT-20 token interface. Total outstanding shares cannot exceed
`SHARE_CEILING` (100,000).

The swap's own backing payments use `transfer_program_402` for shares. It does
not invoke `transfer_402` or `fund_program_402`; pooled-token backing transfers
are the compatible token program's responsibility under the narrowed grants.

## Entries and arithmetic

The five named exports use discriminator `LXS` followed by the entry ordinal,
then the canonical bounded-bytes envelope `[1, 0x20] || length:u32 || payload`.
Lengths and amounts are big-endian; amounts are 16-byte u128 values and account
ids are 32 bytes. The generic `layerx_call` entry refuses.

- `add_liquidity` (1): payload `recipient || shares || maximum_a || maximum_b`
  (80 bytes). The recipient must be the invoking user's derived share account.
  For an existing pool it charges `ceil(shares * reserve / supply)` in each
  token, subject to the supplied maxima. The first deposit charges `shares`
  in A and `maximum_b` in B, setting the initial price. Both charges and the
  share count must be nonzero and the next supply must stay within the ceiling.
- `remove_liquidity` (2): payload `treasury || shares || minimum_a || minimum_b`
  (80 bytes). The treasury must match the rederived treasury account. It returns
  `floor(shares * reserve / supply)` in each token, requiring nonzero payouts
  at least as large as the minima, and debits the invoking user's shares.
- `swap_exact_in` (3): payload `direction || amount_in || min_out` (33 bytes).
  Direction 0 is A to B and 1 is B to A. It computes
  `net_in = floor(amount_in * 9970 / 10000)` and
  `out = floor(reserve_out * net_in / (reserve_in + net_in))`. It refuses zero
  input, empty reserves, zero net input or output, an output that drains the
  reserve, and output below `min_out`. The full input enters the reserves;
  the 30-basis-point fee and integer rounding remainder stay in the pool.
- `quote` (4): payload `direction || amount_in` (17 bytes). It uses the same
  output calculation without moving value.
- `reserves` (5): empty payload. It reads `reserve_a`, `reserve_b` and
  `total_shares` from shared storage; absent values read as zero.

Responses use the same bounded-bytes envelope without a discriminator.
Liquidity mutations return an empty payload; swaps and quotes return a 16-byte
output amount; `reserves` returns `reserve_a || reserve_b || total_shares`
(48 bytes).

Products use SDK `U256::widening_mul` through `bigint_mul_256`, followed by
checked-width division in the guest. Products beyond 256 bits and results
beyond u128 are refused, and reserve/supply updates use checked arithmetic.
For successful settlement with a compatible token, the stored reserves track
amounts paid in minus amounts paid out.

## Build

```sh
cd programs/sdk/rust/examples/swap-cpmm
cargo build --locked --release --target wasm32-unknown-unknown
```

Run Cargo from the example directory so
[`programs/.cargo/config.toml`](../../../../.cargo/config.toml) applies its
vendored, offline sources and WebAssembly code-generation flags.

The registry's [`swap::reference_interface`](../../../../crates/layerx-programs-registry/src/swap.rs)
binds the module to the five named exports, schemas and capability declarations.
Its `swap_interface` example writes a canonical interface and registry state
value for a supplied program id and prints the interface digest. These outputs
do not register or deploy the pooled tokens.

The committed `swap-cpmm.wasm`, `swap-cpmm.interface` and
`swap-cpmm.registry-value` in [`programs/fixtures/pay5`](../../../../fixtures/pay5)
use a swap program id of byte `0x55` repeated 32 times. From the repository root,
`make programs-reference-fixtures` invokes `scripts/programs/program-fixtures.sh`
to regenerate the fixture set. It remaps checkout source paths and generates
the interface and registry state value from the module built in that run.
Those artifacts bind the swap module to its interface. Token deployment and
account registration are separate requirements.
