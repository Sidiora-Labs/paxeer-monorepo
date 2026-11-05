import assert from 'node:assert/strict';
import { readFileSync, writeFileSync, mkdirSync, copyFileSync, symlinkSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { stripTypeScriptTypes, registerHooks } from 'node:module';
import { spawnSync } from 'node:child_process';

const [mode, repo, temporary] = process.argv.slice(2);
const source = resolve(repo, 'sdk/launchpad/packages/sdk');
const destination = resolve(temporary, 'sdk');

if (mode === 'prepare') {
  for (const dir of ['sdk/src', 'sdk/scripts', 'sdk/dist', 'sdk-v2/src']) mkdirSync(resolve(temporary, dir), { recursive: true });
  const tools = resolve(temporary, 'tools');
  const fixtures = dirname(fileURLToPath(import.meta.url));
  mkdirSync(tools);
  for (const file of ['package.json', 'package-lock.json']) copyFileSync(resolve(fixtures, `${file}.fixture`), resolve(tools, file));
  const installed = spawnSync('npm', ['ci', '--ignore-scripts', '--no-audit', '--no-fund'], { cwd: tools, encoding: 'utf8' });
  assert.equal(installed.status, 0, installed.stderr);
  symlinkSync(resolve(tools, 'node_modules'), resolve(temporary, 'node_modules'), 'dir');
  copyFileSync(resolve(source, 'scripts/compile-validators.mjs'), resolve(destination, 'scripts/compile-validators.mjs'));
  for (const file of ['schemas.gen.json', 'operations.gen.ts']) copyFileSync(resolve(source, 'src', file), resolve(destination, 'src', file));
  copyFileSync(resolve(repo, 'sdk/launchpad/packages/sdk-v2/src/schemas.gen.json'), resolve(temporary, 'sdk-v2/src/schemas.gen.json'));
  writeFileSync(resolve(temporary, 'package.json'), '{"type":"module"}');
  const compiled = spawnSync(process.execPath, [resolve(destination, 'scripts/compile-validators.mjs')], { encoding: 'utf8' });
  assert.equal(compiled.status, 0, compiled.stderr);
  for (const file of ['transport', 'validate', 'operations', 'operations.gen', 'shared-snapshot', 'served-routes', 'route-corrections']) {
    const actual = readFileSync(resolve(source, `src/${file}.ts`), 'utf8');
    writeFileSync(resolve(destination, `dist/${file}.js`), stripTypeScriptTypes(actual, { mode: 'transform' }));
  }
  process.stdout.write('{}');
  process.exit(0);
}

const input = JSON.parse(readFileSync(0, 'utf8'));
const { decodeV2Contract, SdkClient, HttpError } = await import(pathToFileURL(resolve(destination, 'dist/transport.js')));

if (mode === 'decode') {
  const value = decodeV2Contract(input.schema, JSON.parse(input.wire));
  assert.throws(() => decodeV2Contract(input.schema, { ...JSON.parse(input.wire), requestId: 42 }), TypeError);
  process.stdout.write(JSON.stringify(value));
} else if (mode === 'operation') {
  const origin = input.origin;
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: origin.replace(/^http/, 'ws'), approvedStreamOrigins: [origin.replace(/^http/, 'ws')], production: false, session: { mode: 'bearer', token: () => 'contract-session' } });
  const { Operations } = await import(pathToFileURL(resolve(destination, 'dist/operations.js')));
  const value = await new Operations(client).call(input.operation, input.input);
  process.stdout.write(JSON.stringify(value));
} else if (mode === 'retired') {
  const { Operations } = await import(pathToFileURL(resolve(destination, 'dist/operations.js')));
  const origin = input.origin;
  const client = new SdkClient({ apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin: origin.replace(/^http/, 'ws'), approvedStreamOrigins: [origin.replace(/^http/, 'ws')], production: false, session: { mode: 'bearer', token: () => input.session } });
  try {
    await new Operations(client).call('IF_STREAM_INGEST_CREDENTIAL', { path: { streamId: input.id } });
    assert.fail('retired product issued a credential');
  } catch (error) {
    assert.ok(error instanceof HttpError);
    assert.equal(error.status, 410);
    assert.equal(error.envelope.code, 'INGEST_CREDENTIAL_RETIRED');
    assert.equal(error.envelope.retryable, false);
    process.stdout.write(JSON.stringify(error.envelope));
  }
} else if (mode === 'browser-key') {
  // Resolve the application's actual imports; no replacement parser, signer,
  // transport or fetch implementation participates in this proof.
  const app = resolve(repo, 'apps/launchpad/web/src');
  registerHooks({
    resolve(specifier, context, next) {
      if (specifier.startsWith('@/')) return { shortCircuit: true, url: pathToFileURL(resolve(app, `${specifier.slice(2)}.ts`)).href };
      if (specifier.startsWith('.') && context.parentURL?.endsWith('.ts') && !/\.[cm]?[jt]s$/.test(specifier)) return { shortCircuit: true, url: new URL(`${specifier}.ts`, context.parentURL).href };
      return next(specifier, context);
    },
    load(url, context, next) {
      if (url.endsWith('.ts')) return { shortCircuit: true, format: 'module', source: stripTypeScriptTypes(readFileSync(fileURLToPath(url), 'utf8'), { mode: 'transform' }) };
      return next(url, context);
    },
  });
  process.env.NEXT_PUBLIC_LIVESTREAM_API_URL = input.origin;
  process.env.NEXT_PUBLIC_ACTION_ENVIRONMENT = 'test';
  process.env.NODE_ENV = 'test';
  const { getPrivateIngestKey } = await import(pathToFileURL(resolve(app, 'core/clients/livestream-api.ts')));
  const signing = {
    wallet: input.wallet,
    environment: 'test',
    assertCurrent() { assert.equal(signing.wallet, input.wallet); assert.equal(signing.environment, 'test'); },
    async signMessage(message) {
      const response = await fetch(`${input.signer}/sign`, { method: 'POST', body: message });
      assert.equal(response.status, 200);
      return response.text();
    },
  };
  const key = await getPrivateIngestKey(input.id, input.session, signing);
  assert.equal(key.streamKey, input.expectedKey);
  assert.equal(key.ingestEndpoint, input.expectedEndpoint);
  assert.ok(Date.parse(key.visibilityUntil) > Date.now());
  process.stdout.write(JSON.stringify({ accepted: true }));
} else {
  throw new Error(`Unknown proof mode ${mode}`);
}
