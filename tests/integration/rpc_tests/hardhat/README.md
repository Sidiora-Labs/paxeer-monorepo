# Local Hardhat mainnet fork

This config starts an optional Ethereum mainnet fork at
`http://localhost:9546`, using a **required pinned block**. It is a manual
reference for response-shape exploration with real mainnet data. The automated
[Paxeer RPC suite](../README.md) compares against a separate geth `--dev` node;
starting this fork does not switch its reference provider.

## Quick start

Install dependencies from `tests/integration/rpc_tests/` with `npm ci`. From
that same directory, start the fork in a dedicated terminal:

```bash
ETH_MAINNET_UPSTREAM='https://your-ethereum-provider.example/' \
ETH_MAINNET_FORK_BLOCK=21000000 npm run rpc:fork
```

Replace the upstream URL with your Ethereum mainnet provider and choose a
positive pinned block whose historical state it serves. Keep the process running
while making manual JSON-RPC calls to `http://localhost:9546/`.

To run the automated suite, separately start geth with `npm run rpc:geth`,
prepare the local Paxeer cluster, and run `npm run test:rpc` as described in the
[parent README](../README.md). The fork alone does not satisfy bootstrap's geth
requirement.

## Environment

| Variable | Default | Requirement |
| -------- | ------- | ----------- |
| `ETH_MAINNET_UPSTREAM` | None | Required, nonempty Ethereum mainnet RPC URL. |
| `ETH_MAINNET_FORK_BLOCK` | None | Required positive safe integer after JavaScript `Number` conversion. |

[`hardhat.config.ts`](hardhat.config.ts) throws during configuration loading
when the upstream is missing or the block is unset, zero, negative, fractional,
non-numeric or outside the safe-integer range. There is no latest-block default.
An unset, empty or whitespace-only block produces the actionable missing-value
error `ETH_MAINNET_FORK_BLOCK is not set` with a startup command that supplies
both required variables. For other invalid values, the block validation error is
`ETH_MAINNET_FORK_BLOCK must be a positive pinned block number`.

## Notes

- The fork's `chainId` is `1` for Ethereum mainnet. Paxeer mainnet-beta has EVM
  chain ID 125; this fork is a separate test reference.
- Mining is automatic with interval `0`.
- Paths resolve relative to this config directory: artifacts use `.artifacts/`,
  cache uses `.cache/`, and sources use `sources/`. The module's compile config
  reads `contracts/` and writes to its own `artifacts/` and `cache/` instead.
- `RPC_ETH_FORK` configures the optional fork URL in the module's endpoint map;
  `bothProviders()` selects `RPC_ETH_GETH` for automated comparisons.
