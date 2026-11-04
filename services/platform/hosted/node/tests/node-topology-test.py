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
SERVICES = {
    'layerx-pending-core': ('9443', 'core-tls', '9443'),
    'layerx-pending-core-admin': ('9444', 'core-admin-tls', '9444'),
    'layerx-receipt-authority': ('9443', 'authority-tls', '9445'),
    'layerx-agent-boundary': ('9443', 'agent-tls', '9446'),
}
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
    return [row for row in module['check'](topology)
            if any(' -> ' + name + '.' + NAMESPACE + '.svc' in row[1] for name in SERVICES)]


class NodeTopologyTest(unittest.TestCase):
    def setUp(self):
        self.topology, self.documents = load()
        self.node = next(workload for workload in self.topology.workloads
                         if workload['ns'] == NAMESPACE and workload['name'] == 'layerx-node')

    def test_configured_node_edges_and_both_policies(self):
        rows = node_rows(self.topology)
        self.assertTrue(rows)
        self.assertTrue(all(row[0] == 'ok' for row in rows), rows)
        for name in SERVICES:
            self.assertTrue(any(' -> ' + name + '.' in row[1] for row in rows), name)
        for name in ('layerx-testnet-control', 'layerx-gateway', 'layerx-program-registry'):
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
                self.assertEqual(service['ports'], [{'name': 'https', 'port': port,
                                                     'targetPort': target, 'protocol': 'TCP'}])
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
        callers = [workload for workload in topology.workloads if any(
            edge['kind'] == 'service' and edge['service'] in {(NAMESPACE, name) for name in SERVICES}
            for source, value in topology.resolved_env(workload)
            for edge in [topology.url_edge(workload, source, value.strip(), False)] if edge is not None)]
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
        self.assertEqual(template['metadata']['labels']['layerx-plane'], 'trusted-boundary')
        pod = template['spec']
        self.assertEqual(module['text'](pod.get('hostNetwork', False)), 'false')
        self.assertEqual(str(pod['securityContext']['runAsUser']), '4020')
        volumes = {volume['name']: volume for volume in pod['volumes']}
        self.assertIn('emptyDir', volumes['run'])
        claims = {claim['metadata']['name']: claim for claim in node['spec']['volumeClaimTemplates']}
        self.assertIn('data', claims)
        self.assertEqual(claims['data']['spec']['accessModes'], ['ReadWriteOnce'])
        containers = {container['name']: container for container in pod['containers']}
        self.assertEqual(module['text'](pod['shareProcessNamespace']), 'false')
        self.assertEqual(module['text'](pod['automountServiceAccountToken']), 'false')
        self.assertFalse({'layerxd', 'layerxd-authority', 'paxeer-relay'} & containers.keys())
        container = containers['paxd']
        self.assertEqual(container['command'], ['/opt/layerx/supervisor.sh'])
        args = container['args']
        self.assertNotIn('--role', args)
        self.assertNotIn('--lni-uid', args)
        self.assertNotIn('--program-port', args)
        self.assertNotIn('--replica-port', args)
        self.assertEqual(args[args.index('--data-dir') + 1], '/var/lib/layerx/node')
        self.assertEqual(args[args.index('--run-dir') + 1], '/run/layerx/node')
        self.assertEqual(args[args.index('--state-dir') + 1], '/var/lib/layerx/supervisor-state')
        mounts = {mount['name']: mount for mount in container['volumeMounts']}
        self.assertEqual(mounts['data']['mountPath'], '/var/lib/layerx')
        self.assertEqual(mounts['run']['mountPath'], '/run/layerx')
        self.assertEqual(module['text'](mounts['native-private']['readOnly']), 'true')
        self.assertIn('--treasury-signer-socket', args)
        self.assertNotIn('--treasury-key', args)
        self.assertNotIn('--sequencer-key', args)
        self.assertEqual(str(containers['core-boundary']['securityContext']['runAsUser']), '4021')
        for name in ('paxd', 'core-boundary', 'treasury-signer'):
            security = containers[name]['securityContext']
            self.assertEqual(module['text'](security['allowPrivilegeEscalation']), 'false')
            self.assertEqual(module['text'](security['readOnlyRootFilesystem']), 'true')
            self.assertEqual(security['capabilities']['drop'], ['ALL'])
        self.assertEqual({port['name']: str(port['containerPort']) for port in container['ports']},
                         {'chain-p2p': '26656', 'chain-rpc': '8545', 'consensus-rpc': '26657'})
        native_service = self.topology.services[(NAMESPACE, 'layerx-native-chain')]
        self.assertEqual(native_service['selector'], {'app': 'layerx-node'})
        self.assertEqual([(port['port'], port['targetPort']) for port in native_service['ports']],
                         [('8545', 'chain-rpc'), ('26657', 'consensus-rpc')])
        for name in ('chain-rpc', 'consensus-rpc'):
            self.assertIn(name, self.node['ports'])
        for volume_name, owner in (('native-private', 'paxd'),
                                   ('treasury-private', 'treasury-signer'),
                                   ('funding-private', 'core-boundary')):
            self.assertEqual(volumes[volume_name]['emptyDir']['medium'], 'Memory')
            users = [name for name, item in containers.items()
                     if any(mount['name'] == volume_name for mount in item.get('volumeMounts', []))]
            self.assertEqual(users, [owner])
        gateway = next(document for document in self.documents
                       if document.get('kind') == 'Deployment'
                       and document['metadata']['name'] == 'layerx-gateway')
        self.assertEqual(gateway['metadata']['namespace'], NAMESPACE)
        self.assertNotEqual(gateway['spec']['template']['metadata']['labels']['app'], 'layerx-node')



if __name__ == '__main__':
    unittest.main()
