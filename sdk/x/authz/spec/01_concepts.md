<!--
order: 1
-->

# Concepts

## Authorization and Grant

Paxeer's `x/authz` module lets a granter authorize a grantee to submit SDK messages
on the granter's behalf. These messages execute in the same network as other Paxeer
transactions. The protobuf packages retain their `cosmos.*` names in this fork.

A grant stores an authorization and an expiration time for a
`(granter, grantee, message type URL)` tuple. Saving another grant for that tuple
replaces its authorization and expiration. Grants do not transfer coins to the
grantee or reserve the granter's funds.

The [Authorization interface](../authorizations.go) embeds `proto.Message` and
provides `MsgTypeURL() string`, `ValidateBasic() error`, and
`Accept(ctx sdk.Context, msg sdk.Msg) (AcceptResponse, error)`. A module can implement
this interface for a registered SDK message type. `AcceptResponse` tells authz
whether to accept the action, delete the grant (`Delete`), or store a replacement
authorization (`Updated`).

The [dispatch implementation](../keeper/keeper.go) requires each inner message to
have exactly one signer. When that signer differs from the grantee, authz loads the
grant for the signer's address and the message type URL and calls `Accept`. When
the signer is the grantee, no grant is required. An accepted message still goes
through its normal module handler and can fail that handler's checks. See
[Messages](03_messages.md) for validation, expiration, and transaction behavior.

## Built-in Authorizations

### SendAuthorization

Bank's [SendAuthorization](../../bank/types/send_authorization.go) authorizes
`/cosmos.bank.v1beta1.MsgSend`. Its `spend_limit` is the remaining allowance, as
coins grouped by denomination. `Accept` subtracts the requested amount, returns
`ErrInsufficientFunds` if any allowance would become negative, and updates the
remaining allowance. If it reaches zero across all denominations, authz deletes
the grant.

This authorization does not constrain recipients and does not authorize
`MsgMultiSend`. The [bank message rules](../../bank/spec/03_messages.md), including
sender and recipient denomination allowlists, still apply.

### GenericAuthorization

[GenericAuthorization](../generic_authorization.go) stores a message type URL in
`msg`. It imposes no extra restrictions on messages of that type: its `Accept`
returns `Accept: true`. Grant lookup, signer checks, expiration, and the target
module's validation still apply. Choose this only when the grantee should have the
full capabilities of that message on the granter's behalf.

### StakeAuthorization

Staking's [StakeAuthorization](../../staking/types/authz.go) covers one of
`/cosmos.staking.v1beta1.MsgDelegate`,
`/cosmos.staking.v1beta1.MsgUndelegate`, or
`/cosmos.staking.v1beta1.MsgBeginRedelegate`. It stores a validator policy and an
optional `max_tokens` amount, which is updated as messages execute.

The validator policy is a protobuf `oneof`: an allowlist or a denylist, never both.
The constructor used by the CLI requires exactly one non-empty list. An allowlist
permits only its listed validators; a denylist rejects its listed validators. For
redelegation, the policy checks the destination validator. These constructor
checks are distinct from `ValidateBasic`, which checks the amount and authorization
type rather than reconstructing the validator policy.

## Gas

Validator-list gas is charged by `StakeAuthorization.Accept` during `MsgExec`,
rather than as a list scan during `MsgGrant`. Each validator entry actually visited
costs 10 gas. An allowlist scan stops at a match; a denylist scan rejects at a match.
Ordinary transaction, storage, and executed-message gas charges also apply.

See [Client](05_client.md) for the supported grant, revoke, execution, and query
commands.

## A persisted deny-only grant

For example, a persisted deny-only delegation policy can deny
`paxvaloper1qszqgpqyqszqgpqyqszqgpqyqszqgpqy445nau` while accepting delegation to
`paxvaloper1qvpsxqcrqvpsxqcrqvpsxqcrqvpsxqcr49nk6q`. Construct it with
`NewStakeAuthorization(nil, deniedValidators, AUTHORIZATION_TYPE_DELEGATE, nil)`:
the nil allowlist selects the denylist variant, and nil `max_tokens` leaves the
amount unlimited. `MsgGrant` stores the policy; closing and reopening the store
preserves its denylist.

When `MsgExec` delegates `10uhpx` to the second validator, acceptance scans the
single denied validator, charges 10 gas for that entry, and succeeds. The real
staking handler then records the delegation if its usual funding and validator
checks pass. The authorization update retains the deny-only policy for subsequent
executions. Delegation to the denied validator fails with `ErrUnauthorized`.
`TestRepairM_691_ConceptsDenyOnly` proves this with the real store, codec, router,
bank and staking keepers after reopening the store.
