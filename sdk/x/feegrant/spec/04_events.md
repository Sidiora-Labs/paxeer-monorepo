<!--
order: 4
-->

# Events

The [feegrant keeper](../keeper/keeper.go) emits the event types defined in
[events.go](../events.go). These names are event types, with `granter` and
`grantee` attributes. They are distinct from BaseApp's `message.action`.

## Keeper events

### MsgGrantAllowance

A successful grant emits:

| Type         | Attribute Key | Attribute Value  |
| ------------ | ------------- | ---------------- |
| set_feegrant | granter       | {granterAddress} |
| set_feegrant | grantee       | {granteeAddress} |

### MsgRevokeAllowance

A successful revocation emits:

| Type            | Attribute Key | Attribute Value  |
| --------------- | ------------- | ---------------- |
| revoke_feegrant | granter       | {granterAddress} |
| revoke_feegrant | grantee       | {granteeAddress} |

### Exec fee allowance

Successful `UseGrantedFees` emits:

| Type         | Attribute Key | Attribute Value  |
| ------------ | ------------- | ---------------- |
| use_feegrant | granter       | {granterAddress} |
| use_feegrant | grantee       | {granteeAddress} |

If the allowance remains, `use_feegrant` is followed by `set_feegrant` when
the keeper saves its updated state.
If successful use consumes the allowance, the keeper emits `revoke_feegrant`
before `use_feegrant`. These events arise during fee deduction, without a
separate allowance-use transaction message. See [Concepts](01_concepts.md).

An expired allowance emits `revoke_feegrant` when removed by the keeper and
returns `ErrFeeLimitExpired`, without a `use_feegrant` event. A rejected fee or
message filter emits no feegrant event and leaves the stored allowance intact.
These are keeper-level results; events and store changes from a rejected
transaction's cache are not committed.

## Transaction message actions

[BaseApp](../../../baseapp/baseapp.go) emits a `message` event with an `action`
attribute for each successfully handled transaction message. Protobuf message
service dispatch uses the message's full type URL:

| Message             | Protobuf `message.action`                 | Legacy `Type()` action                    |
| ------------------- | ----------------------------------------- | ----------------------------------------- |
| MsgGrantAllowance   | /cosmos.feegrant.v1beta1.MsgGrantAllowance  | /cosmos.feegrant.v1beta1.MsgGrantAllowance  |
| MsgRevokeAllowance  | /cosmos.feegrant.v1beta1.MsgRevokeAllowance | /cosmos.feegrant.v1beta1.MsgRevokeAllowance |

The [legacy `Type()` methods](../msgs.go) also return these type URLs. There is
no `MsgUseAllowance` action: a transaction using a grant has the actions of its
own messages, alongside the feegrant events emitted by the ante handler.
For example, a bank send using a grant has protobuf `message.action` equal to
`/cosmos.bank.v1beta1.MsgSend` (legacy `Type()` is `send`), together with
`use_feegrant` and either `set_feegrant` or `revoke_feegrant`.

An indexer can select `set_feegrant.granter='{granterAddress}'`,
`revoke_feegrant.grantee='{granteeAddress}'`, or
`use_feegrant.granter='{granterAddress}'`. To select grant transactions by
message action, use
`message.action='/cosmos.feegrant.v1beta1.MsgGrantAllowance'`; keeper event names
such as `set_feegrant` are never the feegrant message action.
