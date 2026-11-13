# Git hooks

Installation, from the repository root:

```sh
git config core.hooksPath sdk/contrib/githooks
```

## pre-commit

The hook runs `gofmt -s`, `misspell` and `goimports` on the staged `.go`
files and adds them back to the commit. It skips paths matching `vendor/`,
`client/docs/statik/`, `tests/mocks/` and `*.pb.go`. It then runs
`go mod tidy` and stages `go.mod` and `go.sum`.

The hook requires `git`, `go`, `gofmt`, `goimports` and `misspell` on `PATH`.
Install the two tools that do not ship with Go:

```sh
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/misspell/cmd/misspell@latest
```

The hook uses `-local github.com/cosmos/cosmos-sdk` for import grouping.
