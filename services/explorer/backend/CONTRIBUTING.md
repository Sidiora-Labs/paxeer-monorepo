# Backend development reference

The backend is an Elixir umbrella application. The [backend overview](README.md)
describes its applications, native Paxeer APIs and build commands. Runtime
versions are pinned in [.tool-versions](.tool-versions).

## Source layout

| Application | Responsibility |
| --- | --- |
| [EthereumJSONRPC](apps/ethereum_jsonrpc/README.md) | Node transports and response normalization |
| [Explorer](apps/explorer/README.md) | PostgreSQL schemas, chain queries, imports and migrations |
| [Indexer](apps/indexer/README.md) | Block discovery, fetchers and transformations |
| [BlockScoutWeb](apps/block_scout_web/README.md) | Routers, controllers, views and channels |

Each application's `test/` directory contains its ExUnit tests. The account
HTTP contract is documented in [API.md](apps/block_scout_web/API.md), and
[rate_limits.md](apps/block_scout_web/rate_limits.md) describes endpoint limits.

## Configuration

[config/runtime.exs](config/runtime.exs) reads runtime environment variables.
[config/config_helper.exs](config/config_helper.exs) defines `ConfigHelper`,
including `repos/0`, URL-list parsing and duration parsing. Chain profiles
under [apps/explorer/config](apps/explorer/config) configure node variants
and chain-specific repositories.

[Utils.RuntimeEnvHelper](apps/utils/lib/utils/runtime_env_helper.ex) generates
functions that read application configuration at runtime.
[Utils.CompileTimeEnvHelper](apps/utils/lib/utils/compile_time_env_helper.ex)
reads configuration into module attributes during compilation. Ecto schema
fields are defined when their modules compile.

[ApiRouter](apps/block_scout_web/lib/block_scout_web/routers/api_router.ex)
registers API routes. The
[CheckFeature plug](apps/block_scout_web/lib/block_scout_web/plug/check_feature.ex)
checks configured feature availability before an action runs.

Database migration files live in
[apps/explorer/priv/repo/migrations](apps/explorer/priv/repo/migrations) and
chain-specific directories under [apps/explorer/priv](apps/explorer/priv).
The [heavy index operation reference](apps/explorer/lib/explorer/migrator/heavy_db_index_operation/README.md)
describes the background index framework and a concrete implementation.
