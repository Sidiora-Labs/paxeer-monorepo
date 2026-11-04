import copy
from pathlib import Path

ROOT = Path(__file__).resolve().parents[4]
DIRECTORY = Path(__file__).resolve().parent
script = (DIRECTORY / 'topology-check.sh').read_text()
code = script.split("<<'PY'\n", 1)[1].split('\nPY\n', 1)[0]
module = {}
exec(compile(code.rsplit('sys.exit(main())', 1)[0], str(DIRECTORY / 'topology-check.sh'), 'exec'), module)
PARSERS = ('load_pyyaml', 'load_builtin')
NATIVE_MANIFEST = ROOT / 'services/platform/hosted/node/deployment.yaml'
ADAPTER_FIXTURE = DIRECTORY / 'fixtures/native-rpc-topology.yaml'
RESOLVED_EDGES = (
    'Deployment layerx-testnet/layerx-gateway -> native-rpc.layerx-testnet.svc:8545 [env LAYERX_GATEWAY_PAXEER_RPC_URL]',
    'Deployment layerx-testnet/layerx-gateway -> native-rpc.layerx-testnet.svc:8545 [env LAYERX_GATEWAY_TENANT_RPC_URL]',
    'Deployment layerx-testnet/layerx-testnet-control -> native-rpc.layerx-testnet.svc:8545 [env LAYERX_TESTNET_PAXEER_RPC_URL]',
    'Deployment layerx-testnet/layerx-program-registry -> native-rpc.layerx-testnet.svc:8545 [env LAYERX_REGISTRY_CHAIN_RPC_URL]',
    'Deployment layerx-developer/native-client -> native-rpc.layerx-testnet.svc:8545 [env LAYERX_TENANT_RPC_URL (ConfigMap native-client/rpc-url)]',
)


def load(parser, adapters=True):
    topology = module['Topology']()
    manifests = [(NATIVE_MANIFEST, 'incorrect-default')]
    if adapters:
        manifests.append((ADAPTER_FIXTURE, 'layerx-developer'))
    for path, namespace in manifests:
        for document in module[parser](path.read_text()):
            topology.add(document, str(path), namespace)
    return topology


def failures(topology):
    return [row for row in module['check'](topology) if row[0] == 'FAIL']


def labelled(rows, status):
    return [row[1] for row in rows if row[0] in (status if isinstance(status, tuple) else (status,))]


def named(rows, status, expected, description):
    observed = labelled(rows, status)
    assert len(observed) == len(set(observed)), (description, 'duplicate edge', observed)
    assert sorted(observed) == sorted(expected), (
        description,
        'unnamed: %s' % [edge for edge in observed if edge not in expected],
        'named but absent: %s' % [edge for edge in expected if edge not in observed],
    )
    assert len(observed) == len(expected), (description, len(observed), len(expected))


def native_baseline():
    outcomes = []
    for parser in PARSERS:
        base = load(parser, adapters=False)
        assert not failures(base) and not base.problems, (parser, failures(base), base.problems)
        assert [workload['name'] for workload in base.workloads] == ['layerx-node']
        assert all(workload['ns'] == 'layerx-testnet' for workload in base.workloads)
        assert base.configmaps[('layerx-testnet', 'layerx-node-config')]['rpc-origin'] == 'http://layerx-native-chain.layerx-testnet.svc:8545'
        topology = load(parser)
        rows = module['check'](topology)
        assert not failures(topology) and not topology.problems and not topology.notes, (parser, rows, topology.problems, topology.notes)
        named(rows, 'ok', RESOLVED_EDGES, 'native root/tenant RPC edges (%s)' % parser)
        assert len(rows) == len(RESOLVED_EDGES), rows
        assert all('8545/TCP' in row[2] and 'layerx-native-chain' in row[2] for row in rows), rows
        client = next(workload for workload in topology.workloads if workload['name'] == 'native-client')
        assert client['ns'] == 'layerx-developer', client
        outcomes.append(rows)
    assert outcomes[0] == outcomes[1], outcomes
    print('PASS native root/tenant RPC topology and full-manifest parser parity')


def producer_edges():
    for parser in PARSERS:
        for name, expected_count in (('layerx-native-chain', 5), ('native-adapter-egress', 4), ('native-client-egress', 1)):
            topology = load(parser)
            policy = next(item for item in topology.policies if item['name'] == name)
            direction = 'ingress' if name == 'layerx-native-chain' else 'egress'
            policy[direction] = []
            refused = failures(topology)
            assert len(refused) == expected_count, (parser, name, refused)
            assert all(direction + ' NetworkPolicy' in row[2] for row in refused), refused
            named(module['check'](topology), 'ok',
                  tuple(edge for edge in RESOLVED_EDGES if edge not in labelled(refused, 'FAIL')),
                  'remaining native RPC edges')
            print('PASS producer policy refusal:', parser, name)


def topology_refusals():
    for parser in PARSERS:
        for change, expected, count in (
            ('cycle', 'ExternalName cycle', 5),
            ('missing', 'ExternalName target Service', 5),
            ('offcluster', 'has no declared in-cluster Service', 5),
            ('port', 'exposes 444', 5),
            ('selector', 'selects no workload', 5),
            ('target', 'targetPort missing', 5),
            ('ingress', 'ingress NetworkPolicy', 5),
            ('egress', 'egress NetworkPolicy', 1),
            ('dns', 'does not admit DNS', 4),
        ):
            topology = load(parser)
            alias = topology.services[('layerx-testnet', 'native-rpc')]
            target = topology.services[('layerx-testnet', 'layerx-native-chain')]
            if change == 'cycle':
                alias['externalName'] = 'native-rpc.layerx-testnet.svc'
            elif change == 'missing':
                del topology.services[('layerx-testnet', 'layerx-native-chain')]
            elif change == 'offcluster':
                alias['externalName'] = 'absent.example.invalid'
            elif change == 'port':
                target['ports'][0]['port'] = '444'
            elif change == 'selector':
                target['selector'] = {'app': 'absent'}
            elif change == 'target':
                target['ports'][0]['targetPort'] = 'missing'
            elif change == 'ingress':
                next(p for p in topology.policies if p['name'] == 'layerx-native-chain')['ingress'] = []
            elif change == 'egress':
                next(p for p in topology.policies if p['name'] == 'native-client-egress')['egress'] = []
            else:
                policy = next(p for p in topology.policies if p['name'] == 'native-adapter-egress')
                policy['egress'][0]['to'][0]['podSelector']['matchLabels']['k8s-app'] = 'not-dns'
            result = failures(topology)
            assert len(result) == count and all(expected in row[2] for row in result), (parser, change, result)
            assert len(labelled(module['check'](topology), 'ok')) == len(RESOLVED_EDGES) - count
            print('PASS refusal:', parser, change)


def configmap_refusals():
    for parser in PARSERS:
        topology = load(parser)
        client = next(workload for workload in topology.workloads if workload['name'] == 'native-client')
        name, reference = client['env'][0]
        topology.configmaps[('layerx-developer', 'native-client')].pop('rpc-url')
        client['env'][0] = (name, reference[:3] + (True,))
        assert list(topology.resolved_env(client)) == [] and not topology.problems
        client['env'][0] = (name, reference[:3] + (False,))
        assert list(topology.resolved_env(client)) == []
        assert len(topology.problems) == 1 and 'references key rpc-url missing from ConfigMap native-client' in topology.problems[0]
        print('PASS optional and required ConfigMap references:', parser)


def native_container_parity():
    parsed = []
    for parser in PARSERS:
        documents = module[parser](NATIVE_MANIFEST.read_text())
        stateful = next(doc for doc in documents if doc['kind'] == 'StatefulSet')
        spec = stateful['spec']['template']['spec']
        assert [container['name'] for container in spec['containers']] == ['paxd']
        paxd = spec['containers'][0]
        assert paxd['command'] == ['python3', '/opt/layerx/kernel_runtime.py', '--home', '/data/layerx/node']
        assert {port['name']: str(port['containerPort']) for port in paxd['ports']} == {
            'p2p': '26656', 'rpc': '8545', 'consensus': '26657', 'health': '9460'}
        env = {entry['name']: entry for entry in paxd['env']}
        assert env['LAYERX_KERNEL_PROFILE']['value'] == 'native'
        assert 'LAYERX_NODE_HANDOVER_AUTHORITY_PUBLIC_KEY' not in env
        assert not any('RELAY' in name for name in env)
        home = next(container for container in spec['initContainers'] if container['name'] == 'native-home')
        assert home['command'] == ['python3', '-c'] and len(home['args']) == 1
        assert "manifest['schema'] == 'layerx.beta.native-material.v1'" in home['args'][0]
        assert "retained = home / '.native-material.json'" in home['args'][0]
        assert 'securityContext' in home and 'volumeMounts' in home
        assert {mount['name'] for mount in home['volumeMounts']} == {'data', 'native-inputs'}
        # Scalar typing differs between YAML parsers; the literal block must not.
        parsed.append(home['args'])
    assert parsed[0] == parsed[1], parsed
    print('PASS native-home sequence literal block and following fields')


def short_dns_refusals():
    # Both actual YAML parsers feed the production resolver and policy checker.
    fixture = """
apiVersion: v1
kind: Service
metadata:
  name: native
  namespace: beta
spec:
  selector:
    app: native
  ports:
    - port: 8545
      targetPort: rpc
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: native
  namespace: beta
spec:
  template:
    metadata:
      labels:
        app: native
    spec:
      containers:
        - name: paxd
          ports:
            - name: rpc
              containerPort: 8545
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: caller
  namespace: beta
spec:
  template:
    metadata:
      labels:
        app: caller
    spec:
      containers:
        - name: client
          env:
            - name: RPC
              value: http://native:8545
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: native-deny
  namespace: beta
spec:
  podSelector:
    matchLabels:
      app: native
  policyTypes:
    - Ingress
  ingress: []
"""
    outcomes = []
    for parser in ('load_pyyaml', 'load_builtin'):
        topology = module['Topology']()
        for document in module[parser](fixture):
            topology.add(document, 'short-dns-fixture.yaml', 'incorrect-default')
        for host in ('native', 'native.beta', 'native.beta.svc', 'native.beta.svc.cluster.local.'):
            changed = copy.deepcopy(topology)
            caller = next(workload for workload in changed.workloads if workload['name'] == 'caller')
            caller['env'] = [('RPC', 'http://%s:8545' % host)]
            refused = failures(changed)
            assert len(refused) == 1 and 'ingress NetworkPolicy' in refused[0][2], (parser, host, refused)
            changed.policies = []
            assert not failures(changed), (parser, host, failures(changed))
            changed.policies = [dict(name='caller-deny', ns='beta',
                                     selector={'matchLabels': {'app': 'caller'}},
                                     types=['Egress'], ingress=[], egress=[])]
            assert 'egress NetworkPolicy' in failures(changed)[0][2], (parser, host)
            changed.policies[0]['egress'] = [{'ports': [{'port': 8545, 'protocol': 'TCP'}]}]
            assert 'does not admit DNS' in failures(changed)[0][2], (parser, host)
            changed.policies[0]['egress'].append({'ports': [{'port': 53, 'protocol': 'UDP'}]})
            assert not failures(changed), (parser, host, failures(changed))
        outcomes.append(module['check'](topology))
        print('PASS short Service DNS ingress, egress and DNS refusals:', parser)
    assert outcomes[0] == outcomes[1], outcomes


def naming_program_order():
    cluster_script = (ROOT / 'services/platform/hosted/tests/beta-cluster.sh').read_text()
    provision_script = (ROOT / 'services/platform/hosted/human/provision.sh').read_text()
    assert 'source "$REPO_ROOT/services/platform/hosted/human/provision.sh"\n' in cluster_script
    assert cluster_script.count('\nnaming_program_deploy() {\n') == 1
    evidence = provision_script.split('\nhuman_evidence_provision() (\n', 1)[1].split('\n)\n', 1)[0].split('\n')
    ordered = [evidence.index('    ' + step) for step in (
        'human_native_provision', 'naming_program_deploy', 'registry_deployment_produce', 'human_journal_deploy')]
    assert ordered == sorted(ordered) and len(set(ordered)) == 4, (
        'human_evidence_provision must call naming_program_deploy after human_native_provision, whose owner '
        'admission requires a fresh native genesis head that a committed deployment advances, and before '
        'human_journal_deploy, whose single registry journal export must hold the naming deployment pair', ordered)
    assert evidence.count('    naming_program_deploy') == 1
    calls = [line.strip() for line in cluster_script.split('\n')
             if line.strip() in ('naming_program_deploy', 'human_evidence_provision')]
    assert calls == [], ('native beta bring-up must not invoke the unmigrated Human deployment', calls)
    assert '\nbeta_cluster_up() {\n    native_cluster_up "$1"\n}\n' in cluster_script
    native_script = (DIRECTORY / 'beta-native.sh').read_text()
    bring_up = native_script.split('\nnative_cluster_up() {\n', 1)[1].split('\n}\n', 1)[0]
    steps = [bring_up.index('    ' + step + '\n') for step in (
        'manifests_render', 'native_receipt_ready', 'material_save')]
    assert steps == sorted(steps) and len(set(steps)) == 3, steps
    assert 'kube apply -f "$MANIFESTS_DIR/node.yaml"' in bring_up
    print('PASS naming program deployment follows the fresh-genesis-head owner admission and precedes the registry journal export')


def main():
    short_dns_refusals()
    native_baseline()
    producer_edges()
    topology_refusals()
    configmap_refusals()
    native_container_parity()
    naming_program_order()
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
