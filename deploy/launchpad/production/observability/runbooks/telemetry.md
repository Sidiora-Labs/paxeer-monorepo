# Telemetry incident

Owner: platform-oncall. Incident commander: incident-commander. Acknowledge <=5 minutes; customer updates <=15 minutes.

Reject high-cardinality labels before export. Use the finite vocabularies for
`service`, `environment`, `zone`, `route`, `status_class`, `error_code`, `stage`
and `dependency`. The native metric reader also accepts `journey` for synthetic
metrics, `window` for the burn-rate metric and `le` for histogram buckets;
journeys and windows have finite vocabularies, and bucket bounds are numeric
or `+Inf`. Per-instance budget 20000; do not allow wallet, token, transaction,
incident ID or raw URL dimensions. Drop telemetry rather than blocking business
commits. Use 512 MiB collector memory limit plus runtime/cgroup ceiling; trace
failures may be dropped under pressure. Never export arbitrary bodies, JWTs,
signatures, stream keys or private messages. Collector positive allowlists are
defense in depth; validate scope/span-link/exemplar content at source before
enabling pipeline. Inspect redaction canaries in isolated qualification and
restrict exporter mTLS and network access.

Evidence: UTC timestamps, release digest, configuration revision, eligible counts, affected route templates, resource inventory, observed failure/recovery and rehearsal result. Keep sensitive forensic identifiers in access-controlled incident artifacts, never public metrics.

Release prerequisite: named human responder mapping and a recorded rehearsal; role names here do not establish staffed coverage.
