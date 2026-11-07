#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"
# Documentation generation is separate from Rust compilation. Install protoc
# and protoc-gen-openapiv2 v2.18.1 to regenerate the committed API description.
protoc -I proto \
  --openapiv2_out=swagger \
  --openapiv2_opt=grpc_api_configuration=proto/api_config_http.yaml,output_format=yaml,allow_merge=true,merge_file_name=sig-provider \
  proto/sig-provider.proto proto/health.proto
