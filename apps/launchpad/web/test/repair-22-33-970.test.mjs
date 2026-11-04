import assert from 'node:assert/strict';
import test from 'node:test';
import {createServer} from 'node:http';
import {once} from 'node:events';
import {createHash, randomUUID} from 'node:crypto';
import {readFileSync, existsSync} from 'node:fs';
import {createRequire, registerHooks} from 'node:module';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {
 metadataBrowserLabels, recordMetadataRequest, recordMetadataResponse,
 captureCommittedImage, captureViewerImage,
} from '../src/features/metadata-media/measure.browser.mjs';

const require = createRequire(new URL('../package.json', import.meta.url));
const ts = require('typescript');
const {JSDOM} = require('jsdom');
const React = require('react');
const {act} = React;
const {privateKeyToAccount, generatePrivateKey} = require('viem/accounts');
const {recoverMessageAddress} = require('viem');
const sourceRoot = new URL('../src/', import.meta.url);
const sdkRoot = new URL('../../../../sdk/launchpad/packages/sdk/src/', import.meta.url);
const sdkV2Root = new URL('../../../../sdk/launchpad/packages/sdk-v2/src/', import.meta.url);
const sourceRoots = [sourceRoot, sdkRoot, sdkV2Root];

// The frozen install does not build workspace packages. Compile the SDK's
// committed schemas in memory, with the same strict Ajv settings as its build.
function sdkValidatorModule(kind) {
 const document = new URL('schemas.gen.json', kind === 'v1' ? sdkRoot : sdkV2Root);
 const index = kind === 'v1' ? 'contractValidators' : kind === 'v2' ? 'v2ContractValidators' : 'operationValidators';
 return `
  import {readFileSync} from 'node:fs';
  import {Ajv2020} from 'ajv/dist/2020.js';
  import {fullFormats} from 'ajv-formats/dist/formats.js';
  const document = JSON.parse(readFileSync(new URL(${JSON.stringify(document.href)}), 'utf8'));
  const ajv = new Ajv2020({strict: true, allErrors: true, validateFormats: true, allowUnionTypes: true, inlineRefs: false});
  for (const [name, format] of Object.entries(fullFormats)) ajv.addFormat(name, format);
  const key = 'urn:metadata-repair:${kind}';
  ajv.addSchema(document, key);
  const validators = {};
  if (${JSON.stringify(kind)} !== 'operations') {
   for (const name of Object.keys(document.$defs)) validators[name] = ajv.getSchema(key + '#/$defs/' + name);
  } else {
   const source = readFileSync(new URL(${JSON.stringify(new URL('operations.gen.ts', sdkRoot).href)}), 'utf8');
   const match = /export const operationDefinitions = (\\{[\\s\\S]*\\}) as const;\\s*$/.exec(source);
   if (!match) throw new Error('Missing SDK operation definitions');
   const rebase = value => Array.isArray(value) ? value.map(rebase) : value && typeof value === 'object'
    ? Object.fromEntries(Object.entries(value).map(([name, entry]) => [name, name === '$ref' && typeof entry === 'string' && entry.startsWith('#/') ? key + entry : rebase(entry)])) : value;
   for (const [name, definition] of Object.entries(JSON.parse(match[1]))) {
    for (const location of ['path', 'query']) {
     const parameters = definition.parameters.filter(parameter => parameter.in === location);
     validators[name + ':' + location] = ajv.compile(rebase({type: 'object', additionalProperties: false,
      properties: Object.fromEntries(parameters.map(parameter => [parameter.name, parameter.schema])),
      required: parameters.filter(parameter => parameter.required).map(parameter => parameter.name)}));
    }
    for (const parameter of definition.parameters.filter(parameter => parameter.in === 'header')) {
     validators[name + ':header:' + parameter.name] = ajv.compile(rebase(parameter.schema));
    }
   }
  }
  export const ${index} = Object.freeze(validators);
 `;
}

// Load the actual TS/TSX modules in process, with the app's existing @ alias.
const hooks = registerHooks({
 resolve(specifier, context, nextResolve) {
  if (specifier === '@kindle/sdk') return {url: new URL('index.ts', sdkRoot).href, shortCircuit: true};
  const local = specifier.startsWith('@/') ? new URL(specifier.slice(2), sourceRoot)
   : specifier.startsWith('.') && sourceRoots.some(root => context.parentURL?.startsWith(root.href)) ? new URL(specifier, context.parentURL) : null;
  if (local?.href.startsWith(sdkRoot.href) && /\/validators-(?:v1|v2|operations)\.gen\.js$/.test(local.href)) return {url: local.href, shortCircuit: true};
  if (local?.href.endsWith('.js') && local.href.startsWith(sdkRoot.href)) {
   const candidate = new URL(local.href.replace(/\.js$/, '.ts'));
   if (existsSync(candidate)) return {url: candidate.href, shortCircuit: true};
  }
  if (local) for (const suffix of ['', '.ts', '.tsx', '/index.ts']) {
   const candidate = fileURLToPath(local) + suffix;
   if (existsSync(candidate) && /\.tsx?$/.test(candidate)) return {url: pathToFileURL(candidate).href, shortCircuit: true};
  }
  return nextResolve(specifier, context);
 },
 load(url, context, nextLoad) {
  const validator = url.startsWith(sdkRoot.href) && /\/validators-(v1|v2|operations)\.gen\.js$/.exec(url);
  if (validator) return {format: 'module', shortCircuit: true, source: sdkValidatorModule(validator[1])};
  if (sourceRoots.some(root => url.startsWith(root.href)) && /\.tsx?$/.test(url)) return {
   format: 'module', shortCircuit: true,
   source: ts.transpileModule(readFileSync(new URL(url), 'utf8'), {
    fileName: fileURLToPath(url),
    compilerOptions: {target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext, jsx: ts.JsxEmit.ReactJSX},
   }).outputText,
  };
  return nextLoad(url, context);
 },
});

const tokenAddress = '0x' + '1'.repeat(40);
const poolAddress = '0x' + '2'.repeat(40);
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a1foAAAAASUVORK5CYII=', 'base64');
const contentSha256 = createHash('sha256').update(png).digest('hex');
const variants = Object.fromEntries(['256', '512', '1024'].map(size => [size, `https://images.example/approved/${contentSha256}/${size}/${'b'.repeat(64)}.png`]));

function envelope(data) {
 return {data, generatedAt: new Date().toISOString(), nextCursor: null, quality: 'complete', requestId: randomUUID(), revision: null, watermark: null};
}
function initialMetadata() {
 return {description: '', displayName: null, displaySymbol: null, revision: '7', socialLinks: [], tags: [], token: {
  address: tokenAddress, decimals: 6, decimalsQuality: 'verified', logoAssetId: null, metadataRevision: '7', name: 'Coin', symbol: 'COIN',
  supplyQuality: 'verified', totalSupplyRaw: '1000000', sourcePosition: {
   chainId: '125', height: '1', blockHash: '0x' + 'a'.repeat(64), transactionHash: '0x' + 'b'.repeat(64), transactionIndex: '0', logIndex: '0',
  },
 }};
}
async function openMetadataFixture() {
 const account = privateKeyToAccount(generatePrivateKey());
 const network = [], requests = [], failures = [], signatures = [];
 const state = {metadata: initialMetadata(), uploadCount: 0, outage: false, asset: null, releaseCancelled: null};
 let origin;
 const server = createServer(async (request, response) => {
  try {
   const bytes = Buffer.concat(await Array.fromAsync(request));
   const url = origin + request.url;
   const row = {method: request.method, url, headers: request.headers, body: bytes.toString('utf8')};
   requests.push(row);
   recordMetadataRequest(network, row);
   const send = (status, body) => {
    recordMetadataResponse(network, {...row, status, body});
    response.writeHead(status, {'Content-Type': 'application/json'}).end(JSON.stringify(body));
   };
   const path = new URL(url).pathname;
   if (path.startsWith(`/v2/tokens/${tokenAddress}/assets/`) && request.method === 'PUT') {
    assert.equal(request.headers.authorization, 'Bearer loopback-session');
    assert.equal(request.headers['content-type'], 'image/png');
    assert.deepEqual(bytes, png);
    const operation = path.split('/').at(-1);
    assert.equal(request.headers['idempotency-key'], operation);
    if (state.asset) assert.equal(state.asset.assetId, operation);
    state.asset = {assetId: operation, bytes: bytes.length, mediaType: 'image/png', ownerWallet: account.address.toLowerCase(),
     sha256: '0x' + contentSha256, state: 'approved', publicUrl: variants['512']};
    state.uploadCount++;
    if (state.uploadCount === 1) {request.socket.destroy(); return;}
    if (state.uploadCount === 2) {
     state.releaseCancelled = () => send(200, envelope({asset: state.asset}));
     return;
    }
    send(200, envelope({asset: state.asset}));
    return;
   }
   if (path === `/v2/tokens/${tokenAddress}/metadata`) {
    if (state.outage) {send(503, {error: {code: 'UNAVAILABLE'}}); return;}
    if (request.method === 'PATCH') {
     const patch = JSON.parse(bytes);
     assert.deepEqual(patch, {expectedRevision: '7', logoAssetId: state.asset.assetId});
     assert.equal(request.headers.authorization, 'Bearer loopback-session');
     assert.equal(request.headers['content-type'], 'application/json');
     assert.equal(request.headers['x-action-wallet'], account.address.toLowerCase());
     assert.equal(request.headers['x-action-audience'], origin);
     const statement = ['KindleLaunch metadata action v2', origin, '125', account.address.toLowerCase(),
      request.headers['idempotency-key'], 'PATCH', path, createHash('sha256').update(bytes).digest('hex'),
      request.headers['x-action-nonce'], request.headers['x-action-expires']].join('\n');
     assert.equal((await recoverMessageAddress({message: statement, signature: request.headers['x-action-signature']})).toLowerCase(), account.address.toLowerCase());
     assert.ok(Number(request.headers['x-action-expires']) > Date.now() / 1000);
     assert.equal(signatures.at(-1), statement);
     state.metadata = {...state.metadata, revision: '8', token: {...state.metadata.token, logoAssetId: patch.logoAssetId, metadataRevision: '8'}};
    } else assert.equal(request.method, 'GET');
    send(200, envelope(state.metadata));
    return;
   }
   if (path === `/v2/tokens/${tokenAddress}/media` && request.method === 'GET') {
    if (state.outage) {send(503, {error: {code: 'UNAVAILABLE'}}); return;}
    send(200, envelope({metadata: {chainId: 125, poolAddress, tokenAddress, revision: state.metadata.revision,
     updatedAt: null, source: 'chain', moderationStatus: 'allowed', logoAssetId: state.metadata.token.logoAssetId,
     display: {name: null, symbol: null, description: '', tags: [], socials: []}},
     image: {assetId: state.asset.assetId, sha256: state.asset.sha256, mime: 'image/png', width: 1, height: 1, variants}}));
    return;
   }
   assert.fail(`Unexpected request: ${request.method} ${path}`);
  } catch (error) {
   failures.push(error);
   response.writeHead(500).end(JSON.stringify({error: 'fixture contract violation'}));
  }
 });
 server.listen(0, '127.0.0.1');
 await once(server, 'listening');
 origin = `http://127.0.0.1:${server.address().port}`;
 return {origin, account, network, requests, failures, state, signatures,
  async close() {server.closeAllConnections(); await new Promise(resolve => server.close(resolve));},
 };
}

async function metadata_browser_selectors_real_editor_transport_viewer() {
 const fixture = await openMetadataFixture();
 const dom = new JSDOM('<!doctype html><html lang="en"><head><title>Coin metadata</title></head><body><main id="root"></main></body></html>', {url: fixture.origin});
 const savedGlobals = new Map();
 const install = (name, value) => {savedGlobals.set(name, Object.getOwnPropertyDescriptor(globalThis, name)); Object.defineProperty(globalThis, name, {configurable: true, writable: true, value});};
 for (const [name, value] of Object.entries({window: dom.window, document: dom.window.document, location: dom.window.location, navigator: dom.window.navigator,
  HTMLElement: dom.window.HTMLElement, IS_REACT_ACT_ENVIRONMENT: true})) install(name, value);
 const envNames = ['NODE_ENV', 'NEXT_PUBLIC_DATA_API_URL', 'NEXT_PUBLIC_GATEWAY_URL', 'NEXT_PUBLIC_METADATA_API_URL', 'NEXT_PUBLIC_PNL_API_URL'];
 const savedEnv = new Map(envNames.map(name => [name, process.env[name]]));
 for (const name of envNames) process.env[name] = name === 'NODE_ENV' ? 'test' : fixture.origin;
 let root;
 try {
  const {createRoot} = require('react-dom/client');
  const {MetadataImageEditor} = await import('../src/features/metadata-media/editor.tsx');
  const {MetadataViewer} = await import('../src/features/metadata-media/viewer.tsx');
  const {createMetadataImageTransport} = await import('../src/features/metadata-media/transport.ts');
  const {decodeV2Contract} = await import('@kindle/sdk');
  assert.throws(() => decodeV2Contract('MetadataPatchBody', {expectedRevision: 7}), /Invalid v2 contract/);
  assert.throws(() => decodeV2Contract('AssetGetResponse', envelope({asset: {assetId: randomUUID()}})), /Invalid v2 contract/);
  const {QueryClient, QueryClientProvider} = await import('@tanstack/react-query');
  const queryClient = new QueryClient({defaultOptions: {queries: {retry: false, gcTime: 0}}});
  const authority = {tokenAddress, wallet: fixture.account.address, audience: fixture.origin, revision: '7',
   authenticationHeaders: async () => ({Authorization: 'Bearer loopback-session'}),
   signMessage: async message => {fixture.signatures.push(message); return fixture.account.signMessage({message});},
  };
  const upload = createMetadataImageTransport(authority);
  const approvals = [];
  root = createRoot(document.getElementById('root'));
  await act(async () => root.render(React.createElement(MetadataImageEditor, {upload, activeRevision: '7', onApproved: value => approvals.push(value)})));
  const editor = document.querySelector(`section[aria-label="${metadataBrowserLabels.editor}"]`);
  assert.ok(editor, 'current accessible editor region');
  const label = [...editor.querySelectorAll('label')].find(element => element.textContent === metadataBrowserLabels.chooser);
  assert.ok(label, 'current accessible file label');
  const chooser = document.getElementById(label.htmlFor);
  assert.equal(chooser.type, 'file');
  const alert = document.getElementById(chooser.getAttribute('aria-describedby'));
  assert.equal(alert.getAttribute('role'), 'alert', 'upload errors remain associated');
  const button = name => [...editor.querySelectorAll('button')].find(element => element.textContent === name);
  const uploadButton = button(metadataBrowserLabels.upload);
  const cancelButton = button('Cancel');
  assert.ok(uploadButton && cancelButton);
  assert.equal(uploadButton.disabled, true);
  const file = new File([png], 'coin.png', {type: 'image/png'});
  Object.defineProperty(chooser, 'files', {value: [file]});
  await act(async () => chooser.dispatchEvent(new dom.window.Event('change', {bubbles: true})));
  chooser.focus();
  assert.equal(document.activeElement, chooser);
  uploadButton.focus();
  assert.equal(document.activeElement, uploadButton);
  assert.equal(uploadButton.disabled, false);
  const waitFor = async (predicate, message) => {
   const deadline = Date.now() + 5000;
   while (!predicate() && Date.now() < deadline) await act(async () => {await new Promise(resolve => setTimeout(resolve, 10));});
   assert.ok(predicate(), message);
  };
  await act(async () => uploadButton.click());
  await waitFor(() => alert.textContent === metadataBrowserLabels.uploadUnavailable, 'real network interruption is visible');
  assert.equal(approvals.length, 0);
  assert.ok(!editor.textContent.includes(metadataBrowserLabels.updated), 'no optimistic approval');
  assert.equal(chooser.getAttribute('aria-invalid'), 'true');
  await act(async () => uploadButton.click());
  await waitFor(() => fixture.state.uploadCount === 2, 'retry reached the real server');
  const uploads = fixture.requests.filter(row => row.method === 'PUT');
  assert.equal(uploads[1].headers['idempotency-key'], uploads[0].headers['idempotency-key'], 'retry preserves the selected operation');
  assert.equal(cancelButton.disabled, false);
  cancelButton.focus();
  assert.equal(document.activeElement, cancelButton);
  await act(async () => cancelButton.click());
  assert.equal(alert.textContent, metadataBrowserLabels.cancelled);
  await act(async () => {fixture.state.releaseCancelled(); await new Promise(resolve => setTimeout(resolve, 10));});
  assert.ok(!editor.textContent.includes(metadataBrowserLabels.updated), 'cancel suppresses late approval');
  assert.equal(approvals.length, 0);
  assert.equal(fixture.requests.filter(row => row.method === 'PATCH').length, 0);
  await act(async () => uploadButton.click());
  await waitFor(() => editor.querySelector('[role="status"]').textContent === metadataBrowserLabels.updated, 'real committed approval is visible');
  assert.equal(approvals.length, 1);
  assert.ok(!editor.textContent.includes(metadataBrowserLabels.pending));
  const operation = approvals[0].operationId;
  const approval = captureCommittedImage(fixture.network, tokenAddress, operation, contentSha256);
  assert.equal(approval.approved_revision, '8');
  assert.notEqual(approval.metadata_operation_id, operation, 'asset and signed metadata operations have distinct identities');
  assert.equal(approvals[0].assetId, operation);
  assert.equal(approvals[0].revision, approval.approved_revision);
  assert.equal(fixture.signatures.length, 1);
  assert.equal(fixture.state.uploadCount, 3);

  const renderViewer = async () => {
   await act(async () => root.render(React.createElement(QueryClientProvider, {client: queryClient},
    React.createElement(MetadataViewer, {tokenAddress, tradePanel: React.createElement('button', null, 'Trade coin')}))));
  };
  await renderViewer();
  const viewer = document.querySelector(`section[aria-label="${metadataBrowserLabels.viewer}"]`);
  assert.ok(viewer, 'current accessible viewer region');
  await waitFor(() => viewer.querySelector(`img[alt="${metadataBrowserLabels.image}"]`), 'production viewer consumes metadata and media responses');
  const image = viewer.querySelector('img');
  assert.equal(image.getAttribute('width'), '1');
  assert.equal(image.getAttribute('height'), '1');
  assert.equal(image.getAttribute('srcset'), `${variants['256']} 256w, ${variants['512']} 512w, ${variants['1024']} 1024w`);
  assert.equal(image.getAttribute('sizes'), '(max-width: 512px) 100vw, 512px');
  assert.ok(viewer.textContent.includes(tokenAddress));
  const viewerApproval = captureViewerImage(fixture.network, tokenAddress, approval, contentSha256, image.src);
  assert.deepEqual(viewerApproval, {revision: '8', asset_id: operation});

  assert.throws(() => captureCommittedImage(fixture.network.filter(row => row.method !== 'PATCH'), tokenAddress, operation, contentSha256), /exactly_one_committed_image_revision/);
  const committedResponse = fixture.network.find(row => row.method === 'PATCH' && row.event === 'response');
  assert.throws(() => captureCommittedImage([...fixture.network, committedResponse], tokenAddress, operation, contentSha256), /exactly_one_committed_image_revision/);
  const altered = edit => {const rows = structuredClone(fixture.network); edit(rows); return rows;};
  assert.throws(() => captureCommittedImage(altered(rows => {rows.find(row => row.method === 'PUT').operation_id = randomUUID();}), tokenAddress, operation, contentSha256), /multiple_asset_allocations/);
  assert.throws(() => captureCommittedImage(altered(rows => {rows.find(row => row.method === 'PATCH' && row.event === 'request').patch.expectedRevision = '8';}), tokenAddress, operation, contentSha256), /approved_revision_pending_not_cleared/);
  assert.throws(() => captureCommittedImage(altered(rows => {rows.find(row => row.method === 'PATCH' && row.event === 'response').data.token.logoAssetId = randomUUID();}), tokenAddress, operation, contentSha256), /committed_asset_identity/);
  assert.throws(() => captureCommittedImage(altered(rows => {rows.filter(row => row.event === 'response' && row.data?.asset).at(-1).data.asset.sha256 = '0x' + 'c'.repeat(64);}), tokenAddress, operation, contentSha256), /committed_asset_content/);
  assert.throws(() => captureViewerImage(altered(rows => {rows.filter(row => row.method === 'GET' && row.url.endsWith('/metadata') && row.event === 'response').at(-1).data.revision = '7';}), tokenAddress, approval, contentSha256, image.src), /viewer_committed_revision_identity/);
  assert.throws(() => captureViewerImage(altered(rows => {rows.find(row => row.url.endsWith('/media') && row.event === 'response').data.metadata.revision = '9';}), tokenAddress, approval, contentSha256, image.src), /viewer_media_revision_identity/);
  assert.throws(() => captureViewerImage(altered(rows => {rows.find(row => row.url.endsWith('/media') && row.event === 'response').data.image.assetId = randomUUID();}), tokenAddress, approval, contentSha256, image.src), /viewer_approved_image_identity/);
  assert.throws(() => captureViewerImage(fixture.network, tokenAddress, approval, contentSha256, variants['512'].replace(contentSha256, 'c'.repeat(64))), /viewer_approved_image_identity/);

  fixture.state.outage = true;
  await act(async () => root.render(null));
  queryClient.clear();
  await renderViewer();
  await waitFor(() => document.querySelector('[role="alert"]')?.textContent === metadataBrowserLabels.outage, 'real metadata outage copy');
  const outageViewer = document.querySelector(`section[aria-label="${metadataBrowserLabels.viewer}"]`);
  assert.ok(outageViewer.textContent.includes(tokenAddress), 'outage retains authoritative token');
  assert.equal(outageViewer.querySelector('[aria-label="Trading"] button').textContent, 'Trade coin');
  assert.equal(outageViewer.querySelector('[aria-label="Trading"] button').disabled, false);
  assert.equal(outageViewer.querySelector('img'), null);
  assert.ok(fixture.network.some(row => row.method === 'GET' && row.status === 503 && row.url.endsWith('/metadata')));
  assert.deepEqual(fixture.failures, []);
  queryClient.clear();
 } finally {
  if (root) await act(async () => root.unmount());
  for (const [name, descriptor] of savedGlobals) {
   if (descriptor) Object.defineProperty(globalThis, name, descriptor); else delete globalThis[name];
  }
  for (const [name, value] of savedEnv) {if (value === undefined) delete process.env[name]; else process.env[name] = value;}
  dom.window.close();
  await fixture.close();
 }
}
test(metadata_browser_selectors_real_editor_transport_viewer);
test.after(() => hooks.deregister());
