# scripts

Helper scripts inherited from the upstream Cosmos SDK. Run them from `sdk/`.

| Script | What it does |
| ------ | ------------ |
| `module-tests.sh` | Runs `go test` with coverage in every nested Go module under the current directory (each `go.mod` other than `./go.mod`), regardless of `GIT_DIFF`, so dependent modules are tested too. Publishes a fresh `coverage-go-submod-profile.out` with one atomic-mode header only after all module tests and coverage-mode checks succeed; a failed run preserves the previous profile. |
| `protoc-swagger-gen.sh` | Generates Swagger files from the `query.proto` and `service.proto` files under `./proto` with `buf protoc`. The `proto-swagger-gen` target in [`sdk/Makefile`](../Makefile) calls it. |
| `update-swagger-ui-statik.sh` | Runs `statik` to embed `client/docs/swagger-ui` into `client/docs`. |
| `linkify_changelog.py` | Rewrites ` \#<number>` references in a changelog file into links to upstream Cosmos SDK issues. |
