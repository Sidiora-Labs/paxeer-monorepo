<!--
order: 9
-->

# Client

## CLI

The examples use a local node with a validator signing record. Obtain its
consensus address from the signing-info query before using the gRPC and REST
commands:

```bash
CONS_ADDRESS=$(paxd query slashing signing-info "$(paxd tendermint show-validator)" \
  --output json | jq -er '.address')
```

Consensus addresses use the `paxvalcons` prefix configured in
[`node/params/config.go`](../../../../node/params/config.go). Output examples
below show illustrative values; `<consensus address>` denotes the queried
address. Parameter values depend on the selected node and height.

A user can query and interact with the `slashing` module using the CLI.

### Query

The `query` commands allow users to query `slashing` state.

```bash
paxd query slashing --help
```

#### params

The `params` command allows users to query current parameters for the slashing module.

```bash
paxd query slashing params [flags]
```

Example:

```bash
paxd query slashing params
```

Example Output:

```yaml
downtime_jail_duration: 600s
min_signed_per_window: "0.500000000000000000"
signed_blocks_window: "100"
slash_fraction_double_sign: "0.050000000000000000"
slash_fraction_downtime: "0.010000000000000000"
```

#### signing-info

The `signing-info` command allows users to query signing-info of the validator using consensus public key.

```bash
paxd query slashing signing-info [validator-conspub] [flags]
```

Example:

```bash
paxd query slashing signing-info "$(paxd tendermint show-validator)"

```

Example Output:

```yaml
address: <consensus address>
index_offset: "2068"
jailed_until: "1970-01-01T00:00:00Z"
missed_blocks_counter: "0"
start_height: "0"
tombstoned: false
```

#### signing-infos

The `signing-infos` command allows users to query signing infos of all validators.

```bash
paxd query slashing signing-infos [flags]
```

Example:

```bash
paxd query slashing signing-infos
```

Example Output:

```yaml
info:
- address: <consensus address>
  index_offset: "2075"
  jailed_until: "1970-01-01T00:00:00Z"
  missed_blocks_counter: "0"
  start_height: "0"
  tombstoned: false
pagination:
  next_key: null
  total: "0"
```

### Transactions

The `tx` commands allow users to interact with the `slashing` module.

```bash
paxd tx slashing --help
```

#### unjail

The `unjail` command allows users to unjail a validator previously jailed for downtime.

```bash
paxd tx slashing unjail --from mykey [flags]
```

Example:

```bash
paxd tx slashing unjail --from mykey
```

## gRPC

A user can query the `slashing` module using gRPC endpoints.

### Params

The `Params` endpoint allows users to query the parameters of slashing module.

```text
cosmos.slashing.v1beta1.Query/Params
```

Example:

```bash
grpcurl -plaintext localhost:9090 cosmos.slashing.v1beta1.Query/Params
```

Example Output:

```json
{
  "params": {
    "signedBlocksWindow": "100",
    "minSignedPerWindow": "NTAwMDAwMDAwMDAwMDAwMDAw",
    "downtimeJailDuration": "600s",
    "slashFractionDoubleSign": "NTAwMDAwMDAwMDAwMDAwMDA=",
    "slashFractionDowntime": "MTAwMDAwMDAwMDAwMDAwMDA="
  }
}
```

### SigningInfo

The SigningInfo queries the signing info of given cons address.

```text
cosmos.slashing.v1beta1.Query/SigningInfo
```

Example:

```bash
grpcurl -plaintext -d "{\"cons_address\":\"$CONS_ADDRESS\"}" localhost:9090 cosmos.slashing.v1beta1.Query/SigningInfo
```

Example Output:

```json
{
  "valSigningInfo": {
    "address": "<consensus address>",
    "indexOffset": "3493",
    "jailedUntil": "1970-01-01T00:00:00Z"
  }
}
```

### SigningInfos

The SigningInfos queries signing info of all validators.

```text
cosmos.slashing.v1beta1.Query/SigningInfos
```

Example:

```bash
grpcurl -plaintext localhost:9090 cosmos.slashing.v1beta1.Query/SigningInfos
```

Example Output:

```json
{
  "info": [
    {
      "address": "<consensus address>",
      "indexOffset": "2467",
      "jailedUntil": "1970-01-01T00:00:00Z"
    }
  ],
  "pagination": {
    "total": "1"
  }
}
```

## REST

A user can query the `slashing` module using REST endpoints.

### Params

```text
/cosmos/slashing/v1beta1/params
```

Example:

```bash
curl "localhost:1317/cosmos/slashing/v1beta1/params"
```

Example Output:

```json
{
  "params": {
    "signed_blocks_window": "100",
    "min_signed_per_window": "0.500000000000000000",
    "downtime_jail_duration": "600s",
    "slash_fraction_double_sign": "0.050000000000000000",
    "slash_fraction_downtime": "0.010000000000000000"
  }
}
```

### signing_info

```text
/cosmos/slashing/v1beta1/signing_infos/%s
```

Example:

```bash
curl "localhost:1317/cosmos/slashing/v1beta1/signing_infos/$CONS_ADDRESS"
```

Example Output:

```json
{
  "val_signing_info": {
    "address": "<consensus address>",
    "start_height": "0",
    "index_offset": "4184",
    "jailed_until": "1970-01-01T00:00:00Z",
    "tombstoned": false,
    "missed_blocks_counter": "0"
  }
}
```

### signing_infos

```text
/cosmos/slashing/v1beta1/signing_infos
```

Example:

```bash
curl "localhost:1317/cosmos/slashing/v1beta1/signing_infos"
```

Example Output:

```json
{
  "info": [
    {
      "address": "<consensus address>",
      "start_height": "0",
      "index_offset": "4169",
      "jailed_until": "1970-01-01T00:00:00Z",
      "tombstoned": false,
      "missed_blocks_counter": "0"
    }
  ],
  "pagination": {
    "next_key": null,
    "total": "1"
  }
}
```
