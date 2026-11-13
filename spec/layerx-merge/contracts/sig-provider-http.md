# Signature provider HTTP contract

The [signature provider](../../../services/explorer/services/sig-provider) resolves function/event ABI information and accepts ABI signature registration. It is an explorer component; its responses do not establish transaction execution or finality.

## grpc-abiservice

`blockscout.sig_provider.v1.AbiService` declares `GetFunctionAbi`, `GetEventAbi` and `BatchGetEventAbis` in [sig-provider.proto](../../../services/explorer/services/sig-provider/sig-provider-proto/proto/sig-provider.proto). Function requests carry `tx_input`; event requests carry `data` and comma-separated hexadecimal `topics`. ABI responses carry names and decoded arguments, including tuple components and event indexed flags.

## grpc-signatureservice

`blockscout.sig_provider.v1.SignatureService.CreateSignatures` takes `CreateSignaturesRequest { string abi = 1; }` and returns an empty response.

## grpc-health

`blockscout.sig_provider.v1.Health.Check` takes a service string and returns the health status enum. The [health schema](../../../services/explorer/services/sig-provider/sig-provider-proto/proto/health.proto) declares unary Check; Watch is not declared as an enabled RPC.

## sig-provider-function

`GET /api/v1/abi/function` maps to `GetFunctionAbi`, exposing the `abi` response field.

## sig-provider-event

`GET /api/v1/abi/event` maps to `GetEventAbi`, exposing the `abi` response field.

## sig-provider-events

`POST /api/v1/abi/events:batch-get` maps the JSON body to `BatchGetEventAbis`, exposing the `responses` field.

## sig-provider-signatures

`POST /api/v1/signatures` maps the JSON body to `CreateSignatures`.

## sig-provider-health

`GET /health` maps to `Health.Check`. These HTTP mappings are defined in [api_config_http.yaml](../../../services/explorer/services/sig-provider/sig-provider-proto/proto/api_config_http.yaml) and use the configured service origin.
