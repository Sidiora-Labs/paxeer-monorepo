import { readFile, writeFile, rename, mkdir, open, unlink } from 'node:fs/promises';
import { basename, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { tmpdir } from 'node:os';
import { ENVIRONMENT_ID, PROJECT_ID, DOKPLOY_URL, MODEL, ReleaseError, ensure, environmentNames, mergeEnvironment, parseArgs, releaseId, safeFailure, sha256, siteConfig, bindArchiveManifest } from './release-config.mjs';

const delay = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
const terminal = new Set(['done', 'error', 'failed', 'cancelled', 'canceled']);
function deploymentSummary(value) {
  return { deploymentId: value.deploymentId, status: value.status, createdAt: value.createdAt };
}
function applicationSummary(value) {
  return { applicationId: value.applicationId, name: value.name, appName: value.appName, environmentId: value.environmentId, sourceType: value.sourceType, buildType: value.buildType, dockerfile: value.dockerfile, dockerContextPath: value.dockerContextPath, applicationStatus: value.applicationStatus, environmentKeys: environmentNames(value.env), domains: (value.domains || []).map(domain => ({ domainId: domain.domainId, host: domain.host, port: domain.port, path: domain.path, https: domain.https, certificateType: domain.certificateType })) };
}
async function readJSON(path) { return JSON.parse(await readFile(path, 'utf8')); }
async function optionalJSON(path) { try { return await readJSON(path); } catch (error) { if (error.code === 'ENOENT') return null; throw error; } }
async function saveState(path, state) {
  const temporary = `${path}.${process.pid}.tmp`;
  await writeFile(temporary, JSON.stringify({ ...state, updatedAt: new Date().toISOString() }, null, 2) + '\n', { mode: 0o600 });
  await rename(temporary, path);
}
function scopedApplication(application, site) {
  const config = siteConfig(site);
  ensure(application.name === config.name && (application.appName === site || application.appName?.startsWith(`${site}-`)), 'application_identity', 'The selected application does not match the exact docs application identity.');
  ensure(application.environmentId === ENVIRONMENT_ID && !application.serverId && !application.buildServerId, 'application_scope', 'The docs application is outside the authorized environment or local build/runtime server.');
}
function client(key) {
  return async (path, { method = 'GET', body, form, timeout = 30000 } = {}) => {
    let response;
    try {
      response = await fetch(DOKPLOY_URL + path, { method, headers: { 'x-api-key': key, ...(body ? { 'content-type': 'application/json' } : {}) }, body: form || (body ? JSON.stringify(body) : undefined), signal: AbortSignal.timeout(timeout), redirect: 'error' });
    } catch (error) {
      throw new ReleaseError('dokploy_request_uncertain', `Dokploy request did not return a response (${error.name}); inspect recorded state before another mutation.`);
    }
    if (!response.ok) {
      await response.body?.cancel();
      throw new ReleaseError('dokploy_http_error', `Dokploy returned HTTP ${response.status}. Response bodies and credentials are omitted.`);
    }
    const text = await response.text();
    if (!text) return null;
    try { return JSON.parse(text); } catch { throw new ReleaseError('dokploy_response_shape', 'Dokploy returned a non-JSON response.'); }
  };
}
async function validateRelease(path) {
  const manifest = await readJSON(path), config = siteConfig(manifest.site);
  releaseId(manifest.release);
  ensure(manifest.schema === 1 && manifest.origin === config.origin && manifest.port === 3000 && manifest.model === MODEL, 'manifest_scope', 'The manifest does not match the fixed docs deployment configuration.');
  ensure(manifest.zipFile === basename(manifest.zipFile || '') && manifest.zipFile.endsWith('.zip'), 'zip_path', 'The ZIP must be beside its manifest.');
  const zip = await readFile(resolve(dirname(path), manifest.zipFile));
  ensure(zip.length === manifest.zipBytes && sha256(zip) === manifest.zipSha256, 'zip_hash', 'The release ZIP does not match its manifest.');
  for (const name of ['sourceSha256', 'buildSha256', 'zipSha256']) ensure(/^[a-f0-9]{64}$/.test(manifest[name]), 'manifest_hash', 'A release fingerprint is invalid.');
  bindArchiveManifest(manifest, zip);
  return { manifest, config, zip };
}
async function pollDeployment(api, state, statePath, timeout) {
  const deadline = Date.now() + timeout;
  do {
    const deployments = await api(`/api/deployment.all?applicationId=${encodeURIComponent(state.applicationId)}`);
    ensure(Array.isArray(deployments), 'deployment_response', 'Dokploy returned an unexpected deployment list.');
    const newRows = deployments.filter(row => row.deploymentId && !state.baselineDeploymentIds.includes(row.deploymentId));
    if (!state.deploymentId) {
      ensure(newRows.length <= 1, 'ambiguous_deployment', 'Multiple new deployments appeared. Inspect their IDs before continuing; the upload will not be repeated.');
      if (newRows[0]) state.deploymentId = newRows[0].deploymentId;
    }
    const deployment = deployments.find(row => row.deploymentId === state.deploymentId);
    if (deployment) {
      state.deployment = deploymentSummary(deployment); state.phase = 'deployment_observed';
      await saveState(statePath, state);
      if (terminal.has(String(deployment.status).toLowerCase())) {
        state.status = String(deployment.status).toLowerCase() === 'done' ? (state.uploadAcknowledged ? 'deployment_done' : 'deployment_observed_unattributed') : 'failed';
        if (!state.uploadAcknowledged) state.nextAction = 'A deployment finished after the uncertain upload. Verify the expected release and content before attributing that deployment to this archive; this tool will not re-upload.';
        state.phase = 'terminal';
        return deployment;
      }
    }
    if (Date.now() >= deadline) break;
    await delay(Math.min(3000, deadline - Date.now()));
  } while (Date.now() <= deadline);
  state.status = state.deploymentId ? 'pending' : 'upload_outcome_unknown';
  state.phase = 'resume_required';
  state.nextAction = 'Run the same command with --execute --resume to check this upload. It will not upload or configure again.';
  return null;
}
async function main() {
  const args = parseArgs(process.argv.slice(2), ['--manifest', '--status-file', '--wait-ms'], ['--execute', '--resume', '--help']);
  if (args['--help']) {
    console.log('Usage: node --env-file=.env docs-assistant/scripts/deploy-release.mjs --manifest <file.manifest.json> [--execute] [--resume] [--wait-ms 600000] [--status-file <file.json>]\nWithout --execute: local validation and sanitized plan only.\nWith --execute: creates/updates only the three named docs applications, uploads once and polls.\nAfter an interrupted/uncertain upload, --resume performs read-only deployment checks; it never uploads or configures again.\nRequires DOKPLOY_API_KEY and GATEWAY_ROUTER_API_KEY in the process environment.'); return;
  }
  ensure(args['--manifest'], 'missing_manifest', 'Supply --manifest for a packaged release.');
  ensure(!args['--resume'] || args['--execute'], 'resume_requires_execute', 'Use --execute --resume for remote readback.');
  const manifestPath = resolve(args['--manifest']), { manifest, config, zip } = await validateRelease(manifestPath);
  const timeout = Number(args['--wait-ms'] || 600000);
  ensure(Number.isInteger(timeout) && timeout >= 1000 && timeout <= 3600000, 'invalid_wait', 'The wait time must be between 1000 and 3600000 milliseconds.');
  const statePath = resolve(args['--status-file'] || manifestPath.replace(/\.json$/, '') + '.deployment.json');
  const plan = { status: 'plan', site: manifest.site, applicationName: config.name, release: manifest.release, origin: config.origin, projectId: PROJECT_ID, environmentId: ENVIRONMENT_ID, dokploy: DOKPLOY_URL, port: 3000, model: MODEL, zipSha256: manifest.zipSha256, zipBytes: zip.length, statusFile: statePath, credentialNames: ['DOKPLOY_API_KEY', 'GATEWAY_ROUTER_API_KEY'], runtimeCredentialName: 'CENTRA_GATEWAY_API_KEY', uploadQueuesDeployment: true };
  if (!args['--execute']) { console.log(JSON.stringify(plan, null, 2)); return; }
  ensure(process.env.DOKPLOY_API_KEY, 'missing_dokploy_key', 'DOKPLOY_API_KEY must be set in the process environment.');
  ensure(args['--resume'] || process.env.GATEWAY_ROUTER_API_KEY, 'missing_gateway_key', 'GATEWAY_ROUTER_API_KEY must be set in the process environment.');
  await mkdir(dirname(statePath), { recursive: true });
  const lockDirectory = resolve(tmpdir(), 'centra-docs-deployment-locks'); await mkdir(lockDirectory, { recursive: true, mode: 0o700 });
  const lockPath = resolve(lockDirectory, `${manifest.site}.lock`);
  let lock;
  try { lock = await open(lockPath, 'wx', 0o600); await lock.writeFile(JSON.stringify({ pid: process.pid, release: manifest.release, site: manifest.site })); }
  catch (error) { if (error.code === 'EEXIST') throw new ReleaseError('deployment_locked', 'Another deployment process or a stale lock exists. Inspect the lock and process before resuming.'); throw error; }
  let state;
  try {
    const prior = await optionalJSON(statePath), api = client(process.env.DOKPLOY_API_KEY);
    if (prior) ensure(prior.site === manifest.site && prior.release === manifest.release && prior.zipSha256 === manifest.zipSha256, 'state_mismatch', 'The status file belongs to a different release.');
    state = prior || { ...plan, status: 'configuring', phase: 'starting', startedAt: new Date().toISOString(), uploadAttempted: false, baselineDeploymentIds: [], qualification: 'Dokploy queue status only. Canonical DNS, TLS, health release marker and actual assistant responses require separate verification.' };
    const projects = await api('/api/project.all');
    const project = Array.isArray(projects) && projects.find(item => item.projectId === PROJECT_ID);
    const environment = project?.environments?.find(item => item.environmentId === ENVIRONMENT_ID);
    ensure(environment, 'environment_missing', 'The authorized Dokploy project/environment is unavailable.');
    if (prior?.uploadAttempted || args['--resume']) {
      ensure(state.applicationId && state.uploadAttempted, 'nothing_to_resume', 'There is no recorded upload to resume.');
      const application = await api(`/api/application.one?applicationId=${encodeURIComponent(state.applicationId)}`); scopedApplication(application, manifest.site);
      await pollDeployment(api, state, statePath, timeout);
      const readback = await api(`/api/application.one?applicationId=${encodeURIComponent(state.applicationId)}`); scopedApplication(readback, manifest.site); state.readback = applicationSummary(readback);
      await saveState(statePath, state); console.log(JSON.stringify(state, null, 2));
      if (state.status !== 'deployment_done') process.exitCode = state.status === 'failed' ? 1 : 2;
      return;
    }
    const matches = (environment.applications || []).filter(item => item.name === config.name);
    ensure(matches.length <= 1, 'duplicate_application', 'Multiple applications have the exact docs name; select and reconcile them before deploying.');
    let application;
    if (matches[0]) application = await api(`/api/application.one?applicationId=${encodeURIComponent(matches[0].applicationId)}`);
    else {
      state.phase = 'creating_application'; await saveState(statePath, state);
      const created = await api('/api/application.create', { method: 'POST', body: { name: config.name, appName: manifest.site, environmentId: ENVIRONMENT_ID } });
      ensure(created?.applicationId, 'application_create_response', 'Dokploy did not return an application ID. Check the project before retrying.');
      application = await api(`/api/application.one?applicationId=${encodeURIComponent(created.applicationId)}`);
    }
    scopedApplication(application, manifest.site); state.applicationId = application.applicationId; state.appName = application.appName;
    const baseline = await api(`/api/deployment.all?applicationId=${encodeURIComponent(application.applicationId)}`);
    ensure(Array.isArray(baseline), 'deployment_response', 'Dokploy returned an unexpected deployment list.');
    ensure(!baseline.some(row => !terminal.has(String(row.status).toLowerCase())), 'deployment_in_progress', 'This application already has an unfinished deployment. Wait before configuring another release.');
    state.baselineDeploymentIds = baseline.map(row => row.deploymentId).filter(Boolean);
    state.phase = 'configuring_application'; await saveState(statePath, state);
    const bodyId = { applicationId: application.applicationId };
    await api('/api/application.update', { method: 'POST', body: { ...bodyId, sourceType: 'drop', dropBuildPath: '' } });
    await api('/api/application.saveBuildType', { method: 'POST', body: { ...bodyId, buildType: 'dockerfile', dockerfile: 'Dockerfile', dockerContextPath: '.', dockerBuildStage: null, herokuVersion: application.herokuVersion ?? null, railpackVersion: application.railpackVersion ?? null } });
    application = await api(`/api/application.one?applicationId=${encodeURIComponent(application.applicationId)}`); scopedApplication(application, manifest.site);
    const managed = { NODE_ENV: 'production', PORT: '3000', TRUST_PROXY: '1', CENTRA_GATEWAY_API_KEY: process.env.GATEWAY_ROUTER_API_KEY, CENTRA_GATEWAY_URL: 'https://gateway.centra.ag/v1', DOCS_ASSISTANT_MODEL: MODEL, RELEASE_ID: manifest.release };
    const merged = mergeEnvironment(application.env, managed);
    await api('/api/application.saveEnvironment', { method: 'POST', body: { ...bodyId, env: merged, buildArgs: application.buildArgs ?? null, buildSecrets: application.buildSecrets ?? null, createEnvFile: application.createEnvFile ?? false } });
    const host = new URL(config.origin).hostname, domains = (application.domains || []).filter(domain => domain.host === host);
    ensure(domains.length <= 1, 'duplicate_domain', 'The canonical host is attached more than once to this application.');
    const settings = { host, path: '/', internalPath: '/', stripPath: false, port: 3000, https: true, certificateType: 'letsencrypt', domainType: 'application' };
    if (domains[0]) {
      const current = domains[0];
      await api('/api/domain.update', { method: 'POST', body: { ...settings, domainId: current.domainId, middlewares: current.middlewares ?? [], forwardAuthEnabled: current.forwardAuthEnabled ?? false, customEntrypoint: current.customEntrypoint ?? null } });
    } else await api('/api/domain.create', { method: 'POST', body: { ...settings, ...bodyId } });
    const configured = await api(`/api/application.one?applicationId=${encodeURIComponent(application.applicationId)}`); scopedApplication(configured, manifest.site);
    ensure(configured.env === merged && configured.buildArgs === (application.buildArgs ?? null) && configured.buildSecrets === (application.buildSecrets ?? null), 'environment_readback', 'Application environment/build settings did not match the preserved update. No upload was attempted.');
    ensure(configured.buildType === 'dockerfile' && configured.dockerfile === 'Dockerfile' && configured.sourceType === 'drop', 'build_readback', 'Application source/build readback differs from the release configuration.');
    ensure(configured.domains?.some(domain => domain.host === host && domain.port === 3000 && domain.https && domain.certificateType === 'letsencrypt'), 'domain_readback', 'The canonical domain configuration was not confirmed. No upload was attempted.');
    state.readback = applicationSummary(configured);
    const beforeUpload = await api(`/api/deployment.all?applicationId=${encodeURIComponent(application.applicationId)}`);
    ensure(Array.isArray(beforeUpload) && !beforeUpload.some(row => !state.baselineDeploymentIds.includes(row.deploymentId)), 'concurrent_deployment', 'Another deployment appeared during configuration. No upload was attempted.');
    state.uploadAttempted = true; state.uploadStartedAt = new Date().toISOString(); state.phase = 'upload_intent_recorded'; state.status = 'uploading'; await saveState(statePath, state);
    const form = new FormData(); form.set('applicationId', application.applicationId); form.set('zip', new Blob([zip], { type: 'application/zip' }), manifest.zipFile); form.set('dropBuildPath', '');
    try {
      await api('/api/application.dropDeployment', { method: 'POST', form, timeout: 120000 });
      state.uploadAcknowledged = true; state.phase = 'upload_acknowledged';
    } catch (error) {
      state.uploadAcknowledged = false; state.uploadError = safeFailure(error); state.phase = 'checking_uncertain_upload';
    }
    await saveState(statePath, state);
    await pollDeployment(api, state, statePath, timeout);
    const readback = await api(`/api/application.one?applicationId=${encodeURIComponent(application.applicationId)}`); scopedApplication(readback, manifest.site); state.readback = applicationSummary(readback);
    state.qualification = 'Dokploy deployment status only. Canonical DNS, TLS, health release marker and real assistant responses must be checked separately.';
    await saveState(statePath, state); console.log(JSON.stringify(state, null, 2));
    if (state.status !== 'deployment_done') process.exitCode = state.status === 'failed' ? 1 : 2;
  } catch (error) {
    if (state) { state.error = safeFailure(error); state.status = state.uploadAttempted ? 'resume_required' : 'failed'; await saveState(statePath, state); }
    throw error;
  } finally { await lock.close(); await unlink(lockPath); }
}
if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) main().catch(error => { console.error(JSON.stringify({ status: 'failed', ...safeFailure(error) })); process.exitCode = 1; });
