# Blockscout Docker integration

The `Makefile` wraps the Compose files in
[`../docker-compose`](../docker-compose/README.md). From
`services/explorer/backend/docker`, run `make start` to start the database,
backend, microservices, frontend and proxy. Run `make stop` to stop and remove
the running application containers managed by the wrapper.

Manage the explorer database separately with its Compose file. From the same
directory, stop it with:

```sh
docker-compose -f ../docker-compose/services/db.yml down
```

See the [Compose guide](../docker-compose/README.md) for service configuration
and environment files.
