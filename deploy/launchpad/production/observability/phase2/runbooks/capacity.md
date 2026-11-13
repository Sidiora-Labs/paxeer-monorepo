# Capacity and cost reconciliation

Capture resource and workload telemetry for the same release digest and interval as the scenario manifest. Retain independently sourced invoice units and hash-bound native artifacts. Separate baseline reservation, burst ceiling, storage growth and CDN egress. Normalize reads and events per million, sockets per thousand and video consumption per viewer-hour. Missing or zero measured denominators reject qualification; a schema-valid allocation does not establish capacity.

Keep log, trace and metric retention at independently configured bounds. Cost alarms must not delete durable events, required WAL or application records. Unit prices are observations for the recorded provider and invoice interval, not guarantees. Capacity release certification remains dependent on the complete measured workload and independently reconciled resource evidence.

In the phase 3 rehearsal, omit measured-demand or invoice captures once and retain the failed qualification result. Each measured demand and workload denominator must be positive, all referenced resource artifacts must be available under their recorded hashes, and invoice units must reconcile exactly. Restore the original captures before evaluating recovery. Capture authoritative rows before and after budget pressure so cost controls cannot silently discard durable state.

Phase 4 measurement plans have exact fields version (`4.0`), binding_sha256, configuration_sha256, source_sha256, d16_bindings, run_id, namespace, sample_interval_seconds (`30`), duration_seconds, max_capture_bytes, budget_path and budget_sha256. The duration is genuine wall-clock time from 60 seconds through 86400 seconds in 30-second multiples. Pin the actual workload candidate, interface, topology and generator hashes, the exact current observability source and the same isolated namespace as the attested scenario evidence. Choose the approved capture budget from actual provisioned disk, not an estimated series count. The final scrape closes the counter interval after the declared duration; keep shared collection active until it finishes.

The bound budget document has exact fields version (`4.0`), bindings, limits, retention_seconds, cost and allocations. Limits and cost use the shared workload provisioned-budget contract, including actual canonical archive storage, replicated broker headroom, network/egress allowance, 72-hour retention and an approved operating-budget revision. Allocation rows contain service, instance, zone, cpu_cores, memory_bytes, disk_bytes and provider_revision. Include every actual allocated instance across all twelve runtime roles. Preserve positive CPU and exact integer byte allocations; invoice and measured demand captures remain mandatory independent inputs.

Collect native `resource_pool` rows from the owning telemetry provider. Each row carries its actual instance, observed_utc_ns, per-instance process metrics and native source timestamps. Memory limits must match the bound allocation, observed memory stays below that limit, CPU utilization stays at or below 70 percent and OOM count remains zero. A missing allocated instance fails measurement. Native gateway captures must cover the same run, replicas and interval; retain the runtime-authority and clock-calibration documents under their original hashes in the shared capture.

An isolated component capture measures only that component. A `final_shared`
capture requires the actual 24-hour interval, declared peak origin, quote and
mutation demand, committed-event and delivery counts, and peak socket inventory.
Shortened or sparse inputs fail. Connection ramps, fanout bursts, event surges,
managed playback, zone loss and restoration require their own measured artifacts.
A finite observation does not establish monthly availability or a guaranteed
provider price.
