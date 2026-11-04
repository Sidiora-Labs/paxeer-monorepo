# Isolated Paxeer smoke procedure

Use a disposable Paxeer test namespace and an adapted controller. This procedure
checks supported network, node, workflow and task operations. It does not certify
mainnet operations, governance signing, validator recovery or a store migration.
All placeholders below are public examples; substitute reviewed deployment
values locally and keep credentials and operational evidence private.

## Preflight

Install a qualified `seictl` as described in [the README](../../README.md).
Use kubectl, curl and jq from a host with access to the isolated API server and
the pod proxy DNS network. The namespace must be dedicated to this run and empty
of pre-existing resources with the names below. Use no production keys/accounts.

```sh
set -eu
KUBECONFIG='<isolated-kubeconfig-path>'
export KUBECONFIG
NS=paxeer-smoke
NETWORK=paxeer-smoke
NODE=paxeer-smoke-rpc
CHAIN_ID=paxeer-smoke-1
PAXD_IMAGE='registry.example.invalid/paxd@sha256:<qualified-public-image-digest>'
PROXY_CA='<path-to-isolated-proxy-ca.pem>'
TM_RPC='https://comet.example.invalid'
EVM_RPC='https://evm.example.invalid'
EXPECTED_EVM_CHAIN_ID='<configured-isolated-chain-id-in-hex>'

seictl --help
kubectl config current-context
# Stop unless this is the intended disposable cluster/context.
kubectl get namespace "$NS"
kubectl get crd seinetworks.sei.io seinodes.sei.io seinodetaskworkflows.sei.io
kubectl auth can-i patch seinetworks.sei.io -n "$NS"
kubectl auth can-i patch seinodes.sei.io -n "$NS"
kubectl auth can-i patch seinodetaskworkflows.sei.io -n "$NS"
```

Require `yes` for all permission checks. The served versions must include
`sei.io/v1alpha1` and the controller must be qualified for the chosen Paxeer
image, genesis, fees, address prefixes and proxy isolation. The tool retains Sei
API spellings; it does not translate arbitrary upstream deployments to Paxeer.
Do not proceed while any image, CA, chain identity or controller prerequisite is
unresolved. Namespace creation is deployment setup, not implicit provisioning by
the CLI.

## Network and follower

Render and validate with server-side dry-run first. Inspect the returned CR:
`spec.genesis.chainId` / `spec.chainId`, image digest, namespace and
`metadata.annotations["seictl.sei.io/version"]` must match the reviewed inputs;
release provenance must not be `dev`. `--dry-run` still needs cluster access.

```sh
seictl network apply --kubeconfig "$KUBECONFIG" -n "$NS" --preset genesis-chain --chain-id "$CHAIN_ID" --image "$PAXD_IMAGE" --dry-run "$NETWORK"
seictl node apply --kubeconfig "$KUBECONFIG" -n "$NS" --preset rpc --chain-id "$CHAIN_ID" --image "$PAXD_IMAGE" --network "$NETWORK" --dry-run "$NODE"
```

Once those exact objects are reviewed, create only this isolated network and
follower. Network genesis and replicas are immutable after creation; do not
change them by reapplying a different identity.

```sh
seictl network apply --kubeconfig "$KUBECONFIG" -n "$NS" --preset genesis-chain --chain-id "$CHAIN_ID" --image "$PAXD_IMAGE" "$NETWORK"
seictl network watch --kubeconfig "$KUBECONFIG" -n "$NS" --until Ready --timeout 15m "$NETWORK"
seictl network get --kubeconfig "$KUBECONFIG" -n "$NS" -o json "$NETWORK"
seictl node apply --kubeconfig "$KUBECONFIG" -n "$NS" --preset rpc --chain-id "$CHAIN_ID" --image "$PAXD_IMAGE" --network "$NETWORK" "$NODE"
seictl node watch --kubeconfig "$KUBECONFIG" -n "$NS" --until Running --timeout 15m "$NODE"
seictl node get --kubeconfig "$KUBECONFIG" -n "$NS" -o json "$NODE"
seictl network list --kubeconfig "$KUBECONFIG" -n "$NS"
seictl node list --kubeconfig "$KUBECONFIG" -n "$NS"
```

`SeiNetwork` reaches `Ready`; `SeiNode` reaches `Running`. Either is insufficient
to prove chain identity. Resolve the two RPC URLs above to this isolated
follower using the deployment's approved routing/port-forward procedure, then
check both identities and progress explicitly:

```sh
curl --fail --silent --show-error "$TM_RPC/status" | jq -e --arg chain "$CHAIN_ID" '.result.node_info.network == $chain and .result.sync_info.catching_up == false and (.result.sync_info.latest_block_height | tonumber) > 1'
curl --fail --silent --show-error -H 'Content-Type: application/json' -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}' "$EVM_RPC" | jq -e --arg chain "$EXPECTED_EVM_CHAIN_ID" '.result == $chain and .error == null'
HEIGHT_BEFORE=$(curl --fail --silent --show-error "$TM_RPC/status" | jq -er '.result.sync_info.latest_block_height | tonumber')
sleep 2
curl --fail --silent --show-error "$TM_RPC/status" | jq -e --argjson before "$HEIGHT_BEFORE" '(.result.sync_info.latest_block_height | tonumber) > $before'
```

Stop on a mismatched chain or lack of progress. Do not point these checks at a
public production chain to make the smoke pass.

## Workflow validation and a real read-only task

Validate the supported StateSync CR without executing a data wipe. Confirm
`kind=SeiNodeTaskWorkflow`, `spec.kind=StateSync` and the exact target reference.
This is a rendering/admission check only; it does not prove controller execution
or live witness quorum. Destructive state-sync qualification is a separate
procedure described in [workflow recovery guidance](../../workflow/README.md).

```sh
seictl workflow apply --kubeconfig "$KUBECONFIG" -n "$NS" --preset state-sync --target "$NODE" --dry-run paxeer-smoke-state-sync
seictl workflow list --kubeconfig "$KUBECONFIG" -n "$NS"
```

The task client must run where the pod's DNS resolves and TLS :8443 is reachable.
Use kubeconfig credentials with proxy RBAC and the proxy CA. Its TokenReview and
SubjectAccessReview authenticate/authorize the caller; do not forge identity
headers or bypass TLS validation. The sidecar stays on loopback :7777 in
trusted-header mode. The four probe/metrics bypass paths do not grant task access.

```sh
TASK_ID=$(seictl task submit --kubeconfig "$KUBECONFIG" -n "$NS" --node "$NODE" --proxy-ca-file "$PROXY_CA" config-validate | jq -er '.id')
seictl task get --kubeconfig "$KUBECONFIG" -n "$NS" --node "$NODE" --proxy-ca-file "$PROXY_CA" "$TASK_ID"
seictl task list --kubeconfig "$KUBECONFIG" -n "$NS" --node "$NODE" --proxy-ca-file "$PROXY_CA"
```

Poll `task get` for that same ID until `status=completed`; fail this smoke on
`failed`, a non-empty error or a two-minute timeout. Submission only admits the
operation. Inspect the completed record before cleanup; keep its private
revision/command evidence. Tasks run once; no built-in cron scheduler retries a
failure. HTTP 429 means capacity exhausted, 503 means recovery state is
unavailable, and 401 means the proxy identity boundary failed. Resolve the cause
instead of inventing a new ID and duplicating work.

## Scoped cleanup and sign-off

Delete only the recorded task and objects created by this procedure after their
results are inspected. Workflow dry-run created no persistent workflow. Inspect
remaining resources and controller-managed storage before deciding whether the
dedicated namespace may be deleted under the deployment's cleanup policy.

```sh
seictl task delete --kubeconfig "$KUBECONFIG" -n "$NS" --node "$NODE" --proxy-ca-file "$PROXY_CA" "$TASK_ID"
seictl node delete --kubeconfig "$KUBECONFIG" -n "$NS" "$NODE"
seictl network delete --kubeconfig "$KUBECONFIG" -n "$NS" "$NETWORK"
kubectl get seinetworks,seinodes,seinodetaskworkflows,pods,pvc -n "$NS"
```

Sign off only this isolated command smoke when both chain identities match,
blocks advance, dry-runs use current resource kinds, the read-only task completes
and scoped cleanup is observed. Retain release version, source revision and
private evidence locations; never publish kubeconfig, keys or operational logs.
