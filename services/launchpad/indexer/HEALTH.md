# Indexer health answers

The indexer exposes `/health`, `/health/live`, and `/health/ready`. The first
two paths report process liveness. The platform reads `/health/ready`, which
checks the database, Redis, the event broker connection, lifecycle state, and
the ingestion component. A disconnected broker makes readiness unavailable
until its connection recovers.

The ingestion check is `ok` while this instance holds the canonical lease. An
instance that is waiting for another holder reports `standby` for ingestion
and still answers readiness successfully when the other checks are healthy.
Standby instances do not write batches or report durable-height lag as their
own failure. When the lease becomes available, the instance acquires it and
resumes from the durable canonical watermark.

The internal listener also serves `/internal/source-selection` for signed source
selection and `GET /internal/ranking-evidence` for cutoff-bound pause evidence.
The latter accepts `cutoffHeight`, `cutoffTime` (UTC Unix seconds), and `blockHash`.
It returns canonical pool pause states and SHA-256 evidence digests without
self-volume observations. Unknown pools are omitted; an unavailable base or a
cutoff before the base returns 503. Invalid or noncanonical cutoffs return 4xx.

A background worker builds the base from `paused()` calls pinned to a complete
block hash, with 32 concurrent reads, two seconds per read, and a 90-second total
budget. It refreshes event coverage every second with a 30-second budget and
rebuilds the base every 60 seconds. Complete configured emitter coverage uses
stored raw logs; other ranges use one topic-filtered chain log query. Beacon
upgrade events hold affected cutoffs unavailable until the base is rebuilt.
Requests use cached states and one database cutoff check, without per-pool RPC.
