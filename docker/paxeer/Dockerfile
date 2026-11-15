FROM rust:1.91.1-alpine3.22 AS build
WORKDIR /src
RUN apk add --no-cache musl-dev openssl-dev openssl-libs-static pkgconfig
COPY . .
RUN cargo build --locked --release --manifest-path services/platform/Cargo.toml -p layerx-platform-paxeer-boundary

FROM alpine:3.22
RUN apk add --no-cache ca-certificates libgcc openssl \
    && addgroup -g 4020 layerx \
    && adduser -D -H -u 4020 -G layerx layerx \
    && mkdir -p /run/layerx-paxeer \
    && chown 4020:4020 /run/layerx-paxeer \
    && chmod 0700 /run/layerx-paxeer
COPY --from=build /src/services/platform/target/release/layerx-paxeer-boundary /usr/local/bin/layerx-paxeer-boundary
COPY --chmod=0555 docker/common/env-files.sh /usr/local/bin/layerx-env-files
COPY docker/paxeer/files.tsv /etc/layerx/files.tsv
COPY --chmod=0555 docker/paxeer/entrypoint.sh /usr/local/bin/paxeer-entrypoint
EXPOSE 9443
USER 4020:4020
ENTRYPOINT ["/usr/local/bin/paxeer-entrypoint"]
