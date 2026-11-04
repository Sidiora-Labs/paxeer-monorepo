# We need to build in a Linux environment to support C libraries, e.g. RocksDB.
# We use Debian instead of Alpine, so that we can use binary database packages
# instead of spending time compiling them.
FROM golang:1.25.8-bookworm

RUN apt-get -qq update \
    && apt-get -qq install -y --no-install-recommends libleveldb-dev librocksdb-dev \
    && rm -rf /var/lib/apt/lists/*

# Build with the repository root as the Docker context and Go module root.
WORKDIR /src/paxeer

# Fetch dependencies separately (for layer caching)
COPY go.mod go.sum ./
COPY core/deps/geth/go.mod core/deps/geth/go.sum ./core/deps/geth/
RUN go mod download

# The maintained e2e command embeds both the consensus node and application.
COPY . .
RUN CGO_ENABLED=1 go build -mod=readonly -tags badgerdb,boltdb,cleveldb,rocksdb \
    -o /usr/bin/app ./consensus/test/e2e/node
COPY --chmod=0755 consensus/test/e2e/docker/entrypoint /usr/bin/entrypoint
COPY --chmod=0755 consensus/test/e2e/docker/entrypoint-builtin /usr/bin/entrypoint-builtin

# Set up runtime directory. We don't use a separate runtime image since we need
# e.g. leveldb and rocksdb which are already installed in the build image.
WORKDIR /tendermint
VOLUME ["/tendermint"]
ENV TMHOME=/tendermint

EXPOSE 26656 26657 26660 6060
ENTRYPOINT ["/usr/bin/entrypoint"]
CMD ["start"]
STOPSIGNAL SIGTERM
