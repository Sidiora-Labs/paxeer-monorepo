# Build from the monorepo root: local go.mod replacements require core/ and
# the other committed sibling modules at the same revision.
FROM docker.io/golang:1.26 AS build
WORKDIR /src/tools/paxctl
COPY . /src
RUN go mod download
RUN go build -o /go/bin/releaseverify ./internal/release && \
    VERSION="$(/go/bin/releaseverify version)" && \
    CGO_ENABLED=0 go build -trimpath \
      -ldflags "-s -w -X github.com/sei-protocol/seictl/internal/cliutil.Version=${VERSION}" \
      -o /go/bin/seictl . && \
    /go/bin/releaseverify verify --binary /go/bin/seictl --expected "$VERSION"

FROM gcr.io/distroless/static-debian12
COPY --from=build /go/bin/seictl /usr/bin/seictl
ENTRYPOINT ["/usr/bin/seictl"]
