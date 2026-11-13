# Admin

The `admin` package provides runtime logging administration for the Paxeer network's `paxd` node. Its dedicated loopback-only gRPC service changes log levels, reports a logger's current level, and lists registered loggers without restarting the node.

## Layout

- [config.go](config.go): configuration defaults, application option loading, loopback validation, and the configuration template.
- [config_test.go](config_test.go): configuration loading and address validation tests.
- [server.go](server.go): listener creation and registration of the admin gRPC service.
- [server_test.go](server_test.go): listener validation, service calls over gRPC, and shutdown tests.
- [service.go](service.go): `SetLogLevel`, `GetLogLevel`, and `ListLoggers` implementations using `paxlog`.
- [service_test.go](service_test.go): log level changes, pattern matching, request validation, and logger listing tests.
- [types/](types/): generated protobuf messages and gRPC bindings in [admin.pb.go](types/admin.pb.go), defined by [admin.proto](../api/pax/admin/v0/admin.proto).

## Build and test

This package belongs to the root [Go module](../go.mod). It builds with the node and has no separate Makefile. Run the covering targets from the repository root, as defined in [chain.mk](../chain.mk):

```sh
make -f chain.mk build
make -f chain.mk test
```

The build target compiles `paxd` into `build/paxd` after building its native dependencies. The test target discovers Go packages with tests, including `admin`, and runs them in groups with race detection and coverage.

## Configuration

[config.go](config.go) reads these application options from the `admin_server` section of `app.toml` through the node's application options:

- `admin_server.admin_enabled`: controls whether the admin server starts.
- `admin_server.admin_address`: selects its listen address, which must contain a numeric loopback IP and port when enabled.

## Documentation

- [Admin & HPX](../docs/site/paxeer-docs/public/markdown/admin-hpx.md): admin service configuration, methods, and client usage.
- [REST and gRPC](../docs/site/paxeer-docs/public/markdown/rest-grpc.md): the admin service alongside the node's other gRPC APIs.
