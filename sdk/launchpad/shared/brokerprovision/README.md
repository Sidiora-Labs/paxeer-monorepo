# Broker provisioning

Run the command from `sdk/launchpad/shared`:

```sh
go run ./brokerprovision/cmd/brokerprovision check \
  --document /run/launchpad/broker.json \
  --broker-env LAUNCHPAD_BROKER_URL --credentials-file /run/launchpad/broker.creds
```

`provision` creates absent streams and durables. Equal resources produce no
writes or output. A differing resource requires `--update-stream NAME` or
`--update-durable STREAM/DURABLE`; repeat the argument for each approved update.
The command preflights all differences before writing. Updates use the broker's
update APIs; immutable settings can require an operator-managed migration.
Neither action deletes a resource. An interrupted provision may leave some
changes applied; rerunning it reconciles the remaining resources.

`check` performs reads only and prints one line per absent or differing stream
or durable. Exit status is 0 for equality, 1 for drift, and 2 for invalid input
or an operational error. Provision reports each applied resource and exits 0
only after checking equality. Broker errors contain resource names, never
connection addresses or credentials.

Both actions require `--broker-file` or `--broker-env` and one authentication
source: `--credentials-file` / `--credentials-env` for a NATS user credentials
bundle, or `--token-file` / `--token-env` for a token. Environment arguments name
the variable; values are read from the process environment. Addresses use
`nats://` or `tls://` and cannot embed authentication. TLS uses the system trust
store. `--timeout` bounds the entire operation from 1 second to 5 minutes
(default 30 seconds).

The JSON document contains `streams` and `durables`. Each stream has `name`,
`subjects` and `limits`; every field shown in the example is required. Limits
are positive and finite, replicas are 1..5, maxConsumers is 1..10000, maxBytes
must hold maxMessageBytes, and duplicateWindow cannot exceed maxAge. Discard
is `new` or `old`. Streams use file storage and limits retention. Subjects
admit NATS `*` and terminal `>` wildcards. Durables use the exact members of
`natsdriver.ConsumerConfig`; durations throughout the document are integer
nanoseconds. Its `BrokerConsumerConfig` method validates all consumer bounds
and derives the broker configuration. A durable must reference a listed stream
whose subjects cover its literal subject. Unknown, duplicate and null fields,
duplicate resource identities and trailing documents are refused.

The composer example is `deploy/launchpad/documents/values/broker.example.json`.
It includes broker streams, not the logical `sourceStream` identifiers of
projection documents. Its social chat and API sockets streams use three
replicas; provisioning these requires a broker cluster with three servers.
It keeps messages for 72 hours with explicit message, byte and consumer limits.
Subjects supplied separately from composer values, such as the user service's
`ACCOUNT_NATS_STREAM`, must be added to the operator's deployment document.
