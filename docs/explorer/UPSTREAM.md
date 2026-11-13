# Explorer components and licences

The explorer includes a frontend, a backend and supporting services. Paths are
relative to the monorepo root.

| Component | Source | Licence text |
| --- | --- | --- |
| Frontend | [apps/explorer](../../apps/explorer/) | [GPL-3.0](../../apps/explorer/LICENSE) |
| Backend | [services/explorer/backend](../../services/explorer/backend/) | [GPL-3.0](../../services/explorer/backend/LICENSE) |
| Supporting services | [services/explorer/services](../../services/explorer/services/) | [MIT](../../services/explorer/services/LICENSE-MIT) |

The supporting-services workspace contains `smart-contract-verifier`, `sig-provider`
and shared `libs`. Explorer workflows live in `.github/workflows/`.

Frontend configuration lives in [configs/envs](../../apps/explorer/configs/envs/).
It includes [unit-test configuration](../../apps/explorer/configs/envs/.env.vitest)
and the [public](../../apps/explorer/configs/envs/paxeer-x.env) and
[development](../../apps/explorer/configs/envs/paxeer-x-dev.env) presets.
Local Compose configuration is under [deploy/env](../../services/explorer/deploy/env/).

The [explorer overview](../../services/explorer/README.md) describes the network,
layout and process boundaries. The [backend overview](../../services/explorer/backend/README.md)
and [account API blueprint](../../services/explorer/backend/apps/block_scout_web/API%20blueprint.md)
describe the HTTP interfaces. Frontend API paths and payload types live in
[lib/api](../../apps/explorer/lib/api).
