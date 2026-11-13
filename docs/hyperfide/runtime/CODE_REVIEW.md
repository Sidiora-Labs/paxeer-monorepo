# Runtime contracts

Hyperfide separates client state, server coordination, host execution and plugin
extensions. The server validates API input and persists shared state; the host
daemon owns local filesystem and provider processes. Clients connect to the
server and render its thread and environment state.

The [repository overview](repository-overview.md) locates the runtime packages.
The [provider plugin API](provider-plugin-api.md) describes provider registration,
interactions and rendering. The [bridge protocol](provider-bridge-protocol.md)
defines the wire requests, notifications and capabilities between a provider
artifact and the host daemon.

[Configuration](configuration.md) describes startup and host boundaries.
[PostgreSQL storage](postgresql-storage.md) describes the asynchronous database
API, transaction callbacks and dialect selection. [Environment provisioning](environment-provisioning.md)
describes lifecycle state, setup and cleanup.
