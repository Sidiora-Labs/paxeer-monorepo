<!--
order: 3
-->

# Messages

The authz message definitions are in
[tx.proto](../../../proto/cosmos/authz/v1beta1/tx.proto). The processing below follows
[message validation](../msgs.go), the [message server](../keeper/msg_server.go), and
[grant storage and dispatch](../keeper/keeper.go). See [Concepts](01_concepts.md)
for authorization types and [Client](05_client.md) for commands.

## MsgGrant

`/cosmos.authz.v1beta1.MsgGrant` contains `granter`, `grantee`, and a `grant` holding
an authorization `Any` and `expiration`. The granter signs. A new grant overwrites
the stored grant for the same `(granter, grantee, authorization.MsgTypeURL())` tuple,
including its previous allowance and expiration.

Basic validation rejects invalid addresses, equal granter and grantee addresses,
an authorization that does not unpack to `authz.Authorization`, or an authorization
whose own `ValidateBasic` fails. The handler rejects a missing unpacked
authorization with `ErrUnpackAny` and a type URL without a registered
`MsgServiceRouter` handler with `ErrInvalidType`. Successful storage emits
`EventGrant`.

The current handler, `SaveGrant`, and `NewGrant` do **not** reject expiration times
in the past. A successful grant transaction therefore does not prove that the
authorization is usable. Clients should choose a future expiration explicitly.
Expiration is checked when loading a clean authorization for execution or a
message-type-filtered query, as described below.

## MsgRevoke

`/cosmos.authz.v1beta1.MsgRevoke` contains `granter`, `grantee`, and `msg_type_url`.
The granter signs. Basic validation rejects invalid or equal addresses and an empty
type URL. The handler deletes the matching grant and emits `EventRevoke`; if no
grant exists, it returns `ErrNotFound` with `authorization not found`.

## MsgExec

`/cosmos.authz.v1beta1.MsgExec` contains the signing `grantee` and a non-empty list
of messages packed as `Any`. Basic validation rejects an invalid grantee or an
empty list. The handler must unpack every entry as `sdk.Msg` before dispatch.

For each inner message, dispatch performs these steps in order:

1. Require exactly one signer, or return `ErrInvalidRequest`.
2. If that signer is the grantee, proceed without an authorization. Otherwise,
   load the grant for the signer as granter, the grantee, and the inner message's
   type URL. A missing or expired authorization returns `ErrUnauthorized` with
   `authorization not found`.
3. For an action requiring a grant, call `Authorization.Accept`. Propagate its
   error, apply its requested deletion or authorization update, and reject
   `Accept: false` with `ErrUnauthorized`.
4. Find the inner message's `MsgServiceRouter` handler. An absent handler returns
   `ErrUnknownRequest`. Execute the handler and propagate any failure, including
   the target module's balance and policy errors.

Successful execution returns one result byte slice per message and emits the
inner events with an `authz_msg_index` attribute identifying the zero-based message
index. Grant updates and inner message writes participate in the transaction's
cached state; a failed SDK transaction does not commit those message writes.

### Expiration and cleanup

`GetCleanAuthorization` treats a grant as expired only when
`expiration.Before(ctx.BlockHeader().Time)` is true. Equality with block time is
not expired. For an expired grant, it calls `DeleteGrant` in the supplied context
and returns no authorization.

This cleanup is subject to that context's commit rules. In particular, deletion
attempted by a failing `MsgExec` is rolled back with the transaction's message
state; a query does not commit a deletion to chain state. Do not rely on a failed
execution to persistently remove expired grants. Unfiltered list queries can still
return stored expired grants; revoke explicitly when persistent removal is needed.
