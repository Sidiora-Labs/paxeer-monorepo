# human-api

The versioned HTTPS and JSON contract between the human service and the web application. `v1.kvx` is the root: it declares the schema version, the compatibility rule (additive only within a major version), the authorization model, and includes the contract modules `journeys`, `errors`, `stream`, `identity`, `movement`, `agents`, `activity`, `support`, `home`, `intent` and `program-approvals`. `baseline.kvx` is the additive-only baseline written by `schema-check --write-baseline`, and `compatibility.kvx` records every released pairing of schema, service and web application versions.

`golden/` holds a request, response and failure JSON example for each operation.

Operations also own supplemental fixtures through `golden_requests`, `golden_responses`, `golden_failures`, `golden_inputs` and `golden_artifacts`: lists of `set/file` paths within `golden/`. The checker validates supplemental HTTP vectors against the owning operation, parses producer inputs as non-empty JSON objects and requires non-empty binary artifacts. Every supplemental file has exactly one owner; missing, duplicate and unowned files fail the check. Historical examples retain their wire vocabulary; the existing producer and decoder tests continue to consume their original paths and verify their semantics and artifact bytes.

From the repository root:

```sh
make human-gen-api   # generate the web client into apps/web/src/api/generated
make human-check     # includes schema-check over this directory
```

The generated client is never authoritative; this schema is.
