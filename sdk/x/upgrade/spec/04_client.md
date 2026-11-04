<!--
order: 4
-->

# Client

The `upgrade` module exposes the scheduled plan, applied upgrade heights and
stored module consensus versions for the Paxeer node. These queries read the
state of the same network that executes transactions and commits blocks. See the
[upgrade overview](README.md) and [state layout](02_state.md) for context.

The examples below use a local `paxd` node. The CLI uses its configured consensus
RPC connection; REST requires the node's SDK API server, and gRPC requires its
gRPC server. The local REST and gRPC addresses shown here are examples, not public
mainnet endpoints. In `app.toml`, the relevant settings are `api.enable`,
`api.address`, `grpc.enable` and `grpc.address`.

Public Ethereum JSON-RPC is available at the root of the mainnet node URLs, such
as `https://api1.mainnet-beta.paxeer.network`, with EVM chain ID `125` (`0x7d`).
The Cosmos REST paths and gRPC service names below belong to the SDK query
interfaces; configure an SDK query connection to use them.

All outputs below are illustrative, not a snapshot of deployed upgrade state.
The plan examples assume a scheduled plan named `test-upgrade` at height `130`
with empty `info`. The applied-plan examples assume that name was already applied
at height `30` in a separate example. Version examples assume a stored `upgrade`
module consensus version of `1`; query the node for its actual version map.

## CLI

Use `paxd query upgrade` to inspect the module:

```bash
paxd query upgrade --help
```

### Query

#### applied

The `applied` command looks up the named upgrade's recorded height and then
fetches the block metadata at that height using the configured consensus RPC
node:

```bash
paxd query upgrade applied test-upgrade
```

The output is JSON block metadata, including `block_id`, `block_size`, `header`
and `num_txs`. It is not the height-only response returned by REST and gRPC. The
header describes the block in which the upgrade was applied, which helps locate
past migrations. Retrieving it also requires the node to retain that block's
metadata.

If the module has no recorded applied height for that name, the CLI returns
`no upgrade found`. If the block lookup returns no metadata, it returns
`no headers returned for height <height>`; connection and lookup errors are also
returned to the caller.

#### module versions

The `module_versions` command returns the stored module names and their consensus
versions. With no argument it returns all stored entries; with a module name it
returns only that entry:

```bash
paxd query upgrade module_versions --output json
paxd query upgrade module_versions upgrade --output json
```

Illustrative output for the second command:

```json
{
  "module_versions": [
    {
      "name": "upgrade",
      "version": "1"
    }
  ]
}
```

These are module consensus versions, not node software release versions. An
unknown module name returns a not-found error rather than an empty matching
entry.

#### plan

The `plan` command returns the currently scheduled plan:

```bash
paxd query upgrade plan --output json
```

Unlike REST and gRPC, the CLI prints the `Plan` directly, without a `plan` wrapper:

```json
{
  "name": "test-upgrade",
  "time": "0001-01-01T00:00:00Z",
  "height": "130",
  "info": "",
  "upgraded_client_state": null
}
```

For text output, the CLI converts the protobuf JSON to YAML:

```bash
paxd query upgrade plan --output text
```

```yaml
height: "130"
info: ""
name: test-upgrade
time: "0001-01-01T00:00:00Z"
upgraded_client_state: null
```

A plan has a nonempty `name`, a positive `height` and application-specific `info`.
The `time` and `upgraded_client_state` fields remain in the wire format for
compatibility, but time-based upgrades and setting an upgraded client state on
this plan are rejected by `Plan.ValidateBasic`. The zero timestamp and null
client state above are unset values, not scheduling instructions.

When no plan is scheduled, the CLI exits with `no upgrade scheduled`. It does not
print an empty plan or a bare upgrade name.

## REST

These GET routes are registered through the SDK gRPC gateway. The examples assume
the SDK API server is enabled at `http://localhost:1317`. Its marshaler uses
protobuf field names such as `module_versions` and emits default values. Protobuf
64-bit integer fields, including heights and versions, are JSON strings.

### Applied Plan

`AppliedPlan` queries the recorded height for an upgrade name:

```text
/cosmos/upgrade/v1beta1/applied_plan/{name}
```

```bash
curl --fail --silent --show-error \
  'http://localhost:1317/cosmos/upgrade/v1beta1/applied_plan/test-upgrade'
```

```json
{
  "height": "30"
}
```

An unknown or unapplied name is a successful query with `{"height":"0"}`. This
endpoint does not fetch a block header.

### Current Plan

`CurrentPlan` returns an object containing the scheduled `Plan`:

```text
/cosmos/upgrade/v1beta1/current_plan
```

```bash
curl --fail --silent --show-error \
  'http://localhost:1317/cosmos/upgrade/v1beta1/current_plan'
```

```json
{
  "plan": {
    "name": "test-upgrade",
    "time": "0001-01-01T00:00:00Z",
    "height": "130",
    "info": "",
    "upgraded_client_state": null
  }
}
```

When no plan is scheduled, the keeper returns a successful response with no
`Plan`. With the node's default gateway marshaler, this is:

```json
{
  "plan": null
}
```

### Module versions

`ModuleVersions` returns stored module consensus versions. An optional
`module_name` query parameter selects a single module:

```text
/cosmos/upgrade/v1beta1/module_versions
```

```bash
curl --fail --silent --show-error \
  'http://localhost:1317/cosmos/upgrade/v1beta1/module_versions'
curl --fail --silent --show-error \
  'http://localhost:1317/cosmos/upgrade/v1beta1/module_versions?module_name=upgrade'
```

Illustrative output for the filtered request:

```json
{
  "module_versions": [
    {
      "name": "upgrade",
      "version": "1"
    }
  ]
}
```

If a requested module is absent, the keeper returns a not-found error containing
`x/upgrade: QueryModuleVersions module <name> not found`. An unfiltered query
returns the complete stored list, which can be empty.

## gRPC

The service is `cosmos.upgrade.v1beta1.Query`, registered by the upgrade module.
The commands below assume a local plaintext gRPC server at `localhost:9090` with
reflection available. Use the transport and address configured for your node.

`grpcurl` renders protobuf JSON with lower camel case field names by default,
whereas the REST gateway uses the original snake case names. The commands below
use `-emit-defaults` to make unset fields visible. Heights and versions remain
JSON strings.

### Applied Plan

```text
cosmos.upgrade.v1beta1.Query/AppliedPlan
```

```bash
grpcurl -plaintext -emit-defaults \
  -d '{"name":"test-upgrade"}' \
  localhost:9090 cosmos.upgrade.v1beta1.Query/AppliedPlan
```

```json
{
  "height": "30"
}
```

As with REST, an unapplied name returns height `"0"` successfully; this method
returns no block metadata.

### Current Plan

```text
cosmos.upgrade.v1beta1.Query/CurrentPlan
```

```bash
grpcurl -plaintext -emit-defaults -d '{}' \
  localhost:9090 cosmos.upgrade.v1beta1.Query/CurrentPlan
```

```json
{
  "plan": {
    "name": "test-upgrade",
    "time": "0001-01-01T00:00:00Z",
    "height": "130",
    "info": "",
    "upgradedClientState": null
  }
}
```

When no plan is scheduled, the RPC succeeds with an unset `plan` field. With
`-emit-defaults`, `grpcurl` renders `{"plan":null}`; without that option it omits
the field and renders `{}`. The CLI translates this same absence into its
`no upgrade scheduled` error.

### Module versions

```text
cosmos.upgrade.v1beta1.Query/ModuleVersions
```

Query all stored modules with an empty request, or supply `module_name` to filter:

```bash
grpcurl -plaintext -emit-defaults -d '{}' \
  localhost:9090 cosmos.upgrade.v1beta1.Query/ModuleVersions
grpcurl -plaintext -emit-defaults -d '{"module_name":"upgrade"}' \
  localhost:9090 cosmos.upgrade.v1beta1.Query/ModuleVersions
```

Illustrative output for the filtered request:

```json
{
  "moduleVersions": [
    {
      "name": "upgrade",
      "version": "1"
    }
  ]
}
```

A missing requested module returns the same keeper not-found error described
above. An unfiltered query with no stored versions succeeds with an empty list.

The wire contracts are defined in
[`query.proto`](../../../proto/cosmos/upgrade/v1beta1/query.proto) and
[`upgrade.proto`](../../../proto/cosmos/upgrade/v1beta1/upgrade.proto).
[`grpc_query.go`](../keeper/grpc_query.go) implements the state queries,
[`query.go`](../client/cli/query.go) defines the CLI's additional behavior, and
[`module.go`](../module.go) registers the query service and gateway routes.
