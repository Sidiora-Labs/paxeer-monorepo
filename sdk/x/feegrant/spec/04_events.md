<!--
order: 4
-->

# Events

The [feegrant keeper](../keeper/keeper.go) emits the event types defined in
[events.go](../events.go). These names are event types, with `granter` and
`grantee` attributes. They are distinct from BaseApp's `message.action`.

## Msg Server

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

If the allowance remains, saving its updated state also emits `set_feegrant`.
If successful use consumes the allowance, the keeper emits `revoke_feegrant`
before `use_feegrant`. These events arise during fee deduction, without a
separate allowance-use transaction message. See [Concepts](01_concepts.md).

## Transaction message actions

[BaseApp](../../../baseapp/baseapp.go) emits a `message` event with an `action`
attribute for each successfully handled transaction message. Protobuf message
service dispatch uses the message's full type URL:

| Message             | `message.action`                              |
| ------------------- | --------------------------------------------- |
| MsgGrantAllowance   | /cosmos.feegrant.v1beta1.MsgGrantAllowance    |
| MsgRevokeAllowance  | /cosmos.feegrant.v1beta1.MsgRevokeAllowance   |

The [legacy `Type()` methods](../msgs.go) also return these type URLs. There is
no `MsgUseAllowance` action: a transaction using a grant has the actions of its
own messages, alongside the feegrant events emitted by the ante handler.
