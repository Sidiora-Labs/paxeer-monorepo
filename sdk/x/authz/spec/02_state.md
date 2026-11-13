<!--
order: 2
-->

# State

## Grant

Grants are identified by combining granter address (the address bytes of the granter), grantee address (the address bytes of the grantee) and Authorization type (its type URL). Hence we only allow one grant for the (granter, grantee, Authorization) triple.

- Grant: `0x01 | granter_address_len (1 byte) | granter_address_bytes | grantee_address_len (1 byte) | grantee_address_bytes |  msgType_bytes-> Protobuf(Grant)`

The grant object encapsulates an `Authorization` type and an expiration timestamp:

See [`Grant` in authz.proto](../../../proto/cosmos/authz/v1beta1/authz.proto).
