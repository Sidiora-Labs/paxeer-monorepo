# OpenAPI docs

When changing a consensus RPC endpoint in this package, update the maintained
[OpenAPI document](../../../rpc/openapi/openapi.yaml) in the same change. Keep
method names, request parameters, response schemas and errors aligned with the
[route registration](routes.go), the endpoint handler and the
[request and result types](../../../rpc/coretypes/).

The existing
[`TestOpenAPIPathsMatchRoutesMap`](openapi_routes_test.go) check compares the
OpenAPI method paths with `NewRoutesMap` with unsafe methods enabled. It also
requires the root JSON-RPC POST operation and the `unsafe_flush_mempool` path.
Use this check when changing the route inventory; review parameter and response
schemas against the implementation as well, since the check only verifies route
coverage and the presence of the root POST operation.
