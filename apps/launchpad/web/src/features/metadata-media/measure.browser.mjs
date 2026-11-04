import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {pathToFileURL, fileURLToPath} from 'node:url';
import {spawnSync} from 'node:child_process';

const assert = (condition, code) => {if (!condition) throw new Error(code);};
const sha = bytes => crypto.createHash('sha256').update(bytes).digest('hex');
export const metadataBrowserLabels = Object.freeze({
 editor: 'Coin image', chooser: 'Choose image', upload: 'Upload', cancelled: 'Upload cancelled',
 uploadUnavailable: 'Connection problem, check your internet and try again',
 updated: 'Image updated', pending: 'Uploaded, checking your image...', viewer: 'Coin details', image: 'Coin image',
 outage: "Can't load coin details right now, you can still trade",
});
const uuid = /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/;
const revision = value => typeof value === 'string' && /^(0|[1-9][0-9]{0,77})$/.test(value) && BigInt(value) < (1n << 256n);
function metadataEndpoint(url) {
 return /^\/v2\/tokens\/0x[a-f0-9]{40}\/(?:assets\/[a-f0-9-]{36}|metadata|media)$/.test(new URL(url).pathname);
}
export function recordMetadataRequest(network, {method, url, headers, body}) {
 if (!metadataEndpoint(url)) return;
 const row = {event: 'request', at_ms: Date.now(), method, url, operation_id: headers['idempotency-key'] ?? null};
 if (method === 'PATCH') row.patch = JSON.parse(body);
 network.push(row);
}
export function recordMetadataResponse(network, {method, url, headers, status, body, failure}) {
 if (!metadataEndpoint(url)) return;
 network.push({event: 'response', at_ms: Date.now(), method, url, status, operation_id: headers['idempotency-key'] ?? null, data: body?.data ?? null, ...(failure ? {failure} : {})});
}
export function captureCommittedImage(network, tokenAddress, operation, contentSha256) {
 const metadataPath = `/v2/tokens/${tokenAddress}/metadata`;
 const assetPath = `/v2/tokens/${tokenAddress}/assets/${operation}`;
 assert(uuid.test(operation), 'actual_upload_operation');
 const uploads = network.filter(row => row.event === 'request' && row.method === 'PUT');
 assert(uploads.length >= 3 && uploads.every(row => row.operation_id === operation && new URL(row.url).pathname === assetPath), 'multiple_asset_allocations');
 const committed = network.filter(row => row.method === 'PATCH' && row.event === 'response' && row.status === 200);
 assert(committed.length === 1, 'exactly_one_committed_image_revision');
 const commit = committed[0];
 const request = network.filter(row => row.event === 'request' && row.method === 'PATCH' && row.operation_id === commit.operation_id);
 assert(uuid.test(commit.operation_id) && request.length === 1 && new URL(commit.url).pathname === metadataPath && new URL(request[0].url).pathname === metadataPath, 'committed_operation_identity');
 const patch = request[0].patch;
 const metadata = commit.data;
 assert(patch?.logoAssetId === operation && revision(patch.expectedRevision) && revision(metadata?.revision) && BigInt(metadata.revision) > BigInt(patch.expectedRevision), 'approved_revision_pending_not_cleared');
 assert(metadata.token?.address === tokenAddress && metadata.token.logoAssetId === operation && metadata.token.metadataRevision === metadata.revision, 'committed_asset_identity');
 const assets = network.filter(row => row.event === 'response' && row.status === 200 && new URL(row.url).pathname === assetPath && row.data?.asset?.state === 'approved');
 assert(assets.length > 0, 'committed_asset_approval_missing');
 const asset = assets.at(-1).data.asset;
 assert(asset.assetId === operation && asset.sha256 === '0x' + contentSha256 && typeof asset.publicUrl === 'string', 'committed_asset_content');
 const imageUrl = new URL(asset.publicUrl);
 assert(imageUrl.protocol === 'https:' && !imageUrl.username && !imageUrl.password && !imageUrl.search && !imageUrl.hash && new RegExp(`^/approved/${contentSha256}/(256|512|1024)/[a-f0-9]{64}\\.png$`).test(imageUrl.pathname), 'committed_asset_content');
 return {operation_id: operation, metadata_operation_id: commit.operation_id, approved_revision: metadata.revision};
}
export function captureViewerImage(network, tokenAddress, approval, contentSha256, currentSrc) {
 const response = suffix => network.filter(row => row.event === 'response' && row.method === 'GET' && row.status === 200 && new URL(row.url).pathname === `/v2/tokens/${tokenAddress}/${suffix}`).at(-1)?.data;
 const metadata = response('metadata');
 const media = response('media');
 assert(metadata?.token?.address === tokenAddress && metadata.revision === approval.approved_revision && metadata.token.metadataRevision === approval.approved_revision && metadata.token.logoAssetId === approval.operation_id, 'viewer_committed_revision_identity');
 assert(media?.metadata?.chainId === 125 && media.metadata.tokenAddress === tokenAddress && media.metadata.revision === approval.approved_revision && media.metadata.logoAssetId === approval.operation_id && media.metadata.moderationStatus === 'allowed', 'viewer_media_revision_identity');
 assert(media.image?.assetId === approval.operation_id && media.image.sha256 === '0x' + contentSha256 && Object.values(media.image.variants).includes(currentSrc), 'viewer_approved_image_identity');
 const url = new URL(currentSrc);
 assert(url.protocol === 'https:' && !url.username && !url.password && !url.search && !url.hash && new RegExp(`^/approved/${contentSha256}/(256|512|1024)/[a-f0-9]{64}\\.png$`).test(url.pathname), 'viewer_approved_image_identity');
 return {revision: metadata.revision, asset_id: media.image.assetId};
}
const checkFile = reference => {
 assert(reference && path.isAbsolute(reference.path) && /^[a-f0-9]{64}$/.test(reference.sha256), 'browser_file_reference');
 let current = reference.path;
 while (current !== path.dirname(current)) {assert(!fs.lstatSync(current).isSymbolicLink(), 'browser_symlink_denied');current = path.dirname(current);}
 const bytes = fs.readFileSync(reference.path);assert(sha(bytes) === reference.sha256, 'browser_file_digest');return bytes;
};
export async function runMetadataBrowser(input) {
 const nativeCapture = phase => {
  const config = input.native_probe;
  checkFile(config.executable);
  assert(config.kind === 'native_provider_capture' && config.fault_authorization_id && config.owned_scope, 'browser_fault_authority');
  for (const name of config.required_environment) assert(process.env[name], 'browser_provider_credentials');
  const result = spawnSync(config.executable.path, config.argv, {input: JSON.stringify({phase, bindings: input.bindings, owned_scope: config.owned_scope, observed_utc_ns: String(BigInt(Date.now()) * 1000000n)}), encoding: 'utf8', timeout: 30000, maxBuffer: 8388608});
  assert(result.status === 0, 'browser_native_capture_failed');
  const record = JSON.parse(result.stdout);
  assert(Object.keys(input.bindings).every(key => record.bindings[key] === input.bindings[key]) && Object.keys(record.bindings).length === Object.keys(input.bindings).length && record.phase === phase && record.owned_scope === config.owned_scope, 'browser_native_binding');
  return record;
 };
 const startedUtcNs = String(BigInt(Date.now()) * 1000000n);
 const state = JSON.parse(checkFile(input.storage_state).toString('utf8'));
 const imageBytes = checkFile(input.image);
 checkFile(input.license_receipt);checkFile(input.source_receipt);
 assert(imageBytes.length > 0 && imageBytes.length <= 6291456, 'actual_image_size');
 assert(['image/jpeg', 'image/png', 'image/webp'].includes(input.mime), 'actual_image_mime');
 assert(/^0x[a-f0-9]{40}$/.test(input.token_address), 'token_identity');
 const creatorUrl = new URL(input.creator_url);
 const viewerUrl = new URL(input.viewer_url);
 assert(creatorUrl.protocol === 'https:' && viewerUrl.protocol === 'https:' && creatorUrl.origin === viewerUrl.origin && !creatorUrl.username && !creatorUrl.password && !viewerUrl.username && !viewerUrl.password, 'browser_owned_https_origin');
 const moduleManifest = JSON.parse(checkFile(input.playwright_package).toString('utf8'));
 assert(moduleManifest.name === 'playwright', 'actual_playwright_package');
 checkFile(input.playwright_entry);
 assert(path.dirname(input.playwright_entry.path) === path.dirname(input.playwright_package.path), 'playwright_entry_identity');
 const playwright = await import(pathToFileURL(input.playwright_entry.path).href);
 const axeBytes = checkFile(input.axe_source);
 const sourceRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../../../../../../launchpad');
 const allowedOutput = path.join(sourceRoot, 'evidence/production-system/L39/phase4');
 assert(input.output && path.isAbsolute(input.output) && path.resolve(input.output).startsWith(allowedOutput + path.sep) && !fs.existsSync(input.output), 'browser_unique_output');
 let browser;
 let context;
 let metadataFaultStarted = false;
 const network = [];
 const responseCaptures = [];
 try {
  checkFile(input.chromium_executable);
  assert(path.isAbsolute(input.browser_profile_path) && fs.statSync(input.browser_profile_path).isDirectory() && !fs.lstatSync(input.browser_profile_path).isSymbolicLink(), 'real_leased_wallet_profile');
  checkFile(input.wallet_extension_manifest);
  const extensionDirectory = path.dirname(input.wallet_extension_manifest.path);
  assert(JSON.parse(fs.readFileSync(input.wallet_extension_manifest.path, 'utf8')).manifest_version >= 2, 'actual_wallet_extension');
  context = await playwright.chromium.launchPersistentContext(input.browser_profile_path, {executablePath: input.chromium_executable.path, headless: false, args: [`--disable-extensions-except=${extensionDirectory}`, `--load-extension=${extensionDirectory}`], viewport: {width: 390, height: 844}, serviceWorkers: 'block'});
  await context.addCookies(state.cookies);
  await context.addInitScript(origins => {for (const origin of origins) if (location.origin === origin.origin) for (const item of origin.localStorage) localStorage.setItem(item.name, item.value);}, state.origins);
  const page = await context.newPage();
  page.setDefaultTimeout(60000);
  await page.addInitScript(() => {
   globalThis.__metadataLayoutShift = 0;
   new PerformanceObserver(list => {for (const entry of list.getEntries()) if (!entry.hadRecentInput) globalThis.__metadataLayoutShift += entry.value;}).observe({type: 'layout-shift', buffered: true});
  });
  page.on('request', request => {
   recordMetadataRequest(network, {method: request.method(), url: request.url(), headers: request.headers(), body: request.method() === 'PATCH' ? request.postData() : null});
  });
  page.on('response', response => {
   if (!metadataEndpoint(response.url())) return;
   const capture = (async () => {
    const request = response.request();
    let body = null;
    let failure;
    try {if (response.status() === 200) body = await response.json();}
    catch (error) {
     // A deliberately interrupted upload may have headers but no readable body.
     failure = request.failure()?.errorText;
     if (request.method() !== 'PUT' || !failure) throw error;
    }
    recordMetadataResponse(network, {method: request.method(), url: response.url(), headers: request.headers(), status: response.status(), body, failure});
   })();
   // Keep asynchronous response failures in the journey's qualification path.
   responseCaptures.push(capture.then(() => null, error => error));
  });
  const capturedResponses = async () => {
   const errors = await Promise.all(responseCaptures);
   for (const error of errors) if (error) throw error;
  };
  await page.goto(input.creator_url, {waitUntil: 'networkidle'});
  const region = page.getByRole('region', {name: metadataBrowserLabels.editor, exact: true});
  await region.waitFor({state: 'visible'});
  const chooser = region.getByLabel(metadataBrowserLabels.chooser, {exact: true});
  assert(await chooser.getAttribute('aria-describedby'), 'associated_upload_error');
  const inputId = await chooser.getAttribute('id');
  assert(inputId && await page.locator('label').evaluateAll((labels, id) => labels.some(label => label.htmlFor === id), inputId), 'keyboard_image_label');
  await chooser.focus();
  const fileChooser = page.waitForEvent('filechooser');
  await page.keyboard.press('Enter');
  await (await fileChooser).setFiles({name: path.basename(input.image.path), mimeType: input.mime, buffer: imageBytes});
  await chooser.focus();
  await page.keyboard.press('Tab');
  assert(await region.getByRole('button', {name: metadataBrowserLabels.upload, exact: true}).evaluate(button => button === document.activeElement), 'keyboard_upload_focus');
  const began = page.waitForRequest(request => request.method() === 'PUT' && request.url().includes(`/v2/tokens/${input.token_address}/assets/`));
  const interrupt = page.waitForRequest(request => request.method() === 'PUT' && request.url().includes(`/v2/tokens/${input.token_address}/assets/`)).then(async request => {
   await context.setOffline(true);
   return request.headers()['idempotency-key'];
  });
  await page.keyboard.press('Enter');
  await began;
  const operation = await interrupt;
  assert(operation && /^[a-f0-9-]{36}$/.test(operation), 'actual_upload_operation');
  await region.getByRole('alert').filter({hasText: metadataBrowserLabels.uploadUnavailable}).waitFor({state: 'visible'});
  assert(!(await region.textContent()).includes(metadataBrowserLabels.updated), 'no_optimistic_approval');
  await context.setOffline(false);
  await region.getByRole('button', {name: metadataBrowserLabels.upload, exact: true}).focus();
  const retried = page.waitForRequest(request => request.method() === 'PUT' && request.url().includes(`/v2/tokens/${input.token_address}/assets/`));
  await page.keyboard.press('Enter');
  assert((await retried).headers()['idempotency-key'] === operation, 'retry_operation_changed');
  await page.keyboard.press('Tab');
  const cancel = region.getByRole('button', {name: 'Cancel', exact: true});
  assert(await cancel.evaluate(button => button === document.activeElement && !button.disabled), 'keyboard_cancel_focus');
  await page.keyboard.press('Enter');
  await region.getByRole('alert').filter({hasText: metadataBrowserLabels.cancelled}).waitFor({state: 'visible'});
  assert(!(await region.textContent()).includes(metadataBrowserLabels.updated), 'cancel_published_late_approval');
  await region.getByRole('button', {name: metadataBrowserLabels.upload, exact: true}).focus();
  const finalRetry = page.waitForRequest(request => request.method() === 'PUT' && request.url().includes(`/v2/tokens/${input.token_address}/assets/`));
  await page.keyboard.press('Enter');
  assert((await finalRetry).headers()['idempotency-key'] === operation, 'cancel_retry_operation_changed');
  if (input.wallet_approval_selector) {
   assert(typeof input.wallet_approval_selector === 'string', 'wallet_approval_selector');
   const walletPage = await new Promise((resolve, reject) => {
    const existing = context.pages().find(candidate => candidate.url().startsWith('chrome-extension://'));
    if (existing) {resolve(existing);return;}
    const timeout = setTimeout(() => {context.off('page', opened);reject(new Error('actual_wallet_approval_page_missing'));}, 60000);
    const opened = candidate => {candidate.waitForURL(/^chrome-extension:\/\//).then(() => {clearTimeout(timeout);context.off('page', opened);resolve(candidate);}).catch(reject);};
    context.on('page', opened);
   });
   await walletPage.locator(input.wallet_approval_selector).click();
  }
  await region.getByRole('status').filter({hasText: metadataBrowserLabels.updated}).waitFor({state: 'visible', timeout: 120000});
  await capturedResponses();
  const approval = captureCommittedImage(network, input.token_address, operation, input.image.sha256);
  const approvedRevision = approval.approved_revision;
  assert(!(await region.textContent()).includes(metadataBrowserLabels.pending), 'approved_revision_pending_not_cleared');
  await page.goto(input.viewer_url, {waitUntil: 'networkidle'});
  const token = page.getByRole('region', {name: metadataBrowserLabels.viewer, exact: true});
  assert((await token.textContent()).includes(input.token_address), 'viewer_token_identity');
  const image = token.getByRole('img', {name: metadataBrowserLabels.image});
  await image.waitFor({state: 'visible'});
  const dimensions = await image.evaluate(element => ({width: element.getAttribute('width'), height: element.getAttribute('height'), currentSrc: element.currentSrc, loaded: element.complete && element.naturalWidth > 0}));
  assert(Number(dimensions.width) > 0 && Number(dimensions.height) > 0 && dimensions.loaded && /\/approved\/[a-f0-9]{64}\/(256|512|1024)\/[a-f0-9]{64}\.png$/.test(new URL(dimensions.currentSrc).pathname), 'responsive_approved_image');
  await capturedResponses();
  const viewerApproval = captureViewerImage(network, input.token_address, approval, input.image.sha256, dimensions.currentSrc);
  await page.addScriptTag({content: axeBytes.toString('utf8')});
  const axe = await page.evaluate(async () => globalThis.axe.run(document, {runOnly: {type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa']}}));
  assert(axe.violations.length === 0, 'actual_accessibility_violations');
  const layoutShift = await page.evaluate(() => globalThis.__metadataLayoutShift);
  assert(Number.isFinite(layoutShift) && layoutShift <= 0.1, 'responsive_layout_shift');
  metadataFaultStarted = true;
  const outage = nativeCapture('browser-metadata-loss');
  assert(outage.metadata_outage_observed === true, 'actual_metadata_outage_missing');
  await page.reload({waitUntil: 'domcontentloaded'});
  assert((await token.textContent()).includes(input.token_address), 'metadata_outage_lost_token');
  assert(await page.getByLabel('Trading', {exact: true}).isVisible(), 'metadata_outage_lost_trading');
  await token.getByRole('alert').filter({hasText: metadataBrowserLabels.outage}).waitFor({state: 'visible'});
  const result = {bindings: input.bindings, started_utc_ns: startedUtcNs, ended_utc_ns: String(BigInt(Date.now()) * 1000000n), token_address: input.token_address, content_sha256: input.image.sha256, license_receipt_sha256: input.license_receipt.sha256, source_receipt_sha256: input.source_receipt.sha256, operation_id: operation, approved_revision: approvedRevision, metadata_operation_id: approval.metadata_operation_id, viewer_revision: viewerApproval.revision, network, keyboard_upload_passed: true, keyboard_cancel_passed: true, network_interruption_recovered: true, metadata_outage_trade_accessible: true, axe_violations: axe.violations.length, layout_shift: layoutShift, viewport: {width: 390, height: 844}, image: dimensions};
  nativeCapture('browser-metadata-restore');
  metadataFaultStarted = false;
  fs.writeFileSync(input.output, JSON.stringify(result) + '\n', {flag: 'wx', mode: 0o600});
 } catch (error) {
  fs.writeFileSync(input.output, JSON.stringify({bindings: input.bindings, failure_class: error.name, code: /^[a-z0-9_]+$/.test(error.message) ? error.message : 'actual_browser_journey_failed', qualified: false, network}) + '\n', {flag: 'wx', mode: 0o600});
  process.exitCode = 1;
 } finally {
  try {if (metadataFaultStarted) nativeCapture('browser-metadata-restore');} catch {process.exitCode = 1;}
  await context?.close();
  await browser?.close();
 }
}

async function readBrowserInput() {
 const inputChunks = [];
 let inputBytes = 0;
 for await (const chunk of process.stdin) {inputBytes += chunk.length;assert(inputBytes <= 1048576, 'browser_input_bound');inputChunks.push(chunk);}
 return JSON.parse(Buffer.concat(inputChunks).toString('utf8'));
}

if (process.argv[1] && pathToFileURL(path.resolve(process.argv[1])).href === import.meta.url) {
 await runMetadataBrowser(await readBrowserInput());
}
