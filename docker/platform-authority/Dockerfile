FROM rust:1.91.1-alpine3.22 AS build
WORKDIR /src
ENV CARGO_BUILD_JOBS=4
RUN apk add --no-cache musl-dev openssl-dev openssl-libs-static pkgconfig
COPY . .
RUN cargo build --locked --release --manifest-path services/platform/Cargo.toml -p layerx-platform-authority -p layerx-runtime-clock

FROM alpine:3.22
RUN apk add --no-cache ca-certificates libgcc openssl python3 util-linux setpriv \
    && ln -s /bin/setpriv /usr/bin/setpriv \
    && addgroup -g 4020 layerx && adduser -D -H -u 4020 -G layerx layerx
RUN /usr/bin/setpriv --help | grep -- --pdeathsig
COPY --from=build /src/services/platform/target/release/layerx-receipt-authority /usr/local/bin/layerx-receipt-authority
COPY --from=build /src/services/platform/target/release/layerx-runtime-clock /usr/local/bin/layerx-runtime-clock
COPY --from=build /src/services/platform/hosted/node/generation_client.py /src/services/platform/hosted/node/generation_transport.py /src/services/platform/hosted/node/reset_state.py /opt/layerx/node/
COPY --chmod=0555 docker/common/env-files.sh /usr/local/bin/layerx-env-files
COPY docker/platform-authority/files.tsv /etc/layerx/files.tsv
COPY --chmod=0555 docker/platform-authority/entrypoint.sh /usr/local/bin/authority-entrypoint
RUN mkdir -p /run/layerx-authority \
    && chown 4020:4020 /run/layerx-authority \
    && chmod 0700 /run/layerx-authority
EXPOSE 9445
USER 4020:4020
ENTRYPOINT ["/usr/local/bin/authority-entrypoint"]
