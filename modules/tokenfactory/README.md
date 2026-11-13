# Token Factory

The `tokenfactory` module runs inside `paxd` on the merged Paxeer network
(EVM chain ID 125). Any account can create a bank denom named
`factory/{creator address}/{subdenom}`. The creator namespace avoids collisions
between accounts; one account can create multiple denoms with different
subdenoms. Creating a denom does not mint a balance.

The creator becomes the initial admin. Authority metadata also has an optional
`issuer_did`, independent of the admin address:

| Authority | Representation | Capabilities |
| --- | --- | --- |
| Admin | Native bech32 account address | Mint to itself, burn from itself, set bank metadata, replace the transfer allow list, change admin; pause or unpause a denom with no issuer binding. |
| Issuer | `did:layerx:` followed by 64 lowercase hex characters | Use the issuer keeper APIs to mint to or burn from a specified account and control pause state for its bound denom. |

Changing admin preserves `issuer_did` and the original creator namespace. Issuer
binding does not remove the admin's mint, burn, metadata or allow-list powers;
it changes who may control pause state. A validated `MsgChangeAdmin` can set
`new_admin` to an empty string to renounce admin authority. Nonempty replacement
admins must be valid bech32 addresses. Renunciation preserves the issuer binding
and pause state, but permanently removes the admin's powers: a former admin
cannot mint, burn, update metadata or allow lists, change admin, or use the admin
pause message. A bound issuer retains its independent keeper authority.

The wire definitions are in [tx.proto](../../api/tokenfactory/tx.proto),
[query.proto](../../api/tokenfactory/query.proto) and
[authority_metadata.proto](../../api/tokenfactory/authority_metadata.proto).
See [message validation](types/msgs.go) and the
[message server](keeper/msg_server.go) for the transaction checks.

## Messages

All messages carry a native bech32 `sender`, which is their signer. Except for
creation, the messages below require the sender to equal the current admin.

### CreateDenom

`MsgCreateDenom` has `sender`, `subdenom` and optional bank `allow_list` fields.
It returns `new_token_denom` with the full denom string.

Creation validates the full denom, rejects a denom that already has bank
metadata, and currently rejects a subdenom whose name already has bank supply
(the native-denom collision check in [createdenom.go](keeper/createdenom.go)).
It initializes bank metadata with a base denom unit at exponent zero and sets
`base`, `display`, `name` and `symbol` to the full denom. It stores the creator
as admin, leaves the issuer unbound, and indexes the denom under its creator.

When provided, `allow_list` contains an `addresses` array of native bech32
addresses. Its length must not exceed `denom_allowlist_max_size` (default 2000;
query the live params). A nonempty list restricts both senders and recipients
of bank user transfers. An absent or empty list leaves those transfers
unrestricted by the allow list. Pause and other bank policies still apply.

### Mint

`MsgMint` has `sender` and a positive, valid bank `amount` coin. The denom must
have bank metadata, use the factory denom format, and belong to the sender's
admin authority. Minting creates coins in the tokenfactory module account and
sends them to the sender. There is no recipient field on this message.

### Burn

`MsgBurn` has `sender` and a positive, valid bank `amount` coin. It checks the
factory denom and admin authority, transfers coins from the sender to the
tokenfactory module account, and burns them. There is no source-account field
on this message; the sender must hold sufficient spendable coins.

Both mint and burn refuse a paused denom with `ErrDenomPaused`.

### ChangeAdmin

`MsgChangeAdmin` has `sender`, `denom` and `new_admin`. The current admin can
transfer authority to another valid native bech32 address. Selecting the
existing admin returns `ErrAdminAlreadyExists`. The denom name, creator index
and issuer binding remain unchanged. Passing `new_admin: ""` renounces admin
authority. The sender must still be the current admin and a valid native bech32
address; an empty sender cannot reclaim a renounced denom.

### SetDenomMetadata

`MsgSetDenomMetadata` has `sender` and bank `metadata`. The metadata must pass
bank validation; its `base` identifies the factory denom whose admin is
checked. The message replaces that denom's bank metadata.

### UpdateDenom

`MsgUpdateDenom` has `sender`, `denom` and `allow_list`. It requires an existing
denom and a non-nil list, validates the list size and bech32 addresses, then
replaces the bank allow list. Use `{"addresses":[]}` to remove the transfer
restriction; omitting the list returns `ErrAllowListUndefined`.

The CLI accepts `--allow-list path/to/allow-list.json` on creation and requires
it on update. The file is one JSON document such as
`{"addresses":["<native bech32 address>"]}`. The CLI also accepts associated EVM
addresses and resolves them through the EVM address query before constructing
the message; the keeper receives native bech32 addresses.

### SetDenomPaused

`MsgSetDenomPaused` has `sender`, `denom` and boolean `paused`. It validates the
factory denom, requires bank metadata and current admin authority, and refuses
any issuer-bound denom with `ErrIssuerMismatch`. This refusal also applies if
the admin controls the issuer identity: the admin message cannot change an
issuer-bound denom's pause state.

For an unbound denom, `paused: true` pauses and `paused: false` unpauses.
Requesting the existing value returns `ErrPauseUnchanged`; a newly created
denom starts unpaused. A successful change emits `set_denom_paused` with
`denom` and `paused` attributes.

## Issuer binding and keeper APIs

The [IssuerKeeper interface](types/expected_issuer.go) is a Go integration API.
Tokenfactory exposes no `MsgSetIssuerDid` or issuer mint/burn CLI command.
Trusted callers must authenticate their actor before passing a caller DID;
the keeper compares the supplied DID with persisted authority metadata.

- `SetIssuerDid(ctx, denom, issuerDid)` binds an existing denom to a valid DID.
  Repeating the same binding succeeds without a new event. Replacing or clearing
  an existing nonempty binding returns `ErrInvalidAuthorityMetadata`.
- `IssuerDid(ctx, denom)` reads the stored binding. Authority metadata is
  validated when read, including the DID format.
- `MintByIssuer(ctx, callerDid, amount, to)` and
  `BurnByIssuer(ctx, callerDid, amount, from)` require a nonempty binding that
  matches `callerDid`. They use the same pause checks as admin mint and burn;
  account arguments are native bech32 addresses.
- `SetDenomPausedByIssuer(ctx, denom, callerDid, paused)` requires that same
  binding match. It changes the persistent pause flag and emits
  `set_denom_paused` with `denom`, `paused` and `issuer_did` attributes.

The [ledger issuer adapter](../layerxledger/keeper/issuer_binding.go) implements
asset registration with a factory denom binding and mirrors ledger pause
changes into tokenfactory when used. These are keeper integration paths, not
additional tokenfactory transaction messages. `MintByIssuer` validates the
recipient before minting and uses a cache context so a refused send leaves
supply and balances unchanged. `BurnByIssuer` uses the caller's context;
integrations that require rollback across its transfer and burn must provide
a transaction or cache context and commit only on success.

## Pause enforcement and errors

The pause flag is stored in the tokenfactory KV store as `paused|{denom}`;
unpausing deletes the key. [pause.go](keeper/pause.go) exposes
`IsDenomPaused` and `SendEnabledChecker`. The application registers that checker
on both BankKeeper and GigaBankKeeper in [node/app.go](../../node/app.go).

Bank `MsgSend` and `MsgMultiSend` consult send-enabled checks and refuse paused
denoms with bank `ErrSendDisabled`. Tokenfactory admin and issuer mint/burn
refuse with tokenfactory `ErrDenomPaused` (`LXP_ASSET_PAUSED: denom is paused`).
Other denoms are unaffected by this denom's pause flag.

The pause is not a universal bank balance freeze. Low-level bank keeper
transfers, including account-to-module, module-to-account and module-to-module
flows, bypass the send-enabled check. This is the intentional module-transfer
exemption; direct keeper integrations must enforce their own operation policy.
See the bank [message server](../../sdk/x/bank/keeper/msg_server.go) and
[send keeper](../../sdk/x/bank/keeper/send.go).

Relevant errors from [types/errors.go](types/errors.go) use the `tokenfactory`
codespace:

| Error | Code | Meaning |
| --- | --- | --- |
| `ErrDenomExists` | 2 | Bank metadata already exists for the requested denom. |
| `ErrUnauthorized` | 3 | Message sender is not the current admin. |
| `ErrInvalidAuthorityMetadata` | 6 | Invalid authority metadata or an attempt to replace an issuer binding. |
| `ErrDenomDoesNotExist` | 10 | Required denom state is missing. |
| `ErrAdminAlreadyExists` | 16 | New admin equals current admin. |
| `ErrAllowListTooLarge` | 24 | List exceeds the configured maximum. |
| `ErrAllowListUndefined` | 25 | Update supplied no allow list. |
| `ErrDenomPaused` | 26 | Mint or burn refused: `LXP_ASSET_PAUSED`. |
| `ErrPauseUnchanged` | 27 | Requested pause value already holds: `LXP_PAUSED_SCOPE`. |
| `ErrIssuerMismatch` | 28 | Issuer mismatch or admin pause on an issuer-bound denom: `LXP_UNAUTHORIZED_DEBIT`. |

These tokenfactory codes do not replace bank errors or gRPC status codes.

## Queries

The `paxprotocol.paxchain.tokenfactory.Query` gRPC service and
`paxd query tokenfactory` expose the following queries. REST paths are relative
to a node's configured SDK gRPC-gateway API, with the common prefix
`/pax-protocol/paxchain/tokenfactory`. They are distinct from the node's EVM
JSON-RPC service.

| gRPC method | CLI subcommand and arguments | REST GET suffix |
| --- | --- | --- |
| `Params` | `params` | `/params` |
| `DenomAuthorityMetadata` | `denom-authority-metadata <denom>` | `/denom_authority_metadata?denom=<encoded-denom>` |
| `DenomsFromCreator` | `denoms-from-creator <creator>` | `/denoms_from_creator/{creator}` |
| `DenomMetadata` | `denom-metadata <denom>` | `/denoms/metadata?denom=<encoded-denom>` |
| `DenomAllowList` | `denom-allow-list <denom>` | `/denoms/allow_list?denom=<encoded-denom>` |
| `DenomPaused` | `denom-paused <denom>` | `/denom_paused?denom=<encoded-denom>` |

URL-encode the full denom in query parameters. Authority and pause queries also
have `/denoms/{denom}/authority_metadata` and `/denoms/{denom}/paused` bindings;
the query-parameter alternatives avoid path handling issues with denom slashes.
`DenomsFromCreator` lists the original creator's denoms, even after admin
changes, and supports gRPC/CLI pagination.

Authority responses contain `authority_metadata` with `admin` and `issuer_did`.
Pause responses contain `paused` and `issuer_did`. `DenomPaused` returns gRPC
`InvalidArgument` for a nil request or empty denom, and `NotFound` for absent or
invalid authority metadata. An unknown denom is not reported as an unpaused
existing denom. `DenomMetadata` returns `NotFound` when bank metadata is absent;
an empty allow list alone does not establish that a denom exists.

### CosmWasm

The custom query wrapper uses `route: "tokenfactory"` and `query_data` with one
of `denom_authority_metadata`, `denoms_from_creator` or `denom_paused`:

```json
{
  "route": "tokenfactory",
  "query_data": {
    "denom_paused": { "denom": "factory/<creator address>/ufoo" }
  }
}
```

It returns the corresponding query response directly. The wasm implementation
uses Go JSON encoding with `omitempty`, so `paused: false` and an empty
`issuer_did` may be omitted; consumers must handle those default values.
Multiple query variants are rejected with `ErrParsingPaxTokenFactoryQuery`;
an empty query returns `ErrUnknownPaxTokenFactoryQuery`.

A contract can emit the custom message below to pause an unbound denom it
administers. The encoder supplies the contract address as `sender`; normal
admin and issuer-binding checks still apply.

```json
{
  "set_denom_paused": {
    "denom": "factory/<contract address>/ufoo",
    "paused": true
  }
}
```

Both a nonempty `denom` and an explicit boolean `paused` are required. Missing
or null fields fail encoding with `ErrEncodeTokenFactorySetDenomPaused` (29).
See the [wasm bindings](client/wasm/bindings/msgs.go),
[encoder](../../node/wasmbinding/encoder.go) and
[query wrapper](../../node/wasmbinding/query_plugin.go).

## State and recovery

Admin and issuer metadata, creator indexes and pause flags are persistent
chain state. Tokenfactory [genesis](../../api/tokenfactory/genesis.proto)
contains `params`, `factory_denoms` (each with `denom` and
`authority_metadata`) and `paused_denoms`. Export includes issuer bindings
and all paused denoms. Import validates the full input before initialization,
recreates the denoms and creator indexes, restores their authority metadata,
then restores pause flags. Duplicate factory denoms, duplicate pause entries,
invalid authority metadata and pause entries outside `factory_denoms` are
rejected. Unknown paused denoms cause `InitGenesis` to panic with
`ErrDenomDoesNotExist`.

An empty admin remains empty across export/import; genesis does not reinstate
the creator's authority. `ExportGenesisStream` captures the same JSON bytes as
`ExportGenesis` before returning a one-element, closed channel. Mutating or
closing the producer store before draining the channel does not alter that
snapshot.

Tokenfactory genesis does not contain bank balances, supply, customized bank
metadata or allow lists. Preserve the corresponding bank state as part of a
full application export/import; tokenfactory-only genesis recreates default
bank metadata. A restored paused denom remains subject to the same admin/issuer
checks and operation refusals. See [genesis.go](keeper/genesis.go) and
[genesis validation](types/genesis.go).

## Tokenfactory Denom Restrictions

Denoms have the form `factory/{creator address}/{subdenom}`. The full string
must pass SDK denom validation. [denoms.go](types/denoms.go) sets these limits:

- Maximum subdenom length: 44 bytes.
- Maximum creator string length: 75 bytes.
- Bech32 HRP budget: 16 bytes (`MaxHrpLength`, used to derive the creator cap).
- Prefix: `factory` followed by `/`.

Do not parse with fixed offsets: a subdenom can contain `/`, and
`DeconstructDenom` joins all components after the creator back together.
The HRP budget is a sizing assumption, not a separate HRP check in
`GetTokenDenom`.

## Examples

These commands use a local key named `mylocalwallet` and the node/chain settings
in your `paxd` client configuration. Use native bech32 addresses for admin
arguments. EVM chain ID 125 is not a substitute for the SDK transaction chain
ID configured for the node.

### Creating a token

```sh
paxd tx tokenfactory create-denom ufoo --from mylocalwallet
CREATOR="$(paxd keys show mylocalwallet -a)"
DENOM="factory/$CREATOR/ufoo"
```

Wait for the creation transaction to succeed before issuing further messages.

### Mint a new token

Use the complete factory denom and an integer amount in its base unit. The
same current admin can burn coins from its own balance:

```sh
paxd tx tokenfactory mint "100000000000$DENOM" --from mylocalwallet
paxd tx tokenfactory burn "1000000$DENOM" --from mylocalwallet
```

### Checking Token metadata

```sh
paxd query tokenfactory denom-metadata "$DENOM"
paxd query tokenfactory denom-authority-metadata "$DENOM"
paxd query tokenfactory denom-allow-list "$DENOM"
```

### Check the tokens created by an account

```sh
paxd query tokenfactory denoms-from-creator "$CREATOR"
paxd query tokenfactory params
```

### Pause and resume an unbound token

The current admin can pause this CLI-created, unbound denom. Query it before
and after the change; submitting the same pause value twice is refused.

```sh
paxd query tokenfactory denom-paused "$DENOM"
paxd tx tokenfactory set-denom-paused "$DENOM" true --from mylocalwallet
# After the pause transaction succeeds:
paxd query tokenfactory denom-paused "$DENOM"
paxd tx tokenfactory set-denom-paused "$DENOM" false --from mylocalwallet
```

To change admin, use
`paxd tx tokenfactory change-admin "$DENOM" <new-native-bech32-address> --from mylocalwallet`.
Subsequent admin messages must be signed by the new admin.

To permanently renounce admin authority, pass an explicitly quoted empty
argument while signing as the current admin:

```sh
paxd tx tokenfactory change-admin "$DENOM" "" --from mylocalwallet
```

After that transaction succeeds, the former admin cannot restore its authority.
A bound issuer can still control pause state and use its mint/burn keeper APIs;
an unbound denom has no remaining admin to change its pause state.

## Appendix: Expectations from the Chain

The 44-byte subdenom and 16-byte HRP budgets derive from the SDK's 128-byte
maximum denom and a longest address payload of 32 bytes. Its bech32 encoding
uses the HRP, one separator and 58 data/checksum characters. Thus:

```text
128 = 7 (factory) + 2 (slashes) + max_subdenom + max_hrp + 1 + 58
max_subdenom + max_hrp = 60
44 + 16 = 60
max_creator = 16 + 1 + 58 = 75
```

These are the current module constants. If address or SDK denom constraints
change, revisit the limits together rather than relying on these sizes when
parsing a denom.
