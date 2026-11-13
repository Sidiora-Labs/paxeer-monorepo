# Prometheus metrics

The node exposes application telemetry through the REST API and consensus
metrics through the instrumentation listener.

## Node configuration

In `<node home>/config/app.toml`, enable the API and telemetry and choose a
positive retention time in seconds:

```toml
[api]
enable = true

[telemetry]
enabled = true
prometheus-retention-time = 15
```

The API serves `GET /metrics?format=prometheus`; its default port is 1317.
The `format` parameter selects Prometheus output. A positive
`prometheus-retention-time` enables the telemetry Prometheus sink.

In `<node home>/config/config.toml`, enable consensus metrics:

```toml
[instrumentation]
prometheus = true
```

The consensus instrumentation listener serves `/metrics` on port 26660 by
default. The supplied [`prometheus.yaml`](prometheus.yaml) scrapes that
listener every five seconds. To collect application telemetry, configure an
additional scrape target for the API with the `format=prometheus` query
parameter.

## Local Prometheus

From the `wasm/` directory:

```sh
docker run -it -v "$(pwd)/contrib/prometheus:/prometheus" -p 9092:9090 prom/prometheus --config.file=/prometheus/prometheus.yaml
```

This publishes the Prometheus console on port 9092. The node's default gRPC
and gRPC-web ports are 9090 and 9091. Configure the supplied scrape target so
that the container can resolve and reach the host instrumentation listener.

## Local Grafana

```sh
docker run -it -p 3000:3000 grafana/grafana
```

Open Grafana on port 3000 and add the Prometheus server on port 9092 as a data
source reachable from the Grafana container.

## Wasm timings

The [keeper](../../x/wasm/keeper/keeper.go) records telemetry timings under
`wasm_contract_instantiate`, `wasm_contract_execute`, `wasm_contract_migrate`,
`wasm_contract_sudo`, `wasm_contract_query-smart` and
`wasm_contract_query-raw`. [IBC callbacks](../../x/wasm/keeper/relay.go) use
`wasm_contract_ibc-*` timing keys. Application telemetry must be included in
the scrape configuration to collect these metrics.
