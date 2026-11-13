# Explorer backend devcontainer

The devcontainer runs the Blockscout-based explorer backend with an Elixir
container and PostgreSQL 17. Its Compose definition is
[`docker-compose.yml`](docker-compose.yml); VS Code settings are in
[`devcontainer.json`](devcontainer.json).

## Opening the backend

From the repository root, open the backend directory in VS Code:

```sh
code services/explorer/backend
```

Select **Dev Containers: Reopen in Container**. Compose mounts the backend at
`/workspace`, uses the `vscode` user and adds `.devcontainer/bin` to `PATH`.
The post-create commands fetch Mix dependencies and configure Git access.
The configured extensions include ElixirLS and a PostgreSQL client.

The Elixir container uses the database container's network namespace. VS Code
forwards ports 4000, 4001 and 5432. The database persists in the
`postgres-data` volume.

## Database access

The database service is named `db`, listens on port 5432 and creates database
`app`. Use the local connection settings in
[`docker-compose.yml`](docker-compose.yml) and the `DATABASE_URL` consumed by
[`bin/bs`](bin/bs). The helper supplies its development database URL when that
variable is unset.

For a VS Code database connection inside the devcontainer, select database
`app` on `db:5432` and use the credentials configured for that local service.

## Backend configuration

Run backend commands from `/workspace` inside the container.
The `bs` helper loads `.devcontainer/.blockscout_config` when present and
exports the variables it contains. Create that file with the settings required
by your local setup, such as:

```sh
CHAIN_TYPE=paxeer_x
ETHEREUM_JSONRPC_VARIANT=paxeer_x
```

Set `ETHEREUM_JSONRPC_HTTP_URL` to the root JSON-RPC URL of the node to index.
Paxeer mainnet-beta uses EVM chain ID 125. See the
[explorer overview](../../README.md#network-and-interfaces) and
[environment variable reference](https://docs.blockscout.com/setup/env-variables).

## The bs helper

[`bin/bs`](bin/bs) exposes these commands:

| Command | Behavior |
| --- | --- |
| `bs --init` | Compile dependencies and the application, then generate local Phoenix certificates if absent |
| `bs --db-init` | Drop the database, create it and run migrations; removes its existing data |
| `bs` | Start the Phoenix server with indexing enabled |
| `bs --no-sync` | Start the server with indexing disabled |
| `bs --compile` | Compile the application |
| `bs --recompile` | Clean umbrella application dependencies, fetch dependencies and recompile them |
| `bs --spellcheck` | Run CSpell |
| `bs --dialyzer` | Run Dialyzer |
| `bs --credo` | Run Credo |
| `bs --format` | Format Elixir files |
| `bs --docs` | Generate documentation |
| `bs --help` | List options |

Initialize the application and database before starting the server. With the
local HTTP listener on port 4000, the blocks API is available at:

```sh
curl http://localhost:4000/api/v2/blocks
```

The HTTP API is separate from node JSON-RPC.

## Elixir version

Compose selects the prebuilt `ghcr.io/blockscout/devcontainer-elixir` image
and its Elixir/Erlang tag. Change the image tag in
[`docker-compose.yml`](docker-compose.yml) to select a different prebuilt
version, then rebuild the container.

For a local image build, enable that service's commented `build` section and
set `VARIANT` in [`Dockerfile`](Dockerfile) to the desired compatible base image.
The Dockerfile also declares the Phoenix and Node versions used by that build.

## Chain-specific checks

[`bin/chain-specific-checks`](bin/chain-specific-checks) selects checks by
`CHAIN_TYPE`. The `bs` helper sources it before starting Phoenix.
