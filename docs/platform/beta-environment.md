# Beta environments

Paxeer mainnet-beta runs on chain 125 (`0x7d`). The kernel is part of the
network, and a transaction is final when its block commits. Public node
JSON-RPC uses the node root path; connection details are in the
[public RPC guide](../site/docs/reference/public-rpc.md).

## Mainnet-beta

Fund accounts through chain transfers on chain 125. Hosted application
services use separately configured origins. The
[mainnet-beta contract](content/beta.md) describes the network and application
surfaces. A local emulator or private test balance is not a mainnet balance.

## Private test environment

`services/platform/hosted/testnet` supplies private control and status
endpoints. Its network identifier is 402 and its EVM chain identifier is 125;
these are distinct fields. Package semantic version comes from the package
manifest, and LXP wire version comes from the wire crate. `GET /v1/parameters`
reports both independently.

The private reset schedule is 09:00 UTC on the first Tuesday of each month;
[beta-resets.ics](beta-resets.ics) carries the calendar. Balances and receipts
in that environment are disposable across a reset. Resetting it is an
authenticated operation on its private control service.

The private faucet requires bearer authentication, a unique `Idempotency-Key`
and the destination identity. Quotas apply to identity, destination and client
network. An indeterminate funding response stays `still_checking` under the
same key. A `429` response carries `Retry-After`.

`GET /v1/status` reports `testnet`, `gateway`, `core` and `paxeer` separately.
`GET /readyz` reports readiness. The supported journey routes are
`/v1/journeys/funding`, `/v1/journeys/payment` and
`/v1/journeys/receipt-inspection`.

Obtain the private HTTPS origins and authentication from the environment
operator. Public mainnet node endpoints do not serve private funding or reset
operations.
