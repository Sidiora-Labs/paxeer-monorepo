# sid-snapshot

[`snapshot.py`](snapshot.py) builds the SID holder list for the v6.12.0 migration:
a JSON array of
`{"address": "0x…", "balance": "<base units>"}` entries, sorted by address,
in the shape of
[`node/testdata/sid-holders.json`](../../../node/testdata/sid-holders.json).
Addresses are lowercase and balances are decimal strings without unit conversion.
The token address is fixed at `0x21f7b20a555199fa73a238b1a91fd0f549068fee`.

The SID holder migration and SID mint-authority move run as hooks of the
v6.12.0 handler in [`node/upgrades_v6120.go`](../../../node/upgrades_v6120.go)
and ship together in that release. Paxeer is one BFT network whose kernel is
part of the network and proposes blocks. A receipt is final when its block is
committed; the exporter selects that committed state via the RPC `finalized` tag.

## Export

Run from the repository root with Python 3. Postgres sources additionally require
`psycopg2`; CSV sources use only the Python standard library.

```sh
python3 tools/paxeer-x/sid-snapshot/snapshot.py \
  --source "$EXPLORER_DSN" \
  --source "$OLD_PAXSCAN_DSN_OR_CSV" \
  --rpc "$PAXEER_RPC_URL" \
  --out sid-holders.json
```

`--source` may be repeated; `--rpc` and `--out` are required. Set
`PAXEER_RPC_URL` to an archive node serving Paxeer EVM chain ID `125` (`0x7d`).
The [public node JSON-RPC URLs](../../../docs/site/docs/reference/public-rpc.md)
use the root path, for example `https://api1.mainnet-beta.paxeer.network`.
Historical state and method support must be available on the chosen node; a
public node URL alone does not guarantee archive support. The exporter records
`eth_chainId` in its evidence but does not enforce an expected value, so check
that value before accepting the artifact.

## Snapshot and reconciliation

- `--rpc` is required. Use an archive RPC supporting `finalized`, canonical
  block-hash selectors (EIP-1898) for `eth_call`, and complete `eth_getLogs`
  history. Unsupported or unavailable historical state stops the export.
- The exporter selects the finalized block once. All `balanceOf` and
  `totalSupply` calls use that block's hash. Source queries and canonical
  Transfer log queries stop at its height; the hash is checked again before
  writing output. There is no height override: a later invocation may select a
  different finalized block.
- `--source` takes a Postgres DSN (Blockscout `token_transfers` table, needs
  `psycopg2`) or a CSV with columns
  `transaction_hash,log_index,from_address,to_address,amount,block_number`.
  Transaction hashes are 32-byte hex values. Repeat the option to merge
  sources; duplicate `(transaction_hash, log_index)` entries must agree.
  SQL selects canonical transfers through the pinned height; future CSV rows
  are excluded. Old CSVs without block numbers must be re-exported.
- The merged source must contain every canonical SID Transfer from genesis
  through the pinned height. The exporter checks it against RPC logs queried in
  ranges of at most 2000 blocks, comparing the transfer address, amount and
  height fields keyed by transaction hash and log index.
  Stale, incomplete, or conflicting history stops the export.
- Candidates are all nonzero addresses appearing as sender or recipient in
  either source history or canonical RPC logs, including addresses with zero or
  negative history balances. Each candidate is read with `balanceOf` at the
  pinned hash. A balance mismatch prints a diagnostic and uses the on-chain
  value; the subsequent history and supply checks still have to pass.
- Only positive on-chain balances are emitted, once per address. Their sum and
  the canonical history's minted supply minus burns must each equal pinned
  `totalSupply`. Addresses absent from both histories are not enumerated; the
  supply equality is a check against omitted positive balances.

## Evidence and migration handoff

- `sid-holders.json.evidence.json` records chain id, SID address, finalized
  block number/hash, history range, transfer count, total supply, holder count,
  and SHA-256 of the holder file. Retain both files together.
- The run prints `holders <count> sum <total>` only after validation succeeds.

Review the evidence's chain ID, token, block number/hash and holder-file digest
before handing off the artifact. The exporter writes only to `--out` and its
evidence sidecar; it does not replace the embedded release snapshot. Retain the
qualified pair together so the snapshot block can be identified later.

The upgrade embeds `node/testdata/sid-holders.json` through
[`node/upgrades_v611.go`](../../../node/upgrades_v611.go). Its
`migrateSIDHolders` implementation reads each listed holder's actual legacy
storage balance at migration time, uses that value if it differs from the list,
and records any uncovered supply as a claimable shortfall. Snapshot validation
therefore establishes the exported block's balances; it does not freeze balances
until the upgrade or discover holders added after the snapshot. The evidence
sidecar is retained for review and is not consumed by the upgrade handler.
See the [governance window kit](../gov/README.md) for the `v6.12.0` plan renderer.

## Existing tests

[`test_repair_22_31_429.py`](test_repair_22_31_429.py) exercises the current
exporter with CSVs and an in-process JSON-RPC server. Its focused command from
the repository root is
`python3 -m pytest -q tools/paxeer-x/sid-snapshot/test_repair_22_31_429.py`.
[`test.sh`](test.sh) is a legacy smoke test for the earlier unpinned interface:
its invocations omit required RPC and block-number inputs and do not qualify
the current exporter.
