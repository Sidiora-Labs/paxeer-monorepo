# Reference

Operator and developer references for the merged Paxeer network. Node RPC,
hosted service APIs and historical protocol encodings have different contracts.

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

This index and [CONTRIBUTING.md](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/CONTRIBUTING.md)
provide the contributor specification references. Product guides explain behavior
and link tests by what they check; numbered development history belongs in
contributor records.

The current merged-network specification is `spec/layerx-merge/spec.kvx`.
It describes one execution domain in `paxd`, including the kernel, native
ledger and authority modules, markets and committed receipts. Its `[meta]`
status is `draft`; that status is not a claim that every requirement is
qualified or every feature is deployed. Consult each task's evidence and
the running network's activation state.

| Reference | Checked-in path | Purpose |
| --- | --- | --- |
| Merged network | `spec/layerx-merge/spec.kvx` | Requirements, decisions, tasks and qualification evidence |
| Interface registry | `spec/layerx-merge/interfaces.kvx` | Declared interfaces, producers and consumers |
| Contracts | `spec/layerx-merge/contracts/` | Detailed module and client contracts |
| Retirements | `spec/layerx-merge/programme/data/retired.kvx` | Retired components and their replacements |

Earlier protocol, platform, beta and agent-interface specification names in
archival prose are historical references. The previously listed
`spec/layerx-protocol`, `spec/layerx-platform`, `spec/layerx-beta` and
`spec/.beta/layerx-agent-interface` directories are absent from this checkout.
Use the merged specification and checked-in implementation for current behavior.

KVX is the source for specification text. Generated Markdown mirrors are not
independent normative sources and must not be edited by hand. Older pages
that describe external checkpoint settlement are archival context; current
receipt finality is BFT block commitment.

## Qualification and layout

- [Qualification](../operators/qualification.md)
- [Monorepo](../overview/monorepo.md)
- [Translations](../overview/translations.md)

## Contributing and security

- [CONTRIBUTING.md](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/CONTRIBUTING.md)
- [SECURITY.md](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/SECURITY.md)
- [LICENSE](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/LICENSE)
