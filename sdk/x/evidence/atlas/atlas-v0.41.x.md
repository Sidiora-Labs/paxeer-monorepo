# x/evidence

The evidence module stores evidence of validator misbehavior, dispatches submitted
evidence to registered in-process handlers, and processes consensus equivocation
through the staking and slashing keepers. Valid actionable equivocation can
slash, jail and tombstone a validator. Evidence is persisted by its hash and can
be queried individually or with pagination.

**Upstream provenance:** this filename and [atlas manifest](manifest.toml)
retain the Cosmos SDK v0.41 origin; the manifest links identify upstream sources.
The integration details below describe the local fork used by
`paxd`. Paxeer is one BFT network, with EVM chain ID `125`; its kernel participates
in proposing blocks. These evidence operations belong to that node's SDK
application.

## Usage

The current wiring is in [node/app.go](../../../../node/app.go), with consensus
processing in [node/legacyabci/begin_block.go](../../../../node/legacyabci/begin_block.go).
The module depends on the [staking and slashing interfaces](../types/expected_keepers.go).

1. Import the fork packages when integrating the module into an application:

   ```go
   import (
       "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence"
       evidencekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/keeper"
       evidencetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/types"
   )
   ```

2. Include `evidence.AppModuleBasic{}` in the application's `module.BasicManager`
   and its interface/codec registration. `evidence.NewAppModuleBasic(handlers...)`
   can supply client evidence handlers for an embedding application. Registering
   client handlers does not configure keeper-side evidence handling.

3. Add a persistent store with `evidencetypes.StoreKey` to the application's store
   keys and mount it. The node stores the keeper as a value:

   ```go
   EvidenceKeeper evidencekeeper.Keeper
   ```

4. Construct the keeper after staking and slashing are available:

   ```go
   evidenceKeeper := evidencekeeper.NewKeeper(
       appCodec, keys[evidencetypes.StoreKey],
       &app.StakingKeeper, app.SlashingKeeper,
   )
   ```

   `NewKeeper` returns `*evidencekeeper.Keeper`. The application later copies it
   with `app.EvidenceKeeper = *evidenceKeeper` and passes that value to
   `evidence.NewAppModule(app.EvidenceKeeper)`.

5. An embedding application that accepts `MsgSubmitEvidence` must first configure
   the keeper's `types.Router`, an internal evidence-handler dispatch table.
   Register each evidence route with `AddRoute(route, handler)`, then call
   `evidenceKeeper.SetRouter(...)` before copying the keeper into the application
   or module. `SetRouter` seals the table and may be called only once.

   A deliberately empty table can be installed with:

   ```go
   evidenceKeeper.SetRouter(evidencetypes.NewRouter())
   ```

   Such a table rejects new submissions with `ErrNoEvidenceHandlerExists`.
   Supporting a concrete evidence type requires a real handler for its
   `Evidence.Route()` and registration of its protobuf implementation. Registering
   `Equivocation` in the interface registry alone does not install a handler.

   **Current node policy:** `node/app.go` explicitly installs an empty sealed
   router before copying the keeper. The constructor also defaults to a sealed
   empty router. New transaction submissions return `ErrNoEvidenceHandlerExists`
   until an embedding application registers a handler. Consensus equivocation
   processing and stored-evidence queries use separate keeper paths.

6. Add `evidence.NewAppModule(app.EvidenceKeeper)` to `module.NewManager(...)`.
   Its `RegisterServices` registers the evidence Query server and SDK Msg server.
   The node registers the module with its manager and includes
   `evidencetypes.ModuleName` in genesis initialization order.

7. Wire consensus evidence into block processing. In this tree,
   `legacyabci.BeginBlock` calls `evidence.BeginBlocker` after distribution and
   slashing. It handles `DUPLICATE_VOTE` and `LIGHT_CLIENT_ATTACK` misbehavior by
   converting each record to `Equivocation` and calling
   `HandleEquivocationEvidence`. Other misbehavior types are logged and ignored.
   This path does not dispatch through the transaction evidence table.

8. If an application uses SDK simulations, include
   `evidence.NewAppModule(app.EvidenceKeeper)` in its simulation manager and call
   `RegisterStoreDecoders`, as the node does.

### Equivocation handling

[HandleEquivocationEvidence](../keeper/infraction.go) ignores evidence for an
unknown consensus public key, a missing or unbonded validator, or an already
tombstoned validator. It also ignores evidence that exceeds **both** consensus
age limits, `MaxAgeDuration` and `MaxAgeNumBlocks`. Missing signing information
for an otherwise eligible validator causes a panic.

For actionable evidence, the keeper calls the slashing keeper with
`SlashFractionDoubleSign` and the infraction height adjusted by
`ValidatorUpdateDelay`. It jails the validator if necessary, sets the double-sign
jail end time, tombstones it, and persists the evidence. The handler's early
returns do not persist a new evidence record.

## Genesis

The [genesis definition](../../../proto/cosmos/evidence/v1beta1/genesis.proto)
contains a repeated protobuf `Any` field:

```protobuf
message GenesisState {
  repeated google.protobuf.Any evidence = 1;
}
```

The default state contains an empty list. Each entry must unpack to a registered
`exported.Evidence` implementation and pass `ValidateBasic`. Initialization stores
each entry by hash and panics if a hash is duplicated; export repacks stored
evidence as `Any` values. Genesis import stores the records directly without
replaying their punishment handlers.

## Messages

[MsgSubmitEvidence](../spec/03_messages.md) carries a submitter and evidence packed
in `Any`. Its signer is the submitter. The
[keeper](../keeper/keeper.go) rejects duplicate hashes, requires an evidence
handler, executes that handler, emits `submit_evidence`, and persists the record
on success. The response contains the evidence hash as protobuf bytes.

These steps require the keeper setup described under [Usage](#usage). The
[transaction definition](../../../proto/cosmos/evidence/v1beta1/tx.proto) is an SDK
message contract executed within a signed transaction; it is not a public call
that bypasses transaction signing and broadcast.

## Client

See [Client](../spec/07_client.md) for commands, response shapes and hash encoding.

### CLI

The fork's executable is `paxd`:

```sh
paxd query evidence
paxd query evidence 6DE180E498DA2745F6E8243AEB9837A93A36056C8D2788BE51A08DF973583CB4
```

A single positional hash selects one record; no positional hash selects the
paginated list. The CLI decodes hashes as hex without a `0x` prefix. The digest
above is illustrative and may not exist in your node's store. The declared
`--node` option refers to consensus RPC for SDK queries. This command currently
disables flag parsing, and the page-key helper does not base64-decode returned
continuation keys. Use REST or gRPC for explicit pagination; the
[client guide](../spec/07_client.md#cli) describes these limitations.

The current `client/cli/tx.go` creates a submission command internally but does
not attach it to the returned transaction root. There is no usable generic
`paxd tx evidence submit` command in this tree.

### REST

When the node's SDK API is enabled, the gRPC gateway exposes query routes under
`/cosmos/evidence/v1beta1/evidence`. Its single-record path decodes the hash as
base64 bytes. The older `/evidence/{hash}` route uses hexadecimal instead.
See the [transport encoding table](../spec/07_client.md#hash-encoding).

The localhost API and gRPC examples use a node with those services enabled. The
public EVM JSON-RPC nodes serve requests at their root URL; the SDK evidence
routes are not added to that URL by changing the path.

### gRPC

The query service offers `cosmos.evidence.v1beta1.Query/Evidence` and
`cosmos.evidence.v1beta1.Query/AllEvidence`. The first returns one `Any` in an
`evidence` field; the second returns a list of `Any` values and a pagination
response. Protobuf JSON includes the concrete `@type`, and encodes the request
hash bytes as base64.

The Msg service declares `cosmos.evidence.v1beta1.Msg/SubmitEvidence`. Use the SDK
transaction flow for signed submission, and account for the node's current
handler and CLI limitations above.
