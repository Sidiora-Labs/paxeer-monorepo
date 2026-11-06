# Upstream revisions

The explorer consists of imported Blockscout sources plus local Paxeer
changes. Paths in this table are relative to the monorepo root; the pins
identify the original imports, not the current contents of the fork.

| Current path | Upstream repository | Tag | Commit | Licence |
| --- | --- | --- | --- | --- |
| [`services/explorer/backend/`](../../services/explorer/backend/) | `blockscout/blockscout` | `v10.2.6` | `90f7dd8e9348123b74dfd23f69ab7da76191a820` | GPL-3.0 |
| [`apps/explorer/`](../../apps/explorer/) | `blockscout/frontend` | `v2.7.2` | `446c409eeb54274aab90ff371a705f9699cd84ef` | GPL-3.0 |
| [`services/explorer/services/`](../../services/explorer/services/) | `blockscout/blockscout-rs` | none | `934a80f42976bac8e13dc45770104a0ede8bd7d7` | MIT |

`934a80f42976bac8e13dc45770104a0ede8bd7d7` is the last `blockscout-rs` revision
that still carries `LICENSE-MIT`; the next commit,
`d1342aaa1ea8beeb3d55601a9989e680a5d79657`, removed it. The imported MIT text
is retained at [`services/LICENSE-MIT`](../../services/explorer/services/LICENSE-MIT). GPL-3.0 texts
are retained at [`LICENSE`](../../services/explorer/LICENSE), [`backend/LICENSE`](../../services/explorer/backend/LICENSE)
and [`apps/explorer/LICENSE`](../../apps/explorer/LICENSE).

## What was changed during the import

- Upstream `.github/` directories were dropped from all three trees so their
  workflows would not run in this repository's CI. Explorer workflows live in
  the monorepo's `.github/workflows/`.
- From `blockscout-rs`, only `smart-contract-verifier`, `sig-provider` and the
  shared `libs` workspace were kept under `services/explorer/services/`.
  Their workspace path dependencies remain within those imported directories.
- The frontend import now maps to `apps/explorer/`. An upstream path such as
  `configs/envs/` or `deploy/tools/envs-validator/test/` is relative to that
  directory, not to `services/explorer/`.
- Upstream per-chain `.env.*` presets under `configs/envs/` and
  `deploy/tools/envs-validator/test/`, and the backend's
  `.devcontainer/.blockscout_config.example`, were omitted from the tracked
  import by the repository's ignore rules. Recover an omitted upstream preset
  from its pinned revision when needed.
- [`apps/explorer/configs/envs/.env.vitest`](../../apps/explorer/configs/envs/.env.vitest)
  is tracked explicitly because frontend unit tests load it. It retains
  upstream test values, including localhost placeholders and the upstream
  network name, which the imported snapshots and specs expect. Product
  presets are [`paxeer-x.env`](../../apps/explorer/configs/envs/paxeer-x.env)
  and [`paxeer-x-dev.env`](../../apps/explorer/configs/envs/paxeer-x-dev.env)
  in the same directory. Local Compose configuration is under
  [`deploy/env/`](../../services/explorer/deploy/env/).

## Comparing the fork with upstream

The imported content was byte-identical at the pins above except for the
stated omissions. Subsequent repository changes include the Paxeer adapter,
native account and feed APIs, archive routes, frontend features and deployment
configuration. Compare a current file with its corresponding upstream-relative
path at the pinned commit, then consult this repository's history for local
changes; the current trees are not claimed to be byte-identical to upstream.

The [explorer overview](../../services/explorer/README.md) describes the merged network, the current
layout and the independent process and licence boundaries. The
[backend overview](../../services/explorer/backend/README.md) and
[account API blueprint](../../services/explorer/backend/apps/block_scout_web/API%20blueprint.md)
describe the fork's current HTTP interfaces rather than the imported examples.
