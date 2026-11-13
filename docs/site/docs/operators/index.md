# Operators

Bring-up, hosted internals, qualification, and program porting for Paxeer X
Network. Paxeer mainnet-beta is one BFT chain, run by `paxd`, with EVM chain ID
`125` (`0x7d`). The kernel is part of this network and proposes blocks. A
receipt is final when its block commits.

Public JSON-RPC uses the node root URLs, including
`https://api-mainnet-beta.paxeer.network` and
`https://api1.mainnet-beta.paxeer.network`; see [Public RPC endpoints](../reference/public-rpc.md)
for the numbered nodes. Hosted gateway `px_` methods have their own declared
contract in `services/platform/hosted/gateway/openrpc.json`; their availability
does not follow from public node availability. Sub-chains are tenant domains
in the database, rather than additional chains to bring up.

| Topic | Page |
| --- | --- |
| Disposable beta cluster | [Beta cluster](beta-cluster.md) |
| Control and journey admission | [Control](beta-control.md) |
| Internal KMS and event sources | [Hosted internal](hosted-internal.md) |
| Historical checkpoint tooling (retired) | [Guarantor notes](guarantor.md) |
| Evidence publication tooling | [Settlement evidence](settlement-evidence.md) |
| Qualification gates | [Qualification](qualification.md) |
| Program porting kits | [Porting](porting.md) |

The checkpoint producer role and the execution-on-one-network,
settlement-on-another model are retired. Historical operator runbooks do not
define a second finality step for the merged network.

Mainnet-beta handles real value. Hosted applications require their configured
services, credentials and admission checks. A local `make ci` result covers
the native suites listed in [Qualification](qualification.md); deployment
readiness also requires the intended node and application paths. See
[Porting](porting.md#network-and-deployment-boundary) for the current Programs
deployment boundary.
