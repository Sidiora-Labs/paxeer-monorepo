# Release v6.12.0 contracts

These contracts describe the native market interfaces, application and data services of release v6.12.0.

## Contract files

- [README.md](README.md): this index, the file list and the reading order.
- [address-table.md](address-table.md): every precompile address, module account and token of the release, with constructor, module, activation rule and owning contract file.
- [pricefeed.md](pricefeed.md): the pricefeed module and the precompile at 0x1020, its markets, feed keys, submission message, aggregation, read API, activation and price sidecar.
- [perps.md](perps.md): the perpetuals precompile interface at 0x1015, its units, events, storage, activation and versioning and EndBlock.
- [perps-parameters.md](perps-parameters.md): the perpetuals parameter sheet, trading fees, margin, funding, liquidation, insurance fund and the spot fee schedule.
- [perps-markets.md](perps-markets.md): the perpetuals markets at activation.
- [spot.md](spot.md): the spot venue at 0x1021 to 0x1025, its address set, token model, opening pairs, fees, EndBlock execution, activation and errors.
- [launchpad.md](launchpad.md): the launchpad precompile at 0x1017, its interface by role, surfaces, quote asset, migration record, optical hooks, storage and activation.
- [application.md](application.md): the application on the v6.12.0 precompiles, its address table, amounts and error handling.
- [data-services.md](data-services.md): the indexer, stream, event decoding, websocket protocol, GraphQL operations, explorer routes, portal pages and service layout.
- [release.md](release.md): the release build and plan, release artifacts, price sidecar, upgrade rehearsal, rollout, cut-over, rollback, post-rollout verification, operator inputs and name retirement.
- [../docs/rollout-runbook.md](../docs/rollout-runbook.md): the rollout runbook and its operator inputs.

## Address table

address-table.md is the single list of addresses, module accounts and tokens. The table links each address, account and denom to its owning contract and implementation.

## Reading order

1. [README.md](README.md)
2. [address-table.md](address-table.md)
3. [pricefeed.md](pricefeed.md)
4. [perps.md](perps.md)
5. [perps-parameters.md](perps-parameters.md)
6. [perps-markets.md](perps-markets.md)
7. [spot.md](spot.md)
8. [launchpad.md](launchpad.md)
9. [application.md](application.md)
10. [data-services.md](data-services.md)
11. [release.md](release.md)
12. [../docs/rollout-runbook.md](../docs/rollout-runbook.md)
