# scripts

Helper scripts inherited from the upstream Cosmos SDK. Run them from `sdk/`.

| Script | What it does |
| ------ | ------------ |
| `module-tests.sh` | Runs `go test` with coverage in every nested Go module under the current directory (each `go.mod` other than `./go.mod`), regardless of `GIT_DIFF`, so dependent modules are tested too. Publishes a fresh `coverage-go-submod-profile.out` with one atomic-mode header only after all module tests and coverage-mode checks succeed; a failed run preserves the previous profile. |
| `protoc-swagger-gen.sh` | Combines the Swagger inputs configured in `client/docs/config.json` into `client/docs/swagger-ui/swagger.yaml` with `swagger-combine`. The `proto-swagger-gen` target in [`sdk/Makefile`](../Makefile) invokes it in the Protobuf generation container. |
| `update-swagger-ui-statik.sh` | Delegates to the [root asset script](../../scripts/update-swagger-ui-statik.sh) with `--assets-only`, embedding `docs/swagger-ui` into `docs/swagger/statik.go` without regenerating OpenAPI. |
| `linkify_changelog.py` | Rewrites ` \#<number>` references in a changelog file into links to upstream Cosmos SDK issues. |
