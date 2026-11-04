# Client

The `evidence` module queries stored misbehavior evidence on the merged `paxd`
network. Its [Query service](../../../proto/cosmos/evidence/v1beta1/query.proto)
returns protobuf `Any` values, preserving the concrete evidence type. The
[atlas integration guide](../atlas/atlas-v0.41.x.md) describes module setup and
consensus processing.

The examples below use a local node: consensus RPC for CLI queries, an enabled
SDK HTTP API on `localhost:1317`, and gRPC on `localhost:9090`. Use the addresses
configured for your node. Public EVM JSON-RPC is served at the root of the public
node URLs on chain ID `125`; these SDK API paths are served by the SDK API
listener.

## Hash encoding

All transports identify the same raw digest, but their text encodings differ:

| Transport | Input | Encoding |
| --- | --- | --- |
| CLI | Positional hash | Hex, without `0x` |
| Legacy REST | `/evidence/{hash}` | Hex, without `0x` |
| Generated REST gateway | `/cosmos/evidence/v1beta1/evidence/{evidence_hash}` | Base64 bytes, URL-escaped as needed |
| gRPC binary | `QueryEvidenceRequest.evidence_hash` | Raw bytes |
| gRPC protobuf JSON (`grpcurl`) | `evidence_hash` / `evidenceHash` | Base64 bytes |

The CLI calls `hex.DecodeString` in
[client/cli/query.go](../client/cli/query.go). The legacy REST handler also decodes
hex, whereas the [generated gateway](../types/query.pb.gw.go) calls
`runtime.Bytes` to decode base64. The protobuf field's Go `HexBytes` cast does
not change that gateway conversion or protobuf JSON's bytes encoding.

For the illustrative digest used in these commands:

```text
hex:    DF0C23E8634E480F84B9D5674A7CDC9816466DEC28A3358F73260F68D28D7660
base64: 3wwj6GNOSA+EudVnSnzcmBZGbewoozWPcyYPaNKNdmA=
```

Replace it with a digest stored on your node. These examples do not assert that
this particular record exists. `Equivocation.Hash()` is SHA-256 of the concrete
evidence's protobuf encoding, not the transaction hash.

## CLI

### Query

The query command accepts zero or one positional hash:

```bash
paxd query evidence
```

`GetQueryCmd` declares output, node, height and pagination flags, but also sets
`DisableFlagParsing: true`. Its handler does not parse those flags explicitly,
so flag arguments can be treated as the positional hash. The examples here use
only positional arguments; use REST or gRPC for explicit pagination and JSON
responses until the command wiring parses its declared flags.

### evidence

A positional hash selects one record:

```bash
paxd query evidence DF0C23E8634E480F84B9D5674A7CDC9816466DEC28A3358F73260F68D28D7660
```

The CLI prints `QueryEvidenceResponse.Evidence` directly. When the client output
format is JSON, its output is the expanded `Any` object, with no outer
`evidence` field. For a stored
`Equivocation`, the shape is as follows (illustrative values, independent of the
digest above):

```json
{
  "@type": "/cosmos.evidence.v1beta1.Equivocation",
  "height": "100",
  "time": "2006-01-02T15:04:05Z",
  "power": "1000000",
  "consensus_address": "paxvalcons1vehk7h6lta047h6lta047h6lta047h6l4d56wf"
}
```

Without a hash, the CLI prints the complete paginated `QueryAllEvidenceResponse`:

```bash
paxd query evidence
```

With JSON output configured and total counting requested by an embedding client,
a one-record response with no further page has this shape:

```json
{
  "evidence": [
    {
      "@type": "/cosmos.evidence.v1beta1.Equivocation",
      "height": "100",
      "time": "2006-01-02T15:04:05Z",
      "power": "1000000",
      "consensus_address": "paxvalcons1vehk7h6lta047h6lta047h6lta047h6l4d56wf"
    }
  ],
  "pagination": {
    "next_key": null,
    "total": "1"
  }
}
```

`evidence` is always a list in the all-evidence response. An empty result has
`"evidence": []`. A total of zero does not establish an empty store unless total
counting was requested. Default text output is YAML converted from the same JSON
representation, preserving the list and `@type` discriminator.

There is a separate continuation-key limitation: [ReadPageRequest](../../../client/utils.go)
copies the `--page-key` string directly to bytes. It does not decode a returned
base64 `pagination.next_key`, so passing that printed value back unchanged would
select a different byte key even if flag parsing were enabled. Use the REST or
gRPC continuation examples below.

## REST

The SDK API's generated gateway exposes queries through the same Query service.
These routes do not submit transactions.

### Evidence

Get evidence by hash using the base64 representation in the generated route.
The `+` in this digest is percent-encoded below:

```bash
curl 'http://localhost:1317/cosmos/evidence/v1beta1/evidence/3wwj6GNOSA%2BEudVnSnzcmBZGbewoozWPcyYPaNKNdmA='
```

Unlike the single-record CLI output, this response wraps the expanded `Any` in
an `evidence` field:

```json
{
  "evidence": {
    "@type": "/cosmos.evidence.v1beta1.Equivocation",
    "height": "100",
    "time": "2006-01-02T15:04:05Z",
    "power": "1000000",
    "consensus_address": "paxvalcons1vehk7h6lta047h6lta047h6lta047h6l4d56wf"
  }
}
```

The values shown illustrate a successful response shape. A missing digest
returns a query error instead. If using the legacy `/evidence/{hash}` route,
supply hex; its legacy response format is distinct from this gateway response.

### All evidence

```bash
curl 'http://localhost:1317/cosmos/evidence/v1beta1/evidence?pagination.limit=50&pagination.count_total=true'
```

This returns the same `evidence` array and `pagination` structure shown for the
all-evidence CLI JSON output. To continue, send the returned base64
`pagination.next_key` as the `pagination.key` query parameter, URL-encoding its
value. The SDK gateway preserves protobuf field names such as
`consensus_address` and `next_key`, and emits default fields.

## gRPC

### Evidence

The method is `cosmos.evidence.v1beta1.Query/Evidence`. In `grpcurl`'s protobuf
JSON request, bytes must be base64:

```bash
grpcurl -plaintext \
  -d '{"evidence_hash":"3wwj6GNOSA+EudVnSnzcmBZGbewoozWPcyYPaNKNdmA="}' \
  localhost:9090 cosmos.evidence.v1beta1.Query/Evidence
```

The method returns `QueryEvidenceResponse`, including the outer `evidence`
field and the expanded `Any` with its `@type`. `grpcurl` normally prints protobuf
JSON names in lower camel case, such as `consensusAddress`, while the SDK CLI and
REST gateway use original names such as `consensus_address`. The `int64` fields
`height` and `power` are decimal strings, and `time` is a protobuf timestamp in
RFC 3339 format in all these JSON representations.

### All evidence

The method is `cosmos.evidence.v1beta1.Query/AllEvidence`:

```bash
grpcurl -plaintext \
  -d '{"pagination":{"limit":"50","count_total":true}}' \
  localhost:9090 cosmos.evidence.v1beta1.Query/AllEvidence
```

The response contains repeated `Any` values in `evidence` and a `pagination`
object. Protobuf JSON encodes `pagination.total` as a decimal string and its
continuation bytes as base64. `grpcurl` normally uses `nextKey` for that field and
may omit default-valued fields. Use the returned key in `pagination.key` for the
next request. These commands assume the server exposes service descriptors
through reflection; otherwise supply descriptors matching the node's protobufs.

### Query errors

[keeper/grpc_query.go](../keeper/grpc_query.go) returns `InvalidArgument` for a
nil request or nil single-record hash, `NotFound` for a digest absent from the
store, and `Internal` if a stored record cannot be represented as a protobuf
message or packed in `Any`. A malformed hex argument fails CLI decoding before
querying; malformed base64 fails gateway or protobuf JSON decoding.

### Transactions

`cosmos.evidence.v1beta1.Msg/SubmitEvidence` declares an SDK message with a
submitter and evidence `Any`; its response hash is protobuf bytes (base64 in
protobuf JSON). Submit it through a signed SDK transaction and the transaction
broadcast service, rather than treating the Msg method as an unsigned query.

The current node does not initialize the keeper's transaction evidence dispatch
table, and the CLI transaction root does not attach its internally created
`submit` command. Thus these declarations do not establish a working generic
submission path. See the [atlas setup and limitations](../atlas/atlas-v0.41.x.md#usage)
and [message contract](03_messages.md) before integrating submissions. Consensus
misbehavior still enters through the node's block-processing path.
