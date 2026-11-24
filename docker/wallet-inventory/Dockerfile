FROM golang:1.25.8-bookworm@sha256:4557cf171e3cdf5053a298d5171b1a5f5734d920260c25f22c79e94760eb2084 AS build
WORKDIR /src
COPY . .
WORKDIR /src/services/wallet/attestor
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/inventory ./cmd/inventory

FROM debian:12.13-slim@sha256:67b30a61dc87758f0caf819646104f29ecbda97d920aaf5edc834128ac8493d3
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd -g 65532 nonroot \
    && useradd -u 65532 -g 65532 -M -d /nonexistent -s /usr/sbin/nologin nonroot \
    && mkdir -p /run/secrets /data \
    && chown 65532:65532 /run/secrets /data \
    && chmod 0700 /run/secrets \
    && mkdir -p /etc/layerx
COPY --chmod=0555 docker/common/env-files.sh /usr/local/bin/layerx-env-files
RUN printf '%s\t%s\t%s\t%s\n' \
    'WALLET_INVENTORY_PRIVATE_KEY' '/run/secrets/inventory.key' '0600' '*' \
    'WALLET_INVENTORY_ADMIN_TOKEN' '/run/secrets/admin.token' '0600' '*' \
    > /etc/layerx/files.tsv
COPY --from=build /out/inventory /usr/local/bin/inventory
USER nonroot:nonroot
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/usr/local/bin/layerx-env-files", "/usr/local/bin/inventory"]
CMD ["serve"]
