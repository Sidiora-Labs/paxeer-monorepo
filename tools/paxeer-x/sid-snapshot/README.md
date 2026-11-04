# sid-snapshot

Builds the SID holder list for the v6.12.0 migration: a JSON array of
`{"address": "0x…", "balance": "<base units>"}` entries, sorted by address,
in the shape of `node/testdata/sid-holders.json`.

The SID holder migration and SID mint-authority move run as hooks of the
v6.12.0 handler in `node/upgrades_v6120.go` and ship together in that release.

```sh
python3 snapshot.py \
  --source "$EXPLORER_DSN" \
  --source "$OLD_PAXSCAN_DSN_OR_CSV" \
  --rpc "$PAXEER_RPC_URL" \
  --out sid-holders.json
```

- `--rpc` is required. Use an archive RPC supporting `finalized`, canonical
  block-hash selectors (EIP-1898) for `eth_call`, and complete `eth_getLogs`
  history. Unsupported or unavailable historical state stops the export.
- The exporter selects the finalized block once. All `balanceOf` and
  `totalSupply` calls use that block's hash. Source queries and canonical
  Transfer log queries stop at its height; the hash is checked again before
  writing output.
- `--source` takes a Postgres DSN (Blockscout `token_transfers` table, needs
  `psycopg2`) or a CSV with columns
  `transaction_hash,log_index,from_address,to_address,amount,block_number`.
  Transaction hashes are 32-byte hex values. Repeat the option to merge
  sources; duplicate `(transaction_hash, log_index)` entries must agree.
  SQL selects canonical transfers through the pinned height; future CSV rows
  are excluded. Old CSVs without block numbers must be re-exported.
- The merged source must contain every canonical SID Transfer from genesis
  through the pinned height. The exporter checks it against chunked RPC logs.
  Stale, incomplete, or conflicting history stops the export.
- Every candidate is reconciled, including addresses with zero history
  balances. Positive on-chain balances appear exactly once. Both their sum
  and the history's minted supply minus burns must equal pinned `totalSupply`.
- `sid-holders.json.evidence.json` records chain id, SID address, finalized
  block number/hash, history range, transfer count, total supply, holder count,
  and SHA-256 of the holder file. Retain both files together.
- The run prints `holders <count> sum <total>` only after validation succeeds.

The repair gate is
`python3 -m pytest -q tools/paxeer-x/sid-snapshot/test_repair_22_31_429.py`
from the repository root. It uses CSVs and an in-process JSON-RPC server.
`test.sh` is the preserved legacy smoke test for the earlier unpinned interface.
