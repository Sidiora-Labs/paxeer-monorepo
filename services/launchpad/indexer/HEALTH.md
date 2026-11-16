# Indexer health answers

The indexer exposes `/health`, `/health/live`, and `/health/ready`. The first
two paths report process liveness. The platform reads `/health/ready`, which
checks the database, Redis, lifecycle state, and the ingestion component.

The ingestion check is `ok` while this instance holds the canonical lease. An
instance that is waiting for another holder reports `standby` for ingestion
and still answers readiness successfully when the other checks are healthy.
Standby instances do not write batches or report durable-height lag as their
own failure. When the lease becomes available, the instance acquires it and
resumes from the durable canonical watermark.
