FROM rust:1.91.1-alpine3.22 AS build
WORKDIR /src
RUN apk add --no-cache musl-dev openssl-dev openssl-libs-static pkgconfig
COPY . .
RUN cargo build --locked --release --manifest-path services/platform/Cargo.toml -p layerx-platform-faucet

FROM alpine:3.22
RUN apk add --no-cache ca-certificates libgcc openssl su-exec \
    && addgroup -g 4020 layerx \
    && adduser -D -H -u 4020 -G layerx layerx \
    && mkdir -p /run/layerx
COPY --from=build /src/services/platform/target/release/layerx-faucet /usr/local/bin/layerx-faucet
COPY --chmod=0555 docker/common/env-files.sh /usr/local/bin/layerx-env-files
COPY docker/platform-faucet/files.tsv /etc/layerx/files.tsv
COPY --chmod=0555 docker/platform-faucet/entrypoint.sh /usr/local/bin/layerx-faucet-entrypoint
USER 0:0
ENTRYPOINT ["/usr/local/bin/layerx-env-files", "/usr/local/bin/layerx-faucet-entrypoint"]
