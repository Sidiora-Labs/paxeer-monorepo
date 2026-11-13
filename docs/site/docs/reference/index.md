# Reference

Operator and developer references for the merged Paxeer network. Node RPC,
hosted service APIs and protocol encodings have distinct contracts.

## Public contracts

- [Public node RPC endpoints](public-rpc.md): chain ID 125, root HTTPS and WebSocket URLs
- [Hosted gateway JSON-RPC](../platform/gateway-rpc.md): hosted service methods
- [Public payment API transcript](../platform/public-api.md)
- [CLI](../platform/cli.md)
- [Assets and tokens](../concepts/assets.md)
- [Checkpoints and committed block evidence](../concepts/checkpoints.md)
- [Owner registration details](../human/owner-registration-details.md)
- [Modules](../protocol/modules.md)

## Specifications

The network specification describes one execution domain in `paxd`, including
the kernel, native ledger, authority modules, markets and committed receipts.
Feature availability on a node depends on activation and configuration.

| Reference | Checked-in path | Purpose |
| --- | --- | --- |
| Merged network | `spec/layerx-merge/spec.kvx` | Network requirements and behavior |
| Interface registry | `spec/layerx-merge/interfaces.kvx` | Declared interfaces, producers and consumers |
| Contracts | `spec/layerx-merge/contracts/` | Detailed module and client contracts |
| Retirements | `spec/layerx-merge/programme/data/retired.kvx` | Retired components and their replacements |

## Qualification and layout

- [Qualification](../operators/qualification.md)
- [Monorepo](../overview/monorepo.md)
- [Translations](../overview/translations.md)

## Contributing and security

- [Contribution guide](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/CONTRIBUTING.md)
- [SECURITY.md](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/SECURITY.md)
- [LICENSE](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/LICENSE)
