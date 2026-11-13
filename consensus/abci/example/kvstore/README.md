# KVStore

The example's [`Application`](kvstore.go), constructed with `NewApplication()`, implements the [ABCI application interface](../../types/application.go).

## Application

The application is a simple key-value store backed by an in-memory database.
Transactions of the form `key=value` are stored as key-value pairs.
Transactions that do not split into exactly two parts on `=` set the value to the key.
Transactions of the form `val:pubkey!power` (base64 Ed25519 public key) update the validator set.
Other transactions beginning with `prepare` return success without updating the key-value store.
`FinalizeBlock` applies transactions in order. The application does not track transaction identities to prevent replay.
