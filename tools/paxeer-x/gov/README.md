# Governance window kit

[`render.sh`](render.sh) renders the v6.12.0 upgrade plan, fee-token parameters,
web-search parameters and optional bridge proposals from the current tree. It
dry-runs each proposal through `paxd` with `--generate-only --offline`. A rejected
proposal stops the render; successful output contains unsigned transactions.

Paxeer is one BFT network: the kernel is part of the network and proposes blocks,
and a receipt is final when its block is committed. These are native chain
governance proposals. The kit retains the `v6.12.0` plan name from
[`node/upgrades_v6120.go`](../../../node/upgrades_v6120.go).

## Render

Run from the repository root with Bash, `jq`, Python 3, `realpath` and a compatible
`paxd` binary available. Bridge generation also invokes Go. The command below
shows the required arguments followed by optional arguments in brackets:

```text
tools/paxeer-x/gov/render.sh --out DIR --paxd build/paxd \
  --height H --release-url URL --release-sha256 HEX --deposit COINS --pax-usd PRICE \
  --pax-asset-id HEX64 --sid-asset-id HEX64 \
  --market-pax-sid HEX64 --market-btc-usd HEX64 --market-eth-usd HEX64 \
  --custody-network-id N --kernel-network-id N --deposit-authority HEX64 \
  --sequencer-id HEX64 --sequencer-pubkey HEX64 \
  [--perp-margin pax|sid] [--rate-update-height H] [--sid-pointer 0xADDR] \
  [--first-batch N] [--last-batch N] [--bridge-config FILE]... [--bridge-manifest FILE] \
  [--from BECH32] [--chain-id ID]
```

`--paxd` overrides `PAXD`, which otherwise defaults to `paxd` on `PATH`.
`--chain-id` defaults to the Cosmos chain identifier `hyperpax_125-1`; this is
distinct from EVM chain ID `125` (`0x7d`). `--from` defaults to the governance
module address, which is also fixed in the xweb message's `authority` field;
overriding `--from` does not change that message authority.

The current parser still requires `--pax-asset-id`, `--sid-asset-id`, the three
`--market-*` arguments, `--custody-network-id`, `--deposit-authority`,
`--sequencer-id` and `--sequencer-pubkey`. They are legacy inputs and do not add
market, custody or anchor proposals. In particular, the two sequencer values
must be nonzero, 64-character lowercase hex strings without `0x`, and the custody
network ID must be positive. The retained `--kernel-network-id` is a positive
uint32 used by the xweb proposal; use `125` for mainnet-beta.

The legacy optional arguments `--perp-margin` (default `pax`), `--sid-pointer`,
`--first-batch` (default `0`) and `--last-batch` (default `18446744073709551615`)
also produce no additional proposal. Batch bounds must be ordered uint64 values.

## Outputs

| Output | Content | Dry run |
| --- | --- | --- |
| `01-software-upgrade.json` | Plan `v6.12.0` at `--height`; `info.binaries["linux/amd64"]` contains the release URL plus `?checksum=sha256:<hash>` | `tx gov submit-proposal software-upgrade` |
| `02-fee-token-params.json` | EVM parameter changes: `KeyFeeTokenEnabled` = true, `KeyAllowedFeeDenoms` = `usid` at rate `3114000` and the rendered `rate_update_height`, `KeyMaxFeeTokenRateAge` = `3400000` blocks | `tx gov submit-proposal param-change` |
| `03-xweb-fee.json` | `LayerXProposal` containing `xweb.MsgSetParams`: fee worth 0.001 USD in `uhpx` at `--pax-usd`, payload cap `8192`, callback gas cap `500000`, timeout `3600` blocks, supplied `kernel_network_id`, and `program_fee_denoms` = `uhpx`, `usid`, `uusdc`, `uusdl` | `tx gov submit-proposal layerx-proposal` |
| `bridge/<name>/` | For each `--bridge-config`, bridge message bodies in `bodies/` and proposal content in `proposals/04-proposal-open-chain.json`; `proposals/05-proposal-sidiora-cap.json` is emitted for Solana | `tx gov submit-proposal layerxbridge-proposal` for each emitted proposal |

Only the three numbered top-level templates in [`templates/`](templates/) are
rendered. The retired exchange-market, custody and anchor-authorization proposals
are no longer emitted.

Each successfully dry-run proposal gets a `<file>.tx.json` beside it. The script
also requires exactly one message in that transaction. Bridge bodies are inputs
to their enclosing proposals and do not each get a transaction sidecar.

## Review the generated files

- `--height` and `--rate-update-height` must be positive integers; the latter
  defaults to `--height`. Review it against the intended execution height and
  fee-token rate-age policy. The rendered rate is `3114000` usid per whole PAX.
- `--release-sha256` must be 64 lowercase hex characters. The renderer inserts
  the supplied URL and hash; it does not download or verify the release asset.
- `--pax-usd` must be positive and finite. The xweb fee is
  `0.001 / price * 1000000` uhpx, rounded half up; values rounding below one uhpx
  are rejected.
- Repeat `--bridge-config` for multiple bridge chain configurations. `<name>`
  comes from each configuration's `name` field. `--bridge-manifest` defaults to
  `services/bridge/deploy/attestors.json`. See the
  [bridge proposal generator](../../../services/bridge/deploy/proposals/) for
  its message types and validation. Bridge configurations register remote chains;
  Paxeer sub-chains are tenant domains in the database.
- Use a fresh output directory. Top-level files from previous runs are not
  cleaned, and an aborted render may leave partial files. Offline generation
  checks transaction construction; it does not execute proposals or submit them.
- [`test.sh`](test.sh) builds `paxd` unless `PAXD` is supplied, renders fixed
  inputs and checks the retained proposals and unsigned transactions. Its
  transaction dry runs are offline; build dependencies must be available.

For the holder-list artifact consumed by the upgrade, see the
[SID snapshot exporter](../sid-snapshot/README.md).
