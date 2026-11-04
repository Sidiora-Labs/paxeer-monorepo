#!/usr/bin/env bash

source "${REPO_ROOT}/services/platform/hosted/human/onboarding_provision.sh"

human_owner_provision() (
    set -euo pipefail
    umask 077
    local input="$WORK_DIR/human-evidence-input" output="$WORK_DIR/human-owner-result.json"
    local manifest="$WORK_DIR/human-provision-owner-job.json" state="$WORK_DIR/human-provision-node.json"
    [ ! -e "$output" ] || fail "$output: existing owner result requires explicit reconciliation with retained state"
    python3 "$REPO_ROOT/services/platform/hosted/human/provision.py" --validate-job-input --work-dir "$WORK_DIR"
    python3 "$REPO_ROOT/services/platform/hosted/human/provision.py" --account-requests --work-dir "$WORK_DIR"
    kube -n "$TESTNET_NAMESPACE" get statefulset layerx-node -o json > "$state"
    python3 - "$state" <<'PY'
import json
import sys
value = json.load(open(sys.argv[1]))
containers = value['spec']['template']['spec']['containers']
if any(c['name'].startswith('human') for c in containers):
    raise SystemExit('layerx-node: Human runtime must not be enabled during owner provisioning')
PY
    kube -n "$TESTNET_NAMESPACE" get pods -l app=layerx-node -o json > "$state"
    python3 - "$state" <<'PY'
import json
import sys
value = json.load(open(sys.argv[1]))
if len(value['items']) != 1:
    raise SystemExit('layerx-node: exactly one bootstrap pod is required')
pod = value['items'][0]
if any(c['name'].startswith('human') for c in pod['spec']['containers']):
    raise SystemExit('layerx-node: Human runtime pod must be stopped before owner provisioning')
if not pod['spec'].get('nodeName'):
    raise SystemExit('layerx-node: bootstrap pod is not scheduled')
PY
    python3 - "$REPO_ROOT/services/platform/hosted/human/provision-owner-job.yaml" "$manifest" \
        "$TESTNET_NAMESPACE" "$(image_ref layerx-human)" "$state" <<'PY'
import json
import sys
import yaml
value = yaml.safe_load(open(sys.argv[1]))
value['metadata']['namespace'] = sys.argv[3]
pod = value['spec']['template']['spec']
pod['containers'] = [c for c in pod['containers'] if c['name'] != 'validate-account-head']
for container in pod['containers'] + pod['initContainers']:
    container['image'] = sys.argv[4]
pod['nodeName'] = json.load(open(sys.argv[5]))['items'][0]['spec']['nodeName']
with open(sys.argv[2], 'x') as output:
    json.dump(value, output)
PY
    apply_secret "$TESTNET_NAMESPACE" layerx-human-provision-owner-input \
        --from-file=owner-request.json="$input/owner-request.json" \
        --from-file=recovery-policy.json="$input/recovery-policy.json" \
        --from-file=treasury-request.json="$input/treasury-request.json" \
        --from-file=sequencer-request.json="$input/sequencer-request.json"
    kube create -f "$manifest" > /dev/null
    kube -n "$TESTNET_NAMESPACE" wait --for=condition=complete --timeout=150s job/layerx-human-provision-owner > /dev/null \
        || fail 'layerx-human-provision-owner: Job did not complete; owner result not published'
    kube -n "$TESTNET_NAMESPACE" logs job/layerx-human-provision-owner -c provision-owner > "$output.pending"
    python3 "$REPO_ROOT/services/platform/hosted/human/provision.py" --validate-owner-result \
        --work-dir "$WORK_DIR" --request "$output.pending"
    mv "$output.pending" "$output"
    local account
    for account in treasury sequencer; do
        kube -n "$TESTNET_NAMESPACE" logs job/layerx-human-provision-owner -c "provision-$account" > "$input/$account.json"
    done
)

human_journal_materialize() (
    set -euo pipefail
    umask 077
    local stage pod manifest
    stage=$(mktemp -d "$WORK_DIR/.journal-export-XXXXXXXX")
    pod="human-journal-${stage##*-}"
    pod=${pod,,}
    manifest="$stage/pod.json"
    trap 'rm -rf "$stage"' EXIT
    kube -n "$TESTNET_NAMESPACE" get pod layerx-program-registry-0 -o json > "$stage/registry.json"
    python3 - "$stage/registry.json" "$manifest" "$pod" <<'PYJOURNAL'
import json
import sys
source = json.load(open(sys.argv[1]))
spec = source['spec']
registry = next(c for c in spec['containers'] if c['name'] == 'registry')
path = next(e['value'] for e in registry['env'] if e['name'] == 'LAYERX_REGISTRY_JOURNAL')
mount = next(m for m in registry['volumeMounts'] if m['mountPath'] == path)
volume = next(v for v in spec['volumes'] if v['name'] == mount['name'])
if (path != '/var/lib/layerx-registry-journal' or mount.get('subPath') != 'journal'
        or volume['persistentVolumeClaim']['claimName'] != 'layerx-registry-journal'
        or not spec.get('nodeName')):
    raise SystemExit('registry journal PVC binding refused')
value = {'apiVersion': 'v1', 'kind': 'Pod',
    'metadata': {'name': sys.argv[3], 'namespace': source['metadata']['namespace']},
    'spec': {'nodeName': spec['nodeName'], 'restartPolicy': 'Never',
        'automountServiceAccountToken': False, 'activeDeadlineSeconds': 300,
        'securityContext': {'runAsNonRoot': True, 'runAsUser': 4030, 'runAsGroup': 4030},
        'containers': [{'name': 'journal', 'image': registry['image'],
            'imagePullPolicy': registry['imagePullPolicy'], 'command': ['sleep', '300'],
            'securityContext': {'allowPrivilegeEscalation': False, 'readOnlyRootFilesystem': True,
                'capabilities': {'drop': ['ALL']}, 'seccompProfile': {'type': 'RuntimeDefault'}},
            'resources': {'requests': {'cpu': '10m', 'memory': '16Mi'},
                'limits': {'cpu': '100m', 'memory': '64Mi'}},
            'volumeMounts': [dict(mount, readOnly=True)]}],
        'volumes': [{'name': volume['name'], 'persistentVolumeClaim':
            dict(volume['persistentVolumeClaim'], readOnly=True)}]}}
with open(sys.argv[2], 'x') as output:
    json.dump(value, output)
PYJOURNAL
    kube create -f "$manifest" > /dev/null
    trap 'kube -n "$TESTNET_NAMESPACE" delete pod "$pod" --wait=false > /dev/null; rm -rf "$stage"' EXIT
    kube -n "$TESTNET_NAMESPACE" wait --for=condition=Ready "pod/$pod" --timeout=120s > /dev/null
    mkdir -m 0700 "$stage/journal"
    kube -n "$TESTNET_NAMESPACE" cp -c journal "$pod:/var/lib/layerx-registry-journal/pairs/." "$stage/journal"
    python3 "$REPO_ROOT/services/platform/hosted/human/provision.py" --materialize-journal \
        --work-dir "$WORK_DIR" --journal "$stage/journal"
)

human_journal_deploy() (
    set -euo pipefail
    umask 077
    local request="$WORK_DIR/human-evidence-input/program-deployment.lxa"
    local response="$WORK_DIR/registry-deployment-result.json" status
    local provision="$REPO_ROOT/services/platform/hosted/human/provision.py"
    python3 - "$provision" "$request" "$response" <<'PYDEPLOY'
import importlib.util
from pathlib import Path
import sys
spec = importlib.util.spec_from_file_location('provision', sys.argv[1])
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
module.protected_bytes(Path(sys.argv[2]))
module.require(not Path(sys.argv[3]).exists() and not Path(sys.argv[3]).is_symlink(),
               sys.argv[3], 'existing deployment result requires reconciliation')
PYDEPLOY
    port_forward registry "$TESTNET_NAMESPACE" layerx-program-registry 19455 9420
    local field refused="$WORK_DIR/registry-deployment-refused.json"
    for field in record_hex proof_hex; do
        status=$(curl --silent --show-error --max-time 120 --max-filesize 1048576 --noproxy '*' \
            --cacert "$CA_DIR/ca.crt" --cert "$CA_DIR/gateway-client/cert.pem" --key "$CA_DIR/gateway-client/key.pem" \
            --connect-to 'layerx-program-registry:9420:127.0.0.1:19455' \
            --header "Authorization: Bearer $(cat "$SECRETS_DIR/registry-request.token")" \
            --header 'Content-Type: application/octet-stream' --data "{\"$field\":\"00\"}" \
            --output "$refused" --write-out '%{http_code}' \
            'https://layerx-program-registry:9420/__registry/deployments')
        [ "$status" = 503 ] || fail "registry accepted or misclassified caller $field: status $status"
        python3 - "$refused" <<'PYREFUSAL'
import json
import sys
value = json.load(open(sys.argv[1]))
if value.get('error', {}).get('code') != 'deployment_proof_unavailable':
    raise SystemExit('registry caller projection did not reach deployment verification')
PYREFUSAL
    done
    status=$(curl --silent --show-error --max-time 120 --max-filesize 1048576 --noproxy '*' \
        --cacert "$CA_DIR/ca.crt" --cert "$CA_DIR/gateway-client/cert.pem" --key "$CA_DIR/gateway-client/key.pem" \
        --connect-to 'layerx-program-registry:9420:127.0.0.1:19455' \
        --header "Authorization: Bearer $(cat "$SECRETS_DIR/registry-request.token")" \
        --header 'Content-Type: application/octet-stream' --data-binary "@$request" \
        --output "$response" --write-out '%{http_code}' \
        'https://layerx-program-registry:9420/__registry/deployments')
    [ "$status" = 200 ] || fail "registry ingress refused deployment with status $status; evidence not published"
    human_journal_materialize
    python3 - "$provision" "$response" "$WORK_DIR/registry-journal" <<'PYPAIR'
import importlib.util
from pathlib import Path
import sys
spec = importlib.util.spec_from_file_location('provision', sys.argv[1])
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)
response = module.protected_json(Path(sys.argv[2]))
module.fields(response, 'activity_id receipt_digest state', sys.argv[2], 'deployment response')
module.require(response['state'] == 'deployed', sys.argv[2], 'deployed state')
for key in ('activity_id', 'receipt_digest'):
    module.h32(response[key], sys.argv[2], key)
records = module.journal_records(Path(sys.argv[3]))
for suffix in ('.admission', '.deployment'):
    module.require(response['receipt_digest'] + suffix in records, sys.argv[3], 'ingress journal pair')
PYPAIR
)

human_native_owner_prepare() (
    set -euo pipefail
    umask 077
    local input="$WORK_DIR/human-evidence-input"
    python3 - "$input" "$NODE_NETWORK_ID" "$REPO_ROOT" <<'PYMERGED'
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[3]) / 'services/platform/hosted/human'))
from provision import protected_json, require, write_json
root = Path(sys.argv[1])
config = protected_json(root / 'owner-evm.json')
require(config.get('network_id') == int(sys.argv[2]), root, 'merged owner network')
path = root / 'owner-native.json'
if path.exists():
    require(protected_json(path) == config, path, 'identical retained merged owner configuration')
else:
    write_json(path, config)
PYMERGED
)


registry_kernel_material() (
    set -euo pipefail
    umask 077
    local helper="$REPO_ROOT/services/platform/hosted/human/material.py"
    local stage before after selected
    stage=$(mktemp -d "$WORK_DIR/.registry-kernel-XXXXXXXX")
    trap 'rm -rf "$stage"' EXIT
    kube -n "$TESTNET_NAMESPACE" get pod layerx-node-0 -o json > "$stage/before.json"
    before=$(python3 - "$stage/before.json" <<'PY_KERNEL_POD'
import json,sys
p=json.load(open(sys.argv[1]))
c=next(x for x in p['status']['containerStatuses'] if x['name']=='layerxd')
if p['status']['phase']!='Running' or not c.get('ready') or not c.get('imageID') or not p['metadata'].get('uid'):
    raise SystemExit('selected kernel producer pod is not ready')
print(p['metadata']['uid']+'|'+c['imageID'])
PY_KERNEL_POD
)
    kube -n "$TESTNET_NAMESPACE" exec layerx-node-0 -c layerxd -- \
        python3 /usr/local/lib/layerx-human/material.py --export-registry-material \
        /var/lib/layerx/registry-material > "$stage/export.json"
    kube -n "$TESTNET_NAMESPACE" get pod layerx-node-0 -o json > "$stage/after.json"
    after=$(python3 - "$stage/after.json" <<'PY_KERNEL_POD'
import json,sys
p=json.load(open(sys.argv[1]))
c=next(x for x in p['status']['containerStatuses'] if x['name']=='layerxd')
if p['status']['phase']!='Running' or not c.get('ready'):
    raise SystemExit('selected kernel producer changed during transfer')
print(p['metadata']['uid']+'|'+c['imageID'])
PY_KERNEL_POD
)
    [ "$before" = "$after" ] || fail 'kernel producer pod or selected image changed during private transfer'
    python3 "$helper" --import-registry-material "$stage/export.json" "$SECRETS_DIR/registry-kernel" \
        "$NODE_NETWORK_ID" "$NODE_SEQUENCER_ID" "$NODE_SEQUENCER_PUBLIC_KEY" > "$stage/import.json"
    selected=$(python3 - "$stage/import.json" <<'PY_KERNEL_SELECTED'
import json,sys
print(json.load(open(sys.argv[1]))['directory'])
PY_KERNEL_SELECTED
)
    python3 "$helper" --verify-registry-material "$selected" > /dev/null
    apply_secret "$TESTNET_NAMESPACE" layerx-registry-kernel-generation \
        --from-file=generation.json="$selected/generation.json" \
        --from-file=replica-id="$selected/replica-id" --from-file=trust-history="$selected/trust-history"
)

human_evidence_provision() (
    set -euo pipefail
    umask 077
    local provision="$REPO_ROOT/services/platform/hosted/human/provision.py"
    human_native_owner_prepare
    human_native_provision
    python3 "$provision" --movement-source --work-dir "$WORK_DIR" --secrets-dir "$SECRETS_DIR"
    python3 "$provision" --validate-owner-registration --work-dir "$WORK_DIR"
    naming_program_deploy
    registry_deployment_produce
    human_journal_deploy
    python3 "$provision" --producer-manifest --work-dir "$WORK_DIR" \
        --registry "$SECRETS_DIR/module-registry.json" --journal "$WORK_DIR/registry-journal" \
        --output "$WORK_DIR/human-producers.json"
    python3 "$provision" --validate-evidence-inputs --work-dir "$WORK_DIR" \
        --registry "$SECRETS_DIR/module-registry.json" --journal "$WORK_DIR/registry-journal"
    python3 "$provision" --assemble --work-dir "$WORK_DIR" \
        --registry "$SECRETS_DIR/module-registry.json" --asset "$NODE_ASSET_ID" \
        --journal "$WORK_DIR/registry-journal"
    human_policy_graph_inputs
)

# human_policy_graph_inputs: the ordering gate between the evidence producers and the policy and authority
# consumers. The kernel identity generation, module registry, deployment journal and owner/native/naming
# evidence must all be ready in the declared acyclic graph before human_policy_publish assembles the bundle;
# a missing input leaves its dependants waiting and an inconsistent one refuses, naming the producer.
human_policy_graph_inputs() (
    set -euo pipefail
    umask 077
    local status="$WORK_DIR/human-policy-graph.json"
    python3 "$REPO_ROOT/services/platform/hosted/human/material.py" --policy-graph-status \
        "$WORK_DIR/human-evidence" "$WORK_DIR/human-evidence/journal" "$WORK_DIR/paxeer/deployment.json" \
        "$SECRETS_DIR/module-registry.json" "$WORK_DIR/human-policy" "$SECRETS_DIR/registry-kernel" \
        "$WORK_DIR/human-authority-graph" "$NODE_NETWORK_ID" "$PAXEER_CHAIN_ID" > "$status.pending"
    mv "$status.pending" "$status"
    python3 - "$status" <<'PY_POLICY_GRAPH'
import json
import sys
value = json.load(open(sys.argv[1]))
blocked = {name: (node['state'], node['producer'], node['reason']) for name, node in value['nodes'].items()
           if name not in ('assembled-policy', 'role-authority') and node['state'] != 'ready'}
refused = {name: node['reason'] for name, node in value['nodes'].items() if node['state'] == 'refused'}
if blocked or refused:
    raise SystemExit('human policy graph inputs not ready: ' + json.dumps(dict(blocked, **refused), sort_keys=True))
PY_POLICY_GRAPH
)

human_native_provision() (
    set -euo pipefail
    umask 077
    python3 "$REPO_ROOT/services/platform/hosted/human/provision.py" \
        --produce-owner-registration --work-dir "$WORK_DIR"
    python3 "$REPO_ROOT/services/platform/hosted/human/provision.py" \
        --validate-owner-registration --work-dir "$WORK_DIR"
    human_account_head_observe
)

human_account_head_observe() (
    set -euo pipefail
    umask 077
    local input="$WORK_DIR/human-evidence-input" job
    local manifest="$WORK_DIR/human-account-head-job.json"
    python3 - "$REPO_ROOT" "$WORK_DIR" "$manifest" "$TESTNET_NAMESPACE" "$(image_ref layerx-human)" <<'PY_ACCOUNT_HEAD'
import json
import ssl
import sys
from pathlib import Path
import yaml
root, work = Path(sys.argv[1]), Path(sys.argv[2])
sys.path.insert(0, str(root / 'services/platform/hosted/human'))
from provision import protected_bytes, protected_json, require, write_json, owner_policy
from merged_owner import account_id
inputs = work / 'human-evidence-input'
config = protected_json(inputs / 'owner-native.json')
registration = protected_json(inputs / 'owner-registration.json')
owner = protected_json(work / 'human-owner-result.json')
require(config['rpc_url'].startswith('https://') and config['chain_id'] > 0,
        inputs, 'authenticated committed chain endpoint')
require(registration['identity']['did'] == owner['did'] == 'did:layerx:' + registration['authority']
        and registration['owner_account'] == account_id(registration['authority'], config['asset_id']),
        inputs, 'original provisioned owner and merged asset account')
ca = protected_bytes(config['rpc_ca_file'], 65536).decode('ascii')
request = dict(rpc_url=config['rpc_url'], rpc_ca_der_hex=[ssl.PEM_cert_to_DER_cert(ca).hex()],
    chain_id=config['chain_id'], owner_did=owner['did'], owner_account=registration['owner_account'],
    asset_id=config['asset_id'], maximum_age_seconds=owner_policy()['maximum_age_seconds'])
write_json(inputs / 'account-observation-request.json', request)
value = yaml.safe_load((root / 'services/platform/hosted/human/provision-owner-job.yaml').read_text())
value['metadata'].pop('name')
value['metadata']['generateName'] = 'layerx-human-account-head-'
value['metadata']['namespace'] = sys.argv[4]
pod = value['spec']['template']['spec']
validator = next(c for c in pod['containers'] if c['name'] == 'validate-account-head')
validator['image'] = sys.argv[5]
pod['containers'] = [validator]
pod.pop('initContainers')
pod['volumes'] = [dict(name='input', secret=dict(secretName='layerx-human-account-head-input', defaultMode=0o444))]
with open(sys.argv[3], 'w') as output:
    json.dump(value, output)
PY_ACCOUNT_HEAD
    apply_secret "$TESTNET_NAMESPACE" layerx-human-account-head-input \
        --from-file=account-head-request.json="$input/account-observation-request.json"
    job=$(kube create -f "$manifest" -o jsonpath='{.metadata.name}')
    kube -n "$TESTNET_NAMESPACE" wait --for=condition=complete --timeout=150s "job/$job" > /dev/null \
        || fail 'committed owner account observation refused; limits not published'
    kube -n "$TESTNET_NAMESPACE" logs "job/$job" -c validate-account-head > "$input/account-observation.json.pending"
    python3 - "$REPO_ROOT/services/platform/hosted/human" "$input" <<'PY_ACCOUNT_RESULT'
import sys
from pathlib import Path
sys.path.insert(0, sys.argv[1])
from provision import fields, protected_json, require, write_json
root = Path(sys.argv[2])
request = protected_json(root / 'account-observation-request.json')
result = protected_json(root / 'account-observation.json.pending')
fields(result, 'schema chain_id owner_did account asset_id committed_height block_hash state_root observed_at balance consumed',
       root, 'committed account observation')
require(result['schema'] == 'layerx.human.account-head.v1'
        and result['chain_id'] == request['chain_id'] and result['owner_did'] == request['owner_did']
        and result['account'] == request['owner_account'] and result['asset_id'] == request['asset_id']
        and type(result['committed_height']) is int and result['committed_height'] > 0
        and type(result['consumed']) is int and result['consumed'] == 0,
        root, 'verified original owner account and supported unused limit')
(root / 'account-observation.json.pending').replace(root / 'account-observation.json')
write_json(root / 'account-head-result.json', dict(consumed=result['consumed']))
PY_ACCOUNT_RESULT
)
