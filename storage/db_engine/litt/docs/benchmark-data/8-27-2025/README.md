# LittDB soak benchmark data

This directory contains charts and [raw measurements](data/raw.tar.gz) for a
14-day, approximately 192 TB storage workload with a seven-day TTL. The first
seven days cover ramp-up and the next seven steady-state retention. These are
local database measurements, not network throughput or consensus measurements.

## Environment

| Property          | Value                                        |
|-------------------|----------------------------------------------|
| code              | upstream LittDB |
| environment       | OCI (Oracle Cloud Infrastructure)            |
| region            | US East (Ashburn)                            |
| OS                | Canonical Ubuntu 20.04                       |
| shape             | VM.Optimized3.Flex                           |
| OCPU count        | 1                                            |
| Network Bandwidth | 4 Gbps                                       |
| Memory            | 14 GB                                        |
| Disk              | 8x 32TB block volumes, per disk config below |
| Disk Performance  | Balanced (VPU/GB:10)                         |
| Disk Throughput   | 480 MB/s                                     |
| Disk IOPS         | 25,000 IOPS                                  |
| Disk encryption   | disabled                                     |
| Disk backup       | disabled                                     |

## Configuration

```json
{
  "LittConfig": {
    "Paths": ["~/mount/b", "~/mount/c", "~/mount/d", "~/mount/e", "~/mount/f", "~/mount/g", "~/mount/h", "~/mount/i"],
    "MetricsEnabled": true
  },
  "MaximumWriteThroughputMB": 1024,
  "MetricsLoggingPeriodSeconds": 1,
  "TTLHours": 168
}
```

The storage roots use ext4. The benchmark's default random-data pool occupies
1 GiB; account for that allocation separately from database caches and keymaps.
See [BenchmarkConfig](../../../benchmark/config/benchmark_config.go) for current
fields and defaults.

## Measurements

| | |
|---|---|
| ![Disk Footprint](data/disk-footprint.webp) | ![Key Count](data/key-count.webp) |
| ![Bytes Written / Second](data/bytes-written-second.webp) | ![Keys Written / Second](data/keys-written-second.webp) |
| ![Flushes / Second](data/flushes-second.webp) | ![Write Latency](data/write-latency.webp) |
| ![Flush Latency](data/flush-latency.webp) | ![Segment Flush Latency](data/segment-flush-latency.webp) |
| ![Keymap Flush Latency](data/keymap-flush-latency.webp) | ![GC Latency](data/gc-latency.webp) |
| ![Bytes Read / Second](data/bytes-read-second.webp) | ![Keys Read / Second](data/keys-read-second.webp) |
| ![Read Latency](data/read-latency.webp) | ![Cache Hits / Second](data/cache-hits-second.webp) |
| ![Cache Misses / Second](data/cache-misses-second.webp) | ![Cache Miss Latency](data/cache-miss-latency.webp) |
| ![Memory](data/memory.webp) | ![CPU Seconds](data/cpu-seconds.webp) |

## Run the benchmark

Use the Go version required by the repository's `go.mod`. From the repository root:

```bash
cd storage/db_engine/litt
make build
./bin/litt benchmark /path/to/benchmark_config.json
```

The [basic configuration](../../../benchmark/config/basic-config.json) provides
a smaller set of explicit settings. Choose dedicated storage roots and set
`TimeLimitSeconds` to bound a run; its default of zero runs until interrupted.
The benchmark validates sampled reads against its generated data.
