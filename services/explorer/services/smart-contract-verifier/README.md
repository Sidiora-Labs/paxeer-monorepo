# Smart-contract verifier

**Smart-contract verifier** is a service for verifying EVM-based contracts. The primary function of this service is to receive bytecode and potential source files as inputs and determine whether the files and the bytecode correspond to each other.

This service serves as the core component for all activities related to smart-contract verification in BlockScout. It is essential for enabling smart-contract verification functionality on your instance.

## Requirements

Source builds use the Rust workspace in this directory. The running service
needs writable compiler caches and access to the enabled compiler inventories;
Sourcify verification uses its configured external API. It does not require a
database. See the [server configuration](smart-contract-verifier-server/README.md#configuration)
for enabled services and compiler fetchers.

## How to enable
Set the following ENVs on blockscout instance:
- `MICROSERVICE_SC_VERIFIER_ENABLED=true`
- `MICROSERVICE_SC_VERIFIER_URL={service_url}`
- `MICROSERVICE_SC_VERIFIER_TYPE=sc_verifier`

## Envs
Here, we describe variables specific to this service. Variables common to all services (server, metrics, tracing) are defined in the registry dependency [blockscout-service-launcher 0.17.0](https://docs.rs/blockscout-service-launcher/0.17.0/blockscout_service_launcher/).

[anchor]: <> (anchors.envs.start)

| Variable                                                       | Required | Description                                                             | Default value                                                                |
|----------------------------------------------------------------|----------|-------------------------------------------------------------------------|------------------------------------------------------------------------------|
| `SMART_CONTRACT_VERIFIER__SOLIDITY__ENABLED`                   |          | Enable Solidity verification endpoints                                  | `true`                                                                       |
| `SMART_CONTRACT_VERIFIER__SOLIDITY__FETCHER__LIST__LIST_URL`   |          | URL containing the list of available Solidity compilers                   | `https://binaries.soliditylang.org/linux-amd64/list.json`                    |
| `SMART_CONTRACT_VERIFIER__SOLIDITY__REFRESH_VERSIONS_SCHEDULE` |          | Cron-format schedule to update the list of available Solidity compilers | `0 0 * * * * *`                                                              |
| `SMART_CONTRACT_VERIFIER__SOLIDITY__COMPILERS_DIR`             |          | Directory where Solidity compilers will be downloaded                   | `/tmp/solidity-compilers`                                                    |
| `SMART_CONTRACT_VERIFIER__VYPER__ENABLED`                      |          | Enable Vyper verification endpoints                                     | `true`                                                                       |
| `SMART_CONTRACT_VERIFIER__VYPER__FETCHER__LIST__LIST_URL`      |          | URL containing the list of available Vyper compilers                   | `https://raw.githubusercontent.com/blockscout/solc-bin/main/vyper.list.json` |
| `SMART_CONTRACT_VERIFIER__VYPER__REFRESH_VERSIONS_SCHEDULE`    |          | Cron-format schedule to update the list of available Vyper compilers    | `0 0 * * * * *`                                                              |
| `SMART_CONTRACT_VERIFIER__VYPER__COMPILERS_DIR`                |          | Directory where Vyper compilers will be downloaded                      | `/tmp/vyper-compilers`                                                       |
| `SMART_CONTRACT_VERIFIER__SOURCIFY__ENABLED`                   |          | Enable Sourcify verification endpoints                                    | `true`                                                                       |
| `SMART_CONTRACT_VERIFIER__SOURCIFY__API_URL`                   |          | Sourcify API url                                                        | `https://sourcify.dev/server/`                                               |
| `SMART_CONTRACT_VERIFIER__SOURCIFY__VERIFICATION_ATTEMPTS`     |          | Number of attempts the server makes to Sourcify API. Must be at least 1 | `3`                                                                          |
| `SMART_CONTRACT_VERIFIER__SOURCIFY__REQUEST_TIMEOUT`           |          | Timeout in seconds for a single request to Sourcify API                 | `15`                                                                         |
| `SMART_CONTRACT_VERIFIER__COMPILERS__MAX_THREADS`              |          | Maximum number of concurrent compilations                               | number of available CPU threads                                              |

[anchor]: <> (anchors.envs.end)

ZkSync Solidity verification is also enabled by default through
`SMART_CONTRACT_VERIFIER__ZKSYNC_SOLIDITY__ENABLED`. Its three compiler inventories
and tracing settings are described in the [server guide](smart-contract-verifier-server/README.md#configuration).

## In this repository
The service is built from `docker/explorer-smart-contract-verifier/Dockerfile` at the repository root and published by `.github/workflows/explorer-images.yml` as `ghcr.io/sidiora-labs/paxeer-x-explorer-smart-contract-verifier`. `cargo check` in this directory builds it against the shared crates in `../libs`; [the explorer README](../../README.md) covers the lint script and the local stack.

## Upstream links
- Demo - https://http.sc-verifier.services.blockscout.com
- [Swagger](https://blockscout.github.io/swaggers/services/smart-contract-verifier/index.html)
- [Packages](https://github.com/blockscout/blockscout-rs/pkgs/container/smart-contract-verifier)
- [Releases](https://github.com/blockscout/blockscout-rs/releases?q=smart-contract-verifier&expanded=true)