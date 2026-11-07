FROM docker.io/golang:1.25.8-bookworm@sha256:4557cf171e3cdf5053a298d5171b1a5f5734d920260c25f22c79e94760eb2084 AS build
ENV GOTOOLCHAIN=local CGO_ENABLED=0
RUN test "$(go env GOVERSION)" = "go1.25.8" && go version
WORKDIR /go/src/pax-load
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -mod=readonly -trimpath -buildvcs=false -ldflags='-buildid=' -o /go/bin/paxload ./

FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY --from=build /go/bin/paxload /usr/bin/
ENTRYPOINT ["/usr/bin/paxload"]
