FROM ghcr.io/blockscout/services-base:chef-0.1.72-rust-1.88-protoc-31.1-openapi-2.27.1@sha256:9b0f40b99a74fe0028bc6b1f800928d15de948fa35e24c780d25c7c5f841c0d4 AS proto-tools

FROM rust:1.91.1-bookworm@sha256:c1e5f19e773b7878c3f7a805dd00a495e747acbdc76fb2337a4ebf0418896b33 AS chef
RUN apt-get update && apt-get install -y --no-install-recommends libssl-dev pkg-config \
    && rm -rf /var/lib/apt/lists/* \
    && cargo install cargo-chef --version 0.1.73 --locked
COPY --from=proto-tools /usr/bin/protoc /usr/local/bin/protoc
COPY --from=proto-tools /usr/bin/protoc-gen-openapiv2 /usr/local/bin/protoc-gen-openapiv2
COPY --from=proto-tools /usr/include/google/protobuf /usr/local/include/google/protobuf
WORKDIR /app

FROM chef AS plan
COPY services/explorer/services/smart-contract-verifier .
RUN cargo chef prepare --recipe-path recipe.json

FROM chef AS cache
COPY --from=plan /app/recipe.json recipe.json
RUN cargo chef cook --release --locked --recipe-path recipe.json

FROM chef AS build

COPY services/explorer/services/smart-contract-verifier .
COPY --from=cache /app/target target
COPY --from=cache $CARGO_HOME $CARGO_HOME
RUN cargo build --release --locked

FROM ubuntu:24.04@sha256:534baea6a22c03a63003dbc8dbe78fe34bc0d7e595d9a9dc9834884ff530eb55 AS run
RUN apt-get update && apt-get install -y --no-install-recommends libssl3 ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
ENV APP_USER=app
# Processes in a container should not run as root, so we need to create app user
# https://medium.com/@mccode/processes-in-containers-should-not-run-as-root-2feae3f0df3b
RUN groupadd $APP_USER \
    && useradd -g $APP_USER $APP_USER

COPY --from=build /app/target/release/smart-contract-verifier-server /app/smart-contract-verifier-server
# Change directory access for app user
RUN chown -R $APP_USER:$APP_USER /app
USER app

CMD ["./smart-contract-verifier-server"]
