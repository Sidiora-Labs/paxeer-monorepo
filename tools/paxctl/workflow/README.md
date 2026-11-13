# State-Sync Workflow

`seictl workflow state-sync` renders the StateSync recipe, applies a
`SeiNodeTaskWorkflow` to Kubernetes and watches its progress. The deployed daemon
is `paxd`; `stop-seid` is the sidecar task identifier used to stop it. Workflow
execution belongs to the cluster's controller. The CLI's renderer and watch are
implemented in [render.go](render.go) and [state_sync.go](state_sync.go).

## Standard resync

Use a disposable full node with the intended chain configuration and a controller
that supports `sei.io/v1alpha1` workflows:

```sh
seictl workflow state-sync <node> -n <namespace> --dry-run
seictl workflow state-sync <node> -n <namespace>
```

The first command performs server-side dry-run admission and prints the resource.
The second applies it and watches for `Complete`. The default workflow name is
`<node>-state-sync`; `--name` selects another name. `--kubeconfig` and `-n` select
the cluster and namespace. `--no-watch` applies and prints the resource without
waiting for execution.

The recipe uses the sidecar's `mark-not-ready`, `stop-seid`, `reset-data`,
`configure-state-sync` and `mark-ready` operations. Data reset is destructive.
The sidecar's [reset handler](../sidecar/tasks/reset_data.go) requires a persisted
readiness hold, a completed stop, full-node configuration and no active validator
identity or remote signer before any erasure. It preserves identity files,
configuration and the exact existing `priv_validator_state.json` bytes, with a
durable last-sign backup outside `data/`.

A successful watch means the workflow reached `Complete` at an observed
generation at least as recent as the apply. Catch-up is a separate node check;
the command prints this handoff:

```sh
seictl node watch <node> --until=caught-up -n <namespace>
```

That watch first waits for `Running`, then checks committed height above 1 and
`catching_up=false` through the node's published CometBFT endpoint. If the node
publishes an EVM endpoint, it also waits for that listener. Run it with network
access to those endpoints. Its timeout defaults to 15 minutes.

## Witnesses

`--rpc-servers` supplies repeatable bare `host:port` witnesses:

```sh
seictl workflow state-sync <node> --rpc-servers host-a:26657 --rpc-servers host-b:26657
```

The final rendered `spec.stateSync.rpcServers`, including changes from `--set`,
must contain at least two distinct valid endpoints when supplied. Hostnames are
compared without case or a trailing dot, and numeric ports are normalized for
distinctness. When omitted, the sidecar derives RPC candidates from persistent
peers on port 26657.

The executing controller must pass the selected witnesses to **both** `reset-data` and `configure-state-sync`.
The [sidecar preflight](../sidecar/tasks/statesync.go) requires distinct reachable
witnesses with different public node IDs on the configured genesis chain. It
selects the lowest reported latest height minus 2000, floored at 1, and requires
matching trust hashes from the reachable witnesses. A lone surviving witness is never duplicated.
Plan-time endpoint count does not prove runtime availability.

Reset checks quorum before erasure; configuration checks it again. A reset
preflight refusal leaves the existing chain data and hold retained. Configuration
can refuse after erasure, also with the hold retained. Witnesses supply the trust
point; snapshot chunks arrive separately from snapshot-serving P2P peers.

## Store migration

The renderer accepts one typed migration, `GigaStore`, paired with backend
`pebbledb` or `rocksdb`:

```sh
seictl workflow state-sync <node> --migration GigaStore --backend pebbledb --dry-run
```

Both flags are required for a migration. The CLI prints a destructive migration
warning before applying, including during dry-run. Omitting migration flags
requests a standard resync. A nonempty `--config-patch` value is refused with
instructions to use the typed migration.

| Input | Resource field |
| --- | --- |
| `<node>` | `spec.target.nodeRef.name` |
| `--require-phase` | `spec.target.requirePhase` |
| `--migration GigaStore` | `spec.stateSync.migration.kind` |
| `--backend` | `spec.stateSync.migration.gigaStore.backend` |
| `--rpc-servers` | `spec.stateSync.rpcServers` |

`--require-phase` accepts `Pending`, `Initializing`, `Running`, `Failed` or
`Terminating`. The cluster's controller interprets the recipe and migration
fields; inspect its rendered task plan before executing a wipe.

## Re-run semantics

The watching command refuses a same-named workflow already in `Complete` or
`Failed`. For a completed workflow, delete it with
`seictl workflow delete <name>` or use a fresh `--name`.

The CLI's recovery guidance for a failed workflow is to inspect
`.status.plan.tasks`, restore the required witnesses, annotate
`sei.io/force-delete-workflow=<reason>`, delete the workflow and then re-run.
Force-deletion releases the hold and may start paxd immediately. Keep paxd
stopped through deployment controls until its current data and configuration
are suitable for restart. A new `--name` alone does not release an existing hold.

## Output and exit codes

Watching emits full workflow objects as NDJSON on stdout. Dry-run and
`--no-watch` instead emit one indented resource object. Progress messages go to
stderr with the `seictl:` prefix; an apply or watch error emits a Kubernetes
`metav1.Status` object there.

The watch exits zero at `Complete`, and nonzero on `Failed`, timeout or API error.
`--timeout` defaults to `15m` and bounds the workflow watch, rather than subsequent
node catch-up. The error status reason is `Timeout` for watch expiry and
`InternalError` for a terminal failure. API errors retain their API status reason.
Use `seictl workflow state-sync --help` for the full flag list.
