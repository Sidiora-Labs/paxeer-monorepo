import { expect, test } from 'vitest';
import {
  EndpointClient,
  decodeNodeRouteCatalogue,
  decodeRouteCatalogue,
  type NodeRouteCatalogue,
} from '../src/endpoint.js';
import { JsonRpcTransportError } from '../src/rpc.js';

type MutableCatalogue = {
  schema: unknown;
  chain_id: unknown;
  rpc_paths: unknown;
  services: Record<string, unknown>[];
  routes: Record<string, unknown>[];
  readiness: Record<string, Record<string, unknown>>;
};

async function nodeRouteCatalogueDecodesAndValidates(): Promise<void> {
  const url = process.env.PX_NODE_CATALOGUE_URL;
  expect(url, 'the Go test must supply its real node HTTP root server').toBeTruthy();
  const expectedMethods = JSON.parse(process.env.PX_NODE_CATALOGUE_METHODS!);
  expect(Array.isArray(expectedMethods)).toBe(true);
  expect(expectedMethods.length).toBeGreaterThan(0);
  const client = new EndpointClient({ nodeUrl: url });
  const producer = await client.rpc.call<MutableCatalogue>('px_getRouteCatalogue', []);
  const discovery: NodeRouteCatalogue = await client.getRouteCatalogue();
  const withHosted = await new EndpointClient({
    nodeUrl: url,
    hostedUrl: 'http://127.0.0.1:0/',
    timeoutMs: 5000,
  }).getRouteCatalogue();
  expect(discovery).toEqual(producer);
  expect(withHosted).toEqual(discovery);
  expect(decodeNodeRouteCatalogue(producer)).toEqual(discovery);
  expect(discovery.schema).toBe('paxeer-x.routes.v1');
  expect(discovery.chain_id).toBe(125);
  expect(discovery.rpc_paths).toEqual(['/']);
  expect(Object.keys(discovery).sort()).toEqual([
    'chain_id', 'readiness', 'routes', 'rpc_paths', 'schema', 'services',
  ]);
  const serviceKeys = [
    'authentication', 'discovery_path', 'exposure', 'id', 'purpose', 'transport', 'version',
  ];
  const privateFields = [
    'canonical_origin', 'upstream', 'upstream_path', 'source', 'owner_source',
    'forwarded_headers', 'attestor_authorization_headers',
  ];
  for (const service of discovery.services) {
    expect(Object.keys(service).sort()).toEqual(serviceKeys);
    expect(service.exposure).toBe('product');
    if (service.id !== 'node_rpc') {
      expect(discovery.readiness[service.id]).toEqual({
        ready: false, status: 'unknown', reason: 'hosted_dependency_not_measured',
      });
    }
  }
  expect(Object.keys(discovery.readiness).sort()).toEqual(
    discovery.services.map((service) => service.id).sort()
  );
  expect(discovery.readiness.node_rpc).toEqual({
    ready: true, scope: 'node_registration', methods: expectedMethods,
  });
  for (const method of ['px_getRouteCatalogue', 'px_register', 'px_requestFunds']) {
    expect(discovery.readiness.node_rpc.methods).toContain(method);
  }
  const inferenceMethods = [
    'px_getInferenceRooms', 'px_getInferenceRoom', 'px_getInferenceDeal',
    'px_getInferencePeer', 'px_getInferenceReceipts',
  ];
  const nodeRoutes = discovery.routes.filter((route) => route.service === 'node_rpc');
  expect(nodeRoutes).toHaveLength(inferenceMethods.length);
  expect(nodeRoutes.map((route) => route.method)).toEqual(inferenceMethods);
  for (const route of discovery.routes) {
    for (const field of privateFields) expect(route).not.toHaveProperty(field);
    expect(discovery.services.some((service) => service.id === route.service)).toBe(true);
  }
  for (const route of nodeRoutes) {
    expect(route).toEqual({
      id: `node_rpc:${route.method}`, service: 'node_rpc', method: route.method, path: '/',
      transport: 'json-rpc', authentication: 'none', version: '1',
      consumers: ['wallet', 'explorer', 'developer'],
    });
    expect(discovery.readiness.node_rpc.methods).toContain(route.method);
    for (const field of ['timeout_seconds', 'retries', 'health_predicate']) {
      expect(route).not.toHaveProperty(field);
    }
  }
  expect(() => decodeRouteCatalogue(producer)).toThrow(JsonRpcTransportError);

  const reject = (label: string, mutate: (doc: MutableCatalogue) => void) => {
    const malformed = structuredClone(producer);
    mutate(malformed);
    expect(() => decodeNodeRouteCatalogue(malformed), label).toThrow(JsonRpcTransportError);
  };
  const nodeIndex = producer.routes.findIndex((route) => route.service === 'node_rpc');
  const hostedIndex = producer.routes.findIndex((route) => route.service !== 'node_rpc');
  expect(nodeIndex).toBeGreaterThanOrEqual(0);
  expect(hostedIndex).toBeGreaterThanOrEqual(0);
  const hostedID = producer.services.find((service) => service.id !== 'node_rpc')!.id as string;
  const nodeRoute = (doc: MutableCatalogue) => doc.routes[nodeIndex]!;
  const hostedRoute = (doc: MutableCatalogue) => doc.routes[hostedIndex]!;

  for (const schema of [undefined, null, '', 'paxeer-x.routes.v2', 1]) {
    reject('schema', (doc) => { doc.schema = schema; });
  }
  for (const chain of [undefined, null, '125', 0, 126]) {
    reject('chain identity', (doc) => { doc.chain_id = chain; });
  }
  for (const paths of [undefined, null, [], ['/', '/'], ['/evm'], ['//host'], [7], ['/bad?key=1']]) {
    reject('rpc paths', (doc) => { doc.rpc_paths = paths; });
  }
  for (const field of privateFields) {
    reject('private top-level field', (doc) => { Object.assign(doc, { [field]: 'private' }); });
    reject('private service field', (doc) => { doc.services[0]![field] = 'private'; });
    reject('private route field', (doc) => { hostedRoute(doc)[field] = 'private'; });
  }
  reject('empty services', (doc) => { doc.services = []; });
  reject('empty routes', (doc) => { doc.routes = []; });
  reject('non-array services', (doc) => { Object.assign(doc, { services: {} }); });
  reject('non-array routes', (doc) => { Object.assign(doc, { routes: {} }); });
  reject('oversize service inventory', (doc) => {
    doc.services = Array.from({ length: 4097 }, (_, i) => ({ ...doc.services[0], id: `service-${i}` }));
  });
  reject('4096 route bound', (doc) => {
    doc.routes = Array.from({ length: 4097 }, (_, i) => ({ ...hostedRoute(doc), id: `route-${i}` }));
  });
  reject('duplicate service ID', (doc) => { doc.services.push({ ...doc.services[0] }); });
  reject('duplicate route ID', (doc) => { doc.routes.push({ ...doc.routes[0] }); });
  reject('dangling service reference', (doc) => { hostedRoute(doc).service = 'absent'; });
  reject('missing node service', (doc) => {
    doc.services = doc.services.filter((service) => service.id !== 'node_rpc');
    delete doc.readiness.node_rpc;
  });
  for (const field of ['id', 'purpose', 'discovery_path', 'transport', 'authentication', 'version']) {
    for (const value of [undefined, null, '', 7, 'bad\nfield']) {
      reject(`service ${field}`, (doc) => { doc.services[0]![field] = value; });
    }
  }
  reject('unknown service transport', (doc) => { doc.services[0]!.transport = 'smtp'; });
  reject('unknown route transport', (doc) => { hostedRoute(doc).transport = 'smtp'; });
  reject('unknown route method', (doc) => { hostedRoute(doc).method = 'garbage'; });
  reject('private service exposure', (doc) => { doc.services[0]!.exposure = 'private'; });
  reject('absolute discovery path', (doc) => { doc.services[0]!.discovery_path = 'https://private/'; });
  reject('node service contract', (doc) => {
    doc.services.find((service) => service.id === 'node_rpc')!.transport = 'http';
  });
  for (const field of ['id', 'service', 'method', 'path', 'transport', 'authentication', 'version', 'health_predicate']) {
    for (const value of [undefined, null, '', 7]) {
      reject(`route ${field}`, (doc) => { hostedRoute(doc)[field] = value; });
    }
  }
  reject('absolute route path', (doc) => { hostedRoute(doc).path = 'https://private/'; });
  for (const transport of ['json-rpc', 'websocket', 'mcp']) {
    const operationIndex = producer.routes.findIndex((route) =>
      route.service !== 'node_rpc' && route.transport === transport &&
      !(route.path as string).startsWith('/')
    );
    expect(operationIndex, `${transport} operation contract must come from the producer`).toBeGreaterThanOrEqual(0);
    for (const path of ['https://private/', '../private', 'bad operation', 'bad..operation']) {
      reject(`malformed ${transport} operation`, (doc) => { doc.routes[operationIndex]!.path = path; });
    }
  }
  for (const value of [undefined, null, [], ['wallet', 'wallet'], [''], [7]]) {
    reject('route consumers', (doc) => { hostedRoute(doc).consumers = value; });
    reject('node route consumers', (doc) => { nodeRoute(doc).consumers = value; });
  }
  for (const timeout of [undefined, null, '8', 0, -1, NaN, Infinity]) {
    reject('positive finite hosted timeout', (doc) => { hostedRoute(doc).timeout_seconds = timeout; });
    reject('positive finite connection timeout', (doc) => { hostedRoute(doc).connect_timeout_seconds = timeout; });
  }
  for (const retries of [undefined, null, '0', -1, 1]) {
    reject('zero retries', (doc) => { hostedRoute(doc).retries = retries; });
  }
  for (const field of ['max_body_bytes', 'max_header_bytes', 'asset_max_path_bytes',
    'maximum_lifetime_seconds', 'response_max_bytes', 'stream_lifetime_seconds']) {
    for (const value of [null, '1', 0, -1, 1.5, Infinity]) {
      reject(`route bound ${field}`, (doc) => { hostedRoute(doc)[field] = value; });
    }
  }
  reject('negative request bound', (doc) => { hostedRoute(doc).request_body_max_bytes = -1; });
  reject('malformed route flag', (doc) => { hostedRoute(doc).proxy = 'true'; });
  reject('malformed public list', (doc) => { hostedRoute(doc).required_request_headers = [null]; });
  reject('malformed error classes', (doc) => { hostedRoute(doc).error_classes = { bad: 'error' }; });
  reject('malformed contract address', (doc) => { hostedRoute(doc).contract_address = '0x123'; });
  for (const [field, value] of Object.entries({
    id: 'node_rpc:other', method: 'px_unknown', path: '/evm', transport: 'http',
    authentication: 'gateway-api-key', version: '2', timeout_seconds: 8,
  })) {
    reject(`node route ${field}`, (doc) => { nodeRoute(doc)[field] = value; });
  }
  reject('missing native route', (doc) => { doc.routes.splice(nodeIndex, 1); });
  reject('missing readiness', (doc) => { Object.assign(doc, { readiness: null }); });
  reject('missing readiness entry', (doc) => { delete doc.readiness[hostedID]; });
  reject('extra readiness entry', (doc) => { doc.readiness.extra = { ...doc.readiness[hostedID] }; });
  reject('nested aggregate readiness', (doc) => {
    Object.assign(doc, { readiness: { complete: false, services: doc.readiness } });
  });
  reject('null readiness state', (doc) => { Object.assign(doc.readiness, { [hostedID]: null }); });
  for (const [field, value] of Object.entries({
    ready: true, status: 'ready', reason: 'measured', scope: 'node_registration', methods: [],
  })) {
    reject(`hosted readiness ${field}`, (doc) => { doc.readiness[hostedID]![field] = value; });
  }
  for (const field of ['ready', 'status', 'reason']) {
    reject(`missing hosted readiness ${field}`, (doc) => { delete doc.readiness[hostedID]![field]; });
  }
  for (const [field, value] of Object.entries({ ready: false, scope: 'hosted', status: 'unknown', reason: 'measured' })) {
    reject(`registration readiness ${field}`, (doc) => { doc.readiness.node_rpc![field] = value; });
  }
  for (const methods of [undefined, null, [], ['bad method'], [1], ['px_getRouteCatalogue', 'px_getRouteCatalogue']]) {
    reject('registration methods inventory', (doc) => { doc.readiness.node_rpc!.methods = methods; });
  }
  for (const method of ['px_getRouteCatalogue', ...inferenceMethods]) {
    reject('registration methods must cover native discovery', (doc) => {
      doc.readiness.node_rpc!.methods = expectedMethods.filter((entry: string) => entry !== method);
    });
  }
}

test(nodeRouteCatalogueDecodesAndValidates, nodeRouteCatalogueDecodesAndValidates, 60_000);
