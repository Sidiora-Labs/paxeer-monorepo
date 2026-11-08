#!/usr/bin/env python3
import copy
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[5]
CHECKER = ROOT / 'services/platform/hosted/tests/topology-check.sh'
code = CHECKER.read_text().split("<<'PY'\n", 1)[1].split('\nPY\n', 1)[0]
module = {}
exec(compile(code.rsplit('sys.exit(main())', 1)[0], str(CHECKER), 'exec'), module)
NAMESPACE = 'layerx-testnet'
SERVICES = {'layerx-native-chain': ('8545', 'rpc', '8545')}
MANIFESTS = ('node/deployment.yaml', 'identity/deployment.yaml', 'paxeer/deployment.yaml',
             'testnet/deployment.yaml', 'gateway/deployment.yaml', 'registry/deployment.yaml',
             'human/deployment.yaml', 'human/web-deployment.yaml', 'internal/deployment.yaml',
             'webhooks/deployment.yaml')


def load():
    topology = module['Topology']()
    documents = []
    for name in MANIFESTS:
        path = ROOT / 'services/platform/hosted' / name
        namespace = 'layerx-developer' if name.startswith('webhooks/') else 'default'
        for document in module['load_builtin'](path.read_text()):
            topology.add(document, str(path), namespace)
            documents.append(document)
    return topology, documents


def node_rows(topology):
    caller = next(workload for workload in topology.workloads
                  if workload['ns'] == NAMESPACE and workload['name'] == 'layerx-testnet-control')
    origin = topology.configmaps[(NAMESPACE, 'layerx-node-config')]['rpc-origin']
    edge = topology.url_edge(caller, 'ConfigMap layerx-node-config/rpc-origin', origin, False)
    assert module['resolve_service_host'](edge['host'], caller['ns'], set()) == (
        (NAMESPACE, 'layerx-native-chain'), 'service')
    label = module['describe'](caller) + ' -> ' + edge['host'] + ':' + edge['port']
    try:
        outcomes = module['check_edge'](topology, edge, set())
    except ValueError as error:
        outcomes = [('FAIL', str(error))]
    return [(status, label, detail) for status, detail in outcomes]


class NodeTopologyTest(unittest.TestCase):
    def setUp(self):
        self.topology, self.documents = load()
        self.node = next(workload for workload in self.topology.workloads
                         if workload['ns'] == NAMESPACE and workload['name'] == 'layerx-node')

    def test_tracked_paxeer_boundary_and_observer_listeners(self):
        self.assertIn('paxeer/deployment.yaml', MANIFESTS)
        paxeer = next(workload for workload in self.topology.workloads
                      if workload['ns'] == NAMESPACE and workload['name'] == 'paxeer')
        caller = next(workload for workload in self.topology.workloads
                      if workload['ns'] == NAMESPACE and workload['name'] == 'layerx-testnet-control')
        for name, target, listener in (('paxeer-boundary', 'https', '9443'),
                                       ('paxeer-observer-boundary', 'observer-https', '9444')):
            with self.subTest(service=name):
                service = self.topology.services[(NAMESPACE, name)]
                self.assertEqual(service['selector'], {'app': 'paxeer'})
                self.assertEqual(service['ports'][0]['port'], '9443')
                self.assertEqual(service['ports'][0]['targetPort'], target)
                self.assertEqual(paxeer['ports'][target], (listener, 'TCP'))
                self.assertTrue(self.topology.ingress_admits(paxeer, listener, 'TCP', caller)[0])
                self.assertTrue(self.topology.egress_admits(
                    caller, NAMESPACE, paxeer['labels'], listener, 'TCP', paxeer['ports'])[0])

    def test_configured_node_edges_and_both_policies(self):
        rows = node_rows(self.topology)
        self.assertTrue(rows)
        self.assertTrue(all(row[0] == 'ok' for row in rows), rows)
        for name in SERVICES:
            self.assertTrue(any(' -> ' + name + '.' in row[1] for row in rows), name)
        for name in ('layerx-testnet-control',):
            self.assertTrue(any(NAMESPACE + '/' + name + ' -> ' in row[1] for row in rows), name)
        for row in rows:
            self.assertIn('ingress admitted by ', row[2])
            self.assertIn('egress admitted by ', row[2])

    def test_service_ports_select_the_actual_node_listeners(self):
        for name, (port, target, listener) in SERVICES.items():
            with self.subTest(service=name):
                service = self.topology.services[(NAMESPACE, name)]
                self.assertEqual(service['selector'], {'app': 'layerx-node'})
                self.assertEqual(service['type'], 'ClusterIP')
                self.assertEqual(service['ports'], [
                    {'name': name, 'port': port, 'targetPort': name, 'protocol': 'TCP'}
                    for name, port in (('rpc', '8545'), ('consensus', '26657'), ('health', '9460'))])
                self.assertEqual(self.node['ports'][target], (listener, 'TCP'))
        for (namespace, name), service in self.topology.services.items():
            if namespace == NAMESPACE and service['selector'] == {'app': 'layerx-node'}:
                for port in service['ports']:
                    self.assertNotIn(port['port'], ('9401', '9402'), name)
                    self.assertNotIn(port['targetPort'], ('9401', '9402'), name)

    def test_wrong_service_port_is_refused(self):
        for name in SERVICES:
            with self.subTest(service=name):
                topology = copy.deepcopy(self.topology)
                topology.services[(NAMESPACE, name)]['ports'][0]['port'] = '1'
                rows = [row for row in node_rows(topology) if ' -> ' + name + '.' in row[1]]
                self.assertTrue(rows)
                self.assertTrue(all(row[0] == 'FAIL' and 'exposes' in row[2] for row in rows), rows)

    def test_wrong_service_selector_is_refused(self):
        for name in SERVICES:
            with self.subTest(service=name):
                topology = copy.deepcopy(self.topology)
                topology.services[(NAMESPACE, name)]['selector'] = {'app': 'absent-node'}
                rows = [row for row in node_rows(topology) if ' -> ' + name + '.' in row[1]]
                self.assertTrue(rows)
                self.assertTrue(all(row[0] == 'FAIL' and 'selects no workload' in row[2] for row in rows), rows)

    def test_wrong_listener_is_refused(self):
        for name in SERVICES:
            with self.subTest(service=name):
                topology = copy.deepcopy(self.topology)
                topology.services[(NAMESPACE, name)]['ports'][0]['targetPort'] = 'absent-listener'
                rows = [row for row in node_rows(topology) if ' -> ' + name + '.' in row[1]]
                self.assertTrue(rows)
                self.assertTrue(all(row[0] == 'FAIL' and 'not a container port' in row[2] for row in rows), rows)

    def test_node_ingress_denial_is_refused(self):
        topology = copy.deepcopy(self.topology)
        policies = topology.selected_policies(NAMESPACE, self.node['labels'], 'Ingress')
        self.assertTrue(policies)
        for policy in policies:
            policy['ingress'] = []
        rows = node_rows(topology)
        self.assertTrue(rows)
        self.assertTrue(all(row[0] == 'FAIL' and 'ingress NetworkPolicy' in row[2] for row in rows), rows)

    def test_caller_egress_denial_is_refused(self):
        topology = copy.deepcopy(self.topology)
        callers = [workload for workload in topology.workloads
                   if workload['ns'] == NAMESPACE and workload['name'] == 'layerx-testnet-control']
        self.assertTrue(callers)
        for caller in callers:
            policies = topology.selected_policies(caller['ns'], caller['labels'], 'Egress')
            self.assertTrue(policies, caller['name'])
            for policy in policies:
                policy['egress'] = []
        rows = node_rows(topology)
        self.assertTrue(rows)
        self.assertTrue(all(row[0] == 'FAIL' and 'egress NetworkPolicy' in row[2] for row in rows), rows)

    def test_pod_storage_and_native_role_isolation(self):
        node = next(document for document in self.documents if document.get('kind') == 'StatefulSet'
                    and document['metadata']['name'] == 'layerx-node')
        self.assertEqual(node['metadata']['namespace'], NAMESPACE)
        template = node['spec']['template']
        self.assertEqual(template['metadata']['labels']['layerx-plane'], 'native-chain')
        pod = template['spec']
        self.assertEqual(module['text'](pod.get('hostNetwork', False)), 'false')
        self.assertEqual(module['text'](pod['shareProcessNamespace']), 'false')
        self.assertEqual(module['text'](pod['automountServiceAccountToken']), 'false')
        self.assertEqual(str(pod['securityContext']['runAsUser']), '4020')
        volumes = {volume['name']: volume for volume in pod['volumes']}
        self.assertEqual(volumes['native-inputs']['secret'],
                         {'secretName': 'layerx-native-inputs', 'defaultMode': '0440'})
        self.assertIn('emptyDir', volumes['tmp'])
        claims = {claim['metadata']['name']: claim for claim in node['spec']['volumeClaimTemplates']}
        self.assertEqual(set(claims), {'data'})
        self.assertEqual(claims['data']['spec']['accessModes'], ['ReadWriteOnce'])
        containers = {container['name']: container for container in pod['containers']}
        self.assertEqual(set(containers), {'paxd'})
        self.assertEqual([container['name'] for container in pod['initContainers']], ['native-home'])
        container = containers['paxd']
        self.assertEqual(container['command'],
                         ['python3', '/opt/layerx/kernel_runtime.py', '--home', '/data/layerx/node'])
        self.assertNotIn('args', container)
        mounts = {mount['name']: mount for mount in container['volumeMounts']}
        self.assertEqual(mounts['data']['mountPath'], '/data')
        self.assertEqual(mounts['tmp']['mountPath'], '/tmp')
        self.assertNotIn('native-inputs', mounts)
        for item in pod['containers'] + pod['initContainers']:
            security = item['securityContext']
            self.assertEqual(module['text'](security['allowPrivilegeEscalation']), 'false')
            self.assertEqual(module['text'](security['readOnlyRootFilesystem']), 'true')
            self.assertEqual(security['capabilities']['drop'], ['ALL'])
        self.assertEqual({port['name']: str(port['containerPort']) for port in container['ports']},
                         {'p2p': '26656', 'rpc': '8545', 'consensus': '26657', 'health': '9460'})
        for probe in ('readinessProbe', 'startupProbe'):
            self.assertEqual(container[probe]['httpGet'], {'path': '/readyz', 'port': 'health'})
        gateway = next(document for document in self.documents
                       if document.get('kind') == 'Deployment'
                       and document['metadata']['name'] == 'layerx-gateway')
        self.assertNotEqual(gateway['spec']['template']['metadata']['labels']['app'], 'layerx-node')



if __name__ == '__main__':
    unittest.main()
