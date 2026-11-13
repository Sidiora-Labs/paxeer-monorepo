# Paxeer Names shell variables

The Paxeer Names shell in `apps/names` serves published `.pax` sites after verifying their storage proofs, manifest digests and object digests. Its runtime configuration comes from environment variables; the application reads no configuration file.

## Required

These variables are required to serve or verify a published site. None has a source default. The blank entries in [`apps/names/.env.example`](../../../apps/names/.env.example) must be supplied for the deployment.

| Name | What it sets | Source default | Secret | Source file that reads it |
| --- | --- | --- | --- | --- |
| `NAMES_BUCKET_BASE_URL` | Public content bucket base URL used to fetch manifests and objects. | None | No | [route.ts][route] |
| `NAMES_CONSENSUS_RPC_URL` | Consensus RPC base URL used to obtain committed headers for storage-proof verification. | None | No | [route.ts][route] |
| `NAMES_RESOLVER` | Resolver contract address used for name resolution and displayed in verification results. | None | No | [route.ts][route], [page.tsx][page] |
| `NAMES_RPC_URL` | EVM JSON-RPC URL used for name resolution and storage proofs. | None | No | [route.ts][route] |
| `NAMES_SHELL_DOMAIN` | Shell domain suffix used to recognize site hosts, validate routing and construct verification requests. | None | No | [route.ts][route], [proxy.ts][proxy], [page.tsx][page] |

## Optional

The application reads no optional environment variables or configuration-file settings.

| Name | What it sets | Source default | Secret | Source file that reads it |
| --- | --- | --- | --- | --- |

[route]: <../../../apps/names/src/app/_pns/[label]/[[...path]]/route.ts>
[page]: ../../../apps/names/src/app/page.tsx
[proxy]: ../../../apps/names/src/proxy.ts
