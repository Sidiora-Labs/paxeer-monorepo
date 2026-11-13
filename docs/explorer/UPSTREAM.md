# Upstream revisions

The explorer consists of imported Blockscout sources plus local Paxeer
changes. Paths in this table are relative to the monorepo root; the pins
identify the original imports, not the current contents of the fork.

| Current path | Upstream repository | Tag | Commit | Licence |
| --- | --- | --- | --- | --- |
| [`services/explorer/backend/`](../../services/explorer/backend/) | `blockscout/blockscout` | `v10.2.6` | `90f7dd8e9348123b74dfd23f69ab7da76191a820` | GPL-3.0 |
| [`apps/explorer/`](../../apps/explorer/) | `blockscout/frontend` | `v2.7.2` | `446c409eeb54274aab90ff371a705f9699cd84ef` | GPL-3.0 |
| [`services/explorer/services/`](../../services/explorer/services/) | `blockscout/blockscout-rs` | none | `934a80f42976bac8e13dc45770104a0ede8bd7d7` | MIT |

The MIT text is retained at [`services/LICENSE-MIT`](../../services/explorer/services/LICENSE-MIT). GPL-3.0 texts are retained at [`LICENSE`](../../services/explorer/LICENSE), [`backend/LICENSE`](../../services/explorer/backend/LICENSE) and [`apps/explorer/LICENSE`](../../apps/explorer/LICENSE).

## Repository layout

- Explorer workflows live in the monorepo's `.github/workflows/`.
- `services/explorer/services/` contains `smart-contract-verifier`, `sig-provider` and their shared `libs` workspace.
- Frontend paths such as `configs/envs/` and `deploy/tools/envs-validator/test/` are relative to `apps/explorer/`.
- [`configs/envs/.env.vitest`](../../apps/explorer/configs/envs/.env.vitest) contains the frontend unit-test configuration. Product presets are [`paxeer-x.env`](../../apps/explorer/configs/envs/paxeer-x.env) and [`paxeer-x-dev.env`](../../apps/explorer/configs/envs/paxeer-x-dev.env) in the same directory.
- Local Compose configuration is under [`deploy/env/`](../../services/explorer/deploy/env/).

## Current interfaces

The fork includes the Paxeer adapter, native account and feed APIs, archive routes, frontend features and deployment configuration. The pins above identify upstream baselines for comparison and attribution.

The [explorer overview](../../services/explorer/README.md) describes the merged network, the current layout and the independent process and licence boundaries. The [backend overview](../../services/explorer/backend/README.md) and [account API blueprint](../../services/explorer/backend/apps/block_scout_web/API%20blueprint.md) describe the fork's current HTTP interfaces.
