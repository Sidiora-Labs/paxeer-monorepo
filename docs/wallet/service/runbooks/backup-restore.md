# Attestor backup restore

The attestor stores encrypted shares and its audit log under `ATTESTOR_DATA_DIR`.
`ATTESTOR_NODE_KEY_FILE` supplies the node key. Share encryption binds the key ID,
curve, public key, epoch and participants as associated data.

The [snapshot writer](../../../../services/wallet/attestor/internal/backup/writer.go)
writes encrypted snapshots when `ATTESTOR_BACKUP_DIR` and
`ATTESTOR_BACKUP_KEY_FILE` are configured. `ATTESTOR_SNAPSHOT_INTERVAL` defaults
to `1h`; `ATTESTOR_SNAPSHOT_RETAIN` defaults to `48`. The deployment examples
select `15m` and `96`. Key operations also request snapshots.

## Select a snapshot

Stop the affected node before restoring its data. Preserve its existing data
separately. Use a snapshot for the same node ID with its original node key and
backup key. Obtain the expected SHA-256 digest from the retained snapshot
metadata, separately from the file being restored.

Compare the snapshot's key epochs and participant sets with the current signed
inventory and surviving holders. Shares from an earlier refresh epoch do not
combine with current shares. If the original node key is unavailable, untrusted,
or the snapshot is stale, use [node replacement](node-replacement.md).

## Restore into an empty directory

The [attestor executable](../../../../services/wallet/attestor/cmd/attestor/main.go)
provides the restore command. Load the node's normal runtime configuration,
including `ATTESTOR_NODE_ID`, `ATTESTOR_NODE_KEY_FILE` and
`ATTESTOR_BACKUP_KEY_FILE`, then run:

```sh
attestor restore -snapshot "$SNAPSHOT_FILE" -sha256 "$SNAPSHOT_SHA256" \
  -data-dir "$RESTORE_DATA_DIR"
```

`SNAPSHOT_FILE`, `SNAPSHOT_SHA256` and `RESTORE_DATA_DIR` are operator shell
variables. The destination must be empty; omitting `-data-dir` selects
`ATTESTOR_DATA_DIR`. Restore checks the digest and node identity, decrypts the
snapshot and restores the store and audit data. It appends a restore audit entry.

Start only one instance of the restored node, with the restored directory
mounted as its data directory. Check `/health` over authenticated TLS, compare
key descriptions with surviving holders, and confirm the expected public key,
owner, participant set and epoch before including it in signing. A signature
using a quorum that includes the restored node establishes share compatibility.
See [node replacement](node-replacement.md) for the operator client setup.

## Remote copies

The [SFTP replica shipper](../../../../services/wallet/attestor/internal/replica/config.go)
uses these variables as a complete set:

- `ATTESTOR_REPLICA_ADDR` and `ATTESTOR_REPLICA_USER`.
- `ATTESTOR_REPLICA_KEY_FILE` and `ATTESTOR_REPLICA_HOST_KEY_FILE`.
- `ATTESTOR_REPLICA_REMOTE_DIR` and `ATTESTOR_REPLICA_INTERVAL`.

`ATTESTOR_REPLICA_OPERATION_TIMEOUT` defaults to `2m`. The remote directory must
be an absolute directory below the filesystem root. With shipping configured,
local snapshot retention waits for the corresponding shipping acknowledgment.
Keep the node and backup keys separate from snapshot copies.

## Recovery constraints

A restore does not reverse a refresh or change the approved inventory. Keep an
incompatible restored node out of signing and replace its shares through the
current participants. Preserve the prior data until the restored node has been
checked; never start two copies of the same participant.
